package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type researchPreviews struct {
	Version      int               `json:"version"`
	AccountScope string            `json:"accountScope"`
	ProjectScope string            `json:"projectScope"`
	ActorID      string            `json:"actorId"`
	TeamID       string            `json:"teamId"`
	Captures     []ResearchCapture `json:"captures"`
}

func researchDenied(s *Server, ctx context.Context, id string) bool {
	_, err := s.researchAccess(ctx, id, false)
	return err != nil
}
func preservesCollectedResearch(next, current ProjectSpecification) bool {
	prior := map[string]*ResearchEvidence{}
	if current.Strategy != nil {
		for _, row := range current.Strategy.Evidence {
			prior[row.ID] = row.Research
		}
	}
	if next.Strategy != nil {
		for _, row := range next.Strategy.Evidence {
			if row.Research != nil && (!reflect.DeepEqual(row.Research, prior[row.ID]) || row.URL != row.Research.URL || row.ObservedAt != row.Research.FetchedAt) {
				return false
			}
		}
	}
	return true
}
func preserveCollectedResearch(next *ProjectSpecification, current ProjectSpecification) {
	if next.Strategy == nil || next.Strategy.Evidence == nil || current.Strategy == nil {
		return
	}
	saved := map[string]*ResearchEvidence{}
	for _, row := range current.Strategy.Evidence {
		saved[row.ID] = row.Research
	}
	copy := *next.Strategy
	copy.Evidence = append([]SpecificationStrategyEvidence{}, next.Strategy.Evidence...)
	for i := range copy.Evidence {
		row := &copy.Evidence[i]
		if row.Research == nil && saved[row.ID] != nil {
			capture := *saved[row.ID]
			capture.Quotes = append([]string{}, capture.Quotes...)
			row.Research = &capture
		}
	}
	next.Strategy = &copy
}
func (s *Server) researchPreviewsLocked(p project.Project, actor, team string) (string, researchPreviews, error) {
	scope, e := s.approvalScopes(p.Path, p.ID)
	if e != nil {
		return "", researchPreviews{}, e
	}
	path, e := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "research-"+reviewDigest([]string{p.ID, actor, team}))
	if e != nil {
		return "", researchPreviews{}, e
	}
	empty := researchPreviews{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ActorID: actor, TeamID: team, Captures: []ResearchCapture{}}
	var saved researchPreviews
	e = externalReadJSON(path, 2<<20, &saved)
	if errors.Is(e, os.ErrNotExist) {
		return path, empty, nil
	}
	if e != nil || saved.Version != 1 || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.ActorID != actor || saved.TeamID != team || saved.Captures == nil || len(saved.Captures) > 16 {
		return "", empty, errResearch
	}
	seen := map[string]bool{}
	for _, c := range saved.Captures {
		if !strings.HasPrefix(c.ID, "research-") || !prototypeReleaseID.MatchString(c.ID) || seen[c.ID] || !hexDigest(c.BodySHA256) || !hexDigest(c.TextSHA256) || len(c.Text) > 24<<10 || c.OmittedTextBytes < 0 || c.OmittedTextBytes > 512<<10 || researchTextHash(c.Text) != c.TextSHA256 {
			return "", empty, errResearch
		}
		if _, e = time.Parse(time.RFC3339Nano, c.FetchedAt); e != nil {
			return "", empty, errResearch
		}
		seen[c.ID] = true
	}
	return path, saved, nil
}
func validResearchEvidence(e *ResearchEvidence) bool {
	if e == nil {
		return true
	}
	if !strings.HasPrefix(e.CaptureID, "research-") || !prototypeReleaseID.MatchString(e.CaptureID) || !hexDigest(e.BodySHA256) || !hexDigest(e.TextSHA256) || !validArtifactStoredText(e.Title) || len(e.Title) == 0 || len(e.Title) > 500 || len(e.Quotes) < 1 || len(e.Quotes) > 8 || e.OmittedTextBytes < 0 || e.OmittedTextBytes > 512<<10 {
		return false
	}
	if _, err := researchURL(e.RequestedURL); err != nil {
		return false
	}
	if _, err := researchURL(e.URL); err != nil {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, e.FetchedAt); err != nil {
		return false
	}
	total, seen := 0, map[string]bool{}
	for _, q := range e.Quotes {
		if len(strings.TrimSpace(q)) == 0 || len(q) > 1000 || !validArtifactStoredText(q) || seen[q] {
			return false
		}
		total += len(q)
		seen[q] = true
	}
	return total <= 6000
}
func captureResearchEvidence(c ResearchCapture, quotes []string) *ResearchEvidence {
	return &ResearchEvidence{CaptureID: c.ID, RequestedURL: c.RequestedURL, URL: c.URL, Title: c.Title, FetchedAt: c.FetchedAt, BodySHA256: c.BodySHA256, TextSHA256: c.TextSHA256, Quotes: quotes, OmittedTextBytes: c.OmittedTextBytes}
}

// Attributed capture metadata is minted by Collect. An unchanged saved capture
// remains valid after preview expiry; a new/changed capture must match this
// actor's exact retained preview. Quotes are verbatim substrings, never model
// assertions represented as fetched text. Caller holds s.mu.
func (s *Server) validateResearchEvidenceLocked(ctx context.Context, p project.Project, next, current ProjectSpecification) error {
	existing := map[string]*ResearchEvidence{}
	if current.Strategy != nil {
		for _, e := range current.Strategy.Evidence {
			existing[e.ID] = e.Research
		}
	}
	if next.Strategy == nil {
		return nil
	}
	var previews *researchPreviews
	for _, e := range next.Strategy.Evidence {
		if e.Research == nil {
			continue
		}
		if !validResearchEvidence(e.Research) || e.URL != e.Research.URL || e.ObservedAt != e.Research.FetchedAt {
			return errResearch
		}
		if reflect.DeepEqual(existing[e.ID], e.Research) {
			continue
		}
		if previews == nil {
			ref, err := management.CaptureExecutionControlActor(ctx, p.ID)
			if err != nil || s.researchAuthorize == nil || s.researchAuthorize(ctx, ref, false) != nil {
				return errResearch
			}
			_, saved, err := s.researchPreviewsLocked(p, ref.UserID, ref.TeamID)
			if err != nil {
				return err
			}
			previews = &saved
		}
		found := false
		for _, c := range previews.Captures {
			if c.ID != e.Research.CaptureID {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, c.FetchedAt)
			if err != nil || time.Since(at) < 0 || time.Since(at) > 30*time.Minute {
				return errResearch
			}
			if !reflect.DeepEqual(captureResearchEvidence(c, e.Research.Quotes), e.Research) {
				return errResearch
			}
			for _, q := range e.Research.Quotes {
				if !strings.Contains(c.Text, q) {
					return errResearch
				}
			}
			found = true
		}
		if !found {
			return errResearch
		}
	}
	return nil
}
func (s *Server) registerResearch(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/research/connection", s.researchConnection)
	mux.HandleFunc("PUT /v1/product/projects/{id}/research/connection", s.researchConnection)
	mux.HandleFunc("POST /v1/product/projects/{id}/research/collect", s.collectResearch)
	mux.HandleFunc("POST /v1/product/projects/{id}/research/search", s.searchResearch)
	mux.HandleFunc("POST /v1/product/projects/{id}/research/investigate", s.investigateResearch)
	mux.HandleFunc("GET /v1/product/projects/{id}/observations", s.getProjectObservations)
	mux.HandleFunc("PUT /v1/product/projects/{id}/observations/route", s.saveProjectObservationRoute)
	mux.HandleFunc("POST /v1/product/projects/{id}/observations/queue", s.queueProjectObservations)
}
func (s *Server) collectResearch(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ref, err := s.researchAccess(r.Context(), p.ID, false)
	if err != nil {
		problem(w, 403, "research_unavailable", "Current project edit access is required.")
		return
	}
	var req struct {
		URL                   string `json:"url"`
		AcknowledgePublicRead bool   `json:"acknowledgePublicRead"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.AcknowledgePublicRead {
		problem(w, 400, "research_acknowledgement", "Explicitly choose a public URL. The website will receive an ordinary public read.")
		return
	}
	capture, err := fetchPublicResearch(r.Context(), req.URL)
	if err != nil || researchDenied(s, r.Context(), p.ID) {
		problem(w, 422, "research_unavailable", errResearch.Error())
		return
	}
	err = s.retainResearchCapture(r.Context(), p, ref, capture, false)
	if err != nil || researchDenied(s, r.Context(), p.ID) {
		problem(w, 422, "research_unavailable", errResearch.Error())
		return
	}
	journeyJSON(w, http.StatusOK, capture)
}
func (s *Server) retainResearchCapture(ctx context.Context, p project.Project, ref management.ExecutionActorRef, capture ResearchCapture, secrets bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref.ProjectID != p.ID || s.researchAuthorize == nil || s.researchAuthorize(ctx, ref, secrets) != nil {
		return management.ErrExecutionAuthority
	}
	path, saved, err := s.researchPreviewsLocked(p, ref.UserID, ref.TeamID)
	if err == nil {
		retained := []ResearchCapture{}
		for _, c := range saved.Captures {
			at, e := time.Parse(time.RFC3339Nano, c.FetchedAt)
			if e != nil {
				err = errResearch
				break
			}
			if time.Since(at) >= 0 && time.Since(at) <= 30*time.Minute {
				retained = append(retained, c)
			}
		}
		if len(retained) >= 16 {
			retained = retained[len(retained)-15:]
		}
		saved.Captures = append(retained, capture)
		raw, e := json.MarshalIndent(saved, "", "  ")
		if e != nil || len(raw)+1 > 2<<20 {
			err = errResearch
		} else if err == nil {
			err = atomicJSON(path, saved)
		}
	}
	return err
}
