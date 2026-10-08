//go:build unix

package product

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProjectOverviewRefusesSpecialPrivateFilesWithoutOpeningThem(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-special-private")
	if err := writeJourney(p.Path, forgedReviewState(p.ID)); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"review-approvals", "orchestrator", "project-overview-questions"} {
		if err := os.MkdirAll(filepath.Join(s.stateDir, directory), 0700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(s.stateDir, directory, p.ID+".json"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A FIFO must not turn an overview into an indefinitely waiting read, and an
	// unreadable authority can never turn public approved-looking JSON into review.
	got := overviewGet(t, mux, p.ID)
	if got.Context == nil || got.Context.DefinitionStatus == "approved" || got.Status != "partial" || got.Questions.Count != nil {
		t.Fatalf("special private files trusted: %+v", got)
	}
}
