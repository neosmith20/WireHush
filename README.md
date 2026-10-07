# WireHush

**A simple, user-friendly WireGuard-based VPN client with smarter DNS.**

WireHush is an independent VPN client built around WireGuard with one core goal: **keep it simple and make the software do the work.**

The first release is being developed for Windows, with Android planned after the Windows client is stable and polished.

## Why WireHush?

WireHush aims to preserve a simple tunnel workflow while adding quality-of-life features that should not require users to understand the plumbing underneath.

The first major addition is transparent encrypted DNS support.

A normal DNS entry should continue to work normally:

```ini
DNS = 1.1.1.1
```

WireHush will also understand a DoH endpoint directly:

```ini
DNS = https://dns.example.com/dns-query
```

WireHush detects the HTTPS endpoint, bootstraps it automatically, and sends encrypted DNS traffic through the active tunnel. The implementation is still alpha software and awaits owner testing on an elevated Windows system with real tunnel peers.

No separate `EncryptedDNS=true` switch. No unnecessary configuration maze.

## Windows v1 scope

- Windows-first client
- Familiar, simple tunnel workflow
- Import existing WireGuard tunnel configurations
- Standard DNS support using IP addresses
- DNS-over-HTTPS support using `DNS = https://...`
- Automatic DNS bootstrap using sensible built-in defaults
- User-configurable bootstrap resolvers in Settings
- DNS leak protection
- No silent fallback from encrypted DNS to plain DNS
- Tunnel status, handshake information, and basic diagnostics
- Simple Windows installer

## Bootstrap DNS

Encrypted DNS endpoints use hostnames, so WireHush may need a traditional DNS resolver briefly to locate the DoH endpoint before encrypted DNS is available.

WireHush ships with multiple bootstrap resolvers for reliability, while allowing users to change, disable, reorder, or replace them in **Settings**.

Bootstrap DNS is only intended to locate the encrypted DNS endpoint. Normal DNS queries should then use the configured encrypted resolver.

## Project Philosophy

**KISS — Keep It Simple.**

The user should be able to:

1. Download WireHush.
2. Install it.
3. Import a tunnel.
4. Connect.

Advanced networking details belong inside the software whenever they can be handled safely and automatically.

## Project Status

> **Alpha/development software.**

WireHush is not currently ready for production use. Automated tests and reproducible development builds pass, while real Windows install/coexistence, service, packet-leak, sleep/wake, IPv6, and real-peer acceptance remain owner verification items.

See [ROADMAP.md](ROADMAP.md) for the initial development plan.

The split Windows V1 candidate uses an ordinary-user .NET 10 WinUI application
and a separate authenticated Go manager service. Current x64/ARM64 build and
installation instructions are in [the V1 build guide](docs/wirehush-v1-build.md).
Follow [owner acceptance](docs/windows-v1-acceptance.md) for the remaining elevated,
multiuser, real-network and native ARM64 tests. Earlier task reports describe
historical combined-client candidates and are not current release instructions.

## Project Policies

- [Contributing](CONTRIBUTING.md)
- [Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md)
- [Security Policy](SECURITY.md)
- [Support](SUPPORT.md)
- [Privacy](PRIVACY.md)
- [Commercial Licensing](COMMERCIAL_LICENSING.md)
- [Third-Party Notices](THIRD_PARTY_NOTICES.md)
- [Trademarks and Branding](TRADEMARKS.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Security

Please report security issues according to [SECURITY.md](SECURITY.md). Do not publish private keys, credentials, tunnel configurations, or exploit details in public issues.

## Licensing

WireHush's original project code is licensed under the terms in [LICENSE](LICENSE). Third-party components remain subject to their own licenses and required notices; those notices will be maintained in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) as dependencies are incorporated.

## WireGuard

WireHush uses WireGuard as its VPN tunnel technology while providing its own client experience and additional functionality around it.

WireHush is an independent project and is not affiliated with or endorsed by the WireGuard project.
