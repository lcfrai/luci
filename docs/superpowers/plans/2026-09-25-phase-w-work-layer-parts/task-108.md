### Task 108: `step.fold` on the issue; one ephemeral line; session, lane and preview links; the first-activity budget

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 107 (`whoLabel`, the card an agent line opens). Uses task 48's `IssueAgentLine`
(`task-48.md:317-363`), task 40's `AgentLine`, task 61's 10-second first sign on the row, the one
heartbeat (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts:64`, `BEAT_MS = 2400` at `:13`),
phase 3's OD-8 transcript adapter (`internal/transcript/claudecode`, phase 3 plan `:144`, its
task 17; **UNVERIFIED until merge**) and SW.8.

Three things an issue view says about each agent on it, all from facts:

1. **What it did, folded.** 02 `step.fold` (`02:320`) and AX §8.3 (`AGENTIC-UX.md:204`): *"the
   collapsed row carries command, duration, exit code and line count: `pytest -q · 43s · exit 1 ·
   812 lines`"*. Never the raw output, never a prose summary.
2. **Whether it is alive, on the beat.** One ephemeral line (02 `row.three`, `02:319`) whose
   liveness coarsens to `live` under 10 s, then `quiet 2m`, `quiet 14m`, and changes only when the
   one heartbeat ticks (05 §4.1, D I10; AX §8.8 *"Honest liveness only"*).
3. **The first-activity budget.** 02 `budget.ack` (`02:285`): 10 s from launch to the first sign
   of work, else the line says so as a fact, *"no output yet · 10s"*. Task 61 built it for Start
   on the row; this task gives every agent line on the issue view the same budget.

**Decisions made here, so nobody re-derives them:**

1. **Steps come only from the adapter, and the adapter is never required.** The fold reads a new
   control-plane read, `GET /v1/sessions/{id}/steps`, which asks phase 3's adapter for the
   session's shell tool calls. When the adapter declines (`*claudecode.Unreadable`: not pinned,
   missing, over size, another harness), the read answers `tier: "commits"` with the lane's
   commits and the session's exit, and the fold shows those; there is no error state (`OD-8`).
2. **The adapter delta is phase 3's file, extended.** `Adapter.Steps` is added beside
   `ToolCalls` (`task-17.md:119`) under the same pin, size and 1% rules, reading `Bash` tool calls
   and their results. Rule H2: if phase 3's adapter has not landed, this task waits. It is not in
   the skeleton's delta table (gap 1).
3. **A step is four fields and a command cut at 80 characters.** `command` (the tool call's own
   command string, cut with `…`), `durationMs` (the result's timestamp minus the call's),
   `exitCode` when the result states one, `lines` (the result's line count). The result's text is
   read to count lines and is never returned.
4. **The first sign is the earlier of the first step and the first observed output.** The control
   plane's `Session.lastOutputAt` (`desktop/src/store/types.ts:157`) is an observed timestamp, a
   mechanical fact AX §8.1 allows; it counts as a sign only once it is later than `createdAt`.
5. **Liveness words, one rule.** Under 10 s since the last sign: `live · last output 3s ago`;
   10-59 s: `quiet · under 1m`; then `quiet 2m`, `quiet 14m`, `quiet 1h 05m`. Computed from the
   beat's time, so the text can only change on a beat. No spinner, no `progressbar` role.
6. **A link renders only where its fact exists.** *open session* for the reader's own sessions
   (task 48's three store actions); the **worktree** link (the screen word for a lane, 05 §4.1)
   when the session has a `worktreeId`; *preview* only where phase 7's session record carries a
   URL (**UNVERIFIED** that it exposes one per session).
7. **Delegated agents get the line, not the fold.** A delegated agent is Fleet's managed session
   (TM-05); its liveness comes from TM-52's `PersonDetail.agents[].status` through task 107's card
   model, and it has no step fold, because the adapter runs beside a local transcript only (gap 2).

**Files:**
- `desktop/src/components/issue/StepFold.tsx` (+ `StepFold.test.tsx`) — the fold, `formatStep` (new)
- `desktop/src/components/issue/SessionLinks.tsx` (+ `SessionLinks.test.tsx`) — the links, the
  ephemeral line, `livenessWords`, `firstSign` (new)
- `desktop/src/components/issue/IssueAgentLine.tsx` ⚑ (task 48) — mounts both per agent (modified)
- `desktop/src/api/client.ts` ⚑ — `getSessionSteps(sessionId)` (modified)
- `control-plane/internal/transcript/claudecode/steps.go` (+ test) ⚑ (phase 3 task 17's package) —
  `Adapter.Steps` (new file in a sibling package; gap 1)
- `control-plane/internal/api/sessions.go` ⚑ (phase 2, P4 task 10), `routes_audit_test.go` ⚑ — the
  steps read and its catalog row (modified; gap 1)
