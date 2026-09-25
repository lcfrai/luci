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

**(e) Secret-shaped text is a fixed table of twelve RE2 patterns.** Go's `regexp` is RE2, linear in
the input, so an attacker cannot make the check slow. The table is closed: a new pattern is a code
change with its own test. It refuses; it never redacts, because a redacted string is still a string
an agent chose to send, and the sender should hear *"refused: secret"* and stop.

**(f) `Text` marshals itself in the Hub's response rules.** The scope gives `Text.At` as a
`time.Time`, and it stays one in Go. But the Hub writes every time as RFC 3339 UTC with exactly
three fractional digits (contract §1; `hubapi.FormatTime`, `hubapi/envelope.go:392-394`), which
`time.Time`'s own `MarshalJSON` does not (a whole second loses its `.000`), and a Hub response never
omits a field (`hubapi/types.go:6-11`). So `Text` has its own `MarshalJSON`: the Hub's time form,
and `"authorAgentId": null` for a human's text. The layout is repeated here, not imported, because
`hubapi` imports this package; task 03's golden test pins the two layouts equal.

**Files:**
- `control-plane/internal/teamtext/text.go` — `Author`, `Kind`, the limits, `Text`, `Bound`, `New`,
  `Error` and the three class sentinels (new)
- `control-plane/internal/teamtext/hygiene.go` — `Clean`, `CleanName`, the secret table (new)
- `control-plane/internal/teamtext/glob.go` — `ValidGlob`, `ValidPath`, `Match`, `Breadth`,
  `CheckSet` (new)
- `control-plane/internal/teamtext/text_test.go`, `hygiene_test.go`, `glob_test.go` (new)

No `go.mod` change: standard library only (`encoding/json`, `errors`, `fmt`, `regexp`, `strings`, `time`,
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
	"encoding/json"
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
// every text·A and text·H field (Phase T task 01, Step 6); MarshalJSON writes
// it in the Hub's response rules.
type Text struct {
	Body        string
	Author      Author
	AuthorAgent string // "" for a human's text
	Owner       string
	At          time.Time
}

// timeLayout is the Hub's one time form (contract §1): UTC, exactly three
// fractional digits. It repeats hubapi.FormatTime's layout because this
// package may not import hubapi; task 03's golden test pins the two equal.
const timeLayout = "2006-01-02T15:04:05.000Z"

// textJSON is Text on the wire. Hub responses never omit a field (contract
// §7.1): a human's text says "authorAgentId": null.
type textJSON struct {
	Body        string  `json:"body"`
	Author      Author  `json:"author"`
	AuthorAgent *string `json:"authorAgentId"`
	Owner       string  `json:"ownerUserId"`
	At          string  `json:"at"`
}

// MarshalJSON writes t with its time in the Hub's form and a null agent for a
// human's text.
func (t Text) MarshalJSON() ([]byte, error) {
	j := textJSON{Body: t.Body, Author: t.Author, Owner: t.Owner, At: t.At.UTC().Format(timeLayout)}
	if t.AuthorAgent != "" {
		j.AuthorAgent = &t.AuthorAgent
	}
	return json.Marshal(j)
}

// UnmarshalJSON reads what MarshalJSON writes, and any RFC 3339 time.
func (t *Text) UnmarshalJSON(b []byte) error {
	var j textJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339Nano, j.At)
	if err != nil {
		return fmt.Errorf("teamtext: at: %w", err)
	}
	*t = Text{Body: j.Body, Author: j.Author, Owner: j.Owner, At: at.UTC()}
	if j.AuthorAgent != nil {
		t.AuthorAgent = *j.AuthorAgent
	}
	return nil
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

func TestTextJSONFollowsTheHubRules(t *testing.T) {
	// mutants: time.Time's own marshalling (drops ".000" on a whole second);
	// omitempty on authorAgentId (the Hub never omits a response field).
	h, _ := New(KindNote, "hi", AuthorHuman, "", owner, time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC))
	b, _ := json.Marshal(h)
	const golden = `{"body":"hi","author":"human","authorAgentId":null,` +
		`"ownerUserId":"u-0123456789abcdef","at":"2026-09-25T01:00:00.000Z"}`
	if string(b) != golden {
		t.Fatalf("json = %s", b)
	}
	var back Text
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if b2, _ := json.Marshal(back); string(b2) != golden || back.AuthorAgent != "" {
		t.Fatalf("round trip = %s", b2)
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

- [ ] **Step 2: Write the failing tests for `hygiene.go`**

Every sample secret is **assembled at run time** (`"ghp_" + strings.Repeat(…)`), so no
secret-shaped literal sits in the repo for a push-protection scanner, or a reader, to trip on.
`control-plane/internal/teamtext/hygiene_test.go`:

```go
package teamtext

import (
	"strings"
	"testing"
)

func TestCleanRefusesEachCharacterClass(t *testing.T) {
	for _, c := range []struct {
		in     string
		rule   Rule
		off    int
		mutant string
	}{
		{"a\x00b", RuleControl, 1, "C0 check starts at 0x01"},
		{"a\x07b", RuleControl, 1, "C0 check dropped"},
		{"a\rb", RuleControl, 1, "CR allowed with LF and TAB"},
		{"a\x7fb", RuleControl, 1, "DEL (0x7f) not treated as a control"},
		{"a\u0085b", RuleControl, 1, "C1 range 0x80-0x9f dropped"},
		{"a\u009fb", RuleControl, 1, "C1 upper bound 0x9e"},
		{"a\x1b[31mb", RuleANSI, 1, "ESC reported as a plain control, or allowed"},
		{"a\u009b31mb", RuleANSI, 1, "C1 CSI (0x9b) not reported as ANSI"},
		{"a\u009d0;x\u0007", RuleANSI, 1, "C1 OSC (0x9d) not reported as ANSI"},
		{"x‪y", RuleBidi, 1, "bidi range starts at 0x202b"},
		{"x‮y", RuleBidi, 1, "bidi range ends at 0x202d (RLO let through)"},
		{"x⁦y", RuleBidi, 1, "isolates 0x2066-0x2069 dropped"},
		{"x⁩y", RuleBidi, 1, "isolate upper bound 0x2068 (PDI let through)"},
		{"x\U000E0000y", RuleTagBlock, 1, "tag block starts at 0xe0001"},
		{"x\U000E0041y", RuleTagBlock, 1, "tag block range dropped: U+E0041 is an invisible 'A'"},
		{"x\U000E007Fy", RuleTagBlock, 1, "tag block ends at 0xe007e"},
		{"é\x00", RuleControl, 1, "Offset counts bytes, not code points"},
	} {
		if err := Clean(c.in); !isRule(err, ErrText, c.rule) || err.(*Error).Offset != c.off {
			t.Errorf("%q: got %v, want %s at %d (mutant: %s)", c.in, err, c.rule, c.off, c.mutant)
		}
	}
}

func TestCleanAllowsBodyWhitespaceAndNeighbours(t *testing.T) {
	// mutants: LF or TAB refused in bodies; ranges widened past their true bounds.
	for _, s := range []string{"line one\nline two", "a\tb", "a  b", " lead", "x y",
		"x y", "x⁥y", "x⁪y", "x\U000E0080y", "ok é 中"} {
		if err := Clean(s); err != nil {
			t.Errorf("%q refused: %v", s, err)
		}
	}
}

func TestCleanNameRefusesWhitespaceShapes(t *testing.T) {
	// mutants: each arm of the name rule removed in turn.
	for _, s := range []string{"a  b", " a", "a ", "a b", "a　b", "a\tb", "a\nb"} {
		if err := CleanName(s); !isRule(err, ErrText, RuleWhitespace) {
			t.Errorf("%q: got %v, want whitespace", s, err)
		}
	}
	if err := CleanName("Add the export route"); err != nil {
		t.Errorf("plain name refused: %v", err)
	}
}

func TestCleanRefusesEverySecretShape(t *testing.T) {
	r := strings.Repeat
	for name, s := range map[string]string{
		"private_key":      "-----BEGIN " + "OPENSSH PRIVATE KEY-----",
		"aws_access_key":   "id " + "AKIA" + r("Q", 16) + " x",
		"github_token":     "ghp_" + r("a", 36),
		"github_pat":       "github_pat_" + r("A", 30),
		"anthropic_key":    "sk-ant-" + r("b", 24),
		"sk_key":           "sk-" + r("c", 24),
		"slack_token":      "xoxb-" + r("1", 12),
		"jwt":              "eyJ" + r("h", 10) + ".eyJ" + r("p", 10) + "." + r("s", 10),
		"azure_key":        "AccountKey=" + r("K", 40) + "==",
		"url_credentials":  "https://bob:" + "hunter2@example.com",
		"assignment":       "password" + " = " + r("z", 9),
		"azure_devops_pat": r("a", 26) + r("7", 26),
	} {
		err := Clean(s)
		if !isRule(err, ErrText, RuleSecret) || err.(*Error).Detail != name {
			t.Errorf("%s: got %v (mutant: its row dropped or reordered)", name, err)
		}
	}
	if len(secrets) != 12 {
		t.Fatalf("secret table has %d rows, want 12: a row added without a test", len(secrets))
	}
}

func TestCleanSecretNearMisses(t *testing.T) {
	// mutant: a pattern loosened until ordinary prose trips it.
	for _, s := range []string{"the AKIA prefix", "ghp_short", "sk-short", "password is required",
		"see https://example.com/a@b", "-----BEGIN CERTIFICATE-----", r52Hex()} {
		if err := Clean(s); err != nil {
			t.Errorf("%q refused: %v", s, err)
		}
	}
}

// r52Hex is 52 hex characters with a 0: outside the ADO PAT alphabet.
func r52Hex() string { return "0" + strings.Repeat("a", 51) }
```

- [ ] **Step 3: Write the failing tests for `glob.go`**

`control-plane/internal/teamtext/glob_test.go`:

```go
package teamtext

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidGlobRefusals(t *testing.T) {
	for _, c := range []struct {
		in     string
		rule   Rule
		mutant string
	}{
		{"", RuleEmpty, "empty glob accepted"},
		{"/etc/passwd", RuleAbsolute, "leading slash accepted"},
		{"C:/Users/x", RuleAbsolute, "drive letter accepted"},
		{`src\app.ts`, RuleBackslash, "backslash accepted"},
		{"src/../../etc", RuleDotDot, "a .. segment accepted"},
		{"..", RuleDotDot, ".. checked only between slashes"},
		{"src//a.ts", RuleSegment, "an empty segment accepted"},
		{"src/", RuleSegment, "a trailing slash accepted"},
		{"./src", RuleSegment, "a . segment accepted"},
		{"src/a**b", RuleSegment, "** inside a segment accepted"},
		{"**", RuleWholeRepo, "whole-repo glob accepted"},
		{"*", RuleWholeRepo, "single-star whole-repo glob accepted"},
		{"**/*", RuleWholeRepo, "whole-repo check looks at the first segment only"},
		{strings.Repeat("a", 201), RuleTooLong, "path cap 201"},
		{"src/\x1b[2Ja", RuleANSI, "glob not hygiene-checked"},
		{"src/a\tb", RuleControl, "TAB allowed in a path"},
	} {
		if err := ValidGlob(c.in); !isRule(err, ErrGlob, c.rule) {
			t.Errorf("%q: got %v, want %s (mutant: %s)", c.in, err, c.rule, c.mutant)
		}
	}
}

func TestValidGlobAccepts(t *testing.T) {
	// mutants: [ ] { } treated as metacharacters; the cap at 199; ".git" style names refused.
	for _, s := range []string{"src/export/**", "app/[id]/page.tsx", "docs/{draft}.md",
		"**/*.test.ts", "a?c", ".github/workflows/ci.yml", strings.Repeat("a", 200)} {
		if err := ValidGlob(s); err != nil {
			t.Errorf("%q refused: %v", s, err)
		}
	}
}

func TestValidPathTakesMetacharactersLiterally(t *testing.T) {
	// mutant: ValidPath shares the whole-repo rule, so a file named "*" cannot be reported.
	if err := ValidPath("*"); err != nil {
		t.Errorf(`file "*" refused: %v`, err)
	}
	if !isRule(ValidPath("../x"), ErrGlob, RuleDotDot) {
		t.Error("ValidPath lost the .. rule")
	}
}

func TestMatch(t *testing.T) {
	for _, c := range []struct {
		g, p   string
		want   bool
		mutant string
	}{
		{"src/**", "src/a/b/c.ts", true, "** matches one segment only"},
		{"src/**", "src", true, "** does not match zero segments"},
		{"src/**/x.ts", "src/x.ts", true, "inner ** does not match zero segments"},
		{"src/*.ts", "src/a/b.ts", false, "* crosses a slash"},
		{"src/?.ts", "src/ab.ts", false, "? matches two characters"},
		{"src/?.ts", "src/é.ts", true, "? matches a byte, not a code point"},
		{"app/[id]/page.tsx", "app/[id]/page.tsx", true, "[ ] read as a class"},
		{"app/[id]/page.tsx", "app/i/page.tsx", false, "[ ] read as a class"},
		{"a*b*c", "aXXbYYc", true, "star backtracking broken"},
		{"a*b", "a*b", true, "literal * in a path unmatched"},
		{"src/a.ts", "src/a.tsx", false, "prefix match instead of whole segment"},
	} {
		if got := Match(c.g, c.p); got != c.want {
			t.Errorf("Match(%q, %q) = %v (mutant: %s)", c.g, c.p, got, c.mutant)
		}
	}
}

func TestMatchIsPolynomial(t *testing.T) {
	// mutant: ** matched by plain recursion; this input is exponential without the memo.
	g := strings.Repeat("**/a/", 20) + "b"
	p := strings.Repeat("a/", 60) + "c"
	if Match(g, p) {
		t.Fatal("matched")
	}
}

func tracked(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("f/%03d.go", i)
	}
	return out
}

func TestBreadthIsTheWidestSingleGlob(t *testing.T) {
	// mutant: the union of all globs measured instead of the widest one.
	files := append(tracked(10), "a/x.go", "b/y.go")
	got := Breadth([]string{"a/**", "b/**"}, files)
	if want := 1.0 / 12; got != want {
		t.Fatalf("Breadth = %v, want %v", got, want)
	}
	if Breadth([]string{"**/*.go"}, nil) != 0 {
		t.Fatal("empty tracked list not 0")
	}
}

func TestCheckSet(t *testing.T) {
	// mutant: an empty set accepted as "claims nothing".
	if err := CheckSet(nil, tracked(10)); !isRule(err, ErrGlob, RuleEmpty) {
		t.Errorf("empty set: %v", err)
	}
	// f/00?.go matches f/000.go … f/009.go, ten files.
	// mutant: >= instead of >, so exactly 10% (10 of 100) is refused.
	if err := CheckSet([]string{"f/00?.go"}, tracked(100)); err != nil {
		t.Errorf("10 of 100 refused: %v", err)
	}
	// mutant: the breadth check dropped; 10 of 99 is over 10%.
	if err := CheckSet([]string{"f/00?.go"}, tracked(99)); !isRule(err, ErrBreadth, RuleTooBroad) {
		t.Errorf("10 of 99: %v", err)
	}
	// mutant: with no tracked list (the Hub's call) breadth divides by zero or refuses all.
	if err := CheckSet([]string{"f/00?.go"}, nil); err != nil {
		t.Errorf("no tracked list: %v", err)
	}
	// mutant: the 20-glob cap at 21, or checked after the per-glob loop.
	many := make([]string, 21)
	for i := range many {
		many[i] = fmt.Sprintf("f/%03d.go", i)
	}
	if err := CheckSet(many, nil); !isRule(err, ErrBreadth, RuleTooMany) {
		t.Errorf("21 globs: %v", err)
	}
	// mutant: CheckSet reports the rune offset, not the set index, of a bad glob.
	if err := CheckSet([]string{"ok/a.go", "/abs"}, nil); !isRule(err, ErrGlob, RuleAbsolute) ||
		err.(*Error).Offset != 1 {
		t.Errorf("index: %v", err)
	}
}
```

- [ ] **Step 4: Run the tests and watch them fail**

`hygiene.go` and `glob.go` do not exist yet, so the package does not compile. Put `text.go` in
place first (the Interfaces block above, verbatim), then run from the repository root:

```bash
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && GOPROXY=off go test ./internal/teamtext/ 2>&1 | head -5'
```

Expected: `undefined: Clean`, `undefined: CleanName`, `undefined: secrets`, `undefined:
ValidGlob` (in some order), then `FAIL github.com/kpmg-au-emu/panacea-app/control-plane/internal/teamtext [build failed]`.

- [ ] **Step 5: Write `hygiene.go`**

```go
package teamtext

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

// secret is one row of the closed table of secret shapes. A new row is a code
// change with its own test (hygiene_test.go counts the rows).
type secret struct {
	name string
	re   *regexp.Regexp
}

// secrets is checked in order; the first match names the refusal. RE2 is
// linear in its input, so no string makes this slow.
var secrets = []secret{
	{"private_key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	{"aws_access_key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"github_token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,255}\b`)},
	{"github_pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,255}\b`)},
	{"anthropic_key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
	{"sk_key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{"slack_token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)},
	{"azure_key", regexp.MustCompile(`(?i)(?:AccountKey|SharedAccessKey)=[A-Za-z0-9+/]{30,}={0,2}`)},
	{"url_credentials", regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s/:@]+:[^\s/@]+@`)},
	{"assignment", regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token|client[_-]?secret)\s*[:=]\s*["']?[^\s"']{8,}`)},
	{"azure_devops_pat", regexp.MustCompile(`\b[a-z2-7]{52}\b`)},
}

// Clean refuses a body that carries a character no reader should receive
// unlabelled: C0 and C1 controls other than LF and TAB, ANSI escape
// introducers (ESC, CSI, OSC), bidi overrides and isolates, the Unicode tag
// block, or secret-shaped text (05-design.md §7.1, G3). It does not bound
// length; Bound does.
func Clean(s string) error { return clean(s, false) }

// CleanName is Clean plus the one-line rule for names: no LF or TAB, no space
// but U+0020, no run of two, none at either end.
func CleanName(s string) error { return clean(s, true) }

func clean(s string, name bool) error {
	if !utf8.ValidString(s) {
		return refuse(ErrText, RuleInvalidUTF8, -1)
	}
	i, prevSpace := 0, false
	for _, r := range s {
		switch {
		case r == 0x1b || r == 0x9b || r == 0x9d:
			return refuse(ErrText, RuleANSI, i)
		case r == '\n' || r == '\t':
			if name {
				return refuse(ErrText, RuleWhitespace, i)
			}
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return refuse(ErrText, RuleControl, i)
		case (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return refuse(ErrText, RuleBidi, i)
		case r >= 0xe0000 && r <= 0xe007f:
			return refuse(ErrText, RuleTagBlock, i)
		}
		space := unicode.IsSpace(r)
		if name && space && (r != ' ' || prevSpace || i == 0) {
			return refuse(ErrText, RuleWhitespace, i)
		}
		prevSpace = space
		i++
	}
	if name && prevSpace {
		return refuse(ErrText, RuleWhitespace, i-1)
	}
	for _, p := range secrets {
		if p.re.MatchString(s) {
			e := refuse(ErrText, RuleSecret, -1)
			e.Detail = p.name
			return e
		}
	}
	return nil
}
```

- [ ] **Step 6: Write `glob.go`**

```go
package teamtext

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxGlobs is the most globs one scope or lease may name; MaxBreadth the
// largest share of tracked files one glob may match (05-design.md §6.3).
// CheckSet tests MaxBreadth as the integer n*10 > len: change both together.
const (
	MaxGlobs   = 20
	MaxBreadth = 0.10
)

// ValidGlob refuses a glob that is empty, over MaxPath code points, not
// clean, absolute, contains a backslash, a "..", "." or empty segment, a "**"
// inside a segment, or is made only of "*" and "**" segments. Its only
// metacharacters are "*", "?" and a whole-segment "**"; everything else,
// "[", "]", "{" and "}" included, is literal.
func ValidGlob(g string) error { return validPath(g, true) }

// ValidPath is ValidGlob's rules for a literal repo-relative path: "*" and
// "?" are ordinary characters in it, and no whole-repo rule applies.
func ValidPath(p string) error { return validPath(p, false) }

func validPath(p string, glob bool) error {
	switch {
	case p == "":
		return refuse(ErrGlob, RuleEmpty, -1)
	case !utf8.ValidString(p):
		return refuse(ErrGlob, RuleInvalidUTF8, -1)
	case utf8.RuneCountInString(p) > MaxPath:
		return refuse(ErrGlob, RuleTooLong, MaxPath)
	}
	if i := strings.IndexAny(p, "\n\t"); i >= 0 {
		return refuse(ErrGlob, RuleControl, utf8.RuneCountInString(p[:i]))
	}
	if err := Clean(p); err != nil {
		var e *Error
		errors.As(err, &e)
		return &Error{Class: ErrGlob, Rule: e.Rule, Offset: e.Offset, Detail: e.Detail}
	}
	if i := strings.IndexByte(p, '\\'); i >= 0 {
		return refuse(ErrGlob, RuleBackslash, utf8.RuneCountInString(p[:i]))
	}
	if p[0] == '/' || (len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0])) {
		return refuse(ErrGlob, RuleAbsolute, 0)
	}
	segs := strings.Split(p, "/")
	wild := true
	for _, s := range segs {
		switch {
		case s == "..":
			return refuse(ErrGlob, RuleDotDot, -1)
		case s == "" || s == ".":
			return refuse(ErrGlob, RuleSegment, -1)
		case glob && s != "**" && strings.Contains(s, "**"):
			return refuse(ErrGlob, RuleSegment, -1)
		}
		if s != "*" && s != "**" {
			wild = false
		}
	}
	if glob && wild {
		return refuse(ErrGlob, RuleWholeRepo, -1)
	}
	return nil
}

func isASCIILetter(b byte) bool { return (b|0x20) >= 'a' && (b|0x20) <= 'z' }

// Match reports whether a repo-relative path matches a glob of ValidGlob's
// grammar. It is O(len(glob segments) × len(path segments)): "**" is matched
// over a memo, never by plain recursion.
func Match(glob, path string) bool {
	g, p := strings.Split(glob, "/"), strings.Split(path, "/")
	memo := make([][]uint8, len(g)+1) // 0 unknown, 1 match, 2 no match
	for i := range memo {
		memo[i] = make([]uint8, len(p)+1)
	}
	var m func(i, j int) bool
	m = func(i, j int) bool {
		if memo[i][j] != 0 {
			return memo[i][j] == 1
		}
		var ok bool
		switch {
		case i == len(g):
			ok = j == len(p)
		case g[i] == "**":
			ok = m(i+1, j) || (j < len(p) && m(i, j+1))
		default:
			ok = j < len(p) && matchSegment(g[i], p[j]) && m(i+1, j+1)
		}
		memo[i][j] = 2
		if ok {
			memo[i][j] = 1
		}
		return ok
	}
	return m(0, 0)
}

// matchSegment matches one segment: "*" is any run of code points, "?" one
// code point; every other rune is literal. Iterative, backtracking to the
// last "*" only, so it is linear-times-star-count, never exponential.
func matchSegment(glob, seg string) bool {
	g, s := []rune(glob), []rune(seg)
	gi, si, star, mark := 0, 0, -1, 0
	for si < len(s) {
		switch {
		case gi < len(g) && g[gi] == '*':
			star, mark = gi, si
			gi++
		case gi < len(g) && (g[gi] == '?' || g[gi] == s[si]):
			gi++
			si++
		case star >= 0:
			mark++
			gi, si = star+1, mark
		default:
			return false
		}
	}
	for gi < len(g) && g[gi] == '*' {
		gi++
	}
	return gi == len(g)
}

// widest is the most tracked files any one glob matches.
func widest(globs, tracked []string) (n, at int) {
	at = -1
	for i, g := range globs {
		c := 0
		for _, f := range tracked {
			if Match(g, f) {
				c++
			}
		}
		if c > n {
			n, at = c, i
		}
	}
	return n, at
}

// Breadth is the share of tracked files matched by the widest single glob
// (not the union: twenty narrow claims are not one broad one). It is 0 when
// tracked is empty.
func Breadth(globs, tracked []string) float64 {
	if len(tracked) == 0 {
		return 0
	}
	n, _ := widest(globs, tracked)
	return float64(n) / float64(len(tracked))
}

// CheckSet is the whole rule for a scope or a lease's globs: 1 to MaxGlobs
// globs, each ValidGlob, and, when tracked is given, none matching more than
// MaxBreadth of it. Offset in a refusal is the glob's index in the set. The
// Hub, which holds no file list, calls it with tracked == nil (count and
// grammar only); Fleet calls it with the integration head's tree (§6.3).
func CheckSet(globs, tracked []string) error {
	switch {
	case len(globs) == 0:
		return refuse(ErrGlob, RuleEmpty, -1)
	case len(globs) > MaxGlobs:
		return refuse(ErrBreadth, RuleTooMany, MaxGlobs)
	}
	for i, g := range globs {
		if err := ValidGlob(g); err != nil {
			var e *Error
			errors.As(err, &e)
			return &Error{Class: e.Class, Rule: e.Rule, Offset: i, Detail: e.Detail}
		}
	}
	for i, g := range globs {
		// Integer form of n/len > MaxBreadth: exactly 10% passes, and no float rounds it.
		if n, _ := widest([]string{g}, tracked); n*10 > len(tracked) {
			return refuse(ErrBreadth, RuleTooBroad, i)
		}
	}
	return nil
}
```

The integer test `n*10 > len(tracked)` hard-codes `MaxBreadth`'s 10%. A test cannot see that
coupling, so the constant's comment says it, and a change to `MaxBreadth` must change this line.

- [ ] **Step 7: Run the tests to green, then vet and format**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && GOPROXY=off go test -count=1 -timeout 60s ./internal/teamtext/ && GOPROXY=off go vet ./internal/teamtext/ && gofmt -l internal/teamtext/ && echo gofmt-clean'
```

Expected, exactly:

```text
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/teamtext	0.0XXs
gofmt-clean
```

`-timeout 60s` is part of `TestMatchIsPolynomial`'s contract: without the memo that test does not
fail, it hangs, and the timeout turns the hang into a failure.

Then kill three mutants by hand, one at a time, each reverted before the next, to prove the tests
bite (the house rule: a test that never failed has not been shown to test anything):

1. In `hygiene.go`, change `r <= 0xe007f` to `r <= 0xe007e`. Expected: `TestCleanRefusesEachCharacterClass`
   fails on `"x\U000E007Fy"`.
2. In `glob.go`, change `n*10 > len(tracked)` to `n*10 >= len(tracked)`. Expected:
   `TestCheckSet` fails with `10 of 100 refused`.
3. In `text.go`, change `utf8.RuneCountInString(s)` in `Bound` to `len(s)`. Expected:
   `TestBoundCountsCodePointsNotBytes` fails.

- [ ] **Step 8: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add control-plane/internal/teamtext && git commit -F .superpowers/sdd/team/msg/t02.txt -- control-plane/internal/teamtext'
```

Subject: `feat(teamtext): bounded text, author class, G3 hygiene, globs and breadth (Phase T task
02)`; body: the three mutants killed by hand; last line the `Co-Authored-By` line.

**What this task does not claim.**
- **That it catches persuasive text.** A polite, well-formed message that talks an Orchestrator
  into a bad yield passes every check here (§7.3). The taint rule (task 31) and the escalation
  rows (task 30) are the controls for that, not this package.
- **That the secret table is complete.** It is twelve shapes, chosen for this stack (Entra JWTs,
  Azure keys, ADO and GitHub PATs, Anthropic and LiteLLM-style `sk-` keys). A secret in no listed
  shape passes. The gateway's own token format is **UNVERIFIED** (phase 6 task 08 fixes it); if it
  is not `sk-`-shaped, phase 6's delta adds a row here with its test.
- **That invisible characters outside the named ranges are refused.** Zero-width space (U+200B),
  zero-width joiners (U+200C, U+200D), bidi *marks* (U+200E, U+200F, U+061C) and the BOM (U+FEFF)
  pass: G3 names overrides, isolates and the tag block, and this task implements G3 as written.
  Widening it is a one-line change per range, each with a test, if the owner asks.
- **That breadth is cheap.** `widest` is globs × tracked files × match. At 20 globs and 100,000
  files that is two million matches per check; task 23 measures it against the real integration
  head and caches the tree if it must.
- **That `.git/**` is refused.** A glob into `.git` is valid here; Fleet's lease engine (task 23)
  decides whether a lease on it means anything.
