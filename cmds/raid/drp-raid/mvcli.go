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

type MVCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
	enabled    bool
}

var (
	mvPdRE       = regexp.MustCompile(`^Adapter:.*([^ ]+)$`)
	mvLdRE       = regexp.MustCompile(`^id:.*([^ ]+)$`)
	mvCliCSlotRE = regexp.MustCompile(`^Adapter ID:.*([^ 	]+)$`)
)

func (s *MVCli) Logger(l *log.Logger) {
	s.log = l
}

func (s *MVCli) Order() int    { return s.order }
func (s *MVCli) Enabled() bool { return s.enabled }
func (s *MVCli) Enable()       { s.enabled = true }
func (s *MVCli) Disable()      { s.enabled = false }

func (s *MVCli) Name() string { return s.name }

func (s *MVCli) Executable() string { return s.executable }

func (s *MVCli) checkLinesForError(lines []string) error {
	if len(lines) > 1 && strings.HasPrefix(lines[1], "Error:") {
		return errors.New(lines[1])
	}
	return nil
}

func (s *MVCli) run(args ...string) ([]string, error) {
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

func (s *MVCli) Useable() bool {
	_, err := s.run("info", "-o", "hba")
	return err == nil
}

func (s *MVCli) fillDisk(d *PhysicalDisk, lines []string) bool {
	d.Info = map[string]string{}
	for _, line := range lines {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		d.Info[k] = v
		switch k {
		case "Adapter":
			if v != d.ControllerID {
				return false
			}
		case "Size":
			d.Size, _ = sizeParser(v + "B")
		case "Type":
			tt := strings.ToLower(v)
			if strings.Contains(tt, "sata") {
				d.Protocol = "sata"
			}
			if strings.Contains(tt, "sas") {
				d.Protocol = "sas"
			}
			if strings.Contains(tt, "nvme") {
				d.Protocol = "nvme"
			}
			if strings.Contains(tt, "scsi") {
				d.Protocol = "scsi"
			}
		case "SSD Type":
			tt := strings.ToLower(v)
			if strings.Contains(tt, "ssd") {
				d.MediaType = "ssd"
			} else {
				d.MediaType = "disk"
			}
		case "PD ID":
			d.Slot, _ = strconv.ParseUint(v, 10, 64)
		}
	}
	d.Status = "Good"
	return true
}

func (s *MVCli) fillVolume(vol *Volume, lines []string) {
	vol.Info = map[string]string{}
	for _, line := range lines {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		vol.Info[k] = v
		switch k {
		case "name":
			vol.Name = v
		case "id":
			vol.ID = v
		case "status":
			vol.Status = v
		case "RAID mode":
			switch v {
			case "RAID1":
				vol.RaidLevel = "raid1"
			default:
				vol.RaidLevel = "raid" + v
			}
		case "size":
			vol.Size, _ = sizeParser(v + "B")
		case "Stripe size":
			vol.StripeSize, _ = sizeParser(v + " KB") // Size is in K
		}
	}
}

func (s *MVCli) fillArray(c *Controller) {
	out, _ := s.run("info", "-o", "pd")
	_, phys := partitionAt(out, mvPdRE)
	disks := make([]*PhysicalDisk, len(phys))
	for i, phy := range phys {
		d := &PhysicalDisk{
			controller:       c,
			ControllerID:     c.ID,
			ControllerDriver: c.Driver,
			driver:           s,
		}
		if addMe := s.fillDisk(d, phy); addMe {
			disks[i] = d
		}
	}
	c.Disks = append(c.Disks, disks...)

	out, _ = s.run("info", "-o", "vd")
	_, lds := partitionAt(out, mvLdRE)
	if len(lds) > 0 {
		for _, ld := range lds {
			vol := &Volume{
				ControllerID:     c.ID,
				ControllerDriver: c.Driver,
				Disks:            disks,
				controller:       c,
				driver:           s,
			}
			s.fillVolume(vol, ld)
			c.Volumes = append(c.Volumes, vol)
		}
	}

	// Make fake jbods
	seenDisks := map[string][]uint64{}
	for _, vol := range c.Volumes {
		for _, vd := range vol.Disks {
			as, ok := seenDisks[vol.ID]
			if !ok {
				as = []uint64{}
			}
			as = append(as, vd.Slot)
			seenDisks[vol.ID] = as
		}
	}
	for vid, vpids := range seenDisks {
		for _, vpid := range vpids {
			for i, d := range c.Disks {
				if d.Slot == vpid {
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

func (s *MVCli) fillController(c *Controller, lines []string) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	controller := lines
	c.JBODCapable = true
	c.RaidCapable = true
	c.AutoJBOD = true
	c.RaidLevels = []string{"raid1", "jbod"}
	c.Info = map[string]string{}
	for _, line := range controller {
		k, v := kv(line, ": ")
		if k == "" {
			continue
		}
		c.Info[k] = v
		switch k {
		case "Adapter ID":
			c.ID = v
		}
	}
	s.fillArray(c)
}

func (s *MVCli) Controllers() []*Controller {
	out, err := s.run("info", "-o", "hba")
	if err != nil {
		return nil
	}
	_, controllers := partitionAt(out, mvCliCSlotRE)
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

func (s *MVCli) canBeCleared(c *Controller) bool {
	for _, vol := range c.Volumes {
		if vol.Fake {
			continue
		}
		return true
	}
	return false
}

func (s *MVCli) Clear(c *Controller, onlyForeign bool) error {
	if onlyForeign {
		// as far as I can tell, mvcli has no notion of a foreign config.
		// So if we are asked to clear just the foreign config, do nothing.
		return nil
	}
	if !s.canBeCleared(c) {
		return nil
	}
	_, err := s.run("delete", "-o", "vd", "-i", "0", "-f", "--waiveconfirmation")
	return err
}

func (s MVCli) Refresh(c *Controller) {
	lines, err := s.run("info", "-o", "hba", "-i", c.ID)
	if err != nil {
		return
	}
	s.fillController(c, lines)
}

func (s *MVCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		parts[i] = fmt.Sprintf("%d", disks[i].Slot)
	}
	return fmt.Sprintf("%s", strings.Join(parts, ","))
}

func (s *MVCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	cmdLine := []string{
		"create",
		"-o",
		"vd",
		"--waiveconfirmation",
		"-b",
		fmt.Sprintf("%d", v.stripeSize()>>10),
	}
	switch v.RaidLevel {
	case "jbod":
		if len(v.Disks) == 1 {
			s.log.Printf("Controller always puts drives as volumes")
			return nil
		}
		return fmt.Errorf("Cannot create multi-drive jbod")
	case "raid0":
		cmdLine = append(cmdLine, "-r0")
	case "raid1":
		if len(v.Disks) == 2 {
			cmdLine = append(cmdLine, "-r1")
		} else {
			return fmt.Errorf("Cannot create more than 2 drive raid1")
		}
	case "raid5":
		cmdLine = append(cmdLine, "-r5")
	case "raid10":
		cmdLine = append(cmdLine, "-r10")
	case "raid1e":
		cmdLine = append(cmdLine, "-r1e")
	default:
		return fmt.Errorf("Raid level %s not supported", v.RaidLevel)
	}
	if v.Name != "" {
		cmdLine = append(cmdLine, "-n")
		cmdLine = append(cmdLine, v.Name)
	}
	cmdLine = append(cmdLine, "-d", s.diskList(v.Disks))
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

func (s *MVCli) Encrypt(c *Controller, key, password string) error {
	return fmt.Errorf("Encryption not supported")
}
