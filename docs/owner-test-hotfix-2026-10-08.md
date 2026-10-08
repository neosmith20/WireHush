# WireHush V1 installer hotfix owner evidence — 2026-10-08

This note records the owner-side Windows validation performed after the merged V1
candidate incorrectly rejected a current Windows 11 machine. It supplements the
historical WH-036 handoff; it does not rewrite that earlier evidence.

## Hotfix source

Branch: `hotfix/windows-version-gate`

- `a8d176635ca7f2866dbad3b740e63b72216ccd94` — fix the modern Windows installer version gate and add artifact clean-tree provenance checks.
- `5b45b66aa5f19276972a579324d24fa40bdeab65` — remove the mutable destination-folder wizard path and add regression coverage for the fixed-path UI.

The tested packaged source is exactly `5b45b66aa5f19276972a579324d24fa40bdeab65`.

## Test host

- Windows 11, DisplayVersion 25H2
- OS build 26200
- x64
- Existing upstream WireGuard and a historical TunnelMint tree were intentionally retained for coexistence observation.

## Final hotfix candidates

| Architecture | MSI | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| x64 | `.artifacts/v1/x64/b44a936d985e453e894b2324abbc18ac/WireHush-0.1.0-x64-test.msi` | 75398147 | `0c2203c0b8b547d97c7034660446eb144132c9b738a0e47207ef684c4ca43145` |
| ARM64 | `.artifacts/v1/arm64/1175ac60e4654ebc8c9ce2be32c63854/WireHush-0.1.0-arm64-test.msi` | 70855683 | `a578c9716052f49577a127dfb430065fe2f2ec161e4e209ad9f0bc707a86c1a3` |

Both payload `SHA256SUMS.txt` manifests verified with zero failures, and both
`INSTALLER-SHA256SUMS.txt` values matched the MSI hashes above. Both installers
were linked with normal ICE validation; the documented ICE03 and ICE61 exclusions
remain in effect and `-SkipIce` was not used.

## Automated validation

`scripts/test-v1.ps1 -Race` exited 0 after both installer fixes. This included the
regression suites, production-tag manager tests, protected logging test, TLS
regression, race suites, native installer safety fixture, UI traffic safety tests,
dependency gate, Windows build-detection regression guard, fixed-path installer UI
regression guard, artifact clean-tree provenance guard, and `git diff --check`.

## Installed x64 validation

A prior WireHush product was removed, WireHush Program Files/ProgramData/LocalAppData
and the local access group were cleared for the clean baseline, while upstream
WireGuard and the historical TunnelMint tree were left intact.

The final committed x64 MSI then completed a full interactive fresh install with
exit code 0. The live wizard path was Welcome -> License -> Ready to Install ->
Install -> Finish. No Destination Folder or Change-path page was exposed.

Installed state verified:

- `WireHushManager` is Manual/Demand Start, LocalSystem.
- Canonical service image is `C:\Program Files\WireHush\WireHush-Manager.exe /managerservice`.
- Required manager privileges are `SeChangeNotifyPrivilege` and `SeImpersonatePrivilege`.
- Service SID type is unrestricted.
- Starting the exact installed manager creates named pipe `WireHush.Manager.v1`.
- No TCP listener was owned by the manager process.
- `C:\Program Files\WireHush` and `C:\ProgramData\WireHush` are SYSTEM-owned.
- ProgramData uses a protected SYSTEM/Administrators-only DACL.
- Start Menu shortcut is present.
- Installed WireHush.exe, WireHush.dll, WireHush-Manager.exe, WireHush.Protocol.dll,
  and WireHush.pri hashes match the committed x64 build payload.
- Normal unelevated UI launch stays open. A second same-session launch redirects to
  the existing instance.
- Before a fresh logon, the UI clearly reports that the account needs a refreshed
  `WireHush Users` token.
- Elevated UI launch is intentionally rejected by the application.
- No relevant Application/System error or warning events were observed in the final
  install/runtime sanity window.

## Final uninstall/reinstall validation

The exact final x64 MSI also passed:

- Silent uninstall with default Keep Data: product, service and Program Files were
  removed while SYSTEM-owned ProgramData/log data remained intact.
- Reinstall over retained data: canonical service returned and retained data kept
  the same owner and size.
- Explicit `DELETE_WIREHUSH_DATA=1` uninstall: WireHush product, service,
  Program Files, ProgramData and LocalAppData were removed successfully.
- The interactive maintenance path reaches the custom permanent-delete warning and
  checkbox.
- An earlier owner-test unsafe-owner sentinel was correctly refused by explicit
  deletion, proving the fail-closed deletion boundary.
- Upstream WireGuard and the separate TunnelMint tree remained present throughout.

After these destructive tests, the final x64 candidate was installed again from a
zero WireHush baseline. The installer recreated `WireHush Users` and added the
installing account. The manager is currently stopped as expected for demand start.

## Still required before release

This evidence does not complete V1 acceptance. Remaining runtime gates include a
fresh human logon followed by ordinary-user authenticated manager connection,
installed two-account Private/Shared authorization, real VPN peer and DNS/DoH
IPv4/IPv6 packet/leak testing, sleep/wake and network-change transitions,
visual/DPI/keyboard review, native ARM64 execution, remote CI, signing, tagging and
publication.
