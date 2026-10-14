# Phase 8 env — R4 re-plan rulings (2026-09-25)

Rulings the re-planners of env tasks 20, 21, 24b and 25 apply without deliberating. Each rests on a
cited line. Sources: phase 3 rulings R4 (sibling `phase-3` worktree,
`.superpowers/sdd/p3/orchestrator-rulings.md:3-67`), phase 3 `task-24.md` and `task-03.md:161-173`,
this plan's P8-D4 (`2026-09-23-phase-8-own-environment.md:134`) and P8-D7 (`:137`), and the Q18
blocks in `task-21.md:20-45` and `task-25.md:13-21`.

Note on the gates: at env `c348a88`, only task-21 (`:20`) and task-25 (`:13`) carry a
"Re-plan required (Q18)" block. Task-20 (`c98e256`) and task-24b (`281e652`) carry R4 text
(`task-20.md:64-66`, `task-24b.md:85-93`) but no gate block; task-24b says the lane choice is
"open, for the task 21 and 25 re-plan" (`task-24b.md:93`). Ruling 5 closes that.

## Ruling 1 — a lane is named by the pod that mounts it; the lane id picks the pod

- **No lane-git request goes to `machined`.** Ruling 2 moves the runner out of the machine pod.
  Lane git reaches phase 3's own runner, `sandboxd`'s `/git/v1/`, in the one pod that mounts the
  lane. The route is new on the broker and mirrors attach (`task-18.md:124`):
  `POST /v1/sandboxes/{id}/git/v1/{op}`, headers `Authorization: Bearer <entra>` and
  `Panacea-Attach-Token: <token>`. The broker proxies it to that pod's `POST /git/v1/{op}` with
  `Authorization: Bearer <token>` (sandboxd's own check, phase-3 `task-24.md:71-74`, `:410-417`).
- **Failures before the proxy are attach's:** `bad_request` 400 (no token), `sandbox_not_found`
  404 (another owner's id too, P8-D4 `:134`), `sandbox_not_running` 409, `attach_rejected` 403
  when sandboxd says 401. Every other runner answer passes through as received: status, body,
  error envelope.
- **Caps, per direction** (phase-3 `task-24.md:91-93`, the op table; `:365-370`, the limits):

  | op | request body cap | response body cap |
  |---|---|---|
  | `fetch` | `MaxBundleBytes` (the body is a bundle) | `MaxJSONBytes` |
  | `bundle` | `MaxJSONBytes` | `MaxBundleBytes` (the answer is a bundle) |
  | `status`, `refs`, `diff` | `MaxJSONBytes` | `MaxJSONBytes` |

  Both directions stream; nothing is buffered. A **request** past its cap is refused before the
  pod sees the rest: 413 in the **runner's** envelope, `{"code":"too_large"}`
  (`application/json`, phase-3 `task-24.md:194-202`), so phase 3's client decodes it as a
  `RunnerError{Status: 413, Code: "too_large"}` exactly as the runner's own 413 (`:388`), never
  as "the runner did not answer" (`:582-583`). A **response** past its cap is cut: the status is
  already sent, so the broker stops copying and aborts the stream, and phase 3's client, which
  stops at the same cap (`:568-569`), reports it. Its deadline is the op's timeout plus 5 s
  (`:366-369`, `:577-578`); past it before any header, 504 in the runner's envelope,
  `{"code":"git_timeout"}`.
- **The request names no directory**, as phase 3's does (`task-24.md:94`). The runner has one,
  `/home/dev/work`, its `-git-worktree` default (`task-24.md:85-87`, `:421-423`). The broker
  adds no lane parameter, so there is nothing to validate per call.
- **The identifier is the lane id**: `project.WorktreeID(Path)`, `w-` + 8 hex (phase-3
  `task-03.md:62`), issued by the laptop's lane store at `Create` (`task-03.md:148-168`) and kept
  in the laptop's lane index, `<dir>/lanes.json` (`task-03.md:110`). It picks the pod in phase 3's
  `Lanes.For` (`task-24.md:658-659`). Step 1 is the live session whose `WorktreeID` is the lane id
  (`:672-677`). Step 2 is a runner-only pod (ruling 2).
- **Id to directory happens once, at pod create, never per call.** The laptop's lane index is
  the authority (phase 3's `For` refuses a lane `ByID` does not answer, `task-24.md:624-625`). The
  lane's `Path`, relative to `machined`'s root, becomes the pod's one lane mount,
  `MachineMount{MachinePath: "lanes/<projectId>/<dir>", ContainerPath: "/home/dev/work"}`
  (task 15's `MachineMount`, `task-15.md:37`).
- **Refusals.**
  - A lane id that does not match `^w-[0-9a-f]{8}$` is 400.
  - A lane the index does not hold is `worktree_not_found` 404.
  - `machined`'s path check (task 20): the path must be exactly three segments `lanes/<p>/<d>`,
    with no `..`, and each segment must be an `Lstat` directory, not a symlink. Otherwise it is
    409 `conflict`, `details.reason "lane_path_invalid"`.
  - A `MachineMount` whose first segment is `projects` is refused by `backend.ValidateSpec`
    (`bad_request`, naming R4.1). ValidateSpec runs on the laptop and in the broker
    (`task-18.md:120`), so neither side can build a pod that mounts a project.

## Ruling 2 — lane git never runs in the machine pod (amends P8-D7)

**Decision.** `machined` serves the catalog and **project** git only. Lane git runs in a pod whose
only lane mount is that lane:
- the live session's pod, whose `sandboxd` already serves `/git/v1/` (phase-3
  `task-24.md:65-79`); or
- a **runner-only pod** the laptop's pool asks the broker for (phase 3's pool step 2,
  `task-24.md:678-684`, on remotebe instead of `ErrNoRunner`). It has the same image, `Machine`
  and affinity as a session, `MachineMounts` = the one lane at `/home/dev/work` and nothing else
  (no conversation store, and no `.git/config` bind: that bind is a session's hygiene only,
  as phase 3's local runner-only pod has none, `task-24.md:918`; task 24b), a fresh 32-byte
  attach token, and the smallest resources the broker admits.

P8-D7 (`plan:137`) changes from "serves the catalog and the lane git runner" to "serves the
catalog and project git". Task 21's open item 1 (vendor or reimplement, `task-21.md:30-33`)
**disappears**: the cloud runner *is* `sandboxd/internal/gitrunner`, because a session pod runs
the sandbox image (P8-D8, `plan:138`). The goldens stay the one shared artifact
(`task-24.md:29-32`).

**Why not a uid split inside the machine pod.**
- Lane git runs lane-defined filter drivers on `status` (phase 3's recorded residual,
  `task-24.md:116-118`). That is agent code running in the machine pod.
- Everything `machined` creates is uid 1001 (task 20, its opening), the agent's uid. So that code
  would read every lane, `projects/`, every conversation (task 20, "Configuration (complete)",
  the directories made at start), and `machined`'s environment, which holds
  `PANACEA_APP_MACHINED_TOKEN` (the same table), through `/proc/<pid>/environ` of the same uid.
- `machined` cannot change uid for git, because the pod drops `ALL` capabilities and forbids
  privilege escalation (`control-plane/internal/sandbox/manager.go:337-343`).
- A per-lane uid would also mean per-lane uids for sessions, which the image's `dev`/1001 home
  and phase 3's measured `safe.directory` fix (`task-24.md:97-104`) do not support.

**What the runner's git reaches, in either pod.**
- **Uid:** 1001, the agent's own (`manager.go:298-304`, `:337-343`). It can gain no capability
  and cannot escalate.
- **Environment:** the pod's `containerEnv` (`manager.go:325`). That holds the pod's own attach
  token, which the agent can already read (phase 3's trust note, `task-24.md:51-56`). It has no
  service-account token (`manager.go:290-292`), no service links (`:293-294`), and no
  `PANACEA_APP_MACHINED_TOKEN`, which is only ever in the machine pod's Secret (task 19).
- **Mounts:**
  - the one lane at `/home/dev/work`, as a subPath (`task-15.md:152`);
  - emptyDir `home` and `tmp` (`manager.go:318-330`);
  - for a session only, `conversations/<w>`.

  It gets no `projects/`, no other lane, and no disk root.
- **File modes are no line at all** (review r4-1 M2). Every process here is uid 1001, and
  `lanes/<p>` and `lanes/<p>/<d>` are made by `git clone` under git's umask (task 21's clone
  step), not by `machined` at 0700. The subPath is the boundary, and the only one claimed.

**UNVERIFIED on Kata** (task 32 must probe; nothing here ran on a cluster):
- (a) that a subPath of a block-device volume confines a container to that subPath inside the
  Kata guest. The guest may mount the whole filesystem and bind the subPath, so a guest-root
  escape would reach the whole disk.
- (b) kubelet's subPath symlink safety when the runtime is Kata.
- (c) that a runner-only pod fits beside the machine pod on its node (P8-D6's ceiling of about
  three sessions, `plan:136`). A runner-only pod counts against it and can be refused with
  `machine_full`.
- (d) Kata cold start for a runner-only pod (seconds, `CLAUDE.md`). Task 07's sweep leaves quiet
  sessionless lanes alone (`task-24.md:647-652`), so this cost falls on land, refresh and
  remove only.

## Ruling 3 — a bundle lives in `machined`'s owned root, and the laptop relays it

**Where.** The bundle lives in `/tmp/machined-git/bundles`, the `bundles` directory under
`machined`'s owned root (`task-21.md:40-42`, `:136-138`).
- The directory is mode 0700. Each file is `<16 hex>.bundle`, created `O_EXCL` 0600 (phase 3's
  own rule, `task-24.md:705-708`).
- `/tmp` is the machine pod's emptyDir, never the disk. Task 19 gives it `sizeLimit: 3Gi`: two
  live bundles at `MaxBundleBytes` (the 429 allows no third), plus clone scratch. A `POST`ed
  body is capped as it is read; a reserved `bundle create` is capped by `machined` stat'ing the
  path and its `.lock` every `BundleSizePoll` and killing git past the cap, then `413
  too_large` with both files removed (`task-21b.md`, review-2 M5). Peak: 2 GiB plus one poll's
  overshoot (write rate **UNVERIFIED**), leaving under 1 GiB for clone scratch.
- **UNVERIFIED:** whether a Kata emptyDir is guest memory or node disk. The machine pod's limit
  is 1Gi (`plan:136`). Task 32 must probe it.

**New `machined` routes (task 21b).** The broker's agent proxy reaches them, streamed and
unbuffered: the bundle `POST` through task 19's one exception to its 16 MiB request cap
(`task-19.md:96-103`). Every one takes `machined`'s bearer.
- `POST /v1/bundles`: the body is a bundle. The answer is 201 `{"id":"<16 hex>"}`.
  - An empty body with `?reserve=1` makes an id and no file, for a `bundle create`.
  - Past `MaxBundleBytes` it is 413 `too_large`. A body read past `BundleTimeout` is 504
    `git_timeout`. Either way the partial file is removed.
  - With two ids live, it is 429 `bundle_busy`; `machined` never queues. Task 25b's `Reserve`
    (refresh, which phase 3 task 09 now runs one bundle at a time) waits on it within its `ctx`;
    a `Put` (sync, land, archive) is `ErrBundleBusy`: a sync retries at its backoff, a land or
    archive fails and the user runs it again (seams review 1 I2; task 21b "The store").
- `GET /v1/bundles/{id}` streams the file. `DELETE /v1/bundles/{id}` is 204, and idempotent.
- `POST /v1/git` admits an argument under `/tmp/machined-git/` only when it is exactly
  `/tmp/machined-git/bundles/<id>.bundle` and that id is live. It is admitted only on
  `bundle verify`, `bundle create`, `bundle list-heads` and `fetch`, with `Scope` `project`.
  Anything else is 400. Phase 3's code passes the path unchanged.

**Lane to project (land, archive).**
1. The laptop's `Runner.Bundle` goes through ruling 1's broker route. It streams into its writer
   and stops at the cap (`task-24.md:568-569`).
2. That writer is an `io.Pipe` into `POST /v1/bundles`. Nothing touches the laptop's disk, and
   nothing is buffered.
3. The project's `bundle verify` and `fetch` run through `POST /v1/git`, then `DELETE`.

The bytes cross the laptop's link twice. That is acceptable for bundles the size of a lane's
work. A broker-side splice is a later optimisation and is not built now.

**Project to lane (refresh).**
1. `POST /v1/bundles?reserve=1`.
2. SendBranch's `bundle create` into that path, through `POST /v1/git`.
3. `GET /v1/bundles/{id}`, streamed as `Runner.Fetch`'s body (`task-24.md:570`) through the
   broker.
4. `DELETE`.

**Authentication, per hop.** No hop adds a credential.
- Laptop to broker: the Entra bearer plus the owner filter (P8-D4, `plan:134`).
- Broker to `sandboxd`: the pod's attach token (ruling 1).
- Broker to `machined`: the per-machine token (task 20, "Configuration (complete)"; P8-D7).

**Limits.** `MaxBundleBytes = 1 << 30` and `BundleTimeout = 120 s` (`task-24.md:365-370`) apply
at the runner, at both broker routes and at `machined`'s `POST`. The relay has one deadline,
`BundleTimeout` + 5 s (`task-24.md:577-578`). The two legs run concurrently through the pipe, so
that one deadline covers both.

**Who deletes.** `machined` owns the file.
- The laptop calls `DELETE` from a defer on every path, success included (phase 3's rule,
  `task-24.md:707-708`; SendBranch's `done`, `:742-745`).
- `machined` backstops it: a sweep every 60 s removes any bundle older than `2 × BundleTimeout`,
  and start empties the directory.

**New seam S8-P3.4 (asked of phase 3; its acceptance is UNVERIFIED).**
- Today `FetchLane`, `ArchiveLane` and `SendBranch` take `*gitx.Exec` and write to
  `<OwnedRoot>/bundles` on the host (`task-24.md:704-720`, `:742-745`).
- The ask: they take `gitx.Runner` plus a `lanegit.Bundles` with `Put(io.Reader)`, `Reserve()`,
  `Open(id)`, `Remove(id)` and `Path(id)`. The local implementation is phase 3's files. The cloud
  implementation is task 25's, over the routes above.
- Without the seam, task 25 would have to copy the split and its checks (`task-24.md:748-758`).
  That makes two copies that drift, the class R2 forbids for the hardening.

## Ruling 4 — seams S8-P3.3 (the create shape) and S8-P3.6 (`MachineFS`), restated from phase 3's current lane create

The shape comes from phase-3 `task-03.md:148-199` and `task-02.md:90-96`, `:534`. `machined`'s
`POST /v1/git` admits these calls and no others for a lane create. Each replaces the stale shape
at `task-21.md:176-197`.
- **The directory.**
  - It is `<CloneRoot>/<ProjectID>/<Name>-<6 hex nonce>` (`task-03.md:152-153`). The id is
    `WorktreeID(Path)`, and is not the directory name (`task-03.md:62`).
  - `CloneRoot()` of the cloud `gitx.Machine` is `/machine/lanes`.
  - Task 20's `DirLanes` comment (its "Contract — `machineapi/wire.go`") reads
    `lanes/<projectId>/<name>-<nonce>`.
  - The destination check at `task-21.md:189-191` still holds.
- **Step 2, step 5's `var GIT_AUTHOR_IDENT` and step 6's `config --local --get core.hooksPath`**
  run in the project, with `Scope` `project` (`task-03.md:191-192`). These are ordinary project
  calls.
- **Step 4, the clone.** `Dir` is `projects/<name>`, `Scope` `project`, `Class` `local`, and
  `NewLane` is `""` (`task-02.md:93`). The options are closed:
  - `--no-hardlinks`, `--branch <b>` and `--origin origin`;
  - `--config` for **six** keys: `core.autocrlf=false`, `core.eol=lf`, `core.checkStat=minimal`,
    `core.trustctime=false`, `core.longpaths=true` and `core.symlinks=false`, each with exactly
    that value (`task-03.md:161-164`).

  It was four keys with any value. `Env` must **equal `NoFilterEnv`**: `GIT_CONFIG_COUNT=3`,
  `filter.lfs.smudge=""`, `filter.lfs.process=""` and `filter.lfs.required=false`
  (`task-03.md:212-213`). It was "Env is empty"; the `Env` allowlist admits exactly those
  entries for this shape.
- **Steps 4a, 5 and 6: the writes in the new lane.**
  - They run with `Dir` the new lane, `Scope` `none` and `NewLane` set (`task-03.md:169-173`,
    `:175-181`, `:192-193`). The wire gains `NewLane string`.
  - `machined` admits `Dir` under `lanes/` only when `Dir == NewLane` **and** that path is in its
    in-memory creating set. A lane-create clone `machined` itself ran adds the path to the set.
  - The shapes are closed:
    - `config remote.origin.url` = `NoFetchURL`;
    - `config remote.origin.pushurl` = `NoPushURL` (`task-03.md:207-210`);
    - `config user.name <v>` and `config user.email <v>`;
    - `config core.hooksPath <v>`, with `CopyableHooksPath(v)`;
    - CopyGeneratedHooks' `ls-files`.
  - The set entry closes on the laptop's `POST /v1/lanes/sealed {path}` (sent after step 6,
    before `Store.Add`), or after 5 minutes. After it closes, no `machined` git runs there again
    (R4.5). While the entry is open, the path route of ruling 1 answers 409 `lane_creating`.
- **The filesystem half, which phase 3's `gitx.Machine` lacks.** `gitx.Machine` has only `ID`,
  `Git` and `CloneRoot` (`task-02.md:242-250`). Three create steps would touch the *laptop's*
  disk on a cloud machine:
  - step 3's `os.Lstat` (`task-03.md:153-154`);
  - the failure path's `RemoveTree` (`task-03.md:189-190`, `:268`);
  - CopyGeneratedHooks' copy (`task-03.md:227-262`, which takes paths; that it uses `os` is
    **UNVERIFIED**, read from the doc comment only).

  A separate seam, **S8-P3.6**, therefore asks phase 3 for `gitx.MachineFS`. S8-P3.3 stays what
  task 33 records: the create shape and `Env` allowlist `machined` pins from phase 3 (above). The
  local implementation is `os`. If phase 3 declines S8-P3.6, the cloud machine cannot create
  lanes. That makes it a blocker, not a fallback.

  **The one S8-P3.6 contract.** Tasks 21 (`task-21.md:192-214`) and 25 copy this verbatim,
  comments included; no other spelling exists.

  ```go
  package gitx

  import "context"

  // MachineFS is the filesystem half of lane create and remove (phase-3
  // task-03.md:153-154, :189-190, :227-268). Paths are absolute on that machine.
  // The local implementation is os, on the laptop's disk.
  type MachineFS interface {
  	// Exists is os.Lstat at path: exists is false only for not-exist; dir is true only for a
  	// real directory, never a symlink. Step 3 reads exists; Catalog.Path's worktree.Verify reads dir.
  	Exists(ctx context.Context, path string) (exists, dir bool, err error)
  	// RemoveTree is task 03's RemoveTree(root, path): it refuses a path not under root.
  	RemoveTree(ctx context.Context, root, path string) error
  	// CopyGeneratedHooks is the copy half of task 03's CopyGeneratedHooks: its Lstat
  	// guards, its SameFile check and its 64-file / 1 MiB budget. Its ls-files check and
  	// the take-back stay in lanes.CopyGeneratedHooks, over m.Git() and RemoveTree.
  	CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error)
  	// Seal ends R4.5's window for path: Create calls it after step 6, before Store.Add.
  	// Locally it does nothing; on a cloud machine it closes machined's creating-set entry.
  	Seal(ctx context.Context, path string) error
  }
  ```

  The laptop's cloud implementation is task 25's `machines.Remote.FS()`, in phase 8's
  `machines`, so phase 3's `gitx` imports nothing of phase 8's (review r4-1 M4).

  | Route (`machined`, task 21) | Body (`machineapi`, relative to the root) | Answer |
  |---|---|---|
  | `POST /v1/fs/lstat` | `FSPath{"path"}`, a lane `lanes/<p>/<d>` | `200 FSStat{"exists","dir"}` |
  | `POST /v1/fs/remove-tree` | `FSRemoveTree{"root","path"}`: `root` is `lanes` or a lane, `path` under it | `204`, also when nothing was there |
  | `POST /v1/fs/copy-hooks` | `FSCopyHooks{"projectTop","cloneTop","rel"}`: `projects/<name>`, a creating lane, a `CopyableHooksPath` | `200 FSCopied{"copied"}` |
  | `POST /v1/lanes/sealed` | `FSPath{"path"}`, a lane | `204`, idempotent |

  `dir` is there because phase 3's `Catalog.Path` runs `worktree.Verify`, which needs "a real
  directory" as well as "exists" (task 25's `MachineFS` comment).

## Ruling 5 — the primary worktree is served by its backing lane, exactly as in phase 3 task 05

**Decision.** A cloud session on a worktree of kind `primary` or `linked` runs in that
worktree's **backing lane**. This is phase 3's rule unchanged (`task-05.md:96-112`, `:290-292`).
- On the first start, create calls `lanes.Creator.Create` with:
  - the worktree's project, its checked-out branch, and `Name` = the worktree's id;
  - the cloud `gitx.Machine` (ruling 4).
- Later starts find that lane with `Store.ByProject`.
- A detached HEAD is 409 `worktree_detached`, with phase 3's sentence. No lane and no pod is
  made.
- The registry keeps the rail's `WorktreeID`, so the conversation store stays
  `conversations/<rail worktree id>`. Only the mounted path changes (`task-05.md:105-106`).
- The backing lane's directory is `lanes/<projectId>/<worktreeId>-<nonce>`.
- No pod ever mounts `projects/<name>` or a linked worktree under it (R4.1; ruling 1's
  ValidateSpec prefix rule enforces this on the laptop and in the broker).

**Why.** R4.1 says "every session, including one on what the rail calls the primary worktree,
runs in a lane clone" (phase-3 `orchestrator-rulings.md:14-17`). Phase 3 already built the rule
and its detached-HEAD answer. Reusing it keeps one rule, not a cloud twin. A backing lane made
once and refreshed by task 09 is the behaviour the owner already sees locally.

**What it changes.**
- **Task 20.** The worktree path route (task 20, "The worktree path route, in order") answers
  only paths under `lanes/`, with ruling 1's shape check. A primary or linked worktree's id is
  409 `conflict`, `details.reason "worktree_not_a_lane"`, `details.kind`. The `DirLanes` comment
  reads `lanes/<projectId>/<name>-<nonce>`.
- **Task 24b.** Step 1 (`task-24b.md:69-72`) runs phase 3 task 05's backing-lane resolution
  before it asks for the path, and then asks for the *lane's* path. The open sentence at
  `task-24b.md:91-93` is replaced by this ruling.
  - 24b does this against the `gitx.Machine` that phase 3's machine resolver answers (its task
    04 wiring). Its tests use phase 3's fakes. Task 25 adds the real cloud arm, so 24b needs no
    stub and no cycle with 25.

## What each task changes

The hours are this ruling's estimates; the plan's current figures are `plan:593-604`.

**Task 20** (2.5 h to 3 h)
- The path route is lanes-only, with ruling 1's shape check and ruling 5's
  `worktree_not_a_lane`. It adds tests for a symlinked segment, `..`, a `projects/` id, and a
  primary id.
- The `DirLanes` comment becomes `lanes/<projectId>/<name>-<nonce>` (ruling 4).
- Line 12's "a compromise of it is contained" gains: no agent code runs in the machine pod,
  because lane git is not here (ruling 2).
- ~~Add a Q18 gate block (it has none at `c348a88`) until this ruling is applied.~~ Applied and
  withdrawn (review-2 M4): the ruling is in the part, so no gate block is added.

**Task 21** (3.5 h, split into 21 at 3.5 h plus a new **21b** at 2 h)
- Delete the lane runner and its open item 1. `machined` serves no `/git/v1/` (ruling 2).
  P8-D7 (`plan:137`) and the task row (`plan:594`) are restated to match.
- The lane-create shape follows ruling 4: six fixed keys, `Env == NoFilterEnv`, `NewLane` on
  the wire, the creating set, and `POST /v1/lanes/sealed`.
- Add the `MachineFS` routes (`Exists`, `RemoveTree`, `CopyGeneratedHooks`), confined to
  `lanes/`.
- **21b:** the bundle routes of ruling 3 (`POST`, `?reserve=1`, `GET`, `DELETE`), the argument
  admission, the 429, and the sweep.
- **New dependencies:** phase 3 seams S8-P3.6 (`MachineFS`, ruling 4) and S8-P3.4 (`Bundles`).
  S8-P3.3 stays task 33's record of the `Env` allowlist and `cloneArgv` pin. Task 19 owns the
  `/tmp` emptyDir `sizeLimit: 3Gi` and the proxy's streamed bundle `POST` (`task-19.md:26`,
  `:96-103`, `:225-231`); 21b's Step 1 checks both.

**Task 24b** (3 h to 3.5 h)
- Cloud create on a primary or linked worktree resolves its backing lane first (ruling 5).
- Its mounts are the lane path, then the optional `.git/config` hygiene bind, then
  `conversations/<rail id>`.
- New tests:
  - `TestCloudCreateOnPrimaryMountsItsBackingLane`;
  - `TestCloudCreateNeverMountsProjects`, which fails if any `MachineMount` starts `projects/`;
  - `TestCloudCreateDetachedIs409`.
- **New dependency:** phase 3 task 05 (its backing-lane resolution).
- ~~Add a Q18 gate block until this ruling is applied.~~ Applied and withdrawn (review-2 M4):
  the ruling is in the part; 24b's hold is its HELD line (plan P3A-6), not a gate block.

**Task 25** (2.5 h, split into 25 at 2.5 h plus a new **25b** at 2.5 h)
- **25:** `machines.Remote` (phase 8's package, not `gitx`), project-only; its `CloneRoot` is
  `/machine/lanes`, from task 20's `machineapi.MachineRoot`. It also covers the cloud
  `MachineFS` and the laptop's switch to phase 3's composite catalog. That catalog lists lanes
  from the lane index, never by running git in `lanes/` (R4.2); `machined` keeps phase 2's
  Scanner.
- **25b:** remotebe's `Lanes.For`.
  - Step 1 is the live session's pod, dialled through ruling 1's broker route.
  - Step 2 is a runner-only pod (ruling 2). Its id is in `HeldSet`, so the reaper keeps it
    (phase-3 `task-24.md:661-668`, `:690-695`).
  - It also covers the cloud `Bundles` implementation and the laptop relay (ruling 3).
- Retire `FetchURL` as the clone's path, as the Q18 block already says.
- **New dependencies:** task 18 gains `POST /v1/sandboxes/{id}/git/v1/{op}` and admits a
  runner-only spec (+0.5 h; applied, `task-18.md:127-160`, tests `:392`, `:398`). Task 15's
  ValidateSpec gains the `projects/` refusal (+0.25 h; applied, `task-15.md:57`, test `:148`). Task 23's
  per-machine reaper honours phase 3's `Held` (applied). Task 32 gains ruling 2's probes (a) to
  (d) and the emptyDir medium.
- **25b's runner-only create** asks `machined`'s worktree path route before the pod is made
  (`LaneCheck`, task 25b), so ruling 1's `Lstat` walk and `lane_creating` hold for it too.

**Stream order** (the plan's "Order", "Held"; P3A-5). 21 and 25 need S8-P3.6; 21b needs 21;
25b needs S8-P3.4 and S8-P3.5. All are phase 3 asks, filed by the orchestrator or the owner.
Until phase 3 answers, 21, 21b, 25 and 25b are out of the build stream, so no implementer is
dispatched onto a known block. 21b sits after 21 and 19; 25b after 25, 23, 21b and 18. Task 20
and task 24b's backing-lane step can land first.

