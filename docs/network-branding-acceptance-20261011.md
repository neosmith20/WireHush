# WireHush Network Connections branding — October 11, 2026

## GitHub verification

GitHub origin is https://github.com/neosmith20/WireHush.git. Commit effbd4da6983b48f06463c8b367adabb8003b618 is present at refs/heads/ui/detail-presentation-20261011. It is not an ancestor of origin/main c1e773e02fdfa2163c04251441345b75b72def22. PR #21 merged the earlier 65ebf29 presentation commit; the later compact-spacing/capitalization fix still needs merging. No retry of the old push or history rewrite was necessary.

## Preservation

Local checkpoint branch checkpoint/validated-ui-effbd4d retains effbd4d. Branding branch starts at that validated UI commit. Installed baseline backed up under C:\Dev\WireHush\.artifacts\branding-baseline-effbd4d; baseline SHA256 inventory retained in branding-baseline-hashes.json. The existing untracked docs/recovery-acceptance-20261010.md was left untouched and excluded from commits.

## Ownership of the visible strings

The top ncpa.cpl line is the Windows connection alias, distinct from the private WireGuard adapter lookup name. WireGuardNT initially sets it through NciSetConnectionName. WireHush now calls that same Windows API with the GUID obtained from its freshly created adapter LUID. Unlike upstream's NciSetAdapterName collision fallback, this call never renames another adapter. NciSetConnectionName is a Windows API used by upstream, but is not a documented public application API; missing API or rename failure is logged and leaves the functional connection at its original alias.

The bottom description comes from per-device software/PnP metadata created by WireGuardCreateAdapter. Its TunnelType input is formatted as '<type> Tunnel' and used for DeviceDesc, FriendlyName and software-device description. Passing 'WireHush' gives 'WireHush Tunnel'; it does not require changing the signed INF or driver. The live device confirmed DEVPKEY_Device_DeviceDesc, DEVPKEY_Device_FriendlyName and DEVPKEY_Device_BusReportedDeviceDesc are all 'WireHush Tunnel'. Driver remains version 1.1.0.0 using oem18.inf.

Internal lookup uses DEVPKEY_WireGuard_Name ({65726957-7547-7261-644E-616D654B6579}, property 3). That remains WireHush-7849ea3dc98e4beeb499515ae3f61039, and the Psychz GUID remains {FAC3458C-3D49-95D5-D191-FB3ABC408800}. IDs, stored records, service names and manager lookups are unchanged. Script hook environment uses the actual alias after successful branding so interface operations still address the connection.

Branding applies only to record-backed WireHush tunnel creation (RecordLocator != nil). Legacy/upstream tunnel creation retains 'WireGuard' and does not enter the alias rename path. No official adapters are searched for or changed. No driver/DLL/INF binaries were patched. WireGuard DLL Authenticode remains Valid.

## Validation

Implementation commit: a0ed760e6391551741c0d859b33b693a40a7c03a.

PASS: full scripts/test-v1.ps1 automated suite, including collision fallback tests, ordinary and production manager tests, DNS/runtime/config/tunnel/firewall checks, protected logging, TLS overlay, installer safety, UI measurement safety and diff checks. Race option not run; no concurrency behavior changed.

PASS: x64 production manager build using overlay and wirehush_v1,load_wgnt_from_rsrc tags; installed manager reports 0.1.0+a0ed760e6391551741c0d859b33b693a40a7c03a. Installed manager hash equals built executable. Every other installed file matches the preserved validated baseline, including all UI files.

PASS: real Psychz connection, recent handshake, connected save/reconnect, disconnect, disconnected save without activation, final reconnect. Original reference configuration restored exactly. Final check verifies Psychz connected and no temporary tunnel fixtures remain. Live manager reads work with unchanged private runtime name despite the new alias.

PASS: actual ncpa.cpl window visually captured and inspected using Win32 PrintWindow after normal computer-use captures timed out twice. Top line is 'WireHush Connection'; bottom line is 'WireHush Tunnel'. Get-NetAdapter independently confirms both strings, original GUID and Up status. Evidence on VM: .artifacts/branding-ncpa.png, branding-live.txt, branding-regression.txt.

Only the manager executable was replaced; validated UI preserved and reopened. Deployment encountered transient remote transport loss during tunnel stop and delayed executable release during service stop; final installation/hash/live checks passed.

Limitations: an existing alias collision logs a warning and retains the generated name rather than mutating the occupying adapter or breaking tunnel connectivity. Collision handling tested with injected ERROR_DUP_NAME; no foreign adapter was renamed for testing. Real official-WireGuard co-install acceptance was not exercised. No clean-install owner-test MSI was produced.

Upstream references:
- https://git.zx2c4.com/wireguard-nt/tree/api/wireguard.h
- https://github.com/WireGuard/wireguard-nt/blob/master/api/adapter.c