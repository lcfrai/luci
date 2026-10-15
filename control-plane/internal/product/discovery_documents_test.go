package product

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func discoveryDocumentFixture() storedDiscoveryDocument {
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	return storedDiscoveryDocument{Version: 1, Document: DiscoveryDocument{ID: "doc-" + strings.Repeat("a", 64), ProjectID: "p-fixture", Revision: 1, Original: DiscoveryDocumentOriginal{Digest: discoveryTextDigest("Original document"), Bytes: 17, Name: "brief.txt", MimeType: "text/plain"}, CreatedAt: now.Format(time.RFC3339Nano), CreatedBy: ContextActor{ID: "person-one", DisplayName: "One"}, Attempts: []DiscoveryDocumentAttempt{}, Extractions: []DiscoveryDocumentExtraction{}}, Requests: []discoveryRequestReceipt{}}
}
func discoveryExtractionFixture() DiscoveryDocumentExtraction {
	text := "Customers need to resume saved work."
	return DiscoveryDocumentExtraction{OriginalDigest: discoveryTextDigest("Original document"), Extractor: "utf8", ExtractorVersion: "luci-text-v1", Status: "ready", Pages: []DiscoveryDocumentPage{{Page: 1, Segments: []DiscoveryDocumentSegment{{ID: "page1-line1", Kind: "text", Text: text, TextDigest: discoveryTextDigest(text), Location: DiscoveryDocumentLocation{FirstLine: 1, LastLine: 1}}}}}, Issues: []string{}}
}
func discoveryCompleteFixture(t *testing.T) storedDiscoveryDocument {
	t.Helper()
	saved := discoveryDocumentFixture()
	now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
	actor := saved.Document.CreatedBy
	attempt, err := discoveryStartAttempt(&saved, 1, "start-fixture-123456789", actor, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = discoveryFinishAttempt(&saved, 2, attempt.ID, "finish-fixture-123456789", discoveryExtractionFixture(), actor, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	return saved
}
func TestDiscoveryAttemptCASReplayCancellationAndLateOutput(t *testing.T) {
	saved := discoveryDocumentFixture()
	now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
	actor := saved.Document.CreatedBy
	attempt, err := discoveryStartAttempt(&saved, 1, "start-fixture-123456789", actor, now)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := discoveryStartAttempt(&saved, 1, "start-fixture-123456789", actor, now.Add(time.Second))
	if err != nil || repeated.ID != attempt.ID || saved.Document.Revision != 2 {
		t.Fatal("start replay allocated new work", err)
	}
	if _, err = discoveryStartAttempt(&saved, 2, "different-start-123456789", actor, now.Add(time.Second)); err == nil {
		t.Fatal("second active attempt admitted")
	}
	if err = discoveryCancelAttempt(&saved, 2, attempt.ID, "cancel-fixture-123456789", actor, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = discoveryCancelAttempt(&saved, 2, attempt.ID, "cancel-fixture-123456789", actor, now.Add(2*time.Second)); err != nil || saved.Document.Revision != 3 {
		t.Fatal("cancel replay changed state", err)
	}
	if err = discoveryFinishAttempt(&saved, 3, attempt.ID, "late-finish-123456789", discoveryExtractionFixture(), actor, now.Add(3*time.Second)); err == nil {
		t.Fatal("cancelled worker output became evidence")
	}
	next, err := discoveryStartAttempt(&saved, 3, "fresh-start-123456789", actor, now.Add(4*time.Second))
	if err != nil || next.ID == attempt.ID || len(saved.Document.Attempts) != 2 {
		t.Fatal("explicit restart lost prior receipt", err)
	}
	if err = discoveryFinishAttempt(&saved, 4, next.ID, "finish-other-owner-12345", discoveryExtractionFixture(), ContextActor{ID: "person-two", DisplayName: "Two"}, now.Add(5*time.Second)); err == nil {
		t.Fatal("different person completed someone else's attempt")
	}
}
func TestDiscoveryExpiryIsReadOnlyAndRestartExplicit(t *testing.T) {
	saved := discoveryDocumentFixture()
	now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
	actor := saved.Document.CreatedBy
	attempt, err := discoveryStartAttempt(&saved, 1, "start-fixture-123456789", actor, now)
	if err != nil {
		t.Fatal(err)
	}
	view := discoveryDocumentView(saved.Document, now.Add(121*time.Second))
	if view.Attempts[0].Status != "interrupted" || saved.Document.Attempts[0].Status != "extracting" || saved.Document.Revision != 2 {
		t.Fatal("read mutated/restarted expired work")
	}
	if discoveryFinishAttempt(&saved, 2, attempt.ID, "finish-expired-123456789", discoveryExtractionFixture(), actor, now.Add(121*time.Second)) == nil {
		t.Fatal("expired owner completed evidence")
	}
	next, err := discoveryStartAttempt(&saved, 2, "restart-expired-12345678", actor, now.Add(121*time.Second))
	if err != nil || next.ID == attempt.ID || saved.Document.Attempts[0].Status != "interrupted" {
		t.Fatal("explicit restart did not preserve expired attempt", err)
	}
}
func TestDiscoveryExtractionValidationRejectsClaimsAndBounds(t *testing.T) {
	saved := discoveryCompleteFixture(t)
	valid := saved.Document.Extractions[0]
	for name, change := range map[string]func(*DiscoveryDocumentExtraction){
		"foreign original":   func(e *DiscoveryDocumentExtraction) { e.OriginalDigest = strings.Repeat("0", 64) },
		"forged text digest": func(e *DiscoveryDocumentExtraction) { e.Pages[0].Segments[0].TextDigest = strings.Repeat("0", 64) },
		"duplicate segment": func(e *DiscoveryDocumentExtraction) {
			e.Pages[0].Segments = append(e.Pages[0].Segments, e.Pages[0].Segments[0])
		},
		"unsupported method":     func(e *DiscoveryDocumentExtraction) { e.Pages[0].Segments[0].Kind = "verified" },
		"unqualified OCR":        func(e *DiscoveryDocumentExtraction) { e.Pages[0].Segments[0].Kind = "ocr" },
		"invalid geometry":       func(e *DiscoveryDocumentExtraction) { e.Pages[0].Segments[0].Location.Width = 10001 },
		"oversized text":         func(e *DiscoveryDocumentExtraction) { e.Pages[0].Segments[0].Text = strings.Repeat("a", (64<<10)+1) },
		"empty ready":            func(e *DiscoveryDocumentExtraction) { e.Pages = []DiscoveryDocumentPage{} },
		"correction forward ref": func(e *DiscoveryDocumentExtraction) { e.CorrectsRevision = e.Revision },
	} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(valid)
			var next DiscoveryDocumentExtraction
			json.Unmarshal(raw, &next)
			next.Digest = ""
			change(&next)
			if validateDiscoveryExtraction(next, saved.Document.Original) == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
	raw, _ := json.Marshal(valid)
	if strings.Contains(string(raw), "sourceFilePath") {
		t.Fatal("private path exposed")
	}
}

func TestDiscoveryNormalizedLocationRejectsHostileIntegerOverflow(t *testing.T) {
	saved := discoveryCompleteFixture(t)
	next := saved.Document.Extractions[0]
	next.Digest = ""
	segment := &next.Pages[0].Segments[0]
	segment.Location = DiscoveryDocumentLocation{X: int(^uint(0) >> 1), Width: int(^uint(0) >> 1)}
	if validateDiscoveryExtraction(next, saved.Document.Original) == nil {
		t.Fatal("overflowing normalized horizontal geometry accepted")
	}
	segment.Location = DiscoveryDocumentLocation{Y: int(^uint(0) >> 1), Height: int(^uint(0) >> 1)}
	if validateDiscoveryExtraction(next, saved.Document.Original) == nil {
		t.Fatal("overflowing normalized vertical geometry accepted")
	}
	segment.Location = DiscoveryDocumentLocation{X: 9999, Y: 9999, Width: 1, Height: 1}
	if err := validateDiscoveryExtraction(next, saved.Document.Original); err != nil {
		t.Fatal("valid normalized boundary refused", err)
	}
}
func TestDiscoveryCorrectionReasonIncludesRetainedPrefixUTF8Bytes(t *testing.T) {
	for name, reason := range map[string]string{"exactASCII": strings.Repeat("a", discoveryCorrectionReasonBytes), "exactMultibyte": strings.Repeat("é", discoveryCorrectionReasonBytes/2), "tooManyASCII": strings.Repeat("a", discoveryCorrectionReasonBytes+1), "tooManyMultibyte": strings.Repeat("é", discoveryCorrectionReasonBytes/2+1)} {
		t.Run(name, func(t *testing.T) {
			saved := discoveryCompleteFixture(t)
			segment := saved.Document.Extractions[0].Pages[0].Segments[0]
			now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
			err := discoveryCorrectExtraction(&saved, 3, 1, "correction-boundary-123456", []discoveryCorrection{{Page: 1, SegmentID: segment.ID, ExpectedTextDigest: segment.TextDigest, Text: "Corrected", Reason: reason}}, saved.Document.CreatedBy, now.Add(3*time.Second))
			if len(reason) <= discoveryCorrectionReasonBytes {
				if err != nil {
					t.Fatal("valid complete reason refused", err)
				}
				if saved.Document.Extractions[1].Pages[0].Segments[0].Uncertainty != discoveryCorrectionPrefix+reason {
					t.Fatal("reason silently clipped")
				}
			} else if err == nil {
				t.Fatal("over-limit reason accepted")
			}
		})
	}
}
func TestDiscoveryCorrectionPreservesExactOldSourceAndReadback(t *testing.T) {
	saved := discoveryCompleteFixture(t)
	actor := ContextActor{ID: "editor-two", DisplayName: "Two"}
	now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
	originalJSON, _ := json.Marshal(saved.Document.Extractions[0])
	segment := saved.Document.Extractions[0].Pages[0].Segments[0]
	changes := []discoveryCorrection{{Page: 1, SegmentID: segment.ID, ExpectedTextDigest: segment.TextDigest, Text: "Teams need to resume shared saved work.", Reason: "Corrected the speaker's terminology"}}
	if err := discoveryCorrectExtraction(&saved, 3, 1, "correction-fixture-12345", changes, actor, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	afterJSON, _ := json.Marshal(saved.Document.Extractions[0])
	if string(originalJSON) != string(afterJSON) {
		t.Fatal("old linked source was mutated")
	}
	next := saved.Document.Extractions[1]
	if next.CorrectsRevision != 1 || next.Actor.ID != actor.ID || next.Status != "needs-review" || next.Pages[0].Segments[0].Kind != "correction" || next.Pages[0].Segments[0].Confidence != nil {
		t.Fatal("correction lost provenance", next)
	}
	if err := discoveryCorrectExtraction(&saved, 3, 1, "correction-fixture-12345", changes, actor, now.Add(5*time.Second)); err != nil || saved.Document.Revision != 4 {
		t.Fatal("correction replay changed evidence", err)
	}
	changes[0].Text = "different bytes"
	if err := discoveryCorrectExtraction(&saved, 4, 2, "correction-fixture-12345", changes, actor, now); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatal("key reused for different correction", err)
	}
	link := DiscoveryEvidenceLink{DocumentID: saved.Document.ID, OriginalDigest: saved.Document.Original.Digest, ExtractionRevision: 1, Page: 1, SegmentID: segment.ID, TextDigest: segment.TextDigest}
	found, err := discoveryFindSegment(saved.Document, link)
	if err != nil || found.Text != segment.Text {
		t.Fatal("old source no longer resolves", err)
	}
}
func TestDiscoveryCompletionReadbackBindsExactOutput(t *testing.T) {
	saved := discoveryCompleteFixture(t)
	now, _ := time.Parse(time.RFC3339Nano, saved.Document.CreatedAt)
	attempt := saved.Document.Attempts[0]
	if err := discoveryFinishAttempt(&saved, 2, attempt.ID, "finish-fixture-123456789", discoveryExtractionFixture(), saved.Document.CreatedBy, now.Add(5*time.Second)); err != nil || saved.Document.Revision != 3 {
		t.Fatal("lost finish response cannot reconcile", err)
	}
	changed := discoveryExtractionFixture()
	changed.Issues = []string{"Changed result"}
	if err := discoveryFinishAttempt(&saved, 3, attempt.ID, "finish-fixture-123456789", changed, saved.Document.CreatedBy, now); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatal("finish key accepted changed output", err)
	}
}
func TestDiscoveryDiscussionQuotesExactEvidenceWithoutAuthority(t *testing.T) {
	saved := discoveryCompleteFixture(t)
	segment := saved.Document.Extractions[0].Pages[0].Segments[0]
	link := DiscoveryEvidenceLink{DocumentID: saved.Document.ID, OriginalDigest: saved.Document.Original.Digest, ExtractionRevision: 1, Page: 1, SegmentID: segment.ID, TextDigest: segment.TextDigest}
	message, err := discoveryDiscussion(map[string]DiscoveryDocument{saved.Document.ID: saved.Document}, []DiscoveryEvidenceLink{link}, "Which outcome should be resolved first?")
	if err != nil || !strings.Contains(message, "UNTRUSTED DOCUMENT EVIDENCE") || !strings.Contains(message, segment.TextDigest) || !strings.Contains(message, segment.Text) {
		t.Fatal("source-backed discussion lost exact evidence", err)
	}
	link.TextDigest = strings.Repeat("0", 64)
	if _, err := discoveryDiscussion(map[string]DiscoveryDocument{saved.Document.ID: saved.Document}, []DiscoveryEvidenceLink{link}, ""); err == nil {
		t.Fatal("caller quote accepted without exact source")
	}
}
func TestDiscoveryPrivateArtifactScopeAndOriginalIntegrity(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "document-artifact")
	other := createJourneyProject(t, mux, "other-document-artifact")
	saved := discoveryCompleteFixture(t)
	saved.Document.ProjectID = p.ID
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved.AccountScope = scope.AccountScope
	saved.ProjectScope = scope.ProjectScope
	if err = s.writeDiscoveryOriginal(p, saved.Document.ID, saved.Document.Original, []byte("Original document")); err != nil {
		t.Fatal(err)
	}
	if err = s.saveDiscoveryDocument(saved); err != nil {
		t.Fatal(err)
	}
	actual, err := s.readDiscoveryDocument(p, saved.Document.ID)
	if err != nil || actual.Document.Original != saved.Document.Original {
		t.Fatal("private artifact unavailable", err)
	}
	if _, err = s.readDiscoveryDocument(other, saved.Document.ID); err == nil {
		t.Fatal("foreign project adopted artifact")
	}
	path, err := s.discoveryDocumentPath(p.ID, saved.Document.ID+".original", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("Changed document!"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.writeDiscoveryOriginal(p, saved.Document.ID, saved.Document.Original, []byte("Original document")); err == nil {
		t.Fatal("changed original silently adopted/replaced")
	}
}
