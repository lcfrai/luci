### Task 154: outside PRs, part 1: host webhooks in from Azure DevOps and GitHub; commit links; closing and non-closing magic words

W5, stream WH, seam **SW.9** consumed, about 3 hours (an estimate, not a measurement). Depends on
task 21 (the `Panacea-Issue` trailer, `TrailerIssue`, `ValidIssueTrailer`, `task-21.md:88-96`), task
22 (`IssueFacts.Landed` and `hubapi.StateMove{To, Cause, Reason, LandCommit, Evidence}`,
`task-22.md:77`, `:135-137`, `:267-270`) and task 145 (the `host` actor kind, the three outside
causes and the guard that admits them; **scope only, no part on this tree**,
`…-scopes/tasks-141-160.md:230-276`, names **UNVERIFIED**). Reads task 07's guard and causes
(`task-07.md:207-227`), task 08's `SetProjectKey` and `project.key_set {key, previousKey}`
(`task-08.md:201`, `:282-285`), task 143's `OutsidePRLink`, `OutsideCommitLink`, `HostHook`,
`CreatedHostHook` (scope `tasks-141-160.md:153-158`, no part) and task 62's Desk *since* band,
where §3.7's informs land (scope `tasks-61-80.md:73-76`, no part).

Closes, with task 155, the outside-PR halves of **L-199** (05 §1: *"For PRs made outside the
product, on both hosts"*), **L-200** (*"host webhooks cover outside commits"*) and **L-201**
(*"host PR stages apply to outside PRs"*), on both hosts as `OD-4` requires.

**What this task must not get wrong:** a host is not a person, so it moves a fact only **as** a
mapped person and never to `done`; an unsigned or replayed call writes nothing; and an old alias
still finds its issue. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **Both hosts normalise into one `HostEvent`** before anything is written. `apply` sees only that
   shape, so every rule below is written once and task 155 extends one function, not two.
2. **Ingress lives outside `/v1`.** `POST /hooks/github/{projectId}` and `POST /hooks/ado/{projectId}`
   are called by a host, not an Entra principal, so they sit beside `/v1` as task 80's `/p/` pages
   do: a proposed Hub-contract addition for the `feat/hub` owner (the route table says *"Nothing
   else is routed"*, `hub-contract.md:609`, quoted in the skeleton's rule H1).
3. **Authentication before parsing.** The body is read to at most 1 MiB + 1 byte. Over the cap, a
   missing hook, or a failed check is **401 with nothing stored** and the same body
   (`{"code":"unauthenticated"}`) for all three, so a caller cannot tell which failed. GitHub:
   `X-Hub-Signature-256` is `sha256=` and the hex HMAC-SHA256 of the raw body under the hook's
   secret, compared with `hmac.Equal`. Azure DevOps: HTTP basic auth, user `panacea`, password the
   hook's secret, compared in constant time, and the request must arrive over https (the edge's
   `X-Forwarded-Proto: https`; plain http is 401). A GitHub push over 1 MiB is therefore dropped;
   GitHub's own payload ceiling is larger (**UNVERIFIED** by how much), and the delivery shows as
   failed in the repository's hook log, where its owner can see it.
4. **A delivery is written once.** GitHub's `X-GitHub-Delivery` and Azure DevOps' event `id` are
   the delivery id. `(project, host, deliveryId)` is inserted first in the same transaction as the
   effects; a conflict answers `200 {"duplicate": true}` and writes nothing. Rows older than 7 days
   are deleted by the janitor (one statement, task 150's sync step's neighbour). A redelivery after
   7 days is a new delivery; its links are unique already (task 145's indexes), so it writes none.
5. **The words are the git hosts' own**, closing `close closes closed fix fixes fixed resolve
   resolves resolved` and non-closing `refs`, `part of`, `related to`, case-insensitive, each
   followed by one or more references `EP-142` or `is-…`, separated by `,` or `and`. A reference
   with no word before it is ignored. `ParseRefs` is pure and knows nothing of the Hub.
6. **Aliases resolve at receipt, through every key the project has had.** `EP-142` resolves through
   the project's current key and every previous key (the gap below: task 08 records only the latest
   `previousKey` in its event, so this task adds `project_key_history`). The stored link holds the
   `is-` id, never the alias (skeleton rule 1). An alias that resolves to nothing, or to an issue in
   another project, is skipped silently: the hook never fails on a word.
7. **A `Panacea-Issue` trailer on an outside commit is a link, never a fact.** Only our land writes
   the trailers that move facts (SW.9); a trailer in an outside push could be typed by anyone.
8. **What each event does**, with `hub.work_actor_kind = 'host'`, `hub.work_actor_user` the mapped
   person and `hub.work_cause` one of task 145's three outside causes (a gap if 143 names them
   otherwise):
   - a pushed commit naming an issue → one `OutsideCommitLink` per `(repo, sha, issue)`;
   - a PR opened or edited naming an issue → one `OutsidePRLink`, and if the issue is before
     `in_review` (`triage`, `backlog`, `todo`, `started`) the `in_review` fact, cause `outside_pr`;
   - a PR merged whose title or body holds a **closing** word for an issue → `landed`, cause
     `outside_merge`, `LandCommit` the merge commit, so task 23's done rule reads the same inputs as
     for our land;
   - a non-closing word only links; a PR closed without merge sets the link's `prState: "closed"`.
9. **Never `done`.** An issue with empty `serves` that lands this way stops at `landed` until its
   assignee's control plane evaluates it (task 23) or a person moves it. 05 §3.3 says an issue with
   no criteria *"turns `done` at `landed`"*; for an outside land this task deliberately does not, and
   the difference is recorded for the owner.
10. **Identity, or nothing.** Azure DevOps names an identity ref; the Hub matches its `uniqueName`
    to a Hub user's Entra UPN only when the hook's organisation is backed by the Hub org's tenant
    (checked at registration; **UNVERIFIED** that `uniqueName` is the UPN in every organisation).
    GitHub names a login, matched only through an Enterprise Managed Users identity link
    (**UNVERIFIED** that one is readable). No match → links only, no fact, and one metadata-only
    event `issue.outside_unmatched {issueId, host, prNumber}` to the assignee, which task 62's Desk
    shows in its *since* band as *"EP-142 was merged outside the product by a GitHub user we cannot
    match: move it yourself"*. The event carries no login.

**Files:**

- `control-plane/internal/hub/server/work_hooks.go` — registration, the two ingress handlers,
  normalising to `HostEvent`, and `apply` (new)
- `control-plane/internal/hub/server/work_hooks_words.go` — `ParseRefs`, pure (new; **not in the
  File map**, whose row names only `server/work_hooks.go`: a gap)
- `control-plane/internal/hub/server/work_hooks_words_test.go` — the parser's tests (new)
- `control-plane/internal/hub/store/work_hooks.go` — hooks, deliveries, links, alias resolution (new)
- `control-plane/internal/hub/server/work_hooks_integration_test.go` — `-tags integration` (new)
- `control-plane/internal/hub/server/work_matrix_rows.go` ⚑ (task 07) — the registration route's
  rows and the *as a host webhook* fixture's rows (modified)
- `control-plane/internal/hub/server/server.go` ⚑ — `s.registerWorkHooks(mux)` beside task 15's
  `registerWorkImports` (modified)

**Interfaces:**

- Produces: `PUT /v1/projects/{projectId}/hooks/{host}` → `CreatedHostHook`; `GET
  /v1/projects/{projectId}/outside-links` → task 143's `ListOutsidePRLinks`; `POST
  /hooks/github/{projectId}`, `POST /hooks/ado/{projectId}`; `server.HostEvent`, `server.ParseRefs`,
  `server.Ref`; the event kind `issue.outside_unmatched`
- Consumes: task 143's `HostHook`, `CreatedHostHook`, `OutsidePRLink`, `OutsideCommitLink`,
  `ActorKindHost`; task 145's causes `outside_push`, `outside_pr`, `outside_merge` and the settings
  `hub.work_actor_kind`, `hub.work_actor_user`; task 22's `StateMove`; task 21's `TrailerIssue`;
  task 13's `emitWork`; Hub task 14's `WriteLimit`, `Decode`, `ProjectRole`, `Require`
- Consumed by: task 155 (linkbacks, the filing word, the first push), task 168 (end to end), task
  172 (the admin surface lists hooks and links; **UNVERIFIED** that 172 is the one)

#### The columns — for task 145's migration (complete SQL; **not** applied by this task)

Task 145's table list names `work_host_hooks`, `work_outside_pr_links` and
`work_outside_commit_links` but neither of these two, which this task needs:

```sql
-- Decision 4: a delivery is written once; the janitor deletes rows older than 7 days.
CREATE TABLE work_hook_deliveries (
  org_id      text        NOT NULL,
  project_id  text        NOT NULL,
  host        text        NOT NULL CHECK (host IN ('azure_devops', 'github')),
  delivery_id text        NOT NULL CHECK (char_length(delivery_id) BETWEEN 1 AND 200),
  received_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, project_id, host, delivery_id)
);
ALTER TABLE work_hook_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY work_hook_deliveries_org ON work_hook_deliveries
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT, DELETE ON work_hook_deliveries TO hub_app;

-- Decision 6: every key a project has had, so an old alias still resolves.
CREATE TABLE project_key_history (
  org_id      text        NOT NULL,
  project_id  text        NOT NULL,
  key         text        NOT NULL,
  retired_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, project_id, key)
);
ALTER TABLE project_key_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_key_history_org ON project_key_history
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON project_key_history TO hub_app;
```

Task 08's `SetProjectKey` (`task-08.md:285`) already returns `previous`; its caller inserts
`(project, previous)` into `project_key_history` in the same transaction when `changed` and
`previous != ""` (a one-line delta on task 08's route, recorded here as a gap for that owner). A
key freed by one project and taken by another in the same team resolves to the **current** holder
first, then history; the ambiguity is recorded, not solved. The policy's `hub.org_id` follows
`hub-contract.md:1904` as task 145's scope cites it.

#### Wire shapes

```text
PUT /v1/projects/pj-…/hooks/github        (team admin)
  {"remoteUrl": "https://github.com/acme/web"}
  ← 201 {"hook": {"projectId": "pj-…", "host": "github", "remoteUrl": "https://github.com/acme/web",
          "secretVersion": 1, "createdBy": {…}},
         "secret": "<shown once>", "url": "https://<hub>/hooks/github/pj-…"}
  ← 409 {"code": "conflict", "field": "remoteUrl"} when remoteUrl is not the project's remote
  A second PUT rotates: secretVersion 2, a new secret shown once, the old one refused from then on.

POST /hooks/github/pj-…      X-GitHub-Event: pull_request · X-GitHub-Delivery: 7c1e… ·
                             X-Hub-Signature-256: sha256=<hex>
  ← 200 {"duplicate": false, "links": 1, "facts": 1}   (counts only; never a title)
  ← 401 {"code": "unauthenticated"}                    (cap, missing hook, bad signature: one answer)
```

`CreatedHostHook` gains `url` beside task 143's `{hook, secret}` (a gap for 143): the scope says
*"the URL and the secret shown once"*.

#### Interfaces — complete: `server/work_hooks_words.go`

```go
package server

import (
	"regexp"
	"strings"
)

// Ref is one reference ParseRefs found: exactly one of Alias ("EP-142",
// upper-cased) and IssueID ("is-" + 16 lowercase hex) is set.
type Ref struct {
	Alias   string
	IssueID string
	Word    string // WordClosing | WordNonClosing (task 155 adds WordBranch)
}

const (
	WordClosing    = "closing"
	WordNonClosing = "non_closing"
)

const (
	closingWords  = `close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved`
	nonClosing    = `refs|part\s+of|related\s+to`
	refToken      = `(?:is-[0-9a-f]{16}|[A-Za-z][A-Za-z0-9]{1,9}-[1-9][0-9]{0,8})` // the id first: leftmost-first alternation
	refSeparator  = `\s*(?:,|\band\b)\s*`
	maxRefsInText = 50 // a body naming more is truncated, silently: it is not a list we act on
)

var (
	// wordRe: a word, an optional colon, then one or more references. \b on
	// both sides, so "prefix EP-1" and "unfixed EP-1" match nothing.
	wordRe = regexp.MustCompile(`(?i)\b(` + closingWords + `|` + nonClosing + `)\b:?\s+(` +
		refToken + `(?:` + refSeparator + refToken + `)*)\b`)
	tokenRe = regexp.MustCompile(`(?i)` + refToken)
	// issueIDRe is task 21's ValidIssueTrailer grammar (task-21.md:91), repeated
	// here because the Hub does not import the control plane (skeleton, Go).
	issueIDRe = regexp.MustCompile(`^is-[0-9a-f]{16}$`)
)

// ParseRefs returns every reference that follows a word, in text order, once
// per (reference, word); closing wins over non-closing for the same reference.
// projectKeys limits aliases to keys the project has had (current first); an
// alias under any other key is dropped here, so another project's EP-142 is
// never even looked up.
func ParseRefs(text string, projectKeys []string) []Ref {
	keys := map[string]bool{}
	for _, k := range projectKeys {
		keys[strings.ToUpper(k)] = true
	}
	var out []Ref
	at := map[string]int{} // reference → index in out
	for _, m := range wordRe.FindAllStringSubmatch(text, -1) {
		word := WordNonClosing
		if regexp.MustCompile(`(?i)^(` + closingWords + `)$`).MatchString(m[1]) {
			word = WordClosing
		}
		for _, tok := range tokenRe.FindAllString(m[2], -1) {
			r, ok := toRef(tok, keys)
			if !ok {
				continue
			}
			k := r.Alias + r.IssueID
			if i, seen := at[k]; seen {
				if word == WordClosing {
					out[i].Word = WordClosing
				}
				continue
			}
			if len(out) == maxRefsInText {
				return out
			}
			r.Word, at[k] = word, len(out)
			out = append(out, r)
		}
	}
	return out
}

func toRef(tok string, keys map[string]bool) (Ref, bool) {
	if l := strings.ToLower(tok); issueIDRe.MatchString(l) && strings.HasPrefix(tok, "is-") {
		return Ref{IssueID: l}, true
	}
	key, _, _ := strings.Cut(strings.ToUpper(tok), "-")
	if !keys[key] {
		return Ref{}, false
	}
	return Ref{Alias: strings.ToUpper(tok)}, true
}
```

The closing-word check compiles its pattern per match for readability here; the implementer
hoists it to a package variable (`closingRe`) and the mutant table's row still applies. The `is-`
form must be written in lower case (`strings.HasPrefix(tok, "is-")`): the id is lowercase hex and a
`IS-0123…` reads, to the regexp, like an alias under the key `IS`, which `toRef` then drops unless
the project's key is `IS`.

#### Interfaces — complete: `server/work_hooks.go`, the shape and the registration

```go
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

const (
	HostGitHub      = "github"
	HostAzureDevOps = "azure_devops"
	maxHookBody     = 1 << 20 // decision 3
)

// HostEvent is decision 1's one shape. Kind is "push" or "pr"; the fields of
// the other kind are zero. Nothing in it is written before authentication.
type HostEvent struct {
	Host       string
	Repo       string // "acme/web", or "org/project/repo" for Azure DevOps
	DeliveryID string
	Kind       string
	Actor      HostActor
	Commits    []HostCommit // push
	CreatedRef string       // push: a branch this push created, "" if none (task 155)
	PR         *HostPR      // pr
}

// HostActor is who the host says acted. Never written as a person's name.
type HostActor struct {
	Login      string // GitHub login
	UniqueName string // Azure DevOps identity uniqueName
}

type HostCommit struct {
	SHA, Message, URL string
}

type HostPR struct {
	Number      int
	URL         string
	Title, Body string
	Action      string // "opened" | "edited" | "merged" | "closed"
	MergeCommit string // set when Action == "merged"
}

func (s *Server) registerWorkHooks(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/projects/{projectId}/hooks/{host}", s.putHostHook)
	mux.HandleFunc("GET /v1/projects/{projectId}/outside-links", s.listOutsideLinks)
	mux.HandleFunc("POST /hooks/github/{projectId}", s.hookIngress(HostGitHub, normaliseGitHub))
	mux.HandleFunc("POST /hooks/ado/{projectId}", s.hookIngress(HostAzureDevOps, normaliseADO))
}

// putHostHook registers or rotates a project's hook (team admin). The remote
// must be the project's own; the secret is minted here, stored behind a
// secret_ref (task 145's proposal) and returned once.
func (s *Server) putHostHook(w http.ResponseWriter, r *http.Request) {
	projectID, host := r.PathValue("projectId"), r.PathValue("host")
	s.WriteLimit(w, r, 4<<10, func(tx *Tx, body []byte) (WriteResult, error) {
		ctx := r.Context()
		var req struct {
			RemoteURL string `json:"remoteUrl"`
		}
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		if host != HostGitHub && host != HostAzureDevOps {
			return WriteResult{}, Refuse(http.StatusBadRequest, "bad_request", "host is azure_devops or github", map[string]string{"field": "host"})
		}
		role, _, ok, err := ProjectRole(ctx, tx, projectID)
		if err != nil {
			return WriteResult{}, err
		}
		if err := Require(role, ok, hubapi.RoleAdmin, notFoundProject(projectID)); err != nil {
			return WriteResult{}, err
		}
		if remote, err := store.ProjectRemote(ctx, tx, projectID); err != nil || !sameRemote(remote, req.RemoteURL) {
			return WriteResult{}, Refuse(http.StatusConflict, "conflict", "remoteUrl is not this project's remote", map[string]string{"field": "remoteUrl"})
		}
		secret, ref, err := s.secrets.Mint(ctx, "hook/"+projectID+"/"+host) // task 145's seam; UNVERIFIED name
		if err != nil {
			return WriteResult{}, err
		}
		hook, created, err := store.PutHostHook(ctx, tx, projectID, host, req.RemoteURL, ref, CallerID(ctx))
		if err != nil {
			return WriteResult{}, err
		}
		if err := emitWork(ctx, tx, "host_hook.set", map[string]any{"projectId": projectID, "host": host,
			"secretVersion": hook.SecretVersion}); err != nil { // metadata only: no secret, no URL
			return WriteResult{}, err
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		return WriteResult{Status: status, NoStore: true, Body: hubapi.CreatedHostHook{Hook: hook, Secret: secret,
			URL: s.publicURL + "/hooks/" + hostPath(host) + "/" + projectID}}, nil
	})
}
```

`WriteResult.NoStore` keeps the reply out of `idempotency_keys` (a replay must never re-serve a
secret); if Hub task 14's `WriteResult` has no such flag, that is a gap for the Hub owner, and until
it lands this route refuses an `Idempotency-Key` header with 400. `ProjectRemote`, `sameRemote`
(scheme, host and path compared after trimming `.git` and a trailing `/`, case-insensitive host),
`hostPath` (`github` → `github`, `azure_devops` → `ado`), `CallerID` and `s.publicURL` are small
helpers; `emitWork` is task 13's (`task-13.md:332`). Which Hub column holds a project's remote is
**UNVERIFIED** (phase 3's lane remote or the Hub project record).

#### Interfaces — complete: `server/work_hooks.go`, ingress

```go
type normaliser func(r *http.Request, body []byte) (HostEvent, bool, error) // ok false: an event we ignore

var unauthenticated = []byte(`{"code":"unauthenticated"}`)

func deny(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write(unauthenticated)
}

// hookIngress is decisions 3 and 4: cap, authenticate, normalise, then one
// transaction that inserts the delivery and applies it.
func (s *Server) hookIngress(host string, norm normaliser) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, projectID := r.Context(), r.PathValue("projectId")
		body, err := io.ReadAll(io.LimitReader(r.Body, maxHookBody+1))
		if err != nil || len(body) > maxHookBody {
			deny(w)
			return
		}
		hook, secret, ok, err := s.hookSecret(ctx, projectID, host) // system read by project id; UNVERIFIED path
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if !ok || !authentic(host, r, body, secret) {
			deny(w)
			return
		}
		ev, relevant, err := norm(r, body)
		if err != nil || !relevant { // a ping, an event kind we do not read, or a shape we cannot
			writeJSON(w, http.StatusOK, map[string]any{"duplicate": false, "links": 0, "facts": 0})
			return
		}
		if ev.Repo != hook.RepoPath() { // a hook fired for another repository is not ours to read
			writeJSON(w, http.StatusOK, map[string]any{"duplicate": false, "links": 0, "facts": 0})
			return
		}
		var res applyResult
		err = s.hostTx(ctx, hook.OrgID, func(tx *Tx) error {
			fresh, err := store.InsertHookDelivery(ctx, tx, projectID, host, ev.DeliveryID)
			if err != nil || !fresh {
				res.Duplicate = !fresh
				return err
			}
			res, err = s.apply(ctx, tx, hook, ev)
			return err
		})
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError) // the host retries; the delivery row rolled back
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"duplicate": res.Duplicate, "links": res.Links, "facts": res.Facts})
	}
}

// authentic is decision 3's check for each host; both compare in constant time.
func authentic(host string, r *http.Request, body, secret []byte) bool {
	switch host {
	case HostGitHub:
		sig, ok := strings.CutPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		got, err := hex.DecodeString(sig)
		if !ok || err != nil {
			return false
		}
		m := hmac.New(sha256.New, secret)
		m.Write(body)
		return hmac.Equal(got, m.Sum(nil))
	case HostAzureDevOps:
		user, pass, ok := r.BasicAuth()
		https := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		return ok && https && user == "panacea" && subtle.ConstantTimeCompare([]byte(pass), secret) == 1
	}
	return false
}

type applyResult struct {
	Duplicate    bool
	Links, Facts int
}
```

`s.hostTx(ctx, org, fn)` opens a transaction with `hub.org_id` set to the hook's org and **no
Entra caller**, as the janitor's steps do (`server/janitor.go`, Hub plan; **UNVERIFIED** name);
`hub.work_actor_kind` is set per write inside `apply`, never for the whole transaction, so a link
written for an unmatched person never inherits a fact's settings. `s.hookSecret` resolves the
hook's `secret_ref` through task 145's Key Vault seam and may cache it for 5 minutes; a rotated
secret therefore takes up to 5 minutes to be refused (recorded). `X-Forwarded-Proto` is trusted
only because the Hub listens behind the edge alone (phase 8; **UNVERIFIED** until the edge
exists).

#### Interfaces — complete: `server/work_hooks.go`, the two normalisers

Only the fields read are declared; the decoders are lenient (a host adds fields without notice).
The payload shapes are ours, written from the fields named, **UNVERIFIED** against each host's
documentation on the day.

```go
func normaliseGitHub(r *http.Request, body []byte) (HostEvent, bool, error) {
	ev := HostEvent{Host: HostGitHub, DeliveryID: r.Header.Get("X-GitHub-Delivery")}
	var p struct {
		Ref        string `json:"ref"`
		Created    bool   `json:"created"`
		Repository struct{ FullName string `json:"full_name"` } `json:"repository"`
		Sender     struct{ Login string } `json:"sender"`
		Commits    []struct{ ID, Message, URL string } `json:"commits"`
		Action      string `json:"action"`
		PullRequest *struct {
			Number         int    `json:"number"`
			HTMLURL        string `json:"html_url"`
			Title          string `json:"title"`
			Body           string `json:"body"`
			Merged         bool   `json:"merged"`
			MergeCommitSHA string `json:"merge_commit_sha"`
		} `json:"pull_request"`
	}
	if ev.DeliveryID == "" || json.Unmarshal(body, &p) != nil {
		return ev, false, nil
	}
	ev.Repo, ev.Actor = p.Repository.FullName, HostActor{Login: p.Sender.Login}
	switch r.Header.Get("X-GitHub-Event") {
	case "push":
		ev.Kind = "push"
		for _, c := range p.Commits {
			ev.Commits = append(ev.Commits, HostCommit{SHA: c.ID, Message: c.Message, URL: c.URL})
		}
		if p.Created {
			ev.CreatedRef = strings.TrimPrefix(p.Ref, "refs/heads/")
		}
	case "pull_request":
		if p.PullRequest == nil {
			return ev, false, nil
		}
		pr := p.PullRequest
		ev.Kind, ev.PR = "pr", &HostPR{Number: pr.Number, URL: pr.HTMLURL, Title: pr.Title, Body: pr.Body}
		switch {
		case p.Action == "opened" || p.Action == "reopened":
			ev.PR.Action = "opened"
		case p.Action == "edited":
			ev.PR.Action = "edited"
		case p.Action == "closed" && pr.Merged:
			ev.PR.Action, ev.PR.MergeCommit = "merged", pr.MergeCommitSHA
		case p.Action == "closed":
			ev.PR.Action = "closed"
		default:
			return ev, false, nil // synchronize, labeled, …: nothing a word can change
		}
	default:
		return ev, false, nil // ping and every other event
	}
	return ev, true, nil
}

func normaliseADO(_ *http.Request, body []byte) (HostEvent, bool, error) {
	var p struct {
		ID        string `json:"id"`
		EventType string `json:"eventType"`
		Resource  struct {
			Repository struct {
				Name    string
				Project struct{ Name string }
				RemoteURL string `json:"remoteUrl"`
			} `json:"repository"`
			PushedBy   struct{ UniqueName string } `json:"pushedBy"`
			CreatedBy  struct{ UniqueName string } `json:"createdBy"`
			Commits    []struct{ CommitID, Comment, URL string } `json:"commits"`
			RefUpdates []struct{ Name, OldObjectID string } `json:"refUpdates"`
			PullRequestID   int    `json:"pullRequestId"`
			Title           string `json:"title"`
			Description     string `json:"description"`
			Status          string `json:"status"`
			LastMergeCommit struct{ CommitID string } `json:"lastMergeCommit"`
		} `json:"resource"`
	}
	if json.Unmarshal(body, &p) != nil || p.ID == "" {
		return HostEvent{}, false, nil
	}
	res := p.Resource
	ev := HostEvent{Host: HostAzureDevOps, DeliveryID: p.ID, Repo: adoRepoPath(res.Repository.RemoteURL)}
	switch p.EventType {
	case "git.push":
		ev.Kind, ev.Actor = "push", HostActor{UniqueName: res.PushedBy.UniqueName}
		for _, c := range res.Commits {
			ev.Commits = append(ev.Commits, HostCommit{SHA: c.CommitID, Message: c.Comment, URL: c.URL})
		}
		for _, u := range res.RefUpdates { // an all-zero old id is a created ref
			if strings.Trim(u.OldObjectID, "0") == "" && strings.HasPrefix(u.Name, "refs/heads/") {
				ev.CreatedRef = strings.TrimPrefix(u.Name, "refs/heads/")
			}
		}
	case "git.pullrequest.created", "git.pullrequest.updated", "git.pullrequest.merged":
		ev.Kind, ev.Actor = "pr", HostActor{UniqueName: res.CreatedBy.UniqueName}
		ev.PR = &HostPR{Number: res.PullRequestID, Title: res.Title, Body: res.Description,
			URL: strings.TrimSuffix(res.Repository.RemoteURL, "/") + "/pullrequest/" + strconv.Itoa(res.PullRequestID)}
		switch {
		case res.Status == "completed": // only completion lands; see below
			ev.PR.Action, ev.PR.MergeCommit = "merged", res.LastMergeCommit.CommitID
		case res.Status == "abandoned":
			ev.PR.Action = "closed"
		case p.EventType == "git.pullrequest.created":
			ev.PR.Action = "opened"
		default:
			ev.PR.Action = "edited"
		}
	default:
		return ev, false, nil
	}
	return ev, true, nil
}
```

Azure DevOps names its merge event *merge attempted*: it fires when the service tries the merge, not when the PR completes (**UNVERIFIED** on the day). So only `status: "completed"` lands; a `git.pullrequest.merged` whose status is still `active` is an edit. For a merged Azure DevOps PR the actor is the PR's creator (`createdBy`), not whoever completed it:
the service-hook payload's completing identity is **UNVERIFIED**, and the author of the words is the
creator, who is the person the fact should name. GitHub's `sender` is whoever acted (for a merge,
the merger). The two differ, and the difference is recorded for the owner. `work_hooks.go` gains
`strconv`; `adoRepoPath` turns `https://dev.azure.com/org/project/_git/repo` into
`org/project/repo`, the form `HostHook.RepoPath()` returns for Azure DevOps.

#### Interfaces — complete: `server/work_hooks.go`, `apply`

```go
// PersonMapper is decision 10's seam: a host's actor to a Hub user id, or no
// match. The GitHub implementation reads an EMU identity link; until that is
// shown readable (UNVERIFIED) it answers no match, so GitHub facts wait.
type PersonMapper interface {
	Match(ctx context.Context, tx *Tx, hook hubapi.HostHook, a HostActor) (userID string, ok bool, err error)
}

// apply is decision 8, in one place for both hosts. It never writes done.
func (s *Server) apply(ctx context.Context, tx *Tx, hook hubapi.HostHook, ev HostEvent) (applyResult, error) {
	var res applyResult
	keys, err := store.ProjectKeysEver(ctx, tx, hook.ProjectID) // current first, then history
	if err != nil {
		return res, err
	}
	userID, matched, err := s.people.Match(ctx, tx, hook, ev.Actor)
	if err != nil {
		return res, err
	}
	switch ev.Kind {
	case "push":
		for _, c := range ev.Commits {
			for _, ref := range ParseRefs(c.Message, keys) { // a trailer is text here too: a link only (decision 7)
				issueID, ok, err := store.ResolveRef(ctx, tx, hook.ProjectID, keys, ref)
				if err != nil || !ok {
					if err != nil {
						return res, err
					}
					continue
				}
				n, err := store.InsertOutsideCommitLink(ctx, tx, hubapi.OutsideCommitLink{IssueID: issueID, Host: ev.Host,
					Repo: ev.Repo, SHA: c.SHA, URL: c.URL, Word: ref.Word, PushedBy: optional(userID, matched)})
				if err != nil {
					return res, err
				}
				res.Links += n // 0 when (repo, sha, issue) exists already
			}
		}
		for _, id := range trailerIDs(ev.Commits) { // SW.9: an outside Panacea-Issue is a link only
			n, err := store.InsertOutsideCommitLinkByTrailer(ctx, tx, hook.ProjectID, ev, id, optional(userID, matched))
			if err != nil {
				return res, err
			}
			res.Links += n
		}
	case "pr":
		for _, ref := range ParseRefs(ev.PR.Title+"\n"+ev.PR.Body, keys) {
			issueID, ok, err := store.ResolveRef(ctx, tx, hook.ProjectID, keys, ref)
			if err != nil {
				return res, err
			}
			if !ok {
				continue
			}
			link, fresh, err := store.UpsertOutsidePRLink(ctx, tx, issueID, ev, ref.Word)
			if err != nil {
				return res, err
			}
			if fresh {
				res.Links++
			}
			fact := ""
			switch {
			case ev.PR.Action == "merged" && ref.Word == WordClosing:
				fact = "landed"
			case (ev.PR.Action == "opened" || ev.PR.Action == "edited") && link.PRState == "open":
				fact = "in_review"
			}
			if fact == "" {
				continue
			}
			if !matched { // decision 10: links only, and one inform to the assignee
				if err := emitWork(ctx, tx, "issue.outside_unmatched", map[string]any{"issueId": issueID,
					"host": ev.Host, "prNumber": ev.PR.Number}); err != nil {
					return res, err
				}
				continue
			}
			moved, err := s.hostFact(ctx, tx, issueID, fact, userID, ev.PR.MergeCommit)
			if err != nil {
				return res, err
			}
			if moved {
				res.Facts++
			}
		}
	}
	return res, nil
}

// hostFact moves one issue into in_review (cause outside_pr) or landed (cause
// outside_merge) as the mapped person, only forward: an issue already at or
// past the fact is left alone. The guard (task 145) is the second layer: it
// refuses done and every other state for actor kind host whatever Go does.
func (s *Server) hostFact(ctx context.Context, tx *Tx, issueID, fact, userID, mergeCommit string) (bool, error) {
	cur, err := store.IssueStateForUpdate(ctx, tx, issueID)
	if err != nil {
		return false, err
	}
	if !before(cur, fact) {
		return false, nil
	}
	cause, move := hubapi.CauseOutsidePR, hubapi.StateMove{To: hubapi.StateInReview}
	if fact == "landed" {
		cause, move = hubapi.CauseOutsideMerge, hubapi.StateMove{To: hubapi.StateLanded, LandCommit: mergeCommit}
	}
	move.Cause = cause
	if err := store.SetWorkActor(ctx, tx, hubapi.ActorKindHost, userID, cause); err != nil { // set_config(…, true)
		return false, err
	}
	defer store.ClearWorkActor(ctx, tx) // the next write in this transaction starts unset
	return true, s.moveIssue(ctx, tx, issueID, move, userID) // task 10's path; its event names userID, data.via "host:<host>"
}
```

`before(cur, fact)` is the fixed order `triage < backlog < todo < started < in_review < landed <
done`; `cancelled` and `duplicate` are never before anything, so an outside PR cannot revive a
declined issue. `optional`, `trailerIDs` (task 21's grammar over each message's trailer lines) and
`ProjectKeysEver` (the current key, then `project_key_history` newest first) are small helpers.
`hubapi.CauseOutsidePR`, `CauseOutsideMerge` and `ActorKindHost` are task 143's names as its scope
words them (`outside_pr`, `outside_merge`, `host`); **UNVERIFIED** until its part.

`work_hooks.go`'s import block above also needs `context` (for `PersonMapper` and `apply`).

#### Tests to write first — `server/work_hooks_words_test.go` (pure, complete)

```go
package server

import (
	"reflect"
	"testing"
)

func TestParseRefs(t *testing.T) {
	keys := []string{"RS", "EP"} // current RS, previously EP
	id := "is-0123456789abcdef"
	for _, tc := range []struct {
		name, text string
		want       []Ref
	}{
		{"closing alias", "Fixes EP-142", []Ref{{Alias: "EP-142", Word: WordClosing}}},
		{"closing with colon, list", "resolves: EP-1, RS-2 and EP-3",
			[]Ref{{Alias: "EP-1", Word: WordClosing}, {Alias: "RS-2", Word: WordClosing}, {Alias: "EP-3", Word: WordClosing}}},
		{"non-closing", "refs EP-142", []Ref{{Alias: "EP-142", Word: WordNonClosing}}},
		{"part of", "Part of  ep-9", []Ref{{Alias: "EP-9", Word: WordNonClosing}}},
		{"bare id", "closes " + id, []Ref{{IssueID: id, Word: WordClosing}}},
		{"closing wins", "refs EP-5 then fixes EP-5", []Ref{{Alias: "EP-5", Word: WordClosing}}},
		{"no word", "EP-142 is broken", nil},
		{"word inside a word", "unfixed EP-1; prefixes EP-2", nil},
		{"another project's key", "fixes XY-7", nil},
		{"number zero is not an alias", "fixes EP-0", nil},
		{"a tracker's own word is not ours", "completes EP-4", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseRefs(tc.text, keys); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseRefs(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseRefsIsBounded(t *testing.T) {
	text := "refs"
	for i := 1; i <= 80; i++ {
		text += " EP-" + itoa(i) + ","
	}
	if got := ParseRefs(text, []string{"EP"}); len(got) != maxRefsInText {
		t.Fatalf("%d refs, want %d", len(got), maxRefsInText)
	}
}
```

`itoa` is `strconv.Itoa`; the list's trailing comma is deliberate (a separator with nothing after it
ends the match without error).

#### Tests to write first — `server/work_hooks_integration_test.go` (`//go:build integration`)

Against `control-plane/cmd/hub/compose.yaml`, with a test `PersonMapper` that matches `alice` (and
`alice@kpmg.example`) to `u-alice` and nothing else, and a project whose key changed `EP` → `RS`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGitHubHookWrongSignatureStoresNothing` | a body signed with another secret is 401 `unauthenticated`; `work_hook_deliveries`, both link tables and `work_events` unchanged | "unsigned hooks accepted" |
| `TestHookOverCapIsRefused` | a 1 MiB + 1 byte body correctly signed is 401 with nothing stored | "cap after parse" |
| `TestRedeliveryWritesOnce` | the same `X-GitHub-Delivery` twice: one link, the second reply `duplicate: true` | "redelivery duplicates" |
| `TestOldAliasStillLinks` | `Fixes EP-142` after the key became `RS`: the link's `issue_id` is `is-…`, and no column holds `EP-142` | "alias as key" |
| `TestNonClosingOnMergeLeavesState` | a merged PR with `refs EP-142`: the state is unchanged, one `OutsidePRLink` with `word: non_closing` | "non-closing closes" |
| `TestMergedClosingPRLandsAsThePerson` | merged by `alice` with `Fixes EP-142`: `landed`, `land_commit` the merge SHA, cause `outside_merge`, the event's actor `u-alice`, `data.via` `host:github` | "fact as the system" |
| `TestUnmatchedMergeLinksAndInforms` | merged by `mallory`: a link, one `issue.outside_unmatched` with no login in its data, and no state change | "fact with no person" |
| `TestOutsideLandNeverDone` | an issue with empty `serves` merged by `alice`: `landed`, not `done`; a direct `UPDATE … state = 'done'` under actor `host` raises `42501` | "host writes done" |
| `TestADOWithoutBasicAuthIsRefused` | an Azure DevOps body with no `Authorization`, and one over plain http, are 401 | "ADO unauthenticated" |
| `TestTrailerOnOutsideCommitIsLinkOnly` | a pushed commit carrying `Panacea-Issue: is-…`: one `OutsideCommitLink`, state unchanged | "outside trailer moves a fact" |
| `TestOpenedPRMovesToInReviewOnce` | opened then edited with `Fixes EP-142`, the issue in `started`: one `in_review` move, one link | "every edit re-moves" |

#### Mutants for the parser, applied by hand

| Mutant | Row that fails |
|---|---|
| drop the leading `\b` in `wordRe` | *word inside a word* |
| `closes` removed from `closingWords` | *closing with colon, list* |
| the key filter removed from `toRef` | *another project's key* |
| closing does not overwrite non-closing | *closing wins* |
| `[1-9]` becomes `[0-9]` in `refToken` | *number zero is not an alias* |
| `maxRefsInText` check removed | `TestParseRefsIsBounded` |

#### Matrix rows — `server/work_matrix_rows.go` ⚑ (task 07)

`PUT /v1/projects/{projectId}/hooks/{host}`: ✓ for `A`; 403 `required: admin` for `M` and `V`; 404
`project_not_found` for `O N X` (the columns of `hub-contract.md:337-339`, as task 15 cites them).
`GET /v1/projects/{projectId}/outside-links`: ✓ for `M A V`; 404 for `O N X`. The *as a host
webhook* fixture (task 145) is refused every `/v1` route with 401 (it holds no Entra token), and
the two `/hooks/…` routes are not in the matrix: they are authenticated by the hook's secret, which
the integration rows above pin.

#### Steps

1. Confirm tasks 21, 22, 143 and 145 are on the sync base, and whether 145 carries
   `work_hook_deliveries` and `project_key_history`. If not, **stop** and raise the gap: this task
   writes no migration. Confirm the Key Vault seam (`s.secrets`) exists; until it does, the
   registration route answers 503 `secrets_unavailable` and the ingress routes 401, and the
   integration rows run against a test secret store.
2. Write `work_hooks_words_test.go`; run it red. Write `work_hooks_words.go`; run it green; apply
   the parser mutants by hand and revert them.
3. Write the integration rows; write `store/work_hooks.go` and `server/work_hooks.go`; register the
   routes in `server.go`; add the matrix rows. Run with `-tags integration` green.
4. Add the janitor's delivery-expiry statement beside task 150's sync step (`DELETE FROM
   work_hook_deliveries WHERE received_at < now() - interval '7 days'`, under its advisory lock).
5. Commit the files listed above and nothing else.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/server/ -race -count=1 -run "ParseRefs" -v'
# expect: TestParseRefs (11 subtests) and TestParseRefsIsBounded PASS; ok
~/.claude/tools/claudezsh/claudezsh 'go test -tags integration ./control-plane/internal/hub/server/... -race -count=1 -run "Hook|Redelivery|OldAlias|NonClosing|MergedClosing|Unmatched|OutsideLand|ADOWithout|TrailerOnOutside|OpenedPR"'
# expect: the eleven rows PASS against control-plane/cmd/hub/compose.yaml; ok
~/.claude/tools/claudezsh/claudezsh 'go test -tags integration ./control-plane/internal/hub/server/... -race -count=1 -run Matrix'
# expect: ok, with the two new routes' rows present
```

#### What this task does NOT claim

- That either host can reach the Hub's ingress through the edge (**UNVERIFIED**, phase 8), or that
  the edge sets `X-Forwarded-Proto` as trusted here.
- That the outside causes are accepted into 05 §3.3's cause table: they extend it, and decision 9's
  *never `done`* departs from 05 §3.3's *"turns `done` at `landed`"*. Both are the owner's call.
- That any GitHub login maps to an Entra user (EMU identity links, **UNVERIFIED** readable), or that
  Azure DevOps' `uniqueName` is the UPN in every organisation.
- That the payload fields read here are what the hosts send on the day: the shapes are ours.
- That a GitHub push over 1 MiB is handled (it is refused; decision 3).
- Linkbacks, autolinks, the filing word and the first push (task 155).

#### Gaps found while writing this part

1. **The File map lacks `server/work_hooks_words.go`** (the scope names it as a gap to add).
2. **Task 145: two tables** — `work_hook_deliveries` and `project_key_history` (complete SQL above).
3. **Task 08: key history.** The scope's *"task 08's key history"* does not exist in task 08's part:
   it keeps one key per project and records the last `previousKey` in an event only
   (`task-08.md:201`, `:282-285`). Its key route should insert the retired key into
   `project_key_history` in the same transaction.
4. **Task 143: `CreatedHostHook.url`** (the scope says the URL is shown once with the secret), and
   the event kinds `host_hook.set` and `issue.outside_unmatched`.
5. **Hub task 14: `WriteResult.NoStore`**, so a secret-bearing reply is never kept for replay.
6. **Hub contract: the `/hooks/` ingress routes and the Key Vault seam** are proposed additions for
   the `feat/hub` owner, like task 80's `/p/` pages; nothing here runs until both are accepted.
7. **Owner decision needed:** for Azure DevOps the fact names the PR's creator; for GitHub, the
   sender (the merger). One rule for both would be better; this part records the difference.
