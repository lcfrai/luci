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
- `control-plane/internal/fleet/store/store.go`, `memstore/memstore.go`, `pgstore/pgstore.go`, `storetest/suite.go` — `PolicyLayer`, `Tx.AppendPolicyLayer`, `Tx.ActivatePolicyLayer`, `Store.PolicyLayers`, `Store.CurrentPolicyLayer`, `Store.PolicyProjects`, and `PolicyVersion.Layers` (modified; task 10's files)
- `control-plane/internal/fleet/store/migrations/00NN_policy_layers.sql` — numbered after the last migration at merge (tasks 19, 28 and 29 also add them, `task-10.md:571-572`) (new)
- `control-plane/cmd/fleetd/main.go` — one line, `server.RegisterPolicy(srv, pol, feed)` (modified)
- Hub delta, after `feat/hub`'s sync-merge: `control-plane/internal/hub/server/policyversions.go` and its test, one row in `hubapi/team.go` (`PolicyVersionRequest`, `PolicyVersionRecord`), the feed kind `policy.versioned` in `store/changes.go`'s Fleet list, and route 80 in the contract's §7.0 team table (new / modified; **UNVERIFIED until merge**: the Hub server package is Hub task 14's, unbuilt on 2026-09-25)

**Interfaces:**
- Produces:
  - `policy.Store` with `NewStore(s store.Store, now func() time.Time) *Store`, `(*Store).Put(ctx, org, project string, author model.Owner, doc []byte) (store.PolicyLayer, bool, error)` (`bool` = created), `List(ctx, org, project) ([]store.PolicyLayer, error)`, `Activate(ctx, org, project, id, digest string) error`, `Effective(ctx, org, project string, class model.Class) (store.PolicyVersion, error)`, `policy.LayerDigest(doc) (string, error)`, `policy.ErrNotPending`, `*policy.RefusedError{Project string; Err error}`
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

**Additions to task 10's `store` package, complete.** In `store.go`, beside `PolicyVersion`:

```go
// PolicyLayer is one append-only version of one scope's OD-23 layer (task 13's
// envelope). It is pending until the Hub's policy.versioned event names it.
type PolicyLayer struct {
	ID          string      // pv-…, ids.Wide("pv")
	Org         string
	Project     string      // "" is the org scope
	Seq         int         // 1, 2, 3 … per (Org, Project)
	Doc         []byte      // the envelope exactly as posted
	Digest      string      // policy.LayerDigest(Doc): sha256 of its canonical form
	Author      model.Owner // from the principal, never the body
	CreatedAt   time.Time
	ActivatedAt time.Time // zero while pending
}
```

`PolicyVersion` gains one field, so a rendered row names what it was rendered from:

```go
	Layers []string // the pv-… ids merged into Body, org first; empty for defaults only
```

`Store` gains two reads and `Tx` two writes:

```go
	// PolicyLayers is every version of one scope, newest first.
	PolicyLayers(ctx context.Context, org, project string) ([]PolicyLayer, error)
	// CurrentPolicyLayer is the newest ACTIVE version of one scope.
	CurrentPolicyLayer(ctx context.Context, org, project string) (PolicyLayer, bool, error)
	// PolicyProjects is every project of org with at least one active layer.
	PolicyProjects(ctx context.Context, org string) ([]string, error)
```

```go
	// AppendPolicyLayer sets l.Seq to the scope's max + 1 and inserts; the
	// (org, project, seq) key makes a concurrent append ErrConflict.
	AppendPolicyLayer(ctx context.Context, l PolicyLayer) (PolicyLayer, error)
	// ActivatePolicyLayer sets ActivatedAt once; a second call is a no-op, and
	// an unknown id is ErrNotFound.
	ActivatePolicyLayer(ctx context.Context, org, id string, at time.Time) error
```

`storetest` gains three rows, each with the mutant it kills: `PolicyLayersAppendOnly` (two appends
get seq 1 and 2; there is no update method, and the `pgstore` run proves the grant refuses an
`UPDATE` of `doc`: a layer rewritten in place), `CurrentLayerIsNewestActive` (seq 2 pending, seq 1
active → current is 1; activate 2 → current is 2: a pending layer rendered into a pod), and
`ActivateIsOnce` (the second activation keeps the first `ActivatedAt`: an audit time that moves).

`control-plane/internal/fleet/store/migrations/00NN_policy_layers.sql`, complete:

```sql
-- OD-23 layer versions (Phase T task 14). Append-only: the one permitted
-- change is setting activated_at once, when the Hub has audited the version.
CREATE TABLE fleet.policy_layers (
  id            text        PRIMARY KEY CHECK (id ~ '^pv-[0-9a-f]{16}$'),
  org_id        text        NOT NULL,
  project_id    text        NOT NULL DEFAULT '',        -- '' is the org scope
  seq           integer     NOT NULL CHECK (seq >= 1),
  doc           bytea       NOT NULL CHECK (octet_length(doc) <= 65536),
  digest        text        NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
  author        text        NOT NULL,
  created_at    timestamptz NOT NULL,
  activated_at  timestamptz,
  UNIQUE (org_id, project_id, seq)
);
CREATE INDEX policy_layers_current ON fleet.policy_layers (org_id, project_id, seq DESC)
  WHERE activated_at IS NOT NULL;

ALTER TABLE fleet.policy_versions ADD COLUMN layers text[] NOT NULL DEFAULT '{}';

-- Only activated_at may change, and only from NULL.
CREATE FUNCTION fleet.policy_layers_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'policy_layers is append-only';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.seq, NEW.doc, NEW.digest, NEW.author, NEW.created_at)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.project_id, OLD.seq, OLD.doc, OLD.digest, OLD.author, OLD.created_at)
     OR OLD.activated_at IS NOT NULL THEN
    RAISE EXCEPTION 'policy_layers: only activated_at may be set, once';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER policy_layers_guard BEFORE UPDATE OR DELETE ON fleet.policy_layers
  FOR EACH ROW EXECUTE FUNCTION fleet.policy_layers_guard();
REVOKE DELETE ON fleet.policy_layers FROM PUBLIC;
```

A trigger and not only a grant, because task 10 leaves open whether Fleet's runtime role is the
migration owner (`task-10.md:377`); an owner is not stopped by a `REVOKE`, and a trigger stops
everyone but a superuser.

**`control-plane/internal/policy/store.go`, complete (two blocks, one file):**

```go
package policy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/store"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

var (
	ErrNotPending     = errors.New("policy: no pending version has that id")
	ErrDigestMismatch = errors.New("policy: the Hub audited a different digest than Fleet holds")
)

// RefusedError is a layer Render refused, with the project it was rendered
// against ("" for the org scope alone). Err is ErrLayer or *InvariantError.
type RefusedError struct {
	Project string
	Err     error
}

func (e *RefusedError) Error() string {
	if e.Project == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + " (rendered with project " + e.Project + ")"
}
func (e *RefusedError) Unwrap() error { return e.Err }

var allClasses = []model.Class{model.ClassWorker, model.ClassOrchestrator, model.ClassMirror}

type Store struct {
	s   store.Store
	now func() time.Time
}

func NewStore(s store.Store, now func() time.Time) *Store { return &Store{s: s, now: now} }

// LayerDigest is Digest of the layer's canonical form, so two posts that
// differ only in key order or whitespace are one version.
func LayerDigest(doc []byte) (string, error) {
	v, err := decode(doc)
	if err != nil {
		return "", err
	}
	b, err := canonical(v)
	if err != nil {
		return "", err
	}
	return Digest(b), nil
}

// Put validates doc, then appends it pending. A doc identical (canonically)
// to the scope's newest version returns that version and created = false.
func (s *Store) Put(ctx context.Context, org, project string, author model.Owner, doc []byte) (store.PolicyLayer, bool, error) {
	if _, err := ParseLayer(doc); err != nil {
		return store.PolicyLayer{}, false, &RefusedError{Project: project, Err: err}
	}
	digest, err := LayerDigest(doc)
	if err != nil {
		return store.PolicyLayer{}, false, err
	}
	versions, err := s.s.PolicyLayers(ctx, org, project)
	if err != nil {
		return store.PolicyLayer{}, false, err
	}
	if len(versions) > 0 && versions[0].Digest == digest {
		return versions[0], false, nil
	}
	if err := s.validate(ctx, org, project, doc); err != nil {
		return store.PolicyLayer{}, false, err
	}
	var out store.PolicyLayer
	err = s.s.Tx(ctx, func(tx store.Tx) error {
		var err error
		out, err = tx.AppendPolicyLayer(ctx, store.PolicyLayer{ID: ids.Wide("pv"), Org: org,
			Project: project, Doc: doc, Digest: digest, Author: author, CreatedAt: s.now()})
		return err
	})
	return out, err == nil, err
}

func (s *Store) List(ctx context.Context, org, project string) ([]store.PolicyLayer, error) {
	return s.s.PolicyLayers(ctx, org, project)
}
```

```go
// Activate is the feed consumer's: the Hub has audited (org, project, id)
// with digest. It re-validates first, because the other scope may have
// changed since the Put: a layer valid then may not be valid now.
func (s *Store) Activate(ctx context.Context, org, project, id, digest string) error {
	versions, err := s.s.PolicyLayers(ctx, org, project)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.ID != id {
			continue
		}
		if v.Digest != digest {
			return fmt.Errorf("%w: %s holds %s, the Hub audited %s", ErrDigestMismatch, id, v.Digest, digest)
		}
		if !v.ActivatedAt.IsZero() {
			return nil // a replayed event
		}
		if err := s.validate(ctx, org, project, v.Doc); err != nil {
			return err // stays pending; the refusal is logged and the admin posts again
		}
		return s.s.Tx(ctx, func(tx store.Tx) error { return tx.ActivatePolicyLayer(ctx, org, id, s.now()) })
	}
	return ErrNotPending
}

// validate renders every class against every pairing doc could break.
func (s *Store) validate(ctx context.Context, org, project string, doc []byte) error {
	type pair struct {
		project  string
		org, prj Layer
	}
	var pairs []pair
	if project != "" {
		o, _, err := s.current(ctx, org, "")
		if err != nil {
			return err
		}
		pairs = append(pairs, pair{project, o, Layer{Scope: ScopeProject, Doc: doc}})
	} else {
		o := Layer{Scope: ScopeOrg, Doc: doc}
		pairs = append(pairs, pair{"", o, Layer{}})
		projects, err := s.s.PolicyProjects(ctx, org)
		if err != nil {
			return err
		}
		for _, p := range projects {
			pl, _, err := s.current(ctx, org, p)
			if err != nil {
				return err
			}
			pairs = append(pairs, pair{p, o, pl})
		}
	}
	for _, p := range pairs {
		for _, c := range allClasses {
			if _, _, err := Render(Layers{Org: p.org, Project: p.prj}, c); err != nil {
				return &RefusedError{Project: p.project, Err: err}
			}
		}
	}
	return nil
}

// current is a scope's active layer as Render's input, and its row.
func (s *Store) current(ctx context.Context, org, project string) (Layer, store.PolicyLayer, error) {
	row, ok, err := s.s.CurrentPolicyLayer(ctx, org, project)
	if err != nil || !ok {
		return Layer{}, store.PolicyLayer{}, err
	}
	scope := ScopeOrg
	if project != "" {
		scope = ScopeProject
	}
	return Layer{Scope: scope, Version: row.ID, Doc: row.Doc}, row, nil
}

// Effective is the file an agent of class starts with in project, as a
// rendered version: the newest row if its bytes are still right, else a new
// row. Task 20 calls it at every start. Two replicas racing agree on bytes.
func (s *Store) Effective(ctx context.Context, org, project string, class model.Class) (store.PolicyVersion, error) {
	o, orow, err := s.current(ctx, org, "")
	if err != nil {
		return store.PolicyVersion{}, err
	}
	p, prow, err := s.current(ctx, org, project)
	if err != nil {
		return store.PolicyVersion{}, err
	}
	file, digest, err := Render(Layers{Org: o, Project: p}, class)
	if err != nil { // both were valid when activated; this is a defect, and no agent starts on it
		return store.PolicyVersion{}, fmt.Errorf("policy: the active layers no longer render for %s/%s: %w", project, class, err)
	}
	var layers []string
	author := orow.Author
	for _, r := range []store.PolicyLayer{orow, prow} {
		if r.ID != "" {
			layers = append(layers, r.ID)
		}
	}
	if prow.ID != "" && prow.CreatedAt.After(orow.CreatedAt) {
		author = prow.Author
	}
	for attempt := 0; attempt < 3; attempt++ {
		cur, ok, err := s.s.CurrentPolicy(ctx, org, project, class)
		if err != nil {
			return store.PolicyVersion{}, err
		}
		if ok && cur.Digest == digest {
			return cur, nil
		}
		next := store.PolicyVersion{Org: org, Project: project, Class: class, Version: cur.Version + 1,
			Digest: digest, Body: file, CreatedBy: author, CreatedAt: s.now(), Layers: layers}
		err = s.s.Tx(ctx, func(tx store.Tx) error { return tx.PutPolicy(ctx, next) })
		if err == nil {
			return next, nil
		}
		if !errors.Is(err, store.ErrConflict) {
			return store.PolicyVersion{}, err
		}
	}
	return store.PolicyVersion{}, fmt.Errorf("policy: rendered versions for %s/%s did not settle", project, class)
}
```

**A version 1 exists before any admin writes a layer.** With no active layer at all, `Effective`
renders the defaults and the gateway fragment and stores that as version 1 with `Layers` empty and
`CreatedBy` `""`, so every agent start carries a real `{version, digest}` and the Hub never sees a
zero. `""` is allowed by task 10's `created_by text NOT NULL`; it means *the product's minimal
default*, and the GET route shows it that way.
