#!/usr/bin/env bash
# This script is responsible for building a tarball that contains
# all the tooling required to manage BIOS and flash support on hardware
# platforms natively supported by the bios and flash plugins and content.

# Note that this does not build in utilities required to support RAID controllers,
# as we generally take a system vendor independent approach to that.

# Dell support comes from the Linux DSU repos.
# Pin to the last DSU block release that had OMSA 9.5.x in it.
# We need to test 10.x before making it generally available.
# To float and pull in the latest OMSA bits always, set this to 'dsu'
set -e
DSU_RELEASE='DSU_21.06.01'
CREATEREPO="createrepo"

if which createrepo_c &>/dev/null; then
  CREATEREPO="createrepo_c"
fi

RH7_RPMS=(
    # HPE tools we need.
    'https://downloads.linux.hpe.com/SDR/repo/hpsum/rhel/7/x86_64/current/sum-8.8.0-39.rhel7.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/ilorest/rhel/7/x86_64/current/ilorest-3.2.2-32.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/7/x86_64/current/hponcfg-5.6.0-0.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/7/x86_64/current/ssacli-5.10-44.0.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/7/x86_64/current/sut-2.8.0-26.linux.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/stk/rhel/7/x86_64/current/hp-scripting-tools-11.40-9.rhel7.x86_64.rpm'
    # Extra dependencies required for tooling to function.
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/libxml2-python-2.9.1-6.el7.5.x86_64.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/libxslt-1.1.28-6.el7.x86_64.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/net-tools-2.0-0.25.20131004git.el7.x86_64.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/python-chardet-2.2.1-3.el7.noarch.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/python-kitchen-1.1.1-5.el7.noarch.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/yum-plugin-ovl-1.1.31-54.el7_8.noarch.rpm'
    'http://mirror.centos.org/centos/7/os/x86_64/Packages/yum-utils-1.1.31-54.el7_8.noarch.rpm'
)
RH8_RPMS=(
    # HPE tools we need.
    'https://downloads.linux.hpe.com/SDR/repo/hpsum/rhel/8/x86_64/current/sum-8.8.0-39.rhel8.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/ilorest/rhel/8/x86_64/current/ilorest-3.2.2-32.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/8/x86_64/current/hponcfg-5.6.0-0.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/8/x86_64/current/ssacli-5.10-44.0.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/spp-gen10/redhat/8/x86_64/current/sut-2.8.0-26.linux.x86_64.rpm'
    'https://downloads.linux.hpe.com/SDR/repo/stk/rhel/8/x86_64/current/hp-scripting-tools-11.40-13.rhel8.x86_64.rpm'
    # Extra dependencies
    'http://mirrors.kernel.org/fedora-epel/8/Everything/x86_64/Packages/d/dnf-plugin-ovl-0.0.3-1.el8.noarch.rpm'
    'http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/dnf-plugins-core-4.0.18-4.el8.noarch.rpm'
    'http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/libxslt-1.1.32-6.el8.x86_64.rpm'
    'http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/net-tools-2.0-0.52.20160912git.el8.x86_64.rpm'
    'http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/yum-utils-4.0.18-4.el8.noarch.rpm'
)

# Empty for now. but more stuff may be added later.
OTHER_RPMS=()

# Mostly OneCLI versions for support of various Lenovo servers.
OTHER_FILES=(
    'https://download.lenovo.com/servers/mig/2021/06/16/54094/lnvgy_utl_lxce_onecli01n-3.2.0_rhel_x86-64.tgz'
    'https://download.lenovo.com/servers/mig/2021/05/26/53954/lnvgy_utl_lxce_onecli01l-3.1.2_rhel_x86-64.tgz'
    'https://download.lenovo.com/servers/mig/2019/04/10/19873/lnvgy_utl_lxce_onecli01v-2.5.0_rhel_x86-64.tgz'
)

# Utilities that are not available as direct downloads.
declare -A CLICKTHRU_FILES
CLICKTHRU_FILES['sum_2.5.2_Linux_x86_64_20210112.tar.gz']='https://www.supermicro.com/SwDownload/UserInfo.aspx?sw=0&cat=SUM'

rm -rf hw_repo || :
mkdir -p hw_repo/os_dependent/RHEL7_64 hw_repo/os_dependent/RHEL8_64 hw_repo/os_independent

pull_rpms_for() (
    cd "$1"
    dsubase="${1#hw_repo/}"
    case $dsubase in
        os_dependent*)
            wget -nd -nH -r --no-parent "https://linux.dell.com/repo/hardware/${DSU_RELEASE}/${dsubase}/metaRPMS/"
            wget -nd -nH -r --no-parent "https://linux.dell.com/repo/hardware/${DSU_RELEASE}/${dsubase}/srvadmin/"
            ;;
        os_independent*)
            wget -nd -nH -r --no-parent "https://linux.dell.com/repo/hardware/${DSU_RELEASE}/os_independent/x86_64/"
            ;;
    esac
    rm -f index.* robots.txt* || :
    shift
    for i in "$@"; do
        curl -fgLO "$i"
    done
    $CREATEREPO .
)

pull_rpms_for hw_repo/os_dependent/RHEL7_64 "${RH7_RPMS[@]}"
pull_rpms_for hw_repo/os_dependent/RHEL8_64 "${RH8_RPMS[@]}"
pull_rpms_for hw_repo/os_independent "${OTHER_RPMS[@]}"

(
  cd hw_repo
  for i in "${OTHER_FILES[@]}"; do
    curl -fgLO "$i"
  done
)

for i in "${!CLICKTHRU_FILES[@]}"; do
    while [[ ! -f hw_repo/$i ]]; do
        echo "Please download $i from ${CLICKTHRU_FILES[$i]} into ${PWD}/hw_repo"
        read -p "Press Enter when done." throwaway
    done
done

tar -zcvf hw_repo.tgz hw_repo
