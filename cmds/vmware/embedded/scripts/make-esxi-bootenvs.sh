#!/usr/bin/env bash
# Batch create ESXi BootEnvs and boot.cfg templates via 'bsdtar' or mounted ISO volume

###
#  This tool creates a Digital Rebar Provision bootenv and matching boot.cfg
#  template for a given set of VMware ESXi ISO files.  It does not attempt to
#  read the ISO meta information as that data is hugely inconsistent from
#  vendor to vendor.  This tool relies on the use of a simple map of data to
#  set the meta data information necessary for the bootenv.
#
#  YES - IT SUCKS TO BUILD THE ISO_MAP FOR THIS TOOL - but we value consistency
#  of named objects over ease of adding new bootenvs.  To create a standalone
#  content pack from a single ISO file, use the 'make-esxi-content-pack.sh'
#  script.
#
#   USAGE: run with the '-u' flag to see  full usage instructions
#
#  OUTPUT: The specified 'output_directory' will contain one bootenv
#          (in 'bootenvs' dir) for each ISO found in the 'input_directory',
#          which also has matched a value in the ISO_MAP,
#          and one boot.cfg template file (in 'templates' dir).
#
#          If no ISO is listed in the data map, then it will be
#          skipped.
#
#          Multiple "input_directories" (comma separated, no spaces) can
#          be specified to find ISO images in.
###

export PS4='${BASH_SOURCE}@${LINENO}(${FUNCNAME[0]}): '
set -e

# global variables used through this script
declare -A ISO_MAP               # Associative map of ISO names and meta data
                                 # this can be overriden from an environment
                                 # var of the same name
declare -A ISO_NAMES             # Space separated list of found ISO images
declare ISO                      # ISO name with local path part
declare ISO_MNT                  # location ISO will be mouinted to
declare ISO_NAME                 # Singular ISO to operate on
declare ISO_SHA                  # Calculated sha256 sum of $ISO
declare ESXI_VER                 # Version meta tag for the ESXi version
declare ESXI_SUBVER              # Sub-Version meta tag for the ESXi version
declare VENDOR                   # metadata field for Vendor information
declare MODEL                    # metadata field for Model information
declare ISO_URL                  # metadata field for ISO URL location
declare TITLE                    # the built up Title of the bootenv
declare IN_DIR                   # director(ies) that will be searched for ISOs
declare OUT_DIR                  # where output data will be written to
declare -i dbg=0                 # debug output off by default
declare MODE="bsdtar"            # use 'bsdtar' to extract info out of ISO files
declare BSDTAR                   # the binary location, OS independent
declare EXT_MAP                  # external file that contains ISO_MAP values
declare MARKER="__MARKER__"      # cosmetic marker in ISO_MAP to be ignored
declare -a SPECIFIC_TARGETS      # override and only process listed specific images
                                 # comma separated list, no spaces
declare TMP_DIR                  # specifies alternate tmp directory

###
#  Our ISO_MAP array defines the meta data information to build the bootenv
#  and boot.cfg templates with.
#
#  Format for the array structure
#
#  [key] = [values]
#  like:
#  [key] = ESXI_VER | ESXI_SUBVER | VENDOR | MODEL | ISO_URL
#  spaces above are only for illustration purposes, remove them
#
#  BootEnv Names are built from this ISO_MAP - it's critical to follow
#  some basic rules on the mappings:
#    1. the "key" (ISO name) is not used as the Name of BootEnv, it should
#       follow the Vendor precise naming format as released by the vendor,
#       so as not to cause confusion for the operator, example:
#         VMware-VMvisor-Installer-6.7.0-8169922.x86_64.iso
#    2. none of the fields should contain any dots, dashes, or underscores with
#       the exception of the ISO_URL which can be a properly formed HTTP URL
#       and ESXI_SUBVER - see note 3. below
#    3. if ESXI_SUBVER is carrying a vendor specific sub version, in addition
#       to the standard VMware version (eg '10302608') - you may separate
#       the two with a dash (-), NOT an underscore (_) (eg "13006603-A07")
#    4. use any spaces in any of the value portions of the map will be squashed
#       before evaluating the data
#
#  An example of a valud key/value entry in the map:
#    [VMware-6.7.0u02_13006603.iso]="670u2|13006603|vmware|none|https://downloaad.location.example.com"
#
#  To find all vendor custom ISOs - typically go to the main download page
#  for the VMware version, then click on the "Custom ISOs" tab.  Example:
#
#     https://my.vmware.com/group/vmware/details?downloadGroup=ESXI67U3&productId=742#custom_iso
#
#  "downloadGroup" doesn't always remain stable name format between releases.
#  "productID" changes with each major version of ESXi (eg 6.5 to 6.7)
#
#  VMware DOES NOT always maintain all Vendor ISO references on the "Custom ISOs"
#  tab.  You may have to search for additional reference pages for the downloads.
#
#  NOT ALL VENDORs will be represented in a release on the VMware site - you may
#  need to search the Vendor website for the ISO downloads.
###
ISO_MAP=(
  # 5.x to 6.0 versions - generally not used any more
  [VMware-VMvisor-Installer-201512001-3248547.x86_64.iso]="550u3b|3248547|vmware|none|https://my.vmware.com/web/vmware/details?productId=353&downloadGroup=ESXI55U3B"
  [VMware-VMvisor-Installer-201706001-5572656.x86_64.iso]="600u3a|5572656|vmware|none|https://my.vmware.com/web/vmware/details?productId=491&downloadGroup=ESXI60U3A"
  [VMware-VMvisor-Installer-6.0.0.update02-3620759.x86_64.iso]="600u2|3620759|vmware|none|https://my.vmware.com/web/vmware/details?productId=491&downloadGroup=ESXI60U2"
  # 6.5.0 to 6.5.0u2 versions
  [VMware-ESXi-6.5.0-Update1-7388607-HPE-650.U1.10.2.0.23-Feb2018.iso]="650u1|7388607|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U1-HPE&productId=676"
  [VMware-VMvisor-Installer-201701001-4887370.x86_64.iso]="650a|4887370|vmware|none|https://my.vmware.com/web/vmware/details?downloadGroup=ESXI650A&productId=614"
  [VMware-VMvisor-Installer-6.5.0.update02-10719125.x86_64-DellEMC_Customized-A07.iso]="650u2|10719125-A07|dell|none|https://www.dell.com/support/home/us/en/04/drivers/driversdetails?driverid=5d3h5"
  [VMware-VMvisor-Installer-6.5.0.update02-8294253.x86_64-DellEMC_Customized-A00.iso]="650u2|8294253-A00|dell|none|https://www.dell.com/support/home/us/en/04/drivers/driversdetails?driverid=ckc15"
  [VMware-VMvisor-Installer-6.5.0.update02-8294253.x86_64.iso]="650u2|8294253|vmware|none|https://my.vmware.com/web/vmware/details?productId=614&downloadGroup=ESXI65U2"
  # 6.5.0u3 versions
  [ESXi-6.5.3-13932383-NEC-6.5.3-01.iso]="650u3|13932383|nec|standard|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-NEC&productId=614"
  [VMware-ESXi-6.5.0-Update3-13932383-HPE-preGen9-650.U3.9.6.8.8-Jun2019.iso]="650u3|13932383|hpe|pregen9|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-HPE&productId=614"
  [VMware-ESXi-6.5.0-Update3-14320405-HPE-Gen9plus-650.U3.10.4.5.41-Aug2019.iso]="650u3|14320405|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-HPE&productId=614"
  [VMware-ESXi-6.5.0.update03-13932383-Fujitsu-v430-1.iso]="650u3|13932383-v430|fujitsu|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-FUJITSU&productId=614"
  [VMware-ESXi-6.5.0.update03-13932383-Fujitsu-v431-1.iso]="650u3|13932383-v431|fujitsu|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-FUJITSU&productId=614"
  [VMware-ESXi-6.5U3-RollupISO.iso]="650u3|14293459|vmware|rollup|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI65U3&productId=614"
  [VMware-VMvisor-Installer-6.5.0.update03-13932383.x86_64-DellEMC_Customized-A01.iso]="650u3|13932383-A01|dell|none|https://www.dell.com/support/home/ph/en/phdhs1/drivers/driversdetails?driverid=pcdkd"
  [VMware-VMvisor-Installer-6.5.0.update03-13932383.x86_64.iso]="650u3|13932383|vmware|none|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI65U3&productId=614"
  [VMware-VMvisor-Installer-6.5.0.update03-14320405.x86_64-DellEMC_Customized-A03.iso]="650u3|14320405-A03|dell|none|https://www.dell.com/support/home/ph/en/phdhs1/drivers/driversdetails?driverid=twtv6"
  [VMware_ESXi_6.5.0.update03_15177306_LNV_20191216.iso]="650u3|15177306|lenovo|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-LENOVO&productId=614"
  [VMware_ESXi_6.5.0_13932383_Custom_Cisco_6.5.3.1.iso]="650u3|13932383|cisco|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-CISCO&productId=614"
  [VMware_ESXi_6.5.0_Update3_13932383_hitachi_0400_Blade_HA8000.iso]="650u3|13932383|hitachi|blade-ha8000|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-HITACHI&productId=614"
  [VMware_ESXi_6.5.0_Update3_13932383_hitachi_1400_HA8000VGen10.iso]="650u3|13932383|hitachi|ha8000v-gen10|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI65U3-HITACHI&productId=614"
  # 6.7.0 versions
  [VMware-VMvisor-Installer-6.7.0-8169922.x86_64.iso]="670|8169922|vmware|none|https://my.vmware.com/web/vmware/details?productId=742&downloadGroup=ESXI670"
  # 6.7.0u1 versions
  [ESXi-6.7.1-10302608-NEC-6.7-02.iso]="670u1|10302608|nec|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM_ESXI67U1_NEC&productId=742"
  [ESXi-6.7.1-10302608-NEC-GEN-6.7-02.iso]="670u1|10302608|nec|r120h-t120h-r110j|https://my.vmware.com/group/vmware/details?downloadGroup=OEM_ESXI67U1_NEC&productId=742"
  [VMware-ESXi-6.7.0-10302608-Fujitsu-v460-1.iso]="670u1|10302608|fujitsu|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-FUJITSU&productId=742"
  [VMware-ESXi-6.7.0-Update1-11675023-HPE-Gen9plus-670.U1.10.4.0.19-Apr2019.iso]="670u1|11675023|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-HPE&productId=742"
  [VMware-VMvisor-Installer-6.7.0.update01-10302608.x86_64.iso]="670u1|10302608|vmware|none|https://my.vmware.com/web/vmware/details?productId=742&downloadGroup=ESXI67U1"
  [VMware-VMvisor-Installer-6.7.0.update01-10764712.x86_64-DellEMC_Customized-A04.iso]="670u1|10764712-A04|dell|none|https://www.dell.com/support/home/us/en/19/drivers/driversdetails?driverid=6g5t3"
  [VMware_ESXi_6.7.0.update01_11675023_LNV_20190205.iso]="670u1|11675023|lenovo|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-LENOVO&productId=742"
  [VMware_ESXi_6.7.0_10302608_Custom_Cisco_6.7.1.2.iso]="670u1|10302608|cisco|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-CISCO&productId=742"
  [VMware_ESXi_6.7.0_Update1_10302608_hitachi_0200_Blade_HA8000.iso]="670u1|10302608|hitachi|blade-ha8000|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-HITACHI&productId=742"
  [VMware_ESXi_6.7.0_Update1_11675023_hitachi_1201_HA8000VGen10.iso]="670u1|11675023|hitachi|ha8000v-gen10|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U1-HITACHI&productId=742"
  # 6.7.0u2 versions
  [ESXi-6.7.2-13644319-NEC-6.7-03.iso]="670u2|13644319|nec|standard|https://my.vmware.com/group/vmware/details?downloadGroup=OEM_ESXI67U2_NEC&productId=742"
  [ESXi-6.7.2-13644319-NEC-GEN-6.7-03.iso]="670u2|13644319|nec|r120h-t120h-r110j|https://my.vmware.com/group/vmware/details?downloadGroup=OEM_ESXI67U2_NEC&productId=742"
  [VMware-ESXi-6.7.0-13473784-Fujitsu-v470-1.iso]="670u2|13473784|fujitsu|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U2-FUJITSU&productId=742"
  [VMware-ESXi-6.7.0-Update2-13006603-HPE-Gen9plus-670.U2.10.4.1.8-Apr2019.iso]="670u2|13006603|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U2-HPE&productId=742"
  [VMware-VMvisor-Installer-6.7.0.update02-13006603.x86_64.iso]="670u2|13006603|vmware|none|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI67U2&productId=742"
  [VMware-VMvisor-Installer-6.7.0.update02-13981272.x86_64-DellEMC_Customized-A03.iso]="670u2|13981272-A03|dell|none|https://my.vmware.com/web/vmware/details?downloadGroup=OEM-ESXI67U2-DELLEMC&productId=742"
  [VMware_ESXi_6.7.0.update02_13981272_LNV_20190630.iso]="670u2|13981272|lenovo|none|https://my.vmware.com/web/vmware/details?downloadGroup=OEM-ESXI67U2-LENOVO&productId=742"
  [VMware_ESXi_6.7.0_13006603_Custom_Cisco_6.7.2.1.iso]="670u2|13006603|cisco|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U2-CISCO&productId=742"
  [VMware_ESXi_6.7.0_Update2_13006603_hitachi_1300_HA8KVGen10_RV3K.iso]="670u2|13006603|hitachi|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U2-HITACHI&productId=742"
  # 6.7.0u3 versions
  [ESXi-6.7.3-14320388-NEC-6.7-04.iso]="670u3|14320388|nec|standard|https://my.vmware.com/group/vmware/details?productId=742&downloadGroup=OEM_ESXI67U3_NEC"
  [ESXi-6.7.3-14320388-NEC-GEN-6.7-04.iso]="670u3|14320388|nec|r120h-t120h-r110j|https://my.vmware.com/group/vmware/details?productId=742&downloadGroup=OEM_ESXI67U3_NEC"
  [VMware-ESXi-6.7.0-14320388-Fujitsu-v480-1.iso]="670u3|14320388|fujitsu|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U3-FUJITSU&productId=742"
  [VMware-ESXi-6.7.0-Update3-14320388-HPE-Gen9plus-670.U3.10.4.5.19-Aug2019.iso]="670u3|14320388|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U3-HPE&productId=742"
  [VMware-VMvisor-Installer-6.7.0.update02-13981272.x86_64-DellEMC_Customized-A03.iso]="670u3|13981272-A03|dell|none|https://my.vmware.com/web/vmware/details?downloadGroup=OEM-ESXI67U3-DELLEMC&productId=742"
  [VMware-VMvisor-Installer-6.7.0.update03-14320388.x86_64.iso]="670u3|14320388|vmware|none|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI67U3&productId=742"
  [VMware_ESXi_6.7.0.update03_15160138_LNV_20191216.iso]="670u3|15160138|lenovo|none|https://my.vmware.com/web/vmware/details?downloadGroup=OEM-ESXI67U3-LENOVO&productId=742"
  [VMware_ESXi_6.7.0_14320388_Custom_Cisco_6.7.3.1.iso]="670u3|14320388|cisco|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI67U3-CISCO&productId=742"
  # 6.7.0u3b versions
  [VMware-VMvisor-Installer-201912001-15160138.x86_64.iso]="670u3b|15160138|vmware|none|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI67U3B&productId=742"
  # 7.0.0 versions
  [VMware-VMvisor-Installer-7.0.0-15843807.x86_64.iso]="700|15843807|vmware|none|https://my.vmware.com/group/vmware/details?downloadGroup=ESXI700&productId=974"
  [VMware-VMvisor-Installer-7.0.0-15843807.x86_64-DellEMC_Customized-A00.iso]="700|15843807|dell|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI70GA-DELLEMC&productId=974"
  [VMware_ESXi_7.0.0_15843807_HPE_700.0.0.10.5.0.108_April2020.iso]="700|15843807|hpe|none|https://my.vmware.com/group/vmware/details?downloadGroup=OEM-ESXI70-HPE&productId=974"
  # 6.7.0u3b versions
)

###
#  Output our usage statement
###
function usage() {
  cat <<END_USAGE
USAGE:  $0 [ -d ] [ -m ] -i input_dir(s) [ -o output_dir ] [-a array_file ] [ -s specific_isos ] [ -t tmp_dir ]
   OR:  $0 [ -d ] [ -p | -u ]

        -i input_dir(s)  (required) Set the input director(ies) to scan for ISO files,
                         specify multiple directories separated by commas, no spaces
        -o output_dir    Set the specified directory to output content to
        -a array_file    File with ISO_MAP associative array to use, to override
                         the values provided in this script
        -s specific_isos List of specific ISO file names to process - overriding
                         the ISO_MAP and found isos in the "input_dir(s)" - comma
                         separated with no spaces
        -t tmp_dir       override where the temporary files will be written to
        -m               Use the ISO Mount method instead of use of default 'bsdtar'
                         to extract values and boot.cfg settins from the ISO
        -p               Print the meta data map that will be used to match ISOs
        -u               Output this usage statement
        -d               Turn on output debugging

NOTES:  * an ISO file MUST be found in the input_director(ies), and an ISO_MAP
          set of mata data must also be found
        * ISO file name SHOULD match exactly what the vendor named the ISO file,
          otherwise, operators may be confused which ISO to match to the bootenv
        * 'input_dir' is required
        * if no 'output_dir' specified, one will be generated in /tmp for you
        * default mode is to use 'bsdtar' to extract files from ISO
        * mount mode can be used if 'bsdtar' is not available
        * this tool should run equally well on MacOS X and Linux distros
        * '-p' (print meta data map) will not honor the '-s specific_isos' setting,
          and will instaead print all ISOs listed in the ISO_MAP array
        * if '-a array_file' is used - you must define the ISO_MAP key/value
          pairs:

          ISO_MAP=([key1]=value1 [key2]=value2)
END_USAGE
} # end usage()

###
#  In bsdtar mode check that we can find binary and verify it's a good version ( > v4.x )
###
function check_bsdtar() {
  local _ver
  local _err
  local _msg="FATAL:  'bsdtar' mode requested, but version too old.  Need 3.x or newer.  Install or upgrade.\n\n"

  case $(uname -s) in
    Darwin)
      BSDTAR="$(which tar)"
      _err="'tar' (MacOS X)"
      _msg+=" HINT:  'brew install libarchive --force ; brew link libarchive --force'\n"
      _msg+=" HINT:  also make sure /usr/local/bin in in PATH before /usr/bin\n"
      ;;
    Linux)
      BSDTAR="$(which bsdtar 2> /dev/null)"
      _err="'bsdtar' (Linux)"
      _msg+=" HINT:  'yum -y install bsdtar' or 'apt -y install bsdtar', or similar"
      ;;
  esac

  # bomb if we didn't find it
  [[ $BSDTAR ]] || xiterr 1 "No $_err found.  Install it or try 'iso mount' (-m) mode."

  # we want bsdtar 3.x or newer
  _ver="$($BSDTAR --version | awk ' { print $2 } ' | cut -d"." -f1)"
  ((_ver >= 3)) && return
  echo -e $_msg
  exit 1
} # end check_bsdtar()

###
#  try and provide info to clean up if we exit ungracefully
###
function got_trap() {
  echo ""
  echo "*******************************************************************************"
  echo "TRAP DEBUG:   LASTNO = $1"
  echo "TRAP DEBUG:   LINENO = $2"
  echo "TRAP DEBUG:  COMMAND = $3"
  echo "*******************************************************************************"
  echo ""
  echo "No auto-unmounting or cleanup performed ..."
  [[ $MODE = isomount ]] && echo "Check if ISO image is mounted ('$ISO_MNT')"
  echo "May need to nuke OUT_DIR dir ('$OUT_DIR')"
  echo ""
  echo "UNMOUNT:  ${UNMOUNT[*]}"
  echo " REMOVE:  rm -rf $OUT_DIR.*"
  GOT_TRAP="yes"
} # end got_trap()

###
#  try and unmount iso if in isomount mode, output various exit statements
###
function cleanup() {
  [[ $GOT_TRAP ]] && exit 1
  [[ $OUT_DIR ]] && echo "DRP Contents created in:  $OUT_DIR"
  if [[ $MODE = isomount ]]; then
    grep -q "$ISO_MNT" < <(mount) && [[ $UNMOUNT ]] && ( echo "Unmounting ISO '$ISO'"; $UNMOUNT; )
    [[ -d $ISO_MNT ]] && rmdir "$ISO_MNT"
  fi
  [[ -d $OUT_DIR/tmp && $OUT_DIR/tmp != /tmp ]] && rm -fr "$OUT_DIR/tmp"
} # end cleanup()

###
#  Determine filename baed on lower/uppercase file name variations.
#
#  thank you macOS for making this difficult with upper case names
#  in ISO images, and no mount option to change case
###
function check_files_iso_mount() {
  local _upper_file
  P="$(pwd)"
  for _full in "$@"; do
    local _file="$(basename $_full)"
    local _path="$(dirname $_full)"
    local _good="no"
    cd "$_path"
    _upper_file=$(echo "$_file" | tr '[:lower:]' '[:upper:]')
    if [[ -r $_file ]]; then
      _good="yes"
    else
      [[ -r $_upper_file ]] && _good="yes"
    fi
    [[ $_good = no ]] && xiterr 1 "unable to read file '$_file'"
  done
  cd "$P"
} # end check_files_iso_mount()

###
#  Verify files exist in ISO via bsdtar check
###
function check_files_bsdtar() {
  local _file
  for _file in "$@"; do
    $BSDTAR -tzf "$ISO" "$_file" > /dev/null 2>&1 || xiterr 1 "Unable to validate $ISO contains required $_file."
  done
} # end check_files_bsdtar()

###
#  Process any command line flags and arguments
###
function process_options() {
  local _print_map=0
  while getopts ":udi:o:a:s:t:pm" opt
  do
    case "${opt}" in
      u)  usage; exit 0              ;;
      d)  dbg=1                      ;;
      i)  IN_DIR=$OPTARG             ;;
      o)  OUT_DIR=$OPTARG            ;;
      a)  EXT_MAP=$OPTARG            ;;
      s)  IFS=',' read -a SPECIFIC_TARGETS <<< "$OPTARG"   ;;
      t)  TMP_DIR=$OPTARG            ;;
      m)  MODE="isomount"            ;;
      p)  _print_map=1               ;;
      \?) echo
          echo "Option does not exist : $OPTARG"
          usage
          exit 1
      ;;
    esac
  done

  if [[ $EXT_MAP ]]; then
    EXT_MAP="$(readlink -f "$EXT_MAP")"
    [[ -r $EXT_MAP ]] || xiterr 1 "Unable to read external map file ('$EXT_MAP') for ISO_MAP values."
    source "$EXT_MAP"
    echo "**********************************************************************"
    echo "An external ISO_MAP file specified, not using the internal map values."
    echo "map file:  $EXT_MAP"
    echo "**********************************************************************"
    echo ""
  fi
  if [[ ! $SPECIFIC_TARGETS ]]; then
    SPECIFIC_TARGETS=("${!ISO_MAP[@]}")
  fi

  # print map info and exit if that's what was requested
  (( $_print_map )) && { print_map_info; exit 0; }

  [[ $OUT_DIR ]] || OUT_DIR="$(mktemp -d /tmp/esxi-bootenv-$$.XXXXXXXX)" || mkdir -p "$OUT_DIR"
  OUT_DIR=$(readlink -f "$OUT_DIR")

  [[ -d ${OUT_DIR}/bootenvs ]] || mkdir -p "${OUT_DIR}/bootenvs" || xiterr 1 "bootenvs dir exists already ($OUT_DIR/bootenvs)"
  [[ -d ${OUT_DIR}/templates ]] || mkdir -p "${OUT_DIR}/templates" || xiterr 1 "template dir exists already ($OUT_DIR/templates)"

  # because BASH 5.x FORKS this up - we need to have else true
  [[ $TMP_DIR ]] && mkdir -p "$TMP_DIR" || true

  # set up our isomount/bsdtar TMP working directories
  if [[ $MODE = isomount ]]; then
    [[ $TMP_DIR ]] && ISO_MNT="$TMP_DIR/iso_mnt" || ISO_MNT="${OUT_DIR}/iso_mnt"
    mkdir -p "$ISO_MNT"
  else
    [[ -d ${OUT_DIR}/tmp ]] || mkdir -p "${OUT_DIR}/tmp" || xiterr 1 "bootenvs dir exists already (${OUT_DIR}/tmp)"
  fi

  echo "Output directory initialized:  $OUT_DIR"
  # because BASH 5.x FORKS this up - we need to have else true
  [[ $MODE = isomount ]] && echo " ISO Mount directory created:  $ISO_MNT" || true
} # end process_options()

###
#  If in 'isomount' mode, perform our mounting operations, and set our
#  UNMOUNT tool appropriate to the OS we're on
###
function mount_iso() {
  # sigh ... macOS, thank you
  [[ $ISO ]] || xiterr 1 "ISO is empty - can't mount it"
  [[ $ISO_MNT ]] || xiterr 1 "ISO_MNT not specified"
  [[ $TITLE ]] || xiterr 1 "TITLE not specified"

  echo "    ISO_MNT :: $ISO_MNT"
  echo "  ISO TITLE :: $TITLE"
  echo ""

  case $(uname -s) in
    Darwin)
        hdiutil attach "$ISO" -readonly -mountpoint "$ISO_MNT" > /dev/null
        UNMOUNT=(hdiutil unmount "$ISO_MNT")
      ;;
    Linux)
        mount -o ro,loop "$ISO" "$ISO_MNT" > /dev/null
        UNMOUNT=(umount "$ISO_MNT")
      ;;
    Windows)
        echo "Go get yourself a real Operating System..."
        exit 1
      ;;
    *)  xiterr 1 "Unsupported system type '$(uname -s)'"
      ;;
  esac
} # end mount_iso()

###
#  Build up our list of ISO files in the input_dir specified locations.  If we
#  have a '-s specific_isos' list, filter to only those ISOs
###
function get_iso_names() {
  local _full _check _iso
   _check=$(echo "$IN_DIR" | tr ',' ' ')
  for D in $_check; do
    _full="$(readlink -f $D)"
    DIRS+="$_full "
  done
  while read -r _iso; do
    ISO_NAMES["${_iso##*/}"]="${_iso%/*}"
  done < <(find $DIRS -type f -name "*\.iso" 2> /dev/null)

  [[ ${#ISO_NAMES} = 0 ]] || xiterr 1 "No iso files found in input dir(s):  $DIRS"

  if (( $dbg )); then
    echo "Found ISO names set to:"
    echo ""
    for ISO_NAME in "${!ISO_NAMES[@]}"
    do
      echo "ISO_NAME :: $ISO_NAME"
    done
  fi
} # end get_iso_names()

###
#  Just print the ISO_MAP values for reference.
###
function print_map_info() {
  echo "ISO_MAP information is set as follows:"
  echo ""
  [[ $IN_DIR ]] || KEYS="$(print_map_keys)" || KEYS="${!ISO_NAMES[*]}"
  for ISO_NAME in $KEYS; do
      # skip our cosmetic markers
      [[ $ISO_NAME = $MARKER ]] && continue

      get_iso_meta
      print_iso_meta
      echo ""
  done
} # end print_map_info()

###
#  Helper function to get and print the keys of the ISO_MAP
###
function print_map_keys() {
  # print the full map table
  printf "%s\n" "${!ISO_MAP[@]}"
}

###
#  Sets the global variabls with single ISO meta set of info for ISO_NAME from the ISO_MAP
###
function get_iso_meta() {
  # using the globals var ISO_NAME as the key, get rest of meta data and set global vars
  # ESXI_VER|ESXI_SUBVER|VENDOR|MODEL|ISO_URL
  IFS='|' read -r ESXI_VER ESXI_SUBVER VENDOR MODEL ISO_URL <<< "${ISO_MAP[$ISO_NAME]}"
}

###
#  Print current meta info for ISO_NAME
###
function print_iso_meta() {
  # assume we've already gotten our ISO meta info - fix this in the future
  printf "   ISO Name :: %s\n" "$ISO_NAME"
  printf "   ESXI Ver :: %s\n" "$ESXI_VER"
  printf "ESXI SubVer :: %s\n" "$ESXI_SUBVER"
  printf "     Vendor :: %s\n" "$VENDOR"
  printf "      Model :: %s\n" "$MODEL"
  printf "    ISO URL :: %s\n" "$ISO_URL"
}

###
#  Build our structured TITLE to use in then bootenv name
###
function build_title() {
  [[ $MODEL = none ]] && MOD="" || MOD="_$MODEL"
  TITLE="esxi_${ESXI_VER}-${ESXI_SUBVER}_${VENDOR}${MOD}"
}

###
#  Get the SHA value of the ISO - OS specific - Thanks once again MacOS X
###
function get_iso_sha() {
  # thank you ... macOS
  case $(uname -s) in
    Darwin) ( which shasum > /dev/null ) && SHA="shasum -a 256" ;;
    Linux) ( which sha256sum > /dev/null ) && SHA="sha256sum"   ;;
    *) xiterr 1 "Unsupported system type '$(uname -s)'"         ;;
  esac

  [[ $ISO ]] || xiterr 1 "expect an ISO image path location for ARGv1"
  [[ -r $ISO ]] || xiterr 1 "unable to read specified ISO image ('$ISO')"

  ISO_SHA="$($SHA $ISO | awk ' { print $1 } ')"
}

###
#  Build our bootenv yaml file for a given ISO that has matched
###
function build_bootenv() {

  [[ $MODEL = none ]] && MOD="" || MOD=" ($MODEL)"
  DESC="ESXi ${ESXI_VER}-${ESXI_SUBVER} for ${VENDOR}${MOD}"

  if [[ $MODE = isomount ]]; then
    check_files_iso_mount "$ISO_MNT/mboot.c32" "$ISO_MNT/boot.cfg"
  else
    check_files_bsdtar MBOOT.C32 BOOT.CFG
  fi

  BE_YAML="${OUT_DIR}/bootenvs/$TITLE.yaml"
  cat <<BENV > $BE_YAML
---
Name: $TITLE-install
Description: Install BootEnv for ESXi ${ESXI_VER}-${ESXI_SUBVER} for ${VENDOR}${MOD}
Documentation: |
  Provides VMware BootEnv for $DESC
  For more details, and to download ISO see:

    - $ISO_URL

  NOTE: The ISO filename and sha256sum must match this BootEnv exactly.

Meta:
  color: blue
  icon: zip
  title: RackN Content
OS:
  Codename: esxi
  Family: vmware
  IsoFile: $ISO_NAME
  IsoSha256: $ISO_SHA
  IsoUrl: ""
  Name: $TITLE
  SupportedArchitectures: {}
  Version: $ESXI_VER
OnlyUnknown: false
OptionalParams:
  - provisioner-default-password-hash
RequiredParams: []
Kernel: ../../chain.c32
BootParams: -c {{.Machine.Path}}/boot.cfg
Initrds: []
Loaders:
  amd64-uefi: efi/boot/bootx64.efi
Templates:
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux
    Path: pxelinux.cfg/{{.Machine.HexAddress}}
  - ID: esxi-chain-pxelinux.tmpl
    Name: pxelinux-mac
    Path: pxelinux.cfg/{{.Machine.MacAddr "pxelinux"}}
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe
    Path: '{{.Machine.Address}}.ipxe'
  - ID: esxi-ipxe.cfg.tmpl
    Name: ipxe-mac
    Path: '{{.Machine.MacAddr "ipxe"}}.ipxe'
  - ID: esxi-install-py3.ks.tmpl
    Name: compute.ks
    Path: '{{.Machine.Path}}/compute.ks'
  - ID: ${TITLE}.boot.cfg.tmpl
    Name: boot.cfg
    Path: '{{.Machine.Path}}/boot.cfg'
  - ID: ${TITLE}.boot.cfg.tmpl
    Name: boot-uefi.cfg
    Path: '{{.Env.PathForArch "tftp" "" "amd64"}}/efi/boot/{{.Machine.MacAddr "pxelinux"}}/boot.cfg'
BENV
} # end build_bootenv()

###
#  Sets the KERNEL and MODULES  for boot.cfg - this is called out separately
#  since in the future, we might expand all the ISO files to a share location
#  (eg S3) and just reference the pieces from there - then we can pull that
#  info from that location instead of the ISOs via mount or bsdtar
###
function get_bootcfg_info() {
  if [[ $MODE = isomount ]]; then
    BOOTCFG="$ISO_MNT/boot.cfg"
  else
    mkdir -p "$OUT_DIR/tmp"
    BOOTCFGDIR="$(mktemp -d $OUT_DIR/tmp.boot.cfg.$$.XXXXXXXX)"
    ( cd "$BOOTCFGDIR"; $BSDTAR -xzf "$ISO" BOOT.CFG)
    BOOTCFG="$BOOTCFGDIR/BOOT.CFG"
  fi

  [[ -r $BOOTCFG ]] || xiterr 1 "Unable to read boot.cfg ('$BOOTCFG')"

  KERNEL="$(grep '^kernel=' "$BOOTCFG" | cut -d "=" -f2 | sed 's:/::g')"
  # strip prepended slashes so the "prefix:" redirects work correctly
  MODULES="$(grep '^modules=' "$BOOTCFG" | sed -e 's:^modules=::' -e 's:/::g')"
  # inject freeform modules after "s.v00", but before many other driver modules
#  MODULES=$(echo "$MODULES" | sed 's| --- s.v00| --- s.v00{{ range $key := .Param "esxi/boot-cfg-extra-modules" }} --- {{$key}}{{ end }}|')
  # inject golang template to enable/disable installing tools modules
  MODULES="$(echo "$MODULES" | sed 's| --- tools.t00|{{ if eq (.Param \"esxi/skip-tools\") false -}} --- tools.t00{{end}}|')"

  # because BASH 5.x FORKS this up - we need to have else true
  [[ $MODE = bsdtar && -d $BOOTCFGDIR ]] && rm -rf "$BOOTCFGDIR" || true
} # end get_bootcfg_info()

###
#  Build up our boot.cfg template file
###
### TODO: Hmm..."norts" could be smarter - make it a template, use inventory to determine
### if it's a none "norts" offender, and then set it automatically, then the Param
### could be used to customize for systems we don't know about, or set to False to disable
### the automatic detection templating pieces.
function build_bootcfg() {
  BCFG_TMPL="${OUT_DIR}/templates/$TITLE.boot.cfg.tmpl"
  get_bootcfg_info
  cat <<BOOT > $BCFG_TMPL
bootstate=0
title=Loading ESXi installer for $TITLE
timeout=2
prefix=/{{ trimSuffix "/" (.Env.PathFor "tftp" "/") }}
kernel=$KERNEL
kernelopt=ks={{.Machine.Url}}/compute.ks{{if .ParamExists "kernel-options"}} {{.Param "kernel-options"}}{{end}}{{if .ParamExists "esxi/serial-console"}} {{.Param "esxi/serial-console"}}{{end}}
build=
updated=0
{{ if eq (.Param "esxi/set-norts") true }}norts=1{{ end }}
{{ if .ParamExists "esxi/boot-cfg-extra-options" }}{{ .Param "esxi/boot-cfg-extra-options" }}{{ end }}
modules=$MODULES
BOOT

} # end build_bootcfg()

# simple exit helper function for short conditional command lines
function xiterr() { [[ $1 =~ ^[0-9]+$ ]] && { XIT=$1; shift; } || XIT=1; printf "FATAL: $*\n"; exit $XIT; }

# set our trap and exit functions
trap cleanup EXIT
trap 'got_trap $LASTNO $LINENO $BASH_COMMAND' ERR SIGINT SIGTERM SIGQUIT

# process our command line flags, we also set the ISO_MAP in this function
process_options $*
[[ $MODE = bsdtar ]] && check_bsdtar
get_iso_names
if (( $dbg )); then
    echo ""
    echo "ISO_NAMES:"
    echo "----------"
    printf '%s\n' "${!ISO_NAMES[@]}"
    echo ""
fi
echo ""
declare -A missing_isos missing_map_targets

# "ISO" var used through out functions
for ISO_NAME in "${SPECIFIC_TARGETS[@]}"; do
  DIR_NAME="${ISO_NAMES[$ISO_NAME]}"
    if [[ ! $DIR_NAME ]]; then
      missing_isos["$ISO_NAME"]=true
      continue
    fi
    if [[ ! ${ISO_MAP[$ISO_NAME]} ]]; then
        missing_map_targets["$ISO_NAME"]=true
        continue
    fi
    ISO="$DIR_NAME/$ISO_NAME"
    echo ">>> Building ISO content for: $ISO"
    get_iso_meta
    print_iso_meta
    get_iso_sha
    build_title
    [[ $MODE = isomount ]] && mount_iso
    build_bootenv
    build_bootcfg
    [[ $MODE = isomount ]] && { "${UNMOUNT[@]}" > /dev/null; }
    echo "  COMPLETED :: $ISO"
    echo ""
done
for ISO_NAME in "${!missing_isos[@]}"; do
    echo "Failed to handle vmware $ISO_NAME: could not find ISO file"
done
for ISO_NAME in "${!missing_isos[@]}"; do
    echo "Failed to handle vmware $ISO_NAME: no entry in ISO_MAP"
done
