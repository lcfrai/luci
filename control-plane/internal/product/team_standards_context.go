package product

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const teamStandardsContextLimit = 64 << 10

var ErrTeamStandardsTooLarge = errors.New("The exact team standards and evidence exceed the bounded Design or execution context. Reduce the selected control/evidence set before preparing work.")

type OrchestrationStandardEvidence struct {
	Link DiscoveryEvidenceLink `json:"link"`
	Text string                `json:"text,omitempty"`
}

// No imported unknown columns or private original bytes enter a worker prompt.
// The immutable library and assessment revisions remain the authority for
// these selected, bounded fields.
type OrchestrationStandardControl struct {
	ID               string                          `json:"id"`
	Requirement      string                          `json:"requirement"`
	Category         string                          `json:"category"`
	Applicability    string                          `json:"applicability"`
	Owner            string                          `json:"owner"`
	EvidenceExpected string                          `json:"evidenceExpected"`
	Severity         string                          `json:"severity,omitempty"`
	ReviewCadence    string                          `json:"reviewCadence,omitempty"`
	AssessmentStatus string                          `json:"assessmentStatus,omitempty"`
	AssessmentNote   string                          `json:"assessmentNote,omitempty"`
	Evidence         []OrchestrationStandardEvidence `json:"evidence,omitempty"`
}

// Caller holds s.mu. Import writes are separately serialized by
// teamControlsMu; a later dispatch/apply checks the captured revision/digest.
func (s *Server) teamStandardsContext(ctx context.Context, p project.Project, state journeyState, quotes bool) (*OrchestrationStandards, error) {
	access, scoped := management.TeamAccessFromContext(ctx)
	if !scoped {
		return nil, nil
	}
	if !access.Permissions.Read || access.ProjectID != p.ID {
		return nil, errControlWorkbook
	}
	library, err := s.currentTeamLibrary(ctx)
	if err != nil || library == nil {
		return nil, err
	}
	out := &OrchestrationStandards{Status: "team-library-unassessed", Source: "team-control-library", Revision: library.Revision, Digest: library.Digest, TeamID: access.Team.ID, Requirements: make([]OrchestrationStandardControl, 0, len(library.Controls))}
	_, assessment, err := s.readControlAssessment(p)
	if err != nil {
		return nil, err
	}
	currentAssessment := assessment.Revision > 0 && assessment.TeamID == access.Team.ID && assessment.LibraryRevision == library.Revision && assessment.LibraryDigest == library.Digest && assessment.ContextDigest == assessmentContextDigest(state)
	results := map[string]ControlAssessmentResult{}
	if currentAssessment {
		out.Status = "team-library-assessed"
		out.AssessmentRevision = assessment.Revision
		out.AssessmentDigest = reviewDigest(assessment)
		for _, result := range assessment.Results {
			results[result.ControlID] = result
		}
	}
	refs := map[string][][2]int{}
	refCount := 0
	for _, control := range library.Controls {
		item := OrchestrationStandardControl{ID: control.ID, Requirement: control.Requirement, Category: control.Category, Applicability: control.Applicability, Owner: control.Owner, EvidenceExpected: control.EvidenceExpected, Severity: control.Severity, ReviewCadence: control.ReviewCadence}
		if result, ok := results[control.ID]; ok {
			item.AssessmentStatus, item.AssessmentNote = result.Status, result.Note
			for _, link := range result.EvidenceRefs {
				if link.TargetKind != "purpose" || link.TargetID != "" || refCount >= 200 {
					return nil, ErrTeamStandardsTooLarge
				}
				refCount++
				refs[link.DocumentID] = append(refs[link.DocumentID], [2]int{len(out.Requirements), len(item.Evidence)})
				item.Evidence = append(item.Evidence, OrchestrationStandardEvidence{Link: link})
			}
		}
		out.Requirements = append(out.Requirements, item)
	}
	if len(refs) > 16 {
		return nil, ErrTeamStandardsTooLarge
	}
	out.ContentDigest = reviewDigest(out.Requirements)
	if quotes {
		for id, positions := range refs {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			saved, err := s.readDiscoveryDocument(p, id)
			if err != nil {
				return nil, err
			}
			for _, position := range positions {
				entry := &out.Requirements[position[0]].Evidence[position[1]]
				segment, err := discoveryFindSegment(saved.Document, entry.Link)
				if err != nil {
					return nil, err
				}
				entry.Text = segment.Text
			}
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded) > teamStandardsContextLimit {
		return nil, ErrTeamStandardsTooLarge
	}
	return out, nil
}
