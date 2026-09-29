# 01 — Cloud agent machines: buy, adopt or integrate instead of build

**Status:** IN PROGRESS (skeleton). Sections fill in one at a time.
**Date:** 2026-09-30. **Capability:** isolated per-session sandboxes that run unmodified
coding-agent CLIs (Claude Code, Codex, Gemini CLI) with a real PTY, warm pools, microVM-class
isolation, on Azure or bought with an AU region, driven by API as a `Backend` of our Go control plane.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## 1. Answer first

_TBD — the one recommendation, its integration days, and what it replaces._

## 2. What the capability must do (the bar every candidate is scored against)

What already exists, so the bar is concrete: the phase 2 control plane drives every sandbox through
one Go interface, `backend.Backend` (`control-plane/internal/backend/backend.go:337`): `Kind`,
`Create(Spec)`, `Get`, `List`, `Attach(id, token) -> *websocket.Conn`, `Destroy`, `Close`. `Spec`
carries an image, a command plus args, env, a per-sandbox attach token, a workdir and resources.
Inside the image, our own `sandboxd` owns the PTY and serves `/healthz` and a WebSocket `/attach`.
`dockerbe` (local) is built; `katabe` (Kubernetes pods with a Kata `RuntimeClass`) exists in code
but has never run against Azure. So a candidate does **not** have to supply a terminal UI, an agent
loop or a cockpit. It has to supply **a place to run our image, fast, isolated, in AU, by API**.

| # | Must | Why it is hard-required |
|---|---|---|
| M1 | Runs an **arbitrary OCI image** (ours, with `sandboxd` and the unmodified CLIs) and a long-lived process with a **real PTY** | `OD-8`: nothing reads PTY bytes for meaning; the harness stays unmodified. A code-interpreter API is not enough |
| M2 | **Hardware-virtualised isolation** per session (microVM, Hyper-V or equivalent), not shared-kernel containers alone | Agents are untrusted. The locked plan is Kata on AKS for exactly this |
| M3 | A **bidirectional stream** into the sandbox (WebSocket or TCP port) reachable from our control plane, not only request/response exec | `Backend.Attach` returns a WebSocket; live terminals, attach, fork and resume depend on it |
| M4 | **Warm start**: a claim served from a pre-created pool in about a second, not a cold VM boot | Kata cold start is seconds to tens of seconds; the cockpit spawns sessions in parallel |
| M5 | **Azure, AU region**, in our subscription first (`OD-5`, `OD-18`), then a customer's own tenancy; or a bought service with an **AU region** and zero data retention | `OD-19`, `OD-24`: AU-only projects must stay in AU |
| M6 | **Licence we can ship into a customer tenancy**: permissive, or commercial that KPMG can buy | AGPL, GPL, SSPL, BUSL, fair-code and Elastic are restricted |
| M7 | **Egress control**: default-deny with an allowlist (the LiteLLM gateway, the git hosts, package mirrors) | Agents must reach only the gateway (`OD-19`, `OD-21`) and the repo hosts (`OD-4`) |
| M8 | **Session lifetime of hours to days**, and a way to keep a user's clones and `~/.claude` transcripts between sessions | Phase 8's "machine": a disk that keeps clones and conversations (`OD-2`, `OD-8`) |

| # | Should | Why |
|---|---|---|
| S1 | An API a Go program can call (REST or a Go SDK) | It becomes a third `Backend` beside `dockerbe` and `katabe` |
| S2 | Per-session env and secrets (the attach token, the per-session gateway token) | `OD-19`: each sandbox gets a scoped token, never an org key |
| S3 | Entra-based admin and audit (Azure RBAC, Activity Log, Log Analytics) | Regulated-enterprise evidence (phase 9) |
| S4 | Fast image bumps, so the cloud tracks the latest Claude Code CLI (`OD-20`) | The canary-gated image pipeline is the whole cloud-first argument |
| S5 | Snapshot or fork of a running sandbox | The cockpit already forks sessions; a memory snapshot makes it instant |

## 3. Candidates at a glance

_TBD — one row per candidate: licence, deploy, AU, isolation, PTY, warm start, verdict._

## 4. Candidates in detail

_TBD — one subsection each._

## 5. Recommendation

_TBD — solution, days, what it replaces (phases and task counts), glue we still build, risks._

## 6. Calls only the owner can make

_TBD_

## 7. What this study could not verify

_TBD_

## 8. Sources

_TBD_
