package discover

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type OpKind string

const (
	OpRecord          OpKind = "record"            // Key, Text, Source (picked | typed)
	OpEdit            OpKind = "edit"              // Key, Text — a settled, non-skipped answer, in place
	OpSkip            OpKind = "skip"              // Key
	OpAccept          OpKind = "accept"            // Key — model_drafted -> model_accepted, text unchanged
	OpAddFollowUp     OpKind = "add_follow_up"     // Question (its Key is minted here)
	OpDismissFollowUp OpKind = "dismiss_follow_up" // Key — a follow-up; removes the question and its answer
	OpRestore         OpKind = "restore"           // Restore, RestoreRevision — questions and answers of an earlier revision
	OpReinherit       OpKind = "reinherit"         // branch only; the 4-D3 fan-out
	OpDraft           OpKind = "draft"             // Drafts — the repo on-ramp
)

// Op is one change a person (or the fan-out, or the on-ramp) asked for. The
// JSON tags are the wire shape the desktop sends inside POST …/brief/ops
// (task 11); Restore and Drafts are filled by the server, never the client.
type Op struct {
	Kind            OpKind    `json:"kind"`
	Key             Key       `json:"key,omitempty"`
	Text            string    `json:"text,omitempty"`
	Source          Source    `json:"source,omitempty"`
	RestoreRevision int       `json:"restoreRevision,omitempty"`
	Question        *Question `json:"-"`
	Restore         *Content  `json:"-"`
	Drafts          []Answer  `json:"-"`
}

// Env is what Apply needs from outside and cannot compute.
type Env struct {
	Scope Scope
	// Inherits is the project brief's latest complete revision NOW. Set for
	// every op on a branch brief, so each branch revision pins what it
	// resolves against; nil for a project brief or when none is complete.
	Inherits *Inherit
	Now      time.Time
	NewID    func(prefix string) string // ids.New in production; nil is a server fault
}

var (
	ErrUnknownKey  = errors.New("discover: no question with that key")
	ErrEmptyAnswer = errors.New("discover: an answer cannot be empty")
	ErrNotAnswered = errors.New("discover: that question has no answer to edit")
	ErrNotDrafted  = errors.New("discover: that answer was not drafted by the model")
	ErrNotFollowUp = errors.New("discover: that question is not a follow-up")
	ErrBadOp       = errors.New("discover: that change is not valid here")
	ErrNoChange    = errors.New("discover: that change leaves the brief as it was")
	// ErrReservedText: a text or prompt the op added or changed contains
	// ReservedMarker, the sequence the managed block's markers are made of
	// (plan §4 Security, S3). Apply returns it wrapped in *ReservedTextError,
	// so errors.Is(err, ErrReservedText) holds and the key is recoverable.
	ErrReservedText = errors.New("discover: text cannot contain the reserved sequence panacea:discover:")
	// ErrTooLong: an answer text the op added or changed is over
	// MaxAnswerRunes, or a prompt, help or option over MaxPromptRunes (rule
	// 13, review M6, plan §4 Security, S4). Apply returns it wrapped in
	// *TooLongError, so errors.Is(err, ErrTooLong) holds and the key is
	// recoverable.
	ErrTooLong = errors.New("discover: text is too long")
)

// Server faults, not a person's: unexported, so the API layer maps them to a
// 500 rather than a 400 that blames the request.
var (
	errBadScope  = errors.New("discover: env scope is neither project nor branch")
	errNoFreshID = errors.New("discover: could not mint an unused id")
	errNoNewID   = errors.New("discover: env has no NewID")
)

// mintAttempts bounds the retries when NewID returns an id already in use.
// With 32 random bits a second attempt is already vanishingly rare.
const mintAttempts = 8

// The length caps (review M6), counted in Unicode code points
// (utf8.RuneCountInString) on the string as stored (rule 14's LF, then rule
// 1's trim for an answer; prompts, help and options are not trimmed; rule
// 13): the unit JSON Schema's maxLength uses, so task 04's and task 13's
// schemas (200/120 and 400) sit inside them and model text never meets the
// cap. They make the Hub's 64 KiB envelope refusal (hub-contract.md:751 at
// hub 53583f2) unreachable for ordinary text and give a person a refusal
// that names the answer; task 08's size check is the bound itself (S4).
const (
	MaxAnswerRunes = 4000
	MaxPromptRunes = 300
)

// TooLongError names the question whose text is over its cap, and the cap,
// so the 400 can point at the answer to shorten (task 11 puts Key and Limit
// in details; task 18 shows the message at that answer).
type TooLongError struct {
	Key   string
	Limit int // MaxAnswerRunes or MaxPromptRunes
}

func (e *TooLongError) Error() string {
	return ErrTooLong.Error() + " (question " + e.Key + ", at most " + strconv.Itoa(e.Limit) + " characters)"
}
func (e *TooLongError) Unwrap() error { return ErrTooLong }

// ReservedTextError names the question whose prompt, answer or criterion
// holds ReservedMarker, so the 400 can point at the answer to fix (task 11
// puts Key in details.key; task 18 shows the message at that answer).
type ReservedTextError struct{ Key string }

func (e *ReservedTextError) Error() string {
	return ErrReservedText.Error() + " (question " + e.Key + ")"
}
func (e *ReservedTextError) Unwrap() error { return ErrReservedText }

// ReservedMarker is the prefix of every managed-block marker task 05 writes
// (`<!-- panacea:discover:begin …-->`, `<!-- panacea:discover:end -->`). No
// question prompt, answer text or criterion text may contain it.
const ReservedMarker = "panacea:discover:"

// Apply returns the content after op. It never mutates c, op or env, and the
// result shares no memory with any of them. It sets V, the
// scope-appropriate Inherits, Change, and the derived Status on every
// result. It returns ErrNoChange - and the caller writes no revision - when
// questions, answers (ignoring AnsweredAt) and Inherits are all unchanged.
// On any error the returned Content is the zero value.
//
// Order: the op runs on a deep copy (rules 1-10, with rule 14 applied to an
// answer's text before rule 1's trim); then rule 14 over every other string
// the op added or changed; then rule 12 over all of those, then rule 13; then
// finish (rule 11); then the no-change check.
func Apply(c Content, op Op, env Env) (Content, error) {
	if env.Scope != ScopeProject && env.Scope != ScopeBranch {
		return Content{}, fmt.Errorf("%w: %q", errBadScope, env.Scope)
	}
	if env.NewID == nil {
		return Content{}, errNoNewID
	}
	orig := clone(c)
	n := clone(c)
	change, err := applyOp(&n, op, env)
	if err != nil {
		return Content{}, err
	}
	normaliseChanged(&orig, &n)
	if err := checkChanged(&orig, &n); err != nil {
		return Content{}, err
	}
	finish(&n, env, change)
	if sameContent(&orig, &n) {
		return Content{}, ErrNoChange
	}
	return n, nil
}

// applyOp runs one op on n, which Apply owns, and says what it did.
func applyOp(n *Content, op Op, env Env) (Change, error) {
	switch op.Kind {
	case OpRecord:
		if op.Source != SourcePicked && op.Source != SourceTyped {
			return Change{}, fmt.Errorf("%w: a person's answer is picked or typed, not %q", ErrBadOp, op.Source)
		}
		return Change{Kind: ChangeRecord, Key: op.Key}, setText(n, op.Key, op.Text, op.Source, AuthorPerson, nil, env)

	case OpEdit:
		if _, ok := n.questionIndex(op.Key); !ok {
			return Change{}, fmt.Errorf("%w: %q", ErrUnknownKey, op.Key)
		}
		if a, ok := n.AnswerFor(op.Key); !ok || a.Skipped || !a.Settled() {
			return Change{}, fmt.Errorf("%w: %q", ErrNotAnswered, op.Key)
		}
		return Change{Kind: ChangeEdit, Key: op.Key}, setText(n, op.Key, op.Text, SourceTyped, AuthorPerson, nil, env)

	case OpSkip:
		if _, ok := n.questionIndex(op.Key); !ok {
			return Change{}, fmt.Errorf("%w: %q", ErrUnknownKey, op.Key)
		}
		n.put(Answer{Key: op.Key, Skipped: true, Author: AuthorPerson,
			Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: FormatTime(env.Now)})
		return Change{Kind: ChangeSkip, Key: op.Key}, nil

	case OpAccept:
		if _, ok := n.questionIndex(op.Key); !ok {
			return Change{}, fmt.Errorf("%w: %q", ErrUnknownKey, op.Key)
		}
		i, ok := n.answerIndex(op.Key)
		if !ok || n.Answers[i].Author != AuthorModelDrafted {
			return Change{}, fmt.Errorf("%w: %q", ErrNotDrafted, op.Key)
		}
		n.Answers[i].Author = AuthorModelAccepted
		return Change{Kind: ChangeAccept, Key: op.Key}, nil

	case OpAddFollowUp:
		if op.Question == nil {
			return Change{}, fmt.Errorf("%w: add_follow_up carries no question", ErrBadOp)
		}
		q := *op.Question
		q.Options = append([]string{}, q.Options...)
		q.FollowUp = true
		if strings.TrimSpace(q.Prompt(env.Scope)) == "" {
			return Change{}, fmt.Errorf("%w: a follow-up needs a prompt for a %s brief", ErrBadOp, env.Scope)
		}
		id, err := mint(env.NewID, "f", func(id string) bool {
			_, inQ := n.questionIndex(Key(id))
			_, inA := n.answerIndex(Key(id))
			return inQ || inA
		})
		if err != nil {
			return Change{}, err
		}
		q.Key = Key(id)
		n.Questions = append(n.Questions, q)
		return Change{Kind: ChangeAddFollowUp, Key: q.Key}, nil

	case OpDismissFollowUp:
		qi, ok := n.questionIndex(op.Key)
		if !ok {
			return Change{}, fmt.Errorf("%w: %q", ErrUnknownKey, op.Key)
		}
		if !n.Questions[qi].FollowUp {
			return Change{}, fmt.Errorf("%w: %q", ErrNotFollowUp, op.Key)
		}
		n.Questions = append(n.Questions[:qi], n.Questions[qi+1:]...)
		if ai, ok := n.answerIndex(op.Key); ok {
			n.Answers = append(n.Answers[:ai], n.Answers[ai+1:]...)
		}
		return Change{Kind: ChangeDismissFollowUp, Key: op.Key}, nil

	case OpRestore:
		if op.Restore == nil || op.RestoreRevision < 1 {
			return Change{}, fmt.Errorf("%w: restore needs the revision and its content", ErrBadOp)
		}
		r := clone(*op.Restore)
		n.Questions, n.Answers = r.Questions, r.Answers // Inherits: finish takes env's (rule 8)
		return Change{Kind: ChangeRestore, Restores: op.RestoreRevision}, nil

	case OpReinherit:
		if env.Scope != ScopeBranch {
			return Change{}, fmt.Errorf("%w: only a branch brief inherits", ErrBadOp)
		}
		return Change{Kind: ChangeReinherit}, nil // finish sets Inherits from env

	case OpDraft:
		for _, d := range op.Drafts {
			if _, ok := n.questionIndex(d.Key); !ok {
				return Change{}, fmt.Errorf("%w: %q", ErrUnknownKey, d.Key)
			}
			if a, ok := n.AnswerFor(d.Key); ok && a.Settled() {
				continue // never over a person's answer, or one a person accepted (rule 10)
			}
			if err := setText(n, d.Key, d.Text, SourceDrafted, AuthorModelDrafted, d.DraftedFrom, env); err != nil {
				return Change{}, err
			}
		}
		return Change{Kind: ChangeDraft}, nil
	}
	return Change{}, fmt.Errorf("%w: unknown op kind %q", ErrBadOp, op.Kind)
}

// setText writes one non-skipped answer: rule 14 then rule 1's trim, the
// done criteria (rule 5), in place or appended (defect 7).
func setText(n *Content, key Key, raw string, src Source, author Author, from []string, env Env) error {
	if _, ok := n.questionIndex(key); !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	text := strings.TrimSpace(toLF(raw))
	if text == "" {
		return fmt.Errorf("%w (question %s)", ErrEmptyAnswer, key)
	}
	a := Answer{Key: key, Text: text, Source: src, Author: author,
		Criteria: []Criterion{}, DraftedFrom: append([]string{}, from...), AnsweredAt: FormatTime(env.Now)}
	if key == KeyDone {
		var prev []Criterion
		if old, ok := n.AnswerFor(key); ok {
			prev = old.Criteria
		}
		crit, err := criteriaFor(text, prev, env.NewID)
		if err != nil {
			return err
		}
		if len(crit) == 0 {
			return fmt.Errorf("%w: the definition of done holds no item (question %s)", ErrEmptyAnswer, key)
		}
		a.Criteria = crit
	}
	n.put(a)
	return nil
}

// criteriaFor is rule 5 (R6): each item of the new text keeps the id of an
// unused previous criterion with exactly its text; every other item gets a
// fresh id, never one the previous criteria held, so evidence recorded
// against a sentence never attaches to a different one. Order follows the
// new text.
func criteriaFor(text string, prev []Criterion, newID func(string) string) ([]Criterion, error) {
	items := SplitCriteria(text)
	out := make([]Criterion, len(items))
	used := make([]bool, len(prev))
	taken := map[string]bool{}
	for _, p := range prev {
		taken[p.ID] = true
	}
	for i, item := range items {
		out[i].Text = item
		for j, p := range prev {
			if !used[j] && p.Text == item {
				used[j] = true
				out[i].ID = p.ID
				break
			}
		}
	}
	for i := range out {
		if out[i].ID != "" {
			continue
		}
		id, err := mint(newID, "c", func(id string) bool { return taken[id] })
		if err != nil {
			return nil, err
		}
		taken[id] = true
		out[i].ID = id
	}
	return out, nil
}

// mint asks newID for an id that inUse rejects at most mintAttempts times.
func mint(newID func(string) string, prefix string, inUse func(string) bool) (string, error) {
	for i := 0; i < mintAttempts; i++ {
		if id := newID(prefix); !inUse(id) {
			return id, nil
		}
	}
	return "", fmt.Errorf("%w (prefix %q)", errNoFreshID, prefix)
}

func (c *Content) questionIndex(k Key) (int, bool) {
	for i, q := range c.Questions {
		if q.Key == k {
			return i, true
		}
	}
	return -1, false
}

func (c *Content) answerIndex(k Key) (int, bool) {
	for i, a := range c.Answers {
		if a.Key == k {
			return i, true
		}
	}
	return -1, false
}

// put replaces the answer with a's key in place, or appends it; it never
// touches another answer (known defect 7).
func (c *Content) put(a Answer) {
	if i, ok := c.answerIndex(a.Key); ok {
		c.Answers[i] = a
		return
	}
	c.Answers = append(c.Answers, a)
}

// clone deep-copies c: every slice of structs, each Options, Criteria and
// DraftedFrom, and the Inherits pointer. Nil slices come back empty.
func clone(c Content) Content {
	out := c
	out.Questions = make([]Question, len(c.Questions))
	for i, q := range c.Questions {
		q.Options = append([]string{}, q.Options...)
		out.Questions[i] = q
	}
	out.Answers = make([]Answer, len(c.Answers))
	for i, a := range c.Answers {
		a.Criteria = append([]Criterion{}, a.Criteria...)
		a.DraftedFrom = append([]string{}, a.DraftedFrom...)
		out.Answers[i] = a
	}
	if c.Inherits != nil {
		in := *c.Inherits
		out.Inherits = &in
	}
	return out
}

// finish is rule 11: version, the scope's Inherits (a copy of env's on a
// branch, always nil on a project), the change, no nil slice, and the status
// derived from the answers.
func finish(n *Content, env Env, change Change) {
	n.V = ContentVersion
	n.Inherits = nil
	if env.Scope == ScopeBranch && env.Inherits != nil {
		in := *env.Inherits
		n.Inherits = &in
	}
	n.Change = change
	fillSlices(n)
	n.Status = deriveStatus(n)
}

// deriveStatus: complete exactly when there are questions and every one has
// a settled answer.
func deriveStatus(c *Content) Status {
	if len(c.Questions) == 0 {
		return StatusOpen
	}
	for _, q := range c.Questions {
		if a, ok := c.AnswerFor(q.Key); !ok || !a.Settled() {
			return StatusOpen
		}
	}
	return StatusComplete
}

// toLF is rule 14: each "\r\n", then each lone "\r", becomes "\n".
func toLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func (c *Content) question(k Key) *Question {
	if i, ok := c.questionIndex(k); ok {
		return &c.Questions[i]
	}
	return nil
}

func (c *Content) answer(k Key) *Answer {
	if i, ok := c.answerIndex(k); ok {
		return &c.Answers[i]
	}
	return nil
}

// field is one string the op added or changed: the question it belongs to,
// whether rule 12 checks it, and its rule 13 cap (0: none).
type field struct {
	key    Key
	s      string
	marker bool
	limit  int
}

// visitChanged calls fn for every string in n that has no counterpart in
// orig (same question key, same answer key, same criterion id; for an
// option, the same text among that question's options) or differs from it.
// fn may rewrite the string. Because is visited for rule 14 only.
func visitChanged(orig, n *Content, fn func(f field) string) {
	for i := range n.Questions {
		q := &n.Questions[i]
		p := orig.question(q.Key)
		visit := func(s *string, was func(p *Question) string, marker bool, limit int) {
			if p == nil || was(p) != *s {
				*s = fn(field{key: q.Key, s: *s, marker: marker, limit: limit})
			}
		}
		visit(&q.ProjectPrompt, func(p *Question) string { return p.ProjectPrompt }, true, MaxPromptRunes)
		visit(&q.BranchPrompt, func(p *Question) string { return p.BranchPrompt }, true, MaxPromptRunes)
		visit(&q.Help, func(p *Question) string { return p.Help }, false, MaxPromptRunes)
		visit(&q.Because, func(p *Question) string { return p.Because }, false, 0)
		for j := range q.Options {
			if p == nil || !containsString(p.Options, q.Options[j]) {
				q.Options[j] = fn(field{key: q.Key, s: q.Options[j], limit: MaxPromptRunes})
			}
		}
	}
	for i := range n.Answers {
		a := &n.Answers[i]
		p := orig.answer(a.Key)
		if p == nil || p.Text != a.Text {
			a.Text = fn(field{key: a.Key, s: a.Text, marker: true, limit: MaxAnswerRunes})
		}
		for j := range a.Criteria {
			cr := &a.Criteria[j]
			if was := criterionByID(p, cr.ID); was == nil || was.Text != cr.Text {
				cr.Text = fn(field{key: a.Key, s: cr.Text, marker: true})
			}
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func criterionByID(a *Answer, id string) *Criterion {
	if a == nil {
		return nil
	}
	for i := range a.Criteria {
		if a.Criteria[i].ID == id {
			return &a.Criteria[i]
		}
	}
	return nil
}

// normaliseChanged is rule 14 over the strings the op added or changed; a
// string the op left alone is not rewritten, so a head the Hub stored
// unchecked stays as it was (task 05's Render normalises again at the file).
func normaliseChanged(orig, n *Content) {
	visitChanged(orig, n, func(f field) string { return toLF(f.s) })
}

// checkChanged is rule 12 over every added or changed string, then rule 13:
// a string that breaks both reports the marker.
func checkChanged(orig, n *Content) error {
	var changed []field
	visitChanged(orig, n, func(f field) string {
		changed = append(changed, f)
		return f.s
	})
	for _, f := range changed {
		if f.marker && strings.Contains(f.s, ReservedMarker) {
			return &ReservedTextError{Key: string(f.key)}
		}
	}
	for _, f := range changed {
		if f.limit > 0 && utf8.RuneCountInString(f.s) > f.limit {
			return &TooLongError{Key: string(f.key), Limit: f.limit}
		}
	}
	return nil
}

// sameContent is the no-change test: questions, answers ignoring
// AnsweredAt, and Inherits. V, Status and Change are outputs, not inputs.
func sameContent(orig, n *Content) bool {
	if !reflect.DeepEqual(orig.Questions, n.Questions) || len(orig.Answers) != len(n.Answers) {
		return false
	}
	for i := range orig.Answers {
		a, b := orig.Answers[i], n.Answers[i]
		a.AnsweredAt, b.AnsweredAt = "", ""
		if !reflect.DeepEqual(a, b) {
			return false
		}
	}
	switch {
	case orig.Inherits == nil && n.Inherits == nil:
		return true
	case orig.Inherits == nil || n.Inherits == nil:
		return false
	}
	return *orig.Inherits == *n.Inherits
}
