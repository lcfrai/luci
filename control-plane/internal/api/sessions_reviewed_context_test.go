package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// This follows the authority actually delivered to the agent. The legacy
// prompt names the writable checkout; the reviewed bundle names a distinct
// mounted directory. Running the same witness before the fix therefore fails
// on changed context bytes, rather than merely expecting a new API field.
func TestReviewedSessionKeepsDeliveredContextWhileCollaboratorsAmendCheckout(t *testing.T) {
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	connection := sessionSaveConnection(t, env, 0, "", "codex", true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "agentConnectionId": connection.ID, "implementationContext": selection})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	saved, err := env.st.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	const authority = "/home/dev/.luci-reviewed-context"
	deliveredRoot := p.Path
	readOnly := false
	if strings.Contains(saved.OpeningPrompt, "Reviewed context directory: "+authority) {
		deliveredRoot = ""
		for _, mount := range env.be.specs[0].Mounts {
			if mount.ContainerPath == authority {
				deliveredRoot, readOnly = mount.HostPath, mount.ReadOnly
			}
		}
		if deliveredRoot == "" {
			t.Fatal("reviewed prompt named an authority that the sandbox did not receive")
		}
	} else if !strings.Contains(saved.OpeningPrompt, "Read LUCI.md, .luci/plan.json") {
		t.Fatal("reviewed prompt has no supported explicit context authority")
	}
	paths := []string{"LUCI.md", ".luci/plan.json", ".luci/specification.json", ".luci/specification.md", ".luci/design-handoff.json", ".luci/design-handoff.md", ".luci/design.json"}
	original := map[string][]byte{}
	for _, name := range paths {
		data, err := os.ReadFile(filepath.Join(deliveredRoot, name))
		if err != nil {
			t.Fatal("initial delivered context", name, err)
		}
		original[name] = data
	}
	indexBefore, err := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(p.Path, "agent-working-file.js")
	if err := os.WriteFile(appPath, []byte("// independent agent work remains writable\n"), 0600); err != nil {
		t.Fatal(err)
	}
	request := func(method, suffix string, body any, out any) {
		t.Helper()
		data := ""
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			data = string(b)
		}
		w := env.do(t, method, "/v1/product/projects/"+p.ID+suffix, data)
		if w.Code != 200 {
			t.Fatal(suffix, w.Code, w.Body.String())
		}
		if out != nil && json.Unmarshal(w.Body.Bytes(), out) != nil {
			t.Fatal("invalid saved context response")
		}
	}
	var spec product.ProjectSpecification
	request("GET", "/specification", nil, &spec)
	spec.Purpose = "Collaboratively amended purpose B"
	spec.Requirements[0].Acceptance = "Amended outcome B includes an explicit recovery action."
	request("POST", "/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, &spec)
	var design product.ProjectDesignPlan
	request("GET", "/design-plan", nil, &design)
	design.Screens[0].Purpose = "Amended screen B makes recovery visible"
	request("POST", "/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, &design)
	// The newer plan is deliberately an unreviewed draft, as it would be
	// while collaborators are still refining B. Do not manufacture approval.
	request("POST", "/plan", map[string]any{"expectedRevision": selection.PlanRevision, "tasks": []product.ProjectTask{{ID: "save-note", Title: "Amended implementation B", Status: "todo"}}}, nil)
	canonical, err := os.ReadFile(filepath.Join(p.Path, ".luci", "specification.json"))
	if err != nil || !bytes.Contains(canonical, []byte(spec.Purpose)) {
		t.Fatal("the current checkout must expose the actual collaborator amendment", err)
	}
	for _, name := range paths {
		after, err := os.ReadFile(filepath.Join(deliveredRoot, name))
		if err != nil || !bytes.Equal(after, original[name]) {
			t.Errorf("the same running session reread amended %s instead of its original reviewed bytes: %v", name, err)
		}
	}
	if deliveredRoot == p.Path || !readOnly {
		t.Error("reviewed context must be delivered in a separate read-only mount")
	}
	indexAfter, err := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
	if err != nil || !bytes.Equal(indexBefore, indexAfter) {
		t.Fatal("context delivery or amendment rewrote the Git index", err)
	}
	app, err := os.ReadFile(appPath)
	if err != nil || string(app) != "// independent agent work remains writable\n" {
		t.Fatal("context amendment replaced independent coding work", err)
	}
	after, err := env.st.Get(created.ID)
	if err != nil || after.Status != saved.Status || env.be.destroyCount() != 0 || len(env.be.specs) != 1 {
		t.Fatal("amending collaborative context must not stop, replace or allocate a coding child", err)
	}
}

func createReviewedFixture(t *testing.T, provider string) (*sessionsEnv, registry.Session, <-chan string) {
	t.Helper()
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	connection := sessionSaveConnection(t, env, 0, "", provider, true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": provider, "agentConnectionId": connection.ID, "implementationContext": selection})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	saved, err := env.st.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	return env, saved, done
}

func TestReviewedManifestBindsAddedApprovedContractBytes(t *testing.T) {
	env, saved, _ := createReviewedFixture(t, "codex")
	capture, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	_, original, err := reviewedManifest(capture)
	if err != nil {
		t.Fatal(err)
	}
	const approved = "\n## Approved execution contract\ncontractDigest: " + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "\n"
	for i := range capture.Files {
		if capture.Files[i].Path == "LUCI.md" {
			capture.Files[i].Data = append(capture.Files[i].Data, approved...)
		}
	}
	encoded, changed, err := reviewedManifest(capture)
	if err != nil || changed.Digest == original.Digest || reviewedHash(encoded) != changed.Digest {
		t.Fatal("approved content did not change the immutable manifest", err)
	}
	var manifest reviewedContextManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		if file.Path != "LUCI.md" {
			continue
		}
		for _, delivered := range capture.Files {
			if delivered.Path == "LUCI.md" && file.Digest == reviewedHash(delivered.Data) && bytes.Contains(delivered.Data, []byte(approved)) {
				return
			}
		}
	}
	t.Fatal("manifest did not bind the added LUCI.md bytes")
}

func TestReviewedOpenCodeInstructionPolicyFollowsCapturedBindingAcrossRestart(t *testing.T) {
	readInstructions := func(t *testing.T, env *sessionsEnv, index int) ([]string, string) {
		t.Helper()
		var config struct {
			Instructions []string `json:"instructions"`
			Agent        map[string]struct {
				Prompt string `json:"prompt"`
			} `json:"agent"`
		}
		if err := json.Unmarshal([]byte(env.be.spec(t, index).Env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
			t.Fatal(err)
		}
		return config.Instructions, config.Agent["build"].Prompt
	}
	env, saved, _ := createReviewedFixture(t, "opencode")
	if saved.ProjectRunID != "" || saved.ProjectRunOperationID != "" || saved.ImplementationContext == nil || saved.ReviewedContext == nil {
		t.Fatal("fixture must match the normal UI's captured task without a project run identity")
	}
	if got, prompt := readInstructions(t, env, 0); len(got) != 0 || prompt != "" {
		t.Fatalf("reviewed create loaded mutable checkout instructions: %v", got)
	}
	done := sessionsWatchDone(t)
	restarted := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions/"+saved.ID+"/restart", "{}"), 200)
	sessionsAwaitWatch(t, done)
	if restarted.ReviewedContextDelivery == nil || restarted.ReviewedContextDelivery.Digest != saved.ReviewedContext.Digest {
		t.Fatal("restart lost the exact captured context")
	}
	if got, prompt := readInstructions(t, env, 1); len(got) != 0 || prompt != "" {
		t.Fatalf("reviewed restart loaded mutable checkout instructions: %v", got)
	}

	ordinary, _, wt, _ := sessionsPrimaryBranchFixture(t)
	connection := sessionSaveConnection(t, ordinary, 0, "", "opencode", true)
	sessionsStubReady(t, nil)
	done = sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "opencode", "agentConnectionId": connection.ID})
	ordinarySession := sessionsDecodeOne(t, ordinary.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	if got, prompt := readInstructions(t, ordinary, 0); !reflect.DeepEqual(got, []string{"LUCI.md"}) || prompt != "" {
		t.Fatalf("ordinary session lost its saved instructions: %v", got)
	}
	done = sessionsWatchDone(t)
	sessionsDecodeOne(t, ordinary.do(t, "POST", "/v1/sessions/"+ordinarySession.ID+"/restart", "{}"), 200)
	sessionsAwaitWatch(t, done)
	if got, prompt := readInstructions(t, ordinary, 1); !reflect.DeepEqual(got, []string{"LUCI.md"}) || prompt != "" {
		t.Fatalf("ordinary restart lost its saved instructions: %v", got)
	}
}

func TestReviewedSessionAllHarnessesReceivePrivateCapturedAuthority(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			env, saved, _ := createReviewedFixture(t, provider)
			if saved.ReviewedContext == nil || saved.ReviewedContext.Version != 1 || !reviewedDigest.MatchString(saved.ReviewedContext.Digest) {
				t.Fatal("session record did not retain the exact bundle receipt")
			}
			capture, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
			if err != nil || len(capture.Files) != 8 {
				t.Fatal("captured scope cannot be verified", err)
			}
			spec := env.be.spec(t, 0)
			if err := backend.ValidateSpec(spec); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(spec.Args, " "), harness.ReviewedContextDirectory) || strings.Contains(strings.Join(spec.Args, " "), harness.ProjectBriefPrompt) {
				t.Fatal("harness retained contradictory live-checkout guidance")
			}
			mounts := 0
			for _, mount := range spec.Mounts {
				if mount.ContainerPath == reviewedContextContainerDir {
					mounts++
					if !mount.ReadOnly || strings.HasPrefix(mount.HostPath, env.dir+string(filepath.Separator)) {
						t.Fatal("reviewed scope is writable through the project mount")
					}
				}
			}
			if mounts != 1 {
				t.Fatal("expected exactly one private read-only context mount")
			}
			public, _ := json.Marshal(toJSON(saved, sessionBasePath))
			if !bytes.Contains(public, []byte(`"reviewedContextDelivery":{"status":"captured","version":1,"digest":"`+saved.ReviewedContext.Digest+`"}`)) || bytes.Contains(public, []byte(env.conversations)) {
				t.Fatal("public delivery receipt was missing or exposed a host path")
			}
			for _, file := range capture.Files {
				if bytes.Contains(file.Data, []byte(sessionGatewayKey)) || bytes.Contains(file.Data, []byte("u-reviewed-task-fixture")) {
					t.Fatal("bundle exported private authentication or review actors")
				}
			}
		})
	}
}

func TestReviewedSessionRefusesUnverifiableRestartAndResumeBeforeSideEffects(t *testing.T) {
	for _, defect := range []string{"legacy", "missing", "changed", "symlink", "hardlink", "foreign-binding"} {
		t.Run(defect, func(t *testing.T) {
			env, saved, _ := createReviewedFixture(t, "codex")
			dir, _ := env.api.reviewedContextPath(saved.ID)
			name := filepath.Join(dir, ".luci", "plan.json")
			switch defect {
			case "legacy":
				if _, err := env.st.Update(saved.ID, func(s *registry.Session) error { s.ReviewedContext = nil; return nil }); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			case "changed":
				if err := os.WriteFile(name, []byte("changed after capture"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "hardlink":
				original, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "private-plan.json")
				if err = os.WriteFile(outside, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Remove(name); err != nil {
					t.Fatal(err)
				}
				if defect == "symlink" {
					err = os.Symlink(outside, name)
				} else {
					err = os.Link(outside, name)
				}
				if err != nil {
					t.Skipf("fixture link unavailable: %v", err)
				}
			case "foreign-binding":
				capture, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
				if err != nil {
					t.Fatal(err)
				}
				capture.Binding.ProjectID = "p-ffffffff"
				manifest, foreign, err := reviewedManifest(capture)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err = env.st.Update(saved.ID, func(s *registry.Session) error { s.ReviewedContext = foreign; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			beforeCreate, beforeDestroy := len(env.be.specs), env.be.destroyCount()
			w := env.do(t, "POST", "/v1/sessions/"+saved.ID+"/restart", "{}")
			if w.Code != 409 || !strings.Contains(w.Body.String(), "Start a new reviewed run") || len(env.be.specs) != beforeCreate || env.be.destroyCount() != beforeDestroy || strings.Contains(w.Body.String(), dir) {
				t.Fatal("invalid bundle restart changed a live sandbox or leaked its private path", w.Code, w.Body.String())
			}
			public := sessionsDecodeOne(t, env.do(t, "GET", "/v1/sessions/"+saved.ID, ""), 200)
			if defect == "legacy" && (public.ReviewedContextDelivery == nil || public.ReviewedContextDelivery.Status != "legacy-live") {
				t.Fatal("legacy delivery was silently claimed frozen")
			}
			nativeWrite(t, nativeStateMount(t, env.be.spec(t, 0)), "sessions/fixture.jsonl", "owned original conversation")
			sessionsDecodeOne(t, env.do(t, "DELETE", "/v1/sessions/"+saved.ID, ""), 200)
			body, _ := json.Marshal(map[string]any{"worktreeId": saved.WorktreeID, "harness": saved.Harness, "resume": saved.ID})
			w = env.do(t, "POST", "/v1/sessions", string(body))
			if w.Code != 409 || !strings.Contains(w.Body.String(), "Start a new reviewed run") || len(env.be.specs) != beforeCreate || env.be.destroyCount() != beforeDestroy+1 {
				t.Fatal("invalid bundle resume allocated or replaced a child", w.Code, w.Body.String())
			}
		})
	}
}

func TestReviewedSessionResumeOwnsIndependentOriginalBundleAndKillRetainsIt(t *testing.T) {
	env, first, done := createReviewedFixture(t, "codex")
	firstDir, _ := env.api.reviewedContextPath(first.ID)
	nativeWrite(t, nativeStateMount(t, env.be.spec(t, 0)), "sessions/original.jsonl", "original conversation")
	sessionsDecodeOne(t, env.do(t, "DELETE", "/v1/sessions/"+first.ID, ""), 200)
	if _, err := env.api.readReviewedContext(first.ID, first.ImplementationContext, first.ReviewedContext); err != nil {
		t.Fatal("kill lost the original context", err)
	}
	worktree, err := env.api.deps.Worktrees.Path(context.Background(), first.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, worktree, "later-source.js", "// newer source does not change reviewed scope\n")
	body, _ := json.Marshal(map[string]any{"worktreeId": first.WorktreeID, "harness": first.Harness, "resume": first.ID})
	resumed := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	second, err := env.st.Get(resumed.ID)
	if err != nil || !reflect.DeepEqual(second.ReviewedContext, first.ReviewedContext) || second.ImplementationContext.SourceDigest == first.ImplementationContext.SourceDigest {
		t.Fatal("resume lost frozen context identity or re-used the old source observation", err)
	}
	secondDir, _ := env.api.reviewedContextPath(second.ID)
	left, err := os.Stat(filepath.Join(firstDir, ".luci", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.Stat(filepath.Join(secondDir, ".luci", "plan.json"))
	if err != nil || firstDir == secondDir || os.SameFile(left, right) {
		t.Fatal("resumed context shares deletable ownership with its source", err)
	}
	if w := env.do(t, "DELETE", "/v1/sessions/"+first.ID+"?purge=true", ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(firstDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("forget retained the original session bundle", err)
	}
	if _, err := env.api.readReviewedContext(second.ID, second.ImplementationContext, second.ReviewedContext); err != nil {
		t.Fatal("forgetting source broke the resumed session", err)
	}
	sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions/"+second.ID+"/restart", "{}"), 200)
	sessionsAwaitWatch(t, done)
	if spec := env.be.spec(t, 2); !strings.Contains(strings.Join(spec.Args, " "), harness.ReviewedContextDirectory) {
		t.Fatal("restart lost the original authority guidance")
	}
}

func TestReviewedContextIsRecordedBeforeAllocationAndUnrecordedCopyIsRemoved(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "recorded", true: "registry-failure"}[reject], func(t *testing.T) {
			env, p, wt, _ := sessionsPrimaryBranchFixture(t)
			selection := reviewedSessionPlan(t, env, p.ID)
			connection := sessionSaveConnection(t, env, 0, "", "codex", true)
			sessionsStubReady(t, nil)
			done := sessionsWatchDone(t)
			if reject {
				env.st.state.createErr = errors.New("fixture durable registry failure")
			} else {
				env.be.onCreate = func(spec backend.Spec) {
					saved, err := env.st.Get(spec.SessionID)
					if err != nil || saved.ReviewedContext == nil {
						t.Fatal("allocation began before its reviewed context receipt was durable", err)
					}
					if _, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext); err != nil {
						t.Fatal("allocation began before its complete context was verifiable", err)
					}
				}
			}
			body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "agentConnectionId": connection.ID, "implementationContext": selection})
			w := env.do(t, "POST", "/v1/sessions", string(body))
			if !reject {
				sessionsDecodeOne(t, w, 201)
				sessionsAwaitWatch(t, done)
				return
			}
			if w.Code != 500 || len(env.be.specs) != 0 {
				t.Fatal("registry failure allocated a sandbox", w.Code)
			}
			entries, err := os.ReadDir(filepath.Join(env.conversations, sessionConversationOwner, reviewedContextDirectory))
			if err != nil || len(entries) != 0 {
				t.Fatal("unrecorded session retained an unowned context directory", err)
			}
		})
	}
}
