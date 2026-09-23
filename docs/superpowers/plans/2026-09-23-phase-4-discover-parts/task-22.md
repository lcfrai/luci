### Task 22: Contract amendments — Go, UI, provenance

Phase 2's two contracts and the provenance contract draft say things this phase has changed. A fact
changed in one place and left standing in another is how two sources come to disagree (ruling R3's
clause 5). This task edits **only documents**, after the code tasks it describes have merged, and
each amendment names the task that made it true.

**Before editing a section, open it and quote its current sentence in the commit message.** The
section numbers below are the ones the provenance contract's §8.2 and the Hub contract cite; this
plan did not open every one of them (§11).

**Files:**
- `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` (modify)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` (modify)
- `docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` (modify)

**Go contract amendments:**

| Section | Amendment | Made true by |
|---|---|---|
| §5.1 (session create order) | step "materialise" between resolve and record: `Deps.Materialiser.Materialise`, refusal `409 brief_not_written` writes nothing | task 10 |
| §3.9 (create request) | `contextRef` ignored; `withoutBrief` added | task 10 |
| §3.14 (restart) | optional body `{withoutBrief}`; `contextRef` restamped | task 10 |
| §3.2 (error codes) | `brief_not_written`, `project_not_shared`, `fanout_too_large`, `branch_not_found` | tasks 10, 11 |
| §10a (route audit) | `RouteMounter`; the Discover GETs and their reasons | tasks 10–12 |
| §7.2 (`writeAtomic`) | lifted to `statepath.WriteAtomic` with a `Mode` | task 01 |

**UI contract amendments:**

| Section | Amendment | Made true by |
|---|---|---|
| §3.1 (`CockpitState`) | `discoverSlice`, `provenanceSlice`, neither persisted; `PERSIST_VERSION` unchanged | task 16 |
| §4 (client) | create, resume and fork do not send `contextRef`; `restartSession(id, opts?)`; `createTransport`; `onHubCursor` | tasks 15, 16 |
| §4.2 (problem codes) | the codes of task 15; `classify` unchanged | task 15 |
| §4.7 (poller) | the header of task 16: one `listSessions` per tick; on a good tick at most one batched refs call and one feed call | task 16 |
| §6 (AppShell, pane header) | the `discover` arm; the chip on every pane header | tasks 17, 18 |
| §11 (the vitest config flag) | closed: phase 4 never wrote `vitest.config.ts` | task 15 |

**Provenance contract amendments:**

| Section | Amendment | Made true by |
|---|---|---|
| §1.2 | brief ids are the Hub's `bf-` + 16 hex; the grammar is the Hub's plus `none:declined` | task 06 |
| §1.3 | a revision's content is `discover.Content` v1; the Hub's envelope and digest | tasks 03, 07 |
| §1.4 | `Ref` as task 06 declares it; `contextPath` removed; superseded is `revision < latestCompleteRevision` (P4-B) | tasks 06, 16 |
| §2.1 | the brief store and ledger are the Hub's (`OD-3`); nothing is on the laptop | tasks 07, 08 |
| §2.4 | Materialise writes the latest complete branch revision, else the project's, else nothing (P4-A); `none:declined` | tasks 07, 09, 10 |
| §3.1 | the paths of task 12; `unavailable` slots | task 12 |
| §3.3 | the target rule (P4-D); the content drift check | task 09 |
| §4.1 | eight states: adds `declined` and `erased` | task 17 |
| §6.4 | key sources (R6); the Hub change feed replaces "head refs are not polled" | task 16 |
| §8.1 | marked **done** row by row, each with its task | — |

**Steps:**

- [ ] **Step 1:** For each row, open the section and confirm the sentence still says what the row
  assumes. A row whose section has moved is updated here, not skipped.
- [ ] **Step 2:** Edit; each amendment is a dated paragraph beginning *Amended by phase 4, task
  NN:*, and the old sentence is kept struck through only where the contract's own style does so.
- [ ] **Step 3:** Commit the three documents by pathspec.

**Acceptance:**

```
$ grep -c "Amended by phase 4" docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md
(6, 6 and 9 respectively — one per row above)
```

**Does NOT claim:** the Hub contract's amendments (the Hub asks of §9 are the Hub's to write).
