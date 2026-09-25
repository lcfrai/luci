### Task 19: Helm chart, NetworkPolicies, Key Vault CSI

**The gateway on AKS, beside phase 8's workloads, with no route in from outside.** This task writes `deploy/charts/gateway`: LiteLLM, keysmith and the gatekeeper as three Deployments of two replicas, their Services, PDBs and service accounts, the Key Vault secrets as **files**, and the NetworkPolicies that make P6-D1 true in the cloud: a sandbox reaches the gatekeeper and nothing else here, and nothing but the gatekeeper and keysmith reaches LiteLLM. It renders; it is **never applied** in this phase. Applying it is phase 8's pipeline, once asks P8-1 to P8-5 are answered.

**It follows phase 8's conventions, read from the env tree** (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/`), so the two trees meet without a rename:
- a cross-namespace peer is `namespaceSelector` `kubernetes.io/metadata.name: <ns>` **and** `podSelector` in one peer (`task-13.md:289-291`);
- the broker's pods are `app.kubernetes.io/name: broker` in `panacea-broker`, the Hub's `app.kubernetes.io/name: hub` in `panacea-hub` on `8080` (`task-13.md:210-215`);
- session pods are `panacea-app/component: sandbox` in `panacea-sandboxes`, machine pods `panacea-app/component: machine` (`task-11.md:151-154`); DNS is `kube-system` pods `k8s-app: kube-dns` on 53 (`task-11.md:149-150`);
- internet egress is `0.0.0.0/0` **except** IMDS `169.254.169.254/32` and the Azure host address `168.63.129.16/32` (`task-11.md:155-156`), plus the three RFC 1918 blocks, as phase 8's cert-manager egress already does (`task-13.md:158`), so "443 to the internet" never means "443 to the VNet";
- the API server is reached through the `apiServer.mode: entity|cidr` seam (`task-13.md:76-77`, `:206-207`): a `CiliumNetworkPolicy` with `toEntities: [kube-apiserver]`, or an `ipBlock` per CIDR on 443;
- workload identity is the pod label `azure.workload.identity/use: "true"` plus the service account's `azure.workload.identity/client-id` annotation (`task-13.md:94-95`, phase 8 P8-D11).

**Decisions this part makes, beyond the skeleton's scope:**

- **(a) No Kubernetes Secret holds a gateway secret.** The skeleton syncs LiteLLM's master key and database URL into a Kubernetes Secret because LiteLLM reads them from the environment. This chart instead gives LiteLLM the entrypoint task 17 already proves locally: `/bin/sh -c` exports `LITELLM_MASTER_KEY`, `LITELLM_SALT_KEY`, `DATABASE_URL` and `ANTHROPIC_API_KEY` from the CSI-mounted **files** and `exec`s LiteLLM. One mechanism in both modes, no value in etcd, and no `secretKeyRef` at all, `optional` or not. The render test still fails on any `optional: true` and on any `secretKeyRef` or `secretObjects`, so the skeleton's guard stands and the stronger rule is enforced. The fallback, if the pinned image lost `/bin/sh` (task 17's step 5 would have said so), is the skeleton's synced Secret with `optional: false`.
- **(b) The AU credential is AWS web identity, set on the LiteLLM pod.** Task 02's decision (d) leaves it out of the config: the chart sets `AWS_ROLE_ARN` (task 20's output), `AWS_WEB_IDENTITY_TOKEN_FILE` and `AWS_REGION=ap-southeast-2`, and projects a service-account token with audience `sts.amazonaws.com` (task 20's trust condition). That boto3's default chain, which LiteLLM's Bedrock client is expected to fall back to, reads those two variables is **training knowledge, UNVERIFIED** until task 24's P04.
- **(c) keysmith gets `system:auth-delegator`**, so its `kube` caller authentication (task 08) can create `TokenReview`s. That is one `ClusterRoleBinding`, a cluster-scoped kind, and phase 8's owner question Q4 governs who may install it; it is listed in ask P8-2 below.
- **(d) Callers present projected tokens with audience `keysmith`.** The gatekeeper's pod mounts one at `/var/run/secrets/panacea/keysmith/token` and names it in `PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE` (the variable task 15 uses locally). The broker does the same in its own chart (ask P8-1).
- **(e) The gateway's own default-deny**, named `gateway-default-deny`, so the chart is sealed even before phase 8's platform chart lays its own, and never collides with that one's name.

**Files:**
- `deploy/charts/gateway/Chart.yaml`, `values.yaml`, `values-dev.yaml` (new)
- `deploy/charts/gateway/files/litellm.config.yaml` — task 02's generated cloud config, read, never edited here
- `deploy/charts/gateway/templates/_helpers.tpl` — labels, the image guard, the pod security block (new)
- `deploy/charts/gateway/templates/serviceaccounts.yaml`, `configmap.yaml`, `secretproviderclass.yaml`, `rbac.yaml` (new)
- `deploy/charts/gateway/templates/litellm.yaml`, `keysmith.yaml`, `gatekeeper.yaml` — Deployment, Service, PDB each (new)
- `deploy/charts/gateway/templates/networkpolicies.yaml` (new)
- `deploy/charts/gateway/tests/render_test.sh` (new)

**Interfaces:**
- Produces, in namespace `panacea-gateway`:
  - Services `gatekeeper:4000`, `keysmith:4100`, `litellm:4001` (ClusterIP; the skeleton's shared names), so a sandbox's `ANTHROPIC_BASE_URL` is `http://gatekeeper.panacea-gateway.svc.cluster.local:4000`
  - service accounts `litellm`, `keysmith`, `gatekeeper`, each annotated with its identity's client id
  - pod labels `app.kubernetes.io/name: {litellm|keysmith|gatekeeper}` and `app.kubernetes.io/part-of: panacea-gateway`, which phase 8's `netpolcover` and the Hub's ingress policy select on
- Consumes: task 02's `files/litellm.config.yaml`; task 08's and task 12's variables; task 17's image layout (`/usr/local/bin/{keysmith,gatekeeper}`); task 20's `role_arn`; phase 8's namespaces and labels above.
- Consumed by: task 24's proving scripts (the Services and labels), phase 8's pipeline (the release, asks P8-1 to P8-5), seam S6.9 (`netpolcover`).

- [ ] **Step 1: Write the failing render test**

`deploy/charts/gateway/tests/render_test.sh` renders the chart with a fixture of well-formed, fake values (no real id, digest or name), in both `apiServer` modes, and checks the text. It needs Helm 3 and `python3`, standard library only. The checks read this chart's own rendered layout with regular expressions; that is enough for a chart whose templates this task writes, and it is **not** a general Kubernetes parser, which is why seam S6.9 (phase 8's `netpolcover`, a Go test over typed objects) still matters.

```bash
#!/usr/bin/env bash
# render_test.sh: static checks over `helm template` for the gateway chart (task 19).
set -euo pipefail
chart="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
d() { printf 'sha256:%064d' "$1"; }   # a well-formed, fake digest
cat > "$tmp/fixture.yaml" <<EOF
images:
  litellm: ghcr.io/berriai/litellm:fixture@$(d 1)
  gateway: fixture.azurecr.io/panacea-app/gateway@$(d 2)
identities: {litellm: 00000000-0000-0000-0000-00000000000a, keysmith: 00000000-0000-0000-0000-00000000000b, gatekeeper: 00000000-0000-0000-0000-00000000000c}
tenantId: 00000000-0000-0000-0000-0000000000ff
keyVault: {name: kv-fixture}
postgres: {cidr: 10.9.8.7/32}
bedrock: {roleArn: "arn:aws:iam::000000000000:role/fixture"}
EOF
fail=0
bad() { echo "FAIL: $*"; fail=1; }

# Kills a chart that renders with an unpinned or missing image.
if helm template gw "$chart" -n panacea-gateway >/dev/null 2>"$tmp/err"; then
  bad "the chart rendered with no values: a default image or identity slipped in"
elif ! grep -q "images.litellm is required" "$tmp/err"; then
  bad "the missing-value error does not name images.litellm: $(head -c 200 "$tmp/err")"
fi

# Kills a release in another namespace: the sandbox image pins the gatekeeper's
# Service name, gatekeeper.panacea-gateway.svc.cluster.local (review I-4).
if helm template gw "$chart" -n gateway-other -f "$chart/values-dev.yaml" -f "$tmp/fixture.yaml" >/dev/null 2>"$tmp/err"; then
  bad "the chart rendered into namespace gateway-other"
elif ! grep -q "installs only into namespace panacea-gateway" "$tmp/err"; then
  bad "the namespace error does not name panacea-gateway: $(head -c 200 "$tmp/err")"
fi

for mode in entity cidr; do
  helm template gw "$chart" -n panacea-gateway -f "$chart/values-dev.yaml" -f "$tmp/fixture.yaml" \
    --set apiServer.mode="$mode" --set 'apiServer.cidrs={10.0.0.1/32}' > "$tmp/$mode.yaml"
  python3 - "$tmp/$mode.yaml" "$mode" <<'PY' || fail=1
import re, sys
text, mode = open(sys.argv[1]).read(), sys.argv[2]
docs = [d for d in re.split(r"(?m)^---\s*$", text) if re.search(r"(?m)^kind:", d)]
kind = lambda d: re.search(r"(?m)^kind: (\S+)", d).group(1)
name = lambda d: re.search(r"(?m)^  name: (\S+)", d).group(1)
by = lambda k: [d for d in docs if kind(d) == k]
errs = []
# Kills the Panacea silent key, and a Kubernetes Secret holding a gateway secret (decision a).
for pat in (r"optional: true", r"secretKeyRef", r"secretObjects", r"(?m)^kind: Secret$"):
    if re.search(pat, text):
        errs.append(f"rendered output contains {pat!r}")
# Kills a public path.
for k in ("HTTPRoute", "Ingress", "Gateway", "GRPCRoute", "TLSRoute"):
    if by(k):
        errs.append(f"a {k} is rendered")
if re.search(r"type: (LoadBalancer|NodePort)", text):
    errs.append("a Service is LoadBalancer or NodePort")
# Kills a tag-only image.
for img in re.findall(r"(?m)^\s+image: (\S+)", text):
    if not re.search(r"@sha256:[0-9a-f]{64}\"?$", img):
        errs.append(f"image {img} is not pinned by digest")
pols = {name(d): d for d in by("NetworkPolicy")}
# Kills a chart that is open until phase 8's platform chart lands (decision e).
dd = pols.get("gateway-default-deny", "")
if not (re.search(r"podSelector: \{\}", dd) and "- Ingress" in dd and "- Egress" in dd):
    errs.append("gateway-default-deny is missing or does not deny both directions")
# The netpolcover idea: kills a workload sealed by default-deny with no allow.
selected = set(re.findall(r"podSelector:\s*\n\s+matchLabels:\s*\n\s+app.kubernetes.io/name: (\S+)",
                          "".join(d for n, d in pols.items() if n != "gateway-default-deny")))
for dep in by("Deployment"):
    if name(dep) not in selected:
        errs.append(f"Deployment {name(dep)} is selected by no allow policy")
# Kills a sandbox-wide or cluster-wide gatekeeper (machines, other namespaces).
gk = pols.get("gatekeeper-ingress", "")
if not ("kubernetes.io/metadata.name: panacea-sandboxes" in gk
        and "panacea-app/component: sandbox" in gk and "port: 4000" in gk):
    errs.append("gatekeeper-ingress does not admit exactly sandbox pods on 4000")
if "component: machine" in gk or re.search(r"from:\s*\n\s+- \{\}", gk):
    errs.append("gatekeeper-ingress admits machines or everyone")
# Kills a LiteLLM reachable from anything but gatekeeper and keysmith.
ll = pols.get("litellm-ingress", "")
peers = re.findall(r"app.kubernetes.io/name: (\S+)", ll.split("ingress:", 1)[-1])
if sorted(peers) != ["gatekeeper", "keysmith"] or "namespaceSelector" in ll:
    errs.append(f"litellm-ingress admits {peers}, want gatekeeper and keysmith of this namespace")
# Kills IMDS or the VNet reachable on the internet rule (node identity theft, lateral moves).
for n, d in pols.items():
    if "0.0.0.0/0" in d:
        for cidr in ("169.254.169.254/32", "168.63.129.16/32", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"):
            if cidr not in d:
                errs.append(f"{n}: the internet rule does not except {cidr}")
# Kills a container without the hardening block, and a single replica.
containers = len(re.findall(r"(?m)^\s+- name: \S+\n\s+image:", text))
for key in ("readOnlyRootFilesystem: true", "runAsNonRoot: true",
            "allowPrivilegeEscalation: false", "limits:"):
    if text.count(key) < containers:
        errs.append(f"{containers} containers but {text.count(key)} carry {key!r}")
for dep in by("Deployment"):
    if "replicas: 2" not in dep:
        errs.append(f"Deployment {name(dep)} is not two replicas")
if len(by("PodDisruptionBudget")) != len(by("Deployment")) or text.count("minAvailable: 1") != len(by("Deployment")):
    errs.append("each Deployment needs one PDB with minAvailable: 1")
# Kills forgetting task 02's decision (d): the AU route's web identity.
lit = next((d for d in by("Deployment") if name(d) == "litellm"), "")
for need in ("AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE", "audience: sts.amazonaws.com",
             "exec litellm --config /etc/litellm/config.yaml --port 4001"):
    if need not in lit:
        errs.append(f"litellm Deployment lacks {need!r}")
if re.search(r"--(detailed_)?debug", lit):
    errs.append("litellm runs with a debug flag (P6-D9)")
# The API-server seam, both modes (keysmith's TokenReview).
cnp = [d for d in docs if kind(d) == "CiliumNetworkPolicy"]
if mode == "entity" and not any("kube-apiserver" in d for d in cnp):
    errs.append("entity mode renders no CiliumNetworkPolicy to kube-apiserver")
if mode == "cidr" and (cnp or "10.0.0.1/32" not in pols.get("keysmith-egress", "")):
    errs.append("cidr mode must render an ipBlock on keysmith-egress and no CiliumNetworkPolicy")
for e in errs:
    print(f"FAIL ({mode}): {e}")
sys.exit(1 if errs else 0)
PY
done

[ "$fail" -eq 0 ] && echo "PASS: render_test (entity and cidr modes)"
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/charts/gateway/tests/render_test.sh'
```

Expected: `FAIL: the missing-value error does not name images.litellm: Error: Chart.yaml file is missing` (or Helm's wording for a missing chart) and exit 1. If Helm is not on `PATH`, the error is `helm: command not found`; installing Helm 3 is a prerequisite this task states, and whether it is on this device is **UNVERIFIED**.

- [ ] **Step 3: Write `Chart.yaml`, `values.yaml` and `values-dev.yaml`**

`deploy/charts/gateway/Chart.yaml`:

```yaml
apiVersion: v2
name: gateway
description: Phase 6 inference gateway (LiteLLM, keysmith, gatekeeper). No public route.
type: application
version: 0.1.0
appVersion: "phase-6"
```

`deploy/charts/gateway/values.yaml`. Every value without a safe default is empty and **required** (the `gw.validate` helper, step 4), so a missing one fails the render with its name rather than rendering something open:

```yaml
# Phase 6 gateway chart (task 19). Rendered by phase 8's pipeline; never applied by phase 6.
images:
  litellm: ""   # ghcr.io/berriai/litellm:<tag>@sha256:<64 hex>: task 17's pin, task 18's probed digest
  gateway: ""   # <registry>/panacea-app/gateway@sha256:<64 hex>: images/gateway (task 17)
replicas: 2
identities:     # user-assigned identity client ids, one per workload (ask P8-2)
  litellm: ""
  keysmith: ""
  gatekeeper: ""
tenantId: ""
keyVault:
  name: ""      # the four secrets below live here (ask P8-2)
  secrets:      # Key Vault secret name -> file name in the pod's /mnt/secrets
    litellm: [anthropic-api-key, litellm-master-key, litellm-salt-key, gateway-database-url]
    keysmith: [litellm-master-key, gateway-database-url]
postgres:
  cidr: ""      # the gateway database's private endpoint, <ip>/32 (ask P8-4)
  port: 5432
bedrock:
  roleArn: ""   # task 20's output role_arn
  region: ap-southeast-2
hub:
  url: http://hub.panacea-hub.svc.cluster.local:8080
  namespace: panacea-hub
  podLabels: {app.kubernetes.io/name: hub}
  port: 8080
broker:
  namespace: panacea-broker
  podLabels: {app.kubernetes.io/name: broker}
sandboxes:
  namespace: panacea-sandboxes
  podLabels: {panacea-app/component: sandbox}   # session pods only; machine pods never reach the gateway
apiServer:
  mode: entity  # entity | cidr: phase 8's seam
  cidrs: []
egressExcept:   # never reachable on the internet rule
  - 169.254.169.254/32
  - 168.63.129.16/32
  - 10.0.0.0/8
  - 172.16.0.0/12
  - 192.168.0.0/16
resources:
  litellm: {requests: {cpu: 500m, memory: 1Gi}, limits: {cpu: "2", memory: 2Gi}}
  keysmith: {requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 500m, memory: 256Mi}}
  gatekeeper: {requests: {cpu: 250m, memory: 256Mi}, limits: {cpu: "1", memory: 512Mi}}
```

The resource figures are proposals, **UNVERIFIED** against load: LiteLLM's is Panacea-sized (a Python proxy with a Prisma client), keysmith's is an admin-path service, and the gatekeeper's must hold many long streams, which is memory per connection, not CPU. Task 24's proving run is where they are first observed.

`deploy/charts/gateway/values-dev.yaml` carries only what dev decides and is not a secret or an id; the ids and digests arrive from phase 8's pipeline as `--set`, never committed:

```yaml
# Dev overrides for the gateway chart. Ids, digests and names come from the pipeline (--set).
replicas: 2
apiServer:
  mode: entity
```

- [ ] **Step 4: Write the helpers and the shared objects**

`deploy/charts/gateway/templates/_helpers.tpl`. `gw.validate` runs first in every template, so whichever file Helm renders first, the first missing value it reports is the same one:

```yaml
{{- define "gw.labels" -}}
app.kubernetes.io/name: {{ . }}
app.kubernetes.io/part-of: panacea-gateway
{{- end }}

{{- define "gw.image" -}}
{{- $v := required (printf "%s is required, as <repo>@sha256:<64 hex>" .key) .v -}}
{{- if not (regexMatch "@sha256:[0-9a-f]{64}$" $v) -}}
{{- fail (printf "%s must be pinned by digest, got %s" .key $v) -}}
{{- end -}}
{{- $v | quote -}}
{{- end }}

{{- define "gw.validate" -}}
{{- $_ := include "gw.image" (dict "v" .Values.images.litellm "key" "images.litellm") -}}
{{- $_ = include "gw.image" (dict "v" .Values.images.gateway "key" "images.gateway") -}}
{{- range $k := list "litellm" "keysmith" "gatekeeper" -}}
{{- $_ = required (printf "identities.%s is required (ask P8-2)" $k) (index $.Values.identities $k) -}}
{{- end -}}
{{- $_ = required "tenantId is required" .Values.tenantId -}}
{{- $_ = required "keyVault.name is required (ask P8-2)" .Values.keyVault.name -}}
{{- $_ = required "postgres.cidr is required (ask P8-4)" .Values.postgres.cidr -}}
{{- $_ = required "bedrock.roleArn is required (task 20's role_arn)" .Values.bedrock.roleArn -}}
{{- if not (has .Values.apiServer.mode (list "entity" "cidr")) -}}
{{- fail "apiServer.mode must be entity or cidr" -}}
{{- end -}}
{{- if and (eq .Values.apiServer.mode "cidr") (empty .Values.apiServer.cidrs) -}}
{{- fail "apiServer.cidrs must be non-empty in cidr mode" -}}
{{- end -}}
{{- end }}

{{- define "gw.securityContext" -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: {drop: [ALL]}
  seccompProfile: {type: RuntimeDefault}
{{- end }}
```

`templates/serviceaccounts.yaml`. Only keysmith mounts the API token, because only keysmith calls the API server (`TokenReview`):

```yaml
{{- include "gw.validate" . }}
{{- range $name := list "litellm" "keysmith" "gatekeeper" }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $name }}
  labels: {{- include "gw.labels" $name | nindent 4 }}
  annotations:
    azure.workload.identity/client-id: {{ index $.Values.identities $name | quote }}
automountServiceAccountToken: {{ eq $name "keysmith" }}
{{- end }}
```

`templates/configmap.yaml`, the generated config, never templated further:

```yaml
{{- include "gw.validate" . }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: litellm-config
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
data:
  config.yaml: |
    {{- .Files.Get "files/litellm.config.yaml" | nindent 4 }}
```

`templates/secretproviderclass.yaml`. One class per workload, each read as that workload's own identity, and **no `secretObjects`**: the files are the only copy in the cluster (decision (a)). The parameter names are the Azure Key Vault provider's (`usePodIdentity`, `clientID`, `keyvaultName`, `tenantId`, `objects`), as remembered from the AKS documentation for workload identity, **UNVERIFIED** here; phase 8's charts use the same add-on (P8-D11) and task 24 sees the mount:

```yaml
{{- include "gw.validate" . }}
{{- range $name, $objs := .Values.keyVault.secrets }}
---
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: gateway-{{ $name }}
  labels: {{- include "gw.labels" $name | nindent 4 }}
spec:
  provider: azure
  parameters:
    usePodIdentity: "false"
    clientID: {{ index $.Values.identities $name | quote }}
    keyvaultName: {{ $.Values.keyVault.name | quote }}
    tenantId: {{ $.Values.tenantId | quote }}
    objects: |
      array:
      {{- range $objs }}
        - |
          objectName: {{ . }}
          objectType: secret
      {{- end }}
{{- end }}
```

`templates/rbac.yaml`, decision (c). The one cluster-scoped object in the chart:

```yaml
{{- include "gw.validate" . }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: panacea-gateway-keysmith-tokenreview
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - kind: ServiceAccount
    name: keysmith
    namespace: {{ .Release.Namespace }}
```

`system:auth-delegator` also grants `SubjectAccessReview` creation (training knowledge of the built-in role, **UNVERIFIED**); keysmith uses only `TokenReview`, and a narrower custom ClusterRole granting `create` on `tokenreviews` alone is the stricter alternative phase 8's Q4 review may prefer. It is one file either way.

- [ ] **Step 5: Write the three workloads**

`templates/litellm.yaml`. `$$(` is Kubernetes' escape for a literal `$(` in `args` (the kubelet expands `$(NAME)` references), so the shell, not the kubelet, runs each `cat`. The liveness probe is `/health/liveliness`, **never** `/health`, which in LiteLLM calls every configured upstream (training knowledge, **UNVERIFIED**; Panacea's liveness probe uses `/health`, `litellm-values.yaml:230`, which on this gateway would send a request to Anthropic and Bedrock every period):

```yaml
{{- include "gw.validate" . }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: litellm
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels: {app.kubernetes.io/name: litellm}
  template:
    metadata:
      labels:
        {{- include "gw.labels" "litellm" | nindent 8 }}
        azure.workload.identity/use: "true"
      annotations:
        checksum/config: {{ .Files.Get "files/litellm.config.yaml" | sha256sum }}
    spec:
      serviceAccountName: litellm
      containers:
        - name: litellm
          image: {{ include "gw.image" (dict "v" .Values.images.litellm "key" "images.litellm") }}
          command: ["/bin/sh", "-c"]
          args:
            - |
              export LITELLM_MASTER_KEY="$$(cat /mnt/secrets/litellm-master-key)"
              export LITELLM_SALT_KEY="$$(cat /mnt/secrets/litellm-salt-key)"
              export DATABASE_URL="$$(cat /mnt/secrets/gateway-database-url)"
              export ANTHROPIC_API_KEY="$$(cat /mnt/secrets/anthropic-api-key)"
              exec litellm --config /etc/litellm/config.yaml --port 4001
          env:
            - {name: HOME, value: /tmp}
            - {name: AWS_ROLE_ARN, value: {{ .Values.bedrock.roleArn | quote }}}
            - {name: AWS_WEB_IDENTITY_TOKEN_FILE, value: /var/run/secrets/aws/token}
            - {name: AWS_REGION, value: {{ .Values.bedrock.region | quote }}}
            - {name: AWS_STS_REGIONAL_ENDPOINTS, value: regional}
          ports:
            - {name: http, containerPort: 4001}
          readinessProbe: {httpGet: {path: /health/readiness, port: 4001}, periodSeconds: 5}
          livenessProbe: {httpGet: {path: /health/liveliness, port: 4001}, periodSeconds: 10, failureThreshold: 6}
          resources: {{- toYaml .Values.resources.litellm | nindent 12 }}
          {{- include "gw.securityContext" . | nindent 10 }}
          volumeMounts:
            - {name: config, mountPath: /etc/litellm, readOnly: true}
            - {name: secrets, mountPath: /mnt/secrets, readOnly: true}
            - {name: aws-token, mountPath: /var/run/secrets/aws, readOnly: true}
            - {name: tmp, mountPath: /tmp}
      volumes:
        - name: config
          configMap: {name: litellm-config}
        - name: secrets
          csi: {driver: secrets-store.csi.k8s.io, readOnly: true, volumeAttributes: {secretProviderClass: gateway-litellm}}
        - name: aws-token
          projected:
            sources:
              - serviceAccountToken: {audience: sts.amazonaws.com, expirationSeconds: 3600, path: token}
        - name: tmp
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: litellm
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
spec:
  type: ClusterIP
  selector: {app.kubernetes.io/name: litellm}
  ports:
    - {name: http, port: 4001, targetPort: 4001}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: litellm
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
spec:
  minAvailable: 1
  selector:
    matchLabels: {app.kubernetes.io/name: litellm}
```

`templates/keysmith.yaml`. Its variables are the skeleton's shared names; `kube` caller authentication validates projected tokens of audience `keysmith` (task 08), and `hub` reads the live route as its workload identity (task 13, ask H-3):

```yaml
{{- include "gw.validate" . }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keysmith
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels: {app.kubernetes.io/name: keysmith}
  template:
    metadata:
      labels:
        {{- include "gw.labels" "keysmith" | nindent 8 }}
        azure.workload.identity/use: "true"
    spec:
      serviceAccountName: keysmith
      containers:
        - name: keysmith
          image: {{ include "gw.image" (dict "v" .Values.images.gateway "key" "images.gateway") }}
          command: ["/usr/local/bin/keysmith"]
          env:
            - {name: PANACEA_APP_GATEWAY_LITELLM_URL, value: "http://litellm:4001"}
            - {name: PANACEA_APP_GATEWAY_MASTER_KEY_FILE, value: /mnt/secrets/litellm-master-key}
            - {name: PANACEA_APP_GATEWAY_DATABASE_URL_FILE, value: /mnt/secrets/gateway-database-url}
            - {name: PANACEA_APP_GATEWAY_CALLER_AUTH, value: kube}
            - {name: PANACEA_APP_GATEWAY_ROUTE_SOURCE, value: hub}
            - {name: PANACEA_APP_GATEWAY_HUB_URL, value: {{ .Values.hub.url | quote }}}
            {{- if ne .Release.Namespace "panacea-gateway" }}{{ fail "the gateway chart installs only into namespace panacea-gateway: the sandbox image pins ANTHROPIC_BASE_URL=http://gatekeeper.panacea-gateway.svc.cluster.local:4000 (phase 6 task 14 decision (e))" }}{{ end }}
            - {name: PANACEA_APP_GATEWAY_BASE_URL, value: "http://gatekeeper.panacea-gateway.svc.cluster.local:4000"}
          ports:
            - {name: http, containerPort: 4100}
          readinessProbe: {httpGet: {path: /readyz, port: 4100}, periodSeconds: 5}
          livenessProbe: {httpGet: {path: /healthz, port: 4100}, periodSeconds: 10}
          resources: {{- toYaml .Values.resources.keysmith | nindent 12 }}
          {{- include "gw.securityContext" . | nindent 10 }}
          volumeMounts:
            - {name: secrets, mountPath: /mnt/secrets, readOnly: true}
      volumes:
        - name: secrets
          csi: {driver: secrets-store.csi.k8s.io, readOnly: true, volumeAttributes: {secretProviderClass: gateway-keysmith}}
---
apiVersion: v1
kind: Service
metadata:
  name: keysmith
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  type: ClusterIP
  selector: {app.kubernetes.io/name: keysmith}
  ports:
    - {name: http, port: 4100, targetPort: 4100}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: keysmith
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  minAvailable: 1
  selector:
    matchLabels: {app.kubernetes.io/name: keysmith}
```

`templates/gatekeeper.yaml`. The grace period is stated at 30 s to match task 12's decision (e): the server drains open streams for 25 s on `SIGTERM`, then closes, so a rolling update can cut a stream longer than that. Raising both together (a longer drain in task 12, a longer grace here) is the lever if task 23 or task 24 shows cut streams on deploys; this chart does not raise one without the other:

```yaml
{{- include "gw.validate" . }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gatekeeper
  labels: {{- include "gw.labels" "gatekeeper" | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels: {app.kubernetes.io/name: gatekeeper}
  template:
    metadata:
      labels:
        {{- include "gw.labels" "gatekeeper" | nindent 8 }}
        azure.workload.identity/use: "true"
    spec:
      serviceAccountName: gatekeeper
      terminationGracePeriodSeconds: 30   # task 12 drains for 25 s
      containers:
        - name: gatekeeper
          image: {{ include "gw.image" (dict "v" .Values.images.gateway "key" "images.gateway") }}
          command: ["/usr/local/bin/gatekeeper"]
          env:
            - {name: PANACEA_APP_GATEWAY_LITELLM_URL, value: "http://litellm:4001"}
            - {name: PANACEA_APP_GATEWAY_KEYSMITH_URL, value: "http://keysmith:4100"}
            - {name: PANACEA_APP_GATEWAY_ROUTE_SOURCE, value: hub}
            - {name: PANACEA_APP_GATEWAY_HUB_URL, value: {{ .Values.hub.url | quote }}}
            - {name: PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE, value: /var/run/secrets/panacea/keysmith/token}
          ports:
            - {name: http, containerPort: 4000}
          readinessProbe: {httpGet: {path: /readyz, port: 4000}, periodSeconds: 5}
          livenessProbe: {httpGet: {path: /healthz, port: 4000}, periodSeconds: 10}
          resources: {{- toYaml .Values.resources.gatekeeper | nindent 12 }}
          {{- include "gw.securityContext" . | nindent 10 }}
          volumeMounts:
            - {name: keysmith-token, mountPath: /var/run/secrets/panacea/keysmith, readOnly: true}
      volumes:
        - name: keysmith-token
          projected:
            sources:
              - serviceAccountToken: {audience: keysmith, expirationSeconds: 3600, path: token}
---
apiVersion: v1
kind: Service
metadata:
  name: gatekeeper
  labels: {{- include "gw.labels" "gatekeeper" | nindent 4 }}
spec:
  type: ClusterIP
  selector: {app.kubernetes.io/name: gatekeeper}
  ports:
    - {name: http, port: 4000, targetPort: 4000}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: gatekeeper
  labels: {{- include "gw.labels" "gatekeeper" | nindent 4 }}
spec:
  minAvailable: 1
  selector:
    matchLabels: {app.kubernetes.io/name: gatekeeper}
```

The Service is plain `ClusterIP` with no `sessionAffinity` and no `externalTrafficPolicy`, because it is reached only from inside the cluster. Whether the address the gatekeeper then sees in `r.RemoteAddr` is the sandbox pod's own (kube-proxy or Cilium service translation does not source-NAT pod-to-service traffic within a cluster, training knowledge) is task 24's P03, and `05-design.md:212-214`'s open item.

- [ ] **Step 6: Write the NetworkPolicies**

`templates/networkpolicies.yaml`, standard `networking.k8s.io/v1` enforced by Cilium, as phase 8's are. Block style throughout, so the render test's patterns read it. The two repeated rules are helpers at the top of the file:

```yaml
{{- include "gw.validate" . }}
{{- define "gw.internet443" -}}
- to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
          {{- toYaml .Values.egressExcept | nindent 10 }}
  ports:
    - {protocol: TCP, port: 443}
{{- end }}
{{- define "gw.toHub" -}}
- to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: {{ .Values.hub.namespace }}
      podSelector:
        matchLabels:
          {{- toYaml .Values.hub.podLabels | nindent 10 }}
  ports:
    - {protocol: TCP, port: {{ .Values.hub.port }}}
{{- end }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gateway-default-deny
  labels: {{- include "gw.labels" "gateway" | nindent 4 }}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gateway-dns
  labels: {{- include "gw.labels" "gateway" | nindent 4 }}
spec:
  podSelector: {}
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - {protocol: UDP, port: 53}
        - {protocol: TCP, port: 53}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gatekeeper-ingress
  labels: {{- include "gw.labels" "gatekeeper" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: gatekeeper
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{ .Values.sandboxes.namespace }}
          podSelector:
            matchLabels:
              {{- toYaml .Values.sandboxes.podLabels | nindent 14 }}
      ports:
        - {protocol: TCP, port: 4000}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gatekeeper-egress
  labels: {{- include "gw.labels" "gatekeeper" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: gatekeeper
  policyTypes:
    - Egress
  egress:
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: litellm
      ports:
        - {protocol: TCP, port: 4001}
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: keysmith
      ports:
        - {protocol: TCP, port: 4100}
    {{- include "gw.toHub" . | nindent 4 }}
    {{- include "gw.internet443" . | nindent 4 }}
```

The same file, continued: keysmith and LiteLLM.

```yaml
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: keysmith-ingress
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: keysmith
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{ .Values.broker.namespace }}
          podSelector:
            matchLabels:
              {{- toYaml .Values.broker.podLabels | nindent 14 }}
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: gatekeeper
      ports:
        - {protocol: TCP, port: 4100}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: keysmith-egress
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: keysmith
  policyTypes:
    - Egress
  egress:
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: litellm
      ports:
        - {protocol: TCP, port: 4001}
    - to:
        - ipBlock:
            cidr: {{ .Values.postgres.cidr }}
      ports:
        - {protocol: TCP, port: {{ .Values.postgres.port }}}
    {{- include "gw.toHub" . | nindent 4 }}
    {{- include "gw.internet443" . | nindent 4 }}
    {{- if eq .Values.apiServer.mode "cidr" }}
    - to:
      {{- range .Values.apiServer.cidrs }}
        - ipBlock:
            cidr: {{ . }}
      {{- end }}
      ports:
        - {protocol: TCP, port: 443}
    {{- end }}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: litellm-ingress
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: litellm
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: gatekeeper
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: keysmith
      ports:
        - {protocol: TCP, port: 4001}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: litellm-egress
  labels: {{- include "gw.labels" "litellm" | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: litellm
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: {{ .Values.postgres.cidr }}
      ports:
        - {protocol: TCP, port: {{ .Values.postgres.port }}}
    {{- include "gw.internet443" . | nindent 4 }}
{{- if eq .Values.apiServer.mode "entity" }}
---
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: keysmith-api-server
  labels: {{- include "gw.labels" "keysmith" | nindent 4 }}
spec:
  endpointSelector:
    matchLabels:
      app.kubernetes.io/name: keysmith
  egress:
    - toEntities:
        - kube-apiserver
      toPorts:
        - ports:
            - {port: "443", protocol: TCP}
{{- end }}
```

LiteLLM's internet rule carries **both** upstreams (`api.anthropic.com`, `bedrock-runtime.ap-southeast-2.amazonaws.com`, and STS for the web-identity exchange) on one port; which host names are allowed is the firewall's (ask P8-3), because a NetworkPolicy cannot name a host. The gatekeeper's and keysmith's internet rules exist only for the workload-identity token exchange with Entra, and the firewall allowlist for them is Entra's endpoints alone.

- [ ] **Step 7: Run the render test to verify it passes, and that it can fail**

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/charts/gateway/tests/render_test.sh'
```

Expected: `PASS: render_test (entity and cidr modes)` and exit 0.

**Prove three checks are not vacuous**, restoring each change after:
1. In `litellm-ingress`, replace the two peers with `- {}`: the test must fail with `litellm-ingress admits [], want gatekeeper and keysmith of this namespace`.
2. Delete `168.63.129.16/32` from `egressExcept` in `values.yaml`: it must fail three times with `the internet rule does not except 168.63.129.16/32`.
3. Change the fixture's LiteLLM image to `ghcr.io/berriai/litellm:main-stable`: the render must fail with `images.litellm must be pinned by digest`, which the test reports as a failed `helm template`.

- [ ] **Step 8: Check LiteLLM on a read-only root, locally**

The chart runs LiteLLM with `readOnlyRootFilesystem: true`, `HOME=/tmp` and one `emptyDir` at `/tmp`. Whether the pinned image, which runs Prisma at start-up, tolerates that is **UNVERIFIED**; task 17's stack answers it at no cost. Save as a scratch override `ro.yaml`:

```yaml
services:
  litellm:
    read_only: true
    user: "65532:65532"
    environment: {HOME: /tmp}
    tmpfs: [/tmp]
```

```bash
~/.claude/tools/claudezsh/claudezsh 'cd deploy/local/gateway && docker compose -f compose.yaml -f <scratch>/ro.yaml up -d --wait litellm && docker compose ps litellm --format "{{.State}} {{.Health}}"'
```

Expected: `running healthy`. If it fails, `docker compose logs litellm` names the path it could not write; add one `emptyDir` for that path to `templates/litellm.yaml` and to `ro.yaml`, and re-run until healthy. Restore the stack with `docker compose up -d --wait` afterwards. Record the paths added, if any, in the commit message.

- [ ] **Step 9: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add deploy/charts/gateway/Chart.yaml deploy/charts/gateway/values.yaml deploy/charts/gateway/values-dev.yaml deploy/charts/gateway/templates deploy/charts/gateway/tests/render_test.sh'
~/.claude/tools/claudezsh/claudezsh 'git commit -F /tmp/msg-task19.txt -- deploy/charts/gateway/Chart.yaml deploy/charts/gateway/values.yaml deploy/charts/gateway/values-dev.yaml deploy/charts/gateway/templates deploy/charts/gateway/tests/render_test.sh'
```

`files/litellm.config.yaml` is task 02's commit, not this one. `/tmp/msg-task19.txt`:

```text
feat(gateway): the gateway chart, no public route

LiteLLM, keysmith and the gatekeeper as two-replica Deployments with PDBs,
ClusterIP Services on 4001, 4100 and 4000, digests required, non-root and
read-only. Key Vault secrets arrive as CSI files and LiteLLM's entrypoint
exports them, so no Kubernetes Secret holds a gateway secret and no
secretKeyRef exists to be optional. The AU route's AWS credential is web
identity: AWS_ROLE_ARN plus a projected sts.amazonaws.com token.

Default deny, then: sandbox session pods to the gatekeeper on 4000 only;
the broker and the gatekeeper to keysmith; only the gatekeeper and keysmith
to LiteLLM; 443 out with IMDS, the Azure host address and RFC 1918 excepted.
No HTTPRoute, no Ingress. render_test.sh checks all of it in both API-server
modes. Rendered, never applied.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**What this part asks of phase 8, beyond the skeleton's P8-1 to P8-5:**
- the Hub's ingress policy (phase 8 task 29's) admits `app.kubernetes.io/name: {gatekeeper,keysmith}` from `panacea-gateway` on 8080 (ask H-3's network half);
- P8-3's sandbox egress rule targets exactly this chart's `gatekeeper` pods on 4000, and phase 8's `egress-internet` except list stays as it is, so a sandbox cannot reach LiteLLM or keysmith by address;
- the `ClusterRoleBinding` of decision (c) is installed under whatever Q4 decides;
- the four Key Vault secret names above, created by a human, never by a pipeline.

**What this task does NOT claim:**

- **An apply**, or that any object here is accepted by a real API server. Only `helm template` has run.
- **That phase 8's `netpolcover` passes** over this chart. It lives in the env tree (seam S6.9); the render test is this chart's own, weaker, text check.
- **That the Key Vault provider's parameter names, the CSI driver name, or Cilium's `kube-apiserver` entity are right** for the cluster phase 8 builds. They are remembered, and the first apply is where a wrong one shows.
- **That the source address survives** from a sandbox pod to `r.RemoteAddr` (task 24's P03), or that Cilium admits kubelet probes under default deny (phase 8's charts rely on the same behaviour).
- **That LiteLLM runs read-only** until step 8 has run, or that its resource figures hold under load.
- **That `AWS_ROLE_ARN` and the token file reach LiteLLM's Bedrock client**; task 24's P04 is the first real call.
