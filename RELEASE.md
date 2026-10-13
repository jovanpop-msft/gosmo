# Release notes

The current release, in brief. Detail and history are in
[CHANGELOG.md](CHANGELOG.md).

## v0.0.17

### New

- Full-Text Search: catalogs, stoplists, search property lists, indexes, populations, scripting.
- Replication (read-only): distributor, publications, articles, subscriptions.
- Replication Monitor reads: publishers, agents, sessions, errors.
- Live Query Statistics: `QueryProfiles`, `InFlightPlan`, `SessionLoginTime`.
- `ApplyPermission` — one GRANT/DENY/REVOKE for any securable class.
- `BlockedRequests`, `SetEnabled` on every enable/disable handle, `ErrInvalidRequest`.
- `ScriptDatabase` writes file layout and database options.
- `BackupSetType`, `Login.UserMappingsIn`, `SplitName`, rule and default definitions.

### Fixes

- Reordering Agent steps with a "go to step N" action rolled back.
- Schedule start date drifted by a day across time zones.
- Extended properties with a NULL value failed the whole listing.
- Histogram keys of NULL, binary and date types rendered as Go artifacts.
- Files of 2 TB and larger scripted as `Msg 102`.
- `LOGINPROPERTY` "never" dates came back as a 1900 value.
- Query Store wait reports counted executions once per category.
- Reads on `…Ref` handles queried id 0 and answered "no children".

### Changes

- **Breaking:** permission writes are `ApplyPermission`; per-class Grant/Deny/Revoke removed.
- **Breaking:** change tracking and signatures are methods on `Table` and the module handles.
- **Breaking:** `DatabaseFile` is `*DatabaseFileInfo`; `BackupType` is `SetType`.
- Argument refusals wrap `ErrInvalidRequest`; `ErrUnsupported` text is "not supported".
- Dependencies: Go 1.27.2, `azcore` v1.23.3.
