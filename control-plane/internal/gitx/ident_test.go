package gitx

import "testing"

// TestParseIdent (task 03): the first '<' and the first '>' after it delimit the email, and an
// identity missing either half is not an identity.
func TestParseIdent(t *testing.T) {
	cases := []struct {
		line string
		want Ident
		ok   bool
	}{
		{"Ada <ada@x.io> 1 +0000", Ident{Name: "Ada", Email: "ada@x.io"}, true},
		{"Ada Lovelace <ada@x.io> 1790156728 +1000\n", Ident{Name: "Ada Lovelace", Email: "ada@x.io"}, true},
		{"<a@b> 1 +0", Ident{}, false},
		{"Ada 1 +0", Ident{}, false},
		{"Ada <> 1 +0", Ident{}, false},
		{"Ada <ada@x.io 1 +0", Ident{}, false},
		{"", Ident{}, false},
	}
	for _, c := range cases {
		got, ok := ParseIdent(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseIdent(%q) = %+v, %v; want %+v, %v", c.line, got, ok, c.want, c.ok)
		}
	}
}
