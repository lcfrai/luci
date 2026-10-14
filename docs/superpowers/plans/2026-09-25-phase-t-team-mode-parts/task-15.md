### Task 15: Phase 8 delta — sandbox classes (task 14b), the policy mount, `sandboxd`'s digest check

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** 13 (the rendered file and its digest,
ST.4), P8-14 (**built**: `control-plane/internal/sandbox/manager.go` on `feat/env`, `09ae70a`),
P8-15 (**planned**: machine volumes, `phase-8 …/task-15.md`). Built here after `feat/env`'s
sync-merge into `feat/team-mode`. **Seams:** consumes ST.4; produces `Spec.Class` and
`Spec.PolicyFile`, which task 20 fills (`task-20.md:266-296`).

**This reopens built phase 8 task 14, as the design says it must.** `05-design.md` §11.2 names it
*task 14b* (line 1145): *"an `orchestrator` class with no machine disk and no affinity, its own
size (proposed 250m / 512 MiB), the `panacea-orchestrator` runner as its PTY command"*. The
reason is critique C2 (line 1390): an Orchestrator that took a machine disk and co-scheduled with
it would eat the worker slots the owner is paying for, and it has no worktree to edit anyway — it
sees the project through `panacea files` and `show`. The manager's `Spec` (`manager.go:69-83`)
gains `Class` and `PolicyFile`, and the pod builder (`manager.go:270-348`) branches on the class
in exactly three places: the command, the size defaults, and whether a machine volume may exist.

**Every class gets the policy, and it gets only the policy.** The rendered file (task 13) is
written into a per-pod Secret `<pod>-policy`, key `managed-settings.json`, and mounted read-only
as the whole directory `/etc/claude-code/`, Claude Code's Linux managed path
(`phase-6 …/task-14.md:11`, read from the docs on 2026-09-25; whether the **pinned** CLI version
reads it is **UNVERIFIED**, and task 33's `/status` check settles it). Mounting the directory and
not the one file is deliberate: the image's rootfs is read-only (`manager.go:337`,
`ReadOnlyRootFilesystem`), and a directory mount shadows **everything** the image put in
`/etc/claude-code/`, including phase 6's drop-in `managed-settings.d/50-panacea-gateway.json`
(`phase-6 …/task-14.md:16`). Nothing reaches Claude Code's managed scope that the digest does
not cover. The fragment is not lost: task 13 renders it into the file (layer 2).

**The Secret is `<pod>-policy`, beside the attach Secret, with the same lifecycle.** Task 20
already writes against that name (`task-20.md:289-293`). Phase 8's `Create` treats the attach
Secret as the id's claim and orders *Secret, Pod, owner reference* (`manager.go:352-363`); the
policy Secret joins that order second, is deleted with the attach Secret when a pod create is
refused, gets the same owner reference to the pod, and is deleted by `Delete`. A policy is not a
secret, but a Secret volume is what the kubelet projects as a read-only `tmpfs`, and it keeps the
file out of the pod spec, where every reader of pods would see it.

**Every `PANACEA_APP_*` variable is rendered by the manager from a typed field, never passed in
`Env`.** Phase 8 reserves the prefix: `backend.ValidateSpec` rejects a `PANACEA_APP_*` key in
`Env` (`internal/backend/backend.go:60-62,204`) and `containerEnv` drops one as its last line
(`sandbox/manager.go:52-55,230`), so that no caller can replace the attach token. Team mode needs
four more such variables, and each gets a field: `PANACEA_APP_CLASS` from `Spec.Class`,
`PANACEA_APP_AGENT_ID` from the existing `Spec.SessionID` (`manager.go:82`), and
`PANACEA_APP_CONTROL_TOKEN` from a new `Spec.ControlToken`, stored as key `control` in the attach
Secret and referenced like the attach token (`manager.go:218-223`), so neither token is in the pod
spec — the three task 16's `sandboxd` reads (`task-16.md:1072-1074`); and
`PANACEA_APP_POLICY_SHA256`, which the manager computes itself from `PolicyFile`, so the file and
its digest cannot disagree at the source. Task 20's spec builder puts four of these in `Env`
(`task-20.md:272-279`); they would be refused, and that is recorded as a gap: task 20 sets the
fields instead.

**The digest travels beside the file, and `sandboxd` checks one against the other before the
harness exists.** In the pod, `sandboxd`'s new `policycheck` runs **before** `newServer`
(`sandboxd/cmd/sandboxd/main.go:56`), which is where the PTY starts: it reads the mounted file,
recomputes the digest, and refuses to start if the file is absent, differs, or has company in the
directory. A refusal exits 3 with a message that never contains file content, before any process
the agent could influence has run.

**Local mode is untouched, and it is the only place a missing policy is allowed.** Phase 2's
Docker backend sets no `PANACEA_APP_POLICY_SHA256` and mounts nothing, so `policycheck` sees
*neither* and lets the harness start; OD-20 keeps local mode. The two half-states are refused:
the variable without the file (a mount that failed), and the file without the variable (a pod
built by something that is not this manager). On the cloud side the guarantee is the manager's:
`Create` refuses a spec with an empty `PolicyFile`, for every class.

**Files:**
- `control-plane/internal/sandbox/manager.go` (`feat/env`) — `Class`, `ClassWorker`, `ClassOrchestrator`, `OrchestratorCommand`, `Spec.Class`, `Spec.PolicyFile`, `Spec.ControlToken`, `PolicySecretKey`, `ControlSecretKey`, `PolicyMountPath`, `policySecretFor`, the class rules in `validateClass`, the four team variables in `containerEnv`, the `/etc/claude-code`, `/run/panacea` and `/panacea` volumes in `pod`, the second Secret in `Create` and `Delete` (modified)
- `control-plane/internal/backend/backend.go` (`feat/env`, built) — `backend.Spec` gains `Class`, `PolicyFile`, `ControlToken`; `ValidateSpec` checks them (modified)
- `control-plane/internal/backend/katabe/kata.go` — passes the three through to `sandbox.Spec` (`kata.go:105-115`) (modified)
- `control-plane/internal/backend/dockerbe/*.go` — refuses a spec with `Class` `orchestrator`, a `PolicyFile` or a `ControlToken`, `ErrUnsupported`: team sandboxes are cloud-only (OD-14), and local mode has no managed mount (modified)
- `control-plane/internal/sandbox/class_test.go` — the class shapes and the policy mount, on the fake clientset (new)
- `control-plane/internal/sandbox/manager_test.go` — the refused-create and delete tests extended to the second Secret (modified)
- `sandboxd/internal/policycheck/policycheck.go`, `policycheck_test.go` — `Check`, `Env`, `Dir`, the four refusals (new)
- `sandboxd/cmd/sandboxd/main.go` — the one call before `newServer`, and `policyDir`; `main_test.go` gains the tampered-file process test (modified)
- `testdata/policy/v1/{worker.json,worker.sha256}` — the shared digest fixture: `policy.Render(policy.Layers{}, worker)` and its digest (new)
- `control-plane/internal/policy/fixture_test.go` — `TestTheSharedFixtureIsCurrent`, which regenerates the fixture under `-update` and otherwise fails if `Render` no longer produces it (new)
- **Not** `images/sandbox/Dockerfile`: phase 6's image already creates `/etc/claude-code/` for its drop-in (`phase-6 …/task-14.md:202,260`), which is the mount point; if a merged image lacks it, the runtime creates the mount point before the rootfs goes read-only (runc's order; for the Kata agent **UNVERIFIED**, and task 33's first pod shows it)

**Interfaces:**
- Produces:
  - `sandbox.Class`, `sandbox.ClassWorker`, `sandbox.ClassOrchestrator`, `sandbox.OrchestratorCommand` (`/usr/local/bin/panacea-orchestrator`, task 27's binary, `task-27.md:89`)
  - `sandbox.Spec.Class`, `Spec.PolicyFile []byte`, `Spec.ControlToken string`; the same three on `backend.Spec`
  - `sandbox.PolicySecretKey` (`managed-settings.json`), `sandbox.ControlSecretKey` (`control`), `sandbox.PolicyMountPath` (`/etc/claude-code`), `sandbox.PolicySecretName(owner, id)`
  - the pod contract: env `PANACEA_APP_CLASS`, `PANACEA_APP_AGENT_ID`, `PANACEA_APP_CONTROL_TOKEN` (a `secretKeyRef`), `PANACEA_APP_POLICY_SHA256`; volumes at `/etc/claude-code` (read-only Secret), `/run/panacea` and `/panacea` (memory `emptyDir`s)
  - `policycheck.Check(dir, want string) error`, `policycheck.Env`, `policycheck.Dir`, `policycheck.File`, `ErrMissing`, `ErrMismatch`, `ErrExtra`, `ErrUnexpected`, `ErrMalformed`; `sandboxd` exit code 3
- Consumes:
  - P8-14, built — `Spec` (`manager.go:69-83`), `pod` (`:270-348`), `Create` and its order (`:350-401`), `Delete` (`:420-435`), `containerEnv` (`:189-242`), `envReservedPrefix` (`:52-55`), `backend.ValidateSpec` (`backend.go:204`), `katabe.Create` (`kata.go:95-120`)
  - P8-15, planned — `Spec.Machine` (`phase-8 …/task-15.md:71`: *""* *means no machine volume and no affinity*); **UNVERIFIED until merge**
  - task 13 — the rendered bytes and `sha256:` digest form (`policy.Digest`), re-implemented in `sandboxd`, which cannot import `control-plane` (skeleton file map); a shared fixture pins the two
  - task 16 — the three variables it reads (`task-16.md:1072-1074`), `/run/panacea/control.sock` and `/panacea/team.json` (`task-16.md:162-165`)
- Consumed by: task 20 (sets `Class`, `PolicyFile`, `ControlToken`, `SessionID` on the broker's spec), task 17 (the broker passes them through), task 27 (the runner is the orchestrator's command), task 33 (`/status`), task 58 (adds the `mirror` class here).

**The additions to `control-plane/internal/sandbox/manager.go`, complete.** New declarations,
beside `AttachSecretKey` (`manager.go:31-32`):

```go
// Class is a sandbox's class (05-design.md §2.1; task 14b). "" is a worker, so
// every Phase 8 caller that predates team mode keeps its shape.
type Class string

const (
	ClassWorker       Class = "worker"
	ClassOrchestrator Class = "orchestrator" // task 58 adds "mirror"
)

const (
	// OrchestratorCommand is the orchestrator class's one PTY command (task 27).
	OrchestratorCommand = "/usr/local/bin/panacea-orchestrator"
	// The orchestrator's size is proposed (05-design.md §11.2 line 1145) and
	// measured in task 47; a caller's CPU and Memory still win.
	orchestratorCPU    = "250m"
	orchestratorMemory = "512Mi"

	PolicySecretKey  = "managed-settings.json" // the policy Secret's one key
	ControlSecretKey = "control"               // the attach Secret's second key
	PolicyMountPath  = "/etc/claude-code"      // Claude Code's Linux managed directory
	maxPolicyBytes   = 64 << 10                // task 13's MaxLayerBytes bounds each layer; the file is smaller

	envClass   = "PANACEA_APP_CLASS"
	envAgentID = "PANACEA_APP_AGENT_ID"
	envControl = "PANACEA_APP_CONTROL_TOKEN"
	envPolicy  = "PANACEA_APP_POLICY_SHA256" // task 13's policy.EnvDigest
)

// PolicySecretName is PodName(owner, id) + "-policy", beside AttachSecretName.
func PolicySecretName(owner, id string) string { return policySecretFor(PodName(owner, id)) }

func policySecretFor(podName string) string { return podName + "-policy" }

func policyDigest(file []byte) string {
	sum := sha256.Sum256(file)
	return "sha256:" + hex.EncodeToString(sum[:])
}
```

`Spec` (`manager.go:69-83`) gains, after `SessionID`:

```go
	// Added in Phase T (task 15). Class shapes the pod; PolicyFile is the
	// rendered managed settings every cloud sandbox starts under (OD-23);
	// ControlToken is /control's bearer (task 16), "" for a session with no Fleet.
	Class        Class
	PolicyFile   []byte
	ControlToken string
```

`validateClass` runs first in `Create`, before any object exists, and normalises the spec:

```go
// validateClass is task 14b's three rules and OD-23's one. It returns the
// spec as the pod builder must see it.
func validateClass(spec Spec) (Spec, error) {
	if len(spec.PolicyFile) == 0 {
		return spec, fmt.Errorf("sandbox %q: PolicyFile is required: every cloud sandbox starts under a rendered policy (OD-23)", spec.ID)
	}
	if len(spec.PolicyFile) > maxPolicyBytes {
		return spec, fmt.Errorf("sandbox %q: PolicyFile is %d bytes, the limit is %d", spec.ID, len(spec.PolicyFile), maxPolicyBytes)
	}
	if spec.ControlToken != "" && spec.SessionID == "" {
		return spec, fmt.Errorf("sandbox %q: a ControlToken needs a SessionID, which sandboxd stamps on every control request", spec.ID)
	}
	switch spec.Class {
	case "", ClassWorker:
		spec.Class = ClassWorker
	case ClassOrchestrator:
		if spec.Machine != "" { // P8-15's field
			return spec, fmt.Errorf("sandbox %q: an orchestrator has no machine disk and no affinity (task 14b), got machine %q", spec.ID, spec.Machine)
		}
		if spec.Command != "" && spec.Command != OrchestratorCommand {
			return spec, fmt.Errorf("sandbox %q: an orchestrator runs %s, not %q", spec.ID, OrchestratorCommand, spec.Command)
		}
		spec.Command = OrchestratorCommand
		if spec.CPU == "" {
			spec.CPU = orchestratorCPU
		}
		if spec.Memory == "" {
			spec.Memory = orchestratorMemory
		}
	default:
		return spec, fmt.Errorf("sandbox %q: unknown class %q", spec.ID, spec.Class)
	}
	return spec, nil
}
```

**If P8-15 has not merged when this is built,** `Spec.Machine` does not exist yet: delete the
`Machine` check, keep a `// P8-15: refuse Machine here` line, and the orchestrator is diskless
trivially because no pod has a machine disk yet. The check lands with P8-15's merge, and
`TestAnOrchestratorRefusesAMachine` is written then; this is the one test in the task that waits.

`containerEnv` (`manager.go:206-242`) renders the four team variables after its three and before
the sorted `Env`; `backend.Spec.EnvEntries()` renders the same four in the same order, and phase 8's
pin test (`TestThePodEnvironmentIsExactlySpecEnvEntries`, `manager.go:199-205`) keeps the two equal:

```go
	env = append(env,
		corev1.EnvVar{Name: envClass, Value: string(spec.Class)},
		corev1.EnvVar{Name: envPolicy, Value: policyDigest(spec.PolicyFile)})
	if spec.SessionID != "" {
		env = append(env, corev1.EnvVar{Name: envAgentID, Value: spec.SessionID})
	}
	if spec.ControlToken != "" { // a reference, like the attach token: never a value in the pod spec
		env = append(env, corev1.EnvVar{Name: envControl, ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  ControlSecretKey,
			},
		}})
	}
```

`attachSecret` (`manager.go:256-268`) adds `ControlSecretKey: []byte(spec.ControlToken)` to `Data`
when the token is set. The policy Secret is its twin, one key, same labels:

```go
func (m *Manager) policySecret(spec Spec, name string) *corev1.Secret {
	s := m.attachSecret(spec, name) // the labels and namespace, one place
	s.Data = map[string][]byte{PolicySecretKey: spec.PolicyFile}
	return s
}
```

In `pod` (`manager.go:318-331`), three volumes and three mounts join `home` and `tmp`, for every
class. The worker's machine volume and affinity stay exactly as P8-15 writes them, gated on
`Spec.Machine`, which `validateClass` has already refused for an orchestrator:

```go
	policyMode := int32(0o444)
	runLimit, snapLimit := resource.MustParse("1Mi"), resource.MustParse("4Mi")
	volumes := []corev1.Volume{
		{Name: "home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		// OD-23: the whole managed directory, so nothing from the image is in scope.
		{Name: "policy", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName:  policySecretFor(name),
			Items:       []corev1.KeyToPath{{Key: PolicySecretKey, Path: PolicySecretKey}},
			DefaultMode: &policyMode,
		}}},
		// task 16's control socket and phase 6's token directory live under /run/panacea.
		{Name: "run-panacea", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory, SizeLimit: &runLimit}}},
		// task 16's team.json: memory-backed, outside the worktree (§5.3; ADR-H1's A5).
		{Name: "snapshot", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory, SizeLimit: &snapLimit}}},
	}
	mounts := []corev1.VolumeMount{
		{Name: "home", MountPath: "/home/dev"},
		{Name: "tmp", MountPath: "/tmp"},
		{Name: "policy", MountPath: PolicyMountPath, ReadOnly: true},
		{Name: "run-panacea", MountPath: "/run/panacea"},
		{Name: "snapshot", MountPath: "/panacea"},
	}
```

The two memory `emptyDir`s count against the container's memory limit, which is also the Kata
guest's size (`manager.go:331-332`); 5 MiB of 512 MiB is the Orchestrator's cost of them. If phase
6's merged manager already mounts something at `/run/panacea/gateway`, it nests under this volume
and both stay; if it mounts `/run/panacea` itself, reuse that volume and add none (step 4 checks).

`Create` (`manager.go:364-401`) keeps its order and inserts the policy Secret second:

```go
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	spec, err := validateClass(spec)
	if err != nil {
		return "", err // before any object: a bad spec leaves nothing behind
	}
	name := PodName(m.owner, spec.ID)
	secretName := attachSecretFor(name)
	policyName := policySecretFor(name)
	secrets := m.client.CoreV1().Secrets(m.cfg.Namespace)

	secret, err := secrets.Create(ctx, m.attachSecret(spec, secretName), metav1.CreateOptions{})
	if err != nil {
		return "", err // the claim is taken: touch nothing (unchanged)
	}
	cleanup := func(names ...string) error { // a fresh, bounded context, as before
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		var errs []error
		for _, n := range names {
			if derr := secrets.Delete(c, n, metav1.DeleteOptions{}); derr != nil && !apierrors.IsNotFound(derr) {
				errs = append(errs, fmt.Errorf("delete secret %s: %w", n, derr))
			}
		}
		return errors.Join(errs...)
	}
	pol, err := secrets.Create(ctx, m.policySecret(spec, policyName), metav1.CreateOptions{})
	if err != nil { // no pod yet: the claim goes too, so a retry can take it
		return "", errors.Join(err, cleanup(secretName, policyName))
	}

	created, err := m.client.CoreV1().Pods(m.cfg.Namespace).Create(ctx, m.pod(spec, name, secretName), metav1.CreateOptions{})
	if err != nil {
		if podCreateRefused(err) {
			if cerr := cleanup(secretName, policyName); cerr != nil {
				return "", errors.Join(err, fmt.Errorf("sandbox %q: after the pod was refused: %w", spec.ID, cerr))
			}
		}
		return "", err // ambiguous: both Secrets kept, Delete removes all three (unchanged rule)
	}

	for _, s := range []*corev1.Secret{secret, pol} {
		if err := m.ownByPod(ctx, s, created); err != nil {
			return created.Name, fmt.Errorf("sandbox %q: own secret %s by pod %s (the pod is kept; Delete removes all three): %w",
				spec.ID, s.Name, created.Name, err)
		}
	}
	return created.Name, nil
}
```

`ownByPod` is the existing owner-reference block (`manager.go:388-399`), moved into a method so it
runs for both Secrets. `Delete` (`manager.go:420-435`) deletes `policySecretFor(podName)` after the
attach Secret, with the same *absence is success* rule. The attach Secret stays the claim: the
policy Secret is created only by the request that holds it, and is never what a retry checks.

**`backend.Spec` and `katabe`.** `backend.Spec` (`backend.go:142-153`) gains `Class string`,
`PolicyFile []byte` and `ControlToken string` with the same comments; `ValidateSpec` refuses a
`Class` other than `""`, `worker` or `orchestrator` and a `PolicyFile` over 64 KiB, and **never logs
either token**; `katabe.Create` copies the three into `sandbox.Spec` beside `SessionID`
(`kata.go:105-115`), converting `Class` with `sandbox.Class(spec.Class)`. `dockerbe` returns
`ErrUnsupported` for an `orchestrator` class, a `PolicyFile` or a `ControlToken`: a local sandbox
has no managed mount, and a team sandbox in local mode would be one `policycheck` lets through
unchecked.

**`sandboxd/internal/policycheck/policycheck.go`, complete:**

```go
// Package policycheck makes sandboxd refuse to start the harness unless the
// managed-settings file the control plane mounted is the one it rendered
// (OD-23; Phase T task 15). It is standard library only: sandboxd vendors.
package policycheck

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	Env  = "PANACEA_APP_POLICY_SHA256"          // control-plane: policy.EnvDigest
	Dir  = "/etc/claude-code"                   // control-plane: sandbox.PolicyMountPath
	File = "managed-settings.json"              // control-plane: sandbox.PolicySecretKey
	maxFile = 64 << 10
)

var (
	ErrMissing    = errors.New("policycheck: a digest is set but the managed-settings file is absent")
	ErrMismatch   = errors.New("policycheck: the managed-settings file does not match its digest")
	ErrExtra      = errors.New("policycheck: the managed directory holds a file the control plane did not render")
	ErrUnexpected = errors.New("policycheck: a managed-settings file is present with no digest to check it against")
	ErrMalformed  = errors.New("policycheck: the digest is not sha256:<64 lower-case hex>")
)

var digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Check is the whole rule; want is Env's value, "" when unset. Local mode is
// "no digest and no file", and passes. No error ever quotes the file.
func Check(dir, want string) error {
	path := filepath.Join(dir, File)
	b, err := readBounded(path)
	absent := errors.Is(err, fs.ErrNotExist)
	if err != nil && !absent {
		return fmt.Errorf("policycheck: reading %s: %w", path, err)
	}
	switch {
	case want == "" && absent:
		return nil
	case want == "":
		return ErrUnexpected
	case !digestRE.MatchString(want):
		return ErrMalformed
	case absent:
		return ErrMissing
	}
	sum := sha256.Sum256(b)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%w: mounted %s, expected %s", ErrMismatch, got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("policycheck: listing %s: %w", dir, err)
	}
	for _, e := range entries {
		// A Secret volume holds the key's symlink plus the kubelet's own
		// "..data" and "..<timestamp>" entries; nothing else may be there,
		// or Claude Code would merge it (a managed-settings.d/ drop-in).
		if e.Name() == File || strings.HasPrefix(e.Name(), "..") {
			continue
		}
		return fmt.Errorf("%w: %s", ErrExtra, e.Name())
	}
	return nil
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFile {
		return nil, fmt.Errorf("policycheck: %s is over %d bytes", path, maxFile)
	}
	return b, nil
}
```

`sandboxd/cmd/sandboxd/main.go`, the one addition, immediately before `newServer` (`:56`):

```go
	// OD-23: the harness starts only under the policy the control plane
	// rendered. Before newServer, because newServer starts the PTY.
	if err := policycheck.Check(policyDir(), os.Getenv(policycheck.Env)); err != nil {
		slog.Error("refusing to start the harness", "err", err)
		os.Exit(3)
	}
```

**One fixture pins the two digests.** `control-plane` computes the digest in `policy.Digest` and in
`sandbox.policyDigest`; `sandboxd` in `policycheck`. The three share
`testdata/policy/v1/{worker.json,worker.sha256}` at the repository root — task 13's rendered
worker default and its digest — and each module's test reads it by relative path, as the ST.1
fixtures are read (skeleton file map). A change to any one of the three digest functions fails at
least one test against the shared bytes.

- [ ] **Step 1: Write the failing tests**

`control-plane/internal/sandbox/class_test.go` uses the fake clientset phase 8's tests already use
(`k8s.io/client-go/kubernetes/fake`), and a `classSpec(class)` helper returning a valid spec with
`PolicyFile` = the fixture's bytes.

| File · test | What it proves | The mutant it kills |
|---|---|---|
| sandbox · `TestAnOrchestratorPodHasNoMachineVolumeAndNoAffinity` | the pod's volumes are exactly `home`, `tmp`, `policy`, `run-panacea`, `snapshot`; `Affinity` is nil | the orchestrator given the worker's shape (C2) |
| sandbox · `TestAnOrchestratorRefusesAMachine` (after P8-15 merges) | `Machine: "m-0a1b2c3d"` → error, and the fake saw **no** create of any kind | the check made after the Secrets exist |
| sandbox · `TestAnOrchestratorRunsTheRunner` | `Command` `""` → the pod's `PANACEA_APP_COMMAND` is `/usr/local/bin/panacea-orchestrator`; `Command: "claude"` → error | a worker CLI run in a diskless pod |
| sandbox · `TestAnOrchestratorDefaultsTo250mAnd512Mi` | requests and limits `250m`/`512Mi` when unset; a caller's `500m`/`1Gi` wins | the class size lost, or forced over the caller's |
| sandbox · `TestAWorkerKeepsItsShape` | `Class ""` and `worker` give byte-identical pods, and (after P8-15) a worker with `Machine` keeps P8-15's volume and affinity | the class branch changing the worker |
| sandbox · `TestEveryClassMountsThePolicyReadOnly` | for both classes: Secret `<pod>-policy` holds exactly `{managed-settings.json: PolicyFile}`; volume `policy` projects only that key, mode `0444`; mount `/etc/claude-code`, `ReadOnly: true`; env `PANACEA_APP_POLICY_SHA256` = the fixture's digest | a writable mount; the attach token projected as a file; a digest of anything but the file |
| sandbox · `TestAPolicylessSpecCreatesNothing` | `PolicyFile` empty → error; the fake saw no create | a cloud sandbox with no policy (OD-23) |
| sandbox · `TestTheControlTokenIsAReference` | `ControlToken` set: the attach Secret has key `control`; `PANACEA_APP_CONTROL_TOKEN` is a `secretKeyRef`, and the token string appears nowhere in the marshalled pod | a token in the pod spec, readable by any pod reader |
| sandbox · `TestAControlTokenNeedsASessionID` | `ControlToken` without `SessionID` → error | a `/control` stream sandboxd cannot stamp |
| sandbox · `TestCallerEnvCannotSetTheTeamVariables` | `Env{"PANACEA_APP_CLASS": "orchestrator", "PANACEA_APP_POLICY_SHA256": "sha256:0…"}` on a worker → the pod has `worker` and the true digest, each once | a caller overriding the class or the digest through `Env` |
| sandbox · `TestARefusedPodDeletesBothSecrets` | the fake refuses the pod create with 422 → neither Secret remains | the policy Secret orphaned |
| sandbox · `TestAFailedPolicySecretReleasesTheClaim` | the fake fails the policy Secret create → the attach Secret is deleted too, and a second `Create` succeeds | an id locked forever by a half-made claim |
| sandbox · `TestBothSecretsAreOwnedByThePod` and `TestDeleteRemovesAllThree` | owner references on both; `Delete` removes pod and both Secrets, and tolerates any already gone | a Secret outliving its pod |
| backend · `TestTheEnvEntriesMatchThePod` (phase 8's pin, extended) | `EnvEntries()` and `containerEnv` agree for a team spec | the two orders drifting |
| dockerbe · `TestLocalModeRefusesTeamSpecs` | orchestrator, `PolicyFile`, `ControlToken` → `ErrUnsupported` each | a team sandbox run where no policy can be mounted |
| policycheck · `TestLocalModePasses` | `want ""`, no file → nil | local mode broken (OD-20) |
| policycheck · `TestEachHalfStateIsRefused` | `want` set, no file → `ErrMissing`; file, no `want` → `ErrUnexpected`; `want` `sha256:ABC` → `ErrMalformed` | a failed mount treated as local mode |
| policycheck · `TestATamperedFileIsRefused` | the fixture with one byte changed → `ErrMismatch`, and the message has no byte of the file | a check that trusts the mount; content leaking into logs |
| policycheck · `TestADropInIsRefused` | a `managed-settings.d/` directory beside the file → `ErrExtra`; `..data` and `..2026_09_25_…` entries → pass | a drop-in merged outside the digest |
| policycheck · `TestAnOversizedFileIsRefused` | 64 KiB + 1 → error | an unbounded read |
| sandboxd · `TestATamperedFileExitsBeforeThePTY` | below | the check placed after `newServer`, or its exit ignored |

The process test, complete, because *before the PTY starts* is the property and only a process can
show it. It uses the helper-process pattern: the test binary re-runs itself as `sandboxd`, with a
command that would leave a marker file if a PTY ever ran it.

```go
func TestATamperedFileExitsBeforeThePTY(t *testing.T) {
	if os.Getenv("SANDBOXD_HELPER") == "1" {
		os.Args = append([]string{"sandboxd"}, strings.Split(os.Getenv("SANDBOXD_ARGS"), "\x1f")...)
		main()
		return
	}
	dir, marker := t.TempDir(), filepath.Join(t.TempDir(), "pty-ran")
	good, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "policy", "v1", "worker.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "policy", "v1", "worker.sha256"))
	tampered := append(bytes.Clone(good[:len(good)-2]), []byte(" }")...) // same length, other bytes
	if err := os.WriteFile(filepath.Join(dir, "managed-settings.json"), tampered, 0o444); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestATamperedFileExitsBeforeThePTY$")
	cmd.Env = append(os.Environ(), "SANDBOXD_HELPER=1",
		"PANACEA_APP_POLICY_SHA256="+strings.TrimSpace(string(digest)),
		"PANACEA_APP_POLICY_DIR_FOR_TEST="+dir,
		"SANDBOXD_ARGS="+strings.Join([]string{"-addr", "127.0.0.1:0", "-allow-unauthenticated",
			"-command", "/bin/sh", "-args-json", `["-c","touch `+marker+`"]`}, "\x1f"))
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 3 {
		t.Fatalf("sandboxd exited %v; want exit 3 before the PTY. Output:\n%s", err, out)
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the PTY ran the command: the check came after newServer")
	}
	if bytes.Contains(out, tampered[:40]) {
		t.Fatal("sandboxd logged the file's content")
	}
}
```

The test needs the directory to be a temp dir, so `main.go`'s `policyDir()` is `policycheck.Dir`
unless `PANACEA_APP_POLICY_DIR_FOR_TEST` is set **and** the binary is a test binary
(`testing.Testing()`, Go 1.21+); a production `sandboxd` cannot be pointed elsewhere:

```go
func policyDir() string {
	if d := os.Getenv("PANACEA_APP_POLICY_DIR_FOR_TEST"); d != "" && testing.Testing() {
		return d
	}
	return policycheck.Dir
}
```

`testing.Testing()` reports true only in a test binary, so the override is dead code in the shipped
one. Importing `testing` into `main` adds nothing a user can reach; if a reviewer objects to the
import, the alternative is a `-policy-dir` flag hidden from `-help`, which a pod spec could then set,
which is worse.

Run: `cd control-plane && go test ./internal/sandbox/ ./internal/backend/... ./internal/policy/
-count=1` and `cd sandboxd && go test ./internal/policycheck/ ./cmd/sandboxd/ -count=1`. Expected
before step 2: compile failures naming `ClassOrchestrator`, `PolicySecretName`, `policycheck.Check`.

- [ ] **Step 2: The fixture.** Write `fixture_test.go`, run `go test ./internal/policy/ -run
  TestTheSharedFixtureIsCurrent -update`, then without `-update`. Expected: `ok`, and
  `testdata/policy/v1/worker.sha256` holds one `sha256:` line. Commit (with `git add` for the two new
  fixture files): `test(policy): the shared rendered-policy fixture for both modules`.

- [ ] **Step 3: `policycheck` and the `main.go` call.** Run the `sandboxd` command above. Expected:
  `ok` twice. Commit: `feat(sandboxd): refuse to start the harness unless the managed file matches its digest`.

- [ ] **Step 4: The manager, `backend.Spec`, `katabe`, `dockerbe`.** Before writing, check the
  merged tree: `grep -n "run/panacea\|Machine " control-plane/internal/sandbox/manager.go`. If
  P8-15's `Machine` is there, keep the `validateClass` check and write
  `TestAnOrchestratorRefusesAMachine`; if phase 6 already mounts `/run/panacea`, drop this task's
  `run-panacea` volume and mount. Run the `control-plane` command above. Expected: `ok` for every
  package. Commit: `feat(sandbox): task 14b's orchestrator class, the policy Secret and mount, the team variables`.

- [ ] **Step 5: Acceptance**

```
cd control-plane && go vet ./internal/sandbox/ ./internal/backend/... && go test ./internal/sandbox/ ./internal/backend/... ./internal/policy/ -count=1 -race
cd ../sandboxd && go vet ./... && go test ./... -count=1
cd .. && grep -rn "PANACEA_APP_POLICY_SHA256\|PANACEA_APP_CONTROL_TOKEN" control-plane/internal/sandbox/manager.go sandboxd/internal/policycheck/policycheck.go
```

Expected: `go vet` silent twice; every package `ok`; the `grep` prints exactly three lines — the two
constants in `manager.go` and `Env` in `policycheck.go` — so no second spelling of either name has
crept in. The real-pod half — that the kubelet projects the Secret at `/etc/claude-code/` on a Kata
guest with a read-only rootfs, that `sandboxd` exits 3 on a tampered file there, and that Claude
Code's `/status` shows this file as the managed source — is **deferred to task 33**, which runs it
on the real cluster (it needs phase 8's cluster and the pinned CLI, neither of which a unit test has).

**What this task does not claim.**
- That Claude Code reads `/etc/claude-code/managed-settings.json` at the pinned version, or that a
  directory with only that file is what it expects (**UNVERIFIED**; task 33's `/status`).
- That the Kata agent creates a missing mount point before the rootfs goes read-only
  (**UNVERIFIED**; phase 6's image already has the directory, so it should not arise).
- That the policy reaches a **solo** cloud session. OD-23 says every cloud sandbox; this task makes
  `PolicyFile` mandatory for every Kata pod, so phase 8's solo path must pass one, and the
  simplest correct value is `policy.Render(policy.Layers{}, model.ClassWorker)` (defaults and the
  fragment). Getting the org's layer into a solo session needs a policy lookup phase 8 does not
  plan; that is a gap, not built here.
- The mirror class (task 58), the Orchestrator's egress (task 32), and measuring 250m / 512 MiB
  (task 47).
- That `sandboxd`'s check stops an agent that later rewrites its own environment: the check is at
  start. The mount is read-only and root-owned by the kubelet, the agent runs as uid 1001 with
  every capability dropped (`manager.go:337-344`), so it cannot rewrite the file; it can set its own
  `env` for child processes, which is §7.3's open question and task 33's check.
