package main

import (
	"sort"
	"strconv"
)

// Volume represents a RAID array
type Volume struct {
	ControllerID     string
	ControllerDriver string
	ID               string
	Name             string
	Status           string
	RaidLevel        string
	Size             uint64
	StripeSize       uint64
	Spans            uint64
	SpanLength       uint64
	Disks            []*PhysicalDisk
	Info             map[string]string
	controller       *Controller
	driver           Driver
	Fake             bool
	idx              int
}

func (v *Volume) VolSpec() *VolSpec {
	res := &VolSpec{
		VolumeID:   v.ID,
		RaidLevel:  v.RaidLevel,
		Name:       v.Name,
		Size:       sizeStringer(v.Size),
		StripeSize: sizeStringer(v.StripeSize),
		Disks:      make([]VolSpecDisk, len(v.Disks)),
		Fake:       v.Fake,
	}
	if v.controller == nil {
		res.Controller, _ = strconv.Atoi(v.ControllerID)
	} else {
		res.Controller = v.controller.idx
	}
	for i := range v.Disks {
		res.Disks[i] = VolSpecDisk{
			Size:      v.Disks[i].Size,
			Volume:    v.ID,
			Enclosure: v.Disks[i].Enclosure,
			Type:      v.Disks[i].MediaType,
			Protocol:  v.Disks[i].Protocol,
			Slot:      v.Disks[i].Slot,
		}
	}
	sort.Stable(vsdByPos(res.Disks))
	res.Type = res.Disks[0].Type
	res.Protocol = res.Disks[0].Protocol
	return res
}

func (v *Volume) PerDiskSize() uint64 {
	return roundToStripe(v.StripeSize,
		raidLevels[v.RaidLevel].perDiskSize(v.Spans, v.SpanLength, v.Size))
}

func (v *Volume) Less(other *Volume) bool {
	if v.Disks == nil || len(v.Disks) == 0 {
		if other.Disks == nil || len(other.Disks) == 0 {
			return true
		}
		return false
	}
	if other.Disks == nil || len(other.Disks) == 0 {
		return true
	}
	return v.Disks[0].Less(other.Disks[0])
}

type Volumes []*Volume

func (v Volumes) Len() int      { return len(v) }
func (v Volumes) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v Volumes) Less(i, j int) bool {
	return v[i].Less(v[j])
}
