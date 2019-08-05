package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//
// Disk defines a logical disk in the system.
//
// This could be a SSD, Spinning disk, or
// external raid drives.  It is basis for all
// other components.
//
// One Path must be specified in the config section.
//
type Disk struct {
	// Name of disk for future reference
	Name string `json:"name"`

	// Path specifies the path to disk
	Path string `json:"path,omitempty"`

	// Path specifies the path to disk by id
	PathByID string `json:"path_by_id,omitempty"`

	// Path specifies the path to disk by uuid
	PathByUUID string `json:"path_by_uuid,omitempty"`

	// Path specifies the path to disk by pci path
	PathByPath string `json:"path_by_path,omitempty"`

	// Image to apply to the disk.
	// Image needs to be a raw format (not tar/zip)
	Image *Image `json:"image,omitempty"`

	// Wipe indicates if the disk should be wiped first
	// meaning PVs, VGs, and FSes removed.
	//
	// Having an image object implies wipe
	Wipe bool `json:"wipe,omitempty"`

	// Zero indicates that the disk should also be zeroed.
	//
	// Having an image object implies zeros
	Zero bool `json:"zero,omitempty"`

	// Partition to use for this disk (if needed).
	// required: true
	PTable string `json:"ptable,omitempty"`

	// GrubDevice indicates if grub should be placed on this device.
	GrubDevice bool `json:"grub_device,omitempty"`

	// Partitions contain the sub-pieces of the disk
	Partitions Partitions `json:"partitions,omitempty"`

	// Size is the size of this disk in bytes
	// Derived value
	Size string `json:"size,omitempty"`

	// FileSystem if defined
	FileSystem *FileSystem `json:"fs,omitempty"`

	// has the image been applied
	imageApplied bool

	// has the label been applied
	labelApplied bool

	// The next starting point for the partition
	nextStart string
}

//
// Disks defines a list of disk
//
type Disks []*Disk

// toCompList converts a list of disks to a list of Comparators
func (ds Disks) toCompList() []Comparator {
	ns := []Comparator{}
	for _, d := range ds {
		ns = append(ns, d)
	}
	return ns
}

// FindDiskByPath finds a disk by path
func (ds Disks) FindDiskByPath(path string) *Disk {
	for _, d := range ds {
		if d.Path == path {
			return d
		}
	}
	return nil
}

// FindByName finds a piece by name
func (ds Disks) FindByName(name string) interface{} {
	for _, d := range ds {
		if d.Name == name {
			return d
		}
		if obj := d.Partitions.FindByName(name); obj != nil {
			return obj
		}
	}
	return nil
}

// FindPartitionByPath finds a partition by path
func (ds Disks) FindPartitionByPath(path string) *Partition {
	for _, d := range ds {
		if p := d.Partitions.FindPartitionByPath(path); p != nil {
			return p
		}
	}
	return nil
}

// ToDiskList converts a list of comparators into a disk list.
// If an error is passed in, then return that error.
func ToDiskList(src []Comparator, err error) (Disks, error) {
	if err != nil {
		return nil, err
	}
	ds := Disks{}
	for _, s := range src {
		ds = append(ds, s.(*Disk))
	}
	return ds, nil
}

// updateBlkInfo updates the disk with key/values from lsblk
func (d *Disk) updateBlkInfo(keys map[string]string) error {

	if fst, ok := keys["FSTYPE"]; ok && fst != "" {
		if d.FileSystem == nil {
			d.FileSystem = &FileSystem{}
		}
		fs := d.FileSystem
		fs.updateBlkInfo(keys)
	}

	return nil
}

// Equal compares to disks for identity equivalence.
// This is used to make sure two disks reference the same
// disk.  This is used for config to actual and actual to actual.
func (d *Disk) Equal(c Comparator) bool {
	nd := c.(*Disk)
	// GREG: this needs work to handle other path comparators.
	return d.Path == nd.Path
}

// Merge takes the incoming argument and replaces the actual parts.
func (d *Disk) Merge(c Comparator) error {
	nd := c.(*Disk)
	d.PTable = nd.PTable
	d.GrubDevice = nd.GrubDevice
	d.Size = nd.Size
	d.nextStart = nd.nextStart
	if d.FileSystem != nil {
		if err := d.FileSystem.Merge(nd.FileSystem); err != nil {
			return err
		}
	} else {
		d.FileSystem = nd.FileSystem
	}
	var err error
	d.Partitions, err = ToPartitionList(
		MergeList(d.Partitions.toCompList(), nd.Partitions.toCompList()))
	return err
}

//
// Validate validates a Disk object.
//
func (d *Disk) Validate() error {
	out := []string{}

	if d.Name == "" {
		out = append(out, "name must be specified")
	}
	if d.Path == "" {
		out = append(out, "path must be specified")
	}

	switch d.PTable {
	case "aix", "amiga", "bsd", "dvh", "gpt", "mac", "msdos", "pc98", "sun", "loop":
	default:
		out = append(out, fmt.Sprintf("ptable is not valid: %s", d.PTable))
	}

	for i, p := range d.Partitions {
		// If partition id is not specified, then build it from list position.
		if p.ID == 0 {
			p.ID = i + 1 // 1-based not 0-based
		}
		if e := p.Validate(); e != nil {
			out = append(out, fmt.Sprintf("disk %s: %s is invalid: %v", d.Name, p.Name, e))
		}
	}

	ids := map[int]struct{}{}
	for _, p := range d.Partitions {
		if _, ok := ids[p.ID]; ok {
			out = append(out, fmt.Sprintf("partitions using duplicate IDs on disk %s", d.Name))
		}
		ids[p.ID] = struct{}{}
	}

	if d.FileSystem != nil {
		if e := d.FileSystem.Validate(); e != nil {
			out = append(out, fmt.Sprintf("disk %s: FileSystem is invalid: %v", d.Name, e))
		}
	}

	if d.Image != nil {
		d.Image.rawOnly = true
		if e := d.Image.Validate(); e != nil {
			out = append(out, fmt.Sprintf("disk %s: Image is invalid: %v", d.Name, e))
		}
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// Action takes a config version of the object and applies it
// to the actual object.  return ResultRescan when a
// re-evaluation is needed.
func (d *Disk) Action(c Comparator) (Result, error) {
	di := c.(*Disk)
	dmsg(DbgDisk|DbgAction, "Disk (%s,%s) action\n", d.Name, di.Name)
	if !d.imageApplied {
		d.imageApplied = true

		if di.Wipe || di.Zero || di.Image != nil {
			// GREG: Wipe disk
		}

		if di.Zero || di.Image != nil {
			// GREG: Zero disk
		}

		if di.Image != nil {
			// Put image in place
			di.Image.Path = d.Path
			if err := di.Image.Action(); err != nil {
				return ResultFailed, err
			}
			d.labelApplied = true
			return ResultRescan, nil
		}
	}

	if !d.labelApplied && di.PTable != d.PTable {
		d.labelApplied = true
		return ResultRescan, runParted(d.Path, "mklabel", di.PTable)
	}

	// Ensure partition table is correct.
	if di.PTable != d.PTable {
		return ResultFailed, fmt.Errorf("Disk, %s, disk label doesn't match (A: %s C: %s)", di.Name, d.PTable, di.PTable)
	}

	// Build the partitions.
	for _, pi := range di.Partitions {
		found := false
		for _, p := range d.Partitions {
			if p.Equal(pi) {
				r, e := p.Action(pi)
				if e != nil || r == ResultRescan {
					return r, e
				}
				found = true
				break
			}
		}
		if !found {
			// We just recreated a partition, rescan.
			// This is overkill but safe.
			p := &Partition{parent: d}
			r, e := p.Action(pi)
			if e != nil {
				return r, e
			}
			return ResultRescan, nil
		}
	}

	return ResultSuccess, nil
}

// ScanDisks scans the system for disks and partitions.
func ScanDisks() (Disks, error) {
	out, err := exec.Command("parted", "-s", "-m", "/dev/sda", "unit", "b", "print", "list").CombinedOutput()
	if err != nil {
		return nil, err
	}

	disks := Disks{}

	lines := strings.Split(string(out), "\n")
	var currentDisk *Disk
	diskLine := false
	partLine := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			diskLine = false
			partLine = false
			continue
		}
		if strings.HasPrefix(line, "Error") {
			dmsg(DbgDisk|DbgScan, "Skipping Error line: '%s'\n", line)
			continue
		}
		if strings.Contains(line, "BYT;") {
			currentDisk = &Disk{nextStart: "1048576B"}
			disks = append(disks, currentDisk)
			diskLine = true
			partLine = false
			continue
		}
		if diskLine {
			dmsg(DbgDisk|DbgScan, "processing disk line: %s\n", line)
			parts := strings.Split(line, ":")
			currentDisk.Path = parts[0]
			currentDisk.Size = parts[1]
			currentDisk.PTable = parts[5]
			partLine = true
			diskLine = false
			continue
		}
		if partLine {
			dmsg(DbgDisk|DbgScan, "processing part line: %s\n", line)
			part := &Partition{}
			parts := strings.Split(line, ":")

			part.ID, _ = strconv.Atoi(parts[0])
			part.Start = parts[1]
			part.End = parts[2]
			part.Size = parts[3]
			part.PType = parts[4]
			part.Name = parts[5]
			part.Flags = parts[6]
			part.parent = currentDisk
			currentDisk.Partitions = append(currentDisk.Partitions, part)
			end, _ := sizeParser(part.End)
			currentDisk.nextStart = sizeStringer(end+1, "B")
			continue
		}
	}

	return disks, nil
}
