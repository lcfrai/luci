package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const artifactLedgerLimit = 8 << 20

var errArtifact = errors.New("The private artifact or its exact source is unavailable or invalid.")

type DistributeArtifactEvidence struct {
	RunID         string `json:"runId"`
	CheckID       string `json:"checkId"`
	ReleaseID     string `json:"releaseId"`
	Commit        string `json:"commit"`
	SourceDigest  string `json:"sourceDigest"`
	RunRevision   int    `json:"runRevision"`
	CheckDigest   string `json:"checkDigest"`
	ReleaseDigest string `json:"releaseDigest"`
	AcceptedAt    string `json:"acceptedAt"`
}

type DistributeArtifactSource struct {
	BriefRevision         int                         `json:"briefRevision"`
	BriefDigest           string                      `json:"briefDigest"`
	SpecificationRevision int                         `json:"specificationRevision"`
	SpecificationDigest   string                      `json:"specificationDigest"`
	DesignRevision        int                         `json:"designRevision"`
	DesignDigest          string                      `json:"designDigest"`
	PlanRevision          int                         `json:"planRevision"`
	PlanDigest            string                      `json:"planDigest"`
	TeamID                string                      `json:"teamId,omitempty"`
	LibraryRevision       int                         `json:"libraryRevision,omitempty"`
	LibraryDigest         string                      `json:"libraryDigest,omitempty"`
	AssessmentRevision    int                         `json:"assessmentRevision,omitempty"`
	AssessmentDigest      string                      `json:"assessmentDigest,omitempty"`
	SelectedEvidence      *DistributeArtifactEvidence `json:"selectedEvidence,omitempty"`
	// A report does not imply a check or release. Those evidence joins remain
	// explicit gaps until selected immutable receipts can be validated.
	Checks  string `json:"checks"`
	Release string `json:"release"`
}
type DistributeArtifactSection struct {
	ID         string   `json:"id"`
	Heading    string   `json:"heading"`
	Body       string   `json:"body"`
	Bullets    []string `json:"bullets"`
	SourceRefs []string `json:"sourceRefs"`
}
type DistributeArtifactReview struct {
	Actor  ContextActor `json:"actor"`
	At     string       `json:"at"`
	Digest string       `json:"digest"`
}
type DistributeArtifact struct {
	ID               string                      `json:"id"`
	ProjectID        string                      `json:"projectId"`
	Revision         int                         `json:"revision"`
	Kind             string                      `json:"kind"`
	Source           DistributeArtifactSource    `json:"source"`
	SourceDigest     string                      `json:"sourceDigest"`
	Title            string                      `json:"title"`
	Sections         []DistributeArtifactSection `json:"sections"`
	Gaps             []string                    `json:"gaps"`
	Assumptions      []string                    `json:"assumptions"`
	State            string                      `json:"state"`
	Digest           string                      `json:"digest"`
	Review           *DistributeArtifactReview   `json:"review,omitempty"`
	Actor            ContextActor                `json:"actor"`
	At               string                      `json:"at"`
	RequestKeyDigest string                      `json:"requestKeyDigest,omitempty"`
	RequestDigest    string                      `json:"requestDigest,omitempty"`
}
type distributeArtifactRecord struct {
	ID        string               `json:"id"`
	Revisions []DistributeArtifact `json:"revisions"`
}
type distributeArtifactLedger struct {
	Version                               int `json:"version"`
	ProjectID, AccountScope, ProjectScope string
	Records                               []distributeArtifactRecord `json:"records"`
}

func validArtifactKind(kind string) bool {
	switch kind {
	case "risk", "go-to-market", "proposal", "pricing", "presentation":
		return true
	}
	return false
}
func artifactDigest(value DistributeArtifact) string {
	value.Digest = ""
	value.RequestKeyDigest = ""
	value.RequestDigest = ""
	return reviewDigest(value)
}

// Older ledgers accepted every non-control UTF-8 scalar. Keep their exact
// bytes readable so a user can revise a retained draft without rewriting it.
func validArtifactStoredText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < 0x20 && char != '\t' && char != '\n' && char != '\r' {
			return false
		}
	}
	return true
}
func validArtifactXMLText(value string) bool {
	if !validArtifactStoredText(value) {
		return false
	}
	for _, char := range value {
		// XML 1.0 Fifth Edition character range, also used by encoding/xml.
		if char != '\t' && char != '\n' && char != '\r' &&
			!(char >= 0x20 && char <= 0xd7ff || char >= 0xe000 && char <= 0xfffd || char >= 0x10000 && char <= 0x10ffff) {
			return false
		}
	}
	return true
}
func validArtifactNewText(value DistributeArtifact) bool {
	for _, line := range artifactExportLines(value, "current") {
		if !validArtifactXMLText(line.Label) || !validArtifactXMLText(line.Value) {
			return false
		}
	}
	return true
}
func validArtifactSource(value DistributeArtifactSource) bool {
	if !hexDigest(value.BriefDigest) || !hexDigest(value.SpecificationDigest) || !hexDigest(value.DesignDigest) || !hexDigest(value.PlanDigest) || value.BriefRevision < 0 || value.SpecificationRevision < 0 || value.DesignRevision < 0 || value.PlanRevision < 0 || value.LibraryRevision < 0 || value.AssessmentRevision < 0 {
		return false
	}
	if value.LibraryRevision > 0 && (!hexDigest(value.LibraryDigest) || value.TeamID == "") || value.AssessmentRevision > 0 && (!hexDigest(value.AssessmentDigest) || value.LibraryRevision == 0) {
		return false
	}
	if selected := value.SelectedEvidence; selected != nil {
		if value.Checks != "accepted-selected" || value.Release != "accepted-selected" || !prototypeReleaseID.MatchString(selected.RunID) || !prototypeReleaseID.MatchString(selected.CheckID) || !prototypeReleaseID.MatchString(selected.ReleaseID) || selected.RunRevision < 1 || !deliverySHA.MatchString(selected.Commit) || !hexDigest(selected.SourceDigest) || !hexDigest(selected.CheckDigest) || !hexDigest(selected.ReleaseDigest) {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, selected.AcceptedAt); err != nil {
			return false
		}
	} else if value.Checks != "not-selected" || value.Release != "not-selected" {
		return false
	}
	return true
}
func validArtifactContent(value DistributeArtifact) bool {
	if !validArtifactKind(value.Kind) || !validArtifactSource(value.Source) || !validArtifactStoredText(value.Title) || strings.TrimSpace(value.Title) == "" || len(value.Title) > 200 || len(value.Sections) == 0 || len(value.Sections) > 40 || len(value.Gaps) > 100 || len(value.Assumptions) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, section := range value.Sections {
		if !controlIDPattern.MatchString(section.ID) || seen[section.ID] || strings.TrimSpace(section.Heading) == "" || !validArtifactStoredText(section.Heading) || len(section.Heading) > 200 || len(section.Body) > 8192 || !validArtifactStoredText(section.Body) || len(section.Bullets) > 100 || len(section.SourceRefs) > 100 {
			return false
		}
		seen[section.ID] = true
		for _, item := range append(append([]string{}, section.Bullets...), section.SourceRefs...) {
			if len(item) > 8192 || !validArtifactStoredText(item) {
				return false
			}
		}
	}
	for _, item := range append(append([]string{}, value.Gaps...), value.Assumptions...) {
		if len(item) > 8192 || !validArtifactStoredText(item) {
			return false
		}
	}
	raw, err := json.Marshal(value)
	return err == nil && len(raw) <= 256<<10
}

func (s *Server) artifactLedgerPath(p project.Project) (string, distributeArtifactLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", distributeArtifactLedger{}, err
	}
	path, err := guardedProjectPath(s.stateDir, "distribute-artifacts-"+p.ID+".json", true)
	return path, distributeArtifactLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Records: []distributeArtifactRecord{}}, err
}
func (s *Server) readArtifactLedger(p project.Project) (string, distributeArtifactLedger, error) {
	path, empty, err := s.artifactLedgerPath(p)
	if err != nil {
		return "", empty, err
	}
	var saved distributeArtifactLedger
	err = externalReadJSON(path, artifactLedgerLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != p.ID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || len(saved.Records) > 100 {
		return "", empty, errArtifact
	}
	ids := map[string]bool{}
	for _, record := range saved.Records {
		if !prototypeReleaseID.MatchString(record.ID) || ids[record.ID] || len(record.Revisions) < 1 || len(record.Revisions) > 20 {
			return "", empty, errArtifact
		}
		ids[record.ID] = true
		for i, row := range record.Revisions {
			if row.ID != record.ID || row.ProjectID != p.ID || row.Revision != i+1 || !hexDigest(row.SourceDigest) || row.SourceDigest != reviewDigest(row.Source) || row.Digest != artifactDigest(row) || !hexDigest(row.RequestKeyDigest) || !hexDigest(row.RequestDigest) || !validArtifactContent(row) || row.State != "draft" && row.State != "reviewed" || row.Actor.ID == "" || row.Actor.DisplayName == "" || row.At == "" {
				return "", empty, errArtifact
			}
			if row.State == "draft" && row.Review != nil || row.State == "reviewed" && (i == 0 || row.Review == nil || row.Review.Digest != record.Revisions[i-1].Digest || row.Review.Actor != row.Actor || row.Review.At != row.At) {
				return "", empty, errArtifact
			}
		}
	}
	return path, saved, nil
}
func writeArtifactLedger(path string, ledger distributeArtifactLedger) error {
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > artifactLedgerLimit {
		return errArtifact
	}
	return durableRunJSON(path, ledger)
}

// Caller holds s.mu then teamControlsMu. All source claims are server-derived from one private
// project snapshot; no selected check/release is invented from generic status.
func acceptedArtifactEvidence(run ProjectRun) (*DistributeArtifactEvidence, error) {
	if run.State != "complete" || run.Acceptance == nil || run.Acceptance.CheckID == "" || run.Acceptance.ReleaseID == "" {
		return nil, errArtifact
	}
	var check *ProjectRunCheckEvidence
	for i := range run.Checks {
		if run.Checks[i].CheckID == run.Acceptance.CheckID {
			check = &run.Checks[i]
			break
		}
	}
	var release *ProjectRunReleaseEvidence
	for i := range run.Releases {
		if run.Releases[i].ReleaseID == run.Acceptance.ReleaseID {
			release = &run.Releases[i]
			break
		}
	}
	if check == nil || release == nil || !check.OriginalAvailable || release.CheckID != check.CheckID || release.Commit != check.Commit || release.SourceDigest != check.SourceDigest {
		return nil, errArtifact
	}
	return &DistributeArtifactEvidence{RunID: run.ID, RunRevision: run.Revision, CheckID: check.CheckID, ReleaseID: release.ReleaseID, Commit: check.Commit, SourceDigest: check.SourceDigest, CheckDigest: reviewDigest(check), ReleaseDigest: reviewDigest(release), AcceptedAt: run.Acceptance.AcceptedAt}, nil
}

func (s *Server) artifactSource(ctx context.Context, p project.Project, selectedRun string) (DistributeArtifactSource, journeyState, *TeamControlRevision, *ControlAssessment, error) {
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return DistributeArtifactSource{}, state, nil, nil, err
	}
	spec, design, plan := currentSpecification(state), currentDesignPlan(state), currentPlan(state, p.ID)
	source := DistributeArtifactSource{BriefRevision: state.Brief.Revision, BriefDigest: reviewDigest(state.Brief), SpecificationRevision: spec.Revision, SpecificationDigest: reviewDigest(spec), DesignRevision: design.Revision, DesignDigest: reviewDigest(design), PlanRevision: plan.Revision, PlanDigest: reviewDigest(plan), Checks: "not-selected", Release: "not-selected"}
	if selectedRun != "" {
		if !prototypeReleaseID.MatchString(selectedRun) {
			return source, state, nil, nil, errArtifact
		}
		run, e := s.projectRuns.Get(p.ID, p.Path, selectedRun)
		if e != nil {
			return source, state, nil, nil, e
		}
		selected, e := acceptedArtifactEvidence(run)
		if e != nil {
			return source, state, nil, nil, e
		}
		source.SelectedEvidence = selected
		source.Checks = "accepted-selected"
		source.Release = "accepted-selected"
	}
	access, scoped := management.TeamAccessFromContext(ctx)
	if !scoped {
		return source, state, nil, nil, nil
	}
	if !access.Permissions.Read || access.ProjectID != p.ID {
		return source, state, nil, nil, errArtifact
	}
	source.TeamID = access.Team.ID
	_, ledger, err := s.readTeamControls(access.Team.ID)
	if err != nil {
		return source, state, nil, nil, err
	}
	library := latestControls(ledger)
	if library == nil {
		return source, state, nil, nil, nil
	}
	source.LibraryRevision, source.LibraryDigest = library.Revision, library.Digest
	_, assessment, err := s.readControlAssessment(p)
	if err != nil {
		return source, state, nil, nil, err
	}
	if assessment.Revision > 0 && assessment.TeamID == access.Team.ID && assessment.LibraryRevision == library.Revision && assessment.LibraryDigest == library.Digest && assessment.ContextDigest == assessmentContextDigest(state) {
		source.AssessmentRevision, source.AssessmentDigest = assessment.Revision, reviewDigest(assessment)
		return source, state, library, &assessment, nil
	}
	return source, state, library, nil, nil
}

func generatedArtifact(kind, projectID string, source DistributeArtifactSource, state journeyState, library *TeamControlRevision, assessment *ControlAssessment) DistributeArtifact {
	brief, spec, design, plan := state.Brief, currentSpecification(state), currentDesignPlan(state), currentPlan(state, projectID)
	value := DistributeArtifact{ProjectID: projectID, Kind: kind, Source: source, SourceDigest: reviewDigest(source), Sections: []DistributeArtifactSection{}, Gaps: []string{}, Assumptions: []string{}, State: "draft"}
	add := func(id, heading, body string, bullets []string, refs ...string) {
		value.Sections = append(value.Sections, DistributeArtifactSection{ID: id, Heading: heading, Body: body, Bullets: bullets, SourceRefs: refs})
	}
	value.Title = map[string]string{"risk": "Risk and evidence review", "go-to-market": "Go-to-market plan", "proposal": "Product proposal", "pricing": "Pricing evidence and assumptions", "presentation": "Project presentation"}[kind] + ": " + projectID
	add("purpose", "Saved product purpose", brief.Purpose, []string{"Audience: " + brief.Audience, "Constraints: " + brief.Constraints}, fmt.Sprintf("brief:%d:%s", source.BriefRevision, source.BriefDigest))
	if len(spec.Requirements) > 0 {
		items := []string{}
		for _, r := range spec.Requirements {
			items = append(items, r.ID+": "+r.Title+" — "+r.Acceptance)
		}
		add("requirements", "Reviewed requirement criteria", "These are retained project requirements, not proof of implementation.", items, fmt.Sprintf("specification:%d:%s", source.SpecificationRevision, source.SpecificationDigest))
	} else {
		value.Gaps = append(value.Gaps, "No saved requirement criteria are available.")
	}
	if len(design.Screens) > 0 {
		items := []string{}
		for _, screen := range design.Screens {
			items = append(items, screen.Name+": "+screen.Purpose)
		}
		add("experience", "Planned experience", "Saved Design screens and their stated purpose.", items, fmt.Sprintf("design:%d:%s", source.DesignRevision, source.DesignDigest))
	} else {
		value.Gaps = append(value.Gaps, "No saved Design screen plan is available.")
	}
	if len(plan.Tasks) > 0 {
		items := []string{}
		for _, task := range plan.Tasks {
			items = append(items, task.ID+": "+task.Title+" (recorded "+task.Status+")")
		}
		add("delivery", "Saved implementation plan", "Task status is recorded project data; it is not independently checked completion.", items, fmt.Sprintf("plan:%d:%s", source.PlanRevision, source.PlanDigest))
	} else {
		value.Gaps = append(value.Gaps, "No saved implementation plan is available.")
	}
	if library != nil {
		items := []string{}
		statuses := map[string]ControlAssessmentResult{}
		if assessment != nil {
			for _, result := range assessment.Results {
				statuses[result.ControlID] = result
			}
		}
		for _, control := range library.Controls {
			status := "unreviewed"
			if row, ok := statuses[control.ID]; ok {
				status = row.Status
			}
			items = append(items, control.ID+" · "+control.Requirement+" · "+status)
		}
		add("controls", "Imported controls and assessed gaps", "A recorded assessment is not a compliance certification.", items, fmt.Sprintf("team-library:%d:%s", source.LibraryRevision, source.LibraryDigest))
		if assessment == nil {
			value.Gaps = append(value.Gaps, "The current team control library has no current project assessment.")
		}
	}
	if source.SelectedEvidence == nil {
		value.Gaps = append(value.Gaps, "No exact implementation-check receipt was selected for this artifact.", "No accepted release evidence was selected for this artifact.")
	} else {
		selected := source.SelectedEvidence
		add("verification", "Selected accepted run evidence", "The named run retains an exact checked candidate and accepted observed release. The evidence records this project's past outcome; it does not verify new edits.", []string{"Run: " + selected.RunID, "Check: " + selected.CheckID, "Release: " + selected.ReleaseID, "Candidate commit: " + selected.Commit}, "run:"+selected.RunID+":"+selected.CheckDigest, "release:"+selected.ReleaseID+":"+selected.ReleaseDigest)
	}
	switch kind {
	case "risk":
		add("risk-register", "Risks to review", "This is a review queue, not a scored or mitigated risk register.", []string{"Evidence quality: check each cited source and unresolved assessment gap.", "Delivery: confirm the saved tasks have passing independent checks before claiming readiness.", "Operations: confirm an accepted release and recovery evidence before external delivery."}, fmt.Sprintf("specification:%d:%s", source.SpecificationRevision, source.SpecificationDigest))
		value.Assumptions = append(value.Assumptions, "Likelihood and impact require review against real incidents and check evidence.")
	case "go-to-market":
		add("audience", "Audience and value", "The saved audience and purpose supply a starting value statement; demand and positioning have not been tested.", []string{"Audience: " + brief.Audience, "Purpose: " + brief.Purpose}, fmt.Sprintf("brief:%d:%s", source.BriefRevision, source.BriefDigest))
		add("launch-readiness", "Launch readiness", "Choose channels and timing only after saved requirement checks, release observation and commercial review.", []string{"Channel: unconfirmed", "Launch date: unconfirmed", "Distribution owner: unconfirmed"})
		value.Gaps = append(value.Gaps, "No observed demand, channel decision or launch date was selected.")
		value.Assumptions = append(value.Assumptions, "Distribution channel, launch timing, adoption and demand are not verified by saved project context.")
	case "proposal":
		add("scope", "Proposed scope and acceptance", "Use the saved requirements and their criteria as the reviewable scope. Amend the source before treating a changed promise as approved.", []string{"Deliverables require a reviewed implementation plan and exact check evidence.", "Schedule, service terms and external commitments remain unconfirmed."}, fmt.Sprintf("specification:%d:%s", source.SpecificationRevision, source.SpecificationDigest))
		value.Assumptions = append(value.Assumptions, "Delivery estimate and customer commitment require separate evidence and approval.")
	case "pricing":
		add("commercial-basis", "Pricing basis", "No rate, unit, currency, tax treatment or margin can be calculated from the saved project context alone.", []string{"Unit and quantity: not selected", "Price and currency: not selected", "Cost evidence and payment terms: not selected"})
		value.Gaps = append(value.Gaps, "A real price quote needs reviewed units, costs, currency, tax and commercial approval.")
		value.Assumptions = append(value.Assumptions, "No product price, currency, margin or external cost is known from this project; enter and review real commercial evidence before quoting.")
	case "presentation":
		add("narrative", "Presentation sequence", "Use the saved purpose, criteria, Design screens and implementation plan as the presentation outline. Keep unresolved claims visible.", []string{"Problem and audience: saved brief", "Solution and acceptance: saved requirements", "Experience: saved Design", "Evidence, risks and next decision: selected receipts or explicit gaps"}, fmt.Sprintf("brief:%d:%s", source.BriefRevision, source.BriefDigest), fmt.Sprintf("design:%d:%s", source.DesignRevision, source.DesignDigest))
		value.Assumptions = append(value.Assumptions, "Claims in presentation material require exact current check and release evidence before external use.")
	}
	return value
}

func (s *Server) registerDistributeArtifacts(mux Registrar) {
	base := "/v1/product/projects/{id}/distribute/artifacts"
	mux.HandleFunc("GET "+base, s.listDistributeArtifacts)
	mux.HandleFunc("POST "+base+"/prepare", s.boundedDistributeWork(s.prepareDistributeArtifact))
	mux.HandleFunc("POST "+base, s.boundedDistributeWork(s.saveDistributeArtifact))
	mux.HandleFunc("GET "+base+"/{artifactId}", s.getDistributeArtifact)
	mux.HandleFunc("GET "+base+"/{artifactId}/exports/{format}", s.boundedDistributeWork(s.exportDistributeArtifact))
	mux.HandleFunc("GET "+base+"/{artifactId}/exports/pdf", s.boundedDistributePDFWork(s.exportDistributePDFArtifact))
	mux.HandleFunc("POST "+base+"/{artifactId}/revise", s.reviseDistributeArtifact)
	mux.HandleFunc("POST "+base+"/{artifactId}/review", s.reviewDistributeArtifact)
}

func artifactAccess(w http.ResponseWriter, r *http.Request, edit bool) bool {
	return discoveryDocumentAccess(w, r, edit)
}
func artifactProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, errArtifact) {
		journeyError(w, 409, "artifact_unavailable", err.Error())
	} else {
		journeyError(w, 409, "artifact_source_unavailable", "The exact saved project source cannot be verified. Refresh before changing this artifact.")
	}
}

func (s *Server) prepareDistributeArtifact(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind  string `json:"kind"`
		RunID string `json:"runId"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validArtifactKind(req.Kind) {
		journeyError(w, 422, "artifact_kind_invalid", "Choose a supported Distribute artifact kind.")
		return
	}
	s.mu.RLock()
	s.teamControlsMu.Lock()
	source, state, library, assessment, err := s.artifactSource(r.Context(), p, req.RunID)
	s.teamControlsMu.Unlock()
	s.mu.RUnlock()
	if err != nil {
		artifactProblem(w, err)
		return
	}
	draft := generatedArtifact(req.Kind, p.ID, source, state, library, assessment)
	if !validArtifactContent(draft) {
		journeyError(w, 413, "artifact_source_too_large", "The complete saved source exceeds the bounded artifact draft; split the reviewed scope.")
		return
	}
	if !validArtifactNewText(draft) {
		journeyError(w, 422, "artifact_content_invalid", "The saved source contains text Office XML cannot preserve. Correct the source before preparing this artifact.")
		return
	}
	journeyJSON(w, 200, map[string]any{"draft": draft, "prepareDigest": artifactDigest(draft)})
}

func artifactRecord(ledger *distributeArtifactLedger, id string) *distributeArtifactRecord {
	for i := range ledger.Records {
		if ledger.Records[i].ID == id {
			return &ledger.Records[i]
		}
	}
	return nil
}
func artifactRequestKey(projectID, actorID, key string) string {
	return reviewDigest(struct{ Project, Actor, Key string }{projectID, actorID, key})
}
func artifactReplay(ledger distributeArtifactLedger, key, digest string) (DistributeArtifact, bool, error) {
	for _, record := range ledger.Records {
		for _, row := range record.Revisions {
			if row.RequestKeyDigest == key {
				if row.RequestDigest != digest {
					return row, true, ErrProjectRunKeyConflict
				}
				return row, true, nil
			}
		}
	}
	return DistributeArtifact{}, false, nil
}
func artifactCurrentView(row DistributeArtifact, source DistributeArtifactSource) map[string]any {
	state := "current"
	if row.SourceDigest != reviewDigest(source) {
		state = "outdated"
	}
	return map[string]any{"artifact": row, "freshness": state}
}
func (s *Server) saveDistributeArtifact(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct{ Kind, RunID, PrepareDigest, IdempotencyKey string }
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validArtifactKind(req.Kind) || !hexDigest(req.PrepareDigest) || !validRunKey(req.IdempotencyKey) {
		journeyError(w, 400, "artifact_request_invalid", "Prepare a bounded draft and retain its exact request key.")
		return
	}
	key, digest := artifactRequestKey(p.ID, actor.ID, req.IdempotencyKey), reviewDigest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if row, found, e := artifactReplay(ledger, key, digest); found {
		if e != nil {
			artifactProblem(w, e)
		} else {
			journeyJSON(w, 200, map[string]any{"artifact": row, "freshness": "readback"})
		}
		return
	}
	if len(ledger.Records) >= 100 {
		journeyError(w, 409, "artifact_history_full", "This project reached its bounded artifact history. Existing records remain readable.")
		return
	}
	source, state, library, assessment, err := s.artifactSource(r.Context(), p, req.RunID)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	row := generatedArtifact(req.Kind, p.ID, source, state, library, assessment)
	if artifactDigest(row) != req.PrepareDigest || !validArtifactContent(row) {
		journeyError(w, 409, "artifact_source_changed", "The prepared source changed or exceeds the artifact bound. Prepare again before saving.")
		return
	}
	if !validArtifactNewText(row) {
		journeyError(w, 422, "artifact_content_invalid", "The saved source contains text Office XML cannot preserve. Correct the source before saving this artifact.")
		return
	}
	row.ID, row.Revision, row.Actor, row.At = newID("artifact-"), 1, actor, time.Now().UTC().Format(time.RFC3339Nano)
	row.RequestKeyDigest, row.RequestDigest = key, digest
	row.Digest = artifactDigest(row)
	if !validArtifactNewText(row) {
		journeyError(w, 422, "artifact_content_invalid", "This saved source contains text Office XML cannot preserve. Correct the source before saving this artifact.")
		return
	}
	ledger.Records = append(ledger.Records, distributeArtifactRecord{ID: row.ID, Revisions: []DistributeArtifact{row}})
	if err = writeArtifactLedger(path, ledger); err != nil {
		artifactProblem(w, err)
		return
	}
	journeyJSON(w, 201, artifactCurrentView(row, source))
}
func (s *Server) listDistributeArtifacts(w http.ResponseWriter, r *http.Request) {
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
	_, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	source, _, _, _, err := s.artifactSource(r.Context(), p, "")
	if err != nil {
		artifactProblem(w, err)
		return
	}
	rows := make([]map[string]any, 0, len(ledger.Records))
	for _, record := range ledger.Records {
		row := record.Revisions[len(record.Revisions)-1]
		if row.Source.TeamID != "" && row.Source.TeamID != source.TeamID {
			continue
		}
		freshness := artifactCurrentView(row, source)["freshness"]
		if row.Source.SelectedEvidence != nil {
			freshness = "requires-readback"
		}
		rows = append(rows, map[string]any{"id": row.ID, "kind": row.Kind, "title": row.Title, "revision": row.Revision, "state": row.State, "freshness": freshness, "digest": row.Digest})
	}
	journeyJSON(w, 200, map[string]any{"projectId": p.ID, "artifacts": rows, "exportFormats": artifactExportFormats()})
}
func (s *Server) getDistributeArtifact(w http.ResponseWriter, r *http.Request) {
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
	_, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	record := artifactRecord(&ledger, r.PathValue("artifactId"))
	if record == nil {
		journeyError(w, 404, "artifact_unavailable", "This artifact is not retained in the current project.")
		return
	}
	row := record.Revisions[len(record.Revisions)-1]
	access, scoped := management.TeamAccessFromContext(r.Context())
	if row.Source.TeamID != "" && (!scoped || access.Team.ID != row.Source.TeamID || !access.Permissions.Read || access.ProjectID != p.ID) {
		journeyError(w, 403, "artifact_scope_changed", "Current team access does not cover this artifact.")
		return
	}
	selectedRun := ""
	if row.Source.SelectedEvidence != nil {
		selectedRun = row.Source.SelectedEvidence.RunID
	}
	source, _, _, _, err := s.artifactSource(r.Context(), p, selectedRun)
	if err != nil {
		journeyJSON(w, 200, map[string]any{"artifact": row, "history": record.Revisions, "freshness": "unavailable"})
		return
	}
	if row.Source.TeamID != "" && row.Source.TeamID != source.TeamID {
		journeyError(w, 403, "artifact_scope_changed", "Current team access does not cover this artifact.")
		return
	}
	journeyJSON(w, 200, map[string]any{"artifact": row, "history": record.Revisions, "freshness": artifactCurrentView(row, source)["freshness"]})
}

func (s *Server) reviseDistributeArtifact(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision                                            int `json:"expectedRevision"`
		ExpectedDigest, ExpectedSourceDigest, IdempotencyKey, Title string
		Sections                                                    []DistributeArtifactSection
		Gaps, Assumptions                                           []string
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !hexDigest(req.ExpectedDigest) || !hexDigest(req.ExpectedSourceDigest) || !validRunKey(req.IdempotencyKey) {
		journeyError(w, 400, "artifact_request_invalid", "Use the current artifact revision, source digest and a fresh request key.")
		return
	}
	key, digest := artifactRequestKey(p.ID, actor.ID, req.IdempotencyKey), reviewDigest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if row, found, e := artifactReplay(ledger, key, digest); found {
		if e != nil {
			artifactProblem(w, e)
		} else {
			journeyJSON(w, 200, map[string]any{"artifact": row, "freshness": "readback"})
		}
		return
	}
	record := artifactRecord(&ledger, r.PathValue("artifactId"))
	if record == nil {
		journeyError(w, 404, "artifact_unavailable", "This artifact is not retained in the current project.")
		return
	}
	prior := record.Revisions[len(record.Revisions)-1]
	selectedRun := ""
	if prior.Source.SelectedEvidence != nil {
		selectedRun = prior.Source.SelectedEvidence.RunID
	}
	source, _, _, _, err := s.artifactSource(r.Context(), p, selectedRun)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if prior.Source.TeamID != source.TeamID || prior.Revision != req.ExpectedRevision || prior.Digest != req.ExpectedDigest || prior.SourceDigest != req.ExpectedSourceDigest || prior.SourceDigest != reviewDigest(source) {
		journeyError(w, 409, "artifact_changed", "The artifact or its exact saved source changed. Review the current version before editing.")
		return
	}
	if len(record.Revisions) >= 20 {
		journeyError(w, 409, "artifact_history_full", "This artifact reached its bounded revision history.")
		return
	}
	next := prior
	if len(req.Sections) != len(prior.Sections) {
		journeyError(w, 422, "artifact_sources_changed", "Keep the exact prepared source sections and references; prepare a new draft for changed source.")
		return
	}
	for i := range prior.Sections {
		if req.Sections[i].ID != prior.Sections[i].ID || req.Sections[i].Heading != prior.Sections[i].Heading || reviewDigest(req.Sections[i].SourceRefs) != reviewDigest(prior.Sections[i].SourceRefs) {
			journeyError(w, 422, "artifact_sources_changed", "Keep the exact prepared source sections and references; prepare a new draft for changed source.")
			return
		}
	}
	next.Revision++
	next.Title, next.Sections, next.Gaps, next.Assumptions = req.Title, req.Sections, req.Gaps, req.Assumptions
	next.State = "draft"
	next.Review = nil
	next.Actor = actor
	next.At = time.Now().UTC().Format(time.RFC3339Nano)
	next.RequestKeyDigest, next.RequestDigest = key, digest
	next.Digest = artifactDigest(next)
	if !validArtifactContent(next) || !validArtifactNewText(next) {
		journeyError(w, 422, "artifact_content_invalid", "Keep all reviewed artifact content within the declared section and size bounds.")
		return
	}
	record.Revisions = append(record.Revisions, next)
	if err = writeArtifactLedger(path, ledger); err != nil {
		artifactProblem(w, err)
		return
	}
	journeyJSON(w, 200, artifactCurrentView(next, source))
}
func (s *Server) reviewDistributeArtifact(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req struct {
		ExpectedRevision               int `json:"expectedRevision"`
		ExpectedDigest, IdempotencyKey string
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !hexDigest(req.ExpectedDigest) || !validRunKey(req.IdempotencyKey) {
		journeyError(w, 400, "artifact_request_invalid", "Review the exact saved revision with a fresh request key.")
		return
	}
	key, digest := artifactRequestKey(p.ID, actor.ID, req.IdempotencyKey), reviewDigest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if row, found, e := artifactReplay(ledger, key, digest); found {
		if e != nil {
			artifactProblem(w, e)
		} else {
			journeyJSON(w, 200, map[string]any{"artifact": row, "freshness": "readback"})
		}
		return
	}
	record := artifactRecord(&ledger, r.PathValue("artifactId"))
	if record == nil {
		journeyError(w, 404, "artifact_unavailable", "This artifact is not retained in the current project.")
		return
	}
	prior := record.Revisions[len(record.Revisions)-1]
	if prior.State != "draft" {
		journeyError(w, 409, "artifact_already_reviewed", "This exact revision has already been reviewed.")
		return
	}
	selectedRun := ""
	if prior.Source.SelectedEvidence != nil {
		selectedRun = prior.Source.SelectedEvidence.RunID
	}
	source, _, _, _, err := s.artifactSource(r.Context(), p, selectedRun)
	if err != nil {
		artifactProblem(w, err)
		return
	}
	if prior.Source.TeamID != source.TeamID || prior.Revision != req.ExpectedRevision || prior.Digest != req.ExpectedDigest || prior.SourceDigest != reviewDigest(source) {
		journeyError(w, 409, "artifact_changed", "The artifact or its exact saved source changed. Review the current version before approval.")
		return
	}
	if len(record.Revisions) >= 20 {
		journeyError(w, 409, "artifact_history_full", "This artifact reached its bounded revision history.")
		return
	}
	next := prior
	next.Revision++
	next.State = "reviewed"
	next.Actor = actor
	next.At = time.Now().UTC().Format(time.RFC3339Nano)
	next.Review = &DistributeArtifactReview{Actor: actor, At: next.At, Digest: prior.Digest}
	next.RequestKeyDigest, next.RequestDigest = key, digest
	next.Digest = artifactDigest(next)
	if !validArtifactNewText(next) {
		journeyError(w, 422, "artifact_content_invalid", "This retained draft contains text Office XML cannot preserve. Revise the text before review or export.")
		return
	}
	record.Revisions = append(record.Revisions, next)
	if err = writeArtifactLedger(path, ledger); err != nil {
		artifactProblem(w, err)
		return
	}
	journeyJSON(w, 200, artifactCurrentView(next, source))
}
