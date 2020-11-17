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
			{Command: "poweron",
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
			{Command: "poweroff",
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
			{Command: "powercycle",
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
			{Command: "nextbootcd",
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
			{Command: "nextbootpxe",
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
			{Command: "nextbootdisk",
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
			{Command: "forcebootpxe",
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
			{Command: "forcebootdisk",
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
			{Command: "identify",
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
			{Command: "powerstatus",
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
			{Command: "status",
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
			{Command: "getInfo",
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
			{Command: "getBios",
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
			{Command: "getMemory",
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
			{Command: "getStorage",
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
			{Command: "getSimpleStorage",
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
			{Command: "getProcessor",
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
			{Command: "getNetworkInterfaces",
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
			{Command: "getEthernetInterfaces",
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
			{Command: "getSecureBoot",
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
			{Command: "getBoot",
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
			{Command: "mountVirtualMedia",
				Model: "machines",
				RequiredParams: []string{
					"ipmi/username",
					"ipmi/password",
					"ipmi/address",
					"ipmi/mode",
					"ipmi/virtual-media-url",
					"ipmi/virtual-media-boot",
				},
				OptionalParams: []string{
					"ipmi/port-ipmitool",
					"ipmi/port-racadm",
					"ipmi/port-redfish",
				},
			},
			{Command: "unmountVirtualMedia",
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
			{Command: "statusVirtualMedia",
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
		port = int(ma.Params["ipmi/port-ipmitool"].(float64) + 0.5)
	case "racadmn", "racadm":
		ipmiDriver = &racadm{}
		port = int(ma.Params["ipmi/port-racadm"].(float64) + 0.5)
	case "redfish":
		ipmiDriver = &redfish{}
		port = int(ma.Params["ipmi/port-redfish"].(float64) + 0.5)
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
