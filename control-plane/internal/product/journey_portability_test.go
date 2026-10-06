package product

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestPortableJourneyRealCloneRetainsLegacyContextWithoutReviewAuthority(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "original")
	approveEvidenceJourney(t, mux, p.ID, "AC-save")
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || !specificationApprovalMatches(state) || !designApprovalMatches(state) {
		t.Fatalf("source approval: %v", err)
	}
	// Legacy v1 files have no portable identity field. Both original and clone
	// must derive it from the saved source ID, never from their destination path.
	state.Version = 1
	state.ContextID = ""
	data, _ := json.MarshalIndent(state, "", "  ")
	if err = os.WriteFile(filepath.Join(p.Path, ".luci/state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/commit", map[string]string{"message": "reviewed legacy project"}, 200)
	cloneDir := filepath.Join(s.projectRoot(), "second-folder")
	command := exec.Command("git", "-c", "protocol.file.allow=always", "clone", "--", p.Path, cloneDir)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("real clone: %v %s", err, out)
	}
	cloneID := project.ProjectID(cloneDir)
	before, err := os.ReadFile(filepath.Join(cloneDir, ".luci/state.json"))
	if err != nil {
		t.Fatal(err)
	}
	imported, err := s.readApprovedJourney(cloneDir, cloneID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ContextID != original.ContextID || !imported.importedContext || imported.Brief.ProjectID != cloneID || imported.DesignPlan.ProjectID != cloneID {
		t.Fatal("portable lineage or local binding lost")
	}
	if imported.Specification.Status != "draft" || imported.DesignPlan.Status != "draft" || specificationApprovalMatches(imported) || designApprovalMatches(imported) {
		t.Fatal("public clone transferred review authority")
	}
	if imported.Plan != nil && imported.Plan.SpecificationDigest != "" {
		t.Fatal("old plan review binding survived import")
	}
	after, _ := os.ReadFile(filepath.Join(cloneDir, ".luci/state.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("reading clone rewrote project files")
	}
	base := "/v1/product/projects/" + cloneID
	journeyRequest(t, mux, "GET", base+"/specification", nil, 200)
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": imported.Brief.Revision, "purpose": "Continue cloned work"}, 200)
	updated, err := readJourney(cloneDir, cloneID)
	if err != nil || updated.ContextID != original.ContextID || updated.importedContext || updated.Brief.Revision != imported.Brief.Revision+1 {
		t.Fatalf("clone cannot continue editing: %+v %v", updated.Brief, err)
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": imported.Brief.Revision, "purpose": "Stale writer"}, 409)
}

func TestPortableJourneySubstantialScenesPreserveHistoryAcrossBackupRestore(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "large-design")
	base := "/v1/product/projects/" + p.ID
	scene := json.RawMessage(`{"elements":[],"appState":{"name":"` + strings.Repeat("design", 220000) + `"}}`)
	for revision := 0; revision < 12; revision++ {
		journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": revision, "frames": []Frame{}, "scene": scene}, 200)
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(p.Path, ".luci/state.json"))
	if err != nil || info.Size() > journeyStateLimit {
		t.Fatalf("canonical size: %v %v", info, err)
	}
	records, err := readJourneyArchive(p.Path, state.ContextID, state.HistoryHead)
	if err != nil || len(records) == 0 {
		t.Fatalf("missing preserved history: %v", err)
	}
	revisions := map[int]bool{}
	for _, row := range state.DesignRevisions {
		revisions[row.Revision] = true
	}
	for _, record := range records {
		for _, row := range record.Chunk.DesignRevisions {
			if revisions[row.Revision] {
				t.Fatal("duplicate archived revision")
			}
			revisions[row.Revision] = true
			var value any
			if json.Unmarshal(row.Scene, &value) != nil {
				t.Fatal("archived scene damaged")
			}
		}
	}
	if len(revisions) != 12 || !revisions[1] || !revisions[12] {
		t.Fatalf("lost revisions: %v", revisions)
	}
	// The ordinary one-click Git action accepts only verified reachable chunks.
	journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "preserve large-scene history"}, 200)
	archive := journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	restoredResponse := backupUpload(t, mux, "large-restored", archive.Body.Bytes(), 201)
	var restored project.Project
	decodeEvidenceResponse(t, restoredResponse.Body.Bytes(), &restored)
	restoredState, err := readJourney(restored.Path, restored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restoredState.ContextID != state.ContextID || restoredState.HistoryHead != state.HistoryHead || restoredState.Design.Revision != 12 || restoredState.Design.ProjectID != restored.ID {
		t.Fatal("restored context lost lineage, content or local binding")
	}
	restoredRecords, err := readJourneyArchive(restored.Path, restoredState.ContextID, restoredState.HistoryHead)
	if err != nil || len(restoredRecords) != len(records) {
		t.Fatalf("restored archive: %v", err)
	}
	for i := range records {
		if !bytes.Equal(records[i].Data, restoredRecords[i].Data) {
			t.Fatal("restore rewrote immutable history")
		}
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+restored.ID+"/design", map[string]any{"revision": 12, "frames": []Frame{}, "scene": scene}, 200)
	// Corruption stops new saves and backups without erasing the latest context.
	headPath := filepath.Join(p.Path, filepath.FromSlash(journeyArchiveFile(state.HistoryHead)))
	if err = os.WriteFile(headPath, []byte(`{"corrupt":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 12, "frames": []Frame{}}, 500)
	journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 422)
	still, err := readJourney(p.Path, p.ID)
	if err != nil || still.Design.Revision != 12 {
		t.Fatal("archive corruption erased latest context")
	}
}

func TestPortableJourneyCapacityRequiresExplicitBackupAndPrune(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "capacity")
	base := "/v1/product/projects/" + p.ID
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(p.Path, ".luci/history"), 0700); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= journeyArchiveChunksLimit; i++ {
		chunk := journeyArchiveChunk{Version: 1, ContextID: state.ContextID, ProjectID: p.ID, Previous: state.HistoryHead, BriefRevisions: []Brief{{ProjectID: p.ID, Revision: i, Purpose: "retained"}}}
		data, _ := json.Marshal(chunk)
		head := journeyArchiveDigest(data)
		if err = writeProjectAtomic(p.Path, journeyArchiveFile(head), data); err != nil {
			t.Fatal(err)
		}
		state.HistoryHead = head
	}
	for i := 1; i <= journeyRecentRevisions; i++ {
		state.Brief = Brief{ProjectID: p.ID, Revision: journeyArchiveChunksLimit + i, Purpose: "Recent work"}
		state.BriefRevisions = append(state.BriefRevisions, state.Brief)
	}
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	failed := journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": state.Brief.Revision, "purpose": "Next edit"}, 500)
	if !strings.Contains(failed.Body.String(), "export a project backup") {
		t.Fatalf("capacity failure offers no recovery: %s", failed.Body.String())
	}
	after, _ := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("full archive partially changed canonical state")
	}
	var status journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &status)
	journeyRequest(t, mux, "POST", base+"/context/archive/prune", map[string]any{"expectedDigest": status.ExpectedDigest}, 400)
	journeyRequest(t, mux, "POST", base+"/context/archive/prune", map[string]any{"expectedDigest": strings.Repeat("a", 64), "acknowledgeArchivedHistoryRemoval": true}, 409)
	s.registerBackups(mux)
	backup := journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	if backup.Body.Len() == 0 {
		t.Fatal("capacity blocked export")
	}
	journeyRequest(t, mux, "POST", base+"/context/archive/prune", map[string]any{"expectedDigest": status.ExpectedDigest, "acknowledgeArchivedHistoryRemoval": true}, 200)
	retained, err := readJourney(p.Path, p.ID)
	if err != nil || retained.HistoryHead != "" || retained.Brief.Revision != state.Brief.Revision || len(retained.BriefRevisions) != journeyRecentRevisions {
		t.Fatal("prune changed current/recent work")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": state.Brief.Revision, "purpose": "Next edit"}, 200)
	// The downloaded backup still restores the removed archive in full.
	response := backupUpload(t, mux, "capacity-recovered", backup.Body.Bytes(), 201)
	var restored project.Project
	decodeEvidenceResponse(t, response.Body.Bytes(), &restored)
	recovered, err := readJourney(restored.Path, restored.ID)
	if err != nil {
		t.Fatal(err)
	}
	records, err := readJourneyArchive(restored.Path, recovered.ContextID, recovered.HistoryHead)
	if err != nil || len(records) != journeyArchiveChunksLimit {
		t.Fatalf("pruned history wasn't recoverable: %v", err)
	}
}

func TestPortableJourneyExportRepairIsExplicitAndRejectsStaleWriters(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "repair")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "First"}, 200)
	var initial journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &initial)
	target := filepath.Join(p.Path, ".luci/design.json")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	failed := journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 1, "purpose": "Durable despite partial export"}, 500)
	if !strings.Contains(failed.Body.String(), "context was saved") {
		t.Fatalf("ambiguous save outcome: %s", failed.Body.String())
	}
	canonical, err := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &status)
	for _, item := range status.Exports {
		if item.Path == ".luci/design.json" && item.Status != "unavailable" {
			t.Fatal("partial export not detected")
		}
	}
	if info, _ := os.Stat(target); info == nil || !info.IsDir() {
		t.Fatal("GET repaired exports")
	}
	journeyRequest(t, mux, "POST", base+"/context/exports/repair", map[string]string{"expectedDigest": initial.ExpectedDigest}, 409)
	if err = os.Remove(target); err != nil {
		t.Fatal(err)
	}
	repaired := journeyRequest(t, mux, "POST", base+"/context/exports/repair", map[string]string{"expectedDigest": status.ExpectedDigest}, 200)
	decodeEvidenceResponse(t, repaired.Body.Bytes(), &status)
	for _, item := range status.Exports {
		if item.Status != "current" {
			t.Fatalf("not repaired: %+v", item)
		}
	}
	after, _ := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	if !bytes.Equal(canonical, after) {
		t.Fatal("export repair changed canonical context")
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || state.Brief.Revision != 2 || state.approvalAuthority != nil {
		t.Fatal("repair changed revision or granted approval")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 1, "purpose": "Stale retry"}, 409)
	// Keep the request path exercised through real Git without allocating agents.
	if _, err = s.journeyGit(context.Background(), p.Path, "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
}

func TestPortableJourneyBackupRejectsInvalidArchiveChainsBeforeCreatingProject(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	for _, kind := range []string{"missing-parent", "wrong-hash", "foreign-context", "unreferenced", "mixed-project"} {
		t.Run(kind, func(t *testing.T) {
			id := "test"
			contextID := journeyContextIdentity(id)
			chunk := journeyArchiveChunk{Version: 1, ContextID: contextID, ProjectID: id, BriefRevisions: []Brief{{ProjectID: id, Revision: 1, Purpose: "Archived"}}}
			if kind == "missing-parent" {
				chunk.Previous = strings.Repeat("e", 64)
			}
			if kind == "foreign-context" {
				chunk.ContextID = journeyContextIdentity("other")
			}
			if kind == "mixed-project" {
				chunk.BriefRevisions[0].ProjectID = "other"
			}
			data, _ := json.Marshal(chunk)
			head := journeyArchiveDigest(data)
			if kind == "wrong-hash" {
				data = append(data, ' ')
			}
			state := journeyState{Version: 1, ContextID: contextID, HistoryHead: head, Brief: Brief{ProjectID: id, Revision: 2}, Design: Design{ProjectID: id}}
			if kind == "unreferenced" {
				state.HistoryHead = ""
			}
			canonical, _ := json.Marshal(state)
			backup := craftedBackup(t, map[string][]byte{"journey.json": canonical, "context-history/" + head + ".json": data}, nil, nil)
			name := "invalid-" + kind
			backupUpload(t, mux, name, backup, 400)
			if _, err := os.Stat(filepath.Join(s.projectRoot(), name)); !os.IsNotExist(err) {
				t.Fatal("invalid archive created a project folder")
			}
		})
	}
}

func TestPortableJourneyInterruptedArchiveIsBoundedBackedUpAndExplicitlyRemoved(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "interrupted")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Saved"}, 200)
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Model a crash after writing the immutable chunk, before publishing its
	// canonical head. The old canonical history must remain authoritative.
	chunk := journeyArchiveChunk{Version: 1, ContextID: state.ContextID, ProjectID: p.ID, BriefRevisions: []Brief{{ProjectID: p.ID, Revision: 1, Purpose: "Saved"}}}
	data, _ := json.Marshal(chunk)
	head := journeyArchiveDigest(data)
	if err = os.MkdirAll(filepath.Join(p.Path, ".luci/history"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = writeProjectAtomic(p.Path, journeyArchiveFile(head), data); err != nil {
		t.Fatal(err)
	}
	var status journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &status)
	if status.ArchiveChunks != 0 || status.UnusedArchiveChunks != 1 || status.UnusedArchiveBytes != int64(len(data)) {
		t.Fatalf("orphan was adopted or hidden: %+v", status)
	}
	journeyRequest(t, mux, "GET", base+"/context/archive?cursor="+head, nil, 404)
	archive := journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	journeyRequest(t, mux, "POST", base+"/context/archive/prune", map[string]any{"expectedDigest": status.ExpectedDigest, "acknowledgeArchivedHistoryRemoval": true}, 200)
	if _, err = os.Stat(filepath.Join(p.Path, filepath.FromSlash(journeyArchiveFile(head)))); !os.IsNotExist(err) {
		t.Fatal("explicit orphan cleanup did not remove archive")
	}
	response := backupUpload(t, mux, "interrupted-restored", archive.Body.Bytes(), 201)
	var restored project.Project
	decodeEvidenceResponse(t, response.Body.Bytes(), &restored)
	restoredState, err := readJourney(restored.Path, restored.ID)
	if err != nil {
		t.Fatal(err)
	}
	restoredStatus := inspectJourneyContext(restored.Path, restoredState)
	if restoredStatus.ArchiveChunks != 0 || restoredStatus.UnusedArchiveChunks != 1 || restoredStatus.ContextID != state.ContextID {
		t.Fatalf("orphan backup changed attached history: %+v", restoredStatus)
	}
	actual, err := os.ReadFile(filepath.Join(restored.Path, filepath.FromSlash(journeyArchiveFile(head))))
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("backup lost interrupted archive")
	}
}

func TestPortableJourneyPruneRejectsOrphanCreatedAfterBackup(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "backup-race")
	base := "/v1/product/projects/" + p.ID
	for revision := 0; revision < 9; revision++ {
		journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": revision, "purpose": "Retain work"}, 200)
	}
	var backedUp journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &backedUp)
	journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Another attempted save preserves a different chunk, then crashes before
	// changing state.json. It was not in the already downloaded backup.
	chunk := journeyArchiveChunk{Version: 1, ContextID: state.ContextID, ProjectID: p.ID, Previous: state.HistoryHead, BriefRevisions: []Brief{{ProjectID: p.ID, Revision: 2, Purpose: "Interrupted work"}}}
	data, _ := json.Marshal(chunk)
	head := journeyArchiveDigest(data)
	if err = writeProjectAtomic(p.Path, journeyArchiveFile(head), data); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/context/archive/prune", map[string]any{"expectedDigest": backedUp.ExpectedDigest, "acknowledgeArchivedHistoryRemoval": true}, 409)
	if _, err = os.Stat(filepath.Join(p.Path, filepath.FromSlash(journeyArchiveFile(head)))); err != nil {
		t.Fatal("stale cleanup removed newly preserved work")
	}
	var now journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &now)
	if now.ExpectedDigest == backedUp.ExpectedDigest || now.UnusedArchiveChunks != 1 {
		t.Fatal("storage CAS omitted interrupted archive")
	}
}

func TestPortableJourneyRejectsSceneIndentationExpansionOnSaveAndImport(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "nested-scene")
	base := "/v1/product/projects/" + p.ID
	scene := json.RawMessage(`{"elements":[],"appState":{"deep":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}}`)
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 0, "frames": []Frame{}, "scene": scene}, 400)
	state, err := readJourney(p.Path, p.ID)
	if err != nil || state.Design.Revision != 0 {
		t.Fatal("rejected scene changed saved revision")
	}
	state.Design.Scene = scene
	if err = os.MkdirAll(filepath.Join(p.Path, ".luci"), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(state)
	if err = os.WriteFile(filepath.Join(p.Path, ".luci/state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readJourney(p.Path, p.ID); err == nil {
		t.Fatal("imported scene bypassed safe serialization limit")
	}
	// Brackets and escaped quotes in ordinary text are not structural nesting.
	safe, _ := json.Marshal(map[string]any{"elements": []any{}, "appState": map[string]string{"name": strings.Repeat(`[\"]`, 100)}})
	if err = validateScene(safe); err != nil {
		t.Fatalf("normal scene text rejected: %v", err)
	}
}

func TestPortableJourneyReadsLegacyIndentedSceneByContentSize(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "legacy-scene")
	element := `{"type":"line","points":[[1,2],[3,4],[5,6],[7,8],[9,10]]}`
	scene := json.RawMessage(`{"elements":[` + strings.TrimSuffix(strings.Repeat(element+",", 5000), ",") + `]}`)
	if err := validateScene(scene); err != nil {
		t.Fatal(err)
	}
	state := journeyState{Version: 1, Brief: Brief{ProjectID: p.ID}, Design: Design{ProjectID: p.ID, Revision: 1, Scene: scene}}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var stored journeyState
	if err = json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Design.Scene) <= 1536<<10 {
		t.Fatal("fixture did not exercise legacy indentation amplification")
	}
	if err = os.MkdirAll(filepath.Join(p.Path, ".luci"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Path, ".luci/state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatalf("legacy valid scene became unreadable: %v", err)
	}
	if err = writeJourney(p.Path, loaded); err != nil {
		t.Fatal(err)
	}
	compact, err := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(compact) > journeyActiveTarget {
		t.Fatal("canonical save retained indentation amplification")
	}
	if _, err = readJourney(p.Path, p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPortableJourneyRecoversPreRenameCrashOnlyOnExplicitSave(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "staging-crash")
	base := "/v1/product/projects/" + p.ID
	for revision := 0; revision < 9; revision++ {
		journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": revision, "purpose": "Preserved work"}, 200)
	}
	before, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(p.Path, ".luci", "history", ".luci-write-123456789")
	if err := os.WriteFile(staged, []byte(`{"interrupted":`), 0600); err != nil {
		t.Fatal(err)
	}
	var status journeyContextStatus
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/context", nil, 200).Body.Bytes(), &status)
	if status.ArchiveError == "" {
		t.Fatal("incomplete storage was hidden")
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatal("GET discarded interrupted staging")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 8, "purpose": "Stale work"}, 409)
	if _, err := os.Stat(staged); err != nil {
		t.Fatal("stale writer mutated storage")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 9, "purpose": "Continue after crash"}, 200)
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatal("explicit save did not recover staging")
	}
	after, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Brief.Revision != 10 {
		t.Fatal("save lost current revision")
	}
	if _, err := readJourneyArchiveData(p.Path, before.HistoryHead); err != nil {
		t.Fatal("recovery removed immutable accepted history")
	}
	journeyRequest(t, mux, "POST", base+"/backups", map[string]any{}, 200)
	outside := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, staged); err == nil {
		if err := discardJourneyArchiveStaging(p.Path); err == nil {
			t.Fatal("unsafe staging accepted")
		}
		data, err := os.ReadFile(outside)
		if err != nil || string(data) != "keep" {
			t.Fatal("unsafe recovery changed external file")
		}
	}
}
