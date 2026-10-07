# WireHush V1 owner-test handoff — 2026-10-07

The remaining WH-027 through WH-036 implementation slices are complete and
checkpointed locally on `blueprint/wirehush-v1`. The final unsigned x64 and ARM64
candidates are ready for full owner testing. This is not production acceptance.
No tag, release publication, signing, push, or remote CI run was performed.

Accepted starting checkpoint: `9391f12f98e35f5ff7e65704d1c271e4a9f64613` (WH-026).
Both final artifacts were built from `399102f0e3214e65763c9b3e05c01d226e40c15c`.
The following handoff commit changes documentation only; packaged product source
is unchanged. The working tree was clean at candidate validation.

Implemented scope includes the authenticated Windows named-pipe gRPC boundary,
immutable record identities and Private/Shared authorization, serialized admission,
SCM recovery and bounded failure-preserving cleanup, events/reconnection/session
exit, the approved live WinUI shell, protected storage/settings and redacted
diagnostics, encrypted migration backups/journal, native installer lifecycle,
checksum-pinned builds, notices, and CI preparation. The owner-authorized OPEN
decisions are recorded in `wirehush-v1-contract.md`; LOCKED requirements remain
authoritative.

## Final candidates

| Architecture | MSI, relative to `C:\Dev\WireHush` | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| x64 | `.artifacts/v1/x64/c8d82e3695e24fcaaec0623bf14327c6/WireHush-0.1.0-x64-test.msi` | 75390984 | `7d240bd089ec67e0e07eeac538e2716087cfaba6421699b1ff351fc9383d44cd` |
| ARM64 | `.artifacts/v1/arm64/379595b0ce9e40f3b0749249a116351c/WireHush-0.1.0-arm64-test.msi` | 70860808 | `b0af7a04b3dd22f1539983dd449baf0bcc41be7a48c601041c7fa2e632fe618f` |

Each directory includes `SHA256SUMS.txt`, `INSTALLER-SHA256SUMS.txt`, and the
self-contained `app/` with notices. Use these final paths; earlier intermediate
candidates under `.artifacts` are historical and should not be tested.

## Exact final validation

`./scripts/test-v1.ps1 -Race` exited 0 on the committed candidate source. The saved
transcript is `.artifacts/v1/validation.txt` (end: 2026-10-07 07:58:20 local).
All durations below are the tool's measured package durations, not estimates.

```text
Regression (-vet=off, -count=1):
bootstrap         PASS 0.905s
dnsproxy          PASS 1.631s
doh               PASS 1.081s
dohruntime        PASS 0.285s
product           PASS 0.255s
conf              PASS 2.057s
manager           PASS 0.371s
protocol          PASS 0.238s
tunnel            PASS 0.619s
tunnel/firewall   PASS 0.404s
Production tag wirehush_v1:
manager           PASS 0.306s
cmd/wirehush-manager compiled (no test files)
Protected ringlogger TestWireHush: PASS 0.283s
Overlay TLS TestTLSClientHelloDoesNotPanic: PASS 1.198s
Race (-vet=off, -count=1):
conf              PASS 2.856s
manager           PASS 2.270s
dohruntime        PASS 1.229s
bootstrap         PASS 1.848s
Production manager race: PASS 2.111s
Installer C safety fixture: PASS
UI measured-traffic safety executable: PASS
Production dependency gate: PASS
git diff --check: PASS
```

The installer fixture verifies canonical service ownership, hardlink rejection,
foreign-owner rejection and safe handle-based deletion. UI safety tests verify
elapsed-time rates, no fabricated first rate, counter restart, bursts, observation
gaps, nonmonotonic timing, time expiration and the 600-sample bound. The inherited
interactive ringlogger `TestFollow` intentionally never exits and is excluded;
protected V1 logging tests are run explicitly. Vet remains disabled for the
inherited suite; this report does not claim a vet pass.

`./scripts/build-v1.ps1 -SkipBootstrap -Installer` exited 0 for both architectures.
Production Go builds use `wirehush_v1,load_wgnt_from_rsrc` and the pinned overlay;
WinUI publishes self-contained with locked restore. WiX linking passed with ICE03
and ICE61 excluded for the documented runtime language metadata/same-version
candidate reasons. Other ICE checks ran; `-SkipIce` was not used.

Final artifact verification passed all 499 payload-file checksums per architecture,
with no additional uninventoried files. UI executable, manager executable and
installer custom-action DLL machine headers were `0x8664` for x64 and `0xaa64`
for ARM64. No `.conf`, `.dpapi`, `.pdb`, `.go` or `.cs` files, or unused AI/ML/search/
widget modules were found in either payload. The x64 manager reports
`WireHush-Manager 0.1.0`. Evidence is `.artifacts/v1/artifact-verification.txt`.

Runtime testing found and corrected a missing published XAML resource index;
the packaging guard now requires the index and actual branding PNG. The final
x64 payload created a live window, survived 16 seconds, and redirected a second
same-session launch (exit 0) while the original window remained alive. Only the
new smoke processes were terminated. Evidence is `.artifacts/v1/ui-smoke.txt`.
An existing combined-client manager registration was observed at
`C:\Program Files\WireHush\wirehush.exe /managerservice`; the candidate was not
installed over it. This smoke result does not establish authenticated production
manager connectivity or visual acceptance.

## Commit checkpoints

| Commit | Result |
| --- | --- |
| `84991e309290aa906a0973e17da1089e34f16f73` | WH-027 bounded cleanup and preserved records |
| `008374deb79d7ca3c851f3ee8db78dcb6cbcb688` | WH-028 serialized mutations and single active tunnel |
| `f3b123417a1973319caebaf9b93ff3bf8ccb7fb8` | WH-029 shared protobuf contract |
| `dcc19845a7b99138143248d633680a5a8ecd10b6` | WH-030 Windows-token pipe authentication |
| `67c7632bfa05a6acf4f13fa257b56f19d316329d` | WH-031 RPC, sessions, events and lifecycle |
| `2a3217c4d6f30d35c3d3993692e0a266231193f7` | WH-032 live approved WinUI integration |
| `e47f5fc6b940549db3c9c6fdab31d637c53cf4bc` | WH-031 failed cleanup evidence retained |
| `d65951f61cd10bac41b325424f18b3635a4b0197` | WH-033 protected settings and redacted logging |
| `3f203a62fc2866d5c5510276440249d4b62a353b` | WH-034 encrypted migration and durable identities |
| `4ddc1739c5030a87a4a9318952a52d48cb4336d8` | WH-035 split packaging, installer protection and CI |
| `4885bc31de765b5271307bfb9f037fef6d24da3d` | WH-036 isolated production backend and worker privacy |
| `113090712ab7613bf6e385561be065aea7f24695` | WH-036 LocalSystem worker ownership requirement |
| `3f29602621ba469486186130bd87205c13d5585d` | WH-036 UI focus/status/sizing/real traffic polish |
| `222004f92c30552e52059e523badad268f34ab28` | WH-036 published XAML resources and build guard |
| `cde940d5b8bb766d8c226520097b17ac637b08b6` | WH-036 current acceptance and exact validation logs |
| `399102f0e3214e65763c9b3e05c01d226e40c15c` | WH-036 published branding content; final artifact source |

## Remaining acceptance and known limits

Follow `windows-v1-acceptance.md` for elevated install/upgrade/rollback/uninstall,
actual SYSTEM-owned pipe and fresh group tokens, two-account authorization,
real VPN/IPv4/IPv6/DNS leak captures, cleanup failures, sleep/wake/network changes,
visual/DPI/keyboard checks, upstream coexistence, and native ARM64 execution.
No usable owner test peer was supplied and the current process is not elevated.
Configured manager privileges (`SeChangeNotifyPrivilege`, `SeImpersonatePrivilege`)
still require installed-token verification to establish the validated minimum.

Continuous DNS health and uptime are not supplied by the current protocol; the UI
shows configured/unavailable states without inventing measurements. Updates are
manual MSI transactions. Redirected/nonstandard LocalAppData preferences require
manual owner cleanup. Candidates are unsigned. Remote CI execution and final
release signing/tagging/publication remain behind owner acceptance.
