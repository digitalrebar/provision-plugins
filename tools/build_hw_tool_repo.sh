#!/usr/bin/env bash

#rm -rf hw_repo
mkdir -p hw_repo
cd hw_repo

rm -rf os_dependent os_independent

# Get HP SUM
if [[ ! -e sum-8.4.0-48.rhel7.x86_64.rpm ]] ; then
    curl -O https://downloads.linux.hpe.com/SDR/repo/hpsum/redhat/7/x86_64/current/sum-8.4.0-48.rhel7.x86_64.rpm
fi

# Get STK
if [[ ! -e hp-scripting-tools-11.20-2.rhel7.x86_64.rpm ]] ; then
    curl -O https://downloads.linux.hpe.com/SDR/repo/stk/rhel/7/x86_64/current/hp-scripting-tools-11.20-2.rhel7.x86_64.rpm
fi

# sut sut, ssacli, hponcfg
url="https://downloads.linux.hpe.com/SDR/repo/spp-gen10/rhel/7/x86_64/current/"
declare -a arr=(
  "sut-2.4.0-61.linux.x86_64.rpm"
  "ssacli-3.40-3.0.x86_64.rpm"
  "hponcfg-5.4.0-0.x86_64.rpm"
)
for i in "${arr[@]}"; do
    if [[ ! -e $i ]] ; then
        curl -O $url/$i
    fi
done

url="http://mirror.centos.org/centos/7/os/x86_64/Packages/"
declare -a arr=(
  "libxslt-1.1.28-5.el7.x86_64.rpm"
  "net-tools-2.0-0.24.20131004git.el7.x86_64.rpm"
)
for i in "${arr[@]}"; do
    if [[ ! -e $i ]] ; then
        curl -O $url/$i
    fi
done

if [[ ! -e srvadmin-idrac-9.2.0-3142.13664.el7.x86_64.rpm ]] ; then
    curl -O https://linux.dell.com/repo/hardware/dsu/os_dependent/RHEL7_64/metaRPMS/srvadmin-idrac-9.2.0-3142.13664.el7.x86_64.rpm
fi

url="http://linux.dell.com/repo/hardware/dsu/os_independent/x86_64/"
declare -a arr=(
  "dell-system-update-1.6.0-18.11.00.x86_64.rpm"
)
for i in "${arr[@]}"; do
    if [[ ! -e $i ]] ; then
        curl -O $url/$i
    fi
done

url="https://linux.dell.com/repo/hardware/dsu/os_dependent/RHEL7_64/srvadmin/"
declare -a arr=(
  "srvadmin-omilcore-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-deng-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-omacs-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-argtable2-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-hapi-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-isvc-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-racdrsc-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-rac-components-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-idrac-vmcli-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-racadm4-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-ominst-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-idrac-ivmcli-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-idracadm7-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-idrac7-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-idracadm-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-omcommon-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-nvme-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-storelib-sysfs-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-storelib-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-xmlsup-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-omacore-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-marvellib-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-realssd-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-smcommon-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-storage-9.2.0-3142.13664.el7.x86_64.rpm"
  "srvadmin-storage-cli-9.2.0-3142.13664.el7.x86_64.rpm"
)
for i in "${arr[@]}"; do
    if [[ ! -e $i ]] ; then
        curl -O $url/$i
    fi
done

createrepo ../hw_repo

ln -s ../hw_repo os_independent
mkdir -p os_dependent
cd os_dependent
ln -s ../../hw_repo RHEL7_64
cd -

cd ..
tar -zcvf hw_repo.tgz hw_repo


