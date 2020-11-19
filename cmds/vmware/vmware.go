package main

//go:generate ./download-vibs.sh
//go:generate sh -c "cd process ; GOARCH=${GOHOSTARCH} GOOS=${GOHOSTOS} go build -o ../process-content"
//go:generate ./process-content
//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > vmware.rst"
//go:generate rm -f process-content content.yaml
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
		Name:          "vmware",
		Version:       version,
		PluginVersion: 4,
		AutoStart:     true,
		HasPublish:    false,
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
	plugin.InitApp("vmware", "Add VMware vSphere and vCloud capabilities.", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
