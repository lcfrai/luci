package provenance

import (
	"errors"
	"strings"
	"testing"
)

// TestParseAcceptsTheHubsForms: Hub contract §3.2's examples, and the edges
// of its grammar, parse; String() gives the input back.
func TestParseAcceptsTheHubsForms(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Key
	}{
		// Hub contract §3.2's two examples, verbatim.
		{"brief:bf-3f2a91c07b1e4d2a@5", Key{Kind: KindBrief, ID: "bf-3f2a91c07b1e4d2a", Revision: 5}},
		{"design:cv-7c01d2e3f4a5b6c7@11", Key{Kind: KindDesign, ID: "cv-7c01d2e3f4a5b6c7", Revision: 11}},
		// The one local form (P4-C).
		{"none:declined", Key{Kind: KindNone, NoneReason: "declined"}},
		{Declined, Key{Kind: KindNone, NoneReason: "declined"}},
		// The edges: the largest revision, 2 and 4 letters, 4 and 32 hex.
		{"brief:bf-0000000000000000@999999999", Key{Kind: KindBrief, ID: "bf-0000000000000000", Revision: 999999999}},
		{"brief:bf-00000000000000a2@1", Key{Kind: KindBrief, ID: "bf-00000000000000a2", Revision: 1}},
		{"design:ab-0123@1", Key{Kind: KindDesign, ID: "ab-0123", Revision: 1}},
		{"design:abcd-" + strings.Repeat("f", 32) + "@10", Key{Kind: KindDesign, ID: "abcd-" + strings.Repeat("f", 32), Revision: 10}},
		// The design form's largest revision too (mutant R8-P1).
		{"design:cv-7c01d2e3f4a5b6c7@999999999", Key{Kind: KindDesign, ID: "cv-7c01d2e3f4a5b6c7", Revision: 999999999}},
	} {
		got, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		if s := got.String(); s != tc.in {
			t.Errorf("Parse(%q).String() = %q, want the input back", tc.in, s)
		}
	}
}

// TestParseRejects: nothing outside the grammar parses, so the desktop has
// no lenient parser to lean on.
func TestParseRejects(t *testing.T) {
	const id = "bf-3f2a91c07b1e4d2a"
	for _, in := range []string{
		"brief:b-42@3", // a pre-Hub id
		"brief:" + id + "@0",
		"brief:" + id + "@01",
		"brief:bf-3F2A91C07B1E4D2A@5",  // uppercase hex
		"brief:bf-3f2a91c07b1e4d2@5",   // 15 hex
		"brief:bf-3f2a91c07b1e4d2a0@5", // 17 hex
		"brief:" + id + "@5 ",          // trailing space
		"brief:" + id + "@5\n",         // trailing line feed: $ is end of text
		" brief:" + id + "@5",
		"brief:" + id + "@1000000000", // ten digits
		"brief:" + id + "@-1",
		"brief:" + id + "@+5",
		"brief:" + id + "@５", // a full-width digit
		"brief:" + id,
		"brief:" + id + "@5@6",
		"Brief:" + id + "@5",
		"brief:" + id + "@",
		"design:cv-7@11", // too short: 4-32 hex
		"design:c-0123@1",
		"design:abcde-0123@1",
		"design:cv-" + strings.Repeat("a", 33) + "@1",
		"design:CV-0123@1",
		"design:cv-7c01d2e3f4a5b6c7@0",
		"design:cv-7c01d2e3f4a5b6c7@01",
		"design:cv-7c01d2e3f4a5b6c7@1000000000", // ten digits (mutant R8-P2)
		"none:",
		"none:other",
		"none:Declined",
		"none:declined ",
		"none",
		"",
		"bf-3f2a91c07b1e4d2a@5",
	} {
		k, err := Parse(in)
		if !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalidKey", in, err)
		}
		if k != (Key{}) {
			t.Errorf("Parse(%q) = %+v, want the zero Key beside the error", in, k)
		}
	}
}

// TestKeyStringNeverWritesANonKey: String gives "" for any Key the grammar
// would not parse back, so a hand-built Key cannot mint a string that looks
// like a key and is not one.
func TestKeyStringNeverWritesANonKey(t *testing.T) {
	for _, k := range []Key{
		{},
		{Kind: KindBrief, ID: "b-42", Revision: 3},
		{Kind: KindBrief, ID: "bf-3f2a91c07b1e4d2a"},
		{Kind: KindBrief, ID: "bf-3f2a91c07b1e4d2a", Revision: -1},
		{Kind: KindDesign, ID: "cv-7", Revision: 11},
		{Kind: KindNone},
		{Kind: KindNone, NoneReason: "other"},
		{Kind: "session", ID: "bf-3f2a91c07b1e4d2a", Revision: 1},
	} {
		if s := k.String(); s != "" {
			t.Errorf("%+v.String() = %q, want \"\"", k, s)
		}
	}
}

// TestBriefKey: the one writer of a brief key. A bad id or revision is a
// programming error, not input, so it panics.
func TestBriefKey(t *testing.T) {
	if got := BriefKey("bf-3f2a91c07b1e4d2a", 5); got != "brief:bf-3f2a91c07b1e4d2a@5" {
		t.Errorf("BriefKey = %q", got)
	}
	if got := BriefKey("bf-00000000000000a2", 999999999); got != "brief:bf-00000000000000a2@999999999" {
		t.Errorf("BriefKey at the largest revision = %q", got)
	}
	for _, tc := range []struct {
		id  string
		rev int
	}{
		{"b-42", 5},
		{"bf-3F2A91C07B1E4D2A", 5},
		{"cv-7c01d2e3f4a5b6c7", 5},
		{"", 5},
		// The id is the whole id: 17 hex, a trailing line feed, or a prefix
		// before bf- would mint a key Parse refuses (mutants R6-P1, R6-P1b).
		{"bf-3f2a91c07b1e4d2a0", 5},
		{"bf-3f2a91c07b1e4d2a\n", 5},
		{"xbf-3f2a91c07b1e4d2a", 5},
		{"bf-3f2a91c07b1e4d2a", 0},
		{"bf-3f2a91c07b1e4d2a", -1},
		{"bf-3f2a91c07b1e4d2a", 1000000000},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("BriefKey(%q, %d) did not panic", tc.id, tc.rev)
				}
			}()
			_ = BriefKey(tc.id, tc.rev)
		}()
	}
}
