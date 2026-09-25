package provenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

type Resolution string

const (
	Resolved Resolution = "resolved" // the revision exists, is readable, and its bytes match its digest
	Missing  Resolution = "missing"  // no such revision, or one the caller cannot read (Hub §7.5: indistinguishable on purpose)
	Erased   Resolution = "erased"   // an org admin erased it; key, digest, savedAt, savedBy remain (ADR-H1 §5.6)
	Invalid  Resolution = "invalid"  // not a key in the grammar
	None     Resolution = "none"     // "" or none:declined — no brief was inherited
	// Unavailable is a slot the server could not answer because the Hub did
	// not: any Hub error, signed_out, or no Hub configured (task 07 rule 5,
	// review I6). A key already cached is still answered (ADR-H1 §5.4); a
	// branch key counts as cached only with its pinned project revision
	// cached too (task 07 rule 5, re-review b05-1 R-M1). Build also answers
	// it, beside its error, for a source it refuses (ErrBadSource).
	// Missing is never used for these: it comes only from the Hub's answer.
	// Never cached by the desktop; the loader retries it (task 16).
	Unavailable Resolution = "unavailable"
)

type CriterionState string

const (
	StateOpen      CriterionState = "open"
	StateSettled   CriterionState = "settled"
	StateContested CriterionState = "contested"
)

// Criterion is the part of a criterion the key fixes: a function of the key
// alone, so it lives in the Ref and is cached for ever (review I14).
type Criterion struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Digest string `json:"digest"` // "sha256:" + hex(SHA-256(UTF-8 of Text)) — phase 3's P4-2
}

// CriterionReading is the part that MOVES: phase 3's verdict on one
// criterion of one key, which changes as evidence arrives (S4-P3.4). It is
// never inside a Ref. It is served beside heads, per key
// (RefsResponse.criterionStates, task 12), and beside each head reading, per
// target (HeadReading.criterionStates, task 12), and the desktop overwrites
// it on every answer (task 16). Review I14.
type CriterionReading struct {
	ID      string         `json:"id"`      // a Criterion.ID of the same key
	State   CriterionState `json:"state"`   // "open" throughout phase 4
	Because string         `json:"because"` // required when State != open; "" when open
}

type InheritedRef struct {
	Key      string `json:"key"`
	BriefID  string `json:"briefId"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
	// Erased: the pinned project revision was erased, so the Ref holds none
	// of its text (task 07 rule 7; re-review b06-1 R-I2).
	Erased bool `json:"erased"`
}

// Ref is one brief key, resolved. Cacheable for ever EXCEPT headRevision and
// latestCompleteRevision, which the desktop keeps per brief (contract §6.4).
// It holds no criterion state: that moves, so it is a CriterionReading
// served beside the Ref, never cached with it (review I14).
// Superseded is not a resolution: it is revision < latestCompleteRevision,
// derived by the desktop from the per-brief heads (P4-B).
// Every slice marshals as [], never null, in every Ref this package makes.
type Ref struct {
	Key                    string             `json:"key"`
	Resolution             Resolution         `json:"resolution"`
	NoneReason             string             `json:"noneReason"` // "" | "declined"
	Present                bool               `json:"present"`    // resolution is resolved or erased
	BriefID                string             `json:"briefId"`
	Revision               int                `json:"revision"`
	Digest                 string             `json:"digest"`
	Scope                  string             `json:"scope"`     // "project" | "branch" | ""
	ProjectID              string             `json:"projectId"` // the HUB project id, pj-…
	BranchRef              string             `json:"branchRef"` // refs/heads/… or ""
	Status                 string             `json:"status"`    // the revision's content status
	HeadRevision           int                `json:"headRevision"`
	LatestCompleteRevision int                `json:"latestCompleteRevision"`
	Inherits               *InheritedRef      `json:"inherits"`
	Title                  string             `json:"title"`    // discover.Title of the revision the key names: a function of the key alone (task 05, review I5), open revisions included
	Headline               string             `json:"headline"` // the purpose section's body
	Audience               string             `json:"audience"`
	Constraints            string             `json:"constraints"`
	OutOfScope             string             `json:"outOfScope"`
	Sections               []discover.Section `json:"sections"`
	Criteria               []Criterion        `json:"criteria"`
	SavedAt                string             `json:"savedAt"`
	SavedBy                string             `json:"savedBy"` // display name
	ErasedAt               string             `json:"erasedAt"`
}

// Head is the time-varying part, per brief.
type Head struct {
	HeadRevision           int  `json:"headRevision"`
	LatestCompleteRevision int  `json:"latestCompleteRevision"`
	Archived               bool `json:"archived"`
}

// EvalQuery is one brief key's criteria, and the branch they are evaluated
// at. BriefKey, ProjectID and Criteria are functions of the key. BranchRef is
// not: it is the caller's (Readings' at), because a branch is where evidence
// is, and a key names a revision, not a branch (re-review b08-2 R2-I1, R2-M1).
type EvalQuery struct {
	BriefKey  string      // brief:<id>@<rev>
	ProjectID string      // the HUB project id, pj-… (Source.ProjectID)
	BranchRef string      // refs/heads/<name>: Readings' at; never "" here
	Criteria  []Criterion // Build's, digests included (P4-2), in order
}

// Verdict is phase 3's evidence.State narrowed to what a reading shows
// (phase 3 task-15.md:87-95 at a255408).
type Verdict struct {
	CriterionID string
	State       CriterionState
	Because     []string // evidence.State.Because, verbatim; Readings joins it
}

// Evaluate is the seam phase 3's evidence.Evaluator fills (ask P4-4, S4-P3.4).
// It has the shape of (*evidence.Evaluator).States (phase 3
// task-15.md:112 at a255408): one call for every criterion of one key,
// a ctx, an error, and a []string because. The adapter
// (control-plane/cmd/api/evidence.go, task 06's "The adapter") maps the
// query to States' (repo, projectPath, subject) and converts types; it
// decides nothing else. Nil means every criterion is open - phase 4's only
// behaviour.
type Evaluate func(ctx context.Context, q EvalQuery) ([]Verdict, error)

// Source is a resolved revision as discoverhub hands it over.
type Source struct {
	Revision discover.Revision  // the revision the key names
	Project  *discover.Revision // its pinned project revision (branch keys), or nil
	// PinErased: Revision pins a project revision the Hub answered erased.
	// Project is then nil, and Build renders the branch's own answers alone
	// (task 07 rule 7; re-review b06-1 R-I2).
	PinErased bool
	Digest    string
	ProjectID string
	SavedAt   string
	SavedBy   string
	// No Names: discover.Resolve takes none, and the title comes from
	// Revision.BranchRef, the envelope's own ref (review I5, task 05).
	// No BranchRef either: Ref.BranchRef is Revision.BranchRef, so the fact
	// has one carrier and two copies cannot disagree (re-review b05-1 R-M2).
	// The branch criteria are evaluated at is not a fact of the revision at
	// all: it is Readings' at.
}

// ErrBadSource: Build was handed a source that is not the revision its key
// names, or a pin that cannot be the one the revision holds. No Hub answer
// produces one; it is a caller fault (a 500), never the person's.
var ErrBadSource = errors.New("provenance: the source is not the revision the key names")

// Build resolves the revision the key names through discover.Resolve and
// fills every field; Ref.Title is the Document's Title and Ref.BranchRef is
// src.Revision.BranchRef. An open revision (only a history row can name one)
// gets Sections from its own settled answers, no inheritance, Status "open",
// and Ref.Title = discover.Title(&src.Revision) for a branch revision or
// discover.Title(nil) for a project one - task 05's one copy of the rule,
// never a string written here (re-review b05-1 R-M2). Resolve refuses an
// incomplete revision, so Build hands it a copy marked complete with no pin;
// the stored status is what Ref.Status says, and a held draft still renders
// nothing (task 05). Ref.Inherits is the pin the content holds, open or not.
// Build takes no Evaluate: nothing in a Ref may depend on evidence
// (review I14).
// With src.PinErased, Build calls discover.Resolve on a copy of src.Revision
// whose Content.Inherits is nil (so a nil project is legal, task 05), then
// sets Ref.Inherits from the original Content.Inherits with Erased true: no
// OriginProject section, and an audience, constraint or out-of-scope the
// branch skipped stays "".
//
// The error (task 06 as built): a source the key does not name - another
// revision, a key that is not a brief key, a pin passed to a project
// revision, an erased pin beside a pin in hand, a pin that is not a brief
// key - is ErrBadSource, and a pin that is not the one the revision holds is
// discover.ErrInheritMismatch. Either way the Ref is UnavailableRef(key):
// never one key on another revision's text.
func Build(key string, src Source, head Head) (Ref, error) {
	doc, pin, err := document(key, src)
	if err != nil {
		return UnavailableRef(key), err
	}
	ref := Ref{
		Key: key, Resolution: Resolved, Present: true,
		BriefID: src.Revision.BriefID, Revision: src.Revision.Number, Digest: src.Digest,
		Scope: string(src.Revision.Scope), ProjectID: src.ProjectID, BranchRef: src.Revision.BranchRef,
		Status:       string(src.Revision.Content.Status),
		HeadRevision: head.HeadRevision, LatestCompleteRevision: head.LatestCompleteRevision,
		Inherits: pin, Title: doc.Title, Sections: doc.Sections, Criteria: criteria(doc),
		SavedAt: src.SavedAt, SavedBy: src.SavedBy,
	}
	for _, s := range doc.Sections {
		switch s.Key {
		case discover.KeyPurpose:
			ref.Headline = s.Body
		case discover.KeyAudience:
			ref.Audience = s.Body
		case discover.KeyConstraints:
			ref.Constraints = s.Body
		case discover.KeyOutOfScope:
			ref.OutOfScope = s.Body
		}
	}
	return ref, nil
}

// document is the one resolution Build and Readings share, so a reading can
// never be of a criterion the Ref does not hold.
func document(key string, src Source) (discover.Document, *InheritedRef, error) {
	r := src.Revision
	k, err := Parse(key)
	if err != nil || k.Kind != KindBrief {
		return refuse("%q is not a brief key", key)
	}
	if r.Key != key || r.BriefID != k.ID || r.Number != k.Revision {
		return refuse("%s was handed revision %q (brief %q, number %d)", key, r.Key, r.BriefID, r.Number)
	}
	var pin *InheritedRef
	if in := r.Content.Inherits; in != nil {
		pk, err := Parse(in.Key)
		if err != nil || pk.Kind != KindBrief {
			return refuse("%s pins %q, which is not a brief key", key, in.Key)
		}
		pin = &InheritedRef{Key: in.Key, BriefID: pk.ID, Revision: pk.Revision, Digest: in.Digest, Erased: src.PinErased}
	}
	switch {
	case r.Scope == discover.ScopeProject && (pin != nil || src.Project != nil):
		return refuse("%s is a project revision, which pins nothing", key)
	case src.PinErased && pin == nil:
		return refuse("%s pins nothing, so no pin of it was erased", key)
	case src.PinErased && src.Project != nil:
		return refuse("%s: its pin was passed and also said to be erased", key)
	}

	var doc discover.Document
	switch {
	case r.Content.Status != discover.StatusComplete:
		own := r
		own.Content.Status, own.Content.Inherits = discover.StatusComplete, nil
		doc, err = resolveAlone(&own)
	case src.PinErased:
		own := r
		own.Content.Inherits = nil
		doc, err = resolveAlone(&own)
	case r.Scope == discover.ScopeBranch:
		doc, err = discover.Resolve(src.Project, &r)
	default:
		doc, err = discover.Resolve(&r, nil)
	}
	if err != nil {
		return discover.Document{}, nil, fmt.Errorf("provenance: %s: %w", key, err)
	}
	return doc, pin, nil
}

// resolveAlone resolves one revision with nothing inherited, in its scope's
// place: a branch revision alone is Resolve(nil, r), which a nil pin makes
// legal (task 05).
func resolveAlone(r *discover.Revision) (discover.Document, error) {
	if r.Scope == discover.ScopeBranch {
		return discover.Resolve(nil, r)
	}
	return discover.Resolve(r, nil)
}

func refuse(format string, args ...any) (discover.Document, *InheritedRef, error) {
	return discover.Document{}, nil, fmt.Errorf("%w: "+format, append([]any{ErrBadSource}, args...)...)
}

// criteria is the done section's items with their digests, in order; [] when
// the document has no definition of done.
func criteria(doc discover.Document) []Criterion {
	out := []Criterion{}
	for _, s := range doc.Sections {
		if s.Key != discover.KeyDone {
			continue
		}
		for _, c := range s.Items {
			out = append(out, Criterion{ID: c.ID, Text: c.Text, Digest: CriterionDigest(c.Text)})
		}
	}
	return out
}

// CriterionDigest is "sha256:" + the lowercase hex SHA-256 of text's UTF-8
// bytes, verbatim: not trimmed, not case-folded, not JSON-quoted (P4-2).
func CriterionDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Readings is the moving part of one resolved key: one CriterionReading per
// Criterion of Build(key, src, …), in the same order ([] when Build would
// refuse src, or the key has no definition of done). at is the branch the
// criteria are evaluated at, named by the caller when it calls, never read
// from the key: a key names a revision, not a branch (re-review b08-2).
//   - HeadRef(t) (task 07) passes t's branch for whatever key t inherits, a
//     project key included: a branch with no complete branch brief inherits
//     the project key, and its head reading shows verdicts at its own head
//     (R2-I1). That reading is per target, and so are its states.
//   - Resolve (task 07, by key) passes "" for a project key, which it never
//     evaluates: one project key inherited by two branches has two sets of
//     verdicts, and a by-key answer holds one. For a branch key it passes the
//     branch the key's brief is bound to now, not the revision's envelope
//     BranchRef: a rebind writes no revision, so after a rename the envelope
//     names a ref that is gone, or an unrelated branch that took the name
//     (R2-M1).
//
// Its rules (S4-P3.4):
//  1. ONE eval call per key, with every criterion of the done section in
//     order, never one call per criterion; none at all when there are no
//     criteria.
//  2. Every state is open with because "" when eval is nil, when at is not
//     "refs/heads/" and a name (no branch named: eval is not called, so a
//     subject is never guessed), when eval returns an error, and when its
//     verdicts do not match the criteria one for one, by id and in order. An
//     error is not the refs call's error.
//  3. because is the verdict's Because, each element trimmed, empty ones
//     dropped, joined with "; " in phase 3's order.
//  4. A non-open verdict whose because is then "" is shown open: an
//     unexplained verdict is not shown (P12). So is a state that is not
//     open, settled or contested, on that criterion alone.
//
// Called on EVERY answer, cached revision or not, so a verdict is never older
// than the refs call that carried it.
func Readings(ctx context.Context, key string, src Source, at string, eval Evaluate) []CriterionReading {
	doc, _, err := document(key, src)
	if err != nil {
		return []CriterionReading{}
	}
	crit := criteria(doc)
	out := make([]CriterionReading, len(crit))
	for i, c := range crit {
		out[i] = CriterionReading{ID: c.ID, State: StateOpen}
	}
	if eval == nil || len(crit) == 0 || !branchRef(at) {
		return out
	}
	q := EvalQuery{BriefKey: key, ProjectID: src.ProjectID, BranchRef: at, Criteria: append([]Criterion(nil), crit...)}
	vs, err := eval(ctx, q)
	if err != nil || len(vs) != len(crit) {
		return out
	}
	for i, v := range vs {
		if v.CriterionID != crit[i].ID {
			return out
		}
	}
	for i, v := range vs {
		because := joinBecause(v.Because)
		if (v.State == StateSettled || v.State == StateContested) && because != "" {
			out[i] = CriterionReading{ID: crit[i].ID, State: v.State, Because: because}
		}
	}
	return out
}

// branchRef reports whether at names a branch: "refs/heads/" and a name.
func branchRef(at string) bool {
	name, ok := strings.CutPrefix(at, "refs/heads/")
	return ok && name != ""
}

func joinBecause(because []string) string {
	kept := make([]string, 0, len(because))
	for _, b := range because {
		if b = strings.TrimSpace(b); b != "" {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "; ")
}

// slot is a Ref with no revision behind it: the key and a resolution, and
// every slice [].
func slot(key string, r Resolution) Ref {
	return Ref{Key: key, Resolution: r, Sections: []discover.Section{}, Criteria: []Criterion{}}
}

// briefSlot is slot, with a brief key's id and revision filled from the key,
// so the desktop, which never parses a key (P5), can say which revision the
// slot is. A design key fills neither: a Ref is a brief's.
func briefSlot(key string, r Resolution) Ref {
	ref := slot(key, r)
	if k, err := Parse(key); err == nil && k.Kind == KindBrief {
		ref.BriefID, ref.Revision = k.ID, k.Revision
	}
	return ref
}

func MissingRef(key string) Ref     { return briefSlot(key, Missing) }
func UnavailableRef(key string) Ref { return briefSlot(key, Unavailable) }
func InvalidRef(key string) Ref     { return slot(key, Invalid) }

// NoneRef is the Ref of a contextRef that inherited no brief: "" (none
// existed) or Declined (the person started without it, 4-D1). Anything else
// is a caller that skipped Parse - a programming error - and panics.
func NoneRef(contextRef string) Ref {
	if contextRef == "" {
		return slot("", None)
	}
	k, err := Parse(contextRef)
	if err != nil || k.Kind != KindNone {
		panic("provenance: NoneRef(" + fmt.Sprintf("%q", contextRef) + ") is not \"\" or a none: key")
	}
	ref := slot(contextRef, None)
	ref.NoneReason = k.NoneReason
	return ref
}

// ErasedRef is a revision an org admin erased (Hub §7.12): key, digest,
// savedAt and savedBy remain, present is true, and no text does.
func ErasedRef(key string, briefID string, revision int, digest, savedAt, savedBy, erasedAt string, head Head) Ref {
	ref := slot(key, Erased)
	ref.Present = true
	ref.BriefID, ref.Revision, ref.Digest = briefID, revision, digest
	ref.SavedAt, ref.SavedBy, ref.ErasedAt = savedAt, savedBy, erasedAt
	ref.HeadRevision, ref.LatestCompleteRevision = head.HeadRevision, head.LatestCompleteRevision
	return ref
}
