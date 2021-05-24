package main

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
)

type hpConfigEnt struct {
	XMLName xml.Name `xml:"Section"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",innerxml"`
}

type hpConfig struct {
	source  io.Reader
	XMLName xml.Name      `xml:"Conrep"`
	Ents    []hpConfigEnt `xml:",any"`
}

func (h *hpConfig) Source(src io.Reader) {
	h.source = src
}

func (h *hpConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if h.source == nil {
		cmd := exec.Command("conrep", "-s")
		out := []byte{}
		out, err = cmd.CombinedOutput()
		os.Stderr.Write(out)
		if err != nil {
			return
		}
		if !cmd.ProcessState.Success() {
			err = errors.New("Error running conrep")
			return
		}
		var fi *os.File
		fi, err = os.Open("conrep.dat")
		if err != nil {
			return
		}
		defer fi.Close()
		h.source = fi
	}
	dec := xml.NewDecoder(h.source)
	cfg := hpConfig{}
	if err = dec.Decode(&cfg); err != nil {
		return
	}
	for i := range cfg.Ents {
		ent := cfg.Ents[i]
		res[ent.Name] = Entry{
			Name:    ent.Name,
			Current: ent.Value,
		}
	}
	return
}

func (d *hpConfig) FixWanted(wanted map[string]string) map[string]string {
	return wanted
}

func (h *hpConfig) Apply(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	toAdd := &hpConfig{
		Ents: make([]hpConfigEnt, 0, len(trimmed)),
	}
	for k, v := range trimmed {
		toAdd.Ents = append(toAdd.Ents, hpConfigEnt{Name: k, Value: v})
	}
	var fi *os.File
	fi, err = os.Create("conrep.dat")
	if err != nil {
		return
	}
	defer fi.Close()
	enc := xml.NewEncoder(fi)
	if err = enc.Encode(toAdd); err != nil {
		return
	}
	if dryRun {
		return
	}
	cmd := exec.Command("conrep", "-l")
	out := []byte{}
	out, err = cmd.CombinedOutput()
	os.Stderr.Write(out)
	if err != nil {
		return
	}
	if !cmd.ProcessState.Success() {
		err = errors.New("Error running conrep")
		return
	}
	needReboot = true
	return
}
