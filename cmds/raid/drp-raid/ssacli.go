package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type SsaCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
	enabled    bool
}

var (
	ssaArrayRE    = regexp.MustCompile(`^   (Unassigned|Array: (.*))$`)
	ssaPdRE       = regexp.MustCompile(`^      physicaldrive ([^ ]+)$`)
	ssaLdRE       = regexp.MustCompile(`^      Logical Drive: (.*)$`)
	ssaCliCSlotRE = regexp.MustCompile(` in Slot (.+)$`)
	ssaPciRE      = regexp.MustCompile(`([[:xdigit:]]{4}):([[:xdigit:]]{2}):([[:xdigit:]]{2})\.([[:xdigit:]])`)
)

func (s *SsaCli) Logger(l *log.Logger) {
	s.log = l
}

func (s *SsaCli) Order() int    { return s.order }
func (s *SsaCli) Enabled() bool { return s.enabled }
func (s *SsaCli) Enable()       { s.enabled = true }
func (s *SsaCli) Disable()      { s.enabled = false }

func (s *SsaCli) Name() string { return s.name }

func (s *SsaCli) Executable() string { return s.executable }

func (s *SsaCli) checkLinesForError(lines []string) error {
	if len(lines) > 1 && strings.HasPrefix(lines[1], "Error:") {
		return errors.New(lines[1])
	}
	return nil
}

func (s *SsaCli) run(args ...string) ([]string, error) {
	if fake {
		return []string{}, nil
	}
	cmd := exec.Command(s.executable, args...)
	outBuf := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = outBuf, outBuf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	out := strings.Split(outBuf.String(), "\n")
	return out, s.checkLinesForError(out)
}

func (s *SsaCli) Useable() bool {
	_, err := s.run("controller", "all", "show")
	return err == nil
}

func (s *SsaCli) fillDisk(d *PhysicalDisk, lines []string) {
	d.Info = map[string]string{}
	for _, line := range lines[1:] {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		d.Info[k] = v
		switch k {
		case "Size":
			d.Size, _ = sizeParser(v)
		case "Interface Type":
			v = strings.ToLower(v)
			if strings.Contains(v, "solid state") || strings.Contains(v, "nvme") {
				d.MediaType = "ssd"
			} else {
				d.MediaType = "disk"
			}
			if strings.Contains(v, "sata") {
				d.Protocol = "sata"
			} else if strings.Contains(v, "sas") {
				d.Protocol = "sas"
			} else if strings.Contains(v, "nvme") {
				d.Protocol = "nvme"
			} else if strings.Contains(v, "scsi") {
				d.Protocol = "scsi"
			} else if strings.Contains(v, "pcie") {
				d.Protocol = "pcie"
			} else {
				d.Protocol = "unknown"
			}
		case "Logical/Physical Block Size":
			sizes := strings.SplitN(v, "/", 2)
			d.LogicalSectorSize, _ = strconv.ParseUint(sizes[0], 10, 64)
			d.PhysicalSectorSize, _ = strconv.ParseUint(sizes[1], 10, 64)
		case "Status":
			d.Status = v
		case "Bay":
			d.Slot, _ = strconv.ParseUint(v, 10, 64)
		case "Drive exposed to OS":
			d.JBOD = v == "True"
		}
	}
	d.Enclosure = d.Info["Port"] + ":" + d.Info["Box"]
}

func (s *SsaCli) fillVolume(vol *Volume, lines []string) {
	vol.Info = map[string]string{}
	for _, line := range lines[1:] {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		vol.Info[k] = v
		switch k {
		case "Logical Drive Label":
			vol.Name = v
		case "Logical Drive":
			vol.ID = v
		case "Status":
			vol.Status = v
		case "Fault Tolerance":
			switch v {
			case "0":
				vol.RaidLevel = "raid0"
			case "1", "1adm":
				vol.RaidLevel = "raid1"
			case "5":
				vol.RaidLevel = "raid5"
			case "6":
				vol.RaidLevel = "raid6"
			case "1+0":
				vol.RaidLevel = "raid10"
			case "1+0adm":
				vol.RaidLevel = "raid10"
			case "50":
				vol.RaidLevel = "raid50"
			case "60":
				vol.RaidLevel = "raid60"
			default:
				vol.RaidLevel = "raid" + v
			}
		case "Size":
			vol.Size, _ = sizeParser(v)
		case "Strip Size":
			vol.StripeSize, _ = sizeParser(v)

		}
	}
}

func (s *SsaCli) fillArray(c *Controller, array []string) {
	ldstuff, phys := partitionAt(array, ssaPdRE)
	arrayInfo, lds := partitionAt(ldstuff, ssaLdRE)
	disks := make([]*PhysicalDisk, len(phys))
	for i, phy := range phys {
		d := &PhysicalDisk{
			controller:       c,
			ControllerID:     c.ID,
			ControllerDriver: c.Driver,
			driver:           s,
		}
		s.fillDisk(d, phy)
		disks[i] = d
		if d.JBOD {
			c.addJBODVolume(d)
		}
	}
	c.Disks = append(c.Disks, disks...)
	if len(lds) == 0 {
		return
	}
	for _, ld := range lds {
		vol := &Volume{
			ControllerID:     c.ID,
			ControllerDriver: c.Driver,
			Disks:            disks,
			controller:       c,
			driver:           s,
		}
		s.fillVolume(vol, append(ld, arrayInfo...))
		c.Volumes = append(c.Volumes, vol)
	}
}

func (s *SsaCli) fillController(c *Controller, lines []string) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	controller, arrays := partitionAt(lines, ssaArrayRE)
	c.JBODCapable = false
	c.RaidCapable = true
	c.Info = map[string]string{}
	for _, line := range controller[1:] {
		k, v := kv(line, ": ")
		if k == "" {
			continue
		}
		c.Info[k] = v
		switch k {
		case "Slot":
			c.ID = v
		case "Controller Mode":
			switch v {
			case "Mixed":
				c.JBODCapable = true
				c.RaidCapable = true
				c.AutoJBOD = true
			default:
				c.RaidCapable = true
			}
		case "PCI Address (Domain:Bus:Device.Function)":
			pciParts := ssaPciRE.FindStringSubmatch(v)
			if len(pciParts) != 5 {
				s.log.Printf("Error parsing PCI address %s", v)
			} else {
				c.PCI.Bus, _ = strconv.ParseInt(pciParts[2], 16, 64)
				c.PCI.Device, _ = strconv.ParseInt(pciParts[3], 16, 64)
				c.PCI.Function, _ = strconv.ParseInt(pciParts[4], 16, 64)
			}
		}
	}
	for _, array := range arrays {
		s.fillArray(c, array)
	}
}

func (s *SsaCli) Controllers() []*Controller {
	out, err := s.run("controller", "all", "show", "config", "detail")
	if err != nil {
		return nil
	}
	_, controllers := partitionAt(out, ssaCliCSlotRE)
	res := make([]*Controller, len(controllers))
	for i, controller := range controllers {
		res[i] = &Controller{
			Driver: s.name,
			driver: s,
			idx:    i,
		}
		s.fillController(res[i], controller)
	}
	return res
}

func (s *SsaCli) canBeCleared(c *Controller) bool {
	for _, vol := range c.Volumes {
		if vol.Fake {
			continue
		}
		return true
	}
	return false
}

func (s *SsaCli) Clear(c *Controller, onlyForeign bool) error {
	if onlyForeign {
		// as far as I can tell, ssacli has no notion of a foreign config.
		// So if we are asked to clear just the foreign config, do nothing.
		return nil
	}
	if !s.canBeCleared(c) {
		return nil
	}
	_, err := s.run("controller", "slot="+c.ID, "delete", "forced", "override")
	return err
}

func (s SsaCli) Refresh(c *Controller) {
	lines, err := s.run("controller", "slot="+c.ID, "show", "config", "detail")
	if err != nil {
		return
	}
	s.fillController(c, lines)
}

func (s *SsaCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		parts[i] = fmt.Sprintf("%s:%d", disks[i].Enclosure, disks[i].Slot)
	}
	return fmt.Sprintf("drives=%s", strings.Join(parts, ","))
}

func (s *SsaCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	cmdLine := []string{
		"controller",
		"slot=" + c.ID,
		"create",
		"type=ld",
		"size=max",
		fmt.Sprintf("stripsize=%d", v.stripeSize()>>10),
	}
	switch v.RaidLevel {
	case "jbod":
		if c.Info["Controller Mode"] == "Mixed" && len(v.Disks) == 1 {
			// Controller will automatically expose non-configured drives to the OS
			// So, do nothing.
			s.log.Printf("Controller in mixed mode, drive already exposed to OS")
			return nil
		}
		// Yes, I know this is wrong for jbod, but ssacli Is Not Helpful.
		cmdLine = append(cmdLine, "raid=0")
	case "raid0":
		cmdLine = append(cmdLine, "raid=0")
	case "raid1":
		if len(v.Disks) == 2 {
			cmdLine = append(cmdLine, "raid=1")
		} else {
			cmdLine = append(cmdLine, "raid=1adm")
		}
	case "raid5":
		cmdLine = append(cmdLine, "raid=5")
	case "raid6":
		cmdLine = append(cmdLine, "raid=6")
	case "raid10":
		cmdLine = append(cmdLine, "raid=1+0")
	case "raid50":
		cmdLine = append(cmdLine, "raid=50")
	case "raid60":
		cmdLine = append(cmdLine, "raid=60")
	default:
		return fmt.Errorf("Raid level %s not supported", v.RaidLevel)
	}
	if v.Name != "" {
		cmdLine = append(cmdLine, fmt.Sprintf("logicaldrivelabel=%s", v.Name))
	}
	cmdLine = append(cmdLine, s.diskList(v.Disks), "forced")
	s.log.Printf("Running %s %s", s.executable, strings.Join(cmdLine, " "))
	res, err := s.run(cmdLine...)
	if len(res) > 0 {
		s.log.Println(strings.Join(res, "\n"))
	}
	if err != nil {
		s.log.Printf("Error running command: %s", strings.Join(cmdLine, " "))
	}
	return err
}

func (s *SsaCli) Encrypt(c *Controller, key, password string) error {
	cmdLine := []string{
		"controller",
		"slot=" + c.ID,
		"clearencryptionconfig",
		"forced",
	}
	s.log.Printf("Running %s %s", s.executable, strings.Join(cmdLine, " "))
	res, err := s.run(cmdLine...)
	if len(res) > 0 {
		s.log.Println(strings.Join(res, "\n"))
	}
	// Assume clear worked

	cmdLine = []string{
		"controller",
		"slot=" + c.ID,
		"enableencryption",
		"encryption=on",
		"eula=yes",
		fmt.Sprintf("masterkey=\"%s\"", key),
		"localkeymanagermode=on",
		"mixedvolumes=on",
		fmt.Sprintf("password=\"%s\"", password),
	}
	s.log.Printf("Running %s %s", s.executable, strings.Join(cmdLine, " "))
	res, err = s.run(cmdLine...)
	if len(res) > 0 {
		s.log.Println(strings.Join(res, "\n"))
	}
	if err != nil {
		s.log.Printf("Error running command: %s", strings.Join(cmdLine, " "))
	}
	return err
}
