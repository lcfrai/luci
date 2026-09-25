### Task 14: erase for issues and comments; title search

W1, stream WH; no seam. Depends on task 12 (comments, the last text this erase must reach) and Hub
task H27 (the audit route, brief erasure and the full `hub verify`). Hub anchors were read in the
`feat/hub` worktree at `9f2fa31`: the contract (cited `hub-contract:N`), Hub task 14's part
(`H14:N`) and Hub task 27's part `docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-27.md`
(cited `H27:N`). `server/erase.go` is Hub task 27's file and every `server/` and `store/` path is
**UNVERIFIED until merge** (rule H1).

Two small things that share one column. **Erase** removes the text of an issue or a comment when
an org admin must (a client's request, a matter number pasted by mistake), and keeps everything
that makes the record findable: ids, keys, digests, revision numbers, numbers, who wrote what and
when. Every `Panacea-Issue` trailer, every chip, every `issue:` key still resolves, as `erased`
(05 §3.6; 04 §5.5). **Title search** lets the desktop's `/` and the Hub find an issue by what it is
called, over the issues the caller can read, and nothing else.

The shared column is `issues.head_title`: the head revision's title, the one piece of issue text
kept outside `canonical`. Search needs it (matching inside canonical bytes would mean decoding
every revision on every query), and erase must blank it, which is why both live in one task and
why the erase test reads **every** text column in the schema rather than a list: a denormalised
copy is exactly what a list forgets.

Five decisions the scope left open, made here.

1. **Erase follows Hub task 27's brief erase, route for route.** Org admin only: every team role
   gets 403 `forbidden`, `required: org_admin` (`H27:31-34`); the issue or comment is found by id
   under row-level security alone, with no team check, so an org admin who is in no team can still
   erase, and one of another org gets 404 (`issue_not_found`, `comment_not_found`). A second
   erase answers zeros **and is still audited** with its own event: an erasure attempt is a
   security event (`H27:37-39`, ADR-H1 §5.6). The scope's "a second erase answers zeros" holds;
   the second event is Hub task 27's rule, followed.
2. **An issue's erase takes its comments with it.** One transaction calls task 07's
   `hub.erase_issue(issueId)` and then `hub.erase_comment` for every comment on the issue not yet
   erased, and appends **one** `issue.erased` event with both counts (`{erasedRevisions,
   erasedComments}`, task 13's `IssueErasedData`), as `brief.erased` is one event for many
   revisions. A comment's own erase appends one `comment.erased`.
3. **Two columns task 07's function does not blank are blanked here.** `issues.head_title` (added
   here) and `issues.state_reason` (a hand-set *done, not proven* reason is a person's words).
   `erase_issue` is **replaced** (`CREATE OR REPLACE`, same signature, same org check) by a
   section appended to this plan's W1 migration; task 15 appends one more replacement for its
   `external_author_name`. Task 07's `guard_issue` needs no change: after `erased_at` is set it
   already refuses every change by anyone but `hub_owner` (`task-07.md:164`), so `hub_app` can
   never write a title or a reason back onto an erased issue. This extends task 07's SQL in this
   plan's own file, in task order; it is not a Hub file.
4. **The head carries its title, additively.** Task 03's `IssueHead` has no title
   (`task-03.md:282-303`), so a search hit, like every list row, would be a number with no name.
   With `head_title` stored, every head read fills `IssueHead.Title` (`""` once erased). Because
   the head now holds issue text, **every stored replay that holds a head blanks `head.title`** as
   it already blanks revision content: task 08's create (`task-08.md:378`), task 09's revision
   writes and task 11's relation reply. Task 14's canary test is the check that none was missed.
5. **Search is `ILIKE` on `head_title`, with no index.** At 10,000 issues its speed is
   **unmeasured**; a trigram index (`pg_trgm`) is a later task if a measurement asks for it, and
   whether the Hub's Postgres build ships `pg_trgm` is **UNVERIFIED**. `q` is matched as a literal:
   `%`, `_` and `\` in it are escaped, so a person typing `50%` finds `50%` and not everything.

**The one thing this task must not get wrong** is leaving text behind. The brief erase's test
found the two places text hides: a denormalised column and a stored idempotency reply
(`H27:72`). Tasks 08, 11 and 12 store their replies without text (`task-08.md:45-51`; task 11's
*Replay storage*; task 12's decision 5), and `head_title` is blanked here; the canary test then
proves it by scanning **every** `text`, `varchar`, `jsonb` and `bytea` column of schema `hub`,
using Hub task 27's `markerHits` helper unchanged.

**Blobs are not deleted.** An attachment names a project blob by digest; whether the Hub deletes a
project blob when the record that attached it is erased is the Hub plan's decision, and Hub task
27 erases briefs only. Attached blobs stay, and the part names it as open (**UNVERIFIED**).

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's, additive): `IssueHead.Title`,
  `IssueEraseResponse`, `CommentEraseResponse`, `EscapeLike`, `MaxSearchLimit`; the request body
  is the Hub's own `hubapi.EraseRequest` (`types.go:565-567`, verified at `9f2fa31`), reused, not
  redeclared
- `control-plane/internal/hub/hubapi/work_search_test.go` (new): the pure rules
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` ⚑ (task 06's file; a section appended,
  or the next free number if task 06 has merged into `feat/hub`, rule H1)
- `control-plane/internal/hub/server/erase.go` ⚑ (Hub task 27's file, `H27:15`): two routes
  registered beside `POST /v1/briefs/{briefId}/erase`, which is not changed
- `control-plane/internal/hub/store/work_erase.go`, `store/work_search.go`,
  `server/work_search.go` (new)
- `control-plane/internal/hub/server/work_erase_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/store/work_issues.go`, `store/work_revisions.go` ⚑ (tasks 08 and 09,
  this plan's own files): `SetHeadTitle` beside every `MoveIssueHead`; the head projection reads
  `head_title`
- `control-plane/internal/hub/server/work_{issues,revisions,relations}.go` ⚑ (tasks 08, 09, 11):
  each `Stored` reply blanks `head.title` (decision 4)

**Interfaces:**

- Produces:
  - the routes, in 07's list and 07's matrix rows: `POST /v1/issues/{issueId}/erase` and `POST
    /v1/comments/{commentId}/erase` (✓ for `O` only; 403 `required: org_admin` for `V M A N X`,
    as route 45's row, `H27:94`), and `GET /v1/issues/search?q=&projectId=&limit=`
    (`Page[IssueHead]`; ✓ for any caller, over what they can read)
  - `hubapi.IssueHead.Title`; `hubapi.IssueEraseResponse`, `hubapi.CommentEraseResponse`
  - `store.EraseIssue`, `store.EraseComment`, `store.SearchIssueTitles`, `store.SetHeadTitle`
  - the events `issue.erased` and `comment.erased` (task 13's constants and structs)
- Consumes:
  - task 07: `hub.erase_issue`, `hub.erase_comment` (`task-07.md:273-303`), replaced here as
    decision 3 says; `guard_issue` (`:146-171`)
  - task 08: the head projection, `MoveIssueHead`, the `Stored` reply of the create; task 09: the
    revision write and restore paths; task 12: `issue_comments`; task 13: `emitWork`,
    `IssueErasedData`, `CommentErasedData`
  - Hub task 27: route 45's shape and the audit's `reason` rule (`H27:31-46`); `markerHits`
    (`H27:72`), the helper its test declares, reused in this task's test
  - Hub task 14: `server.Write`, `server.Read`, `server.Refuse`, `server.Forbidden`
- Consumed by: task 16 (the relay's search and erase paths), task 17 (the end-to-end erase row),
  task 57 (the replica's erase handling), task 58 (the desktop's `/` searches the replica and
  falls back to this route)

#### The migration section (appended to `NNNN_work_w1.sql`, complete)

```sql
-- ── task 14: the head's title, for search; blanked by erase ─────────────────────
-- The only issue text outside issue_revisions.canonical. Written with the head move
-- (tasks 08, 09), read by every head projection and by search, blanked by erase_issue.
ALTER TABLE issues ADD COLUMN head_title text NOT NULL DEFAULT ''
  CHECK (octet_length(head_title) <= 1024);
GRANT UPDATE (head_title) ON issues TO hub_app;

-- erase_issue, replaced: task 07's body plus head_title and state_reason (decision 3).
CREATE OR REPLACE FUNCTION erase_issue(p_issue text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = p_issue AND i.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: work: no issue % in this org', p_issue USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE issue_revisions SET canonical = NULL, erased_at = now()
   WHERE issue_id = p_issue AND canonical IS NOT NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  UPDATE issues SET external_url = '', head_title = '', state_reason = '',
                    erased_at = coalesce(erased_at, now()), updated_at = now()
   WHERE id = p_issue AND erased_at IS NULL;
  RETURN v_n;
END $$;
REVOKE ALL ON FUNCTION erase_issue(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_issue(text) TO hub_app;
```

The title cap is 1024 bytes because task 02's title limit is 200 characters (`ErrIssueTitle`,
`task-02.md:216`, `MaxIssueTitleChars`), and 200 characters of UTF-8 are at most 800 bytes; the `CHECK` is the
second layer, never the answer a person sees. `CREATE OR REPLACE` keeps the function's owner and
`SECURITY DEFINER`; the `REVOKE`/`GRANT` pair is repeated because a reader of this section should
not need task 07's to know who may call it. Task 06's `TestWorkGrantsAreExactlyTheList` gains
`issues.head_title`.

#### `hubapi/work.go`, the additions (complete)

```go
// ── Task 14: erase and title search ─────────────────────────────────────────

// Additive field on task 03's IssueHead (shown as the diff):
//   Title string `json:"title"` // the head revision's title; "" once erased

// IssueEraseResponse is POST /v1/issues/{issueId}/erase's 200. A second erase
// answers zeros (and is still audited, H27:37-39).
type IssueEraseResponse struct {
	IssueID         string `json:"issueId"`
	ErasedRevisions int    `json:"erasedRevisions"`
	ErasedComments  int    `json:"erasedComments"`
	ErasedAt        string `json:"erasedAt"` // the first erasure's time
}

// CommentEraseResponse is POST /v1/comments/{commentId}/erase's 200.
type CommentEraseResponse struct {
	CommentID string `json:"commentId"`
	Erased    int    `json:"erased"` // 1 the first time, 0 after
	ErasedAt  string `json:"erasedAt"`
}

const (
	MaxSearchQueryChars = 200
	MaxSearchLimit      = 50
)

// EscapeLike makes q a literal for ILIKE … ESCAPE '\': a person typing "50%"
// finds "50%", not every title.
func EscapeLike(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(q)
}
```

#### The store (`store/work_erase.go`, `store/work_search.go`, signatures complete)

```go
package store

// EraseIssue runs, in the caller's write transaction: SELECT hub.erase_issue($1);
// then SELECT hub.erase_comment(id) FROM issue_comments WHERE issue_id = $1 AND
// erased_at IS NULL, summing the results. P0002 (no_data_found) is ErrNotInOrg.
func EraseIssue(ctx context.Context, tx pgx.Tx, issueID string) (revisions, comments int, erasedAt time.Time, err error)

// EraseComment: SELECT hub.erase_comment($1). P0002 is ErrNotInOrg.
func EraseComment(ctx context.Context, tx pgx.Tx, commentID string) (n int, erasedAt time.Time, err error)

// IssueForErase and CommentForErase find the record by id under row-level
// security alone, with no team check (H27:32-34: an org admin need not be a member).
func IssueForErase(ctx context.Context, tx pgx.Tx, issueID string) (teamID, projectID string, found bool, err error)
func CommentForErase(ctx context.Context, tx pgx.Tx, commentID string) (teamID, projectID, issueID string, found bool, err error)

var ErrNotInOrg = errors.New("store: no such record in this org")

// SetHeadTitle: UPDATE issues SET head_title = $2 WHERE id = $1. Called beside
// every MoveIssueHead (task 08's create, task 09's write and restore).
func SetHeadTitle(ctx context.Context, tx pgx.Tx, issueID, title string) error

// SearchIssueTitles is the search query. alias, when non-nil, is the issue a
// q like "EP-142" names, placed first; the rest are title matches, newest
// first, excluding erased issues and the alias hit.
//   SELECT … FROM issues i JOIN projects p ON p.id = i.project_id
//    WHERE i.erased_at IS NULL
//      AND i.team_id IN (SELECT team_id FROM team_members WHERE user_id = $1)
//      AND ($2 = '' OR i.project_id = $2)
//      AND i.head_title ILIKE '%' || $3 || '%' ESCAPE '\'
//    ORDER BY i.created_at DESC, i.id DESC LIMIT $4
func SearchIssueTitles(ctx context.Context, q Querier, userID, projectID, escapedQ string, limit int) ([]IssueRow, error)

// IssueByAlias resolves "KEY-N" through project_keys at query time, inside the
// caller's readable teams. It is never stored and never treated as a key.
func IssueByAlias(ctx context.Context, q Querier, userID, projectID, key string, n int) (IssueRow, bool, error)
```

The readability join is the one the change feed uses (`H26:27-28`): a team the caller is in, or a
project of such a team. Row-level security already bounds the query to the caller's org; the
membership join bounds it to the caller's teams, and a test proves each half.

#### The routes (`server/erase.go` ⚑, `server/work_search.go`)

```go
// In Hub task 27's registerErase, beside route 45, which is not changed:
mux.HandleFunc("POST /v1/issues/{issueId}/erase", s.eraseIssue)
mux.HandleFunc("POST /v1/comments/{commentId}/erase", s.eraseComment)

// work_search.go
mux.HandleFunc("GET /v1/issues/search", s.searchIssues)
```

Go 1.22's mux prefers the more specific pattern, so `GET /v1/issues/search` never reaches task
08's `GET /v1/issues/{issueId}`. `search` is not an id (`is-` + 16 hex, task 02), so no issue can
shadow the route. A test pins it (`TestSearchIsNotAnIssueID`).

**`eraseIssue`**, inside `s.Write`, in this order: the principal is not an org admin →
`server.Forbidden("org_admin")`; `server.Decode[hubapi.EraseRequest]`; `reason` trimmed 1-500
characters → else 400 `field: "reason"`; `IssueForErase` → not found is 404 `issue_not_found`;
`EraseIssue`; `emitWorkErase(tx, EventIssueErased, team, project, issueID,
IssueErasedData{revisions, comments}, reason)` (below), appended on a second erase too; 200
`IssueEraseResponse`. `eraseComment` is the same with `CommentForErase`, `EraseComment`, 404
`comment_not_found` and `comment.erased`.

**The reason reaches the audit and not the feed.** Hub task 27 writes `brief.erased` with
`{"erasedRevisions", "reason"}` and the feed strips `reason` (`H27:36-37`; hub-contract §8's
`brief.erased` row). The Work erase follows the same path: the event's stored `data` holds
`reason`, and the feed's projection drops it for `issue.erased` and `comment.erased` exactly as it
does for `brief.erased`. So task 13's `server/work_emit.go` gains a sibling of `emitWork`,
`emitWorkErase(tx, kind, teamID, projectID, subject string, data any, reason string) error`, which
runs `emitWork`'s checks, accepts only the two erase kinds, and adds `"reason"` to the map; Hub
task 26's projection lists the two kinds beside `brief.erased`; and task 13's
`TestTheHubWritesOnlyDeclaredWorkKinds` allows exactly one extra stored key, `reason`, on exactly
these two kinds. How task 26 strips the key (a per-kind list or a rule) is **UNVERIFIED** until its
code lands; this task adds its two kinds to whichever it is. `IssueErasedData` itself has no
`reason` field, so nothing but the erase handlers can put one there.

**`searchIssues`**, inside `s.Read`: `q` trimmed 1-`MaxSearchQueryChars` characters → else 400
`field: "q"`; `limit` 1-50, 0 or absent meaning 50 → else 400 `field: "limit"`; `projectId`, when
set, a project the caller can read → else 404 `project_not_found`. Then:

1. **The alias first.** When `q` matches `^([A-Z]{2,5})-([1-9][0-9]{0,8})$` after trimming, and the
   key part satisfies task 02's `ValidProjectKey` (`task-02.md:167`), `IssueByAlias` looks it up
   in the caller's readable projects **through `project_keys` now**; a hit goes first. The alias
   is matched case-sensitively, as `DisplayAlias` writes it (`task-02.md:179`); `ep-142` is a
   title search. An alias is never stored, never parsed as a key, and a key that moved (`EP` →
   `PAY`) finds the issue under `PAY-142` and not `EP-142`.
2. **Then the titles.** `SearchIssueTitles` with `EscapeLike(q)`; the alias hit, if any, is
   dropped from this list so it appears once.
3. `Page[IssueHead]` with `next: null` (search does not page in W1: at most 50, the desktop narrows
   `q`), each head built by task 08's projection, so `alias`, `title` and `threadState` read as on
   every other route.

```jsonc
// GET /v1/issues/search?q=resume&projectId=pj-…&limit=20     any caller; their readable issues only
{ "items": [
    { "id": "is-0a1b…", "projectId": "pj-…", "number": 142, "alias": "EP-142",
      "title": "Resume a dropped upload", "state": "todo", … IssueHead … } ],
  "next": null }

// POST /v1/issues/is-0a1b…/erase                               org admin only
{ "reason": "Client request 2026-11-02, matter 4471" }
// → 200
{ "issueId": "is-0a1b…", "erasedRevisions": 3, "erasedComments": 5, "erasedAt": "2026-11-02T01:15:07.000Z" }
// Again → 200 { … "erasedRevisions": 0, "erasedComments": 0, "erasedAt": "<the first time>" }

// POST /v1/comments/ic-4e2f…/erase                             org admin only
{ "reason": "Pasted a customer password" }
// → 200 { "commentId": "ic-4e2f…", "erased": 1, "erasedAt": "…" }

// GET /v1/issues/is-0a1b… after the erase: number, key, alias, digests intact; text gone
{ "id": "is-0a1b…", "number": 142, "alias": "EP-142", "title": "", "stateReason": "",
  "headKey": "issue:is-0a1b…@3", "external": { "source": "linear", "externalId": "LIN-88",
  "url": "", "authorName": "" }, … }
```

After an erase every text reads `""`, and the record says so: `IssueRevision.erasedAt` and
`Comment.erasedAt` are set (task 03's decision (e)), and `/v1/refs` answers each revision key
`erased` with its digest (task 09). An erased issue refuses every further write with 409
`conflict`, because task 07's guard refuses any `hub_app` change after `erased_at`
(`task-07.md:164`) and the handlers check first so the answer is a 409, not a 500.

#### Tests to write first, part 1 — `hubapi/work_search_test.go` (pure)

```go
package hubapi

import "testing"

func TestEscapeLikeMakesTheQueryLiteral(t *testing.T) {
	for in, want := range map[string]string{
		"resume": "resume", "50%": `50\%`, "a_b": `a\_b`, `c:\tmp`: `c:\\tmp`, `%_\`: `\%\_\\`,
	} {
		if got := EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
```

| Test | Mutant it kills |
|---|---|
| `TestEscapeLikeMakesTheQueryLiteral` | a search for `50%` or `a_b` that matches every title, and a backslash that escapes the next character instead of itself |

