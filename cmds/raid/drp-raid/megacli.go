package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	mcliVolRE     = regexp.MustCompile(`Virtual Drive:\s*(\d+)\s*\(Target Id:\s*(\d+)\)`)
	mcliDiskRE    = regexp.MustCompile(`PD:\s*(\d+)\s*Information`)
	mcliEnclLine  = regexp.MustCompile(`^Enclosure Device ID: (.+)`)
	mcliAdaptRE   = regexp.MustCompile(`^Adapter #(\d+)$`)
	mcliCountRE   = regexp.MustCompile(`Controller Count:.*([0-9]+)\.`)
	mcliRaidLvlRE = regexp.MustCompile(`Primary-(\d+), Secondary-(\d+)`)
)

type MegaCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
}

func (m *MegaCli) Logger(l *log.Logger) {
	m.log = l
}

func (m *MegaCli) Order() int { return m.order }

func (m *MegaCli) Name() string { return m.name }

func (m *MegaCli) Executable() string { return m.executable }

func (m *MegaCli) run(args ...string) (out []string, err string, cmdError error) {
	if fake {
		return []string{}, "", nil
	}
	cmd := exec.Command(m.executable, args...)
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = outBuf, errBuf
	cmdError = cmd.Run()
	return strings.Split(outBuf.String(), "\n"), errBuf.String(), cmdError
}

func (m *MegaCli) Useable() bool {
	out, _, err := m.run("-adpCount")
	_, lines := partitionAt(out, mcliCountRE)
	if len(lines) == 0 {
		return false
	}
	// No checking the exit status, just parse the out.
	matches := mcliCountRE.FindStringSubmatch(lines[0][0])
	if len(matches) < 2 {
		return false
	}
	n, err := strconv.ParseUint(matches[1], 10, 64)
	return err == nil && n > 0
}

func (m *MegaCli) fillVolume(volume *Volume, section []string) {
	for _, line := range section {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, ok := volume.Info[k]; ok {
			continue
		}
		if k == "Span" {
			continue
		}
		volume.Info[k] = v
		switch k {
		case "Virtual Drive":
			matches := mcliVolRE.FindStringSubmatch(line)
			volume.ID = matches[1]
			volume.Name = matches[2]
		case "Name":
			volume.Name = v
		case "State":
			volume.Status = v
		case "Span Depth":
			volume.Spans, _ = strconv.ParseUint(v, 10, 64)
		case "Number Of Drives", "Number Of Drives per span":
			volume.SpanLength, _ = strconv.ParseUint(v, 10, 64)
		case "Size":
			vs, err := sizeParser(v)
			if err != nil {
				m.log.Fatalf("megacli returned a non-parseable Size %s: %v", v, err)
			}
			volume.Size = vs
		case "Strip Size":
			ss, err := sizeParser(v)
			if err != nil {
				m.log.Fatalf("megacli returned a non-parseable stripe size %s: %v", v, err)
			}
			volume.StripeSize = ss
		case "RAID Level":
			matches := mcliRaidLvlRE.FindStringSubmatch(v)
			switch matches[1] + "," + matches[2] {
			case "0,0":
				volume.RaidLevel = "raid0"
			case "1,0":
				volume.RaidLevel = "raid1"
			case "3,0":
				volume.RaidLevel = "raid3"
			case "4,0":
				volume.RaidLevel = "raid4"
			case "5,0":
				volume.RaidLevel = "raid5"
			case "6,0":
				volume.RaidLevel = "raid6"
			case "7,0":
				volume.RaidLevel = "raid7"
			case "15,0":
				volume.RaidLevel = "jbod"
			case "17,0":
				volume.RaidLevel = "raid1e"
			case "31,0":
				volume.RaidLevel = "concat"
			case "21,0":
				volume.RaidLevel = "raid5e"
			case "37,0":
				volume.RaidLevel = "raid5ee"
			case "53,0":
				volume.RaidLevel = "raid5r"
			case "0,3":
				volume.RaidLevel = "raid00"
			case "1,3":
				volume.RaidLevel = "raid10"
			case "5,3":
				volume.RaidLevel = "raid50"
			case "6,3":
				volume.RaidLevel = "raid60"
			default:
				volume.RaidLevel = "unknown"
			}
		}
	}
}

func (m *MegaCli) fillVolumes(c *Controller) {
	out, _, _ := m.run("-ldpdInfo", "-a"+c.ID)
	_, sections := partitionAt(out, mcliVolRE)
	res := make([]*Volume, len(sections))
	for i, section := range sections {
		volPart, diskSections := partitionAt(section, mcliEnclLine)
		volume := &Volume{
			ControllerID:     c.ID,
			ControllerDriver: m.name,
			controller:       c,
			driver:           m,
			Info:             map[string]string{},
		}
		m.fillVolume(volume, volPart)
		res[i] = volume
		disks := make([]*PhysicalDisk, len(diskSections))
		for j, diskSection := range diskSections {
			disk := &PhysicalDisk{
				ControllerID:     c.ID,
				ControllerDriver: c.Driver,
				VolumeID:         volume.ID,
				controller:       c,
				volume:           volume,
				driver:           m,
				Info:             map[string]string{},
			}
			disks[j] = disk
			m.fillDisk(disk, diskSection)
		}
		volume.Disks = disks
	}
	c.Volumes = append(c.Volumes, res...)
}

func (m *MegaCli) finalizeDisk(d *PhysicalDisk) {
	d.ControllerID = d.controller.ID
	d.ControllerDriver = d.controller.Driver
	for k, v := range d.Info {
		switch k {
		case "Enclosure Device ID":
			if v != `N/A` {
				d.Enclosure = v
			}
		case "Slot Number":
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				m.log.Panic(err)
			}
			d.Slot = n
		case "PD Type":
			d.Protocol = strings.ToLower(v)
		case "Media Type":
			switch v {
			case "Hard Disk Device":
				d.MediaType = "disk"
			case "Solid State Device", "Nytro SFM Device":
				d.MediaType = "ssd"
			default:
				d.MediaType = v
			}
		case "Logical Sector Size":
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				m.log.Panic(err)
			}
			if n == 0 {
				n = 512
			}
			d.LogicalSectorSize = n
		case "Physical Sector Size":
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				m.log.Panic(err)
			}
			if n == 0 {
				n = 512
			}
			d.PhysicalSectorSize = n
		case "Coerced Size":
			sectorRE := regexp.MustCompile(`0x([0-9a-f]+) Sectors`)
			sectorMatches := sectorRE.FindStringSubmatch(v)
			if len(sectorMatches) != 2 {
				m.log.Panicf("COuld not find number of disk sectors")
			}
			n, err := strconv.ParseUint(sectorMatches[1], 16, 64)
			if err != nil {
				m.log.Panicf("Could not parse number of disk sectors")
			}
			sz, err := sizeParser(v)
			if err != nil {
				m.log.Fatalf("megacli: could not parse disk size %s: %v", v, err)
			}
			d.SectorCount = n
			d.Size = sz
		case "Firmware state":
			d.JBOD = v == "JBOD"
			d.Status = v
		}
	}
	/* Guesstimate size time!
	for sz := range []int64{512, 4096} {
		testSize := int64(sz) * d.SectorCount
		if testSize < d.Size<<1 && testSize > d.Size>>1 {
			d.Size = testSize
			d.LogicalSectorSize = int64(sz)
			break
		}
	}
	*/
}

func (m *MegaCli) fillDisk(d *PhysicalDisk, section []string) {
	for _, line := range section {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, ok := d.Info[k]; ok {
			continue
		}
		d.Info[k] = v
	}
	m.finalizeDisk(d)
}

func (m *MegaCli) fillDisks(c *Controller) {
	out, _, _ := m.run("-PDList", "-a"+c.ID)
	_, sections := partitionAt(out, mcliEnclLine)
	res := make([]*PhysicalDisk, len(sections))
	for i, section := range sections {
		d := &PhysicalDisk{
			Info:       map[string]string{},
			controller: c,
			driver:     m,
		}
		m.fillDisk(d, section)
		if d.JBOD {
			c.addJBODVolume(d)
		}
		res[i] = d
	}
	c.Disks = append(c.Disks, res...)
}

func (m *MegaCli) finalizeController(c *Controller) {
	stdOut, _, _ := m.run("-AdpGetPciInfo", "-a"+c.ID)
	for _, line := range stdOut {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		n, err := strconv.ParseUint(v, 16, 64)
		if err != nil {
			continue
		}
		switch k {
		case "Bus Number":
			c.PCI.Bus = int64(n)
		case "Device Number":
			c.PCI.Device = int64(n)
		case "Function Number":
			c.PCI.Function = int64(n)
		}
	}
	for k, v := range c.Info {

		switch k {
		case "Enable JBOD":
			c.JBODCapable = strings.ToLower(v) == "yes"
		case "RAID Level Supported":
			c.RaidCapable = true
			c.RaidLevels = []string{}
			for _, raidLevel := range strings.Split(v, ",") {
				raidLevel = strings.ToLower(strings.TrimSpace(raidLevel))
				if _, ok := raidLevels[raidLevel]; ok {
					c.RaidLevels = append(c.RaidLevels, raidLevel)
				}
			}
		}
	}
}

func (m *MegaCli) fillController(c *Controller, section []string) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	for _, line := range section {
		if mcliAdaptRE.MatchString(line) {
			matches := mcliAdaptRE.FindStringSubmatch(line)
			c.ID = matches[1]
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 || strings.HasPrefix(parts[1], " ") {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, ok := c.Info[key]; ok {
			continue
		}
		c.Info[key] = val
	}
	m.finalizeController(c)
	m.fillDisks(c)
	m.fillVolumes(c)
}

func (m *MegaCli) Controllers() []*Controller {
	out, _, err := m.run("-AdpAllinfo", "-aAll")
	if err != nil {
		return nil
	}
	_, sections := partitionAt(out, mcliAdaptRE)
	res := make([]*Controller, len(sections))
	for i, section := range sections {
		c := &Controller{
			Info:   map[string]string{},
			Driver: m.name,
			driver: m,
		}
		m.fillController(c, section)
		res[i] = c
	}
	return res
}

func (m *MegaCli) Refresh(c *Controller) {
	out, _, err := m.run("-AdpAllInfo", "-a"+c.ID)
	if err != nil {
		return
	}
	m.fillController(c, out)
}

func (m *MegaCli) Clear(c *Controller, onlyForeign bool) error {
	out, outErr, err := m.run("-CfgForeign", "-Clear", "-a"+c.ID)
	if !onlyForeign {
		for _, v := range c.Volumes {
			if v.RaidLevel == "jbod" {
				_, outErr, err = m.run("-PDMakeGood", "PhysDrv", fmt.Sprintf(`[%s:%d]`, v.Disks[0].Enclosure, v.Disks[0].Slot), "-Force", "-a"+c.ID)
				if err != nil {
					return fmt.Errorf("Error %s:\n%s", err, outErr)
				}
			}
		}
		out, outErr, err = m.run("-CfgClr", "-Force", "-a"+c.ID)
		if err != nil {
			return fmt.Errorf("Error %s:\n%s", err, outErr)
		}
	}
	m.fillController(c, out)
	return nil
}

func (m *MegaCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		parts[i] = fmt.Sprintf("%s:%d", disks[i].Enclosure, disks[i].Slot)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}

func (m *MegaCli) forceGood(disks []VolSpecDisk) []string {
	return []string{"-PDMakeGood", "-PhysDrv", m.diskList(disks), "-Force"}
}

func (m *MegaCli) jbodDisks(v *VolSpec) []VolSpecDisk {
	jDisks := []VolSpecDisk{}
	for _, disk := range v.Disks {
		if disk.info["Firmware state"] == "JBOD" {
			jDisks = append(jDisks, disk)
		}
	}
	return jDisks
}

func (m *MegaCli) hasJBOD(v *VolSpec) bool {
	for _, disk := range v.Disks {
		if disk.info["Firmware state"] == "JBOD" {
			return true
		}
	}
	return false
}

func (m *MegaCli) createSimple(v *VolSpec) []string {
	res := []string{"-CfgLdAdd", "-r", m.diskList(v.Disks), "WB", "RA", "-strpsz", "-Force"}
	res[5] = fmt.Sprintf("-strpsz%d", v.stripeSize()>>10)
	switch v.RaidLevel {
	case "raid0":
		res[1] = "-r0"
	case "raid1":
		res[1] = "-r1"
	case "raid5":
		res[1] = "-r5"
	case "raid6":
		res[1] = "-r6"
	}
	return res
}

func (m *MegaCli) createSpanned(v *VolSpec) []string {
	lvl := v.raid()
	spans := lvl.Partition(v.Disks)
	cmd := []string{"-CfgSpanAdd", "-r"}
	for span, spanDisks := range spans {
		cmd = append(cmd, fmt.Sprintf("-Array%d%s", span, m.diskList(spanDisks)))
	}
	cmd = append(cmd, "WB", "RA", fmt.Sprintf("-strpsz%d", v.stripeSize()>>10), "-Force")
	switch v.RaidLevel {
	case "raid00":
		cmd[1] = "-r00"
	case "raid10":
		cmd[1] = "-r10"
	case "raid50":
		cmd[1] = "-r50"
	case "raid60":
		cmd[1] = "-r60"
	}
	return cmd
}

func (m *MegaCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	cmds := [][]string{}
	if forceGood {
		cmds = append(cmds, m.forceGood(v.Disks))
	}
	switch v.RaidLevel {
	case "jbod":
		if m.hasJBOD(v) {
			m.log.Printf("%s is already a JBOD, nothing to do", m.diskList(v.Disks))
			return nil
		}
		cmds = append(cmds, []string{"-PDMakeJBOD", "-PhysDrv", m.diskList(v.Disks)})
	case "raid0", "raid1", "raid5", "raid6":
		if m.hasJBOD(v) {
			cmds = append(cmds, m.forceGood(m.jbodDisks(v)))
		}
		cmds = append(cmds, m.createSimple(v))
	case "raid00", "raid10", "raid50", "raid60":
		if m.hasJBOD(v) {
			cmds = append(cmds, m.forceGood(m.jbodDisks(v)))
		}
		cmds = append(cmds, m.createSpanned(v))
	default:
		return fmt.Errorf("Cannot create a %s volume", v.RaidLevel)
	}
	if v.Encrypt {
		ecmd := []string{
			"-LDMakeSecure",
		}
		ecmd = append(ecmd, fmt.Sprintf("-L%d", v.index))
		cmds = append(cmds, ecmd)
	}
	var (
		out    []string
		outErr string
		err    error
	)
	for _, cmd := range cmds {
		cmd = append(cmd, "-a"+c.ID)
		m.log.Printf("Running command: %s %s", m.executable, strings.Join(cmd, " "))
		out, outErr, err = m.run(cmd...)
		if err == nil {
			continue
		}
		return fmt.Errorf("Error running cmd `%s`: %v\n%s", strings.Join(cmd, " "), err, outErr)
	}
	m.log.Println(strings.Join(out, "\n"))
	return nil
}

func (m *MegaCli) Encrypt(c *Controller, key, password string) error {
	passwordParam := "-SecurityKey"
	if m.Name() == "megacli" {
		passwordParam = "-Passphrase"
	}
	cmds := [][]string{
		[]string{"-DeleteSecurityKey", "-Force"},
		[]string{"-CreateSecurityKey", passwordParam, password, "-KeyID", key},
	}
	var (
		out    []string
		outErr string
		err    error
	)
	for i, cmd := range cmds {
		cmd = append(cmd, "-a"+c.ID)
		m.log.Printf("Running command: %s %s", m.executable, strings.Join(cmd, " "))
		out, outErr, err = m.run(cmd...)
		if i == 0 || err == nil {
			continue
		}
		return fmt.Errorf("Error running cmd `%s`: %v\n%v\n%s", strings.Join(cmd, " "), err, out, outErr)
	}
	m.log.Println(strings.Join(out, "\n"))
	return nil
}
