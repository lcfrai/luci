### Task 25: Close the phase: `PROGRAM.md`, seams, asks, what stays unverified

The phase is not closed when the code merges. It is closed when the programme document stops describing a phase that no longer exists, when every seam and ask this plan leaves is written where the next phase will find it, and when everything still **UNVERIFIED** is listed in one place instead of scattered across twenty-four parts. This task edits two documents and writes no code.

**The one thing this task must not get wrong is claiming acceptance.** An ask is a request of another phase; writing it down does not mean it was accepted. A seam is a place the next phase plugs in; listing it does not mean anyone has. Every line this task adds says which it is, and none says "agreed", "done" or "confirmed" about work outside this phase.

**Files:**
- `docs/PROGRAM.md`: phase 6 rewritten (`:113-114`); the never-pool line amended (`:132`); open risk 2 retired (`:138`); one open risk added (modified)
- `docs/superpowers/plans/2026-09-25-phase-6-gateway.md`: "Seams and asks" brought up to date from every part's gap list; a new closing section, "At close: what stays unverified"; the re-scope note's task number corrected (modified)

**Interfaces:**
- Consumes: every part's **"What this task does not claim"** and **"Gaps against the skeleton"** sections; task 18's probe record; task 23's local record
- Produces: the programme's current description of phase 6; the phase's seam, ask and unverified registers
- Required by: phases H, 4, 7 and 8 (their asks), and the owner (the stale lines and the owner questions)

**Decisions where the skeleton is silent:**

**(a) Files other phases own are listed, not edited.** The skeleton names two stale lines outside this phase's reach: the root `CLAUDE.md`'s "per-user rotation" line, and phase 8's precondition. Checked 2026-09-25: this worktree's `CLAUDE.md:22` **already** carries OD-19's wording, so that line is stale only in the copy in phase 8's worktree (`.claude/worktrees/env/CLAUDE.md:22`), which updates when that branch next merges `main`. Phase 8's precondition is `.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:330` (*"Confirmation that users' own Claude subscriptions may be used from Azure egress IPs (`PROGRAM.md` open risk 2)"*), which this task's step 4 makes point at a retired risk. A third stale line, not in the skeleton, is in this tree: `docs/FUNCTIONAL-SPEC.md:42` (*"Sessions spend the signed-in user's **own** subscriptions, rotating as each 5-hour or weekly window exhausts …"*). The functional spec is the owner's document, so it too is listed for the owner rather than edited (gap 2).

**(b) The unverified register lives in the plan, not in `PROGRAM.md`.** `PROGRAM.md` is the programme's summary and carries open risks, not per-claim evidence. The plan's new closing section holds every UNVERIFIED item with the task that could verify it; `PROGRAM.md` gains one open risk that points there.

**(c) Gap lists are merged, not resolved.** Each part ends with gaps it found against the skeleton. This task copies each into the right register (a seam, an ask, an owner question, or a follow-up task) with its source part, and resolves none of them. A gap that an earlier task's executor already closed in code is marked closed with the commit that closed it.

- [ ] **Step 1: Preflight**

```bash
grep -n '### Phase 6 — Capacity and credential rotation' docs/PROGRAM.md          # expect :113
grep -n 'per-user subscription registration' docs/PROGRAM.md                       # expect :114
grep -n 'Never pool subscriptions across users' docs/PROGRAM.md                   # expect :132
grep -n 'Anthropic OAuth enforcement' docs/PROGRAM.md                             # expect :138
grep -n '^## Open risks' -A12 docs/PROGRAM.md | grep -E '^[0-9]+-[0-9]+\.' | tail -1  # the last risk number
test -f docs/verification/2026-09-25-phase-6-local-e2e.md && echo "task 23 recorded"
test -f docs/verification/2026-09-25-phase-6-litellm-probe.md && echo "task 18 recorded"
grep -c 'UNVERIFIED' docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/*.md   # the harvest, per part
```

If a line number differs, another change has landed on `PROGRAM.md`; edit by the quoted text, not the number, and record the new numbers in the commit body. If either verification record is missing, stop: the phase cannot close without them, and this task says so rather than closing around them.

- [ ] **Step 2: Rewrite phase 6 in `PROGRAM.md`**

Replace `docs/PROGRAM.md:113-114` (the heading and its one paragraph) with exactly:

```markdown
### Phase 6 — The inference gateway (LiteLLM)
*Re-scoped 2026-09-25 (`OD-19`, `OD-21`); was "Capacity and credential rotation".* Every agent, cloud or local, gets its inference from one org-level gateway and from nothing else: LiteLLM, pinned by digest and never exposed, behind two services of ours, the **gatekeeper** (the only listener a sandbox can reach: pod binding, the live project route, a one-field `model` rewrite) and **keysmith** (the only holder of LiteLLM's master key: per-session keys, budgets, the usage rollup). Zero data retention on both routes, and no personal subscription anywhere. **Global** (the direct Anthropic API) is the default; **AU** (Amazon Bedrock, `ap-southeast-2`) is chosen per project by its data class, which is set when the project is created and can be updated (`OD-24`), and any member may raise a project to AU; on a team project lowering needs the org compliance role, and nobody, in any role, goes below the class's floor. A solo project keeps its owner's choice, so its owner may lower it back to that floor (owner question Q5). A sandbox holds only `ANTHROPIC_BASE_URL` and a one-hour, pod-bound, per-session token that Claude Code reads through `apiKeyHelper`; never an org key, never a credential in its environment. Spend is attributed per person, team and project, and each has a budget that is on by default. Runs on Docker Desktop for development (`OD-20`) and on AKS beside phase 8's workloads. Plan: `docs/superpowers/plans/2026-09-25-phase-6-gateway.md`. Proof: `docs/verification/2026-09-25-phase-6-local-e2e.md` (local, unmodified Claude Code); the cloud proofs in `infra/smoke/gateway/` are written and not run until phase 8's environment exists.
```

The paragraph names only what the plan built and the two records prove or leave open. It does not say the cloud path works.

- [ ] **Step 3: Amend the never-pool line**

Replace `docs/PROGRAM.md:132` (`- **Never pool subscriptions across users** — breaches Anthropic's Consumer Terms.`) with exactly:

```markdown
- **No consumer subscription is used, and no org key reaches a sandbox.** *Amended 2026-09-25 (`OD-19`); was "Never pool subscriptions across users — breaches Anthropic's Consumer Terms", a rule that existed only because consumer subscriptions forbid sharing.* Inference is the org's, through the phase 6 gateway, attributed and budgeted per person, team and project.
```

- [ ] **Step 4: Retire open risk 2, and add the gateway's one**

Replace `docs/PROGRAM.md:138` (open risk 2) with exactly:

```markdown
2. **RETIRED 2026-09-25 by `OD-19`** — was "Anthropic OAuth enforcement: subscription tokens are restricted to Claude Code/Claude.ai server-side". No subscription token is used; inference is org-level through the phase 6 gateway. What replaces it is risk <N> below.
```

and append, as the next number `<N>` after the last open risk (step 1's last line), exactly:

```markdown
<N>. **The inference gateway's two cloud unknowns.** Pod binding depends on the gatekeeper seeing a sandbox pod's own source address on AKS, and AU sessions depend on LiteLLM's Anthropic-to-Bedrock translation carrying what Claude Code sends. Both are proven only by phase 6's cloud scripts P03 and P04 (`infra/smoke/gateway/`), which have not run. Fallbacks: a broker-minted client certificate (seam S6.10), and a Bedrock-format connection for AU sessions (seam S6.4). The full list of what phase 6 leaves unverified is the plan's closing section.
```

Replace `<N>` in both with the number.

- [ ] **Step 5: Bring "Seams and asks" up to date**

Read every part's "Gaps against the skeleton" section (`grep -n '^\*\*Gaps against the skeleton' docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/*.md`, then each list). For each gap, decide one register and add it to the plan's "Seams and asks" with its source part in brackets. The gaps known when this part was written, and the register each belongs in:

| Gap | Source | Register | Entry |
|---|---|---|---|
| The gateway client has no `Usage` method; it must expose the owner and team it mints for | task 21 gaps 1, 2 | follow-up inside phase 6 | closed if task 15a's executor added `Usage` and 15b's the owner and team; else a task |
| keysmith's caller table has no `usage` route | task 21 gap 5, task 23 gap 4 | follow-up inside phase 6 | tasks 08a (the built-in `kube` table) and 17 |
| `projects.json` must be mounted as a directory; compose must pass budget variables through; name the control plane's token file | task 21 gap 3, task 23 gaps 3, 4 | follow-up inside phase 6 | task 17 |
| Local mode puts sandboxes and LiteLLM on one network | task 23 gap 2, item 9's result | follow-up inside phase 6 | *closed in the plan by review I-6 (fix batch b03)*: task 17's sandbox network holds the gatekeeper alone and the internal `panacea-app-gateway-backend` the rest; task 15 refuses another sandbox network. Item 9 is the check |
| A repository's `.claude/settings.json` can move `ANTHROPIC_BASE_URL` | review I-4 | follow-up inside phase 6 | *closed in the plan by review I-4 (fix batch b03)*: task 14's fragment pins the endpoint, proxy and provider-switch keys, turns telemetry export off (`CLAUDE_CODE_ENABLE_TELEMETRY` `"0"`) and project hooks off (`allowManagedHooksOnly`, re-review R-1); task 23's item 10 is the check. Still open, and named in task 14 "does not claim": any `env` key the fragment does not pin, and a project MCP server or plugin, which are seam S6.6's to close |
| No problem code for "below the floor" or "budget policy unreadable" | task 21 gap 4, task 06 gap 1 | follow-up inside phase 6 | task 03: `below_floor` (409), `budget_unknown` (503) |
| keysmith's rows omit a zero-spend current period | task 21 gap 6 | follow-up inside phase 6 | task 09 |
| Phase 8's proving list should carry P02 and P03 | task 24 gap 1 | **new ask P8-7** | "Task 32 runs phase 6's P02 and P03 in the same human-path session as S50" |
| P03 writes a temporary smoke-only egress policy in `panacea-sandboxes` | task 24 gap 2 | **new ask P8-8** | "Phase 8 accepts it, or supplies its own probe path" |
| Chart labels the scripts assume | task 24 gap 3 | follow-up inside phase 6 | task 19 |
| P06b proves ZDR on an **unreachable** upstream; an upstream that answers an error **with a body** is a second failure path, and this stack can make none. Its PASS is also bounded in time: the search follows a 90-second poll for LiteLLM's record of the call, so nothing is claimed about a write made after that window (re-review b07-1) | review M-6, task 18 "does not claim" | follow-up inside phase 6 | task 16: a `-fail-with <status>` flag whose message is fixed text, so the fake still echoes no input; then task 18 re-runs P06b through it and searches for that fixed text as well as the canary |
| Whether local projects get a class picker in the cockpit | task 22 gap 2 | **new owner question Q6** | until then, classification is by `curl` or phase 4's create flow (P4-1) |
| Whether the CLI should see `budget_exhausted` rather than LiteLLM's own budget error | task 23 gap 1 | **new owner question Q7** | a second proxy edit if yes |
| Whether "Budget reached" should name the dimension's real reach (a person's or team's budget stops every project) | task 22 gap 4 | **new owner question Q8** | wording only |
| The top bar has no project name | task 22 gap 1 | **new owner question Q9** (phase 2 design) | the chip carries the name in its label |
| A *Move back to Global* control, and the `canLowerRoute` field it needs, if the owner answers Q5 with the design's rule | task 22 gap 5 | **carried by Q5** | no new id: the skeleton's Q5 already asks it, and this row records where the answer lands |

A later part's gap not in this table is added the same way. A gap whose fix has landed is written as *closed by `<commit>`*, not deleted, so the record shows it was found.

- [ ] **Step 6: Correct the skeleton's own slip**

The plan's re-scope note (`docs/superpowers/plans/2026-09-25-phase-6-gateway.md:42`) says *"OD-19 supersedes that text; task 20 rewrites it"*. Task 20 is the Bedrock federation module; the rewrite is this task's. Change `task 20` to `task 25` in that sentence, and nothing else in it.

- [ ] **Step 7: Write "At close: what stays unverified"**

Append to the plan, after "Open risks", a section `## At close: what stays unverified`, one row per item, with the evidence that would settle it. Harvest first: `grep -n 'UNVERIFIED' docs/superpowers/plans/2026-09-25-phase-6-gateway.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/*.md`. Then strike every row that task 18's or task 23's record settled, citing the record's section, and keep the rest. The rows known when this part was written:

| # | Unverified | Where it is assumed | Settled by |
|---|---|---|---|
| U1 | The pinned LiteLLM version, and that it honours every rendered config key (`store_prompts_in_spend_logs`, `turn_off_message_logging`, `drop_params`) | tasks 02, 17 | task 18's record, P06, **P06b** (the same keys on the failure path, review M-6) and P10 |
| U2 | A caller-chosen `key` on `/key/generate`; `duration` expiry; user, team and key budgets; `/key/delete` at once; `/key/info` spend | tasks 04, 07, 09 | task 18's P01 to P05, P11, P12 |
| U3 | Which of LiteLLM's features are enterprise-only in the pinned image | tasks 06, 08b, 09 | task 18's record |
| U4 | Every concrete Bedrock `au.` inference-profile id, and that the profiles span only Sydney and Melbourne | tasks 02, 20 | listing the account's profiles at the first cloud run (task 24 gap 6) |
| U5 | LiteLLM's Anthropic-to-Bedrock translation carrying what Claude Code sends (P6-D3) | task 11, P6-D3 | task 24's P04 |
| U6 | The gatekeeper seeing a sandbox pod's own source address on AKS | task 10 | task 24's P03 |
| U7 | LiteLLM's web-identity parameters for the AWS role, and whether it reads its master key and database URL from files | tasks 19, 20 | the first cloud apply |
| U8 | `GET /v1/models`'s field set as the CLI reads it; the helper refreshed at `CLAUDE_CODE_API_KEY_HELPER_TTL_MS`, **and whether the CLI keeps using the last key for that whole interval** (task 23's item 11 waits it out before it can observe a failure at all, and every answered attempt is a lower bound on the cache; re-review b07-1); the fake's SSE sequence accepted by the CLI; **the CLI's own helper timeout, which the docs do not state, and whether a session recovers from a failed helper on the next message** (review M-5: task 14 decision (d) accepts one failed first request when a mint needs its retry, because the client's budget is about 61 s and the helper waits 20 s) | tasks 02, 14, 15, 16 | task 23's items 1, 8 and **11** |
| U9 | The Linux managed-settings path; the settings-reference and server-managed-settings facts cited from `05-design.md`, not re-read | task 14, the skeleton's docs table | task 14's own reading, task 23's item 2 |
| U10 | `CLAUDE_CODE_SKIP_BEDROCK_AUTH`'s name | seam S6.4 | whoever builds S6.4 |
| U11 | A Kata pod seeing an updated Secret volume (the token refresh), **and how long it takes** — kubelet's sync period plus cache TTL, about 1–2 min from training knowledge and unmeasured inside Kata, is the propagation term of `helper TTL + propagation < Grace` (review M-4, `task-14.md` gap 1) | ask P8-1, task 14 | phase 8's proving list; over the 2-minute budget, the fragment's TTL comes down to a 30-second floor before `Grace` goes up |
| U12 | LiteLLM logging in to Postgres with an Entra token | ask P8-4 | phase 8's decision |
| U13 | The tenant accepting the workload-identity exchange that reads the Hub | task 13, ask H-3 | phase H |
| U14 | Docker Desktop showing a host rename inside a container on a directory mount, on Windows | task 21 | task 23's item 3 |
| U15 | The reply shapes task 24 parses (`agnhost /clientip`, Bedrock's logging-configuration reply, LiteLLM's `/team/info` and `/user/info`) | task 24 | task 24's first run |
| U16 | LiteLLM's own budget reset aligning with the UTC month | task 06 | task 18's P03 and P04, over a month boundary |
| U17 | What the CLI's direct calls to `api.anthropic.com` carry | open risk 5 | a capture on an AU sandbox, which blocks them (P8-3) |
| U18 | The Panacea 405 incident's cause, read from source | the skeleton's pitfalls table | task 18's P07 measures the behaviour instead |
| U19 | Anthropic's and AWS's zero-retention terms for the org's accounts | P6-D9 | owner question Q3 |
| U20 | The names of LiteLLM's request-body control fields (`proxy.ControlFields`), and whether LiteLLM honours a body fallback outside a key's model list | task 11 decision (f), task 07 decision (d) | task 18's P14 (two of the twenty-four names); the rest by reading the pinned image's request parameters |
| U21 | A managed-settings `env` key outranking a project `.claude/settings.json`'s, key by key; the provider-switch names and `"0"` read as off; the CLI honouring `NO_PROXY`/`no_proxy` for its own requests; `CLAUDE_CODE_ENABLE_TELEMETRY` `"0"` making a project's `OTEL_*` keys inert; a managed `allowManagedHooksOnly: true` stopping a project `UserPromptSubmit` hook; a drop-in winning a top-level key over the base file | task 14 decision (e) (review I-4, re-review R-1) | task 23's item 10 |
| U22 | Docker's embedded DNS answering the dotted alias `gatekeeper.panacea-gateway.svc.cluster.local`; an `internal: true` network having no route out; a container only on internal networks publishing no port (on Docker Desktop for Windows); whether a sandbox reaches a host-published `127.0.0.1` port (keysmith `:4100`) through `host.docker.internal` | task 17 (review I-6, re-review M-4) | task 17's step 7 acceptance and task 23's item 9 |

- [ ] **Step 8: Verify and commit**

```bash
grep -c 'Capacity and credential rotation' docs/PROGRAM.md          # expect 1: only the "was …" clause in the new paragraph
grep -c 'per-user subscription registration' docs/PROGRAM.md        # expect 0
grep -c '### Phase 6 — The inference gateway (LiteLLM)' docs/PROGRAM.md   # expect 1
grep -c 'RETIRED 2026-09-25 by `OD-19`' docs/PROGRAM.md             # expect 1
grep -c '^## At close: what stays unverified' docs/superpowers/plans/2026-09-25-phase-6-gateway.md   # expect 1
grep -n 'task 20 rewrites' docs/superpowers/plans/2026-09-25-phase-6-gateway.md   # expect nothing
grep -niE '\b(agreed|accepted|confirmed)\b' docs/superpowers/plans/2026-09-25-phase-6-gateway.md | grep -iv 'OD-2[14]\|owner'   # read every hit
git commit -F <message file> -- docs/PROGRAM.md docs/superpowers/plans/2026-09-25-phase-6-gateway.md
```

The second-last line is the check on this task's one rule: every hit must be about an owner decision, never about an ask. Message: `docs(program): phase 6 is the inference gateway; never-pool amended; risk 2 retired (phase 6 task 25)`, a body listing the stale lines for the owner (step 9) and any line-number drift from step 1, and the attribution line.

- [ ] **Step 9: Tell the owner about the lines this phase does not own**

In the commit body and in the plan's "Owner questions", one entry: **Stale lines outside phase 6.** (1) `.claude/worktrees/env/CLAUDE.md:22`, the "per-user rotation … Never pool subscriptions" paragraph, stale in phase 8's branch only; it updates when that branch merges `main`. (2) Phase 8's precondition at `.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:330`, which asks for confirmation of a retired risk; it can be deleted from phase 8's precondition table. (3) `docs/FUNCTIONAL-SPEC.md:42`, which still describes sessions spending the user's own subscriptions and a Capacity view of accounts; OD-19 replaces both with the gateway and the inference panel (task 22).

**Acceptance:** step 8's six counts as stated; its last grep shows no "agreed", "accepted" or "confirmed" about an ask; the commit touches exactly the two files.

**What this task does not claim:**

- **That any ask has been accepted.** H-1 to H-5, P8-1 to P8-8, P7-1 and P4-1 are requests. Each phase that owns one decides.
- **That any seam is closed.** S6.1 to S6.12 stay open; this task lists them. S6.12, the two budget clocks, is fix batch b06's (review M-7): it closes on task 18's P15 and P16, inside this phase, not on another team.
- **That the cloud path works.** The programme text says the cloud proofs have not run, and the new open risk says what they must show.
- **That the unverified register is complete.** It is complete for what the parts wrote down. A claim nobody marked cannot be harvested by a grep.

**Gaps against the skeleton, for the orchestrator:**

1. **The skeleton's re-scope note names task 20 for this rewrite**; step 6 corrects it to task 25.
2. **`docs/FUNCTIONAL-SPEC.md:42` is stale** and is not in the skeleton's list; it is added to the owner's list rather than edited (decision (a)).
3. **The root `CLAUDE.md` line the skeleton calls stale is already current** in this tree; only phase 8's worktree copy is stale.
4. **New ask and question ids** (P8-7, P8-8, Q6 to Q9) are proposed in step 5; if another part has already used any of those ids, renumber here. They were P8-6, P8-7 and Q5 to Q8 until fix batch b05, when the skeleton took `P8-6` (the Hub reaches keysmith, seam S6.11) and `Q5` (may a solo project go back to Global); this rule is what renumbered them. Read the skeleton's ask and question lists before writing, not this sentence: the last id in each is the one to count from.
