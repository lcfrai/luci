package doc

import "fmt"

// OpKind is one kind of mutation. Every change to a canvas is a batch of ops:
// an agent's MCP call, a human's drag, a paste and an undo all arrive here.
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
	TokenValue(t TokenType, value string) error // the declaration refusals and more, then the type
	// SVG refuses markup the sanitiser would drop anything from, and returns the
	// canonical form for THIS node id (root xmlns, well-formed XML, ids prefixed
	// with nodeID). Apply stores the returned string, and writes it back into the
	// op it records in the Change, so the event stream, every desktop DocStore and
	// a replay apply the same bytes.
	SVG(nodeID, markup string) (canonical string, err error)
}

// Batch limits. MaxBatchOps bounds one atomic batch; MaxLabelRunes bounds the
// human label the cockpit shows in the activity feed and the undo menu.
const (
	MaxBatchOps      = 5_000
	MaxLabelRunes    = 120
	MaxBriefKeyBytes = 64
)

// OpError is every refusal. Index is the op's position in the batch.
type OpError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`   // one of the Code* constants
	NodeID  string `json:"nodeId"` // or the page, thread, review or token the op named; "" if none
	Message string `json:"message"`
}

func (e *OpError) Error() string {
	if e.NodeID == "" {
		return fmt.Sprintf("doc: op %d: %s: %s", e.Index, e.Code, e.Message)
	}
	return fmt.Sprintf("doc: op %d (%s): %s: %s", e.Index, e.NodeID, e.Code, e.Message)
}

const (
	CodeNotFound = "not-found" // the op names something that does not exist
	CodeExists   = "exists"    // an id, or a sibling order key, is already taken
	CodeInvalid  = "invalid"   // a field is malformed, missing, or wrong for the kind
	CodeCycle    = "cycle"     // a move would put a node inside its own subtree
	CodeLimit    = "limit"     // a MaxX of validate.go would be passed
	CodeStyle    = "style"     // Checker.Style refused a declaration
	CodeEmpty    = "empty"     // a batch with no ops
)

// Change is what one applied batch did. The authority turns it into an event
// (task 11), an activity row, an undo record (task 12) and a log line (task 10).
type Change struct {
	Revision       int      `json:"revision"`
	Author         Author   `json:"author"`
	At             string   `json:"at"`
	Label          string   `json:"label"`     // ≤ 120 runes, e.g. "write_html", "Move 3 layers"
	Ops            []Op     `json:"ops"`       // as applied
	Inverse        []Op     `json:"inverse"`   // in application order; applying it restores the prior content
	Nodes          []string `json:"nodes"`     // created or modified node ids, sorted
	Deleted        []string `json:"deleted"`   // deleted node ids, sorted
	Pages          []string `json:"pages"`     // created, modified or deleted page ids, sorted
	Artboards      []string `json:"artboards"` // every artboard any op touched (before or after), sorted
	Tokens         []string `json:"tokens"`    // set or deleted token names, sorted
	Assets         []string `json:"assets"`
	Threads        []string `json:"threads"`
	Reviews        []string `json:"reviews"`
	ContentChanged bool     `json:"contentChanged"` // false for a batch of only comment, review, asset or review_mark ops
}
