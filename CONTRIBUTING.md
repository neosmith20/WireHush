# Contributing to WireHush

Thanks for helping improve WireHush.

WireHush is intentionally focused on a small, reliable user experience. Contributions should solve clearly defined problems with the smallest safe change that preserves compatibility and security.

## Licensing and contribution policy

WireHush is source-available under the [PolyForm Noncommercial License 1.0.0](LICENSE).

- **Bug reports, feature suggestions, logs, testing results, and general feedback** may be submitted freely without accepting the Contributor License Agreement.
- **Code, documentation, artwork, tests, or any other copyrightable contribution** intended for inclusion in WireHush requires acceptance of the [Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md) ("CLA").
- Contributors retain copyright in their original Contributions, but the CLA grants the WireHush Project Owner permanent, worldwide, irrevocable rights to use, modify, distribute, sublicense, relicense, and commercially use those Contributions.
- Those rights include use in free, source-available, proprietary, paid, hosted, bundled, appliance, enterprise, or other current or future WireHush offerings.
- A contributor is not entitled to royalties, ownership in WireHush, or other compensation solely because a Contribution is used.
- If a Contribution may be owned in whole or in part by an employer, client, school, or another entity, the contributor must disclose that before merge. Authorization from that entity may be required.
- WireHush may decline any Contribution whose ownership, provenance, licensing, or third-party rights are unclear.

For GitHub pull requests, the CLA may be accepted electronically using the acknowledgement in the project's pull request template. Maintainers must not merge an external copyrightable Contribution until the required acceptance has been recorded.

## Before Contributing

Please:

1. Read `README.md`, `AGENTS.md`, `ROADMAP.md`, `SECURITY.md`, `LICENSE`, and `CONTRIBUTOR_LICENSE_AGREEMENT.md`.
2. Search existing issues and pull requests before starting duplicate work.
3. For anything beyond a small fix, open an issue first to discuss the approach.
4. Keep changes focused. Large unrelated refactors should be discussed before implementation.
5. Do not submit private keys, credentials, real tunnel configurations, personal data, or other secrets.
6. Security vulnerabilities must be reported through the process in `SECURITY.md`, not disclosed publicly before remediation.

## Development Expectations

Contributions should:

- preserve ordinary WireGuard tunnel behavior unless a change is explicitly required;
- avoid unnecessary dependencies and background services;
- preserve required third-party copyright and license notices;
- include focused tests when practical;
- build successfully for the affected target;
- avoid disabling TLS validation, leak protections, or other security controls to make a test pass;
- avoid unrelated formatting or cleanup changes in feature commits.

## Pull Requests

A pull request should clearly describe:

- what changed;
- why the change is needed;
- how it was tested;
- any known limitations or untested behavior;
- any new dependency, license, or third-party code introduced.

Keep commits reasonably focused and descriptive.

## Third-Party Material

Do not submit code or assets copied from another project unless its license permits the proposed use and all required notices are included. Identify third-party material clearly in the pull request.

See `THIRD_PARTY_NOTICES.md` for the project's third-party notice policy.
