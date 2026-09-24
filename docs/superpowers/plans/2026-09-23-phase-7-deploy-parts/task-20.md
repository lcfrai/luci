### Task 20: Seams, the live check (owner-batched), and the Panacea ask

**Half:** — . **Hours:** 1.5 to write; the owner's live session is about 45 minutes. **Depends
on:** all. **Elsewhere:** Hub H5 running for the Ship steps; the owner's hands (OD-12).

Three things close the phase. **The seams**, written down as built, so the Panacea lift (OD-13) and
the later phases find each one by path and test, not by memory. **The live check**, the one place
phase 7 meets real Panacea: every item earlier parts marked *unverified* is settled here, in one
session the owner runs at the end with everything else (OD-12) — until then each stays unverified,
and every report says so. **The Panacea ask**, drafted for the owner to send (plan §11 Q-P7-2,
Q-P7-3; ruling R7): sending it is the owner's, not the plan's.

**Nothing in this task calls Panacea while it is being written.** The live test is build-tagged and
runs only in the owner's session.

**Files:**
- `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/SEAMS.md` (new) — the seams and the Hub asks, as built
- `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/LIVE-CHECK.md` (new) — the owner's session, step by step, with a column for what was seen
- `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/panacea-metadata-ask.md` (new) — the note to Panacea's team
- `control-plane/internal/deploy/panacea/live_test.go` (new, `//go:build live`) — the read-only half of the live check
- `control-plane/internal/deploy/panacea/live_guard_test.go` (new, untagged) — keeps `live_test.go` tagged and secret-free
- `control-plane/internal/deploy/seams_test.go` (new, untagged) — keeps `SEAMS.md` honest
- `docs/superpowers/plans/2026-09-23-phase-7-deploy.md` (modified) — §12 pointed at `LIVE-CHECK.md`, each settled item moved out after the session

#### `SEAMS.md` — complete, with the paths the implementer fills in

Each row names the file and the test that holds it. The implementer replaces every `<…>`
with the real line when the task lands; a seam with no test is not done.

```markdown
# Phase 7 — seams, as built

| Seam | Held by (file : line) | Pinned by (test) | The later phase | Must not |
|---|---|---|---|---|
| S7-N1 the native deployer | `deploy.Deployer` (`control-plane/internal/deploy/deploy.go:<…>`); `panacea.Client` wired once in `cmd/api/main.go:<…>` | `deploytest.Conformance` over `Fake`, and `TestConformanceOverConnector` (task 03) | the Panacea lift (OD-13) passes the suite and swaps one line in `main.go`; deletes the `PanaceaApp/panacea/<host>` credential row | change a domain type to fit Panacea's wire; add a Panacea-only method |
| S7-N2 the metadata mirror | `panacea.Client.Ship` (`panacea/acts.go:<…>`) sends no `metadata` | `TestShipSendsExactlyBranchCommit` (task 04) | if ask 1 below is granted: send `{"panacea.app/release": "<rl-…>"}`; the watcher may confirm a crashed reservation by it (ADR-H1 R3) | pin by the annotation — it names a reservation; the pin stays the Hub's |
| S7-N3 Panacea-side hash suffix | the collision rule, `deploywatch/view.go:<…>` | `TestCollisionExactStrings` (task 08) | if ask 3 is granted, collisions of long names cut at 20 stop firing on their own; a push-and-Ship label split of one branch (plan §12) still shows | re-implement `sanitizeBranchEnv` here |
| S7-N4 a rollback as a Panacea row | "serving after rollback", `deploywatch/view.go:<…>` (plan 7P-4) | `TestRollbackIsCurrent` (task 08) | if ask 2 is granted: a portal rollback shows as a `rollback` row; the Hub rule stays for cockpit rollbacks | drop the Hub's confirmed-rollback reading before Panacea's rows are observed live |
| S7-E1 phase 8's environment | the Panacea origin and CA are configuration (`cmd/api/config.go:<…>`, plan §6) | `TestParseOriginTable` (task 02), `TestLockedOriginRefusesAnother` (task 05) | phase 8 sets them for the product's environment | hard-code `dev.panacea.kpmg` |
| S7-X1 another deploy target | `deploy.Deployer` | the conformance suite | a customer-hosted tier implements the interface | branch on target type above it |
| S7-P1 the criterion evaluator | criteria drawn `open`, *"no evidence recorded yet"* (phase 4's `ShippedManifest`, mounted by `deploy/PinManifest.tsx:<…>`) | `pinned renders the pinned ref` (task 17) | phase 3's `evidence.Evaluator` at `Deployment.Commit` (`S3-P7.5`) | evaluate at the branch head (P12) |
| S7-P2 the merge link | `PinManifest` passes no `mergeRef` (`deploy/PinManifest.tsx:<…>`) | `no merge link is claimed` (task 17) | whichever phase records a merge link in the Hub adds a field to the release or a route; `PinManifest` passes it | infer a merge from a commit being on `main` |
| S7-S1 sessions by trailer | `sessions.Reader.Read` (`deploy/sessions/sessions.go:<…>`) | `TestRangeIsExclusiveInclusive` (task 19) | phase 3's hook is the only writer of `Panacea-Session` | infer a session from a commit's time (`S3-P7.1`) |
| S7-R1 a caller that waits out a long `Retry-After` | `do` returns a `Retry-After` over 2 s on `Error.RetryAfter` (`panacea/client.go:<…>`); the watcher does not read it (plan §10.2) | `TestGetRetryHonoursRetryAfter` (task 02) | open (b07 round 2, M-3): the watcher's next cycle for that project waits `max(cadence, RetryAfter)`; not t07's per-cycle deadline (M14) | block inside `do` on a long `Retry-After`; retry a POST, PUT or DELETE |

## Hub asks — state when phase 7 landed

| Ask | Blocks | State (asked / answered / built) | Meanwhile, in code |
|---|---|---|---|
| HA7-1 environment may be null at reserve — **withdrawn** (review b02) | nothing: Panacea names a never-deployed branch's environment (`GET P/branches/{branch}/access` `data.env`) | withdrawn | `Reader.EnvironmentFor` before reserve (task 09 step 4) |
| HA7-2 rollback confirm is `{}` | nothing (phase 7 sends `{}`) | <…> | `TestRollbackConfirmsEmpty` (task 10) |
| HA7-3 a shared deploy binding | nothing | <…> | the local choice file (task 06) |
| HA7-4 a generic cursor mechanism | the desktop's cursor edge | <…> | the declared amendment (tasks 07, 13) |
| HA7-5 a `kind` filter on releases | nothing | <…> | the watcher filters (task 07) |
| HA7-6 one Azure DevOps repo-key spelling — to phase 3 (fix its review I10: the H-3 row quotes task 13's form) and the Hub (keep correction A1; fold it into §7.4 in task 44a); review I6, plan re-review b05 R-1 | **Ship for every Azure DevOps repository** (OD-4), if the built code disagrees: phase 3's `RepoKey()` spells `azure-devops/<org>/<project>/<repo>`; the Hub plan's A1 accepts that form, but the contract's §7.4 text still shows `<org>.visualstudio.com/<project>/<repo>` | <…> — and whether `TestHubBindsADOWithPhase3sKey` (task 06) passed against the built code when phase 7 landed: pass (closed) or red (open, with the key `githost` gave and the `CreateProject` refusal) | `hubKey` sends `RepoKey()` verbatim under either answer; `missCause` asks `hubapi.HostOfRepoKey`, so `HubMiss "key_spelling"` follows the Hub's built rule and names the cause in readiness, the Ship refusal (`details.cause`) and the lens line (tasks 06, 07, 09, 14) |
```

#### `live_test.go` — the read-only half, complete

It reads **the token the owner pasted into the connect panel**, from the credential store row task
05 writes, through task 05's own `Manager` — never from an environment variable, a flag or a file,
so the token is in no shell history. It writes nothing to Panacea. It logs field **names** and
counts, never a body and never the token.

```go
//go:build live

// Run in the owner's session only (LIVE-CHECK.md step 3):
//   PANACEA_APP_LIVE_PROJECT=<panacea project> PANACEA_APP_LIVE_ENV=<an environment name> \
//   go test -tags live -run Live -count=1 -v ./control-plane/internal/deploy/panacea/
// Step 5a adds PANACEA_APP_LIVE_PROD_BRANCH=<the production branch from the portal's settings>.
// These three are test-only inputs, never read by the control plane.
package panacea_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/credstore"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/githost"
)

func liveDeployer(t *testing.T) (deploy.Deployer, deploy.ProjectRef, string) {
	t.Helper()
	m, ref, env := liveManager(t)
	return m.Current(), ref, env
}

// liveCAFile is the same variable the control plane reads (plan §6): step 1 of LIVE-CHECK.md
// says to set it when this laptop's Go does not trust the corporate interception root (7P-10).
func liveCAFile() string { return os.Getenv("PANACEA_APP_PANACEA_CA_FILE") }

func liveManager(t *testing.T) (*connection.Manager, deploy.ProjectRef, string) {
	t.Helper()
	project, env := os.Getenv("PANACEA_APP_LIVE_PROJECT"), os.Getenv("PANACEA_APP_LIVE_ENV")
	if project == "" || env == "" {
		t.Skip("PANACEA_APP_LIVE_PROJECT and PANACEA_APP_LIVE_ENV name what to read")
	}
	m, err := connection.New(connection.Config{
		Secrets:    credstore.WinCred(),
		IsNotFound: func(err error) bool { return errors.Is(err, credstore.ErrNotFound) },
		CAFile:     liveCAFile(),
		NewClient: func(origin string, tokens panacea.TokenSource) (deploy.Deployer, error) {
			return panacea.New(panacea.Config{Origin: origin, Tokens: tokens, CAFile: liveCAFile(),
				UserAgent: "PanaceaApp/live-check"})
		},
	})
	if err != nil {
		t.Fatalf("connection: %v", err)
	}
	return m, deploy.ProjectRef(project), env
}

func TestLiveWhoamiAndProjects(t *testing.T) {
	d, ref, _ := liveDeployer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	acct, err := d.Whoami(ctx)
	if err != nil {
		t.Fatalf("Whoami: %v", err) // KindUnauthorized here means the pasted token is rejected
	}
	t.Logf("account admin=%v", acct.Admin)
	projects, err := d.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	for _, p := range projects {
		if p.Ref != ref {
			continue
		}
		// Plan §12: does GET /api/v1/projects return repoUrl to a non-admin token?
		if p.RepoURL == "" {
			t.Errorf("repoUrl is empty for %s: part 06's fallback (the user names the project) applies", ref)
			return
		}
		// Task 06: Panacea normalises clone URLs; phase 3's parser must accept the result.
		if _, err := githost.ParseRemote(p.RepoURL); err != nil {
			t.Errorf("githost.ParseRemote(<repoUrl of %s>): %v", ref, err)
		}
		return
	}
	t.Fatalf("project %s is not visible to this token", ref)
}

// Records two answers the plan could not read from source (plan §12; task 08; 7P-8), and the
// environment labels it did read from source (review M5). It logs them for LIVE-CHECK.md; it
// fails only when a read fails.
func TestLiveEnvironmentsAndBranches(t *testing.T) {
	d, ref, _ := liveDeployer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	envs, err := d.Environments(ctx, ref)
	if err != nil {
		t.Fatalf("Environments: %v", err)
	}
	for _, e := range envs {
		t.Logf("environment name=%q primary=%v replicasMeasured=%v", e.Name, e.Primary, e.Replicas != nil)
	}
	deps, err := d.Deployments(ctx, ref)
	if err != nil {
		t.Fatalf("Deployments: %v", err)
	}
	slashed, underscored := 0, 0
	for _, x := range deps {
		if strings.Contains(x.Branch, "/") {
			slashed++
		}
		if strings.Contains(x.Branch, "_") {
			underscored++
		}
	}
	// Task 08: from source, Deployment.branch is sanitizeLabelValue(branch) from the API
	// (clients/pipeline.go:609) or sanitizeLabel(event.Branch) from the webhook-receiver
	// (webhook-receiver/pipeline/trigger.go:1270); both turn "/" into "-", so this should log
	// withSlashInBranch=0 (review b02). A non-zero count reopens task 08's note. Only the webhook
	// keeps "_", so withUnderscoreInBranch > 0 is the live sign that push-to-deploy runs reach this
	// project's list (re-review b03 I-1; unverified from source whether the dev estate runs it).
	t.Logf("deployments=%d withSlashInBranch=%d withUnderscoreInBranch=%d", len(deps), slashed, underscored)
	// Review M5: from source, production's label is "prod" and "" is legacy-only (plan §12,
	// Settled from source). Log each distinct environment label with its count, so the owner
	// sees "prod" and how many legacy rows task 08 rule 1b has to join by label.
	envCount := map[string]int{}
	for _, x := range deps {
		envCount[x.Environment]++
	}
	t.Logf("deploymentEnvironments=%v withEmptyEnvironment=%d", envCount, envCount[""])
	// Review b02: Panacea's own environment name for a slash branch, read through the ingress with
	// the branch %2F-escaped (plan §12). A read failure here is the answer: it fails the test.
	name, err := d.EnvironmentFor(ctx, ref, "live-check/p7")
	if err != nil {
		t.Fatalf("EnvironmentFor(live-check/p7): %v", err)
	}
	t.Logf("environmentFor live-check/p7=%q", name)
	// b02 round 2, R1: the access route reads the production branch from the label only
	// (branch_security.go:32); the deploy resolves label → annotation → main (pipeline.go:233-237).
	// For the project's real production branch, read from the portal's settings by the owner, the
	// answer should join the primary row. A name that does not is the Q-P7-6 defect, seen live.
	if pb := os.Getenv("PANACEA_APP_LIVE_PROD_BRANCH"); pb != "" {
		got, err := d.EnvironmentFor(ctx, ref, pb)
		if err != nil {
			t.Fatalf("EnvironmentFor(%s): %v", pb, err)
		}
		t.Logf("environmentFor productionBranch=%q joinsProduction=%v", got, d.SameEnvironment(got, "production"))
	}
}

// Task 12: frames must arrive while the stream is open, not all at once when it closes — a
// buffering proxy between this laptop and Panacea would batch them (DESIGN-SYSTEM :65 cannot
// hold if it does). Panacea emits one reading at once (handlers/readiness_stream.go:199-201) and
// its next bytes are a 25 s keepalive (:27), so a frame within 15 s means nothing held it.
func TestLiveReadinessStreamIsNotBuffered(t *testing.T) {
	d, ref, env := liveDeployer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	s, err := d.Stream(ctx, ref, deploy.StreamReadiness, env)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer s.Close()
	first := make(chan error, 1)
	go func() { _, err := s.Next(); first <- err }()
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first frame: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("no readiness frame within 15 s: a proxy is buffering the stream, or Panacea sends none")
	}
}
```

```go
// Task 03's promise: its recorded bodies were hand-written from source, and this is where they
// meet the live answer — one raw GET per read route of plan §4 rows 2-9, with the stored token.
// Never the diagnosis (row 9's third route): it may spend a Panacea model call.
//
// A key EVERY recorded item holds, and NO live item holds, fails for a route the connector
// decodes. A key some recorded item lacks is optional (omitempty — environments.json's second
// row has no `replicas`), so comparing first items would fail on a live list whose first row is
// the other kind; the sets below compare all items. A key only live bodies hold is logged.
func TestLiveBodiesMatchRecordedKeys(t *testing.T) {
	m, ref, env := liveManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	token, err := m.PanaceaToken(ctx)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	origin := m.Status().Origin
	if origin == "" {
		t.Fatal("no Panacea address is stored: LIVE-CHECK.md step 2 comes first")
	}
	hc, err := panacea.BuildHTTPClient(liveCAFile(), 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, []byte) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			t.Errorf("GET %s: %v", path, err) // *url.Error names the URL, never a header
			return 0, nil
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		return resp.StatusCode, b
	}
	// Every route below is decoded by a task 03 DTO (task 03, "The routes and their envelopes"),
	// pods and events included (podDTO, eventDTO), so a recorded key missing live is a defect.
	type check struct {
		path     string
		recorded []string // their key sets are intersected: a key one of them lacks is optional
	}
	p := "/api/v1/projects/" + url.PathEscape(string(ref))
	e := p + "/environments/" + url.PathEscape(env)
	checks := []check{
		{"/api/v1/me", []string{"me-approved.json", "me-pending.json"}},
		{"/api/v1/projects", []string{"projects.json"}},
		{p + "/environments", []string{"environments.json"}},
		{p + "/deploys/latest", []string{"latest.json"}},
		{p + "/deployments", []string{"deployments.json"}},
		{e + "/pods", []string{"pods.json"}},
		{e + "/events", []string{"events.json"}},
	}
	// Rows 7 and 8 need a run. A deployment's id IS its PipelineRun name (clients/kubernetes.go:5743,
	// `ID: pr.GetName()`), spelled "id" (models/models.go:666); deployId takes that name
	// (handlers/security_status.go:211-214). A project that never shipped answers 404 (row 5).
	// env is the deployment's own environment (json "environment,omitempty", models/models.go:681),
	// sent when non-empty exactly as the connector sends it (task 03): without it Panacea reads
	// production's namespace (security_status.go:192-201) and row 8 would compare the wrong
	// environment's report (review C3).
	if code, b := get(p + "/deploys/latest"); code == http.StatusOK {
		var latest struct {
			ID          string `json:"id"`
			Environment string `json:"environment"`
		}
		if json.Unmarshal(b, &latest) == nil && latest.ID != "" {
			q := url.Values{"deployId": {latest.ID}}
			if latest.Environment != "" {
				q.Set("env", latest.Environment)
			}
			checks = append(checks,
				check{p + "/pipelines/" + url.PathEscape(latest.ID), []string{"run-running.json", "run-failed.json"}},
				check{p + "/security-status?" + q.Encode(), []string{"security-status.json"}})
		}
	} else {
		t.Logf("deploys/latest answered %d: rows 7 and 8 are not compared", code)
	}
	for _, c := range checks {
		code, b := get(c.path)
		if code != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", c.path, code) // 0 = the transport error above
			continue
		}
		_, liveSome, n := keySets(t, b)
		if n == 0 {
			t.Logf("%s: no live item, so no keys to compare", c.path)
			continue
		}
		var wantEvery, wantSome map[string]bool
		for _, f := range c.recorded {
			every, some, _ := keySets(t, mustRead(t, filepath.Join("testdata", f)))
			wantEvery, wantSome = intersect(wantEvery, every), union(wantSome, some)
		}
		for k := range wantEvery {
			if !liveSome[k] {
				t.Errorf("%s: recorded key %q is in no live item — task 03's DTO reads it", c.path, k)
			}
		}
		for k := range liveSome {
			if !wantSome[k] {
				t.Logf("%s: live-only key %q", c.path, k) // a name, never a value
			}
		}
	}
}

// keySets unwraps plan §4's envelopes — `list` {items,totalCount}, `data` {data,error,message},
// a bare array, or a bare object — and returns the keys every item holds, the keys any item
// holds, and the item count. A body that is not JSON fails the test; the body is never logged.
func keySets(t *testing.T, b []byte) (every, some map[string]bool, n int) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("not JSON (%d bytes): %v", len(b), err)
	}
	if o, ok := v.(map[string]any); ok {
		delete(o, "_source") // task 03's provenance key, stripped before serving; never live
		if items, ok := o["items"].([]any); ok {
			if _, ok := o["totalCount"]; ok {
				v = items
			}
		} else if d, ok := o["data"]; ok && len(o) <= 3 {
			v = d
		}
	}
	items, ok := v.([]any)
	if !ok {
		items = []any{v}
	}
	some = map[string]bool{}
	for _, it := range items {
		o, ok := it.(map[string]any)
		if !ok {
			continue
		}
		keys := map[string]bool{}
		for k := range o {
			keys[k], some[k] = true, true
		}
		every, n = intersect(every, keys), n+1
	}
	return every, some, n
}

// intersect treats nil as "no set yet", so the first set passed in is kept whole.
func intersect(a, b map[string]bool) map[string]bool {
	if a == nil {
		return b
	}
	out := map[string]bool{}
	for k := range a {
		if b[k] {
			out[k] = true
		}
	}
	return out
}

func union(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("recorded body: %v", err) // task 03 writes every file this test names
	}
	return b
}
```

The build-log stream is checked by eye in step 7 below, because it needs a run in progress.

#### `LIVE-CHECK.md` — complete: the owner's one session (OD-12)

```markdown
# Phase 7 — live check (the owner's session, batched with phases 3–6)

**You provide** (plan §11): one `pnc_` token from the dev Panacea portal (Account → API keys); one
dev Panacea project whose repository is also a Hub project; about 45 minutes. **Running:** the
control plane and desktop from `feat/phase-7-deploy`, and the Hub (H5) in `compose.yaml`.
**Scratch branch:** `live-check/p7`, pushed, with one commit made from a sandbox session. Every
write below touches **only** that branch. Write down its posture and auto-deploy before step 10.

| # | Do | Expect | Settles | Seen |
|---|---|---|---|---|
| 1 | `go run ./control-plane/internal/deploy/panacea/cmd/versioncheck <dev origin>` | a version string. An `x509` error: stop, set `PANACEA_APP_PANACEA_CA_FILE`. A dial error or timeout where a browser reaches the address: stop, set `HTTPS_PROXY` to the corporate proxy, because Go reads no Windows or PAC proxy (t02:84-86, review M2) | 7P-10 on this laptop, and whether this network needs a proxy (the command task 02 writes and does not run; review I9) | |
| 2 | Deploy lens → paste the token → Connect | *connected*, or the awaiting-approval words; the field is empty after; `cmdkey /list` shows a `PanaceaApp/panacea/<host>` row | task 05, live, including the `PUT` preflight (review C1) | |
| 3 | `PANACEA_APP_LIVE_PROJECT=<project> PANACEA_APP_LIVE_ENV=<an env> go test -tags live -run Live -count=1 -v ./control-plane/internal/deploy/panacea/` | `ok`; copy the logged environment names, the `withSlashInBranch` and `withUnderscoreInBranch` counts, the `deploymentEnvironments` line and every `live-only key` line here | task 03's recorded bodies against live keys (plan §4 rows 2–9); repoUrl for a non-admin token (plan §12); `ParseRemote` on a real URL (task 06); `prod` vs `production` (7P-8: `"prod"` from source, review M5; this confirms it live, and `withEmptyEnvironment` counts the legacy rows task 08 rule 1b joins by label); branch names with `/` (task 08); push-to-deploy labels that keep `_` (task 08 rule 9, re-review b03 I-1); stream not buffered (task 12) | |
| 4 | Select the project | *bound* to the Panacea project, with no question asked — or the one choice | task 06 | |
| 5 | Compare the environment card with the Panacea portal for one environment | version, commit and pods ready agree; the three facts are three | tasks 08, 15 | |
| 5a | Read the project's production branch in the portal's settings; rerun step 3 with `PANACEA_APP_LIVE_PROD_BRANCH=<it>` added | the log line `environmentFor productionBranch="prod" joinsProduction=true` | whether the access route agrees with the deploy for this project (plan §12, Q-P7-6; b02 round 2, R1). `joinsProduction=false` is the Panacea defect live: record it and send it with the Q-P7-2 note | |
| 6 | On `live-check/p7`: Ship, then Undo inside 5 s | nothing new in the portal's pipeline list; before the Ship, the card names Panacea's environment for the branch (`live-check-p7`, the portal's name for it) and not *"Asking Panacea…"*. If the card instead reads *"Panacea did not name this branch's environment: Panacea could not read this branch name…"*, the ingress does not carry `%2F`: record it, and Ship refuses `409 deploy_environment_unreadable` (tasks 09, 15; b02 round 2, R2) | 7P-15, task 16; the `%2F` read of `GET P/branches/live-check%2Fp7/access` through the ingress (plan §12, review b02) | |
| 7 | Ship again and wait; open the release; **Follow build log**; scroll up while it runs | the run shows within about 3 s; lines arrive as they happen; the pane does not jump | tasks 09, 12, 17; the `Idempotency-Key` preflight (task 05, review C1) | |
| 8 | Read the manifest | the pin is the brief's latest complete revision; the session from the scratch commit is listed | tasks 09, 17, 19; Hub H5 | |
| 9 | Ship a second commit; then **Roll back to** the first release | the warmth line; the lens says the first is serving; the portal lists **no** rollback deployment | 7P-4, plan §12 | |
| 10 | **Rebuild** the first release | a new run of the first release's commit; its pin says *inherited* | 7P-3, task 10 | |
| 11 | Governance on the scratch branch: posture → `none`, Undo; → `password`, then back; deploy on push on, then off; claim an address, then release it | each held act sends nothing before 5 s; each refusal is Panacea's sentence | tasks 11, 18 | |
| 12 | If a second, non-owner account is available, open the panel with it. Then, as yourself, mint a second key restricted to this project at *Read only*, connect with it, and afterwards reconnect with an unrestricted key — mint a new one if you did not keep the first: Panacea shows a key once (`AccountPage.tsx:1406-1407`) and the cockpit never returns the stored one (t05's `TestConnectionGETNeverHasToken`) — and revoke the restricted one. Reconnect **before** step 13: putting auto-deploy back is owner-gated (`PUT …/auto-deploy`, `authedProjectOwner`, `routes/routes.go:1309`), which the restricted key cannot pass (plan re-review b06 M-1) | non-owner: the owner-only lines, before any press. Restricted key: the owner-only lines although you own the project; the no-match line on any other local repository; Ship refused with Panacea's *"… does not permit developer access to this project"* | 7P-9, Q-P7-7, review I5 | |
| 13 | Put the scratch branch's posture and auto-deploy back as written down | as they were | — | |

**After:** date, Panacea version (step 1) and this repository's commit at the top; commit this
file; move each settled item out of plan §12. Anything that did not match is a defect against the
task named in *Settles*, not a note.
```

#### `panacea-metadata-ask.md` — complete: the note, for the owner to send

Every line it cites was re-read at Panacea `b02424691` (2026-09-24). The note asks; it does not
assume a yes, and phase 7 builds nothing that needs one (seams S7-N2, S7-N3, S7-N4).

```markdown
# Three small asks of panacea-api, from the PanaceaApp cockpit

The cockpit ships to Panacea through its existing API, as the signed-in user, with their own
`pnc_` token. It records, in its own store, which brief each release was built against. Three gaps
make that record weaker than it could be. Each ask is additive; none changes an existing answer.

## 1. Carry a caller's reference on a deploy

**Today.** `POST /api/v1/projects/{name}/deploy` decodes `models.TriggerDeployOptions`
(`handlers/projects.go:2966-2968`), which has no field for a caller's reference
(`models/models.go:2342-2382`); the decode ignores unknown fields, so anything extra is silently
dropped. The PipelineRun gets only `panacea.io/deployed-by` and `panacea.io/trigger`
(`clients/pipeline.go:82-97`, applied at `:681`).

**Ask.** An optional `"metadata": {"<key>": "<value>"}` on that body — keys limited to a prefix
such as `panacea.app/`, at most 8 entries of 256 bytes — written as PipelineRun annotations beside
the two above, and returned on the deployment reads. The cockpit would send
`{"panacea.app/release": "rl-…"}`.

**Why.** Our deploy call can time out after Panacea has created the run: the handler spends up to
15 s creating it (`handlers/projects.go:2983-2988`). Today the cockpit cannot tell which of its
releases that run belongs to, so it records the release as abandoned and the run shows with no
brief. With the reference, it can match them exactly.

## 2. Record a rollback as a deployment

**Today.** `RollbackDeployment` re-points the environment and returns the **source** deployment's
id (`clients/kubernetes.go:5982`, `:6030`); no `panacea_deployments` row is written
(`clients/deployment_store.go:195` is the only writer, and no caller sets `Trigger` to `rollback`,
though `models/models.go:2378-2379` lists it as a value). So `deploys/latest` keeps reporting the
newer build after a rollback, and a rollback made in the portal is invisible to any client.

**Ask.** On a successful rollback, write a deployment row with a new id, `trigger_kind
"rollback"`, the source deployment's id, `deployed_by`, and `created_at`; return that id.

## 3. Keep two long branch names from sharing one environment

**Today.** `sanitizeBranchEnv` lower-cases, turns `/` and `_` into `-`, and keeps the first 20
characters (`handlers/projects.go:3814-3826`). `feature/evidence-upload-ui` and
`feature/evidence-upload-api` both become `feature-evidence-upl`: each deploy replaces the other's,
and nothing says so.

**Ask.** When a name is longer than 20, keep 13 characters, a `-`, and the first 6 hex of the
SHA-256 of the whole branch name (20 in all). This renames existing long-branch environments, so
it needs your migration call; a flag on new projects only would also serve us.

---
Sent by <owner>, <date>. Contact for the cockpit side: <owner>.
```

#### Tests to write first

The live test is the only code; what guards it runs in every ordinary build.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `go vet -tags live ./control-plane/internal/deploy/panacea/` | compiles against the real `connection`, `credstore`, `githost` and `deploy` names | a live check that rots unseen because nothing builds it until the owner's session |
| `TestLiveFileIsTagged` (untagged, `panacea/live_guard_test.go`) | `live_test.go`'s first line is exactly `//go:build live` | a live call running in CI, or in an agent's `go test ./...` |
| `TestLiveReadsNoSecretFromEnv` (untagged) | `live_test.go` contains no `Getenv` of a name holding `TOKEN`, `KEY` or `SECRET`, and no `t.Log` of a body | a token pasted into shell history, or written to a test log |
| `TestSeamsNamesRealTests` (untagged, `control-plane/internal/deploy/seams_test.go`) | every `Test…` name in `SEAMS.md`'s *Pinned by* column exists in the repository (`go test -list` output, or a source grep) | a seam pinned by a test that was renamed away |
| `TestSeamsHasNoPlaceholders` (same file) | after this task, `SEAMS.md` contains no `<…>` | a seam table left as a template |

#### Steps

- [ ] `SEAMS.md` from the block above; replace every `<…>` with the real line, and every
  Hub-ask state with its state that day.
- [ ] `LIVE-CHECK.md` and `panacea-metadata-ask.md` from the blocks above, verbatim.
- [ ] `live_test.go`, `live_guard_test.go`, `seams_test.go`; `go vet -tags live`.
- [ ] Check that `SEAMS.md`'s rows are plan §10.2's nine (S7-N1…N4, E1, X1, P1, P2, S1), one for
  one; point §12 at `LIVE-CHECK.md`.
- [ ] Commit by pathspec. The message says: *"The live check has not run. It is the owner's, batched
  (OD-12)."*

#### Acceptance

```
go vet -tags live ./control-plane/internal/deploy/panacea/            → no output
go test ./control-plane/internal/deploy/... -count=1 -run 'Live|Seams' → ok (the live tests themselves skip: untagged)
grep -c "<…>" docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/SEAMS.md → 0
```
**The owner's session** (`LIVE-CHECK.md`) is the acceptance of every *unverified* line in the
earlier parts. Until it runs, phase 7 is **built and tested against fakes, not verified live**.

#### What this task does NOT claim

- That the live check passed. It has not run; it runs once, with the owner, at the end (OD-12).
- That matching keys prove matching meaning. `TestLiveBodiesMatchRecordedKeys` compares key
  **names** only — not values, types or nesting below the first object — and it cannot see a key
  that neither the recorded bodies nor this one project's live answer happens to hold.
- That Panacea's team will say yes to any ask. Phase 7 needs none of them to work; each only makes a
  gap smaller, and each seam says what changes if it is granted.
- That `LIVE-CHECK.md` covers visual acceptance. The owner's eyes on each lens state are the UI
  tasks' own *Owner's eyes* lines, run in the same session.
