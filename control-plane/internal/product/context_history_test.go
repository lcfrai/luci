package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Exercise the actual worker bearer/context boundary. Test headers enter only
// through the worker's authenticated gateway path; raw handlers cannot mint it.
func contextHistoryActorHandler(next http.Handler, actor management.User) http.Handler {
	token := strings.Repeat("context-private-gateway-", 3)
	user, _ := json.Marshal(actor)
	policy, _ := json.Marshal(management.Policy{AllowHostCommands: true, AllowPublicPublishing: true, MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, MonthlyBudgetUSD: 30, AllowedHarnesses: []string{"claude-code", "codex", "opencode", "shell"}, BackingMode: "docker", InstanceName: "Fixture"})
	worker := management.WorkerHandler(token, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(r.Context())
		clone.Header.Set("Authorization", "Bearer "+token)
		clone.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(user))
		clone.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
		worker.ServeHTTP(w, clone)
	})
}

func TestContextHistoryAttributesTwoEditorsAndReviewersOutsideProject(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "context-authors")
	mux := http.NewServeMux()
	s.registerProjects(mux)
	s.registerProjectWork(mux)
	alice := contextHistoryActorHandler(mux, management.User{ID: "alice-owner", Username: "alice", DisplayName: "Alice Editor", Role: "member"})
	bob := contextHistoryActorHandler(mux, management.User{ID: "bob-reviewer", Username: "bob", DisplayName: "Bob Reviewer", Role: "member"})
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Collaborate on product context", Audience: "Product builders"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-team", Title: "Shared definition", Priority: "must", Acceptance: "Two people can edit and review the same saved context"}}
	request := map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}
	decodeEvidenceResponse(t, journeyRequest(t, alice, "POST", base+"/specification", request, 200).Body.Bytes(), &spec)
	journeyRequest(t, bob, "POST", base+"/specification", request, 409)
	decodeEvidenceResponse(t, journeyRequest(t, bob, "POST", base+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 200).Body.Bytes(), &spec)
	var design ProjectDesignPlan
	decodeEvidenceResponse(t, journeyRequest(t, alice, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &design)
	design.Screens = []DesignScreen{{ID: "home", Name: "Shared context", Purpose: "Review team context", States: []string{"Loading", "Ready", "Error"}, Widths: []int{390, 1440}, RequirementIDs: []string{"AC-team"}}}
	decodeEvidenceResponse(t, journeyRequest(t, alice, "POST", base+"/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, 200).Body.Bytes(), &design)
	decodeEvidenceResponse(t, journeyRequest(t, bob, "POST", base+"/design-plan/approve", map[string]int{"revision": design.Revision}, 200).Body.Bytes(), &design)
	journeyRequest(t, bob, "POST", base+"/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []ProjectTask{{ID: "team-context", Title: "Build collaboration", Status: "todo", RequirementIDs: []string{"AC-team"}}}}, 200)
	var view struct {
		Events []ContextChangeEvent `json:"events"`
		Pruned bool                 `json:"pruned"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, bob, "GET", base+"/context-history", nil, 200).Body.Bytes(), &view)
	if len(view.Events) != 6 || view.Pruned {
		t.Fatal("lost or invented successful edits", view)
	}
	counts := map[string]int{}
	for _, event := range view.Events {
		if event.Status != "applied" || event.ProjectID != p.ID || !hexDigest(event.Digest) {
			t.Fatal("unverified attribution", event)
		}
		if event.Action == "approve" || event.Kind == "plan" {
			if event.Actor.ID != "bob-reviewer" || event.Actor.DisplayName != "Bob Reviewer" {
				t.Fatal("wrong authenticated reviewer", event)
			}
		} else if event.Actor.ID != "alice-owner" || event.Actor.DisplayName != "Alice Editor" {
			t.Fatal("wrong authenticated editor", event)
		}
		counts[event.Kind+":"+event.Action]++
	}
	if counts["specification:save"] != 1 || counts["specification:approve"] != 1 || counts["design:approve"] != 1 || counts["plan:save"] != 1 {
		t.Fatal("mutation kinds lost", counts)
	}
	path, err := s.contextHistoryPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateFile(t, path)
	for _, file := range []string{"LUCI.md", ".luci/state.json", ".luci/specification.json", ".luci/design-handoff.json", ".luci/plan.json"} {
		data, err := os.ReadFile(filepath.Join(p.Path, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "alice-owner") || strings.Contains(string(data), "bob-reviewer") || strings.Contains(string(data), "Alice Editor") || strings.Contains(string(data), "Bob Reviewer") {
			t.Fatal("private actor metadata leaked into agent context", file)
		}
	}
	before := auditTree(t, s.stateDir)
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	stored, err := reopened.readContextHistory(p.Path, p.ID)
	if err != nil || !reflect.DeepEqual(stored.Events, view.Events) {
		t.Fatal("restart lost authenticated history", err)
	}
	journeyRequest(t, bob, "GET", base+"/context-history", nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("reading attribution rewrote private state")
	}
}

func TestContextHistoryRejectsForgedHeadersAndExportedActorClaims(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "context-forgery")
	mux := http.NewServeMux()
	s.registerProjects(mux)
	base := "/v1/product/projects/" + p.ID
	body := `{"revision":0,"purpose":"Legacy direct edit","audience":"Builders","criteria":[]}`
	r := httptest.NewRequest("POST", base+"/brief", strings.NewReader(body))
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString([]byte(`{"id":"forged-reviewer","displayName":"Forged identity","role":"super-admin"}`)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("legacy edit behavior changed", w.Code, w.Body.String())
	}
	history, err := s.readContextHistory(p.Path, p.ID)
	if err != nil || len(history.Events) != 0 {
		t.Fatal("raw actor header minted attribution", history, err)
	}
	r = httptest.NewRequest("POST", base+"/specification/approve", strings.NewReader(`{"revision":1,"briefRevision":1}`))
	r.Header.Set("X-Luci-Actor", `forged-reviewer`)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("raw actor header granted review authority")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 1, "purpose": "Forged", "actor": map[string]string{"id": "forged-reviewer"}}, 400)
	// Agent-authored state may include extra exported claims; the private
	// endpoint never reads them or retroactively attributes them to a person.
	data, err := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state["contextHistory"] = []map[string]any{{"actor": map[string]string{"id": "forged-reviewer"}, "status": "applied"}}
	if err := atomicJSON(filepath.Join(p.Path, ".luci", "state.json"), state); err != nil {
		t.Fatal(err)
	}
	w = journeyRequest(t, mux, "GET", base+"/context-history", nil, 200)
	if strings.Contains(w.Body.String(), "forged-reviewer") || !strings.Contains(w.Body.String(), `"events":[]`) {
		t.Fatal("project export granted authorship", w.Body.String())
	}
}

func TestContextHistoryUnavailableStorageStopsAttributedMutationAndCopiesFail(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "context-private")
	mux := http.NewServeMux()
	s.registerProjects(mux)
	alice := contextHistoryActorHandler(mux, management.User{ID: "alice", DisplayName: "Alice", Role: "member"})
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, alice, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Original", "audience": "Builders", "criteria": []string{}}, 200)
	path, err := s.contextHistoryPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, p.Path)
	if err := os.WriteFile(path, []byte(`{"version":1,"events":"corrupt"}`), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, alice, "POST", base+"/brief", map[string]any{"revision": 1, "purpose": "Must not save", "audience": "Builders", "criteria": []string{}}, 500)
	if !reflect.DeepEqual(before, auditTree(t, p.Path)) {
		t.Fatal("unrecordable attributed change modified project")
	}
	if err := os.WriteFile(path, private, 0600); err != nil {
		t.Fatal(err)
	}
	other, err := New(Options{StateDir: t.TempDir(), ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	copyPath, err := other.contextHistoryPath(p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, private, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := other.readContextHistory(p.Path, p.ID); err == nil {
		t.Fatal("copied workspace ledger conferred authorship")
	}
}

func TestContextHistoryFailedProjectWriteRetainsUnconfirmedIntent(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "context-partial")
	var actorContext context.Context
	handler := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { actorContext = r.Context() }), management.User{ID: "alice", DisplayName: "Alice", Role: "member"})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil))
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision = 1
	state.Brief.Purpose = "Saved intent"
	if err := os.MkdirAll(filepath.Join(p.Path, ".luci", "design.json"), 0700); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	err = s.writeAttributedJourney(actorContext, p.Path, state, contextMutation{"brief", "save"})
	s.mu.Unlock()
	if err == nil {
		t.Fatal("invalid project export did not fail")
	}
	history, readErr := s.readContextHistory(p.Path, p.ID)
	if readErr != nil || len(history.Events) != 1 || history.Events[0].Status != "unconfirmed" || history.Events[0].Actor.ID != "alice" {
		t.Fatal("partial write manufactured applied attribution", history, readErr)
	}
}

func TestContextHistoryRetainsRecentAttributionWithinCountAndByteBounds(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "context-bounded")
	saved, err := s.readContextHistory(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	actor := ContextActor{ID: "alice", DisplayName: strings.Repeat(`"`, 512)}
	event := ContextChangeEvent{ID: "context-" + strings.Repeat("a", 24), ProjectID: p.ID, Kind: "brief", Action: "save", Revision: 1, Digest: strings.Repeat("b", 64), At: time.Now().UTC().Format(time.RFC3339Nano), Actor: actor, Status: "applied"}
	saved.Events = []ContextChangeEvent{event}
	one, _ := json.MarshalIndent(saved, "", "  ")
	saved.Events = append(saved.Events, event)
	two, _ := json.MarshalIndent(saved, "", "  ")
	count := (contextHistoryFileLimit-len(one)-1)/(len(two)-len(one)) + 1
	if count >= contextHistoryLimit {
		t.Fatal("fixture does not exercise byte limit before count limit")
	}
	saved.Events = make([]ContextChangeEvent, count)
	for i := range saved.Events {
		saved.Events[i] = event
		saved.Events[i].ID = fmt.Sprintf("context-%024x", i)
	}
	if err := s.saveContextHistory(saved); err != nil {
		t.Fatal(err)
	}
	var actorContext context.Context
	handler := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { actorContext = r.Context() }), management.User{ID: actor.ID, DisplayName: actor.DisplayName, Role: "member"})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil))
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision = 2
	state.Brief.Purpose = "Newest retained author"
	s.mu.Lock()
	err = s.writeAttributedJourney(actorContext, p.Path, state, contextMutation{"brief", "save"})
	s.mu.Unlock()
	if err != nil {
		t.Fatal("byte-bound pruning blocked new attributed edit", err)
	}
	loaded, err := s.readContextHistory(p.Path, p.ID)
	if err != nil || !loaded.Pruned || len(loaded.Events) > count || loaded.Events[0].Revision != 2 || loaded.Events[0].Status != "applied" {
		t.Fatal("recent attribution was not retained within bound", err)
	}
	path, _ := s.contextHistoryPath(p.ID, false)
	info, err := os.Stat(path)
	if err != nil || info.Size() > contextHistoryFileLimit {
		t.Fatal("saved ledger exceeds read bound", err)
	}
}
