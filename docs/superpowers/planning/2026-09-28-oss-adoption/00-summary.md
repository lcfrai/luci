# 00 — Open-source adoption: the decision

**Date:** 2026-09-28. **Inputs:** `01-work-layer.md` … `07-ui-primitives.md` and their seven
`licence-*.md` audits, read in full for their verdict, saving and licence sections. **Ground truth
for "a task":** the task tables in the seven live plans (Phase W 187, Phase T 69, Phase 5 30,
Phase H 44, Phase 3 24, Phase 6 27, Phase 8 35 — 416 of the ~486 planned; the remaining ~70 sit in
phase 2 residue, phase 4, phase 7 and phase 9, which have no comparable table).

**Rule applied throughout:** a candidate the licence auditor marked `no` is dead and is not carried
forward. A candidate marked `conditional` carries its obligation everywhere it is named below.

---

## 1. The headline number

**Open source removes about 7 of the ~433 unbuilt tasks outright, materially reduces about 62, and
adds about 8 new ones. Net: roughly 1.6% of the task count deleted, 14% made cheaper.**

That is the honest answer and it is deliberately unflattering. The reason is structural and it is
good news about the product: **the program's cost is concentrated in the things nobody has
open-sourced, because nobody else has our constraints** — the provenance thread, "done means
proven" (`OD-27`), the cross-owner Orchestrator model (`OD-16`), the real-HTML frame model with an
MCP surface (`OD-11`), and a design system that exists because the first prototype looked
machine-generated.

Where open source does pay, it pays in **days and in defect classes**, not in deleted rows.

### Working, per phase

| Phase | Unbuilt tasks | Removed outright | Materially reduced | Added | What does the work |
|---|---|---|---|---|---|
| **W** — work layer | 187 | **0** | **~22** | +1 | TanStack Virtual/Table, dnd-kit, React Aria, Tiptap, TanStack DB, uFuzzy, participle+squirrel, expr, vendored Gitea |
| **T** — team mode | 69 | **0** | **~13** | 0 | NATS JetStream, centrifuge, River, Cedar, CloudEvents, A2A schema |
| **5** — canvas | 30 | **~1** (task 14) | **~8** | +1 (spike) | Moveable, Selecto, use-gesture, rbush, React Flow, MCP Go SDK, Playwright, Excalidraw |
| **H** — Hub | 44 | **~2** (06, 10) | **~8** | +2 | x/oauth2, coreos/go-oidc, go-keyring, goose, transparency-dev/merkle, in-toto format |
| **3** — git lanes | 24 | **0** | **~4** | +1 | go-gitdiff, sergi/go-diff, go-github, azure-devops-go-api |
| **6** — gateway | 27 | **0** | **0** | **+2** | **Nothing new — `OD-21` already banked LiteLLM. This research ADDS work.** |
| **8** — environment | 30 (5 approved) | **~4** | **~7** | +1 (spike) | kubernetes-sigs/agent-sandbox, Kyverno, kube-fledged, OpenTofu, External Secrets |
| Outside these plans | ~70 | 0 | ~10 (ph. 2/9) | 0 | six `@xterm/*` addons, `tauri-apps/plugins-workspace`, asciinema-player, keyring-rs |
| **Total** | **~433** | **~7** | **~62** | **~8** | |

### Three numbers that are easy to get wrong

**Phase 6 is not a saving. It is a bill.** `OD-21` already locked LiteLLM, and the 27-task plan is
already written on top of it — task 02 generates a LiteLLM config, task 04 is a LiteLLM admin
client, task 07 mints its virtual keys. `06-platform.md`'s "about 18 of 27 tasks become
configuration" is measured against a *hand-built* gateway that we were never going to build. What
this research actually contributes to phase 6 is **two new tasks and one defect**:

| New phase 6 work | Why |
|---|---|
| **Rebuild the LiteLLM image from the MIT tree with `enterprise/` removed, plus a CI layer check** | The stock `berriai/litellm` Dockerfile copies `enterprise/` into the shipped image (lines 75, 150). That directory is the **BerriAI Enterprise License: "forbidden to copy, merge, publish, distribute…"**. We ship to customers' own clouds. Half a day; blocks the first customer-hosted deploy, not today. |
| **An AU-residency conformance canary for `/v1/messages/count_tokens`** | LiteLLM ignores a custom `api_base` on that endpoint and calls `api.anthropic.com` with the deployment's own key (issues #31149 open since 2026-06-24, #42073; **verified open 2026-09-28**). Claude Code calls it on the hot path, so an `OD-19`/`OD-24` "AU data only" project leaks to Global silently. **Write the test before any phase 6 code.** |

**Phase W's 12% is real and it is not the difference between shipping and not shipping.**
`01-work-layer.md` puts it at ~17 of ~138 focused days. W1's cost is 33 desktop days and 11 Hub
days; almost all of the adoptable machinery is in the desktop third. W3 (agents on the work,
~12.5 d) gets **zero** — no permissively licensed project touches it.

**The plan already adopted some of this.** Phase W task 49 already says *"one Markdown editor on
`prosemirror-*` (MIT)"*; phase W task 179 already says *"through a permissively licensed differ"*;
phase 8 task 06 already says *"Overlay + Cilium"*; phase H task 06 already says *"`pgx` pinned"*;
phase 5 task 04 already builds the `tools/licencegate` this report keeps asking for. Do not count
those twice.

---

## 2. The ranked adopt list

Best value first. The licence column is the auditor's verified verdict; `CONDITIONAL` items carry
their obligation with them and it is restated in §6. Integration costs are the researchers'
estimates and are unmeasured.

### Tier 1 — do these regardless of any other decision (cheap, certain, no architecture change)

| # | Component | Licence | Tasks it removes / reduces | Integration cost | What we still build |
|---|---|---|---|---|---|
| 1 | **Six `@xterm/*` addons** — `webgl`, `serialize`, `search`, `unicode11`, `web-links`, `clipboard` | MIT (each package verified individually) | Phase 2 residue + phase 3 task 21. WebGL is the renderer answer to Columns' 9 panes and Canvas' 10 tiles, each an independent live terminal — DESIGN-SYSTEM's 10 Hz rule is a throughput rule the DOM renderer loses. `serialize` gives exact-screen reattach with **no replay protocol to design**, and its HTML mode can carry a provenance id as an attribute where a screenshot cannot | **1–2 days.** Touches only `makeTerm()` in `desktop/src/components/Terminal.tsx`. No new vendor — same monorepo we already depend on | The WebGL context-loss fallback, and making it an acceptance test |
| 2 | **`tauri-apps/plugins-workspace`** — `deep-link`, `single-instance`, `updater`, `window-state`, `store`, `log`, `notification`, `os`, `process` | Apache-2.0 **OR** MIT (dual, verified at the Rust source) | The largest pure gap-fill in the report. `src-tauri/Cargo.toml` declares only `tauri`, `tauri-build`, `tauri-plugin-opener`. Covers the `OD-30` PKCE callback (`deep-link`), the one-terminal-per-session invariant `attachRegistry` assumes (`single-instance`), the localStorage persistence `CanvasLayout.tsx` complains about (`store`), the delivery mechanism `AGENTIC-UX.md`'s intermittently-present user currently has none of (`notification`), and most of phase 9's distribution | **1–2 days** for the plugins | **Start the KPMG code-signing and update-channel conversation now.** `updater` cannot ship without it, and that is people lead time, not code — `PROGRAM.md` open risk 3 says the same |
| 3 | **`bluekeyes/go-gitdiff`** + **`sergi/go-diff`** | MIT (both) | Phase 3 tasks 16 and 21. `TextFragment` already carries `OldPosition`/`NewPosition`/`LinesAdded`/`LinesDeleted` — the exact coordinate system the hunk-to-provenance back-link (3-D7) needs. Saves hand-writing rename detection, mode changes, the no-newline-at-end marker and binary deltas | **1 day.** Two small Go modules | The back-link itself: which agent turn wrote this hunk |
| 4 | **`google/go-github`** + **`microsoft/azure-devops-go-api`** | BSD-3-Clause / MIT | Phase 3 task 14 — `githost`, both hosts behind one interface (`OD-4`) | **1 day** behind our own one-host interface | The interface, the lane-to-PR mapping, credential handling |
| 5 | **`@floating-ui/react`** | MIT (all five packages in its shipped tree) | The **only genuine deletion of existing code** in the whole report: the viewport-edge flip/shift geometry inside `desktop/src/components/common/Popover.tsx` and `menu/ContextMenu.tsx`. Markup and tokens stay | **Half a day** | Everything visible — only the geometry goes |
| 6 | **`@tanstack/react-virtual`** or **`virtua`** | MIT (both) | Two binding `DESIGN-SYSTEM.md` rules with **no implementation anywhere in the tree today**: virtualise any list over 50 rows, and log panes that never scroll-jump when the user has scrolled up. Serves phase 2's log pane and rail, phase W tasks 40/42/43/45, phase 3 diff lists | **~1 day.** `virtua` is the better fit for the log pane: reverse-scrolled, items inserted above the viewport | Our own ~30-line stick-to-bottom, and the 10 Hz throttle |

### Tier 2 — adopt, but each needs a named decision first

| # | Component | Licence | Tasks it removes / reduces | Integration cost | Decision required |
|---|---|---|---|---|---|
| 7 | **NATS / JetStream** | Apache-2.0 (server, `nats.go`, `nkeys`) | Phase T tasks 19, 23, 26, 28, 31, 35, 39 — five hand-built subsystems: the typed cross-Orchestrator bus, durable per-user Orchestrator inboxes, the who-caused-it stream, presence for the team strip, and file leases (create-or-fail on `lease.<repo>.<path>` with a TTL). **8–12 days of TM1's ~95** | A new runtime component in phase 8: chart, network policy, backup | Write `OD-16`'s *"Orchestrators are the only cross-user channel"* as **NATS subject permissions**, so the security review has something enforceable to read instead of prose |
| 8 | **`kubernetes-sigs/agent-sandbox`** | Apache-2.0 | Phase 8 tasks 12, 14, 15 and parts of 18/19/22/23 — the warm pool, claim/allocation semantics, sandbox lifecycle, TTL and reaping, per-sandbox storage. Sits **behind the existing `Backend` interface (`docker` or `kata`) with no architecture change**. `SandboxWarmPool` has a scale subresource, so a plain HPA drives it | A spike, then a controller we do not own | **Spike `examples/kata-aks/` against a scratch AKS cluster before phase 8's pool controller is written.** Upstream ships the four manifests but **no burst harness — write the load driver ourselves**. One spike decides whether 8–12 tasks are deleted or kept |
| 9 | **TanStack DB** | **CONDITIONAL** — MIT declared, but it ships `@tanstack/db-ivm`, a fork of Apache-2.0 `@electric-sql/d2ts` declared MIT **with no licence file** | Most of phase W tasks 55–58: the in-memory replica, optimistic writes, rollback-on-refusal and `optimistic.undo`. `/v1/changes` becomes a custom collection's sync source. ~2.5 days | Behind **our** `replica/` interface | Two: (a) resolve the `db-ivm` licence question or record the Apache-2.0 ancestry in `THIRD-PARTY.md`; (b) accept BETA status — the interface makes it a swap, not a rewrite |
| 10 | **React Aria (react-spectrum)** *or* **Radix Primitives** — **pick one, not both** | Apache-2.0 (**CONDITIONAL**: NOTICE obligation) / MIT (root; per-component manifests not enumerated) | Phase W tasks 34, 46–48, 66, and the card menus in 62–65. Carries `forced-colors`, grid roles and live regions — W1's accessibility floor is its strongest area and our weakest | **1 day including the ADR** | **Recommendation: React Aria**, written into `DESIGN-SYSTEM.md`. Both are headless, so neither brings a look — which is why they survive §4 |
| 11 | **dnd-kit** | MIT | Phase W tasks 43 (board drag), 79 and 89 (rank/move), 129 (timeline drag-to-reschedule). Also takes a chunk of task 66 — keyboard DnD and announcements come free | ~1 day | Task 43's rule that drag **offers a fact instead of faking it** is ours, and it is the whole point |
| 12 | **Tiptap** (`@tiptap/*` core only) on top of ProseMirror | MIT — all 60 `packages/*` verified individually | Phase W tasks 49 and 50. The plan **already names `prosemirror-*`**, so this is an upgrade, not a new adoption: Tiptap brings the slash menu and tables | ~1 day | **Denylist `@tiptap-pro/*` by name in the licence gate** before anything is installed |
| 13 | **Moveable + Selecto + `@use-gesture/react` + `rbush`** | MIT (all four) | Phase 5 task 25 — transform handles, resize, snapping, alignment guides, marquee, pan/zoom including the trackpad-versus-wheel distinction that is always the bug, and hit-test culling. `02-canvas.md` calls this *the classic two weeks that became six* | ~2 days | Depends on **D-OSS-1 = DOM-first**, which follows from "real-HTML frames" anyway |
| 14 | **MCP Go SDK** | Apache-2.0 AND MIT (live relicence in progress; docs CC-BY-4.0) | Phase 5 task 14 largely **removed**; tasks 18 and 19 reduced — transport, framing, tool registration | ~1 day | **The tool surface is the product and stays ours.** Only the plumbing is adopted |
| 15 | **`coreos/go-oidc` v3 + `golang.org/x/oauth2`** | Apache-2.0 (**CONDITIONAL**, has a NOTICE) / BSD-3-Clause | Phase H task 10 largely **removed**; tasks 30–31 reduced. Covers the whole validation table in `2026-09-25-identity-keycloak-broker.md` sections 3 to 5 — every line of which is a place to write a subtle security bug | Half a day. `x/oauth2` is **already an indirect dependency** — promote it and use its built-in PKCE | `kc_idp_hint` and `login_hint` stay ours (`OD-30`). Verify `go-oidc`'s release cadence; if dormant, `zitadel/oidc` (Apache-2.0) is the swap |
| 16 | **`transparency-dev/merkle`** | Apache-2.0 | Phase H tasks 14 and 27. Append-only is a Postgres grant plus a trigger; **tamper-evident needs a Merkle tree** — a signed tree head proves nobody edited ledger row 4102 after the fact. `ledger_entries` stays in our own Postgres | ~1 day | The ledger schema, chain-head anchoring, and what a proof is shown as |
| 17 | **`pressly/goose`** | **CONDITIONAL** — MIT, but `go.mod` directly requires MPL-2.0 `go-sql-driver/mysql` | Phase H task 06 largely **removed** — the migrate runner, including the ledger's append-only trigger and RLS policies | Hours | Exclude the mysql driver from the build, or record the MPL obligation |
| 18 | **`zalando/go-keyring` + `danieljoos/wincred`** | MIT (both) | Phase H tasks 28–29 (`OD-6`: the Keycloak offline refresh token, the ADO PAT, the GitHub token, in Windows Credential Manager) | Hours | **+1 new task: the 2560-byte Credential Manager limit.** Preferred fix — an opaque handle in Credential Manager, the token in a DPAPI-protected file via `golang.org/x/sys/windows`, which is **already a direct dependency**. **Also settle Go-side (`go-keyring`, Hub task 28) versus Rust-side (`keyring-rs`, Tauri): two token stores is a bug** |
| 19 | **in-toto Attestation v1 as the ledger FORMAT** (plus `in-toto-golang`'s DSSE envelope, vendored) | **CONDITIONAL** — Apache-2.0, short boilerplate notice only | Not a task removal — a decision closed. `provenance-contract-draft.md` section 1.2 **already invented the same shape independently**: brief key plus sha256 digest is the `subject`, link kind is the `predicateType`, link payload is the `predicate` | Hours, if taken before phase H task 02 freezes its golden vectors | It makes phase 9's ISM and audit evidence a **recognised artifact** rather than a bespoke JSON blob. `05-hub-identity.md` calls it the one recommendation most worth putting in front of the owner |
| 20 | **Cedar (`cedar-go`)** | Apache-2.0 (**CONDITIONAL**, has a NOTICE) | Phase T tasks 06, 07, 43, 44. `OD-16` (no user steers another user's agent) and `OD-24` (tighten, never loosen — which falls out of forbid-wins for free) become policy instead of an `if` ladder spread across handlers | ~1 day | **Scope it to Phase T, not Phase H.** `05-hub-identity.md` is right that Phase H's org/project/membership checks are Postgres row-level security and nothing more. The two researchers disagreed; this is the reconciliation |
| 21 | **centrifuge** (the Go library, not Centrifugo the server) | MIT | Phase T tasks 52, 53 and part of 39 — the `OD-15` team strip: who is working, one summary line each, files touched. Replaces a hand-built WebSocket hub, presence tracking and reconnect/recovery on both ends. **4–6 days** | In-process, no new runtime | "Without clutter" is a design problem, not a transport problem |
| 22 | **CloudEvents (spec + `sdk-go`)** | Apache-2.0 (both) | Phase T tasks 01, 03, 08 — the envelope for `directive`, Orchestrator-turn, human-message and escalation records, and the ledger's event shape across Hub, Fleet, desktop and later Panacea's deploy side. 1–2 days | Hours | The record types themselves |
| 23 | **Kyverno + kube-fledged + External Secrets + Azure Workload Identity** | Apache-2.0 / Apache-2.0 (at `master`, **not** `main`) / Apache-2.0 / MIT | Phase 8 task 11 (Pod Security, network policies) and task 12's image pre-pull DaemonSet **removed**. Kyverno enforces the locked Kata constraints at admission: mandatory CPU/memory requests (the guest VM is sized from them), no host networking, `runtimeClass` pinning | ~2 days across four charts | Apply the Cilium egress default-deny **at the same time as the first Kata node pool**. Retrofitting a default-deny to a running estate is the expensive version |
| 24 | **OpenTofu** + `terraform-provider-azurerm` | **CONDITIONAL** — MPL-2.0 (both) | Not a saving — a **licence fix**. HashiCorp Terraform has been **BUSL-1.1 since August 2023** and is on the forbidden list; so is Vault (use OpenBao, MPL-2.0) | Low **now**; a rewrite of every pipeline **later** | **Switch before phase 8's Terraform is written.** This is the most time-sensitive item in the report |
| 25 | **`participle` + `squirrel`** (plus `expr` or `cel-go`) | MIT / MIT / MIT or Apache-2.0 | Phase W tasks 80 and 91 (the saved-view filter grammar, parsed to an AST and compiled to safe Postgres SQL), tasks 162–163 (loop `lp-` triggers), task 147 (intake rules) | ~1 day | **Write the grammar before W2, not after** — its vocabulary (`serves:`, `thread:`, `proven:`) is a product decision that wants settling once. `cel-go` also gives `OD-23` a credible policy-expression surface |
| 26 | **uFuzzy** | MIT | Scoring inside phase W tasks 58 and 59 | Hours | The palette itself is **already built and under test** — do not replace it |
| 27 | **Gitea's issue model, vendored in pieces** — `issue_index.go`, `dependency.go`, `issue_xref.go` | MIT — confirmed at the root **and** per file | Phase W task 08 (the per-project number race), task 11 (blocked-by cycle detection), tasks 154–155 (magic words, linkbacks). ~2 days | Vendor into `hub/internal/vendor/` with notices and a `THIRD-PARTY.md` | Everything around them. **Vendor the three files; do not take Gitea** |
| 28 | **Excalidraw, embedded whole** | **CONDITIONAL** — MIT (package); bundled fonts unverified; transitive `elkjs` (**EPL-2.0 OR GPL-3.0**) risk at mermaid 12 | `OD-11` section 3's sketch mode as a separate embedded surface, agent-editable over MCP on day one. Not one of the 30 phase 5 tasks — optional scope bought cheaply | ~1 day | **Pin away from mermaid 12 and `elkjs`, and verify the bundled fonts** before it ships |
| 29 | **asciinema-player** (player only) | Apache-2.0 at **every** published version — no relicensing boundary exists | Phase 9 session replay linked from the provenance ledger. We write the **recorder** (phase 2 already owns the PTY relay; asciicast is a JSON header plus timestamped output lines — about a day). The player is the months-of-terminal-correctness part | ~2 days total | The ledger link and the provenance id on the recording |
| 30 | **Playwright**, server-side, for frame-to-PNG | **CONDITIONAL** — Apache-2.0 package; the default-installed browsers are separately licensed: Chromium BSD-3, **WebKit LGPL-2.1+**, **Firefox MPL**, **ffmpeg LGPL** | The rendered-PNG-per-approved-screen half of phase 5's handoff bundle (tasks 21–22) | ~1 day | **Install Chromium only.** It is also better than a browser-side screenshot, which we would not trust as evidence |

### Adopt as a FORMAT, not as code

**SLSA Provenance v1** is **CONDITIONAL**: its only licence is the **Community Specification
License 1.0**, a specification licence and not an OSS code licence. Emit it as a second predicate
beside our own `ReleaseLink`, so a release states both *this came from this brief* (ours) and *this
was built by this builder from this source* (theirs). **`CycloneDX/cyclonedx-go`** (Apache-2.0,
**CONDITIONAL**, has a NOTICE) carries the SBOM half of the same release record — which `OD-24`
makes part of the provenance thread, because data classification drives which scans a deployment
requires.

---

## 3. The do-not-adopt list

**Nobody researches these again.** Grouped by why they are dead, because the reason changes what a
future researcher should do with them.

### 3a. Dead on licence — copyleft, source-available or commercial

| Project | Licence found | Would have been |
|---|---|---|
| **Plane** | AGPL-3.0 | A whole tracker for phase W |
| **Focalboard** | AGPL-3.0 for source; MIT only for Mattermost-produced binaries; Apache-2.0 only for admin tools; **plus a trademark restriction** | A whole board for phase W |
| **Webstudio** | AGPL-3.0 | An HTML-backed visual builder for phase 5 — the closest functional match, and unusable |
| **Daytona** | AGPL-3.0 at every release tag checked | The sandbox broker for phase 8 |
| **y-redis / "y-hub"** | AGPL-3.0 OR proprietary (dual) | Horizontal scale for the canvas document server |
| **Liveblocks** | Split: Apache-2.0 for most, **AGPL-3.0-or-later for `packages/liveblocks-server` and `tools/liveblocks-cli`** | The multiplayer stack end to end. **The classic permissive-core / copyleft-server trap** |
| **Huly Platform (Tracker)** | EPL-2.0 | Conceptually most of W1–W2 |
| **elkjs** (transitive, via mermaid 12) | EPL-2.0 OR GPL-3.0-or-later | Nothing directly — it is a **transitive risk inside Excalidraw**. Pin away from it |
| **BlockNote** | **No root licence at all**; core MPL-2.0; `xl-*` packages GPL-3.0 | The phase W editor. Denylist `@blocknote/xl-*` |
| **Meilisearch** | MIT **AND** BUSL-1.1 (Enterprise Edition) | Search in phase W |
| **tldraw** | Proprietary / source-available — already excluded by `OD-11` §3 | The phase 5 canvas viewport |
| **Planka** | NOASSERTION — no root licence file; self-described source-available | Nothing we would take |
| **HashiCorp Terraform and Vault** | **BUSL-1.1 since August 2023** — not MPL any more | Phase 8's IaC engine and secret store. Use **OpenTofu** and **OpenBao** (both MPL-2.0) |
| **charmbracelet/crush** | FSL-1.1-MIT (Functional Source License) | Nothing we need |
| **Anthropic Agent SDK** (`claude-agent-sdk-*`) | Not open source — all rights reserved, commercial ToS | Nothing — `OD-8` has us running the unmodified CLI |
| **Claude apps gateway** | Not open source; no source repository exists | On paper, phase 6 plus `OD-23`'s policy seam. It serves only Anthropic's own clients, and `CLAUDE.md` locks four harnesses |
| **libgit2** (under `git2go`) | `git2go` MIT, **libgit2 GPL-2.0-only with a linking exception** | The go-git alternative for phase 3 |
| **`kochul2000/laymux`** | **No licence of any kind** — no licence means no permission | A terminal multiplexer to read |
| **`awaysu/AwayTerminal2`** | MIT for its own code, but its `THIRD-PARTY-NOTICES.md` names AGPL, GPL-3.0 and LGPL entries | A cockpit to read |
| **`dockview-enterprise`** | Proprietary, inside an otherwise-MIT monorepo | Docking layout. We do not want VS Code docking anyway |
| **`react-virtuoso`** | Auditor verdict `no`: **the repository has no LICENSE file** (`api.github.com/.../license` returns 404 as at 2026-09-28); MIT text exists only in the published tarball | The phase 2 log pane. `virtua` or TanStack Virtual serve the same slot with clean provenance |
| **Paperdesign / `kgoedecke/doop`** | AGPL-3.0 (`package.json:5` = `AGPL-3.0-only`; **§13 Remote Network Interaction** at `LICENSE:540-546`) | The whole design canvas. Withdrawn by `OD-11`. **Not opened, not read, not searched.** Phase 5 is a clean room |

### 3b. Licence is clean — rejected on architecture, on `OD-8`, or because we already built it

| Project | Licence | Why not |
|---|---|---|
| **`coder/agentapi`** | MIT | **Rejected on `OD-8`, not licence.** It drives Claude Code inside a terminal emulator and **reads the rendered screen** to synthesise a chat API. That is exactly the class phase 1 and `OD-8` ruled out. Its README makes it look like a shortcut — write the reason down |
| **`SWE-ReX`** | MIT | Same: it extracts command output and exit codes by **sentinel-matching PTY bytes**. Study the multi-backend session abstraction only |
| **`cmdk`** | MIT (licence clean; four Radix deps MIT) | `desktop/src/components/palette/CommandPalette.tsx` (145 lines) and `commands.ts` **already exist and are under test**. Also: **last release 2025-03-14, 18 months**, and it drags a Radix dialog into a shell with its own overlay model |
| **`tinykeys`** | MIT (licence clean) | `desktop/src/hooks/useKeyboardMap.ts` (243 lines) already exists, including a hard-won xterm key-routing fix. Adopting it would delete our own fix |
| **`go-git/go-git`** | Apache-2.0 | **Cannot do the job.** Its own `COMPATIBILITY.md` lists as unsupported: `bundle`, `update-ref`, `rebase`, `notes`, `gc`; merge is **fast-forward only**; pack-protocol **v1 only**. That is precisely phase 3's critical path — R4's bundle crossing, A5's CAS land, 3-D4's `merge --no-ff`, modern host transport |
| **Konva, Fabric.js, PixiJS** | MIT | Rejected by **D-OSS-1 = DOM-first**. "Real-HTML frames" that an agent can read back is incompatible with a canvas-pixel engine |
| **`fluxcd/go-git-providers`** | Apache-2.0 | The ready-made multi-host abstraction for `OD-4`, and the wrong abstraction for our lane model |
| **`kbar` / `ninja-keys`** | MIT (both) | Alternatives to `cmdk`, and `cmdk` is already rejected |
| **`Fuse.js`** | Apache-2.0 | uFuzzy is smaller and faster for the same slot |
| **Rocicorp Zero** | Apache-2.0 (verified twice) | Alternative to TanStack DB; a much larger commitment for the same phase W tasks |
| **`@xterm/addon-attach`** | MIT | Looks like it replaces `desktop/src/api/attachRegistry.ts`. It does not — it has no reconnect, no backpressure, no session model |
| **`tmux` in the sandbox** | ISC | Session survival across a relay restart for free — and it puts a second terminal emulator between the harness and the PTY, which `OD-8` and phase 1 both rule out |
| **Knative Serving** | Apache-2.0 | Request-scoped; our sessions are long-lived interactive PTYs |
| **Langfuse** | MIT core + Langfuse Enterprise License in `ee/`, `web/src/ee/`, `worker/src/ee/`; copyright now **ClickHouse, Inc.** | LiteLLM already gives spend per key/user/team plus Prometheus metrics |
| **Any styled component library** | various | `DESIGN-SYSTEM.md` is binding and exists **because the first prototype failed by looking machine-generated**. A library that brings its own visual language is a permanent tax |

### 3c. MPL-2.0 and dual-licence items — usable, but the obligation travels with them

MPL-2.0 is **file-level** copyleft. The obligation: if we **modify** an MPL-covered file and
distribute the result, that file's source must be made available. Using it unmodified, linked into
proprietary software, triggers nothing. The practical rule: **never patch these in place — wrap
them.**

| Item | Licence | Obligation |
|---|---|---|
| **OpenTofu**, `terraform-provider-azurerm` | MPL-2.0 | Fine as a build tool and a provider binary. Do not fork a provider file |
| **River** (phase T durable turns) | MPL-2.0; a commercial "River Pro" sits beside it as a paid private Go module | Vendor unmodified. Do not take River Pro |
| **`pressly/goose`** | MIT, but pulls MPL-2.0 `go-sql-driver/mysql` | Exclude the mysql driver |
| **`@melloware/react-logviewer`**, **satori**, **Penpot**, **Bugzilla** | MPL-2.0 | Study-only or unmodified use |
| **Falco** `driver/`, **Tetragon** `bpf/`, **Cilium** `bpf/` | Apache-2.0 root with **dual GPL-2.0 / BSD-2-Clause or MIT** datapath | We consume the **binaries and the Apache-2.0 userspace**, not the eBPF source. Do not vendor the datapath |
| **gVisor** | Apache-2.0 **plus** per-file MIT **plus** per-file BSD-3-Clause | Tri-licensed, not flat Apache — the licence gate must not assume one SPDX id per repository |
| **`shiki`** grammars | shiki's own code MIT; the **155 bundled TextMate grammars are 111 MIT, 22 Apache-2.0, 12 NOASSERTION, 5 GPL-3.0, 2 MPL-2.0, 1 "GNU", 1 ISC, 1 BSD-3** | **Ship a subset.** Pick the languages we highlight and exclude the GPL and NOASSERTION grammars explicitly. This is the single most easily missed trap in the whole report |
| **`git` itself** | GPL-2.0-only | **Fine, and it is the locked approach.** We invoke it as a subprocess; we do not link it. Phase 3's A5 plumbing plan is confirmed by the research, not replaced |
| **`humanlayer`** | Apache-2.0 **by reference only** — the LICENSE file is a header stub, not the licence text | Study-only. Do not take code from it until upstream fixes its LICENSE |

---

## 4. What must stay ours

Argued from `CLAUDE.md`. The test is not "is there a library" — it is **"does the thread survive"**.
`CLAUDE.md`: *"Any feature that breaks that thread is wrong regardless of how good it is in
isolation."* The corollary for this report: **adoption that breaks the thread is wrong however much
time it saves.**

| What | Why nothing open source can carry it | Tasks it is |
|---|---|---|
| **The provenance thread** — brief written in Define, visibly referenced in Design, carried into Develop, attached to the release in Deploy | It is a **cross-phase identity**, not a feature. Every candidate that touches it is a *format* (in-toto, SLSA, CycloneDX, CloudEvents) or a *transport* (NATS, centrifuge) — none of them knows what a criterion is. `05-hub-identity.md` puts it plainly: *nobody has open-sourced our product* | Phase H 18–24; phase 5 21–22; phase 3 15–17; phase W 09, 23; phase 4 throughout |
| **"Done means proven" (`OD-27`)** — proof per criterion, no predictions; no percentages, progress bars, ETAs or N-of-M | Every tracker on earth models *done* as a status column. The evaluator over ancestry, *done, not proven*, and observed evidence at the landed commit exist in **no** tracker, permissive or otherwise. Phase W's test kit (`expectNoPrediction`, task 20) is a rule no library can enforce for us | Phase W 10, 22, 23, 48, 104; phase 3 15 |
| **The design system** | It exists *because the first prototype failed by looking machine-generated*. Adopting a component library that brings a look is negative progress. This is exactly why the surviving UI adoptions are all **headless** — React Aria, Radix, TanStack, dnd-kit, floating-ui, virtua carry behaviour and accessibility, never pixels. One global heartbeat clock, 1px ring plus layered shadow, `tabular-nums`, 10 Hz log throttle: all ours | Phase W 26–36, 95–96 |
| **The agent-fleet cockpit** | The survey looked hard for someone who had already built it (`07-ui-primitives.md` Appendix A; `03-agent-fleet.md` §5.2). The closest permissively licensed whole products are `Surething-io/cockpit` (37 stars) and `timwukp/agent-terminal` (0 stars). Read the UX, take no code | Phase 2, and all of phase T's surfaces |
| **The cross-owner Orchestrator model (`OD-16`)** | **No permissively licensed project implements it.** The closest, `humanlayer`, does human-in-the-loop approval for a *single* owner. That gap **is the product** | Phase T 25–31, 40–42, 62 |
| **The real-HTML frame model and its MCP tool surface (`OD-11`)** | GrapesJS (BSD-3) shows the *shape* of an HTML-backed, agent-editable component tree and is the best thing to read. Webstudio does it properly and is AGPL. The tool surface — `list_frames`, `read_frame`, `patch_element` — is the differentiator, and it is ours | Phase 5 05–09, 15–17 |
| **The Define → Design → Develop → Deploy journey** | Four phases as **states of one project**. Every adoptable component sits inside one phase. Nothing spans them, because spanning them is the thing nobody else has built | All of it |

**One adoption needs watching against this test.** TanStack DB (#9) is the only candidate that
would touch the shape of data the desktop holds. Keeping it behind our own `replica/` interface is
not tidiness — it is what stops a third-party collection model from deciding what an issue is.

---

## 5. The order to do it in, and what should pause now

Three streams are in flight: **the phase W plan fix, the phase T plan fix, and phase 5's build.**
The rule that decides each case: **a plan edit is cheap now and a code rewrite is expensive later.**
Under `OD-31` the priority is still phase 2's close-out and a streamlined process, so nothing below
asks for a new stream.

### Pause now

| Stream | Pause what, exactly | Why |
|---|---|---|
| **Phase 5 build** | **Task 25** (viewport, selection, marquee, move, resize, snapping) and **task 14** (the MCP protocol layer). Do **not** pause tasks 04, 07, 08, 09 — the licence gate, the dialect, canonical HTML and JSX are ours regardless | Task 25 is exactly what Moveable + Selecto + `@use-gesture` + rbush replace, and `02-canvas.md` calls it the classic two-weeks-became-six task. Task 14 is largely the MCP Go SDK |
| **Phase 5 build — a bigger question** | **Before tasks 10, 11 and 12 are built**, answer D-OSS-2: does the collaborative document use **Yjs** or the plan's current op-log-plus-authority model? | The current plan (tasks 05, 06, 10, 11, 12) is an op-log with `Apply`, `ifRevision` and per-author undo — **not a CRDT**. Adopting Yjs after those are built is a rewrite of five tasks, not an addition. And Yjs forces a runtime choice: Go owning the document via `ygo` (MIT, ~55 stars — gate behind our own byte-level conformance test), or a Node **Hocuspocus** sidecar in every Kata image. **This is the single largest architectural fork in the report** |
| **Phase T plan fix** | Fold in **NATS, centrifuge, River, Cedar and CloudEvents before the part files freeze** | Tasks 01, 03, 06, 07, 08, 19, 23, 26, 28, 31, 35, 39, 43, 44, 52, 53 are all currently written as hand-built. This is **the largest single saving in the report** (16–24 days of TM1's ~95) and it is a plan edit today |
| **Phase W plan fix** | Fold in the headless-primitives decision, and **settle React Aria vs Radix in `DESIGN-SYSTEM.md` before W1's part files freeze**. Also move the filter-grammar decision (`participle`) ahead of W2 | Tasks 34, 40, 43, 46–50, 55–58, 66, 80, 91 are written as hand-built. The grammar's vocabulary (`serves:`, `thread:`, `proven:`) is a product decision that wants settling once |
| **Phase 8** | **Do not write the Terraform yet** | Switch the engine to **OpenTofu** first. Terraform is BUSL-1.1. Changing it after tasks 01–10, 28 and 30 exist is a rewrite of every pipeline that calls it |
| **Phase 6** | **Do not write the Helm chart (task 19)** until the LiteLLM image is rebuilt from the MIT tree with `enterprise/` removed | Publishing the stock image into a customer tenancy distributes code whose licence forbids distribution |

### Do not pause

Phase 2's close-out, and phases 3, H and 4. Phase 3's architecture is **confirmed** by the research,
not replaced — the four dependencies it adopts (#3, #4) drop into tasks 14, 16 and 21 without
changing any plan. Phase H's adoptions are all drop-ins too, with one exception: **the in-toto
format decision should land before task 02 freezes its golden vectors.**

### The order

| # | Do | Effort | Why first |
|---|---|---|---|
| 1 | **Extend `tools/licencegate`**: denylist `@tiptap-pro/*`, `@blocknote/xl-*`, any `meilisearch` enterprise package, `elkjs`; run over the **transitive** tree, not direct requires; flag MPL-2.0 rather than fail it; fail on AGPL/GPL/LGPL/SSPL/BUSL/FSL/Elastic/Commons-Clause and any unrecognised string | **Half a day** | Before anything is installed. A prefix rule is cheaper than a reviewer's memory |
| 2 | **Phase 6: write the AU-residency `count_tokens` canary**, then rebuild the LiteLLM image from the MIT tree | **1 day** | The only item whose absence is a **compliance finding**, not a schedule item |
| 3 | **Decide OpenTofu**, and record it before phase 8's Terraform is written | **Hours** | Most time-sensitive; the cost curve is vertical |
| 4 | **Phase 2 residue: the six `@xterm/*` addons, then the Tauri plugins.** In parallel, **open the KPMG code-signing conversation** | **2–4 days** | Highest saving-per-hour in the report; no new vendor; fits `OD-31`'s phase-2-first instruction exactly |
| 5 | **Phase T plan fix folds in NATS / centrifuge / River / Cedar / CloudEvents**, with `OD-16` written as NATS subject permissions | **1–2 days of plan edit** | Largest saving, and it is only available while the plan is still a document |
| 6 | **Phase W plan fix folds in the headless primitives**, plus the React Aria ADR and the filter grammar | **1–2 days of plan edit** | Same argument, second-largest |
| 7 | **Phase 5: answer D-OSS-1 / D-OSS-2 / D-OSS-3**, then resume. D-OSS-3 is a **2-day spike** — render three real-HTML frames as React Flow nodes and try one MCP-shaped mutation. If the frame model survives `nodes[]`, adopt; if we are fighting `edges`, write our own world layer on Moveable and keep the rest of the list | **2–3 days** | Unblocks the paused tasks |
| 8 | **Phase 8: spike `agent-sandbox` `examples/kata-aks/`** against a scratch cluster, behind the existing `katabe` interface, with a load driver we write | **2–3 days** | One spike decides 8–12 tasks. Needs cloud spend, so it is last |
| 9 | **Phase H: take the in-toto format decision** before task 02 | **Hours** | Cheap now, expensive after the golden vectors exist |

### Three half-day reads that change designs, not code

`03-agent-fleet.md` and `04-git-lanes.md` both argue the highest-value items are reads, not
adoptions. Do these before phase T's records and phase 3's convergence tasks are written:

1. **OpenHands' EventStream** (MIT) — the strongest published argument that team mode's task-card
   and directive records should be an **append-only typed stream**, not mutable rows.
2. **`google/git-appraise`** (Apache-2.0) — append-only JSON-lines notes with `cat_sort_uniq`, which
   makes metadata conflicts on `refs/panacea/lanes/…` a non-problem. Directly relevant to the
   provenance thread's git-side carriage.
3. **`jj` (Jujutsu)** (Apache-2.0) and **`git-town`** (MIT) — undo as a *recorded before-state per
   land id*, not as the inverse of a land; and a conflict as a **recorded object** the cockpit can
   show, route and defer rather than a broken worktree handed to the user. Shapes phase 3's
   "landed history with undo" and 3-D3.

### One non-adoption worth more than most adoptions

`04-git-lanes.md`'s highest-value finding is not a dependency at all: **close the two R4 gaps in
`control-plane/internal/project/gitrunner.go`** — `GIT_CONFIG_*` isolation and fsck-on-ingest —
and read Gitea's `modules/git` (MIT) for what else our runner is missing, **before** phase 3 widens
its call surface to `merge-tree`, `commit-tree`, `update-ref` and `bundle`. **1–2 days.**

---

## 6. The risks of adopting

Stated plainly, with what we do about each.

| Risk | What it looks like here | Mitigation |
|---|---|---|
| **A project relicenses after we ship** | This is not hypothetical. **HashiCorp went BUSL-1.1 in August 2023**; Meilisearch added a BUSL Enterprise Edition; Langfuse's copyright holder is now ClickHouse, Inc.; the MCP Go SDK is **mid-relicence right now**. `OD-11` already cost this program a withdrawn decision | **Pin by version and digest, and record the version boundary.** A relicence is not retroactive: the version we took under MIT stays MIT. The gate must run on the transitive tree at every build, so a licence change under a version bump fails the build rather than shipping |
| **A component dictates our data model** | TanStack DB deciding what an issue is; React Flow's `nodes[]`/`edges` deciding what a frame is; Yjs deciding what a design revision is; A2A's `Task` deciding what an Orchestrator turn is | **Every one of them goes behind our interface** — `replica/`, the frame model, the document store, the turn record. Where that is not possible (React Flow), the 2-day spike exists precisely to find out before we commit |
| **The upgrade treadmill** | Thirty new names across seven trees, each with its own cadence. `cmdk` has had **no release in 18 months** — that is the failure mode, not a hypothetical | **Prefer projects with a release in the last 12 months** and say so when there is not one. The tier-1 list is deliberately boring: xterm addons and Tauri plugins version with dependencies we already carry |
| **Dependency maintenance and the security review** | Five new names in phase T's review; two new runtime components in phase 8 (NATS; centrifuge is in-process); `ygo` at ~55 stars would be a thin dependency on phase 5's critical path | Gate `ygo` behind **our own byte-level conformance test** before committing. Keep NATS' subject permissions as the enforceable form of `OD-16`, so the new component earns its review |
| **Permissive core, non-permissive add-on** | The trap this survey hit five times: LiteLLM `enterprise/`, BlockNote `xl-*`, Liveblocks `liveblocks-server`, Langfuse `ee/`, dockview `dockview-enterprise`, Meilisearch EE | **Check the directory, not the badge.** The gate needs a per-path rule, not a per-repository one. gVisor proves the same point from the other side: tri-licensed, one repository |
| **Vendored code drifting from upstream** | Four vendored pieces: `cmdk`'s scoring is dropped, but Gitea's three files, `in-toto-golang`'s DSSE envelope and the overprovisioner pattern all get copied in | `THIRD-PARTY.md` with notices, and **fail the build if an MPL-2.0 file in `vendor/` differs from upstream** — that is the only way MPL's obligation bites us |
| **Apache-2.0 NOTICE obligations** | React Aria, `coreos/go-oidc`, Cedar, CycloneDX, in-toto and NATS all carry or reference a NOTICE | **The Hub's distribution needs a `NOTICE` file.** Small, and nobody has written it. Add the task |
| **Adoption breaking the thread** | The failure mode `CLAUDE.md` names directly. A component that cannot carry a provenance reference is a poor fit however good it is | The reason `addon-serialize` beats a screenshot, Playwright beats a browser-side capture, and in-toto beats a bespoke blob: **each one can carry a provenance id.** Make "can it carry the reference" the first question at every adoption |

---

## 7. What is unverified, and what to re-check at adoption time

- **Every task-count in §1 is my mapping of the researchers' day-estimates onto the plans' task
  tables.** The day figures are the design documents' own unmeasured estimates; the task mapping is
  mine and equally unmeasured. Nothing here is measured.
- **Licence facts are as at 2026-09-28** and must be re-verified when a version is actually pinned.
  A licence can change between a research pass and a `package.json` line.
- **Two researchers disagreed and this document rules on both.** Cedar: adopt in Phase T, not Phase
  H (§2 #20). The command palette and keyboard map: **not adopted** — the UI researcher read the
  tree and found them already built and under test, which beats the work-layer researcher's
  "vendor" verdict.
- **Two token stores is a bug that is currently in both plans**: Phase H task 28 builds `credstore`
  with `WinCred` in Go; `07-ui-primitives.md` proposes `keyring-rs` in Rust. One of them must go.
- **The corporate proxy blocked some fetches** during the research; each domain file records its own
  blocked list in its "what I could not verify" section. Nothing in this summary rests on a page a
  researcher could not read.

*Prepared 2026-09-28. Findings only — no source was changed, no test was run, nothing was started.*
