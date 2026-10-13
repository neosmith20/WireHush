[CmdletBinding()]
param(
    [string] $ResultPath = 'C:\TunnelMint-Test\task014-live-tunnel-state.json'
)

$ErrorActionPreference = 'Stop'

function Get-AdapterCounters {
    param([string[]] $Names)

    $counters = @{}
    foreach ($name in $Names) {
        $stats = Get-NetAdapterStatistics -Name $name -ErrorAction Stop
        $counters[$name] = [ordered]@{
            sent = [Int64] $stats.SentBytes
            received = [Int64] $stats.ReceivedBytes
        }
    }
    return $counters
}

$result = [ordered]@{
    startedAtUtc = [DateTime]::UtcNow.ToString('o')
    administrator = $false
    manager = [ordered]@{ exists = $false; running = $false }
    tunnelServices = [ordered]@{ count = 0; runningCount = 0 }
    tunnelAdapters = [ordered]@{ matchedCount = 0; upCount = 0; routeCount = 0 }
    trafficProbe = [ordered]@{
        attempted = $false
        tcpConnected = $false
        tunnelAdapterSentBytesDelta = 0
        tunnelAdapterReceivedBytesDelta = 0
    }
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
    $result.manager.exists = $true
    $result.manager.running = $manager.Status -eq 'Running'

    $services = @(Get-Service -Name 'TunnelMintTunnel$*' -ErrorAction SilentlyContinue)
    $result.tunnelServices.count = $services.Count
    $result.tunnelServices.runningCount = @($services | Where-Object { $_.Status -eq 'Running' }).Count
    if ($result.tunnelServices.runningCount -eq 0) {
        throw 'No running WireHush tunnel service was found.'
    }

    # Tunnel adapter aliases match the service suffix. Keep aliases only in
    # memory: tunnel names are intentionally never written to the result.
    $aliases = @($services | ForEach-Object { $_.Name.Substring('TunnelMintTunnel$'.Length) })
    $adapters = @(Get-NetAdapter -Name $aliases -IncludeHidden -ErrorAction Stop)
    $result.tunnelAdapters.matchedCount = $adapters.Count
    $result.tunnelAdapters.upCount = @($adapters | Where-Object { $_.Status -eq 'Up' }).Count
    $result.tunnelAdapters.routeCount = @($adapters | ForEach-Object {
        @(Get-NetRoute -InterfaceIndex $_.ifIndex -ErrorAction SilentlyContinue)
    }).Count
    if ($result.tunnelAdapters.upCount -eq 0) {
        throw 'No WireHush tunnel adapter is Up.'
    }

    $before = Get-AdapterCounters -Names $aliases
    $result.trafficProbe.attempted = $true
    try {
        # This bounded TCP attempt is used only to create ordinary client
        # traffic. The result does not claim which route policy selected it.
        $wireHushProbeHost = '1.1.1.1'
        $result.trafficProbe.tcpConnected = [bool] (Test-NetConnection -ComputerName $wireHushProbeHost -Port 443 -InformationLevel Quiet -WarningAction SilentlyContinue)
    }
    catch {
        $result.trafficProbe.tcpConnected = $false
    }
    Start-Sleep -Seconds 5
    $after = Get-AdapterCounters -Names $aliases
    foreach ($alias in $aliases) {
        $result.trafficProbe.tunnelAdapterSentBytesDelta += $after[$alias].sent - $before[$alias].sent
        $result.trafficProbe.tunnelAdapterReceivedBytesDelta += $after[$alias].received - $before[$alias].received
    }
}
catch {
    $result.error = $_.Exception.Message
}
finally {
    $result.completedAtUtc = [DateTime]::UtcNow.ToString('o')
    $resultDirectory = Split-Path -Parent $ResultPath
    New-Item -ItemType Directory -Path $resultDirectory -Force | Out-Null
    $result | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $ResultPath -Encoding utf8
}

if ($result.error) {
    Write-Error "Task 014 live tunnel state collection failed. Sanitized result: $ResultPath"
    exit 1
}

Write-Host "Task 014 live tunnel state collection passed. Sanitized result: $ResultPath"
