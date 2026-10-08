# WireHush V1 owner acceptance

The V1 implementation is a test candidate. Automated evidence is recorded in
`wh036-owner-test-handoff.md`; this checklist is the remaining acceptance gate.
Use disposable Windows test machines and test peers before production use.
Record OS/build, architecture, account roles, candidate hash, result, and sanitized
evidence for each row. A linked installer or passing fixture is not a runtime pass.

| Area | Required owner checks | Current evidence |
| --- | --- | --- |
| Fresh installation | Native x64 and ARM64 MSI; normal Start menu launch; no UI from SYSTEM; service demand-start; correct Program Files ownership; new logon after group assignment | x64 interactive fresh install passed on Windows 11 25H2 build 26200; fixed Program Files path, Start Menu, LocalSystem demand-start manager, SYSTEM-owned roots and group provisioning verified. Native ARM64 execution and fresh-token post-logon launch remain pending. |
| Authentication | Real SYSTEM-owned production pipe; ordinary group member succeeds; nonmember, anonymous, remote, spoofed server and additional pipe-instance attempts fail; process/session token binding; no TCP listener | Installed LocalSystem manager created `WireHush.Manager.v1`; service ACL and required privileges inspected; no manager TCP listener observed. Normal UI correctly reports stale group token. Fresh-logon group-member success and installed negative/multi-instance checks remain pending. |
| Authorization | User A's Private metadata/config inaccessible to user B, including B as administrator; Shared basic/control available to group; Shared configuration/edit/export and machine settings require administrator; identities immutable | Automated negative boundary tests pass; two-account installed checks pending |
| Concurrency | Simultaneous starts admit one worker; pending/unknown/failed cleanup blocks another start; other-account connection displays generic busy status; stale requests cannot cross session identities | Admission, lifecycle and race tests pass; installed stress pending |
| Lifecycle | Manager restarts recover SCM truth; client disconnect/crash preserves VPN; explicit sole-client exit waits for authorized cleanup; another live session preserves VPN; cleanup failure keeps UI open; idle manager stops after 30 seconds only with no clients/workers | State/session tests pass; installed final x64 manager stopped after the expected idle window with no authenticated clients/workers. Installed crash/reconnect, active-tunnel cleanup and multi-session lifecycle remain pending. |
| Storage | Service-context DPAPI, protected ProgramData; no ordinary-user config access; no keys/configs/SIDs/endpoints in diagnostics; reparse/hardlink substitutions fail; corrupt settings/journal fail closed | Protection/redaction/fixture tests pass; installed `C:\ProgramData\WireHush` is SYSTEM-owned with SYSTEM/Administrators-only DACL, and protected backend log content was unreadable from the elevated test shell. Fresh ordinary-user token inspection remains pending. |
| Networking | Plain DNS; DoH full/split tunnels; IPv4 and IPv6; actual handshake/traffic; bootstrap ordering; certificate/endpoint failure never silently falls back to plaintext; DNS/route/adapter restoration including retry failure | Parsing, TLS, proxy, runtime and rollback tests pass; real-peer packet capture pending |
| Windows transitions | Sleep/wake, Ethernet/Wi-Fi change, offline/reconnect, reboot, manager/worker crash and independently running upstream WireGuard | Owner execution pending |
| UI | Import/edit/export/delete; duplicate labels; Private/Shared visibility; keyboard navigation/focus; Help remains open during events; minimum 1100×680 DIP, default 1280×800 DIP; 100/150/200% scaling; resizing; real traffic; unavailable metrics remain honest | x64 installed normal-token launch stays open, reports stale access token clearly, and same-session second launch redirects to the original instance. Fixed-path installer wizard passed. Full visual/DPI/keyboard and live tunnel workflows remain pending. |
| Upgrade/migration | Predecessor TunnelMint/WireHush upgrade; encrypted backup/readback; reserved IDs survive interruption; repeated migration preserves owner edits/deletions; rollback restores service binding/data; foreign service collision refuses upgrade | Migration and installer safety fixtures pass; elevated transaction execution pending |
| Uninstall | Interactive Keep Data default; silent uninstall always keeps data; explicit interactive deletion warning and fixed-root cleanup; unsafe files refuse deletion; reinstall retained records; upstream product/data preserved | Final x64 candidate: silent Keep Data uninstall passed; retained SYSTEM-owned `backend.bin` kept the identical SYSTEM-context SHA-256 across uninstall/reinstall; explicit delete removed WireHush ProgramData/LocalAppData; custom delete-warning UI was traversed; earlier unsafe-owner sentinel refusal passed; upstream WireGuard and TunnelMint remained intact. |
| Privileges | Capture actual SYSTEM manager token and exercise auth, storage, settings, migration and SCM with the configured privilege list; verify worker driver privileges separately | Installed manager verified as LocalSystem with only `SeChangeNotifyPrivilege` and `SeImpersonatePrivilege` configured and unrestricted service SID. Fresh-token auth/storage flows and worker driver privilege validation remain pending. |
| Release | Run CI on committed source; verify both MSI hashes/notices; signing, native ARM64 execution and all owner rows before final acceptance | Final hotfix candidates built from committed source `5f14fece6cde1c0f94e1be33dc26af9fe409911f`; local full `-Race` validation and ICE-enabled x64/ARM64 builds pass; 499-file payload manifests, MSI hashes, PE architecture and executable metadata verify. Remote CI/signing/publication and remaining owner rows are pending. |

Install the MSI matching the native architecture with administrator privileges.
Then sign out/in so the installing user's `WireHush Users` membership reaches their
token. Launch WireHush normally, without elevation. Assign other test users to the
local group as administrator and give them fresh logon tokens. SYSTEM deployment
does not automatically add a human user. Start with a Private test configuration;
use a separate administrator session for Shared creation and machine settings.

During real-network tests capture both the physical adapter and tunnel traffic.
Distinguish the intentional DoH-hostname bootstrap queries from normal DNS queries.
Test unreachable resolvers, invalid certificates, proxy failures and failed cleanup
as well as successful connections. Capture original DNS/routes and compare after
disconnect. Do not infer leak protection from a successful connection alone.

The manager's candidate required privileges are `SeChangeNotifyPrivilege` and
`SeImpersonatePrivilege`; their presence is not proof of a validated minimum.
The UI does not invent continuous DNS health or uptime. Nonstandard redirected
LocalAppData preferences require manual owner cleanup. Updates are manual MSI
transactions in V1. The candidate is unsigned and no final release is tagged or
published. ICE03 and ICE61 are explicitly excluded; the remaining ICE checks run.

Final release acceptance requires completing this matrix, recording failures and
fixes, then rerunning affected automated and runtime checks on the final source.
