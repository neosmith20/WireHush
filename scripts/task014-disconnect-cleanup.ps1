[CmdletBinding()]
param(
    [string] $ResultPath = 'C:\TunnelMint-Test\task014-disconnect-cleanup.json'
)

$ErrorActionPreference = 'Stop'

function Get-AdapterSummary {
    param([string] $Alias)

    $adapter = Get-NetAdapter -Name $Alias -IncludeHidden -ErrorAction SilentlyContinue
    if (-not $adapter) {
        return [ordered]@{ exists = $false; up = $false; routeCount = 0 }
    }
    return [ordered]@{
        exists = $true
        up = $adapter.Status -eq 'Up'
        routeCount = @(Get-NetRoute -InterfaceIndex $adapter.ifIndex -ErrorAction SilentlyContinue).Count
    }
}

$result = [ordered]@{
    startedAtUtc = [DateTime]::UtcNow.ToString('o')
    administrator = $false
    managerRunning = $false
    before = $null
    after = $null
    postDisconnect = [ordered]@{ tcpConnected = $false; dnsResolved = $false }
    error = $null
    completedAtUtc = $null
}

try {
    $principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
    $result.administrator = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $result.administrator) {
        throw 'Run this script from an Administrator PowerShell window.'
    }

    $manager = Get-Service -Name 'TunnelMintManager' -ErrorAction Stop
    $result.managerRunning = $manager.Status -eq 'Running'
    if (-not $result.managerRunning) {
        throw 'WireHush Manager service (TunnelMintManager) is not running.'
    }

    $services = @(Get-Service -Name 'TunnelMintTunnel$*' -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'Running' })
    if ($services.Count -ne 1) {
        throw "Expected exactly one running WireHush tunnel service; found $($services.Count)."
    }
    $serviceName = $services[0].Name
    $alias = $serviceName.Substring('TunnelMintTunnel$'.Length)
    $result.before = [ordered]@{
        serviceRunning = $true
        adapter = Get-AdapterSummary -Alias $alias
    }
    if (-not $result.before.adapter.up -or $result.before.adapter.routeCount -eq 0) {
        throw 'The active WireHush adapter is not up with routes before disconnect.'
    }

    Write-Host 'Disconnect the active tunnel from the WireHush UI, wait for its status to show disconnected, then press Enter.'
    [void] (Read-Host)

    $afterService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    $result.after = [ordered]@{
        serviceExists = $null -ne $afterService
        serviceRunning = $null -ne $afterService -and $afterService.Status -eq 'Running'
        adapter = Get-AdapterSummary -Alias $alias
    }
    if ($result.after.serviceRunning) {
        throw 'The WireHush tunnel service is still running after disconnect.'
    }
    if ($result.after.adapter.up) {
        throw 'The WireHush adapter is still Up after disconnect.'
    }
    if ($result.after.adapter.routeCount -gt 0) {
        throw 'WireHush adapter routes remain after disconnect.'
    }

    $wireHushProbeHost = '1.1.1.1'
    $result.postDisconnect.tcpConnected = [bool] (Test-NetConnection -ComputerName $wireHushProbeHost -Port 443 -InformationLevel Quiet -WarningAction SilentlyContinue)
    try {
        Resolve-DnsName -Name 'example.com' -Type A -DnsOnly -ErrorAction Stop | Out-Null
        $result.postDisconnect.dnsResolved = $true
    }
    catch {
        $result.postDisconnect.dnsResolved = $false
    }
    if (-not $result.postDisconnect.tcpConnected -or -not $result.postDisconnect.dnsResolved) {
        throw 'Ordinary TCP or DNS connectivity did not recover after disconnect.'
    }
}
catch {
    $result.error = $_.Exception.Message
}
finally {
    $result.completedAtUtc = [DateTime]::UtcNow.ToString('o')
    New-Item -ItemType Directory -Path (Split-Path -Parent $ResultPath) -Force | Out-Null
    $result | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $ResultPath -Encoding utf8
}

if ($result.error) {
    Write-Error "Task 014 disconnect cleanup failed. Sanitized result: $ResultPath"
    exit 1
}

Write-Host "Task 014 disconnect cleanup completed. Sanitized result: $ResultPath"
