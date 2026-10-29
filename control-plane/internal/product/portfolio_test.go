package product

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func portfolioFixture() (project.Project, ProjectOverview, ProjectSwarmQueue) {
	zero := 0
	return project.Project{ID: "p-one", Name: "One"}, ProjectOverview{ProjectID: "p-one", Status: "ready", ObservedAt: "2026-10-29T00:00:00Z", Attention: ProjectOverviewAttention{Status: "recorded", Count: &zero, Omitted: &zero}}, ProjectSwarmQueue{Items: []SwarmQueueItem{}}
}

func TestPortfolioRanksUncertaintyAndDecisionsWithExactEvidence(t *testing.T) {
	p, view, queue := portfolioFixture()
	questions := 1
	view.Questions.Count = &questions
	view.LatestRun = ProjectOverviewRun{Status: "recorded", ID: "run-old", ContextStatus: "changed"}
	queue.Items = []SwarmQueueItem{
		{ID: "queue-wait", RunID: "run-1", TaskTitle: "Wait for API", State: "queued", ReasonCode: "capacity", Revision: 4},
		{ID: "queue-unknown", RunID: "run-2", TaskTitle: "Unconfirmed attempt", State: "unknown", Revision: 7},
	}
	row, items := portfolioProject(p, view, queue, nil, coordinatorContinuations{}, nil)
	got, err := AssemblePortfolio([]PortfolioProject{row}, items, 1, time.Now())
	if err != nil || len(got.Interventions) != 4 {
		t.Fatalf("projection: %+v %v", got, err)
	}
	for i, kind := range []string{"unconfirmed", "question", "context", "capacity"} {
		if got.Interventions[i].Kind != kind {
			t.Fatalf("wrong causal order: %+v", got.Interventions)
		}
	}
	first := got.Interventions[0]
	if first.Evidence[0].ID != "queue-unknown" || first.Evidence[0].Revision != 7 || first.NextAction.RunID != "run-2" {
		t.Fatalf("lost exact attempt identity: %+v", first)
	}
	// Input order must not become a hidden scheduling/priority decision.
	items[0], items[1] = items[1], items[0]
	again, _ := AssemblePortfolio([]PortfolioProject{row}, items, 1, time.Now())
	for i := range got.Interventions {
		if got.Interventions[i].ID != again.Interventions[i].ID {
			t.Fatal("unstable intervention identity/order")
		}
	}
}

func TestPortfolioUnavailableEvidenceNeverBecomesClear(t *testing.T) {
	p, view, queue := portfolioFixture()
	view.Status = "unavailable"
	row, items := portfolioProject(p, view, queue, errors.New("private backend detail"), coordinatorContinuations{}, errors.New("private ledger path"))
	got, err := AssemblePortfolio([]PortfolioProject{row}, items, 1, time.Now())
	if err != nil || got.Status != "partial" || row.Coordinator.Status != "unavailable" || len(items) != 3 {
		t.Fatalf("missing evidence treated as idle: %+v %v", got, err)
	}
	for _, item := range items {
		if item.Kind != "unconfirmed" || strings.Contains(item.Reason, "private") {
			t.Fatalf("raw error leaked or uncertainty lost: %+v", item)
		}
	}
}

func TestPortfolioCoordinatorUsesAllUnresolvedRecordsNotOnlyLatest(t *testing.T) {
	p, view, queue := portfolioFixture()
	private := coordinatorContinuations{Rows: []coordinatorContinuationRecord{
		{CoordinatorContinuation: CoordinatorContinuation{ID: "continue-old", Revision: 3, Status: "unknown", RunID: "run-old", UpdatedAt: "2026-10-28T00:00:00Z", Message: "secret model output"}},
		{CoordinatorContinuation: CoordinatorContinuation{ID: "continue-new", Revision: 8, Status: "verified", RunID: "run-new", UpdatedAt: "2026-10-29T00:00:00Z"}},
	}}
	row, items := portfolioProject(p, view, queue, nil, private, nil)
	if row.Coordinator.ID != "continue-new" || row.Coordinator.State != "verified" || row.Coordinator.Unresolved != 1 || len(items) != 1 || items[0].NextAction.RunID != "run-old" {
		t.Fatalf("older unresolved work hidden by latest: %+v %+v", row, items)
	}
	if strings.Contains(items[0].Reason, "secret") {
		t.Fatal("model prose entered portfolio")
	}
}

func TestPortfolioBoundsAndCrossProjectEvidenceFailClosed(t *testing.T) {
	projects := []PortfolioProject{{ID: "p-one", Status: "ready"}}
	items := []PortfolioIntervention{}
	for i := 0; i < portfolioInterventionLimit+1; i++ {
		items = append(items, PortfolioIntervention{ID: reviewDigest(i), ProjectID: "p-one", Kind: "review"})
	}
	view, err := AssemblePortfolio(projects, items, 2, time.Now())
	if err != nil || view.Status != "partial" || view.OmittedProjects != 1 || view.OmittedInterventions != 1 || len(view.Interventions) != portfolioInterventionLimit {
		t.Fatalf("bounds: %+v %v", view, err)
	}
	items[0].ProjectID = "p-hidden"
	if _, err := AssemblePortfolio(projects, items, 1, time.Now()); err == nil {
		t.Fatal("orphan private evidence accepted")
	}
	if _, err := AssemblePortfolio(append(projects, projects[0]), nil, 2, time.Now()); err == nil {
		t.Fatal("duplicate project accepted")
	}
}

func TestPortfolioOmittedRunObligationsRemainPartialAndPrivateReviewsKeepTheirKind(t *testing.T) {
	p, view, queue := portfolioFixture()
	count, omitted := 4, 1
	view.Attention = ProjectOverviewAttention{Status: "recorded", Count: &count, Omitted: &omitted, Items: []ProjectRunAttention{{ID: "run-one"}, {ID: "run-two"}, {ID: "run-three"}}}
	private := coordinatorContinuations{Findings: []CoordinatorFinding{{ID: "finding-one", Status: "review"}}}
	row, items := portfolioProject(p, view, queue, nil, private, nil)
	if row.Status != "partial" || len(items) != 5 {
		t.Fatalf("hidden review obligations treated as complete: %+v %+v", row, items)
	}
	found := false
	for _, item := range items {
		if item.Evidence[0].Source == "run-history" {
			found = item.NextAction.RunID == "" && item.NextAction.Label == "Open full project work"
		}
		if item.Evidence[0].Source == "finding" && item.Kind != "review" {
			t.Fatalf("private review became a question: %+v", item)
		}
	}
	if !found {
		t.Fatal("missing full-history destination")
	}
	view.Attention = ProjectOverviewAttention{Status: "unavailable"}
	row, items = portfolioProject(p, view, queue, nil, coordinatorContinuations{}, nil)
	if row.Status != "partial" || len(items) != 1 || items[0].Kind != "unconfirmed" {
		t.Fatalf("unknown attention treated as empty: %+v %+v", row, items)
	}
}
