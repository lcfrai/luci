package doc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	tPage  = "pg-000000000001"
	tAt    = "2026-09-23T10:00:00.000Z"
	tAsset = "asset:sha256-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func tAuthor() Author {
	return Author{Kind: AuthorHuman, UserID: "local", Handle: "the owner"}
}

func tSessionAuthor() Author {
	return Author{Kind: AuthorSession, UserID: "local", SessionID: "sess-1", Handle: "Design agent"}
}

func tNode(id, kind, tag, parentID, order string) Node {
	return Node{
		ID: id, Kind: Kind(kind), Tag: tag, Name: "a layer", ParentID: parentID, Order: order,
		Styles: map[string]string{}, Attrs: map[string]string{}, EditedBy: tAuthor(),
	}
}

func tArtboard(id, order string) Node { return tNode(id, "artboard", "div", tPage, order) }

func tText(id, parentID, order string) Node {
	n := tNode(id, "text", "p", parentID, order)
	n.Text = "hello"
	return n
}

func tImage(id, parentID, order string) Node {
	n := tNode(id, "image", "img", parentID, order)
	n.Asset, n.Alt = tAsset, "a picture"
	return n
}

func tSVG(id, parentID, order string) Node {
	n := tNode(id, "svg", "svg", parentID, order)
	n.SVG = `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
	return n
}

func TestNodeValidate(t *testing.T) {
	mut := func(f func(*Node), base Node) Node { f(&base); return base }
	cases := []struct {
		name string
		node Node
		ok   bool
	}{
		{"artboard", tArtboard("n-000000000001", "V"), true},
		{"frame", tNode("n-000000000002", "frame", "section", "n-000000000001", "V"), true},
		{"text", tText("n-000000000003", "n-000000000001", "V"), true},
		{"image", tImage("n-000000000004", "n-000000000001", "V"), true},
		{"svg", tSVG("n-000000000005", "n-000000000001", "V"), true},
		{"text with svg", mut(func(n *Node) { n.SVG = "<svg/>" }, tText("n-000000000003", "n-000000000001", "V")), false},
		{"image with no asset", mut(func(n *Node) { n.Asset = "" }, tImage("n-000000000004", "n-000000000001", "V")), false},
		{"image with a bad asset ref", mut(func(n *Node) { n.Asset = "asset:sha256-zz" }, tImage("n-000000000004", "n-000000000001", "V")), false},
		{"frame tagged p", mut(func(n *Node) { n.Tag = "p" }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"input with an href", mut(func(n *Node) { n.Tag, n.Attrs = "input", map[string]string{"href": "#"} }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"input with a bad type", mut(func(n *Node) { n.Tag, n.Attrs = "input", map[string]string{"type": "file"} }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"input with a good type", mut(func(n *Node) { n.Tag, n.Attrs = "input", map[string]string{"type": "email"} }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), true},
		{"aria-label on any tag", mut(func(n *Node) { n.Attrs = map[string]string{"aria-label": "Save"} }, tNode("n-000000000002", "frame", "button", "n-000000000001", "V")), true},
		{"121-rune name", mut(func(n *Node) { n.Name = strings.Repeat("é", MaxNameRunes+1) }, tArtboard("n-000000000001", "V")), false},
		{"120-rune name", mut(func(n *Node) { n.Name = strings.Repeat("é", MaxNameRunes) }, tArtboard("n-000000000001", "V")), true},
		{"text over the limit", mut(func(n *Node) { n.Text = strings.Repeat("a", MaxTextRunes+1) }, tText("n-000000000003", "n-000000000001", "V")), false},
		{"alt over the limit", mut(func(n *Node) { n.Alt = strings.Repeat("a", MaxAltRunes+1) }, tImage("n-000000000004", "n-000000000001", "V")), false},
		{"svg over the limit", mut(func(n *Node) { n.SVG = strings.Repeat("a", MaxSVGBytes+1) }, tSVG("n-000000000005", "n-000000000001", "V")), false},
		{"frame with text", mut(func(n *Node) { n.Text = "x" }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"frame with an asset", mut(func(n *Node) { n.Asset = tAsset }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"artboard parented to a node", mut(func(n *Node) { n.ParentID = "n-000000000009" }, tArtboard("n-000000000001", "V")), false},
		{"frame parented to a page", mut(func(n *Node) { n.ParentID = tPage }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"bad id", mut(func(n *Node) { n.ID = "node-1" }, tArtboard("n-000000000001", "V")), false},
		{"unknown kind", mut(func(n *Node) { n.Kind = "group" }, tArtboard("n-000000000001", "V")), false},
		{"order ending in zero", mut(func(n *Node) { n.Order = "V0" }, tArtboard("n-000000000001", "V")), false},
		{"x on a frame", mut(func(n *Node) { n.X = 10 }, tNode("n-000000000002", "frame", "div", "n-000000000001", "V")), false},
		{"x on an artboard", mut(func(n *Node) { n.X, n.Y = 10, -20 }, tArtboard("n-000000000001", "V")), true},
		{"negative revision", mut(func(n *Node) { n.Revision = -1 }, tArtboard("n-000000000001", "V")), false},
		{"human with a session id", mut(func(n *Node) { n.EditedBy.SessionID = "sess-1" }, tArtboard("n-000000000001", "V")), false},
		{"session with no session id", mut(func(n *Node) { n.EditedBy = tSessionAuthor(); n.EditedBy.SessionID = "" }, tArtboard("n-000000000001", "V")), false},
		{"session author", mut(func(n *Node) { n.EditedBy = tSessionAuthor() }, tArtboard("n-000000000001", "V")), true},
	}
	for _, c := range cases {
		err := c.node.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: Validate() = %v, want nil", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: Validate() = nil, want an error", c.name)
		}
	}
}

// tDocument is one small canvas: two pages, an artboard with a text child and
// an image child, a token, an asset, a thread and a review request.
func tDocument() Document {
	board := tArtboard("n-000000000001", "V")
	board.Revision = 2
	return Document{
		V: 1, CanvasID: "cv-00000000000000a1", ProjectID: "p-0001", BranchID: "b-0001",
		Revision: 2, CreatedAt: tAt,
		Pages: []Page{{ID: tPage, Name: "Page 1", Order: "V"}, {ID: "pg-000000000002", Name: "Page 2", Order: "k"}},
		Nodes: []Node{board, tText("n-000000000003", "n-000000000001", "V"), tImage("n-000000000004", "n-000000000001", "k")},
		Tokens: []Token{
			{Name: "brand-ink", Type: TokenColor, Value: "#101828"},
			{Name: "space-2", Type: TokenSpacing, Value: "8px"},
		},
		Assets:  []Asset{{Ref: tAsset, MediaType: "image/png", Size: 12, Width: 4, Height: 4}},
		Threads: []Thread{{ID: "th-000000000001", NodeID: "n-000000000003", Status: ThreadOpen, CreatedAt: tAt, CreatedBy: tAuthor(), Messages: []Message{{ID: "msg-000000000001", Text: "tighten this", At: tAt, Author: tAuthor()}}}},
		Reviews: []ReviewRequest{{ID: "rv-000000000001", ArtboardIDs: []string{"n-000000000001"}, Note: "ready", Revision: 2, At: tAt, Author: tSessionAuthor(), State: ReviewOpen}},
	}
}

func TestDocument_sortedAndStable(t *testing.T) {
	sorted := tDocument()
	shuffled := tDocument()
	shuffled.Pages[0], shuffled.Pages[1] = shuffled.Pages[1], shuffled.Pages[0]
	shuffled.Nodes[0], shuffled.Nodes[2] = shuffled.Nodes[2], shuffled.Nodes[0]
	shuffled.Tokens[0], shuffled.Tokens[1] = shuffled.Tokens[1], shuffled.Tokens[0]

	marshal := func(d Document) []byte {
		doc, err := FromDocument(d)
		if err != nil {
			t.Fatalf("FromDocument: %v", err)
		}
		b, err := json.Marshal(doc.Document())
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		again, err := json.Marshal(doc.Document())
		if err != nil {
			t.Fatalf("Marshal again: %v", err)
		}
		if string(b) != string(again) {
			t.Fatalf("the same Doc marshalled to two different byte strings")
		}
		return b
	}
	if a, b := string(marshal(sorted)), string(marshal(shuffled)); a != b {
		t.Fatalf("insertion order leaked into the JSON:\n%s\n%s", a, b)
	}

	d, err := FromDocument(sorted)
	if err != nil {
		t.Fatalf("FromDocument: %v", err)
	}
	if got := d.Document().Pages; got[0].ID != tPage || got[1].ID != "pg-000000000002" {
		t.Errorf("pages are not by Order: %v", got)
	}
	if kids := d.Children("n-000000000001"); len(kids) != 2 || kids[0].ID != "n-000000000003" {
		t.Errorf("Children is not by Order: %v", kids)
	}
	if boards := d.Artboards(tPage); len(boards) != 1 || boards[0].ID != "n-000000000001" {
		t.Errorf("Artboards(%q) = %v", tPage, boards)
	}
	if sub := d.Subtree("n-000000000001"); len(sub) != 3 || sub[0].ID != "n-000000000001" {
		t.Errorf("Subtree is not pre-order: %v", sub)
	}
	if id, ok := d.ArtboardOf("n-000000000003"); !ok || id != "n-000000000001" {
		t.Errorf("ArtboardOf = %q, %v; want the artboard", id, ok)
	}
	if id, ok := d.ArtboardOf("n-000000000001"); !ok || id != "n-000000000001" {
		t.Errorf("ArtboardOf of an artboard = %q, %v; want itself", id, ok)
	}
	if _, ok := d.ArtboardOf("n-00000000000f"); ok {
		t.Error("ArtboardOf of an unknown node = ok, want not ok")
	}
	if d.Revision() != 2 {
		t.Errorf("Revision() = %d, want 2", d.Revision())
	}
	// Empty collections are [] and empty maps are {}, so two equal documents
	// marshal to equal bytes whatever built them.
	empty, err := json.Marshal(NewDoc("cv-00000000000000a1", "p-0001", "b-0001", tAt).Document())
	if err != nil {
		t.Fatalf("Marshal of a new doc: %v", err)
	}
	for _, field := range []string{`"pages":[]`, `"nodes":[]`, `"tokens":[]`, `"assets":[]`, `"threads":[]`, `"reviews":[]`, `"revision":0`} {
		if !strings.Contains(string(empty), field) {
			t.Errorf("a new canvas marshals without %s: %s", field, empty)
		}
	}
	// A Node handed back is a copy: mutating it must not reach the Doc.
	n, _ := d.Node("n-000000000003")
	n.Styles["color"] = "red"
	if again, _ := d.Node("n-000000000003"); len(again.Styles) != 0 {
		t.Error("Node() returned a node sharing its Styles map with the Doc")
	}
}

func TestFromDocument_rejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Document)
	}{
		{"a parent cycle", func(d *Document) {
			a := tNode("n-00000000000a", "frame", "div", "n-00000000000b", "V")
			b := tNode("n-00000000000b", "frame", "div", "n-00000000000a", "V")
			d.Nodes = append(d.Nodes, a, b)
		}},
		{"a self parent", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-00000000000a", "frame", "div", "n-00000000000a", "V"))
		}},
		{"an artboard whose parent is a node", func(d *Document) {
			n := tArtboard("n-00000000000a", "k")
			n.ParentID = "n-000000000001"
			d.Nodes = append(d.Nodes, n)
		}},
		{"an artboard on an unknown page", func(d *Document) {
			n := tArtboard("n-00000000000a", "k")
			n.ParentID = "pg-0000000000ff"
			d.Nodes = append(d.Nodes, n)
		}},
		{"a child of a text node", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-00000000000a", "frame", "div", "n-000000000003", "V"))
		}},
		{"a child of an image", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-00000000000a", "frame", "div", "n-000000000004", "V"))
		}},
		{"a child of an input", func(d *Document) {
			in := tNode("n-00000000000b", "frame", "input", "n-000000000001", "z")
			d.Nodes = append(d.Nodes, in, tNode("n-00000000000a", "frame", "div", "n-00000000000b", "V"))
		}},
		{"depth 65", func(d *Document) {
			parent := "n-000000000001"
			for i := 0; i < MaxDepth; i++ { // the artboard is depth 1, so this is depth 65
				id := nthNodeID(i)
				order := "V"
				if i == 0 {
					order = "z" // the artboard's existing children hold V and k
				}
				d.Nodes = append(d.Nodes, tNode(id, "frame", "div", parent, order))
				parent = id
			}
		}},
		{"two siblings with one order", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-00000000000a", "frame", "div", "n-000000000001", "V"))
		}},
		{"two pages with one order", func(d *Document) {
			d.Pages = append(d.Pages, Page{ID: "pg-000000000003", Name: "Page 3", Order: "V"})
		}},
		{"a repeated node id", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-000000000003", "frame", "div", "n-000000000001", "z"))
		}},
		{"an unknown parent", func(d *Document) {
			d.Nodes = append(d.Nodes, tNode("n-00000000000a", "frame", "div", "n-0000000000ff", "V"))
		}},
		{"a wrong version", func(d *Document) { d.V = 2 }},
		{"a bad canvas id", func(d *Document) { d.CanvasID = "cv-0000000000a1" }},
		{"a negative revision", func(d *Document) { d.Revision = -1 }},
		{"a node ahead of the canvas", func(d *Document) { d.Nodes[0].Revision = d.Revision + 1 }},
		{"a bad timestamp", func(d *Document) { d.CreatedAt = "2026-09-23 10:00:00" }},
		{"a bad token name", func(d *Document) { d.Tokens[0].Name = "Brand-Ink" }},
		{"a repeated token name", func(d *Document) { d.Tokens[1].Name = d.Tokens[0].Name }},
		{"an unknown media type", func(d *Document) { d.Assets[0].MediaType = "image/gif" }},
		{"an oversized asset", func(d *Document) { d.Assets[0].Size = MaxAssetBytes + 1 }},
		{"a thread with no messages", func(d *Document) { d.Threads[0].Messages = nil }},
		{"an empty message", func(d *Document) { d.Threads[0].Messages[0].Text = "" }},
		{"a review with no artboards", func(d *Document) { d.Reviews[0].ArtboardIDs = nil }},
		{"a note over the limit", func(d *Document) { d.Reviews[0].Note = strings.Repeat("a", MaxNoteRunes+1) }},
	}
	for _, c := range cases {
		d := tDocument()
		c.edit(&d)
		if _, err := FromDocument(d); err == nil {
			t.Errorf("%s: FromDocument() = nil error, want an error", c.name)
		}
	}
	if _, err := FromDocument(tDocument()); err != nil {
		t.Fatalf("the unedited document is rejected: %v", err)
	}
}

// nthNodeID is a distinct node id per i, well away from the fixtures' ids.
func nthNodeID(i int) string { return fmt.Sprintf("n-8%011x", i) }

func TestFromDocument_nodeCap(t *testing.T) {
	d := tDocument()
	d.Nodes = d.Nodes[:1] // the artboard alone
	for i := 0; i < MaxNodes; i++ {
		n := tNode(nthNodeID(i+16), "frame", "div", "n-000000000001", orderAt(i))
		d.Nodes = append(d.Nodes, n)
	}
	if _, err := FromDocument(d); err == nil {
		t.Errorf("FromDocument with %d nodes = nil error, want an error", len(d.Nodes))
	}
	d.Nodes = d.Nodes[:MaxNodes]
	if _, err := FromDocument(d); err != nil {
		t.Errorf("FromDocument with %d nodes = %v, want nil", len(d.Nodes), err)
	}
}

// orderAt is a fixed-width base-62 counter whose last digit is never '0'.
func orderAt(i int) string {
	a := OrderAlphabet
	return string([]byte{a[i/(62*61)], a[(i/61)%62], a[i%61+1]})
}

func TestDoc_lookups(t *testing.T) {
	d, err := FromDocument(tDocument())
	if err != nil {
		t.Fatalf("FromDocument: %v", err)
	}
	if p, ok := d.Page(tPage); !ok || p.Name != "Page 1" {
		t.Errorf("Page(%q) = %v, %v", tPage, p, ok)
	}
	if len(d.Pages()) != 2 || len(d.Tokens()) != 2 || len(d.Threads()) != 1 || len(d.Reviews()) != 1 {
		t.Errorf("counts: %d pages, %d tokens, %d threads, %d reviews", len(d.Pages()), len(d.Tokens()), len(d.Threads()), len(d.Reviews()))
	}
	if tok, ok := d.Token("brand-ink"); !ok || tok.Value != "#101828" {
		t.Errorf("Token(brand-ink) = %v, %v", tok, ok)
	}
	if a, ok := d.Asset(tAsset); !ok || a.MediaType != "image/png" {
		t.Errorf("Asset = %v, %v", a, ok)
	}
	if th, ok := d.Thread("th-000000000001"); !ok || len(th.Messages) != 1 {
		t.Errorf("Thread = %v, %v", th, ok)
	}
	for _, missing := range []func() bool{
		func() bool { _, ok := d.Node("n-0000000000ff"); return ok },
		func() bool { _, ok := d.Page("pg-0000000000ff"); return ok },
		func() bool { _, ok := d.Token("nope"); return ok },
		func() bool { _, ok := d.Asset("asset:sha256-00"); return ok },
		func() bool { _, ok := d.Thread("th-0000000000ff"); return ok },
	} {
		if missing() {
			t.Error("a lookup of something absent reported ok")
		}
	}
	if kids := d.Children("n-0000000000ff"); len(kids) != 0 {
		t.Errorf("Children of an unknown parent = %v", kids)
	}
}

var _ = sha256.Sum256
