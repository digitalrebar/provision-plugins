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

type PercCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
	enabled    bool
}

var (
	percEidLine    = regexp.MustCompile(`^([0-9 ]+):([0-9]+)[\t ]+([0-9]+)[\t ]+([^ ]+)[\t ]+([^ ]+)[\t ]+([0-9.]+ [TG]B)[\t ]+([^ \t]+)[ \t]+([^ \t]+)[ \t]+([^ \t]+)[ \t]+([^ \t]+)[ \t]+([^ \t]+)[ \t]+([^ \t]+)`)
	percArrayRE    = regexp.MustCompile(`^Enclosure Information :$`)
	percPdRE       = regexp.MustCompile(`^Drive ([^ ]+) :$`)
	percLdRE       = regexp.MustCompile(`^Logical Drive (.*) :$`)
	percCliCSlotRE = regexp.MustCompile(`^Basics :$`)
	percPciRE      = regexp.MustCompile(`([[:xdigit:]]{2}):([[:xdigit:]]{2}):([[:xdigit:]]{2}):([[:xdigit:]]{2})`)
)

func (s *PercCli) Logger(l *log.Logger) {
	s.log = l
}

func (s *PercCli) Order() int    { return s.order }
func (s *PercCli) Enabled() bool { return s.enabled }
func (s *PercCli) Enable()       { s.enabled = true }
func (s *PercCli) Disable()      { s.enabled = false }

func (s *PercCli) Name() string { return s.name }

func (s *PercCli) Executable() string { return s.executable }

func (s *PercCli) checkLinesForError(lines []string) error {
	if len(lines) > 1 && strings.HasPrefix(lines[1], "Error:") {
		return errors.New(lines[1])
	}
	return nil
}

func (s *PercCli) run(args ...string) ([]string, error) {
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

func (s *PercCli) Useable() bool {
	_, err := s.run("/call", "show")
	return err == nil
}

func (s *PercCli) fillDisk(d *PhysicalDisk, lines []string) {
	d.Info = map[string]string{}
	for _, line := range lines[1:] {
		k, v := kv(line, "= ")
		if k == "" {
			// Check for EID Line
			if percEidLine.MatchString(line) {
				pieces := percEidLine.FindStringSubmatch(line)
				d.Enclosure = pieces[1]
				d.Slot, _ = strconv.ParseUint(pieces[2], 10, 64)
				d.Status = pieces[4]
				d.Protocol = pieces[7]
				d.MediaType = pieces[8]
			}
			continue
		}
		d.Info[k] = v
		switch k {
		case "Raw size":
			d.Size, _ = sizeParser(v)
		case "Number of Blocks":
			d.SectorCount, _ = strconv.ParseUint(v, 10, 64)
		case "Sector Size":
			d.LogicalSectorSize, _ = sizeParser(v)
			d.PhysicalSectorSize, _ = sizeParser(v)
		case "Drive exposed to OS":
			d.JBOD = v == "True"
		}
	}
}

// XXX: This is not currently supported.
func (s *PercCli) fillVolume(vol *Volume, lines []string) {
	vol.Info = map[string]string{}
	for _, line := range lines[1:] {
		k, v := kv(line, "= ")
		if k == "" {
			continue
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

func (s *PercCli) fillArray(c *Controller, array []string) {
	ldstuff, phys := partitionAt(array, percPdRE)
	arrayInfo, lds := partitionAt(ldstuff, percLdRE)
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
	if len(lds) > 0 {
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

	// Make fake jbods
	seenDisks := map[string][]string{}
	for _, vol := range c.Volumes {
		for _, vd := range vol.Disks {
			as, ok := seenDisks[vol.ID]
			if !ok {
				as = []string{}
			}
			as = append(as, vd.Name())
			seenDisks[vol.ID] = as
		}
	}
	for vid, vpids := range seenDisks {
		for _, vpid := range vpids {
			for i, d := range c.Disks {
				if d.Name() == vpid {
					c.Disks[i].VolumeID = vid
				}
			}
		}
	}
	for _, d := range c.Disks {
		if d.VolumeID == "" {
			c.addJBODVolume(d)
		}
	}
}

func (s *PercCli) fillController(c *Controller, lines []string) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	controller, arrays := partitionAt(lines, percArrayRE)
	c.JBODCapable = true
	c.RaidCapable = false
	c.AutoJBOD = true
	c.RaidLevels = []string{"jbod"}
	c.Info = map[string]string{}
	for _, line := range controller[1:] {
		k, v := kv(line, "= ")
		if k == "" {
			continue
		}
		c.Info[k] = v
		switch k {
		case "Controller":
			c.ID = v
		case "PCI Address":
			pciParts := percPciRE.FindStringSubmatch(v)
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

func (s *PercCli) Controllers() []*Controller {
	out, err := s.run("/call", "show", "all")
	if err != nil {
		return nil
	}
	_, controllers := partitionAt(out, percCliCSlotRE)
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

func (s *PercCli) canBeCleared(c *Controller) bool {
	for _, vol := range c.Volumes {
		if vol.Fake {
			continue
		}
		return true
	}
	return false
}

// XXX: This is somewhat implemented - initial testing done on HBA only controller
func (s *PercCli) Clear(c *Controller, onlyForeign bool) error {
	if onlyForeign {
		// as far as I can tell, ssacli has no notion of a foreign config.
		// So if we are asked to clear just the foreign config, do nothing.
		return nil
	}
	if !s.canBeCleared(c) {
		return nil
	}
	out, err := s.run("controller", "slot="+c.ID, "delete", "forced", "override")
	s.log.Printf("GREG: perccli: clear: %v %v\n", out, err)
	return err
}

func (s PercCli) Refresh(c *Controller) {
	lines, err := s.run("/c"+c.ID, "show", "all")
	if err != nil {
		return
	}
	s.fillController(c, lines)
}

func (s *PercCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		parts[i] = fmt.Sprintf("%s:%d", disks[i].Enclosure, disks[i].Slot)
	}
	return fmt.Sprintf("drives=%s", strings.Join(parts, "|"))
}

// XXX: This is somewhat implemented - initial testing done on HBA only controller
func (s *PercCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	cmdLine := []string{
		"add",
		"/c" + c.ID,
		"vd",
		fmt.Sprintf("name=%s", v.Name),
		fmt.Sprintf("Strip=%d", v.stripeSize()>>10),
	}
	switch v.RaidLevel {
	case "jbod":
		if len(v.Disks) == 1 {
			// Controller will automatically expose non-configured drives to the OS
			// So, do nothing.
			s.log.Printf("Controller in mixed, drive already exposed to OS")
			return nil
		}
		// Yes, I know this is wrong for jbod, but ssacli Is Not Helpful.
		cmdLine = append(cmdLine, "r0")
	case "raid0":
		cmdLine = append(cmdLine, "r0")
	case "raid1":
		if len(v.Disks) == 2 {
			cmdLine = append(cmdLine, "r1")
		} else {
			cmdLine = append(cmdLine, "r1adm")
		}
	case "raid5":
		cmdLine = append(cmdLine, "r5")
	case "raid6":
		cmdLine = append(cmdLine, "r6")
	case "raid10":
		cmdLine = append(cmdLine, "r10")
	case "raid50":
		cmdLine = append(cmdLine, "r50")
	case "raid60":
		cmdLine = append(cmdLine, "r60")
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

func (s *PercCli) Encrypt(c *Controller, key, password string) error {
	return fmt.Errorf("Encryption is not currently supported")
}
