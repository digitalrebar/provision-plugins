package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > filebeat.rst"
//go:generate rm content.yaml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

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
		Name:          "filebeat",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    true,
		OptionalParams: []string{
			"filebeat/path",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	Path string
	mux  sync.Mutex
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (err *models.Error) {
	p.Path, _ = config["filebeat/path"].(string)
	dir := filepath.Dir(p.Path)
	if merr := os.MkdirAll(dir, 0700); merr != nil {
		return utils.ConvertError(400, merr)
	}
	// Make sure file is there.
	f, e := os.OpenFile(p.Path, os.O_RDONLY|os.O_CREATE, 0600)
	if e == nil {
		f.Close()
	} else {
		err = utils.ConvertError(400, e)
		err.Errorf("file: %s", p.Path)
	}

	return
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) (err *models.Error) {
	// Filter out gohai data.
	if e.Type == "machines" {
		if obj, err := e.Model(); err != nil {
			l.Errorf("Could not filter machine object: %s", e.Key)
		} else {
			m := obj.(*models.Machine)
			m.Params["gohai-inventory"] = map[string]interface{}{}
			e.Object = m
		}
	}

	text, merr := json.Marshal(e)
	if merr != nil {
		return utils.ConvertError(400, merr)
	}

	return p.SendMessage(string(text) + "\n")
}

func (p *Plugin) SendMessage(msg string) *models.Error {
	p.mux.Lock()
	defer p.mux.Unlock()

	f, err := os.OpenFile(p.Path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return utils.ConvertError(400, err)
	}
	defer f.Close()
	_, err = f.WriteString(msg)
	if err != nil {
		return utils.ConvertError(400, err)
	}
	return nil
}

func main() {
	plugin.InitApp("filebeat", "Sends events to filebeat event.", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
