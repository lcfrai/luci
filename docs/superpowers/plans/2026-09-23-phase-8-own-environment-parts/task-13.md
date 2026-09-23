### Task 13: `deploy/charts/edge` — Traefik on the Gateway API, cert-manager, one Gateway

The edge is where a user's token enters the environment, so it is small and fails closed: one
public IP owned by Terraform (task 07), one Traefik deployment implementing the Kubernetes Gateway
API (P8-D9), one Gateway with one HTTPS listener, certificates from cert-manager over DNS-01 with a
federated identity (no secret), and a source-range allowlist that is **required** (owner question
Q3). HTTPRoutes are not here: the Hub and broker charts bring their own (task 29) into namespaces
the Gateway admits by label (task 11).

Panacea's edge declares its IP allowlist in the release because a live `kubectl patch` was being
reverted (survey §1.5, `:129-131`) and keeps `externalTrafficPolicy: Local` so the allowlist sees
client IPs (`:131-132`). Both carry over, written from scratch.

**Files:**
- `deploy/charts/edge/Chart.yaml`, `Chart.lock`, `values.yaml`, `values.schema.json` (new)
- `deploy/charts/edge/crds/gateway-api-standard-<version>.yaml` — the upstream Gateway API standard CRDs, vendored unmodified (Apache-2.0), with a `crds/README.md` naming the version and its source URL (new)
- `deploy/charts/edge/templates/{gateway.yaml,certificate.yaml,clusterissuer.yaml,pdb.yaml}` (new)
- `deploy/charts/edge/tests/*_test.yaml` (new)

#### Contract — `Chart.yaml` (versions pinned exactly at implementation; no ranges)

```yaml
apiVersion: v2
name: edge
description: PanaceaApp edge - Traefik (Gateway API), cert-manager, the Gateway and its certificate.
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: traefik
    repository: https://traefik.github.io/charts
    version: "<exact, pinned in step 1>"     # MIT
  - name: cert-manager
    repository: https://charts.jetstack.io
    version: "<exact, pinned in step 1>"     # Apache-2.0
    alias: certManager
```

#### Contract — `values.yaml` (complete)

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

traefik:
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

certManager:
  namespace: panacea-edge
  crds:
    enabled: true
  priorityClassName: panacea-platform-critical
  nodeSelector:
    panacea-app/role: apps
  podLabels:
    azure.workload.identity/use: "true"
  serviceAccount:
    name: cert-manager           # must equal task 10's namespaces.cert_manager / service account
```

The Service's annotations and `loadBalancerSourceRanges` are set from the top-level values by
the parent chart's values (`traefik.service.annotations`, `traefik.service.spec.loadBalancerSourceRanges`
templated through `tpl` in a small wrapper, or passed by the pipeline) so that there is one
source for each: `service.beta.kubernetes.io/azure-pip-name: {{ .Values.edgeIp.name }}`,
`service.beta.kubernetes.io/azure-load-balancer-resource-group: {{ .Values.edgeIp.resourceGroup }}`,
`service.beta.kubernetes.io/azure-load-balancer-tcp-idle-timeout: "{{ .Values.lbIdleTimeoutMinutes }}"`.
**The three annotation names are from training knowledge of the Azure cloud provider,
UNVERIFIED**; the test pins them and task 31's smoke proves the IP binds. The cert-manager service
account's `azure.workload.identity/client-id` annotation comes from `certManagerIdentity.clientId`.

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
| `cert_manager_is_federated` | the cert-manager pod label `azure.workload.identity/use: "true"` and the SA annotation with the client id | a federated credential nobody presents |
| `dependencies_are_pinned_exactly` | a test reads `Chart.yaml`: every dependency `version` matches `^[0-9]+\.[0-9]+\.[0-9]+$` | a `~` range that changes the edge on a rebuild |
| `edge_runs_on_apps_at_platform_priority` | Traefik and cert-manager `nodeSelector` apps and `priorityClassName: panacea-platform-critical` | the edge preempted by a session burst |

#### Steps

- [ ] **Step 1:** Choose the current Traefik chart (v3 line) and cert-manager chart releases; record
  each version, its licence (MIT, Apache-2.0) and its source in `Chart.yaml` comments;
  `helm dependency build`; commit `Chart.lock` (never the downloaded `charts/*.tgz`, which
  `.gitignore` gains). Download the Gateway API standard CRDs of the version the chosen Traefik
  release supports; vendor unmodified into `crds/`.
- [ ] **Step 2:** Values, schema, suites; the suites fail.
- [ ] **Step 3:** Templates until they pass; `helm lint` with test values.
- [ ] **Step 4:** Commit.

#### Acceptance

```bash
helm dependency build deploy/charts/edge
helm lint deploy/charts/edge -f deploy/charts/edge/tests/values-test.yaml
# 1 chart(s) linted, 0 chart(s) failed
helm unittest deploy/charts/edge
# Tests: 10 passed, 10 total
```

#### What this task does not claim

- That `readTimeout: 0s` keeps an upgraded WebSocket open indefinitely in the chosen Traefik
  release — **UNVERIFIED**; task 32's attach soak (30 minutes idle through the edge) checks it,
  and the broker's 30-second WebSocket pings (task 18) are the second line.
- That public ACME is allowed (Q2), or that the zone is delegated (owner input).
- That the upstream charts' own templates pass `restricted` Pod Security in `panacea-edge` with
  these values — the first install proves it (task 31).
