#!/bin/bash

ARCH=$(uname -s | tr "[:upper:]" "[:lower:]")

drpcli contents destroy rackn-license 2>/dev/null >/dev/null
drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 == 0 || $RC2 == 0 ]] ; then
        echo "FAILED - no license - these should have both failed"
else
        echo "Success - no license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 == 0 ]] ; then
        echo "FAILED - no license - ipmi upload"
else
        echo "Success - no license - ipmi upload"
fi

drpcli contents upload cb-expired-license.yaml 2>/dev/null >/dev/null
drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 == 0 || $RC2 == 0 ]] ; then
        echo "FAILED - cb expired license - these should have both failed"
else
        echo "Success - expired cb license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 == 0 ]] ; then
        echo "FAILED - cb expired license - ipmi upload"
else
        echo "Success - cb expired license - ipmi upload"
fi

drpcli contents upload cb-license.yaml 2>/dev/null >/dev/null
drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 != 0 || $RC2 != 0 ]] ; then
        echo "FAILED - cb valid license - these should have both failed"
else
        echo "Success - valid cb license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 != 0 ]] ; then
        echo "FAILED - cb valid license - ipmi upload"
else
        echo "Success - cb valid license - ipmi upload"
fi

drpcli contents upload un-expired-license.yaml 2>/dev/null >/dev/null
drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 == 0 || $RC2 == 0 ]] ; then
        echo "FAILED - un expired license - these should have both failed"
else
        echo "Success - un expired license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 == 0 ]] ; then
        echo "FAILED - cb un expired license - ipmi upload"
else
        echo "Success - cb un expired license - ipmi upload"
fi

drpcli contents upload un-license.yaml 2>/dev/null >/dev/null
drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 != 0 || $RC2 != 0 ]] ; then
        echo "FAILED - un license - these should have both failed"
else
        echo "Success - un license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 != 0 ]] ; then
        echo "FAILED - cb un license - ipmi upload"
else
        echo "Success - cb un license - ipmi upload"
fi

for ((ii=0;ii<11;ii++)) ; do
        drpcli machines create t-$ii 2>/dev/null >/dev/null
done

drpcli tenants create '{ "Name": "teresa" }' 2>/dev/null >/dev/null
RC1=$?
drpcli tenants destroy teresa 2>/dev/null >/dev/null
RC2=$?
if [[ $RC1 == 0 || $RC2 == 0 ]] ; then
        echo "FAILED - un too many nodes license - these should have both failed"
else
        echo "Success - un too many nodes license"
fi
drpcli plugin_providers upload ipmi from ../../provision-plugins/bin/$ARCH/amd64/ipmi 2>/dev/null >/dev/null
RC3=$?
if [[ $RC3 == 0 ]] ; then
        echo "FAILED - cb un too many nodes license - ipmi upload"
else
        echo "Success - cb un too many nodes license - ipmi upload"
fi

for ((ii=0;ii<11;ii++)) ; do
        drpcli machines destroy Name:t-$ii 2>/dev/null >/dev/null
done


