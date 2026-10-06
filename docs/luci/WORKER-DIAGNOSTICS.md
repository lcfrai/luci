# Private worker failure diagnostics

Managed user and team workers retain the newest 128 classified API failures in
`<worker state directory>/diagnostics/api-failures.json`, with a 64 KiB total
limit. The file survives normal worker restarts. It is separate from project
files, provider homes, agent mounts and the public audit ledger. Only the host
operator can inspect it through the host filesystem; no API serves it.

Each record contains a UTC timestamp, a fixed operation code and a fixed cause
code. For example, `native_state_prepare` identifies provider-home preparation.
Unknown operations or causes are `unclassified`. Raw error messages, paths,
headers, queries, request bodies, commands, environment values and credentials
are never retained in this file. Worker stdout and stderr remain discarded.

Records are bounded and atomically replaced with private permissions (0700/0600
on Unix; a protected current-user DACL on Windows). Linked, oversized or invalid
existing diagnostic storage is refused and preserved. If diagnostics cannot be
written, the original API error and workspace behavior remain unchanged; the
absence of a record does not prove that no failure occurred. Diagnostics are
best-effort operational evidence, not a complete audit or transaction journal.

Workers use their own existing state roots: team workers under
`control-plane/teams/<team>/runtime/state`, and personal workers under
`control-plane/users/<user>/runtime/state`. The standalone API does not enable
this recorder. Retention requires no new service, database or configuration.
