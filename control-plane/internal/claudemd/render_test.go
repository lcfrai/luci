package claudemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// The fact sheet's world (prototype-fact-sheet.md §2-§3), with Hub-shaped
// ids: b-7 is bf-00000000000000b7, b-42 is bf-00000000000000a2.
const (
	projectID = "bf-00000000000000b7"
	branchID  = "bf-00000000000000a2"
	stamp     = "2026-09-24T10:00:00.000Z"
)

func person(k discover.Key, text string) discover.Answer {
	return discover.Answer{Key: k, Text: text, Source: discover.SourceTyped, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

func skip(k discover.Key) discover.Answer {
	return discover.Answer{Key: k, Skipped: true, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

func done(tag string, items ...string) discover.Answer {
	a := person(discover.KeyDone, strings.Join(items, "\n"))
	for i, it := range items {
		a.Criteria = append(a.Criteria, discover.Criterion{ID: fmt.Sprintf("c-%s%07d", tag, i+1), Text: it})
	}
	return a
}

func key(id string, n int) string { return fmt.Sprintf("brief:%s@%d", id, n) }

func complete(answers ...discover.Answer) discover.Content {
	return discover.Content{V: discover.ContentVersion, Status: discover.StatusComplete,
		Questions: discover.BaseQuestions(), Answers: answers}
}

// b7 is b-7@5, the project revision b-42@5 pins.
func b7() *discover.Revision {
	return &discover.Revision{BriefID: projectID, Number: 5, Key: key(projectID, 5), Scope: discover.ScopeProject,
		Content: complete(
			person(discover.KeyPurpose, "Client-facing portal for audit engagements: requests, evidence and status."),
			person(discover.KeyAudience, "KPMG staff broadly"),
			person(discover.KeyConstraints, "Existing client data; the published API"),
			done("b", "Tests pass", "No change to the published API without a version bump"),
			person(discover.KeyOutOfScope, "Billing and invoicing"),
		)}
}

// b42 is b-42@5 on evidence-upload: audience skipped, so it inherits b-7's.
func b42() *discover.Revision {
	c := complete(
		person(discover.KeyPurpose, "Let client users upload evidence files against an open request."),
		skip(discover.KeyAudience),
		person(discover.KeyConstraints, "Existing uploads and their download links"),
		done("a", "Upload works for PDF, XLSX and images", "A virus scan runs before any file is visible", "Tests pass"),
		person(discover.KeyOutOfScope, "Bulk upload; the reviewer side of evidence"),
	)
	c.Inherits = &discover.Inherit{Key: key(projectID, 5), Digest: "sha256:" + strings.Repeat("7", 64)}
	return &discover.Revision{BriefID: branchID, Number: 5, Key: key(branchID, 5), Scope: discover.ScopeBranch,
		BranchRef: "refs/heads/evidence-upload", Content: c}
}

func resolve(t *testing.T, project, branch *discover.Revision) discover.Document {
	t.Helper()
	doc, err := discover.Resolve(project, branch)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return doc
}

func golden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\r") {
		t.Fatalf("testdata/%s holds a \\r: the checkout converted it (testdata/.gitattributes is * -text)", name)
	}
	return string(raw)
}

// branchBlock is Render(Resolve(b-7@5, b-42@5)).
func branchBlock(t *testing.T) string {
	t.Helper()
	return Render(resolve(t, b7(), b42()))
}

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

func TestResolveBranchOverProjectGolden(t *testing.T) {
	if got, want := branchBlock(t), golden(t, "golden-branch.md"); got != want {
		t.Errorf("render differs from golden-branch.md (fix the renderer, never the golden)\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestResolveProjectAloneGolden(t *testing.T) {
	got := Render(resolve(t, b7(), nil))
	if want := golden(t, "golden-project.md"); got != want {
		t.Errorf("render differs from golden-project.md\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	for _, branchOnly := range []string{"Branch brief", "Set on this branch", "Inherits project brief", "evidence-upload"} {
		if strings.Contains(got, branchOnly) {
			t.Errorf("the project block prints the branch-only %q", branchOnly)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	first := branchBlock(t)
	for i := 0; i < 5; i++ {
		if again := branchBlock(t); again != first {
			t.Fatalf("render %d differs from the first", i+2)
		}
	}
}

func TestAllSkippedRendersTheKeyAndNoSection(t *testing.T) {
	c := complete(skip(discover.KeyPurpose), skip(discover.KeyAudience), skip(discover.KeyConstraints),
		skip(discover.KeyDone), skip(discover.KeyOutOfScope))
	r := &discover.Revision{BriefID: branchID, Number: 2, Key: key(branchID, 2), Scope: discover.ScopeBranch,
		BranchRef: "refs/heads/spike", Content: c}
	got := Render(resolve(t, nil, r))
	want := BeginPrefix + " brief=" + branchID + " rev=2 -->\n" +
		"# Branch brief — spike\n\n" +
		"Written by Discover in Panacea App. Every session started on this branch reads it first.\n" +
		"Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.\n" +
		"\nNo question in this brief has an answer.\n" +
		"\n---\n\nBrief " + branchID + ", revision 2.\n" + EndMarker + "\n"
	if got != want {
		t.Errorf("all skipped\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestOverrideNoteIsOneLine(t *testing.T) {
	// A head the Hub stored unchecked may hold "\r" (Render's M-5 premise):
	// the note is cleaned first, then joined, so no "\r" splits it (review I2,
	// RV15).
	for _, text := range []string{
		"Portal for audits.\n\nAlso for tax.",
		"Portal for audits.\r\nAlso for tax.",
		"Portal for audits.\rAlso for tax.",
		"Portal for audits.\r\n\r\nAlso for tax.\r\n",
	} {
		project := b7()
		project.Content.Answers[0] = person(discover.KeyPurpose, text)
		got := Render(resolve(t, project, b42()))
		if !strings.Contains(got, "\n_Set on this branch, replacing the project brief: Portal for audits. Also for tax._\n") {
			t.Errorf("the project text %q broke the override note:\n%s", text, got)
		}
	}
}

func TestMergeReplacesOnlyTheManagedBlock(t *testing.T) {
	old := Render(resolve(t, b7(), nil))
	above := "# Team notes\n\nRun `make test` before pushing.\n\n"
	below := "\n## Local setup\n\nUse Go 1.23.\n"
	existing := above + old + below
	block := branchBlock(t)
	got, err := Merge(existing, block)
	if err != nil {
		t.Fatal(err)
	}
	if want := above + block + below; got != want {
		t.Errorf("merge\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	if n := strings.Count(got, BeginPrefix); n != 1 {
		t.Errorf("%d begin markers after merge", n)
	}
	// Merging the same block again changes nothing.
	again, err := Merge(got, block)
	if err != nil || again != got {
		t.Errorf("a second merge of the same block changed the file (err %v)", err)
	}
}

func TestMergeAppendsAfterABlankLine(t *testing.T) {
	block := branchBlock(t)
	cases := []struct{ existing, want string }{
		{"", block},
		{"# Notes\n", "# Notes\n\n" + block},
		{"# Notes", "# Notes\n\n" + block},
		{"# Notes\n\n", "# Notes\n\n\n" + block},
		{"# Notes\r\n", "# Notes\r\n\r\n" + crlf(block)},
		{"# Notes\r\nMore", "# Notes\r\nMore\r\n\r\n" + crlf(block)},
		// Mixed endings: the first line ending decides.
		{"# Notes\nMore\r\n", "# Notes\nMore\r\n\n" + block},
		{"# Notes\r\nMore\n", "# Notes\r\nMore\n\r\n" + crlf(block)},
		// A file that starts with a line ending (review I2, RV3): nothing
		// precedes the first "\n", so it is LF; a first "\r\n" is CRLF.
		{"\nA\n", "\nA\n\n" + block},
		{"\n", "\n\n" + block},
		{"\r\nA\r\n", "\r\nA\r\n\r\n" + crlf(block)},
	}
	for _, tc := range cases {
		got, err := Merge(tc.existing, block)
		if err != nil {
			t.Errorf("Merge(%q): %v", tc.existing, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Merge(%q)\n got %q\nwant %q", tc.existing, got, tc.want)
		}
	}
}

// beginLine is the begin marker of branchBlock, without its line ending.
const beginLine = BeginPrefix + " brief=" + branchID + " rev=5 -->"

// brokenCase is a file whose markers do not pair.
type brokenCase struct {
	name, existing string
	want           error
}

func brokenFiles(block string) []brokenCase {
	middle := strings.Index(block, "## Who it is for")
	return []brokenCase{
		{"a begin with no end", "A\n\n" + strings.TrimSuffix(block, EndMarker+"\n") + "B\n", ErrUnterminatedBlock},
		{"an end with no begin", "A\n" + EndMarker + "\nB\n", ErrUnterminatedBlock},
		{"an end before the begin", EndMarker + "\n" + strings.Replace(block, EndMarker, "", 1), ErrUnterminatedBlock},
		{"an end, then a whole block", "A\n" + EndMarker + "\n\n" + block, ErrUnterminatedBlock},
		// The forged early end: begin, forged end, the brief's tail, the real end.
		{"a second end marker", "A\n\n" + block[:middle] + EndMarker + "\n" + block[middle:] + "B\n", ErrUnterminatedBlock},
		{"begin, end, then a stray end", "A\n\n" + block + "B\n" + EndMarker + "\n", ErrUnterminatedBlock},
		{"a begin, a begin and one end", "A\n\n" + beginLine + "\n" + block, ErrDuplicateBlock},
		{"two whole blocks", "A\n\n" + block + "\n" + block, ErrDuplicateBlock},
		{"a begin, a begin and no end", beginLine + "\n" + beginLine + "\n", ErrDuplicateBlock},
	}
}

func TestMergeAndStripFailClosed(t *testing.T) {
	block := branchBlock(t)
	for _, tc := range brokenFiles(block)[:1] {
		checkBroken(t, tc, block)
	}
	for _, tc := range brokenFiles(block)[6:] {
		checkBroken(t, tc, block)
	}
}

func TestStrayEndMarkerIsBroken(t *testing.T) {
	block := branchBlock(t)
	for _, tc := range brokenFiles(block) {
		checkBroken(t, tc, block)
		// The same file re-saved with CRLF is just as broken.
		checkBroken(t, brokenCase{tc.name + " (CRLF)", crlf(tc.existing), tc.want}, block)
	}
	// No marker of either kind is not an error.
	for _, plain := range []string{"", "A\n", "Mentions panacea:discover:end without a comment opener\n"} {
		if got, err := Strip(plain); err != nil || got != plain {
			t.Errorf("Strip(%q) = %q, %v; want it unchanged", plain, got, err)
		}
		if b, ok, err := Inspect(plain); ok || err != nil || !reflect.DeepEqual(b, Block{}) {
			t.Errorf("Inspect(%q) = %+v, %v, %v; want nothing found", plain, b, ok, err)
		}
		if _, err := Merge(plain, block); err != nil {
			t.Errorf("Merge(%q): %v", plain, err)
		}
	}
}

func checkBroken(t *testing.T, tc brokenCase, block string) {
	t.Helper()
	if got, err := Merge(tc.existing, block); !errors.Is(err, tc.want) || got != tc.existing {
		t.Errorf("%s: Merge err = %v (want %v), input changed: %v", tc.name, err, tc.want, got != tc.existing)
	}
	if got, err := Strip(tc.existing); !errors.Is(err, tc.want) || got != tc.existing {
		t.Errorf("%s: Strip err = %v (want %v), input changed: %v", tc.name, err, tc.want, got != tc.existing)
	}
	if b, ok, err := Inspect(tc.existing); !errors.Is(err, tc.want) || ok || !reflect.DeepEqual(b, Block{}) {
		t.Errorf("%s: Inspect = %+v, %v, %v; want nothing and %v", tc.name, b, ok, err, tc.want)
	}
}

// neutralisedDoc holds a comment opener in every string Render interpolates:
// the title (a git ref may hold "<" and "!"), an answer, a criterion, a
// follow-up prompt, the replaced project text. A head the Hub stored
// unchecked; task 03's rule 12 refuses these on the way in.
func neutralisedDoc(t *testing.T) discover.Document {
	t.Helper()
	project := b7()
	project.Content = complete(
		person(discover.KeyPurpose, "Old <!-- x --> purpose"),
		person(discover.KeyAudience, "KPMG staff broadly"),
		skip(discover.KeyConstraints),
		done("b", "Tests pass"),
		skip(discover.KeyOutOfScope),
	)
	c := complete(
		person(discover.KeyPurpose, "Stop at <!-- panacea:discover:end --> here"),
		skip(discover.KeyAudience),
		skip(discover.KeyConstraints),
		done("a", "Upload <!-- x --> works", "Tests pass"),
		skip(discover.KeyOutOfScope),
		person("f-0000000a", "Yes <!-- x -->"),
	)
	c.Questions = append(c.Questions, discover.Question{Key: "f-0000000a",
		BranchPrompt: "Does <!-- panacea:discover:end --> hold?", Options: []string{}, FollowUp: true})
	c.Inherits = &discover.Inherit{Key: key(projectID, 5), Digest: "sha256:" + strings.Repeat("7", 64)}
	branch := &discover.Revision{BriefID: branchID, Number: 3, Key: key(branchID, 3), Scope: discover.ScopeBranch,
		BranchRef: "refs/heads/<!--x", Content: c}
	return resolve(t, project, branch)
}

func TestRenderNeutralisesCommentOpeners(t *testing.T) {
	doc := neutralisedDoc(t)
	block := Render(doc)
	if want := golden(t, "golden-neutralised.md"); block != want {
		t.Errorf("render differs from golden-neutralised.md\n--- got ---\n%s--- want ---\n%s", block, want)
	}
	if n := strings.Count(block, "<!--"); n != 2 {
		t.Errorf("%d comment openers in the block, want its own 2", n)
	}
	for _, s := range []string{"Old <!-\\- x --> purpose", "Stop at <!-\\- panacea:discover:end --> here",
		"- Upload <!-\\- x --> works", "## Does <!-\\- panacea:discover:end --> hold", "# Branch brief — <!-\\-x"} {
		if !strings.Contains(block, s) {
			t.Errorf("block lacks the neutralised %q", s)
		}
	}
	b, ok, err := Inspect(block)
	if err != nil || !ok || b.Marker != (Marker{BriefID: branchID, Revision: 3}) || b.Text != block {
		t.Errorf("Inspect(Render) = %+v, %v, %v", b.Marker, ok, err)
	}
	merged, err := Merge("", block)
	if err != nil {
		t.Fatal(err)
	}
	if stripped, err := Strip(merged); err != nil || stripped != "" {
		t.Errorf("Strip(Merge(\"\", block)) = %q, %v", stripped, err)
	}
	// A document built by hand is neutralised too, the ids included.
	hand := discover.Document{Title: "T <!-- x", BriefID: "bf-<!--1", Revision: 1, ProjectBriefID: "bf-<!--2", ProjectRevision: 1,
		Sections: []discover.Section{{Key: "k", Heading: "H <!--", Body: "B <!--", Origin: discover.OriginOverride, Replaced: "R <!--",
			Items: []discover.Criterion{}}}}
	if n := strings.Count(Render(hand), "<!--"); n != 2 {
		t.Errorf("a hand-built document renders %d comment openers, want 2", n)
	}
}

// bareLF reports whether s holds a "\n" not preceded by "\r".
func bareLF(s string) bool {
	return strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n")
}

func TestStripRemovesTheBlockAndNothingElse(t *testing.T) {
	block := branchBlock(t)
	old := Render(resolve(t, b7(), nil))

	// Merge then Strip. Every file is spelled out; block ends in "\n".
	roundTrips := []struct {
		name, existing, merged, stripped string
	}{
		{"an empty file", "", block, ""},
		{"text above", "A\n", "A\n\n" + block, "A\n"},
		{"text above with no final line ending", "A", "A\n\n" + block, "A\n"},
		// b10 re-review R-I1: the block is followed directly by B. The blank
		// line above it is the person's paragraph break, and it stays.
		{"text above and below", "A\n\n" + old + "B\n", "A\n\n" + block + "B\n", "A\n\nB\n"},
		{"CRLF, text above", "A\r\n", "A\r\n\r\n" + crlf(block), "A\r\n"},
		{"CRLF, text above and below", "A\r\n\r\n" + crlf(old) + "B\r\n", "A\r\n\r\n" + crlf(block) + "B\r\n", "A\r\n\r\nB\r\n"},
	}
	for _, tc := range roundTrips {
		merged, err := Merge(tc.existing, block)
		if err != nil || merged != tc.merged {
			t.Errorf("%s: Merge = %q, %v\nwant %q", tc.name, merged, err, tc.merged)
			continue
		}
		if strings.Contains(tc.existing, "\r\n") && bareLF(merged) {
			t.Errorf("%s: Merge wrote an LF line ending into a CRLF file", tc.name)
		}
		if got, err := Strip(merged); err != nil || got != tc.stripped {
			t.Errorf("%s: Strip = %q, %v; want %q", tc.name, got, err, tc.stripped)
		}
	}

	// Strip alone, on files a person has touched.
	strips := []struct{ name, existing, want string }{
		{"the blank line deleted by hand", "A\n" + block, "A\n"},
		{"a file that begins with the block", block + "B\n", "B\n"},
		{"a blank line between the block and B", "A\n\n" + block + "\nB\n", "A\n\n\nB\n"},
		// A line ending after the block's own is something following it, so
		// the blank line above stays: B = "" in the row above (review I2, RV11).
		{"a blank line after an appended block", "A\n\n" + block + "\n", "A\n\n\n"},
		{"CRLF, a blank line after an appended block", "A\r\n\r\n" + crlf(block) + "\r\n", "A\r\n\r\n\r\n"},
		{"a block with no final line ending", "A\n\n" + strings.TrimSuffix(block, "\n"), "A\n"},
		{"two blank lines above an appended block", "A\n\n\n" + block, "A\n\n"},
		{"text on the end marker's line", "A\n\n" + strings.TrimSuffix(block, "\n") + "B\n", "A\n\nB\n"},
		// The editor case: merged with LF, re-saved with every "\n" as "\r\n".
		{"an LF merge re-saved as CRLF", crlf("A\n\n" + block), "A\r\n"},
		{"an LF merge with text below, re-saved as CRLF", crlf("A\n\n" + block + "B\n"), "A\r\n\r\nB\r\n"},
		{"CRLF, the blank line deleted by hand", "A\r\n" + crlf(block), "A\r\n"},
	}
	for _, tc := range strips {
		if got, err := Strip(tc.existing); err != nil || got != tc.want {
			t.Errorf("%s: Strip = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestCRLFResaveIsNotAnEdit(t *testing.T) {
	block := branchBlock(t)
	resaved := "A\r\n\r\n" + crlf(block)
	b, ok, err := Inspect(resaved)
	if err != nil || !ok || b.Marker != (Marker{BriefID: branchID, Revision: 5}) {
		t.Fatalf("Inspect(CRLF) = %+v, %v, %v", b.Marker, ok, err)
	}
	if b.Text != crlf(block) {
		t.Errorf("Inspect's text is not the CRLF block verbatim: %q", b.Text)
	}
	if !SameText(b.Text, block) {
		t.Error("SameText: a CRLF re-save reads as an edit")
	}
	for _, d := range DiffLines(block, b.Text) {
		if d.Op != DiffSame || strings.Contains(d.Text, "\r") {
			t.Errorf("CRLF re-save diff line %+v, want same with no \\r", d)
		}
	}

	edited := strings.Replace(b.Text, "KPMG staff broadly", "KPMG staff, broadly", 1)
	if SameText(edited, block) {
		t.Error("SameText: a changed line reads as current")
	}
	var removed, added int
	for _, d := range DiffLines(block, edited) {
		switch d.Op {
		case DiffRemoved:
			removed++
		case DiffAdded:
			added++
		}
	}
	if removed != 1 || added != 1 {
		t.Errorf("one CRLF line changed: %d removed, %d added; want 1 and 1", removed, added)
	}

	if SameText("a\rb\n", "ab\n") {
		t.Error(`SameText("a\rb\n", "ab\n") is true: a lone \r is a difference`)
	}
	if SameText("a\r\n", "a\r") || !SameText("a\r\nb", "a\nb") {
		t.Error("SameText reads only \\r\\n as \\n")
	}
}

func TestRenderWritesOnlyLF(t *testing.T) {
	c := complete(
		person(discover.KeyPurpose, "x\r\ny\rz"),
		skip(discover.KeyAudience),
		skip(discover.KeyConstraints),
		discover.Answer{Key: discover.KeyDone, Text: "x\r\ny\rz", Source: discover.SourceTyped, Author: discover.AuthorPerson,
			Criteria: []discover.Criterion{{ID: "c-00000001", Text: "x\r\ny\rz"}}, DraftedFrom: []string{}, AnsweredAt: stamp},
		skip(discover.KeyOutOfScope),
		person("f-0000000a", "ok"),
	)
	c.Questions = append(c.Questions, discover.Question{Key: "f-0000000a", BranchPrompt: "x\r\ny\rz?", Options: []string{}, FollowUp: true})
	r := &discover.Revision{BriefID: branchID, Number: 5, Key: key(branchID, 5), Scope: discover.ScopeBranch,
		BranchRef: "refs/heads/evidence-upload", Content: c}
	doc := resolve(t, nil, r)
	block := Render(doc)
	if strings.Contains(block, "\r") {
		t.Fatalf("Render holds a \\r: %q", block)
	}
	for _, want := range []string{"\n\nx\ny\nz\n", "\n- x\ny\nz\n", "\n## x\ny\nz\n"} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}

	intoCRLF, err := Merge("A\r\n", block)
	if err != nil {
		t.Fatal(err)
	}
	if bareLF(intoCRLF) || strings.Contains(intoCRLF, "\r\r\n") {
		t.Errorf("Merge into a CRLF file: %q", intoCRLF)
	}
	if b, ok, err := Inspect(intoCRLF); err != nil || !ok || !SameText(b.Text, block) {
		t.Errorf("the CRLF file's block is not the render: ok %v, err %v", ok, err)
	}
	if intoLF, err := Merge("A\n", block); err != nil || strings.Contains(intoLF, "\r") {
		t.Errorf("Merge into an LF file holds a \\r (err %v)", err)
	}
	if again, err := Merge("A\r\n", crlf(block)); err != nil || again != intoCRLF {
		t.Errorf("a CRLF block merged into a CRLF file: %q, want %q (err %v)", again, intoCRLF, err)
	}
}

func TestInspectReadsTheMarker(t *testing.T) {
	block := branchBlock(t)
	b, ok, err := Inspect("A\n\n" + block + "B\n")
	if err != nil || !ok {
		t.Fatalf("Inspect = %v, %v", ok, err)
	}
	if b.Marker != (Marker{BriefID: branchID, Revision: 5}) || b.Text != block {
		t.Errorf("Inspect = %+v", b)
	}
	body := strings.TrimPrefix(block, beginLine)
	for _, line := range []string{
		BeginPrefix + " brief=" + branchID + " rev=05 -->",
		BeginPrefix + " brief=" + branchID + " rev=0 -->",
		BeginPrefix + " brief=" + branchID + " -->",
		BeginPrefix + " brief=" + branchID + " rev= -->",
		BeginPrefix + " brief=" + branchID + " rev=-5 -->",
		BeginPrefix + " brief=" + branchID + " rev=+5 -->",
		BeginPrefix + " brief=" + branchID + " rev=5x -->",
		BeginPrefix + " brief=" + branchID + " rev=99999999999999999999 -->",
		BeginPrefix + " brief= rev=5 -->",
		BeginPrefix + " rev=5 brief=" + branchID + " -->",
		BeginPrefix + "  brief=" + branchID + " rev=5 -->",
		BeginPrefix + " brief=" + branchID + " rev=5-->",
		BeginPrefix + " brief=" + branchID + " rev=5",
		BeginPrefix + " brief=" + branchID + " rev=5 --> trailing",
		BeginPrefix + " brief=" + branchID + " rev=5 extra=1 -->",
		BeginPrefix + "x brief=" + branchID + " rev=5 -->",
		BeginPrefix + " brief=bf-<a2 rev=5 -->",
		BeginPrefix + " brief=bf->a2 rev=5 -->",
		BeginPrefix + " brief=bf-\ta2 rev=5 -->",
		BeginPrefix + " brief=bf- a2 rev=5 -->",
	} {
		if got, ok, err := Inspect(line + body); !errors.Is(err, ErrBadMarker) || ok || !reflect.DeepEqual(got, Block{}) {
			t.Errorf("Inspect(%q) = %+v, %v, %v; want ErrBadMarker", line, got.Marker, ok, err)
		}
	}
}

// Every id Resolve accepts reads back through Inspect, and every id it
// refuses, Inspect refuses in a begin marker: one rule, discover.MarkerSafe,
// on both sides, so a block Render(Resolve(…)) writes is never broken on the
// next start (review I1; part As built 4). The ids are the review's.
func TestEveryResolvedIDReadsBack(t *testing.T) {
	body := strings.TrimPrefix(branchBlock(t), beginLine)
	for _, tc := range []struct {
		id       string
		resolves bool
	}{
		{branchID, true},
		{"bf-!a2", true},
		{"bf->a2", false},
		{"bf-<a2", false},
		{"bf-<!--a2", false},
		{"bf a2", false},
		{"bf\ta2", false},
		{"bf\na2", false},
		{"bf a2", false},
		{"", false},
	} {
		r := b7()
		r.BriefID = tc.id
		doc, rerr := discover.Resolve(r, nil)
		if (rerr == nil) != tc.resolves {
			t.Errorf("Resolve(brief %q): err %v, want resolves %v", tc.id, rerr, tc.resolves)
		}
		_, ok, ierr := Inspect(BeginPrefix + " brief=" + tc.id + " rev=5 -->" + body)
		if ok != (rerr == nil) || (ierr == nil) != (rerr == nil) {
			t.Errorf("brief %q: Resolve err %v, but Inspect of its marker: ok %v, err %v", tc.id, rerr, ok, ierr)
		}
		if rerr != nil {
			continue
		}
		b, ok, err := Inspect("A\n\n" + Render(doc))
		if err != nil || !ok || b.Marker != (Marker{BriefID: tc.id, Revision: 5}) {
			t.Errorf("brief %q: Inspect(Render(Resolve)) = %+v, %v, %v", tc.id, b.Marker, ok, err)
		}
	}
}

// SameText is true exactly when DiffLines shows no change (review M1), so
// the drift check never calls a block "edited by hand" over a diff with
// nothing in it. Both split on \r?\n, and a final line ending ends the last
// line rather than starting another.
func TestSameTextAgreesWithDiffLines(t *testing.T) {
	block := branchBlock(t)
	noFinal := strings.TrimSuffix(block, "\n")
	edited := strings.Replace(block, "KPMG staff broadly", "KPMG staff, broadly", 1)
	for _, tc := range []struct {
		name, got, want string
		same            bool
	}{
		{"the render itself", block, block, true},
		{"re-saved as CRLF", crlf(block), block, true},
		{"the end marker ends the file with no line ending", noFinal, block, true},
		{"CRLF, with no final line ending", strings.TrimSuffix(crlf(block), "\r\n"), block, true},
		{"one line changed", edited, block, false},
		{"one line changed, with no final line ending", strings.TrimSuffix(edited, "\n"), block, false},
		{"a blank line added at the end", block + "\n", block, false},
		{"a lone \\r", "a\rb\n", "ab\n", false},
		{"a\\r\\n against a\\r", "a\r\n", "a\r", false},
		{"one line with and without its ending", "a", "a\n", true},
		{"nothing against one empty line", "", "\n", false},
	} {
		if got := SameText(tc.got, tc.want); got != tc.same {
			t.Errorf("%s: SameText = %v, want %v", tc.name, got, tc.same)
		}
		changed := false
		for _, d := range DiffLines(tc.want, tc.got) {
			changed = changed || d.Op != DiffSame
		}
		if changed == tc.same {
			t.Errorf("%s: DiffLines shows a change: %v; want %v", tc.name, changed, !tc.same)
		}
	}

	// The file the review probed: the end marker is its last byte.
	b, ok, err := Inspect("A\n\n" + noFinal)
	if err != nil || !ok || b.Text != noFinal || !SameText(b.Text, block) {
		t.Errorf("a block with no final line ending: %q, ok %v, err %v; want current", b.Text, ok, err)
	}
}

func TestDiffLinesShowsAHandEdit(t *testing.T) {
	block := branchBlock(t)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	at := -1
	for i, l := range lines {
		if l == "KPMG staff broadly" {
			at = i
		}
	}
	edited := strings.Replace(block, "KPMG staff broadly\n", "KPMG staff, broadly\n", 1)
	d := DiffLines(block, edited)
	if len(d) != len(lines)+1 {
		t.Fatalf("%d diff lines for %d lines and one change", len(d), len(lines))
	}
	for i, l := range d {
		switch {
		case i == at:
			if l != (DiffLine{Op: DiffRemoved, Text: "KPMG staff broadly"}) {
				t.Errorf("line %d = %+v, want the render's line removed", i, l)
			}
		case i == at+1:
			if l != (DiffLine{Op: DiffAdded, Text: "KPMG staff, broadly"}) {
				t.Errorf("line %d = %+v, want the file's line added", i, l)
			}
		default:
			j := i
			if i > at {
				j = i - 1
			}
			if l != (DiffLine{Op: DiffSame, Text: lines[j]}) {
				t.Errorf("line %d = %+v, want same %q", i, l, lines[j])
			}
		}
	}

	// A line only in the file is added; a line only in the render is removed.
	extra := strings.Replace(block, "KPMG staff broadly\n", "KPMG staff broadly\nand me\n", 1)
	onlyOps(t, DiffLines(block, extra), DiffAdded, "and me")
	onlyOps(t, DiffLines(extra, block), DiffRemoved, "and me")

	// Two edits far apart: each shows in place, and every line between them
	// is same, not the stretch reported removed and added again.
	two := strings.Replace(edited, "Bulk upload; the reviewer side of evidence\n", "Bulk upload\n", 1)
	var changed []DiffLine
	same := 0
	for _, l := range DiffLines(block, two) {
		if l.Op == DiffSame {
			same++
		} else {
			changed = append(changed, l)
		}
	}
	wantChanged := []DiffLine{
		{Op: DiffRemoved, Text: "KPMG staff broadly"}, {Op: DiffAdded, Text: "KPMG staff, broadly"},
		{Op: DiffRemoved, Text: "Bulk upload; the reviewer side of evidence"}, {Op: DiffAdded, Text: "Bulk upload"},
	}
	if !reflect.DeepEqual(changed, wantChanged) || same != len(lines)-2 {
		t.Errorf("two edits: changed %+v, %d same; want %+v and %d same", changed, same, wantChanged, len(lines)-2)
	}
}

func TestDiffLineWireShape(t *testing.T) {
	for op, want := range map[DiffOp]string{
		DiffSame:    `{"op":"same","text":"x"}`,
		DiffAdded:   `{"op":"added","text":"x"}`,
		DiffRemoved: `{"op":"removed","text":"x"}`,
	} {
		raw, err := json.Marshal(DiffLine{Op: op, Text: "x"})
		if err != nil || string(raw) != want {
			t.Errorf("DiffLine %s marshals to %s (err %v), want %s", op, raw, err, want)
		}
	}
	// Added is the file's line, removed the render's.
	d := DiffLines("render\n", "file\n")
	if !reflect.DeepEqual(d, []DiffLine{{Op: "removed", Text: "render"}, {Op: "added", Text: "file"}}) {
		t.Errorf("DiffLines(render, file) = %+v", d)
	}
}

func onlyOps(t *testing.T, d []DiffLine, op DiffOp, text string) {
	t.Helper()
	n := 0
	for _, l := range d {
		if l.Op != DiffSame {
			n++
			if l.Op != op || l.Text != text {
				t.Errorf("diff line %+v, want %s %q", l, op, text)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d changed lines, want 1", n)
	}
}

func TestDiffLinesIsCapped(t *testing.T) {
	var a, b []string
	for i := 0; i < 3000; i++ {
		a = append(a, fmt.Sprintf("render %d", i))
		b = append(b, fmt.Sprintf("file %d", i))
	}
	want, got := strings.Join(a, "\n")+"\n", strings.Join(b, "\n")+"\n"
	if d := DiffLines(want, got); len(d) != 400 {
		t.Errorf("%d diff lines, want the cap of 400", len(d))
	}
	if d := DiffLines(want, want); len(d) != 400 {
		t.Errorf("%d diff lines for 3000 equal lines, want the cap of 400", len(d))
	} else {
		for i, l := range d {
			if l != (DiffLine{Op: DiffSame, Text: a[i]}) {
				t.Fatalf("3000 equal lines: line %d = %+v, want same %q", i, l, a[i])
			}
		}
	}
	if d := DiffLines("", ""); d == nil || len(d) != 0 {
		t.Errorf("DiffLines of nothing = %#v, want []", d)
	}
}
