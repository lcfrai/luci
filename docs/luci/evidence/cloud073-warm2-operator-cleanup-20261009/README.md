# Warm attempt 2: explicit operator cleanup

This capsule records one operator removal after the failed installed warm attempt 2 on source `786652704fa8bebd1ba1af41cfa1d414d7def100`. The original failed result remains bound by SHA256 `aa1b7d2976ad0c3425be604d3bf46394376adda06a3c6b164f64ff86735de3dc`. This operation does not establish Restart acknowledgement, remote POST settlement, automatic cleanup, lease release, or successful recovery.

The operation ran once from **2026-10-09 17:55:57.991186 UTC** to **17:56:01.096129 UTC**. It first checked the same Docker engine, exact container identity and ownership metadata, Created/non-running state, expected image/resources, unchanged fixture/evidence hashes, and five known process identities. It then issued one non-force `docker rm` for the exact retained container ID. The command returned exit 0 and that exact ID in 2.769881943 seconds; the ID was subsequently absent from daemon inventory, and exact inspection returned an explicit no-such-object result. No replay was performed while publishing this capsule.

The result records **31 fixture files and 71 original evidence files unchanged**. The lease-ledger SHA256 remained `ecbd4c2ee5b97005643503eb5d01d3c8d77736113eac643bb7d853fcf41f3a9a`; **one charged lease remains**, with zero checks, fences and check-fences. The original pending Restart and registry are unchanged. Before and after, three exact known process identities were absent and two same-identity browser processes were PID-1 zombies. These observations do not establish whole-machine absence.

Available root-filesystem bytes were observed at 2,029,010,944 before and 6,789,287,936 after the operation. This is an observed free-space change, not a measurement of that container's unique disk usage.

`before.json` and `result.json` are byte-exact receipts. The complete helper source is retained with one private relative fixture-path literal replaced by `fixtures/<PRIVATE_FIXTURE>`; all other source bytes are unchanged. This published helper is an evidence copy of a completed one-off mutation, not a replay instruction. `original-bindings.json` binds original and published bytes; `original-immutability.json` verifies those three originals stayed unchanged during publication. Private fixture contents, ledger contents, token values and failed-run private files were not read or copied by this publisher.

Verify the public capsule with `sha256sum -c SHA256SUMS`. Its integrity check does not repeat the cleanup or validate product acceptance.
