package main

import (
	"bufio"
	"encoding/xml"
	"io"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strings"
)

type StrValid struct {
	Val    string `xml:",chardata"`
	IsNull string `xml:"isnull,attr"`
}

// This struct has to encode the pertinent information from several
// different blobs of XML.  Because Reasons, that is why.
type dellBiosOnlyConfigEnt struct {
	// We will need the name of the element decoded to figure out how to handle it.
	// These appear to be:
	//
	// HIIFormObj, which we will ignore.
	//
	// HIIEnumObj, which specifies a BIOS setting that can take on
	// a small set of values.
	//
	// HIIEnumValueObj, which specifies a value that the preceeding
	// HIIEnumObj can take.
	//
	// HIIStringObj, which specifies an arbitrary string a user can set
	//
	// HIIOrderedListObj, which specifies a BIOS setting that can have multiple values
	//
	// HIIOrderedListEntryObj, which specifies an entry in an OrderedListObj
	XMLName xml.Name
	// Valid for HIIEnumObj, HIIStringObj, and HIIIntegerObj
	ReadOnly bool   `xml:"hdr>bReadOnly"`
	Name     string `xml:"hdr>Name"`
	// Valid for HIIEnumObj
	PendingValid   bool `xml:"bPendingValid"`
	DefaultValid   bool `xml:"bDefaultValid"`
	CurrentState   int  `xml:"currentState"`
	PendingState   int  `xml:"pendingState"`
	DefaultState   int  `xml:"defaultState"`
	PossibleStates int  `xml:"numPossibleStates"`
	// Valid for HIIIntegerObj
	MinVal       *big.Int `xml:"minValue"`
	MaxVal       *big.Int `xml:"maxVal"`
	CurrentValue *big.Int `xml:"currentValue"`
	PendingValue *big.Int `xml:"pendingValue"`
	DefaultValue *big.Int `xml:"defaultValue"`
	// Valid for HIIStringObj
	MinLength  *big.Int `xml:"minLength"`
	MaxLength  *big.Int `xml:"maxLength"`
	CurrentStr StrValid `xml:"Current"`
	PendingStr StrValid `xml:"Pending"`
	DefaultStr StrValid `xml:"Default"`
	// valid for HIIEnumValueObj
	ValIdx  int    `xml:"stateNumber"`
	ValName string `xml:"Name"`
	// valid for HIIOrderedListObj
	ListLen int `xml:"numOrdListEntries"`
	// valid for HIIOrderedListEntryObj
	ListIdx int `xml:"currentIndex"`
}

type dellBiosOnlyConfig struct {
	source  io.Reader
	XMLName xml.Name                `xml:"OMA"`
	Ents    []dellBiosOnlyConfigEnt `xml:",any"`
}

func (d *dellBiosOnlyConfig) Source(src io.Reader) {
	d.source = src
}

// Need the following packages installed:
//
// srvadmin-omacore srvadmin-idrac srvadmin-storage-cli
//
// openmanage services need to start:
//
// /opt/dell/srvadmin/sbin/srvadmin-services.sh start
func (d *dellBiosOnlyConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if d.source == nil {
		cmd := exec.Command("/opt/dell/srvadmin/bin/omreport", "chassis", "biossetup", "-fmt", "xml")
		var w io.ReadCloser
		w, err = cmd.StdoutPipe()
		if err != nil {
			return
		}
		buf := bufio.NewReader(w)
		if err = cmd.Start(); err != nil {
			return
		}
		defer cmd.Wait()
		defer w.Close()
		_, err = buf.ReadSlice('<')
		if err != nil {
			return
		}
		d.source = buf
	}
	// We should be at the start of the XML now.
	dec := xml.NewDecoder(d.source)
	cfg := &dellBiosOnlyConfig{}
	if err = dec.Decode(cfg); err != nil {
		return
	}
	for i := 0; i < len(cfg.Ents); i++ {
		ent := &cfg.Ents[i]
		var working Entry
		switch ent.XMLName.Local {
		case "HIIIntegerObj":
			working = Entry{
				Name:    ent.Name,
				Type:    "Number",
				Current: ent.CurrentValue.String(),
			}
			if ent.PendingValid {
				working.PendingValid = ent.PendingValid
				working.Pending = ent.PendingValue.String()
			}
			if ent.DefaultValid {
				working.Default = ent.DefaultValue.String()
			}
			working.Checker.Int.Valid = true
			working.Checker.Int.Max = ent.MaxVal
			working.Checker.Int.Min = ent.MinVal
		case "HIIStringObj":
			working = Entry{
				Name:    ent.Name,
				Type:    "String",
				Current: ent.CurrentStr.Val,
				Pending: ent.PendingStr.Val,
				Default: ent.DefaultStr.Val,
			}
			working.PendingValid = ent.PendingStr.IsNull != "true"
			working.Checker.String.Valid = true
			working.Checker.String.MaxLen = ent.MaxLength
			working.Checker.String.MinLen = ent.MinLength
		case "HIIEnumObj":
			working = Entry{
				Name: ent.Name,
				Type: "Option",
			}
			working.Checker.Enum.Valid = true
			j := i + 1
			for j < len(cfg.Ents) && cfg.Ents[j].XMLName.Local == "HIIEnumValueObj" {
				ev := &cfg.Ents[j]
				working.Checker.Enum.Values = append(working.Checker.Enum.Values, ev.ValName)
				if ent.CurrentState == ev.ValIdx {
					working.Current = ev.ValName
				}
				if ent.PendingState == ev.ValIdx && ent.PendingValid {
					working.PendingValid = ent.PendingValid
					working.Pending = ev.ValName
				}
				if ent.DefaultState == ev.ValIdx && ent.DefaultValid {
					working.Default = ev.ValName
				}
				j++
			}
			i = j - 1
		case "HIIEnumValueObj":
			log.Panicf("Should not happen")
			return
		case "HIIOrderedListObj":
			working = Entry{
				Type: "Seq",
				Name: ent.Name,
			}
			working.Checker.Seq.Valid = true
			vals := make([]string, ent.ListLen)
			j := i + 1
			for j < len(cfg.Ents) && cfg.Ents[j].XMLName.Local == "HIIOrderedListEntryObj" {
				ev := &cfg.Ents[j]
				vals[ev.ListIdx] = ev.Name
				j++
			}
			working.Current = strings.Join(vals, ",")
			i = j - 1
		case "HIIOrderedListEntryObj":
			log.Panicf("Should not happen")
			return
		case "OMAUserRights", "HIIFormObj", "HIIFormReferenceObj", "ObjCount", "SMStatus":
			continue
		default:
			log.Fatalf("Unknown object type %s from omreport", ent.XMLName.Local)
			continue
		}
		working.ReadOnly = ent.ReadOnly
		res[working.Name] = working
	}
	return
}

func (d *dellBiosOnlyConfig) FixWanted(wanted map[string]string) map[string]string {
	return wanted
}

func (c *dellBiosOnlyConfig) Apply(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	for {
		for k, v := range trimmed {
			args := []string{
				"chassis",
				"biossetup",
				"attribute=" + k,
			}
			if current[k].Checker.Seq.Valid {
				args = append(args, "sequence="+v)
			} else {
				args = append(args, "setting="+v)
			}
			if dryRun {
				return
			}
			cmd := exec.Command("/opt/dell/srvadmin/bin/omconfig", args...)
			var out []byte
			out, err = cmd.CombinedOutput()
			os.Stderr.Write(out)
			if err == nil {
				needReboot = true
				continue
			}
			return
		}
	}
}
