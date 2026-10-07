#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
[CmdletBinding()]
param([ValidateSet('x64', 'arm64', 'all')][string]$Architecture = 'all', [switch]$SkipBootstrap, [switch]$Installer)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $repo
if (!$SkipBootstrap) { & "$PSScriptRoot/bootstrap-v1.ps1" }
$versionText = Get-Content -LiteralPath "$repo/version/version.go" -Raw
if ($versionText -notmatch 'Number\s*=\s*"(\d+\.\d+\.\d+)"') { throw 'Invalid product version' }
$version = $Matches[1]
$deps = Join-Path $repo '.deps'
$go = "$deps/go/bin/go.exe"
$dotnet = "$deps/dotnet/dotnet.exe"
function Invoke-Checked($Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Build operation failed with exit code $LASTEXITCODE" }
}
$icon = "$repo/windows-ui/WireHush.UI/Assets/WireHush.ico"
Invoke-Checked "$deps/convert.exe" @("$repo/windows-ui/WireHush.UI/Assets/WireHush_Icon.png", '-define', 'icon:auto-resize=256,128,96,64,48,32,24,16', $icon)
$env:GOOS = 'windows'
$env:CGO_ENABLED = '0'
$env:GOROOT = "$deps/go"
$env:GOPATH = "$deps/gopath"
$targets = if ($Architecture -eq 'all') { @('x64', 'arm64') } else { @($Architecture) }
foreach ($target in $targets) {
    $goArch = if ($target -eq 'x64') { 'amd64' } else { 'arm64' }
    $prefix = if ($target -eq 'x64') { 'x86_64' } else { 'aarch64' }
    $env:GOARCH = $goArch
    # A unique payload folder avoids stale files from an earlier publish.
    $run = [guid]::NewGuid().ToString('N')
    $app = "$repo/.artifacts/v1/$target/$run/app"
    New-Item -ItemType Directory -Path $app -Force | Out-Null
    Invoke-Checked "$deps/bin/$prefix-w64-mingw32-windres.exe" @('-I', "$deps/wireguard-nt/bin/$goArch", '-I', "$repo/cmd/wirehush-manager", '-I', (Split-Path $icon), "-DWIREHUSH_VERSION_ARRAY=$($version.Replace('.', ',')),0", "-DWIREHUSH_VERSION=$version", '-i', "$repo/cmd/wirehush-manager/resources.rc", '-o', "$repo/cmd/wirehush-manager/resources_$goArch.syso", '-O', 'coff')
    Invoke-Checked $go @('build', '-overlay', '.overlay/overlay.json', '-tags', 'load_wgnt_from_rsrc', '-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-o', "$app/WireHush-Manager.exe", './cmd/wirehush-manager')
    Invoke-Checked $dotnet @('restore', 'windows-ui/WireHush.UI/WireHush.UI.csproj', "-p:Platform=$target", '--locked-mode', '--nologo')
Invoke-Checked $dotnet @('publish', 'windows-ui/WireHush.UI/WireHush.UI.csproj', '-c', 'Release', "-p:Platform=$target", '-r', "win-$target", "-p:Version=$version", '-p:DebugType=None', '-p:DebugSymbols=false', '--no-restore', '-o', $app, '--nologo')
    $notices = Join-Path $app 'Notices'
    New-Item -ItemType Directory -Path $notices -Force | Out-Null
    foreach ($name in @('LICENSE', 'WIREGUARD-COPYING', 'THIRD_PARTY_NOTICES.md')) {
        if (Test-Path -LiteralPath "$repo/$name") { Copy-Item -LiteralPath "$repo/$name" -Destination $notices }
    }
    Get-ChildItem -LiteralPath "$repo/licenses" -File | Copy-Item -Destination $notices
    Copy-Item -LiteralPath "$repo/GPL-2.0.txt" -Destination $notices
Copy-Item -LiteralPath "$deps/go/LICENSE" -Destination "$notices/go-runtime-LICENSE.txt"
if (Test-Path -LiteralPath "$deps/go/PATENTS") { Copy-Item -LiteralPath "$deps/go/PATENTS" -Destination "$notices/go-runtime-PATENTS.txt" }
    # Carry licenses/notices for the actual resolved NuGet and Go dependencies,
    # rather than relying on a manually maintained list of runtime DLL names.
    $nuget = if ($env:NUGET_PACKAGES) { $env:NUGET_PACKAGES } else { "$env:USERPROFILE/.nuget/packages" }
    foreach ($lock in @('windows-ui/WireHush.UI/packages.lock.json', 'protocol/csharp/packages.lock.json')) {
        $resolved = Get-Content -LiteralPath "$repo/$lock" -Raw | ConvertFrom-Json
        foreach ($framework in $resolved.dependencies.PSObject.Properties) {
            foreach ($package in $framework.Value.PSObject.Properties) {
                if (!$package.Value.resolved) { continue }
                $packageRoot = Join-Path $nuget "$($package.Name.ToLowerInvariant())/$($package.Value.resolved)"
                Get-ChildItem -LiteralPath $packageRoot -File | Where-Object Name -match '^(license|copying|notice|third.?party)' | ForEach-Object {
                    Copy-Item -LiteralPath $_.FullName -Destination "$notices/nuget-$($package.Name)-$($package.Value.resolved)-$($_.Name)" -Force
                }
            }
        }
    }
    $modules = & $go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' ./cmd/wirehush-manager
    if ($LASTEXITCODE) { throw 'Dependency license inventory failed' }
    foreach ($module in $modules | Where-Object { $_ } | Sort-Object -Unique) {
        $parts = $module.Split('|')
        Get-ChildItem -LiteralPath $parts[2] -File | Where-Object Name -match '^(license|copying|notice)' | ForEach-Object {
            $safeName = $parts[0].Replace('/', '_')
            Copy-Item -LiteralPath $_.FullName -Destination "$notices/go-$safeName-$($parts[1])-$($_.Name)" -Force
        }
    }
    Copy-Item -LiteralPath "$deps/wireguard-nt/LICENSE.txt" -Destination "$notices/wireguard-nt-LICENSE.txt"
    foreach ($name in @('LICENSE.txt', 'ThirdPartyNotices.txt')) {
        Copy-Item -LiteralPath "$deps/dotnet/$name" -Destination "$notices/dotnet-$name"
    }
    if (Get-ChildItem -LiteralPath $app -Recurse -File | Where-Object { $_.Extension -in @('.conf', '.dpapi', '.pdb', '.go', '.cs') }) { throw 'Unexpected source, configuration, or debug file in payload' }
    if ($target -eq 'x64') { Invoke-Checked "$app/WireHush-Manager.exe" @('/version') }
    $latest = "$repo/.artifacts/v1/$target/latest.json"
    @{ Architecture = $target; Version = $version; App = $app; Commit = (& git rev-parse HEAD) } | ConvertTo-Json | Set-Content -LiteralPath $latest -Encoding utf8
    if ($Installer) { & "$PSScriptRoot/build-installer-v1.ps1" -Architecture $target -Payload $app -Version $version }
    if ($Installer) {
        Get-ChildItem -LiteralPath (Split-Path $app) -Filter '*.msi' -File | ForEach-Object {
            '{0}  {1}' -f (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $_.Name
        } | Set-Content -LiteralPath (Join-Path (Split-Path $app) 'INSTALLER-SHA256SUMS.txt') -Encoding ascii
    }
    Get-ChildItem -LiteralPath $app -Recurse -File | Sort-Object FullName | ForEach-Object {
        '{0}  {1}' -f (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), [IO.Path]::GetRelativePath($app, $_.FullName)
    } | Set-Content -LiteralPath (Join-Path (Split-Path $app) 'SHA256SUMS.txt') -Encoding ascii
    Write-Host "Built $target owner-test payload: $app"
}
