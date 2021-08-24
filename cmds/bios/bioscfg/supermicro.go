package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/net/html/charset"
)

type superMicroBiosInfo struct {
	XMLName xml.Name `xml:"Information"`
	// Only meaningful if Setting.type == "Option"
	Options       []string `xml:"AvailableOptions>Option"`
	DefaultOption string
	// Only meaningful when Setting.type == "Numeric"
	MaxValue     *big.Int
	MinValue     *big.Int
	StepSize     *big.Int
	DefaultValue *big.Int
	// Only meaningful when Setting.type == "CheckBox"
	DefaultStatus string
	// Only meaningful when Setting.type == "Password"
	HasPassword string
	// Meaningful when Setting.type == "Password" or "String"
	MinSize *big.Int
	MaxSize *big.Int
	// Only meaningful when Setting.type == "String"
	DefaultString        string
	AllowingMultipleLine string
}

type smBiosString struct {
	XMLName xml.Name `xml:"StringValue"`
	Value   string   `xml:",cdata"`
}

type smBiosNewPassword struct {
	XMLName xml.Name `xml:"NewPassword"`
	Value   string   `xml:",cdata"`
}

type smBiosConfirmPassword struct {
	XMLName xml.Name `xml:"ConfirmNewPassword"`
	Value   string   `xml:",cdata"`
}

type superMicroBiosSetting struct {
	XMLName            xml.Name               `xml:"Setting"`
	Name               string                 `xml:"name,attr"`
	Type               string                 `xml:"type,attr"`
	NumericValue       string                 `xml:"numericValue,attr,omitempty"`
	Checked            string                 `xml:"checkedStatus,attr,omitempty"`
	Option             string                 `xml:"selectedOption,attr,omitempty"`
	String             *smBiosString          `xml:"StringValue,omitempty"`
	NewPassword        *smBiosNewPassword     `xml:",omitempty"`
	ConfirmNewPassword *smBiosConfirmPassword `xml:",omitempty"`
	Information        *superMicroBiosInfo    `xml:",omitempty"`
}

func (s *superMicroBiosSetting) decode(res map[string]Entry, names []string) {
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
			ent.Default = s.Information.DefaultValue.String()
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

type superMicroBiosMenu struct {
	XMLName  xml.Name              `xml:"Menu"`
	Name     string                `xml:"name,attr"`
	Menu     []*superMicroBiosMenu `xml:"Menu"`
	menu     map[string]*superMicroBiosMenu
	Settings []*superMicroBiosSetting `xml:"Setting"`
}

func (m *superMicroBiosMenu) decode(res map[string]Entry, names []string) {
	sub := append(names, m.Name)
	for _, s := range m.Settings {
		s.decode(res, sub)
	}
	for _, mm := range m.Menu {
		mm.decode(res, sub)
	}
}

func (m *superMicroBiosMenu) render() {
	if len(m.menu) == 0 {
		return
	}
	m.Menu = make([]*superMicroBiosMenu, 0, len(m.menu))
	for _, v := range m.menu {
		m.Menu = append(m.Menu, v)
		v.render()
	}
	sort.Slice(m.Menu, func(i, j int) bool { return m.Menu[i].Name < m.Menu[j].Name })
	sort.Slice(m.Settings, func(i, j int) bool { return m.Settings[i].Name < m.Settings[j].Name })
}

type superMicroBios struct {
	XMLName xml.Name              `xml:"BiosCfg"`
	Menu    []*superMicroBiosMenu `xml:"Menu"`
	menu    map[string]*superMicroBiosMenu
}

type smBmcValNode struct {
	name     xml.Name
	attrs    map[string]string
	v        string
	children []*smBmcValNode
}

func newBmcNode() *smBmcValNode {
	return &smBmcValNode{
		attrs:    map[string]string{},
		children: []*smBmcValNode{},
	}
}

func (s *smBmcValNode) toAttr() []xml.Attr {
	res := make([]xml.Attr, 0, len(s.attrs))
	for k, v := range s.attrs {
		res = append(res, xml.Attr{
			Name:  xml.Name{Local: k},
			Value: v,
		})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Name.Local < res[j].Name.Local
	})
	return res
}

func (s *smBmcValNode) settingName() string {
	parts := []string{s.name.Local}
	for _, attr := range s.toAttr() {
		if attr.Name.Local == "Action" {
			continue
		}
		parts = append(parts, attr.Name.Local+"="+attr.Value)
	}
	return strings.Join(parts, " ")
}

func (s *smBmcValNode) setName(n string) {
	parts := strings.Split(n, " ")
	s.name.Local = parts[0]
	for _, attr := range parts[1:] {
		av := strings.SplitN(attr, "=", 2)
		s.attrs[av[0]] = av[1]
	}
}

//Setting groups that will need special handling:
// BmcCfg::OemCfg::AlertList, for adding/removing Alert configs
// BmcCfg::OemCfg::AD:: for adding/changing/removing ADGroup configs
// BmcCfg::OemCfg::IPAccessControl, for adding/removing ControlRule configs

var nonIdempotentGroups = map[string]struct{}{
	`BmcCfg::OemCfg::AlertList::`: {},
	`BmcCfg::OemCfg::AD::`: {},
	`BmcCfg::OemCfg::IPAccessControl::`: {},
}

func isIgnored(s string) bool {
	for k := range nonIdempotentGroups {
		if strings.HasPrefix(s,k) {
			return true
		}
	}
	return false
}


func (s *smBmcValNode) decode(res map[string]Entry, names []string, readonly bool) {
	if len(s.children) == 0 {
		name := strings.Join(append(names, s.settingName()), "::")
		if isIgnored(name) {
			return
		}
		ent := Entry{
			Type:     "String",
			Name:     name,
			ReadOnly: readonly,
			Current:  s.v,
		}
		res[ent.Name] = ent
		return
	}
	names = append(names, s.settingName())
	for _, val := range s.children {
		val.decode(res, names, s.name.Local == "Information")
	}
}

func (s *smBmcValNode) UnmarshalXML(dec *xml.Decoder, elem xml.StartElement) error {
	s.name = elem.Name
	for _, attr := range elem.Attr {
		if attr.Name.Local == "Action" {
			continue
		}
		s.attrs[attr.Name.Local] = attr.Value
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch rt := tok.(type) {
		case xml.Comment:
			continue
		case xml.ProcInst:
			continue
		case xml.EndElement:
			sort.Slice(s.children, func(i, j int) bool {
				return s.children[i].name.Local < s.children[j].name.Local
			})
			return nil
		case xml.CharData:
			buf := bytes.TrimSpace(rt)
			if len(buf) > 0 {
				s.v = string(rt)
			}
		case xml.Directive:
			buf := bytes.TrimSpace(rt)
			if !(bytes.HasPrefix(buf, []byte("[CDATA[")) && bytes.HasSuffix(buf, []byte("]]"))) {
				return fmt.Errorf("Encountered non-CDATA XML directive")
			}
			s.v = strings.TrimSpace(string(buf[7 : len(buf)-2]))
		case xml.StartElement:
			child := newBmcNode()
			err = child.UnmarshalXML(dec, rt)
			if err != nil {
				return err
			}
			s.children = append(s.children, child)
		}
	}
}

func (s *smBmcValNode) MarshalXML(enc *xml.Encoder, _ xml.StartElement) (err error) {
	if err = enc.EncodeToken(xml.StartElement{Name: s.name, Attr: s.toAttr()}); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			return
		}
		err = enc.EncodeToken(xml.EndElement{Name: s.name})
	}()
	if len(s.children) == 0 {
		val := []byte(s.v)
		err = enc.EncodeToken(xml.CharData(val))
		return
	}
	for _, val := range s.children {
		if err = val.MarshalXML(enc, xml.StartElement{}); err != nil {
			return
		}
	}
	return
}

type superMicroConfig struct {
	licenseKey string
	source     io.Reader
}

var commentStart = []byte(`<!--`)
var commentEnd = []byte(`-->`)

// sigh
func ignoreBrokenXMLComments(src io.Reader) io.Reader {
	buf, err := io.ReadAll(src)
	if err != nil {
		log.Panicf("Error reading all: %v", err)
	}
	segments := []io.Reader{}
	for {
		start := bytes.Index(buf, commentStart)
		if start == -1 {
			segments = append(segments, bytes.NewBuffer(buf[:]))
			break
		}
		segments = append(segments, bytes.NewBuffer(buf[:start]))
		end := bytes.Index(buf[start+len(commentStart):], commentEnd)
		if end == -1 {
			break
		}
		buf = buf[start+len(commentStart)+end+len(commentEnd):]
	}
	return io.MultiReader(segments...)
}

func (s *superMicroConfig) getBMC(src io.Reader, res map[string]Entry) (err error) {
	dec := xml.NewDecoder(ignoreBrokenXMLComments(src))
	dec.CharsetReader = charset.NewReaderLabel
	dec.Strict = false
	rootNode := newBmcNode()
	err = dec.Decode(rootNode)
	if err == nil && rootNode.name.Local == "BmcCfg" {
		rootNode.decode(res, []string{}, false)
	}
	return
}

func (s *superMicroConfig) getBios(src io.Reader, res map[string]Entry) error {
	cfg := superMicroBios{}
	dec := xml.NewDecoder(ignoreBrokenXMLComments(src))
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&cfg); err != nil {
		return err
	}
	if len(cfg.Menu) == 0 {
		return nil
	}
	for _, c := range cfg.Menu {
		c.decode(res, []string{"Bios"})
	}
	return nil
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
		if err := runSum("-c", "GetCurrentBiosCfg", "--file", "BIOSCfg.xml", "--overwrite"); err != nil {
			return nil, err
		}
		defer os.Remove("BIOSCfg.xml")
		var w *os.File
		w, err = os.Open("BIOSCfg.xml")
		if err != nil {
			return
		}
		defer w.Close()
		if err = s.getBios(w, res); err != nil {
			return
		}
		w.Close()
		if err := runSum(`-c`, `GetBmcCfg`, `--file`, `BMCCfg.xml`, `--overwrite`); err != nil {
			return nil, err
		}
		defer os.Remove("BMCCfg.xml")
		w, err = os.Open("BMCCfg.xml")
		if err != nil {
			return
		}
		err = s.getBMC(w, res)
		return
	}
	if err = s.getBios(s.source, res); err != nil {
		if seek, ok := s.source.(*os.File); ok {
			seek.Seek(0, io.SeekStart)
		}
		err = s.getBMC(s.source, res)
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

func (s *superMicroConfig) applyBios(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	cfg := superMicroBios{
		Menu: []*superMicroBiosMenu{},
		menu: map[string]*superMicroBiosMenu{},
	}
	cfg.XMLName.Local = "BiosCfg"
	for k, v := range trimmed {
		curSetting, ok := current[k]
		if !ok {
			// No such current setting, skip it
			continue
		}
		k = strings.TrimPrefix(k, "Bios::")
		menuName, settingName := splitMenuName(k)
		setting := &superMicroBiosSetting{Name: settingName}
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
			setting.NewPassword = &smBiosNewPassword{
				XMLName: xml.Name{Local: "NewPassword"},
				Value:   v,
			}
			setting.ConfirmNewPassword = &smBiosConfirmPassword{
				XMLName: xml.Name{Local: "ConfirmNewPassword"},
				Value:   v,
			}
		case "String":
			setting.String = &smBiosString{XMLName: xml.Name{Local: "StringValue"}, Value: v}
		default:
			err = fmt.Errorf("Unknown type '%s' for setting '%s'", curSetting.Type, k)
			return
		}
		menuEntries := cfg.menu
		var menu *superMicroBiosMenu

		for _, k := range strings.Split(menuName, "::") {
			menu, ok = menuEntries[k]
			if !ok {
				menu = &superMicroBiosMenu{
					XMLName:  xml.Name{Local: "Menu"},
					Name:     k,
					menu:     map[string]*superMicroBiosMenu{},
					Settings: []*superMicroBiosSetting{},
				}
				menuEntries[k] = menu
			}
			menuEntries = menu.menu
		}
		menu.Settings = append(menu.Settings, setting)
	}
	if len(cfg.menu) == 0 {
		return
	}
	cfg.Menu = make([]*superMicroBiosMenu, 0, len(cfg.menu))
	for _, v := range cfg.menu {
		v.render()
		cfg.Menu = append(cfg.Menu, v)
	}
	sort.Slice(cfg.Menu, func(i, j int) bool { return cfg.Menu[i].Name < cfg.Menu[j].Name })
	var tgt *os.File
	tgt, err = os.Create("updateBios.xml")
	if err != nil {
		err = fmt.Errorf("failed to create updateBios.xml: %v", err)
		return
	}
	defer tgt.Close()
	enc := xml.NewEncoder(tgt)
	if err = enc.Encode(cfg); err != nil {
		err = fmt.Errorf("failed to encode updateBios.xml: %v", err)
		return
	}
	tgt.Sync()
	if dryRun {
		return
	}
	if err = runSum("-c", "ChangeBiosCfg", "--file", "updateBios.xml"); err != nil {
		err = fmt.Errorf("Supermicro failed to update: %v", err)
		return
	}
	needReboot = true
	return
}

func (s *superMicroConfig) applyBmc(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	cfg := newBmcNode()
	cfg.name.Local = "BmcCfg"
	var working *smBmcValNode
	ignored := []string{}
	for k, v := range trimmed {
		currSetting, ok := current[k]
		if ok && (currSetting.Current == v || currSetting.ReadOnly) {
			continue
		}
		if isIgnored(k) {
			ignored = append(ignored, k)
			continue
		}
		parent := cfg
		parts := strings.Split(k, "::")
		for _, v := range parts[1:] {
			idx := sort.Search(len(parent.children), func(i int) bool {
				return parent.children[i].settingName() >= v
			})
			if idx == len(parent.children) {
				working = newBmcNode()
				working.setName(v)
				parent.children = append(parent.children, working)
			} else if parent.children[idx].settingName() == v {
				parent = parent.children[idx]
				continue
			} else {
				working = newBmcNode()
				working.setName(v)
				parent.children = append(parent.children, nil)
				copy(parent.children[idx+1:], parent.children[idx:])
				parent.children[idx] = working
			}
			if v == "Configuration" {
				parent.attrs["Action"] = "Change"
			}
			parent = working
		}
		if working == nil {
			err = fmt.Errorf("No value to set for %s", k)
			return
		}
		working.v = v
	}
	if len(cfg.children) == 0 {
		return
	}
	for _, child := range cfg.children {
		child.attrs["Action"] = "Change"
	}
	var tgt *os.File
	tgt, err = os.Create("updateBmc.xml")
	if err != nil {
		err = fmt.Errorf("failed to create updateBmc.xml: %v", err)
		return
	}
	defer tgt.Close()
	enc := xml.NewEncoder(tgt)
	if err = enc.Encode(cfg); err != nil {
		err = fmt.Errorf("failed to encode updateBmc.xml: %v", err)
		return
	}
	tgt.Sync()
	if dryRun {
		return
	}
	if err = runSum("-c", "ChangeBmcCfg", "--file", "updateBmc.xml"); err != nil {
		err = fmt.Errorf("Supermicro failed to update: %v", err)
		return
	}
	needReboot = true
	if len(ignored) > 0 {
		sort.Strings(ignored)
		fmt.Fprintf(os.Stderr, "Non-idempotent settings ignored:\n")
		for _,line := range ignored {
			fmt.Fprintf(os.Stderr, "    %s\n",line)
		}
		fmt.Fprintf(os.Stderr, "You will need to handle these settings using SuperMicro Update Manager in a custom" +
			" task.\n")
	}
	return
}

func (s *superMicroConfig) Apply(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	biosCurrent, biosTrimmed := map[string]Entry{}, map[string]string{}
	bmcCurrent, bmcTrimmed := map[string]Entry{}, map[string]string{}
	for k, v := range current {
		if strings.HasPrefix(k, "Bios::") {
			biosCurrent[k] = v
			continue
		}
		if strings.HasPrefix(k, "BmcCfg::") {
			bmcCurrent[k] = v
		}
	}
	for k, v := range trimmed {
		if strings.HasPrefix(k, "Bios::") {
			biosTrimmed[k] = v
			continue
		}
		if strings.HasPrefix(k, "BmcCfg::") {
			bmcTrimmed[k] = v
		}
	}
	needReboot, err = s.applyBios(biosCurrent, biosTrimmed, dryRun)
	if needReboot || err != nil {
		return
	}
	needReboot, err = s.applyBmc(bmcCurrent, bmcTrimmed, dryRun)
	return
}
