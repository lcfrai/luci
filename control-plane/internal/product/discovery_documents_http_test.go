package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func discoveryUploadRequest(t *testing.T, handler http.Handler, path, name, key string, data []byte, expected int) DiscoveryDocument {
	t.Helper()
	request := httptest.NewRequest("POST", path, bytes.NewReader(data))
	request.Header.Set("X-Luci-Document-Name", url.QueryEscape(name))
	request.Header.Set("Idempotency-Key", key)
	request.Header.Set("Content-Type", "application/octet-stream")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expected {
		t.Fatalf("upload status %d want %d: %s", response.Code, expected, response.Body.String())
	}
	var document DiscoveryDocument
	if expected == 200 || expected == 201 {
		if json.Unmarshal(response.Body.Bytes(), &document) != nil {
			t.Fatal("invalid upload receipt")
		}
	}
	return document
}
func TestDiscoveryUploadRetainsPrivateHashAndDuplicateReceipt(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "document-intake")

	handler := contextHistoryActorHandler(mux, management.User{ID: "person-one", Username: "One", Role: "member"})
	base := "/v1/product/projects/" + p.ID + "/discovery/documents"
	raw := []byte("Original document")
	document := discoveryUploadRequest(t, handler, base, "brief.txt", "upload-original-12345678", raw, 201)
	if document.Original.Digest != discoveryTextDigest(string(raw)) || document.Original.Bytes != len(raw) || document.CreatedBy.ID != "person-one" || len(document.Attempts) != 0 {
		t.Fatal("upload fabricated an extraction or lost source identity", document)
	}
	repeated := discoveryUploadRequest(t, handler, base, "brief.txt", "upload-original-12345678", raw, 200)
	if repeated.ID != document.ID || repeated.Revision != document.Revision {
		t.Fatal("duplicate upload created another artifact")
	}
	discoveryUploadRequest(t, handler, base, "brief.txt", "upload-original-12345678", []byte("changed bytes"), 409)
	endpoint := base + "/" + document.ID
	originalPath, err := s.discoveryDocumentPath(p.ID, document.ID+".original", false)
	if err != nil {
		t.Fatal(err)
	}
	metadataPath, err := s.discoveryDocumentPath(p.ID, document.ID+".json", false)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	w := journeyRequest(t, handler, "GET", endpoint+"/original?digest="+document.Original.Digest, nil, 200)
	if !bytes.Equal(w.Body.Bytes(), raw) || w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("original response lost its safe private boundary")
	}
	journeyRequest(t, handler, "GET", endpoint, nil, 200)
	journeyRequest(t, handler, "GET", base, nil, 200)
	after, err := os.ReadFile(metadataPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("reading intake mutated metadata", err)
	}
	if err = os.WriteFile(originalPath, []byte("changed bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, handler, "GET", endpoint+"/original?digest="+document.Original.Digest, nil, 422)
}
func TestDiscoveryUploadAndReadRequireCurrentProjectScope(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "scoped-document-intake")

	base := "/v1/product/projects/" + p.ID + "/discovery/documents"
	journeyRequest(t, mux, "GET", base, nil, 401)
	for _, access := range []management.TeamAccess{{Team: management.Team{ID: "team-example"}, ProjectID: p.ID}, {Team: management.Team{ID: "team-example"}, ProjectID: "other-project", Permissions: management.TeamPermissions{Read: true, Edit: true}}} {
		handler := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r.WithContext(management.WithTeamAccess(r.Context(), access)))
		}), management.User{ID: "person-one", Username: "One", Role: "member"})
		journeyRequest(t, handler, "GET", base, nil, 403)
		discoveryUploadRequest(t, handler, base, "brief.txt", "scoped-upload-123456789", []byte("Original document"), 403)
	}
}
func TestDiscoveryUploadDoesNotExecuteUnsupportedContent(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "unsupported-intake")

	handler := contextHistoryActorHandler(mux, management.User{ID: "person-one", Username: "One", Role: "member"})
	base := "/v1/product/projects/" + p.ID + "/discovery/documents"
	document := discoveryUploadRequest(t, handler, base, "macros.docm", "unsupported-upload-12345", []byte("PK\x03\x04an unsupported office package"), 201)
	if document.Original.MimeType != "application/octet-stream" || len(document.Extractions) != 0 || len(document.Attempts) != 0 {
		t.Fatal("unsupported bytes produced fabricated parsing work")
	}
	discoveryUploadRequest(t, handler, base, "../outside.txt", "unsafe-name-upload-1234", []byte("source"), 400)
}
