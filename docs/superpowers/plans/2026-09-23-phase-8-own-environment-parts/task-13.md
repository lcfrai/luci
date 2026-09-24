### Task 13: `deploy/charts/cert-manager` and `deploy/charts/edge` — cert-manager, then Traefik on the Gateway API and one Gateway

The edge is where a user's token enters the environment, so it is small and fails closed: one
public IP owned by Terraform (task 07), one Traefik deployment implementing the Kubernetes Gateway
API (P8-D9), one Gateway with one HTTPS listener, certificates from cert-manager over DNS-01 with a
federated identity (no secret), and a source-range allowlist that is **required** (owner question
Q3). HTTPRoutes are not here: the Hub and broker charts bring their own (task 29) into namespaces
the Gateway admits by label (task 11).

**Two releases, in this order** (review C1). cert-manager is its own release, installed and
waited on **before** `edge`, because the edge's `ClusterIssuer` and `Certificate` are
`cert-manager.io/v1` objects: in one release, Helm cannot map a kind whose CRD the same install is
creating, and even with the CRDs present the `ClusterIssuer` create is sent to a webhook that is
not yet serving. As separate releases, `helm upgrade --install --atomic --wait cert-manager` does
not return until the webhook answers (the upstream chart's `startupapicheck` hook), so `edge`
finds both the CRDs and a live webhook. Task 30's stage 4 order is `platform` → `cert-manager` →
`edge` → `broker` → `hub`.

**Both charts carry their own NetworkPolicy** (review C1). Task 11 lays `default-deny` (Ingress
and Egress) in `panacea-edge` like every other namespace, so a pod in it can do nothing a policy of
its own chart does not allow. Without these, Traefik receives nothing from the load balancer and
cannot watch the API server, and cert-manager cannot reach the API server, Let's Encrypt, Azure DNS
or Entra. A cross-chart render test (`deploy/tests/netpolcover`, below) fails if any workload in a
default-denied namespace has no allow, because helm-unittest renders one chart at a time and cannot
see a policy another chart lays down.

Panacea's edge declares its IP allowlist in the release because a live `kubectl patch` was being
reverted (survey §1.5, `:129-131`) and keeps `externalTrafficPolicy: Local` so the allowlist sees
client IPs (`:131-132`). Both carry over, written from scratch.

**Files:**
- `deploy/charts/cert-manager/Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json` (new) — a thin wrapper: the upstream chart as its one dependency, plus this repo's `templates/{serviceaccount.yaml,networkpolicy.yaml}` (new)
- `deploy/charts/cert-manager/tests/*_test.yaml` (new)
- `deploy/charts/edge/Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json` (new)
- `deploy/charts/edge/crds/gateway-api-standard-<version>.yaml` — the upstream Gateway API standard CRDs, vendored unmodified (Apache-2.0), with a `crds/README.md` naming the version and its source URL (new)
- `deploy/charts/edge/templates/{gateway.yaml,certificate.yaml,clusterissuer.yaml,pdb.yaml,networkpolicy.yaml}` (new)
- `deploy/charts/edge/tests/*_test.yaml` (new)
- `deploy/tests/netpolcover/{go.mod,go.sum,netpolcover_test.go,testdata/backends.yaml,testdata/unsized-kinds.yaml}` — the cross-chart NetworkPolicy coverage test (`unsized-kinds.yaml` is the negative fixture of `every_sandbox_container_is_sized`, b05 re-review 8, and stays when task 29 deletes `backends.yaml`), its own Go module (`go 1.23`; one dependency, `sigs.k8s.io/yaml`, already in the control plane's module graph) (new)
- `go.work` — `use ./deploy/tests/netpolcover` (modify)
- `licences/charts.yaml` — one row per `Chart.lock` entry, the OD-11 gate's Helm half (review m6) (modify, task 28)
- `pipelines/charts.yml` — one job after the per-chart jobs: Go 1.23 (`GoTool@0`), `helm` at task 28's pin, `go test ./deploy/tests/netpolcover/... -count=1`. The test's `TestMain` runs `helm dependency build` for every chart it renders, so the job works on a fresh agent with no `charts/*.tgz` (review M1) (modify, task 28)

#### Contract — `deploy/charts/cert-manager/Chart.yaml` (versions pinned exactly at implementation; no ranges)

```yaml
apiVersion: v2
name: cert-manager
description: PanaceaApp cert-manager - the upstream chart, its federated service account and its network policy. Installed before edge.
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: cert-manager
    repository: https://charts.jetstack.io
    version: "<exact, pinned in step 1>"     # licence: licences/charts.yaml (task 13 step 1)
    alias: certManager                       # a Go-template-friendly values key; the names below are pinned lowercase
```

**The alias renames the subchart** (review I4). Helm sets an aliased subchart's `.Chart.Name` to
the alias, and the upstream `cert-manager.fullname` helper builds every resource name from it, so
`alias: certManager` alone renders `cert-manager-certManager`, `cert-manager-certManager-webhook`
and so on. Uppercase is not a DNS-1123 name: the API server refuses them, and `helm template`,
helm-unittest and `kubeconform` do not check the pattern. The values contract therefore sets
`certManager.fullnameOverride: cert-manager` (names read `cert-manager`, `cert-manager-webhook`,
`cert-manager-cainjector`, `cert-manager-startupapicheck`) and `certManager.nameOverride:
cert-manager` (the `app.kubernetes.io/name` and `app` labels read `cert-manager`, not
`certManager`). **The helper's behaviour and both override keys are from training knowledge of the
cert-manager chart, UNVERIFIED**; `netpolcover`'s `rendered_names_are_dns_1123` renders the real
subchart and fails on any uppercase name, whatever the helper does.

#### Contract — `deploy/charts/cert-manager/values.yaml` (complete)

```yaml
identity:
  clientId: ""                   # workloads output cert_manager_client_id; schema minLength 1
apiServer:                       # the same block, same keys, in the edge and broker charts (task 29)
  mode: entity                   # entity: CiliumNetworkPolicy toEntities/fromEntities kube-apiserver; cidr: ipBlock below
  cidrs: []                      # cidr mode only; schema minItems 1 then. Passed by deploy: pipelines/scripts/api-server-cidrs.sh (task 30)
  webhookSourceCidrs: []         # cidr mode only; schema minItems 1 then. Committed in deploy/values/<env>/cert-manager.yaml from task 32's P20 finding

certManager:
  fullnameOverride: cert-manager # review I4: without it the alias puts "certManager" into every resource name
  nameOverride: cert-manager     # the same, for the app.kubernetes.io/name label the policies and tests read
  namespace: panacea-edge
  crds:
    enabled: true
    keep: true                   # uninstalling the release never deletes every Certificate in the cluster
  global:
    priorityClassName: panacea-platform-critical
  prometheus:
    enabled: false               # nothing in this environment scrapes it; its metrics Service would need an ingress allow nobody uses (netpolcover counts Service-backed pods)
  nodeSelector:
    panacea-app/role: apps
  podLabels:
    azure.workload.identity/use: "true"
  serviceAccount:
    create: false                # templates/serviceaccount.yaml renders it, with the client id
    name: cert-manager           # must equal task 10's namespaces.cert_manager / service account
  webhook:
    securePort: 10250            # one value: the webhook's container port and the policy's ingress port
    nodeSelector:
      panacea-app/role: apps
  cainjector:
    nodeSelector:
      panacea-app/role: apps
  startupapicheck:
    enabled: true                # the post-install hook that makes --wait mean "the webhook answers"
    nodeSelector:
      panacea-app/role: apps
```

**The upstream key names (`fullnameOverride`, `nameOverride`, `global.priorityClassName`,
`prometheus.enabled`, `webhook.securePort`, `crds.keep`, `startupapicheck.enabled`, the
per-component `nodeSelector`s) and the pod labels the policy selects
on are from training knowledge of the cert-manager chart, UNVERIFIED**; the tests assert the
rendered pods, not the keys, and `netpolcover` matches every policy selector against the pods the
upstream templates actually render.

**The `apiServer` seam fails closed, never open** (review I3). Kubernetes reads an egress rule
with no `to` peer, or an ingress rule with no `from` peer, as "every destination" or "every
source". A template that writes one rule and fills its peers with `range` over an empty list would
turn `mode: cidr` with `cidrs: []` into 443 to anywhere, and `webhookSourceCidrs: []` into a
webhook open to every pod. Three guards, each tested:
- **Schema.** `values.schema.json` holds `if: {properties: {apiServer: {properties: {mode:
  {const: cidr}}}}}` / `then:` `apiServer.cidrs` and `apiServer.webhookSourceCidrs` `required`,
  `minItems: 1`, each item a CIDR `pattern`. The edge and broker schemas carry the same rule for
  `cidrs` (JSON Schema draft 7 `if`/`then`, which Helm 3's validator supports — training knowledge,
  UNVERIFIED; the `cidr_mode_with_an_empty_list_is_refused` runs prove it either way).
- **Template.** Each peer list is guarded with `{{- if not .Values.apiServer.cidrs }}{{ fail
  "apiServer.cidrs is empty in cidr mode" }}{{- end }}` before the rule, so a schema that is ever
  loosened still does not render an open rule.
- **Cross-chart.** `netpolcover`'s `no_allow_rule_is_peerless` fails on any rendered rule with no
  peer, in any chart.

**Where the values come from** (review I3). `apiServer.mode` and `webhookSourceCidrs` are non-id
settings, committed per environment in `deploy/values/<env>/{cert-manager,edge,broker}.yaml`
(task 29 step 4). `apiServer.cidrs` is looked up at deploy time, because Terraform does not export
the API server's address and it can change: task 30's `pipelines/scripts/api-server-cidrs.sh`
reads the addresses of `EndpointSlice`s labelled `kubernetes.io/service-name=kubernetes` in
`default`, after `get-credentials`, and emits one `--set apiServer.cidrs={<ip>/32,…}` for each of
the three charts, only when that environment's values set `mode: cidr`. **That the `kubernetes`
EndpointSlice holds the address Cilium sees for API-server traffic on AKS is training knowledge,
UNVERIFIED**; P20 (task 32) checks it before `cidr` mode is chosen.

`templates/serviceaccount.yaml` — `ServiceAccount/cert-manager` in `panacea-edge` with
`azure.workload.identity/client-id: {{ .Values.identity.clientId }}`; the upstream chart's RBAC
binds to it by name.

`templates/networkpolicy.yaml` — in `panacea-edge`, on top of task 11's `default-deny`, each
policy selecting cert-manager pods by `app.kubernetes.io/instance` and `app.kubernetes.io/component`:
- `cert-manager-dns` — egress from every cert-manager pod to `kube-system` pods `k8s-app: kube-dns`,
  UDP and TCP 53.
- `cert-manager-api-server` — egress from the controller, cainjector, webhook and startupapicheck
  to the API server: in `entity` mode a `CiliumNetworkPolicy` with `toEntities: [kube-apiserver]`,
  in `cidr` mode an `ipBlock` per `apiServer.cidrs` on 443. **AKS-managed Cilium's support for the
  `kube-apiserver` entity is UNVERIFIED** (the same caveat as task 29); task 32 probes it.
- `cert-manager-controller-egress` — the controller only, to `0.0.0.0/0` **except**
  `169.254.169.254/32`, `168.63.129.16/32` and the three RFC 1918 blocks: TCP 443 (Let's Encrypt,
  the Azure DNS API, Entra's token endpoint for the federated credential) and UDP/TCP 53 (the DNS-01
  propagation self-check queries the zone's authoritative nameservers directly — from training
  knowledge of cert-manager, UNVERIFIED; if it proves false, the 53 rule is deleted, never widened).
  Workload identity needs no IMDS, so IMDS stays excluded.
- `cert-manager-webhook-ingress` — the webhook accepts TCP `webhook.securePort` only from the API
  server: `fromEntities: [kube-apiserver]` in `entity` mode, `ipBlock` per
  `apiServer.webhookSourceCidrs` in `cidr` mode. **UNVERIFIED and task 32's to settle:** on AKS the
  control plane reaches an in-cluster webhook through a tunnel, and which source identity Cilium
  sees for that traffic is not known from here. A wrong guess fails closed and visibly:
  `startupapicheck` fails, `--atomic` rolls the release back, and stage 4 stops before `edge`.

#### Contract — `deploy/charts/edge/Chart.yaml` (versions pinned exactly at implementation; no ranges)

```yaml
apiVersion: v2
name: edge
description: PanaceaApp edge - Traefik (Gateway API), the Gateway, its certificate and issuer, its network policy. Needs the cert-manager release first.
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: traefik
    repository: https://traefik.github.io/charts
    version: "<exact, pinned in step 1>"     # licence: licences/charts.yaml (task 13 step 1)
```

#### Contract — `deploy/charts/edge/values.yaml` (complete)

```yaml
# Everything empty here is passed by the deploy pipeline from Terraform outputs; the schema
# refuses an empty value, so a render with a missing input fails instead of half-working.
zone: ""                         # foundation dns_zone_name
edgeIp:
  name: ""                       # foundation edge_public_ip_name
  resourceGroup: ""              # foundation resource_group_name
allowedSourceRanges: []          # owner question Q3; schema minItems 1 — an empty allowlist does not render
lbIdleTimeoutMinutes: 30         # Azure LB drops idle TCP at 4 minutes by default; attach sockets idle longer
acme:
  server: https://acme-v02.api.letsencrypt.org/directory   # owner question Q2
  email: ""
  subscriptionId: ""             # the zone's subscription; pipeline --set, never committed
  zoneResourceGroup: ""          # foundation resource_group_name
certManagerIdentity:
  clientId: ""                   # workloads output cert_manager_client_id
routeNamespaceLabel:
  key: panacea-app/edge-routes
  value: "true"
apiServer:                       # the same seam as the cert-manager chart and task 29
  mode: entity                   # entity | cidr
  cidrs: []                      # cidr mode only; schema minItems 1 then; passed by api-server-cidrs.sh (task 30)
backends:                        # schema minItems 1 (an empty list must not render a peerless egress rule); each must match a policy task 29 lays in that namespace
  - namespace: panacea-broker
    podLabels: {app.kubernetes.io/name: broker}
    port: 8080
  - namespace: panacea-hub
    podLabels: {app.kubernetes.io/name: hub}
    port: 8080

traefik:
  global:
    checkNewVersion: false       # the edge's egress policy denies it anyway; do not try
    sendAnonymousUsage: false
  deployment:
    replicas: 2
  priorityClassName: panacea-platform-critical
  nodeSelector:
    panacea-app/role: apps
  providers:
    kubernetesGateway:
      enabled: true
    kubernetesIngress:
      enabled: false
    kubernetesCRD:
      enabled: false
  gateway:
    enabled: false               # this chart renders its own Gateway (templates/gateway.yaml)
  ports:
    websecure:
      transport:
        respondingTimeouts:
          readTimeout: 0s        # a WebSocket attach is one long request
          idleTimeout: 3600s
  service:
    type: LoadBalancer
    spec:
      externalTrafficPolicy: Local
  logs:
    access:
      enabled: true
```

The Service's annotations and `loadBalancerSourceRanges` are set from the top-level values by
the parent chart's values (`traefik.service.annotations`, `traefik.service.spec.loadBalancerSourceRanges`
templated through `tpl` in a small wrapper, or passed by the pipeline) so that there is one
source for each: `service.beta.kubernetes.io/azure-pip-name: {{ .Values.edgeIp.name }}`,
`service.beta.kubernetes.io/azure-load-balancer-resource-group: {{ .Values.edgeIp.resourceGroup }}`,
`service.beta.kubernetes.io/azure-load-balancer-tcp-idle-timeout: "{{ .Values.lbIdleTimeoutMinutes }}"`.
**The three annotation names are from training knowledge of the Azure cloud provider,
UNVERIFIED**; the test pins them and task 31's smoke proves the IP binds. `certManagerIdentity.clientId`
feeds the `ClusterIssuer`'s `managedIdentity.clientID` only; the cert-manager service account and
its `azure.workload.identity/client-id` annotation belong to the cert-manager release
(`identity.clientId`), and the deploy pipeline passes the one Terraform output to both (task 10's
`helm_values`). The Traefik chart's `global.checkNewVersion` / `global.sendAnonymousUsage` key
names are from training knowledge, UNVERIFIED; the test asserts the rendered arguments.

#### Templates

- `gateway.yaml` — `Gateway/panacea` in `panacea-edge`, `gatewayClassName: traefik`, **one**
  listener: `name: https`, `protocol: HTTPS`, `port: 8443` (Traefik's websecure entrypoint; the
  Service maps 443 → 8443), `hostname: "*.{{ .Values.zone }}"`, `tls.mode: Terminate`,
  `tls.certificateRefs: [{name: edge-tls}]`, `allowedRoutes.namespaces.from: Selector` with
  `matchLabels` from `routeNamespaceLabel`. **No port-80 listener**: DNS-01 needs none and nothing
  should be served in the clear.
- `certificate.yaml` — `Certificate/edge-tls` in `panacea-edge`, `secretName: edge-tls`,
  `dnsNames: ["*.{{ zone }}", "{{ zone }}"]`, `issuerRef: {kind: ClusterIssuer, name: acme}`,
  `privateKey: {algorithm: ECDSA, size: 256, rotationPolicy: Always}`.
- `clusterissuer.yaml` — `ClusterIssuer/acme`: `acme.server`, `acme.email`,
  `privateKeySecretRef: {name: acme-account}`, one `dns01` solver `azureDNS { subscriptionID,
  resourceGroupName, hostedZoneName, environment: AzurePublicCloud, managedIdentity: { clientID } }`.
  **No `clientSecretSecretRef`.**
- `pdb.yaml` — `PodDisruptionBudget/traefik`, `minAvailable: 1`.
- `networkpolicy.yaml` — in `panacea-edge`, on top of task 11's `default-deny`, each policy
  selecting the Traefik pods by `app.kubernetes.io/name: traefik` and
  `app.kubernetes.io/instance` (the upstream chart's labels, from training knowledge, UNVERIFIED —
  `netpolcover` matches them against the rendered Deployment):
  - `traefik-ingress` — TCP **8443** only, `from` one `ipBlock` per `allowedSourceRanges` entry and
    nothing else. The client IP survives to the pod because the Service is
    `externalTrafficPolicy: Local`, so this is the same allowlist enforced a second time, at the pod.
    The load balancer's health probe goes to the node's `healthCheckNodePort`, not the pod (training
    knowledge of `Local` services, UNVERIFIED; task 31's smoke sees it).
  - `traefik-backends` — egress TCP to each `backends` entry: `namespaceSelector`
    `kubernetes.io/metadata.name: <namespace>` **and** `podSelector` `<podLabels>` in one peer, on
    `<port>`. Nothing else in-cluster: Traefik routes to endpoint IPs it reads from the API server,
    so it needs no DNS.
  - `traefik-api-server` — egress to the API server (it watches Gateways, HTTPRoutes, Services,
    EndpointSlices and the `edge-tls` Secret): `toEntities: [kube-apiserver]` in `entity` mode, an
    `ipBlock` per `apiServer.cidrs` on 443 in `cidr` mode; the same seam and caveat as the
    cert-manager chart and task 29.
  - No egress to the internet. Traefik never originates a connection outward.

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `service_binds_the_terraform_ip` | the Traefik Service carries the pip-name and resource-group annotations from values | Azure allocating a fresh IP the A records do not point at |
| `service_keeps_client_ips_and_the_allowlist` | `externalTrafficPolicy: Local`; `loadBalancerSourceRanges` equals `allowedSourceRanges` | `Cluster`, which SNATs clients so an allowlist sees only node IPs |
| `empty_allowlist_does_not_render` | `allowedSourceRanges: []` → render fails with the schema message | an edge open to the internet by omission |
| `idle_timeout_is_raised` | the tcp-idle-timeout annotation is `"30"` | attach sockets cut at 4 minutes of quiet |
| `one_https_listener_no_http` | the Gateway has exactly one listener, HTTPS, TLS Terminate, `edge-tls` | a plaintext listener |
| `routes_only_from_labelled_namespaces` | `allowedRoutes.namespaces.from: Selector` with the label | `from: All`, which lets a route in any namespace (a sandbox's) publish itself |
| `issuer_uses_workload_identity_not_a_secret` | the solver has `managedIdentity.clientID` and no `clientSecretSecretRef` | a service-principal secret in the cluster |
| `dependencies_are_pinned_exactly` | a test reads `Chart.yaml`: every dependency `version` matches `^[0-9]+\.[0-9]+\.[0-9]+$` | a `~` range that changes the edge on a rebuild |
| `edge_runs_on_apps_at_platform_priority` | Traefik `nodeSelector` apps and `priorityClassName: panacea-platform-critical` | the edge preempted by a session burst |
| `edge_does_not_bundle_cert_manager` | `Chart.yaml` has exactly one dependency, `traefik`; no rendered document is a cert-manager Deployment or CRD | cert-manager folded back into `edge`, racing its own `ClusterIssuer` (C1) |
| `traefik_ingress_is_the_allowlist_on_8443` | `traefik-ingress` has one port, 8443/TCP, and its `from` is exactly one `ipBlock` per `allowedSourceRanges` entry | a `from: []` or `0.0.0.0/0` that makes the allowlist a load-balancer-only control |
| `traefik_egress_is_backends_and_api_server_only` | `traefik-backends` has one peer per `backends` entry, each namespace **and** pod selector on its port; `traefik-api-server` renders `toEntities: [kube-apiserver]` by default and one `ipBlock` per `apiServer.cidrs` with `mode: cidr`; no `ipBlock` `0.0.0.0/0` in any edge policy | a Traefik that can reach sandboxes or the internet |
| `cidr_mode_with_an_empty_list_is_refused` | `apiServer.mode: cidr` with `cidrs: []` → render fails with the schema message; `backends: []` → render fails the same way | a peerless egress rule: 443 to anywhere, or every port to anywhere (review I3) |

`deploy/charts/cert-manager/tests/`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `cert_manager_is_federated` | the controller pod label `azure.workload.identity/use: "true"`; `ServiceAccount/cert-manager` carries `azure.workload.identity/client-id` from `identity.clientId`; the controller runs as it | a federated credential nobody presents |
| `empty_client_id_does_not_render` | `identity.clientId: ""` → render fails with the schema message | cert-manager started with no identity, failing only at the first DNS-01 challenge |
| `cert_manager_runs_on_apps_at_platform_priority` | the controller, webhook, cainjector and startupapicheck pods: `nodeSelector` apps, `priorityClassName: panacea-platform-critical` | a cert-manager component preempted by a session burst |
| `crds_install_and_are_kept` | CRDs render with `helm.sh/resource-policy: keep` | an uninstall that deletes every `Certificate` |
| `startupapicheck_is_on` | the post-install hook Job renders | `--wait` returning before the webhook serves, so `edge` races it again |
| `webhook_ingress_only_from_the_api_server` | `cert-manager-webhook-ingress` selects only the webhook, one port equal to `webhook.securePort` and to the webhook container's port, `fromEntities: [kube-apiserver]` by default, `ipBlock`s from `webhookSourceCidrs` in `cidr` mode | a webhook open to every pod, or a policy port that drifted from the container port |
| `every_component_reaches_dns_and_the_api_server` | `cert-manager-dns` selects every cert-manager pod (controller, cainjector, webhook, startupapicheck) with egress to `kube-system` `k8s-app: kube-dns` on UDP and TCP 53; `cert-manager-api-server` selects the same four, `toEntities: [kube-apiserver]` by default and one `ipBlock` per `apiServer.cidrs` on 443 in `cidr` mode (review I2) | either policy deleted or narrowed: every component still has *an* egress allow, so `netpolcover` stays green, and on a cluster cert-manager is sealed again and rolled back at `--atomic` (C1) |
| `cidr_mode_with_an_empty_list_is_refused` | `apiServer.mode: cidr` with `cidrs: []` → render fails; with `cidrs` set and `webhookSourceCidrs: []` → render fails; both with the schema message (review I3) | a peerless rule: controller egress to anywhere on 443, or the webhook open to every pod |
| `controller_egress_excludes_imds_and_private_ranges` | `cert-manager-controller-egress` selects only the controller; ports 443/TCP and 53/UDP+TCP; `except` holds `169.254.169.254/32`, `168.63.129.16/32` and the three RFC 1918 blocks | the node's identity token readable from cert-manager |
| `dependencies_are_pinned_exactly` | as the edge's | as the edge's |

`deploy/tests/netpolcover` (Go, `go test ./deploy/tests/netpolcover/... -count=1`; needs `helm` on
`PATH` and **fails**, never skips, without it). Its `TestMain` first runs `helm dependency build`
for every chart it renders (review M1: `charts/*.tgz` is git-ignored, so a fresh agent has no
subcharts and `helm template` would fail with "missing in charts/ directory"). It then runs
`helm template <release> deploy/charts/<chart> --namespace <ns>` on `platform`, `cert-manager` and
`edge` with each chart's `tests/values-test.yaml` (hooks included), **using the release names and
namespaces of task 30's stage 4** (review M2: `cert-manager` and `edge` in `panacea-edge`; the
Traefik chart renders into `.Release.Namespace`, so without `--namespace` it lands in `default`,
which has no `default-deny`, and the coverage cases pass it unchecked). The release → namespace
list is one Go table, a copy of task 30's stage 4 table; its doc comment names `pipelines/deploy.yml`
as the source, and a change to either is made to both in one commit. It parses every
document, records each hook's `helm.sh/hook` phase and `hook-weight`, and asserts across charts:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `every_denied_workload_has_an_allow` | for every namespace where `platform` renders `default-deny`, every pod template rendered into it by any chart (Deployment, StatefulSet, DaemonSet, Job, hooks included) is selected by an allow **in each direction it needs**, counted separately (review I2): an **Egress** allow (a `NetworkPolicy` with `Egress` in `policyTypes` and a rule, or a `CiliumNetworkPolicy` with `egress`) for every pod; an **Ingress** allow for every pod selected by a rendered `Service` or behind a `Validating`/`MutatingWebhookConfiguration`. `default-deny` never counts | **C1 itself**: an edge chart with no policy, silenced by `platform`; and a pod with an ingress allow but no egress allow (or the reverse), which a one-direction count passes |
| `every_workload_is_in_a_denied_namespace` | every pod template rendered by any release sits in a namespace where `platform` renders `default-deny` (the test's exemption list is empty) (review M2) | a release rendered without `--namespace`: its pods land in `default` and every coverage case passes them unchecked |
| `hook_pods_are_allowed_before_they_run` | for every `pre-install` / `pre-upgrade` hook pod (review I1): its `ServiceAccount` and every allow that satisfies `every_denied_workload_has_an_allow` for it is either a hook of the same phase with a **lower** `hook-weight`, or rendered by a release earlier in stage 4's order. A `post-install` hook may rely on the release's regular templates (Helm loads them first) | a pre-install hook whose allow or service account is a regular template of its own release: static render green, first install sealed by `default-deny` (the Hub's migrate Job, task 29) |
| `no_allow_rule_is_peerless` | every `ingress` rule of an allow policy has a non-empty `from` (or `fromEntities`), every `egress` rule a non-empty `to` (or `toEntities`) (review I3) | an empty list in `cidr` mode, or `backends: []`, rendered as "allow every source/destination" |
| `every_selector_matches_a_pod` | every allow policy's `podSelector` / `endpointSelector` matches at least one rendered pod template in its namespace | a policy written against labels the upstream chart does not render (it passes helm-unittest and allows nothing) |
| `the_webhook_is_reachable` | a policy admits ingress to the pods behind the `ValidatingWebhookConfiguration`'s service, on that service's `targetPort`. The upstream webhook Service targets a **named** port (`https`, training knowledge, UNVERIFIED), so the test resolves a named `targetPort` through the selected pod's `containers[].ports[].name` to its `containerPort` before comparing; a name that resolves to nothing fails (review M3) | the API server unable to call the webhook, so every `ClusterIssuer` create fails |
| `traefik_reaches_every_routed_backend` | for each `backends` entry: the Traefik egress peer matches a rendered pod, and a policy in that namespace admits ingress from Traefik's pod labels in `panacea-edge` on that port | label drift between the edge and the broker/Hub charts: a front door to a wall |
| `rendered_names_are_dns_1123` | every rendered document's `metadata.name` in the `cert-manager` and `edge` releases (both wrapper charts, their subcharts included), and later in all five, matches `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$` and is at most 253 characters; a `Service`'s name at most 63 and without `.` (review I4) | `alias: certManager` without `fullnameOverride`: `cert-manager-certManager-webhook`, which the API server refuses at the first install while `helm template`, helm-unittest and `kubeconform` pass it |
| `every_sandbox_container_is_sized` | every container **and init container** of every pod template rendered into `panacea-sandboxes` by any release (today `platform`'s balloon Deployment and pre-pull DaemonSet; the walk also covers a CronJob's `jobTemplate`, a `Job` and a bare `Pod`, hooks included, so a new kind cannot slip past; no release renders those kinds there today, so the walk's reach is proven by `testdata/unsized-kinds.yaml`: an unsized CronJob, an unsized Job and an unsized bare Pod, each in `panacea-sandboxes`, which the case's check must report one by one, by kind and name, in a sub-test that fails if any of the three goes unreported, b05 re-review 8) sets `resources.requests` and `resources.limits` for both `cpu` and `memory`, and requests equal limits, compared as the rendered strings (each template renders both from one value, so they are identical; `1000m` against `1` fails, the stricter reading, and it keeps the module's one dependency) (review I4 of the plan review, the pre-pull sizing — not the re-review's I4 above; the Kata rule of `PROGRAM.md:133`) | an unsized pre-pull init container: LimitRanger gives it the namespace default, the pod asks for a session (or, with only a `max`, a whole node) on every sandbox node and holds the namespace quota; or a balloon with requests ≠ limits, which Kata sizes wrongly |

Task 29 adds `broker` and `hub` to the chart list, at which point `traefik_reaches_every_routed_backend`
also checks every rendered `HTTPRoute` `backendRef` against `backends`; until then
`deploy/tests/netpolcover/testdata/backends.yaml` stands in for them (a Deployment and an
ingress NetworkPolicy per backend, written from task 29's contract), and task 29 deletes it.

#### Steps

- [ ] **Step 1:** Choose the current Traefik chart (v3 line) and cert-manager chart releases;
  `helm dependency build` for both charts; commit both `Chart.lock`s (never the downloaded
  `charts/*.tgz`, which `.gitignore` gains). Read each chart's licence from its `LICENSE` file
  (expected MIT for Traefik and Apache-2.0 for cert-manager, UNVERIFIED until read) and add one
  row per `Chart.lock` entry to task 28's `licences/charts.yaml`: name, repository, version,
  SPDX licence, source URL. `bash pipelines/scripts/chart-licences.sh` must first fail naming
  both entries, then pass with `2 chart dependencies` (review m6). Each `Chart.yaml` comment
  points at that file rather than restating the licence. Download the Gateway API standard CRDs of the version
  the chosen Traefik release supports; vendor unmodified into `edge/crds/`.
- [ ] **Step 2:** Values, schemas, suites for both charts; the suites fail.
- [ ] **Step 3:** `deploy/tests/netpolcover` and its `testdata/`; add it to `go.work`; run it
  against `platform` alone plus the two charts with **no** `networkpolicy.yaml` yet:
  `every_denied_workload_has_an_allow` fails naming Traefik and every cert-manager component. That
  red run is C1 reproduced; keep its output in the task report.
- [ ] **Step 4:** Templates until every suite and `netpolcover` pass; `helm lint` both with test
  values.
- [ ] **Step 5:** Mutation check on a `$TEMP` copy, each verified applied and the named test red:
  - delete `edge/templates/networkpolicy.yaml` → `every_denied_workload_has_an_allow`;
  - change the Traefik policy's pod label → `every_selector_matches_a_pod`;
  - change `webhook.securePort` in the policy only → `the_webhook_is_reachable`,
    `webhook_ingress_only_from_the_api_server`;
  - add `cert-manager` back as an `edge` dependency → `edge_does_not_bundle_cert_manager`;
  - delete `cert-manager-api-server` → `every_component_reaches_dns_and_the_api_server` (review I2:
    `netpolcover` stays green, because every pod keeps an egress allow; record that it does);
  - delete `cert-manager-dns` → `every_component_reaches_dns_and_the_api_server` (the same);
  - drop `policyTypes: [Egress]` and the egress rules from `traefik-api-server` and
    `traefik-backends`, leaving `traefik-ingress` → `every_denied_workload_has_an_allow` (the
    direction count);
  - make the `cidr`-mode template `range` over `apiServer.cidrs` with no `fail` guard and loosen
    the schema's `minItems`, then render `mode: cidr` with `cidrs: []` → `no_allow_rule_is_peerless`
    and `cidr_mode_with_an_empty_list_is_refused`;
  - delete `certManager.fullnameOverride` and `nameOverride` → `rendered_names_are_dns_1123`;
  - drop `--namespace` for `edge` from the Go table → `every_workload_is_in_a_denied_namespace`
    (Traefik lands in `default`, where no case would otherwise look; review M2);
  - delete `resources` from `platform`'s `pull-machine` init container → `every_sandbox_container_is_sized`
    (and task 12's `prepull_init_containers_are_tiny`); then set the balloon's `limits.memory` to
    `8192Mi` with requests unchanged → `every_sandbox_container_is_sized` (plan review I4, the
    pre-pull sizing);
  - the walk skips `CronJob` → `every_sandbox_container_is_sized`'s fixture sub-test (the CronJob
    goes unreported); then the walk skips bare `Pod`s → the same sub-test (b05 re-review 8, the
    fixer's named survivor (b)).

  Commit.

#### Acceptance

```bash
helm dependency build deploy/charts/cert-manager
helm dependency build deploy/charts/edge
helm lint deploy/charts/cert-manager -f deploy/charts/cert-manager/tests/values-test.yaml
helm lint deploy/charts/edge -f deploy/charts/edge/tests/values-test.yaml
# 1 chart(s) linted, 0 chart(s) failed   (each)
helm unittest deploy/charts/cert-manager
# Tests: 10 passed, 10 total
helm unittest deploy/charts/edge
# Tests: 13 passed, 13 total
go test ./deploy/tests/netpolcover/... -count=1
# ok
```

#### What this task does not claim

- That `readTimeout: 0s` keeps an upgraded WebSocket open indefinitely in the chosen Traefik
  release — **UNVERIFIED**; task 32's attach soak (30 minutes idle through the edge) checks it,
  and the broker's 30-second WebSocket pings (task 18) are the second line.
- That public ACME is allowed (Q2), or that the zone is delegated (owner input).
- That the upstream charts' own templates pass `restricted` Pod Security in `panacea-edge` with
  these values — the first install proves it (task 31).
- That the policies admit the traffic they are written for on a real cluster. `netpolcover`
  proves every workload has an allow and every selector matches a pod; it cannot prove AKS's
  Cilium honours `kube-apiserver` as an entity, which source the webhook's calls arrive from, or
  that kubelet probes pass the default-deny. Task 32 probes each; the `apiServer.mode: cidr` seam
  is the fallback, and every wrong guess fails closed at `--atomic`, not open.
