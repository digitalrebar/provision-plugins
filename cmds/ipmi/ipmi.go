package main

import (
	"os/exec"
	"strconv"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
)

type ipmi struct {
	username, password, address string
	port                        int
}

func (i *ipmi) Name() string {
	return "ipmi"
}

func (i *ipmi) run(cmd ...string) ([]byte, error) {
	args := []string{"-H", i.address, "-U", i.username, "-P", i.password, "-I", "lanplus"}
	if i.port != 0 {
		args = append(args, "-p", strconv.Itoa(i.port))
	}

	args = append(args, cmd...)
	return exec.Command("ipmitool", args...).CombinedOutput()
}

func (i *ipmi) Probe(l logger.Logger, address string, port int, username, password string) bool {
	i.address = address
	i.port = port
	i.username = username
	i.password = password
	res, err := exec.Command("ipmitool", "-V").CombinedOutput()
	if len(res) > 0 && err == nil {
		return true
	}
	l.Warnf("Missing ipmitool")
	return false
}

func (i *ipmi) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	bootOptionsStick := []string{"chassis", "bootparam", "set", "bootflag", "none", "options=no-power,no-reset,no-watchdog,no-timeout"}
	cmds := [][]string{}
	switch ma.Command {
	case "powerstatus":
		cmds = append(cmds, []string{"chassis", "power", "status"})
	case "poweron":
		cmds = append(cmds, []string{"chassis", "power", "on"})
	case "poweroff":
		cmds = append(cmds, []string{"chassis", "power", "off"})
	case "powercycle":
		cmds = append(cmds, []string{"chassis", "power", "cycle"})
	case "nextbootpxe":
		bootCmd := []string{"chassis", "bootdev", "pxe"}
		if ma.Params["detected-bios-mode"].(string) == "uefi" {
			bootCmd = append(bootCmd, "options=efiboot")
		}
		cmds = append(cmds, bootOptionsStick, bootCmd)
	case "nextbootdisk":
		bootCmd := []string{"chassis", "bootdev", "disk"}
		if ma.Params["detected-bios-mode"].(string) == "uefi" {
			bootCmd = append(bootCmd, "options=efiboot")
		}
		cmds = append(cmds, bootOptionsStick, bootCmd)
	case "forcebootpxe":
		bootCmd := []string{"chassis", "bootdev", "pxe"}
		if ma.Params["detected-bios-mode"].(string) == "uefi" {
			// older versions of ipmitool have issues setting efoboot and persistent at the same time.
			// use raw commands to do it instead.
			bootCmd = []string{"raw", "0x00", "0x08", "0x05", "0xe0", "0x04", "0x00", "0x00", "0x00"}
		} else {
			bootCmd = append(bootCmd, "options=persistent")
		}
		cmds = append(cmds, bootOptionsStick, bootCmd)
	case "forcebootdisk":
		bootCmd := []string{"chassis", "bootdev", "disk"}
		if ma.Params["detected-bios-mode"].(string) == "uefi" {
			// older versions of ipmitool have issues setting efoboot and persistent at the same time.
			// use raw commands to do it instead.
			bootCmd = []string{"raw", "0x00", "0x08", "0x05", "0xe0", "0x08", "0x00", "0x00", "0x00"}
		} else {
			bootCmd = append(bootCmd, "options=persistent")
		}
		cmds = append(cmds, bootOptionsStick, bootCmd)
	case "identify":
		blink := 15
		if val, ok := ma.Params["ipmi/identify-duration"]; ok {
			blink = val.(int)
		}
		if blink > 0 {
			cmds = append(cmds, []string{"chassis", "identify", strconv.Itoa(blink)})
		} else {
			cmds = append(cmds, []string{"chassis", "identify"})
		}
	default:
		return
	}
	supported = true
	for _, cmd := range cmds {
		out, cmdErr := i.run(cmd...)
		if cmdErr != nil {
			err = &models.Error{
				Code:  404,
				Model: "plugin",
				Key:   "ipmi",
			}
			err.Errorf("ipmi error: %v", cmdErr)
			return
		}
		res = string(out)
	}
	return
}
