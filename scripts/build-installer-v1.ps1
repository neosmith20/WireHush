#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
[CmdletBinding()]
param([Parameter(Mandatory)][ValidateSet('x64','arm64')][string]$Architecture,
      [Parameter(Mandatory)][string]$Payload, [Parameter(Mandatory)][string]$Version,
      [switch]$SkipIce)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$Payload = (Resolve-Path -LiteralPath $Payload).Path
if (!$Payload.StartsWith((Join-Path $repo '.artifacts/v1'), [StringComparison]::OrdinalIgnoreCase)) { throw 'Installer payload must be a V1 build output' }
$out = Split-Path $Payload
$prefix = if ($Architecture -eq 'x64') {'x86_64'} else {'aarch64'}
function Checked($exe, [string[]]$arguments) { & $exe @arguments; if ($LASTEXITCODE) { throw "Installer build failed: $LASTEXITCODE" } }
Checked "$repo/.deps/bin/$prefix-w64-mingw32-gcc.exe" @('-O2','-Wall','-Wextra','-Werror','-shared','-DUNICODE','-D_UNICODE','-DWINVER=0x0A00','-D_WIN32_WINNT=0x0A00','-Wl,--export-all-symbols', '-o', "$out/v1-customactions.dll", "$repo/installer/v1-customactions.c", '-lmsi','-ladvapi32','-lnetapi32','-lshell32','-lole32','-luuid')
function Id($prefix, $path) { $hash=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($path.ToLowerInvariant()))); return $prefix+$hash.Substring(0,24) }
function ComponentGuid($path) {
    $hash=[Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes("WireHush.V1/$Architecture/$($path.ToLowerInvariant())"))
    $bytes=$hash[0..15]; $bytes[7]=($bytes[7] -band 15) -bor 80; $bytes[8]=($bytes[8] -band 63) -bor 128
    return [guid]::new([byte[]]$bytes).ToString()
}
function Esc($text) { return [Security.SecurityElement]::Escape($text) }
$components=[Collections.Generic.List[string]]::new()
function DirectoryXml($path, $relative, $directoryId, $name) {
    $xml='<Directory Id="'+$directoryId+'" Name="'+(Esc $name)+'">'
    foreach ($file in Get-ChildItem -LiteralPath $path -File | Sort-Object Name) {
        $rel=if($relative){"$relative/$($file.Name)"}else{$file.Name}
        $component=Id 'C' $rel; $components.Add($component)
        $fileId=if($rel -eq 'WireHush-Manager.exe'){'ManagerExe'}elseif($rel -eq 'WireHush.exe'){'UiExe'}else{Id 'F' $rel}
        $xml+='<Component Id="'+$component+'" Guid="'+(ComponentGuid $rel)+'" Win64="yes"><File Id="'+$fileId+'" Source="'+(Esc $file.FullName)+'" KeyPath="yes">'
        if($fileId -eq 'UiExe'){$xml+='<Shortcut Id="StartMenu" Directory="ProgramMenuFolder" Name="WireHush" WorkingDirectory="INSTALLFOLDER" Advertise="yes" Icon="ProductIcon" IconIndex="0" />'}
        $xml+='</File>'
        if($fileId -eq 'ManagerExe'){$xml+='<ServiceInstall Id="ManagerInstall" Name="WireHushManager" DisplayName="WireHush Manager" Type="ownProcess" Account="LocalSystem" Start="demand" ErrorControl="normal" Arguments="/managerservice" Vital="yes" /><ServiceControl Id="ManagerControl" Name="WireHushManager" Stop="both" Remove="uninstall" Wait="yes" />'}
        $xml+='</Component>'
    }
    foreach($sub in Get-ChildItem -LiteralPath $path -Directory | Sort-Object Name){
        $rel=if($relative){"$relative/$($sub.Name)"}else{$sub.Name}
        $xml+=DirectoryXml $sub.FullName $rel (Id 'D' $rel) $sub.Name
    }
    return $xml+'</Directory>'
}
$directory=DirectoryXml $Payload '' 'INSTALLFOLDER' 'WireHush'
$references=($components | ForEach-Object {'<ComponentRef Id="'+$_+'" />'}) -join ''
$upgrade=if($Architecture -eq 'x64'){'a9fd3e3c-7e5b-4d6d-9a6b-6d7f497ba1de'}else{'e3b4a4e6-8070-4655-9d64-d0d4b098d2f1'}
$source=@"
<?xml version="1.0" encoding="utf-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
<Product Id="*" Name="WireHush V1 Test Candidate" Language="1033" Version="$Version" Manufacturer="WireHush contributors" UpgradeCode="$upgrade">
<Package InstallerVersion="500" Compressed="yes" InstallScope="perMachine" Description="WireHush owner-test candidate" />
<MediaTemplate EmbedCab="yes" CompressionLevel="high" />
<MajorUpgrade AllowSameVersionUpgrades="yes" DowngradeErrorMessage="A newer WireHush version is installed." Schedule="afterInstallExecute" />
<Property Id="DELETE_WIREHUSH_DATA" Value="0" Secure="yes" />
<Property Id="BeginMaintenance" Hidden="yes" /><Property Id="RollbackMaintenance" Hidden="yes" /><Property Id="ProvisionAccess" Hidden="yes" /><Property Id="CommitProvisionAccess" Hidden="yes" />
<Icon Id="ProductIcon" SourceFile="$(Esc "$repo/windows-ui/WireHush.UI/Assets/WireHush.ico")" /><Property Id="ARPPRODUCTICON" Value="ProductIcon" />
<Property Id="WINDOWSBUILDNUMBER" Secure="yes"><RegistrySearch Id="BuildNumberSearch" Root="HKLM" Key="SOFTWARE\Microsoft\Windows NT\CurrentVersion" Name="CurrentBuildNumber" Type="raw" Win64="yes" /></Property>
<Condition Message="WireHush requires Windows 10 1809 or later.">Installed OR (VersionNT64 AND WINDOWSBUILDNUMBER AND WINDOWSBUILDNUMBER &gt;= 17763)</Condition>
<Directory Id="TARGETDIR" Name="SourceDir"><Directory Id="ProgramFiles64Folder">$directory</Directory><Directory Id="ProgramMenuFolder" /></Directory>
<Feature Id="Main" Title="WireHush" Level="1">$references</Feature>
<Binary Id="InstallerActions" SourceFile="$(Esc "$out/v1-customactions.dll")" />
<Binary Id="InstallerManager" SourceFile="$(Esc "$Payload/WireHush-Manager.exe")" />
<CustomAction Id="CaptureMaintenance" BinaryKey="InstallerActions" DllEntry="CaptureMaintenance" Return="check" />
<CustomAction Id="RollbackMaintenance" BinaryKey="InstallerActions" DllEntry="RollbackMaintenance" Execute="rollback" Impersonate="no" HideTarget="yes" Return="check" />
<CustomAction Id="BeginMaintenance" BinaryKey="InstallerActions" DllEntry="BeginMaintenance" Execute="deferred" Impersonate="no" HideTarget="yes" Return="check" />
<CustomAction Id="OwnershipPreflight" BinaryKey="InstallerManager" ExeCommand="/installerpreflight" Execute="deferred" Impersonate="no" Return="check" />
<CustomAction Id="ShutdownOwned" BinaryKey="InstallerManager" ExeCommand="/shutdownownedservices" Execute="deferred" Impersonate="no" Return="check" />
<CustomAction Id="MigrateLegacy" BinaryKey="InstallerManager" ExeCommand="/migratelegacy" Execute="deferred" Impersonate="no" Return="check" />
<CustomAction Id="ProvisionAccessData" Property="ProvisionAccess" Value="[UserSID]" HideTarget="yes" />
<CustomAction Id="ProvisionAccess" BinaryKey="InstallerActions" DllEntry="ProvisionAccess" Execute="deferred" Impersonate="no" HideTarget="yes" Return="check" />
<CustomAction Id="CommitProvisionAccessData" Property="CommitProvisionAccess" Value="[UserSID]" HideTarget="yes" />
<CustomAction Id="CommitProvisionAccess" BinaryKey="InstallerActions" DllEntry="ProvisionAccess" Execute="commit" Impersonate="no" HideTarget="yes" Return="check" />
<CustomAction Id="FinalizeLegacy" FileKey="ManagerExe" ExeCommand="/finalizelegacy" Execute="commit" Impersonate="no" Return="check" />
<CustomAction Id="CommitMaintenance" BinaryKey="InstallerActions" DllEntry="CommitMaintenance" Execute="commit" Impersonate="no" Return="check" />
<CustomAction Id="DeleteOwnedData" BinaryKey="InstallerActions" DllEntry="DeleteOwnedData" Execute="deferred" Impersonate="no" Return="check" />
<InstallExecuteSequence>
 <Custom Action="CaptureMaintenance" Before="InstallInitialize">NOT UPGRADINGPRODUCTCODE</Custom>
 <Custom Action="RollbackMaintenance" After="InstallInitialize">NOT UPGRADINGPRODUCTCODE</Custom>
 <Custom Action="OwnershipPreflight" After="RollbackMaintenance">NOT UPGRADINGPRODUCTCODE</Custom>
 <Custom Action="BeginMaintenance" After="OwnershipPreflight">NOT UPGRADINGPRODUCTCODE</Custom>
 <Custom Action="ShutdownOwned" After="BeginMaintenance">NOT UPGRADINGPRODUCTCODE</Custom>
 <Custom Action="MigrateLegacy" After="ShutdownOwned">NOT REMOVE AND NOT UPGRADINGPRODUCTCODE</Custom>

 <Custom Action="ProvisionAccessData" Before="ProvisionAccess">NOT REMOVE</Custom>
 <Custom Action="ProvisionAccess" After="InstallServices">NOT REMOVE</Custom>
 <Custom Action="DeleteOwnedData" Before="RemoveFiles">REMOVE="ALL" AND DELETE_WIREHUSH_DATA="1" AND UILevel &gt;= 3 AND NOT UPGRADINGPRODUCTCODE AND NOT WIX_UPGRADE_DETECTED</Custom>
 <Custom Action="CommitProvisionAccessData" Before="CommitProvisionAccess">NOT REMOVE</Custom>
 <Custom Action="CommitProvisionAccess" Before="FinalizeLegacy">NOT REMOVE</Custom>
 <Custom Action="FinalizeLegacy" Before="CommitMaintenance">NOT REMOVE</Custom>
 <Custom Action="CommitMaintenance" Before="InstallFinalize">NOT UPGRADINGPRODUCTCODE</Custom>
</InstallExecuteSequence>
<WixVariable Id="WixUILicenseRtf" Value="$(Esc "$repo/installer/v1-license.rtf")" />
<UI Id="WireHush_FixedPath">
 <TextStyle Id="WixUI_Font_Normal" FaceName="Tahoma" Size="8" />
 <TextStyle Id="WixUI_Font_Bigger" FaceName="Tahoma" Size="12" />
 <TextStyle Id="WixUI_Font_Title" FaceName="Tahoma" Size="9" Bold="yes" />
 <Property Id="DefaultUIFont" Value="WixUI_Font_Normal" />
 <Property Id="WixUI_Mode" Value="InstallDir" />
 <Property Id="ARPNOMODIFY" Value="1" />
 <DialogRef Id="ErrorDlg" />
 <DialogRef Id="FatalError" />
 <DialogRef Id="FilesInUse" />
 <DialogRef Id="MsiRMFilesInUse" />
 <DialogRef Id="PrepareDlg" />
 <DialogRef Id="ProgressDlg" />
 <DialogRef Id="ResumeDlg" />
 <DialogRef Id="UserExit" />
 <DialogRef Id="WelcomeDlg" />
 <DialogRef Id="LicenseAgreementDlg" />
 <DialogRef Id="VerifyReadyDlg" />
 <DialogRef Id="MaintenanceWelcomeDlg" />
 <DialogRef Id="MaintenanceTypeDlg" />
 <DialogRef Id="ExitDialog" />
 <Dialog Id="RemoveDataChoiceDlg" Width="370" Height="270" Title="Remove WireHush">
  <Control Id="Title" Type="Text" X="20" Y="20" Width="330" Height="30" Text="Keep your data unless you explicitly choose deletion." />
  <Control Id="Explanation" Type="Text" X="20" Y="60" Width="330" Height="70" Text="By default, uninstall keeps encrypted tunnel records, settings, migration backups, logs, and UI preferences. Checking the box deletes WireHush data for all local users. Deletion cannot be rolled back. Close every WireHush window before continuing." />
  <Control Id="Delete" Type="CheckBox" X="20" Y="140" Width="330" Height="40" Property="DELETE_WIREHUSH_DATA" CheckBoxValue="1" Text="Delete all WireHush data permanently" />
  <Control Id="Back" Type="PushButton" X="180" Y="240" Width="56" Height="17" Text="Back"><Publish Event="NewDialog" Value="MaintenanceTypeDlg">1</Publish></Control>
  <Control Id="Next" Type="PushButton" X="236" Y="240" Width="56" Height="17" Default="yes" Text="Next"><Publish Event="NewDialog" Value="VerifyReadyDlg">1</Publish></Control>
  <Control Id="Cancel" Type="PushButton" X="304" Y="240" Width="56" Height="17" Cancel="yes" Text="Cancel"><Publish Event="SpawnDialog" Value="CancelDlg">1</Publish></Control>
 </Dialog>
 <Publish Dialog="ExitDialog" Control="Finish" Event="EndDialog" Value="Return" Order="999">1</Publish>
 <Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="LicenseAgreementDlg">NOT Installed</Publish>
 <Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="VerifyReadyDlg">Installed AND PATCH</Publish>
 <Publish Dialog="LicenseAgreementDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg">1</Publish>
 <Publish Dialog="LicenseAgreementDlg" Control="Next" Event="NewDialog" Value="VerifyReadyDlg">LicenseAccepted = "1"</Publish>
 <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="LicenseAgreementDlg" Order="1">NOT Installed</Publish>
 <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="RemoveDataChoiceDlg" Order="1">Installed AND WixUI_InstallMode = "Remove"</Publish>
 <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="MaintenanceTypeDlg" Order="2">Installed AND NOT PATCH AND WixUI_InstallMode &lt;&gt; "Remove"</Publish>
 <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg" Order="3">Installed AND PATCH</Publish>
 <Publish Dialog="MaintenanceWelcomeDlg" Control="Next" Event="NewDialog" Value="MaintenanceTypeDlg">1</Publish>
 <Publish Dialog="MaintenanceTypeDlg" Control="RepairButton" Event="NewDialog" Value="VerifyReadyDlg">1</Publish>
 <Publish Dialog="MaintenanceTypeDlg" Control="RemoveButton" Event="NewDialog" Value="RemoveDataChoiceDlg">1</Publish>
 <Publish Dialog="MaintenanceTypeDlg" Control="Back" Event="NewDialog" Value="MaintenanceWelcomeDlg">1</Publish>
</UI>
<UIRef Id="WixUI_Common" />
</Product></Wix>
"@
$wxs=Join-Path $out 'wirehush-v1.wxs'
$source | Set-Content -LiteralPath $wxs -Encoding utf8
$wix="$repo/installer/.deps/wix/bin"
Checked "$wix/candle.exe" @('-nologo','-arch',$Architecture,'-ext','WixUIExtension','-out',"$out/wirehush-v1.wixobj",$wxs)
$msi="$out/WireHush-$Version-$Architecture-test.msi"
$link=@('-nologo','-ext','WixUIExtension','-spdb','-sice:ICE61','-sice:ICE03','-out',$msi,"$out/wirehush-v1.wixobj")
if($SkipIce){$link+= '-sval'}
Checked "$wix/light.exe" $link
Write-Host "Built unsigned owner-test installer: $msi"
