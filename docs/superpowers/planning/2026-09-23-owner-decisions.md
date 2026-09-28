# Owner decisions — 2026-09-23 (evening)

The product owner answered the open questions in four rounds on 2026-09-23, 20:00–20:30. This file
is the **single record** of those answers. Every plan written after this date cites it by decision
id (`OD-n`), and where an older document disagrees, **this file wins**. The answers are quoted
where the owner wrote free text, because the words carry the intent.

## The decisions

| Id | Question | Answer | Consequence |
|---|---|---|---|
| **OD-1** | How to proceed on phases 3, 4, 5 and 7, which have prototypes but no plans | **Build now, on the recommendations on file** | The review gate (`CLAUDE.md`: prototypes before plans, plans before code) is waived for these phases by the owner. Every plan is still adversarially reviewed before code. UI branches stay unmerged until the owner has looked at them. Anything the owner later overrules is reworked. |
| **OD-2** | `3-D1`: what a worktree is in git | **A local clone per worktree** | Phase 3 as its brief recommends: objects hard-linked, work fetched back under `refs/panacea/lanes/…`, agent blast radius is its own clone, and the model survives Kata. |
| **OD-3** | Critique-2 §7.1 #2: briefs, approvals, ledger per person or shared | **Shared from day one** | A server-side store exists before phase 4 needs it. See OD-5 and the new phase below. |
| **OD-4** | `3-D8`: git host | **Both Azure DevOps and GitHub** | Phase 3's push/PR path supports both hosts from v1, behind one host interface. |
| **OD-5** | Where the shared stores live | Owner's words: *"assume that you are building from scratch infrastructure wise, and build what would work best … we will basically give you a whole new environment to run your own stuff … it all needs to be in this repo"* | **The orchestrator decides the architecture.** PanaceaApp gets its own server side and its own environment, all in this monorepo. Panacea is the model for how to run the web and deploy side, not the host. A monorepo is kept (`go.work` plus the desktop and infra trees): one team, one release train, and cross-cutting changes land atomically. |
| **OD-6** | How the cockpit signs the user in | **Entra SSO via OAuth 2.0 authorization-code + PKCE** | The system browser opens for Entra. Tokens are kept in Windows Credential Manager (DPAPI). The same store holds the Azure DevOps and GitHub tokens (OD-4). Users are always themselves; nothing is pooled. |
| **OD-7** | Backup | The owner created **`https://github.com/lcfrai/panaceaapp`**. The orchestrator pushes feature branches as needed. | The PAT lives at `~/.secrets/panaceaapp-github.pat`, read only by a repo-scoped git credential helper (`git config --local`), never printed. The history was scanned for secrets and large blobs before the first push. |
| **OD-8** | `3-D7`: may a feature read a harness's transcript **file**? | Owner: *"If it doesn't make sense, figure it out and amend it to make sense for others after you."* | **Ruled yes, with guards. `PROGRAM.md`'s constraint is amended (below).** |
| **OD-9** | Windows: the cockpit's Ctrl chords with a terminal focused | **Keep, plus a toggle** | The cockpit wins by default, as in VS Code. A setting, *"Send shortcut keys to the terminal"*, hands them all back. Phase 2 follow-up. |
| **OD-10** | What Forget does to a persisted transcript | **Forget deletes it** | Forget deletes the session's conversation file, and removing a worktree deletes its store. Phase 2 follow-up, on the Resume fix's store. **Built 2026-09-24** (go-contract §3.15): Forget (`DELETE …?purge=true`) deletes `<id>.jsonl` for each conversation the record names as its own — its current one and those its restarts left — by the minted id only, before the record goes. A failed deletion answers 500 and keeps the record, so Forget can be retried. An id no mint produces deletes nothing and is logged. A fork keeps its own file: the CLI writes it as a whole copy, verified to survive its source's deletion — observed in `-p` mode; the interactive fork before its first turn is unverified until OD-12's hands-on session. **Worktree removal: no product action removes a worktree in phase 2**, so the store deletion is a tested seam (`sessionForgetWorktreeStore`) that phase 3's worktree-removal verb calls. |
| **OD-11** | Paperdesign (AGPL-3.0) for the design canvas | Owner: *"Number 2 [avoid AGPL entirely], but I want it pretty much exactly like paper.design with the MCP, it is super powerful."* | **Phase 5 is re-scoped as a clean-room, paper.design-class canvas with a first-class MCP server,** built only on permissively licensed parts. The locked decision "adopt `Projects/Paperdesign`" is **withdrawn**. The clean-room rules are below. |
| **OD-12** | When the owner does task 25's hands-on steps (`/login` in a sandbox, watching the window) | **Batch at the very end** | One hands-on session after phases 3–7 are built. Until then every owner-observed item stays **unverified**, and each report says so. |
| **OD-13** | How much of Panacea's source comes into this repo | Owner: *"Keep Panacea separate for now, as we are working on an update at the moment. So once we finish everything else, we will lift and shift Panacea into this, and build it out within the app to be a beautiful seamless part of the app."* | **No Panacea source is copied in now.** Phase 7 calls Panacea's API as a client, exactly as `PROGRAM.md` says. A later phase lifts Panacea into this repo and makes deploy a native part of the app. Designs today keep the deploy side behind an interface so that lift is a swap, not a rewrite. |

**Every other recommendation on file is accepted as written** (OD-1), including the phase 3 brief's
`3-D2`–`3-D6`, `3-D9` and `3-D10`, `completeness-critique-2.md` §7.1 #1, #3 and #5, and §7.2–§7.3,
plus the phase 4, 5 and 7 briefs' recommendations. Three exceptions:
- where OD-3/OD-5 changes a store's home, the recommendation's *shape* stands but its *location* is
  the Hub;
- `5-D1`/`5-D2` (where Northstar's server runs, and whether its second spine goes) are moot under
  OD-11;
- critique-2 §7.1 #5 (start the AGPL legal clock) is moot under OD-11. No AGPL code is used.

Critique-2 §7.1 #4 (who owns control-plane authentication) is **answered by OD-6 plus the Hub**: the
Hub phase owns identity, and phase 7 consumes it.

## The new phase: the Hub

OD-3 and OD-5 need a server side that nothing in the phase table owns. **It is added as a lettered
phase, "Phase H — the Hub", between phases 3 and 4.** It is lettered so that no existing phase number
moves: the cockpit's copy and three prototypes cite phase numbers, and a renumber has already cost
this program one sweep.

The Hub is the shared server: identity (Entra), the brief store, approvals, the provenance ledger,
releases, and teams. It is built **local-first**, like everything else (`PROGRAM.md`, "Local-first"):
Docker Postgres and a real Entra app registration on a laptop, deployable to the owner's new
environment when it exists. The desktop's control plane calls it **as the user**, with the user's
Entra token. Its exact decomposition is the Hub architecture decision, recorded as an ADR in its
own plan.

The owner's new environment ("a whole new environment to run your own stuff") is phase 8's to
stand up. Phase 8's scope grows from "Kata backend + Terraform + warm pools" to **the product's own
environment**: AKS with Kata, the Hub's hosting and database, identity configuration and
networking. It is modelled on Panacea's live `deploy/main.tf` and written from scratch in this repo.
Its Terraform is **written and planned, never applied**, until the owner provides the environment.

## OD-8 — the transcript rule, amended

The old `PROGRAM.md` constraint said *"Any feature requiring us to parse or replace agent UI is
wrong"*. Phase 2's plan said *"Agent CLIs are never modified, wrapped or parsed. No feature in this
phase may read PTY bytes to derive meaning."* The two read differently for a transcript **file**,
and the owner asked for a rule that makes sense.

**What the rule protects:**
1. The CLI stays the real, unmodified product the user already trusts.
2. We never guess meaning from a terminal stream, which is fragile and can be forged by anything
   the agent prints.
3. We never depend silently on undocumented internals.

Reading a transcript file threatens only the third. So:

> **Harnesses run unmodified and unwrapped, and no feature derives meaning from PTY bytes.** A
> harness's own on-disk records — for Claude Code, its JSON-lines transcript — **may be read,
> read-only**, through a per-harness adapter that is **pinned to the CLI version in the sandbox
> image**, covered by a **canary test** that fails when the format moves, and whose every consumer
> **degrades to commit- and session-level facts** when the adapter cannot read a file. No feature may
> *require* the adapter to function. Deleting a harness's file by the id the control plane minted
> (OD-10) is allowed. Rewriting its contents is not.

Phase 2's own restatements stay as they are, because phase 2 reads no transcript. The amended
rule is the one every later phase cites.

## OD-11 — clean-room rules for the design canvas

The owner wants paper.design's power, the MCP especially, without AGPL. That is a **clean-room**
build. The rules bind every phase 5 agent:

1. **Do not open, read, search or copy** the source of `~/Projects/Paperdesign`, `kgoedecke/doop`,
   or any other AGPL or GPL canvas. Earlier planning documents that *describe* Northstar's behaviour
   (`survey-phase-5-design.md`, `scope-and-risk-phase-5-design.md`) may be read for product ideas.
   No code from them is reused.
2. **The specification comes from public product behaviour**: paper.design's public site, docs and
   changelog, and its published MCP tool surface. Record every source URL. Where the corporate
   network blocks a site, say so; do not guess.
3. **Dependencies are MIT, Apache-2.0, BSD or ISC only**, checked by a licence gate in CI. **tldraw
   is excluded**: its licence requires a commercial key for production. Excalidraw (MIT) is allowed
   for a sketch mode.
4. The canvas is its own package in this monorepo, with a clean interface to the cockpit, so it can
   ship and be tested alone.

## Phase 2 follow-ups these decisions add

- **OD-9**: the "Send shortcut keys to the terminal" toggle (task 23's area).
- **OD-10**: Forget deletes the transcript; worktree removal deletes its store (Resume fix's area).
  **Forget half built 2026-09-24. The worktree half is a seam for phase 3**, because phase 2 has no
  worktree-removal action (go-contract §3.15).
- **The Resume fix's concern 5, "Fork does not carry the conversation"**, is a defect against the
  functional spec's "branch the session from this point". It is **fixed, not decided**, because the
  CLI's `--fork-session` exists for exactly this. **Built 2026-09-24** (go-contract §3.9): a Fork
  sends `resume` like a Resume, into the other worktree, and gets Resume's argv; its sandbox is
  shown a staged, read-only copy of the source's transcript, and Forget deletes what carrying left.

## Team mode — owner decisions of 2026-09-24 (evening), OD-14 to OD-17

The owner's framing, verbatim: *"Aren't we managing all of the agents centrally, rather than them
being on each machine? I am thinking like a Word Document on Sharepoint being edited by multiple
people, where each agent can see other agents doing work, understand what the agent is currently
working on and the task they have and the reason they are updating that document, they are able to
communicate and understand each other. That is the beauty of us having it sit at this top level as
a managed agent and as a managed solution."*

| Id | Question | Answer | Consequence |
|---|---|---|---|
| **OD-14** | Where team agents run | **Centrally, in our environment** | The control plane for team projects runs server-side, as a managed service. Every team agent is a managed session in a cloud sandbox (Kata) that the platform sees and coordinates. The desktop is a client. Laptop Docker stays as a single-person offline and development mode. Reverses ADR-H1's "sessions, machines, worktrees and PTYs stay local" for team projects, and phase 8's owner-only broker visibility. |
| **OD-15** | What teammates see | Owner: *"what is important is that the agents see what the other agents are doing. The user can see who else is working on the repo, and a general summary of what they are doing, files they are touching, but it needs to be in a simple and meaningful way to not clutter the dashboard. The main thing is that the agents know."* | **Agent awareness is the primary surface; human awareness is a calm summary.** Humans see who is working, a one-line summary of each person's work, and the files being touched, without clutter (AGENTIC-UX: change, not state). A teammate's terminal is not part of the default experience. |
| **OD-16** | Who talks to whom, and who decides | Owner: *"the orchestrators need to be the communicators and work with each other to make sure things progress, and if there are conflicts, they raise them to the users … if an agent is doing something that conflicts with another agent, or something a user doesn't like, the user will message their Orchestrator, that orchestrator will work with the other user's orchestrator to understand why, and they will decide whether to raise that to the user informing them, or asking for a decision."* | **Each user has an Orchestrator agent that runs their sub-agents.** Orchestrators are the ONLY cross-user channel. They negotiate peer to peer through the platform, resolve what they can, and escalate the rest to their humans, either to **inform** or to **ask for a decision**. **No user steers another user's agent.** A user talks only to their own Orchestrator. Accountability and billing stay with each agent's owner, and no subscription is pooled. |
| **OD-17** | When team mode is planned and built | **Design now, build next** | The research workflow produces the full design and plan now. It is built straight after the current wave, as the product's next phase. The Hub and phase 8 are re-scoped where they touch it. The in-flight streams continue as foundations. |

## Cloud, inference and local mode — owner decisions of 2026-09-25, OD-18 to OD-20

The owner's framing, verbatim: *"The goal of making all of this managed in the cloud, is that they
will not use their own subscriptions, we would use direct API, that allows for ZDR and other
security stuff for us … The app on device should be the portal to the cloud, for them to build
solutions."*

| Id | Question | Answer | Consequence |
|---|---|---|---|
| **OD-18** | AWS or Azure | Owner: *"Is there any main benefit to either? I want the solution that is fastest for all of the other stuff, the inferencing is easy, as we can just run a LiteLLM kind of solution that routes all the API keys."* **Orchestrator's call, on that criterion: Azure.** | Phase 8 is already written and part-built for AKS, and Kata Pod Sandboxing is managed there; AWS has no managed Kata (Fargate microVMs lack what agents need, and Kata on EKS needs metal instances). Panacea runs on Azure and is lifted in later (OD-13). KPMG's identity and DevOps are Microsoft. The one AWS advantage, Bedrock in Sydney, is reached from Azure through the inference gateway (OD-19). Phase 8 resumes unchanged. |
| **OD-19** | How agents get inference | **Org-level API through a LiteLLM-style gateway; no personal subscriptions. Two paths, both zero data retention: Global = the direct Anthropic API (the default); AU = Amazon Bedrock in Sydney (ap-southeast-2).** Owner: *"The user should be able to trigger it. The default should be Anthropic, then if triggered it goes to Bedrock"*; and it is selectable by project data classification. | The gateway holds the org keys. Each sandbox gets a **scoped per-session token** (`ANTHROPIC_BASE_URL` plus a token), never an org key. Usage is attributed per user, team and project, with quotas. The switch is per project (a classification of "AU data only" forces AU) and user-triggered. **Supersedes `CLAUDE.md`'s "per-user rotation across that user's OWN Claude subscriptions"**, and the never-pool rule, which existed only because consumer subscriptions forbid sharing: no consumer subscription is used at all. It also retires "`/login` in every sandbox". Phase 6 is re-scoped from "capacity and credential rotation" to **the inference gateway**. |
| **OD-20** | Do users run agents on their own machine | Owner: *"Right now, yes. After this, the goal will be not to. As we are still getting the benefits from running them 'locally', i.e. we are getting the latest claude code cli updates … if you make it good enough, there would be no reason to do things locally for 90% of people."* | **Local mode stays supported now; the product is cloud-first.** The target is that about 90% of people never run locally. That target puts one requirement on the cloud: **it must track the latest Claude Code CLI quickly**, with an automated sandbox-image bump pipeline gated by OD-8's canary test, so cloud is never behind local. The desktop app is the portal to the cloud. |

## Team-mode build decisions — 2026-09-25, OD-21 to OD-24

From the team-mode design's owner questions (`2026-09-24-team-collaboration/00-summary.md`).

| Id | Question | Answer | Consequence |
|---|---|---|---|
| **OD-21** | Which inference gateway (phase 6) | **LiteLLM** | Phase 6 builds on LiteLLM, reusing Panacea's LiteLLM patterns: virtual per-session keys, Global/AU routing, attribution and budgets. No spike of the alternative. |
| **OD-22** | May the cloud fetch a project's repo while nobody is signed in | **Yes, read-only, per project** | A read-only credential scoped to one repository, used only for fetches such as keeping lanes fresh. Every push still uses a signed-in person's own credential. |
| **OD-23** | Cloud agents' Claude Code policy | Owner: *"For the time being, don't draft too much policy. Just give the control plane a way to put our policies in."* | **Build the mechanism, not the policy.** The control plane gets a way to supply a policy (Claude Code managed settings and permissions) per org, and per project where that matters, into every cloud sandbox at session start, versioned and audited. Ship a minimal default. No InfoSec policy document is drafted now. |
| **OD-24** | The four team defaults | **All four accepted**: budgets on by default; the escalation defaults (tighten, never loosen); opening prompts private; a data classification required. Owner, on the last: *"Data class should be set on creation, and can be updated as needed. It will also determine what security scans are needed from a deployment stand point."* | **The data classification is set when a project is CREATED, not at Share, and can be updated.** It drives two things: AU vs Global inference (OD-19), and **which security scans the deploy side requires** (phase 7 and Panacea's scan posture). The project model, phase 4's create flow and phase 7's release gates all carry it. |

## The work layer (Linear parity-plus) — 2026-09-25, OD-25 to OD-28

The owner's ask, verbatim: *"make sure that we are doing everything they are doing and more. Make
it prettier, better, everything. Mixed with the other benefits we have found, and the ability for
agents across owners to work together."* The design is in `2026-09-25-linear-parity/`.

| Id | Question | Answer | Consequence |
|---|---|---|---|
| **OD-25** | When the work layer is built, relative to team mode | **Alongside team mode** | Phase W runs on its own streams beside Phase T (team mode). W1's demo works in local mode first, then moves to the cloud when the gateway and environment land. Team mode's critical path is untouched. |
| **OD-26** | How far to commit | **Everything: W1 to W5** | Full parity-plus: about 138 focused days plus team mode, across all 257 Linear items (MATCH 139, EXCEED 67, PARTIAL 24, SKIP 27). The 27 skips stand as the design argues (Q6), until the owner overrules. |
| **OD-27** | Progress displays | **Proof per criterion, no predictions** | Show which acceptance criteria are proven, and plain counts per group. Still banned: percentages, progress bars, ETAs and "N of M" completion predictions. `AGENTIC-UX.md` gets the design's four one-line amendments (Q13). |
| **OD-28** | The mono face for machine facts | **Commit Mono** (OFL, free) | Every id, path and time is set in Commit Mono. The sans is **Segoe UI Variable** (Q14's recommendation, accepted under OD-1). |
| **OD-31** | How to cut the spend (asked 2026-09-26: *"I feel like we are using a shit ton of tokens and usage … how close we are to being able to test the solution"*). The measured cost was 964 agents, 125M output and 16B cache-read tokens, with about 11% of the non-phase-2 build approved. | **Phase 2 first, then trim** | Only the phase 2 close-out runs until the cockpit is merged to `main` and in the owner's hands. That covers task 26, task 25, the final review and X-3. **Paused:** hub, env, phases 3 to 7, team's plan fix, and the work plan's writing (OD-29's "everything now" is paused, not reversed). **Then:** at most 4 streams at once; 1 review round per task (a fix plus one scoped re-review), with a second only for a Critical; mutation testing only for security-sensitive code; reviewers do not re-run the whole suite. Every restart reports tasks done per stream, how close the product is to testable, and the spend. **Amended the same evening:** *"I am not going to test, you just get it done in the more streamlined manner so we aren't just wasting tokens."* So there is no pause for owner testing: the lighter process (`args.liteFrom`) is on in every stream now, with at most 4 streams at once, rotating through the queue. |
| **OD-30** | How users sign in (asked 2026-09-25: "can we not just use the Panacea Entra ID stuff?") | **A Keycloak broker on Panacea's existing Entra app registration, with no clicks**: *"keycloak broker, but, because it is an app, could it also auto go through Entra, rather than any clicks, while still being managed by Keycloak?"* | The desktop signs in with auth-code + PKCE against a Keycloak realm, not directly against Entra. Keycloak brokers to Entra through the registration that Panacea's environments already share (client id `37f8bd70…`; the repo names it `AU-PROD-Panacea-RITM6316333`, and whether it is the planned "Panacea Platform API" is the ruling's U-1), so **no new Entra registration and no IT ticket for sign-in**. Zero clicks: the desktop client's browser-flow override skips Keycloak's page (`kc_idp_hint` is sent and can only name `entra`, the realm's one IdP; ruling K1), Entra device SSO and `login_hint` skip Entra's prompts, first-broker-login auto-links, and the refresh token in Credential Manager (OD-6) makes later launches silent. Until phase 8 runs its own Keycloak, the realm is Panacea's; the realm change is an ask for Panacea's update (OD-13 keeps Panacea separate). Azure DevOps stays a personal access token (Q5 flips) unless IT later grants ADO consent. **Supersedes** OD-6's "directly against Entra", ADR-H1 §5.2 ("direct Entra, no broker"), phase 8's OQ-4, and Part 1 of the IT request. The ruling is `2026-09-25-identity-keycloak-broker.md`. |
| **OD-29** | Plan depth for Phase T and Phase W (asked 2026-09-25, about 234 part files, about 66 agent-hours) | **Everything now** | Every part file of both plans (T: 69 tasks; W: W0 to W5, about 165) is written before either build starts, instead of writing later slices when their build begins. Accepted risk: parts for late slices (TM2, W4, W5) may drift from the code by the time they are built, so each task's implementer re-verifies its part's citations first. |

The Linear design's other questions take their recommended answers under OD-1:
- Q3: issue text goes into the agent's context, with guards.
- Q4: the Desk frame is in W1.
- Q5: the replica lives in memory.
- Q7: Linear's "project" is called a *feature*.
- Q8: no title slug in AU-only branch names.
- Q9: build our own, and sync one way in.
- Q11: option B.
- Q12: cross-team work comes in W4.

## Licence and compliance — 2026-09-28, OD-32 to OD-35

These four answer the open-source adoption research
(`docs/superpowers/planning/2026-09-28-oss-adoption/`), which surveyed 180 candidates across seven
domains, verified every licence from source, and had its recommendations attacked by a sceptic
(`08-sceptic.md`, **Recommendation quality: Needs fixes** — it disproved most of the claimed
savings). Its honest headline: open source removes about **7 of ~433 unbuilt tasks**, roughly
1.6%. Three of the four decisions below are therefore about compliance, not speed.

### OD-32 — the IaC engine is OpenTofu, and the secret store is OpenBao

**HashiCorp Terraform has been BUSL-1.1 since August 2023**, which is on this program's forbidden
list: we ship to customers' own clouds. **OpenTofu (MPL-2.0) replaces it.** The same ruling
replaces **Vault with OpenBao**.

- The **providers do not change**: `hashicorp/terraform-provider-azurerm` is MPL-2.0, with no
  Change Date and no BUSL heading (verified 2026-09-28). Only the engine moves.
- `.tf` syntax is compatible, so the 14 `.tf` files already in `env`'s `infra/` largely survive.
  What changes is the CLI invocation, the lock file, the provider source addresses, every pipeline
  and script that shells out, the plan text, and the tests that assert on either.
- This blocks phase 8 tasks 01-10, 28 and 30. It was decided the moment it was found because the
  cost grows with every further Terraform task written.

### OD-33 — phase 6 keeps LiteLLM (OD-21 stands) and gains two compliance tasks

Two defects were verified in LiteLLM itself, not in our configuration of it:

1. **The AU-residency leak.** `/v1/messages/count_tokens` ignores a custom `api_base` and calls
   `api.anthropic.com` with the deployment's own key (upstream issues #31149, open since
   2026-06-24, and #42073; both verified open on 2026-09-28). **Claude Code calls that endpoint on
   the hot path**, so an OD-19/OD-24 project classified *AU data only* silently reaches Global.
   That makes the residency promise false. **New task: an AU-residency canary** that asserts no
   request leaves for `api.anthropic.com` on an AU-only project. It can be written and made to
   fail today, before any phase 6 code exists to argue with.
2. **The image ships a file whose licence forbids distribution.** The stock `berriai/litellm`
   Dockerfile copies `enterprise/` into the shipped image (lines 75 and 150), and that directory's
   licence forbids copying, merging, publishing and distributing. We would publish that image into
   a customer's own tenancy. **New task: rebuild the image from the MIT tree with `enterprise/`
   removed, pinned by digest.**

So phase 6 is 29 tasks, not 27. The research's claim that "18 of 27 become configuration" is
measured against a hand-built gateway we were never going to build; it is not a saving.

### OD-34 — the canvas keeps its own op-log and authority model

Phase 5 tasks 05, 06, 10, 11 and 12 build an op-log plus authority document model with `Apply`,
`ifRevision` and per-author undo. The research recommended Yjs instead. **We keep ours.**

- The op-log gives per-author undo and `ifRevision`, which a plain CRDT does not.
- Yjs would force a runtime choice we do not want: Go owning the document through `ygo` (about 55
  stars) or a Node Hocuspocus sidecar inside every Kata image.
- Tasks 04, 07, 08 and 09 — the licence gate, the dialect, canonical HTML and JSX — were ours
  under either answer, so nothing there was ever at risk.

The phase 5 build does **not** pause. This decision closes the fork; do not re-open it without new
evidence.

### OD-35 — build the full plan; no scope cut

Asked whether to cut scope with about 53 of ~486 tasks approved, the owner chose to **keep
building the full plan**. Phase W's 187 tasks and Phase T's 69 stay in. The two large "fold-in"
recommendations were in any case refuted: Phase T carries 1,009 cross-file citations across 65 of
69 part files and Phase W 1,678 across 187, so those plans are frozen, not "still a document", and
a fold-in is a rewrite rather than a plan edit.

**Also rejected, so nobody researches them again:** `@floating-ui` (the code it would replace is
~15 lines inside a 55-line `Popover.tsx`), `coreos/go-oidc` for Hub task 10 (phase 8 already built
`internal/entra` — stdlib-only RS256 and JWKS with tenant-confusion tests — to save half an hour),
`goose` (it records a version, not the SHA-256 of the migration text, which is the requirement),
`cmdk` and `tinykeys` (`CommandPalette.tsx` and `useKeyboardMap.ts` are already built and tested),
and the `@xterm/*` addons for now (`addon-webgl` 0.19.0 is 29 months old and peer-depends on
`@xterm/xterm ^5` while we are on `^6`). The one survivor worth taking later is list
virtualisation (TanStack Virtual, MIT): `DESIGN-SYSTEM.md:90` requires it above 50 rows and `:65`
requires log panes that never scroll-jump, and neither has any implementation in the tree today.

## Where the older documents now disagree, and what wins

- `CLAUDE.md` → "Design canvas: adopt `Projects/Paperdesign`": **superseded by OD-11.**
- `CLAUDE.md` → "Hosting: we-host SaaS on Panacea's AKS first": **superseded by OD-5.** It is
  we-host SaaS on our own environment, modelled on Panacea.
- `CLAUDE.md` → "Reuse from the Panacea monorepo, do not rebuild": **still true as a guide.** Panacea's
  patterns are the model and its API is phase 7's target (OD-13). No Panacea source is copied now.
- `PROGRAM.md` → "Identity: EntraID via the existing oauth2-proxy → Keycloak chain": **superseded by
  OD-6** for the desktop. The Hub ADR decides whether a broker (Keycloak) sits in front of Entra
  server-side.
- `PROGRAM.md` → Phase 5, "Adapt Paperdesign": **superseded by OD-11.**
- `PROGRAM.md` → open risk 4 (Paperdesign licence): **closed by OD-11.**

## What earns an OD

Thirty-five decisions are on this page and until now nothing here said what a decision *is*. The
three-part test below is adopted from the method surveyed in `2026-09-28-discovery-skills/` — the
method only; none of that source's wording is used. A question earns an `OD` row when **all three**
are true.

1. **Expensive to undo.** Reversing it costs code, data or a re-plan, not a setting. This excludes
   anything a toggle, a config value or a single-file edit walks back — those are notes, and a note
   belongs in the plan it affects, next to the work.
2. **Not derivable from the tree.** An agent reading the repo cold would guess wrong, or would not
   know the question existed. This excludes restatements of what the code already says: where the
   answer is legible from the source, the source is the record and a second copy of it rots.
3. **A real fork.** Two defensible options existed and one was given up at a named cost. This
   excludes the obvious call — if every alternative is worse for everyone, nothing was traded and a
   future reader has nothing to weigh.

All three, not two. Each limb removes a different kind of clutter, and a register that admits easy,
obvious or already-legible calls stops being worth reading end to end — which is the only way a
register is ever read. The test is deliberately harsh: most questions should fail it.

**What the gate is not for.**

- **It does not govern orchestrator rulings.** A ruling is the structural call an agent makes when
  its inputs do not cover the case in front of it (`2026-09-25-identity-keycloak-broker.md` is one).
  Rulings are made under authority this register already granted, and they are recorded where the
  work is. A ruling that turns out to need the owner becomes a pending question, and then an `OD`.
- **It does not create `docs/adr/`.** We have a register, with owner authority, that `CLAUDE.md`
  says wins over anything older. A second decision store is the failure `PROGRAM.md` already paid
  for once. We are adopting a test, not a second system.
- **It is not retrospective — and that claim is about ids, not lines.** Nothing already numbered is
  deleted or renumbered, because every `OD` id is cited across the programme. Line anchors are the
  separate hazard the id argument does not cover: 41 citations across 12 documents point into this
  file by line range, and an insertion above the decision tables moves all of them silently, with no
  broken link to notice. That is why this section sits here, after the decisions, rather than at the
  top where it was first written. **Anything added to this file later goes below the tables too.**
  The backwards audit at the end of this file names the rows that would not pass today and leaves
  every one of them in place.

The dependency frontier, immediately below, records the edges between decisions once.

## The dependency frontier

What each decision waited on, and what it released. It is one table rather than two more columns on
the five decision tables above, whose rows are already paragraph-length and would become unreadable.

**Only decisions with an edge appear.** An edge is recorded only where this file, or a document it
names, states the link. A phase consequence restated from the row's own *Consequence* cell is not an
edge. 29 of the 35 decisions carry at least one; **OD-2, OD-4, OD-7, OD-9, OD-28 and OD-35 carry
none**, and that is the honest answer for them, not an omission.

Supersession is **not** a dependency edge and is not listed here — a superseded decision blocks
nothing, it has simply stopped being in force. Those chains stay inside the rows that record them
(OD-30 over OD-6, OD-11 over the Paperdesign line, OD-19 over `CLAUDE.md`'s rotation rule).

| Id | Unblocked by | Unblocks |
|---|---|---|
| **OD-1** | — | The phase 3, 4, 5 and 7 plans and builds; every "accepted as written" recommendation on file |
| **OD-3** | — | Phase H, with OD-5; phase 4's brief store, which needs it to exist first |
| **OD-5** | — | Phase H; phase 8's growth from "Kata backend" to the product's own environment |
| **OD-6** | — | The Credential Manager store that OD-30's silent relaunch reuses, and that holds OD-4's two host tokens |
| **OD-8** | — | OD-10 (deletion by minted id is allowed); OD-20 (the canary that gates an image bump); every later phase's transcript adapter |
| **OD-10** | OD-8 | Phase 3's worktree-removal verb, which calls the tested store-deletion seam |
| **OD-11** | — | Phase 5's re-scope to a clean-room canvas; OD-34, whose tasks exist only under that scope |
| **OD-12** | Phases 3–7 being built (OD-1, ordered by OD-31) | The verification of every owner-observed item, all of which stay unverified until that session happens |
| **OD-13** | — | Phase 7 as an API client; OD-30's realm, which stays Panacea's until phase 8 runs its own |
| **OD-14** | OD-5 (an environment to run them in) | OD-15, OD-16, OD-17, OD-22, OD-23; the ADR-H1 and phase 8 re-scope |
| **OD-15** | OD-14 | Phase T's awareness surfaces |
| **OD-16** | OD-14 | Phase T's Orchestrator channel and its escalation design |
| **OD-17** | OD-14, OD-15, OD-16 | Phase T's plan (with OD-29) and its build straight after the current wave |
| **OD-18** | OD-19 — Bedrock is reached across clouds through the gateway, so AWS's one advantage does not decide it | Phase 8 resuming unchanged on AKS with Kata |
| **OD-19** | — | OD-21; OD-24's AU/Global switch; OD-33; phase 6's re-scope to the gateway; retires `/login` in every sandbox |
| **OD-20** | OD-8 | The automated sandbox-image bump pipeline |
| **OD-21** | OD-19 | OD-33 — both of its compliance tasks are defects in LiteLLM itself |
| **OD-22** | OD-14 | Keeping lanes fresh while nobody is signed in |
| **OD-23** | OD-14 | The control plane's per-org policy channel into every cloud sandbox at session start |
| **OD-24** | OD-19 | OD-33's residency canary; phase 4's create flow; phase 7's release gates and scan selection |
| **OD-25** | — | OD-26; OD-29's phase W half; phase W's streams beside phase T |
| **OD-26** | OD-25 | Phase W's 187 tasks, which OD-35 then keeps in |
| **OD-27** | — | `AGENTIC-UX.md`'s four one-line amendments (Q13) |
| **OD-29** | OD-17, OD-25 | Phase T's 69 and phase W's ~165 part files — the frozen plans OD-35's refutation rests on |
| **OD-30** | OD-6, OD-13 | The Hub's identity tasks; phase 7's `pnc_` key question; pending questions 5, 7 and 8 |
| **OD-31** | Phase 2 merged to `main` and in the owner's hands | The paused work: the Hub, the environment, phases 3 to 7, team's plan fix, and the work plan's writing |
| **OD-32** | — | Phase 8 tasks 01–10, 28 and 30 |
| **OD-33** | OD-19, OD-21, OD-24 | Phase 6's two new tasks: the AU-residency canary and the rebuilt image |
| **OD-34** | OD-11 | Phase 5 tasks 05, 06, 10, 11 and 12 |

## Backwards audit: which rows would not pass the gate

Read-only pass over all 35 rows, 2026-09-28. **Nothing was deleted, renumbered or rewritten.** Every
`OD` id is cited across the programme, and a renumber has already cost this program one sweep. The
only purpose is that a future reader knows the register is **not uniform**: some rows are decisions
and some are notes that were written down in a decision round because that is when they came up.

**Six rows read more like a note than a decision.**

| Id | Limb it misses | Why |
|---|---|---|
| **OD-7** (backup repo) | expensive to undo; a real fork | A second remote and a PAT path. It reverses with `git remote remove`, and no alternative was given up. Useful to know, not a decision |
| **OD-9** (Ctrl chords) | expensive to undo; a real fork | "Match VS Code, plus a toggle" is the obvious call, and it reverses by flipping the very setting the row adds |
| **OD-10** (Forget deletes the transcript) | a real fork | *Forget* deleting what it forgot is the only answer the word allows. The row has since grown a build note — its second half is status from 2026-09-24, not decision |
| **OD-12** (batch the hands-on session) | expensive to undo | A scheduling call the owner reverses any evening by sitting down at the machine. It stays because it defines what "unverified" means in every report |
| **OD-28** (Commit Mono, Segoe UI Variable) | expensive to undo; not derivable | Two font tokens, reversible in one file. Worth writing down once so nobody re-litigates it — which is not the same as earning a row |
| **OD-35** (no scope cut) | not derivable | A refusal to change course already implied by OD-26 and OD-29. Its real content is the *rejected* list in its second half, which is evidence for future readers, not a decision |

**Two structural observations, which matter more than the six rows.**

1. **Rows carry status as well as decisions.** OD-10, OD-30 and OD-31 have been amended in place
   with what was later built or re-decided ("**Built 2026-09-24**", "**Amended the same evening**").
   Good for a reader who lands on the row; bad for one who wants the decision alone. Going forward an
   amendment keeps its bold date marker, so the decision stays the first sentence of the Answer cell.
2. **The order is arrival, not structure.** OD-29 sits after OD-31 and OD-30 in the work-layer table
   because that is when it was asked. **The frontier table above is the reading order**; the
   numbering is an id and nothing more.

**Not audited:** whether any decision was *correctly* answered. This pass judged form only — would
the question have earned a row — and changed no row.
