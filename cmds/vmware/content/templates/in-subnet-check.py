#!/usr/bin/env python
# Simple utility to checks a given IP and GW are in the same subnet, using Python.

###
#  Requres 3 or 4 argumenst as input:
#
#      [quiet] ip_address default_gateway netmask
#
#  quiet     is optional and can only be first argument if used
#  netmask   standard (255.255.255.0) or CIDR bits (24 - no slash) notation
#
#  Exit code will be set to:
#      0    success - IP and GW in same subnet
#      1    failure - IP and GW are not in same subnet
###

import ipaddress
import sys

args = len(sys.argv) - 1
verbose = 1

if args == 4:
  verbose = 0
  del sys.argv[1]
  args = args  - 1

if args == 3:
  if sys.argv[1] == "quiet":
    sys.exit("USAGE: Three arguments passed, first can not be 'quiet'.")
else:
  sys.exit("FATAL: Expect 3 arguments (in order: ip_address gateway netmask")

ip = sys.argv[1]; gw = sys.argv[2]; nm = sys.argv[3]
sn = gw + "/" + nm

if verbose:
  print("Checking:")
  print("...............IP Address: ", ip)
  print("..........Default Gateway: ", gw)
  print("..................Netmask: ", nm)
  print("constructed........Subnet: ", sn)

if ipaddress.IPv4Address(ip) in ipaddress.IPv4Network(sn, strict=False):
  if verbose:
    print("In subnet check succeeded:  IP and GW appear to be in the same subnet")
  sys.exit(0)
else:
  if verbose:
    print("FATAL:  Address '" + ip + "' is NOT in subnet '" + sn + "'.")
  sys.exit(1)
