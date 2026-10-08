#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
[CmdletBinding()]
param([switch]$Race)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $repo
$go = "$repo/.deps/go/bin/go.exe"
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:GOROOT = "$repo/.deps/go"
$env:GOPATH = "$repo/.deps/gopath"
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force -Path "$repo/.artifacts/v1" | Out-Null
Start-Transcript -LiteralPath "$repo/.artifacts/v1/validation.txt" -Force | Out-Null
try {
    & $go test -vet=off ./bootstrap ./dnsproxy ./doh ./dohruntime ./product ./conf ./manager ./protocol ./tunnel ./tunnel/firewall -count=1 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Regression suite failed' }
    & $go test -tags wirehush_v1 -vet=off ./manager ./cmd/wirehush-manager -count=1 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Production manager suite failed' }
    $dependencies = & $go list -tags wirehush_v1,load_wgnt_from_rsrc -deps ./cmd/wirehush-manager
    if ($LASTEXITCODE) { throw 'Production dependency inventory failed' }
    if ($dependencies | Where-Object { $_ -eq 'encoding/gob' -or $_ -match '(^github.com/lxn/walk($|/))|(/windows/(ui|updater)$)' }) { throw 'Legacy IPC, UI, or updater entered the production backend' }
    # The inherited interactive ringlogger TestFollow deliberately never exits.
    & $go test -vet=off ./ringlogger -run TestWireHush -count=1 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Protected logging suite failed' }
    & $go test -overlay .overlay/overlay.json ./doh -run TestTLSClientHelloDoesNotPanic -count=1 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Overlay TLS regression failed' }
    if ($Race) {
        $env:CGO_ENABLED = '1'
        $env:CC = "$repo/.deps/bin/x86_64-w64-mingw32-gcc.exe"
        & $go test -race -vet=off ./conf ./manager ./dohruntime ./bootstrap -count=1 2>&1 | Out-Host
        if ($LASTEXITCODE) { throw 'Race suite failed' }
        & $go test -tags wirehush_v1 -race -vet=off ./manager -count=1 2>&1 | Out-Host
        if ($LASTEXITCODE) { throw 'Production manager race suite failed' }
    }
    & "$repo/.deps/bin/x86_64-w64-mingw32-gcc.exe" -O2 -Wall -Wextra -Werror -municode -DUNICODE -D_UNICODE -DWINVER=0x0A00 -D_WIN32_WINNT=0x0A00 -o "$repo/.artifacts/v1/installer-safety-test.exe" installer/v1-customactions-test.c -lmsi -ladvapi32 -lnetapi32 -lshell32 -lole32 -luuid 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Installer safety test compilation failed' }
    & "$repo/.artifacts/v1/installer-safety-test.exe" 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Installer safety tests failed' }
    $installerBuilder = Get-Content -LiteralPath "$repo/scripts/build-installer-v1.ps1" -Raw
    $buildSearch = '<Property Id="WINDOWSBUILDNUMBER" Secure="yes"><RegistrySearch Id="BuildNumberSearch" Root="HKLM" Key="SOFTWARE\Microsoft\Windows NT\CurrentVersion" Name="CurrentBuildNumber" Type="raw" Win64="yes" /></Property>'
    $buildCondition = '<Condition Message="WireHush requires Windows 10 1809 or later.">Installed OR (VersionNT64 AND WINDOWSBUILDNUMBER AND WINDOWSBUILDNUMBER &gt;= 17763)</Condition>'
    if (!$installerBuilder.Contains($buildSearch) -or !$installerBuilder.Contains($buildCondition)) { throw 'Installer Windows build detection regression' }
    if ($installerBuilder -match '\bWindowsBuild\b') { throw 'Legacy WindowsBuild MSI property must not gate V1 installation' }
    if (!$installerBuilder.Contains('<UI Id="WireHush_FixedPath">') -or !$installerBuilder.Contains('<UIRef Id="WixUI_Common" />') -or !$installerBuilder.Contains('<Dialog Id="RemoveDataChoiceDlg"')) { throw 'Installer fixed-path UI regression' }
    if ($installerBuilder -match 'WixUI_InstallDir|WIXUI_INSTALLDIR|InstallDirDlg') { throw 'V1 installer must not expose a mutable installation path' }
    $buildScript = Get-Content -LiteralPath "$repo/scripts/build-v1.ps1" -Raw
    if (!$buildScript.Contains('git status --porcelain --untracked-files=normal') -or !$buildScript.Contains('Owner-test builds require a clean Git working tree')) { throw 'Artifact provenance clean-tree guard regression' }
    & "$repo/.deps/dotnet/dotnet.exe" run --project windows-ui/WireHush.UI.SafetyTests -c Release 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'UI measurement safety tests failed' }
    & git diff --check 2>&1 | Out-Host
    if ($LASTEXITCODE) { throw 'Diff check failed' }
    Write-Host 'PASS: V1 automated validation. Privileged and real-network acceptance remains separate.'
} finally { Stop-Transcript | Out-Null }
