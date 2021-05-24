package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type superMicroInfo struct {
	XMLName xml.Name `xml:"Information,omitempty"`
	Help    string   `xml:",omitempty,cdata"`
	// Only meaningful if Setting.type == "Option"
	Options       []string `xml:"AvailableOptions>Option,omitempty"`
	DefaultOption string   `xml:"DefaultOption,omitempty"`
	// Only meaningful when Setting.type == "Numeric"
	MaxValue     int `xml:",omitempty"`
	MinValue     int `xml:",omitempty"`
	StepSize     int `xml:",omitempty"`
	DefaultValue int `xml:",omitempty"`
	// Only meaningful when Setting.type == "CheckBox"
	DefaultStatus string `xml:",omitempty"`
	// Only meaningful when Setting.type == "Password"
	HasPassword string `xml:",omitempty"`
	// Meaningful when Setting.type == "Password" or "String"
	MinSize int `xml:",omitempty"`
	MaxSize int `xml:",omitempty"`
	// Only meaningful when Setting.type == "String"
	DefaultString        string `xml:",omitempty"`
	AllowingMultipleLine string `xml:",omitempty"`
}

type superMicroSetting struct {
	XMLName            xml.Name        `xml:"Setting"`
	Name               string          `xml:"name,attr"`
	Type               string          `xml:"type,attr"`
	NumericValue       string          `xml:"numericValue,attr,omitempty"`
	Checked            string          `xml:"checkedStatus,attr,omitempty"`
	Option             string          `xml:"selectedOption,attr,omitempty"`
	String             string          `xml:"StringValue,omitempty,cdata"`
	NewPassword        string          `xml:"NewPassword,omitempty,cdata"`
	ConfirmNewPassword string          `xml:"ConfirmNewPassword,omitempty,cdata"`
	Information        *superMicroInfo `xml:",omitempty"`
}

func (s *superMicroSetting) decode(res map[string]Entry, names []string) {
	name := path.Join(append(names, s.Name)...)
	ent := Entry{
		Type: s.Type,
		Name: name,
	}
	switch s.Type {
	case "Numeric":
		ent.Current = s.NumericValue
		if s.Information != nil {
			ent.Checker.Int.Valid = true
			ent.Checker.Int.Min = s.Information.MinValue
			ent.Checker.Int.Max = s.Information.MaxValue
			ent.Default = strconv.Itoa(s.Information.DefaultValue)
		}
	case "CheckBox":
		ent.Current = s.Checked
		if s.Information != nil {
			ent.Checker.Enum.Valid = true
			ent.Checker.Enum.Values = []string{"Checked", "Unchecked"}
			ent.Default = s.Information.DefaultStatus
		}
	case "Option":
		ent.Current = s.Option
		if s.Information != nil {
			ent.Checker.Enum.Valid = true
			ent.Checker.Enum.Values = s.Information.Options
			ent.Default = s.Information.DefaultOption
			sort.Strings(ent.Checker.Enum.Values)
		}
	case "String", "Password":
		ent.Current = s.String
		if s.Information != nil {
			ent.Checker.String.Valid = true
			ent.Checker.String.MinLen = s.Information.MinSize
			ent.Checker.String.MaxLen = s.Information.MaxSize
			ent.Default = s.Information.DefaultString
		}
	}
	res[name] = ent
}

type superMicroMenu struct {
	XMLName  xml.Name          `xml:"Menu"`
	Name     string            `xml:"name,attr"`
	Menu     []*superMicroMenu `xml:"Menu"`
	menu     map[string]*superMicroMenu
	Settings []*superMicroSetting `xml:"Setting"`
}

func (m *superMicroMenu) decode(res map[string]Entry, names []string) {
	sub := append(names, m.Name)
	for _, s := range m.Settings {
		s.decode(res, sub)
	}
	for _, mm := range m.Menu {
		mm.decode(res, sub)
	}
}

type superMicroBios struct {
	XMLName xml.Name          `xml:"BiosCfg"`
	Menu    []*superMicroMenu `xml:"Menu"`
	menu    map[string]*superMicroMenu
}

type superMicroConfig struct {
	licenseKey string
	source     io.Reader
}

func (s *superMicroConfig) Source(r io.Reader) {
	s.source = r
}

func (s *superMicroConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if s.source == nil {
		cmd := exec.Command("sum", "-c", "GetCurrentBiosCfg", "--file", "config.xml", "--overwrite")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running sum: %v", err)
		}
		defer os.Remove("config.xml")
		var w *os.File
		w, err = os.Open("config.xml")
		if err != nil {
			return
		}
		defer w.Close()
		s.source = w
	}
	cfg := superMicroBios{}
	dec := xml.NewDecoder(s.source)

	if err = dec.Decode(&cfg); err != nil {
		return
	}
	for _, c := range cfg.Menu {
		c.decode(res, []string{"Bios"})
	}
	return
}

func (s *superMicroConfig) FixWanted(wanted map[string]string) map[string]string {
	return wanted
}

func (s *superMicroConfig) Apply(current map[string]Entry, trimmed map[string]string) (needReboot bool, err error) {
	cfg := superMicroBios{Menu: []*superMicroMenu{}}
	cfg.XMLName.Local = "BiosCfg"
	menuEntries := map[string]*superMicroMenu{}
	for k, v := range trimmed {
		if !strings.HasPrefix(k, "Bios/") {
			continue
		}
		k = strings.TrimPrefix(k, "Bios/")
		menuName, settingName := path.Dir(k), path.Base(k)
		setting := &superMicroSetting{Name: settingName}
		setting.XMLName.Local = "Setting"
		curSetting, ok := current[k]
		if !ok {
			// No such current setting, skip it
			continue
		}
		switch curSetting.Type {
		case "Numeric":
			setting.NumericValue = v
		case "CheckBox":
			setting.Checked = v
		case "Option":
			setting.Option = v
		case "Password":
			setting.NewPassword = v
			setting.ConfirmNewPassword = v
		case "String":
			setting.String = v
		default:
			err = fmt.Errorf("Unknown type '%s' for setting '%s'", curSetting.Type, k)
			return
		}
		menu, ok := menuEntries[menuName]
		if !ok {
			menu = &superMicroMenu{
				Name:     path.Base(menuName),
				menu:     map[string]*superMicroMenu{},
				Settings: []*superMicroSetting{},
			}
			menuEntries[menuName] = menu
		}
		menu.Settings = append(menu.Settings, setting)
		for parentName := path.Dir(k); parentName != ""; parentName = path.Dir(parentName) {
			parent, ok := menuEntries[parentName]
			if !ok {
				parent = &superMicroMenu{
					Name: path.Base(parentName),
					menu: map[string]*superMicroMenu{},
				}
			}
			parent.menu[menu.Name] = menu
			menu = parent
		}
	}
	for k, v := range menuEntries {
		v.Menu = make([]*superMicroMenu, 0, len(v.menu))
		for _, vv := range v.menu {
			vv.XMLName.Local = "Menu"
			v.Menu = append(v.Menu, vv)
		}
		sort.Slice(v.Menu, func(i, j int) bool { return v.Menu[i].Name < v.Menu[j].Name })
		sort.Slice(v.Settings, func(i, j int) bool { return v.Settings[i].Name < v.Settings[j].Name })
		if !strings.Contains(k, "/") {
			cfg.Menu = append(cfg.Menu, v)
		}
	}
	sort.Slice(cfg.Menu, func(i, j int) bool { return cfg.Menu[i].Name < cfg.Menu[j].Name })
	var tgt *os.File
	tgt, err = os.Create("update.xml")
	if err != nil {
		err = errors.New(fmt.Sprintf("failed to create update.xml: %v", err))
		return
	}
	defer tgt.Close()
	enc := xml.NewEncoder(tgt)
	if err = enc.Encode(cfg); err != nil {
		err = errors.New(fmt.Sprintf("failed to encode update.xml: %v", err))
		return
	}
	tgt.Sync()
	cmd := exec.Command("sum", "-c", "ChangeBiosCfg", "--file", "update.xml")
	buf, err := cmd.CombinedOutput()
	if err != nil {
		err = errors.New(fmt.Sprintf("Supermicro failed to update: %s %v", string(buf), err))
		return
	}
	needReboot = true
	return
}
