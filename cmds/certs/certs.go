package main

//go:generate drbundler content content.go
//go:generate drbundler content content.yaml
//go:generate sh -c "drpcli contents document content.yaml > certs.rst"
//go:generate rm content.yaml

import (
	"fmt"
	"os"

	"github.com/cloudflare/cfssl/log"
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
		Name:          "certs",
		Version:       version,
		AutoStart:     true,
		PluginVersion: 2,
		AvailableActions: []models.AvailableAction{
			{
				Command: "makeroot",
				Model:   "machines",
				RequiredParams: []string{
					"certs/root",
				},
				OptionalParams: []string{
					"certs/root-pw",
				},
			},
			{
				Command: "deleteroot",
				Model:   "machines",
				RequiredParams: []string{
					"certs/root",
					"certs/root-pw",
				},
			},
			{
				Command: "signcert",
				Model:   "machines",
				RequiredParams: []string{
					"certs/root",
					"certs/root-pw",
					"certs/csr",
				},
			},
			{
				Command: "getca",
				Model:   "machines",
				RequiredParams: []string{
					"certs/root",
				},
			},
			{
				Command: "makeroot",
				Model:   "plugins",
				RequiredParams: []string{
					"certs/root",
				},
				OptionalParams: []string{
					"certs/root-pw",
				},
			},
			{
				Command: "deleteroot",
				Model:   "plugins",
				RequiredParams: []string{
					"certs/root",
				},
			},
			{
				Command: "signcert",
				Model:   "plugins",
				RequiredParams: []string{
					"certs/root",
					"certs/csr",
				},
			},
			{
				Command: "getca",
				Model:   "plugins",
				RequiredParams: []string{
					"certs/root",
				},
			},
			{
				Command: "list",
				Model:   "plugins",
			},
			{
				Command: "show",
				Model:   "plugins",
				RequiredParams: []string{
					"certs/root",
				},
			},
		},
		Content: contentYamlString,
	}
)

type Plugin struct {
	session *api.Client
	name    string

	signers map[string]*SignerWrapper
}

func (p *Plugin) certsProfileName() string {
	return fmt.Sprintf("%s-data", p.name)
}

func (p *Plugin) Config(thelog logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	thelog.Infof("Config: %v\n", config)
	p.session = session
	p.signers = map[string]*SignerWrapper{}
	if name, err := utils.ValidateStringValue("Name", config["Name"]); err != nil {
		p.name = "unknown"
	} else {
		p.name = name
	}

	b, err := session.ExistsModel("profiles", p.certsProfileName())
	if err != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to check for certs profile: %v", err))
	}
	if !b {
		profile := &models.Profile{
			Name: p.certsProfileName(),
			Params: map[string]interface{}{
				"certs/data": map[string]interface{}{},
			},
		}
		if err := session.CreateModel(profile); err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to create for certs profile: %v", err))
		}
	}

	if err := p.loadSigners(); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) IsPresentAndAuth(root string, ma *models.Action) (*SignerWrapper, *models.Error) {
	s, ok := p.signers[root]
	if !ok {
		return nil, utils.MakeError(404, fmt.Sprintf("root not found: %s", root))
	}

	plugin := &models.Plugin{}
	// Plugin actions don't require password, remarshal into a plugin (if it fails it is not a plugin)
	if e := models.Remarshal(ma.Model, &plugin); e != nil {
		authKey, ok := ma.Params["certs/root-pw"].(string)
		if !ok {
			return nil, utils.MakeError(400, fmt.Sprintf("Missing certs root-pw: %v", e))
		}
		if s.Data.AuthKey != authKey {
			return nil, utils.MakeError(403, fmt.Sprintf("Not allowed to access: %s", root))
		}
	}
	return s, nil
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, err *models.Error) {
	root, ok := ma.Params["certs/root"].(string)
	if !ok && ma.Command != "list" {
		err = utils.MakeError(400, "Missing certs root")
		return
	}

	switch ma.Command {
	case "list":
		arr := []CertData{}
		for _, cd := range p.signers {
			arr = append(arr, cd.Data)
		}
		answer = arr

	case "show":
		if tmp, ok := p.signers[root]; !ok {
			err = utils.MakeError(404, fmt.Sprintf("Root %s not found", root))
		} else {
			answer = tmp.Data
		}

	case "makeroot":
		key := ""
		if nkey, ok := ma.Params["certs/root-pw"]; ok {
			key = nkey.(string)
		}
		answer, err = p.newRootCertificate(root, key)

	case "deleteroot":
		if _, err2 := p.IsPresentAndAuth(root, ma); err2 != nil {
			err = err2
			return
		}
		answer, err = p.deleteRootCertificate(root)

	case "signcert":
		s, err2 := p.IsPresentAndAuth(root, ma)
		if err2 != nil {
			err = err2
			return
		}
		csr, ok := ma.Params["certs/csr"].(string)
		if !ok {
			err = utils.MakeError(400, "Missing certs csr")
			return
		}
		profile, ok := ma.Params["certs/profile"].(string)
		if !ok {
			profile = "default"
		}

		answer, err = p.signCert(s, profile, csr)

	case "getca":
		answer, err = p.getCa(root)
	}

	return
}

func (p *Plugin) Validate(thelog logger.Logger, session *api.Client) (interface{}, *models.Error) {
	return def, nil
}

func main() {
	log.Level = log.LevelCritical
	plugin.InitApp("certs", "Certificate Management Helpers ", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
