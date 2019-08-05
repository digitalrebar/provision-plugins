package main

import (
	"fmt"
	"io/ioutil"

	"github.com/digitalocean/go-netbox/netbox/client/dcim"
	nbm "github.com/digitalocean/go-netbox/netbox/models"
	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/go-openapi/runtime"
	"github.com/digitalrebar/provision-plugins/v4/utils"
)

func handleNetBoxError(err error) string {
	if nr, ok := err.(*runtime.APIError); ok {
		var jj interface{}
		if r, ok := nr.Response.(runtime.ClientResponse); ok {
			if b, e := ioutil.ReadAll(r.Body()); e != nil {
				jj = fmt.Sprintf("M: %s, %s", r.Message(), e.Error())
			} else {
				jj = fmt.Sprintf("M: %s, %s", r.Message(), string(b))
			}
		} else {
			jj = nr
		}

		return fmt.Sprintf("Op: %s, Err: %v", nr.OperationName, jj)

	} else {
		return err.Error()
	}
}

func (p *Plugin) ImportNetboxDevices(l logger.Logger) *models.Error {

	if data, err := p.drpClient.ListModel("machines"); err != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to get drp machines: %s", err.Error()))
	} else {
		for _, d := range data {
			m := d.(*models.Machine)
			if e := p.CreateOrUpdateNetboxDevice(l, m); e != nil {
				l.Errorf(e.Error())
			}
		}
	}

	return nil
}

func (p *Plugin) RemoveNetboxDevice(l logger.Logger, m *models.Machine) *models.Error {
	id := utils.GetParamOrInt64(m.Params, "netbox/id", -1)
	if id == -1 {
		return nil
	}
	params := dcim.NewDcimDevicesDeleteParams().WithID(id)
	if _, err := p.netboxClient.Dcim.DcimDevicesDelete(params, nil); err != nil {
		return utils.MakeError(400, fmt.Sprintf("Failed to delete %d: %v", id,
			handleNetBoxError(err)))

	}
	return nil
}

func (p *Plugin) CreateOrUpdateNetboxDevice(l logger.Logger, m *models.Machine) *models.Error {
	device := &nbm.WritableDevice{}
	id := utils.GetParamOrInt64(m.Params, "netbox/id", -1)
	if id != -1 {
		// Get it to update
		ids := fmt.Sprintf("%d", id)
		filter := dcim.NewDcimDevicesListParams().WithIDIn(&ids)
		answer, err := p.netboxClient.Dcim.DcimDevicesList(filter, nil)
		if err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to lookup: %d: %v", id,
				handleNetBoxError(err)))
		}
		if *answer.Payload.Count == 1 {
			device = convertDeviceToWritableDevice(answer.Payload.Results[0])
		} else {
			id = -1
		}
	}

	updateNetboxDeviceFromDrpMachine(l, device, m)

	if id == -1 {
		params := dcim.NewDcimDevicesCreateParams().WithData(device)
		if answer, err := p.netboxClient.Dcim.DcimDevicesCreate(params, nil); err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to create: %s: %v", m.UUID(),
				handleNetBoxError(err)))
		} else {
			device = answer.Payload
		}
	} else {
		params := dcim.NewDcimDevicesUpdateParams().WithData(device).WithID(id)
		if answer, err := p.netboxClient.Dcim.DcimDevicesUpdate(params, nil); err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to update: %s: %v", m.UUID(),
				handleNetBoxError(err)))
		} else {
			device = answer.Payload
		}
	}

	if device.ID != id {
		// Update DRP machine entry.
		m.Params["netbox/id"] = device.ID

		if err := p.drpClient.PutModel(m); err != nil {
			return utils.MakeError(400, fmt.Sprintf("Failed to update %s: %v", m.UUID(), err.Error()))
		}
	}

	return nil
}

func updateNetboxDeviceFromDrpMachine(l logger.Logger, d *nbm.WritableDevice, m *models.Machine) {
	d.Name = m.Name

	// Must have DeviceRole set
	if d.DeviceRole == nil {
		id := int64(8)
		d.DeviceRole = &id
	}
	// Must have DeviceType set
	if d.DeviceType == nil {
		id := int64(1)
		d.DeviceType = &id
	}
	// Must have Site set
	if d.Site == nil {
		id := int64(1)
		d.Site = &id
	}
}

func convertDeviceToWritableDevice(d *nbm.Device) *nbm.WritableDevice {
	wb := &nbm.WritableDevice{}

	wb.AssetTag = d.AssetTag
	wb.Comments = d.Comments
	wb.CustomFields = d.CustomFields
	wb.Name = d.Name
	if d.Position != nil {
		wb.Position = *d.Position
	}
	wb.Serial = d.Serial

	if d.Cluster != nil {
		wb.Cluster = d.Cluster.ID
	}

	if d.DeviceRole != nil {
		wb.DeviceRole = &d.DeviceRole.ID
	}

	if d.DeviceType != nil {
		wb.DeviceType = &d.DeviceType.ID
	}

	if d.Face != nil {
		wb.Face = *d.Face.Value
	}

	if d.Platform != nil {
		wb.Platform = d.Platform.ID
	}

	if d.Rack != nil {
		wb.Rack = d.Rack.ID
	}

	if d.Site != nil {
		wb.Site = &d.Site.ID
	}

	if d.Tenant != nil {
		wb.Tenant = d.Tenant.ID
	}

	if d.VcPosition != nil {
		wb.VcPosition = d.VcPosition
	}

	if d.VcPriority != nil {
		wb.VcPriority = d.VcPriority
	}

	if d.VirtualChassis != nil {
		wb.VirtualChassis = d.VirtualChassis.ID
	}

	if d.Status != nil {
		wb.Status = *d.Status.Value
	}

	if d.PrimaryIp4 != nil {
		wb.PrimaryIp4 = d.PrimaryIp4.ID

	}
	if d.PrimaryIp6 != nil {
		wb.PrimaryIp6 = d.PrimaryIp6.ID
	}

	return wb
}
