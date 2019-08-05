package main

import (
	"fmt"
	"strings"
)

// LogicalVolume defines the LV part of the LVM subsystem pieces.
// This contains the actual or configuration desired.
type LogicalVolume struct {
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
	UUID string `json:"uuid,omitempty"`
	Path string `json:"path,omitempty"`

	// Image defines the image to dd into this disk
	Image *Image `json:"image,omitempty"`

	// FileSystem defines the file system that should be on this LV.
	FileSystem *FileSystem `json:"fs,omitempty"`

	imageApplied bool
	parentVg     string
}

// LogicalVolumes defines a list of LogicalVolume
type LogicalVolumes []*LogicalVolume

// toCompList converts a list of LogicalVolume into a list of Comparator
func (lvs LogicalVolumes) toCompList() []Comparator {
	ns := []Comparator{}
	for _, lv := range lvs {
		ns = append(ns, lv)
	}
	return ns
}

// ToLogicalVolumeList converts a list of Comparator into a list of LogicalVolume
func ToLogicalVolumeList(src []Comparator, err error) (LogicalVolumes, error) {
	if err != nil {
		return nil, err
	}
	lvs := LogicalVolumes{}
	for _, s := range src {
		lvs = append(lvs, s.(*LogicalVolume))
	}
	return lvs, nil
}

// updateBlkInfo builds info about the logical volume and fs from lsblk
func (lv *LogicalVolume) updateBlkInfo(keys map[string]string) error {
	if fst, ok := keys["FSTYPE"]; ok && fst != "" {
		if lv.FileSystem == nil {
			lv.FileSystem = &FileSystem{parentPath: lv.Path}
		}
		fs := lv.FileSystem
		fs.updateBlkInfo(keys)
	}
	return nil
}

// Equal tests identity equivalence
func (lv *LogicalVolume) Equal(c Comparator) bool {
	nlv := c.(*LogicalVolume)
	return lv.Name == nlv.Name
}

// Merge merges the comparator into the actual object
func (lv *LogicalVolume) Merge(c Comparator) error {
	nlv := c.(*LogicalVolume)
	lv.Size = nlv.Size
	if lv.FileSystem != nil {
		if err := lv.FileSystem.Merge(nlv.FileSystem); err != nil {
			return err
		}
	} else {
		lv.FileSystem = nlv.FileSystem
	}
	return nil
}

//
// Validate an Image object.
//
func (lv *LogicalVolume) Validate() error {
	out := []string{}

	if lv.Name == "" {
		out = append(out, "LV must have a name")
	}
	if lv.Size == "" {
		out = append(out, fmt.Sprintf("LV %s must have a size", lv.Name))
	}

	if lv.Image != nil {
		lv.Image.rawOnly = true
		if e := lv.Image.Validate(); e != nil {
			out = append(out, fmt.Sprintf("lv %s: Image is invalid: %v", lv.Name, e))
		}
	}

	if lv.FileSystem != nil {
		e := lv.FileSystem.Validate()
		if e != nil {
			out = append(out, fmt.Sprintf("lv %s filesystem failed validation: %v", lv.Name, e))
		}
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "n"))
	}
	return nil
}

// Action applies the configuration object to the actual object.
func (lv *LogicalVolume) Action(nlv *LogicalVolume) (Result, error) {
	dmsg(DbgAction|DbgLv, "Logical Volume Action %v %v\n", lv, nlv)

	if lv.Name == "" {
		args := []string{}
		args = append(args, "-n", nlv.Name)

		if nlv.Size == "REST" {
			args = append(args, "-l", "100%FREE")
		} else {
			args = append(args, "-L", nlv.Size)
		}

		args = append(args, lv.parentVg)
		if _, err := runCommand("lvcreate", args...); err != nil {
			return ResultFailed, err
		}
		return ResultRescan, nil
	}

	// GREG: validate something.

	// Image the logical volume.
	if nlv.Image != nil && !lv.imageApplied {
		lv.imageApplied = true

		// Put image in place
		nlv.Image.Path = lv.Path
		if err := nlv.Image.Action(); err != nil {
			return ResultFailed, err
		}
		return ResultRescan, nil
	}

	// Make filesystem if one is here.
	if nlv.FileSystem != nil {
		// Create it if missing.
		rescan := false
		if lv.FileSystem == nil {
			lv.FileSystem = &FileSystem{parentPath: lv.Path}
			rescan = true
		}

		if rs, err := lv.FileSystem.Action(nlv.FileSystem); err != nil {
			return ResultFailed, err
		} else if rescan || rs == ResultRescan {
			return ResultRescan, nil
		}
	}

	return ResultSuccess, nil
}
