package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var discoveryUploadGate = make(chan struct{}, 1)

func (s *Server) registerDiscoveryDocuments(mux Registrar) {
	const base = "/v1/product/projects/{id}/discovery/documents"
	mux.HandleFunc("GET "+base, s.listDiscoveryDocuments)
	mux.HandleFunc("POST "+base, s.uploadDiscoveryDocument)
	mux.HandleFunc("GET "+base+"/{documentId}", s.getDiscoveryDocument)
	mux.HandleFunc("GET "+base+"/{documentId}/original", s.getDiscoveryDocumentOriginal)
	mux.HandleFunc("POST "+base+"/discussion-context", s.discoveryDocumentDiscussion)
	mux.HandleFunc("POST "+base+"/request-disposition", s.discoveryDocumentRequestDisposition)
	mux.HandleFunc("POST "+base+"/{documentId}/corrections", s.correctDiscoveryDocument)
	mux.HandleFunc("POST "+base+"/{documentId}/extractions", s.startDiscoveryTextExtraction)
	mux.HandleFunc("PUT "+base+"/{documentId}/extractions/{attemptId}", s.cancelDiscoveryTextExtraction)
}

// Read-only exact request readback. Absence at the same CAS revision (and all
// absent raw uploads) remains unconfirmed, never permission for a new request key.
func (s *Server) discoveryDocumentRequestDisposition(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		Action             string                     `json:"action"`
		IdempotencyKey     string                     `json:"idempotencyKey"`
		DocumentID         string                     `json:"documentId"`
		ExpectedRevision   int                        `json:"expectedRevision"`
		Original           *DiscoveryDocumentOriginal `json:"original,omitempty"`
		AttemptID          string                     `json:"attemptId,omitempty"`
		ExtractionRevision int                        `json:"extractionRevision,omitempty"`
		Changes            []discoveryCorrection      `json:"changes,omitempty"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validRunKey(req.IdempotencyKey) || req.ExpectedRevision < 0 {
		discoveryDocumentProblem(w, errDiscoveryDocument)
		return
	}
	id := req.DocumentID
	if req.Action == "upload" {
		id = "doc-" + reviewDigest(struct{ Project, Actor, Key string }{p.ID, actor.ID, req.IdempotencyKey})
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readDiscoveryDocument(p, id)
	if errors.Is(err, os.ErrNotExist) && req.Action == "upload" {
		writeJSON(w, 200, map[string]any{"state": "unconfirmed", "document": nil})
		return
	}
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	var digest string
	switch req.Action {
	case "upload":
		if req.Original == nil || req.Original.Digest != saved.Document.Original.Digest || req.Original.Name != saved.Document.Original.Name || req.Original.Bytes != saved.Document.Original.Bytes {
			discoveryDocumentProblem(w, ErrProjectRunKeyConflict)
			return
		}
		digest = reviewDigest(saved.Document.Original)
	case "extract":
		digest = reviewDigest(struct {
			Original string
			Actor    string
		}{saved.Document.Original.Digest, actor.ID})
	case "cancel":
		digest = reviewDigest(struct {
			Attempt string
			Actor   string
		}{req.AttemptID, actor.ID})
	case "correct":
		digest = reviewDigest(struct {
			Revision int
			Changes  []discoveryCorrection
			Actor    string
		}{req.ExtractionRevision, req.Changes, actor.ID})
	default:
		discoveryDocumentProblem(w, errDiscoveryDocument)
		return
	}
	receipt, found, err := discoveryRequestReadback(saved, req.IdempotencyKey, digest, req.Action)
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	state := "unconfirmed"
	if found {
		state = "recorded"
	} else if req.Action != "upload" && saved.Document.Revision > req.ExpectedRevision {
		state = "rejected"
	}
	writeJSON(w, 200, map[string]any{"state": state, "document": discoveryDocumentView(saved.Document, time.Now()), "attemptId": receipt.AttemptID, "extractionRevision": receipt.ExtractionRevision})
}

// Extraction start/finish will share these exact read/edit boundaries. Neither
// route registration nor viewing a document starts a worker or model job.
func discoveryDocumentAccess(w http.ResponseWriter, r *http.Request, edit bool) bool {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Context().Err() != nil {
		return false
	}
	if _, ok := authenticatedContextActor(r.Context()); !ok {
		problem(w, 401, "authentication_required", "Sign in to read private project evidence.")
		return false
	}
	if access, scoped := management.TeamAccessFromContext(r.Context()); scoped && (access.ProjectID != r.PathValue("id") || !access.Permissions.Read || edit && !access.Permissions.Edit) {
		problem(w, 403, "forbidden", "Current project access is required for this document action.")
		return false
	}
	return true
}
func discoveryDocumentProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		problem(w, 404, "document_unavailable", "That private document is not retained in this project.")
	case errors.Is(err, ErrProjectRunConflict), errors.Is(err, ErrProjectRunKeyConflict):
		problem(w, 409, "revision_conflict", err.Error())
	case errors.Is(err, errDiscoveryCapacity):
		problem(w, 413, "document_history_full", err.Error())
	default:
		problem(w, 422, "document_unavailable", errDiscoveryDocument.Error())
	}
}
func (s *Server) getDiscoveryDocument(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	writeJSON(w, 200, discoveryDocumentView(saved.Document, time.Now()))
}
func (s *Server) discoveryDocumentNames(p project.Project) ([]string, int64, error) {
	root, err := s.discoveryDocumentPath(p.ID, "", false)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	dir, err := os.Open(root)
	if err != nil {
		return nil, 0, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(2049)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	if len(entries) > 2048 {
		return nil, 0, errDiscoveryCapacity
	}
	names := []string{}
	var size int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil, 0, errDiscoveryDocument
		}
		size += info.Size()
		if strings.HasSuffix(entry.Name(), ".json") && discoveryDocumentID.MatchString(strings.TrimSuffix(entry.Name(), ".json")) {
			names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	sort.Strings(names)
	return names, size, nil
}
func (s *Server) listDiscoveryDocuments(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	after := r.URL.Query().Get("after")
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 25 {
			problem(w, 400, "invalid_page", "Use a document page size from 1 to 25.")
			return
		}
		limit = parsed
	}
	if after != "" && !discoveryDocumentID.MatchString(after) {
		problem(w, 400, "invalid_cursor", "Use the exact previous document cursor.")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	names, _, err := s.discoveryDocumentNames(p)
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	out := []DiscoveryDocument{}
	next := ""
	omitted := false
	for _, name := range names {
		if name <= after {
			continue
		}
		if len(out) >= limit {
			omitted = true
			next = out[len(out)-1].ID
			break
		}
		if r.Context().Err() != nil {
			return
		}
		saved, err := s.readDiscoveryDocument(p, name)
		if err != nil {
			discoveryDocumentProblem(w, err)
			return
		}
		item := discoveryDocumentView(saved.Document, time.Now())
		item.Extractions = []DiscoveryDocumentExtraction{}
		out = append(out, item)
	}
	writeJSON(w, 200, map[string]any{"documents": out, "nextCursor": next, "omitted": omitted})
}
func discoveryOriginalType(data []byte, name string) string {
	switch {
	case len(data) >= 5 && string(data[:5]) == "%PDF-":
		return "application/pdf"
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case utf8.Valid(data) && !strings.ContainsRune(string(data), 0) && (strings.HasSuffix(strings.ToLower(name), ".txt") || strings.HasSuffix(strings.ToLower(name), ".md")):
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			return "text/markdown"
		}
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
func (s *Server) uploadDiscoveryDocument(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	select {
	case discoveryUploadGate <- struct{}{}:
		defer func() { <-discoveryUploadGate }()
	default:
		problem(w, 429, "document_upload_busy", "Another bounded document upload is active. Preserve this request key and retry after inspecting its receipt.")
		return
	}
	name, err := url.QueryUnescape(r.Header.Get("X-Luci-Document-Name"))
	key := r.Header.Get("Idempotency-Key")
	if err != nil || !discoveryPlainText(name, 255) || strings.TrimSpace(name) == "" || strings.ContainsAny(name, "/\\\r\n") || !validRunKey(key) {
		problem(w, 400, "invalid_document", "Choose a named file and retain a stable upload request key.")
		return
	}
	body := http.MaxBytesReader(w, r.Body, discoveryOriginalBytes)
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || len(data) == 0 || len(data) > discoveryOriginalBytes {
		problem(w, 413, "document_too_large", "Choose a document between 1 byte and 10 MiB. Its bytes were not accepted as complete.")
		return
	}
	if r.Context().Err() != nil {
		return
	}
	original := DiscoveryDocumentOriginal{Digest: discoveryTextDigest(string(data)), Bytes: len(data), Name: name, MimeType: discoveryOriginalType(data, name)}
	requestDigest := reviewDigest(original)
	id := "doc-" + reviewDigest(struct{ Project, Actor, Key string }{p.ID, actor.ID, key})
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Context().Err() != nil {
		return
	}
	if previous, err := s.readDiscoveryDocument(p, id); err == nil {
		if _, found, e := discoveryRequestReadback(previous, key, requestDigest, "upload"); e != nil {
			discoveryDocumentProblem(w, e)
			return
		} else if !found {
			discoveryDocumentProblem(w, ErrProjectRunKeyConflict)
			return
		}
		writeJSON(w, 200, discoveryDocumentView(previous.Document, time.Now()))
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		discoveryDocumentProblem(w, err)
		return
	}
	names, size, err := s.discoveryDocumentNames(p)
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	if len(names) >= 512 || size+int64(len(data)) > discoveryProjectBytes {
		discoveryDocumentProblem(w, errDiscoveryCapacity)
		return
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	saved := storedDiscoveryDocument{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Document: DiscoveryDocument{ID: id, ProjectID: p.ID, Revision: 1, Original: original, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), CreatedBy: actor, Attempts: []DiscoveryDocumentAttempt{}, Extractions: []DiscoveryDocumentExtraction{}}, Requests: []discoveryRequestReceipt{}}
	if err = discoveryRememberRequest(&saved, key, requestDigest, "upload", "", 0); err == nil {
		err = s.writeDiscoveryOriginal(p, id, original, data)
	}
	if err == nil {
		err = s.saveDiscoveryDocument(saved)
	}
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	writeJSON(w, 201, saved.Document)
}
func (s *Server) getDiscoveryDocumentOriginal(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	if r.URL.Query().Get("digest") != saved.Document.Original.Digest {
		problem(w, 409, "document_changed", "An exact original digest is required.")
		return
	}
	path, err := s.discoveryDocumentPath(p.ID, saved.Document.ID+".original", false)
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	data, err := discoveryReadPrivate(path, discoveryOriginalBytes)
	if err != nil || len(data) != saved.Document.Original.Bytes || discoveryTextDigest(string(data)) != saved.Document.Original.Digest {
		discoveryDocumentProblem(w, errDiscoveryDocument)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\"document-original\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (s *Server) correctDiscoveryDocument(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision   int                   `json:"expectedRevision"`
		ExtractionRevision int                   `json:"extractionRevision"`
		IdempotencyKey     string                `json:"idempotencyKey"`
		Changes            []discoveryCorrection `json:"changes"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	before := saved.Document.Revision
	if err == nil {
		err = discoveryCorrectExtraction(&saved, req.ExpectedRevision, req.ExtractionRevision, req.IdempotencyKey, req.Changes, actor, time.Now())
	}
	if err == nil && saved.Document.Revision != before {
		err = s.saveDiscoveryDocument(saved)
	}
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	writeJSON(w, 200, discoveryDocumentView(saved.Document, time.Now()))
}
func (s *Server) discoveryDocumentDiscussion(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Links    []DiscoveryEvidenceLink `json:"links"`
		Question string                  `json:"question"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if len(req.Links) < 1 || len(req.Links) > 40 {
		problem(w, 400, "invalid_selection", "Select between 1 and 40 source segments.")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	documents := map[string]DiscoveryDocument{}
	for _, link := range req.Links {
		if _, found := documents[link.DocumentID]; found {
			continue
		}
		saved, err := s.readDiscoveryDocument(p, link.DocumentID)
		if err != nil {
			discoveryDocumentProblem(w, err)
			return
		}
		documents[link.DocumentID] = saved.Document
	}
	message, err := discoveryDiscussion(documents, req.Links, req.Question)
	if err != nil {
		problem(w, 422, "evidence_selection_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"projectId": p.ID, "message": message, "links": req.Links})
}

// Available for the narrow existing specification save/application seam.
// Caller supplies only links; original bytes and quoted text are never trusted.
func (s *Server) validateDiscoveryEvidenceLinks(ctx context.Context, p project.Project, links []DiscoveryEvidenceLink) error {
	if len(links) > 200 {
		return errDiscoveryCapacity
	}
	documents := map[string]DiscoveryDocument{}
	for _, link := range links {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		document, found := documents[link.DocumentID]
		if !found {
			saved, err := s.readDiscoveryDocument(p, link.DocumentID)
			if err != nil {
				return err
			}
			document = saved.Document
			documents[link.DocumentID] = document
		}
		if _, err := discoveryFindSegment(document, link); err != nil {
			return err
		}
	}
	return nil
}

// Strict bounded JSON result ingestion is intentionally separate from raw uploads.
func decodeDiscoveryExtraction(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, discoveryExtractionBytes+(64<<10))
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		problem(w, 400, "invalid_extraction", "The complete extraction receipt is invalid or exceeded its bound.")
		return false
	}
	return true
}
