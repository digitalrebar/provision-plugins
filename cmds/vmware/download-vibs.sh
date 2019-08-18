#!/usr/bin/env bash

# VIB needs to be stage in S3 bucket for build process
# specify the VIB version to use, or allow at compile time to override which to use
VIB_VERSION=${VIB_VERSION:-"v0.5.5-0.0"}
VIB_FORCE=${VIB_FORCE:-""}
VIB_BASE=${VIB_BASE:-"https://s3-us-west-2.amazonaws.com/get.rebar.digital/artifacts/vibs"}

set -e

main() {
    echo "VIB_VERSION set to:  $VIB_VERSION"
    echo "   VIB_BASE set to:  $VIB_BASE"
    echo "  VIB_FORCE set to:  $VIB_FORCE"
    echo ""

    mkdir -p embedded
    files=(DRP-Agent DRP-Firewall-Rule)

    for i in "${files[@]}"
    do
        VIB="$i-${VIB_VERSION}.vib"
        B64="$i.vib.base64.tmpl"

        if [[ -e embedded/$i-${VIB_VERSION}.vib && -z "$VIB_FORCE" ]] ; then
            printf "VIB %-19s with version %-10s exists already, skipping...\n" "'$i'" "'$VIB_VERSION'"
            continue
        else
            echo -n "Downloading VIB ('$VIB') ...  "
            wget --quiet -O embedded/$VIB ${VIB_BASE}/${VIB} || dl_failed $?
            echo "Done."
        fi

        echo "  Staging versioned and non-versioned VIB for:  $i"
        cp embedded/$VIB embedded/$i.vib
        cat embedded/$i.vib | base64 > content/templates/$B64

        for V in "embedded/$VIB" "embedded/$i.vib" "content/templates/$B64" 
        do
            [[ -r "$V" ]] && echo "  Successfully staged VIB/base64 encoded file:  $V"
        done
    done
}

dl_failed() {
    local _err[1]="Generic error code."
    local _err[2]="Parse error---for instance, when parsing command-line options, the .wgetrc or .netrc..."
    local _err[3]="File I/O error."
    local _err[4]="Network failure."
    local _err[5]="SSL verification failure."
    local _err[6]="Username/password authentication failure."
    local _err[7]="Protocol errors."
    local _err[8]="Server issued an error response."

    rm -f embedded/$VIB
    echo "!! FAILED !!"
    echo "FATAL: VIB file failed to download"
    echo "       source: $VIB_BASE/$VIB"
    echo "       wget error:  $1 - ${_err[$1]}"
    exit 1
}

main $*
