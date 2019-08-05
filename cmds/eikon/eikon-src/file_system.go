package main

import (
	"fmt"
	"regexp"
	"strings"
)

// FileSystem defines a File System in the system.
// Either the actual or the configured choice.
type FileSystem struct {
	Name string `json:"name,omitempty"`
	Type string `json:"fstype,omitempty"`

	Label string `json:"label,omitempty"`
	Force bool   `json:"force,omitempty"`
	Quick bool   `json:"quick,omitempty"`
	Quiet bool   `json:"quiet,omitempty"`
	UUID  string `json:"uuid,omitempty"`

	Mount string `json:"mount,omitempty"`

	Resize bool `json:"resize,omitempty"`

	Image *Image `json:"image,omitempty"`

	parentPath   string
	imageApplied bool
}

// updateBlkInfo updates the filesystem with info from lsblk
func (fs *FileSystem) updateBlkInfo(keys map[string]string) error {
	if val, ok := keys["FSTYPE"]; ok {
		fs.Type = val
	}
	if val, ok := keys["UUID"]; ok {
		fs.UUID = val
	}
	if val, ok := keys["MOUNTPOINT"]; ok {
		fs.Mount = val
	}
	if val, ok := keys["LABEL"]; ok {
		fs.Label = val
	}

	return nil
}

// Equal tests identity equivalence
func (fs *FileSystem) Equal(c Comparator) bool {
	nfs := c.(*FileSystem)
	return fs.Name == nfs.Name
}

// Merge merges the actual comparator into the current FileSystem
func (fs *FileSystem) Merge(c Comparator) error {
	nfs, _ := c.(*FileSystem)

	fs.Name = nfs.Name
	fs.Type = nfs.Type
	fs.Label = nfs.Label
	fs.UUID = nfs.UUID

	return nil
}

//
// Validate a FileSystem object.
//
func (fs *FileSystem) Validate() error {
	out := []string{}

	if fs.Name == "" {
		out = append(out, fmt.Sprintf("Filesystem %s must have a name", fs.Name))
	}
	if fs.Type == "" {
		out = append(out, fmt.Sprintf("Filesystem %s must have a type", fs.Name))
	}

	if fs.Mount != "" && !strings.HasPrefix(fs.Mount, "/") {
		out = append(out, fmt.Sprintf("Filesystem %s mount %s must start with /", fs.Name, fs.Mount))
	}

	if fs.Image != nil {
		fs.Image.tarOnly = true
		if e := fs.Image.Validate(); e != nil {
			out = append(out, fmt.Sprintf("FS %s: Image is invalid: %v", fs.Name, e))
		}
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// MkfsData defines the args and options for running the various mkfs commands
type MkfsData struct {
	Command        string
	Family         string
	LabelLen       int
	ExtraArgs      []string
	LabelArgs      []string
	ForceArgs      []string
	QuietArgs      []string
	UUIDArgs       []string
	SectorSizeArgs []string
	QuickArgs      []string
	ResizeCommand  string
}

var mkfsdata = map[string]MkfsData{
	"btrfs": {
		Command:        "mkfs.btrfs",
		LabelLen:       256,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"--force"},
		LabelArgs:      []string{"--label"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{"--uuid"},
		SectorSizeArgs: []string{"--sector-size"},
		QuickArgs:      []string{},
		ResizeCommand:  "",
	},
	"ext2": {
		Command:        "mkfs.ext2",
		Family:         "ext",
		LabelLen:       16,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-F"},
		LabelArgs:      []string{"-L"},
		QuietArgs:      []string{"-q"},
		UUIDArgs:       []string{"-U"},
		SectorSizeArgs: []string{"-b"},
		QuickArgs:      []string{},
		ResizeCommand:  "resize2fs",
	},
	"ext3": {
		Command:        "mkfs.ext3",
		Family:         "ext",
		LabelLen:       16,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-F"},
		LabelArgs:      []string{"-L"},
		QuietArgs:      []string{"-q"},
		UUIDArgs:       []string{"-U"},
		SectorSizeArgs: []string{"-b"},
		QuickArgs:      []string{},
		ResizeCommand:  "resize2fs",
	},
	"ext4": {
		Command:        "mkfs.ext4",
		Family:         "ext",
		LabelLen:       16,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-F"},
		LabelArgs:      []string{"-L"},
		QuietArgs:      []string{"-q"},
		UUIDArgs:       []string{"-U"},
		SectorSizeArgs: []string{"-b"},
		QuickArgs:      []string{},
		ResizeCommand:  "resize2fs",
	},
	"fat": {
		Command:        "mkfs.vfat",
		Family:         "fat",
		LabelLen:       11,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-I"},
		LabelArgs:      []string{"-n"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"-S"},
		QuickArgs:      []string{},
		ResizeCommand:  "fatresize",
	},
	"fat12": {
		Command:        "mkfs.vfat",
		Family:         "fat",
		LabelLen:       11,
		ExtraArgs:      []string{"-F", "12"},
		ForceArgs:      []string{"-I"},
		LabelArgs:      []string{"-n"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"-S"},
		QuickArgs:      []string{},
		ResizeCommand:  "fatresize",
	},
	"fat16": {
		Command:        "mkfs.vfat",
		Family:         "fat",
		LabelLen:       11,
		ExtraArgs:      []string{"-F", "16"},
		ForceArgs:      []string{"-I"},
		LabelArgs:      []string{"-n"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"-S"},
		QuickArgs:      []string{},
		ResizeCommand:  "fatresize",
	},
	"fat32": {
		Command:        "mkfs.vfat",
		Family:         "fat",
		LabelLen:       11,
		ExtraArgs:      []string{"-F", "32"},
		ForceArgs:      []string{"-I"},
		LabelArgs:      []string{"-n"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"-S"},
		QuickArgs:      []string{},
		ResizeCommand:  "fatresize",
	},
	"vfat": {
		Command:        "mkfs.vfat",
		Family:         "fat",
		LabelLen:       11,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-I"},
		LabelArgs:      []string{"-n"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"-S"},
		QuickArgs:      []string{},
		ResizeCommand:  "fatresize",
	},
	"jfs": {
		Command:        "jfs_mkfs",
		Family:         "",
		LabelLen:       16,
		ExtraArgs:      []string{},
		LabelArgs:      []string{"-L"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{},
		QuickArgs:      []string{},
		ResizeCommand:  "mount -o remount,resize /mount/point; umount /mount/point", // GREG: !!
	},
	"ntfs": {
		Command:        "mkntfs",
		Family:         "",
		LabelLen:       32,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"--force"},
		LabelArgs:      []string{"--label"},
		QuietArgs:      []string{"-q"},
		UUIDArgs:       []string{},
		SectorSizeArgs: []string{"--sector-size"},
		QuickArgs:      []string{"-Q"},
		ResizeCommand:  "ntfsresize -x path",
	},
	"reiserfs": {
		Command:        "mkfs.reiserfs",
		Family:         "",
		LabelLen:       16,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-f"},
		LabelArgs:      []string{"--label"},
		QuietArgs:      []string{"-q"},
		UUIDArgs:       []string{"--uuid"},
		SectorSizeArgs: []string{"--block-size"},
		QuickArgs:      []string{},
		ResizeCommand:  "resize_reiserfs path",
	},
	"swap": {
		Command:        "mkswap",
		Family:         "",
		LabelLen:       15,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"--force"},
		LabelArgs:      []string{"--label"},
		QuietArgs:      []string{},
		UUIDArgs:       []string{"--uuid"},
		SectorSizeArgs: []string{},
		QuickArgs:      []string{},
		ResizeCommand:  "xfs_growfs /mount/point", // GREG: !!
	},
	"xfs": {
		Command:        "mkfs.xfs",
		Family:         "",
		LabelLen:       12,
		ExtraArgs:      []string{},
		ForceArgs:      []string{"-f"},
		LabelArgs:      []string{"-L"},
		QuietArgs:      []string{"--quiet"},
		UUIDArgs:       []string{"-m"},
		SectorSizeArgs: []string{"-s"},
		QuickArgs:      []string{},
		ResizeCommand:  "xfs_growfs /mount/point", // GREG: !!
	},
}

// Action applies the configuration object to the actual object.
func (fs *FileSystem) Action(nfs *FileSystem) (Result, error) {
	dmsg(DbgFs|DbgAction, "FileSystem Action %v %v\n", fs, nfs)

	// Make sure the actual can mount later
	fs.Mount = nfs.Mount

	// We are creating
	if fs.Type == "" {
		data, ok := mkfsdata[nfs.Type]
		if !ok {
			return ResultFailed, fmt.Errorf("%s: Unknown type: %s", nfs.Name, nfs.Type)
		}

		args := []string{}

		args = append(args, data.ExtraArgs...)
		if nfs.Force {
			args = append(args, data.ForceArgs...)
		}
		if nfs.Quiet {
			args = append(args, data.QuietArgs...)
		}
		if nfs.Quick {
			args = append(args, data.QuickArgs...)
		}
		if nfs.Label != "" {
			args = append(args, data.LabelArgs...)
			args = append(args, nfs.Label)
		}
		if nfs.UUID != "" {
			args = append(args, data.UUIDArgs...)
			args = append(args, nfs.UUID)
		}

		// GREG: Figure out sectorsize and send that arg.

		args = append(args, fs.parentPath)

		_, err := runCommand(data.Command, args...)
		if err != nil {
			return ResultFailed, err
		}
		return ResultRescan, nil
	}

	if fs.Type != nfs.Type {
		return ResultFailed, fmt.Errorf("FileSystem's don't match: A: %s C: %s", fs.Type, nfs.Type)
	}

	// Try to resize
	if nfs.Resize {
		// GREG: Resize the FS
	}

	if nfs.Image != nil && !fs.imageApplied {
		fs.imageApplied = true

		tmpMount, err := mountTmpFS(fs.parentPath)
		if err != nil {
			return ResultFailed, err
		}

		nfs.Image.Path = tmpMount
		nfs.Image.tarOnly = true
		err = nfs.Image.Action()
		if err != nil {
			return ResultFailed, err
		}

		err = unmountTmpFS(tmpMount)
		if err != nil {
			return ResultFailed, err
		}
	}

	return ResultSuccess, nil
}

// ScanFS scans the system for filesystems and other info.
// It updates the pieces.
func ScanFS(disks Disks, vgs VolumeGroups) error {
	// XXX: lsblk doesn't return FSinfo even with -f (from CLI yes, from go exec no)
	args := []string{}
	args = append(args, "-b", "-n", "-p", "-P", "-f")
	args = append(args, "-o", "NAME,KNAME,MAJ:MIN,FSTYPE,MOUNTPOINT,LABEL,UUID,PARTLABEL,PARTUUID,RA,RO,RM,MODEL,SERIAL,SIZE,STATE,OWNER,GROUP,MODE,ALIGNMENT,MIN-IO,OPT-IO,PHY-SEC,LOG-SEC,ROTA,SCHED,RQ-SIZE,TYPE,DISC-ALN,DISC-GRAN,DISC-MAX,DISC-ZERO,WSAME,WWN,RAND,PKNAME,HCTL,TRAN,REV,VENDOR")

	out, err := runCommand("lsblk", args...)
	if err != nil {
		return err
	}

	// Each line in the out is a blk device entry.  We need to get that line and convert into
	// a key/value map.
	allKeys := map[string]map[string]string{}
	lines := strings.Split(out, "\n")
	re := regexp.MustCompile(`([^=]*)="([^"]*)"`)
	for _, line := range lines {
		data := re.FindAllStringSubmatch(line, -1)
		keys := map[string]string{}
		for _, sd := range data {
			keys[strings.TrimSpace(sd[1])] = strings.TrimSpace(sd[2])
		}
		val, ok := keys["NAME"]
		if !ok || val == "" {
			continue
		}
		allKeys[val] = keys
	}

	// Get FSInfo for blkid - sigh shouldn't have to do this see above.
	out, err = runCommand("blkid")
	if err != nil {
		return err
	}
	lines = strings.Split(out, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		data := re.FindAllStringSubmatch(parts[1], -1)
		keys, ok := allKeys[parts[0]]
		if !ok {
			keys = map[string]string{}
			allKeys[parts[0]] = keys
		}
		for _, sd := range data {
			key := strings.TrimSpace(sd[1])
			val := strings.TrimSpace(sd[2])
			if key == "TYPE" {
				key = "FSTYPE"
			}
			keys[key] = val
		}
	}

	for _, keys := range allKeys {
		switch keys["TYPE"] {
		case "disk":
			d := disks.FindDiskByPath(keys["NAME"])
			if d == nil {
				return fmt.Errorf("Couldn't find disk: %s", keys["NAME"])
			}
			d.updateBlkInfo(keys)
		case "part":
			p := disks.FindPartitionByPath(keys["NAME"])
			if p == nil {
				return fmt.Errorf("Couldn't find part: %s", keys["NAME"])
			}
			p.updateBlkInfo(keys)
		case "lvm":
			lv := vgs.FindLvByName(keys["NAME"])
			if lv == nil {
				return fmt.Errorf("Couldn't find part: %s", keys["NAME"])
			}
			lv.updateBlkInfo(keys)
		default:
			dmsg(DbgScan|DbgFs, "Skipping %s because it is a %s\n", keys["NAME"], keys["TYPE"])
		}
	}

	return nil
}
