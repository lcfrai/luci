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

