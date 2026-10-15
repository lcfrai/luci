package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var discoveryTextGate = make(chan struct{}, 1)

// A linear, bounded native text read needs no parser, decompressor, script or
// model. PDF/raster OCR does not enter this path until a contained runtime exists.
func extractDiscoveryText(ctx context.Context, original DiscoveryDocumentOriginal, data []byte) DiscoveryDocumentExtraction {
	out := DiscoveryDocumentExtraction{OriginalDigest: original.Digest, Extractor: "utf8", ExtractorVersion: "luci-text-v1", Status: "ready", Pages: []DiscoveryDocumentPage{}, Issues: []string{}}
	fail := func(status, message string) DiscoveryDocumentExtraction {
		out.Status = status
		out.Pages = []DiscoveryDocumentPage{}
		out.Issues = []string{message}
		return out
	}
	if original.MimeType != "text/plain" && original.MimeType != "text/markdown" {
		return fail("dependency-unavailable", "PDF and image OCR require the reviewed contained extractor. No browser parser or remote fallback was started.")
	}
	if len(data) != original.Bytes || discoveryTextDigest(string(data)) != original.Digest {
		return fail("corrupt", "Original bytes do not match the saved private hash.")
	}
	if len(data) > discoveryExtractionBytes || strings.Count(string(data), "\n") >= discoverySegments {
		return fail("over-limit", "The complete text exceeds 1 MiB or 10,000 lines. Split the original; no truncated extraction was retained as ready.")
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return fail("corrupt", "Text must be valid UTF-8 without binary NUL bytes. Attach a UTF-8 copy as a new original.")
	}
	page := DiscoveryDocumentPage{Page: 1, Segments: []DiscoveryDocumentSegment{}}
	for i, text := range strings.Split(string(data), "\n") {
		if ctx.Err() != nil {
			return fail("cancelled", "The exact text extraction request was cancelled. The original remains private and retained.")
		}
		text = strings.TrimSuffix(text, "\r")
		if strings.TrimSpace(text) == "" {
			continue
		}
		if len(text) > 64<<10 {
			return fail("over-limit", "A text line exceeds 64 KiB. Reformat or split the original; no partial line was accepted.")
		}
		page.Segments = append(page.Segments, DiscoveryDocumentSegment{ID: "p1-line-" + strconv.Itoa(i+1), Kind: "text", Text: text, TextDigest: discoveryTextDigest(text), Location: DiscoveryDocumentLocation{FirstLine: i + 1, LastLine: i + 1}})
	}
	out.Pages = append(out.Pages, page)
	if len(page.Segments) == 0 {
		out.Status = "needs-review"
		out.Issues = append(out.Issues, "No meaningful text was found. Inspect the original before drawing conclusions.")
	}
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded) > discoveryExtractionBytes-(2<<10) {
		return fail("over-limit", "The complete source-linked extraction exceeds the 1 MiB receipt budget. Split the original; nothing was silently truncated.")
	}
	return out
}

func (s *Server) startDiscoveryTextExtraction(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	select {
	case discoveryTextGate <- struct{}{}:
		defer func() { <-discoveryTextGate }()
	default:
		problem(w, 429, "document_extraction_busy", "Another bounded text extraction is active. Preserve this exact request and inspect its receipt before retrying.")
		return
	}
	s.mu.Lock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	if err != nil {
		s.mu.Unlock()
		discoveryDocumentProblem(w, err)
		return
	}
	if saved.Document.Original.MimeType != "text/plain" && saved.Document.Original.MimeType != "text/markdown" {
		s.mu.Unlock()
		problem(w, 422, "extractor_unavailable", "This original is retained. PDF/image OCR needs a reviewed contained extractor; none was started.")
		return
	}
	before := saved.Document.Revision
	attempt, err := discoveryStartAttempt(&saved, req.ExpectedRevision, req.IdempotencyKey, actor, time.Now())
	if err == nil && saved.Document.Revision != before {
		err = s.saveDiscoveryDocument(saved)
	}
	s.mu.Unlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	if saved.Document.Revision == before {
		writeJSON(w, 200, map[string]any{"document": discoveryDocumentView(saved.Document, time.Now()), "attempt": attempt})
		return
	}
	// The owned receipt is durable before reading. A stopped request is never replayed by GET.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	original := saved.Document.Original
	path, err := s.discoveryDocumentPath(p.ID, saved.Document.ID+".original", false)
	var data []byte
	if err == nil && original.Bytes <= discoveryExtractionBytes {
		data, err = discoveryReadPrivate(path, discoveryExtractionBytes)
	} else if original.Bytes > discoveryExtractionBytes {
		err = errors.New("original text exceeds bound")
	}
	result := extractDiscoveryText(ctx, original, data)
	if err != nil {
		result.Status = "over-limit"
		if original.Bytes <= discoveryExtractionBytes {
			result.Status = "corrupt"
		}
		result.Pages = []DiscoveryDocumentPage{}
		result.Issues = []string{"The complete private original could not be read within the text limit. Preserve it before retrying or splitting the document."}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readDiscoveryDocument(p, saved.Document.ID)
	if err == nil {
		err = discoveryFinishAttempt(&current, saved.Document.Revision, attempt.ID, "text-result-"+reviewDigest(req.IdempotencyKey), result, actor, time.Now())
	}
	if err == nil {
		err = s.saveDiscoveryDocument(current)
	}
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	for _, finished := range current.Document.Attempts {
		if finished.ID == attempt.ID {
			attempt = finished
			break
		}
	}
	writeJSON(w, 200, map[string]any{"document": current.Document, "attempt": attempt})
}
func (s *Server) cancelDiscoveryTextExtraction(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
		Cancel           bool   `json:"cancel"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.Cancel {
		problem(w, 400, "invalid_extraction_action", "Only explicit cancellation is accepted here. Extraction output is created from verified original bytes by the service.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	before := saved.Document.Revision
	if err == nil {
		err = discoveryCancelAttempt(&saved, req.ExpectedRevision, r.PathValue("attemptId"), req.IdempotencyKey, actor, time.Now())
	}
	if err == nil && saved.Document.Revision != before {
		err = s.saveDiscoveryDocument(saved)
	}
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	writeJSON(w, 200, saved.Document)
}
