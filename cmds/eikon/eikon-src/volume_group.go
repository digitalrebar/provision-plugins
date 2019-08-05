package main

import (
	"fmt"
	"strings"
)

// VolumeGroup defines the base LVM unit.
// It contains the mappings of PVs, LVs, and VGs.
// This contains both actual state and desired configs.
type VolumeGroup struct {
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
	UUID string `json:"uuid,omitempty"`

	LogicalVolumes LogicalVolumes `json:"lvs,omitempty"`
	Devices        []string       `json:"pvs,omitempty"`

	physicalVolumes PhysicalVolumes
}

// VolumeGroups defines a list of VolumeGroups
type VolumeGroups []*VolumeGroup

// FindLvByName finds the lv in the system.
func (vgs VolumeGroups) FindLvByName(name string) *LogicalVolume {
	var vgName, lvName string

	if strings.HasPrefix(name, "/dev/mapper/") {
		piece := name[12:]
		parts := strings.SplitN(piece, "-", 2)
		vgName = parts[0]
		lvName = parts[1]
	} else {
		dmsg(-1, "Handle other names: %s\n", name)
		return nil
	}

	for _, vg := range vgs {
		if vg.Name == vgName {
			for _, lv := range vg.LogicalVolumes {
				if lv.Name == lvName {
					return lv
				}
			}
		}
	}

	return nil

}

// toCompList converts a list of VolumeGroups into a list of Comparator
func (vgs VolumeGroups) toCompList() []Comparator {
	ns := []Comparator{}
	for _, vg := range vgs {
		ns = append(ns, vg)
	}
	return ns
}

// ToVolumeGroupList converts a list of Comparators to a list of VolumeGroups
// If err is set, then return that error
func ToVolumeGroupList(src []Comparator, err error) (VolumeGroups, error) {
	if err != nil {
		return nil, err
	}
	vgs := VolumeGroups{}
	for _, s := range src {
		vgs = append(vgs, s.(*VolumeGroup))
	}
	return vgs, nil
}

// Equal returns if the comparator is identity equivalent to the
// current volume group
func (vg *VolumeGroup) Equal(c Comparator) bool {
	nvg := c.(*VolumeGroup)
	return vg.Name == nvg.Name
}

// Merge merges two volume groups.  This assumes that they are actual
// instances.
func (vg *VolumeGroup) Merge(c Comparator) error {
	nvg := c.(*VolumeGroup)
	vg.Size = nvg.Size

	var err error
	vg.LogicalVolumes, err = ToLogicalVolumeList(MergeList(vg.LogicalVolumes.toCompList(), nvg.LogicalVolumes.toCompList()))
	if err != nil {
		return err
	}

	vg.physicalVolumes, err = ToPhysicalVolumeList(MergeList(vg.physicalVolumes.toCompList(), nvg.physicalVolumes.toCompList()))
	if err != nil {
		return err
	}
	return nil
}

//
// Validate validates a volume group object.
//
func (vg *VolumeGroup) Validate() error {
	out := []string{}

	if vg.Name == "" {
		out = append(out, "VolumeGroup needs a name")
	}
	if len(vg.Devices) == 0 {
		out = append(out, fmt.Sprintf("VolumeGroup %s needs pvs", vg.Name))
	}

	for _, lv := range vg.LogicalVolumes {
		if err := lv.Validate(); err != nil {
			out = append(out, fmt.Sprintf("VolumeGroup %s: lv %s failed: %v", vg.Name, lv.Name, err))
		}
	}
	for _, pv := range vg.physicalVolumes {
		if err := pv.Validate(); err != nil {
			out = append(out, fmt.Sprintf("VolumeGroup %s: pv %s failed: %v", vg.Name, pv.Name, err))
		}
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// Action builds a volume group as defined by the config object.
func (vg *VolumeGroup) Action(nvg *VolumeGroup) (Result, error) {
	dmsg(DbgAction|DbgVg, "Volume Group Action %v %v\n", vg, nvg)

	// This is a create
	if vg.Name == "" {
		args := []string{nvg.Name}
		for _, pv := range nvg.physicalVolumes {
			args = append(args, pv.parentPath)
		}
		out, err := runCommand("vgcreate", args...)
		dmsg(DbgAction|DbgVg, "vgcreate %v: %s\n%v\n", args, string(out), err)
		if err != nil {
			return ResultFailed, err
		}
		return ResultRescan, nil
	}

	// Feel like we should validate something
	// GREG: Validate PVs are present (config vs actual)

	// Build the logical volumes
	for _, nlv := range nvg.LogicalVolumes {
		found := false
		for _, lv := range vg.LogicalVolumes {
			if lv.Equal(nlv) {
				r, e := lv.Action(nlv)
				if e != nil || r == ResultRescan {
					return r, e
				}
				found = true
				break
			}
		}
		if !found {
			// We just recreated a lv, rescan.
			// This is overkill but safe.
			lv := &LogicalVolume{parentVg: vg.Name}
			r, e := lv.Action(nlv)
			if e != nil {
				return r, e
			}
			return ResultRescan, nil
		}
	}

	return ResultSuccess, nil
}
