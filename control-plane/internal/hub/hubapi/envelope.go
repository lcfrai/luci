package hubapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// Brief statuses (contract §7.5). The Hub reads exactly this one field of content.
const (
	StatusOpen     = "open"
	StatusComplete = "complete"
	StatusSkipped  = "skipped"
)

// Brief scopes.
const (
	ScopeProject = "project"
	ScopeBranch  = "branch"
)

// Content limits (contract §7.5).
const (
	MaxContentDepth  = 16
	MaxEnvelopeBytes = 64 << 10 // the canonical envelope, content included
)

var (
	ErrContentNotObject = errors.New("hubapi: content must be a JSON object")
	ErrContentStatus    = errors.New(`hubapi: content.status must be "open", "complete" or "skipped"`)
	ErrContentDepth     = errors.New("hubapi: content nests more than 16 deep")
	ErrEnvelopeTooLarge = errors.New("hubapi: the canonical revision exceeds 64 KiB")
	// ErrBadEnvelope is an envelope field outside its grammar: an id with the
	// wrong prefix, a revision or seq below 1, a scope and branchRef that
	// disagree, a time not in FormatTime's form, a data value that is not a
	// string, integer or boolean; or, reading stored bytes, a wrong v or kind,
	// a missing or extra member, or bytes that are not canonical. (Task 02
	// departure D2: the part names no error for these.)
	ErrBadEnvelope = errors.New("hubapi: not a well-formed envelope")
)

// The envelopes' kinds (contract §3.4, §3.6).
const (
	kindBrief = "brief-revision"
	kindAudit = "audit-event"
)

func bad(field string) error { return &FieldError{field, ErrBadEnvelope} }

// under puts prefix in front of err's field: "content" + "status".
func under(prefix string, err error) error {
	var fe *FieldError
	if !errors.As(err, &fe) {
		return err
	}
	if fe.Field == "" {
		return &FieldError{prefix, fe.Err}
	}
	return &FieldError{prefix + "." + fe.Field, fe.Err}
}

// ValidateContent checks a revision's content as a writer sent it and returns
// its canonical bytes and its status. It is the Hub's only reading of content.
// Depth counts the content object itself as 1.
func ValidateContent(raw json.RawMessage) (canonical []byte, status string, err error) {
	tree, err := parse(raw)
	if err != nil {
		return nil, "", err
	}
	obj, ok := tree.(object)
	if !ok {
		return nil, "", &FieldError{"", ErrContentNotObject}
	}
	if at, deep := tooDeep(obj, 1, ""); deep {
		return nil, "", &FieldError{at, ErrContentDepth}
	}
	status, _ = obj.get("status").(string)
	switch status {
	case StatusOpen, StatusComplete, StatusSkipped:
	default:
		return nil, "", &FieldError{"status", ErrContentStatus}
	}
	return encode(obj), status, nil
}

// get is the member named key, or nil.
func (o object) get(key string) any {
	for _, m := range o {
		if m.key == key {
			return m.val
		}
	}
	return nil
}

// tooDeep finds the first container past MaxContentDepth; v is at depth.
func tooDeep(v any, depth int, path string) (string, bool) {
	switch x := v.(type) {
	case object:
		if depth > MaxContentDepth {
			return path, true
		}
		for _, m := range x {
			if at, deep := tooDeep(m.val, depth+1, join(path, m.key)); deep {
				return at, true
			}
		}
	case array:
		if depth > MaxContentDepth {
			return path, true
		}
		for i, e := range x {
			if at, deep := tooDeep(e, depth+1, path+"["+strconv.Itoa(i)+"]"); deep {
				return at, true
			}
		}
	}
	return "", false
}

// BriefEnvelope is contract §3.4's envelope. Its canonical bytes are what
// brief_revisions.canonical stores and what the digest covers.
type BriefEnvelope struct {
	BriefID   string          // bf-…
	Revision  int             // ≥ 1
	ProjectID string          // pj-…
	Scope     string          // ScopeProject | ScopeBranch
	BranchRef string          // "" exactly when Scope is ScopeProject
	Content   json.RawMessage // already canonical (from ValidateContent)
}

func (e BriefEnvelope) check() error {
	switch {
	case !ValidID(PrefixBrief, e.BriefID):
		return bad("briefId")
	case e.Revision < 1:
		return bad("revision")
	case !ValidID(PrefixProject, e.ProjectID):
		return bad("projectId")
	case e.Scope != ScopeProject && e.Scope != ScopeBranch:
		return bad("scope")
	case e.Scope == ScopeProject && e.BranchRef != "":
		return bad("branchRef")
	case e.Scope == ScopeBranch && (e.BranchRef == "" || !ValidBranchRef(e.BranchRef)):
		return bad("branchRef") // ValidBranchRef takes "" as project-scoped; a branch brief has a ref
	}
	return nil
}

// Canonical encodes {"v":1,"kind":"brief-revision","briefId",…,"content"}.
// It fails ErrEnvelopeTooLarge past MaxEnvelopeBytes. Content goes through
// ValidateContent, so a content that is not yet canonical gives the same bytes
// as its canonical form, and one that breaks §7.5 gives no bytes at all.
func (e BriefEnvelope) Canonical() ([]byte, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	content, _, err := ValidateContent(e.Content)
	if err != nil {
		return nil, under("content", err)
	}
	out, err := Canonical(map[string]any{
		"v": 1, "kind": kindBrief, "briefId": e.BriefID, "revision": e.Revision,
		"projectId": e.ProjectID, "scope": e.Scope, "branchRef": e.BranchRef,
		"content": json.RawMessage(content),
	})
	if err != nil {
		return nil, err
	}
	if len(out) > MaxEnvelopeBytes {
		return nil, &FieldError{"content", ErrEnvelopeTooLarge}
	}
	return out, nil
}

// Digest is DigestOf(Canonical()).
func (e BriefEnvelope) Digest() (Digest, []byte, error) {
	c, err := e.Canonical()
	if err != nil {
		return Digest{}, nil, err
	}
	return DigestOf(c), c, nil
}

var briefKeys = []string{"v", "kind", "briefId", "revision", "projectId", "scope", "branchRef", "content"}

// ParseBriefEnvelope reads stored canonical bytes back. It is how the Hub
// serves content — by parsing C, never from a second copy (contract §3.4) —
// and how hubclient re-checks a revision it received. It accepts exactly what
// Canonical writes: the eight members, v 1, the brief kind, every field in its
// grammar, and bytes that are already canonical. Content is a fresh copy.
func ParseBriefEnvelope(canonical []byte) (BriefEnvelope, error) {
	m, err := readEnvelope(canonical, kindBrief, briefKeys)
	if err != nil {
		return BriefEnvelope{}, err
	}
	var (
		e   BriefEnvelope
		rev int64
		ok  [5]bool
	)
	e.BriefID, ok[0] = m["briefId"].(string)
	rev, ok[1] = m["revision"].(int64)
	e.ProjectID, ok[2] = m["projectId"].(string)
	e.Scope, ok[3] = m["scope"].(string)
	e.BranchRef, ok[4] = m["branchRef"].(string)
	for i, k := range []string{"briefId", "revision", "projectId", "scope", "branchRef"} {
		if !ok[i] {
			return BriefEnvelope{}, bad(k)
		}
	}
	if e.Revision = int(rev); int64(e.Revision) != rev {
		return BriefEnvelope{}, bad("revision")
	}
	e.Content = encode(m["content"]) // ValidateContent, inside Canonical, judges it
	again, err := e.Canonical()
	if err != nil {
		return BriefEnvelope{}, err
	}
	if !bytes.Equal(again, canonical) {
		return BriefEnvelope{}, bad("") // parsed, but not as Canonical writes it
	}
	return e, nil
}

// readEnvelope parses an envelope's bytes into its members by name, holding
// them to exactly keys, v 1 and kind.
func readEnvelope(canonical []byte, kind string, keys []string) (map[string]any, error) {
	tree, err := parse(json.RawMessage(canonical))
	if err != nil {
		return nil, err
	}
	obj, ok := tree.(object)
	if !ok {
		return nil, bad("")
	}
	m := make(map[string]any, len(obj))
	for _, mem := range obj {
		if !slices.Contains(keys, mem.key) {
			return nil, bad(mem.key)
		}
		m[mem.key] = mem.val
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return nil, bad(k)
		}
	}
	if v, _ := m["v"].(int64); v != 1 {
		return nil, bad("v")
	}
	if k, _ := m["kind"].(string); k != kind {
		return nil, bad("kind")
	}
	return m, nil
}

// AuditEnvelope is contract §3.6's audit event, WITH teamId (plan correction
// A10). Data values are strings, integers or booleans only (§8).
type AuditEnvelope struct {
	OrgID       string // org-…
	Seq         int64  // ≥ 1
	At          string // RFC 3339 UTC, three fractional digits
	ActorUserID string // u-…, or "" for the janitor
	Event       string // the kind, e.g. "brief.revised"
	TeamID      string // tm-…, or ""
	ProjectID   string // pj-…, or ""
	Subject     string
	Data        map[string]any // string | int64 | bool values only (int is taken as int64)
}

// eventKind is §8's spelling of an event kind: "team.created", "brief.revised".
var eventKind = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (a AuditEnvelope) check() error {
	switch {
	case !ValidID(PrefixOrg, a.OrgID):
		return bad("orgId")
	case a.Seq < 1:
		return bad("seq")
	case !hubTime(a.At):
		return bad("at")
	case a.ActorUserID != "" && !ValidID(PrefixUser, a.ActorUserID):
		return bad("actorUserId")
	case !eventKind.MatchString(a.Event):
		return bad("event")
	case a.TeamID != "" && !ValidID(PrefixTeam, a.TeamID):
		return bad("teamId")
	case a.ProjectID != "" && !ValidID(PrefixProject, a.ProjectID):
		return bad("projectId")
	}
	for k, v := range a.Data {
		switch v.(type) {
		case string, bool, int, int64:
		default:
			return bad(join("data", k))
		}
	}
	return nil
}

// Canonical encodes {"v":1,"kind":"audit-event","orgId",…,"data"}. A nil Data
// is the empty object.
func (a AuditEnvelope) Canonical() ([]byte, error) {
	if err := a.check(); err != nil {
		return nil, err
	}
	data := a.Data
	if data == nil {
		data = map[string]any{}
	}
	return Canonical(map[string]any{
		"v": 1, "kind": kindAudit, "orgId": a.OrgID, "seq": a.Seq, "at": a.At,
		"actorUserId": a.ActorUserID, "event": a.Event, "teamId": a.TeamID,
		"projectId": a.ProjectID, "subject": a.Subject, "data": data,
	})
}

var auditKeys = []string{"v", "kind", "orgId", "seq", "at", "actorUserId", "event", "teamId", "projectId", "subject", "data"}

// ParseAuditEnvelope reads audit_events.canonical back, for hub verify (task
// 27) to compare with the row's columns. It accepts exactly what Canonical
// writes; integers in Data come back as int64. (Task 02 departure D3: task 27
// needs a reader and no task wrote one.)
func ParseAuditEnvelope(canonical []byte) (AuditEnvelope, error) {
	m, err := readEnvelope(canonical, kindAudit, auditKeys)
	if err != nil {
		return AuditEnvelope{}, err
	}
	var a AuditEnvelope
	strs := []struct {
		key string
		dst *string
	}{{"orgId", &a.OrgID}, {"at", &a.At}, {"actorUserId", &a.ActorUserID}, {"event", &a.Event},
		{"teamId", &a.TeamID}, {"projectId", &a.ProjectID}, {"subject", &a.Subject}}
	for _, s := range strs {
		v, ok := m[s.key].(string)
		if !ok {
			return AuditEnvelope{}, bad(s.key)
		}
		*s.dst = v
	}
	seq, ok := m["seq"].(int64)
	if !ok {
		return AuditEnvelope{}, bad("seq")
	}
	a.Seq = seq
	data, ok := m["data"].(object)
	if !ok {
		return AuditEnvelope{}, bad("data")
	}
	a.Data = make(map[string]any, len(data))
	for _, mem := range data {
		switch mem.val.(type) {
		case string, bool, int64:
			a.Data[mem.key] = mem.val
		default:
			return AuditEnvelope{}, bad(join("data", mem.key))
		}
	}
	again, err := a.Canonical()
	if err != nil {
		return AuditEnvelope{}, err
	}
	if !bytes.Equal(again, canonical) {
		return AuditEnvelope{}, bad("")
	}
	return a, nil
}

// ChainHash is SHA-256(prevHash ‖ canonical) — audit_events.hash (contract §3.6,
// §10.2's CHECK). prevHash is 32 zero bytes for seq 1.
func ChainHash(prevHash [32]byte, canonical []byte) [32]byte {
	h := sha256.New()
	h.Write(prevHash[:])
	h.Write(canonical)
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// timeLayout is FormatTime's one form. The "Z" is a literal: the time is UTC.
const timeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime writes t in the Hub's one time format: RFC 3339, UTC, exactly three
// fractional digits ("2026-09-23T04:02:11.123Z"), truncating, never rounding.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// hubTime reports whether s is exactly what FormatTime writes.
func hubTime(s string) bool {
	t, err := time.Parse(timeLayout, s)
	return err == nil && FormatTime(t) == s
}
