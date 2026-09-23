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
| **OD-8** | The transcript adapter is pinned to the CLI version in the sandbox image, so the **image digest is the adapter pin**: the `images` pipeline runs the adapter's canary against the image it is about to push | 27 |
| **OD-10** | Forget deletes the transcript; removing a worktree deletes its store. On a cloud machine both are paths on the machine disk, deleted by the machine agent | 20 |
| **OD-11 rule 3** | Dependencies are MIT, Apache-2.0, BSD or ISC, checked in CI. This plan applies the gate **repo-wide**, including Terraform providers and Helm chart dependencies (MPL-2.0 providers are tools, not linked code — see task 28's allowlist) | 28 |
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
| **P8-D6** | **A cloud machine is one Azure Disk (RWO, `managed-csi-premium`) plus one machine pod.** Every session pod of that machine mounts **sub-paths** of the same disk and is co-scheduled with the machine pod by **required pod affinity** on `panacea-app/machine-id`. Layout on the disk: `projects/`, `lanes/`, `conversations/<worktree id>/`. Sleeping = the disk with no pods | Clones hard-link objects only within one filesystem (OD-2); small-file I/O (`npm install` in a worktree) is far better on a block disk than on NFS. A machine bounded by one node is also what the word means. **Fallback**, if task 32 finds Kata cannot share one RWO disk between two pods on a node: Azure Files NFS (RWX), selected by `PANACEA_APP_BROKER_MACHINE_STORAGE_CLASS` with affinity off. **Kata support for either is UNVERIFIED** (survey G-5) |
| **P8-D7** | **The machine agent** (`control-plane/cmd/machined`) serves the catalog and the git runner **inside the machine pod**, on the pod network, behind a per-machine bearer token held in a Kubernetes Secret. Only the broker can reach it. **Nothing uses `pods/exec`** | Reuses `internal/project.Scanner` and phase 3's local git runner unchanged, on Linux paths. `pods/exec` would put git arguments (and, on a clone, credentials) into the API server's audit log and would need an SPDY dependency |
| **P8-D8** | **Images are built from binaries compiled outside Docker** (`CGO_ENABLED=0 GOOS=linux go build`) and copied into the image, and every image is referenced **by digest**. The machine image is `FROM` the sandbox image's digest plus the `machined` binary, so one image pin covers the CLI version (OD-8) | The corporate proxy breaks `go mod download` inside a container (`images/sandbox/Dockerfile:3-8`); the control-plane module is not vendored and should not become so to build two binaries |
| **P8-D9** | **Ingress is the Kubernetes Gateway API, implemented by Traefik**, with TLS from cert-manager (DNS-01 against our Azure DNS zone, workload identity). **Terraform owns the public IP and the A records**, so no DNS step is manual and no external-dns runs | Panacea's wildcard A record is a manual step after Traefik gets an IP (survey §1.5, `:124-125`). The AKS application-routing add-on is built on ingress-nginx, whose upstream announced its retirement — **from training knowledge, UNVERIFIED**; Gateway API resources keep the implementation swappable either way |
| **P8-D10** | **Postgres Flexible Server with Entra-only authentication** (`password_auth_enabled = false`), public access off, a private endpoint, and an owner-provided Entra group as its administrator. The Hub connects as its own user-assigned identity through workload identity. The Hub's database role is created by the `deploy` pipeline running as a member of the admin group — SQL, not Terraform | No database password exists to leak into state (survey §2.2, `:318`) |
| **P8-D11** | **No long-lived secret anywhere**: OIDC service connections in CI, workload identity in the cluster, Key Vault (RBAC mode, private endpoint) for whatever ADR-H1's Hub needs, read through the Secrets Store CSI add-on. Per-machine and per-sandbox bearer tokens are minted by the broker and live as Kubernetes Secrets owned by their pod, so they are garbage-collected with it | Survey §2.2 `:315-317` |
| **P8-D12** | **Sandbox egress** (dev): internet via the NAT gateway; **denied** to `169.254.169.254`, the pod and service CIDRs, the VNet, and every other sandbox; DNS to kube-dns only; ingress to a sandbox only from the broker on 8080 and to a machine pod only from the broker on 8081 | Kata isolates the kernel, not the network (survey G-4, `:420`) |
| **P8-D13** | **Azure DevOps is the CI of record**; one pipeline per concern under `pipelines/`, path-triggered, sharing pinned-tool templates. The root `azure-pipelines.yml` stays, as the Go pipeline, so the definition already registered against it keeps working. `infra-plan` and `deploy` are written and **not registered** until the environment exists | Survey §5 (`:437-476`); owner decisions `:56` |
| **P8-D14** | **Tool and provider pins.** Terraform `>= 1.9.0, < 2.0.0` in every stack, CI pinned to **1.15.8** (the version on the build laptop, `terraform version` on 2026-09-23); `hashicorp/azurerm` **4.81.0** (already locked, `infra/terraform/.terraform.lock.hcl:4-6`), `hashicorp/azuread` **3.9.0**. Each stack commits its own `.terraform.lock.hcl` | Every argument name this plan cites was checked against `terraform providers schema -json` for exactly these versions, in a scratch directory, on 2026-09-23 |
| **P8-D15** | **The RuntimeClass name becomes configuration**, `PANACEA_APP_RUNTIME_CLASS`, default `kata-vm-isolation`, and the first smoke check is `kubectl get runtimeclass` | The code hardcodes `kata-mshv-vm-isolation` (`manager.go:20`) against a pool created with `KataVmIsolation` (`infra/terraform/kata.tf:16`). Which name AKS installs is **UNVERIFIED** (survey G-3); the default is the GA name from training knowledge, and task 32 proves it |

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
`terraform fmt`, `terraform init -backend=false -input=false` and `terraform validate`, run by
`infra/scripts/validate.sh`. Tasks 30–32 describe Azure-touching steps as **runbooks** and scripts
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
`go.work` and both `go.mod` files — check `git diff` on them before every commit; `sandboxd/vendor/`
stays committed and **sandboxd gains no dependency in this phase**. A new control-plane dependency
(tasks 16, 22) needs a pinning review in its part: licence, `go` directive of the module, and the
transitive additions to `go.sum`, listed.

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

**The Hub contract did not exist when this plan was written.** At 2026-09-23 21:40 the sibling
worktree held `docs/superpowers/adr/ADR-H1-hub-architecture.md` at 100 lines — §0 (method) and §1
(context and forces) only; its §1.4 force 5 cites this environment's survey — and
`docs/superpowers/contracts/` held only phase 2's two contracts. So every Hub-dependent task below
is written against **the minimal shape phase 8 needs**, marked as an ask. When the Hub contract
lands, it satisfies each ask or names the difference, and tasks 09, 16, 22 and 29 are re-checked
against it **before** they start (each has that as step 1). Nothing here invents a Hub route.

| Ask | What phase 8 needs, exactly | Needed by | If the Hub contract does not provide it |
|---|---|---|---|
| **HA-1** Shared Entra token validation | One Go package, importable by both the Hub and `control-plane/cmd/broker`, with the `entra.Validator` interface in task 16's part (complete source there). The broker and the Hub must accept and reject the **same** tokens | 16, 17 | Task 16 writes it as `control-plane/internal/entra`; the Hub keeps its own and the two are reconciled at the Hub's review |
| **HA-2** A token source in the control plane | `type TokenSource interface { Token(ctx context.Context, scope string) (string, error) }` and `var ErrSignedOut`, provided by whatever the Hub phase builds for OD-6 (PKCE, Windows Credential Manager). `remotebe` calls it once per request with the broker's scope | 22, 23 | `remotebe` ships with the interface and a stub that returns `ErrSignedOut`; cloud machines show "Sign in to use cloud machines" (task 26) until the Hub's lands |
| **HA-3** The app registrations | The exact list ADR-H1 requires — display names, sign-in audience, the desktop's redirect URIs, every exposed scope and app role, the token version, the group claim — so task 09 writes them. Phase 8's own need is **one resource API for the broker exposing the delegated scope `Sandboxes.Access`**, with the desktop's public client pre-authorised for it | 09 | Task 09 writes three registrations (desktop public client, Hub API with `Hub.Access`, broker API with `Sandboxes.Access`) and flags the Hub one as provisional |
| **HA-4** The Hub's deployable | The binary's build path and `main` package; its listen port; liveness and readiness paths and what readiness checks; every `PANACEA_APP_HUB_*` variable with its default; whether it can run two replicas (any in-process state); graceful shutdown time; outbound calls it makes (Entra JWKS, Microsoft Graph, anything else) | 27, 29 | Task 29 writes the chart against the defaults in its part (`:8080`, `/healthz`, `/readyz`, stateless, 2 replicas) and marks each as provisional |
| **HA-5** The Hub's database | Postgres major version, required extensions (`azure.extensions` allowlist), expected size and connection count, and **how migrations run**: a `migrate` subcommand the chart runs as a pre-upgrade Job under a separate DDL identity, or at start-up under an advisory lock | 08, 10, 29 | Task 08 provisions Postgres **16**, no extensions, `B_Standard_B2s` in dev; task 10 creates two identities, `id-hub` (DML) and `id-hub-migrate` (DDL); task 29 runs migrations as a pre-upgrade Job |
| **HA-6** Who calls the Hub | Whether only the control plane calls the Hub (so no browser CORS at the edge), or the desktop webview does too, and whether any machine-to-machine caller exists (the broker does not call the Hub in this plan) | 13, 29 | The edge sets no CORS headers; the Hub is reached by the control plane only |
| **HA-7** Audit and object storage | Whether the audit log is a Postgres table only, or also a Log Analytics stream (a DCR and an ingestion endpoint), and whether the Hub needs Blob storage (design bundles, release artefacts) | 04, 08 | Postgres only; no storage account is provisioned for the Hub |

