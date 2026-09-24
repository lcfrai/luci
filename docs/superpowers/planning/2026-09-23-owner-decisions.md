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
