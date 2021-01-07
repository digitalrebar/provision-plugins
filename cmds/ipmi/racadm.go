package main

import (
	"log"
	"net"
	"os/exec"
	"strconv"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
)

type racadm struct {
	username, password, address string
	port                        int
}

func (r *racadm) Name() string { return "racadm" }

func (r *racadm) run(cmd ...string) ([]byte, error) {
	addr := r.address
	if r.port != 0 {
		addr = net.JoinHostPort(addr, strconv.Itoa(r.port))
	}
	args := []string{"-r", addr, "-u", r.username, "-p", r.password}
	args = append(args, cmd...)
	return exec.Command("racadm", args...).CombinedOutput()
}

func (r *racadm) Probe(l logger.Logger, address string, port int, username, password string) bool {
	r.address = address
	r.username = username
	r.password = password
	r.port = port
	res, err := exec.Command("racadm", "version").CombinedOutput()
	if len(res) > 0 && err == nil {
		return true
	}
	log.Printf("%q", res)
	l.Warnf("Missing racadm")
	return false
}

func (r *racadm) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	cmds := [][]string{}
	switch ma.Command {
	case "powerstatus":
		cmds = append(cmds, []string{"serveraction", "powerstatus"})
	case "poweron":
		cmds = append(cmds, []string{"serveraction", "powerup"})
	case "poweroff":
		cmds = append(cmds, []string{"serveraction", "powerdown"})
	case "powercycle":
		cmds = append(cmds, []string{"serveraction", "powercycle"})
	case "nextbootpxe":
		cmds = append(cmds,
			[]string{"set", "iDRAC.ServerBoot.BootOnce", "Enabled"},
			[]string{"set", "iDRAC.serverboot.FirstBootDevice", "PXE"})
	case "nextbootdisk":
		cmds = append(cmds,
			[]string{"set", "iDRAC.ServerBoot.BootOnce", "Enabled"},
			[]string{"set", "iDRAC.serverboot.FirstBootDevice", "HDD"})
	case "forcebootpxe":
		cmds = append(cmds,
			[]string{"set", "iDRAC.ServerBoot.BootOnce", "Disabled"},
			[]string{"set", "iDRAC.serverboot.FirstBootDevice", "PXE"})
	case "forcebootdisk":
		cmds = append(cmds,
			[]string{"set", "iDRAC.ServerBoot.BootOnce", "Disabled"},
			[]string{"set", "iDRAC.serverboot.FirstBootDevice", "HDD"})
	case "identify":
		cmds = append(cmds, []string{"setled", "-l", "1"})
	default:
		return
	}
	supported = true
	for _, cmd := range cmds {
		out, cmdErr := r.run(cmd...)
		if cmdErr != nil {
			err = &models.Error{
				Code:  404,
				Model: "plugin",
				Key:   "ipmi",
			}
			err.Errorf("Racadm error: %v", cmdErr)
			err.Errorf("Racadm out: %s", string(out))
			return
		}
		res = string(out)
	}
	return
}
