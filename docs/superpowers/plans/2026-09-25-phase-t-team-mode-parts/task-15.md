### Task 15: Phase 8 delta — sandbox classes (task 14b), the policy mount, `sandboxd`'s digest check

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** 13 (the rendered file and its digest,
ST.4), P8-14 (**built**: `control-plane/internal/sandbox/manager.go` on `feat/env`, `09ae70a`),
P8-15 (**planned**: machine volumes, `phase-8 …/task-15.md`). Built here after `feat/env`'s
sync-merge into `feat/team-mode`. **Seams:** consumes ST.4; produces `Spec.Class` and
`Spec.PolicyFile`, which task 20 fills (`task-20.md:252-282`).

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
already writes against that name (`task-20.md:275-279`). Phase 8's `Create` treats the attach
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
(`task-20.md:258-265`); they would be refused, and that is recorded as a gap: task 20 sets the
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
- `sandboxd/cmd/sandboxd/main.go` — the one call before `newServer`; `main_test.go` gains the tampered-file process test (modified)
- `images/sandbox/Dockerfile` — nothing for the policy (the mount brings the directory); **one** line only if phase 6's image does not already create `/etc/claude-code`, which a directory mount on a read-only rootfs needs as its mount point (checked in step 5) (modified, conditionally)
