@echo off
rem SPDX-License-Identifier: MIT
rem Copyright (C) 2026 WireHush. All Rights Reserved.
call "%~dp0..\build.bat" -Installer %*
exit /b %errorlevel%
