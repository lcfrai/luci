### Task 155: outside PRs, part 2: linkbacks and autolinks; a PR-body magic word files an issue; copy `ep-142-slug`, the first push is `started`

W5, streams WH and WP, about 3 hours (an estimate, not a measurement). Depends on task 154 (the
ingress, `HostEvent`, `HostEvent.CreatedRef`, `apply`, `hostFact`, `ParseRefs`, `PersonMapper`,
`ProjectKeysEver`, `ResolveRef`, `task-154.md:292-321`, `:609-727`). Reads task 152's
`GitHubAppToken` and `worksync.TokenFunc` (`task-152.md:354-404`, `task-151.md:108`), task 21's
`lanes.IssueBranchName` (`task-21.md:202-236`), task 24's `workstart.DataClass`
(`task-24.md:161-165`), task 16's relay file and `Deps.Work` (`task-16.md:496-563`), task 37's
`WORK_LOCAL` (`task-37.md:301-310`), task 46's `IssueHead` (`task-46.md:554-633`), task 146's
intake create path (`task-146.md:575-629`), task 143's `OutsidePRLink` and `WordBranch`
(`task-143.md:291-311`), task 20's `expectNoPrediction` (`task-20.md:41`), and three tasks with
**no part on this tree**: 80 (`server/work_pages.go`, the `/p/` page and its browser session,
scope `tasks-61-80.md:665-700`), 59 (`palette/commands.ts`, scope `tasks-41-60.md:579`) and 145
(the `host` guard, scope `tasks-141-160.md:230-276`). Every name taken from those three is
**UNVERIFIED** until their parts exist.

Closes **L-177** (05 §1: *"Magic word in a PR body files an issue … on both hosts"*), **L-203**
(*"PR linkback comments, autolinks … Both hosts"*: GitHub in full; on Azure DevOps the linkback
only, so **PARTIAL** there, decision 5), and, with task 154, the W5 halves of **L-198** (*"For work
outside the product, copy `ep-142-slug`; the first push is the `started` fact"*), **L-199**,
**L-200** and **L-201**.

**What this task must not get wrong:** nothing the Hub writes on a customer's remote carries an
issue's text. A linkback is an alias and a link, never a title, because an *AU data only* title
must not reach a remote the Hub's erase cannot follow (`OD-24`, Q8). A branch name for an AU
project is the alias alone for the same reason. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **The Hub never calls a host inside a transaction.** `apply` claims a linkback in the database
   (`linkback_state` `NULL` → `pending`, one conditional `UPDATE`) and returns it; the ingress posts
   it after commit and records `posted` or `failed` in a second, one-statement transaction. A host
   that is slow or down therefore cannot hold a row lock or roll back a link, and two deliveries
   racing on the same PR claim each link once: the second `UPDATE` matches no row.
2. **One linkback per `(PR, issue)`, ever, and one comment per delivery.** A delivery that links
   EP-142 and EP-143 for the first time posts one comment with two lines. A later `edited` event
   that names them again claims nothing and posts nothing. A failed post is not retried: it stays
   `failed`, visible in task 172's admin list (**UNVERIFIED** that 172 shows it), because a retry
   loop that posts twice is worse than a missing comment.
3. **The linkback text is built from constants, the alias and the id link, and nothing else.**
   Each line is `Linked to EP-142 in PanaceaApp: https://<hub>/p/issues/is-…`. The link is the
   id form, not the alias form, so it survives every key change (skeleton rule 1). The builder
   takes no title parameter at all, so no future caller can pass one in by mistake.
4. **The filing word is one line, `issue: new`,** alone on a line of the PR body, any case, with
   spaces allowed around the colon (our own word, not a host's). It files one issue per PR, ever,
   recorded in `work_outside_pr_filings` whether it filed or not, so an `edited` event that still
   carries the line files nothing more and an unmatched author is told once. The issue: title the
   PR title cut to SW.1's bound, description `Filed from <PR URL>` (the PR body is not copied),
   state `triage`, `source: intake`, `not tied: support`, created by the **mapped PR author**
   (`HostPR.Author`, added here). Its PR link is `non_closing`: a filed issue is intake, and a
   person accepts it out of `triage` (05 §3.5), so a merge does not land it.
5. **Autolinks are GitHub only.** At registration, and whenever the project's current key differs
   from the key the hook last registered (`work_host_hooks.autolink_key`), the Hub sends one
   `POST /repos/{owner}/{repo}/autolinks` with prefix `EP-` and the template
   `https://<hub>/p/issues/by-alias/EP-<num>?p=pj-…`. The janitor step compares the two columns
   each tick, so a key change is caught without a hook on task 08's route; the old prefix's
   autolink is left on the repository, so an old `EP-142` in a GitHub comment still resolves.
   Azure DevOps: this part found no free-prefix autolink for repositories (**UNVERIFIED** against
   the current Azure DevOps documentation; the implementer checks on the day and, if one exists,
   raises a follow-up rather than widening this task).
6. **The by-alias page needs its project.** `GET /p/issues/by-alias/{alias}?p={projectId}`
   resolves the alias at request time through the project's current key and every earlier key
   (task 154's `ProjectKeysEver`), then answers `302` to `/p/issues/{issueId}`. A missing `p`, an
   alias that resolves to nothing, or an issue the signed-in reader cannot read is the same `404`.
   The scope's path had no project; an alias alone is ambiguous once a key is freed and reused,
   and the autolink, the only producer of these URLs, always knows the project.
7. **The first push is a created branch, parsed, never guessed.** Task 154's normalisers already
   set `CreatedRef` (GitHub `created: true`; Azure DevOps an all-zero old object id). `ParseBranch`
   reads `^([a-z][a-z0-9]{1,9})-([1-9][0-9]{0,8})(-|$)`, upper-cases the key, keeps it only if the
   project has ever had it, and resolves `KEY-n` through `ResolveRef`. An issue in `triage`,
   `backlog` or `todo` moves to `started`, cause `outside_push`, **as the mapped pusher**; one
   `OutsideCommitLink` with word `branch` records the push's head commit. An issue at or past
   `started`, a push that created no branch, a name that parses to nothing, or an alias naming no
   issue writes nothing and fails nothing. A branch deleted and pushed again while its issue was
   moved back to `todo` by a person is a new first push, and moves it again: the rule reads state,
   not history.
8. **Unmatched people get the same treatment as in task 154.** An unmatched pusher: the branch
   link, no fact, one `issue.outside_unmatched`. That event's data gains `kind` (`"pr"` or
   `"push"`) beside task 154's `{issueId, host, prNumber}`, and `prNumber` is `0` for a push. An
   unmatched PR author with `issue: new`: nothing filed, a filing row with no issue, and the
   linkback line `could not file: we cannot match your account`. Neither carries a login.
9. **The branch name is computed at the press, in the control plane, by one function.** `GET
   /v1/work/issues/{issueId}/branch-name` reads the issue through `Deps.Work.Issue` (task 16's
   re-hash included) and the class through task 24's `DataClass`, and returns
   `lanes.IssueBranchName(alias, title, auOnly)`. An error from the class, or no class wired, is
   `auOnly = true`, as task 24's Start does (`task-24.md:262`): a missing class loses a slug, never
   leaks one. The desktop never caches the answer, so a renamed issue copies its new slug.
10. **The filing and the first push write as `host`.** Both set `hub.work_actor_kind = 'host'` and
    `hub.work_actor_user` to the mapped person for their writes only, through task 154's
    `SetWorkActor` / `ClearWorkActor` pair. The filed issue's `INSERT` is not state-guarded (task
    07's `guard_issue_initial_state` admits `triage`, `task-07.md:235-240`), and task 145's `host`
    branch admits `started` with `outside_push` and a mapped person; nothing here needs a new guard.
11. **Whose credential writes on the remote.** GitHub: the Hub's own GitHub App (task 152's
    `GitHubAppToken`), one installation per repository owner, so task 154's `PUT …/hooks/github`
    body gains a required `installationId` (a delta to task 154's route, below) and the hook row
    keeps it. Tokens are cached per installation, so a busy repository mints once an hour, not per
    comment. Azure DevOps: the Hub's workload identity, as task 151's Boards adapter uses it
    (`task-151.md:52`); the identity must be a member of the organisation with *Contribute to pull
    requests* on the repository (**UNVERIFIED**, an org-admin act, and not checked at
    registration: the first linkback's `failed` state is how an admin finds out).

**Files:**

- `control-plane/internal/hub/server/work_linkback.go` — `Linkback`, `LinkbackText`, the GitHub and
  Azure DevOps implementations, `Autolinker` (new; **not in the File map**: a gap)
- `control-plane/internal/hub/server/work_linkback_test.go` — pure and `httptest` rows (new)
- `control-plane/internal/hub/server/work_hooks_words.go` ⚑ (task 154) — `ParseBranch`,
  `WantsFiling`, `WordBranch` (modified)
- `control-plane/internal/hub/server/work_hooks_words_test.go` ⚑ — their rows (modified)
- `control-plane/internal/hub/server/work_hooks.go` ⚑ (task 154) — `HostPR.Author`; `apply`'s
  filing, first push and linkback claims; the post after commit; the autolink at registration
  (modified)
- `control-plane/internal/hub/store/work_hooks.go` ⚑ (task 154) — `ClaimLinkback`,
  `MarkLinkback`, `InsertFiling`, `HooksNeedingAutolink`, `SetAutolinkKey` (modified)
- `control-plane/internal/hub/server/work_pages.go` ⚑ (task 80) — the by-alias redirect (modified)
- `control-plane/internal/hub/server/janitor.go` ⚑ — the autolink step beside task 154's delivery
  expiry (modified; the file name is the Hub plan's, **UNVERIFIED**)
- `control-plane/internal/hub/server/work_hooks_integration_test.go` ⚑ — nine rows (modified)
- `control-plane/internal/api/work.go` ⚑ (task 16) — the branch-name route (modified)
- `control-plane/internal/api/work_branch_test.go` — its rows (new)
- `desktop/src/api/work.ts` ⚑ (task 37) — one `WORK_LOCAL` row and `issueBranchName` (modified;
  **not in the scope's list**: a gap)
- `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46), `IssueHead.test.tsx` ⚑ — *Copy branch
  name* (modified)
- `desktop/src/components/palette/commands.ts` ⚑ (task 59) — the same verb, no chord (modified)

**Interfaces:**

- Produces: `server.Linkback`, `server.LinkbackText`, `server.GitHubLinkback`,
  `server.ADOLinkback`, `server.Autolinker`, `server.ParseBranch`, `server.WantsFiling`; `GET
  /p/issues/by-alias/{alias}?p=`; `GET /v1/work/issues/{issueId}/branch-name` → `{"name": …}`;
  the desktop's `WorkClient.issueBranchName(issueId)`; the palette verb `issue.copyBranchName`
- Consumes: task 154's `HostEvent`, `apply`, `hostFact`, `SetWorkActor`, `ClearWorkActor`,
  `ProjectKeysEver`, `ResolveRef`, `UpsertOutsidePRLink`; task 152's `GitHubAppToken`; task 151's
  `TokenFunc` under the Hub's workload identity; task 146's intake create sequence; task 21's
  `IssueBranchName`; task 24's `DataClass`; task 20's `expectNoPrediction`
- Consumed by: task 168 (end to end), task 172 (the admin list shows `linkback_state`)

#### The columns — for task 145's migration (complete SQL; **not** applied by this task)

Task 145 creates `work_outside_pr_links` and `work_host_hooks`; these fold into its `CREATE TABLE`
statements (written here as `ALTER` so the delta reads on its own), and the third table is new:

```sql
-- Decisions 1-2: a linkback is claimed once per (PR, issue); NULL means never claimed.
ALTER TABLE work_outside_pr_links
  ADD COLUMN linkback_state text CHECK (linkback_state IN ('pending', 'posted', 'failed')),
  ADD COLUMN linkback_at    timestamptz;
GRANT UPDATE (linkback_state, linkback_at) ON work_outside_pr_links TO hub_app;

-- Decision 5: the project key the repository's autolink was last registered for; and, for
-- GitHub, the Hub app's installation on the repository's owner (decision 11).
ALTER TABLE work_host_hooks
  ADD COLUMN autolink_key    text CHECK (autolink_key IS NULL OR autolink_key ~ '^[A-Z][A-Z0-9]{1,9}$'),
  ADD COLUMN installation_id bigint CHECK (installation_id IS NULL OR installation_id > 0),
  ADD CONSTRAINT work_host_hooks_github_installation
    CHECK (host <> 'github' OR installation_id IS NOT NULL);
GRANT UPDATE (autolink_key, installation_id) ON work_host_hooks TO hub_app;

-- Decision 4: one filing per PR, ever; issue_id NULL means the author did not match.
CREATE TABLE work_outside_pr_filings (
  org_id     text        NOT NULL,
  project_id text        NOT NULL,
  host       text        NOT NULL CHECK (host IN ('azure_devops', 'github')),
  repo       text        NOT NULL CHECK (char_length(repo) BETWEEN 1 AND 300),
  number     integer     NOT NULL CHECK (number > 0),
  issue_id   text        CHECK (issue_id IS NULL OR issue_id ~ '^is-[0-9a-f]{16}$'),
  filed_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, project_id, host, repo, number)
);
ALTER TABLE work_outside_pr_filings ENABLE ROW LEVEL SECURITY;
CREATE POLICY work_outside_pr_filings_org ON work_outside_pr_filings
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_outside_pr_filings TO hub_app;
```

The key check on `autolink_key` is task 02's `ValidProjectKey` widened to task 154's `refToken`
key (2 to 10 characters); if task 02's grammar is narrower (2 to 5 capitals, `task-21.md:259-260`),
the narrower one wins and the check is tightened at merge. The erase function (task 146's
`server/erase.go`) needs no delta: no column added here holds text.

#### Wire shapes

```text
The linkback, one comment per delivery (decisions 2, 3, 8):
  Linked to EP-142 in PanaceaApp: https://<hub>/p/issues/is-0123456789abcdef
  Filed EP-143 in PanaceaApp from this pull request: https://<hub>/p/issues/is-89abcdef01234567
  could not file: we cannot match your account

GitHub, the comment:   POST {api}/repos/{owner}/{repo}/issues/{number}/comments
                       Authorization: Bearer <installation token>   {"body": "<lines>"}   ← 201
GitHub, the autolink:  POST {api}/repos/{owner}/{repo}/autolinks
                       {"key_prefix": "EP-", "is_alphanumeric": false,
                        "url_template": "https://<hub>/p/issues/by-alias/EP-<num>?p=pj-…"}
                       ← 201; ← 422 when the prefix exists already (treated as done; UNVERIFIED)
Azure DevOps, the comment:
                       POST https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/
                            pullRequests/{number}/threads?api-version=7.1
                       Authorization: Bearer <workload identity token>
                       {"comments": [{"parentCommentId": 0, "content": "<lines>", "commentType": 1}],
                        "status": "closed"}                                           ← 200
                       ("closed" so a linkback never blocks a PR policy that requires every
                        thread resolved; the string form of the status is UNVERIFIED)

GET /p/issues/by-alias/EP-142?p=pj-…   (the browser session of task 80)
  ← 302 Location: /p/issues/is-0123456789abcdef
  ← 404 (the page's one not-found page) for no p, no match, or not readable

GET /v1/work/issues/is-…/branch-name   (the loopback control plane)
  ← 200 {"name": "ep-142-resume-upload-fails-on-safari"}   (Global)
  ← 200 {"name": "ep-142"}                                 (AU data only, or the class unknown)
  ← 409 {"code": "no_project_key", "message": "the project has no key yet"}
  ← the relay rule's answer for a Hub refusal (task 16), e.g. 404 issue_not_found
```

The payload shapes are ours, written from the fields named, **UNVERIFIED** against each host's
documentation on the day, as task 154's are.

#### Interfaces — complete: `server/work_hooks_words.go` ⚑, appended

```go
// WordBranch is a link made by a branch name (ep-142-slug); task 143's
// hubapi.WordBranch carries the same string on the wire.
const WordBranch = "branch"

var (
	// branchRe is decision 7's grammar: a lower-case key, a number with no
	// leading zero, then "-" or the end. "ep-142x" is not a branch for EP-142.
	branchRe = regexp.MustCompile(`^([a-z][a-z0-9]{1,9})-([1-9][0-9]{0,8})(?:-|$)`)
	// filingRe is decision 4's word: "issue: new" alone on a line, any case.
	filingRe = regexp.MustCompile(`(?im)^[ \t]*issue[ \t]*:[ \t]*new[ \t]*\r?$`)
)

// ParseBranch reads a created branch's name. It returns the alias with
// Word WordBranch, or false when the name is not ours or its key is one the
// project never had. Like ParseRefs it knows nothing of the Hub.
func ParseBranch(name string, projectKeys []string) (Ref, bool) {
	m := branchRe.FindStringSubmatch(name)
	if m == nil {
		return Ref{}, false
	}
	key := strings.ToUpper(m[1])
	for _, k := range projectKeys {
		if strings.ToUpper(k) == key {
			return Ref{Alias: key + "-" + m[2], Word: WordBranch}, true
		}
	}
	return Ref{}, false
}

// WantsFiling reports whether a PR body asks for an issue. Only the body is
// read, never the title: a title is one line a reviewer skims, not a request.
func WantsFiling(body string) bool {
	return filingRe.MatchString(body)
}
```

`branchRe` needs no `(?i)`: git hosts keep the case a person typed, and `EP-142-x` is not the
product's form (task 21 lower-cases every name it makes). A person who types `EP-142` by hand gets
no fact, and the *Copy branch name* verb exists so that nobody has to.

#### Interfaces — complete: `server/work_linkback.go`, the text

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/worksync"
)

// Linkback writes the one comment a delivery earns on a PR (decisions 1-3).
type Linkback interface {
	Comment(ctx context.Context, pr hubapi.OutsidePRLink, text string) error
}

// LinkLine is one line of a linkback. It has no title field, on purpose:
// nothing that can reach a customer's remote is ever handed issue text.
type LinkLine struct {
	Kind    string // LineLinked | LineFiled | LineUnmatched
	Alias   string // "EP-142"; "" for LineUnmatched
	IssueID string // "is-…";   "" for LineUnmatched
}

const (
	LineLinked    = "linked"
	LineFiled     = "filed"
	LineUnmatched = "unmatched"
	productName   = "PanaceaApp"
)

var aliasRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]{0,8}$`)

// LinkbackText builds the comment. hubURL is the Hub's public origin; one
// that is not a bare https origin is refused, so a reviewer is never sent to
// a plain-http page. At most maxRefsInText lines (task 154's cap): a PR
// cannot link more issues than ParseRefs returns.
func LinkbackText(hubURL string, lines []LinkLine) (string, error) {
	u, err := url.Parse(hubURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || (u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" || u.User != nil {
		return "", fmt.Errorf("linkback: hub url %q is not an https origin", hubURL)
	}
	origin := "https://" + u.Host
	var b strings.Builder
	for i, l := range lines {
		if i == maxRefsInText {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		if l.Kind == LineUnmatched {
			b.WriteString("could not file: we cannot match your account")
			continue
		}
		if (l.Kind != LineLinked && l.Kind != LineFiled) || !aliasRe.MatchString(l.Alias) ||
			!issueIDRe.MatchString(l.IssueID) {
			return "", fmt.Errorf("linkback: line %d is not a kind, an alias and an id", i)
		}
		link := origin + "/p/issues/" + l.IssueID
		if l.Kind == LineFiled {
			fmt.Fprintf(&b, "Filed %s in %s from this pull request: %s", l.Alias, productName, link)
		} else {
			fmt.Fprintf(&b, "Linked to %s in %s: %s", l.Alias, productName, link)
		}
	}
	return b.String(), nil
}
```

`issueIDRe` and `maxRefsInText` are task 154's (`task-154.md:196`, `:207`). The alias check is
what makes decision 3 hold even against a bug upstream: a line whose `Alias` field was filled with
a title fails `aliasRe` and the whole comment is refused, so nothing is posted rather than
something wrong.

#### Interfaces — complete: `server/work_linkback.go`, the two hosts and the autolink

```go
// hostClient is the Hub's one outbound path to a git host: a bearer token,
// JSON, 10 s, the reply read to at most 64 KiB and discarded. No retry
// (decision 2). worksync's fetcher is package-private, so this is its own.
type hostClient struct {
	HTTP  *http.Client
	Token worksync.TokenFunc
}

func (c hostClient) post(ctx context.Context, u string, body any) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tok, err := c.Token(ctx)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return res.StatusCode, nil
}

// GitHubLinkback comments and registers autolinks as the Hub's GitHub App,
// for one installation (decision 11). HubURL is only for the autolink's
// template; the comment's text arrives built.
type GitHubLinkback struct {
	Client  hostClient
	BaseURL string // "https://api.github.com"; an httptest server in tests
	HubURL  string
}

func (g GitHubLinkback) Comment(ctx context.Context, pr hubapi.OutsidePRLink, text string) error {
	owner, repo, ok := strings.Cut(pr.Repo, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") || pr.Number < 1 {
		return fmt.Errorf("linkback: github repo %q #%d", pr.Repo, pr.Number)
	}
	u := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", strings.TrimRight(g.BaseURL, "/"),
		url.PathEscape(owner), url.PathEscape(repo), pr.Number)
	return want2xx(g.Client.post(ctx, u, map[string]string{"body": text}))
}

// Autolink is decision 5: one prefix per call. 422 means the prefix is on
// the repository already, which is the state we wanted (UNVERIFIED code).
func (g GitHubLinkback) Autolink(ctx context.Context, repo, key, projectID string) error {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || !aliasRe.MatchString(key+"-1") {
		return fmt.Errorf("autolink: github repo %q key %q", repo, key)
	}
	u := fmt.Sprintf("%s/repos/%s/%s/autolinks", strings.TrimRight(g.BaseURL, "/"),
		url.PathEscape(owner), url.PathEscape(name))
	tmpl := strings.TrimRight(g.HubURL, "/") + "/p/issues/by-alias/" + key + "-<num>?p=" + url.QueryEscape(projectID)
	st, err := g.Client.post(ctx, u, map[string]any{"key_prefix": key + "-", "url_template": tmpl,
		"is_alphanumeric": false})
	if err == nil && st == http.StatusUnprocessableEntity {
		return nil
	}
	return want2xx(st, err)
}

// ADOLinkback writes one closed PR thread as the Hub's workload identity.
type ADOLinkback struct {
	Client  hostClient
	BaseURL string // "https://dev.azure.com"
}

func (a ADOLinkback) Comment(ctx context.Context, pr hubapi.OutsidePRLink, text string) error {
	p := strings.Split(pr.Repo, "/") // org/project/repo: task 154's adoRepoPath
	if len(p) != 3 || p[0] == "" || p[1] == "" || p[2] == "" || pr.Number < 1 {
		return fmt.Errorf("linkback: azure devops repo %q #%d", pr.Repo, pr.Number)
	}
	u := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/pullRequests/%d/threads?api-version=7.1",
		strings.TrimRight(a.BaseURL, "/"), url.PathEscape(p[0]), url.PathEscape(p[1]),
		url.PathEscape(p[2]), pr.Number)
	return want2xx(a.Client.post(ctx, u, map[string]any{"status": "closed",
		"comments": []map[string]any{{"parentCommentId": 0, "content": text, "commentType": 1}}}))
}

func want2xx(status int, err error) error {
	if err == nil && (status < 200 || status > 299) {
		err = fmt.Errorf("host answered %d", status)
	}
	return err
}

// linkbacks is the Server's wiring: one ADO client, and one GitHub token per
// installation, minted by task 152's GitHubAppToken and kept for the process.
type linkbacks struct {
	HTTP      *http.Client
	GitHubAPI string
	ADOBase   string
	HubURL    string
	App       worksync.GitHubAppConfig // InstallationID is set per hook, below
	ADOToken  worksync.TokenFunc       // the workload identity (task 151, decision 8)

	mu     sync.Mutex
	tokens map[int64]worksync.TokenFunc
}

// For returns the hook's host's Linkback; nil when the host has none wired,
// in which case the claim is marked failed and nothing is posted.
func (l *linkbacks) For(hook hubapi.HostHook) Linkback {
	switch hook.Host {
	case hubapi.HostAzureDevOps:
		if l.ADOToken == nil {
			return nil
		}
		return ADOLinkback{Client: hostClient{HTTP: l.HTTP, Token: l.ADOToken}, BaseURL: l.ADOBase}
	case hubapi.HostGitHub:
		if hook.InstallationID == nil || l.App.Key == nil {
			return nil
		}
		return l.github(*hook.InstallationID)
	}
	return nil
}

func (l *linkbacks) github(installation int64) GitHubLinkback {
	l.mu.Lock()
	defer l.mu.Unlock()
	tok, ok := l.tokens[installation]
	if !ok {
		c := l.App
		c.InstallationID = installation
		tok = worksync.GitHubAppToken(c)
		if l.tokens == nil {
			l.tokens = map[int64]worksync.TokenFunc{}
		}
		l.tokens[installation] = tok
	}
	return GitHubLinkback{Client: hostClient{HTTP: l.HTTP, Token: tok}, BaseURL: l.GitHubAPI, HubURL: l.HubURL}
}
```

The import block above gains `sync`. `hubapi.HostHook` gains `InstallationID *int64
\`json:"installationId"\`` (a delta to task 143, null for Azure DevOps). `Server` gains one field,
`links *linkbacks`, built in `cmd/hub`'s wiring from configuration; with no GitHub App key or no
ADO identity configured, `For` answers nil and every claim for that host ends `failed`, which is
honest and visible, rather than a silent success.

#### Interfaces — complete: `server/work_hooks.go` ⚑, the author and the registration delta

```go
// HostPR (task-154.md:315-321), one field added:
	Author HostActor // who opened the PR: GitHub pull_request.user, Azure DevOps createdBy

// normaliseGitHub: the PullRequest struct gains
		User struct{ Login string } `json:"user"`
// and the "pull_request" case, after ev.PR is made:
		ev.PR.Author = HostActor{Login: pr.User.Login}

// normaliseADO: the pull-request case, after ev.PR is made:
		ev.PR.Author = HostActor{UniqueName: res.CreatedBy.UniqueName}

// putHostHook (task-154.md:333-375): the request gains installationId,
// checked before the role so a malformed body is 400 whoever sends it.
		var req struct {
			RemoteURL      string `json:"remoteUrl"`
			InstallationID *int64 `json:"installationId"`
		}
		// … after the host check:
		switch {
		case host == HostGitHub && (req.InstallationID == nil || *req.InstallationID < 1):
			return WriteResult{}, Refuse(http.StatusBadRequest, "bad_request",
				"installationId is the Hub app's installation on this repository's owner",
				map[string]string{"field": "installationId"})
		case host == HostAzureDevOps && req.InstallationID != nil:
			return WriteResult{}, Refuse(http.StatusBadRequest, "bad_request",
				"installationId is for github only", map[string]string{"field": "installationId"})
		}
		// … store.PutHostHook gains the installation (nil for Azure DevOps):
		hook, created, err := store.PutHostHook(ctx, tx, projectID, host, req.RemoteURL,
			req.InstallationID, ref, CallerID(ctx))

// … and after s.WriteLimit returns, outside the transaction:
	if host == HostGitHub {
		s.autolinks.Kick() // decision 5: the step runs now, not at the next tick
	}
```

For a GitHub author, `sender` and `pull_request.user` differ when someone else edits the body: the
filing names the PR's author, never the editor, so an admin who adds `issue: new` to another
person's PR files the issue **as that person**. That is the scope's *"created by the mapped
author"*, and it is recorded for the owner, because the alternative (the editor) is arguable.
`Kick` sends on a one-slot channel and never blocks; a kick that finds the write refused finds no
due hook, so it costs one query.

#### Interfaces — complete: `server/work_hooks.go` ⚑, `apply`'s three additions

```go
// applyResult (task-154.md:465-468) gains the claim to post after commit.
type applyResult struct {
	Duplicate    bool
	Links, Facts int
	Linkback     *pendingLinkback // nil when this delivery earned no comment
}

// pendingLinkback is decision 1: claimed inside the transaction, posted after.
type pendingLinkback struct {
	PR      hubapi.OutsidePRLink // where the comment goes: host, repo, number
	Lines   []LinkLine
	Claimed []string // hl- ids moved NULL → pending; MarkLinkback settles them
}

func (p *pendingLinkback) add(link hubapi.OutsidePRLink, line LinkLine, claimed bool) {
	p.PR = link
	p.Lines = append(p.Lines, line)
	if claimed {
		p.Claimed = append(p.Claimed, link.ID)
	}
}

// In apply's "pr" case, before task 154's refs loop:
		var lb pendingLinkback
		if (ev.PR.Action == "opened" || ev.PR.Action == "edited") && WantsFiling(ev.PR.Body) {
			if err := s.fileFromPR(ctx, tx, hook, ev, keys, &lb, &res); err != nil {
				return res, err
			}
		}

// … inside the refs loop, right after UpsertOutsidePRLink returns link:
				if alias, err := store.CurrentAlias(ctx, tx, issueID); err != nil {
					return res, err
				} else if alias != "" {
					claimed, err := store.ClaimLinkback(ctx, tx, link.ID)
					if err != nil {
						return res, err
					}
					if claimed {
						lb.add(link, LinkLine{Kind: LineLinked, Alias: alias, IssueID: issueID}, true)
					}
				}

// … after the refs loop:
		if len(lb.Lines) > 0 {
			res.Linkback = &lb
		}

// In apply's "push" case, after task 154's two loops:
		if ref, ok := ParseBranch(ev.CreatedRef, keys); ok && ev.CreatedRef != "" {
			n, f, err := s.firstPush(ctx, tx, hook, ev, keys, ref, userID, matched)
			if err != nil {
				return res, err
			}
			res.Links, res.Facts = res.Links+n, res.Facts+f
		}
```

`store.CurrentAlias` is `DisplayAlias(current key, number)` read at the claim, so the comment on a
PR that wrote `Fixes EP-142` after the key became `RS` says `RS-142`: the name a reader will find
in the product today. It is `""` while the project has no key, and then no claim is made at all,
so a key set later can still earn that PR its linkback on the next event. `ClaimLinkback` is
`UPDATE work_outside_pr_links SET linkback_state = 'pending', linkback_at = now() WHERE id = $1
AND linkback_state IS NULL RETURNING id`, true when a row came back.

#### Interfaces — complete: `server/work_hooks.go` ⚑, the filing word

```go
// fileFromPR is decision 4. The filing row goes in first: a conflict means
// this PR has filed (or failed to) before, and nothing else happens.
func (s *Server) fileFromPR(ctx context.Context, tx *Tx, hook hubapi.HostHook, ev HostEvent,
	keys []string, lb *pendingLinkback, res *applyResult) error {
	authorID, matched, err := s.people.Match(ctx, tx, hook, ev.PR.Author)
	if err != nil {
		return err
	}
	issueID := ""
	if matched {
		issueID = ids.Wide(hubapi.PrefixIssue)
	}
	fresh, err := store.InsertFiling(ctx, tx, hook.ProjectID, ev.Host, ev.Repo, ev.PR.Number,
		optional(issueID, matched)) // ON CONFLICT DO NOTHING; fresh when a row went in
	if err != nil || !fresh {
		return err
	}
	pr := hubapi.OutsidePRLink{Host: hubapi.HostKind(ev.Host), Repo: ev.Repo, Number: ev.PR.Number, URL: ev.PR.URL}
	if !matched {
		lb.add(pr, LinkLine{Kind: LineUnmatched}, false) // decision 8: once, and no login
		return nil
	}
	if err := store.SetWorkActor(ctx, tx, hubapi.ActorHost, authorID, ""); err != nil {
		return err
	}
	defer store.ClearWorkActor(ctx, tx)
	n, err := s.createFiledIssue(ctx, tx, hook.ProjectID, issueID, authorID, ev.PR)
	if err != nil {
		return err
	}
	link, _, err := store.UpsertOutsidePRLink(ctx, tx, issueID, ev, WordNonClosing)
	if err != nil {
		return err
	}
	res.Links++
	if len(keys) == 0 { // no key: filed and linked, but there is no alias to name
		return nil
	}
	claimed, err := store.ClaimLinkback(ctx, tx, link.ID)
	if err != nil || !claimed {
		return err
	}
	lb.add(link, LinkLine{Kind: LineFiled, Alias: hubapi.DisplayAlias(keys[0], n), IssueID: issueID}, true)
	return nil
}

// createFiledIssue is task 146's create sequence (task-146.md:592-627), with
// three differences: the author is the mapped person, not tx.P.UserID; the
// state is triage whatever the team's default (intake lands in triage, 05
// §3.5); and the description is the PR's link, never its body.
func (s *Server) createFiledIssue(ctx context.Context, tx *Tx, projectID, id, authorID string, pr *HostPR) (int, error) {
	teamID, err := store.ProjectTeam(ctx, tx, projectID) // UNVERIFIED name
	if err != nil {
		return 0, err
	}
	content := hubapi.IssueRevisionContent{
		Title:       hubapi.AuthoredText{Text: filedTitle(pr), Author: hubapi.AuthorHuman},
		Description: hubapi.AuthoredText{Text: "Filed from " + pr.URL, Author: hubapi.AuthorHuman},
		Serves:      []hubapi.Serve{}, Implements: []string{}}
	if hubapi.ValidateIssueRevisionContent(content) != nil { // a title the grammar refuses
		content.Title.Text = "Pull request #" + strconv.Itoa(pr.Number)
	}
	n, err := store.NextIssueNumber(ctx, tx, tx.OrgID(), projectID)
	if err != nil {
		return 0, err
	}
	head := store.IssueRow{ID: id, OrgID: tx.OrgID(), ProjectID: projectID, TeamID: teamID, Number: n,
		State: hubapi.StateTriage, Priority: hubapi.PriorityNone, Source: hubapi.SourceIntake,
		NotTiedReason: hubapi.NotTiedSupport, CreatedAt: tx.Now, UpdatedAt: tx.Now, CreatedBy: authorID}
	env := hubapi.IssueRevisionEnvelope{V: 1, Kind: "issue-revision", IssueID: id, Revision: 1,
		ProjectID: projectID, Content: content}
	digest, canonical, err := hubapi.IssueRevisionDigest(env)
	if err != nil {
		return 0, err
	}
	if err := store.InsertIssue(ctx, tx, head); err != nil {
		return 0, err
	}
	if err := store.InsertIssueRevision(ctx, tx, store.RevisionRow{OrgID: tx.OrgID(), IssueID: id,
		ProjectID: projectID, Revision: 1, Canonical: canonical, Digest: digest, WrittenAt: tx.Now,
		WrittenBy: authorID, Serves: content.Serves}); err != nil {
		return 0, err
	}
	if err := store.MoveIssueHead(ctx, tx, id, 1, tx.Now); err != nil {
		return 0, err
	}
	if err := store.SetHeadTitle(ctx, tx, id, content.Title.Text); err != nil {
		return 0, err
	}
	return n, emitWork(tx, hubapi.EventIssueCreated, teamID, projectID, id, hubapi.IssueCreatedData{
		Number: int64(n), Key: hubapi.IssueKey{IssueID: id, Revision: 1}.String(), Digest: digest.String(),
		State: string(hubapi.StateTriage), Source: string(hubapi.SourceIntake)})
}

// filedTitle is the PR title, trimmed and cut to task 02's 200 code points
// (ErrIssueTitle, task-02.md:216); an empty one becomes "Pull request #N".
func filedTitle(pr *HostPR) string {
	t := []rune(strings.TrimSpace(pr.Title))
	if len(t) > 200 {
		t = t[:200]
	}
	if len(t) == 0 {
		return "Pull request #" + strconv.Itoa(pr.Number)
	}
	return string(t)
}
```

`tx.OrgID()` is the org `s.hostTx` opened the transaction for (`task-154.md:471-474`); task 146
reads `tx.P.OrgID` because it has an Entra caller and this path has none (the accessor's name is
**UNVERIFIED**, a small gap for the Hub's `Tx`). `emitWork` is task 13's six-argument form
(`task-13.md:332`); task 154's calls are written with a four-argument form and are fixed to this
one at merge (a gap recorded below). `work_hooks.go` gains `strconv` (already added by task 154)
and `ids`.

#### Interfaces — complete: `server/work_hooks.go` ⚑, the first push and `hostFact`

```go
// firstPush is decision 7. It returns the links and facts it wrote.
func (s *Server) firstPush(ctx context.Context, tx *Tx, hook hubapi.HostHook, ev HostEvent,
	keys []string, ref Ref, userID string, matched bool) (int, int, error) {
	issueID, ok, err := store.ResolveRef(ctx, tx, hook.ProjectID, keys, ref)
	if err != nil || !ok {
		return 0, 0, err // ep-999: nothing, and the hook still answers 200
	}
	links := 0
	if n := len(ev.Commits); n > 0 {
		head := ev.Commits[n-1] // a push lists its commits oldest first (UNVERIFIED for both hosts)
		links, err = store.InsertOutsideCommitLink(ctx, tx, hubapi.OutsideCommitLink{IssueID: issueID,
			Host: hubapi.HostKind(ev.Host), Repo: ev.Repo, SHA: head.SHA, URL: head.URL,
			Word: hubapi.WordBranch, PushedBy: optional(userID, matched)})
		if err != nil {
			return 0, 0, err
		}
	}
	cur, err := store.IssueStateForUpdate(ctx, tx, issueID)
	if err != nil || !before(cur, "started") {
		return links, 0, err // already started, or declined: nothing more (decision 7)
	}
	if !matched {
		teamID, projectID, err := store.IssueTeamProject(ctx, tx, issueID) // UNVERIFIED name
		if err != nil {
			return links, 0, err
		}
		return links, 0, emitWork(tx, "issue.outside_unmatched", teamID, projectID, issueID,
			map[string]any{"issueId": issueID, "host": ev.Host, "prNumber": 0, "kind": "push"})
	}
	moved, err := s.hostFact(ctx, tx, issueID, "started", userID, "")
	if err != nil || !moved {
		return links, 0, err
	}
	return links, 1, nil
}

// hostFact (task-154.md:703-721): the cause switch gains a third arm.
	cause, move := hubapi.CauseOutsidePR, hubapi.StateMove{To: hubapi.StateInReview}
	switch fact {
	case "started":
		cause, move = hubapi.CauseOutsidePush, hubapi.StateMove{To: hubapi.StateStarted}
	case "landed":
		cause, move = hubapi.CauseOutsideMerge, hubapi.StateMove{To: hubapi.StateLanded, LandCommit: mergeCommit}
	}
```

`hostFact` calls `IssueStateForUpdate` again; the row is already locked by this transaction, so
the second read is the same value and costs one index lookup. Task 154's `in_review` arm keeps
reading `link.PRState == "open"`, and nothing here changes it.
