package discover

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Revision is one stored brief revision as this package needs it. Key is the
// Hub's key, carried opaquely: this package never parses it (P5).
type Revision struct {
	BriefID string
	Number  int
	Key     string
	Scope   Scope
	// BranchRef is the revision envelope's own branchRef (Hub contract §3.4):
	// "refs/heads/evidence-upload" for a branch revision, "" for a project one.
	// The digest covers it, and a rebind never changes it: PATCH writes no
	// revision, and only the next revision's envelope carries the new ref
	// (Hub contract §7.5). Review I5.
	BranchRef string
	Content   Content
}

// There is no Names type. The block's title is computed from the revision the
// key names and nothing else - never from the Hub project's current name
// (PATCH /v1/projects/{id} changes it) or the branch's current name (a rebind
// changes it). A render that read either would make every worktree's block
// look hand-edited after a rename (review I5; TestRenameDoesNotLookLikeAHandEdit).

type Origin string

const (
	OriginProject  Origin = "project"  // inherited unchanged
	OriginBranch   Origin = "branch"   // set on the branch; the project has no answer
	OriginOverride Origin = "override" // set on the branch, replacing the project's answer
)

type Section struct {
	Key      Key         `json:"key"`
	Heading  string      `json:"heading"`
	Body     string      `json:"body"`
	Items    []Criterion `json:"items"` // KeyDone only; [] for every other key
	Origin   Origin      `json:"origin"`
	Replaced string      `json:"replaced"` // OriginOverride: the project text replaced; "" otherwise
}

// Document is everything Render needs. It is produced by Resolve and by
// nothing else.
type Document struct {
	// Title is a function of the key alone (review I5). A branch key:
	// "Branch brief — " + the branch revision's BranchRef minus "refs/heads/"
	// ("Branch brief — evidence-upload"). A project key: "Project brief".
	// Computed by Title below, the rule's only copy (re-review b05-1 R-M2).
	// No project name and no current branch name, so a rename changes no byte
	// of any block, and every title byte is covered by the key's digest.
	Title           string    `json:"title"`
	BriefID         string    `json:"briefId"` // the revision the key names: the branch's when branch != nil
	Revision        int       `json:"revision"`
	Key             string    `json:"key"`
	ProjectBriefID  string    `json:"projectBriefId"` // the inherited project revision, when there is one
	ProjectRevision int       `json:"projectRevision"`
	Sections        []Section `json:"sections"` // never nil
}

var ErrInheritMismatch = errors.New("discover: the project revision is not the one this branch revision inherits")
var ErrIncomplete = errors.New("discover: only a complete revision can be resolved into a document")

// errBadRevision: nothing to resolve, a revision passed in the other scope's
// place, or one with no usable id or number. A caller fault (a 500), never
// the person's: the Hub never hands out such a revision.
var errBadRevision = errors.New("discover: not a revision that can be resolved")

// headings maps a base key to its section heading. Order here is the order
// in the file, and it does not depend on the order the questions were
// answered in (the 2026-09-18 plan, :1734-1743).
var headings = []struct {
	Key     Key
	Heading string
}{
	{KeyPurpose, "What this is for"},
	{KeyAudience, "Who it is for"},
	{KeyConstraints, "What must not break"},
	{KeyDone, "Definition of done"},
	{KeyOutOfScope, "Out of scope"},
}

// Resolve builds the document for exactly one key.
//
//   - branch == nil: the project revision alone; every section OriginProject.
//   - branch != nil: project MUST be the revision branch.Content.Inherits
//     names (compared by Key), or nil when Inherits is nil. Anything else is
//     ErrInheritMismatch: rendering a branch key against a project revision
//     it did not pin is how one key comes to name two texts (critique-2 §1.1).
//   - A branch answer that is Skipped inherits the project answer (OriginProject).
//   - A project answer that is Skipped, with no branch answer, is omitted.
//   - A model_drafted answer is never rendered. More generally only a
//     settled, non-skipped answer carries text into the document; anything
//     else counts as no answer, so a branch draft inherits the project's.
//   - Each revision passed must be StatusComplete, else ErrIncomplete: only a
//     complete revision is inherited (ADR-H1 Q1, extended by P4-A). The
//     stored status is what is checked; a forged one still renders no draft.
//
// Sections follow the base heading order, then follow-ups - project first,
// then branch - in the order they were added. A follow-up's heading is its
// prompt for the scope of the revision that holds it, minus a final "?";
// when both hold the key, the revision whose answer the section shows wins.
//
// An override's Replaced is the project answer's text; for the definition of
// done it is the project's criteria joined by "; ", so the note Render puts
// on one line reads as the items SplitCriteria would split again.
//
// Title follows Document.Title's rule: Resolve sets Document.Title to
// Title(branch). Resolve takes no name argument: nothing mutable can reach
// the block.
func Resolve(project, branch *Revision) (Document, error) {
	if err := checkRevisions(project, branch); err != nil {
		return Document{}, err
	}
	for _, r := range []*Revision{branch, project} {
		if r != nil && r.Content.Status != StatusComplete {
			return Document{}, fmt.Errorf("%w (%s is %s)", ErrIncomplete, r.Key, r.Content.Status)
		}
	}
	if branch != nil {
		pin := branch.Content.Inherits
		switch {
		case pin == nil && project != nil:
			return Document{}, fmt.Errorf("%w: %s pins no project revision, %s was passed", ErrInheritMismatch, branch.Key, project.Key)
		case pin != nil && project == nil:
			return Document{}, fmt.Errorf("%w: %s pins %s, none was passed", ErrInheritMismatch, branch.Key, pin.Key)
		case pin != nil && pin.Key != project.Key:
			return Document{}, fmt.Errorf("%w: %s pins %s, %s was passed", ErrInheritMismatch, branch.Key, pin.Key, project.Key)
		}
	}

	doc := Document{Title: Title(branch), Sections: []Section{}}
	if branch != nil {
		doc.BriefID, doc.Revision, doc.Key = branch.BriefID, branch.Number, branch.Key
		if project != nil {
			doc.ProjectBriefID, doc.ProjectRevision = project.BriefID, project.Number
		}
	} else {
		doc.BriefID, doc.Revision, doc.Key = project.BriefID, project.Number, project.Key
	}

	for _, k := range orderedKeys(project, branch) {
		pa, haveProject := textAnswer(project, k)
		ba, haveBranch := textAnswer(branch, k)
		var s Section
		switch {
		case haveBranch && haveProject:
			s = section(k, ba, OriginOverride)
			s.Replaced = replacedText(pa)
			s.Heading = headingFor(k, branch, project)
		case haveBranch:
			s = section(k, ba, OriginBranch)
			s.Heading = headingFor(k, branch, project)
		case haveProject:
			s = section(k, pa, OriginProject)
			s.Heading = headingFor(k, project, branch)
		default:
			continue
		}
		doc.Sections = append(doc.Sections, s)
	}
	return doc, nil
}

// Title is Document.Title's rule, and the only copy of it (re-review b05-1
// R-M2): "Branch brief — " + branch.BranchRef minus "refs/heads/" when
// branch != nil, else "Project brief". It reads nothing but BranchRef, and
// it does not look at Content.Status, so an open revision gets the same
// title its complete successor on the same ref will: provenance.Build calls
// it for an open history row, which Resolve refuses with ErrIncomplete
// (task 06). No package writes "Branch brief" or "Project brief" itself.
func Title(branch *Revision) string {
	if branch == nil {
		return "Project brief"
	}
	return "Branch brief — " + strings.TrimPrefix(branch.BranchRef, "refs/heads/")
}

// checkRevisions refuses what no Hub revision is: nothing at all, a revision
// in the other scope's place, a missing or unmarkable id, a number below 1.
// An id is marker-safe when it is non-empty and holds no space and no "<" or
// ">", so the begin marker claudemd writes for it always reads back.
func checkRevisions(project, branch *Revision) error {
	if project == nil && branch == nil {
		return fmt.Errorf("%w: no revision passed", errBadRevision)
	}
	for _, c := range []struct {
		r     *Revision
		scope Scope
	}{{project, ScopeProject}, {branch, ScopeBranch}} {
		if c.r == nil {
			continue
		}
		if c.r.Scope != c.scope {
			return fmt.Errorf("%w: %s has scope %q where a %s revision belongs", errBadRevision, c.r.Key, c.r.Scope, c.scope)
		}
		if c.r.Number < 1 || !markerSafe(c.r.BriefID) {
			return fmt.Errorf("%w: brief %q revision %d", errBadRevision, c.r.BriefID, c.r.Number)
		}
	}
	return nil
}

func markerSafe(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if unicode.IsSpace(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}

// textAnswer returns r's answer for k when it carries text into the
// document: settled and not skipped. A skip, a held draft, an answer with no
// text or no author, or no revision at all, is no answer.
func textAnswer(r *Revision, k Key) (Answer, bool) {
	if r == nil {
		return Answer{}, false
	}
	a, ok := r.Content.AnswerFor(k)
	if !ok || a.Skipped || !a.Settled() {
		return Answer{}, false
	}
	return a, true
}

func section(k Key, a Answer, o Origin) Section {
	s := Section{Key: k, Body: a.Text, Items: []Criterion{}, Origin: o}
	if k == KeyDone {
		s.Items = append(s.Items, a.Criteria...)
	}
	return s
}

func replacedText(a Answer) string {
	if a.Key == KeyDone && len(a.Criteria) > 0 {
		texts := make([]string, len(a.Criteria))
		for i, c := range a.Criteria {
			texts[i] = c.Text
		}
		return strings.Join(texts, "; ")
	}
	return a.Text
}

// orderedKeys is the base keys in file order, then every other question's
// key in the order it was added - project first, then branch.
func orderedKeys(project, branch *Revision) []Key {
	seen := map[Key]bool{}
	out := make([]Key, 0, len(headings)+4)
	for _, h := range headings {
		seen[h.Key] = true
		out = append(out, h.Key)
	}
	for _, r := range []*Revision{project, branch} {
		if r == nil {
			continue
		}
		for _, q := range r.Content.Questions {
			if !seen[q.Key] {
				seen[q.Key] = true
				out = append(out, q.Key)
			}
		}
	}
	return out
}

// headingFor is a base key's heading, else the prompt of the question that
// holds the key, for that revision's scope, minus a final "?"; the key
// itself when there is no wording. shown is the revision whose answer the
// section shows, asked first, so a key both revisions hold is headed in the
// words its text was written against; other is asked when shown lacks it.
func headingFor(k Key, shown, other *Revision) string {
	for _, h := range headings {
		if h.Key == k {
			return h.Heading
		}
	}
	for _, r := range []*Revision{shown, other} {
		if r == nil {
			continue
		}
		for _, q := range r.Content.Questions {
			if q.Key == k {
				if p := strings.TrimSuffix(strings.TrimSpace(q.Prompt(r.Scope)), "?"); p != "" {
					return p
				}
				return string(k)
			}
		}
	}
	return string(k)
}
