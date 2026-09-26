package discoverhub

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Hub contract §3.5, vector 1, every byte (hub-contract.md:175-185).
const (
	vector1Bytes  = `{"branchRef":"refs/heads/evidence-upload","briefId":"bf-00000000000000a1","content":{"answers":[],"status":"open"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":1,"scope":"branch","v":1}`
	vector1Digest = "sha256:a72a06a155457ac321227e660a220ab030f0367a5184f3f8ac8e8f636cd5e9fa"
)

func TestLocalDigestMatchesTheHubsVector(t *testing.T) {
	content := json.RawMessage(`{"status":"open","answers":[]}`) // unsorted, as a writer sends it
	env, err := revisionEnvelope("bf-00000000000000a1", 1, "pj-00000000000000b2", hubapi.ScopeBranch,
		"refs/heads/evidence-upload", content)
	if err != nil || string(env) != vector1Bytes {
		t.Fatalf("revisionEnvelope = %s, %v\nwant %s", env, err, vector1Bytes)
	}
	d, err := revisionDigest("bf-00000000000000a1", 1, "pj-00000000000000b2", hubapi.ScopeBranch,
		"refs/heads/evidence-upload", content)
	if err != nil || d != vector1Digest {
		t.Fatalf("revisionDigest = %q, %v; want %s", d, err, vector1Digest)
	}
}

// TestLocalEnvelopeIsTheHubsEnvelope holds the local builder to H0's
// hubapi.BriefEnvelope byte for byte, for the escapes that differ between
// encoders ("<", "&", U+2028, a control character, CJK), both scopes, and
// the limit: at 64 KiB both give the same bytes; one byte over, the Hub's
// refuses and gives none, and the local one gives all 65,537, which is what
// the size errors report.
func TestLocalEnvelopeIsTheHubsEnvelope(t *testing.T) {
	cases := []struct {
		scope, ref, content string
	}{
		{hubapi.ScopeBranch, "refs/heads/evidence-upload", `{"status":"open","t":"a<b>&c   \u0001 監査 é"}`},
		{hubapi.ScopeProject, "", `{"status":"complete","n":-0,"answers":[{"k":"v"}],"none":null}`},
	}
	for _, c := range cases {
		hub := hubapi.BriefEnvelope{BriefID: "bf-00000000000000a1", Revision: 7, ProjectID: "pj-00000000000000b2",
			Scope: c.scope, BranchRef: c.ref, Content: json.RawMessage(c.content)}
		want, err := hub.Canonical()
		if err != nil {
			t.Fatalf("hubapi Canonical(%s): %v", c.content, err)
		}
		got, err := revisionEnvelope(hub.BriefID, hub.Revision, hub.ProjectID, hub.Scope, hub.BranchRef, hub.Content)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("revisionEnvelope = %s, %v\nwant %s", got, err, want)
		}
		d, _, _ := hub.Digest()
		if ld, err := revisionDigest(hub.BriefID, hub.Revision, hub.ProjectID, hub.Scope, hub.BranchRef, hub.Content); err != nil || ld != d.String() {
			t.Fatalf("revisionDigest = %q, %v; want %s", ld, err, d)
		}
	}

	env := func(pad int) (hubapi.BriefEnvelope, json.RawMessage) {
		raw := json.RawMessage(`{"status":"open","t":"` + strings.Repeat("a", pad) + `"}`)
		return hubapi.BriefEnvelope{BriefID: "bf-00000000000000a1", Revision: 1, ProjectID: "pj-00000000000000b2",
			Scope: hubapi.ScopeProject, Content: raw}, raw
	}
	e0, _ := env(0)
	empty, err := e0.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	at, raw := env(MaxEnvelopeBytes - len(empty))
	want, err := at.Canonical()
	if err != nil || len(want) != MaxEnvelopeBytes {
		t.Fatalf("hubapi at the limit: %d bytes, %v", len(want), err)
	}
	if got, err := revisionEnvelope(at.BriefID, 1, at.ProjectID, at.Scope, "", raw); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("revisionEnvelope at the limit: %d bytes, %v", len(got), err)
	}
	over, raw := env(MaxEnvelopeBytes - len(empty) + 1)
	if _, err := over.Canonical(); !errors.Is(err, hubapi.ErrEnvelopeTooLarge) {
		t.Fatalf("hubapi one over: %v, want ErrEnvelopeTooLarge", err)
	}
	if got, err := revisionEnvelope(over.BriefID, 1, over.ProjectID, over.Scope, "", raw); err != nil || len(got) != MaxEnvelopeBytes+1 {
		t.Fatalf("revisionEnvelope one over: %d bytes, %v; want %d", len(got), err, MaxEnvelopeBytes+1)
	}
}
