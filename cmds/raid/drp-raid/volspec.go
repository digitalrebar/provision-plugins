package main

import (
	"errors"
	"fmt"
	"log"
	"math/bits"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type bucketKey struct {
	Type     string
	Protocol string
}

type VolSpecDisk struct {
	Size      uint64
	Slot      uint64
	Enclosure string
	Type      string
	Protocol  string
	Volume    string
	info      map[string]string
}

func (v VolSpecDisk) Less(o VolSpecDisk) bool {
	if v.Enclosure < o.Enclosure {
		return true
	}
	if v.Enclosure == o.Enclosure {
		return v.Slot < o.Slot
	}
	return false
}

func (v VolSpecDisk) Equal(o VolSpecDisk) bool {
	return v.Enclosure == o.Enclosure && v.Slot == o.Slot
}

type vsdByPos []VolSpecDisk

func (v vsdByPos) Len() int      { return len(v) }
func (v vsdByPos) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v vsdByPos) Less(i, j int) bool {
	if v[i].Enclosure == v[j].Enclosure {
		return v[i].Slot < v[j].Slot
	}
	return v[i].Enclosure < v[j].Enclosure
}

type vsdByType []VolSpecDisk

func (v vsdByType) Len() int      { return len(v) }
func (v vsdByType) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v vsdByType) Less(i, j int) bool {
	return v[i].Type < v[j].Type
}

type vsdByProtocol []VolSpecDisk

func (v vsdByProtocol) Len() int      { return len(v) }
func (v vsdByProtocol) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v vsdByProtocol) Less(i, j int) bool {
	return v[i].Protocol < v[j].Protocol
}

type vsdBySize []VolSpecDisk

func (v vsdBySize) Len() int      { return len(v) }
func (v vsdBySize) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v vsdBySize) Less(i, j int) bool {
	return v[i].Size < v[j].Size
}

type vsdByVolume []VolSpecDisk

func (v vsdByVolume) Len() int      { return len(v) }
func (v vsdByVolume) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v vsdByVolume) Less(i, j int) bool {
	return v[i].Size < v[j].Size
}

type VolSpecDisks []VolSpecDisk

func (v VolSpecDisks) Equal(o VolSpecDisks) bool {
	if len(v) != len(o) {
		return false
	}
	for i := range v {
		if !v[i].Equal(o[i]) {
			return false
		}
	}
	return true
}

func (v VolSpecDisks) Size() uint64 {
	if len(v) == 0 {
		return 0
	}
	return v[0].Size * uint64(len(v))
}

func (v VolSpecDisks) ByPos() VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByPos(res))
	return res
}

func (v VolSpecDisks) AtPos(encl string, slot uint64) VolSpecDisk {
	res := v.ByPos()
	idx := sort.Search(len(res), func(i int) bool { return res[i].Enclosure >= encl && res[i].Slot >= slot })
	if idx < len(res) && res[idx].Enclosure == encl && res[idx].Slot == slot {
		return res[idx]
	}
	return VolSpecDisk{}
}

func (v VolSpecDisks) OnlyUnused() VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByVolume(res))
	idx := sort.Search(len(res), func(i int) bool { return res[i].Volume > "" })
	if idx == 0 {
		return nil
	}
	if idx == len(v) {
		return res
	}
	return res[:idx-1]
}

func (v VolSpecDisks) BySize() VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdBySize(res))
	return res
}

func (v VolSpecDisks) MinSize(size uint64) VolSpecDisks {
	res := v.BySize()
	idx := sort.Search(len(res), func(i int) bool { return res[i].Size >= size })
	if idx == len(res) {
		return nil
	}
	return res[idx:]
}

func (v VolSpecDisks) ByType() VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByType(res))
	return res
}

func (v VolSpecDisks) Type(t string) VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByType(res))
	startIdx := sort.Search(len(v), func(i int) bool { return res[i].Type >= t })
	endIdx := sort.Search(len(v), func(i int) bool { return res[i].Type > t })
	if startIdx == endIdx {
		return nil
	}
	return res[startIdx:endIdx]
}

func (v VolSpecDisks) ByProtocol() VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByProtocol(res))
	return res
}

func (v VolSpecDisks) Protocol(p string) VolSpecDisks {
	res := make([]VolSpecDisk, len(v))
	copy(res, v)
	sort.Stable(vsdByProtocol(res))
	startIdx := sort.Search(len(v), func(i int) bool { return res[i].Protocol >= p })
	endIdx := sort.Search(len(v), func(i int) bool { return res[i].Protocol > p })
	if startIdx == endIdx {
		return nil
	}
	return res[startIdx:endIdx]
}

func (v VolSpecDisks) First(n int) VolSpecDisks {
	res := make([]VolSpecDisk, n)
	copy(res, v)
	return res
}

func (v VolSpecDisks) Last(n int) VolSpecDisks {
	res := make([]VolSpecDisk, n)
	copy(res, v[len(v)-n:])
	return res
}

func (v VolSpecDisks) Contains(disks ...VolSpecDisk) VolSpecDisks {
	res := []VolSpecDisk{}
	for _, disk := range disks {
		c := v.AtPos(disk.Enclosure, disk.Slot)
		if reflect.DeepEqual(c, VolSpecDisk{}) {
			return nil
		}
		res = append(res, c)
	}
	return VolSpecDisks(res)
}

func (v VolSpecDisks) Bucketize() map[bucketKey]VolSpecDisks {
	res := map[bucketKey]VolSpecDisks{}
	for _, disk := range v {
		key := bucketKey{Type: disk.Type, Protocol: disk.Protocol}
		if _, ok := res[key]; !ok {
			res[key] = []VolSpecDisk{}
		}
		res[key] = append(res[key], disk)
	}
	return res
}

func (v VolSpecDisks) Remove(o VolSpecDisks) VolSpecDisks {
	type rk struct {
		encl string
		sl   uint64
	}
	claimed := map[rk]struct{}{}
	for idx := range o {
		claimed[rk{o[idx].Enclosure, o[idx].Slot}] = struct{}{}
	}
	res := make(VolSpecDisks, 0, len(v)-len(o))
	for _, vsd := range v {
		if _, ok := claimed[rk{vsd.Enclosure, vsd.Slot}]; ok {
			continue
		}
		res = append(res, vsd)
	}
	return res
}

// VolSpec contains information on how to create a volume.  Right now,
// we only can create volumes that use entire physical disks -- we do
// not allow disk configurations that wind up with multiple virtual
// disks sharing the same physical disks.
type VolSpec struct {
	// RaidLevel is the type of RAID to create.  It can be any of the known RAID levels.
	RaidLevel string
	// Size is the desired size of the RAID level.  It can be one of:
	//
	//  * "min", Make a RAID volume out of the smallest disks available
	//
	//  * "max", Make a RAID volume out of the largest disks available
	//
	//  * An integer value with an optional KB, MB, GB, or TB suffix
	//    containing the total desired size of the volume. This will
	//    have drp-raid pick the smallest disks
	//
	//  If Size is the empty string, we will assume you want the max size.
	Size string
	// StripeSize is the size of an individual data stripe in the RAID
	// array.  It must be a power of two, and defaults to 64KB
	StripeSize string
	// Name is the desired name of the RAID volume. If omitted, a
	// unique-ish name will be created.
	Name string
	// VolumeID is the ID of the volume that was created using this VolSpec.
	// It is only meaningful when reporting on existing volumes.
	VolumeID string
	// Bootable should be set of you want this virtual disk to be the
	// one that the RAID controller boots to by default.  This only
	// handlles the RAID controller aspects of making a disk bootable,
	// it deliberately does not try to handle BIOS boot sequence issues.
	Bootable bool
	// Type is the type of disk that should be used to build the
	// array from.  It can be one of "disk", "ssd", "disk,ssd", or
	// "ssd,disk".  If it is unset, "disk,ssd" will be used.  The
	// final array will only be built with disks of the same type.
	Type string
	// Protocol is the low-level protocol that the disks should use to
	// talk to the RAID controller.  It can be one of "sata", "sas",
	// "nvme", "sata,sas", "sas,sata", "nvme,sas,sata". If unset, it
	// will default to "nvme,sas,sata" The final array will only be
	// built with disks using the same protocol.
	Protocol string
	// Controller is the index of the controller that this VolSpec
	// should be placed on.  drp-raid orders controllers in ascending
	// order of their PCI address.  Since this value defaults to 0,
	// VolSpecs will be placed on the first controller unless otherwise
	// specified.
	Controller int
	// Disks is the list of VolSpecDisks to be used to build the RAID volume.
	// Only the Enclosure and Slot fields need to be filled out.
	// Either this must be set or DiskCount must be non-zero.
	Disks VolSpecDisks
	// DiskCount is the number of disks you want to use to create the volume.
	// It can be one of:
	//
	// * "min", make the RAID volume using the minimum number of disks it can
	//   be constructed out of
	//
	// * "max", make the RAID volume using the rest of the available disks
	//   that match the type/protocol of the Volume,
	//
	// * A non-zero integer specifying the number of disks to use
	//
	// * An empty string, in which case Disks must be populated.
	DiskCount string
	// Encrypt indicates if the disk should be encrypted.  For some
	// controllers, this is done after the volume is created.  For
	// others, it just happens as part of making the controller encrypt.
	Encrypt    bool
	diskCount  int
	controller *Controller
	compiled   bool
	index      int // Index in the created volumes.
	// Used to indicate this is a drp-raid created volume because of Passthru or other case.
	Fake bool
}

func (v *VolSpec) Key() string {
	kps := make([]string, len(v.Disks)+1)
	kps[0] = fmt.Sprintf("%d:%s", v.Controller, v.RaidLevel)
	for i, d := range v.Disks {
		kps[i+1] = fmt.Sprintf("%s:%d", d.Enclosure, d.Slot)
	}
	return strings.Join(kps, ",")
}

func (v *VolSpec) Equal(o *VolSpec) bool {
	if v.Controller != o.Controller {
		return false
	}
	if v.RaidLevel != o.RaidLevel {
		return false
	}
	return v.Disks.Equal(o.Disks)
}

func (v *VolSpec) raid() raidLevel {
	return raidLevels[v.RaidLevel]
}

func (v *VolSpec) IsManual() bool {
	return v.DiskCount == ""
}

func (v *VolSpec) sizeBytes() uint64 {
	sz, err := sizeParser(v.Size)
	if err != nil {
		log.Fatalf("volspec: invalid size %s: %v", v.Size, err)
	}
	return sz
}

func (v *VolSpec) stripeSize() uint64 {
	sz, err := sizeParser(v.StripeSize)
	if err != nil {
		log.Fatalf("volspec: invalid stripe size %s: %v", v.StripeSize, err)
	}
	return sz
}

func (v *VolSpec) Fill() error {
	if v == nil {
		return fmt.Errorf("Cannot fill a nil VolSpec")
	}
	testRL := v.RaidLevel
	if testRL == "raidS" {
		testRL = "raid0"
	}
	if _, ok := raidLevels[testRL]; !ok {
		return fmt.Errorf("Raid level '%v' is not supported", v.RaidLevel)
	}
	switch v.Size {
	case "":
		v.Size = "max"
	case "min", "max":
	default:
		reqSize, err := sizeParser(v.Size)
		if err != nil {
			return err
		}
		if reqSize < 100<<20 {
			return fmt.Errorf("Minimum supported size for a virtual disk is 100 MB")
		}
	}
	switch v.StripeSize {
	case "":
		v.StripeSize = "64 KB"
	default:
		reqSize, err := sizeParser(v.StripeSize)
		if err != nil {
			return fmt.Errorf("Stripe size %s not a valid Size: %v", v.StripeSize, err)
		}
		if reqSize > 1<<20 || reqSize < 4<<10 {
			return fmt.Errorf("Invalid stripe size '%s'.  It must be between 4KB and 1 MB", v.StripeSize)
		}
		if bits.OnesCount64(reqSize) != 1 {
			return fmt.Errorf("Invalid stripe size '%s'. It must be a power of two", v.StripeSize)
		}
	}
	if v.Disks != nil && len(v.Disks) > 0 {
		if v.DiskCount != "" {
			return fmt.Errorf("Cannot have Disks and DiskCount set in the same VolSpec")
		}
	} else if v.DiskCount == "" {
		v.DiskCount = "min"
	} else {
		switch v.DiskCount {
		case "min", "max":
		default:
			val, err := strconv.ParseInt(v.DiskCount, 10, 64)
			if err != nil || val < 1 {
				return fmt.Errorf("DiskCount must be one of `min`,`max`, or a positive integer, not `%s`", v.DiskCount)
			}
		}
	}

	if v.IsManual() {
		spans, dps := raidLevels[v.RaidLevel].spans(uint64(len(v.Disks)))
		minDisks := raidLevels[v.RaidLevel].minDisks(spans)
		if uint64(len(v.Disks)) < minDisks {
			return fmt.Errorf("Raid level %s wants at least %d disks", v.RaidLevel, minDisks)
		}
		if spans*dps != uint64(len(v.Disks)) {
			return fmt.Errorf("Raid level %s would want %d disks, not %d", v.RaidLevel, spans*dps, len(v.Disks))
		}
		return nil
	}
	switch v.Type {
	case "disk", "ssd", "disk,ssd", "ssd,disk":
	case "":
		v.Type = "disk,ssd"
	default:
		return fmt.Errorf("'%s' is not a valid disk type", v.Type)
	}
	switch v.Protocol {
	case "sas", "sata", "nvme", "sas,sata", "sata,sas", "nvme,sas,sata":
	case "":
		v.Protocol = "nvme,sas,sata"
	default:
		return fmt.Errorf("'%s' is not a valid disk protocol", v.Protocol)
	}
	return nil
}

func (v *VolSpec) compileManual(s *session, disks VolSpecDisks) (VolSpecDisks, error) {
	s.log.Printf("Picking disks directly")
	candidates := disks.Contains(v.Disks...)
	if candidates == nil {
		return nil, fmt.Errorf("Not enough disks to make volume")
	}
	return candidates.ByPos(), nil
}

func (v *VolSpec) compileOneBucket(s *session, disks VolSpecDisks) VolSpecDisks {
	tgtLvl := v.raid()
	var wantedDisks, spans, disksPerSpan uint64
	switch v.DiskCount {
	case "min":
		spans := 1
		if tgtLvl.spanned {
			spans = 2
		}
		wantedDisks = tgtLvl.minDisks(uint64(spans))
	case "max":
		spans, disksPerSpan = tgtLvl.spans(uint64(len(disks)))
		wantedDisks = spans * disksPerSpan
	default:
		diskCnt, err := strconv.ParseUint(v.DiskCount, 10, 64)
		if err != nil {
			s.Errorf("%s is not a number", v.DiskCount)
			return nil
		}
		wantedDisks = diskCnt
	}
	spans, disksPerSpan = tgtLvl.spans(wantedDisks)
	useDisks := spans * disksPerSpan
	if uint64(len(disks)) < tgtLvl.minDisks(spans) {
		s.log.Printf("Not enough disks to make %s: have %d, want at least %d", v.RaidLevel, len(disks), tgtLvl.minDisks(spans))
		return nil
	}

	if wantedDisks < tgtLvl.minDisks(spans) || useDisks < tgtLvl.minDisks(spans) {
		s.log.Printf("Want to make a %s with %d disks, but need at least %d disks",
			v.RaidLevel,
			useDisks,
			tgtLvl.minDisks(spans))
		return nil
	}
	if useDisks != wantedDisks && v.DiskCount != "max" {
		s.log.Printf("Want to make a %s with %d disks, but can only use %d",
			v.RaidLevel,
			wantedDisks,
			useDisks)
		return nil
	}
	if useDisks < wantedDisks {
		s.log.Printf("Want to make a %s with %d disks, but only %d available",
			v.RaidLevel,
			useDisks,
			len(disks))
		return nil
	}
	bySize := disks.BySize()
	var perDiskSize uint64
	switch v.Size {
	case "min":
		perDiskSize = bySize.First(int(useDisks))[0].Size
	case "max":
		perDiskSize = bySize.Last(int(useDisks))[0].Size
	default:
		totalSize := v.sizeBytes()
		perDiskSize = roundToStripe(v.stripeSize(), tgtLvl.perDiskSize(spans, disksPerSpan, totalSize))
	}
	bySize = bySize.MinSize(perDiskSize).First(int(useDisks))
	if useDisks > uint64(len(bySize)) {
		s.log.Printf("Want to make a %s with %d disks, but only %d available",
			v.RaidLevel,
			useDisks,
			len(bySize))
		return nil
	}
	return bySize.ByPos()
}

func (v *VolSpec) compileAuto(s *session, disks VolSpecDisks) (VolSpecDisks, error) {
	s.log.Printf("Picking disks heuristically")
	if v.RaidLevel == "jbod" && v.DiskCount == "max" {
		s.log.Printf("Max JBOD, taking the rest of the disks")
		// Take all of the remaining disks on the controller
		return disks, nil
	}
	if v.RaidLevel == "raidS" && v.DiskCount == "max" {
		s.log.Printf("Max Raid0, taking the rest of the disks")
		// Take all of the remaining disks on the controller
		return disks, nil
	}
	buckets := disks.Bucketize()
	res := VolSpecDisks{}
	var err error
	var pt, dt string
	for _, proto := range strings.Split(v.Protocol, ",") {
		for _, diskType := range strings.Split(v.Type, ",") {
			err = nil
			chosen := VolSpecDisks{}
			key := bucketKey{
				Protocol: strings.TrimSpace(proto),
				Type:     strings.TrimSpace(diskType),
			}
			candidates := buckets[key]
			if candidates == nil || len(candidates) == 0 {
				continue
			}
			s.log.Printf("Considering disks of type %s speaking protocol %s", diskType, proto)
			chosen = v.compileOneBucket(s, candidates)
			if len(chosen) == 0 {
				s.log.Printf("Not enough of type %s speaking protocol %s (want %s, have %d)", diskType, proto, v.DiskCount, len(candidates))
				continue
			}
			csz := chosen.Size()
			rsz := res.Size()
			if len(res) == 0 {
				res = chosen
				pt, dt = proto, diskType
				continue
			}
			if v.DiskCount == "max" && len(chosen) > len(res) {
				s.log.Printf("max disk count wanted, and %s:%s has %d more useable disks than %s:%s",
					diskType, proto,
					len(chosen)-len(res),
					dt, pt)
				res = chosen
				pt, dt = proto, diskType
				continue
			}
			if v.Size == "max" && csz > rsz {
				s.log.Printf("max size wanted, and %d %s:%s disks has %d more useable space than %d %s:%s disks",
					len(chosen), diskType, proto,
					csz-rsz,
					len(res), dt, pt)
				res = chosen
				pt, dt = proto, diskType
				continue
			}
			if v.Size != "max" && rsz > csz {
				s.log.Printf(" %d %s:%s disks wastes %d less space than %d %s:%s disks",
					len(chosen), diskType, proto,
					rsz-csz,
					len(res), dt, pt)
				res = chosen
				pt, dt = proto, diskType
				continue
			}
		}
	}
	if err == nil && len(res) > 0 {
		s.log.Printf("%d candidates of type %s speaking protocol %s chosen", len(res), dt, pt)
	}
	return res, err
}

func (v *VolSpec) Compile(s *session, disks VolSpecDisks) (res VolSpecDisks, err error) {
	if err = v.Fill(); err != nil {
		return
	}
	lvl := v.raid()
	if v.IsManual() {
		res, err = v.compileManual(s, disks)
	} else {
		res, err = v.compileAuto(s, disks)
	}
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, errors.New("No disks available")
	}
	s.log.Printf("Picked %d disks", len(res))
	v.Size = sizeStringer(lvl.FinalSize(res))
	v.Type = res[0].Type
	v.Protocol = res[0].Protocol
	return
}

type VolSpecs []*VolSpec

func (v VolSpecs) ByController() map[int]VolSpecs {
	res := map[int]VolSpecs{}
	if v == nil || len(v) == 0 {
		return res
	}
	for _, spec := range v {
		c := spec.Controller
		if res[c] == nil {
			res[c] = VolSpecs{}
		}
		res[c] = append(res[c], spec)
	}
	return res
}

func (v VolSpecs) ByKey() map[string]*VolSpec {
	res := map[string]*VolSpec{}
	for _, spec := range v {
		res[spec.Key()] = spec
	}
	return res
}

func (v VolSpecs) Len() int      { return len(v) }
func (v VolSpecs) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v VolSpecs) Less(i, j int) bool {
	if v[i].Controller == v[j].Controller {
		if v[i].IsManual() && v[j].IsManual() {
			return v[i].Disks.ByPos()[0].Less(v[j].Disks.ByPos()[0])
		}
		return v[i].IsManual() && !v[j].IsManual()
	}
	return v[i].Controller < v[j].Controller
}

func (v VolSpecs) Equal(o VolSpecs) bool {
	if len(v) != len(o) {
		return false
	}
	for i := range v {
		if !v[i].Equal(o[i]) {
			return false
		}
	}
	return true
}

func (v VolSpecs) Compile(s *session, c Controllers) (VolSpecs, error) {
	res := VolSpecs([]*VolSpec{})
	if len(c) == 0 {
		return res, nil
	}
	diskPools := make([]VolSpecDisks, len(c))
	for i := range c {
		diskPools[i] = c[i].VolSpecDisks()
	}
	// First pass: pick disks from controllers
	for i := range v {
		spec := v[i]
		s.log.Printf("Considering spec %s:%s at %d", spec.RaidLevel, spec.DiskCount, i)
		pool := diskPools[spec.Controller]
		pickedDisks, err := spec.Compile(s, pool)
		if err != nil {
			s.Errorf("spec at %d: %v", i, err)
			continue
		}
		diskPools[spec.Controller] = pool.Remove(pickedDisks)
		spec.Disks = pickedDisks
		spec.Type = pickedDisks[0].Type
		spec.Protocol = pickedDisks[0].Protocol
	}
	// Second pass: split JBOD/RAID0 VolSpecs into one per disk, and convert to RAID0 if needed
	for i, spec := range v {
		if len(spec.Disks) == 0 {
			s.Errorf("Spec %s:%s at %d missing disks", spec.RaidLevel, spec.DiskCount, i)
			continue
		}
		if spec.RaidLevel != "jbod" && spec.RaidLevel != "raidS" {
			spec.DiskCount = ""
			spec.compiled = true
			res = append(res, spec)
		} else {
			for _, disk := range spec.Disks {
				nv := *spec
				nv.Disks = VolSpecDisks([]VolSpecDisk{disk})
				if nv.RaidLevel == "raidS" {
					nv.RaidLevel = "raid0"
				}
				if !c[nv.Controller].JBODCapable {
					nv.RaidLevel = "raid0"
				}
				nv.DiskCount = ""
				nv.Size = sizeStringer(nv.Disks[0].Size)
				nv.Type = nv.Disks[0].Type
				nv.Protocol = nv.Disks[0].Protocol
				nv.compiled = true
				res = append(res, &nv)
			}
		}
	}
	return res, nil
}
