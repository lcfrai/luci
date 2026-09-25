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
column this task writes (`task-06.md:291-337`, read at this writing): the body's 16 KiB `CHECK`,
the `(org_id, issue_id, parent_id)` foreign key that keeps a reply on its parent's issue, the
`author_class IN ('H','A')` check, and the reaction key.

**Interfaces:**

- Produces:
  - the routes, in 07's list and 07's matrix rows: `GET` and `POST /v1/issues/{issueId}/comments`,
    `POST /v1/comments/{commentId}/reactions`, `POST /v1/comments/{commentId}/resolve`. **No
    `PATCH` and no `DELETE` on `/v1/comments/…`**, and a test reads `server.Patterns()` to hold it
  - `hubapi.ValidReactionEmoji(s) bool`, `hubapi.UTF16Len(s) int`, `hubapi.MaxMentions` (20),
    `hubapi.MaxAnchorRefBytes` (512), `hubapi.AnchorText`, `AnchorCriterion`, `AnchorSpan`,
    `AnchorHunk`
  - the events `comment.created`, `comment.mentioned`, `comment.reacted`, `comment.resolved`, as
    drafts that task 13 makes constants (SW.5)
  - `store.MentionedUsers(commentID)`, which task 13's subscription trigger reads in the same
    transaction
- Consumes:
  - task 03: `Comment`, `CommentAnchor`, `Reaction`, `AuthoredText`, `CreateCommentRequest`,
    `ReactRequest`, `ResolveRequest`, `Page[T]`
  - task 06: `issue_comments`, `comment_mentions`, `comment_reactions`
  - task 07: the matrix rows (comment writes 403 for `V`, ✓ for `M A`, 404 for `O N X`; reads ✓ for
    `V M A`)
  - task 08: the readable-issue lookup; the team-membership read used for `mentions`
  - task 09: the read of one stored revision's content and its `serves`
  - Hub task 14: `server.Write`, `server.Read`, `(*Tx).Emit`, `server.Refuse`, `server.Decode`
- Consumed by: task 13 (subscribe the commenter and each mentioned user in the same
  transaction), task 14 (erase blanks `body` and `anchor`), task 51 (the comment UI), task 60 (a
  hunk or span anchor from the Merge lens), task 99 (the W3 kinds)

#### Wire shapes

```jsonc
// POST /v1/issues/is-0a1b…/comments                 member; 201
{ "body": { "text": "The resume test still fails on Windows.", "author": "H" },
  "parentId": null,
  "anchor": { "kind": "criterion", "revision": 3, "criterionId": "c-resume" },
  "mentions": ["u-7c1d…"] }
// → 201 Comment
{ "id": "ic-4e2f…", "issueId": "is-0a1b…", "parentId": null, "kind": "comment",
  "body": { "text": "The resume test still fails on Windows.", "author": "H" },
  "anchor": { "kind": "criterion", "revision": 3, "start": 0, "end": 0, "criterionId": "c-resume", "ref": "" },
  "mentions": ["u-7c1d…"], "reactions": [], "resolvedAt": null, "resolvedBy": null,
  "createdAt": "2026-09-25T04:02:11.412Z", "createdBy": { "id": "u-…", "displayName": "Grace", "email": "…" } }

// The four anchor kinds, as sent (a field of another kind is 400 naming it)
{ "kind": "text",      "revision": 3, "start": 120, "end": 164 }
{ "kind": "criterion", "revision": 3, "criterionId": "c-resume" }
{ "kind": "span",      "ref": "sp-9a8b…" }                       // a transcript span id, verbatim
{ "kind": "hunk",      "ref": "3f2a91c src/session/resume.ts:40-58" } // commit path:lines, verbatim

// GET /v1/issues/is-0a1b…/comments?after=&limit=    viewer; Page[Comment], oldest first,
// top-level and replies in one list (the desktop threads them by parentId)

// POST /v1/comments/ic-4e2f…/reactions              member; 200 Comment
{ "emoji": "👍", "on": true }
// POST /v1/comments/ic-4e2f…/resolve                member; 200 Comment
{ "resolved": true }
```

`anchor` is `null` when the comment has none. `start`, `end`, `criterionId` and `ref` encode as
`0` and `""` when they do not apply (task 03's struct has no `omitempty` on them; the response
never omits a field, the rule `hubapi/types_test.go:257` `TestResponsesNeverOmitAField` holds for
the Hub's own types, verified at `9f2fa31`). `Reaction.users` is in
reaction order, and a `Comment.reactions` list is ordered by the first reaction's time, so two
reads of one comment answer the same bytes.

#### `hubapi/work.go`, the additions (complete)

```go
// ── Task 12: comments ──────────────────────────────────────────────────────
// Appended to work.go (task 03's file). Additive only.

const (
	AnchorText      = "text"      // revision, start, end (UTF-16 code units of the description)
	AnchorCriterion = "criterion" // revision, criterionId (a criterion that revision serves)
	AnchorSpan      = "span"      // ref: a transcript span id, stored verbatim
	AnchorHunk      = "hunk"      // ref: "<commit> <path>:<lines>", stored verbatim

	MaxCommentBodyBytes = 16 << 10 // task 06's CHECK (octet_length(body) <= 16384)
	MaxMentions         = 20
	MaxAnchorRefBytes   = 512
	MaxReactionBytes    = 32
)

// UTF16Len is the length of s in UTF-16 code units: the unit a browser selection
// reports, so a text anchor's start and end are compared in the editor's own unit.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// ValidReactionEmoji accepts one emoji sequence of at most 32 bytes: a base
// emoji, optionally followed by variation selector 16, a skin-tone modifier,
// tag characters or a keycap mark, and further bases only after a zero-width
// joiner; or a flag of exactly two regional indicators; or a keycap
// (0-9, #, *, then U+FE0F U+20E3). It refuses text, a :shortcode: (custom
// emoji is *later*, L-41) and two emoji side by side. It is not a full UTS #51
// parser: a rare malformed ZWJ sequence can pass; nothing renders it as markup.
func ValidReactionEmoji(s string) bool {
	if s == "" || len(s) > MaxReactionBytes || !utf8.ValidString(s) || strings.ContainsRune(s, ':') {
		return false
	}
	rs := []rune(s)
	if isKeycapBase(rs[0]) {
		return len(rs) == 3 && rs[1] == 0xFE0F && rs[2] == 0x20E3
	}
	if isRegionalIndicator(rs[0]) {
		return len(rs) == 2 && isRegionalIndicator(rs[1])
	}
	if !isEmojiBase(rs[0]) {
		return false
	}
	afterJoiner := false
	for _, r := range rs[1:] {
		switch {
		case r == 0x200D:
			if afterJoiner {
				return false
			}
			afterJoiner = true
		case r == 0xFE0F, r >= 0x1F3FB && r <= 0x1F3FF, r >= 0xE0020 && r <= 0xE007F:
			if afterJoiner {
				return false
			}
		case isEmojiBase(r) && afterJoiner:
			afterJoiner = false
		default:
			return false
		}
	}
	return !afterJoiner
}
```

