// Package deploytest holds the in-memory Deployer (Fake) and the conformance suite every
// Deployer passes: the Fake here, the Panacea connector in tasks 03 and 04, and a native
// implementation after the OD-13 lift.
package deploytest

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Fake is an in-memory Deployer. Every method is safe for concurrent use. Its clock is
// injectable so tests of the watcher and of Ship are deterministic.
//
// The Fake derives NOTHING a target decides: no environment name, no run label, no host, no
// env alias. What a test did not seed is refused loudly (KindRejected or KindNotFound) or is
// the zero answer, never a guess. A project no seeding method has named is KindNotFound.
//
// Every Reader, Actor, Governor and Identity call is recorded in Calls, and may be armed to
// fail (FailNext) or to run a hook first (BeforeNext). SameEnvironment is exact equality; it is
// the target's rule, not a call to the target, and it is not recorded.
type Fake struct {
	now func() time.Time

	mu       sync.Mutex
	account  deploy.Account
	order    []deploy.ProjectRef // AddProject order: Projects answers in it
	projects map[deploy.ProjectRef]*project
	calls    []string
	fails    map[string][]error
	hooks    map[string][]func(context.Context)
	runSeq   int
	tokenSeq int
}

type scanKey struct{ env, deploymentID string }

type streamKey struct {
	kind deploy.StreamKind
	id   string
}

type rollbackTarget struct {
	warmth deploy.Warmth
	cold   int
}

// project is one project's seeded and acted-on state. deps is kept newest first by CreatedAt.
type project struct {
	info        deploy.Project
	listed      bool
	envs        []deploy.Environment
	branchEnv   map[string]string
	deps        []deploy.Deployment
	runs        map[string]deploy.RunProgress
	scans       map[scanKey]deploy.ScanReport
	rollbacks   map[string]rollbackTarget
	postures    map[string]deploy.Posture
	notEnforced map[string]bool
	autoDeploy  map[string]deploy.AutoDeploy
	access      []deploy.AccessGrant
	prefixes    map[string]deploy.URLPrefix
	pods        map[string][]deploy.Pod
	events      map[string][]deploy.Event
	diagnoses   map[string]deploy.Diagnosis
	streams     map[streamKey][]deploy.StreamEvent
}

// methods is every name FailNext and BeforeNext accept: the interface's own method names.
var methods = map[string]bool{
	"Whoami": true, "Projects": true,
	"Environments": true, "EnvironmentFor": true, "Latest": true, "Deployments": true, "Run": true,
	"Scans": true, "Pods": true, "Events": true, "Diagnosis": true, "Stream": true,
	"Ship": true, "Cancel": true, "Rollback": true,
	"AutoDeploy": true, "SetAutoDeploy": true, "Posture": true, "SetPosture": true, "Access": true,
	"Grant": true, "Revoke": true, "SetURLPrefix": true, "ClearURLPrefix": true,
}

var _ deploy.Deployer = (*Fake)(nil)

// NewFake is an empty Fake on the clock now; nil means time.Now.
func NewFake(now func() time.Time) *Fake {
	if now == nil {
		now = time.Now
	}
	return &Fake{
		now:      now,
		projects: map[deploy.ProjectRef]*project{},
		fails:    map[string][]error{},
		hooks:    map[string][]func(context.Context){},
	}
}

// seed is p's state, created on first mention. The caller holds f.mu.
func (f *Fake) seed(p deploy.ProjectRef) *project {
	pr := f.projects[p]
	if pr == nil {
		pr = &project{
			info:        deploy.Project{Ref: p},
			branchEnv:   map[string]string{},
			runs:        map[string]deploy.RunProgress{},
			scans:       map[scanKey]deploy.ScanReport{},
			rollbacks:   map[string]rollbackTarget{},
			postures:    map[string]deploy.Posture{},
			notEnforced: map[string]bool{},
			autoDeploy:  map[string]deploy.AutoDeploy{},
			prefixes:    map[string]deploy.URLPrefix{},
			pods:        map[string][]deploy.Pod{},
			events:      map[string][]deploy.Event{},
			diagnoses:   map[string]deploy.Diagnosis{},
			streams:     map[streamKey][]deploy.StreamEvent{},
		}
		f.projects[p] = pr
	}
	return pr
}

// lookup is p's state, or KindNotFound when nothing seeded p. The caller holds f.mu.
func (f *Fake) lookup(method string, p deploy.ProjectRef) (*project, error) {
	if pr := f.projects[p]; pr != nil {
		return pr, nil
	}
	return nil, notFound(method, "project %q not found", p)
}

// enter records the call and takes its armed hook and failure in one step, runs the hook outside
// the lock, then answers the failure, then a done context (as a transport would: KindUnavailable,
// Status 0, the ctx error as Cause). See BeforeNext for why the failure is taken before the hook.
func (f *Fake) enter(ctx context.Context, method string, args ...string) error {
	f.mu.Lock()
	f.calls = append(f.calls, method+"("+strings.Join(args, ",")+")")
	hook := pop(f.hooks, method)
	fail := pop(f.fails, method)
	f.mu.Unlock()
	if hook != nil {
		hook(ctx)
	}
	if fail != nil {
		return fail
	}
	if err := ctx.Err(); err != nil {
		return &deploy.Error{Kind: deploy.KindUnavailable, Op: opName(method), Cause: err}
	}
	return nil
}

// pop removes and returns the first queued value for k, or the zero value.
func pop[T any](m map[string][]T, k string) T {
	var zero T
	q := m[k]
	if len(q) == 0 {
		return zero
	}
	v := q[0]
	if len(q) == 1 {
		delete(m, k)
	} else {
		m[k] = q[1:]
	}
	return v
}

func opName(method string) string { return strings.ToLower(method[:1]) + method[1:] }

// The Fake's own refusals carry the status class a target's would (404, 400), so the route
// code of tasks 05 and 13 reads them as refusals, never as "nothing was sent".
func notFound(method, format string, a ...any) error {
	return &deploy.Error{Kind: deploy.KindNotFound, Op: opName(method), Status: 404, Message: fmt.Sprintf(format, a...)}
}

func rejected(method, format string, a ...any) error {
	return &deploy.Error{Kind: deploy.KindRejected, Op: opName(method), Status: 400, Message: fmt.Sprintf(format, a...)}
}

func mustMethod(caller, method string) {
	if !methods[method] {
		panic(fmt.Sprintf("deploytest: %s(%q): not a Deployer method name", caller, method))
	}
}

// The clones below copy every pointer, map and slice, in and out, so neither the seeding test
// nor a caller can change the Fake's state through a value. A nil map or slice stays nil and an
// empty one stays empty: nil Gates ("no gate ran") must never read as an empty map ("clean").

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneMap[K comparable, V any](m map[K]V, each func(V) V) map[K]V {
	if m == nil {
		return nil
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		if each != nil {
			v = each(v)
		}
		out[k] = v
	}
	return out
}

func cloneSlice[T any](s []T, each func(T) T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	for i, v := range s {
		if each != nil {
			v = each(v)
		}
		out[i] = v
	}
	return out
}

func cloneDeployment(d deploy.Deployment) deploy.Deployment {
	d.LiveReady = clonePtr(d.LiveReady)
	d.Gates = cloneMap(d.Gates, func(g deploy.Gate) deploy.Gate {
		g.Criticals, g.Highs, g.Findings = clonePtr(g.Criticals), clonePtr(g.Highs), clonePtr(g.Findings)
		return g
	})
	d.Provisioning = cloneSlice(d.Provisioning, nil)
	return d
}

func cloneEnvironment(e deploy.Environment) deploy.Environment {
	e.Replicas = clonePtr(e.Replicas)
	return e
}

func cloneRun(r deploy.RunProgress) deploy.RunProgress {
	r.CompletedSteps = cloneSlice(r.CompletedSteps, nil)
	r.IgnoredFailures = cloneSlice(r.IgnoredFailures, nil)
	return r
}

func cloneScanReport(r deploy.ScanReport) deploy.ScanReport {
	r.Blocking = cloneSlice(r.Blocking, nil)
	r.Incomplete = cloneSlice(r.Incomplete, nil)
	r.Scans = cloneSlice(r.Scans, func(s deploy.Scan) deploy.Scan {
		s.Counts = cloneMap(s.Counts, nil)
		return s
	})
	r.DetailsRetained = clonePtr(r.DetailsRetained)
	return r
}

func cloneDiagnosis(d deploy.Diagnosis) deploy.Diagnosis {
	d.Findings = cloneSlice(d.Findings, func(x deploy.Finding) deploy.Finding {
		x.Evidence = cloneSlice(x.Evidence, nil)
		return x
	})
	return d
}

func cloneEvent(e deploy.StreamEvent) deploy.StreamEvent {
	e.Log = clonePtr(e.Log)
	if e.Readiness != nil {
		r := *e.Readiness
		r.Ready, r.Restarts, r.ExitCode = clonePtr(r.Ready), clonePtr(r.Restarts), clonePtr(r.ExitCode)
		e.Readiness = &r
	}
	return e
}

// Seeding — tests arrange state through these, never by reaching into the struct.

// SetAccount seeds Whoami's answer. Unseeded, Whoami answers the zero Account (not approved).
func (f *Fake) SetAccount(a deploy.Account) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.account = a
}

// AddProject seeds one project Projects lists, in call order. Adding a Ref again replaces its
// details and keeps its place.
func (f *Fake) AddProject(p deploy.Project) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := f.seed(p.Ref)
	pr.info = p
	if !pr.listed {
		pr.listed = true
		f.order = append(f.order, p.Ref)
	}
}

func (f *Fake) SetEnvironments(p deploy.ProjectRef, envs []deploy.Environment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).envs = cloneSlice(envs, cloneEnvironment)
}

// SetBranchEnvironment seeds EnvironmentFor's answer for one branch. The Fake derives NOTHING
// (no copy of any target's naming rule): an unseeded branch returns a KindRejected *Error whose
// Message names the branch, so a test that forgot to seed fails loudly instead of matching
// names by accident. Ship reads the same seed for the Deployment's Environment.
func (f *Fake) SetBranchEnvironment(p deploy.ProjectRef, branch, env string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).branchEnv[branch] = env
}

// AddDeployment seeds one deployment; newest-first order is kept by CreatedAt (a later add wins
// a tie). The row's run is registered for Run, Advance and Cancel. An existing ID is replaced,
// and its run follows the new row: the run's Build, BuildRaw, FailedStep, ErrorText and StartedAt
// (the row's CreatedAt) are the row's, so Run and the row never give two answers for one build
// and Cancel decides from the state the row shows (review t01 I-1). What a row does not carry
// (CurrentStep, CompletedSteps, IgnoredFailures) is kept. Rows are stored verbatim: the Fake does
// not strip LiveReady or Gates from a row that is not its environment's newest; the seeding test
// owns that.
func (f *Fake) AddDeployment(p deploy.ProjectRef, d deploy.Deployment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := f.seed(p)
	pr.insert(cloneDeployment(d))
	rp := pr.runs[d.ID]
	rp.Run, rp.Build, rp.BuildRaw, rp.StartedAt = d.ID, d.Build, d.BuildRaw, d.CreatedAt
	rp.FailedStep, rp.ErrorText = d.FailedStep, d.ErrorText
	pr.runs[d.ID] = rp
}

// ClearDeployments makes p a project that has never been deployed: no deployments, no runs.
func (f *Fake) ClearDeployments(p deploy.ProjectRef) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := f.seed(p)
	pr.deps = nil
	pr.runs = map[string]deploy.RunProgress{}
}

// insert places d newest first by CreatedAt, before any row it ties with, replacing its ID.
func (pr *project) insert(d deploy.Deployment) {
	pr.deps = slices.DeleteFunc(pr.deps, func(x deploy.Deployment) bool { return x.ID == d.ID })
	i := sort.Search(len(pr.deps), func(i int) bool { return !pr.deps[i].CreatedAt.After(d.CreatedAt) })
	pr.deps = slices.Insert(pr.deps, i, d)
}

// SetScans seeds the report Scans(p, env, deploymentID) returns. The key is the EXACT env
// string (the Fake knows no target's env aliases); an unseeded pair is KindNotFound, so a
// caller that drops or mangles env fails loudly (review C3). Panacea itself never answers an
// error here (a mangled env gets a 200 with NOT_RUN or empty rows, security_status.go:204-254):
// this KindNotFound is a test-bug signal, and task 07 step 5 says what the watcher does with it
// (Scans unset, not stale, retried; re-review b03 M-2). The conformance suite does not exercise
// an unseeded pair: panaceatest answers it 500 plus t.Errorf, also a test-bug signal (task 03).
func (f *Fake) SetScans(p deploy.ProjectRef, env, deploymentID string, r deploy.ScanReport) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).scans[scanKey{env, deploymentID}] = cloneScanReport(r)
}

// SetRollbackTarget seeds what Rollback of deploymentID reports about the image it returns
// to. The deployment itself is seeded with AddDeployment; unseeded, the warmth is unknown.
func (f *Fake) SetRollbackTarget(p deploy.ProjectRef, deploymentID string, w deploy.Warmth, coldStartSeconds int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).rollbacks[deploymentID] = rollbackTarget{warmth: w, cold: coldStartSeconds}
}

// SetPostureWord seeds the target's raw access-mode word for a branch. Posture parses it with
// deploy.ParsePostureMode, so an unknown word fails closed. Unseeded, the word is "".
func (f *Fake) SetPostureWord(p deploy.ProjectRef, branch, word string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := f.seed(p)
	ps := pr.postures[branch]
	ps.ModeRaw = word
	pr.postures[branch] = ps
}

// SetPostureEnforced decides whether the next SetPosture changes on branch are applied
// (default true). false seeds the target that SAVES a mode it could not apply.
func (f *Fake) SetPostureEnforced(p deploy.ProjectRef, branch string, enforced bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).notEnforced[branch] = !enforced
}

// SetAutoDeployState seeds AutoDeploy's answer for a.Branch. Unseeded, a branch reads the zero
// AutoDeploy: follows the project default, which is off.
func (f *Fake) SetAutoDeployState(p deploy.ProjectRef, a deploy.AutoDeploy) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.Explicit = clonePtr(a.Explicit)
	f.seed(p).autoDeploy[a.Branch] = a
}

// SetAccess seeds Access's rows, legacy group bindings included, in order.
func (f *Fake) SetAccess(p deploy.ProjectRef, grants []deploy.AccessGrant) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).access = cloneSlice(grants, nil)
}

// SetPods, SetEvents and SetDiagnosis seed one environment's explanation. Unseeded, Pods and
// Events answer none; Diagnosis answers KindNotFound.
func (f *Fake) SetPods(p deploy.ProjectRef, env string, pods []deploy.Pod) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).pods[env] = cloneSlice(pods, nil)
}

func (f *Fake) SetEvents(p deploy.ProjectRef, env string, events []deploy.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).events[env] = cloneSlice(events, nil)
}

func (f *Fake) SetDiagnosis(p deploy.ProjectRef, env string, d deploy.Diagnosis) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).diagnoses[env] = cloneDiagnosis(d)
}

// SetStream seeds the events one Stream(p, kind, id) yields, in order, before io.EOF. Every
// Stream call of that key replays them. Unseeded, Stream is KindNotFound. A Problem event ends
// the stream (deploy.StreamEvent: "the stream ends after it"), so nothing seeded after one is
// yielded, as no target sends it (review t01 M-1).
func (f *Fake) SetStream(p deploy.ProjectRef, kind deploy.StreamKind, id string, events []deploy.StreamEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seed(p).streams[streamKey{kind, id}] = cloneSlice(events, cloneEvent)
}

// Advance moves a run, and its Deployment's Build, to a new state. The Deployment follows every
// state, not only a terminal one, as the target's history shows a running build as running.
// succeeded and failed stamp CompletedAt with the clock when it is zero (the target records a
// duration), and keep one already recorded; historical stamps nothing. Advance of a run nothing
// seeded or shipped panics: it is a test bug.
func (f *Fake) Advance(p deploy.ProjectRef, run string, to deploy.BuildState) {
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := f.projects[p]
	if pr == nil {
		panic(fmt.Sprintf("deploytest: Advance(%q, %q): no such project", p, run))
	}
	if _, ok := pr.runs[run]; !ok {
		panic(fmt.Sprintf("deploytest: Advance(%q, %q): no such run", p, run))
	}
	pr.setBuild(run, to, now)
}

// setBuild moves run and its Deployment to state to. The caller holds f.mu.
func (pr *project) setBuild(run string, to deploy.BuildState, now time.Time) {
	rp := pr.runs[run]
	rp.Build, rp.BuildRaw = to, string(to)
	pr.runs[run] = rp
	for i := range pr.deps {
		if pr.deps[i].ID != run {
			continue
		}
		d := &pr.deps[i]
		d.Build, d.BuildRaw = to, string(to)
		if (to == deploy.BuildSucceeded || to == deploy.BuildFailed) && d.CompletedAt.IsZero() {
			d.CompletedAt = now
		}
	}
}

// FailNext makes the next call of the named method return err. Method names are the
// interface's: "Ship", "Rollback", "Environments", … Arming a method twice fails its next two
// calls, in order. A name that is not a Deployer method, or a nil err, panics, so a misspelt
// arm cannot pass a test that never exercised the failure.
func (f *Fake) FailNext(method string, err error) {
	mustMethod("FailNext", method)
	if err == nil {
		panic(fmt.Sprintf("deploytest: FailNext(%q, nil): arm an error", method))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails[method] = append(f.fails[method], err)
}

// BeforeNext runs fn at the start of the next call of the named method, once, with that call's
// context and WITHOUT the Fake's lock, so fn may call the Fake or the code under test (a second
// Ship, a retry, a cancel). The call records itself and takes its hook and its FailNext arm
// together, as it begins; then it runs fn, then answers that arm, then a done context, then does
// its work. So the arm a call answers is one made BEFORE it began: an arm fn makes fails the next
// call to begin, not this one, and a same-method call fn makes does not take this call's arm
// (review t01 M-5). Tasks 09 and 10 act "inside the Fake's Ship hook" and "Rollback hook" through
// it.
func (f *Fake) BeforeNext(method string, fn func(ctx context.Context)) {
	mustMethod("BeforeNext", method)
	if fn == nil {
		panic(fmt.Sprintf("deploytest: BeforeNext(%q, nil): arm a hook", method))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks[method] = append(f.hooks[method], fn)
}

// Calls is every call made, in order, as "Method(project,arg)" — for asserting what was NOT
// called. Arguments are comma-joined: Scans(engagement-portal,feat-x,d-feat). A posture
// password is never recorded; SetPosture records its mode only.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Identity.

func (f *Fake) Whoami(ctx context.Context) (deploy.Account, error) {
	if err := f.enter(ctx, "Whoami"); err != nil {
		return deploy.Account{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.account, nil
}

func (f *Fake) Projects(ctx context.Context) ([]deploy.Project, error) {
	if err := f.enter(ctx, "Projects"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]deploy.Project, 0, len(f.order))
	for _, ref := range f.order {
		out = append(out, f.projects[ref].info)
	}
	return out, nil
}

// Reader.

func (f *Fake) Environments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
	if err := f.enter(ctx, "Environments", string(p)); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Environments", p)
	if err != nil {
		return nil, err
	}
	return cloneSlice(pr.envs, cloneEnvironment), nil
}

func (f *Fake) EnvironmentFor(ctx context.Context, p deploy.ProjectRef, branch string) (string, error) {
	if err := f.enter(ctx, "EnvironmentFor", string(p), branch); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("EnvironmentFor", p)
	if err != nil {
		return "", err
	}
	return pr.environmentFor("EnvironmentFor", branch)
}

// environmentFor is the seeded answer, or the loud refusal. The caller holds f.mu.
func (pr *project) environmentFor(method, branch string) (string, error) {
	env, ok := pr.branchEnv[branch]
	if !ok {
		return "", rejected(method, "no environment is seeded for branch %q (deploytest: SetBranchEnvironment)", branch)
	}
	return env, nil
}

func (f *Fake) Latest(ctx context.Context, p deploy.ProjectRef) (deploy.Deployment, bool, error) {
	if err := f.enter(ctx, "Latest", string(p)); err != nil {
		return deploy.Deployment{}, false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Latest", p)
	if err != nil {
		return deploy.Deployment{}, false, err
	}
	if len(pr.deps) == 0 {
		return deploy.Deployment{}, false, nil
	}
	return cloneDeployment(pr.deps[0]), true, nil
}

func (f *Fake) Deployments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Deployment, error) {
	if err := f.enter(ctx, "Deployments", string(p)); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Deployments", p)
	if err != nil {
		return nil, err
	}
	return cloneSlice(pr.deps, cloneDeployment), nil
}

func (f *Fake) Run(ctx context.Context, p deploy.ProjectRef, run string) (deploy.RunProgress, error) {
	if err := f.enter(ctx, "Run", string(p), run); err != nil {
		return deploy.RunProgress{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Run", p)
	if err != nil {
		return deploy.RunProgress{}, err
	}
	rp, ok := pr.runs[run]
	if !ok {
		return deploy.RunProgress{}, notFound("Run", "pipeline run %q not found in project %q", run, p)
	}
	return cloneRun(rp), nil
}

func (f *Fake) Scans(ctx context.Context, p deploy.ProjectRef, env, deploymentID string) (deploy.ScanReport, error) {
	if err := f.enter(ctx, "Scans", string(p), env, deploymentID); err != nil {
		return deploy.ScanReport{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Scans", p)
	if err != nil {
		return deploy.ScanReport{}, err
	}
	r, ok := pr.scans[scanKey{env, deploymentID}]
	if !ok {
		return deploy.ScanReport{}, notFound("Scans",
			"no scan report is seeded for deployment %q in environment %q (deploytest: SetScans)", deploymentID, env)
	}
	return cloneScanReport(r), nil
}

func (f *Fake) Pods(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Pod, error) {
	if err := f.enter(ctx, "Pods", string(p), env); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Pods", p)
	if err != nil {
		return nil, err
	}
	return cloneSlice(pr.pods[env], nil), nil
}

func (f *Fake) Events(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Event, error) {
	if err := f.enter(ctx, "Events", string(p), env); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Events", p)
	if err != nil {
		return nil, err
	}
	return cloneSlice(pr.events[env], nil), nil
}

func (f *Fake) Diagnosis(ctx context.Context, p deploy.ProjectRef, env string) (deploy.Diagnosis, error) {
	if err := f.enter(ctx, "Diagnosis", string(p), env); err != nil {
		return deploy.Diagnosis{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Diagnosis", p)
	if err != nil {
		return deploy.Diagnosis{}, err
	}
	d, ok := pr.diagnoses[env]
	if !ok {
		return deploy.Diagnosis{}, notFound("Diagnosis", "no diagnosis is seeded for environment %q (deploytest: SetDiagnosis)", env)
	}
	return cloneDiagnosis(d), nil
}

func (f *Fake) Stream(ctx context.Context, p deploy.ProjectRef, kind deploy.StreamKind, id string) (deploy.Stream, error) {
	if err := f.enter(ctx, "Stream", string(p), string(kind), id); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Stream", p)
	if err != nil {
		return nil, err
	}
	evs, ok := pr.streams[streamKey{kind, id}]
	if !ok {
		return nil, notFound("Stream", "no %s stream is seeded for %q (deploytest: SetStream)", kind, id)
	}
	return &fakeStream{ctx: ctx, events: cloneSlice(evs, cloneEvent)}, nil
}

// SameEnvironment is exact equality: the Fake knows no target's aliases (task 03 gives the
// connector Panacea's prod == production).
func (f *Fake) SameEnvironment(a, b string) bool { return a == b }

// fakeStream yields its events in order, then io.EOF. A Problem is the last event it yields, and
// Close ends it early. A done context ends it with ctx.Err().
type fakeStream struct {
	ctx    context.Context
	mu     sync.Mutex
	events []deploy.StreamEvent
	closed bool
}

func (s *fakeStream) Next() (deploy.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return deploy.StreamEvent{}, err
	}
	if s.closed || len(s.events) == 0 {
		return deploy.StreamEvent{}, io.EOF
	}
	ev := s.events[0]
	s.events = s.events[1:]
	if ev.Problem != "" {
		s.events = nil // the stream ends after a Problem
	}
	return ev, nil
}

func (s *fakeStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// Actor.

// Ship names the run "run-" + a counter and adds a BuildPending Deployment whose ID is the run,
// in the environment SetBranchEnvironment seeded for the branch (unseeded: KindRejected, as
// EnvironmentFor). The counter skips a name a seeded row already holds, so a Ship never
// replaces seeded history, as a target never reuses a run name. It refuses what phase 7 never
// sends: an empty branch, or a commit that is not 40 lowercase hex. It records no run label
// (Deployment.Branch ""): that is a target rule.
func (f *Fake) Ship(ctx context.Context, p deploy.ProjectRef, req deploy.ShipRequest) (deploy.ShipResult, error) {
	if err := f.enter(ctx, "Ship", string(p), req.Branch, req.Commit); err != nil {
		return deploy.ShipResult{}, err
	}
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Ship", p)
	if err != nil {
		return deploy.ShipResult{}, err
	}
	if req.Branch == "" {
		return deploy.ShipResult{}, rejected("Ship", "a Ship names a branch")
	}
	if !deploy.ValidCommit(req.Commit) {
		return deploy.ShipResult{}, rejected("Ship", "commit %q is not 40 lowercase hex", req.Commit)
	}
	env, err := pr.environmentFor("Ship", req.Branch)
	if err != nil {
		return deploy.ShipResult{}, err
	}
	run := f.nextRun(pr)
	pr.insert(deploy.Deployment{ID: run, Environment: env, Commit: req.Commit,
		Build: deploy.BuildPending, BuildRaw: string(deploy.BuildPending), CreatedAt: now})
	pr.runs[run] = deploy.RunProgress{Run: run, Build: deploy.BuildPending,
		BuildRaw: string(deploy.BuildPending), StartedAt: now}
	return deploy.ShipResult{Run: run}, nil
}

// nextRun is "run-" + the next counter value that names no run of pr. Every row registers its
// run (AddDeployment, Ship), so a name no run holds is a name no row holds. The caller holds f.mu.
func (f *Fake) nextRun(pr *project) string {
	for {
		f.runSeq++
		run := fmt.Sprintf("run-%d", f.runSeq)
		if _, taken := pr.runs[run]; !taken {
			return run
		}
	}
}

// Cancel of an unfinished run moves it, and its Deployment, to BuildFailed (the target does not
// tell cancelled from failed). A finished run answers AlreadyFinished, not an error.
func (f *Fake) Cancel(ctx context.Context, p deploy.ProjectRef, run string) (deploy.CancelResult, error) {
	if err := f.enter(ctx, "Cancel", string(p), run); err != nil {
		return deploy.CancelResult{}, err
	}
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Cancel", p)
	if err != nil {
		return deploy.CancelResult{}, err
	}
	rp, ok := pr.runs[run]
	if !ok {
		return deploy.CancelResult{}, notFound("Cancel", "pipeline run %q not found in project %q", run, p)
	}
	if rp.Build.Terminal() {
		return deploy.CancelResult{Outcome: deploy.AlreadyFinished, Message: fmt.Sprintf("Run %s has already finished.", run)}, nil
	}
	pr.setBuild(run, deploy.BuildFailed, now)
	return deploy.CancelResult{Outcome: deploy.Cancelled, Message: fmt.Sprintf("Run %s cancelled.", run)}, nil
}

// Rollback of a seeded deployment answers it as SourceDeploymentID and ADDS NOTHING: no run, no
// deployment (7P-4). Warmth is SetRollbackTarget's, else unknown.
func (f *Fake) Rollback(ctx context.Context, p deploy.ProjectRef, deploymentID string) (deploy.RollbackResult, error) {
	if err := f.enter(ctx, "Rollback", string(p), deploymentID); err != nil {
		return deploy.RollbackResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Rollback", p)
	if err != nil {
		return deploy.RollbackResult{}, err
	}
	i := slices.IndexFunc(pr.deps, func(d deploy.Deployment) bool { return d.ID == deploymentID })
	if i < 0 {
		return deploy.RollbackResult{}, notFound("Rollback", "deployment %q not found in project %q", deploymentID, p)
	}
	rt := pr.rollbacks[deploymentID]
	return deploy.RollbackResult{SourceDeploymentID: deploymentID, Image: pr.deps[i].Image,
		Warmth: rt.warmth, ColdStartSeconds: rt.cold}, nil
}

// Governor.

func (f *Fake) AutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.AutoDeploy, error) {
	if err := f.enter(ctx, "AutoDeploy", string(p), branch); err != nil {
		return deploy.AutoDeploy{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("AutoDeploy", p)
	if err != nil {
		return deploy.AutoDeploy{}, err
	}
	return pr.autoDeployOf(branch), nil
}

// autoDeployOf is the branch's seeded or set state, or the zero answer. The caller holds f.mu.
func (pr *project) autoDeployOf(branch string) deploy.AutoDeploy {
	a, ok := pr.autoDeploy[branch]
	if !ok {
		a = deploy.AutoDeploy{Branch: branch}
	}
	a.Explicit = clonePtr(a.Explicit)
	return a
}

// SetAutoDeploy makes the branch's choice explicit; Effective follows it.
func (f *Fake) SetAutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string, enabled bool) (deploy.AutoDeploy, error) {
	if err := f.enter(ctx, "SetAutoDeploy", string(p), branch, fmt.Sprint(enabled)); err != nil {
		return deploy.AutoDeploy{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("SetAutoDeploy", p)
	if err != nil {
		return deploy.AutoDeploy{}, err
	}
	a := pr.autoDeployOf(branch)
	a.Explicit, a.Effective = &enabled, enabled
	pr.autoDeploy[branch] = a
	return pr.autoDeployOf(branch), nil
}

func (f *Fake) Posture(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.Posture, error) {
	if err := f.enter(ctx, "Posture", string(p), branch); err != nil {
		return deploy.Posture{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Posture", p)
	if err != nil {
		return deploy.Posture{}, err
	}
	return pr.postureOf(branch), nil
}

// postureOf reads the stored raw word through deploy.ParsePostureMode, so an unknown word
// fails closed here exactly as it must in every implementation. The caller holds f.mu.
func (pr *project) postureOf(branch string) deploy.Posture {
	ps := pr.postures[branch]
	ps.Branch, ps.Environment = branch, pr.branchEnv[branch]
	ps.Mode, ps.ModeKnown = deploy.ParsePostureMode(ps.ModeRaw)
	ps.IPAllowlist = cloneSlice(ps.IPAllowlist, nil)
	return ps
}

// SetPosture refuses a change the domain says is never sent: an unknown mode, a password with
// any mode but password (or password without one), a token rotation with any mode but
// bypass-token. The password is never recorded, stored or echoed: only PasswordSet moves.
// SetPostureEnforced(false) saves the mode and answers Enforced false.
func (f *Fake) SetPosture(ctx context.Context, p deploy.ProjectRef, branch string, c deploy.PostureChange) (deploy.PostureResult, error) {
	if err := f.enter(ctx, "SetPosture", string(p), branch, string(c.Mode)); err != nil {
		return deploy.PostureResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("SetPosture", p)
	if err != nil {
		return deploy.PostureResult{}, err
	}
	if _, known := deploy.ParsePostureMode(string(c.Mode)); !known {
		return deploy.PostureResult{}, rejected("SetPosture", "unknown access mode %q", c.Mode)
	}
	if (c.Password != "") != (c.Mode == deploy.PosturePassword) {
		return deploy.PostureResult{}, rejected("SetPosture", "a password is sent with, and only with, the password mode")
	}
	if c.RotateBypassToken && c.Mode != deploy.PostureBypassToken {
		return deploy.PostureResult{}, rejected("SetPosture", "a bypass token is rotated only in the bypass-token mode")
	}
	ps := pr.postures[branch]
	ps.ModeRaw = string(c.Mode)
	if c.Password != "" {
		ps.PasswordSet = true
	}
	var tokens []string
	if c.RotateBypassToken {
		f.tokenSeq++
		tokens = []string{fmt.Sprintf("bypass-%d", f.tokenSeq)}
		ps.BypassTokenCount++
	}
	pr.postures[branch] = ps
	res := deploy.PostureResult{Posture: pr.postureOf(branch), Enforced: !pr.notEnforced[branch], BypassTokens: tokens}
	if res.Enforced {
		res.Message = "The new access mode is in force."
	} else {
		res.Message = "The new access mode was saved but not applied: the old mode still stands."
	}
	return res, nil
}

func (f *Fake) Access(ctx context.Context, p deploy.ProjectRef) ([]deploy.AccessGrant, error) {
	if err := f.enter(ctx, "Access", string(p)); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Access", p)
	if err != nil {
		return nil, err
	}
	return cloneSlice(pr.access, nil), nil
}

// Grant is single-role, as Panacea's is: a grant to a person who holds a role REPLACES it, in
// place. A person's row has Group == Grantee. RoleUnknown, or a blank grantee, is refused.
func (f *Fake) Grant(ctx context.Context, p deploy.ProjectRef, g deploy.GrantRequest) error {
	if err := f.enter(ctx, "Grant", string(p), g.Grantee, string(g.Role)); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Grant", p)
	if err != nil {
		return err
	}
	if strings.TrimSpace(g.Grantee) == "" {
		return rejected("Grant", "a grant names one person")
	}
	switch g.Role {
	case deploy.RoleViewer, deploy.RoleDeveloper, deploy.RoleOwner:
	default:
		return rejected("Grant", "role %q cannot be granted", g.Role)
	}
	row := deploy.AccessGrant{Grantee: g.Grantee, Group: g.Grantee, Role: g.Role, RoleRaw: string(g.Role)}
	if i := slices.IndexFunc(pr.access, func(a deploy.AccessGrant) bool { return !a.Legacy && a.Group == g.Grantee }); i >= 0 {
		pr.access[i] = row
	} else {
		pr.access = append(pr.access, row)
	}
	return nil
}

// Revoke removes every row whose Group is group. A group that names no row — a legacy row
// revoked by its Grantee, say — is a silent no-op, as on Panacea (review M30).
func (f *Fake) Revoke(ctx context.Context, p deploy.ProjectRef, group string) error {
	if err := f.enter(ctx, "Revoke", string(p), group); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("Revoke", p)
	if err != nil {
		return err
	}
	pr.access = slices.DeleteFunc(pr.access, func(a deploy.AccessGrant) bool { return a.Group == group })
	return nil
}

// SetURLPrefix stores the prefix. Host is "": a host is the target's to compute.
func (f *Fake) SetURLPrefix(ctx context.Context, p deploy.ProjectRef, branch, prefix string) (deploy.URLPrefix, error) {
	if err := f.enter(ctx, "SetURLPrefix", string(p), branch, prefix); err != nil {
		return deploy.URLPrefix{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("SetURLPrefix", p)
	if err != nil {
		return deploy.URLPrefix{}, err
	}
	if prefix == "" {
		return deploy.URLPrefix{}, rejected("SetURLPrefix", "an empty prefix is cleared, not set")
	}
	u := deploy.URLPrefix{Branch: branch, Prefix: prefix}
	pr.prefixes[branch] = u
	return u, nil
}

func (f *Fake) ClearURLPrefix(ctx context.Context, p deploy.ProjectRef, branch string) error {
	if err := f.enter(ctx, "ClearURLPrefix", string(p), branch); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, err := f.lookup("ClearURLPrefix", p)
	if err != nil {
		return err
	}
	delete(pr.prefixes, branch)
	return nil
}
