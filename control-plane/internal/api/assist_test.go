package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

func independentSignIn(t *testing.T, e *sessionsEnv) string {
	t.Helper()
	e.api.registerAssistance(e.mux)
	e.api.deps.Worktrees = worktree.Static{}
	done := sessionsWatchDone(t)
	s := sessionsDecodeOne(t, e.do(t, "POST", codexSignInRoute, `{}`), 201)
	sessionsAwaitWatch(t, done)
	if s.ProjectID != "" || s.BranchID != "" || s.WorktreeID != assistWorktree || !s.AuthOnly || s.AuthMode != "chatgpt" {
		t.Fatal("sign-in incorrectly requires a project")
	}
	spec := e.be.spec(t, 0)
	if spec.Command != "codex" || !strings.Contains(strings.Join(spec.Args, " "), "login --device-auth") {
		t.Fatal("not an official device sign-in")
	}
	for _, m := range spec.Mounts {
		rel, err := filepath.Rel(e.dir, m.HostPath)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			t.Fatal("sign-in mounted a project")
		}
	}
	nativeWrite(t, nativeStateMount(t, spec), "auth.json", codexFixtureAuth)
	sessionsDecodeOne(t, e.do(t, "DELETE", "/v1/sessions/"+s.ID, ""), 200)
	codexTestAdopt(t, e, s.ID)
	listed := e.do(t, "GET", "/v1/sessions", "")
	if !strings.Contains(listed.Body.String(), s.ID) {
		t.Fatal("setup sign-in cannot be reopened")
	}
	return s.ID
}

func assistanceFixture(t *testing.T) (*sessionsEnv, project.Project) {
	t.Helper()
	e := codexTestEnv(t)
	sessionsStubReady(t, nil)
	independentSignIn(t, e)
	e.api.deps.ProjectsDir = e.dir
	p, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: e.dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	e.api.deps.Product = p
	p.Register(e.mux)
	dir := filepath.Join(e.dir, "definition-project")
	if err = os.MkdirAll(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	return e, project.Project{ID: project.ProjectID(dir), Name: "definition-project", Path: dir}
}

func decodeAssist(t *testing.T, e *sessionsEnv, method, path, body string, status int) assistance.Job {
	t.Helper()
	w := e.do(t, method, path, body)
	if w.Code != status {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
	}
	var job assistance.Job
	if json.Unmarshal(w.Body.Bytes(), &job) != nil {
		t.Fatal("invalid job response")
	}
	return job
}

func awaitAssist(t *testing.T, e *sessionsEnv, job assistance.Job) assistance.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got := decodeAssist(t, e, "GET", "/v1/product/projects/"+job.ProjectID+"/assist/"+job.ID, "", 200)
		s, _ := e.st.Get(job.SessionID)
		e.api.assistMu.Lock()
		_, running := e.api.assistCancels[job.ID]
		e.api.assistMu.Unlock()
		if assistance.Terminal(got.Status) && s.Status == registry.StatusExited && !running {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("assistant did not finish and release its owned session")
	return assistance.Job{}
}

func assistOutput(t *testing.T, spec backend.Spec, result []byte) {
	t.Helper()
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == "/luci-output" {
			if err := os.WriteFile(filepath.Join(mount.HostPath, "result.json"), result, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(mount.HostPath, "exit.json"), []byte(`{"exitCode":0}`), 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("missing isolated output mount")
}

func TestIndependentSignInAndDefineAssistantPreserveSubscriptionIsolationAndExplicitAcceptance(t *testing.T) {
	e, p := assistanceFixture(t)
	base := "/v1/product/projects/" + p.ID + "/assist"
	before, err := e.api.deps.Product.SpecificationSnapshot(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w := e.do(t, "POST", "/v1/setup/agents/openai", `{"apiKey":"fixture-api-key-must-not-be-used"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	started := make(chan backend.Spec, 1)
	release := make(chan struct{})
	e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
	job := decodeAssist(t, e, "POST", base, `{"expectedRevision":0,"message":"Help clarify who can approve a delivery."}`, 202)
	var spec backend.Spec
	select {
	case spec = <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("assistant Docker creation not reached")
	}
	if spec.Command != "python3" || spec.Env["OPENAI_API_KEY"] != "" || spec.Env["CODEX_API_KEY"] != "" {
		t.Fatal("assistant used a host process or API-key fallback")
	}
	for _, mount := range spec.Mounts {
		rel, _ := filepath.Rel(e.dir, mount.HostPath)
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("assistant mounted product source")
		}
	}
	if got, _ := os.ReadFile(filepath.Join(nativeStateMount(t, spec), "auth.json")); string(got) != codexFixtureAuth {
		t.Fatal("saved subscription was not copied to a private job home")
	}
	if code := sessionsProblemCode(t, e.do(t, "POST", base, `{"expectedRevision":0,"message":"Overlap"}`), 409); code != "codex_account_busy" {
		t.Fatal("credential rotation lease overlap was allowed")
	}
	proposal := before
	proposal.Purpose = "A delivery approval app"
	proposal.Status = "draft"
	proposalJSON, _ := json.Marshal(proposal)
	resultJSON, _ := json.Marshal(assistance.Result{Question: "Who approves the delivery?", Options: []string{"Manager", "Customer"}, Rationale: "Approval changes the permissions.", Proposal: proposalJSON})
	assistOutput(t, spec, resultJSON)
	nativeWrite(t, nativeStateMount(t, spec), "auth.json", codexFixtureRotated)
	close(release)
	got := awaitAssist(t, e, job)
	if got.Status != "succeeded" || got.Result == nil || got.Result.Question != "Who approves the delivery?" {
		t.Fatalf("completion not retained: %#v", got)
	}
	after, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
	afterJSON, _ := json.Marshal(after)
	beforeJSON, _ := json.Marshal(before)
	if string(afterJSON) != string(beforeJSON) {
		t.Fatal("AI proposal silently changed the project")
	}
	connection, err := e.api.readCodexConnection()
	if err != nil || connection.ActiveSessionID != "" || string(connection.Auth) != codexFixtureRotated {
		t.Fatal("rotated subscription was not retained and released")
	}
	other := codexTestEnv(t)
	other.api.registerAssistance(other.mux)
	sessionsProblemCode(t, other.do(t, "GET", base+"/"+job.ID, ""), 404)
	reopened, err := New(e.api.deps)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := reopened.assistanceStore().Read(job.ID)
	if err != nil || saved.Status != "succeeded" || saved.Result == nil {
		t.Fatal("completed job did not survive API restart")
	}
	public, _ := json.Marshal(got)
	for _, secret := range []string{"fixture-access", "fixture-refresh", "fixture-api-key", "attachToken"} {
		if strings.Contains(string(public), secret) {
			t.Fatal("assistant response exported credentials")
		}
	}
	if w := e.do(t, "POST", base, `{"expectedRevision":1,"message":"Stale definition"}`); w.Code != 409 {
		t.Fatal("stale expected revision was accepted")
	}
	second := decodeAssist(t, e, "POST", base, `{"expectedRevision":0,"message":"The manager approves it."}`, 202)
	select {
	case spec = <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("follow-up did not start")
	}
	foundHistory := false
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == "/luci-input" {
			prompt, err := os.ReadFile(filepath.Join(mount.HostPath, "prompt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			foundHistory = strings.Contains(string(prompt), "Who approves the delivery?") && strings.Contains(string(prompt), "Help clarify who can approve") && strings.Contains(string(prompt), "The manager approves it.")
		}
	}
	if !foundHistory {
		t.Fatal("the one-question interview lost its previous question or answer")
	}
	assistOutput(t, spec, []byte(`{"question":"Who can view the approval?","options":[],"rationale":"Clarify visibility next.","proposal":null}`))
	if got := awaitAssist(t, e, second); got.Status != "succeeded" {
		t.Fatal("follow-up turn failed")
	}
	if w := e.do(t, "DELETE", codexAccountRoute, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if code := sessionsProblemCode(t, e.do(t, "POST", base, `{"expectedRevision":0,"message":"No subscription","authMode":"chatgpt"}`), 409); code != "codex_account_required" {
		t.Fatal("missing subscription silently used saved API key")
	}
}

func TestDefineAssistantCancellationAndInvalidProposalCannotApplyOrLeaveActiveCredentials(t *testing.T) {
	for _, cancelJob := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel", false: "foreign-proposal"}[cancelJob], func(t *testing.T) {
			e, p := assistanceFixture(t)
			base := "/v1/product/projects/" + p.ID + "/assist"
			started := make(chan backend.Spec, 1)
			release := make(chan struct{})
			e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
			job := decodeAssist(t, e, "POST", base, `{"expectedRevision":0,"message":"Clarify my idea."}`, 202)
			var spec backend.Spec
			select {
			case spec = <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("assistant did not start")
			}
			if cancelJob {
				decodeAssist(t, e, "POST", base+"/"+job.ID+"/cancel", `{}`, 200)
			}
			proposal, _ := e.api.deps.Product.SpecificationSnapshot(p.ID)
			proposal.ProjectID = "p-foreign"
			data, _ := json.Marshal(proposal)
			result, _ := json.Marshal(assistance.Result{Question: "Continue?", Options: []string{}, Rationale: "Fixture", Proposal: data})
			assistOutput(t, spec, result)
			close(release)
			got := awaitAssist(t, e, job)
			want := "failed"
			if cancelJob {
				want = "cancelled"
			}
			if got.Status != want || got.Result != nil {
				t.Fatalf("invalid/cancelled response became accepted: %#v", got)
			}
			connection, err := e.api.readCodexConnection()
			if err != nil || connection.ActiveSessionID != "" {
				t.Fatal("credential lease left active")
			}
			if e.be.destroyCount() < 2 {
				t.Fatal("assistant's Docker session was not destroyed")
			}
		})
	}
}

func TestAssistantReportsOnlyObservedOwnedOOMWithoutProviderOutput(t *testing.T) {
	for _, oom := range []bool{true, false} {
		t.Run(map[bool]string{true: "observed-oom", false: "unproven-exit"}[oom], func(t *testing.T) {
			e, p := assistanceFixture(t)
			started := make(chan backend.Spec, 1)
			release := make(chan struct{})
			e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
			job := decodeAssist(t, e, "POST", "/v1/product/projects/"+p.ID+"/assist", `{"expectedRevision":0,"message":"Clarify my idea."}`, 202)
			spec := <-started
			e.be.mu.Lock()
			e.be.observed = &backend.Sandbox{ID: job.SessionID, Ref: "ref-" + job.SessionID, Kind: backend.KindDocker, State: backend.StateStopped, OOMKilled: oom}
			e.be.mu.Unlock()
			for _, mount := range spec.Mounts {
				if mount.ContainerPath == "/luci-output" {
					if err := os.WriteFile(filepath.Join(mount.HostPath, "exit.json"), []byte(`{"exitCode":137}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			close(release)
			got := awaitAssist(t, e, job)
			session, err := e.st.Get(job.SessionID)
			if err != nil || got.Status != "failed" || strings.Contains(got.Message, "provider output") {
				t.Fatal(got, session, err)
			}
			if oom {
				if got.FailureCode != "memory_limit" || got.Message != assistMemoryLimitMessage || session.ExitReason != assistMemoryLimitMessage {
					t.Fatal(got, session)
				}
			} else if got.FailureCode != "" || got.Message == assistMemoryLimitMessage || session.ExitReason == assistMemoryLimitMessage {
				t.Fatal(got, session)
			}
		})
	}
}

func TestDefineAssistantRecoveryOnlyObservesExistingJobAndNeverRepeatsInference(t *testing.T) {
	e, p := assistanceFixture(t)
	account, err := e.api.readCodexConnection()
	if err != nil {
		t.Fatal(err)
	}
	e.api.codexAccountMu.Lock()
	s, spec, err := e.api.utilitySession(context.Background(), p.ID, "Interrupted assistant", account, false)
	e.api.codexAccountMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	job := assistance.Job{ID: "assist-recovered", ProjectID: p.ID, ExpectedRevision: 0, SessionID: s.ID, Status: "running", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err = e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	output, err := e.api.assistDirectory(filepath.Join("assistance", "runs", job.ID, "output"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := e.api.assistDirectory(filepath.Join("assistance", "runs", job.ID, "input"))
	if err != nil {
		t.Fatal(err)
	}
	if err = codexAtomicPrivate(filepath.Join(input, "message.txt"), []byte("Help define the audience.")); err != nil {
		t.Fatal(err)
	}
	spec.Mounts = append(spec.Mounts, backend.Mount{HostPath: output, ContainerPath: "/luci-output"})
	assistOutput(t, spec, []byte(`{"question":"Who uses it?","options":[],"rationale":"Identify the audience.","proposal":null}`))
	before := len(e.be.specs)
	e.api.recoverAssistance()
	got := awaitAssist(t, e, job)
	if got.Status != "succeeded" || len(e.be.specs) != before {
		t.Fatal("recovery reran inference or lost the completed result")
	}
}
