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

It also states **the one-node ceiling** (P8-D6, review I5): a cloud machine's sessions all run on
the node holding its machine pod, so the unit of cost is not "a session" but "a node that can
hold a machine's sessions". On `Standard_D8s_v5` that is about three default sessions per machine
(arithmetic, until P07 measures it), which leaves the per-user cap of 8 over 2 machines
unreachable. The table therefore has one row per candidate sandbox SKU (`Standard_D8s_v5`,
`Standard_D16s_v5`, `Standard_D32s_v5`), each with its sessions-per-machine column from P07 and
the survey's per-node anchor, plus a row for a smaller default session on `D8s_v5`; the SKU P07
selects (task 32) is marked, and its warm floor (`min_count`) is costed at that SKU's price, not
`D8s_v5`'s.

#### Questions this phase leaves with its siblings (recorded in `phase-8-results.md`)

| To | Question | This phase assumed |
|---|---|---|
| phase 3 | P3A-1: join machine paths with `path.Join` (or add `Machine.Join`) | `Machine.Join` added (task 25) |
| phase 3 | P3A-2: the lane index keyed by machine | laptop-side, keyed by `(machineId, projectId)`; single-laptop limitation stated |
| phase 3 | P3A-3: answered by phase 3's task 02 §HGH (H1–H5), which hardens every `gitx.Exec` call; `machined` runs it unchanged (task 21, review I7). What stays open: a git code path §HGH does not list is a finding for both phases, because on the machine it runs with the whole disk in reach | §HGH lands before task 21 |
| Hub | Display names carry an environment suffix outside prod; the broker API is assignment-required (task 09) | as stated |
| Hub | `entra` adds `ErrKeySetUnavailable`, `Middleware`, `Ready`, `Claims.Key` beside the contract's interface (task 16) | additive; the Hub may ignore them |
| Hub | **HC-1** (review I9): `roles.sql` (contract §10.1, `:1421-1430`) begins `CREATE ROLE hub_owner NOLOGIN;`, which errors on a second run. Wrap both role creations in `DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '…') THEN CREATE ROLE … NOLOGIN; END IF; … END $$;`, so a re-run after a restore or a half-finished bootstrap is safe. `GRANT CREATE ON DATABASE hub TO hub_owner` is already safe to repeat. Also state, in the file's header comment, that it runs as `psql -v ON_ERROR_STOP=1 --single-transaction -f roles.sql` (plan re-review b03 I-1: psql's default carries on after an error and exits 0) | not answered: task 31 §7 runs `roles.sql` in one transaction with `ON_ERROR_STOP` only when neither role exists; when both do, it re-runs `GRANT CREATE ON DATABASE hub TO hub_owner` alone and checks `has_database_privilege('hub_owner','hub','CREATE')`; it stops when exactly one exists. That is correct for both the current file and the amended one, and a first run that fails at the `GRANT` leaves no role behind (plan re-review b03 M-4) |
| Hub | **HC-2** (review I9): `hub migrate`, before anything else, runs two queries in order: `SELECT 1 FROM pg_roles WHERE rolname = 'hub_owner'`, then, only if a row came back, `pg_has_role(current_user, 'hub_owner', 'MEMBER')`. Two queries, not one `AND`, because `pg_has_role` **raises** on a role that does not exist rather than returning false, and Postgres does not promise to evaluate an `AND` left to right (plan re-review b03 M-7). No row, `false`, or a refused login → exit non-zero with one line naming the missing database bootstrap (`roles.sql` and the principals) and `current_user` | not answered: the Hub's first install still fails closed without it (task 30, `hub` last), but its log shows only the server's error |
| Hub | Deployed principals are bound by **object id** (`pgaadauth_create_principal_with_oid`, task 29), while the contract's commented grants (`:1424-1426`) name `"id-hub-migrate"` and `"id-hub"` literally. The role name is the identity's name as the `workloads` outputs give it, which carries the environment (task 10) | the grants are rendered from the outputs, never the literal names |

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
