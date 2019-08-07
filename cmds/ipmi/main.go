package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > ipmi.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
)

var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "ipmi",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    false,
		AutoStart:     true,
		AvailableActions: []models.AvailableAction{
			models.AvailableAction{Command: "poweron",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "poweroff",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "powercycle",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "nextbootpxe",
				Model: "machines",
				RequiredParams: []string{
					"detected-bios-mode",
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "nextbootdisk",
				Model: "machines",
				RequiredParams: []string{
					"detected-bios-mode",
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "forcebootpxe",
				Model: "machines",
				RequiredParams: []string{
					"detected-bios-mode",
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "forcebootdisk",
				Model: "machines",
				RequiredParams: []string{
					"detected-bios-mode",
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
			models.AvailableAction{Command: "identify",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{"ipmi/identify-duration"},
			},
			models.AvailableAction{Command: "powerstatus",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
			},
		},
		Content: contentYamlString,
	}
)

type driver interface {
	Name() string
	Probe(l logger.Logger, address, username, password string) bool
	Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error)
}

type Plugin struct {
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (err *models.Error) {
	if info, infoErr := session.Info(); err == nil {
		found := false
		for _, feature := range info.Features {
			if feature == "secure-param-upgrade" {
				found = true
			}
		}
		if found == false {
			return &models.Error{
				Code:     500,
				Model:    "plugin",
				Key:      "ipmi",
				Type:     "rpc",
				Messages: []string{"dr-provision missing secure-param-upgrade"},
			}
		}
	} else {
		return infoErr.(*models.Error)
	}
	return
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	var ipmiDriver driver

	switch ma.Params["ipmi/mode"].(string) {
	case "ipmitool":
		ipmiDriver = &ipmi{}
	case "racadmn", "racadm":
		ipmiDriver = &racadm{}
	case "redfish":
		ipmiDriver = &redfish{}
	default:
		err = &models.Error{Code: 404,
			Model:    "plugin",
			Key:      "ipmi",
			Type:     "rpc",
			Messages: []string{fmt.Sprintf("Invalid mode: %v", ma.Params["ipmi/mode"])},
		}
		return
	}
	if !ipmiDriver.Probe(l,
		ma.Params["ipmi/address"].(string),
		ma.Params["ipmi/username"].(string),
		ma.Params["ipmi/password"].(string)) {
		err = &models.Error{
			Code:     404,
			Model:    "plugin",
			Key:      "ipmi",
			Type:     "rpc",
			Messages: []string{fmt.Sprintf("Unavailable ipmi driver: %s", ipmiDriver.Name())},
		}
		return
	}
	supported := false
	supported, answer, err = ipmiDriver.Action(l, ma)
	if !supported {
		err = &models.Error{
			Code:     404,
			Model:    "plugin",
			Key:      "ipmi",
			Type:     "rpc",
			Messages: []string{fmt.Sprintf("Action %s not supported on ipmi driver: %s", ma.Command, ipmiDriver.Name())},
		}
	}
	return
}

func main() {
	plugin.InitApp("ipmi", "Provides out-of-band IPMI controls", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
