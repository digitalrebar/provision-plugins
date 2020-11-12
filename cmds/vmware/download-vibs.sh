#!/usr/bin/env bash

# VIB needs to be stage in S3 bucket for build process
# specify the VIB version to use, or allow at compile time to override which to use
FW_VIB_VERSION=${FW_VIB_VERSION:-"v1.0.0-3"}
AGT_VIB_VERSION=${AGT_VIB_VERSION:-"v1.2-1"}
VIB_FORCE=${VIB_FORCE:-""}
VIB_BASE=${VIB_BASE:-"https://s3-us-west-2.amazonaws.com/get.rebar.digital/artifacts/vibs"}
AGT_VIB_NAME="DRP-Agent"
FW_VIB_NAME="DRP-Firewall-Rule"

set -e

main() {
    echo "FW_VIB_VERSION set to:  $FW_VIB_VERSION"
    echo "AGT_VIB_VERSION set to:  $AGT_VIB_VERSION"
    echo "   VIB_BASE set to:  $VIB_BASE"
    echo "  VIB_FORCE set to:  $VIB_FORCE"
    echo ""

    mkdir -p embedded

    AGT_VIB="$AGT_VIB_NAME-${AGT_VIB_VERSION}.vib"
    AGT_B64="$AGT_VIB_NAME.vib.base64.tmpl"
    # Fetch AGT if we dont already have it
    if [[ -e embedded/${AGT_VIB_NAME}-${AGT_VIB_VERSION}.vib && -z "$VIB_FORCE" ]] ; then
        printf "VIB %-19s with version %-10s exists already, skipping download...\n" "'${AGT_VIB_NAME}'" "'${AGT_VIB_VERSION}'"
    else
        echo -n "Downloading VIB ('$AGT_VIB_NAME') ...  "
        wget --quiet -O embedded/$AGT_VIB ${VIB_BASE}/${AGT_VIB} || dl_failed $?
        echo "Done."
    fi

    FW_VIB="$FW_VIB_NAME-${FW_VIB_VERSION}.vib"
    FW_B64="$FW_VIB_NAME.vib.base64.tmpl"
    # Fetch FW is we dont already have it
    if [[ -e embedded/${FW_VIB_NAME}-${FW_VIB_VERSION}.vib && -z "$VIB_FORCE" ]] ; then
        printf "VIB %-19s with version %-10s exists already, skipping download...\n" "'${FW_VIB_NAME}'" "'${FW_VIB_VERSION}'"
    else
        echo -n "Downloading VIB ('$FW_VIB_NAME') ...  "
        wget --quiet -O embedded/$FW_VIB ${VIB_BASE}/${FW_VIB} || dl_failed $?
        echo "Done."
    fi

    # Stage the AGT file
    echo "  Staging versioned and non-versioned VIB for:  $AGT_VIB_NAME"
    cp embedded/$AGT_VIB embedded/$AGT_VIB_NAME.vib
    cat embedded/$AGT_VIB_NAME.vib | base64 > content/templates/$AGT_B64

    for V in "embedded/$AGT_VIB" "embedded/$AGT_VIB_NAME.vib" "content/templates/$AGT_B64"
    do
        [[ -r "$V" ]] && echo "  Successfully staged VIB/base64 encoded file:  $V"
    done

    # Stage the FW file
    echo "  Staging versioned and non-versioned VIB for:  $FW_VIB_NAME"
    cp embedded/$FW_VIB embedded/$FW_VIB_NAME.vib
    cat embedded/$FW_VIB_NAME.vib | base64 > content/templates/$FW_B64

    for V in "embedded/$FW_VIB" "embedded/$FW_VIB_NAME.vib" "content/templates/$FW_B64"
    do
        [[ -r "$V" ]] && echo "  Successfully staged VIB/base64 encoded file:  $V"
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

    rm -f embedded/$AGT_VIB
    echo "!! FAILED !!"
    echo "FATAL: VIB file failed to download"
    echo "       source: $VIB_BASE/$VIB"
    echo "       wget error:  $1 - ${_err[$1]}"
    exit 1
}

main $*
