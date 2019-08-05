package main

import (
	"fmt"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	ovirtsdk4 "gopkg.in/imjoey/go-ovirt.v4"
)

func (p *Plugin) updateDrpMachineFromOvirtVM(l logger.Logger, m *models.Machine, d *ovirtsdk4.Vm) (bool, bool) {
	hasAddr := false
	/*
		m.Name = d.Hostname
		m.Params["ovirt/uuid"] = d.ID
		m.Params["machine-plugin"] = p.name

		// Record the IP Address
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
	*/
	return hasAddr, len(m.HardwareAddrs) > 0
}

func (p *Plugin) CreateDrpMachineFromOvirtVM(l logger.Logger, m *models.Machine, d *ovirtsdk4.Vm) error {
	if m == nil {
		m = &models.Machine{}
	}
	m.Fill()

	p.updateDrpMachineFromOvirtVM(l, m, d)

	if err := p.drpClient.CreateModel(m); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) SaveDrpMachineFromOvirtVM(l logger.Logger, m *models.Machine, d *ovirtsdk4.Vm) error {
	p.updateDrpMachineFromOvirtVM(l, m, d)

	// GREG: Convert to patch
	if err := p.drpClient.PutModel(m); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) CreateOvirtVMFromDrpMachine(l logger.Logger, m *models.Machine, params map[string]interface{}) (*models.Machine, bool, error) {
	/*
		if uuid, ok := params["packet/uuid"].(string); ok {
			d, _, err := p.packetClient.Devices.Get(uuid)
			if err == nil && d != nil {
				return m, true, nil
			}
		}

		// Make PXE booting machine
		dcr := packngo.DeviceCreateRequest{
			Hostname:      m.Name,
			Facility:      utils.GetParamOrString(params, "packet/facility", "ewr1"),
			Plan:          utils.GetParamOrString(params, "packet/plan", "baremetal_0"),
			ProjectID:     utils.GetParamOrString(params, "packet/project-id", p.defProj),
			BillingCycle:  "hourly",
			OS:            "custom_ipxe",
			IPXEScriptURL: fmt.Sprintf("%s/default.ipxe", os.Getenv("RS_FILESERVER")),
			AlwaysPXE:     true,
		}

		if d, _, err := p.packetClient.Devices.Create(&dcr); err != nil {
			return nil, false, err
		} else {
			// 15 minutes = 180 * 5sec-retry
			uuid := d.ID
			for i := 0; i < 180; i++ {
				<-time.After(5 * time.Second)
				d, _, err := p.packetClient.Devices.Get(uuid)
				if err != nil {
					return nil, false, err
				}
				hasAddr, hasMacs := p.updateDrpMachineFromOvirtVM(l, m, d)
				if d.State == "active" || (hasAddr && hasMacs) {
					return m, false, nil
				}
			}
			p.packetClient.Devices.Delete(uuid)
			return nil, false, fmt.Errorf("device %s is still not active after timeout", uuid)
		}
	*/
	return nil, false, fmt.Errorf("Not supported")
}

func (p *Plugin) PowerOnVm(uuid string) error {
	// To use `Must` methods, you should recover it if panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panics occurs, try the non-Must methods to find the reason: %s", err)
		}
	}()

	// Get the reference to the "vms" service
	vmsService := p.ovirtConnection.SystemService().VmsService()

	// Locate the service that manages the virtual machine, as that is where the action methods are defined
	vmService := vmsService.VmService(uuid)

	// Call the "start" method of the service to start it
	vmService.Start().MustSend()

	return nil
}

func (p *Plugin) PowerOffVm(uuid string) error {
	// To use `Must` methods, you should recover it if panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panics occurs, try the non-Must methods to find the reason: %s", err)
		}
	}()

	// Get the reference to the "vms" service
	vmsService := p.ovirtConnection.SystemService().VmsService()

	// Locate the service that manages the virtual machine, as that is where the action methods are defined
	vmService := vmsService.VmService(uuid)

	// Call the "stop" method of the service to stop it
	vmService.Stop().MustSend()

	return nil
}

func (p *Plugin) RebootVm(uuid string) error {
	// To use `Must` methods, you should recover it if panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panics occurs, try the non-Must methods to find the reason: %s", err)
		}
	}()

	// Get the reference to the "vms" service
	vmsService := p.ovirtConnection.SystemService().VmsService()

	// Locate the service that manages the virtual machine, as that is where the action methods are defined
	vmService := vmsService.VmService(uuid)

	// Call the "Reboot" method of the service to Reboot it
	vmService.Reboot().MustSend()

	return nil
}

func (p *Plugin) RemoveVm(uuid string) error {
	// To use `Must` methods, you should recover it if panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panics occurs, try the non-Must methods to find the reason: %s", err)
		}
	}()

	// Get the reference to the "vms" service
	vmsService := p.ovirtConnection.SystemService().VmsService()

	// Locate the service that manages the virtual machine, as that is where the action methods are defined
	vmService := vmsService.VmService(uuid)

	// Call the "Reboot" method of the service to Reboot it
	vmService.Remove().MustSend()

	return nil
}

func (p *Plugin) GetVm(uuid string) (*ovirtsdk4.Vm, error) {
	// To use `Must` methods, you should recover it if panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panics occurs, try the non-Must methods to find the reason: %s", err)
		}
	}()

	// Get the reference to the "vms" service
	vmsService := p.ovirtConnection.SystemService().VmsService()

	// Find the virtual machine
	vm := vmsService.List().Search(fmt.Sprintf("id=%s", uuid)).MustSend().MustVms().Slice()[0]

	return vm, nil
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
