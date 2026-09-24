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
 │  egress subnets (both modes): system, apps, sandbox and agents (the CI pool) — nothing else   │
 └──────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Five new moving parts, and nothing else.**

1. **Terraform, five stacks** under `infra/terraform/stacks/` — `bootstrap`, `foundation`, `data`,
   `identity`, `workloads` — built from small modules under `infra/terraform/modules/`, one state
   per stack per environment. No stack talks to the Kubernetes API (P8-D3).
2. **Helm charts** under `deploy/charts/` — `platform` (namespaces, priority classes, policies,
   quotas, warm capacity), `cert-manager` (its own release, so its CRDs and webhook exist before
   anything uses them), `edge` (Traefik, the Gateway, the issuer), `broker`, `hub` — each bringing
   its own NetworkPolicy onto `platform`'s default-deny — applied in that order by the `deploy`
   pipeline with Entra credentials (`kubelogin`), never by Terraform.
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

---

## The decisions this plan rests on

### Owner decisions (they win over everything below)

| Id | What it forces here | Where |
|---|---|---|
| **OD-5** | Our own environment, written from scratch in this monorepo, modelled on Panacea's live `deploy/main.tf` but copying none of it | every task |
| **OD-13** | No Panacea source enters this repo. The survey's `P:` citations are **patterns**; no `.tf`, script or YAML is transcribed | every Terraform and chart task |
| **OD-3** | The Hub's stores are server-side from day one, so its hosting and Postgres are in this phase | 08, 10, 29 |
| **OD-6** | The desktop signs in with Entra auth-code + PKCE; the control plane calls servers **as the user**. So the broker authenticates the user's token and nothing is a shared service account | 09, 16, 17, 22 |
| **OD-2** | A worktree is a local clone. On a cloud machine the project repo and its lane clones live **on one filesystem** — the machine's disk — so the phase 3 model (hard-linked clones, fetch back under `refs/panacea/lanes/…`) runs unchanged inside the machine | 15, 19, 20, 25 |
| **OD-4** | ADO and GitHub both: sandbox egress and the machine agent's clone path must reach both hosts | 11, 21 |
| **OD-8** | The transcript adapter is pinned to the CLI version in the sandbox image, so the **image digest is the adapter pin**: the `images` pipeline runs the adapter's canary against the image it is about to push. Moving the sandbox base moves the pin, so the move off end-of-life Node 20 is its own task (review m15) | 27, 27a |
| **OD-10** | Forget deletes the transcript; removing a worktree deletes its store. On a cloud machine both are paths on the machine disk, deleted by the machine agent | 20 |
| **OD-11 rule 3** | Dependencies are MIT, Apache-2.0, BSD or ISC, checked in CI. This plan applies the gate **repo-wide** to two kinds of dependency. (1) Every linked graph: Go modules, the desktop's production npm graph and Cargo, in task 28's `licences.yml`. (2) Every Helm chart dependency: `charts.yml` checks each `Chart.lock` entry against `licences/charts.yaml` (task 28 writes the check, task 13 the first entries). **Terraform providers are outside the gate.** They are MPL-2.0 tools: Terraform runs them as separate processes, they ship in nothing, and each stack's committed `.terraform.lock.hcl` pins them by hash. Task 28's allowlist says so by name | 28, 13 |
| **OD-12** | Owner hands-on steps are batched at the end. Every owner-observed item in this phase stays **unverified** until then | 31, 32 |
| **OD-7** | `github.com/lcfrai/panaceaapp` is a backup remote, not a CI system | 28 |

### The survey's open questions, ruled

Recorded in `survey-own-environment.md` §8 (`:561-575`), each with a recommendation. This plan
proceeds on every recommendation. **Only OQ-3 needs the owner**, and it is carried as open question
Q1 below.

| Id | Ruling in this plan |
|---|---|
| **OQ-1** | **An in-cluster broker** runs `katabe`; the laptop's control plane gains `remotebe`. Every broker Get/List/Attach/Destroy is scoped to the caller — today `sandbox.Manager.List` returns every pod in the namespace (`control-plane/internal/sandbox/manager.go:240-247`) → P8-D4 |
| **OQ-2** | **The Hub runs on the same AKS, on the `apps` pool.** ADR-H1 owns the Hub's shape; if it rules otherwise, task 29 changes and nothing else does |
| **OQ-3** | **Owner.** Plan proceeds on: dev = NAT gateway plus network policy; the Azure Firewall module is written (task 07) and selected by `egress_mode = "firewall"` where the classification requires it → Q1 |
| **OQ-4** | **No server-side identity broker (Keycloak) in v1**; the Hub and the broker validate Entra tokens themselves. ADR-H1 may overrule for the Hub; the broker needs no broker either way |
| **OQ-5** | **Dev: public API server with authorised IP ranges and Entra RBAC. Prod: private cluster**, reached by the in-VNet agent pool (task 30). A private cluster is create-time immutable, so `prod.tfvars` sets it at create |
| **OQ-6** | **One environment (`dev`) is applied first.** Every stack is parameterised by `environment`; `prod.tfvars` is written beside `dev.tfvars` so the prod posture is reviewable now. **No `terraform.tfvars` exists in any stack**, and a test enforces it (task 01) |

### This plan's own decisions

| Id | Decision | Why |
|---|---|---|
| **P8-D1** | **Five stacks, one state each per environment**: `bootstrap` (state storage) → `foundation` (network, observability, registry, key vault, AKS, DNS, egress) → `data` (Postgres) → `identity` (Entra, `azuread`) → `workloads` (managed identities, federated credentials, role assignments). State key `<stack>.<env>.tfstate`. A later stack reads an earlier one through `terraform_remote_state`, never by guessing a name | A Helm value or an app-registration edit can never plan a cluster replacement. Panacea's single 22-file root with one state is what made every edit a cluster risk (survey §2.2, `:311`) |
| **P8-D2** | **Secure at create time, no posture flags** for create-time fields: Azure CNI Overlay + Cilium, Entra RBAC with local accounts disabled, workload identity + OIDC issuer, host encryption, private endpoints for every PaaS dependency. Per-environment *choices* (SKU tier, private API, egress mode, HA) are variables; *hardening* is not | We have no live cluster to keep byte-identical (survey §2.2, `:309`) |
| **P8-D3** | **No `kubernetes` or `helm` provider in any stack.** In-cluster objects are Helm charts in `deploy/charts/`, installed by the `deploy` pipeline with `kubelogin` (Entra), `helm upgrade --install --atomic` | Panacea's Helm provider authenticates with the local admin certificate and breaks the day local accounts are disabled (survey §1.3, `:93-95`) — which P8-D2 does on day one |
| **P8-D4** | **The broker** (`control-plane/cmd/broker`) is the only thing with Kubernetes credentials for sandboxes. It validates an Entra access token (issuer = our tenant, audience = the broker API, scope `Sandboxes.Access`), derives **owner = `tid` + `oid`**, labels every object it creates with a hash of that owner, and filters every read by it | The alternative — per-user kubeconfigs and port-forwards — hands every user the cluster API (survey OQ-1). The laptop keeps no cluster credential, so the control plane's `PANACEA_APP_KUBECONFIG` refusal (`control-plane/cmd/api/main.go:241-271`) stays exactly as it is (seam S8.2) |
| **P8-D5** | **One sandbox namespace, `panacea-sandboxes`.** The broker's RBAC is a namespaced `Role` there and nowhere else. Users are separated by owner labels enforced in the broker, by NetworkPolicy (no sandbox reaches another), and by per-owner limits in the broker | A namespace per user needs the broker to hold cluster-scoped RBAC to create namespaces and bind roles in them, which is a larger blast radius than the one it removes |
| **P8-D6** | **A cloud machine is one Azure Disk (RWO, task 11's `panacea-machine-disk` class on `Premium_ZRS`, the broker's default through `PANACEA_APP_BROKER_MACHINE_STORAGE_CLASS`, task 17) plus one machine pod.** Zone-redundant, not the built-in `managed-csi-premium` (LRS), because the sandbox pool spans three zones and a sleeping machine's disk must re-attach wherever its next pod lands; `Premium_ZRS` in the region is task 32's P14, **UNVERIFIED**, with `Premium_LRS` and zone-pinned machines as the fallback. Every session pod of that machine mounts **sub-paths** of the same disk and is co-scheduled with the machine pod by **required pod affinity** on `panacea-app/machine-id`. Layout on the disk: `projects/`, `lanes/`, `conversations/<worktree id>/`. Sleeping = the disk with no pods | Clones hard-link objects only within one filesystem (OD-2); small-file I/O (`npm install` in a worktree) is far better on a block disk than on NFS. A machine bounded by one node is also what the word means. **The ceiling that follows (review I5):** every session of a machine must fit on the one node that holds its machine pod, beside whatever else that node runs (other users' machines and sessions, the DaemonSets), and neither a balloon nor the autoscaler can add room there, because a new node does not hold the machine pod. On dev's `Standard_D8s_v5` (8 vCPU, allocatable a little under 8) with the machine pod at `500m`/`1Gi` and 2 CPU / 4 GiB sessions, that is **about three sessions per machine on an otherwise empty node**, fewer on a shared one (arithmetic; allocatable and the per-pod Kata overhead are task 32's P07, **UNVERIFIED**). The per-user cap of 8 sessions over 2 machines (task 17) is therefore not reachable on one machine. A session's size is server policy: remotebe sends none, so every cloud session takes the broker's `PANACEA_APP_BROKER_SESSION_*`, and the largest a caller may ask for (`PANACEA_APP_BROKER_MAX_*`, equal to the LimitRange `max`) is **derived from the sandbox SKU**, the largest session that fits beside a machine pod on an empty node (`6000m` / `20480Mi` on `Standard_D8s_v5`, arithmetic, P07 measures; tasks 11, 17, 22, 29; b05 re-review 2, 4). Under the affinity, the broker refuses the session that does not fit with **`409 machine_full`** (`details.sessionsPerMachine`) within seconds, on the scheduler's own `Unschedulable` verdict, and deletes the pod it tried, identified by its attach token; a readiness timeout deletes its pod too (task 18). With the affinity off (the fallback below) it never answers `machine_full`, because the autoscaler can then place the session (b05 re-review 3). The control plane maps it (tasks 22, 24b) and the cockpit shows its one sentence (task 26); P07 decides between a larger sandbox SKU and a smaller default session. **Fallback**, if task 32 finds Kata cannot share one RWO disk between two pods on a node: Azure Files NFS (RWX), selected by `PANACEA_APP_BROKER_MACHINE_STORAGE_CLASS` with affinity off. **Kata support for either is UNVERIFIED** (survey G-5) |
| **P8-D7** | **The machine agent** (`control-plane/cmd/machined`) serves the catalog and the git runner **inside the machine pod**, on the pod network, behind a per-machine bearer token held in a Kubernetes Secret. Only the broker can reach it. **Nothing uses `pods/exec`** | Reuses `internal/project.Scanner` and phase 3's local git runner unchanged, on Linux paths, with phase 3's §HGH hardening and, on top, an explicit `Env` allowlist, no credential in a lane, and no `PANACEA_APP_*` in git's environment (task 21, review I7). `pods/exec` would put git arguments (and, on a clone, credentials) into the API server's audit log and would need an SPDY dependency |
| **P8-D8** | **Images are built from binaries compiled outside Docker** (`CGO_ENABLED=0 GOOS=linux go build`) and copied into the image, and every image is referenced **by digest**. The machine image is `FROM` the sandbox image's digest plus the `machined` binary, so one image pin covers the CLI version (OD-8) | The corporate proxy breaks `go mod download` inside a container (`images/sandbox/Dockerfile:3-8`); the control-plane module is not vendored and should not become so to build two binaries |
| **P8-D9** | **Ingress is the Kubernetes Gateway API, implemented by Traefik**, with TLS from cert-manager (DNS-01 against our Azure DNS zone, workload identity). **Terraform owns the public IP and the A records**, so no DNS step is manual and no external-dns runs | Panacea's wildcard A record is a manual step after Traefik gets an IP (survey §1.5, `:124-125`). The AKS application-routing add-on is built on ingress-nginx, whose upstream announced its retirement — **from training knowledge, UNVERIFIED**; Gateway API resources keep the implementation swappable either way |
| **P8-D10** | **Postgres Flexible Server with Entra-only authentication** (`password_auth_enabled = false`), public access off, a private endpoint, and an owner-provided Entra group as its administrator. The Hub connects as its own user-assigned identity through workload identity. The Hub's database roles are created once per environment by a **human** member of the admin group (task 31 §7). This is SQL, not Terraform, and no pipeline identity is in that group (P8-D16, review I8) | No database password exists to leak into state (survey §2.2, `:318`) |
| **P8-D11** | **No usable long-lived secret**: OIDC service connections in CI, workload identity in the cluster, Key Vault (RBAC mode, private endpoint) for whatever ADR-H1's Hub needs, read through the Secrets Store CSI add-on. Per-machine and per-sandbox bearer tokens are minted by the broker and live as Kubernetes Secrets owned by their pod, so they are garbage-collected with it. **"Usable" is the word that matters (review m7):** some keys still exist and Terraform state still records them, namely the state account's access keys and the Log Analytics workspace's shared keys (azurerm exports both as attributes, **from training knowledge, UNVERIFIED**). They are unusable because the plane that would accept them is off, and a test pins each: `shared_access_key_enabled = false` (task 02, `state_account_has_no_shared_key`) and `local_authentication_enabled = false` (task 04, `workspace_has_no_shared_key_auth`). **State is therefore treated as sensitive**, and so is everything that carries it: the `tfstate` container is reachable only through Entra data-plane roles (task 02), and task 30's binary `plan-<stack>` artefact holds the prior state as well (a saved plan embeds it, **training knowledge, UNVERIFIED**), so the artefact is as sensitive as the state it came from | Survey §2.2 `:315-317` |
| **P8-D12** | **Sandbox egress** (dev): internet via the NAT gateway; **denied** to `169.254.169.254`, the pod and service CIDRs, the VNet, and every other sandbox; DNS to kube-dns only; ingress to a sandbox only from the broker on 8080 and to a machine pod only from the broker on 8081 | Kata isolates the kernel, not the network (survey G-4, `:420`) |
| **P8-D13** | **Azure DevOps is the CI of record**; one pipeline per concern under `pipelines/`, path-triggered, sharing pinned-tool templates. The root `azure-pipelines.yml` stays, as the Go pipeline, so the definition already registered against it keeps working. `infra-plan` and `deploy` are written and **not registered** until the environment exists | Survey §5 (`:437-476`); owner decisions `:56` |
| **P8-D14** | **Tool and provider pins.** Terraform `>= 1.9.0, < 2.0.0` in every stack, CI pinned to **1.15.8** (the version on the build laptop, `terraform version` on 2026-09-23); `hashicorp/azurerm` **4.81.0** (already locked, `infra/terraform/.terraform.lock.hcl:4-6`), `hashicorp/azuread` **3.9.0**. Each stack commits its own `.terraform.lock.hcl` | Every argument name this plan cites was checked against `terraform providers schema -json` for exactly these versions, in a scratch directory, on 2026-09-23 |
| **P8-D15** | **The RuntimeClass name becomes configuration**, `PANACEA_APP_RUNTIME_CLASS`, default `kata-vm-isolation`, and the first smoke check is `kubectl get runtimeclass` | The code hardcodes `kata-mshv-vm-isolation` (`manager.go:20`) against a pool created with `KataVmIsolation` (`infra/terraform/kata.tf:16`). Which name AKS installs is **UNVERIFIED** (survey G-3); the default is the GA name from training knowledge, and task 32 proves it |
| **P8-D16** | **Four pipeline identities per environment, split by what they may do** (plan, read-only, the only one PR builds reach; infra; cluster; push). The write connections carry ADO branch control and per-pipeline permission. No pipeline identity is a Postgres or cluster administrator or holds Graph application rights, so the database bootstrap and `stacks/identity` are human steps. No stack grants a pipeline identity anything: the owner grants every pipeline role (task 31 §2a, §4a). This removes the **administrator** routes only. An identity that deploys a workload can still act as it — the infra identity as the Hub and cert-manager, the cluster identity as the broker — and the ADO checks bound those routes (the declared residuals below). The guards in the repository are hygiene, not the boundary. The full table is "Deploy identities" below | Review C3 (a PR build ran with the apply identity), review I8 (one principal held Azure RBAC, cluster Secrets, the Hub's data and possibly tenant Graph rights) |

---

## Global constraints

Every task inherits all of these. A task that needs to break one stops and reports it.

**Shell.** Every command runs through `~/.claude/tools/claudezsh/claudezsh '<command>'` (the
device-wide rule; the blocks in the parts show commands bare). No heredoc through the wrapper:
files are written with the editor tools, and commit messages go to a file first.

**Git.** Commit by pathspec, `git commit -F <file> -- <paths>`; a new file needs `git add <path>`
first. Never `git add -A`/`-u`/`.`, `stash`, `reset`, `checkout`, `switch`, `rebase`, `merge`,
`--amend` or `push`. Every commit message ends with exactly:

```
Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

Write each commit-message file at a path unique to the task (for example
`$TEMP/p8-task-NN-msg.txt`), never a generic name: on 2026-09-23 two agents sharing one scratch
directory wrote the same `m1.txt`, and commit `35e73fd` in this branch carries a sibling's phase 5
message over this plan's first piece.

**Azure.** No task runs `terraform plan`, `apply`, `import`, `state`, `console` against a backend,
`az`, `kubectl` or `helm install` against any cluster. The only Terraform commands are
`terraform fmt`, `terraform init -backend=false -input=false`, `terraform validate` and
`terraform test` against mock providers, run by `infra/scripts/validate.sh`, plus one by-hand
`terraform graph` in task 07 (configuration only: no backend, no credentials; review m8). Tasks 30–32 describe Azure-touching steps as **runbooks** and scripts
that are written, linted and **not run**. Provider downloads from `registry.terraform.io` work from
the build laptop (`terraform init -backend=false` in a scratch directory, 2026-09-23); if the proxy
ever breaks them, stop — do not hand-edit a lock file.

**Terraform style.** No `terraform.tfvars` anywhere (auto-loaded files are how Panacea pointed a bare
`apply` at production, survey §1.1 `:46-51`). Only `<env>.tfvars`, holding no subscription id,
tenant id, object id or secret — those arrive as `TF_VAR_*` from the pipeline. Every stack has
`versions.tf` (pins), `backend.tf` (`backend "azurerm" {}`, partial), `variables.tf`, `outputs.tf`,
`main.tf`, and a committed `.terraform.lock.hcl`. Every variable has a `description` and a `type`;
every input with a closed set has a `validation`. Half-configured features fail the plan through a
`check` or `precondition` (the survey's `check`-block idiom, `:298-299`). No `count = 1` + `moved {}`
scaffolding (survey §2.2, `:310`). Every resource carries `local.tags`, which includes
`product = "panacea-app"`, `environment`, `stack` and `managed-by = "terraform"`.

**Go.** From phase 2, unchanged: the module prefix is `github.com/kpmg-au-emu/panacea-app`;
`go test ./...` is invalid at the workspace root, so always
`go test ./sandboxd/... ./control-plane/... -race -count=1`; the `go 1.23` directive stays in
`go.work` and every `go.mod` (the two existing, and task 13's test-only
`deploy/tests/netpolcover`, run as `go test ./deploy/tests/netpolcover/... -count=1`) — check
`git diff` on them before every commit; `sandboxd/vendor/`
stays committed and **sandboxd gains no dependency in this phase**. **Neither does the
control-plane module**: `internal/entra` verifies RS256 with the standard library (task 16), and
the broker, the machine agent and `remotebe` use only `coder/websocket` and `client-go`, which
`control-plane/go.mod` already requires. A task that finds it needs a module stops and reports it,
with the licence, the module's `go` directive and the `go.sum` additions listed.

**Listeners.** The control plane binds `127.0.0.1:8730` and nothing in this phase changes that.
The broker (`:8080`) and the machine agent (`:8081`) are the only new listeners; both exist only
inside the cluster, both refuse to start without authentication configured, and the broker is
reached from outside only through the Gateway's TLS listener. Sandbox pods stay on `:8080`
(sandboxd, unchanged).

**The `MaxMessageBytes` rule gains two sites.** `backend.go:29-37` names three places that must call
`SetReadLimit(MaxMessageBytes)` after `Accept`/`Dial`. After this phase there are five: sandboxd's
`/attach`, `backend.Dial`, the control plane's client `Accept`, **the broker's `Accept`** (task 18)
and **`remotebe`'s dial to the broker** (task 22). Each of the two new ones has a test that sends a
64 KiB frame.

**Harnesses.** Agent CLIs run unmodified and unwrapped; nothing derives meaning from PTY bytes; the
broker relays frames it never decodes. No subscription credential is pooled, baked into an image,
stored in Key Vault or shared between users: each user runs `/login` inside their own session
(credential rotation is phase 6's).

**Config.** Every new setting is a `PANACEA_APP_*` environment variable, read in exactly one
`config.go` per binary, and every one is listed in its part with its default. No secret in source.

**UI.** Task 26 is the only UI task. It follows `docs/DESIGN-SYSTEM.md`'s non-negotiables (no
indigo/violet gradients, 1px ring plus layered shadow, radius tied to size, motion only to explain
causality with `transform`/`opacity` and never `transition: all`, the one global heartbeat, never a
spinner per row, `tabular-nums` on live numbers), and its visual acceptance is **the owner's eyes**
(OD-12), never a self-report or a screenshot.

---

## Hub contract asks

**The Hub contract did not exist when this plan was written.** The asks below were written at
21:40 against a 100-line ADR-H1 (§0–§1 only). By the time tasks 16–29 were written the ADR was
complete and committed on `feat/hub` (`67609ab`, 769 lines, read by absolute path) and answered
most of them — see "What ADR-H1 answered" under the table. The contract it builds
(`docs/superpowers/contracts/2026-09-23-hub-contract.md`, "§12.3 answers HA-4 and HA-5 field by
field", ADR-H1 §5.7, `67609ab` `:536-538`) was **still absent** from the sibling tree. So every
Hub-dependent task is written against the ADR's answers, and tasks 09, 16, 22 and 29 re-check the
contract **before** they start (each has that as step 1). Nothing here invents a Hub route.

**How ADR-H1 is cited.** `feat/hub` has kept moving since `67609ab`, so a bare line number drifts
(on 2026-09-24 the branch head's ADR was 776 lines, with everything from §5.1 on six or seven lines
lower than at `67609ab`). Every ADR-H1 line citation in this plan and its parts is therefore
pinned: `67609ab :<line>` means line `<line>` of
`git show 67609ab:docs/superpowers/adr/ADR-H1-hub-architecture.md`, and each carries its § number,
which is what to follow on a later revision.

| Ask | What phase 8 needs, exactly | Needed by | If the Hub contract does not provide it |
|---|---|---|---|
| **HA-1** Shared Entra token validation | One Go package, importable by both the Hub and `control-plane/cmd/broker`, with the `entra.Validator` interface in task 16's part (complete source there). The broker and the Hub must accept and reject the **same** tokens | 16, 17 | Task 16 writes it as `control-plane/internal/entra`; the Hub keeps its own and the two are reconciled at the Hub's review |
| **HA-2** A token source in the control plane | `type TokenSource interface { Token(ctx context.Context, scope string) (string, error) }` and `var ErrSignedOut`, provided by whatever the Hub phase builds for OD-6 (PKCE, Windows Credential Manager). `remotebe` calls it once per request with the broker's scope | 22, 23 | `remotebe` ships with the interface and a stub that returns `ErrSignedOut`; cloud machines show "Sign in to use cloud machines" (task 26) until the Hub's lands |
| **HA-3** The app registrations | The exact list ADR-H1 requires — display names, sign-in audience, the desktop's redirect URIs, every exposed scope and app role, the token version, the group claim — so task 09 writes them. Phase 8's own need is **one resource API for the broker exposing the delegated scope `Sandboxes.Access`**, with the desktop's public client pre-authorised for it | 09 | Task 09 writes three registrations (desktop public client, Hub API with `Hub.Access`, broker API with `Sandboxes.Access`) and flags the Hub one as provisional |
| **HA-4** The Hub's deployable | The binary's build path and `main` package; its listen port; liveness and readiness paths and what readiness checks; every `PANACEA_APP_HUB_*` variable with its default; whether it can run two replicas (any in-process state); graceful shutdown time; outbound calls it makes (Entra JWKS, Microsoft Graph, anything else) | 27, 29 | Task 29 writes the chart against the defaults in its part (`:8080`, `/healthz`, `/readyz`, stateless, 2 replicas) and marks each as provisional |
| **HA-5** The Hub's database | Postgres major version, required extensions (`azure.extensions` allowlist), expected size and connection count, and **how migrations run**: a `migrate` subcommand the chart runs as a pre-upgrade Job under a separate DDL identity, or at start-up under an advisory lock | 08, 10, 29 | Task 08 provisions Postgres **16**, no extensions, `B_Standard_B2s` in dev; task 10 creates two identities, `id-hub` (DML) and `id-hub-migrate` (DDL); task 29 runs migrations as a pre-upgrade Job |
| **HA-6** Who calls the Hub | Whether only the control plane calls the Hub (so no browser CORS at the edge), or the desktop webview does too, and whether any machine-to-machine caller exists (the broker does not call the Hub in this plan) | 13, 29 | The edge sets no CORS headers; the Hub is reached by the control plane only |
| **HA-7** Audit and object storage | Whether the audit log is a Postgres table only, or also a Log Analytics stream (a DCR and an ingestion endpoint), and whether the Hub needs Blob storage (design bundles, release artefacts) | 04, 08 | Postgres only; no storage account is provisioned for the Hub |

### What ADR-H1 answered (sibling `feat/hub` at `67609ab`, read by absolute path)

| Ask | ADR-H1's answer | Where | Effect on this plan |
|---|---|---|---|
| HA-1 | **The Hub is a second binary in the control-plane module** (`control-plane/cmd/hub`), and it uses **this plan's `control-plane/internal/entra`** — "one package, not two to reconcile". Its validation rules are fixed: RS256 only with `kid`; JWKS cached 24 h, refetched on an unknown `kid` at most once per 5 min; `ver` 2.0; `iss` exact for the token's `tid`, which must be enrolled; `aud` = the API's client id; `scp` must contain the API's scope, app-only tokens refused; `azp` in an allowlist; 120 s skew; principal `(tid, oid)`, never `sub` or `preferred_username` | ADR §4.1, §5.2; `67609ab` `:206-238`, `:354-366` | Task 16 implements exactly that table, parameterised by audience, scope and allowed clients, so one package serves both servers |
| HA-2 | `entraauth.TokenSource.Token(ctx, scope)` in the **control plane**, single-flight refresh 5 min before `exp`, one refresh token for every resource (Hub, broker, optionally ADO); built in the Hub's client track **C1** | ADR §5.2, §5.9 (C1); `67609ab` `:334-342`, `:564` | Task 22 consumes that interface; until C1 lands, its stub returns `ErrSignedOut` |
| HA-3 | **Three registrations**, "as task 09 already guessed": desktop public client, Hub API (`Hub.Access`), broker API (`Sandboxes.Access`); details in the contract | ADR §5.2, §7.1 (row 8); `67609ab` `:340-341`, `:666` | Task 09 stands; step 1 reconciles the details |
| HA-4 | `hub serve` on `:8080`, `/healthz` (up) and `/readyz` (Postgres within 1 s, schema at the built version, JWKS fetched), stateless, **two replicas**, **20 s** graceful shutdown, config `PANACEA_APP_HUB_*` | ADR §5.1, §5.7; `67609ab` `:286-288`, `:528-538` | Task 29's chart defaults hold; task 27 builds `images/hub` from `control-plane/cmd/hub` |
| HA-5 | **Postgres 16, no extensions**; migrations by **`hub migrate`** in a pre-upgrade Job under the **DDL identity**; `hub serve` never migrates and is not ready until the schema matches; DML identity `id-hub`, DDL identity `id-hub-migrate`; the database is reached with workload identity and an Entra token from a standard-library token exchange | ADR §5.6, §5.7, §7.1 (row 8); `67609ab` `:496-503`, `:532-535`, `:666` | Tasks 08, 10 and 29 stand as written |
| HA-6 | **Only the control plane calls the Hub**; the Hub rejects any request carrying an `Origin` header; no CORS | ADR §5.4, §5.8; `67609ab` `:433-437`, `:546` | The edge sets no CORS headers (task 13 stands) |
| HA-7 | Audit in Postgres (`audit_events`, hash-chained); blobs in a Postgres table behind a `blobstore` interface; **no storage account**; audit anchoring to Log Analytics is later | ADR §5.6, §5.9; `67609ab` `:504-510`, `:580-581` | No storage account; the DCR has no Hub stream yet |

**One ADR decision touches phase 8's broker:** audiences are separate and "none is ever forwarded
to another service or into a sandbox" (ADR §5.8, `67609ab` `:543-546`). The broker therefore strips the caller's
`Authorization` header before it dials a pod; a sandbox sees only its own attach token (task 18).
**And one touches the control plane's hop:** the Hub's C3 adds a local caller key to every
control-plane route except the WebSocket attach (ADR §5.4, `67609ab` `:439-449`). Task 26 does not add it — C3
does — but it must not break it.

### What the Hub contract then confirmed (sibling `feat/hub`, `2026-09-23-hub-contract.md`, read at 2,289 lines)

The contract landed while tasks 27–33 were being written, and its §14 (`:2257-2272`) maps every
phase 8 ask to an answer: **HA-1** → §6.3's `entra` interface, which task 16 now adopts verbatim;
**HA-2** → `entraauth.TokenSource` and `ErrSignedOut` "exactly as asked" (§9.1), which task 22's
interface matches by signature; **HA-3** → §6.5's registration table, which task 09 transcribes;
**HA-4…HA-7** → §12.3 (`:2193-2208`), whose answers are the defaults tasks 08, 10 and 29 already
used, with the full variable table in §12.2 (`:2140-2191`) that task 29's chart sets and nothing
beyond. The two differences phase 8 introduces are recorded for the Hub's reviewer in task 33.

---

## Asks of phase 3 (sibling `phase-3` worktree; its code is not edited silently)

| Ask | What | Plan proceeds on | Task |
|---|---|---|---|
| **P3A-1** | Machine paths are slash-separated on every OS: `gitx.Machine` gains `Join(elem ...string) string` (`Local` → `filepath.Join`, `Remote` → `path.Join`), and phase 3 joins clone paths through it — the control plane runs on Windows | the method is added in phase 3's task 02, or by task 25 in one commit if phase 3 has landed | 25 |
| **P3A-2** | The lane index for a cloud machine is keyed by machine id | laptop-side, keyed by `(machineId, projectId)`; the single-laptop limitation is stated in the cockpit. It holds for sessions too: a cloud sandbox carries its creator's install id, and only that control plane reaps it (review I6, tasks 18, 22, 23). The default machine's id is derived from its owner, so the index also holds `createdAt` and drops entries when it changes; the broker refuses to delete the default (b09 re-review 4, task 19) | 25 |
| **P3A-3** | *Answered by phase 3 (review I7, b06):* its task 02 §HGH (H1–H5) hardens every `gitx.Exec` call, hooks and `core.fsmonitor` among many; `machined` runs that hardening unchanged, and adds an explicit `Env` allowlist, the scope-follows-`Dir` check, no credential `Env` in a lane, and an environment without `PANACEA_APP_*`. H5's read-only `.git/config`, which phase 3 builds only in its Docker `sessionMounts`, task 24b builds for cloud sessions (b06 re-review 1) | phase 3's §HGH lands before task 21 starts (task 21 step 1 checks) | 20, 21, 24b, 33 |
| **P3A-4** | Phase 3's task 05 and this phase's task 14 both edit the sandbox environment and its parity test | phase 3 lands first; task 14 step 0 checks | 14 |

---

## Open questions for the owner, each with the recommendation this plan proceeds on

Rule: a genuine owner decision is recorded, and planning goes ahead on the recommendation. None of
these blocks a task before task 31; all of them block the first `apply`.

| Id | Question | Recommendation (what the plan builds) | Why | Blocks |
|---|---|---|---|---|
| **Q1** (= OQ-3) | What is the data classification, and so: may sandboxes reach the internet through a NAT gateway with network policy, or must egress go through an FQDN-allowlist firewall? | **Dev: NAT + policy** (P8-D12). **PROTECTED or above: Azure Firewall Standard** with the agent allowlist of task 07 (package registries, `github.com`, `dev.azure.com`, model providers), selected by `egress_mode = "firewall"` | An allowlist that misses one registry breaks agents silently; open egress may fail a PROTECTED assessment. Cost gap about A$1.5–2k a month (survey §6.2, `:507`). **What the firewall buys, stated plainly (review I10): it limits *where* agents go; it does not stop a determined agent sending data out.** Under CNI Overlay a sandbox pod leaves as its node's address, so the firewall cannot tell a sandbox from a node: every rule meant for nodes (the `AzureKubernetesService` FQDN tag, Entra, the registry) is open to sandboxes too, and allowlisted hosts such as `github.com` and `*.githubusercontent.com` accept arbitrary uploads. Real per-sandbox FQDN control needs Cilium FQDN policy on pod identity, which is the option that follows. A cheaper third option exists in the provider schema — AKS Advanced Container Networking's `security_enabled` (`network_profile.advanced_networking`, azurerm 4.81.0), which enables Cilium FQDN policies; **that it meets the classification's egress control is UNVERIFIED** and it is not built here | 31 |
| **Q2** | May the edge use a public ACME CA (Let's Encrypt) for `*.<zone>`, or must certificates come from a KPMG CA? | **Let's Encrypt via DNS-01** for dev; the ClusterIssuer is a value, so a KPMG ACME endpoint or a Key Vault-issued certificate is a values change | Survey §7.2 item 3 (`:543-544`) | 31 |
| **Q3** | Is the edge internet-facing with an IP allowlist, or reachable only from the corporate network (peering / ExpressRoute)? | **Internet-facing with an allowlist**, in two separate variables (review m5). **Edge users:** `allowed_ingress_cidrs` in `stacks/foundation` (the 443 allow on the `nodes-apps` NSG, tasks 03 and 07) and the same list as the edge chart's `allowedSourceRanges` (the Traefik service's `loadBalancerSourceRanges` and its `traefik-ingress` NetworkPolicy, task 13). **Admins:** `api_authorized_ip_ranges` (task 07) feeds `authorized_ip_ranges` on the dev API server, and the environment's own egress /32 is added to it automatically; a private cluster ignores it. They stay separate because an admin workstation is not an edge user, and an edge user must not reach the cluster API. Corporate-only is a later network task | Panacea allowlists KPMG egress ranges the same way (survey §7.2 item 4, `:545-547`) | 31 |
| **Q4** | May the cluster identity install cluster-scoped RBAC (the ClusterRoles and ClusterRoleBindings of cert-manager and Traefik), which may let it reach cluster-admin through Kubernetes RBAC `bind`? Or must those objects move to an owner-run prerequisite release, installed under the cluster admin group when a chart version changes? | **Dev: the pipeline installs them.** The ADO checks are the boundary, and the `cluster-kinds` line keeps the list visible. **Before prod: decide.** The seam is `infra/rbac/cluster-deployer.json`: removing `bind`/`escalate` and the RBAC kinds from it, and moving those objects to a separate release, is a change to one file and one release table (task 30, task 13's `netpolcover` table) | Cluster-admin reaches everything in the cluster. The stricter option adds an owner step to every cert-manager or Traefik upgrade. **What Q4 does not decide:** users' session tokens. The cluster identity reaches those under either answer, because it deploys the broker and so can act as the broker (residual 3 below, plan re-review I-1). Closing that route would mean the broker is deployed by someone other than the pipeline | 30, 31 |

### Owner inputs, and the variable each one fills

From survey §7 (`:519-557`). None is in source; each arrives as a pipeline variable or `TF_VAR_*`.

| Input | Fills | Needed before |
|---|---|---|
| **One subscription per environment** (dev and prod never share one: a subscription carries one `panacea-app-estate` value, so a shared one can never pass the prod guard, plan re-review b03 M-6), each with its id, tenant id and region (recommended `australiaeast`, three zones) | `ARM_SUBSCRIPTION_ID`, `ARM_TENANT_ID`, `var.location`; `PANACEA_APP_<ENV>_SUBSCRIPTION_ID` | first `plan` |
| The tag `panacea-app-estate=<env>` on each handed-over subscription, set by the owner (task 31 §1) | nothing in code: the estate guard reads it on every Terraform path and every workstation section, and refuses a subscription without it, so a Panacea subscription is never a Terraform target (task 30, review I3). Stages 4–5 and the image push do not read it; they rely on stage 1's read and their resource-scoped roles (task 30's stated residual, plan re-review b03 M-3) | bootstrap (task 31 §2) |
| Kata vCPU quota: max sandbox nodes × the sandbox SKU's vCPUs (8 for dev's `Standard_D8s_v5`; 16 or 32 if task 32's P07 moves the pool to `Standard_D16s_v5` or `D32s_v5`, P8-D6's ceiling, review I5), plus system and apps pools | `var.sandbox_pool.max_count`, `var.apps_pool` | first `apply` |
| **Four pipeline identities per environment** (plan, infra, cluster, push), created first with no role (task 31 §1a), outside `rg-panacea-app-<env>`, then each granted exactly its row of "Deploy identities" below as its scopes come to exist (§2a, §4a). The infra identity's role-assignment right is **scoped by an ABAC condition** to the five roles task 10 names for it, and excludes its own object id as a principal | `TF_VAR_state_writer_object_ids`, `TF_VAR_state_reader_object_ids` (task 02); `PANACEA_APP_<ENV>_CLUSTER_PRINCIPAL_OBJECT_ID`, `PANACEA_APP_<ENV>_PUSH_PRINCIPAL_OBJECT_ID` for the owner's `grant-pipeline-roles.sh` (task 30) | bootstrap (task 31 §2) |
| The three custom role definitions, `panacea-app-cluster-deployer`, `panacea-app-sandboxes-deployer` and `panacea-app-plan-reader`, created from `infra/rbac/*.json` (task 30). No pipeline identity can write role definitions | the role names `grant-pipeline-roles.sh` assigns, and the plan identity's third role | task 31 §2a |
| The owner's own grants: the cluster and push identities' roles, by `infra/rbac/grant-pipeline-roles.sh` after the first `foundation` apply; the prod Postgres server's `CanNotDelete` lock after the first prod `data` apply; the Deny policy on AKS local accounts (plan re-review I-2, I-4) | nothing in Terraform | task 31 §4a, §6a, §2a |
| An ADO project; **four** OIDC (workload identity federation) service connections per environment, with branch control and per-pipeline permissions on the three write connections (task 30's table); paid parallel jobs; and the right to create a Managed DevOps Pool | `PANACEA_APP_<ENV>_{PLAN,INFRA,CLUSTER,PUSH}_SERVICE_CONNECTION`, task 30's pool | first `plan` |
| Entra: the identity stack is **owner-run only**, applied by the owner with their own credentials, **or** the registrations are made by hand and their ids handed over. No pipeline identity holds Graph application rights (review I8) | `stacks/identity` run by the owner, or `TF_VAR_*_client_id` | first `apply` of `workloads` |
| Entra groups: cluster admins, Postgres admins, product users. **Humans only**: no pipeline identity is a member of the cluster admin or Postgres admin groups (task 31 §2a checks) | `var.cluster_admin_group_object_ids`, `var.postgres_admin_group`, `var.user_group_object_id` | first `apply` |
| A human member of the Postgres admin group to run the database bootstrap once per environment (task 31 §7) | nothing in code | first `deploy` with `deployCharts: true` |
| A DNS name and its delegation from the parent zone | `var.dns_zone_name`; the NS records from `foundation`'s output go to the parent | first `apply` of `edge` |
| A VNet range that does not overlap the corporate network | `var.vnet_cidr` (default `10.40.0.0/16`, a placeholder) | first `apply` |
| A monthly budget and alert recipients | `var.monthly_budget_aud`, `var.budget_contact_emails` | first `apply` |
| Confirmation that users' own Claude subscriptions may be used from Azure egress IPs (`PROGRAM.md` open risk 2) | nothing in code — a precondition of the first real session | task 32 |

### Deploy identities: every principal, what it holds, at what scope (review C3, I8)

The roles below are **held** roles. The owner grants them, and no stack does: the state roles
through the operator-run `bootstrap` (task 02), the cluster and push roles through the owner's
`infra/rbac/grant-pipeline-roles.sh` (task 30), and the rest by hand (task 31 §2a). Task 10's
allowlist is a different list: the roles a stack **assigns** to the pods' and the cluster's own
identities. It holds no AKS data-plane role and no AcrPush (plan re-review I-2). Each environment has its own
four pipeline identities, and none is shared between dev and prod. The ADO checks live outside
the repository, so a pull request cannot edit them (task 30). The guards inside the repository
are hygiene, not a boundary.

| Principal | Holds | Scope | Used by | ADO checks | Cannot |
|---|---|---|---|---|---|
| **plan** | Reader; Storage Blob Data Reader; `panacea-app-plan-reader` (`listClusterUserCredential` only, task 30) | the subscription; the state container; `rg-panacea-app-<env>` | `infra-plan` (PR validation and manual), `-lock=false`; `deploy`'s outputs-only job | pipeline permission `infra-plan`, `deploy`. No branch control: PR builds use it | write anything in Azure; take a state lease; read a Key Vault secret or Graph; use the cluster (its kubeconfig holds no secret and it holds no AKS data-plane role) |
| **infra** | Contributor; Role Based Access Control Administrator with the ABAC condition (the five roles task 10 names, never itself as principal); Storage Blob Data Contributor; Cost Management Contributor | `rg-panacea-app-<env>` (created by `bootstrap`, task 02); the same group; the state container; the subscription (the budget, task 04) | `deploy` stage 3 | pipeline permission `deploy` only; branch control `main` (prod), `main`/`dev` (dev); the Environment approval | grant any AKS data-plane role or AcrPush (none is on its ABAC list); read the Hub's data **as a database administrator** (not in the admin group); touch Entra applications (no Graph right); create or lift a management lock. **It can** act as any identity it deploys, and reach the cluster through the cluster resource it manages (residual 1) |
| **cluster** | Azure Kubernetes Service Cluster User Role; Azure Kubernetes Service RBAC Admin; `panacea-app-cluster-deployer`; `panacea-app-sandboxes-deployer` | the cluster; namespaces `default`, `panacea-edge`, `panacea-broker`, `panacea-hub`; the cluster (cluster-scoped kinds only); namespace `panacea-sandboxes` (never `secrets` or `pods` as a resource; `escalate` and `bind` on `roles`, for the broker's Role) | `deploy` stages 4–5 | as infra | change Azure RBAC or resources; read or write state. **It can** read every user's token Secret: it deploys the broker and so can act as the broker (residual 3) |
| **push** | AcrPush | the environment's registry | `images.yml` `push` (task 27) | pipeline permission `images` only; branch control as infra | anything but push images; get an image deployed (task 30 refuses digests from another branch or registry) |
| **owner** (a human) | subscription owner, or User Access Administrator; the Entra rights for `stacks/identity` | the subscription; the tenant | task 31 §0–§2a, §4a (`grant-pipeline-roles.sh`), §6a (prod's database lock); `stacks/identity`, owner-run only (task 09) | — | — |
| **Postgres admin** (a human, in `var.postgres_admin_group`) | the server's Entra administrator group | the Hub's server | the database bootstrap, task 31 §7, once per environment | — | — |
| **cluster admin** (a human, in `var.cluster_admin_group_object_ids`) | cluster admin, through the AKS Entra integration | the cluster | break-glass; and the human smoke that creates a pod in `panacea-sandboxes` (task 31 §8's `run.sh --require-kata`, task 32's P02 and P08), which no pipeline identity may do | — | — |

**What the split still leaves open (declared, not fixed):**
1. **The infra identity can act as anything it deploys** (plan re-review I-2). Its role-assignment
   route to the cluster is closed: the ABAC list holds no AKS data-plane role and no AcrPush, so
   granting one to an identity it creates is refused (an ABAC condition limits role and
   principal, not scope, **training knowledge, UNVERIFIED**, which is why the list itself had to
   shrink). Three routes remain, none of them through the allowlist:
   - **Federated credentials.** It creates the Hub's, the migration Job's and cert-manager's
     identities (task 10), so it can make one trust an issuer it controls, then sign in to
     Postgres as the Hub or write the product zone as cert-manager. (Contributor covering
     `federatedIdentityCredentials/write` is **training knowledge, UNVERIFIED**.) The pipeline
     identities themselves live outside its group (task 31 §1a), so it cannot do this to them.
   - **The allowlisted roles it may still grant**: `Key Vault Secrets User` to an identity it
     creates reads the vault's secrets, and Contributor over the vault may let it change the
     vault's own access model (**training knowledge, UNVERIFIED**).
   - **The cluster resource.** Contributor on the cluster can turn local accounts back on and
     then list the admin credential, which is cluster-admin, which is every user's token. The
     owner's Deny policy on AKS local accounts (task 31 §2a) blocks that update, and the infra
     identity cannot remove a policy assignment (Contributor excludes
     `Microsoft.Authorization/*/Write`). **Both are training knowledge, UNVERIFIED.** AKS run
     command is the same kind of route, and whether a cluster with local accounts off runs it
     with the caller's own rights is **UNVERIFIED**. Task 31 §4a probes both routes.

   What bounds all three is the infra connection's branch control and the Environment approval.
   Whoever deploys an identity or a cluster can act as it: this is inherent in one pipeline
   owning the estate, and it is declared here rather than hidden.
2. **The cluster identity may be able to bind `cluster-admin` to itself.** The upstream
   cert-manager and Traefik charts bring ClusterRoles and ClusterRoleBindings. Creating those may
   need Kubernetes RBAC `bind`, which would allow that self-binding (task 30's
   `cluster-kinds_test.sh` prints the list; training knowledge, UNVERIFIED). The recommended
   build keeps those objects in the pipeline's releases, so `deploy` works end to end, and relies
   on the ADO checks. The stricter option is owner question **Q4** (the open questions above).
3. **The cluster identity can read every user's token Secret** (plan re-review I-1). It deploys
   the broker, and RBAC Admin on `panacea-broker` lets it run a pod as the broker's service
   account, which can `get` every Secret in `panacea-sandboxes` (task 29's Role). Installing that
   Role also needs `escalate` and `bind` on `roles` there, and those let it write itself a Role
   that reads the Secrets directly (task 30's `cluster-kinds_test.sh` prints the verbs; **training
   knowledge of Kubernetes RBAC under the AKS webhook, UNVERIFIED**). Keeping RBAC Admin off
   `panacea-sandboxes` removes a direct grant, not this route. What bounds it is the cluster
   connection's ADO checks, the same boundary as residual 2. Q4 does not change it.
4. **Reader may be too little for some refreshes.** `panacea-app-plan-reader` starts with the one
   action the AKS read needs (task 30, plan re-review M-2). Task 30 adds only actions that return
   no secret; task 31 §6 records the 403s.
5. **A PR build runs inside the VNet** (plan re-review M-3). `infra-plan` runs PR validation on
   the Managed DevOps Pool in `snet-agents` (task 30), so a PR's code has a network position next
   to the Postgres, Key Vault and registry private endpoints and the node subnets. It holds no
   write credential and no data-plane role, so what it can reach is what answers without one.
   **Deferred, with its reason:** the fix is an NSG on `private-endpoints` that admits
   `snet-agents` only on the ports `deploy` needs (443), plus private-endpoint network policies
   on that subnet, or a separate pool for PR validation. Both are network changes to task 03 and
   task 30's pool, outside this identity batch; they belong with the network review items.
6. **Contributor on one resource group may be too little for AKS.** Creating a cluster whose node
   resource group is `rg-panacea-app-<env>-nodes` may require the caller to be allowed to create
   that group. **UNVERIFIED**: the AKS resource provider may create it on its own authority. If
   task 31's first `foundation` apply fails for this reason, the owner widens the infra identity's
   Contributor to the subscription and records it as a widening in the results file. A role
   cannot be scoped to a group that does not yet exist.


---

## File map

```
infra/terraform/
  kata.tf providers.tf variables.tf dev.tfvars .terraform.lock.hcl   DELETED (01): the pool moves into modules/aks
  README.md                                   stack order, commands, the rules                     01
  backends/dev.hcl  backends/prod.hcl        partial backend config, no ids                        02
  role-allowlist.txt                          the roles any stack may assign                       10
  modules/network/            VNet, subnets, NSGs, NAT gateway, private DNS zones                  03
  modules/observability/      Log Analytics, DCR, action group, budget                             04
  modules/registry/           ACR Premium, private endpoint, retention                             05
  modules/keyvault/           Key Vault (RBAC), private endpoint                                   05
  modules/aks/                cluster, system/apps/sandbox (Kata) pools, diagnostics               06
  modules/egress-firewall/    Azure Firewall + policy + the agent allowlist (egress_mode=firewall) 07
  modules/postgres/           Flexible Server, Entra-only, private endpoint                        08
  modules/workload-identity/  one UAMI + federated credential + role assignments                   10
  modules/ci-pool/            Dev Center project + Managed DevOps Pool in the agents subnet        30
  stacks/bootstrap/           state RG, storage account, container                                 02
  stacks/foundation/          composition of network … aks, DNS zone, edge IP + A records, checks  07, 30
  stacks/data/                Postgres for the Hub                                                 08
  stacks/identity/            azuread: desktop client, Hub API, broker API (Hub contract §6.5)     09
  stacks/workloads/           id-hub, id-hub-migrate, id-cert-manager, role assignments            10
  */tests/*.tftest.hcl        terraform test, mock providers only                          every task
infra/scripts/validate.sh  guards.sh  guards_test.sh   fmt, init -backend=false, validate, test    01
infra/scripts/role-allowlist.sh                                                                    10
infra/smoke/                  post-apply smoke, written and tested on canned output, NOT run       31
infra/smoke/proving/          the Kata proving scripts, NOT run                                    32
deploy/charts/platform/       namespaces, PSA, priority classes, quotas, policies, storage class   11
                              + warm capacity (balloons, pre-pull)                                 12
deploy/charts/cert-manager/   upstream cert-manager, its WI service account, its NetworkPolicy     13
deploy/charts/edge/           Traefik (Gateway API), ClusterIssuer, Gateway, edge NetworkPolicy    13
deploy/tests/netpolcover/     cross-chart render test: no workload sealed by default-deny          13
deploy/charts/broker/         broker Deployment, SA, Role in panacea-sandboxes, HTTPRoute, PDB     29
deploy/charts/hub/            Hub Deployment, migrate Job, SA (WI), HTTPRoute, PDB (contract §12)  29
deploy/sql/hub-principals.sql.tpl   the Hub identities as Postgres principals, bound by object id  29
pipelines/scripts/render-sql.sh     fills the SQL template from workloads outputs; refuses a gap   29
pipelines/scripts/lib/where.sh      is_pipeline: one pipeline-or-workstation rule for two scripts  30
deploy/values/{dev,prod}/*.yaml     per-environment values; ids injected by the pipeline           29
control-plane/internal/sandbox/manager.go      config, hardening, token Secret, machine mounts; the control-plane label (18)   14, 15, 18
control-plane/internal/sandbox/machine.go      a machine's disk, token, pod and PDB                19
control-plane/internal/backend/backend.go      KubeLabelOwner; MachineMount, Spec.Machine; KubeLabelControlPlane, Spec.ControlPlane, the MaxMessageBytes comment (18)   14, 15, 18
control-plane/internal/backend/ready.go        ReadyWaiter, ErrQuotaExceeded, ErrMachineFull       22
control-plane/internal/backend/katabe/kata.go  owner scope, WorkDir, machine mounts; ControlPlane passed through (18)   14, 15, 18
control-plane/internal/backend/dockerbe/docker.go  refuses MachineMounts                           15
control-plane/internal/backend/remotebe/       Client and Backend over the broker                  22
control-plane/internal/entra/                  Entra token validation (Hub contract §6.3)          16
control-plane/internal/brokerapi/              codes (17), sandbox wire (18), machine wire (19)    17-19
control-plane/cmd/broker/                      the broker                                          17-19
control-plane/internal/machineapi/             machine-agent wire: catalog (20), git (21)          20, 21
control-plane/cmd/machined/                    the machine agent                                   20, 21, 25
control-plane/internal/machines/               the machine set and InstallID (23); agent calls (24b, 25)   23, 24b, 25
control-plane/internal/api/{api,sessions,attach}.go   route by machine; the machine routes (24a); cloud create (24b)   23, 24a, 24b
control-plane/internal/api/machines.go         cloud machines: list, ensure-default, wake          24a
control-plane/internal/api/cloudstore.go       the cloud conversation store                        24b
control-plane/internal/api/catalog.go          handleMachines moves out (24a); machineId on the catalog routes (24b)   24a, 24b
control-plane/internal/httpx/problem.go        machine_not_found; quota_exceeded, machine_full     23, 24a, 24b
control-plane/internal/reaper/reaper.go        sweeps per machine; rule 4 on a cloud machine acts on its own install id only (review I6)   23
control-plane/cmd/api/{config,main}.go         RUNTIME_CLASS (14); BROKER_URL, BROKER_SCOPE (23)   14, 23
control-plane/internal/gitx/remote.go          phase 3's Machine for a cloud machine (S3-P8.1)     25
desktop/src/{store,api}/…, components/rail/{MachineChip,MachinesPopover}.tsx   cloud machines      26
docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md §2   amended with types.ts            26
images/{broker,hub,machine}/Dockerfile  images/build.sh  images/images_test.sh                    27 (+27a)
images/sandbox/Dockerfile                      the base off Node 20, by digest (OD-8 pin moves)    27a
azure-pipelines.yml                             the Go pipeline: PR trigger, path filter, govulncheck 28
pipelines/templates/tools.yml  pipelines/scripts/{install-tool,lint,chart-licences,chart-licences_test}.sh  28 (+27, 30)
pipelines/scripts/testdata/chart-licences/<case>/   the eight fixture trees chart-licences_test.sh runs  28
pipelines/{desktop,licences,secrets,infra,charts}.yml  licences/  .gitleaks.toml                  28 (+10, 13)
pipelines/images.yml                                                                               27
pipelines/{infra-plan,deploy}.yml  pipelines/scripts/{estate-guard,tf-stack,helm-values}.sh  written, NOT registered  30
pipelines/scripts/cluster-kinds_test.sh  infra/rbac/{cluster,sandboxes}-deployer.json   the cluster identity's roles (C3/I8)  30
infra/rbac/plan-reader.json  infra/rbac/grant-pipeline-roles{,_test}.sh   the plan role; the owner's cluster/push grants  30
docs/runbooks/phase-8-first-apply.md  docs/verification/phase-8-first-apply-results.md            31
docs/verification/phase-8-kata-proving.md                                                          32
docs/SEAMS.md  docs/PROGRAM.md  docs/superpowers/planning/env/cost-model.md  docs/verification/phase-8-results.md   33
```

**Shared files, and the order they are touched in.** A task that finds an earlier task's change
missing stops and reports it.

| File | Tasks, in order |
|---|---|
| `control-plane/go.mod` | none — this phase adds no dependency to it |
| `go.work` | 13 only (`use ./deploy/tests/netpolcover`, a test-only module that ships in no image) |
| `deploy/tests/netpolcover/*` | 13 → 29 (29 adds `broker` and `hub`, deletes the stand-in `testdata/backends.yaml`, and adds `session_size_is_one_value`; 13 also owns `every_sandbox_container_is_sized`, review I4) |
| `pipelines/charts.yml` | 28 → 13 (13 adds the cross-chart `netpolcover` job) |
| `licences/charts.yaml` | 28 → 13 (28 writes it empty with `chart-licences.sh`; 13 adds a row per `Chart.lock` entry, review m6) |
| `pipelines/infra.yml` | 28 → 10 (10 adds the `role-allowlist.sh` step it writes the script for; review m2) |
| `internal/sandbox/manager.go` | 14 → 15 → 18 (and phase 3's task 05, which lands first — task 14 step 0) |
| `internal/backend/backend.go` | 14 → 15 → 18 (18 adds `KubeLabelControlPlane` and `Spec.ControlPlane`, and updates the `MaxMessageBytes` comment at `:29-37` to name five sites; and phase 3's task 05) |
| `internal/backend/katabe/kata.go` | 14 → 15 → 18 |
| `internal/api/{sessions,attach,api}.go` | 23 → 24a (`api.go`: the machine routes) → 24b (`sessions.go`: cloud create) |
| `internal/api/catalog.go` | 24a (`handleMachines` moves out) → 24b (`machineId`) |
| `internal/httpx/problem.go` | 23 → 24a (`quota_exceeded`) → 24b (`machine_full`) |
| `cmd/broker/server.go`, `internal/brokerapi/wire.go` | 17 → 18 → 19 |
| `cmd/machined/*`, `internal/machineapi/wire.go` | 20 → 21 → 25 |
| `internal/machines/agent.go` | 24b → 25 |
| `stacks/foundation/*` | 07 → 30 |
| `deploy/charts/platform/*` | 11 → 12 |
| `pipelines/templates/tools.yml`, `pipelines/scripts/lint.sh` | 28 → 27 → 30 |
| `images/machine/Dockerfile`, `images/images_test.sh` | 27 → 27a (27a moves the git source with the base and adds four rows, review m15) |

---

## Task list

Every part is in `2026-09-23-phase-8-own-environment-parts/task-NN.md`. Three ids carry a letter
so that no other number moved (review m15, m17): `task-24a.md` and `task-24b.md` are the two
halves of what was task 24, and `task-27a.md` is the base-image task inserted after 27. Hours are
for one implementer who has read this file and the part, and assume the part's tests are written
first.

### Group A — Terraform, written and validated (no Azure call)

| # | Task | Depends on | Hours |
|---|---|---|---|
| 01 | The `infra/terraform` layout, the conventions, `validate.sh` and `guards.sh`; the phase 1 root (which targets Panacea's cluster) is retired | — | 2 |
| 02 | `stacks/bootstrap`: the state storage account, Entra-only, versioned; partial backend files | 01 | 1.5 |
| 03 | `modules/network`: VNet, five subnets (six in firewall mode, which adds `AzureFirewallSubnet`), NSGs, NAT gateway, the privatelink DNS zones | 01 | 2.5 |
| 04 | `modules/observability`: Log Analytics with a daily cap, the Container Insights DCR, action group, budget | 01 | 2 |
| 05 | `modules/registry` and `modules/keyvault`: ACR Premium and Key Vault, private endpoints, no admin user, RBAC only | 01, 03 | 2 |
| 06 | `modules/aks`: the cluster (Overlay + Cilium, Entra RBAC, local accounts off, WI) and three pools, the Kata pool among them | 01, 03 | 3 |
| 07 | `stacks/foundation`: the composition, `modules/egress-firewall`, the DNS zone, the edge IP and its A records, role assignments, checks, `dev`/`prod` tfvars | 02–06 | 5 (review m17: a module plus a nine-file stack whose `terraform test` runs take minutes each, once per tfvars) |
| 08 | `modules/postgres` and `stacks/data`: Flexible Server for the Hub, Entra-only | 07 | 2 |
| 09 | `stacks/identity`: the Entra registrations and groups (HA-3) | 01 | 2.5 |
| 10 | `modules/workload-identity` and `stacks/workloads`: the pods' identities and their grants; the `role-allowlist.sh` step in `pipelines/infra.yml` | 07, 08, 09, 28 | 2 |

### Group B — the cluster's own objects, as charts

| # | Task | Depends on | Hours |
|---|---|---|---|
| 11 | `deploy/charts/platform`: namespaces, Pod Security, priority classes, quotas, network policies | 06 (names only) | 3 |
| 12 | Warm capacity in `platform`: Kata balloons and the image pre-pull DaemonSet | 11 | 2 |
| 13 | `deploy/charts/cert-manager` (its own release) and `deploy/charts/edge`: Traefik on Gateway API, the ClusterIssuer, the Gateway, both charts' NetworkPolicies, the cross-chart `netpolcover` test | 11, 28 | 3.5 |

### Group C — Kata, for real

| # | Task | Depends on | Hours |
|---|---|---|---|
| 14 | Sandbox pod hardening: RuntimeClass as config (S8.4), no SA token, seccomp, the attach token in a Secret, owner labels | — | 2.5 |
| 15 | Machine volumes: `backend.Spec.Machine` + `MachineMounts`, WorkDir, pod affinity; S8.3's Kata half | 14 | 3 |

### Group D — the broker

| # | Task | Depends on | Hours |
|---|---|---|---|
| 16 | `internal/entra`: Entra access-token validation (HA-1) | — | 2.5 |
| 17 | `cmd/broker`: config, listener, authentication, health, problems | 16 | 2 |
| 18 | Broker sandboxes: the `backend.Backend` surface over HTTP and the attach relay, owner-scoped; each sandbox labelled with its creator's install id (review I6) | 15, 17 | 3.5 |
| 19 | Broker machines: disk + machine pod + token, ensure-default, wake, sleep, delete, the idle sleeper, the agent proxy; owner-prefixed names, a derived default id, re-counted caps and the sleeper's Lease, so two replicas never race (review m11, m12) | 18 | 4 |

### Group E — the machine agent

| # | Task | Depends on | Hours |
|---|---|---|---|
| 20 | `cmd/machined`: the catalog on the machine disk, store deletion (OD-10), authentication | 15 | 2.5 |
| 21 | `machined` git: phase 3's runner with its §HGH hardening, an explicit `Env` allowlist derived from phase 3's call sites, no token in a lane, a scrubbed environment, a credentialed add-project clone, and phase 3's lane-create clone confined to `lanes/` (review I7, b06 re-review) | 20, **phase 3 task 02 with §HGH** | 3.5 |

### Group F — the laptop's control plane serves cloud machines

| # | Task | Depends on | Hours |
|---|---|---|---|
| 22 | `remotebe`: `backend.Backend` over the broker, with the Hub's token source (HA-2) | 18 | 3 |
| 23 | `internal/machines`: a backend per machine through sessions, attach, reaper and `main`; the install id, and a reaper that destroys only its own cloud sandboxes (review I6) | 22 | 3.5 |
| 24a | Cloud machines in `GET /v1/machines`, `POST /v1/machines/default` (ensure-default) and `POST /v1/machines/{machineId}/wake`; the list cache and its invalidation (review m17 split task 24) | 23 | 2 |
| 24b | Machine-scoped catalog, session create on a cloud machine with wake-before-create and `.git/config` bound read-only (phase 3 H5), the cloud conversation store (`cloudstore`, OD-10); closes S8.3 | 24a, 20 | 3 |
| 25 | Phase 3's `gitx.Machine` for a cloud machine (S3-P8.1) | 24b, 21, **phase 3 tasks 02–04 and 11** | 2.5 |
| 26 | The cockpit shows cloud machines: states, implicit wake, the isolation badge (S8.5), signed-out | 24b (and so 24a) | 3 |

### Group G — packaging and CI/CD

| # | Task | Depends on | Hours |
|---|---|---|---|
| 27 | `images/broker`, `images/machine`, and the `images` pipeline: digest, SBOM, scan, the OD-8 canary | 17, 20, 28 | 2.5 |
| 27a | The sandbox base off end-of-life Node 20, by digest, before its first publish; it moves the OD-8 pin, so it has its own canary run and commit (review m15) | 27 | 1.5 |
| 28 | Pipelines: tool templates, the Go pipeline, `desktop`, `licences` (OD-11, repo-wide), `secrets`, `infra`, `charts` | 01 | 3 |
| 29 | `deploy/charts/broker` and `deploy/charts/hub` (HA-4, HA-5), with per-environment values | 11, 13, 19, 27 | 3 |
| 30 | `infra-plan` and `deploy` pipelines, the estate guard, `modules/ci-pool` — written, not registered | 07, 28, 29 | 3 |

### Group H — owner-gated: written now, executable only when the environment exists

| # | Task | Depends on | Hours to write |
|---|---|---|---|
| 31 | The first-apply runbook and the smoke scripts | 07–13, 27a, 29, 30 | 2 |
| 32 | The Kata proving checklist: RuntimeClass, disk sharing, cold start, overhead, isolation | 31 | 2 |
| 33 | Close the phase: the S8 seams, the compliance gap, the cost model, what stays unverified | all | 1.5 |

### Order

The graph allows ten waves. Tasks inside a wave share no file (see "Shared files" above), and no
task sits in the same wave as, or an earlier wave than, a task it depends on (review m2: 28 was in
wave 1 beside 01, which it depends on, and 32 and 33 were in wave 8 beside 31). Task 11's
dependency on 06 is marked "names only" in the task list (a chart that renders against no cluster
needs 06's names, such as the `kata-vm-isolation` RuntimeClass, not its code), so 11 may run
before 06.

```
wave 1   01  14  16
wave 2   02  03  04  09  11  15  17  28
wave 3   05  06  12  13  18  20
wave 4   07  19  22  27
wave 5   08  23  27a  29     21 (when phase 3 task 02 has landed)
wave 6   10  24a  30
wave 7   24b
wave 8   25 (when phase 3 tasks 02–04 and 11 have landed)  26  31 (owner-gated, OD-12)
wave 9   32                  (owner-gated, OD-12; depends on 31)
wave 10  33                  (depends on every task)
```

**The critical path** is 14 → 15 → 18 → 19 and 16 → 17 → 18 → 22 → 23 → 24a → 24b → 26: the cloud machine
end to end. The Terraform groups can finish in parallel with it and gate nothing in Go.

---

## Seams

### Seams this phase consumes (phase 2 task 26, `task-26.md:936-994`; phase 3's plan)

| Seam | What this phase does | Task |
|---|---|---|
| **S8.1** `backend.Backend`, seven methods | `remotebe` implements it; the interface does not change | 22 |
| **S8.2** kata → in-cluster client only; `PANACEA_APP_KUBECONFIG` refused | **Unchanged.** The laptop never holds a cluster credential (P8-D4); only the broker builds an in-cluster client, and it is a different binary | 17 |
| **S8.3** create sends a host mount and `WorkDir`; `katabe` refuses both | Resolved the right way round: a cloud-machine create sends `MachineMounts`, never `Mounts`; `katabe` keeps refusing host mounts and **now honours `WorkDir`**. `TestCreateRejectsMountsAndCreatesNoPod` stays green; `TestCreateRejectsAWorkDir` is replaced, in the same commit, by a test that `WorkDir` becomes the container's `workingDir` | 15, 24b |
| **S8.4** `runtimeClass` hardcoded | Becomes `PANACEA_APP_RUNTIME_CLASS` (P8-D15) | 14 |
| **S8.5** `isolationFor` → `microvm` for Kata | A cloud machine reports `microvm`; the rail drops the `unsandboxed` badge for it (`MachineChip.tsx:45`) | 24a, 26 |
| **S8.6** no `Authorization` header in `desktop/src/api/client.ts` | **Unchanged on the desktop hop.** The token travels control plane → broker, never webview → control plane; `client.ts:6-8`'s comment is updated to say where the auth went | 22, 26 |
| **S8.7** `proxy.RelayHooked`, the one relay | The broker relays laptop ↔ pod through it; no second relay is written | 18 |
| **SX.10** the conversation store outlives its sandbox | On a cloud machine the store is `conversations/<worktree id>/` on the machine disk, mounted at `/home/dev/.claude/projects` | 15, 24b |
| **S3-P8.1** `gitx.Machine` | A cloud machine's `Machine`: `Git()` runs on `machined` through the broker; `CloneRoot()` is `/machine/lanes`; `FetchURL` is the path, unchanged, because clone and repo share one disk | 25 |
| **S3-P8.2** `lanes.Store` location | Cloud clones live under `/machine/lanes` on the machine disk, behind S3-P8.1 | 21, 25 |

If phase 2 task 26 has not landed when a task here starts, the S-ids above are this plan's names
for the same code locations, each cited to `file:line` in its part.

### Seams this phase leaves for later phases

| Seam | Where | The later phase | Must not |
|---|---|---|---|
| **S8-P6.1** Claude credentials in a cloud session | `~/.claude` in a session pod is an `emptyDir` (`manager.go:175-187`); only `~/.claude/projects` is on the disk (SX.10), so `/login` is per session | Phase 6's credential vault supplies the **user's own** credentials to that user's pods | Put credentials on the machine disk beside transcripts; pool or share them across users; bake them into an image |
| **S8-P9.1** Customer-managed keys, etcd KMS, Defender, Sentinel, WAF | `modules/aks` takes `disk_encryption_set_id = null`; no `key_management_service` block; no WAF in `edge` | Phase 9 (enterprise hardening) adds each as a variable of the module it belongs to | Add a posture flag that defaults a create-time field off (P8-D2) |
| **S8-P9.2** Defender for Containers cannot assess Kata pods | recorded as a compliance gap in `PROGRAM.md` (task 33), as its cross-cutting constraints ask | Phase 9's ISM evidence | Claim coverage from a subscription-level Defender plan |
| **S8-H.1** The Hub's chart values | `deploy/charts/hub/values.yaml`, built against HA-4/HA-5 | The Hub phase, when its contract lands, reconciles the chart | Hard-code a Hub setting in the chart that is not in the Hub's config table |
| **S8-C.1** Customer-hosted tier | every stack takes `environment`, `location`, tenant and subscription from variables; `identity` is separable | A later phase runs the same stacks in a customer's subscription and Entra tenant | Hard-code our tenant, zone or registry name in a module |
| **S8-I.1** The sandbox image's base | `ARG NODE_BASE` in `images/sandbox/Dockerfile`, by digest (task 27a, review m15); the machine image is `FROM` the sandbox image | Every later phase: at least six months before the base's Node line reaches end of life (not when it enters maintenance, plan re-review b08-1 issue 4), a task of its own moves it, runs the OD-8 canary and names the old and new digests | Move the base inside another change: its digest is OD-8's adapter pin |
| **S8-D6.1** Machine-disk data protection (review m13) | `StorageClass/panacea-machine-disk` is `reclaimPolicy: Delete` with no `VolumeSnapshotClass` and no schedule (task 11); AKS's snapshot controller is on and unused (task 06). The disk is the only copy of a machine's worktrees, unpushed lane commits and conversations | The task that follows the owner's answer to the b09 question: a `VolumeSnapshotClass` on `disk.csi.azure.com` with `incremental: "true"` in the platform chart; a daily `CronJob` in the broker chart with its **own** service account and a `Role` in `panacea-sandboxes` (`persistentvolumeclaims` `list`; `snapshot.storage.k8s.io` `volumesnapshots` `create`, `list`, `delete`), never the broker's; snapshots named and labelled like their claim (owner, machine id); retention the owner's number; a missing claim's snapshots are deleted **only when they age out** at that retention, like any other, because the CronJob can tell a missing claim only by its absence, and an explicit delete and a claim lost to a broker defect look the same, while the defect is the loss the snapshots exist for (b09 re-review 6); `cluster-deployer.json` gains the class's kind (task 30's `cluster-kinds`); a restore runbook (task 33) and a task 32 probe that restores one | Snapshot a disk while OD-10 says Forget deletes, unless the owner has accepted the retention window in writing; give the broker's own Role snapshot rights; delete a snapshot early because its claim is gone, or keep one past the retention |
| **S8-P13.1** The Panacea lift (OD-13) | the `apps` pool and the `panacea-*` namespace convention | The lift phase lands Panacea's services beside the Hub | Pre-create namespaces or pools for it now |

---

## What this plan does not claim

- **Nothing about Azure.** No stack has been planned or applied. Every statement about how AKS Pod
  Sandboxing behaves — the RuntimeClass name, whether two Kata pods on one node can share an RWO
  disk, per-pod Kata overhead, allocatable CPU on `Standard_D8s_v5` (and so how many sessions one
  machine holds, P8-D6's ceiling), cold-start time on a warm
  node, whether `pods` in a Kata sandbox mount CSI volumes at all — is **UNVERIFIED** and is task
  32's to prove.
- **The Hub's code.** The Hub's shapes are taken from ADR-H1 and its contract as read on
  2026-09-23 (both still being written — the contract grew from absent to 2,289 lines during this
  plan); none of the Hub exists in this branch, and tasks 09, 16, 22 and 29 re-read the contract
  before they start. The Hub contract's own list of what it did not verify (`:2276-2288`) applies
  to everything this plan takes from it.
- **Phase 3's code.** Tasks 21 and 25 build on `gitx.Runner` and `gitx.Machine` as phase 3's task 02
  part defines them (`phase-3` worktree, `2026-09-23-phase-3-git-convergence-parts/task-02.md:85`,
  `:150-159`); none of it exists in this branch.
- **Prices.** The cost model (task 33) carries the survey's order-of-magnitude figures, which quote
  Panacea's in-code estimates (survey §6, `:480-515`); no price was fetched.
- **Owner-observed behaviour.** Anything the owner must watch — a cloud session in the window, the
  rail's machine states — stays unverified until the batched hands-on session (OD-12).
- **Training-knowledge facts**, each flagged where used: the GA RuntimeClass name, the ingress-nginx
  retirement, Managed DevOps Pools' availability in KPMG's ADO organisation.

