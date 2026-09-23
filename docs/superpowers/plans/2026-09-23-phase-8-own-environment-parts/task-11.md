### Task 11: `deploy/charts/platform` — namespaces, Pod Security, priority, quotas, network policy

Kata isolates a sandbox's **kernel**; it does nothing for its **network** or its **API access**
(survey G-4, `:420`): today a sandbox pod could reach the metadata endpoint, the API server with a
default token, and every other sandbox. Task 14 fixes the pod spec; this chart makes the cluster
**refuse** a pod that regresses (Pod Security `restricted` enforced on `panacea-sandboxes`) and
**drop** the traffic a pod must never send (default-deny plus the few allows of P8-D12). It also
carries Panacea's scheduling lesson (survey §1.4, `:111`, `:114-117`): a priority ladder so a burst
of sessions can never preempt login.

Installed by the `deploy` pipeline before every other chart (task 30). Terraform never touches it
(P8-D3).

**Tooling.** Helm is not on the build laptop (`which helm` fails, 2026-09-23). Step 1 installs the
version pinned by task 28 (`scoop install helm@<pin>`; if scoop cannot reach GitHub through the
proxy, stop and report — do not vendor a binary into the repo), plus the `helm-unittest` plugin
(MIT) at its pinned version. `kubeconform` (Apache-2.0) runs in CI only.

**Files:**
- `deploy/charts/platform/Chart.yaml`, `values.yaml`, `values.schema.json` (new)
- `deploy/charts/platform/templates/{_helpers.tpl,namespaces.yaml,priorityclasses.yaml,quota.yaml,networkpolicy-defaults.yaml,networkpolicy-sandboxes.yaml,storageclasses.yaml}` (new)
- `deploy/charts/platform/tests/*_test.yaml` (new, helm-unittest)

#### Contract — `Chart.yaml` and `values.yaml` (complete)

```yaml
# Chart.yaml
apiVersion: v2
name: platform
description: PanaceaApp cluster baseline - namespaces, Pod Security, priority classes, quotas, network policy, storage classes.
type: application
version: 0.1.0
appVersion: "0.1.0"
```

```yaml
# values.yaml — every key is required by values.schema.json (additionalProperties: false)
namespaces:
  hub: panacea-hub
  broker: panacea-broker
  sandboxes: panacea-sandboxes
  edge: panacea-edge

network:
  # From foundation outputs, passed by the deploy pipeline. Empty fails the render.
  vnetCidr: ""
  podCidr: ""
  serviceCidr: ""
  privateEndpointsCidr: ""

sandboxes:
  # P8-D15. Must equal the broker's PANACEA_APP_RUNTIME_CLASS (task 29 passes both from one value).
  runtimeClass: kata-vm-isolation
  quota:                      # a ceiling for the whole namespace, not per user (the broker enforces per-user limits)
    requestsCpu: "64"
    requestsMemory: 128Gi
    pods: "64"
    persistentVolumeClaims: "64"
    requestsStorage: 4Ti
  limitRange:                 # the largest single container the cluster will admit
    maxCpu: "8"
    maxMemory: 32Gi

storage:
  machineDisk:
    name: panacea-machine-disk
    skuName: Premium_ZRS      # zone-redundant: a machine's disk can re-attach in another zone
  nfsFallback:                # P8-D6's fallback, off unless task 32 finds RWO sharing fails under Kata
    enabled: false
    name: panacea-machine-nfs

warm: {}                      # task 12 fills this block
```

`values.schema.json` is JSON Schema draft-07 generated from the block above: every key
`required`, every object `additionalProperties: false`, the four CIDRs `"pattern":
"^[0-9]{1,3}(\\.[0-9]{1,3}){3}/[0-9]{1,2}$"`, the SKU an enum of `Premium_LRS`, `Premium_ZRS`,
`StandardSSD_ZRS`. Helm validates values against it on `lint`, `template` and `install`.

#### What the templates render (described; each object is named here exactly)

`namespaces.yaml` — four `Namespace`s. Labels on each: `app.kubernetes.io/part-of: panacea-app`,
**`pod-security.kubernetes.io/enforce: restricted`**, `pod-security.kubernetes.io/enforce-version:
latest`, and the same for `audit` and `warn`. `panacea-hub` and `panacea-broker` also carry
`panacea-app/edge-routes: "true"`, the label task 13's Gateway admits routes from; no other
namespace carries it, so a route created in `panacea-sandboxes` is never attached to the edge. `restricted` on `panacea-sandboxes` is the point:
task 14's pod (non-root, no escalation, all capabilities dropped, `RuntimeDefault` seccomp,
volumes of type emptyDir / PVC / secret only) passes; a regression is rejected by the API server,
not merely missed by a review.

`priorityclasses.yaml` — four `PriorityClass`es, none `globalDefault`:

| Name | value | preemptionPolicy | Who |
|---|---|---|---|
| `panacea-platform-critical` | 1000000 | PreemptLowerPriority | Hub, broker, Traefik, cert-manager |
| `panacea-machine` | 100000 | PreemptLowerPriority | machine pods — they anchor a user's sessions (P8-D6) |
| `panacea-session` | 10000 | PreemptLowerPriority | session pods |
| `panacea-balloon` | -10 | **Never** | warm-capacity balloons (task 12): evicted first, never evict anything |

`quota.yaml` — `ResourceQuota/sandboxes` and `LimitRange/sandboxes` in `panacea-sandboxes` from
`sandboxes.quota` and `sandboxes.limitRange` (`type: Container`, `max`).

`networkpolicy-defaults.yaml` — in each of the four namespaces, `NetworkPolicy/default-deny`:
`podSelector: {}`, `policyTypes: [Ingress, Egress]`, no rules. This chart lays allows only in
`panacea-sandboxes` (below). Every other chart that puts a pod in a denied namespace brings its own
allows: `cert-manager` and `edge` in `panacea-edge` (task 13), `broker` and `hub` in theirs
(task 29). Because helm-unittest cannot see across charts, `deploy/tests/netpolcover` (task 13)
renders this chart with the others and fails if any workload in a denied namespace has no allow
(review C1: without it, `panacea-edge` was silently sealed).

`networkpolicy-sandboxes.yaml` — in `panacea-sandboxes` (standard `networking.k8s.io/v1`,
enforced by Cilium):
- `allow-dns` — egress from every pod to `kube-system` pods labelled `k8s-app: kube-dns`,
  UDP and TCP 53.
- `session-ingress-from-broker` — pods `panacea-app/component: sandbox` accept TCP **8080** only
  from pods `app.kubernetes.io/name: broker` in namespace `panacea-broker`.
- `machine-ingress-from-broker` — pods `panacea-app/component: machine` accept TCP **8081** only
  from the broker.
- `egress-internet` — every pod (sessions, machines, balloons) may send to `ipBlock 0.0.0.0/0`
  **except** `169.254.169.254/32` (IMDS), `168.63.129.16/32` (the Azure host's DNS and wire
  server), `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, and each of `network.*` if it lies
  outside those. No rule allows a sandbox to reach another pod, so sandbox↔sandbox and
  sandbox→Hub/broker traffic is denied by the default.

Cilium applies `ipBlock` to traffic leaving the cluster and matches in-cluster pods by identity,
so the pod-to-pod deny does not depend on the `except` list. **That this is how AKS's managed
Cilium treats `ipBlock` is from training knowledge, UNVERIFIED**; task 32 probes every deny from
inside a real Kata pod.

`storageclasses.yaml` — `StorageClass/panacea-machine-disk`: `provisioner: disk.csi.azure.com`,
`parameters.skuName`, `reclaimPolicy: Delete` (the broker deletes a machine's claim only on an
explicit machine delete, task 19), `volumeBindingMode: WaitForFirstConsumer`,
`allowVolumeExpansion: true`. And, when `storage.nfsFallback.enabled`,
`StorageClass/panacea-machine-nfs` on `file.csi.azure.com` with `protocol: nfs`,
`skuName: Premium_LRS`.

#### Tests to write first (helm-unittest suites in `tests/`)

| Suite / test | Asserts | Mutant it kills |
|---|---|---|
| `namespaces_test` / `sandboxes_enforce_restricted` | the `panacea-sandboxes` Namespace has `pod-security.kubernetes.io/enforce: restricted` | `baseline`, which admits a pod that keeps capabilities |
| `namespaces_test` / `four_namespaces` | exactly four Namespace documents with the four names from values | a namespace dropped (its default-deny with it) |
| `netpol_test` / `every_namespace_denies_by_default` | four `default-deny` policies, each `podSelector: {}` and both policy types | a default-deny covering Ingress only, leaving egress open |
| `netpol_test` / `imds_and_private_ranges_are_excluded` | `egress-internet`'s `except` contains `169.254.169.254/32`, `168.63.129.16/32` and the three RFC 1918 blocks | IMDS reachable — an agent reads the node's identity token |
| `netpol_test` / `sessions_accept_only_the_broker_on_8080` | `session-ingress-from-broker` has one `from` (the broker's labels in `panacea-broker`) and one port, 8080/TCP | a `from: []` that admits every pod |
| `netpol_test` / `machines_accept_only_the_broker_on_8081` | the same for 8081 and `component: machine` | a machine agent reachable from a session pod |
| `netpol_test` / `no_sandbox_to_sandbox_allow` | no policy in `panacea-sandboxes` has an egress `to` with a `podSelector` other than kube-dns | a convenience rule that lets sessions talk |
| `priority_test` / `ladder_is_ordered` | balloon < session < machine < platform-critical; balloon `preemptionPolicy: Never`; none `globalDefault` | a balloon that preempts a session, or a global default that silently prioritises everything |
| `storage_test` / `machine_disk_waits_for_its_pod` | `volumeBindingMode: WaitForFirstConsumer`, `provisioner: disk.csi.azure.com` | `Immediate`, which provisions the disk in a zone the pod may not schedule in |
| `storage_test` / `nfs_only_when_enabled` | default values render no `panacea-machine-nfs`; `enabled: true` renders it | the fallback always present |
| `values_test` / `empty_vnet_cidr_fails` | `network.vnetCidr: ""` → the render fails with the schema's message | a policy rendered with an empty `except` entry |

And in CI (task 28): `helm lint deploy/charts/platform -f <test values>` and
`helm template … | kubeconform -strict -summary`.

#### Steps

- [ ] **Step 1:** Install the pinned helm and helm-unittest; `helm version` prints the pin.
- [ ] **Step 2:** `Chart.yaml`, `values.yaml`, `values.schema.json`; the suites; `helm unittest`
  fails (no templates).
- [ ] **Step 3:** Templates one file at a time until every suite passes.
- [ ] **Step 4:** `helm lint` with a test-values file holding plausible CIDRs; commit.

#### Acceptance

```bash
helm lint deploy/charts/platform -f deploy/charts/platform/tests/values-test.yaml
# 1 chart(s) linted, 0 chart(s) failed
helm unittest deploy/charts/platform
# Charts:      1 passed, 1 total
# Test Suites: 5 passed, 5 total
# Tests:       11 passed, 11 total
```

#### What this task does not claim

- That AKS's Cilium enforces these policies on Kata pods exactly as on runc pods — **UNVERIFIED**,
  task 32.
- That `restricted` admits AKS's own add-on pods in these namespaces — none run there by design;
  any that do (a CSI helper, say) show up at install and are a task 31 finding.
- That `Premium_ZRS` managed disks are offered in the chosen region — **UNVERIFIED**; the value is
  one line to change.
