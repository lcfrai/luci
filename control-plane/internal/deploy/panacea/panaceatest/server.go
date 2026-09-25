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
// It holds NO naming rule: an environment's name, a branch's environment and a seeded run's name
// are all seeded (a Ship's run is named by this server, in no Panacea form). The Panacea rules it
// copies are each the rule a mutant hides behind: how security-status resolves env (Server.Scans);
// and, for task 04's writes, the handlers' own refusals and state changes, each cited where it is
// written (the cancel of a run neither Pending nor Running, the share's group-name role, the
// access mode's mint).
package panaceatest

import (
	"encoding/json"
	"fmt"
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
	seq      int // names a Ship's run and a minted token
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
	// Task 04's writes.
	extras map[string]postureExtras // per branch: what GET B/access reports beside the mode
	groups map[string][]string      // Keycloak group name → its members, in join order
}

// postureExtras is what SetBranchAccessMode persists beside the mode, as GET B/access reads it
// back (handlers/project_security.go:649-666): a password gate, and how many bypass tokens.
type postureExtras struct {
	passwordSet  bool
	bypassTokens int
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
	// Task 04: the write routes, and the access list they change.
	mux.HandleFunc("POST "+P+"/deploy", s.inProject(s.ship))
	mux.HandleFunc("POST "+P+"/pipelines/{run}/cancel", s.inProject(s.cancel))
	mux.HandleFunc("POST "+P+"/deployments/{id}/rollback", s.inProject(s.rollback))
	mux.HandleFunc("PUT "+P+"/branches/{branch}/access", s.inProject(s.setBranchAccess))
	mux.HandleFunc("GET "+P+"/access", s.inProject(s.listAccess))
	mux.HandleFunc("POST "+P+"/access", s.inProject(s.addAccess))
	mux.HandleFunc("DELETE "+P+"/access/{group}", s.inProject(s.removeAccess))
	// No seeder names auto-deploy or a URL prefix, and each answer needs a Panacea rule this
	// server does not hold (a namespace's existence, a host): the recorded bodies test them.
	for _, route := range []string{"GET " + P + "/branches/{branch}/auto-deploy", "PUT " + P + "/branches/{branch}/auto-deploy",
		"PUT " + P + "/branches/{branch}/url-prefix", "DELETE " + P + "/branches/{branch}/url-prefix"} {
		mux.HandleFunc(route, s.inProject(s.notModelled))
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
	x := pr.extras[branch] // task 04's PUT B/access writes them
	body := map[string]any{"branch": branch, "ipWhitelist": []string{}, "passwordSet": x.passwordSet, "bypassTokenCount": x.bypassTokens}
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
	RollbackWarmth    string                  `json:"rollbackWarmth,omitempty"`   // task 04: the rollback answer (models.go:704)
	ColdStartSeconds  int                     `json:"coldStartSeconds,omitempty"` // (models.go:708)
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
			scans: map[scanKey]wireSecurityStatus{}, rollbacks: map[string]deploy.Warmth{},
			extras: map[string]postureExtras{}, groups: map[string][]string{}}
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
	s.seed(p).insert(row)
}

// insert puts row in the history, newest first by CreatedAt (a later row wins a tie), replacing
// a row with its ID. The caller holds s.mu.
func (pr *project) insert(row wireDeployment) {
	pr.deps = slices.DeleteFunc(pr.deps, func(x wireDeployment) bool { return x.ID == row.ID })
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

// Task 04: the write routes. Each handler runs with s.mu held (inProject).

// apiResponse is models.APIResponse with its omitempty tags (models/models.go:2147-2151).
type apiResponse struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// ship answers POST P/deploy as TriggerDeploy does (handlers/projects.go:2942-3005): the body
// decoded leniently (:2967-2968), a run named, and its row in the history at once as a run with
// no condition yet ("Unknown", clients/kubernetes.go:5662) in the branch's seeded environment.
// The row carries no branch label: that is Panacea's sanitizer, not this server's. An unseeded
// branch is a test bug. The run's name is this server's own ("<project>-run-<n>").
func (s *Server) ship(w http.ResponseWriter, r *http.Request, pr *project) {
	var opts struct {
		Branch string `json:"branch"`
		Commit string `json:"commit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&opts)
	env, ok := pr.branchEnv[opts.Branch]
	if !ok {
		s.testBug(w, "Ship of branch %q, which has no BranchEnvironment seed", opts.Branch)
		return
	}
	name := r.PathValue("name")
	var run string
	for taken := true; taken; {
		s.seq++
		run = fmt.Sprintf("%s-run-%d", name, s.seq)
		taken = slices.ContainsFunc(pr.deps, func(d wireDeployment) bool { return d.ID == run })
	}
	pr.insert(wireDeployment{ID: run, ProjectName: name, Status: "Unknown", Commit: opts.Commit, Environment: env,
		CreatedAt: time.Now().UTC().Truncate(time.Second)})
	writeJSON(w, http.StatusAccepted, apiResponse{Message: "deployment started",
		Data: map[string]string{"project": name, "status": "running", "pipelineRun": run}})
}

// cancel answers POST P/pipelines/{run}/cancel as CancelDeploy does (handlers/projects.go:3644-3687):
// an unknown run is the 404 (:3660); only a Pending or Running run is cancelled (:3669), and then
// reads Failed (the domain's "failed or cancelled"); any other, Unknown included, is
// already_completed with "Pipeline run is already <status>" (:3670-3673).
func (s *Server) cancel(w http.ResponseWriter, r *http.Request, pr *project) {
	run := r.PathValue("run")
	i := slices.IndexFunc(pr.deps, func(d wireDeployment) bool { return d.ID == run })
	if i < 0 {
		writeError(w, http.StatusNotFound, "pipeline run not found: pipelineruns.tekton.dev \""+run+"\" not found")
		return
	}
	if st := pr.deps[i].Status; st != "Pending" && st != "Running" {
		writeJSON(w, http.StatusOK, apiResponse{Data: map[string]string{"status": "already_completed",
			"message": "Pipeline run is already " + st}})
		return
	}
	pr.deps[i].Status = "Failed"
	writeJSON(w, http.StatusOK, apiResponse{Data: map[string]string{"status": "cancelled",
		"message": "Deployment cancelled successfully"}})
}

// rollback answers POST P/deployments/{id}/rollback as Rollback does (handlers/projects.go:1547-1639):
// an unknown id is the 404 (:1570), a row with no image the 400 (:1575), and otherwise the
// Deployment rolled back TO, id unchanged (clients/kubernetes.go:5982, :6030), enriched with the
// target's image, commit, branch and environment (:1619-1623), warm with no seconds or cold with
// rollbackColdStartSeconds (clients/immutable_deploy.go:77). It adds NO row (7P-4). A target with
// no RollbackTarget seed is a test bug: Panacea always answers warm or cold.
func (s *Server) rollback(w http.ResponseWriter, r *http.Request, pr *project) {
	id := r.PathValue("id")
	i := slices.IndexFunc(pr.deps, func(d wireDeployment) bool { return d.ID == id })
	if i < 0 {
		writeError(w, http.StatusNotFound, "deployment not found: "+id)
		return
	}
	d := pr.deps[i]
	if d.Image == "" {
		writeError(w, http.StatusBadRequest, "deployment has no image tag — cannot rollback")
		return
	}
	warmth := pr.rollbacks[id]
	if warmth != deploy.WarmthWarm && warmth != deploy.WarmthCold {
		s.testBug(w, "rollback of %q, which has no warm or cold RollbackTarget seed", id)
		return
	}
	out := wireDeployment{ID: id, ProjectName: r.PathValue("name"), Status: "rolling-back", Image: d.Image, Commit: d.Commit,
		Branch: d.Branch, Environment: d.Environment, CreatedAt: time.Now().UTC(), RollbackWarmth: string(warmth)}
	if warmth == deploy.WarmthCold {
		out.ColdStartSeconds = 20
	}
	writeJSON(w, http.StatusOK, apiResponse{Message: "rollback initiated to " + id, Data: out})
}

// accessModes are ValidAccessMode's five (handlers/project_security.go:718-722), spelled here
// rather than read from package deploy, so the fake does not agree with the connector by sharing.
var accessModes = []string{"panacea-sso", "kpmg-sso", "password", "bypass-token", "none"}

// setBranchAccess answers PUT P/branches/{branch}/access as SetBranchAccessMode does
// (handlers/project_security.go:701-845), always enforced (no seeder names a failed reconcile):
// a mode outside the five is the 400 (:718-722); password needs one (:766-770) and sets the gate;
// bypass-token with no supplied token, or a rotation, mints one, returned ONCE (:779-803,
// :830-833), and the stored count is the tokens kept. The mode is stored as PostureWord's.
func (s *Server) setBranchAccess(w http.ResponseWriter, r *http.Request, pr *project) {
	branch := strings.TrimSpace(r.PathValue("branch"))
	var req struct {
		AccessMode         string   `json:"accessMode"`
		Password           string   `json:"password"`
		BypassTokens       []string `json:"bypassTokens"`
		RotateBypassTokens bool     `json:"rotateBypassTokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.testBug(w, "PUT %s: the body is not JSON: %v", r.URL.Path, err)
		return
	}
	mode := strings.TrimSpace(req.AccessMode)
	if !slices.Contains(accessModes, mode) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid accessMode %q: must be one of panacea-sso, kpmg-sso, password, bypass-token, none", mode))
		return
	}
	x := pr.extras[branch]
	var minted []string
	switch mode {
	case "password":
		if strings.TrimSpace(req.Password) == "" {
			writeError(w, http.StatusBadRequest, "password is required for accessMode=password")
			return
		}
		x.passwordSet = true
	case "bypass-token":
		kept := slices.DeleteFunc(slices.Clone(req.BypassTokens), func(t string) bool { return strings.TrimSpace(t) == "" })
		if req.RotateBypassTokens || len(kept) == 0 {
			s.seq++
			kept = []string{fmt.Sprintf("pnb_%032x", s.seq)}
			minted = kept
		}
		x.bypassTokens = len(kept)
	}
	pr.postureWord[branch], pr.extras[branch] = mode, x
	resp := map[string]any{"branch": branch, "accessMode": mode, "enforced": true}
	if env, ok := pr.branchEnv[branch]; ok {
		resp["env"] = env
	}
	if len(minted) > 0 {
		resp["bypassTokens"] = minted
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: resp,
		Message: fmt.Sprintf("access mode set to %s for %s@%s", mode, r.PathValue("name"), branch)})
}

// wireBinding is models.AccessBinding (models/models.go:2004-2015).
type wireBinding struct {
	Group   string `json:"group"`
	Role    string `json:"role"`
	Grantee string `json:"grantee,omitempty"`
}

// roleFromGroupName is roleFromProjectGroupName (clients/keycloak.go:503-512), copied because it
// is the rule the "developer" mutant hides behind: -owner, -editor, and anything else viewer.
func roleFromGroupName(name string) string {
	switch {
	case strings.HasSuffix(name, "-owner"):
		return "owner"
	case strings.HasSuffix(name, "-editor"):
		return "editor"
	}
	return "viewer"
}

// listAccess answers GET P/access as ListProjectAccess does (clients/keycloak.go:126-162): one
// row per member, group == grantee == the member (:156-159), role from the group's name, groups
// in name order. A member-less group is skipped: no seeder names a legacy attribute (:143-153).
func (s *Server) listAccess(w http.ResponseWriter, _ *http.Request, pr *project) {
	names := make([]string, 0, len(pr.groups))
	for g := range pr.groups {
		names = append(names, g)
	}
	slices.Sort(names)
	var out []wireBinding
	for _, g := range names {
		for _, m := range pr.groups[g] {
			out = append(out, wireBinding{Group: m, Role: roleFromGroupName(g), Grantee: m})
		}
	}
	writeJSON(w, http.StatusOK, list(out))
}

// addAccess answers POST P/access as AccessHandler.Add and AddProjectAccess do: "" is the 400
// (handlers/access.go:50-52); whitespace the 500 of clients/keycloak.go:182 through access.go:57;
// otherwise the grantee leaves the project's other groups (:198-214) and joins
// "project-<name>-"+role, the role unchecked (:192, :216-218). Every grantee is a user: no seeder
// names Keycloak's directory, so "no Keycloak user found" (:189) is the recorded bodies' to test.
func (s *Server) addAccess(w http.ResponseWriter, r *http.Request, pr *project) {
	var req struct {
		Group string `json:"group"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.testBug(w, "POST %s: the body is not JSON: %v", r.URL.Path, err)
		return
	}
	if req.Group == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "group and role are required")
		return
	}
	grantee := strings.TrimSpace(req.Group)
	if grantee == "" {
		writeError(w, http.StatusInternalServerError, "failed to add access binding: a grantee (username or email) is required")
		return
	}
	group := "project-" + r.PathValue("name") + "-" + req.Role
	for g, members := range pr.groups {
		if g != group {
			pr.groups[g] = slices.DeleteFunc(members, func(m string) bool { return m == grantee })
		}
	}
	if !slices.Contains(pr.groups[group], grantee) {
		pr.groups[group] = append(pr.groups[group], grantee)
	}
	writeJSON(w, http.StatusCreated, apiResponse{Message: "access binding added"})
}

// removeAccess answers DELETE P/access/{group} as RemoveProjectAccess does
// (clients/keycloak.go:229-268): a member is removed from every group (:245-260); otherwise a
// group of that name is deleted (:262-267); otherwise nothing matched, and it is still 200 (:268).
func (s *Server) removeAccess(w http.ResponseWriter, r *http.Request, pr *project) {
	who := strings.TrimSpace(r.PathValue("group"))
	member := false
	for g, members := range pr.groups {
		if slices.Contains(members, who) {
			member = true
			pr.groups[g] = slices.DeleteFunc(members, func(m string) bool { return m == who })
		}
	}
	if !member {
		delete(pr.groups, who)
	}
	writeJSON(w, http.StatusOK, apiResponse{Message: "access binding removed"})
}
