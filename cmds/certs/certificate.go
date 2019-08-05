package main

import (
	"fmt"

	utils2 "github.com/VictorLowther/jsonpatch2/utils"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/rackn/provision-plugins/v4/utils"
)

type SignerWrapper struct {
	signer signer.Signer
	Data   CertData
}

type CertData struct {
	Name    string
	Cert    string
	Key     string
	AuthKey string
}

type ParamData map[string]CertData

var (
	basicConfig string = `{
  "signing": {
    "profiles": {
      "CA": {
        "expiry": "43800h",
        "usages": [
          "signing",
          "key encipherment",
          "server auth",
          "client auth"
        ]
      },
      "server": {
        "expiry": "43800h",
        "usages": [
          "signing",
          "key encipherment",
          "server auth",
          "client auth"
        ]
      },
      "peer": {
        "expiry": "43800h",
        "usages": [
          "signing",
          "key encipherment",
          "server auth",
          "client auth"
        ]
      },
      "client": {
        "expiry": "43800h",
        "usages": [
          "signing",
          "key encipherment",
          "client auth"
        ]
      }
    },
    "default": {
      "expiry": "43800h",
      "usages": [
        "signing",
        "key encipherment",
        "server auth",
        "client auth"
      ]
    }
  }
}
`
)

func (p *Plugin) buildSigner(root, authKey, certS, keyS string) (*SignerWrapper, error) {
	key, err := helpers.ParsePrivateKeyPEM([]byte(keyS))
	if err != nil {
		return nil, err
	}

	cert, err := helpers.ParseCertificatePEM([]byte(certS))
	if err != nil {
		return nil, err
	}

	s, err := local.NewSigner(key, cert, signer.DefaultSigAlgo(key), nil)
	if err != nil {
		return nil, err
	}

	c, err := config.LoadConfig([]byte(basicConfig))
	if err != nil {
		return nil, err
	}
	s.SetPolicy(c.Signing)

	answer := &SignerWrapper{
		signer: s,
		Data: CertData{
			Name:    root,
			Cert:    certS,
			Key:     keyS,
			AuthKey: authKey,
		},
	}

	return answer, nil
}

func (p *Plugin) newRootCertificate(root, authKey string) (string, *models.Error) {
	if _, ok := p.signers[root]; ok {
		return "", utils.MakeError(409, fmt.Sprintf("Root %s already exists", root))
	}

	// Make CSR for this root - use root as CN
	names := []csr.Name{
		{
			C:  "US",
			ST: "Texas",
			L:  "Austin",
			O:  "RackN",
			OU: "CA Services",
		},
	}
	req := csr.CertificateRequest{
		KeyRequest: csr.NewBasicKeyRequest(),
		CN:         root,
		Names:      names,
	}
	certB, _, keyB, err := initca.New(&req)
	if err != nil {
		return "", utils.MakeError(400, fmt.Sprintf("Failed to init ca: %v", err))
	}

	// If AuthKey is null, create one
	if authKey == "" {
		authKey = randString(32)
	}

	s, err := p.buildSigner(root, authKey, string(certB), string(keyB))
	if err != nil {
		return "", utils.MakeError(400, fmt.Sprintf("Failed to build signer: %v", err))
	}

	p.signers[root] = s
	if err2 := p.saveData(); err2 != nil {
		return "", err2
	}

	return authKey, nil
}

func (p *Plugin) deleteRootCertificate(root string) (string, *models.Error) {
	if _, ok := p.signers[root]; !ok {
		return "", utils.MakeError(404, fmt.Sprintf("Root %s does not exist", root))
	}

	delete(p.signers, root)
	if err2 := p.saveData(); err2 != nil {
		return "", err2
	}

	return "Success", nil
}

func (p *Plugin) signCert(s *SignerWrapper, profile, csr string) (string, *models.Error) {
	if profile == "" {
		profile = "default"
	}

	sigRequest := signer.SignRequest{
		Request: string(csr),
		Label:   s.Data.Name,
		Profile: profile,
	}

	cert, err := s.signer.Sign(sigRequest)
	if err != nil {
		return "", utils.MakeError(400, fmt.Sprintf("failed to sign cert: %v", err))
	}

	return string(cert), nil
}

func (p *Plugin) getCa(root string) (string, *models.Error) {
	s, ok := p.signers[root]
	if !ok {
		return "", utils.MakeError(404, fmt.Sprintf("root not found: %s", root))
	}
	return s.Data.Cert, nil
}

func (p *Plugin) saveData() *models.Error {
	pp := ParamData{}
	for k, s := range p.signers {
		pp[k] = s.Data
	}
	if len(pp) == 1 {
		return utils.AddDrpParam(p.session, "profiles", p.certsProfileName(), "certs/data", pp)
	} else {
		return utils.SetDrpParam(p.session, "profiles", p.certsProfileName(), "certs/data", nil, pp)
	}
}

func (p *Plugin) loadSigners() *models.Error {
	raw, err := utils.GetDrpParam(p.session, "profiles", p.certsProfileName(), "certs/data")
	if err != nil {
		return err
	}
	pp := ParamData{}
	if err2 := utils2.Remarshal(raw, &pp); err2 != nil {
		return utils.MakeError(400, fmt.Sprintf("Invalid Root data: %v", err2))
	}

	for k, cd := range pp {
		s, err := p.buildSigner(k, cd.AuthKey, cd.Cert, cd.Key)
		if err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to load: %s: %v", k, err))
		}
		p.signers[k] = s
	}

	return nil
}
