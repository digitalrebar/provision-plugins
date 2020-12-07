package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > netbox.rst"
//go:generate rm content.yaml

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/digitalocean/go-netbox/netbox"
	"github.com/digitalocean/go-netbox/netbox/client"
	"github.com/digitalrebar/logger"
	v4 "github.com/digitalrebar/provision-plugins/v4"
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
		RequiredParams: []string{
			"netbox/access-point",
			"netbox/superuser-token",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	*sync.Mutex
	drpClient    *api.Client
	netboxClient *client.NetBox
	name         string
	pmq          *utils.PerIdQueue
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.Lock()
	defer p.Unlock()
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

func (p *Plugin) SelectEvents() []string {
	return []string{
		"machines.*.*",
	}
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) *models.Error {
	// Make sure we get a model.
	obj, err := e.Model()
	if err != nil {
		// Bad model ignore.
		return nil
	}
	p.Lock()
	nb := p.netboxClient
	drp := p.drpClient
	p.Unlock()
	switch e.Type {
	case "machines":
		m := obj.(*models.Machine)
		if e.Action == "delete" {
			removeNetboxDevice(l, nb, m)
		} else {
			createOrUpdateNetboxDevice(l, nb, drp, m)
		}
	default:
	}

	return nil
}

func main() {
	plugin.InitApp("netbox", "Allows for integration with NetBox", version, &def, &Plugin{
		Mutex: &sync.Mutex{},
		pmq:   utils.NewQueues(context.Background(), 100),
	})
	if err := plugin.App.Execute(); err != nil {
		os.Exit(1)
	}
}
