package main

import "fmt"

// PhysicalDisk represents an individual physical disk attached to a
// RAID controller.
type PhysicalDisk struct {
	ControllerID       string
	ControllerDriver   string
	VolumeID           string
	Enclosure          string
	Size               uint64
	UsedSize           uint64
	SectorCount        uint64
	PhysicalSectorSize uint64
	LogicalSectorSize  uint64
	Slot               uint64
	Protocol           string
	MediaType          string
	Status             string
	JBOD               bool
	Info               map[string]string
	volume             *Volume
	controller         *Controller
	driver             Driver
}

func (pd *PhysicalDisk) FreeSpace() uint64 {
	return pd.Size - pd.UsedSize
}

func (pd *PhysicalDisk) Less(other *PhysicalDisk) bool {
	if pd.Enclosure == other.Enclosure {
		return pd.Slot < other.Slot
	}
	return pd.Enclosure < other.Enclosure
}

func (pd *PhysicalDisk) Name() string {
	if pd.Enclosure == "" {
		return fmt.Sprintf("%d", pd.Slot)
	}
	return fmt.Sprintf("%s:%d", pd.Enclosure, pd.Slot)
}

type PhysicalDisks []*PhysicalDisk

func (p PhysicalDisks) Len() int      { return len(p) }
func (p PhysicalDisks) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p PhysicalDisks) Less(i, j int) bool {
	return p[i].Less(p[j])
}
