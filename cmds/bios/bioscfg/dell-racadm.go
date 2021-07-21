package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

type dellRacadmAttrib struct {
	XMLName xml.Name `xml:"Attribute"`
	Name    string   `xml:"Name,attr"`
	Value   string   `xml:",chardata"`
}

func (d *dellRacadmAttrib) decode(target map[string]Entry, names []string) map[string]Entry {
	name := strings.Join(append(names, d.Name), "/")
	target[name] = Entry{Name: name, Current: d.Value}
	return target
}

type dellRacadmComponent struct {
	XMLName    xml.Name                        `xml:"Component"`
	FQDD       string                          `xml:"FQDD,attr"`
	Attributes []dellRacadmAttrib              `xml:"Attribute"`
	Components []dellRacadmComponent           `xml:"Component"`
	components map[string]*dellRacadmComponent `xml:"-"`
}

func (d *dellRacadmComponent) decode(target map[string]Entry, names []string) map[string]Entry {
	n2 := append(names, d.FQDD)
	for _, a := range d.Attributes {
		target = a.decode(target, n2)
	}
	for _, c := range d.Components {
		target = c.decode(target, n2)
	}
	return target
}

func (d *dellRacadmComponent) fill(value string, names []string) {
	if len(names) == 1 {
		if d.Attributes == nil {
			d.Attributes = []dellRacadmAttrib{}
		}
		d.Attributes = append(d.Attributes, dellRacadmAttrib{
			XMLName: xml.Name{Local: "Attribute"},
			Value:   value,
			Name:    names[0],
		})
		return
	}
	if _, ok := d.components[names[0]]; !ok {
		d.components[names[0]] = &dellRacadmComponent{
			XMLName:    xml.Name{Local: "Component"},
			FQDD:       names[0],
			components: map[string]*dellRacadmComponent{},
		}
	}
	c := d.components[names[0]]
	c.fill(value, names[1:])
}

func (d *dellRacadmComponent) fc() {
	d.Components = make([]dellRacadmComponent, 0, len(d.components))
	for _, c := range d.components {
		c.fc()
		d.Components = append(d.Components, *c)
	}
}

type dellRacadmConfig struct {
	source     io.Reader
	XMLName    xml.Name              `xml:"SystemConfiguration"`
	Components []dellRacadmComponent `xml:"Component"`
}

func (d *dellRacadmConfig) Source(src io.Reader) {
	d.source = src
}

func (d *dellRacadmConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if d.source == nil {
		cmd := exec.Command("/opt/dell/srvadmin/sbin/racadm", "get", "-f", "config.xml", "-t", "xml", "--clone")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running racadm: %v", err)
		}
		defer os.Remove("config.xml")
		var w *os.File
		w, err = os.Open("config.xml")
		if err != nil {
			return
		}
		defer w.Close()
		d.source = w
	}
	dec := xml.NewDecoder(d.source)
	cfg := &dellRacadmConfig{}
	if err = dec.Decode(cfg); err != nil {
		return
	}
	for _, c := range cfg.Components {
		res = c.decode(res, []string{})
	}
	return
}

func (d *dellRacadmConfig) FixWanted(wanted map[string]string) map[string]string {
	// Compatibility with the omconfig-based Dell BIOS config, which can only do
	// BIOS settings.
	fixedWanted := map[string]string{}
	for k, v := range wanted {
		if !strings.Contains(k, "/") {
			k = "BIOS.Setup.1-1/" + k
		}
		fixedWanted[k] = v
	}
	return fixedWanted
}

func (d *dellRacadmConfig) Apply(_ map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	d.XMLName.Local = "SystemComponent"
	tk := make([]string, 0, len(trimmed))
	for k := range trimmed {
		tk = append(tk, k)
	}
	cmap := map[string]*dellRacadmComponent{}
	sort.Strings(tk)
	for _, key := range tk {
		parts := strings.Split(key, "/")
		if len(parts) == 0 {
			continue
		}
		if _, ok := cmap[parts[0]]; !ok {
			cmap[parts[0]] = &dellRacadmComponent{
				XMLName:    xml.Name{Local: "Component"},
				FQDD:       parts[0],
				components: map[string]*dellRacadmComponent{},
			}
		}
		cmap[parts[0]].fill(trimmed[key], parts[1:])
	}
	d.Components = make([]dellRacadmComponent, 0, len(cmap))
	for _, c := range cmap {
		c.fc()
		d.Components = append(d.Components, *c)
	}
	var tgt *os.File
	tgt, err = os.Create("update.xml")
	if err != nil {
		err = fmt.Errorf("failed to create update.xml: %v", err)
		return
	}
	defer tgt.Close()
	enc := xml.NewEncoder(tgt)
	if err = enc.Encode(d); err != nil {
		err = fmt.Errorf("failed to encode update.xml: %v", err)
		return
	}
	tgt.Sync()
	if dryRun {
		return
	}
	queueRE := regexp.MustCompile(`racadm jobqueue view -i (JID_[[:digit:]]+)`)
	cmd := exec.Command("/opt/dell/srvadmin/sbin/racadm", "set", "-f", "update.xml", "-t", "xml", "--preview")
	buf, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Racadm set preview failed: %s %v", string(buf), err)
		return
	}
	matches := queueRE.FindSubmatch(buf)
	if len(matches) < 2 {
		err = errors.New("Job not created for system settings update test")
		return
	}
	jid := string(matches[1])
	for {
		time.Sleep(time.Second)
		cmd = exec.Command("/opt/dell/srvadmin/sbin/racadm", "jobqueue", "view", "-i", jid)
		buf, err = cmd.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("Racadm failed to view the jobqueue: %s %v", string(buf), err)
			return
		}
		if !bytes.Contains(buf, []byte(`Status=Completed`)) {
			continue
		}
		break
	}
	if bytes.Contains(buf, []byte(`SYS069: No changes were applied since the current component configuration matched the requested configuration.`)) {
		log.Printf("Nothing needs to be updated\n")
		return
	}
	if !bytes.Contains(buf, []byte(`SYS081: Successfully previewed Server Configuration Profile import operation.`)) {
		err = fmt.Errorf("Requested system settings update will not succeed\n%s", string(buf))
		return
	}
	cmd = exec.Command("/opt/dell/srvadmin/sbin/racadm", "set", "-f", "update.xml", "-t", "xml", "-b", "NoReboot")
	buf, err = cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Racadm failed to update: %s %v", string(buf), err)
		return
	}
	matches = queueRE.FindSubmatch(buf)
	if len(matches) < 2 {
		err = errors.New("Job not created for system settings update")
		return
	}
	jid = string(matches[1])
	log.Printf("Job %s created for system settings application", jid)
	needReboot = true
	return
}
