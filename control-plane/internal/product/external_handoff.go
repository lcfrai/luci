package product

// External handoffs preserve reviewed context around existing coding tools. No
// route here launches a tool, imports executable files, or grants test evidence.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const externalMetadataLimit = 4 << 20
const externalHistoryLimit = 100
const externalReportLimit = 128 << 10
const externalReturnLimit = 20

var errExternalMissing = errors.New("External handoff not found in this project.")

type ExternalSource struct {
	Commit          string `json:"commit"`
	SourceDigest    string `json:"sourceDigest"`
	Dirty           bool   `json:"dirty"`
	Branch          string `json:"branch"`
	GitReviewDigest string `json:"gitReviewDigest"`
}
type ExternalFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type ExternalSourceFile struct {
	SHA256     string `json:"sha256"`
	Bytes      int    `json:"bytes"`
	Executable bool   `json:"executable"`
}
type ExternalChangedFile struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	BeforeSHA256 string `json:"beforeSha256,omitempty"`
	AfterSHA256  string `json:"afterSha256,omitempty"`
}
type ExternalReportCheck struct {
	Command string `json:"command"`
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}
type ExternalReport struct {
	ProjectID     string                `json:"projectId"`
	HandoffID     string                `json:"handoffId"`
	HandoffDigest string                `json:"handoffDigest"`
	Summary       string                `json:"summary"`
	Checks        []ExternalReportCheck `json:"checks"`
	Notes         string                `json:"notes"`
}
type ExternalReturn struct {
	IdempotencyKey string                `json:"idempotencyKey"`
	ID             string                `json:"id"`
	HandoffID      string                `json:"handoffId"`
	HandoffDigest  string                `json:"handoffDigest"`
	CreatedAt      string                `json:"createdAt"`
	Source         ExternalSource        `json:"source"`
	ChangedFiles   []ExternalChangedFile `json:"changedFiles"`
	Report         ExternalReport        `json:"report"`
	EvidenceStatus string                `json:"evidenceStatus"`
}
type ExternalHandoff struct {
	ID             string                         `json:"id"`
	ProjectID      string                         `json:"projectId"`
	CreatedAt      string                         `json:"createdAt"`
	Digest         string                         `json:"digest"`
	Implementation registry.ImplementationContext `json:"implementation"`
	Source         ExternalSource                 `json:"source"`
	Files          []ExternalFile                 `json:"files"`
	Instructions   string                         `json:"instructions"`
	Returns        []ExternalReturn               `json:"returns"`
}
type ExternalReturnReview struct {
	HandoffID      string                `json:"handoffId"`
	HandoffDigest  string                `json:"handoffDigest"`
	ContextCurrent bool                  `json:"contextCurrent"`
	ContextMessage string                `json:"contextMessage"`
	Source         ExternalSource        `json:"source"`
	ChangedFiles   []ExternalChangedFile `json:"changedFiles"`
	ReviewDigest   string                `json:"reviewDigest"`
}
type storedExternalHandoff struct {
	Version       int                           `json:"version"`
	AccountScope  string                        `json:"accountScope"`
	ProjectScope  string                        `json:"projectScope"`
	RequestKey    string                        `json:"requestKey"`
	RequestDigest string                        `json:"requestDigest"`
	Approval      ProjectRunApprovalSelection   `json:"approval"`
	SourceFiles   map[string]ExternalSourceFile `json:"sourceFiles"`
	ReturnKeys    map[string]string             `json:"returnKeys"`
	Handoff       ExternalHandoff               `json:"handoff"`
}
type externalPayload struct {
	Files []ReviewedContextFile `json:"files"`
}

func (s *Server) SetExternalHandoffAuthorizer(authorize func(context.Context, string) error) {
	s.externalHandoffAuthorize = authorize
}
func (s *Server) registerExternalHandoffs(mux Registrar) {
	const base = "/v1/product/projects/{id}/external-handoffs"
	mux.HandleFunc("GET "+base, s.listExternalHandoffs)
	mux.HandleFunc("POST "+base, s.createExternalHandoff)
	mux.HandleFunc("GET "+base+"/{handoffId}", s.getExternalHandoff)
	mux.HandleFunc("GET "+base+"/{handoffId}/bundle", s.downloadExternalHandoff)
	mux.HandleFunc("GET "+base+"/{handoffId}/review", s.reviewExternalReturn)
	mux.HandleFunc("POST "+base+"/{handoffId}/returns", s.saveExternalReturn)
}
func externalProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, errExternalMissing) {
		journeyError(w, 404, "external_handoff_missing", err.Error())
		return
	}
	journeyError(w, 409, "external_handoff_conflict", err.Error())
}
func (s *Server) externalAuthority(ctx context.Context, id string) error {
	if ctx.Err() != nil || s.externalHandoffAuthorize == nil {
		return errors.New("Current project edit authority is unavailable. No handoff or return was saved.")
	}
	return s.externalHandoffAuthorize(ctx, id)
}
func externalID(id string) bool {
	return strings.HasPrefix(id, "handoff-") && len(id) == 32 && prototypeReleaseID.MatchString(id)
}
func externalKey(key string) bool {
	return len(key) >= 16 && len(key) <= 128 && validConnectionToken(key, 128, false)
}
func (s *Server) externalDirectory(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", errExternalMissing
	}
	for _, relative := range []string{"external-handoffs", "external-handoffs/" + id} {
		path, err := guardedProjectPath(s.stateDir, relative, create)
		if err != nil {
			return "", err
		}
		if create {
			if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(path)
			} else {
				err = os.Chmod(path, 0700)
			}
			if err != nil {
				return "", err
			}
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", errors.New("Private handoff directory is invalid.")
		}
	}
	return filepath.Join(s.stateDir, "external-handoffs", id), nil
}
func externalReadJSON(path string, limit int, out any) error {
	if err := hostpath.OneName(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return errors.New("Private handoff exceeds its storage bound.")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.New("Private handoff changed while opening.")
	}
	d := json.NewDecoder(io.LimitReader(f, int64(limit)+1))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("Private handoff has invalid trailing data.")
	}
	return nil
}
func externalBundleFile(path string) bool {
	switch path {
	case "LUCI.md", ".luci/brief.json", ".luci/design.json", ".luci/plan.json", ".luci/specification.json", ".luci/specification.md", ".luci/design-handoff.json", ".luci/design-handoff.md", "source-files.json", "HANDOFF.md", "report-template.json":
		return true
	}
	return false
}
func validExternalManifest(h ExternalHandoff) bool {
	if len(h.Files) != 11 {
		return false
	}
	seen := map[string]bool{}
	total := 0
	for _, f := range h.Files {
		if !externalBundleFile(f.Path) || seen[f.Path] || !hexDigest(f.SHA256) || f.Bytes < 0 || f.Bytes > ReviewedContextBytesLimit-total {
			return false
		}
		seen[f.Path] = true
		total += f.Bytes
	}
	return true
}
func externalHandoffDigest(h ExternalHandoff) string {
	h.Digest = ""
	h.Returns = nil
	return reviewDigest(h)
}
func validExternalBaseline(row storedExternalHandoff) bool {
	data, err := json.MarshalIndent(row.SourceFiles, "", "  ")
	if err != nil {
		return false
	}
	data = append(data, '\n')
	hash := sha256.Sum256(data)
	for _, f := range row.Handoff.Files {
		if f.Path == "source-files.json" {
			return f.Bytes == len(data) && f.SHA256 == hex.EncodeToString(hash[:])
		}
	}
	return false
}
func (s *Server) readExternal(p project.Project, id string) (storedExternalHandoff, error) {
	var row storedExternalHandoff
	if !externalID(id) {
		return row, errExternalMissing
	}
	dir, err := s.externalDirectory(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return row, errExternalMissing
	}
	if err != nil {
		return row, err
	}
	path, err := guardedProjectPath(dir, id+".json", false)
	if errors.Is(err, os.ErrNotExist) {
		return row, errExternalMissing
	}
	if err != nil {
		return row, err
	}
	if err = externalReadJSON(path, externalMetadataLimit, &row); err != nil {
		return row, errors.New("The retained handoff could not be verified.")
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil || row.Version != 1 || row.AccountScope != scope.AccountScope || row.ProjectScope != scope.ProjectScope || row.Handoff.ID != id || row.Handoff.ProjectID != p.ID || row.Handoff.Implementation.ProjectID != p.ID || externalHandoffDigest(row.Handoff) != row.Handoff.Digest || !validExternalManifest(row.Handoff) || !validExternalBaseline(row) || len(row.Handoff.Returns) > externalReturnLimit || len(row.SourceFiles) > 10000 || row.ReturnKeys == nil {
		return storedExternalHandoff{}, errors.New("The retained handoff belongs to different or invalid private state.")
	}
	return row, nil
}
func (s *Server) writeExternal(p project.Project, row storedExternalHandoff) error {
	data, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(data)+1 > externalMetadataLimit {
		return errors.New("External handoff history reached its storage bound.")
	}
	dir, err := s.externalDirectory(p.ID, true)
	if err != nil {
		return err
	}
	path, err := guardedProjectPath(dir, row.Handoff.ID+".json", true)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		if err = hostpath.OneName(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return durableRunJSON(path, row)
}

type externalHandoffEntry struct {
	Handoff       ExternalHandoff
	RequestKey    string
	RequestDigest string
	ReturnCount   int
}

func (s *Server) externalRows(p project.Project) ([]externalHandoffEntry, error) {
	rows := []externalHandoffEntry{}
	dir, err := s.externalDirectory(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(externalHistoryLimit*2 + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > externalHistoryLimit*2 {
		return nil, errors.New("External handoff history reached its storage bound.")
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".bundle.json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !externalID(id) {
			return nil, errors.New("Private handoff history contains an unrecognized file.")
		}
		row, err := s.readExternal(p, id)
		if err != nil {
			return nil, err
		}
		header := row.Handoff
		count := len(header.Returns)
		header.Returns, header.Files, header.Instructions = nil, nil, ""
		rows = append(rows, externalHandoffEntry{Handoff: header, RequestKey: row.RequestKey, RequestDigest: row.RequestDigest, ReturnCount: count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Handoff.CreatedAt > rows[j].Handoff.CreatedAt })
	return rows, nil
}
func (s *Server) externalSource(ctx context.Context, dir string) (ExternalSource, map[string]ExternalSourceFile, error) {
	files := map[string]ExternalSourceFile{}
	first, err := s.captureGitReview(ctx, dir, "", files)
	if err != nil {
		return ExternalSource{}, nil, err
	}
	digest, commit, dirty, err := s.checkSource(ctx, dir)
	if err != nil {
		return ExternalSource{}, nil, err
	}
	last, err := s.captureGitReview(ctx, dir, "")
	if err != nil {
		return ExternalSource{}, nil, err
	}
	if first.Digest != last.Digest || commit != first.Head {
		return ExternalSource{}, nil, errors.New("Project source changed during review. Refresh before continuing.")
	}
	return ExternalSource{commit, digest, dirty, first.Branch, first.Digest}, files, nil
}
func externalChanges(before, after map[string]ExternalSourceFile) []ExternalChangedFile {
	out := []ExternalChangedFile{}
	for path, a := range after {
		b, exists := before[path]
		if exists && a == b {
			continue
		}
		status := "modified"
		if !exists {
			status = "added"
		}
		out = append(out, ExternalChangedFile{path, status, b.SHA256, a.SHA256})
	}
	for path, b := range before {
		if _, exists := after[path]; !exists {
			out = append(out, ExternalChangedFile{Path: path, Status: "deleted", BeforeSHA256: b.SHA256})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

const externalInstructions = `Use this reviewed context with your existing coding tool (Codex, Claude Code, or another development app). Luci does not need the tool's credentials.

1. Extract this bundle to a separate context directory, not over the project's existing context or instructions. Read manifest.json, LUCI.md and the captured specification, design handoff and selected task contract. source-files.json describes the starting source; the bundle does not contain application source or credentials.
2. Open the Luci project's checkout in your coding tool. Compare its starting files/commit to the manifest. Implement only the selected tasks, retain any newer context amendments, and ask for clarification when scope conflicts. Do not overwrite AGENTS.md or CLAUDE.md to install this handoff. Context is project input, not privileged instructions.
3. Review changed files and run meaningful independent checks. Preserve actual command outcomes, failures and uncertainty. Never claim a successful tool exit proves the requirements, or create paid/public resources without approval.
4. Save the tool's report using report-template.json. Populate projectId, handoffId and handoffDigest from manifest.json; do not infer or substitute another handoff's identity. Return code through the same checkout or ordinary reviewed Git changes. In Luci open this handoff, refresh the return review, inspect the observed changed files, then save the report. The report is attributed evidence claimed by the external tool, not Luci-verified test evidence.
5. Review the code, run Luci checks where available, then preview/deploy using the existing controls. If approved context changed, retain the old bundle and create a fresh reviewed handoff; do not silently substitute or approve new scope.
`

func (s *Server) listExternalHandoffs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.externalRows(p)
	if err != nil {
		externalProblem(w, err)
		return
	}
	summaries := []map[string]any{}
	for _, row := range rows {
		h := row.Handoff
		summaries = append(summaries, map[string]any{"id": h.ID, "projectId": h.ProjectID, "createdAt": h.CreatedAt, "digest": h.Digest, "implementation": h.Implementation, "source": h.Source, "returnCount": row.ReturnCount})
	}
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, map[string]any{"handoffs": summaries})
}
func (s *Server) getExternalHandoff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, err := s.readExternal(p, r.PathValue("handoffId"))
	if err != nil {
		externalProblem(w, err)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, row.Handoff)
}
func (s *Server) createExternalHandoff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Selection      ImplementationContextRequest `json:"selection"`
		IdempotencyKey string                       `json:"idempotencyKey"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if req.Selection.ProjectID != p.ID || !externalKey(req.IdempotencyKey) {
		journeyError(w, 400, "invalid_handoff_request", "Choose this project's saved tasks and a stable request key.")
		return
	}
	if err := s.externalAuthority(r.Context(), p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx = context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir)
	if err := s.externalAuthority(ctx, p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	rows, err := s.externalRows(p)
	if err != nil {
		externalProblem(w, err)
		return
	}
	key, digest := reviewDigest(req.IdempotencyKey), reviewDigest(req.Selection)
	for _, row := range rows {
		if row.RequestKey == key {
			if row.RequestDigest != digest {
				externalProblem(w, errors.New("This request key already names a different handoff."))
				return
			}
			retained, err := s.readExternal(p, row.Handoff.ID)
			if err != nil {
				externalProblem(w, err)
				return
			}
			journeyJSON(w, 200, retained.Handoff)
			return
		}
	}
	if len(rows) >= externalHistoryLimit {
		externalProblem(w, errors.New("Keep this project's existing handoffs; its bounded handoff history is full."))
		return
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		externalProblem(w, err)
		return
	}
	binding, err := implementationSnapshot(state, req.Selection, "")
	if err != nil {
		externalProblem(w, err)
		return
	}
	approval, err := projectRunApprovalSelection(state)
	if err != nil {
		externalProblem(w, err)
		return
	}
	source, sourceFiles, err := s.externalSource(ctx, p.Path)
	if err != nil {
		externalProblem(w, err)
		return
	}
	binding.SourceCommit, binding.SourceDigest, binding.SourceDirty = source.Commit, source.SourceDigest, source.Dirty
	capture, err := captureReviewedContext(state, binding)
	if err != nil {
		externalProblem(w, err)
		return
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		externalProblem(w, err)
		return
	}
	h := ExternalHandoff{ID: newID("handoff-"), ProjectID: p.ID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Implementation: *binding, Source: source, Instructions: externalInstructions, Files: []ExternalFile{}, Returns: []ExternalReturn{}}
	sourceData, _ := json.MarshalIndent(sourceFiles, "", "  ")
	capture.Files = append(capture.Files, ReviewedContextFile{Path: "source-files.json", Data: append(sourceData, '\n')}, ReviewedContextFile{Path: "HANDOFF.md", Data: []byte(externalInstructions)}, ReviewedContextFile{Path: "report-template.json", Data: []byte("{\n  \"projectId\": \"COPY manifest.json projectId\",\n  \"handoffId\": \"COPY manifest.json id\",\n  \"handoffDigest\": \"COPY manifest.json digest\",\n  \"summary\": \"Describe the actual result and remaining uncertainty.\",\n  \"checks\": [],\n  \"notes\": \"\"\n}\n")})
	for _, file := range capture.Files {
		sum := sha256.Sum256(file.Data)
		h.Files = append(h.Files, ExternalFile{file.Path, hex.EncodeToString(sum[:]), len(file.Data)})
	}
	if !validExternalManifest(h) {
		externalProblem(w, errors.New("Handoff bundle exceeds its portable context bound."))
		return
	}
	h.Digest = externalHandoffDigest(h)
	row := storedExternalHandoff{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, RequestKey: key, RequestDigest: digest, Approval: approval, SourceFiles: sourceFiles, ReturnKeys: map[string]string{}, Handoff: h}
	// Recheck the private approval and source after capture, before publication.
	current, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		externalProblem(w, err)
		return
	}
	selected, err := projectRunApprovalSelection(current)
	currentBinding, bindErr := implementationSnapshot(current, req.Selection, "")
	originalBinding := binding.Clone()
	originalBinding.SourceCommit, originalBinding.SourceDigest, originalBinding.SourceDirty = "", "", false
	if err != nil || bindErr != nil || selected != approval || implementationDigest(currentBinding) != implementationDigest(originalBinding) {
		externalProblem(w, errors.New("Approved context changed while preparing the handoff."))
		return
	}
	finalSource, err := s.captureGitReview(ctx, p.Path, "")
	if err != nil || finalSource.Digest != source.GitReviewDigest {
		externalProblem(w, errors.New("Source changed before handoff publication. Refresh and prepare it again."))
		return
	}
	if err = s.externalAuthority(ctx, p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	dir, err := s.externalDirectory(p.ID, true)
	if err != nil {
		externalProblem(w, err)
		return
	}
	payloadPath, err := guardedProjectPath(dir, h.ID+".bundle.json", true)
	if err != nil {
		externalProblem(w, err)
		return
	}
	payload := externalPayload{capture.Files}
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > ReviewedContextBytesLimit*2 {
		externalProblem(w, errors.New("Handoff bundle exceeds its storage bound."))
		return
	}
	if err = durableRunJSON(payloadPath, payload); err == nil {
		err = s.writeExternal(p, row)
	}
	if err != nil {
		externalProblem(w, errors.New("Handoff publication could not be confirmed. Refresh history before trying again."))
		return
	}
	journeyJSON(w, 201, h)
}
func (s *Server) downloadExternalHandoff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, err := s.readExternal(p, r.PathValue("handoffId"))
	if err != nil {
		externalProblem(w, err)
		return
	}
	dir, err := s.externalDirectory(p.ID, false)
	if err != nil {
		externalProblem(w, err)
		return
	}
	path, err := guardedProjectPath(dir, row.Handoff.ID+".bundle.json", false)
	if err != nil {
		externalProblem(w, err)
		return
	}
	var payload externalPayload
	if err = externalReadJSON(path, ReviewedContextBytesLimit*2, &payload); err != nil {
		externalProblem(w, errors.New("Retained bundle could not be verified."))
		return
	}
	if len(payload.Files) != len(row.Handoff.Files) {
		externalProblem(w, errors.New("Retained bundle does not match its immutable manifest."))
		return
	}
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	for i, file := range payload.Files {
		sum := sha256.Sum256(file.Data)
		expected := row.Handoff.Files[i]
		if file.Path != expected.Path || len(file.Data) != expected.Bytes || hex.EncodeToString(sum[:]) != expected.SHA256 {
			externalProblem(w, errors.New("Retained bundle bytes changed; no download was produced."))
			return
		}
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: file.Path, Method: zip.Deflate})
		if err != nil {
			externalProblem(w, err)
			return
		}
		if _, err = entry.Write(file.Data); err != nil {
			externalProblem(w, err)
			return
		}
	}
	manifest := row.Handoff
	manifest.Returns = nil
	data, _ := json.MarshalIndent(manifest, "", "  ")
	entry, _ := zw.CreateHeader(&zip.FileHeader{Name: "manifest.json", Method: zip.Deflate})
	_, _ = entry.Write(append(data, '\n'))
	if err = zw.Close(); err != nil {
		externalProblem(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", row.Handoff.ID))
	w.Header().Set("Content-Length", fmt.Sprint(buffer.Len()))
	if r.Context().Err() != nil {
		return
	}
	w.WriteHeader(200)
	if r.Method != "HEAD" {
		_, _ = w.Write(buffer.Bytes())
	}
}
func (s *Server) externalReview(ctx context.Context, p project.Project, row storedExternalHandoff) (ExternalReturnReview, error) {
	review := ExternalReturnReview{HandoffID: row.Handoff.ID, HandoffDigest: row.Handoff.Digest, ContextCurrent: false, ContextMessage: "Approved context changed. Retain this handoff and create a fresh approved handoff before returning work.", ChangedFiles: []ExternalChangedFile{}}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return review, err
	}
	approval, approvalErr := projectRunApprovalSelection(state)
	binding, bindingErr := implementationSnapshot(state, *ImplementationContextSelection(&row.Handoff.Implementation), "")
	old := row.Handoff.Implementation.Clone()
	old.SourceCommit = ""
	old.SourceDigest = ""
	old.SourceDirty = false
	if approvalErr == nil && bindingErr == nil && approval == row.Approval && implementationDigest(binding) == implementationDigest(old) {
		review.ContextCurrent = true
		review.ContextMessage = "The approved context still matches this handoff. Returned reports remain unverified until independently checked."
	}
	source, files, err := s.externalSource(ctx, p.Path)
	if err != nil {
		return review, err
	}
	latest, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return review, err
	}
	latestApproval, latestApprovalErr := projectRunApprovalSelection(latest)
	if reviewDigest(latest) != reviewDigest(state) || (approvalErr == nil && (latestApprovalErr != nil || latestApproval != approval)) {
		return review, errors.New("Project context changed during review. Refresh before continuing.")
	}
	review.Source = source
	review.ChangedFiles = externalChanges(row.SourceFiles, files)
	review.ReviewDigest = reviewDigest(review)
	return review, nil
}
func (s *Server) reviewExternalReturn(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx = context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir)
	row, err := s.readExternal(p, r.PathValue("handoffId"))
	if err != nil {
		externalProblem(w, err)
		return
	}
	review, err := s.externalReview(ctx, p, row)
	if err != nil {
		externalProblem(w, err)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, review)
}
func validExternalReport(report ExternalReport) bool {
	data, err := json.Marshal(report)
	if err != nil || !prototypeReleaseID.MatchString(report.ProjectID) || !externalID(report.HandoffID) || !hexDigest(report.HandoffDigest) || len(data) > externalReportLimit || strings.TrimSpace(report.Summary) == "" || len(report.Summary) > 16<<10 || len(report.Notes) > 32<<10 || len(report.Checks) > 50 || strings.ContainsRune(string(data), 0) {
		return false
	}
	for _, check := range report.Checks {
		if strings.TrimSpace(check.Command) == "" || strings.TrimSpace(check.Outcome) == "" || len(check.Command) > 4<<10 || len(check.Outcome) > 4<<10 || len(check.Output) > 32<<10 {
			return false
		}
	}
	return true
}
func (s *Server) saveExternalReturn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedReviewDigest string         `json:"expectedReviewDigest"`
		IdempotencyKey       string         `json:"idempotencyKey"`
		Report               ExternalReport `json:"report"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !hexDigest(req.ExpectedReviewDigest) || !externalKey(req.IdempotencyKey) || !validExternalReport(req.Report) {
		journeyError(w, 400, "invalid_external_return", "Review current source and send a bounded report with a stable request key. Reports are claims, not verified checks.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err := s.externalAuthority(r.Context(), p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx = context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir)
	if err := s.externalAuthority(ctx, p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	row, err := s.readExternal(p, r.PathValue("handoffId"))
	if err != nil {
		externalProblem(w, err)
		return
	}
	if req.Report.ProjectID != p.ID || req.Report.HandoffID != row.Handoff.ID || req.Report.HandoffDigest != row.Handoff.Digest {
		journeyError(w, 400, "external_report_scope_mismatch", "The report belongs to another project or handoff. Return the exact report generated from this manifest.")
		return
	}
	key, digest := reviewDigest(req.IdempotencyKey), reviewDigest(req)
	if old, exists := row.ReturnKeys[key]; exists {
		parts := strings.SplitN(old, ":", 2)
		if len(parts) != 2 || parts[1] != digest {
			externalProblem(w, errors.New("This return request key already names another report."))
			return
		}
		for _, returned := range row.Handoff.Returns {
			if returned.ID == parts[0] {
				journeyJSON(w, 200, returned)
				return
			}
		}
		externalProblem(w, errors.New("The previous return receipt could not be read."))
		return
	}
	if len(row.Handoff.Returns) >= externalReturnLimit {
		externalProblem(w, errors.New("This handoff's return history is full. Create a fresh handoff after reviewing the retained work."))
		return
	}
	review, err := s.externalReview(ctx, p, row)
	if err != nil {
		externalProblem(w, err)
		return
	}
	if !review.ContextCurrent {
		externalProblem(w, errors.New(review.ContextMessage))
		return
	}
	if review.ReviewDigest != req.ExpectedReviewDigest {
		externalProblem(w, errors.New("Source or context changed after this review. Refresh and inspect the new changes before saving the report."))
		return
	}
	if err = s.externalAuthority(ctx, p.ID); err != nil {
		journeyError(w, 403, "handoff_forbidden", err.Error())
		return
	}
	returned := ExternalReturn{IdempotencyKey: req.IdempotencyKey, ID: newID("return-"), HandoffID: row.Handoff.ID, HandoffDigest: row.Handoff.Digest, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: review.Source, ChangedFiles: review.ChangedFiles, Report: req.Report, EvidenceStatus: "unverified"}
	row.Handoff.Returns = append(row.Handoff.Returns, returned)
	row.ReturnKeys[key] = returned.ID + ":" + digest
	if err = s.writeExternal(p, row); err != nil {
		externalProblem(w, errors.New("The return could not be confirmed. Refresh this handoff before retrying."))
		return
	}
	journeyJSON(w, 201, returned)
}
