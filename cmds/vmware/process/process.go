package main

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/VictorLowther/jsonpatch2/utils"

	"github.com/ghodss/yaml"
)

type IsoData struct {
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
}

type IsoDatas []*IsoData

var httpClient *http.Client

func getBootcfg(data []byte, sha256sum string) (string, string, error) {

	err := ioutil.WriteFile("/tmp/t.iso", data, 0755)
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

			line = strings.Replace(line, " --- s.v00", " --- s.v00{{ range $key := .Param \"esxi/boot-cfg-extra-modules\" }} --- {{$key}}{{end}}", 1)
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

		if needBootcfg || needSha256sum {
			sha256sum, idata, err := summedBuffer(isoUrl)

			if err != nil {
				fmt.Printf("Failed to download iso: %s: %v\n", isoUrl, err)
				error = true
				continue
			}

			k, b, err := getBootcfg(idata, sha256sum)
			if err != nil {
				fmt.Printf("Failed to process iso: %s: %v\n", isoUrl, err)
				error = true
				continue
			}

			id.Kernel = k
			id.Bootcfg = b
			id.Sha256sum = sha256sum
			updated = true

			// GREG: one day set sha256sum header on the file in aws
			/*
				} else {
					sha256sum, err := summedNoBuffer(isoUrl)

					if err != nil {
						fmt.Printf("Failed to download iso: %s\n", isoUrl)
						error = true
						continue
					}

					if sha256sum != id.Sha256sum {
						fmt.Printf("Sha256sum doesn't match: %s != %s: %s\n", id.Sha256sum, sha256sum, isoUrl)
						error = true
						continue
					}
			*/
		}
	}

	if updated {
		d2["default"] = datas
		d["Schema"] = d2

		bdata, err := yaml.Marshal(d)
		if err != nil {
			fmt.Printf("Failed marshal output: %s\n", err)
			error = true
		} else {
			err := ioutil.WriteFile("content/params/esxi-iso-catalog.yaml.2", bdata, 0755)
			if err != nil {
				fmt.Printf("Failed write output: %s\n", err)
				error = true
			}
			os.Rename("content/params/esxi-iso-catalog.yaml.2", "content/params/esxi-iso-catalog.yaml")
		}
	}

	if error {
		os.Exit(1)
	}
	os.Exit(0)
}
