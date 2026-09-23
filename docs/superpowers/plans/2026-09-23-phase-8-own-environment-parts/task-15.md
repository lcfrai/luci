### Task 15: Machine mounts — how a worktree and its conversations reach a Kata pod (S8.3's Kata half)

Seam S8.3 is phase 8's first problem (`task-26.md:984-994`): every session create sends a host
mount of the worktree and a working directory (`control-plane/internal/api/sessions.go:430-431`),
and `katabe.Create` refuses both (`katabe/kata.go:84-89`), correctly — a host path on an AKS node
is meaningless. The resume fix added a second mount, the conversation store at
`/home/dev/.claude/projects` (`sessions.go:35-51`, `:94-101`), and the Go contract recorded that
Kata must supply it "as a volume that outlives the pod, scoped per user and per worktree"
(`2026-09-18-phase-2-go-contract.md:781-788`).

P8-D6 answers both: a cloud machine owns one disk, and a session mounts **sub-paths of it**. This
task adds the vocabulary for that to the backend seam and teaches `katabe` to render it. It does
**not** decide which sub-paths a session gets — that is the control plane's, in task 24 — and it
does **not** drop the host-mount refusal: that stays, and its test stays green, exactly as S8.3
says ("the refusal stays until that design replaces the mount, never the other way round").

**Files:**
- `control-plane/internal/backend/backend.go` — `MachineMount`, `Spec.Machine`, `Spec.MachineMounts`, `ValidateSpec` (modify)
- `control-plane/internal/backend/backend_test.go` (modify: new tests)
- `control-plane/internal/backend/dockerbe/docker.go`, its test — refuse machine mounts (modify)
- `control-plane/internal/sandbox/manager.go` — `Spec.Machine`, `Spec.MachineMounts`, `Spec.WorkDir`, `Config.MachineAffinity`; the volume, mounts, working dir and affinity (modify)
- `control-plane/internal/sandbox/machine_mounts_test.go` (new)
- `control-plane/internal/backend/katabe/kata.go`, `kata_test.go` — WorkDir honoured, machine mounts passed through (modify)

#### Contract

```go
package backend

// MachineMount is a path on the sandbox's machine disk exposed inside the
// sandbox. It is the cloud machine's answer to Mount: a Mount names a host
// path, which on an AKS node is meaningless (katabe refuses it); a
// MachineMount names a path relative to the root of the disk the machine owns
// (P8-D6), which the pod mounts as a sub-path of that machine's claim.
//
// The two are never mixed in one Spec: a sandbox runs on exactly one machine.
type MachineMount struct {
	// MachinePath is slash-separated and relative to the machine disk's root:
	// clean (path.Clean(p) == p), not ".", no ".." segment, no leading "/", and
	// only [A-Za-z0-9._/-]. e.g. "lanes/p-1a2b3c4d/w-5e6f7a8b".
	MachinePath   string
	ContainerPath string // absolute container path, e.g. "/home/dev/work"
	ReadOnly      bool
}

// Spec gains two fields, after Mounts:
//
//	Machine       string         // the cloud machine this sandbox runs on, ^m-[0-9a-f]{8}$; "" on the local machine
//	MachineMounts []MachineMount // requires Machine; kata only
//
// ValidateSpec gains, each wrapping ErrUnsupported:
//   - Machine, when set, must match ^m-[0-9a-f]{8}$
//   - MachineMounts requires Machine
//   - Mounts and MachineMounts together are refused
//   - every MachinePath obeys the rules above; every ContainerPath is absolute
```

```go
package sandbox

// VolumeMount is one sub-path of the machine claim, as the pod will mount it.
type VolumeMount struct {
	SubPath   string // backend.MachineMount.MachinePath, already validated
	MountPath string
	ReadOnly  bool
}

// Spec gains:
//
//	WorkDir       string        // the container's workingDir; "" leaves the image default
//	Machine       string        // the machine id; "" means no machine volume and no affinity
//	MachineMounts []VolumeMount // rendered only when Machine is set
//
// Config gains:
//
//	// MachineAffinity co-schedules every session pod with its machine's pod
//	// (required pod affinity on kubernetes.io/hostname), which a ReadWriteOnce
//	// disk needs (P8-D6). false only with shared (RWX) machine storage — the NFS
//	// fallback task 32 may select.
//	MachineAffinity bool

// MachineClaimName is the PersistentVolumeClaim holding machine id's disk.
// Task 19 creates it; this task only references it.
func MachineClaimName(machineID string) string { return "machine-" + machineID }

// Labels this task adds to backend's set:
//   backend.KubeLabelMachineID = "panacea-app/machine-id"  (session pods and, task 19, machine pods)
```

#### The pod, when `Spec.Machine` is set

- label `panacea-app/machine-id: <machine>`;
- volume `machine`: `persistentVolumeClaim.claimName: MachineClaimName(machine)`;
- one `volumeMount` per `MachineMounts` entry: `name: machine`, `subPath`, `mountPath`,
  `readOnly`;
- `containers[0].workingDir: Spec.WorkDir` when set;
- when `Config.MachineAffinity`: `affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution`
  = one term, `labelSelector` `{panacea-app/component: machine, panacea-app/machine-id: <machine>}`,
  `topologyKey: kubernetes.io/hostname`.

`/home/dev` and `/tmp` stay `emptyDir` (`manager.go:171-187`): `~/.claude` outside `projects/`
holds `/login`'s credentials, which must **not** land on the machine disk beside transcripts (the
resume fix's own reason, `sessions.go:44-48`; seam S8-P6.1).

**Sub-path directories must exist before the pod starts.** If a sub-path is missing, the kubelet
creates it — as root — and the agent (uid 1001) cannot write to it. The machine agent creates
both directories, owned by 1001, before the control plane creates the session (task 24 calls
task 20's `POST /v1/dirs`). Whether `fsGroup` repairs a root-created sub-path under Kata is
**UNVERIFIED** and is on task 32's list; the design does not rely on it.

#### `katabe.Create` after this task

```go
// Mounts: still refused, unchanged (a host path on an AKS node is meaningless).
// WorkDir: now honoured — it becomes the container's workingDir.
// MachineMounts: passed through as sandbox.VolumeMount, with Machine.
```

#### Tests to write first

`backend/backend_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestValidateSpecAcceptsAMachineMount` | `Machine: "m-0123abcd"`, one mount `lanes/p-1/w-2` → nil | a validator that rejects every machine mount |
| `TestValidateSpecRejectsMachineMountsWithoutAMachine` | `MachineMounts` set, `Machine: ""` → `ErrUnsupported` | mounts of an unnamed disk |
| `TestValidateSpecRejectsEscapingMachinePaths` | each of `../x`, `a/../b`, `/abs`, `.`, `a//b`, `a/`, `a b` → `ErrUnsupported` | a sub-path that climbs out of the owner's disk — `..` is the whole attack |
| `TestValidateSpecRejectsHostAndMachineMountsTogether` | both set → `ErrUnsupported` | a sandbox "on two machines" |
| `TestValidateSpecRejectsABadMachineID` | `Machine: "M-XYZ"` → `ErrUnsupported` | a claim name built from arbitrary text |

`dockerbe` test: `TestCreateRejectsMachineMounts` — `ErrUnsupported`, and the fake Docker client
saw no `ContainerCreate` (kills a dockerbe that silently ignores the field and starts an agent in
an empty directory).

`sandbox/machine_mounts_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMachineSpecMountsTheMachineClaim` | a volume `machine` with `claimName == "machine-m-0123abcd"` | the wrong claim — another machine's disk |
| `TestMachineMountsAreSubPaths` | each mount has `subPath == MachinePath`, `mountPath == ContainerPath`, `readOnly` carried | the whole disk mounted at `/home/dev/work` (every project visible to every session) |
| `TestWorkDirBecomesTheWorkingDirectory` | `workingDir == "/home/dev/work"` | the agent starting in the image default (`/home/dev`) |
| `TestSessionIsCoLocatedWithItsMachine` | `MachineAffinity: true` → one required term with the two-label selector and `kubernetes.io/hostname` | preferred (not required) affinity — a pod scheduled where the disk cannot attach, stuck in ContainerCreating |
| `TestNoAffinityForSharedStorage` | `MachineAffinity: false` → no affinity | the NFS fallback unable to spread sessions |
| `TestNoMachineMeansNoMachineVolume` | `Machine: ""` → no `machine` volume, no affinity, no machine label | a local-style spec referencing a claim that does not exist |
| `TestCredentialsStayOffTheDisk` | with machine mounts, `/home/dev` is still the `home` emptyDir and no mount targets `/home/dev/.claude` itself | `/login` credentials persisted beside transcripts |

`katabe/kata_test.go`:
- **Replace** `TestCreateRejectsAWorkDir` (`:228-237`) with `TestCreateSetsTheWorkingDirectory` —
  S8.3's "the one that flips first names the side that changed": this is the Kata side changing,
  and the commit message says so.
- Add `TestCreatePassesMachineMountsThrough` (the pod has the sub-path mounts).
- **Keep** `TestCreateRejectsMountsAndCreatesNoPod` (`:207-226`) unchanged and green.

#### Steps

- [ ] **Step 1:** Write every test above; they fail to compile.
- [ ] **Step 2:** `backend.go` types and validation; `dockerbe` refusal; the backend and dockerbe
  tests pass.
- [ ] **Step 3:** `sandbox.Spec`/`Config`/`VolumeMount`/`MachineClaimName`; the pod rendering; the
  sandbox tests pass.
- [ ] **Step 4:** `katabe.Create`: drop the WorkDir refusal (and its test, replaced), pass machine
  mounts through. `TestCreateRejectsMountsAndCreatesNoPod` still passes.
- [ ] **Step 5:** Update `backend.Mount`'s comment (`backend.go:124-130`), which says the
  conversation store is "one more thing Phase 8 must provide another way": name `MachineMount` as
  that way. Full test run, commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1          # ok, every package
go test ./control-plane/internal/backend/katabe/ -run 'TestCreateRejectsMountsAndCreatesNoPod|TestCreateSetsTheWorkingDirectory' -v -count=1
# --- PASS: TestCreateRejectsMountsAndCreatesNoPod
# --- PASS: TestCreateSetsTheWorkingDirectory
grep -c "func TestCreateRejectsAWorkDir" control-plane/internal/backend/katabe/kata_test.go   # 0
```

#### What this task does not claim

- That session create uses any of this: until task 24 it still sends host mounts, so under
  `PANACEA_APP_BACKEND=kata` it is still refused — correctly.
- That two Kata pods on one node can mount sub-paths of one RWO disk at the same time
  (**UNVERIFIED**, P8-D6, task 32).
