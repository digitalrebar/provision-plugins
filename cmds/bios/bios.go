package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > bios.rst"
//go:generate rm content.yaml
//go:generate ./build-drp-bioscfg.sh
//go:generate go-bindata -pkg main -o embed.go -prefix embedded embedded/...

import (
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
		Name:          "bios",
		Version:       version,
		PluginVersion: 4,
		AutoStart:     true,
		Content:       contentYamlString,
	}
)

type Plugin struct {
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, data map[string]interface{}) *models.Error {
	return nil
}

func (p *Plugin) Unpack(thelog logger.Logger, path string) error {
	return RestoreAssets(path, "")
}

func main() {
	plugin.InitApp("raid", "Allows in-band BIOS configuration on a machine", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
