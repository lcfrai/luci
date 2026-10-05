package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func approveEvidenceJourney(t *testing.T, mux http.Handler, id, requirementID string) (ProjectSpecification, ProjectDesignPlan) {
	t.Helper()
	base := "/v1/product/projects/" + id
	spec := specificationFromBrief(Brief{ProjectID: id, Purpose: "Keep saved work", Audience: "Project owners"})
	spec.Requirements = []SpecificationRequirement{{ID: requirementID, Title: "Restore saved work", Priority: "must", Acceptance: "Given saved work, reopening returns its exact contents."}}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200).Body.Bytes(), &spec)
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 200).Body.Bytes(), &spec)
	var design ProjectDesignPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &design)
	design.Screens = []DesignScreen{{ID: "home", Name: "Saved work", Route: "/", Purpose: "Reopen saved work", States: []string{"Empty", "Loading", "Saved work", "Error and retry"}, Widths: []int{390, 1440}, RequirementIDs: []string{requirementID}}}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": 0, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, 200).Body.Bytes(), &design)
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": design.Revision}, 200).Body.Bytes(), &design)
	return spec, design
}

func decodeEvidenceResponse(t *testing.T, data []byte, value any) {
	t.Helper()
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

type projectEvidenceResponse struct {
	Runs     []ProjectCheckRun          `json:"runs"`
	Coverage ProjectRequirementCoverage `json:"coverage"`
}

func getEvidence(t *testing.T, mux http.Handler, id string) projectEvidenceResponse {
	t.Helper()
	var value projectEvidenceResponse
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", "/v1/product/projects/"+id+"/checks", nil, 200).Body.Bytes(), &value)
	return value
}

func evidenceRequest(spec ProjectSpecification, design ProjectDesignPlan, args ...string) map[string]any {
	return map[string]any{"commands": []CheckCommand{{Label: "Recorded native process", Executable: "node", Args: args}}, "acknowledgeHostExecution": true,
		"requirementIds": []string{spec.Requirements[0].ID}, "expectedSpecificationRevision": spec.Revision, "expectedDesignPlanRevision": design.Revision}
}

// Only substitute the Docker transport; execute real Node and every native Git
// operation, preserving production source hashing and persisted job lifecycle.
func useActualEvidenceProcesses(t *testing.T, s *Server) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is required for the real evidence command outcomes")
	}
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "docker" {
			return original(ctx, name, args, dir)
		}
		if len(args) > 0 && args[0] == "rm" {
			return "", nil
		}
		for i, arg := range args {
			if arg == "--entrypoint" && i+3 <= len(args) && args[i+1] == "node" {
				return RunCommand(ctx, node, args[i+3:], dir)
			}
		}
		return "", errors.New("unexpected evidence runtime command")
	}
}

func finishEvidenceRun(t *testing.T, s *Server, projectID, runID string) ProjectCheckRun {
	t.Helper()
	row := waitProjectCheck(t, s, projectID, runID)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		_, active := s.projectChecks[runID]
		s.mu.RUnlock()
		if !active {
			return row
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("terminal evidence did not release its owned check reservation")
	return row
}

func TestProjectRequirementBindingRejectsForeignUnknownDuplicateAndSupersededApprovals(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "evidence-owner")
	q := createJourneyProject(t, mux, "evidence-neighbour")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	approveEvidenceJourney(t, mux, q.ID, "AC-foreign")
	base := "/v1/product/projects/" + p.ID
	plan := func(ids []string, expected any) map[string]any {
		return map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": expected, "tasks": []ProjectTask{{ID: "save", Title: "Implement saving", Status: "done", RequirementIDs: ids}}}
	}
	for _, invalid := range []struct {
		ids      []string
		revision any
		status   int
	}{
		{[]string{"AC-foreign"}, spec.Revision, 400}, {[]string{"unknown"}, spec.Revision, 400}, {[]string{"AC-save", "AC-save"}, spec.Revision, 400},
		{[]string{"AC-save"}, nil, 409}, {[]string{"AC-save"}, spec.Revision - 1, 409},
	} {
		journeyRequest(t, mux, "POST", base+"/plan", plan(invalid.ids, invalid.revision), invalid.status)
	}
	var saved ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", plan([]string{"AC-save"}, spec.Revision), 200).Body.Bytes(), &saved)
	if saved.SpecificationRevision != spec.Revision || saved.SpecificationDigest != spec.Digest || !reflect.DeepEqual(saved.Tasks[0].RequirementIDs, []string{"AC-save"}) {
		t.Fatal("plan lost approval binding", saved)
	}
	coverage := getEvidence(t, mux, p.ID).Coverage
	if !coverage.Source.Available || coverage.Source.Commit != "" || len(coverage.Rows) != 1 || coverage.Rows[0].EvidenceStatus != evidenceUnverified || !reflect.DeepEqual(coverage.Rows[0].DoneTaskIDs, []string{"save"}) {
		t.Fatal("user Done was converted into evidence", coverage)
	}
	export, err := os.ReadFile(filepath.Join(p.Path, ".luci", "plan.json"))
	if err != nil || !strings.Contains(string(export), `"requirementIds"`) {
		t.Fatal("agent context omitted links", err)
	}
	dockerCalls := 0
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name == "docker" {
			dockerCalls++
			return "", errors.New("rejected requests must not allocate checks")
		}
		return original(ctx, name, args, dir)
	}
	for _, invalid := range []struct {
		ids                          []string
		specRevision, designRevision any
		status                       int
	}{
		{[]string{"AC-foreign"}, spec.Revision, design.Revision, 400}, {[]string{"unknown"}, spec.Revision, design.Revision, 400},
		{[]string{"AC-save", "AC-save"}, spec.Revision, design.Revision, 400}, {[]string{"AC-save"}, nil, design.Revision, 409},
		{[]string{"AC-save"}, spec.Revision - 1, design.Revision, 409}, {[]string{"AC-save"}, spec.Revision, nil, 409},
		{[]string{"AC-save"}, spec.Revision, design.Revision - 1, 409},
	} {
		req := evidenceRequest(spec, design, "-e", "process.exit(0)")
		req["requirementIds"], req["expectedSpecificationRevision"], req["expectedDesignPlanRevision"] = invalid.ids, invalid.specRevision, invalid.designRevision
		journeyRequest(t, mux, "POST", base+"/checks", req, invalid.status)
	}
	// A saved draft supersedes an earlier approved snapshot; neither its stale
	// revision nor the same stable requirement ID can authorize a new linked run.
	spec.Requirements[0].Acceptance = "Given saved work, reopening also restores its labels."
	var draft ProjectSpecification
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 200).Body.Bytes(), &draft)
	journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "-e", "process.exit(0)"), 422)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": draft.Revision, "briefRevision": draft.BriefRevision}, 200)
	journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "-e", "process.exit(0)"), 409)
	var approved ProjectSpecification
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &approved)
	journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(approved, design, "-e", "process.exit(0)"), 422)
	if dockerCalls != 0 {
		t.Fatal("a refused binding allocated a runtime")
	}
	s.mu.RLock()
	history, err := s.readCheckHistory(p.ID)
	active := len(s.projectChecks)
	s.mu.RUnlock()
	if err != nil || len(history) != 0 || active != 0 {
		t.Fatal("a refused binding allocated a check receipt", history, active, err)
	}
	// Legacy unlinked plans remain usable, including explicitly removing links.
	journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": saved.Revision, "tasks": []ProjectTask{{ID: "save", Title: "Implement saving", Status: "done"}}}, 200)
}

func TestLinkedChecksPersistNativeEvidenceAndInvalidateSourceCommitAndAcceptanceChanges(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "native-evidence")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	base := "/v1/product/projects/" + p.ID
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`console.log('actual linked result');process.exit(0)`), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "Save exact source and approvals"}, 200)
	useActualEvidenceProcesses(t, s)
	var run ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "check.js"), 202).Body.Bytes(), &run)
	finished := finishEvidenceRun(t, s, p.ID, run.ID)
	if finished.Status != "passed" || finished.SourceChanged || finished.Commit == "" || !hexDigest(finished.SourceDigest) || len(finished.RequirementSnapshots) != 1 || finished.RequirementSnapshots[0].Acceptance != spec.Requirements[0].Acceptance || finished.SpecificationDigest != spec.Digest || finished.DesignPlanDigest != design.Digest || !strings.Contains(finished.Commands[0].Output, "actual linked result") {
		t.Fatal("native outcome lost immutable bindings", finished)
	}
	value := getEvidence(t, mux, p.ID)
	if len(value.Runs) != 1 || value.Runs[0].EvidenceStatus != evidenceLinked || value.Coverage.Rows[0].EvidenceStatus != evidenceLinked || !value.Coverage.Source.Available || value.Coverage.Source.Dirty || value.Coverage.Source.SourceDigest != finished.SourceDigest {
		t.Fatal("current native evidence was not linked", value)
	}
	historyBefore, err := os.ReadFile(s.checkHistoryPath(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	// A distinct commit with identical file bytes still changes the provenance.
	if _, err := s.journeyGit(context.Background(), p.Path, "-c", "user.name=Luci Test Person", "-c", "user.email=person@example.com", "commit", "--allow-empty", "-m", "Advance unchanged source"); err != nil {
		t.Fatal(err)
	}
	value = getEvidence(t, mux, p.ID)
	if value.Coverage.Source.SourceDigest != finished.SourceDigest || value.Coverage.Source.Commit == finished.Commit || value.Runs[0].EvidenceStatus != evidenceStale || value.Runs[0].Status != "passed" {
		t.Fatal("Git provenance changed without invalidating evidence", value)
	}
	var lookup ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/checks/"+run.ID, nil, 200).Body.Bytes(), &lookup)
	if lookup.EvidenceStatus != evidenceStale {
		t.Fatal("direct lookup omitted staleness", lookup)
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "check.js"), 202).Body.Bytes(), &run)
	newRun := finishEvidenceRun(t, s, p.ID, run.ID)
	if getEvidence(t, mux, p.ID).Runs[0].EvidenceStatus != evidenceLinked {
		t.Fatal("rerun did not bind current commit")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`console.log('edited source');process.exit(0)`), 0600); err != nil {
		t.Fatal(err)
	}
	value = getEvidence(t, mux, p.ID)
	if value.Coverage.Source.SourceDigest == newRun.SourceDigest || value.Runs[0].EvidenceStatus != evidenceStale || !value.Coverage.Source.Dirty {
		t.Fatal("edited source remained current evidence", value)
	}
	spec.Requirements[0].Acceptance = "Reopening returns exact contents and new labels."
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 200)
	value = getEvidence(t, mux, p.ID)
	if value.Coverage.Specification.Status != "draft" || value.Coverage.Rows[0].Acceptance != spec.Requirements[0].Acceptance || value.Runs[0].EvidenceStatus != evidenceStale || !strings.Contains(strings.Join(value.Runs[0].StaleReasons, " "), "acceptance text") {
		t.Fatal("changed same-ID requirement hid stale evidence", value)
	}
	// Reads preserve original outcomes and snapshots; they only derive a view.
	var persisted []ProjectCheckRun
	data, _ := os.ReadFile(s.checkHistoryPath(p.ID))
	decodeEvidenceResponse(t, data, &persisted)
	if persisted[1].RequirementSnapshots[0].Acceptance != finished.RequirementSnapshots[0].Acceptance || persisted[1].Status != "passed" || !strings.Contains(string(historyBefore), finished.SourceDigest) {
		t.Fatal("staleness rewrote immutable evidence")
	}
}

func TestLinkedFailedCancelledAndHistoricalChecksStayUnverified(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "unverified-evidence")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	useActualEvidenceProcesses(t, s)
	base := "/v1/product/projects/" + p.ID + "/checks"
	start := func(req map[string]any) ProjectCheckRun {
		var row ProjectCheckRun
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base, req, 202).Body.Bytes(), &row)
		return row
	}
	passing := start(evidenceRequest(spec, design, "-e", "process.exit(0)"))
	finishEvidenceRun(t, s, p.ID, passing.ID)
	failed := start(evidenceRequest(spec, design, "-e", "process.exit(3)"))
	failed = finishEvidenceRun(t, s, p.ID, failed.ID)
	if failed.Status != "failed" || failed.Commands[0].ExitCode == nil || *failed.Commands[0].ExitCode != 3 {
		t.Fatal("real failure was lost", failed)
	}
	cancelled := start(evidenceRequest(spec, design, "-e", "setInterval(()=>{},1000)"))
	journeyRequest(t, mux, "POST", base+"/"+cancelled.ID+"/cancel", map[string]any{}, 202)
	cancelled = finishEvidenceRun(t, s, p.ID, cancelled.ID)
	if cancelled.Status != "cancelled" {
		t.Fatal("cancellation lost terminal status", cancelled)
	}
	legacy := start(map[string]any{"commands": []CheckCommand{{Label: "Generic legacy check", Executable: "node", Args: []string{"-e", "process.exit(0)"}}}, "acknowledgeHostExecution": true})
	finishEvidenceRun(t, s, p.ID, legacy.ID)
	value := getEvidence(t, mux, p.ID)
	if len(value.Runs) != 4 || value.Runs[0].EvidenceStatus != evidenceUnverified || value.Runs[1].EvidenceStatus != evidenceUnverified || value.Runs[2].EvidenceStatus != evidenceUnverified || value.Runs[3].EvidenceStatus != evidenceLinked || !reflect.DeepEqual(value.Coverage.Rows[0].EvidenceRunIDs, []string{cancelled.ID, failed.ID, passing.ID}) {
		t.Fatal("contrary or legacy outcomes were hidden", value)
	}
	if !strings.Contains(strings.Join(value.Runs[1].StaleReasons, " "), "cancelled") || !strings.Contains(strings.Join(value.Runs[2].StaleReasons, " "), "failed") {
		t.Fatal("failure reasons were not distinct", value.Runs)
	}
	// A reload retains bindings and generic historical runs without retroactive
	// linking; wrong-project lookup cannot borrow the owner's run.
	reloaded, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	other := http.NewServeMux()
	reloaded.registerProjects(other)
	reloaded.registerProjectWork(other)
	got := getEvidence(t, other, p.ID)
	if !reflect.DeepEqual(got.Coverage.Rows[0].EvidenceRunIDs, value.Coverage.Rows[0].EvidenceRunIDs) || got.Runs[0].EvidenceStatus != evidenceUnverified {
		t.Fatal("reload changed evidence links", got)
	}
	q := createJourneyProject(t, other, "unverified-neighbour")
	journeyRequest(t, other, "GET", "/v1/product/projects/"+q.ID+"/checks/"+passing.ID, nil, 404)
}

func TestEvidenceSemanticDriftIncompleteOutcomesAndSnapshotsAreIndependent(t *testing.T) {
	id := "p-evidence"
	spec := specificationFromBrief(Brief{ProjectID: id, Revision: 1, Purpose: "Saved work", Audience: "People"})
	spec.Revision, spec.Status = 2, "approved"
	spec.Requirements = []SpecificationRequirement{{ID: "AC-save", Title: "Save work", Priority: "must", Acceptance: "Exact contents return"}}
	spec.Digest = specificationContentDigest(spec)
	design := ProjectDesignPlan{ProjectID: id, Revision: 2, BriefRevision: 1, SpecificationRevision: 2, Status: "approved", ImplementationNotes: "Use saved state"}
	design.Digest = designPlanContentDigest(design)
	state := journeyState{Brief: Brief{ProjectID: id, Revision: 1}, Design: Design{ProjectID: id}, Specification: &spec, DesignPlan: &design,
		Plan: &ProjectPlan{ProjectID: id, Tasks: []ProjectTask{{ID: "task", Title: "Save", Status: "done", RequirementIDs: []string{"AC-save"}}}}}
	zero := 0
	source := EvidenceSource{Available: true, SourceDigest: strings.Repeat("a", 64), Commit: strings.Repeat("b", 40)}
	row := ProjectCheckRun{ID: "check-1", ProjectID: id, Status: "passed", Commit: source.Commit, SourceDigest: source.SourceDigest, RequirementIDs: []string{"AC-save"}, RequirementSnapshots: []RequirementSnapshot{{ID: "AC-save", Title: "Save work", Acceptance: "Exact contents return"}}, SpecificationRevision: 2, SpecificationDigest: spec.Digest, DesignPlanRevision: 2, DesignPlanDigest: design.Digest, Commands: []CheckCommandResult{{CheckCommand: CheckCommand{Args: []string{"test"}}, Status: "passed", ExitCode: &zero}}}
	view := func() ProjectCheckRun {
		_, rows := projectEvidenceView(state, id, source, []ProjectCheckRun{row})
		return rows[0]
	}
	if view().EvidenceStatus != evidenceLinked {
		t.Fatal(view())
	}
	// Semantic mutation with identical revision/status/digest fields invalidates
	// the old approval; no naive revision-only comparison can keep it current.
	spec.Requirements[0].Acceptance = "Exact contents and labels return"
	if view().EvidenceStatus != evidenceStale {
		t.Fatal("same-revision text edit was accepted", view())
	}
	spec.Requirements[0].Acceptance = "Exact contents return"
	design.ImplementationNotes = "Different persistence mechanism"
	if view().EvidenceStatus != evidenceStale {
		t.Fatal("same-revision design edit was accepted", view())
	}
	design.ImplementationNotes = "Use saved state"
	state.Design.Revision++
	if view().EvidenceStatus != evidenceStale {
		t.Fatal("new sketch retained approved design", view())
	}
	state.Design.Revision--
	source.Available = false
	if view().EvidenceStatus != evidenceUnverified {
		t.Fatal("unreadable current source was treated as evidence", view())
	}
	source.Available = true
	row.Commands[0].Status = "skipped"
	if view().EvidenceStatus != evidenceUnverified {
		t.Fatal("incomplete commands were treated as evidence", view())
	}
	row.Commands[0].Status = "passed"
	copy := snapshotProjectCheck(row)
	copy.RequirementIDs[0], copy.RequirementSnapshots[0].Acceptance, copy.Commands[0].Args[0], *copy.Commands[0].ExitCode = "other", "changed", "changed", 1
	planCopy := currentPlan(state, id)
	planCopy.Tasks[0].RequirementIDs[0] = "other"
	if row.RequirementIDs[0] != "AC-save" || row.RequirementSnapshots[0].Acceptance != "Exact contents return" || row.Commands[0].Args[0] != "test" || *row.Commands[0].ExitCode != 0 || state.Plan.Tasks[0].RequirementIDs[0] != "AC-save" {
		t.Fatal("HTTP snapshot shared mutable evidence")
	}
	row.RequirementIDs, row.RequirementSnapshots = nil, nil
	if view().EvidenceStatus != evidenceUnverified {
		t.Fatal("old unbound history was retroactively verified")
	}
}

func TestEvidenceReadRefreshesConcurrentApprovalsAndFailsClosedDuringCheckout(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(s.guardProjectSourceRoutes(mux))
	p := createJourneyProject(t, mux, "concurrent-evidence")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	useActualEvidenceProcesses(t, s)
	base := "/v1/product/projects/" + p.ID
	var run ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "-e", "process.exit(0)"), 202).Body.Bytes(), &run)
	finishEvidenceRun(t, s, p.ID, run.ID)
	if getEvidence(t, mux, p.ID).Runs[0].EvidenceStatus != evidenceLinked {
		t.Fatal("fixture did not establish native linked evidence")
	}
	// The source-reading request must not hold s.mu while Git runs. A normal
	// saved definition can advance concurrently and the returned coverage must
	// refresh its approval instead of using the pre-hash snapshot.
	original := s.runner
	started, unblock := make(chan struct{}), make(chan struct{})
	var block, release sync.Once
	defer release.Do(func() { close(unblock) })
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		for _, arg := range args {
			if name == "git" && arg == "ls-files" {
				block.Do(func() {
					close(started)
					select {
					case <-unblock:
					case <-ctx.Done():
					}
				})
				break
			}
		}
		return original(ctx, name, args, dir)
	}
	read := make(chan projectEvidenceResponse, 1)
	readError := make(chan error, 1)
	go func() {
		coverage, rows, err := s.readProjectEvidence(context.Background(), p.ID, p.Path)
		if err != nil {
			readError <- err
			return
		}
		read <- projectEvidenceResponse{Coverage: coverage, Runs: rows}
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("native source capture never started")
	}
	spec.Requirements[0].Acceptance = "Changed while the coverage source was being read"
	saved := make(chan int, 1)
	go func() {
		body, _ := json.Marshal(map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", base+"/specification", strings.NewReader(string(body))))
		saved <- w.Code
	}()
	select {
	case status := <-saved:
		if status != 200 {
			t.Fatal("concurrent definition save failed", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native hashing held the shared journey mutex")
	}
	release.Do(func() { close(unblock) })
	select {
	case value := <-read:
		if value.Coverage.Specification.Status != "draft" || value.Runs[0].EvidenceStatus != evidenceStale || value.Coverage.Rows[0].Acceptance != spec.Requirements[0].Acceptance {
			t.Fatal("coverage retained superseded approval", value)
		}
	case err := <-readError:
		t.Fatal(err)
	case <-time.After(20 * time.Second):
		t.Fatal("native evidence capture did not finish")
	}
	// GET run history remains readable while checkout has exclusive admission,
	// but cannot claim to know the changing source. POST cannot reserve a run.
	guard := s.branchGuard(p.ID)
	guard.Lock()
	defer guard.Unlock()
	before, err := os.ReadFile(s.checkHistoryPath(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	var busy ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/checks/"+run.ID, nil, 200).Body.Bytes(), &busy)
	if busy.EvidenceStatus == evidenceLinked || !strings.Contains(strings.Join(busy.StaleReasons, " "), "could not be read safely") {
		t.Fatal("exclusive checkout guessed current source", busy)
	}
	journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(spec, design, "-e", "process.exit(0)"), 409)
	after, err := os.ReadFile(s.checkHistoryPath(p.ID))
	if err != nil || string(before) != string(after) {
		t.Fatal("read-only busy reconciliation changed check history", err)
	}
	if len(s.projectChecks) != 0 {
		t.Fatal("busy admission allocated a check")
	}
}
