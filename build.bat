@echo off
rem SPDX-License-Identifier: MIT
rem Copyright (C) 2026 WireHush. All Rights Reserved.
where pwsh.exe >nul 2>nul
if errorlevel 1 (
  echo PowerShell 7.2 or later is required to build WireHush V1.
  exit /b 1
)
pwsh.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\build-v1.ps1" %*
exit /b %errorlevel%
