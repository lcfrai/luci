// Package claudemd renders a Discover document as the managed block of the
// CLAUDE.md that agent sessions read at start-up, and merges it into a file a
// person may also own.
//
// The rendered block is DETERMINISTIC: the same document gives the same
// bytes. There is deliberately no timestamp inside it: a timestamp would
// rewrite the file on every materialise, which shows up as a spurious diff in
// the worktree and as noise in review.
//
// Nothing here reads brief text back out of a file as a source. Inspect
// exists for the drift check alone.
package claudemd

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// Markers delimit the block this package owns. Everything outside them
// belongs to the person and is never touched.
const (
	BeginPrefix = "<!-- panacea:discover:begin"
	EndMarker   = "<!-- panacea:discover:end -->"
)

var (
	// A marker with no partner: a begin with no end after it, an end with no
	// begin before it (a stray end, or an end ahead of the begin), or a second
	// end marker. Review C2: a forged early end would otherwise split the block.
	ErrUnterminatedBlock = errors.New("claudemd: a managed-block marker with no partner")
	ErrDuplicateBlock    = errors.New("claudemd: more than one managed block")
	ErrBadMarker         = errors.New("claudemd: begin marker is not brief=<id> rev=<n>")
)

// commentOpener is what every marker starts with; Render never emits it
// from a Document string.
const commentOpener = "<!--"

// neutralised replaces commentOpener inside a Document string.
const neutralised = `<!-\-`

// clean is what every string Render interpolates goes through: each "\r\n",
// then each lone "\r", becomes "\n" (b10 re-review M-5), then each "<!--"
// becomes `<!-\-` (review C2). "<!--" cannot overlap itself and the
// replacement cannot form one with its neighbours, so one pass leaves none.
func clean(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.ReplaceAll(s, commentOpener, neutralised)
}

// oneLine joins the lines of s with one space, dropping empty ones: the
// override note is a single italic line.
func oneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' }), " ")
}

// Render returns the managed block, both markers and a trailing newline.
// Deterministic: the same Document gives the same bytes, and nothing in it
// depends on the clock (a timestamp would rewrite the file on every start).
// Every string it interpolates from doc (headings from prompts, answer and
// criterion text, the replaced project text, the title's branch ref, the
// ids) has each "<!--" replaced by `<!-\-` first. This is defence in depth
// behind task 03's rule 12 (ErrReservedText): a rendered block contains
// exactly two "<!--" sequences, its own two markers, so no content can forge
// either one.
// In the same pass, each "\r\n" and then each lone "\r" in those strings
// becomes "\n" (b10 re-review M-5). Task 03 rule 14 stores only LF, but a
// head the Hub stored unchecked may still hold "\r"; so Render's output holds
// no "\r" at all, which is what makes Merge's CRLF conversion exact.
// A document with no section (every question skipped) still names its key.
func Render(doc discover.Document) string {
	var b strings.Builder
	id := clean(doc.BriefID)
	fmt.Fprintf(&b, "%s brief=%s rev=%d -->\n", BeginPrefix, id, doc.Revision)
	fmt.Fprintf(&b, "# %s\n\n", clean(doc.Title))
	b.WriteString("Written by Discover in Panacea App. Every session started on this branch reads it first.\n")
	b.WriteString("Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.\n")

	if len(doc.Sections) == 0 {
		b.WriteString("\nNo question in this brief has an answer.\n")
	}
	for _, s := range doc.Sections {
		fmt.Fprintf(&b, "\n## %s\n", clean(s.Heading))
		switch s.Origin {
		case discover.OriginProject:
			b.WriteString("_Inherited from the project brief._\n")
		case discover.OriginOverride:
			fmt.Fprintf(&b, "_Set on this branch, replacing the project brief: %s_\n", oneLine(clean(s.Replaced)))
		}
		b.WriteString("\n")
		if len(s.Items) > 0 {
			for _, it := range s.Items {
				fmt.Fprintf(&b, "- %s\n", clean(it.Text))
			}
		} else {
			fmt.Fprintf(&b, "%s\n", clean(s.Body))
		}
	}

	fmt.Fprintf(&b, "\n---\n\nBrief %s, revision %d.", id, doc.Revision)
	if doc.ProjectBriefID != "" {
		fmt.Fprintf(&b, " Inherits project brief %s, revision %d.", clean(doc.ProjectBriefID), doc.ProjectRevision)
	}
	b.WriteString("\n" + EndMarker + "\n")
	return b.String()
}

// locate is the one marker count Merge, Strip and Inspect share. begin is
// the index of BeginPrefix and end is past EndMarker and the "\r\n" or "\n"
// right after it, if there is one; both are -1 when the file holds no marker
// of either kind. Exactly one begin followed later by exactly one end is the
// only other well-formed shape. Two or more begins is ErrDuplicateBlock; any
// other shape is ErrUnterminatedBlock.
func locate(existing string) (begin, end int, err error) {
	begins := strings.Count(existing, BeginPrefix)
	ends := strings.Count(existing, EndMarker)
	switch {
	case begins == 0 && ends == 0:
		return -1, -1, nil
	case begins > 1:
		return -1, -1, ErrDuplicateBlock
	case begins == 1 && ends == 1:
		begin = strings.Index(existing, BeginPrefix)
		e := strings.Index(existing, EndMarker)
		if e < begin {
			break
		}
		end = e + len(EndMarker)
		switch {
		case strings.HasPrefix(existing[end:], "\r\n"):
			end += 2
		case strings.HasPrefix(existing[end:], "\n"):
			end++
		}
		return begin, end, nil
	}
	return -1, -1, ErrUnterminatedBlock
}

// lineEnding is the file's own: "\r\n" when its first line ending is one,
// "\n" otherwise (and for a file with none).
func lineEnding(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 && s[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// trimLineEnding removes one final "\r\n" or "\n".
func trimLineEnding(s string) (string, bool) {
	if strings.HasSuffix(s, "\r\n") {
		return s[:len(s)-2], true
	}
	if strings.HasSuffix(s, "\n") {
		return s[:len(s)-1], true
	}
	return s, false
}

// Merge splices block into existing: appended after a blank line when there
// is no block, replacing the one block when there is, every other byte kept.
// Appending: an empty existing gets the block alone; one that ends in a line
// ending gets one more line ending (the blank line), then the block; one that
// does not gets two, then the block (b10 re-review M-6).
// It first counts every BeginPrefix and every EndMarker in existing. The
// only well-formed shapes are none of either, or exactly one begin followed
// later by exactly one end. Two or more begins is ErrDuplicateBlock; any
// other shape (a begin with no end, an end with no begin before it, an end
// ahead of the begin, two ends) is ErrUnterminatedBlock. Both change
// nothing - existing comes back as it came, with the error: the guess would
// delete somebody's writing, or leave brief text behind as if it were theirs.
//
// Line endings (review M7: Windows first, and an editor may re-save the file
// with CRLF). Every newline this package looks for - after a marker, the one
// Strip removes, the blank line Merge appends after - is matched as \r?\n.
// Merge writes the block, and the blank line before an appended one, in the
// file's own ending: CRLF when existing's first line ending is "\r\n", LF
// otherwise (and for an empty file). For CRLF, Merge first reads block's
// "\r\n" as "\n", then writes every "\n" as "\r\n", so a block that already
// holds CRLF never becomes "\r\r\n" (Render's output holds no "\r"; a caller
// passing a CRLF block is still exact). So Merge then Strip is byte-exact for
// a CRLF file too, and no merge leaves a file of mixed endings it did not
// already have. A lone "\r" is not a line ending here.
func Merge(existing, block string) (string, error) {
	begin, end, err := locate(existing)
	if err != nil {
		return existing, err
	}
	eol := lineEnding(existing)
	block = strings.ReplaceAll(block, "\r\n", "\n")
	if eol == "\r\n" {
		block = strings.ReplaceAll(block, "\n", "\r\n")
	}
	if begin >= 0 {
		return existing[:begin] + block + existing[end:], nil
	}
	switch {
	case existing == "":
		return block, nil
	case strings.HasSuffix(existing, "\n"):
		return existing + eol + block, nil
	default:
		return existing + eol + eol + block, nil
	}
}

// Strip removes the managed block and the one line ending after it, "\n" or
// "\r\n", so no "\r" is left behind. When the block is the last thing in the
// file (nothing follows it but that line ending, if it has one), it also
// removes the blank line
// Merge put before an appended block: the one line ending ("\n" or "\r\n")
// just before BeginPrefix, and only when that line ending is itself preceded
// by another. So "A\n\n<block>\n" becomes "A\n", and "A\r\n\r\n<block>\r\n"
// becomes "A\r\n" (b10 re-review M-6). When anything follows the block, it
// sits between a person's paragraphs and nothing before it is removed:
// "A\n\n<block>\nB\n" becomes "A\n\nB\n" (here <block> ends at the end
// marker; the "\n" after it is the one Strip removes). A file that did not
// end in a line ending before Merge gets one back, because Merge gave "A"
// and "A\n" the same bytes. No marker of either kind is not an error. The
// same failures, by the same count, as Merge, and existing comes back with
// them unchanged.
func Strip(existing string) (string, error) {
	begin, end, err := locate(existing)
	if err != nil {
		return existing, err
	}
	if begin < 0 {
		return existing, nil
	}
	before, after := existing[:begin], existing[end:]
	if after == "" {
		if t, ok := trimLineEnding(before); ok {
			if _, blank := trimLineEnding(t); blank {
				before = t
			}
		}
	}
	return before + after, nil
}

// Marker is what a begin marker names.
type Marker struct {
	BriefID  string
	Revision int
}

// Block is the managed block found in a file.
type Block struct {
	Marker Marker
	Text   string // from BeginPrefix through EndMarker and its line ending, verbatim (a CRLF block keeps its "\r")
}

// Inspect finds the block. ok is false when there is no marker of either
// kind; a stray or early end marker is ErrUnterminatedBlock, by the same
// count as Merge, never an `edited` block. Markers match as in Merge, with
// \r?\n (review M7). The begin marker's line is exactly
// BeginPrefix + " brief=" + id + " rev=" + n + " -->": id passing
// discover.MarkerSafe (non-empty, no white space, no "<" or ">"), n a
// decimal of 1 or more with no sign and no leading zero, fitting an int;
// anything else is ErrBadMarker. On every error Block is
// zero. Its only use is the drift check (contract §3.3 condition 3):
// NOTHING reads brief text back out of a file as a source.
func Inspect(existing string) (b Block, ok bool, err error) {
	begin, end, err := locate(existing)
	if err != nil || begin < 0 {
		return Block{}, false, err
	}
	text := existing[begin:end]
	m, err := parseMarker(text)
	if err != nil {
		return Block{}, false, err
	}
	return Block{Marker: m, Text: text}, true, nil
}

func parseMarker(text string) (Marker, error) {
	line := text
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSuffix(line, "\r")
	rest, ok := strings.CutPrefix(line, BeginPrefix+" brief=")
	if !ok {
		return Marker{}, fmt.Errorf("%w: %q", ErrBadMarker, line)
	}
	rest, ok = strings.CutSuffix(rest, " -->")
	if !ok {
		return Marker{}, fmt.Errorf("%w: %q", ErrBadMarker, line)
	}
	id, rev, ok := strings.Cut(rest, " rev=")
	// The id rule is discover.MarkerSafe, its only copy: Resolve refuses a
	// revision whose id fails it, so every block it renders reads back here,
	// and no copy of the rule can drift from another (review I1).
	if !ok || !discover.MarkerSafe(id) || !decimal(rev) {
		return Marker{}, fmt.Errorf("%w: %q", ErrBadMarker, line)
	}
	n, err := strconv.Atoi(rev)
	if err != nil {
		return Marker{}, fmt.Errorf("%w: %q", ErrBadMarker, line)
	}
	return Marker{BriefID: id, Revision: n}, nil
}

// decimal is [1-9][0-9]*.
func decimal(s string) bool {
	if s == "" || s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// SameText reports whether got and want are the same lines, split as
// DiffLines splits them: on \r?\n, a final line ending ending the last line
// rather than starting another. It is the drift check's only comparison of a
// found block with a fresh render (task 09's current/edited split), so a
// block an editor re-saved with CRLF is current, not edited over every line
// (review M7), and so is one whose end marker lost the file's final line
// ending (review M1): SameText is false exactly when DiffLines, but for its
// 400-line cap, shows a change. Any other byte difference, a lone "\r"
// included, is a difference.
func SameText(got, want string) bool {
	return slices.Equal(splitLines(got), splitLines(want))
}

type DiffOp string

const (
	DiffSame    DiffOp = "same"
	DiffAdded   DiffOp = "added"   // in the file, not in a fresh render
	DiffRemoved DiffOp = "removed" // in a fresh render, not in the file
)

type DiffLine struct {
	Op   DiffOp `json:"op"`
	Text string `json:"text"`
}

// maxDiffLines caps DiffLines' output. maxDiffCells bounds the longest-
// common-subsequence table; past it the changed middle is shown as every
// render line removed, then every file line added - still a true diff, just
// not the shortest - so a huge hand edit cannot make the drift check slow.
const (
	maxDiffLines = 400
	maxDiffCells = 1 << 20
)

// DiffLines is a line diff (longest common subsequence) of want against got,
// capped at 400 lines of output, for the "block edited by hand" view. Lines
// are split on \r?\n and compared, and returned, without their ending, so a
// CRLF block's diff shows only the lines whose text changed (review M7).
// Where a line changed, the render's line comes first (removed), then the
// file's (added). Never nil.
func DiffLines(want, got string) []DiffLine {
	a, b := splitLines(want), splitLines(got)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	out := make([]DiffLine, 0, min(len(a)+len(b), maxDiffLines))
	for _, l := range a[:p] {
		out = append(out, DiffLine{Op: DiffSame, Text: l})
	}
	out = diffMiddle(out, a[p:len(a)-s], b[p:len(b)-s])
	for _, l := range a[len(a)-s:] {
		out = append(out, DiffLine{Op: DiffSame, Text: l})
	}
	if len(out) > maxDiffLines {
		out = out[:maxDiffLines]
	}
	return out
}

// splitLines splits on \r?\n; a final line ending ends the last line and
// does not start another.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diffMiddle appends the LCS diff of x against y to out.
func diffMiddle(out []DiffLine, x, y []string) []DiffLine {
	n, m := len(x), len(y)
	if n == 0 || m == 0 || n*m > maxDiffCells {
		for _, l := range x {
			out = append(out, DiffLine{Op: DiffRemoved, Text: l})
		}
		for _, l := range y {
			out = append(out, DiffLine{Op: DiffAdded, Text: l})
		}
		return out
	}
	// lcs[i*(m+1)+j] is the LCS length of x[i:] and y[j:].
	lcs := make([]int32, (n+1)*(m+1))
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case x[i] == y[j]:
				lcs[i*(m+1)+j] = lcs[(i+1)*(m+1)+j+1] + 1
			default:
				lcs[i*(m+1)+j] = max(lcs[(i+1)*(m+1)+j], lcs[i*(m+1)+j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case x[i] == y[j]:
			out = append(out, DiffLine{Op: DiffSame, Text: x[i]})
			i++
			j++
		case lcs[(i+1)*(m+1)+j] >= lcs[i*(m+1)+j+1]:
			out = append(out, DiffLine{Op: DiffRemoved, Text: x[i]})
			i++
		default:
			out = append(out, DiffLine{Op: DiffAdded, Text: y[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffLine{Op: DiffRemoved, Text: x[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffLine{Op: DiffAdded, Text: y[j]})
	}
	return out
}
