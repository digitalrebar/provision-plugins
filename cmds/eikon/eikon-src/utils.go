package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DebugLevel is an enum that defines log level groups
type DebugLevel int

// These are the debug groups
const (
	DbgScan   DebugLevel = 1 << iota // 1
	DbgDisk                          // 2
	DbgPart                          // 4
	DbgSwRaid                        // 8
	DbgPv                            // 16
	DbgVg                            // 32
	DbgLv                            // 64
	DbgFs                            // 128
	DbgImage                         // 256
	DbgConfig                        // 512
	DbgAction                        // 1024
	DbgCmd                           // 2048
)

// Function to print debug
func dmsg(level DebugLevel, f string, args ...interface{}) {
	if (debug & level) > 0 {
		fmt.Fprintf(os.Stderr, f, args...)
	}
}

// Comparator is an interface that allows for
// common routines to manipulate the objects without
// regard to their type.
type Comparator interface {
	Equal(Comparator) bool
	Merge(Comparator) error
}

// MergeList - This is actual merge over time.
// The merge function assumes that nd is the new object and
// its actual should replace the current d.
func MergeList(base, merge []Comparator) ([]Comparator, error) {
	if len(base) == 0 {
		base = merge
	} else {
		for _, nd := range merge {
			found := false
			for _, d := range base {
				if d.Equal(nd) {
					if err := d.Merge(nd); err != nil {
						return nil, err
					}
					found = true
					break
				}
			}
			if !found {
				base = append(base, nd)
			}
		}

		remove := []Comparator{}
		for _, d := range base {
			found := false
			for _, nd := range merge {
				if nd.Equal(d) {
					found = true
					break
				}
			}
			if !found {
				remove = append(remove, d)
			}
		}

		for _, rd := range remove {
			for i, d := range base {
				if d.Equal(rd) {
					s := base
					s[len(s)-1], s[i] = s[i], s[len(s)-1]
					base = s[:len(s)-1]
					break
				}
			}
		}
	}
	return base, nil
}

// SizeParseError is used to indicate a bad size
type SizeParseError error

// sizeParser parses a string size and returns a number.
func sizeParser(v string) (uint64, error) {
	sizeRE := regexp.MustCompile(`([0-9.]+) *([KMGT]?[B]?)`)
	parts := sizeRE.FindStringSubmatch(v)
	if len(parts) < 2 {
		return 0, SizeParseError(fmt.Errorf("%s cannot be parsed as a Size", v))
	}
	f, err := strconv.ParseFloat(parts[1], 10)
	if err != nil {
		return 0, SizeParseError(err)
	}
	if len(parts) == 3 {
		switch parts[2] {
		case "PB", "P":
			f = f * 1024
		case "TB", "T":
			f = f * 1024
			fallthrough
		case "GB", "G":
			f = f * 1024
			fallthrough
		case "MB", "M":
			f = f * 1024
			fallthrough
		case "KB", "K":
			f = f * 1024
		case "B":
		default:
			return 0, SizeParseError(fmt.Errorf("%s is not a valid size suffix", parts[2]))
		}
	}
	return uint64(f), nil
}

// sizeStringer takes a number and returns a string size.
func sizeStringer(s uint64, unit string) string {
	var suffix string
	var i int
	for i, suffix = range []string{"B", "KB", "MB", "GB", "TB", "PB"} {
		if unit == suffix {
			break
		}
		mul := uint64(1) << ((uint64(i) + 1) * 10)
		if uint64(s) < mul {
			break
		}
	}
	if i != 0 {
		resVal := float64(s) / float64(uint64(1)<<(uint64(i)*10))
		return fmt.Sprintf("%s%s", strconv.FormatFloat(resVal, 'g', -1, 64), suffix)
	}
	return fmt.Sprintf("%dB", s)
}

func runCommand(command string, args ...string) (string, error) {
	dmsg(DbgCmd, "Run Command: %s %v\n", command, args)
	out, err := exec.Command(command, args...).CombinedOutput()
	dmsg(DbgCmd, "Returned: %s\n%v\n", string(out), err)
	if err != nil {
		err = fmt.Errorf("Command: %s failed\n%v\n%s", command, err, string(out))
	}
	return string(out), err
}

func runCommandNoStdErr(command string, args ...string) (string, error) {
	dmsg(DbgCmd, "Run Command: %s %v\n", command, args)
	out, err := exec.Command(command, args...).Output()
	dmsg(DbgCmd, "Returned: %s\n%v\n", string(out), err)
	if err != nil {
		err = fmt.Errorf("Command: %s failed\n%v\n%s", command, err, string(out))
	}
	return string(out), err
}

// runParted runs parted with the provided options.
func runParted(options ...string) error {
	dmsg(DbgCmd, "Parted Command: parted -a opt -s %s\n", strings.Join(options, " "))
	lopts := []string{"-a", "opt", "-s"}
	lopts = append(lopts, options...)
	out, err := exec.Command("parted", lopts...).CombinedOutput()
	dmsg(DbgCmd, "Parted Returned: %s\n%v\n", string(out), err)
	if err != nil {
		err = fmt.Errorf("Parted Failed: %v\n%s", err, string(out))
	}
	return err
}

// udevUpdate causes the system to rescan devices.
func udevUpdate() error {
	if out, err := exec.Command("udevadm", "trigger").CombinedOutput(); err != nil {
		return fmt.Errorf("udevadm trigger Failed: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("udevadm", "settle").CombinedOutput(); err != nil {
		return fmt.Errorf("udevadm settle Failed: %v\n%s", err, string(out))
	}
	return nil
}

func mountTmpFS(path string) (string, error) {
	dir, err := ioutil.TempDir("/tmp", "example")
	if err != nil {
		dmsg(DbgAction, "Failed to create tempdir: %v\n", err)
		return "", err
	}
	_, err = exec.Command("mount", path, dir).CombinedOutput()
	if err != nil {
		dmsg(DbgAction, "Failed to mount tempdir: %s on %s:  %v\n", path, dir, err)
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func unmountTmpFS(path string) error {
	_, err := exec.Command("umount", path).CombinedOutput()
	if err != nil {
		dmsg(DbgAction, "Failed to mount tempdir: %s:  %v\n", path, err)
	}
	os.RemoveAll(path)
	return err
}

type byLen []string

func (a byLen) Len() int {
	return len(a)
}

func (a byLen) Less(i, j int) bool {
	return len(a[i]) < len(a[j])
}

func (a byLen) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func getAllFileSystems(disks Disks, vgs VolumeGroups) ([]string, map[string]string) {
	fsPathMap := map[string]string{}
	fsPathList := []string{}

	for _, d := range disks {
		if d.FileSystem != nil && d.FileSystem.Mount != "" {
			fsPathMap[d.FileSystem.Mount] = d.Path
			fsPathList = append(fsPathList, d.FileSystem.Mount)
		}

		for _, p := range d.Partitions {
			if p.FileSystem != nil && p.FileSystem.Mount != "" {
				fsPathMap[p.FileSystem.Mount] = fmt.Sprintf("%s%d", p.parent.Path, p.ID)
				fsPathList = append(fsPathList, p.FileSystem.Mount)
			}
		}
	}

	// GREG: SwRaid

	for _, vg := range vgs {
		for _, lv := range vg.LogicalVolumes {
			if lv.FileSystem != nil && lv.FileSystem.Mount != "" {
				fsPathMap[lv.FileSystem.Mount] = lv.Path
				fsPathList = append(fsPathList, lv.FileSystem.Mount)
			}
		}
	}

	sort.Sort(byLen(fsPathList))

	return fsPathList, fsPathMap
}

func mountAll(disks Disks, vgs VolumeGroups) (string, error) {
	fsPathList, fsPathMap := getAllFileSystems(disks, vgs)

	dir, err := ioutil.TempDir("/tmp", "example")
	if err != nil {
		dmsg(DbgAction, "Failed to create tempdir: %v\n", err)
		return "", err
	}

	for _, fs := range fsPathList {
		path := fmt.Sprintf("%s%s", dir, fs)
		if err := os.MkdirAll(path, 0755); err != nil {
			unmountAll(dir, disks, vgs)
			return "", err
		}
		if _, err := runCommand("mount", fsPathMap[fs], path); err != nil {
			unmountAll(dir, disks, vgs)
			return "", err
		}
	}

	return dir, nil
}

func unmountAll(dir string, disks Disks, vgs VolumeGroups) error {
	fsPathList, _ := getAllFileSystems(disks, vgs)
	for i := len(fsPathList) - 1; i >= 0; i-- {
		fs := fsPathList[i]
		path := fmt.Sprintf("%s%s", dir, fs)
		if _, err := runCommand("umount", path); err != nil {
			return err
		}
	}
	return os.RemoveAll(dir)
}
