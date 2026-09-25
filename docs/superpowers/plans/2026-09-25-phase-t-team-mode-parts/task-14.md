### Task 14: OD-23 mechanism, part 2 — versioned layers, admin routes, audit, the version on agent start

**Slice:** TM1-α. **Focused time:** 2.5 hours in the skeleton; **3 hours** as written, because the
Hub audit needs a route task 01 does not have (below). **dependsOn:** 06 (the agent start carries
`policy`), 12 (`server.Route`, the verbs `policy.org` and `policy.project`), 13 (`Render`, ST.4).
**Seams:** consumes ST.4; the `policy` object on agent start (ST.2, `hubapi.PolicyRef`).

**Two kinds of version, and only one of them is what a pod mounts.** An admin writes a **layer**:
the org's, or a project's, one envelope for every class (task 13). A pod mounts a **rendered
file**: defaults, the gateway fragment, the org layer and the project layer, merged for one class.
So there are two append-only records, and they answer two questions:

| Record | Key | Answers | Where |
|---|---|---|---|
| layer version `pv-…` | `(org, project, seq)`; project `""` is the org scope | *who changed our policy, when, to what bytes* | `fleet.policy_layers`, added by this task's migration |
| rendered version | `(org, project, class, version)` | *which exact file governed this agent* | task 10's `fleet.policy_versions` (`task-10.md:345-355`), `store.PolicyVersion` |

The agent start (task 06) carries `policy: {version, digest}` (`hubapi.PolicyRef`,
`task-03.md:548-551`; `task-06.md:162`), and the `agent.started` event records `policyVersion`
(`task-06.md:240`). Here `version` is the **rendered** version for the agent's `(org, project,
class)` and `digest` is `sha256:` of its exact bytes, so *"which policy governed this agent"* is
answered from the Hub by the digest, and Fleet's rendered row (whose body *is* those bytes, and
which records the two layer ids it was rendered from) turns it back into *which admin wrote which
layer*.

**A rendered version is made when it is first needed, never fanned out.** An org change would
otherwise have to re-render every project in the org at `POST` time. Instead `Effective(org,
project, class)` renders from the current active layers at agent start (task 20 calls it), compares
the digest with the newest rendered row, and appends `version + 1` only if the bytes differ.
Rendering is deterministic (task 13), so two replicas starting two agents at once compute the same
digest; the loser of the primary-key race (`store.ErrConflict`) re-reads and finds its own bytes
already there.

**`POST` validates against every class and every project it could break.** A new org layer is
rendered for all three classes with no project layer, **and** with each project's current layer,
because a project layer that was valid under the old org layer must still be valid under the new
one; a new project layer is rendered for all three classes under the current org layer. Any
failure is 422 `policy_refused` with task 13's invariant, class and scope in `details`, and no
version is written. The same bytes as the scope's newest version give that version back (200), not
a second one: *"the same bytes twice give one digest"* (skeleton 14).

**The audit event is the admin's own Hub write, and a version is not live until the Hub has it.**
The skeleton has the event go through Fleet's outbox, but an outbox write needs a delegation
(task 01 (c): every Fleet write names one, `task-01.md:31-36`), and an admin editing policy holds
none: delegations are made at launch, per project and class. Fleet writing in its own name would
need a second exemption from the delegation rule, which task 01 reserves for a second amendment
(`task-01.md:26-29`), and would put the author in a request body, which task 01 rule 4 forbids
(`task-01.md:125-129`). So the order is:

1. the admin's client `POST`s the layer to Fleet; Fleet validates, stores it **pending**, and
   answers `{id: "pv-…", scope, projectId, seq, digest}`;
2. the same client `POST`s that metadata to the Hub **as the admin** — route 80,
   `POST /v1/orgs/{orgId}/policy-versions`, a Hub delta this task builds, whose actor the Hub takes
   from the token like every other user write;
3. the Hub writes audit event `policy.versioned` (metadata only: id, scope, project, seq, digest)
   and puts it on Fleet's feed;
4. Fleet's feed consumer (task 11's `Feed.Also`) sees it and marks the layer **active**, if the
   digest matches the pending row's.

Only active layers render. **No policy governs any agent unless the Hub holds its audit event and
its author**, which is a stronger statement than an outbox could make, and needs no exemption.
This is a deviation from the skeleton, recorded as a gap: route 80 is not in task 01's §7.0 team
table, whose last row is 79 (`task-01.md:284`). The admin's client in TM1-α is task 33's script;
**no admin view for policy is in this plan** (task 67 is quotas only), which is a second gap.

**Files:**
- `control-plane/internal/policy/store.go` — `Store`, `Put`, `List`, `Activate`, `Effective`, `LayerDigest` (new)
- `control-plane/internal/policy/store_test.go` — on task 10's `memstore` (new)
- `control-plane/internal/fleet/server/policy.go` — `RegisterPolicy`: the four Fleet routes, `ActivateOnFeed` (new)
- `control-plane/internal/fleet/server/policy_test.go` — roles, refusals, idempotence, activation (new)
- `control-plane/internal/fleet/store/store.go`, `memstore/memstore.go`, `pgstore/pgstore.go`, `storetest/suite.go` — `PolicyLayer`, `Tx.AppendPolicyLayer`, `Tx.ActivatePolicyLayer`, `Store.PolicyLayers`, `Store.CurrentPolicyLayer`, and `PolicyVersion.Layers` (modified; task 10's files)
- `control-plane/internal/fleet/store/migrations/00NN_policy_layers.sql` — numbered after the last migration at merge (tasks 19, 28 and 29 also add them, `task-10.md:571-572`) (new)
- `control-plane/cmd/fleetd/main.go` — one line, `server.RegisterPolicy(srv, pol, feed)` (modified)
- Hub delta, after `feat/hub`'s sync-merge: `control-plane/internal/hub/server/policyversions.go` and its test, one row in `hubapi/team.go` (`PolicyVersionRequest`, `PolicyVersionRecord`), the feed kind `policy.versioned` in `store/changes.go`'s Fleet list, and route 80 in the contract's §7.0 team table (new / modified; **UNVERIFIED until merge**: the Hub server package is Hub task 14's, unbuilt on 2026-09-25)

**Interfaces:**
- Produces:
  - `policy.Store` with `NewStore(s store.Store, now func() time.Time) *Store`, `(*Store).Put(ctx, org, project string, author model.Owner, doc []byte) (store.PolicyLayer, bool, error)` (`bool` = created), `List(ctx, org, project) ([]store.PolicyLayer, error)`, `Activate(ctx, org, id, digest string) error`, `Effective(ctx, org, project string, class model.Class) (store.PolicyVersion, error)`, `policy.LayerDigest(doc) (string, error)`, `policy.ErrNotPending`, `*policy.RefusedError{Project string; Err error}`
  - Fleet routes: `GET`/`POST /v1/orgs/{org}/policy` (`VerbOrgPolicy`), `GET`/`POST /v1/projects/{p}/policy` (`VerbProjectPolicy`); code `policy_refused` (422)
  - Hub route 80 and audit/feed kind `policy.versioned`
- Consumes:
  - task 13 — `policy.Render`, `policy.Layers`, `policy.Layer`, `policy.ParseLayer`, `policy.Digest`, `*policy.InvariantError`, `policy.ScopeOrg`, `policy.ScopeProject`, `policy.MaxLayerBytes`
  - task 10 — `store.Store`, `store.Tx`, `Store.Tx(ctx, fn)`, `Store.CurrentPolicy`, `Tx.PutPolicy`, `store.PolicyVersion` (`task-10.md:204-214`), `store.ErrConflict`, `store.ErrNotFound`, `memstore.New`
  - task 12 — `server.Server.Route`, `server.Handler`, `server.Principal`, `server.VerbOrgPolicy`, `server.VerbProjectPolicy`, `httpx.WriteProblem`
  - task 11 — `(*delegation.Feed).Also`, and the kind added to `main.go`'s `kinds`
  - task 06 — the Hub write shape: `server.RequireDesktop(p)` (`task-06.md:267`), the §10.6 transaction, one audit event per write
  - Hub, built — `ids.Wide` (`internal/ids/ids.go:61`), `hubapi.Me.OrgAdmin` (`hubapi/types.go:209-213`)
- Consumed by: task 20 (`Effective` at every agent start; its `Version` and `Digest` go into the agent start's `policy`, its `Body` into task 15's Secret), task 33 (the scripted policy change), task 41.
