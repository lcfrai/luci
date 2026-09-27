package doc

import "sort"

// Kind is what a node is. Every kind exports as one of AllowedTags[Kind].
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

// VoidTag is a tag that may hold no children whatever its kind allows.
func VoidTag(tag string) bool { return tag == "input" }

// AuthorKind is who made an edit: a person at the cockpit, or an agent session.
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
	Handle    string     `json:"handle"`    // the session's title (+ " · " + its id while another live session on the branch shares the title, task 19), or the person's display name
}

// Node is one element of a canvas.
type Node struct {
	ID       string            `json:"id"`       // n-<12 hex>
	Kind     Kind              `json:"kind"`     //
	Tag      string            `json:"tag"`      // one of AllowedTags[Kind]
	Name     string            `json:"name"`     // the layer name; ≤ 120 runes; "" = derived in the UI
	ParentID string            `json:"parentId"` // a page id for an artboard, a node id otherwise
	Order    string            `json:"order"`    // fractional index among siblings (order.go)
	Styles   map[string]string `json:"styles"`   // allowlisted property → value (task 07); {} not null
	Text     string            `json:"text"`     // KindText only; ≤ 10,000 runes
	Asset    string            `json:"asset"`    // KindImage only: an asset ref
	Alt      string            `json:"alt"`      // KindImage only; ≤ 500 runes
	SVG      string            `json:"svg"`      // KindSVG only: canonical sanitised markup (task 07); ≤ 256 KiB
	Attrs    map[string]string `json:"attrs"`    // AllowedAttrs only; {} not null
	X        int               `json:"x"`        // KindArtboard only: its place on the page, CSS px (0 otherwise)
	Y        int               `json:"y"`
	Locked   bool              `json:"locked"`
	Hidden   bool              `json:"hidden"`
	Revision int               `json:"revision"` // the last canvas revision that changed THIS node's fields (P5-K)
	EditedBy Author            `json:"editedBy"`
}

// Page is one page of a canvas; artboards are its children.
type Page struct {
	ID    string `json:"id"`    // pg-<12 hex>
	Name  string `json:"name"`  // ≤ 120 runes
	Order string `json:"order"` // fractional index among pages
}

// TokenType is one of the ten token types of Paper's public docs (spec §2.4, [S3]).
type TokenType string

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
	Type  TokenType `json:"type"`  //
	Value string    `json:"value"` // checked by dialect.Checker.TokenValue (task 07)
}

// Asset is content-addressed image bytes. The bytes live in the store (task 10).
type Asset struct {
	Ref       string `json:"ref"`       // AssetRef(sha256) — "asset:sha256-<64 hex>"
	MediaType string `json:"mediaType"` // the Hub's blob types, contract §7.6
	Size      int64  `json:"size"`      // ≤ 10 MiB (spec §8.4)
	Width     int    `json:"width"`     // 0 when unknown
	Height    int    `json:"height"`
}

// AllowedMediaTypes is every media type an asset may carry.
var AllowedMediaTypes = []string{"image/png", "image/jpeg", "image/webp", "image/svg+xml"}

// ThreadStatus is whether a comment thread is still open.
type ThreadStatus string

const (
	ThreadOpen     ThreadStatus = "open"
	ThreadResolved ThreadStatus = "resolved"
)

// Message is one comment in a thread.
type Message struct {
	ID     string `json:"id"`   // msg-<12 hex>
	Text   string `json:"text"` // 1–4,000 runes
	At     string `json:"at"`   // RFC 3339 UTC, milliseconds
	Author Author `json:"author"`
}

// Thread is a comment thread anchored to a node.
type Thread struct {
	ID        string       `json:"id"`     // th-<12 hex>
	NodeID    string       `json:"nodeId"` // the anchor; kept when the node is deleted (shown detached)
	Status    ThreadStatus `json:"status"`
	Messages  []Message    `json:"messages"`
	CreatedAt string       `json:"createdAt"`
	CreatedBy Author       `json:"createdBy"`
}

// ReviewState is whether a review request is still waiting on a person.
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
	BranchID  string          `json:"branchId"`  // the LOCAL branch id the canvas was CREATED on
	Revision  int             `json:"revision"`  // 0 for a new canvas
	CreatedAt string          `json:"createdAt"`
	Pages     []Page          `json:"pages"`   // by Order, then ID
	Nodes     []Node          `json:"nodes"`   // by ID
	Tokens    []Token         `json:"tokens"`  // by Name
	Assets    []Asset         `json:"assets"`  // by Ref
	Threads   []Thread        `json:"threads"` // by ID
	Reviews   []ReviewRequest `json:"reviews"` // by ID
}

// DocumentVersion is the only Document.V this package reads or writes.
const DocumentVersion = 1

// Doc is the in-memory form the authority mutates. It is not safe for
// concurrent use; the authority serialises access per canvas (task 11).
type Doc struct {
	v         int
	canvasID  string
	projectID string
	branchID  string
	createdAt string
	revision  int

	pages   map[string]Page
	nodes   map[string]Node
	tokens  map[string]Token
	assets  map[string]Asset
	threads map[string]Thread
	reviews map[string]ReviewRequest

	// children is the child index: a page or node id → its child node ids,
	// in no order. Readers sort by Order, then ID.
	children map[string][]string
}

// NewDoc is an empty canvas at revision 0.
func NewDoc(canvasID, projectID, branchID, createdAt string) *Doc {
	return &Doc{
		v: DocumentVersion, canvasID: canvasID, projectID: projectID,
		branchID: branchID, createdAt: createdAt,
		pages:    map[string]Page{},
		nodes:    map[string]Node{},
		tokens:   map[string]Token{},
		assets:   map[string]Asset{},
		threads:  map[string]Thread{},
		reviews:  map[string]ReviewRequest{},
		children: map[string][]string{},
	}
}

// copyStrings is a defensive copy of a map field, always non-nil.
func copyStrings(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// copyNode returns n with its maps copied and never nil, so a caller cannot
// reach into the Doc through a value it was handed.
func copyNode(n Node) Node {
	n.Styles = copyStrings(n.Styles)
	n.Attrs = copyStrings(n.Attrs)
	return n
}

func copyThread(t Thread) Thread {
	t.Messages = append([]Message(nil), t.Messages...)
	if t.Messages == nil {
		t.Messages = []Message{}
	}
	return t
}

func copyReview(r ReviewRequest) ReviewRequest {
	r.ArtboardIDs = append([]string(nil), r.ArtboardIDs...)
	if r.ArtboardIDs == nil {
		r.ArtboardIDs = []string{}
	}
	return r
}

// FromDocument validates every invariant of validate.go and indexes the
// document. A document that fails is never partly loaded.
func FromDocument(d Document) (*Doc, error) {
	if err := validateDocument(d); err != nil {
		return nil, err
	}
	doc := NewDoc(d.CanvasID, d.ProjectID, d.BranchID, d.CreatedAt)
	doc.v = d.V
	doc.revision = d.Revision
	for _, p := range d.Pages {
		doc.pages[p.ID] = p
	}
	for _, n := range d.Nodes {
		doc.nodes[n.ID] = copyNode(n)
		doc.children[n.ParentID] = append(doc.children[n.ParentID], n.ID)
	}
	for _, tk := range d.Tokens {
		doc.tokens[tk.Name] = tk
	}
	for _, a := range d.Assets {
		doc.assets[a.Ref] = a
	}
	for _, th := range d.Threads {
		doc.threads[th.ID] = copyThread(th)
	}
	for _, rv := range d.Reviews {
		doc.reviews[rv.ID] = copyReview(rv)
	}
	return doc, nil
}

// sortNodes puts a slice in the sibling order: by Order, then by ID. Two
// siblings never share an Order (validate.go), so the ID is only a tie-break
// across parents.
func sortNodes(ns []Node) {
	sort.Slice(ns, func(i, j int) bool {
		if ns[i].Order != ns[j].Order {
			return ns[i].Order < ns[j].Order
		}
		return ns[i].ID < ns[j].ID
	})
}

// Revision is the canvas revision this Doc holds.
func (d *Doc) Revision() int { return d.revision }

// CanvasID is the canvas's id, so a caller needing one field does not build a
// whole Document (tasks 10 and 11).
func (d *Doc) CanvasID() string { return d.canvasID }

// Node is one node by id; the value is a copy.
func (d *Doc) Node(id string) (Node, bool) {
	n, ok := d.nodes[id]
	if !ok {
		return Node{}, false
	}
	return copyNode(n), true
}

// Page is one page by id.
func (d *Doc) Page(id string) (Page, bool) {
	p, ok := d.pages[id]
	return p, ok
}

// Children is every child node of a page or node, by Order, then ID.
func (d *Doc) Children(parentID string) []Node {
	ids := d.children[parentID]
	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := d.nodes[id]; ok {
			out = append(out, copyNode(n))
		}
	}
	sortNodes(out)
	return out
}

// Pages is every page, by Order, then ID.
func (d *Doc) Pages() []Page {
	out := make([]Page, 0, len(d.pages))
	for _, p := range d.pages {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Artboards is a page's artboards, by Order, then ID.
func (d *Doc) Artboards(pageID string) []Node { return d.Children(pageID) }

// ArtboardOf is the artboard ancestor of a node, or the node itself when it is
// one. It reports false for a node this canvas does not hold.
func (d *Doc) ArtboardOf(nodeID string) (string, bool) {
	cur, ok := d.nodes[nodeID]
	if !ok {
		return "", false
	}
	for i := 0; i <= MaxDepth; i++ { // the depth bound also bounds a cycle
		if cur.Kind == KindArtboard {
			return cur.ID, true
		}
		next, ok := d.nodes[cur.ParentID]
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

// Subtree is a node and its descendants, pre-order, children by Order.
func (d *Doc) Subtree(id string) []Node {
	root, ok := d.nodes[id]
	if !ok {
		return []Node{}
	}
	out := []Node{copyNode(root)}
	var walk func(string, int)
	walk = func(parentID string, depth int) {
		if depth > MaxDepth {
			return
		}
		for _, c := range d.Children(parentID) {
			out = append(out, c)
			walk(c.ID, depth+1)
		}
	}
	walk(id, 1)
	return out
}

// Token is one token by name.
func (d *Doc) Token(name string) (Token, bool) {
	t, ok := d.tokens[name]
	return t, ok
}

// Tokens is every token, by Name.
func (d *Doc) Tokens() []Token {
	out := make([]Token, 0, len(d.tokens))
	for _, t := range d.tokens {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Asset is one asset by ref.
func (d *Doc) Asset(ref string) (Asset, bool) {
	a, ok := d.assets[ref]
	return a, ok
}

// Thread is one comment thread by id; the value is a copy.
func (d *Doc) Thread(id string) (Thread, bool) {
	t, ok := d.threads[id]
	if !ok {
		return Thread{}, false
	}
	return copyThread(t), true
}

// Threads is every comment thread, by ID.
func (d *Doc) Threads() []Thread {
	out := make([]Thread, 0, len(d.threads))
	for _, t := range d.threads {
		out = append(out, copyThread(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Reviews is every review request, by ID.
func (d *Doc) Reviews() []ReviewRequest {
	out := make([]ReviewRequest, 0, len(d.reviews))
	for _, r := range d.reviews {
		out = append(out, copyReview(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Document is the canvas as JSON, sorted as Document requires and with every
// slice and map non-nil, so two equal documents marshal to equal bytes.
func (d *Doc) Document() Document {
	out := Document{
		V: d.v, CanvasID: d.canvasID, ProjectID: d.projectID, BranchID: d.branchID,
		Revision: d.revision, CreatedAt: d.createdAt,
		Pages:   d.Pages(),
		Nodes:   make([]Node, 0, len(d.nodes)),
		Tokens:  d.Tokens(),
		Assets:  make([]Asset, 0, len(d.assets)),
		Threads: d.Threads(),
		Reviews: d.Reviews(),
	}
	for _, n := range d.nodes {
		out.Nodes = append(out.Nodes, copyNode(n))
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	for _, a := range d.assets {
		out.Assets = append(out.Assets, a)
	}
	sort.Slice(out.Assets, func(i, j int) bool { return out.Assets[i].Ref < out.Assets[j].Ref })
	return out
}
