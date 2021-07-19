package main

import (
	"fmt"
	"io"
	"log"
	"math/big"
	"regexp"
	"sort"
)

// Entry is what we expect a BIOS configuration setting to contain.
type Entry struct {
	Name         string
	Type         string
	ReadOnly     bool   `json:",omitempty"`
	PendingValid bool   `json:",omitempty"`
	Current      string `json:",omitempty"`
	Pending      string `json:",omitempty"`
	Default      string `json:",omitempty"`
	Checker      struct {
		Int struct {
			Valid bool     `json:",omitempty"`
			Min   *big.Int `json:",omitempty"`
			Max   *big.Int `json:",omitempty"`
		} `json:",omitempty"`
		String struct {
			Valid  bool     `json:",omitempty"`
			MinLen *big.Int `json:",omitempty"`
			MaxLen *big.Int `json:",omitempty"`
			Regex  string   `json:",omitempty"`
		} `json:",omitempty"`
		Enum struct {
			Valid  bool     `json:",omitempty"`
			Values []string `json:",omitempty"`
		} `json:",omitempty"`
		Seq struct {
			Valid bool `json:",omitempty"`
		} `json:",omitempty"`
	}
}

func (e *Entry) Valid(val string) error {
	if e.Checker.Enum.Valid {
		vals := e.Checker.Enum.Values
		sort.Strings(vals)
		if idx := sort.SearchStrings(vals, val); idx < len(vals) && vals[idx] == val {
			return nil
		}
		return fmt.Errorf("%s: %s is not a valid value.  It must be one of %v", e.Name, val, vals)
	}
	if e.Checker.String.Valid {
		min, max := e.Checker.String.MinLen, e.Checker.String.MaxLen
		regex := e.Checker.String.Regex
		vlen := big.NewInt(int64(len(val)))
		if vlen.Cmp(min) == -1 || vlen.Cmp(max) == 1 {
			return fmt.Errorf("%s: %s is not a valid string, it must be between %d and %d in length", e.Name, val, min, max)
		}
		if regex != `` {
			re, err := regexp.Compile(regex)
			if err != nil {
				return fmt.Errorf("%s: Invalid regex test %s: %v", e.Name, regex, err)
			}
			if !re.MatchString(val) {
				return fmt.Errorf("%s: %s does not match regex %s", e.Name, val, regex)
			}
		}
		return nil
	}
	if e.Checker.Int.Valid {
		v, err := (&big.Int{}).SetString(val, 0)
		if err {
			return fmt.Errorf("%s: %s is not a number", e.Name, val)
		}
		if v.Cmp(e.Checker.Int.Min) == -1 || v.Cmp(e.Checker.Int.Max) == 1 {
			return fmt.Errorf("%s: %d must be between %d and %d", e.Name, v, e.Checker.Int.Min, e.Checker.Int.Max)
		}
		return nil
	}
	return nil
}

func Test(c Configurator, wanted map[string]string) (map[string]Entry, map[string]string, error) {
	current, err := c.Current()
	if err != nil {
		return current, nil, err
	}
	res := map[string]string{}
	for k, v := range wanted {
		ent, ok := current[k]
		if !ok {
			log.Printf("Ignoring setting %s, this BIOS does not support it.", k)
			continue
		}
		if ent.ReadOnly {
			continue
		}
		if ent.PendingValid {
			if v == ent.Pending {
				continue
			}
		} else if v == ent.Current {
			continue
		}
		if err = ent.Valid(v); err != nil {
			return current, nil, err
		}
		ent.Pending = v
		ent.PendingValid = true
		current[k] = ent
		res[k] = v
	}
	return current, res, nil
}

type Configurator interface {
	// Set the source to read configration from.  If left unset, will use current system config
	Source(io.Reader)
	// Get the current BIOS config.
	Current() (map[string]Entry, error)
	// Takes the current and things than need to change in maps to apply.
	// The trimmed (second parameter) is the difference.
	Apply(map[string]Entry, map[string]string, bool) (bool, error)
	// Fix wanted
	FixWanted(map[string]string) map[string]string
}
