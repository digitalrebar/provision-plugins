package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > netbox.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"

	"github.com/digitalocean/go-netbox/netbox"
	"github.com/digitalocean/go-netbox/netbox/client"
	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
)

var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "netbox",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    true,
		RequiredParams: []string{
			"netbox/access-point",
			"netbox/superuser-token",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	drpClient    *api.Client
	netboxClient *client.NetBox
	name         string
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.drpClient = session
	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}
	utils.SetErrorName(p.name)

	accessPoint, err := utils.GetDrpStringParam(session, "plugins", p.name, "netbox/access-point")
	if err != nil {
		return err
	}
	token, err := utils.GetDrpStringParam(session, "plugins", p.name, "netbox/superuser-token")
	if err != nil {
		return err
	}

	p.netboxClient = netbox.NewNetboxWithAPIKey(accessPoint, token)
	if p.netboxClient == nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to attach to the netbox access point: %s", accessPoint))
	}

	return p.ImportNetboxDevices(l)
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) *models.Error {
	// Make sure we get a model.
	obj, err := e.Model()
	if err != nil {
		// Bad model ignore.
		return nil
	}

	switch e.Type {
	case "machines", "machine":
		m := obj.(*models.Machine)
		if e.Action == "delete" {
			p.RemoveNetboxDevice(l, m)
		} else {
			p.CreateOrUpdateNetboxDevice(l, m)
		}
	default:
	}

	return nil
}

func main() {
	plugin.InitApp("netbox", "Allows for integration with NetBox", version, &def, &Plugin{})
	if err := plugin.App.Execute(); err != nil {
		os.Exit(1)
	}
}
