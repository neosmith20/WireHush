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
$widths = @(1100, 1280, 1560, 1900)
$results = @()
try {
    foreach ($width in $widths) {
        if (-not [WireHushUiLayoutWin32]::MoveWindow($hwnd, 90, 70, $width, 680, $true)) {
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
