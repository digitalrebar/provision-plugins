package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > kvm-test.rst"
//go:generate rm content.yaml

import (
	"bytes"
	"fmt"
	"math/big"
	"net"
	"os"
	"sort"
	"time"

	"github.com/VictorLowther/jsonpatch2/utils"
	"github.com/VictorLowther/simplexml/dom"
	"github.com/VictorLowther/simplexml/search"
	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalrebar/logger"
	v4 "github.com/digitalrebar/provision-plugins/v4"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
	"github.com/digitalrebar/provision/v4/plugin"
	"github.com/pborman/uuid"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
)

var (
	version = v4.RSVersion
	def     = models.PluginProvider{
		Name:          "kvm-test",
		Version:       version,
		PluginVersion: 4,

		AvailableActions: []models.AvailableAction{
			models.AvailableAction{
				Command:        "startVM",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "destroyVM",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "poweroff",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "poweron",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "powercycle",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "reboot",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
			models.AvailableAction{
				Command:        "reset",
				Model:          "machines",
				RequiredParams: []string{"kvm-test/machine"},
			},
		},
		Content: contentYamlString,
	}
)

type Subnet struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Nameserver net.IP `json:"nameserver"`
	Gateway    net.IP `json:"gateway"`
	Domain     string `json:"domain"`
}

type Plugin struct {
	session     *api.Client
	Subnet      Subnet `json:"kvm-test/subnet"`
	StoragePool string `json:"kvm-test/storage-pool"`
}

func (p *Plugin) lv() (*libvirt.Libvirt, error) {
	conn, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 1*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Error dialing into local libvirt: %v", err)
	}
	lv := libvirt.New(conn)
	return lv, lv.Connect()
}

func (p *Plugin) createMachine(lv *libvirt.Libvirt,
	l logger.Logger,
	spec *Machine, fws map[string]Firmware) (machine libvirt.Domain, fault *models.Error) {
	fault = &models.Error{
		Code:  500,
		Model: "plugin",
		Key:   "kvm-test",
		Type:  "rpc",
	}
	pool, err := lv.StoragePoolLookupByName(p.StoragePool)
	if err != nil {
		fault.Errorf("Storage pool %s not defined: %v", p.StoragePool, err)
		return
	}
	specXml, err := spec.domainXML()
	if err != nil {
		fault.Errorf("Unable to render spec XML: %v", err)
		return
	}
	volXml, err := spec.volXML()
	if err != nil {
		fault.Errorf("Error rendering volume spec for %#v: %v", spec, err)
		return
	}
	vol, err := lv.StorageVolLookupByName(pool, spec.DiskName())
	if err != nil {
		vol, err = lv.StorageVolCreateXML(pool, volXml, 1)
		if err != nil {
			fault.Errorf("Error creating volume for %#v: %v", spec, err)
			return
		}
	}
	machine, err = lv.DomainDefineXML(specXml)
	if err != nil {
		fault.Errorf("Error creating VM %s: %v", spec.Name, err)
		fault.Errorf("Domain: %s", specXml)
		fault.AddError(lv.StorageVolDelete(vol, 0))
		return
	}
	return machine, nil
}

func (p *Plugin) destroyMachine(lv *libvirt.Libvirt, l logger.Logger, m uuid.UUID) *models.Error {
	err := &models.Error{
		Code:  500,
		Model: "plugin",
		Key:   "kvm-test",
		Type:  "rpc",
	}
	var id libvirt.UUID
	copy(id[:], m)
	machine, merr := lv.DomainLookupByUUID(id)
	if merr != nil {
		err.AddError(merr)
		return err
	}
	mXML, xerr := lv.DomainGetXMLDesc(machine, 2)
	if xerr != nil {
		err.AddError(xerr)
		return err
	}
	buf := bytes.NewBuffer([]byte(mXML))
	machineDoc, docerr := dom.Parse(buf)
	if docerr != nil {
		err.AddError(docerr)
		return err
	}
	state, _, _, _, _, _ := lv.DomainGetInfo(machine)
	if state == 1 {
		err.AddError(lv.DomainDestroy(machine))
	}
	err.AddError(lv.DomainUndefineFlags(machine, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram))
	disks := search.All(
		search.And(
			search.Tag("source", "*"),
			search.Attr("pool", "*", "*"),
			search.Attr("volume", "*", "*")),
		machineDoc.Root().All())
	for _, elem := range disks {
		pa := elem.GetAttr("pool", "*", "*")[0]
		va := elem.GetAttr("volume", "*", "*")[0]
		pool, perr := lv.StoragePoolLookupByName(pa.Value)
		if perr != nil {
			err.AddError(perr)
			continue
		}
		vol, verr := lv.StorageVolLookupByName(pool, va.Value)
		if verr != nil {
			err.AddError(verr)
			continue
		}
		err.AddError(lv.StorageVolDelete(vol, 0))
	}
	return err
}

func (p *Plugin) machineAction(lv *libvirt.Libvirt, l logger.Logger, m uuid.UUID, action string, err *models.Error) {
	var id libvirt.UUID
	copy(id[:], m)
	machine, merr := lv.DomainLookupByUUID(id)
	if merr != nil {
		err.AddError(merr)
		return
	}
	switch action {
	case "poweron":
		err.AddError(lv.DomainCreate(machine))
	case "poweroff":
		err.AddError(lv.DomainDestroy(machine))
	case "powercycle":
		err.AddError(lv.DomainDestroy(machine))
		time.Sleep(2 * time.Second)
		err.AddError(lv.DomainCreate(machine))
	case "shutdown":
		err.AddError(lv.DomainShutdown(machine))
	case "reset":
		err.AddError(lv.DomainReset(machine, 0))
	case "reboot":
		err.AddError(lv.DomainReboot(machine, 0))
	default:
		err.Errorf("Unknown action")
	}
}

func (p *Plugin) SelectEvents() []string {
	return []string{
		"machines.create.*",
		"machines.delete.*",
	}
}

func (p *Plugin) Publish(l logger.Logger, e *models.Event) (res *models.Error) {
	obj, err := e.Model()
	if err != nil {
		return
	}
	m := obj.(*models.Machine)
	if pname, ok := m.Params["machine-plugin"].(string); !ok || pname != "kvm-test" {
		return
	}
	lv, err := p.lv()
	if err != nil {
		res = &models.Error{
			Code:  500,
			Model: "plugin",
			Key:   "kvm-test",
			Type:  "rpc",
		}
		res.AddError(err)
		return
	}
	defer lv.Disconnect()
	switch e.Action {
	case "create":
		res = &models.Error{
			Code:  409,
			Model: "plugin",
			Key:   "kvm-test",
			Type:  "rpc",
		}
		fws := map[string]Firmware{}
		if err := p.session.Req().UrlFor("machines", m.UUID(), "params", "kvm-test/bios").Params("aggregate", "true").Do(&fws); err != nil {
			res.AddError(err)
		}
		l.Infof("New machine %s, wants to be handled by kvm-test", m.Uuid)
		mSpec := &Machine{}
		if v, ok := m.Params["kvm-test/machine"]; ok {
			l.Infof("Populating new machine spec with %#v", v)
			utils.Remarshal(v, &mSpec)
		}
		mSpec.Uuid = m.Uuid
		mSpec.Name = m.Name
		mSpec.Arch = m.Arch
		if mSpec.Mem == 0 {
			mSpec.Mem = 2048
		}
		if mSpec.Cores == 0 {
			mSpec.Cores = 2
		}
		if fw, ok := fws[mSpec.Arch]; ok {
			mSpec.Firmware = fw
		}
		if mSpec.Arch != "amd64" && mSpec.Firmware.Code == "" {
			res.Errorf("Arch %s requires Firmware to be set", mSpec.Arch)
			return
		}
		if mSpec.DiskSize == 0 {
			mSpec.DiskSize = 6
		}
		mSpec.Pool = p.StoragePool
		mSpec.Bridge = p.Subnet.Name
		machine, merr := p.createMachine(lv, l, mSpec, fws)
		if merr != nil {
			res = merr
			return
		}
		mXML, xerr := lv.DomainGetXMLDesc(machine, 2)
		if xerr != nil {
			res.AddError(xerr)
			return
		}
		buf := bytes.NewBuffer([]byte(mXML))
		machineDoc, docerr := dom.Parse(buf)
		if err != nil {
			res.AddError(docerr)
			return
		}
		intfs := search.All(
			search.And(
				search.Tag("mac", "*"),
				search.Attr("address", "*", "*"),
				search.Ancestor(search.Tag("interface", "*"))),
			machineDoc.Root().All())
		macaddrs := []string{}
		for _, elem := range intfs {
			for _, attr := range elem.Attributes {
				if attr.Name.Local == "address" {
					macaddrs = append(macaddrs, attr.Value)
				}
			}
		}
		sort.Strings(macaddrs)
		newM := models.Clone(m).(*models.Machine)
		newM.Params["kvm-test/machine"] = mSpec
		newM.HardwareAddrs = macaddrs
		_, pErr := p.session.PatchTo(m, newM)
		res.AddError(pErr)
	case "delete":
		res = p.destroyMachine(lv, l, m.Uuid)
	}
	if res != nil && !res.ContainsError() {
		res = nil
	}
	return
}

func (p *Plugin) Config(l logger.Logger, session *api.Client, config map[string]interface{}) (res *models.Error) {
	p.session = session
	res = &models.Error{Code: 404,
		Model: "plugin",
		Key:   "kvm-test",
		Type:  "rpc",
	}
	if err := utils.Remarshal(config, p); err != nil {
		res.Errorf("Failed to fill plugin configuration. %v", err)
		return res
	}
	br, err := netlink.LinkByName(p.Subnet.Name)
	if err != nil {
		l.Infof("Creating bridge %s", p.Subnet.Name)
		la := netlink.NewLinkAttrs()
		la.Name = p.Subnet.Name
		br = &netlink.Bridge{LinkAttrs: la}
		if err := netlink.LinkAdd(br); err != nil {
			res.Errorf("Failed to create bridge for subnet: %v", err)
			return
		}
	}
	netlink.LinkSetUp(br)
	wantedIP, wantedNet, err := net.ParseCIDR(p.Subnet.Address)
	if wantedIP.To4() == nil {
		res.Errorf("Subnet must be for IPv4, not %s", p.Subnet.Address)
	}
	if err != nil {
		res.Errorf("Invalid subnet CIDR form address %s: %v", p.Subnet.Address, err)
		return
	}
	if !wantedIP.IsGlobalUnicast() {
		res.Errorf("Invalid subnet %s", p.Subnet.Address)
		return
	}
	wantedAddr := netlink.Addr{
		IPNet: &net.IPNet{
			IP:   wantedIP.To4(),
			Mask: net.IPMask(net.IP(wantedNet.Mask).To4()),
		},
	}
	// We have a bridge, make sure it has the right IP address
	addrs, err := netlink.AddrList(br, nl.FAMILY_ALL)
	if err != nil {
		res.Errorf("Error listing addresses on interface %s: %v", p.Subnet.Name, err)
		return
	}
	found := false
	for i := range addrs {
		if addrs[i].Equal(wantedAddr) {
			found = true
			break
		}
	}
	if !found {
		if err := netlink.AddrAdd(br, &wantedAddr); err != nil {
			res.Errorf("Error adding %s to %s: %v", p.Subnet.Address, p.Subnet.Name, err)
			return
		}
	}
	// Calculate start and end addresses
	one := big.NewInt(1)
	subnet := &models.Subnet{}
	subnet.Name = p.Subnet.Name
	subnet.Subnet = p.Subnet.Address
	subnet.Enabled = true
	subnet.Proxy = false
	start, mask := &big.Int{}, &big.Int{}
	mask.SetBytes(wantedAddr.Mask)
	start.SetBytes(wantedAddr.IP)
	start.And(start, mask)
	start.Add(start, one)
	prefixlen, bits := wantedAddr.Mask.Size()
	end := big.NewInt(1)
	end.Lsh(end, uint(bits)-uint(prefixlen))
	end.Sub(end, one)
	end.Or(end, start)
	end.Sub(end, one)
	subnet.ActiveStart = net.IP(start.Bytes()).To4()
	subnet.ActiveEnd = net.IP(end.Bytes()).To4()
	subnet.ActiveLeaseTime = 3600
	subnet.ReservedLeaseTime = 7200000
	subnet.Fill()
	// Calculate DHCP options
	subnet.Options = []models.DhcpOption{
		models.DhcpOption{
			Code:  3,
			Value: p.Subnet.Gateway.String(),
		},
		models.DhcpOption{
			Code:  6,
			Value: p.Subnet.Nameserver.String(),
		},
		models.DhcpOption{
			Code:  15,
			Value: p.Subnet.Domain,
		},
	}
	oldSubnet := &models.Subnet{}
	if err := session.FillModel(oldSubnet, p.Subnet.Name); err == nil {
		if _, err := session.PatchTo(oldSubnet, subnet); err != nil {
			res.Errorf("Unable to update subnet information in DRP for %s: %v", p.Subnet.Name, err)
			return
		}
	} else {
		if err := session.CreateModel(subnet); err != nil {
			res.Errorf("Unable to create subnet in DRP for %s: %v", p.Subnet.Name, err)
			return
		}
	}
	lv, err := p.lv()
	if err != nil {
		res.Errorf("Unable to connect to libvirt: %v", err)
	}
	defer lv.Disconnect()
	if _, err = lv.StoragePoolLookupByName(p.StoragePool); err != nil {
		res.Errorf("Storage pool %s not defined: %v", p.StoragePool, err)
		return
	}
	return nil
}

func (p *Plugin) Action(l logger.Logger, ma *models.Action) (answer interface{}, res *models.Error) {
	res = &models.Error{
		Code: 400,
		Key:  "kvm-test",
		Type: "rpc",
	}
	answer = "Unknown"
	m := &Machine{}
	if err := utils.Remarshal(ma.Params["kvm-test/machine"], &m); err != nil {
		res.Errorf("%s command requires machine spec: %v", ma.Command, err)
		return
	}
	lv, err := p.lv()
	if err != nil {
		res = &models.Error{
			Code:  500,
			Model: "plugin",
			Key:   "kvm-test",
			Type:  "rpc",
		}
		res.AddError(err)
		return
	}
	defer lv.Disconnect()
	switch ma.Command {
	case "destroyVM":
		answer, res = "Destroyed", p.destroyMachine(lv, l, m.Uuid)
	case "poweroff":
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
		answer = "Stopped"
	case "poweron":
		answer = "Started"
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
	case "startVM":
		answer = "Started"
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
	case "powercycle":
		answer = "Powercycled"
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
	case "reboot":
		answer = "Rebooted"
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
	case "reset":
		answer = "Reset"
		p.machineAction(lv, l, m.Uuid, ma.Command, res)
	default:
		res = &models.Error{Code: 404,
			Model:    "plugin",
			Key:      "kvm-test",
			Type:     "rpc",
			Messages: []string{fmt.Sprintf("Unknown command: %s", ma.Command)}}
	}
	if !res.ContainsError() {
		res = nil
	}
	return
}

func main() {
	p := &Plugin{}
	plugin.InitApp("kvm-test", "Provides basic support for creating local KVM vms", version, &def, p)
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
