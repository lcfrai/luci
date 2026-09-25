### Task 174: W5's proof and results

Stream **W5** (intake and the ecosystem), about 2.5 hours of work beside the run itself (an
estimate, not a measurement). Depends on tasks **143-173**. Writes the W5 section of
`docs/verification/2026-09-25-phase-w-results.md` ⚑, owned jointly with tasks 70, 97, 113, 142 and
187 and appended in task order; task 70 creates the file, and task 113's W3 section is the shape
this one follows (a lead line naming the environment, the run, the rows, the suites, the owner's
look, what is still unproven).

`OD-27` is the rule: **proof per criterion, no prediction.** Every row this section marks closed
carries the command that was run, an excerpt of what it printed, and the commit it ran at. A row
with no output beside it is not closed, whatever a part or a person says.

**Decisions made here, so nobody re-derives them:**

1. **The environment is named, first line.** The run is on the local Hub stack through `devissuer`
   (Phase H's local issuer) with six fakes: mail, the git host (Azure DevOps and GitHub webhooks),
   the tracker (Azure Boards, GitHub Issues, Jira), Teams, the ingestion endpoint standing in for
   Sentinel, and a scripted CI step. The lead line lists every fake that stood in, with the Hub,
   control plane and desktop commits. A fake proves our side of a wire and nothing about the
   service behind it, and the section says so.
2. **The rows are re-derived, not copied.** `check-w5.sh` reads 05 §1's matrix and selects every
   row whose slice column names W5. On 2026-09-26 that gives **forty-one** (checked by running the
   selection over `05-design.md` while writing this part): the forty the scope lists (thirty-five
   whole rows and the W5 halves of L-198, L-200, L-201, L-220, L-239) **plus L-116**, whose slice
   is `W2 / W5` (`05-design.md:255`: *"Desktop and email digest, then Teams"*). Its W5 half is the
   Teams digest, closed by task 165; the scope's count of forty missed it (gap 1). If 05 has moved
   a row in or out, the check fails and the section is corrected before it is filed.
3. **Each row names its closing task and test.** The table below is the plan, taken from each
   scope paragraph's *Closes* sentence (`tasks-141-160.md`, `tasks-161-180.md`). Parts 143-173 are
   not written on 2026-09-26, so the test column names the behaviour the scope's test holds; step 1
   replaces each with the real test name from that part's mutant table, and the run pastes its
   `PASS` line. Six rows are **halves** (L-116, L-198, L-200, L-201, L-220, L-239) and say which
   half; L-138 and L-203 are PARTIAL by design and say why.
4. **One scripted run, sixteen frames**, each a step with its command, the observation to record
   and its time (`docs/verification/phase-w/w5-run.md`). The frames are the scope's run, in its
   order, one frame per clause.
5. **Observed, never inferred.** A frame's observation is what a person saw or a command printed,
   quoted. *"The rule routed it"* is not an observation; *"`GET /v1/teams/tm-…/requests`
   returned the request, and its `request.created` event carries `ruleId` and `ruleAuthor`"* is. Nothing is read from a harness's terminal
   (`OD-8`); what an agent or a loop did is read from the Hub's events and Fleet's records.
6. **The owner's look is its own row.** The W5 surfaces (the request row and panel, the form and
   rules editors, tracker connect and the sync log, loops, admin) are shown to the owner and the
   verdict is recorded in the owner's words. `feat/work-layer` is not merged onward before it
   (`OD-1`; `CLAUDE.md`: *"Visual acceptance is the owner's eyes"*).
7. **What stays unverified is listed by name**, the scope's seven: sign-in from a named MCP client
   against the real tenant (Entra has no dynamic client registration); a real Teams tenant, Azure
   DevOps, GitHub and Jira; a Sentinel workspace; phase 7's manifest for a release no Panacea
   deployment joined; Phase T's `worker spawn` honouring *Undo Start*; phase 6's per-loop spend;
   and the owner's visual acceptance of 170-173. Step 6 adds whatever parts 143-173 name when they
   are written.

**Files:**
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the W5 section (modified; task 70's file)
- `docs/verification/phase-w/w5-run.md` — the sixteen frames a person runs (new)
- `docs/verification/phase-w/check-w5.sh` — the coverage check (new)
- `docs/verification/phase-w/testdata/{w5-complete,w5-missing-output}.md` — the check's fixtures (new)
- `desktop/src/test/w5Sweep.test.tsx` — `expectNoPrediction` over every W5 surface in one file (new)

#### The rows W5 closes, and what closes each

Line numbers are `05-design.md`'s on 2026-09-26. *Closed by* is each scope's *Closes* sentence;
*the test* is the behaviour the scope's test holds, replaced by its real name in step 1.

| Row | 05 §1 | Closed by | The test that must pass |
|---|---|---|---|
| L-27 | `:136` | 148 and 170 | a required typed field left empty is refused by the Hub form, not only the page |
| L-28 | `:137` | 147 and 170 | a request names the rule that routed it, and its author, on the event |
| L-31 | `:140` | 149 | mail to the team's intake address becomes a request in `triage` |
| L-49 | `:163` | 147 and 170 | first match wins; a second matching rule does not fire |
| L-80 | `:204` | 157 | a Teams message becomes a request with its thread linked |
| L-81 | `:205` | 157 | a feature's channel receives the feature's events, metadata only |
| L-97 | `:231` | 157 | a view's changes post to the chosen channel |
| L-116, W5 half | `:255` PARTIAL | 165 (W2 half: task 94) | the Teams digest is sent at most once in 15 minutes under fake timers |
| L-132 | `:284` | 146 and 170 | the requester's quote is stored verbatim as `text·H` |
| L-133 | `:285` | 146 and 170 | accept makes an issue in `triage` that keeps the request's link |
| L-137 | `:289` | 157 (served by 146) | a request from Teams keeps its thread synced to the request |
| L-138 (PARTIAL) | `:290` | 149 | an intake mail body is kept as plain text; an attachment is not stored |
| L-139 | `:291` | 148 and 170 | a non-member files through the Entra-protected form and sees only their own request |
| L-140 | `:292` | 149, 157 | the requester's mail holds *shipped* with the release name |
| L-149 | `:306` | 164 | `panacea-release create` from a scripted CI step records a release |
| L-159 | `:321` | 166 | a team skill reaches the sandbox from the image or the team library; the harness is unmodified |
| L-160 | `:322` | 162, 163, 172 | an event trigger and a schedule trigger each fire one turn of the owner's Orchestrator, within their grid |
| L-161 | `:323` | 162, 163, 172 | a loop's edit makes a new version and keeps the old; *run now* logs one run |
| L-168 | `:330` | 163 | "on accept, Start" for a label fires once on accept; evidence stays a person's act |
| L-177 | `:339` | 155 | a PR-body magic word files an issue |
| L-178 | `:340` | 149 (mail), 157 (Teams) | intake from a thread keeps the quote |
| L-180 | `:347` | 166 | a session with a harness the team does not allow is refused |
| L-196 | `:363` | 161 | the Work MCP server in read-only mode serves `/mcp/read` and refuses a write |
| L-197 | `:364` | 161, 166, 173 | an MCP connector not on the allow-list is refused in the sandbox |
| L-198, W5 half | `:370` EXCEED | 155 (W1 half: Start) | the first push on `ep-142-slug` writes `started` |
| L-199 | `:371` | 154, 155 | an outside commit with `Fixes EP-142` links to the issue |
| L-200, W5 half | `:372` EXCEED | 154, 155 | a host commit webhook links an outside commit |
| L-201, W5 half | `:373` EXCEED | 154, 155 | an outside PR's merge writes `landed` as the mapped person, never `done` |
| L-202 | `:374` | 151, 152, 153, 171 | each tracker's item lands in `triage` as *not tied · imported* |
| L-203 | `:375` | 155 (PARTIAL on Azure DevOps, per its scope) | a linkback and an autolink post on GitHub; a linkback only on Azure DevOps |
| L-207 | `:379` | 157, 165 | a Teams ask reaches the sender's own Orchestrator only |
| L-215 | `:387` | 153 | Jira in one way; comments two way only, and a state move is refused and logged |
| L-220, W5 half | `:392` PARTIAL | 151, 152, 153 (W1 half: Linear and CSV) | Jira, Azure Boards and GitHub Issues import into `triage` |
| L-239, W5 half | `:421` PARTIAL | 167 (the audit exists) | the exporter posts the audit chain to the ingestion endpoint |
| L-244 | `:426` | 159 (API), 161 (MCP) | an unapproved app is refused on the Hub API and on MCP |
| L-245 | `:427` | 160 | a revoked key's next call is 401 `key_revoked` |
| L-249 | `:436` | 158 | the published docs and both generated clients match the route table |
| L-250 | `:437` | 159 | an app acts as a person; an app-only token is 401; only an org admin approves |
| L-251 | `:438` | 156 | an outbound webhook verifies by HMAC-SHA256 and holds no text |
| L-252 | `:439` | 160 | a key over its limit gets 429 with the rate headers |
| L-256 | `:448` | 161, 166, 173 | the harness allow-list and connector list are admin-only and enforced |

Rows W5 **serves but does not close**: L-148 (release events on the feed, task 156) and L-154
(warehouse sync, WL task 178). Task 168 closes no row alone; it is the path the Hub halves of
146-167 run through, and its end-to-end test is pasted under *The suites*.

#### Interfaces — complete: the W5 section's shape (appended to the results file)

```markdown
## W5: intake and the ecosystem — <date>

**Environment:** the local Hub stack through `devissuer`; fakes standing in: <mail, git host,
tracker, Teams, ingestion, CI: each named, with the one it stands for>. Hub at `<sha>`, control
plane and desktop at `<sha>`. <One sentence: what these fakes do not prove.>

### The run (docs/verification/phase-w/w5-run.md)

| Frame | Command or act | Observed (quoted) | Time | Commit |
|---|---|---|---|---|
| 1 form | … | … | … | … |

### The rows

| Row | Task | Test (real name) | Command | Observed (excerpt) | Commit |
|---|---|---|---|---|---|
| L-27 | 148, 170 | … | … | … | … |

### The suites

| Suite | Command | Result line (quoted) |
|---|---|---|
| Go | `go test ./control-plane/... -race -count=1` | … |
| Go, integration | `go test ./control-plane/... -race -count=1 -tags integration` | … |
| W5 Hub end to end | task 168's acceptance command | … |
| Desktop | `npm --prefix desktop test` | … |
| Types | `npm --prefix desktop run typecheck` | … |

### The owner's look

<the owner's words, verbatim, with the date; or "not yet shown">

### What is still unproven

- <each item of task 174's decision 7, one line each, with the test that would settle it>
```

Every `…` is filled from the run. A cell left `…` fails the coverage check below.

#### Interfaces — complete: `docs/verification/phase-w/check-w5.sh`

```bash
#!/usr/bin/env bash
# check-w5.sh: every W5 parity row in 05 §1 has, in the W5 results section, a task, a test, a
# command, an observed excerpt and a commit (OD-27). Exit 0 only when all do.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
MATRIX="${W5_MATRIX:-$ROOT/docs/superpowers/planning/2026-09-25-linear-parity/05-design.md}"
RESULTS="${W5_RESULTS:-$ROOT/docs/verification/2026-09-25-phase-w-results.md}"

rows=$(awk -F'|' '/^\| [0-9]+ \|/ && $(NF-1) ~ /W5/ { gsub(/ /, "", $2); print "L-" $2 }' "$MATRIX")
[ -n "$rows" ] || { echo "no W5 rows in 05 §1: the matrix moved"; exit 2; }

section=$(awk '/^## W5: /{on=1; print; next} on && /^## /{exit} on{print}' "$RESULTS")
[ -n "$section" ] || { echo "no W5 section in $RESULTS"; exit 2; }
table=$(printf '%s\n' "$section" | awk '/^### The rows/{on=1; next} on && /^### /{exit} on')

fail=0
for r in $rows; do
  line=$(printf '%s\n' "$table" | grep -E "^\| $r( |,|\))" | head -1 || true)
  if [ -z "$line" ]; then echo "$r: no row in the W5 section"; fail=1; continue; fi
  empty=$(printf '%s\n' "$line" | awk -F'|' '{c=0; for (i=2; i<NF; i++) { v=$i; gsub(/^ +| +$/, "", v); if (v=="" || v=="…") c++ }; print c}')
  commit=$(printf '%s\n' "$line" | awk -F'|' '{v=$(NF-1); gsub(/^ +| +$/, "", v); gsub(/`/, "", v); print v}')
  [ "$empty" = 0 ] || { echo "$r: $empty empty cell(s)"; fail=1; }
  printf '%s' "$commit" | grep -Eq '^[0-9a-f]{7,40}$' || { echo "$r: commit cell is not a sha"; fail=1; }
done
for e in $(printf '%s\n' "$table" | grep -oE '^\| L-[0-9]+' | tr -d '| ' | sort -u); do
  printf '%s\n' "$rows" | grep -qx "$e" || { echo "$e: in the section but not a W5 row of 05 §1"; fail=1; }
done
if printf '%s\n' "$section" | grep -q '…'; then echo "an unfilled … is left in the W5 section"; fail=1; fi
[ "$fail" = 0 ] && echo "W5 coverage: $(printf '%s\n' "$rows" | wc -l | tr -d ' ') rows, each with a command, an observation and a commit"
exit "$fail"
```

It is task 113's `check-w3.sh` with `W3` read as `W5`, and one change: the row match also accepts
a comma after the id (`| L-116, W5 half |`), because W5 writes halves that way. If task 70 or 113
has by then grown a shared check that takes a slice name, `check-w5.sh` is a one-line call to it
instead (task 113's gap 2).

`W5_RESULTS` points the check at a fixture for its own test: `testdata/w5-complete.md` (all
forty-one rows filled with a sample sha) and `testdata/w5-missing-output.md` (the same with
L-251's *Observed* cell empty). The first must print the coverage line and exit 0; the second must
print `L-251: 1 empty cell(s)` and exit 1 (mutant: a row marked closed from a self-report).

#### Interfaces — complete: `docs/verification/phase-w/w5-run.md`

```markdown
# W5 run: intake and the ecosystem

The local Hub stack through devissuer. Three identities: Dylan (member and team admin), Priya (an
org admin), and Sam (in the tenant, not a member of the team). Fakes: mail, git host, tracker,
Teams, ingestion, CI. Record every observation as quoted text or a pasted output line, with the
time (Commit Mono). Shorten with [cut], never with an ellipsis.

1. **Form.** Sam opens the team's Hub form, fills its required fields and submits.
   Record: the page's confirmation line and the `rq-…` id; then Sam's view of the team's requests.
2. **Rule.** From the change feed: the `request.created` event (task 147).
   Record: its `ruleId`, `ruleVersion` and `ruleAuthor` (the first matching rule), and the route.
3. **Accept.** Dylan accepts the request into an issue and ties it to criterion `c-2`.
   Record: the new `is-…` alias, its `servesCriteria`, and the request's `linked` issue.
4. **Shipped.** The issue lands (task 21's path); a release is reserved and confirmed (task 131).
   Record: the release name, then the fake mail outbox line to Sam containing *shipped* and that name.
5. **Trackers.** Connect the fake Azure Boards, GitHub Issues and Jira; each has one item.
   Record: the three rows in Triage, each reading *not tied · imported*, and their `(source, externalId)`.
6. **Refused state.** The fake Jira sends a state move on its item.
   Record: the sync log row (its reason text) and that the issue's state is unchanged.
7. **Outside PRs.** On each fake host, open a PR titled `Fixes EP-…` and merge it.
   Record: the link on the issue, and the `landed` event's actor (the mapped person) and cause.
8. **Magic word files.** A PR body on the GitHub fake carries the filing word.
   Record: the new issue's alias and its state `triage`.
9. **Webhook.** Subscribe the fake receiver; make one issue edit.
   Record: the receiver's HMAC check result and the payload's field names (no `title`, no `text`).
10. **Teams intake.** Send a message in the fake Teams channel to the app.
    Record: the `rq-…` id and the thread link on the request.
11. **Digest.** Drive the Desk through three changes inside 15 minutes (fake clock).
    Record: the number of digest posts the Teams fake received (expected by the rule: one).
12. **Teams ask.** Dylan asks from Teams; so does Priya.
    Record: each turn's Orchestrator owner from Fleet's records (each their own; never the other's).
13. **Apps and keys.** Priya approves the fake app; it reads `/mcp/read` and tries a write. A test
    key is driven past its limit.
    Record: the read's status, the write's refusal code, then the 429 and its three rate headers.
14. **Loop.** Dylan's loop "on accept, Start" for label `bug`; accept one labelled request.
    Record: the loop's run row (one) and its `turnId`.
15. **CI.** Run the scripted CI step: `panacea-release create …` with a team key.
    Record: the command's output line and the release record's id.
16. **Harness and audit.** Start a session whose harness is not on the team's list; then run the
    audit exporter against the ingestion fake.
    Record: the refusal line; the ingestion fake's count of posted events and the chain's last hash.

No step reads a harness's terminal output (OD-8). If a step cannot be run in the environment, write
**not run** and why; never a predicted result.
```

The frames name fakes by role because their packages belong to tasks 149-167, whose parts are not
written on 2026-09-26; step 1 replaces each role with the package and start command those parts
give. The route and event names quoted in *Record* lines are the scopes' own; each one is checked
against the task's part in step 1 and corrected here if it moved.

#### Tests to write first — `desktop/src/test/w5Sweep.test.tsx`

The sweep checks that **every** W5 surface's own test file calls `expectNoPrediction` (task 20's
helper), so a surface that skipped the helper is caught even though its props are not fixed here:
parts 170-173 are not written on 2026-09-26, so no W5 surface's props are known to this plan, and
the sweep renders none (task 113's sweep rendered the four whose props were known). The file list
is the four scopes' *Files* lines (`tasks-161-180.md:390`, `:425`, `:448`, `:479`).

```tsx
import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(__dirname, "..");
/** Every W5 surface and its owning task. A new W5 surface is added here by its task. */
const W5_SURFACES: [string, number][] = [
  ["components/intake/RequestRow.test.tsx", 170],
  ["components/intake/RequestPanel.test.tsx", 170],
  ["components/intake/IntakeRulesEditor.test.tsx", 170],
  ["components/intake/IntakeFormEditor.test.tsx", 170],
  ["components/intake/TrackerConnect.test.tsx", 171],
  ["components/intake/TrackerList.test.tsx", 171],
  ["components/intake/SyncLog.test.tsx", 171],
  ["components/loops/LoopList.test.tsx", 172],
  ["components/loops/LoopEditor.test.tsx", 172],
  ["components/loops/LoopVersions.test.tsx", 172],
  ["components/loops/LoopRuns.test.tsx", 172],
  ["components/admin/AdminHome.test.tsx", 173],
  ["components/admin/Apps.test.tsx", 173],
  ["components/admin/Keys.test.tsx", 173],
  ["components/admin/Webhooks.test.tsx", 173],
  ["components/admin/Deliveries.test.tsx", 173],
  ["components/admin/Connectors.test.tsx", 173],
  ["components/admin/AgentPolicy.test.tsx", 173],
];

describe("W5 sweep: no prediction on any W5 surface (OD-27)", () => {
  it.each(W5_SURFACES)("%s (task %i) exists and calls expectNoPrediction", (file) => {
    const path = resolve(SRC, file);
    expect(existsSync(path)).toBe(true);
    expect(readFileSync(path, "utf8")).toMatch(/expectNoPrediction\(/);
  });

  it("names every W5 surface once", () => {
    const files = W5_SURFACES.map(([f]) => f);
    expect(new Set(files).size).toBe(files.length);
  });
});
```

The number-prone W5 strings are the rate line (`Keys.tsx`: remaining calls against a limit, task
160's `RateLimit-*` headers), the sync log during an import (`SyncLog.tsx`), and the delivery log's
retry count (`Deliveries.tsx`). Each of those parts' own tests renders them through the helper; if
the helper reads one as progress, the words change in the surface, never the guard.

| Test | Mutant it kills |
|---|---|
| each W5 surface's test calls the helper | a surface that skipped `expectNoPrediction` |
| every surface named once | a pasted duplicate that hides a missing surface |
| `check-w5.sh` on `w5-missing-output.md` exits 1 | a row marked closed from a self-report |
| `check-w5.sh` on `w5-complete.md` exits 0 | a check that fails everything, so nobody runs it |
| `check-w5.sh` rows from 05 §1 | a row that moved in 05 and was not re-derived (L-116 is the case that proved it) |

#### Steps

- [ ] **Step 1: read on the sync base** the parts and code of tasks 143-173; fill the closing
  table's *test* column with each real test name, and replace each fake's role in `w5-run.md`
  with its package and start command. Read tasks 70's and 113's sections and keep the W5 section
  in the same shape.
- [ ] **Step 2: the tests first**: `w5Sweep.test.tsx`, then `check-w5.sh` with its two fixtures.
  `npm --prefix desktop test -- test/w5Sweep` fails until every W5 surface exists; the check on
  the missing-output fixture exits 1.
- [ ] **Step 3: name the environment** (decision 1), bring up the local Hub stack and the six
  fakes, and record the commits.
- [ ] **Step 4: run the sixteen frames** of `w5-run.md` and fill the run table, quoting each
  observation. An excerpt is shortened with `[cut]`, never with `…`.
- [ ] **Step 5: run the suites**, with and without `-tags integration`, and each of tasks
  143-173's acceptance commands; paste each result line beside its row.
- [ ] **Step 6: list what is still unproven** (decision 7 plus any a part added), and show the W5
  surfaces to the owner; record the owner's words, or *not yet shown*.
- [ ] **Step 7: run the acceptance commands.** Commit the results file, the run script, the check
  and its fixtures, and the sweep by path, each message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
bash docs/verification/phase-w/check-w5.sh
# expect: "W5 coverage: 41 rows, each with a command, an observation and a commit", exit 0
W5_RESULTS=docs/verification/phase-w/testdata/w5-missing-output.md bash docs/verification/phase-w/check-w5.sh; echo "exit $?"
# expect: "L-251: 1 empty cell(s)" then "exit 1"
npm --prefix desktop test -- test/w5Sweep
# expect: 19 tests pass (18 surfaces and the once-each check)
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
go test ./control-plane/... -race -count=1 -tags integration
# expect: ok for every package; no FAIL line
npm --prefix desktop test && npm --prefix desktop run typecheck
# expect: every suite passes; exit 0
grep -cE "^## W5: " docs/verification/2026-09-25-phase-w-results.md
# expect: 1
```

The row count is 41 on 2026-09-26; if 05 §1 moves a row, the expected line moves with it and the
section says why.

#### What this task does NOT claim

- That W5 is adopted, or that any team has left its tracker.
- Anything a fake stood in for: a real Teams tenant, Azure DevOps, GitHub, Jira, a Sentinel
  workspace, or a mail server. Each fake proves our side of its wire only.
- Sign-in from a named MCP client against the real tenant (Entra has no dynamic client
  registration).
- Phase 7's manifest for a release no Panacea deployment joined; Phase T's `worker spawn`
  honouring *Undo Start*; phase 6's per-loop spend.
- The owner's acceptance of 170-173 until it is recorded in the owner's words.

**Parity.** This task closes no row itself; it records the forty-one rows of decision 2 as closed
**only** where the rows table carries a command, an observation and a commit.

#### Gaps found while writing this part

1. **L-116's W5 half is claimed by no task.** 05 §1 gives L-116 the slice `W2 / W5`
   (`05-design.md:255`), so the W5 selection holds forty-one rows; the scope of 174 lists forty,
   and task 165's scope names the 15-minute digest without closing L-116. The table above assigns
   it to 165; 165's part should say so.
2. **Parts 143-173 are not written** on 2026-09-26. The closing table's tests, the fakes' packages
   and the event and route names in `w5-run.md` come from scope paragraphs and are **UNVERIFIED**
   against parts; step 1 is where they are checked.
3. **Six results sections, six copies of one check.** `check-w3.sh` and `check-w5.sh` differ in a
   slice name and one pattern. Task 70 (or 187) should own one `check-slice.sh <slice>`; task
   113's gap 2 says the same.
4. **`docs/verification/phase-w/`** and `src/components/loops/` are not in the skeleton's file map
   (its row `:314` has `intake` and `admin` but no `loops`; task 172's scope records that half).
5. **L-203 is MATCH in 05 §1** (*"Both hosts"*) but task 155's scope makes the Azure DevOps side a
   linkback only. The row closes on GitHub in full; 05 or 155 should say which is right.
