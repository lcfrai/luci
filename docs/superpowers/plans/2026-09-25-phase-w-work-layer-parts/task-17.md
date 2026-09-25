### Task 17: W1's Hub end to end through `devissuer`

W1, stream WH; no seam. Depends on tasks 10 (the fact moves and the done rule, SW.4), 13 (the Work
event kinds on the change feed, SW.5), 14 (erase), 16 (`hubclient`'s Work methods over HTTP), and
Hub task H43 (the Hub's own end-to-end proof through `devissuer`, its harness and results record).

Every W1 Hub task proved its piece against a scratch database. This one proves **the story**: the
Hub half of 05 §5.3's demo, driven the way the desktop will drive it, through **task 16's
`hubclient` over HTTP** and never the store, by four people holding real PKCE tokens from
`devissuer`. After each step it asserts two things: the response, and **the exact event the step
appended** to the change feed. A route that answers correctly and appends two events, or none, is
the bug this test exists to catch, because the desktop's loader (task 37) invalidates on events and
a missing one leaves a stale board.

**The one thing this task must not get wrong is who can see what.** The story runs in team T; a
member of team U in the same org watches the same feed throughout and must see **no** Work event
and get **404** for EP-142. A readability hole across teams is the defect that would ship
unnoticed by every single-team test in tasks 08-15.

**Files:**
- `control-plane/internal/hub/work_e2e_integration_test.go` — `TestWorkE2E`, `//go:build
  integration` (new)
- nothing else. The results of the run go in H43's record,
  `docs/verification/2026-09-23-phase-h-results.md` (H43 `task-43.md:16`), as one appended section
  **by task 70**, which records W1's parity rows with their desktop halves; this task writes no
  document.

**What is on the tree today (verified 2026-09-25).** `control-plane/internal/hub/` on `feat/hub`
(`.claude/worktrees/hub` at `9f2fa31`) holds only `hubapi` and `hubclient`; `hubtest`, `server`,
`devissuer`, `store` and `control-plane/cmd/hub/compose.yaml` are **planned** (H-plan tasks 02-16,
11-12, 43) and **UNVERIFIED until merge**. H43's part names its world
(`task-43.md:22-38`): `hubtest.Migrated`, `devissuer` in-process with `Auto` and `AccessTTL`, the
Hub's real handler (`server.New` + `entra.New`, `AllowInsecureAuthority`) on an `httptest.Server`,
and one frozen test clock `clk` shared by the issuer and the Hub's validator. This task builds the
same world and reuses H43's helpers by their merged names; the names below are H43's part's and are
re-read at sync.

**Interfaces:**
- Produces: `TestWorkE2E` and its step list, the proof task 70 cites.
- Consumes: task 16's `hubclient.Client` Work methods; H34's `Briefs`, `Projects`, `Teams` and
  `Evidence` methods; H43's world; `devissuer.Options`, `Config`, `User{Roles}` (H11
  `task-11.md:66-129`) and its `Auto` + `login_hint` code flow (H12 `task-12.md:47`); task 10's
  `StateMove` causes and `proof` words; task 13's Work event kinds; task 14's erase.
- Required by: task 70 (W1's local-mode demo and its results record).

#### Contract — the world and the four people

```go
// control-plane/internal/hub/work_e2e_integration_test.go
//go:build integration

package hub_test

// workPeople is devissuer's one tenant for this run: four users, one org.
// No secret is in it; devissuer's config refuses unknown fields (H11).
var workPeople = []devissuer.User{
	{OID: "5b0c1c9e-0000-4000-8000-00000000d71a", Name: "Dylan Example",
		Username: "dylan@example.test", Email: "dylan@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-0000000a11ce", Name: "Alice Example",
		Username: "alice@example.test", Email: "alice@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-00000000ad01", Name: "Olga Example",
		Username: "olga@example.test", Email: "olga@example.test",
		Roles: []string{"Hub.OrgAdmin"}}, // O: org admin, member of no team
	{OID: "5b0c1c9e-0000-4000-8000-000000000bea", Name: "Bea Example",
		Username: "bea@example.test", Email: "bea@example.test"}, // N: team U only
}

// person is one signed-in principal: a real hubclient.Client over HTTP whose
// TokenSource holds the access token the code + PKCE S256 flow returned.
type person struct {
	name string
	oid  string
	hub  *hubclient.Client
}

// signIn runs devissuer's /authorize (Auto + login_hint, H12) and /token with
// a fresh verifier, and returns a person whose every call carries that token.
// It never calls MintAccessToken: the proof is that the browserless code flow
// the desktop's sign-in uses yields a token the Hub accepts for Work routes.
func signIn(t *testing.T, w *workWorld, username string) person
```

**The roles, set up in step 0 through the Hub's own routes.** Dylan creates team **T** (so he is
its `admin`) and project **P** in it with remote `github.com/lcfrai/panaceaapp`; he invites Alice,
who accepts as a `member` (H43's invite binding). Bea creates team **U** and a project there. Olga
is in no team and holds `Hub.OrgAdmin`: the matrix's `O` column (hub-contract §5.3, `:337-341`),
who may erase but may **not** read project content she is not a member of (`:331`).

**The frozen clock.** `clk` is H43's: frozen through every step (`task-43.md:29-35`), so no token
expires and no refresh grant is made mid-story. Every timestamp this test compares is read from a
response, never from the wall clock.

