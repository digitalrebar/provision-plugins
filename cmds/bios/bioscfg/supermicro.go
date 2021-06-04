package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
)

type superMicroInfo struct {
	XMLName xml.Name `xml:"Information"`
	// Only meaningful if Setting.type == "Option"
	Options       []string `xml:"AvailableOptions>Option"`
	DefaultOption string
	// Only meaningful when Setting.type == "Numeric"
	MaxValue     int
	MinValue     int
	StepSize     int
	DefaultValue int
	// Only meaningful when Setting.type == "CheckBox"
	DefaultStatus string
	// Only meaningful when Setting.type == "Password"
	HasPassword string
	// Meaningful when Setting.type == "Password" or "String"
	MinSize int
	MaxSize int
	// Only meaningful when Setting.type == "String"
	DefaultString        string
	AllowingMultipleLine string
}

type smString struct {
	XMLName xml.Name `xml:"StringValue"`
	Value   string   `xml:",cdata"`
}

type smNewPassword struct {
	XMLName xml.Name `xml:"NewPassword"`
	Value   string   `xml:",cdata"`
}

type smConfirmPassword struct {
	XMLName xml.Name `xml:"ConfirmNewPassword"`
	Value   string   `xml:",cdata"`
}

type superMicroSetting struct {
	XMLName            xml.Name           `xml:"Setting"`
	Name               string             `xml:"name,attr"`
	Type               string             `xml:"type,attr"`
	NumericValue       string             `xml:"numericValue,attr,omitempty"`
	Checked            string             `xml:"checkedStatus,attr,omitempty"`
	Option             string             `xml:"selectedOption,attr,omitempty"`
	String             *smString          `xml:"StringValue,omitempty"`
	NewPassword        *smNewPassword     `xml:",omitempty"`
	ConfirmNewPassword *smConfirmPassword `xml:",omitempty"`
	Information        *superMicroInfo    `xml:",omitempty"`
}

func (s *superMicroSetting) decode(res map[string]Entry, names []string) {
	name := strings.Join(append(names, s.Name), "::")
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
	case "String":
		ent.Current = s.String.Value
		fallthrough
	case "Password":
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

func (m *superMicroMenu) render() {
	if len(m.menu) == 0 {
		return
	}
	m.Menu = make([]*superMicroMenu, 0, len(m.menu))
	for _, v := range m.menu {
		m.Menu = append(m.Menu, v)
		v.render()
	}
	sort.Slice(m.Menu, func(i, j int) bool { return m.Menu[i].Name < m.Menu[j].Name })
	sort.Slice(m.Settings, func(i, j int) bool { return m.Settings[i].Name < m.Settings[j].Name })
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

func runSum(args ...string) error {
	cmd := exec.Command("/opt/bin/sum", args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error running sum: %v", err)
	}
	return nil
}

func (s *superMicroConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if s.source == nil {
		if err := runSum("-c", "GetCurrentBiosCfg", "--file", "config.xml", "--overwrite"); err != nil {
			return nil, err
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
	dec.CharsetReader = charset.NewReaderLabel
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

func splitMenuName(s string) (string, string) {
	sep := strings.LastIndex(s, "::")
	if sep == -1 {
		return "", s
	}
	return s[:sep], s[sep+2:]
}

func menuBase(s string) string {
	_, res := splitMenuName(s)
	return res
}

func menuPath(s string) string {
	res, _ := splitMenuName(s)
	return res
}

func (s *superMicroConfig) Apply(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	cfg := superMicroBios{
		Menu: []*superMicroMenu{},
		menu: map[string]*superMicroMenu{},
	}
	cfg.XMLName.Local = "BiosCfg"
	for k, v := range trimmed {
		curSetting, ok := current[k]
		if !ok {
			// No such current setting, skip it
			continue
		}
		if !strings.HasPrefix(k, "Bios::") {
			continue
		}
		k = strings.TrimPrefix(k, "Bios::")
		menuName, settingName := splitMenuName(k)
		setting := &superMicroSetting{Name: settingName}
		setting.XMLName.Local = "Setting"
		setting.Type = curSetting.Type
		switch curSetting.Type {
		case "Numeric":
			setting.NumericValue = v
		case "CheckBox":
			setting.Checked = v
		case "Option":
			setting.Option = v
		case "Password":
			setting.NewPassword = &smNewPassword{
				XMLName: xml.Name{Local: "NewPassword"},
				Value:   v,
			}
			setting.ConfirmNewPassword = &smConfirmPassword{
				XMLName: xml.Name{Local: "ConfirmNewPassword"},
				Value:   v,
			}
		case "String":
			setting.String = &smString{XMLName: xml.Name{Local: "StringValue"}, Value: v}
		default:
			err = fmt.Errorf("Unknown type '%s' for setting '%s'", curSetting.Type, k)
			return
		}
		menuEntries := cfg.menu
		var menu *superMicroMenu

		for _, k := range strings.Split(menuName, "::") {
			menu, ok = menuEntries[k]
			if !ok {
				menu = &superMicroMenu{
					XMLName:  xml.Name{Local: "Menu"},
					Name:     k,
					menu:     map[string]*superMicroMenu{},
					Settings: []*superMicroSetting{},
				}
				menuEntries[k] = menu
			}
			menuEntries = menu.menu
		}
		menu.Settings = append(menu.Settings, setting)
	}
	cfg.Menu = make([]*superMicroMenu, 0, len(cfg.menu))
	for _, v := range cfg.menu {
		v.render()
		cfg.Menu = append(cfg.Menu, v)
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
	if dryRun {
		return
	}
	if err = runSum("-c", "ChangeBiosCfg", "--file", "update.xml"); err != nil {
		err = fmt.Errorf("Supermicro failed to update: %v", err)
		return
	}
	needReboot = true
	return
}
