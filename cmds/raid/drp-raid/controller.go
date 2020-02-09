package main

import (
	"fmt"
	"sort"
)

// Controller represents a RAID controller
type Controller struct {
	ID     string
	Driver string
	PCI    struct {
		Bus      int64
		Device   int64
		Function int64
	}
	AutoJBOD    bool
	JBODCapable bool
	RaidCapable bool
	RaidLevels  []string
	Volumes     []*Volume
	Disks       []*PhysicalDisk
	Info        map[string]string
	driver      Driver
	idx         int
}

func (c *Controller) Name() string {
	return fmt.Sprintf("%s:%s", c.Driver, c.ID)
}

func (c *Controller) Less(other *Controller) bool {
	if c.PCI.Bus == other.PCI.Bus {
		if c.PCI.Device == other.PCI.Device {
			return c.PCI.Function < other.PCI.Function
		}
		return c.PCI.Device < other.PCI.Device
	}
	return c.PCI.Bus < other.PCI.Bus
}

type Controllers []*Controller

func (c Controllers) Len() int      { return len(c) }
func (c Controllers) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c Controllers) Less(i, j int) bool {
	return c[i].Less(c[j])
}

// ToVolSpecs translates the current present virtual disks to a list of
// VolSpecs that shouls be comparable to a user-provided VolSpec to determine
// whether we need to take action.
//
// specific defines if it should return a non-disk specific format or not.
// true means exact disk description.  false means generalize.
func (c Controllers) ToVolSpecs(specific bool) VolSpecs {
	res := VolSpecs{}
	for _, controller := range c {
		tvols := VolSpecs{}
		raidS := false
		jbod := false
		encrypt := false
		for _, v := range controller.Volumes {
			tv := v.VolSpec()
			if !specific && tv.RaidLevel == "raid0" && len(tv.Disks) == 1 {
				if tv.Encrypt {
					encrypt = true
				}
				raidS = true
				continue
			}
			if !specific && tv.RaidLevel == "jbod" && len(tv.Disks) == 1 {
				jbod = true
				continue
			}
			tvols = append(tvols, tv)
		}
		if !specific {
			// Convert the disks to counts
			for _, v := range tvols {
				v.DiskCount = fmt.Sprintf("%d", len(v.Disks))
				v.Disks = VolSpecDisks{}
			}
			if raidS {
				tvol := &VolSpec{
					Controller: controller.idx,
					RaidLevel:  "raidS",
					DiskCount:  "max",
					Encrypt:    encrypt,
				}
				tvols = append(tvols, tvol)
			}
			if jbod {
				tvol := &VolSpec{
					Controller: controller.idx,
					RaidLevel:  "jbod",
					DiskCount:  "max",
					Encrypt:    encrypt,
				}
				tvols = append(tvols, tvol)
			}
		}

		res = append(res, tvols...)
	}
	sort.Stable(res)
	return res
}

func (c *Controller) addJBODVolume(d *PhysicalDisk) {
	// This is a JBOD, and we need to fake up a volume for it.
	c.Volumes = append(c.Volumes, &Volume{
		ControllerID:     c.ID,
		ControllerDriver: c.Driver,
		Disks:            []*PhysicalDisk{d},
		controller:       c,
		driver:           c.driver,
		Name:             "jbod for " + d.Name(),
		ID:               d.Name(),
		Status:           d.Status,
		RaidLevel:        "jbod",
		Size:             d.Size,
		Fake:             c.AutoJBOD,
	})
}

func (c *Controller) VolSpecDisks() VolSpecDisks {
	res := make(VolSpecDisks, len(c.Disks))
	for i := range c.Disks {
		res[i] = VolSpecDisk{
			Size:      c.Disks[i].Size,
			Enclosure: c.Disks[i].Enclosure,
			Type:      c.Disks[i].MediaType,
			Protocol:  c.Disks[i].Protocol,
			Slot:      c.Disks[i].Slot,
			info:      c.Disks[i].Info,
		}
	}
	return res
}

func (c *Controller) Encrypt(key, password string) error {
	return c.driver.Encrypt(c, key, password)
}

func (c *Controller) Clear() error {
	return c.driver.Clear(c, false)
}

func (c *Controller) Create(v *VolSpec, forceGood bool) error {
	return c.driver.Create(c, v, forceGood)
}
