package doc

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Apply applies ops atomically as one new revision (Revision()+1), stamping
// author on every node it creates or changes and setting that node's Revision.
// On error the Doc is exactly as before and the error is *OpError. label is
// copied into the Change. at is RFC 3339 UTC with milliseconds.
// Apply is Prepare followed by Commit.
func (d *Doc) Apply(ops []Op, author Author, at, label string, c Checker) (Change, error) {
	p, err := d.Prepare(ops, author, at, label, c)
	if err != nil {
		return Change{}, err
	}
	p.Commit()
	return p.Change, nil
}

// Prepare validates ops and computes the Change and its inverse against a
// copy-on-write overlay of d, without changing d. The authority prepares, makes
// the change durable (task 10), and only then commits — so a failed disk write
// leaves memory and disk agreeing.
func (d *Doc) Prepare(ops []Op, author Author, at, label string, c Checker) (*Pending, error) {
	if c == nil {
		return nil, opErrf(0, CodeInvalid, "", "a batch is applied against a dialect, and none was given")
	}
	if err := author.Validate(); err != nil {
		return nil, opErrf(0, CodeInvalid, "", "%s", err)
	}
	if !ValidTime(at) {
		return nil, opErrf(0, CodeInvalid, "", "%q is not a %s time", at, TimeFormat)
	}
	if utf8.RuneCountInString(label) > MaxLabelRunes {
		return nil, opErrf(0, CodeInvalid, "", "the label is over %d runes", MaxLabelRunes)
	}
	if len(ops) == 0 {
		return nil, opErrf(0, CodeEmpty, "", "a batch with no ops")
	}
	if len(ops) > MaxBatchOps {
		return nil, opErrf(MaxBatchOps, CodeLimit, "", "a batch is at most %d ops, this one is %d", MaxBatchOps, len(ops))
	}

	b := &batch{
		w: &work{base: d}, c: c, author: author, rev: d.revision + 1,
		ops:       make([]Op, 0, len(ops)),
		inverse:   make([][]Op, 0, len(ops)),
		nodes:     map[string]bool{},
		deleted:   map[string]bool{},
		pages:     map[string]bool{},
		artboards: map[string]bool{},
		tokens:    map[string]bool{},
		assets:    map[string]bool{},
		threads:   map[string]bool{},
		reviews:   map[string]bool{},
	}
	for i, op := range ops {
		if err := b.one(i, op); err != nil {
			return nil, err
		}
	}
	return &Pending{Change: b.change(at, label), d: d, rev: d.revision, w: b.w}, nil
}

// Pending is a prepared, uncommitted batch.
type Pending struct {
	Change Change

	d   *Doc
	rev int
	w   *work
}

// Commit makes the prepared batch the Doc's state. It panics if the Doc has
// changed since Prepare: the authority holds the canvas lock across both.
func (p *Pending) Commit() {
	if p.d.revision != p.rev {
		panic(fmt.Sprintf("doc: a batch prepared at revision %d committed against revision %d",
			p.rev, p.d.revision))
	}
	w := p.w
	if w.pages != nil {
		p.d.pages = w.pages
	}
	if w.nodes != nil {
		p.d.nodes = w.nodes
	}
	if w.tokens != nil {
		p.d.tokens = w.tokens
	}
	if w.assets != nil {
		p.d.assets = w.assets
	}
	if w.threads != nil {
		p.d.threads = w.threads
	}
	if w.reviews != nil {
		p.d.reviews = w.reviews
	}
	if w.children != nil {
		p.d.children = w.children
	}
	p.d.revision = p.Change.Revision
}

func opErrf(index int, code, id, format string, a ...any) *OpError {
	return &OpError{Index: index, Code: code, NodeID: id, Message: fmt.Sprintf(format, a...)}
}

// work is the copy-on-write overlay Prepare builds against. Each top-level map
// is cloned the first time the batch writes to it and left shared otherwise, so
// a batch that touches three nodes does not deep-clone a 50,000 node canvas.
// A node's own Styles and Attrs maps are copied before they are written, which
// is the second half of the same promise.
type work struct {
	base     *Doc
	pages    map[string]Page
	nodes    map[string]Node
	tokens   map[string]Token
	assets   map[string]Asset
	threads  map[string]Thread
	reviews  map[string]ReviewRequest
	children map[string][]string
}

func clone[K comparable, V any](m map[K]V) map[K]V {
	out := maps.Clone(m)
	if out == nil {
		out = map[K]V{}
	}
	return out
}

func (w *work) pageMap() map[string]Page {
	if w.pages == nil {
		w.pages = clone(w.base.pages)
	}
	return w.pages
}

func (w *work) nodeMap() map[string]Node {
	if w.nodes == nil {
		w.nodes = clone(w.base.nodes)
	}
	return w.nodes
}

func (w *work) tokenMap() map[string]Token {
	if w.tokens == nil {
		w.tokens = clone(w.base.tokens)
	}
	return w.tokens
}

func (w *work) assetMap() map[string]Asset {
	if w.assets == nil {
		w.assets = clone(w.base.assets)
	}
	return w.assets
}

func (w *work) threadMap() map[string]Thread {
	if w.threads == nil {
		w.threads = clone(w.base.threads)
	}
	return w.threads
}

func (w *work) reviewMap() map[string]ReviewRequest {
	if w.reviews == nil {
		w.reviews = clone(w.base.reviews)
	}
	return w.reviews
}

func (w *work) childMap() map[string][]string {
	if w.children == nil {
		w.children = clone(w.base.children)
	}
	return w.children
}

func (w *work) page(id string) (Page, bool) {
	if w.pages != nil {
		p, ok := w.pages[id]
		return p, ok
	}
	p, ok := w.base.pages[id]
	return p, ok
}

func (w *work) node(id string) (Node, bool) {
	if w.nodes != nil {
		n, ok := w.nodes[id]
		return n, ok
	}
	n, ok := w.base.nodes[id]
	return n, ok
}

func (w *work) token(name string) (Token, bool) {
	if w.tokens != nil {
		t, ok := w.tokens[name]
		return t, ok
	}
	t, ok := w.base.tokens[name]
	return t, ok
}

func (w *work) asset(ref string) (Asset, bool) {
	if w.assets != nil {
		a, ok := w.assets[ref]
		return a, ok
	}
	a, ok := w.base.assets[ref]
	return a, ok
}

func (w *work) thread(id string) (Thread, bool) {
	if w.threads != nil {
		t, ok := w.threads[id]
		return t, ok
	}
	t, ok := w.base.threads[id]
	return t, ok
}

func (w *work) review(id string) (ReviewRequest, bool) {
	if w.reviews != nil {
		r, ok := w.reviews[id]
		return r, ok
	}
	r, ok := w.base.reviews[id]
	return r, ok
}

func (w *work) pageCount() int {
	if w.pages != nil {
		return len(w.pages)
	}
	return len(w.base.pages)
}

func (w *work) nodeCount() int {
	if w.nodes != nil {
		return len(w.nodes)
	}
	return len(w.base.nodes)
}

// eachPage walks every page of the working copy, in no order.
func (w *work) eachPage(fn func(Page)) {
	m := w.base.pages
	if w.pages != nil {
		m = w.pages
	}
	for _, p := range m {
		fn(p)
	}
}

func (w *work) childIDs(parent string) []string {
	if w.children != nil {
		return w.children[parent]
	}
	return w.base.children[parent]
}

func (w *work) addChild(parent, id string) {
	m := w.childMap()
	cur := m[parent]
	next := make([]string, len(cur), len(cur)+1)
	copy(next, cur)
	m[parent] = append(next, id)
}

func (w *work) removeChild(parent, id string) {
	m := w.childMap()
	cur := m[parent]
	next := make([]string, 0, len(cur))
	for _, c := range cur {
		if c != id {
			next = append(next, c)
		}
	}
	if len(next) == 0 {
		delete(m, parent)
		return
	}
	m[parent] = next
}

// sortedChildren is a parent's children by Order, then ID.
func (w *work) sortedChildren(parent string) []Node {
	ids := w.childIDs(parent)
	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := w.node(id); ok {
			out = append(out, n)
		}
	}
	sortNodes(out)
	return out
}

// subtreeNodes is a node and its descendants, pre-order, children by Order.
func (w *work) subtreeNodes(id string) []Node {
	root, ok := w.node(id)
	if !ok {
		return nil
	}
	out := []Node{root}
	var walk func(string, int)
	walk = func(parent string, depth int) {
		if depth > MaxDepth {
			return
		}
		for _, k := range w.sortedChildren(parent) {
			out = append(out, k)
			walk(k.ID, depth+1)
		}
	}
	walk(id, 1)
	return out
}

// artboardOf is the artboard ancestor of a node, or the node itself.
func (w *work) artboardOf(id string) (string, bool) {
	cur, ok := w.node(id)
	if !ok {
		return "", false
	}
	for i := 0; i <= MaxDepth; i++ {
		if cur.Kind == KindArtboard {
			return cur.ID, true
		}
		next, ok := w.node(cur.ParentID)
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

// depthOf is 1 for an artboard and one more for each step down. It reports an
// error past MaxDepth, which is also how an unreachable chain ends.
func (w *work) depthOf(n Node) (int, bool) {
	cur, depth := n, 1
	for cur.Kind != KindArtboard {
		parent, ok := w.node(cur.ParentID)
		if !ok {
			return 0, false
		}
		cur = parent
		depth++
		if depth > MaxDepth {
			return depth, false
		}
	}
	return depth, true
}

// height is the depth of the deepest node of a subtree, counting its root as 1.
func (w *work) height(id string, guard int) int {
	if guard > MaxDepth {
		return guard
	}
	h := 1
	for _, k := range w.childIDs(id) {
		if kh := w.height(k, guard+1) + 1; kh > h {
			h = kh
		}
	}
	return h
}

// batch is one atomic run of ops over a work overlay, and the Change it built.
type batch struct {
	w      *work
	c      Checker
	author Author
	rev    int

	ops     []Op   // as applied, with any value the dialect canonicalised
	inverse [][]Op // one group per op, in op order; reversed into the Change

	nodes     map[string]bool
	deleted   map[string]bool
	pages     map[string]bool
	artboards map[string]bool
	tokens    map[string]bool
	assets    map[string]bool
	threads   map[string]bool
	reviews   map[string]bool
	content   bool
}

// record keeps the op as applied and its inverse group, and is called once per
// op, by one/1 only after every refusal of that op has been ruled out.
func (b *batch) record(op Op, inverse ...Op) {
	b.ops = append(b.ops, op)
	b.inverse = append(b.inverse, inverse)
	if Invertible(op.Kind) {
		b.content = true
	}
}

// touchArtboard notes the artboard a node sat in or now sits in, if it has one.
func (b *batch) touchArtboard(nodeID string) {
	if a, ok := b.w.artboardOf(nodeID); ok {
		b.artboards[a] = true
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (b *batch) change(at, label string) Change {
	inv := make([]Op, 0, len(b.inverse))
	for i := len(b.inverse) - 1; i >= 0; i-- {
		inv = append(inv, b.inverse[i]...)
	}
	return Change{
		Revision: b.rev, Author: b.author, At: at, Label: label,
		Ops: b.ops, Inverse: inv,
		Nodes:     sortedKeys(b.nodes),
		Deleted:   sortedKeys(b.deleted),
		Pages:     sortedKeys(b.pages),
		Artboards: sortedKeys(b.artboards),
		Tokens:    sortedKeys(b.tokens),
		Assets:    sortedKeys(b.assets),
		Threads:   sortedKeys(b.threads),
		Reviews:   sortedKeys(b.reviews),

		ContentChanged: b.content,
	}
}

// one applies a single op to the overlay. Every refusal is an *OpError naming
// this op's index, and leaves the overlay untouched for that op — which is what
// makes the batch atomic, because Prepare drops the whole overlay on error.
func (b *batch) one(i int, op Op) error {
	switch op.Kind {
	case OpCreatePage:
		return b.createPage(i, op)
	case OpUpdatePage:
		return b.updatePage(i, op)
	case OpDeletePage:
		return b.deletePage(i, op)
	case OpCreateNode:
		return b.createNode(i, op)
	case OpSetProps:
		return b.setProps(i, op)
	case OpMove:
		return b.move(i, op)
	case OpDeleteNode:
		return b.deleteNode(i, op)
	case OpSetToken:
		return b.setToken(i, op)
	case OpDeleteToken:
		return b.deleteToken(i, op)
	case OpAddAsset:
		return b.addAsset(i, op)
	case OpCreateThread:
		return b.createThread(i, op)
	case OpAddMessage:
		return b.addMessage(i, op)
	case OpSetThreadStatus:
		return b.setThreadStatus(i, op)
	case OpRequestReview:
		return b.requestReview(i, op)
	case OpCloseReview:
		return b.closeReview(i, op)
	case OpReviewMark:
		return b.reviewMark(i, op)
	}
	return opErrf(i, CodeInvalid, "", "%q is not an op kind", op.Kind)
}

// inSubtree reports whether candidate is root or one of its descendants. It
// walks upwards, so it is bounded by MaxDepth whatever the index says.
func (w *work) inSubtree(root, candidate string) bool {
	cur := candidate
	for i := 0; i <= MaxDepth; i++ {
		if cur == root {
			return true
		}
		n, ok := w.node(cur)
		if !ok {
			return false
		}
		cur = n.ParentID
	}
	return false
}

// pxLength is a CSS px length written plainly: digits, at most one point, "px".
func pxLength(v string) bool {
	num, ok := strings.CutSuffix(v, "px")
	if !ok || num == "" {
		return false
	}
	for i := 0; i < len(num); i++ {
		if (num[i] < '0' || num[i] > '9') && num[i] != '.' {
			return false
		}
	}
	f, err := strconv.ParseFloat(num, 64)
	return err == nil && f >= 0
}

// artboardSized is the one rule an artboard may never be left without: a frame
// the exporter can give a real size to. A width in px, and a height in px or
// fit-content (a page that grows with its content).
func artboardSized(styles map[string]string) error {
	w, ok := styles["width"]
	if !ok || !pxLength(w) {
		return fmt.Errorf("an artboard carries a width in px, not %q", w)
	}
	h, ok := styles["height"]
	if !ok || (h != "fit-content" && !pxLength(h)) {
		return fmt.Errorf("an artboard carries a height in px or fit-content, not %q", h)
	}
	return nil
}

func (b *batch) createPage(i int, op Op) error {
	if op.Page == nil {
		return opErrf(i, CodeInvalid, "", "create_page carries no page")
	}
	p := *op.Page
	if !ValidID("pg", p.ID) {
		return opErrf(i, CodeInvalid, p.ID, "%q is not a page id", p.ID)
	}
	if _, taken := b.w.page(p.ID); taken {
		return opErrf(i, CodeExists, p.ID, "the page %s already exists", p.ID)
	}
	if !ValidOrder(p.Order) {
		return opErrf(i, CodeInvalid, p.ID, "%q is not an order key", p.Order)
	}
	if utf8.RuneCountInString(p.Name) > MaxNameRunes {
		return opErrf(i, CodeInvalid, p.ID, "the name is over %d runes", MaxNameRunes)
	}
	var clash string
	b.w.eachPage(func(o Page) {
		if o.Order == p.Order {
			clash = o.ID
		}
	})
	if clash != "" {
		return opErrf(i, CodeExists, p.ID, "the page %s already sits at %q", clash, p.Order)
	}

	b.w.pageMap()[p.ID] = p
	b.pages[p.ID] = true
	b.record(op, Op{Kind: OpDeletePage, PageID: p.ID})
	return nil
}

func (b *batch) updatePage(i int, op Op) error {
	p, ok := b.w.page(op.PageID)
	if !ok {
		return opErrf(i, CodeNotFound, op.PageID, "no page %s", op.PageID)
	}
	old := p
	inv := Op{Kind: OpUpdatePage, PageID: p.ID}
	if op.Name != nil {
		if utf8.RuneCountInString(*op.Name) > MaxNameRunes {
			return opErrf(i, CodeInvalid, p.ID, "the name is over %d runes", MaxNameRunes)
		}
		name := old.Name
		inv.Name = &name
		p.Name = *op.Name
	}
	if op.Order != nil {
		if !ValidOrder(*op.Order) {
			return opErrf(i, CodeInvalid, p.ID, "%q is not an order key", *op.Order)
		}
		var clash string
		b.w.eachPage(func(o Page) {
			if o.ID != p.ID && o.Order == *op.Order {
				clash = o.ID
			}
		})
		if clash != "" {
			return opErrf(i, CodeExists, p.ID, "the page %s already sits at %q", clash, *op.Order)
		}
		order := old.Order
		inv.Order = &order
		p.Order = *op.Order
	}

	b.w.pageMap()[p.ID] = p
	b.pages[p.ID] = true
	b.record(op, inv)
	return nil
}

func (b *batch) deletePage(i int, op Op) error {
	p, ok := b.w.page(op.PageID)
	if !ok {
		return opErrf(i, CodeNotFound, op.PageID, "no page %s", op.PageID)
	}
	if b.w.pageCount() <= 1 {
		return opErrf(i, CodeInvalid, op.PageID, "a canvas keeps at least one page")
	}

	page := p
	inv := []Op{{Kind: OpCreatePage, Page: &page}}
	for _, art := range b.w.sortedChildren(p.ID) {
		for _, n := range b.w.subtreeNodes(art.ID) {
			node := n
			inv = append(inv, Op{Kind: OpCreateNode, Node: &node})
			delete(b.w.nodeMap(), n.ID)
			delete(b.w.childMap(), n.ID)
			b.deleted[n.ID] = true
			delete(b.nodes, n.ID)
		}
		b.artboards[art.ID] = true
	}
	delete(b.w.childMap(), p.ID)
	delete(b.w.pageMap(), p.ID)
	b.pages[p.ID] = true
	b.record(op, inv...)
	return nil
}

// stamp marks a node as this batch's work: its own fields changed, so its
// Revision moves to the batch's and its EditedBy is the batch's author.
func (b *batch) stamp(n *Node) {
	n.Revision = b.rev
	n.EditedBy = b.author
	n.Styles = copyStrings(n.Styles)
	n.Attrs = copyStrings(n.Attrs)
}

// orderTaken is the sibling already sitting at order under parent, excluding
// the node being moved, or "" when the place is free.
func (b *batch) orderTaken(parent, order, except string) string {
	for _, s := range b.w.sortedChildren(parent) {
		if s.ID != except && s.Order == order {
			return s.ID
		}
	}
	return ""
}

func (b *batch) createNode(i int, op Op) error {
	if op.Node == nil {
		return opErrf(i, CodeInvalid, "", "create_node carries no node")
	}
	n := *op.Node
	if !ValidID("n", n.ID) {
		return opErrf(i, CodeInvalid, n.ID, "%q is not a node id", n.ID)
	}
	if _, taken := b.w.node(n.ID); taken {
		return opErrf(i, CodeExists, n.ID, "the node %s already exists", n.ID)
	}
	b.stamp(&n)
	if n.Kind == KindSVG {
		canonical, err := b.c.SVG(n.ID, n.SVG)
		if err != nil {
			return opErrf(i, CodeInvalid, n.ID, "%s", err)
		}
		n.SVG = canonical
	}
	if err := n.Validate(); err != nil {
		return opErrf(i, CodeInvalid, n.ID, "%s", err)
	}

	if n.Kind == KindArtboard {
		if _, ok := b.w.page(n.ParentID); !ok {
			return opErrf(i, CodeNotFound, n.ID, "no page %s", n.ParentID)
		}
	} else {
		parent, ok := b.w.node(n.ParentID)
		if !ok {
			return opErrf(i, CodeNotFound, n.ID, "no node %s", n.ParentID)
		}
		if err := canHoldChildren(parent); err != nil {
			return opErrf(i, CodeInvalid, n.ID, "%s", err)
		}
	}
	if clash := b.orderTaken(n.ParentID, n.Order, ""); clash != "" {
		return opErrf(i, CodeExists, n.ID, "%s already sits at %q under %s", clash, n.Order, n.ParentID)
	}
	if _, ok := b.w.depthOf(n); !ok {
		return opErrf(i, CodeLimit, n.ID, "a node is at most %d deep", MaxDepth)
	}
	if b.w.nodeCount() >= MaxNodes {
		return opErrf(i, CodeLimit, n.ID, "a canvas holds at most %d nodes", MaxNodes)
	}
	if err := b.styles(n.ID, n.Styles); err != nil {
		return opErrf(i, CodeStyle, n.ID, "%s", err)
	}
	if n.Kind == KindImage {
		if _, ok := b.w.asset(n.Asset); !ok {
			return opErrf(i, CodeInvalid, n.ID, "the canvas holds no asset %s", n.Asset)
		}
	}
	if n.Kind == KindArtboard {
		if err := artboardSized(n.Styles); err != nil {
			return opErrf(i, CodeInvalid, n.ID, "%s", err)
		}
	}

	b.w.nodeMap()[n.ID] = n
	b.w.addChild(n.ParentID, n.ID)
	b.nodes[n.ID] = true
	applied := op
	applied.Node = &n
	b.record(applied, Op{Kind: OpDeleteNode, NodeID: n.ID})
	b.touchArtboard(n.ID)
	return nil
}

// styles runs every declaration past the dialect.
func (b *batch) styles(_ string, styles map[string]string) error {
	for property, value := range styles {
		if err := b.c.Style(property, value); err != nil {
			return err
		}
	}
	return nil
}

// patch is one map of the node — Styles or Attrs — written key by key: a nil
// value removes the key, and the inverse names exactly the keys the op named.
func patch(into map[string]string, from map[string]*string) map[string]*string {
	inv := make(map[string]*string, len(from))
	for k, v := range from {
		if old, had := into[k]; had {
			was := old
			inv[k] = &was
		} else {
			inv[k] = nil
		}
		if v == nil {
			delete(into, k)
			continue
		}
		into[k] = *v
	}
	return inv
}

func (b *batch) setProps(i int, op Op) error {
	old, ok := b.w.node(op.NodeID)
	if !ok {
		return opErrf(i, CodeNotFound, op.NodeID, "no node %s", op.NodeID)
	}
	b.touchArtboard(op.NodeID)

	for property, value := range op.Styles {
		if value == nil {
			continue
		}
		if err := b.c.Style(property, *value); err != nil {
			return opErrf(i, CodeStyle, op.NodeID, "%s", err)
		}
	}

	n := old
	b.stamp(&n) // copies Styles and Attrs, so the old maps are never written
	inv := Op{Kind: OpSetProps, NodeID: op.NodeID}
	if len(op.Styles) > 0 {
		inv.Styles = patch(n.Styles, op.Styles)
	}
	if len(op.Attrs) > 0 {
		inv.Attrs = patch(n.Attrs, op.Attrs)
	}
	applied := op
	setField(op.Name, &n.Name, &inv.Name)
	setField(op.Tag, &n.Tag, &inv.Tag)
	setField(op.Text, &n.Text, &inv.Text)
	setField(op.Alt, &n.Alt, &inv.Alt)
	setField(op.Asset, &n.Asset, &inv.Asset)
	setField(op.Locked, &n.Locked, &inv.Locked)
	setField(op.Hidden, &n.Hidden, &inv.Hidden)
	setField(op.X, &n.X, &inv.X)
	setField(op.Y, &n.Y, &inv.Y)
	if op.SVG != nil {
		canonical, err := b.c.SVG(op.NodeID, *op.SVG)
		if err != nil {
			return opErrf(i, CodeInvalid, op.NodeID, "%s", err)
		}
		was := old.SVG
		inv.SVG = &was
		n.SVG = canonical
		applied.SVG = &canonical
	}

	if err := n.Validate(); err != nil {
		return opErrf(i, CodeInvalid, n.ID, "%s", err)
	}
	if len(b.w.childIDs(n.ID)) > 0 {
		if err := canHoldChildren(n); err != nil {
			return opErrf(i, CodeInvalid, n.ID, "%s", err)
		}
	}
	if op.Asset != nil {
		if _, ok := b.w.asset(n.Asset); !ok {
			return opErrf(i, CodeInvalid, n.ID, "the canvas holds no asset %s", n.Asset)
		}
	}
	if n.Kind == KindArtboard {
		if err := artboardSized(n.Styles); err != nil {
			return opErrf(i, CodeInvalid, n.ID, "%s", err)
		}
	}

	b.w.nodeMap()[n.ID] = n
	b.nodes[n.ID] = true
	b.record(applied, inv)
	return nil
}

// setField writes a pointer field of an op into the node and the old value into
// the inverse, and does nothing at all when the op left the field alone.
func setField[T any](from *T, into *T, inverse **T) {
	if from == nil {
		return
	}
	was := *into
	*inverse = &was
	*into = *from
}

func (b *batch) move(i int, op Op) error {
	old, ok := b.w.node(op.NodeID)
	if !ok {
		return opErrf(i, CodeNotFound, op.NodeID, "no node %s", op.NodeID)
	}
	if op.ParentID == nil || op.Order == nil {
		return opErrf(i, CodeInvalid, op.NodeID, "a move names a parent and an order")
	}
	parentID, order := *op.ParentID, *op.Order
	b.touchArtboard(op.NodeID)

	if old.Kind == KindArtboard {
		if !ValidID("pg", parentID) {
			return opErrf(i, CodeInvalid, op.NodeID, "an artboard's parent is a page, not %q", parentID)
		}
		if _, ok := b.w.page(parentID); !ok {
			return opErrf(i, CodeNotFound, op.NodeID, "no page %s", parentID)
		}
	} else {
		if !ValidID("n", parentID) {
			return opErrf(i, CodeInvalid, op.NodeID, "the parent %q is not a node id", parentID)
		}
		parent, ok := b.w.node(parentID)
		if !ok {
			return opErrf(i, CodeNotFound, op.NodeID, "no node %s", parentID)
		}
		if err := canHoldChildren(parent); err != nil {
			return opErrf(i, CodeInvalid, op.NodeID, "%s", err)
		}
		if b.w.inSubtree(op.NodeID, parentID) {
			return opErrf(i, CodeCycle, op.NodeID, "%s is inside the subtree of %s", parentID, op.NodeID)
		}
	}
	if !ValidOrder(order) {
		return opErrf(i, CodeInvalid, op.NodeID, "%q is not an order key", order)
	}
	if clash := b.orderTaken(parentID, order, op.NodeID); clash != "" {
		return opErrf(i, CodeExists, op.NodeID, "%s already sits at %q under %s", clash, order, parentID)
	}

	n := old
	b.stamp(&n)
	n.ParentID, n.Order = parentID, order
	if depth, ok := b.w.depthOf(n); !ok || depth+b.w.height(op.NodeID, 1)-1 > MaxDepth {
		return opErrf(i, CodeLimit, op.NodeID, "a node is at most %d deep", MaxDepth)
	}

	b.w.removeChild(old.ParentID, op.NodeID)
	b.w.nodeMap()[n.ID] = n
	b.w.addChild(parentID, op.NodeID)
	b.nodes[n.ID] = true
	b.touchArtboard(op.NodeID)
	wasParent, wasOrder := old.ParentID, old.Order
	b.record(op, Op{Kind: OpMove, NodeID: op.NodeID, ParentID: &wasParent, Order: &wasOrder})
	return nil
}

func (b *batch) deleteNode(i int, op Op) error {
	n, ok := b.w.node(op.NodeID)
	if !ok {
		return opErrf(i, CodeNotFound, op.NodeID, "no node %s", op.NodeID)
	}
	b.touchArtboard(op.NodeID)

	inv := make([]Op, 0, 8)
	for _, s := range b.w.subtreeNodes(op.NodeID) {
		node := s
		inv = append(inv, Op{Kind: OpCreateNode, Node: &node})
		delete(b.w.nodeMap(), s.ID)
		delete(b.w.childMap(), s.ID)
		b.deleted[s.ID] = true
		delete(b.nodes, s.ID)
	}
	b.w.removeChild(n.ParentID, n.ID)
	b.record(op, inv...)
	return nil
}

func (b *batch) setToken(i int, op Op) error {
	if op.Token == nil {
		return opErrf(i, CodeInvalid, "", "set_token carries no token")
	}
	t := *op.Token
	if !ValidTokenName(t.Name) {
		return opErrf(i, CodeInvalid, t.Name, "%q is not a token name", t.Name)
	}
	if !slices.Contains(AllowedTokenTypes, t.Type) {
		return opErrf(i, CodeInvalid, t.Name, "unknown token type %q", t.Type)
	}
	if t.Value == "" {
		return opErrf(i, CodeInvalid, t.Name, "a token carries a value")
	}
	if err := b.c.TokenValue(t.Type, t.Value); err != nil {
		return opErrf(i, CodeInvalid, t.Name, "%s", err)
	}

	inv := Op{Kind: OpDeleteToken, TokenName: t.Name}
	if was, had := b.w.token(t.Name); had {
		old := was
		inv = Op{Kind: OpSetToken, Token: &old}
	}
	b.w.tokenMap()[t.Name] = t
	b.tokens[t.Name] = true
	b.record(op, inv)
	return nil
}

func (b *batch) deleteToken(i int, op Op) error {
	was, ok := b.w.token(op.TokenName)
	if !ok {
		return opErrf(i, CodeNotFound, op.TokenName, "no token %s", op.TokenName)
	}
	old := was
	delete(b.w.tokenMap(), op.TokenName)
	b.tokens[op.TokenName] = true
	b.record(op, Op{Kind: OpSetToken, Token: &old})
	return nil
}

func (b *batch) addAsset(i int, op Op) error {
	if op.AssetRecord == nil {
		return opErrf(i, CodeInvalid, "", "add_asset carries no asset")
	}
	a := *op.AssetRecord
	if !ValidAssetRef(a.Ref) {
		return opErrf(i, CodeInvalid, a.Ref, "%q is not an asset ref", a.Ref)
	}
	if !slices.Contains(AllowedMediaTypes, a.MediaType) {
		return opErrf(i, CodeInvalid, a.Ref, "unknown media type %q", a.MediaType)
	}
	if a.Size > MaxAssetBytes {
		return opErrf(i, CodeLimit, a.Ref, "an asset is at most %d bytes, this one is %d", MaxAssetBytes, a.Size)
	}
	if a.Size <= 0 || a.Width < 0 || a.Height < 0 {
		return opErrf(i, CodeInvalid, a.Ref, "%d bytes, %dx%d", a.Size, a.Width, a.Height)
	}
	if _, already := b.w.asset(a.Ref); already {
		b.record(op) // a ref is its bytes: the same ref twice is a no-op, not a refusal
		return nil
	}
	b.w.assetMap()[a.Ref] = a
	b.assets[a.Ref] = true
	b.record(op)
	return nil
}

func (b *batch) createThread(i int, op Op) error {
	if op.Thread == nil {
		return opErrf(i, CodeInvalid, "", "create_thread carries no thread")
	}
	th := copyThread(*op.Thread)
	if !ValidID("th", th.ID) {
		return opErrf(i, CodeInvalid, th.ID, "%q is not a thread id", th.ID)
	}
	if _, taken := b.w.thread(th.ID); taken {
		return opErrf(i, CodeExists, th.ID, "the thread %s already exists", th.ID)
	}
	if err := validateThreads([]Thread{th}); err != nil {
		return opErrf(i, CodeInvalid, th.ID, "%s", err)
	}
	if len(th.Messages) != 1 {
		return opErrf(i, CodeInvalid, th.ID, "a new thread carries exactly one message, not %d", len(th.Messages))
	}
	if _, ok := b.w.node(th.NodeID); !ok {
		return opErrf(i, CodeNotFound, th.ID, "no node %s to anchor the thread to", th.NodeID)
	}

	b.w.threadMap()[th.ID] = th
	b.threads[th.ID] = true
	b.record(op)
	return nil
}

func (b *batch) addMessage(i int, op Op) error {
	th, ok := b.w.thread(op.ThreadID)
	if !ok {
		return opErrf(i, CodeNotFound, op.ThreadID, "no thread %s", op.ThreadID)
	}
	if op.Message == nil {
		return opErrf(i, CodeInvalid, op.ThreadID, "add_message carries no message")
	}
	for _, m := range th.Messages {
		if m.ID == op.Message.ID {
			return opErrf(i, CodeExists, op.ThreadID, "the message %s is already in the thread", m.ID)
		}
	}
	next := copyThread(th)
	next.Messages = append(next.Messages, *op.Message)
	if err := validateThreads([]Thread{next}); err != nil {
		return opErrf(i, CodeInvalid, op.ThreadID, "%s", err)
	}

	b.w.threadMap()[th.ID] = next
	b.threads[th.ID] = true
	b.record(op)
	return nil
}

func (b *batch) setThreadStatus(i int, op Op) error {
	th, ok := b.w.thread(op.ThreadID)
	if !ok {
		return opErrf(i, CodeNotFound, op.ThreadID, "no thread %s", op.ThreadID)
	}
	if op.Status == nil {
		return opErrf(i, CodeInvalid, op.ThreadID, "set_thread_status carries no status")
	}
	if *op.Status != ThreadOpen && *op.Status != ThreadResolved {
		return opErrf(i, CodeInvalid, op.ThreadID, "unknown status %q", *op.Status)
	}

	next := copyThread(th)
	next.Status = *op.Status
	b.w.threadMap()[th.ID] = next
	b.threads[th.ID] = true
	b.record(op)
	return nil
}

func (b *batch) requestReview(i int, op Op) error {
	if op.Review == nil {
		return opErrf(i, CodeInvalid, "", "request_review carries no review request")
	}
	rv := copyReview(*op.Review)
	if !ValidID("rv", rv.ID) {
		return opErrf(i, CodeInvalid, rv.ID, "%q is not a review request id", rv.ID)
	}
	if _, taken := b.w.review(rv.ID); taken {
		return opErrf(i, CodeExists, rv.ID, "the review request %s already exists", rv.ID)
	}
	if rv.State != ReviewOpen {
		return opErrf(i, CodeInvalid, rv.ID, "a review request is made open, not %q", rv.State)
	}
	if err := validateReviews([]ReviewRequest{rv}, b.rev); err != nil {
		return opErrf(i, CodeInvalid, rv.ID, "%s", err)
	}
	for _, id := range rv.ArtboardIDs {
		n, ok := b.w.node(id)
		if !ok {
			return opErrf(i, CodeNotFound, rv.ID, "no node %s", id)
		}
		if n.Kind != KindArtboard {
			return opErrf(i, CodeInvalid, rv.ID, "%s is a %s, and a review is of artboards", id, n.Kind)
		}
		b.artboards[id] = true
	}

	b.w.reviewMap()[rv.ID] = rv
	b.reviews[rv.ID] = true
	b.record(op)
	return nil
}

func (b *batch) closeReview(i int, op Op) error {
	rv, ok := b.w.review(op.ReviewID)
	if !ok {
		return opErrf(i, CodeNotFound, op.ReviewID, "no review request %s", op.ReviewID)
	}
	if rv.State == ReviewClosed {
		return opErrf(i, CodeInvalid, op.ReviewID, "the review request %s is already closed", op.ReviewID)
	}

	next := copyReview(rv)
	next.State = ReviewClosed
	b.w.reviewMap()[rv.ID] = next
	b.reviews[rv.ID] = true
	for _, id := range next.ArtboardIDs {
		b.artboards[id] = true
	}
	b.record(op)
	return nil
}

// reviewMark changes no content. It exists so a re-review has a new design
// revision to approve after a brief moved on (brief D3, plan P5-M). The key is
// bounded and never parsed here.
func (b *batch) reviewMark(i int, op Op) error {
	if op.BriefKey == "" {
		return opErrf(i, CodeInvalid, "", "a review_mark carries a brief key")
	}
	if len(op.BriefKey) > MaxBriefKeyBytes {
		return opErrf(i, CodeInvalid, "", "the brief key is over %d bytes", MaxBriefKeyBytes)
	}
	b.record(op)
	return nil
}
