package main

import (
	"reflect"
	"testing"
)

var disks = VolSpecDisks{
	{
		Enclosure: "3",
		Slot:      16,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "3",
		Slot:      17,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "3",
		Slot:      18,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "3",
		Slot:      19,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "3",
		Slot:      20,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "3",
		Slot:      21,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 30,
	},
	{
		Enclosure: "3",
		Slot:      22,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 30,
	},
	{
		Enclosure: "3",
		Slot:      23,
		Type:      "disk",
		Protocol:  "sas",
		Size:      2 << 30,
	},
	{
		Enclosure: "4",
		Slot:      24,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "4",
		Slot:      25,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "4",
		Slot:      26,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "4",
		Slot:      27,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "4",
		Slot:      28,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "4",
		Slot:      29,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 40,
	},
	{
		Enclosure: "4",
		Slot:      30,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 30,
	},
	{
		Enclosure: "4",
		Slot:      31,
		Type:      "ssd",
		Protocol:  "sas",
		Size:      2 << 35,
	},
	{
		Enclosure: "1",
		Slot:      0,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "1",
		Slot:      1,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "1",
		Slot:      2,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "1",
		Slot:      3,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "1",
		Slot:      4,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "1",
		Slot:      5,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "1",
		Slot:      6,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 35,
	},
	{
		Enclosure: "1",
		Slot:      7,
		Type:      "disk",
		Protocol:  "sata",
		Size:      2 << 35,
	},
	{
		Enclosure: "2",
		Slot:      8,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 35,
	},
	{
		Enclosure: "2",
		Slot:      9,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "2",
		Slot:      10,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "2",
		Slot:      11,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 40,
	},
	{
		Enclosure: "2",
		Slot:      12,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "2",
		Slot:      13,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "2",
		Slot:      14,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 30,
	},
	{
		Enclosure: "2",
		Slot:      15,
		Type:      "ssd",
		Protocol:  "sata",
		Size:      2 << 35,
	},
}

func TestVSDByPos(t *testing.T) {
	byPos := disks.ByPos()
	for i := range byPos {
		if byPos[i].Slot != uint64(i) {
			t.Errorf("Expected VSD at %d to be %d, but it was %d", byPos[i].Slot, byPos[i].Slot, i)
		} else {
			t.Logf("Disk at expected position %d", i)
		}
	}
}

func TestVSDByProtocol(t *testing.T) {
	byProtocol := disks.ByProtocol()
	// Expect sas, then sata.  In each group, expect slot numbers to increase
	idx := uint64(16)
	diskProtocol := "sas"
	for _, disk := range byProtocol {
		if disk.Protocol == "sata" && diskProtocol != "sata" {
			diskProtocol = "sata"
			idx = 0
		}
		if disk.Protocol != diskProtocol {
			t.Errorf("Expected disk protocol %s, not %s", diskProtocol, disk.Protocol)
		} else {
			t.Logf("Got expected disk protocol %s", disk.Protocol)
		}
		if idx != disk.Slot {
			t.Errorf("Expected %s disk at %d, not %d", diskProtocol, idx, disk.Slot)
		} else {
			t.Logf("Got expected %s disk at %d", diskProtocol, idx)
		}
		idx++
	}
}

func TestVSDByType(t *testing.T) {
	byType := disks.ByType()
	idx := uint64(16)
	diskType := "disk"
	// Expect a transtion from 23 -> 0, then 7 -> 24, then 31 -> 8
	for _, disk := range byType {
		switch disk.Slot {
		case 16:
			diskType = "disk"
			idx = 16
		case 24:
			diskType = "ssd"
			idx = 24
		case 0:
			diskType = "disk"
			idx = 0
		case 8:
			diskType = "ssd"
			idx = 8
		}
		if disk.Type != diskType {
			t.Errorf("Expected disk type %s, got %s", diskType, disk.Type)
		} else {
			t.Logf("Got expected disk type %s", diskType)
		}
		if disk.Slot != idx {
			t.Errorf("Expected disk in slot %d, got %d", idx, disk.Slot)
		} else {
			t.Logf("Got expected disk in slot %d", idx)
		}
		idx++
	}
}

func TestVSDBySize(t *testing.T) {
	bySize := disks.BySize()
	idxMap := map[int]uint64{
		0:  21,
		1:  22,
		2:  23,
		3:  30,
		4:  3,
		5:  4,
		6:  5,
		7:  12,
		8:  13,
		9:  14,
		10: 16,
		11: 17,
		12: 24,
		13: 25,
		14: 26,
		15: 31,
		16: 6,
		17: 7,
		18: 8,
		19: 15,
		20: 18,
		21: 19,
		22: 20,
		23: 27,
		24: 28,
		25: 29,
		26: 0,
		27: 1,
		28: 2,
		29: 9,
		30: 10,
		31: 11,
	}
	spaceMap := map[uint64]int{
		2 << 40: 12,
		2 << 35: 10,
		2 << 30: 10,
	}
	counts := map[uint64]int{
		2 << 40: 0,
		2 << 35: 0,
		2 << 30: 0,
	}

	startSpace := uint64(2 << 30)
	for i, disk := range bySize {
		if disk.Slot != idxMap[i] {
			t.Errorf("Expected disk %d at %d, got %d", idxMap[i], i, disk.Slot)
		} else {
			t.Logf("Got expected slot %d at %d", disk.Slot, i)
		}
		if startSpace > disk.Size {
			t.Errorf("TestVSDBySize got out-of-order entry %v at %d", disk, i)
			return
		}
		if startSpace < disk.Size {
			startSpace = disk.Size
		}
		counts[startSpace]++
	}
	for size, count := range spaceMap {
		if counts[size] != count {
			t.Errorf("Expected %d entries for %d, got %d", count, size, counts[size])
		} else {
			t.Logf("Got %d disks for %d, as expected", count, size)
		}
	}
}

func TestVSDBucketize(t *testing.T) {
	buckets := disks.Bucketize()
	keys := []bucketKey{
		{
			Type:     "ssd",
			Protocol: "sas",
		},
		{
			Type:     "disk",
			Protocol: "sas",
		},
		{
			Type:     "ssd",
			Protocol: "sata",
		},
		{
			Type:     "disk",
			Protocol: "sata",
		},
	}
	idxs := [][]uint64{
		{24, 25, 26, 27, 28, 29, 30, 31},
		{16, 17, 18, 19, 20, 21, 22, 23},
		{8, 9, 10, 11, 12, 13, 14, 15},
		{0, 1, 2, 3, 4, 5, 6, 7, 8},
	}
	for kIdx, key := range keys {
		theseDisks, ok := buckets[key]
		if !ok {
			t.Fatalf("Expected to get disks for %v, but there are no disks in the map!", key)
		}
		if theseDisks == nil || len(theseDisks) != 8 {
			t.Fatalf("Expected 8 disks for %v", key)
		}
		for i, disk := range theseDisks {
			if disk.Type != key.Type || disk.Protocol != key.Protocol {
				t.Errorf("Expected type %s proto %s, not type %s proto %s", key.Type, key.Protocol,
					disk.Type, disk.Protocol)
			} else {
				t.Logf("Got expected type %s proto %s", key.Type, disk.Type)
			}
			if idxs[kIdx][i] != disk.Slot {
				t.Errorf("Expected disk %v at %d to be in slot %d, not %d",
					key, i, idxs[kIdx][i], disk.Slot)
			} else {
				t.Logf("Disk %v at %d in expected slot %d",
					key, i, disk.Slot)
			}
		}
	}
}

type fillTest struct {
	name   string
	f, res *VolSpec
	err    string
}

func (f *fillTest) Run(t *testing.T) {
	expectFail := len(f.err) > 0
	err := f.f.Fill()
	if err != nil {
		if expectFail {
			if err.Error() == f.err {
				t.Logf("Fill %s: got expected error '%s'", f.name, err)
			} else {
				t.Errorf("Fill %s: got error '%s', expected '%s'", f.name, err, f.err)
			}
		} else {
			t.Errorf("Fill %s: Got error '%s', but did not expect an error", f.name, err)
		}
		return
	}
	if expectFail {
		t.Errorf("Fill %s: Expected err '%s', but did not get err", f.name, f.err)
		return
	}
	if reflect.DeepEqual(f.f, f.res) {
		t.Logf("Fill %s: VolSpec filled in as expected", f.name)
		return
	}
	t.Errorf("Fill %s: Did not fill VolSpec as expected.", f.name)
	t.Errorf("\tExpected: %v", f.res)
	t.Errorf("\tGot: %v", f.f)

}

func TestVolSpecFill(t *testing.T) {
	fillTests := []fillTest{
		{"empty", nil, nil, "Cannot fill a nil VolSpec"},
		{
			"basic auto",
			&VolSpec{RaidLevel: "jbod"},
			&VolSpec{
				RaidLevel:  "jbod",
				Size:       "max",
				StripeSize: "64 KB",
				Name:       "",
				Bootable:   false,
				Type:       "disk,ssd",
				Protocol:   "nvme,sas,sata",
				Controller: 0,
				DiskCount:  "min",
			},
			"",
		},
		{"bad raid level", &VolSpec{RaidLevel: "jjbod"}, nil, "Raid level 'jjbod' is not supported"},
	}

	for _, ft := range fillTests {
		ft.Run(t)
	}
}
