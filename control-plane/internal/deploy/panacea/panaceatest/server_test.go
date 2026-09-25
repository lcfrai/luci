package panaceatest

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// The fake's own fidelity: each answer below is the shape the cited Panacea writer produces.

func get(t *testing.T, s *Server, path, bearer string) (int, string, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.URL()+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := s.HTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}

func fields(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("%q is not a JSON object: %v", body, err)
	}
	return m
}

// TestGateAndMembership: any bearer but Token is apiKeyMW's 401 (middleware/api_key.go:171), and
// a project nothing seeded is RequireProjectAccess's text/plain 403 (middleware/rbac.go:221-228).
// Every request is recorded, escaped path and query kept.
func TestGateAndMembership(t *testing.T) {
	s := New(t)
	s.Project(deploy.Project{Ref: "p1"})
	if st, _, body := get(t, s, "/api/v1/me", "pnc_other"); st != 401 || !strings.Contains(body, `"invalid api key"`) {
		t.Errorf("wrong bearer = %d %q, want 401 invalid api key", st, body)
	}
	st, ct, body := get(t, s, "/api/v1/projects/nope/environments", s.Token())
	if st != 403 || !strings.HasPrefix(ct, "text/plain") || strings.TrimSpace(body) != `{"error":"forbidden: you do not have access to this project"}` {
		t.Errorf("unseeded project = %d %q %q, want the text/plain 403", st, ct, body)
	}
	if st, _, body := get(t, s, "/api/v1/projects/p1/environments?x=a%20b", s.Token()); st != 200 || strings.TrimSpace(body) != `{"items":[],"totalCount":0}` {
		t.Errorf("no environments = %d %q, want an empty list", st, body)
	}
	want := []string{"GET /api/v1/me", "GET /api/v1/projects/nope/environments", "GET /api/v1/projects/p1/environments?x=a%20b"}
	if got := s.Requests(); !slices.Equal(got, want) {
		t.Errorf("Requests = %q, want %q", got, want)
	}
}

// TestDeploymentShape: a seeded row renders with Panacea's omitempty keys (models/models.go:665-770),
// its duration as kubernetes.go:5683-5689 writes it, newest first, a re-seed replacing its ID.
func TestDeploymentShape(t *testing.T) {
	s := New(t)
	at := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	s.Deployment("p1", deploy.Deployment{ID: "old", Build: deploy.BuildSucceeded, CreatedAt: at.Add(-time.Hour)})
	s.Deployment("p1", deploy.Deployment{ID: "new", Build: deploy.BuildFailed, CreatedAt: at})
	s.Deployment("p1", deploy.Deployment{ID: "new", Build: deploy.BuildRunning, CreatedAt: at,
		CompletedAt: at.Add(4*time.Minute + 12*time.Second + 400*time.Millisecond)})
	_, _, body := get(t, s, "/api/v1/projects/p1/deployments", s.Token())
	var list struct {
		Items      []map[string]json.RawMessage `json:"items"`
		TotalCount int                          `json:"totalCount"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil || list.TotalCount != 2 || len(list.Items) != 2 {
		t.Fatalf("deployments = %q (%v), want 2 rows", body, err)
	}
	row := list.Items[0]
	if string(row["id"]) != `"new"` || string(row["status"]) != `"Pending"` || string(row["duration"]) != `"4m12s"` {
		t.Errorf("newest row = %v, want new, Pending (a running build), 4m12s", row)
	}
	for _, k := range []string{"version", "liveReadyReplicas", "scanStatus", "provisioning", "backfilled"} {
		if _, ok := row[k]; ok {
			t.Errorf("an unset %s was written; Panacea's omitempty drops it", k)
		}
	}
	if _, ok := list.Items[1]["duration"]; ok || string(list.Items[1]["id"]) != `"old"` {
		t.Errorf("second row = %v, want old with no duration", list.Items[1])
	}
}

// TestScansResolveEnvAsPanacea: a request without env, or with prod or production, reads
// production's report (handlers/security_status.go:192-201, handlers/tier_store.go:106-113);
// any other env is looked up exactly, so a connector that drops env reads production's.
func TestScansResolveEnvAsPanacea(t *testing.T) {
	for in, want := range map[string]string{"": "production", "prod": "production", "production": "production",
		" prod ": "production", "feat-x": "feat-x", "prod-eu": "prod-eu"} {
		if got := productionKey(in); got != want {
			t.Errorf("productionKey(%q) = %q, want %q", in, got, want)
		}
	}
	s := New(t)
	s.Scans("p1", "prod", "d1", deploy.ScanReport{Overall: "PASS", DeploymentID: "d1"})
	s.Scans("p1", "feat-x", "d1", deploy.ScanReport{Overall: "FAIL", DeploymentID: "d1"})
	for q, want := range map[string]string{"?deployId=d1": "PASS", "?deployId=d1&env=production": "PASS",
		"?deployId=d1&env=prod": "PASS", "?deployId=d1&env=feat-x": "FAIL"} {
		_, _, body := get(t, s, "/api/v1/projects/p1/security-status"+q, s.Token())
		var d struct {
			Data struct {
				Overall  string            `json:"overall"`
				Blocking []json.RawMessage `json:"blocking"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(body), &d) != nil || d.Data.Overall != want || d.Data.Blocking == nil {
			t.Errorf("security-status%s = %q, want overall %s and blocking []", q, body, want)
		}
	}
}

// TestAccessOmitsWhatIsUnseeded: env comes from BranchEnvironment, accessMode from PostureWord,
// each omitted when unseeded (handlers/project_security.go:668-677 always writes both; the
// omission is how a posture-only branch reads a missing env). The branch is the mux's unescaped
// segment.
func TestAccessOmitsWhatIsUnseeded(t *testing.T) {
	s := New(t)
	s.BranchEnvironment("p1", "feat/x", "feat-x")
	s.PostureWord("p1", "posture/only", "open-to-all")
	_, _, body := get(t, s, "/api/v1/projects/p1/branches/feat%2Fx/access", s.Token())
	d := fields(t, string(fields(t, body)["data"]))
	if string(d["env"]) != `"feat-x"` || string(d["branch"]) != `"feat/x"` || d["accessMode"] != nil {
		t.Errorf("env-only branch = %s, want env feat-x, branch feat/x, no accessMode", body)
	}
	_, _, body = get(t, s, "/api/v1/projects/p1/branches/posture%2Fonly/access", s.Token())
	d = fields(t, string(fields(t, body)["data"]))
	if d["env"] != nil || string(d["accessMode"]) != `"open-to-all"` {
		t.Errorf("posture-only branch = %s, want no env, accessMode open-to-all", body)
	}
}

// TestLatestAndRun: no deployments is Panacea's 404 sentence (handlers/projects.go:1539); a
// seeded row's run answers the pipelines route in the data envelope.
func TestLatestAndRun(t *testing.T) {
	s := New(t)
	s.NoDeployments("p1")
	st, _, body := get(t, s, "/api/v1/projects/p1/deploys/latest", s.Token())
	if st != 404 || strings.TrimSpace(body) != `{"error":"no deployments yet for project p1"}` {
		t.Errorf("latest of none = %d %q", st, body)
	}
	at := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	s.Deployment("p1", deploy.Deployment{ID: "r1", Build: deploy.BuildPending, CreatedAt: at})
	_, _, body = get(t, s, "/api/v1/projects/p1/pipelines/r1", s.Token())
	d := fields(t, string(fields(t, body)["data"]))
	if string(d["name"]) != `"r1"` || string(d["status"]) != `"Unknown"` || string(d["startTime"]) != `"2026-09-23T10:00:00Z"` {
		t.Errorf("run r1 = %s, want name r1, status Unknown, startTime", body)
	}
}
