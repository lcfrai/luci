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
5. **The head carries its title, additively.** Task 03's `IssueHead` has no title
   (`task-03.md:282-303`), so a search hit, like every list row, would be a number with no name.
   With `head_title` stored, every head read fills `IssueHead.Title` (`""` once erased). Because
   the head now holds issue text, **every stored replay that holds a head blanks `head.title`** as
   it already blanks revision content: task 08's create (`task-08.md:378`), task 09's revision
   writes and task 11's relation reply. Task 14's canary test is the check that none was missed.
4. **Search is `ILIKE` on `head_title`, with no index.** At 10,000 issues its speed is
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

