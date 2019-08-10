#!/usr/bin/env bash
set -e

# Work out the GO version we are working with:
GO_VERSION=$(go version | awk '{ print $3 }' | sed 's/go//')
WANTED_VER=(1 12)
if ! [[ "$GO_VERSION" =~ ([0-9]+)\.([0-9]+) ]]; then
    echo "Cannot figure out what version of Go is installed"
    exit 1
elif ! (( ${BASH_REMATCH[1]} > ${WANTED_VER[0]} || ${BASH_REMATCH[2]} >= ${WANTED_VER[1]} )); then
    echo "Go Version needs to be ${WANTED_VER[0]}.${WANTED_VER[1]} or higher: currently $GO_VERSION"
    exit -1
fi

export GO111MODULE=on

mkdir -p tools/build
exepath="$PWD/tools/build"

if [[ $TRAVIS = true ]]; then
    # Sigh.  Work around some rate-limiting hoodoo, hopefully
    for i in 1 2 3 4 5; do
        go mod download && break
        sleep $i
    done
fi

for f in drpcli; do
    [[ -x $exepath/$f ]] && continue
    go build -o "$exepath/$f" github.com/digitalrebar/provision/v4/cmds/$f
done
[[ -x $exepath/go-bindata ]] || go build -o "$exepath/go-bindata" github.com/kevinburke/go-bindata/go-bindata
export PATH="$PWD/tools/build:$PATH"

# set our arch:os build pairs to compile for
builds="amd64:linux amd64:darwin arm64:linux arm:7:linux"

# anything on command line will override our pairs listed above
[[ $* ]] && builds="$*"

for tool in cmds/*; do
    [[ -d $tool ]] || continue
    printf 'Building %s for' "$tool"
    for build in ${builds}; do
        [[ $tool = */kvm-test && ${build##*:} = darwin ]] && continue
        export GOOS="${build##*:}" GOARCH="${build%%:*}"
        printf ' %s:%s' "$GOOS" "$GOARCH"
        tools/build-one.sh "$tool"
    done
    echo
done
echo "To run tests, run: tools/test.sh"
