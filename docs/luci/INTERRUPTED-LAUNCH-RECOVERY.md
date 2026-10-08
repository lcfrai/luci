# Recovering an interrupted task launch

This records the implemented recovery contract. Installed and native results require their own exact package receipts; this document does not establish them.

## What this recovery supports

An isolated swarm task whose Docker Create/Start outcome became uncertain can be explicitly retired only if its original durable execution lease recorded **fence protocol 1**. That marker is captured for new launches while the cross-process child lifecycle lock is held. It is never inferred from an old session or added retroactively.

The initial implementation requires a local Unix Docker socket or verified local Docker Desktop named pipe, a Linux-container engine, and the original recorded engine and Luci instance identities. Remote TCP, unsupported backends, changed engines, missing/corrupt private ownership, and ambiguous or renamed original containers refuse recovery. A live allocator holding the lifecycle lock refuses recovery promptly; killing the controller alone does not prove the old Docker request finished.

Recovery may forcibly remove the exact positively owned original worker by its immutable container ID. It then creates and verifies an inert scratch container at the original deterministic name. Its empty image, command `/`, disabled network, readonly root, dropped capabilities and other verified restrictions prevent it from becoming an agent. The retained name blocks a late old Create; a late old Start addresses the removed original immutable ID. No image pull or replacement task is part of recovery.

Only a successful Docker Create acknowledgement plus exact fresh inspection supplies fence proof. A timeout, empty inventory, name conflict, or inspection of an unacknowledged marker is insufficient. An explicitly continued lost-ack recovery can remove only its own exact unacknowledged inert marker and create a new acknowledged generation; a saved acknowledged proof is verified and reused.

## Operator controls

1. Open the build's **Agent team** and **Check team status**. Use **Inspect workspace output** to retain/review partial files; output inspection does not start a process.
2. Select **Stop uncertain launch and retain output** for the exact task attempt. The dialog identifies project, task, build revision and attempt; its capability read has no runtime effects. Current project Edit authority is required independently of Execute, provider access and billing.
3. If supported, explicitly acknowledge that Luci may stop the original worker and retain a permanent Docker marker, then confirm. The server rechecks current authority, private identities, lifecycle exclusivity and engine identity; the earlier capability response is not authorization.
4. **Pending** means the old launch is not yet proved blocked. **Fenced** means proof is retained but final retirement is incomplete. Treat either state as unavailable for another attempt. Following a lost response or partial failure, reload/check state and explicitly select **Continue launch recovery** / **Continue reviewed recovery** with the current revision. Reading, polling and reopening never replay the action.
5. **Finalized** records released execution capacity and retained output. Starting or queueing a fresh attempt remains a separate action with current execution permission and resource/provider admission. Recovery does not establish success, integrate changes, release task dependencies or verify the combined build.

**Pause new starts**, **Cancel remaining tasks**, and queue pause/cancel affect future dispatch; they do not stop already running agents. Ordinary session Stop does not establish settlement of an ambiguous launch. **Retain changes and retry** is for eligible ordinary stopped/unclaimed attempts; it cannot bypass a private unsettled lease. Guarded **pre-backend missing-child recovery** is a different proof/action and cannot retire an uncertain Docker Create/Start.

## Retained state and bounds

- Task checkout, partial files, captured context, operation history and private proof remain. Previous attempts stay inspectable. Recovery starts no replacement and deletes no task checkout.
- Permanent fence slots are capped at **64 in the shared execution ledger**, including interrupted intents and completed recoveries. Docker also bounds fence inventory on the selected daemon, including markers from other instances. Repeating the same exact intent uses its existing slot. There is no TTL, prune, ordinary Stop or reaper cleanup for these markers.
- The shared retirement ledger is independently capped at **4096 records**. Both caps can block a new recovery; existing exact recovery intent/proof is preserved. No routine UI operation increases or resets either bound.
- Marker configuration has a 6 MiB memory limit, 100 millicore CPU limit and PID limit 1, but the marker stays never-started. These are configured limits, not measured overhead or a lifetime storage guarantee. Host/guest disk pressure still applies to new work; do not describe retained markers as free.
- Current per-action backend time budgets are 3 seconds for identity, 30 seconds for proof construction/inspection, and 10 seconds for final verification. Expiry leaves uncertainty retained; it does not release capacity.
- Public operations expose recovery state/timestamps. Actor attribution, daemon/container IDs, private intent and proof stay private. Ordinary Docker list/reaper handling excludes fences; finalized Exited rows do not cause routine deletion attempts. Generic Get returns a distinct retained-fence error, not absence.

## Concrete limits to disclose

**Legacy ambiguous protocol-0 launch:** unsupported by this retirement action because old delayed name-based deletion could erase a new marker. Upgrading Luci, waiting, stopping a visible container, clearing a registry row, or retrying cannot turn it into protocol 1. Preserve the lease, files and evidence. An operator can inspect them, but this implementation provides no supported automated engine-fencing/migration procedure that proves an old request impossible and reclaims that reservation. A guarded pre-backend claim may separately qualify for the existing missing-child proof; do not call every protocol-0 claim permanently unrecoverable.

**64 permanent slots exhausted:** the next new recovery refuses before stopping its worker or releasing its reservation; continuing an existing intent remains possible if its proof can be established. There is no safe in-app reset/garbage-collection or documented verified migration procedure. The operator can diagnose the retained state, not reclaim a slot by deleting a marker or editing the ledger. This is a bounded safety limitation and an unmet long-term local actionability gate, not a solved recovery path.

**Unavailable/changed engine or lost required marker:** recovery refuses and preserves output/state. Restoring the original verified engine/owned marker state may permit an explicit retry; no replacement-engine acceptance or absence-only override exists. Do not instruct the user to prune Luci containers or clear execution state to fix capacity.

The current UI correctly explains retained markers, separate consent, pending/fenced/finalized states and unsupported launch behavior. API documentation describes these boundaries. The dialog does not proactively show numeric permanent-slot usage/cap. Separate messages identify the 4,096-retirement and 64-marker limits; both new retirement-bound regressions pass. The “host operator must inspect” sentence is diagnostic guidance, not an implemented safe reclamation procedure.

## Verification scope

Controlled backend/API/product checks and a tiny real-Docker fence test cover individual mechanisms; source-hashed integrated suites are running separately. The installed fault driver must still demonstrate process death, delayed real Create/Start denial, lost acknowledgement, durable continuation and a subsequent useful task against the exact package. Native Windows launch recovery and named-pipe/process-lock behavior remain explicit acceptance gates until exercised on Dylan's machine. No mocked or Linux-only result establishes those Windows outcomes or model-quality/market-leadership claims.

Implementation anchors: `backend/launch_fence.go`; `backend/dockerbe/launch_fence.go`; `api/execution_fence_store.go`; `api/swarm_fenced_recovery.go`; `product/project_swarm_fence_recovery.go`; `desktop/src/components/journey/SwarmLaunchRecoveryDialog.tsx`; `docs/luci/API.md` “Isolated agent teams”.
