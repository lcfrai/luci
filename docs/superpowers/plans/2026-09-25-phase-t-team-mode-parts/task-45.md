### Task 45: Hub delta: audit to Sentinel; the anchored chain head

**Slice TM1. Depends on: Hub task 27 (and reads Hub tasks 14, 20, 24; phase 8 tasks 04, 10; Phase
T task 44). About 2.5 hours. Seams: none new.**

The Hub's audit is a SHA-256 hash chain in its own Postgres (Hub task 14's `store.Append`, Hub
`task-14.md:140-178`), and `hub verify` walks it (Hub `task-27.md:48-61`). That catches one edited
row. It does not catch an owner who rewrites **every** row after a point, consistently: the chain
still links, and Hub task 27 says so (`task-27.md:115-116`). ADR-H1 promised the fix, "hash-chained
audit streamed to Log Analytics and Sentinel" (`ADR-H1:162-163`), and deferred the anchor to later
(`ADR-H1` §5.9 item 4, `:587-588`: "the chain head exported to Log Analytics hourly … phase 8's
DCR"). The team-mode design brings that later forward: audit leaves the building **before team mode
reaches anyone but the owner** (`05-design.md` §11.1, `:1132`), and the skeleton makes it part of
the gate before the first real teammate (skeleton `:1029`).

This task builds three things:

1. **The stream.** Every audit event, within about 15 seconds of its commit, is sent to the Log
   Analytics workspace of phase 8 task 04 through the Logs Ingestion API, as a metadata record:
   its org, `seq`, time, kind, team, project, subject, actor, `hash` and `prevHash`, and the
   **names** of its data keys, never their string values unless the pair is on a short enum list.
2. **The anchor.** Every hour, for each org, the chain head (`audit_heads.seq`, `hash`) goes to a
   second table. Sentinel is enabled on the workspace, and one scheduled rule fires when an org
   with events has no anchor for two hours, because a Hub that stops anchoring is the attack.
3. **The comparison.** `hub verify` gains an `anchor` check: every anchored head and every streamed
   event hash must equal the database's row at that `seq`, and the head must not be behind the
   newest anchor. A consistent rewrite that passes the `audit` check fails this one.

It is a delta on the Hub branch, built after the sync-merge of `feat/hub`, plus a small delta on
phase 8's `modules/observability` (task 04) and workload identities (task 10), after the
sync-merge of `feat/env`. Every Hub and phase 8 path below is **planned**, so each is **UNVERIFIED
until merge**.

**The one thing this task must not get wrong is that the stream carries no text.** Log Analytics
is a second store of the audit, readable by anyone with workspace read, and its retention is not
the Hub's erasure (Hub task 27, `:118-129`). Contract §8's `data` holds team and project names,
invite emails, branch refs and two free-text `reason`s (`brief.erased`, `release.abandoned`)
(Hub contract `:1137-1167`). So the mapping is an allowlist: a string value leaves only when its
`(kind, key)` pair is in `StreamEnums` **and** the value matches the enum grammar; integers and
booleans leave; everything else leaves as its key's name alone. `canonical` never leaves.

**The second thing is that the Hub cannot rewrite what it has sent.** The Hub's identity holds
`Monitoring Metrics Publisher` on the collection rule (send only) and `Log Analytics Reader` on the
workspace (read only), and nothing that deletes or purges. The phase 8 test asserts that set
exactly, so a later role added to `id-hub` fails the plan.

**Decisions this part makes.**

- **A cursor, not an in-transaction send.** A write must not wait on, or fail with, Azure Monitor;
  the streamer reads committed rows after a per-org cursor and advances it after a 2xx. A crash
  between the send and the cursor update sends a batch twice; records carry `(orgId, seq)`, and
  the check de-duplicates on it. The window an owner could rewrite an unsent row in is the stream
  lag, which the `anchor` check reports.
- **One replica streams and anchors,** by Hub task 24's advisory-lock shape (Hub `task-24.md:80-90`),
  with lock names `panacea-app-hub-audit-stream` and `panacea-app-hub-audit-anchor`.
- **Anchors are sent every hour even when the head has not moved,** so their absence is a signal.
- **Streaming is off unless configured**: `PANACEA_APP_HUB_AUDIT_SINK` is `off` (local, the
  default), `fake` (tests, dev) or `logs`. `hub serve` refuses to start with `off` when
  `PANACEA_APP_HUB_TEAM_MODE=on` (task 06's switch, **UNVERIFIED**: its name on the merged branch),
  which is how "part of the gate before the first real teammate" is enforced in code.
