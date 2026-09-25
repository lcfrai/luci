### Task 25b: lane git on a cloud machine: phase 3's `Lanes.For` over the broker, the runner-only pod, and the bundle relay

Split out of task 25 by the env R4 rulings of 2026-09-25 (`.superpowers/sdd/env/r4-env-rulings.md`,
"What each task changes", task 25). Task 25 builds **project** git on a cloud machine. This task
builds the laptop's half of **lane** git, which never runs in the machine pod (ruling 2):
- **Step 1 of phase 3's pool, a live session.** The `Runner` for a lane with a live session is
  that session's pod's `sandboxd` `/git/v1/`, dialled through the broker's new sandbox git route
  (ruling 1): `POST /v1/sandboxes/{id}/git/v1/{op}`, `Authorization: Bearer <entra>` and
  `Panacea-Attach-Token: <token>`. The request names no directory.
- **Step 2, a runner-only pod.** Phase 3's pool asks this machine's `remotebe.Backend` for a pod
  whose one mount is the lane at `/home/dev/work` (ruling 2; phase-3 `task-24.md:678-684`). Its id
  is in phase 3's `HeldSet`, so the reaper keeps it (`task-24.md:661-668`, `:690-695`).
- **Bundles.** The cloud `lanegit.Bundles` (seam S8-P3.4, ruling 3) over `machined`'s bundle
  routes (task 21b), and the relay: lane work streams from the runner into `POST /v1/bundles`
  through an `io.Pipe`; a refresh bundle streams from `GET /v1/bundles/{id}` into `Runner.Fetch`.
  Nothing touches the laptop's disk.

No call here reads a lane's `.git`, and no lane git request reaches `machined` (ruling 1).

**Two places the rulings are silent, and the smallest thing consistent with R4.**
(Phase 3 citations say `phase-3`; bare `task-NN.md` is this plan's part.)
- **Phase 3's pool serves one backend.** `Lanes` holds one `Backend` and one `Dial`
  (phase-3 `task-24.md:621-636`), and phase 3's callers hold one `*lanegit.Lanes` (phase-3
  `task-07.md:70`, `task-09.md:92`, `task-11.md:160`, `task-16.md:177`). Step 2 calls
  `backend.WaitReady` on the sandbox's `Endpoint` (phase-3 `task-24.md:683`), and remotebe's
  `Endpoint` is `remote:<id>`, never dialled (`task-22.md:13-17`). Step 2 is `ErrNoRunner` "on
  katabe" (phase-3 `task-24.md:684`), and remotebe's `Kind()` is `KindKata`
  (`task-22.md:128-130`). `NewRunner` dials `"http://" + endpoint` (phase-3
  `task-24.md:574-579`). So this task asks phase 3 for **S8-P3.5** (below): the pool reaches a
  lane on the lane's own machine. This task does not start without it.
- **The agent proxy caps request bodies at 16 MiB** (`task-19.md:91-93`), and ruling 3 sends a
  bundle of up to `MaxBundleBytes` (1 GiB) through it to `POST /v1/bundles`. The proxy's cap on
  `POST /v1/machines/{id}/agent/v1/bundles` must be `MaxBundleBytes`. That is task 21b's
  (it serves the route), recorded as a dependency here; this task does not edit task 19's code.

**Depends on** task 25 (`gitx.Remote`, `MachineRoot`), task 21b (the bundle routes and the proxy
cap above), task 18 (the sandbox git route and a runner-only spec, ruling "What each task
changes"), task 15 (`ValidateSpec` refuses a `projects/` mount), task 23 (`machines.Set`, the
per-machine reaper), phase 3 task 24 (`lanegit`), and phase 3's **acceptance of S8-P3.4 and
S8-P3.5**. The rulings keep 25b gated until phase 3 answers (their "Stream order"); Step 1 checks.

**Files:**
- `control-plane/internal/backend/remotebe/lanegit.go`, `lanegit_test.go` — the runner-only spec
  translation, `Client.Raw`, `Client.GitTransport`, `DialLane` (new)
- `control-plane/internal/backend/remotebe/remote.go` — `Create` runs the translation first (modify)
- `control-plane/internal/machines/bundles.go`, `bundles_test.go` — `Bundles`, the cloud
  `lanegit.Bundles` (new)
- `control-plane/internal/machineapi/bundles.go` — the bundle wire, if task 21b has not already
  added it (new; task 21b owns it, and its spelling wins)
- `control-plane/cmd/api/main.go` — `laneGit.Machines` (modify, where phase 3 builds `laneGit`,
  phase-3 `task-24.md:841-850`)
- `control-plane/cmd/api/lanegit_cloud_test.go` — the pool against a fake broker (new)
