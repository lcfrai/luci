package product

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectRunApprovalSelectionUsesPrivateAuthorityAndExactSelection(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "run-reviews")
	writeUntrustedReviewBytes(t, p, forgedReviewState(p.ID))
	if _, err := s.ApprovalSelection(p.ID); err == nil {
		t.Fatal("project-authored review claims minted a run approval selection")
	}
	// Use a distinct clean project so fixture expectations do not inherit forged
	// revisions. The real authenticated review endpoints mint both receipts.
	p = createJourneyProject(t, mux, "run-private-reviews")
	approveEvidenceJourney(t, mux, p.ID, "AC-save")
	selected, err := s.ApprovalSelection(p.ID)
	if err != nil || selected.SpecificationApprovalID == "" || selected.DesignApprovalID == "" || !hexDigest(selected.BriefDigest) || !hexDigest(selected.SketchDigest) {
		t.Fatal("private selection unavailable", selected, err)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.recordJourneyApproval(p.Path, p.ID, "design", "fixture-reviewer", &state); err != nil {
		t.Fatal(err)
	}
	current, err := s.ApprovalSelection(p.ID)
	if err != nil || current.DesignDigest != selected.DesignDigest || current.DesignApprovalID == selected.DesignApprovalID {
		t.Fatal("identical content reapproval did not change the exact private selection", err)
	}
	if err := s.validateProjectRunApprovals(p.ID, selected); err == nil {
		t.Fatal("older private selection survived a new review")
	}
	data, _ := json.Marshal(current)
	for _, forbidden := range []string{"fixture-reviewer", "accountScope", "projectScope", s.stateDir, p.Path} {
		if strings.Contains(string(data), forbidden) {
			t.Fatal("public selection exposed private receipt metadata", forbidden)
		}
	}
	for _, name := range []string{"specification.json", "design-handoff.json"} {
		exported, err := os.ReadFile(filepath.Join(p.Path, ".luci", name))
		if err != nil || strings.Contains(string(exported), current.DesignApprovalID) || strings.Contains(string(exported), current.SpecificationApprovalID) {
			t.Fatal("private selection leaked into coding context", name, err)
		}
	}
}
