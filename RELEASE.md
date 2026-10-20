# Release notes

The current release, in brief. Detail and history are in
[CHANGELOG.md](CHANGELOG.md).

## v0.1.0

### New

- `tsql` package: quoting, comment-safe names and a lexical scanner, standard library only.
- Extended properties in table, view, routine, schema, type and database scripts.
- `Database.EncryptionKey` (TDE), `Server.AvailabilityDatabaseCandidates`, `RecoverFromRestoring`.
- Differential base on `DatabaseRecoveryStatus`; `IsDistributor`, `SizeOnDiskKB`, `Shrink`.
- More database options, `TableDetail.Owner`, `Column.Collation`, bound rules and defaults.
- `VerifyOptions`, `SupportsBackupCompression`, `ChangePasswordOptions` policy flags.

### Fixes

- A panic in `InTransaction` rolls back and propagates.
- Forced drop, rename and detach keep the access mode and handle RESTORING databases.
- Snapshot file names no longer collide; `ALTER COLUMN` keeps the collation.
- Table space parts sum to reserved; tempdb files read their real size.
- Scripts: renamed modules, index filegroups, masking functions, server trigger options.
- Permissions and dependencies cover every securable class and unqualified names.

### Changes

- **Breaking:** boolean mode parameters are named types (`Termination`, `TableDropMode`, `SessionScope`, …).
- **Breaking:** `VerifyBackup` takes `VerifyOptions`; `SecurableRef.Type` is `SecurableClass`.
- **Breaking:** `TableDetail.SchemaOwner` → `Owner`; `RequiredSynchronizedSecondariesToCommit` is `*int`.
- Scripts gain login server roles, indexed-view indexes and orphaned-user comments.
- Bad server ports are refused; `shopspring/decimal` v1.6.0.
