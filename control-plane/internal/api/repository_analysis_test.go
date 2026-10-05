package api

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func repositoryFixture(t *testing.T) (*sessionsEnv, project.Project) {
	t.Helper()
	e, p := assistanceFixture(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("actual Git required")
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "no-global-config"))
	command := exec.Command("git", "init", p.Path)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for path, text := range map[string]string{"README.md": "An app to track a customer's delivery.", "app.ts": "export const deliveries = [];", ".gitignore": "ignored.ts\n", "ignored.ts": "excluded-source-secret", ".env": "excluded-source-secret"} {
		if err := os.WriteFile(filepath.Join(p.Path, path), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return e, p
}

func TestRepositoryAnalysisRecoveryObservesOwnedOutputAndNeverStartsAnotherInference(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved-output", true: "expired"}[expired], func(t *testing.T) {
			e, p := repositoryFixture(t)
			snapshot, err := e.api.deps.Product.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
			if err != nil {
				t.Fatal(err)
			}
			e.api.codexAccountMu.Lock()
			s, spec, err := e.api.analysisSession(context.Background(), p.ID, "codex", "gpt-fixture", "api", map[string]string{"OPENAI_API_KEY": "fixture-selected-key"}, nil)
			e.api.codexAccountMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			created := time.Now().UTC()
			if expired {
				created = created.Add(-assistDeadline - time.Minute)
			}
			job := assistance.Job{ID: "assist-owned-recovery", ProjectID: p.ID, ExpectedRevision: 0, ExpectedBriefRevision: 0, SessionID: s.ID, Kind: "repository", Harness: "codex", Model: "gpt-fixture", AuthMode: "api", Source: &snapshot.Source, Status: "running", CreatedAt: created.Format(time.RFC3339Nano)}
			if err = e.api.assistanceStore().Create(job); err != nil {
				t.Fatal(err)
			}
			output, err := e.api.assistDirectory(filepath.Join("assistance", "runs", job.ID, "output"))
			if err != nil {
				t.Fatal(err)
			}
			spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: output, ContainerPath: "/luci-output"})
			assistOutput(t, spec, repositoryResult(t, snapshot.Specification, "app.ts"))
			before := len(e.be.specs)
			e.api.recoverAssistance()
			got := awaitAssist(t, e, job)
			want := "succeeded"
			if expired {
				want = "failed"
			}
			if got.Status != want || len(e.be.specs) != before {
				t.Fatal("recovery restarted inference or ignored existing deadline", got.Message)
			}
			if expired && got.Result != nil {
				t.Fatal("expired response was accepted")
			}
		})
	}
}

func repositoryRequest(harness, model, auth string) string {
	b, _ := json.Marshal(map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "sourceMode": "worktree", "harness": harness, "model": model, "authMode": auth, "acknowledgeSourceRead": true})
	return string(b)
}

func repositoryResult(t *testing.T, p product.ProjectSpecification, path string) []byte {
	t.Helper()
	p.StartingPoint = "existing"
	p.Status = "draft"
	p.Purpose = "Track deliveries"
	p.Audience = "Customers"
	p.Requirements = []product.SpecificationRequirement{{ID: "AC-track", Title: "Save a delivery", Priority: "must", Acceptance: "Refreshing retains the saved delivery"}}
	proposal, _ := json.Marshal(p)
	b, _ := json.Marshal(assistance.Result{Question: "Who can share a delivery?", Options: []string{}, Rationale: "This source sample is partial; no runtime behavior was tested.", Proposal: proposal, Findings: []assistance.Finding{{Kind: "observed", Summary: "Delivery state is defined in source.", Paths: []string{path}}}})
	return b
}

func TestRepositoryAnalysisExplicitSubscriptionUsesReadOnlySourceAndRecoverableReview(t *testing.T) {
	e, p := repositoryFixture(t)
	base := "/v1/product/projects/" + p.ID + "/assist"
	before, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	connection, _ := e.api.readCodexConnection()
	if w := e.do(t, "POST", "/v1/setup/agents/openai", `{"apiKey":"fixture-api-key-not-selected"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	started := make(chan backend.Spec, 1)
	release := make(chan struct{})
	e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
	job := decodeAssist(t, e, "POST", base+"/repository", repositoryRequest("codex", "gpt-fixture", "chatgpt"), 202)
	var spec backend.Spec
	select {
	case spec = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("Docker analysis not started")
	}
	if job.Harness != "codex" || job.Model != "gpt-fixture" || job.AuthMode != "chatgpt" || job.Source == nil || !job.Source.Partial || spec.Env["OPENAI_API_KEY"] != "" {
		t.Fatal("chosen connection/provenance not retained")
	}
	for _, mount := range spec.Mounts {
		rel, _ := filepath.Rel(e.dir, mount.HostPath)
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("project was mounted into analysis")
		}
		if mount.ContainerPath == "/luci-input" {
			prompt, _ := os.ReadFile(filepath.Join(mount.HostPath, "prompt.txt"))
			if !mount.ReadOnly || !strings.Contains(string(prompt), "export const deliveries") || strings.Contains(string(prompt), "excluded-source-secret") || strings.Contains(string(prompt), "fixture-api-key") {
				t.Fatal("source snapshot boundary failed")
			}
		}
	}
	if w := e.do(t, "POST", base+"/repository", repositoryRequest("codex", "gpt-fixture", "chatgpt")); w.Code != 409 {
		t.Fatal("duplicate analysis job started")
	}
	listed := e.do(t, "GET", base, "")
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), job.ID) || strings.Contains(listed.Body.String(), `"result"`) {
		t.Fatal("live resumable metadata missing")
	}
	assistOutput(t, spec, repositoryResult(t, before, "app.ts"))
	close(release)
	got := awaitAssist(t, e, job)
	if got.Status != "succeeded" || got.Result == nil {
		t.Fatal("valid source findings were lost", got.Message)
	}
	after, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	if after.Revision != before.Revision || after.Purpose != before.Purpose {
		t.Fatal("analysis automatically wrote a draft")
	}
	history, err := e.api.assistHistory(p.ID)
	if err != nil || len(history) != 0 {
		t.Fatal("repository analysis polluted interview history")
	}
	public, _ := json.Marshal(got)
	for _, secret := range []string{"fixture-access", "fixture-refresh", "fixture-api-key", "attachToken", "excluded-source-secret"} {
		if strings.Contains(string(public), secret) {
			t.Fatal("private data exported")
		}
	}
	current, _ := e.api.readCodexConnection()
	if current.ID != connection.ID || current.ActiveSessionID != "" {
		t.Fatal("subscription lease lost/not released")
	}
	reopened, err := New(e.api.deps)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := reopened.projectAssistance(p.ID)
	if err != nil || len(metadata) != 1 || metadata[0].ID != job.ID {
		t.Fatal("completed analysis did not survive restart")
	}
	if w := e.do(t, "POST", base+"/"+job.ID+"/apply", `{"expectedRevision":0,"expectedBriefRevision":0,"acknowledgePartial":false}`); w.Code != 400 {
		t.Fatal("unreviewed draft applied")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "app.ts"), []byte("export const deliveries = [1];"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := e.do(t, "POST", base+"/"+job.ID+"/apply", `{"expectedRevision":0,"expectedBriefRevision":0,"acknowledgePartial":true}`); w.Code != 409 {
		t.Fatal("changed source was applied")
	}
	os.WriteFile(filepath.Join(p.Path, "app.ts"), []byte("export const deliveries = [];"), 0600)
	w := e.do(t, "POST", base+"/"+job.ID+"/apply", `{"expectedRevision":0,"expectedBriefRevision":0,"acknowledgePartial":true}`)
	var saved product.ProjectSpecification
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &saved) != nil || saved.Status != "draft" || saved.Revision != 1 {
		t.Fatalf("reviewed draft save failed: %d %s", w.Code, w.Body.String())
	}
	if w := e.do(t, "POST", base+"/"+job.ID+"/apply", `{"expectedRevision":0,"expectedBriefRevision":0,"acknowledgePartial":true}`); w.Code != 409 {
		t.Fatal("proposal applied twice")
	}
	other := codexTestEnv(t)
	other.api.registerAssistance(other.mux)
	if w := other.do(t, "GET", base+"/"+job.ID, ""); w.Code != 404 {
		t.Fatal("other account read analysis")
	}
}

func TestRepositoryAnalysisExplicitProvidersCannotFallbackOrReceiveForeignCredentials(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			e, p := repositoryFixture(t)
			base := "/v1/product/projects/" + p.ID + "/assist/repository"
			model, auth := "model-fixture", "api"
			if provider == "opencode" {
				model, auth = "luci-local/model-fixture", "configured"
			}
			if w := e.do(t, "POST", base, repositoryRequest(provider, model, auth)); w.Code != 409 {
				t.Fatal("missing configured provider used subscription fallback")
			}
			var route, body string
			switch provider {
			case "codex":
				route, body = "openai", `{"apiKey":"fixture-openai-selected-key"}`
			case "claude-code":
				route, body = "anthropic", `{"apiKey":"fixture-anthropic-selected-key"}`
			case "opencode":
				route, body = "local", `{"baseURL":"http://localhost:11434/v1","model":"model-fixture","apiKey":"fixture-local-selected-key"}`
			}
			if w := e.do(t, "POST", "/v1/setup/agents/"+route, body); w.Code != 200 {
				t.Fatalf("provider configure: %d %s", w.Code, w.Body.String())
			}
			before, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
			connection, _ := e.api.readCodexConnection()
			started := make(chan backend.Spec, 1)
			e.be.onCreate = func(spec backend.Spec) { started <- spec; assistOutput(t, spec, repositoryResult(t, before, "app.ts")) }
			job := decodeAssist(t, e, "POST", base, repositoryRequest(provider, model, auth), 202)
			var spec backend.Spec
			select {
			case spec = <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("selected provider not started")
			}
			if job.Harness != provider || job.Model != model || job.AuthMode != auth {
				t.Fatal("record substituted provider")
			}
			if _, err := os.Stat(filepath.Join(nativeStateMount(t, spec), "auth.json")); !os.IsNotExist(err) {
				t.Fatal("API provider received subscription auth")
			}
			if provider == "opencode" && (!strings.Contains(spec.Env["OPENCODE_CONFIG_CONTENT"], "host.docker.internal") || spec.Env["OPENCODE_PERMISSION"] != `{"*":"deny"}`) {
				t.Fatal("configured local model/tools boundary missing")
			}
			if got := awaitAssist(t, e, job); got.Status != "succeeded" {
				t.Fatal("selected provider failed validated result")
			}
			current, _ := e.api.readCodexConnection()
			if current.ActiveSessionID != connection.ActiveSessionID || string(current.Auth) != string(connection.Auth) {
				t.Fatal("non-subscription analysis modified saved subscription")
			}
		})
	}
}

func TestRepositoryAnalysisRejectsForgedEvidenceAndProviderModes(t *testing.T) {
	e, p := repositoryFixture(t)
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	for _, input := range []string{repositoryRequest("shell", "bash", "api"), repositoryRequest("codex", "default", "chatgpt"), repositoryRequest("codex", "real-model", "configured"), repositoryRequest("claude-code", "sonnet", "chatgpt")} {
		if w := e.do(t, "POST", base, input); w.Code != 400 {
			t.Fatal("invalid/guessed model mode admitted")
		}
	}
	before, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	e.be.onCreate = func(spec backend.Spec) { assistOutput(t, spec, repositoryResult(t, before, ".env")) }
	job := decodeAssist(t, e, "POST", base, repositoryRequest("codex", "gpt-fixture", "chatgpt"), 202)
	if got := awaitAssist(t, e, job); got.Status != "failed" || got.Result != nil {
		t.Fatal("unread private source became evidence")
	}
}

func TestRepositoryAnalysisAdmissionRequiresConsentCurrentRevisionsAndExactSavedLocalModel(t *testing.T) {
	e, p := repositoryFixture(t)
	base := "/v1/product/projects/" + p.ID + "/assist/repository"
	beforeSessions := len(e.be.specs)
	connection, err := e.api.readCodexConnection()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		strings.Replace(repositoryRequest("codex", "chosen-model", "chatgpt"), `"acknowledgeSourceRead":true`, `"acknowledgeSourceRead":false`, 1),
		repositoryRequest("codex", "", "chatgpt"),
	} {
		if w := e.do(t, "POST", base, input); w.Code != 400 {
			t.Fatal("missing source consent or model admitted", w.Code)
		}
	}
	for _, field := range []string{"expectedRevision", "expectedBriefRevision"} {
		input := strings.Replace(repositoryRequest("codex", "chosen-model", "chatgpt"), `"`+field+`":0`, `"`+field+`":1`, 1)
		if w := e.do(t, "POST", base, input); w.Code != 409 {
			t.Fatal("stale definition admitted", field, w.Code)
		}
	}
	if w := e.do(t, "POST", "/v1/setup/agents/local", `{"baseURL":"http://localhost:11434/v1","model":"saved-model","apiKey":"fixture-local-key"}`); w.Code != 200 {
		t.Fatal("fixture connection not saved", w.Code)
	}
	for _, model := range []string{"saved-model", "luci-local/previous-model", "other-provider/saved-model"} {
		if w := e.do(t, "POST", base, repositoryRequest("opencode", model, "configured")); w.Code != 409 {
			t.Fatal("requested local model differed from the saved provider", model, w.Code)
		}
	}
	if len(e.be.specs) != beforeSessions {
		t.Fatal("rejected analysis created a provider session")
	}
	listed := e.do(t, "GET", strings.TrimSuffix(base, "/repository"), "")
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), `"jobs":[]`) {
		t.Fatal("rejected analysis retained an executable job")
	}
	current, err := e.api.readCodexConnection()
	if err != nil || current.ActiveSessionID != connection.ActiveSessionID || string(current.Auth) != string(connection.Auth) {
		t.Fatal("rejected analysis leased or changed the subscription connection")
	}
}

func TestRepositoryAnalysisCancellationStopsOwnedUtilityAndCannotApplyDraft(t *testing.T) {
	e, p := repositoryFixture(t)
	base := "/v1/product/projects/" + p.ID + "/assist"
	before, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	started := make(chan backend.Spec, 1)
	release := make(chan struct{})
	e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
	job := decodeAssist(t, e, "POST", base+"/repository", repositoryRequest("codex", "gpt-fixture", "chatgpt"), 202)
	var spec backend.Spec
	select {
	case spec = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("utility not started")
	}
	decodeAssist(t, e, "POST", base+"/"+job.ID+"/cancel", `{}`, 200)
	assistOutput(t, spec, repositoryResult(t, before, "app.ts"))
	close(release)
	if got := awaitAssist(t, e, job); got.Status != "cancelled" || got.Result != nil {
		t.Fatal("cancelled response was retained")
	}
	current, _ := e.api.readCodexConnection()
	if current.ActiveSessionID != "" {
		t.Fatal("cancelled utility retained subscription lease")
	}
	if w := e.do(t, "POST", base+"/"+job.ID+"/apply", `{"expectedRevision":0,"expectedBriefRevision":0,"acknowledgePartial":true}`); w.Code != 409 {
		t.Fatal("cancelled job applied a draft")
	}
	after, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	if after.Revision != before.Revision || after.Purpose != before.Purpose {
		t.Fatal("cancelled analysis wrote project context")
	}
}
