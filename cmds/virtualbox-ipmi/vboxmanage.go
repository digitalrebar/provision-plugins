package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/rackn/provision-plugins/v4/utils"
)

func (p *Plugin) vboxmanageCmd(l logger.Logger, ma *models.Action, command []string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer

	commands := []string{}
	commands = append(commands, "-u")
	commands = append(commands, p.theuser)
	commands = append(commands, p.thecommand)
	commands = append(commands, command...)

	cmd := exec.Command("sudo", commands...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	l.Infof("Ran: %s %v\n", p.thecommand, command)
	l.Debugf("sto: %s\n", string(stdout.Bytes()))
	l.Debugf("ste: %s\n", string(stderr.Bytes()))
	l.Debugf("Err: %v\n", err)
	return stdout.Bytes(), stderr.Bytes(), err
}

type VmListEntry struct {
	name string
	id   string
}

func (p *Plugin) getDrpMachineByVboxId(l logger.Logger, id string) (*models.Machine, *models.Error) {
	return utils.GetDrpMachineByParam(p.session, "virtualbox/id", strings.ToUpper(id))
}

func (p *Plugin) getDrpMachineByVboxName(l logger.Logger, name string) (*models.Machine, *models.Error) {
	return utils.GetDrpMachineByParam(p.session, "virtualbox/name", name)
}

func (p *Plugin) updateDrpMachineFromVmData(l logger.Logger, m *models.Machine, data map[string]string) {
	// Gather mac addresses
	for k, v := range data {
		if !strings.HasPrefix(k, "macaddress") {
			continue
		}

		hw := ""
		for ii := 0; ii < len(v); ii += 2 {
			if ii > 0 {
				hw += ":"
			}
			hw += v[ii : ii+2]
		}
		m.HardwareAddrs = append(m.HardwareAddrs, strings.ToLower(hw))
	}
}

func (p *Plugin) createDrpMachineFromVmData(l logger.Logger, m *models.Machine, data map[string]string) error {
	if m == nil {
		m = &models.Machine{Name: data["name"]}
	}
	m.Fill()
	m.Params["virtualbox/id"] = strings.ToUpper(data["UUID"])
	m.Params["virtualbox/name"] = m.Name
	m.Params["machine-plugin"] = p.name

	p.updateDrpMachineFromVmData(l, m, data)

	if err := p.session.CreateModel(m); err != nil {
		return err
	}
	return nil
}

func (p *Plugin) saveDrpMachineFromVmData(
	l logger.Logger,
	oldm, m *models.Machine,
	data map[string]string) (*models.Machine, error) {
	p.updateDrpMachineFromVmData(l, m, data)

	newM, err := p.session.PatchTo(oldm, m)
	if err != nil {
		return nil, err
	}
	return newM.(*models.Machine), nil
}

func (p *Plugin) vboxGetVmInfo(l logger.Logger, id string) (map[string]string, error) {
	out, se, err := p.vboxmanageCmd(l, nil, []string{"showvminfo", id, "--machinereadable"})
	if err != nil {
		return nil, fmt.Errorf("Error: %v, %s\n", err, string(se))
	}

	answer := map[string]string{}
	infoScanner := bufio.NewScanner(strings.NewReader(string(out)))
	for infoScanner.Scan() {
		infoLine := infoScanner.Text()
		parts := strings.SplitN(infoLine, "=", 2)
		trim0 := strings.Trim(parts[0], "\"")
		trim1 := strings.Trim(parts[1], "\"")
		answer[trim0] = trim1
	}

	return answer, nil
}

func (p *Plugin) createVboxVmFromDrpMachine(
	l logger.Logger,
	m *models.Machine, params map[string]interface{}) (map[string]string, bool, error) {
	diskSizeInMB := utils.GetParamOrInt(params, "virtualbox/disk-size-mb", 20480)
	memSizeInMB := utils.GetParamOrInt(params, "virtualbox/mem-size-mb", 2048)
	vramSizeInMB := utils.GetParamOrInt(params, "virtualbox/vram-size-mb", 128)
	cpuCount := utils.GetParamOrInt(params, "virtualbox/cpus", 2)
	vmid := utils.GetParamOrString(params, "virtualbox/id", "")
	if vmid == "" {
		vmid = m.UUID()
	}

	/* GREG: validate hostonly network and create subnet if missing. */

	if vmid != "" {
		if vm, err := p.vboxGetVmInfo(l, vmid); err == nil {
			return vm, true, nil
		}
	}

	// Create the vm
	createArgs := []string{"createvm", "--name", m.Name, "--register"}
	if vmid != "" {
		createArgs = append(createArgs, "--uuid", vmid)
	}
	so, se, err := p.vboxmanageCmd(l, nil, createArgs)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to create vbox machine: %v %s", err, string(se))
	}
	re := regexp.MustCompile("UUID: ([a-fA-F0-9-]+)")
	parts := re.FindStringSubmatch(string(so))
	if parts == nil {
		p.destroyVboxVm(l, m.Name)
		return nil, false, fmt.Errorf("Failed to parse create vbox machine: %s", string(so))
	}
	vmid = parts[1]
	m.Params["virtualbox/id"] = strings.ToUpper(vmid)
	m.Params["virtualbox/name"] = m.Name
	m.Params["machine-plugin"] = p.name

	// Attach Storage
	attachController := []string{"storagectl", m.Name,
		"--name", "SATA Controller", "--add", "sata", "--controller", "IntelAHCI"}
	_, se, err = p.vboxmanageCmd(l, nil, attachController)
	if err != nil {
		p.destroyVboxVm(l, vmid)
		return nil, false, fmt.Errorf("Failed to create vbox hd machine: %v %s", err, string(se))
	}

	hdName := fmt.Sprintf("%s.vdi", m.Name)
	filePath := hdName
	if p.path != "" {
		filePath = filepath.Join(p.path, m.Name, hdName)
	}
	createHdArgs := []string{"createmedium", "disk",
		"--filename", filePath,
		"--size", fmt.Sprintf("%d", diskSizeInMB)}
	_, se, err = p.vboxmanageCmd(l, nil, createHdArgs)
	if err != nil {
		p.destroyVboxVm(l, vmid)
		return nil, false, fmt.Errorf("Failed to create vbox hd machine: %v %s", err, string(se))
	}

	attachHd := []string{"storageattach", m.Name,
		"--storagectl", "SATA Controller",
		"--port", "0", "--device", "0", "--type", "hdd",
		"--medium", filePath}
	_, se, err = p.vboxmanageCmd(l, nil, attachHd)
	if err != nil {
		p.vboxmanageCmd(l, nil, []string{"closemedium", "disk", filePath, "--delete"})
		p.destroyVboxVm(l, vmid)
		return nil, false, fmt.Errorf("Failed to attach disk to controller: %v %s", err, string(se))
	}

	// Modify VM To our liking
	modifyArgs := []string{"modifyvm", m.Name,
		"--ioapic", "on",
		"--cpus", fmt.Sprintf("%d", cpuCount),
		"--boot1", "net", "--boot2", "disk", "--boot3", "none", "--boot4", "none",
		"--memory", fmt.Sprintf("%d", memSizeInMB), "--vram", fmt.Sprintf("%d", vramSizeInMB),
		"--nic1", "hostonly", "--hostonlyadapter1", "vboxnet0", "--nictype1", "82545EM",
		"--nic2", "nat", "--nictype2", "82545EM"}
	_, se, err = p.vboxmanageCmd(l, nil, modifyArgs)
	if err != nil {
		p.destroyVboxVm(l, vmid)
		return nil, false, fmt.Errorf("Failed to modify vm: %v %s", err, string(se))
	}
	vm, err := p.vboxGetVmInfo(l, vmid)
	return vm, false, err
}

func (p *Plugin) startVboxVm(l logger.Logger, id string) error {
	data, err := p.vboxGetVmInfo(l, id)
	if err != nil {
		return err
	}

	if v, ok := data["VMState"]; ok && v == "running" {
		return fmt.Errorf("startvm failed: Machine is already running")
	}

	// Want to stop the vm if not stopped.
	_, se, err := p.vboxmanageCmd(l, nil, []string{"startvm", id})
	if err != nil {
		return fmt.Errorf("startvm failed: %v %s\n", err, string(se))
	}

	return nil
}

func (p *Plugin) destroyVboxVm(l logger.Logger, id string) error {
	/* Delete process
	   vboxmanage controlvm test-3 poweroff
	   # loop testing for VBoxManage: error: Cannot unregister the machine 'test-3' while it is locked
	   vboxmanage unregistervm test-3 --delete
	*/
	data, err := p.vboxGetVmInfo(l, id)
	if err != nil {
		return err
	}

	if v, ok := data["VMState"]; ok && v == "running" {
		// Want to stop the vm if not stopped.
		_, se, err := p.vboxmanageCmd(l, nil, []string{"controlvm", id, "poweroff"})
		if err != nil {
			return fmt.Errorf("poweroff failed: %v %s\n", err, string(se))
		}
	}

	// Try to unregister 10 times
	for count := 0; count < 10; count += 1 {
		out, se, err := p.vboxmanageCmd(l, nil, []string{"unregistervm", id, "--delete"})
		if strings.Index(string(se), "while it is locked") != -1 {
			time.Sleep(1000)
			continue
		}
		if strings.Index(string(out), "while it is locked") != -1 {
			time.Sleep(1000)
			continue
		}
		return err
	}

	return fmt.Errorf("Failed to destroy VM because it is locked: %s", id)
}

func (p *Plugin) vboxGetVms(l logger.Logger) ([]VmListEntry, error) {
	out, se, err := p.vboxmanageCmd(l, nil, []string{"list", "vms"})
	if err != nil {
		return nil, fmt.Errorf("Error: %v, %s\n", err, string(se))
	}

	answer := []VmListEntry{}
	lineRe := regexp.MustCompile("^\"([a-zA-Z0-9.-]+)\" {([a-zA-z0-9-]+)}$")
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		parts := lineRe.FindStringSubmatch(line)
		if parts == nil {
			continue
		}

		// We return upper here because dmidecode return upper.  vboxmanage takes both.
		answer = append(answer, VmListEntry{name: parts[1], id: strings.ToUpper(parts[2])})
	}
	return answer, nil
}

func (p *Plugin) getDrpMachineParams(uuid string) (map[string]interface{}, error) {
	req := p.session.Req().UrlFor("machines", uuid, "params")
	req.Params("aggregate", "true")
	res := map[string]interface{}{}
	if err := req.Do(&res); err != nil {
		return nil, fmt.Errorf("Failed to fetch params %v: %v", uuid, err)
	}
	return res, nil
}
