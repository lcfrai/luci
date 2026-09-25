### Task 02: `teamtext` — bounded text, author class, G3 hygiene, globs and breadth

**Slice:** TM1-α. **Size:** 2.5 focused hours. **dependsOn:** none. **Seams:** none defined; every
seam that carries free text or a path (ST.1, ST.2, ST.6, ST.8) uses this package.

Every string an agent or a teammate writes in team mode is read, later, by someone else's agent or
by someone else's eyes. That makes each one an injection surface, and `05-design.md` answers it
with one rule applied everywhere: **bounded, hygiene-checked, labelled with who wrote it** (§3.3,
G3 in §7.1). If each Hub route and each Fleet verb wrote its own check, one of them would forget the
Unicode tag block, and a message would carry invisible instructions (`U+E0041` is an invisible
`A`) past every human who reads it. So there is exactly one implementation, in one package with no
dependencies outside the standard library, which the Hub deltas (tasks 03 to 07, 43, 46), Fleet
(tasks 22 to 31) and the activity sweep (task 34) all call.

This package makes no judgement about meaning. It cannot tell a persuasive message from an honest
one, and it does not try (§7.3, OD-8): its checks are on characters, lengths and path shapes, all
decidable from the bytes.

Five decisions the scope left open, made here.

**(a) Length is counted in Unicode code points, not bytes.** `utf8.RuneCountInString`, because task
04's database `CHECK` uses Postgres's `char_length`, which counts characters in a UTF-8 database,
and the two layers must refuse exactly the same strings. A test pins the boundary with a
multi-byte character at position 600.

**(b) Names are stricter than bodies.** A *name* (a task title, a brief title, an option label) is
shown on one line beside a person's name, so it may not contain a newline, a tab, a run of spaces,
a leading or trailing space, or any space other than U+0020 (a no-break or ideographic space
impersonates alignment). A *body* (a message, a note, a directive) may contain `\n` and `\t`, and
nothing else below U+0020. `Clean` is the body rule; `CleanName` adds the name rule; `New` picks by
kind.

**(c) The glob grammar has three metacharacters and no escapes.** `*` (any run within one path
segment), `?` (one character, not `/`), and `**` as a **whole** segment (zero or more segments).
`[`, `]`, `{` and `}` are literals, because real repos have `app/[id]/page.tsx`, and a class syntax
would make those files impossible to lease. A glob made only of `*` and `**` segments (`**`,
`*`, `**/*`) is refused outright as `whole_repo`, so the Hub, which holds no file list, still stops
the widest claim; the 10% breadth check needs the tracked files and is Fleet's (task 23).

**(d) `Breadth` returns the widest single glob's share, not the union's.** The design caps
*each* glob at 10% of tracked files (§6.3 item 1). Twenty globs of 5% each are twenty narrow
claims, and a union measure would refuse them for the wrong reason. An empty tracked list has
breadth 0.

**(e) Secret-shaped text is a fixed table of eleven RE2 patterns.** Go's `regexp` is RE2, linear in
the input, so an attacker cannot make the check slow. The table is closed: a new pattern is a code
change with its own test. It refuses; it never redacts, because a redacted string is still a string
an agent chose to send, and the sender should hear *"refused: secret"* and stop.

**Files:**
- `control-plane/internal/teamtext/text.go` — `Author`, `Kind`, the limits, `Text`, `Bound`, `New`,
  `Error` and the three class sentinels (new)
- `control-plane/internal/teamtext/hygiene.go` — `Clean`, `CleanName`, the secret table (new)
- `control-plane/internal/teamtext/glob.go` — `ValidGlob`, `ValidPath`, `Match`, `Breadth`,
  `CheckSet` (new)
- `control-plane/internal/teamtext/text_test.go`, `hygiene_test.go`, `glob_test.go` (new)

No `go.mod` change: standard library only (`errors`, `fmt`, `regexp`, `strings`, `time`,
`unicode`, `unicode/utf8`). The module is `github.com/kpmg-au-emu/panacea-app/control-plane`, `go
1.23` (`control-plane/go.mod:1-3`, read 2026-09-25); the directive does not move.

**Interfaces:**
- Produces (consumed by tasks 03 to 07, 22 to 31, 34, 43, 46, 52):
  - `teamtext.Author` (`AuthorAgent`, `AuthorHuman`, `Valid`), `teamtext.Kind` and its thirteen
    constants, the `Max…` limits and `Limits`, `IsName`
  - `teamtext.Text` — the JSON every `text·A`/`text·H` field is on the wire (task 01, Step 6)
  - `Bound(Kind, string) error`, `Clean(string) error`, `CleanName(string) error`,
    `New(Kind, body string, Author, authorAgent, owner string, time.Time) (Text, error)`
  - `ValidGlob(string) error`, `ValidPath(string) error`, `Match(glob, path string) bool`,
    `Breadth(globs, tracked []string) float64`, `CheckSet(globs, tracked []string) error`,
    `MaxGlobs`, `MaxBreadth`
  - `*Error{Class, Rule, Offset, Detail}` with `Unwrap`; the sentinels `ErrText`, `ErrGlob`,
    `ErrBreadth`, one per Hub problem code (`text_refused`, `glob_refused`, `breadth_exceeded`)
- Consumes: nothing outside the standard library. **It must never import `hubapi`**: `hubapi`
  imports it (task 03), and the cycle would not compile.

`text.go`, complete:

```go
// Package teamtext bounds, cleans and labels every free-text field and path
// that team mode stores or delivers (05-design.md §3.3, G3). It judges
// characters, lengths and shapes, never meaning (§7.3). Standard library only;
// it must never import hubapi, which imports it.
package teamtext

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

// Author is who wrote a text: an agent (text·A) or a human (text·H).
type Author string

const (
	AuthorAgent Author = "agent"
	AuthorHuman Author = "human"
)

// Valid reports whether a is one of the two author classes.
func (a Author) Valid() bool { return a == AuthorAgent || a == AuthorHuman }

// Kind names a bounded field; its limit is Limits[kind].
type Kind string

const (
	KindTitle        Kind = "title"
	KindWhy          Kind = "why"
	KindMessage      Kind = "message"
	KindReason       Kind = "reason"
	KindQuestion     Kind = "question"
	KindDirective    Kind = "directive"
	KindHumanMessage Kind = "human_message"
	KindNote         Kind = "note"
	KindBriefTitle   Kind = "brief_title"
	KindPath         Kind = "path"
	KindOptionLabel  Kind = "option_label"
	KindConsequence  Kind = "consequence"
	KindAnswer       Kind = "answer"
)

// The limits in Unicode code points: 05-design.md §3.3, and task 01 §7.19 for
// the option label, consequence and answer. Task 04's CHECKs mirror them.
const (
	MaxTitle        = 140
	MaxWhy          = 200
	MaxMessage      = 600
	MaxReason       = 300
	MaxQuestion     = 300
	MaxDirective    = 600
	MaxHumanMessage = 2000
	MaxNote         = 1000
	MaxBriefTitle   = 120
	MaxPath         = 200
	MaxOptionLabel  = 80
	MaxConsequence  = 200
	MaxAnswer       = 300
)

// Limits maps every Kind to its limit. A kind absent here is refused.
var Limits = map[Kind]int{
	KindTitle: MaxTitle, KindWhy: MaxWhy, KindMessage: MaxMessage,
	KindReason: MaxReason, KindQuestion: MaxQuestion, KindDirective: MaxDirective,
	KindHumanMessage: MaxHumanMessage, KindNote: MaxNote, KindBriefTitle: MaxBriefTitle,
	KindPath: MaxPath, KindOptionLabel: MaxOptionLabel, KindConsequence: MaxConsequence,
	KindAnswer: MaxAnswer,
}

// IsName reports whether k is shown on one line beside a person, so that
// CleanName, not Clean, applies to it.
func IsName(k Kind) bool {
	return k == KindTitle || k == KindBriefTitle || k == KindOptionLabel
}

// The three refusal classes, one per Hub problem code: ErrText is
// text_refused, ErrGlob glob_refused, ErrBreadth breadth_exceeded.
var (
	ErrText    = errors.New("teamtext: text refused")
	ErrGlob    = errors.New("teamtext: glob refused")
	ErrBreadth = errors.New("teamtext: breadth exceeded")
)

// Rule says why a string was refused. It is the Hub's details.rule.
type Rule string

const (
	RuleEmpty       Rule = "empty"
	RuleTooLong     Rule = "too_long"
	RuleUnknownKind Rule = "unknown_kind"
	RuleInvalidUTF8 Rule = "invalid_utf8"
	RuleControl     Rule = "control"
	RuleANSI        Rule = "ansi"
	RuleBidi        Rule = "bidi"
	RuleTagBlock    Rule = "tag_block"
	RuleWhitespace  Rule = "whitespace"
	RuleSecret      Rule = "secret"
	RuleLabel       Rule = "label"
	RuleAbsolute    Rule = "absolute"
	RuleDotDot      Rule = "dotdot"
	RuleBackslash   Rule = "backslash"
	RuleSegment     Rule = "segment"
	RuleWholeRepo   Rule = "whole_repo"
	RuleTooMany     Rule = "too_many"
	RuleTooBroad    Rule = "too_broad"
)

// Error is every refusal this package makes. Offset is the code-point offset
// of the first refused character, or the index of the refused glob in a set,
// or -1. Detail names a secret pattern; it never repeats the matched text.
type Error struct {
	Class  error
	Rule   Rule
	Offset int
	Detail string
}

func (e *Error) Error() string {
	s := fmt.Sprintf("%v: %s", e.Class, e.Rule)
	if e.Offset >= 0 {
		s += fmt.Sprintf(" at %d", e.Offset)
	}
	if e.Detail != "" {
		s += " (" + e.Detail + ")"
	}
	return s
}

// Unwrap lets errors.Is(err, ErrText) (and ErrGlob, ErrBreadth) work.
func (e *Error) Unwrap() error { return e.Class }

func refuse(class error, r Rule, off int) *Error {
	return &Error{Class: class, Rule: r, Offset: off}
}

// Text is a stored free-text field and who wrote it. It is the wire shape of
// every text·A and text·H field (Phase T task 01, Step 6).
type Text struct {
	Body        string    `json:"body"`
	Author      Author    `json:"author"`
	AuthorAgent string    `json:"authorAgentId,omitempty"`
	Owner       string    `json:"ownerUserId"`
	At          time.Time `json:"at"`
}

var (
	agentIDRe = regexp.MustCompile(`^ag-[0-9a-f]{16}$`)
	userIDRe  = regexp.MustCompile(`^u-[0-9a-f]{16}$`)
)

// Bound refuses s when it is empty, is not UTF-8, or is longer than kind's
// limit in code points. An unknown kind is refused, never unbounded.
func Bound(kind Kind, s string) error {
	limit, ok := Limits[kind]
	if !ok {
		return refuse(ErrText, RuleUnknownKind, -1)
	}
	if s == "" {
		return refuse(ErrText, RuleEmpty, -1)
	}
	if !utf8.ValidString(s) {
		return refuse(ErrText, RuleInvalidUTF8, -1)
	}
	if n := utf8.RuneCountInString(s); n > limit {
		return refuse(ErrText, RuleTooLong, limit)
	}
	return nil
}

// New bounds and cleans body for kind and labels it. An agent's text names
// its ag-… id; a human's names none. owner is a u-… id; at is stored in UTC
// to the millisecond (Hub contract §1).
func New(kind Kind, body string, author Author, authorAgent, owner string, at time.Time) (Text, error) {
	if err := Bound(kind, body); err != nil {
		return Text{}, err
	}
	clean := Clean
	if IsName(kind) {
		clean = CleanName
	}
	if err := clean(body); err != nil {
		return Text{}, err
	}
	switch {
	case !author.Valid(),
		author == AuthorAgent && !agentIDRe.MatchString(authorAgent),
		author == AuthorHuman && authorAgent != "",
		!userIDRe.MatchString(owner),
		at.IsZero():
		return Text{}, refuse(ErrText, RuleLabel, -1)
	}
	return Text{Body: body, Author: author, AuthorAgent: authorAgent, Owner: owner,
		At: at.UTC().Truncate(time.Millisecond)}, nil
}
```

- [ ] **Step 1: Write the failing tests for `text.go`**

Each case names the mutant it kills: the one-line change to the implementation that the case
exists to catch. `control-plane/internal/teamtext/text_test.go`:

```go
package teamtext

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	agent = "ag-3f2a91c07b1e4d2a"
	owner = "u-0123456789abcdef"
)

var at = time.Date(2026, 9, 25, 1, 2, 3, 456789000, time.FixedZone("AEST", 10*3600))

func TestBoundEveryLimit(t *testing.T) {
	// mutant: any one limit off by one, or a kind missing from Limits.
	want := map[Kind]int{KindTitle: 140, KindWhy: 200, KindMessage: 600, KindReason: 300,
		KindQuestion: 300, KindDirective: 600, KindHumanMessage: 2000, KindNote: 1000,
		KindBriefTitle: 120, KindPath: 200, KindOptionLabel: 80, KindConsequence: 200,
		KindAnswer: 300}
	if len(Limits) != len(want) {
		t.Fatalf("Limits has %d kinds, want %d", len(Limits), len(want))
	}
	for k, n := range want {
		if err := Bound(k, strings.Repeat("a", n)); err != nil {
			t.Errorf("%s at %d: %v", k, n, err)
		}
		if err := Bound(k, strings.Repeat("a", n+1)); !isRule(err, ErrText, RuleTooLong) {
			t.Errorf("%s at %d: got %v, want too_long", k, n+1, err)
		}
	}
}

func TestBoundCountsCodePointsNotBytes(t *testing.T) {
	// mutant: len(s) instead of utf8.RuneCountInString(s); 600 "é" is 1,200 bytes.
	if err := Bound(KindMessage, strings.Repeat("é", 600)); err != nil {
		t.Fatalf("600 code points refused: %v", err)
	}
}

func TestBoundRefusesEmptyInvalidAndUnknown(t *testing.T) {
	// mutants: the empty check dropped; the UTF-8 check dropped; an unknown kind unbounded.
	if !isRule(Bound(KindNote, ""), ErrText, RuleEmpty) {
		t.Error("empty accepted")
	}
	if !isRule(Bound(KindNote, "a\xffb"), ErrText, RuleInvalidUTF8) {
		t.Error("invalid UTF-8 accepted")
	}
	if !isRule(Bound(Kind("essay"), "a"), ErrText, RuleUnknownKind) {
		t.Error("unknown kind accepted")
	}
}

func TestNewLabelsAndNormalises(t *testing.T) {
	// mutant: At kept in its own zone, or not truncated to the millisecond.
	x, err := New(KindMessage, "fine", AuthorAgent, agent, owner, at)
	if err != nil {
		t.Fatal(err)
	}
	if x.At.Location() != time.UTC || x.At.Nanosecond() != 456000000 {
		t.Fatalf("At = %v", x.At)
	}
	b, _ := json.Marshal(x)
	const golden = `{"body":"fine","author":"agent","authorAgentId":"ag-3f2a91c07b1e4d2a",` +
		`"ownerUserId":"u-0123456789abcdef","at":"2026-09-24T15:02:03.456Z"}`
	if string(b) != golden {
		t.Fatalf("json = %s", b)
	}
}

func TestNewRefusesWrongLabels(t *testing.T) {
	// mutants: each arm of New's label switch removed in turn.
	for name, c := range map[string]struct {
		a     Author
		ag, u string
		at    time.Time
	}{
		"unknown author":   {Author("bot"), agent, owner, at},
		"agent without id": {AuthorAgent, "", owner, at},
		"agent bad id":     {AuthorAgent, "s-1234", owner, at},
		"human with id":    {AuthorHuman, agent, owner, at},
		"bad owner":        {AuthorHuman, "", "alice@kpmg", at},
		"zero time":        {AuthorHuman, "", owner, time.Time{}},
	} {
		if _, err := New(KindNote, "ok", c.a, c.ag, c.u, c.at); !isRule(err, ErrText, RuleLabel) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestNewAppliesCleanNameToNames(t *testing.T) {
	// mutant: New calls Clean for every kind, so a title may carry a newline.
	if _, err := New(KindTitle, "one\ntwo", AuthorHuman, "", owner, at); !isRule(err, ErrText, RuleWhitespace) {
		t.Errorf("title with newline: %v", err)
	}
	if _, err := New(KindNote, "one\ntwo", AuthorHuman, "", owner, at); err != nil {
		t.Errorf("note with newline: %v", err)
	}
}

func isRule(err error, class error, r Rule) bool {
	var e *Error
	return errors.As(err, &e) && errors.Is(err, class) && e.Rule == r
}
```
