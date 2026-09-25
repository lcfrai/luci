### Task 17: Phase 8 delta — the broker's second caller, the control relay, pod source addresses

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 16 (`sandboxd`'s `/control`), P8-17 (the broker's
chassis), P8-18 (its sandbox routes and attach relay). **Seams:** consumes ST.3 (delegation) and
P8-D4; leaves P8-D12 unchanged.

**Built after the sync-merge of `feat/env`.** The broker does not exist yet: `control-plane/cmd/`
on `feat/env` holds `api` only (read 2026-09-25). Every broker path below is phase 8's **plan**,
`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-17.md`
(chassis: `Config` at `:69-94`, `newServer` and `ownerOf` at `:130-143`) and `task-18.md` (the wire
types at `:44-94`, the five routes at `:120-124`, attach in order at `:231-249`), and is **UNVERIFIED
until merge**. One file this task extends *is* built: `control-plane/internal/entra/verify.go` on
`feat/env` (phase 8 task 16), whose `azp` check is at `:117` and whose `scp` check is at `:119-121`.

Phase 8 made the broker owner-only on purpose (P8-D4,
`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:133`): it
derives owner = `tid` + `oid` from the caller's token, labels every object with a hash of it, and
filters every read by it. OD-14 moves team agents into a managed service, so a second caller now
creates sandboxes, and that caller is not the owner. This task lets Fleet in **without** loosening
P8-D4. Fleet never becomes an owner. It acts **for** one, and only under a delegation that the owner
created from their own desktop (task 21) and that the broker checks **itself**, against the Hub, rather
than taking Fleet's word for it. Every object Fleet creates carries the owner's label, exactly as if
the owner's desktop had created it, so the owner's own `remotebe` lists it and nothing about owner
scoping forks into two cases.

It adds two more things Fleet needs and the owner's desktop must not have. A second relay kind,
`control`, reaches `sandboxd`'s `/control` (task 16) on the same port 8080 attach uses, so P8-D12's
single ingress rule (`:141`, *"ingress to a sandbox only from the broker on 8080"*) does not change.
And the broker records each pod's source address when it becomes ready, because phase 6's keysmith
binds a gateway token to the address it will be used from (P6-D7,
`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:71`), and in team mode
it is Fleet, not the broker, that mints (S6.5, `:290`; task 20).

**Decisions made here.**

1. **Fleet is an app-only caller, recognised by two facts.** A token with no `scp`, whose `azp` equals
   `PANACEA_APP_BROKER_FLEET_CLIENT_ID` and whose `roles` holds `Sandboxes.Fleet`. The built validator
   refuses any token without the required scope (`verify.go:119-121`), so `entra.Config` gains
   `AppCallers []AppCaller` and `Claims` gains `AppOnly bool`. An app-only token matching no entry is
   refused as before. This mirrors task 01's Hub rule 1 (`task-01.md:116-119`), spelled for the broker.
   If task 05 lands the same `AppCallers` field first, this task consumes it and adds nothing to
   `entra`.
2. **Fleet must name a delegation, and nobody else may.** A Fleet request without
   `X-Panacea-Delegation: dl-…` is `401 delegation_required`; a desktop token carrying the header is
   `403 forbidden` with `details.required = "fleet"`. The same two rules as the Hub's (`task-01.md:120-128`),
   so no client can borrow a delegation from either service.
3. **The broker verifies the delegation against the Hub, as itself.** It calls `GET
   /v1/delegations/{id}` with its own workload-identity token and caches the answer by id for at most
   `PANACEA_APP_BROKER_DELEGATION_TTL` (default and maximum 60 s). A revoked answer is cached as
   revoked. A Hub error with no cached answer is `503 delegation_unverifiable`: **fail closed**. This
   needs two things task 01 does not give (*Gap S17-1*): the broker as a caller of route 47
   (`task-01.md:254` lists the owner and Fleet only), and the owner's `tid`/`oid` in the answer,
   because the broker's owner label is `sandbox.OwnerLabelValue(tid, oid)` and task 01's delegation
   carries `ownerUserId` (`u-…`, `:323`).
4. **`ownerOf` becomes `principalOf`, still the only place a request becomes an owner.** It returns
   the owner from the token for a desktop, and from the verified delegation for Fleet, plus `via` and
   the `dl-…`. Every route calls it; no route reads an owner, a project or a class from a body.
5. **Fleet's scope is narrower than the owner's.** Under a delegation Fleet sees only the owner's
   sandboxes **in the delegation's project** (a second label selector, `panacea-app/project`), may create
   only the classes the delegation's class covers (task 15's `Spec.Class`): a `worker` delegation
   creates `worker` sandboxes only; an `orchestrator` delegation creates the Orchestrator and the
   workers it spawns (`panacea worker spawn`, task 24), because both act for the same owner and task
   01 gives the Orchestrator class every route (`task-01.md:295`). Anything else is `403
   delegation_class_mismatch`. Fleet may also use `control` and `source`, which a desktop token may not.
   Of P8-19's machine routes (`task-19.md:82-88`) Fleet may call exactly three, owner-scoped as the
   owner's own: `POST /v1/machines/default`, `GET /v1/machines/{id}` and `POST /v1/machines/{id}/wake`,
   because every cloud worker needs the owner's machine (`SandboxSpec.Machine` is required,
   `task-18.md:59`) and task 20 places a worker on the owner's default one. Creating, sleeping or
   deleting a machine stays the owner's desktop's act; under a delegation those routes are `403 forbidden`.
6. **Every object Fleet creates is labelled for audit, not for authority.** `panacea-app/via: fleet`,
   `panacea-app/delegation: dl-…` and `panacea-app/project: <id>` join the owner and control-plane
   labels. Authority still comes from the owner label and a fresh delegation check; the labels let an
   operator answer "which delegation made this pod" from `kubectl`.
7. **The broker does not mint for a Fleet sandbox.** Phase 6 asks the broker to mint at launch and
   write the token into a pod-owned Secret (ask P8-1, `2026-09-25-phase-6-gateway.md:306`). Under
   `via = fleet` the broker skips that mint and mounts `/run/panacea/gateway/` as a memory-backed
   `emptyDir` instead, because Fleet mints with team tags and a fresh key per Orchestrator turn (task
   20) and delivers each key down the control stream (task 16's `token` event, *Gap S17-2*). A Secret
   volume is refreshed by the kubelet on its own period, which a per-turn key cannot wait for, and
   whether a Kata pod sees a Secret update at all is itself unverified (`:306`).
8. **A live relay re-checks its delegation.** `control` and `attach` relays opened under a delegation
   re-verify it every `DELEGATION_TTL`; a revoked or lapsed answer closes both ends with status `4003`,
   reason `delegation ended`. That is what makes §7.4's "every socket is revocable within 60 seconds"
   true at the broker, not only in Fleet.
9. **The source address is recorded once, at ready, and read only by Fleet.** When katabe's readiness
   wait succeeds, the broker writes the pod's `status.podIP`, normalised as phase 6's `wire.NormalAddr`
   does, to the pod annotation `panacea-app/source-addr`, so both broker replicas and a restarted broker
   agree. `GET /v1/sandboxes/{id}/source` returns it to a Fleet principal; a desktop principal is
   `403`, because P8-18's `Sandbox` "deliberately has no Endpoint: the laptop never learns a pod IP"
   (`task-18.md:77-78`).
10. **The audit line names all three.** The request logger's line (`task-17.md:245`, "owner hash")
    gains `via` and `delegation`, and for a Fleet principal `owner` is the delegation owner's hash, not
    Fleet's. Fleet's own `oid` is never a label, a selector or an owner.
11. **A dead delegation may undo its own work, and nothing else.** When a delegation is revoked or
    lapses, Fleet must still destroy the sandboxes it made (task 20 decision 9, task 11's `Revoker`).
    So `principalOf` returns a *restricted* principal for a revoked or expired delegation, instead of
    the 401, on exactly one route: `DELETE /v1/sandboxes/{id}`, and only for a sandbox whose
    `panacea-app/delegation` label equals that `dl-…`. Every other route under it keeps its 401. The
    audit line records `via=fleet`, the `dl-…` and `delegation_state=revoked|expired`. Test:
    `TestRevokedDelegationMayDeleteOnlyItsOwn` — after revocation, `DELETE` of a sandbox labelled
    `dl-alice` is 204; of Alice's desktop sandbox (no label) is 401 `delegation_revoked`; `GET` is 401.
    Kills: a revocation that strands running pods, and a revocation that still lets Fleet act.

**Files (all UNVERIFIED until the `feat/env` merge, except `entra`):**
- `control-plane/internal/entra/entra.go`, `verify.go` — `Config.AppCallers`, `Claims.AppOnly`; the
  app-only branch beside `:117-121` (modified; built on `feat/env`)
- `control-plane/internal/entra/verify_test.go` — the app-only cases (modified)
- `control-plane/cmd/broker/delegated.go` — `Principal`, `principalOf`, `DelegationSource`, the
  60-second `delegationCache`, `hubDelegations` (new)
- `control-plane/cmd/broker/control.go` — `GET /v1/sandboxes/{id}/control` (new)
- `control-plane/cmd/broker/source.go` — the ready-time annotation and `GET /v1/sandboxes/{id}/source` (new)
- `control-plane/cmd/broker/{delegated,control,source}_test.go` (new)
- `control-plane/cmd/broker/config.go` — four `PANACEA_APP_BROKER_*` variables (modified)
- `control-plane/cmd/broker/server.go` — `ownerOf` replaced by `principalOf`; the two routes join
  the table; the log fields (modified)
- `control-plane/cmd/broker/sandboxes.go`, `attach.go` — the principal's labels, the class check, the
  skipped mint, the relay re-check (modified)
- `control-plane/internal/sandbox/manager.go` — `(*Manager).WithLabels`, beside `WithOwner` (`:113-134`,
  built) (modified)
- `control-plane/internal/brokerapi/codes.go`, `wire.go` — four codes, `HeaderDelegation`,
  `HeaderControlToken`, `SourceAddr` (modified)

**Interfaces:**

```go
// control-plane/internal/entra/entra.go — added to Config
//
// AppCallers admits an app-only token (no scp) whose azp is ClientID and whose
// roles hold Role. Empty keeps the validator delegated-only, as today.
AppCallers []AppCaller

type AppCaller struct {
	ClientID string // a GUID, validated as AllowedClients are
	Role     string // e.g. "Sandboxes.Fleet"
}

// Claims gains:
AppOnly bool // true only for a token admitted through AppCallers
```

```go
// control-plane/internal/brokerapi — added
const (
	HeaderDelegation   = "X-Panacea-Delegation" // ST.3; task 01's header name
	HeaderControlToken = "Panacea-Control-Token"

	CodeForbidden               = "forbidden"                 // 403 — a desktop on a Fleet-only route or header
	CodeDelegationRequired      = "delegation_required"       // 401 — Fleet without the header
	CodeDelegationRevoked       = "delegation_revoked"        // 401
	CodeDelegationExpired       = "delegation_expired"        // 401
	CodeDelegationUnverifiable  = "delegation_unverifiable"   // 503 — Hub unreachable, nothing cached
	CodeDelegationClassMismatch = "delegation_class_mismatch" // 403 — the spec class is not one the delegation covers
	CodeControlRejected         = "control_rejected"          // 403 — sandboxd refused the control token
)

// SourceAddr is GET /v1/sandboxes/{id}/source's body, Fleet only.
type SourceAddr struct {
	PodAddr    string    `json:"podAddr"`    // normalised, no zone, never IPv4-mapped
	RecordedAt time.Time `json:"recordedAt"` // when the pod first answered /healthz
}
```

```go
// control-plane/cmd/broker/delegated.go
package main

// Principal is who a request acts for. Owner is always an OwnerLabelValue.
// For a desktop, Via, Delegation, Project and Class are empty.
type Principal struct {
	Owner      string // sandbox.OwnerLabelValue(tid, oid) of the person
	Via        string // "" or "fleet"
	Delegation string // dl-…, when Via == "fleet"
	Project    string // the delegation's project, when Via == "fleet"
	Class      string // the delegation's class: orchestrator | worker
}

// Delegation is what the broker needs from the Hub's delegation record.
type Delegation struct {
	ID        string
	ProjectID string
	Class     string
	OwnerTID  string // Gap S17-1: task 01's record carries ownerUserId only
	OwnerOID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// DelegationSource reads one delegation. hubDelegations implements it over
// GET {PANACEA_APP_BROKER_HUB_URL}/v1/delegations/{id} with the broker's own
// workload-identity token; tests use a map.
type DelegationSource interface {
	Delegation(ctx context.Context, id string) (Delegation, error)
}

// ErrNoSuchDelegation is the source's answer for a 404: cached like a revocation.
var ErrNoSuchDelegation = errors.New("broker: no such delegation")

// principalOf is the only place a request becomes a Principal. It replaces
// P8-17's ownerOf. ok is false only after it has written the problem response.
func principalOf(w http.ResponseWriter, r *http.Request, d serverDeps) (p Principal, ok bool)
```

`serverDeps` (`task-17.md:253-257`) gains `Delegations *delegationCache` and `FleetClientID string`.
`delegationCache` wraps a `DelegationSource` with a per-id entry `{d Delegation; err error; at time.Time}`,
refreshed when `now - at >= TTL`; concurrent misses for one id make one Hub call (`singleflight`-style,
written by hand with a map of in-flight channels, because `control-plane/go.mod` does not require
`golang.org/x/sync` on either `feat/env` or `feat/team-mode`, checked 2026-09-25).

- [ ] **Step 1: Write the failing tests**

All on the broker's test harness from P8-17/P8-18 (a fake clientset, `entra`'s test key, a `sandboxd`
stand-in on `httptest`), plus a map-backed `DelegationSource` and a settable clock. Two principals:
Alice's desktop token, and Fleet's app-only token. Two delegations: `dl-alice` (Alice, `pj-shop`,
`worker`) and `dl-bob` (Bob, `pj-shop`, `worker`).

| Test | What it checks | Mutant it kills |
|---|---|---|
| `TestFleetWithoutDelegationIs401` | Fleet's token, no header, `GET /v1/sandboxes` → 401 `delegation_required` | treating Fleet's own `oid` as an owner |
| `TestDesktopCarryingDelegationIs403` | Alice's token with `X-Panacea-Delegation: dl-alice` → 403, `details.required = "fleet"` | a client borrowing a delegation |
| `TestAppOnlyTokenOfAnotherClientIs401` | an app-only token, right role, other `azp` → 401 | admitting on role alone |
| `TestAppOnlyTokenWithoutRoleIs401` | Fleet's `azp`, no `Sandboxes.Fleet` → 401 | admitting on `azp` alone |
| `TestDelegationForAliceCannotListBob` | Bob's desktop creates `s-b`; Fleet under `dl-alice` lists → `s-b` absent; `GET /v1/sandboxes/s-b` → 404 `sandbox_not_found` | selecting on project without owner |
| `TestDelegationScopesToItsProject` | Alice's desktop creates `s-solo` (no project label); Fleet under `dl-alice` lists → absent | selecting on owner without project |
| `TestFleetCreateCarriesOwnerLabel` | Fleet creates under `dl-alice`; the pod's `panacea-app/owner` = `OwnerLabelValue(alice)`, and Alice's desktop `GET /v1/sandboxes` lists it | labelling with Fleet's identity |
| `TestClassMismatchIs403` | a `worker` delegation creating `Class: "orchestrator"` → 403 `delegation_class_mismatch`, no pod; an `orchestrator` delegation creating `worker` → 201 | trusting the spec's class, or a rule so strict an Orchestrator cannot spawn |
| `TestRevokedDelegationFailsWithinTTL` | at t0 a call succeeds (cached); revoke at t0+1 s; at t0+59 s the cache may still admit; at t0+60 s → 401 `delegation_revoked` | a cache without a lifetime, or one longer than 60 s |
| `TestHubDownWithNothingCachedIs503` | the source errors, no entry → 503 `delegation_unverifiable` | failing open when the Hub is unreachable |
| `TestAuditLineNamesTheDelegation` | a captured `slog` line for Fleet's create holds `via=fleet`, `delegation=dl-alice`, `owner=<alice hash>`, and no `oid` of Fleet | an audit line naming only the caller |
| `TestControlRelayIsFleetOnly` | Alice's desktop `GET …/s-a/control` → 403 | letting an owner supersede Fleet's stream (task 16 decision 1) |
| `TestControlRelayReachesSandboxdControl` | Fleet under `dl-alice` relays; the stand-in's `/control` receives a frame and the answer comes back | a relay that dials `/attach` |
| `TestControlRelayClosesOnRevocation` | relay open; revoke; advance the clock 60 s → both ends closed `4003` | a relay that checks only at the upgrade |
| `TestSourceAddrRecordedAtReady` | after create, the pod carries `panacea-app/source-addr`; `GET …/source` → the stand-in pod's IP; Alice's desktop → 403 | leaking the pod IP to the laptop |
| `TestFleetMachineRoutesAreThree` | under `dl-alice`: `POST /v1/machines/default` → 200/201 for Alice's machine; `POST /v1/machines`, `…/sleep`, `DELETE …` → 403 `forbidden` | a Fleet that can delete a person's disk |
| `TestBrokerDoesNotMintForFleet` | the fake keysmith records a mint for Alice's desktop create and **none** for Fleet's, and Fleet's pod mounts an `emptyDir` at `/run/panacea/gateway` | two live keys for one session |

In `control-plane/internal/entra/verify_test.go`: `TestAppCallerAdmitted` (no `scp`, listed `azp`, role
held → `Claims.AppOnly`), `TestAppCallerNeedsBothFacts` (each of the two missing → the existing
`ErrClient` or `ErrScope`), `TestDelegatedTokenUnchanged` (every existing case still passes; kills a
branch that skips the `scp` check for every token when `AppCallers` is set).

The revocation test is the one the scope names; its shape, since the window is the contract:

```go
func TestRevokedDelegationFailsWithinTTL(t *testing.T) {
	h := newHarness(t) // fake clock at t0, dl-alice live, TTL 60 s
	if code := h.fleetList(t, "dl-alice"); code != http.StatusOK {
		t.Fatalf("t0: %d", code)
	}
	h.src.Revoke("dl-alice", h.now().Add(time.Second))
	h.advance(59 * time.Second) // inside the window: a cached admit is allowed, not required
	_ = h.fleetList(t, "dl-alice")
	h.advance(time.Second) // t0 + 60 s: the entry is stale and must be re-read
	if code, prob := h.fleetListProblem(t, "dl-alice"); code != http.StatusUnauthorized ||
		prob.Code != brokerapi.CodeDelegationRevoked {
		t.Fatalf("t0+60s: %d %s, want 401 delegation_revoked", code, prob.Code)
	}
}
```

- [ ] **Step 2: The app-only branch in `entra`**

In `verify.go`, the `azp` check (`:117`) and the `scp` check (`:119-121`) become one decision. The
existing path is untouched for a token that has an `scp`:

```go
	scopes := strings.Fields(p.Scp)
	appOnly := false
	switch {
	case len(scopes) == 0 && v.appCaller(p.Azp, p.Roles):
		appOnly = true // Fleet: no scp, a listed azp, the listed role
	case !v.allowed(p.Azp): // the existing :117 check, unchanged
		return Claims{}, fmt.Errorf("%w: azp is not an allowed client", ErrClient)
	case !contains(scopes, v.scope): // the existing :119-121 check, unchanged
		return Claims{}, fmt.Errorf("%w: scp does not hold %s", ErrScope, v.scope)
	}
```

`appCaller` is true when some `AppCaller` has `ClientID == azp` and `Role` in `roles`. `New` validates
each `AppCaller.ClientID` as it validates `AllowedClients` and refuses an empty `Role`. `Claims.AppOnly`
is set from `appOnly`. The helper names `allowed` and `contains` stand for whatever the built file
uses at `:117` and `:120`; the implementer keeps the existing calls rather than renaming them.

- [ ] **Step 3: `principalOf` and the cache (`delegated.go`)**

```go
func principalOf(w http.ResponseWriter, r *http.Request, d serverDeps) (Principal, bool) {
	c, _ := entra.ClaimsFrom(r.Context()) // entra.Middleware ran first
	dl := r.Header.Get(brokerapi.HeaderDelegation)
	if !c.AppOnly {
		if dl != "" {
			httpx.WriteProblem(w, http.StatusForbidden, brokerapi.CodeForbidden,
				"the delegation header is Fleet's", map[string]string{"required": "fleet"})
			return Principal{}, false
		}
		return Principal{Owner: sandbox.OwnerLabelValue(c.TenantID, c.ObjectID)}, true
	}
	if dl == "" {
		httpx.WriteProblem(w, http.StatusUnauthorized, brokerapi.CodeDelegationRequired,
			"Fleet must name a delegation", nil)
		return Principal{}, false
	}
	got, err := d.Delegations.Get(r.Context(), dl)
	now := d.Now()
	switch {
	case errors.Is(err, ErrNoSuchDelegation), err == nil && got.RevokedAt != nil:
		httpx.WriteProblem(w, http.StatusUnauthorized, brokerapi.CodeDelegationRevoked, "delegation ended", nil)
	case err != nil:
		httpx.WriteProblem(w, http.StatusServiceUnavailable, brokerapi.CodeDelegationUnverifiable,
			"the Hub could not confirm the delegation", nil)
	case !now.Before(got.ExpiresAt):
		httpx.WriteProblem(w, http.StatusUnauthorized, brokerapi.CodeDelegationExpired, "delegation lapsed", nil)
	default:
		return Principal{Owner: sandbox.OwnerLabelValue(got.OwnerTID, got.OwnerOID), Via: "fleet",
			Delegation: got.ID, Project: got.ProjectID, Class: got.Class}, true
	}
	return Principal{}, false
}
```

An unknown delegation id answers `delegation_revoked`, not a distinct "not found": a caller probing
ids learns nothing it could not learn from a revoked one. `httpx.WriteProblem(w, status, code, msg,
details map[string]string)` is phase 2's, built (`control-plane/internal/httpx/problem.go:93`); `httpx`
has no `forbidden` code (its 403s are `forbidden_origin` and `forbidden_host`, `:49-50`), so
`brokerapi` adds `CodeForbidden = "forbidden"`, the Hub's spelling. The owner view every handler then uses is
`d.Manager.WithOwner(p.Owner)`, plus `.WithLabels(map[string]string{"panacea-app/project": p.Project})`
when `p.Via == "fleet"`. On create under Fleet, the labels are all three of decision 6, the class is
checked against what `p.Class` covers (decision 5) before any Kubernetes call, and P8-1's mint is skipped (decision 7).

The request logger reads the principal from the request context (`principalOf` stores it with
`context.WithValue` under an unexported key) and logs `owner`, `via` and `delegation`; for a request
refused before a principal exists it logs `via=fleet` and the header's `dl-…` if present, so a refused
Fleet call is still attributable.

- [ ] **Step 4: The `control` relay (`control.go`)**

The same order as P8-18's attach (`task-18.md:270-284`), dial before upgrade, so every failure is an
HTTP status and a problem code:

1. `principalOf`; a principal with `Via != "fleet"` is `403 forbidden`.
2. `Panacea-Control-Token` present, else `400 bad_request`.
3. The owner-and-project view's `Get(id)`: absent → `404 sandbox_not_found`; not running → `409`.
4. `backend.Dial` to `ws://<podIP>:8080/control` with the control token as bearer and
   `SetReadLimit(backend.MaxMessageBytes)`. `sandboxd` answering 401 or 403 → `403 control_rejected`.
   The broker's own address is what `sandboxd` sees, so task 16's in-pod refusal passes it.
5. `websocket.Accept` the caller, `SetReadLimit(backend.MaxMessageBytes)` at once, a 30 s ping
   keepalive, then `proxy.RelayHooked(ctx, caller, upstream, proxy.Hooks{})`.
6. A goroutine re-reads the delegation through the cache every `DelegationTTL`; revoked, expired or
   unverifiable closes both ends `4003 delegation ended`. `attach.go` under a Fleet principal gets the
   same goroutine (decision 8). Under a desktop principal nothing changes.

`backend.Dial`'s path argument is today the attach path; if it is hard-coded to `/attach` at merge,
this task adds a `path` parameter rather than copying the function (seam S8.7, one relay).

- [ ] **Step 5: The source address (`source.go`)**

After katabe's readiness wait returns in the create handler, and only for `Via == "fleet"` (a desktop
sandbox never needs it), the broker patches the pod's annotation `panacea-app/source-addr` with
`netip.ParseAddr(pod.Status.PodIP)` unmapped and zone-free. The patch failing deletes the new pod and
answers `503 backend_unavailable`, because a Fleet sandbox without a recorded address cannot receive a
bound token (task 20 would fail its mint) and a half-made session is worse than none. `GET …/source`
reads the annotation through the owner-and-project view; `recordedAt` is a second annotation,
`panacea-app/source-addr-at`, in RFC 3339.

- [ ] **Step 6: Run the tests and the acceptance commands**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go test ./cmd/broker/... ./internal/entra/... ./internal/sandbox/... ./internal/brokerapi/... && go vet ./cmd/broker/... ./internal/entra/...'
```

Expected: `ok` for each of the four packages; `go vet` prints nothing. These are ordinary Go tests
on a fake clientset and run on Windows.

```bash
~/.claude/tools/claudezsh/claudezsh 'cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && go test -run "^Test(FleetWithout|DesktopCarrying|AppOnlyToken|DelegationFor|DelegationScopes|FleetCreate|ClassMismatch|RevokedDelegation|HubDown|AuditLine|ControlRelay|SourceAddr|FleetMachineRoutes|BrokerDoesNotMint|RevokedDelegationMayDelete)" -v ./cmd/broker/ | grep -c "^--- PASS"'
```

Expected: `18`, one per broker row of step 1 plus decision 11's (the three `entra` cases run in their own package). A
lower number means a row was not written, not that it was skipped: none of them calls `t.Skip`.

- [ ] **Step 7: Commit**, `git add` for the new files, then `git commit -F
  .superpowers/sdd/team/msg/t17.txt -- control-plane/cmd/broker control-plane/internal/entra
  control-plane/internal/sandbox/manager.go control-plane/internal/brokerapi`.

**What this task does not claim.**

- That any of this runs against a real cluster, a real Hub or a real Entra app role. The Fleet
  identity and its `Sandboxes.Fleet` role are created by task 32 (Fleet's chart and identity); the
  end-to-end check is task 33.
- That P8-D12's NetworkPolicy admits the relay unchanged. It should — the broker is already the only
  8080 ingress — but the policy file is phase 8 task 11's and is read only at merge.
- That the pod's `status.podIP` is the address the gatekeeper sees. That is phase 6 task 24's probe
  P03 (`2026-09-25-phase-6-gateway.md:276`); if it fails, seam S6.10's client certificate replaces the
  annotation.
- **Gap S17-1 — the broker as a Hub caller.** Task 01's route 47, `GET /v1/delegations/{id}`, admits
  "its owner (desk), or Fleet under it" (`task-01.md:254`) and its record names the owner as
  `ownerUserId` (`:323`). The broker needs to be a third caller of that one route (app-only, role
  `Hub.DelegationReader`) and needs `ownerTid` and `ownerOid` in the answer to it. Without that the
  broker would have to trust Fleet's statement of whom it acts for, which is the thing P8-D4 exists to
  prevent. Task 01 (contract) and task 06 (the route) should add both; this task's tests run on a
  `DelegationSource` fake until they do.
- **Gap S17-2 — P8-1 assumes the broker mints.** Phase 6's ask P8-1 has the broker mint for every
  cloud session. Decision 7 skips it for Fleet's sandboxes and moves delivery to the control stream
  (task 16 step 6b). Phase 6's plan names S6.5 ("Fleet as a mint caller") but not the delivery path;
  the skeleton's delta table row for this task does not mention it either.

`(*sandbox.Manager).WithLabels(extra map[string]string) *Manager` stamps each pair on every object it
creates and adds each to every selector, like `WithOwner`; it panics on a key outside
`panacea-app/{via,delegation,project}` so it cannot be used to widen or to overwrite the owner label.

New configuration (`config.go`, parsed with the rest, a bad value fails start naming the variable):

| Variable | Meaning | Default |
|---|---|---|
| `PANACEA_APP_BROKER_FLEET_CLIENT_ID` | Fleet's workload identity's client id (a GUID). Empty = no Fleet caller at all | empty |
| `PANACEA_APP_BROKER_FLEET_ROLE` | the app role Fleet's token must hold | `Sandboxes.Fleet` |
| `PANACEA_APP_BROKER_HUB_URL` | the Hub's base URL; `https` unless loopback. Required when the Fleet client id is set | empty |
| `PANACEA_APP_BROKER_DELEGATION_TTL` | cache lifetime; > 0 and ≤ 60 s | `60s` |

Routes this task adds to P8-18's table (`task-18.md:118-127`):

| Route | Who | Answer |
|---|---|---|
| `GET /v1/sandboxes/{id}/control` | Fleet under a delegation | `101`, then frames both ways, copied, never decoded. Before the upgrade, as HTTP: `bad_request` 400 (no `Panacea-Control-Token`) · `forbidden` 403 (a desktop principal) · `sandbox_not_found` 404 (also another owner's or another project's) · `sandbox_not_running` 409 · `control_rejected` 403 (`sandboxd` said 401 or 403) · `backend_unavailable` 503 |
| `GET /v1/sandboxes/{id}/source` | Fleet under a delegation | `200` + `SourceAddr` · `forbidden` 403 (desktop) · `sandbox_not_found` 404 · `sandbox_not_running` 409 (not yet ready: no annotation) |

Both are `GET`s that change nothing, so P8-17's `TestNoRouteIsASideEffectingGet` (`task-17.md:313`)
gains them in its read list. The four existing sandbox routes (create, list, get, delete) keep their
answers; under a delegation they add `delegation_required` 401, `delegation_revoked` 401,
`delegation_expired` 401, `delegation_unverifiable` 503 and, on create only,
`delegation_class_mismatch` 403. Attach under a delegation is decision 8's.

**Gap S17-3 — the sandbox git route postdates this part.** P8-18's table now holds
`POST /v1/sandboxes/{id}/git/v1/{op}` (env R4 ruling 1, `task-18.md:127`). Its handler answers
`403 forbidden` when `principalOf` returns `Via == "fleet"`, as the machine handlers outside decision
5's three do, until task 20's *Gap S20-3* decides how a cloud worker's lane comes back. Step 1 gains
the row `TestFleetMayNotUseSandboxGit` (a delegated `POST …/git/v1/fetch` is 403 and the fake
`sandboxd` sees nothing; kills a git handler that skips the `Via` check). Failing closed costs nothing
TM1-α needs.

Consumes: P8-17's `newServer`, `serverDeps`, `ownerOf`; P8-18's katabe `Attach` path and
`proxy.RelayHooked` (`control-plane/internal/proxy/proxy.go:38`, built); `sandbox.OwnerLabelValue`
(`control-plane/internal/sandbox/manager.go:141`, built on `feat/env`); task 15's `Spec.Class`; ST.3's
header name. Consumed by: task 19 (opens `control`), task 20 (creates, reads `source`), task 39
(attach under a delegation), task 44 (revocation end to end).
