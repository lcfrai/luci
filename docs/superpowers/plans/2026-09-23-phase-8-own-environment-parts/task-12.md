### Task 12: Warm capacity — Kata balloons and the image pre-pull

`CLAUDE.md` makes warm pools mandatory ("Kata cold start is seconds"); `PROGRAM.md` open risk 1
says their sizing is unproven. Survey G-6 (`:427`) sets the order: **node-warm first**, then
measure; a pod-level pool (pre-booted pods claimed later) is not expressible today because the
command, argv and attach token are the pod's environment at start (`manager.go:98-127`).

Node-warm has three parts. The first is the sandbox pool's `min_count` (task 06) — Panacea's
"warm capacity is a floor, not a timeout" (survey §1.4, `:119-120`). This task adds the other two:
- **Balloons**: low-priority Kata pods the size of one session. A new session preempts a balloon
  instantly (priority ladder, task 11) and lands on an already-running node; the evicted balloon
  goes Pending, and the autoscaler adds a node for it **in the background**, so the next session
  also finds room.
  **Balloons warm new machines, not a busy one** (review I5). A session must land on its
  machine pod's node (required affinity on `kubernetes.io/hostname`, task 15), so a balloon helps
  it only when the balloon happens to sit on that node, and after one preemption the evicted
  balloon reschedules wherever there is room, usually elsewhere. What balloons buy is a warm Kata
  slot for the next **machine** (and that machine's first session). A machine whose node is full
  is not rescued by balloons or by the autoscaler, because a new node does not hold the machine
  pod: the broker refuses that session with `409 machine_full` within seconds and deletes the
  pod it tried (task 18), and the ceiling is stated in P8-D6.
- **Pre-pull**: a DaemonSet on every sandbox node whose init containers pull the sandbox and machine
  images by digest, so no session waits on a registry.

**Files:**
- `deploy/charts/platform/values.yaml`, `values.schema.json` — the `warm` block (modify)
- `deploy/charts/platform/templates/warm-balloons.yaml`, `warm-prepull.yaml` (new)
- `deploy/charts/platform/tests/warm_test.yaml` (new)

#### Contract — the `warm` values (complete; replaces `warm: {}`)

```yaml
warm:
  images:
    # Both by digest (P8-D8). The deploy pipeline passes the digests the images pipeline pushed.
    sandbox: ""   # e.g. acrpanaceaappdevk7q2.azurecr.io/panacea-app/sandbox@sha256:<64 hex>
    machine: ""
  balloons:
    # How many sessions' worth of headroom to keep beyond what is running. 0 disables balloons.
    replicas: 1
    # MUST equal the broker's default session size (PANACEA_APP_SESSION_CPU_MILLIS / _MEMORY_MIB);
    # task 29 passes both from one value so they cannot differ.
    cpu: 2000m
    memory: 4096Mi
  prepull:
    enabled: true
```

The schema requires both images to match `^[a-z0-9.\-]+(:[0-9]+)?/[a-z0-9._/\-]+@sha256:[a-f0-9]{64}$`
— a tag is refused.

#### What the templates render

`warm-balloons.yaml` — `Deployment/warm-balloon` in `panacea-sandboxes`, `replicas:
{{ .Values.warm.balloons.replicas }}`; pod template:
- labels `panacea-app/component: balloon`; annotation
  `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"`;
- `priorityClassName: panacea-balloon`, **`runtimeClassName: {{ .Values.sandboxes.runtimeClass }}`**
  (a balloon occupies a Kata slot the way a session does, overhead included),
  `nodeSelector: {panacea-app/role: sandbox}`, the `panacea-app/sandbox=true:NoSchedule` toleration;
- `automountServiceAccountToken: false`, `enableServiceLinks: false`;
- one container `balloon`, **image `warm.images.sandbox`**, `command: ["/bin/sleep", "infinity"]` —
  the real sandbox image, so a balloon also proves the image boots under Kata;
- `resources.requests == resources.limits == {cpu, memory}` from values (Kata sizes the guest VM
  from them, `manager.go:188-193`);
- `securityContext`: `runAsNonRoot: true`, `runAsUser: 1001`, `allowPrivilegeEscalation: false`,
  `capabilities.drop: [ALL]`, `readOnlyRootFilesystem: true`, `seccompProfile: RuntimeDefault`;
- `topologySpreadConstraints` over `kubernetes.io/hostname`, `whenUnsatisfiable: ScheduleAnyway`.

`warm-prepull.yaml` — `DaemonSet/image-prepull` in `panacea-sandboxes`, when `prepull.enabled`:
sandbox-node selector and toleration; **no** `runtimeClassName` (a plain runc pod: the pull goes
to the node's containerd image store); two `initContainers` (`pull-sandbox`, `pull-machine`) with
the two digests and `command: ["/bin/true"]`, **each with `requests: {cpu: 10m, memory: 16Mi}`
and limits the same**; one container `hold`, the sandbox image,
`command: ["/bin/sleep", "infinity"]`, `requests: {cpu: 10m, memory: 16Mi}`, limits the same;
the same restricted `securityContext`; `priorityClassName: panacea-session` (it must not be
preempted by a balloon).

**Why the init containers are sized (review I4).** An unsized container in `panacea-sandboxes`
takes the `LimitRange` defaults at admission, and LimitRanger defaults init containers too. The
scheduler sizes a pod by the larger of its biggest init container and the sum of its containers,
so one unsized init container makes the pre-pull pod as large as the default. With only a `max`,
the default *is* the max (8 CPU / 32 GiB): more than a `Standard_D8s_v5` can allocate, so the pod
sat Pending on every sandbox node, and Pending pods count against the quota — four of them hold
the namespace's whole 128 GiB. Task 11 now also sets the default to one session (2 CPU / 4 GiB),
but that would still cost one session of quota and one session of node per sandbox node. Sized at
`10m`/`16Mi`, the pod costs almost nothing. (The defaulting behaviour is from training knowledge
of Kubernetes, **UNVERIFIED** here; explicit sizes make it irrelevant.) Task 13's
`netpolcover` case `every_sandbox_container_is_sized` fails the build if any container here, init
or not, is left unsized or has requests ≠ limits.

**The pre-pull only helps if Kata pods start from the host's image store.** For AKS Pod Sandboxing
(not Confidential Containers) the host pulls and shares the rootfs into the guest — **from
training knowledge, UNVERIFIED**. If task 32 finds the guest pulls, the DaemonSet is removed and
this sentence says why.

#### Tests to write first (`tests/warm_test.yaml`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `balloon_is_lowest_priority_and_evictable` | `priorityClassName: panacea-balloon`; the safe-to-evict annotation is `"true"` | a balloon at session priority, which the next session cannot displace |
| `balloon_runs_under_kata` | `runtimeClassName` equals `sandboxes.runtimeClass` | a runc balloon that holds CPU but not a Kata slot's overhead, under-sizing headroom |
| `balloon_is_exactly_one_session` | requests == limits == `2000m` / `4096Mi` with default values | requests ≠ limits (Kata sizes the guest from them) |
| `balloon_lands_only_on_sandbox_nodes` | the sandbox nodeSelector and toleration are present | balloons spilling onto the apps pool |
| `balloon_passes_restricted` | runAsNonRoot, no escalation, drop ALL, RuntimeDefault seccomp, no SA token | a balloon the `restricted` namespace rejects, so warm capacity silently never exists |
| `images_must_be_digests` | `warm.images.sandbox: "x/sandbox:latest"` → render fails with the schema message | a mutable tag, which defeats OD-8's pin |
| `prepull_pulls_both_images_on_sandbox_nodes` | two init containers with the two digests; sandbox nodeSelector; no runtimeClassName | the machine image not pre-pulled, so a machine's first wake is cold |
| `prepull_init_containers_are_tiny` | `pull-sandbox` and `pull-machine` each have `requests == limits == {cpu: 10m, memory: 16Mi}` | an unsized init container, which LimitRanger defaults to a whole session and so holds a session's quota on every sandbox node (review I4) |
| `zero_replicas_is_allowed` | `balloons.replicas: 0` renders `replicas: 0` | a schema that forbids turning balloons off |

#### Steps

- [ ] **Step 1:** Values and schema; the suite fails.
- [ ] **Step 2:** The two templates until it passes; rerun task 11's suites (still green).
- [ ] **Step 3:** Commit.

#### Acceptance

```bash
helm unittest deploy/charts/platform
# Test Suites: 7 passed, 7 total
# Tests:       21 passed, 21 total
```

#### What this task does not claim

- **Any number.** How many balloons, how large a floor, and whether node-warm is enough are task
  32's measurements (cold start on a warm node vs a new node). The defaults are one session of
  headroom and one node of floor, chosen to be cheap, not right.
- That preemption of a Kata balloon frees its slot as fast as a runc pod's — the guest VM's
  shutdown is part of the eviction; **UNVERIFIED**.
