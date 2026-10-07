#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$deps = Join-Path $repo '.deps'
New-Item -ItemType Directory -Force -Path $deps | Out-Null

# Archives are retained with their checksums. Existing dependency trees are never
# recursively deleted. A failed download cannot become an accepted dependency.
function Install-Archive($Name, $Url, $Digest, $Algorithm, $Destination, $Probe, $Nested = '') {
    if (Test-Path -LiteralPath $Probe) { return }
    $archive = Join-Path $deps ($Name + '.zip')
    if (!(Test-Path -LiteralPath $archive)) { Invoke-WebRequest $Url -OutFile $archive }
    if ((Get-FileHash -LiteralPath $archive -Algorithm $Algorithm).Hash -ne $Digest) {
        throw "Checksum mismatch for $Name; remove the failed archive before retrying."
    }
    $unpack = Join-Path $deps ($Name + '-unpacked')
    Expand-Archive -LiteralPath $archive -DestinationPath $unpack -Force
    $source = if ($Nested) { Join-Path $unpack $Nested } else { $unpack }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    Get-ChildItem -LiteralPath $source -Force | Copy-Item -Destination $Destination -Recurse -Force
    if (!(Test-Path -LiteralPath $Probe)) { throw "Missing expected tool after extracting $Name" }
}
Install-Archive 'go-1.27.1' 'https://go.dev/dl/go1.27.1.windows-amd64.zip' 'a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d' 'SHA256' $deps "$deps/go/bin/go.exe"
Install-Archive 'llvm-mingw-20260311' 'https://download.wireguard.com/windows-toolchain/distfiles/llvm-mingw-20260311-ucrt-x86_64.zip' 'dd4c67d98959479c7be2fb6709ba074475991590848cb9d0eb2620be06b182e1' 'SHA256' $deps "$deps/bin/aarch64-w64-mingw32-gcc.exe" 'llvm-mingw-20260311-ucrt-x86_64'
Install-Archive 'imagemagick-7.0.8-42' 'https://download.wireguard.com/windows-toolchain/distfiles/ImageMagick-7.0.8-42-portable-Q16-x64.zip' '584e069f56456ce7dde40220948ff9568ac810688c892c5dfb7f6db902aa05aa' 'SHA256' $deps "$deps/convert.exe"
Install-Archive 'wireguard-nt-1.1' 'https://download.wireguard.com/wireguard-nt/wireguard-nt-1.1.zip' 'dceb30a9bc4be48cce0f74160fc88a585a2c2627366e8f846fc6658f9038dace' 'SHA256' $deps "$deps/wireguard-nt/bin/arm64/wireguard.dll"
Install-Archive 'dotnet-10.0.401' 'https://builds.dotnet.microsoft.com/dotnet/Sdk/10.0.401/dotnet-sdk-10.0.401-win-x64.zip' '24b670ad3d923bfcf47df6c3b034152398b42f6dbc388e10d783aee1cfb5e5817d399fc0ae2a12cfa822a55e61d34830ccb15c50ef6efee437ab874bb7c79430' 'SHA512' "$deps/dotnet" "$deps/dotnet/dotnet.exe"
Install-Archive 'wix-3.14.1' 'https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314-binaries.zip' '6ac824e1642d6f7277d0ed7ea09411a508f6116ba6fae0aa5f2c7daa2ff43d31' 'SHA256' "$repo/installer/.deps/wix/bin" "$repo/installer/.deps/wix/bin/candle.exe"
if ((& "$deps/go/bin/go.exe" version) -notmatch 'go1\.27\.1 windows/amd64') { throw 'Expected Go 1.27.1 on an x64 build host' }
if ((& "$deps/dotnet/dotnet.exe" --version) -ne '10.0.401') { throw 'Expected .NET SDK 10.0.401' }
