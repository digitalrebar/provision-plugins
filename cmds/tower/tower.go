package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > tower.rst"
//go:generate rm content.yaml

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

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
		Name:          "tower",
		Version:       version,
		PluginVersion: 2,
		HasPublish:    false,
		AvailableActions: []models.AvailableAction{
			models.AvailableAction{Command: "tower-register",
				Model: "machines",
				RequiredParams: []string{
					"tower/inventory",
					"tower/group",
				},
			},
			models.AvailableAction{Command: "tower-delete",
				Model: "machines",
			},
			models.AvailableAction{Command: "tower-invoke",
				Model: "machines",
				RequiredParams: []string{
					"tower/job-template",
				},
				OptionalParams: []string{
					"tower/job-timeout",
					"tower/extra-vars",
				},
			},
			models.AvailableAction{Command: "tower-job-status",
				Model: "machines",
				OptionalParams: []string{
					"tower/job-id",
				},
			},
		},
		RequiredParams: []string{
			"tower/login",
			"tower/password",
			"tower/url",
		},
		OptionalParams: []string{},
		Content:        contentYamlString,
	}
)

type Plugin struct {
	towerLogin    string
	towerPassword string
	towerUrl      string

	name    string
	session *api.Client
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (err *models.Error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	p.session = session

	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}
	utils.SetErrorName(p.name)

	err = &models.Error{Type: "plugin", Model: "plugins", Key: p.name}
	var ok bool
	p.towerLogin, ok = config["tower/login"].(string)
	if !ok {
		err.Code = 400
		err.Errorf("Plugin %s is missing tower/login", p.name)
	}
	p.towerPassword, ok = config["tower/password"].(string)
	if !ok {
		err.Code = 400
		err.Errorf("Plugin %s is missing tower/password", p.name)
	}
	p.towerUrl, ok = config["tower/url"].(string)
	if !ok {
		err.Code = 400
		err.Errorf("Plugin %s is missing tower/url", p.name)
	}

	// Return the error
	if err.HasError() != nil {
		return
	}
	err = nil

	return
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	switch ma.Command {
	case "tower-register":
		answer, err = p.registerTower(ma.Model, ma.Params)

	case "tower-invoke":
		answer, err = p.invokeTower(ma.Model, ma.Params)

	case "tower-delete":
		answer, err = p.deleteTower(ma.Model, ma.Params)

	case "tower-job-status":
		answer, err = p.towerJobStatus(ma.Model, ma.Params)
	default:
		err = &models.Error{Code: 404,
			Model:    "plugin",
			Key:      "ipmi",
			Type:     "rpc",
			Messages: []string{fmt.Sprintf("Unknown command: %s", ma.Command)}}
	}

	return
}

func main() {
	plugin.InitApp("tower", "Provides controls for Tower/AWX", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
