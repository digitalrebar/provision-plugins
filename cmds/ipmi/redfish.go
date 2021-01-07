package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	utils2 "github.com/VictorLowther/jsonpatch2/utils"

	"github.com/digitalrebar/provision-plugins/v4/utils"

	"github.com/stmcginnis/gofish/common"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/stmcginnis/gofish"
	rf "github.com/stmcginnis/gofish/redfish"
)

type redfish struct {
	client                  *gofish.APIClient
	system                  *rf.ComputerSystem
	url, username, password string
}

func (r *redfish) Name() string { return "redfish" }

func (r *redfish) Probe(l logger.Logger, address string, port int, username, password string) bool {
	r.username = username
	r.password = password

	// Create a new instance of gofish client, ignoring self-signed certs
	config := gofish.ClientConfig{
		Endpoint: fmt.Sprintf("https://%s", net.JoinHostPort(address, strconv.Itoa(port))),
		Username: r.username,
		Password: r.password,
		Insecure: true,
	}
	r.url = config.Endpoint
	var err error
	r.client, err = gofish.Connect(config)
	if err != nil {
		l.Errorf("Unable to get connect to redfish: %v", err)
		return false
	}

	svc := r.client.Service

	systems, err := svc.Systems()
	if err != nil {
		l.Errorf("Unable to get systems information from redfish")
		r.client.Logout()
		return false
	}

	if len(systems) == 0 {
		l.Infof("No systems defined")
		r.client.Logout()
		return false
	}

	// a dirty hack for now
	r.system = systems[0]
	return true
}

func (r *redfish) getManager() (string, *models.Error) {
	m, merr := r.client.Get("/redfish/v1/Managers/")
	if merr != nil {
		return "", utils.ConvertError(400, merr)
	}
	defer m.Body.Close()
	data, derr := ioutil.ReadAll(m.Body)
	if derr != nil {
		return "", utils.ConvertError(400, derr)
	}
	mdata := map[string]interface{}{}
	jerr := json.Unmarshal(data, &mdata)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	mem := []map[string]string{}
	jerr = utils2.Remarshal(mdata["Members"], &mem)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	s, ok := mem[0]["@odata.id"]
	if !ok {
		return "", utils.MakeError(400, "bad struct")
	}
	return s, nil
}

func (r *redfish) getVirtualMedia() (string, *models.Error) {
	mgr, err := r.getManager()
	if err != nil {
		return "", err
	}
	m, merr := r.client.Get(mgr)
	if merr != nil {
		return "", utils.ConvertError(400, merr)
	}
	defer m.Body.Close()
	data, derr := ioutil.ReadAll(m.Body)
	if derr != nil {
		return "", utils.ConvertError(400, derr)
	}
	mdata := map[string]interface{}{}
	jerr := json.Unmarshal(data, &mdata)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	d, ok := mdata["VirtualMedia"]
	if !ok {
		return "", utils.MakeError(400, "No virtual media field")
	}

	mem := map[string]string{}
	jerr = utils2.Remarshal(d, &mem)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	s, ok := mem["@odata.id"]
	if !ok {
		return "", utils.MakeError(400, "bad struct")
	}

	return s, nil
}

func (r *redfish) getVirtualMediaCd() (string, *models.Error) {
	mgr, verr := r.getVirtualMedia()
	if verr != nil {
		return "", verr
	}
	m, merr := r.client.Get(mgr)
	if merr != nil {
		return "", utils.ConvertError(400, merr)
	}
	defer m.Body.Close()
	data, derr := ioutil.ReadAll(m.Body)
	if derr != nil {
		return "", utils.ConvertError(400, derr)
	}
	mdata := map[string]interface{}{}
	jerr := json.Unmarshal(data, &mdata)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	mem := []map[string]string{}
	jerr = utils2.Remarshal(mdata["Members"], &mem)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	// First one is removeable disk, second is cd
	s, ok := mem[1]["@odata.id"]
	if !ok {
		return "", utils.MakeError(400, "bad struct")
	}
	return s, nil
}

func (r *redfish) getVirtualMediaCdStatus() (interface{}, *models.Error) {
	mgr, verr := r.getVirtualMediaCd()
	if verr != nil {
		return "", verr
	}

	m, merr := r.client.Get(mgr)
	if merr != nil {
		return "", utils.ConvertError(400, merr)
	}
	defer m.Body.Close()
	data, derr := ioutil.ReadAll(m.Body)
	if derr != nil {
		return "", utils.ConvertError(400, derr)
	}
	mdata := map[string]interface{}{}
	jerr := json.Unmarshal(data, &mdata)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}

	type answer struct {
		Inserted interface{}
		Image    interface{}
	}
	ans := &answer{
		Inserted: mdata["Inserted"],
		Image:    mdata["Image"],
	}
	return ans, nil
}

func (r *redfish) doVirtualMediaCdAction(action, actionData string, nextBoot bool) (interface{}, *models.Error) {
	mgr, verr := r.getVirtualMediaCd()
	if verr != nil {
		return "", verr
	}

	hpe := false
	dell := false
	if strings.Contains(mgr, "iDRAC") {
		dell = true
	} else {
		// XXX: Assume HPE>>>>
		hpe = true
	}

	data := map[string]interface{}{}
	method := "POST"
	if actionData != "" {
		data["Image"] = actionData
		if nextBoot && hpe {
			method = "PATCH"
			action = ""
			boolData := map[string]bool{}
			boolData["BootOnNextServerReset"] = true
			moreData := map[string]interface{}{}
			moreData["Hpe"] = boolData
			data["Oem"] = moreData
		}
	}
	if !strings.HasSuffix(mgr, "/") {
		mgr = mgr + "/"
	}
	var merr error
	var m *http.Response
	if method == "POST" {
		m, merr = r.client.Post(mgr+action, data)
	} else if method == "PATCH" {
		m, merr = r.client.Patch(mgr+action, data)
	} else {
		return "", utils.MakeError(500, fmt.Sprintf("Unknown method: %s", method))
	}
	if merr != nil {
		return "", utils.ConvertError(400, merr)
	}
	defer m.Body.Close()

	if m.StatusCode == http.StatusNoContent {
		if nextBoot {
			if dell {
				mgr, verr = r.getManager() // Should cache this
				if verr != nil {
					return "", verr
				}
				if !strings.HasSuffix(mgr, "/") {
					mgr = mgr + "/"
				}

				aadata := map[string]string{}
				aadata["Target"] = "ALL"
				adata := map[string]interface{}{}
				adata["ShareParameters"] = aadata
				adata["ImportBuffer"] = "<SystemConfiguration><Component FQDD=\"iDRAC.Embedded.1\"><Attribute Name=\"ServerBoot.1#BootOnce\">Enabled</Attribute><Attribute Name=\"ServerBoot.1#FirstBootDevice\">VCD-DVD</Attribute></Component></SystemConfiguration>"
				arep, aerr := r.client.Post(mgr+"Actions/Oem/EID_674_Manager.ImportSystemConfiguration", adata)
				if aerr != nil {
					return "", utils.MakeError(400, fmt.Sprintf("GREG: %s %v %v", mgr+"Attributes/", adata, aerr))
				}
				defer arep.Body.Close()

				if arep.StatusCode == 204 || arep.StatusCode == 202 {
					return "Success", nil
				}

				bs, derr := ioutil.ReadAll(arep.Body)
				if derr != nil {
					return "", utils.ConvertError(400, derr)
				}
				return "", utils.MakeError(400, fmt.Sprintf("%d: %s", arep.StatusCode, string(bs)))
			}
		}
		return "Success", nil
	}

	bs, derr := ioutil.ReadAll(m.Body)
	if derr != nil {
		return "", utils.ConvertError(400, derr)
	}
	mdata := map[string]interface{}{}
	jerr := json.Unmarshal(bs, &mdata)
	if jerr != nil {
		return "", utils.ConvertError(400, jerr)
	}
	return mdata, nil
}

func (r *redfish) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	var cmdErr error
	// Close session when done
	defer r.client.Logout()

	switch ma.Command {
	case "statusVirtualMedia":
		supported = true
		res, err = r.getVirtualMediaCdStatus()
		return
	case "mountVirtualMedia":
		supported = true
		imageName := ma.Params["ipmi/virtual-media-url"].(string)
		bootIt := ma.Params["ipmi/virtual-media-boot"].(bool)
		res, err = r.doVirtualMediaCdAction("Actions/VirtualMedia.InsertMedia", imageName, bootIt)
		return
	case "unmountVirtualMedia":
		supported = true
		res, err = r.doVirtualMediaCdAction("Actions/VirtualMedia.EjectMedia", "", false)
		return
	case "getBoot":
		p := r.system.Boot
		return true, p, nil
	case "getSecureBoot":
		p, e := r.system.SecureBoot()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish secure boot error: %v", e)
			return
		}
		if p == nil {
			p = &rf.SecureBoot{}
			p.SecureBootEnable = false
		}
		p.Client = nil
		return true, p, nil
	case "getEthernetInterfaces":
		p, e := r.system.EthernetInterfaces()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish ethernet interfaces error: %v", e)
			return
		}
		for _, pp := range p {
			pp.Client = nil
		}
		return true, p, nil
	case "getNetworkInterfaces":
		p, e := r.system.NetworkInterfaces()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish network interfaces error: %v", e)
			return
		}
		for _, pp := range p {
			pp.Client = nil
		}
		return true, p, nil
	case "getProcessor":
		p, e := r.system.Processors()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish processor error: %v", e)
			return
		}
		for _, pp := range p {
			pp.Client = nil
		}
		return true, p, nil
	case "getSimpleStorage":
		s, e := r.system.SimpleStorages()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish simple storage error: %v", e)
			return
		}
		for _, ss := range s {
			ss.Client = nil
		}
		return true, s, nil
	case "getStorage":
		s, e := r.system.Storage()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish storage error: %v", e)
			return
		}
		for _, ss := range s {
			ss.Client = nil
		}
		return true, s, nil
	case "getMemory":
		m, e := r.system.Memory()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish memory error: %v", e)
			return
		}
		for _, mm := range m {
			mm.Client = nil
		}
		return true, m, nil
	case "getBios":
		m, e := r.system.Bios()
		if e != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish bios error: %v", e)
			return
		}
		m.Client = nil
		return true, m, nil
	case "getInfo":
		r.system.Client = nil
		return true, r.system, nil
	case "status":

		var ret struct {
			Status              common.Health
			ProcessorStatus     common.Health
			MemoryStatus        common.Health
			EthernetStatus      common.Health
			StorageStatus       common.Health
			SimpleStorageStatus common.Health
		}

		ret.Status = r.system.Status.Health
		ret.ProcessorStatus = r.system.ProcessorSummary.Status.Health
		ret.MemoryStatus = r.system.MemorySummary.Status.Health

		eh := common.OKHealth
		ee, err := r.system.EthernetInterfaces()
		if err == nil {
			for _, e := range ee {
				if e.Status.Health != "" && e.Status.Health != common.OKHealth {
					eh = e.Status.Health
					break
				}
			}
		} else {
			eh = common.CriticalHealth
		}
		ret.EthernetStatus = eh

		sh := common.OKHealth
		ss, err := r.system.Storage()
		if err == nil {
			for _, s := range ss {
				if s.Status.Health != "" && s.Status.Health != common.OKHealth {
					sh = s.Status.Health
					break
				}
			}
		} else {
			sh = common.CriticalHealth
		}
		ret.StorageStatus = sh

		ssh := common.OKHealth
		sss, err := r.system.SimpleStorages()
		if err == nil {
			for _, s := range sss {
				if s.Status.Health != "" && s.Status.Health != common.OKHealth {
					ssh = s.Status.Health
					break
				}
			}
		} else {
			ssh = common.CriticalHealth
		}
		ret.SimpleStorageStatus = ssh

		return true, ret, nil
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
		resp, cmdErr := r.client.Patch(r.system.ODataID, ident)
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  resp.StatusCode,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		} else {
			dec := json.NewDecoder(resp.Body)
			e := dec.Decode(&res)
			if e != nil {
				err = &models.Error{
					Model: "plugin",
					Key:   "ipmi",
					Type:  "rpc",
					Code:  400,
				}
				err.Errorf("Redfish json error: %v", e)
			}
		}
	case "poweron", "poweroff", "powercycle":
		supported = true
		av := map[rf.ResetType]struct{}{}
		for _, allowed := range r.system.SupportedResetTypes {
			av[allowed] = struct{}{}
		}
		powerAction := rf.ForceOnResetType
		fillForOn := func() {
			if _, ok := av[rf.ForceOnResetType]; ok {
				powerAction = rf.ForceOnResetType
			} else if _, ok := av[rf.OnResetType]; ok {
				powerAction = rf.OnResetType
			} else if _, ok := av[rf.PushPowerButtonResetType]; ok {
				powerAction = rf.PushPowerButtonResetType
			} else {
				powerAction = rf.ForceRestartResetType
			}
		}
		fillForOff := func() {
			if _, ok := av[rf.ForceOffResetType]; ok {
				powerAction = rf.ForceOffResetType
			} else if _, ok := av["Off"]; ok {
				powerAction = "Off"
			} else if _, ok := av[rf.GracefulShutdownResetType]; ok {
				powerAction = rf.GracefulShutdownResetType
			} else if _, ok := av[rf.PushPowerButtonResetType]; ok {
				powerAction = rf.PushPowerButtonResetType
			} else {
				powerAction = rf.ForceRestartResetType
			}
		}
		switch ma.Command {
		case "poweron":
			if r.system.PowerState != "On" {
				fillForOn()
				cmdErr = r.system.Reset(powerAction)
			}

		case "poweroff":
			if r.system.PowerState != "Off" {
				fillForOff()
				cmdErr = r.system.Reset(powerAction)
			}
		case "powercycle":
			if _, ok := av[rf.PowerCycleResetType]; ok {
				powerAction = rf.PowerCycleResetType
			} else if _, ok := av[rf.ForceRestartResetType]; ok {
				powerAction = rf.ForceRestartResetType
			} else {
				if r.system.PowerState != "Off" {
					fillForOff()
					r.system.Reset(powerAction)
					time.Sleep(2 * time.Second)
				}
				fillForOn()
			}
			cmdErr = r.system.Reset(powerAction)
		}
		sc := 400
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  sc,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		}
		res = "{}"
	case "nextbootcd", "nextbootpxe", "nextbootdisk", "forcebootpxe", "forcebootdisk":
		supported = true
		type rsBootUpdate struct {
			Boot struct {
				BootSourceOverrideEnabled string
				BootSourceOverrideTarget  string
			}
		}
		bootUpdate := rsBootUpdate{}
		switch ma.Command {
		case "nextbootcd":
			bootUpdate.Boot.BootSourceOverrideEnabled = "Once"
			bootUpdate.Boot.BootSourceOverrideTarget = "Cd"
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
		resp, cmdErr := r.client.Patch(r.system.ODataID, bootUpdate)
		if cmdErr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("Redfish error: %v", cmdErr)
		} else {
			dec := json.NewDecoder(resp.Body)
			e := dec.Decode(&res)
			if e != nil {
				err = &models.Error{
					Model: "plugin",
					Key:   "ipmi",
					Type:  "rpc",
					Code:  400,
				}
				err.Errorf("Redfish json error: %v", e)
			}
		}
	}
	return
}
