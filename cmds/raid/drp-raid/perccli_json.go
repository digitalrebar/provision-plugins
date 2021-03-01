package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictorLowther/jsonpatch2/utils"
)

/*
 * PercCli is for an HBA adapter system that kinda works, but doesn't handle general perccli support
 *
 * This attempts to extend full perccli support using the json interface.
 */

type PercJsonCli struct {
	name       string
	executable string
	order      int
	log        *log.Logger
	enabled    bool
}

func (s *PercJsonCli) Logger(l *log.Logger) {
	s.log = l
}

func (s *PercJsonCli) Order() int    { return s.order }
func (s *PercJsonCli) Enabled() bool { return s.enabled }
func (s *PercJsonCli) Enable()       { s.enabled = true }
func (s *PercJsonCli) Disable()      { s.enabled = false }

func (s *PercJsonCli) Name() string { return s.name }

func (s *PercJsonCli) Executable() string { return s.executable }

func (s *PercJsonCli) checkLinesForError(lines []string) error {
	if len(lines) > 1 && strings.HasPrefix(lines[1], "Error:") {
		return errors.New(lines[1])
	}
	return nil
}

func (s *PercJsonCli) convertToKV(raw interface{}, data map[string]string) map[string]string {
	tmp := map[string]interface{}{}
	rerr := utils.Remarshal(raw, &tmp)
	if rerr != nil {
		s.log.Printf("Failed to remarshal: %v\n", rerr)
		return data
	}
	for k, v := range tmp {
		switch v := v.(type) {
		case bool:
			data[k] = fmt.Sprintf("%v", v)
		case int, uint, uint8, int8, uint16, int16, uint32, int32, uint64, int64:
			data[k] = fmt.Sprintf("%d", v)
		case float32, float64:
			data[k] = fmt.Sprintf("%.0f", v)
		case string:
			data[k] = v
		case []interface{}:
			// Nothing to do
		case map[string]interface{}:
			data = s.convertToKV(v, data)
		default:
			s.log.Printf("I don't know about type for %s %T!\n", k, v)
		}
	}
	return data
}

func (s *PercJsonCli) run(args ...string) (string, error) {
	if fake {
		return "", nil
	}
	cmd := exec.Command(s.executable, args...)
	outBuf := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = outBuf, outBuf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	out := strings.Split(outBuf.String(), "\n")
	return outBuf.String(), s.checkLinesForError(out)
}

func (s *PercJsonCli) Useable() bool {
	_, err := s.run("/call", "show")
	return err == nil
}

func (s *PercJsonCli) fillDisk(d *PhysicalDisk, phy *PercJsonPhysicalDiskCntr) {
	d.Enclosure = strings.Split(phy.EidSlt, ":")[0]
	if d.Enclosure == " " {
		d.Enclosure = ""
	}
	d.Slot, _ = strconv.ParseUint(strings.Split(phy.EidSlt, ":")[1], 10, 64)
	d.Status = phy.State
	d.Protocol = strings.ToLower(phy.Intf)
	d.MediaType = strings.ToLower(phy.Med)
	if d.MediaType == "hdd" {
		d.MediaType = "disk"
	}
	d.Size, _ = sizeParser(phy.Size)

	d.Info = map[string]string{}
	s.convertToKV(phy, d.Info)

	// Get more detailed data
	path := fmt.Sprintf("/c%d", d.controller.idx)
	if d.Enclosure != "" {
		path += fmt.Sprintf("/e%s", d.Enclosure)
	}
	path += fmt.Sprintf("/s%d", d.Slot)
	if out, err := s.run(path, "show", "all", "J"); err == nil {
		c := &struct {
			Controllers []*PercJsonCommand
		}{}
		if err := json.Unmarshal([]byte(out), c); err != nil {
			s.log.Printf("Failed to process json for disk %s: %v", path, err)
			return
		}

		s.convertToKV(c.Controllers[0].ResponseData, d.Info)
		for k, v := range d.Info {
			switch strings.ToLower(k) {
			case "physical sector size":
				d.PhysicalSectorSize, _ = sizeParser(v)
			case "logical sector size":
				d.LogicalSectorSize, _ = sizeParser(v)
			case "coerced size":
				sectorRE := regexp.MustCompile(`0x([0-9a-f]+) Sectors`)
				sectorMatches := sectorRE.FindStringSubmatch(v)
				if len(sectorMatches) != 2 {
					s.log.Panicf("COuld not find number of disk sectors")
				}
				n, err := strconv.ParseUint(sectorMatches[1], 16, 64)
				if err != nil {
					s.log.Panicf("Could not parse number of disk sectors")
				}
				sz, err := sizeParser(v)
				if err != nil {
					s.log.Fatalf("megacli: could not parse disk size %s: %v", v, err)
				}
				d.SectorCount = n
				d.Size = sz
			}
		}
	} else {
		s.log.Printf("Failed to query for %s: %v\n", path, err)
	}
}

func (s *PercJsonCli) fillVolume(vol *Volume, ld *PercJsonVolumeCntr) {
	vol.Info = map[string]string{}
	s.convertToKV(ld, vol.Info)

	vol.Name = ld.Name
	vol.ID = strings.Split(ld.DGVD, "/")[1]
	vol.Status = ld.State
	switch ld.TYPE {
	case "RAID0":
		vol.RaidLevel = "raid0"
	case "RAID1", "RAID1ADM":
		vol.RaidLevel = "raid1"
	case "RAID5":
		vol.RaidLevel = "raid5"
	case "RAID6":
		vol.RaidLevel = "raid6"
	case "RAID1+0", "RAID10":
		vol.RaidLevel = "raid10"
	case "RAID1+0ADM":
		vol.RaidLevel = "raid10"
	case "RAID50":
		vol.RaidLevel = "raid50"
	case "RAID60":
		vol.RaidLevel = "raid60"
	default:
		vol.RaidLevel = ld.TYPE
	}
	vol.Size, _ = sizeParser(ld.Size)

	path := fmt.Sprintf("/c%d", vol.controller.idx)
	path += fmt.Sprintf("/v%d", vol.idx)
	if out, err := s.run(path, "show", "all", "J"); err == nil {
		c := &struct {
			Controllers []*PercJsonCommand
		}{}
		if err := json.Unmarshal([]byte(out), c); err != nil {
			s.log.Printf("Failed to process json for disk %s: %v", path, err)
			return
		}
		s.convertToKV(c.Controllers[0].ResponseData, vol.Info)
		for k, v := range vol.Info {
			switch strings.ToLower(k) {
			case "name":
				vol.Name = v
			case "span depth":
				vol.Spans, _ = strconv.ParseUint(v, 10, 64)
			case "number of drives", "number of drives per span":
				vol.SpanLength, _ = strconv.ParseUint(v, 10, 64)
			case "size":
				vs, err := sizeParser(v)
				if err != nil {
					s.log.Fatalf("megacli returned a non-parseable Size %s: %v", v, err)
				}
				vol.Size = vs
			case "strip size":
				ss, err := sizeParser(v)
				if err != nil {
					s.log.Fatalf("megacli returned a non-parseable stripe size %s: %v", v, err)
				}
				vol.StripeSize = ss
			}
		}

		tmp := map[string]interface{}{}
		utils.Remarshal(c.Controllers[0].ResponseData, &tmp)

		tmp2 := []interface{}{}
		utils.Remarshal(tmp[fmt.Sprintf("PDs for VD %s", vol.ID)], &tmp2)

		for _, d := range tmp2 {
			newdata := map[string]string{}
			newdata = s.convertToKV(d, newdata)

			for k, v := range newdata {
				if k == "EID:Slt" {
					parts := strings.Split(v, ":")
					enc := parts[0]
					if enc == " " {
						enc = ""
					}
					slot, _ := strconv.ParseUint(parts[1], 10, 64)

					for _, disk := range vol.controller.Disks {
						if disk.Enclosure == enc && disk.Slot == slot {
							vol.Disks = append(vol.Disks, disk)
							disk.VolumeID = vol.ID
							disk.volume = vol
							break
						}
					}
					break
				}
			}
		}
	} else {
		s.log.Printf("Failed to query for %s: %v\n", path, err)
	}
}

func (s *PercJsonCli) fillArray(c *Controller, rs *PercJsonController) {
	disks := make([]*PhysicalDisk, len(rs.PDList))
	for i, phy := range rs.PDList {
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
	if len(rs.VDList) > 0 {
		for idx, ld := range rs.VDList {
			vol := &Volume{
				ControllerID:     c.ID,
				ControllerDriver: c.Driver,
				controller:       c,
				driver:           s,
				idx:              idx,
			}
			s.fillVolume(vol, ld)
			c.Volumes = append(c.Volumes, vol)
		}
	}

	// Make fake jbods
	if c.AutoJBOD {
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
}

func (s *PercJsonCli) fillController(c *Controller, rs *PercJsonController) {
	c.Disks = []*PhysicalDisk{}
	c.Volumes = []*Volume{}
	c.PCI.Bus = int64(rs.Bus.BusNumber)
	c.PCI.Device = int64(rs.Bus.DeviceNumber)
	c.PCI.Function = int64(rs.Bus.FunctionNumber)
	c.ID = fmt.Sprintf("%d", rs.Basics.Controller)

	c.RaidLevels = []string{}
	c.AutoJBOD = false

	c.Info = map[string]string{}
	s.convertToKV(rs, c.Info)

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

	s.fillArray(c, rs)
}

type PercJsonCommandStatus struct {
	CliVersion      string `json:"CLI Version"`
	Controller      int
	Description     string
	OperatingSystem string `json:"Operating system"`
	Status          string
}

type PercJsonCommand struct {
	CommandStatus PercJsonCommandStatus `json:"Command Status"`
	ResponseData  interface{}           `json:"Response Data"`
}

type PercJsonControllerBasics struct {
	Controller   int
	Model        string
	PCIAddress   string `json:"PCI Address"`
	RevisionNo   string `json:"Revision No"`
	ReworkData   string `json:"Rework Date"`
	SASAddress   string `json:"SAS Address"`
	SerialNumber string `json:"Serial Number"`
}

type PercJsonControllerBus struct {
	BusNumber       int    `json:"Bus number"`
	DeviceId        int    `json:"Device Id"`
	DeviceInterface string `json:"Device Interface"`
	DeviceNumber    int    `json:"Device Number"`
	DomainID        int    `json:"Domain Id"`
	FunctionNumber  int    `json:"Function Number"`
	HostInterface   string `json:"Host Interface"`
	SubDeviceId     int    `json:"SubDevice Id"`
	SubVendorId     int    `json:"SubVendor Id"`
	VendorId        int    `json:"Vendor Id"`
}

type PercJsonPhysicalDiskCntr struct {
	DG     int    `json:"DG"`
	DID    int    `json:"DID"`
	EidSlt string `json:"EID:Slt"`
	Intf   string
	Med    string
	Model  string
	PI     string
	SED    string
	SeSz   string
	Size   string
	Sp     string
	State  string
	Type   string
}

type PercJsonVolumeCntr struct {
	Access  string
	Cac     string
	Cache   string
	Consist string
	DGVD    string `json:"DG/VD"`
	Name    string
	Size    string
	State   string
	TYPE    string
	SCC     string `json:"sCC"`
}

type PercJsonController struct {
	Basics       PercJsonControllerBasics
	Bus          PercJsonControllerBus
	Capabilities map[string]interface{}
	Defaults     map[string]interface{}

	PDList []*PercJsonPhysicalDiskCntr `json:"PD List"`
	VDList []*PercJsonVolumeCntr       `json:"VD List"`

	Version map[string]string
}

func (s *PercJsonCli) Controllers() []*Controller {
	out, err := s.run("/call", "show", "all", "J")
	if err != nil {
		return nil
	}

	c := &struct {
		Controllers []*PercJsonCommand
	}{}
	if err := json.Unmarshal([]byte(out), c); err != nil {
		s.log.Printf("Failed to process json: %v", err)
		return nil
	}

	res := make([]*Controller, len(c.Controllers))
	for i, cmd := range c.Controllers {
		res[i] = &Controller{
			Driver: s.name,
			driver: s,
			idx:    i,
		}

		controller := &PercJsonController{}
		utils.Remarshal(cmd.ResponseData, controller)

		s.fillController(res[i], controller)
	}
	return res
}

func (s *PercJsonCli) canBeCleared(c *Controller) bool {
	for _, vol := range c.Volumes {
		if vol.Fake {
			continue
		}
		return true
	}
	return false
}

func (s *PercJsonCli) Clear(c *Controller, onlyForeign bool) error {
	if onlyForeign {
		// as far as I can tell, ssacli has no notion of a foreign config.
		// So if we are asked to clear just the foreign config, do nothing.
		return nil
	}
	if !s.canBeCleared(c) {
		return nil
	}
	_, err := s.run("/c"+c.ID+"/vall", "del", "force", "J")
	return err
}

func (s PercJsonCli) Refresh(c *Controller) {
	out, err := s.run("/c"+c.ID, "show", "all", "J")
	if err != nil {
		return
	}
	cc := &struct {
		Controllers []*PercJsonController
	}{}
	if err := json.Unmarshal([]byte(out), cc); err != nil {
		s.log.Printf("Failed to process json: %v", err)
		return
	}
	s.fillController(c, cc.Controllers[0])
}

func (s *PercJsonCli) diskList(disks []VolSpecDisk) string {
	parts := make([]string, len(disks))
	for i := range disks {
		e := ""
		if disks[i].Enclosure != "" {
			e = disks[i].Enclosure + ":"
		}
		parts[i] = fmt.Sprintf("%s%d", e, disks[i].Slot)
	}
	return fmt.Sprintf("drives=%s", strings.Join(parts, ","))
}

func (s *PercJsonCli) Create(c *Controller, v *VolSpec, forceGood bool) error {
	if !v.compiled {
		return fmt.Errorf("Cannot create a VolSpec that has not been compiled")
	}
	cmdLine := []string{
		"/c" + c.ID,
		"add",
		"vd",
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
		cmdLine = append(cmdLine, "r1")
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
	cmdLine = append(cmdLine, s.diskList(v.Disks))
	if v.Name != "" {
		cmdLine = append(cmdLine, fmt.Sprintf("name=\"%s\"", v.Name))
	}
	cmdLine = append(cmdLine, fmt.Sprintf("strip=%d", v.stripeSize()>>10))
	cmdLine = append(cmdLine, "force", "J")
	s.log.Printf("Running %s %s", s.executable, strings.Join(cmdLine, " "))
	res, err := s.run(cmdLine...)
	if res != "" {
		s.log.Println(res)
	}
	if err != nil {
		s.log.Printf("Error running command: %s", strings.Join(cmdLine, " "))
	}
	return err
}

func (s *PercJsonCli) Encrypt(c *Controller, key, password string) error {
	cmds := [][]string{
		[]string{"delete", "securitykey"},
		[]string{"set", fmt.Sprintf("securitykey=%s", password), fmt.Sprintf("keyid=%s", key)},
	}
	var (
		out string
		err error
	)
	for i, cmd := range cmds {
		cmd = append(cmd, "/c"+c.ID)
		s.log.Printf("Running command: %s %s", s.executable, strings.Join(cmd, " "))
		out, err = s.run(cmd...)
		if i == 0 || err == nil {
			continue
		}
		return fmt.Errorf("Error running cmd `%s`: %v\n%s", strings.Join(cmd, " "), err, out)
	}
	s.log.Println(out)
	return nil
}
