#!/usr/bin/env bash

set -e

cmd=$1

. tools/version.sh

echo "Version = $Prepart$MajorV.$MinorV.$PatchV$Extra-$GITHASH"

export CGO_ENABLED=0

VERFLAGS="-s -w \
          -X github.com/rackn/provision_plugins.RS_MAJOR_VERSION=$MajorV \
          -X github.com/rackn/provision_plugins.RS_MINOR_VERSION=$MinorV \
          -X github.com/rackn/provision_plugins.RS_PATCH_VERSION=$PatchV \
          -X github.com/rackn/provision_plugins.RS_EXTRA=$Extra \
          -X github.com/rackn/provision_plugins.RS_PREPART=$Prepart \
          -X github.com/rackn/provision_plugins.BuildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` \
          -X github.com/rackn/provision_plugins.GitHash=$GITHASH \
          -X github.com/rackn/provision-plugins.RS_MAJOR_VERSION=$MajorV \
          -X github.com/rackn/provision-plugins.RS_MINOR_VERSION=$MinorV \
          -X github.com/rackn/provision-plugins.RS_PATCH_VERSION=$PatchV \
          -X github.com/rackn/provision-plugins.RS_EXTRA=$Extra \
          -X github.com/rackn/provision-plugins.RS_PREPART=$Prepart \
          -X github.com/rackn/provision-plugins.BuildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` \
          -X github.com/rackn/provision-plugins.GitHash=$GITHASH"

arches=("amd64")
oses=("linux" "darwin")
for arch in "${arches[@]}"; do
    for os in "${oses[@]}"; do
        (
            export GOOS="$os" GOARCH="$arch"
            binpath="bin/$os/$arch"
            mkdir -p "$binpath"

            if [ -d "cmds/$cmd/content" ] ; then
                    echo -n "$Prepart$MajorV.$MinorV.$PatchV$Extra-$GITHASH" > cmds/$cmd/content/._Version.meta
            fi

            (cd "cmds/$cmd"; go generate)

            echo "Building binary: ${cmd} for ${arch} ${os}"
            go build -ldflags "$VERFLAGS" -o "$binpath/$cmd" cmds/$cmd/*.go
        )
        done
done
