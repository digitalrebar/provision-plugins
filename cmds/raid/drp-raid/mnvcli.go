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

type MNVCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
	enabled    bool
}

var (
	mnvPdRE       = regexp.MustCompile(`^PD ID:.*([^ ]+)$`)
	mnvLdRE       = regexp.MustCompile(`^VD ID:.*([^ ]+)$`)
	mnvCliCSlotRE = regexp.MustCompile(`^NVMe Controller ID.*([^ 	]+)$`)
)

func (s *MNVCli) Logger(l *log.Logger) {
	s.log = l
}

func (s *MNVCli) Order() int    { return s.order }
func (s *MNVCli) Enabled() bool { return s.enabled }
func (s *MNVCli) Enable()       { s.enabled = true }
func (s *MNVCli) Disable()      { s.enabled = false }

func (s *MNVCli) Name() string { return s.name }

func (s *MNVCli) Executable() string { return s.executable }

func (s *MNVCli) checkLinesForError(lines []string) error {
	if len(lines) > 1 && strings.HasPrefix(lines[1], "Error:") {
		return errors.New(lines[1])
	}
	return nil
}

func (s *MNVCli) run(args ...string) ([]string, error) {
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

func (s *MNVCli) Useable() bool {
	_, err := s.run("info", "-o", "hba")
	return err == nil
}

/*
PD ID:                           0
Model:                           MZ1LB960HAJQ-000V7
Serial:                          S50NNA0NA00240
Sector Size:                     512 bytes
LBA:                             1875385008
Size:                            894 GB
SSD backend RC/Slot ID:          0
SSD backend Namespace ID:        1
Firmware version:                CR30
Status:                          Idle
Assigned:                        Yes
SMART Critical Warning:          No
*/
func (s *MNVCli) fillDisk(d *PhysicalDisk, lines []string) bool {
	d.Info = map[string]string{}
	d.Protocol = "nvme"
	d.MediaType = "ssd"
	for _, line := range lines {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		d.Info[k] = v
		switch k {
		case "Size":
			d.Size, _ = sizeParser(v + "B")
		case "PD ID":
			d.Slot, _ = strconv.ParseUint(v, 10, 64)
		}
	}
	d.Status = "Good"
	return true
}

/*
VD ID:               0
Name:                VD_0
Status:              Functional
Importable:          No
RAID Mode:           RAID1
size:                894 GB
PD Count:            2
PDs:                 0 1
Stripe Block Size:   128K
Sector Size:         512 bytes

Total # of VD:       1
*/
func (s *MNVCli) fillVolume(vol *Volume, lines []string) {
	vol.Info = map[string]string{}
	for _, line := range lines {
		k, v := kv(line, ": ")
		if k == "" {
			break
		}
		vol.Info[k] = v
		switch k {
		case "Name":
			vol.Name = v
		case "VD ID":
			vol.ID = v
		case "Status":
			vol.Status = v
		case "RAID Mode":
			switch v {
			case "RAID1":
				vol.RaidLevel = "raid1"
			default:
				vol.RaidLevel = "raid" + v
			}
		case "size":
			vol.Size, _ = sizeParser(v)
		case "Stripe size":
			vol.StripeSize, _ = sizeParser(v) // Size is in K
		}
	}
}

func (s *MNVCli) fillArray(c *Controller) {
	out, _ := s.run("info", "-o", "pd")
	_, phys := partitionAt(out, mnvPdRE)
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
	_, lds := partitionAt(out, mnvLdRE)
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

func (s *MNVCli) fillController(c *Controller, lines []string) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	c.JBODCapable = true
	c.RaidCapable = true
	c.AutoJBOD = true
	c.RaidLevels = []string{"raid1", "jbod"}
	c.Info = map[string]string{}
	if strings.HasPrefix(lines[0], "NVMe Controller ID") {
		c.ID = strings.TrimSpace(lines[0][18:])
	}
	for _, line := range lines {
		k, v := kv(line, ": ")
		if k == "" {
			continue
		}
		c.Info[k] = v
		switch k {
		case "NVMe Controller ID":
			c.ID = v
		}
	}
	s.fillArray(c)
}

func (s *MNVCli) Controllers() []*Controller {
	out, err := s.run("info", "-o", "hba")
	if err != nil {
		return nil
	}
	_, controllers := partitionAt(out, mnvCliCSlotRE)
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

func (s *MNVCli) canBeCleared(c *Controller) bool {
	for _, vol := range c.Volumes {
		if vol.Fake {
			continue
		}
		return true
	}
	return false
}

func (s *MNVCli) Clear(c *Controller, onlyForeign bool) error {
	if onlyForeign {
		// as far as I can tell, mnvcli has no notion of a foreign config.
		// So if we are asked to clear just the foreign config, do nothing.
		return nil
	}
	if !s.canBeCleared(c) {
		return nil
	}
	_, err := s.run("delete", "-o", "vd", "-i", "0", "--waiveconfirmation")
	return err
}

func (s MNVCli) Refresh(c *Controller) {
	lines, err := s.run("info", "-o", "hba", "-i", c.ID)
	if err != nil {
		return
	}
	s.fillController(c, lines)
}

func (s *MNVCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		parts[i] = fmt.Sprintf("%d", disks[i].Slot)
	}
	return fmt.Sprintf("%s", strings.Join(parts, ","))
}

func (s *MNVCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	sSize := v.stripeSize() >> 10
	if sSize < 128 {
		sSize = 128
	}
	cmdLine := []string{
		"create",
		"-b",
		fmt.Sprintf("%d", sSize),
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

func (s *MNVCli) Encrypt(c *Controller, key, password string) error {
	return fmt.Errorf("Encryption not supported")
}
