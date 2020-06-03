package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > ipmi.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"

	"github.com/digitalrebar/logger"
	v4 "github.com/digitalrebar/provision-plugins/v4"
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
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
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
					"ipmi/identify-duration",
				},
			},
			models.AvailableAction{Command: "powerstatus",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "status",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getInfo",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getBios",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getMemory",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getStorage",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getSimpleStorage",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getProcessor",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getNetworkInterfaces",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getEthernetInterfaces",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getSecureBoot",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			models.AvailableAction{Command: "getBoot",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
		},
		Content: contentYamlString,
	}
)

type driver interface {
	Name() string
	Probe(l logger.Logger, address string, port int, username, password string) bool
	Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error)
}

type Plugin struct {
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (err *models.Error) {
	if info, infoErr := session.Info(); infoErr == nil {
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
	port := 0

	switch ma.Params["ipmi/mode"].(string) {
	case "ipmitool":
		ipmiDriver = &ipmi{}
		port = ma.Params["ipmi/port-ipmitool"].(int)
	case "racadmn", "racadm":
		ipmiDriver = &racadm{}
		port = ma.Params["ipmi/port-racadm"].(int)
	case "redfish":
		ipmiDriver = &redfish{}
		port = ma.Params["ipmi/port-redfish"].(int)
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
		port,
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
