### Task 12: comments `ic-`: mentions, reactions, anchors, resolve

W1, stream WH; no seam of its own (it fills SW.1's `Comment`; W3's comment kinds are SW.19, task
99). Depends on task 08 (issue heads and the readable-issue lookup) and reads task 09's revisions
(an anchor names one). Hub anchors were read in the `feat/hub` worktree at `9f2fa31` (the contract,
cited `hub-contract:N`; Hub task 14's part, cited `H14:N`). `server/` and `store/` are **planned,
not on `feat/hub`** at `9f2fa31`, so their paths are **UNVERIFIED until Hub tasks 08-14 land**.

A comment is how teammates talk about an issue, and how an agent's draft reaches the record when a
person posts it. This task stores comments as **append-only** Hub records: a comment is never
edited and never deleted by its author. A correction is a new comment; removal is task 14's erase,
which is an org admin's, audited, and keeps the comment's id so every reference still resolves as
`erased` (05 §3.2). That is the whole design: a record of who said what, next to the issue on the
thread, that nobody can quietly rewrite.

Four decisions the scope left open, made here.

1. **There is no `kind` field on `CreateCommentRequest` in W1.** Task 03's body is `{body,
   parentId, anchor, mentions}`, decoded strictly, so a `"kind": "note"` is 400 `bad_request`
   (`field: "kind"`) by the decoder, with no special case. Every W1 comment stores `kind =
   'comment'` (task 06's `issue_comments_kind_known`, `task-06.md` at the `issue_comments` table).
   Task 99 adds the field and widens the constraint.
2. **An anchor names exactly one thing, and carries no field of another kind.** `text` carries
   `revision`, `start`, `end`; `criterion` carries `revision`, `criterionId`; `span` and `hunk`
   carry `ref` only. A field that does not belong to the kind is 400 naming it
   (`anchor.criterionId` on a `text` anchor). A loose anchor is where "points at nothing" starts.
3. **Anchors are checked against the revision they name, not the head.** A comment on the text of
   revision 3 stays anchored to revision 3 when revision 4 is written; the desktop shows it as *on
   an earlier revision* (task 51). So the check reads that revision's stored content through task
   09's store read, and the offsets are **UTF-16 code units**, the unit a browser selection reports
   (task 49 fixes this; **UNVERIFIED** until task 49's part says so).
4. **Reactions are a set, not a counter.** One row per `(comment, user, emoji)` (task 06's primary
   key); `on: true` twice is one row and one event; `on: false` on no row is 200 and no event.

**The one thing this task must not get wrong** is that no comment body ever reaches the change
feed or the audit `data`. Every event here carries ids, kinds and booleans; the body stays in
`issue_comments.body`, where task 14's erase can reach it. A body copied into `data` could never
be erased, because the audit chain is hashed (hub-contract §7.12, `:1111-1113`).

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, additive): `ValidReactionEmoji`,
  `MaxMentions`, `MaxAnchorRefBytes`, the anchor-kind constants, `UTF16Len`, and the codes below if
  task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_comments_test.go` (new): the pure rules
- `control-plane/internal/hub/store/work_comments.go` (new)
- `control-plane/internal/hub/server/work_comments.go` (new)
- `control-plane/internal/hub/server/work_comments_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ (task 05's file): only where a rule here
  differs from what 05 wrote; the conformance rows are the arbiter

No migration: task 06's `issue_comments`, `comment_mentions` and `comment_reactions` carry every
column this task writes (`task-06.md`, the comments block, read at this writing).

