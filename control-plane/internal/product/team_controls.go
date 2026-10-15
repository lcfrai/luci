package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

const controlLedgerLimit = 8 << 20
const controlRevisionLimit = 16

var controlIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
var controlFields = []string{"id", "requirement", "category", "applicability", "owner", "evidenceExpected", "severity", "reviewCadence"}

type TeamControl struct {
	ID               string            `json:"id"`
	Requirement      string            `json:"requirement"`
	Category         string            `json:"category"`
	Applicability    string            `json:"applicability"`
	Owner            string            `json:"owner"`
	EvidenceExpected string            `json:"evidenceExpected"`
	Severity         string            `json:"severity,omitempty"`
	ReviewCadence    string            `json:"reviewCadence,omitempty"`
	Sheet            string            `json:"sheet"`
	Row              int               `json:"row"`
	Unmapped         map[string]string `json:"unmapped"`
	UnmappedHeaders  map[string]string `json:"unmappedHeaders"`
}
type TeamControlRevision struct {
	Revision     int               `json:"revision"`
	Digest       string            `json:"digest"`
	SourceSHA256 string            `json:"sourceSHA256"`
	Sheet        string            `json:"sheet"`
	Columns      map[string]string `json:"columns"`
	Controls     []TeamControl     `json:"controls"`
	Added        []string          `json:"added"`
	Changed      []string          `json:"changed"`
	Removed      []string          `json:"removed"`
	DiffDigest   string            `json:"diffDigest"`
	ActorID      string            `json:"actorId"`
	At           string            `json:"at"`
	KeyDigest    string            `json:"keyDigest"`
}
type teamControlLedger struct {
	Version   int                   `json:"version"`
	TeamID    string                `json:"teamId"`
	Revisions []TeamControlRevision `json:"revisions"`
}
type teamControlPreview struct {
	SourceSHA256     string            `json:"sourceSHA256"`
	Workbook         controlWorkbook   `json:"workbook"`
	SuggestedColumns map[string]string `json:"suggestedColumns"`
}
type teamControlImport struct {
	ExpectedRevision   int               `json:"expectedRevision"`
	IdempotencyKey     string            `json:"idempotencyKey"`
	SourceSHA256       string            `json:"sourceSHA256"`
	Sheet              string            `json:"sheet"`
	Columns            map[string]string `json:"columns"`
	WorkbookBase64     string            `json:"workbookBase64"`
	ExpectedDiffDigest string            `json:"expectedDiffDigest"`
}

func (s *Server) registerDistributeControls(mux Registrar) {
	mux.HandleFunc("GET /v1/product/team-controls", s.getTeamControls)
	mux.HandleFunc("POST /v1/product/team-controls/preview", s.previewTeamControls)
	mux.HandleFunc("POST /v1/product/team-controls/compare", s.compareTeamControls)
	mux.HandleFunc("POST /v1/product/team-controls/import", s.importTeamControls)
	mux.HandleFunc("GET /v1/product/projects/{id}/control-assessment", s.getControlAssessment)
	mux.HandleFunc("GET /v1/product/projects/{id}/control-assessment/report.xlsx", s.exportControlAssessment)
	mux.HandleFunc("POST /v1/product/projects/{id}/control-assessment", s.saveControlAssessment)
	mux.HandleFunc("POST /v1/product/projects/{id}/control-assessment/gaps/{controlId}/proposal", s.proposeControlGap)
	mux.HandleFunc("POST /v1/product/projects/{id}/control-assessment/gaps/{controlId}/proposal/{proposalId}/apply", s.applyControlGap)
}

func teamControlAccess(w http.ResponseWriter, r *http.Request, admin bool) (management.TeamAccess, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	if _, ok := authenticatedContextActor(r.Context()); !ok {
		journeyError(w, 401, "authentication_required", "Sign in to use team controls.")
		return management.TeamAccess{}, false
	}
	access, ok := management.TeamAccessFromContext(r.Context())
	if !ok {
		journeyError(w, 409, "team_not_configured", "Choose a team workspace to use a shared control library.")
		return access, false
	}
	if !access.Permissions.Read || admin && !access.Permissions.Admin {
		journeyError(w, 403, "team_access_denied", "Current team administration is required for this change.")
		return access, false
	}
	return access, true
}
func teamControlPath(s *Server, teamID string) (string, error) {
	if teamID == "" || len(teamID) > 256 {
		return "", errControlWorkbook
	}
	return guardedProjectPath(s.stateDir, "team-controls-"+controlSourceHash([]byte(teamID))+"-v1.json", true)
}
func (s *Server) readTeamControls(teamID string) (string, teamControlLedger, error) {
	path, err := teamControlPath(s, teamID)
	ledger := teamControlLedger{Version: 1, TeamID: teamID, Revisions: []TeamControlRevision{}}
	if err != nil {
		return "", ledger, err
	}
	var saved teamControlLedger
	err = externalReadJSON(path, controlLedgerLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, ledger, nil
	}
	if err != nil || saved.Version != 1 || saved.TeamID != teamID || len(saved.Revisions) > controlRevisionLimit {
		return "", ledger, errControlWorkbook
	}
	for i, revision := range saved.Revisions {
		if revision.Revision != i+1 || len(revision.Controls) > controlRowLimit || revision.Digest != reviewDigest(revision.Controls) || len(revision.SourceSHA256) != 64 || revision.KeyDigest == "" {
			return "", ledger, errControlWorkbook
		}
	}
	return path, saved, nil
}
func writeTeamControls(path string, ledger teamControlLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(data)+1 > controlLedgerLimit {
		return errControlWorkbook
	}
	return durableRunJSON(path, ledger)
}
func latestControls(ledger teamControlLedger) *TeamControlRevision {
	if len(ledger.Revisions) == 0 {
		return nil
	}
	return &ledger.Revisions[len(ledger.Revisions)-1]
}
func controlSourceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Server) getTeamControls(w http.ResponseWriter, r *http.Request) {
	access, ok := teamControlAccess(w, r, false)
	if !ok {
		return
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readTeamControls(access.Team.ID)
	if err != nil {
		journeyError(w, 409, "library_unavailable", "The private team library cannot be verified.")
		return
	}
	current := latestControls(ledger)
	if current == nil {
		journeyJSON(w, 200, map[string]any{"state": "not-configured", "teamId": access.Team.ID, "revision": 0, "library": nil})
		return
	}
	journeyJSON(w, 200, map[string]any{"state": "configured", "teamId": access.Team.ID, "revision": current.Revision, "library": current})
}

func suggestedControlColumns(sheet controlSheet) map[string]string {
	aliases := map[string][]string{"id": {"id", "control id", "requirement id"}, "requirement": {"requirement", "control", "description"}, "category": {"category", "domain"}, "applicability": {"applicability", "applies to"}, "owner": {"owner", "responsible"}, "evidenceExpected": {"evidence expected", "evidence"}, "severity": {"severity", "risk"}, "reviewCadence": {"review cadence", "cadence"}}
	out := map[string]string{}
	for field, names := range aliases {
		matches := map[string]bool{}
		for _, head := range sheet.Headers {
			for _, name := range names {
				if strings.EqualFold(strings.TrimSpace(head.Name), name) {
					matches[head.Column] = true
				}
			}
		}
		if len(matches) == 1 {
			for col := range matches {
				out[field] = col
			}
		}
	}
	return out
}
func (s *Server) previewTeamControls(w http.ResponseWriter, r *http.Request) {
	if _, ok := teamControlAccess(w, r, true); !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, controlWorkbookLimit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		journeyError(w, 413, "workbook_too_large", "Choose an XLSX under 2 MiB.")
		return
	}
	book, err := parseControlWorkbook(r.Context(), raw)
	if err != nil {
		journeyError(w, 422, "invalid_workbook", err.Error())
		return
	}
	for _, sheet := range book.Sheets {
		guess := suggestedControlColumns(sheet)
		idColumn := guess["id"]
		if idColumn == "" {
			book.Issues = append(book.Issues, "Sheet "+sheet.Name+": map the control ID column before import (missing or ambiguous header).")
		}
		seen := map[string]bool{}
		for _, row := range sheet.Rows {
			if idColumn == "" {
				break
			}
			id := strings.TrimSpace(row.Cells[idColumn])
			if id == "" {
				book.Issues = append(book.Issues, fmt.Sprintf("%s row %d is missing a control ID", sheet.Name, row.Number))
			} else if seen[id] {
				book.Issues = append(book.Issues, fmt.Sprintf("%s row %d duplicates control ID %s", sheet.Name, row.Number, id))
			}
			seen[id] = true
			if len(book.Issues) > 100 {
				break
			}
		}
	}
	// Return at most five sample rows per sheet; the import still reparses every
	// bounded row from the exact bytes supplied by the administrator.
	for i := range book.Sheets {
		if len(book.Sheets[i].Rows) > 5 {
			book.Sheets[i].Rows = book.Sheets[i].Rows[:5]
		}
	}
	suggested := map[string]string{}
	if len(book.Sheets) > 0 {
		suggested = suggestedControlColumns(book.Sheets[0])
	}
	journeyJSON(w, 200, teamControlPreview{SourceSHA256: controlSourceHash(raw), Workbook: book, SuggestedColumns: suggested})
}

func controlsFromWorkbook(book controlWorkbook, sheetName string, columns map[string]string) ([]TeamControl, error) {
	if len(columns) > len(controlFields) {
		return nil, errControlWorkbook
	}
	allowed := map[string]bool{}
	for _, field := range controlFields {
		allowed[field] = true
	}
	used := map[string]bool{}
	for field, col := range columns {
		if !allowed[field] || col == "" || columnIndex(col) > controlColumnLimit || used[col] || !regexp.MustCompile(`^[A-Z]{1,2}$`).MatchString(col) {
			return nil, errControlWorkbook
		}
		used[col] = true
	}
	for _, field := range controlFields[:6] {
		if columns[field] == "" {
			return nil, fmt.Errorf("map the %s column before import", field)
		}
	}
	for _, sheet := range book.Sheets {
		if sheet.Name != sheetName {
			continue
		}
		controls := make([]TeamControl, 0, len(sheet.Rows))
		ids := map[string]bool{}
		for _, row := range sheet.Rows {
			value := func(field string) string { return strings.TrimSpace(row.Cells[columns[field]]) }
			id := value("id")
			if !controlIDPattern.MatchString(id) || ids[id] || value("requirement") == "" {
				return nil, fmt.Errorf("sheet %s row %d has a missing/duplicate ID or requirement", sheetName, row.Number)
			}
			ids[id] = true
			other, headers := map[string]string{}, map[string]string{}
			for col, text := range row.Cells {
				if !used[col] {
					other[col] = text
				}
			}
			for _, head := range sheet.Headers {
				if !used[head.Column] {
					headers[head.Column] = head.Name
				}
			}
			controls = append(controls, TeamControl{ID: id, Requirement: value("requirement"), Category: value("category"), Applicability: value("applicability"), Owner: value("owner"), EvidenceExpected: value("evidenceExpected"), Severity: value("severity"), ReviewCadence: value("reviewCadence"), Sheet: sheetName, Row: row.Number, Unmapped: other, UnmappedHeaders: headers})
		}
		if len(controls) == 0 {
			return nil, fmt.Errorf("the selected sheet has no control rows")
		}
		return controls, nil
	}
	return nil, fmt.Errorf("the selected sheet is unavailable")
}
func controlDiff(previous *TeamControlRevision, next []TeamControl) (added, changed, removed []string) {
	old := map[string]TeamControl{}
	if previous != nil {
		for _, c := range previous.Controls {
			old[c.ID] = c
		}
	}
	seen := map[string]bool{}
	for _, c := range next {
		seen[c.ID] = true
		if before, ok := old[c.ID]; !ok {
			added = append(added, c.ID)
		} else if reviewDigest(before) != reviewDigest(c) {
			changed = append(changed, c.ID)
		}
	}
	for id := range old {
		if !seen[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(changed)
	sort.Strings(removed)
	return
}

func decodeControlImport(w http.ResponseWriter, r *http.Request, req *teamControlImport) ([]TeamControl, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(req) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errControlWorkbook
	}
	if req.ExpectedRevision < 0 || len(req.SourceSHA256) != 64 {
		return nil, errControlWorkbook
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(req.WorkbookBase64)
	if err != nil || len(raw) > controlWorkbookLimit || controlSourceHash(raw) != req.SourceSHA256 {
		return nil, errControlWorkbook
	}
	book, err := parseControlWorkbook(r.Context(), raw)
	if err != nil || len(book.Issues) != 0 {
		return nil, errControlWorkbook
	}
	return controlsFromWorkbook(book, req.Sheet, req.Columns)
}
func controlDiffDigest(source string, columns map[string]string, controls []TeamControl, added, changed, removed []string) string {
	return reviewDigest(struct {
		Source                  string
		Columns                 map[string]string
		Controls                []TeamControl
		Added, Changed, Removed []string
	}{source, columns, controls, added, changed, removed})
}
func (s *Server) compareTeamControls(w http.ResponseWriter, r *http.Request) {
	access, ok := teamControlAccess(w, r, true)
	if !ok {
		return
	}
	var req teamControlImport
	controls, err := decodeControlImport(w, r, &req)
	if err != nil {
		journeyError(w, 422, "invalid_import", "Review a bounded formula-free workbook and map its columns.")
		return
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readTeamControls(access.Team.ID)
	if err != nil || len(ledger.Revisions) != req.ExpectedRevision {
		journeyError(w, 409, "library_changed", "Refresh the team library before reviewing its diff.")
		return
	}
	added, changed, removed := controlDiff(latestControls(ledger), controls)
	journeyJSON(w, 200, map[string]any{"expectedRevision": req.ExpectedRevision, "sourceSHA256": req.SourceSHA256, "sheet": req.Sheet, "columns": req.Columns, "added": added, "changed": changed, "removed": removed, "controlCount": len(controls), "diffDigest": controlDiffDigest(req.SourceSHA256, req.Columns, controls, added, changed, removed)})
}

func (s *Server) importTeamControls(w http.ResponseWriter, r *http.Request) {
	access, ok := teamControlAccess(w, r, true)
	if !ok {
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var req teamControlImport
	controls, err := decodeControlImport(w, r, &req)
	if err != nil {
		journeyError(w, 422, "invalid_import", "Review a bounded formula-free workbook and map its columns.")
		return
	}
	if !validRunKey(req.IdempotencyKey) || len(req.ExpectedDiffDigest) != 64 {
		journeyError(w, 400, "invalid_import", "Use a fresh key and the exact reviewed diff.")
		return
	}
	key := reviewDigest(struct{ Team, Actor, Key string }{access.Team.ID, actor.ID, req.IdempotencyKey})
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	path, ledger, err := s.readTeamControls(access.Team.ID)
	if err != nil {
		journeyError(w, 409, "library_unavailable", "The current library cannot be verified.")
		return
	}
	for _, prior := range ledger.Revisions {
		if prior.KeyDigest == key {
			if prior.SourceSHA256 != req.SourceSHA256 || prior.Sheet != req.Sheet || reviewDigest(prior.Columns) != reviewDigest(req.Columns) {
				journeyError(w, 409, "import_key_conflict", "That import key belongs to different workbook inputs.")
				return
			}
			journeyJSON(w, 200, prior)
			return
		}
	}
	if len(ledger.Revisions) != req.ExpectedRevision {
		journeyError(w, 409, "library_changed", "The team library changed. Preview and review the new diff before importing.")
		return
	}
	if len(ledger.Revisions) >= controlRevisionLimit {
		journeyError(w, 409, "library_history_full", "The retained team library revision limit was reached.")
		return
	}
	previous := latestControls(ledger)
	added, changed, removed := controlDiff(previous, controls)
	diffDigest := controlDiffDigest(req.SourceSHA256, req.Columns, controls, added, changed, removed)
	if diffDigest != req.ExpectedDiffDigest || r.Context().Err() != nil {
		journeyError(w, 409, "diff_changed", "The reviewed import diff changed. Compare it again.")
		return
	}
	revision := TeamControlRevision{Revision: len(ledger.Revisions) + 1, Digest: reviewDigest(controls), SourceSHA256: req.SourceSHA256, Sheet: req.Sheet, Columns: req.Columns, Controls: controls, Added: added, Changed: changed, Removed: removed, DiffDigest: diffDigest, ActorID: actor.ID, At: time.Now().UTC().Format(time.RFC3339Nano), KeyDigest: key}
	ledger.Revisions = append(ledger.Revisions, revision)
	if err := writeTeamControls(path, ledger); err != nil {
		journeyError(w, 409, "library_write_failed", "The library could not be saved within its bound.")
		return
	}
	journeyJSON(w, 200, revision)
}

// Used by project assessments and orchestration freshness without a network
// request. A personal workspace has no team library, even if freeform Design
// notes are present.
func (s *Server) currentTeamLibrary(ctx context.Context) (*TeamControlRevision, error) {
	access, ok := management.TeamAccessFromContext(ctx)
	if !ok {
		return nil, nil
	}
	if !access.Permissions.Read {
		return nil, errControlWorkbook
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readTeamControls(access.Team.ID)
	if err != nil {
		return nil, err
	}
	return latestControls(ledger), nil
}
func (s *Server) teamLibraryForID(teamID string) (*TeamControlRevision, error) {
	if teamID == "" {
		return nil, nil
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readTeamControls(teamID)
	if err != nil {
		return nil, err
	}
	return latestControls(ledger), nil
}

func controlInRevision(revision *TeamControlRevision, id string) bool {
	if revision == nil {
		return false
	}
	for _, c := range revision.Controls {
		if c.ID == id {
			return true
		}
	}
	return false
}
