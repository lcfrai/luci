# LOCAL-009: completion observation fairness

A slow first task could consume the complete shared observation budget while the registry page cursor had already wrapped. With four eligible rows, each later turn selected that same first row and never attempted the other three.

The observer now retains at most one page of eight **unattempted metadata candidates**. It removes a candidate only when its attempt starts and resumes the remainder before requesting another page. The existing 128-position scan limit, three-second shared budget, exact-generation checks and report/ownership guards are unchanged. No registry schema or task authority changed.

The [verification receipt](verification.json) preserves the same four-project regression failing on the original implementation and passing after the correction. The fixture uses actual manual session HTTP handlers, persistent registry/private state, isolated Git/context, and the real reaper with controlled backend states. A deliberately slow first collector waits for the turn deadline; after restart, the other three reports are captured while the slow task remains pending. Queue bytes and capacity-ledger bytes stay unchanged; no extra worker, acceptance, integration or verification is inferred.

Final affected race verification: **139 passing test outcomes, no failures or skips**, with 675 Go/module input hashes unchanged during each command. The only input difference between the before/after batches is the production observer correction; the regression bytes are identical. The original expected failure, exact commands/timestamps and raw-log hashes remain in the receipt. Both batches used dirty source based on `f829a5e5a9c25e6e97e90e0d95884c0b738f88c2`; this does not bind a later commit automatically.

This is source-level controlled regression evidence on Linux. The original Windows report was source-confirmed, and no native Windows or installed Docker reproduction is claimed here. Diagnostic test durations are not a speed benchmark. Existing f829/CLOUD-018 evidence remains unchanged; matching-package and native verification are separate gates.
