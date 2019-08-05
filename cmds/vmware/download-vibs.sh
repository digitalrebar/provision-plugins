#!/usr/bin/env bash

VIB_VERSION=v0.5.2-0.0

mkdir -p embedded
files=(DRP-Agent DRP-Firewall-Rule)
for i in "${files[@]}"
do
    if [[ ! -e embedded/$i-${VIB_VERSION}.vib ]] ; then
        curl -o embedded/$i-${VIB_VERSION}.vib https://s3-us-west-2.amazonaws.com/get.rebar.digital/artifacts/vibs/$i-${VIB_VERSION}.vib
    fi

    cp embedded/$i-${VIB_VERSION}.vib embedded/$i.vib
    cat embedded/$i.vib | base64 > content/templates/$i.vib.base64.tmpl
done

