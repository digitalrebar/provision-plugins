#!/usr/bin/env bash

set -e

mkdir -p embedded

files=(libyaml-0.1.4-11.el7_0.x86_64.rpm PyYAML-3.10-11.el7.x86_64.rpm libtommath-0.42.0-6.el7.x86_64.rpm curtin-19.2-mbcache.tar.gz python-oauthlib-0.6.0-2.el7.noarch.rpm libtomcrypt-1.17-26.el7.x86_64.rpm python2-crypto-2.6.1-15.el7.x86_64.rpm dpkg-1.17.27-1.el7.x86_64.rpm jsonschema-3.0.2.tar.gz)
for i in "${files[@]}"
do
    if [[ ! -e embedded/$i ]] ; then
        curl -o embedded/$i https://s3-us-west-2.amazonaws.com/rackn-sledgehammer/curtin-plugin-files/$i
    fi
done

