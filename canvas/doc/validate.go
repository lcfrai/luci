package doc

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

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

	// MaxAttrValueRunes bounds an HTML attribute's value. The spec gives no
	// number; an aria-label or a placeholder is a short human string, so it
	// takes the same bound as alt text.
	MaxAttrValueRunes = MaxAltRunes
	// MaxTokenNameBytes is the length of ^[a-z][a-z0-9-]{0,62}$.
	MaxTokenNameBytes = 63
)

// TimeFormat is the one spelling of a time in a document: RFC 3339, UTC, to
// the millisecond. One spelling is what makes two equal documents marshal to
// equal bytes and a digest reproducible.
const TimeFormat = "2006-01-02T15:04:05.000Z"

// FormatTime is the only way this package's callers should mint a timestamp.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeFormat) }

// ValidTime reports whether s is exactly one TimeFormat spelling.
func ValidTime(s string) bool {
	if len(s) != len(TimeFormat) || !strings.HasSuffix(s, "Z") {
		return false
	}
	t, err := time.Parse(TimeFormat, s)
	return err == nil && FormatTime(t) == s
}

// Validate checks an author: the kind, and the session id that belongs to it.
func (a Author) Validate() error {
	switch a.Kind {
	case AuthorHuman:
		if a.SessionID != "" {
			return fmt.Errorf("doc: a human author carries a session id %q", a.SessionID)
		}
	case AuthorSession:
		if a.SessionID == "" {
			return fmt.Errorf("doc: a session author carries no session id")
		}
	default:
		return fmt.Errorf("doc: unknown author kind %q", a.Kind)
	}
	if a.UserID == "" {
		return fmt.Errorf("doc: an author carries no user id")
	}
	return nil
}

// attrAllowed reports whether a tag keeps an attribute (AllowedAttrs).
func attrAllowed(tag, name string) bool {
	return slices.Contains(AllowedAttrs["*"], name) || slices.Contains(AllowedAttrs[tag], name)
}

// Validate checks one node alone: id, kind, tag, attrs, the field that belongs
// to its kind and no other, the limits. Style properties are checked by the
// dialect (task 07), not here.
func (n Node) Validate() error {
	if !ValidID("n", n.ID) {
		return fmt.Errorf("doc: %q is not a node id", n.ID)
	}
	tags, known := AllowedTags[n.Kind]
	if !known {
		return fmt.Errorf("doc: node %s: unknown kind %q", n.ID, n.Kind)
	}
	if !slices.Contains(tags, n.Tag) {
		return fmt.Errorf("doc: node %s: a %s may not be a <%s>", n.ID, n.Kind, n.Tag)
	}
	if utf8.RuneCountInString(n.Name) > MaxNameRunes {
		return fmt.Errorf("doc: node %s: the name is over %d runes", n.ID, MaxNameRunes)
	}
	if n.Kind == KindArtboard {
		if !ValidID("pg", n.ParentID) {
			return fmt.Errorf("doc: node %s: an artboard's parent is a page, not %q", n.ID, n.ParentID)
		}
	} else if !ValidID("n", n.ParentID) {
		return fmt.Errorf("doc: node %s: the parent %q is not a node id", n.ID, n.ParentID)
	}
	if !ValidOrder(n.Order) {
		return fmt.Errorf("doc: node %s: %q is not an order key", n.ID, n.Order)
	}
	if err := n.validateKindFields(); err != nil {
		return err
	}
	if n.Kind != KindArtboard && (n.X != 0 || n.Y != 0) {
		return fmt.Errorf("doc: node %s: only an artboard has a place on the page", n.ID)
	}
	if n.Revision < 0 {
		return fmt.Errorf("doc: node %s: revision %d", n.ID, n.Revision)
	}
	if err := n.EditedBy.Validate(); err != nil {
		return fmt.Errorf("doc: node %s: %w", n.ID, err)
	}
	return n.validateAttrs()
}

// validateKindFields is "the field that belongs to its kind and no other".
func (n Node) validateKindFields() error {
	if n.Text != "" && n.Kind != KindText {
		return fmt.Errorf("doc: node %s: a %s carries no text", n.ID, n.Kind)
	}
	if (n.Asset != "" || n.Alt != "") && n.Kind != KindImage {
		return fmt.Errorf("doc: node %s: a %s carries no asset", n.ID, n.Kind)
	}
	if n.SVG != "" && n.Kind != KindSVG {
		return fmt.Errorf("doc: node %s: a %s carries no svg", n.ID, n.Kind)
	}
	switch n.Kind {
	case KindText:
		if utf8.RuneCountInString(n.Text) > MaxTextRunes {
			return fmt.Errorf("doc: node %s: the text is over %d runes", n.ID, MaxTextRunes)
		}
	case KindImage:
		if !ValidAssetRef(n.Asset) {
			return fmt.Errorf("doc: node %s: %q is not an asset ref", n.ID, n.Asset)
		}
		if utf8.RuneCountInString(n.Alt) > MaxAltRunes {
			return fmt.Errorf("doc: node %s: the alt text is over %d runes", n.ID, MaxAltRunes)
		}
	case KindSVG:
		if n.SVG == "" {
			return fmt.Errorf("doc: node %s: an svg node carries no markup", n.ID)
		}
		if len(n.SVG) > MaxSVGBytes {
			return fmt.Errorf("doc: node %s: the markup is over %d bytes", n.ID, MaxSVGBytes)
		}
	}
	return nil
}

// validateAttrs keeps a node to AllowedAttrs, and an input to AllowedInputTypes.
func (n Node) validateAttrs() error {
	for name, value := range n.Attrs {
		if !attrAllowed(n.Tag, name) {
			return fmt.Errorf("doc: node %s: <%s> may not keep the attribute %q", n.ID, n.Tag, name)
		}
		if utf8.RuneCountInString(value) > MaxAttrValueRunes {
			return fmt.Errorf("doc: node %s: the attribute %q is over %d runes", n.ID, name, MaxAttrValueRunes)
		}
		if name == "type" && !slices.Contains(AllowedInputTypes, value) {
			return fmt.Errorf("doc: node %s: %q is not an allowed input type", n.ID, value)
		}
	}
	return nil
}

// AllowedTokenTypes is every TokenType, in the order the spec lists them.
var AllowedTokenTypes = []TokenType{
	TokenColor, TokenRadius, TokenSpacing, TokenContainer, TokenBreakpoint,
	TokenFontFamily, TokenFontWeight, TokenFontSize, TokenLineHeight, TokenLetterSpacing,
}

// ValidTokenName reports whether s matches ^[a-z][a-z0-9-]{0,62}$.
func ValidTokenName(s string) bool {
	if s == "" || len(s) > MaxTokenNameBytes || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// canHoldChildren is which nodes may be a parent: a text, image or svg node
// is a leaf, and a void tag (input) holds nothing whatever its kind allows.
func canHoldChildren(parent Node) error {
	switch parent.Kind {
	case KindText, KindImage, KindSVG:
		return fmt.Errorf("doc: node %s: a %s holds no children", parent.ID, parent.Kind)
	}
	if VoidTag(parent.Tag) {
		return fmt.Errorf("doc: node %s: <%s> is void and holds no children", parent.ID, parent.Tag)
	}
	return nil
}

// checkDepth walks up to the artboard. A chain longer than MaxDepth is either
// too deep or a cycle; both are the same refusal, and both are bounded.
func checkDepth(n Node, nodes map[string]Node) error {
	cur, depth := n, 1
	for cur.Kind != KindArtboard {
		parent, ok := nodes[cur.ParentID]
		if !ok {
			return fmt.Errorf("doc: node %s: unknown parent %q", cur.ID, cur.ParentID)
		}
		cur = parent
		depth++
		if depth > MaxDepth {
			return fmt.Errorf("doc: node %s is deeper than %d, or in a parent cycle", n.ID, MaxDepth)
		}
	}
	return nil
}

// validateDocument checks a whole Document: the header, every page and node
// alone, then the invariants only the whole canvas shows — parents that exist
// and may hold children, one Order per sibling, depth and cycles.
func validateDocument(d Document) error {
	if d.V != DocumentVersion {
		return fmt.Errorf("doc: document version %d, want %d", d.V, DocumentVersion)
	}
	if !ValidCanvasID(d.CanvasID) {
		return fmt.Errorf("doc: %q is not a canvas id", d.CanvasID)
	}
	if d.ProjectID == "" || d.BranchID == "" {
		return fmt.Errorf("doc: canvas %s carries no project or branch", d.CanvasID)
	}
	if d.Revision < 0 {
		return fmt.Errorf("doc: canvas %s: revision %d", d.CanvasID, d.Revision)
	}
	if !ValidTime(d.CreatedAt) {
		return fmt.Errorf("doc: canvas %s: %q is not a %s time", d.CanvasID, d.CreatedAt, TimeFormat)
	}
	if len(d.Nodes) > MaxNodes {
		return fmt.Errorf("doc: canvas %s holds %d nodes, over the %d limit", d.CanvasID, len(d.Nodes), MaxNodes)
	}

	pages := make(map[string]struct{}, len(d.Pages))
	pageOrder := make(map[string]string, len(d.Pages))
	for _, p := range d.Pages {
		if !ValidID("pg", p.ID) {
			return fmt.Errorf("doc: %q is not a page id", p.ID)
		}
		if _, dup := pages[p.ID]; dup {
			return fmt.Errorf("doc: page %s appears twice", p.ID)
		}
		if utf8.RuneCountInString(p.Name) > MaxNameRunes {
			return fmt.Errorf("doc: page %s: the name is over %d runes", p.ID, MaxNameRunes)
		}
		if !ValidOrder(p.Order) {
			return fmt.Errorf("doc: page %s: %q is not an order key", p.ID, p.Order)
		}
		if other, dup := pageOrder[p.Order]; dup {
			return fmt.Errorf("doc: pages %s and %s share the order %q", other, p.ID, p.Order)
		}
		pages[p.ID] = struct{}{}
		pageOrder[p.Order] = p.ID
	}

	nodes := make(map[string]Node, len(d.Nodes))
	for _, n := range d.Nodes {
		if err := n.Validate(); err != nil {
			return err
		}
		if _, dup := nodes[n.ID]; dup {
			return fmt.Errorf("doc: node %s appears twice", n.ID)
		}
		if n.Revision > d.Revision {
			return fmt.Errorf("doc: node %s is at revision %d, ahead of the canvas at %d", n.ID, n.Revision, d.Revision)
		}
		nodes[n.ID] = n
	}

	siblings := make(map[string]string, len(d.Nodes))
	for _, n := range d.Nodes {
		if n.Kind == KindArtboard {
			if _, ok := pages[n.ParentID]; !ok {
				return fmt.Errorf("doc: artboard %s is on the unknown page %q", n.ID, n.ParentID)
			}
		} else {
			parent, ok := nodes[n.ParentID]
			if !ok {
				return fmt.Errorf("doc: node %s: unknown parent %q", n.ID, n.ParentID)
			}
			if err := canHoldChildren(parent); err != nil {
				return fmt.Errorf("%w (the child is %s)", err, n.ID)
			}
		}
		key := n.ParentID + "\x00" + n.Order
		if other, dup := siblings[key]; dup {
			return fmt.Errorf("doc: nodes %s and %s share the order %q under %s", other, n.ID, n.Order, n.ParentID)
		}
		siblings[key] = n.ID
	}
	for _, n := range d.Nodes {
		if err := checkDepth(n, nodes); err != nil {
			return err
		}
	}
	return validateCollections(d)
}

// validateCollections checks the tokens, assets, threads and review requests.
// A thread's anchor is NOT required to exist: a thread outlives the node it
// was left on, and is shown detached.
func validateCollections(d Document) error {
	names := make(map[string]struct{}, len(d.Tokens))
	for _, t := range d.Tokens {
		if !ValidTokenName(t.Name) {
			return fmt.Errorf("doc: %q is not a token name", t.Name)
		}
		if _, dup := names[t.Name]; dup {
			return fmt.Errorf("doc: token %s appears twice", t.Name)
		}
		if !slices.Contains(AllowedTokenTypes, t.Type) {
			return fmt.Errorf("doc: token %s: unknown type %q", t.Name, t.Type)
		}
		if t.Value == "" {
			return fmt.Errorf("doc: token %s carries no value", t.Name)
		}
		names[t.Name] = struct{}{}
	}

	refs := make(map[string]struct{}, len(d.Assets))
	for _, a := range d.Assets {
		if !ValidAssetRef(a.Ref) {
			return fmt.Errorf("doc: %q is not an asset ref", a.Ref)
		}
		if _, dup := refs[a.Ref]; dup {
			return fmt.Errorf("doc: asset %s appears twice", a.Ref)
		}
		if !slices.Contains(AllowedMediaTypes, a.MediaType) {
			return fmt.Errorf("doc: asset %s: unknown media type %q", a.Ref, a.MediaType)
		}
		if a.Size <= 0 || a.Size > MaxAssetBytes {
			return fmt.Errorf("doc: asset %s: %d bytes", a.Ref, a.Size)
		}
		if a.Width < 0 || a.Height < 0 {
			return fmt.Errorf("doc: asset %s: %dx%d", a.Ref, a.Width, a.Height)
		}
		refs[a.Ref] = struct{}{}
	}
	if err := validateThreads(d.Threads); err != nil {
		return err
	}
	return validateReviews(d.Reviews, d.Revision)
}

func validateThreads(threads []Thread) error {
	ids := make(map[string]struct{}, len(threads))
	for _, th := range threads {
		if !ValidID("th", th.ID) {
			return fmt.Errorf("doc: %q is not a thread id", th.ID)
		}
		if _, dup := ids[th.ID]; dup {
			return fmt.Errorf("doc: thread %s appears twice", th.ID)
		}
		ids[th.ID] = struct{}{}
		if !ValidID("n", th.NodeID) {
			return fmt.Errorf("doc: thread %s: %q is not a node id", th.ID, th.NodeID)
		}
		if th.Status != ThreadOpen && th.Status != ThreadResolved {
			return fmt.Errorf("doc: thread %s: unknown status %q", th.ID, th.Status)
		}
		if !ValidTime(th.CreatedAt) {
			return fmt.Errorf("doc: thread %s: %q is not a %s time", th.ID, th.CreatedAt, TimeFormat)
		}
		if err := th.CreatedBy.Validate(); err != nil {
			return fmt.Errorf("doc: thread %s: %w", th.ID, err)
		}
		if len(th.Messages) == 0 {
			return fmt.Errorf("doc: thread %s carries no message", th.ID)
		}
		msgs := make(map[string]struct{}, len(th.Messages))
		for _, m := range th.Messages {
			if !ValidID("msg", m.ID) {
				return fmt.Errorf("doc: thread %s: %q is not a message id", th.ID, m.ID)
			}
			if _, dup := msgs[m.ID]; dup {
				return fmt.Errorf("doc: thread %s: message %s appears twice", th.ID, m.ID)
			}
			msgs[m.ID] = struct{}{}
			if r := utf8.RuneCountInString(m.Text); r == 0 || r > MaxMessageRunes {
				return fmt.Errorf("doc: thread %s: message %s is %d runes", th.ID, m.ID, r)
			}
			if !ValidTime(m.At) {
				return fmt.Errorf("doc: thread %s: message %s: %q is not a %s time", th.ID, m.ID, m.At, TimeFormat)
			}
			if err := m.Author.Validate(); err != nil {
				return fmt.Errorf("doc: thread %s: message %s: %w", th.ID, m.ID, err)
			}
		}
	}
	return nil
}

func validateReviews(reviews []ReviewRequest, revision int) error {
	ids := make(map[string]struct{}, len(reviews))
	for _, rv := range reviews {
		if !ValidID("rv", rv.ID) {
			return fmt.Errorf("doc: %q is not a review request id", rv.ID)
		}
		if _, dup := ids[rv.ID]; dup {
			return fmt.Errorf("doc: review request %s appears twice", rv.ID)
		}
		ids[rv.ID] = struct{}{}
		if len(rv.ArtboardIDs) == 0 {
			return fmt.Errorf("doc: review request %s names no artboard", rv.ID)
		}
		seen := make(map[string]struct{}, len(rv.ArtboardIDs))
		for _, id := range rv.ArtboardIDs {
			if !ValidID("n", id) {
				return fmt.Errorf("doc: review request %s: %q is not a node id", rv.ID, id)
			}
			if _, dup := seen[id]; dup {
				return fmt.Errorf("doc: review request %s names %s twice", rv.ID, id)
			}
			seen[id] = struct{}{}
		}
		if utf8.RuneCountInString(rv.Note) > MaxNoteRunes {
			return fmt.Errorf("doc: review request %s: the note is over %d runes", rv.ID, MaxNoteRunes)
		}
		if rv.Revision < 0 || rv.Revision > revision {
			return fmt.Errorf("doc: review request %s is at revision %d, the canvas at %d", rv.ID, rv.Revision, revision)
		}
		if !ValidTime(rv.At) {
			return fmt.Errorf("doc: review request %s: %q is not a %s time", rv.ID, rv.At, TimeFormat)
		}
		if err := rv.Author.Validate(); err != nil {
			return fmt.Errorf("doc: review request %s: %w", rv.ID, err)
		}
		if rv.State != ReviewOpen && rv.State != ReviewClosed {
			return fmt.Errorf("doc: review request %s: unknown state %q", rv.ID, rv.State)
		}
	}
	return nil
}
