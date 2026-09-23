### Task 32: The Kata proving checklist — every Azure fact this plan assumed, as a check with a result slot

**Owner-gated.** This task writes a checklist and the scripts that measure it. It runs them only
when the environment exists, and the checks that need a person (a signed-in user, the KPMG network,
the window) wait for the owner's batched hands-on session (OD-12). Until a row has a result, it is
**UNVERIFIED**, and every report of phase 8 says so.

Survey §9 (`:582-584`) required it: "The AKS Pod Sandboxing facts — the RuntimeClass name (G-3),
CSI volume support (G-5), per-pod Kata overhead and allocatable CPU (§6.1), feature-registration
needs — are from the repo's own unverified notes or training knowledge. The phase 8 plan's first
cluster task must prove them." Each row names the plan decision that depends on it and **what
changes if it is false**, so a failed row is a known edit, not a redesign.

**Files:**
- `docs/verification/phase-8-kata-proving.md` — the checklist, every result `NOT RUN` (new)
- `infra/smoke/proving/{p01-runtimeclass.sh … p16-audit-volume.sh,p20-api-server-entity.sh,p21-webhook-source.sh,p22-ci-egress.sh}` and `proving_test.sh` (new)

#### The checklist

| Id | Fact assumed | Rests on it | How it is measured | If false |
|---|---|---|---|---|
| P01 | AKS installs RuntimeClass `kata-vm-isolation` for `KataVmIsolation` | P8-D15, task 14's default | `kubectl get runtimeclass -o name` | set `PANACEA_APP_RUNTIME_CLASS` (broker chart, platform chart) to what it printed; the code needs no change |
| P02 | A Kata pod runs its own kernel | the whole isolation claim (`CLAUDE.md`) | `uname -r` in the pod vs the node (smoke S50.1) | **stop**: the pool is not Kata; nothing proceeds |
| P03 | A machine pod and two session pods **on one node** mount sub-paths of **one RWO Azure Disk** at the same time; all three write; `git status` works on it | P8-D6 | `p03-shared-disk.sh`: the three pods with task 15's affinity, each writes and reads the others' files, git init/commit/status | switch to the NFS fallback: `storage.nfsFallback.enabled`, broker `MACHINE_STORAGE_CLASS=panacea-machine-nfs`, `MACHINE_AFFINITY=false` (tasks 11, 17, 29); measure P03 again on it |
| P04 | A sub-path the machine agent created (uid 1001, `0700`) is writable from the session; a missing one created by the kubelet is **not** (so task 24's `POST /v1/dirs` is necessary) | task 15's rule | `p04-subpath-owner.sh` | if `fsGroup` repairs root-created sub-paths, the dirs call stays (it is harmless) and the note in task 15 is corrected |
| P05 | A Secret-backed env var reaches the guest's process | task 14's token Secret | `env` in the pod shows the token | move the token back into the spec and record why |
| P06 | Cold start, four ways: (a) new node, (b) warm node without the image, (c) warm node with the pre-pulled image, (d) warm node by preempting a balloon — time from pod create to `/healthz` | G-6, tasks 06/12 floor and balloons | `p06-cold-start.sh`, 5 runs each, median and max | size `sandbox_pool.min_count` and `warm.balloons.replicas` from (c)/(d); if (b) ≈ (c), the pre-pull DaemonSet is removed (task 12's note) |
| P07 | Kata overhead and allocatable CPU: how many 2 CPU / 4 GiB sessions fit on one `Standard_D8s_v5` | cost model (§6.1 of the survey), the per-user cap | `p07-density.sh`: schedule sessions until Pending; read `kubectl describe node` allocatable and the RuntimeClass `overhead` | the cost model and `MAX_SANDBOXES_PER_USER` change; possibly the SKU (and `Standard_D8ds_v5` with an ephemeral OS disk is measured here too) |
| P08 | From inside a session pod: IMDS, the API server, another session's 8080, a machine pod's 8081, and the VNet are unreachable; DNS, `registry.npmjs.org`, `github.com`, `dev.azure.com` and `api.anthropic.com` are reachable | P8-D12, task 11 | `p08-isolation.sh`, each probe with a 3 s timeout | fix the NetworkPolicy — this row may not be `FAIL` at go-live |
| P09 | Cilium applies `ipBlock` to cluster-external traffic only and matches pods by identity | task 11's design note | covered by P08's pod-to-pod probes | rewrite task 11's policies as `CiliumNetworkPolicy` |
| P10 | An attach survives 30 minutes idle through Traefik, the Azure LB and the broker's 30 s pings | tasks 13, 18 | `p10-attach-soak.sh` through the edge with a test token | adjust Traefik timeouts / LB idle timeout / ping interval |
| P11 | A 64 KiB frame crosses laptop → broker → pod whole | the `MaxMessageBytes` rule's two new sites | part of the owner's session (P18) and a scripted `wscat`-style probe through the edge | find the missing `SetReadLimit` |
| P12 | `undrainable_node_behavior = "Cordon"` cordons (does not force-drain) a node whose machine a PDB protects, during a node-image upgrade | task 06, task 19's PDB | trigger a node-image upgrade in a maintenance window with a machine running | drop the PDB and accept session loss on upgrades, or pause auto-upgrade on the sandbox pool |
| P13 | Host encryption is on for all three pools | P8-D2 | `az aks nodepool show … --query enableEncryptionAtHost` | register the feature and recreate the pool (create-time) |
| P14 | `Premium_ZRS` disks exist in the region, and a machine disk re-attaches in another zone | task 11's storage class | delete a machine's node; wake the machine | `Premium_LRS` and accept zone-pinned machines |
| P15 | The Hub's workload-identity token reaches the pod with `automountServiceAccountToken: false` | task 29 | `/readyz` 200 (Postgres reached with an Entra token) | turn automount on for the Hub pod only |
| P16 | kube-audit-admin plus Container Insights stay under `log_daily_quota_gb` on a working day | task 04 | the workspace's `Usage` table after 24 h of use | raise the cap or move audit to a cheaper table |
| P17 | **(owner)** The laptop reaches `broker.<zone>` over WSS through the KPMG network, with or without `HTTPS_PROXY` | task 22 | the owner's session | document the proxy setting, or ask for a proxy bypass for the zone |
| P18 | **(owner)** End to end: sign in; the cloud machine appears (no create form); a Claude Code session on it; `/login` inside it with the user's own subscription (never pooled); a sleep; typing wakes it ("waking… Ns"); resume restores the conversation | tasks 19–26, SX.10, OD-12 | the owner's batched session | per finding |
| P19 | **(owner)** Anthropic accepts the user's own subscription from Azure egress IPs (`PROGRAM.md` open risk 2) | the product | part of P18 | phase 6's backstop (API keys) becomes the default |
| P20 | AKS-managed Cilium honours `toEntities: [kube-apiserver]`: a pod with only that egress allow reaches the API server and nothing else | tasks 13 and 29's `apiServer.mode: entity` (Traefik, cert-manager, broker) | `p20-api-server-entity.sh`: a throwaway pod in `panacea-edge` under the entity policy, `GET /version` via the `kubernetes` Service, then a probe to a non-API address; and, for the fallback, that a second pod with only an `ipBlock` for the `default/kubernetes` EndpointSlice's addresses reaches it too | set `apiServer.mode: cidr` in `deploy/values/<env>/{cert-manager,edge,broker}.yaml` — one value per chart, no chart change; `pipelines/scripts/api-server-cidrs.sh` (task 30) then passes the addresses as `apiServer.cidrs`, and the schemas refuse an empty list |
| P21 | The API server's calls to the cert-manager webhook are admitted by `fromEntities: [kube-apiserver]` (on AKS they arrive through the control-plane tunnel) | task 13's `cert-manager-webhook-ingress` | `p21-webhook-source.sh`: `kubectl create --dry-run=server` of a `ClusterIssuer`, which the webhook must validate; on failure, the Hubble/Cilium drop log names the source identity | `apiServer.mode: cidr` with `apiServer.webhookSourceCidrs` set to the source the drop log named, committed in `deploy/values/<env>/cert-manager.yaml` (a CIDR, not an id; the schema refuses an empty list) |
| P22 | The Managed DevOps Pool's agents in `snet-agents` register and complete a pipeline through the environment's egress: NAT in nat mode; in firewall mode `ci_agent_allowed_fqdns` (443) and `ci_agent_allowed_http_fqdns` (apt on 80) are enough, and the API server admits the egress /32 | review C2; tasks 03, 07, 30 | `p22-ci-egress.sh`: the first `infra-plan` run on `PANACEA_APP_CI_POOL` (runbook step 4's hand-over) succeeds; in firewall mode, the workspace's application-rule denies from the agents subnet's range over that run are listed, and must be none | add the host the deny log named to `ci_agent_allowed_fqdns`, or to `ci_agent_allowed_http_fqdns` if the deny was on port 80 (one host, never a wider wildcard); if the pool cannot take a NAT or a route table on its delegated subnet, the VMSS-agent fallback of task 30 |

#### Tests to write first — `proving_test.sh`

Like task 31's: each `pNN` script's pass/fail logic runs against canned output. Named cases:
`P03 fails when any pod cannot read another's file` (kills a "sharing works" that only checked
mounts), `P06 reports median and max, not a single run` (kills a lucky-sample cold start),
`P08 fails on the first reachable forbidden target and names it`, `P07 prints sessions per node
and the overhead it read`.

#### Steps

- [ ] **Step 1:** `proving_test.sh`; the scripts until it passes; shellcheck clean.
- [ ] **Step 2:** The checklist document, every result cell `NOT RUN`, every row citing the task it
  would change.
- [ ] **Step 3:** Commit.

#### Acceptance

```bash
bash infra/smoke/proving/proving_test.sh     # proving_test: all cases passed
grep -c "NOT RUN" docs/verification/phase-8-kata-proving.md     # 22
```

#### What this task does not claim

Everything in the table, until it is run.
