package product

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type ResearchCandidate struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}
type ResearchSearch struct {
	Provider  string              `json:"provider"`
	Query     string              `json:"query"`
	FetchedAt string              `json:"fetchedAt"`
	Results   []ResearchCandidate `json:"results"`
}

func (s *Server) SetResearchAuthorizer(authorize management.ResearchAuthorizer) {
	s.researchAuthorize = authorize
}
func (s *Server) researchAccess(ctx context.Context, id string, secrets bool) (management.ExecutionActorRef, error) {
	ref, err := management.CaptureExecutionControlActor(ctx, id)
	if err != nil || ref.Local || ref.UserID == "" || s.researchAuthorize == nil {
		return ref, management.ErrExecutionAuthority
	}
	return ref, s.researchAuthorize(ctx, ref, secrets)
}

// Fixed endpoint and header; key comes only from the existing private project
// research connection. It is never copied into snapshots, model prompts or errors.
func (s *Server) performResearchSearch(ctx context.Context, ref management.ExecutionActorRef, query string) (ResearchSearch, error) {
	if len(query) < 3 || len(query) > 300 || !validArtifactStoredText(query) || s.researchAuthorize(ctx, ref, true) != nil {
		return ResearchSearch{}, errResearch
	}
	p, err := s.resolveProject(ref.ProjectID)
	if err != nil {
		return ResearchSearch{}, errResearch
	}
	s.mu.RLock()
	_, config, err := s.researchConnectionLocked(p, ref)
	s.mu.RUnlock()
	key := config.Key
	if err != nil || len(key) < 10 || len(key) > 4096 || strings.ContainsAny(key, "\r\n\x00") {
		return ResearchSearch{}, errResearch
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DialContext: researchDial, DisableKeepAlives: true, DisableCompression: true, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second, MaxResponseHeaderBytes: 32 << 10, MaxConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errResearch }}
	endpoint := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) + "&count=5&safesearch=strict"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ResearchSearch{}, errResearch
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Subscription-Token", key)
	if s.researchAuthorize(ctx, ref, true) != nil {
		return ResearchSearch{}, management.ErrExecutionAuthority
	}
	response, err := client.Do(request)
	if err != nil {
		return ResearchSearch{}, errResearch
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
	var decoded struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err != nil || len(raw) > 256<<10 || response.StatusCode != 200 || json.Unmarshal(raw, &decoded) != nil || len(decoded.Web.Results) > 20 || ctx.Err() != nil {
		return ResearchSearch{}, errResearch
	}
	s.mu.RLock()
	_, current, err := s.researchConnectionLocked(p, ref)
	s.mu.RUnlock()
	if err != nil || current.Revision != config.Revision || current.Key != key || s.researchAuthorize(ctx, ref, true) != nil {
		return ResearchSearch{}, management.ErrExecutionAuthority
	}
	result := ResearchSearch{Provider: "brave", Query: query, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), Results: []ResearchCandidate{}}
	seen := map[string]bool{}
	for _, item := range decoded.Web.Results {
		u, e := researchURL(item.URL)
		if e != nil || seen[u.String()] || len(result.Results) >= 5 {
			continue
		}
		title := strings.Join(strings.Fields(item.Title), " ")
		snippet := strings.Join(strings.Fields(item.Description), " ")
		if len(title) > 500 || len(snippet) > 2000 || !validArtifactStoredText(title) || !validArtifactStoredText(snippet) {
			continue
		}
		result.Results = append(result.Results, ResearchCandidate{title, u.String(), snippet})
		seen[u.String()] = true
	}
	return result, nil
}

type researchSearchRequest struct {
	Query                  string `json:"query"`
	AcknowledgeSearchCost  bool   `json:"acknowledgeSearchCost"`
	AcknowledgePublicReads bool   `json:"acknowledgePublicReads"`
}

func (s *Server) searchResearch(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ref, err := s.researchAccess(r.Context(), p.ID, true)
	if err != nil {
		problem(w, 403, "research_access", "Current project Edit and Secrets access are required.")
		return
	}
	var req researchSearchRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.AcknowledgeSearchCost {
		problem(w, 400, "research_consent", "Explicitly review the query and configured provider charges.")
		return
	}
	result, err := s.performResearchSearch(r.Context(), ref, strings.TrimSpace(req.Query))
	if err != nil {
		problem(w, 422, "research_search_unavailable", "Search could not be confirmed. Configure the private Brave connection in Research and check current Secrets access.")
		return
	}
	journeyJSON(w, 200, result)
}

// One reviewed question authorizes at most one search and three public reads.
// The provider-ranked candidates are disclosed; failed/omitted sources remain
// explicit. No model, specification save or arbitrary recursive crawl follows.
func (s *Server) investigateResearch(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ref, err := s.researchAccess(r.Context(), p.ID, true)
	if err != nil {
		problem(w, 403, "research_access", "Current project Edit and Secrets access are required.")
		return
	}
	var req researchSearchRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.AcknowledgeSearchCost || !req.AcknowledgePublicReads {
		problem(w, 400, "research_consent", "Review the question, search charges and up to three public page reads.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	search, err := s.performResearchSearch(ctx, ref, strings.TrimSpace(req.Query))
	if err != nil {
		problem(w, 422, "research_search_unavailable", errResearch.Error())
		return
	}
	captures, unavailable := []ResearchCapture{}, []string{}
	for i, candidate := range search.Results {
		if i >= 3 {
			break
		}
		if s.researchAuthorize(ctx, ref, true) != nil {
			problem(w, 403, "research_access", "Current research access changed. Results were withheld.")
			return
		}
		capture, e := fetchPublicResearch(ctx, candidate.URL)
		if e != nil {
			unavailable = append(unavailable, candidate.URL)
			continue
		}
		if s.researchAuthorize(ctx, ref, true) != nil {
			problem(w, 403, "research_access", "Current research access changed. Results were withheld.")
			return
		}
		if e = s.retainResearchCapture(ctx, p, ref, capture, true); e != nil {
			problem(w, 422, "research_retention", "Some public reads may already be retained as previews. No specification was saved.")
			return
		}
		captures = append(captures, capture)
	}
	if ctx.Err() != nil || s.researchAuthorize(r.Context(), ref, true) != nil {
		problem(w, 422, "research_changed", "The bounded read or authority expired. No results are published; already retained previews were not rolled back.")
		return
	}
	journeyJSON(w, 200, struct {
		Search      ResearchSearch    `json:"search"`
		Captures    []ResearchCapture `json:"captures"`
		Unavailable []string          `json:"unavailable"`
		Omitted     int               `json:"omitted"`
	}{search, captures, unavailable, max(0, len(search.Results)-3)})
}
