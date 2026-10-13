# Windows v1 Execution Queue

## Purpose

This file authorizes a coding agent to continue through the remaining Windows v1 work without waiting for owner approval after every individual task.

The current starting point is `main` after Task 002 has been merged.

## Execution model

1. Read `AGENTS.md`, `ROADMAP.md`, `SECURITY.md`, `THIRD_PARTY_NOTICES.md`, and all task files before beginning.
2. Create one integration branch from current `main`:
   - `codex/windows-v1`
3. Execute Tasks **003 through 012 in numeric order**.
4. The branch/merge instructions in this file supersede branch suggestions in the individual task files while this queue is being executed.
5. Make at least one focused checkpoint commit after each completed task and push the integration branch after every task.
6. Create/update the requested `docs/task-XXX-report.md` for every task.
7. Keep `ROADMAP.md` truthful. Check off only behavior that was actually implemented and tested.
8. Do not merge the integration branch into `main` during the queue.
9. After Task 012, open one final pull request from `codex/windows-v1` to `main` containing the complete Windows v1 work and acceptance report.

## Keep working without waiting

Do **not** stop merely because:

- one task has been committed;
- a checkpoint has been pushed;
- a task report has been written;
- a non-critical test requires a different test strategy;
- an existing upstream test is environment-dependent and unrelated to the changed behavior.

Fix issues found during self-review, document them, and continue when safe.

## Stop conditions

Stop and report a blocker only if continuing would require one of the following:

- a real secret, private key, credential, signing certificate, or paid/private service that is not already available;
- destructive changes to the owner's host or data outside the disposable development VM;
- weakening TLS validation, leak protection, privilege boundaries, or another security control;
- silently changing licensing terms or removing required third-party notices;
- guessing at behavior that cannot be tested safely and is a prerequisite for later tasks.

If an external/manual verification is unavailable but later engineering work can still be completed safely, document the unverified item and continue with the non-dependent work.

## Global requirements

- Prefer the smallest robust implementation.
- Normal `DNS = <IP>` behavior must remain compatible.
- `DNS = https://...` means DoH and must fail closed; no silent plaintext downgrade.
- Production TLS certificate validation must never be bypassed.
- DoH traffic must ultimately use the active tunnel, including split-tunnel configurations.
- Bootstrap DNS is only for locating the configured DoH endpoint and must never become normal user DNS.
- TunnelMint must coexist with an installed WireGuard Windows client without sharing TunnelMint service names, application data, IPC/singleton identity, updater behavior, or installer identity.
- Preserve all required upstream copyright/license notices.
- Do not add telemetry, accounts, ads, filtering, ad blocking, DoT, DoQ, or unrelated features.
- Do not begin Android work. Windows v1 must be owner-accepted first.

## Final deliverable

At the end of Task 012, the branch should contain a Windows v1 release-candidate-quality build, all automated work that can be completed in the development VM, a clear list of anything requiring owner/manual verification, and a final pull request ready for owner review.