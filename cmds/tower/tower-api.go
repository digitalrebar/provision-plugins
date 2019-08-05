package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	utils2 "github.com/VictorLowther/jsonpatch2/utils"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision-plugins/v4/utils"
)

func statusNotFinished(status string) bool {
	if status == "successful" || status == "Successful" {
		return false
	}
	if status == "failed" || status == "Failed" {
		return false
	}
	if status == "canceled" || status == "Canceled" {
		return false
	}
	if status == "cancelled" || status == "Cancelled" {
		return false
	}
	if status == "error" || status == "Error" {
		return false
	}
	return true
}

// TOWER API DOC: https://docs.ansible.com/ansible-tower/latest/html/towerapi/
// see github.com/moolitayer/awx-client-go

// lookup inventories with:
// /api/v2/inventories?name=%s

// lookup groups with:
// /api/v2/inventories/#id/groups?name=%s

// lookup job_templates with:
// /api/v2/job_templates?name=%s

// POST /api/v2/hosts/
type TowerHost struct {
	Name        string `json:"name"`        // fqdn of node
	Inventory   int    `json:"inventory"`   // int of inventory
	InstanceId  string `json:"instance_id"` // fqdn of node
	Description string `json:"description"` // fqdn of node
}

type TowerHostReply struct {
	Id int `json:"id"`
}

type TowerLaunchReply struct {
	Id int `json:"id"`
}

type TowerJob struct {
	Status       string `json:"status"`
	ResultStdout string `json:"result_stdout"`
}

// POST /api/v2/groups/#id/hosts/
type TowerHostGroup struct {
	Id int `json:"id"` // int of group
}

// POST /api/v2/job_templates/#id/launch/
type TowerJobTemplate struct {
	Limit     string `json:"limit"`
	ExtraVars string `json:"extra_vars"`
}

type ListReply struct {
	Count   int       `json:"count"`
	Results []IdReply `json:"results"`
}

type IdReply struct {
	Id int `json:"id"`
}

func (p *Plugin) getTowerList(ltype, jname string) (int, *models.Error) {
	retryCount := 0
retryGetTowerList:
	url := fmt.Sprintf("%s/api/v2/%s/?name=%s", p.towerUrl, ltype, jname)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	resp, err := client.Do(req)
	if err != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to query: %s", url))
		theErr.AddError(err)
		return -1, theErr
	}
	defer resp.Body.Close()
	bodyText, err := ioutil.ReadAll(resp.Body)

	// If we get GatewayTimeout, we should retry.
	if resp.StatusCode == 504 && retryCount < 10 {
		time.Sleep(time.Second * time.Duration(retryCount+1))
		retryCount += 1
		goto retryGetTowerList
	}

	if resp.StatusCode != 200 {
		return -1, nil
	}

	tjtr := &ListReply{}
	if merr := json.Unmarshal(bodyText, &tjtr); merr != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to unmarshal: %s", string(bodyText)))
		theErr.AddError(merr)
		return -1, theErr
	}

	if tjtr.Count == 0 {
		return -1, nil
	}

	return tjtr.Results[0].Id, nil
}

func (p *Plugin) getTowerInventoryGroup(id int, group string) (int, *models.Error) {
	retryCount := 0
retryGetTowerInventoryGroup:
	url := fmt.Sprintf("%s/api/v2/inventories/%d/groups?name=%s", p.towerUrl, id, group)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	resp, err := client.Do(req)
	if err != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to query: %s", url))
		theErr.AddError(err)
		return -1, theErr
	}
	defer resp.Body.Close()
	bodyText, err := ioutil.ReadAll(resp.Body)

	// If we get GatewayTimeout, we should retry.
	if resp.StatusCode == 504 && retryCount < 10 {
		time.Sleep(time.Second * time.Duration(retryCount+1))
		retryCount += 1
		goto retryGetTowerInventoryGroup
	}

	if resp.StatusCode != 200 {
		return -1, nil
	}

	tgr := &ListReply{}
	if merr := json.Unmarshal(bodyText, &tgr); merr != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to unmarshal: %s", string(bodyText)))
		theErr.AddError(merr)
		return -1, theErr
	}

	if tgr.Count == 0 {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to find: %s in %d", group, id))
		return -1, theErr
	}

	return tgr.Results[0].Id, nil
}

func (p *Plugin) getTowerJobStatus(jid int) (string, string, *models.Error) {
	retryCount := 0
retryGetTowerJobStatus:
	url := fmt.Sprintf("%s/api/v2/jobs/%d/", p.towerUrl, jid)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	resp, err := client.Do(req)
	if err != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to query: %s", url))
		theErr.AddError(err)
		return "", "", theErr
	}
	defer resp.Body.Close()
	bodyText, err := ioutil.ReadAll(resp.Body)

	// If we get GatewayTimeout, we should retry.
	if resp.StatusCode == 504 && retryCount < 10 {
		time.Sleep(time.Second * time.Duration(retryCount+1))
		retryCount += 1
		goto retryGetTowerJobStatus
	}

	if resp.StatusCode != 200 {
		theErr := utils.MakeError(400, fmt.Sprintf("Status code was not success: %s : %d", url, resp.StatusCode))
		theErr.AddError(err)
		return "", "", theErr
	}

	tj := &TowerJob{}
	if merr := json.Unmarshal(bodyText, &tj); merr != nil {
		theErr := utils.MakeError(400, fmt.Sprintf("Failed to unmarshal TJ: %s", string(bodyText)))
		theErr.AddError(merr)
		return "", "", theErr
	}

	return tj.ResultStdout, tj.Status, nil
}

func (p *Plugin) registerTower(model interface{}, params map[string]interface{}) (string, *models.Error) {
	var group, inventory string
	err := &models.Error{
		Model: "plugin",
		Key:   p.name,
		Type:  "rpc",
	}
	if val, ok := params["tower/group"]; ok {
		group = val.(string)
	} else {
		err.Code = 400
		err.Errorf("tower-register action: missing tower/group")
	}
	if val, ok := params["tower/inventory"]; ok {
		inventory = val.(string)
	} else {
		err.Code = 400
		err.Errorf("tower-register action: missing tower/inventory")
	}

	if err.HasError() != nil {
		return "", err
	}

	out := "Tower Register: \n"

	iid, gerr := p.getTowerList("inventories", inventory)
	if gerr != nil || iid == -1 {
		err.Code = 400
		err.Errorf("tower-register action: missing tower/inventory (%s) in tower", inventory)
		if gerr != nil {
			err.AddError(gerr)
		}
		return out, err
	}

	gid, ierr := p.getTowerInventoryGroup(iid, group)
	if ierr != nil || gid == -1 {
		err.Code = 400
		err.Errorf("tower-register action: missing tower/group (%s) in tower", group)
		if ierr != nil {
			err.AddError(ierr)
		}
		return out, err
	}

	machine := &models.Machine{}
	machine.Fill()
	if err := utils2.Remarshal(model, &machine); err != nil {
		return "", utils.ConvertError(400, err)
	} else {
		out += fmt.Sprintf("machine: %s\n", machine.Uuid)
	}

	hid, herr := p.getTowerList("hosts", machine.Name)
	if herr != nil {
		err.Code = 400
		err.Errorf("Failed to query hosts")
		err.AddError(herr)
		return out, err
	}

	if hid == -1 {
		retryCount := 0
	retry1:
		tempData := &TowerHost{
			Name:       machine.Name,
			Inventory:  iid,
			InstanceId: machine.UUID(),
		}

		buf2, merr := json.Marshal(tempData)
		if merr != nil {
			out += fmt.Sprintf("JSON marshal error: %v\n", merr)
			return "Failed " + out, utils.ConvertError(400, merr)
		}

		url := fmt.Sprintf("%s/api/v2/hosts/", p.towerUrl)
		out += fmt.Sprintf("url: %s\n", url)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(buf2))
		req.SetBasicAuth(p.towerLogin, p.towerPassword)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp2, rerr := client.Do(req)
		if rerr != nil {
			e := utils.MakeError(400, "Failed to POST tower API")
			e.AddError(rerr)
			return string(buf2), e
		}
		defer resp2.Body.Close()

		out += fmt.Sprintf("request: %s\n", string(buf2))
		out += fmt.Sprintf("response Status: %v\n", resp2.Status)
		out += fmt.Sprintf("response Headers: %v\n", resp2.Header)
		body, _ := ioutil.ReadAll(resp2.Body)
		out += fmt.Sprintf("response Body: %s\n", string(body))

		// If we get GatewayTimeout, we should retry.
		if resp2.StatusCode == 504 && retryCount < 10 {
			time.Sleep(time.Second * time.Duration(retryCount+1))
			retryCount += 1
			goto retry1
		}

		if resp2.StatusCode != 201 {
			e := utils.MakeError(400, fmt.Sprintf("Failed to call Host reply in tower API: %d", resp2.StatusCode))
			e.AddError(merr)
			e.Errorf("Failed: %s", out)
			return string(out), e
		}

		hr := &TowerHostReply{}
		if merr := json.Unmarshal(body, &hr); merr != nil {
			e := utils.MakeError(400, "Failed to parse Host reply in tower API")
			e.AddError(merr)
			e.Errorf("Failed: %s", out)
			return "", e
		}
		hid = hr.Id
	}

	tempDataHG := &TowerHostGroup{
		Id: hid,
	}

	out += "Tower Register(Host/Group): \n"

	retryCount := 0
retry2:

	buf2, merr := json.Marshal(tempDataHG)
	if merr != nil {
		out += fmt.Sprintf("JSON marshal error: %v\n", merr)
		return "Failed " + out, utils.ConvertError(400, merr)
	}

	url := fmt.Sprintf("%s/api/v2/groups/%d/hosts/", p.towerUrl, gid)
	out += fmt.Sprintf("url: %s\n", url)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(buf2))
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp3, rerr := client.Do(req)
	if rerr != nil {
		e := utils.MakeError(400, "Failed to POST tower API")
		e.AddError(rerr)
		return string(buf2), e
	}
	defer resp3.Body.Close()

	out += fmt.Sprintf("response Status: %v\n", resp3.Status)
	out += fmt.Sprintf("response Headers: %v\n", resp3.Header)
	body, _ := ioutil.ReadAll(resp3.Body)
	out += fmt.Sprintf("response Body: %s\n", string(body))

	// If we get GatewayTimeout, we should retry.
	if resp3.StatusCode == 504 && retryCount < 10 {
		time.Sleep(time.Second * time.Duration(retryCount+1))
		retryCount += 1
		goto retry2
	}

	if resp3.StatusCode != 204 {
		e := utils.MakeError(400, fmt.Sprintf("Failed to call Host/Group reply in tower API: %d", resp3.StatusCode))
		e.AddError(merr)
		e.Errorf("Failed: %s", out)
		return "", e
	}

	return "Success", nil
}

// https://docs.ansible.com/ansible-tower/latest/html/towerapi/launch_jobtemplate.html
func (p *Plugin) invokeTower(model interface{}, params map[string]interface{}) (string, *models.Error) {
	machine := &models.Machine{}
	machine.Fill()
	if err := utils2.Remarshal(model, &machine); err != nil {
		return "", utils.ConvertError(400, err)
	}
	drpJid := machine.CurrentJob.String()
	utils.AddToJobLog(p.session, drpJid,
		fmt.Sprintf("Tower Invoke Called\nMachine: %s\n", machine.Uuid))

	var jobTemplate, extraVars string
	err := &models.Error{
		Model: "plugin",
		Key:   p.name,
		Type:  "rpc",
	}
	if val, ok := params["tower/job-template"]; ok {
		jobTemplate = val.(string)
	} else {
		err.Code = 400
		err.Errorf("tower-invoke action: missing tower/job-template")
	}
	if val, ok := params["tower/extra-vars"]; ok {
		// This is an object.  Just need to json encode it as a string
		buf2, merr := json.Marshal(val)
		if merr != nil {
			err.Code = 400
			err.Errorf("Failed to Marshal extra-vars into json: %v", val)
			err.AddError(merr)
			return "", err
		}
		extraVars = string(buf2)
	} else {
		err.Code = 400
		err.Errorf("tower-invoke action: missing tower/extra-vars")
	}

	if err.HasError() != nil {
		utils.AddToJobLog(p.session, drpJid,
			fmt.Sprintf("Failed with argument errors: %v\n", err))
		return "", err
	}

	jtid, ierr := p.getTowerList("job_templates", jobTemplate)
	if ierr != nil || jtid == -1 {
		err.Code = 400
		err.Errorf("tower-invoke: job_template not found: %s", jobTemplate)
		if ierr != nil {
			err.AddError(ierr)
		}
		utils.AddToJobLog(p.session, drpJid,
			fmt.Sprintf("Failed to get job_templates: %v\n", err))
		return "", err
	}

	retryCount := 0
retry3:

	tempData := &TowerJobTemplate{
		Limit:     machine.Name,
		ExtraVars: extraVars,
	}

	buf2, merr := json.Marshal(tempData)
	if merr != nil {
		e := utils.MakeError(400, fmt.Sprintf("JSON marshal: %v", tempData))
		e.AddError(merr)
		utils.AddToJobLog(p.session, drpJid,
			fmt.Sprintf("Failed to marshal job_template: %v\n", e))
		return "", e
	}

	url := fmt.Sprintf("%s/api/v2/job_templates/%d/launch/", p.towerUrl, jtid)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(buf2))
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp2, rerr := client.Do(req)
	if rerr != nil {
		e := utils.MakeError(400, "Failed to POST tower job_templates API")
		e.Errorf("Buffer data: %s", string(buf2))
		e.AddError(rerr)
		utils.AddToJobLog(p.session, drpJid,
			fmt.Sprintf("Failed to POST job_template: %v\n", e))
		return "", e
	}
	defer resp2.Body.Close()

	out := "Initial Post Request\n"
	out += fmt.Sprintf("response Status: %v\n", resp2.Status)
	out += fmt.Sprintf("response Headers: %v\n", resp2.Header)
	body, _ := ioutil.ReadAll(resp2.Body)
	out += fmt.Sprintf("response Body: %s\n", string(body))
	utils.AddToJobLog(p.session, drpJid, out)

	// If we get GatewayTimeout, we should retry.
	if resp2.StatusCode == 504 && retryCount < 10 {
		time.Sleep(time.Second * time.Duration(retryCount+1))
		retryCount += 1
		goto retry3
	}

	if resp2.StatusCode != 201 {
		e := utils.MakeError(400, fmt.Sprintf("Failed to create template job: %d", resp2.StatusCode))
		e.AddError(merr)
		utils.AddToJobLog(p.session, drpJid, fmt.Sprintf("Failed to create new job: %v", e))
		return "", e
	}

	tlr := &TowerLaunchReply{}
	if merr := json.Unmarshal(body, &tlr); merr != nil {
		e := utils.MakeError(400, "Failed to parse Tower Launch Reply in tower API")
		e.AddError(merr)
		utils.AddToJobLog(p.session, drpJid, fmt.Sprintf("Failed to process reply job: %v", e))
		return "", e
	}

	if merr := utils.AddDrpParam(p.session, "machines", machine.Key(), "tower/job-id", tlr.Id); merr != nil {
		e := utils.MakeError(400, "Failed to set JOB ID on machine")
		e.AddError(merr)
		utils.AddToJobLog(p.session, drpJid, fmt.Sprintf("Failed to set JOB ID on machine: %v", e))
		return "", e
	}
	return "Success", nil
}

func (p *Plugin) towerJobStatus(model interface{}, params map[string]interface{}) (string, *models.Error) {
	machine := &models.Machine{}
	machine.Fill()
	if err := utils2.Remarshal(model, &machine); err != nil {
		return "", utils.ConvertError(400, err)
	}
	drpJid := machine.CurrentJob.String()

	iv, ok := machine.Params["tower/job-id"]
	if !ok {
		return "NoJob", nil
	}
	towerJobId := int(iv.(float64))

	_, status, err := p.getTowerJobStatus(towerJobId)
	if err != nil {
		e := utils.MakeError(400, fmt.Sprintf("Failed to get Tower Job Status in tower API: %d", towerJobId))
		e.AddError(err)
		utils.AddToJobLog(p.session, drpJid, fmt.Sprintf("Failed to get job status: %v", e))
		return "", e
	}

	// We are done, remove the parameter.
	if !statusNotFinished(status) {
		if merr := utils.RemoveDrpParam(p.session, "machines", machine.Key(), "tower/job-id", nil); merr != nil {
			e := utils.MakeError(400, "Failed to remove JOB ID on machine")
			e.AddError(merr)
			utils.AddToJobLog(p.session, drpJid, fmt.Sprintf("Failed to remove JOB ID on machine: %v", e))
			return "", e
		}
	}

	return status, nil
}

func (p *Plugin) deleteTower(model interface{}, params map[string]interface{}) (string, *models.Error) {
	err := &models.Error{
		Model: "plugin",
		Key:   p.name,
		Type:  "rpc",
	}

	out := "Tower Delete: \n"

	machine := &models.Machine{}
	machine.Fill()
	if err := utils2.Remarshal(model, &machine); err != nil {
		return "", utils.ConvertError(400, err)
	} else {
		out += fmt.Sprintf("machine: %s\n", machine.Uuid)
	}

	hid, herr := p.getTowerList("hosts", machine.Name)
	if herr != nil {
		err.Code = 400
		err.Errorf("Failed to query hosts")
		err.AddError(herr)
		return out, err
	}

	if hid == -1 {
		return "Already deleted", nil
	}

	url := fmt.Sprintf("%s/api/v2/hosts/%d/", p.towerUrl, hid)
	out += fmt.Sprintf("url: %s\n", url)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.SetBasicAuth(p.towerLogin, p.towerPassword)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp3, rerr := client.Do(req)
	if rerr != nil {
		e := utils.MakeError(400, "Failed to DELETE tower API")
		e.Errorf("Failed: %s", out)
		e.AddError(rerr)
		return "", e
	}
	defer resp3.Body.Close()

	out += fmt.Sprintf("response Status: %v\n", resp3.Status)
	out += fmt.Sprintf("response Headers: %v\n", resp3.Header)
	body, _ := ioutil.ReadAll(resp3.Body)
	out += fmt.Sprintf("response Body: %s\n", string(body))

	if resp3.StatusCode != 204 {
		e := utils.MakeError(400, fmt.Sprintf("Failed to delete Host in tower API: %d", resp3.StatusCode))
		e.Errorf("Failed: %s", out)
		return "", e
	}

	return "Success", nil
}
