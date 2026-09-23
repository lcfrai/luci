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

### 1.6 Egress — Azure Firewall, off in the live estate

`P:deploy/firewall.tf`, every resource `count = var.egress_firewall_enabled ? 1 : 0`, "the live
innovation env … egress_firewall_enabled = false" (`:27-33`). When on: `AzureFirewallSubnet`
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
