package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func (a *API) projectAssistance(id string) ([]assistance.Job, error) {
	if a.deps.Product == nil {
		return nil, os.ErrNotExist
	}
	if _, err := a.deps.Product.ResolveProject(id); err != nil {
		return nil, os.ErrNotExist
	}
	dir := filepath.Join(a.deps.ConversationsDir, "assistance", "jobs")
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return []assistance.Job{}, nil
	}
	if err := codexRegularPath(a.deps.ConversationsDir, dir); err != nil {
		return nil, err
	}
	file, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(4097)
	if err != nil && len(entries) == 0 {
		return nil, err
	}
	if len(entries) > 4096 {
		return nil, errors.New("assistant history exceeds its bounded listing limit")
	}
	jobs := []assistance.Job{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		job, err := a.assistanceStore().Read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, errors.New("private assistant history cannot be read safely")
		}
		if job.ProjectID == id {
			job.Result = nil
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	return jobs, nil
}

func (a *API) handleAssistList(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.projectAssistance(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			httpx.WriteProblem(w, 404, "project_not_found", "No project in your workspace has that id.", nil)
		} else {
			httpx.WriteProblem(w, 500, "assistant_history_unavailable", "Private assistant history could not be loaded.", nil)
		}
		return
	}
	if len(jobs) > 20 {
		jobs = jobs[:20]
	}
	httpx.WriteJSON(w, 200, map[string]any{"jobs": jobs})
}

// analysisSession has a new empty home and only the selected connection's
// environment. No host CLI profile, project directory or Git settings mount.
// Caller holds codexAccountMu, which also serializes analysis admission.
func (a *API) analysisSession(ctx context.Context, projectID, harness, model, auth string, env map[string]string, account *codexConnection) (registry.Session, backend.Spec, error) {
	id, conversation := ids.New("s"), ids.UUID()
	s := registry.Session{ID: id, Title: "Repository analysis", MachineID: LocalMachineID, ProjectID: projectID, WorktreeID: assistWorktree, Harness: harness, Model: model, PermissionMode: "plan", AuthMode: auth, CodexConnectionID: codexConnectionID(account), ConversationID: conversation, Status: registry.StatusStarting, Backend: string(a.deps.Backend.Kind()), SandboxID: id, AttachToken: ids.Token(), ContextRef: "luci:repository-analysis", Command: "python3", Args: []string{"/luci-input/run.py"}}
	var home string
	var err error
	if nativeHarness(harness) {
		home, err = a.prepareNativeState(assistWorktree, harness, conversation, sessionResumeSource{}, false)
	} else {
		home, err = a.assistDirectory(filepath.Join("assistance", "homes", id))
	}
	if err != nil {
		return s, backend.Spec{}, err
	}
	if account != nil {
		if err = seedCodexConnection(home, account); err != nil {
			return s, backend.Spec{}, err
		}
	}
	workspace, err := a.assistDirectory(filepath.Join("assistance", "workspaces", id))
	if err != nil {
		return s, backend.Spec{}, err
	}
	env["GIT_CONFIG_GLOBAL"], env["GIT_CONFIG_NOSYSTEM"] = "/dev/null", "1"
	if nativeHarness(harness) {
		nativeEnvironment(harness, env, "plan")
		if harness == "opencode" {
			nativeDockerEndpoint(env)
			env["OPENCODE_PERMISSION"] = `{"*":"deny"}`
		}
	} else {
		env["HOME"] = nativeAgentDir
		env["CLAUDE_CONFIG_DIR"] = nativeAgentDir + "/.claude"
	}
	spec := backend.Spec{ID: id, SessionID: id, Image: a.sessionImage(), Command: s.Command, Args: s.Args, Env: env, AttachToken: s.AttachToken, Resources: a.sessionResources(ctx), WorkDir: sessionWorkDir, Mounts: []backend.Mount{{HostPath: workspace, ContainerPath: sessionWorkDir}, {HostPath: home, ContainerPath: nativeAgentDir}}}
	if a.deps.CABundle != "" {
		spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: a.deps.CABundle, ContainerPath: sessionCABundlePath, ReadOnly: true})
		env["NODE_EXTRA_CA_CERTS"] = sessionCABundlePath
	}
	if err = a.deps.Store.Create(s); err != nil {
		return s, spec, err
	}
	if err = a.leaseCodexConnection(account, id); err != nil {
		a.markSessionExited(id, "analysis connection could not be reserved")
		return s, spec, err
	}
	return s, spec, nil
}

func (a *API) handleRepositoryAnalysis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision      int    `json:"expectedRevision"`
		ExpectedBriefRevision int    `json:"expectedBriefRevision"`
		SourceMode            string `json:"sourceMode"`
		Harness               string `json:"harness"`
		Model                 string `json:"model"`
		AuthMode              string `json:"authMode"`
		AcknowledgeSourceRead bool   `json:"acknowledgeSourceRead"`
		Message               string `json:"message"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := assistance.ValidateSelection(req.Harness, req.Model, req.AuthMode); err != nil {
		httpx.WriteProblem(w, 400, "invalid_analysis_provider", err.Error(), nil)
		return
	}
	if !req.AcknowledgeSourceRead || len(req.Message) > 16000 || req.ExpectedRevision < 0 || req.ExpectedBriefRevision < 0 || (req.SourceMode != "worktree" && req.SourceMode != "commit") {
		httpx.WriteProblem(w, 400, "invalid_analysis_request", "Choose a source, acknowledge sending public source to the selected provider, and use current definition revisions.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "product_unavailable", "Project definitions are unavailable.", nil)
		return
	}
	if a.deps.Backend.Kind() != backend.KindDocker {
		httpx.WriteProblem(w, 409, "docker_required", "Repository analysis requires the Docker runtime.", nil)
		return
	}
	// Every analysis provider is serialized here during admission only. Running
	// jobs retain their own session; other projects can use different providers.
	a.codexAccountMu.Lock()
	defer a.codexAccountMu.Unlock()
	jobs, err := a.projectAssistance(r.PathValue("id"))
	if err != nil {
		httpx.WriteProblem(w, 404, "project_not_found", "This owned project's assistant history is unavailable.", nil)
		return
	}
	for _, job := range jobs {
		if !assistance.Terminal(job.Status) {
			httpx.WriteProblem(w, 409, "analysis_busy", "An assistant is already running for this project. Resume or cancel it before starting another.", nil)
			return
		}
	}
	env := a.deps.Product.AgentEnvironment(req.Harness)
	if env == nil {
		env = map[string]string{}
	}
	var account *codexConnection
	if req.Harness == "codex" && req.AuthMode == "chatgpt" {
		account, _, err = a.selectCodexConnection(r.Context(), registry.Session{}, false, "")
		if err != nil {
			codexAccountProblem(w, err)
			return
		}
		if account == nil {
			codexAccountProblem(w, errCodexDisconnected)
			return
		}
		env = map[string]string{}
	} else if req.Harness == "codex" && env["OPENAI_API_KEY"] == "" || req.Harness == "claude-code" && env["ANTHROPIC_API_KEY"] == "" || req.Harness == "opencode" && env["OPENCODE_CONFIG_CONTENT"] == "" {
		httpx.WriteProblem(w, 409, "analysis_connection_required", "Connect the selected provider in Settings before analysing source. A different provider will not be substituted.", nil)
		return
	}
	if req.Harness == "opencode" {
		if err = assistance.ReadOnlyOpenCode(env, req.Model); err != nil {
			httpx.WriteProblem(w, 409, "analysis_model_changed", err.Error(), nil)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	snapshot, err := a.deps.Product.RepositoryAnalysisSnapshot(ctx, r.PathValue("id"), req.SourceMode)
	cancel()
	if err != nil {
		httpx.WriteProblem(w, 409, "analysis_source_unavailable", "A bounded public source snapshot could not be collected. Check source visibility or use the guided interview.", nil)
		return
	}
	if snapshot.Specification.Revision != req.ExpectedRevision || snapshot.Specification.BriefRevision != req.ExpectedBriefRevision {
		httpx.WriteProblem(w, 409, "revision_conflict", "The definition changed. Reload before analysing source.", nil)
		return
	}
	prompt, err := product.RepositoryAnalysisPrompt(snapshot, req.Message)
	if err != nil {
		httpx.WriteProblem(w, 413, "analysis_input_too_large", err.Error(), nil)
		return
	}
	schema, err := assistance.RepositorySchema(product.ProjectSpecification{})
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	jobID := ids.New("assist")
	inputDir, err := a.assistDirectory(filepath.Join("assistance", "runs", jobID, "input"))
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	outputDir, err := a.assistDirectory(filepath.Join("assistance", "runs", jobID, "output"))
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	selection, _ := json.Marshal(map[string]string{"harness": req.Harness, "model": req.Model, "authMode": req.AuthMode})
	for name, b := range map[string][]byte{"prompt.txt": prompt, "schema.json": schema, "selection.json": selection, "run.py": []byte(assistance.RepositoryLauncher)} {
		if err = codexAtomicPrivate(filepath.Join(inputDir, name), b); err != nil {
			codexAccountProblem(w, err)
			return
		}
	}
	s, spec, err := a.analysisSession(r.Context(), snapshot.Specification.ProjectID, req.Harness, req.Model, req.AuthMode, env, account)
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: inputDir, ContainerPath: "/luci-input", ReadOnly: true}, backend.Mount{HostPath: outputDir, ContainerPath: "/luci-output"})
	job := assistance.Job{ID: jobID, ProjectID: snapshot.Specification.ProjectID, ExpectedRevision: req.ExpectedRevision, ExpectedBriefRevision: req.ExpectedBriefRevision, SessionID: s.ID, Kind: "repository", Harness: req.Harness, Model: req.Model, AuthMode: req.AuthMode, Source: &snapshot.Source, Status: "queued", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: "Waiting for your selected provider in the private Docker runtime."}
	if err = a.assistanceStore().Create(job); err != nil {
		a.markSessionExited(s.ID, "assistant record could not be retained")
		codexAccountProblem(w, err)
		return
	}
	jobCtx, jobCancel := context.WithTimeout(context.Background(), assistDeadline)
	a.assistMu.Lock()
	if a.assistCancels == nil {
		a.assistCancels = map[string]context.CancelFunc{}
	}
	a.assistCancels[jobID] = jobCancel
	a.assistMu.Unlock()
	go a.runAssistance(jobCtx, jobCancel, job, &spec)
	job.UpdatedAt = job.CreatedAt
	httpx.WriteJSON(w, 202, job)
}

func (a *API) handleAnalysisApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision      int  `json:"expectedRevision"`
		ExpectedBriefRevision int  `json:"expectedBriefRevision"`
		AcknowledgePartial    bool `json:"acknowledgePartial"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	job, ok := a.assistJob(w, r)
	if !ok {
		return
	}
	if !req.AcknowledgePartial {
		httpx.WriteProblem(w, 400, "analysis_review_required", "Review the partial source coverage and proposed requirements before applying this draft.", nil)
		return
	}
	if job.Kind != "repository" || job.Status != "succeeded" || job.Source == nil || job.Result == nil || string(job.Result.Proposal) == "null" || job.AppliedRevision != 0 || req.ExpectedRevision != job.ExpectedRevision || req.ExpectedBriefRevision != job.ExpectedBriefRevision {
		httpx.WriteProblem(w, 409, "analysis_not_applicable", "This analysis cannot be applied to the current draft. Review its status or run a new analysis.", nil)
		return
	}
	var proposal product.ProjectSpecification
	if a.validateAssistProposal(job, job.Result) != nil || json.Unmarshal(job.Result.Proposal, &proposal) != nil {
		httpx.WriteProblem(w, 409, "analysis_not_applicable", "The saved proposal is not valid for this project.", nil)
		return
	}
	var saved product.ProjectSpecification
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	_, err := a.assistanceStore().Update(job.ID, func(current *assistance.Job) error {
		if current.AppliedRevision != 0 || current.Status != "succeeded" {
			return product.ErrAnalysisChanged
		}
		var applyErr error
		saved, applyErr = a.deps.Product.ApplyRepositoryAnalysisDraft(ctx, job.ProjectID, req.ExpectedRevision, req.ExpectedBriefRevision, job.Source.Mode, job.Source.Digest, proposal)
		if applyErr != nil {
			return applyErr
		}
		current.AppliedRevision = saved.Revision
		current.Message = "Reviewed analysis saved as a draft. Approve it separately before designing or implementing."
		return nil
	})
	if err != nil {
		httpx.WriteProblem(w, 409, "analysis_changed", "The source or definition changed, or the draft could not be saved. Reload saved work before applying a fresh analysis.", nil)
		return
	}
	httpx.WriteJSON(w, 200, saved)
}
