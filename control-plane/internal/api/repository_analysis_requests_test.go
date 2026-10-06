package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func analysisOwnedRequest(t *testing.T, e *sessionsEnv, ctx context.Context, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, sessionsTarget(path), strings.NewReader(string(raw))).WithContext(ctx)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func analysisResponseJob(t *testing.T, w *httptest.ResponseRecorder, status int) assistance.Job {
	t.Helper()
	if w.Code != status {
		t.Fatalf("analysis HTTP %d, expected %d: %s", w.Code, status, w.Body.String())
	}
	var job assistance.Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil || job.ID == "" {
		t.Fatal("missing analysis receipt", err)
	}
	return job
}

func recoverableAnalysisRequest(key string) repositoryAnalysisRequest {
	return repositoryAnalysisRequest{SourceMode: "worktree", Harness: "codex", Model: "gpt-fixture", AuthMode: "chatgpt", AcknowledgeSourceRead: true, IdempotencyKey: key}
}

func retainedAnalysisQuestion(t *testing.T, e *sessionsEnv, projectID string) assistance.Job {
	t.Helper()
	snapshot, err := e.api.deps.Product.RepositoryAnalysisSnapshot(context.Background(), projectID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	job := assistance.Job{ID: "assist-canonical-question", ProjectID: projectID, RequestedBy: "u-reviewed-task-fixture", Status: "succeeded", Kind: "repository", Source: &snapshot.Source, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Result: &assistance.Result{Question: "Must existing deliveries survive the change?", Options: []string{"Keep existing deliveries", "No existing data"}, Rationale: "Changing storage could discard saved records.", Proposal: json.RawMessage(`null`), Findings: []assistance.Finding{{Kind: "observed", Summary: "Delivery state is defined in source.", Paths: []string{"app.ts"}}}}}
	if err := e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	return job
}

func TestRepositoryAnalysisFollowUpUsesCanonicalQuestionCurrentSourceAndExactSelection(t *testing.T) {
	e, p := repositoryFixture(t)
	prior := retainedAnalysisQuestion(t, e, p.ID)
	if err := os.WriteFile(filepath.Join(p.Path, "app.ts"), []byte("export const deliveries = ['existing record'];"), 0600); err != nil {
		t.Fatal(err)
	}
	current, err := e.api.deps.Product.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("reply-canonical-question-001")
	req.ReplyToAnalysisID, req.Message = prior.ID, "Preserve every existing delivery."
	base := "/v1/product/projects/" + p.ID + "/assist"
	started, release := make(chan backend.Spec, 1), make(chan struct{})
	e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
	job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base+"/repository", req), 202)
	if !job.ReplySourceChanged || job.ReplyToAnalysisID != prior.ID || job.RequestedBy != prior.RequestedBy || job.IdempotencyKey != req.IdempotencyKey || job.Model != req.Model || job.AuthMode != req.AuthMode || job.Source.Digest != current.Source.Digest {
		t.Fatal("follow-up ownership/selection/source receipt lost")
	}
	var spec backend.Spec
	select {
	case spec = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not dispatch")
	}
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == "/luci-input" {
			raw, err := os.ReadFile(filepath.Join(mount.HostPath, "prompt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.SplitN(string(raw), "\n\n", 2)
			var payload struct {
				Snapshot product.RepositoryAnalysisSnapshot `json:"snapshot"`
				Message  string                             `json:"userMessage"`
			}
			if len(parts) != 2 || json.Unmarshal([]byte(parts[1]), &payload) != nil || payload.Snapshot.FollowUp == nil {
				t.Fatal("follow-up prompt missing")
			}
			follow := payload.Snapshot.FollowUp
			if follow.AnalysisID != prior.ID || follow.Source.Digest != prior.Source.Digest || follow.Result.Question != prior.Result.Question || follow.Result.Rationale != prior.Result.Rationale || len(follow.Result.Options) != 2 || !follow.SourceChanged || payload.Message != req.Message {
				t.Fatal("canonical saved question or human answer changed")
			}
			if !strings.Contains(string(raw), "existing record") || strings.Contains(string(raw), "excluded-source-secret") || !strings.Contains(parts[0], "historical claims") {
				t.Fatal("fresh source boundary or drift guidance lost")
			}
		}
	}
	// Lost response / double click rejoin before consulting the leased provider.
	again := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base+"/repository", req), 200)
	if again.ID != job.ID || again.SessionID != job.SessionID {
		t.Fatal("exact retry allocated another analysis")
	}
	changed := req
	changed.Message = "Different answer"
	if w := analysisOwnedRequest(t, e, ctx, "POST", base+"/repository", changed); w.Code != 409 || !strings.Contains(w.Body.String(), "analysis_request_conflict") {
		t.Fatal("same key overwrote original request", w.Code)
	}
	got := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/repository/requests/"+req.IdempotencyKey, nil), 200)
	if got.ID != job.ID {
		t.Fatal("read-only lookup lost request")
	}
	other := resolutionViewerContext(t, "u-other-analysis-owner", p.ID)
	if w := analysisOwnedRequest(t, e, other, "GET", base+"/repository/requests/"+req.IdempotencyKey, nil); w.Code != 404 {
		t.Fatal("foreign actor read original request", w.Code)
	}
	assistOutput(t, spec, repositoryResult(t, current.Specification, "app.ts"))
	close(release)
	completed := awaitAssist(t, e, job)
	if completed.Status != "succeeded" {
		t.Fatal(completed.Message)
	}
	after, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	if after.Revision != current.Specification.Revision || after.Purpose != current.Specification.Purpose {
		t.Fatal("answer automatically applied definition")
	}
	// The new result is still bound to the snapshot it actually read.
	if err := os.WriteFile(filepath.Join(p.Path, "app.ts"), []byte("newer unrelated source"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := analysisOwnedRequest(t, e, ctx, "POST", base+"/"+job.ID+"/apply", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "acknowledgePartial": true}); w.Code != 409 {
		t.Fatal("follow-up bypassed source CAS", w.Code)
	}
	// A restarted API/store still returns the original terminal job, even though
	// its current source and provider lease have since changed.
	e.api.assistStore = nil
	recovered := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base+"/repository", req), 200)
	if recovered.ID != job.ID || recovered.Status != "succeeded" {
		t.Fatal("restart replayed inference")
	}
}

func TestRepositoryAnalysisOwnershipProjectionUsesCurrentWorkspaceActorAndNeverPersistsAuthority(t *testing.T) {
	e, p := repositoryFixture(t)
	job := retainedAnalysisQuestion(t, e, p.ID)
	remoteActor := "u-selected-cloud-account"
	req := recoverableAnalysisRequest("analysis-remote-account-001")
	job.ID = repositoryRequestJobID(remoteActor, p.ID, req.IdempotencyKey)
	job.RequestedBy, job.IdempotencyKey, job.RequestDigest = remoteActor, req.IdempotencyKey, repositoryRequestDigest(req)
	if err := e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	localCtx, remoteCtx := orchestratorExecutionActor(t), resolutionViewerContext(t, remoteActor, p.ID)
	base := "/v1/product/projects/" + p.ID + "/assist"
	for _, tc := range []struct {
		name string
		ctx  context.Context
		own  bool
	}{{"selected remote actor", remoteCtx, true}, {"different local login", localCtx, false}, {"unattributed", context.Background(), false}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{base + "/" + job.ID, base} {
				w := analysisOwnedRequest(t, e, tc.ctx, "GET", path, nil)
				if w.Code != 200 {
					t.Fatal(w.Code)
				}
				var value struct {
					OwnRequest  bool   `json:"ownRequest"`
					RequestedBy string `json:"requestedBy"`
					Jobs        []struct {
						ID          string `json:"id"`
						OwnRequest  bool   `json:"ownRequest"`
						RequestedBy string `json:"requestedBy"`
					} `json:"jobs"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
					t.Fatal(err)
				}
				if path == base {
					found := false
					for _, row := range value.Jobs {
						if row.ID == job.ID {
							found = true
							if row.OwnRequest != tc.own || row.RequestedBy != remoteActor {
								t.Fatal("history ownership used wrong account")
							}
						}
					}
					if !found {
						t.Fatal("owned history missing")
					}
				} else if value.OwnRequest != tc.own || value.RequestedBy != remoteActor {
					t.Fatal("detail ownership used wrong account")
				}
			}
		})
	}
	for _, method := range []string{"GET", "POST"} {
		path := base + "/repository"
		var body any = req
		if method == "GET" {
			path += "/requests/" + req.IdempotencyKey
			body = nil
		}
		w := analysisOwnedRequest(t, e, remoteCtx, method, path, body)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"ownRequest":true`) {
			t.Fatal("exact remote request ownership unavailable", method, w.Code)
		}
	}
	if w := analysisOwnedRequest(t, e, localCtx, "GET", base+"/repository/requests/"+req.IdempotencyKey, nil); w.Code != 404 {
		t.Fatal("foreign actor obtained remote request identity")
	}
	w := analysisOwnedRequest(t, e, remoteCtx, "POST", base+"/"+job.ID+"/cancel", map[string]any{})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ownRequest":true`) {
		t.Fatal("canonical terminal cancellation lost remote ownership")
	}
	file := filepath.Join(e.api.deps.ConversationsDir, "assistance", "jobs", job.ID+".json")
	stored, err := os.ReadFile(file)
	if err != nil || strings.Contains(string(stored), "ownRequest") {
		t.Fatal("HTTP ownership projection entered durable authority", err)
	}
}

func TestRepositoryAnalysisReplyRejectsForeignUnattributedAndDamagedResultsBeforeDispatch(t *testing.T) {
	for _, kind := range []string{"foreign-actor", "foreign-project", "legacy-unattributed", "failed", "no-question", "damaged-result"} {
		t.Run(kind, func(t *testing.T) {
			e, p := repositoryFixture(t)
			prior := retainedAnalysisQuestion(t, e, p.ID)
			_, err := e.api.assistanceStore().Update(prior.ID, func(job *assistance.Job) error {
				switch kind {
				case "foreign-actor":
					job.RequestedBy = "u-other"
				case "foreign-project":
					job.ProjectID = "p-other"
				case "legacy-unattributed":
					job.RequestedBy = ""
				case "failed":
					job.Status = "failed"
				case "no-question":
					job.Result.Question = ""
				case "damaged-result":
					job.Result.Options = []string{""}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			req := recoverableAnalysisRequest("reply-rejected-request-001")
			req.ReplyToAnalysisID = prior.ID
			req.Message = "Preserve existing records."
			e.be.mu.Lock()
			before := len(e.be.specs)
			e.be.mu.Unlock()
			w := analysisOwnedRequest(t, e, orchestratorExecutionActor(t), "POST", "/v1/product/projects/"+p.ID+"/assist/repository", req)
			if w.Code != 409 || !strings.Contains(w.Body.String(), "analysis_reply_unavailable") {
				t.Fatal(w.Code, w.Body.String())
			}
			e.be.mu.Lock()
			after := len(e.be.specs)
			e.be.mu.Unlock()
			if before != after {
				t.Fatal("invalid reply allocated inference")
			}
		})
	}
}

func TestRepositoryAnalysisMissingReceiptDoesNotAuthorizeReplacementAndDelayedRetryIsSingle(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("analysis-delayed-request-001")
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	e.api.codexAccountMu.Lock()
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() { response <- analysisOwnedRequest(t, e, ctx, "POST", base, req) }()
	w := analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "delayed request may still arrive") {
		e.api.codexAccountMu.Unlock()
		t.Fatal(w.Code)
	}
	started, release := make(chan backend.Spec, 1), make(chan struct{})
	e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
	e.api.codexAccountMu.Unlock()
	job := analysisResponseJob(t, <-response, 202)
	var spec backend.Spec
	select {
	case spec = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("missing first dispatch")
	}
	again := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200)
	if again.ID != job.ID {
		t.Fatal("delayed duplicate started another job")
	}
	current, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	assistOutput(t, spec, repositoryResult(t, current, "app.ts"))
	close(release)
	if awaitAssist(t, e, job).Status != "succeeded" {
		t.Fatal("original did not complete")
	}
}

func TestRepositoryAnalysisInterruptedAdmissionAndCorruptReceiptNeverReplay(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("analysis-interrupted-request-001")
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	// A known private input-store failure occurs after the durable no-dispatch
	// receipt, before any session or backend work is allocated.
	root, err := e.api.assistDirectory("assistance")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "runs"), []byte("unavailable"), 0600); err != nil {
		t.Fatal(err)
	}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	w := analysisOwnedRequest(t, e, ctx, "POST", base, req)
	if w.Code < 400 {
		t.Fatal("unavailable inputs accepted")
	}
	job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200)
	if job.Status != "failed" || job.SessionID != "" || !strings.Contains(job.Message, "before inference began") {
		t.Fatal("interrupted admission not explicit")
	}
	if err := os.Remove(filepath.Join(root, "runs")); err != nil {
		t.Fatal(err)
	}
	e.api.assistStore = nil
	if again := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200); again.ID != job.ID || again.Status != "failed" {
		t.Fatal("interrupted request redispatched")
	}
	file := filepath.Join(root, "jobs", job.ID+".json")
	if err := os.WriteFile(file, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		path := base
		var body any = req
		if method == "GET" {
			path += "/requests/" + req.IdempotencyKey
			body = nil
		}
		w := analysisOwnedRequest(t, e, ctx, method, path, body)
		if w.Code != 503 {
			t.Fatal("corrupt receipt treated as absent", method, w.Code)
		}
	}
	raw, _ := os.ReadFile(file)
	if string(raw) != "{" {
		t.Fatal("GET or retry repaired/overwrote corrupt receipt")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("recovery allocated a provider")
	}
}

func TestRepositoryAnalysisRequestBoundsCapacityAndReadOnlyOwnership(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	for _, key := range []string{"short", strings.Repeat("a", 129), "request/with/slash", "request-with-newline\n"} {
		req := recoverableAnalysisRequest(key)
		if w := analysisOwnedRequest(t, e, ctx, "POST", base, req); w.Code != 400 {
			t.Fatal("invalid key accepted", w.Code)
		}
	}
	req := recoverableAnalysisRequest("analysis-capacity-request-001")
	if w := analysisOwnedRequest(t, e, context.Background(), "POST", base, req); w.Code != 403 {
		t.Fatal("unattributed recoverable request accepted", w.Code)
	}
	if w := analysisOwnedRequest(t, e, context.Background(), "GET", base+"/requests/"+req.IdempotencyKey, nil); w.Code != 403 {
		t.Fatal("unattributed receipt lookup accepted", w.Code)
	}
	dir, err := e.api.assistDirectory(filepath.Join("assistance", "jobs"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4096; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("capacity-%04d", i)), []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
	}
	w := analysisOwnedRequest(t, e, ctx, "POST", base, req)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "analysis_capacity") {
		t.Fatal("capacity did not fail closed", w.Code)
	}
	if w := analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil); w.Code != 404 {
		t.Fatal("capacity check created request")
	}
}

func TestRepositoryAnalysisClarificationChainRetainsHumanDecisionsAndRejectsBrokenAncestry(t *testing.T) {
	e, p := repositoryFixture(t)
	first := retainedAnalysisQuestion(t, e, p.ID)
	second := first
	second.ID = "assist-second-canonical-question"
	second.ReplyToAnalysisID = first.ID
	second.UserMessage = "Preserve all existing delivery records."
	copyResult := *first.Result
	second.Result = &copyResult
	second.Result.Question = "Who may edit a saved delivery?"
	if err := e.api.assistanceStore().Create(second); err != nil {
		t.Fatal(err)
	}
	follow, err := e.api.repositoryFollowUp(second, first.RequestedBy, p.ID, *first.Source)
	if err != nil || len(follow.Earlier) != 1 || follow.Earlier[0].AnalysisID != first.ID || follow.UserMessage != second.UserMessage || follow.Result.Question != second.Result.Question {
		t.Fatal("earlier question/answer was lost", err)
	}
	snapshot, err := e.api.deps.Product.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.FollowUp = follow
	prompt, err := product.RepositoryAnalysisPrompt(snapshot, "Only the delivery owner may edit it.")
	if err != nil || !strings.Contains(string(prompt), second.UserMessage) || !strings.Contains(string(prompt), first.Result.Question) || !strings.Contains(string(prompt), "Only the delivery owner") {
		t.Fatal("continuation prompt lost human constraints", err)
	}
	for _, kind := range []string{"foreign", "cycle", "missing"} {
		t.Run(kind, func(t *testing.T) {
			_, err := e.api.assistanceStore().Update(first.ID, func(job *assistance.Job) error {
				job.RequestedBy = first.RequestedBy
				job.ReplyToAnalysisID = ""
				switch kind {
				case "foreign":
					job.RequestedBy = "u-another-owner"
				case "cycle":
					job.ReplyToAnalysisID = second.ID
				case "missing":
					job.ReplyToAnalysisID = "assist-missing-parent"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = e.api.repositoryFollowUp(second, first.RequestedBy, p.ID, *first.Source); err == nil {
				t.Fatal("unverified ancestry entered prompt")
			}
		})
	}
}

func TestRepositoryAnalysisReservedReceiptIsReadOnlyUntilStartupClosesAbandonedAdmission(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("analysis-reserved-request-001")
	actor := "u-reviewed-task-fixture"
	job := assistance.Job{ID: repositoryRequestJobID(actor, p.ID, req.IdempotencyKey), ProjectID: p.ID, RequestedBy: actor, IdempotencyKey: req.IdempotencyKey, RequestDigest: repositoryRequestDigest(req), Kind: "repository", Status: "reserved", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: "Dispatch is not confirmed."}
	if err := e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	for i := 0; i < 2; i++ {
		got := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200)
		if got.Status != "reserved" || got.SessionID != "" {
			t.Fatal("GET repaired or dispatched reservation")
		}
	}
	if got := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200); got.Status != "reserved" {
		t.Fatal("retry prematurely closed reservation")
	}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	e.api.assistStore = nil
	e.api.recoverAssistance()
	got := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200)
	if got.Status != "failed" || !strings.Contains(got.Message, "before inference began") {
		t.Fatal("startup did not close orphan safely")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if before != after {
		t.Fatal("startup allocated inference")
	}
}

func TestRepositoryAnalysisDefinitiveValidationFailuresRetainNonDispatchReceipts(t *testing.T) {
	for _, kind := range []string{"revision", "source", "connection", "model", "reply"} {
		t.Run(kind, func(t *testing.T) {
			e, p := repositoryFixture(t)
			ctx := orchestratorExecutionActor(t)
			req := recoverableAnalysisRequest("analysis-rejected-request-001")
			switch kind {
			case "revision":
				req.ExpectedRevision = 9
			case "source":
				req.SourceMode = "commit"
			case "connection":
				connection := "connection-missing"
				req.AgentConnectionID = &connection
			case "model":
				req.Model = "default"
			case "reply":
				req.ReplyToAnalysisID = "assist-missing-question"
				req.Message = "Keep existing data."
			}
			base := "/v1/product/projects/" + p.ID + "/assist/repository"
			e.be.mu.Lock()
			before := len(e.be.specs)
			e.be.mu.Unlock()
			w := analysisOwnedRequest(t, e, ctx, "POST", base, req)
			if w.Code < 400 {
				t.Fatal("invalid request accepted")
			}
			job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200)
			if job.Status != "failed" || job.SessionID != "" || !strings.Contains(job.Message, "No analysis was started") {
				t.Fatal("definitive rejection left ambiguous request", job.Status, job.Message)
			}
			if again := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200); again.ID != job.ID || again.Status != "failed" {
				t.Fatal("rejected key could dispatch later")
			}
			e.be.mu.Lock()
			after := len(e.be.specs)
			e.be.mu.Unlock()
			if before != after {
				t.Fatal("rejection allocated provider work")
			}
		})
	}
}

func TestRepositoryAnalysisCancelledReservationCannotDispatchAfterSourceRead(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("analysis-cancel-reserved-001")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	server, err := product.New(product.Options{ProjectsDir: e.dir, StateDir: t.TempDir(), Runner: func(ctx context.Context, executable string, args []string, dir string) (string, error) {
		if executable == "git" {
			once.Do(func() { close(entered); <-release })
		}
		return product.RunCommand(ctx, executable, args, dir)
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	e.api.deps.Product = server
	base := "/v1/product/projects/" + p.ID + "/assist"
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() { response <- analysisOwnedRequest(t, e, ctx, "POST", base+"/repository", req) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("source collection did not reach pause")
	}
	reserved := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/repository/requests/"+req.IdempotencyKey, nil), 200)
	if reserved.Status != "reserved" {
		t.Fatal("admission not visibly pending")
	}
	cancelled := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base+"/"+reserved.ID+"/cancel", map[string]any{}), 200)
	if cancelled.Status != "cancelled" {
		t.Fatal("reservation cancellation not retained")
	}
	close(release)
	got := analysisResponseJob(t, <-response, 200)
	if got.Status != "cancelled" || got.ID != reserved.ID {
		t.Fatal("late admission replaced cancellation")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if before != after {
		t.Fatal("cancelled admission dispatched provider")
	}
}

func TestRepositoryAnalysisUnconfirmedReceiptWritePreventsDispatchAndSurvivesRecovery(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	req := recoverableAnalysisRequest("analysis-write-failure-001")
	dir, err := e.api.assistDirectory(filepath.Join("assistance", "jobs"))
	if err != nil {
		t.Fatal(err)
	}
	e.api.assistStore = assistance.NewStore(func(id string) ([]byte, error) {
		return codexRead(e.api.deps.ConversationsDir, filepath.Join(dir, id+".json"), assistance.MaxJobBytes)
	}, func(id string, raw []byte) error {
		if err := writeAssistanceReceipt(filepath.Join(dir, id+".json"), raw); err != nil {
			return err
		}
		var job assistance.Job
		if err := json.Unmarshal(raw, &job); err != nil {
			return err
		}
		if job.Status == "reserved" {
			return errors.New("injected unconfirmed receipt durability")
		}
		return nil
	})
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	if w := analysisOwnedRequest(t, e, ctx, "POST", base, req); w.Code != 503 {
		t.Fatal("unconfirmed publication accepted", w.Code)
	}
	if job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200); job.Status != "reserved" || job.SessionID != "" {
		t.Fatal("unconfirmed write fabricated dispatch")
	}
	e.api.assistStore = nil
	e.api.recoverAssistance()
	if job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200); job.Status != "failed" || job.SessionID != "" {
		t.Fatal("restart replayed failed publication")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if before != after {
		t.Fatal("unconfirmed receipt allocated provider")
	}
}

func TestRepositoryAnalysisSourceConsentBindsNamedConnectionRevisionButExactRetryKeepsOriginal(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	route := saveAssistanceGateway(t, e)
	req := recoverableAnalysisRequest("analysis-bound-route-001")
	req.AuthMode = ""
	req.AgentConnectionID = &route.ID
	req.Model = route.Models.Codex
	expected := route.Revision
	req.ExpectedAgentConnectionRevision = &expected
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	current, err := e.api.deps.Product.SpecificationSnapshot(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.be.onCreate = func(spec backend.Spec) { assistOutput(t, spec, repositoryResult(t, current, "app.ts")) }
	job := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 202)
	if got := awaitAssist(t, e, job); got.Status != "succeeded" || got.AgentConnectionRevision != expected {
		t.Fatal("explicit route revision was not used")
	}
	// A teammate edits only the endpoint: same ID/model, new public revision.
	changed := map[string]any{"action": "save", "expectedRevision": route.Revision, "id": route.ID, "name": route.Name, "kind": route.Kind, "enabled": true, "endpoints": map[string]string{"responses": "https://changed.example/v1", "anthropicMessages": "https://changed.example", "chatCompletions": "https://changed.example/v1"}, "models": route.Models}
	raw, _ := json.Marshal(changed)
	w := e.do(t, "POST", "/v1/setup/agent-connections", string(raw))
	if w.Code != 200 {
		t.Fatal("route change fixture failed", w.Code, w.Body.String())
	}
	var catalogue struct{ Connections []product.AgentConnection }
	if err := json.Unmarshal(w.Body.Bytes(), &catalogue); err != nil {
		t.Fatal(err)
	}
	newRevision := catalogue.Connections[0].Revision
	if newRevision == expected {
		t.Fatal("fixture route did not change revision")
	}
	again := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 200)
	if again.ID != job.ID || again.AgentConnectionRevision != expected {
		t.Fatal("exact retry silently substituted changed route")
	}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	req.IdempotencyKey = "analysis-stale-route-002"
	w = analysisOwnedRequest(t, e, ctx, "POST", base, req)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "analysis_connection_changed") {
		t.Fatal("stale source consent used changed endpoint", w.Code, w.Body.String())
	}
	failed := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "GET", base+"/requests/"+req.IdempotencyKey, nil), 200)
	if failed.Status != "failed" || failed.Source != nil || failed.SessionID != "" || !strings.Contains(failed.Message, "renew source consent") {
		t.Fatal("stale route rejection lacked safe durable remedy")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if before != after {
		t.Fatal("stale route dispatched inference")
	}
	// Correcting metadata under the same paid-request identity is forbidden.
	req.ExpectedAgentConnectionRevision = &newRevision
	if w := analysisOwnedRequest(t, e, ctx, "POST", base, req); w.Code != 409 || !strings.Contains(w.Body.String(), "analysis_request_conflict") {
		t.Fatal("same key silently upgraded source consent", w.Code)
	}
	// A reviewed fresh request uses the new exact route; older API clients that
	// omit the optional revision retain their explicit current-connection behavior.
	for _, legacy := range []bool{false, true} {
		req.IdempotencyKey = fmt.Sprintf("analysis-current-route-%v", legacy)
		if legacy {
			req.ExpectedAgentConnectionRevision = nil
		}
		got := analysisResponseJob(t, analysisOwnedRequest(t, e, ctx, "POST", base, req), 202)
		if completed := awaitAssist(t, e, got); completed.Status != "succeeded" || completed.AgentConnectionRevision != newRevision {
			t.Fatal("current/legacy request lost chosen connection")
		}
	}
}
