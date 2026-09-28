package doc

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

const testAt = "2026-09-28T01:02:03.456Z"

// tid is a deterministic id of the width ids.go reads: prefix + 12 hex digits.
func tid(prefix string, n int) string { return fmt.Sprintf("%s-%012x", prefix, n) }

func testAuthor() Author {
	return Author{Kind: AuthorHuman, UserID: "local", Handle: "Owner"}
}

func agentAuthor() Author {
	return Author{Kind: AuthorSession, UserID: "local", SessionID: "s-1", Handle: "Claude"}
}

// okChecker allows every declaration and returns SVG markup unchanged.
type okChecker struct{}

func (okChecker) Style(string, string) error         { return nil }
func (okChecker) TokenValue(TokenType, string) error { return nil }
func (okChecker) SVG(_, markup string) (string, error) {
	return markup, nil
}

// strictChecker is the dialect the plan's step 1 asks for: it refuses the two
// properties `margin` and `grid` and every token value spelled "bad".
type strictChecker struct{}

func (strictChecker) Style(property, _ string) error {
	if property == "margin" || property == "grid" {
		return fmt.Errorf("dialect: %q is not in the allowlist", property)
	}
	return nil
}

func (strictChecker) TokenValue(_ TokenType, value string) error {
	if value == "bad" {
		return errors.New("dialect: not a value of that type")
	}
	return nil
}

func (strictChecker) SVG(_, markup string) (string, error) { return markup, nil }

// tagChecker canonicalises SVG to nodeID + "|" + markup, and records every
// node id it was asked about, so a test can prove Apply passed the op's own id.
type tagChecker struct {
	seen []string
	err  error
}

func (c *tagChecker) Style(string, string) error         { return nil }
func (c *tagChecker) TokenValue(TokenType, string) error { return nil }
func (c *tagChecker) SVG(nodeID, markup string) (string, error) {
	c.seen = append(c.seen, nodeID)
	if c.err != nil {
		return "", c.err
	}
	return nodeID + "|" + markup, nil
}

// artboardStyles is the smallest size an artboard is allowed to carry.
func artboardStyles() map[string]string {
	return map[string]string{"width": "1440px", "height": "900px"}
}

// seeded is a canvas with one page and one artboard on it, at revision 1.
func seeded(t *testing.T) *Doc {
	t.Helper()
	d := NewDoc("cv-0000000000000001", "p-1", "b-1", testAt)
	_, err := d.Apply([]Op{
		{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 1), Name: "Page 1", Order: "V"}},
		{Kind: OpCreateNode, Node: &Node{
			ID: tid("n", 1), Kind: KindArtboard, Tag: "div", Name: "Home",
			ParentID: tid("pg", 1), Order: "V", Styles: artboardStyles(),
		}},
	}, testAuthor(), testAt, "seed", okChecker{})
	if err != nil {
		t.Fatalf("seeding the canvas: %v", err)
	}
	return d
}

func frame(id int, parent string, order string) Node {
	return Node{ID: tid("n", id), Kind: KindFrame, Tag: "div", ParentID: parent, Order: order}
}

func text(id int, parent, order, body string) Node {
	return Node{ID: tid("n", id), Kind: KindText, Tag: "p", ParentID: parent, Order: order, Text: body}
}

func opErr(t *testing.T, err error) *OpError {
	t.Helper()
	var oe *OpError
	if !errors.As(err, &oe) {
		t.Fatalf("want an *OpError, got %T: %v", err, err)
	}
	return oe
}

func mustApply(t *testing.T, d *Doc, ops []Op, c Checker) Change {
	t.Helper()
	ch, err := d.Apply(ops, testAuthor(), testAt, "batch", c)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return ch
}

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func ip(i int) *int       { return &i }

func TestApply_atomic(t *testing.T) {
	d := seeded(t)
	before := d.Document()
	rev := d.Revision()

	_, err := d.Apply([]Op{
		{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))},
		{Kind: OpCreateNode, Node: ptr(frame(4, tid("n", 999), "V"))}, // parent missing
		{Kind: OpCreateNode, Node: ptr(frame(5, tid("n", 2), "b"))},
	}, testAuthor(), testAt, "four frames", okChecker{})
	if err == nil {
		t.Fatal("want a refusal, the third op names a parent that does not exist")
	}
	oe := opErr(t, err)
	if oe.Index != 2 {
		t.Errorf("Index = %d, want 2 (the first op that failed)", oe.Index)
	}
	if oe.Code != CodeNotFound {
		t.Errorf("Code = %q, want %q", oe.Code, CodeNotFound)
	}
	if d.Revision() != rev {
		t.Errorf("Revision() = %d, want %d unchanged", d.Revision(), rev)
	}
	if !reflect.DeepEqual(d.Document(), before) {
		t.Error("the document changed under a refused batch")
	}
}

func ptr[T any](v T) *T { return &v }

func TestApply_oneRevisionPerBatch(t *testing.T) {
	d := seeded(t)
	rev := d.Revision()

	ops := make([]Op, 0, 30)
	for i := 0; i < 30; i++ {
		ops = append(ops, Op{Kind: OpCreateNode, Node: ptr(frame(100+i, tid("n", 1), orderKey(i)))})
	}
	ch, err := d.Apply(ops, agentAuthor(), testAt, "write_html", okChecker{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if d.Revision() != rev+1 {
		t.Fatalf("Revision() = %d, want %d: a batch is one revision", d.Revision(), rev+1)
	}
	if ch.Revision != rev+1 {
		t.Errorf("Change.Revision = %d, want %d", ch.Revision, rev+1)
	}
	for i := 0; i < 30; i++ {
		n, ok := d.Node(tid("n", 100+i))
		if !ok {
			t.Fatalf("node %s was not created", tid("n", 100+i))
		}
		if n.Revision != rev+1 {
			t.Errorf("node %s Revision = %d, want %d", n.ID, n.Revision, rev+1)
		}
		if n.EditedBy != agentAuthor() {
			t.Errorf("node %s EditedBy = %+v, want the batch author", n.ID, n.EditedBy)
		}
	}
}

// orderKey is 30 distinct, ascending, well-formed order keys.
func orderKey(i int) string {
	return string(OrderAlphabet[i%len(OrderAlphabet)]) + "V"
}

func TestApply_perPropertyLWW(t *testing.T) {
	d := seeded(t)
	n := tid("n", 2)
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}}, okChecker{})

	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: n, Styles: map[string]*string{"color": sp("red")}}}, okChecker{})
	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: n, Styles: map[string]*string{"padding": sp("8px")}}}, okChecker{})

	got, _ := d.Node(n)
	if got.Styles["color"] != "red" || got.Styles["padding"] != "8px" {
		t.Fatalf("styles = %v, want both color and padding: set_props overwrites only what it names", got.Styles)
	}

	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: n, Styles: map[string]*string{"color": sp("blue")}}}, okChecker{})
	got, _ = d.Node(n)
	if got.Styles["color"] != "blue" || got.Styles["padding"] != "8px" {
		t.Fatalf("styles = %v, want colour blue and padding kept", got.Styles)
	}

	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: n, Styles: map[string]*string{"color": nil}}}, okChecker{})
	got, _ = d.Node(n)
	if _, ok := got.Styles["color"]; ok {
		t.Fatalf("styles = %v, want colour removed by a null value", got.Styles)
	}
	if got.Styles["padding"] != "8px" {
		t.Fatalf("styles = %v, want padding kept through a removal", got.Styles)
	}
}

func TestApply_nodeRevisionOnlyForOwnFields(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}}, okChecker{})
	parent, _ := d.Node(tid("n", 2))
	parentRev := parent.Revision

	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: ptr(text(3, tid("n", 2), "V", "hello"))}}, okChecker{})

	after, _ := d.Node(tid("n", 2))
	if after.Revision != parentRev {
		t.Fatalf("parent Revision = %d, want %d: a child insert changes no field of the parent (P5-K)", after.Revision, parentRev)
	}
	if len(d.Children(tid("n", 2))) != 1 {
		t.Fatal("the child was not inserted")
	}
}

func TestApply_seesEarlierOps(t *testing.T) {
	d := seeded(t)
	ch := mustApply(t, d, []Op{
		{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
		{Kind: OpCreateNode, Node: ptr(text(3, tid("n", 2), "V", "hi"))},
	}, okChecker{})
	if _, ok := d.Node(tid("n", 3)); !ok {
		t.Fatal("the child of a frame created two ops earlier was refused")
	}
	if len(ch.Nodes) != 2 {
		t.Errorf("Change.Nodes = %v, want both created nodes", ch.Nodes)
	}
}

func TestApply_cycle(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{
		{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))},
		{Kind: OpCreateNode, Node: ptr(frame(4, tid("n", 3), "V"))},
	}, okChecker{})

	_, err := d.Apply([]Op{{Kind: OpMove, NodeID: tid("n", 2), ParentID: sp(tid("n", 4)), Order: sp("V")}},
		testAuthor(), testAt, "move", okChecker{})
	oe := opErr(t, err)
	if oe.Code != CodeCycle {
		t.Fatalf("Code = %q, want %q: a move under its own grandchild", oe.Code, CodeCycle)
	}
	if got, _ := d.Node(tid("n", 2)); got.ParentID != tid("n", 1) {
		t.Errorf("parent = %q, want the move refused", got.ParentID)
	}
}

func TestApply_artboardSize(t *testing.T) {
	d := seeded(t)

	fit := Node{ID: tid("n", 10), Kind: KindArtboard, Tag: "div", ParentID: tid("pg", 1), Order: "b",
		Styles: map[string]string{"width": "800px", "height": "fit-content"}}
	if _, err := d.Apply([]Op{{Kind: OpCreateNode, Node: &fit}}, testAuthor(), testAt, "fit", okChecker{}); err != nil {
		t.Fatalf("an artboard with height fit-content was refused: %v", err)
	}

	pct := Node{ID: tid("n", 11), Kind: KindArtboard, Tag: "div", ParentID: tid("pg", 1), Order: "c",
		Styles: map[string]string{"width": "50%", "height": "900px"}}
	_, err := d.Apply([]Op{{Kind: OpCreateNode, Node: &pct}}, testAuthor(), testAt, "pct", okChecker{})
	if oe := opErr(t, err); oe.Code != CodeInvalid {
		t.Fatalf("Code = %q, want %q: an artboard's width is a px length", oe.Code, CodeInvalid)
	}

	_, err = d.Apply([]Op{{Kind: OpSetProps, NodeID: tid("n", 1), Styles: map[string]*string{"width": nil}}},
		testAuthor(), testAt, "unsize", okChecker{})
	if oe := opErr(t, err); oe.Code != CodeInvalid {
		t.Fatalf("Code = %q, want %q: an artboard may not be left without its size", oe.Code, CodeInvalid)
	}
	if got, _ := d.Node(tid("n", 1)); got.Styles["width"] != "1440px" {
		t.Errorf("width = %q, want the removal refused", got.Styles["width"])
	}
}

func TestApply_checkerCalled(t *testing.T) {
	d := seeded(t)
	n := frame(2, tid("n", 1), "V")
	n.Styles = map[string]string{"padding": "4px", "margin": "4px"}

	_, err := d.Apply([]Op{
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 1), "b"))},
		{Kind: OpCreateNode, Node: &n},
	}, testAuthor(), testAt, "styled", strictChecker{})
	oe := opErr(t, err)
	if oe.Code != CodeStyle {
		t.Fatalf("Code = %q, want %q: the dialect refused margin", oe.Code, CodeStyle)
	}
	if oe.Index != 1 {
		t.Fatalf("Index = %d, want 1: the op that carried the refused declaration", oe.Index)
	}
	if oe.NodeID != tid("n", 2) {
		t.Errorf("NodeID = %q, want %q", oe.NodeID, tid("n", 2))
	}
	if _, ok := d.Node(tid("n", 3)); ok {
		t.Error("the first op of a refused batch was applied")
	}
}

func TestApply_svgCanonical(t *testing.T) {
	d := seeded(t)
	c := &tagChecker{}
	markup := `<svg viewBox="0 0 1 1"></svg>`
	node := Node{ID: tid("n", 2), Kind: KindSVG, Tag: "svg", ParentID: tid("n", 1), Order: "V", SVG: markup}

	ch := mustApply(t, d, []Op{{Kind: OpCreateNode, Node: &node}}, c)
	want := tid("n", 2) + "|" + markup
	got, _ := d.Node(tid("n", 2))
	if got.SVG != want {
		t.Fatalf("stored SVG = %q, want the checker's canonical return %q", got.SVG, want)
	}
	if ch.Ops[0].Node.SVG != want {
		t.Fatalf("Change.Ops[0] carries %q, want the canonical form: a replica must apply the same bytes", ch.Ops[0].Node.SVG)
	}
	if len(c.seen) != 1 || c.seen[0] != tid("n", 2) {
		t.Fatalf("the checker saw %v, want the op's own node id", c.seen)
	}
}

func TestApply_svgCanonicalOnSetProps(t *testing.T) {
	d := seeded(t)
	c := &tagChecker{}
	node := Node{ID: tid("n", 2), Kind: KindSVG, Tag: "svg", ParentID: tid("n", 1), Order: "V", SVG: `<svg/>`}
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: &node}}, c)

	next := `<svg viewBox="0 0 2 2"></svg>`
	ch := mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: tid("n", 2), SVG: sp(next)}}, c)
	want := tid("n", 2) + "|" + next
	got, _ := d.Node(tid("n", 2))
	if got.SVG != want {
		t.Fatalf("stored SVG = %q, want the canonical return %q: set_props must not store markup no sanitiser saw", got.SVG, want)
	}
	if ch.Ops[0].SVG == nil || *ch.Ops[0].SVG != want {
		t.Fatalf("Change.Ops[0].SVG = %v, want the canonical form", ch.Ops[0].SVG)
	}

	bad := &tagChecker{err: errors.New("dialect: a script is not markup")}
	if _, err := d.Apply([]Op{{Kind: OpSetProps, NodeID: tid("n", 2), SVG: sp(`<svg><script/></svg>`)}},
		testAuthor(), testAt, "evil", bad); err == nil {
		t.Fatal("set_props stored SVG the checker refused")
	} else if oe := opErr(t, err); oe.Code != CodeInvalid {
		t.Fatalf("Code = %q, want %q", oe.Code, CodeInvalid)
	}
	if got, _ := d.Node(tid("n", 2)); got.SVG != want {
		t.Errorf("SVG = %q, want the refused set_props to have changed nothing", got.SVG)
	}
}

func TestChange_contentChanged(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}}, okChecker{})

	thread := Thread{
		ID: tid("th", 1), NodeID: tid("n", 2), Status: ThreadOpen, CreatedAt: testAt, CreatedBy: testAuthor(),
		Messages: []Message{{ID: tid("msg", 1), Text: "tighten this", At: testAt, Author: testAuthor()}},
	}
	ch := mustApply(t, d, []Op{{Kind: OpCreateThread, Thread: &thread}}, okChecker{})
	if ch.ContentChanged {
		t.Fatal("a batch of only comments reports a content change: it would trip 'changed after approval'")
	}
	if len(ch.Threads) != 1 || ch.Threads[0] != tid("th", 1) {
		t.Errorf("Change.Threads = %v, want the new thread", ch.Threads)
	}

	thread2 := thread
	thread2.ID = tid("th", 2)
	thread2.Messages = []Message{{ID: tid("msg", 2), Text: "and this", At: testAt, Author: testAuthor()}}
	ch = mustApply(t, d, []Op{
		{Kind: OpCreateThread, Thread: &thread2},
		{Kind: OpSetProps, NodeID: tid("n", 2), Styles: map[string]*string{"color": sp("red")}},
	}, okChecker{})
	if !ch.ContentChanged {
		t.Fatal("a batch with a set_props reports no content change")
	}
}

func TestChange_artboards(t *testing.T) {
	d := seeded(t)
	second := Node{ID: tid("n", 2), Kind: KindArtboard, Tag: "div", ParentID: tid("pg", 1), Order: "b",
		Styles: artboardStyles()}
	mustApply(t, d, []Op{
		{Kind: OpCreateNode, Node: &second},
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 1), "V"))},
	}, okChecker{})

	ch := mustApply(t, d, []Op{{Kind: OpMove, NodeID: tid("n", 3), ParentID: sp(tid("n", 2)), Order: sp("V")}}, okChecker{})
	want := []string{tid("n", 1), tid("n", 2)}
	if !reflect.DeepEqual(ch.Artboards, want) {
		t.Fatalf("Change.Artboards = %v, want both the source and the destination %v", ch.Artboards, want)
	}
}

func TestPrepare_doesNotMutate(t *testing.T) {
	d := seeded(t)
	before := d.Document()
	rev := d.Revision()

	p1, err := d.Prepare([]Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
		testAuthor(), testAt, "one", okChecker{})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if d.Revision() != rev || !reflect.DeepEqual(d.Document(), before) {
		t.Fatal("Prepare wrote through: a failed disk append would leave memory ahead of disk")
	}

	p2, err := d.Prepare([]Op{{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 1), "b"))}},
		testAuthor(), testAt, "two", okChecker{})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	p1.Commit()
	if d.Revision() != rev+1 {
		t.Fatalf("Revision() = %d, want %d after Commit", d.Revision(), rev+1)
	}
	if _, ok := d.Node(tid("n", 2)); !ok {
		t.Fatal("Commit did not apply the prepared batch")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("committing a batch prepared against an older revision did not panic")
		}
	}()
	p2.Commit()
}

func TestApply_nodeCountLimit(t *testing.T) {
	d := seeded(t)
	// Pad the node index to the bound without paying for 50,000 valid nodes:
	// Apply counts what the canvas holds, and that is what the limit is about.
	for i := len(d.nodes); i < MaxNodes; i++ {
		d.nodes[tid("n", 900000+i)] = Node{ID: tid("n", 900000+i)}
	}
	_, err := d.Apply([]Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
		testAuthor(), testAt, "one too many", okChecker{})
	if oe := opErr(t, err); oe.Code != CodeLimit {
		t.Fatalf("Code = %q, want %q at %d nodes", oe.Code, CodeLimit, MaxNodes)
	}
}

// rule is one cell of the plan's refusal table.
type rule struct {
	name    string
	pre     []Op // applied first, and expected to succeed
	ops     []Op
	code    string
	checker Checker
}

func runes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// chain is n frames, each the child of the last, starting under the artboard.
func chain(n int) []Op {
	ops := make([]Op, 0, n)
	parent := tid("n", 1)
	for i := 0; i < n; i++ {
		id := 200 + i
		ops = append(ops, Op{Kind: OpCreateNode, Node: ptr(frame(id, parent, "V"))})
		parent = tid("n", id)
	}
	return ops
}

func oneThread(id, node int, msg string) *Thread {
	return &Thread{
		ID: tid("th", id), NodeID: tid("n", node), Status: ThreadOpen, CreatedAt: testAt, CreatedBy: testAuthor(),
		Messages: []Message{{ID: tid("msg", id), Text: msg, At: testAt, Author: testAuthor()}},
	}
}

func oneReview(id int, artboards []string, note string) *ReviewRequest {
	return &ReviewRequest{
		ID: tid("rv", id), ArtboardIDs: artboards, Note: note, Revision: 1,
		At: testAt, Author: agentAuthor(), State: ReviewOpen,
	}
}

func TestApply_rules(t *testing.T) {
	for _, r := range rulesTable() {
		t.Run(r.name, func(t *testing.T) {
			d := seeded(t)
			c := r.checker
			if c == nil {
				c = okChecker{}
			}
			if len(r.pre) > 0 {
				mustApply(t, d, r.pre, okChecker{})
			}
			before := d.Document()
			_, err := d.Apply(r.ops, testAuthor(), testAt, "rule", c)
			if err == nil {
				t.Fatalf("want a %s refusal, the batch was applied", r.code)
			}
			if oe := opErr(t, err); oe.Code != r.code {
				t.Fatalf("Code = %q, want %q (%v)", oe.Code, r.code, err)
			}
			if !reflect.DeepEqual(d.Document(), before) {
				t.Error("a refused batch changed the document")
			}
		})
	}
}

func rulesTable() []rule {
	out := batchRules()
	out = append(out, pageRules()...)
	out = append(out, nodeRules()...)
	out = append(out, moveRules()...)
	out = append(out, collectionRules()...)
	return out
}

func batchRules() []rule {
	tooMany := make([]Op, MaxBatchOps+1)
	for i := range tooMany {
		tooMany[i] = Op{Kind: OpReviewMark, BriefKey: "b"}
	}
	return []rule{
		{name: "empty batch", ops: nil, code: CodeEmpty},
		{name: "over the batch bound", ops: tooMany, code: CodeLimit},
		{name: "unknown kind", ops: []Op{{Kind: "teleport"}}, code: CodeInvalid},
	}
}

func pageRules() []rule {
	return []rule{
		{name: "create_page id invalid", code: CodeInvalid,
			ops: []Op{{Kind: OpCreatePage, Page: &Page{ID: "pg-nope", Name: "n", Order: "V"}}}},
		{name: "create_page id taken", code: CodeExists,
			ops: []Op{{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 1), Name: "n", Order: "b"}}}},
		{name: "create_page order invalid", code: CodeInvalid,
			ops: []Op{{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: "n", Order: "b0"}}}},
		{name: "create_page order collides", code: CodeExists,
			ops: []Op{{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: "n", Order: "V"}}}},
		{name: "create_page name too long", code: CodeInvalid,
			ops: []Op{{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: runes(MaxNameRunes + 1), Order: "b"}}}},
		{name: "update_page missing", code: CodeNotFound,
			ops: []Op{{Kind: OpUpdatePage, PageID: tid("pg", 9), Name: sp("x")}}},
		{name: "update_page order collides", code: CodeExists,
			pre: []Op{{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: "two", Order: "b"}}},
			ops: []Op{{Kind: OpUpdatePage, PageID: tid("pg", 2), Order: sp("V")}}},
		{name: "update_page name too long", code: CodeInvalid,
			ops: []Op{{Kind: OpUpdatePage, PageID: tid("pg", 1), Name: sp(runes(MaxNameRunes + 1))}}},
		{name: "delete_page missing", code: CodeNotFound,
			ops: []Op{{Kind: OpDeletePage, PageID: tid("pg", 9)}}},
		{name: "delete_page the last one", code: CodeInvalid,
			ops: []Op{{Kind: OpDeletePage, PageID: tid("pg", 1)}}},
	}
}

func assetRef(n int) string { return fmt.Sprintf("asset:sha256-%064x", n) }

func nodeRules() []rule {
	styled := frame(2, tid("n", 1), "V")
	styled.Styles = map[string]string{"margin": "4px"}
	img := Node{ID: tid("n", 2), Kind: KindImage, Tag: "img", ParentID: tid("n", 1), Order: "V", Asset: assetRef(7)}
	svg := Node{ID: tid("n", 2), Kind: KindSVG, Tag: "svg", ParentID: tid("n", 1), Order: "V", SVG: "<svg/>"}
	unsized := Node{ID: tid("n", 3), Kind: KindArtboard, Tag: "div", ParentID: tid("pg", 1), Order: "b",
		Styles: map[string]string{"height": "900px"}}
	badTag := frame(2, tid("n", 1), "V")
	badTag.Tag = "svg"

	return []rule{
		{name: "create_node id invalid", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: &Node{ID: "n-nothex", Kind: KindFrame, Tag: "div",
				ParentID: tid("n", 1), Order: "V"}}}},
		{name: "create_node id taken", code: CodeExists,
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(1, tid("n", 1), "V"))}}},
		{name: "create_node Validate fails", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: &badTag}}},
		{name: "create_node parent missing", code: CodeNotFound,
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 9), "V"))}}},
		{name: "create_node artboard not on a page", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: &Node{ID: tid("n", 2), Kind: KindArtboard, Tag: "div",
				ParentID: tid("n", 1), Order: "V", Styles: artboardStyles()}}}},
		{name: "create_node non-artboard on a page", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("pg", 1), "b"))}}},
		{name: "create_node under a text node", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(text(2, tid("n", 1), "V", "hi"))}},
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))}}},
		{name: "create_node under an input", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: &Node{ID: tid("n", 2), Kind: KindFrame, Tag: "input",
				ParentID: tid("n", 1), Order: "V"}}},
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))}}},
		{name: "create_node order collides", code: CodeExists,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 1), "V"))}}},
		{name: "create_node too deep", code: CodeLimit, ops: chain(MaxDepth)},
		{name: "create_node style refused", code: CodeStyle, checker: strictChecker{},
			ops: []Op{{Kind: OpCreateNode, Node: &styled}}},
		{name: "create_node unregistered asset", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: &img}}},
		{name: "create_node svg refused", code: CodeInvalid, checker: &tagChecker{err: errors.New("no")},
			ops: []Op{{Kind: OpCreateNode, Node: &svg}}},
		{name: "create_node artboard without width", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateNode, Node: &unsized}}},

		{name: "set_props node missing", code: CodeNotFound,
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 9), Name: sp("x")}}},
		{name: "set_props text on a frame", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 2), Text: sp("no")}}},
		{name: "set_props tag not allowed", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 2), Tag: sp("svg")}}},
		{name: "set_props input with children", code: CodeInvalid,
			pre: []Op{
				{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
				{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))}},
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 2), Tag: sp("input")}}},
		{name: "set_props style refused", code: CodeStyle, checker: strictChecker{},
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 2), Styles: map[string]*string{"grid": sp("1fr")}}}},
		{name: "set_props place on a non-artboard", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpSetProps, NodeID: tid("n", 2), X: ip(10)}}},
	}
}

func moveRules() []rule {
	two := []Op{
		{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 1), "b"))},
	}
	return []rule{
		{name: "move node missing", code: CodeNotFound,
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 9), ParentID: sp(tid("n", 1)), Order: sp("b")}}},
		{name: "move parent missing", code: CodeNotFound, pre: two,
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 2), ParentID: sp(tid("n", 9)), Order: sp("V")}}},
		{name: "move under a leaf", code: CodeInvalid,
			pre: []Op{
				{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
				{Kind: OpCreateNode, Node: ptr(text(3, tid("n", 1), "b", "hi"))}},
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 2), ParentID: sp(tid("n", 3)), Order: sp("V")}}},
		{name: "move order collides", code: CodeExists, pre: two,
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 3), ParentID: sp(tid("n", 1)), Order: sp("V")}}},
		{name: "move an artboard off its page", code: CodeInvalid,
			pre: append(two, Op{Kind: OpCreateNode, Node: &Node{ID: tid("n", 10), Kind: KindArtboard,
				Tag: "div", ParentID: tid("pg", 1), Order: "b", Styles: artboardStyles()}}),
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 10), ParentID: sp(tid("n", 2)), Order: sp("V")}}},
		{name: "move too deep", code: CodeLimit,
			pre: append(chain(MaxDepth-1), Op{Kind: OpCreateNode, Node: ptr(frame(300, tid("n", 1), "b"))}),
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 300), ParentID: sp(tid("n", 200+MaxDepth-2)), Order: sp("V")}}},
		{name: "move order invalid", code: CodeInvalid, pre: two,
			ops: []Op{{Kind: OpMove, NodeID: tid("n", 2), ParentID: sp(tid("n", 3)), Order: sp("b0")}}},
		{name: "delete_node missing", code: CodeNotFound,
			ops: []Op{{Kind: OpDeleteNode, NodeID: tid("n", 9)}}},
	}
}

func collectionRules() []rule {
	asset := func(a Asset) []Op { return []Op{{Kind: OpAddAsset, AssetRecord: &a}} }
	thread := []Op{{Kind: OpCreateThread, Thread: oneThread(1, 1, "look")}}
	review := []Op{{Kind: OpRequestReview, Review: oneReview(1, []string{tid("n", 1)}, "ready")}}

	return []rule{
		{name: "set_token bad name", code: CodeInvalid,
			ops: []Op{{Kind: OpSetToken, Token: &Token{Name: "Brand", Type: TokenColor, Value: "#fff"}}}},
		{name: "set_token unknown type", code: CodeInvalid,
			ops: []Op{{Kind: OpSetToken, Token: &Token{Name: "brand", Type: "vibe", Value: "#fff"}}}},
		{name: "set_token value refused", code: CodeInvalid, checker: strictChecker{},
			ops: []Op{{Kind: OpSetToken, Token: &Token{Name: "brand", Type: TokenColor, Value: "bad"}}}},
		{name: "delete_token missing", code: CodeNotFound,
			ops: []Op{{Kind: OpDeleteToken, TokenName: "brand"}}},

		{name: "add_asset ref malformed", code: CodeInvalid,
			ops: asset(Asset{Ref: "asset:sha256-xyz", MediaType: "image/png", Size: 1})},
		{name: "add_asset media type", code: CodeInvalid,
			ops: asset(Asset{Ref: assetRef(2), MediaType: "application/pdf", Size: 1})},
		{name: "add_asset too big", code: CodeLimit,
			ops: asset(Asset{Ref: assetRef(3), MediaType: "image/png", Size: MaxAssetBytes + 1})},

		{name: "create_thread id invalid", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateThread, Thread: &Thread{ID: "th-bad", NodeID: tid("n", 1),
				Status: ThreadOpen, CreatedAt: testAt, CreatedBy: testAuthor(),
				Messages: []Message{{ID: tid("msg", 1), Text: "x", At: testAt, Author: testAuthor()}}}}},
		},
		{name: "create_thread id taken", code: CodeExists, pre: thread, ops: thread},
		{name: "create_thread anchor missing", code: CodeNotFound,
			ops: []Op{{Kind: OpCreateThread, Thread: oneThread(1, 9, "look")}}},
		{name: "create_thread not one message", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateThread, Thread: &Thread{ID: tid("th", 1), NodeID: tid("n", 1),
				Status: ThreadOpen, CreatedAt: testAt, CreatedBy: testAuthor()}}}},
		{name: "create_thread message too long", code: CodeInvalid,
			ops: []Op{{Kind: OpCreateThread, Thread: oneThread(1, 1, runes(MaxMessageRunes+1))}}},
		{name: "add_message thread missing", code: CodeNotFound,
			ops: []Op{{Kind: OpAddMessage, ThreadID: tid("th", 9),
				Message: &Message{ID: tid("msg", 2), Text: "x", At: testAt, Author: testAuthor()}}}},
		{name: "add_message id taken", code: CodeExists, pre: thread,
			ops: []Op{{Kind: OpAddMessage, ThreadID: tid("th", 1),
				Message: &Message{ID: tid("msg", 1), Text: "x", At: testAt, Author: testAuthor()}}}},
		{name: "set_thread_status thread missing", code: CodeNotFound,
			ops: []Op{{Kind: OpSetThreadStatus, ThreadID: tid("th", 9), Status: ptr(ThreadResolved)}}},

		{name: "request_review id invalid", code: CodeInvalid,
			ops: []Op{{Kind: OpRequestReview, Review: &ReviewRequest{ID: "rv-bad",
				ArtboardIDs: []string{tid("n", 1)}, Revision: 1, At: testAt, Author: agentAuthor(), State: ReviewOpen}}}},
		{name: "request_review id taken", code: CodeExists, pre: review, ops: review},
		{name: "request_review not an artboard", code: CodeInvalid,
			pre: []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}},
			ops: []Op{{Kind: OpRequestReview, Review: oneReview(1, []string{tid("n", 2)}, "ready")}}},
		{name: "request_review note too long", code: CodeInvalid,
			ops: []Op{{Kind: OpRequestReview, Review: oneReview(1, []string{tid("n", 1)}, runes(MaxNoteRunes+1))}}},
		{name: "close_review missing", code: CodeNotFound,
			ops: []Op{{Kind: OpCloseReview, ReviewID: tid("rv", 9)}}},
		{name: "close_review already closed", code: CodeInvalid,
			pre: append(review, Op{Kind: OpCloseReview, ReviewID: tid("rv", 1)}),
			ops: []Op{{Kind: OpCloseReview, ReviewID: tid("rv", 1)}}},

		{name: "review_mark empty key", code: CodeInvalid, ops: []Op{{Kind: OpReviewMark}}},
		{name: "review_mark key too long", code: CodeInvalid,
			ops: []Op{{Kind: OpReviewMark, BriefKey: runes(MaxBriefKeyBytes + 1)}}},
	}
}

func TestApply_addAssetIdempotent(t *testing.T) {
	d := seeded(t)
	a := Asset{Ref: assetRef(1), MediaType: "image/png", Size: 1024, Width: 32, Height: 32}
	ch := mustApply(t, d, []Op{{Kind: OpAddAsset, AssetRecord: &a}}, okChecker{})
	if len(ch.Assets) != 1 || ch.Assets[0] != a.Ref {
		t.Fatalf("Change.Assets = %v, want the new ref", ch.Assets)
	}

	again := a
	again.Size = 2048
	if _, err := d.Apply([]Op{{Kind: OpAddAsset, AssetRecord: &again}}, testAuthor(), testAt, "again", okChecker{}); err != nil {
		t.Fatalf("a second add_asset of the same ref is a no-op, not a refusal: %v", err)
	}
	got, _ := d.Asset(a.Ref)
	if got.Size != 1024 {
		t.Fatalf("Size = %d, want the first record kept: a ref is its bytes", got.Size)
	}
}

func TestApply_imageAssetMustBeRegistered(t *testing.T) {
	d := seeded(t)
	a := Asset{Ref: assetRef(1), MediaType: "image/png", Size: 1024}
	img := Node{ID: tid("n", 2), Kind: KindImage, Tag: "img", ParentID: tid("n", 1), Order: "V", Asset: a.Ref}
	mustApply(t, d, []Op{{Kind: OpAddAsset, AssetRecord: &a}, {Kind: OpCreateNode, Node: &img}}, okChecker{})

	_, err := d.Apply([]Op{{Kind: OpSetProps, NodeID: tid("n", 2), Asset: sp(assetRef(2))}},
		testAuthor(), testAt, "swap", okChecker{})
	if oe := opErr(t, err); oe.Code != CodeInvalid {
		t.Fatalf("Code = %q, want %q: set_props may not point an image at bytes the canvas does not hold", oe.Code, CodeInvalid)
	}
}
