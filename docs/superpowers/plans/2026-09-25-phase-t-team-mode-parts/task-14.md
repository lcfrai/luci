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
`task-03.md:555-558`; `task-06.md:189`), and the `agent.started` event records `policyVersion`
(`task-06.md:323`). Here `version` is the **rendered** version for the agent's `(org, project,
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
(`task-01.md:130-134`). So the order is:

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
table, whose last row is 79 (`task-01.md:290`). The admin's client in TM1-α is task 33's script;
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
  - task 06 — the Hub write shape: `server.RequireDesktop(p)` (`task-06.md:355`), the §10.6 transaction, one audit event per write
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

#### Fleet's four routes — complete wire

`{org}` is the caller's org (task 12's `authorize` refuses another); `{p}` is a `pj-…` the caller
is team *admin* of. The body of a `POST` is task 13's envelope itself, at most 64 KiB.

```jsonc
// POST /v1/orgs/{org}/policy            VerbOrgPolicy (org admin)
// POST /v1/projects/{p}/policy          VerbProjectPolicy (team admin)
{ "v": 1, "all": { "permissions": { "deny": [ "Bash(curl *)" ] } }, "worker": { … } }
// 201 created, or 200 when the canonical bytes equal the scope's newest version
{ "id": "pv-3f2a91c07b1e4d2a", "scope": "org" | "project", "projectId": null | "pj-…",
  "seq": 4, "digest": "sha256:…", "state": "pending" | "active",
  "author": "us-…", "createdAt": "…", "activatedAt": null | "…",
  "audit": { "method": "POST", "path": "/v1/orgs/{orgId}/policy-versions",
             "body": { "id": "pv-…", "projectId": null, "seq": 4, "digest": "sha256:…" } } }
// 422 — Render refused it; nothing was written
{ "error": "policy_refused", "message": "project layer breaks I10 for the worker class: …",
  "details": { "invariant": "I10", "class": "worker", "scope": "project", "project": "pj-…" } }
// 422 — not the envelope (task 13's ErrLayer)
{ "error": "policy_refused", "message": "policy: layer refused: unknown section \"workers\" …",
  "details": { "invariant": "layer" } }
// 409 — two admins posted at once; the loser retries
{ "error": "conflict", "message": "Another version was written at the same moment. Try again." }

// GET /v1/orgs/{org}/policy             VerbOrgPolicy
// GET /v1/projects/{p}/policy           VerbProjectPolicy
{ "items": [ { …the POST answer without "audit", plus "doc": { …the envelope… } } ],  // newest first, at most 20
  "current": "pv-…" | null }                                                         // newest active
```

`audit` tells the admin's client exactly what to send to the Hub next (step 2 of the order
above), so the client does not re-derive it. It is instructions, not a record.
`TestRefusalIsNamedPerInvariant` pins that `details.invariant` is task 13's id and never a
message string, because the admin's client shows the invariant's name.

`control-plane/internal/fleet/server/policy.go`, complete:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/delegation"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/store"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/policy"
)

const CodePolicyRefused = "policy_refused" // 422

// RegisterPolicy adds OD-23's four routes and the feed consumer that
// activates a version once the Hub has audited it.
func RegisterPolicy(s *Server, pol *policy.Store, feed *delegation.Feed) {
	s.Route("GET /v1/orgs/{org}/policy", VerbOrgPolicy, func(w http.ResponseWriter, r *http.Request, p Principal) {
		listPolicy(w, r, pol, p.OrgID, "")
	})
	s.Route("POST /v1/orgs/{org}/policy", VerbOrgPolicy, func(w http.ResponseWriter, r *http.Request, p Principal) {
		putPolicy(w, r, s.log, pol, p, "")
	})
	s.Route("GET /v1/projects/{p}/policy", VerbProjectPolicy, func(w http.ResponseWriter, r *http.Request, p Principal) {
		listPolicy(w, r, pol, p.OrgID, p.ProjectID)
	})
	s.Route("POST /v1/projects/{p}/policy", VerbProjectPolicy, func(w http.ResponseWriter, r *http.Request, p Principal) {
		putPolicy(w, r, s.log, pol, p, p.ProjectID)
	})
	feed.Also(func(ctx context.Context, ev hubapi.Event) error { return activateOn(ctx, s.log, pol, ev) })
}

func putPolicy(w http.ResponseWriter, r *http.Request, log *slog.Logger, pol *policy.Store, p Principal, project string) {
	doc, err := io.ReadAll(http.MaxBytesReader(w, r.Body, policy.MaxLayerBytes+1))
	if err != nil || len(doc) > policy.MaxLayerBytes {
		httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, CodePolicyRefused,
			"A policy layer is at most 64 KiB.", map[string]string{"invariant": "layer"})
		return
	}
	l, created, err := pol.Put(r.Context(), p.OrgID, project, model.Owner(p.UserID), doc)
	var re *policy.RefusedError
	var ie *policy.InvariantError
	switch {
	case errors.As(err, &re) && errors.As(err, &ie): // Put wraps every refusal in a RefusedError
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, CodePolicyRefused, err.Error(), map[string]string{
			"invariant": ie.Invariant, "class": string(ie.Class), "scope": string(ie.Scope), "project": re.Project})
		return
	case errors.As(err, &re): // ErrLayer: the envelope itself
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, CodePolicyRefused, err.Error(),
			map[string]string{"invariant": "layer"})
		return
	case errors.Is(err, store.ErrConflict):
		httpx.WriteProblem(w, http.StatusConflict, "conflict",
			"Another version was written at the same moment. Try again.", nil)
		return
	case err != nil:
		log.Error("policy put", "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal, "Something went wrong.", nil)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	out := view(l, false)
	out["audit"] = map[string]any{"method": "POST", "path": "/v1/orgs/{orgId}/policy-versions",
		"body": map[string]any{"id": l.ID, "projectId": nullable(l.Project), "seq": l.Seq, "digest": l.Digest}}
	writeJSON(w, status, out)
}

func listPolicy(w http.ResponseWriter, r *http.Request, pol *policy.Store, org, project string) {
	ls, err := pol.List(r.Context(), org, project)
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal, "Something went wrong.", nil)
		return
	}
	items, current := []map[string]any{}, any(nil)
	for i, l := range ls {
		if i < 20 {
			items = append(items, view(l, true))
		}
		if current == nil && !l.ActivatedAt.IsZero() {
			current = l.ID
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "current": current})
}

// activateOn is the feed consumer. Only a store failure is returned (so the
// page is re-read); a refusal is logged and dropped, or the feed would stall.
func activateOn(ctx context.Context, log *slog.Logger, pol *policy.Store, ev hubapi.Event) error {
	if ev.Kind != "policy.versioned" {
		return nil
	}
	org, _ := ev.Data["orgId"].(string)
	project, _ := ev.Data["projectId"].(string) // absent or null for the org scope
	digest, _ := ev.Data["digest"].(string)
	err := pol.Activate(ctx, org, project, ev.Subject, digest)
	var re *policy.RefusedError
	switch {
	case err == nil:
		log.Info("policy version active", "id", ev.Subject, "org", org, "project", project)
		return nil
	case errors.Is(err, policy.ErrNotPending), errors.Is(err, policy.ErrDigestMismatch), errors.As(err, &re):
		log.Warn("policy version not activated", "id", ev.Subject, "err", err)
		return nil
	}
	return err
}

func view(l store.PolicyLayer, withDoc bool) map[string]any {
	scope, state, activated := "org", "pending", any(nil)
	if l.Project != "" {
		scope = "project"
	}
	if !l.ActivatedAt.IsZero() {
		state, activated = "active", l.ActivatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	v := map[string]any{"id": l.ID, "scope": scope, "projectId": nullable(l.Project), "seq": l.Seq,
		"digest": l.Digest, "state": state, "author": string(l.Author),
		"createdAt": l.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"), "activatedAt": activated}
	if withDoc {
		v["doc"] = json.RawMessage(l.Doc)
	}
	return v
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

The first case sets both targets: `re` for the project the layer was rendered against, `ie`
(reached through `RefusedError.Unwrap`) for the invariant. An `ErrLayer` refusal matches only the
second case.

#### The Hub delta: route 80, complete wire

Built on the Hub branch after its sync-merge, in task 06's handler shape (`task-06.md:406-448`:
`s.Write`, caller class first, then the scope, then the body). It is a **write of an audit event
and nothing else**, like task 07's orchestrator turns: the Hub keeps no policy record, because
the bytes are Fleet's and the Hub's job is *who, when, which digest*.

```jsonc
// POST /v1/orgs/{orgId}/policy-versions      desk only; Idempotency-Key = the pv id
{ "id": "pv-3f2a91c07b1e4d2a", "projectId": null | "pj-…", "seq": 4, "digest": "sha256:…" }
// 201
{ "id": "pv-…", "orgId": "org-…", "projectId": null, "seq": 4, "digest": "sha256:…",
  "recordedBy": "us-…", "recordedAt": "…" }
// 403 forbidden, details.required = "org_admin" (projectId null) or "team_admin"
// 400 bad_request: id not ^pv-[0-9a-f]{16}$, seq < 1, digest not ^sha256:[0-9a-f]{64}$, unknown field
// 404 project_not_found: projectId not in orgId (or not readable by the caller)
```

| Contract row | Method | Path | Caller and role | Route name | Task | Section |
|---|---|---|---|---|---|---|
| 80 | POST | `/v1/orgs/{orgId}/policy-versions` | desk; org admin for the org scope, team admin for a project | — | T14 | §7.27 |

| Audit kind | Audit | Member feed | Subject | `data` | Fleet feed |
|---|---|---|---|---|---|
| `policy.versioned` | ✓ | — | the `pv-…` id | `{"orgId","projectId","seq","digest"}` | yes, **whatever `projectId` is** |

Two consequences for task 08's route 78: its Fleet filter serves "team projects" events, and an
org-scope `policy.versioned` has `projectId` null. `store/changes.go`'s Fleet kind list gains
`policy.versioned` with the rule *served for the org regardless of project*; the conformance row
`team/policy-versioned-reaches-fleet` pins it for both scopes. And the `data` carries `orgId`,
because task 11's `Feed.Also` hands a consumer the event, not the org it polled.

```go
// control-plane/internal/hub/server/policyversions.go (the handler)
func (s *Server) recordPolicyVersion(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgId")
	s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
		if err := RequireDesktop(tx.P); err != nil { // Fleet, or a borrowed delegation, never
			return WriteResult{}, err
		}
		if tx.P.OrgID != orgID {
			return WriteResult{}, Refuse(404, hubapi.CodeRouteNotFound, "No such route.", nil)
		}
		var req hubapi.PolicyVersionRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		if err := req.Validate(); err != nil { // the three patterns above; seq >= 1
			return WriteResult{}, Refuse(400, hubapi.CodeBadRequest, err.Error(), nil)
		}
		if req.ProjectID == nil {
			if !tx.P.OrgAdmin {
				return WriteResult{}, Refuse(403, hubapi.CodeForbidden, "Only an org admin sets org policy.",
					map[string]string{"required": "org_admin"})
			}
		} else if _, role, ok, err := ProjectRole(r.Context(), tx, *req.ProjectID); err != nil {
			return WriteResult{}, err
		} else if !ok {
			return WriteResult{}, Refuse(404, hubapi.CodeProjectNotFound, "No such project.", nil)
		} else if role != hubapi.RoleAdmin {
			return WriteResult{}, Refuse(403, hubapi.CodeForbidden, "Only a team admin sets project policy.",
				map[string]string{"required": "team_admin"})
		}
		tx.Emit(Event{Kind: "policy.versioned", ProjectID: deref(req.ProjectID), Subject: req.ID,
			Data: map[string]any{"orgId": orgID, "projectId": req.ProjectID, "seq": req.Seq, "digest": req.Digest}})
		return WriteResult{Status: 201, Body: hubapi.PolicyVersionRecord{ID: req.ID, OrgID: orgID,
			ProjectID: req.ProjectID, Seq: req.Seq, Digest: req.Digest, RecordedBy: tx.P.UserID, RecordedAt: tx.Now}}, nil
	})
}
```

`tx.P.OrgID`, `tx.P.OrgAdmin`, `tx.Now` and `ProjectRole`'s three results are the Hub's names as
task 06 uses them (`task-06.md:426-433`); the Principal fields are Hub task 13's, **UNVERIFIED
until merge**. The request and record types go in task 03's `hubapi/team.go`:

```go
type PolicyVersionRequest struct {
	ID        string  `json:"id"`        // ^pv-[0-9a-f]{16}$
	ProjectID *string `json:"projectId"` // nil: the org scope
	Seq       int     `json:"seq"`       // >= 1
	Digest    string  `json:"digest"`    // ^sha256:[0-9a-f]{64}$
}

type PolicyVersionRecord struct {
	ID         string  `json:"id"`
	OrgID      string  `json:"orgId"`
	ProjectID  *string `json:"projectId"`
	Seq        int     `json:"seq"`
	Digest     string  `json:"digest"`
	RecordedBy string  `json:"recordedBy"`
	RecordedAt string  `json:"recordedAt"`
}
```

A replay is the Hub's idempotency (contract §1, `hub-contract.md:50-55`): the admin's client sends
`Idempotency-Key: <the pv id>`, so a retry after a lost response returns the stored 201 and writes
no second event, and the same key with another digest is 422 `idempotency_mismatch`.

**A client that dies between the two calls leaves a pending version, and that is recoverable.**
Re-posting the same layer to Fleet returns the same pending version (200) with the same `audit`
instructions, and sending those to the Hub activates it. Nothing is live in between, so a crash
can delay a policy change but never apply one unaudited.

- [ ] **Step 1: Write the failing tests**

`policy/store_test.go` runs on task 10's `memstore.New()`; "planted" means written with the store's
own `Tx` methods, bypassing `Put`, to reach states `Put` would refuse. `server/policy_test.go`
uses task 12's `fakeValidator` and `fakeMembers`. The Hub test runs on Hub task 06's harness.

| File · test | What it proves | The mutant it kills |
|---|---|---|
| store · `TestVersionOneIsTheDefault` | no layer: `Effective` → version 1, `Layers` empty, `CreatedBy ""`, digest = `Render(Layers{}, class)`'s | a zero version reaching the agent start |
| store · `TestAPendingLayerIsNeverRendered` | `Put` an org layer adding `env.FOO`: `Effective` still has no `FOO`; `Activate` → it has | rendering the newest layer instead of the newest **active** one |
| store · `TestTheSameBytesTwiceGiveOneVersion` | `Put` a doc, then the same doc with keys reordered and re-indented → `created` false, same id, same digest | a digest over the raw bytes |
| store · `TestARefusedLayerCreatesNoVersion` | `Put` a deny of `Bash(panacea *)` → `*RefusedError` wrapping I10; `List` is empty | appending before validating |
| store · `TestAnOrgLayerIsCheckedAgainstEveryProject` | plant an active project layer that breaks I7; `Put` a harmless org layer → `*RefusedError{Project: that project}` | an org `POST` rendered with no project only |
| store · `TestActivateRevalidates` | plant a pending project layer that breaks I10; `Activate` with its digest → `*RefusedError`, still pending | activation trusting the validation done at `Put` |
| store · `TestActivateRefusesAnotherDigest` | `Activate` with a digest the row does not hold → `ErrDigestMismatch`, still pending | activating whatever id the Hub names |
| store · `TestActivateIsOnce` | two activations: the second returns nil and `ActivatedAt` is the first's | a moving audit time |
| store · `TestEffectiveAppendsOnlyWhenBytesChange` | `Effective` ×2 → version 1 both; activate an org layer → 2, `Layers` = [its id]; again → 2 | a new version per agent start |
| store · `TestEffectiveRaceSettles` | 16 goroutines call `Effective` at once → one digest, one version, exactly one row appended | no `ErrConflict` retry, or a retry that appends a duplicate |
| server · `TestAMemberCannotReadOrWritePolicy` | a project *member*'s `GET` and `POST` → 403 `required: "policy.project"` | the routes registered under a broader verb |
| server · `TestAnOrgAdminIsNotATeamAdmin` | org admin, team *member*: project `POST` → 403; org `POST` → 201 | org admin granted every project's layer |
| server · `TestRefusalIsNamedPerInvariant` | a project layer breaking I10 → 422, `details` = `{invariant: I10, class: worker, scope: project, project: pj-…}` | an error string where the admin's client needs a field |
| server · `TestAnEnvelopeErrorIs422Layer` | `{"v":2}` → 422, `details.invariant = "layer"` | a malformed layer answered 500 |
| server · `TestAnOversizedLayerIs413` | 64 KiB + 1 → 413, nothing stored | an unbounded read |
| server · `TestPostCarriesTheAuditInstructions` | `audit.body` equals `{id, projectId: null, seq, digest}` of the new version | a client left to guess the Hub call |
| server · `TestTheFeedActivatesAndNeverStalls` | a `policy.versioned` event → active; a second event with a wrong digest → the consumer returns nil and logs | a refusal returned to `Poll`, which re-reads the page forever |
| hub · `TestFleetCannotRecordAPolicyVersion` | a Fleet token under a delegation → 403 `required: "desktop"` | Fleet auditing its own policy in a user's name |
| hub · `TestTheScopeDecidesTheRole` | org scope: team admin → 403 `org_admin`; project scope: org admin not on the team → 403 `team_admin`; each right role → 201 | one role check for both scopes |
| hub · `TestTheEventIsMetadataOnly` | the audit `data` keys are exactly `orgId, projectId, seq, digest` | the layer's bytes copied into the audit chain |
| hub · `TestAReplayWritesNoSecondEvent` | two `POST`s with one `Idempotency-Key` → one event, the second answer has `Idempotent-Replay: true` | a double audit on retry |
| conformance · `team/policy-versioned-reaches-fleet` | both scopes appear on route 78 for the org | route 78's team-project filter dropping the org scope |

One test, complete — the property the design names (*"which policy governed this agent"*):

```go
func TestEffectiveAppendsOnlyWhenBytesChange(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	clk := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	pol := NewStore(st, func() time.Time { return clk })

	v1, err := pol.Effective(ctx, "org-a", "pj-one", model.ClassWorker)
	if err != nil || v1.Version != 1 || len(v1.Layers) != 0 {
		t.Fatalf("first Effective = %+v, %v; want version 1 from the defaults", v1, err)
	}
	if again, _ := pol.Effective(ctx, "org-a", "pj-one", model.ClassWorker); again.Version != 1 {
		t.Fatalf("unchanged layers gave version %d; want 1 again", again.Version)
	}
	orch1, _ := pol.Effective(ctx, "org-a", "pj-one", model.ClassOrchestrator)
	l, _, err := pol.Put(ctx, "org-a", "", "us-admin",
		[]byte(`{"v":1,"worker":{"permissions":{"deny":["Bash(curl *)"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.Activate(ctx, "org-a", "", l.ID, l.Digest); err != nil {
		t.Fatal(err)
	}
	v2, err := pol.Effective(ctx, "org-a", "pj-one", model.ClassWorker)
	if err != nil || v2.Version != 2 || !slices.Equal(v2.Layers, []string{l.ID}) || v2.CreatedBy != "us-admin" {
		t.Fatalf("after the org layer: %+v, %v; want version 2 from %s by us-admin", v2, err, l.ID)
	}
	if !bytes.Contains(v2.Body, []byte(`"Bash(curl *)"`)) || Digest(v2.Body) != v2.Digest {
		t.Fatalf("version 2's body is not the rendered file its digest names")
	}
	orch2, _ := pol.Effective(ctx, "org-a", "pj-one", model.ClassOrchestrator)
	if orch1.Version != 1 || orch2.Version != 1 || orch1.Digest != orch2.Digest {
		t.Fatalf("a worker-only section moved the orchestrator: %d -> %d", orch1.Version, orch2.Version)
	}
}
```

Run: `cd control-plane && go test ./internal/policy/ ./internal/fleet/server/ -count=1`.
Expected before step 2: compile failures naming `NewStore`, `RegisterPolicy`,
`store.PolicyLayer`.

- [ ] **Step 2: Extend task 10's store** — the type, the four methods, `PolicyVersion.Layers`, both
  implementations, the three `storetest` rows, and the migration. Run `go test
  ./internal/fleet/store/... -count=1` (with `PANACEA_APP_FLEET_TEST_PG_DSN` set, so `pgstore`
  runs the suite too). Expected: `ok` for `memstore` and `pgstore`, and the new rows among the
  passes. Commit: `feat(fleet/store): OD-23 layer versions, append-only, activated once`.

- [ ] **Step 3: Write `policy/store.go`, then `server/policy.go`, then the `main.go` line**
  `server.RegisterPolicy(srv, policy.NewStore(st, time.Now), feed)` after `server.New`, and
  `"policy.versioned"` appended to `kinds`. Run step 1's command. Expected: `ok` twice. Commit:
  `feat(fleet): OD-23 versions and routes; a version is live only once the Hub audits it`.

- [ ] **Step 4: The Hub delta**, on the merged Hub branch: `hubapi` types, the handler, the route
  in `server.go`, `policy.versioned` in the audit kinds and Fleet's feed list, contract row 80
  and §7.27 (the wire above, verbatim). Run `go test ./internal/hub/... -count=1`. Expected: `ok`,
  and the conformance row passes against the fake and the real Hub. Commit:
  `feat(hub): route 80, the policy.versioned audit event on Fleet's feed`.

- [ ] **Step 5: Acceptance**

```
cd control-plane
go vet ./internal/policy/ ./internal/fleet/... ./internal/hub/...
go test ./internal/policy/ ./internal/fleet/server/ ./internal/fleet/store/... ./internal/hub/... -count=1 -race
grep -rn "outbox\." internal/policy internal/fleet/server/policy.go
```

Expected: `go vet` silent; every package `ok`; the `grep` prints nothing — the audit goes through
the admin's own Hub call, not Fleet's outbox, and a future edit that routes it through the outbox
(where it would need an exemption task 01 does not grant) shows up here. The end-to-end run —
an admin posts a layer, calls route 80, and a new agent starts with the new `{version, digest}` —
is **deferred to task 33**, which scripts it on the real cluster.

**What this task does not claim.**
- The skeleton's "through the outbox" is not built; the order above replaces it, for the reasons
  given. Route 80 is a Hub delta this task adds; task 01's contract table does not have it yet.
- No admin view. TM1-α's only client for these routes is task 33's script; a desktop surface for
  policy is not in this plan.
- It puts nothing in a pod. Task 20 calls `Effective` and task 15 mounts `Body` and checks
  `Digest`; this task proves only that the right bytes and numbers are produced and stored.
- A pending version never expires; one whose Hub call never comes stays pending and is listed as
  such. A janitor for abandoned pending versions is not built.
- A rendered version records the layer ids it came from, not the gateway fragment's version: the
  fragment is pinned by task 13's test and changes only with an image build, and the digest covers
  it.
- The Hub-side names (`tx.P.OrgID`, `tx.P.OrgAdmin`, `ProjectRole`) are as task 06 uses them and
  **UNVERIFIED until the Hub merge**.
