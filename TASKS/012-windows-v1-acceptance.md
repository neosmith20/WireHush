# Task 012 — Windows v1 Acceptance and Release-Candidate Review

## Goal

Perform the final engineering validation for the Windows v1 work, fix defects found during review, produce a truthful release-candidate report, and open the final pull request from `codex/windows-v1` to `main`.

This task is not permission to call the product stable if required real-world verification is still missing.

## Full review

Review all changes from Tasks 003 through 011 together, not only Task 011.

Confirm that the final design still obeys:

- a straightforward connection workflow;
- normal `DNS = <IP>` compatibility;
- `DNS = https://...` encrypted-DNS semantics;
- no silent plaintext fallback;
- normal TLS certificate validation;
- DoH endpoint traffic through the active tunnel;
- narrowly scoped bootstrap DNS;
- clean teardown and rollback;
- coexistence with the installed upstream Windows client;
- no telemetry, tracking, filtering, ad blocking, or unrelated scope.

## Automated verification

Run all focused TunnelMint tests added during the task sequence plus the broadest upstream-compatible test set that can run reliably in the VM.

Run the normal Windows build and installer build from a clean checkout/state as practical.

Inspect for at least:

- accidental `InsecureSkipVerify` or equivalent production TLS bypasses;
- hard-coded test certificates, credentials, private keys, tokens, or real tunnel configs;
- unintended calls to the upstream application's update service;
- unexpected use of upstream service/data/installer identity;
- stale TunnelMint routes/firewall/DNS state after shutdown;
- required license/notice omissions;
- obvious debug-only behavior enabled in release-candidate artifacts.

## Windows VM acceptance checks

Perform all checks possible without external private credentials:

1. Install TunnelMint while the upstream WireGuard client remains installed.
2. Launch both independently and verify no manager/service collision.
3. Verify tunnel import/edit UI works with sanitized/generated test configuration data.
4. Verify plain-DNS configuration parsing/runtime path remains unchanged as far as the environment permits.
5. Exercise the local DNS proxy and DoH stack with local deterministic test infrastructure.
6. Verify encrypted-DNS activation rollback/failure behavior.
7. Verify DNS settings/routes/firewall state restore after disconnect and application/service restart.
8. Use Windows-native network observation where practical to verify steady encrypted-DNS operation does not emit unexpected plaintext DNS.
9. Exercise full-tunnel and split-tunnel route logic with safe synthetic/local fixtures where possible.
10. Exercise IPv4; exercise IPv6 where the VM/environment actually supports it.
11. Exercise sleep/wake or equivalent service/network restart behavior if the VM platform permits reliable testing.
12. Install/uninstall/reinstall the release-candidate package and check cleanup/coexistence.

## Real tunnel verification

If a usable test VPN peer/config is available in the development environment, additionally verify:

- connect/disconnect;
- handshake;
- traffic flow;
- plain DNS through a real tunnel;
- DoH through a real full tunnel;
- DoH through a real split tunnel;
- no unexpected plaintext DNS during those sessions.

If no real test peer/config is available, **do not invent one, use public credentials, or copy owner secrets into the repository**. Mark those items as owner/manual verification required and continue all other validation.

## Documentation

Create/update:

- `docs/task-012-report.md` — detailed engineering results;
- `docs/windows-v1-acceptance.md` — concise acceptance matrix with PASS / FAIL / NOT TESTED and evidence;
- `KNOWN_LIMITATIONS.md` — only genuine current limitations, not speculative future features;
- `ROADMAP.md` — check off only items actually completed and tested.

Fix failures that are safely fixable within Windows v1 scope, rerun affected tests, and document the final results.

## Final artifact

Produce a release-candidate Windows installer/build for owner testing. Record exact artifact filenames and hashes in the report.

Do not commit build artifacts to Git unless the repository's release process intentionally requires them.

## Final pull request

After all automatable Windows v1 work is complete:

1. Push the final `codex/windows-v1` branch.
2. Open a pull request to `main` titled along the lines of `Complete TunnelMint Windows v1 implementation`.
3. Summarize Tasks 003–012, test results, security properties, installer status, and every remaining owner/manual verification item.
4. Do **not** merge the final pull request yourself.
5. Stop after opening the PR and report the PR number/URL and release-candidate artifact locations.

## Completion criteria

Task 012 is complete when the Windows v1 implementation has been reviewed as one system, all automatable failures within scope have been addressed or truthfully documented, the release-candidate build is ready for owner testing, and the final PR is open for owner review.

Do not begin Android work.