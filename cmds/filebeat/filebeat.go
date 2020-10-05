package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > filebeat.rst"
//go:generate rm content.yaml

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

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
		AutoStart:     true,
		OptionalParams: []string{
			"filebeat/path",
			"filebeat/mode",
			"filebeat/tcp",
		},
		Content: contentYamlString,
	}
)

type logbuf struct {
	data []byte
}

type Plugin struct {
	logs chan *logbuf
	cm   *sync.RWMutex
	tcp  string
	wc   io.WriteCloser
}

func (p *Plugin) SelectEvents() []string {
	return []string{"*.*.*"}
}

func (p *Plugin) writer(l logger.Logger) {
	defer func() {
		if p.wc != nil {
			p.wc.Close()
		}
	}()

	tcpF := func() {
		var err error
		if p.wc != nil {
			p.wc.Close()
			p.wc = nil
		}
		for {
			p.wc, err = net.Dial("tcp", p.tcp)
			if err == nil {
				break
			}
			l.Errorf("Failed to connect to filebeat on %s: %v", p.tcp, err)
			time.Sleep(3 * time.Second)
		}
	}
	if p.tcp != "" {
		tcpF()
	}

	for buf := range p.logs {
		if buf.data[len(buf.data)-1] != '\n' {
			buf.data = append(buf.data, '\n')
		}
		_, err := p.wc.Write(buf.data)
		if p.tcp != "" && err != nil {
			l.Errorf("Failed to send event to filebeat on %s: %v", p.tcp, err)
			tcpF()
		}
	}
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	p.cm.Lock()
	defer p.cm.Unlock()
	close(p.logs)

	mode, ok := config["filebeat/mode"].(string)
	if !ok {
		return utils.ConvertError(500, fmt.Errorf("filebeat/mode not a string"))
	}

	switch mode {
	case "file":
		tgtName, ok := config["filebeat/path"].(string)
		if !ok {
			return utils.ConvertError(500, fmt.Errorf("filebeat/path not a string"))
		}
		dir := filepath.Dir(tgtName)
		if merr := os.MkdirAll(dir, 0700); merr != nil {
			return utils.ConvertError(400, merr)
		}
		// Make sure file is there.
		f, e := os.OpenFile(tgtName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			err := utils.ConvertError(400, e)
			err.Errorf("file: %s", tgtName)
			return err
		}
		p.wc = f
	case "tcp":
		p.tcp, ok = config["filebeat/tcp"].(string)
		if !ok {
			return utils.ConvertError(500, fmt.Errorf("filebeat/tcp not a string"))
		}
		p.wc = nil
	default:
		return utils.ConvertError(400, fmt.Errorf("filebeat/mode is not valid: %s", mode))
	}

	p.logs = make(chan *logbuf, 16)
	go p.writer(l)
	return nil
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
	lb := &logbuf{
		data: text,
	}
	p.cm.RLock()
	if len(p.logs) == cap(p.logs) {
		p.cm.RUnlock()
		l.Errorf("Exceeded 16 queued log writes, dropping event.")
		return nil
	}
	p.logs <- lb
	p.cm.RUnlock()
	return nil
}

func main() {
	p := &Plugin{
		logs: make(chan *logbuf, 16),
		cm:   &sync.RWMutex{},
	}
	plugin.InitApp("filebeat", "Sends events to filebeat event.", version, &def, p)

	err := plugin.App.Execute()
	p.cm.Lock()
	close(p.logs)
	time.Sleep(500 * time.Millisecond)
	if err != nil {
		os.Exit(1)
	}
}
