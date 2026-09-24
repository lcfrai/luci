// Package discover turns intent into durable context: the Brief. Its content
// object is what phase 4 writes into every Hub brief revision, and a
// revision is immutable and kept for ever - so this file is a WIRE FORMAT.
// Field names, JSON tags and the rule "every slice marshals as [], never
// null" are part of every digest ever computed. Change them only by adding a
// new ContentVersion that every reader also understands.
package discover

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ContentVersion is the version this build writes. Readers accept every
// version <= ContentVersion; v1 is the only one.
const ContentVersion = 1

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeBranch  Scope = "branch"
)

// Key identifies one question. Base keys are fixed; a follow-up's key is
// "f-" + 8 hex, minted when the follow-up is added.
type Key string

const (
	KeyPurpose     Key = "purpose"
	KeyAudience    Key = "audience"
	KeyConstraints Key = "constraints"
	KeyDone        Key = "done"
	KeyOutOfScope  Key = "out_of_scope"
)

// Status is derived, never set by hand: complete exactly when every question
// has a settled answer. The Hub also accepts "skipped"; v1 never writes it.
type Status string

const (
	StatusOpen     Status = "open"
	StatusComplete Status = "complete"
)

type Source string

const (
	SourcePicked  Source = "picked"
	SourceTyped   Source = "typed"
	SourceDrafted Source = "drafted"
)

// Author says who wrote an answer (4-D2). A model_drafted answer is held:
// it is shown, marked, and NOT settled, so a revision holding one is never
// complete and is never inherited, pinned or approved against.
type Author string

const (
	AuthorPerson        Author = "person"
	AuthorModelDrafted  Author = "model_drafted"
	AuthorModelAccepted Author = "model_accepted" // a person accepted the model's text unchanged
)

// Question is one step of the flow. Options are pickable answers; the
// free-text escape is always available and is not modelled here.
type Question struct {
	Key           Key      `json:"key"`
	ProjectPrompt string   `json:"projectPrompt"`
	BranchPrompt  string   `json:"branchPrompt"`
	Help          string   `json:"help"`
	Options       []string `json:"options"`
	FollowUp      bool     `json:"followUp"`
	Because       string   `json:"because"` // a follow-up's reason, e.g. "Suggested because of your answer to …"; "" for base questions
}

// Prompt returns the wording for a scope: the project wording on a project
// brief when there is one, the branch wording otherwise (the 2026-09-18
// plan's rule, :861-866).
func (q Question) Prompt(s Scope) string {
	if s == ScopeProject && q.ProjectPrompt != "" {
		return q.ProjectPrompt
	}
	return q.BranchPrompt
}

// BaseQuestions is the closed set, word for word from the 2026-09-18 plan
// (:872-935). Closed on purpose: "QUESTION 2 OF 5" is honest only because
// this list cannot grow mid-flow. Model-proposed follow-ups are appended and
// shown without a denominator (docs/AGENTIC-UX.md anti-pattern 2). Every call
// returns a fresh copy.
func BaseQuestions() []Question {
	return []Question{
		{
			Key:           KeyPurpose,
			ProjectPrompt: "What is this project for?",
			BranchPrompt:  "What is this branch for?",
			Help:          "One sentence. The first thing every agent here reads.",
			Options: []string{
				"Add a capability we do not have yet",
				"Fix something broken",
				"Change how something works",
				"Try an idea we might throw away",
			},
		},
		{
			Key:           KeyAudience,
			ProjectPrompt: "Who is this project for?",
			BranchPrompt:  "Who is this branch for?",
			Help:          "Agents make different choices for an internal tool than a client-facing one.",
			Options: []string{
				"Our own team",
				"KPMG staff broadly",
				"Client users",
				"Nobody yet — a spike",
			},
		},
		{
			Key:           KeyConstraints,
			ProjectPrompt: "What must this project never break?",
			BranchPrompt:  "What must this branch not break?",
			Help:          "Agents will refuse work that violates this instead of guessing.",
			Options: []string{
				"The published API",
				"Existing data and migrations",
				"The deployed environment",
				"Nothing — it is isolated",
			},
		},
		{
			Key:           KeyDone,
			ProjectPrompt: "What has to be true before work here is finished?",
			BranchPrompt:  "What has to be true before this branch can merge?",
			Help:          "This becomes the definition of done, shown again at merge.",
			Options: []string{
				"It has tests that pass",
				"It does not break the published API",
				"It works on mobile",
				"Someone else has reviewed it",
			},
		},
		{
			Key:           KeyOutOfScope,
			ProjectPrompt: "What is explicitly out of scope for this project?",
			BranchPrompt:  "What is explicitly out of scope for this branch?",
			Help:          "Naming this is how you stop an agent widening the job while you are away.",
			Options: []string{
				"Performance work",
				"Visual design",
				"Migrations and data changes",
				"Nothing is off the table",
			},
		},
	}
}

// Criterion is one line of a definition of done. ID is minted once and kept
// for as long as a line with exactly this Text survives edits (R6); phase 3
// keys evidence by brief key + ID, and by the text digest provenance adds.
type Criterion struct {
	ID   string `json:"id"` // "c-" + 8 hex
	Text string `json:"text"`
}

type Answer struct {
	Key         Key         `json:"key"`
	Text        string      `json:"text"` // "" exactly when Skipped
	Skipped     bool        `json:"skipped"`
	Source      Source      `json:"source"` // "" when Skipped
	Author      Author      `json:"author"`
	Criteria    []Criterion `json:"criteria"`    // KeyDone only; [] for every other key and when skipped
	DraftedFrom []string    `json:"draftedFrom"` // repo on-ramp: the manifest and directory names the draft was read from; [] otherwise
	AnsweredAt  string      `json:"answeredAt"`  // RFC 3339 UTC, exactly three fractional digits (Hub contract §1)
}

// Settled reports whether an answer counts towards completion: skipped, or
// text from a person or accepted by one.
func (a Answer) Settled() bool {
	if a.Skipped {
		return true
	}
	return a.Text != "" && (a.Author == AuthorPerson || a.Author == AuthorModelAccepted)
}

// Inherit pins the project revision a branch revision resolves against
// (4-D3). With it, a branch key names immutable resolved text.
type Inherit struct {
	Key    string `json:"key"`    // "brief:bf-…@n"
	Digest string `json:"digest"` // that revision's Hub digest
}

type ChangeKind string

const (
	ChangeRecord          ChangeKind = "record"
	ChangeEdit            ChangeKind = "edit"
	ChangeSkip            ChangeKind = "skip"
	ChangeAccept          ChangeKind = "accept"
	ChangeAddFollowUp     ChangeKind = "add_follow_up"
	ChangeDismissFollowUp ChangeKind = "dismiss_follow_up"
	ChangeRestore         ChangeKind = "restore"
	ChangeReinherit       ChangeKind = "reinherit"
	ChangeDraft           ChangeKind = "draft"
)

// Change records what produced a revision. It is read by the history view
// and never rendered into CLAUDE.md.
type Change struct {
	Kind     ChangeKind `json:"kind"`
	Key      Key        `json:"key"`      // "" for restore, reinherit, draft
	Restores int        `json:"restores"` // restore: the revision whose answers came back; 0 otherwise
}

// Content is the Hub revision's content object, version 1.
type Content struct {
	V         int        `json:"v"`
	Status    Status     `json:"status"`
	Questions []Question `json:"questions"`
	Answers   []Answer   `json:"answers"`
	Inherits  *Inherit   `json:"inherits"` // branch: the project's latest complete revision when written, or null; project: always null
	Change    Change     `json:"change"`
}

// NewContent is the content before the first op: the base questions, no
// answers, status open. It is never written itself - revision 1 is the
// first op applied to it.
func NewContent() Content {
	return Content{
		V:         ContentVersion,
		Status:    StatusOpen,
		Questions: BaseQuestions(),
		Answers:   []Answer{},
	}
}

var (
	ErrUnknownVersion = errors.New("discover: content version is newer than this build")
	// errNoVersion: a content object with no "v", or one below 1. No build
	// ever wrote one, so it is refused rather than read as v1.
	errNoVersion = errors.New("discover: content has no version")
)

// Decode reads a content object from any version this build understands.
// An unknown higher version is ErrUnknownVersion, never a guess. A null or
// missing slice reads as [], so a decoded head compares and marshals like
// one this build wrote.
func Decode(raw []byte) (Content, error) {
	var probe struct {
		V *int `json:"v"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Content{}, fmt.Errorf("discover: content is not a JSON object: %w", err)
	}
	switch {
	case probe.V == nil || *probe.V < 1:
		return Content{}, errNoVersion
	case *probe.V > ContentVersion:
		return Content{}, fmt.Errorf("%w (v%d, this build reads up to v%d)", ErrUnknownVersion, *probe.V, ContentVersion)
	}
	var c Content
	if err := json.Unmarshal(raw, &c); err != nil {
		return Content{}, fmt.Errorf("discover: content v%d does not decode: %w", *probe.V, err)
	}
	fillSlices(&c)
	return c, nil
}

// fillSlices replaces every nil slice with an empty one, in place.
func fillSlices(c *Content) {
	if c.Questions == nil {
		c.Questions = []Question{}
	}
	if c.Answers == nil {
		c.Answers = []Answer{}
	}
	for i := range c.Questions {
		if c.Questions[i].Options == nil {
			c.Questions[i].Options = []string{}
		}
	}
	for i := range c.Answers {
		if c.Answers[i].Criteria == nil {
			c.Answers[i].Criteria = []Criterion{}
		}
		if c.Answers[i].DraftedFrom == nil {
			c.Answers[i].DraftedFrom = []string{}
		}
	}
}

// AnswerFor returns the answer held for a key, settled or not.
func (c Content) AnswerFor(k Key) (Answer, bool) {
	for _, a := range c.Answers {
		if a.Key == k {
			return a, true
		}
	}
	return Answer{}, false
}

// Pending lists the model_drafted answers awaiting a person, in question
// order. Never nil.
func (c Content) Pending() []Key {
	out := []Key{}
	for _, q := range c.Questions {
		if a, ok := c.AnswerFor(q.Key); ok && !a.Settled() && a.Author == AuthorModelDrafted {
			out = append(out, q.Key)
		}
	}
	return out
}

type StepKind string

const (
	StepQuestion StepKind = "question"
	StepFollowUp StepKind = "follow_up"
	StepReview   StepKind = "review" // only drafted answers remain; accept or change each
	StepComplete StepKind = "complete"
)

// Step is what the flow shows next. Total is the size of the closed base set
// for StepQuestion and ZERO otherwise; the only legal rendering of it is the
// literal "QUESTION n OF m" (G5). Ordinal is likewise zero for every kind
// but StepQuestion. Pending is c.Pending() on every kind, so a question step
// can say drafts are waiting; Options and Pending are never nil.
type Step struct {
	Kind    StepKind `json:"kind"`
	Ordinal int      `json:"ordinal"`
	Total   int      `json:"total"`
	Key     Key      `json:"key"`
	Prompt  string   `json:"prompt"`
	Help    string   `json:"help"`
	Options []string `json:"options"`
	Because string   `json:"because"`
	Pending []Key    `json:"pending"`
}

// Step returns the next thing to show. It walks the questions in order and
// stops at the first whose answer is neither settled nor a pending draft; a
// model_drafted answer does not stop the walk (it waits for review) and does
// not count as settled. With no such question left it is StepReview while
// drafts are pending, and StepComplete after - so StepComplete holds exactly
// when the derived Status is complete.
func (c Content) Step(s Scope) Step {
	pending := c.Pending()
	base := 0
	for _, q := range c.Questions {
		if !q.FollowUp {
			base++
		}
	}
	ordinal := 0
	for _, q := range c.Questions {
		if !q.FollowUp {
			ordinal++
		}
		if a, ok := c.AnswerFor(q.Key); ok && (a.Settled() || a.Author == AuthorModelDrafted) {
			continue
		}
		step := Step{Kind: StepQuestion, Ordinal: ordinal, Total: base, Key: q.Key,
			Prompt: q.Prompt(s), Help: q.Help, Options: append([]string{}, q.Options...),
			Because: q.Because, Pending: pending}
		if q.FollowUp {
			step.Kind, step.Ordinal, step.Total = StepFollowUp, 0, 0
		}
		return step
	}
	if len(pending) > 0 {
		return Step{Kind: StepReview, Options: []string{}, Pending: pending}
	}
	return Step{Kind: StepComplete, Options: []string{}, Pending: pending}
}

// SplitCriteria turns a definition-of-done answer into discrete items.
// Deterministic: split on newlines and semicolons, strip bullet markers, drop
// empties. No model call - the merge checklist must not drift between reads.
// The 2026-09-18 plan's (:1186-1202), unchanged.
func SplitCriteria(text string) []string {
	rough := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ';' })
	items := make([]string, 0, len(rough))
	for _, part := range rough {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "- ")
		part = strings.TrimPrefix(part, "* ")
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

// FormatTime is the one time format inside content: UTC, exactly three
// fractional digits (Hub contract §1).
func FormatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
