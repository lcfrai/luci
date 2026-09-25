package deploytest

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Seeder arranges the same state on any implementation under test: the Fake directly, or the
// Panacea connector through the recorded-body server of tasks 03 and 04.
type Seeder interface {
	Project(p deploy.Project)
	Environments(p deploy.ProjectRef, envs []deploy.Environment)
	// BranchEnvironment seeds the target's own environment name for a git branch
	// (EnvironmentFor). Tests seed Panacea's real answers, never a rule that computes them.
	BranchEnvironment(p deploy.ProjectRef, branch, env string)
	Deployment(p deploy.ProjectRef, d deploy.Deployment)
	NoDeployments(p deploy.ProjectRef)
	RollbackTarget(p deploy.ProjectRef, deploymentID string, warmth deploy.Warmth)
	// Scans seeds one deployment's report as the target answers it for env. "prod" is
	// production (Panacea's word, t01 Environment).
	Scans(p deploy.ProjectRef, env, deploymentID string, r deploy.ScanReport)
	// PostureWord seeds the target's raw access-mode word for a branch, so the fail-closed
	// parse is tested through every implementation, not only the helper.
	PostureWord(p deploy.ProjectRef, branch, word string)
}

// Harness builds a fresh implementation and its seeder for one subtest.
type Harness func(t *testing.T) (deploy.Deployer, Seeder)

const (
	confProject = deploy.ProjectRef("conformance-portal")
	confCommit  = "4f0c2a9d1b7e3c5a8f6d0b2e4c6a8f1d3b5e7c9a"
)

// confBase is whole seconds, so a target that rounds a duration to a second reads it exactly.
var confBase = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

// row is one conformance assertion. Each runs on a fresh implementation whose project
// confProject is already seeded.
type row struct {
	name string
	run  func(t *testing.T, d deploy.Deployer, s Seeder)
}

// Conformance runs every row below against h. Tasks 03 and 04 add rows; a row is never removed
// to make an implementation pass.
func Conformance(t *testing.T, h Harness) {
	t.Helper()
	rows := []row{
		{"LatestOfEmptyProject", rowLatestOfEmptyProject},
		{"LiveReadyNilStaysNil", rowLiveReadyNilStaysNil},
		{"GatesNilStaysNil", rowGatesNilStaysNil},
		{"CompletedAtSurvives", rowCompletedAtSurvives},
		{"RollbackCreatesNoDeployment", rowRollbackCreatesNoDeployment},
		{"ShipNamesTheRun", rowShipNamesTheRun},
		{"UnknownPostureFailsClosed", rowUnknownPostureFailsClosed},
		{"EnvironmentForIsTheTargets", rowEnvironmentForIsTheTargets},
		{"ScansReadTheDeploymentsEnvironment", rowScansReadTheDeploymentsEnvironment},
		// Task 03.
		{"ProvisioningSurvives", rowProvisioningSurvives},
		{"EmptyVersionIsUnknown", rowEmptyVersionIsUnknown},
		{"ReplicasAbsentIsNil", rowReplicasAbsentIsNil},
		// Task 04.
		{"RollbackSourceIsArgument", rowRollbackSourceIsArgument},
		{"CancelFinishedIsNotAnError", rowCancelFinishedIsNotAnError},
		{"GrantDeveloperRoundTrips", rowGrantDeveloperRoundTrips},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			d, s := h(t)
			s.Project(deploy.Project{Ref: confProject, DefaultBranch: "main"})
			r.run(t, d, s)
		})
	}
}

// deployments reads the history and indexes it by ID; a read error fails the row.
func deployments(t *testing.T, d deploy.Deployer) ([]deploy.Deployment, map[string]deploy.Deployment) {
	t.Helper()
	ds, err := d.Deployments(context.Background(), confProject)
	if err != nil {
		t.Fatalf("Deployments: %v", err)
	}
	byID := make(map[string]deploy.Deployment, len(ds))
	for _, x := range ds {
		byID[x.ID] = x
	}
	return ds, byID
}

// need fails the row when a seeded deployment did not read back, so no assertion on a
// missing row passes vacuously.
func need(t *testing.T, byID map[string]deploy.Deployment, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			t.Fatalf("Deployments lost the seeded row %q (read %d rows)", id, len(byID))
		}
	}
}

// seedRow is a built deployment in its own environment, so it is that environment's newest
// and may carry LiveReady and Gates (Reader.Deployments).
func seedRow(id, env string, at time.Time) deploy.Deployment {
	return deploy.Deployment{ID: id, Environment: env, Branch: env, Commit: confCommit,
		Build: deploy.BuildSucceeded, CreatedAt: at, CompletedAt: at.Add(3 * time.Minute)}
}

// LatestOfEmptyProject kills an implementation that turns "never deployed" into an error,
// which the lens would draw as a failure.
func rowLatestOfEmptyProject(t *testing.T, d deploy.Deployer, s Seeder) {
	s.NoDeployments(confProject)
	got, ok, err := d.Latest(context.Background(), confProject)
	if err != nil || ok || !reflect.DeepEqual(got, deploy.Deployment{}) {
		t.Fatalf("Latest of a never-deployed project = (%+v, %v, %v), want (Deployment{}, false, nil)", got, ok, err)
	}
}

// LiveReadyNilStaysNil kills flattening the pointer to int, which makes "not measured" read
// as "down".
func rowLiveReadyNilStaysNil(t *testing.T, d deploy.Deployer, s Seeder) {
	zero := 0
	unmeasured := seedRow("d-unmeasured", "feat-a", confBase)
	down := seedRow("d-down", "feat-b", confBase.Add(time.Minute))
	down.LiveReady = &zero
	s.Deployment(confProject, unmeasured)
	s.Deployment(confProject, down)
	_, byID := deployments(t, d)
	need(t, byID, "d-unmeasured", "d-down")
	u, w := byID["d-unmeasured"], byID["d-down"]
	if u.LiveReady != nil {
		t.Errorf("LiveReady nil (not measured) read back as &%d", *u.LiveReady)
	}
	if w.LiveReady == nil || *w.LiveReady != 0 {
		t.Errorf("LiveReady &0 (measured, down) read back as %v", w.LiveReady)
	}
}

// GatesNilStaysNil kills "no gate ran" becoming "all gates clean".
func rowGatesNilStaysNil(t *testing.T, d deploy.Deployer, s Seeder) {
	none := seedRow("d-nogates", "feat-a", confBase)
	gated := seedRow("d-gated", "feat-b", confBase.Add(time.Minute))
	gated.Gates = map[string]deploy.Gate{"iac": {Verdict: "passed"}}
	s.Deployment(confProject, none)
	s.Deployment(confProject, gated)
	_, byID := deployments(t, d)
	need(t, byID, "d-nogates", "d-gated")
	if g := byID["d-nogates"].Gates; g != nil {
		t.Errorf("Gates nil (none ran) read back as %#v", g)
	}
	if g := byID["d-gated"].Gates; g["iac"].Verdict != "passed" {
		t.Errorf("a gate that ran read back as %#v", g)
	}
}

// CompletedAtSurvives kills dropping the recorded duration, so no deployment is ever current
// (task 08 rule 6, review I3). To the second: Panacea rounds the duration.
func rowCompletedAtSurvives(t *testing.T, d deploy.Deployer, s Seeder) {
	done := seedRow("d-done", "feat-a", confBase)
	done.CompletedAt = confBase.Add(4*time.Minute + 12*time.Second)
	open := seedRow("d-open", "feat-b", confBase.Add(time.Minute))
	open.Build, open.CompletedAt = deploy.BuildRunning, time.Time{}
	s.Deployment(confProject, done)
	s.Deployment(confProject, open)
	_, byID := deployments(t, d)
	need(t, byID, "d-done", "d-open")
	got := byID["d-done"].CompletedAt
	if diff := got.Sub(done.CompletedAt); got.IsZero() || diff <= -time.Second || diff >= time.Second {
		t.Errorf("CompletedAt read back as %v, want %v to the second", got, done.CompletedAt)
	}
	if got := byID["d-open"].CompletedAt; !got.IsZero() {
		t.Errorf("a zero CompletedAt read back as %v", got)
	}
}

// RollbackCreatesNoDeployment kills a fake that invents a rollback row, hiding 7P-4 from every
// test above it.
func rowRollbackCreatesNoDeployment(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Deployment(confProject, imaged(seedRow("d1", "feat-x", confBase)))
	s.Deployment(confProject, imaged(seedRow("d2", "feat-x", confBase.Add(10*time.Minute))))
	s.RollbackTarget(confProject, "d1", deploy.WarmthWarm)
	// Both rows must read back first, or a Deployments that answers none passes: [] == [] after a
	// Rollback answered from the RollbackTarget seed alone (review t01 M-2).
	before, byID := deployments(t, d)
	need(t, byID, "d1", "d2")
	res, err := d.Rollback(context.Background(), confProject, "d1")
	if err != nil {
		t.Fatalf("Rollback(d1): %v", err)
	}
	if res.SourceDeploymentID != "d1" {
		t.Errorf("SourceDeploymentID = %q, want the deployment rolled back to, d1", res.SourceDeploymentID)
	}
	after, _ := deployments(t, d)
	if !slices.Equal(ids(before), ids(after)) {
		t.Errorf("Rollback changed the history: before %q, after %q", ids(before), ids(after))
	}
}

func ids(ds []deploy.Deployment) []string {
	out := make([]string, len(ds))
	for i, x := range ds {
		out[i] = x.ID
	}
	return out
}

// ShipNamesTheRun kills a Run name that is not the Deployment id, which breaks the join
// (plan §2 step 6). The project has no history, so any row is the Ship's.
func rowShipNamesTheRun(t *testing.T, d deploy.Deployer, s Seeder) {
	s.NoDeployments(confProject)
	s.BranchEnvironment(confProject, "feat/x", "feat-x")
	res, err := d.Ship(context.Background(), confProject, deploy.ShipRequest{Branch: "feat/x", Commit: confCommit})
	if err != nil || res.Run == "" {
		t.Fatalf("Ship = (%+v, %v), want a named run", res, err)
	}
	ds, _ := deployments(t, d)
	for _, x := range ds {
		if x.ID != res.Run {
			t.Errorf("after Ship named %q, Deployments holds %q: the run is not the deployment's id", res.Run, x.ID)
		}
	}
}

// UnknownPostureFailsClosed kills mapping an unknown word to public.
func rowUnknownPostureFailsClosed(t *testing.T, d deploy.Deployer, s Seeder) {
	s.BranchEnvironment(confProject, "feat/x", "feat-x")
	s.PostureWord(confProject, "feat/x", "open-to-all")
	p, err := d.Posture(context.Background(), confProject, "feat/x")
	if err != nil {
		t.Fatalf("Posture: %v", err)
	}
	if p.Mode != deploy.PostureKPMGSSO || p.ModeKnown || p.ModeRaw != "open-to-all" {
		t.Errorf("an unknown word read as (Mode %q, ModeKnown %v, ModeRaw %q), want (%q, false, %q)",
			p.Mode, p.ModeKnown, p.ModeRaw, deploy.PostureKPMGSSO, "open-to-all")
	}
}

// EnvironmentForIsTheTargets kills returning the branch unchanged, or reading the answer off
// an environment row, which only works for trunk-like names (review C2). The answers are
// Panacea's real ones, seeded, and no row exists: the answer exists before the first deploy.
func rowEnvironmentForIsTheTargets(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Environments(confProject, nil)
	want := map[string]string{
		"feat/x":                          "feat-x",
		"lane/7-deploy-connector-watcher": "lane-7-deploy-connec", // 31 characters in, 20 out
		"main":                            "prod",
	}
	for branch, env := range want {
		s.BranchEnvironment(confProject, branch, env)
	}
	ctx := context.Background()
	if envs, err := d.Environments(ctx, confProject); err != nil || len(envs) != 0 {
		t.Fatalf("Environments = (%+v, %v), want no rows", envs, err)
	}
	for branch, env := range want {
		got, err := d.EnvironmentFor(ctx, confProject, branch)
		if err != nil || got != env {
			t.Errorf("EnvironmentFor(%q) = (%q, %v), want the target's %q", branch, got, err, env)
		}
	}
}

// ScansReadTheDeploymentsEnvironment kills dropping env, which on Panacea reads production's
// namespace and shows its PASS as the branch release's verdict (review C3). Reading prod back
// as well kills an implementation that ignores env and answers the last report seeded.
func rowScansReadTheDeploymentsEnvironment(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Deployment(confProject, seedRow("d-feat", "feat-x", confBase))
	report := func(verdict string) deploy.ScanReport {
		return deploy.ScanReport{DeploymentID: "d-feat", Overall: verdict,
			Scans: []deploy.Scan{{Name: "iac", Status: verdict, Mode: "enforce"}}}
	}
	s.Scans(confProject, "prod", "d-feat", report("PASS"))
	s.Scans(confProject, "feat-x", "d-feat", report("FAIL"))
	ctx := context.Background()
	for env, want := range map[string]string{"feat-x": "FAIL", "prod": "PASS"} {
		r, err := d.Scans(ctx, confProject, env, "d-feat")
		if err != nil {
			t.Fatalf("Scans(%q, d-feat): %v", env, err)
		}
		i := slices.IndexFunc(r.Scans, func(x deploy.Scan) bool { return x.Name == "iac" })
		if i < 0 || r.Scans[i].Status != want {
			t.Errorf("Scans(%q, d-feat) iac = %+v, want %s", env, r.Scans, want)
		}
	}
}

// ProvisioningSurvives kills dropping the backing-service outcomes, so "app deployed, database
// failed" reads green (task 03).
func rowProvisioningSurvives(t *testing.T, d deploy.Deployer, s Seeder) {
	d1 := seedRow("d-prov", "feat-a", confBase)
	d1.Provisioning = []deploy.Provisioning{
		{Component: "database", Status: "failed", Detail: "role creation timed out"},
		{Component: "storage", Status: "succeeded"},
	}
	s.Deployment(confProject, d1)
	_, byID := deployments(t, d)
	need(t, byID, "d-prov")
	if got := byID["d-prov"].Provisioning; !reflect.DeepEqual(got, d1.Provisioning) {
		t.Errorf("Provisioning read back as %+v, want %+v", got, d1.Provisioning)
	}
}

// EmptyVersionIsUnknown kills fabricating a version the target did not report, such as the
// commit's first seven characters (task 03). A reported one reads back verbatim.
func rowEmptyVersionIsUnknown(t *testing.T, d deploy.Deployer, s Seeder) {
	blank := seedRow("d-noversion", "feat-a", confBase)
	named := seedRow("d-version", "feat-b", confBase.Add(time.Minute))
	named.Version = "2.3.1"
	s.Deployment(confProject, blank)
	s.Deployment(confProject, named)
	_, byID := deployments(t, d)
	need(t, byID, "d-noversion", "d-version")
	if v := byID["d-noversion"].Version; v != "" {
		t.Errorf("an unreported version read back as %q (commit %s), want \"\"", v, confCommit)
	}
	if v := byID["d-version"].Version; v != "2.3.1" {
		t.Errorf("a reported version read back as %q, want 2.3.1", v)
	}
}

// ReplicasAbsentIsNil kills rendering an unread namespace as 0/0 pods (task 03; Panacea omits
// the key when it could not look, handlers/projects.go:2125-2140).
func rowReplicasAbsentIsNil(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Environments(confProject, []deploy.Environment{
		{Name: "production", Branch: "main", Primary: true, Replicas: &deploy.Replicas{Ready: 2, Desired: 3}},
		{Name: "feat-a", Branch: "feat-a"},
	})
	envs, err := d.Environments(context.Background(), confProject)
	if err != nil || len(envs) != 2 {
		t.Fatalf("Environments = (%+v, %v), want the 2 seeded rows", envs, err)
	}
	byName := map[string]deploy.Environment{envs[0].Name: envs[0], envs[1].Name: envs[1]}
	if r := byName["feat-a"].Replicas; r != nil {
		t.Errorf("an unmeasured environment read back Replicas %+v, want nil", *r)
	}
	if r := byName["production"].Replicas; r == nil || *r != (deploy.Replicas{Ready: 2, Desired: 3}) {
		t.Errorf("a measured environment read back Replicas %v, want &{2 3}", r)
	}
}

// imaged is a seeded row with the image a rollback returns to: Panacea refuses to roll back to a
// deployment with none (handlers/projects.go:1574-1575).
func imaged(d deploy.Deployment) deploy.Deployment {
	d.Image = "registry.test/conformance-portal/app:" + d.ID
	return d
}

// RollbackSourceIsArgument kills a connector that reports a new id, which task 10 would send to
// the Hub and be refused with a 409 (task 04). Warm and cold both.
func rowRollbackSourceIsArgument(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Deployment(confProject, imaged(seedRow("d-warm", "feat-x", confBase)))
	s.Deployment(confProject, imaged(seedRow("d-cold", "feat-x", confBase.Add(time.Minute))))
	s.Deployment(confProject, imaged(seedRow("d-now", "feat-x", confBase.Add(2*time.Minute))))
	s.RollbackTarget(confProject, "d-warm", deploy.WarmthWarm)
	s.RollbackTarget(confProject, "d-cold", deploy.WarmthCold)
	for id, warmth := range map[string]deploy.Warmth{"d-warm": deploy.WarmthWarm, "d-cold": deploy.WarmthCold} {
		res, err := d.Rollback(context.Background(), confProject, id)
		if err != nil || res.SourceDeploymentID != id || res.Warmth != warmth {
			t.Errorf("Rollback(%s) = (%+v, %v), want SourceDeploymentID %s, Warmth %q", id, res, err, id, warmth)
		}
	}
}

// CancelFinishedIsNotAnError kills an error toast for an undo that raced the build's end: a
// finished run answers AlreadyFinished with a nil error.
func rowCancelFinishedIsNotAnError(t *testing.T, d deploy.Deployer, s Seeder) {
	s.Deployment(confProject, seedRow("d-done", "feat-x", confBase))
	res, err := d.Cancel(context.Background(), confProject, "d-done")
	if err != nil || res.Outcome != deploy.AlreadyFinished {
		t.Errorf("Cancel of a succeeded run = (%+v, %v), want (AlreadyFinished, nil)", res, err)
	}
}

// GrantDeveloperRoundTrips kills sending the domain's "developer" where Panacea's writer spells
// "editor": Panacea builds the group "project-<name>-"+role unchecked and lists any suffix but
// -owner and -editor as viewer (clients/keycloak.go:192, :503-512), so a developer granted as
// "developer" reads back a viewer. Read from source; never tried live (task 04).
func rowGrantDeveloperRoundTrips(t *testing.T, d deploy.Deployer, _ Seeder) {
	ctx := context.Background()
	const who = "a.person@kpmg.com.au"
	if err := d.Grant(ctx, confProject, deploy.GrantRequest{Grantee: who, Role: deploy.RoleDeveloper}); err != nil {
		t.Fatalf("Grant(developer): %v", err)
	}
	grants, err := d.Access(ctx, confProject)
	if err != nil {
		t.Fatalf("Access: %v", err)
	}
	i := slices.IndexFunc(grants, func(g deploy.AccessGrant) bool { return g.Grantee == who })
	if i < 0 || grants[i].Role != deploy.RoleDeveloper || grants[i].Legacy || grants[i].Group != who {
		t.Errorf("after Grant(developer), Access = %+v, want %s as a developer on a person's row", grants, who)
	}
}
