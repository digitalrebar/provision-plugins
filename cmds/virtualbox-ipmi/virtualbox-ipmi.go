package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > virtualbox-ipmi.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
)

var (
	version = v4.RS_VERSION
	def     = models.PluginProvider{
		Name:          "virtualbox-ipmi",
		Version:       version,
		PluginVersion: 2,
		HasPublish:    true,
		AvailableActions: []models.AvailableAction{
			models.AvailableAction{Command: "createVm",
				Model: "plugins",
				RequiredParams: []string{
					"virtualbox/user",
				},
				OptionalParams: []string{
					"virtualbox/name",
					"virtualbox/cpus",
					"virtualbox/disk-size-mb",
					"virtualbox/mem-size-mb",
					"virtualbox/vram-size-mb",
					"virtualbox/id",
				},
			},
			models.AvailableAction{Command: "startVm",
				Model: "plugins",
				RequiredParams: []string{
					"virtualbox/user",
				},
				OptionalParams: []string{
					"virtualbox/id",
					"virtualbox/name",
				},
			},
			models.AvailableAction{Command: "stopVm",
				Model: "plugins",
				RequiredParams: []string{
					"virtualbox/user",
				},
				OptionalParams: []string{
					"virtualbox/id",
					"virtualbox/name",
				},
			},
			models.AvailableAction{Command: "destroyVm",
				Model: "plugins",
				RequiredParams: []string{
					"virtualbox/user",
				},
				OptionalParams: []string{
					"virtualbox/id",
					"virtualbox/name",
				},
			},
			models.AvailableAction{Command: "poweron",
				Model: "machines",
				RequiredParams: []string{
					"virtualbox/id",
				},
			},
			models.AvailableAction{Command: "poweroff",
				Model: "machines",
				RequiredParams: []string{
					"virtualbox/id",
				},
			},
			models.AvailableAction{Command: "powercycle",
				Model: "machines",
				RequiredParams: []string{
					"virtualbox/id",
				},
			},
			models.AvailableAction{Command: "nextbootpxe",
				Model: "machines",
				RequiredParams: []string{
					"virtualbox/id",
				},
			},
			models.AvailableAction{Command: "nextbootdisk",
				Model: "machines",
				RequiredParams: []string{
					"virtualbox/id",
				},
			},
		},
		RequiredParams: []string{
			"virtualbox/user",
		},
		OptionalParams: []string{
			"virtualbox/vm-path",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	thecommand string
	theuser    string
	name       string
	path       string

	session *api.Client
	lock    sync.Mutex
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}
	utils.SetErrorName(p.name)

	if vu, err := utils.GetDrpStringParam(session, "plugins", p.name, "virtualbox/user"); err != nil {
		return err
	} else {
		p.theuser = vu
	}

	if vu, err := utils.GetDrpStringParam(session, "plugins", p.name, "virtualbox/vm-path"); err != nil {
		p.path = ""
	} else {
		p.path = vu
	}

	p.session = session
	p.thecommand = "vboxmanage"
	answer, err2 := exec.Command("which", p.thecommand).Output()
	if err2 != nil {
		return utils.MakeError(404, fmt.Sprintf("Failed to find VBoxManage/vboxmanage. %v", err2))
	} else {
		p.thecommand = strings.TrimSpace(string(answer))
	}

	/* GREG: validate hostonly network and create vboxnet and subnet if missing. */

	vmlist, err2 := p.vboxGetVms(l)
	if err2 != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to list vms in VBoxManage/vboxmanage. %v", err2))
	}

	l.Infof("Found %d vms to process. \n", len(vmlist))
	for _, vm := range vmlist {
		// Does the machine exist already in DRP?
		l.Infof("Checking to see if %s already exists ...\n", vm.id)
		if m, err2 := p.getDrpMachineByVboxId(l, vm.id); err2 != nil {
			return err2
		} else if m != nil {
			l.Infof("%s already exists ... continuing\n", vm.id)
			continue
		}

		l.Infof("Getting info for %s \n", vm.id)
		vmdata, err2 := p.vboxGetVmInfo(l, vm.id)
		if err2 != nil {
			return utils.MakeError(400,
				fmt.Sprintf("Failed to get specific info for vbox machine, %s. %v", vm.id, err2))
		}

		l.Infof("Creating %s in DRP\n", vm.id)
		if err2 := p.createDrpMachineFromVmData(l, nil, vmdata); err2 != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to add machine, %s. %v", vm.id, err2))
		}
	}

	return nil
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	parm, ok := ma.Params["virtualbox/id"].(string)
	if !ok {
		parm, ok = ma.Params["virtualbox/name"].(string)
		if !ok {
			err = utils.MakeError(400, "Missing virtualbox id or name")
			return
		}
	}

	l.Infof("Action %s on %s\n", ma.Command, parm)

	switch ma.Command {
	case "poweron":
		p.lock.Lock()
		r := rand.Intn(500) + 1000
		time.Sleep(time.Duration(r) * time.Millisecond)
		p.lock.Unlock()
		out, _, err2 := p.vboxmanageCmd(l, ma, []string{"startvm", parm})
		if err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = string(out)
		}
	case "poweroff":
		out, _, err2 := p.vboxmanageCmd(l, ma, []string{"controlvm", parm, "poweroff"})
		if err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = string(out)
		}
	case "powercycle":
		p.lock.Lock()
		r := rand.Intn(500) + 1000
		time.Sleep(time.Duration(r) * time.Millisecond)
		p.lock.Unlock()
		out, _, err2 := p.vboxmanageCmd(l, ma, []string{"controlvm", parm, "reset"})
		if err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = string(out)
		}
	case "nextbootpxe", "nextbootdisk":
		// nothing, maybe one day
		answer = "Success"

	case "createVm":
		m := &models.Machine{}
		m.Fill()

		name, ok := ma.Params["virtualbox/name"].(string)
		if !ok {
			err = utils.MakeError(400, "Missing virtualbox name")
			l.Infof("Returning for %s on %s: %v %v\n", ma.Command, parm, answer, err)
			return
		}
		m.Name = name
		m.Params["virtualbox/disk-size-mb"] = utils.GetParamOrInt(ma.Params, "virtualbox/disk-size-mb", 20480)
		m.Params["virtualbox/mem-size-mb"] = utils.GetParamOrInt(ma.Params, "virtualbox/mem-size-mb", 2048)
		m.Params["virtualbox/vram-size-mb"] = utils.GetParamOrInt(ma.Params, "virtualbox/vram-size-mb", 128)
		m.Params["virtualbox/cpus"] = utils.GetParamOrInt(ma.Params, "virtualbox/cpus", 2)

		_, _, err2 := p.createVboxVmFromDrpMachine(l, m, m.Params)
		if err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			id := m.Params["virtualbox/id"].(string)

			if data, err2 := p.vboxGetVmInfo(l, id); err2 != nil {
				p.destroyVboxVm(l, id)
				err = utils.MakeError(400,
					fmt.Sprintf("Failed to get specific info for vbox machine, %s. %v", id, err2))
			} else {
				if err2 := p.createDrpMachineFromVmData(l, m, data); err2 != nil {
					p.destroyVboxVm(l, data["UUID"])
					err = utils.MakeError(400, fmt.Sprintf("Save Machine: %s: %s", ma.Command, err2.Error()))
				}
			}
			answer = fmt.Sprintf("Created %s", parm)
		}

	case "startVm":
		if err2 := p.startVboxVm(l, parm); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = fmt.Sprintf("Started %s", parm)
		}

	case "stopVm":
		_, _, err2 := p.vboxmanageCmd(l, ma, []string{"controlvm", parm, "poweroff"})
		if err2 != nil {
			err = utils.MakeError(409, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			answer = fmt.Sprintf("Stopped %s", parm)
		}

	case "destroyVm":
		if err2 := p.destroyVboxVm(l, parm); err2 != nil {
			err = utils.MakeError(400, fmt.Sprintf("Command failed: %s: %s", ma.Command, err2.Error()))
		} else {
			if m, err2 := p.getDrpMachineByVboxId(l, parm); err2 == nil && m != nil {
				if _, err2 := p.session.DeleteModel("machines", m.UUID()); err2 != nil {
					err = utils.MakeError(409, fmt.Sprintf("Failed to destroy machine: %v", err2.Error()))
				}
			} else {
				if m, err2 := p.getDrpMachineByVboxName(l, parm); err2 == nil && m != nil {
					if _, err2 := p.session.DeleteModel("machines", m.UUID()); err2 != nil {
						err = utils.MakeError(409, fmt.Sprintf("Failed to destroy machine: %v", err2.Error()))
					}
				}
			}
			answer = fmt.Sprintf("Destroy %s", parm)
		}

	default:
		err = utils.MakeError(404, fmt.Sprintf("Unknown command: %s", ma.Command))
	}

	l.Infof("Returning for %s on %s: %v %v\n", ma.Command, parm, answer, err)
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
	pname, ok := m.Params["machine-plugin"].(string)
	if !ok || pname != p.name {
		return nil
	}

	oldM := models.Clone(m).(*models.Machine)
	params, err := p.getDrpMachineParams(m.UUID())
	if err != nil {
		l.Errorf("Failed to get params for %s: %v", m.UUID(), err)
	}
	switch e.Action {
	// Create the machine
	case "create":
		data, exists, err := p.createVboxVmFromDrpMachine(l, m, params)
		if err != nil {
			l.Errorf("Failed to create: %s: %s", m.Name, err.Error())
			break
		}
		id := m.Params["virtualbox/id"].(string)
		if m, err = p.saveDrpMachineFromVmData(l, oldM, m, data); err != nil {
			p.destroyVboxVm(l, data["UUID"])
			l.Errorf("Save Machine: %s", err.Error())
			break
		}
		if exists {
			p.vboxmanageCmd(l, nil, []string{"controlvm", id, "poweroff"})
		}
		if err2 := p.startVboxVm(l, data["UUID"]); err2 != nil {
			p.destroyVboxVm(l, data["UUID"])
			l.Errorf("Start VM failed: %s", err2.Error())
		}
		return nil
	case "delete":
		if id, ok := m.Params["virtualbox/id"].(string); ok {
			if err := p.destroyVboxVm(l, id); err != nil {
				l.Errorf("Failed to destroy machine: %s: %s", id, err.Error())
			}
		}
		return nil
	}
	return nil
}

func main() {
	plugin.InitApp("virtualbox-ipmi",
		"Provides out-of-band IPMI controls for local VirtualBox VMs",
		version,
		&def,
		&Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
