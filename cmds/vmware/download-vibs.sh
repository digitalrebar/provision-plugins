#!/usr/bin/env bash

# VIB needs to be stage in S3 bucket for build process
# specify the VIB version to use, or allow at compile time to override which to use
VIB_VERSION=${VIB_VERSION:-"v0.5.2-0.0"}
VIB_FORCE=${VIB_FORCE:-""}
VIB_BASE=${VIB_BASE:-"https://s3-us-west-2.amazonaws.com/get.rebar.digital/artifacts/vibs"}

set -e

echo "VIB_VERSION set to: $VIB_VERSION"

dl_failed() { echo -e "\nFATAL: VIB file failed to download ('$VIB_BASE/$VIB')."; rm -f embedded/$VIB; exit 1; }

mkdir -p embedded
files=(DRP-Agent DRP-Firewall-Rule)
for i in "${files[@]}"
do
    VIB="$i-${VIB_VERSION}.vib"
    B64="$i.vib.base64.tmpl"
    if [[ -e embedded/$i-${VIB_VERSION}.vib && -z "$VIB_FORCE" ]] ; then
        printf "VIB %-17s with version %-10s exists already, skipping...\n" "'$i'" "'$VIB_VERSION'"
        continue
    else
        echo -n "Downloading VIB ('$VIB') ...  "
        wget --quiet -O embedded/$VIB ${VIB_BASE}/${VIB} || dl_failed
        echo "Done."
    fi

    echo "Staging versioned and non-versioned VIBs for '$i'..."
    cp embedded/$VIB embedded/$i.vib
    cat embedded/$i.vib | base64 > content/templates/$B64
    for V in "embedded/$VIB" "embedded/$i.vib" "content/templates/$B64" 
    do
        [[ -r "$V" ]] && echo "Successfully staged VIB/base64 encoded file: $V"
    done
done

