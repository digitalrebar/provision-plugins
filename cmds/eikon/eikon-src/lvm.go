package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// LvmVG defines the LVM VolumeGroup data
type LvmVG struct {
	Fmt              string `json:"vg_fmt"`
	UUID             string `json:"vg_uuid"`
	Name             string `json:"vg_name"`
	Attr             string `json:"vg_attr"`
	Permissions      string `json:"vg_permissions"`
	Extendable       string `json:"vg_extendable"`
	Partial          string `json:"vg_partial"`
	AllocationPolicy string `json:"vg_allocation_policy"`
	Clustered        string `json:"vg_clustered"`
	Size             string `json:"vg_size"`
	Free             string `json:"vg_free"`
	SysID            string `json:"vg_sysid"`
	SystemID         string `json:"vg_systemid"`
	LockType         string `json:"vg_lock_type"`
	LockArgs         string `json:"vg_lock_args"`
	ExtentSize       string `json:"vg_extent_size"`
	ExtentCount      string `json:"vg_extent_count"`
	FreeCount        string `json:"vg_free_count"`
	MaxLv            string `json:"max_lv"`
	MaxPv            string `json:"max_pv"`
	PvCount          string `json:"pv_count"`
	VgMissingPvCount string `json:"vg_missing_pv_count"`
	LvCount          string `json:"lv_count"`
	SnapCount        string `json:"snap_count"`
	SeqNo            string `json:"vg_seqno"`
	Tags             string `json:"vg_tags"`
	Profile          string `json:"vg_profile"`
	MdaCount         string `json:"vg_mda_count"`
	MdaUsedCount     string `json:"vg_mda_used_count"`
	MdaFree          string `json:"vg_mda_free"`
	MdaSize          string `json:"vg_mda_size"`
	MdaCopies        string `json:"vg_mda_copies"`
}

// LvmPV defines the LVM PhysicalVolume data
type LvmPV struct {
	Fmt          string `json:"pv_fmt"`
	UUID         string `json:"pv_uuid"`
	Name         string `json:"pv_name"`
	DevSize      string `json:"dev_size"`
	Major        string `json:"pv_major"`
	Minor        string `json:"pv_minor"`
	MdaFree      string `json:"pv_mda_free"`
	MdaSize      string `json:"pv_mda_size"`
	ExtVsn       string `json:"pv_ext_vsn"`
	PeStart      string `json:"pe_start"`
	Size         string `json:"pv_size"`
	Free         string `json:"pv_free"`
	Used         string `json:"pv_used"`
	Attr         string `json:"pv_attr"`
	Allocatable  string `json:"pv_allocatable"`
	Exported     string `json:"pv_exported"`
	Missing      string `json:"pv_missing"`
	PeCount      string `json:"pv_pe_count"`
	PeAllocCount string `json:"pv_pe_alloc_count"`
	Tags         string `json:"pv_tags"`
	MdaCount     string `json:"pv_mda_count"`
	MdaUsedCount string `json:"pv_mda_used_count"`
	BaStart      string `json:"pv_ba_start"`
	BaSize       string `json:"pv_ba_size"`
	InUse        string `json:"pv_in_use"`
	Duplicate    string `json:"pv_duplicate"`
}

// LvmLV defines the LVM LogicalVolume data
type LvmLV struct {
	UUID                 string `json:"lv_uuid"`
	Name                 string `json:"lv_name"`
	FullName             string `json:"lv_full_name"`
	Path                 string `json:"lv_path"`
	DmPath               string `json:"lv_dm_path"`
	Parent               string `json:"lv_parent"`
	Layout               string `json:"lv_layout"`
	Role                 string `json:"lv_role"`
	InitialImageSync     string `json:"lv_initial_image_sync"`
	ImageSynced          string `json:"lv_image_synced"`
	Merging              string `json:"lv_merging"`
	Converting           string `json:"lv_converting"`
	AllocationPolicy     string `json:"lv_allocation_policy"`
	AllocationLocked     string `json:"lv_allocation_locked"`
	FixedMinor           string `json:"lv_fixed_minor"`
	SkipActivation       string `json:"lv_skip_activation"`
	WhenFull             string `json:"lv_when_full"`
	Active               string `json:"lv_active"`
	ActiveLocally        string `json:"lv_active_locally"`
	ActiveExclusively    string `json:"lv_active_exclusively"`
	Major                string `json:"lv_major"`
	Minor                string `json:"lv_minor"`
	ReadAhead            string `json:"lv_read_ahead"`
	Size                 string `json:"lv_size"`
	MetadataSize         string `json:"lv_metadata_size"`
	SegCount             string `json:"seg_count"`
	Origin               string `json:"origin"`
	OriginUUID           string `json:"origin_uuid"`
	OriginSize           string `json:"origin_size"`
	Ancestors            string `json:"lv_ancestors"`
	FullAncestors        string `json:"lv_full_ancestors"`
	Descendants          string `json:"lv_descendants"`
	FullDescendants      string `json:"lv_full_descendants"`
	RaidMismatchCount    string `json:"raid_mismatch_count"`
	RaidWriteBehind      string `json:"raid_write_behind"`
	RaidMinRecoveryRate  string `json:"raid_min_recovery_rate"`
	RaidMaxRecoveryRate  string `json:"raid_max_recovery_rate"`
	MovePv               string `json:"move_pv"`
	MovePvUUID           string `json:"move_pv_uuid"`
	ConvertLv            string `json:"convert_lv"`
	ConvertLvUUID        string `json:"convert_lv_uuid"`
	MirrorLog            string `json:"mirror_log"`
	MirrorLogUUID        string `json:"mirror_log_uuid"`
	DataLv               string `json:"data_lv"`
	DataLvUUID           string `json:"data_lv_uuid"`
	MetadataLv           string `json:"metadata_lv"`
	MetadataLvUUID       string `json:"metadata_lv_uuid"`
	PoolLv               string `json:"pool_lv"`
	PoolLvUUID           string `json:"pool_lv_uuid"`
	Tags                 string `json:"lv_tags"`
	Profile              string `json:"lv_profile"`
	Lockargs             string `json:"lv_lockargs"`
	Time                 string `json:"lv_time"`
	TimeRemoved          string `json:"lv_time_removed"`
	Host                 string `json:"lv_host"`
	Modules              string `json:"lv_modules"`
	Historical           string `json:"lv_historical"`
	KernelMajor          string `json:"lv_kernel_major"`
	KernelMinor          string `json:"lv_kernel_minor"`
	KernelReadAhead      string `json:"lv_kernel_read_ahead"`
	Permissions          string `json:"lv_permissions"`
	Suspended            string `json:"lv_suspended"`
	LiveTable            string `json:"lv_live_table"`
	InactiveTable        string `json:"lv_inactive_table"`
	DeviceOpen           string `json:"lv_device_open"`
	DataPercent          string `json:"data_percent"`
	SnapPercent          string `json:"snap_percent"`
	MetadataPercent      string `json:"metadata_percent"`
	CopyPercent          string `json:"copy_percent"`
	SyncPercent          string `json:"sync_percent"`
	CacheTotalBlocks     string `json:"cache_total_blocks"`
	CacheUsedBlocks      string `json:"cache_used_blocks"`
	CacheDirtyBlocks     string `json:"cache_dirty_blocks"`
	CacheReadHits        string `json:"cache_read_hits"`
	CacheReadMisses      string `json:"cache_read_misses"`
	CacheWriteHits       string `json:"cache_write_hits"`
	CacheWriteMisses     string `json:"cache_write_misses"`
	KernelCacheSettings  string `json:"kernel_cache_settings"`
	KernelCachePolicy    string `json:"kernel_cache_policy"`
	KernelMetadataFormat string `json:"kernel_metadata_format"`
	HealthStatus         string `json:"lv_health_status"`
	KernelDiscards       string `json:"kernel_discards"`
	CheckNeeded          string `json:"lv_check_needed"`
	MergeFailed          string `json:"lv_merge_failed"`
	SnapshotInvalid      string `json:"lv_snapshot_invalid"`
	Attr                 string `json:"lv_attr"`
}

// PvSeg defines the LVM PhyiscalVolume to LogicalVolume mapping
type PvSeg struct {
	PvSegStart string `json:"pvseg_start"`
	PvSegSize  string `json:"pvseg_size"`
	PvUUID     string `json:"pv_uuid"`
	LvUUID     string `json:"lv_uuid"`
}

// LvmReport defines the data gathered from the LVM subsystem
type LvmReport struct {
	Vgs    []LvmVG `json:"vg"`
	Pvs    []LvmPV `json:"pv"`
	Lvs    []LvmLV `json:"lv"`
	PvSegs []PvSeg `json:"pvseg"`

	// Skip seg
}

// LvmReports defines a list of LvmReports
type LvmReports struct {
	Reports []LvmReport `json:"report"`
}

func convertLvmVGtoVolumeGroup(vg LvmVG) *VolumeGroup {
	nvg := &VolumeGroup{}

	nvg.Name = vg.Name
	nvg.Size = vg.Size
	nvg.UUID = vg.UUID

	return nvg
}

func convertLvmLVtoLogicalVolume(lv LvmLV) *LogicalVolume {
	nlv := &LogicalVolume{}

	nlv.Name = lv.Name
	nlv.Size = lv.Size
	nlv.UUID = lv.UUID
	nlv.Path = lv.Path

	return nlv
}

func convertLvmPVtoPhysicalVolume(pv LvmPV) *PhysicalVolume {
	npv := &PhysicalVolume{}

	npv.Name = pv.Name
	npv.Size = pv.Size
	npv.UUID = pv.UUID

	return npv
}

// ScanLVM scans the actual system and returns a list of VolumeGroup
// in the system
func ScanLVM() (vgs VolumeGroups, err error) {
	vgs = VolumeGroups{}

	var out string
	out, err = runCommandNoStdErr("lvm",
		"fullreport",
		"--reportformat",
		"json",
		"--units",
		"b")
	if err != nil {
		err = fmt.Errorf("e: %v\n%s", err, string(out))
		return
	}

	reports := &LvmReports{}
	err = json.Unmarshal([]byte(out), &reports)
	if err != nil {
		return
	}

	// Get PV to VG map
	out, err = runCommand("pvs", "--separator", ",", "--noheading")
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	pvToVg := map[string]string{}
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		pv := strings.TrimSpace(parts[0])
		vg := strings.TrimSpace(parts[1])
		pvToVg[pv] = vg
	}

	// Note a pvseg with no lv_uuid is a pv without an lv or vg
	for _, r := range reports.Reports {
		pvMap := map[string]*PhysicalVolume{}
		for _, pv := range r.Pvs {
			newPv := convertLvmPVtoPhysicalVolume(pv)
			pvMap[newPv.UUID] = newPv
		}
		for _, vg := range r.Vgs {
			nvg := convertLvmVGtoVolumeGroup(vg)
			for _, pv := range pvMap {
				if name, ok := pvToVg[pv.Name]; ok && nvg.Name == name {
					nvg.physicalVolumes = append(nvg.physicalVolumes, pv)
				}
			}
			vgs = append(vgs, nvg)
		}
		for _, lv := range r.Lvs {
			newLv := convertLvmLVtoLogicalVolume(lv)

			parts := strings.Split(lv.FullName, "/")
			for _, tvg := range vgs {
				if tvg.Name == parts[0] {
					tvg.LogicalVolumes = append(tvg.LogicalVolumes, newLv)
					break
				}
			}
		}
	}

	return
}
