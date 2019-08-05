package main

import (
	"fmt"
	"testing"
)

func TestCompile(t *testing.T) {
	// start by making sure JBOD'ing all the things does exactly what is says on the tin.
	jbod := readController(`test-data/controllers/jbod.json`)
	nojbod := readController(`test-data/controllers/nojbod.json`)
	compile(t, jbod, `[{"RaidLevel":"jbod","DiskCount":"max"}]`)
	compile(t, nojbod, `[{"RaidLevel":"jbod","DiskCount":"max"}]`)
	noDisks := ctrlrs(1, "megacli")
	compile(t, noDisks, `[{"RaidLevel":"jbod","DiskCount":"max"}]`)
	// Make sure the RAID volumes get created in exactly the order specified.  THe pattern here is pass/fail/pass/fail
	manyDisks := ctrlrs(1, "ssacli")
	manyDisks[0].addDisks(4, mustSize("1 TB"), "sas", "disk").addDisks(2, mustSize("600 MB"), "sas", "disk")
	compile(t, manyDisks, `[{"DiskCount":"min","RaidLevel":"raid1","Size":"min"},{"DiskCount":"max","RaidLevel":"raid5"}]`)
	compile(t, manyDisks, `[{"DiskCount":"max","RaidLevel":"raid5"},{"DiskCount":"min","RaidLevel":"raid1","Size":"min"}]`)
	manyDisks2 := ctrlrs(1, "megacli")
	manyDisks2[0].addDisks(2, mustSize("600 MB"), "sas", "disk").addDisks(4, mustSize("1 TB"), "sas", "disk")
	compile(t, manyDisks2, `[{"DiskCount":"min","RaidLevel":"raid1","Size":"min"},{"DiskCount":"max","RaidLevel":"raid5"}]`)
	compile(t, manyDisks2, `[{"DiskCount":"max","RaidLevel":"raid5"},{"DiskCount":"min","RaidLevel":"raid1","Size":"min"}]`)
	// Test a mix of requested RAID volume types with available disks
	raids := []string{"jbod", "raid0", "raid1", "raid10", "raid5", "raid6", "raid50", "raid60"}
	for i := 1; i < 9; i++ {
		cmod := "megacli"
		if i%2 == 0 {
			cmod = "ssacli"
		}
		ctrl := ctrlrs(1, cmod)
		ctrl[0].addDisks(i, mustSize("100 GB"), "sas", "ssd")
		for _, lvl := range raids {
			compile(t, ctrl, fmt.Sprintf(`[{"DiskCount":"max","RaidLevel":"%s"}]`, lvl))
		}
	}
	// Add raidS for jbod:max testing
	for i := 1; i < 9; i++ {
		cmod := "megacli"
		if i%2 == 0 {
			cmod = "ssacli"
		}
		ctrl := ctrlrs(1, cmod)
		ctrl[0].addDisks(i, mustSize("100 GB"), "sas", "ssd")
		compile(t, ctrl, `[{"DiskCount":"max","RaidLevel":"raidS"}]`)
	}
}
