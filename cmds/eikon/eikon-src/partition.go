package main

import (
	"fmt"
	"strings"
)

//
// Partition defines a partition to put on the system
//
type Partition struct {
	// ID defines the partition index
	ID int `json:"id,omitempty"`

	// Start defines the start of the partition
	Start string `json:"start,omitempty"`
	// End defines the end of the partition
	End string `json:"end,omitempty"`
	// Size defines the size of the partition
	Size string `json:"size,omitempty"`

	// PType is the type of the partition
	PType string `json:"ptype,omitempty"`
	// Name is the name of the partition
	Name string `json:"name,omitempty"`
	// Flags defines a comma separated list of flags for the partition
	Flags string `json:"flags,omitempty"`

	// Image is the image to put on this partition
	Image *Image `json:"image,omitempty"`

	// FileSystem is the filesystem to apply to this partition.
	FileSystem *FileSystem `json:"fs,omitempty"`

	// Filled in by ReadConfig.
	physicalVolume *PhysicalVolume
	parent         *Disk
	imageApplied   bool
}

// Partitions defines list of Partition
type Partitions []*Partition

// FindPartitionByPath finds a partition in the list by its Path
func (ps Partitions) FindPartitionByPath(path string) *Partition {
	for _, p := range ps {
		if fmt.Sprintf("%s%d", p.parent.Path, p.ID) == path {
			return p
		}
	}
	return nil
}

// FindByName finds a piece by name
func (ps Partitions) FindByName(name string) interface{} {
	for _, p := range ps {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// toCompList converts a list of Partition into a list of Comparator
func (ps Partitions) toCompList() []Comparator {
	ns := []Comparator{}
	for _, p := range ps {
		ns = append(ns, p)
	}
	return ns
}

// ToPartitionList coverts a list of Comparator to a list of Partition
func ToPartitionList(src []Comparator, err error) (Partitions, error) {
	if err != nil {
		return nil, err
	}
	ps := Partitions{}
	for _, s := range src {
		ps = append(ps, s.(*Partition))
	}
	return ps, nil
}

// updateBlkInfo updates the partition with key/values from lsblk
func (p *Partition) updateBlkInfo(keys map[string]string) error {
	if fst, ok := keys["FSTYPE"]; ok && fst != "" {
		if p.FileSystem == nil {
			p.FileSystem = &FileSystem{parentPath: fmt.Sprintf("%s%d", p.parent.Path, p.ID)}
		}
		fs := p.FileSystem
		fs.updateBlkInfo(keys)
	}
	return nil
}

// Equal tests partition identity.
func (p *Partition) Equal(c Comparator) bool {
	np := c.(*Partition)
	return p.ID == np.ID
}

// Merge merges the actual comparator with the actual partition
func (p *Partition) Merge(c Comparator) error {
	np := c.(*Partition)
	if p.FileSystem != nil {
		if err := p.FileSystem.Merge(np.FileSystem); err != nil {
			return err
		}
	} else {
		p.FileSystem = np.FileSystem
	}
	return nil
}

//
// Validate an Partition object.
//
func (p *Partition) Validate() error {
	out := []string{}

	if strings.ContainsAny(p.Name, " \t") {
		out = append(out, "Name should not space or tab")
	}
	if p.ID == 0 {
		out = append(out, "ID must be something other than 0")
	}
	if p.End == "" && p.Size == "" {
		out = append(out, "One of Size or End must be specified")
	}
	if p.End != "" && p.Size != "" {
		out = append(out, "Only one of Size or End should be specified")
	}

	if p.Image != nil {
		p.Image.rawOnly = true
		if e := p.Image.Validate(); e != nil {
			out = append(out, fmt.Sprintf("partition %s: Image is invalid: %v", p.Name, e))
		}
	}

	if p.FileSystem != nil {
		e := p.FileSystem.Validate()
		if e != nil {
			out = append(out, fmt.Sprintf("Partition %s filesystem failed validation: %v", p.Name, e))
		}
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// Action applies the configuration object to the actual object.
func (p *Partition) Action(np *Partition) (Result, error) {
	dmsg(DbgPart|DbgAction, "Partition Action %v %v\n", p, np)

	// This is a NOT new create.
	if p.ID != 0 {
		out := ""

		// We need to be a specific location
		if p.ID != np.ID && np.ID != 0 {
			out += fmt.Sprintf("Partition IDs don't match.  A: %d C: %d\n", p.ID, np.ID)
		}

		if np.Start != "" && np.Start != p.Start {
			o, _ := sizeParser(p.Start)
			n, _ := sizeParser(np.Start)
			if o != n {
				out += fmt.Sprintf("%s: Start does not match. A: %s C: %s\n", np.Name, p.Start, np.Start)
			}
		}
		if np.End != "" && np.End != "100%" && np.End != "REST" && np.End != p.End {
			o, _ := sizeParser(p.End)
			n, _ := sizeParser(np.End)
			if o != n {
				out += fmt.Sprintf("%s: End does not match. A: %s C: %s\n", np.Name, p.End, np.End)
			}
		}
		if np.Size != "REST" && np.Size != "100%" && np.Size != "" && np.Size != p.Size {
			o, _ := sizeParser(p.Size)
			n, _ := sizeParser(np.Size)
			if o != n {
				out += fmt.Sprintf("%s: Size does not match. A: %s C: %s\n", np.Name, p.Size, np.Size)
			}
		}
		// XXX: Not all part types show up here. can we validate the partition type

		// Fail if there are errors.
		if out != "" {
			return ResultFailed, fmt.Errorf(out)
		}

		// Image the partition.
		if np.Image != nil && !p.imageApplied {
			p.imageApplied = true

			// Put image in place
			np.Image.Path = fmt.Sprintf("%s%d", p.parent.Path, p.ID)
			if err := np.Image.Action(); err != nil {
				return ResultFailed, err
			}
			return ResultRescan, nil
		}

		// Make filesystem if one is here.
		if np.FileSystem != nil {
			// Create it if missing.
			rescan := false
			if p.FileSystem == nil {
				p.FileSystem = &FileSystem{parentPath: fmt.Sprintf("%s%d", p.parent.Path, p.ID)}
				rescan = true
			}

			if rs, err := p.FileSystem.Action(np.FileSystem); err != nil {
				return ResultFailed, err
			} else if rescan || rs == ResultRescan {
				return ResultRescan, nil
			}
		}

		if np.physicalVolume != nil {
			rescan := false
			if p.physicalVolume == nil {
				p.physicalVolume = &PhysicalVolume{parentPath: fmt.Sprintf("%s%d", p.parent.Path, p.ID)}
				rescan = true
			}

			if rs, err := p.physicalVolume.Action(np.physicalVolume); err != nil {
				return ResultFailed, err
			} else if rescan || rs == ResultRescan {
				return ResultRescan, nil
			}
		}

		return ResultSuccess, nil
	}

	dmsg(DbgPart|DbgAction, "Starting to partition %s\n", np.Name)
	dmsg(DbgPart|DbgAction, "  Parent NextStart %s\n", p.parent.nextStart)

	args := []string{p.parent.Path, "mkpart"}
	if p.parent.PTable == "msdos" {
		if p.ID >= 4 {
			args = append(args, "extended")
		} else {
			args = append(args, "primary")
		}
	} else {
		args = append(args, np.Name)
	}

	// Figure out start and end.
	start := p.parent.nextStart
	if np.Start != "" {
		start = np.Start
	}
	dmsg(DbgPart|DbgAction, "  Start %s\n", start)
	end := "100%"
	if np.Size != "" {
		dmsg(DbgPart|DbgAction, "  Config Size specified %s\n", np.Size)
		if np.Size == "REST" {
			np.End = "100%"
		} else {
			size, e := sizeParser(np.Size)
			if e != nil {
				return ResultFailed, e
			}
			sb, e := sizeParser(start)
			if e != nil {
				return ResultFailed, e
			}
			np.End = sizeStringer(sb+size-1, "B")
		}
	}
	if np.End != "" {
		if np.End == "REST" {
			end = "100%"
		}
		end = np.End
	}
	dmsg(DbgPart|DbgAction, "  End %s\n", end)

	fsType := ""
	if np.FileSystem != nil {
		fsType = np.FileSystem.Type
	} else if np.physicalVolume != nil {
		fsType = ""
	} else if np.PType != "" {
		fsType = np.PType
	}
	dmsg(DbgPart|DbgAction, "  FSType %s\n", fsType)
	dmsg(DbgPart|DbgAction, "  Path %s\n", p.parent.Path)

	if fsType != "" {
		args = append(args, fsType)
	}
	args = append(args, start, end)

	err := runParted(args...)
	return ResultRescan, err
}
