# 03 — Agent fleet and cockpit: what open source can carry

**Researched 2026-09-28.** Domain: Phase 2 (the cockpit, 24/26 tasks done), **Team mode**
(≈69 tasks, unbuilt — `planning/2026-09-24-team-collaboration/`), and the **cloud path**
(phase 8's Fleet, warm pools, `machined`).

**Binding constraints this research was judged against**

| | Rule | Source |
|---|---|---|
| L | MIT / Apache-2.0 / BSD / ISC / Unlicense / 0BSD / Zlib allowed. MPL-2.0 conditional. **AGPL, GPL, SSPL, BUSL, Elastic, Commons Clause, source-available: forbidden.** | task brief; OD-11 |
| A | **Nothing modifies, wraps or parses the agent CLI. Nothing derives meaning from PTY bytes.** A harness's own on-disk records may be read read-only through a version-pinned adapter with a canary test. | `OD-8`, `PROGRAM.md` cross-cutting |
| P | Provenance is the spine. A component that cannot carry a brief/spec reference through to the release is a poor fit. | `CLAUDE.md` |
| U | Usability is the wedge. "Powerful but takes a wall of configuration" fails the bar. | `CLAUDE.md` |
| V | `docs/DESIGN-SYSTEM.md` is binding. A library that drags in its own visual language is a cost. | `CLAUDE.md` |

**Constraint A is the single biggest filter in this domain, and it is not a licence
problem.** The best-funded, most-starred projects in "run many coding agents" — `coder/agentapi`,
`claude-squad`, several cockpit clones — work by driving the agent CLI inside a terminal emulator
and **reading the rendered screen** to decide what the agent is doing. That is precisely what OD-8
forbids. Several candidates below are rejected on A while being perfectly licensed.

---

## Method, and what "verified" means here

Every licence below was read from the repository's own `LICENSE`/`COPYING` file over
`raw.githubusercontent.com` on **2026-09-28**, or from the GitHub API's `license.spdx_id`
for the same repo on the same day, not from a badge or from memory. Where a project has a
**dual structure** (a permissive root plus a paid directory) the paid licence text was read too;
that trap caught one locked decision — see LiteLLM below. Stars and `pushed_at` are the GitHub
API values on 2026-09-28 and are recorded as maturity evidence only.

Two projects were **not** opened beyond their licence file, by rule L: `smtg-ai/claude-squad`
(AGPL-3.0) and `getAsterisk/claudia` (AGPL-3.0). Their names and licences are recorded in §6;
no source of theirs was read.

---

## 1. The four things worth acting on

**1. A licence defect in a locked decision.** `OD-21` locks **LiteLLM** for phase 6. LiteLLM's
root is MIT, but its `enterprise/` directory is a **paid, non-distributable** licence — and the
project's own `Dockerfile` **copies `enterprise/` into the shipped runtime image** (lines 75, 150,
with a comment at 147–149: *"enterprise/ is imported by source path at runtime"*). PanaceaApp ships
to **customers' own clouds**. Publishing the stock `berriai/litellm` image into a customer tenancy
distributes code whose licence says *"it is forbidden to copy, merge, publish, distribute,
sublicense, and/or sell the Software."* **OD-21 survives; the build does not.** Fix is small and
must happen before phase 6 writes a Helm chart. §3.1.

**2. Three team-mode subsystems are buyable, not buildable.** The cross-Orchestrator typed bus,
presence for the team strip, and durable Orchestrator turns are each a multi-week hand-build in the
TM1 estimate, and each has a boring, permissive, decade-old answer: **NATS JetStream** (Apache-2.0),
**centrifuge** (MIT, a Go *library*, not a server), and **River** (MPL-2.0) or **Temporal** (MIT).
Together they are the largest honest saving in this domain — my estimate is **18–26 of TM1's ≈95
focused days**, and they remove three bespoke protocols we would otherwise own forever.

**3. The most attractive-looking projects in this space are barred by `OD-8`, not by licence.**
`coder/agentapi` (MIT, well made, Coder Technologies) drives Claude Code inside a terminal emulator
and **reads the rendered screen** to synthesise a chat API. `SWE-ReX` (MIT) extracts command output
and exit codes by sentinel-matching PTY bytes. Both are exactly the class of design phase 1 and
`OD-8` ruled out. They are **study-only at best**, and the reason must be written down, because
their READMEs make them look like a shortcut.

**4. Phase 2's last tasks have a €0 answer.** Six `@xterm/*` addons — all MIT, all from the
xterm.js monorepo already in `desktop/package.json` — cover WebGL rendering for many concurrent
terminals (the design system's "one global heartbeat clock" scale problem), scrollback
serialisation for reattach, search, Unicode 11, clipboard and link handling. About a day of work,
no new vendor.

---

## 2. Candidates

Verdicts: **adopt** = take as a dependency · **vendor** = copy a permissive piece in with
attribution · **study-only** = read the approach, write our own · **reject**.

| Project | Licence (read 2026-09-28) | Lang | Maturity (2026-09-28) | Replaces | Verdict |
|---|---|---|---|---|---|
| **NATS / JetStream** `nats-io/nats-server` | Apache-2.0 | Go | 20,778★, pushed 2026-09-25 | Team mode: the typed cross-Orchestrator bus, durable inboxes, KV presence, file leases | **adopt** |
| **centrifuge** `centrifugal/centrifuge` | MIT | Go (library) | 1,479★, pushed 2026-09-27 | Team mode C2: the team strip's presence, join/leave, history-on-reconnect | **adopt** |
| **River** `riverqueue/river` | **MPL-2.0** (conditional) | Go | 5,717★, pushed 2026-09-28 | Team mode: durable Orchestrator turns on the Hub's Postgres | **adopt (conditional)** |
| **Temporal** `temporalio/temporal` | MIT | Go | 23,319★, pushed 2026-09-27 | Same as River, at 10× the operational surface | **study-only** |
| **`@xterm/addon-{webgl,serialize,search,unicode11,clipboard,web-links}`** | MIT (each, npm `latest` metadata) | TS | part of xterm.js, 21,227★ | Phase 2 residue: renderer scale, reattach scrollback, find, wide chars | **adopt** |
| **asciinema-player** `asciinema/asciinema-player` | **Apache-2.0** (relicensed; see §3.5) | JS/Rust | 2,925★, pushed 2026-09-13 | Team mode + phase 9: session replay in the provenance ledger | **adopt (player only)** |
| **CloudEvents** `cloudevents/spec` + `sdk-go` | Apache-2.0 both | spec + Go | sdk-go 963★, pushed 2026-09-26 | Team mode `directive`/escalation envelope; ledger event shape | **adopt** |
| **Cedar** `cedar-policy/cedar-go` | Apache-2.0 | Go | 235★, pushed 2026-07-15 | `OD-16` "no user steers another user's agent"; the Hub's role-to-verb table | **adopt** |
| **A2A** `a2aproject/A2A`, `a2a-go` | Apache-2.0 | spec + Go | a2a-go 474★, pushed 2026-09-25 | The *shape* of Orchestrator↔Orchestrator messages | **vendor (the schema, not the runtime)** |
| **ACP** `agentclientprotocol/agent-client-protocol` | Apache-2.0 | spec | pushed 2026 | Nothing today; the escape hatch if a harness ever speaks it natively | **study-only** |
| **OpenFGA / SpiceDB / OPA** | Apache-2.0 each | Go | 5,875★ / 7,100★ / 12,280★ | Alternatives to Cedar; all are servers | **study-only** |
| **OpenHands** `OpenHands/OpenHands` | MIT | Python | 89,312★, pushed 2026-09-27 | Nothing directly. Study its EventStream + Runtime split | **study-only** |
| **Goose** `block/goose` | Apache-2.0 | Rust | pushed 2026 | Study `goosed` + its per-tool allow/ask/deny model for `OD-23` | **study-only** |
| **Cline** `cline/cline` | Apache-2.0 | TS | 69,448★, pushed 2026-09-27 | Study the auto-approve matrix — the best consent UI in the space | **study-only** |
| **OpenCode** `sst/opencode` | MIT | Go/TS | pushed 2026 (`opencode-ai/opencode` is **archived**) | Study the closest sibling: Go server, documented session API, thin clients | **study-only** |
| **Continue** `continuedev/continue` | Apache-2.0 | TS | 36,046★, pushed 2026-09-27 | Study config + rules model | **study-only** |
| **SWE-ReX** `SWE-agent/SWE-ReX` | MIT | Python | part of SWE-agent, 20,429★ | Multi-backend session abstraction — but it sentinel-parses PTY bytes | **study-only** |
| **humanlayer** `humanlayer/humanlayer` | Apache-2.0 per `LICENSE`; **GitHub reports `NOASSERTION`** | Go/TS | 11,614★, pushed **2026-06-19** (stale) | Study the approve/escalate channel — closest prior art to `OD-16` inform/decide | **study-only** |
| **vibe-kanban** `BloopAI/vibe-kanban` | Apache-2.0 | Rust/TS | 28,207★, pushed 2026-09-19, **company shut down 2026-04, sunsetting** | Study the workspace-per-task model | **study-only** |
| **Crystal** `stravu/crystal` | MIT | TS | 3,121★, pushed **2026-02-26** (7 months stale) | Study many-sessions-one-repo UX | **study-only** |
| **e2b `infra`** `e2b-dev/infra` | Apache-2.0 | Go | production, pushed 2026-09 | Phase 8 warm pools: read their snapshot/resume + pool sizing | **study-only** |
| **microsandbox** | Apache-2.0 | Rust/Go | active | Same; microVM-per-session without Kubernetes | **study-only** |
| **Kata, Firecracker, gVisor, Sysbox, OpenKruise, Podman** | Apache-2.0 each | — | all active | Already locked (Kata) or alternatives to it | **adopted already / study-only** |
| **ttyd / gotty / wetty** | MIT each | C / Go / TS | 12,431★ / 2,549★ / 5,449★ | Nothing — phase 2 already has the relay | **study-only (gotty's reconnect ring buffer)** |
| **tmux** | **ISC** (allowed) | C | canonical | Session survival across relay restarts | **reject** — see §6.2 |
| **coder/agentapi** | MIT | Go | active | Would give a chat API over any agent CLI | **reject** — violates `OD-8` |
| **charmbracelet/crush** | **FSL-1.1-MIT** — source-available, competitor restriction | Go | active | — | **reject on licence** |
| **Anthropic Agent SDK** (`claude-agent-sdk-*`) | **Not open source**: "© Anthropic PBC. All rights reserved", Commercial ToS | TS/Py | — | — | **reject as an OSS dependency** |
| **Daytona** `daytonaio/daytona` | licence not resolvable at `main`; repo says **unmaintained since 2026-06**, core moved private | Go | 71,700★ but abandoned | — | **reject** |

---

## 3. Adopt — the argument for each

### 3.1 LiteLLM: `OD-21` stands, but the shipped image must be ours

Not a candidate — a **correction to a locked decision**, and the most load-bearing item here.

- Root licence: **MIT**, read at `https://raw.githubusercontent.com/BerriAI/litellm/main/LICENSE`
  (2026-09-28). Its own text carves out `enterprise/`.
- `enterprise/LICENSE.md`, read the same day: the **BerriAI Enterprise License**. *"may only be used
  in production, if you … have agreed to … the BerriAI Subscription Terms of Service … and have a
  valid BerriAI Enterprise license for the correct number of user seats"*, and *"it is forbidden to
  copy, merge, publish, distribute, sublicense, and/or sell the Software."*
- The repo's `Dockerfile` copies it into the runtime: `COPY enterprise/pyproject.toml enterprise/`
  (L75) and `COPY --from=builder /app/enterprise /app/enterprise` (L150), under the comment
  *"enterprise/ is imported by source path at runtime"* (L147–149).

**Why it bites us specifically.** `CLAUDE.md` says customer-hosted Azure Kubernetes tiers come
later; the moment one exists, we hand a customer an image. If that image is the stock LiteLLM one,
we are distributing BerriAI's proprietary directory. A we-host-only posture reduces but does not
remove the problem, because "used in production" is the Enterprise License's own trigger.

**What is actually gated, and whether we care.** The MIT core carries everything phase 6's design
needs: **virtual keys** (the scoped per-session token of `OD-19`), **per-key/user/team budgets and
spend tracking**, routing and fallbacks, RPM/TPM limits, and OTel/Langfuse logging. Enterprise-gated
are SSO on their Admin UI beyond 5 users, OIDC/JWT auth, SCIM, **audit logs**, organisations, key
rotation and secret-manager integrations. Two of those look alarming and are not:

- **Audit logs** must live in the Hub's provenance ledger anyway (rule P) — a gateway-local audit
  log that the ledger cannot thread is the wrong artefact regardless of price.
- **SSO / their Admin UI** we never expose. The Hub is the only user-facing surface; the gateway is
  an internal component behind Keycloak. Do not ship their dashboard.

**Do this, before phase 6 writes a chart (about half a day):**
1. Build our own image from the MIT tree with `enterprise/` and `ui/litellm-dashboard/` deleted,
   and confirm the proxy boots without them.
2. Add a CI licence gate that fails if `enterprise/` appears in any image layer or any vendored
   path — the same gate `OD-11` mandates for phase 5, extended to server images.
3. Record the residual risk honestly: GitHub issue
   [`BerriAI/litellm#34241`](https://github.com/BerriAI/litellm/issues/34241) argues the MIT /
   Enterprise boundary *does not hold in practice*, because some Enterprise-documented features have
   their gating logic in MIT-licensed files. That is a question for legal, not for us, and it is a
   reason to keep the gateway behind a thin interface of our own so a swap stays cheap.

### 3.2 NATS JetStream — the cross-Orchestrator bus, presence store and lease manager

`nats-io/nats-server`, **Apache-2.0** (`LICENSE`, read 2026-09-28), 20,778★, pushed 2026-09-25.
Client `nats-io/nats.go` Apache-2.0, `nats-io/nkeys` Apache-2.0.

**What it replaces.** Team mode's design (`05-design.md` §4, §6) needs, in our own words: typed
messages between Orchestrators that survive a killed Orchestrator; a per-user durable inbox; a
record of who caused each directive; presence ("who is working"); and **file leases** so two agents
do not claim the same path. Written by hand that is a message schema, a Postgres outbox, a retry
and dedup policy, a heartbeat/expiry scheme and a lock table with fencing tokens.

JetStream gives all five as configuration:

- **Durable, replayable, per-subject streams** with at-least-once delivery and explicit ack — the
  Orchestrator inbox, and the *"every instruction to an agent is recorded, with who caused it"*
  requirement (`00-summary.md` §5) falls out of stream retention rather than a second table.
- **Subject-level authorisation per account/user.** `OD-16`'s *"Orchestrators are the ONLY cross-user
  channel"* and *"no user steers another user's agent"* become subject permissions on a credential,
  enforced by the broker, not by our code remembering to check. That is a materially stronger
  security story than an application-level check, and it is the kind of thing a KPMG security review
  will ask for.
- **KV buckets with per-key TTL, compare-and-swap on revision, and watchers**
  (docs.nats.io, JetStream KV, read 2026-09-28). A file lease is `Create`-or-fail on
  `lease.<repo>.<path>` with a TTL; expiry is the broker's job, not a sweeper of ours. A watcher
  gives the team strip live lease changes without polling.
- **Request/reply with timeouts** for the Orchestrator-to-Orchestrator negotiation of `OD-16`.

**Integration cost: low-to-moderate, and it fits the locked architecture.** Go-native, one binary,
runs as a StatefulSet on the same AKS as Fleet, embeddable in-process (`nats-server` is importable)
for local mode so `OD-20`'s laptop path needs no extra service. Auth is NKey/JWT, which sits beside
Keycloak rather than fighting it — the Hub mints per-session NATS credentials the same way it mints
gateway tokens. No visual language, no bundle size.

**Risks, honestly.** (a) It is a new operational component in phase 8 — replication, storage sizing,
backup. (b) JetStream's durability guarantees depend on a correctly sized R3 cluster; a single
replica is a silent single point of loss. (c) There is a temptation to move the provenance ledger
onto it; **do not** — the ledger is Postgres in the Hub, and NATS is transport plus ephemeral state.
Keeping that line is what preserves rule P.

**Verdict: adopt.** This is the single largest defensible saving in team mode.

### 3.3 centrifuge (the Go library, not Centrifugo the server) — the team strip

`centrifugal/centrifuge`, **MIT** (`LICENSE`, "Copyright (c) 2023 Centrifugal Labs LTD", read
2026-09-28), 1,479★, pushed 2026-09-27. The standalone server `centrifugal/centrifugo` is
Apache-2.0, 10,807★ — also allowed, but the library is the better fit.

**What it replaces.** `OD-15`/C2: *"The user can see who else is working on the repo, and a general
summary of what they are doing, files they are touching … in a simple and meaningful way to not
clutter the dashboard."* Built by hand that is a WebSocket hub, per-channel subscription with
authorisation, **presence with join/leave**, **history with recovery on reconnect** (so a laptop that
sleeps does not show a stale strip), and client-side reconnect with backoff.

centrifuge ships exactly that set as a library you embed in an existing Go server — presence,
presence stats, join/leave, channel history, automatic message recovery on reconnect, and a
JS/TS client (`centrifuge-js`, MIT) that handles the reconnect logic the cockpit would otherwise
own. It has a Redis engine for multi-node, and an in-memory engine for one node and for local mode.

**Why the library and not Centrifugo the server:** rule U. Centrifugo is a separate service with its
own config file, its own admin UI and its own token model — a wall of configuration for a feature
that is one strip of a cockpit. The library is `import` plus a handler, authorised by our existing
session, with no second identity system.

**Risks.** 1,479★ is modest, but it is the engine under Centrifugo's 10,807★ and has ten years of
history; single-vendor (Centrifugal Labs) is the real risk, mitigated by MIT and by the fact that
the surface we use — subscribe, publish, presence — is small enough to reimplement if it died.
Its Redis engine would add Redis to phase 8; with NATS already there (§3.2), prefer one node plus
the memory engine until the team strip actually needs to scale out, and say so in the plan rather
than adding Redis on spec.

**Verdict: adopt**, library form, for C2 only. Do not let it become the transport for agent
directives — that is NATS, where the authorisation model is stronger.

### 3.4 River — durable Orchestrator turns on the Hub's Postgres

`riverqueue/river`, **MPL-2.0** (`LICENSE`, read 2026-09-28), 5,717★, pushed 2026-09-28.

**What it replaces.** TM1's done-criterion includes *"a killed Orchestrator loses nothing"*
(`00-summary.md`). That is a durable-execution problem: an Orchestrator turn must be retryable,
idempotent, and observable, and its state must survive the pod. River is a Postgres-backed job
queue for Go with transactional enqueue (the job commits in the same transaction as the Hub record
that caused it — which is what makes "no lost directive" true rather than hoped for), retries with
backoff, unique jobs, periodic jobs and a maintenance/rescue path for stuck work.

**Why River over Temporal.** Temporal (MIT, 23,319★) is the stronger durable-execution engine and
would also model the Orchestrator's multi-step negotiation as workflow code. It is also a cluster of
four services plus its own datastore, its own CLI, its own UI and a programming model the team must
learn. Rule U: *"powerful but takes a wall of configuration"*. The Hub already runs Postgres; River
is a library in the same process and a migration. Take Temporal only if the Orchestrator's
negotiation turns out to need long-running, signal-driven workflows that span days — record that as
the trigger to revisit, not as today's decision.

**Licence obligation — state it in the plan.** MPL-2.0 is **file-level copyleft**: if we modify a
River source file and distribute it, that file's source must be offered under MPL-2.0. Used as an
unmodified `go.mod` dependency it imposes nothing beyond notice. **Rule: no forks, no patched
vendoring; if we need a change, upstream it or wrap it.** That is a cheap rule to hold and it keeps
the obligation at zero.

**Second trap, recorded.** **River Pro** — workflows, sequences, concurrency control — is a
**separate paid commercial module** distributed as a private Go module under its own EULA. It is not
in the MPL-2.0 repository, so there is no contamination, but a developer reaching for "workflows"
will find a paid product. The OSS package covers our need: a per-Orchestrator queue with one worker
gives the one-at-a-time ordering without Pro's sequences.

**Verdict: adopt, conditional** on the no-fork rule and on the plan naming MPL-2.0 explicitly.

### 3.5 asciinema-player — session replay, with the recorder written by us

`asciinema/asciinema-player`, **Apache-2.0**, 2,925★, pushed 2026-09-13. npm `asciinema-player`
`latest` is **3.17.0, `"license": "Apache-2.0"`**, and I checked 2.6.1, 3.0.0, 3.6.3 and 3.10.0 —
**Apache-2.0 at every one**, so there is no relicensing boundary on the player. Its whole runtime
dependency tree is three packages: `solid-js`, `@babel/runtime`, `solid-transition-group`, all MIT.

**The split that matters, and it is a licence split.** The asciinema project is three repositories
under three different licences:

| Repo | Licence (read 2026-09-28) | For us |
|---|---|---|
| `asciinema/asciinema-player` | **Apache-2.0** | **adopt** |
| `asciinema/asciinema` (the recorder CLI) | **GPL-3.0** | **do not use, do not read** |
| `asciinema/asciinema-server` | Apache-2.0 at `main` | not needed; see §7 |

**We do not need the recorder, and that is the point.** Phase 2 already owns the PTY relay
end to end. Writing **asciicast** frames — a JSON header then `[time, "o", data]` lines — from a
relay we already own is a day's work and touches no GPL code. What we would otherwise build by hand
is the *player*: ANSI-correct rendering of a recording, seeking, speed control, idle-time capping,
poster frames, and a live-stream mode. That is months of terminal-emulator correctness, and the
player is the same emulator family the cockpit already trusts.

**Why this earns its place against rule P.** A replay that the provenance ledger can link is the
missing evidence in *"attached to the release in Deploy"*: a release cites a brief, the brief cites
the sessions, and a session cites a recording that can be played back in the cockpit. `OD-8` is
untroubled — replay **renders** bytes, it does not derive meaning from them.

**Risks.** (a) It is Solid.js inside a React app — it mounts into a DOM node and does not fight
React, but it is a second framework in the bundle (~100 KB class, not measured here; measure before
committing). (b) Rule V: it arrives with its own control bar and themes. It is styled by CSS custom
properties and drivable through its JS API, so the mitigation is to use the API and supply our own
controls, and to budget half a day for that rather than shipping its default chrome into a design
system that forbids machine-generated looks.

**Verdict: adopt the player. Write the recorder. Never vendor the GPL-3.0 CLI.**

### 3.6 The six xterm.js addons — phase 2's cheapest remaining win

All MIT, all read from npm `latest` metadata on 2026-09-28, all from the xterm.js monorepo
(21,227★, pushed 2026-09-13) already in `desktop/package.json` (`@xterm/xterm` ^6.0.0,
`@xterm/addon-fit` ^0.11.0):

| Addon | Version | What it buys |
|---|---|---|
| `@xterm/addon-webgl` | 0.19.0 | GPU rendering. The design system's *"many concurrent live processes"* is a renderer problem before it is a UI problem; the canvas/DOM renderer is what falls over at a wall of terminals |
| `@xterm/addon-serialize` | 0.14.0 | Serialise a live buffer to a string. This is the honest answer to reattach: snapshot the buffer server-side or client-side and restore in one write, instead of replaying a scrollback and triggering the scroll-jump the design system forbids |
| `@xterm/addon-search` | 0.16.0 | Find in a session, with decorations |
| `@xterm/addon-unicode11` | 0.9.0 | Correct wide-character widths — agent CLIs emit box drawing and emoji, and wrong widths corrupt the frame |
| `@xterm/addon-clipboard` | 0.2.0 | OSC 52, so a copy inside the sandbox reaches the host clipboard |
| `@xterm/addon-web-links` | 0.12.0 | Clickable URLs, which every agent prints |

**Integration cost: about a day, most of it deciding the WebGL fallback path** (WebGL context loss
is real under a remote desktop; keep the DOM renderer as fallback). **Risk: none material.** Same
vendor, same licence, same release train as a dependency we already ship.

**Verdict: adopt.** This is the cheapest item in the document.

### 3.7 CloudEvents — the envelope for typed cross-owner messages and ledger records

`cloudevents/spec` **Apache-2.0**, `cloudevents/sdk-go` **Apache-2.0** (963★, pushed 2026-09-26).

**What it replaces.** Team mode invents several record types — `directive`, Orchestrator-turn,
human-message, escalation (`00-summary.md` §11) — and the ledger needs an event shape that is stable
across Hub, Fleet, the desktop and, later, Panacea's deploy side. Hand-rolling that is not hard; it
is *permanent*, and every new producer re-litigates field names.

CloudEvents is a CNCF specification with exactly the fields this design needs: `source` (which
Orchestrator), `subject` (which project/lane/session), `type` (the typed message name), `id`,
`time`, and **extension attributes** — which is where the **brief/spec id rides**. That last point is
rule P in one line: a provenance reference becomes a first-class, schema-checked attribute rather
than a convention someone forgets.

**Integration cost: low, and it is mostly discipline.** The SDK is optional — the spec's JSON
binding is small enough to implement — but the SDK gives NATS, HTTP and Kafka bindings for free,
which pairs with §3.2.

**Risk.** Over-adoption: CloudEvents' full SDK carries transport abstractions we do not want.
Take the **event type and the JSON binding**; keep our own transport code.

**Verdict: adopt** the spec; adopt `sdk-go` only for its event type and JSON codec.

### 3.8 Cedar (`cedar-go`) — "no user steers another user's agent", enforced

`cedar-policy/cedar` **Apache-2.0** (1,751★, pushed 2026-09-25); `cedar-policy/cedar-go`
**Apache-2.0** (235★, pushed 2026-07-15).

**What it replaces.** `OD-16` is an authorisation statement: Orchestrators are the only cross-user
channel; no user steers another user's agent; escalation defaults may tighten and never loosen
(`OD-24`). Team mode also adds *"a role-to-verb table"* and *"leaver checks"* to the Hub
(`00-summary.md` §11). Hand-built, that is an `if` ladder spread across handlers — the failure mode
the security critique (`07-critique-security.md`) exists to catch.

Cedar is a policy language with a **formally verified** evaluation core, designed for exactly this
shape: principal, action, resource, context. Policies are data, so the role-to-verb table becomes a
file under review rather than code, and *"tighten, never loosen"* becomes a `forbid` policy that no
later `permit` can override — Cedar's forbid-wins semantics give that for free, which is a real
property and not a slogan.

**Why Cedar over OpenFGA / SpiceDB / OPA** (all Apache-2.0, all credible): those three are
**servers**. Cedar-go is a **library** — no new deployment, no second datastore, no network hop in
the authorisation path. Rule U again. Choose SpiceDB or OpenFGA only if we later need
relationship-based authorisation across a large graph (who-can-see-whose-lane at org scale); that is
a phase 9 question, not a TM1 one.

**Risks.** `cedar-go` is 235★ and pushed 2026-07-15 — two and a half months stale at the date of
writing, and much younger than the Rust core. It is maintained by AWS, which is both the reassurance
and the lock-in. Mitigation: the policy *language* is the durable asset; a Go evaluator can be
replaced.

**Verdict: adopt** for the Hub's authorisation decisions, and write the `OD-16` and `OD-24` rules as
Cedar policies in the plan, so the critique can read them.

---

## 4. Vendor — take the schema, leave the runtime

### 4.1 A2A (Agent2Agent) — the shape of an Orchestrator-to-Orchestrator message

`a2aproject/A2A` **Apache-2.0**; `a2aproject/a2a-go` **Apache-2.0** (474★, pushed 2026-09-25).
Linux Foundation project, originally Google.

`OD-16` invents a peer protocol between Orchestrators: negotiate, resolve what you can, escalate the
rest to your human *to inform* or *to ask for a decision*. A2A has already solved the vocabulary for
that, and the fit is closer than it looks. Read at `a2a-protocol.org/latest/specification/`
(2026-09-28), its `TaskState` enum is:

`TASK_STATE_SUBMITTED` · `WORKING` · `COMPLETED` · `FAILED` · `CANCELED` · **`INPUT_REQUIRED`** ·
`REJECTED` · **`AUTH_REQUIRED`** · `UNSPECIFIED`

**`INPUT_REQUIRED` is `OD-16`'s "ask for a decision", already named and already modelled as a task
state rather than as a side-channel.** Its `Task` / `Message` / `Part` / `Artifact` split maps onto a
task card with observed files and leases; `AgentCard` maps onto "what this Orchestrator can be asked
to do".

**Why vendor the schema and not adopt the runtime.** A2A's transport assumes agents are network
services discovered by an `AgentCard` over HTTP — a federation model we do not have and do not want
inside one tenancy, where the platform brokers everything (`OD-14`) and NATS subjects are the
authorisation boundary (§3.2). Adopting `a2a-go` would import that shape and fight the locked
architecture. Taking the **type names and state machine** costs nothing, makes the design legible to
an outside reviewer, and leaves a door open if a customer ever asks us to speak A2A at the edge.

**Verdict: vendor** — the type vocabulary and `TaskState` machine, with an Apache-2.0 attribution
note in the package. **Do not** take the transport.

### 4.2 gotty's reconnect ring buffer — read it, take the idea, it is 200 lines

`sorenisanerd/gotty` **MIT**, 2,549★, pushed 2026-08-05 (the maintained fork; the original
`yudai/gotty` is dead). MIT means we *may* copy with attribution, and its server-side output ring
plus client reconnect handshake is the piece phase 2's attach path re-solves. Honestly: our relay
already exists and `@xterm/addon-serialize` (§3.6) is a better answer to the same problem. Vendor
only if the serialize approach fails a real reattach test.

---

## 5. Study-only — where the value is the design, not the code

### 5.1 The harnesses, for their session, workspace and permission models

None of these replaces Claude Code (`OD-8`), and none should become a dependency — they are Python,
Rust and TypeScript against a Go control plane. What they carry is **prior art that a critique can
be measured against**, and all four are permissively licensed so reading them is safe.

- **OpenHands** (MIT, 89,312★, pushed 2026-09-27; `openhands-aci` MIT). Read its **EventStream** —
  every agent step is a typed `Action` and its `Observation`, persisted, replayable, and the only
  thing the UI reads. That is *structurally* what team mode's task card and `directive` records are
  reaching for, and it is the strongest argument in this document that our records should be an
  append-only typed stream rather than mutable rows. Read also its `Runtime` split (the agent talks
  to a runtime client over an API; the sandbox is swappable) — the same shape as `PROGRAM.md`'s
  `Backend` interface, independently arrived at, which is reassurance.
- **Cline** (Apache-2.0, 69,448★, pushed 2026-09-27). Read its **auto-approve matrix**: per-tool
  allow/ask, with scopes (read in workspace, write in workspace, execute, browser, MCP) and a
  max-requests ceiling. `OD-23` says *"build the mechanism, not the policy"*; this is the clearest
  published mechanism of that exact shape, and it is a UI as much as a model — which matters for
  rule U.
- **Goose** (Apache-2.0, `LICENSE` read 2026-09-28). Read `goosed` — a daemon that owns sessions
  while thin clients attach — and its extension/permission model. Closest to our
  control-plane-owns-the-session posture.
- **OpenCode** (`sst/opencode`, **MIT**, `LICENSE` at `dev` read 2026-09-28). The closest
  architectural sibling in the whole survey: a **Go server** with a documented HTTP/SSE session API
  and thin clients (TUI, web, editor). Note the trap: `opencode-ai/opencode` (13,775★) is
  **archived**; the live repo is `sst/opencode`. MIT, so a specific piece could be vendored if one
  were worth it — I did not find one that beats what phase 2 already has.
- **Continue** (Apache-2.0, 36,046★). Read its config/rules model for how policy reaches a session.
- **SWE-ReX** (MIT, part of SWE-agent, 20,429★). Read its **multi-backend session abstraction** —
  one agent-facing API over local, Docker, Modal, Fargate — which validates the `Backend` interface.
  **But note the disqualifier**: its README states it *"will recognize when commands are finished,
  extract the output and exit code"* — i.e. it sentinel-parses PTY bytes. Our backends use real exec
  with real exit codes. Read the abstraction; reject the mechanism.
- **humanlayer** (Apache-2.0 per its `LICENSE` file; see §7 for a caveat; 11,614★, pushed
  **2026-06-19**). The closest prior art to `OD-16`'s inform/decide escalation: an agent requests
  approval, a human answers over a channel, the request is typed and durable. Worth a half-day read
  before team mode's escalation records are designed.

### 5.2 Multi-agent cockpits — read the UX, not the code

- **vibe-kanban** (Apache-2.0, 28,207★, pushed 2026-09-19). Ten agents, a workspace per task with
  *"a branch, a terminal, and a dev server"* — the same product shape as phase 2/3. **Its README says
  the project is sunsetting; Bloop shut down in 2026-04** and the cloud features were removed.
  Read the workspace-per-task UX and the diff-review-with-inline-comments flow; depend on nothing.
- **Crystal** (MIT, 3,121★, pushed **2026-02-26**, seven months stale). Many Claude Code sessions
  against one repo, with worktrees. Read the session-list UX.
- **ttyd** (MIT, C, 12,431★) and **wetty** (MIT, 5,449★). Nothing to take — phase 2's relay exists —
  but ttyd is the reference implementation for terminal-over-WebSocket correctness if a fidelity bug
  appears.

### 5.3 The cloud path — warm pools and microVM brokers

`OPEN RISK 1` in `PROGRAM.md` is *"Kata cold start is seconds; warm pools are mandatory and their
sizing is unproven at our concurrency."* Two permissive projects have solved a near-identical
problem in public and are worth reading before phase 8 writes its pool controller:

- **e2b `infra`** (Apache-2.0, read 2026-09-28) — a production Firecracker orchestrator with
  **snapshot and resume**, giving sub-second sandbox starts from a warm template rather than a warm
  *pool*. That is a genuinely different answer to the cold-start problem and worth understanding
  before we commit to pool sizing. **Do not adopt**: it is Firecracker + its own scheduler, not
  AKS + Kata, and adopting it would unpick `OD-18` and phase 8.
- **microsandbox** (Apache-2.0) — microVM-per-session without Kubernetes. Same reading, smaller.
- **OpenKruise** (Apache-2.0) — for pod pre-heating patterns on Kubernetes specifically.
- **Kata** (Apache-2.0), **Firecracker** (Apache-2.0), **gVisor** (Apache-2.0), **Sysbox**
  (Apache-2.0), **Podman** (Apache-2.0) — all permissive; Kata is already the locked choice.

### 5.4 Observing files touched from outside the sandbox

Team mode requires *"files touched are observed by the platform from outside the sandbox"*
(`00-summary.md` §4) — deliberately, so a sandboxed agent cannot lie about its own activity.
**Falco** (Apache-2.0) and **Tetragon** (Apache-2.0 Go, with `bpf/` dual GPL-2.0 / BSD-2-Clause —
verified: `bpf/LICENSE.BSD-2-Clause` exists, the repository root has no BSD-2 file, and the README
carries Apache, BSD and GPL badges) both do host-side syscall observation.

**My recommendation is to read them and build nothing from them**, for a reason that is
architectural rather than legal: under Kata each pod is a **microVM with its own kernel**, so a
host-side eBPF probe does not see guest file syscalls without work that `PROGRAM.md` already flags
as a compliance gap (*"Defender for Containers cannot assess Kata pods"*). The cheap, honest and
lie-proof source of "files touched" is the **lane clone's own git state**, which the platform reads
from outside the sandbox and which phase 3 already produces. Use git; keep eBPF out of phase 8.

**Note the GPL-2.0 trap for the record**: eBPF programs that use certain kernel helpers must declare
a GPL-compatible licence in the program itself. Any future eBPF work is a licence conversation, not
a plumbing task.

---

## 6. Reject — and why

### 6.1 Rejected on `OD-8`, not on licence

| Project | Licence | Why rejected |
|---|---|---|
| **`coder/agentapi`** | **MIT** (`LICENSE`, "Copyright (c) 2025 Coder Technologies, Inc.", read 2026-09-28) | It runs Claude Code, Goose, Aider and Codex **inside a terminal emulator and reads the rendered screen** to synthesise a chat API and detect turn boundaries. This is the exact design `OD-8` forbids and phase 1 was built to avoid. Its licence is perfect and its engineering is good; **the constraint is what kills it**, and that has to be written down, because it is the most obvious-looking shortcut in the domain. |
| **`SWE-ReX`** as a dependency | MIT | Sentinel-parses PTY output for command completion and exit codes (§5.1). Study the abstraction, reject the mechanism. |
| **`tmux`** inside the sandbox | **ISC — allowed** | Would give session survival across a relay restart for free. **Reject anyway**: it inserts an ANSI-rewriting multiplexer between the unmodified CLI and xterm.js, which re-opens the exact fidelity risk phase 1 was built to retire (resize, permission prompts, alt-screen). Our `sandboxd` already owns the PTY and can hold the buffer. Revisit only with a measured fidelity test. |
| **Anthropic Agent SDK** (`claude-agent-sdk-typescript` / `-python`) | **Not open source.** `LICENSE.md` reads *"© Anthropic PBC. All rights reserved. Use is subject to Anthropic's Commercial Terms of Service"* (read 2026-09-28) | Recorded because it is routinely described as if it were OSS. It is not, and no OSS-adoption plan may treat it as a permissive dependency. `OD-8` has us running the unmodified CLI anyway, so nothing is lost — but the licence status should be in writing. |

### 6.2 Rejected on licence

| Project | Licence (read 2026-09-28) | Note |
|---|---|---|
| **`charmbracelet/crush`** | **FSL-1.1-MIT** (`LICENSE.md`: *"Functional Source License, Version 1.1, MIT Future License"*) | Source-available with a **competitor restriction**; converts to MIT two years after each release. Forbidden by rule L today. A *specific old release* whose two years have elapsed would be MIT — do not build on that; it is a trap for a future engineer. |
| **`daytonaio/daytona`** | Not resolvable: no `LICENSE` at `main`; README points at a `v0.190.0` licence and states the repo is **unmaintained since 2026-06** with core development moved to a **private** codebase | Reject on abandonment regardless of licence. 71,700★ is not maturity here. |

### 6.3 Copyleft register — found, named, not read

By rule L these were identified and their licence file read **only to confirm the licence**. No
source, architecture document or issue of theirs was opened, and none may be.

| Project | Licence | What it is, and why it was relevant |
|---|---|---|
| `coder/coder` | **AGPL-3.0** | Self-hosted cloud development workspaces on Kubernetes — the closest commercial analogue to Fleet's workspace broker |
| `gitpod-io/gitpod` | **AGPL-3.0** | Ephemeral cloud dev environments; prior art for warm pools and workspace lifecycle |
| `gravitational/teleport` | **AGPL-3.0** | Session recording and replay with RBAC for SSH/Kubernetes — the nearest thing to team mode's audit surface |
| `smtg-ai/claude-squad` | **AGPL-3.0** (`LICENSE.md`) | Terminal manager for multiple Claude Code / Codex / Aider sessions with worktrees — the closest OSS analogue to phase 2 |
| `getAsterisk/claudia` | **AGPL-3.0** | A desktop GUI for Claude Code — the closest OSS analogue to the cockpit |
| `asciinema/asciinema` (CLI) | **GPL-3.0** | The asciicast recorder. **We write our own recorder**; only the Apache-2.0 player is adopted (§3.5) |
| `paradoxxxzero/butterfly` | **GPL-3.0** | Web terminal, named in the brief. Dead since 2017 in any case |
| `loft-sh/devpod` | **MPL-2.0** (conditional, not forbidden) | Dev-environment provisioner. Not recommended: pushed **2025-11-14**, ten months stale |
| `hashicorp/yamux` | **MPL-2.0** (conditional) | Go stream multiplexing. Usable if ever needed; no fork, same rule as River |

`tldraw` remains excluded by `OD-11` (commercial licence) and is out of this domain anyway.

---

## 7. What I could not verify, and what failed to fetch

**No fetch in this run was blocked by the corporate proxy.** Every HTTPS request resolved and
returned a real body; the failures below are genuine 404s and GitHub's unauthenticated API rate
limit, not Netskope. `curl --ssl-no-revoke` was used throughout and no TLS error occurred. That is
worth recording because it differs from previous sessions on this machine.

**Fetches that failed (all 2026-09-28):**

| URL | Result | Consequence |
|---|---|---|
| `docs.litellm.ai/docs/proxy/enterprise` | HTTP 404 (page moved) | The **feature-by-feature** OSS/Enterprise split in §3.1 comes from search summaries of `docs.litellm.ai/docs/enterprise` and a third-party pricing guide, **not** read first-hand. **The load-bearing evidence — the two `LICENSE` files and the `Dockerfile` lines — is first-hand.** Confirm the feature list before phase 6 plans around it |
| `raw.githubusercontent.com/riverqueue/river/master/README.md` | HTTP 404, twice (default branch is not `master`) | River's OSS-vs-Pro split is from `riverqueue.com/docs/pro` via search, not read directly. The **MPL-2.0 `LICENSE` is first-hand** |
| `raw.githubusercontent.com/daytonaio/daytona/main/LICENSE` | HTTP 404 | Daytona's current licence is **unestablished**. Rejected on abandonment instead |
| `raw.githubusercontent.com/a2aproject/A2A/main/specification/json/a2a.json` | HTTP 404 (path moved) | `TaskState` values in §4.1 were read from `a2a-protocol.org/latest/specification/` instead. The **Apache-2.0 `LICENSE` on `a2aproject/A2A` and `a2a-go` is first-hand** |
| `asciinema-server` tags `v2.0.0`, `v3.0.0`, `v20210424`, `v20231122` | HTTP 404 (tag names differ) | **I could not establish whether asciinema-server was ever AGPL-3.0.** `main` is Apache-2.0 today. It does not matter to us — we adopt the player and store recordings in the Hub — but do not cite it as a relicensing example without checking |
| `api.github.com` (twice) | HTTP 403, rate limit exceeded (unauthenticated, 60/hr) | Stars and `pushed_at` are missing for `e2b-dev/infra`, `microsandbox`, `falco`, `yjs`, `autogen`, `langgraph`, `crewAI`. Their **licences were read first-hand** from `raw.githubusercontent.com` |
| `OpenHands/OpenHands/main/enterprise/README.md` | HTTP 404 | Good news: I probed for the LiteLLM-style paid-directory trap in OpenHands and **found none**. Not proof of absence — I checked one path |

**Claims I have not verified and would not defend without more work:**

1. **asciinema-player's bundle size** is unmeasured. I said "~100 KB class"; treat that as a guess
   and measure before committing (§3.5).
2. **centrifuge's exact presence and recovery semantics** were taken from prior knowledge plus the
   repository's activity, not from reading its documentation this session. The **MIT `LICENSE` is
   first-hand**. Read the docs before the plan quotes behaviour.
3. **NATS per-key TTL version floor.** The KV docs confirm per-key TTL, CAS and watchers but give no
   minimum version; per-key TTL is a recent feature. Pin and test.
4. **humanlayer's licence is ambiguous.** Its `LICENSE` file says *"Apache Software License 2.0 …
   Licensed under the Apache License, Version 2.0"*, but the GitHub API reports `NOASSERTION`,
   which means their detector could not match the file — it carries a header rather than the full
   text. Study-only, so nothing turns on it; **if it were ever a dependency this needs legal eyes.**
5. **Transitive dependency trees were not walked** except for `asciinema-player` (three deps, all
   MIT). For NATS, centrifuge, River, Cedar and the xterm addons I read only the top-level licence.
   A full `go-licenses` / `license-checker` sweep belongs in the CI gate `OD-11` already mandates,
   not in a research document.
6. **Release dates.** I used the GitHub API's `pushed_at`, not the latest release tag. "Pushed
   2026-09-27" means commits, not a release.

---

## 8. Honest accounting

**What this saves.** Against TM1's ≈95 focused days (`00-summary.md` §12), my estimate:

| Item | Days it removes | Confidence |
|---|---|---|
| NATS JetStream: typed bus, durable inbox, subject-level cross-user authorisation, KV leases, presence store | 8–12 | high — this is five hand-built subsystems |
| centrifuge: team-strip transport, presence, join/leave, reconnect recovery | 4–6 | medium-high |
| River: durable Orchestrator turns, transactional enqueue, retries | 4–5 | medium |
| Cedar: authorisation engine, role-to-verb table, tighten-never-loosen | 3–4 | medium |
| CloudEvents: event envelope, provenance attribute | 1–2 | high |
| *less* integration, learning and operational-plan cost | −4 to −6 | — |
| **Net** | **16–24 days of ≈95 (roughly a fifth of TM1)** | |

Separately: about **a day** on phase 2's xterm addons, and — outside TM1's budget — the
asciinema-player adoption removes a terminal-replay build that would otherwise be weeks in phase 9.

**What this costs.** Five new third-party names in the security review, two new runtime components
in phase 8 (NATS; centrifuge is in-process), one MPL-2.0 obligation to hold, and about **half a day**
of un-saved work to fix the LiteLLM image (§3.1) — which is a cost that buys the removal of a real
licence exposure.

**What open source cannot do for this domain.** Everything that makes PanaceaApp *PanaceaApp* is
still ours to write: the provenance thread through four phases, the Orchestrator's negotiation
policy, the calm team strip that `OD-15` describes, the Kata warm-pool controller, and the cockpit
itself. The survey found **no permissively licensed project that implements the cross-owner
Orchestrator model of `OD-16`** — the closest, `humanlayer`, does human-in-the-loop approval for a
single owner. That gap is the product.

**The honest headline is not "we can take a lot."** It is: *we can take the plumbing — messaging,
presence, durability, authorisation, replay — and it is worth about a fifth of team mode; and there
is a licence defect in a locked decision that must be fixed before phase 6 writes a chart.*

---

## 9. Do these five things

1. **Fix the LiteLLM image** (§3.1): build from the MIT tree with `enterprise/` removed, add the CI
   layer check. Half a day. Blocks nothing today, blocks everything at first customer-hosted deploy.
2. **Put NATS JetStream in the team-mode plan** as the bus, presence store and lease manager (§3.2),
   and write `OD-16`'s "Orchestrators are the only cross-user channel" as **subject permissions**,
   so the security critique has something enforceable to read.
3. **Add the six `@xterm/*` addons** to `desktop/package.json` and settle the WebGL fallback (§3.6).
   One day; it is phase 2 residue and it is free.
4. **Write the `OD-16` / `OD-24` rules as Cedar policies** in the Hub plan (§3.8) before the
   role-to-verb table becomes an `if` ladder.
5. **Read three things before team mode's records are designed**: OpenHands' EventStream, Cline's
   auto-approve matrix, humanlayer's approval channel (§5.1). Half a day each, and they will change
   the shape of the `directive` record.

*Sources are cited inline. Every licence statement was read from the project's own `LICENSE`,
`COPYING` or package metadata on 2026-09-28; where a claim rests on anything weaker, §7 says so.*
