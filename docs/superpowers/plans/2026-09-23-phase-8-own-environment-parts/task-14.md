### Task 14: Harden the sandbox pod — RuntimeClass as config, no API token, the attach token in a Secret, owners

The pod `sandbox.Manager` builds (`control-plane/internal/sandbox/manager.go:139-210`) already runs
non-root with a read-only root, no escalation and every capability dropped (`:194-200`). Survey
G-4 (`:420`) lists what it lacks before it can share a cluster with other users' pods:

| Gap | Today | After this task |
|---|---|---|
| RuntimeClass | the constant `kata-mshv-vm-isolation` (`:19-20`, `:142`, `:160`) — seam S8.4, unverified (G-3) | `Config.RuntimeClass`, from `PANACEA_APP_RUNTIME_CLASS`, default `kata-vm-isolation` (P8-D15) |
| API credentials | the namespace's default service-account token is mounted (nothing turns it off) | `automountServiceAccountToken: false`, `enableServiceLinks: false` |
| Syscalls | no seccomp profile | `RuntimeDefault`, pod and container — also what `restricted` Pod Security requires (task 11) |
| The attach token | a plain env value in the pod spec (`:107-111`), readable by anything that can `get pods` — which contradicts `katabe`'s own claim that the token "is not recoverable by anything that can merely list pods" (`kata.go:229-231`) | a `Secret` per sandbox, referenced by `secretKeyRef`, owned by the pod so it is collected with it |
| Owner | none; `List` returns every pod in the namespace (`:240-247`) | `WithOwner(owner)` views: `Create` stamps `panacea-app/owner`, `Get`/`List` select on it (OQ-1) |
| Eviction | the autoscaler may evict a running session to consolidate | `cluster-autoscaler.kubernetes.io/safe-to-evict: "false"` |
| Priority | none | `Config.PriorityClass`, `panacea-session` in the cluster (task 11) |

This task changes **no behaviour on the laptop**: the Docker backend never builds a pod, and
`PANACEA_APP_BACKEND=kata` still refuses to run outside a cluster (`cmd/api/main.go:241-271`,
seam S8.2).

**Files:**
- `control-plane/internal/sandbox/manager.go` — `Config`, `NewManager(client, Config)`, `WithOwner`, `OwnerLabelValue`, the pod and Secret (modify)
- `control-plane/internal/sandbox/manager_test.go`, `manager_growth_test.go` — callers move to `Config`; two assertions change, named below (modify)
- `control-plane/internal/sandbox/hardening_test.go` (new)
- `control-plane/internal/backend/backend.go` — `KubeLabelOwner`; `KubeLabelComponent`'s comment (modify)
- `control-plane/internal/backend/katabe/kata.go` — `Config.Owner`; header comment (modify)
- `control-plane/internal/backend/katabe/kata_test.go` — one assertion changes, named below; one test added (modify)
- `control-plane/cmd/api/config.go`, `config_test.go`, `main.go` — `PANACEA_APP_RUNTIME_CLASS` (modify)
- `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` — §4 gains the
  `PANACEA_APP_RUNTIME_CLASS` row, as `backendScoped`'s comment requires (added as built)

#### Contract

```go
// package backend (backend.go), beside KubeLabelComponent:
const KubeLabelOwner = "panacea-app/owner" // OwnerLabelValue(tid, oid); absent on the laptop

// and KubeLabelComponent's comment (backend.go:76), which says `// always "sandbox"`, becomes
// (review m14):
KubeLabelComponent = "panacea-app/component" // "sandbox" (a session, this package), "machine" (task 19), "balloon" (task 12's chart)
```

The old comment goes stale in this phase: task 12's balloons and task 19's machine pods carry the
same key in the same namespace, and task 11's and task 29's NetworkPolicies select on its value.
Task 14 edits the line beside it, so it updates the comment; tasks 12 and 19 need not touch
`backend.go` again.

```go
package sandbox

// DefaultRuntimeClass is what AKS Pod Sandboxing is believed to install for
// workload_runtime = KataVmIsolation. UNVERIFIED (survey G-3): the first smoke
// check of phase 8 (task 32) is `kubectl get runtimeclass`, and the value is
// configuration precisely so that a wrong guess is an environment change.
const DefaultRuntimeClass = "kata-vm-isolation"

// Config configures a Manager. It replaces the bare namespace argument.
type Config struct {
	// Namespace holds every pod and Secret this manager creates. Required.
	Namespace string
	// RuntimeClass is every pod's runtimeClassName. "" means DefaultRuntimeClass.
	RuntimeClass string
	// PriorityClass is every session pod's priorityClassName. "" sets none — a
	// fake clientset or a kind cluster has no panacea-session class, and a pod
	// naming a class that does not exist is rejected at admission.
	PriorityClass string
}

// Manager creates and removes sandbox pods in one namespace, optionally scoped
// to one owner.
type Manager struct {
	client kubernetes.Interface
	cfg    Config
	owner  string // "" = unscoped (the in-cluster tests, and phase 2's direct callers)
}

// NewManager returns an unscoped Manager.
func NewManager(client kubernetes.Interface, cfg Config) *Manager

// WithOwner returns a view of m that stamps backend.KubeLabelOwner = owner on
// every pod and Secret it creates, and whose Get and List select on it, so one
// owner can neither see nor reach another's sandbox. It also names objects
// "sandbox-<owner[:8]>-<id>" (and "…-attach" for the Secret) instead of
// "sandbox-<id>", so two owners whose laptops happen to mint the same id never
// collide — and a collision cannot reveal that another owner's id exists. The
// longest pod name is 8 + 8 + 1 + 40 = 57 characters, inside the 63 a pod's
// hostname is cut to; names are DNS subdomains, so the Secret's 64 is fine.
// owner must come from OwnerLabelValue. It panics on "" — an owner view with no owner is the bug
// this method exists to prevent.
func (m *Manager) WithOwner(owner string) *Manager

// OwnerLabelValue is the owner label for an Entra principal: the first 32 lower-
// case hex characters of sha256(tenantID + "/" + objectID). Label-safe (≤ 63),
// stable across sessions, and not an object id anyone can read back.
func OwnerLabelValue(tenantID, objectID string) string

// PodName is the one place a sandbox id becomes a pod name (it replaces
// podNameFor, manager.go:69). owner "" (the unscoped manager) gives
// "sandbox-<id>"; an OwnerLabelValue gives "sandbox-<owner[:8]>-<id>", the name
// WithOwner(owner) creates. Exported because the broker's rollback (task 18,
// deleteIfOurs) must name the pod its request created (b09 re-review 2).
func PodName(owner, id string) string

// AttachSecretName is PodName(owner, id) + "-attach": the Secret holding that
// sandbox's attach token.
func AttachSecretName(owner, id string) string

// AttachSecretKey is the Secret's one data key.
const AttachSecretKey = "token"
```

```go
package katabe

type Config struct {
	Manager      *sandbox.Manager
	ReadyTimeout time.Duration
	Port         int
	// Owner, when set, scopes this backend to one owner: New wraps Manager in
	// Manager.WithOwner(Owner). The broker builds one Backend per request with
	// the caller's OwnerLabelValue. "" is the unscoped backend cmd/api builds.
	Owner string
}
```

```go
// cmd/api/config.go — one more constant and one more backendScoped row:
envRuntimeClass = "PANACEA_APP_RUNTIME_CLASS" // kata only; default sandbox.DefaultRuntimeClass
// Config gains:  RuntimeClass string
// Validation: a DNS-1123 subdomain (^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$,
// ≤ 253), else exit 2 naming the variable.
// main.go: sandbox.NewManager(client, sandbox.Config{Namespace: cfg.Namespace, RuntimeClass: cfg.RuntimeClass})
```

#### The pod after this task (the differences only)

- `metadata.annotations`: `cluster-autoscaler.kubernetes.io/safe-to-evict: "false"`.
- `metadata.labels`: `panacea-app/owner: <owner>` in an owner view.
- `spec.runtimeClassName: cfg.RuntimeClass` (or the default); `spec.priorityClassName` when set.
- `spec.automountServiceAccountToken: false`; `spec.enableServiceLinks: false`.
- `spec.securityContext`: `runAsNonRoot: true`, `runAsUser: 1001`, `runAsGroup: 1001`,
  `fsGroup: 1001`, `fsGroupChangePolicy: OnRootMismatch` (task 15's disk must be writable by uid
  1001), `seccompProfile: {type: RuntimeDefault}`.
- container `securityContext` gains `seccompProfile: {type: RuntimeDefault}`.
- env `PANACEA_APP_ATTACH_TOKEN` becomes `valueFrom.secretKeyRef {name: AttachSecretName(owner, id), key: token}`
  (`sandbox-<id>-attach` unscoped, `sandbox-<owner[:8]>-<id>-attach` in an owner view)
  with an empty `value`; the other entries, and their order, are unchanged.

**Create's order:** Secret (labels as the pod's) → Pod → update the Secret's `ownerReferences` to
the Pod (`controller: false`, `blockOwnerDeletion: false`).
- **The Secret is the id's claim.** A create of a name whose Secret exists fails at the Secret
  with `AlreadyExists`, returned unwrapped (katabe maps it to `ErrAlreadyExists`, `kata.go:102-103`),
  and the existing Secret is not touched. So only the request that created the Secret can go on
  to create the pod, and the token in the Secret names that request (task 18's `deleteIfOurs`
  relies on this, b09 re-review 2).
- **A refused Pod create** (an `apierrors.APIStatus` reached by `errors.As` whose code is 4xx,
  other than `IsTimeout` and `IsServerTimeout`: the API server answered, so the pod was not
  stored): the Secret is deleted before the error returns.
- **An ambiguous Pod create** (any other error: transport, the context ending, a timeout status,
  **any 5xx**; the pod may have been stored): the Secret is **kept** and the error returned.
  *Amended by the implementation (b09-2 Minor 4): an etcd failure during the write answers 500 or
  503 and may still have committed it, so "the API server answered" does not prove "not stored"
  for a 5xx.* The caller holds
  the proof of which pod is its own, and `Delete` removes both. Deleting the Secret here would
  leave a stored pod with no identity to check, which the broker could then never roll back.
- If the owner-reference update fails, the Pod is kept and the Secret is deleted by `Delete`,
  which now removes **both** and treats either's absence as success.

A token Secret with no owner reference can therefore exist only between a failed update or an
ambiguous create and the next `Delete` or reap (the broker's rollback is such a `Delete`; in the
unscoped path, an ambiguous create whose pod was never stored leaves an inert Secret that no pod
reads, until that id's `Delete`).

#### Tests to write first

New, in `hardening_test.go` (fake clientset, the package's existing `growthSpec()` helper):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateUsesTheConfiguredRuntimeClass` | `Config{RuntimeClass: "rc-x"}` → `*pod.Spec.RuntimeClassName == "rc-x"` | the constant left in `Create` |
| `TestCreateDefaultsTheRuntimeClass` | `Config{}` → `DefaultRuntimeClass` | an empty runtimeClassName, which schedules a **runc** pod onto a Kata node |
| `TestPodMountsNoServiceAccountToken` | `*pod.Spec.AutomountServiceAccountToken == false` | the default token mounted — the API server reachable with credentials |
| `TestPodHasNoServiceLinks` | `*pod.Spec.EnableServiceLinks == false` | every Service's address injected as env |
| `TestPodAndContainerUseRuntimeDefaultSeccomp` | both profiles `RuntimeDefault` | unconfined syscalls; `restricted` rejects the pod |
| `TestPodSecurityContextOwnsItsVolumesAsTheAgentUser` | `FSGroup == 1001`, `RunAsGroup == 1001` | a disk the agent cannot write (task 15) |
| `TestAttachTokenIsInASecretNotThePodSpec` | the token env entry has `Value == ""` and a `SecretKeyRef` to `sandbox-<id>-attach`/`token`; that Secret's `Data["token"]` is the token; **no** env `Value` anywhere in the pod equals the token | the token back in the spec |
| `TestTheTokenSecretIsOwnedByItsPod` | the Secret's single owner reference is `Kind: Pod`, `Name: sandbox-<id>` | an orphan secret that outlives the sandbox |
| `TestCreateDeletesTheSecretWhenThePodIsRefused` | a reactor fails pod creation with `apierrors.NewForbidden(…)` → error returned, no Secret left | a token left behind by every refused create |
| `TestCreateKeepsTheSecretWhenThePodCreateIsAmbiguous` | a reactor fails pod creation with `context.DeadlineExceeded` (and, as built, a transport error, `Timeout` 504, `ServerTimeout`, `InternalError` 500, `ServiceUnavailable` 503) → error returned, the Secret is still there with the token (b09 re-review 2; b09-2 Minor 4) | the Secret deleted on every error, which leaves a stored pod with no identity for the broker's rollback |
| `TestCreateLeavesAnExistingSecretAlone` | a Secret `AttachSecretName("", id)` holding `tok-A` exists; a create of `id` with `tok-B` → `apierrors.IsAlreadyExists(err)`, no pod created, and the Secret still holds `tok-A` | a create that overwrites or deletes another request's claim |
| `TestPodNameIsWhatTheOwnerViewCreates` | `PodName("", "s-1")` is `sandbox-s-1`; `WithOwner(o).Create` of `s-1` makes a pod named `PodName(o, "s-1")` = `sandbox-<o[:8]>-s-1` and a Secret named `AttachSecretName(o, "s-1")` | a broker that names its rollback's pod differently from the pod it created, so the rollback is a no-op |
| `TestDeleteRemovesPodAndSecretAndIsIdempotent` | after `Delete` both are gone; a second `Delete` returns nil | a leaked Secret, or a 500 on the reaper's second pass |
| `TestOwnerViewStampsTheLabel` | `WithOwner("o1").Create` → pod and Secret labelled `panacea-app/owner=o1` | an unlabelled pod the owner cannot find again |
| `TestOwnerViewsCannotSeeEachOther` | create under `o1`; `WithOwner("o2").Get(id)` → `ErrNotFound`; `.List` → empty; the unscoped manager lists it | the cross-user read OQ-1 names |
| `TestTwoOwnersMayUseTheSameID` | `o1` and `o2` both create id `s-1234abcd` → both succeed, two pods, each view gets its own | `sandbox-<id>` names, where the second owner's create fails with a conflict that proves the first exists |
| `TestWithOwnerRefusesAnEmptyOwner` | `WithOwner("")` panics | a silently unscoped "scoped" view |
| `TestOwnerLabelValueIsStableAndLabelSafe` | same inputs → same 32-hex value; different `oid` → different; matches `^[a-f0-9]{32}$` | a value that changes per call, or embeds the raw oid |
| `TestSessionPodIsNotSafeToEvict` | the annotation is `"false"` | the autoscaler killing a running agent to save a node |
| `TestPriorityClassOnlyWhenConfigured` | `Config{}` → empty; `Config{PriorityClass: "panacea-session"}` → set | a pod rejected on a cluster without the class |
| `TestAFailedOwnerReferenceKeepsThePodAndDeleteRemovesBoth` *(added as built)* | a reactor fails the Secret `update` → `Create` returns an error **and** `PodName("", id)`; the pod exists; `Delete` then leaves no Secret | a rollback of a stored pod, or a Secret `Delete` forgets |
| `TestDeleteKeepsTheSecretWhenThePodDeleteFails` *(added as built)* | a reactor fails the pod `delete` with 503 → `Delete` errors and the Secret is still there | a Secret deleted first, which leaves a live pod whose claim is gone |

*As built, also:* `TestTheTokenSecretIsOwnedByItsPod` stamps a UID in a pod-create reactor and
asserts the owner reference carries it (the fake assigns none; a real API server rejects an empty
`uid`, b09-2 Minor 1), and `TestWithOwnerRefusesAnEmptyOwner` also refuses `O1`, `o 1` and
`o1,panacea-app/component=sandbox`: `WithOwner` panics on anything outside `^[a-z0-9]{1,63}$`,
because the owner is concatenated into a label selector.

In `katabe/kata_test.go`, new: `TestAnOwnerScopedBackendCannotAttachToAnothersSandbox` — create
through `New(Config{Owner: "o1"})`, then `Attach` and `Destroy` through `Owner: "o2"` →
`Attach` is `ErrNotFound`, and the pod and its Secret still exist (kills a broker that scopes List
but not Attach). *As built: `Destroy` of an id this view cannot see is `nil`, not `ErrNotFound`,
because `Destroy` is idempotent (`TestDestroyOfAnUnknownSandboxIsNil`); the test accepts either
and asserts the pod survives.*

In `cmd/api/config_test.go`, new: `TestRuntimeClassIsReadUnderKata`,
`TestRuntimeClassUnderDockerIsAWarning` (the `backendScoped` rule: no variable silently ignored,
`config.go:65-81`), `TestRuntimeClassMustBeADNSSubdomain`.

**Existing assertions this task changes, deliberately, in the same commit:**
1. `manager_test.go:34-36` (`TestCreateProducesAKataPodWithExplicitResources`) wants
   `kata-mshv-vm-isolation`; it becomes `sandbox.DefaultRuntimeClass`.
2. `manager_growth_test.go:61-70` (`TestCreateInjectsTheSandboxdContractEnvironment`) wants the
   token as a `Value`; the third entry becomes "name `PANACEA_APP_ATTACH_TOKEN`, empty value,
   `SecretKeyRef` to `sandbox-<id>-attach`".
3. `kata_test.go:355-377` (`TestThePodEnvironmentIsExactlySpecEnvEntries`) compares names and
   values pairwise; for the token entry it now compares the name and resolves the `SecretKeyRef`
   through the fake clientset to compare the value. The test's purpose — katabe and dockerbe give
   the agent the same environment — is unchanged: the process sees the same variable.
4. *Found by the implementation, not named above:* `manager_growth_test.go:107-111`
   (`TestCreateAppendsExtraEnvironmentSortedAndDropsReservedKeys`) proved the reserved key was
   dropped by reading the token's `Value`; it now requires the token entry to be the
   `secretKeyRef` with an empty value, and no entry to carry the caller's `stolen`.
5. *Likewise:* `kata_test.go:337-339` (`TestCreateRendersKubernetesQuantitiesAndTheArgvContract`)
   read the token's `Value`; it now resolves the entry through the fake clientset, as 3 does.

Every `sandbox.NewManager(client, "ns")` call in tests becomes `NewManager(client,
sandbox.Config{Namespace: "ns"})`.

#### Steps

- [ ] **Step 0: Phase 3 touches the same environment.** Phase 3's task 05 adds one variable to
  `backend.Spec.EnvEntries()` and to `containerEnv`, and extends the same env-parity test (phase 3
  plan, sibling `phase-3` worktree, file map: "`internal/backend/backend.go` (p2) EnvSessionID,
  written by EnvEntries … `katabe/kata_test.go` (p2) the env-parity pin gains one variable").
  Phase 3 lands first in program order. Check with `git grep -n EnvSessionID -- control-plane/`;
  if present, the pod's env order below includes it and assertion 3 keeps it; if absent, record
  that in the commit message so phase 3's task 05 knows the token entry moved to a Secret.
- [ ] **Step 1:** Write `hardening_test.go` and the three config tests. They fail to compile
  (`Config`, `WithOwner` missing).
- [ ] **Step 2:** Add `Config`, change `NewManager`, fix every caller (`main.go:231` and the tests).
  The three existing assertions above now fail for the reasons named; change them.
- [ ] **Step 3:** Implement the pod changes, the Secret and its order, `WithOwner`,
  `OwnerLabelValue`, `katabe.Config.Owner`, the config variable. All tests pass.
- [ ] **Step 4:** Rewrite `katabe`'s package comment: it is no longer "a configuration change and
  not a rewrite" (`kata.go:1-4`); say that phase 8 runs it **inside the broker** (task 18) and that
  mounts arrive as machine mounts (task 15). Update `Attach`'s comment (`kata.go:229-231`) to name
  the Secret as where the token now lives in the cluster. Replace `KubeLabelComponent`'s
  `// always "sandbox"` (`backend.go:76`) with the three values in the contract above (review
  m14); `grep -n 'always "sandbox"' control-plane/internal/backend/backend.go` prints nothing.
- [ ] **Step 5:** `go vet`, the full test run, `git diff control-plane/go.mod` (no change), commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1
# ok … every package
go vet ./sandboxd/... ./control-plane/...
git diff --stat control-plane/go.mod go.work          # empty
grep -n "kata-mshv-vm-isolation" control-plane/internal/sandbox/manager.go   # no output
```

#### What this task does not claim

- That `kata-vm-isolation` is the RuntimeClass AKS installs (task 32).
- That a Secret-backed env var reaches a Kata guest exactly as a literal does — Kubernetes resolves
  it before the runtime sees the spec, so it should; **unobserved** on Kata.
- Per-user namespaces: P8-D5 chose one namespace with owner labels.
