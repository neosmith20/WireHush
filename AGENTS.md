# WireHush Agent Instructions

## Project Goal

WireHush is a simple, user-friendly VPN client. The first target is Windows. Android is planned only after the Windows implementation is stable and polished.

WireHush uses WireGuard as the underlying tunnel technology. Everything else in this document refers to WireHush itself.

Prioritize focused implementation, safe defaults, and maintainable code.

Do not add complexity unless it is required for correctness, security, compatibility, or a clearly approved user-facing feature.

## Core Behavior

WireHush must preserve ordinary tunnel behavior for standard configurations.

Examples:

```ini
DNS = 1.1.1.1
```

Must behave as normal DNS.

```ini
DNS = https://dns.example.com/dns-query
```

Must be recognized as DNS-over-HTTPS (DoH).

For DoH:

1. Bring up the tunnel.
2. Automatically bootstrap the DoH hostname when needed.
3. Establish the DoH connection through the active tunnel.
4. Use the DoH endpoint for DNS queries.
5. Do not silently fall back to plain DNS if encrypted DNS fails.
6. Restore networking state cleanly when the tunnel stops.

Bootstrap resolvers must have sensible built-in defaults and be configurable from a Settings area. The user should not be required to configure bootstrap DNS for normal use.

## Product Principles

- Preserve compatibility with standard tunnel configurations whenever possible.
- Do not change the underlying tunnel protocol.
- Do not reimplement tunnel cryptography.
- Reuse proven tunnel and platform integration code where appropriate.
- Keep encrypted DNS and related additions outside the tunnel protocol itself.
- Prefer automatic behavior with safe defaults over unnecessary configuration.
- Keep the UI simple and familiar, but make WireHush visually and functionally its own application.
- Avoid unnecessary dependencies.
- Avoid unnecessary background services, frameworks, runtimes, and telemetry.
- Never add tracking, analytics, advertising, or account requirements unless explicitly approved.

## Initial Windows Scope

The initial Windows milestone is limited to:

- Existing tunnel functionality
- Importing and editing standard tunnel configurations
- Plain DNS using IP addresses
- DoH using `DNS = https://...`
- Automatic bootstrap DNS
- Configurable bootstrap resolver settings
- DNS leak protection
- Clean tunnel connect/disconnect behavior
- Basic tunnel status and diagnostics
- Simple installer

Do not expand scope into ad blocking, DNS filtering, mesh networking, IDS/IPS, commercial VPN accounts, or unrelated networking features unless explicitly requested.

## Development Workflow

Before modifying code:

1. Read this file and `ROADMAP.md`.
2. Inspect the existing implementation before proposing replacements.
3. Prefer the smallest safe change that accomplishes the task.
4. Preserve all required upstream notices and licensing when importing or modifying upstream code.

For each meaningful change:

1. Build the relevant target.
2. Run existing tests.
3. Add focused tests for new parsing, DNS, bootstrap, routing, or failure behavior where practical.
4. Verify ordinary tunnel behavior has not regressed.
5. Document any behavior that could not be tested.

Do not claim a feature works unless it was actually built and tested or the limitation is clearly stated.

## Git Discipline

- Keep commits focused and descriptive.
- Do not mix unrelated cleanup with feature work.
- Do not rewrite shared history unless explicitly instructed.
- Do not force-push without explicit approval.
- Use feature branches for significant work.
- One coding agent should own a branch at a time to avoid concurrent-edit conflicts.

## Security and Networking Rules

- DoH certificate validation must not be bypassed in production code.
- Do not disable TLS verification as a convenience fix.
- Do not silently leak DNS outside the requested encrypted resolver path.
- Do not silently route around the VPN when tunnel-only behavior is expected.
- Fail closed for encrypted DNS unless an explicit fallback setting exists and the user enabled it.
- Treat configuration values, keys, endpoints, and logs as potentially sensitive.
- Never commit private keys, credentials, tokens, or real user tunnel configurations.

## Current Priority

The first engineering objective is to establish a clean, reproducible Windows baseline before adding features.

After that, implement DoH support incrementally:

1. Config parsing and model support
2. DoH transport
3. Bootstrap resolution
4. Tunnel-bound routing
5. Windows DNS integration
6. Failure and leak handling
7. Bootstrap Settings UI
8. Branding and UI polish
9. Installer and release packaging
