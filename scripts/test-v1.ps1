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
    & $go test -vet=off ./bootstrap ./dnsproxy ./doh ./dohruntime ./product ./conf ./manager ./protocol ./tunnel ./tunnel/firewall -count=1
    if ($LASTEXITCODE) { throw 'Regression suite failed' }
    # The inherited interactive ringlogger TestFollow deliberately never exits.
    & $go test -vet=off ./ringlogger -run TestWireHush -count=1
    if ($LASTEXITCODE) { throw 'Protected logging suite failed' }
    & $go test -overlay .overlay/overlay.json ./doh -run TestTLSClientHelloDoesNotPanic -count=1
    if ($LASTEXITCODE) { throw 'Overlay TLS regression failed' }
    if ($Race) {
        $env:CGO_ENABLED = '1'
        $env:CC = "$repo/.deps/bin/x86_64-w64-mingw32-gcc.exe"
        & $go test -race -vet=off ./conf ./manager ./dohruntime ./bootstrap -count=1
        if ($LASTEXITCODE) { throw 'Race suite failed' }
    }
    & "$repo/.deps/bin/x86_64-w64-mingw32-gcc.exe" -O2 -Wall -Wextra -Werror -municode -DUNICODE -D_UNICODE -DWINVER=0x0A00 -D_WIN32_WINNT=0x0A00 -o "$repo/.artifacts/v1/installer-safety-test.exe" installer/v1-customactions-test.c -lmsi -ladvapi32 -lnetapi32 -lshell32 -lole32 -luuid
    if ($LASTEXITCODE) { throw 'Installer safety test compilation failed' }
    & "$repo/.artifacts/v1/installer-safety-test.exe"
    if ($LASTEXITCODE) { throw 'Installer safety tests failed' }
    & git diff --check
    if ($LASTEXITCODE) { throw 'Diff check failed' }
    Write-Host 'PASS: V1 automated validation. Privileged and real-network acceptance remains separate.'
} finally { Stop-Transcript | Out-Null }
