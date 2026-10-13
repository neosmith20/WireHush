# Task 009 — Settings UI for Bootstrap DNS

## Goal

Add a simple TunnelMint Settings area that exposes bootstrap resolver control without making ordinary users learn DNS plumbing.

## Required UI

Provide a Settings section for bootstrap DNS that allows the user to:

1. See the built-in bootstrap resolver list.
2. Enable/disable individual resolver entries.
3. Reorder enabled resolvers.
4. Add custom bootstrap resolver IP addresses.
5. Remove custom entries.
6. Restore the built-in defaults.

The initial behavior should use the configured list in order with failover. Do not add a complicated benchmark/latency-selection system unless the existing implementation already makes that trivial.

## Validation

- Bootstrap entries must be IP literals, not hostnames, to avoid recursive bootstrap.
- Reject malformed addresses clearly.
- Support IPv4 and IPv6 literals where the bootstrap engine supports them.
- Do not allow the user to accidentally leave encrypted DNS configured with no usable bootstrap path for a hostname endpoint without a clear warning/error.

## Persistence

Persist settings in a TunnelMint-owned location established by Task 005. Do not use the upstream application's registry/data locations.

Changes should apply predictably to subsequent tunnel activation. If safe live reload is simple, support it; otherwise clearly require reconnect rather than inventing a fragile hot-reload path.

## Security defaults

- No silent plaintext fallback option in the default UI.
- DoH TLS verification remains mandatory.
- Bootstrap remains limited to endpoint discovery.
- Do not add telemetry or external settings sync.

## UX

Keep the Settings experience consistent with the existing Windows UI. Advanced details may be tucked behind the Settings page, but the user should not need to visit Settings for normal default operation.

## Tests

Add tests for settings serialization/persistence, ordering, validation, restore-defaults behavior, and handoff into the bootstrap subsystem.

Run focused tests and `cmd /c build.bat`.

Create `docs/task-009-report.md` with UI behavior, persistence location, validation rules, tests, build result, and remaining unverified items.

Commit, push, and continue to Task 010.