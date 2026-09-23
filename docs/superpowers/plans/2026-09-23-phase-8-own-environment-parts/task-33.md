### Task 33: Close the phase — the seams, the compliance gap, the cost model, what stays unverified

Phase 2's task 26 learned that a seam written only in a plan is rediscovered by the next phase
(`phase-2-parts/task-26.md:1-9`), and that a document nothing checks drifts. This task writes phase
8's end state where the next phase will look, and pins it where it can be pinned.

**Files:**
- `docs/SEAMS.md` and `desktop/src/seams.test.ts` — if phase 2 task 26 has landed: the S8 rows updated, the new rows added, each with an anchor (modify)
- `docs/PROGRAM.md` — phase 8's block: what now exists; the Defender-for-Kata compliance gap; open risk 1's status (modify)
- `docs/superpowers/planning/env/cost-model.md` — the cost model with task 32's P07 slot (new)
- `docs/verification/phase-8-results.md` — what was built, what was proven where, what is `NOT RUN` (new)

#### The seam rows (for `docs/SEAMS.md`; ids are the plan's)

| ID | Where | The next phase does | Must not | Anchor |
|---|---|---|---|---|
| S8.1 | `backend.Backend` — now three implementations (`dockerbe`, `katabe`, `remotebe`) | add a fourth behind the same seven methods | widen the interface for one backend — use an optional interface, as `backend.ReadyWaiter` does | the phase 2 reflection test, unchanged |
| S8.3 | cloud create sends `MachineMounts` (`api/sessions.go`, task 24); `katabe` still refuses host `Mounts` | — (closed) | drop the host-mount refusal | `TestCreateRejectsMountsAndCreatesNoPod`, `TestCloudCreateSendsMachineMountsNeverHostMounts` |
| S8.4 | `PANACEA_APP_RUNTIME_CLASS` | set it from task 32's P01 | reintroduce a constant | seams manifest anchor on `sandbox.DefaultRuntimeClass` |
| S8.6 | the desktop hop carries the Hub's local caller key; the Entra token never enters the webview | — | send a user token to the renderer | `client.ts` anchor |
| S8-P6.1 | `~/.claude` outside `projects/` is an `emptyDir` in every session pod | phase 6 supplies **the user's own** credentials to that user's pods | put credentials on the machine disk; pool them; bake them into an image | anchor on the `home` emptyDir in `manager.go` |
| S8-P3.2 | adding a project to a cloud machine = `git clone` through `machined`'s `/v1/git` (task 21) | the phase that owns "add project" builds the surface | add a clone-specific route | anchor on the route table |
| S8-P9.1 | CMK, etcd KMS, Defender, Sentinel, WAF absent by design | phase 9 adds each as a module variable | a default-false posture flag on a create-time field | anchor: `modules/aks` has no `key_management_service` |
| S8-D.1 | a first machine provisions a disk on demand | a pool of pre-provisioned unclaimed disks, if P18's first-sign-in time asks for it | pre-create disks per user | — (a gap, not code) |
| S8-C.1 | every stack is parameterised by environment, location, tenant and subscription | the customer-hosted tier runs the same stacks | hard-code our tenant, zone or registry | `guards.sh` G2 |
| S8-H.1 | `deploy/charts/hub` sets only the Hub contract's variables | the Hub phase reconciles when its config grows | set an undocumented variable | the hub chart's env test |

If phase 2 task 26 has **not** landed (no `docs/SEAMS.md`), the rows go into
`docs/verification/phase-8-results.md` under "Seams", and task 26's closeout moves them.

#### `PROGRAM.md` amendments

- Phase 8's block: the five moving parts (plan "Architecture"), each as built or not.
- Cross-cutting constraints: **"Defender for Containers cannot assess Kata pods — document as a
  compliance gap"** (`PROGRAM.md:133`) becomes a named gap with its compensating controls: Pod
  Security `restricted` on `panacea-sandboxes`, image scanning in CI (task 27), kube-audit-admin
  (task 06), and no Azure identity or API token in any sandbox (tasks 10, 14).
- Open risk 1 (Kata cold start and warm-pool sizing): **still open** until task 32's P06 and P07
  have results; the text says which rows close it.

#### `cost-model.md`

The survey's anchors (§6, `:480-515`), which are Panacea's in-code estimates, **not fetched
prices**; the levers in order (warm floor, idle reaper, machine idle sleep, session size, SKU, off-
hours scale-down); and one table with an empty column for task 32's P07 result (sessions per node)
and P16 (log ingest per day). No number in it is presented as measured until those are filled.

#### Questions this phase leaves with its siblings (recorded in `phase-8-results.md`)

| To | Question | This phase assumed |
|---|---|---|
| phase 3 | P3A-1: join machine paths with `path.Join` (or add `Machine.Join`) | `Machine.Join` added (task 25) |
| phase 3 | P3A-2: the lane index keyed by machine | laptop-side, keyed by `(machineId, projectId)`; single-laptop limitation stated |
| phase 3 | P3A-3: does `gitx.Local` disable repository hooks and `core.fsmonitor` on the laptop, as `machined` does on the machine (task 21)? An agent writes the lane clone's `.git` | not assumed; asked |
| Hub | Display names carry an environment suffix outside prod; the broker API is assignment-required (task 09) | as stated |
| Hub | `entra` adds `ErrKeySetUnavailable`, `Middleware`, `Ready`, `Claims.Key` beside the contract's interface (task 16) | additive; the Hub may ignore them |

#### Steps

- [ ] **Step 1:** Check whether `docs/SEAMS.md` exists; update it and the manifest, or write the
  rows into the results document.
- [ ] **Step 2:** `PROGRAM.md`; `cost-model.md`; `phase-8-results.md` with, per task, what was
  proven where (Go test, `terraform test` against mocks, helm-unittest, script against canned
  output) and every owner-gated row `NOT RUN`.
- [ ] **Step 3:** Run the whole suite one last time; commit.

#### Acceptance

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1        # ok
cd desktop && npm test                                           # ok (seams manifest in sync, if SEAMS.md exists)
bash infra/scripts/guards.sh && bash infra/scripts/role-allowlist.sh   # both pass
grep -c "NOT RUN" docs/verification/phase-8-results.md           # every owner-gated row
```

#### What this task does not claim

That the environment exists, works, or costs what the model says. Those are tasks 31 and 32's,
run when the owner provides it.
