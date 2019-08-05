package main

import (
	"fmt"
	"strings"
)

// SwRaid defines a software raid system
type SwRaid struct {
	Name string `json:"name"`
}

// SwRaids defines list of SwRaid
type SwRaids []*SwRaid

// toCompList converts a list of SwRaid to a list of Comparator
func (sws SwRaids) toCompList() []Comparator {
	ns := []Comparator{}
	for _, sw := range sws {
		ns = append(ns, sw)
	}
	return ns
}

// ToSwRaidList converts a list of Comparator to a list of SwRaid
func ToSwRaidList(src []Comparator, err error) (SwRaids, error) {
	if err != nil {
		return nil, err
	}
	sws := SwRaids{}
	for _, s := range src {
		sws = append(sws, s.(*SwRaid))
	}
	return sws, nil
}

// Equal tests identity equavalence for SwRaid pieces.
func (sw *SwRaid) Equal(c Comparator) bool {
	nsw := c.(*SwRaid)
	return sw.Name == nsw.Name
}

// Merge merges to actual SwRaid pieces.
func (sw *SwRaid) Merge(c Comparator) error {
	return nil
}

//
// Validate a SwRaid object.
//
func (sw *SwRaid) Validate() error {
	out := []string{}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

// Action applies the configuration in the comparator to the
// actual SwRaid.
func (sw *SwRaid) Action(c Comparator) (Result, error) {
	return ResultFailed, fmt.Errorf("SwRaid Not Implemented")
}
