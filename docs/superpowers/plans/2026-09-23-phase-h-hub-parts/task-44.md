### Task 44: the document amendments — the contract made true to what was built; the seams handed over

**`feat/hub`, last. Depends on: 43. About 1.5 hours.**

ADR-H1 §7.2 lists amendments *"each made by the phase that next edits the file"*; this phase is
that phase for four documents. And this plan made ten corrections to the Hub contract (A1–A10) and
several additions, which must be folded into the contract **before** another phase reads it and
builds against the old text — the failure this program has paid for a dozen times.

**Files:**
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` (modify)
- `docs/PROGRAM.md` (modify ⚑ — other phases edit it too; additions only)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` (modify: one note in §4.5)
- `docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` (modify: one amendments block at the top)

#### What changes, exactly

**The Hub contract.** Each change is made **in place**, at the section it corrects, and marked
`(Phase H plan, A<n>)` so a reader can trace it; a new §16, *"Amendments from the Phase H plan"*,
lists them in one table with the task that built each:

| Section | Change |
|---|---|
| §7.4 | A1: the two repo-key regexes and the lowercase rule; the `repo_already_linked` message sentence (task 17) |
| §12.1 | A2: Postgres-only compose; `devstack.sh`; why |
| §9.4, §13 C3 | A3: the exempt routes are `/v1/healthz`, `/api/v1/healthz` and the attach WebSocket |
| §9.4 | A4: `Access-Control-Allow-Headers: Content-Type, Authorization`; the key check inside the origin guard |
| §2, §6.4 | A5: `internal/devissuer` as a package, `cmd/devissuer` its `main`; three dev tenants and five users (task 11) |
| §6.1 step 9, §9.1 | A6: the `id_token` checked without a signature under OIDC Core §3.1.3.7(6), and the tenant cross-check; `AfterSignIn` returns the Hub tenant id |
| §4.2, §6.3 | A7: `ErrKeySetUnavailable` → 503 `unavailable` |
| §9.1, §9.3 | A8: the three credential routes |
| §2 | A9: `hubapi` may import `httpx`; `hubclient` may import `httpx` and `ids` |
| §3.6 | A10: `teamId` in the audit envelope; `hub verify` checks `feed`; golden vectors 3 and 4 (task 02) |
| §9.2 | additive: `Teams`, `Projects.Project`/`AddRemote`, `Designs.Approval`/`SessionLinks`, `Releases.Release`, `ErrSignedOut`, `ErrNotConfigured`, `ErrTokenUnavailable`, `Invalidator`, `WriteError`, `VerifyRevision`, `FormatRecorder` |
| §9.3 | additive: the `Panacea-Auth-State` header (task 36); the hub-link and team routes (task 35) |
| §6.2 | the refinement: redemptions serialised across scopes (task 31) |
| §7.9 | an overdue reservation reads `expired` before the janitor runs (task 23) |
| §15 | remove what this plan verified: `opener:default` permits `https` URLs; `x/sys/windows` has no Cred wrappers; the pgx pin and the `go 1.23.0` directive |

**`PROGRAM.md`.** The identity row of *Decisions locked* (`PROGRAM.md:18`) gains: *"The control plane
runs the flow and holds the tokens; the Hub validates Entra access tokens itself — no broker
(ADR-H1 §5.2, §6.1)."* The Phase H paragraph (`:94-95`) gains one sentence naming the plan and its
seams: *"Plan: `docs/superpowers/plans/2026-09-23-phase-h-hub.md`; seams S-CRED (the credential
store phases 3, 6 and 7 use), S-TOKEN, S-HUB, S-LINK, S-FEED."* Nothing else in the file changes.

**UI contract §4.5.** A note: *"Superseded for REST by the local caller key (Phase H plan, tasks
37–38): every REST call carries `Authorization: Bearer <local key>`; the attach WebSocket does not."*

**Provenance contract draft.** One block at the top, *"Amended by Phase H (ADR-H1 §7.2)"*, listing:
§2.1's store locations are now the Hub's; §1.4 and §4.1 gain the `erased` state; §6.4's *"head refs
are not polled"* is replaced by the change feed (tasks 26, 36, 40); §6.6's open exposure is closed
by the local caller key (tasks 37–38); A5 is widened to the whole machine (ADR-H1 §5.4). Each line
points at the Hub contract section that now holds the rule. The draft's body is not rewritten: its
owner phase does that.

#### Tests to write first

Documents have no unit tests; the checks are mechanical and are the acceptance below. One
executable check is added to `control-plane/internal/hub/hubapi/keys_test.go`:
`TestTheContractsRepoKeyRegexIsTheCodes` — it reads `docs/superpowers/contracts/2026-09-23-hub-contract.md`,
extracts the two repo-key patterns from §7.4, and asserts they equal `githubKeyRe` and `adoKeyRe`'s
sources. It kills the drift that correction A1 had to fix once.

#### Steps

- [ ] **Step 1:** Re-read ADR-H1 §7.2 and this plan's corrections table.
- [ ] **Step 2:** Make each change above; the new test.
- [ ] **Step 3:** Commit by pathspec — the four documents and the test file.

#### Acceptance

```bash
grep -c "(Phase H plan, A" docs/superpowers/contracts/2026-09-23-hub-contract.md          # ≥ 10
grep -n "^## 16" docs/superpowers/contracts/2026-09-23-hub-contract.md                      # present
grep -n "no broker" docs/PROGRAM.md                                                         # the identity row
grep -n "Amended by Phase H" docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md   # present
go test ./control-plane/internal/hub/hubapi/ -run TestTheContractsRepoKeyRegexIsTheCodes -count=1              # ok
```

#### What this task does not claim

- That sibling phases have re-read the amended contract: the orchestrator tells them; a phase plan
  written against the old §7.4 regex or the old §12.1 compose needs its own correction.
- Any change to the ADR's decision: ADR-H1 stands; only the contract's text moves.
