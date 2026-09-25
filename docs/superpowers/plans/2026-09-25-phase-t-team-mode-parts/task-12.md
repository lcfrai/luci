### Task 12: `cmd/fleetd` — config, Entra per request, role-to-verb, singleton loops

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** 10 (Fleet's `Store` and its `pgx`
pool), 11 (the outbox `Deliverer`, the delegation `Feed`, `Rebuild`), P8-16 (`internal/entra`,
**built** on `feat/env` at `1772f25`). **Seams:** none defined; it hosts ST.1's control streams
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

**The Hub has no route yet that tells Fleet a principal's role.** Task 01's team table
(`task-01.md:226-261`) gives Fleet the per-org feed and delegated reads, but no way to resolve an
Entra `(tid, oid)` to a Hub user and their team role, and `hubapi.User` carries no `tid`/`oid`
(`hubapi/types.go:95-99`). This part therefore consumes a `Membership` interface and states the
Hub read it needs, `GET /v1/orgs/{orgId}/principals?tid=&oid=&projectId=` (Fleet, **no
delegation**, metadata only: user id, team role, `orgAdmin`, `auditor`, `compliance`), as a
**skeleton gap** for task 01 and 08 to add. Until it lands, `hubMembership` is written against the
proposed `hubclient.Team.Principal` method and the tests run on a fake `Membership`.

**Files:**
- `control-plane/cmd/fleetd/config.go` — `Config`, `loadConfig`, the variable table (new)
- `control-plane/cmd/fleetd/config_test.go` — one test per variable rule (new)
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
  - task 11 — `outbox.New`, `(*Deliverer).RunOnce`, `(*Deliverer).Janitor`; `delegation.NewCache`, `NewClient`, `NewFeed`, `Rebuild`, `(*Feed).Poll`, `Sweep`, `Reset`, `Orgs`, `FeedKinds`, `PollEvery`
  - task 08 (proposed, see the gap above) — `hubclient.Team.Principal(ctx, orgID, tid, oid, projectID string) (hubapi.Principal, error)` over `GET /v1/orgs/{orgId}/principals`

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
