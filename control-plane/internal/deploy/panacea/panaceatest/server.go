// Package panaceatest is a fake panacea-api for tests: an httptest TLS server that holds a model
// and answers the routes of plan §4 in Panacea's exact envelopes. It implements
// deploytest.Seeder, so deploytest.Conformance runs unchanged against the real connector.
// Task 03 answers the reads; task 04 adds the write routes.
//
// Its wire shapes are declared here, apart from package panacea's DTOs, with the omitempty tags
// Panacea's own types carry (models/models.go, handlers/*.go at 406d0da95): a struct shared by the
// connector and its fake would agree with itself however wrong its tags were. The recorded
// bodies under panacea/testdata/ are what pin the connector to Panacea's source.
//
// It holds NO naming rule: an environment's name, a branch's environment and a run's name are all
// seeded. The one Panacea rule it copies is how security-status resolves env (Server.Scans).
package panaceatest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
)

// token is the only bearer the server accepts.
const token = "pnc_test"

// Server is a fake panacea-api. Every method is safe for concurrent use.
type Server struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	reqs     []string
	order    []deploy.ProjectRef // Project order: /projects answers in it
	projects map[deploy.ProjectRef]*project
}

type scanKey struct{ env, deploymentID string }

// project is one project's seeded state, held in wire form so a seed is copied once.
type project struct {
	info        deploy.Project
	listed      bool
	envs        []wireEnvironment
	branchEnv   map[string]string
	postureWord map[string]string
	deps        []wireDeployment // newest first by CreatedAt
	scans       map[scanKey]wireSecurityStatus
	rollbacks   map[string]deploy.Warmth // task 04's rollback route answers from it
}

var _ deploytest.Seeder = (*Server)(nil)

// New starts the server; it closes with t.Cleanup. A seeding gap is a test bug, reported with
// t.Errorf and answered 500, where real Panacea would answer something.
func New(t *testing.T) *Server {
	t.Helper()
	s := &Server{t: t, projects: map[deploy.ProjectRef]*project{}}
	const P = "/api/v1/projects/{name}"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", s.me)
	mux.HandleFunc("GET /api/v1/projects", s.listProjects)
	mux.HandleFunc("GET "+P+"/environments", s.inProject(s.environments))
	mux.HandleFunc("GET "+P+"/branches/{branch}/access", s.inProject(s.branchAccess))
	mux.HandleFunc("GET "+P+"/deploys/latest", s.inProject(s.latest))
	mux.HandleFunc("GET "+P+"/deployments", s.inProject(s.deployments))
	mux.HandleFunc("GET "+P+"/pipelines/{run}", s.inProject(s.run))
	mux.HandleFunc("GET "+P+"/security-status", s.inProject(s.securityStatus))
	for _, drill := range []string{"pods", "events", "diagnose"} {
		mux.HandleFunc("GET "+P+"/environments/{env}/"+drill, s.inProject(s.notModelled))
	}
	s.srv = httptest.NewTLSServer(s.gate(mux))
	t.Cleanup(s.srv.Close)
	return s
}

// URL is the https origin.
func (s *Server) URL() string { return s.srv.URL }

// HTTPClient trusts the server's certificate.
func (s *Server) HTTPClient() *http.Client { return s.srv.Client() }

// Token is the only token the server accepts.
func (s *Server) Token() string { return token }

// Requests is every request received, in order, as "METHOD /escaped/path?query".
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reqs)
}

// gate records the request, then refuses any bearer but token as apiKeyMW does
// (middleware/api_key.go:171, writeAPIKeyError :396-400).
func (s *Server) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, r.Method+" "+r.URL.RequestURI())
		s.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+token {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// inProject is authedProject's membership gate (routes.go:3279-3285): a project nothing seeded
// is one the caller holds no role on, so RequireProjectAccess's 403 answers, as http.Error
// writes it (middleware/rbac.go:221-228). The handler runs with s.mu held.
func (s *Server) inProject(h func(http.ResponseWriter, *http.Request, *project)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		pr := s.projects[deploy.ProjectRef(r.PathValue("name"))]
		if pr == nil {
			http.Error(w, `{"error":"forbidden: you do not have access to this project"}`, http.StatusForbidden)
			return
		}
		h(w, r, pr)
	}
}

// writeJSON and writeError are Panacea's (handlers/helpers.go:34-43).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// testBug reports a seeding gap and answers 500.
func (s *Server) testBug(w http.ResponseWriter, format string, a ...any) {
	s.t.Errorf("panaceatest: "+format, a...)
	writeError(w, http.StatusInternalServerError, "panaceatest: not seeded")
}

// listOf is models.ListResponse (models/models.go:2153-2157).
type listOf[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
}

func list[T any](items []T) listOf[T] {
	if items == nil {
		items = []T{}
	}
	return listOf[T]{Items: items, TotalCount: len(items)}
}

// data is models.APIResponse{Data} (models/models.go:2147-2151).
type data struct {
	Data any `json:"data"`
}

// me answers GET /api/v1/me (handlers/me.go:48-59). No seeder names an account, so it is the
// zero Account the Fake answers unseeded: not approved.
func (s *Server) me(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"username": "", "email": "", "groups": []string{}, "isAdmin": false, "role": "none", "approved": false,
	})
}

// wireProject is models.Project's always-written fields plus the four phase 7 reads, with
// Panacea's tags (models/models.go:563-623).
type wireProject struct {
	Name          string    `json:"name"`
	DisplayName   string    `json:"displayName,omitempty"`
	RepoURL       string    `json:"repoUrl,omitempty"`
	DefaultBranch string    `json:"defaultBranch,omitempty"`
	Owner         string    `json:"owner"`
	Groups        []string  `json:"groups"`
	TeamVisible   bool      `json:"teamVisible"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// listProjects answers GET /api/v1/projects (handlers/projects.go:785) in Project order.
func (s *Server) listProjects(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []wireProject
	for _, ref := range s.order {
		p := s.projects[ref].info
		out = append(out, wireProject{Name: string(p.Ref), DisplayName: p.DisplayName, RepoURL: p.RepoURL,
			DefaultBranch: p.DefaultBranch, Groups: []string{}})
	}
	writeJSON(w, http.StatusOK, list(out))
}

// environments answers GET P/environments (handlers/projects.go:2116-2152).
func (s *Server) environments(w http.ResponseWriter, _ *http.Request, pr *project) {
	writeJSON(w, http.StatusOK, list(pr.envs))
}

// branchAccess answers GET P/branches/{branch}/access (handlers/project_security.go:668-677),
// the branch taken from the mux's unescaped segment. env comes from the BranchEnvironment seed
// and accessMode from the PostureWord seed (task 04 reads it); each is omitted when unseeded,
// which real Panacea never does, so EnvironmentFor on a posture-only branch reads a missing env.
// A branch seeded in neither is a test bug, where real Panacea answers for any branch.
func (s *Server) branchAccess(w http.ResponseWriter, r *http.Request, pr *project) {
	branch := r.PathValue("branch")
	env, hasEnv := pr.branchEnv[branch]
	word, hasWord := pr.postureWord[branch]
	if !hasEnv && !hasWord {
		s.testBug(w, "branch %q seeded in neither table", branch)
		return
	}
	body := map[string]any{"branch": branch, "ipWhitelist": []string{}, "passwordSet": false, "bypassTokenCount": 0}
	if hasEnv {
		body["env"] = env
	}
	if hasWord {
		body["accessMode"] = word
	}
	writeJSON(w, http.StatusOK, data{body})
}

// latest answers GET P/deploys/latest: the bare newest deployment, or Panacea's 404 when the
// project has none (handlers/projects.go:1537-1545).
func (s *Server) latest(w http.ResponseWriter, r *http.Request, pr *project) {
	if len(pr.deps) == 0 {
		writeError(w, http.StatusNotFound, "no deployments yet for project "+r.PathValue("name"))
		return
	}
	writeJSON(w, http.StatusOK, pr.deps[0])
}

// deployments answers GET P/deployments, newest first (handlers/projects.go:1252).
func (s *Server) deployments(w http.ResponseWriter, _ *http.Request, pr *project) {
	writeJSON(w, http.StatusOK, list(pr.deps))
}

// wireRun is the subset of models.PipelineRunStatus a seeded deployment knows
// (models/models.go:2385-2410).
type wireRun struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	StartTime    string `json:"startTime,omitempty"`
	FailedStep   string `json:"failedStep,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// run answers GET P/pipelines/{run} from the deployment the run built (a deployment's ID is its
// run's name). An unknown run is GetPipelineRunStatus's 404 (handlers/projects.go:3588-3593).
func (s *Server) run(w http.ResponseWriter, r *http.Request, pr *project) {
	name := r.PathValue("run")
	i := slices.IndexFunc(pr.deps, func(d wireDeployment) bool { return d.ID == name })
	if i < 0 {
		writeError(w, http.StatusNotFound, "pipeline run not found: pipelineruns.tekton.dev \""+name+"\" not found")
		return
	}
	d := pr.deps[i]
	writeJSON(w, http.StatusOK, data{wireRun{Name: d.ID, Status: d.Status,
		StartTime: d.CreatedAt.UTC().Format(time.RFC3339), FailedStep: d.FailedStep, ErrorMessage: d.ErrorMessage}})
}

// notModelled answers the drill-ins no seeder names: the recorded bodies test those routes.
func (s *Server) notModelled(w http.ResponseWriter, r *http.Request, _ *project) {
	s.testBug(w, "%s is not modelled; panacea/testdata's recorded bodies test it", r.URL.Path)
}

// productionKey is the one Panacea rule this server copies, because it is the rule the review C3
// mutant hides behind: security-status reads a missing env as "production" (resolveEnv,
// handlers/security_status.go:192-201), and tenantNamespaceForEnv sends "", "prod" and
// "production" to the base namespace (handlers/tier_store.go:106-113). Both trim the value.
// So a connector that drops env is answered with production's report, as live Panacea answers it.
func productionKey(env string) string {
	env = strings.TrimSpace(env)
	if env == "" || env == "prod" || env == "production" {
		return "production"
	}
	return env
}

// securityStatus answers GET P/security-status?env=&deployId= (handlers/security_status.go:204-254)
// from the Scans seed, resolving env as productionKey does. A pair seeded in neither form is a
// test bug; real Panacea answers 200 with NOT_RUN or empty rows.
func (s *Server) securityStatus(w http.ResponseWriter, r *http.Request, pr *project) {
	q := r.URL.Query()
	env, id := q.Get("env"), strings.TrimSpace(q.Get("deployId"))
	rep, ok := pr.scans[scanKey{productionKey(env), id}]
	if !ok {
		s.testBug(w, "no scans seeded for env %q, deploy %q", env, id)
		return
	}
	writeJSON(w, http.StatusOK, data{rep})
}

// wireEnvironment is one ListEnvironments row (handlers/projects.go:2116-2140): replicas only
// when the namespace was read.
type wireEnvironment struct {
	Name        string        `json:"name"`
	Branch      string        `json:"branch"`
	DisplayName string        `json:"displayName"`
	Primary     bool          `json:"primary"`
	Subdomain   string        `json:"subdomain"`
	Status      string        `json:"status"`
	LastDeploy  string        `json:"lastDeploy"`
	Replicas    *wireReplicas `json:"replicas,omitempty"`
}

type wireReplicas struct {
	Current int `json:"current"`
	Desired int `json:"desired"`
}

// wireDeployment is models.Deployment with Panacea's tags (models/models.go:665-770).
type wireDeployment struct {
	ID                string                  `json:"id"`
	ProjectName       string                  `json:"projectName"`
	Revision          string                  `json:"revision"`
	Status            string                  `json:"status"`
	Image             string                  `json:"image,omitempty"`
	Version           string                  `json:"version,omitempty"`
	Commit            string                  `json:"commit,omitempty"`
	Branch            string                  `json:"branch,omitempty"`
	Environment       string                  `json:"environment,omitempty"`
	Duration          string                  `json:"duration,omitempty"`
	FailedStep        string                  `json:"failedStep,omitempty"`
	ErrorMessage      string                  `json:"errorMessage,omitempty"`
	CreatedAt         time.Time               `json:"createdAt"`
	Provisioning      []wireProvisioning      `json:"provisioning,omitempty"`
	LiveReadyReplicas *int                    `json:"liveReadyReplicas,omitempty"`
	ScanStatus        map[string]wireScanGate `json:"scanStatus,omitempty"`
	DeployedBy        string                  `json:"deployedBy,omitempty"`
	Trigger           string                  `json:"trigger,omitempty"`
	Backfilled        bool                    `json:"backfilled,omitempty"`
}

// wireProvisioning is models.ProvisioningStatus (models/models.go:794-804).
type wireProvisioning struct {
	Project     string `json:"project"`
	Environment string `json:"environment"`
	DeployID    string `json:"deployId,omitempty"`
	Component   string `json:"component"`
	Status      string `json:"status"`
	Detail      string `json:"detail,omitempty"`
}

// wireScanGate is models.ScanGate (models/models.go:782-787).
type wireScanGate struct {
	Status    string `json:"status"`
	Criticals *int   `json:"criticals,omitempty"`
	Highs     *int   `json:"highs,omitempty"`
	Findings  *int   `json:"findings,omitempty"`
}

// wireSecurityStatus is models.SecurityStatus (models/security.go:205-242).
type wireSecurityStatus struct {
	Overall         string     `json:"overall"`
	Blocking        []wireNote `json:"blocking"`
	Incomplete      []wireNote `json:"incomplete,omitempty"`
	Scans           []wireScan `json:"scans"`
	LastUpdated     string     `json:"lastUpdated"`
	DeployID        string     `json:"deployId,omitempty"`
	DetailsRetained *bool      `json:"detailsRetained,omitempty"`
	DetailsSource   string     `json:"detailsSource,omitempty"`
}

// wireNote is models.BlockingItem (models/security.go:199-202).
type wireNote struct {
	Scan   string `json:"scan"`
	Reason string `json:"reason"`
}

// wireScan is models.ScanResult (models/security.go:117-197).
type wireScan struct {
	Scan           string         `json:"scan"`
	Status         string         `json:"status"`
	Mode           string         `json:"mode"`
	Counts         map[string]int `json:"counts"`
	Summary        string         `json:"summary,omitempty"`
	IsBlocking     bool           `json:"isBlocking"`
	LastScanTime   string         `json:"lastScanTime"`
	SkipReason     string         `json:"skipReason,omitempty"`
	ErrorReason    string         `json:"errorReason,omitempty"`
	FindingsSource string         `json:"findingsSource,omitempty"`
}

// seed is p's state, created on first mention. The caller holds s.mu.
func (s *Server) seed(p deploy.ProjectRef) *project {
	pr := s.projects[p]
	if pr == nil {
		pr = &project{info: deploy.Project{Ref: p}, branchEnv: map[string]string{}, postureWord: map[string]string{},
			scans: map[scanKey]wireSecurityStatus{}, rollbacks: map[string]deploy.Warmth{}}
		s.projects[p] = pr
	}
	return pr
}

// Project seeds a project GET /api/v1/projects lists, in call order; again replaces its details.
func (s *Server) Project(p deploy.Project) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pr := s.seed(p.Ref)
	pr.info = p
	if !pr.listed {
		pr.listed = true
		s.order = append(s.order, p.Ref)
	}
}

// Environments seeds the rows GET P/environments answers. Host is the row's subdomain, Replicas
// nil omits the replicas key (an unread namespace).
func (s *Server) Environments(p deploy.ProjectRef, envs []deploy.Environment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]wireEnvironment, 0, len(envs))
	for _, e := range envs {
		row := wireEnvironment{Name: e.Name, Branch: e.Branch, DisplayName: e.Branch, Primary: e.Primary,
			Subdomain: e.Host, Status: e.Status, LastDeploy: "awaiting first deploy"}
		if e.Replicas != nil {
			row.Replicas = &wireReplicas{Current: e.Replicas.Ready, Desired: e.Replicas.Desired}
		}
		rows = append(rows, row)
	}
	s.seed(p).envs = rows
}

// BranchEnvironment seeds data.env of GET P/branches/{branch}/access: Panacea's real answer,
// never one this server computes.
func (s *Server) BranchEnvironment(p deploy.ProjectRef, branch, env string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed(p).branchEnv[branch] = env
}

// PostureWord seeds data.accessMode of the same route, raw (task 04's Posture parses it).
func (s *Server) PostureWord(p deploy.ProjectRef, branch, word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed(p).postureWord[branch] = word
}

// Deployment seeds one row of GET P/deployments, kept newest first by CreatedAt (a later seed
// wins a tie); an existing ID is replaced. Its run answers GET P/pipelines/{run}. CompletedAt is
// written as Panacea writes it, the rounded duration since CreatedAt (clients/kubernetes.go:5683-5689).
func (s *Server) Deployment(p deploy.ProjectRef, d deploy.Deployment) {
	s.t.Helper()
	row := s.wireDeployment(p, d)
	s.mu.Lock()
	defer s.mu.Unlock()
	pr := s.seed(p)
	pr.deps = slices.DeleteFunc(pr.deps, func(x wireDeployment) bool { return x.ID == d.ID })
	i := sort.Search(len(pr.deps), func(i int) bool { return !pr.deps[i].CreatedAt.After(row.CreatedAt) })
	pr.deps = slices.Insert(pr.deps, i, row)
}

// NoDeployments makes p a project that has never been deployed.
func (s *Server) NoDeployments(p deploy.ProjectRef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed(p).deps = nil
}

// RollbackTarget seeds the warmth task 04's rollback route reports for deploymentID.
func (s *Server) RollbackTarget(p deploy.ProjectRef, deploymentID string, warmth deploy.Warmth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed(p).rollbacks[deploymentID] = warmth
}

// Scans seeds the report GET P/security-status answers for (env, deploymentID). "", "prod" and
// "production" are one key, production's (productionKey).
func (s *Server) Scans(p deploy.ProjectRef, env, deploymentID string, r deploy.ScanReport) {
	rep := wireSecurityStatus{Overall: r.Overall, Blocking: []wireNote{}, Scans: []wireScan{},
		LastUpdated: r.UpdatedAt, DeployID: r.DeploymentID, DetailsSource: r.DetailsSource}
	if r.DetailsRetained != nil {
		v := *r.DetailsRetained
		rep.DetailsRetained = &v
	}
	for _, n := range r.Blocking {
		rep.Blocking = append(rep.Blocking, wireNote(n))
	}
	for _, n := range r.Incomplete {
		rep.Incomplete = append(rep.Incomplete, wireNote(n))
	}
	for _, x := range r.Scans {
		rep.Scans = append(rep.Scans, wireScan{Scan: x.Name, Status: x.Status, Mode: x.Mode, Counts: cloneCounts(x.Counts),
			Summary: x.Summary, IsBlocking: x.Blocking, LastScanTime: x.LastScanAt, SkipReason: x.SkipReason,
			ErrorReason: x.ErrorReason, FindingsSource: x.FindingsSource})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed(p).scans[scanKey{productionKey(env), deploymentID}] = rep
}

func cloneCounts(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{} // ScanResult.Counts has no omitempty
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func intCopy(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// statusWords is the word Panacea writes in Deployment.status for each state: derivePipelineStatus
// (clients/pipeline.go:2073-2084) writes Pending for a run in progress, a run with no condition
// yet is Unknown (clients/kubernetes.go:5662), a backfilled row historical
// (clients/deployment_backfill.go:173). BuildUnknown has no word: the seed must carry BuildRaw.
var statusWords = map[deploy.BuildState]string{
	deploy.BuildSucceeded:  "Succeeded",
	deploy.BuildFailed:     "Failed",
	deploy.BuildRunning:    "Pending",
	deploy.BuildPending:    "Unknown",
	deploy.BuildHistorical: "historical",
}

// wireDeployment renders a seeded deployment. BuildRaw, when set, is the word sent verbatim.
func (s *Server) wireDeployment(p deploy.ProjectRef, d deploy.Deployment) wireDeployment {
	s.t.Helper()
	word := d.BuildRaw
	if word == "" {
		var ok bool
		if word, ok = statusWords[d.Build]; !ok {
			s.t.Errorf("panaceatest: deployment %q has Build %q and no BuildRaw; Panacea has no word for it", d.ID, d.Build)
		}
	}
	row := wireDeployment{ID: d.ID, ProjectName: string(p), Status: word, Image: d.Image, Version: d.Version,
		Commit: d.Commit, Branch: d.Branch, Environment: d.Environment, FailedStep: d.FailedStep,
		ErrorMessage: d.ErrorText, CreatedAt: d.CreatedAt, LiveReadyReplicas: intCopy(d.LiveReady),
		DeployedBy: d.DeployedBy, Trigger: d.Trigger, Backfilled: d.Reconstructed}
	if !d.CompletedAt.IsZero() {
		if d.CompletedAt.Before(d.CreatedAt) {
			s.t.Errorf("panaceatest: deployment %q completes before it is created", d.ID)
		}
		row.Duration = d.CompletedAt.Sub(d.CreatedAt).Round(time.Second).String()
	}
	for _, x := range d.Provisioning {
		row.Provisioning = append(row.Provisioning, wireProvisioning{Project: string(p), Environment: d.Environment,
			DeployID: d.ID, Component: x.Component, Status: x.Status, Detail: x.Detail})
	}
	if d.Gates != nil {
		row.ScanStatus = make(map[string]wireScanGate, len(d.Gates))
		for k, g := range d.Gates {
			row.ScanStatus[k] = wireScanGate{Status: g.Verdict, Criticals: intCopy(g.Criticals),
				Highs: intCopy(g.Highs), Findings: intCopy(g.Findings)}
		}
	}
	return row
}
