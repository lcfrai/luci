### Task 110: the read-only *ask* worker: answers that link files and lines

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 101 (the W3 verbs and their dispatch rows) and **TM-21** (`fleetbe` and the team launch on the
local control plane, Phase T plan `:636-643`, read at `64ab2c4`; **UNVERIFIED until merge**). Uses
TM-09's control wire v1 (`{ "v": 1, "id", "verb", "args" }` and its response, plan `:506-514`),
TM-20's `Launch(owner, project, class, harness, delegation)` and its gateway-key tags (`:625-634`),
TM-13's class invariants (`:546-557`), task 109's `Gate` (`OriginAsk`) and the local control plane's
`project.Runner` (`control-plane/internal/project/project.go:56-58`, built).

L-162 asks for grounded codebase questions (`05-design.md:324`: *"An **ask** worker: read-only
session on the repo; answers link files and lines"*). This part claims one thing: **every link in
an answer resolves**. It does not claim the answer is right.

**Decisions made here, so nobody re-derives them:**

1. **Team projects only.** The ask worker is a Fleet session, so it needs Fleet. In local mode
   the palette command is shown disabled with *"Ask needs a team project"*; L-162 for local
   projects is gap 5.
2. **A fourth class, `ask`.** TM-09's `Class` is `orchestrator`·`worker`·`mirror` (plan `:506-507`)
   and TM-13's defaults are `{worker,orchestrator,mirror}.json` (`:206`); none is read-only. This
   part adds `ask` to both, a delta whose owner phase wins (rule H2), with these invariants on top
   of TM-13's common ones: allowed tools `Read`, `Grep`, `Glob` and `Bash(panacea answer *)`;
   denied `Edit`, `Write`, `MultiEdit`, `NotebookEdit` and every other `Bash`; `defaultMode`
   pinned to `plan`. Fleet's verb table gives the `ask` class **one** verb, `answer`, and nothing
   else: no `lease`, no `report`, no `peer`.
3. **Nothing an ask session does can land.** Its checkout is a throwaway clone at the named
   commit, with no lane and no fetch-back (`OD-2`'s `refs/panacea/lanes/…` is never written for it).
   Even a write that got past the policy would reach nothing a person merges.
4. **The commit is named by the asker.** The desktop sends the commit its own clone has for the
   project's default branch remote-tracking ref, so the worker and the citation check read the
   same tree. If the remote lacks it, Fleet refuses with `commit_not_on_remote`.
5. **The answer comes back through one new verb**, `answer {answer, citations}`, callable once per
   ask session; the session ends after it. The scope names no return channel, and TM-18's
   `report done <text>` caps text at a length and has no typed citations, so this is a delta to
   TM-09's wire (gap 2). Fleet stores the answer for the asker only.
6. **It is the asker's session, billed to the asker.** Fleet launches it with `owner = asker`
   under the asker's delegation (TM-21 creates it as the user), so TM-20's key tags name the
   asker's `oid` (`OD-19`); the project owner is never named. It appears in the asker's own grid as
   `you · agent N`, with the word *ask* beside it.
7. **The control plane checks every citation before render.** On the local control plane, with the
   asker's clone, `git cat-file -e <commit>:<path>` then `git show <commit>:<path>` and a line count.
   A citation is `ok`, `missing_path`, `missing_lines` or `commit_absent`; only `ok` is a link. Any
   other renders as the fact *"not found at 7c1f0a2"* or *"not checked: 7c1f0a2 is not in this
   clone"*. No citation at all reads *"no file cited"*.
8. **An answer never lands on an issue by itself.** *Post as my comment* opens the issue's composer
   (task 50) with the answer quoted under a line `ask (agent) at 7c1f0a2:`; what the person posts
   is their own comment, `text·H`, and they may edit it first.
9. **The switch applies.** Launching an ask calls task 109's `gate.Allow` with `OriginAsk` first;
   with Orchestrators off the ask is refused with `agents_disabled` and the desktop shows
   `agentsOffLine`.

**Files:**
- `control-plane/internal/fleet/work/ask.go` (+ `_test.go`) — launch, the `answer` handler, the
  asker-only read (new; **path UNVERIFIED**, no Fleet row in the file map)
- `control-plane/internal/policy/defaults/ask.json` ⚑ (TM-13's directory) and TM-09's class and
  verb table in `control-plane/internal/fleet/model` ⚑ — the `ask` class (modified; **UNVERIFIED
  until merge**)
- `testdata/panacea-control/v1/answer.*.json` ⚑ (TM-09's fixture set) — the verb's fixtures (new)
- `control-plane/internal/workask/cite.go` (+ `_test.go`) — the citation check on the local clone
  (new; not in the file map)
- `control-plane/internal/api/workask.go` (+ `_test.go`), `api.go` ⚑, `routes_audit_test.go` ⚑ —
  the loopback routes (new, modified; not in the file map)
- `desktop/src/components/work/AskAnswer.tsx` (+ `.test.tsx`) — the question, the answer, the
  checked citations (new)
- `desktop/src/components/palette/commands.ts` ⚑ (task 59's) — *Ask about the code…* (modified)
- `desktop/src/api/work.ts` ⚑ — `ask` and `askResult` (modified)

#### Wire — complete

The loopback routes (desktop → local control plane), then Fleet's (through `fleetbe`, TM-21):

```text
POST /v1/work/ask            body AskRequest        → 202 {"askId":"ag-…","sessionId":"s-…"}
GET  /v1/work/ask/{ag}       (asker only)           → 200 AskResult
Fleet: POST /v1/projects/{pj}/asks  body AskRequest  → 202 {"agentId":"ag-…"}   (as the asker, via delegation)
Fleet: GET  /v1/projects/{pj}/asks/{ag}              → 200 {"state", "answer"?} (asker only; 404 for anyone else)
```

```json
{ "project": "pj-01J…", "commit": "7c1f0a2d9e…", "question": "Where is the lease breadth cap enforced?",
  "subject": { "issue": "is-01J…", "path": "control-plane/internal/fleet/leases" } }
```

```json
{ "state": "answered",
  "question": "Where is the lease breadth cap enforced?", "questionClass": "H",
  "answer": "The cap is checked when a lease is requested …", "answerClass": "A",
  "commit": "7c1f0a2d9e…",
  "citations": [
    { "path": "control-plane/internal/fleet/leases/glob.go", "lineStart": 41, "lineEnd": 58, "check": "ok" },
    { "path": "control-plane/internal/fleet/leases/cap.go", "lineStart": 1, "lineEnd": 9, "check": "missing_path" } ] }
```

`state` is `running`, `answered`, `ended_without_answer` or `refused` (with `code`). `check` is
added by the local control plane (decision 7) and is never sent by Fleet or the worker.

The new control verb, as TM-09 writes every verb (the fixture files are
`answer.request.json`, `answer.result.json` and `answer.refused.json`):

```json
{ "v": 1, "id": "7", "verb": "answer",
  "args": { "answer": "The cap is checked when …",
            "citations": [ { "path": "control-plane/internal/fleet/leases/glob.go", "lineStart": 41, "lineEnd": 58 } ] } }
```

```json
{ "id": "7", "ok": true, "result": { "stored": true, "sessionEnds": true } }
{ "id": "8", "ok": false, "error": { "code": "already_answered", "message": "one answer per ask" } }
```

Limits, checked by Fleet before storing (TM-05 §7.1's G2 and G3): `answer` at most 4,000
characters after character hygiene; at most 20 citations; `path` relative, no `..`, at most 300
characters; `1 ≤ lineStart ≤ lineEnd`; `lineEnd - lineStart < 400`. A violation is refused whole.

#### Interfaces — complete: `control-plane/internal/policy/defaults/ask.json` (TM-13's directory)

```json
{
  "permissions": {
    "defaultMode": "plan",
    "allow": ["Read", "Grep", "Glob", "Bash(panacea answer *)"],
    "deny": ["Edit", "Write", "MultiEdit", "NotebookEdit", "WebFetch", "WebSearch", "Bash"]
  }
}
```

TM-13's common invariants (`apiKeyHelper`, `allowManagedHooksOnly`, the `env` pin) are added by
its `Render` for every class and are not repeated here. Whether a `deny` of bare `Bash` leaves the
more specific `allow` of `Bash(panacea answer *)` working is **UNVERIFIED** against the pinned
Claude Code version: step 3 runs TM-13's render test and a one-command probe in TM-33's canary
sandbox and records the result. If the deny wins, `Bash` leaves the deny list and every other
command is refused by `defaultMode: plan` and Fleet's one-verb table instead.

#### Interfaces — complete: `control-plane/internal/fleet/work/ask.go`

```go
package work

import (
	"context"
	"errors"
	"strings"
)

type AskSubject struct {
	Issue string `json:"issue,omitempty"` // is-…; a lookup key only, never copied into the answer
	Path  string `json:"path,omitempty"`
}

type AskRequest struct {
	Project  string      `json:"project"`
	Commit   string      `json:"commit"`   // full 40-hex SHA from the asker's clone (decision 4)
	Question string      `json:"question"` // text·H, at most 1,000 characters
	Subject  *AskSubject `json:"subject,omitempty"`
}

type Citation struct {
	Path      string `json:"path"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
}

// AnswerArgs is the `answer` verb's args, written by the ask worker: text·A.
type AnswerArgs struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
}

type AskState string

const (
	AskRunning      AskState = "running"
	AskAnswered     AskState = "answered"
	AskEndedWithout AskState = "ended_without_answer"
	AskRefused      AskState = "refused"
)

type StoredAsk struct {
	AgentID, Owner, Project, Commit, Question string
	State                                     AskState
	Answer                                    *AnswerArgs
	Code                                      string // set when State is refused
}

// Caller is the authenticated person behind a Fleet route (TM-12), acting through their delegation.
type Caller struct{ OrgID, UserID, DelegationID string }

// Stamp is what TM-16 stamps on every control request and TM-19's dispatch hands a handler.
type Stamp struct{ AgentID, Class, OwnerID string }

// AskLauncher is TM-20's Launch with class "ask", a throwaway checkout at commit and no lane.
type AskLauncher interface {
	LaunchAsk(ctx context.Context, owner, delegation, project, commit, question string) (agentID string, err error)
}

type AskStore interface {
	Put(ctx context.Context, a StoredAsk) error
	Get(ctx context.Context, agentID string) (StoredAsk, error)
	// SetAnswer stores the first answer only; a second call reports already=true and changes nothing.
	SetAnswer(ctx context.Context, agentID string, a AnswerArgs) (already bool, err error)
}

var (
	ErrNotAskClass     = errors.New("class_forbidden")
	ErrAlreadyAnswered = errors.New("already_answered")
	ErrNotYours        = errors.New("not_found") // another person's ask reads as absent
	ErrBadAnswer       = errors.New("invalid_args")
	ErrBadRequest      = errors.New("invalid_request")
)

type Asks struct {
	Gate   Gate
	Launch AskLauncher
	Store  AskStore
}

// Start launches the asker's own ask session. The owner is always the caller (decision 6).
func (a Asks) Start(ctx context.Context, c Caller, req AskRequest) (string, error) {
	if len(req.Commit) != 40 || strings.TrimSpace(req.Question) == "" || len([]rune(req.Question)) > 1000 {
		return "", ErrBadRequest
	}
	if err := a.Gate.Allow(ctx, Turn{OrgID: c.OrgID, ProjectID: req.Project, OwnerID: c.UserID, Origin: OriginAsk}); err != nil {
		return "", err
	}
	id, err := a.Launch.LaunchAsk(ctx, c.UserID, c.DelegationID, req.Project, req.Commit, req.Question)
	if err != nil {
		return "", err
	}
	return id, a.Store.Put(ctx, StoredAsk{AgentID: id, Owner: c.UserID, Project: req.Project,
		Commit: req.Commit, Question: req.Question, State: AskRunning})
}

// Answer is the `answer` verb's handler, registered in TM-19's dispatch for class "ask" only.
func (a Asks) Answer(ctx context.Context, s Stamp, args AnswerArgs) error {
	if s.Class != "ask" {
		return ErrNotAskClass // belt and braces: TM-19's table refuses first
	}
	if err := ValidateAnswer(args); err != nil {
		return err
	}
	already, err := a.Store.SetAnswer(ctx, s.AgentID, args)
	if err != nil {
		return err
	}
	if already {
		return ErrAlreadyAnswered
	}
	return nil
}

// Read returns the ask to its asker; to anyone else it does not exist.
func (a Asks) Read(ctx context.Context, c Caller, agentID string) (StoredAsk, error) {
	got, err := a.Store.Get(ctx, agentID)
	if err != nil || got.Owner != c.UserID {
		return StoredAsk{}, ErrNotYours
	}
	return got, nil
}

func ValidateAnswer(a AnswerArgs) error {
	if n := len([]rune(a.Answer)); n == 0 || n > 4000 || len(a.Citations) > 20 {
		return ErrBadAnswer
	}
	for _, c := range a.Citations {
		if c.Path == "" || len(c.Path) > 300 || strings.HasPrefix(c.Path, "/") ||
			strings.Contains(c.Path, "..") || strings.Contains(c.Path, `\`) ||
			c.LineStart < 1 || c.LineEnd < c.LineStart || c.LineEnd-c.LineStart >= 400 {
			return ErrBadAnswer
		}
	}
	return nil
}
```

Character hygiene (G3) runs in TM-19's dispatch before any handler (Phase T plan `:619-621`
checks sizes and fields there); this handler adds the per-verb limits only. When the session ends
without an `answer`, TM-20's end record sets `State` to `ended_without_answer` through one added
line in its end path (**UNVERIFIED until merge**).
