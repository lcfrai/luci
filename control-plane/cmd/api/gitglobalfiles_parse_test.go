package main

import "testing"

// X-3 review 4 I-C, FAIL CLOSED: parseShowOrigin reads git's -z record — one
// `<origin type>:<name>` NUL `<value>` NUL, measured on git 2.53.0.windows.2 —
// in which a name is raw bytes and nothing is quoted, so there is no quoting to
// get wrong. Every other answer is an error, which fails the create: an origin
// this parse cannot name is an origin no walk checked. Without -z, git
// C-quotes a name holding a backslash, a quote, a control byte or a non-ASCII
// byte, and the parse before this one kept the quotes as part of the path.
func TestAShowOriginAnswerIsOneRawRecordOrAnError(t *testing.T) {
	for _, tc := range []struct{ name, out, value, origin string }{
		{"a file, raw backslashes and a non-ASCII byte",
			"file:C:\\Users\\me\\projects\\dot\\\xc3\xa9vil.inc\x00C:/x/secret\x00",
			"C:/x/secret", "C:\\Users\\me\\projects\\dot\\\xc3\xa9vil.inc"},
		{"a file whose name holds a quote and a tab", "file:/p/a\"b\tc.inc\x00/v\x00", "/v", "/p/a\"b\tc.inc"},
		{"an empty value", "file:/host/.gitconfig\x00\x00", "", "/host/.gitconfig"},
		{"a value kept byte for byte", "file:/g\x00 a\nb \x00", " a\nb ", "/g"},
		{"--default's answer names no file", "command line:\x00/home/u/.config/git/ignore\x00", "/home/u/.config/git/ignore", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, origin, err := parseShowOrigin(tc.out)
			if err != nil || value != tc.value || origin != tc.origin {
				t.Errorf("parseShowOrigin(%q) = %q, %q, %v; want %q, %q, nil", tc.out, value, origin, err, tc.value, tc.origin)
			}
		})
	}
	for name, out := range map[string]string{
		"no answer":                        "",
		"the quoted form, without -z":      "file:\"C:\\\\Users\\\\me\\\\b2.inc\"\tC:/x/secret\n",
		"an unquoted line, without -z":     "file:/host/.gitconfig\t/x\n",
		"a record with no terminator":      "file:/g\x00/x",
		"two records":                      "file:/g\x00/x\x00file:/h\x00/y\x00",
		"a value and no origin":            "/x/ignore\x00",
		"a file with no name":              "file:\x00/x\x00",
		"a blob":                           "blob:0123abcd\x00/x\x00",
		"standard input":                   "standard input:\x00/x\x00",
		"a command line that names a file": "command line:/g\x00/x\x00",
		"an origin type git never names":   "fil:/g\x00/x\x00",
	} {
		t.Run(name, func(t *testing.T) {
			if value, origin, err := parseShowOrigin(out); err == nil {
				t.Errorf("parseShowOrigin(%q) = %q, %q and no error; an answer that is not one -z record must fail the read", out, value, origin)
			}
		})
	}
}
