package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/pborman/uuid"
)

var (
	storTemplate = template.Must(template.New("vol").Parse(`
<volume type='file'>
  <name>{{.DiskName}}</name>
  <allocation unit='KB'>16</allocation>
  <capacity unit='GB'>{{.DiskSize}}</capacity>
  <target>
    <format type='qcow2'/>
    <compat>1.1</compat>
    <features>
      <lazy_refcounts/>
    </features>
  </target>
</volume>
`))
	domTemplates = map[string]*template.Template{
		"amd64": template.Must(template.New("domain").Parse(`
<domain type='kvm'>
  <metadata>
    <kvt:kvm-test xmlns:kvt="http://rackn.com/">kvm-test</kvt:kvm-test>
  </metadata>
  <name>{{.Name}}</name>
  <uuid>{{.Uuid}}</uuid>
  <memory unit='MB'>{{.Mem}}</memory>
  <os>
    <type arch='x86_64' machine='{{.MachineType}}'>hvm</type>
    {{if ne .Firmware.Code "" }}
    <loader readonly="yes" type="{{.Firmware.Format}}">{{.Firmware.Code}}</loader>
    <nvram template="{{.Firmware.Settings}}">/var/lib/libvirt/qemu/{{.Uuid}}_VARS.fd</nvram>
    {{end}}
    <smbios mode='emulate'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <vcpu>{{.Cores}}</vcpu>
  <cpu>
    <topology sockets='1' cores='{{.Cores}}' threads='1'/>
  </cpu>
  <clock offset='utc'>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='hpet' present='no'/>
    <timer name='hypervclock' present='yes'/>
  </clock>
  <devices>
    <disk type='volume' device='disk'>
      <source pool='{{.Pool}}' volume='{{.DiskName}}'/>
      <target dev='sda' bus='sata'/>
      <boot order='2'/>
    </disk>
    <interface type='bridge'>
      <source bridge='{{.Bridge}}'/>
      <model type='{{.NicType}}'/>
      <boot order='1'/>
    </interface>
    <controller type='virtio-serial' index='0'/>
    <channel type='spicevmc'>
      <target type='virtio' name='com.redhat.spice.0'/>
    </channel>
    <input type='tablet' bus='usb'/>
    <input type='mouse' bus='ps2'/>
    <input type='keyboard' bus='ps2'/>
    <graphics type='spice' autoport='yes' listen='127.0.0.1'>
      <listen type='address' address='127.0.0.1'/>
      <image compression='off'/>
    </graphics>
    <video>
      <model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>
    </video>
    <serial type='pty'>
      <target port='0'/>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
    <memballoon model='virtio'/>
    <rng model='virtio'>
      <backend model='random'>/dev/urandom</backend>
    </rng>
  </devices>
</domain>
`)),
		"arm64": template.Must(template.New("domain").Parse(`
<domain type='qemu'>
  <metadata>
    <kvt:kvm-test xmlns:kvt="http://rackn.com/">kvm-test</kvt:kvm-test>
  </metadata>
  <name>{{.Name}}</name>
  <uuid>{{.Uuid}}</uuid>
  <memory unit='MB'>{{.Mem}}</memory>
  <os>
    <type arch='aarch64' machine='{{.MachineType}}'>hvm</type>
    {{if ne .Firmware.Code "" }}
    <loader readonly="yes" type="{{.Firmware.Format}}">{{.Firmware.Code}}</loader>
    <nvram template="{{.Firmware.Settings}}">/var/lib/libvirt/qemu/{{.Uuid}}_VARS.fd</nvram>
    {{end}}
    <smbios mode='emulate'/>
  </os>
  <features>
    <apic/>
    <gic version="2"/>
  </features>
  <vcpu>{{.Cores}}</vcpu>
  <cpu mode='custom' match='exact' check='none'>
    <topology sockets='1' cores='{{.Cores}}' threads='1'/>
    <model fallback='allow'>cortex-a57</model>
  </cpu>
  <clock offset='utc'/>
  <devices>
    <controller type='usb' index='0'/>
    <controller type='scsi' index='0' model='virtio-scsi'/>
    <disk type='volume' device='disk'>
      <source pool='{{.Pool}}' volume='{{.DiskName}}'/>
      <target dev='sda' bus='scsi'/>
      <boot order='2'/>
    </disk>
    <interface type='bridge'>
      <source bridge='{{.Bridge}}'/>
      <model type='virtio'/>
      <boot order='1'/>
    </interface>
    <input type='keyboard' bus='usb'/>
    <input type='mouse' bus='usb'/>
    <input type='tablet' bus='usb'/>
    <graphics type='vnc'/>
    <video>
      <model type='virtio' heads='1' primary='yes'/>
    </video>
    <serial type='pty'>
      <target port='0'/>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
    <memballoon model='virtio'/>
    <rng model='virtio'>
      <backend model='random'>/dev/urandom</backend>
    </rng>
  </devices>
</domain>
`)),
	}
)

type Firmware struct {
	Code     string `json:"code"`
	Settings string `json:"settings"`
	Format   string `json:"format"`
}

type Machine struct {
	Mem      uint64    `json:"memory"`
	DiskSize uint      `json:"disk-size"`
	Cores    uint      `json:"cores"`
	Uuid     uuid.UUID `json:"uuid"`
	Name     string    `json:"name"`
	Arch     string    `json:"arch"`
	Pool     string    `json:"pool"`
	Bridge   string    `json:"bridge"`
	Firmware Firmware  `json:"firmware"`
}

func (m *Machine) NicType() string {
	switch m.Arch {
	case "amd64", "arm64":
		return "e1000"
	default:
		return ""
	}
}

func (m *Machine) MachineType() string {
	switch m.Arch {
	case "amd64":
		return "q35"
	case "arm64":
		return "virt"
	default:
		return ""
	}
}

func (m *Machine) DiskName() string {
	return fmt.Sprintf("%s.qcow2", m.Uuid)
}

func (m *Machine) domainXML() (string, error) {
	buf := &bytes.Buffer{}
	err := domTemplates[m.Arch].Execute(buf, m)
	return buf.String(), err
}

func (m *Machine) volXML() (string, error) {
	buf := &bytes.Buffer{}
	err := storTemplate.Execute(buf, m)
	return buf.String(), err
}
