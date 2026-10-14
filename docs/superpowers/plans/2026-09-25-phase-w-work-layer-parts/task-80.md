### Task 80: views `vw-`, drafts `df-`, https links and the read-only page, CSV export

**Slice:** W2. **Size:** the skeleton says 3 hours; this part says **4 to 5**, and says why under
*Corrections* — the read-only page is a second HTTP surface with its own session, and it does not
fit beside four route groups in three hours. **Stream:** WH. **Depends on** task 74 (the `views`,
`drafts` and `issue_aliases` tables, the draft policy, the one-default index). **Seam:** none of
its own; it fills SW.18's view, draft and export methods.

**Anchors, re-read on 2026-09-27** (parts of this branch, none yet code): `task-72.md:872-958`
(`WorkPlan`'s view, draft and export methods), `:351-385` (the codes), `:394-410`;
`task-74.md`'s `views`, `drafts`, `views_one_team_default` and `draft_author_only`;
`task-08.md:312-323` (`s.route`, the recorder), `:333` (`ProjectRole`); `task-75.md:86-88` (the
matrix file's real name); `task-77.md` (the janitor, which expires drafts, not this task);
`task-79.md`'s `movedFrom` on `by-number`. **UNVERIFIED until merge:** the Hub contract's
*"No browsers"* line at `hub-contract.md:38`, quoted from the scope and not re-read on
`feat/hub`; the Entra auth-code parameters the page's session needs (`OD-30`'s Keycloak broker is
the desktop's path, and the page's is proposed, not ruled).

#### Decisions the scope left open, made here

**(a) One evaluator, in one place, run twice.** `store/work_filter.go` holds the only Go function
that reads a `FilterGroup`: `Where(g hubapi.FilterGroup) (sql string, args []any, err error)`. It
builds a parameterised `WHERE` clause and never a string a value is pasted into. The desktop's
TypeScript evaluator (tasks 83 and 91) is a second implementation, which is why the vectors in
`hubapi/testdata/view-filter/*.json` are **shared data, not a shared library**: each side runs the
same file and must give the same ids.

**(b) Row-level security is the filter's floor, not its ceiling.** `Where` never adds an org or a
project clause: `hub_app` cannot see another org's rows at all (task 74), and the project read is
the route's `ProjectRole` check. A vector that names a project the caller cannot read therefore
returns no rows rather than 403 — a filter is a query, and a 403 on a filter leaks which projects
exist.

**(c) A view is a team's; a personal view is a desktop preference** (L-94). Nothing personal is
stored in the Hub, so `POST /v1/teams/{teamId}/views` takes a member and `PUT
/v1/views/{viewId}/team-default` takes a team admin (L-105). The one-default index (task 74) means
a second `PUT` clears the first in the same transaction, and both writes are one event.

**(d) A draft is a person's, and its absence is indistinguishable from another person's.** Every
draft route answers `404 draft_not_found` for a draft that is not the caller's, and the storage
says the same (task 74's `RESTRICTIVE` policy means the row is not readable at all). There is no
403 on a draft anywhere in the Hub (L-33).

**(e) CSV is a rendering of a read, never a second query path.** `GET /issues.csv?viewId=` runs
the same store call the JSON list route runs, with the same `ProjectRole` check and the same
`Where` clause, then writes cells. A caller who cannot read the project gets the route's 403
before a row is fetched; a row the caller cannot read never reaches the writer, because the
database did not return it.

**(f) Every cell is escaped twice: once as CSV, once against a spreadsheet.** `encoding/csv` quotes
commas, quotes and newlines. On top of it, a cell whose first byte is `=`, `+`, `-`, `@`, a tab or
a carriage return is prefixed with an apostrophe. This is applied to **every** cell, not only to
titles: an assignee's display name, a label and a state are all attacker-influenced somewhere.

**(g) An erased issue exports and renders as the word `erased`.** Its alias, its state and its
timestamps are facts about the thread and stay; its title, description and comments are gone
(05 §3.6). The CSV writes `erased` in the title cell and empty strings elsewhere; the page renders
one line and no revision.

**(h) The page is a separate `http.ServeMux` on the same listener, mounted at `/p/`.** It shares
no middleware with `/v1`: no bearer parsing, no caller key, no relay. Its session cookie is
rejected by `/v1` because `/v1`'s authenticator reads the `Authorization` header and nothing else
— that is asserted as a test, not assumed. The page sets `Content-Security-Policy: default-src
'none'; style-src 'self'; img-src 'self'; form-action 'none'; frame-ancestors 'none'`, with
`script-src 'none'` named explicitly so a reader sees it.

**(i) The page ships behind a flag and merges only after the contract owner reviews it.**
`PANACEA_APP_HUB_PAGES=off` by default. The proposal document is written in this task; the route
exists in this task; the flag defaults to on only when the `feat/hub` owner has answered.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/store/work_filter.go` | new: `Where`, the only Go reader of `FilterGroup` |
| `control-plane/internal/hub/store/work_filter_test.go` | new: the shared vectors, no build tag |
| `control-plane/internal/hub/store/work_views.go` | new: the view rows and the default swap |
| `control-plane/internal/hub/server/work_views.go` | new: five routes |
| `control-plane/internal/hub/store/work_drafts.go` | new: four calls, all scoped by `hub.user_id` |
| `control-plane/internal/hub/server/work_drafts.go` | new: four routes |
| `control-plane/internal/hub/server/work_export.go` | new: two CSV routes and `csvCell` |
| `control-plane/internal/hub/server/work_pages.go` | new: the `/p/` mux, its session, its template |
| `control-plane/internal/hub/server/work_pages_template.html` | new: one file, no script tag |
| `control-plane/internal/hub/hubapi/testdata/view-filter/*.json` | new: 12 vectors, read by Go here and by TypeScript in tasks 83 and 91 |
| `control-plane/internal/hub/server/work_views_integration_test.go`, `work_drafts_integration_test.go`, `work_export_integration_test.go`, `work_pages_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ | `workMatrixRows["80"]`; `"80"` in `workMatrixLive` |
| `control-plane/internal/hub/server/server.go` ⚑ | `s.registerWorkViews()`, `s.registerWorkDrafts()`, `s.registerWorkExport()`, and the `/p/` mount behind the flag |
| `control-plane/cmd/hub/config.go` ⚑ | `PANACEA_APP_HUB_PAGES`, `PANACEA_APP_HUB_PAGES_BASE_URL` |
| `docs/superpowers/planning/2026-09-25-linear-parity/w2-hub-browser-surface-proposal.md` | new: the proposal for the `feat/hub` owner |

#### The shared filter vectors (complete shape)

```json
{
  "name": "or-of-two-states",
  "issues": [
    {"id": "is-1", "state": "todo",    "labels": ["bug"],  "assigneeId": "u-1", "estimate": 3},
    {"id": "is-2", "state": "started", "labels": [],       "assigneeId": null,  "estimate": null},
    {"id": "is-3", "state": "done",    "labels": ["bug"],  "assigneeId": "u-2", "estimate": 8}
  ],
  "filter": {"op": "or", "conditions": [
    {"field": "state", "cmp": "eq", "value": "todo"},
    {"field": "state", "cmp": "eq", "value": "started"}
  ]},
  "expected": ["is-1", "is-2"]
}
```

Twelve vectors, each one rule: `and-of-two`, `or-of-two`, `nested-and-in-or`, `not-of-one`,
`in-a-list`, `is-empty-assignee`, `is-not-empty-estimate`, `label-contains-any`,
`label-contains-all`, `due-before-a-date`, `text-contains-case-folded`, and `unknown-field`
(whose `expected` is the string `"error"`, so both evaluators must refuse rather than match
everything). The Go side seeds `issues` into the database and runs `Where`; the TypeScript side
runs its evaluator over the same array in memory. Neither side may edit a vector without the
other's test failing in the same pull request — that is the whole point of the file.

#### The routes, in order

**Views.**

1. `POST /v1/teams/{teamId}/views` (member) — `CreateViewRequest{name, filter, grouping, sort}`.
   `Where(filter)` is called at write time and a build error is `422 filter_invalid` with
   `details.field` the offending path (`conditions[2].field`). 201 with the `View`, one
   `view.created`.
2. `GET /v1/teams/{teamId}/views` (viewer) — the team's unarchived views, name order, the default
   first.
3. `PATCH /v1/views/{viewId}` (member; the creator or any member, since the view is the team's) —
   name, filter, grouping, sort. One `view.updated` with `Fields` the changed names.
4. `POST /v1/views/{viewId}/archive` (member) — sets `archived_at`; an archived view that was the
   default clears the flag in the same write. One `view.archived`.
5. `PUT /v1/views/{viewId}/team-default` (**team admin**, L-105) — `{isDefault: bool}`. Setting it
   clears the previous default in the same transaction: two rows, **one** `view.default_changed`
   event naming `fromViewId` and `toViewId`, because it is one act.

**Drafts.** Every one sets `hub.user_id` from the token before the query, so task 74's
`RESTRICTIVE` policy does the work and the Go check is the second line, not the only one.

6. `POST /v1/me/drafts` (member) — `{projectId, body}`; `expires_at = now() + 30 days`; 201 with
   `Draft`. **No event**: a draft is not on the provenance thread until it becomes an issue.
7. `PUT /v1/me/drafts/{draftId}` — replaces `body`, bumps `updated_at` and `expires_at`. Not the
   caller's → `404 draft_not_found`.
8. `GET /v1/me/drafts[/{draftId}]` — the caller's list, `updated_at` descending, or one draft.
9. `DELETE /v1/me/drafts/{draftId}` — 204. Not the caller's → `404`. Expiry is task 77's janitor.

**Export.**

10. `GET /v1/projects/{projectId}/issues.csv?viewId=` (viewer) — `text/csv; charset=utf-8`,
    `Content-Disposition: attachment; filename="<key>-issues-<date>.csv"`. A UTF-8 BOM leads the
    body, because Excel reads a BOM-less UTF-8 CSV as the system code page. Columns, fixed:
    `alias, title, state, subState, priority, assignee, estimate, dueOn, labels, cycle, feature,
    parent, createdAt, updatedAt, url`. `viewId` absent exports the project's unarchived issues.
11. `GET /v1/projects/{projectId}/features.csv` (viewer) — `alias, title, state, branchRef,
    briefId, issues, createdAt, url`.

```go
// csvCell escapes a value against a spreadsheet, on top of encoding/csv's own
// quoting. Every cell goes through it, not only free text (decision (f)).
func csvCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
```

**The page.**

12. `GET /p/issues/{issueId}` and `GET /p/issues/{issueId}@{rev}` — server-rendered HTML, no
    script, the CSP of decision (h), a `HttpOnly; Secure; SameSite=Strict` session cookie set by
    the Entra auth-code exchange at `GET /p/auth/callback`, and a body of: the alias and title, the
    state and sub-state, the assignee, the named revision (or the head) rendered read-only, the
    `serves` criteria with their proof words, and an **Open in Panacea** anchor to
    `panacea://issue/{issueId}` (task 93). No comment box, no form, no `POST` anywhere under
    `/p/`. A reader with no project read gets `404`, never `403`.

#### The store (signatures complete)

```go
// store/work_filter.go — the only Go reader of a FilterGroup.
func Where(g hubapi.FilterGroup, next int) (clause string, args []any, err error)

// store/work_views.go
func (s *Store) CreateView(ctx context.Context, tx pgx.Tx, v hubapi.View) (hubapi.View, error)
func (s *Store) ListViews(ctx context.Context, teamID string) ([]hubapi.View, error)
func (s *Store) PatchView(ctx context.Context, tx pgx.Tx, id string, p hubapi.ViewPatch) (hubapi.View, error)
func (s *Store) ArchiveView(ctx context.Context, tx pgx.Tx, id string) (hubapi.View, error)
// SetTeamDefaultView clears the team's previous default and sets this one in
// the caller's transaction, and returns the id it cleared ("" when there was
// none) so the route can name it in one event.
func (s *Store) SetTeamDefaultView(ctx context.Context, tx pgx.Tx, id string, on bool) (cleared string, _ error)

// store/work_drafts.go — each takes the caller's id and relies on the
// RESTRICTIVE policy as well; a miss is ErrNotFound, never ErrForbidden.
func (s *Store) CreateDraft(ctx context.Context, tx pgx.Tx, d hubapi.Draft) (hubapi.Draft, error)
func (s *Store) PutDraft(ctx context.Context, tx pgx.Tx, id string, body json.RawMessage) (hubapi.Draft, error)
func (s *Store) ListDrafts(ctx context.Context, authorID string) ([]hubapi.Draft, error)
func (s *Store) GetDraft(ctx context.Context, id string) (hubapi.Draft, error)
func (s *Store) DeleteDraft(ctx context.Context, tx pgx.Tx, id string) error

// store/work_views.go, the export and page reads. Both stream: the rows are
// written to the response as they come, so a 20,000-issue project does not
// build a slice first.
func (s *Store) EachIssueForExport(ctx context.Context, projectID string, w hubapi.FilterGroup,
	fn func(hubapi.IssueExportRow) error) error
func (s *Store) EachFeatureForExport(ctx context.Context, projectID string,
	fn func(hubapi.FeatureExportRow) error) error
func (s *Store) IssueForPage(ctx context.Context, issueID, rev string) (hubapi.IssuePage, error)
```

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑), complete

The fixture gains `ViewID` (a team view made by Grace) and `DraftID` (a draft made by **Linus**,
so the member column reads its own and the others read a miss). Drafts need a shape of their own,
because the right answer for five of the six principals is `404`, not `403`:

```go
// wmine: the caller's own row. Everyone who can reach the route gets 404 on a
// row that is not theirs; nobody gets 403 (L-33, decision (d)).
func wmine(p, method, path string, ok int, body func(*fixture) any) workRow {
	return workRow{p, row{name: p, method: method, path: path, body: body,
		want: [6]int{404, ok, 404, 404, 404, 403}}}
}

	"80": {
		wwrite("POST /v1/teams/{teamId}/views", "POST", "/v1/teams/{TeamID}/views", 201, "team_not_found",
			func(*fixture) any { return map[string]any{"name": "My work", "filter": anyFilter()} }),
		wread("GET /v1/teams/{teamId}/views", "/v1/teams/{TeamID}/views", "team_not_found"),
		wwrite("PATCH /v1/views/{viewId}", "PATCH", "/v1/views/{ViewID}", 200, "view_not_found",
			func(*fixture) any { return map[string]any{"name": "Mine"} }),
		wwrite("POST /v1/views/{viewId}/archive", "POST", "/v1/views/{ViewID}/archive", 200, "view_not_found", empty),
		wadmin("PUT /v1/views/{viewId}/team-default", "PUT", "/v1/views/{ViewID}/team-default", 200, "view_not_found",
			func(*fixture) any { return map[string]any{"isDefault": true} }),
		wwrite("POST /v1/me/drafts", "POST", "/v1/me/drafts", 201, "",
			func(f *fixture) any { return map[string]any{"projectId": f.ProjectID, "body": h("{}")} }),
		wmine("PUT /v1/me/drafts/{draftId}", "PUT", "/v1/me/drafts/{DraftID}",
			200, func(*fixture) any { return map[string]any{"body": h("{}")} }),
		wmine("GET /v1/me/drafts/{draftId}", "GET", "/v1/me/drafts/{DraftID}", 200, nil),
		wmine("DELETE /v1/me/drafts/{draftId}", "DELETE", "/v1/me/drafts/{DraftID}", 204, nil),
		wread("GET /v1/projects/{projectId}/issues.csv", "/v1/projects/{ProjectID}/issues.csv", "project_not_found"),
		wread("GET /v1/projects/{projectId}/features.csv", "/v1/projects/{ProjectID}/features.csv", "project_not_found"),
	},
```

`POST /v1/me/drafts` takes `wwrite` because it creates the caller's own row: a viewer is 403 (a
viewer files no work), the sync principal 403, everyone else 201. The page is **not** a matrix
row: it is not a `/v1` route, its principals are browser sessions, and its own test file covers
it.

#### Tests to write first, each with the mutant it kills

`store/work_filter_test.go` is a unit file (the vectors seed and query a database, so it carries
`//go:build integration` after all — stated, because the scope implies it is pure and it is not).
The four server files carry the tag too.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEachFilterVector` | the 12 files in `testdata/view-filter/`, each seeded and queried → the ids of `expected`, in that order; `unknown-field` → `422 filter_invalid` | `or` evaluated as `and`; an unknown field matching everything |
| `TestAFilterIsNeverInterpolated` | a vector whose value is `'; DROP TABLE hub.issues --` → zero rows and the table still there; `Where` returns `$n` placeholders and no literal | a value pasted into the clause |
| `TestOnlyATeamAdminSetsTheDefault` | Linus (member) `PUT team-default` → 403; Grace → 200; a second view set → the first cleared, one `view.default_changed` naming both | any member reordering the team (L-105) |
| `TestAnArchivedDefaultIsNotTheDefault` | archive the default → `is_team_default` false, `GET /views` names no default | a dangling default |
| `TestAnothersDraftIsFourOhFour` | Grace `GET`, `PUT` and `DELETE` Linus's draft → `404 draft_not_found` each, and the row is untouched | `403`, which leaks that the draft exists (L-33) |
| `TestADraftWritesNoEvent` | create, update and delete a draft → `/v1/changes` unchanged | a private note on the provenance thread |
| `TestACsvCellIsDefused` | a title `=HYPERLINK("http://x","click")` → the cell is `'=HYPERLINK…`; `+1`, `-1`, `@x`, a leading tab → each prefixed; `a=b` → unchanged | a raw cell, which is a formula in Excel |
| `TestTheCsvHoldsOnlyReadableRows` | a project the caller cannot read → 403 before any row; a view whose filter names two projects → only the readable one's rows | an export that bypasses the route's check |
| `TestAnErasedIssueCarriesNoText` | erase an issue → its CSV row reads `erased` in `title`, empty elsewhere, with its alias and state intact; `GET /p/issues/{id}` renders one line and no revision | a cached title surviving an erase (05 §3.6) |
| `TestThePageSendsItsCsp` | `GET /p/issues/{id}` → `Content-Security-Policy` contains `script-src 'none'` and `frame-ancestors 'none'`; the body contains no `<script` and no `on*=` attribute | a script tag, or a CSP on the wrong surface |
| `TestThePageCookieIsNotAnApiToken` | the page's session cookie sent to `GET /v1/projects/{id}/issues` → 401, and no `Set-Cookie` ever comes back from `/v1` | a browser session honoured by the API (hub-contract §9) |
| `TestThePageIsOffByDefault` | `PANACEA_APP_HUB_PAGES` unset → `GET /p/issues/{id}` is 404 and the mux is not mounted | a reviewed-nowhere surface shipping on |
| `TestThePageNeedsTheProjectRead` | a person outside the project → `404`, not `403`, and no title in the body | an existence leak on a link that can be pasted anywhere |
| `TestEachWriteHasItsEvents` | the five view writes write exactly their kinds; the four draft routes and the two CSV routes write none | a silent write; a draft on the feed |

#### The proposal document (its sections, complete)

`docs/superpowers/planning/2026-09-25-linear-parity/w2-hub-browser-surface-proposal.md`, for the
`feat/hub` owner, in W0's shape: **what** (one read-only HTML page under `/p/`, no script, no
form); **why** (L-98 and L-233: a link a person pastes in Teams must open something for a reader
who has no desktop yet); **what it is not** (no write, no `/v1` route, no cookie the API honours);
**the contract line it touches** (*"No browsers"*, `hub-contract.md:38`, which this reads as a rule
about `/v1`, and asks the owner to confirm or refuse); **the session** (Entra auth code into an
`HttpOnly`, `SameSite=Strict`, 12-hour cookie, no refresh); **the flag** (`PANACEA_APP_HUB_PAGES`,
off until the answer); **the tests that hold it** (the four page tests above). It asks one
question and names the default if no answer comes: the flag stays off and the desktop is the only
reader.

#### Steps

- [ ] **Step 1: Check the base.** Task 74's `views`, `drafts`, `views_one_team_default` and
  `draft_author_only` as merged; whether `hub.user_id` is set by middleware or by task 07's store;
  the Hub contract's line 38 as it really reads on `feat/hub`; task 08's `s.route` recorder.
- [ ] **Step 2: The vectors and `Where`.** Write the 12 JSON files, then `store/work_filter_test.go`,
  then `Where`. Nothing else starts until the twelve are green: two evaluators depend on them.
- [ ] **Step 3: Views**, tests then store then routes, including the default swap in one event.
- [ ] **Step 4: Drafts**, tests then store then routes; assert the 404 four times.
- [ ] **Step 5: Export**, `csvCell` first as a unit test, then the two streaming routes.
- [ ] **Step 6: The proposal, then the page.** Write the document before the route, so the
  question is on the tree even if the page is cut. Then the mux, the template, the session and
  the four tests, behind the flag.
- [ ] **Step 7: Green**, then the matrix, then commit by pathspec — the proposal in its own commit,
  so the owner can read it without the code.

#### Acceptance

```bash
cd control-plane
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./internal/hub/store/ -tags integration -race -count=1 -run '^TestEachFilterVector$' -v 2>&1 | grep -c '^    --- PASS'   # 12
go test ./internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(AFilterIsNeverInterpolated|OnlyATeamAdminSetsTheDefault|AnArchivedDefaultIsNotTheDefault|AnothersDraftIsFourOhFour|ADraftWritesNoEvent|ACsvCellIsDefused|TheCsvHoldsOnlyReadableRows|AnErasedIssueCarriesNoText|ThePageSendsItsCsp|ThePageCookieIsNotAnApiToken|ThePageIsOffByDefault|ThePageNeedsTheProjectRead|EachWriteHasItsEvents)$' \
  2>&1 | grep -c '^--- PASS'                                                       # 13
go test ./internal/hub/... -tags integration -race -count=1                        # every package ok
grep -c "script" internal/hub/server/work_pages_template.html                      # 0
grep -rn "FilterGroup" internal/hub --include=*.go | grep -v _test.go | grep -v hubapi/ | grep -vc "store/work_filter.go"   # 0
ls internal/hub/hubapi/testdata/view-filter/*.json | wc -l                         # 12
```

Expected: `12`, `13`, every package `ok`, `0`, `0`, `12`. None has been run; no file exists yet
(OD-27). The two `0`s are the point: no script reaches the page, and exactly one Go file reads a
filter.

#### What this task does not claim

- **Not that the page may exist.** The Hub contract forbids browsers on `/v1`, and whether `/p/`
  is inside that rule is the **owner's** call. The flag is off, the proposal is written, and the
  page merges only after the answer (decision (i)).
- **Not that an https link opens the installed desktop on Windows** (**UNVERIFIED**). The
  `panacea://` handler is task 93's, and no one has tested a protocol handler under the corporate
  policy. The page's button is the path that works without one.
- **Not intake CSV** (W5, L-153's request half), **not import** (nothing here reads a CSV),
  **not subscriptions to a view** (the digest, task 81).
- **Not personal views** (L-94): they stay desktop preferences, and nothing here stores one.
- **Not a measured export.** The reads stream, which is a design choice, not a measurement; the
  largest project anyone has exported is zero issues.
- **Not the desktop.** The view editor, the draft restore and the export button are tasks 91-94.

#### Corrections to the scope

1. **This task does not fit three hours** — the scope says so itself, and this part sizes it at
   **4 to 5**. Splitting it is cleaner: `80a` views, drafts and CSV; `80b` the page and its
   proposal. **Reported as a skeleton gap**, with the split as the recommendation.
2. **The filter tests are integration, not unit.** `Where` builds SQL; proving two evaluators
   agree needs the database to run the SQL. The scope implies a pure function.
3. **The matrix file is task 07's** `work_matrix_rows_integration_test.go`, not
   `server/matrix_integration_test.go` (task 75's correction).
4. **A third matrix shape, `wmine`**, because a draft's refusal is `404` for five principals and
   the two existing shapes both assume `403`.
5. **The page needs two config keys, a template file and a flag**, none of which the scope named.
6. **The default swap is one event, not two** (`view.default_changed` with `fromViewId` and
   `toViewId`), because one act changed two rows.
7. **A UTF-8 BOM leads every CSV**, or Excel misreads non-ASCII titles. Not in the scope.

**Parity rows served:** the Hub halves of **L-93**, **L-94** (by storing nothing), **L-95**,
**L-99**, **L-105**, **L-33**, **L-44**, **L-98**'s W2 half and **L-153**'s issues, views and
features. Tasks 91-94 close them with the desktop; task 97 records them.
