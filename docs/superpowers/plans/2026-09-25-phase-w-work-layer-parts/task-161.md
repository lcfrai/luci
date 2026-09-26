### Task 161: the Work MCP server on the Hub behind the realm's OAuth, with read-only mode; the MCP connector allow-list

W5, stream WH, about 3 hours (an estimate, not a measurement). Depends on task 159 (apps: the
`App` record, its `entraClientId`, its `work.read`/`work.write` scopes and the admin decision) and
task 160 (keys and per-token limits, the `RateLimit-*` headers). **Neither part exists at this
writing** (`…-parts/` holds no `task-159.md` or `task-160.md`, checked 2026-09-26), so every name
taken from them is their scopes' (`…-scopes/tasks-141-160.md:746-775`, `:776-812`) and
**UNVERIFIED**. Task 143's part (the W5 wire) does not exist either; the one wire record this task
adds follows task 169's desktop mirror (`task-169.md:308`, `MCPConnector`), and task 143 wins on
names when it lands (SW.21). Every `server/` and `store/` path is **UNVERIFIED until merge** (rule
H1). Parity: closes **L-196** (the Work MCP server); **L-197** and **L-256** (the connector
allow-list) jointly with tasks 166 and 173; the MCP half of **L-244** with task 159. Design: 05 §1
M, P, R; §3.6.

**OD-30 changes this task, and the skeleton row and scope predate it.** The scope
(`tasks-161-180.md:29-83`) says the token is checked by `entra.Validator` and names
`https://login.microsoftonline.com/<tid>/v2.0` and `api://<hub-client-id>/Hub.Access`. OD-30
(main `docs/superpowers/planning/2026-09-23-owner-decisions.md:166`) moved sign-in to a Keycloak
realm brokering Entra, and the Hub contract's §17 (`hub-contract.md:2301-2385`, on `feat/hub` at
`3df9fab8`) says it **wins over §5.1 and §6**: validation is `oidcauth.Validator` under the
`keycloak` profile, the audience and required scope are both `panacea-app-hub`, and `Hub.Access`
*"no longer exist[s]"* (`:2352-2353`). The ruling names this task directly: *"the MCP server's
`authorization_servers` = the realm; … third-party apps are realm clients under K6's rule"*
(main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md:601`, K6 at `:543-549`).
This part follows OD-30. The work tree's copy of the owner decisions stops at OD-28 and has no
OD-30 line (gap 1).

**What the reader does.** Priya, an engineer, adds *PanaceaApp Work* to an MCP client her org's
admin approved (task 159). The client calls `https://work-mcp.<domain>/mcp`, is told 401 with a
pointer to the protected-resource document, signs her in through the realm, and lists nine tools.
She asks it *"what is in Triage for Checkout?"*; the client calls `work_list`, which the Hub
answers as Priya, showing only what she may read. Her colleague Sam points his client at
`/mcp/read`: he sees five tools, and a hand-written `tools/call work_comment` is refused.

**Decisions made here, so nobody re-derives them:**

1. **`workmcp` holds no credential of its own, and says so in code.** It has no key, no secret and
   no client credentials in its config. Every Hub call goes through `hubclient.New(baseURL, scope,
   tokens, http)` (`hubclient/client.go:21`, on `feat/hub`) with a `TokenSource` that returns the
   caller's own bearer from the request context (`callerToken` below). A call with no caller in
   context fails before any byte leaves. The boundary rule holds: `workmcp` imports `hubapi`,
   `hubclient` and `oidcauth` only (`hub-contract.md:78-82`).
2. **`workmcp` validates too, but the Hub decides.** `auth.go` runs `oidcauth.Validator` on every
   request so a bad token is a 401 with the discovery header *here*, where the MCP client reads it.
   It then forwards the same bearer; the Hub validates again and applies task 159's `AppClients`
   lookup, task 160's limits and the role matrix. Two checks, one authority: `workmcp`'s check
   only ever refuses more, never admits what the Hub would refuse.
3. **The approved-app check is the Hub's, reached through one read.** An `azp` that is the
   desktop's client (`panacea-app-desktop`) is refused here with 401 `client`: the desktop never
   talks MCP, and a stolen desktop token must not be replayed through it. Any other `azp` is
   forwarded, and task 159's hook answers 403 `app_not_approved` for an unapproved one; `workmcp`
   maps that one code to 401 `client` with the discovery header, so the MCP client re-runs sign-in
   instead of showing a dead error. The scope's *"401 `client`"* and task 159's *"403
   `app_not_approved`"* are both kept, each at its own layer (gap 3).
4. **The transport is the smallest streamable-HTTP subset that is correct.** One endpoint per mode,
   `POST` only; each request is one JSON-RPC 2.0 object and each answer one `application/json`
   object. No SSE stream is opened (the server never sends a request or notification to the
   client), `GET` is 405 with `Allow: POST`, batches (a JSON array) are refused with `-32600`,
   and `Mcp-Session-Id` is not issued, since no tool keeps state between calls. Protocol revision
   `2025-06-18`; `initialize` answers with that revision whatever the client asks, and a client
   that cannot speak it disconnects (the revision negotiation rule; which clients accept it is
   **UNVERIFIED**, see *Not claimed*).
5. **Read-only is a route, not a flag in the token.** `/mcp/read` builds its tool table from the
   read tools only, so `tools/list` cannot show a write and `tools/call` cannot find one:
   *unknown tool* (`-32602`), never a 403 after dispatch. A `work.read` app on `/mcp` sees the
   write tools listed (the list is the endpoint's) and the Hub refuses the call (task 159).
6. **No tool writes a fact state, twice over.** `work_move.to` is an enum of the human bands only,
   checked by `workmcp`'s schema validator before any Hub call; and the Hub refuses a fact state or
   `done` from an app token anyway (task 145's guard, SW.3). The enum is `triage`, `backlog`,
   `todo`, `cancelled`: 05 §3.3's person-set states (`05-design.md:722-734`) as task 37's
   `IssueState` spells them (`task-37.md:87-89`). `duplicate` is person-set too but needs the
   original's key, which `work_move` does not carry, so it stays out; W2's custom states inside
   the bands are left out until their Go names are on the sync base (gap 4).
7. **An alias is a lookup, and the key is the answer** (skeleton rule 1). `work_issue {issue:
   "EP-142"}` resolves through the Hub's alias lookup and returns `issue:is-…@r` first; the alias
   appears only as a display field.
8. **Text keeps its author class.** Every text field in a tool result is rendered as a fenced block
   headed `text·H` or `text·A` with the author's display name, never merged into the tool's own
   prose, so an MCP client's model reads it as quoted data (05 §3.4).
9. **The connector allow-list stores no connector credential.** The Hub keeps `{url, name,
   allowedBy, allowedAt}` and nothing else; each person connects their own account in their own
   client. `ConnectorFragment` renders what task 166 applies in the sandbox.

**Files:**
- `control-plane/cmd/workmcp/main.go` — flags, config, the two routes, graceful stop (new)
- `control-plane/cmd/workmcp/config.go` (+ `config_test.go`) — `PANACEA_APP_WORKMCP_*`, no secret field (new)
- `control-plane/cmd/workmcp/auth.go` (+ `auth_test.go`) — bearer check, RFC 9728 document, `callerToken` (new)
- `control-plane/cmd/workmcp/mcp.go` (+ `mcp_test.go`) — JSON-RPC 2.0 over one POST, the five methods (new)
- `control-plane/cmd/workmcp/tools.go` (+ `tools_test.go`) — the nine tools, their schemas, dispatch to `hubclient` (new)
- `control-plane/cmd/workmcp/schema.go` — the small JSON Schema subset the tools use, validated before dispatch (new)
- `control-plane/internal/hub/store/work_mcp.go` (+ `work_mcp_integration_test.go`) — the allow-list table (new)
- `control-plane/internal/hub/server/work_mcp.go` — three routes and `ConnectorFragment` (new)
- `control-plane/internal/hub/server/work_mcp_fragment_test.go` — the pure fragment rows (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_mcp_connectors.sql` — next free number at sync time (new, rule H1)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — `workMatrixRows["161"]` (modified)

The File map row `control-plane/cmd/workmcp/` (skeleton `:257`) says *"behind Entra OAuth"*; under
OD-30 it is the realm's OAuth (gap 1). The Hub half has no File map row of its own; it sits under
the `store/work_*.go` and `server/work_*.go` rows as every WH task does.

**What is on the tree today (verified 2026-09-26).** On `feat/work-layer`: `control-plane/cmd/`
holds only `api/`; `control-plane/go.mod` is `go 1.23`. On `feat/hub` (`3df9fab8`):
`hubclient.New(baseURL, scope string, tokens TokenSource, httpClient *http.Client) *Client`
(`hubclient/client.go:21`) and `TokenSource.Token(ctx, scope) (string, error)` (`hubclient/hubclient.go:19-20`);
`oidcauth.Config` and `Claims` are hub task 10's part (`phase-h-hub-parts/task-10.md:20-58`), the
package itself env task 17's, **UNVERIFIED until merge**. No `Work` or `WorkIntake` interface is on
`feat/hub`; the methods `tools.go` calls are task 143's scope (`tasks-141-160.md:124-200`) and
**UNVERIFIED**.

#### Interfaces — complete: `control-plane/cmd/workmcp/config.go`

```go
package main

// Config is every variable workmcp reads. It has no secret field on purpose:
// workmcp acts only as the caller (decision 1), so there is nothing to hold.
type Config struct {
	Listen        string // PANACEA_APP_WORKMCP_LISTEN, default ":8090"
	PublicBase    string // PANACEA_APP_WORKMCP_PUBLIC_BASE, https origin clients see; required
	HubURL        string // PANACEA_APP_WORKMCP_HUB_URL, the Hub's base; required
	OIDCIssuer    string // PANACEA_APP_OIDC_ISSUER, the realm (hub-contract §17); required
	OIDCProfile   string // PANACEA_APP_OIDC_PROFILE, "keycloak"; anything else fails start
	HubAudience   string // PANACEA_APP_HUB_AUDIENCE, default "panacea-app-hub"
	TenantIDs     []string // from PANACEA_APP_HUB_TENANTS, "<guid>=<name>,…"; GUIDs kept; required
	DesktopClient string // PANACEA_APP_OIDC_CLIENT_ID, default "panacea-app-desktop"; refused here
	ServiceClients []string // PANACEA_APP_HUB_FLEET_CLIENT_ID, _GATEWAY_CLIENT_ID; refused here
	MaxBody       int64  // PANACEA_APP_WORKMCP_MAX_BODY, default 65536 bytes
}

// LoadConfig reads environ (os.Environ() in main; a slice in tests). A missing
// required variable, a PublicBase that is not https (loopback http allowed only
// with PANACEA_APP_WORKMCP_INSECURE=1, for tests), or any variable whose name
// starts PANACEA_APP_WORKMCP_ and contains KEY, SECRET or TOKEN fails start
// with that variable's name.
func LoadConfig(environ []string) (Config, error)
```

The last rule is the test's hook for decision 1: an operator who adds
`PANACEA_APP_WORKMCP_HUB_TOKEN` gets a failed start, not a service token. `PANACEA_APP_HUB_TENANTS`
is the Hub's own variable (`hub-contract.md:2165`, `<tenant guid>=<display name>`, comma-separated),
read the same way so both processes enrol the same orgs.

#### Tests to write first

`hubapi/workstates_test.go` (unit, no database):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMoveAllowedIsTheTable` | every `(from, to, cause)` of the 9 × 9 × 6 cube against a literal expected set written out in the test | a loosened row (`hand` → `started`); a fact cause accepted for the wrong target |
| `TestEveryCriterionNotAny` | two served criteria, one cited → `waiting`; both → `proven` | *any* criterion instead of *every* |
| `TestObservedIsNeverSelfAttested` | creator = starter = recorder, cause observed → `proven`; cause recorded → `proven_self_attested`; recorded by a reviewer → `proven` | observed evidence marked self-attested; self-attestation keyed on the recorder alone |
| `TestTheStrongestCitationCounts` | one criterion with a self-recorded and an observed citation → `proven` | the first citation found deciding the word |
| `TestHandSetAndNoCriteria` | `HandSet` with serves → `not_proven`; with none → `no_criteria`; no serves, not hand-set → `no_criteria` | a hand-set done that reads proven |

`server/work_states_integration_test.go` (`//go:build integration`). Each seeds a brief with two
criteria, an issue serving them, a lane's `lane_bound` and `land_trailer` moves as Linus, and
evidence through the Hub's own `POST /v1/evidence` (H25), so no row is forged:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEvidenceBeforeTheLandingDoesNotClose` | evidence recorded, then the issue lands; citing it → 422, `why: before_landed`; the issue stays `landed` | a rule without the time bound |
| `TestASiblingsEvidenceAtAnotherCommit` | EP-150 landed at C2 with evidence at C2; EP-142 landed at C1 cites it → 422, `why: commit` | matching evidence by criterion alone |
| `TestSharedEvidenceNeedsTheSameLanding` | EP-142 cites a record with `evidenceCommit` C2; EP-150, landed at C1 ≠ C2's issue landing, cites it too → 422 `cited_elsewhere`; two issues both landed at C1 share one record → both `done` | reuse of another issue's proof |
| `TestEveryServedCriterionIsNeeded` | evidence for one of two criteria → 200, `landed`, `proof: waiting`; the second → `done`, `proven` | *any* instead of *every*, end to end |
| `TestAFactIsNotSetByHand` | `to: started` with cause `hand` → 422 `state_not_allowed`; `to: landed` with `hand` → 422 | a hand-faked fact |
| `TestAHandSetDoneNeedsAReason` | on the serving issue: no `reason` → 400 naming `reason`; with one → 200, `done`, `proof: not_proven`, `stateReason` the reason | a reasonless hand-set done; a hand-set done that reads proven |
| `TestSelfAttestedAndObserved` | Linus creates, starts and records → `proven_self_attested`; the same Linus with `evidence_observed` → `proven` | observed evidence marked self-attested |
| `TestNoCriteriaLandsAtDone` | an issue with `serves: []` (reason `chore`): `land_trailer` → `done`, `no_criteria`; with the team's `stopAtLandedWithoutServes` on → `landed`, `no_criteria` | the switch ignored |
| `TestProjectLevelIsItsOwnThreadState` | an issue serving only the project brief's criterion reads `threadState.kind` `project_level` | scope ignored (task 08's `threadStateOf`, pinned end to end) |
| `TestFactsAreIdempotentAndContradictionsRefused` | `lane_bound` twice → one event; `land_trailer` C1 twice → one event; `land_trailer` C2 after C1 → 409 `state_conflict` | a hook firing twice writing two facts; a second landing replacing the first proof |
| `TestAReopenClearsTheProof` | a `done` issue reopened to `todo` by hand → `proof: ""`, `landCommit: ""`, its citations `cleared_at` set, its evidence citable by another issue | a stale proof surviving a reopen |
| `TestEvidenceNeverSkipsTheLanding` | as `hub_app` directly, with the settings for `evidence_observed`, `UPDATE` a `started` issue to `done` → `42501` (the replaced guard's clause) | done by evidence on an issue that never landed, at the database layer |
| `TestOneStateEventPerMove` | every move above writes exactly one `issue.state`, whose `data` holds only the five keys, none of them text | two events per move; content in the feed |

#### Steps

- [ ] **Step 1: Check the base.** H25's evidence route and table on the sync base; task 07's
  guard as merged; task 08's `issueHead`. If `evidence` changed shape, the Hub's file wins and
  decision 3's columns follow it.
- [ ] **Step 2: The unit tests**, then `MoveAllowed` and `DecideProof`, until green. Then the
  integration tests; add `"10"` to `workMatrixLive`; they fail.
- [ ] **Step 3: The migration additions**, the store, the route and the two lines in `issueHead`;
  extend task 06's grant-list test with the two tables. Run this task's, task 06's, task 07's and
  task 08's suites, and the matrix, until all are green.
- [ ] **Step 4: Commit by pathspec**, the ten files, naming the migration file in the message.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -run '^Test(MoveAllowedIsTheTable|EveryCriterionNotAny|ObservedIsNeverSelfAttested|TheStrongestCitationCounts|HandSetAndNoCriteria)$' -v 2>&1 | grep -c '^--- PASS'   # 5
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(EvidenceBeforeTheLandingDoesNotClose|ASiblingsEvidenceAtAnotherCommit|SharedEvidenceNeedsTheSameLanding|EveryServedCriterionIsNeeded|AFactIsNotSetByHand|AHandSetDoneNeedsAReason|SelfAttestedAndObserved|NoCriteriaLandsAtDone|ProjectLevelIsItsOwnThreadState|FactsAreIdempotentAndContradictionsRefused|AReopenClearsTheProof|EvidenceNeverSkipsTheLanding|OneStateEventPerMove)$' \
  2>&1 | grep -c '^--- PASS'                                                    # 13
go test ./control-plane/internal/hub/... -tags integration -race -count=1       # every package ok
grep -rn "hub.work_cause" control-plane/internal --include=*.go | grep -v _test.go   # store/work_states.go only
```

Expected: `5`, then `13`, then every package `ok`, then one line, in `store/work_states.go`.

#### What this task does not claim

- **Not that a test run was watched.** The Hub records the cause the person's control plane sends;
  `evidence_observed` meaning "the platform saw the exit code at the land commit" is task 23's
  claim, and so is the ancestry of an `evidenceCommit`.
- **Not that the lane facts arrive.** Task 22 sends `lane_bound`, `lane_queued` and `land_trailer`;
  this task only accepts them.
- **Not custom states (W2, task 75) or a parent's roll-up (L-21, W2).**
- **Not the third proof word.** Evidence recorded by a reviewer reads `proven` by this task's
  reading; the owner may want it named apart.
- **No measured cost.** One evidence read and one citation query per move; nobody has timed it.

**Parity rows served:** **L-8** (fixed fact states; closed with task 22 and the glyphs, task 28),
**L-10** (the `duplicate` state), and the Hub half of 05 §2.3's *done means proven*, which no L row
names alone; it underlies **L-146** and **L-201** (closed with tasks 21-22).
