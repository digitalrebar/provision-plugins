#!/usr/bin/env bash

set -e

if ! [[ -x bin/linux/amd64/ipmi ]]; then
    echo "plugins have not been built!"
    exit 1
fi

case $(uname -s) in
    Darwin)
        shasum="command shasum -a 256";;
    Linux)
        shasum="command sha256sum";;
    *)
        echo "No idea how to check sha256sums"
        exit 1;;
esac

# set our arch:os build pairs to compile for
builds="amd64:linux amd64:darwin arm64:linux arm:7:linux"

# anything on command line will override our pairs listed above
[[ $* ]] && builds="$*"

for build in ${builds}; do
    os=${build##*:}
    arch=${build%:*}

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

    filename="drp-rackn-plugins-$os-$arch"

    tmpdir="$(mktemp -d /tmp/rs-bundle-XXXXXXXX)"
    cp -a ${binpath}/* "$tmpdir"
    (
        cd "$tmpdir"
        $shasum $(find . -type f) >sha256sums
        zip -p -r $filename.zip *
    )
    cp "$tmpdir/$filename.zip" .
    $shasum $filename.zip > $filename.sha256
    rm -rf "$tmpdir"
done


exit 0
