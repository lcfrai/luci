### Task 144: the fake's W5 rules and conformance rows

**Slice:** W5. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 143 (SW.21: every W5 type, `WorkIntake`, the codes, the stubs). **Seam:** **SW.21**,
frozen here the way task 115 froze SW.20: after this task only added methods, fields or routes.
Consumed by 146-167 (each route passes these rows against the real Hub in task 168) and 169-173
(the desktop's contract tests run on this fake).

Task 143 made `*Fake` satisfy `WorkIntake` with 501s. Every W5 handler and surface is tested
against the fake first, so the fake must refuse what the Hub refuses, for the same reason, with the
same code. W5 is the first slice whose writers are **not all people**: a tracker's integration
principal, an approved app and an API key write too. This task gives the fake those principals,
writes the W5 rules once, in memory, and writes each rule as a conformance row, so task 168 runs
the identical rows against the Hub and a rule the two disagree on fails in one place.

**Anchors: the model is task 115** (`task-115.md:76-169` the state-beside-the-world pattern,
`:751-873` the rows' head, `:874-901` the table). Its reading of H's fake (`fake.go` `enter`,
`stamp`, `problem`, `badRequest`, `need`, at `feat/hub` `1271ddb`) is taken as written, not
re-read: the Hub worktree at `3df9fab8` has no `hubclient/fake.go` yet (checked 2026-09-26:
`control-plane/internal/hub/` holds `hubapi` only), so every fake helper below is **UNVERIFIED**.

**Read in this branch's parts, UNVERIFIED until merge:** task 143's types, `WorkIntake`,
`WorkIntakeCodeStatus` and `ActorKinds` (`task-143.md`); task 03's `FactStates`, `IsFactState`
and `MoveCauses` (`task-03.md:116-124`, `:211-219`); task 04's `fakeMoveAllowed`
(`task-04.md:861`), `workEvent` and `issueOf`; task 05's `WorkEnv`, `WorkRow`, `RunWork`,
`wRefused`, `wKinds`, `wHead`; task 15's `ImportBatch` idempotency (`task-15.md:175-177`); the
route owners' rules (146 decisions 1-8, `task-146.md:27-70`; 147's `Route`, `task-147.md:211`;
148's `CheckAnswers`, `task-148.md:271`; 156's `WebhookKinds`, `task-156.md:75`).

#### Decisions the scope left open, made here

1. **W5 state lives beside the world**, `intakeWorlds sync.Map` of `*fakeWorld` → `*fakeIntake`
   (task 115's decision 1), so no earlier struct gains a field.
2. **A principal is a view of the fake, not a user.** `(*Fake).ActingAs(a hubapi.Actor) *Fake`
   returns a copy of the client bound to the same world, and every call through it carries `a`.
   `actorOf` is a `sync.Map` of `*Fake` → `hubapi.Actor`; a `*Fake` with no entry is a person. The
   real Hub gets the same effect from the caller's token (tasks 150, 159, 160), so the rows need
   only `WorkEnv` constructors: `e.Integration(t, conn)`, `e.AppFor(t, user)`, `e.KeyFor(t, user)`.
3. **One guard, called from W1-W4's writers.** `guardActor(kind, move)` is the fake's copy of task
   145's trigger: `integration` may write no state and no revision, only `ApplySyncBatch`; `app`
   and `key` may make every hand move a person may and are refused every fact state and `done`;
   `agent` and `janitor` keep W1-W4's rules. Three call sites change (⚑): task 04's
   `MoveIssueState` and `WriteIssueRevision`, and task 05's `CreateComment`.
4. **Comments from a tracker come only through the batch.** An integration principal's direct
   `CreateComment` is 403 `forbidden` in both modes; `SyncBatch.Comments` is accepted in mode
   `in_and_comments`, each written with `author` the external author's name (task 153's
   `ExternalComment`), and in mode `in` the whole batch is 403 `forbidden` before any item applies.
5. **The quote's immutability is a shape row**, as task 115's decision 3: `WorkIntake` has no
   method that writes a request's `quote` or `requester` but `EraseRequest`, and a request reads
   byte-equal across `MarkImportant`, `AcceptRequest` and a rule change except for `important`,
   `state`, `issueId`, `closedAt` and `closedBy`.
6. **The harness rule is split.** The Hub refuses a policy naming a harness outside the catalogue
   with 403 `harness_not_allowed`, `details.harness`; refusing a *session* is the control plane's
   (task 166), which reads the policy, because the Hub starts no session. The row covers the Hub
   half only.
7. **The grid is the owner's recorded levels.** A loop version whose `bound.levels` sets any class
   above the owner's own level for that class is 403 `loop_outside_grid`, `details.class`. The fake
   reads the owner's grid from `w.agentGrid(userID)` (Phase T's autonomy record, **UNVERIFIED**
   name); an owner with no recorded grid has `ask_first` everywhere, so any level above it is out.
8. **Every write appends exactly one metadata-only event**, and no read appends; the event's data
   never holds a quote, an answer, an email address or a secret (SW.5's rule, W5's kinds).

#### Files

| Path (under `control-plane/internal/hub/hubclient/`) | Change |
|---|---|
| `fake_work_intake.go` | new: W5 state, `ActingAs`, the guard, all fifty-one `WorkIntake` methods |
| `fake_work_intake_stub.go` | **deleted** (task 143's), in the same commit |
| `fake_work.go` ⚑ (task 04) | `MoveIssueState` and `WriteIssueRevision` call `guardActor` first, two lines each |
| `fake_work_social.go` ⚑ (task 05) | `CreateComment` refuses an integration principal, two lines |
| `fake_work_intake_test.go` | new: fake-only tests (per-world state, the view, the guard table) |
| `conformance/work_intake.go` | new: `WorkRowsW5`, eighteen rows |
| `conformance/work_intake_fake_test.go` | new: `RunWork(t, fakeEnv, WorkRowsW5...)` |
| `conformance/work.go` ⚑ (task 05) | `WorkEnv` gains `Integration`, `AppFor`, `KeyFor` (decision 2) |
