# Task 016 — Final Pre-Acceptance Cleanup

## Goal

Fix the remaining correctness and fail-closed issues found during the post-Task-015 review before owner/manual Windows acceptance begins. This is a small hardening pass, not a feature expansion.

Start from current `main` after PR #5 (`Harden pre-acceptance recovery paths`) and create a focused branch, preferably:

`codex/final-pre-acceptance-cleanup`

Read `AGENTS.md`, `ROADMAP.md`, `KNOWN_LIMITATIONS.md`, `TASKS/014-owner-windows-v1-acceptance.md`, and `docs/task-015-report.md` before changing code.

Work through every item below. Add deterministic tests for each fix where practical. Push the completed branch and open a PR, but do not merge it yourself. Do not start Android work and do not mark Task 014 owner/manual checks complete.

---

## 1. Preserve Windows host-route ownership across encrypted-DNS recovery

`tunnel/doh_integration_windows.go` tracks whether a DoH endpoint host route was created by TunnelMint in `ownedWindowsRoutes`.

Current recovery behavior can incorrectly downgrade an already-owned route from `true` to `false`: `dohruntime.Session.Recover` calls `AddRoute` for tracked routes, and if `luid.Route(prefix, nextHop)` reports that the route still exists, `AddRoute` currently stores `ownedWindowsRoutes[prefix] = false`.

That loses the fact that TunnelMint originally created the route. A later stale-route cleanup/refresh may then leave the TunnelMint-created Windows host route behind because `DelRoute` believes it is not owned.

Requirements:

- Reapplying/recovering a route that already exists must never erase prior TunnelMint ownership.
- A route that genuinely pre-existed TunnelMint activation must still never be deleted as TunnelMint-owned.
- If interface reconfiguration removed a TunnelMint-owned route and recovery recreates it, ownership must remain TunnelMint-owned.
- Route ownership changes must remain consistent when peer-route updates or Windows route operations fail.
- Add focused tests around ownership preservation/recovery. Refactor the route bookkeeping behind testable helpers if needed rather than relying only on privileged Windows integration tests.

---

## 2. Make adapter reinitialization prerequisites fail closed

`tunnel/interfacewatcher.go` now invokes encrypted-DNS recovery, but when the adapter is down the inherited reinitialization path still only logs errors from:

- `iw.adapter.SetConfiguration(iw.conf.ToDriverConfiguration())`
- `iw.adapter.SetAdapterState(driver.AdapterStateUp)`

and then continues into recovery.

That contradicts the Task 015 fail-closed goal. If restoring the base WireGuard configuration or bringing the adapter up fails, the tunnel service must not continue as though recovery could succeed normally.

Requirements:

- Propagate adapter configuration failure through `iw.errors` with the appropriate service error and stop processing that recovery event.
- Propagate adapter bring-up failure the same way.
- Do not call encrypted-DNS recovery after either prerequisite fails.
- Preserve existing plain-DNS behavior except for the improved error propagation.
- Add deterministic tests for the decision/order where practical. If direct watcher tests are impractical because of Windows callbacks/driver handles, extract a small testable helper for the recovery/reinitialization sequencing.

---

## 3. Keep refresh rollback ownership consistent when rollback cleanup itself fails

`dohruntime.Session.Refresh` correctly handles stale-route deletion failures after a successful proxy swap, but its earlier rollback paths still ignore errors while deleting newly-added routes after:

- DoH verification failure; or
- proxy transport replacement failure.

If `DelRoute` fails during either rollback, the newly-added route may remain live while `s.routes` still contains only the old route set. That creates the same live-state/bookkeeping mismatch Task 015 was intended to remove.

Requirements:

- Never silently discard a route-cleanup failure.
- If rollback cannot remove a newly-added owned route, retain that prefix in session ownership so `Close` or a later operation can retry cleanup.
- Return an error that preserves both the primary verification/proxy failure and any cleanup failure, e.g. with `errors.Join` where appropriate.
- Keep the old proxy/endpoint state when verification or proxy replacement fails.
- Add tests that inject `DelRoute` failure specifically during verification-failure rollback and proxy-swap-failure rollback, then confirm session ownership matches the simulated live state and `Close` retries cleanup.

---

## 4. Define safe mixed-DNS semantics before owner leak testing

The parser currently accepts a single `DNS =` list containing both plain DNS IP addresses and an HTTPS DoH endpoint. When any DoH endpoint is present, TunnelMint enters encrypted-DNS mode, but `configureInterface` can initially install the plain `Interface.DNS` addresses and the bootstrap firewall policy explicitly permits those addresses during activation.

For Windows v1, do not leave this ambiguous. TunnelMint's stated encrypted-DNS behavior is fail-closed with no silent plaintext fallback.

For Windows v1:

- Reject a configuration that mixes one or more plain DNS server IPs with a DoH URL in the same interface configuration.
- Continue allowing DNS search suffixes alongside DoH.
- Reject more than one DoH endpoint at parse/import validation time because the runtime currently supports exactly one per tunnel. Do not let such a configuration import successfully only to fail later on activation.
- Error messages should clearly tell the user what is unsupported.
- Preserve normal all-plain DNS configurations unchanged.
- Preserve one-DoH-plus-search-suffix configurations and round trips.
- Add parser/round-trip tests for allowed and rejected combinations.

Do not add a plaintext fallback setting in this task.

---

## 5. Ensure CI actually runs the new endpoint-resolution tests

`.github/workflows/windows.yml` now runs the Task 013/015 hardening packages, but the focused `./conf` test regex still omits the Task 015 `ResolveEndpointsWith` tests that verify controlled peer-hostname bootstrap behavior.

Requirements:

- Update the focused `./conf` test expression so `TestResolveEndpointsWithUsesExplicitResolverAndPreservesPort` and `TestResolveEndpointsWithFailsClosed` run in GitHub Actions.
- Include the new mixed-DNS validation tests from item 4 in CI as well.
- Keep environment-bound DPAPI/protected-storage/real-adapter tests out of hosted CI unless they can run deterministically without elevation.
- Confirm the Windows validation workflow passes on the pushed branch.

---

## 6. Review only — do not over-engineer endpoint refresh

`dohruntime.Session.Refresh` exists and is tested, but Windows v1 does not need a new periodic background refresh subsystem in this task. Keep automatic endpoint refresh/re-resolution out of scope unless a current production caller already requires a correction for correctness.

The owner acceptance pass can validate normal tunnel reconnect/recovery behavior. If real testing later demonstrates a need for periodic/retry re-resolution, record that as a separate focused issue rather than expanding Task 016.

---

## Required verification

At minimum run and report:

```text
go test -vet=off ./bootstrap ./dnsproxy ./doh ./dohruntime ./product
go test -vet=off ./tunnel ./tunnel/firewall ./manager ./ui
go test -vet=off ./conf -run 'Test(DNSParsing|DNSRoundTripPreservesDoH|DNSURLValidation|FromWgQuick|ResolveEndpointsWith)'
cmd /c build.bat
cmd /c installer\build.bat
```

If the environment prevents Windows Installer ICE validation, report that explicitly; do not treat it as a source-code failure and do not claim ICE passed.

After push, verify the GitHub Actions Windows validation workflow succeeds and that it includes the new Task 016 deterministic tests.

---

## Completion report

Create `docs/task-016-report.md` containing:

1. branch and implementation commit(s);
2. each issue above and how it was fixed;
3. tests added/changed;
4. exact local test/build results;
5. GitHub Actions result;
6. anything still unverified or environment-bound;
7. explicit statement that Task 014 real-machine acceptance remains open.

Do not claim Windows v1 beta-ready or production-ready. Do not merge the PR yourself.

## Exit criteria

Task 016 is complete only when:

- recovery cannot lose TunnelMint's Windows host-route ownership;
- adapter reinitialization prerequisite failures stop/fail closed instead of merely logging;
- failed refresh rollback cannot leave untracked live endpoint routes;
- mixed plain-DNS + DoH configurations and multiple DoH endpoints are rejected clearly for v1;
- CI runs the controlled peer-hostname-resolution and new DNS validation tests;
- deterministic tests/builds pass; and
- Task 014 remains the next and only owner acceptance gate.
