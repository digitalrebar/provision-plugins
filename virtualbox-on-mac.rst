
To test full lifecycle on mac with virtualbox, you will need to do the following things.


1. Create a virtualbox machine with two nics.
   a. First nic is on a host-only network.
   b. Second nic is on a NAT network.
   c. Disk size at least 50G (to test windows)
   d. Set box to pxe boot always and then disk
2. Clone step one (make sure nets get new MACs).
3. Edit the prefs for the hostonly-network and turn off the DHCP server.
4. From terminal, install DRP tip.
   a. mkdir drp-test
   b. cd drp-test
   c. curl -fsSL https://raw.githubusercontent.com/digitalrebar/provision/stable/tools/install.sh | bash -s -- --isolated --drp-version=tip install
   d. Be sure to run: sudo route add 255.255.255.255 <IP of hostonly-network for the host>
      e.g. sudo route add 255.255.255.255 192.168.100.1
   e. Remember the start command but don't run it:
      sudo ./dr-provision <IP of hostonly-nework for the host> --base-root=`pwd`/drp-data --local-content=\"\" --default-content=\"\"
5. Put files in place.
   a. mkdir -p drp-data/tftpboot/isos
   b. cp sledgehammer-8f81f9981342cc1651e1e410fed1aef1cdbf29fe.tar curtin_windows2k16_packer-windows_undefined_bf71162.tar.gz and the centos packer image you already have into this directory.
   c. mkdir -p drp-data/saas-content
   d. cp all the yaml files from the saas-content directory into here.
   e. mkdir -p drp-data/plugins
   f. unzip the plugins for your platform - drp-rackn-plugins-darwin-amd64.zip - here

6. At this point, you should be able to start DRP.
   a. https://127.0.0.1:8092 from the browser will allow you to use and see the new UI.
7. Create the hostonly network in DRP.
   a. Taking the default should work.

8. Set the default prefs.
   a. Change the defaultStage to discover

9. Boot a machine in virtual box.
   a. It should be discovered and power off.

10. Create a plugin to manage virtualbox.
    a. get the user you run virtualbox as (most likely running: id from termal will get it.
       e.g. id => uid=501(galthaus) gid=20(staff) groups=20(staff),701(com.apple.sharepoint.group.1),12(everyone),61(localaccounts),79(_appserverusr),80(admin),81(_appserveradm),98(_lpadmin),399(com.apple.access_ssh),33(_appstore),100(_lpoperator),204(_developer),395(com.apple.access_ftp),398(com.apple.access_screensharing)
       it would be galthaus
    b. drpcli plugins create '{ "Name": "virtualbox-ipmi", "Provider": "virtualbox-ipmi", "Params": { "virtualbox/user": "galthaus" } }'

11. Set up terraform.
    a. unzip the terraform and get the platform one and move it your terraform play area.

12. Update tf file to look like the packet.tf file.









