### Task 12: `cmd/fleetd` — config, Entra per request, role-to-verb, singleton loops

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** 10 (Fleet's `Store` and its `pgx`
pool), 11 (the outbox `Deliverer`, the delegation `Feed`, `Rebuild`), P8-16 (`internal/entra`,
**built** on `feat/env` at `1772f25`), and — added by fix batch b02, review S02-C2 — **01** (the
contract §7.28 and row 88), **06** (route 88 on the Hub) and **08**
(`hubclient.Team.Principal`). Those three are what `Membership` reads; before b02 they were a gap
this part raised and no task owned, so `go run ./cmd/fleetd` could not build. **Seams:** none defined; it hosts ST.1's control streams
(task 19) and every Fleet route that follows.

**Fleet is a network service, so nothing here is phase 2's loopback chassis** (§7.4, global
constraint 11). Phase 2's `cmd/api` trusts loopback and an Origin allowlist; Fleet listens inside
the cluster behind the ingress, and every request is authorised on its own: an Entra access token
whose audience is Fleet's API, whose `azp` is the desktop's client id, and whose `(tid, oid)` is
the principal (`05-design.md` §2.4 item 1). The validator is phase 8's `internal/entra`, reused
as built (`entra.go:23-62`, `entra.New` at `:119`, `Middleware` and `ClaimsFrom` at
`middleware.go:21,62` on `feat/env`); this task writes no JWT code.

**Who you are comes from the token; what you may do comes from the Hub.** Fleet stores no
membership (it is a Hub record, global constraint 1). `authz.go` asks a `Membership` for the
principal's Hub user id, their role on the project's team and their org flags, caches the answer for
at most 60 seconds (§2.4 item 4's freshness bound, and the skeleton's "cached ≤ 60 s"), and checks
the route's `Verb` against §3.4's table as data, per request. A viewer's launch is 403 from the
table, not from a handler's `if`. "Own only" and "parties only" are object checks the verb's
handler makes against the principal's user id; the table says which verbs carry one, so a handler
cannot forget that it owes one.

**Two replicas, one of each loop.** Fleet runs two stateless replicas (task 32), and three loops
must run exactly once across them: outbox delivery (task 11), the escalation deadline sweeper
(task 30) and the feed's acting role (task 11). Each loop holds a session-level Postgres advisory
lock on a **dedicated** connection, taken with `pg_try_advisory_lock(hashtext('panacea-app-fleet-<name>'))`,
the Hub's own pattern for its janitor (`hub-contract.md:2232`). A replica that fails to take the lock
retries every 5 s; a replica that loses its connection loses the lock with it, cancels the loop and
goes back to trying. The feed's *reading* role runs on every replica (task 11: each keeps its
cache current), and only its acting role is a singleton; on acquiring `fleet-feed` the replica
calls `Feed.Reset` then `Rebuild`, which is how a leader change cannot lose a revocation.

**What each replica reads is shared, not what it happened to serve.** A launch is adopted on
whichever replica served the request, which is neither loop holder more often than by chance. So
`readFeed` calls `Feed.Refresh` at the top of every tick: it reads task 10's `DelegationsInUse`,
the one state both replicas see, and learns the orgs and delegations this replica never adopted
(`task-11.md`, Refresh). The same `Feed` is the outbox `Client`'s `Source`, so a write of a
launch the other replica served resolves its delegation instead of dead-lettering. Without both,
the non-serving replica dead-letters that launch's whole Hub trail and never sweeps its
revocation (review S02-C1).

**A bad value fails start with its name.** Every `PANACEA_APP_FLEET_*` variable is read once, in
`loadConfig(getenv)`, phase 2's shape (`cmd/api/config.go:165`, `parseDuration` at `:421`); a
missing required value or an unusable one is an error whose text begins with the variable's name,
and `main` exits 2 before opening a socket. No other file reads the environment.

**Readiness is Postgres and the Hub, liveness is the process.** `/healthz` answers 200 while the
process serves. `/readyz` answers 200 only when a Postgres ping and a Hub `GET /readyz`
(`hub-contract.md:563-564`: it requires nothing) both succeed within 2 s, and when this replica's
`Rebuild` has completed once; otherwise 503 with the failing dependency in `details.dependency`. A
replica that cannot reach the Hub is not ready, but it is alive, so the kubelet does not restart
it in a Hub outage (the outbox's pause, task 11, is the designed behaviour there).

**The Hub read that tells Fleet a principal's role. Owned since fix batch b02** (review S02-C2).
`hubapi.User` carries no `tid`/`oid` (`hubapi/types.go:95-99`), so Fleet cannot resolve an Entra
`(tid, oid)` to a Hub user and their team role on its own. This part raised that as a skeleton gap
and named the read it needed; b02 made it a route nobody has to invent:

- **task 01** writes it — contract **§7.28**, **row 88**, `GET /v1/principals?tid=&oid=&projectId=`,
  Fleet with **no delegation** (§5.10 rule 2's second and last exemption), metadata only: user id,
  the **project's** `orgId`, team role, `orgAdmin`, `auditor`, `compliance`;
- **task 06** builds it on the Hub, in one store statement that resolves the project first;
- **task 08** gives it a client method, `hubclient.Team.Principal(ctx, tid, oid, projectID)`, and
  **three** conformance rows (`task-08.md:498-500`; b02's other two are the broker's, `:501-502`,
  for a total
  of five new rows and 38 altogether).

`hubMembership` is therefore written against a **real** interface, not a proposal, and `Member`
carries the project's `OrgID` from §7.28's answer. The `Membership` fake stays, for the unit tests
that must not reach a Hub. Two rules of §7.28 this part leans on, and must not re-derive: `orgId`
is the **project's** org, never the caller's; and the three org flags are that org's, so a user
with `Hub.Auditor` in another org arrives here with all three false and `role` `""`.

**Files:**
- `control-plane/cmd/fleetd/config.go` — `Config`, `loadConfig`, the variable table (new)
- `control-plane/cmd/fleetd/config_test.go`, `token_test.go` — one test per variable rule; the token source's cache, rotation and error hygiene (new)
- `control-plane/cmd/fleetd/main.go` — wiring, the loops, graceful shutdown (new)
- `control-plane/cmd/fleetd/token.go` — the workload-identity `hubclient.TokenSource` (new; not in the skeleton's file map, which lists only `main.go` and `config.go` — the Hub's token source has to live somewhere, and `main.go` is the wrong place to test it)
- `control-plane/internal/fleet/server/server.go` — `Server`, `Deps`, `New`, the mux, `/healthz`, `/readyz`, `Route` (new)
- `control-plane/internal/fleet/server/authz.go` — `Principal`, `Verb`, the §3.4 table, `Membership`, the 60 s cache, `authorize` (new)
- `control-plane/internal/fleet/server/singleton.go` — `Singleton`, the advisory lock on a dedicated connection (new; also not in the skeleton's map, for the same reason)
- `control-plane/internal/fleet/server/server_test.go`, `authz_test.go`, `singleton_test.go` — the tests below (new)

**Interfaces:**
- Produces:
  - `server.Deps`, `server.New(Deps) (*Server, error)`, `(*Server).Handler() http.Handler`, `(*Server).Route(pattern string, v Verb, h Handler)`, `server.Handler` = `func(http.ResponseWriter, *http.Request, Principal)`, `(*Server).MarkRebuilt()` — every later Fleet route is added through `Route` (tasks 14, 19, 20, 39, 52), so none can skip `authorize`
  - `server.Principal`, `server.Verb` and its constants, `server.Allowed(Verb, Principal) bool`, `server.OwnOnly(Verb) bool`, `server.Membership`, `server.Member`, `server.NewMembershipCache`, `server.MembershipTTL` (60 s)
  - `server.Singleton`, `server.NewSingleton(pool *pgxpool.Pool, name string, log *slog.Logger) *Singleton`, `(*Singleton).Run(ctx, onAcquire, loop func(context.Context) error) error`, `(*Singleton).Held() bool`
  - `fleetd` env: the table in `config.go` below, consumed by task 32's chart
- Consumes:
  - `feat/env` P8-16, built — `entra.Config`, `entra.New(cfg) (*JWKSValidator, error)`, `entra.Validator`, `entra.Claims{TenantID, ObjectID, ClientID, …}`, `entra.Middleware(v, next)`, `entra.ClaimsFrom(ctx)` (`internal/entra/entra.go:23-62,119`, `middleware.go:21,62`). **UNVERIFIED until merge** into `feat/team-mode`: the package is not on this branch today
  - phase 2, built — `httpx.WriteProblem(w, status, code, msg, details)` (`internal/httpx/problem.go:93`), `httpx.CodeInternal`
  - Hub, built — `hubapi.CodeUnauthenticated`, `hubapi.CodeForbidden` (`hubapi/codes.go:20-22`): Fleet answers auth failures with the Hub's two codes, so the desktop switches on one set; `hubapi.Role` (`types.go:20-25`); `hubclient.New(baseURL, scope, tokens, httpClient)` (`hubclient/client.go:21`, a stub until Hub tasks 33–34)
  - task 10 — `pgstore.Open(ctx, dsn string) (*pgstore.Store, error)` and `(*pgstore.Store).Pool() *pgxpool.Pool`, `pgstore.Migrate(ctx, pool) error`
  - task 11 — `outbox.New`, `(*Deliverer).RunOnce`, `(*Deliverer).Janitor`; `delegation.NewCache`, `NewClient(cache, src, now)`, `NewFeed(hub, team, in, cache, rev, leader, now)`, `delegation.StoreInUse`, `Rebuild(ctx, *Feed, *HubState)`, `(*Feed).Poll`, `Refresh`, `Sweep`, `Reset`, `Orgs`, `FeedKinds`, `PollEvery`
  - task 08 — `hubclient.Team.Principal(ctx, tid, oid, projectID string) (hubapi.Principal, error)` over `GET /v1/principals` (route 88, contract §7.28). **No longer proposed:** fix batch b02 gave it to tasks 01, 06 and 08, which this part now depends on. `hubapi.Principal` carries `UserID`, `OrgID` (the project's), `ProjectID`, `Role`, `OrgAdmin`, `Auditor`, `Compliance`

**`control-plane/cmd/fleetd/config.go`, complete:**

```go
package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Every variable fleetd reads. Nothing else in the process calls os.Getenv.
const (
	envAddr           = "PANACEA_APP_FLEET_ADDR"             // default ":8080"
	envPGDSN          = "PANACEA_APP_FLEET_PG_DSN"           // required; no password in it (below)
	envHubURL         = "PANACEA_APP_FLEET_HUB_URL"          // required, https unless DEV_INSECURE
	envHubScope       = "PANACEA_APP_FLEET_HUB_SCOPE"        // required, "<hub app id uri>/.default"
	envTokenURL       = "PANACEA_APP_FLEET_TOKEN_URL"        // required: Entra's v2 token endpoint, or devissuer's
	envClientID       = "PANACEA_APP_FLEET_CLIENT_ID"        // required: Fleet's own app (client) id
	envAssertionFile  = "PANACEA_APP_FLEET_ASSERTION_FILE"   // required: the projected workload-identity token
	envAuthority      = "PANACEA_APP_FLEET_ENTRA_AUTHORITY"  // default "https://login.microsoftonline.com"
	envTenants        = "PANACEA_APP_FLEET_ENTRA_TENANTS"    // required, comma-separated tenant GUIDs
	envAudience       = "PANACEA_APP_FLEET_AUDIENCE"         // required: Fleet's API app id URI
	envScope          = "PANACEA_APP_FLEET_REQUIRED_SCOPE"   // default "Fleet.Access"
	envDesktopClients = "PANACEA_APP_FLEET_DESKTOP_CLIENTS"  // required: the desktop's client id(s), the only azp
	envMembershipTTL  = "PANACEA_APP_FLEET_MEMBERSHIP_TTL"   // default 60s; at most 60s
	envOutboxTick     = "PANACEA_APP_FLEET_OUTBOX_TICK"      // default 500ms; 100ms..5s
	envLogLevel       = "PANACEA_APP_FLEET_LOG_LEVEL"        // debug|info|warn|error, default info
	envDevInsecure    = "PANACEA_APP_FLEET_DEV_INSECURE"     // "1" only in the offline stack: http Hub and authority
)

type Config struct {
	Addr           string
	PGDSN          string
	HubURL         string
	HubScope       string
	TokenURL       string
	ClientID       string
	AssertionFile  string
	Authority      string
	Tenants        []string
	Audience       string
	RequiredScope  string
	DesktopClients []string
	MembershipTTL  time.Duration
	OutboxTick     time.Duration
	LogLevel       string
	DevInsecure    bool
}

// loadConfig reads every variable once. Each error begins with the variable's
// name, so a failed start names what to fix.
func loadConfig(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:          or(getenv(envAddr), ":8080"),
		PGDSN:         getenv(envPGDSN),
		HubURL:        getenv(envHubURL),
		HubScope:      getenv(envHubScope),
		TokenURL:      getenv(envTokenURL),
		ClientID:      getenv(envClientID),
		AssertionFile: getenv(envAssertionFile),
		Authority:     or(getenv(envAuthority), "https://login.microsoftonline.com"),
		Audience:      getenv(envAudience),
		RequiredScope: or(getenv(envScope), "Fleet.Access"),
		LogLevel:      or(getenv(envLogLevel), "info"),
		DevInsecure:   getenv(envDevInsecure) == "1",
	}
	var errs []error // a slice, not a map, so the joined message has a stable order
	for _, kv := range [][2]string{{envPGDSN, c.PGDSN}, {envHubURL, c.HubURL},
		{envHubScope, c.HubScope}, {envTokenURL, c.TokenURL}, {envClientID, c.ClientID},
		{envAssertionFile, c.AssertionFile}, {envAudience, c.Audience}} {
		if strings.TrimSpace(kv[1]) == "" {
			errs = append(errs, fmt.Errorf("%s is required", kv[0]))
		}
	}
	c.Tenants = splitList(getenv(envTenants))
	if len(c.Tenants) == 0 {
		errs = append(errs, fmt.Errorf("%s is required: at least one tenant GUID", envTenants))
	}
	c.DesktopClients = splitList(getenv(envDesktopClients))
	if len(c.DesktopClients) == 0 {
		errs = append(errs, fmt.Errorf("%s is required: the desktop's client id is the only allowed azp", envDesktopClients))
	}
	for _, kv := range [][2]string{{envHubURL, c.HubURL}, {envTokenURL, c.TokenURL}, {envAuthority, c.Authority}} {
		name, raw := kv[0], kv[1]
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		switch {
		case err != nil || u.Host == "":
			errs = append(errs, fmt.Errorf("%s: not an absolute URL: %q", name, raw))
		case u.Scheme != "https" && !(c.DevInsecure && u.Scheme == "http"):
			errs = append(errs, fmt.Errorf("%s: must be https (http only with %s=1)", name, envDevInsecure))
		}
	}
	if !c.DevInsecure && dsnHasPassword(c.PGDSN) {
		errs = append(errs, fmt.Errorf("%s: carries a password; Fleet authenticates to Postgres with its workload identity (a password only with %s=1)", envPGDSN, envDevInsecure))
	}
	var err error
	if c.MembershipTTL, err = duration(getenv, envMembershipTTL, 60*time.Second, time.Second, 60*time.Second); err != nil {
		errs = append(errs, err)
	}
	if c.OutboxTick, err = duration(getenv, envOutboxTick, 500*time.Millisecond, 100*time.Millisecond, 5*time.Second); err != nil {
		errs = append(errs, err)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("%s: %q is not debug, info, warn or error", envLogLevel, c.LogLevel))
	}
	return c, errors.Join(errs...)
}

func duration(getenv func(string) string, name string, def, lo, hi time.Duration) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < lo || d > hi {
		return 0, fmt.Errorf("%s: %q is not a duration in [%s, %s]", name, raw, lo, hi)
	}
	return d, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// dsnHasPassword covers both DSN forms pgx accepts: a URL with user:pass@,
// and the keyword form with password=.
func dsnHasPassword(dsn string) bool {
	if u, err := url.Parse(dsn); err == nil && u.User != nil {
		if _, has := u.User.Password(); has {
			return true
		}
	}
	for _, f := range strings.Fields(dsn) {
		if strings.HasPrefix(strings.ToLower(f), "password=") {
			return true
		}
	}
	return false
}
```

**Why the ceiling on `MEMBERSHIP_TTL` is a start failure and not a clamp.** Sixty seconds is the
bound within which a removed member stops being able to act (§2.4 item 4). A clamp would let a
chart set 300 s, start cleanly and silently honour 60, which is correct today and wrong the day
someone "fixes" the clamp. Failing start makes the bound visible in the chart's review.

**Why the DSN may not carry a password.** Fleet's Postgres login is its workload identity (task
32), so a password in the DSN is either a leftover or a second credential nobody rotates. Which
Azure Database for PostgreSQL token flow `pgx` uses for it (a `BeforeConnect` that sets the
password to an Entra access token) is task 10's and task 32's, **UNVERIFIED** here.

**`control-plane/internal/fleet/server/authz.go`, complete:**

```go
package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/entra"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// Verb is one row of 05-design.md §3.4, as data. Every Fleet route names one.
type Verb string

const (
	VerbTeamRead      Verb = "team.read"          // the strip and task cards
	VerbLaunch        Verb = "sessions.launch"    // launch an Orchestrator or a worker; hold leases
	VerbSessionsOwn   Verb = "sessions.own"       // list, stop, attach to one's own sessions
	VerbThreadRead    Verb = "threads.read"       // a thread's messages
	VerbPrivateRead   Verb = "private.read"       // escalations, directives, human messages
	VerbNoteWrite     Verb = "notes.write"
	VerbTeamPolicy    Verb = "team_policy.write"  // quotas, budgets, more asking: within floors
	VerbRouteRaise    Verb = "route.raise"        // to AU
	VerbRouteLower    Verb = "route.lower"        // back to Global, audited
	VerbLeaseForce    Verb = "leases.release_stale"
	VerbOrgPolicy     Verb = "policy.org"         // OD-23 org layer, read and write (task 14)
	VerbProjectPolicy Verb = "policy.project"     // OD-23 project layer, read and write (task 14)
)

// grant is who holds a verb. own marks the §3.4 cells "own only" and
// "parties only": the handler must still check the object against UserID.
type grant struct {
	viewer, member, admin, auditor, compliance, orgAdmin bool
	own                                                  bool // applies to member and admin grants
}

// table is §3.4, row for row, plus task 14's two policy verbs. A verb
// missing here is refused for everyone, and TestEveryVerbHasARow fails.
var table = map[Verb]grant{
	VerbTeamRead:      {viewer: true, member: true, admin: true, auditor: true},
	VerbLaunch:        {member: true, admin: true, own: true},
	VerbSessionsOwn:   {member: true, admin: true, own: true},
	VerbThreadRead:    {member: true, admin: true, auditor: true, own: true},
	VerbPrivateRead:   {member: true, admin: true, auditor: true, own: true},
	VerbNoteWrite:     {member: true, admin: true},
	VerbTeamPolicy:    {admin: true},
	VerbRouteRaise:    {member: true, admin: true},
	VerbRouteLower:    {compliance: true},
	VerbLeaseForce:    {admin: true},
	VerbOrgPolicy:     {orgAdmin: true},
	VerbProjectPolicy: {admin: true},
}

// Member is what the Hub says about a principal, for one project.
type Member struct {
	UserID     string      // us-…; "" if the principal has never signed in to the Hub
	OrgID      string
	Role       hubapi.Role // on the project's team; "" if not a member, or no project
	OrgAdmin   bool
	Auditor    bool // org auditor: task 43 adds it to the Hub; false until then
	Compliance bool // org compliance: likewise
}

// Principal is the authorised caller of one request.
type Principal struct {
	TID, OID  string // from the token, never from a body
	ProjectID string // the route's {p}, "" for org routes
	Member
	// OwnOnly is true when the grant was member or admin on an own-only verb:
	// the handler must refuse an object whose owner is not UserID.
	OwnOnly bool
}

// Allowed applies the table, broadest grant first: an auditor who is also a
// member reads every thread, not only their own (§3.4's auditor column).
func Allowed(v Verb, p Principal) (ok, ownOnly bool) {
	g, found := table[v]
	if !found || p.UserID == "" {
		return false, false
	}
	switch {
	case g.orgAdmin && p.OrgAdmin, g.compliance && p.Compliance, g.auditor && p.Auditor:
		return true, false
	case g.admin && p.Role == hubapi.RoleAdmin, g.member && p.Role == hubapi.RoleMember:
		return true, g.own
	case g.viewer && p.Role == hubapi.RoleViewer:
		return true, false
	}
	return false, false
}

// OwnOnly reports whether v carries an object check for member and admin grants.
func OwnOnly(v Verb) bool { return table[v].own }

// Membership is the Hub's answer for (tid, oid) on a project. hubMembership
// (main.go) implements it over the proposed Team.Principal; tests fake it.
type Membership interface {
	Lookup(ctx context.Context, tid, oid, projectID string) (Member, error)
}

// MembershipTTL is the most a cached answer may be trusted (§2.4 item 4).
const MembershipTTL = 60 * time.Second

type memKey struct{ tid, oid, project string }

type memEntry struct {
	m  Member
	at time.Time
}

// MembershipCache caches answers, negative ones included, for ttl. Errors are
// never cached: a Hub blip must not become a minute of refusals.
type MembershipCache struct {
	next Membership
	ttl  time.Duration
	now  func() time.Time
	mu   sync.Mutex
	m    map[memKey]memEntry
}

func NewMembershipCache(next Membership, ttl time.Duration, now func() time.Time) *MembershipCache {
	if ttl <= 0 || ttl > MembershipTTL {
		ttl = MembershipTTL
	}
	return &MembershipCache{next: next, ttl: ttl, now: now, m: map[memKey]memEntry{}}
}

func (c *MembershipCache) Lookup(ctx context.Context, tid, oid, project string) (Member, error) {
	k := memKey{tid, oid, project}
	c.mu.Lock()
	e, ok := c.m[k]
	c.mu.Unlock()
	if ok && c.now().Sub(e.at) < c.ttl {
		return e.m, nil
	}
	m, err := c.next.Lookup(ctx, tid, oid, project)
	if err != nil {
		return Member{}, err
	}
	c.mu.Lock()
	c.m[k] = memEntry{m: m, at: c.now()}
	c.mu.Unlock()
	return m, nil
}

type principalKey struct{}

// PrincipalFrom is for code below a Route handler that was not handed p.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// authorize runs after entra.Middleware, which has already refused a bad
// token, a wrong audience, a missing scope and any azp but the desktop's.
func (s *Server) authorize(v Verb, h Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := entra.ClaimsFrom(r.Context())
		if !ok { // unreachable behind the middleware; refuse rather than trust
			httpx.WriteProblem(w, http.StatusUnauthorized, hubapi.CodeUnauthenticated, "Sign in again.", nil)
			return
		}
		project := r.PathValue("p")
		m, err := s.members.Lookup(r.Context(), c.TenantID, c.ObjectID, project)
		if err != nil {
			s.log.Warn("membership lookup failed", "err", err, "verb", v)
			httpx.WriteProblem(w, http.StatusServiceUnavailable, CodeUpstreamUnavailable,
				"The team service cannot reach the Hub right now.", map[string]string{"upstream": "hub"})
			return
		}
		p := Principal{TID: c.TenantID, OID: c.ObjectID, ProjectID: project, Member: m}
		if org := r.PathValue("org"); org != "" && org != m.OrgID {
			httpx.WriteProblem(w, http.StatusForbidden, hubapi.CodeForbidden,
				"That organisation is not yours.", map[string]string{"required": "org"})
			return
		}
		allowed, own := Allowed(v, p)
		if !allowed {
			httpx.WriteProblem(w, http.StatusForbidden, hubapi.CodeForbidden,
				"Your role on this project does not allow that.", map[string]string{"required": string(v)})
			return
		}
		p.OwnOnly = own
		h(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)), p)
	})
}
```

`CodeUpstreamUnavailable = "upstream_unavailable"` (503, `details.upstream`) is declared in
`server.go`: it is the string the Hub's relay already uses for the same condition (the
`ErrUnavailable` comment, `hubclient/hubclient.go:131-133`), so the desktop maps one code.

**`control-plane/internal/fleet/server/server.go`, complete:**

```go
// Package server is fleetd's HTTP surface: Entra on every request, §3.4's
// table on every route, and the two probes that need nothing.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/entra"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

const (
	CodeUpstreamUnavailable = "upstream_unavailable" // 503, details.upstream
	CodeNotReady            = "not_ready"            // 503, details.dependency
	CodeRouteNotFound       = "route_not_found"      // 404, the Hub's string (hubapi/codes.go:32)
	readyTimeout            = 2 * time.Second
)

// Handler is an authorised route's body; p is never the zero value.
type Handler func(w http.ResponseWriter, r *http.Request, p Principal)

// Pinger is a readiness dependency.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Validator entra.Validator // entra.New(cfg), audience Fleet, AllowedClients the desktop's
	Members   Membership      // already wrapped in NewMembershipCache
	Postgres  Pinger
	Hub       Pinger // GET {hub}/readyz
	Log       *slog.Logger
}

type Server struct {
	root    *http.ServeMux // the probes, and everything else behind Entra
	mux     *http.ServeMux // authorised routes only
	members Membership
	pg, hub Pinger
	log     *slog.Logger
	rebuilt atomic.Bool
}

func New(d Deps) (*Server, error) {
	if d.Validator == nil || d.Members == nil || d.Postgres == nil || d.Hub == nil || d.Log == nil {
		return nil, errors.New("server: Validator, Members, Postgres, Hub and Log are all required")
	}
	s := &Server{root: http.NewServeMux(), mux: http.NewServeMux(), members: d.Members,
		pg: d.Postgres, hub: d.Hub, log: d.Log}
	s.root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	s.root.HandleFunc("GET /readyz", s.readyz)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteProblem(w, http.StatusNotFound, CodeRouteNotFound, "No such route.", nil)
	})
	s.root.Handle("/", entra.Middleware(d.Validator, s.mux))
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.root }

// Route registers an authorised route. A verb with no §3.4 row is a
// programming error and panics at start, not a 403 in production.
func (s *Server) Route(pattern string, v Verb, h Handler) {
	if _, ok := table[v]; !ok {
		panic("server: route " + pattern + " names verb " + string(v) + ", which has no §3.4 row")
	}
	s.mux.Handle(pattern, s.authorize(v, h))
}

// MarkRebuilt is called once, after this replica's first Rebuild (task 11).
func (s *Server) MarkRebuilt() { s.rebuilt.Store(true) }

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	for _, dep := range []struct {
		name string
		p    Pinger
	}{{"postgres", s.pg}, {"hub", s.hub}} {
		if err := dep.p.Ping(ctx); err != nil {
			s.log.Warn("not ready", "dependency", dep.name, "err", err)
			httpx.WriteProblem(w, http.StatusServiceUnavailable, CodeNotReady,
				"Not ready.", map[string]string{"dependency": dep.name})
			return
		}
	}
	if !s.rebuilt.Load() {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, CodeNotReady,
			"Not ready.", map[string]string{"dependency": "rebuild"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}
```

**`/healthz` and `/readyz` are outside Entra, and nothing else is.** They are registered on the
root mux with methods; every other path, including an unknown one, goes through
`entra.Middleware` first, so a probe for a route's existence without a token is a 401, not a 404.

**`control-plane/internal/fleet/server/singleton.go`, complete:**

```go
package server

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Singleton runs one loop across every replica that shares a database. The
// lock is session-level, on a connection hijacked out of the pool, so the
// only ways to lose it are to close that connection or for Postgres to lose
// it — and either one ends the session, releases the lock, and (through the
// watchdog's ping) cancels the loop here.
type Singleton struct {
	pool  *pgxpool.Pool
	name  string // "outbox", "feed", "deadlines"
	log   *slog.Logger
	retry time.Duration
	check time.Duration
	held  atomic.Bool
}

func NewSingleton(pool *pgxpool.Pool, name string, log *slog.Logger) *Singleton {
	return &Singleton{pool: pool, name: name, log: log, retry: 5 * time.Second, check: 5 * time.Second}
}

// Held is task 11's leader() for the feed.
func (s *Singleton) Held() bool { return s.held.Load() }

// Run blocks until ctx ends. On each acquisition it calls onAcquire (nil is
// allowed), then loop with a context that is cancelled when the lock is lost.
func (s *Singleton) Run(ctx context.Context, onAcquire, loop func(context.Context) error) error {
	for {
		if err := s.once(ctx, onAcquire, loop); err != nil && ctx.Err() == nil {
			s.log.Warn("singleton released", "name", s.name, "err", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.retry):
		}
	}
}

func (s *Singleton) once(ctx context.Context, onAcquire, loop func(context.Context) error) error {
	pc, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	conn := pc.Hijack() // ours alone now; closing it is the unlock
	defer conn.Close(context.Background())
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`,
		"panacea-app-fleet-"+s.name).Scan(&got); err != nil || !got {
		return err // not got: another replica holds it; try again after retry
	}
	lctx, cancel := context.WithCancel(ctx)
	watchdog := make(chan struct{})
	go func() { // only this goroutine touches conn while the loop runs
		defer close(watchdog)
		t := time.NewTicker(s.check)
		defer t.Stop()
		for {
			select {
			case <-lctx.Done():
				return
			case <-t.C:
				if err := conn.Ping(lctx); err != nil {
					s.log.Warn("singleton connection lost", "name", s.name, "err", err)
					cancel()
					return
				}
			}
		}
	}()
	s.held.Store(true)
	s.log.Info("singleton acquired", "name", s.name)
	defer func() {
		s.held.Store(false) // before the lock goes: never "held" without the lock
		cancel()
		<-watchdog // then conn.Close runs with no concurrent Ping
	}()
	if onAcquire != nil {
		if err := onAcquire(lctx); err != nil {
			return err
		}
	}
	return loop(lctx)
}
```

**`held` goes false before the lock is released, never after.** The feed's acting role asks
`Held()` before each Revoker call (task 11). If the flag outlived the lock, a replica that had just
lost it could act in the same instant as the one that had just taken it. Clearing the flag first
leaves the opposite window, a moment with no actor, which costs at most one poll of delay.

**The pgx dependency.** `github.com/jackc/pgx/v5` is task 10's addition to `control-plane/go.mod`
(no branch pins it on 2026-09-25: checked `go.mod` on `feat/team-mode`, `feat/hub` and `feat/env`);
this task adds no module. `pgxpool.Conn.Hijack` is in `pgx/v5`'s `pgxpool` (**UNVERIFIED** at the
version task 10 pins; it has been there since v5.0).

**`control-plane/cmd/fleetd/token.go`, complete.** Fleet's own token for the Hub, and the only
credential it holds: an app-only token by the client-credentials grant with a **client
assertion**, the projected service-account token Azure Workload Identity mounts into the pod. No
secret is configured; the assertion file is re-read on every fetch because the kubelet rotates it.
In the offline stack the same code calls `devissuer`'s client-credentials grant (task 05), which
accepts the same form.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// assertionSource is hubclient.TokenSource and hubclient.Invalidator.
type assertionSource struct {
	tokenURL, clientID, assertionFile string
	http                              *http.Client
	now                               func() time.Time

	mu    sync.Mutex
	cache map[string]cachedToken // scope -> token
}

type cachedToken struct {
	value string
	exp   time.Time
}

// refreshBefore keeps a token from being used in its last five minutes.
const refreshBefore = 5 * time.Minute

func newAssertionSource(c Config, hc *http.Client) *assertionSource {
	return &assertionSource{tokenURL: c.TokenURL, clientID: c.ClientID,
		assertionFile: c.AssertionFile, http: hc, now: time.Now, cache: map[string]cachedToken{}}
}

func (a *assertionSource) Token(ctx context.Context, scope string) (string, error) {
	a.mu.Lock()
	t, ok := a.cache[scope]
	a.mu.Unlock()
	if ok && a.now().Add(refreshBefore).Before(t.exp) {
		return t.value, nil
	}
	assertion, err := os.ReadFile(a.assertionFile)
	if err != nil {
		return "", fmt.Errorf("fleetd: reading the workload-identity assertion: %w", err)
	}
	form := url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {a.clientID},
		"scope":                 {scope},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {strings.TrimSpace(string(assertion))},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fleetd: token endpoint: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("fleetd: token endpoint answered %d, unreadable: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || body.AccessToken == "" || body.ExpiresIn <= 0 {
		// body.Error is an OAuth code ("invalid_client"), never a secret; the
		// assertion and the response body are never logged.
		return "", fmt.Errorf("fleetd: token endpoint answered %d %q", resp.StatusCode, body.Error)
	}
	t = cachedToken{value: body.AccessToken, exp: a.now().Add(time.Duration(body.ExpiresIn) * time.Second)}
	a.mu.Lock()
	a.cache[scope] = t
	a.mu.Unlock()
	return t.value, nil
}

// Invalidate drops a cached token; hubclient calls it before its one retry on
// a 401 "expired" (hubclient/hubclient.go:23-30).
func (a *assertionSource) Invalidate(scope string) {
	a.mu.Lock()
	delete(a.cache, scope)
	a.mu.Unlock()
}
```

**`control-plane/cmd/fleetd/main.go`, complete.** Later tasks add exactly two kinds of line here,
and nothing else: a `register…(srv, …)` call for their routes (tasks 14, 19, 20, 39, 52), and an
entry in `revokers` or `kinds` (tasks 20, 22–25, 39). Task 30 adds the `deadlines` singleton beside
`outbox`.

```go
// Command fleetd is Fleet: team mode's coordinator, beside the Hub.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/entra"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/delegation"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/outbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/server"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/store/pgstore"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

func main() { os.Exit(run(context.Background(), os.Getenv, os.Stderr)) }

func run(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "fleetd: configuration:", err)
		return 2
	}
	log := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level(cfg.LogLevel)}))
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := pgstore.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Error("postgres", "err", err)
		return 1
	}
	defer st.Close()
	if err := pgstore.Migrate(ctx, st.Pool()); err != nil {
		log.Error("migrate", "err", err)
		return 1
	}

	hc := &http.Client{Timeout: 10 * time.Second}
	tokens := newAssertionSource(cfg, hc)
	hub := hubclient.New(cfg.HubURL, cfg.HubScope, tokens, hc)
	validator, err := entra.New(entra.Config{AuthorityHost: cfg.Authority, TenantIDs: cfg.Tenants,
		Audience: cfg.Audience, RequiredScope: cfg.RequiredScope, AllowedClients: cfg.DesktopClients,
		AllowInsecureAuthority: cfg.DevInsecure})
	if err != nil {
		log.Error("entra", "err", err)
		return 1
	}
	srv, err := server.New(server.Deps{
		Validator: validator,
		Members:   server.NewMembershipCache(hubMembership{hub}, cfg.MembershipTTL, time.Now),
		Postgres:  pgPinger{st.Pool()},
		Hub:       hubReady{url: cfg.HubURL + "/readyz", http: hc},
		Log:       log,
	})
	if err != nil {
		log.Error("server", "err", err)
		return 1
	}

	cache := delegation.NewCache()
	feedLock := server.NewSingleton(st.Pool(), "feed", log)
	outboxLock := server.NewSingleton(st.Pool(), "outbox", log)
	revokers := &fanout{log: log} // tasks 20, 23, 39 add theirs: revokers.add(r)
	// The feed is built before the outbox because it is the outbox's Source:
	// a launch adopted on the other replica is resolved from Fleet's own
	// store rather than waited a poll out (task-11.md, Refresh and Resolve).
	feed := delegation.NewFeed(hub, hub, delegation.StoreInUse{S: st}, cache, revokers,
		feedLock.Held, time.Now)
	deliver := outbox.New(outbox.StoreQueue{S: st},
		delegation.Sender{Client: delegation.NewClient(cache, feed, time.Now), Hub: hub}, time.Now)
	state := delegation.NewHubState()
	kinds := slices.Clone(delegation.FeedKinds) // tasks 22-25 append their kinds
	rebuild := func(c context.Context) error { return delegation.Rebuild(c, feed, state) }

	go readFeed(ctx, log, srv, feed, rebuild, kinds)
	go feedLock.Run(ctx, func(c context.Context) error { feed.Reset(); return rebuild(c) },
		func(c context.Context) error { <-c.Done(); return nil }) // the lock is the role
	go outboxLock.Run(ctx, nil, func(c context.Context) error { return deliverLoop(c, deliver, cfg.OutboxTick) })

	return serve(ctx, log, cfg.Addr, srv.Handler())
}

// readFeed is the feed's reading role, on every replica: rebuild until it
// succeeds once, mark ready, then refresh the tracked orgs from Fleet's own
// store, poll every one of them, and sweep. Refresh comes first every tick
// because Orgs would otherwise hold only the launches this replica served.
func readFeed(ctx context.Context, log *slog.Logger, srv *server.Server, feed *delegation.Feed,
	rebuild func(context.Context) error, kinds []string) {
	for {
		if err := rebuild(ctx); err == nil {
			srv.MarkRebuilt()
			break
		} else if ctx.Err() != nil {
			return
		} else {
			log.Warn("rebuild failed; retrying", "err", err)
		}
		if !sleep(ctx, 5*time.Second) {
			return
		}
	}
	for sleep(ctx, delegation.PollEvery) {
		if err := feed.Refresh(ctx); err != nil {
			log.Warn("feed refresh", "err", err) // the known orgs are still polled
		}
		for _, org := range feed.Orgs() {
			for more := true; more; {
				var err error
				if more, err = feed.Poll(ctx, org, kinds); err != nil {
					log.Warn("feed poll", "org", org, "err", err)
					break
				}
			}
		}
		if err := feed.Sweep(ctx); err != nil {
			log.Warn("feed sweep", "err", err)
		}
	}
}

func deliverLoop(ctx context.Context, d *outbox.Deliverer, every time.Duration) error {
	tick, jan := time.NewTicker(every), time.NewTicker(time.Hour)
	defer tick.Stop()
	defer jan.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if _, err := d.RunOnce(ctx); err != nil {
				return err // the Singleton logs it, releases the lock, and retries
			}
		case <-jan.C:
			if _, err := d.Janitor(ctx); err != nil {
				return err
			}
		}
	}
}

func serve(ctx context.Context, log *slog.Logger, addr string, h http.Handler) int {
	// No WriteTimeout: tasks 19 and 39 serve long-lived WebSockets on this server.
	hs := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	log.Info("fleetd listening", "addr", addr)
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error("listen", "err", err)
		return 1
	}
	return 0
}

// hubMembership is server.Membership over route 88, contract §7.28 (fix batch
// b02: tasks 01, 06 and 08 own it, so this is no longer a proposal). p.OrgID is
// the PROJECT's org, never this caller's, and Member carries it unchanged: every
// org-level grant in authorize is bound to it.
type hubMembership struct{ hub hubclient.Team }

func (m hubMembership) Lookup(ctx context.Context, tid, oid, project string) (server.Member, error) {
	p, err := m.hub.Principal(ctx, tid, oid, project)
	if hubclient.Code(err) == "principal_not_found" {
		return server.Member{}, nil // unknown to the Hub: UserID "" is refused by Allowed
	}
	if err != nil {
		return server.Member{}, err
	}
	return server.Member{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role, OrgAdmin: p.OrgAdmin,
		Auditor: p.Auditor, Compliance: p.Compliance}, nil
}

type hubReady struct {
	url  string
	http *http.Client
}

func (h hubReady) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub /readyz answered %d", resp.StatusCode)
	}
	return nil
}

type pgPinger struct{ pool *pgxpool.Pool }

func (p pgPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// fanout is the one delegation.Revoker Fleet has; every effect joins it.
type fanout struct {
	log  *slog.Logger
	list []delegation.Revoker
}

func (f *fanout) add(r delegation.Revoker) { f.list = append(f.list, r) }

func (f *fanout) Revoke(ctx context.Context, d delegation.Delegation, why delegation.Reason) error {
	f.log.Info("delegation act", "delegation", d.ID, "reason", why, "effects", len(f.list))
	var errs []error
	for _, r := range f.list { // every effect is tried, whichever fails
		errs = append(errs, r.Revoke(ctx, d, why))
	}
	return errors.Join(errs...)
}

func (f *fanout) Resume(ctx context.Context, d delegation.Delegation) error {
	var errs []error
	for _, r := range f.list {
		errs = append(errs, r.Resume(ctx, d))
	}
	return errors.Join(errs...)
}

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func level(s string) slog.Level {
	var l slog.Level
	_ = l.UnmarshalText([]byte(s)) // loadConfig has already refused anything else
	return l
}
```

`principal_not_found` is the code the proposed Hub read answers for a `(tid, oid)` it has never
provisioned; it is part of the gap, and task 08's conformance row for the route pins it.

- [ ] **Step 1: Write the failing tests**

Package-internal tests throughout. `server` tests use a `fakeValidator` (returns fixed
`entra.Claims`, or an error wrapping one of `entra`'s sentinels), a `fakeMembers` (a map, plus a
call counter and an injectable error), and `fakePinger`s. `singleton_test.go` needs a real
Postgres: it reads `PANACEA_APP_FLEET_TEST_PG_DSN`, the variable task 10's `pgstore` tests use, and
`t.Skip`s without it, printing that it skipped — **a skip is not a pass**, and step 5 runs it with
the DSN set.

| File · test | What it proves | The mutant it kills |
|---|---|---|
| config · `TestEveryRequiredVariableIsNamedWhenMissing` | empty env → one error naming each of the nine required variables | a required variable given a silent default |
| config · `TestMembershipTTLOverSixtyFailsStart` | `61s` → an error beginning `PANACEA_APP_FLEET_MEMBERSHIP_TTL` | a clamp instead of a refusal |
| config · `TestHTTPIsRefusedUnlessDevInsecure` | `http://hub` → error; with `DEV_INSECURE=1` → accepted | the scheme check dropped |
| config · `TestAPasswordInTheDSNIsRefused` | `postgres://u:p@h/db` and `host=h password=p` → both refused; with `DEV_INSECURE=1` → accepted | only one DSN form checked |
| config · `TestRunExitsTwoBeforeListening` | `run` with an empty env returns 2 and `net.Listen` on the default addr still succeeds | a config error logged and start continued |
| token · `TestTheAssertionIsReReadOnEveryFetch` | rotate the file between two fetches (after `Invalidate`): the second form carries the new assertion | the assertion read once at start, which fails an hour later |
| token · `TestATokenIsReusedUntilFiveMinutesBeforeExpiry` | `expires_in: 3600`: fetches at 0 and 54 min → one POST; at 56 min → two | no cache (a token request per Hub call) or no margin |
| token · `TestTheAssertionNeverAppearsInAnError` | the endpoint answers 400 echoing the form; `err.Error()` does not contain the assertion | an error that quotes the response body |
| authz · `TestEveryVerbHasARow` | every `Verb` constant (listed in the test) is a key of `table` | a new verb refused for everyone by omission |
| authz · `TestTheTableIsSection34` | below: 12 verbs × 7 principals, each cell `(ok, ownOnly)` | any cell of §3.4 flipped |
| authz · `TestAViewersLaunchIs403` | a viewer's `POST` to a route registered with `VerbLaunch` → 403 `forbidden`, `details.required = "sessions.launch"`, and the handler never ran | `Route` registering the bare handler |
| authz · `TestATokenForAnotherAudienceIs401` | the validator returns `entra.ErrAudience` → 401 from `entra.Middleware`, the handler never ran, `fakeMembers` never called | a route mounted on the root mux, outside `Middleware` |
| authz · `TestMembershipIsCachedAtMostSixtySeconds` | lookups at 0 s and 59 s → one Hub call; at 60 s → two; `NewMembershipCache(…, 5*time.Minute, …)` still expires at 60 s | a TTL above the §2.4 bound |
| authz · `TestMembershipErrorsAreNotCached` | an error, then a success 1 s later → two calls | a Hub blip cached as a refusal |
| authz · `TestHubDownIs503NotForbidden` | lookup error → 503 `upstream_unavailable`, `details.upstream = "hub"` | a lookup failure answered as "not a member", which tells a member they were removed |
| authz · `TestTheOrgInThePathMustBeTheCallers` | `GET /v1/orgs/{org}/…` with another org → 403 `required: "org"` | an org admin of org A reading org B's policy |
| server · `TestHealthzNeedsNoTokenAndNothingElseDoes` | `/healthz` 200 with no token; `/readyz` 503 or 200 with no token; `GET /nope` with no token → 401 | the probes behind Entra, or unknown routes outside it |
| server · `TestReadyzNamesTheFailingDependency` | Postgres down → `postgres`; Hub down → `hub`; both up, no `MarkRebuilt` → `rebuild`; then 200 | ready before the first rebuild, when the cache is empty and every delegated write would fail |
| server · `TestARouteWithAnUnknownVerbPanicsAtStart` | `Route("GET /x", "made.up", h)` panics | a route silently refused in production |
| singleton · `TestTwoReplicasRunOneFeedConsumer` | below | a transaction-level lock, or a lock taken on a pooled connection that goes back to the pool |
| singleton · `TestLosingTheConnectionHandsOver` | `pg_terminate_backend` of the holder's pid: the holder's loop context is cancelled within `check`, and the other replica acquires within `retry + check` | no watchdog: the old holder runs on without the lock |

The matrix, complete, because it *is* §3.4 and a reviewer should be able to hold the two side by
side. `own` is the second value; `-` is refused.

```go
func TestTheTableIsSection34(t *testing.T) {
	type cell struct{ ok, own bool }
	Y, O, N := cell{true, false}, cell{true, true}, cell{false, false}
	principals := []struct {
		name string
		p    Principal
	}{
		{"viewer", Principal{Member: Member{UserID: "us-v", Role: hubapi.RoleViewer}}},
		{"member", Principal{Member: Member{UserID: "us-m", Role: hubapi.RoleMember}}},
		{"admin", Principal{Member: Member{UserID: "us-a", Role: hubapi.RoleAdmin}}},
		{"auditor", Principal{Member: Member{UserID: "us-au", Auditor: true}}},
		{"compliance", Principal{Member: Member{UserID: "us-c", Compliance: true}}},
		{"orgadmin", Principal{Member: Member{UserID: "us-o", OrgAdmin: true}}},
		{"nobody", Principal{Member: Member{UserID: "us-n"}}},
	}
	//                          viewer member admin auditor compliance orgadmin nobody
	want := map[Verb][7]cell{
		VerbTeamRead:      {Y, Y, Y, Y, N, N, N},
		VerbLaunch:        {N, O, O, N, N, N, N},
		VerbSessionsOwn:   {N, O, O, N, N, N, N},
		VerbThreadRead:    {N, O, O, Y, N, N, N},
		VerbPrivateRead:   {N, O, O, Y, N, N, N},
		VerbNoteWrite:     {N, Y, Y, N, N, N, N},
		VerbTeamPolicy:    {N, N, Y, N, N, N, N},
		VerbRouteRaise:    {N, Y, Y, N, N, N, N},
		VerbRouteLower:    {N, N, N, N, Y, N, N},
		VerbLeaseForce:    {N, N, Y, N, N, N, N},
		VerbOrgPolicy:     {N, N, N, N, N, Y, N},
		VerbProjectPolicy: {N, N, Y, N, N, N, N},
	}
	if len(want) != len(table) {
		t.Fatalf("table has %d verbs, the test %d: a verb was added without its §3.4 row", len(table), len(want))
	}
	for v, row := range want {
		for i, pr := range principals {
			ok, own := Allowed(v, pr.p)
			if got := (cell{ok, own}); got != row[i] {
				t.Errorf("Allowed(%s, %s) = %+v; §3.4 says %+v", v, pr.name, got, row[i])
			}
		}
	}
	// An auditor who is also a member reads every thread, not only their own.
	both := Principal{Member: Member{UserID: "us-b", Role: hubapi.RoleMember, Auditor: true}}
	if ok, own := Allowed(VerbThreadRead, both); !ok || own {
		t.Errorf("auditor+member on threads.read = (%v, %v); want (true, false)", ok, own)
	}
	// A principal the Hub has never provisioned holds nothing, whatever else is set.
	if ok, _ := Allowed(VerbTeamRead, Principal{Member: Member{Role: hubapi.RoleAdmin}}); ok {
		t.Error("a principal with no Hub user id was allowed")
	}
}
```

Where §3.4 says "within floors" (team policy) or "audited" (lowering the route, releasing a stale
lease), the table grants the verb and the handler (tasks 43, 56, 35) owes the floor check or the
audit event; the matrix cannot see either, and does not pretend to.

The two-replica test, complete:

```go
func TestTwoReplicasRunOneFeedConsumer(t *testing.T) {
	dsn := os.Getenv("PANACEA_APP_FLEET_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("PANACEA_APP_FLEET_TEST_PG_DSN unset: the advisory lock needs a real Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var running, maxRunning, runs atomic.Int32
	loop := func(c context.Context) error {
		n := running.Add(1)
		defer running.Add(-1)
		for m := maxRunning.Load(); n > m && !maxRunning.CompareAndSwap(m, n); m = maxRunning.Load() {
		}
		runs.Add(1)
		<-c.Done()
		return nil
	}
	var singles []*Singleton
	for i := 0; i < 2; i++ { // two pools: two replicas, never one pool shared
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		s := NewSingleton(pool, "feed-test-"+t.Name(), slog.New(slog.NewTextHandler(io.Discard, nil)))
		s.retry, s.check = 100*time.Millisecond, 100*time.Millisecond
		singles = append(singles, s)
		go func() { _ = s.Run(ctx, nil, loop) }()
	}
	deadline := time.Now().Add(3 * time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(1 * time.Second) // ten retry periods for the second to (wrongly) get in
	if got := maxRunning.Load(); got != 1 {
		t.Fatalf("%d loops ran at once across two replicas; want exactly 1", got)
	}
	if singles[0].Held() == singles[1].Held() {
		t.Fatalf("Held() = %v, %v; exactly one replica must hold the lock", singles[0].Held(), singles[1].Held())
	}
}
```

This is the one test in the task that sleeps, and it sleeps on a real database's lock, which has no
fake clock; the lock name carries `t.Name()` so a parallel run of another test cannot collide.

Run: `cd control-plane && go test ./cmd/fleetd/ ./internal/fleet/server/ -count=1`. Expected
before step 2: compile failures naming `loadConfig`, `newAssertionSource`, `New`, `Allowed`,
`NewSingleton`.

- [ ] **Step 2: Write `server.go`, `authz.go`, `singleton.go`** as above. Run
  `go test ./internal/fleet/server/ -count=1`. Expected: `ok`, with `TestTwoReplicasRunOneFeedConsumer`
  and `TestLosingTheConnectionHandsOver` reported as skipped when the DSN is unset (`-v` shows
  `--- SKIP`). Commit: `feat(fleet): fleetd's server: Entra per request, §3.4 as data, singletons`.

- [ ] **Step 3: Write `config.go` and `token.go`** as above. Run `go test ./cmd/fleetd/ -run
  'Config|Token|RunExits' -count=1`. Expected: `ok`. Commit:
  `feat(fleetd): config read once, the workload-identity token source`.

- [ ] **Step 4: Write `main.go`** as above. It compiles only once tasks 10 and 11 are merged
  (`pgstore`, `outbox`, `delegation`) and task 08's `hubclient.Team` carries `Principal` — which
  since fix batch b02 is task 08's own acceptance item 5, not a gap to raise. If `Principal` is
  missing, the fault is in task 08 and the fix belongs there; **never stub it here**, because a
  stubbed `Membership` is an authorisation bypass wearing a TODO. Run `go build ./cmd/fleetd/`.
  Expected: no output. Commit: `feat(fleetd): wiring, the feed reader, the outbox and feed
  singletons`.

- [ ] **Step 5: Acceptance**

```
cd control-plane
go vet ./cmd/fleetd/ ./internal/fleet/server/
PANACEA_APP_FLEET_TEST_PG_DSN="$PANACEA_APP_FLEET_TEST_PG_DSN" go test ./cmd/fleetd/ ./internal/fleet/server/ -count=1 -race -v 2>&1 | grep -E "^(--- FAIL|--- SKIP|ok|FAIL)"
grep -rn "os.Getenv\|os.LookupEnv" cmd/fleetd internal/fleet | grep -v _test.go
env -i PATH="$PATH" go run ./cmd/fleetd; echo "exit=$?"
```

Expected: `go vet` silent; two `ok` lines and **no** `--- SKIP` (the DSN is set: the offline
stack's Postgres, or task 10's test database); the `grep` prints exactly one line, `main.go`'s
`os.Getenv` passed to `run`; the last command prints `fleetd: configuration:` and nine errors,
one per line, each beginning with the variable it names, then `exit=2`.

The two-replica property on the real cluster (two pods, one outbox loop, a pod kill hands over) is
**deferred to task 33**, which runs it; this task proves it on one database with two pools.

**What this task does not claim.**
- The Hub read that resolves a principal's role does not exist yet (task 01's table has no such
  route). Until tasks 01 and 08 add it, `fleetd` cannot authorise a real request: the unit tests
  pass on a fake `Membership`, and `main.go` does not build. This is the task's largest gap.
- *auditor* and *compliance* are carried as flags but always false until task 43 makes them Hub
  roles. A route that grants a verb only to them is unreachable until then, which is the safe
  direction.
- No route but the probes exists after this task; each later task adds its own through `Route`,
  and each joins the Z5 matrix there (global constraints).
- The `Revoker` fan-out is empty here, so a revocation or a lapse is logged and acted on by
  nothing until tasks 20, 23 and 39 add their effects. The 60-second property is task 11's to
  prove at the `Revoker` boundary and task 44's to prove end to end.
- It does not rate-limit, and it does not set a body limit: per-route body limits are the routes'
  (phase 2's `httpx` decode, 1 MiB), and a Fleet rate limit is not in the design.
- Which Azure Postgres token flow authenticates Fleet's DSN is **UNVERIFIED** (task 10, task 32).

