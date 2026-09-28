package doc

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// content is everything the inverse must restore: the pages, the nodes' own
// fields, and the tokens. Revision and EditedBy are provenance, not content,
// and threads, reviews and assets are never undone.
func content(d *Doc) Document {
	out := d.Document()
	out.Revision = 0
	out.Threads = nil
	out.Reviews = nil
	out.Assets = nil
	for i := range out.Nodes {
		out.Nodes[i].Revision = 0
		out.Nodes[i].EditedBy = Author{}
	}
	return out
}

func TestInverse_deleteSubtreeOrder(t *testing.T) {
	d := seeded(t)
	ops := []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}}
	for i := 0; i < 12; i++ {
		ops = append(ops, Op{Kind: OpCreateNode, Node: ptr(text(10+i, tid("n", 2), orderKey(i), "row"))})
	}
	mustApply(t, d, ops, okChecker{})

	want := d.Children(tid("n", 2))
	if len(want) != 12 {
		t.Fatalf("children = %d, want 12", len(want))
	}

	ch := mustApply(t, d, []Op{{Kind: OpDeleteNode, NodeID: tid("n", 2)}}, okChecker{})
	if _, ok := d.Node(tid("n", 12)); ok {
		t.Fatal("delete_node left a descendant behind")
	}
	if len(ch.Deleted) != 13 {
		t.Fatalf("Change.Deleted = %d ids, want the frame and its 12 children", len(ch.Deleted))
	}

	mustApply(t, d, ch.Inverse, okChecker{})
	got := d.Children(tid("n", 2))
	if len(got) != 12 {
		t.Fatalf("children after the inverse = %d, want 12", len(got))
	}
	for i := range got {
		if got[i].ID != want[i].ID || got[i].Order != want[i].Order {
			t.Fatalf("child %d = %s at %q, want %s at %q", i, got[i].ID, got[i].Order, want[i].ID, want[i].Order)
		}
	}
}

func TestInverse_setPropsNamesOnlyWhatItSet(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))}}, okChecker{})
	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: tid("n", 2),
		Styles: map[string]*string{"color": sp("red"), "padding": sp("8px")}}}, okChecker{})

	before := content(d)
	ch := mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: tid("n", 2),
		Styles: map[string]*string{"color": sp("blue"), "gap": sp("4px"), "padding": nil},
		Name:   sp("Card")}}, okChecker{})

	if len(ch.Inverse) != 1 || ch.Inverse[0].Kind != OpSetProps {
		t.Fatalf("Inverse = %+v, want one set_props", ch.Inverse)
	}
	inv := ch.Inverse[0]
	if len(inv.Styles) != 3 {
		t.Fatalf("the inverse names %d style keys, want exactly the 3 the op named", len(inv.Styles))
	}
	if inv.Styles["gap"] != nil {
		t.Errorf("the inverse of a key that was absent must be a removal, got %q", *inv.Styles["gap"])
	}
	if inv.Styles["color"] == nil || *inv.Styles["color"] != "red" {
		t.Errorf("the inverse of colour = %v, want red", inv.Styles["color"])
	}
	if inv.Styles["padding"] == nil || *inv.Styles["padding"] != "8px" {
		t.Errorf("the inverse of a removal = %v, want the old value", inv.Styles["padding"])
	}
	if inv.Name == nil || *inv.Name != "" {
		t.Errorf("the inverse of Name = %v, want the old empty name", inv.Name)
	}

	mustApply(t, d, ch.Inverse, okChecker{})
	if !reflect.DeepEqual(content(d), before) {
		t.Fatal("the inverse did not restore the node")
	}
}

func TestInverse_deletePageRestoresItsArtboards(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{
		{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: "Two", Order: "b"}},
		{Kind: OpCreateNode, Node: &Node{ID: tid("n", 2), Kind: KindArtboard, Tag: "div",
			ParentID: tid("pg", 2), Order: "V", Styles: artboardStyles()}},
		{Kind: OpCreateNode, Node: ptr(frame(3, tid("n", 2), "V"))},
		{Kind: OpCreateNode, Node: ptr(text(4, tid("n", 3), "V", "hello"))},
	}, okChecker{})

	before := content(d)
	ch := mustApply(t, d, []Op{{Kind: OpDeletePage, PageID: tid("pg", 2)}}, okChecker{})
	if _, ok := d.Node(tid("n", 4)); ok {
		t.Fatal("delete_page left a node of one of its artboards behind")
	}
	if len(ch.Deleted) != 3 {
		t.Fatalf("Change.Deleted = %v, want the artboard and its two descendants", ch.Deleted)
	}

	mustApply(t, d, ch.Inverse, okChecker{})
	if !reflect.DeepEqual(content(d), before) {
		t.Fatal("the inverse of delete_page did not restore the page and its artboards")
	}
}

func TestInverse_tokens(t *testing.T) {
	d := seeded(t)
	ch := mustApply(t, d, []Op{{Kind: OpSetToken, Token: &Token{Name: "brand", Type: TokenColor, Value: "#0af"}}}, okChecker{})
	if len(ch.Inverse) != 1 || ch.Inverse[0].Kind != OpDeleteToken || ch.Inverse[0].TokenName != "brand" {
		t.Fatalf("Inverse = %+v, want a delete_token: the token did not exist before", ch.Inverse)
	}
	if !reflect.DeepEqual(ch.Tokens, []string{"brand"}) {
		t.Errorf("Change.Tokens = %v, want [brand]", ch.Tokens)
	}

	ch = mustApply(t, d, []Op{{Kind: OpSetToken, Token: &Token{Name: "brand", Type: TokenColor, Value: "#f0a"}}}, okChecker{})
	if len(ch.Inverse) != 1 || ch.Inverse[0].Kind != OpSetToken || ch.Inverse[0].Token.Value != "#0af" {
		t.Fatalf("Inverse = %+v, want a set_token back to #0af", ch.Inverse)
	}

	ch = mustApply(t, d, []Op{{Kind: OpDeleteToken, TokenName: "brand"}}, okChecker{})
	if len(ch.Inverse) != 1 || ch.Inverse[0].Kind != OpSetToken || ch.Inverse[0].Token.Value != "#f0a" {
		t.Fatalf("Inverse = %+v, want a set_token of the deleted value", ch.Inverse)
	}
}

func TestInverse_notContentHasNone(t *testing.T) {
	d := seeded(t)
	a := Asset{Ref: assetRef(5), MediaType: "image/webp", Size: 64}
	ch := mustApply(t, d, []Op{
		{Kind: OpAddAsset, AssetRecord: &a},
		{Kind: OpCreateThread, Thread: oneThread(1, 1, "hi")},
		{Kind: OpRequestReview, Review: oneReview(1, []string{tid("n", 1)}, "ready")},
		{Kind: OpReviewMark, BriefKey: "brief/D3"},
	}, okChecker{})
	if len(ch.Inverse) != 0 {
		t.Fatalf("Inverse = %+v, want none: an asset, a comment, a review and a mark are never undone", ch.Inverse)
	}
	if ch.ContentChanged {
		t.Error("ContentChanged = true for a batch that changed no design content")
	}
	if len(ch.Reviews) != 1 || len(ch.Threads) != 1 || len(ch.Assets) != 1 {
		t.Errorf("Change collections = %v %v %v, want one of each", ch.Reviews, ch.Threads, ch.Assets)
	}
}

func TestInverse_reverseOrder(t *testing.T) {
	d := seeded(t)
	ch := mustApply(t, d, []Op{
		{Kind: OpCreateNode, Node: ptr(frame(2, tid("n", 1), "V"))},
		{Kind: OpCreateNode, Node: ptr(text(3, tid("n", 2), "V", "hi"))},
	}, okChecker{})
	if len(ch.Inverse) != 2 {
		t.Fatalf("Inverse = %+v, want two deletes", ch.Inverse)
	}
	if ch.Inverse[0].NodeID != tid("n", 3) || ch.Inverse[1].NodeID != tid("n", 2) {
		t.Fatalf("Inverse deletes %s then %s, want the child first: the inverse runs the batch backwards",
			ch.Inverse[0].NodeID, ch.Inverse[1].NodeID)
	}
	mustApply(t, d, ch.Inverse, okChecker{})
	if _, ok := d.Node(tid("n", 2)); ok {
		t.Fatal("the inverse left the frame behind")
	}
}

// TestInverse_svgBytesSurviveUndo pins the bytes, not the shape. Every inverse
// Apply builds itself carries markup that already came out of Checker.SVG, so
// a dialect that prefixes ids — the textbook canonicalisation, and what the
// Checker contract says task 07 does — would prefix them a second time and hand
// the human back different bytes than they deleted (review C1).
func TestInverse_svgBytesSurviveUndo(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   Op
	}{
		{"delete_node", Op{Kind: OpDeleteNode, NodeID: tid("n", 3)}},
		{"delete_page", Op{Kind: OpDeletePage, PageID: tid("pg", 2)}},
		{"set_props", Op{Kind: OpSetProps, NodeID: tid("n", 3), SVG: sp(`<svg id="a"/>`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &tagChecker{}
			d := seeded(t)
			mustApply(t, d, []Op{
				{Kind: OpCreatePage, Page: &Page{ID: tid("pg", 2), Name: "Two", Order: "b"}},
				{Kind: OpCreateNode, Node: &Node{ID: tid("n", 2), Kind: KindArtboard, Tag: "div",
					ParentID: tid("pg", 2), Order: "V", Styles: artboardStyles()}},
			}, okChecker{})
			mustApply(t, d, []Op{{Kind: OpCreateNode, Node: &Node{ID: tid("n", 3), Kind: KindSVG,
				Tag: "svg", ParentID: tid("n", 2), Order: "V", SVG: "<svg/>"}}}, c)

			want, _ := d.Node(tid("n", 3))
			ch := mustApply(t, d, []Op{tc.op}, c)
			mustApply(t, d, ch.Inverse, c)

			got, ok := d.Node(tid("n", 3))
			if !ok {
				t.Fatal("the inverse did not restore the svg node")
			}
			if got.SVG != want.SVG {
				t.Fatalf("after the undo the canvas holds %q, want the %q it held before", got.SVG, want.SVG)
			}
		})
	}
}

// TestInverse_setPropsNamesOnlyTheAttrsItSet is the Styles block of
// TestInverse_setPropsNamesOnlyWhatItSet for the other map: an agent's
// aria-label written through set_props must come back off on undo (review I2).
func TestInverse_setPropsNamesOnlyTheAttrsItSet(t *testing.T) {
	d := seeded(t)
	mustApply(t, d, []Op{{Kind: OpCreateNode, Node: &Node{ID: tid("n", 2), Kind: KindFrame,
		Tag: "input", ParentID: tid("n", 1), Order: "V"}}}, okChecker{})
	mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: tid("n", 2),
		Attrs: map[string]*string{"placeholder": sp("Name"), "value": sp("old")}}}, okChecker{})

	before := content(d)
	ch := mustApply(t, d, []Op{{Kind: OpSetProps, NodeID: tid("n", 2),
		Attrs: map[string]*string{"placeholder": sp("Email"), "type": sp("email"), "value": nil}}}, okChecker{})

	if len(ch.Inverse) != 1 || ch.Inverse[0].Kind != OpSetProps {
		t.Fatalf("Inverse = %+v, want one set_props", ch.Inverse)
	}
	inv := ch.Inverse[0]
	if len(inv.Attrs) != 3 {
		t.Fatalf("the inverse names %d attributes, want exactly the 3 the op named", len(inv.Attrs))
	}
	if inv.Attrs["type"] != nil {
		t.Errorf("the inverse of an attribute that was absent must be a removal, got %q", *inv.Attrs["type"])
	}
	if inv.Attrs["placeholder"] == nil || *inv.Attrs["placeholder"] != "Name" {
		t.Errorf("the inverse of placeholder = %v, want Name", inv.Attrs["placeholder"])
	}
	if inv.Attrs["value"] == nil || *inv.Attrs["value"] != "old" {
		t.Errorf("the inverse of a removal = %v, want the old value", inv.Attrs["value"])
	}

	mustApply(t, d, ch.Inverse, okChecker{})
	if !reflect.DeepEqual(content(d), before) {
		t.Fatal("the inverse did not restore the node's attributes")
	}
}

type fuzzer struct {
	rng  *rand.Rand
	next int
}

func (f *fuzzer) id(prefix string) string { f.next++; return tid(prefix, f.next) }

func (f *fuzzer) order() string {
	k := string(OrderAlphabet[f.rng.Intn(len(OrderAlphabet))])
	if f.rng.Intn(2) == 0 {
		k += string(OrderAlphabet[f.rng.Intn(len(OrderAlphabet))])
	}
	if k[len(k)-1] == '0' {
		k = k[:len(k)-1] + "V"
	}
	return k
}

func (f *fuzzer) one(d *Doc) Op {
	pages := d.Pages()
	nodes := d.Document().Nodes
	page := pages[f.rng.Intn(len(pages))]
	var node Node
	if len(nodes) > 0 {
		node = nodes[f.rng.Intn(len(nodes))]
	}
	style := []string{"color", "padding", "gap"}[f.rng.Intn(3)]

	switch f.rng.Intn(10) {
	case 0:
		return Op{Kind: OpCreateNode, Node: &Node{ID: f.id("n"), Kind: KindArtboard, Tag: "div",
			ParentID: page.ID, Order: f.order(), Styles: artboardStyles()}}
	case 1, 2:
		n := Node{ID: f.id("n"), Kind: KindFrame, Tag: "div", ParentID: node.ID, Order: f.order()}
		switch f.rng.Intn(4) { // every kind, so the leaf kinds are undone too
		case 1:
			n.Kind, n.Tag, n.Text = KindText, "p", "lorem"
		case 2:
			n.Kind, n.Tag, n.SVG = KindSVG, "svg", "<svg/>"
		case 3:
			n.Kind, n.Tag, n.Asset, n.Alt = KindImage, "img", fuzzAsset, "a picture"
		}
		return Op{Kind: OpCreateNode, Node: &n}
	case 3, 4:
		op := Op{Kind: OpSetProps, NodeID: node.ID, Styles: map[string]*string{}}
		if f.rng.Intn(3) == 0 {
			op.Styles[style] = nil
		} else {
			op.Styles[style] = sp([]string{"red", "4px", "8px"}[f.rng.Intn(3)])
		}
		if f.rng.Intn(2) == 0 {
			op.Name = sp([]string{"", "Card", "Row"}[f.rng.Intn(3)])
		}
		if f.rng.Intn(4) == 0 {
			op.Hidden = bp(f.rng.Intn(2) == 0)
		}
		if f.rng.Intn(4) == 0 {
			op.Locked = bp(f.rng.Intn(2) == 0)
		}
		if f.rng.Intn(4) == 0 {
			op.Attrs = map[string]*string{"aria-label": nil}
			if f.rng.Intn(2) == 0 {
				op.Attrs["aria-label"] = sp([]string{"Close", "Menu"}[f.rng.Intn(2)])
			}
		}
		// The kind-bound fields, one at a time: on the wrong kind they are a
		// refusal, which the loop asserts changes nothing, and on the right one
		// they are the fields the inverse used never to be tried on.
		if f.rng.Intn(3) == 0 {
			switch f.rng.Intn(6) {
			case 0:
				op.Text = sp([]string{"", "lorem ipsum"}[f.rng.Intn(2)])
			case 1:
				op.SVG = sp(`<svg viewBox="0 0 1 1"/>`)
			case 2:
				op.Alt = sp([]string{"", "alt"}[f.rng.Intn(2)])
			case 3:
				op.Asset = sp(fuzzAsset)
			case 4:
				op.Tag = sp([]string{"div", "section", "p"}[f.rng.Intn(3)])
			case 5:
				op.X, op.Y = ip(f.rng.Intn(4000)), ip(f.rng.Intn(4000))
			}
		}
		return op
	case 5:
		return Op{Kind: OpMove, NodeID: node.ID, ParentID: sp(nodes[f.rng.Intn(len(nodes))].ID), Order: sp(f.order())}
	case 6:
		return Op{Kind: OpDeleteNode, NodeID: node.ID}
	case 7:
		return Op{Kind: OpCreatePage, Page: &Page{ID: f.id("pg"), Name: "P", Order: f.order()}}
	case 8:
		if f.rng.Intn(2) == 0 {
			return Op{Kind: OpDeletePage, PageID: page.ID}
		}
		return Op{Kind: OpUpdatePage, PageID: page.ID, Name: sp("Renamed"), Order: sp(f.order())}
	default:
		name := []string{"brand", "ink", "gap"}[f.rng.Intn(3)]
		if f.rng.Intn(3) == 0 {
			return Op{Kind: OpDeleteToken, TokenName: name}
		}
		return Op{Kind: OpSetToken, Token: &Token{Name: name, Type: TokenColor,
			Value: []string{"#0af", "#f0a", "#fff"}[f.rng.Intn(3)]}}
	}
}

// fuzzAsset is the one registered asset the fuzzer's image nodes point at.
var fuzzAsset = assetRef(7)

func TestInverse_restores(t *testing.T) {
	seed := time.Now().UnixNano()
	f := &fuzzer{rng: rand.New(rand.NewSource(seed)), next: 1000}
	d := seeded(t)
	// tagChecker, not okChecker: a dialect that returns its input unchanged is
	// the one dialect that cannot show an inverse being canonicalised twice.
	c := &tagChecker{}
	mustApply(t, d, []Op{{Kind: OpAddAsset,
		AssetRecord: &Asset{Ref: fuzzAsset, MediaType: "image/webp", Size: 64}}}, c)
	applied := 0

	for i := 0; i < 2000; i++ {
		before := content(d)
		ops := make([]Op, 1+f.rng.Intn(3))
		for j := range ops {
			ops[j] = f.one(d)
		}
		ch, err := d.Apply(ops, agentAuthor(), testAt, "fuzz", c)
		if err != nil {
			if !reflect.DeepEqual(content(d), before) {
				t.Fatalf("seed %d, iteration %d: a refused batch changed the document", seed, i)
			}
			continue
		}
		applied++
		if _, err := d.Apply(ch.Inverse, testAuthor(), testAt, "undo", c); err != nil {
			t.Fatalf("seed %d, iteration %d: the inverse of %+v was refused: %v", seed, i, ops, err)
		}
		if got := content(d); !reflect.DeepEqual(got, before) {
			t.Fatalf("seed %d, iteration %d: the inverse of %+v did not restore the content\n got %+v\nwant %+v",
				seed, i, ops, got, before)
		}
	}
	if applied < 200 {
		t.Fatalf("only %d of 2000 batches applied (seed %d): the generator is not exercising Apply", applied, seed)
	}
}
