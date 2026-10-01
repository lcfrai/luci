# 11 - The missing pieces, and the build map

**Status:** PROPOSAL (2026-10-01), beside `10-panacea-in-app.md`, for the owner's sign-off with it.
Until then `00-plan.md` stays the plan of record and none of its rows or totals change; its
section 5.3 points here.
**Date:** 2026-10-01. **Question:** the owner's, *"is there any other missing pieces of the
puzzle. Is everything mapaped out to what we need to build, what needs to be done, etc?"*
**Builds on, does not re-argue:** `00-plan.md` (rows 1 to 110, §3's order, §4's branches),
`10-panacea-in-app.md` (rows 110 to 137, its §2.6 calendar method and §3.3 streams),
`09-cohesion.md` §6 (the K1 to K14 gate), and the register through `OD-53`.
**Method:** a two-agent check (a journey walk and an independent gap hunt) was launched as run
`wf_55f1fb28-4aa` and stopped at the org's monthly spend limit at 16:09 to 16:10 with no findings
returned. This file was then written in the main session from a keyword sweep of the six largest
assemble documents (section 1.1), read against `00-plan.md` §3 to §4 and `10-panacea-in-app.md`
§1.8 to §3.5. **No verify round has run on it** (section 7).

## 0. The answer

**No, not until today. The features were mapped; running them was not.** `00-plan.md`, 09 and 10
map every journey step and every Panacea function, each through a verify round. None of them maps
the layer that lets KPMG people rely on the product:
- a threat model, agent safety, and a pen test;
- KPMG's technology-risk and privacy forms;
- monitoring, incident response, backups and restore drills for our own platform;
- its staging and release line, and the desktop's update channel;
- metering of everything, and evals once live;
- accessibility and load testing;
- a way to report a problem.

The sweep found no mention at all of most of these (section 1.1). **They are now rows 138 to
150, plus row 151 for the first-app task plans: about 41 engineer-days (about 30 to 55). None is on
the first app's path except row 151's half day per stream.** With them, every journey step and
every operational piece has a row, a stream and a cost. Section 6 lists what is deliberately left
unmapped, each with the milestone that will map it.

| Measure | `00-plan.md` | After 10 | After 10 and 11 |
|---|---|---|---|
| Remaining work, engineer-days | 132.275 (120 to 150) | 239.475 (205 to 290) | **280.475, about 280 (235 to 345)** |
| Capability rows | 110 | 137 | 151 |
| Done | about 11% | about 6% | **about 6% (5 to 7)** |
| By capability: open source / Panacea / managed / ours | 35 / 15 / 13 / 37 | 28 / 18 / 11 / 43 | **26 / 16 / 10 / 48** |
| By capability: adopted, together | 63% | 57% | 52% |
| By effort: open source / Panacea / managed / ours | 24 / 4 / 11 / 61 | 15 / 22 / 6 / 57 | **13 / 19 / 5 / 63** |
| By effort: glue on adopted pieces, together | 39% | 43% | 38% |
| First app, five builders | 13 to 17 working days | 22 to 25 | **about 23 to 26** (at least 21) |
| All of it, five builders | 26 to 27 (28 to 31 by 10's method) | 50 to 56 | **about 59 to 66** (at least 56), about 12 to 13 weeks |
| All of it, two builders | about 13 weeks | 25 to 28 weeks | about 30 to 33 weeks |

Five owner questions remain (section 5). Everything else in this file is our call, and section 5
lists those calls so he can override any of them.

## 1. The missing pieces, now mapped

### 1.1 How they were found

Case-insensitive matches per file:

| Term | `00-plan` | 10 | 09 | 07-verify | 01-verify | 06-verify |
|---|---|---|---|---|---|---|
| threat model | 0 | 0 | 0 | 0 | 0 | 0 |
| pen test / penetration / red team | 0 | 0 | 0 | 0 | 0 | 0 |
| prompt injection | 0 | 0 | 0 | 0 | 0 | 0 |
| TSRA / tech-risk / technology risk | 0 | 0 | 0 | 0 | 0 | 0 |
| privacy / PIA | 0 | 0 | 0 | 0 | 0 | 0 |
| auto-update / update channel | 0 | 0 | 0 | 0 | 0 | 0 |
| crash report | 0 | 0 | 0 | 0 | 0 | 0 |
| accessibility / WCAG | 0 | 0 | 0 | 0 | 0 | 0 |
| cost dashboard / recharge | 0 | 0 | 0 | 0 | 0 | 0 |
| incident | 0 | 0 | 0 | 1 | 0 | 3 |
| monitor / alert / on-call / observability | 1 | 1 | 0 | 0 | 0 | 0 |
| audit log / audit trail | 0 | 1 | 0 | 1 | 0 | 0 |
| eval | 1 | 0 | 0 | 0 | 0 | 1 |

**What the few hits are.**
- **Monitoring:** 10's row 118 covers the apps estate only, not our platform cluster.
- **Audit trail:** 10's row 125 is the shipped apps' trail, not our admin acts.
- **Eval:** `00-plan.md` row 47 is the done evaluator, which checks provenance, not product quality.
- **Telemetry:** `00-plan.md` rows 11, 24 and 34 turn the vendors' telemetry off; none turns ours on.
- **Our CI** exists only as 10's CI for the lift and `feat/env`'s pipelines and licence gate (`00-plan.md` §2.1).

**Already covered, so not added:**
- Shipped apps' backups and restore (10 rows 58, 107, 122).
- The apps estate's alerts and runbooks (row 118).
- Help inside the app (row 126).
- Sandbox egress (`00-plan.md` row 5, and 01-verify's 20 mentions).
- Shipped apps' supply chain: Tekton Chains and signed images (row 67; 06-verify's 15 mentions).
- The security reviews of row 112, the hand-off and the shared sandbox (10 §3.4, `00-plan.md` §3.4).
- The snippet ledger's CI (row 72).

### 1.2 The rows

Kinds and columns as in `00-plan.md` §1 and 10 §1.1. Days are engineer-days.

| Row | What | Needed for | Stream | Kind | Low | Mid | High | Basis |
|---|---|---|---|---|---|---|---|---|
| 138 | **Threat model of the whole platform.** Sandboxes and agent tools; the identity chain (our Keycloak, Panacea's realm, Entra, the authenticator); gateway keys; the lifted Panacea and the apps estate; the Hub. Includes the agent-safety review: prompt injection through briefs, repositories, web pages and tracker text, and what an agent may do unasked | pilot | B5 leads; every builder reviews its boundary | Ours | 2 | 3 | 4.5 | One pass per trust boundary, about seven boundaries. 10 §3.4's reviews cover single parts and stay |
| 139 | **Agent-safety controls** the threat model calls for: tool allow-lists per lane; a person's approval for risky acts (a push to a protected branch, a deploy, a secret read); untrusted text marked as such in an agent's context; secret scanning on agent output and commits | pilot | B1 | Ours | 1.5 | 2.5 | 4 | Built on agent-sandbox's template and the recipe (rows 2, 8). Egress is already row 5 |
| 140 | **An independent penetration test** before the first customer, by KPMG's own security testers if they take it (question 4), and fixing what it finds | customers | whoever owns each finding | Ours | 2 | 3.5 | 6 | 1 day of support and 1 to 5 of fixes. The test itself is not our days |
| 141 | **KPMG governance.** The technology-risk submission and a privacy impact assessment for the new Panacea, starting from Panacea's own submission pack; records-retention and deletion rules for briefs, transcripts, logs and recordings, set in the Hub, the gateway and the apps estate | pilot; opened on day 0 because of its lead time | B4, with the owner as sponsor | Ours | 3 | 4.5 | 7 | Panacea's pack exists (its tech-risk submission, 2026-09). The deltas are the agents, the gateway and the desktop |
| 142 | **Monitoring and alerts for our own platform.** Golden signals and alerts for the Hub, the gateway, our Keycloak, the sandbox pools and warm-pool depth, Penpot, and certificate expiry, on the observability module `feat/env` already built, routed to one on-call channel | pilot | B2 | OSS | 1.5 | 2.5 | 4 | Row 118 does this for the apps estate; this row covers the platform cluster |
| 143 | **Backups, drills and incidents for our platform.** The Hub's database, our Keycloak, LiteLLM, Penpot and the ledger, backed up and restored once in a drill; the platform rebuilt from OpenTofu once; severity levels, who is called, a runbook per alert, and a status line in the app | pilot | B2 | Ours | 2.5 | 4 | 6 | Panacea's own lesson: production certificate renewals failed silently for about six weeks (06-verify §2.4) |
| 144 | **An audit log of admin acts.** Provider keys and routes (row 104), team and role changes, approvals and sign-in settings, kept, searchable and exportable in the operator console | pilot | B4 | Ours | 0.5 | 1 | 1.5 | If row 130's governance area already holds this log, the row shrinks to its export |
| 145 | **A staging copy of our platform, and its release line.** A staging namespace on the platform cluster for v1, and a pipeline that promotes staging to live, in our own Azure DevOps organisation | pilot | B2 | Ours | 0.5 | 1 | 2 | A separate staging cluster waits for customers |
| 146 | **The desktop's release line.** Our Orca build built and signed in CI (Artifact Signing, call 4); an update feed with staged rollout and rollback; row 11's Intune package fed from it | pilot | B1 | Ours | 1.5 | 2.5 | 4 | The first app can run on the owner's own install. Pilot users need updates |
| 147 | **Meter everything.** One usage record per act (sessions, sandbox minutes, AI tokens by provider, builds, previews, storage), each tagged with the project's Cost Centre, Job Code and Phase (row 105); a monthly recharge export by those codes; desktop crash reports to our own endpoint. No third-party telemetry: rows 11, 24 and 34 keep the vendors' off | pilot, from its first day | B5 | Ours | 2.5 | 4 | 6 | The owner's "meter everything". The gateway already attributes AI usage per user, team and project (`OD-46`) |
| 148 | **Evals once live.** One eval per dev-workflow function (brief quality, plan quality, lane success, review, landing, deploy), run nightly against fixed projects, with results on an admin page | after live | B3 | Ours | 3 | 4.5 | 7 | The owner's "evals after live", and the eval-per-function suite he asked of Panacea on 2026-09-29 |
| 149 | **Accessibility and load.** A WCAG 2.2 AA pass over our pages and the themed embeds; a load test at pilot-plus scale (concurrent sessions and lanes, warm-pool sizing, gateway throughput, the apps estate's build queue) | customers | B3 and B4 | Ours | 3 | 4.5 | 7 | The warm-pool size is already a later owner call (`00-plan.md` §5.3) |
| 150 | **Report a problem.** A button on every page that sends the build team the page, the lane and its recent logs; a feedback channel; row 143's status line | pilot | B4 | Ours | 0.75 | 1 | 1.5 | Row 126 already carries help inside the app |
| 151 | **The first-app task plans.** One task-level plan per stream for its first-app rows, with hour tables like the phase plans, written in the before-the-build window (section 4 step 2) | day 1 | every builder | Ours | 2 | 2.5 | 4 | About half a day per stream. Later rows' plans are written just in time (section 2.2) |
| | **Sum** | | | | **26.25** | **41** | **64.5** | Taken as about 30 to 55, because not every estimate errs the same way |

**By when they are needed (mid).** Day 1: 2.5 (row 151). Pilot: 26 (rows 138, 139, 141 to 147,
150). Customers: 8 (rows 140, 149). After live: 4.5 (row 148).

### 1.3 The arithmetic

The method is 10's (§2.2 to §2.6), unchanged.

**Total.** 239.475 + 41 = 280.475.
- 10's range: 205 to 290.
- With this file's 30 to 55: about 235 to 345.

**Done.** 16.5 / (16.5 + 280.475) = 16.5 / 296.975 = **5.6%, about 6%.**
- With phase 2's backend at half: 14 / 294.475 = 4.8%.
- At double: 21.5 / 301.975 = 7.1%.
- Hence about 5 to 7%.

**By capability, 151 rows.** Row 142 is open source; the other thirteen rows are ours.

| Kind | Rows | Exact | Rounded |
|---|---|---|---|
| Open source | 40 | 26.5% | **26%** |
| Panacea | 24 | 15.9% | **16%** |
| Managed service | 15 | 9.9% | **10%** |
| Ours | 72 | 47.7% | **48%** |

- Rounded by largest remainder so the four add to 100. Floors are 26, 15, 9 and 47 (97); the three largest remainders (0.93, 0.89 and 0.68) go to managed services, Panacea and ours.
- Adopted together: 79 / 151 = **52%**.

**By remaining effort, 280.475 days.**

| Where the days go | Days | Exact | Rounded |
|---|---|---|---|
| Glue on open source | 34.9 + 2.5 = 37.4 | 13.3% | **13%** |
| Glue on Panacea | 53.125 | 18.9% | **19%** |
| Glue on managed services | 15 | 5.3% | **5%** |
| Our own code | 136.45 + 36 + 2.5 = 174.95 | 62.4% | **63%** |

- Rounded by largest remainder: floors are 13, 18, 5 and 62 (98); the two largest remainders (0.94 and 0.38) go to Panacea and ours.
- Glue on adopted pieces together: 105.525 / 280.475 = **38%**.

**Why "ours" grows again.** The operational layer is ours by nature: no product we adopt runs our
threat model, our drills or our recharge report.

**Calendar** (10 §2.6: at most one engineer-day per builder per working day; forecasts at 85 to 95% of that).
- **All of it, five builders:** 280.475 / 5 = 56.1 working days at least, so about **59 to 66** (about 12 to 13 weeks).
- **Three builders:** at least 93.5, so 98 to 110 working days (about 20 to 22 weeks).
- **Two builders:** at least 140.2, so 148 to 165 (about 30 to 33 weeks).
- **First app, five builders:** 10's most-loaded builder carries about 21.2 engineer-days; row 151's half day makes 21.7, so **about 23 to 26** working days (at least 21.7). No other row of this file is on the first app's path.

## 2. The build map

### 2.1 The streams

The base is 10 §3.3, which re-plans `00-plan.md` §3.3. This file adds rows only after the first
app, plus row 151's half day at each stream's start.

| Builder | The first app (10 §3.3) | After it (10 §3.3) | Added by 11 |
|---|---|---|---|
| **B1, sandboxes** | Rows 1 to 9; our Orca build's first-app rows (19 to 24, 93, 99, 102, 103); then 82 and 95. About 20 | Row 39, Design stream B, 131 | 139, 146 |
| **B2, our own Panacea** | Rows 110 and 111, then row 117's first-app share. About 19 | 117, 118, the weekly re-sync 119, 120 at go-live; Design stream A (34, 36 to 38, 44); 87, 94 | 142, 143, 145 |
| **B3, the lane and deploy path** | Rows 11 to 18; row 114's first-app share; row 137's GitHub half; row 116. About 19.75 | Define (25 to 33), 48, 55, 134, 136, row 137's Azure DevOps half | 148; 149's load half |
| **B4, the Hub** | Row 74's minimum, 75, 88, row 105's Hub side, the Desk (89 to 91), 100, row 112's realm side. About 20.75 | The rest of the Hub, 47, 50, 108, 41, 42, 127, 128, half of 121 to 126 | 141 (with the owner), 144, 150; 149's accessibility half |
| **B5, gateway and Deploy** | Rows 76 and 76b, 104, row 105's hand-off, the first-ship Deploy glue (about 4.25 of rows 61 to 66), 115, row 112's middleware. About 21.2 | 107, 109, 113, 106, the other half of 121 to 126, 129, 130, 132, 133, 135, 45, 46, 52, 53, 92 | 138 (lead), 147, row 140's fixes |

10 §3.3's sixth-builder variant is unchanged by this file. It lifts the five-builder cap for three
weeks (question 1).

### 2.2 Where each stream starts, and its plans

From `00-plan.md` §4 (which branch each stream continues) and the phase plans already written:

| Builder | Continues | Existing task plans that still apply | Plans still to write |
|---|---|---|---|
| B1 | The base branch's `control-plane/` and `sandboxd/`; `feat/env` | `feat/env` tasks 05 to 08 (rows 1, 78), 17 and 18 (row 7); task 14's hardening moves into agent-sandbox's template | Rows 2 to 6, 8 and 9; our Orca build's rows |
| B2 | Nothing built | None | Rows 110, 111 and 117, from Panacea's engine and 10 §1.3 |
| B3 | `feat/phase-3-git`, `feat/phase-7-deploy`, later `feat/phase-4-discover` | phase 3 tasks 01 to 05, 07, 11 to 15 and 23; phase 7's row 66 and 61 tasks; phase 4 tasks 01 to 10 for Define | Rows 114, 116 and 137 |
| B4 | `feat/hub` | Hub tasks 08 to 10, 13 to 24, 28 to 35, 37 and 39 (row 74) | The Desk (89 to 91), 100, and row 105's Hub side |
| B5 | `feat/phase-6-gateway` (a plan, unbuilt), `feat/phase-7-deploy` | Phase 6's 29 tasks (rows 76, 76b) | Rows 104, 115, row 105's hand-off, and row 112's middleware |

- **Paused:** `feat/team-mode` and `feat/work-layer` (`00-plan.md` §4).
- **Salvage branches pushed on 2026-10-01:** `salvage/env-2026-09-28` (6 files), `salvage/hub-2026-09-28` (3) and `salvage/phase-7-2026-09-28` (3). They hold the uncommitted work last touched on 2026-09-27 and 28, neither built nor tested. B1, B4 and B3 respectively review each one before resuming, and keep or drop it.
- **The plan rule:** each stream's first-app plan is written before its first row (row 151). Every later row's plan is written just in time, when its stream reaches it, which is the owner's standing rule for far-off plans.

### 2.3 Done, per milestone

- **First app.** Each stream's rows are green in CI, and the K1 to K13 automation (row 86) is green for the journey steps the stream owns. Then the owner's walk, K14: *"Does this feel like one app?"* A no fails the build.
- **Pilot.** All of the above, plus:
  - every row marked "pilot" in section 1.2;
  - the security reviews of 10 §3.4 and `00-plan.md` §3.4;
  - row 141's submission accepted.
- **First customer.** All of the above, plus rows 140 and 149, and the production estate (10 §1.7).

### 2.4 Progress reports

Every stream report and every resume gives, per stream: tasks done of total, how close the first
app is to testable, and the honest token spend. This is the owner's standing rule.

## 3. Day 0, in one list

This merges `00-plan.md` §3.1 as 10 §3.1 amends it, plus this file's items 9 and 10.

| # | Ask | Who | Blocks |
|---|---|---|---|
| 1 | Two resource groups in Panacea's subscription: our platform's (`OD-42`) and our apps estate's (10 §3.1 item 1; question 1). Also: vCPU quota in Australia East (about 376 for the move plus at least 108 for us), the bring-your-own public IP registration, and the Kata policy exemption | Panacea's subscription owners; KPMG cloud governance if the blocking policy sits at a management group | B1 and B2 from day 1 |
| 2 | MDE suppression and the ASR allowlist for our signed build; Artifact Signing's eligibility check (call 4) | KPMG IT and SOC | The first ship to anyone but the owner |
| 3 | Today's Panacea: the realm block for our broker (`OD-44`; `00-plan.md` §3.1 item 3) and our egress address on its edge. Both are declared to the production move, and need the owner's yes during its freeze | The owner; Panacea's pipeline | Real sign-in (`devissuer` covers the build meanwhile) |
| 4 | One DNS record delegating `apps.panacea.kpmg` (question 3) | The owner's account, as for dev's on 2026-08-19 | The first preview on our estate |
| 5 | Our own Azure DevOps organisation in tenant `eb9a11c9…`. Its pipeline's service connection, Contributor on the apps resource group only, and one paid parallel job. Its admins and `kpmgcomau`'s add our managed identity | The owner; Azure DevOps admins | B2's estate bootstrap; row 135 |
| 6 | An owner of `kpmg-au-emu` creates and installs our GitHub App | A `kpmg-au-emu` owner (the owner is a member there, not an owner) | GitHub lanes (row 137's GitHub half) |
| 7 | The two IT requests (10 §4, question 4) | KPMG IT | Azure DevOps as the person (row 136); until then, question 4's fallback |
| 8 | A Teams admin for our bot; a member-readable roster route; at least one inference provider's endpoint and key for the provider page (KPMG Workbench today) | Teams admin; the owner | Row 52; row 104's first use |
| 9 | **New.** KPMG's technology-risk submission and privacy impact assessment opened, with the owner as sponsor (row 141; question 4) | The owner; KPMG technology risk | The pilot beyond the owner |
| 10 | **New.** Where the five streams run, and the monthly AI spend limit (question 5) | The owner | The calendar |

## 4. The order

1. **Day 0** (section 3).
2. **Before the build** (10 §3.2):
   - phase 2's close-out and merge to `main`;
   - the owner's review of the shell prototype, now with one Deploy page over our own Panacea;
   - the day-1 spikes: the browser route; the realm hand-off and the Windows bridge; the journey bar on the first Orca pin; Penpot's headless runner; OpenBao under the engine's Vault steps; Git Credential Manager on a managed laptop;
   - row 151's five plans.
3. **The first app:** the five streams' first-app rows (section 2.1), about 23 to 26 working days. The gate is J1 to J9 with K1 to K14, and K14 is the owner's eyes.
4. **The pilot:**
   - the "after it" rows the pilot needs (`00-plan.md` §3.3, 10 §2.6);
   - section 1.2's pilot rows: 26 engineer-days, about 6 working days across five builders, beside those rows;
   - the security reviews;
   - row 141 accepted.

   The pilot is not dated: 10 places its own pilot rows after the first app without dating them, and this file does not re-sequence them (section 7).
5. **All of it:** about 59 to 66 working days at five builders. Rows 140 and 149 come before the first customer; row 148 comes once live.
6. **Later:** section 6.

## 5. Owner questions

Five, each with its recommendation. All are his because they are money, customer-visible, or his
own systems.

1. **Build our own Panacea at this cost?** (10's question 1)
   - About 107 engineer-days more: 71 for our own Panacea, 8 for the authenticator, and 28 for Panacea's screens the app owed anyway.
   - Running cost: about US$1,700 to 2,100 a month at pilot size. The production copy, later, is about US$3,300 to 5,200 a month more.
   - Shipped apps get their own resource group, in the same subscription and on the same bill.

   **Recommended: yes to all three, with five builders.**
2. **Today's Panacea and its users** (10's question 2). His apps on today's Panacea (RAPTR, GenFR, Eunomia, Manifold, Trace) stay on today's website until the merge, and teams live in both places until then. **Recommended: leave everything as it is until the production move signs off.**
3. **The web address of shipped apps** (10's question 3). **Recommended: `<app>.apps.panacea.kpmg`.** The trade-off: it shares a cookie scope with today's Panacea. A separate domain would avoid that, but needs IT.
4. **Day-0 asks of IT and KPMG** (10's question 4, plus rows 140 and 141):
   - send both IT requests on day 0;
   - open the technology-risk submission and the privacy assessment, with him as sponsor;
   - book KPMG's own security testers for a pen test before the first customer.

   Until IT approves, Azure DevOps comments post under our account naming the person, and personal votes wait. **Recommended: yes to all three.**
5. **Where the builders run, and the spend limit** (new).
   - **The laptop:** with 13 agents running, a command took about 136 seconds instead of 5 (his note of 2026-10-01).
   - **The spend limit:** the org's monthly AI spend limit stopped every agent on 2026-09-30, and again on 2026-10-01 at 16:09.
   - Five streams need more of both. **Recommended: one cloud build machine in our resource group (about US$350 to 700 a month; from knowledge, not checked), and ask KPMG to raise the monthly limit for the build's weeks.**
   - Without both, plan on two or three streams at once. All of it then takes about 20 to 33 weeks rather than 12 to 13.

**Decided by us** (he may override any of these):
- the shape of rows 138 to 151;
- a staging namespace before a staging cluster;
- no third-party telemetry, only our own metering;
- evals after live, not before;
- each stream reviews its salvage branch before resuming.

## 6. Deliberately not mapped yet

Each is mapped just in time, when its milestone is next.

- The merge of today's Panacea into ours, and the old Portal's retirement (10 §1.7; forced exception 5).
- Our Panacea's production estate (10 §1.7).
- The customer-hosted tier (`CLAUDE.md`).
- The Orchestrator negotiation after team v1 (`OD-51`), and Phase W's remainder (`OD-48`).
- Edge functions (10 §1.7).
- `00-plan.md` §5.3's "Later" list: the Penpot Enterprise quote; licence treatment before a customer-hosted tier; the customer isolation statement; the WAF edge; GitHub Enterprise Cloud; the warm-pool size; a Figma adapter; the new icon.

## 7. Not verified

- **Every figure in section 1.2 is this session's estimate, with no verify round.** The two agents that would have checked it stopped at the org's monthly spend limit before returning anything.
- The keyword sweep can miss a piece named differently. It was read against `00-plan.md` §3 to §4 and 10 §1.8 to §3.5 only.
- We assume KPMG requires a technology-risk submission and a privacy assessment even for an internal, class-`none` pilot. That comes from Panacea's precedent, not from asking.
- Whether row 130's governance area already holds an admin audit log. If it does, row 144 shrinks.
- Whether `feat/env`'s observability module suits the platform cluster, and its licence (Grafana's is on `00-plan.md` §5.3's "Later" list).
- The cloud build machine's price.
- The pilot's date: not computed.
- The three salvage branches' contents: neither built nor tested.

## 8. Sources

- `00-plan.md` §1, §2, §3.1 to §3.4, §4 and §5.3.
- `10-panacea-in-app.md` §1.3 to §1.8, §2.1 to §2.6, §3.1 to §3.5 and §4.
- `09-cohesion.md` §6; `06-deploy-verify.md` §2.4.
- `2026-09-23-owner-decisions.md`, `OD-37` to `OD-53`.
- The owner's notes:
  - 2026-09-29: an eval per dev-workflow function;
  - 2026-09-30: "meter everything", "evals after live";
  - 2026-10-01: fewer agents at once, measured at 136 s per command.
- Run `wf_55f1fb28-4aa`, stopped by the spend limit at 16:09 to 16:10.
