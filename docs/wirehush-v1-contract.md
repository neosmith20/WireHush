# WireHush V1 implementation contract

This contract resolves previously OPEN implementation details under the owner's
2026-10-07 authorization to define and implement conservative V1 contracts. It
supplements the locked Draft 0.1 blueprint; it does not replace or edit it.
WH-026 at `9391f12f98e35f5ff7e65704d1c271e4a9f64613` is the starting checkpoint.

## Identity and access

The existing TunnelID, record, service ownership, adapter identity, and WH-025
authorization rules remain authoritative. Private records remain private even
from a different-user administrator. Shared metadata and connect/disconnect are
available to WireHush Users; shared configuration, export, creation, editing,
deletion, and machine settings require an administrator. Imports default to
Private, derive OwnerSID from the authenticated token, and generate TunnelID in
the manager. Requests never select an OwnerSID, protected path, or new identity.
Scope/owner transitions are not supported in V1; capability negotiation reports
this honestly. Shared records may be created explicitly by administrators.

## Networking and sessions

V1 admits at most one active WireHush tunnel machine-wide, including pending or
unknown state. All network and record mutations pass through one manager gate;
waiting for the gate is cancellable. Start never disconnects another tunnel
implicitly. An unauthorized active tunnel is reported only as generic device
busy state. Authorization precedes all lookup, admission, and mutations.
Updating an active tunnel is refused; it must first stop successfully. Duplicate
names retain the accepted case-insensitive per-namespace rule.

Each authenticated connection is bound to its Windows token SID, effective group
membership, and session. One frontend per Windows session uses a session-local
single-instance mechanism. Closing one session cannot stop another user's private
tunnel or stop a manager still needed by another connected session. An application
exit stops only the caller's authorized active tunnel and keeps the UI visible
until cleanup completes. Closing IPC is never treated as intentional exit.
With no clients and no active tunnel, the manager exits after 30 seconds. Unknown
network state prevents idle exit. Manager recovery inventories SCM truth before
admitting mutations. No automatic reconnect across OS reboot is enabled in V1.

## Transport and protocol

One versioned `protocol/wirehush.proto` generates Go and C# bindings. gRPC HTTP/2
runs only over `\\.\pipe\WireHush.Manager.v1`; there is no TCP listener. Pipe ACLs
permit SYSTEM, Administrators, and WireHush Users only, deny anonymous/network
identities, and reject remote clients. Authentication uses Windows pipe-client
impersonation after reading the HTTP/2 preface, never client identity fields or
PID alone. Failure to authenticate or safely revert impersonation fails closed.
The UI validates the manager pipe's protected identity before sending secrets.

Handshake negotiates protocol major/minor and advertises only implemented
capabilities. Different majors fail. RPCs cover handshake, visible snapshot,
configuration read/export, private/shared create, identity-preserving update,
delete, start, stop, bootstrap settings, event subscription, and session exit.
Requests are limited to 1 MiB. Ordinary unary calls have a maximum 10-second
deadline; service startup and pipe connection use 10 and 5 seconds respectively.
Long-lived subscriptions are cancellable and heartbeat every 5 seconds. Cleanup
must be bounded, propagate failure, and preserve records on incomplete stop.

Use standard gRPC codes: InvalidArgument, Unauthenticated, PermissionDenied,
NotFound, AlreadyExists, FailedPrecondition, Aborted, DeadlineExceeded,
Unavailable, and Internal. Stable user-facing messages contain no raw config,
keys, owner SIDs, endpoints, protected paths, or other-user metadata. Cancellation
does not mean an operation was rolled back; clients resynchronize manager truth.

Events are visible snapshots with monotonically increasing manager-local revision
and a boot-instance ID. Traffic/handshake refresh is at most once per second while
subscribed. Slow readers do not block mutations; a bounded queue coalesces to the
newest snapshot. Reconnect uses bounded backoff, displays unavailable/unknown
state, and replaces cached state after a full authenticated snapshot.

## Storage, diagnostics, and delivery

Protected state remains service-context DPAPI with the accepted stable TunnelID
description and no LOCAL_MACHINE flag. Records/settings/logs/migration data live
in protected ProgramData. UI preferences and redacted UI diagnostics live in
LocalAppData. Logs never include configuration text, keys, credentials, or full
DoH URLs. Diagnostics exports omit tunnel names/IDs, owners, endpoints, addresses,
and configuration by default. No telemetry or automatic crash upload.

Legacy migration is idempotent and preserve-first: stop verified owned legacy
services, decrypt and validate sources, create Shared records, reload and compare,
persist a protected migration journal, and retain encrypted sources. Migration
backups have no automatic deletion; explicit owner cleanup is separate.

V1 updates use explicit owner installation of a verified installer; background
download/execution and the inherited WireGuard updater are excluded from the
production path. Upgrade and silent uninstall preserve data. Interactive uninstall
defaults to Keep Data; deletion requires an explicit installer property and must
validate ownership and reject reparse points before deleting.

Production outputs are the .NET 10/WinUI `WireHush.exe` and Go
`WireHush-Manager.exe`, plus required licensed dependencies, for x64 and ARM64.
The manager never launches/respawns a UI. Walk and legacy gob stay outside the
production dependency path when WinUI integration is accepted. Preserve the
approved visual-reference design, responsive window sizing, real data only, and
normal-user startup. Release builds disable development/sample modes.

## Sequential acceptance queue

1. WH-027: bounded service-removal waits and safe stop/delete completion.
2. WH-028: serialized manager mutations, single-active admission, and session rules.
3. WH-029: shared protobuf contract, generated bindings, and version/error model.
4. WH-030: authenticated local named-pipe gRPC transport with negative tests.
5. WH-031: manager RPC integration, events, recovery, lifecycle, and shutdown.
6. WH-032: approved WinUI frontend, real data, reconnection, and session exit.
7. WH-033: protected settings/logs, redaction, and diagnostics.
8. WH-034: preserve-first migration and explicit installer update policy.
9. WH-035: split binaries, installer/group/service ACLs, x64/ARM64 packaging and CI.
10. WH-036: final self-review, regression/build/install verification, UI polish,
    and exact owner-testing handoff evidence.

Each slice gets focused tests, applicable x64/ARM64 builds, diff verification, and
a focused commit only after passing. Privileged integration, real VPN traffic,
DNS leak capture, two-user isolation, installer upgrade/uninstall, sleep/wake,
network changes, and native ARM64 execution require actual evidence; compile-only
checks do not count. No final tag, publication, signing claim, or owner acceptance
is made before the final acceptance stage.
