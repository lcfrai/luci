package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// ── the contract's own examples (§7) ────────────────────────────────────────

// jsoncKeyPaths lists every key of a §7 jsonc example as a dotted path from the
// object's root ("pin.status", "deployments.joinedAt"). A key is a quoted string
// followed by ":"; a placeholder such as User or { … } contributes no keys.
func jsoncKeyPaths(src string) []string {
	type frame struct {
		name  string
		array bool
	}
	var stack []frame
	var paths []string
	pending := ""
	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '"':
			j := strings.IndexByte(src[i+1:], '"') // the examples hold no escaped quote
			s := src[i+1 : i+1+j]
			i += j + 1
			rest := strings.TrimLeft(src[i+1:], " \t\r\n")
			if strings.HasPrefix(rest, ":") {
				pending = s
				var names []string
				for _, f := range stack[1:] {
					if !f.array {
						names = append(names, f.name)
					}
				}
				paths = append(paths, strings.Join(append(names, s), "."))
			}
		case '{', '[':
			name := pending
			if n := len(stack); n > 0 && stack[n-1].array {
				name = stack[n-1].name
			}
			stack = append(stack, frame{name, c == '['})
			pending = ""
		case '}', ']':
			stack = stack[:len(stack)-1]
		}
	}
	sort.Strings(paths)
	return slices.Compact(paths)
}

// contractObject returns the jsonc object that follows the comment line naming
// label in §section ("// Release", "// 201 — DesignApproval", "// Team — …").
func contractObject(t *testing.T, section, label string) string {
	t.Helper()
	text := contractSection(t, section)
	at := -1
	for _, line := range strings.SplitAfter(text, "\n") {
		if c, ok := strings.CutPrefix(strings.TrimSpace(line), "// "); ok && slices.Contains(strings.Split(c, " — "), label) {
			at = strings.Index(text, line) + len(line)
			break
		}
	}
	if at < 0 {
		t.Fatalf("§%s has no example labelled %q", section, label)
	}
	return firstObject(t, section, text[at:])
}

// firstObject returns the first balanced {…} in text, braces inside strings
// ignored.
func firstObject(t *testing.T, section, text string) string {
	t.Helper()
	start := strings.IndexByte(text, '{')
	depth, inString := 0, false
	for i := max(start, 0); start >= 0 && i < len(text); i++ {
		switch text[i] {
		case '"':
			inString = !inString
		case '{':
			if !inString {
				depth++
			}
		case '}':
			if !inString {
				if depth--; depth == 0 {
					return text[start : i+1]
				}
			}
		}
	}
	t.Fatalf("§%s: no complete {…} object in %.80q", section, text)
	return ""
}

// jsonKeyPaths is jsoncKeyPaths for real JSON decoded by decodeAny. It does not
// descend into a key in skip: the members the contract writes as a placeholder
// (User, [Remote], { … }), whose own keys the contract lists elsewhere.
func jsonKeyPaths(v any, skip map[string]bool) []string {
	var paths []string
	var walk func(v any, prefix string)
	walk = func(v any, prefix string) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				p := k
				if prefix != "" {
					p = prefix + "." + k
				}
				paths = append(paths, p)
				if !skip[k] {
					walk(e, p)
				}
			}
		case []any:
			for _, e := range x {
				walk(e, prefix)
			}
		}
	}
	walk(v, "")
	sort.Strings(paths)
	return slices.Compact(paths)
}

// placeholders are the members §7 shows as a placeholder, not as keys.
var placeholders = map[string]bool{
	"user": true, "createdBy": true, "addedBy": true, "boundUser": true, "savedBy": true,
	"approvedBy": true, "reservedBy": true, "recordedBy": true, "actor": true,
	"remotes": true, "content": true, "data": true,
}

// exampleJSON fills an example's placeholders with valid values.
func exampleJSON(s string) string {
	return strings.NewReplacer(
		"USER", `{"id":"u-3f2a91c07b1e4d2a","displayName":"A Person","email":"a.person@kpmg.com.au"}`,
		"TIME", `"2026-09-23T04:02:11.123Z"`,
		"DIG", `"`+abcDigest+`"`,
	).Replace(s)
}

// contractExamples are §7's examples, typed in with every "…" filled.
var contractExamples = []struct {
	section, label string
	target         func() any
	json           string
}{
	{"7.1", "User", func() any { return new(User) }, `USER`},
	{"7.1", "Org", func() any { return new(Org) }, `{"id":"org-0c1d2e3f4a5b6c7d","displayName":"KPMG Australia","tenantId":"6a1c2e3f-0000-4000-8000-000000000001"}`},
	{"7.1", "Team", func() any { return new(Team) }, `{"id":"tm-0123456789abcdef","name":"Engagement Portal","role":"admin","createdAt":TIME,"createdBy":USER}`},
	{"7.1", "Member", func() any { return new(Member) }, `{"user":USER,"role":"member","addedAt":TIME,"addedBy":USER}`},
	{"7.1", "Invite", func() any { return new(Invite) }, `{"id":"iv-0123456789abcdef","teamId":"tm-0123456789abcdef","email":"a.person@kpmg.com.au","role":"member","state":"bound","createdAt":TIME,"createdBy":USER,"boundUser":USER}`},
	{"7.1", "Remote", func() any { return new(Remote) }, `{"repoKey":"github.com/lcfrai/panaceaapp","host":"github","addedAt":TIME}`},
	{"7.1", "Project", func() any { return new(Project) }, `{"id":"pj-0123456789abcdef","teamId":"tm-0123456789abcdef","name":"Engagement Portal","remotes":[{"repoKey":"github.com/lcfrai/panaceaapp","host":"github","addedAt":TIME}],"createdAt":TIME,"createdBy":USER}`},
	{"7.1", "BriefHead", func() any { return new(BriefHead) }, `{"id":"bf-3f2a91c07b1e4d2a","projectId":"pj-0123456789abcdef","scope":"branch","branchRef":"refs/heads/evidence-upload","headRevision":6,"headKey":"brief:bf-3f2a91c07b1e4d2a@6","headDigest":DIG,"headStatus":"complete","latestCompleteRevision":6,"archivedAt":null,"createdAt":TIME,"createdBy":USER,"updatedAt":TIME}`},
	{"7.1", "BriefRevision", func() any { return new(BriefRevision) }, `{"key":"brief:bf-3f2a91c07b1e4d2a@5","briefId":"bf-3f2a91c07b1e4d2a","revision":5,"digest":DIG,"projectId":"pj-0123456789abcdef","scope":"branch","branchRef":"refs/heads/evidence-upload","status":"complete","content":{"status":"complete","questions":[],"answers":[]},"savedAt":TIME,"savedBy":USER,"erasedAt":null}`},
	{"7.7", "DesignApproval", func() any { return new(DesignApproval) }, `{"id":"ap-0123456789abcdef","projectId":"pj-0123456789abcdef","designKey":"design:cv-7c01d2e3f4a5b6c7@11","canvasId":"cv-7c01d2e3f4a5b6c7","designRevision":11,"reviewDigest":DIG,"briefKey":"brief:bf-3f2a91c07b1e4d2a@5","briefDigest":DIG,"branchRef":"refs/heads/evidence-upload","snapshot":{"manifest":DIG,"blobs":[{"digest":DIG,"size":48213,"contentType":"text/html; charset=utf-8"}]},"approvedAt":TIME,"approvedBy":USER}`},
	{"7.8", "SessionLink", func() any { return new(SessionLink) }, `{"id":"ln-0123456789abcdef","projectId":"pj-0123456789abcdef","designKey":"design:cv-7c01d2e3f4a5b6c7@11","briefKey":"brief:bf-3f2a91c07b1e4d2a@6","sessionId":"s-3f2a91c0","branchRef":"refs/heads/evidence-upload","createdAt":TIME,"createdBy":USER}`},
	{"7.9", "Release", func() any { return new(Release) }, `{"id":"rl-0123456789abcdef","projectId":"pj-0123456789abcdef","kind":"run","state":"confirmed","panaceaProject":"engagement-portal","environment":"evidence-upload","branchRef":"refs/heads/evidence-upload","commit":"4e7a91c0000000000000000000000000000000aa","pipelineRun":"engagement-portal-build-x7k2p","sourceDeploymentId":null,"sourceReleaseId":null,"pin":{"status":"pinned","reason":"ship_head","briefKey":"brief:bf-3f2a91c07b1e4d2a@6","briefDigest":DIG,"headRevisionAtShip":6},"designKey":"design:cv-7c01d2e3f4a5b6c7@11","deployments":[{"deploymentId":"d-6f2","joinedAt":TIME}],"reservedAt":TIME,"reservedBy":USER,"expiresAt":TIME,"confirmedAt":TIME,"abandonedAt":null,"abandonReason":null,"abandonMessage":null}`},
	{"7.10", "EvidenceRecord", func() any { return new(EvidenceRecord) }, `{"id":"ev-0123456789abcdef","repo":"github.com/lcfrai/panaceaapp","projectId":"pj-0123456789abcdef","briefKey":"brief:bf-3f2a91c07b1e4d2a@5","criterionId":"done:2","criterionDigest":DIG,"commit":"77aa310000000000000000000000000000000abc","verdict":"settled","because":"Upload of a 40 MB PDF passes in test/upload.spec.ts","recordedAt":TIME,"recordedBy":USER}`},
	{"7.11", "Event", func() any { return new(Event) }, `{"seq":1043,"at":TIME,"kind":"brief.revised","teamId":"tm-0123456789abcdef","projectId":"pj-0123456789abcdef","subject":"bf-3f2a91c07b1e4d2a","actor":USER,"data":{"key":"brief:bf-3f2a91c07b1e4d2a@6","revision":6,"digest":DIG,"status":"complete"}}`},
}

// Every example decodes with no unknown field, marshals back with the same keys
// at every depth, and holds exactly the keys the contract's own text lists.
func TestFieldNamesMatchTheContractExamples(t *testing.T) {
	for _, ex := range contractExamples {
		t.Run(ex.label, func(t *testing.T) {
			in := []byte(exampleJSON(ex.json))
			dst := ex.target()
			dec := json.NewDecoder(bytes.NewReader(in))
			dec.DisallowUnknownFields()
			if err := dec.Decode(dst); err != nil {
				t.Fatalf("the §%s example does not decode into %T: %v", ex.section, dst, err)
			}
			out, err := json.Marshal(dst)
			if err != nil {
				t.Fatalf("json.Marshal(%T): %v", dst, err)
			}
			if got, want := jsonKeyPaths(decodeAny(t, out), nil), jsonKeyPaths(decodeAny(t, in), nil); !slices.Equal(got, want) {
				t.Errorf("%T round trip:\n got keys %v\nwant keys %v", dst, got, want)
			}
			if got, want := jsonKeyPaths(decodeAny(t, out), placeholders), jsoncKeyPaths(contractObject(t, ex.section, ex.label)); !slices.Equal(got, want) {
				t.Errorf("%T against §%s's %s example:\n got keys %v\nwant keys %v", dst, ex.section, ex.label, got, want)
			}
		})
	}
}

// ── responses: every field present (§7.1) ──────────────────────────────────

type jsonField struct {
	name      string
	omitempty bool
	typ       reflect.Type
}

// jsonFields lists t's JSON members as encoding/json sees them, the promoted
// fields of an untagged embedded struct included.
func jsonFields(t reflect.Type) []jsonField {
	var out []jsonField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, tagged := f.Tag.Lookup("json")
		if ft := f.Type; f.Anonymous && !tagged && (ft.Kind() == reflect.Struct || ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct) {
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			out = append(out, jsonFields(ft)...)
			continue
		}
		if !f.IsExported() || tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		out = append(out, jsonField{name, slices.Contains(strings.Split(opts, ","), "omitempty"), f.Type})
	}
	return out
}

// responseTypes is every object the Hub writes in a body.
var responseTypes = []any{
	User{}, Org{}, Team{}, Member{}, Invite{}, Remote{}, Project{}, BriefHead{}, BriefRevision{},
	RevisionSummary{}, Page[Team]{}, Page[RevisionSummary]{}, Me{}, TeamDetail{}, BriefWithRevision{},
	ForBranch{}, WriteRevisionsResponse{}, RefResult{}, HeadInfo{}, RefsResponse{}, Blob{},
	Snapshot{}, DesignApproval{}, SessionLink{}, Pin{}, DeploymentJoin{}, Release{},
	EvidenceRecord{}, Event{}, ChangesPage{}, AuditEvent{}, AuditPage{}, EraseResponse{},
	Healthz{}, Readyz{},
}

// topKeys is the sorted key set of one JSON object.
func topKeys(t *testing.T, b []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not one JSON object: %s: %v", b, err)
	}
	return slices.Sorted(maps.Keys(m))
}

func TestResponsesNeverOmitAField(t *testing.T) {
	for _, v := range responseTypes {
		typ := reflect.TypeOf(v)
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal(%s{}): %v", typ, err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("%s{} is not one JSON object: %v", typ, err)
		}
		fields := jsonFields(typ)
		want := len(fields)
		for _, f := range fields {
			if typ == reflect.TypeOf(Readyz{}) && f.name == "failing" {
				// §12.4: 200 {"status":"ready"} has no "failing". The one omitempty
				// on a response, and it must stay one.
				if !f.omitempty {
					t.Errorf("Readyz.failing has no omitempty; §12.4's ready body has no failing key")
				}
				want--
				continue
			}
			if f.omitempty {
				t.Errorf("%s.%s carries omitempty; a response field is always present (§7.1)", typ, f.name)
			}
			raw, ok := got[f.name]
			if !ok {
				t.Errorf("%s{} marshals without %q", typ, f.name)
				continue
			}
			if f.typ.Kind() == reflect.Pointer && string(raw) != "null" {
				t.Errorf("%s{}.%s = %s, want null", typ, f.name, raw)
			}
		}
		if len(got) != want {
			t.Errorf("%s{} marshals %d keys %v, want %d", typ, len(got), slices.Sorted(maps.Keys(got)), want)
		}
	}

	// The zero value carries exactly the top-level keys the contract lists.
	for _, ex := range contractExamples {
		zero := reflect.New(reflect.TypeOf(ex.target()).Elem()).Interface()
		b, err := json.Marshal(zero)
		if err != nil {
			t.Fatalf("json.Marshal(%T): %v", zero, err)
		}
		var top []string
		for _, p := range jsoncKeyPaths(contractObject(t, ex.section, ex.label)) {
			if !strings.Contains(p, ".") {
				top = append(top, p)
			}
		}
		if got := topKeys(t, b); !slices.Equal(got, top) {
			t.Errorf("zero %T:\n got keys %v\nwant §%s's %v", zero, got, ex.section, top)
		}
	}

	// §7.12: an audit event is §7.11's event plus "hash" and "prevHash".
	ev, _ := json.Marshal(Event{})
	au, _ := json.Marshal(AuditEvent{})
	want := slices.Sorted(slices.Values(append(topKeys(t, ev), "hash", "prevHash")))
	if got := topKeys(t, au); !slices.Equal(got, want) {
		t.Errorf("AuditEvent{} keys %v, want %v", got, want)
	}
}

func TestDigestFieldsAreStrings(t *testing.T) {
	b, err := json.Marshal(BriefRevision{Digest: DigestOf([]byte("abc"))})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"digest":"` + abcDigest + `"`; !strings.Contains(string(b), want) {
		t.Errorf("BriefRevision marshals %s, want it to hold %s", b, want)
	}
	d := DigestOf([]byte("abc"))
	b, _ = json.Marshal(Pin{BriefDigest: &d})
	if want := `"briefDigest":"` + abcDigest + `"`; !strings.Contains(string(b), want) {
		t.Errorf("Pin marshals %s, want it to hold %s", b, want)
	}
	b, _ = json.Marshal(BriefHead{})
	if !strings.Contains(string(b), `"headDigest":null`) {
		t.Errorf("BriefHead{} marshals %s, want \"headDigest\":null", b)
	}
	b, _ = json.Marshal(SnapshotRequest{Manifest: d, Blobs: []Digest{d}})
	if want := `{"manifest":"` + abcDigest + `","blobs":["` + abcDigest + `"]}`; string(b) != want {
		t.Errorf("SnapshotRequest marshals %s, want %s", b, want)
	}
}

// ── requests (§1, §5.3) ─────────────────────────────────────────────────────

// requestTypes is every top-level request body; RevisionWrite and
// SnapshotRequest occur only inside one.
var requestTypes = []any{
	CreateTeamRequest{}, RenameTeamRequest{}, SetMemberRequest{}, CreateInviteRequest{},
	RemoteRequest{}, CreateProjectRequest{}, RenameProjectRequest{}, CreateBriefRequest{},
	RebindRequest{}, ArchiveRequest{}, WriteRevisionsRequest{}, ApprovalRequest{},
	SessionLinkRequest{}, ReserveRequest{}, ConfirmRequest{}, AbandonRequest{},
	DeploymentJoinRequest{}, EvidenceRequest{}, EraseRequest{},
}

// decodeInto runs httpx.DecodeJSON, the Hub's decoding rules, on body; dst is a
// pointer, which encoding/json fills through the interface.
func decodeInto(dst any, body string) (*httptest.ResponseRecorder, bool) {
	rec := httptest.NewRecorder()
	ok := httpx.DecodeJSON(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), &dst)
	return rec, ok
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("not a Problem: %s", rec.Body)
	}
	return p
}

// allKeys lists every JSON key a type can carry, nested structs included.
func allKeys(t reflect.Type, prefix string, out *[]string) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for _, f := range jsonFields(t) {
		*out = append(*out, prefix+f.name)
		allKeys(f.typ, prefix+f.name+".", out)
	}
}

func TestRequestsRejectUnknownFields(t *testing.T) {
	rec, ok := decodeInto(new(CreateTeamRequest), `{"name":"x","actor":"u-0123456789abcdef"}`)
	if ok || rec.Code != http.StatusBadRequest || problemOf(t, rec).Error != CodeBadRequest {
		t.Fatalf("CreateTeamRequest with an actor: ok=%v %d %s, want 400 bad_request", ok, rec.Code, rec.Body)
	}
	for _, v := range requestTypes {
		typ := reflect.TypeOf(v)
		// No request body carries an actor, approver or recorder: the principal is
		// the only one (plan global rule 2, ADR-H1 §5.3; per route, contract §7.7 "There is
		// no approver field", §7.10 "recordedBy is the principal"), so a field that
		// could name another is refused. TestEveryBodyHasTheContractsKeys is the
		// exact net; this is the second one.
		for _, smuggled := range []string{"actor", "actorUserId", "createdBy", "approvedBy", "recordedBy", "reservedBy", "savedBy", "addedBy", "approver", "recorder", "reserver"} {
			rec, ok := decodeInto(reflect.New(typ).Interface(), `{"`+smuggled+`":"u-0123456789abcdef"}`)
			if ok || rec.Code != http.StatusBadRequest || problemOf(t, rec).Error != CodeBadRequest {
				t.Errorf("%s with %q: ok=%v %d %s, want 400 bad_request", typ.Name(), smuggled, ok, rec.Code, rec.Body)
			}
		}
		var keys []string
		allKeys(typ, "", &keys)
		for _, k := range keys {
			last := k[strings.LastIndex(k, ".")+1:]
			if strings.HasPrefix(last, "actor") || strings.HasSuffix(last, "By") || last == "userId" ||
				strings.HasSuffix(strings.ToLower(last), "approver") || strings.HasSuffix(strings.ToLower(last), "recorder") || strings.HasSuffix(strings.ToLower(last), "reserver") {
				t.Errorf("%s carries %q; no request body names a principal (plan global rule 2)", typ.Name(), k)
			}
		}
	}
	// Route 24's body is {} and nothing else.
	if rec, ok := decodeInto(new(ArchiveRequest), `{}`); !ok {
		t.Errorf("ArchiveRequest {}: %d %s, want it accepted", rec.Code, rec.Body)
	}
}

// jsonBlocks returns the ```json (not jsonc) blocks of a contract section.
func jsonBlocks(t *testing.T, section string) []string {
	t.Helper()
	var out []string
	for _, part := range strings.Split(contractSection(t, section), "```json\n")[1:] {
		out = append(out, part[:strings.Index(part, "```")])
	}
	return out
}

func TestReserveRequestOmitsWhatItDoesNotSend(t *testing.T) {
	env, src := "evidence-upload", "d-4d0"
	b, err := json.Marshal(ReserveRequest{Kind: KindRollback, PanaceaProject: "engagement-portal", Environment: &env, SourceDeploymentID: &src})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"rollback","panaceaProject":"engagement-portal","environment":"evidence-upload","sourceDeploymentId":"d-4d0"}`
	if string(b) != want {
		t.Errorf("a rollback marshals\n %s\nwant\n %s", b, want)
	}
	// §7.9's own two bodies: the run and the rollback, in that order.
	blocks := jsonBlocks(t, "7.9")
	if len(blocks) < 2 {
		t.Fatalf("§7.9 has %d json blocks, want the run and the rollback", len(blocks))
	}
	for i, kind := range []ReleaseKind{KindRun, KindRollback} {
		var r ReserveRequest
		if rec, ok := decodeInto(&r, blocks[i]); !ok || r.Kind != kind {
			t.Fatalf("§7.9's %s body: kind %q, %d %s", kind, r.Kind, rec.Code, rec.Body)
		}
		back, _ := json.Marshal(r)
		if kind == KindRollback && !slices.Equal(topKeys(t, back), topKeys(t, []byte(blocks[i]))) {
			t.Errorf("§7.9's rollback re-marshals with keys %v, want %v", topKeys(t, back), topKeys(t, []byte(blocks[i])))
		}
	}
}

// Task 00's answers to phases 4 and 7, as the S0 interface they were told.
func TestTheSiblingAsksAreInTheTypes(t *testing.T) {
	has := func(v any, want string) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%T marshals %s, want it to hold %s", v, b, want)
		}
	}
	has(BriefWithRevision{}, `"latestComplete":null`) // HA4-4
	commit := "4e7a91c0000000000000000000000000000000aa"
	has(ReserveRequest{Kind: KindRun, PanaceaProject: "engagement-portal", Commit: &commit}, `"environment":null`) // HA7-1
	has(Release{}, `"environment":null`)                                                                           // HA7-1
	b, _ := json.Marshal(DeploymentJoinRequest{DeploymentID: "d"})
	if string(b) != `{"deploymentId":"d"}` {
		t.Errorf("DeploymentJoinRequest{DeploymentID: d} marshals %s, want {\"deploymentId\":\"d\"}", b)
	}
	env := "feat-x"
	has(DeploymentJoinRequest{DeploymentID: "d", Environment: &env}, `"environment":"feat-x"`)
	if MaxRevisionBatch != 64 || MaxRevisionBatchBytes != 4_194_304 { // HA4-3
		t.Errorf("MaxRevisionBatch, MaxRevisionBatchBytes = %d, %d; want 64, 4,194,304", MaxRevisionBatch, MaxRevisionBatchBytes)
	}
	// HA4-2 and HA7-5 are query shapes; that these compile is the assertion.
	_ = BriefQuery{Scope: ScopeBranch, BranchRef: "refs/heads/x", IncludeArchived: true, Page: PageRequest{After: "c", Limit: 1}}
	_ = ReleaseQuery{Kind: KindRollback, State: StateConfirmed, Page: PageRequest{}}
	_ = ApprovalQuery{CanvasID: "cv-0000", Page: PageRequest{}}
	_ = SessionLinkQuery{DesignKey: "design:cv-0000@1", Page: PageRequest{}}
}

func TestBlobContentTypesAreTheAllowlist(t *testing.T) {
	want := []string{
		"text/html; charset=utf-8", "text/css; charset=utf-8", "text/plain; charset=utf-8",
		"application/json", "image/png", "image/jpeg", "image/webp", "image/svg+xml",
		"font/woff2", "application/octet-stream",
	}
	if !slices.Equal(BlobContentTypes, want) {
		t.Errorf("BlobContentTypes = %q\nwant %q", BlobContentTypes, want)
	}
	// And the contract's own row says the same, in the same order.
	var row string
	for _, line := range strings.Split(contractSection(t, "7.6"), "\n") {
		if strings.HasPrefix(line, "| `Content-Type` |") {
			row = line
		}
	}
	var fromContract []string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(row, -1) {
		if strings.Contains(m[1], "/") {
			fromContract = append(fromContract, m[1])
		}
	}
	if !slices.Equal(BlobContentTypes, fromContract) {
		t.Errorf("BlobContentTypes = %q\n§7.6 lists %q", BlobContentTypes, fromContract)
	}
	if MaxBlobBytes != 16_777_216 || DefaultProjectQuota != 1<<30 {
		t.Errorf("MaxBlobBytes, DefaultProjectQuota = %d, %d; want 16,777,216 and 1 GiB", MaxBlobBytes, DefaultProjectQuota)
	}
}

// ── required digests (carried: task 01 review C1) ───────────────────────────

var (
	approvalBody = `{"designKey":"design:cv-7c01d2e3f4a5b6c7@11","reviewDigest":"` + abcDigest +
		`","briefKey":"brief:bf-3f2a91c07b1e4d2a@5","briefDigest":"` + abcDigest +
		`","branchRef":"refs/heads/evidence-upload","snapshot":{"manifest":"` + abcDigest +
		`","blobs":["` + abcDigest + `","` + DigestOf([]byte("abd")).String() + `"]}}`
	evidenceBody = `{"repo":"github.com/lcfrai/panaceaapp","briefKey":"brief:bf-3f2a91c07b1e4d2a@5",` +
		`"criterionId":"done:2","criterionDigest":"` + abcDigest + `",` +
		`"commit":"77aa310000000000000000000000000000000abc","verdict":"settled","because":"passes"}`
)

// edit re-marshals a JSON object body after f changes it.
func edit(t *testing.T, body string, f func(m map[string]any)) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	b, _ := json.Marshal(m)
	return string(b)
}

// digestPaths lists the value Digests a request type holds: the required ones.
func digestPaths(t reflect.Type, prefix string, out *[]string) {
	digest := reflect.TypeOf(Digest{})
	for _, f := range jsonFields(t) {
		switch ft := f.typ; {
		case ft == digest:
			*out = append(*out, prefix+f.name)
		case ft.Kind() == reflect.Slice && ft.Elem() == digest:
			*out = append(*out, prefix+f.name+"[]")
		case ft.Kind() == reflect.Struct:
			digestPaths(ft, prefix+f.name+".", out)
		case ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct:
			// A present optional object's digests are required too (review M5).
			digestPaths(ft.Elem(), prefix+f.name+".", out)
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
			digestPaths(ft.Elem(), prefix+f.name+"[].", out)
		case ft.Kind() == reflect.Map && (ft.Elem() == digest || ft.Elem().Kind() == reflect.Struct):
			// RequireDigests does not walk a map (no request type has one), so a
			// digest in one would go unchecked: this path can have no row.
			*out = append(*out, prefix+f.name+"{} (a map RequireDigests does not walk)")
		}
	}
}

// encoding/json leaves an absent or null Digest at Digest{}, which marshals as
// sha256:000…0 and would be stored for good (evidence is append-only, §7.10).
// The decoder cannot see it; RequireDigests does, and names the field.
func TestARequiredDigestAbsentOrNullIsNamed(t *testing.T) {
	if !(Digest{}).IsZero() || DigestOf(nil).IsZero() {
		t.Fatalf("IsZero: Digest{} %v, DigestOf(nil) %v; want true, false", (Digest{}).IsZero(), DigestOf(nil).IsZero())
	}
	// Any one byte set is a digest, not an absence: an IsZero that reads a few
	// bytes would refuse 1 real digest in 2^16 as missing (review M1).
	for i := range len(Digest{}) {
		var one Digest
		one[i] = 1
		if one.IsZero() {
			t.Errorf("IsZero() of a digest with only byte %d set = true, want false", i)
		}
	}
	del := func(k string) func(map[string]any) { return func(m map[string]any) { delete(m, k) } }
	null := func(k string) func(map[string]any) { return func(m map[string]any) { m[k] = nil } }
	inSnap := func(f func(map[string]any)) func(map[string]any) {
		return func(m map[string]any) { f(m["snapshot"].(map[string]any)) }
	}
	approval := func() any { return new(ApprovalRequest) }
	evidence := func() any { return new(EvidenceRequest) }
	rows := []struct {
		dst   func() any
		body  string
		field string
	}{
		{approval, approvalBody, ""},
		{evidence, evidenceBody, ""},
		{approval, edit(t, approvalBody, del("reviewDigest")), "reviewDigest"},
		{approval, edit(t, approvalBody, null("reviewDigest")), "reviewDigest"},
		{approval, edit(t, approvalBody, del("briefDigest")), "briefDigest"},
		{approval, edit(t, approvalBody, null("briefDigest")), "briefDigest"},
		{approval, edit(t, approvalBody, inSnap(del("manifest"))), "snapshot.manifest"},
		{approval, edit(t, approvalBody, inSnap(null("manifest"))), "snapshot.manifest"},
		{approval, edit(t, approvalBody, del("snapshot")), "snapshot.manifest"},
		{approval, edit(t, approvalBody, inSnap(func(m map[string]any) { m["blobs"] = []any{abcDigest, nil} })), "snapshot.blobs[1]"},
		{approval, edit(t, approvalBody, inSnap(func(m map[string]any) { m["blobs"] = []any{nil} })), "snapshot.blobs[0]"},
		{evidence, edit(t, evidenceBody, del("criterionDigest")), "criterionDigest"},
		{evidence, edit(t, evidenceBody, null("criterionDigest")), "criterionDigest"},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		dst := r.dst()
		if rec, ok := decodeInto(dst, r.body); !ok {
			t.Fatalf("DecodeJSON refused %s: %d %s", r.body, rec.Code, rec.Body)
		}
		err := RequireDigests(dst)
		if r.field == "" {
			if err != nil {
				t.Errorf("RequireDigests(%T) on a complete body = %v, want nil", dst, err)
			}
			continue
		}
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != r.field || !errors.Is(err, ErrRequired) {
			t.Errorf("RequireDigests(%T) on %s\n = %v, want ErrRequired at %q", dst, r.body, err, r.field)
		}
		covered[reflect.TypeOf(dst).Elem().Name()+"."+regexp.MustCompile(`\[\d+\]`).ReplaceAllString(r.field, "[]")] = true
	}
	// A value passed by value is checked the same way.
	var ev EvidenceRequest
	decodeInto(&ev, edit(t, evidenceBody, del("criterionDigest")))
	if !errors.Is(RequireDigests(ev), ErrRequired) {
		t.Errorf("RequireDigests(EvidenceRequest value) = %v, want ErrRequired", RequireDigests(ev))
	}
	// No body, and a field encoding/json never fills ("-"), are not refusals
	// (mutants S2 and P24).
	if err := RequireDigests(nil); err != nil {
		t.Errorf("RequireDigests(nil) = %v, want nil", err)
	}
	if err := RequireDigests((*EvidenceRequest)(nil)); err != nil {
		t.Errorf("RequireDigests((*EvidenceRequest)(nil)) = %v, want nil", err)
	}
	type skipped struct {
		Internal Digest `json:"-"`
		Sent     Digest `json:"sent"`
	}
	if err := RequireDigests(skipped{Sent: DigestOf(nil)}); err != nil {
		t.Errorf("RequireDigests with a json:\"-\" digest = %v, want nil: the wire never carries it", err)
	}
	// It reaches what encoding/json reaches, by the name encoding/json uses
	// (review M5): a present optional object's digests are required, and an
	// embedded struct's fields are the parent's.
	type optionalSnapshot struct {
		Snapshot *SnapshotRequest `json:"snapshot"`
		Blobs    []*Digest        `json:"blobs"`
	}
	full := DigestOf([]byte("abc"))
	for _, r := range []struct {
		req   any
		field string
	}{
		{&optionalSnapshot{}, ""},
		{&optionalSnapshot{Blobs: []*Digest{nil, {}}}, ""}, // a *Digest is optional, even zero
		{&optionalSnapshot{Snapshot: &SnapshotRequest{Blobs: []Digest{full}}}, "snapshot.manifest"},
		{&optionalSnapshot{Snapshot: &SnapshotRequest{Manifest: full, Blobs: []Digest{full, {}}}}, "snapshot.blobs[1]"},
		{&struct{ EvidenceRequest }{}, "criterionDigest"},
		{&struct {
			*SnapshotRequest
			Other Digest `json:"other"`
		}{SnapshotRequest: &SnapshotRequest{Manifest: full}, Other: full}, ""},
		{&struct{ *SnapshotRequest }{&SnapshotRequest{Blobs: []Digest{full}}}, "manifest"},
		{&struct {
			Items []struct {
				S []SnapshotRequest `json:"s"`
			} `json:"items"`
		}{Items: []struct {
			S []SnapshotRequest `json:"s"`
		}{{}, {S: []SnapshotRequest{{Manifest: full, Blobs: []Digest{full}}, {Manifest: full}}}}}, ""},
		{&struct {
			Items []struct {
				S []SnapshotRequest `json:"s"`
			} `json:"items"`
		}{Items: []struct {
			S []SnapshotRequest `json:"s"`
		}{{}, {S: []SnapshotRequest{{Manifest: full}, {Blobs: []Digest{full}}}}}}, "items[1].s[1].manifest"},
	} {
		err := RequireDigests(r.req)
		var fe *FieldError
		switch {
		case r.field == "" && err != nil:
			t.Errorf("RequireDigests(%T) = %v, want nil", r.req, err)
		case r.field != "" && (!errors.As(err, &fe) || fe.Field != r.field || !errors.Is(err, ErrRequired)):
			t.Errorf("RequireDigests(%T) = %v, want ErrRequired at %q", r.req, err, r.field)
		}
	}
	// Every required digest of every request type has a row above.
	for _, v := range requestTypes {
		var paths []string
		digestPaths(reflect.TypeOf(v), "", &paths)
		for _, p := range paths {
			if name := reflect.TypeOf(v).Name() + "." + p; !covered[name] {
				t.Errorf("%s is a required digest with no absent-field row", name)
			}
		}
	}
}

// RequireDigests runs on every decoded body (task 14's Decode), and route 26
// decodes holding the org's write lock (task 19). A batch of 64 maximum-size
// revisions holds no Digest, so the walk must cost nothing however large the
// content: walking content byte by byte cost 8,381,696 allocations and 1.75 s
// per call (task 03 review I1). Nor may a clean body pay for paths it never
// reports.
func TestRequireDigestsCostsNothingOnACleanBody(t *testing.T) {
	content := json.RawMessage(`{"status":"open","pad":"` + strings.Repeat("x", 64<<10-32) + `"}`)
	batch := WriteRevisionsRequest{Revisions: make([]RevisionWrite, MaxRevisionBatch)}
	for i := range batch.Revisions {
		batch.Revisions[i] = RevisionWrite{BriefID: "bf-3f2a91c07b1e4d2a", BaseRevision: i, Content: content}
	}
	blobs := make([]Digest, 500)
	for i := range blobs {
		blobs[i] = DigestOf([]byte{byte(i), byte(i >> 8)})
	}
	approval := ApprovalRequest{ReviewDigest: blobs[0], BriefDigest: blobs[1], Snapshot: SnapshotRequest{Manifest: blobs[0], Blobs: blobs}}
	for _, r := range []struct {
		name string
		req  any
	}{{"a batch of 64 × 64 KiB revisions", &batch}, {"an approval of 500 blobs", &approval}} {
		var err error
		if n := testing.AllocsPerRun(1, func() { err = RequireDigests(r.req) }); n > 0 || err != nil {
			t.Errorf("RequireDigests(%s) = %v with %.0f allocations, want nil with none", r.name, err, n)
		}
	}
}

// A digest in the wrong form is 400 bad_request naming its field (§4.2; §7.7
// check 1), never invalid_json: the body is one well-formed JSON object.
func TestAMalformedDigestIsBadRequestNamingItsField(t *testing.T) {
	hex64 := abcDigest[len("sha256:"):]
	set := func(v any, path ...string) func(map[string]any) {
		return func(m map[string]any) {
			for _, k := range path[:len(path)-1] {
				m = m[k].(map[string]any)
			}
			m[path[len(path)-1]] = v
		}
	}
	rows := []struct {
		dst         any
		body, field string
	}{
		{new(EvidenceRequest), edit(t, evidenceBody, set("sha256:zz", "criterionDigest")), "criterionDigest"},
		{new(ApprovalRequest), edit(t, approvalBody, set("sha256:"+strings.ToUpper(hex64), "reviewDigest")), "reviewDigest"},
		{new(ApprovalRequest), edit(t, approvalBody, set(hex64, "briefDigest")), "briefDigest"},
		{new(ApprovalRequest), edit(t, approvalBody, set("sha256:"+hex64[:63], "snapshot", "manifest")), "snapshot.manifest"},
		{new(ApprovalRequest), edit(t, approvalBody, set([]any{abcDigest, "nope"}, "snapshot", "blobs")), "snapshot.blobs"},
	}
	for _, r := range rows {
		rec, ok := decodeInto(r.dst, r.body)
		if ok {
			t.Errorf("DecodeJSON accepted %s", r.body)
			continue
		}
		if p := problemOf(t, rec); rec.Code != http.StatusBadRequest || p.Error != CodeBadRequest || !strings.Contains(p.Message, `"`+r.field+`"`) {
			t.Errorf("%T with a malformed %s: %d %+v, want 400 bad_request naming %q", r.dst, r.field, rec.Code, p, r.field)
		}
	}
	// encoding/json's own error carries the path, for the Hub's Decode to put in
	// details.field (task 14).
	var w struct {
		S SnapshotRequest `json:"snapshot"`
	}
	err := json.Unmarshal([]byte(`{"snapshot":{"manifest":"sha256:zz"}}`), &w)
	var te *json.UnmarshalTypeError
	if !errors.As(err, &te) || te.Field != "snapshot.manifest" {
		t.Errorf("json.Unmarshal of a malformed manifest = %#v, want an UnmarshalTypeError at snapshot.manifest", err)
	}
}

// ── every body, request and response, against the contract text (review I2) ──

// fencedObject returns the first {…} of the n-th ```<fence> block of a contract
// section, fence "json" or "jsonc", counted from 0.
func fencedObject(t *testing.T, section, fence string, n int) string {
	t.Helper()
	parts := strings.Split(contractSection(t, section), "```"+fence+"\n")[1:]
	if n >= len(parts) {
		t.Fatalf("§%s has %d ```%s blocks, want at least %d", section, len(parts), fence, n+1)
	}
	return firstObject(t, section, parts[n][:strings.Index(parts[n], "```")])
}

// inlineObject returns the first {…} after each anchor in turn: a body §7
// writes inline, as in **`POST /v1/teams`** `{"name":"Engagement Portal"}`.
func inlineObject(t *testing.T, section string, anchors ...string) string {
	t.Helper()
	text := contractSection(t, section)
	for _, a := range anchors {
		i := strings.Index(text, a)
		if i < 0 {
			t.Fatalf("§%s has no %q", section, a)
		}
		text = text[i+len(a):]
	}
	return firstObject(t, section, text)
}

// namedObjects are the objects §7 writes by name inside a body ("user": User,
// "teams": [Team]). Each has its own example (contractExamples), so a body's
// key set stops at them, as the contract's text does.
var namedObjects = map[reflect.Type]bool{
	reflect.TypeFor[User](): true, reflect.TypeFor[Org](): true, reflect.TypeFor[Team](): true,
	reflect.TypeFor[Member](): true, reflect.TypeFor[BriefHead](): true, reflect.TypeFor[BriefRevision](): true,
	reflect.TypeFor[DesignApproval](): true, reflect.TypeFor[Event](): true,
}

// typeKeyPaths is jsoncKeyPaths for a Go type: every key path encoding/json
// writes or reads for it, through pointers, slices and structs, a map's keys as
// "*", stopping at a named object.
func typeKeyPaths(t reflect.Type) []string {
	var paths []string
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for _, f := range jsonFields(t) {
			ft, p := f.typ, prefix+f.name
			paths = append(paths, p)
			for k := ft.Kind(); k == reflect.Pointer || k == reflect.Slice || k == reflect.Array || k == reflect.Map; k = ft.Kind() {
				if k == reflect.Map {
					p += ".*"
					paths = append(paths, p)
				}
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && !namedObjects[ft] {
				walk(ft, p+".")
			}
		}
	}
	walk(t, "")
	sort.Strings(paths)
	return slices.Compact(paths)
}

// pendingAmendment is a key task 00 added to a type (a sibling's ask, answered)
// that the contract text gains only in task 44a. Each entry must still be in
// the type and still be absent from the text: once task 44a writes it in, the
// entry is stale and this test says to delete it.
type pendingAmendment struct{ path, ask string }

// Every request body and every response body has exactly the keys the contract
// shows for it, at every depth. Equality, not inclusion: a renamed tag and an
// added field both fail, so no client-controlled input (an approver, a
// client-chosen revision number) can arrive under a name the contract never
// granted (task 03 review I2). The §7.1–§7.11 objects are pinned by
// TestFieldNamesMatchTheContractExamples; this covers every other body.
func TestEveryBodyHasTheContractsKeys(t *testing.T) {
	joinRoute := "**`POST /v1/releases/{releaseId}/deployments`**"
	confirm := "**`POST /v1/releases/{releaseId}/confirm`**"
	rows := []struct {
		typ     any
		objects []string          // the contract's bodies for the type; its keys are their union
		cut     []string          // a key whose members are phase data, not fields ("content")
		rename  map[string]string // an example's map key → "*"
		pending []pendingAmendment
	}{
		// Requests (§7.3–§7.12).
		{typ: CreateTeamRequest{}, objects: []string{inlineObject(t, "7.3", "**`POST /v1/teams`**")}},
		{typ: RenameTeamRequest{}, objects: []string{inlineObject(t, "7.3", "**`PATCH /v1/teams/{teamId}`**")}},
		{typ: SetMemberRequest{}, objects: []string{inlineObject(t, "7.3", "**`PUT /v1/teams/{teamId}/members/{userId}`**")}},
		{typ: CreateInviteRequest{}, objects: []string{inlineObject(t, "7.3", "**`POST /v1/teams/{teamId}/invites`**")}},
		{typ: CreateProjectRequest{}, objects: []string{fencedObject(t, "7.4", "json", 0)}},
		{typ: RenameProjectRequest{}, objects: []string{inlineObject(t, "7.4", "**`PATCH /v1/projects/{projectId}`**")}},
		{typ: RemoteRequest{}, objects: []string{inlineObject(t, "7.4", "**`POST /v1/projects/{projectId}/remotes`**")}},
		{typ: CreateBriefRequest{}, objects: []string{fencedObject(t, "7.5", "json", 0)}},
		{typ: RebindRequest{}, objects: []string{inlineObject(t, "7.5", "**`PATCH /v1/briefs/{briefId}`**")}},
		{typ: ArchiveRequest{}, objects: []string{inlineObject(t, "7.5", "**`POST /v1/briefs/{briefId}/archive`**")}},
		{typ: WriteRevisionsRequest{}, objects: []string{fencedObject(t, "7.5", "json", 1)}, cut: []string{"revisions.content"}},
		{typ: ApprovalRequest{}, objects: []string{fencedObject(t, "7.7", "json", 0)}},
		{typ: SessionLinkRequest{}, objects: []string{fencedObject(t, "7.8", "json", 0)}},
		{typ: ReserveRequest{}, objects: []string{fencedObject(t, "7.9", "json", 0), fencedObject(t, "7.9", "json", 1)}},
		{typ: ConfirmRequest{}, objects: []string{inlineObject(t, "7.9", confirm, "Run: "), inlineObject(t, "7.9", confirm, "Rollback: `{}`, or ")}},
		{typ: AbandonRequest{}, objects: []string{inlineObject(t, "7.9", "**`POST /v1/releases/{releaseId}/abandon`**")}},
		{typ: DeploymentJoinRequest{}, objects: []string{inlineObject(t, "7.9", joinRoute)},
			pending: []pendingAmendment{{"environment", "HA7-1"}}},
		{typ: EvidenceRequest{}, objects: []string{fencedObject(t, "7.10", "json", 0)}},
		{typ: EraseRequest{}, objects: []string{inlineObject(t, "7.12", "**`POST /v1/briefs/{briefId}/erase`**")}},
		// Responses (§7.2–§7.12, §12.4).
		{typ: Me{}, objects: []string{fencedObject(t, "7.2", "jsonc", 0)}},
		{typ: Page[Team]{}, objects: []string{inlineObject(t, "7.3", "**`GET /v1/teams`** → `200 ")}},
		{typ: TeamDetail{}, objects: []string{inlineObject(t, "7.3", "**`GET /v1/teams/{teamId}`** → `200 ")}},
		{typ: ForBranch{}, objects: []string{fencedObject(t, "7.5", "jsonc", 0)},
			pending: []pendingAmendment{{"branchBrief.latestComplete", "HA4-4"}, {"projectBrief.latestComplete", "HA4-4"}}},
		{typ: BriefWithRevision{}, objects: []string{inlineObject(t, "7.5", "**`GET /v1/briefs/{briefId}`** → `200 ")},
			pending: []pendingAmendment{{"latestComplete", "HA4-4"}}},
		{typ: Page[RevisionSummary]{}, objects: []string{fencedObject(t, "7.5", "jsonc", 1)}},
		{typ: WriteRevisionsResponse{}, objects: []string{inlineObject(t, "7.5", "- `201 ")}},
		{typ: RefsResponse{}, objects: []string{fencedObject(t, "7.5", "jsonc", 2)}, rename: map[string]string{"heads.bf-…": "heads.*"}},
		{typ: Blob{}, objects: []string{fencedObject(t, "7.6", "jsonc", 0)}},
		{typ: DeploymentJoin{}, objects: []string{inlineObject(t, "7.9", joinRoute, "→ `201")}},
		{typ: ChangesPage{}, objects: []string{fencedObject(t, "7.11", "jsonc", 0)}},
		{typ: EraseResponse{}, objects: []string{inlineObject(t, "7.12", "→ `200 ")}},
		{typ: Healthz{}, objects: []string{inlineObject(t, "12.4", "`GET /healthz` → `200 ")}},
		{typ: Readyz{}, objects: []string{inlineObject(t, "12.4", "`GET /readyz` → `200 "), inlineObject(t, "12.4", "else `503 ")}},
	}
	seen := map[reflect.Type]bool{}
	for _, r := range rows {
		typ := reflect.TypeOf(r.typ)
		seen[typ] = true
		var want []string
		for _, o := range r.objects {
			for _, p := range jsoncKeyPaths(o) {
				if slices.ContainsFunc(r.cut, func(c string) bool { return strings.HasPrefix(p, c+".") }) {
					continue
				}
				for from, to := range r.rename {
					if p == from || strings.HasPrefix(p, from+".") {
						p = to + p[len(from):]
					}
				}
				want = append(want, p)
			}
		}
		got := typeKeyPaths(typ)
		for _, a := range r.pending {
			switch {
			case slices.Contains(want, a.path):
				t.Errorf("%s: the contract now shows %q, so task 44a has written %s in: delete its pendingAmendment", typ, a.path, a.ask)
			case !slices.Contains(got, a.path):
				t.Errorf("%s: pendingAmendment %q (%s) is not a key of the type", typ, a.path, a.ask)
			}
			want = append(want, a.path)
		}
		sort.Strings(want)
		if want = slices.Compact(want); !slices.Equal(got, want) {
			t.Errorf("%s against the contract:\n got keys %v\nwant keys %v", typ, got, want)
		}
	}
	// Every request type has a row: a new body cannot join the package unpinned.
	for _, v := range requestTypes {
		if !seen[reflect.TypeOf(v)] {
			t.Errorf("%T has no row: pin its key set to its body in the contract", v)
		}
	}
	// §7.12: the audit page is the change feed's shape (its events are Event plus
	// hash and prevHash: TestResponsesNeverOmitAField).
	top := func(paths []string) []string {
		return slices.DeleteFunc(paths, func(p string) bool { return strings.Contains(p, ".") })
	}
	if got, want := top(typeKeyPaths(reflect.TypeFor[AuditPage]())), top(typeKeyPaths(reflect.TypeFor[ChangesPage]())); !slices.Equal(got, want) {
		t.Errorf("AuditPage keys %v, want ChangesPage's %v", got, want)
	}
}
