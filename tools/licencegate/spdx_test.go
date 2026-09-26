package main

import "testing"

// TestParseSPDX pins the evaluator: AND binds tighter than OR, a WITH
// exception must itself be allowed, and every "not a licence" form (a guess
// marked "*", UNKNOWN, the empty string) is never allowed.
func TestParseSPDX(t *testing.T) {
	cases := []struct {
		in      string
		allowed bool
		wantErr bool
	}{
		{"MIT", true, false},
		{"(MIT OR Apache-2.0)", true, false},
		{"(MIT AND MPL-2.0)", false, false},
		{"MPL-2.0 OR MIT", true, false},
		{"Apache-2.0 WITH LLVM-exception", true, false},
		{"GPL-2.0 WITH Classpath-exception-2.0", false, false},
		// An allowed licence with an exception not on AllowedExceptions: the
		// row the GPL one cannot be, since GPL fails without the exception
		// (task 04 mutation P2 survived without it).
		{"MIT WITH Classpath-exception-2.0", false, false},
		// The mirror: an allowed exception cannot launder a licence that is not
		// allowed (fix round 1, review I3: mutant V1, with.allowed() reading
		// the exception alone, survived every row above).
		{"GPL-2.0-only WITH LLVM-exception", false, false},
		{"MPL-2.0 WITH LLVM-exception", false, false},
		{"MIT*", false, false},
		{"UNKNOWN", false, false},
		{"", false, false},
		{"(MIT", false, true},
		// AND binds tighter than OR: MPL AND GPL is one term, MIT the other.
		{"MIT OR MPL-2.0 AND GPL-3.0", true, false},
		{"MPL-2.0 AND MIT OR GPL-3.0", false, false},
		{"(MIT OR MPL-2.0) AND GPL-3.0", false, false},
		{"(MIT OR Apache-2.0) AND ISC", true, false},
		{"UNLICENSED", false, false},
		{"SEE LICENSE IN LICENSE.txt", false, false},
		{"Custom: https://example.com/licence", false, false},
		{"LicenseRef-unclassified", false, false},
		{"MIT AND LicenseRef-unclassified", false, false},
		{"mit", true, false},
		{"BSD-2-Clause", true, false},
		{"BSD-3-Clause", true, false},
		{"0BSD", true, false},
		{"ISC", true, false},
		{"GPL-3.0-only", false, false},
		{"Apache-2.0 WITH", false, true},
		{"MIT OR", false, true},
		{"MIT)", false, true},
		{"MIT Apache-2.0", false, true},
	}
	for _, c := range cases {
		e, err := ParseSPDX(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseSPDX(%q): want an error, got %v", c.in, e)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSPDX(%q): unexpected error %v", c.in, err)
			continue
		}
		if got := e.allowed(); got != c.allowed {
			t.Errorf("ParseSPDX(%q).allowed() = %v, want %v", c.in, got, c.allowed)
		}
	}
}
