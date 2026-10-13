#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Offline contract tests; never download dependencies or alter .deps.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'get-pinned-archive.ps1')

function Assert([bool] $Condition, [string] $Message) {
    if (!$Condition) { throw $Message }
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("WireHush-pinned-fetch-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $testRoot | Out-Null
$fixture = Join-Path $testRoot 'expected.zip'
[IO.File]::WriteAllText($fixture, 'test-only pinned archive fixture')
$expectedHash = (Get-FileHash -LiteralPath $fixture -Algorithm SHA256).Hash
$script:fixturePath = $fixture
$script:downloadCalls = 0
$script:failCount = 0
$script:writeInvalid = $false
$script:delays = @()

# Inject fake download and delay callbacks; never shadow built-in PowerShell commands.
$mockDownload = {
    param([string] $Uri, [string] $OutFile)
    Assert ($Uri -eq 'https://example.invalid/test.zip') 'Unexpected fixture URL.'
    $script:downloadCalls++
    if ($script:downloadCalls -le $script:failCount) {
        [IO.File]::WriteAllText($OutFile, 'untrusted partial response')
        throw 'HTTP 502 from test-only mock'
    }
    if ($script:writeInvalid) {
        [IO.File]::WriteAllText($OutFile, 'wrong dependency contents')
    }
    else {
        Copy-Item -LiteralPath $script:fixturePath -Destination $OutFile -Force
    }
}
$mockWait = { param([int] $Seconds) $script:delays += $Seconds }

try {
    $archive = Join-Path $testRoot 'retry.zip'
    $script:failCount = 1
    Get-PinnedArchive -Name 'fixture-retry' -Url 'https://example.invalid/test.zip' -Archive $archive -Digest $expectedHash -Algorithm SHA256 -Download $mockDownload -Wait $mockWait
    Assert (Test-Path $archive) 'Verified archive was not installed after transient retry.'
    Assert ($script:downloadCalls -eq 2) '502 recovery should retry exactly once before succeeding.'
    Assert ($script:delays.Count -eq 1 -and $script:delays[0] -eq 2) 'Retry backoff was incorrect.'
    Assert (!(Test-Path "$archive.part")) 'Temporary download file survived successful installation.'

    Get-PinnedArchive -Name 'fixture-cache' -Url 'https://example.invalid/test.zip' -Archive $archive -Digest $expectedHash -Algorithm SHA256 -Download $mockDownload -Wait $mockWait
    Assert ($script:downloadCalls -eq 2) 'Verified cached archive must not be downloaded again.'

    [IO.File]::WriteAllText($archive, 'corrupted cache')
    $caught = $false
    try {
        Get-PinnedArchive -Name 'fixture-bad-cache' -Url 'https://example.invalid/test.zip' -Archive $archive -Digest $expectedHash -Algorithm SHA256 -Download $mockDownload -Wait $mockWait
    }
    catch { $caught = $_.Exception.Message -like '*Checksum mismatch*' }
    Assert $caught 'Corrupted cached archive was not rejected.'
    Assert ($script:downloadCalls -eq 2) 'A corrupt cache must not trigger an automatic, unreviewed replacement.'

    $script:downloadCalls = 0
    $script:failCount = 0
    $script:writeInvalid = $true
    $badArchive = Join-Path $testRoot 'bad-download.zip'
    $caught = $false
    try {
        Get-PinnedArchive -Name 'fixture-bad-bytes' -Url 'https://example.invalid/test.zip' -Archive $badArchive -Digest $expectedHash -Algorithm SHA256 -Download $mockDownload -Wait $mockWait
    }
    catch { $caught = $_.Exception.Message -like '*Checksum mismatch*' }
    Assert $caught 'Downloaded bytes with invalid checksum were accepted.'
    Assert ($script:downloadCalls -eq 1) 'Checksum mismatch must fail immediately.'
    Assert (!(Test-Path $badArchive) -and !(Test-Path "$badArchive.part")) 'Bad download or partial file was left behind.'

    $script:downloadCalls = 0
    $script:failCount = 99
    $script:writeInvalid = $false
    $script:delays = @()
    $exhausted = Join-Path $testRoot 'exhausted.zip'
    $caught = $false
    try {
        Get-PinnedArchive -Name 'fixture-unavailable' -Url 'https://example.invalid/test.zip' -Archive $exhausted -Digest $expectedHash -Algorithm SHA256 -Download $mockDownload -Wait $mockWait
    }
    catch { $caught = $_.Exception.Message -like '*after 4 attempts*' }
    Assert $caught 'Unreachable source should fail with bounded retry count.'
    Assert ($script:downloadCalls -eq 4) 'Retry budget exceeded four attempts.'
    Assert (($script:delays -join ',') -eq '2,4,8') 'Exponential backoff schedule incorrect.'
    Assert (!(Test-Path $exhausted) -and !(Test-Path "$exhausted.part")) 'Failed requests left a partial dependency archive.'
    Write-Output 'PASS: pinned dependency retry, cache integrity, bad digest and clean bounded failure.'
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}
