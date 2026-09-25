package panaceatest

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 04: the write routes' fidelity. Each answer is the shape the cited Panacea writer produces.

// send makes one authenticated request with a JSON body ("" for none).
func send(t *testing.T, s *Server, method, path, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, s.URL()+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+s.Token())
	resp, err := s.HTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func str(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("%s is not a string: %v", raw, err)
	}
	return s
}

// TestShipRoute: POST P/deploy answers 202 {message, data {project, status, pipelineRun}}
// (handlers/projects.go:2998-3005), and the run's row is in the history at once, in the branch's
// seeded environment, "Unknown" until a condition is set (clients/kubernetes.go:5662).
func TestShipRoute(t *testing.T) {
	s := New(t)
	s.Project(deploy.Project{Ref: "p1"})
	s.BranchEnvironment("p1", "feat/x", "feat-x")
	const commit = "4f0c2a9d1b7e3c5a8f6d0b2e4c6a8f1d3b5e7c9a"
	st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/deploy", `{"branch":"feat/x","commit":"`+commit+`"}`)
	if st != http.StatusAccepted || str(t, fields(t, body)["message"]) != "deployment started" {
		t.Fatalf("POST deploy = %d %s, want 202 deployment started", st, body)
	}
	d := fields(t, string(fields(t, body)["data"]))
	run := str(t, d["pipelineRun"])
	if str(t, d["project"]) != "p1" || str(t, d["status"]) != "running" || run == "" {
		t.Errorf("data = %s, want project p1, status running, a named run", fields(t, body)["data"])
	}
	_, _, list := get(t, s, "/api/v1/projects/p1/deployments", s.Token())
	var rows struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(list), &rows); err != nil || len(rows.Items) != 1 {
		t.Fatalf("deployments = %s (%v), want the Ship's one row", list, err)
	}
	r := rows.Items[0]
	if r["id"] != run || r["status"] != "Unknown" || r["environment"] != "feat-x" || r["commit"] != commit {
		t.Errorf("the Ship's row = %v, want id %s, Unknown, feat-x, the commit", r, run)
	}
}

// TestCancelRoute copies CancelDeploy's one rule: only Pending or Running cancels
// (handlers/projects.go:3669-3674), so a run with no condition yet (Unknown) answers
// already_completed, as Panacea does. A cancelled run reads Failed; an unknown one is 404.
func TestCancelRoute(t *testing.T) {
	s := New(t)
	at := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	for i, b := range []deploy.BuildState{deploy.BuildSucceeded, deploy.BuildRunning, deploy.BuildPending} {
		s.Deployment("p1", deploy.Deployment{ID: []string{"r-done", "r-run", "r-new"}[i], Environment: "feat-x", Build: b, CreatedAt: at.Add(time.Duration(i) * time.Minute)})
	}
	for path, want := range map[string]string{
		"r-done": `{"data":{"message":"Pipeline run is already Succeeded","status":"already_completed"}}`,
		"r-run":  `{"data":{"message":"Deployment cancelled successfully","status":"cancelled"}}`,
		"r-new":  `{"data":{"message":"Pipeline run is already Unknown","status":"already_completed"}}`,
	} {
		if st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/pipelines/"+path+"/cancel", ""); st != 200 || body != want {
			t.Errorf("cancel %s = %d %s, want 200 %s", path, st, body, want)
		}
	}
	if _, _, body := get(t, s, "/api/v1/projects/p1/pipelines/r-run", s.Token()); !strings.Contains(body, `"status":"Failed"`) {
		t.Errorf("a cancelled run reads %s, want Failed", body)
	}
	if st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/pipelines/nope/cancel", ""); st != 404 || !strings.HasPrefix(body, `{"error":"pipeline run not found`) {
		t.Errorf("cancel of an unknown run = %d %s, want 404", st, body)
	}
}

// TestRollbackRoute: 200 {message "rollback initiated to <id>", data: the Deployment rolled back
// TO} (handlers/projects.go:1619-1639), warm with no coldStartSeconds key, cold with 20
// (clients/immutable_deploy.go:77); no image is the 400 (:1575), an unknown id the 404 (:1570).
// The history is unchanged.
func TestRollbackRoute(t *testing.T) {
	s := New(t)
	at := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"d-warm", "d-cold", "d-bare"} {
		d := deploy.Deployment{ID: id, Environment: "feat-x", Build: deploy.BuildSucceeded, CreatedAt: at.Add(time.Duration(i) * time.Minute)}
		if id != "d-bare" {
			d.Image = "registry.test/app:" + id
		}
		s.Deployment("p1", d)
	}
	s.RollbackTarget("p1", "d-warm", deploy.WarmthWarm)
	s.RollbackTarget("p1", "d-cold", deploy.WarmthCold)
	_, _, before := get(t, s, "/api/v1/projects/p1/deployments", s.Token())
	for id, want := range map[string]string{"d-warm": `"rollbackWarmth":"warm"`, "d-cold": `"rollbackWarmth":"cold","coldStartSeconds":20`} {
		st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/deployments/"+id+"/rollback", "")
		f := fields(t, body)
		if st != 200 || str(t, f["message"]) != "rollback initiated to "+id || !strings.Contains(body, `"id":"`+id+`"`) ||
			!strings.Contains(body, want) || !strings.Contains(body, `"image":"registry.test/app:`+id+`"`) {
			t.Errorf("rollback %s = %d %s, want 200 with %s", id, st, body, want)
		}
		if id == "d-warm" && strings.Contains(body, "coldStartSeconds") {
			t.Errorf("a warm rollback carries coldStartSeconds, which is omitempty (models/models.go:708): %s", body)
		}
	}
	if st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/deployments/d-bare/rollback", ""); st != 400 ||
		body != `{"error":"deployment has no image tag — cannot rollback"}` {
		t.Errorf("rollback of an imageless row = %d %s, want Panacea's 400", st, body)
	}
	if st, body := send(t, s, http.MethodPost, "/api/v1/projects/p1/deployments/nope/rollback", ""); st != 404 ||
		body != `{"error":"deployment not found: nope"}` {
		t.Errorf("rollback of an unknown id = %d %s, want Panacea's 404", st, body)
	}
	if _, _, after := get(t, s, "/api/v1/projects/p1/deployments", s.Token()); after != before {
		t.Errorf("a rollback changed the history:\nbefore %s\nafter  %s", before, after)
	}
}

// TestAccessRoutes copies the share's rules from clients/keycloak.go: a grant joins the group
// "project-<name>-"+role, unchecked (:192), after leaving the project's other groups (:198-214);
// the list reads the role from the group's suffix, -owner and -editor, anything else viewer
// (:503-512), one row per member with group == grantee (:156-159); a revoke by a member's name
// removes them from every group (:245-260), and a name that matches nothing is a 200 no-op (:268).
// Add's refusals: "" is the 400 (handlers/access.go:50-52), whitespace the 500 of :182 via :57.
func TestAccessRoutes(t *testing.T) {
	s := New(t)
	s.Project(deploy.Project{Ref: "p1"})
	const list = "/api/v1/projects/p1/access"
	grant := func(group, role string) (int, string) {
		return send(t, s, http.MethodPost, list, `{"group":"`+group+`","role":"`+role+`"}`)
	}
	listed := func() string { _, _, b := get(t, s, list, s.Token()); return strings.TrimSpace(b) }
	if got := listed(); got != `{"items":[],"totalCount":0}` {
		t.Errorf("no grants = %s", got)
	}
	if st, body := grant("a@x.au", "viewer"); st != 201 || body != `{"message":"access binding added"}` {
		t.Errorf("grant = %d %s, want 201 access binding added", st, body)
	}
	grant("a@x.au", "editor") // a role change is one grant: single-role
	grant("b", "developer")   // no such suffix: listed as a viewer
	want := `{"items":[{"group":"b","role":"viewer","grantee":"b"},{"group":"a@x.au","role":"editor","grantee":"a@x.au"}],"totalCount":2}`
	if got := listed(); got != want {
		t.Errorf("after the grants = %s\nwant %s", got, want)
	}
	if st, body := grant("", "viewer"); st != 400 || body != `{"error":"group and role are required"}` {
		t.Errorf("an empty grantee = %d %s, want Panacea's 400", st, body)
	}
	if st, body := grant("  ", "viewer"); st != 500 || body != `{"error":"failed to add access binding: a grantee (username or email) is required"}` {
		t.Errorf("a blank grantee = %d %s, want Panacea's 500", st, body)
	}
	for _, g := range []string{"a@x.au", "nobody"} {
		if st, body := send(t, s, http.MethodDelete, list+"/"+g, ""); st != 200 || body != `{"message":"access binding removed"}` {
			t.Errorf("revoke %s = %d %s, want 200", g, st, body)
		}
	}
	if got := listed(); got != `{"items":[{"group":"b","role":"viewer","grantee":"b"}],"totalCount":1}` {
		t.Errorf("after the revokes = %s", got)
	}
}

var mintedToken = regexp.MustCompile(`^pnb_[0-9a-f]{32}$`)

// TestBranchAccessPut copies SetBranchAccessMode's rules (handlers/project_security.go:701-845):
// a mode outside the five is the 400 (:718-722), password needs a password (:766-770),
// bypass-token with no supplied token mints one, returned once (:786-797, :830-833), and the
// answer is {message, data {branch, env, accessMode, enforced}}. The GET then reads the new state.
func TestBranchAccessPut(t *testing.T) {
	s := New(t)
	s.BranchEnvironment("p1", "feat/x", "feat-x")
	const path = "/api/v1/projects/p1/branches/feat%2Fx/access"
	st, body := send(t, s, http.MethodPut, path, `{"accessMode":"password","password":"p"}`)
	if want := `{"data":{"accessMode":"password","branch":"feat/x","enforced":true,"env":"feat-x"},"message":"access mode set to password for p1@feat/x"}`; st != 200 || body != want {
		t.Errorf("PUT password = %d %s\nwant 200 %s", st, body, want)
	}
	if _, _, got := get(t, s, path, s.Token()); !strings.Contains(got, `"accessMode":"password"`) || !strings.Contains(got, `"passwordSet":true`) {
		t.Errorf("GET after a password = %s", got)
	}
	st, body = send(t, s, http.MethodPut, path, `{"accessMode":"bypass-token"}`)
	var minted struct {
		Data struct {
			BypassTokens []string `json:"bypassTokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &minted); st != 200 || err != nil || len(minted.Data.BypassTokens) != 1 || !mintedToken.MatchString(minted.Data.BypassTokens[0]) {
		t.Errorf("PUT bypass-token = %d %s, want one minted pnb_ token", st, body)
	}
	if _, _, got := get(t, s, path, s.Token()); !strings.Contains(got, `"bypassTokenCount":1`) || strings.Contains(got, "pnb_") {
		t.Errorf("GET after a mint = %s, want the count and never the token", got)
	}
	for req, want := range map[string]string{
		`{"accessMode":"public"}`:   `{"error":"invalid accessMode \"public\": must be one of panacea-sso, kpmg-sso, password, bypass-token, none"}`,
		`{"accessMode":"password"}`: `{"error":"password is required for accessMode=password"}`,
	} {
		if st, body := send(t, s, http.MethodPut, path, req); st != 400 || body != want {
			t.Errorf("PUT %s = %d %s, want 400 %s", req, st, body, want)
		}
	}
}
