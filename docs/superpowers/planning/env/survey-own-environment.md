# Survey — our own environment (phase 8, widened by OD-5)

**Date:** 2026-09-23. **Branch:** `feat/env`, based on `3303fd6`. **Status:** survey only. Nothing
here was applied, planned or initialised. No `terraform` command was run against anything.

**Scope, from the owner.** OD-5 (`docs/superpowers/planning/2026-09-23-owner-decisions.md:16`):
*"assume that you are building from scratch infrastructure wise … we will basically give you a
whole new environment to run your own stuff … it all needs to be in this repo"*. The same file
widens phase 8 to "AKS with Kata, the Hub's hosting and database, identity configuration and
networking … modelled on Panacea's live `deploy/main.tf` and written from scratch in this repo. Its
Terraform is **written and planned, never applied**, until the owner provides the environment"
(`…/2026-09-23-owner-decisions.md:52-56`). OD-13 keeps Panacea's source out of this repo for now
(`…:24`), so everything below is **pattern**, never copied text.

**How to read the citations.** `P:` means `~/Projects/Panacea/`, read-only. A bare path is this
worktree. Every `file:line` below was opened in this session unless the line says **UNVERIFIED**.
The last section lists what was not opened.

## The answer, in six lines

1. **Panacea's estate** is one AKS cluster with five tainted pools, Traefik, cert-manager, Vault,
   Keycloak and Argo CD from Terraform, and a 6,400-line script for everything else (§1).
2. **Most of the hardening Panacea wants is already written behind default-false flags**, because
   its cluster predates it. We have no cluster, so we take all of it at create time (§2).
3. **This repo has one Kata node pool pointed at Panacea's dev cluster**, a dormant but tested Kata
   backend, an unpublished sandbox image, and Go-only CI (§3).
4. **Five gaps block the build**: no environment in code; the laptop cannot reach a Kata pod (G-2,
   an in-cluster broker is recommended); an unverified RuntimeClass name; no network isolation for
   sandbox pods; no storage for clones (§4.1).
5. **Kata nodes are the cost**: requests equal limits, so about 3 default sessions per D8s_v5,
   roughly AUD 120–200 per always-on slot a month on Panacea's own estimate (§6).
6. **The owner provides** a subscription, Kata vCPU quota, a deploy identity that can assign roles,
   an OIDC service connection, and Entra app-registration rights or pre-made registrations (§7).

---

## 1. What Panacea's live estate is made of

### 1.1 How it is applied, and the two traps in that

- **One pre-existing resource group, one root module, Cloud Shell identity.** The header says it
  "Deploys the entire Panacea dev environment into ONE pre-existing resource group. Run from Azure
  Cloud Shell (uses your logged-in identity, no service principal)" (`P:deploy/main.tf:5-8`). The RG
  is a `data` source, not a resource (`P:deploy/main.tf:39-41`). The root is 22 `.tf` files,
  4,489 lines (`P:deploy/*.tf`, counted with `wc -l`).
- **Trap 1 — the auto-loaded tfvars is production.** `P:deploy/terraform.tfvars:1-22` says every
  value is production, that the RG name `rg-panacea-platform-dev` is "historical and misleading",
  that `environment = dev` is why nobody noticed, and that "a bare `terraform apply` in this
  directory targets PRODUCTION". The safe path passes `-var` flags after `-var-file`, from a
  sourced estate file (`P:deploy/terraform.tfvars:14-22`); estates live in
  `P:deploy/environments/{panacea-aue,panacea-dev-hardened,panacea-dev,panacea-prod,panacea-sandbox}.env`.
- **Trap 2 — `count = 1` plus `moved {}` on everything.** Every core resource gained `count = 1`
  late, and each carries a `moved` block so an apply re-keys rather than destroys
  (`P:deploy/main.tf:48-50`, `:65-68`, `:232-234`, `:395-402`, `:474-479`). The comment at
  `:395-398` is blunt: without it "`terraform apply` on the live Innovation cluster would DESTROY
  it".
- **Hardening is opt-in by variable, default off.** Most security posture is gated on
  `var.secure_baseline`, `var.private_cluster_enabled`, `var.aad_rbac_enabled`,
  `var.cni_overlay_enabled`, `var.egress_firewall_enabled` and friends, with defaults chosen so the
  live cluster is "byte-identical" (`P:deploy/main.tf:247-251`, `:303-312`, `:346-361`,
  `:784-787`). The reason is that the live cluster predates the hardening. **We have no live
  cluster, so we do not need this convention** (section 2).

### 1.2 Network

| Piece | Where | Value |
|---|---|---|
| VNet | `P:deploy/main.tf:52-63` | `vnet-panacea-${env}`, `10.1.0.0/16` |
| System subnet | `P:deploy/main.tf:70-76` | `snet-aks-system`, `10.1.0.0/23` |
| Workload subnet | `P:deploy/main.tf:83-89` | `snet-aks-workload`, `10.1.4.0/23` |
| One NSG on both | `P:deploy/main.tf:97-200` | 443 and 80 from `*`, 443 from `AzureCloud`, VNet↔VNet, deny-all at 4096 |
| Service CIDR | `P:deploy/main.tf:313-314` | `172.16.0.0/16`, DNS `172.16.0.10` |
| CNI | `P:deploy/main.tf:301-321` | Azure CNI, Calico policy; Overlay + Cilium only behind `cni_overlay_enabled` ("CREATE-TIME IMMUTABLE") |
| Egress | `P:deploy/main.tf:315-319` | `loadBalancer`, or `userDefinedRouting` through the firewall when `egress_firewall_enabled` |

The `/23` subnets with classic Azure CNI cap pods per node by VNet IPs; the comment at
`P:deploy/main.tf:303-307` names that ceiling as the reason for Overlay. Panacea's own comment also
records that in-cluster transit encryption is **an open gap** because Linkerd was chosen and never
installed (`P:deploy/main.tf:859-871`).

### 1.3 The AKS cluster

`P:deploy/main.tf:235-393`. One cluster, `aks-panacea-${env}`.

- **Version:** `kubernetes_version` may be `"default"` → `null`, because a pinned minor fell out of
  standard support and creates were rejected (`:241-245`). Live tfvars pins `"1.32"`
  (`P:deploy/terraform.tfvars:29`).
- **Control plane:** public by default, `private_cluster_enabled` opt-in and immutable (`:247-251`);
  `sku_tier` Free unless `aks_standard_tier` (`:253-259`); an authorised-IP list only when supplied
  (`:333-344`).
- **Identity:** `SystemAssigned` (`:261-263`), **workload identity and OIDC issuer on**
  (`:265-266`). Entra RBAC plus `local_account_disabled` move together behind `aad_rbac_enabled`
  (`:346-361`). The Helm provider authenticates with the **local admin client certificate** from
  `kube_config` (`P:deploy/main.tf:26-33`), which stops working the moment local accounts are
  disabled — a coupling we must not copy.
- **Encryption:** host encryption behind `secure_baseline` (`:285-289`); etcd KMS behind a non-empty
  `etcd_kms_key_id`, blocked in their estate because "the Contributor-only deploy SP cannot create"
  the role assignment (`:363-376`).
- **Ordering:** explicit `depends_on` the route-table association and firewall rule group, because a
  UDR cluster that boots before its allow rules "comes up broken (pentest #2)" (`:378-387`).

### 1.4 Node pools — one per blast radius

Panacea learned its pool layout from outages. Each pool is a taint plus a label plus a reason.

| Pool | Where | Size | Taint / label | Why it exists |
|---|---|---|---|---|
| `system2` | `P:deploy/main.tf:269-299` | `system_node_size` (live `Standard_B4ms`, `P:deploy/terraform.tfvars:30`), 1–2 | label `panacea.io/role=system` | AKS system pods |
| `workload` | `P:deploy/main.tf:405-472` | `workload_node_size` (live `B4ms`), **min 4**, max 6 | label `panacea.io/role=workload` | Portal, Keycloak, tenant apps. Min 4 codifies a live `az aks nodepool update` so an apply "never reverts it" (`:410-424`) |
| `build` | `P:deploy/build-pool.tf:21-62` | min `build_min_count` (0), 128 GB disk | `panacea.io/build=true:NoSchedule` | Rootless BuildKit runs untrusted Dockerfiles; scale-to-zero, warmth from the ACR registry cache (`:11-20`) |
| `ci` | `P:deploy/ci-pool.tf:32-78` | `max_pods = 30`, Ephemeral 300 GB | `panacea.io/ci=true:NoSchedule` | The 2026-08-12 outage: build pods at priority 1e6 **preempted** the platform (`:3-18`) |
| `elastic` | `P:deploy/elastic-pool.tf:9-42` | min/max vars | `panacea.io/elastic=true:NoSchedule` | Memory-spiking tier kept off shared nodes |

Two lessons carry straight over to Kata sandboxes. First, **workload segmentation by taint is how
you stop one class of pod evicting another**; Panacea also runs a priority-class ladder
(`P:deploy/pipelines/platform-priority-classes.yaml`, cited by `P:deploy/main.tf:947-949`, file not
opened) and an overprovision balloon plus a pre-pull DaemonSet
(`P:deploy/pipelines/overprovision-balloon.yaml`, `prepull-daemonset.yaml`, listed, not opened).
Second, **warm capacity is a floor, not a timeout**: "THIS is the fix for the queuing — not longer
task timeouts" (`P:deploy/main.tf:422-423`).

### 1.5 Edge — DNS, TLS, ingress, WAF

- **Public DNS zone** in the same RG (`P:deploy/main.tf:485-493`). The wildcard A record is a
  **manual step** after Traefik gets an IP (`:495-496`).
- **cert-manager** `v1.14.5` in `panacea-system` (`P:deploy/main.tf:504-563`), with an optional
  recursive-nameserver DNS-01 self-check for clusters where egress to `:53` is blocked
  (`:541-560`).
- **Traefik** chart `26.1.0` in `panacea-ingress`, one replica, `LoadBalancer`
  (`P:deploy/main.tf:566-695`). The edge IP allowlist is declared in the release because a
  `kubectl patch` was being reverted by `replace`/`force_update` (`:580-607`).
  `externalTrafficPolicy: Local` so the IP allowlist sees real client IPs (`:619-624`); cross-
  namespace and ExternalName routes are allowed (`:626-645`); metrics carry service and router
  labels for the per-tenant golden signals (`:670-692`).
- **WAF:** an Application Gateway WAF_v2 with OWASP CRS 3.2, regional rather than Front Door "for
  AU sovereignty", default **off**, "~A$300-500+/month" (`P:deploy/waf-appgw.tf:1-32`). Traffic
  model: client → App Gateway (TLS) → Traefik (HTTP) → pods (`:20-22`).

### 1.6 Egress — Azure Firewall, gated per estate

`P:deploy/firewall.tf`, every resource `count = var.egress_firewall_enabled ? 1 : 0`, "the live
innovation env … egress_firewall_enabled = false" (`:27-33`); the prod and Australia East estate
files turn it on (section 1.14). When on: `AzureFirewallSubnet`
(`:42-48`), static public IP (`:51-66`), a Standard policy with DNS proxy (`:69-91`), and one rule
group (`:98-210`): NTP and DNS to `168.63.129.16` only (`:112-128`), `AzureCloud.<region>` on
443/1194/9000 (`:131-140`), the `AzureKubernetesService` FQDN tag (`:149-161`), ACR's dedicated
data endpoint **instead of `*.blob.core.windows.net`** (`:166-179`), the LLM gateway, Entra, ARM,
Key Vault (`:180-184`), and tenant build egress to GitHub, Azure DevOps and PyPI (`:192-207`). A
`check` block fails the plan if `var.location` and the RG region diverge (`:12-17`). Cost note:
"~A$1.5-2k/month + data processing" (`:35-37`).

### 1.7 Identity — oauth2-proxy → Keycloak → Entra

- **Keycloak** via `codecentric/keycloakx` `2.4.4` in `panacea-auth`, **one replica,
  `--cache=local`** (`P:deploy/main.tf:915-957`). Sessions live in memory, so each pod move "logged
  every user out" (`:941-946`). It gained `priorityClassName: panacea-platform-critical` after an
  SSO outage on 2026-09-21 (`:930-950`). It runs non-optimised, re-building Quarkus on every start
  (~5 min), so the startup probe was widened to ~765 s (`:997-1018`). `KC_PROXY_HEADERS=xforwarded`
  so auth cookies keep their `Secure` flag across the Entra return (`:969-979`).
- **Keycloak's database** is a `postgres:16-alpine` StatefulSet applied by `kubectl` from the deploy
  script, not Terraform, because Bitnami stopped publishing tags (`P:deploy/main.tf:908-912`).
- **oauth2-proxy** is **not in Terraform**: no `.tf` file declares it (grep of `P:deploy/*.tf`
  finds only the comment at `P:deploy/main.tf:938`); the deploy script mentions it 74 times
  (`grep -c` on `P:deploy/cloud-shell-deploy.sh`). How it is deployed was not read.
- **Workload identity** is on at the cluster (`P:deploy/main.tf:265-266`) and used by the S3Proxy
  (`P:deploy/object-storage-blob.tf:248-315`, federated credential at `:283`) and optionally by
  Vault for auto-unseal (`P:deploy/main.tf:730-770`).

### 1.8 Secrets and keys

- **HashiCorp Vault** `0.28.1`, standalone Raft, 10 Gi data + 5 Gi audit on `managed-csi`
  (`P:deploy/main.tf:825-906`). Shamir unseal by default; Azure Key Vault auto-unseal is opt-in
  (`:697-728`, `:819-824`). The listener is **plaintext** and the comment records ISM-0469 as "AN
  OPEN GAP" (`:856-872`).
- **Credentials in state:** a `helm_release`'s values are persisted in state, so the Keycloak admin
  and DB passwords "sat in foundation.<env>.tfstate in plaintext" until an opt-in `secretKeyRef`
  form was added (`P:deploy/main.tf:772-816`). The live default is still the inline form (`:784-787`).
- **ACR and Key Vault are created imperatively** by the deploy scripts (`az acr create`,
  `az keyvault create`) and only referenced by Terraform as `data` sources
  (`P:deploy/private-endpoints.tf:5-8`, `:22-37`).
- **CMK substrate** (Premium KV, RSA-HSM key, Disk Encryption Set) is written but off
  (`P:deploy/cmk.tf:1-31`); ACR CMK likewise (`P:deploy/acr-cmk.tf`, not opened past the index).
- **The RBAC gap.** The deploy identity is Contributor-only and cannot write role assignments
  ("NO_RBAC_MODE"), so every managed-identity grant sits behind a second flag and an owner-run
  RBAC Administrator identity (`P:deploy/rbac-admin-identity.tf:1-34`). This single constraint
  explains most of the estate's shared admin credentials.

### 1.9 Data

- **Live:** in-cluster CloudNativePG. The managed Flexible Server file says so: with its gate off
  "it does NOT touch the live CloudNativePG data plane" (`P:deploy/postgres-flexible.tf:16-21`).
- **Written, off:** `azurerm_postgresql_flexible_server.shared` (`P:deploy/postgres-flexible.tf:237-314`)
  — Postgres 16 (`:65-68`), `GP_Standard_D2ds_v4` (`:76-79`), 32 GiB with auto-grow (`:82-85`,
  `:251`), 14-day PITR (`:93-96`), zone-redundant HA 1→2 (`:110-124`, `:267-271`), **public access
  off, private endpoint only** (`:258-263`, `:318-344`), `azure.extensions = VECTOR` (`:349-354`),
  `require_secure_transport = ON` (`:357-362`). Authentication is **admin login + password**
  (`:244-245`), with a `check` that the password is ≥16 characters (`:178-183`). Per-tenant databases
  are created imperatively by the Go API, not Terraform (`:23-27`).
- **Object storage:** live is a self-hosted MinIO; the written-but-off replacement is an ADLS Gen2
  account behind S3Proxy with a private endpoint and a workload-identity-federated S3Proxy identity
  (`P:deploy/object-storage-blob.tf:1-22`, `:72`, `:215`, `:248`, `:283`, `:310`).

### 1.10 Registry and builds

- **ACR** is created by `az acr create` in the deploy script and referenced by name
  (`P:deploy/private-endpoints.tf:5-8`, `:22`). The build pool pushes and pulls a registry cache
  (`P:deploy/build-pool.tf:14-18`). Pull auth in the live estate is the shared ACR admin credential,
  because managed-identity `AcrPull` needs the RBAC write the deploy identity lacks
  (`P:deploy/rbac-admin-identity.tf:20-21`).
- Builds run in-cluster on Tekton (`P:deploy/ci-pool.tf:1-18`) with rootless BuildKit on the tainted
  `build` pool (`P:deploy/build-pool.tf:1-10`).

### 1.11 Observability and security services

- **Log Analytics** `PerGB2018`, retention and a **daily ingest cap** from variables, shared-key
  auth disabled under the baseline (`P:deploy/main.tf:206-226`). Container Insights uses MSI
  (`:323-331`) — which then collects **nothing** without a Data Collection Rule, the gap
  `P:deploy/monitoring-dcr.tf:4-16` closed.
- **Control-plane and kube-audit logs:** none in the live estate; written but off because kube-audit
  "can blow the dev 1 GB/day workspace cap" (`P:deploy/aks-diagnostics.tf:4-22`).
- **Defender for Cloud and Sentinel:** written, off, and **subscription-scoped** unlike the rest of
  the RG-scoped module (`P:deploy/defender-sentinel.tf:1-30`). Note `PROGRAM.md:133`: Defender for
  Containers cannot assess Kata pods.

### 1.12 CI/CD and the deploy engine

- **Pipelines** are several YAML files at the repo root: `azure-pipelines.yml`, `-dev.yml`,
  `-drift.yml`, `-main.yml`, `-v2.yml` (listed, only `-dev.yml` opened).
- **`azure-pipelines-dev.yml`** is manual (`trigger: none`, `:26-27`), runs on a
  Microsoft-hosted `ubuntu-latest` agent with a 360-minute timeout (`:71-73`), selects one of 20
  subcommands (`:34-58`), and **refuses to run** unless the branch is `refs/heads/dev` and the env
  file is `panacea-dev.env` (`:78-86`). It installs kubectl, helm, terraform 1.9.8 and jq by
  `curl` at run time (`:88-114`) and runs under a **secret-based** service principal with
  Contributor (`:17-24`, `:116-130`).
- **The engine is a shell script:** `P:deploy/cloud-shell-deploy.sh` is 6,406 lines and
  `P:deploy/panacea-deploy.sh` 2,849 (`wc -l`). Terraform is one step inside it. It generates
  `imports.generated.tf` from per-resource existence probes before each plan, because static
  `import` blocks hard-fail on a fresh subscription (`P:deploy/imports.tf:1-23`).
- **State** is an `azurerm` backend with partial configuration supplied at `init`
  (`P:deploy/versions.tf:8-15`); the estate file carries `TF_STATE_STORAGE_ACCOUNT`,
  `TF_STATE_CONTAINER`, `TF_STATE_KEY` (`P:deploy/environments/panacea-dev.env:70-72`, names only).
- **The in-VNet agent** for a private cluster is written, off: a VMSS in its own subnet
  (`P:deploy/ado-selfhosted-agent.tf:1-23`, `:123`, `:162`) — because a hosted agent "CANNOT reach
  a private API server or a private-endpoint-only registry" (`:5-9`).

### 1.13 GitOps

- **Argo CD** chart `6.7.3`, one replica per component, in `panacea-system`
  (`P:deploy/main.tf:1039-1077`). The estate file names `PANACEA_GITOPS_REPO_URL` and
  `PANACEA_GITOPS_REVISION` (`P:deploy/environments/panacea-dev.env:52-53`, names only). What the
  applications are was not read. Panacea's own record shows GitOps-managed values drifting from what
  runs: the Keycloak priority class sat in a values file "nothing on the live deploy reads"
  (`P:deploy/main.tf:933-940`).

### 1.14 The estate on one page

| Layer | Live in Panacea | Managed by |
|---|---|---|
| RG, ACR, Key Vault | pre-created / `az … create` | **outside Terraform** |
| VNet, 2 subnets, NSG | yes | Terraform |
| AKS + 5 pools | yes; posture per estate file (below) | Terraform |
| DNS zone | yes; wildcard A manual | Terraform + hand |
| cert-manager, Traefik, Vault, Keycloak, Argo CD | yes | Terraform `helm_release` |
| oauth2-proxy, Keycloak's Postgres, CNPG, MinIO, Tekton | yes | **deploy script** (`kubectl`) |
| Firewall, WAF, Flexible Server, Blob, CMK, Defender, in-VNet agent | written; **on only in the target estate files** | Terraform, gated |
| CI | hosted agent + secret SP + 6k-line script | ADO |

**The posture differs by estate file, and the files disagree with the comments.** Dev sets a public
API, no firewall, no Entra RBAC (`P:deploy/environments/panacea-dev.env:78-82`). The prod file sets
private cluster, Entra RBAC, firewall and private PaaS on (`…/panacea-prod.env:57-62`) and Vault
auto-unseal (`:68`). The Australia East file adds CNI Overlay + Cilium, Flexible Server and WAF
(`…/panacea-aue.env:54-68`, `:81`, `:127`). All three keep `NO_RBAC_MODE="true"`
(`…/panacea-dev.env:92`, `…/panacea-prod.env:73`, `…/panacea-aue.env:136`). Yet `P:deploy/firewall.tf:27-29`
says "the live innovation env … egress_firewall_enabled = false", and a private cluster is
create-time immutable (`P:deploy/main.tf:247-251`). **Whether the running prod cluster was created
with the prod file's posture is UNVERIFIED**; it was not checked against Azure, by design.

The pattern that matters most: **everything Panacea wishes it had is already written behind a
default-false flag, because the live cluster predates it and a create-time field cannot be changed
on a running cluster.** We start with no live cluster, so we can have all of it at create time.

---

## 2. What we reuse as a pattern, and what we deliberately do differently

OD-13 forbids copying. Everything in the left column is re-written from scratch in `infra/`.

### 2.1 Reuse as pattern

| Pattern | Panacea evidence | How it lands here |
|---|---|---|
| One pool per blast radius, taint + label + nodeSelector | `P:deploy/ci-pool.tf:3-18`, `build-pool.tf:43-52` | `system`, `apps` (Hub + broker), `sandbox` (Kata). Phase 1 already uses `panacea-app/role=sandbox` + a taint (`infra/terraform/kata.tf:23-29`, `control-plane/internal/sandbox/manager.go:164-170`) |
| Warm capacity is a floor, not a timeout | `P:deploy/main.tf:410-424` | `min_count` on the Kata pool plus a balloon and an image pre-pull DaemonSet (section 4, G-6) |
| Priority classes so builds cannot preempt login | `P:deploy/main.tf:947-950`, `P:deploy/ci-pool.tf:5-9` | Hub and broker above sandboxes; sandboxes above balloons |
| Private endpoints + privatelink DNS zones for PaaS | `P:deploy/postgres-flexible.tf:202-231`, `:318-344` | Postgres, Key Vault, ACR, from the first apply |
| Workload identity + federated credentials | `P:deploy/main.tf:265-266`, `P:deploy/object-storage-blob.tf:283` | Every pod that touches Azure; **no client secrets** |
| Egress allowlist scoped to ACR's dedicated data endpoint, not `*.blob` | `P:deploy/firewall.tf:166-179` | Same rule, if a firewall is chosen (OQ-3) |
| Region single-source `check` | `P:deploy/firewall.tf:12-17` | A `check` that every region input equals the RG's |
| `check` blocks that fail the plan on a half-configured feature | `P:deploy/postgres-flexible.tf:178-191` | Same idiom for Entra ids, DNS delegation and CMK pairs |
| Estate guard: pipeline refuses a wrong branch/env pairing | `P:azure-pipelines-dev.yml:78-86` | One guard step per environment stage |
| Declare in the release what was once patched live | `P:deploy/main.tf:586-592`, `:632-645` | Rule: no `kubectl patch` in any runbook; drift is a bug |
| Log Analytics with a daily ingest cap + a DCR | `P:deploy/main.tf:206-226`, `P:deploy/monitoring-dcr.tf:4-16` | Same, with the DCR in the first apply |
| Regional WAF, not Front Door, for sovereignty | `P:deploy/waf-appgw.tf:8-12` | If a WAF is bought, it is App Gateway in-region |

### 2.2 Deliberately different

| Panacea does | We do instead | Why |
|---|---|---|
| Everything opt-in behind default-false flags so the live cluster is "byte-identical" (`P:deploy/main.tf:784-787`) | **Secure at create time, no posture flags** for create-time fields: CNI Overlay + Cilium, Entra RBAC with local accounts off, workload identity, host encryption | We have no live cluster to protect, and these fields are immutable (`P:deploy/main.tf:249`, `:307`) |
| `count = 1` plus `moved {}` on every resource (`P:deploy/main.tf:395-402`) | Plain resources in small modules; `moved` only for real refactors | The `moved` blocks exist to rescue history we do not have |
| One flat root of 22 files, one state (`P:deploy/versions.tf:15`) | **Separate stacks with separate state**: `bootstrap`, `foundation`, `data`, `identity`, `workloads` | A mistake in a Helm value cannot plan a cluster replacement |
| Helm provider authenticated with the local admin client cert (`P:deploy/main.tf:26-33`) | No `helm_release` in the cluster's own stack; in-cluster objects are deployed by the pipeline with Entra auth (`kubelogin`) | Panacea's provider breaks the day `local_account_disabled` flips (`:361`) |
| ACR and Key Vault created by `az … create`, referenced as `data` (`P:deploy/private-endpoints.tf:5-8`) | Both are Terraform resources | Nothing that the estate depends on lives outside state |
| Contributor-only deploy identity, `NO_RBAC_MODE`, shared ACR admin credential (`P:deploy/rbac-admin-identity.tf:5-21`) | The deploy identity gets role-assignment rights **scoped by an ABAC condition to a role allowlist** (owner-provided, section 7) | Otherwise every managed-identity grant becomes a second flag and a shared secret |
| Secret-based service connection whose secret is reused for DNS writes (`P:azure-pipelines-dev.yml:17-20`) | Workload identity federation (OIDC) service connection; cert-manager uses its own federated identity | No long-lived secret anywhere in CI |
| Keycloak (1 replica, in-memory sessions, 5-min rebuild) + oauth2-proxy (`P:deploy/main.tf:915-1018`) | **No broker in v1.** The desktop does PKCE against Entra (OD-6); the Hub validates Entra JWTs itself | Removes Panacea's recurring SSO outage class (`:930-946`, `:997-1005`). The Hub ADR may add a broker for customer BYO-IdP later |
| HashiCorp Vault, Shamir or AKV unseal, plaintext listener (`P:deploy/main.tf:825-906`, `:856-872`) | **Azure Key Vault only**, read through workload identity or the Secrets Store CSI add-on | No unseal ceremony, no plaintext in-cluster hop |
| Postgres admin password in a variable (`P:deploy/postgres-flexible.tf:244-245`) | Flexible Server with **Entra-only authentication**; the Hub connects as its managed identity | No DB password exists to leak into state (`P:deploy/main.tf:772-781`) |
| In-cluster CNPG (`P:deploy/postgres-flexible.tf:16-21`) and self-hosted MinIO (`P:deploy/object-storage-blob.tf:5-8`) | Managed Postgres from day one; object storage (Blob, private endpoint) only if the Hub ADR needs it | Panacea's own history has a shared in-cluster volume filling and taking every tenant down (a program memory note, **not re-verified in this survey**) |
| Imports generated from existence probes (`P:deploy/imports.tf:16-23`) | No imports; a fresh subscription is a plain create | We are the fresh subscription |
| A 6,406-line deploy script, 20 subcommands | Terraform + a thin pipeline + one `smoke` script per stack | A deploy is reviewable in a diff |
| Many pools on one `/23` workload subnet, `max_pods = 30` (`P:deploy/ci-pool.tf:25-31`) | Overlay pod CIDR; subnets sized for nodes only | The IP ceiling was Panacea's scheduling limiter |

---

## 3. What this repo already has

### 3.1 Terraform — one node pool, attached to someone else's cluster

`infra/terraform/` is four files and a lock file (`ls -a`; no `.terraform/`, no state). It was
committed in `21c174e` on 2026-09-17 and last touched in `e475d6a`.

- `kata.tf:4-42` — one `azurerm_kubernetes_cluster_node_pool` named `katapool`: `AzureLinux`
  (`:8`), `workload_runtime = "KataVmIsolation"` (`:16`), autoscaling `node_count`…`max_count`
  (`:18-21`), label `panacea-app/role=sandbox` (`:23-25`), taint
  `panacea-app/sandbox=true:NoSchedule` (`:27-29`), `ignore_changes = [node_count]` (`:36-41`).
  The comment at `:10-15` records that the brief's `KataMshvVmIsolation` was rejected by provider
  v4.81.0.
- `providers.tf:1-13` — `azurerm ~> 4.0`, Terraform `>= 1.9.0`, **no backend block** (local state).
- `variables.tf:1-21` — `aks_cluster_id` (no default), `node_count = 1`, `max_count = 4`,
  `vm_size = "Standard_D8s_v5"`.
- `dev.tfvars:1-7` — the cluster id is supplied from
  `az aks show -g rg-aks-panacea-dev -n aks-panacea-dev` (`:4`), i.e. **Panacea's dev cluster**.
  OD-5 makes that target obsolete: the pool now belongs in our own cluster.
- `.gitignore:7-9` ignores `.terraform/`, `*.tfstate`, `*.tfstate.backup`.

### 3.2 The Kata backend — complete for one pod, dormant

- `control-plane/internal/backend/katabe/kata.go:1-4` — "DORMANT … turning it on in Phase 8 is a
  configuration change and not a rewrite". It satisfies `backend.Backend` (`:25`); `Create` refuses
  `Mounts` and `WorkDir` rather than dropping them (`:75-89`), waits for scheduling and then
  `/healthz` (`:108-119`, `:128-149`); the endpoint is **the pod IP** on port 8080 (`:185-188`);
  `Attach` dials it with the bearer token (`:232-241`); `Destroy` is idempotent (`:245-257`).
- `control-plane/internal/sandbox/manager.go:139-210` — the pod: `runtimeClassName`
  `kata-mshv-vm-isolation` (`:20`, `:160`), no host network (`:161-162`), `RestartPolicy: Never`
  (`:163`), the sandbox toleration and nodeSelector (`:164-170`), `emptyDir` for `/home/dev` and
  `/tmp` under a read-only root (`:171-187`), requests = limits (`:188-193`), non-root uid 1001,
  no privilege escalation, all capabilities dropped (`:194-200`). One namespace per manager
  (`:55-64`), default `panacea-app` (`control-plane/cmd/api/config.go:57`).
- Tests: 20 in `katabe/kata_test.go` (`:154`–`:606`) and 14 in `sandbox/*_test.go`, all against a
  fake clientset. **None has ever run against a cluster.**
- Selection: `PANACEA_APP_BACKEND=kata` (`control-plane/cmd/api/config.go:25`, `:164-165`); main
  builds the client from **the in-cluster service account only** and refuses
  `PANACEA_APP_KUBECONFIG` (`control-plane/cmd/api/main.go:225-233`, `:241-271`), on the stated
  assumption that "the real Kata deployment runs inside the cluster" (`:248`).
- Default session size is 2 vCPU / 4 GiB (`control-plane/internal/backend/backend.go:110`).

### 3.3 The sandbox image

`images/sandbox/Dockerfile:1-34`: sandboxd built from the vendored tree because the corporate proxy
breaks `go mod download` (`:3-8`); `node:20-bookworm-slim` with git, ripgrep, curl (`:10-13`);
Claude Code pinned to `2.1.274` (`:15-17`, the version OD-8's adapter pins to); user `dev` uid 1001
(`:19`); `~/.claude/projects` pre-created for the Docker bind-mount of the conversation store
(`:26-32`). `entrypoint.sh:1-17` execs sandboxd on `:8080` with the `PANACEA_APP_*` argv contract.
**The image is not published anywhere**: there is no registry and no image build in CI.

### 3.4 CI

`azure-pipelines.yml:1-17` — triggers on `main` and `dev` only, `ubuntu-latest`, Go 1.23, then
`go build`, `go test -race -count=1` and `go vet` over `./sandboxd/...` and `./control-plane/...`.
**Nothing** for `desktop/` (which has `typecheck`, `test` and `build` scripts,
`desktop/package.json:7-13`), `infra/`, `images/`, licences or secrets. No PR trigger for `feat/*`.
No `.github/` directory, though OD-7 made `github.com/lcfrai/panaceaapp` the backup remote.

### 3.5 What else the environment must host, from other phases

- **The Hub** (Phase H): identity, briefs, approvals, ledger, releases, teams, built local-first on
  Docker Postgres and "hosted in our own environment once phase 8 stands it up"
  (`docs/PROGRAM.md:94-95`). Its decomposition is its own ADR (owner decisions `:49-50`).
- **Clones in pods:** phase 3 rules a worktree is a local clone, and "Getting a clone into a Kata pod"
  is assigned to phase 8 (`docs/superpowers/planning/2026-09-23-phases-4-5-7/scope-and-risk-phase-3-git-convergence.md:30`, `:288`).
- **Control-plane authentication:** `desktop/src/api/client.ts:6-8` says no `Authorization` header
  "before Phase 8"; critique-2 records the owner of that boundary as unnamed
  (`…/completeness-critique-2.md:47`, `:524`). OD-6 plus the Hub answer who; phase 8 must host it.

---

## 4. The gaps

### 4.0 The shape the gaps are measured against (recommendation, for the phase 8 plan to confirm)

```
 desktop (Tauri) ── control plane, 127.0.0.1 only (unchanged) ──┐  user's Entra token
                                                                 ▼
 our subscription ─ Australia East ─ one VNet ─ AKS (Overlay + Cilium, Entra RBAC, WI)
   ├─ ingress (TLS, DNS)  ──►  apps pool:   Hub API · sandbox broker (katabe in-cluster)
   │                           sandbox pool (Kata, tainted): one microVM pod per session
   ├─ Postgres Flexible (Entra auth, private endpoint)   ├─ Key Vault (private endpoint)
   ├─ ACR Premium (private endpoint, kubelet AcrPull)     └─ Log Analytics (+ DCR, cap)
   └─ egress: NAT gateway (dev) or Firewall (prod) — OQ-3
```

### 4.1 Blocking — the plan cannot start its build tasks without an answer

| Id | Gap | Evidence | Recommendation |
|---|---|---|---|
| **G-1** | **No environment in code.** No RG, VNet, cluster, registry, Key Vault, workspace, DNS or state backend; the one pool targets Panacea's dev cluster | `infra/terraform/variables.tf:1-4`, `dev.tfvars:4`, `providers.tf:1-13` | Stacks `bootstrap` → `foundation` → `data` → `identity` → `workloads`, one state each, in `infra/terraform/` |
| **G-2** | **The laptop cannot reach a Kata pod.** The control plane binds loopback (`control-plane/cmd/api/config.go:54`), builds its Kubernetes client from the in-cluster service account only (`main.go:264`), and dials the **pod IP** (`katabe/kata.go:185-188`). From a laptop none of that resolves | as cited | An **in-cluster sandbox broker**: the same `katabe` code, deployed in the cluster, behind the ingress, authenticating the user's Entra token, which the local control plane calls as one more backend. **OQ-1** |
| **G-3** | **The RuntimeClass name is unverified.** Code hardcodes `kata-mshv-vm-isolation` (`manager.go:20`) against a pool created with `KataVmIsolation` (`kata.tf:16`); already flagged in `docs/verification/2026-09-17-phase-1-results.md:64-65` and `phase-1-operator-runbook.md:126-132` | as cited | From training knowledge, **not verified**: the GA pairing is `KataVmIsolation` ↔ `kata-vm-isolation`, and `kata-mshv-vm-isolation` was the preview name. Make it a `PANACEA_APP_*` variable and make `kubectl get runtimeclass` the first smoke check |
| **G-4** | **Sandbox pods are not network- or API-isolated.** No `automountServiceAccountToken: false`, no seccomp profile, no NetworkPolicy, no quota; one shared namespace (`manager.go:55-64`, `:153-202`). Kata isolates the kernel, not the network: an agent can reach the IMDS endpoint, the API server with the default token, and every other sandbox | as cited | Token off, `RuntimeDefault` seccomp, default-deny ingress except from the broker, deny sandbox↔sandbox and `169.254.169.254`, `ResourceQuota` per user namespace or per user label |
| **G-5** | **No storage for a clone or a conversation.** Kata refuses mounts (`kata.go:84-86`); home is `emptyDir` (`manager.go:175-187`); the image expects a bind-mounted `~/.claude/projects` (`images/sandbox/Dockerfile:26-32`). Resume (OD-10), `/login` credentials and the phase 3 clone all vanish with the pod | as cited | A per-worktree PVC (Azure Disk CSI, RWO). **Whether AKS Pod Sandboxing supports CSI volumes is UNVERIFIED**; phase 8's first cluster task proves it |

### 4.2 Needed for a usable environment, not blocking the first apply

| Id | Gap | Evidence | Recommendation |
|---|---|---|---|
| **G-6** | **No warm pool.** `min_count = node_count = 1` (`kata.tf:19-20`), no balloon, no pre-pull. A pod-level pool (pre-booted, claimed later) is **not expressible today**: command, argv and the attach token are environment at pod start (`manager.go:98-127`) | as cited; open risk 1 (`docs/PROGRAM.md:137`) | Node-warm first (floor + low-priority Kata balloon + pre-pull DaemonSet), then **measure** cold start. Pod-warm only if the measurement demands it, as a sandboxd contract amendment |
| **G-7** | **No egress design for agents.** Panacea's allowlist has no Anthropic, npm or model-provider host (`P:deploy/firewall.tf:170-207`); a coding agent needs package registries, git hosts (OD-4) and its provider | as cited | **OQ-3** |
| **G-8** | **Identity is not configuration yet.** No Entra app registrations, groups or federated credentials in code; OD-6 needs a desktop public client (PKCE, loopback redirect) and the Hub needs an API with a scope | OD-6 (`…/2026-09-23-owner-decisions.md:17`) | An `identity` stack on the `azuread` provider, **if** the owner grants app-registration rights; otherwise the owner creates them and we import the ids as variables (section 7) |
| **G-9** | **Nothing server-side is packaged.** No Dockerfile for the Hub or the broker, no chart or manifests, no ingress, no TLS | `ls images/` shows only `sandbox` | `images/hub`, `images/broker`; one Helm chart per service under `deploy/charts/` applied by the pipeline |
| **G-10** | **No registry or image supply chain.** The sandbox image is built locally and never pushed; the CLI version pin (`Dockerfile:17`) is the OD-8 adapter's pin | as cited | ACR Premium, digest-pinned images, SBOM + scan in CI, kubelet `AcrPull` (no admin user) |
| **G-11** | **No observability or budget guard.** | nothing in `infra/` | Log Analytics with a daily cap and a DCR in the first apply; kube-audit on, with the cap sized for it; a subscription budget alert |
| **G-12** | **Defender for Containers cannot assess Kata pods** | `docs/PROGRAM.md:133` | Record as a compliance gap in the phase 8 plan, as `PROGRAM.md` already asks |

---

## 5. CI/CD shape for a monorepo of `go.work`, `desktop/` and `infra/`

**Principle.** OD-5 keeps a monorepo so cross-cutting changes land atomically
(`…/2026-09-23-owner-decisions.md:16`). CI must therefore validate *every tree a change touches* in
one PR, and deploy nothing a reviewer did not see in a plan.

**Host.** Azure DevOps is the CI of record: `PROGRAM.md:11` says "own ADO pipeline" and
`azure-pipelines.yml` exists. `github.com/lcfrai/panaceaapp` is a backup remote (OD-7), not a
second CI; two CI systems drift. (OD-4 is about *users'* git hosts, not ours.)

**Layout.** One pipeline file per concern under `pipelines/`, each with a `paths` trigger, sharing
templates; ADO branch policies run each as PR build validation filtered by path. The PR is green only
when every affected pipeline is.

| Pipeline | Trigger paths | Agent | Steps | Gates |
|---|---|---|---|---|
| `go` | `go.work`, `control-plane/**`, `sandboxd/**` | ubuntu | build, `vet`, `test -race -count=1` per module (today's three steps, `azure-pipelines.yml:12-17`, Go 1.23 at `:9-11`), `govulncheck` | PR |
| `desktop` | `desktop/**` | ubuntu + **windows** | `npm ci`, `typecheck`, `test` (`desktop/package.json:11-13`); `cargo clippy` for `src-tauri`; `tauri build` on Windows for `main` only | PR; signing is phase 9 (`docs/PROGRAM.md:139`) |
| `licences` | any lockfile or `go.sum` | ubuntu | npm, Go and Cargo licence reports against **MIT, Apache-2.0, BSD, ISC** | PR — OD-11 rule 3 (`…/owner-decisions.md:96-98`) applied repo-wide |
| `images` | `images/**`, `sandboxd/**`, server code | ubuntu | `buildx` per image, tag = git SHA, SBOM, vulnerability scan, OD-8 transcript canary against the built sandbox image; push to ACR by digest on `dev`/`main` only | PR builds, never pushes |
| `infra` | `infra/**` | ubuntu | `fmt -check`, `init -backend=false` + `validate` per stack, a linter and a static IaC scan — **no Azure call, safe on every PR** | PR |
| `infra-plan` | `infra/**` | in-VNet pool (below) | `plan` against remote state, published as a PR artifact | **Absent until the owner's environment exists** (owner decisions `:56`) |
| `deploy` | manual / `main` | in-VNet pool | `apply` of the saved plan; `helm upgrade --install --atomic` for the Hub and broker; smoke: RuntimeClass exists, a sandbox boots, attach works through the broker | ADO Environment approval; estate guard (branch ↔ env, as `P:azure-pipelines-dev.yml:78-86`) |
| `secrets` | all | ubuntu | a secret scanner over the diff | PR |

**Identity.** One workload-identity-federation (OIDC) service connection per environment; no client
secret (contrast `P:azure-pipelines-dev.yml:17-20`). The deploy identity is the one in section 7.

**Reaching the cluster.** Microsoft-hosted agents have no stable IP and cannot reach a private API
(`P:deploy/ado-selfhosted-agent.tf:5-9`). Recommendation: a **VNet-injected Managed DevOps Pool**
for `infra-plan` and `deploy`; fall back to a VMSS agent in its own subnet, Panacea's pattern.
Whether Managed DevOps Pools are enabled in KPMG's ADO organisation is **UNVERIFIED**.

**Terraform versions.** Terraform `>= 1.9.0` and `azurerm ~> 4.0` (`infra/terraform/providers.tf:2-7`);
the lock file pins the exact provider (`infra/terraform/.terraform.lock.hcl`, not opened). Pin the
Terraform binary in the pipeline, as Panacea pins 1.9.8 (`P:azure-pipelines-dev.yml:102-106`).

**Two traps from Panacea to design out.** A fresh bootstrap exceeded the 60-minute free hosted-job
cap (`P:azure-pipelines-dev.yml:24`), so buy parallel jobs before the first apply. And tools
installed by `curl` at run time (`:88-114`) drift; pin versions in a template.

---

## 6. Cost drivers

**No price below was fetched** (corporate network; no pricing call was made). Anchors are Panacea's
own in-code estimates, marked with their file. Treat everything else as order of magnitude.

### 6.1 The dominant driver: Kata nodes, because requests equal limits

- A session is 2 vCPU / 4 GiB by default (`control-plane/internal/backend/backend.go:110`), and
  requests **equal** limits because the guest VM is sized from them (`manager.go:188-193`,
  `docs/PROGRAM.md:133`). There is no overcommit: an idle agent costs the same as a busy one.
- On the default `Standard_D8s_v5` (8 vCPU, 32 GiB; `infra/terraform/variables.tf:19`) that is **at
  most 3 sessions per node**, CPU-bound, after node reservations and per-pod Kata overhead. The
  exact allocatable CPU and the Kata pod overhead are **UNVERIFIED**; phase 8 measures both.
- Panacea's own estimate for one always-on D8-class node is "~AUD $355-601/mo"
  (`P:deploy/build-pool.tf:19-20`, for `D8ds_v5`). So an always-on session slot costs roughly
  **AUD 120–200 a month**.
- **Worked example, to size a budget, not a quote:** 10 users × 4 concurrent sessions = 40 slots ≈
  14 nodes ≈ AUD 5–8.4k a month if always on, far less with a small floor and autoscaling.
- **The levers**, in order of effect: the warm floor (`min_count`, G-6); the idle reaper
  (`PANACEA_APP_IDLE_AFTER`, default 90 s, `control-plane/cmd/api/config.go:58`); session size
  (`PANACEA_APP_SESSION_CPU_MILLIS` / `_MEMORY_MIB`, `config.go:32-33`); off-hours scale-down; SKU
  shape (a 1 vCPU / 4 GiB session fits a memory-optimised family better than D-series).

### 6.2 The rest, ranked

| Driver | Anchor | Lever |
|---|---|---|
| Azure Firewall Standard, if chosen (OQ-3) | "~A$1.5-2k/month + data processing" (`P:deploy/firewall.tf:35-37`) | NAT gateway in dev; firewall only for a PROTECTED estate |
| Postgres Flexible Server with zone-redundant HA | none in code; HA roughly doubles compute (standby) | Burstable SKU and no HA in dev; HA in prod |
| App Gateway WAF_v2, if chosen | "~A$300-500+/month before data processing" (`P:deploy/waf-appgw.tf:24-27`) | Only if the edge is internet-facing |
| Log Analytics ingest, kube-audit especially | kube-audit "can blow the dev 1 GB/day workspace cap" (`P:deploy/aks-diagnostics.tf:17-21`) | Daily cap (`P:deploy/main.tf:213-214` pattern); basic-logs tier for audit (UNVERIFIED availability) |
| Defender plans + Sentinel | "~A$15/server/mo", Sentinel per-GB on top (`P:deploy/defender-sentinel.tf:26-30`) | Owner sign-off; may already be on at the subscription level |
| System + apps pools, AKS Standard tier | not in code; Standard is a fixed per-cluster fee (`P:deploy/main.tf:253-259`) | Free tier in dev |
| ACR Premium, private endpoints, public IPs | "ACR must be Premium" for private access (`P:deploy/environments/panacea-prod.env:62`) | Standard ACR in dev if private endpoints are skipped there |
| Per-worktree disks (G-5) | none | Delete the PVC with the worktree (the OD-10 rule, extended) |
| CI: parallel jobs, Windows minutes for Tauri | 60-min free cap (`P:azure-pipelines-dev.yml:24`) | Windows builds on `main` only |

---

## 7. What the owner must provide when the environment arrives

### 7.1 Before the first `terraform plan`

1. **A dedicated Azure subscription and its tenant id**, and the region. Recommendation:
   Australia East, for three availability zones (`P:deploy/environments/panacea-aue.env:22`).
2. **vCPU quota for the Kata VM family** in that region: max Kata nodes × 8 vCPU for `D8s_v5`, plus
   the system and apps pools. Pod Sandboxing needs a nested-virtualisation Gen2 SKU
   (`infra/terraform/kata.tf:1-3`).
3. **A deploy identity with role-assignment rights**, scoped by an ABAC condition to a role
   allowlist, so we never run `NO_RBAC_MODE` (`P:deploy/rbac-admin-identity.tf:5-11`). Owner on
   the subscription is the simpler alternative.
4. **An ADO project and a workload-identity-federation service connection** to that subscription,
   plus paid parallel jobs, plus a VNet-injectable agent pool or the right to create one.
5. **A Terraform state location** — or the right to create it in a `bootstrap` stack: a storage
   account with versioning, soft delete and Entra-only data access. Panacea names this in its estate
   file (`P:deploy/environments/panacea-dev.env:70-72`).

### 7.2 Before the first apply

1. **Entra app registrations**, or the right to create them (Application Administrator for the
   deploy identity): a desktop public client for PKCE with a loopback redirect (OD-6), a Hub API
   exposing a delegated scope, admin consent, and the ids handed over as variables.
2. **Entra security groups**: cluster admins (for AKS Entra RBAC) and product users/teams.
3. **A DNS name and its delegation** from the parent zone, and the certificate policy (is a public
   ACME CA allowed, or must certificates come from a KPMG CA).
4. **The network posture**: internet-facing with an IP allowlist (Panacea allowlists KPMG egress
   ranges, `P:deploy/main.tf:580-607`), or corporate-only through peering or ExpressRoute; and a
   VNet address space that does not overlap the corporate network.
5. **The data classification** (for example PROTECTED or not), which decides firewall, WAF,
   customer-managed keys and Defender, and a **monthly budget** with alert recipients.

### 7.3 Later, before real users

- Confirmation that sandboxes may reach Anthropic and the other providers from Azure egress IPs with
  users' own subscriptions (`docs/PROGRAM.md:138`, phase 6's risk).
- The Azure Policy assignments on the subscription, and exemptions if they block Kata SKUs,
  public IPs or the chosen CNI.
- Who is on call, and the log-retention period ISM requires.

---

## 8. Open questions, each with the recommendation the plan proceeds on

Rule 7 of this task: a genuine owner decision is recorded with a recommendation, and planning goes
ahead on the recommendation. **Owner** marks the ones only the owner can make; the rest are the
orchestrator's to rule.

| Id | Question | Recommendation | Who | Why |
|---|---|---|---|---|
| **OQ-1** | How does the desktop reach a Kata sandbox (G-2)? | An **in-cluster sandbox broker** running `katabe`, exposed through the ingress, authenticating the user's Entra token; the local control plane gains a `remote` backend that calls it. The broker **scopes every Get/List to the caller** — today `Manager.List` returns every sandbox in the namespace (`manager.go:240-246`) | Orchestrator, with the Hub ADR | The alternative — each user holds a kubeconfig and port-forwards — hands every user the cluster API. The broker keeps the control plane loopback-only (rule 6) |
| **OQ-2** | Where does the Hub run: AKS or Azure Container Apps? | **The same AKS**, on an `apps` pool | Orchestrator | The broker must be in-cluster anyway (OQ-1); one platform to run and secure |
| **OQ-3** | Sandbox egress: open, NAT with network policy, or an FQDN-allowlist firewall? | **Dev:** NAT gateway; sandboxes may reach the internet but not IMDS, the cluster or each other (G-4). **Prod:** Azure Firewall with a maintained agent allowlist (package registries, git hosts, model providers), if the classification requires it | **Owner** (classification, §7.2 item 5) | An allowlist that misses one registry breaks agents silently; an open egress may fail a PROTECTED assessment. Cost gap ~A$1.5–2k/month (`P:deploy/firewall.tf:35-37`) |
| **OQ-4** | A server-side identity broker (Keycloak) in front of Entra? | **None in v1**; the Hub validates Entra tokens | Hub ADR (owner decisions `:117-119`) | Removes Panacea's SSO outage class (`P:deploy/main.tf:930-1018`); customer BYO-IdP can add a broker later |
| **OQ-5** | Private cluster from day one? | **Prod: yes**, with the in-VNet pool. **Dev: public API with authorised IP ranges + Entra RBAC**, so the first environment is reachable while the pool is proven | Orchestrator | Private is create-time immutable (`P:deploy/main.tf:247-251`); dev can be rebuilt, prod cannot |
| **OQ-6** | How many environments at first? | **One (`dev`)**, with every stack parameterised by environment so `prod` is a second tfvars, never a copy | Orchestrator | Panacea's dev/prod confusion came from a shared RG name and an auto-loaded prod tfvars (`P:deploy/terraform.tfvars:1-12`). Rule: **no `terraform.tfvars` in any stack**, only named `<env>.tfvars` |

---

## 9. What this survey did not verify

- **Nothing was run against Azure.** No `terraform init`, `plan`, `validate` or `apply`, no `az`,
  no `kubectl`. Every statement about Panacea's *running* estate is what its code and comments say.
- **The AKS Pod Sandboxing facts** — the RuntimeClass name (G-3), CSI volume support (G-5), per-pod
  Kata overhead and allocatable CPU (§6.1), feature-registration needs — are from the repo's own
  unverified notes or training knowledge. The phase 8 plan's first cluster task must prove them.
- **Prices** (§6) were not fetched; only Panacea's in-code estimates are quoted.
- **Managed DevOps Pools** availability in KPMG's ADO organisation (§5).
- **Files listed but not opened:** `P:deploy/acr-cmk.tf` past its index lines, `P:deploy/outputs.tf`,
  `P:deploy/variables.tf`, `P:deploy/v2-hardening-vars.tf`, `P:deploy/pipelines/*.yaml`, the other
  four `P:azure-pipelines-*.yml`, and all but the phase headers of `P:deploy/cloud-shell-deploy.sh`
  and `P:deploy/panacea-deploy.sh`. How oauth2-proxy, CNPG, MinIO and Tekton are deployed was not
  read (§1.7, §1.9, §1.10).
- **`P:terraform/`** (the aspirational tree) was deliberately not read; `CLAUDE.md` warns it
  disagrees with the live tree.
- **Sibling worktrees** under `.claude/worktrees/` were not read. The Hub planner's ADR may decide
  OQ-1, OQ-2 and OQ-4 differently; where it does, it wins on the Hub's shape and this survey's
  infrastructure follows.
- **Secrets:** estate files were read for variable **names** and non-secret booleans only; no value
  that could be a credential was printed.


