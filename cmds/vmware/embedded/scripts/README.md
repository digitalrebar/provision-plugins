# Instructions

## To build provision-plugins/vmware using a custom iso

1. Vibs are no longer needed when dealing with 7, now you need only the software depot, also referred to as an offline bundle and more importantly a "component".
2. The hexdk or dsdk need to be installed on a CentOS 7 machine. Hexdk is smaller and provides all needed tools.

3. Update DRPY content then use the build vib scripts.
    
    1. The files this outputs are no longer needed. Just zip/tar/scp up the output dir from the build.
    
    2. For the Firewall content that is vmware_tools/firewall/stage
    
    3. For DRPY it would be vmware_tools/drpy/stage
    
    4. You need all of the content in those from after the build vibs has run.
    
4. Windows is required to build the iso, sorry it just is
5. To build ANY iso you need to obtain the offline bundle of the ISO. You will not be able to build a new ISO from the old ISO.


## From the drpy development box
To build the iso first make all changes to drpy 
next run the ./build_vibs.sh
move the content mentioned above in Step 3 to the CentOS 7 build box to build the components.
move over the vmware_tools/build_tools/build_components.sh file to the build box
In the example below Ill be putting things into /root/vibs (the current build script may be hard coded to expect this path)
    
    scp -r vmware_tools/firewall/stage root@hexdk-box:/root/vibs/firewall
    scp -r vmware_tools/drpy/stage root@hexdk-box:/root/vibs/drpy
    scp vmware_tools/build_tools/build_components.sh root@hexdk-box:/root/vibs
    
## From the HEXDK Build Box
Next its time to build the component.
    
    ssh root@hexdk-box
    cd /root/vibs

Verify that the `<version>` is set properly in the `descriptor.xml` The `descriptor.xml` is located in the stage dir for the vib. There is also a bulletin.xml as well. There is an element in the bulletin.xml called `<vibID>` The value of that should be literally: `<vibID/>` If it is any other value the build will not produce the expected results. 

    chmod +x build_components.sh
    ./build_components.sh

    No positional arguments specified

      Usage: build_components.sh [-h] [ -d VIB_PAYLOAD_VIB_DIR] [-c COMPONENT_BASENAME] [ -v VMW_VIB_NAME]

      VIB_PAYLOAD_VIB_DIR = Path to the staged descriptor.xml
      COMPONENT_BASENAME = Name to give the generated component
      VMW_VIB_NAME = Name to give the outputted vib file


    [root@dsdk-dev vibs]# ./build_components.sh -d /root/vibs/firewall/stage -c RackN-DRPY-Firewall -v firewall
    Successfully created /root/vibs/build/vibs/firewall.vib.
    Offline bundle creation succeeded
    /root/vibs/build/component/metadata.zip creation successful
    
    <version>v0.9.2-dev.1</version>  <--- example valid version number for the descriptor.xml

Once the components are built they are placed in `/root/vibs/build/component/COMPONENT_BASENAME.zip`
They are removed between builds so after you build one move it before building the next one. Once they 
are both built then you can move those components to the windows build machine to build the iso (or these files are what get submitted to VMWare to be signed).

## From The Windows Build Machine
On the windows machine you need to have powershell, and powercli from vmware. Its fine to use your normal user. Admin privs were not needed in my testing. The powercli script to build the iso is part of the vmware plugin. 

    PS C:\Users\errr> Set-PowerCLIConfiguration -Scope User -ParticipateInCEIP $false # only needed the first time you use powercli
    PS C:\Users\errr> mkdir rackn
    PS C:\Users\errr> cd rackn
    PS C:\Users\errr\rackn> Invoke-Webrequest -Uri http://RS_ENDPOINT:8091/files/plugin_providers/vmware/scripts/build_iso.ps1 -Outfile build_iso.ps1
    PS C:\Users\errr\rackn> 

### Screen pops up at the moment, select the first option to build the profile with no tools (or which ever profile you need)
Once that is done a command prompt should be returned, but sometimes it doesnt so after 30-60 seconds if you dont have a prompt hit enter and it should show up if its done if not just wait a bit more. the process takes under 1 min in my  case.
Note: If you get an error building the ISO it will probably be related to the vibs inside the component being unsigned. If that happens and you just need to test building the iso to run it before vmware signs everything you will have to go manually edit the descriptor.xml and the bulletin.xml and make sure they are at "community" on the acceptance level, and a -f may have to be added to the build_components.sh script to force it to do things and rebuild them and try to iso build again.

### Build a content pack from the iso
using the make-esxi-content-pack.sh generate a content pack using the iso created above, then upload that content pack and iso to drp. I cloned the esxi workflow then instead of having it select which iso to run I replaced that stage with the install stage from my new content pack.
