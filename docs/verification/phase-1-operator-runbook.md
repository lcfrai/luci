# Phase 1 — operator runbook

Everything in Phase 1 is implemented and unit-tested. This document is the part that could not be executed from the development machine because it needs Docker and an Azure cluster.

Phase 1 exists to retire exactly one risk: **does a real Claude Code TUI render faithfully through a PTY-over-WebSocket pipe, and can you type back into it?** Everything downstream assumes yes. If any fidelity check below fails, the architecture changes, so run them honestly and write down what you see.

**Time:** about 20 minutes with Docker running and cluster access.

---

## What was already proven, without a cluster

Do not re-verify these; they are done.

| Layer | Evidence |
|---|---|
| HTTP server + `/healthz` | ran the real binary, returned 200 |
| WebSocket upgrade on `/attach` | client dialled successfully |
| Resize frame encode/accept | server consumed it |
| Stdin frame encode/accept | server consumed it |
| Error path | correctly-encoded `Exit` frame returned |
| Frame protocol | 5 unit tests + 2 fuzz targets, ~2M executions each, zero crashes |
| Go↔TypeScript protocol parity | byte values and big-endian resize confirmed identical |

**Not proven:** the PTY spawn itself, the container image, and the Kata node pool. That is what this runbook covers.

---

## Stage 0 — prerequisites

```bash
# Docker Desktop must be running. Verify:
docker info --format '{{.ServerVersion}}'

# Azure CLI logged in to the dev subscription:
az account show --query name -o tsv
```

**Blocking unknowns to settle before Stage 3:**

1. **Is Pod Sandboxing enabled on the dev cluster?** Kata requires it. If `kubectl get runtimeclass` returns nothing Kata-related, it must be enabled first — that is a cluster change, not an app change.
2. **Is there nested-virtualisation quota** for `Standard_D8s_v5` in the dev region? Kata needs a Gen2 nested-virt-capable SKU.

---

## Stage 1 — prove the PTY locally (no cluster needed)

This is the highest-value step and the cheapest. It retires most of Phase 1's risk before Azure is involved.

```bash
cd <repo root>

# Build the image (the Dockerfile was written but has never been built anywhere)
docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:dev .

# Confirm the agent CLI is present and runs as the non-root user
docker run --rm panacea-app/sandbox:dev bash -lc 'claude --version'

# Run the sandbox and expose it
docker run --rm -d -p 8099:8080 --name pa-sandbox panacea-app/sandbox:dev
curl -s -o /dev/null -w '%{http_code}\n' localhost:8099/healthz    # expect 200
```

Now connect the desktop app to `ws://localhost:8099/attach`.

**Expected:** a live `bash` prompt from inside the container. Type `ls` and press Enter — it should list the container's filesystem.

This is the first time a real PTY has run through this pipe. If it works, Phase 1's core risk is retired.

```bash
docker rm -f pa-sandbox
```

### Stage 1b — the fidelity checks that actually matter

With the app connected, run `claude` in the terminal and verify each of these. **Write down pass/fail for each — these are Phase 1's real acceptance criteria.**

- [ ] The `╭─ > ─╮` input frame renders with unbroken box-drawing characters
- [ ] Colour is correct; no raw escape sequences printed as literal text
- [ ] `⏵⏵ bypass permissions on (shift+tab to cycle)` appears
- [ ] **shift+tab actually cycles the permission mode** (keystroke round-trip)
- [ ] Ask it to run a command — the numbered permission prompt renders, and pressing `1` selects Yes
- [ ] Resizing the window reflows the TUI rather than corrupting it
- [ ] Keystroke echo latency is acceptable — estimate it and write the number down

A failure here is a genuine architectural finding, not a bug to paper over.

---

## Stage 2 — push the image

```bash
az acr build --registry <your-acr> --image panacea-app/sandbox:dev \
  --file images/sandbox/Dockerfile .

az acr repository show-tags --name <your-acr> --repository panacea-app/sandbox
```

---

## Stage 3 — the Kata node pool

> **Two things about this block, both checked on 2026-09-28.**
> 1. **The engine is `tofu`, not `terraform` (`OD-32`).** HashiCorp Terraform has been BUSL-1.1
>    since August 2023 and this product ships to customers who run it on their own cloud, so the
>    binary is OpenTofu (MPL-2.0). `.tf` syntax and the flags below are unchanged. The providers
>    do not move: `hashicorp/azurerm` is MPL-2.0.
> 2. **`infra/terraform` no longer holds a root module, so this block cannot be run as written.**
>    `ls infra/terraform/*.tf` returns nothing: phase 8 replaced the flat `kata.tf` layout with
>    `infra/terraform/{modules,stacks}/…`, and the Kata pool moves into a stack there. Do not
>    "fix" this by re-creating a root module. The live procedure is phase 8's first-apply
>    runbook; until that exists, treat this stage as history.

```bash
export TF_VAR_aks_cluster_id=$(az aks show -g rg-aks-panacea-dev -n aks-panacea-dev --query id -o tsv)

cd infra/terraform
tofu init
tofu plan -var-file=dev.tfvars -out=kata.plan   # REVIEW THIS before applying
tofu apply kata.plan

kubectl get nodes -l panacea-app/role=sandbox
```

`aks_cluster_id` deliberately has no default and no tfvars entry, so OpenTofu prompts rather than silently targeting the wrong cluster.

### The identifier trap — read before debugging anything

Three strings look interchangeable and are not:

| Where | Value |
|---|---|
| OpenTofu `workload_runtime` | `KataVmIsolation` |
| `az aks --workload-runtime` | `KataMshvVmIsolation` |
| Kubernetes `runtimeClassName` | `kata-mshv-vm-isolation` |

The plan originally specified the wrong value in the `.tf`; the provider rejected it and it is fixed. **The third one is unverified.** Nothing guarantees that setting `KataVmIsolation` produces a RuntimeClass registered under exactly `kata-mshv-vm-isolation`, which the pod spec hardcodes.

```bash
kubectl get runtimeclass
```

If the name differs from `kata-mshv-vm-isolation`, update `control-plane/internal/sandbox/manager.go` (the `runtimeClass` constant) to the real name. **Symptom if missed:** every sandbox pod stays Pending with "RuntimeClass not found."

---

## Stage 4 — one sandbox pod in-cluster

```bash
kubectl create namespace panacea-app --dry-run=client -o yaml | kubectl apply -f -
```

Create `sandbox-manual.yaml`. Note the volumes — they are load-bearing, not optional:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sandbox-manual
  namespace: panacea-app
spec:
  runtimeClassName: kata-mshv-vm-isolation
  nodeSelector:
    panacea-app/role: sandbox
  tolerations:
    - key: panacea-app/sandbox
      operator: Equal
      value: "true"
      effect: NoSchedule
  restartPolicy: Never
  volumes:
    - name: home
      emptyDir: {}
    - name: tmp
      emptyDir: {}
  containers:
    - name: sandbox
      image: <your-acr>.azurecr.io/panacea-app/sandbox:dev
      env:
        - name: PANACEA_APP_COMMAND
          value: /bin/bash
      ports:
        - containerPort: 8080
      volumeMounts:
        - name: home
          mountPath: /home/dev
        - name: tmp
          mountPath: /tmp
      resources:
        requests: { cpu: 500m, memory: 1Gi }
        limits:   { cpu: 500m, memory: 1Gi }
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        readOnlyRootFilesystem: true
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
```

```bash
time kubectl apply -f sandbox-manual.yaml
kubectl wait -n panacea-app --for=condition=Ready pod/sandbox-manual --timeout=300s
```

**Record the elapsed time.** This is the Kata cold-start number, and it determines warm-pool sizing in Phase 2. Expect seconds, not milliseconds.

**Why the volumes matter:** the image's only writable path is `/home/dev` (created by `useradd -m`, and the WORKDIR), and the root filesystem is read-only. Without these mounts the container starts and then fails on the first write — git clone, npm, shell history, the agent's own state. This was found by cross-task review and fixed; do not remove them.

**Why requests and limits are mandatory:** Kata sizes the guest VM from their sum. Under-request and the guest OOMs at runtime.

```bash
kubectl port-forward -n panacea-app pod/sandbox-manual 8099:8080
```

Connect the desktop app to `ws://localhost:8099/attach` and repeat the Stage 1b fidelity checklist — this time against a real microVM.

```bash
kubectl delete -f sandbox-manual.yaml
```

---

## Stage 5 — record the results

Write `docs/verification/YYYY-MM-DD-phase-1.md` containing:

- every command run and its real output
- the Kata cold-start duration from Stage 4
- pass/fail against each Stage 1b fidelity check, twice (local Docker, then in-cluster)
- the measured keystroke latency
- the actual RuntimeClass name observed
- any defect found

**State failures plainly.** A failed fidelity check changes the architecture, and hiding one wastes the entire phase. That is the only reason this phase exists.

---

## Known compliance note

Microsoft Defender for Containers **cannot assess Kata pods**. That is a documented platform limitation, not a configuration error, and it should be recorded as a compliance gap rather than discovered during a security review.
