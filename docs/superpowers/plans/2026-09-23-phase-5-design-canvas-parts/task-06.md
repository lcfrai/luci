# Task 06 — Ops: the only way a document changes

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 05 · **Hours:** 3

## Why

Rule 1 of the architecture: every mutation — an agent's MCP call, a human's drag, a paste, an
undo — is an op batch, and a batch is **atomic**: every op applies, or none does and the error names
the first that failed. One accepted batch is one canvas revision (P5-K). Each applied batch also
yields its **inverse**, computed from the state it replaced, which is how per-author undo and
revert by session (task 12) work without a second history. The server applies ops in arrival order
and each `set_props` overwrites only the properties it names: per-property last-writer-wins, the
model spec §4 takes from Figma's public write-up [S26].

## Files

- `canvas/doc/op.go`, `apply.go`, `invert.go` (new); `canvas/doc/model.go` gains `Apply`
- `canvas/doc/op_test.go`, `apply_test.go`, `invert_test.go` (new)

## Contracts

```go
// canvas/doc/op.go
package doc

type OpKind string

const (
	OpCreatePage      OpKind = "create_page"       // Page
	OpUpdatePage      OpKind = "update_page"       // PageID; Name and/or Order
	OpDeletePage      OpKind = "delete_page"       // PageID; its artboards and their subtrees go too
	OpCreateNode      OpKind = "create_node"       // Node, complete (Revision and EditedBy are stamped)
	OpSetProps        OpKind = "set_props"         // NodeID; any of Styles, Attrs, Name, Tag, Text, Alt, Asset, SVG, Locked, Hidden, X, Y (X, Y: artboards only)
	OpMove            OpKind = "move"              // NodeID; ParentID and Order
	OpDeleteNode      OpKind = "delete_node"       // NodeID; the whole subtree
	OpSetToken        OpKind = "set_token"         // Token (create or replace)
	OpDeleteToken     OpKind = "delete_token"      // TokenName
	OpAddAsset        OpKind = "add_asset"         // AssetRecord; idempotent. The bytes are stored first (task 10)
	OpCreateThread    OpKind = "create_thread"     // Thread, with exactly one Message
	OpAddMessage      OpKind = "add_message"       // ThreadID, Message
	OpSetThreadStatus OpKind = "set_thread_status" // ThreadID, Status
	OpRequestReview   OpKind = "request_review"    // Review
	OpCloseReview     OpKind = "close_review"      // ReviewID
	OpReviewMark      OpKind = "review_mark"       // BriefKey. Changes no content; exists so a re-review has a new design revision to approve (brief D3, plan P5-M)
)

// Op is one mutation. Pointer fields distinguish "leave alone" (nil) from
// "set" (non-nil). In Styles and Attrs a nil value removes the key; JSON null.
type Op struct {
	Kind        OpKind             `json:"kind"`
	PageID      string             `json:"pageId,omitempty"`
	NodeID      string             `json:"nodeId,omitempty"`
	Page        *Page              `json:"page,omitempty"`
	Node        *Node              `json:"node,omitempty"`
	Styles      map[string]*string `json:"styles,omitempty"`
	Attrs       map[string]*string `json:"attrs,omitempty"`
	Name        *string            `json:"name,omitempty"`
	Tag         *string            `json:"tag,omitempty"`
	Text        *string            `json:"text,omitempty"`
	Alt         *string            `json:"alt,omitempty"`
	Asset       *string            `json:"asset,omitempty"`
	SVG         *string            `json:"svg,omitempty"`
	Locked      *bool              `json:"locked,omitempty"`
	Hidden      *bool              `json:"hidden,omitempty"`
	X           *int               `json:"x,omitempty"` // artboards only: the place on the page
	Y           *int               `json:"y,omitempty"`
	ParentID    *string            `json:"parentId,omitempty"`
	Order       *string            `json:"order,omitempty"`
	Token       *Token             `json:"token,omitempty"`
	TokenName   string             `json:"tokenName,omitempty"`
	AssetRecord *Asset             `json:"assetRecord,omitempty"`
	Thread      *Thread            `json:"thread,omitempty"`
	ThreadID    string             `json:"threadId,omitempty"`
	Message     *Message           `json:"message,omitempty"`
	Status      *ThreadStatus      `json:"status,omitempty"`
	Review      *ReviewRequest     `json:"review,omitempty"`
	ReviewID    string             `json:"reviewId,omitempty"`
	BriefKey    string             `json:"briefKey,omitempty"` // ≤ 64 bytes; never parsed here (plan G9)
}

// Checker validates what doc cannot without the dialect (task 07 implements
// it; doc must not import dialect).
type Checker interface {
	Style(property, value string) error         // one declaration, against the allowlist
	TokenValue(t TokenType, value string) error // a value valid for the type
	SVG(markup string) error                    // markup is already canonical sanitised SVG
}

// OpError is every refusal. Index is the op's position in the batch.
type OpError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`   // one of the Code* constants
	NodeID  string `json:"nodeId"` // or the page, thread, review or token the op named; "" if none
	Message string `json:"message"`
}

func (e *OpError) Error() string

const (
	CodeNotFound = "not-found" // the op names something that does not exist
	CodeExists   = "exists"    // an id, or a sibling order key, is already taken
	CodeInvalid  = "invalid"   // a field is malformed, missing, or wrong for the kind
	CodeCycle    = "cycle"     // a move would put a node inside its own subtree
	CodeLimit    = "limit"     // a MaxX of validate.go would be passed
	CodeStyle    = "style"     // Checker.Style refused a declaration
	CodeEmpty    = "empty"     // a batch with no ops
)

// Change is what one applied batch did. The authority turns it into an
// event (task 11), an activity row, an undo record (task 12) and a log line
// (task 10).
type Change struct {
	Revision       int      `json:"revision"`
	Author         Author   `json:"author"`
	At             string   `json:"at"`
	Label          string   `json:"label"`   // ≤ 120 runes, e.g. "write_html", "Move 3 layers"
	Ops            []Op     `json:"ops"`     // as applied
	Inverse        []Op     `json:"inverse"` // in application order; applying it restores the prior content
	Nodes          []string `json:"nodes"`   // created or modified node ids, sorted
	Deleted        []string `json:"deleted"` // deleted node ids, sorted
	Pages          []string `json:"pages"`   // created, modified or deleted page ids, sorted
	Artboards      []string `json:"artboards"` // every artboard any op touched (before or after), sorted
	Tokens         []string `json:"tokens"`  // set or deleted token names, sorted
	Assets         []string `json:"assets"`
	Threads        []string `json:"threads"`
	Reviews        []string `json:"reviews"`
	ContentChanged bool     `json:"contentChanged"` // false for a batch of only comment, review, asset or review_mark ops
}
```

```go
// canvas/doc/apply.go

// Apply applies ops atomically as one new revision (Revision()+1), stamping
// author on every node it creates or changes and setting that node's
// Revision. On error the Doc is exactly as before and the error is *OpError.
// label is copied into the Change. at is RFC 3339 UTC with milliseconds.
// Apply is Prepare followed by Commit.
func (d *Doc) Apply(ops []Op, author Author, at, label string, c Checker) (Change, error)

// Prepare validates ops and computes the Change and its inverse against a
// copy-on-write overlay of d, without changing d. The authority prepares,
// makes the change durable (task 10), and only then commits — so a failed
// disk write leaves memory and disk agreeing.
func (d *Doc) Prepare(ops []Op, author Author, at, label string, c Checker) (*Pending, error)

// Pending is a prepared, uncommitted batch.
type Pending struct {
	Change Change
	/* unexported: the overlay and the revision it was prepared against */
}

// Commit makes the prepared batch the Doc's state. It panics if the Doc has
// changed since Prepare: the authority holds the canvas lock across both.
func (p *Pending) Commit()
```

**Rules Apply enforces**, each a test:

| Op | Refused when |
|---|---|
| any | the batch is empty (`empty`); more than 5,000 ops (`limit`) |
| `create_page` | id invalid or taken; order invalid or equal to a sibling page's; name over 120 runes |
| `update_page` | page missing; new order collides |
| `delete_page` | page missing; it is the last page (`invalid`: *"a canvas keeps at least one page"*) |
| `create_node` | id invalid or taken; `Node.Validate()` fails; parent missing; an artboard's parent is not a page, or a non-artboard's parent is a page; the parent is `text`, `image`, `svg` or an `input`; order collides with a sibling; depth over 64; node count over 50,000; any style refused by `Checker.Style`; an image whose `Asset` is not a registered asset; an SVG refused by `Checker.SVG`; **an artboard without a `width` in `px`, or without a `height` in `px` or `fit-content`** |
| `set_props` | node missing; a field that does not belong to its kind (e.g. `Text` on a frame); a `Tag` not allowed for the kind; `Tag` set to `input` on a node with children; any style refused; an artboard left without its size |
| `move` | node missing; new parent missing or of a kind that cannot hold it; the new parent is in the node's subtree (`cycle`); order collides; depth over 64 |
| `delete_node` | node missing. Threads anchored in the subtree keep their `NodeID` (shown detached) |
| `set_token` | name not `^[a-z][a-z0-9-]{0,62}$`; unknown type; `Checker.TokenValue` refuses |
| `delete_token` | token missing |
| `add_asset` | ref malformed; media type not one of the four; size over 10 MiB. An existing ref is a no-op, not an error |
| `create_thread` | id invalid or taken; anchor node missing; not exactly one message; message over 4,000 runes |
| `add_message`, `set_thread_status` | thread missing; message id taken |
| `request_review` | id invalid or taken; an artboard id that is not an artboard; note over 2,000 runes |
| `close_review` | review missing or already closed |
| `review_mark` | `BriefKey` empty or over 64 bytes |

Ops within a batch see the effects of earlier ops in the same batch (a `create_node` may name a
parent created two ops earlier).

```go
// canvas/doc/invert.go

// Inverse ops are computed inside Apply, from the state each op replaced:
//   create_node → delete_node          delete_node → create_node × subtree, pre-order
//   set_props   → set_props of exactly the fields and keys it named, with the old values (nil where absent)
//   move        → move to the old parent and order
//   create_page → delete_page          delete_page → create_page + create_node × each artboard subtree
//   update_page → update_page (old)    set_token → set_token (old) or delete_token (new)
//   delete_token → set_token (old)
//   add_asset, threads, reviews, review_mark → no inverse (not content; never undone)
// Change.Inverse is the concatenation in REVERSE op order, so applying it
// with Apply restores the content of the prior revision.

// Invertible reports whether an op kind has an inverse (content ops only).
func Invertible(k OpKind) bool
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestApply_atomic` | a batch whose third of four ops fails leaves `Document()` byte-identical and `Revision()` unchanged; the error is `*OpError{Index: 2}` | a partially applied batch |
| `TestApply_oneRevisionPerBatch` | a 30-op batch moves the revision by exactly 1; every created node's `Revision` is the new revision and `EditedBy` is the author | a revision per op |
| `TestApply_perPropertyLWW` | `set_props{color:red}` then `set_props{padding:8px}` leaves both; a later `set_props{color:blue}` wins; `{color: null}` removes it | a `set_props` that replaces the whole map |
| `TestApply_nodeRevisionOnlyForOwnFields` | inserting a child does not change the parent's `Revision` | the parent bumped on child-list change (breaks P5-K's two-agents case) |
| `TestApply_seesEarlierOps` | `create_node` of a frame then of its child in one batch succeeds | ops validated against the pre-batch state |
| `TestApply_rules` (table, one row per cell of the table above) | each refusal with its code | any one rule missing |
| `TestApply_cycle` | moving `A` under its grandchild is `cycle` | a cycle that loops `ArtboardOf` for ever |
| `TestApply_artboardSize` | an artboard with `height: fit-content` is accepted; with `width: 50%` refused; `set_props{width: null}` on one refused | an artboard that loses its size |
| `TestApply_checkerCalled` | a strict fake `Checker` refusing `margin` turns `create_node` with `margin: 4px` into `style` at the right index | styles not checked on create |
| `TestInverse_restores` (property test, 2,000 random batches over a random document) | `Apply(ops)` then `Apply(change.Inverse)` gives a document whose content — every field except `Revision`, `EditedBy`, threads, reviews and assets — equals the original | an inverse that forgets a field or a subtree's order |
| `TestInverse_deleteSubtreeOrder` | deleting a frame with 12 ordered children and applying the inverse restores their `Order` keys exactly | inverse creating children in the wrong order |
| `TestChange_contentChanged` | a batch of only `create_thread` has `ContentChanged: false`; with one `set_props` added, `true` | comments counted as design changes (would wrongly trip *changed after approval*) |
| `TestChange_artboards` | a move from artboard A to artboard B lists both | claims checked only on the destination (task 12) |
| `TestPrepare_doesNotMutate` | after `Prepare` of a valid batch, `Document()` is unchanged until `Commit`; with two batches prepared against one revision, committing the second after the first panics | a Prepare that writes through (a failed disk append would leave memory ahead of disk) |
| `TestOp_jsonNullRemoves` | `{"kind":"set_props","nodeId":"n-…","styles":{"color":null}}` unmarshals to a nil pointer under `color` | `omitempty` swallowing the removal |

## Steps

1. Write the tests with two fake `Checker`s (permissive, and one that refuses `margin` and `grid`).
2. Implement `op.go`, `apply.go` (validate against a working copy, then commit to the `Doc` —
   copy-on-write of the touched maps, not a deep clone of the document), `invert.go`.
3. Acceptance; commit the six paths plus `model.go` by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/doc/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/doc`.

```bash
go test ./canvas/doc/ -run 'TestInverse_restores' -count=5
```
Expected: `ok` five times (the property test is seeded from the clock and logs its seed on failure).

## Not claimed

- No claims, preconditions or undo stacks: those are the authority's (task 12). `Apply` is pure
  state transition.
- No persistence (task 10) and no events (task 11).
- The style allowlist itself is task 07's; this task tests `Apply` against fakes.
