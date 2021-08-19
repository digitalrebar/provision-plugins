#!/usr/bin/env python

import json
import logging
import os
import sys
import subprocess


def init_logging():
    """
    Initialize the logging to make it easier
    to find out whats going on
    """
    logging.basicConfig(
        stream=sys.stdout,
        level=logging.DEBUG,
        format='%(asctime)s - %(levelname)s - %(message)s'
    )


def get_volumes(vol_filter="all"):
    """
    Return a list of volumes as reported by localcli storage subsystem
    Takes optional filter which can be
    vmfs, or vfat. Other values will be ignored.

    :return:
    """
    outobj = subprocess.run(
        "localcli --formatter json storage filesystem list",
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        shell=True,
    )
    file_list = json.loads(outobj.stdout)
    if vol_filter is not None:
        vol_filter = vol_filter.lower()
        if vol_filter == 'vmfs':
            file_list = [i['Mount Point'] for i in file_list if i['Type'].lower() == 'VFFS' or
                         'vmfs' in i['Type'].lower()]
        elif vol_filter == 'vfat':
            file_list = [i['Mount Point'] for i in file_list if 'vfat' in i['Type'].lower()]
        else:
            file_list = [i['Mount Point'] for i in file_list]
    return file_list


def remove_vib(vib_name=None):
    """
    Remove a vib by name
    """
    if not vib_name:
        vib_name = "DRP-Agent"
    logging.debug("Attempting to remove {}".format(vib_name))
    outobj = subprocess.run(
        "localcli --formatter json software vib remove -n {}".format(vib_name),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        shell=True,
    )
    cmd_res = json.loads(outobj.stdout)
    if cmd_res is None:
        logging.debug("There was an error removing {}".format(vib_name))
        logging.debug(outobj.stderr)
        raise SystemError
    """{
        "Message": "Operation finished successfully.",
        "Reboot Required": false,
        "VIBs Installed": [],
        "VIBs Removed":[
            "RKN_bootbank_DRP-Agent_v1.3-0"
            ],
       "VIBs Skipped": []
       }
    """
    logging.debug("Message: {}".format(cmd_res["Message"]))
    logging.debug("VIBs Removed From System: {}".format(cmd_res["VIBs Removed"]))
    logging.debug("Reboot Required: {}".format(cmd_res["Reboot Required"]))


def find_config_and_remove(vol_list):
    """
    Look for config file on the passed in vol list.
    If found attempt removal.

    """
    suffix = "rackn/drpy.conf"
    logging.debug("Looking for config files.")
    for vol in vol_list:
        vol += "/{}".format(suffix)
        if os.path.isfile(vol):
            logging.debug("Found {}".format(vol))
            logging.debug("removing {}".format(vol))
            os.unlink(vol)


def zero_out_local_sh():
    """
    Make local.sh great again.

    Puts the local.sh file back to what it was when it was a default.
    This could remove something added by the operator, but it probably
    wont.

    :return:
    """
    default_content = """#!/bin/sh

# local configuration options

# Note: modify at your own risk!  If you do/use anything in this
# script that is not part of a stable API (relying on files to be in
# specific places, specific tools, specific output, etc) there is a
# possibility you will end up with a broken system after patching or
# upgrading.  Changes are not supported unless under direction of
# VMware support.
"""
    local_sh = "/etc/rc.local.d/local.sh"
    if os.path.isfile(local_sh):
        logging.debug("Found local.sh and resetting it to default.")
        with open(local_sh, 'w') as local_sh_file:
            local_sh_file.write(default_content)
        logging.debug("Wrote default local.sh")
    else:
        logging.debug("local.sh not found, so it was not modified.")


if __name__ == "__main__":
    init_logging()
    all_vols = get_volumes(vol_filter="all")
    find_config_and_remove(all_vols)
    zero_out_local_sh()
    remove_vib("DRP-Firewall-Rule")
    remove_vib("DRP-Agent")