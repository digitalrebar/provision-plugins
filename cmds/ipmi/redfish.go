package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"io/ioutil"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
)

type rfSystem struct {
	Odata
	Actions struct {
		Reset struct {
			AllowedValues []string `json:"ResetType@Redfish.AllowableValues"`
			Target        string   `json:"target"`
		} `json:"#ComputerSystem.Reset"`
	} `json:",omitempty"`
	AssetTag    string `json:",omitempty"`
	Bios        Odata
	BiosVersion string `json:".omitempty"`
	Boot        struct {
		BootOptions                  Odata    `json:",omitempty"`
		BootOrder                    []string `json:",omitempty"`
		BootSourceOverrideEnabled    string   `json:",omitempty"`
		BootSourceOverrideMode       string   `json:",omitempty"`
		BootSourceOverrideTarget     string   `json:",omitempty"`
		UefiTargetBootSourceOverride string   `json:",omitempty"`
	}
	IndicatorLED string `json:",omitempty"`
	Manufacturer string `json:",omitempty"`
	Model        string `json:",omitempty"`
	PowerState   string `json:",omitempty"`
	SKU          string `json:",omitempty"`
	SerialNumber string `json:",omitempty"`
	SystemType   string `json:",omitempty"`
	UUID         string `json:",omitempty"`
}

type redfish struct {
	client                  *http.Client
	url, username, password string
	system                  *rfSystem
}

type Odata struct {
	OdataId   string `json:"@odata.id,omitempty"`
	OdataCtx  string `json:"@odata.context,omitempty"`
	OdataType string `json:"@odata.type,omitempty""`
}

type rfServiceRoot struct {
	Odata
	AccountService Odata
	Chassis        Odata
	EventService   Odata
	Id             string
	JsonSchemas    Odata
	Oem            interface{}
	RedfishVersion string
	Registries     Odata
	ServiceVersion string
	SessionService Odata
	Systems        Odata
}

type rfCollection struct {
	Odata
	Members      []Odata
	MembersCount int
}

func (r *redfish) Name() string { return "redfish" }

func (r *redfish) Do(method, path string, body, result interface{}) (*http.Response, error) {
	defer r.client.CloseIdleConnections()
	var bodyBuf io.Reader
	if body != nil {
		bb := &bytes.Buffer{}
		enc := json.NewEncoder(bb)
		if err := enc.Encode(body); err != nil {
			log.Printf("Body encode error of %#v: %v", body, err)
			return nil, err
		}
		bodyBuf = bb
	}
	req, err := http.NewRequest(method, r.url+path, bodyBuf)
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return nil, err
	}
	req.SetBasicAuth(r.username, r.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	//ro, _ := httputil.DumpRequestOut(req, true)
	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("Error running request: %v\n%#q", err)
		return resp, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode >= 400 {
		buf, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Redfish call %s %s failed: %d %s", req.Method, req.URL.String(), resp.StatusCode, string(buf))
		return resp, fmt.Errorf("Redfish call %s %s failed: %d %s", req.Method, req.URL.String(), resp.StatusCode, string(buf))
	}
	if result != nil {
		dec := json.NewDecoder(resp.Body)
		return resp, dec.Decode(&result)
	}
	return resp, nil
}

func (r *redfish) Probe(l logger.Logger, address, username, password string) bool {
	r.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	r.url = "https://" + address
	r.username, r.password = username, password
	svc := &rfServiceRoot{}
	_, err := r.Do("GET", "/redfish/v1", nil, svc)
	if err != nil {
		log.Printf("%v", err)
		l.Debugf("%v", err)
		return false
	}
	systems := &rfCollection{}
	_, err = r.Do("GET", svc.Systems.OdataId, nil, systems)
	if err != nil {
		l.Errorf("Unable to get systems information from redfish")
		return false
	}
	if len(systems.Members) == 0 {
		l.Infof("No systems defined")
		return false
	}
	// a dirty hack for now
	r.system = &rfSystem{}
	_, err = r.Do("GET", systems.Members[0].OdataId, nil, r.system)
	if err != nil {
		log.Printf("Unable to get systems information from redfish")
		l.Errorf("Unable to get systems information from redfish")
		return false
	}
	return true
}

func (r *redfish) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	var (
		resp   *http.Response
		cmdErr error
	)

	switch ma.Command {
	case "powerstatus":
		return true, r.system.PowerState, nil
	case "identify":
		supported = true
		type rsIdentify struct {
			IndicatorLED string
		}
		ident := rsIdentify{IndicatorLED: "Off"}
		val, ok := ma.Params["ipmi/identify-duration"]
		if !ok {
			ident.IndicatorLED = "Blinking"
		} else if vv, ok := val.(int); ok && vv > 0 {
			ident.IndicatorLED = "Blinking"
		}
		resp, cmdErr := r.Do("PATCH", r.system.OdataId, ident, &res)
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  resp.StatusCode,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		}
	case "poweron", "poweroff", "powercycle":
		if r.system.Actions.Reset.Target == "" {
			// No power actions available
			return false, nil, nil
		}
		supported = true
		type rsPowerAction struct {
			ResetType string
		}
		av := map[string]struct{}{}
		for _, allowed := range r.system.Actions.Reset.AllowedValues {
			av[allowed] = struct{}{}
		}
		powerAction := rsPowerAction{}
		fillForOn := func() {
			if _, ok := av["ForceOn"]; ok {
				powerAction.ResetType = "ForceOn"
			} else if _, ok := av["On"]; ok {
				powerAction.ResetType = "On"
			} else {
				powerAction.ResetType = "PushPowerButton"
			}
		}
		fillForOff := func() {
			if _, ok := av["ForceOff"]; ok {
				powerAction.ResetType = "ForceOff"
			} else if _, ok := av["Off"]; ok {
				powerAction.ResetType = "Off"
			} else {
				powerAction.ResetType = "PushPowerButton"
			}
		}
		switch ma.Command {
		case "poweron":
			if r.system.PowerState != "On" {
				fillForOn()
				resp, cmdErr = r.Do("POST", r.system.Actions.Reset.Target, powerAction, &res)
			}

		case "poweroff":
			if r.system.PowerState != "Off" {
				fillForOff()
				resp, cmdErr = r.Do("POST", r.system.Actions.Reset.Target, powerAction, &res)
			}
		case "powercycle":
			if _, ok := av["PowerCycle"]; ok {
				powerAction.ResetType = "PowerCycle"
			} else if _, ok := av["ForceRestart"]; ok {
				powerAction.ResetType = "ForceRestart"
			} else {
				if r.system.PowerState != "Off" {
					fillForOff()
					r.Do("POST", r.system.Actions.Reset.Target, powerAction, nil)
					time.Sleep(2 * time.Second)
				}
				fillForOn()
			}
			resp, cmdErr = r.Do("POST", r.system.Actions.Reset.Target, powerAction, &res)
		}
		sc := 500
		if resp != nil {
			sc = resp.StatusCode
		}
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  sc,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		}
	case "nextbootpxe", "nextbootdisk", "forcebootpxe", "forcebootdisk":
		supported = true
		type rsBootUpdate struct {
			Boot struct {
				BootSourceOverrideEnabled string
				BootSourceOverrideTarget  string
			}
		}
		bootUpdate := rsBootUpdate{}
		switch ma.Command {
		case "nextbootpxe":
			bootUpdate.Boot.BootSourceOverrideEnabled = "Once"
			bootUpdate.Boot.BootSourceOverrideTarget = "Pxe"
		case "nextbootdisk":
			bootUpdate.Boot.BootSourceOverrideEnabled = "Once"
			bootUpdate.Boot.BootSourceOverrideTarget = "Hdd"
		case "forcebootpxe":
			bootUpdate.Boot.BootSourceOverrideEnabled = "Continuous"
			bootUpdate.Boot.BootSourceOverrideTarget = "Pxe"
		case "forcebootdisk":
			bootUpdate.Boot.BootSourceOverrideEnabled = "Continuous"
			bootUpdate.Boot.BootSourceOverrideTarget = "Hdd"
		}
		resp, cmdErr := r.Do("PATCH", r.system.OdataId, bootUpdate, &res)
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  resp.StatusCode,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		}
	}
	return
}
