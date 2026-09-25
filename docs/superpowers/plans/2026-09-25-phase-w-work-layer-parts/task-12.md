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

```go
func isKeycapBase(r rune) bool        { return (r >= '0' && r <= '9') || r == '#' || r == '*' }
func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isEmojiBase(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, r >= 0x2600 && r <= 0x27BF, r >= 0x2300 && r <= 0x23FF,
		r >= 0x2B00 && r <= 0x2BFF, r >= 0x2190 && r <= 0x21FF:
		return !isRegionalIndicator(r) && !(r >= 0x1F3FB && r <= 0x1F3FF)
	case r == 0x00A9, r == 0x00AE, r == 0x203C, r == 0x2049, r == 0x2122, r == 0x2139,
		r == 0x3030, r == 0x303D, r == 0x3297, r == 0x3299:
		return true
	}
	return false
}

// ValidateAnchor checks an anchor's shape only: the kind is known and carries
// only its own fields. Whether the revision, the offsets and the criterion exist
// is the store's check (they need the database). The error names the field.
func ValidateAnchor(a CommentAnchor) (field string, ok bool) {
	switch a.Kind {
	case AnchorText:
		switch {
		case a.Revision < 1:
			return "anchor.revision", false
		case a.Start < 0 || a.End <= a.Start:
			return "anchor.end", false
		case a.CriterionID != "":
			return "anchor.criterionId", false
		case a.Ref != "":
			return "anchor.ref", false
		}
	case AnchorCriterion:
		switch {
		case a.Revision < 1:
			return "anchor.revision", false
		case a.CriterionID == "":
			return "anchor.criterionId", false
		case a.Start != 0 || a.End != 0:
			return "anchor.start", false
		case a.Ref != "":
			return "anchor.ref", false
		}
	case AnchorSpan, AnchorHunk:
		switch {
		case a.Ref == "" || len(a.Ref) > MaxAnchorRefBytes || strings.ContainsAny(a.Ref, "\x00\r\n"):
			return "anchor.ref", false
		case a.Revision != 0:
			return "anchor.revision", false
		case a.Start != 0 || a.End != 0:
			return "anchor.start", false
		case a.CriterionID != "":
			return "anchor.criterionId", false
		}
	default:
		return "anchor.kind", false
	}
	return "", true
}

// Work codes task 12 needs. If task 03 declared them, these lines are dropped.
const (
	CodeCommentNotFound    = "comment_not_found"     // 404, details.commentId
	CodeCommentNotTopLevel = "comment_not_top_level" // 422, details.field: "parentId" | "commentId"
	CodeAnchorUnresolvable = "anchor_unresolvable"   // 422, details.field: "anchor.revision" | "anchor.end" | "anchor.criterionId"
)
```

`work.go` gains the imports `strings` and `unicode/utf8` if task 03 did not already use them.
Task 03's scope names a `:custom:` refusal on `Reaction`; if task 03 wrote a validator, this
function replaces its body (one validator, not two) and task 03's test must still pass. The field
names in `CommentAnchor` (`Kind`, `Revision`, `Start`, `End`, `CriterionID`, `Ref`) are task 03's
as its scope spells them; task 03's part wins on spelling.

**The shape errors are 400, the reference errors 422.** `ValidateAnchor` failing is 400
`bad_request` with its `field`: the request is malformed whatever the database holds. A
well-formed anchor that names a revision that does not exist, an `end` past that revision's
description, or a criterion that revision does not serve is 422 `anchor_unresolvable`: the
request is well formed and points at nothing. The split matches the Hub's own (`bad_request` for
a malformed key, `key_unresolvable` for a key that names nothing, hub-contract §4.2 at `:235` and
`:263`).

#### The store (`store/work_comments.go`, signatures complete)

```go
package store

// InsertComment writes the comment row and its mention rows. The caller has
// validated everything; the database's CHECKs and keys are the second layer.
func InsertComment(ctx context.Context, tx pgx.Tx, org string, c CommentRow) error

type CommentRow struct {
	ID, IssueID string
	ParentID    string // "" for a top-level comment
	AuthorClass string // "H" | "A"
	Body        string
	Anchor      []byte // the anchor as JSON, or nil
	Mentions    []string
	CreatedAt   time.Time
	CreatedBy   string
}

// CommentForWrite loads a comment inside the caller's readable issues, with
// its issue's team and project, locking the row (FOR UPDATE) for resolve.
func CommentForWrite(ctx context.Context, tx pgx.Tx, commentID string) (CommentRef, bool, error)

type CommentRef struct {
	ID, IssueID, TeamID, ProjectID string
	ParentID                       string // "" when top-level
	Resolved                       bool
	Erased                         bool
}

// ListComments is Page[Comment] oldest first, keyset on (created_at, id); each
// comment carries its mentions and reactions, aggregated in the same query.
func ListComments(ctx context.Context, q Querier, issueID string, pr hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error)

// SetReaction inserts or deletes one (comment, user, emoji) row; changed is
// false when the row was already in the asked state.
func SetReaction(ctx context.Context, tx pgx.Tx, org, commentID, userID, emoji string, on bool, at time.Time) (changed bool, err error)

// SetResolved sets or clears resolved_at and resolved_by; changed is false when
// the comment was already in the asked state.
func SetResolved(ctx context.Context, tx pgx.Tx, commentID string, resolved bool, by string, at time.Time) (changed bool, err error)

// MentionedUsers is the mention rows of one comment, for task 13's trigger.
func MentionedUsers(ctx context.Context, q Querier, commentID string) ([]string, error)
```

The read of one revision (its canonical bytes and its `serves`) is task 08's
`store.IssueRevisionAt(ctx, q, issueID, rev) (RevisionRow, bool, error)` (`task-08.md:275-276`,
read at this writing); the description is decoded from `RevisionRow.Canonical` with task 02's
content decoder, and an erased revision (`Canonical == nil`) answers 422 `anchor.revision`, because
there is no text left to anchor to.

#### The routes (`server/work_comments.go`)

```go
func (s *Server) registerWorkComments(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/issues/{issueId}/comments", s.listComments)
	mux.HandleFunc("POST /v1/issues/{issueId}/comments", s.postComment)
	mux.HandleFunc("POST /v1/comments/{commentId}/reactions", s.postReaction)
	mux.HandleFunc("POST /v1/comments/{commentId}/resolve", s.postResolve)
	// Deliberately nothing else under /v1/comments/: a comment is append-only
	// (05 §3.2). Task 14 adds POST /v1/comments/{commentId}/erase, an org admin's.
}
```

**`postComment`**, inside `s.Write`, in this order:

1. `server.Decode[hubapi.CreateCommentRequest]` (a `kind` is an unknown field: 400).
2. The issue inside the caller's readable set → 404 `issue_not_found`; role below `member` →
   `server.Forbidden("member")`. An erased issue (task 14) → 409 `conflict`: nothing new is
   written onto an erased record.
3. `body.author` is `H` or `A` (task 03's `AuthoredText` check) → else 400 `field:
   "body.author"`; `body.text` trimmed non-empty and at most `MaxCommentBodyBytes` → else 400
   `field: "body.text"`. `author: "A"` means *an agent drafted this and the person posted it*;
   the principal is the person, and `createdBy` records them. W1 has no other writer (W3 adds the
   Orchestrator path, task 99).
4. `parentId`, when set: a comment on **this** issue (else 422 `comment_not_top_level`, `field:
   "parentId"`, and not 404, because the caller can read it) that is itself top-level (the same
   422 for a reply to a reply: one level of thread, L-41).
5. `anchor`, when set: `ValidateAnchor` → 400 with its field; then against the database: the
   revision exists on this issue (else 422 `anchor_unresolvable`, `anchor.revision`); for `text`,
   `end <= UTF16Len(description.text)` of **that** revision (else `anchor.end`); for `criterion`,
   the revision's `serves` contains `criterionId` (else `anchor.criterionId`). `span` and `hunk`
   refs are stored verbatim and never parsed by the Hub (05 §3.2).
6. `mentions`: at most `MaxMentions`, each a `u-` id of a **current** member of the issue's team,
   duplicates removed in order → else 400 `bad_request`, `field: "mentions[n]"` (400, not 404:
   the id is a field of this request, and saying "not a member" leaks nothing the caller's team
   list does not already show).
7. `ids.Wide("ic")`; `InsertComment`; `Emit` one `comment.created` (subject the comment id) and
   one `comment.mentioned` per mention (subject the mentioned user), in that order.
8. 201 with the stored comment, read back through the same projection `listComments` uses.

**`postReaction`**: the comment in the readable set → 404 `comment_not_found`; member role; an
erased comment → 409 `conflict`; `ValidReactionEmoji` → else 400 `field: "emoji"`; `SetReaction`;
`changed` → one `comment.reacted` `{"emoji", "on"}`; 200 with the comment either way.

**`postResolve`**: as above to the comment; a reply → 422 `comment_not_top_level`, `field:
"commentId"`; `SetResolved`; `changed` → one `comment.resolved` `{"resolved"}`; 200 with the
comment. Any member may resolve or reopen a thread (05 §3.2 names no owner of a thread; the
answering reply is the reader's choice, L-41).

#### The events (drafts; task 13 makes them constants)

| Kind | `teamId` | `projectId` | `subject` | `data` |
|---|---|---|---|---|
| `comment.created` | ✓ | ✓ | comment id | `{"issueId", "parentId", "kind", "author", "anchorKind"}`; `parentId` and `anchorKind` are `""` when absent |
| `comment.mentioned` | ✓ | ✓ | the mentioned user's id | `{"commentId", "issueId"}` |
| `comment.reacted` | ✓ | ✓ | comment id | `{"emoji", "on"}` |
| `comment.resolved` | ✓ | ✓ | comment id | `{"resolved"}` |

One event per changed record (hub-contract:1133): a comment with two mentions changes three records
(the comment and two mention rows), so it appends three events. Every value is a string or a
boolean (hub-contract:1166). No row carries the body, the anchor's offsets or its `ref`: a `hunk`
ref names a file path, which is customer code structure, and the feed is metadata only.

#### Tests to write first, part 1 — `hubapi/work_comments_test.go` (pure)

```go
package hubapi

import "testing"

func TestReactionEmojiIsOneSequence(t *testing.T) {
	good := []string{"👍", "👍🏽", "❤️", "👩‍💻", "👨‍👩‍👧‍👦", "🇦🇺", "1️⃣", "🏳️‍🌈"}
	bad := []string{"", ":partyparrot:", ":+1:", "ok", "👍👍", "🇦", "🇦🇺🇳", "👩‍", "‍👩",
		"1", "1️", "👍 ", "<b>", string(make([]byte, 33))}
	for _, s := range good {
		if !ValidReactionEmoji(s) {
			t.Errorf("ValidReactionEmoji(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if ValidReactionEmoji(s) {
			t.Errorf("ValidReactionEmoji(%q) = true, want false", s)
		}
	}
}

func TestUTF16LenCountsSurrogatePairs(t *testing.T) {
	for s, want := range map[string]int{"": 0, "abc": 3, "é": 1, "👍": 2, "a👍b": 4, "👨‍👩‍👧": 8} {
		if got := UTF16Len(s); got != want {
			t.Errorf("UTF16Len(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestAnAnchorCarriesOnlyItsOwnFields(t *testing.T) {
	cases := []struct {
		a     CommentAnchor
		field string
	}{
		{CommentAnchor{Kind: AnchorText, Revision: 3, Start: 0, End: 4}, ""},
		{CommentAnchor{Kind: AnchorText, Revision: 3, Start: 4, End: 4}, "anchor.end"},
		{CommentAnchor{Kind: AnchorText, Revision: 0, Start: 0, End: 4}, "anchor.revision"},
		{CommentAnchor{Kind: AnchorText, Revision: 3, End: 4, CriterionID: "c-a"}, "anchor.criterionId"},
		{CommentAnchor{Kind: AnchorCriterion, Revision: 3, CriterionID: "c-a"}, ""},
		{CommentAnchor{Kind: AnchorCriterion, Revision: 3}, "anchor.criterionId"},
		{CommentAnchor{Kind: AnchorCriterion, Revision: 3, CriterionID: "c-a", End: 2}, "anchor.start"},
		{CommentAnchor{Kind: AnchorSpan, Ref: "sp-0123"}, ""},
		{CommentAnchor{Kind: AnchorHunk, Ref: "3f2a91c src/a.ts:1-9"}, ""},
		{CommentAnchor{Kind: AnchorHunk, Ref: "a\nb"}, "anchor.ref"},
		{CommentAnchor{Kind: AnchorSpan, Ref: "sp-0123", Revision: 2}, "anchor.revision"},
		{CommentAnchor{Kind: "line", Ref: "x"}, "anchor.kind"},
	}
	for _, c := range cases {
		field, ok := ValidateAnchor(c.a)
		if ok != (c.field == "") || field != c.field {
			t.Errorf("ValidateAnchor(%+v) = (%q, %v), want %q", c.a, field, ok, c.field)
		}
	}
}
```

| Test | Mutant it kills |
|---|---|
| `TestReactionEmojiIsOneSequence` | a `:shortcode:` accepted (custom emoji is *later*, L-41); two emoji as one reaction; plain text as a reaction; a lone regional indicator |
| `TestUTF16LenCountsSurrogatePairs` | a length in bytes or in runes, either of which puts a text anchor after an emoji on the wrong characters |
| `TestAnAnchorCarriesOnlyItsOwnFields` | a loose anchor (a text anchor that also names a criterion), an empty range, a `ref` with a newline that would break the `commit path:lines` form |

#### Tests to write first, part 2 — `server/work_comments_integration_test.go` (`//go:build integration`)

The cast and harness are task 11's (Hub task 15's `as(name)` over `devissuer` and
`hubtest.Migrated`): Ada admin of Portal, Grace member, Vera viewer, Linus outside Portal, Xavier in
another org. EP-1's revision 3 has the description `"Resume must survive a restart 👍 on Windows."`
(an emoji so the UTF-16 length, 44, differs from the rune count, 43; both computed, not
guessed) and serves `c-resume`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCommentsHaveNoEditOrDeleteRoute` | no pattern in `server.Patterns()` is `PATCH /v1/comments/…`, `PUT /v1/comments/…` or `DELETE /v1/comments/…`, nor any `PATCH`/`DELETE` under `/v1/issues/{issueId}/comments` | an editable comment |
| `TestACommentIsStoredAndListedOldestFirst` | Grace posts two comments, Ada one; `GET` lists them in write order with `kind: comment`, the body and author class as sent, `reactions: []`, `mentions: []` | a list in reverse order, or an `omitempty` list the desktop indexes |
| `TestAKindIsRefusedInW1` | a body with `"kind": "note"` → 400 `field: kind`; nothing written | a W3 kind accepted before task 99 widens the constraint |
| `TestAViewerCannotComment` | Vera `POST` → 403 `required: member`; Vera `GET` → 200; Linus `GET` → 404 `issue_not_found` | a viewer writing (07's row), or a non-member reading |
| `TestAReplyToAReplyIs422` | a reply to a top-level comment → 201 with `parentId`; a reply to that reply → 422 `comment_not_top_level`, `field: parentId` | threads deeper than one level (L-41) |
| `TestAParentOnAnotherIssueIs422` | `parentId` a comment of EP-2 on EP-1 → 422 `field: parentId`, not a 500 from the foreign key | a reply filed under the wrong issue |
| `TestATextAnchorIsCheckedInUTF16` | `start 0, end 44` on revision 3 → 201; `end 45` → 422 `anchor_unresolvable`, `field: anchor.end`; revision 9 → 422 `anchor.revision` | an unchecked anchor, or one measured in runes or bytes |
| `TestACriterionAnchorMustBeServed` | `criterionId: c-resume` on revision 3 → 201; `c-other` → 422 `anchor.criterionId` | an anchor on a criterion the revision does not serve |
| `TestSpanAndHunkRefsAreStoredVerbatim` | a hunk ref `3f2a91c src/session/resume.ts:40-58` reads back byte for byte; a 513-byte ref → 400 `anchor.ref` | a Hub that parses or rewrites a ref (05 §3.2) |
| `TestMentionsMustBeTeamMembers` | a mention of Linus → 400 `field: mentions[0]`; of Vera (a viewer is a member of the team) → 201; 21 mentions → 400 | a mention that names a stranger |
| `TestTwoMentionsAppendThreeEvents` | a comment mentioning Ada and Vera → exactly three `comment.*` events: `comment.created`, then `comment.mentioned` with subject Ada, then with subject Vera; a repeated id in `mentions` is one row. The count is of `comment.*` kinds only, because task 13 adds `subscription.set` events to the same write | one event for the batch, or a mention without its event |
| `TestReactingTwiceIsOneRowAndOneEvent` | `👍 on` twice → one row, one `comment.reacted`; `on: false` → the row gone, one more event; `on: false` again → 200, no event | a non-idempotent toggle |
| `TestAShortcodeReactionIs400` | `:partyparrot:` → 400 `field: emoji`; `👍👍` → 400 | custom emoji before it is designed |
| `TestResolvingAReplyIs422` | resolving the reply → 422 `field: commentId`; resolving the top-level → 200 with `resolvedBy` Grace; resolving again → 200, no event; `resolved: false` → `resolvedAt: null` | a resolve on a reply, or a resolve that forgets who |
| `TestAnAgentDraftKeepsItsClass` | Grace posts `author: A` → the comment reads `author: A`, `createdBy` Grace, and the event's `author` is `A` | an author class lost on write (L-124) |
| `TestNoCommentEventCarriesTheBody` | a body `CANARY-12-BODY` and a hunk ref `CANARY-12-PATH`; no event's `data` in the org, read as the owner from `hub.audit_events`, contains either string | the body or a code path in the feed (hub-contract:1133-1134) |

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1) and that tasks 08 and 09 are committed. Re-read
  hub-contract §4.2 and §8, Hub task 14's part, task 06's comment tables and task 09's revision
  read at the new base; follow any move.
- [ ] **Step 2:** Write `hubapi/work_comments_test.go`; it fails to compile. Append the `hubapi`
  block above; it passes.
- [ ] **Step 3:** Write `server/work_comments_integration_test.go`; every row fails (404
  `route_not_found`), and `TestCommentsHaveNoEditOrDeleteRoute` passes already, which is correct:
  it guards the absence.
- [ ] **Step 4:** Write `store/work_comments.go`, then `server/work_comments.go` and
  `registerWorkComments`; run until green.
- [ ] **Step 5:** Run task 05's comment conformance rows against the fake; fix the fake where it
  disagrees, never the row.
- [ ] **Step 6:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v -run '^Test(ReactionEmojiIsOneSequence|UTF16LenCounts|AnAnchorCarriesOnly)' 2>&1 | grep -c "^--- PASS"   # 3
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(CommentsHaveNoEdit|ACommentIsStored|AKindIsRefused|AViewerCannotComment|AReplyToAReply|AParentOnAnother|ATextAnchorIs|ACriterionAnchorMust|SpanAndHunkRefs|MentionsMustBe|TwoMentionsAppend|ReactingTwice|AShortcodeReaction|ResolvingAReply|AnAgentDraftKeeps|NoCommentEventCarries)' \
  2>&1 | grep -c "^--- PASS"                                                                    # 16
go test ./control-plane/... -race -count=1                                                      # ok
```

#### What this task does not claim

- The comment UI, threading on screen, or *on an earlier revision* (task 51).
- Subscriptions from a comment or a mention (task 13), or any notification (W2, task 81 onward).
- Erase of a comment (task 14); until then an erased issue refuses new comments, and nothing erases.
- W3's kinds `note`, `evidence_proposal`, `negotiation` (task 99), or the Orchestrator as a writer.
- Custom emoji (L-41 *later*), comment drafts (L-44, the desktop's), or AI thread summaries (L-43,
  W4).
- That `ValidReactionEmoji` is a full UTS #51 parser: it is not, and the part says which cases it
  refuses.

**Parity rows served:** **L-41** (threads one level deep, resolve, reactions; custom emoji stays
*later*; closed with task 51), **L-42** (anchors to text, a criterion, a transcript span or a
hunk; closed with tasks 51 and 60), **L-124** (the author class on every comment).

