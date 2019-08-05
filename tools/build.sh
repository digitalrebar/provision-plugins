#!/usr/bin/env bash

set -e

# Work out the GO version we are working with:
GO_VERSION=$(go version | awk '{ print $3 }' | sed 's/go//')
WANTED_VER=(1 12)
if ! [[ "$GO_VERSION" =~ ([0-9]+)\.([0-9]+) ]]; then
    echo "Cannot figure out what version of Go is installed"
    exit 1
elif ! (( ${BASH_REMATCH[1]} > ${WANTED_VER[0]} || ${BASH_REMATCH[2]} >= ${WANTED_VER[1]} )); then
    echo "Go Version needs to be 1.12 or higher: currently $GO_VERSION"
    exit -1
fi

for tool in go-bindata; do
    which "$tool" &>/dev/null && continue
    case $tool in
        go-bindata) go get -u github.com/jteeuwen/go-bindata/...;;
        *) echo "Don't know how to install $tool"; exit 1;;
    esac
done

PATH=$PATH:$GOPATH/bin
for TOOL in drbundler drpcli
do
  if ( which $TOOL > /dev/null 2>&1 )
  then
    echo "'$TOOL' set to $(which $TOOL)"
  else
    curl -L -o $GOPATH/bin/$TOOL https://github.com/digitalrebar/provision/releases/download/tip/$TOOL && chmod +x $GOPATH/bin/$TOOL
  fi
done

. tools/version.sh

echo "Version = $Prepart$MajorV.$MinorV.$PatchV$Extra-$GITHASH"

export CGO_ENABLED=0
export GO111MODULE=on

VERFLAGS="-s -w \
          -X github.com/digitalrebar/provision_plugins.RS_MAJOR_VERSION=$MajorV \
          -X github.com/digitalrebar/provision_plugins.RS_MINOR_VERSION=$MinorV \
          -X github.com/digitalrebar/provision_plugins.RS_PATCH_VERSION=$PatchV \
          -X github.com/digitalrebar/provision_plugins.RS_EXTRA=$Extra \
          -X github.com/digitalrebar/provision_plugins.RS_PREPART=$Prepart \
          -X github.com/digitalrebar/provision_plugins.BuildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` \
          -X github.com/digitalrebar/provision_plugins.GitHash=$GITHASH \
          -X github.com/digitalrebar/provision-plugins.RS_MAJOR_VERSION=$MajorV \
          -X github.com/digitalrebar/provision-plugins.RS_MINOR_VERSION=$MinorV \
          -X github.com/digitalrebar/provision-plugins.RS_PATCH_VERSION=$PatchV \
          -X github.com/digitalrebar/provision-plugins.RS_EXTRA=$Extra \
          -X github.com/digitalrebar/provision-plugins.RS_PREPART=$Prepart \
          -X github.com/digitalrebar/provision-plugins.BuildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` \
          -X github.com/digitalrebar/provision-plugins.GitHash=$GITHASH"


# set our arch:os build pairs to compile for
builds="amd64:linux amd64:darwin arm64:linux arm:7:linux"

# anything on command line will override our pairs listed above
[[ $* ]] && builds="$*"

for cmd in cmds/*; do
    [[ -d $cmd ]] || continue

    if [ -d "cmds/$cmd/content" ] ; then
        echo -n "$Prepart$MajorV.$MinorV.$PatchV$Extra-$GITHASH" > cmds/$cmd/content/._Version.meta
    fi
    gofile="${cmd##*/}"
    (cd "$cmd"; go generate)
    case $gofile in
        kvm-test)targets="amd64:linux arm64:linux arm:7:linux";;
        *)targets="$builds";;
    esac
    for build in ${targets}; do
        os=${build##*:}
        arch=${build%:*}
        export GOARM=""

        if [[ "$arch" =~ ^arm:[567]$ ]]
        then
            ver=${arch##*:}
            arch=${arch%:*}
            export GOARM=$ver
            ver_part=" (v$ver)"
            binpath="bin/$os/${arch}_v${GOARM}"
        else
            ver_part=""
            binpath="bin/$os/$arch"
        fi
        if [[ "$os" == "windows" ]] ; then
            ext=".exe"
        else
            ext=""
        fi
        export GOOS="$os" GOARCH="$arch"
        mkdir -p "$binpath"
        echo "Building binary: ${cmd} for ${arch} ${os}"
        go build -ldflags "$VERFLAGS" -o "$binpath/$gofile${ext}" "$cmd"/*.go
    done
done
echo "To run tests, run: tools/test.sh"
