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
   same tree. Fleet runs no git (TM-05 §7.1's G6), so if the remote lacks the commit the ask's
   checkout fails at launch and the ask ends `refused` with `checkout_failed`. How TM-20's launch
   gets a checkout at a named commit is **UNVERIFIED** (its `Launch` takes no commit; gap 3).
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

#### Interfaces — complete: `control-plane/internal/workask/cite.go` (the local control plane)

The local control plane does not import Fleet's packages; its types mirror the wire.

```go
// Package workask serves the ask routes on the local control plane and checks every citation
// against the asker's own clone before the desktop renders it (Phase W task 110).
package workask

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

type Check string

const (
	CheckOK           Check = "ok"
	CheckMissingPath  Check = "missing_path"
	CheckMissingLines Check = "missing_lines"
	CheckCommitAbsent Check = "commit_absent"
)

type Citation struct {
	Path      string `json:"path"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
	Check     Check  `json:"check,omitempty"`
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Checker runs read-only git in the asker's clone through phase 2's runner (project.go:56-58).
type Checker struct{ Git project.Runner }

// Check marks each citation; it never drops one and never reorders them.
func (c Checker) Check(ctx context.Context, repoDir, commit string, cites []Citation) []Citation {
	out := make([]Citation, len(cites))
	absent := !fullSHA.MatchString(commit)
	if !absent {
		_, err := c.Git.Run(ctx, repoDir, "cat-file", "-e", commit+"^{commit}")
		absent = err != nil
	}
	lines := map[string]int{} // one read per path; -1 means not a file at commit
	for i, ct := range cites {
		switch {
		case absent:
			ct.Check = CheckCommitAbsent
		case !safePath(ct.Path):
			ct.Check = CheckMissingPath
		default:
			n, seen := lines[ct.Path]
			if !seen {
				n = c.blobLines(ctx, repoDir, commit, ct.Path)
				lines[ct.Path] = n
			}
			switch {
			case n < 0:
				ct.Check = CheckMissingPath
			case ct.LineStart < 1 || ct.LineEnd < ct.LineStart || ct.LineEnd > n:
				ct.Check = CheckMissingLines
			default:
				ct.Check = CheckOK
			}
		}
		out[i] = ct
	}
	return out
}

func (c Checker) blobLines(ctx context.Context, dir, commit, path string) int {
	spec := commit + ":" + path // starts with 40 hex, so git never reads it as an option
	kind, err := c.Git.Run(ctx, dir, "cat-file", "-t", spec)
	if err != nil || strings.TrimSpace(string(kind)) != "blob" {
		return -1 // absent, or a directory or submodule
	}
	b, err := c.Git.Run(ctx, dir, "cat-file", "blob", spec)
	if err != nil {
		return -1
	}
	if len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte("\n"))
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

func safePath(p string) bool {
	return p != "" && len(p) <= 300 && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "-") &&
		!strings.Contains(p, "..") && !strings.Contains(p, `\`) && !strings.Contains(p, ":")
}
```

The check reads git objects only: no checkout, no working-tree file, so a file edited but
uncommitted in the asker's clone cannot make a citation look right.

#### Interfaces — complete: `control-plane/internal/api/workask.go`

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/workask"
)

// AskFleet is the part of TM-21's fleetbe the ask routes use, called as the signed-in user.
type AskFleet interface {
	StartAsk(ctx context.Context, project, commit, question string, subject json.RawMessage) (agentID, sessionID string, err error)
	ReadAsk(ctx context.Context, project, agentID string) (FleetAsk, error)
}

// FleetAsk is Fleet's GET body and, with the two class fields set here, the loopback AskResult.
type FleetAsk struct {
	State         string             `json:"state"`
	Code          string             `json:"code,omitempty"`
	Question      string             `json:"question"`
	QuestionClass string             `json:"questionClass"` // always "H"
	Answer        string             `json:"answer,omitempty"`
	AnswerClass   string             `json:"answerClass,omitempty"` // "A" whenever Answer is set
	Commit        string             `json:"commit"`
	Citations     []workask.Citation `json:"citations,omitempty"`
}

// AskClones maps a Hub project to the asker's local clone and its default remote-tracking commit
// (task 24's project mapping, SW.10; the name is this part's).
type AskClones interface {
	CloneDir(ctx context.Context, project string) (string, error)
	DefaultRemoteCommit(ctx context.Context, dir string) (string, error) // rev-parse refs/remotes/origin/HEAD^{commit}
}

type askHandler struct {
	fleet  AskFleet
	clones AskClones
	check  workask.Checker
}

func (h askHandler) start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Project  string          `json:"project"`
		Question string          `json:"question"`
		Subject  json.RawMessage `json:"subject,omitempty"`
	}
	if err := decodeStrict(r, &body); err != nil || !strings.HasPrefix(body.Project, "pj-") {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "project and question are required")
		return
	}
	dir, err := h.clones.CloneDir(r.Context(), body.Project)
	if err != nil {
		writeProblem(w, http.StatusConflict, "no_local_clone", "this project has no clone on this machine")
		return
	}
	commit, err := h.clones.DefaultRemoteCommit(r.Context(), dir)
	if err != nil {
		writeProblem(w, http.StatusConflict, "no_remote_default", "fetch the project once, then ask again")
		return
	}
	id, sid, err := h.fleet.StartAsk(r.Context(), body.Project, commit, body.Question, body.Subject)
	if err != nil {
		writeFleetProblem(w, err) // agents_disabled, invalid_request, Fleet down: Fleet's own code
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"askId": id, "sessionId": sid})
}

func (h askHandler) read(w http.ResponseWriter, r *http.Request) {
	project, id := r.URL.Query().Get("project"), r.PathValue("ag")
	got, err := h.fleet.ReadAsk(r.Context(), project, id)
	if err != nil {
		writeFleetProblem(w, err) // another person's ask is Fleet's 404, passed through
		return
	}
	got.QuestionClass = "H"
	if got.State == "answered" {
		got.AnswerClass = "A"
		for i := range got.Citations {
			got.Citations[i].Check = "" // never trust a check that arrived from upstream
		}
		if dir, err := h.clones.CloneDir(r.Context(), project); err == nil {
			got.Citations = h.check.Check(r.Context(), dir, got.Commit, got.Citations)
		} else {
			for i := range got.Citations {
				got.Citations[i].Check = workask.CheckCommitAbsent
			}
		}
	}
	writeJSON(w, http.StatusOK, got)
}
```

Registered in `api.go` as `POST /v1/work/ask` and `GET /v1/work/ask/{ag}` when `Deps.WorkAsk` is
set, with two catalog rows in `routes_audit_test.go`. `decodeStrict`, `writeProblem`, `writeJSON`
and `writeFleetProblem` stand for the package's existing helpers and TM-21's error mapping; step 1
greps `internal/api/*.go` for their real names and uses them. Fleet's own GET serialises
`StoredAsk` into the same flat shape (`answer` and `citations` beside `state`).

#### Interfaces — complete: appended to `desktop/src/api/work.ts`

```ts
export type CitationCheck = "ok" | "missing_path" | "missing_lines" | "commit_absent";
export interface AskCitation { path: string; lineStart: number; lineEnd: number; check: CitationCheck }
export interface AskRequest { project: string; question: string; subject?: { issue?: string; path?: string } }
export interface AskResult {
  state: "running" | "answered" | "ended_without_answer" | "refused";
  code?: string;
  question: string; questionClass: "H";
  answer?: string; answerClass?: "A";
  commit: string;
  citations?: AskCitation[];
}
export interface WorkAskApi {
  ask(req: AskRequest): Promise<{ askId: string; sessionId: string }>;
  askResult(project: string, askId: string): Promise<AskResult>;
}
```

#### Interfaces — complete: `desktop/src/components/work/AskAnswer.tsx`

```tsx
import type { AskCitation, AskResult } from "../../api/work";

export interface AskAnswerProps {
  result: AskResult;
  /** Opens a file at a line range in the Develop lens; only called for a checked citation. */
  openAt: (path: string, lineStart: number, lineEnd: number, commit: string) => void;
  /** Present only when the ask had an issue subject: opens that issue's composer, prefilled. */
  postAsComment?: (body: string) => void;
}

const short = (sha: string) => sha.slice(0, 7);
const range = (c: AskCitation) => (c.lineStart === c.lineEnd ? `${c.lineStart}` : `${c.lineStart}-${c.lineEnd}`);

function CitationRow({ c, commit, openAt }: { c: AskCitation; commit: string; openAt: AskAnswerProps["openAt"] }) {
  const label = `${c.path}:${range(c)}`;
  if (c.check === "ok") {
    return <li><a href="#" className="ask-cite" onClick={(e) => { e.preventDefault(); openAt(c.path, c.lineStart, c.lineEnd, commit); }}>{label}</a></li>;
  }
  const why = c.check === "commit_absent" ? `not checked: ${short(commit)} is not in this clone` : `not found at ${short(commit)}`;
  return <li className="ask-cite-fact"><span>{label}</span> · <span>{why}</span></li>;
}

export function AskAnswer({ result, openAt, postAsComment }: AskAnswerProps) {
  const cites = result.citations ?? [];
  return (
    <article className="ask" aria-label="Answer from your ask agent">
      <p className="ask-q" data-class="H">{result.question}</p>
      {result.state === "running" && <p className="ask-fact">ask agent working · no answer yet</p>}
      {result.state === "ended_without_answer" && <p className="ask-fact">the ask agent ended without an answer</p>}
      {result.state === "refused" && <p className="ask-fact" role="alert">not asked: {result.code}</p>}
      {result.state === "answered" && (
        <>
          <p className="ask-by">ask (agent) at <span className="ask-sha">{short(result.commit)}</span></p>
          <div className="ask-a" data-class="A">{result.answer}</div>
          {cites.length === 0
            ? <p className="ask-fact">no file cited</p>
            : <ul className="ask-cites">{cites.map((c, i) => <CitationRow key={i} c={c} commit={result.commit} openAt={openAt} />)}</ul>}
          {postAsComment && (
            <button type="button" onClick={() => postAsComment(
              `ask (agent) at ${short(result.commit)}:\n\n> ${(result.answer ?? "").split("\n").join("\n> ")}`)}>
              Post as my comment…
            </button>
          )}
        </>
      )}
    </article>
  );
}
```

The answer renders as plain text (`text·A`, no Markdown and no HTML), so an answer cannot draw a
link of its own; the only links are the checked citations. `.ask-cite`, `.ask-sha` and
`.ask-cite-fact` are Commit Mono at `--t-fact`. *"ask agent working · no answer yet"* is a state
word on the one heartbeat's redraw, never a spinner.

#### Interfaces — complete: the palette command (appended to task 59's `commands.ts`)

```ts
{
  id: "work.ask",
  title: "Ask about the code…",
  when: (ctx) => ctx.project !== null,
  disabledReason: (ctx) =>
    !ctx.project?.team ? "Ask needs a team project"
    : ctx.agentsOff ? ctx.agentsOff /* task 109's agentsOffLine */ : null,
  run: (ctx) => ctx.openAskDialog({ project: ctx.project!.id, subject: ctx.focusedIssue ? { issue: ctx.focusedIssue.id } : undefined }),
},
```

The command's field names follow task 59's `Command` type; step 1 renames here if task 59 named
them otherwise. No chord is bound: `Shift+A` is *Ask my Orchestrator about this* (task 102), a
different act.

#### Tests to write first — `control-plane/internal/workask/cite_test.go`

```go
package workask

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const sha = "7c1f0a2d9e0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"

// fakeGit answers the three git calls the checker makes from a map of path → content.
type fakeGit struct {
	files map[string]string
	calls int
}

func (g *fakeGit) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	g.calls++
	switch {
	case args[0] == "cat-file" && args[1] == "-e":
		if args[2] == sha+"^{commit}" {
			return nil, nil
		}
		return nil, errors.New("fatal: not a valid object")
	case args[0] == "cat-file" && args[1] == "-t":
		if _, ok := g.files[strings.TrimPrefix(args[2], sha+":")]; ok {
			return []byte("blob\n"), nil
		}
		return nil, errors.New("fatal: path does not exist")
	case args[0] == "cat-file" && args[1] == "blob":
		return []byte(g.files[strings.TrimPrefix(args[2], sha+":")]), nil
	}
	return nil, errors.New("unexpected git call: " + strings.Join(args, " "))
}

func TestCitationChecks(t *testing.T) {
	g := &fakeGit{files: map[string]string{"a/glob.go": strings.Repeat("x\n", 60)}}
	got := Checker{Git: g}.Check(context.Background(), "/repo", sha, []Citation{
		{Path: "a/glob.go", LineStart: 41, LineEnd: 58},
		{Path: "a/glob.go", LineStart: 59, LineEnd: 61},
		{Path: "a/cap.go", LineStart: 1, LineEnd: 9},
		{Path: "../etc/passwd", LineStart: 1, LineEnd: 1},
	})
	want := []Check{CheckOK, CheckMissingLines, CheckMissingPath, CheckMissingPath}
	for i, w := range want {
		if got[i].Check != w {
			t.Fatalf("citation %d: %s, want %s", i, got[i].Check, w)
		}
	}
}

func TestAnAbsentCommitMarksEveryCitation(t *testing.T) {
	got := Checker{Git: &fakeGit{}}.Check(context.Background(), "/repo", strings.Repeat("0", 40),
		[]Citation{{Path: "a.go", LineStart: 1, LineEnd: 1}})
	if got[0].Check != CheckCommitAbsent {
		t.Fatalf("got %s", got[0].Check)
	}
}

func TestAShortOrOptionShapedCommitRunsNoGit(t *testing.T) {
	g := &fakeGit{}
	Checker{Git: g}.Check(context.Background(), "/repo", "--output=/tmp/x", []Citation{{Path: "a.go", LineStart: 1, LineEnd: 1}})
	if g.calls != 0 {
		t.Fatalf("git ran %d times on an unvalidated commit", g.calls)
	}
}

func TestOneReadPerPath(t *testing.T) {
	g := &fakeGit{files: map[string]string{"a.go": "1\n2\n3\n"}}
	Checker{Git: g}.Check(context.Background(), "/repo", sha, []Citation{
		{Path: "a.go", LineStart: 1, LineEnd: 1}, {Path: "a.go", LineStart: 2, LineEnd: 3}})
	if g.calls != 3 { // cat-file -e, then -t and blob once for a.go
		t.Fatalf("calls = %d, want 3", g.calls)
	}
}
```

#### Tests to write first — `control-plane/internal/fleet/work/ask_test.go`

```go
package work

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recLauncher struct{ owner, delegation string; n int }

func (l *recLauncher) LaunchAsk(_ context.Context, owner, dl, _, _, _ string) (string, error) {
	l.owner, l.delegation, l.n = owner, dl, l.n+1
	return "ag-ask1", nil
}

type memAsks struct{ m map[string]StoredAsk }

func (s *memAsks) Put(_ context.Context, a StoredAsk) error { s.m[a.AgentID] = a; return nil }
func (s *memAsks) Get(_ context.Context, id string) (StoredAsk, error) {
	a, ok := s.m[id]
	if !ok {
		return a, errors.New("absent")
	}
	return a, nil
}
func (s *memAsks) SetAnswer(_ context.Context, id string, ans AnswerArgs) (bool, error) {
	a := s.m[id]
	if a.Answer != nil {
		return true, nil
	}
	a.Answer, a.State = &ans, AskAnswered
	s.m[id] = a
	return false, nil
}

const commit40 = "7c1f0a2d9e0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"

func newAsks(on bool) (Asks, *recLauncher, *memAsks) {
	l, s := &recLauncher{}, &memAsks{m: map[string]StoredAsk{}}
	return Asks{Gate: Gate{Switch: &fakeSwitch{on: on}, Next: &countQueue{}, Refused: &refusals{}}, Launch: l, Store: s}, l, s
}

var alice = Caller{OrgID: "or-1", UserID: "u-alice", DelegationID: "dl-a"}
var q = AskRequest{Project: "pj-dylans", Commit: commit40, Question: "where is the cap?"}

func TestTheAskIsTheAskersEvenOnSomeoneElsesProject(t *testing.T) {
	a, l, _ := newAsks(true)
	if _, err := a.Start(context.Background(), alice, q); err != nil {
		t.Fatal(err)
	}
	if l.owner != "u-alice" || l.delegation != "dl-a" {
		t.Fatalf("launched as %s/%s, want the asker's own", l.owner, l.delegation)
	}
}

func TestOrchestratorsOffRefusesTheAsk(t *testing.T) {
	a, l, _ := newAsks(false)
	if _, err := a.Start(context.Background(), alice, q); !errors.Is(err, ErrAgentsDisabled) || l.n != 0 {
		t.Fatalf("err=%v launches=%d", err, l.n)
	}
}

func TestOnlyTheAskClassMayAnswerAndOnlyOnce(t *testing.T) {
	a, _, _ := newAsks(true)
	id, _ := a.Start(context.Background(), alice, q)
	ans := AnswerArgs{Answer: "in glob.go", Citations: []Citation{{Path: "a/glob.go", LineStart: 41, LineEnd: 58}}}
	if err := a.Answer(context.Background(), Stamp{AgentID: id, Class: "worker"}, ans); !errors.Is(err, ErrNotAskClass) {
		t.Fatalf("worker answered: %v", err)
	}
	if err := a.Answer(context.Background(), Stamp{AgentID: id, Class: "ask"}, ans); err != nil {
		t.Fatal(err)
	}
	if err := a.Answer(context.Background(), Stamp{AgentID: id, Class: "ask"}, ans); !errors.Is(err, ErrAlreadyAnswered) {
		t.Fatalf("second answer: %v", err)
	}
}

func TestAnotherPersonCannotReadTheAsk(t *testing.T) {
	a, _, _ := newAsks(true)
	id, _ := a.Start(context.Background(), alice, q)
	if _, err := a.Read(context.Background(), Caller{UserID: "u-dylan"}, id); !errors.Is(err, ErrNotYours) {
		t.Fatalf("project owner read Alice's ask: %v", err)
	}
}

func TestAnswerLimits(t *testing.T) {
	bad := []AnswerArgs{
		{Answer: ""},
		{Answer: strings.Repeat("x", 4001)},
		{Answer: "x", Citations: []Citation{{Path: "../x", LineStart: 1, LineEnd: 1}}},
		{Answer: "x", Citations: []Citation{{Path: "a.go", LineStart: 9, LineEnd: 3}}},
		{Answer: "x", Citations: make([]Citation, 21)},
	}
	for i, b := range bad {
		if err := ValidateAnswer(b); !errors.Is(err, ErrBadAnswer) {
			t.Fatalf("case %d passed", i)
		}
	}
}
```

`fakeSwitch`, `countQueue` and `refusals` are task 109's test doubles in the same package.

**Rows added to Phase T's own tests** (their files are TM-09's and TM-13's, **UNVERIFIED until
merge**): TM-09's verb-table test gains *the `ask` class may call `answer` and nothing else*, run
over every verb in the table (a `lease request`, `report done` or `peer` from an `ask` session is
refused with the class error); TM-13's render test gains *the `ask` class's file denies `Edit`,
`Write`, `MultiEdit` and `NotebookEdit`, pins `defaultMode: plan`, and a project layer that adds
`Edit` to `allow` fails `Render` naming the invariant*; TM-09's fixture round-trip picks up the
three `answer.*.json` files by glob. TM-20's key-tag test gains *an ask launched by Alice on
Dylan's project tags `oid` Alice*.

#### Tests to write first — `desktop/src/components/work/AskAnswer.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { AskResult } from "../../api/work";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { AskAnswer } from "./AskAnswer";

const SHA = "7c1f0a2d9e0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d";
const answered = (over: Partial<AskResult> = {}): AskResult => ({
  state: "answered", question: "where is the cap?", questionClass: "H",
  answer: "It is checked in glob.go.", answerClass: "A", commit: SHA,
  citations: [
    { path: "a/glob.go", lineStart: 41, lineEnd: 58, check: "ok" },
    { path: "a/cap.go", lineStart: 1, lineEnd: 9, check: "missing_path" },
  ], ...over,
});

describe("AskAnswer", () => {
  it("links only a checked citation and states the rest as a fact", () => {
    const openAt = vi.fn();
    const { container } = render(<AskAnswer result={answered()} openAt={openAt} />);
    expect(container.querySelectorAll("a").length).toBe(1);
    fireEvent.click(screen.getByText("a/glob.go:41-58"));
    expect(openAt).toHaveBeenCalledWith("a/glob.go", 41, 58, SHA);
    expect(screen.getByText("not found at 7c1f0a2")).toBeTruthy();
    expectNoPrediction(container);
  });
  it("says no file cited when there is none", () => {
    render(<AskAnswer result={answered({ citations: [] })} openAt={vi.fn()} />);
    expect(screen.getByText("no file cited")).toBeTruthy();
  });
  it("says a citation was not checked when the commit is not in this clone", () => {
    render(<AskAnswer result={answered({ citations: [{ path: "a.go", lineStart: 1, lineEnd: 1, check: "commit_absent" }] })} openAt={vi.fn()} />);
    expect(screen.getByText("not checked: 7c1f0a2 is not in this clone")).toBeTruthy();
  });
  it("renders the answer as text, so an answer cannot make a link of its own", () => {
    const { container } = render(<AskAnswer openAt={vi.fn()}
      result={answered({ answer: "see <a href='https://x.test'>here</a> and [y](https://y.test)", citations: [] })} />);
    expect(container.querySelectorAll("a").length).toBe(0);
    expect(container.querySelector("[data-class='A']")?.textContent).toMatch(/<a href=/);
  });
  it("posts nothing by itself; the person's composer opens with the answer quoted", () => {
    const post = vi.fn();
    render(<AskAnswer result={answered()} openAt={vi.fn()} postAsComment={post} />);
    expect(post).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Post as my comment…" }));
    expect(post.mock.calls[0][0]).toBe("ask (agent) at 7c1f0a2:\n\n> It is checked in glob.go.");
  });
  it("marks the question as the person's and the answer as the agent's", () => {
    const { container } = render(<AskAnswer result={answered()} openAt={vi.fn()} />);
    expect(container.querySelector(".ask-q")?.getAttribute("data-class")).toBe("H");
    expect(container.querySelector(".ask-a")?.getAttribute("data-class")).toBe("A");
  });
});
```

The loopback route's test (`internal/api/workask_test.go`) holds two rows: a `check` field sent
by the fake Fleet is cleared and re-derived (mutant: an upstream `ok` trusted), and a project with
no local clone marks every citation `commit_absent` (mutant: unchecked citations shown as links).

| Test | Mutant it kills |
|---|---|
| citation checks; absent commit; one read per path | unchecked citations; a directory cited as a file; a git read per citation |
| option-shaped commit runs no git | an argument passed to git unvalidated |
| a checked link only; *not found at …* | a failed citation rendered as a link |
| *no file cited* | an uncited answer that looks grounded |
| answer as text | an answer that draws its own link past the check |
| posts nothing by itself | an answer landing on an issue without a person |
| the asker's own launch | billed to the project owner (`OD-19`), or run as them |
| switch off refuses | the ask bypasses task 109's gate |
| ask class only, once | a worker or Orchestrator writes an answer; a second answer overwrites the first |
| another person cannot read it | an ask visible to the project owner |
| the ask class's policy and verb table (Phase T rows) | an ask session that can write, lease or report |
| upstream `check` cleared | Fleet or the worker marks its own citations `ok` |

#### Steps

- [ ] **Step 1: read on the sync base** TM-09's `model` (the `Class` enum and the verb table),
  TM-13's `defaults/` and render test, TM-19's dispatch registration, TM-20's `Launch` and end
  path, TM-21's `fleetbe` and its error mapping, task 24's project mapping, task 59's `Command`
  type, and the helper names in `control-plane/internal/api/*.go`. Rename in this part's files only.
- [ ] **Step 2: the Go tests first** (`cite_test.go`, `ask_test.go`, `workask_test.go`) and the
  Phase T rows; each fails on the missing code.
- [ ] **Step 3: the `ask` class.** `defaults/ask.json`, the class value and its one-verb row, the
  three fixtures. Run TM-13's render test, then the one-command probe from decision 2's note in
  TM-33's canary sandbox, and write the result into this part's gap 1 as *observed*, with the date
  and the pinned CLI version. No CLI is modified or parsed (`OD-8`): the probe reads the exit
  status of `panacea answer` and of one denied `Write`.
- [ ] **Step 4: `ask.go`**, its registration in TM-19's dispatch and the Fleet routes; then
  `cite.go` and `workask.go` with `api.go`'s registration and the two catalog rows.
- [ ] **Step 5: the desktop test first**, then `work.ts`'s append, `AskAnswer.tsx` and the palette
  command; the dialog that asks the question reuses task 59's palette input.
- [ ] **Step 6: run the acceptance commands.** Commit by path after steps 3, 4 and 5, each message
  ending with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/workask/ ./control-plane/internal/fleet/work/ -race -count=1 -v -run 'Citation|Commit|OneRead|Ask|Answer|Another'
# expect: cite 4 and ask 5 tests, all PASS
go test ./control-plane/internal/api/ -race -count=1 -run 'WorkAsk|RoutesAudit'
# expect: ok; the catalog lists POST /v1/work/ask and GET /v1/work/ask/{ag}
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
npm --prefix desktop test -- work/AskAnswer palette
# expect: every file passes; AskAnswer adds 6 tests
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "dangerouslySetInnerHTML|marked|markdown" desktop/src/components/work/AskAnswer.tsx
# expect: no output
```

The counts are this part's plan, not a run.

#### What this task does NOT claim

- Answer quality. It claims only that every link in an answer resolves at the named commit.
- That the `ask` class's policy works against the pinned Claude Code version until step 3's probe
  has run and its result is written down (gap 1).
- That TM-20's launch can check out a named commit into a throwaway clone (gap 3).
- That an ask costs any particular amount; usage is attributed to the asker, never predicted.
- Grounded Q&A in local mode (decision 1, gap 5).
- That the question text is safe to hand to a model: it is the asker's own `text·H` to their own
  agent, and hostile repository content read by the ask worker is the injection corpus's question
  (05 §3.4, Q3; **UNVERIFIED**).

**Parity** (`05-design.md` rows). Closes **L-162** (`:324`) for team projects.

#### Gaps found while writing this part

1. **`deny: ["Bash"]` beside `allow: ["Bash(panacea answer *)"]`** is unproven on the pinned CLI;
   step 3 settles it and records the answer here.
2. **A new control verb, `answer`, and a new class, `ask`,** are deltas to TM-09's wire and verb
   table and TM-13's defaults (rule H2). The skeleton's delta table (`:287`) lists *"three verbs"*
   for Phase T; it should list the fourth and the class.
3. **TM-20's `Launch(owner, project, class, harness, delegation)` takes no commit**, and TM-15's
   classes say nothing of a throwaway checkout. The ask needs both; a delta for TM-15 and TM-20.
4. **Fleet's store needs an ask table** (asker, commit, question, answer, state), an additive
   migration in TM-10's `fleet` schema, kept for a stated time. This part leaves the time to the
   Phase T owner and names it as a gap, not a number.
5. **Local projects have no ask.** L-162 closes for team projects only unless task 112's solo
   runner can host an ask session too, which its spike could record.
6. **The file map has no row** for `control-plane/internal/workask/`, `internal/api/workask.go`,
   `components/work/AskAnswer.tsx` or Fleet's `work/` directory.

