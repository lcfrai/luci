### Task 48: Phase 3 delta: the read-only per-project fetch credential (OD-22)

**Slice TM1. Depends on: 17, P3-12, P3-13, P3-14. About 3 hours. Seam: none new.**

OD-22 answers the design's Q11 (`05-design.md:1333`): the cloud **may fetch** a project's
repository while nobody is signed in, **read-only, per project**; every push still uses a
signed-in person's own credential (`2026-09-23-owner-decisions.md:150`). Three things need it: the
slow-timer fetch that keeps lanes fresh (phase 3 task 12, `task-12.md:16-17`), a typed `rebase`
directive (task 31), and the Orchestrator's view of `origin` (tasks 22, 25). Phase 3 today fetches
**on the laptop**, with the machine's own git credential helper (`task-12.md:4-10`), which is
exactly the case where nobody is there. This task is a delta on `feat/phase-3-git` (worktree
`.claude/worktrees/phase-3`, read at `9935278`) and on phase 8's broker; **none of the files it
extends is built** on either branch (phase 3 has no `remote`, `credstore` or `githost` code;
phase 8 has no broker), so every path is **UNVERIFIED until merge** and cites its plan row.

**The one thing this task must not get wrong is that a read credential can never write.** The
token a host issues is read-only (`contents: read`; an Azure DevOps identity with Read on one
repository), so the host would refuse a push anyway. That is the second wall, not the first. The
first is in our code: a fetch credential is a distinct type of `remote.Auth` and
`credstore.Credential`, and **`remote.Push` and `githost.OpenPullRequest` refuse it before any
process starts or any request is sent**. A test counts zero git calls and zero HTTP requests.

**The second thing is that the project credential never outlives a person's own access by more
than one sign-in.** Fetches go into *her* clones on *her* machine disk. So the broker issues the
project credential for her machine only while it holds a fresh **proof**: at each sign-in, her
own desktop checks her own repository access with her own token (phase 3 task 13's chain) and
reports the result. A failed or missing proof stops every fetch into her clones, the timer's
included (`05-design.md:1059-1062`).

**Where it runs.** The broker holds the credential's source material and mints a token per fetch;
the fetch itself runs in `machined` through phase 8 task 21's `POST /v1/git` with the header in
`Env`, host-scoped exactly as phase 3's `PushEnv` scopes a push's (`task-13.md:168-175`; phase 8
`task-21.md:168-197`). So in the cloud the phase 3 `remote.Fetcher` runs **in the broker**, over
the remote `gitx.Machine` of each machine, and `machined` never stores a token. The lane mirror
(task 59) is the only other caller. **No credential enters a session sandbox** (global rule 8).

**Files:**
- `control-plane/internal/credstore/credstore.go` (phase 3 task 13, planned): `SourceProjectFetch`
  (modify)
- `control-plane/internal/githost/fetchcred.go` + test: `Binding`, `FetchSource`, `GitHubApp`,
  `AzureDevOpsIdentity`, `FetchEnv` (new, **UNVERIFIED until merge**: the package is phase 3's)
- `control-plane/internal/githost/prove.go` + test: `ProveAccess` (new)
- `control-plane/internal/githost/githost.go` (phase 3 task 14): `OpenPullRequest`'s
  implementations refuse a project-fetch credential (modify)
- `control-plane/internal/remote/fetch.go`, `push.go` (phase 3 task 12): `Auth.readOnly`,
  `FetchOnly`, `Fetcher.Authorize`, `Push`'s refusal (modify)
- `control-plane/cmd/broker/fetch.go` + test: the proof store, the per-machine fetch loop, the two
  routes, the audit line (new, **UNVERIFIED until merge**: phase 8 tasks 17–19)
- `control-plane/internal/api/signin_proof.go` + test: after sign-in, prove and report (new; the
  local control plane's sign-in hook is the Hub's, Hub task 33 **UNVERIFIED until merge**)
- `infra/terraform/modules/keyvault/*`, `infra/terraform/stacks/workloads/*` (phase 8 tasks 05, 10,
  planned): the GitHub App key secret, readable by the broker's identity only; one user-assigned
  identity per Azure DevOps team project, federated to the broker's service account (modify)
- `deploy/charts/broker/templates/*` (phase 8 task 29, planned): the key's CSI mount, the bindings
  file (modify)
- `docs/runbooks/fetch-credential.md`: installing the GitHub App; granting an Azure DevOps
  identity Read on one repository (new)
