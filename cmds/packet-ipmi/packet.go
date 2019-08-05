package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/packethost/packngo"
	"github.com/digitalrebar/provision-plugins/v4/utils"
)

func (p *Plugin) updateDrpMachineFromPacketDevice(l logger.Logger, m *models.Machine, d *packngo.Device) (bool, bool) {
	m.Name = d.Hostname
	m.Params["packet/uuid"] = d.ID
	m.Params["packet/plan"] = d.Plan.Slug
	m.Params["packet/facility"] = d.Facility.Code
	m.Params["packet/sos"] = fmt.Sprintf("%s@sos.%s.packet.net", d.ID, d.Facility.Code)
	m.Params["packet/project-id"] = d.Project.ID
	m.Params["packet/always-pxe"] = d.AlwaysPXE
	m.Params["packet/ipxe-script-url"] = d.IPXEScriptURL
	m.Params["machine-plugin"] = p.name

	// Always add the packet-console profile.
	found := false
	for _, prof := range m.Profiles {
		if prof == "packet-console" {
			found = true
			break
		}
	}
	if !found {
		m.Profiles = append(m.Profiles, "packet-console")
	}

	// Record the IP Address
	hasAddr := false
	for _, ip := range d.Network {
		if ip.AddressFamily == 4 && ip.Public {
			m.Address = net.ParseIP(ip.Address)
			hasAddr = true
		}
	}

	// Record the MAC Addresses
	m.HardwareAddrs = []string{}
	for _, port := range d.NetworkPorts {
		if port.Type == "NetworkPort" {
			m.HardwareAddrs = append(m.HardwareAddrs, port.Data.MAC)
		}
	}
	return hasAddr, len(m.HardwareAddrs) > 0
}

func (p *Plugin) CreateDrpMachineFromPacketDevice(l logger.Logger, m *models.Machine, d *packngo.Device) error {
	if m == nil {
		m = &models.Machine{}
	}
	m.Fill()

	p.updateDrpMachineFromPacketDevice(l, m, d)

	if err := p.drpClient.CreateModel(m); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) SaveDrpMachineFromPacketDevice(l logger.Logger, m *models.Machine, d *packngo.Device) error {
	p.updateDrpMachineFromPacketDevice(l, m, d)

	// GREG: Convert to patch
	if err := p.drpClient.PutModel(m); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) CreatePacketDeviceFromDrpMachine(l logger.Logger, m *models.Machine, params map[string]interface{}) (*models.Machine, bool, error) {
	if uuid, ok := m.Params["packet/uuid"].(string); ok {
		d, _, err := p.packetClient.Devices.Get(uuid, nil)
		if err == nil && d != nil {
			return m, true, nil
		}
	}

	// Make PXE booting machine
	dcr := packngo.DeviceCreateRequest{
		Hostname:     m.Name,
		Facility:     []string{utils.GetParamOrString(params, "packet/facility", "ewr1")},
		Plan:         utils.GetParamOrString(params, "packet/plan", "baremetal_0"),
		ProjectID:    utils.GetParamOrString(params, "packet/project-id", p.defProj),
		BillingCycle: "hourly",
		OS:           "custom_ipxe",
		IPXEScriptURL: utils.GetParamOrString(params, "packet/ipxe-script-url",
			fmt.Sprintf("%s/default.ipxe", os.Getenv("RS_FILESERVER"))),
		AlwaysPXE: utils.GetParamOrBoolean(params, "packet/always-pxe", true),
	}

	if d, _, err := p.packetClient.Devices.Create(&dcr); err != nil {
		return nil, false, err
	} else {
		// 15 minutes = 180 * 5sec-retry
		uuid := d.ID
		for i := 0; i < 180; i++ {
			<-time.After(5 * time.Second)
			d, _, err := p.packetClient.Devices.Get(uuid, nil)
			if err != nil {
				return nil, false, err
			}
			hasAddr, hasMacs := p.updateDrpMachineFromPacketDevice(l, m, d)
			if d.State == "active" || (hasAddr && hasMacs) {
				return m, false, nil
			}
		}
		p.packetClient.Devices.Delete(uuid)
		return nil, false, fmt.Errorf("device %s is still not active after timeout", uuid)
	}
}

func (p *Plugin) getDrpMachineParams(uuid string) (map[string]interface{}, error) {
	req := p.drpClient.Req().UrlFor("machines", uuid, "params")
	req.Params("aggregate", "true")
	res := map[string]interface{}{}
	if err := req.Do(&res); err != nil {
		return nil, fmt.Errorf("Failed to fetch params %v: %v", uuid, err)
	}
	return res, nil
}

func (p *Plugin) getDrpMachine(uuid string) (*models.Machine, error) {
	req := p.drpClient.Req().UrlFor("machines", uuid)
	res := &models.Machine{}
	if err := req.Do(&res); err != nil {
		return nil, fmt.Errorf("Failed to fetch machine %v: %v", uuid, err)
	}
	return res, nil
}
