#!/usr/bin/env bash
# Batch create ESXi BootEnvs and boot.cfg templates via 'bsdtar' or mounted ISO volume

###
#  This tool creates a Digital Rebar Provision bootenv and matching boot.cfg
#  template for a given set of VMware ESXi ISO files.  In addition, if requested,
#  full content pack (workflows and stages) and metadata will also be created.

#  By default this tool will use the built-in (or external file with) ISO_MAP[]
#  metadata information.  If the operator specifies the '-g' (generated) flag,
#  then the ISO filename will be used and minimal metadta and bootenv information
#  will be created.

#  YES - IT SUCKS TO BUILD THE ISO_MAP FOR THIS TOOL - but we value consistency
#  of named objects over ease of adding new bootenvs.  To create a standalone
#  content pack from a single ISO file, use the 'make-esxi-content-pack.sh'
#  script.  Unfortunately - it is NOT possible to obtain consistent metadata
#  from the ISO filename or the ISO meta content information.  Thank you vendors.
#
#   USAGE: run with the '-u' flag to see  full usage instructions
###

export PS4='${BASH_SOURCE}@${LINENO}(${FUNCNAME[0]}): '
set -e

# global variables used through this script
declare -A ISO_MAP               # Associative map of ISO names and meta data
                                 # this can be overriden from an environment
                                 # var of the same name
declare -A ISO_NAMES             # Space separated list of found ISO images
declare -A MISSING_ISOS          # lists ISOs requested but not found
declare ISO                      # ISO name with local path part
declare ISO_MNT                  # location ISO will be mouinted to
declare ISO_NAME                 # Singular ISO to operate on
declare ISO_SHA                  # Calculated sha256 sum of $ISO
declare ESXI_VER=0               # Version meta tag for the ESXi version
declare ESXI_SUBVER              # Sub-Version meta tag for the ESXi version
declare VENDOR                   # metadata field for Vendor information
declare MODEL                    # metadata field for Model information
declare ISO_URL                  # metadata field for ISO URL location
declare TITLE                    # the built up Title of the bootenv
declare IN_DIR                   # director(ies) that will be searched for ISOs
declare OUT_DIR                  # where output data will be written to
declare -i dbg=0                 # debug output off by default
declare CONTENT=""               # create full content pack from ISOs
declare GENERATE=""              # generate Name/metadata from ISO file name
declare DO_BUNDLE=""             # attempt to do a 'drpcli contents bundle ...'
declare DO_UPLOAD=""             # attempt to do a 'drpcli contents upload ...'
declare MODE="bsdtar"            # use 'bsdtar' to extract info out of ISO files
declare BSDTAR                   # the binary location, OS independent
declare EXT_MAP                  # external file that contains ISO_MAP values
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
  # 6.5.0 versions
  [RKN-ESXi-6.5.0-20190701001s-no-tools.iso]="650u3|x13932383|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-ESXi-6.5.0-20190701001s-no-tools.iso"
  [RKN-ESXi-6.5.0-20190701001s-standard.iso]="650u3|13932383|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-ESXi-6.5.0-20190701001s-standard.iso"
  [RKN-ESXi-6.5.0-20190702001-no-tools.iso]="650u3|x13932383|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-ESXi-6.5.0-20190702001-no-tools.iso"
  [RKN-ESXi-6.5.0-20190702001-standard.iso]="650u3|13932383|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-ESXi-6.5.0-20190702001-standard.iso"
  [RKN-HPE-ESXi-6.5.0-Update3-Gen9plus-650.U3.10.5.5.16.iso]="650u3|13932383|hpe|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-HPE-ESXi-6.5.0-Update3-Gen9plus-650.U3.10.5.5.16.iso"
  [RKN-HPE-ESXi-6.5.0-Update3-preGen9-650.U3.9.6.8.8.iso]="650u3|13932383|hpe|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-HPE-ESXi-6.5.0-Update3-Synergy-650.U3.10.5.6.10.iso"
  [RKN-HPE-ESXi-6.5.0-Update3-Synergy-650.U3.10.5.6.10.iso]="650u3|105610|hpe|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.5/RKN-HPE-ESXi-6.5.0-Update3-preGen9-650.U3.9.6.8.8.iso"
  # 6.7.0 versions
  [RKN-DellEMC-ESXi-6.7U2-13981272-A02.iso]="67u2|13981272|dell|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-DellEMC-ESXi-6.7U2-13981272-A02.iso"
  [RKN-ESXi-6.7.0-20190401001s-no-tools.iso]="670|xs20190401001|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.0-20190401001s-no-tools.iso"
  [RKN-ESXi-6.7.0-20190401001s-standard.iso]="670|s20190401001|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.0-20190401001s-standard.iso"
  [RKN-ESXi-6.7.0-20190402001-no-tools.iso]="670|x20190402001|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.0-20190402001-no-tools.iso"
  [RKN-ESXi-6.7.0-20190402001-standard.iso]="670|20190402001|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.0-20190402001-standard.iso"
  [RKN-ESXi-6.7.0-update2-13006603-custom-hitachi-1300.iso]="670u2|13006603|hitachi|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.0-update2-13006603-custom-hitachi-1300.iso"
  [RKN-ESXi-6.7.2-13644319-NEC-6.7-03.iso]="672u3|13644319|nec|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-ESXi-6.7.2-13644319-NEC-6.7-03.iso"
  [RKN-Fujitsu-VMvisor-Installer-6.7-13473784-v470-1.iso]="670u2|13473784|fujitsu|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-Fujitsu-VMvisor-Installer-6.7-13473784-v470-1.iso"
  [RKN-HPE-ESXi-6.7.0-Update2-Gen9plus-670.U2.10.4.1.8.iso]="670u2|13006603|hpe|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-HPE-ESXi-6.7.0-Update2-Gen9plus-670.U2.10.4.1.8.iso"
  [RKN-Lenovo_ESXi6.7u2-13981272_20190630.iso]="670u2|13981272|lenovo|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-Lenovo_ESXi6.7u2-13981272_20190630.iso"
  [RKN-VMware-ESXi-6.7.0-13006603-Custom-Cisco-6.7.2.1.iso]="670|13006603|cisco|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/6.7/RKN-VMware-ESXi-6.7.0-13006603-Custom-Cisco-6.7.2.1.iso"
  # 7.0.0 versions
  [RKN-HPE-Custom-AddOn_700.0.0.10.5.0-108.iso]="700|15843807|hpe|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/7.0/RKN-HPE-Custom-AddOn_700.0.0.10.5.0-108.iso"
  [RKN-ESXi-7.0.0-15843807-no-tools.iso]="700|x15843807|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/7.0/RKN-ESXi-7.0.0-15843807-no-tools.iso"
  [RKN-ESXi-7.0.0-15843807-standard.iso]="700|15843807|vmware|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/7.0/RKN-ESXi-7.0.0-15843807-standard.iso"
  [RKN-DEL-ESXi-700_15843807-A00.iso]="700|15843807|dell|none|https://rackn-repo.s3-us-west-2.amazonaws.com/isos/vmware/esxi/7.0/RKN-DEL-ESXi-700_15843807-A00.iso"
)

###
#  Output our usage statement
###
function usage() {
  _scr=$(basename $0)
  _pad=$(printf "%${#_scr}s" " ")
  cat <<END_USAGE
USAGE:  $_scr [ -d ] [ -gcmBU ] -i input_dir(s) [ -o output_dir ] [-a array_file ]
        $_pad [ -s specific_isos ] [ -t tmp_dir ]
   OR:  $_scr [ -d ] [ -gcm ] -s specific_isos [ -o output_dir ] [-a array_file ]
        $_pad [ -t tmp_dir ]
   OR:  $_scr [ -d ] [ -p | -u | -x ]

        -g               Generate the meta data information from the name of the
                         ISO, not from the metadata map information (ISO_MAP[])
        -c               Create full Content Pack in the '-o output_dir' which
                         includes workflows, stages, etc. (default is to only
                         create bootenvs and boot.cfg templates)
        -B               attempt to do a 'drpcli contents bundle ...' operation
                         (requires '-c', and RS_ENDPOINT, RS_KEY or similar)
        -U               attempt to do a 'drpcli contents upload ...' operation
                         (requires '-c', and RS_ENDPOINT, RS_KEY or similar)
        -m               Use the ISO Mount method instead of use of default 'bsdtar'
                         to extract values and boot.cfg settins from the ISO
        -i input_dir(s)  (required) Set the input director(ies) to scan for ISO files,
                         specify multiple directories separated by commas, no spaces
        -o output_dir    Set the specified directory to output content to
        -a array_file    File with ISO_MAP associative array to use, to override
                         the values provided in this script
        -s specific_isos List of specific ISO file names to process - overriding
                         the ISO_MAP and found isos in the 'input_dir(s)' - comma
                         separated with no spaces
        -t tmp_dir       override where the temporary files will be written to
        -p               Print the meta data map that will be used to match ISOs
        -u               Output the short usage statement
        -x               Output the eXtended long usage statement (Notes and Examples)
        -d               Turn on output debugging

END_USAGE

  [[ -z "$XTND" ]] && printf ">>> For eXtended help output with Notes and Examples, use '-x'. <<<\n\n"

  if [[ "$XTND" ]]
  then
    cat <<XTND_USAGE

NOTES:  * an ISO file MUST be found in the input_director(ies), or directly specified
          via the '-s specific_isos' flag
        * if '-s specific_isos' AND '-i input_director(ies)' are both specified, then
          the '-s specific_isos' filters the ISOs found in the input directories.
        * Names and metadata will be generated from the ISO_MAP metadata in
          the script or from external '-a array_file' UNLESS you specify the
          '-g' flag which will generate Name and metadata information from
          the ISO filename
        * ISO file name SHOULD match exactly what the vendor named the ISO file,
          otherwise, operators may be confused which ISO to match to the bootenv
        * one of '-i input_dir' or '-s specific_isos' is required
        * if no 'output_dir' specified, one will be generated in /tmp for you
        * default mode is to use 'bsdtar' to extract files from ISO
        * '-m' mount mode can be used if 'bsdtar' is not available
        * this tool should run equally well on MacOS X and Linux distros
          (are you on Windows? too bad ... ha ha ha!)
        * '-p' (print meta data map) will not honor the '-s specific_isos' setting,
          and will instead print all ISOs listed in the ISO_MAP array
        * if '-a array_file' is used - you must define the ISO_MAP key/value
          pairs the external array_file, as:
            ISO_MAP=([key1]=value1 [key2]=value2)
          see script example for full usage of ISO_MAP[] array
        * if '-g' (generate) is used, '-a array_file' will be ignored
        * if either '-B' or '-U' specified, '-c' must also be specified,
          requires 'drpcli' binary can access the DRP Endpoint via the standard
          CLI mechanisms (.rsclirc, RS_ENDPOINT, RS_KEY, RS_USERNAME, RS_PASSWORD, etc.)

EXAMPLES:

  * Emulates the removed 'make-esxi-content-packs.sh' script - creates a full
    "mini-content" pack with bootenvs, templates, stages, workflows, etc:

        $_scr -c -g -s foo-1.iso,bar-2.iso

    Optionally add '-B' and '-U' to Bundle/Upload (respectively) to a DRP endpoint.

  * Generate just bootenv and boot.cfg content pieces using the ISO_MAP array,
    for all ISOs found in "./isos" directory:

        $_scr -i ./isos

    This is what RackN uses to build bootenvs and boot.cfg templates that are
    released in the 'vmware' plugin.

  * Using the builtin ISO_MAP[] array, find ISO filies in iso-dir1/ and iso-dir2/ dirs,
    but only build content for the '-s' specified ISO targets listed.  Turn on debug
    mode to provide more output.  Write output to '/tmp/content' directory.

        $_scr -d -i ./iso-dir1,./iso-dir2 -o /tmp/content -s foo-1.iso,bar-2.iso

XTND_USAGE
  fi # end extended usage output statement

} # end usage()

# because, once again - thank you Mac OS X ...
# BASH portable replacement for "readlink -f"
function get_realpath() {
    [[ ! -r "$1" ]] && xiterr 1 "file/dir '$1' does not exist"
    [[ -d "$1" ]] && { cd $1; echo $PWD; return 0; }
    [[ -n "$no_symlinks" ]] && local pwdp='pwd -P' || local pwdp='pwd'
    echo "$( cd "$( echo "${1%/*}" )" 2>/dev/null; $pwdp )"/"${1##*/}"
    return 0
}

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
#  Determine filename based on lower/uppercase file name variations.
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
  while getopts ":uxdcBUgi:o:a:s:t:mp" opt
  do
    case "${opt}" in
      u)  usage; exit 0              ;;
      x)  XTND="true"; usage; exit 0 ;;
      d)  dbg=1                      ;;
      c)  CONTENT="true"             ;;
      B)  DO_BUNDLE="true"           ;;
      U)  DO_UPLOAD="true"           ;;
      g)  GENERATE="true"            ;;
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

  if [[ -n "$EXT_MAP" && "$GENERATE" == "false" ]]
  then
    EXT_MAP="$(get_realpath "$EXT_MAP")"
    [[ -r $EXT_MAP ]] || xiterr 1 "Unable to read external map file ('$EXT_MAP') for ISO_MAP values."
    source "$EXT_MAP"
    echo "**********************************************************************"
    echo "An external ISO_MAP file specified, not using the internal map values."
    echo "map file:  $EXT_MAP"
    echo "**********************************************************************"
    echo ""
  fi

  if [[ ! $SPECIFIC_TARGETS ]]
  then
    SPECIFIC_TARGETS=("${!ISO_MAP[@]}")
  fi

  # print map info and exit if that's what was requested
  (( $_print_map )) && { print_map_info; exit 0; }

  [[ "$OUT_DIR" ]] && mkdir -p "$OUT_DIR" || OUT_DIR="$(mktemp -d /tmp/esxi-bootenv-$$.XXXXXXXX)"
  OUT_DIR=$(get_realpath "$OUT_DIR")

  [[ -d ${OUT_DIR}/bootenvs ]] || mkdir -p "${OUT_DIR}/bootenvs" || xiterr 1 "bootenvs dir exists already ($OUT_DIR/bootenvs)"
  [[ -d ${OUT_DIR}/templates ]] || mkdir -p "${OUT_DIR}/templates" || xiterr 1 "template dir exists already ($OUT_DIR/templates)"

  if [[ "$CONTENT" ]]
  then
    [[ -d ${OUT_DIR}/workflows ]] || mkdir -p "${OUT_DIR}/workflows" || xiterr 1 "workflows dir exists already ($OUT_DIR/workflows)"
    [[ -d ${OUT_DIR}/stages ]] || mkdir -p "${OUT_DIR}/stages" || xiterr 1 "template dir exists already ($OUT_DIR/stages)"
  fi

  [[ "$DO_BUNDLE" ]] && WHAT="Bundle" || true
  [[ "$DO_UPLOAD" ]] && WHAT="$WHAT Upload" || true
  # validate our options around Content, Bundle, and Upload
  if [[ "$CONTENT" ]]
  then
    if [[ "$DO_BUNDLE" || "$DO_UPLOAD" ]]
    then
      echo "Requesting content operations for: $WHAT"
    else
      echo "Requesting content pack creation, but no Bundle or Upload operations."
    fi
  else
    if [[ "$DO_BUNDLE" || "$DO_UPLOAD" ]]
    then
      xiterr 1 "$WHAT operation(s) requested, but no content create specified."
    fi
  fi

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

  # if we have specified input directories, process them
  if [[ -n "$IN_DIR" ]]
  then
    _check=$(echo "$IN_DIR" | tr ',' ' ')
    for D in $_check
    do
      _full="$(get_realpath $D)"
      DIRS+="$_full "
    done

    while read -r _iso
    do
      ISO_NAMES["${_iso##*/}"]="${_iso%/*}"
    done < <(find $DIRS -type f -name "*\.iso" 2> /dev/null)

  # otherwise, we must have individual ISOs in '-s ... '
  else
    echo "No '-i input_dir' specified, trying to process '-s specific_targets'"
    for _iso in ${SPECIFIC_TARGETS[@]}
    do
      ISO_NAMES["${_iso##*/}"]="${_iso%/*}"
    done
  fi

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

  [[ -z "PRINT_LIMITED" && -z "$GENERATE" ]] && echo "Limited metadata in 'generate' mode ('-g')."
  PRINT_LIMITED="true"

  printf "   ISO Name :: %s\n" "$ISO_NAME"
  if [[ -z "$GENERATE" ]]
  then
    printf "   ESXI Ver :: %s\n" "$ESXI_VER"
    printf "ESXI SubVer :: %s\n" "$ESXI_SUBVER"
    printf "     Vendor :: %s\n" "$VENDOR"
    printf "      Model :: %s\n" "$MODEL"
    printf "    ISO URL :: %s\n" "$ISO_URL"
  fi
}

###
#  Build our structured TITLE to use in then bootenv name
###
function build_title() {

  if [[ "$GENERATE" ]]
  then
    TITLE="esxi-$(echo $ISO_NAME | sed 's/\.iso//g')"
  else
    [[ $MODEL = none ]] && MOD="" || MOD="_$MODEL"
    TITLE="esxi_${ESXI_VER}-${ESXI_SUBVER}_${VENDOR}${MOD}"
  fi
}

###
#  Get the SHA value of the ISO - OS specific - Thanks once again MacOS X
###
function get_iso_sha() {
  # thank you ... macOS
  case $(uname -s) in
    Darwin) ( which shasum    > /dev/null ) && SHA="shasum -a 256" ;;
    Linux)  ( which sha256sum > /dev/null ) && SHA="sha256sum"     ;;
    *) xiterr 1 "Unsupported system type '$(uname -s)'"            ;;
  esac

  [[ $ISO ]] || xiterr 1 "expect an ISO image path location for ARGv1"
  [[ -r $ISO ]] || xiterr 1 "unable to read specified ISO image ('$ISO')"

  ISO_SHA="$($SHA $ISO | awk ' { print $1 } ')"
}

###
#  Build our bootenv yaml file for a given ISO that has matched
###
function build_bootenv() {
  if [[ "$GENERATE" ]]
  then
    DESC="${TITLE}"
    ISO_URL="unknown (no download location metadata was provided at build time)"
    ESXI_VER=0
  else
    [[ $MODEL = none ]] && MOD="" || MOD=" ($MODEL)"
    DESC="ESXi ${ESXI_VER}-${ESXI_SUBVER} for ${VENDOR}${MOD}"
  fi

  if [[ $MODE = isomount ]]; then
    check_files_iso_mount "$ISO_MNT/mboot.c32" "$ISO_MNT/boot.cfg"
  else
    check_files_bsdtar MBOOT.C32 BOOT.CFG
  fi

  BE_YAML="${OUT_DIR}/bootenvs/$TITLE.yaml"
  cat <<BENV > $BE_YAML
---
Name: $TITLE-install
Description: Install BootEnv for $DESC
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

  # get original defined Kernel value
  KERNEL="$(grep '^kernel=' "$BOOTCFG" | cut -d "=" -f2 | sed 's|/||g')"
  # strip prepended slashes so the "prefix:" redirects work correctly
  MODULES="$(grep '^modules=' "$BOOTCFG" | sed -e 's|^modules=||' -e 's|/||g')"
  # inject freeform modules after "s.v00", but before many other driver modules
  MODULES=$(echo "$MODULES" | sed 's|\( --- s.v00\)|\1{{ range $key := .Param "esxi/boot-cfg-extra-modules" }} --- {{$key}}{{ end }}|')
  # inject golang template to enable/disable installing tools modules
  MODULES="$(echo "$MODULES" | sed 's| --- tools.t00|{{ if eq (.Param \"esxi/skip-tools\") false }} --- tools.t00{{end}}|')"
  # inject template for adding DRPY agent at install time
  MODULES="$(echo "$MODULES" | sed 's|\(sb.v00 --- \)|\1{{ if (.Param \"esxi/add-drpy-agent\") }}{{ .Param "esxi/add-drpy-agent" }} --- {{end}}|')"
  # inject the DRPY Firewall VIB
  MODULES="$(echo "$MODULES" | sed 's|\(sb.v00 --- \)|\1{{ if (.Param \"esxi/add-drpy-firewall\") }}{{ .Param "esxi/add-drpy-firewall" }} --- {{end}}|')"

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
modules=${MODULES}
BOOT

} # end build_bootcfg()

# build stages content in addition to bootenvs and templates
function build_stage() {
  S_YAML="${OUT_DIR}/stages/$TITLE.yaml"
  cat <<STAGE > $S_YAML
---
Name: $TITLE-install
BootEnv: $TITLE-install
Description: "Stage for custom ESXi $TITLE"
ReadOnly: true
Reboot: false
Meta:
  color: yellow
  icon: download
  title: RackN Content
OptionalParams: []
Profiles: []
RequiredParams: []
Tasks: []
Templates: []
STAGE
}

# build workflows content in addition to bootenvs and templates
function build_workflow() {
  W_YAML="${OUT_DIR}/workflows/$TITLE-install.yaml"
  cat <<WF > $W_YAML
---
Name: $TITLE-install
Description: "Install custom ESXi $TITLE"
Documentation: ""
Meta:
  color: yellow
  icon: shuffle
  title: RackN Content
ReadOnly: true
Stages:
  - $TITLE-install
  - finish-install
  - complete
WF
}

# Build our content meta info files
function build_meta() {
  echo "Building meta data files for content bundle ... "
  printf "RackN, Inc." > ${OUT_DIR}/._Author.meta
  printf "https://github.com/digitalrebar/provision-plugins/tree/v4/cmds/vmware" > ${OUT_DIR}/._CodeSource.meta
  printf "blue" > ${OUT_DIR}/._Color.meta
  printf "archive" > ${OUT_DIR}/._Icon.meta
  printf "RackN" > ${OUT_DIR}/._Copyright.meta
  printf "Generated VMware ESXi Content - $STAMP" > ${OUT_DIR}/._Description.meta
  printf "Generated ESXi Content - $STAMP" > ${OUT_DIR}/._DisplayName.meta
  printf "https://provision.readthedocs.io/en/latest/doc/content-packages/vmware.html" > ${OUT_DIR}/._DocUrl.meta
  printf "RackN" > ${OUT_DIR}/._License.meta
  printf "vmware-generated-$STAMP" > ${OUT_DIR}/._Name.meta
  printf "1000" > ${OUT_DIR}/._Order.meta
  printf "sane-exit-codes" > ${OUT_DIR}/._RequiredFeatures.meta
  printf "RackN" > ${OUT_DIR}/._Source.meta
  printf "enterprise,rackn,esxi,vmware" > ${OUT_DIR}/._Tags.meta
  printf "$STAMP" > ${OUT_DIR}/._Version.meta
}

# simple exit helper function for short conditional command lines
function xiterr() { [[ $1 =~ ^[0-9]+$ ]] && { XIT=$1; shift; } || XIT=1; printf "FATAL: $*\n"; exit $XIT; }

# set our trap and exit functions
trap cleanup EXIT
trap 'got_trap $LASTNO $LINENO $BASH_COMMAND' ERR SIGINT SIGTERM SIGQUIT

# process our command line flags, we also set the ISO_MAP in this function
process_options $*
[[ $MODE = bsdtar ]] && check_bsdtar || true
get_iso_names
if (( $dbg )); then
    echo ""
    echo "ISO_NAMES:"
    echo "----------"
    printf '%s\n' "${!ISO_NAMES[@]}"
    echo ""
fi
echo ""

STAMP=$(date +v%Y.%m.%d-%H%M%S)

# "ISO" var used through out functions
[[ "$CONTENT" ]] && printf "$STAMP\nGenerated content for the following vSphere ESXi components.\n" >> ${OUT_DIR}/._Documentation.meta || true

for ISO_NAME in "${SPECIFIC_TARGETS[@]}"
do
  DIR_NAME="${ISO_NAMES[$ISO_NAME]}"

  if [[ ! $DIR_NAME ]]; then
    MISSING_ISOS["$ISO_NAME"]=true
    continue
  fi
  if [[ ! ${ISO_MAP[$ISO_NAME]} ]]; then
    MISSING_ISOS["$ISO_NAME"]=true
    continue
  fi

  ISO="$DIR_NAME/$ISO_NAME"
  echo ">>> Building ISO content for: $ISO"
  [[ -z "$GENERATE" ]] && get_iso_meta || true
  print_iso_meta
  get_iso_sha
  build_title
  [[ $MODE = isomount ]] && mount_iso || true
  build_bootenv
  build_bootcfg
  [[ "$CONTENT" ]] && build_stage || true
  [[ "$CONTENT" ]] && build_workflow || true
  [[ "$CONTENT" ]] && printf "\nTITLE: $TITLE\nISO_NAME: $ISO_NAME.\n" >> ${OUT_DIR}/._Documentation.meta || true
  [[ $MODE = isomount ]] && { "${UNMOUNT[@]}" > /dev/null; } || true
  echo "  COMPLETED :: $ISO"
  echo ""
done

[[ "$CONTENT" ]] && build_meta

if [[ -n "$GENERATED" ]]
then
  for ISO_NAME in "${!MISSING_ISOS[@]}"; do
    echo "Failed to handle vmware $ISO_NAME: could not find ISO file"
  done
  for ISO_NAME in "${!MISSING_ISOS[@]}"; do
    echo "Failed to handle vmware $ISO_NAME: no entry in ISO_MAP"
  done
fi

cd $OUT_DIR
BUNDLE="$OUT_DIR/vmware-$TITLE.yaml"
DRPCLI=$(which drpcli) || true
if [[ "$DO_BUNDLE" ]]
then
  if [[ -n "$DRPCLI" ]]
  then
    echo "Running 'drpcli' bundle operation ... "
    drpcli contents bundle $BUNDLE
    [[ ! -r "$BUNDLE" ]] && xiterr 1 "Unable to read bundle file '$BUNDLE'"
  else
    echo "No 'drpcli' binary found in PATH ('$PATH')"
    echo "Not running 'drpcli contents bundle...' operation."
  fi
fi

if [[ "$DO_UPLOAD" ]]
then
  if [[ -n "$DRPCLI" ]]
  then
    [[ ! -r "$BUNDLE" ]] && xiterr 1 "Unable to read bundle file '$BUNDLE'"
    echo "Running 'drpcli' upload operation ... "
    drpcli contents upload $BUNDLE
    echo ""
    echo "WARNING:  If you are doing iterative runs of this script, you must remove"
    echo "          previous content packs that were uploaded - each content pack gets"
    echo "          a unique name to allow multiple content pack creations."
    echo ""
    echo "          TO REMOVE:  drpcli contents destroy vmware-generated-$STAMP"
    echo ""
  else
    echo "No 'drpcli' binary found in PATH ('$PATH')"
    echo "Not running 'drpcli contents upload...' operation."
  fi
fi
