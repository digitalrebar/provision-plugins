package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"github.com/VictorLowther/jsonpatch2/utils"

	"github.com/ghodss/yaml"
)

type IsoData struct {
	Title            string   `json:"title"`
	Iso              string   `json:"iso"`
	Version          string   `json:"version"`
	Build            string   `json:"build"`
	Author           string   `json:"author"`
	Vendor           string   `json:"vendor"`
	Subvendor        string   `json:"subvendor"`
	Isourl           string   `json:"isourl"`
	Sha256sum        string   `json:"sha256sum,omitempty"`
	Bootcfg          string   `json:"bootcfg,omitempty"`
	Kernel           string   `json:"kernel,omitempty"`
	Sourcebundleurls []string `json:"sourcebundleurls,omitempty"`
	Bundleurls       []string `json:"bundleurls,omitempty"`
	Description      string   `json:"description"`
}

var force = false
var forceBootCfg = false

type IsoDatas []*IsoData

func (ii IsoDatas) Len() int {
	return len(ii)
}

func (ii IsoDatas) Less(i, j int) bool {
	id := ii[i]
	jd := ii[j]

	if id.Version == jd.Version {
		if id.Vendor != jd.Vendor {
			if id.Subvendor != jd.Subvendor {
				return id.Author < jd.Author
			}
			return id.Subvendor < jd.Subvendor
		}
		return id.Vendor < jd.Vendor
	}
	return id.Version < jd.Version
}

func (ii IsoDatas) Swap(i, j int) {
	t := ii[i]
	ii[i] = ii[j]
	ii[j] = t
}

// elements of the collection be enumerated by an integer index.
type Interface interface {
	// Len is the number of elements in the collection.
	Len() int
	// Less reports whether the element with
	// index i should sort before the element with index j.
	Less(i, j int) bool
	// Swap swaps the elements with indexes i and j.
	Swap(i, j int)
}

var httpClient *http.Client

func getBootcfg(id *IsoData, data []byte, sha256sum string) (string, string, error) {

	err := ioutil.WriteFile("/tmp/t.iso", data, 0644)
	if err != nil {
		return "", "", err
	}
	defer os.Remove("/tmp/t.iso")

	err = exec.Command("bsdtar", "-xf", "/tmp/t.iso", "BOOT.CFG").Run()
	if err != nil {
		return "", "", err
	}
	defer os.Remove("BOOT.CFG")

	file, err := os.Open("BOOT.CFG")
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	kernel := ""
	bootcfg := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "kernel=") {
			line = strings.TrimPrefix(line, "kernel=")
			line = strings.ReplaceAll(line, "/", "")
			kernel = line
		}
		if strings.HasPrefix(line, "modules=") {
			line = strings.TrimPrefix(line, "modules=")
			line = strings.ReplaceAll(line, "/", "")

			if id.Author == "rackn" {
				line = strings.Replace(line, " --- s.v00", " --- s.v00{{ range $key := .Param \"esxi/boot-cfg-extra-modules\" }} --- {{$key}}{{ end }}", 1)
			} else {
				line = strings.Replace(line, " --- s.v00", " --- {{ if (.Param \"esxi/add-drpy-firewall\") }}{{ .Param \"esxi/add-drpy-firewall\" }} --- {{end}}{{ if (.Param \"esxi/add-drpy-agent\") }}{{ .Param \"esxi/add-drpy-agent\" }} --- {{end}}s.v00{{ range $key := .Param \"esxi/boot-cfg-extra-modules\" }} --- {{$key}}{{ end }}", 1)
			}
			line = strings.Replace(line, " --- tools.t00", "{{ if eq (.Param \"esxi/skip-tools\") false }} --- tools.t00{{end}}", 1)

			bootcfg = line
		}
		if kernel != "" && bootcfg != "" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	return kernel, bootcfg, nil
}

func summedNoBuffer(src string) (string, error) {
	resp, err := httpClient.Head(src)
	if err != nil {
		return "NOTFOUND", nil
	}
	defer resp.Body.Close()
	shasum := resp.Header.Get("X-Amz-Meta-Sha256")
	if shasum == "" {
		s, _, e := summedBuffer(src)
		return s, e
	}
	return shasum, nil
}

func summedBuffer(src string) (string, []byte, error) {
	resp, err := httpClient.Get(src)
	if err != nil {
		return "NOTFOUND", nil, nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to get %s: %v", src, err)
	}

	sum := sha256.Sum256(body)
	shasum := fmt.Sprintf("%x", sum)

	return shasum, body, nil
}

func main() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient = &http.Client{Transport: tr}

	data, err := ioutil.ReadFile("content/params/esxi-iso-catalog.yaml")
	if err != nil {
		fmt.Printf("Failed to read esxi/iso-catalog: %v", err)
		return
	}

	d := map[string]interface{}{}
	err = yaml.Unmarshal(data, &d)
	if err != nil {
		fmt.Printf("Failed to marshal esxi/iso-catalog: %v", err)
		return
	}

	d2 := map[string]interface{}{}
	err = utils.Remarshal(d["Schema"], &d2)
	if err != nil {
		fmt.Printf("Failed to marshal schema: %v", err)
		return
	}

	datas := IsoDatas{}
	err = utils.Remarshal(d2["default"], &datas)
	if err != nil {
		fmt.Printf("Failed to marshal schema: %v", err)
		return
	}

	updated := false
	error := false
	for _, id := range datas {
		modTitle := ""
		mod := ""
		if id.Subvendor != "none" {
			modTitle = "_" + id.Subvendor
			mod = " (" + id.Subvendor + ")"
		}
		rackn := ""
		racknTitle := ""
		if id.Author == "rackn" {
			racknTitle = "rkn_"
			rackn = "rkn_"
		}

		if id.Title == "" || force {
			updated = true
			id.Title = fmt.Sprintf("esxi_%s-%s_%s%s%s", id.Version, id.Build, racknTitle, id.Vendor, modTitle)
		}

		if id.Description == "" || force {
			updated = true
			id.Description = fmt.Sprintf("ESXi %s-%s for %s%s%s", id.Version, id.Build, rackn, id.Vendor, mod)
		}

		if id.Iso == "" {
			fmt.Printf("Missing iso name! %v\n", id)
			error = true
			continue
		}

		needBootcfg := false
		needSha256sum := false
		if id.Bootcfg == "unset" || id.Bootcfg == "" {
			needBootcfg = true
		}
		if id.Sha256sum == "unset" || id.Sha256sum == "" {
			needSha256sum = true
		}

		isoUrl := id.Isourl
		if len(id.Bundleurls) == 0 {
			isoUrl = fmt.Sprintf("https://s3-us-west-2.amazonaws.com/get.rebar.digital/images/vmware/esxi/%s", id.Iso)
		}

		if needBootcfg || needSha256sum || forceBootCfg {
			sha256sum, idata, err := summedBuffer(isoUrl)

			if err != nil {
				fmt.Printf("Failed to download iso: %s: %v\n", isoUrl, err)
				error = true
				continue
			}

			k, b, err := getBootcfg(id, idata, sha256sum)
			if err != nil {
				fmt.Printf("Failed to process iso: %s: %v\n", isoUrl, err)
				error = true
				continue
			}

			id.Kernel = k
			id.Bootcfg = b
			id.Sha256sum = sha256sum
			updated = true
		}
	}

	sort.Sort(datas)

	if updated {
		d2["default"] = datas
		d["Schema"] = d2

		bdata, err := yaml.Marshal(d)
		if err != nil {
			fmt.Printf("Failed marshal output: %s\n", err)
			error = true
		} else {
			err := ioutil.WriteFile("content/params/esxi-iso-catalog.yaml.2", bdata, 0644)
			if err != nil {
				fmt.Printf("Failed write output: %s\n", err)
				error = true
			}
			os.Rename("content/params/esxi-iso-catalog.yaml.2", "content/params/esxi-iso-catalog.yaml")
		}
	}

	// build bootenvs
	for _, id := range datas {

		tmpl := template.New("bootenv").Option("missingkey=error")
		tmpl, err = tmpl.Parse(bootEnvTemplate)
		if err != nil {
			fmt.Printf("Failed template bootenv %s: %s\n", id.Title, err)
			error = true
			continue
		}

		buf2 := &bytes.Buffer{}
		err = tmpl.Execute(buf2, id)
		if err != nil {
			fmt.Printf("Failed template expand bootenv %s: %s\n", id.Title, err)
			error = true
			continue
		}
		filename := fmt.Sprintf("content/bootenvs/%s.yaml", id.Title)
		err = ioutil.WriteFile(filename, buf2.Bytes(), 0644)
		if err != nil {
			fmt.Printf("Failed write template expand bootenv %s: %s\n", id.Title, err)
			error = true
			continue
		}

		tmpl = template.New("template").Option("missingkey=error")
		tmpl, err = tmpl.Parse(bootCfgTmpl)
		if err != nil {
			fmt.Printf("Failed template template %s: %s\n", id.Title, err)
			error = true
			continue
		}

		buf2 = &bytes.Buffer{}
		err = tmpl.Execute(buf2, id)
		if err != nil {
			fmt.Printf("Failed template expand template %s: %s\n", id.Title, err)
			error = true
			continue
		}
		filename = fmt.Sprintf("content/templates/%s.boot.cfg.tmpl", id.Title)
		err = ioutil.WriteFile(filename, buf2.Bytes(), 0644)
		if err != nil {
			fmt.Printf("Failed write template expand bootenv %s: %s\n", id.Title, err)
			error = true
			continue
		}
	}

	if error {
		os.Exit(1)
	}
	os.Exit(0)
}

var bootEnvTemplate = `---
Name: {{.Title}}-install
Description: Install BootEnv for {{.Description}}
Documentation: |
  Provides VMware BootEnv for {{.Description}}
  For more details, and to download ISO see:

    - {{.Isourl}}

  NOTE: The ISO filename and sha256sum must match this BootEnv exactly.

Meta:
  color: blue
  icon: zip
  title: RackN Content
OS:
  Codename: esxi
  Family: vmware
  IsoFile: {{.Iso}}
  IsoSha256: {{.Sha256sum}}
  IsoUrl: ""
  Name: {{.Title}}
  SupportedArchitectures: {}
  Version: {{.Version}}
OnlyUnknown: false
OptionalParams:
  - provisioner-default-password-hash
RequiredParams: []
Kernel: ../../chain.c32
{{"BootParams: -c {{.Machine.Path}}/boot.cfg"}}
Initrds: []
Loaders:
  amd64-uefi: efi/boot/bootx64.efi
Templates:
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux
{{"    Path: pxelinux.cfg/{{.Machine.HexAddress}}"}}
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux-mac
{{"    Path: pxelinux.cfg/{{.Machine.MacAddr \"pxelinux\"}}"}}
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe
{{"    Path: '{{.Machine.Address}}.ipxe'"}}
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe-mac
{{"    Path: '{{.Machine.MacAddr \"ipxe\"}}.ipxe'"}}
  - ID: esxi-install-py3.ks.tmpl
    Name: compute.ks
{{"    Path: '{{.Machine.Path}}/compute.ks'"}}
  - ID: {{.Title}}.boot.cfg.tmpl
    Name: boot.cfg
{{"    Path: '{{.Machine.Path}}/boot.cfg'"}}
  - ID: {{.Title}}.boot.cfg.tmpl
    Name: boot-uefi.cfg
{{"    Path: '{{.Env.PathForArch \"tftp\" \"\" \"amd64\"}}/efi/boot/{{.Machine.MacAddr \"pxelinux\"}}/boot.cfg'"}}
`

var bootCfgTmpl = `bootstate=0
title=Loading ESXi installer for {{.Title}}
timeout=2
{{"prefix=/{{ trimSuffix \"/\" (.Env.PathFor \"tftp\" \"/\") }}"}}
kernel={{.Kernel}}
{{"kernelopt=ks={{.Machine.Url}}/compute.ks{{if .ParamExists \"kernel-options\"}} {{.Param \"kernel-options\"}}{{end}}{{if .ParamExists \"esxi/serial-console\"}} {{.Param \"esxi/serial-console\"}}{{end}}"}}
build=
updated=0
{{"{{ if eq (.Param \"esxi/set-norts\") true }}norts=1{{ end }}"}}
{{"{{ if .ParamExists \"esxi/boot-cfg-extra-options\" }}{{ .Param \"esxi/boot-cfg-extra-options\" }}{{ end }}"}}
modules={{.Bootcfg}}
`
