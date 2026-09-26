package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf16"
)

// Canonical returns the canonical encoding of v (contract §3.4): RFC 8785 (JCS)
// restricted to what the Hub accepts.
//
//   - v is a json.RawMessage or []byte of JSON, parsed here; or any other Go
//     value, first marshalled with encoding/json and then parsed here. Either way
//     the bytes are read token by token, so a duplicate object key is SEEN
//     (encoding/json's own decoding silently keeps the last one).
//   - Objects: members sorted by key as sequences of UTF-16 code units; no
//     duplicate keys; no whitespace anywhere.
//   - Strings: UTF-8. `"` → \", `\` → \\, U+0008 \b, U+0009 \t, U+000A \n,
//     U+000C \f, U+000D \r, every other code point below U+0020 → \u00xx in
//     LOWERCASE hex. Everything else — "/", U+007F, U+2028, U+2029, "<", ">",
//     "&", non-ASCII — is written as its UTF-8 bytes. No Unicode normalisation.
//     Invalid UTF-8 in the input has already become U+FFFD by the time the
//     decoder yields the string; that replacement is what is encoded.
//   - Numbers: integers only, in [-(2^53-1), 2^53-1], decimal, no leading zero,
//     no plus sign, no exponent; "-0" is written "0". A fraction or an exponent
//     is ErrNotInteger, even when its value is integral ("1.0", "1e3").
//   - true, false, null as themselves.
//   - Nesting deeper than MaxCanonicalDepth is ErrTooDeep.
//
// The output is itself canonical: Canonical(Canonical(x)) == Canonical(x).
//
// A failure is a *FieldError naming where it was found, wrapping one of the
// Err* values below.
func Canonical(v any) ([]byte, error) {
	tree, err := parse(v)
	if err != nil {
		return nil, err
	}
	return encode(tree), nil
}

// The parsed tree: object, array, string, int64, bool or nil. An object's
// members are sorted when it is read, so encode only walks.
type (
	member struct {
		key   string
		units []uint16 // key as UTF-16 code units: the sort key
		val   any
	}
	object []member
	array  []any
)

// parse reads v, JSON text or a Go value, into the tree.
func parse(v any) (any, error) {
	var text []byte
	switch x := v.(type) {
	case json.RawMessage:
		text = x
	case []byte:
		text = x
	default:
		// The only marshal: a Go value becomes JSON text, then is read by the
		// same reader as text from the wire. Its escaping never reaches the
		// output, because every string is decoded and written again below.
		m, err := json.Marshal(v)
		if err != nil {
			return nil, &FieldError{Err: ErrNotJSON}
		}
		text = m
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()
	root, err := readValue(dec, 0, "")
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF { // a second value, or trailing bytes
		return nil, &FieldError{Err: ErrNotJSON}
	}
	return root, nil
}

// readValue reads one value at path, inside depth containers.
func readValue(dec *json.Decoder, depth int, path string) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, &FieldError{path, ErrNotJSON}
	}
	switch t := tok.(type) {
	case json.Delim:
		if t != '{' && t != '[' {
			return nil, &FieldError{path, ErrNotJSON}
		}
		if depth+1 > MaxCanonicalDepth { // before reading any of it
			return nil, &FieldError{path, ErrTooDeep}
		}
		if t == '{' {
			return readObject(dec, depth+1, path)
		}
		return readArray(dec, depth+1, path)
	case json.Number:
		return readInt(t, path)
	case string, bool, nil:
		return t, nil
	}
	return nil, &FieldError{path, ErrNotJSON}
}

// readObject reads members until "}". The decoder has checked the grammar
// (colons, commas, string keys); what it cannot see is a repeated key.
func readObject(dec *json.Decoder, depth int, path string) (any, error) {
	obj := object{}
	seen := map[string]struct{}{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, &FieldError{path, ErrNotJSON}
		}
		key, ok := tok.(string)
		if !ok {
			return nil, &FieldError{path, ErrNotJSON}
		}
		at := join(path, key)
		if _, dup := seen[key]; dup { // compared decoded: an escaped spelling is the same key
			return nil, &FieldError{at, ErrDuplicateKey}
		}
		seen[key] = struct{}{}
		v, err := readValue(dec, depth, at)
		if err != nil {
			return nil, err
		}
		obj = append(obj, member{key: key, units: utf16.Encode([]rune(key)), val: v})
	}
	if _, err := dec.Token(); err != nil { // the "}"
		return nil, &FieldError{path, ErrNotJSON}
	}
	slices.SortFunc(obj, func(a, b member) int { return slices.Compare(a.units, b.units) })
	return obj, nil
}

func readArray(dec *json.Decoder, depth int, path string) (any, error) {
	arr := array{}
	for i := 0; dec.More(); i++ {
		v, err := readValue(dec, depth, path+"["+strconv.Itoa(i)+"]")
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	if _, err := dec.Token(); err != nil { // the "]"
		return nil, &FieldError{path, ErrNotJSON}
	}
	return arr, nil
}

// integer is JSON's integer grammar: no fraction, no exponent.
var integer = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func readInt(n json.Number, path string) (any, error) {
	if !integer.MatchString(string(n)) {
		return nil, &FieldError{path, ErrNotInteger}
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || i > MaxSafeInteger || i < -MaxSafeInteger {
		return nil, &FieldError{path, ErrIntegerRange}
	}
	return i, nil // "-0" is now 0, and is written "0"
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func encode(v any) []byte {
	var b bytes.Buffer
	writeValue(&b, v)
	return b.Bytes()
}

func writeValue(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case string:
		writeString(b, x)
	case array:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeValue(b, e)
		}
		b.WriteByte(']')
	case object:
		b.WriteByte('{')
		for i, m := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, m.key)
			b.WriteByte(':')
			writeValue(b, m.val)
		}
		b.WriteByte('}')
	}
}

const lowerHex = "0123456789abcdef"

// writeString is the escape table of contract §3.4 and nothing else: seven
// short escapes, the rest of C0 as a lowercase six-character escape, and every
// other code point as its UTF-8 bytes. s came from the decoder, so it is valid
// UTF-8; a range over it could only yield U+FFFD for a bad byte, as the
// decoder did.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(lowerHex[r>>4])
				b.WriteByte(lowerHex[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// MaxCanonicalDepth bounds recursion. The brief envelope is depth 1 and its
// content may be 16 deep (ValidateContent), so 32 is never reached by a valid
// write and exists only so a hostile body cannot exhaust the stack.
const MaxCanonicalDepth = 32

// MaxSafeInteger is 2^53 − 1, the largest integer every JSON reader agrees on.
const MaxSafeInteger = 1<<53 - 1

// Every failure wraps exactly one of these; the Hub maps each to 400
// bad_request with details.field naming where it was found.
var (
	ErrNotJSON      = errors.New("hubapi: not one JSON value")
	ErrDuplicateKey = errors.New("hubapi: an object repeats a key")
	ErrNotInteger   = errors.New("hubapi: a number with a fraction or an exponent")
	ErrIntegerRange = errors.New("hubapi: an integer outside ±(2^53−1)")
	ErrTooDeep      = errors.New("hubapi: nested too deeply")
)

// FieldError is how every failure of Canonical, ValidateContent, the envelopes
// and their parsers is returned: which rule (Err, one of the Err* values), and
// where (Field). The Hub answers it 400 bad_request with details.field, putting
// its own prefix in front ("revisions[3].content." + Field).
type FieldError struct {
	// Field is the path from the value's root: object members by key, joined
	// with ".", array elements as "[i]" ("answers[2].text"). "" is the root.
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	if e.Field == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + " (at " + e.Field + ")"
}

func (e *FieldError) Unwrap() error { return e.Err }
