package panacea

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 03: the reads. Every mapping test reads a recorded body under testdata/, written by hand
// from the Panacea line its "_source" names; inline bodies are only variants of one field.

const project = deploy.ProjectRef("engagement-portal")

// fixture is one recorded answer.
type fixture struct {
	file   string // under testdata/; its _source is removed before serving
	body   string // served verbatim when file is ""
	status int    // 0 is 200
	ctype  string // "" is application/json
}

// loadFixture reads testdata/name as the cited writer sends it: a .json file loses its
// "_source" key, a .txt file its first "_source: " line. A file without its source fails.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	if strings.HasSuffix(name, ".txt") {
		first, rest, ok := strings.Cut(string(raw), "\n")
		if !ok || !strings.HasPrefix(first, "_source: ") || strings.TrimSpace(rest) == "" {
			t.Fatalf("fixture %s: want a first line \"_source: <the Panacea line>\", then the body", name)
		}
		return []byte(rest)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("fixture %s is not a JSON object: %v", name, err)
	}
	var src string
	if json.Unmarshal(fields["_source"], &src) != nil || strings.TrimSpace(src) == "" {
		t.Fatalf("fixture %s has no \"_source\" naming the Panacea line it mirrors", name)
	}
	delete(fields, "_source")
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return out
}

// recorded answers each "METHOD /escaped/path?query" from its routes and records every request
// in order, with its body (task 04's writes). A request no route names fails the test and is
// answered 500.
type recorded struct {
	*harness
	mu     sync.Mutex
	reqs   []string
	bodies []string
}

func serveRecorded(t *testing.T, routes map[string]fixture) *recorded {
	t.Helper()
	bodies := make(map[string][]byte, len(routes))
	for k, f := range routes {
		bodies[k] = []byte(f.body)
		if f.file != "" {
			bodies[k] = loadFixture(t, f.file)
		}
	}
	rs := &recorded{}
	rs.harness = newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.RequestURI()
		sent, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the body of %s: %v", key, err)
		}
		rs.mu.Lock()
		rs.reqs = append(rs.reqs, key)
		rs.bodies = append(rs.bodies, string(sent))
		rs.mu.Unlock()
		f, ok := routes[key]
		if !ok {
			t.Errorf("unexpected request %s", key)
			http.Error(w, `{"error":"no fixture"}`, http.StatusInternalServerError)
			return
		}
		ct, st := f.ctype, f.status
		if ct == "" {
			ct = "application/json"
		}
		if st == 0 {
			st = http.StatusOK
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(st)
		_, _ = w.Write(bodies[key])
	})
	return rs
}

func (rs *recorded) requests() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return slices.Clone(rs.reqs)
}

// sent is every request's body, in request order.
func (rs *recorded) sent() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return slices.Clone(rs.bodies)
}

// at parses an RFC 3339 instant for a want value.
func at(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func intp(n int) *int { return &n }

func wantKind(t *testing.T, err error, kind deploy.Kind, status int, message string) {
	t.Helper()
	e := asDeployError(t, err)
	if e.Kind != kind || e.Status != status || e.Message != message {
		t.Errorf("error = (Kind %q, Status %d, Message %q), want (%q, %d, %q)", e.Kind, e.Status, e.Message, kind, status, message)
	}
}

// TestWhoamiApprovedIsStrict kills reading a missing approved as approved: the pending user
// would see a full lens whose every act 403s (handlers/me.go:29-37).
func TestWhoamiApprovedIsStrict(t *testing.T) {
	for _, tc := range []struct {
		name, file, body string
		want             deploy.Account
	}{
		{name: "approved", file: "me-approved.json",
			want: deploy.Account{Username: "alice", Email: "alice@kpmg.com.au", Approved: true}},
		{name: "pending", file: "me-pending.json",
			want: deploy.Account{Username: "bob", Email: "bob@kpmg.com.au"}},
		{name: "no approved field", file: "me-no-approved-field.json",
			want: deploy.Account{Username: "carol", Email: "carol@kpmg.com.au"}},
		{name: "approved null", body: `{"username":"dan","email":"dan@kpmg.com.au","approved":null}`,
			want: deploy.Account{Username: "dan", Email: "dan@kpmg.com.au"}},
		{name: "admin", body: `{"username":"root","email":"root@kpmg.com.au","isAdmin":true,"role":"superadmin","approved":true}`,
			want: deploy.Account{Username: "root", Email: "root@kpmg.com.au", Approved: true, Admin: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{"GET /api/v1/me": {file: tc.file, body: tc.body}})
			got, err := rs.c.Whoami(context.Background())
			if err != nil || got != tc.want {
				t.Errorf("Whoami = (%+v, %v), want (%+v, nil)", got, err, tc.want)
			}
		})
	}
}

// TestScopedKeyIsOneApprovedProject: a key the portal restricted to one project at Deploy is an
// approved non-admin that sees one project, and its refusal keeps Panacea's sentence, the only
// place Panacea says the KEY, not the person, is the limit (review I5).
func TestScopedKeyIsOneApprovedProject(t *testing.T) {
	const probe = "/api/v1/projects/engagement-portal/branches/main/auto-deploy"
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/me":       {file: "me-scoped-deploy.json"},
		"GET /api/v1/projects": {file: "projects-scoped.json"},
		"GET " + probe:         {file: "error-scope-owner-403.txt", status: 403, ctype: "text/plain; charset=utf-8"},
	})
	ctx := context.Background()
	acct, err := rs.c.Whoami(ctx)
	if err != nil || !acct.Approved || acct.Admin {
		t.Errorf("Whoami = (%+v, %v), want Approved true, Admin false", acct, err)
	}
	ps, err := rs.c.Projects(ctx)
	if err != nil || len(ps) != 1 || ps[0].Ref != project {
		t.Errorf("Projects = (%+v, %v), want exactly engagement-portal", ps, err)
	}
	// The owner probe is task 04's AutoDeploy; the refusal is the transport's, so do carries it.
	var out json.RawMessage
	err = rs.c.do(ctx, "autoDeploy", http.MethodGet, probe, nil, &out)
	wantKind(t, err, deploy.KindForbidden, 403, "forbidden: this token's scope does not permit owner access to this project")
}

// TestProjectsKeepsMissingRepoURL: a project the answer names without repoUrl reads "" (task
// 06's NoRepoInfo state), never a URL built from its name.
func TestProjectsKeepsMissingRepoURL(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET /api/v1/projects": {file: "projects.json"}})
	got, err := rs.c.Projects(context.Background())
	want := []deploy.Project{
		{Ref: "engagement-portal", DisplayName: "Engagement Portal",
			RepoURL: "https://dev.azure.com/kpmg-au/engagement/_git/engagement-portal", DefaultBranch: "main"},
		{Ref: "risk-register"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Projects = (%+v, %v), want %+v", got, err, want)
	}
}

// TestTextPlain403 kills a decoder gated on Content-Type: application/json, which would lose
// Panacea's most actionable sentence (middleware/rbac.go:398-400 writes it as text/plain).
func TestTextPlain403(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/projects": {file: "error-text-plain-403.txt", status: 403, ctype: "text/plain; charset=utf-8"},
	})
	_, err := rs.c.Projects(context.Background())
	wantKind(t, err, deploy.KindForbidden, 403, "no access yet: your account is waiting for team approval — finish onboarding first")
}

const latestPath = "GET /api/v1/projects/engagement-portal/deploys/latest"

// TestLatest404IsNotAnError kills a red error banner on a project that has simply never
// shipped (handlers/projects.go:1539), and a Latest that swallows every other error as "none".
func TestLatest404IsNotAnError(t *testing.T) {
	ctx := context.Background()
	t.Run("never deployed", func(t *testing.T) {
		rs := serveRecorded(t, map[string]fixture{latestPath: {file: "latest-404.json", status: 404}})
		d, ok, err := rs.c.Latest(ctx, project)
		if err != nil || ok || !reflect.DeepEqual(d, deploy.Deployment{}) {
			t.Errorf("Latest = (%+v, %v, %v), want (Deployment{}, false, nil)", d, ok, err)
		}
	})
	t.Run("bare deployment", func(t *testing.T) {
		rs := serveRecorded(t, map[string]fixture{latestPath: {file: "latest.json"}})
		d, ok, err := rs.c.Latest(ctx, project)
		if err != nil || !ok || d.ID != "engagement-portal-deploy-7k2xq" || d.Environment != "prod" ||
			d.Build != deploy.BuildSucceeded || d.LiveReady == nil || *d.LiveReady != 2 ||
			!d.CompletedAt.Equal(at(t, "2026-09-23T10:04:12Z")) || d.Gates["trivy"].Verdict != "passed" {
			t.Errorf("Latest = (%+v, %v, %v), want the recorded prod deployment", d, ok, err)
		}
	})
	for _, tc := range []struct {
		name   string
		f      fixture
		kind   deploy.Kind
		status int
	}{
		{"500", fixture{body: `{"error":"failed to list deployments: etcd timeout"}`, status: 500}, deploy.KindUnavailable, 500},
		{"403", fixture{file: "error-text-plain-403.txt", status: 403, ctype: "text/plain"}, deploy.KindForbidden, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{latestPath: tc.f})
			_, ok, err := rs.c.Latest(ctx, project)
			if ok || deploy.KindOf(err) != tc.kind || asDeployError(t, err).Status != tc.status {
				t.Errorf("Latest on a %s = (ok %v, %v), want (false, %s)", tc.name, ok, err, tc.kind)
			}
		})
	}
}

// history reads deployments.json through Deployments.
func history(t *testing.T) []deploy.Deployment {
	t.Helper()
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/projects/engagement-portal/deployments": {file: "deployments.json"},
	})
	ds, err := rs.c.Deployments(context.Background(), project)
	if err != nil || len(ds) != 4 {
		t.Fatalf("Deployments = (%d rows, %v), want the 4 recorded rows", len(ds), err)
	}
	return ds
}

// TestLiveReadyPointer kills LiveReadyReplicas int in the DTO: measured-and-down (0) and not
// measured (no key) must read differently (models/models.go:709-723).
func TestLiveReadyPointer(t *testing.T) {
	ds := history(t)
	if ds[0].LiveReady == nil || *ds[0].LiveReady != 0 {
		t.Errorf("the newest row's liveReadyReplicas 0 read as %v, want &0", ds[0].LiveReady)
	}
	if ds[2].LiveReady != nil {
		t.Errorf("a row without the key read as &%d, want nil", *ds[2].LiveReady)
	}
}

// TestGatesAbsentIsNil kills make(map…) in the mapper: "no gate ran" is not "all clean".
func TestGatesAbsentIsNil(t *testing.T) {
	ds := history(t)
	if ds[2].Gates != nil {
		t.Errorf("a row without scanStatus read Gates %#v, want nil", ds[2].Gates)
	}
	want := map[string]deploy.Gate{
		"trivy":  {Verdict: "failed", Criticals: intp(2), Highs: intp(5)},
		"sast":   {Verdict: "passed", Findings: intp(0)},
		"secret": {Verdict: "skipped"},
	}
	if !reflect.DeepEqual(ds[0].Gates, want) {
		t.Errorf("the gates that ran read %#v, want %#v", ds[0].Gates, want)
	}
}

// TestDeploymentsMapping reads every recorded row field by field, in the order served.
func TestDeploymentsMapping(t *testing.T) {
	ds := history(t)
	gotIDs := []string{ds[0].ID, ds[1].ID, ds[2].ID, ds[3].ID}
	wantIDs := []string{"engagement-portal-deploy-7k2xq", "engagement-portal-deploy-5m9rt",
		"engagement-portal-deploy-3c8wd", "engagement-portal-deploy-1a4bz"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("order = %q, want as served %q", gotIDs, wantIDs)
	}
	d := ds[0]
	d.LiveReady, d.Gates = nil, nil // pinned by the two tests above
	want := deploy.Deployment{ID: "engagement-portal-deploy-7k2xq", Environment: "prod", Branch: "main",
		Commit: "4f0c2a9d1b7e3c5a8f6d0b2e4c6a8f1d3b5e7c9a", Version: "1.4.0",
		Image: "panaceacr.azurecr.io/engagement-portal:4f0c2a9", Build: deploy.BuildSucceeded, BuildRaw: "Succeeded",
		CreatedAt: at(t, "2026-09-23T10:00:00Z"), CompletedAt: at(t, "2026-09-23T10:04:12Z"),
		DeployedBy: "alice", Trigger: "manual",
		Provisioning: []deploy.Provisioning{
			{Component: "database", Status: "succeeded", Detail: "postgres ready"},
			{Component: "storage", Status: "failed", Detail: "bucket quota exceeded"},
		}}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("row 1 = %+v\nwant    %+v", d, want)
	}
	f := ds[1]
	if f.Build != deploy.BuildFailed || f.FailedStep != "build-image" || f.ErrorText != "step build-image exited 1" ||
		f.Trigger != "git-push" || f.DeployedBy != "bob" || f.Environment != "feat-evidence-upload" ||
		!f.CompletedAt.Equal(at(t, "2026-09-23T10:02:03Z")) {
		t.Errorf("row 2 (failed) = %+v", f)
	}
	if v := ds[2]; v.Version != "" || v.Commit == "" || v.Provisioning != nil {
		t.Errorf("row 3 (no version key) = %+v, want Version \"\", never derived from the commit", v)
	}
	h := ds[3]
	if h.Build != deploy.BuildHistorical || h.BuildRaw != "historical" || !h.Reconstructed ||
		h.DeployedBy != "alice" || !h.CompletedAt.IsZero() || h.Version != "" {
		t.Errorf("row 4 (backfilled) = %+v", h)
	}
}

// listOf wraps rows in Panacea's list envelope (models/models.go:2153-2157).
func listOf(rows ...string) string {
	return `{"items":[` + strings.Join(rows, ",") + `],"totalCount":` + strconv.Itoa(len(rows)) + `}`
}

// TestCompletedAtFromDuration kills CompletedAt = CreatedAt when the duration is missing, which
// makes a running build read as live the moment it starts (review I3). Panacea writes duration
// as Go's Duration.String() of completionTime - creationTimestamp, rounded to a second
// (clients/kubernetes.go:5683-5689), and "" while there is no completionTime.
func TestCompletedAtFromDuration(t *testing.T) {
	row := func(id, extra string) string {
		return `{"id":"` + id + `","status":"Succeeded","createdAt":"2026-09-23T10:00:00Z"` + extra + `}`
	}
	body := listOf(
		row("m", `,"duration":"4m12s"`),
		row("h", `,"duration":"1h2m3s"`),
		row("absent", ``),
		row("empty", `,"duration":""`),
		row("word", `,"duration":"soon"`),
		row("negative", `,"duration":"-5s"`),
		`{"id":"hist","status":"historical","backfilled":true,"createdAt":"2026-09-01T08:00:00Z"}`,
		`{"id":"nocreated","status":"Succeeded","createdAt":"0001-01-01T00:00:00Z","duration":"4m12s"}`,
	)
	rs := serveRecorded(t, map[string]fixture{"GET /api/v1/projects/engagement-portal/deployments": {body: body}})
	ds, err := rs.c.Deployments(context.Background(), project)
	if err != nil || len(ds) != 8 {
		t.Fatalf("Deployments = (%d, %v)", len(ds), err)
	}
	want := map[string]time.Time{"m": at(t, "2026-09-23T10:04:12Z"), "h": at(t, "2026-09-23T11:02:03Z")}
	for _, d := range ds {
		if w := want[d.ID]; !d.CompletedAt.Equal(w) {
			t.Errorf("%s: CompletedAt = %v, want %v (zero unless the duration is a real one)", d.ID, d.CompletedAt, w)
		}
	}
}

// TestStatusMappingTable: one table serves models.Deployment.Status (derivePipelineStatus,
// clients/pipeline.go:2073-2084; "Unknown" before any condition, clients/kubernetes.go:5662;
// "historical", clients/deployment_backfill.go:173) and PipelineRunStatus.Status ("Unknown,
// Running, Succeeded, Failed", models/models.go:2390). Its main mutant reads Pending as terminal
// (the "not True is failure" reading of pipeline.go:2073-2084): Track would stop the 3 s cadence
// while the build runs (t07:75-76), and Advance would write a result for a build with none.
func TestStatusMappingTable(t *testing.T) {
	for _, tc := range []struct {
		word string
		want deploy.BuildState
	}{
		{"Succeeded", deploy.BuildSucceeded},
		{"Failed", deploy.BuildFailed},
		{"Pending", deploy.BuildRunning},
		{"Running", deploy.BuildRunning},
		{"Unknown", deploy.BuildPending},
		{"historical", deploy.BuildHistorical},
		{"Cancelled", deploy.BuildUnknown},
		{"succeeded", deploy.BuildUnknown}, // Panacea's words are exact; a near miss is not one
		{"", deploy.BuildUnknown},
	} {
		t.Run("word "+strconv.Quote(tc.word), func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{
				"GET /api/v1/projects/engagement-portal/deployments": {body: listOf(
					`{"id":"r1","status":` + strconv.Quote(tc.word) + `,"createdAt":"2026-09-23T10:00:00Z"}`)},
				"GET /api/v1/projects/engagement-portal/pipelines/r1": {body: `{"data":{"name":"r1","status":` + strconv.Quote(tc.word) + `}}`},
			})
			ctx := context.Background()
			ds, err := rs.c.Deployments(ctx, project)
			if err != nil || len(ds) != 1 || ds[0].Build != tc.want || ds[0].BuildRaw != tc.word {
				t.Errorf("Deployment status %q = (%+v, %v), want Build %q, BuildRaw kept", tc.word, ds, err, tc.want)
			}
			run, err := rs.c.Run(ctx, project, "r1")
			if err != nil || run.Build != tc.want || run.BuildRaw != tc.word {
				t.Errorf("Run status %q = (%+v, %v), want Build %q, BuildRaw kept", tc.word, run, err, tc.want)
			}
			if tc.word == "Pending" && run.Build.Terminal() {
				t.Errorf("Pending mapped to %q, which is terminal: a running build would stop being tracked", run.Build)
			}
		})
	}
}

// TestSameEnvironment: exact equality plus the one equivalence Panacea applies, from
// clients/kubernetes.go:4213 —
//
//	if env != "" && env != "prod" && env != "production" {
//
// "prod" and "production" both name the base namespace. "" does too in that line, and that part
// is NOT adopted: "" is a legacy run with no environment label, which Panacea's environments
// list attributes by its branch label (:5396-5415), not to production (review M5).
func TestSameEnvironment(t *testing.T) {
	c := &Client{}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"prod", "production", true},
		{"production", "prod", true},
		{"prod", "prod", true},
		{"feat-a", "feat-a", true},
		{"prod", "prod-eu", false},
		{"production", "production-eu", false},
		{"feat-a", "feat-b", false},
		{"", "production", false},
		{"", "prod", false},
		{"production", "", false},
		{"prod", "", false},
	} {
		if got := c.SameEnvironment(tc.a, tc.b); got != tc.want {
			t.Errorf("SameEnvironment(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestPathEscapesBranchAndEnv kills an unescaped segment: a "/" routes to a different Panacea
// handler, a "?" or "#" cuts the path, and a query escape sends a space as "+".
func TestPathEscapesBranchAndEnv(t *testing.T) {
	const P = "GET /api/v1/projects/engagement-portal"
	want := []string{
		P + "/environments/feat%2Fx/pods",
		P + "/environments/feat%2Fx/events",
		P + "/environments/feat%2Fx/diagnose",
		P + "/environments/a%3Fb%23c%20d/pods",
		P + "/pipelines/run%2F1",
		"GET /api/v1/projects/team%2Fportal%20two/environments",
	}
	rs := serveRecorded(t, map[string]fixture{
		want[0]: {file: "pods.json"}, want[1]: {file: "events.json"}, want[2]: {file: "diagnose-ai.json"},
		want[3]: {file: "pods.json"}, want[4]: {file: "run-running.json"}, want[5]: {file: "environments.json"},
	})
	ctx := context.Background()
	_, e1 := rs.c.Pods(ctx, project, "feat/x")
	_, e2 := rs.c.Events(ctx, project, "feat/x")
	_, e3 := rs.c.Diagnosis(ctx, project, "feat/x")
	_, e4 := rs.c.Pods(ctx, project, "a?b#c d")
	_, e5 := rs.c.Run(ctx, project, "run/1")
	_, e6 := rs.c.Environments(ctx, "team/portal two")
	if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
		t.Errorf("reads: %v", err)
	}
	if got := rs.requests(); !slices.Equal(got, want) {
		t.Errorf("requests = %q\nwant       %q", got, want)
	}
}

const accessPath = "GET /api/v1/projects/engagement-portal/branches/"

// TestEnvironmentForReadsPanaceasName kills returning the branch unchanged, or a copy of
// sanitizeBranchEnv in the connector (review C2): the answer is data.env, verbatim, from the
// access route, and no environment row is read to find it. The first three answers are ones a
// local copy of the naming rule also produces, so they alone let such a copy through (mutant M12
// survived them in the t03 run); risk-register's are not: it is labelled production-branch=release,
// so release answers prod and main answers main (handlers/projects.go:3839-3846).
func TestEnvironmentForReadsPanaceasName(t *testing.T) {
	const custom = "GET /api/v1/projects/risk-register/branches/"
	rs := serveRecorded(t, map[string]fixture{
		accessPath + "feat%2Fx/access":                          {file: "branch-access-feat-x.json"},
		accessPath + "lane%2F7-deploy-connector-watcher/access": {file: "branch-access-long.json"},
		accessPath + "main/access":                              {file: "branch-access-main.json"},
		accessPath + "empty/access":                             {body: `{"data":{"branch":"empty","env":"","accessMode":"kpmg-sso"}}`},
		accessPath + "noenv/access":                             {body: `{"data":{"branch":"noenv","accessMode":"kpmg-sso"}}`},
		accessPath + "nodata/access":                            {body: `{}`},
		custom + "release/access":                               {file: "branch-access-custom-release.json"},
		custom + "main/access":                                  {file: "branch-access-custom-main.json"},
	})
	ctx := context.Background()
	for branch, want := range map[string]string{
		"feat/x": "feat-x", "lane/7-deploy-connector-watcher": "lane-7-deploy-connec", "main": "prod",
	} {
		if got, err := rs.c.EnvironmentFor(ctx, project, branch); err != nil || got != want {
			t.Errorf("EnvironmentFor(%q) = (%q, %v), want Panacea's %q", branch, got, err, want)
		}
	}
	for branch, want := range map[string]string{"release": "prod", "main": "main"} {
		if got, err := rs.c.EnvironmentFor(ctx, "risk-register", branch); err != nil || got != want {
			t.Errorf("risk-register EnvironmentFor(%q) = (%q, %v), want Panacea's %q", branch, got, err, want)
		}
	}
	// Panacea never writes an empty env (mapBranchToEnv and sanitizeBranchEnv never return "",
	// handlers/projects.go:3783-3826). An empty or missing one is an unusable 200: KindUnavailable
	// with the status received (re-review M-4).
	for _, branch := range []string{"empty", "noenv", "nodata"} {
		got, err := rs.c.EnvironmentFor(ctx, project, branch)
		if got != "" {
			t.Errorf("EnvironmentFor(%q) = %q, want \"\" with the error", branch, got)
		}
		wantKind(t, err, deploy.KindUnavailable, 200, "")
	}
	for _, r := range rs.requests() {
		if strings.Contains(r, "/environments") {
			t.Errorf("EnvironmentFor read an environment row: %s", r)
		}
	}
	if got := len(rs.requests()); got != 8 {
		t.Errorf("EnvironmentFor made %d requests, want 8 (one each)", got)
	}
}

// TestEnvironmentForRouteNotFound kills a refusal that tells the user their project does not
// exist. The route's own chain never answers 404 (authedProject, routes.go:3279-3285:
// RequireProjectAccess answers 401 or 403, middleware/rbac.go:181-183, :221-228;
// GetBranchAccessMode writes only 400, 503 or 200, handlers/project_security.go:634-678), so a
// 404 is a router's for a path whose %2F was decoded into an extra segment (b02 round 2, R2;
// re-review b02-2 M-2). An ingress 400 with no problem body gets the same sentence (re-review
// b02-2 M-3); the handler's own 400 is JSON, for a blank name or branch (:637-639), and keeps
// Panacea's sentence.
func TestEnvironmentForRouteNotFound(t *testing.T) {
	plain := "text/plain; charset=utf-8"
	rs := serveRecorded(t, map[string]fixture{
		accessPath + "a%2Fb/access": {body: "404 page not found\n", status: 404, ctype: plain},
		accessPath + "c%2Fd/access": {body: `{"error":"no such project"}`, status: 404},
		accessPath + "e%2Ff/access": {body: "<html><body><h1>400 Bad Request</h1></body></html>", status: 400, ctype: "text/html"},
		accessPath + "g%2Fh/access": {body: "Bad Request\n", status: 400, ctype: plain},
		accessPath + "%20/access":   {body: `{"error":"project name and branch are required"}`, status: 400},
		accessPath + "i%2Fj/access": {file: "error-text-plain-403.txt", status: 403, ctype: plain},
	})
	ctx := context.Background()
	for _, tc := range []struct {
		branch  string
		kind    deploy.Kind
		status  int
		message string
	}{
		{"a/b", deploy.KindNotFound, 404, branchUnreadable},
		{"c/d", deploy.KindNotFound, 404, branchUnreadable},
		{"e/f", deploy.KindRejected, 400, branchUnreadable},
		{"g/h", deploy.KindRejected, 400, branchUnreadable},
		{" ", deploy.KindRejected, 400, "project name and branch are required"},
		{"i/j", deploy.KindForbidden, 403, "no access yet: your account is waiting for team approval — finish onboarding first"},
	} {
		env, err := rs.c.EnvironmentFor(ctx, project, tc.branch)
		if env != "" {
			t.Errorf("EnvironmentFor(%q) = %q with an error", tc.branch, env)
		}
		wantKind(t, err, tc.kind, tc.status, tc.message)
	}
	if branchUnreadable != "Panacea could not read this branch name. It may contain `/`, which the route cannot carry." {
		t.Errorf("branchUnreadable = %q, not the plan's sentence (task 03; t15:230 renders it)", branchUnreadable)
	}
}

// TestEnvironmentForIsTheAccessRoutesMapping pins that the answer can be WRONG and is kept:
// branchEnv reads the production branch from the production-branch label only
// (handlers/branch_security.go:32), so on a project whose production branch is the annotation
// primary-branch=develop it answers develop for develop, while the deploy resolves the
// annotation and lands develop in prod (clients/pipeline.go:233-237). The connector copies no
// resolver to "fix" it; task 08 rule 5a detects the disagreement from the deployment (Q-P7-6).
func TestEnvironmentForIsTheAccessRoutesMapping(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{accessPath + "develop/access": {file: "branch-access-annotation-only.json"}})
	if got, err := rs.c.EnvironmentFor(context.Background(), project, "develop"); err != nil || got != "develop" {
		t.Errorf("EnvironmentFor(develop) = (%q, %v), want Panacea's answer develop, not prod", got, err)
	}
}

// TestEnvironmentForJoinsProduction kills an exact-name join that leaves the production branch
// with no card: Panacea answers "prod" for main, and names the row "production".
func TestEnvironmentForJoinsProduction(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/projects/engagement-portal/environments": {file: "environments.json"},
		accessPath + "main/access":                            {file: "branch-access-main.json"},
	})
	ctx := context.Background()
	envs, err := rs.c.Environments(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	env, err := rs.c.EnvironmentFor(ctx, project, "main")
	if err != nil {
		t.Fatal(err)
	}
	var joined []string
	for _, e := range envs {
		if rs.c.SameEnvironment(env, e.Name) {
			joined = append(joined, e.Name)
		}
	}
	if !slices.Equal(joined, []string{"production"}) {
		t.Errorf("EnvironmentFor(main) = %q joins rows %q, want exactly [production]", env, joined)
	}
}

const scansPath = "GET /api/v1/projects/engagement-portal/security-status"

// TestScansSendsTheEnvironment kills dropping env, which shows production's verdict as a branch
// release's, and sending env= empty, which Panacea also reads as production
// (handlers/security_status.go:192-201; review C3). The query is url.Values.Encode's, keys
// sorted, as the portal builds it (SecurityScanReview.tsx:437-440).
func TestScansSendsTheEnvironment(t *testing.T) {
	want := []string{
		scansPath + "?deployId=d-feat&env=feat-x",
		scansPath + "?deployId=d1&env=prod",
		scansPath + "?deployId=d1",
	}
	routes := map[string]fixture{}
	for _, k := range want {
		routes[k] = fixture{file: "security-status.json"}
	}
	rs := serveRecorded(t, routes)
	ctx := context.Background()
	for _, a := range [][2]string{{"feat-x", "d-feat"}, {"prod", "d1"}, {"", "d1"}} {
		if _, err := rs.c.Scans(ctx, project, a[0], a[1]); err != nil {
			t.Errorf("Scans(%q, %q): %v", a[0], a[1], err)
		}
	}
	if got := rs.requests(); !slices.Equal(got, want) {
		t.Errorf("requests = %q\nwant       %q", got, want)
	}
}

// TestScansMapping reads the recorded report field by field (models/security.go:117-242).
func TestScansMapping(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		scansPath + "?deployId=engagement-portal-deploy-5m9rt&env=feat-evidence-upload": {file: "security-status.json"},
	})
	got, err := rs.c.Scans(context.Background(), project, "feat-evidence-upload", "engagement-portal-deploy-5m9rt")
	if err != nil {
		t.Fatal(err)
	}
	f := false
	want := deploy.ScanReport{
		DeploymentID: "engagement-portal-deploy-5m9rt", Overall: "FAIL",
		Blocking:   []deploy.ScanNote{{Scan: "trivy", Reason: "trivy enforce failed: 2 critical, 5 high"}},
		Incomplete: []deploy.ScanNote{{Scan: "dast", Reason: "dast produced no result: TaskRunImagePullFailed"}},
		Scans: []deploy.Scan{
			// ErrorReason is the Tekton reason for a scan that ran and produced nothing
			// (models/security.go:143-158): the one fact that says why a gate is incomplete.
			{Name: "dast", Status: "ERROR", Mode: "enforce", Counts: map[string]int{}, ErrorReason: "TaskRunImagePullFailed",
				LastScanAt: "2026-09-23T09:19:00Z", FindingsSource: "unverified"},
			{Name: "sast", Status: "PASS", Mode: "warn", Counts: map[string]int{"findings": 0}, LastScanAt: "2026-09-23T09:18:00Z",
				FindingsSource: "unverified"},
			{Name: "secret", Status: "SKIPPED", Mode: "off", Counts: map[string]int{}, SkipReason: "posture-off",
				Summary: "not enabled for this branch — turn it on in the Security Scans tab to run it on every build"},
			{Name: "trivy", Status: "FAIL", Mode: "enforce", Counts: map[string]int{"critical": 2, "high": 5, "medium": 11, "low": 3},
				Summary: "2 critical, 5 high", Blocking: true, LastScanAt: "2026-09-23T09:20:00Z", FindingsSource: "unverified"},
		},
		DetailsRetained: &f, DetailsSource: "unverified", UpdatedAt: "2026-09-23T09:21:00Z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scans = %+v\nwant    %+v", got, want)
	}
}

// TestRunMapping reads both recorded runs, and keeps Panacea's IDOR answer a not-found
// (handlers/projects.go:3598-3601).
func TestRunMapping(t *testing.T) {
	const P = "GET /api/v1/projects/engagement-portal/pipelines/"
	rs := serveRecorded(t, map[string]fixture{
		P + "engagement-portal-deploy-9q3vn": {file: "run-running.json"},
		P + "engagement-portal-deploy-5m9rt": {file: "run-failed.json"},
		P + "other-project-run":              {body: `{"error":"pipeline run not found for this project"}`, status: 404},
	})
	ctx := context.Background()
	got, err := rs.c.Run(ctx, project, "engagement-portal-deploy-9q3vn")
	want := deploy.RunProgress{Run: "engagement-portal-deploy-9q3vn", Build: deploy.BuildRunning, BuildRaw: "Pending",
		StartedAt: at(t, "2026-09-23T11:00:05Z"), CurrentStep: "build-image", CompletedSteps: []string{"fetch-source", "detect-env"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Run(running) = (%+v, %v)\nwant %+v", got, err, want)
	}
	got, err = rs.c.Run(ctx, project, "engagement-portal-deploy-5m9rt")
	want = deploy.RunProgress{Run: "engagement-portal-deploy-5m9rt", Build: deploy.BuildFailed, BuildRaw: "Failed",
		StartedAt: at(t, "2026-09-23T09:00:02Z"), FailedStep: "build-image", ErrorText: "step build-image exited 1",
		CompletedSteps: []string{"fetch-source", "detect-env", "trivy-scan"}, IgnoredFailures: []string{"sign-release"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Run(failed) = (%+v, %v)\nwant %+v", got, err, want)
	}
	_, err = rs.c.Run(ctx, project, "other-project-run")
	wantKind(t, err, deploy.KindNotFound, 404, "pipeline run not found for this project")
}

// TestEnvironmentsMapping: replicas is present only when the namespace was read
// (handlers/projects.go:2125-2140), Host is the subdomain, and Branch is the row's label
// verbatim, never the git branch (:2109-2111).
func TestEnvironmentsMapping(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET /api/v1/projects/engagement-portal/environments": {file: "environments.json"}})
	got, err := rs.c.Environments(context.Background(), project)
	want := []deploy.Environment{
		{Name: "production", Branch: "main", Primary: true, Host: "engagement-portal.dev.panacea.kpmg",
			Status: "running", Replicas: &deploy.Replicas{Ready: 2, Desired: 2}},
		{Name: "feat-evidence-upload", Branch: "feat-evidence-upload",
			Host: "engagement-portal-feat-evidence-upload.dev.panacea.kpmg", Status: "pending"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Environments = (%+v, %v)\nwant %+v", got, err, want)
	}
}

// TestDrillInsMapping reads the pods, events and diagnosis bodies verbatim. An event time that
// is not RFC 3339 reads as zero, never as now.
func TestDrillInsMapping(t *testing.T) {
	const P = "GET /api/v1/projects/engagement-portal/environments/feat-evidence-upload/"
	rs := serveRecorded(t, map[string]fixture{
		P + "pods": {file: "pods.json"}, P + "events": {file: "events.json"}, P + "diagnose": {file: "diagnose-ai.json"},
	})
	ctx := context.Background()
	const env = "feat-evidence-upload"
	pods, err := rs.c.Pods(ctx, project, env)
	wantPods := []deploy.Pod{
		{Name: "engagement-portal-7c9d5b6f4-x2kqp", Phase: "Running", Restarts: 7},
		{Name: "engagement-portal-7c9d5b6f4-m8wzt", Phase: "Failed", Reason: "Evicted", Message: "The node was low on resource: memory."},
	}
	if err != nil || !reflect.DeepEqual(pods, wantPods) {
		t.Errorf("Pods = (%+v, %v)\nwant %+v", pods, err, wantPods)
	}
	events, err := rs.c.Events(ctx, project, env)
	wantEvents := []deploy.Event{
		{Type: "Warning", Reason: "BackOff", Object: "Pod/engagement-portal-7c9d5b6f4-x2kqp",
			Message:  "Back-off restarting failed container app in pod engagement-portal-7c9d5b6f4-x2kqp",
			LastSeen: at(t, "2026-09-23T09:31:07Z"), Count: 42},
		{Type: "Normal", Reason: "Pulled", Object: "Pod/engagement-portal-7c9d5b6f4-x2kqp",
			Message: "Container image already present on machine", Count: 1},
	}
	if err != nil || !reflect.DeepEqual(events, wantEvents) {
		t.Errorf("Events = (%+v, %v)\nwant %+v", events, err, wantEvents)
	}
	// Every DiagnoseFinding field (handlers/diagnose.go:60-67), compared whole: Problem and
	// SuggestedFix are what the card says and what the user does (mutants X3, X1 of the t03 run
	// survived a field-by-field check that skipped them). Evidence absent stays nil.
	d, err := rs.c.Diagnosis(ctx, project, env)
	wantDiag := deploy.Diagnosis{Health: "down", Summary: "The app container exits on start, so no pod is ready.", Source: "rules+ai",
		Findings: []deploy.Finding{
			{Problem: "The app container is crash-looping.",
				LikelyCause:  "DATABASE_URL is not set in this environment, so the app exits at boot.",
				SuggestedFix: "Add DATABASE_URL under Env Vars for this branch and redeploy.",
				Evidence:     []string{"BackOff x42 on Pod/engagement-portal-7c9d5b6f4-x2kqp", "restarts: 7"}, Severity: "critical", Owner: "app"},
			{Problem: "One pod was evicted for memory pressure.", LikelyCause: "The node ran low on memory.",
				SuggestedFix: "No action; the platform reschedules evicted pods.", Severity: "info", Owner: "platform"},
		}}
	if err != nil || !reflect.DeepEqual(d, wantDiag) {
		t.Errorf("Diagnosis = (%+v, %v)\nwant %+v", d, err, wantDiag)
	}
}

// TestPodReadyIsPanaceas: Ready is Panacea's own verdict, "all app containers ready AND phase
// Running" (clients/pod_introspection.go:50), read verbatim. Both recorded pods are not ready, so
// a mapper that drops the field passed them (mutant X4 of the t03 run).
func TestPodReadyIsPanaceas(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET /api/v1/projects/engagement-portal/environments/prod/pods": {body: `{"items":[` +
		`{"name":"a","phase":"Running","ready":true,"readyContainers":1,"totalContainers":1,"restarts":0,"image":"i","ageSeconds":60,"containers":[]},` +
		`{"name":"b","phase":"Running","ready":false,"readyContainers":0,"totalContainers":1,"restarts":2,"image":"i","ageSeconds":60,"containers":[]}` +
		`],"totalCount":2}`}})
	pods, err := rs.c.Pods(context.Background(), project, "prod")
	want := []deploy.Pod{{Name: "a", Phase: "Running", Ready: true}, {Name: "b", Phase: "Running", Restarts: 2}}
	if err != nil || !reflect.DeepEqual(pods, want) {
		t.Errorf("Pods = (%+v, %v)\nwant %+v", pods, err, want)
	}
}

// TestEmptySegmentRefusedBeforeRequest: an empty, "." or ".." segment would name a different
// route (a mux cleans dot segments with a redirect), so it is refused with no request.
func TestEmptySegmentRefusedBeforeRequest(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{})
	ctx := context.Background()
	_, e1 := rs.c.Environments(ctx, "")
	_, e2 := rs.c.EnvironmentFor(ctx, project, "")
	_, _, e3 := rs.c.Latest(ctx, "..")
	_, e4 := rs.c.Deployments(ctx, ".")
	_, e5 := rs.c.Run(ctx, project, "")
	_, e6 := rs.c.Scans(ctx, project, "prod", "")
	_, e7 := rs.c.Pods(ctx, project, "")
	_, e8 := rs.c.Events(ctx, project, ".")
	_, e9 := rs.c.Diagnosis(ctx, project, "..")
	for i, err := range []error{e1, e2, e3, e4, e5, e6, e7, e8, e9} {
		if k := deploy.KindOf(err); k != deploy.KindRejected {
			t.Errorf("call %d: Kind %q (%v), want rejected", i+1, k, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("requests = %q, want none", got)
	}
}

// TestRecordedBodiesNameTheirSource: every file under testdata/ names the Panacea line it was
// written from, so a reviewer can re-check it in one open.
func TestRecordedBodiesNameTheirSource(t *testing.T) {
	entries, err := fs.ReadDir(os.DirFS("testdata"), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		loadFixture(t, e.Name())
	}
	if len(entries) < 50 {
		t.Errorf("testdata holds %d files, want task 03's 24 plus task 04's 26 (its 21, the four grant 500s, the stale clear)", len(entries))
	}
}

// TestRunNameIsTheOneAsked: a run answered with no name reads as the run asked for, which
// GetPipelineRun read by that name (clients/pipeline.go:1064), never "", which would break the
// run-to-deployment join (mutation survivor N1, named before the t03 run).
func TestRunNameIsTheOneAsked(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/projects/engagement-portal/pipelines/r1": {body: `{"data":{"status":"Pending"}}`},
	})
	got, err := rs.c.Run(context.Background(), project, "r1")
	if err != nil || got.Run != "r1" || got.Build != deploy.BuildRunning {
		t.Errorf("Run(r1) with no name = (%+v, %v), want Run r1, running", got, err)
	}
}

// TestReadsAcceptOnly200: every read's handler answers 200 on success, so any other 2xx is an
// answer this code does not know: KindUnavailable with that status, never decoded as success
// (mutation survivor N2, named before the t03 run).
func TestReadsAcceptOnly200(t *testing.T) {
	const P = "GET /api/v1/projects/engagement-portal"
	rs := serveRecorded(t, map[string]fixture{
		"GET /api/v1/me":                   {file: "me-approved.json", status: 202},
		"GET /api/v1/projects":             {file: "projects.json", status: 203},
		P + "/environments":                {file: "environments.json", status: 201},
		P + "/branches/feat%2Fx/access":    {file: "branch-access-feat-x.json", status: 202},
		P + "/deploys/latest":              {file: "latest.json", status: 202},
		P + "/deployments":                 {file: "deployments.json", status: 206},
		P + "/pipelines/r1":                {file: "run-running.json", status: 202},
		P + "/security-status?deployId=d1": {file: "security-status.json", status: 202},
		P + "/environments/e1/pods":        {file: "pods.json", status: 202},
		P + "/environments/e1/events":      {file: "events.json", status: 202},
		P + "/environments/e1/diagnose":    {file: "diagnose-ai.json", status: 202},
	})
	ctx := context.Background()
	_, e1 := rs.c.Whoami(ctx)
	_, e2 := rs.c.Projects(ctx)
	_, e3 := rs.c.Environments(ctx, project)
	_, e4 := rs.c.EnvironmentFor(ctx, project, "feat/x")
	_, ok, e5 := rs.c.Latest(ctx, project)
	_, e6 := rs.c.Deployments(ctx, project)
	_, e7 := rs.c.Run(ctx, project, "r1")
	_, e8 := rs.c.Scans(ctx, project, "", "d1")
	_, e9 := rs.c.Pods(ctx, project, "e1")
	_, e10 := rs.c.Events(ctx, project, "e1")
	_, e11 := rs.c.Diagnosis(ctx, project, "e1")
	want := []int{202, 203, 201, 202, 202, 206, 202, 202, 202, 202, 202}
	for i, err := range []error{e1, e2, e3, e4, e5, e6, e7, e8, e9, e10, e11} {
		if deploy.KindOf(err) != deploy.KindUnavailable || asDeployError(t, err).Status != want[i] {
			t.Errorf("read %d answered %d = %v, want KindUnavailable with Status %d", i+1, want[i], err, want[i])
		}
	}
	if ok {
		t.Error("Latest answered 202 read as a deployment")
	}
}

// TestGatesEmptyMapIsNil: Panacea's omitempty never sends "scanStatus": {} (models/models.go:724-781),
// and one would name no gate that ran, so it reads nil like an absent key: "none ran", never "all
// clean" (mutant G3 of the t03 run).
func TestGatesEmptyMapIsNil(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET /api/v1/projects/engagement-portal/deployments": {body: listOf(
		`{"id":"e","status":"Succeeded","createdAt":"2026-09-23T10:00:00Z","scanStatus":{}}`)}})
	ds, err := rs.c.Deployments(context.Background(), project)
	if err != nil || len(ds) != 1 || ds[0].Gates != nil {
		t.Errorf("Deployments with scanStatus {} = (%+v, %v), want one row with Gates nil", ds, err)
	}
}
