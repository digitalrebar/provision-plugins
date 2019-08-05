package main

import (
	"fmt"
	"strings"
)

// PhysicalVolume represents the PV inside a VolumeGroup
type PhysicalVolume struct {
	// Name is the name of the physical volume - swraid or partition really
	Name string `json:"name"`

	// Size is the current size of the physical volume
	Size string `json:"size"`

	// UUID is the physical volume identifier
	UUID string `json:"uuid"`

	parentPath string
}

// PhysicalVolumes is a list of PhysicalVolume
type PhysicalVolumes []*PhysicalVolume

// toCompList converts a list of PhysicalVolume to a list of Comparator
func (pvs PhysicalVolumes) toCompList() []Comparator {
	ns := []Comparator{}
	for _, pv := range pvs {
		ns = append(ns, pv)
	}
	return ns
}

// ToPhysicalVolumeList converts a list of Comparator to a list of PhysicalVolume
func ToPhysicalVolumeList(src []Comparator, err error) (PhysicalVolumes, error) {
	if err != nil {
		return nil, err
	}
	pvs := PhysicalVolumes{}
	for _, s := range src {
		pvs = append(pvs, s.(*PhysicalVolume))
	}
	return pvs, nil
}

// Equal tests identity equivalence between two objects.
func (pv *PhysicalVolume) Equal(c Comparator) bool {
	npv := c.(*PhysicalVolume)
	return pv.Name == npv.Name
}

// Merge the actual values of the comparator into the PhysicalVolume
func (pv *PhysicalVolume) Merge(c Comparator) error {
	npv := c.(*PhysicalVolume)
	pv.Size = npv.Size
	return nil
}

//
// Validate an PhysicalVolume object.
//
func (pv *PhysicalVolume) Validate() error {
	out := []string{}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// Action applies the configution object to the actual object.
func (pv *PhysicalVolume) Action(npv *PhysicalVolume) (Result, error) {
	dmsg(DbgAction|DbgPv, "PV Action: %v %v\n", pv, npv)

	// This is a create
	if pv.Name == "" {
		pv.Name = npv.Name
		npv.parentPath = pv.parentPath

		out, err := runCommand("pvcreate", pv.parentPath)
		dmsg(DbgAction|DbgPv, "pvcreate: %s\n%v\n", string(out), err)
		if err != nil {
			return ResultFailed, err
		}
		return ResultRescan, nil
	}

	// Already in place.  Do nothing.
	return ResultSuccess, nil
}
