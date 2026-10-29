package product

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const gtmHistoryLimit = 50

type GTMSegment struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Problem        string   `json:"problem"`
	Value          string   `json:"value"`
	RequirementIDs []string `json:"requirementIds"`
}
type GTMCompetitor struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Alternative string `json:"alternative"`
	Difference  string `json:"difference"`
}
type GTMChannel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Hypothesis string   `json:"hypothesis"`
	SegmentIDs []string `json:"segmentIds"`
}
type GTMMilestone struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Outcome    string   `json:"outcome"`
	TargetDate string   `json:"targetDate"`
	DependsOn  []string `json:"dependsOn"`
}
type GTMExperiment struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Hypothesis       string `json:"hypothesis"`
	Method           string `json:"method"`
	SuccessCriterion string `json:"successCriterion"`
	SegmentID        string `json:"segmentId"`
	ChannelID        string `json:"channelId"`
	Status           string `json:"status"`
	Result           string `json:"result"`
}
type GTMArtifactLink struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
}
type GTMContent struct {
	CommercialModel    *CommercialModelInput `json:"commercialModel,omitempty"`
	Positioning        string                `json:"positioning"`
	Offer              string                `json:"offer"`
	PricingAssumptions []string              `json:"pricingAssumptions"`
	Segments           []GTMSegment          `json:"segments"`
	Competitors        []GTMCompetitor       `json:"competitors"`
	Channels           []GTMChannel          `json:"channels"`
	Milestones         []GTMMilestone        `json:"milestones"`
	Experiments        []GTMExperiment       `json:"experiments"`
	ArtifactLinks      []GTMArtifactLink     `json:"artifactLinks"`
}
type GTMWorkspace struct {
	ProjectID        string                   `json:"projectId"`
	Revision         int                      `json:"revision"`
	Source           DistributeArtifactSource `json:"source"`
	SourceDigest     string                   `json:"sourceDigest"`
	Content          GTMContent               `json:"content"`
	Digest           string                   `json:"digest"`
	Actor            ContextActor             `json:"actor"`
	At               string                   `json:"at"`
	RequestKeyDigest string                   `json:"requestKeyDigest"`
	RequestDigest    string                   `json:"requestDigest"`
}
type gtmLedger struct {
	Version      int            `json:"version"`
	ProjectID    string         `json:"projectId"`
	AccountScope string         `json:"accountScope"`
	ProjectScope string         `json:"projectScope"`
	Revisions    []GTMWorkspace `json:"revisions"`
}
type GTMSummary struct {
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
	At       string `json:"at"`
}
type GTMRequirement struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
type GTMView struct {
	CommercialModel *CommercialModelProjection `json:"commercialModel,omitempty"`
	ProjectID       string                     `json:"projectId"`
	Workspace       *GTMWorkspace              `json:"workspace"`
	Source          DistributeArtifactSource   `json:"source"`
	SourceDigest    string                     `json:"sourceDigest"`
	Freshness       string                     `json:"freshness"`
	History         []GTMSummary               `json:"history"`
	Requirements    []GTMRequirement           `json:"requirements"`
}

func gtmDigest(row GTMWorkspace) string {
	row.Digest, row.RequestKeyDigest, row.RequestDigest = "", "", ""
	return reviewDigest(row)
}

// Empty fields are valid hypotheses/drafts, never verified commercial facts.
func validateGTM(c GTMContent) error {
	bad := errors.New("Keep GTM records within their bounds and repair all record links before saving.")
	text := func(s string) bool { return len(s) <= 4000 && validArtifactStoredText(s) }
	if !text(c.Positioning) || !text(c.Offer) || c.PricingAssumptions == nil || c.Segments == nil || c.Competitors == nil || c.Channels == nil || c.Milestones == nil || c.Experiments == nil || c.ArtifactLinks == nil {
		return bad
	}
	if c.CommercialModel != nil {
		if _, err := CompileCommercialModel(*c.CommercialModel); err != nil {
			return err
		}
	}
	for _, n := range []int{len(c.PricingAssumptions), len(c.Segments), len(c.Competitors), len(c.Channels), len(c.Milestones), len(c.Experiments), len(c.ArtifactLinks)} {
		if n > 50 {
			return bad
		}
	}
	for _, s := range c.PricingAssumptions {
		if !text(s) {
			return bad
		}
	}
	ids, segments, channels, milestones := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]GTMMilestone{}
	id := func(s string) bool {
		if !controlIDPattern.MatchString(s) || ids[s] {
			return false
		}
		ids[s] = true
		return true
	}
	links := func(xs []string, exists func(string) bool) bool {
		if xs == nil || len(xs) > 50 {
			return false
		}
		seen := map[string]bool{}
		for _, x := range xs {
			if seen[x] || !exists(x) {
				return false
			}
			seen[x] = true
		}
		return true
	}
	for _, s := range c.Segments {
		if !id(s.ID) || !text(s.Name) || !text(s.Problem) || !text(s.Value) || !links(s.RequirementIDs, func(x string) bool { return controlIDPattern.MatchString(x) }) {
			return bad
		}
		segments[s.ID] = true
	}
	for _, s := range c.Competitors {
		if !id(s.ID) || !text(s.Name) || !text(s.Alternative) || !text(s.Difference) {
			return bad
		}
	}
	for _, s := range c.Channels {
		if !id(s.ID) || !text(s.Name) || !text(s.Hypothesis) || !links(s.SegmentIDs, func(x string) bool { return segments[x] }) {
			return bad
		}
		channels[s.ID] = true
	}
	for _, s := range c.Milestones {
		if !id(s.ID) || !text(s.Title) || !text(s.Outcome) {
			return bad
		}
		if s.TargetDate != "" {
			if _, err := time.Parse("2006-01-02", s.TargetDate); err != nil {
				return bad
			}
		}
		milestones[s.ID] = s
	}
	for _, s := range c.Milestones {
		if !links(s.DependsOn, func(x string) bool { return x != s.ID && milestones[x].ID != "" }) {
			return bad
		}
	}
	marks := map[string]int{}
	var visit func(string) bool
	visit = func(id string) bool {
		if marks[id] == 1 {
			return false
		}
		if marks[id] == 2 {
			return true
		}
		marks[id] = 1
		for _, dep := range milestones[id].DependsOn {
			if !visit(dep) {
				return false
			}
		}
		marks[id] = 2
		return true
	}
	for id := range milestones {
		if !visit(id) {
			return bad
		}
	}
	for _, s := range c.Experiments {
		if !id(s.ID) || !text(s.Title) || !text(s.Hypothesis) || !text(s.Method) || !text(s.SuccessCriterion) || !text(s.Result) || s.SegmentID != "" && !segments[s.SegmentID] || s.ChannelID != "" && !channels[s.ChannelID] {
			return bad
		}
		switch s.Status {
		case "planned", "in-progress", "recorded", "retired":
		default:
			return bad
		}
	}
	seen := map[string]bool{}
	for _, a := range c.ArtifactLinks {
		key := a.ID
		if !prototypeReleaseID.MatchString(a.ID) || seen[key] || a.Revision < 1 || !hexDigest(a.Digest) {
			return bad
		}
		seen[key] = true
	}
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > 128<<10 {
		return bad
	}
	return nil
}

func (s *Server) readGTMLedger(p project.Project) (string, gtmLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", gtmLedger{}, err
	}
	path, err := guardedProjectPath(s.stateDir, "gtm-workspace-"+p.ID+".json", true)
	empty := gtmLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Revisions: []GTMWorkspace{}}
	if err != nil {
		return "", empty, err
	}
	var ledger gtmLedger
	err = externalReadJSON(path, artifactLedgerLimit, &ledger)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || ledger.Version != 1 || ledger.ProjectID != p.ID || ledger.AccountScope != empty.AccountScope || ledger.ProjectScope != empty.ProjectScope || ledger.Revisions == nil || len(ledger.Revisions) > gtmHistoryLimit {
		return "", empty, errArtifact
	}
	keys := map[string]bool{}
	for i, row := range ledger.Revisions {
		if row.ProjectID != p.ID || row.Revision != i+1 || validateGTM(row.Content) != nil || !validArtifactSource(row.Source) || row.SourceDigest != reviewDigest(row.Source) || row.Digest != gtmDigest(row) || !hexDigest(row.RequestKeyDigest) || keys[row.RequestKeyDigest] || !hexDigest(row.RequestDigest) || row.Actor.ID == "" || row.Actor.DisplayName == "" {
			return "", empty, errArtifact
		}
		keys[row.RequestKeyDigest] = true
		if _, err := time.Parse(time.RFC3339Nano, row.At); err != nil {
			return "", empty, errArtifact
		}
	}
	return path, ledger, nil
}

func gtmView(p project.Project, ledger gtmLedger, source DistributeArtifactSource, state journeyState) GTMView {
	view := GTMView{ProjectID: p.ID, Source: source, SourceDigest: reviewDigest(source), Freshness: "none", History: []GTMSummary{}, Requirements: []GTMRequirement{}}
	for _, r := range currentSpecification(state).Requirements {
		view.Requirements = append(view.Requirements, GTMRequirement{ID: r.ID, Title: r.Title})
	}
	for _, row := range ledger.Revisions {
		view.History = append(view.History, GTMSummary{Revision: row.Revision, Digest: row.Digest, At: row.At})
	}
	if len(ledger.Revisions) > 0 {
		row := ledger.Revisions[len(ledger.Revisions)-1]
		view.Workspace = &row
		view.Freshness = "current"
		if row.SourceDigest != view.SourceDigest {
			view.Freshness = "outdated"
		}
	}
	if view.Workspace != nil && view.Workspace.Content.CommercialModel != nil {
		projection, err := CompileCommercialModel(*view.Workspace.Content.CommercialModel)
		if err == nil {
			view.CommercialModel = &projection
		}
	}
	return view
}

func (s *Server) registerGTMWorkspace(mux Registrar) {
	base := "/v1/product/projects/{id}/distribute/gtm"
	mux.HandleFunc("GET "+base, s.getGTMWorkspace)
	mux.HandleFunc("POST "+base, s.saveGTMWorkspace)
}
func (s *Server) getGTMWorkspace(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readGTMLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	source, state, _, _, err := s.artifactSource(r.Context(), p, "")
	if err != nil {
		if len(ledger.Revisions) == 0 {
			artifactProblem(w, err)
			return
		}
		view := gtmView(p, ledger, ledger.Revisions[len(ledger.Revisions)-1].Source, journeyState{})
		view.Freshness = "unavailable"
		view.SourceDigest = ""
		journeyJSON(w, 200, view)
		return
	}
	journeyJSON(w, 200, gtmView(p, ledger, source, state))
}
func (s *Server) saveGTMWorkspace(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision     int        `json:"expectedRevision"`
		ExpectedDigest       string     `json:"expectedDigest"`
		ExpectedSourceDigest string     `json:"expectedSourceDigest"`
		IdempotencyKey       string     `json:"idempotencyKey"`
		Content              GTMContent `json:"content"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 0 || (req.ExpectedRevision == 0 && req.ExpectedDigest != "") || (req.ExpectedRevision > 0 && !hexDigest(req.ExpectedDigest)) || !hexDigest(req.ExpectedSourceDigest) || !validRunKey(req.IdempotencyKey) {
		journeyError(w, 400, "gtm_request_invalid", "Use the current revision, source and request key.")
		return
	}
	if err := validateGTM(req.Content); err != nil {
		journeyError(w, 422, "gtm_content_invalid", err.Error())
		return
	}
	if req.Content.CommercialModel != nil {
		normalized, _ := CompileCommercialModel(*req.Content.CommercialModel)
		req.Content.CommercialModel = &normalized.Input
	}
	key, requestDigest := artifactRequestKey(p.ID, actor.ID, req.IdempotencyKey), reviewDigest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, ledger, err := s.readGTMLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	for _, row := range ledger.Revisions {
		if row.RequestKeyDigest == key {
			if row.RequestDigest != requestDigest {
				journeyError(w, 409, "gtm_request_changed", "This request key already names different edits.")
				return
			}
			view := gtmView(p, gtmLedger{Revisions: []GTMWorkspace{row}}, row.Source, journeyState{})
			view.Freshness = "readback"
			journeyJSON(w, 200, view)
			return
		}
	}
	if len(ledger.Revisions) != req.ExpectedRevision || req.ExpectedRevision > 0 && ledger.Revisions[len(ledger.Revisions)-1].Digest != req.ExpectedDigest {
		journeyError(w, 409, "gtm_changed", "The saved GTM workspace changed. Keep your edits and review its current revision.")
		return
	}
	if len(ledger.Revisions) >= gtmHistoryLimit {
		journeyError(w, 409, "gtm_history_full", "This workspace reached its bounded revision history. Existing revisions remain readable.")
		return
	}
	source, state, _, _, err := s.artifactSource(r.Context(), p, "")
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if req.ExpectedSourceDigest != reviewDigest(source) {
		journeyError(w, 409, "gtm_source_changed", "Project context changed. Refresh and explicitly link your draft to the new source.")
		return
	}
	requirements := map[string]bool{}
	for _, r := range currentSpecification(state).Requirements {
		requirements[r.ID] = true
	}
	for _, segment := range req.Content.Segments {
		for _, id := range segment.RequirementIDs {
			if !requirements[id] {
				journeyError(w, 422, "gtm_requirement_changed", "Repair requirement links against the current project definition.")
				return
			}
		}
	}
	if len(req.Content.ArtifactLinks) > 0 {
		_, artifacts, e := s.readArtifactLedger(p)
		if e != nil {
			artifactProblem(w, e)
			return
		}
		for _, link := range req.Content.ArtifactLinks {
			found := false
			for _, record := range artifacts.Records {
				if record.ID == link.ID {
					for _, rev := range record.Revisions {
						if rev.Revision == link.Revision && rev.Digest == link.Digest {
							found = true
						}
					}
				}
			}
			if !found {
				journeyError(w, 422, "gtm_artifact_changed", "Choose an exact retained artifact revision from this project.")
				return
			}
		}
	}
	row := GTMWorkspace{ProjectID: p.ID, Revision: len(ledger.Revisions) + 1, Source: source, SourceDigest: reviewDigest(source), Content: req.Content, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), RequestKeyDigest: key, RequestDigest: requestDigest}
	row.Digest = gtmDigest(row)
	ledger.Revisions = append(ledger.Revisions, row)
	raw, err := json.Marshal(ledger)
	if err != nil || len(raw) > artifactLedgerLimit {
		journeyError(w, 413, "gtm_history_full", "The bounded GTM history is full.")
		return
	}
	if r.Context().Err() != nil {
		return
	}
	if err = durableRunJSON(path, ledger); err != nil {
		artifactProblem(w, err)
		return
	}
	journeyJSON(w, 200, gtmView(p, ledger, source, state))
}
