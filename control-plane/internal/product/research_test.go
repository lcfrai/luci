package product

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestResearchConnectionIsWriteOnlyAndSeparateFromApplicationEnvironment(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "research-connection")
	ctx := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}})
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	key := "generated-test-search-key-not-a-credential"
	request := httptest.NewRequestWithContext(ctx, http.MethodPut, "/research/connection", strings.NewReader(`{"expectedRevision":0,"key":"`+key+`","remove":false}`))
	request.SetPathValue("id", p.ID)
	response := httptest.NewRecorder()
	s.researchConnection(response, request)
	if response.Code != 200 || strings.Contains(response.Body.String(), key) || strings.Contains(response.Body.String(), `"key"`) {
		t.Fatal("search key was returned or update failed", response.Code)
	}
	config, err := s.applicationConfigurationSnapshot(p.ID)
	if err != nil || len(config.Values) != 0 {
		t.Fatal("research credential entered application environment", err)
	}
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error {
		return management.ErrExecutionAuthority
	})
	request = httptest.NewRequestWithContext(ctx, http.MethodGet, "/research/connection", nil)
	request.SetPathValue("id", p.ID)
	response = httptest.NewRecorder()
	s.researchConnection(response, request)
	if response.Code != 403 || strings.Contains(response.Body.String(), key) {
		t.Fatal("revoked metadata read disclosed key")
	}
}

func TestResearchPublicURLAndDNSConfinement(t *testing.T) {
	for _, value := range []string{"http://localhost/a", "http://127.0.0.1/", "http://169.254.169.254/latest", "http://100.64.0.1/", "http://10.0.0.1/", "http://[::1]/", "https://public.example:444/a", "https://user:password@public.example/a", "file:///private", "https://public.example/a#fragment"} {
		if _, err := researchURL(value); err == nil {
			t.Fatalf("unsafe URL accepted: %s", value)
		}
	}
	calls := 0
	resolve := func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	dial := func(context.Context, string, string) (net.Conn, error) { calls++; return nil, nil }
	if _, err := dialResearchAddress(context.Background(), "tcp", "public.example:443", resolve, dial); err == nil || calls != 0 {
		t.Fatal("mixed public/private DNS was dialed")
	}
	reads := 0
	resolve = func(context.Context, string, string) ([]netip.Addr, error) {
		reads++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	dial = func(_ context.Context, _ string, address string) (net.Conn, error) {
		calls++
		if address != "8.8.8.8:443" {
			t.Fatal("hostname was re-resolved instead of pinning address")
		}
		return nil, nil
	}
	if _, err := dialResearchAddress(context.Background(), "tcp", "public.example:443", resolve, dial); err != nil || reads != 1 || calls != 1 {
		t.Fatal("one public lookup/one pinned dial required", err)
	}
}
func TestResearchExtractionIsBoundedPlainAttributedText(t *testing.T) {
	title, text, omitted, err := extractResearch([]byte(`<html><title>Public &amp; useful</title><script>do not quote script</script><style>hidden</style><article><p>A real customer observation.</p><p>Second &lt;claim&gt;.</p></article></html>`), "text/html")
	if err != nil || title != "Public & useful" || strings.Contains(text, "script") || strings.Contains(text, "<article>") || !strings.Contains(text, "A real customer observation.") {
		t.Fatal(title, text, err)
	}
	_, prefix, omitted, err := extractResearch([]byte(strings.Repeat("界", 10000)), "text/plain")
	if err != nil || len(prefix) > 24<<10 || omitted == 0 || len(prefix)+omitted != 30000 {
		t.Fatal("bounded UTF8 prefix lost omission metadata", err)
	}
	if _, _, _, err = extractResearch([]byte{0xff}, "text/plain"); err == nil {
		t.Fatal("non UTF8 source accepted")
	}
}
func TestCollectedResearchQuotesRequireOriginalCaptureAndSurviveSavedExpiry(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "research-proof")
	ctx := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}})
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	ref := management.ExecutionActorRef{UserID: "alice", ProjectID: p.ID}
	c := ResearchCapture{ID: newID("research-"), RequestedURL: "https://public.example/article", URL: "https://public.example/article", Title: "Interview report", FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), BodySHA256: strings.Repeat("a", 64), Text: "Original source observation, not a model conclusion."}
	c.TextSHA256 = researchTextHash(c.Text)
	if err := s.retainResearchCapture(ctx, p, ref, c, false); err != nil {
		t.Fatal(err)
	}
	spec := strategySpecification(p.ID)
	spec.Strategy.Evidence = []SpecificationStrategyEvidence{{ID: "source-collected", Source: c.Title, URL: c.URL, ObservedAt: c.FetchedAt, Observation: "The source reports an observation.", Research: captureResearchEvidence(c, []string{"Original source observation"})}}
	s.mu.Lock()
	err := s.validateResearchEvidenceLocked(ctx, p, spec, ProjectSpecification{})
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	forged := spec
	copy := *spec.Strategy
	forged.Strategy = &copy
	copy.Evidence = append([]SpecificationStrategyEvidence{}, spec.Strategy.Evidence...)
	capture := *copy.Evidence[0].Research
	copy.Evidence[0].Research = &capture
	capture.Quotes = []string{"Invented result"}
	s.mu.Lock()
	err = s.validateResearchEvidenceLocked(ctx, p, forged, ProjectSpecification{})
	s.mu.Unlock()
	if err == nil {
		t.Fatal("invented quote accepted")
	}
	capture = *spec.Strategy.Evidence[0].Research
	capture.BodySHA256 = strings.Repeat("b", 64)
	copy.Evidence[0].Research = &capture
	if preservesCollectedResearch(forged, spec) {
		t.Fatal("model invented collector attribution")
	}
	// Once retained in the canonical specification, unchanged attribution does
	// not depend on the short-lived preview ledger or a new external fetch.
	if err := s.retainResearchCapture(ctx, p, ref, ResearchCapture{ID: newID("research-"), RequestedURL: c.URL, URL: c.URL, Title: "later", FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), BodySHA256: c.BodySHA256, Text: c.Text, TextSHA256: c.TextSHA256}, false); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	err = s.validateResearchEvidenceLocked(ctx, p, spec, spec)
	s.mu.Unlock()
	if err != nil {
		t.Fatal("unchanged saved attribution lost", err)
	}
	rendered := renderSpecificationStrategy(spec.Strategy)
	if !strings.Contains(rendered, c.BodySHA256) || !strings.Contains(rendered, "Original source observation") {
		t.Fatal("agent context dropped capture provenance")
	}
}
