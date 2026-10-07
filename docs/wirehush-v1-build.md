# WireHush V1 builds and installation

Build on x64 Windows with PowerShell 7.2 or later and Git. From the repository:

```powershell
./scripts/bootstrap-v1.ps1
./scripts/test-v1.ps1 -Race
./scripts/build-v1.ps1 -SkipBootstrap -Installer
```

`build.bat` forwards to the production build script. `installer/build.bat`
adds installer packaging. Only x64 and ARM64 are supported. The scripts verify
pinned download checksums and tool versions; they do not delete dependency trees.
NuGet restores use committed lock files. Generated protobuf bindings are committed;
`scripts/generate-protocol.ps1` remains the separate schema-generation workflow.

Each build gets a fresh directory under `.artifacts/v1/<architecture>/<build-id>/`.
The `app/` payload contains the ordinary-user WinUI `WireHush.exe`, independent
Go `WireHush-Manager.exe`, self-contained .NET/WinUI dependencies, and `Notices/`.
`latest.json` records the latest path, architecture, version, and source commit.
`SHA256SUMS.txt` inventories every payload file. `INSTALLER-SHA256SUMS.txt` hashes
the unsigned `WireHush-<version>-<architecture>-test.msi`. No final release is
tagged, signed, uploaded, or published by these scripts or CI.

The SDK's stable WinUI, interactive-experiences, and runtime components are pinned
directly; unused AI, ML, search, and widget modules are not referenced. No model
downloads, account, analytics, or automatic update mechanism is added.

## Installer behavior

Install the MSI matching the machine's native architecture, using an administrator
account. Installation has a fixed protected Program Files location. Start WireHush
from its Start menu shortcut as an ordinary user; setup never launches a UI as
LocalSystem. The manager is LocalSystem, demand-start, with an unrestricted service
SID. The local `WireHush Users` group receives query/start/read-control rights only;
members cannot stop, reconfigure, or delete the service through SCM. The installer's
user SID is added to that group. Other users require administrator group assignment
and a new Windows logon token. A deployment running as SYSTEM adds no human account.

The candidate manager privilege list is `SeChangeNotifyPrivilege` and
`SeImpersonatePrivilege`. This is a configured candidate, **not evidence of a
validated minimum**. Acceptance must record the actual service token privileges and
exercise every manager/storage/authentication path with that token. Tunnel workers
are separate SCM services and retain their independently required driver privileges.

Before standard MSI service controls run, SYSTEM-only preflight verifies canonical
executables, service accounts, command arguments, and every owned-prefix collision.
Cleanup failure stops installation before worker deletion. A SYSTEM-owned kernel
event blocks demand-start during the transaction; rollback/commit clears it and
host termination releases it. Rollback retains the original manager binding/start
mode, and never launches a UI. Migration creates and verifies encrypted backups,
reserved-ID journal entries, and Shared records before predecessor removal. Legacy
registrations are finalized only at commit. Commit reprovisions the canonical
manager if predecessor service removal removed its registration.

Upgrade and silent uninstall keep data. Interactive uninstall defaults to Keep
Data and offers an explicit irreversible deletion checkbox. Deletion targets only
the fixed ProgramData WireHush root and normal local profile WireHush preferences.
It verifies handles, owners, canonical paths, absence of reparse points and hard
links, depth/count/time bounds, and refuses unsafe objects. Redirected/nonstandard
LocalAppData profiles require separate owner cleanup. Legacy sources outside V1
roots remain retained unless the predecessor MSI removes them; verified encrypted
migration backups remain in the V1 root by default.

Windows Installer ICE validation runs during linking. ICE03 is excluded because
distributed Microsoft runtime PE files contain language-resource values that WiX 3
cannot validate; ICE61 is excluded for deliberate same-version test-candidate
upgrades. Other ICE checks remain enabled. `-SkipIce` is available only for a build
host without an ICE service and must be reported as unvalidated packaging.

Actual elevated install, predecessor upgrade/rollback, interactive/silent removal,
SYSTEM-owned production pipe access, ordinary-user group tokens, multiuser isolation,
and native ARM64 execution remain owner acceptance checks. Successful linking and
fixture tests do not count as those checks.
