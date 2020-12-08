// RackN Provision Plugins
// Copyright 2018, RackN
// License: RackN Limited Use

package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > callback.rst"
//go:generate rm content.yaml

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/digitalrebar/logger"
	v4 "github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision-plugins/v4/utils"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
)

// ==== PRIMARY ENTRY POINT FOR PLUGIN ====

// def information is passed to DRP during plugin registration
var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "callback",
		Version:       version,
		PluginVersion: 4,
		AutoStart:     true,
		AvailableActions: []models.AvailableAction{
			{
				Command: "callbackDo",
				Model:   "machines",
				RequiredParams: []string{
					"callback/action",
				},
				OptionalParams: []string{
					"callback/data-override",
				},
			},
		},
		RequiredParams: []string{
			"callback/callbacks",
		},
		OptionalParams: []string{
			"callback/auths",
			"callback/proxy",
		},
		Content: contentYamlString,
	}
)

type callback struct {
	Auth           string
	Auths          []string
	Url            string
	Method         string
	Retry          int
	Timeout        int
	Delay          int
	NoBody         bool
	JsonResponse   bool
	StringResponse bool
	Headers        map[string]string
	Aggregate      bool
	Decode         bool
	ExcludeParams  []string
}

type auth struct {
	AuthType string // basic, json-token, exec

	// Exec Path - should just return a string to use as password - it will be trimmed.
	Path string

	// Basic info
	Username string
	Password string

	// JSON Token Blob / Bearer
	Url           string // URL for auth
	Method        string // Method for AUTH
	Data          string // Data to send for auth
	Query         string // QueryString to send for auth
	TokenField    string // string field
	DurationField string // int field
	Retry         int    // Retry count
	Timeout       int    // Timeout total time
	Delay         int

	bearerCache           string
	bearerCacheTimeout    int
	bearerCacheLookupTime time.Time
}

type runningConfig struct {
	drpClient      *api.Client
	callbackClient *http.Client
	callbacks      map[string]*callback
	auths          map[string]*auth
	proxy          string
}

// Plugin is the overall data holder for the plugin
// If you defined extra operational values or params, they are typically included here
type Plugin struct {
	name  string
	rc    *runningConfig
	rcMux *sync.Mutex
	pmq   *utils.PerIdQueue
}

// Config handles the configuration call from the DRP Endpoint
func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) *models.Error {
	name, vverr := utils.ValidateStringValue("Name", config["Name"])
	if vverr != nil {
		name = "unknown"
	}
	p.name = name
	utils.SetErrorName(p.name)
	p.rcMux.Lock()
	defer p.rcMux.Unlock()
	rc := &runningConfig{}
	rc.drpClient = session
	err := &models.Error{Type: "plugin", Model: "plugins", Key: name}

	callbacks := map[string]*callback{}
	if rerr := models.Remarshal(config["callback/callbacks"], &callbacks); rerr != nil {
		err.AddError(rerr)
	}
	rc.callbacks = callbacks

	auths := map[string]*auth{}
	if rerr := models.Remarshal(config["callback/auths"], &auths); rerr != nil {
		err.AddError(rerr)
	}
	rc.auths = auths

	if sval, ok := config["callback/proxy"]; ok {
		rc.proxy = sval.(string)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if rc.proxy != "" {
		if purl, rerr := url.Parse(rc.proxy); err != nil {
			err.AddError(rerr)
		} else {
			tr.Proxy = http.ProxyURL(purl)
		}
	}
	rc.callbackClient = &http.Client{Transport: tr}

	if err.HasError() != nil {
		return err
	}
	p.rc = rc
	return nil
}

func (p *runningConfig) getJsonToken(l logger.Logger, auth *auth) (string, error) {
	out := "getJsonToken:\n"
	/*
		Url           string // URL for auth
		Method        string // Method for AUTH
		Data          string // Data to send for auth
		Query         string // QueryString to send for auth
		TokenField    string // string field
		DurationField string // int field
	*/
	count := -1
	dt := 7200
	if auth.Timeout > 0 {
		dt = auth.Timeout
	}
	et := time.Now().Add(time.Duration(dt) * time.Second)

auth_retry:
	count++
	if count > 0 && auth.Delay > 0 {
		time.Sleep(time.Duration(auth.Delay) * time.Second)
	}
	if time.Now().After(et) {
		e := utils.MakeError(400, "Timeout of auth call")
		e.AddError(fmt.Errorf("out: %s", out))
		return out, e
	}
	var b *bytes.Buffer
	if auth.Data != "" {
		b = bytes.NewBufferString(auth.Data)
	}
	req, _ := http.NewRequest(auth.Method, auth.Url, b)
	// Add the query string
	if auth.Query != "" {
		req.URL.RawQuery = auth.Query
	}

	resp2, rerr := p.callbackClient.Do(req)
	if rerr != nil {
		if count < auth.Retry {
			goto auth_retry
		}
		e := utils.MakeError(400, "Failed to get auth API token call")
		e.AddError(fmt.Errorf("out: %s", out))
		e.AddError(rerr)
		return out, e
	}
	defer resp2.Body.Close()

	out += fmt.Sprintf("auth request (%d): ---\n", count)
	out += fmt.Sprintf("auth request Data: %v\n", auth.Data)
	out += fmt.Sprintf("auth request Query: %v\n", auth.Query)
	out += "auth response: ---\n"
	out += fmt.Sprintf("auth response Status: %v\n", resp2.Status)
	out += fmt.Sprintf("auth response Headers: %v\n", resp2.Header)
	body, _ := ioutil.ReadAll(resp2.Body)
	out += "auth response Body: ---\n"
	out += fmt.Sprintf("auth body: %s\n", string(body))

	if resp2.StatusCode >= 400 {
		if count < auth.Retry {
			goto auth_retry
		}
		e := utils.MakeError(resp2.StatusCode, fmt.Sprintf("Auth API returned %d", resp2.StatusCode))
		e.AddError(fmt.Errorf("out: %s", out))
		return "", e
	}

	data := map[string]interface{}{}
	if jerr := json.Unmarshal(body, &data); jerr != nil {
		if count < auth.Retry {
			goto auth_retry
		}
		e := utils.MakeError(resp2.StatusCode, fmt.Sprintf("Json unmarshal failure: %s %v", string(body), jerr))
		e.AddError(fmt.Errorf("out: %s", out))
		return "", e
	}

	var val string
	if sval, ok := data[auth.TokenField]; ok {
		if val, ok = sval.(string); !ok {
			if count < auth.Retry {
				goto auth_retry
			}
			e := utils.MakeError(resp2.StatusCode, fmt.Sprintf("Json blob tokenfield is not a string: %s", string(body)))
			e.AddError(fmt.Errorf("out: %s", out))
			return "", e
		}
	} else {
		if count < auth.Retry {
			goto auth_retry
		}
		e := utils.MakeError(resp2.StatusCode, fmt.Sprintf("Json blob missing tokenfield: %s", string(body)))
		e.AddError(fmt.Errorf("out: %s", out))
		return "", e
	}

	tval := 3600
	if ival, ok := data[auth.DurationField]; ok {
		fval, ok := ival.(float64)
		if !ok {
			if count < auth.Retry {
				goto auth_retry
			}
			e := utils.MakeError(resp2.StatusCode, fmt.Sprintf("Json blob durationfield is not a number: %s", string(body)))
			e.AddError(fmt.Errorf("out: %s", out))
			return "", e
		}
		tval = int(fval)
	}

	out += fmt.Sprintf("auth token timeout: %d\n", tval)
	auth.bearerCache = val
	auth.bearerCacheTimeout = 3 * tval / 4
	auth.bearerCacheLookupTime = time.Now()
	return out, nil
}

func getExecString(l logger.Logger, auth *auth) (string, error) {
	out, cerr := exec.Command(auth.Path).CombinedOutput()
	if cerr != nil {
		e := utils.MakeError(400, fmt.Sprintf("Failed to execute: %s", auth.Path))
		e.AddError(fmt.Errorf("out: %s", string(out)))
		e.AddError(cerr)
		return "", e
	}
	return strings.TrimSpace(string(out)), nil
}

func getExecBearer(l logger.Logger, auth *auth) (string, error) {
	/*
		Path          string // URL for auth
	*/
	out := fmt.Sprintf("getExecBearer: %s\n", auth.Path)
	val, err := getExecString(l, auth)
	if err != nil {
		e := utils.MakeError(400, "Failed to get auths")
		e.AddError(fmt.Errorf("out: %s", out))
		e.AddError(err)
		return "", e
	}
	out += fmt.Sprintf("getExecString response: %s\n", val)

	tval := 3600
	auth.bearerCache = val
	auth.bearerCacheTimeout = 3 * tval / 4
	auth.bearerCacheLookupTime = time.Now()
	return out, nil
}

type remoteReq struct {
	p         *runningConfig
	l         logger.Logger
	cb        *callback
	action    string
	authindex int
	auths     []*auth
	cauth     *auth
	localUrl  string
	payload   []byte
}

func (rr *remoteReq) do() (answer interface{}, err *models.Error) {
	count := -1
	dt := 7200
	if rr.cb.Timeout > 0 {
		dt = rr.cb.Timeout
	}
	et := time.Now().Add(time.Duration(dt) * time.Second)

	out := fmt.Sprintf("Doing %s callback\n", rr.action)
cb_retry:
	count++
	if count > 0 && rr.cb.Delay > 0 {
		time.Sleep(time.Duration(rr.cb.Delay) * time.Second)
	}
	if time.Now().After(et) {
		e := utils.MakeError(400, "Timeout of callback call")
		e.AddError(fmt.Errorf("out: %s", out))
		return out, e
	}

	out += fmt.Sprintf("Attempt %s (%d)\n", rr.action, count)
	out += fmt.Sprintf("url: %s %s\n", rr.localUrl, rr.cb.Method)
	var req *http.Request
	if rr.cb.NoBody {
		req, _ = http.NewRequest(rr.cb.Method, rr.localUrl, nil)
	} else {
		req, _ = http.NewRequest(rr.cb.Method, rr.localUrl, bytes.NewBuffer(rr.payload))
	}

	if rr.cauth != nil {
		out += fmt.Sprintf("auth: %s\n", rr.cauth.AuthType)
		switch rr.cauth.AuthType {
		case "basic":
			req.SetBasicAuth(rr.cauth.Username, rr.cauth.Password)
		case "json-token":
			// JSON Token Blob / Bearer
			if rr.cauth.bearerCache == "" ||
				time.Now().Sub(rr.cauth.bearerCacheLookupTime) > time.Duration(rr.cauth.bearerCacheTimeout)*time.Second {
				jout, jterr := rr.p.getJsonToken(rr.l, rr.cauth)
				if jterr != nil {
					rr.cauth.bearerCache = ""
					out += fmt.Sprintf("Auth[%d] failed: %v\n", rr.authindex, jterr)
					rr.authindex++
					if rr.authindex < len(rr.auths) {
						rr.cauth = rr.auths[rr.authindex]
						count = -1
						goto cb_retry
					}
					e := utils.MakeError(400, "Failed to get auths")
					e.AddError(fmt.Errorf("out: %s", out))
					e.AddError(jterr)
					return "", e
				}
				out += jout
			}
			out += fmt.Sprintf("token: %s\n", rr.cauth.bearerCache)
			req.Header.Set("Authentication", fmt.Sprintf("Bearer %s", rr.cauth.bearerCache))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rr.cauth.bearerCache))
		case "exec":
			// Exec program to get a string that is a bearer token.
			if rr.cauth.bearerCache == "" ||
				time.Now().Sub(rr.cauth.bearerCacheLookupTime) > time.Duration(rr.cauth.bearerCacheTimeout)*time.Second {
				jout, jterr := getExecBearer(rr.l, rr.cauth)
				if jterr != nil {
					rr.cauth.bearerCache = ""
					out += fmt.Sprintf("Auth[%d] failed: %v\n", rr.authindex, jterr)
					rr.authindex++
					if rr.authindex < len(rr.auths) {
						rr.cauth = rr.auths[rr.authindex]
						count = -1
						goto cb_retry
					}
					e := utils.MakeError(400, "Failed to get auths")
					e.AddError(fmt.Errorf("out: %s", out))
					e.AddError(jterr)
					return "", e
				}
				out += jout
			}
			out += fmt.Sprintf("token: %s\n", rr.cauth.bearerCache)
			req.Header.Set("Authentication", fmt.Sprintf("Bearer %s", rr.cauth.bearerCache))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rr.cauth.bearerCache))
		}
	}

	hasContentType := false
	hasAccept := false
	if rr.cb.Headers != nil {
		for k, v := range rr.cb.Headers {
			if strings.ToLower(k) == strings.ToLower("Content-Type") {
				hasContentType = true
			}
			if strings.ToLower(k) == strings.ToLower("Accept") {
				hasAccept = true
			}
			req.Header.Set(k, v)
		}
	}
	if !hasContentType {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if !hasAccept {
		req.Header.Set("Accept", "application/json")
	}

	resp2, rerr := rr.p.callbackClient.Do(req)
	if rerr != nil {
		if count < rr.cb.Retry {
			goto cb_retry
		}
		e := utils.MakeError(400, "Failed to callback API")
		e.AddError(fmt.Errorf("out: %s", out))
		e.AddError(rerr)
		return string(rr.payload), e
	}
	defer resp2.Body.Close()

	out += fmt.Sprintf("request: %s\n", string(rr.payload))
	out += fmt.Sprintf("request Headers: %v\n", req.Header)
	out += fmt.Sprintf("response Status: %v\n", resp2.Status)
	out += fmt.Sprintf("response Headers: %v\n", resp2.Header)
	body, _ := ioutil.ReadAll(resp2.Body)
	out += fmt.Sprintf("response Body: %s\n", string(body))

	if resp2.StatusCode >= 400 {
		if count < rr.cb.Retry {
			goto cb_retry
		}
		if resp2.StatusCode == 401 || resp2.StatusCode == 403 {
			// If more auths, try them.
			rr.authindex++
			if rr.authindex < len(rr.auths) {
				rr.cauth = rr.auths[rr.authindex]
				count = -1
				goto cb_retry
			}
		}
		err = utils.MakeError(resp2.StatusCode, fmt.Sprintf("Callback API returned %d", resp2.StatusCode))
		err.AddError(fmt.Errorf("out: %s", out))
	} else {
		if rr.cb.JsonResponse {
			if jerr := json.Unmarshal(body, &answer); jerr != nil {
				err = utils.MakeError(400, "Callback JSON parse")
				err.AddError(fmt.Errorf("body: %s", string(body)))
			}
		} else if rr.cb.StringResponse {
			answer = string(body)
		} else {
			answer = body
		}
	}
	return
}

func (p *runningConfig) postTrigger(l logger.Logger,
	machine *models.Machine,
	overrideData interface{},
	action string,
	cb *callback) (rreq *remoteReq, err *models.Error) {
	rreq = &remoteReq{p: p}
	rreq.l = l
	rreq.auths = []*auth{}
	rreq.action = action
	rreq.cb = cb
	if a, ok := p.auths[cb.Auth]; ok {
		rreq.auths = append(rreq.auths, a)
	}
	if cb.Auths != nil {
		for _, aname := range cb.Auths {
			if a, ok := p.auths[aname]; ok {
				rreq.auths = append(rreq.auths, a)
			}
		}
	}
	if len(rreq.auths) > 0 {
		rreq.cauth = rreq.auths[rreq.authindex]
	}
	var res map[string]interface{}
	rr := p.drpClient.Req().UrlFor("machines", machine.UUID(), "params")
	if cb.Aggregate || overrideData != nil {
		rr = rr.Params("aggregate", "true")
	}
	if cb.Decode || overrideData != nil {
		rr = rr.Params("decode", "true")
	}
	if derr := rr.Do(&res); derr != nil {
		err = utils.ConvertError(400, derr)
		return
	}
	machine.Params = res

	info, ierr := p.drpClient.Info()
	if ierr != nil {
		err = utils.ConvertError(400, ierr)
		return
	}

	if overrideData == nil {
		for _, dk := range cb.ExcludeParams {
			delete(machine.Params, dk)
		}
		overrideData = machine
	} else {
		s, ok := overrideData.(string)
		if ok {
			var rerr error
			overrideData, rerr = Render(l, p.auths, p.drpClient, s, machine, info)
			if rerr != nil {
				err = utils.ConvertError(400, rerr)
				return
			}
		}
	}

	buf2, jerr := json.Marshal(overrideData)
	if jerr != nil {
		err = utils.ConvertError(400, jerr)
		return
	}
	rreq.payload = buf2

	localUrl, uerr := Render(l, p.auths, p.drpClient, cb.Url, machine, info)
	if uerr != nil {
		err = utils.ConvertError(400, uerr)
		return
	}
	rreq.localUrl = localUrl
	return
}

// Action handles the action call from the DRP Endpoint
// using ma.Command, all registered actions should be handled
// reminder when validating params:
//   DRP will pass in required machine params if they exist in hierarchy
func (p *Plugin) Action(l logger.Logger, ma *models.Action) (interface{}, *models.Error) {
	p.rcMux.Lock()
	rc := p.rc
	p.rcMux.Unlock()

	switch ma.Command {
	case "callbackDo":
		// validate action and do
		var action string
		action, err := utils.ValidateStringValue("callback/action", ma.Params["callback/action"])
		if err != nil {
			return nil, err
		}
		overrideData, _ := ma.Params["callback/data-override"]

		machine := &models.Machine{}
		machine.Fill()
		if rerr := models.Remarshal(ma.Model, &machine); rerr != nil {
			return nil, utils.ConvertError(400, rerr)
		}
		cb, ok := rc.callbacks[action]
		if !ok {
			return fmt.Sprintf("Callback attempted, but skipped because action unknown: %s", action), nil
		}
		var rreq *remoteReq
		if rreq, err = rc.postTrigger(l, machine, overrideData, action, cb); err != nil {
			return nil, err
		} else {
			return rreq.do()
		}
	default:
		return nil, utils.MakeError(404, fmt.Sprintf("Unknown command: %s", ma.Command))
	}
}

func (p *Plugin) SelectEvents() []string {
	return []string{
		"machines.update.*",
		"jobs.update.*",
		"jobs.save.*",
	}
}

// Event handler - need to deal with this...
func (p *Plugin) Publish(l logger.Logger, e *models.Event) (err *models.Error) {
	p.rcMux.Lock()
	rc := p.rc
	p.rcMux.Unlock()
	obj, merr := e.Model()
	if merr != nil {
		// Bad object
		return
	}
	var rreq *remoteReq
	tc, tok := rc.callbacks["taskcomplete"]
	jc, jok := rc.callbacks["jobfail"]
	if e.Type == "machines" && tok {
		machine := obj.(*models.Machine)
		if machine.CurrentTask != len(machine.Tasks) {
			return
		}
		rreq, err = rc.postTrigger(l, machine, nil, "taskcomplete", tc)
		if err == nil {
			p.pmq.Add(machine.UUID(), rreq.l, func() {
				data, err := rreq.do()
				if err != nil {
					rreq.l.Errorf("Task complete callback failed: %v\nData: %v\n", err, data)
				}
			})
		} else {
			l.Errorf("Task complete callback prep failed: %v", err)
		}
		return
	}
	if e.Type == "jobs" && jok {
		job := obj.(*models.Job)
		var ojob *models.Job
		if e.Original != nil {
			res, rerr := models.New(e.Type)
			if rerr != nil {
				err = utils.ConvertError(400, rerr)
				return
			}
			if eerr := models.Remarshal(e.Original, &res); eerr != nil {
				err = utils.ConvertError(400, eerr)
				return
			}
			ojob = res.(*models.Job)
		}
		machine := &models.Machine{}
		if perr := rc.drpClient.FillModel(machine, job.Machine.String()); perr != nil {
			err = utils.ConvertError(400, perr)
			return
		}

		if !(job.State == "failed" && (ojob == nil || ojob.State == "running" || ojob.State == "created")) {
			return
		}
		rreq, err = rc.postTrigger(l, machine, nil, "jobfail", jc)
		if err == nil {
			p.pmq.Add(machine.UUID(), rreq.l, func() {
				data, err := rreq.do()
				if err != nil {
					rreq.l.Errorf("Jobfail callback failed: %v\nData: %v\n", err, data)
				}
			})
		} else {
			l.Errorf("Jobfail callback prep failed: %v", err)
		}
	}
	return
}

// main is the entry point for the plugin code
// the InitApp routine should reflect the name and purpose of the plugin
func main() {
	plugin.InitApp("callback",
		"Provides way to callback to other systems",
		version,
		&def,
		&Plugin{
			rcMux: &sync.Mutex{},
			pmq:   utils.NewQueues(context.Background(), 100),
		})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
