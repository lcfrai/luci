package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

type journeyContextExport struct {
	Path string
	Data []byte
}

func journeyContextExports(state journeyState) ([]journeyContextExport, error) {
	exports := []journeyContextExport{}
	addJSON := func(path string, value any) error {
		var data []byte
		var err error
		if path == ".luci/design.json" {
			// Raw scene JSON stays compact: pretty-printing user nesting can
			// amplify a bounded request into a much larger allocation.
			data, err = json.Marshal(value)
		} else {
			data, err = json.MarshalIndent(value, "", "  ")
		}
		if err != nil {
			return err
		}
		if path == ".luci/plan.json" && len(data)+1 > projectPlanExportBytesLimit {
			return errProjectPlanTooLarge
		}
		exports = append(exports, journeyContextExport{path, append(data, '\n')})
		return nil
	}
	if state.Brief.Revision > 0 {
		exports = append(exports, journeyContextExport{"LUCI.md", renderBrief(state.Brief)})
	}
	if err := addJSON(".luci/design.json", state.Design); err != nil {
		return nil, err
	}
	if state.Plan != nil {
		if err := addJSON(".luci/plan.json", state.Plan); err != nil {
			return nil, err
		}
	}
	if state.Specification != nil {
		if err := addJSON(".luci/specification.json", currentSpecification(state)); err != nil {
			return nil, err
		}
		exports = append(exports, journeyContextExport{".luci/specification.md", renderSpecification(currentSpecification(state))})
	}
	if state.DesignPlan != nil {
		if err := addJSON(".luci/design-handoff.json", currentDesignPlan(state)); err != nil {
			return nil, err
		}
		exports = append(exports, journeyContextExport{".luci/design-handoff.md", renderDesignPlan(currentDesignPlan(state))})
	}
	return exports, nil
}
func validateJourneyExportTargets(dir string, exports []journeyContextExport) error {
	for _, item := range exports {
		if _, err := guardedProjectPath(dir, item.Path, true); err != nil {
			return err
		}
	}
	return nil
}

type journeyContextExportStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type journeyContextStatus struct {
	ContextID           string                       `json:"contextId"`
	ProjectID           string                       `json:"projectId"`
	Imported            bool                         `json:"imported"`
	ExpectedDigest      string                       `json:"expectedDigest"`
	Exports             []journeyContextExportStatus `json:"exports"`
	ArchiveHead         string                       `json:"archiveHead"`
	ArchiveChunks       int                          `json:"archiveChunks"`
	ArchiveBytes        int                          `json:"archiveBytes"`
	ArchiveRevisions    int                          `json:"archiveRevisions"`
	UnusedArchiveChunks int                          `json:"unusedArchiveChunks"`
	UnusedArchiveBytes  int64                        `json:"unusedArchiveBytes"`
	ArchiveError        string                       `json:"archiveError,omitempty"`
	ArchiveBytesLimit   int                          `json:"archiveBytesLimit"`
	ArchiveChunksLimit  int                          `json:"archiveChunksLimit"`
	RecentRevisions     int                          `json:"recentRevisions"`
}

// Bind cleanup to the physical archive set as well as canonical revisions.
// A new orphan after backup must invalidate permission to remove archives.
func journeyContextDigest(dir string, state journeyState) (string, error) {
	inventory, _, err := journeyArchiveInventory(dir)
	if err != nil {
		return "", err
	}
	return reviewDigest(struct {
		State    journeyState     `json:"state"`
		Archives map[string]int64 `json:"archives"`
	}{state, inventory}), nil
}
func inspectJourneyContext(dir string, state journeyState) journeyContextStatus {
	result := journeyContextStatus{ContextID: state.ContextID, ProjectID: state.Brief.ProjectID, Imported: state.importedContext, ExpectedDigest: "", Exports: []journeyContextExportStatus{}, ArchiveHead: state.HistoryHead, ArchiveBytesLimit: journeyArchiveBytesLimit, ArchiveChunksLimit: journeyArchiveChunksLimit, RecentRevisions: len(state.BriefRevisions) + len(state.DesignRevisions) + len(state.SpecificationRevisions) + len(state.DesignPlanRevisions)}
	result.ExpectedDigest, _ = journeyContextDigest(dir, state)
	exports, err := journeyContextExports(state)
	if err != nil {
		return result
	}
	for _, item := range exports {
		status := "current"
		path, err := guardedProjectPath(dir, item.Path, false)
		if errors.Is(err, os.ErrNotExist) {
			status = "missing"
		} else if err != nil {
			status = "unavailable"
		} else {
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Size() > journeyStateLimit {
				status = "unavailable"
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					status = "unavailable"
				} else if !bytes.Equal(data, item.Data) {
					status = "changed"
				}
			}
		}
		result.Exports = append(result.Exports, journeyContextExportStatus{Path: item.Path, Status: status})
	}
	err = walkJourneyArchive(dir, state.ContextID, state.HistoryHead, func(record journeyArchiveRecord) {
		result.ArchiveChunks++
		result.ArchiveBytes += len(record.Data)
		result.ArchiveRevisions += journeyArchiveCount(record.Chunk)
	})
	if err != nil {
		result.ArchiveError = err.Error()
	}
	inventory, diskBytes, inventoryErr := journeyArchiveInventory(dir)
	if inventoryErr != nil && result.ArchiveError == "" {
		result.ArchiveError = inventoryErr.Error()
	}
	if inventoryErr == nil && err == nil {
		result.UnusedArchiveChunks = len(inventory) - result.ArchiveChunks
		result.UnusedArchiveBytes = diskBytes - int64(result.ArchiveBytes)
	}
	return result
}
func (s *Server) registerJourneyContext(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/context", s.getJourneyContext)
	mux.HandleFunc("GET /v1/product/projects/{id}/context/archive", s.getJourneyArchive)
	mux.HandleFunc("POST /v1/product/projects/{id}/context/exports/repair", s.repairJourneyExports)
	mux.HandleFunc("POST /v1/product/projects/{id}/context/archive/prune", s.pruneJourneyArchive)
}
func (s *Server) getJourneyContext(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "context_unavailable", "Saved context could not be read.")
		return
	}
	journeyJSON(w, 200, inspectJourneyContext(p.Path, state))
}
func (s *Server) getJourneyArchive(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "context_unavailable", "Saved context could not be read.")
		return
	}
	cursor := r.URL.Query().Get("cursor")
	if cursor == "" {
		cursor = state.HistoryHead
	}
	var page *journeyArchiveRecord
	err = walkJourneyArchive(p.Path, state.ContextID, state.HistoryHead, func(record journeyArchiveRecord) {
		if record.Digest == cursor {
			page = &record
		}
	})
	if err != nil {
		problem(w, 409, "archive_unavailable", err.Error())
		return
	}
	if page != nil {
		// Public historical claims confer no current approval or private identity.
		journeyJSON(w, 200, map[string]any{"digest": page.Digest, "nextCursor": page.Chunk.Previous, "chunk": page.Chunk})
		return
	}
	if cursor == "" {
		journeyJSON(w, 200, map[string]any{"digest": "", "nextCursor": "", "chunk": nil})
		return
	}
	problem(w, 404, "archive_missing", "This revision page is not part of the current project's context archive.")
}
func (s *Server) repairJourneyExports(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedDigest string `json:"expectedDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "context_unavailable", "Saved context could not be read.")
		return
	}
	digest, digestErr := journeyContextDigest(p.Path, state)
	if digestErr != nil || !hexDigest(req.ExpectedDigest) || req.ExpectedDigest != digest {
		problem(w, 409, "context_conflict", "Context changed. Reload before repairing exports.")
		return
	}
	exports, err := journeyContextExports(state)
	if err == nil {
		err = validateJourneyExportTargets(p.Path, exports)
	}
	if err == nil {
		err = writeJourneyContext(p.Path, state)
	}
	if err != nil {
		problem(w, 409, "export_repair_failed", "Generated context exports could not all be repaired. Check project file permissions and retry; saved context is unchanged.")
		return
	}
	journeyJSON(w, 200, inspectJourneyContext(p.Path, state))
}
func (s *Server) pruneJourneyArchive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedDigest                    string `json:"expectedDigest"`
		AcknowledgeArchivedHistoryRemoval bool   `json:"acknowledgeArchivedHistoryRemoval"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.AcknowledgeArchivedHistoryRemoval {
		problem(w, 400, "acknowledgement_required", "Export a project backup first, then acknowledge removal of archived revisions. Current and recent revisions are retained.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "context_unavailable", "Saved context could not be read.")
		return
	}
	digest, digestErr := journeyContextDigest(p.Path, state)
	if digestErr != nil || !hexDigest(req.ExpectedDigest) || req.ExpectedDigest != digest {
		problem(w, 409, "context_conflict", "Context changed. Reload before removing archived revisions.")
		return
	}
	records, err := readJourneyArchive(p.Path, state.ContextID, state.HistoryHead)
	if err != nil {
		problem(w, 409, "archive_unavailable", err.Error())
		return
	}
	// Validate unused chunks too. A crash after severing the head or before
	// attaching a new chunk can leave these; explicit acknowledged pruning is
	// the only operation that removes them.
	inventory, _, err := journeyArchiveInventory(p.Path)
	if err != nil {
		problem(w, 409, "archive_unavailable", err.Error())
		return
	}
	if _, err = readUnusedJourneyArchives(p.Path, state.ContextID, records); err != nil {
		problem(w, 409, "archive_unavailable", err.Error())
		return
	}
	if len(inventory) == 0 {
		journeyJSON(w, 200, inspectJourneyContext(p.Path, state))
		return
	}
	state.HistoryHead = ""
	// The durable head is severed before removing files; interruptions can leave
	// unused chunks, never a canonical pointer to a deleted chunk.
	if err = writeJourney(p.Path, state); err != nil {
		problem(w, 500, "archive_prune_failed", "Could not finish archiving changes; reload Context storage before retrying.")
		return
	}
	for digest := range inventory {
		path, err := guardedProjectPath(p.Path, journeyArchiveFile(digest), false)
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			problem(w, 409, "archive_cleanup_incomplete", "Archived revisions were detached, but some unused files could not be removed. Reload Context storage and retry cleanup.")
			return
		}
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "context_unavailable", "Archived files were removed, but refreshed context could not be read.")
		return
	}
	journeyJSON(w, 200, inspectJourneyContext(p.Path, state))
}
