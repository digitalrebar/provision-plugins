package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > ovirt.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
	ovirtsdk4 "gopkg.in/imjoey/go-ovirt.v4"
)

var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "ovirt",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    true,
		AvailableActions: []models.AvailableAction{
			models.AvailableAction{Command: "createVm",
				Model: "plugins",
				OptionalParams: []string{
					"ovirt/name",
				},
			},
			models.AvailableAction{Command: "startVm",
				Model: "plugins",
				OptionalParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "stopVm",
				Model: "plugins",
				OptionalParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "destroyVm",
				Model: "plugins",
				OptionalParams: []string{
					"ovirt/uuid",
				},
			},

			models.AvailableAction{Command: "poweron",
				Model: "machines",
				RequiredParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "poweroff",
				Model: "machines",
				RequiredParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "powercycle",
				Model: "machines",
				RequiredParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "nextbootpxe",
				Model: "machines",
				RequiredParams: []string{
					"ovirt/uuid",
				},
			},
			models.AvailableAction{Command: "nextbootdisk",
				Model: "machines",
				RequiredParams: []string{
					"ovirt/uuid",
				},
			},
		},
		RequiredParams: []string{
			"ovirt/username",
			"ovirt/password",
			"ovirt/url",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	ovirtConnection *ovirtsdk4.Connection
	drpClient       *api.Client
	name            string
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.drpClient = session
	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}
	utils.SetErrorName(p.name)

	username, err := utils.GetDrpStringParam(session, "plugins", p.name, "ovirt/username")
	if err != nil {
		return err
	}
	password, err := utils.GetDrpStringParam(session, "plugins", p.name, "ovirt/password")
	if err != nil {
		return err
	}
	url, err := utils.GetDrpStringParam(session, "plugins", p.name, "ovirt/url")
	if err != nil {
		return err
	}

	// Create the connection to the api server
	conn, err2 := ovirtsdk4.NewConnectionBuilder().
		URL(url).
		Username(username).
		Password(password).
		Insecure(true).
		Compress(true).
		Timeout(time.Second * 10).
		Build()
	if err2 != nil {
		return utils.MakeError(400, fmt.Sprintf("Make connection failed, reason: %s", err2.Error()))
	}
	p.ovirtConnection = conn
	p.drpClient = session

	// Get the reference to the "vms" service:
	vmsService := conn.SystemService().VmsService()

	// Use the "list" method of the "vms" service to list all the virtual machines
	vmsResponse, err2 := vmsService.List().Send()
	if err2 != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to get vm list, reason: %v\n", err2))
	}
	if vms, ok := vmsResponse.Vms(); ok {
		// Print the virtual machine names and identifiers:
		for _, vm := range vms.Slice() {
			vmID, ok := vm.Id()
			if !ok {
				continue
			}

			if m, err2 := utils.GetDrpMachineByParam(p.drpClient, "ovirt/uuid", vmID); err2 == nil && m != nil {
				continue
			}

			m := &models.Machine{}
			m.Fill()
			if err := p.CreateDrpMachineFromOvirtVM(l, m, vm); err != nil {
				return utils.MakeError(400, fmt.Sprintf("Command failed: create device for %s %s", vmID, err.Error()))
			}
		}
	}

	return nil
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	uuid, ok := ma.Params["ovirt/uuid"].(string)
	if !ok && ma.Command != "createVm" {
		err = utils.MakeError(400, "Missing ovirt uuid")
		return
	}

	switch ma.Command {
	case "poweron", "startVm":
		if err2 := p.PowerOnVm(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = "Success"
		}
	case "poweroff", "stopVm":
		if err2 := p.PowerOffVm(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = "Success"
		}

	case "powercycle":
		if err2 := p.RebootVm(uuid); err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = "Success"
		}

	case "nextbootpxe", "nextbootdisk":
		// nothing
		answer = "Complete"

	case "createVm": // Means start as well.
		m := &models.Machine{}
		m.Fill()

		m.Name = utils.GetParamOrString(ma.Params, "ovirt/name", "fred")

		if m, _, err2 := p.CreateOvirtVMFromDrpMachine(l, m, m.Params); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			uuid := m.Params["ovirt/uuid"].(string)

			if ovirtVM, err2 := p.GetVm(uuid); err2 != nil {
				p.RemoveVm(uuid)
				err = utils.MakeError(400,
					fmt.Sprintf("Failed to get specific info for ovirt vm, %s. %v", uuid, err2))
			} else {
				if err2 := p.CreateDrpMachineFromOvirtVM(l, m, ovirtVM); err2 != nil {
					p.RemoveVm(ovirtVM.MustId())
					err = utils.MakeError(400, fmt.Sprintf("Save Machine: %s: %s", ma.Command, err2.Error()))
				}
			}
			answer = fmt.Sprintf("Created %s", uuid)
		}

	case "destroyVm":
		if err2 := p.RemoveVm(uuid); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			if m, err2 := utils.GetDrpMachineByParam(p.drpClient, "ovirt/uuid", uuid); err2 == nil && m != nil {
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
	if p, err := p.getDrpMachineParams(m.UUID()); err != nil {
		// If create, we fail.  If not create, we use the model.
		if e.Action == "create" {
			l.Errorf(err.Error())
			return nil
		}
	} else {
		params = p
	}

	pname, ok := params["machine-plugin"].(string)
	if !ok || pname != p.name {
		return nil
	}

	// Create the machine
	if e.Action == "create" {
		if m2, exists, err := p.CreateOvirtVMFromDrpMachine(l, m, params); err != nil {
			l.Errorf(fmt.Sprintf("Failed to create: %s: %s", m.Name, err.Error()))
		} else {
			if exists {
				return nil
			}
			uuid := m2.Params["ovirt/uuid"].(string)

			if ovirtVM, err2 := p.GetVm(uuid); err2 != nil {
				p.RemoveVm(uuid)
				l.Errorf(fmt.Sprintf("Failed to get specific info for ovirt vm, %s. %v", uuid, err2))
			} else {
				if err2 := p.SaveDrpMachineFromOvirtVM(l, m2, ovirtVM); err2 != nil {
					p.RemoveVm(ovirtVM.MustId())
					l.Errorf(fmt.Sprintf("Save Machine Failed: %s", err2.Error()))
				}
			}
		}
		return nil
	}

	// Delete the machine
	if e.Action == "delete" {
		if uuid, ok := m.Params["ovirt/uuid"].(string); ok {
			if err := p.RemoveVm(uuid); err != nil {
				l.Errorf(fmt.Sprintf("Failed to destroy machine: %s: %s", uuid, err.Error()))
			}
		}
		return nil
	}

	return nil
}

func main() {
	plugin.InitApp("ovirt", "Provides tools to control ovirt VMs", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
