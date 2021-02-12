package main

import (
	"fmt"
	"log"
	"os"
)

// RaidLevel contains meta-information used to calculate various
// important things about RAID arrays.
type raidLevel struct {
	minDisks          func(spans uint64) uint64
	perDiskSize       func(spans, disksPerSpan, targetUseableSize uint64) uint64
	targetUseableSize func(spans, disksPerSpan, perDiskSize uint64) uint64
	spans             func(totalDisks uint64) (spans, disksPerSpan uint64)
	spanned           bool
	subType           string
}

func roundToStripe(stripeSize, v uint64) uint64 {
	res := v &^ (stripeSize - 1)
	if res != v {
		res += stripeSize
	}
	return res
}

var raidLevels = map[string]raidLevel{
	"jbod": {
		minDisks:          func(uint64) uint64 { return 1 },
		perDiskSize:       func(_, disks, target uint64) uint64 { return target },
		targetUseableSize: func(_, disks, pds uint64) uint64 { return pds },
		spans:             func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raidS": { // raid0:max as one volume / drive instead of all drives in single raid0
		minDisks:          func(uint64) uint64 { return 1 },
		perDiskSize:       func(_, disks, target uint64) uint64 { return target },
		targetUseableSize: func(_, disks, pds uint64) uint64 { return pds },
		spans:             func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"concat": {
		minDisks:          func(uint64) uint64 { return 1 },
		perDiskSize:       func(_, disks, target uint64) uint64 { return target / disks },
		targetUseableSize: func(_, disks, pds uint64) uint64 { return disks * pds },
		spans:             func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid0": {
		minDisks:          func(uint64) uint64 { return 1 },
		perDiskSize:       func(_, disks, target uint64) uint64 { return target / disks },
		targetUseableSize: func(_, disks, pds uint64) uint64 { return disks * pds },
		spans:             func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid1": {
		minDisks: func(uint64) uint64 { return 2 },
		perDiskSize: func(_, disks, targetUseableSize uint64) uint64 {
			return targetUseableSize
		},
		targetUseableSize: func(_, disks, pds uint64) uint64 { return pds },
		spans:             func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid1e": {
		minDisks: func(uint64) uint64 { return 2 },
		perDiskSize: func(_, disks, targetUseableSize uint64) uint64 {
			return (targetUseableSize / disks) * 2
		},
		targetUseableSize: func(_, disks, pds uint64) uint64 {
			return (disks * pds) / 2
		},
		spans: func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid5": {
		minDisks: func(uint64) uint64 { return 3 },
		perDiskSize: func(spans, disksPerSpan, targetUseableSize uint64) uint64 {
			return targetUseableSize / (disksPerSpan - 1)
		},
		targetUseableSize: func(_, disks, pds uint64) uint64 {
			return pds * (disks - 1)
		},
		spans: func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid6": {
		minDisks: func(uint64) uint64 { return 4 },
		perDiskSize: func(spans, disksPerSpan, targetUseableSize uint64) uint64 {
			return targetUseableSize / (disksPerSpan - 2)
		},
		targetUseableSize: func(_, disks, pds uint64) uint64 {
			return pds * (disks - 2)
		},
		spans: func(disks uint64) (uint64, uint64) { return 1, disks },
	},
	"raid00": {
		minDisks:          func(spans uint64) uint64 { return spans },
		perDiskSize:       func(spans, disks, target uint64) uint64 { return target / spans / disks },
		targetUseableSize: func(spans, disks, pds uint64) uint64 { return spans * disks * pds },
		spanned:           true,
		spans:             func(disks uint64) (uint64, uint64) { return 2, disks >> 1 },
	},
	"raid10": {
		minDisks: func(spans uint64) uint64 { return spans * 2 },
		perDiskSize: func(spans, disksPerSpan, targetUseableSize uint64) uint64 {
			return (targetUseableSize / spans / disksPerSpan) * 2
		},
		targetUseableSize: func(spans, disks, pds uint64) uint64 {
			return (pds * spans * disks) / 2
		},
		spans: func(disks uint64) (uint64, uint64) {
			if disks>>1 < 2 {
				return 2, 2
			}
			return disks >> 1, 2
		},
		spanned: true,
	},
	"raid50": {
		minDisks: func(spans uint64) uint64 { return spans * 3 },
		perDiskSize: func(spans, disksPerSpan, targetUseableSize uint64) uint64 {
			return targetUseableSize / ((disksPerSpan * spans) - spans)
		},
		targetUseableSize: func(spans, disks, pds uint64) uint64 {
			return pds * ((disks * spans) - spans)
		},
		spans:   func(disks uint64) (uint64, uint64) { return 2, disks >> 1 },
		spanned: true,
	},
	"raid60": {
		minDisks: func(spans uint64) uint64 { return spans * 4 },
		perDiskSize: func(spans, disksPerSpan, targetUseableSize uint64) uint64 {
			return targetUseableSize / ((disksPerSpan * spans) - (spans * 2))
		},
		targetUseableSize: func(spans, disks, pds uint64) uint64 {
			return pds * ((disks * spans) - (spans * 2))
		},
		spans:   func(disks uint64) (uint64, uint64) { return 2, disks >> 1 },
		spanned: true,
	},
}

func (r raidLevel) Spans(totalDisks uint64) (spans, disksPerSpan uint64) {
	return r.spans(totalDisks)
}

func (r raidLevel) Partition(src []VolSpecDisk) [][]VolSpecDisk {
	spans, dps := r.Spans(uint64(len(src)))
	disks := src
	res := make([][]VolSpecDisk, spans)
	for i := 0; i < int(spans); i++ {
		res[i] = disks[:dps]
		disks = disks[dps:]
	}
	return res
}

// PerDiskSize calculates how much space the RAID array will use on
// each disk given a desired volume size, the desired number of disks
// to use in the array, and the size of each individual data stripe.
func (r raidLevel) PerDiskSize(targetUseableSize, totalDisks, stripeSize uint64) (usedDisks, perDiskSize uint64) {
	perSpanDisks, spans := r.Spans(totalDisks)
	usedDisks = perSpanDisks * spans
	perDiskSize = r.perDiskSize(spans, perSpanDisks, targetUseableSize)
	stripesPerDisk := perDiskSize / stripeSize
	return usedDisks, stripesPerDisk * stripeSize
}

func (r raidLevel) FinalSize(src VolSpecDisks) uint64 {
	disks := src.BySize()
	spans, dps := r.Spans(uint64(len(disks)))
	return r.targetUseableSize(spans, dps, disks[0].Size)
}

// Driver represents the tooling used to manage RAID controllers.
type Driver interface {
	Logger(*log.Logger)
	Name() string
	Executable() string
	Useable() bool
	Controllers() []*Controller
	Refresh(*Controller)
	Clear(c *Controller, foreignOnly bool) error
	Order() int
	Enabled() bool
	Disable()
	Enable()
	Create(c *Controller, v *VolSpec, forceGood bool) error
	Encrypt(c *Controller, key, password string) error
}

func DriverInstalled(d Driver) error {
	fi, err := os.Stat(d.Executable())
	if err != nil {
		return fmt.Errorf("%s: %s is not present", d.Name(), d.Executable())
	}
	if !fi.Mode().IsRegular() || (fi.Mode().Perm()&0333) == 0 {
		return fmt.Errorf("%s executable %s is not an executable", d.Name(), d.Executable())
	}
	return nil
}
