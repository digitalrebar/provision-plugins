#!/bin/bash
# Create an ESXi BootEnv - typically used for Vendor ISOs

###
#  This is a limited use tool for creating a DRP BootEnv, based on
#  an ESXi install ISO.  It is limited in that it only creates a
#  minimal content pack for a single BootEnv - this bootenv content
#  pack still requires the "vmware" plugin or content pack be installed
#  on the system.
#
#   INPUT: * ISO image path
#
#  OUTPUT: * a completed mini content pack that can be added to a
#            running DRP Endpoint
#          * relies on the base 'vmware` content pack being installed
#            for templates and other supporting content
#          * the output DRP content is compliant with v3.13.0 Endpoint
#            and Portal UX v2.0.0 style meta info files
#
#    TODO: * input options to specify the output directory and/or
#            Content YAML file
#          * add option to verride the TITLE to a user settable value
#          * input option to upload to DRP "files" location
#          * input option to upload to a running DRP Endpoint
#          * better cleanup handling
#          * customization tweaks for Meta file settings
###

function got_trap() {
  echo "No auto-unmounting or cleanup performed ..."
  echo "Check if ISO image is mounted ('$ISO_MNT')"
  echo "May need to nuke BLD dir ('$BLD')"
  echo ""
  echo "UNMOUNT:  $UNMOUNT"
  echo " REMOVE:  rm -rf $BLD.*"
  GOT_TRAP="yes"
}

function cleanup(){
  [[ -n "$GOT_TRAP" ]] && exit
  echo "DRP Contents created in:  $BLD"
  if mount | grep -q "$ISO_MNT"
  then
    [[ -n "$UNMOUNT" ]] && echo "Unmounting ISO '$ISO'"
    [[ -n "$UNMOUNT" ]] && $UNMOUNT && rmdir $ISO_MNT
  fi
}

trap cleanup EXIT
trap got_trap ERR SIGINT SIGTERM SIGQUIT

function xiterr() { [[ $1 =~ ^[0-9]+$ ]] && { XIT=$1; shift; } || XIT=1; printf "FATAL: $*\n"; exit $XIT; }

# thank you macOS for making this difficult with upper case names
# in ISO images, and now mount option to change case
function check_files(){
  local _upper_file
  P=$(pwd)
  for _full in $*
  do
    local _file=$(basename $_full)
    local _path=$(dirname $_full)
    local _good="no"
    cd $_path
    _upper_file=$(echo "$_file" | tr '[:lower:]' '[:upper:]')
    if [[ -r $_file ]]
    then
      _good="yes"
    else
      [[ -r $_upper_file ]] && _good="yes"
    fi
    [[ "$_good" == "no" ]] && xiterr 1 "unable to read file '$_file'"
  done
  cd $P
}

set -e
#set -x

# sigh ... macOS, thank you
case $(uname -s) in
  Darwin) ( which shasum ) && SHA="shasum -a 256" ;;
  Linux) ( which sha256sum) && SHA="sha256sum" ;;
  *) xiterr 1 "Unsupported system type '$(uname -s)'" ;;
esac

# this may get set by a DRP Param in the future
# we may provide an HTTP path to the ISO and need to get it first
ISO="$1"
echo "ISO input file set to '$ISO'"
[[ -z "$ISO" ]] && xiterr 1 "expect an ISO image path location for ARGv1"
[[ ! -r "$ISO" ]] && xiterr 1 "unable to read specified ISO image ('$ISO')"

BLD=$(mktemp -d /tmp/esxi-bootenv-$$.XXXXXXXXXXXX)
ISO_MNT=${BLD}.iso
ISO_PATH=$(dirname $ISO)
ISO_FILE=$(basename $ISO)
ISO_NAME=$(echo $ISO_FILE | sed 's/\.iso$//g')
ISO_SHA=$($SHA $ISO | awk ' { print $1 } ')
# thank you macOS, yet again
[[ -z "$ISO_SHA" ]] && xiterr 1 "SHA sum not created successfully (using '$SHA')"

# mount our iso
mkdir -p $ISO_MNT
# sigh ... macOS, thank you
case $(uname -s) in
  Darwin) hdiutil attach $ISO -readonly -mountpoint $ISO_MNT
          TITLE=$(hdiutil imageinfo $ISO 2> /dev/null | grep "partition-name: " | sed 's/^.*partition-name: //g' | tr '[:upper:]' '[:lower:]' || true)
          [[ -z "$TITLE" ]] && TITLE=$ISO_NAME
          UNMOUNT="hdiutil unmount $ISO_MNT"
    ;;
  Linux) mount -o ro,loop $ISO $ISO_MNT
         UNMOUNT="umount $ISO_MOUNT"
         if which isoinfo > /dev/null 2>&1
         then
           TITLE=$(isoinfo -d -i $ISO  | grep "^Volume id: " | awk ' { print $NF } ' | tr '[:upper:]' '[:lower:]')
         else
           TITLE=$(file -b $ISO | cut -d "'" -f 2 || true)
         fi
         [[ -z "$TITLE" ]] && TITLE=$ISO_NAME
    ;;
  *) xiterr 1 "Unsupported system type '$(uname -s)'" ;;
esac

# I regret relying on vendors to add appropriate metadata in their
# ISO build pipeline...

# some vendors seem to wontonly inject spaces in to their
# ISO meta data fields ... thanks guys
TITLE=$(echo $TITLE | tr -d '[:space:]')

# CISCO FIXUPS
# fixup title because Cisco doesn't know how to burn an ISO
# with decent metadata information
if echo "$ISO" | grep -qi cisco
then
  TITLE=$(echo $TITLE | sed 's/custo$/custom/')
  { echo "$TITLE" | grep -qi "cisco"; } || TITLE="$TITLE"
  echo "Changing TITLE because ... CISCO ... to: $TITLE"
fi
# since they alco can't spell, fix mispellings in ISO metadata

# HITACHI FIXUPS
# add specific HItachi product line info and "hitachi" since they
# don't specity that in the metadata
if echo "$ISO" | grep -qi hitachi
then
  HIT=$(echo "$ISO" | sed 's/^.*hitachi\(.*\).iso/hitachi\1/g')
  TITLE=$(echo $TITLE | sed 's/cust$/custom/')
  { echo "$TITLE" | grep -qi "hitachi"; } || TITLE="$TITLE-$HIT"
  echo "Changing TITLE because ... HITACHI ... to: $TITLE"
fi

# generic fixups
TITLE=$(echo $TITLE | sed 's/^[-_~\.*\*]*\(.*\)[-_~\.*\*]/\1/')
[[ "$TITLE" =~ ^esxi-.* ]] || TITLE="esxi-$TITLE"

BUNDLE="$BLD/vmware-$TITLE.yaml"

echo ""
echo "Setting DRP Content build directory to:  $BLD"
echo "FINAL DRP Content TITLE set to:  $TITLE"
echo ""

###
#  Build our content meta info files
###
echo "Building meta data files for content bundle ... "
printf "RackN, Inc." > ${BLD}/._Author.meta
printf "https://github.com/rackn/provision-content" > ${BLD}/._CodeSource.meta
printf "black" > ${BLD}/._Color.meta
printf "RackN" > ${BLD}/._Copyright.meta
printf "VMware ESXi BootEnv for $TITLE" > ${BLD}/._Description.meta
printf "BootEnv for $TITLE" > ${BLD}/._DisplayName.meta
printf "No documentation provided.\n" > ${BLD}/._Documentation.meta
printf "https://provision.readthedocs.io/en/tip/README.html" > ${BLD}/._DocUrl.meta
printf "book" > ${BLD}/._Icon.meta
printf "RackN" > ${BLD}/._License.meta
printf "vmware-$TITLE" > ${BLD}/._Name.meta
printf "1000" > ${BLD}/._Order.meta
printf "sane-exit-codes" > ${BLD}/._RequiredFeatures.meta
printf "RackN" > ${BLD}/._Source.meta
printf "enterprise,rackn,esx,vmware" > ${BLD}/._Tags.meta

echo "Successfully built our meta info files in $BLD"

###
#  make our content directories
###
DIRS="bootenvs templates stages workflows"
for D in $DIRS; do mkdir -p $BLD/$D; done
echo "Made DRP contents directories ('$DIRS')"

###
#  make our bootenv template for the custom ISO
###
check_files $ISO_MNT/mboot.c32 $ISO_MNT/boot.cfg
T_YAML="$BLD/bootenvs/$TITLE.yaml"
cat <<BENV > $T_YAML
---
Name: $TITLE-install
Description: Install BootEnv for $TITLE
Documentation: ""
Meta:
  color: yellow
  icon: zip
  title: RackN Content
OS:
  Name: $TITLE
  Family: vmware
  Codename: esxi
  Version: custom
  IsoFile: $ISO_FILE
  IsoSha256: $ISO_SHA
OptionalParams:
  - provisioner-default-password-hash
Kernel: ../../chain.c32
BootParams: -c {{.Machine.Path}}/boot.cfg
Initrds: []
Loaders:
  amd64-uefi: efi/boot/bootx64.efi
Templates:
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux
    Path: pxelinux.cfg/{{.Machine.HexAddress}}
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux-mac
    Path: pxelinux.cfg/{{.Machine.MacAddr "pxelinux"}}
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe
    Path: '{{.Machine.Address}}.ipxe'
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe-mac
    Path: '{{.Machine.MacAddr "ipxe"}}.ipxe'
  - ID: esxi-install-py3.ks.tmpl
    Name: compute.ks
    Path: '{{.Machine.Path}}/compute.ks'
  - ID: ${TITLE}.boot.cfg.tmpl
    Name: boot.cfg
    Path: '{{.Machine.Path}}/boot.cfg'
  - ID: ${TITLE}.boot.cfg.tmpl
    Name: boot-uefi.cfg
    Path: '{{.Env.PathForArch "tftp" "" "amd64"}}/efi/boot/{{.Machine.MacAddr "pxelinux"}}/boot.cfg'
BENV

echo "Built DRP BootEnv template file '$T_YAML'"

###
#  build the boot.cfg template
###
KERNEL=$(grep "^kernel=" $ISO_MNT/boot.cfg | cut -d "=" -f2 | sed 's:/::g')
MODULES=$(grep "^modules=" $ISO_MNT/boot.cfg| sed -e 's:^modules=::' -e 's:/::g')
MODULES=$(echo "$MODULES" | sed 's: --- tools.t00:{{ if eq (.Param \"esxi/skip-tools\") false -}} --- tools.t00{{end}}:')

check_files $ISO_MNT/$KERNEL
B_TMPL="$BLD/templates/$TITLE.boot.cfg.tmpl"
cat <<BOOT > $B_TMPL
bootstate=0
title=Loading ESXi installer for $TITLE
timeout=2
prefix=/{{ trimSuffix "/" (.Env.PathFor "tftp" "/") }}
kernel=$KERNEL
kernelopt=ks={{.Machine.Url}}/compute.ks{{if .ParamExists "kernel-options"}} {{.Param "kernel-options"}}{{end}}{{if .ParamExists "esxi/serial-console"}} {{.Param "esxi/serial-console"}}{{end}}
build=
updated=0
{{ if eq (.Param "esxi/set-norts") true }}norts=1{{ end -}}
{{ if .ParamExists "esxi/boot-cfg-extra-options" }}{{ .Param "esxi/boot-cfg-extra-options" }}{{ end -}}
modules=$MODULES
BOOT

echo "Built ESXi boot.cfg template file '$B_TMPL'"

###
#  build the stage for our workflow
###
S_YAML="$BLD/stages/$TITLE.yaml"
cat <<STAGE > $S_YAML
---
Name: $TITLE-install
BootEnv: $TITLE-install
Description: "Stage for custom ESXi $TITLE"
ReadOnly: true
Reboot: false
Meta:
  color: yellow
  icon: download
  title: RackN Content
OptionalParams: []
Profiles: []
RequiredParams: []
Tasks: []
Templates: []
STAGE

echo "Built stage yaml file '$S_YAML'"

###
#  build the workflow for our installer
###
W_YAML="$BLD/workflows/$TITLE-install.yaml"
cat <<WF > $W_YAML
---
Name: $TITLE-install
Description: "Install custom ESXi $TITLE"
Documentation: ""
Meta:
  color: yellow
  icon: shuffle
  title: RackN Content
ReadOnly: true
Stages:
  - $TITLE-install
  - finish-install
  - complete
WF

echo "Built workflow yaml file '$W_YAML'"

cd $BLD
DRPCLI=$(which drpcli) || true
if [[ -n "$DRPCLI" ]]
then
  echo "Running 'drpcli' bundle operation ... "
  drpcli contents bundle $BUNDLE
else
  echo "No 'drpcli' binary found in PATH ('$PATH')"
  echo "Not running 'drpcli contents bundle...' operation."
  echo ""
  echo "EXAMPLE BUNDLE:  drpcli contents bundle $BUNDLE"
fi
echo "EXAMPLE UPLOAD:  drpcli contents upload $BUNDLE"

