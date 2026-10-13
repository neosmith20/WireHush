#requires -Version 7.2
# WireHush source quality gate: pinned Go toolchain and Staticcheck.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $repo
$go = Join-Path $repo '.deps/go/bin/go.exe'
$formatter = Join-Path $repo '.deps/go/bin/gofmt.exe'
if (!(Test-Path $go) -or !(Test-Path $formatter)) { throw 'Run scripts/bootstrap-v1.ps1 first' }
$env:GOROOT = Join-Path $repo '.deps/go'
$env:GOPATH = Join-Path $repo '.deps/gopath'
$env:PATH = "$env:GOROOT/bin;$env:PATH"
$packages = @('./bootstrap','./conf','./dnsproxy','./doh','./dohruntime','./manager','./product','./protocol','./tunnel','./tunnel/firewall')
$files = @(git ls-files -- '*.go')
if ($LASTEXITCODE -ne 0 -or !$files.Count) { throw 'Unable to enumerate tracked Go source' }
$unformatted = @(& $formatter -l @files)
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
if ($unformatted.Count) { $unformatted | Out-Host; throw 'Go formatting differs from gofmt' }
Write-Host "PASS: gofmt ($($files.Count) tracked files)"
# The upstream Windows networking code has three reviewed, longstanding
# unsafe.Pointer pointer-arithmetic vet diagnostics. Run every other vet
# analyzer; do not suppress any other category. Track the exceptions in
# docs/code-quality-exceptions.md.
& $go vet -unsafeptr=false @packages
if ($LASTEXITCODE -ne 0) { throw 'go vet reported diagnostics' }
Write-Host 'PASS: go vet (known upstream unsafeptr analyzer exception)'
# The SA* checks find actual Go correctness problems, not the inherited
# Windows/translation string-style and legacy unused-declaration warnings.
& $go install honnef.co/go/tools/cmd/staticcheck@2026.2.1
if ($LASTEXITCODE -ne 0) { throw 'Pinned Staticcheck installation failed' }
$staticcheck = Join-Path $env:GOPATH 'bin/staticcheck.exe'
& $staticcheck '-checks=SA*' @packages
if ($LASTEXITCODE -ne 0) { throw 'Staticcheck SA analyzers reported diagnostics' }
Write-Host 'PASS: Staticcheck 2026.2.1 SA*'
