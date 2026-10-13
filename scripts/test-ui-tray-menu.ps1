#requires -Version 7.2
# SPDX-License-Identifier: MIT
# Interactive Windows UI Automation acceptance for the native notification-area menu.
# Launch a non-elevated QA WireHush build before running this script.
# This never confirms Exit or stops a VPN tunnel.
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
public static class WireHushTrayWin32 {
    [StructLayout(LayoutKind.Sequential)]
    public struct POINT { public int X; public int Y; }
    [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT pos);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool PostMessageW(IntPtr hwnd, uint msg, UIntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
}
'@
$fullPath = [IO.Path]::GetFullPath($ExecutablePath)
$processes = @(Get-Process WireHush -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $fullPath -and $_.MainWindowHandle -ne 0 })
if ($processes.Count -ne 1) {
    throw "Expected one visible QA WireHush at $fullPath; found $($processes.Count)."
}
$process = $processes[0]
$hwnd = [IntPtr] $process.MainWindowHandle
if (-not [WireHushTrayWin32]::IsWindowVisible($hwnd)) {
    throw 'WireHush must start visible so its HWND can be captured safely.'
}
$initialCursor = [WireHushTrayWin32+POINT]::new()
$haveCursor = [WireHushTrayWin32]::GetCursorPos([ref] $initialCursor)

function Invoke-TrayMenuItem([string] $name) {
    if (-not [WireHushTrayWin32]::PostMessageW($hwnd, 0x8251,
            [UIntPtr]::new(1), [IntPtr]::new(0x7B))) {
        throw 'Could not open the WireHush native tray menu.'
    }
    Start-Sleep -Milliseconds 500
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = [System.Windows.Automation.PropertyCondition]::new(
        [System.Windows.Automation.AutomationElement]::NameProperty, $name)
    $menuMatches = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)
    $item = $null
    for ($i = 0; $i -lt $menuMatches.Count; $i++) {
        $element = $menuMatches.Item($i)
        if ($element.Current.ControlType -eq [System.Windows.Automation.ControlType]::MenuItem -and
            -not $element.Current.IsOffscreen) {
            $item = $element
            break
        }
    }
    if ($null -eq $item) { throw "Native tray command was not visible: $name." }
    $bounds = $item.Current.BoundingRectangle
    [WireHushTrayWin32]::SetCursorPos(
        [int] ($bounds.X + $bounds.Width / 2),
        [int] ($bounds.Y + $bounds.Height / 2)) | Out-Null
    Start-Sleep -Milliseconds 150
    [WireHushTrayWin32]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)
    Start-Sleep -Milliseconds 80
    [WireHushTrayWin32]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
    Start-Sleep -Milliseconds 650
}

try {
    Invoke-TrayMenuItem 'Hide WireHush'
    $process.Refresh()
    if ($process.HasExited -or [WireHushTrayWin32]::IsWindowVisible($hwnd)) {
        throw 'Tray Hide failed to preserve the process and hide the window.'
    }
    Write-Output 'PASS: tray Hide keeps VPN UI process running while hiding the window.'

    Invoke-TrayMenuItem 'Show WireHush'
    if (-not [WireHushTrayWin32]::IsWindowVisible($hwnd)) {
        throw 'Tray Show failed to restore the window.'
    }
    Write-Output 'PASS: tray Show restores the hidden window.'

    Invoke-TrayMenuItem 'Exit WireHush...'
    $window = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
    $confirmation = $window.FindFirst(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.PropertyCondition]::new(
            [System.Windows.Automation.AutomationElement]::NameProperty, 'Exit WireHush?'))
    if ($null -eq $confirmation) { throw 'Tray Exit skipped its confirmation dialog.' }
    Write-Output 'PASS: tray Exit opens a confirmation dialog.'

    $cancel = $window.FindFirst(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.PropertyCondition]::new(
            [System.Windows.Automation.AutomationElement]::NameProperty, 'Cancel'))
    if ($null -eq $cancel) { throw 'Exit confirmation lacked a Cancel button.' }
    $cancel.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Start-Sleep -Milliseconds 450
    $process.Refresh()
    if ($process.HasExited) { throw 'Cancel unexpectedly terminated WireHush.' }
    Write-Output 'PASS: Cancel preserves WireHush without disconnecting a tunnel.'
}
finally {
    if ($haveCursor) {
        [WireHushTrayWin32]::SetCursorPos($initialCursor.X, $initialCursor.Y) | Out-Null
    }
    if (-not [WireHushTrayWin32]::IsWindowVisible($hwnd)) {
        [WireHushTrayWin32]::PostMessageW($hwnd, 0x8251,
            [UIntPtr]::new(1), [IntPtr]::new(0x202)) | Out-Null
    }
}
