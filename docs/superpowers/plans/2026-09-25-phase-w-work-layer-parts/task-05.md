### Task 05: the fake, part 2, and the Work conformance rows: relations, labels, policies, comments, subscriptions, imports, search, erase (**WS0**)

**Slice:** W1. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 04. **Seams:** defines **SW.2**, second half, and reaches start line **WS0**: when
this task is green, streams WP, WD1 and WD2 build every Work-backed handler and surface against
`hubclient.Work` and `NewFake()`. After it, a change to SW.1 or SW.2 that is not an added method,
field or route reopens this task and is announced (skeleton, *Start lines*).

Task 04 fixed the method set and part 1's rules. This task gives the other nineteen methods their
rules, deletes the stub, and writes the **rows**: one named assertion per rule, run against the
fake here and against the real Hub in task 16. The rows are the promise WS0 makes: a handler that
passes on the fake passes on the Hub, because a rule the two disagree on fails a named row.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`**, beyond task 04's: the conformance
design, `phase-h-hub-parts/task-05.md:21-73` (`Env{Admin, Member, Viewer, Outsider, TeamID,
Advance, ReservationTTL}`, `Group`, `Run`, rows named `group/row`), `:75-121` (the fake's `Env`
builder), `:209-218`; `hubapi/types.go:349-354` (`BlobContentTypes`), `:565-567`
(`EraseRequest`); `hubclient/fake.go:37-40` (`FakeUser.OrgAdmin`). The Phase W parts whose server
rules these are, read on `feat/work-layer` at `9c4a600`: task 11 (`task-11.md:140-200`,
`:340-372`, tests `:457-494`), task 12 (`:135-146`, tests `:462-492`), task 13 (`:122-139`, tests
`:524-533`), task 14 (tests `:346-357`), task 15 (tests `:646-664`), task 09 (`:306`,
attachments).

**Not yet on any branch, so UNVERIFIED until merge:** Hub task H05's `conformance.go` and its
blob store (`fake_designs.go`); tasks 11-15's `hubapi` additions. Where this part names one, it
says so.

Eight decisions the scope left open, made here.

**(a) The rows run through `RunWork`, beside `Run`, not inside it.** H05's `conformance.go` is a
Hub file; adding a group to its `AllGroups` would change its `TestRunNamesEveryRow` count and put a
Phase W edit in a Hub file. So `conformance/work.go` declares `GroupWork = "work"`, the rows, and
`RunWork(t, newEnv)`, which runs each row as `t.Run("work/<row>")` exactly as `Run` names its own.
One suite in shape, no Hub file touched. Task 16's `work_integration_test.go` calls `RunWork` with
the real Hub.

**(b) `WorkEnv` is `Env` plus one principal.** Erase is an org admin's (task 14:
`TestOnlyAnOrgAdminErases`), and `Env` has no org admin. `WorkEnv{Env; OrgAdmin hubclient.Hub}`;
a `nil` `OrgAdmin` makes the erase rows `t.Skip` and say so, as `Advance: nil` does. A second team
comes from `Outsider`: the outsider creates one, so no other new principal is needed.

**(c) Later tasks' names are the fake's own until those tasks land** (task 04's decisions (e)-(g),
the same way). Task 11's `blocked_by` and `duplicated_by`, `LabelColors`, `TriageExitRefused` and
its codes; task 12's codes and emoji rule; task 13's kinds; task 14's `org_admin`; task 15's import
bounds. Each is an unexported `fk…` constant or function with the later task's exact spelling, and
the rows assert the **strings**, so drift fails a row on either side. Tasks 11-15 each list
`fake_work*.go` ⚑ (tasks 11 and 12 already do, `task-11.md:64`, `task-12.md:56`) and swap the
fake's copy for `hubapi`'s.

**(d) The other end of a relation reads the inverse word** (task 11), stored once. `relates` is
its own inverse and stored once from either end; a repeat is 200 with the same head and no event.

**(e) Text anchors count UTF-16 code units of the named revision's description** (task 12,
`task-12.md:135`), with `unicode/utf16`; task 12's `hubapi.UTF16Len` replaces it.

**(f) Subscriptions follow task 13.** Create subscribes the creator, assign the assignee, a comment
its author and each mentioned member, each once; a row the person set (`manual`) is never changed
by a trigger; an unchanged `Subscribe` appends no event; an import subscribes nobody.

**(g) Erase follows task 14,** which is more exact than the scope: texts read `""` with `erasedAt`
set (the scope's *"resolves `erased`"* is `/v1/refs`'s word, task 09's resolver); ids, keys,
digests, numbers and the alias stay; a second erase answers zeros and still appends its event; an
erased issue refuses every write with 409 `conflict`; search skips it.

**(h) Import follows task 15:** 1-500 items, validated all before any is written, each in
`triage` whatever the team's default, `not_tied`/`imported`, `source: intake`, both texts
`author: H`, labels matched to the team's live labels by name case-insensitively (never created),
idempotent per `(project, source, externalId)`. `ExternalRef` has no `createdAt` until task 15 adds
it (additive), so the fake keeps none.

**Files** (under `control-plane/internal/hub/hubclient/`):

| Path | Change |
|---|---|
| `fake_work_social.go` | new: relations, labels, the team policy, comments, reactions, resolve, subscriptions; four of task 04's hooks |
| `fake_work_intake.go` | new: import, search, attachments, erase; the fifth hook, `erasedWrite` |
| `fake_work_social_test.go` | new: the fake-only tests, each with its mutant |
| `fake_work_stub.go` | **deleted** |
| `conformance/work.go` | new: `GroupWork`, `WorkEnv`, `RunWork`, the `W-NN` rows |
| `conformance/work_fake_test.go` | new: `RunWork` against `NewFake()` |

No other file changes: task 04's `fake_work.go` calls the five hooks by name, and they move from
the stub to these files unchanged in signature.
