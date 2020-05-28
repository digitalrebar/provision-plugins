package main

import (
	"encoding/json"
	"fmt"
	"time"

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

func (r *redfish) Probe(l logger.Logger, address, username, password string) bool {
	r.username = username
	r.password = password
	// Create a new instance of gofish client, ignoring self-signed certs
	config := gofish.ClientConfig{
		Endpoint: fmt.Sprintf("https://%s", address),
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

func (r *redfish) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	var cmdErr error
	// Close session when done
	defer r.client.Logout()

	switch ma.Command {
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
