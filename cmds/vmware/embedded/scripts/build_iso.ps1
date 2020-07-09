<#
.SYNOPSIS
    Script to assist with building an ISO with the RackN Components.
.DESCRIPTION
    Script to build ISOs with drpy agent embeded.
.PARAMETER exportpath
    Full path to where you want the ISO exported. If it does not exist it will
    be created if it has permssion.
.PARAMETER rackNolbDir
    A full path to the required offline bundles.
.PARAMETER esxiOlbDir
    Full path to all the ESXi Offline bundles that should be built with the rackNolbDir bundles.
.EXAMPLE
    C:\PS> \build-iso.ps1 -exportpath C:\temp\isos\7-0\ -rackNolbDir C:\temp\olbs\rkn-7-0\ -esxiOlbDir C:\temp\olbs\7-0\
.NOTES
    Author: RackN
    Date: 06-2020
#>

param(
    [Parameter(Mandatory = $true, HelpMessage = "Full Path to where you want the ISO exported")]
    [string]$exportpath,
    [Parameter(Mandatory = $true, HelpMessage = "Full path to parent dir of RackN OLB for Version of ESXi you want to build. Ex: C:\temp\olbs\rkn\6x\")]
    [String]$rackNolbDir,
    [Parameter(Mandatory = $true, HelpMessage = "Full path to the parent dir of ESXi Offline Bundles to be built. Ex: c:\temp\olbs\vmw\6.7\")]
    [String]$esxiOlbDir
)

If (!(test-path $exportpath)) {
    New-Item -ItemType Directory -Force -Path $exportpath
}

$rkn_olbs = Get-ChildItem -Path $rackNolbDir -Filter *.zip | ForEach-Object { $_.FullName }
$esxi_olbs = Get-ChildItem -Path $esxiOlbDir -Filter *.zip | ForEach-Object { $_.FullName }

#clean the decks
Get-EsxSoftwareDepot | Remove-EsxSoftwareDepot
#add the depot
ForEach ($esxi_olb in $esxi_olbs) {
    # Now for each OLB we need to build an ISO from every profile
    Write-Output "Adding $esxi_olb"
    Add-EsxSoftwareDepot  $esxi_olb
    ForEach ($profile in Get-EsxImageProfile) {
        $imgprofilename = "RKN-$($profile.Name)"
        $baseprofile = $profile

        #create the image profile

        New-EsxImageProfile -CloneProfile $baseprofile.Name -Name $imgprofilename -Vendor $baseprofile.vendor

        # Add in the RKN OLBs
        # This is a grouping of all the vibs for the install
        ForEach ($rkn_olb in $rkn_olbs) {
            Add-EsxSoftwareDepot $rkn_olb
        }
        #add in the DRPY Agent & Firewall
        Add-EsxSoftwarePackage -ImageProfile $imgprofilename -SoftwarePackage DRP-Firewall-Rule
        Add-EsxSoftwarePackage -ImageProfile $imgprofilename -SoftwarePackage DRP-Agent
        # get all profiles that are now loaded that do not
        # include the vendor and our new one
        $allImageProfiles = Get-EsxImageProfile | Where-Object { ($_.Name -ne $baseprofile.Name) -and ($_.Name -ne $imgprofilename) -and ($_.Name -match "-no-tools")}
        # cycle through the image profiles
        # collect new packages and add to baseprofile
        $allImageProfiles | ForEach-Object {
            $thisprofile = $_
            $delta = Compare-EsxImageProfile -ReferenceProfile $baseprofile.Name -ComparisonProfile $thisprofile.Name
            $deltaupdates = $delta | Select-Object -ExpandProperty UpgradeFromRef #upgrade vibs
            $deltaupdates += $delta | Select-Object -ExpandProperty OnlyInComp #new vibs
            if ($deltaupdates) {
                foreach ($d in $deltaupdates) {
                    $pkg = Get-EsxSoftwarePackage | Where-Object { $_.Guid -eq $d }
                    Write-Verbose "Adding $pkg to $imgprofilename" -Verbose
                    Add-EsxSoftwarePackage -ImageProfile $imgprofilename -SoftwarePackage $pkg
                }
            }
        }
        Set-EsxImageProfile -ImageProfile $imgprofilename
        #Export ISO + Bundle
        Export-EsxImageProfile -ImageProfile $imgprofilename -ExportToBundle -FilePath $($exportpath + "/" + $imgprofilename + ".zip")
        Export-EsxImageProfile -ImageProfile $imgprofilename -ExportToISO -FilePath $($exportpath + "/" + $imgprofilename + ".iso")
    }
    Get-EsxSoftwareDepot | Remove-EsxSoftwareDepot
}

#clone a profile from the vendor image as a base to work from