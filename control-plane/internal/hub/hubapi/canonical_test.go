package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// ── the canonical encoding (contract §3.4) ──────────────────────────────────

// Non-ASCII test text is spelled in UTF-8 bytes, so that no character in this
// file is invisible (U+2028, U+E000) or illegal mid-file (U+FEFF).
const (
	pua    = "\xee\x80\x80"     // U+E000, private use: one UTF-16 code unit
	grin   = "\xf0\x9f\x98\x80" // U+1F600: the UTF-16 pair D83D DE00
	lsep   = "\xe2\x80\xa8"     // U+2028
	psep   = "\xe2\x80\xa9"     // U+2029
	eAcute = "\xc3\xa9"         // U+00E9
	fffd   = "\xef\xbf\xbd"     // U+FFFD
	bom    = "\xef\xbb\xbf"     // U+FEFF
)

// u turns each "~u" into JSON's backslash-u escape. The tests write the
// escapes they feed Canonical as ~u, so each one stays an escape in this
// source whatever tool edits it (one did decode them, once).
func u(s string) string { return strings.ReplaceAll(s, "~u", "\\u") }

// canon runs Canonical on JSON text and fails the test on an error.
func canon(t *testing.T, in string) string {
	t.Helper()
	out, err := Canonical(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Canonical(%q): %v", in, err)
	}
	return string(out)
}

// refuses asserts Canonical(in) fails with want, and returns the error.
func refuses(t *testing.T, in any, want error) error {
	t.Helper()
	out, err := Canonical(in)
	if !errors.Is(err, want) {
		t.Fatalf("Canonical(%#v) = %q, %v; want %v", in, out, err, want)
	}
	if out != nil {
		t.Fatalf("Canonical(%#v) returned bytes %q with its error", in, out)
	}
	return err
}

func TestKeysSortByUTF16CodeUnits(t *testing.T) {
	// U+E000 is one code unit, 0xE000. U+1F600 is the pair D83D DE00, and
	// 0xD83D < 0xE000, so the emoji sorts first. UTF-8 bytes (EE… vs F0…)
	// and code points (0xE000 vs 0x1F600) both put U+E000 first.
	want := `{"` + grin + `":2,"` + pua + `":1}`
	cases := []struct{ in, want string }{
		{`{"` + pua + `":1,"` + grin + `":2}`, want},
		{u(`{"~ue000":1,"~ud83d~ude00":2}`), want}, // the same keys, escaped
		{want, want},
		// Where UTF-16 and code-point order agree.
		{`{"b":1,"a":2,"B":3,"":4,"` + eAcute + `":5,"aa":6}`, `{"":4,"B":3,"a":2,"aa":6,"b":1,"` + eAcute + `":5}`},
		// Nested objects are sorted too, and arrays keep their order.
		{`{"z":{"y":1,"x":[3,1,2]},"a":null}`, `{"a":null,"z":{"x":[3,1,2],"y":1}}`},
	}
	for _, c := range cases {
		if got := canon(t, c.in); got != c.want {
			t.Errorf("Canonical(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStringEscapesAreExactlyTheTable(t *testing.T) {
	short := map[rune]string{0x08: `\b`, 0x09: `\t`, 0x0a: `\n`, 0x0c: `\f`, 0x0d: `\r`}
	for r := rune(0); r < 0x20; r++ {
		in := fmt.Sprintf(`"~u%04X"`, r) // uppercase in, to show the output's case is ours
		want, ok := short[r]
		if !ok {
			want = fmt.Sprintf(`~u%04x`, r)
		}
		if got := canon(t, u(in)); got != u(`"`+want+`"`) {
			t.Errorf("U+%04X: got %q, want %q", r, got, u(`"`+want+`"`))
		}
	}
	cases := []struct{ name, in, want string }{
		{"U+001F is lowercase", `"~u001F"`, `"~u001f"`},
		{"U+001A is lowercase", `"~u001A"`, `"~u001a"`},
		{"quote and backslash", `"\"\\"`, `"\"\\"`},
		{"solidus raw, even escaped", `"\/ /"`, `"/ /"`},
		{"HTML characters raw", `"<>&"`, `"<>&"`},
		{"HTML characters raw, even escaped", `"~u003c~u003e~u0026"`, `"<>&"`},
		{"U+007F raw", `"~u007f"`, "\"\x7f\""},
		{"U+2028 and U+2029 raw", `"~u2028~u2029"`, `"` + lsep + psep + `"`},
		{"U+2028 and U+2029 raw, sent raw", `"` + lsep + psep + `"`, `"` + lsep + psep + `"`},
		{"non-ASCII raw", `"Caf~u00e9 ` + grin + `"`, `"Caf` + eAcute + ` ` + grin + `"`},
		{"no normalisation: e + combining acute stays two", "\"e\xcc\x81\"", "\"e\xcc\x81\""},
		{"invalid UTF-8 becomes U+FFFD", "\"a\xffb\"", `"a` + fffd + `b"`},
		{"a lone surrogate becomes U+FFFD", `"~ud800x"`, `"` + fffd + `x"`},
		{"keys follow the same table", `{"<~u0001\t\/":1}`, `{"<~u0001\t/":1}`},
	}
	for _, c := range cases {
		if got, want := canon(t, u(c.in)), u(c.want); got != want {
			t.Errorf("%s: Canonical(%q) = %q, want %q", c.name, u(c.in), got, want)
		}
	}
}

func TestNumbersIntegersOnly(t *testing.T) {
	for _, in := range []string{"1.0", "1e3", "-1.5e-2", "0.5", "1E0", "0e0", "-0.0", `{"a":[2.25]}`} {
		refuses(t, json.RawMessage(in), ErrNotInteger)
	}
	for _, in := range []string{
		"9007199254740992", "-9007199254740992", // one past each bound
		"18446744073709551616", "-99999999999999999999999", // past int64 too
	} {
		refuses(t, json.RawMessage(in), ErrIntegerRange)
	}
	cases := []struct{ in, want string }{
		{"9007199254740991", "9007199254740991"},
		{"-9007199254740991", "-9007199254740991"},
		{"-0", "0"},
		{"0", "0"},
		{"-1", "-1"},
		{"[10,-20,0,-0]", "[10,-20,0,0]"},
	}
	for _, c := range cases {
		if got := canon(t, c.in); got != c.want {
			t.Errorf("Canonical(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	// A leading zero or a plus sign is not JSON at all.
	for _, in := range []string{"01", "+1", "-", "- 1"} {
		refuses(t, json.RawMessage(in), ErrNotJSON)
	}
	if MaxSafeInteger != 9007199254740991 {
		t.Errorf("MaxSafeInteger = %d", MaxSafeInteger)
	}
}

func TestDuplicateKeysAreSeen(t *testing.T) {
	cases := []struct{ in, field string }{
		{`{"a":1,"a":2}`, "a"},
		{`{"x":{"y":{"a":1,"b":0,"a":2}}}`, "x.y.a"}, // the repeating object is three deep
		{u(`{"a":1,"~u0061":2}`), "a"},               // one key, two spellings
		{`[0,{"k":[],"k":[]}]`, "[1].k"},
		{`{"a":{},"b":{"c":true,"c":true}}`, "b.c"},
	}
	for _, c := range cases {
		err := refuses(t, json.RawMessage(c.in), ErrDuplicateKey)
		if got := fieldOf(err); got != c.field {
			t.Errorf("Canonical(%s): error %v names field %q, want %q", c.in, err, got, c.field)
		}
	}
	// The same key in two different objects is not a duplicate.
	if got := canon(t, `{"a":{"a":1},"b":{"a":2}}`); got != `{"a":{"a":1},"b":{"a":2}}` {
		t.Errorf("sibling objects: %s", got)
	}
}

// fieldOf is the Field of err's *FieldError.
func fieldOf(err error) string {
	var fe *FieldError
	if errors.As(err, &fe) {
		return fe.Field
	}
	return "<no FieldError>"
}

func TestOneValueOnly(t *testing.T) {
	for _, in := range []string{
		`{"a":1}{"b":2}`, `{"a":1} x`, ``, "  \n\t ", `1 2`, `[1] [2]`, `"a""b"`,
		`[1,]`, `{"a":1,}`, `{"a" 1}`, `{1:2}`, `[1 2]`, `nul`, `"open`, `{"a":1`, `]`, bom + "{}",
	} {
		refuses(t, json.RawMessage(in), ErrNotJSON)
	}
	// Whitespace around the one value is not a second value, and none survives.
	if got := canon(t, " \r\n\t{ \"a\" : [ 1 , 2 ] }\n "); got != `{"a":[1,2]}` {
		t.Errorf("whitespace: %q", got)
	}
}

func TestDepthGuard(t *testing.T) {
	nest := func(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }
	refuses(t, json.RawMessage(nest(MaxCanonicalDepth+1)), ErrTooDeep)
	if got := canon(t, nest(MaxCanonicalDepth)); got != nest(MaxCanonicalDepth) {
		t.Errorf("%d nested arrays: %q", MaxCanonicalDepth, got)
	}
	objs := func(n int) string { return strings.Repeat(`{"a":`, n) + "0" + strings.Repeat("}", n) }
	refuses(t, json.RawMessage(objs(33)), ErrTooDeep)
	if got := canon(t, objs(32)); got != objs(32) {
		t.Errorf("32 nested objects: %q", got)
	}
	// A hostile body: a million openers and no closer is refused for its
	// depth, long before the end, and without exhausting the stack.
	refuses(t, json.RawMessage(strings.Repeat("[", 1<<20)), ErrTooDeep)
	if MaxCanonicalDepth != 32 {
		t.Errorf("MaxCanonicalDepth = %d", MaxCanonicalDepth)
	}
}

// genPieces are what generated strings are made of: every escape class, and
// every character the table writes raw that an HTML-safe encoder would not.
var genPieces = []string{
	"a", "Z", "0", " ", `"`, `\`, "/", "<", ">", "&", "\x00", "\x01", "\x1f", "\b", "\t", "\n",
	"\f", "\r", "\x7f", eAcute, grin, pua, lsep, psep, fffd, "e\xcc\x81",
}

func genString(r *rand.Rand, max int) string {
	var b strings.Builder
	for i := r.Intn(max + 1); i > 0; i-- {
		b.WriteString(genPieces[r.Intn(len(genPieces))])
	}
	return b.String()
}

// genJSON writes a random JSON value as text: keys in random order, random
// whitespace, strings escaped by encoding/json (so "<" arrives as an escape).
func genJSON(r *rand.Rand, depth int, b *strings.Builder) {
	ws := func() { b.WriteString([]string{"", " ", "\n\t", "\r\n  "}[r.Intn(4)]) }
	kind := r.Intn(7)
	if depth >= 5 {
		kind = r.Intn(4)
	}
	switch kind {
	case 0:
		b.WriteString("null")
	case 1:
		b.WriteString(strconv.FormatBool(r.Intn(2) == 0))
	case 2:
		n := r.Int63n(2*MaxSafeInteger+1) - MaxSafeInteger
		if r.Intn(2) == 0 {
			n %= 1000
		}
		b.WriteString(strconv.FormatInt(n, 10))
	case 3:
		s, _ := json.Marshal(genString(r, 8))
		b.Write(s)
	case 4, 5:
		seen, keys := map[string]bool{}, []string{}
		for i := r.Intn(6); i > 0; i-- {
			if k := genString(r, 3); !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		r.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		b.WriteString("{")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(",")
			}
			ks, _ := json.Marshal(k)
			ws()
			b.Write(ks)
			ws()
			b.WriteString(":")
			ws()
			genJSON(r, depth+1, b)
		}
		b.WriteString("}")
	case 6:
		b.WriteString("[")
		for i := r.Intn(5); i > 0; i-- {
			genJSON(r, depth+1, b)
			if i > 1 {
				b.WriteString(",")
			}
			ws()
		}
		b.WriteString("]")
	}
}

// decodeAny reads JSON text into Go values, numbers kept as text, so two texts
// can be compared for meaning.
func decodeAny(t *testing.T, b []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", b, err)
	}
	return v
}

func TestCanonicalIsIdempotent(t *testing.T) {
	inputs := []string{vector1Bytes, vector2Bytes, vector3Bytes, vector4Bytes}
	r := rand.New(rand.NewSource(20260923))
	for i := 0; i < 50; i++ {
		var b strings.Builder
		genJSON(r, 1, &b)
		inputs = append(inputs, `{"v":`+b.String()+`}`) // under a key, so objects dominate
	}
	for i, in := range inputs {
		once, err := Canonical(json.RawMessage(in))
		if err != nil {
			t.Fatalf("input %d %q: %v", i, in, err)
		}
		twice, err := Canonical(once)
		if err != nil {
			t.Fatalf("input %d: Canonical(Canonical(x)): %v", i, err)
		}
		if !bytes.Equal(once, twice) {
			t.Errorf("input %d: not idempotent:\n once %q\ntwice %q", i, once, twice)
		}
		// The encoding keeps the value: nothing dropped, reordered in an
		// array, or respelled.
		if !reflect.DeepEqual(decodeAny(t, []byte(in)), decodeAny(t, once)) {
			t.Errorf("input %d: Canonical changed the value:\n in %q\nout %q", i, in, once)
		}
	}
}

func TestGoValuesGoThroughTheSameRules(t *testing.T) {
	type holder struct {
		S string          `json:"s"`
		C json.RawMessage `json:"c"`
	}
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"a map, sorted", map[string]any{"b": 1, "a": "x"}, `{"a":"x","b":1}`},
		{"what encoding/json escapes is raw", holder{"<>&" + lsep + psep, json.RawMessage(`{"z":"<","a":-0}`)},
			`{"c":{"a":0,"z":"<"},"s":"<>&` + lsep + psep + `"}`},
		{"[]byte is JSON text, not base64", []byte(`{"b":1,"a":2}`), `{"a":2,"b":1}`},
		{"a string", "Caf" + eAcute + "\x01", `"Caf` + eAcute + u(`~u0001"`)},
		{"nil", nil, "null"},
		{"an integral float64 marshals as an integer", map[string]any{"n": float64(3)}, `{"n":3}`},
		{"the bound itself", []any{int64(MaxSafeInteger), int64(-MaxSafeInteger)}, "[9007199254740991,-9007199254740991]"},
	}
	for _, c := range cases {
		got, err := Canonical(c.in)
		if err != nil || string(got) != c.want {
			t.Errorf("%s: Canonical = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
	refuses(t, map[string]any{"a": float64(1.5)}, ErrNotInteger)
	refuses(t, map[string]any{"a": []any{float64(1e21)}}, ErrNotInteger) // marshals as 1e+21
	refuses(t, map[string]any{"a": int64(1) << 53}, ErrIntegerRange)
	refuses(t, map[string]any{"a": uint64(1) << 63}, ErrIntegerRange)
	refuses(t, holder{C: json.RawMessage(`{"a":1,"a":2}`)}, ErrDuplicateKey)
	refuses(t, holder{C: json.RawMessage(strings.Repeat("[", 40) + strings.Repeat("]", 40))}, ErrTooDeep)
	refuses(t, map[string]any{"c": make(chan int)}, ErrNotJSON) // encoding/json cannot marshal it
	refuses(t, holder{C: json.RawMessage(`{`)}, ErrNotJSON)
}

func TestFailuresNameTheirField(t *testing.T) {
	cases := []struct {
		in    string
		want  error
		field string
	}{
		{`{"content":{"n":1.5}}`, ErrNotInteger, "content.n"},
		{`{"a":[1,2,{"b":1e3}]}`, ErrNotInteger, "a[2].b"},
		{`[9007199254740992]`, ErrIntegerRange, "[0]"},
		{`1.5`, ErrNotInteger, ""},
		{`{"a":` + strings.Repeat("[", 40), ErrTooDeep, "a" + strings.Repeat("[0]", 31)}, // the 33rd container
	}
	for _, c := range cases {
		err := refuses(t, json.RawMessage(c.in), c.want)
		if got := fieldOf(err); got != c.field {
			t.Errorf("Canonical(%.40s…): field %q, want %q", c.in, got, c.field)
		}
		if c.field != "" && !strings.Contains(err.Error(), c.field) {
			t.Errorf("Canonical(%.40s…): message %q does not name %q", c.in, err, c.field)
		}
	}
}
