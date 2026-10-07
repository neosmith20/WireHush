# WireHush V1 migration and update policy

V1 installs its frontend and backend under `Program Files\WireHush`. Its demand
start LocalSystem manager is `WireHushManager`; independent tunnel services use
immutable TunnelIDs. Protected records, settings, logs and migration evidence
live under `ProgramData\WireHush`. Frontend preferences contain no configuration
and live under the current account's `LocalAppData\WireHush`.

Legacy names remain explicit migration identifiers. They do not rename official
WireGuard services, alter its registry keys, or import its configurations.
Legacy library compatibility constants remain available to reference code; the
production entry point sets the verified ProgramData root before running workers.

## Preserve-first migration

The installer's deferred LocalSystem action invokes `/migratelegacy` in the new
backend. This command accepts no source path or owner identity. It rejects other
Windows identities and is absent from the public UI RPC contract.

Only the known-folder roots `Program Files\TunnelMint\Data` and
`Program Files\WireHush\Data` are inspected. Directory/file reparse points and
file hard links are rejected. Legacy `.conf.dpapi` files use their original name
as the DPAPI description. Plain legacy `.conf` files are encrypted before backup.
Source files are never overwritten or deleted by migration.

1. Read, validate and back up legacy sources before stopping anything.
2. Stop only verified legacy services whose account, executable path, command,
   configuration directory and service name match the owned legacy installation.
3. Reread the stable sources after shutdown and preserve encrypted backups under
   `ProgramData\WireHush\Migration\Backups`.
4. Persist an encrypted journal reserving each new TunnelID before record creation.
5. Create each legacy tunnel as Shared, reload it and compare its full identity
   and configuration, then mark its journal entry complete.
6. Preserve valid current V1 settings. Otherwise migrate validated legacy
   bootstrap settings into the encrypted V1 settings store.
7. Remove verified stopped legacy services only after successful migration.

A retry after an interrupted completion reuses the reserved ID. Completed entries
do not overwrite later owner edits or recreate owner-deleted records. A changed
or missing unfinished source, namespace collision, unreadable ciphertext,
unverified service, or failed cleanup stops the affected operation; retained
sources/backups and journal evidence remain available for repair. Backup retention
has no automatic expiry. Explicit owner cleanup is separate.

The predecessor MSI may remove its old Program Files data during an upgrade.
The new installer must run and complete migration before `RemoveExistingProducts`;
encrypted ProgramData backups remain outside predecessor MSI ownership.
The old per-architecture UpgradeCodes are retained to recognize prior installations.
New component identities describe the new file locations.

## Updates and removal

V1 uses explicit installation of a verified installer. It does not invoke the
inherited WireGuard updater, download an update executable, or execute a browser
response. Test candidates remain unsigned unless separately signed through the
owner's signing setup; a hash is integrity evidence, not an Authenticode signature.

Upgrade, repair and silent uninstall preserve owned user data. Interactive
uninstall defaults to Keep Data. Explicit deletion applies only after protected
ownership/path checks and reparse rejection; an upgrade never applies deletion.
No operation targets official WireGuard paths, services, registry or adapters.

## Required owner acceptance evidence

Unit tests cover reserved-ID crash recovery, read-back failure, backup/journal
failure ordering, completed-entry idempotence, source changes and exact service
ownership rejection. These tests do not prove privileged MSI execution, legacy
LocalSystem DPAPI migration, rollback, real tunnel cleanup, or upgrade/uninstall.
Those scenarios must be exercised on disposable installed Windows test machines
before release acceptance. Native ARM64 execution is separate from cross-compilation.
