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
	// Full immutable contexts remain available on the individual owned job. A
	// catalogue must not multiply 20 large snapshots into an oversized response.
	for i := range jobs {
		jobs[i].OrchestratorContext = nil
	}
	views := make([]any, len(jobs))
	for i, job := range jobs {
		views[i] = assistanceJobProjection(r, job)
	}
	httpx.WriteJSON(w, 200, map[string]any{"jobs": views})
}

// analysisSession has a new empty home and only the selected connection's
// environment. No host CLI profile, project directory or Git settings mount.
// Caller holds codexAccountMu, which also serializes analysis admission.
func (a *API) assistantSession(ctx context.Context, projectID, title, contextRef, harness string, connection product.AgentConnectionReceipt, env map[string]string, account *codexConnection) (registry.Session, backend.Spec, error) {
	model, auth := connection.Model, connection.AuthMode
	id, conversation := ids.New("s"), ids.UUID()
	s := registry.Session{ID: id, Title: title, MachineID: LocalMachineID, ProjectID: projectID, WorktreeID: assistWorktree, Harness: harness, Model: model, PermissionMode: "plan", AuthMode: auth, CodexConnectionID: codexConnectionID(account), ConversationID: conversation, Status: registry.StatusStarting, Backend: string(a.deps.Backend.Kind()), SandboxID: id, AttachToken: ids.Token(), ContextRef: contextRef, Command: "python3", Args: []string{"/luci-input/run.py"}}
	bindAssistantConnection(&s, connection)
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
	nativeDockerEndpoint(env)
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
	// Internal resolver metadata must never shadow sandboxd's LUCI_* control
	// variables. The trusted launch seam maps only selected provider settings.
	nativeSandboxEnvironment(env)
	spec := backend.Spec{ID: id, SessionID: id, Image: a.sessionImage(), Command: s.Command, Args: s.Args, Env: env, AttachToken: s.AttachToken, Resources: a.sessionResources(ctx), WorkDir: sessionWorkDir, Mounts: []backend.Mount{{HostPath: workspace, ContainerPath: sessionWorkDir}, {HostPath: home, ContainerPath: nativeAgentDir}}}
	if a.deps.CABundle != "" {
		spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: a.deps.CABundle, ContainerPath: sessionCABundlePath, ReadOnly: true})
		env["NODE_EXTRA_CA_CERTS"] = sessionCABundlePath
	}
	if err = backend.ValidateSpec(spec); err != nil {
		return s, spec, errors.New("The selected assistant configuration cannot enter the private sandbox.")
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
	var req repositoryAnalysisRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	actor := repositoryAnalysisActor(r)
	if req.IdempotencyKey != "" && !validAnalysisRequestKey(req.IdempotencyKey) {
		httpx.WriteProblem(w, 400, "invalid_analysis_request", "Use a unique bounded request key.", nil)
		return
	}
	if (req.IdempotencyKey != "" || req.ReplyToAnalysisID != "") && actor == "" {
		httpx.WriteProblem(w, 403, "analysis_actor_required", "Sign in again before sending a recoverable repository analysis request.", nil)
		return
	}
	requestDigest := repositoryRequestDigest(req)

	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "product_unavailable", "Project definitions are unavailable.", nil)
		return
	}

	// Every analysis provider is serialized here during admission only. Running
	// jobs retain their own session; other projects can use different providers.
	a.codexAccountMu.Lock()
	defer a.codexAccountMu.Unlock()
	jobID := ids.New("assist")
	if req.IdempotencyKey != "" {
		if _, err := a.deps.Product.ResolveProject(r.PathValue("id")); err != nil {
			httpx.WriteProblem(w, 404, "project_not_found", "No project in your workspace has that id.", nil)
			return
		}
		jobID = repositoryRequestJobID(actor, r.PathValue("id"), req.IdempotencyKey)
		previous, readErr := a.assistanceStore().Read(jobID)
		if readErr == nil {
			if !validRepositoryRequestJob(previous, actor, r.PathValue("id"), req.IdempotencyKey) || previous.RequestDigest != requestDigest {
				httpx.WriteProblem(w, 409, "analysis_request_conflict", "This request key already belongs to a different analysis. Recover the exact original request; do not overwrite it.", nil)
				return
			}
			writeAssistanceJob(w, r, 200, previous)
			return
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The original request record cannot be verified. Keep its identity and recover private state before retrying.", nil)
			return
		}
	}
	if err := a.repositoryAnalysisCapacity(); err != nil {
		httpx.WriteProblem(w, 409, "analysis_capacity", "Repository analysis storage is unavailable or has reached its 4096-receipt limit. Existing request identities remain recoverable. Continue editing the definition manually and ask the host operator to inspect private state; do not delete receipts to retry paid work.", nil)
		return
	}
	job := assistance.Job{ID: jobID, ProjectID: r.PathValue("id"), ExpectedRevision: req.ExpectedRevision, ExpectedBriefRevision: req.ExpectedBriefRevision, Kind: "repository", Harness: req.Harness, Model: req.Model, AuthMode: req.AuthMode, Status: "reserved", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: "Your request is retained; dispatch is not yet confirmed. Check this original request before starting another.", RequestedBy: actor, IdempotencyKey: req.IdempotencyKey, RequestDigest: requestDigest, ReplyToAnalysisID: req.ReplyToAnalysisID, UserMessage: req.Message}
	admitted := false
	failureMessage := "This request was interrupted before inference began. No analysis was started. Review your retained answer before explicitly starting a new request."
	// A valid owned key is fenced before definitive validation failures. Once
	// the failed receipt is exposed, neither this handler nor an exact retry can
	// dispatch it, even if the source or provider later becomes available.
	if req.IdempotencyKey != "" {
		if err := a.assistanceStore().Create(job); err != nil {
			httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The request receipt could not be confirmed. Keep the original key and check its status before retrying.", nil)
			return
		}
	}
	defer func() {
		if !admitted {
			a.failReservedRepositoryAnalysis(jobID, failureMessage)
		}
	}()
	failureMessage = "No analysis was started. Choose a supported model, source and current definition revisions, then explicitly send a corrected request."
	validationAuth := req.AuthMode
	if validationAuth == "" {
		validationAuth = "api"
		if req.Harness == "opencode" {
			validationAuth = "configured"
		}
	}
	if err := assistance.ValidateSelection(req.Harness, req.Model, validationAuth); err != nil {
		httpx.WriteProblem(w, 400, "invalid_analysis_provider", err.Error(), nil)
		return
	}
	if !req.AcknowledgeSourceRead || len(req.Message) > 16000 || req.ExpectedRevision < 0 || req.ExpectedBriefRevision < 0 || (req.SourceMode != "worktree" && req.SourceMode != "commit") {
		httpx.WriteProblem(w, 400, "invalid_analysis_request", "Choose a source, acknowledge sending public source to the selected provider, and use current definition revisions.", nil)
		return
	}
	if req.ReplyToAnalysisID != "" && (req.IdempotencyKey == "" || strings.TrimSpace(req.Message) == "") {
		httpx.WriteProblem(w, 400, "invalid_analysis_request", "Use a unique bounded request key and include your answer when continuing a saved question.", nil)
		return
	}
	if req.ExpectedAgentConnectionRevision != nil && (*req.ExpectedAgentConnectionRevision < 1 || req.AgentConnectionID == nil || *req.AgentConnectionID == "") {
		httpx.WriteProblem(w, 400, "invalid_analysis_request", "A saved connection revision requires its exact named connection.", nil)
		return
	}
	failureMessage = "No analysis was started. Restore the Docker runtime before explicitly sending a new request."
	if a.deps.Backend.Kind() != backend.KindDocker {
		httpx.WriteProblem(w, 409, "docker_required", "Repository analysis requires the Docker runtime.", nil)
		return
	}
	failureMessage = "No analysis was started. Choose your own verified question or begin a fresh source analysis."
	var previous assistance.Job
	if req.ReplyToAnalysisID != "" {
		var err error
		previous, err = a.repositoryReply(req.ReplyToAnalysisID, actor, r.PathValue("id"))
		if err != nil {
			httpx.WriteProblem(w, 409, "analysis_reply_unavailable", err.Error(), nil)
			return
		}
	}

	failureMessage = "No analysis was started. Reload the project history and finish or cancel its existing assistant before sending a new request."
	jobs, err := a.projectAssistance(r.PathValue("id"))
	if err != nil {
		httpx.WriteProblem(w, 404, "project_not_found", "This owned project's assistant history is unavailable.", nil)
		return
	}
	for _, job := range jobs {
		if job.ID != jobID && !assistance.Terminal(job.Status) {
			httpx.WriteProblem(w, 409, "analysis_busy", "An assistant is already running for this project. Resume or cancel it before starting another.", nil)
			return
		}
	}
	failureMessage = "No analysis was started. Check the selected provider connection and exact model, then explicitly send a new request."
	connection, env, account, err := a.resolveAssistanceConnection(r.Context(), req.Harness, req.Model, req.AuthMode, req.AgentConnectionID, true)
	if err != nil {
		if errors.Is(err, errCodexDisconnected) || errors.Is(err, errCodexBusy) {
			codexAccountProblem(w, err)
		} else {
			httpx.WriteProblem(w, 409, "analysis_connection_required", err.Error(), nil)
		}
		return
	}
	req.Model, req.AuthMode = connection.Model, connection.AuthMode
	if req.ExpectedAgentConnectionRevision != nil && connection.Revision != *req.ExpectedAgentConnectionRevision {
		failureMessage = "No analysis was started. The selected provider connection changed. Review its current endpoint and model, renew source consent and explicitly send a new request."
		httpx.WriteProblem(w, 409, "analysis_connection_changed", "The selected provider connection changed. Review its current endpoint and model and renew source consent before sending a new request.", nil)
		return
	}
	failureMessage = "No analysis was started. The selected public source could not be read; check its visibility or select an available source before sending a new request."
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	snapshot, err := a.deps.Product.RepositoryAnalysisSnapshot(ctx, r.PathValue("id"), req.SourceMode)
	cancel()
	if err != nil {
		httpx.WriteProblem(w, 409, "analysis_source_unavailable", "A bounded public source snapshot could not be collected. Check source visibility or use the guided interview.", nil)
		return
	}
	if snapshot.Specification.Revision != req.ExpectedRevision || snapshot.Specification.BriefRevision != req.ExpectedBriefRevision {
		failureMessage = "No analysis was started. The definition changed. Reload the saved definition, review your retained answer and explicitly send a new request."
		httpx.WriteProblem(w, 409, "revision_conflict", "The definition changed. Reload before analysing source.", nil)
		return
	}
	if previous.ID != "" {
		snapshot.FollowUp, err = a.repositoryFollowUp(previous, actor, r.PathValue("id"), snapshot.Source)
		if err != nil {
			failureMessage = "No analysis was started. " + err.Error()
			httpx.WriteProblem(w, 409, "analysis_reply_unavailable", err.Error(), nil)
			return
		}
	}

	failureMessage = "No analysis was started. The combined source, definition and clarification history is too large. Review and save agreed decisions in the definition before starting a fresh analysis."
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
	job.Harness, job.Model, job.AuthMode, job.Source = req.Harness, req.Model, req.AuthMode, &snapshot.Source
	if snapshot.FollowUp != nil {
		job.ReplySourceChanged = snapshot.FollowUp.SourceChanged
	}
	bindAssistantJob(&job, connection)
	failureMessage = "This request was interrupted before inference began. No analysis was started. Review your retained answer before explicitly starting a new request."
	if req.IdempotencyKey == "" {
		err = a.assistanceStore().Create(job)
	} else {
		_, err = a.assistanceStore().Update(job.ID, func(saved *assistance.Job) error {
			if saved.Status != "reserved" {
				return errRepositoryAdmissionClosed
			}
			*saved = job
			return nil
		})
	}
	if err != nil {
		if errors.Is(err, errRepositoryAdmissionClosed) {
			a.returnClosedRepositoryAnalysis(w, r, job.ID)
			return
		}
		httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The request receipt could not be confirmed. Keep the original key and check its status before retrying.", nil)
		return
	}
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
	s, spec, err := a.assistantSession(r.Context(), snapshot.Specification.ProjectID, "Repository analysis", "luci:repository-analysis", req.Harness, connection, env, account)
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: inputDir, ContainerPath: "/luci-input", ReadOnly: true}, backend.Mount{HostPath: outputDir, ContainerPath: "/luci-output"})
	if r.Context().Err() != nil {
		a.markSessionExited(s.ID, "analysis request cancelled before dispatch")
		return
	}
	job, err = a.assistanceStore().Update(jobID, func(saved *assistance.Job) error {
		if saved.Status != "reserved" {
			return errRepositoryAdmissionClosed
		}
		saved.SessionID, saved.Status = s.ID, "queued"
		saved.Message = "Waiting for your selected provider in the private Docker runtime."
		return nil
	})
	if err != nil {
		a.markSessionExited(s.ID, "assistant dispatch record could not be confirmed")
		if errors.Is(err, errRepositoryAdmissionClosed) {
			a.returnClosedRepositoryAnalysis(w, r, jobID)
			return
		}
		httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The request dispatch record could not be confirmed. Keep the original key and check its status before retrying.", nil)
		return
	}

	jobCtx, jobCancel := context.WithTimeout(context.Background(), assistDeadline)
	a.assistMu.Lock()
	if a.assistCancels == nil {
		a.assistCancels = map[string]context.CancelFunc{}
	}
	a.assistCancels[jobID] = jobCancel
	a.assistMu.Unlock()
	admitted = true
	go a.runAssistance(jobCtx, jobCancel, job, &spec)
	job.UpdatedAt = job.CreatedAt
	writeAssistanceJob(w, r, 202, job)
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
