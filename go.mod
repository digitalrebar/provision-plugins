module github.com/rackn/provision-plugins/v4

go 1.12

require (
	github.com/VictorLowther/jsonpatch2 v1.0.0
	github.com/VictorLowther/simplexml v0.0.0-20180716164440-0bff93621230
	github.com/cloudflare/cfssl v0.0.0-20181102015659-ea4033a214e7
	github.com/digitalocean/go-libvirt v0.0.0-20181105201604-08f982c676c6
	github.com/digitalocean/go-netbox v0.0.0-20180319151450-29433ec527e7
	github.com/digitalrebar/go-ad-auth v0.0.0-20190319155217-33ce42325bc6
	github.com/digitalrebar/logger v0.3.0
	github.com/digitalrebar/provision/v4 v4.0.0-pre4
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/facebookgo/ensure v0.0.0-20160127193407-b4ab57deab51 // indirect
	github.com/facebookgo/limitgroup v0.0.0-20150612190941-6abd8d71ec01 // indirect
	github.com/facebookgo/muster v0.0.0-20150708232844-fd3d7953fd52 // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20150612182917-8dac2c3c4870 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/analysis v0.19.0 // indirect
	github.com/go-openapi/errors v0.19.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.0 // indirect
	github.com/go-openapi/jsonreference v0.19.0 // indirect
	github.com/go-openapi/loads v0.19.0 // indirect
	github.com/go-openapi/runtime v0.18.0
	github.com/go-openapi/spec v0.18.0 // indirect
	github.com/go-openapi/strfmt v0.18.0 // indirect
	github.com/go-openapi/swag v0.18.0 // indirect
	github.com/go-openapi/validate v0.18.0 // indirect
	github.com/gofunky/semver v3.5.2+incompatible
	github.com/gogo/protobuf v0.0.0-20171007142547-342cbe0a0415 // indirect
	github.com/golang/glog v0.0.0-20141105023935-44145f04b68c // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/btree v0.0.0-20160524151835-7d79101e329e // indirect
	github.com/google/certificate-transparency-go v0.0.0-20181206160638-61650fd8d5be // indirect
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367 // indirect
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d // indirect
	github.com/gorilla/websocket v1.4.0
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7 // indirect
	github.com/honeycombio/libhoney-go v0.0.0-20181205211707-18ceb643e3f3
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869
	github.com/json-iterator/go v0.0.0-20180701071628-ab8a2e0c74be // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/packethost/packngo v0.0.0-20181206143517-b36133050ae5
	github.com/pborman/uuid v1.2.0
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/spf13/cobra v0.0.4-0.20180722215644-7c4570c3ebeb
	github.com/vishvananda/netlink v0.0.0-20181208180451-78a3099b7080
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/oauth2 v0.0.0-20170412232759-a6bd8cefa181 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d // indirect
	google.golang.org/appengine v0.0.0-20181203163710-a37df1387b45 // indirect
	gopkg.in/alexcesaro/statsd.v2 v2.0.0 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/imjoey/go-ovirt.v4 v4.0.6
	gopkg.in/inf.v0 v0.9.0 // indirect
	gopkg.in/korylprince/go-ad-auth.v2 v2.2.0 // indirect
	gopkg.in/ldap.v2 v2.5.1 // indirect
	gopkg.in/ldap.v3 v3.0.3 // indirect
	k8s.io/api v0.0.0-20181004124137-fd83cbc87e76
	k8s.io/apimachinery v0.0.0-20181130031027-a5db2f3d07a9
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/klog v0.0.0-20181108234604-8139d8cb77af // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace github.com/digitalrebar/provision/v4 => github.com/rackn/provision/v4 v4.0.0-pre4
