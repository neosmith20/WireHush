# WireHush recovery validation — 2026-10-10

Status: backend and automated recovery validation passed; GUI visual and real mouse acceptance remains incomplete.

## Durable source and installation

- Repository: C:\Dev\WireHush on the connected TunnelMint build VM.
- Recovery branch: recovery/post-7c90364-20261010.
- Remote: https://github.com/neosmith20/WireHush.git
- Pushed branch ref: refs/heads/recovery/post-7c90364-20261010.
- Baseline: 7c9036412b4b02d83a82c778fdebfb3cf31d1ba1.
- Installed and fully regression-tested source: b8588650070e822e8c6b1a7e47dd7af35a21b51c.
- Installed UI and manager identify as 0.1.0+b8588650070e822e8c6b1a7e47dd7af35a21b51c.
- x64 payload: C:\Dev\WireHush\.artifacts\v1\x64\cc180142e5094dbc8b2aba8616ffb4e0\app.
- VM-only in-place payload installation into C:\Program Files\WireHush. All published payload file hashes match the installed files.
- Rollback copies retained under .artifacts/recovery-install-backup-dad356a and .artifacts/recovery-install-backup-b858865.
- No MSI was produced. The previously registered MSI product/version remains unchanged; this installation is a development payload replacement, not MSI upgrade acceptance.
- Existing history was not rewritten. C:\Dev\tunnelmint was not merged.

| Milestone | Commit |
| --- | --- |
| Authenticated local-user private tunnel access | b0d9872f466dfec8a7e385c247e90e435474bbbb |
| Serialized connected Save and exact stored configuration | 2c4d29b89828554791e62e37795abd9228456704 |
| Dialogs, bounded exit, Overview, icon resources and menu anchor | 7517817f53ce2fefbaddc6af5e3c03c40e207c09 |
| Current provenance and private-only capability handling | 9da6b8db84837620dee77ef5ea4ee3188b2add95 |
| Ungrouped caller and exact configuration round-trip tests | edc3531f1d7955be6e91e8be7a963199feab50a5 |
| Verify worker startup before reconnect success | dad356a9e4c217de85d93fe48245375484f0ce46 |
| Recover after explicit pre-network startup failure | b8588650070e822e8c6b1a7e47dd7af35a21b51c |

## Automated and installed-manager results

PASS: scripts/test-v1.ps1 -Race on b858865, including ordinary and production-tag manager suites, bootstrap, DNS proxy, DoH, runtime, configuration, protocol, tunnel/firewall, protected logging, TLS overlay regression, race suites, installer safety, UI traffic safety and diff checks.

PASS: focused authorization tests cover a caller without WireHush Users membership, anonymous pipe rejection, exact stored configuration round trip, private namespace isolation and shared operation denial. Production live tests ran as the current build-VM Windows account; a separate ordinary nonmember Windows account was not used.

PASS: installed manager imported the real private Psychz configuration from the owner-provided file. ReadConfiguration returned the exact saved text; no keys or configuration were included in this report or committed evidence.

PASS: real Psychz connected with a recent handshake.

PASS: connected Save persisted an edited configuration, disconnected/reconnected, and produced a recent handshake.

PASS: Save while disconnected restored the original configuration and left the tunnel stopped.

PASS: final reconnect succeeded. Original Psychz configuration was verified intact, Psychz remained connected, and no temporary recovery tunnels remained.

PASS: deliberate endpoint lookup failure in an isolated private temporary tunnel. The manager reported that configuration was saved but reconnect failed, and the saved modified configuration remained intact.

The failure test exposed an additional defect: a worker that failed before adapter creation was treated as a possible cleanup failure, blocking removal and recovery. Commit b858865 introduces an explicit service-specific pre-network failure marker. Failure evidence remains nonzero; only that marker permits safe removal/retry. Unknown failures, old raw error codes and failures after adapter creation remain fail-closed. The installed failure test passed again after rebuilding, including automatic temporary fixture deletion and restoration of Psychz.

The original pre-fix failed fixture required one administrative cleanup. Its exact service identity, installed executable, current-account ownership, stopped state and DNS lookup exit code were verified, and no corresponding adapter existed. Only that fixture service was removed; its record was then deleted through the manager. The failure evidence is retained.

PASS: installed Start Menu shortcut now targets WireHush.exe and uses Assets\WireHush.ico. Verification caught the earlier advertised shortcut resolving to the icon file after in-place modification; it was backed up and recreated as a normal application shortcut. Installer source already carries the explicit ProductIcon reference. Visual icon appearance and launch through Start Menu still require GUI acceptance.

## GUI evidence and remaining acceptance

The installed app was launched. Accessibility inspection confirmed Psychz Private Connected, Connected to Manager, explicit Receive (RX)/Send (TX), and Plain DNS (Windows resolver). No exit confirmation remained in the final app state.

Image capture failed with: "FrameArrived timed out: timed out waiting on channel".
A mouse action failed with: "coordinate input geometry is unavailable".
These are tool failures; they do not establish that the desktop was locked. The owner confirmed the desktop was already visible/unlocked.

Alt+F4 opened the expected exit confirmation explaining active-tunnel cleanup. Tab/Escape did not establish a verified final dialog action, so closing is not accepted as passed. The app was later closed during the controlled payload replacement while no tunnel worker remained and reopened afterward.

| Required GUI acceptance | Status |
| --- | --- |
| Add Tunnel end-to-end by real mouse | Pending |
| Import Tunnel(s) end-to-end by real mouse | Pending |
| Disconnect/Reconnect hit targets | Pending; installed RPC lifecycle passed |
| Existing editor populated with all saved details | Pending visual/UI confirmation; exact backend text passed |
| Save in the editor while connected | Pending mouse/UI confirmation; installed RPC lifecycle passed |
| Tabs | Pending |
| Tunnel menu anchored beside its button | Pending visual/mouse confirmation |
| Overview width, layout, colors, green indicator and Disconnect appearance | Pending visual confirmation; expected text found |
| Minimize/restore | Pending |
| X-close and explicit Exit | Pending, including manager-unavailable branch |
| Start Menu/app icon appearance | Pending visual confirmation |
| Diagnostics dialog provenance | Pending GUI confirmation; installed binary/source provenance verified |

The forgotten third owner defect remains OPEN / REPRO-PENDING. No reproduction or description was supplied.

Do not treat this report as completed mouse acceptance or as authorization to produce a clean-install owner-test MSI.

## Evidence

Sanitized logs are retained in docs/recovery-evidence-20261010:
- regression.txt
- build.txt
- live-lifecycle.txt
- reconnect-failure-before-fix.txt
- reconnect-failure-fixed.txt
- final-state.txt
- installed.json
- shortcut.json

Installed executable SHA256:
- WireHush.exe: FF2F157E520764D7B08C0CB3E4C3DE466CE8BBEE679BFEEE9AB06C3817B0FB38
- WireHush-Manager.exe: 0A6F5A7F11A128CF582A60D6C305C0D762976B2744FF17B7B2D8C8F774BC1C5F
