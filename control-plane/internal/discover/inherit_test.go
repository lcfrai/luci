package discover

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The fact sheet's world (prototype-fact-sheet.md §2-§3), with Hub-shaped
// ids: b-7 is bf-00000000000000b7, b-42 is bf-00000000000000a2.
const (
	rvProjectID = "bf-00000000000000b7"
	rvBranchID  = "bf-00000000000000a2"
)

func rvPerson(k Key, text string) Answer {
	return Answer{Key: k, Text: text, Source: SourceTyped, Author: AuthorPerson,
		Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

func rvSkip(k Key) Answer {
	return Answer{Key: k, Skipped: true, Author: AuthorPerson,
		Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

// rvDone is a definition of done typed one item per line; ids c-<tag><n>.
func rvDone(tag string, items ...string) Answer {
	a := rvPerson(KeyDone, strings.Join(items, "\n"))
	for i, it := range items {
		a.Criteria = append(a.Criteria, Criterion{ID: fmt.Sprintf("c-%s%07d", tag, i+1), Text: it})
	}
	return a
}

func rvKey(id string, n int) string { return fmt.Sprintf("brief:%s@%d", id, n) }

// rvProject is b-7@5: complete, audience "KPMG staff broadly" (@6 edits it).
func rvProject() *Revision {
	return &Revision{BriefID: rvProjectID, Number: 5, Key: rvKey(rvProjectID, 5), Scope: ScopeProject,
		Content: Content{V: ContentVersion, Status: StatusComplete, Questions: BaseQuestions(), Answers: []Answer{
			rvPerson(KeyPurpose, "Client-facing portal for audit engagements: requests, evidence and status."),
			rvPerson(KeyAudience, "KPMG staff broadly"),
			rvPerson(KeyConstraints, "Existing client data; the published API"),
			rvDone("b", "Tests pass", "No change to the published API without a version bump"),
			rvPerson(KeyOutOfScope, "Billing and invoicing"),
		}}}
}

// rvBranch is b-42@5 on evidence-upload, pinning b-7@5; audience skipped.
func rvBranch() *Revision {
	return &Revision{BriefID: rvBranchID, Number: 5, Key: rvKey(rvBranchID, 5), Scope: ScopeBranch,
		BranchRef: "refs/heads/evidence-upload",
		Content: Content{V: ContentVersion, Status: StatusComplete, Questions: BaseQuestions(), Answers: []Answer{
			rvPerson(KeyPurpose, "Let client users upload evidence files against an open request."),
			rvSkip(KeyAudience),
			rvPerson(KeyConstraints, "Existing uploads and their download links"),
			rvDone("a", "Upload works for PDF, XLSX and images", "A virus scan runs before any file is visible", "Tests pass"),
			rvPerson(KeyOutOfScope, "Bulk upload; the reviewer side of evidence"),
		}, Inherits: &Inherit{Key: rvKey(rvProjectID, 5), Digest: "sha256:" + strings.Repeat("7", 64)}}}
}

func rvSet(r *Revision, a Answer) *Revision {
	r.Content.put(a)
	return r
}

func rvSection(t *testing.T, doc Document, k Key) (Section, bool) {
	t.Helper()
	for _, s := range doc.Sections {
		if s.Key == k {
			return s, true
		}
	}
	return Section{}, false
}

func mustResolve(t *testing.T, project, branch *Revision) Document {
	t.Helper()
	doc, err := Resolve(project, branch)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return doc
}

func TestTitleComesFromTheRevision(t *testing.T) {
	// No name can be passed: the signature is exactly this.
	var resolve func(project, branch *Revision) (Document, error) = Resolve
	if got := reflect.TypeOf(resolve).String(); got != "func(*discover.Revision, *discover.Revision) (discover.Document, error)" {
		t.Fatalf("Resolve's type = %s", got)
	}

	branch := rvBranch()
	if got := Title(branch); got != "Branch brief — evidence-upload" {
		t.Errorf("Title(branch) = %q", got)
	}
	if got := Title(nil); got != "Project brief" {
		t.Errorf("Title(nil) = %q", got)
	}
	if got := mustResolve(t, rvProject(), nil).Title; got != "Project brief" {
		t.Errorf("project document title = %q", got)
	}
	resolved := mustResolve(t, rvProject(), branch)
	if resolved.Title != "Branch brief — evidence-upload" {
		t.Errorf("branch document title = %q", resolved.Title)
	}

	// An open revision on the same ref gets its complete successor's title.
	open := rvBranch()
	open.Number, open.Key = 4, rvKey(rvBranchID, 4)
	open.Content.Status = StatusOpen
	open.Content.Answers = open.Content.Answers[:4]
	if got := Title(open); got != resolved.Title {
		t.Errorf("Title(open) = %q, Resolve's title for its successor = %q", got, resolved.Title)
	}
	if _, err := Resolve(rvProject(), open); !errors.Is(err, ErrIncomplete) {
		t.Errorf("Resolve(open) err = %v, want ErrIncomplete", err)
	}

	// Only refs/heads/ is removed; a slash inside the branch name stays.
	nested := rvBranch()
	nested.BranchRef = "refs/heads/feat/request-reminders-email"
	if got := Title(nested); got != "Branch brief — feat/request-reminders-email" {
		t.Errorf("Title(nested) = %q", got)
	}
}

func TestResolveRefusesAProjectRevisionTheBranchDidNotPin(t *testing.T) {
	head := rvProject()
	head.Number, head.Key = 6, rvKey(rvProjectID, 6)
	head = rvSet(head, rvPerson(KeyAudience, "Client users, and the KPMG staff who serve them"))

	unpinned := rvBranch()
	unpinned.Content.Inherits = nil

	cases := []struct {
		name            string
		project, branch *Revision
	}{
		{"the project head, not the pinned @5", head, rvBranch()},
		{"a project for a branch that pinned none", rvProject(), unpinned},
		{"no project for a branch that pinned one", nil, rvBranch()},
	}
	for _, tc := range cases {
		doc, err := Resolve(tc.project, tc.branch)
		if !errors.Is(err, ErrInheritMismatch) {
			t.Errorf("%s: err = %v, want ErrInheritMismatch", tc.name, err)
		}
		if !reflect.DeepEqual(doc, Document{}) {
			t.Errorf("%s: a refused resolve returned a document: %+v", tc.name, doc)
		}
	}

	// The pin as written resolves, and a branch that pinned none resolves alone.
	mustResolve(t, rvProject(), rvBranch())
	alone := mustResolve(t, nil, unpinned)
	if alone.ProjectBriefID != "" || alone.ProjectRevision != 0 {
		t.Errorf("an unpinned branch names a project revision: %q @%d", alone.ProjectBriefID, alone.ProjectRevision)
	}
	for _, s := range alone.Sections {
		if s.Origin != OriginBranch {
			t.Errorf("unpinned branch: section %s origin %s, want branch", s.Key, s.Origin)
		}
	}
}

func TestResolveRefusesAnIncompleteRevision(t *testing.T) {
	openBranch := rvBranch()
	openBranch.Content.Status = StatusOpen
	openProject := rvProject()
	openProject.Content.Status = StatusOpen

	cases := []struct {
		name            string
		project, branch *Revision
	}{
		{"an open branch revision", rvProject(), openBranch},
		{"an open project revision alone", openProject, nil},
		{"an open project revision under a complete branch", openProject, rvBranch()},
	}
	for _, tc := range cases {
		doc, err := Resolve(tc.project, tc.branch)
		if !errors.Is(err, ErrIncomplete) {
			t.Errorf("%s: err = %v, want ErrIncomplete", tc.name, err)
		}
		if !reflect.DeepEqual(doc, Document{}) {
			t.Errorf("%s: a refused resolve returned a document", tc.name)
		}
	}
}

func TestSkippedBranchAnswerInheritsTheProject(t *testing.T) {
	doc := mustResolve(t, rvProject(), rvBranch())
	s, ok := rvSection(t, doc, KeyAudience)
	if !ok {
		t.Fatal("the skipped audience has no section")
	}
	if s.Origin != OriginProject || s.Body != "KPMG staff broadly" || s.Replaced != "" {
		t.Errorf("audience = %+v, want the project's text, OriginProject, nothing replaced", s)
	}
	if s.Heading != "Who it is for" {
		t.Errorf("audience heading = %q", s.Heading)
	}
}

func TestSkippedProjectAnswerIsOmitted(t *testing.T) {
	project := rvSet(rvProject(), rvSkip(KeyOutOfScope))
	branch := rvSet(rvBranch(), rvSkip(KeyOutOfScope))
	for name, doc := range map[string]Document{
		"branch silent": mustResolve(t, project, branch),
		"project alone": mustResolve(t, project, nil),
	} {
		if _, ok := rvSection(t, doc, KeyOutOfScope); ok {
			t.Errorf("%s: an Out of scope section for an answer nobody gave", name)
		}
		if len(doc.Sections) != 4 {
			t.Errorf("%s: %d sections, want 4", name, len(doc.Sections))
		}
	}

	// The project skipped, the branch answered: the branch's, OriginBranch.
	doc := mustResolve(t, project, rvBranch())
	s, ok := rvSection(t, doc, KeyOutOfScope)
	if !ok || s.Origin != OriginBranch || s.Body != "Bulk upload; the reviewer side of evidence" || s.Replaced != "" {
		t.Errorf("out of scope = %+v (found %v), want the branch's text, OriginBranch", s, ok)
	}
}

func TestDraftedTextIsNeverRendered(t *testing.T) {
	drafted := func(k Key, text string) Answer {
		a := rvPerson(k, text)
		a.Source, a.Author, a.DraftedFrom = SourceDrafted, AuthorModelDrafted, []string{"package.json"}
		return a
	}
	// Status forced complete by a caller: Resolve reads the stored status,
	// and still never renders a held draft.
	branch := rvSet(rvBranch(), drafted(KeyConstraints, "BRANCH-DRAFT never shown"))
	project := rvSet(rvProject(), drafted(KeyOutOfScope, "PROJECT-DRAFT never shown"))
	branch = rvSet(branch, rvSkip(KeyOutOfScope))

	for name, doc := range map[string]Document{
		"branch over project": mustResolve(t, project, branch),
		"project alone":       mustResolve(t, project, nil),
	} {
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"BRANCH-DRAFT", "PROJECT-DRAFT"} {
			if strings.Contains(string(raw), text) {
				t.Errorf("%s: drafted text %q reached the document: %s", name, text, raw)
			}
		}
		if _, ok := rvSection(t, doc, KeyOutOfScope); ok {
			t.Errorf("%s: a section for a key whose only text is a draft", name)
		}
	}
	// A branch draft is no answer: the project's constraints are inherited.
	doc := mustResolve(t, project, branch)
	if s, ok := rvSection(t, doc, KeyConstraints); !ok || s.Origin != OriginProject || s.Body != "Existing client data; the published API" {
		t.Errorf("constraints under a branch draft = %+v (found %v), want the project's, inherited", s, ok)
	}

	// A person accepted the model's text: that is an answer, and it renders.
	accepted := rvPerson(KeyConstraints, "Accepted wording")
	accepted.Source, accepted.Author = SourceDrafted, AuthorModelAccepted
	doc = mustResolve(t, rvProject(), rvSet(rvBranch(), accepted))
	if s, ok := rvSection(t, doc, KeyConstraints); !ok || s.Body != "Accepted wording" || s.Origin != OriginOverride {
		t.Errorf("model_accepted constraints = %+v (found %v), want rendered as an override", s, ok)
	}
}

func TestResolveOverrideCarriesTheReplacedText(t *testing.T) {
	doc := mustResolve(t, rvProject(), rvBranch())
	want := map[Key]string{
		KeyPurpose:     "Client-facing portal for audit engagements: requests, evidence and status.",
		KeyConstraints: "Existing client data; the published API",
		// Two criteria typed on two lines: the note is one line, the items
		// joined as SplitCriteria would split them again.
		KeyDone:       "Tests pass; No change to the published API without a version bump",
		KeyOutOfScope: "Billing and invoicing",
	}
	for k, replaced := range want {
		s, ok := rvSection(t, doc, k)
		if !ok || s.Origin != OriginOverride || s.Replaced != replaced {
			t.Errorf("%s = %+v (found %v), want an override replacing %q", k, s, ok, replaced)
		}
	}
	done, _ := rvSection(t, doc, KeyDone)
	wantItems := rvBranch().Content.Answers[3].Criteria
	if !reflect.DeepEqual(done.Items, wantItems) {
		t.Errorf("done items = %+v, want the branch's criteria, ids kept: %+v", done.Items, wantItems)
	}
	if done.Body != "Upload works for PDF, XLSX and images\nA virus scan runs before any file is visible\nTests pass" {
		t.Errorf("done body = %q", done.Body)
	}
	for _, s := range doc.Sections {
		if s.Key != KeyDone && (s.Items == nil || len(s.Items) != 0) {
			t.Errorf("%s items = %#v, want [] (never null)", s.Key, s.Items)
		}
	}
}

func TestResolveOrdersSectionsAndHeadsFollowUps(t *testing.T) {
	project := rvProject()
	// Answers stored out of file order: the order is the heading table's.
	a := project.Content.Answers
	project.Content.Answers = []Answer{a[4], a[2], a[0], a[3], a[1]}
	project.Content.Questions = append(project.Content.Questions,
		Question{Key: "f-0000000p", ProjectPrompt: "Which regulator reads the output?", BranchPrompt: "Which regulator, on this branch?", Options: []string{}, FollowUp: true})
	project = rvSet(project, rvPerson("f-0000000p", "APRA"))

	branch := rvBranch()
	branch.Content.Questions = append(branch.Content.Questions,
		Question{Key: "f-0000000b", ProjectPrompt: "Not this wording?", BranchPrompt: "Which browsers must work?", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000c", BranchPrompt: "Is there a deadline", Options: []string{}, FollowUp: true})
	branch = rvSet(branch, rvPerson("f-0000000c", "End of quarter"))
	branch = rvSet(branch, rvPerson("f-0000000b", "Edge and Chrome"))

	doc := mustResolve(t, project, branch)
	var got []string
	for _, s := range doc.Sections {
		got = append(got, string(s.Key)+"|"+s.Heading+"|"+string(s.Origin))
	}
	want := []string{
		"purpose|What this is for|override",
		"audience|Who it is for|project",
		"constraints|What must not break|override",
		"done|Definition of done|override",
		"out_of_scope|Out of scope|override",
		"f-0000000p|Which regulator reads the output|project",
		"f-0000000b|Which browsers must work|branch",
		"f-0000000c|Is there a deadline|branch",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections\n got %q\nwant %q", got, want)
	}

	// One follow-up key held by both revisions (a restore, or an id that
	// collided): the heading is the wording of the revision whose answer the
	// section shows.
	shared := Question{Key: "f-0000000s", ProjectPrompt: "Which team owns it?", BranchPrompt: "Which team owns this branch?", Options: []string{}, FollowUp: true}
	withShared := func(r *Revision, a Answer) *Revision {
		r.Content.Questions = append(r.Content.Questions, shared)
		return rvSet(r, a)
	}
	for _, tc := range []struct {
		name            string
		project, branch Answer
		heading         string
		origin          Origin
	}{
		{"the branch answers", rvPerson("f-0000000s", "Audit"), rvPerson("f-0000000s", "Payments"), "Which team owns this branch", OriginOverride},
		{"the branch skips", rvPerson("f-0000000s", "Audit"), rvSkip("f-0000000s"), "Which team owns it", OriginProject},
		{"the project skips", rvSkip("f-0000000s"), rvPerson("f-0000000s", "Payments"), "Which team owns this branch", OriginBranch},
	} {
		doc := mustResolve(t, withShared(rvProject(), tc.project), withShared(rvBranch(), tc.branch))
		s, ok := rvSection(t, doc, "f-0000000s")
		if !ok || s.Heading != tc.heading || s.Origin != tc.origin {
			t.Errorf("%s: section %+v (found %v), want heading %q, origin %s", tc.name, s, ok, tc.heading, tc.origin)
		}
	}
}

func TestResolveNamesTheKeyAndThePin(t *testing.T) {
	doc := mustResolve(t, rvProject(), rvBranch())
	if doc.BriefID != rvBranchID || doc.Revision != 5 || doc.Key != rvKey(rvBranchID, 5) ||
		doc.ProjectBriefID != rvProjectID || doc.ProjectRevision != 5 {
		t.Errorf("branch document = %s %d %s, project %s %d", doc.BriefID, doc.Revision, doc.Key, doc.ProjectBriefID, doc.ProjectRevision)
	}
	doc = mustResolve(t, rvProject(), nil)
	if doc.BriefID != rvProjectID || doc.Revision != 5 || doc.Key != rvKey(rvProjectID, 5) ||
		doc.ProjectBriefID != "" || doc.ProjectRevision != 0 {
		t.Errorf("project document = %s %d %s, project %q %d", doc.BriefID, doc.Revision, doc.Key, doc.ProjectBriefID, doc.ProjectRevision)
	}
	for _, s := range doc.Sections {
		if s.Origin != OriginProject {
			t.Errorf("project alone: section %s origin %s", s.Key, s.Origin)
		}
	}

	// Every question skipped: a complete revision with nothing to show.
	empty := rvBranch()
	empty.Content.Inherits = nil
	empty.Content.Answers = []Answer{rvSkip(KeyPurpose), rvSkip(KeyAudience), rvSkip(KeyConstraints), rvSkip(KeyDone), rvSkip(KeyOutOfScope)}
	doc = mustResolve(t, nil, empty)
	if doc.Sections == nil || len(doc.Sections) != 0 {
		t.Errorf("every question skipped: sections = %#v, want []", doc.Sections)
	}
}

func TestResolveRefusesAMalformedRevision(t *testing.T) {
	wrongScope := rvProject()
	wrongScope.Scope = ScopeBranch
	branchAsProject := rvBranch()
	branchAsProject.Scope = ScopeProject
	noID := rvProject()
	noID.BriefID = ""
	spaced := rvProject()
	spaced.BriefID = "bf-0000 0000000000b7"
	angled := rvProject()
	angled.BriefID = "bf-<!--b7"
	zero := rvProject()
	zero.Number = 0
	withID := func(id string) *Revision {
		r := rvProject()
		r.BriefID = id
		return r
	}
	// A project revision's envelope has branchRef "" (Hub contract §3.3,
	// §7.5 :759); a branch revision's is "refs/heads/<name>", name non-empty
	// with no ASCII control character, space or DEL (§3.3; the Hub's own
	// CHECK is 'refs/heads/_%'). The title is that ref minus "refs/heads/",
	// on one line.
	projectOnABranch := rvProject()
	projectOnABranch.BranchRef = "refs/heads/main"
	withRef := func(ref string) *Revision {
		r := rvBranch()
		r.BranchRef = ref
		return r
	}

	cases := []struct {
		name            string
		project, branch *Revision
	}{
		{"nothing at all", nil, nil},
		{"a project revision with branch scope", wrongScope, nil},
		{"a branch revision with project scope", nil, branchAsProject},
		{"no brief id", noID, nil},
		{"a brief id with a space", spaced, nil},
		{"a brief id with a comment opener", angled, nil},
		{`a brief id with ">"`, withID("bf->b7"), nil},
		{"a brief id with a tab", withID("bf-\tb7"), nil},
		{"a brief id with a line break", withID("bf-\nb7"), nil},
		{"a brief id with a no-break space", withID("bf- b7"), nil},
		{"revision 0", zero, nil},
		{"a project revision with a branch ref", projectOnABranch, nil},
		{"a branch revision with no branch ref", rvProject(), withRef("")},
		{`a branch ref of "refs/heads/" alone`, rvProject(), withRef("refs/heads/")},
		{"a branch ref that is a bare name", rvProject(), withRef("evidence-upload")},
		{"a tag ref", rvProject(), withRef("refs/tags/v1")},
		{"a branch ref with a line break", rvProject(), withRef("refs/heads/evidence\nupload")},
		{"a branch ref with DEL", rvProject(), withRef("refs/heads/evidence\x7fupload")},
		{"a branch ref with a tab", rvProject(), withRef("refs/heads/evidence\tupload")},
		{"a branch ref with a carriage return", rvProject(), withRef("refs/heads/evidence\rupload")},
		{"a branch ref with 0x1f", rvProject(), withRef("refs/heads/evidence\x1fupload")},
		{"a branch ref with a space", rvProject(), withRef("refs/heads/evidence upload")},
		{"a branch ref ending in a line break", rvProject(), withRef("refs/heads/evidence-upload\n")},
	}
	for _, tc := range cases {
		doc, err := Resolve(tc.project, tc.branch)
		if !errors.Is(err, errBadRevision) || !reflect.DeepEqual(doc, Document{}) {
			t.Errorf("%s: Resolve = %+v, %v; want a zero Document and errBadRevision", tc.name, doc, err)
		}
		if errors.Is(err, ErrInheritMismatch) || errors.Is(err, ErrIncomplete) {
			t.Errorf("%s: err = %v, want a malformed-revision fault", tc.name, err)
		}
	}

	// The same revisions with a well-formed ref and id resolve: the refusals
	// above are the ref's and the id's, not the pairing's.
	for _, ref := range []string{"refs/heads/a", "refs/heads/café/upload", "refs/heads/<!--x"} {
		if _, err := Resolve(rvProject(), withRef(ref)); err != nil {
			t.Errorf("branch ref %q: %v", ref, err)
		}
	}
	if _, err := Resolve(withID("bf-!b7"), nil); err != nil {
		t.Errorf(`brief id "bf-!b7": %v`, err)
	}
}

// MarkerSafe is the id rule's only copy: Resolve refuses a revision whose
// BriefID fails it, and claudemd's begin-marker parser accepts exactly the
// ids that pass it (review I1; claudemd's TestEveryResolvedIDReadsBack).
func TestMarkerSafe(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"bf-00000000000000a2", true},
		{"bf-!a2", true},    // the rule is the marker's, not the Hub's id grammar
		{"bf-\x01a2", true}, // a control byte that is not white space reads back
		{"", false},
		{"bf a2", false},
		{"bf\ta2", false},
		{"bf\na2", false},
		{"bf\ra2", false},
		{"bf a2", false},
		{"bf a2", false},
		{"bf-<a2", false},
		{"bf->a2", false},
		{"bf-->a2", false},
	} {
		if got := MarkerSafe(tc.id); got != tc.want {
			t.Errorf("MarkerSafe(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// A follow-up's heading is its prompt trimmed, minus one final "?"; the key
// itself when that leaves nothing (part As built 9; review I2, RV7 and RV8).
func TestFollowUpHeadingIsTrimmedOrTheKey(t *testing.T) {
	project := rvProject()
	project.Content.Questions = append(project.Content.Questions,
		Question{Key: "f-0000000p", ProjectPrompt: "  Which regulator reads it?  ", BranchPrompt: "Not this", Options: []string{}, FollowUp: true})
	project = rvSet(project, rvPerson("f-0000000p", "APRA"))
	branch := rvBranch()
	branch.Content.Questions = append(branch.Content.Questions,
		Question{Key: "f-0000000d", BranchPrompt: "  Is there a deadline?  ", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000q", BranchPrompt: "?", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000w", BranchPrompt: "  ?  ", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000e", BranchPrompt: "", Options: []string{}, FollowUp: true})
	for _, k := range []Key{"f-0000000d", "f-0000000q", "f-0000000w", "f-0000000e"} {
		branch = rvSet(branch, rvPerson(k, "yes"))
	}
	// A key both revisions hold, where the revision the section shows has no
	// wording: the key, not the other scope's wording (headingFor asks the
	// other revision only when the shown one lacks the question).
	project.Content.Questions = append(project.Content.Questions,
		Question{Key: "f-0000000s", ProjectPrompt: "Which team owns it?", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000t", ProjectPrompt: " ? ", Options: []string{}, FollowUp: true})
	project = rvSet(rvSet(project, rvPerson("f-0000000s", "Audit")), rvPerson("f-0000000t", "Audit"))
	branch.Content.Questions = append(branch.Content.Questions,
		Question{Key: "f-0000000s", BranchPrompt: "?", Options: []string{}, FollowUp: true},
		Question{Key: "f-0000000t", BranchPrompt: "Which team owns this branch?", Options: []string{}, FollowUp: true})
	branch = rvSet(rvSet(branch, rvPerson("f-0000000s", "Payments")), rvSkip("f-0000000t"))

	doc := mustResolve(t, project, branch)
	for k, want := range map[Key]string{
		"f-0000000p": "Which regulator reads it",
		"f-0000000d": "Is there a deadline",
		"f-0000000q": "f-0000000q",
		"f-0000000w": "f-0000000w",
		"f-0000000e": "f-0000000e",
		"f-0000000s": "f-0000000s", // an override: the branch's "?" is shown
		"f-0000000t": "f-0000000t", // inherited: the project's " ? " is shown
	} {
		if s, ok := rvSection(t, doc, k); !ok || s.Heading != want {
			t.Errorf("%s: heading %q (found %v), want %q", k, s.Heading, ok, want)
		}
	}
}
