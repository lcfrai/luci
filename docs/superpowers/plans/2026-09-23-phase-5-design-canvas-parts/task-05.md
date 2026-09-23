# Task 05 — The document model, ids, the design key, fractional order

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 02 · **Hours:** 2

## Why

Every other Go task reads or writes these types. They are the spec's §7.3 entities made exact:
Paper's model (a file of pages of artboards of HTML nodes with real CSS, spec §2.1), plus what we add
for parallel agents (a revision and an author on every node) and for provenance (the design key).
Child order is a **fractional index** (spec §4, [S26]) so a move or an insert touches one node, and
so v2's multiplayer can order concurrent inserts without renumbering. The algorithm is written here,
from its description, because the published package is CC0 and outside the gate (spec L31, OQ-8).

## Files

- `canvas/doc/model.go`, `ids.go`, `key.go`, `order.go`, `validate.go` (new)
- `canvas/doc/model_test.go`, `ids_test.go`, `key_test.go`, `order_test.go` (new)
- `canvas/boundary_test.go` (new) — the module boundary of task 02.

## Contracts

```go
// canvas/doc/model.go
package doc

type Kind string

const (
	KindArtboard Kind = "artboard" // a top-level frame with a size; its parent is a page
	KindFrame    Kind = "frame"    // a container
	KindText     Kind = "text"     // plain text (plan P5-E); "\n" allowed
	KindImage    Kind = "image"    // an img whose bytes are an asset
	KindSVG      Kind = "svg"      // one node holding sanitised markup (plan P5-F)
)

// AllowedTags is the element each kind may export as. The first entry is the
// default. `input` is a void frame: it may have no children.
var AllowedTags = map[Kind][]string{
	KindArtboard: {"div"},
	KindFrame:    {"div", "section", "header", "footer", "nav", "main", "aside", "article", "ul", "ol", "li", "button", "a", "label", "input"},
	KindText:     {"p", "span", "h1", "h2", "h3", "h4", "h5", "h6", "label", "button", "a", "li", "small", "strong", "em", "b", "i", "code", "blockquote", "figcaption"},
	KindImage:    {"img"},
	KindSVG:      {"svg"},
}

// AllowedAttrs is every HTML attribute a node keeps, by tag; "*" is every tag.
// Nothing here is ever loaded or executed: `a` exports href="#" (no href is kept).
var AllowedAttrs = map[string][]string{
	"*":     {"aria-label"},
	"input": {"type", "placeholder", "value", "checked"},
}

// AllowedInputTypes bounds input's "type".
var AllowedInputTypes = []string{"text", "email", "password", "search", "number", "tel", "url", "checkbox", "radio"}

type AuthorKind string

const (
	AuthorHuman   AuthorKind = "human"
	AuthorSession AuthorKind = "session"
)

// Author is stamped by the authority from the caller, never taken from a
// request body (plan rule 4).
type Author struct {
	Kind      AuthorKind `json:"kind"`
	UserID    string     `json:"userId"`    // "local" in v1; the Hub principal's id in v2 (S5-H.1)
	SessionID string     `json:"sessionId"` // "" for a human
	Handle    string     `json:"handle"`    // the session's title, or the person's display name
}

type Node struct {
	ID       string            `json:"id"`       // n-<12 hex>
	Kind     Kind              `json:"kind"`
	Tag      string            `json:"tag"`      // one of AllowedTags[Kind]
	Name     string            `json:"name"`     // the layer name; ≤ 120 runes; "" = derived in the UI
	ParentID string            `json:"parentId"` // a page id for an artboard, a node id otherwise
	Order    string            `json:"order"`    // fractional index among siblings (order.go)
	Styles   map[string]string `json:"styles"`   // allowlisted property → value (task 07); {} not null
	Text     string            `json:"text"`     // KindText only; ≤ 10,000 runes
	Asset    string            `json:"asset"`    // KindImage only: an asset ref
	Alt      string            `json:"alt"`      // KindImage only; ≤ 500 runes
	SVG      string            `json:"svg"`      // KindSVG only: canonical sanitised markup; ≤ 256 KiB
	Attrs    map[string]string `json:"attrs"`    // AllowedAttrs only; {} not null
	Locked   bool              `json:"locked"`
	Hidden   bool              `json:"hidden"`
	Revision int               `json:"revision"` // the last canvas revision that changed THIS node's fields (P5-K)
	EditedBy Author            `json:"editedBy"`
}

type Page struct {
	ID    string `json:"id"`    // pg-<12 hex>
	Name  string `json:"name"`  // ≤ 120 runes
	Order string `json:"order"` // fractional index among pages
}

type TokenType string

// The ten token types of Paper's public docs (spec §2.4, [S3]).
const (
	TokenColor         TokenType = "color"
	TokenRadius        TokenType = "radius"
	TokenSpacing       TokenType = "spacing"
	TokenContainer     TokenType = "container"
	TokenBreakpoint    TokenType = "breakpoint"
	TokenFontFamily    TokenType = "font-family"
	TokenFontWeight    TokenType = "font-weight"
	TokenFontSize      TokenType = "font-size"
	TokenLineHeight    TokenType = "line-height"
	TokenLetterSpacing TokenType = "letter-spacing"
)

// Token is a CSS custom property. Name has no leading "--".
type Token struct {
	Name  string    `json:"name"`  // ^[a-z][a-z0-9-]{0,62}$
	Type  TokenType `json:"type"`
	Value string    `json:"value"` // checked for Type by dialect.CheckTokenValue (task 07)
}

// Asset is content-addressed image bytes. The bytes live in the store (task 10).
type Asset struct {
	Ref       string `json:"ref"`       // AssetRef(sha256) — "asset:sha256-<64 hex>"
	MediaType string `json:"mediaType"` // image/png | image/jpeg | image/webp | image/svg+xml (the Hub's blob types, contract §7.6)
	Size      int64  `json:"size"`      // ≤ 10 MiB (spec §8.4)
	Width     int    `json:"width"`     // 0 when unknown
	Height    int    `json:"height"`
}

type ThreadStatus string

const (
	ThreadOpen     ThreadStatus = "open"
	ThreadResolved ThreadStatus = "resolved"
)

type Message struct {
	ID     string `json:"id"`   // msg-<12 hex>
	Text   string `json:"text"` // 1–4,000 runes
	At     string `json:"at"`   // RFC 3339 UTC, milliseconds
	Author Author `json:"author"`
}

type Thread struct {
	ID        string       `json:"id"`     // th-<12 hex>
	NodeID    string       `json:"nodeId"` // the anchor; kept when the node is deleted (shown detached)
	Status    ThreadStatus `json:"status"`
	Messages  []Message    `json:"messages"`
	CreatedAt string       `json:"createdAt"`
	CreatedBy Author       `json:"createdBy"`
}

type ReviewState string

const (
	ReviewOpen   ReviewState = "open"
	ReviewClosed ReviewState = "closed" // closed by an approval or by the human dismissing it
)

// ReviewRequest is an agent's request_review (plan P5-S). It never approves.
type ReviewRequest struct {
	ID          string      `json:"id"` // rv-<12 hex>
	ArtboardIDs []string    `json:"artboardIds"`
	Note        string      `json:"note"` // ≤ 2,000 runes
	Revision    int         `json:"revision"`
	At          string      `json:"at"`
	Author      Author      `json:"author"`
	State       ReviewState `json:"state"`
}

// Document is a canvas as JSON: the store's snapshot, the REST read, and the
// input to every serialiser. Every slice is sorted as commented, so two equal
// documents marshal to equal bytes.
type Document struct {
	V         int             `json:"v"`         // 1
	CanvasID  string          `json:"canvasId"`  // cv-<16 hex>
	ProjectID string          `json:"projectId"` // the LOCAL project id, p-… (v1)
	BranchID  string          `json:"branchId"`  // the LOCAL branch id, b-…
	Revision  int             `json:"revision"`  // 0 for a new canvas
	CreatedAt string          `json:"createdAt"`
	Pages     []Page          `json:"pages"`   // by Order, then ID
	Nodes     []Node          `json:"nodes"`   // by ID
	Tokens    []Token         `json:"tokens"`  // by Name
	Assets    []Asset         `json:"assets"`  // by Ref
	Threads   []Thread        `json:"threads"` // by ID
	Reviews   []ReviewRequest `json:"reviews"` // by ID
}

// Doc is the in-memory form the authority mutates. It is not safe for
// concurrent use; the authority serialises access per canvas (task 11).
type Doc struct{ /* unexported: the Document's fields as maps, plus a child index */ }

func NewDoc(canvasID, projectID, branchID, createdAt string) *Doc
func FromDocument(d Document) (*Doc, error) // validates every invariant of validate.go
func (d *Doc) Document() Document             // sorted as Document requires
func (d *Doc) Revision() int
func (d *Doc) Node(id string) (Node, bool)
func (d *Doc) Page(id string) (Page, bool)
func (d *Doc) Children(parentID string) []Node // by Order, then ID
func (d *Doc) Pages() []Page                   // by Order, then ID
func (d *Doc) Artboards(pageID string) []Node  // Children(pageID)
func (d *Doc) ArtboardOf(nodeID string) (string, bool) // the artboard ancestor, or the node itself
func (d *Doc) Subtree(id string) []Node        // pre-order, children by Order
func (d *Doc) Token(name string) (Token, bool)
func (d *Doc) Tokens() []Token
func (d *Doc) Asset(ref string) (Asset, bool)
func (d *Doc) Thread(id string) (Thread, bool)
func (d *Doc) Threads() []Thread
func (d *Doc) Reviews() []ReviewRequest
```

```go
// canvas/doc/ids.go

// NewCanvasID is "cv-" + 16 lowercase hex from crypto/rand (Hub contract §3.2:
// phase 5 mints canvas ids wide so they stay unique when v2 moves them).
func NewCanvasID() string

// NewID is prefix + "-" + 12 lowercase hex from crypto/rand. Prefixes in use:
// "pg" page, "n" node, "th" thread, "msg" message, "rv" review request.
func NewID(prefix string) string

func ValidCanvasID(s string) bool // ^cv-[0-9a-f]{16}$
func ValidID(prefix, s string) bool // ^<prefix>-[0-9a-f]{12}$

// AssetRef is "asset:sha256-" + lowercase hex of sum.
func AssetRef(sum [32]byte) string
func ParseAssetRef(ref string) (sum [32]byte, ok bool)
```

```go
// canvas/doc/key.go — the only minting and parsing of a design key (plan G9)

var ErrBadKey = errors.New("doc: not a design key")

// DesignKey is "design:<canvasID>@<revision>". It errs unless ValidCanvasID
// and 1 ≤ revision ≤ 999,999,999 (the Hub's grammar, contract §3.2).
func DesignKey(canvasID string, revision int) (string, error)

// ParseDesignKey inverts DesignKey exactly; anything else is ErrBadKey.
func ParseDesignKey(key string) (canvasID string, revision int, err error)
```

```go
// canvas/doc/order.go

// The order alphabet, in ASCII order: 0-9, A-Z, a-z. A key is a base-62
// fraction 0.d1d2…dn with dn ≠ '0', so byte order equals numeric order and
// no two keys are equal in value.
const OrderAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var ErrOrder = errors.New("doc: invalid order keys")

// Between returns a key strictly between a and b. a == "" is the bottom
// (0), b == "" the top (1). It errs when either key is invalid or a >= b.
//
// Algorithm, digit by digit from the left, with the missing digits of a read
// as 0, of an empty b as 62, and of a non-empty b as 0:
//   - equal digits are copied and the walk continues;
//   - if b's digit exceeds a's by more than 1, emit floor((da+db)/2) and stop;
//   - otherwise emit a's digit, drop the upper bound (b = ""), and continue
//     with the rest of a.
func Between(a, b string) (string, error)

// Spread returns n keys strictly between a and b, ascending: the middle key
// is Between(a, b), and each half is spread recursively.
func Spread(a, b string, n int) ([]string, error)

func ValidOrder(k string) bool // non-empty, alphabet only, last byte not '0'
```

```go
// canvas/doc/validate.go

// Limits (spec §8.4, plan tasks 05–07).
const (
	MaxNameRunes    = 120
	MaxTextRunes    = 10_000
	MaxAltRunes     = 500
	MaxSVGBytes     = 256 << 10
	MaxAssetBytes   = 10 << 20
	MaxNodes        = 50_000 // per canvas
	MaxDepth        = 64     // artboard = depth 1
	MaxMessageRunes = 4_000
	MaxNoteRunes    = 2_000
)

// Validate checks one node alone: id, kind, tag, attrs, the field that
// belongs to its kind and no other, the limits. Style properties are
// checked by the dialect (task 07), not here.
func (n Node) Validate() error
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestBetween_vectors` | `("","")→"V"`, `("V","")→"k"`, `("","V")→"F"`, `("V","W")→"VV"`, `("V","V1")→"V0V"`, `("z","")→"zV"`, `("","1")→"0V"` | an off-by-one midpoint; `db` of a non-empty `b` read as 62 |
| `TestBetween_errors` | `("V","V")`, `("W","V")`, `("V0","")`, `("V!","")` all `ErrOrder` | equal or inverted bounds accepted |
| `TestBetween_property` | 10,000 random valid pairs: `a < k < b` bytewise and `ValidOrder(k)` | a key ending in `'0'` |
| `TestSpread` | `Spread("","",7)` is 7 ascending valid keys; `Spread("V","W",0)` is empty | a spread that repeats a key |
| `TestDesignKey_roundTrip` | `DesignKey("cv-00000000000000a1", 11)` is `design:cv-00000000000000a1@11` and parses back; revision `0`, `1000000000`, `cv-short` and `design:cv-…@011` are errors | a parser that accepts leading zeros (the Hub's grammar does not) |
| `TestIDs` | 1,000 `NewCanvasID()` match `^cv-[0-9a-f]{16}$` and are distinct; `NewID("n")` matches `^n-[0-9a-f]{12}$` | an 8-hex canvas id (the control plane's `ids.New` width) |
| `TestAssetRef` | `AssetRef` of SHA-256("") is `asset:sha256-e3b0c442…b855` and `ParseAssetRef` inverts it | uppercase hex |
| `TestNodeValidate` (table) | a text node with `SVG` set, an image with no `Asset`, a `frame` tagged `p`, an `input` with a `href` attr, a 121-rune name — each an error; a valid node of each kind passes | kind/field confusion |
| `TestDocument_sortedAndStable` | a `Doc` built in random insertion order marshals to identical bytes twice and to the same bytes as one built in sorted order | map iteration leaking into JSON |
| `TestFromDocument_rejects` | a parent cycle, an artboard whose parent is a node, a child of a text node, a child of an `input`, depth 65, two siblings with one `Order` — each an error | a store that loads a corrupt canvas |
| `TestBoundary` (`canvas/boundary_test.go`) | `go list -deps -f {{.ImportPath}} ./...` run in `canvas/` names no path under `…/panacea-app/control-plane` or `…/sandboxd` | the canvas importing the cockpit (`OD-11` rule 4) |

## Steps

1. Write the tests. 2. Implement `ids.go`, `key.go`, `order.go`, then `model.go` and `validate.go`.
3. Run the acceptance commands; commit the nine paths by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/... -race -count=1 && go vet ./canvas/...
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas` and `ok  	…/canvas/doc`; vet silent.

## Not claimed

- No mutation exists yet: `Doc` has readers only. Ops are task 06.
- Style values are not validated here (task 07).
- The order algorithm is written from the description in this part, not from any package.
