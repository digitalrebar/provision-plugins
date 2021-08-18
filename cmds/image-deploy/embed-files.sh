#!/bin/bash

# $0: Script to download prereq packages and curtin for plugin build.
set -e

# curtin_ver should be a REF in our fork of curtin. It should take a form of:
#     "<curtin tagged release>_rackn[rackn tagged release]>"
# Examples:
#     20.2-rackn (HEAD of 20.2-rackn branch) will download curtin-20.2-rackn.tar.gz
#     20.2-rackn3 (commit tagged "3" in 20.2-rackn branch) will download curtin-20.2-rackn3.tar.gz
#     master (HEAD of curtin fork, does not include rackn branch changes) will download curtin-master.tar.gz
curtin_ver="20.2-rackn"

# Packages needed for curtin.
prereqs=(libyaml-0.1.4-11.el7_0.x86_64.rpm
PyYAML-3.10-11.el7.x86_64.rpm
libtommath-0.42.0-6.el7.x86_64.rpm
python-oauthlib-0.6.0-2.el7.noarch.rpm
libtomcrypt-1.17-26.el7.x86_64.rpm
python2-crypto-2.6.1-15.el7.x86_64.rpm
dpkg-1.17.27-1.el7.x86_64.rpm)

mkdir -p embedded

for package in "${prereqs[@]}"; do
	if [[ ! -e embedded/${package} ]] ; then
		curl -sfgLo embedded/${package} https://s3-us-west-2.amazonaws.com/rackn-sledgehammer/curtin-plugin-files/${package}
	fi
done

if [[ ! -e embedded/${curtin_ver}.tar.gz ]] ; then
	curl -sfgLo embedded/curtin-${curtin_ver}.tar.gz https://github.com/digitalrebar/curtin/tarball/${curtin_ver}
fi
