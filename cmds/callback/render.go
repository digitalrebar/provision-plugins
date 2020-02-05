package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"text/template"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	yaml "gopkg.in/yaml.v2"
)

type rMachine struct {
	*models.Machine
	renderData *RenderData
	currMac    string
}

// RenderData is the struct that is passed to templates as a source of
// parameters and useful methods.
type RenderData struct {
	Machine   *rMachine // The Machine that the template is being rendered for.
	info      *models.Info
	drpClient *api.Client
	l         logger.Logger
	auths     map[string]*auth
}

// HexAddress returns Address in raw hexadecimal format, suitable for
// pxelinux and elilo usage.
func (n *rMachine) HexAddress() string {
	return models.Hexaddr(n.Address)
}

// ShortName returns the Hostanme part of an FQDN or the Name if not dot is present
func (n *rMachine) ShortName() string {
	idx := strings.Index(n.Name, ".")
	if idx == -1 {
		return n.Name
	}
	return n.Name[:idx]
}

// Path return the file space path for the machine
// e.g. machines/133-355-3-36-36 <vaild UUID>
func (n *rMachine) Path() string {
	return path.Join(n.Prefix(), n.UUID())
}

// HasProfile returns true if the machine has the specified profile.
func (n *rMachine) HasProfile(name string) bool {
	for _, e := range n.Profiles {
		if e == name {
			return true
		}
	}
	return false
}

func (n *rMachine) Url() string {
	return fmt.Sprintf("http://%s:%d/%s", n.renderData.info.Address, n.renderData.info.FilePort, n.Path())
}

func (n *rMachine) MacAddr(params ...string) string {
	format := "raw"
	if len(params) > 0 {
		format = params[0]
	}
	switch format {
	case "pxelinux":
		return "01-" + strings.Replace(n.currMac, ":", "-", -1)
	case "rpi4":
		return strings.Replace(n.currMac, ":", "-", -1)
	default:
		return n.currMac
	}
}

// ProvisionerAddress returns the IP address to access
// the Provisioner based upon the requesting IP address.
func (r *RenderData) ProvisionerAddress() string {
	return r.info.Address.String()
}

// ProvisionerURL returns a URL to access the
// file server part of the server using the
// requesting IP address as a basis.
func (r *RenderData) ProvisionerURL() string {
	return fmt.Sprintf("http://%s:%d", r.info.Address, r.info.FilePort)
}

// ApiURL returns a URL to access the
// api server part of the server using the
// requesting IP address as a basis.
func (r *RenderData) ApiURL() string {
	return fmt.Sprintf("http://%s:%d", r.info.Address, r.info.ApiPort)
}

// Info returns a *models.Info structure
func (r *RenderData) Info() *models.Info {
	i := *r.info
	return &i
}

// ParseUrl is a template function that return the section
// of the specified URL as a string.
func (r *RenderData) ParseUrl(segment, rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	switch segment {
	case "scheme":
		return parsedURL.Scheme, nil
	case "host":
		return parsedURL.Host, nil
	case "path":
		return parsedURL.Path, nil
	}
	return "", fmt.Errorf("No idea how to get URL part %s from %s", segment, rawURL)
}

// Param is a helper function for extracting a parameter from Machine.Params
func (r *RenderData) Param(key string) (interface{}, error) {
	if r.Machine != nil {
		v, ok := r.Machine.Params[key]
		if ok {
			return v, nil
		}
	}
	res := &models.Param{}
	rr := r.drpClient.Req().UrlFor("params", key)
	if derr := rr.Do(&res); derr != nil {
		return nil, nil
	}
	v, _ := res.DefaultValue()
	return v, nil
}

// ParamExpand does templating on the contents of a string parameter before returning it
func (r *RenderData) ParamExpand(key string) (interface{}, error) {
	sobj, err := r.Param(key)
	if err != nil {
		return nil, err
	}
	s, ok := sobj.(string)
	if !ok {
		return sobj, nil
	}

	res := &bytes.Buffer{}
	tmpl, err := template.New("machine").Funcs(models.DrpSafeFuncMap()).Parse(s)
	if err != nil {
		return nil, fmt.Errorf("Error compiling parameter %s: %v", key, err)
	}
	tmpl = tmpl.Option("missingkey=error")
	if err := tmpl.Execute(res, r); err != nil {
		return nil, fmt.Errorf("Error rendering parameter %s: %v", key, err)
	}
	return res.String(), nil
}

// ParamAsJSON will return the specified parameter as a JSON
// string or an error.
func (r *RenderData) ParamAsJSON(key string) (string, error) {
	v, err := r.Param(key)
	if err != nil {
		return "", err
	}
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	err = enc.Encode(v)
	return buf.String(), err
}

// ParamAsYAML will return the specified parameter as a YAML
// string or an error.
func (r *RenderData) ParamAsYAML(key string) (string, error) {
	v, err := r.Param(key)
	if err != nil {
		return "", err
	}
	b, e := yaml.Marshal(v)
	if e != nil {
		return "", e
	}
	return string(b), nil
}

// ParamExists is a helper function for determining the existence of a machine parameter.
func (r *RenderData) ParamExists(key string) bool {
	_, err := r.Param(key)
	return err == nil
}

func (r *RenderData) GetAuthString(authName string) (string, error) {
	auth, ok := r.auths[authName]
	if !ok {
		return "", fmt.Errorf("Failed to find %s in auths for plugin", authName)
	}
	return getExecString(r.l, auth)
}

func Render(l logger.Logger, auths map[string]*auth, drpClient *api.Client, data string, machine *models.Machine, info *models.Info) (string, error) {
	tmpl, err := template.New("tmp").Funcs(models.DrpSafeFuncMap()).Parse(data)
	if err != nil {
		return "", err
	}

	currMac := ""
	if len(machine.HardwareAddrs) > 0 {
		currMac = machine.HardwareAddrs[0]
	}

	// GREG: fix info.Address to non-loopback

	if auths == nil {
		auths = map[string]*auth{}
	}

	rd := &RenderData{
		l:         l,
		auths:     auths,
		Machine:   &rMachine{Machine: machine, currMac: currMac},
		info:      info,
		drpClient: drpClient,
	}
	rd.Machine.renderData = rd

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, rd); err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}
