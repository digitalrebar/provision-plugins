package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > packet-ipmi.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"

	utils2 "github.com/VictorLowther/jsonpatch2/utils"
	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
	"github.com/packethost/packngo"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
)

var (
	version = v4.RS_VERSION
	def     = models.PluginProvider{
		Name:          "packet-ipmi",
		Version:       version,
		PluginVersion: 2,
		HasPublish:    true,
		AvailableActions: []models.AvailableAction{
			{
				Command: "createVm",
				Model:   "plugins",
				RequiredParams: []string{
					"packet/api-key",
				},
				OptionalParams: []string{
					"packet/name",
					"packet/facility",
					"packet/plan",
					"packet/project-id",
				},
			},
			{
				Command: "startVm",
				Model:   "plugins",
				RequiredParams: []string{
					"packet/api-key",
				},
				OptionalParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "stopVm",
				Model:   "plugins",
				RequiredParams: []string{
					"packet/api-key",
				},
				OptionalParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "destroyVm",
				Model:   "plugins",
				RequiredParams: []string{
					"packet/api-key",
				},
				OptionalParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "poweron",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "poweroff",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "powercycle",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "nextbootpxe",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "nextbootdisk",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
			},
			{
				Command: "updatepxe",
				Model:   "machines",
				RequiredParams: []string{
					"packet/uuid",
				},
				OptionalParams: []string{
					"packet/ipxe-script-url",
					"packet/always-pxe",
				},
			},
		},
		RequiredParams: []string{
			"packet/api-key",
		},
		OptionalParams: []string{
			"packet/project-id",
			"packet/import-existing",
		},
		Content: contentYamlString,
	}
)

func handleResponse(resp *packngo.Response) (string, *models.Error) {
	resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 204 || resp.StatusCode == 202 || resp.StatusCode == 422 {
		return "Success", nil
	}
	return "", utils.MakeError(resp.StatusCode, fmt.Sprintf("Packet Request Failed: %d", resp.StatusCode))
}

type Plugin struct {
	ApiKey       string
	packetClient *packngo.Client
	drpClient    *api.Client
	name         string
	defProj      string
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.drpClient = session
	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}
	utils.SetErrorName(p.name)

	if ak, err := utils.GetDrpStringParam(session, "plugins", p.name, "packet/api-key"); err != nil {
		return err
	} else {
		p.ApiKey = ak
	}

	importExisting := false
	if ak, err := utils.GetDrpBooleanParam(session, "plugins", p.name, "packet/import-existing"); err == nil {
		importExisting = ak
	}

	p.packetClient = packngo.NewClientWithAuth("packet-ipmi plugin", p.ApiKey, nil)

	packet_uuids := map[string]bool{}
	create_packet_devs := []packngo.Device{}

	projs, _, err := p.packetClient.Projects.List(nil)
	if err != nil {
		return utils.MakeError(400, fmt.Sprintf("Command failed: get projects: %s", err.Error()))
	}
	// build add and remove data.
	for ii, proj := range projs {
		if ii == 0 {
			p.defProj = proj.ID
		}
		devs, _, err := p.packetClient.Devices.List(proj.ID, nil)
		if err != nil {
			return utils.MakeError(400, fmt.Sprintf("Command failed: get devices for %s: %s", proj.ID, err.Error()))
		}

		for _, dev := range devs {
			packet_uuids[dev.ID] = true
			if m, err2 := utils.GetDrpMachineByParam(p.drpClient, "packet/uuid", dev.ID); err2 == nil && m != nil {
				continue
			}
			np := proj
			dev.Project = &np
			create_packet_devs = append(create_packet_devs, dev)
		}
	}
	if pid, ok := config["packet/project-id"].(string); ok {
		p.defProj = pid
	}

	// Remove machines in drp not in packet.
	if data, err := p.drpClient.ListModel("machines"); err != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to get drp machines: %s", err.Error()))
	} else {
		for _, d := range data {
			m := d.(*models.Machine)

			if pu, ok := m.Params["packet/uuid"].(string); !ok {
				continue
			} else {
				// If the packet/uuid of the machine is not in the map, remove it.
				if _, ok := packet_uuids[pu]; !ok {
					if e := utils.DeleteDrpMachine(p.drpClient, m.UUID()); e != nil {
						return e
					}
				}
			}
		}
	}

	// Only create machines if asked to
	if importExisting {
		// Add machines in packet to drp
		for _, dev := range create_packet_devs {
			m := &models.Machine{}
			m.Fill()
			if err := p.CreateDrpMachineFromPacketDevice(l, m, &dev); err != nil {
				return utils.MakeError(400, fmt.Sprintf("Command failed: create device for %s:%s %s", dev.Project.ID, dev.ID, err.Error()))
			}
		}
	}

	return nil
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	uuid, ok := ma.Params["packet/uuid"].(string)
	if !ok && ma.Command != "createVm" {
		err = utils.MakeError(400, "Missing packet uuid")
		return
	}

	switch ma.Command {
	case "poweron":
		if r, err2 := p.packetClient.Devices.PowerOn(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer, err = handleResponse(r)
		}
	case "poweroff":
		if r, err2 := p.packetClient.Devices.PowerOff(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer, err = handleResponse(r)
		}

	case "powercycle":
		if r, err2 := p.packetClient.Devices.Reboot(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer, err = handleResponse(r)
		}

	case "nextbootpxe", "nextbootdisk":
		// nothing
		answer = "Complete"

	case "updatepxe":
		mamachine := &models.Machine{}
		mamachine.Fill()
		if err2 := utils2.Remarshal(ma.Model, &mamachine); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Invalid machine %s: %v", ma.Command, err2))
			return
		}

		up := &packngo.DeviceUpdateRequest{}
		if url, ok := ma.Params["packet/ipxe-script-url"].(string); ok {
			up.IPXEScriptURL = &url
		}
		if alwayspxe, ok := ma.Params["packet/always-pxe"].(bool); ok {
			up.AlwaysPXE = &alwayspxe
		}
		if d, _, err2 := p.packetClient.Devices.Get(uuid, nil); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
			return
		} else {
			up.Hostname = &d.Hostname
		}
		if d, r, err2 := p.packetClient.Devices.Update(uuid, up); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			if m, err3 := p.getDrpMachine(mamachine.UUID()); err3 == nil {
				if err2 := p.SaveDrpMachineFromPacketDevice(l, m, d); err2 != nil {
					l.Errorf(fmt.Sprintf("Save Machine Failed: %s", err2.Error()))
				}
			} else {
				l.Errorf(fmt.Sprintf("Get Machine Failed: %s", err3.Error()))
			}

			answer, err = handleResponse(r)
		}

	case "createVm": // Means start as well.
		m := &models.Machine{}
		m.Fill()

		m.Name = utils.GetParamOrString(ma.Params, "packet/name", "fred")
		m.Params["packet/plan"] = utils.GetParamOrString(ma.Params, "packet/plan", "baremetal_0")
		m.Params["packet/facility"] = utils.GetParamOrString(ma.Params, "packet/facility", "ewr1")
		m.Params["packet/project-id"] = utils.GetParamOrString(ma.Params, "packet/project-id", p.defProj)

		if m, _, err2 := p.CreatePacketDeviceFromDrpMachine(l, m, m.Params); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			uuid := m.Params["packet/uuid"].(string)
			getOpts := packngo.GetOptions{Includes: []string{"facility", "project", "plan"}}
			if packetMachine, _, err2 := p.packetClient.Devices.Get(uuid, &getOpts); err2 != nil {
				p.packetClient.Devices.Delete(uuid)
				err = utils.MakeError(400,
					fmt.Sprintf("Failed to get specific info for packet machine, %s. %v", uuid, err2))
			} else {
				if err2 := p.CreateDrpMachineFromPacketDevice(l, m, packetMachine); err2 != nil {
					p.packetClient.Devices.Delete(packetMachine.ID)
					err = utils.MakeError(400, fmt.Sprintf("Create Machine: %s: %s", ma.Command, err2.Error()))
				}
			}
			answer = fmt.Sprintf("Created %s", uuid)
		}

	case "startVm":
		if r, err2 := p.packetClient.Devices.PowerOn(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer, err = handleResponse(r)
		}

	case "stopVm":
		if r, err2 := p.packetClient.Devices.PowerOff(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer, err = handleResponse(r)
		}

	case "destroyVm":
		if _, err2 := p.packetClient.Devices.Delete(uuid); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			// GREG: Should we handle delete response from packet?
			if m, err2 := utils.GetDrpMachineByParam(p.drpClient, "packet/uuid", uuid); err2 == nil && m != nil {
				if _, err2 := p.drpClient.DeleteModel("machines", m.UUID()); err2 != nil {
					err = utils.MakeError(409, fmt.Sprintf("Failed to destroy machine: %v", err2.Error()))
				}
			}
			answer = fmt.Sprintf("Destroyed %s", uuid)
		}

	default:
		err = utils.MakeError(404, fmt.Sprintf("Unknown command: %s", ma.Command))
	}

	return
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) *models.Error {
	// Only care about machines!
	if e.Type != "machines" {
		return nil
	}

	// Only care about create or delete actions
	if e.Action != "create" && e.Action != "delete" {
		return nil
	}

	// Make sure we get a machine model.
	obj, err := e.Model()
	if err != nil {
		// Bad machine ignore.
		return nil
	}
	m := obj.(*models.Machine)

	params := m.Params
	if pars, err := p.getDrpMachineParams(m.UUID()); err != nil {
		// If create, we fail.  If not create, we use the model.
		if e.Action == "create" {
			l.Errorf(err.Error())
			return nil
		}
	} else {
		params = pars
	}

	pname, ok := params["machine-plugin"].(string)
	if !ok || pname != p.name {
		return nil
	}

	// Create the machine
	if e.Action == "create" {
		if m2, exists, err := p.CreatePacketDeviceFromDrpMachine(l, m, params); err != nil {
			l.Errorf(fmt.Sprintf("Failed to create: %s: %s", m.Name, err.Error()))
			if _, err2 := p.drpClient.DeleteModel("machines", m.UUID()); err2 != nil {
				l.Errorf("Failed to destroy machine: %v", err2.Error())
			}
		} else {
			if exists {
				return nil
			}
			uuid := m2.Params["packet/uuid"].(string)

			getOpts := packngo.GetOptions{Includes: []string{"facility", "project", "plan"}}
			if packetMachine, _, err2 := p.packetClient.Devices.Get(uuid, &getOpts); err2 != nil {
				p.packetClient.Devices.Delete(uuid)
				l.Errorf(fmt.Sprintf("Failed to get specific info for packet machine, %s. %v", uuid, err2))
				if _, err2 := p.drpClient.DeleteModel("machines", m.UUID()); err2 != nil {
					l.Errorf("Failed to destroy machine: %v", err2.Error())
				}
			} else {
				if err2 := p.SaveDrpMachineFromPacketDevice(l, m2, packetMachine); err2 != nil {
					p.packetClient.Devices.Delete(packetMachine.ID)
					l.Errorf(fmt.Sprintf("Save Machine Failed: %s", err2.Error()))
					if _, err2 := p.drpClient.DeleteModel("machines", m.UUID()); err2 != nil {
						l.Errorf("Failed to destroy machine: %v", err2.Error())
					}
				}
			}
		}
		return nil
	}

	// Delete the machine
	if e.Action == "delete" {
		if uuid, ok := m.Params["packet/uuid"].(string); ok {
			if _, err := p.packetClient.Devices.Delete(uuid); err != nil {
				l.Errorf(fmt.Sprintf("Failed to destroy machine: %s: %s", uuid, err.Error()))
			}
		}
		return nil
	}

	return nil
}

func main() {
	plugin.InitApp("packet-ipmi", "Provides out-of-band IPMI-like controls Packet.net", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
