package hubapi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ── the golden vectors (contract §3.5; vectors 3 and 4 are plan correction
// A10's, with teamId). Every byte, as the part records them. ─────────────────

var (
	vector1 = BriefEnvelope{BriefID: "bf-00000000000000a1", Revision: 1, ProjectID: "pj-00000000000000b2",
		Scope: ScopeBranch, BranchRef: "refs/heads/evidence-upload", Content: json.RawMessage(`{"answers":[],"status":"open"}`)}
	vector1Bytes  = `{"branchRef":"refs/heads/evidence-upload","briefId":"bf-00000000000000a1","content":{"answers":[],"status":"open"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":1,"scope":"branch","v":1}`
	vector1Digest = "sha256:a72a06a155457ac321227e660a220ab030f0367a5184f3f8ac8e8f636cd5e9fa"

	// The content as the writer sent it: "é" raw, a JSON \t and a JSON
	// backslash-u 0001 escape, -0, the largest safe integer, keys unsorted.
	vector2Content = u(`{"status":"complete","note":"Caf` + eAcute + `\tline~u0001end","n":-0,"big":9007199254740991,"ok":true,"none":null}`)
	vector2Bytes   = u(`{"branchRef":"","briefId":"bf-00000000000000a1","content":{"big":9007199254740991,"n":0,"none":null,"note":"Caf` + eAcute + `\tline~u0001end","ok":true,"status":"complete"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":2,"scope":"project","v":1}`)
	vector2Digest  = "sha256:fa6e8ac2a66bf14e2b0ad2ffb02afac6b6143d042e52f5bdc5a051a0c43d8537"

	vector3 = AuditEnvelope{OrgID: "org-00000000000000c3", Seq: 1, At: "2026-09-23T04:02:11.123Z",
		ActorUserID: "u-00000000000000d4", Event: "team.created", TeamID: "tm-00000000000000e5",
		ProjectID: "", Subject: "tm-00000000000000e5", Data: map[string]any{"name": "Engagement Portal"}}
	vector3Bytes = `{"actorUserId":"u-00000000000000d4","at":"2026-09-23T04:02:11.123Z","data":{"name":"Engagement Portal"},"event":"team.created","kind":"audit-event","orgId":"org-00000000000000c3","projectId":"","seq":1,"subject":"tm-00000000000000e5","teamId":"tm-00000000000000e5","v":1}`
	vector3Hash  = "c6a78d78c4d773a8cad34957f8c0f42cbc1e72e552c3dbf9eceee1f96455fbaa"

	vector4 = AuditEnvelope{OrgID: "org-00000000000000c3", Seq: 2, At: "2026-09-23T04:02:12.000Z",
		ActorUserID: "", Event: "release.expired", TeamID: "tm-00000000000000e5",
		ProjectID: "pj-00000000000000b2", Subject: "rl-00000000000000f6", Data: map[string]any{}}
	vector4Bytes = `{"actorUserId":"","at":"2026-09-23T04:02:12.000Z","data":{},"event":"release.expired","kind":"audit-event","orgId":"org-00000000000000c3","projectId":"pj-00000000000000b2","seq":2,"subject":"rl-00000000000000f6","teamId":"tm-00000000000000e5","v":1}`
	vector4Hash  = "bc7822374dfa797a915c66eb874ecdab5d763c77155b6c15a82f26faedd2010c"
)

// briefBytes asserts e's canonical bytes are want, length and content, and
// that Digest agrees.
func briefBytes(t *testing.T, e BriefEnvelope, wantLen int, want, wantDigest string) {
	t.Helper()
	got, err := e.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if len(got) != wantLen || string(got) != want {
		t.Fatalf("Canonical = %d bytes\n%s\nwant %d bytes\n%s", len(got), got, wantLen, want)
	}
	d, c, err := e.Digest()
	if err != nil || d.String() != wantDigest || !bytes.Equal(c, got) {
		t.Fatalf("Digest = %s, %q, %v; want %s and the canonical bytes", d, c, err, wantDigest)
	}
}

func TestVector1(t *testing.T) {
	briefBytes(t, vector1, 209, vector1Bytes, vector1Digest)
}

func TestVector2(t *testing.T) {
	content, status, err := ValidateContent(json.RawMessage(vector2Content))
	if err != nil || status != StatusComplete {
		t.Fatalf("ValidateContent = %q, %q, %v", content, status, err)
	}
	e := BriefEnvelope{BriefID: "bf-00000000000000a1", Revision: 2, ProjectID: "pj-00000000000000b2",
		Scope: ScopeProject, BranchRef: "", Content: content}
	briefBytes(t, e, 256, vector2Bytes, vector2Digest)
	// The content as sent, not yet canonical, gives the same bytes.
	e.Content = json.RawMessage(vector2Content)
	briefBytes(t, e, 256, vector2Bytes, vector2Digest)
}

// auditBytes asserts a's canonical bytes are want and returns them.
func auditBytes(t *testing.T, a AuditEnvelope, wantLen int, want string) []byte {
	t.Helper()
	got, err := a.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if len(got) != wantLen || string(got) != want {
		t.Fatalf("Canonical = %d bytes\n%s\nwant %d bytes\n%s", len(got), got, wantLen, want)
	}
	return got
}

func TestVector3(t *testing.T) {
	c := auditBytes(t, vector3, 271, vector3Bytes)
	if h := ChainHash([32]byte{}, c); hex.EncodeToString(h[:]) != vector3Hash {
		t.Fatalf("ChainHash(zero, v3) = %x, want %s", h, vector3Hash)
	}
}

func TestVector4(t *testing.T) {
	c := auditBytes(t, vector4, 249, vector4Bytes)
	var prev [32]byte
	if _, err := hex.Decode(prev[:], []byte(vector3Hash)); err != nil {
		t.Fatal(err)
	}
	if h := ChainHash(prev, c); hex.EncodeToString(h[:]) != vector4Hash {
		t.Fatalf("ChainHash(v3 hash, v4) = %x, want %s", h, vector4Hash)
	}
	// The link is real: the same bytes on another predecessor hash differently.
	if h := ChainHash([32]byte{}, c); hex.EncodeToString(h[:]) == vector4Hash {
		t.Fatal("ChainHash ignores prevHash")
	}
	// A nil Data is the empty object, never null.
	a := vector4
	a.Data = nil
	auditBytes(t, a, 249, vector4Bytes)
}

func TestValidateContentRules(t *testing.T) {
	for _, in := range []string{`[]`, `"x"`, `null`, `1`, `true`} {
		if _, _, err := ValidateContent(json.RawMessage(in)); !errors.Is(err, ErrContentNotObject) {
			t.Errorf("ValidateContent(%s) = %v, want ErrContentNotObject", in, err)
		}
	}
	for _, in := range []string{`{}`, `{"status":"done"}`, `{"status":1}`, `{"status":null}`,
		`{"status":"Open"}`, `{"status":"open "}`, `{"status":["open"]}`, `{"Status":"open"}`} {
		_, _, err := ValidateContent(json.RawMessage(in))
		if !errors.Is(err, ErrContentStatus) || fieldOf(err) != "status" {
			t.Errorf("ValidateContent(%s) = %v (field %q), want ErrContentStatus at status", in, err, fieldOf(err))
		}
	}
	// §7.5's depth rule, built once of arrays and once of objects: each branch
	// of the check is held on its own (review task-02 I2: the object branch
	// could be deleted with every test green).
	deepArrays := func(n int) string { // n containers, the content object among them
		return `{"status":"open","d":` + strings.Repeat("[", n-1) + strings.Repeat("]", n-1) + `}`
	}
	deepObjects := func(n int) string { // n containers again, as {"a":{"a":…0…}}
		return `{"status":"open","d":` + strings.Repeat(`{"a":`, n-1) + `0` + strings.Repeat("}", n-1) + `}`
	}
	for _, c := range []struct{ name, in, field string }{
		{"17 arrays", deepArrays(17), "d" + strings.Repeat("[0]", 15)},
		{"17 objects", deepObjects(17), "d" + strings.Repeat(".a", 15)},
	} {
		_, _, err := ValidateContent(json.RawMessage(c.in))
		if !errors.Is(err, ErrContentDepth) || fieldOf(err) != c.field {
			t.Errorf("%s: ValidateContent = %v (field %q), want ErrContentDepth at %s", c.name, err, fieldOf(err), c.field)
		}
	}
	for name, in := range map[string]string{"16 arrays": deepArrays(16), "16 objects": deepObjects(16)} {
		if _, _, err := ValidateContent(json.RawMessage(in)); err != nil {
			t.Errorf("%s: ValidateContent = %v, want nil", name, err)
		}
	}
	// Returns the canonical bytes and the status; requires no other field
	// and reads none (contract §7.5).
	cases := []struct{ in, canonical, status string }{
		{` { "status" : "skipped" , "b":2,"a":1 } `, `{"a":1,"b":2,"status":"skipped"}`, StatusSkipped},
		{`{"status":"open"}`, `{"status":"open"}`, StatusOpen},
		{`{"answers":{"x":[1,"<"]},"status":"complete","title":null}`, `{"answers":{"x":[1,"<"]},"status":"complete","title":null}`, StatusComplete},
	}
	for _, c := range cases {
		got, status, err := ValidateContent(json.RawMessage(c.in))
		if err != nil || string(got) != c.canonical || status != c.status {
			t.Errorf("ValidateContent(%s) = %s, %q, %v; want %s, %q", c.in, got, status, err, c.canonical, c.status)
		}
	}
	// The encoding's own rules still hold inside content.
	if _, _, err := ValidateContent(json.RawMessage(`{"status":"open","status":"open"}`)); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("duplicate status = %v", err)
	}
	if _, _, err := ValidateContent(json.RawMessage(`{"status":"open","n":1.5}`)); !errors.Is(err, ErrNotInteger) {
		t.Errorf("a float in content = %v", err)
	}
}

func TestEnvelopeSizeLimit(t *testing.T) {
	if MaxEnvelopeBytes != 65536 {
		t.Fatalf("MaxEnvelopeBytes = %d", MaxEnvelopeBytes)
	}
	withPad := func(pad string) BriefEnvelope {
		e := vector1
		e.Content = json.RawMessage(`{"pad":"` + pad + `","status":"open"}`)
		return e
	}
	empty, err := withPad("").Canonical()
	if err != nil {
		t.Fatal(err)
	}
	n := MaxEnvelopeBytes - len(empty)
	at, err := withPad(strings.Repeat("a", n)).Canonical()
	if err != nil || len(at) != 65536 {
		t.Fatalf("a 65,536-byte envelope: %d bytes, %v", len(at), err)
	}
	if _, err := ParseBriefEnvelope(at); err != nil {
		t.Fatalf("ParseBriefEnvelope(65,536 bytes) = %v", err)
	}
	over := withPad(strings.Repeat("a", n+1))
	c, err := over.Canonical()
	if !errors.Is(err, ErrEnvelopeTooLarge) || c != nil || fieldOf(err) != "content" {
		t.Fatalf("a 65,537-byte envelope: %d bytes, %v (field %q)", len(c), err, fieldOf(err))
	}
	if _, _, err := over.Digest(); !errors.Is(err, ErrEnvelopeTooLarge) {
		t.Fatalf("Digest of 65,537 bytes = %v", err)
	}
	// The cap is on the envelope: the content alone is well under it.
	if len(over.Content) >= MaxEnvelopeBytes-100 {
		t.Fatalf("the test's content is %d bytes; it must be under the cap by the envelope's size", len(over.Content))
	}
	// Bytes, not characters: 'é' is two, so half as many of them fill it.
	if _, err := withPad(strings.Repeat(eAcute, n/2+1)).Canonical(); !errors.Is(err, ErrEnvelopeTooLarge) {
		t.Fatalf("%d x U+00E9 = %v, want ErrEnvelopeTooLarge", n/2+1, err)
	}
	// Canonical bytes, not wire bytes: whitespace the writer sent does not count.
	e := vector1
	e.Content = json.RawMessage(`{"pad":"` + strings.Repeat("a", n) + `",` + strings.Repeat(" ", 4096) + `"status":"open"}`)
	if c, err := e.Canonical(); err != nil || len(c) != 65536 {
		t.Fatalf("padded with whitespace: %d bytes, %v", len(c), err)
	}
}

func TestParseBriefEnvelopeRoundTrips(t *testing.T) {
	v2content, _, err := ValidateContent(json.RawMessage(vector2Content))
	if err != nil {
		t.Fatal(err)
	}
	vector2 := BriefEnvelope{BriefID: "bf-00000000000000a1", Revision: 2, ProjectID: "pj-00000000000000b2",
		Scope: ScopeProject, Content: v2content}
	// Characters encoding/json would escape, and a writer's key order.
	html := vector1
	html.Content = json.RawMessage(`{"status":"skipped","s":"<>&` + lsep + `"}`)
	htmlBytes, err := html.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	html.Content = json.RawMessage(`{"s":"<>&` + lsep + `","status":"skipped"}`)

	for _, c := range []struct {
		name string
		in   string
		want BriefEnvelope
	}{{"vector 1", vector1Bytes, vector1}, {"vector 2", vector2Bytes, vector2}, {"raw HTML", string(htmlBytes), html}} {
		in := []byte(c.in)
		got, err := ParseBriefEnvelope(in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !bytes.Equal(got.Content, c.want.Content) {
			t.Errorf("%s: Content = %s, want %s", c.name, got.Content, c.want.Content)
		}
		gotNoContent, wantNoContent := got, c.want
		gotNoContent.Content, wantNoContent.Content = nil, nil
		if !reflect.DeepEqual(gotNoContent, wantNoContent) {
			t.Errorf("%s: %+v, want %+v", c.name, gotNoContent, wantNoContent)
		}
		// Content is its own copy: a caller reusing its buffer does not change it.
		want := append([]byte(nil), got.Content...)
		for i := range in {
			in[i] = 'x'
		}
		if !bytes.Equal(got.Content, want) {
			t.Errorf("%s: Content aliases the input buffer", c.name)
		}
	}
}

func TestFormatTimeTruncates(t *testing.T) {
	aest := time.FixedZone("AEST", 10*60*60)
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, 9, 23, 4, 2, 11, 123900000, time.UTC), "2026-09-23T04:02:11.123Z"},
		{time.Date(2026, 9, 23, 4, 2, 11, 999999999, time.UTC), "2026-09-23T04:02:11.999Z"}, // never rounds up
		{time.Date(2026, 9, 23, 14, 2, 11, 123000000, aest), "2026-09-23T04:02:11.123Z"},
		{time.Date(2026, 9, 23, 4, 2, 12, 0, time.UTC), "2026-09-23T04:02:12.000Z"},
		{time.Date(2026, 9, 23, 4, 2, 12, 100000000, time.UTC), "2026-09-23T04:02:12.100Z"},
	}
	for _, c := range cases {
		if got := FormatTime(c.in); got != c.want {
			t.Errorf("FormatTime(%v) = %s, want %s", c.in, got, c.want)
		}
	}
	p, err := time.Parse(time.RFC3339Nano, "2026-09-23T04:02:11.1239Z")
	if err != nil || FormatTime(p) != "2026-09-23T04:02:11.123Z" {
		t.Errorf("…11.1239Z → %s, %v", FormatTime(p), err)
	}
}

// ── beyond the part's table: what the readers refuse (task 02 departure D2) ──

// v1Map is vector 1 as a Go map, for building near-misses of it.
func v1Map() map[string]any {
	return map[string]any{"v": 1, "kind": "brief-revision", "briefId": "bf-00000000000000a1", "revision": 1,
		"projectId": "pj-00000000000000b2", "scope": "branch", "branchRef": "refs/heads/evidence-upload",
		"content": map[string]any{"answers": []any{}, "status": "open"}}
}

func TestAStoredRevisionMustBeCanonical(t *testing.T) {
	if c, err := Canonical(v1Map()); err != nil || string(c) != vector1Bytes {
		t.Fatalf("v1Map is not vector 1: %s, %v", c, err)
	}
	mutate := func(f func(m map[string]any)) string {
		m := v1Map()
		f(m)
		c, err := Canonical(m)
		if err != nil {
			t.Fatal(err)
		}
		return string(c)
	}
	cases := []struct {
		name  string
		in    string
		want  error
		field string // where: "" for bytes that parse but are not canonical
	}{
		{"keys out of order", strings.Replace(vector1Bytes, `"branchRef":"refs/heads/evidence-upload","briefId":"bf-00000000000000a1"`, `"briefId":"bf-00000000000000a1","branchRef":"refs/heads/evidence-upload"`, 1), ErrBadEnvelope, ""},
		{"whitespace", strings.Replace(vector1Bytes, `,"kind"`, `, "kind"`, 1), ErrBadEnvelope, ""},
		{"-0 in content", strings.Replace(vector1Bytes, `"status":"open"}`, `"status":"open","z":-0}`, 1), ErrBadEnvelope, ""},
		{"an escape where raw is canonical", strings.Replace(vector1Bytes, `brief-revision`, u(`brief~u002drevision`), 1), ErrBadEnvelope, ""},
		{"an extra key", mutate(func(m map[string]any) { m["extra"] = 1 }), ErrBadEnvelope, "extra"},
		{"no v", mutate(func(m map[string]any) { delete(m, "v") }), ErrBadEnvelope, "v"},
		{"v 2", mutate(func(m map[string]any) { m["v"] = 2 }), ErrBadEnvelope, "v"},
		{"another kind", mutate(func(m map[string]any) { m["kind"] = "audit-event" }), ErrBadEnvelope, "kind"},
		{"an audit event", vector3Bytes, ErrBadEnvelope, "actorUserId"},
		{"revision 0", mutate(func(m map[string]any) { m["revision"] = 0 }), ErrBadEnvelope, "revision"},
		{"revision as a string", mutate(func(m map[string]any) { m["revision"] = "1" }), ErrBadEnvelope, "revision"},
		{"a project's id as the brief's", mutate(func(m map[string]any) { m["briefId"] = "pj-00000000000000a1" }), ErrBadEnvelope, "briefId"},
		{"a brief's id as the project's", mutate(func(m map[string]any) { m["projectId"] = "bf-00000000000000b2" }), ErrBadEnvelope, "projectId"},
		{"scope unknown", mutate(func(m map[string]any) { m["scope"] = "team" }), ErrBadEnvelope, "scope"},
		{"project scope with a ref", mutate(func(m map[string]any) { m["scope"] = "project" }), ErrBadEnvelope, "branchRef"},
		{"branch scope, no ref", mutate(func(m map[string]any) { m["branchRef"] = "" }), ErrBadEnvelope, "branchRef"},
		{"not a branch ref", mutate(func(m map[string]any) { m["branchRef"] = "refs/tags/v1" }), ErrBadEnvelope, "branchRef"},
		{"content status", mutate(func(m map[string]any) { m["content"] = map[string]any{"status": "done"} }), ErrContentStatus, "content.status"},
		{"content not an object", mutate(func(m map[string]any) { m["content"] = []any{} }), ErrContentNotObject, "content"},
		{"not JSON", vector1Bytes[:100], ErrNotJSON, "content"}, // cut inside content's members
		{"a repeated key", strings.Replace(vector1Bytes, `"v":1}`, `"v":1,"v":1}`, 1), ErrDuplicateKey, "v"},
		// The parser's own type and shape checks name the field (review task-02
		// M1): without them a project brief's non-string branchRef reads as ""
		// and is refused only at the root, and a non-object root at "v".
		{"a project brief's branchRef not a string", mutate(func(m map[string]any) { m["scope"], m["branchRef"] = "project", 5 }), ErrBadEnvelope, "branchRef"},
		{"an array", `[]`, ErrBadEnvelope, ""},
		{"a string", `"brief-revision"`, ErrBadEnvelope, ""},
	}
	for _, c := range cases {
		got, err := ParseBriefEnvelope([]byte(c.in))
		if !errors.Is(err, c.want) || fieldOf(err) != c.field {
			t.Errorf("%s: ParseBriefEnvelope = %+v, %v (field %q); want %v at %q", c.name, got, err, fieldOf(err), c.want, c.field)
		}
	}
}

func TestEnvelopesRefuseMalformedFields(t *testing.T) {
	brief := []struct {
		field string
		f     func(e *BriefEnvelope)
		want  error
	}{
		{"briefId", func(e *BriefEnvelope) { e.BriefID = "bf-1" }, ErrBadEnvelope},
		{"revision", func(e *BriefEnvelope) { e.Revision = 0 }, ErrBadEnvelope},
		{"projectId", func(e *BriefEnvelope) { e.ProjectID = "" }, ErrBadEnvelope},
		{"scope", func(e *BriefEnvelope) { e.Scope = "Branch" }, ErrBadEnvelope},
		{"branchRef", func(e *BriefEnvelope) { e.Scope = ScopeProject }, ErrBadEnvelope},
		{"branchRef", func(e *BriefEnvelope) { e.BranchRef = "" }, ErrBadEnvelope},
		{"branchRef", func(e *BriefEnvelope) { e.BranchRef = "refs/heads/a..b" }, ErrBadEnvelope},
		{"content", func(e *BriefEnvelope) { e.Content = nil }, ErrNotJSON},
		{"content", func(e *BriefEnvelope) { e.Content = json.RawMessage(`null`) }, ErrContentNotObject},
		{"content.status", func(e *BriefEnvelope) { e.Content = json.RawMessage(`{"status":"x"}`) }, ErrContentStatus},
		{"content.n", func(e *BriefEnvelope) { e.Content = json.RawMessage(`{"n":1.5,"status":"open"}`) }, ErrNotInteger},
	}
	for _, c := range brief {
		e := vector1
		c.f(&e)
		out, err := e.Canonical()
		if !errors.Is(err, c.want) || fieldOf(err) != c.field || out != nil {
			t.Errorf("brief %s: Canonical = %q, %v (field %q); want %v at %s", c.field, out, err, fieldOf(err), c.want, c.field)
		}
	}
	audit := []struct {
		field string
		f     func(a *AuditEnvelope)
	}{
		{"orgId", func(a *AuditEnvelope) { a.OrgID = "tm-00000000000000c3" }},
		{"seq", func(a *AuditEnvelope) { a.Seq = 0 }},
		{"at", func(a *AuditEnvelope) { a.At = "2026-09-23T04:02:11Z" }},
		{"at", func(a *AuditEnvelope) { a.At = "2026-09-23T14:02:11.123+10:00" }},
		{"at", func(a *AuditEnvelope) { a.At = "2026-02-30T04:02:11.123Z" }},
		// time.Parse takes both of these with FormatTime's layout; only the
		// round trip refuses them (review task-02 I3). A one-digit hour is a
		// byte short; a comma before the fraction is FormatTime's length
		// exactly, so a length check alone would take it.
		{"at", func(a *AuditEnvelope) { a.At = "2026-09-23T4:02:11.123Z" }},
		{"at", func(a *AuditEnvelope) { a.At = "2026-09-23T04:02:11,123Z" }},
		{"actorUserId", func(a *AuditEnvelope) { a.ActorUserID = "org-00000000000000d4" }},
		{"event", func(a *AuditEnvelope) { a.Event = "" }},
		{"event", func(a *AuditEnvelope) { a.Event = "Team.Created" }},
		// audit_events.kind's CHECK (contract §10.2) refuses each of these, so
		// the envelope gives no bytes either (review task-02 I1).
		{"event", func(a *AuditEnvelope) { a.Event = "a.b.c" }},          // exactly one dot
		{"event", func(a *AuditEnvelope) { a.Event = "team2.created" }},  // no digits
		{"event", func(a *AuditEnvelope) { a.Event = "team_.created" }},  // "_" joins two words
		{"event", func(a *AuditEnvelope) { a.Event = "brief.revised_" }}, // at either end of a word
		{"event", func(a *AuditEnvelope) { a.Event = "_team.created" }},
		{"event", func(a *AuditEnvelope) { a.Event = "team" }},
		{"event", func(a *AuditEnvelope) { a.Event = "team.created\n" }}, // "$" is the end of the text
		{"teamId", func(a *AuditEnvelope) { a.TeamID = "pj-00000000000000e5" }},
		{"projectId", func(a *AuditEnvelope) { a.ProjectID = "tm-00000000000000e5" }},
		{"data.x", func(a *AuditEnvelope) { a.Data = map[string]any{"x": 1.5} }},
	}
	for _, c := range audit {
		a := vector3
		c.f(&a)
		out, err := a.Canonical()
		if !errors.Is(err, ErrBadEnvelope) || fieldOf(err) != c.field || out != nil {
			t.Errorf("audit %s: Canonical = %q, %v (field %q); want ErrBadEnvelope at %s", c.field, out, err, fieldOf(err), c.field)
		}
	}
	// The optional ids may be empty (the janitor has no actor; an org event
	// has no team or project), and the subject is free.
	a := vector3
	a.ActorUserID, a.TeamID, a.ProjectID, a.Subject = "", "", "", ""
	if _, err := a.Canonical(); err != nil {
		t.Errorf("empty optional ids: %v", err)
	}
}

func TestAuditDataTakesScalarsOnly(t *testing.T) {
	for name, v := range map[string]any{"float64": float64(1), "nil": nil, "map": map[string]any{},
		"slice": []any{"a"}, "json.Number": json.Number("1"), "uint64": uint64(1), "raw": json.RawMessage(`1`)} {
		a := vector4
		a.Data = map[string]any{"x": v}
		if _, err := a.Canonical(); !errors.Is(err, ErrBadEnvelope) || fieldOf(err) != "data.x" {
			t.Errorf("%s: %v (field %q), want ErrBadEnvelope at data.x", name, err, fieldOf(err))
		}
	}
	// Of several bad values, the first key in order is named, every time
	// (a map's range order would name either, at random).
	for i := 0; i < 20; i++ {
		a := vector4
		a.Data = map[string]any{"d": 1.5, "c": nil, "b": []any{}, "a2": "ok", "a": "ok"}
		if _, err := a.Canonical(); fieldOf(err) != "data.b" {
			t.Fatalf("several bad values: field %q, want data.b", fieldOf(err))
		}
	}
	a := vector4
	a.Data = map[string]any{"s": "<" + lsep, "i": 7, "n": int64(-MaxSafeInteger), "b": false}
	got, err := a.Canonical()
	want := `"data":{"b":false,"i":7,"n":-9007199254740991,"s":"<` + lsep + `"}`
	if err != nil || !strings.Contains(string(got), want) {
		t.Errorf("scalars: %s, %v; want %s inside", got, err, want)
	}
	a.Data = map[string]any{"n": int64(1) << 53}
	if _, err := a.Canonical(); !errors.Is(err, ErrIntegerRange) {
		t.Errorf("2^53 in data: %v, want ErrIntegerRange", err)
	}
}

func TestParseAuditEnvelopeRoundTrips(t *testing.T) {
	for _, c := range []struct {
		in   string
		want AuditEnvelope
	}{{vector3Bytes, vector3}, {vector4Bytes, vector4}} {
		got, err := ParseAuditEnvelope([]byte(c.in))
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseAuditEnvelope(%s) = %#v, %v; want %#v", c.in, got, err, c.want)
		}
	}
	// Integers come back as int64, whatever integer type went in.
	a := vector4
	a.Data = map[string]any{"i": 7, "n": int64(-3), "b": true, "s": "x"}
	c, err := a.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseAuditEnvelope(c)
	want := map[string]any{"i": int64(7), "n": int64(-3), "b": true, "s": "x"}
	if err != nil || !reflect.DeepEqual(got.Data, want) {
		t.Errorf("data round trip: %#v, %v", got.Data, err)
	}
	for name, c := range map[string]struct{ in, field string }{
		"a brief revision":    {vector1Bytes, "branchRef"},
		"keys out of order":   {strings.Replace(vector3Bytes, `"actorUserId":"u-00000000000000d4","at":"2026-09-23T04:02:11.123Z"`, `"at":"2026-09-23T04:02:11.123Z","actorUserId":"u-00000000000000d4"`, 1), ""},
		"no teamId (pre-A10)": {strings.Replace(vector3Bytes, `"teamId":"tm-00000000000000e5",`, ``, 1), "teamId"},
		"nested data":         {strings.Replace(vector3Bytes, `{"name":"Engagement Portal"}`, `{"name":{"x":1}}`, 1), "data.name"},
		"null data":           {strings.Replace(vector3Bytes, `{"name":"Engagement Portal"}`, `null`, 1), "data"},
		"null in data":        {strings.Replace(vector3Bytes, `"Engagement Portal"`, `null`, 1), "data.name"},
		"seq 0":               {strings.Replace(vector3Bytes, `"seq":1`, `"seq":0`, 1), "seq"},
		"seq as a string":     {strings.Replace(vector3Bytes, `"seq":1`, `"seq":"1"`, 1), "seq"},
		"kind brief-revision": {strings.Replace(vector3Bytes, `audit-event`, `brief-revision`, 1), "kind"},
		"an extra key":        {strings.Replace(vector3Bytes, `"v":1}`, `"v":1,"w":1}`, 1), "w"},
		"v 2":                 {strings.Replace(vector3Bytes, `"v":1}`, `"v":2}`, 1), "v"},
		// The parser's own type and shape checks name the field (review task-02
		// M1): without them these optional strings read as "", which is valid,
		// and the bytes are refused only at the root; a non-object root at "v".
		"actorUserId not a string": {strings.Replace(vector3Bytes, `"actorUserId":"u-00000000000000d4"`, `"actorUserId":false`, 1), "actorUserId"},
		"teamId not a string":      {strings.Replace(vector3Bytes, `"teamId":"tm-00000000000000e5"`, `"teamId":null`, 1), "teamId"},
		"projectId not a string":   {strings.Replace(vector3Bytes, `"projectId":""`, `"projectId":0`, 1), "projectId"},
		"subject not a string":     {strings.Replace(vector3Bytes, `"subject":"tm-00000000000000e5"`, `"subject":5`, 1), "subject"},
		"an array":                 {`[]`, ""},
		"a string":                 {`"audit-event"`, ""},
	} {
		got, err := ParseAuditEnvelope([]byte(c.in))
		if !errors.Is(err, ErrBadEnvelope) || fieldOf(err) != c.field {
			t.Errorf("%s: ParseAuditEnvelope = %#v, %v (field %q); want ErrBadEnvelope at %q", name, got, err, fieldOf(err), c.field)
		}
	}
}

// ── the event kind's one grammar (contract §10.2; review task-02 I1) ────────

// An event kind has one grammar: audit_events.kind's CHECK in contract §10.2,
// which task 07a's migration writes verbatim. eventKind is that pattern,
// character for character, so an envelope the INSERT would refuse gets no
// bytes (400 bad_request, not a CHECK violation inside the write's
// transaction), and hub verify reads events by the same rule. Every kind §8
// lists passes it.
func TestAuditEventKindIsTheContractsCheck(t *testing.T) {
	section := contractSection(t, "10.2")
	start := strings.Index(section, "CREATE TABLE audit_events (")
	if start < 0 {
		t.Fatal("contract §10.2 has no audit_events table")
	}
	table := section[start:]
	if end := strings.Index(table, "\n);"); end >= 0 {
		table = table[:end]
	}
	// A Postgres string literal under standard_conforming_strings (the
	// default): a backslash is itself, so the text between the quotes is the
	// pattern.
	check := regexp.MustCompile(`\n\s*kind\s+text\s+NOT NULL CHECK \(kind ~ '([^']+)'\)`).FindStringSubmatch(table)
	if check == nil {
		t.Fatal("contract §10.2's audit_events has no kind CHECK")
	}
	if eventKind.String() != check[1] {
		t.Errorf("eventKind is %s; audit_events.kind's CHECK is %s", eventKind, check[1])
	}

	kinds := contractEventKinds(t)
	if len(kinds) < 29 {
		t.Fatalf("contract §8 lists %d kinds, want at least 29: %q", len(kinds), kinds)
	}
	for _, k := range kinds {
		a := vector3
		a.Event = k
		if _, err := a.Canonical(); err != nil {
			t.Errorf("§8's kind %q: Canonical = %v, want bytes", k, err)
		}
	}
}

// contractEventKinds reads every kind out of contract §8's table: the
// backquoted names in each row's first cell ("`team.created` · `team.updated`").
func contractEventKinds(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(hubContract)))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	start := strings.Index(text, "\n## 8. Events\n")
	if start < 0 {
		t.Fatal("the contract has no §8")
	}
	chapter := text[start+1:]
	if end := strings.Index(chapter, "\n## "); end >= 0 {
		chapter = chapter[:end]
	}
	head := strings.Index(chapter, "\n| Kind |")
	if head < 0 {
		t.Fatal("contract §8 has no Kind table")
	}
	name := regexp.MustCompile("`([^`]+)`")
	var kinds []string
	for _, line := range strings.Split(chapter[head+1:], "\n") {
		if !strings.HasPrefix(line, "|") {
			break // the table ends at its first line that is not a row
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 || strings.HasPrefix(cells[1], "---") || strings.TrimSpace(cells[1]) == "Kind" {
			continue
		}
		for _, m := range name.FindAllStringSubmatch(cells[1], -1) {
			kinds = append(kinds, m[1])
		}
	}
	return kinds
}
