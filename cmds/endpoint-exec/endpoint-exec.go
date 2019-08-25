// RackN Provision Plugins
// Copyright 2019, RackN
// License: RackN Limited Use

package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > endpoint-exec.rst"
//go:generate rm content.yaml

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
)

// ==== PRIMARY ENTRY POINT FOR PLUGIN ====

// def information is passed to DRP during plugin registration
var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "endpoint-exec",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    false,
		AutoStart:     true,
		AvailableActions: []models.AvailableAction{
			{
				Command: "endpointExecDo",
				Model:   "machines",
				RequiredParams: []string{
					"endpoint-exec/action",
				},
				OptionalParams: []string{
					"endpoint-exec/parameters",
				},
			},
		},
		RequiredParams: []string{
			"endpoint-exec/actions",
		},
		Content: contentYamlString,
	}
)

type action struct {
	Path     string
	BaseArgs string
}

// Plugin is the overall data holder for the plugin
// If you defined extra operational values or params, they are typically included here
type Plugin struct {
	drpClient *api.Client
	name      string

	actions map[string]*action
}

// Config handles the configuration call from the DRP Endpoint
func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.drpClient = session
	name, vverr := utils.ValidateStringValue("Name", config["Name"])
	if vverr != nil {
		name = "unknown"
	}
	p.name = name
	utils.SetErrorName(p.name)

	err := &models.Error{Type: "plugin", Model: "plugins", Key: name}

	actions := map[string]*action{}
	if rerr := models.Remarshal(config["endpoint-exec/actions"], &actions); rerr != nil {
		err.AddError(rerr)
	}
	p.actions = actions

	if err.HasError() != nil {
		return err
	}

	return nil
}

// ActionReturn the answer to the commands
type ActionReturn struct {
	ReturnCode  int
	StandardErr string
	StandardOut string
}

func (p *Plugin) postTrigger(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	// validate action and do
	var action string
	action, err = utils.ValidateStringValue("endpoint-exec/action", ma.Params["endpoint-exec/action"])
	if err != nil {
		return
	}

	localParams := ""
	localParams, err = utils.ValidateStringValue("endpoint-exec/parameters", ma.Params["endpoint-exec/parameters"])
	if err != nil {
		return
	}

	machine := &models.Machine{}
	machine.Fill()
	if rerr := models.Remarshal(ma.Model, &machine); rerr != nil {
		err = utils.ConvertError(400, rerr)
		return
	}

	act, ok := p.actions[action]
	if !ok {
		l.Infof("endpoint-exec action unknown: %s", action)
		answer = &ActionReturn{
			ReturnCode:  1,
			StandardErr: fmt.Sprintf("endpoint-exec attempted, but skipped because action unknown: %s", action),
			StandardOut: "",
		}
		return
	}

	params := []string{}
	if act.BaseArgs != "" {
		params = append(params, strings.Split(act.BaseArgs, " ")...)
	}
	if localParams != "" {
		params = append(params, strings.Split(localParams, " ")...)
	}

	cmd := exec.Command(act.Path, params...)
	// GREG: Add env variables here
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cerr := cmd.Run()
	se := errb.String()
	if cerr != nil && se == "" {
		se = fmt.Sprintf("Exec failed: %v", cerr)
	}

	answer = &ActionReturn{ReturnCode: cmd.ProcessState.ExitCode(), StandardErr: se, StandardOut: outb.String()}
	return
}

// Action handles the action call from the DRP Endpoint
// using ma.Command, all registered actions should be handled
// reminder when validating params:
//   DRP will pass in required machine params if they exist in hierarchy
func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {

	switch ma.Command {
	case "endpointExecDo":
		answer, err = p.postTrigger(l, ma)

	default:
		err = utils.MakeError(404, fmt.Sprintf("Unknown command: %s", ma.Command))
	}

	return
}

// main is the entry point for the plugin code
// the InitApp routine should reflect the name and purpose of the plugin
func main() {
	plugin.InitApp("endpoint-exec", "Provides way to run scripts on the Endpoint", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
