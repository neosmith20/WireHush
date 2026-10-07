# Building and running WireHush V1

Use [the V1 build guide](wirehush-v1-build.md) for the current split Go manager and
.NET 10 WinUI application, pinned tools, x64/ARM64 installers, and CI behavior.

From an x64 Windows checkout with PowerShell 7.2 or later:

```powershell
./scripts/bootstrap-v1.ps1
./scripts/test-v1.ps1 -Race
./scripts/build-v1.ps1 -SkipBootstrap -Installer
```

Install the generated native-architecture test MSI as administrator. Sign out/in
after `WireHush Users` membership is assigned, then launch the Start menu shortcut
normally. `WireHush.exe` is the ordinary-user UI; `WireHush-Manager.exe` is managed
by SCM as LocalSystem. The UI never installs the service or requires elevation.

See [owner acceptance](windows-v1-acceptance.md) before real use. Test installers
are unsigned; successful compilation is not elevated installation or VPN evidence.
Only x64 and ARM64 are supported. The inherited combined WireGuard UI/backend,
its gob IPC, updater and x86 build are excluded from production V1. Historical
source and earlier task reports are retained as references, not current run steps.
