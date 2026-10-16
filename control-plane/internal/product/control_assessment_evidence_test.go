package product

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestControlAssessmentEvidenceLoadsSharedHistoryOnceAndBoundsAggregate(t *testing.T) {
	id, original, text := "doc-"+strings.Repeat("a", 64), strings.Repeat("b", 64), discoveryTextDigest("verified")
	link := DiscoveryEvidenceLink{DocumentID: id, OriginalDigest: original, ExtractionRevision: 1, Page: 1, SegmentID: "fact", TextDigest: text, TargetKind: "purpose"}
	results := make([]ControlAssessmentResult, 100)
	for i := range results {
		results[i] = ControlAssessmentResult{ControlID: fmt.Sprintf("C-%d", i), EvidenceRefs: []DiscoveryEvidenceLink{link}}
	}
	links, err := controlAssessmentEvidenceLinks(results)
	if err != nil || len(links) != 100 {
		t.Fatal(len(links), err)
	}
	reads := 0
	document := DiscoveryDocument{ID: id, Original: DiscoveryDocumentOriginal{Digest: original}, Extractions: []DiscoveryDocumentExtraction{{Revision: 1, Status: "ready", Pages: []DiscoveryDocumentPage{{Page: 1, Segments: []DiscoveryDocumentSegment{{ID: "fact", TextDigest: text}}}}}}}
	if err = validateDiscoveryEvidenceLinksWith(context.Background(), links, func(got string) (DiscoveryDocument, error) {
		reads++
		if got != id {
			t.Fatal(got)
		}
		return document, nil
	}); err != nil || reads != 1 {
		t.Fatal(err, reads)
	}
	results[0].EvidenceRefs = append(results[0].EvidenceRefs, make([]DiscoveryEvidenceLink, 101)...)
	if _, err = controlAssessmentEvidenceLinks(results); err == nil {
		t.Fatal("aggregate reference cap reset per control")
	}
	results[0].EvidenceRefs = []DiscoveryEvidenceLink{link}
	for i := 0; i < 17; i++ {
		extra := link
		extra.DocumentID = fmt.Sprintf("doc-%064x", i+1)
		results[i].EvidenceRefs = []DiscoveryEvidenceLink{extra}
	}
	if _, err = controlAssessmentEvidenceLinks(results); err == nil {
		t.Fatal("distinct history work cap missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads = 0
	if err = validateDiscoveryEvidenceLinksWith(ctx, links, func(string) (DiscoveryDocument, error) { reads++; return document, nil }); err == nil || reads != 0 {
		t.Fatal("cancelled validation read history", err, reads)
	}
}
