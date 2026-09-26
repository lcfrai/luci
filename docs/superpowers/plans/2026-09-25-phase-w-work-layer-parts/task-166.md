### Task 166: the per-team harness allow-list, Claude Code first; team skills in the sandbox image or a team library, the harness unmodified

W5, stream WH plus one control-plane delta, about 2.5 hours (an estimate, not a measurement).
Depends on task **145** (the `work_team_agent_policies` table, `task-145.md:351-373`) and Phase T's
**TM-13** (policy layers and `policy.Render`, team worktree `task-13.md:276-302`, `:388-390`) and
**TM-15** (sandbox classes and the policy mount). It uses task 143's `TeamAgentPolicy` wire and
`ValidateTeamAgentPolicy` (`task-143.md:557-569`, `:729-731`), task 161's `ConnectorFragment`
(`task-161.md:492-497`), and task 144's fake, which this task extends.

Parity: closes **L-159** (Claude Code's own skills, in the image or a team library, the harness
unmodified) and **L-180** (a per-team harness allow-list); **L-197** and **L-256** jointly with 161
and 173. Design: 05 §1 L (L-159), M (L-180), R (L-256); §5.4 W5. OD-8 (never modify or parse an
agent CLI), OD-16 (an admin governs, never acts as the person), OD-23 (policy is layered and
rendered once, TM-13).

**What the reader does.** Dylan, admin of team *Payments*, sets the team's harnesses to
`claude-code`, then `codex`, and points skills at `github.com/kpmg/payments-skills`, path
`skills/`, commit `4b1d…` (40 hex). Priya, a member, starts a session on a Payments worktree with
`codex`: it starts. She tries `gemini-cli`: the cockpit reads *Payments allows claude-code and
codex*. Her Claude Code session has the team's skills in its skills folder; the harness's command
line is the one it would have had without them. A month later Dylan asks *which skills did that
agent have?* and the agent-start record answers with the commit.

**Decisions made here, so nobody re-derives them:**

1. **Unset means `["claude-code"]`, never "all".** A team with no row reads `harnesses:
   ["claude-code"]`, `skills: {source: "image", library: null}`, `updatedBy: null`, `updatedAt:
   null` (`task-143.md:562`, `task-169.md:313-318`). The GET never 404s for a team the caller can
   read; the absence is the default, stated.
2. **The order is the fallback order, and the first is the default.** A session that names no
   harness gets the team's first, not `claude-code` by fiat; `sessions.go:785-788` defaults to
   `claude-code` today, and the gate replaces that default only for a team-bound project.
3. **The route is the scope's**, `GET`/`PUT /v1/teams/{teamId}/work/agent-policy`
   (`tasks-161-180.md:273`), which task 169 mirrors (`task-169.md:412-413`). Task 143's proposal
   `/v1/teams/{teamId}/agent-policy` (`task-143.md:952`) is corrected there (gap 1).
4. **A refusal is task 144's 400, naming the field.** `ValidateTeamAgentPolicy` returns a
   `*FieldError` (`task-143.md:581`), which the server answers 400 `bad_request` naming
   `harnesses` or `skills.library.commit` (task 144's `fieldProblem`, `task-144.md:317`). The
   scope wrote 422; the house mapping wins (gap 2).
5. **The gate is on the control plane, fail closed.** Creating a session on a worktree whose
   project is bound to a Hub team asks the Hub for that team's policy; a harness not on it answers
   403 `harness_not_allowed` with `{allowed}`. If the Hub cannot answer, the session is refused
   503 `agent_policy_unavailable`: a gate that opens when the Hub is down is not a gate. A project
   bound to no team (local-only, OD-20) has no team policy, and the gate does not apply.
6. **Skills are files, never flags.** `library` is fetched at its commit, read-only, into the
   harness's own user-level skills folder in the sandbox's home before the harness starts, never
   into the worktree (a staged file there would be committed with the work). `harness.Build`'s
   argv and env are byte-identical with and without skills (OD-8). `image` stages nothing.
7. **The commit is recorded.** The agent-start record (TM-14) carries `skillsCommit` beside the
   policy digest, `""` for `image`, so *which skills did this agent have* has one answer.
8. **The fragment passes through `Render` or is not posted.** `OrgFragment` joins task 161's
   `ConnectorFragment` into one org-layer envelope, then renders it for every class with
   `policy.Render`; any `InvariantError` is returned and nothing is posted (TM-13).

**Files:**
- `control-plane/internal/hub/hubapi/work_intake.go` ⚑ (task 143) — the request type and event
  kind below (appended)
- `control-plane/internal/hub/store/work_harness.go` (+ `_integration_test.go`) — read and upsert (new)
- `control-plane/internal/hub/server/work_harness.go` (+ `_integration_test.go`) — two routes (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — `workMatrixRows["166"]`
- `control-plane/internal/hub/hubclient/fake_work_intake.go` ⚑ (task 144) — the two methods
- `control-plane/internal/workpolicy/{harness,skills,fragment}.go` (+ `_test.go` each) — the pure
  rules, the staging and the fragment (new; the skeleton's File map has **no row** for this
  package, gap 3)
- `control-plane/internal/api/sessions.go` ⚑ — the gate, after the harness is resolved
  (`:784-794`, read 2026-09-26); `api.go` ⚑ — one optional dependency
- `control-plane/internal/httpx/problem.go` ⚑ and `problem_test.go` ⚑ — two codes

#### The wire — appended to task 143's `hubapi/work_intake.go` (complete)

`TeamAgentPolicy`, `TeamSkills` and `SkillsLibrary` are task 143's (`task-143.md:557-569`) and are
not restated. This task adds:

```go
// PutTeamAgentPolicyRequest is the PUT body: the two fields a team admin sets.
// The Hub stamps teamId, updatedBy and updatedAt.
type PutTeamAgentPolicyRequest struct {
	Harnesses []string   `json:"harnesses"`
	Skills    TeamSkills `json:"skills"`
}

// DefaultHarnesses is what a team with no policy row allows (decision 1).
var DefaultHarnesses = []string{"claude-code"}

// EventAgentPolicySet is audit-and-feed: data {harnesses, skillsSource, commit?}.
// commit is present only for a library. No repo credential is ever in it.
const EventAgentPolicySet = "work.agent_policy_set"
```

#### The store and the routes (signatures complete)

```go
package store

// TeamAgentPolicy reads the team's row, or the default (decision 1) with
// UpdatedBy and UpdatedAt nil when there is none. It never returns ErrNotFound
// for a team the caller's transaction can see.
func (tx *Tx) TeamAgentPolicy(ctx context.Context, teamID string) (hubapi.TeamAgentPolicy, error)

// PutTeamAgentPolicy upserts the row (task 145's table, `task-145.md:352-366`)
// and appends one work.agent_policy_set event in the same transaction. A PUT
// equal to the stored row writes nothing and returns changed=false.
func (tx *Tx) PutTeamAgentPolicy(ctx context.Context, by, teamID string,
	req hubapi.PutTeamAgentPolicyRequest) (p hubapi.TeamAgentPolicy, changed bool, err error)
```

```go
package server

// GET /v1/teams/{teamId}/work/agent-policy   viewer and up → 200 TeamAgentPolicy
// PUT /v1/teams/{teamId}/work/agent-policy   team admin    → 200 TeamAgentPolicy
//   ValidateTeamAgentPolicy(p, harness ids of harness.Catalogue()) first:
//   400 bad_request naming "harnesses" or "skills.library.commit" (decision 4)
// Anyone else: task 04's 404 for a team they cannot see, 403 for a member's PUT.
func (s *Server) routesWorkAgentPolicy(mux *http.ServeMux)

// catalogueIDs is the control plane's own list (harness.go:216, Catalogue()),
// read once at start; the Hub and control plane ship from one module.
func catalogueIDs() []string
```

The Hub imports `control-plane/internal/harness` for the ids only; it builds no argv and starts
no process. If Hub and control plane are ever released apart, the Hub's list would lag a new
harness by one release and refuse it until then: fail closed, and named in gap 4.

#### Interfaces — complete: `control-plane/internal/workpolicy/harness.go`

```go
// Package workpolicy is task 166's pure rules: which harness a team allows,
// where team skills go, and the org-layer fragment. It never builds an argv,
// never reads a harness's output and never edits a harness (OD-8).
package workpolicy

import (
	"slices"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Effective is the ordered list the gate uses. A nil policy (a team with no
// row) and a stored empty list both read as the default; the table's CHECK
// forbids the second, and this refuses to trust that.
func Effective(p *hubapi.TeamAgentPolicy) []harness.ID {
	src := hubapi.DefaultHarnesses
	if p != nil && len(p.Harnesses) > 0 {
		src = p.Harnesses
	}
	out := make([]harness.ID, 0, len(src))
	for _, s := range src {
		out = append(out, harness.ID(s))
	}
	return out
}

// Allowed reports whether id is on the team's list. Fleet's worker spawn
// (TM-24) calls this once Phase T merges (UNVERIFIED until then).
func Allowed(p *hubapi.TeamAgentPolicy, id harness.ID) bool {
	return slices.Contains(Effective(p), id)
}

// First is the team's default harness: a session that names none gets it
// (decision 2).
func First(p *hubapi.TeamAgentPolicy) harness.ID { return Effective(p)[0] }
```

#### Interfaces — complete: `control-plane/internal/workpolicy/fragment.go`

```go
package workpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/policy"
)

// OrgFragment wraps task 161's ConnectorFragment in TM-13's v1 envelope as an
// org layer, in the worker section only: Orchestrators are allowed
// Bash(panacea *) and Read and nothing else (TM-13's I8), so a connector list
// on them would be refused. It renders the layer for every class and returns
// the first error, so a fragment that breaks an invariant is never posted.
func OrgFragment(connectors []byte) ([]byte, error) {
	var worker map[string]any
	if err := json.Unmarshal(connectors, &worker); err != nil {
		return nil, fmt.Errorf("workpolicy: the connector fragment: %w", err)
	}
	env, err := json.Marshal(map[string]any{"v": 1, string(model.ClassWorker): worker})
	if err != nil {
		return nil, err
	}
	layer := policy.Layers{Org: policy.Layer{Scope: policy.ScopeOrg, Version: "pv-draft", Doc: env}}
	for _, c := range []model.Class{model.ClassWorker, model.ClassOrchestrator, model.ClassMirror} {
		if _, _, err := policy.Render(layer, c); err != nil {
			return nil, err // an *policy.InvariantError names the invariant (decision 8)
		}
	}
	return append(env, '\n'), nil
}
```

`encoding/json` sorts map keys, so the envelope's bytes are a function of the connector list
alone; `Render`'s digest (TM-13's) is what task 173 shows, never one computed here. The class
names (`model.ClassWorker`, `ClassOrchestrator`, `ClassMirror`) and the package paths are the
team worktree's `task-13.md:260-316`; they are **UNVERIFIED** on `feat/work-layer`, where
neither `internal/policy` nor `internal/fleet/model` exists yet (Phase T is unmerged).

#### Interfaces — complete: `control-plane/internal/workpolicy/skills.go`

```go
package workpolicy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Fetcher reads one path of a repository at one commit. The control plane's
// implementation reads the team's local clone (OD-2) with `git archive <commit>
// -- <path>`; a commit that is not in the clone is an error, never a fetch of a
// branch. Tests use a map.
type Fetcher interface {
	Files(ctx context.Context, repoKey, commit, path string) (map[string][]byte, error)
}

// SkillsDir is Claude Code's user-level skills folder, relative to the
// sandbox's home. UNVERIFIED at the pinned CLI (gap 5); the canary test pins it.
const SkillsDir = ".claude/skills"

// MaxSkillsBytes caps one library; a larger one is refused, not truncated.
const MaxSkillsBytes = 8 << 20

var ErrSkills = errors.New("workpolicy: skills refused")

// Stage copies a library's files under home/SkillsDir before the harness
// starts: files 0444, directories 0555, so the agent reads them and cannot
// rewrite what the record says it had. It stages nothing for "image", and
// nothing for any harness but Claude Code (L-159 is Claude Code's own skills).
// It returns the staged commit, "" when nothing was staged (decision 7).
// It never touches argv or env: harness.Build is called with the same Request
// either way (decision 6).
func Stage(ctx context.Context, f Fetcher, s hubapi.TeamSkills, id harness.ID, home string) (string, error) {
	if s.Source != "library" || id != harness.ClaudeCode {
		return "", nil
	}
	if s.Library == nil {
		return "", fmt.Errorf("%w: a library source with no library", ErrSkills)
	}
	files, err := f.Files(ctx, s.Library.RepoKey, s.Library.Commit, s.Library.Path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSkills, err)
	}
	root := filepath.Join(home, SkillsDir)
	total := 0
	for name, body := range files {
		total += len(body)
		if total > MaxSkillsBytes {
			return "", fmt.Errorf("%w: over %d bytes", ErrSkills, MaxSkillsBytes)
		}
		if !filepath.IsLocal(name) { // no "..", no absolute path, no drive letter
			return "", fmt.Errorf("%w: %q leaves the skills folder", ErrSkills, name)
		}
		dst := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, body, 0o444); err != nil {
			return "", err
		}
	}
	return s.Library.Commit, lockDirs(root)
}

// lockDirs sets every directory under root to 0555 after the files are in.
func lockDirs(root string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		return os.Chmod(p, 0o555)
	})
}
```

`Files` answers regular files only (the `git archive` reader skips symlinks and submodules), so
no staged path can point outside `root`; its names are relative to `Path`. `filepath.IsLocal` is
Go 1.20's, and `control-plane/go.mod` says `go 1.23` (read 2026-09-26). The home is the session's
own and new each start, so the 0555 folders never meet a second `Stage`.

**Where `Stage` runs.** The sandbox has no general mount list on this tree (`sandbox/manager.go`
mounts fixed volumes at `:192`), so the staged folder rides TM-15's policy mount, the volume that
already carries the rendered managed-settings file into the sandbox. Until Phase T merges, `Stage`
is built and tested as a function and called by nothing; its call site is TM-15's mount step, and
that wiring is **UNVERIFIED** (gap 6).

#### Interfaces — complete: the gate in `api.go` and `sessions.go`

```go
// api.go, in Deps (appended after Worktrees):

	// AgentPolicy is task 166's harness gate. Nil means no gate: a control
	// plane with no Hub. Unlike Projects, New accepts nil.
	AgentPolicy AgentPolicySource

// AgentPolicySource answers the Hub team policy for a local project. bound is
// false when the project is bound to no Hub team: the gate does not apply
// (decision 5). When bound, p is never nil (the Hub answers the default).
type AgentPolicySource interface {
	PolicyFor(ctx context.Context, projectID string) (p *hubapi.TeamAgentPolicy, bound bool, err error)
}
```

```go
// sessions.go, step 2 becomes (replacing :784-794):

	// 2. The harness, and the team's allow-list (task 166).
	allowed, gated, perr := a.harnessPolicy(ctx, req.WorktreeID)
	if perr != nil {
		a.sessionLog().Warn("agent policy unavailable", "worktreeId", req.WorktreeID, "err", perr)
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeAgentPolicyUnavailable,
			"The team's agent policy could not be read, so no session was started.", nil)
		return
	}
	harnessID := harness.ID(strings.TrimSpace(req.Harness))
	if harnessID == "" {
		harnessID = harness.ClaudeCode
		if gated {
			harnessID = allowed[0] // the team's first (decision 2)
		}
	}
	desc, known := harness.Lookup(harnessID)
	if !known {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnknownHarness,
			"No harness with that id.", map[string]string{"harness": string(harnessID)})
		return
	}
	if gated && !slices.Contains(allowed, harnessID) {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.CodeHarnessNotAllowed,
			"This team does not allow that harness.",
			map[string]string{"harness": string(harnessID), "allowed": joinIDs(allowed)})
		return
	}

// harnessPolicy is nil-safe: no source, or a project bound to no team, is not gated.
func (a *API) harnessPolicy(ctx context.Context, worktreeID string) ([]harness.ID, bool, error) {
	if a.deps.AgentPolicy == nil {
		return nil, false, nil
	}
	projectID, _ := a.sessionLocate(ctx, worktreeID)
	if projectID == "" {
		return nil, false, nil
	}
	p, bound, err := a.deps.AgentPolicy.PolicyFor(ctx, projectID)
	if err != nil || !bound {
		return nil, false, err
	}
	return workpolicy.Effective(p), true, nil
}

func joinIDs(ids []harness.ID) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = string(id)
	}
	return strings.Join(s, ",")
}
```

```go
// httpx/problem.go, appended to the const block (and to problem_test.go's table):
	CodeHarnessNotAllowed       = "harness_not_allowed"        // 403 the team does not allow it (task 166)
	CodeAgentPolicyUnavailable  = "agent_policy_unavailable"   // 503 the Hub could not answer (task 166)
```

`details` is `map[string]string` (`problem.go:93`), so `allowed` is the ids comma-joined, in the
team's order: `"claude-code,codex"`. `harness_not_allowed` is task 143's code string
(`task-143.md:783`), so the cockpit and the Hub switch on one value. `sessionLocate` is called a
second time later in the handler (`:896`); the double enumeration is a local read, and caching it
is not this task's.

#### Tests to write first — the gate (`control-plane/internal/api/sessions_policy_test.go`, new)

Over the existing one construction site, `sessionsNewEnvOverWith(t, dir, st, flags, tweak)`
(`sessions_test.go:593`), whose worktree is `w-4a77b1c9` in project `p-8c1d0e42` (`:602-608`),
and its `do` helper (`:664`). `fixedPolicy` is a four-line `AgentPolicySource` double.

```go
type fixedPolicy struct {
	p     *hubapi.TeamAgentPolicy
	bound bool
	err   error
}

func (f fixedPolicy) PolicyFor(context.Context, string) (*hubapi.TeamAgentPolicy, bool, error) {
	return f.p, f.bound, f.err
}

func policyEnv(t *testing.T, src AgentPolicySource) *sessionsEnv {
	t.Helper()
	st := sessionsMustStore(t)
	return sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) { d.AgentPolicy = src })
}

func create(harness string) string {
	return `{"worktreeId":"w-4a77b1c9","harness":"` + harness + `"}`
}

func TestAHarnessOffTheListIs403WithTheList(t *testing.T) {
	env := policyEnv(t, fixedPolicy{p: &hubapi.TeamAgentPolicy{Harnesses: []string{"claude-code"}}, bound: true})
	rec := env.do(t, "POST", "/v1/sessions", create("codex"))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"harness_not_allowed"`) ||
		!strings.Contains(rec.Body.String(), `"allowed":"claude-code"`) {
		t.Fatalf("codex on a claude-code team: %d %s", rec.Code, rec.Body)
	}
	if len(env.be.created()) != 0 {
		t.Fatal("a refused session reached the backend")
	}
}

func TestAnUnsetPolicyAllowsClaudeCodeOnly(t *testing.T) {
	env := policyEnv(t, fixedPolicy{p: &hubapi.TeamAgentPolicy{}, bound: true}) // no row: the default
	if rec := env.do(t, "POST", "/v1/sessions", create("claude-code")); rec.Code != http.StatusCreated {
		t.Fatalf("claude-code on an unset team: %d %s", rec.Code, rec.Body)
	}
	if rec := env.do(t, "POST", "/v1/sessions", create("codex")); rec.Code != http.StatusForbidden {
		t.Fatalf("codex on an unset team: %d, want 403 (unset is not all)", rec.Code)
	}
}

func TestNoHarnessNamedGetsTheTeamsFirst(t *testing.T) {
	env := policyEnv(t, fixedPolicy{p: &hubapi.TeamAgentPolicy{Harnesses: []string{"codex", "claude-code"}}, bound: true})
	rec := env.do(t, "POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"harness":"codex"`) {
		t.Fatalf("no harness named: %d %s, want the team's first, codex", rec.Code, rec.Body)
	}
}

func TestTheHubDownRefusesTheSession(t *testing.T) {
	env := policyEnv(t, fixedPolicy{err: errors.New("dial tcp: refused")})
	if rec := env.do(t, "POST", "/v1/sessions", create("claude-code")); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Hub down: %d, want 503 agent_policy_unavailable (fail closed)", rec.Code)
	}
}

func TestAnUnboundProjectIsNotGated(t *testing.T) {
	env := policyEnv(t, fixedPolicy{bound: false})
	if rec := env.do(t, "POST", "/v1/sessions", create("codex")); rec.Code != http.StatusCreated {
		t.Fatalf("a local-only project: %d, want 201", rec.Code)
	}
}
```

`env.be.created()` names the backend double's record of started specs (`sessionsNewBackend`,
`sessions_test.go:430`); its method name is **UNVERIFIED** and step 1 binds it. `harness:"codex"`
creates only because the test image's catalogue marks it available; if the double's catalogue
does not, the test uses the first harness the double does mark, and says so.

#### Tests to write first — `workpolicy` (unit, no Hub, no sandbox)

`harness_test.go`, `skills_test.go` and `fragment_test.go`, in package `workpolicy`. `mapFetcher`
is the `Fetcher` double: a `map[string]map[string][]byte` keyed by `repoKey+"@"+commit`, which
answers `errors.New("no such commit")` for any other key, so a branch name can never resolve.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNilAndEmptyReadAsClaudeCode` | `Effective(nil)` and `Effective(&{Harnesses: []})` are both `[claude-code]`; `Allowed(nil, "codex")` is false | unset means all |
| `TestTheOrderIsKept` | `Effective(&{Harnesses: [codex, claude-code]})` is `[codex, claude-code]` and `First` is `codex` | a sorted list, so `First` is always `claude-code` |
| `TestStageCopiesAtTheCommitReadOnly` | a library at `4b1d…` (40 hex) with `pay/SKILL.md` lands at `home/.claude/skills/pay/SKILL.md`, mode `0444`, its folder `0555`; returns `4b1d…` | a writable copy the agent can change after the record is made |
| `TestStageImageStagesNothing` | `source: image` returns `""` and `home/.claude` does not exist | a stale library staged under `image` |
| `TestStageOnlyForClaudeCode` | a library policy with `id = codex` returns `""` and stages nothing | skills written into another harness's config |
| `TestStageRefusesAnEscapingName` | a file named `../../.bashrc`, then `/etc/x`, then `C:\x`: each `errors.Is(err, ErrSkills)`, nothing written outside `home/.claude/skills` | a path traversal out of the skills folder |
| `TestStageRefusesAnOversizeLibrary` | two files of `MaxSkillsBytes/2 + 1` each: `ErrSkills`, not a truncated copy | a partial library recorded as the whole |
| `TestStageNeverResolvesABranch` | a library whose `Commit` is `main`: `mapFetcher` has no `main` key, so `ErrSkills` | a floating ref fetched, so what the agent loaded is unrecoverable |
| `TestBuildIsTheSameWithAndWithoutSkills` | `harness.Build(r)` before and after a library `Stage` into a temp home: `command` and `argv` are `reflect.DeepEqual`; and no field of `harness.Request` (by reflection) has `Skill` in its name | a skills flag or field injected into the harness (OD-8) |
| `TestTheFragmentPassesRender` | `OrgFragment(ConnectorFragment([a, b]))` succeeds; its bytes decode to `{"v":1,"worker":{…}}` with no `orchestrator` key | the list put on Orchestrators, which TM-13's I8 refuses |
| `TestAFragmentThatBreaksAnInvariantIsRefused` | a connector fragment carrying `"permissions":{"deny":["Bash(panacea *)"]}`: `OrgFragment` returns an `*policy.InvariantError` and no bytes | a fragment that drops `Bash(panacea *)` posted anyway |
| `TestTheFragmentIsDeterministic` | two orders of one connector list give equal bytes | a digest that moves with no change |

`TestBuildIsTheSameWithAndWithoutSkills` is the OD-8 guard. It runs `Build` on the real
descriptor, so it fails the day anyone adds a skills option to `claudeArgv` (`argv.go:80`), and
the reflection half fails the day anyone adds one to `Request` (`harness.go:105-113`, read
2026-09-26: `Harness`, `Model`, `Effort`, `Permission`, `OpeningPrompt`, `Conversation`, `Resume`).

`TestStageCopiesAtTheCommitReadOnly` reads modes with `os.Stat(...).Mode().Perm()`. On Windows,
Go reports `0444` for a read-only file and `0555` is not enforced for folders, so the folder half
of the assertion is behind `runtime.GOOS != "windows"`; CI runs on Linux, where the sandbox runs.

`TestAFragmentThatBreaksAnInvariantIsRefused` asserts `errors.As(err, &ie)` and `ie.Invariant ==
"I10"` (no deny may match a `panacea` command), never the message. `InvariantError.Invariant` and
I10 are TM-13's (team tree `task-13.md:83`, `:305-306`, and its own test of the same deny at
`:668`, read 2026-09-26); on `feat/work-layer` they are **UNVERIFIED** until Phase T merges.

#### Tests to write first — the Hub half

`store/work_harness_integration_test.go` and `server/work_harness_integration_test.go`, both
`//go:build integration`, over the Hub's existing test database and `devissuer` principals (task
04's harness; its helper names are bound in step 1, **UNVERIFIED** here). Team *Payments* has
Dylan as team admin and Priya as member.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnUnsetTeamReadsTheDefault` | `GET` → 200 `{harnesses:["claude-code"], skills:{source:"image", library:null}, updatedBy:null, updatedAt:null}`; no row written | a 404 for an unset team, or unset read as all |
| `TestOnlyATeamAdminSetsThePolicy` | Priya's `PUT` → 403 `forbidden`, no row, no event; Dylan's → 200 and one `work.agent_policy_set {harnesses, skillsSource:"image"}` | any member edits |
| `TestAnUnknownHarnessIs400` | `PUT {harnesses:["claude-code","nope"]}` → 400 `bad_request`, `details.field: harnesses`; `[]` → the same; `["codex","codex"]` → the same | free text, an empty list, a duplicate accepted |
| `TestABranchRefIs400` | `library.commit: "main"`, then 39 hex, then 40 upper-case hex → each 400, `details.field: skills.library` (task 143's `hex40`, `task-143.md:742`, `:751-760`); 40 lower-case hex → 200, the event's `commit` is it | a floating ref stored |
| `TestTheEventHoldsNoRepoCredential` | a `repoKey` of `github.com/kpmg/payments-skills`: the event's data has exactly `harnesses`, `skillsSource`, `commit`; no `repoKey`, no `path`, no URL | a repository address in the feed |
| `TestANoOpPutWritesNothing` | the same `PUT` twice → one row version, one event, the second answers `changed=false` | an event per call |
| `TestPoliciesAreTheTeamsOwn` | org B's admin `GET`s Payments → 404; `UPDATE work_team_agent_policies` across orgs as `hub_app` → 0 rows | RLS missing on task 145's table |
| `TestTheOrderIsStored` | `PUT [codex, claude-code]` then `GET` → the same order | a set, so the team's first is lost (decision 2) |

**The fake.** Task 144 stubbed `GetTeamAgentPolicy` and `PutTeamAgentPolicy` with 501
`not_in_fake_yet` (`task-144.md:126`, `:140` hold the `policies` map). This task fills them with
the same rules: the default for a missing key, team admin writes, `ValidateTeamAgentPolicy`, one
event per change. **Its refusal is the 400 above, not W5-17's 403** (gap 7). The field names
are task 143's validator's, `harnesses`, `skills.library` and `skills.source`, not decision 4's
`skills.library.commit` (gap 8); `details.field` is the key `fieldProblem` writes, bound in step 1.

#### The matrix rows (`workMatrixRows["166"]`, appended)

Columns V M A O N X, as task 161 (`task-161.md:579-581`). The policy is the team's, so an org
admin who is not a member (`O`) gets task 04's 404, as task 165's team rows do
(`task-165.md:330-331`): OD-16's admin governs through the team, never around it.

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/teams/{teamId}/work/agent-policy` | 200 | 200 | 200 | 404 | 404 | 404 |
| `PUT /v1/teams/{teamId}/work/agent-policy` | 403 | 403 | 200 | 404 | 404 | 404 |

The `PUT` row sends a valid body, so `A`'s 200 is the write and not the 400. The body is not read
before the membership check, so `O N X` see 404 whatever they send.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): task 143's `TeamAgentPolicy`,
  `ValidateTeamAgentPolicy` and `CodeHarnessNotAllowed`; task 144's fake map and `fieldProblem`;
  task 145's `work_team_agent_policies`; task 161's `ConnectorFragment`. Bind the names marked
  **UNVERIFIED** above (the backend double's `created()`, task 04's test principals). A table that
  differs from what the store needs is raised against 145, not patched here.
- [ ] **Step 2:** `harness_test.go` and `skills_test.go`; they fail to compile. Write `harness.go`
  and `skills.go` until their nine rows pass. `fragment.go` and its three rows wait for TM-13
  (`internal/policy` and `internal/fleet/model` do not exist on this branch): until Phase T
  merges, the package holds the first two files only, and the task is not closed.
- [ ] **Step 3:** The two codes in `httpx/problem.go` and its table. `sessions_policy_test.go`; it
  fails. The `Deps` field, `AgentPolicySource`, `harnessPolicy` and the replaced step 2 of the
  create handler, until the five rows pass and the existing `sessions_test.go` rows pass unchanged
  (a nil `AgentPolicy` is the old behaviour, decision 5).
- [ ] **Step 4:** The wire appends to `work_intake.go`. The Hub integration tests and the matrix
  rows; they fail. `store/work_harness.go`, `server/work_harness.go` and the route registration
  until green. Then the fake's two methods, and the W5-17 row changed to the 400 (gap 7).
- [ ] **Step 5:** Once Phase T merges: `fragment.go` and its rows; the call to `Stage` in TM-15's
  mount step with the returned commit on TM-14's agent-start record (decision 7, gap 6); the
  `workpolicy.Allowed` check in Fleet's `worker spawn` (TM-24). Each is its own commit.
- [ ] **Step 6:** `git diff -- go.work control-plane/go.mod` shows nothing (standard library
  only); commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/workpolicy/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# expect: 9 before Phase T merges; 12 after (the three fragment rows)
go test ./control-plane/internal/api/ -race -count=1 -v \
  -run '^Test(AHarnessOffTheListIs403WithTheList|AnUnsetPolicyAllowsClaudeCodeOnly|NoHarnessNamedGetsTheTeamsFirst|TheHubDownRefusesTheSession|AnUnboundProjectIsNotGated)$' \
  2>&1 | grep -c "^--- PASS"
# expect: 5
go test ./control-plane/internal/api/ ./control-plane/internal/httpx/ ./control-plane/internal/harness/ -race -count=1
# expect: ok for each; the existing session and argv tests unchanged
go test -tags integration ./control-plane/internal/hub/store/ ./control-plane/internal/hub/server/ -race -count=1 \
  -run 'AgentPolicy|UnsetTeam|UnknownHarness|BranchRef|RepoCredential|NoOpPut|PoliciesAreTheTeams|OrderIsStored|Matrix'
# expect: ok for both packages, no FAIL line
go list -deps ./control-plane/internal/workpolicy | grep -c "internal/hub/\(server\|store\)"
# expect: 0 (the control plane's gate never links the Hub's server)
git diff --stat -- go.work control-plane/go.mod
# expect: no output
```

#### What this task does NOT claim

- **That any harness but Claude Code runs.** The allow-list names catalogue ids; `codex`,
  `opencode` and `gemini-cli` are `Available() == false` on this tree until their flags are
  verified (`harness.go:96`, the unverified note at `:130`). Allowing one does not install it.
- **That `.claude/skills` is where the pinned CLI reads user skills** (gap 5), or that a staged
  skill is loaded: the canary test that proves it is phase 8's image work.
- **That the gate reaches Fleet or team agents.** `worker spawn` and managed sessions go through
  TM-24 and TM-15, wired in step 5 only once Phase T merges (**UNVERIFIED** until then).
- **That a managed-settings key pins MCP servers** at the pinned CLI (task 161's gap 8), or that
  Orchestrators use connectors: TM-13's I8 allows them `Bash(panacea *)` and `Read` only.
- **The desktop surface** (task 173) or the relay row (task 168).

**Parity rows served:** **L-159** and **L-180** closed; **L-197** and **L-256** with tasks 161
and 173.

#### Gaps found while writing this part

1. **Task 143's route table proposes `/v1/teams/{teamId}/agent-policy`** (`task-143.md:952`);
   this part, the scope and task 169 (`task-169.md:412-413`) say `…/work/agent-policy`. 143's
   row is the one to correct.
2. **The scope's 422 for a bad body** (`tasks-161-180.md:289-291`) is the house 400 here
   (decision 4). 05 and the scope should say 400.
3. **The File map has no row for `control-plane/internal/workpolicy/`.** It needs one, owned by
   this task.
4. **The Hub reads the control plane's harness catalogue at build time.** Released apart, the Hub
   would refuse a new harness for one release: fail closed, but nobody owns keeping them in step.
5. **`SkillsDir = ".claude/skills"` is UNVERIFIED at the pinned CLI.** The canary that proves
   the pinned Claude Code reads user skills there is phase 8's image work; no task names it.
6. **`Stage` has no call site on this branch.** It rides TM-15's policy mount, and the commit rides
   TM-14's agent-start record (decision 7); both are Phase T's and neither names a `skillsCommit`
   field. Step 5 waits for them.
7. **Task 144's W5-17 expects 403 `harness_not_allowed`** for an unknown id in a `PUT`
   (`task-144.md:537`, `:762`); this part answers 400 `bad_request`, `details.field: harnesses`
   (decision 4), and keeps 403 `harness_not_allowed` for the session gate only. Step 4 changes the
   row in `conformance/work.go` ⚑, a file the Files list above does not name. Task 168 runs W5-17
   over HTTP, so the two must agree before 168 starts.
8. **Decision 4 and the route comment say `skills.library.commit`;** task 143's validator names
   `skills.library` for a bad commit and `skills.source` for a bad source (`task-143.md:735-746`).
   143's names win; the tests above assert them.
9. **`AgentPolicySource` has no implementation here.** It needs the local project's Hub project
   (Hub task 35's `hublink.Resolver.Project`, hub tree `task-35.md:73-77`), that project's team,
   and `GetTeamAgentPolicy` over HTTP. Task 168 owns the HTTP method, so it owns the adapter and
   its wiring in `main.go`; until then `Deps.AgentPolicy` is nil and nothing is gated.
10. **The scope's `workpolicy.Allowed(team, id)`** (`tasks-161-180.md:279`) takes a team; this
    part's `Allowed(p, id)` takes the team's policy, so a Fleet caller reads the policy first.
    TM-24's part should say which.
