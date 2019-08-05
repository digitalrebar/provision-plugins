package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	yaml "github.com/ghodss/yaml"
)

var format = "yaml"

// Config contains both the actual scan and the desired config.
// These are held one or the other.  The system will difference
// the two and generate call actions on the parts that are different.
type Config struct {
	// mdadm config - one day

	// Disks define the disks and partitions in the system.
	Disks Disks `json:"disks,omitempty"`

	// VolumeGroups define the LVM components in the system
	VolumeGroups VolumeGroups `json:"vgs,omitempty"`

	// Images define what should be added to the system after the
	// construction of everything.
	Images Images `json:"images,omitempty"`
}

// ScanSystem scans all the components of the system and rebuilds this
// config object with that data.
func (c *Config) ScanSystem() error {
	if e := udevUpdate(); e != nil {
		return fmt.Errorf("Error: udev %v", e)
	}
	disks, e := ScanDisks()
	if e != nil {
		return fmt.Errorf("Error: disk %v", e)
	}
	vgs, e := ScanLVM()
	if e != nil {
		return fmt.Errorf("Error: lvm %v", e)
	}

	if e := ScanFS(disks, vgs); e != nil {
		return fmt.Errorf("Error: fs %v", e)
	}

	return c.Import(disks, vgs)
}

// Import merges the scanned data into the current config object.
func (c *Config) Import(disks Disks, vgs VolumeGroups) error {
	var err error
	c.Disks, err = ToDiskList(MergeList(c.Disks.toCompList(), disks.toCompList()))
	if err != nil {
		return err
	}
	c.VolumeGroups, err = ToVolumeGroupList(MergeList(c.VolumeGroups.toCompList(), vgs.toCompList()))
	if err != nil {
		return err
	}

	// Hook'em together - try to find the devices that become pvs.
	for _, vg := range c.VolumeGroups {
		for _, pv := range vg.physicalVolumes {
			p := disks.FindPartitionByPath(pv.Name)
			if p == nil {
				// XXX: Check swraid one day.
				return fmt.Errorf("Can not find %s in the parts", pv)
			}

			pv := &PhysicalVolume{Name: p.Name}
			p.physicalVolume = pv
		}
	}
	return nil
}

// ReadConfig process the file referenced by filename and validates
// its contents for correctness.
func (c *Config) ReadConfig(filename string) error {
	byteValue, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(byteValue, c)
	if err != nil {
		return err
	}
	err = c.Validate()
	if err != nil {
		return err
	}

	// Hook'em together - try to find the devices that become pvs.
	for _, vg := range c.VolumeGroups {
		for _, dev := range vg.Devices {
			obj := c.Disks.FindByName(dev)
			if obj == nil {
				// XXX: Check swraid one day.
				return fmt.Errorf("Can not find %s in the parts", dev)
			}

			p, ok := obj.(*Partition)
			if !ok {
				return fmt.Errorf("PhysicaVolume only allowed on partitions: %v", obj)
			}

			pv := &PhysicalVolume{Name: p.Name}
			p.physicalVolume = pv
			vg.physicalVolumes = append(vg.physicalVolumes, pv)
		}
	}
	return nil
}

// Validate iteratively calls Validate on all the objects in the
// config structure.  Returns errors as appropriate.
func (c *Config) Validate() error {
	for _, d := range c.Disks {
		if err := d.Validate(); err != nil {
			return err
		}
	}
	for _, vg := range c.VolumeGroups {
		if err := vg.Validate(); err != nil {
			return err
		}
	}
	for _, img := range c.Images {
		img.tarOnly = true
		if err := img.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Apply config file to a discovered config (ic is config file)
func (c *Config) Apply(ic *Config) (Result, error) {
	dmsg(DbgConfig, "Applying disk configuration\n")
	for _, nd := range ic.Disks {
		found := false
		for _, d := range c.Disks {
			if d.Equal(nd) {
				if rs, err := d.Action(nd); err != nil {
					return rs, err
				} else if rs == ResultRescan {
					return rs, nil
				}
				found = true
				break
			}
		}
		if !found {
			return ResultFailed, fmt.Errorf("Failed to find %v", nd)
		}
	}

	dmsg(DbgConfig, "Applying volume group configuration\n")
	for _, nvg := range ic.VolumeGroups {
		var vg *VolumeGroup
		for _, t := range c.VolumeGroups {
			if t.Equal(nvg) {
				vg = t
				break
			}
		}
		if vg == nil {
			vg = &VolumeGroup{}
		}
		if rs, err := vg.Action(nvg); err != nil {
			return rs, err
		} else if rs == ResultRescan {
			return rs, nil
		}
	}

	dmsg(DbgConfig, "Applying image configuration\n")
	baseDir, err := mountAll(c.Disks, c.VolumeGroups)
	if err != nil {
		return ResultFailed, err
	}
	for _, image := range ic.Images {
		image.Path = fmt.Sprintf("%s%s", baseDir, image.Path)
		if err := image.Action(); err != nil {
			return ResultFailed, err
		}
	}
	err = unmountAll(baseDir, c.Disks, c.VolumeGroups)
	if err != nil {
		return ResultFailed, err
	}

	return ResultSuccess, nil
}

// Dump prints to stdout a yaml or json dump of the config object.
func (c *Config) Dump() {
	switch format {
	case "json":
		js, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		fmt.Println(string(js))
	case "yaml":
		y, err := yaml.Marshal(c)
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		fmt.Println(string(y))
	default:
		fmt.Printf("Unsupported format\n")
	}
}
