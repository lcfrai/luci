# Phase 8: Our own environment

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Status of every Terraform file this plan produces: WRITTEN, `terraform fmt`'d and
> `terraform validate`d only.** No task in this plan runs `terraform plan`, `terraform apply`,
> `terraform import`, `az`, or `kubectl` against Azure. No stack is initialised against a remote
> backend (`init -backend=false` only). The first `plan` happens after the owner provides the
> environment (owner decisions `2026-09-23-owner-decisions.md:52-56`), in the owner-gated tasks at
> the end of this plan, which are written but **not executable** until then. A task that finds
> itself needing an Azure call has left this plan.

**Status:** plan, drafted 2026-09-23 on `feat/env` from `3303fd6`. Not reviewed. Not started.

**Goal:** PanaceaApp gets its own Azure environment, entirely described in this repo, so that the
day the owner hands over a subscription one reviewed pipeline run stands it up — and a user who
signs in to the cockpit gets a **cloud machine**: a Kata microVM per session on AKS, a disk that
keeps their clones and conversations between sessions, reached from the desktop through an
in-cluster broker that knows who they are. The same environment hosts **the Hub** (ADR-H1) and its
Postgres. Until the subscription exists, everything here is proven only by `terraform validate`,
`helm lint`/`helm-unittest`, and Go tests against fakes; nothing is claimed about Azure.

**Spec:** `docs/superpowers/planning/2026-09-23-owner-decisions.md` (OD-1…OD-13, which win over
every older document), `docs/PROGRAM.md` (phase 8, cross-cutting constraints, open risks 1 and 2),
`docs/FUNCTIONAL-SPEC.md` "Machines" (`:42-44`), the survey
`docs/superpowers/planning/env/survey-own-environment.md` (G-1…G-12, OQ-1…OQ-6), phase 2's Go
contract (`docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md`, the Kata notes at
`:776-790` and the machine enum at `:1580-1600`), the Hub's ADR
(`.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md`, sibling worktree) and the
seams phase 2 task 26 names for this phase (`docs/superpowers/plans/phase-2-parts/task-26.md:936-994`,
S8.1–S8.7 and SX.10).

---

## Architecture

```
 desktop (Tauri) ──► control plane, 127.0.0.1:8730 (unchanged bind) ──┐ user's Entra access token
                      machines.Set:                                    │ (Hub's token source, HA-2)
                        "local" → dockerbe            (unchanged)      │
                        "m-…"   → remotebe ──── HTTPS / WSS ───────────┘
                                                        │
 our subscription · Australia East · one VNet           ▼
 ┌──────────────────────────────────────────────────────────────────────────────────────────────┐
 │ public IP ─► Traefik (Gateway API) ─► HTTPRoutes, TLS from cert-manager (DNS-01, WI)          │
 │                     │ hub.<zone>                    │ broker.<zone>                           │
 │  apps pool          ▼                               ▼                                         │
 │   ns panacea-hub:  Hub (ADR-H1)          ns panacea-broker: broker (katabe in-cluster)        │
 │        │ Entra-only, private endpoint           │ Role in panacea-sandboxes only              │
 │        ▼                                        ▼                                             │
 │   Postgres Flexible (data stack)     sandbox pool (KataVmIsolation, AzureLinux, tainted)     │
 │                                        ns panacea-sandboxes, one per cluster:                 │
 │                                          machine pod  m-…  ── Azure Disk PVC (RWO), per       │
 │                                            (machined: catalog + git)       machine            │
 │                                          session pods s-… ── same disk, subPath mounts,       │
 │                                            (sandboxd, unchanged)  required affinity to the    │
 │                                                                   machine pod                 │
 │                                          balloons (lowest priority) + image pre-pull          │
 │  ACR Premium (PE, kubelet AcrPull) · Key Vault (PE, RBAC) · Log Analytics (+DCR, cap, audit)  │
 │  egress: NAT gateway (dev); Azure Firewall with an agent allowlist where OQ-3 requires it     │
 └──────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Five new moving parts, and nothing else.**

1. **Terraform, five stacks** under `infra/terraform/stacks/` — `bootstrap`, `foundation`, `data`,
   `identity`, `workloads` — built from small modules under `infra/terraform/modules/`, one state
   per stack per environment. No stack talks to the Kubernetes API (P8-D3).
2. **Helm charts** under `deploy/charts/` — `platform` (namespaces, priority classes, policies,
   quotas, warm capacity), `edge` (Traefik, cert-manager, the Gateway), `broker`, `hub` — applied by
   the `deploy` pipeline with Entra credentials (`kubelogin`), never by Terraform.
3. **The broker** (`control-plane/cmd/broker`): `katabe` moved in-cluster, behind the ingress,
   authenticating the caller's Entra token and scoping every object to them (OQ-1). It owns cloud
   machines: a disk, a machine pod, and the session pods that mount the disk.
4. **The machine agent** (`control-plane/cmd/machined`), in the machine pod: phase 2's catalog
   (`internal/project.Scanner`) and phase 3's git runner, **running where the files are**, reached
   only through the broker.
5. **`remotebe`** in the laptop's control plane: `backend.Backend` over the broker, plus the
   `machines.Set` that lets one control plane serve the local machine and cloud machines side by
   side. The desktop keeps talking to `127.0.0.1:8730` only.

**What does not change.** sandboxd, its wire protocol, the sandbox image's contract with the
control plane (`PANACEA_APP_COMMAND`/`_ARGS_JSON`/`_ATTACH_TOKEN`), the control plane's loopback
bind and CORS/Origin policy, the `backend.Backend` interface's seven methods (seam S8.1), and the
rule that nothing derives meaning from PTY bytes. The broker and the machine agent relay and run;
they never read a terminal stream.

