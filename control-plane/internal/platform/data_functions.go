package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const DataFunctionMaxFiles = 32
const DataFunctionMaxFileBytes = 64 << 10
const DataFunctionMaxBytes = 1 << 20

var functionName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func ValidDataFunctionName(name string) bool {
	return functionName.MatchString(name) && name != "main" && name != "hello"
}
func ValidDataFunctionAuth(mode string) bool { return mode == "authenticated" || mode == "project-key" }

type DataFunctionFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type DataFunctionSource struct {
	Name            string             `json:"name"`
	Path            string             `json:"path"`
	SourceDigest    string             `json:"sourceDigest"`
	AuthMode        string             `json:"authMode"`
	Files           []DataFunctionFile `json:"files"`
	Commit          string             `json:"commit,omitempty"`
	CommittedDigest string             `json:"committedDigest,omitempty"`
	WorkingChanged  bool               `json:"workingChanged"`
}
type DataFunctionSave struct {
	ExpectedDigest string             `json:"expectedDigest"`
	AuthMode       string             `json:"authMode"`
	Files          []DataFunctionFile `json:"files"`
}

// These callbacks have the product catalogue's file and exact Git boundaries.
// They perform no model calls, builds, resource allocation or function execution.
type DataFunctionSources struct {
	List      func(context.Context, string) ([]DataFunctionSource, error)
	Read      func(context.Context, string, string) (DataFunctionSource, error)
	Save      func(context.Context, string, string, DataFunctionSave) (DataFunctionSource, error)
	Committed func(context.Context, string, string, string) (DataFunctionSource, error)
}
type DataFunctionRelease struct {
	ReleaseID    string `json:"releaseId"`
	Commit       string `json:"commit"`
	SourceDigest string `json:"sourceDigest"`
	AuthMode     string `json:"authMode"`
	PublishedAt  string `json:"publishedAt"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}
type DataFunctionSummary struct {
	Name           string                `json:"name"`
	Path           string                `json:"path"`
	SourceDigest   string                `json:"sourceDigest"`
	AuthMode       string                `json:"authMode"`
	WorkingChanged bool                  `json:"workingChanged"`
	Builtin        bool                  `json:"builtin"`
	Active         *DataFunctionRelease  `json:"active,omitempty"`
	History        []DataFunctionRelease `json:"history"`
}
type DataFunctionOperation struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}
type DataFunctionCatalog struct {
	ProjectID        string                 `json:"projectId"`
	Revision         int                    `json:"revision"`
	CanManage        bool                   `json:"canManage"`
	CustomPublishing bool                   `json:"customPublishing"`
	Functions        []DataFunctionSummary  `json:"functions"`
	Operation        *DataFunctionOperation `json:"operation,omitempty"`
	Message          string                 `json:"message"`
}
type DataFunctionAction struct {
	Action             string `json:"action"`
	ExpectedRevision   int    `json:"expectedRevision"`
	Commit             string `json:"commit,omitempty"`
	ReleaseID          string `json:"releaseId,omitempty"`
	AcknowledgeExecute bool   `json:"acknowledgeExecute,omitempty"`
	AcknowledgeRemove  bool   `json:"acknowledgeRemove,omitempty"`
}
type dataFunctionState struct {
	ProjectID    string
	Revision     int
	StableTenant string
	EngineID     string
	ProbeToken   string
	Releases     map[string][]DataFunctionRelease
	Active       map[string]string
	Operation    *DataFunctionOperation
	// A journaled manifest transition is reconciled by an explicit action only.
	Pending *dataFunctionTransition
}
type dataFunctionTransition struct {
	Before map[string]string
	After  map[string]string
}

func (s *Service) loadDataFunctions(id string) (dataFunctionState, error) {
	v := dataFunctionState{ProjectID: id, Releases: map[string][]DataFunctionRelease{}, Active: map[string]string{}}
	e := s.read(id+"-data-functions", &v)
	if errors.Is(e, os.ErrNotExist) {
		e = nil
	}
	if e != nil || v.ProjectID != id || v.Releases == nil || v.Active == nil || v.Revision < 0 {
		return v, errors.New("function state unavailable")
	}
	return v, nil
}
func (s *Service) changeDataFunctions(id string, change func(*dataFunctionState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataFunctions(id)
	if e != nil {
		return e
	}
	if e = change(&v); e != nil {
		return e
	}
	return s.write(id+"-data-functions", v)
}
func functionRelease(v dataFunctionState, name, id string) (DataFunctionRelease, bool) {
	for _, r := range v.Releases[name] {
		if r.ReleaseID == id {
			return r, true
		}
	}
	return DataFunctionRelease{}, false
}
func (s *Service) dataFunctionCatalog(ctx context.Context, id string, manage bool) (DataFunctionCatalog, error) {
	v, e := s.loadDataFunctions(id)
	if e != nil {
		return DataFunctionCatalog{}, e
	}
	out := DataFunctionCatalog{ProjectID: id, Revision: v.Revision, CanManage: manage, CustomPublishing: s.opts.FunctionSources != nil, Functions: []DataFunctionSummary{{Name: "hello", Path: "Built-in example", AuthMode: "project-key", Builtin: true, History: []DataFunctionRelease{}}}, Operation: v.Operation, Message: "Create a function, save its source, then commit and publish when ready."}
	sources := []DataFunctionSource{}
	if s.opts.FunctionSources != nil {
		sources, e = s.opts.FunctionSources.List(ctx, id)
		if e != nil {
			return out, e
		}
	}
	names := map[string]DataFunctionSource{}
	for _, src := range sources {
		names[src.Name] = src
	}
	for name := range v.Releases {
		if _, ok := names[name]; !ok {
			names[name] = DataFunctionSource{Name: name, Path: "supabase/functions/" + name, AuthMode: "authenticated"}
		}
	}
	keys := []string{}
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		src := names[name]
		item := DataFunctionSummary{Name: name, Path: src.Path, SourceDigest: src.SourceDigest, AuthMode: src.AuthMode, WorkingChanged: src.WorkingChanged, History: []DataFunctionRelease{}}
		for n := len(v.Releases[name]) - 1; n >= 0; n-- {
			item.History = append(item.History, v.Releases[name][n])
		}
		if r, ok := functionRelease(v, name, v.Active[name]); ok {
			if v.Pending == nil {
				r.Status = "active"
				item.Active = &r
			}
		}
		out.Functions = append(out.Functions, item)
	}
	s.mu.Lock()
	busy := s.busy[id]
	s.mu.Unlock()
	if out.Operation != nil && out.Operation.Status == "running" && !busy {
		copy := *out.Operation
		copy.Status = "failed"
		copy.Message = "Publishing was interrupted. Refresh to verify the saved release before retrying."
		out.Operation = &copy
	}
	return out, nil
}
func (s *Service) registerDataFunctions(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/functions", s.functionCatalog)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/functions/{function}", s.functionSource)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/functions/{function}/source", s.functionSave)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/functions/{function}/action", s.functionAction)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/functions/{function}/invoke", s.functionInvoke)
}
func (s *Service) functionCatalog(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	ctx, c := context.WithTimeout(r.Context(), 15*time.Second)
	defer c()
	v, e := s.dataFunctionCatalog(ctx, p.ID, admin(r))
	if e != nil {
		problem(w, 409, "function_source_unavailable", "Function source could not be loaded. Check the project files in Develop.")
		return
	}
	respond(w, 200, v)
}
func builtinFunctionSource() DataFunctionSource {
	return DataFunctionSource{Name: "hello", Path: "Built-in example", AuthMode: "project-key", SourceDigest: dataSum([]byte(dataFunctionHello)), Files: []DataFunctionFile{{Path: "index.ts", Content: dataFunctionHello}}}
}
func (s *Service) functionSource(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	name := r.PathValue("function")
	if name == "hello" {
		respond(w, 200, builtinFunctionSource())
		return
	}
	if !ValidDataFunctionName(name) || s.opts.FunctionSources == nil {
		problem(w, 404, "function_unavailable", "Choose a function in this project.")
		return
	}
	ctx, c := context.WithTimeout(r.Context(), 15*time.Second)
	defer c()
	v, e := s.opts.FunctionSources.Read(ctx, p.ID, name)
	if e != nil {
		problem(w, 404, "function_source_unavailable", "This function source is unavailable. Check its files in Develop.")
		return
	}
	respond(w, 200, v)
}
func (s *Service) functionSave(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	if !admin(r) {
		problem(w, 403, "admin_required", "An Admin manages function source and publishing.")
		return
	}
	name := r.PathValue("function")
	if !ValidDataFunctionName(name) || s.opts.FunctionSources == nil {
		problem(w, 400, "invalid_function", "Use a new function name. The built-in example cannot be edited.")
		return
	}
	var in DataFunctionSave
	if !body(w, r, &in) {
		return
	}
	if !s.dataControlClaim(w, r, p.ID) {
		return
	}
	defer s.release(p.ID)
	ctx, c := context.WithTimeout(r.Context(), 15*time.Second)
	defer c()
	v, e := s.opts.FunctionSources.Save(ctx, p.ID, name, in)
	if e != nil {
		problem(w, 409, "function_source_changed", "The source changed or contains unsupported files. Reload it before saving; use local TypeScript or JavaScript files.")
		return
	}
	respond(w, 200, v)
}
func (s *Service) functionAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	if !admin(r) {
		problem(w, 403, "admin_required", "An Admin publishes and manages functions.")
		return
	}
	name := r.PathValue("function")
	var in DataFunctionAction
	if !body(w, r, &in) {
		return
	}
	if !ValidDataFunctionName(name) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 1_000_000_000 || s.opts.FunctionSources == nil {
		problem(w, 400, "invalid_function_action", "Choose a project function and refresh its saved state.")
		return
	}
	if (in.Action == "publish" || in.Action == "rollback") && !in.AcknowledgeExecute {
		problem(w, 400, "function_review_required", "Review the committed source and acknowledge activating this function.")
		return
	}
	if in.Action == "remove" && !in.AcknowledgeRemove {
		problem(w, 400, "function_remove_required", "Confirm removing the active endpoint. Source and earlier releases are retained.")
		return
	}
	if in.Action != "publish" && in.Action != "rollback" && in.Action != "remove" && in.Action != "refresh" {
		problem(w, 400, "invalid_function_action", "Choose publish, rollback, remove or refresh.")
		return
	}
	if !s.dataControlClaim(w, r, p.ID) {
		return
	}
	releaseClaim := true
	defer func() {
		if releaseClaim {
			s.release(p.ID)
		}
	}()
	v, e := s.loadDataFunctions(p.ID)
	if e != nil || v.Revision != in.ExpectedRevision {
		problem(w, 409, "function_revision_changed", "Function state changed. Refresh before retrying.")
		return
	}
	i, _, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, c := context.WithTimeout(r.Context(), 30*time.Second)
	defer c()
	if e = s.verifyDataOwnership(ctx, i, false); e != nil {
		problem(w, 409, "function_runtime_unavailable", "The function service could not be verified. Refresh Data services before retrying.")
		return
	}
	if e = s.functionEngine(ctx, i, &v); e != nil {
		problem(w, 409, "function_runtime_changed", "The selected workspace changed. Verify its Data services before publishing.")
		return
	}
	if e = s.verifyFunctionHost(ctx, i, v, false); e != nil {
		problem(w, 409, "function_runtime_unavailable", "The function service could not be verified. Refresh Data services before retrying.")
		return
	}
	var source DataFunctionSource
	if in.Action == "publish" {
		if !commitID.MatchString(in.Commit) {
			problem(w, 400, "function_commit_required", "Choose the full committed source revision.")
			return
		}
		source, e = s.opts.FunctionSources.Committed(ctx, p.ID, name, in.Commit)
		if e != nil {
			problem(w, 409, "function_source_not_committed", "Commit the function and approved project context in Develop before publishing.")
			return
		}
	} else if in.Action == "rollback" {
		prior, found := functionRelease(v, name, in.ReleaseID)
		if !found || (prior.Status != "active" && prior.Status != "superseded" && prior.Status != "stopped") {
			problem(w, 409, "function_release_unavailable", "Choose an earlier verified release.")
			return
		}
		source, e = s.readFunctionArtifact(p.ID, prior)
		if e != nil {
			problem(w, 409, "function_release_unavailable", "The retained source could not be verified. Publish its reviewed commit again.")
			return
		}
	}
	if v.Pending != nil && in.Action != "refresh" {
		problem(w, 409, "function_reconciliation_required", "Refresh the interrupted release before making another change.")
		return
	}
	if in.Action == "remove" && v.Active[name] == "" {
		problem(w, 409, "function_not_published", "This function has no active endpoint.")
		return
	}
	v.Revision++
	if v.ProbeToken == "" {
		v.ProbeToken = randomID("function-probe-") + randomID("nonce-")
	}
	v.Operation = &DataFunctionOperation{ID: randomID("function-op-"), Action: in.Action, Name: name, Status: "running", Stage: "queued", Message: "Verifying the function release.", UpdatedAt: now()}
	if e = s.write(p.ID+"-data-functions", v); e != nil {
		problem(w, 500, "function_state_unavailable", "The operation could not be retained. No endpoint was changed.")
		return
	}
	releaseClaim = false
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(p.ID)
		run, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()
		err := s.runFunctionAction(run, i, in, name, source)
		_ = s.changeDataFunctions(p.ID, func(current *dataFunctionState) error {
			current.Operation.Status = "succeeded"
			current.Operation.Stage = "complete"
			current.Operation.Message = "Function endpoint updated. Source and earlier releases are retained."
			if err != nil {
				current.Operation.Status = "failed"
				current.Operation.Message = "The function could not be activated or verified. The prior endpoint is retained when recovery succeeds. Refresh, then check the source in Develop."
			}
			current.Operation.UpdatedAt = now()
			return nil
		})
	}()
	out, _ := s.dataFunctionCatalog(r.Context(), p.ID, true)
	respond(w, 202, out)
}

func (s *Service) functionInvoke(w http.ResponseWriter, r *http.Request) {
	i, secret, ok := s.readyData(w, r)
	if !ok {
		return
	}
	name := r.PathValue("function")
	var release DataFunctionRelease
	mode := "project-key"
	if name != "hello" {
		if !ValidDataFunctionName(name) {
			problem(w, 404, "function_unavailable", "Choose a published function.")
			return
		}
		v, e := s.loadDataFunctions(i.ProjectID)
		var found bool
		release, found = functionRelease(v, name, v.Active[name])
		if e != nil || !found || v.Pending != nil {
			problem(w, 404, "function_not_published", "Publish and verify this function before invoking it.")
			return
		}
		mode = release.AuthMode
	}
	var raw map[string]json.RawMessage
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if d.Decode(&raw) != nil || raw == nil || d.Decode(new(any)) != io.EOF {
		problem(w, 400, "invalid_function_input", "Supply one JSON object of up to 64 KiB.")
		return
	}
	// Legacy hello accepts its original object. New requests wrap input so an
	// application session token remains outside the customer request body.
	input := raw
	token := ""
	if wrapped, exists := raw["input"]; exists {
		// Unmarshal must use a fresh map: input otherwise aliases the outer
		// envelope and customer keys become apparent request fields.
		input = nil
		if json.Unmarshal(wrapped, &input) != nil || input == nil {
			problem(w, 400, "invalid_function_input", "Supply a JSON object for input.")
			return
		}
		for key := range raw {
			if key != "input" && key != "applicationAccessToken" {
				problem(w, 400, "invalid_function_input", "Use input and an optional application session token.")
				return
			}
		}
		if b := raw["applicationAccessToken"]; b != nil && json.Unmarshal(b, &token) != nil {
			problem(w, 400, "invalid_function_token", "Use an application user session token.")
			return
		}
	}
	if mode == "authenticated" && token == "" {
		problem(w, 400, "application_session_required", "Use a signed-in application's user token to test this function.")
		return
	}
	if token == "" {
		token = secret.Anon
	} else if !functionUserToken(token) {
		problem(w, 400, "invalid_function_token", "Use an application's user session token. Luci and server credentials are not accepted.")
		return
	}
	if !s.dataControlClaim(w, r, i.ProjectID) {
		return
	}
	defer s.release(i.ProjectID)
	ctx, c := context.WithTimeout(r.Context(), 15*time.Second)
	defer c()
	if e := s.verifyDataOwnership(ctx, i, false); e != nil {
		dataControlError(w)
		return
	}
	b, _ := json.Marshal(input)
	started := time.Now()
	status, typ, result, e := s.functionRequest(ctx, i, secret, name, b, token, "")
	if e != nil {
		problem(w, 502, "function_outcome_uncertain", "The function response could not be received. Its outcome is uncertain; check the application before retrying.")
		return
	}
	respond(w, 200, map[string]any{"function": name, "releaseId": release.ReleaseID, "statusCode": status, "contentType": typ, "result": result, "durationMs": time.Since(started).Milliseconds()})
}
func functionUserToken(token string) bool {
	if len(token) < 32 || len(token) > 4096 || strings.ContainsAny(token, "\r\n\x00 ") {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	// This is an admission filter only; the native worker cryptographically
	// verifies the signature, expiry and role before running customer code.
	claims, e := decodeFunctionClaims(parts[1])
	return e == nil && claims.Role == "authenticated" && claims.Sub != ""
}
