#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Windows interactive UIA acceptance: details tabs, resizing, and scroll persistence.
# Run against a separately launched non-elevated WireHush QA executable.
# Does not create, edit, start, or stop tunnels.
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $ExecutablePath
)
$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName UIAutomationClient
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class WireHushUiLayoutWin32 {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out RECT rectangle);
    [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hwnd, int x, int y, int width, int height, bool repaint);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
}
'@

$fullPath = [IO.Path]::GetFullPath($ExecutablePath)
if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
    throw "UI acceptance executable not found: $fullPath"
}
$running = @(Get-Process WireHush -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $fullPath -and $_.MainWindowHandle -ne 0 })
if ($running.Count -ne 1) {
    throw "Expected exactly one interactive WireHush window at the supplied path; found $($running.Count)."
}
$hwnd = [IntPtr] $running[0].MainWindowHandle
$root = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
$predicate = [System.Windows.Automation.Condition]::TrueCondition
$oldBounds = [WireHushUiLayoutWin32+RECT]::new()
if (-not [WireHushUiLayoutWin32]::GetWindowRect($hwnd, [ref] $oldBounds)) {
    throw 'Could not capture original window bounds for restoration.'
}

function Get-NamedTabButton([string] $name) {
    $elements = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $predicate)
    for ($i = 0; $i -lt $elements.Count; $i++) {
        $element = $elements.Item($i)
        if ($element.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button -and
            $element.Current.Name -eq $name) {
            return $element
        }
    }
    throw "Missing detail tab: $name"
}
function Get-DetailsScroll {
    $elements = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $predicate)
    $winner = $null
    $maxWidth = 0.0
    for ($i = 0; $i -lt $elements.Count; $i++) {
        $element = $elements.Item($i)
        $pattern = $null
        if ($element.TryGetCurrentPattern([System.Windows.Automation.ScrollPattern]::Pattern, [ref] $pattern)) {
            $bounds = $element.Current.BoundingRectangle
            if ($bounds.Width -gt $maxWidth) {
                $winner = $pattern
                $maxWidth = $bounds.Width
            }
        }
    }
    return $winner
}

$sections = @('Overview', 'Network', 'DNS', 'Peer', 'Allowed IPs')
$widths = @(900, 1100, 1280, 1560, 1900)
$results = @()
try {
    foreach ($width in $widths) {
        $height = if ($width -eq 900) { 560 } else { 680 }
        if (-not [WireHushUiLayoutWin32]::MoveWindow($hwnd, 90, 70, $width, $height, $true)) {
            throw "Window resize failed at $width pixels."
        }
        [WireHushUiLayoutWin32]::SetForegroundWindow($hwnd) | Out-Null
        Start-Sleep -Milliseconds 550
        # Verify the connection rail actually switches between compact and full
        # controls instead of simply clipping the full-width search box.
        $elements = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $predicate)
        $railMode = $null
        for ($i = 0; $i -lt $elements.Count; $i++) {
            $element = $elements.Item($i)
            if ($element.Current.IsOffscreen) { continue }
            if ($element.Current.Name -eq 'Search tunnels' -and
                $element.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button) {
                $railMode = 'compact'
            }
            if ($element.Current.Name -eq 'Search tunnels...' -and
                $element.Current.ControlType -eq [System.Windows.Automation.ControlType]::Edit) {
                $railMode = 'expanded'
            }
        }
        $expectedRailMode = if ($width -lt 1380) { 'compact' } else { 'expanded' }
        if ($railMode -ne $expectedRailMode) {
            throw "Navigation rail did not adapt: width=$width, expected=$expectedRailMode, actual=$railMode."
        }
        # The header actions must remain present and accessible at minimum size.
        foreach ($heading in @('Settings', 'Log', 'Help')) {
            $buttonText = $root.FindFirst(
                [System.Windows.Automation.TreeScope]::Descendants,
                [System.Windows.Automation.PropertyCondition]::new(
                    [System.Windows.Automation.AutomationElement]::NameProperty,
                    $heading))
            if ($null -eq $buttonText -or $buttonText.Current.IsOffscreen) {
                throw "Header control inaccessible at width=${width}: $heading."
            }
        }
        if ($width -in @(900, 1100, 1900)) {
            (Get-NamedTabButton 'Overview').GetCurrentPattern(
                [System.Windows.Automation.InvokePattern]::Pattern).Invoke()
            $initialScroll = Get-DetailsScroll
            if ($initialScroll.Current.VerticallyScrollable) {
                $initialScroll.SetScrollPercent(-1, 50)
            }
            Start-Sleep -Milliseconds 250
            $cardHeadings = $root.FindAll(
                [System.Windows.Automation.TreeScope]::Descendants, $predicate)
            $connection = $null
            $dns = $null
            for ($i = 0; $i -lt $cardHeadings.Count; $i++) {
                $candidate = $cardHeadings.Item($i)
                if ($candidate.Current.ControlType -ne [System.Windows.Automation.ControlType]::Text -or
                    $candidate.Current.IsOffscreen) { continue }
                if ($candidate.Current.Name -eq 'Connection') { $connection = $candidate.Current.BoundingRectangle }
                if ($candidate.Current.Name -eq 'DNS') { $dns = $candidate.Current.BoundingRectangle }
            }
            if ($null -eq $connection -or $null -eq $dns) {
                throw "Overview card headings could not be inspected at width=$width."
            }
            $sameColumn = [Math]::Abs($connection.X - $dns.X) -lt 12
            $sameRow = [Math]::Abs($connection.Y - $dns.Y) -lt 12
            if ($width -eq 900 -and (-not $sameColumn -or $sameRow)) {
                throw 'At 900px, Overview cards must be stacked in one column.'
            }
            if ($width -ne 900 -and ($sameColumn -or -not $sameRow)) {
                throw "At width=$width, Overview cards should share a row."
            }
        }
        foreach ($section in $sections) {
            (Get-NamedTabButton $section).GetCurrentPattern(
                [System.Windows.Automation.InvokePattern]::Pattern).Invoke()
            Start-Sleep -Milliseconds 280
            $scroll = Get-DetailsScroll
            if ($null -eq $scroll) {
                throw "Detail viewport inaccessible at width=$width, tab=$section."
            }
            $scrollable = $scroll.Current.VerticallyScrollable
            if ($scrollable) {
                $scroll.SetScrollPercent(-1, 80)
                Start-Sleep -Milliseconds 400
                $initial = $scroll.Current.VerticalScrollPercent
                Start-Sleep -Milliseconds 1000
                $later = $scroll.Current.VerticalScrollPercent
                if ($later -lt 70) {
                    throw "Scroll position reset at width=$width, tab=$section; initial=$initial, later=$later."
                }
            } else {
                $initial = -1
                $later = -1
            }
            $results += [pscustomobject]@{
                Width = $width
                Section = $section
                Overflow = $scrollable
                ScrollBefore = $initial
                ScrollAfter = $later
            }
        }
    }
    $results | Format-Table -AutoSize | Out-Host
    Write-Output "PASS: $($results.Count) tab/width checks, compact/expanded rails, and retained scrolling on overflowing content."
}
finally {
    # Do not leave a developer's interactive desktop resized or on a different tab.
    try { (Get-NamedTabButton 'Overview').GetCurrentPattern(
        [System.Windows.Automation.InvokePattern]::Pattern).Invoke() }
    catch { Write-Warning 'Unable to restore Overview tab.' }
    [WireHushUiLayoutWin32]::MoveWindow($hwnd, $oldBounds.Left, $oldBounds.Top,
        $oldBounds.Right - $oldBounds.Left, $oldBounds.Bottom - $oldBounds.Top, $true) | Out-Null
}
