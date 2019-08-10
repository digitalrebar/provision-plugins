package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > slack.rst"
//go:generate rm content.yaml

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
		Name:          "slack",
		Version:       version,
		PluginVersion: 4,
		HasPublish:    true,
		RequiredParams: []string{
			"slack/bot-key",
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	bot *Bot
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (err *models.Error) {
	key, ok := config["slack/bot-key"].(string)
	if !ok {
		err = &models.Error{Code: 400, Model: "plugin", Key: "slack", Type: "rpc", Messages: []string{"Bad slack key"}}
		return
	}

	var berr error
	p.bot, berr = NewSlackBot(key)
	if berr != nil {
		err = &models.Error{Code: 400, Model: "plugin", Key: "slack", Type: "rpc", Messages: []string{berr.Error()}}
	}
	return
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) (err *models.Error) {
	perr := p.bot.Publish(e)
	if perr != nil {
		err = &models.Error{Code: 400, Model: "plugin", Key: "slack", Type: "rpc", Messages: []string{perr.Error()}}
	}
	return
}

func main() {
	plugin.InitApp("slack", "Sends events as specified slack bot.", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
