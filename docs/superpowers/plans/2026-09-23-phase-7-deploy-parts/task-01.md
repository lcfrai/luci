### Task 01: The `deploy` domain — the interface a lift swaps behind

**Half:** 7a. **Hours:** 2. **Depends on:** nothing.

This is the seam `OD-13` asks for. Every later task holds a `deploy.Deployer` (or one of its four
smaller interfaces) and never a Panacea type. When Panacea is lifted into this repo, a native
implementation passes `deploytest.Conformance` and replaces `panacea.Client` in `main.go`
(seam S7-N1). **Nothing in this package may name a Panacea route, envelope, JSON spelling or
status string.** A reviewer checks that with one grep (acceptance, below).

**Files:**
- `control-plane/internal/deploy/deploy.go` (new) — the interfaces and domain types
- `control-plane/internal/deploy/errors.go` (new) — `Error`, `Kind`, `KindOf`
- `control-plane/internal/deploy/validate.go` (new) — `ValidCommit`, `ParsePostureMode`
- `control-plane/internal/deploy/deploy_test.go` (new)
- `control-plane/internal/deploy/deploytest/fake.go` (new) — `Fake`
- `control-plane/internal/deploy/deploytest/conformance.go` (new) — `Harness`, `Seeder`, `Conformance`
- `control-plane/internal/deploy/deploytest/fake_test.go` (new) — runs `Conformance` on `Fake`

#### Interfaces — complete

```go
// Package deploy is the deploy side of the control plane, as the rest of the control plane sees
// it. It is deliberately ignorant of Panacea: package deploy/panacea implements Deployer over
// panacea-api today (OD-13), and a native implementation replaces it when Panacea is lifted into
// this repository. Nothing here names a Panacea route, envelope, field spelling or status string.
package deploy

import (
	"context"
	"io"
	"time"
)

// ProjectRef names a project on the deploy target. Opaque: compare it, never parse it.
// For Panacea it is the {name} path segment.
type ProjectRef string

// Deployer is everything phase 7 asks of a deploy target. It is four smaller interfaces so each
// consumer depends on the least it needs: the watcher holds a Reader, Ship holds an Actor and a
// Reader, the governance routes hold a Governor.
type Deployer interface {
	Identity
	Reader
	Actor
	Governor
}

// Identity is who the stored credential is, and what it can see.
type Identity interface {
	// Whoami is the credential's principal. KindUnauthorized when the credential is rejected.
	Whoami(ctx context.Context) (Account, error)
	// Projects lists the projects the credential can read. Order is the target's.
	Projects(ctx context.Context) ([]Project, error)
}

// Reader is every read the lens and the watcher make.
type Reader interface {
	// Environments is the project's real environments, as the target reports them.
	Environments(ctx context.Context, p ProjectRef) ([]Environment, error)
	// EnvironmentFor is the TARGET's own name for the environment a git branch deploys to, for
	// any branch, whether or not that environment exists yet. It is the cockpit's ONLY
	// branch → environment mapping: nothing above this interface derives one, and nothing
	// compares a branch name with an Environment's Name or Branch. The answer is joined to
	// Environment rows with SameEnvironment (Panacea's answer for a production branch is "prod",
	// while its row is named "production"). For Panacea it is the access route's answer, which
	// reads the production branch from the label only; where that disagrees with the deploy, the
	// view shows it from the deployment's own label and corrects nothing (task 03, task 08 rule 5a).
	EnvironmentFor(ctx context.Context, p ProjectRef, branch string) (string, error)
	// Latest is the newest deployment across ALL environments. ok is false, with a nil error,
	// when the project has never been deployed.
	Latest(ctx context.Context, p ProjectRef) (d Deployment, ok bool, err error)
	// Deployments is the full history, newest first. Only the newest per environment carries
	// LiveReady and Gates.
	Deployments(ctx context.Context, p ProjectRef) ([]Deployment, error)
	// Run is one build's progress. KindNotFound for a run of another project.
	Run(ctx context.Context, p ProjectRef, run string) (RunProgress, error)
	// Scans is the security report scoped to one deployment IN ITS ENVIRONMENT. env is that
	// deployment's Environment and is sent whenever it is non-empty: Panacea reads a missing env
	// as "production" (handlers/security_status.go:192-201) and env-keyed scanners answer with
	// that environment's latest result (:211-213), so omitting it shows production's verdict as
	// a branch release's (review C3). A production deployment's Environment is "prod", which
	// Panacea maps to the same base namespace as a missing env (handlers/tier_store.go:106-113).
	Scans(ctx context.Context, p ProjectRef, env, deploymentID string) (ScanReport, error)
	// Pods and Events explain an environment that is not serving.
	Pods(ctx context.Context, p ProjectRef, env string) ([]Pod, error)
	Events(ctx context.Context, p ProjectRef, env string) ([]Event, error)
	// Diagnosis is the target's own prose diagnosis of an environment, rendered verbatim.
	Diagnosis(ctx context.Context, p ProjectRef, env string) (Diagnosis, error)
	// Stream opens a live stream of typed events. The caller closes it. The connector parses the
	// target's own wire format, so nothing above this interface — and nothing in the desktop —
	// knows it.
	Stream(ctx context.Context, p ProjectRef, kind StreamKind, id string) (Stream, error)
	// SameEnvironment is the TARGET's rule for whether two environment names name one
	// environment. It is here, not in the view, because the rule is the target's: Panacea's is
	// exact equality plus prod == production (task 03); a native deployer brings its own.
	SameEnvironment(a, b string) bool
}

// Actor is every act that changes what is deployed.
type Actor interface {
	// Ship builds and deploys one commit of one branch. It returns once the target has accepted
	// the run and named it; it does not wait for the build.
	Ship(ctx context.Context, p ProjectRef, req ShipRequest) (ShipResult, error)
	// Cancel stops a run that has not finished. A finished run is not an error.
	Cancel(ctx context.Context, p ProjectRef, run string) (CancelResult, error)
	// Rollback returns an environment to an earlier deployment. It creates no run and no
	// deployment: RollbackResult.SourceDeploymentID is the deployment rolled back TO.
	Rollback(ctx context.Context, p ProjectRef, deploymentID string) (RollbackResult, error)
}

// Governor is every per-branch and per-project control the lens offers.
type Governor interface {
	AutoDeploy(ctx context.Context, p ProjectRef, branch string) (AutoDeploy, error)
	SetAutoDeploy(ctx context.Context, p ProjectRef, branch string, enabled bool) (AutoDeploy, error)
	Posture(ctx context.Context, p ProjectRef, branch string) (Posture, error)
	SetPosture(ctx context.Context, p ProjectRef, branch string, c PostureChange) (PostureResult, error)
	Access(ctx context.Context, p ProjectRef) ([]AccessGrant, error)
	Grant(ctx context.Context, p ProjectRef, g GrantRequest) error
	Revoke(ctx context.Context, p ProjectRef, grantee string) error
	SetURLPrefix(ctx context.Context, p ProjectRef, branch, prefix string) (URLPrefix, error)
	ClearURLPrefix(ctx context.Context, p ProjectRef, branch string) error
}

// Account is the credential's principal.
type Account struct {
	Username string
	Email    string
	// Approved is true only when the target says so explicitly. An account waiting for team
	// approval can authenticate and can do nothing; the lens says which.
	Approved bool
	Admin    bool
}

// Project is one project the credential can read.
type Project struct {
	Ref           ProjectRef
	DisplayName   string // "" when the target has none; the lens then shows Ref
	RepoURL       string // "" when the target does not report one to this credential
	DefaultBranch string
}

// Environment is one live environment.
type Environment struct {
	Name     string    // the target's own name; joined to Deployment.Environment with SameEnvironment
	// Branch is the target's LABEL for the row, verbatim — NOT the git branch. Panacea sets it
	// to the environment key for every non-base row ("feat-x" for feat/x) and to the real branch
	// only for production (handlers/projects.go:2109-2118). Display only; never a join key —
	// EnvironmentFor is how a git branch finds its row.
	Branch   string
	Primary  bool      // the project's base environment
	Host     string    // "" when the target reports none; no scheme
	Status   string    // the target's word, displayed verbatim, never interpreted
	Replicas *Replicas // nil = not measured. Never render nil as "0/0"
}

type Replicas struct{ Ready, Desired int }

// BuildState is the build pipeline's outcome, and only that. It says nothing about serving.
type BuildState string

const (
	BuildPending    BuildState = "pending"    // accepted, not started
	BuildRunning    BuildState = "running"
	BuildSucceeded  BuildState = "succeeded"
	BuildFailed     BuildState = "failed"     // failed or cancelled; the target does not tell them apart
	BuildHistorical BuildState = "historical" // reconstructed from the target's durable record
	BuildUnknown    BuildState = "unknown"    // the target used a word this code does not know
)

// Terminal reports whether the build can change again.
func (b BuildState) Terminal() bool

// Deployment is one build of one commit into one environment.
type Deployment struct {
	ID          string // equals the run name that built it
	Environment string // the target's own word (Panacea: "prod" for the production branch); Scans takes it
	// Branch is a LABEL VALUE, never a git ref, and Panacea has two writers that disagree
	// (review I4, re-review b03 I-1, plan §12):
	//   - the API (Ship, Rebuild) stamps sanitizeLabelValue(branch) (clients/pipeline.go:609,
	//     :2351-2364: "/" and "_" become "-", other invalid characters are dropped, 63-character
	//     cap, "unknown" when empty);
	//   - the webhook-receiver (push-to-deploy) stamps sanitizeLabel(event.Branch)
	//     (webhook-receiver/pipeline/trigger.go:1270, :1553-1574: "/", "\", ":" and space become
	//     "-", "_" is KEPT, 63-character cap, "" when empty).
	// So one branch feat_x reads "feat_x" after a push and "feat-x" after a Ship. Panacea reads
	// the label back verbatim at kubernetes.go:5750. It is for display and label-to-label
	// comparison only, never normalised; nothing sends it as a branch.
	Branch      string
	Commit      string // "" when not recorded
	Version     string // "" means unknown; the lens says "version unknown"
	Image       string
	Build       BuildState
	BuildRaw    string // the target's own word, for the tooltip
	FailedStep  string
	ErrorText   string
	CreatedAt   time.Time
	DeployedBy  string // "" = not recorded, never guessed
	Trigger     string // the target's word ("manual", "git-push", …); "" = not recorded
	// LiveReady is the environment's ready-pod count NOW, and only on the newest deployment per
	// environment. nil = not measured: the lens says nothing. 0 = measured and down.
	LiveReady *int
	// Gates is each security gate that RAN for this build. nil = none ran, which is not "clean".
	Gates map[string]Gate
	// Provisioning is the backing-service outcome per component, when recorded.
	Provisioning []Provisioning
	// Reconstructed marks a row rebuilt from the durable record, not observed live.
	Reconstructed bool
}

type Gate struct {
	Verdict   string // "passed" | "failed" | "skipped", verbatim
	Criticals *int
	Highs     *int
	Findings  *int
}

type Provisioning struct {
	Component string // "database" | "storage" | "ai" | "vault", verbatim
	Status    string // "pending" | "running" | "succeeded" | "failed", verbatim
	Detail    string
}

// RunProgress is one build while it runs. There is no total: the target does not say what
// remains, so the lens draws finished steps and the current one, and no bar.
type RunProgress struct {
	Run             string
	Build           BuildState
	BuildRaw        string
	StartedAt       time.Time // zero = not reported
	CurrentStep     string
	CompletedSteps  []string
	FailedStep      string
	ErrorText       string
	IgnoredFailures []string // steps that failed without failing the run
}

// ScanReport is the security picture for one deployment.
type ScanReport struct {
	DeploymentID    string
	Overall         string // "FAIL" | "INCOMPLETE" | "WARN" | "PASS" | "SKIPPED", verbatim
	Blocking        []ScanNote
	Incomplete      []ScanNote
	Scans           []Scan
	DetailsRetained *bool  // nil = the target did not say
	DetailsSource   string // "live" | "snapshot" | "mixed" | "unverified" | "none" | ""
	UpdatedAt       string // the target's timestamp, verbatim
}

type ScanNote struct{ Scan, Reason string }

type Scan struct {
	Name           string
	Status         string // "PASS" | "FAIL" | "SKIPPED", verbatim
	Mode           string // "off" | "warn" | "enforce", verbatim — shown, never changed by phase 7
	Counts         map[string]int
	Summary        string
	SkipReason     string
	ErrorReason    string
	Blocking       bool
	LastScanAt     string
	FindingsSource string
}

type Pod struct {
	Name     string
	Phase    string // verbatim
	Ready    bool
	Restarts int
	Reason   string // e.g. "Evicted"; "" when none
	Message  string
}

type Event struct {
	Type     string // "Normal" | "Warning", verbatim
	Reason   string // "BackOff", "FailedScheduling", …, verbatim
	Object   string // "Pod/app-7c9d…", verbatim
	Message  string
	LastSeen time.Time // zero = not reported
	Count    int
}

// Diagnosis is the target's own reading of an unhealthy environment. Every string is rendered
// verbatim; the control plane adds nothing to it.
type Diagnosis struct {
	Health   string // "healthy" | "degraded" | "down" | "idle" | "unknown", verbatim
	Summary  string
	Findings []Finding
	// Source is how the target produced it: "rules", or "rules+ai". The lens says so when a
	// model was involved, because a model's "likely cause" is a guess, and must read as one.
	Source string
}

type Finding struct {
	Problem      string
	LikelyCause  string
	SuggestedFix string
	Evidence     []string
	Severity     string // "critical" | "warning" | "info", verbatim
	Owner        string // "app" | "platform" | "unclear", verbatim
}

type StreamKind string

const (
	StreamRunLogs   StreamKind = "run-logs"  // id is a run name
	StreamReadiness StreamKind = "readiness" // id is an environment name
)

// Stream yields events until the target ends it (io.EOF) or ctx is done.
type Stream interface {
	Next() (StreamEvent, error)
	Close() error
}

// StreamEvent is exactly one of its three fields.
type StreamEvent struct {
	Log       *LogLine
	Readiness *ReadinessTick
	// Problem is the target's in-stream error sentence (e.g. "pipeline run not found: …"),
	// rendered verbatim; the stream ends after it.
	Problem string
}

type LogLine struct {
	Step string // the pipeline step ("system" for the target's own notices), verbatim
	Line string
}

// ReadinessTick is the environment's ready count now. Measured false = the target could not
// look, which is never "down".
type ReadinessTick struct {
	Measured    bool
	Ready       *int
	CrashReason string // present only when measured down and the target knows why
	Restarts    *int
	OOMKilled   bool
	ExitCode    *int
}

// ShipRequest names exactly what to build. Commit is 40 lowercase hex (ValidCommit).
type ShipRequest struct {
	Branch string
	Commit string
	// NoCache forces a clean build. nil = the target's default; never sent as false.
	NoCache *bool
}

type ShipResult struct {
	Run string // the run name; also the ID of the Deployment it will produce
}

type CancelOutcome string

const (
	Cancelled       CancelOutcome = "cancelled"
	AlreadyFinished CancelOutcome = "already_finished"
)

type CancelResult struct {
	Outcome CancelOutcome
	Message string // the target's sentence, verbatim
}

type Warmth string

const (
	WarmthWarm    Warmth = "warm"
	WarmthCold    Warmth = "cold"
	WarmthUnknown Warmth = ""
)

type RollbackResult struct {
	SourceDeploymentID string // the deployment rolled back TO — equals the argument
	Image              string
	Warmth             Warmth
	ColdStartSeconds   int // advisory; 0 when warm or unknown
}

type AutoDeploy struct {
	Branch           string
	Explicit         *bool // nil = the branch follows the project default
	ProjectDefault   bool
	Effective        bool
	ProductionBranch bool
}

// PostureMode is who may open a branch environment's URL. Exactly five values exist.
type PostureMode string

const (
	PosturePanaceaSSO  PostureMode = "panacea-sso"  // platform sign-in + the project's allowlist
	PostureKPMGSSO     PostureMode = "kpmg-sso"     // platform sign-in, any KPMG identity (the safe default)
	PosturePassword    PostureMode = "password"     // one shared password
	PostureBypassToken PostureMode = "bypass-token" // sign-in, or a bypass token
	PosturePublic      PostureMode = "none"         // anyone on the internet
)

type Posture struct {
	Branch           string
	Environment      string
	Mode             PostureMode // ParsePostureMode of ModeRaw; an unknown word reads as PostureKPMGSSO
	ModeRaw          string
	ModeKnown        bool // false when ModeRaw was not one of the five
	PasswordSet      bool
	BypassTokenCount int
	IPAllowlist      []string
}

type PostureChange struct {
	Mode PostureMode
	// Password is sent only when Mode is PosturePassword. It is never logged, stored or echoed.
	Password string
	// RotateBypassToken asks the target to mint a fresh token (Mode PostureBypassToken only).
	RotateBypassToken bool
}

type PostureResult struct {
	Posture Posture
	// Enforced is false when the target SAVED the new mode but could not apply it, so traffic
	// still meets the OLD mode. The lens must say so; a saved-but-not-enforced posture drawn as
	// done is the silent-override class this program keeps paying for.
	Enforced bool
	// Message is the target's own sentence about the outcome, verbatim.
	Message string
	// BypassTokens are freshly minted tokens, returned exactly once. Empty when none was minted.
	// The control plane passes them to the desktop once and never keeps them.
	BypassTokens []string
}

// AccessRole is the domain's vocabulary. The Panacea connector maps RoleDeveloper to its wire
// spelling; nothing above the connector knows that spelling.
type AccessRole string

const (
	RoleViewer    AccessRole = "viewer"
	RoleDeveloper AccessRole = "developer"
	RoleOwner     AccessRole = "owner"
	RoleUnknown   AccessRole = "unknown"
)

// AccessGrant is one person's (or one legacy group's) role on the project.
type AccessGrant struct {
	Grantee string // a username or email; "" for a legacy group binding
	Group   string // the target's group name, shown only for a legacy binding
	Role    AccessRole
	RoleRaw string
}

type GrantRequest struct {
	Grantee string // a username or email; one person, never a list
	Role    AccessRole
}

type URLPrefix struct {
	Branch string
	Prefix string
	Host   string
}
```

```go
package deploy

import (
	"errors"
	"time"
)

// Kind is what went wrong, in the terms the control plane's routes map to problem codes.
type Kind string

const (
	KindNotConnected Kind = "not_connected" // no credential, or no address, is configured
	KindUnavailable  Kind = "unavailable"   // transport failure, timeout, 5xx, 429
	KindUnauthorized Kind = "unauthorized"  // the credential was rejected
	KindForbidden    Kind = "forbidden"     // authenticated, not allowed
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"      // the target refused because of its current state
	KindRejected     Kind = "rejected"      // the target refused the request as malformed
)

// Error is every failure a Deployer returns.
type Error struct {
	Kind Kind
	// Message is the deploy target's own sentence, safe to render verbatim, at most 500 bytes.
	// It is "" for KindUnavailable: a 5xx sentence can carry internal detail and is only logged.
	Message    string
	Status     int           // the upstream HTTP status; 0 for a transport failure
	RetryAfter time.Duration // from a 429 or 503, else 0
	Op         string        // "ship", "rollback", "environments", … for the log line
	Cause      error         // logged, never rendered
}

func (e *Error) Error() string // "deploy: <op>: <kind>[: <message>]" — never includes Cause's text of a credential
func (e *Error) Unwrap() error // Cause

// KindOf is the Kind of err's *Error, or KindUnavailable for any other non-nil error, or "".
func KindOf(err error) Kind

// ErrNotConnected is returned by every method of a Deployer that has no credential.
var ErrNotConnected = &Error{Kind: KindNotConnected, Op: "connect"}

var _ = errors.As // errors.As is how callers read an *Error
```

```go
package deploy

// ValidCommit reports whether c is 40 lowercase hex — the one commit form phase 7 ever sends.
func ValidCommit(c string) bool

// ParsePostureMode maps a word to one of the five modes. An unknown or empty word returns
// (PostureKPMGSSO, false): fail closed to the safe default, and say it was unknown, exactly as
// the target's own canonicalAccessMode does. It never maps anything to PosturePublic.
func ParsePostureMode(s string) (PostureMode, bool)
```

```go
package deploytest

// Fake is an in-memory Deployer. Every method is safe for concurrent use. Its clock is
// injectable so tests of the watcher and of Ship are deterministic.
type Fake struct{ /* unexported */ }

func NewFake(now func() time.Time) *Fake

// Seeding — tests arrange state through these, never by reaching into the struct.
func (f *Fake) SetAccount(a deploy.Account)
func (f *Fake) AddProject(p deploy.Project)
func (f *Fake) SetEnvironments(p deploy.ProjectRef, envs []deploy.Environment)
// SetBranchEnvironment seeds EnvironmentFor's answer for one branch. The Fake derives NOTHING
// (no copy of any target's naming rule): an unseeded branch returns a KindRejected *Error whose
// Message names the branch, so a test that forgot to seed fails loudly instead of matching
// names by accident.
func (f *Fake) SetBranchEnvironment(p deploy.ProjectRef, branch, env string)
func (f *Fake) AddDeployment(p deploy.ProjectRef, d deploy.Deployment) // newest-first order is kept by CreatedAt
// SetScans seeds the report Scans(p, env, deploymentID) returns. The key is the EXACT env
// string (the Fake knows no target's env aliases); an unseeded pair is KindNotFound, so a
// caller that drops or mangles env fails loudly (review C3). Panacea itself never answers an
// error here (a mangled env gets a 200 with NOT_RUN or empty rows, security_status.go:204-254):
// this KindNotFound is a test-bug signal, and task 07 step 5 says what the watcher does with it
// (Scans unset, not stale, retried; re-review b03 M-2). The conformance suite does not exercise
// an unseeded pair: panaceatest answers it 500 plus t.Errorf, also a test-bug signal (task 03).
func (f *Fake) SetScans(p deploy.ProjectRef, env, deploymentID string, r deploy.ScanReport)
// Advance moves a run to a new state; a terminal state also sets the Deployment's Build.
func (f *Fake) Advance(p deploy.ProjectRef, run string, to deploy.BuildState)
// FailNext makes the next call of the named method return err. Method names are the
// interface's: "Ship", "Rollback", "Environments", …
func (f *Fake) FailNext(method string, err error)
// Calls is every call made, in order, as "Method(project,arg)" — for asserting what was NOT called.
func (f *Fake) Calls() []string

var _ deploy.Deployer = (*Fake)(nil)
```

```go
package deploytest

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

// Conformance runs every row below against h. Tasks 03 and 04 add rows; a row is never removed
// to make an implementation pass.
func Conformance(t *testing.T, h Harness)
```

#### Conformance rows this task writes (tasks 03 and 04 add more)

| Row | Assertion | The mutant it kills |
|---|---|---|
| `LatestOfEmptyProject` | `Latest` on a project with `NoDeployments` returns `(Deployment{}, false, nil)` | an implementation that turns "never deployed" into an error, which the lens would draw as a failure |
| `LiveReadyNilStaysNil` | a seeded deployment with `LiveReady: nil` reads back `nil`, and one with `&0` reads back `&0` | flattening the pointer to `int`, which makes "not measured" read as "down" |
| `GatesNilStaysNil` | `Gates: nil` reads back `nil`, not an empty map | "no gate ran" becoming "all gates clean" |
| `RollbackCreatesNoDeployment` | after `Rollback(d1)`, `Deployments` has the same length and `SourceDeploymentID == "d1"` | a fake that invents a rollback row, hiding 7P-4 from every test above it |
| `ShipNamesTheRun` | `Ship` returns a non-empty `Run`, and the next `Deployments` either lacks it or has it with `ID == Run` | a Run name that is not the Deployment id, which breaks the join (§2 step 6) |
| `UnknownPostureFailsClosed` | an environment seeded with posture word `"open-to-all"` reads `Mode == PostureKPMGSSO`, `ModeKnown == false` | mapping an unknown word to public |
| `EnvironmentForIsTheTargets` | seeded `feat/x` → `feat-x`, `lane/7-deploy-connector-watcher` (31 characters) → `lane-7-deploy-connec`, and `main` → `prod` read back exactly, with **no** `Environments` row for any of them (the answer exists before the first deploy) | returning the branch unchanged, or reading the answer off an environment row, which only works for trunk-like names (review C2) |
| `ScansReadTheDeploymentsEnvironment` | seed `Scans(p, "prod", "d-feat", <iac PASS>)` and `Scans(p, "feat-x", "d-feat", <iac FAIL>)`; `Scans(ctx, p, "feat-x", "d-feat")` reads iac `FAIL` | dropping `env`, which on Panacea reads production's namespace and shows its PASS as the branch release's verdict (review C3) |

#### Tests to write first (`deploy_test.go`, `fake_test.go`)

| Test | Assertion | Mutant killed |
|---|---|---|
| `TestValidCommit` | 40 lowercase hex → true; 39, 41, uppercase, `"main"`, `""` → false | accepting a symbolic ref, which would ship whatever the branch holds when the pipeline clones |
| `TestParsePostureModeNeverPublic` | every string in a table of 12 (the five, case variants, `""`, `"public"`, `"open"`) — only `"none"` yields `PosturePublic` | a case-insensitive match or a `"public"` synonym that opens a branch to the internet |
| `TestErrorNeverRendersCause` | `(&Error{Kind: KindUnauthorized, Message: "invalid api key", Cause: errors.New("Bearer pnc_abc")}).Error()` does not contain `pnc_` | an `Error()` that formats the cause, which is where a token would leak into a log |
| `TestKindOf` | `KindOf(nil) == ""`; `KindOf(io.EOF) == KindUnavailable`; a wrapped `*Error` yields its kind | treating an unexpected error as "not found" |
| `TestBuildStateTerminal` | succeeded, failed, historical → true; pending, running, unknown → false | the watcher leaving the 3 s cadence on "unknown" |
| `TestFakeConformance` | `Conformance(t, fakeHarness)` passes | — the suite itself |
| `TestFakeFailNextIsOneShot` | `FailNext("Ship", e)`; first `Ship` returns `e`, second succeeds | a sticky failure that masks a retry bug in task 09 |

#### Steps

- [ ] Write `deploy_test.go` and `fake_test.go` against the declarations above; run; they fail to
  compile (`undefined: deploy.ValidCommit`, …).
- [ ] Write `deploy.go`, `errors.go`, `validate.go` exactly as declared.
- [ ] Write `deploytest/fake.go`: a mutex, maps keyed by `ProjectRef`, run names `"run-"` + a
  counter, `Ship` appending a `BuildPending` deployment whose `ID` is the run, `Rollback` returning
  the argument as `SourceDeploymentID` and adding nothing.
- [ ] Write `deploytest/conformance.go` with the seven rows, each a `t.Run`.
- [ ] Run the acceptance commands. Commit:
  `git add control-plane/internal/deploy && git commit -F <msg> -- control-plane/internal/deploy`.

#### Acceptance

```
go test ./control-plane/internal/deploy/... -race -count=1
→ ok  …/internal/deploy  and  ok  …/internal/deploy/deploytest

grep -rniE 'panacea|/api/v1|pipelinerun|tekton|keycloak|"Succeeded"|"Pending"|pnc_' control-plane/internal/deploy/*.go control-plane/internal/deploy/deploytest/*.go
→ only the package comment's "deploy/panacea implements Deployer" line and TestErrorNeverRendersCause's literal

git diff --stat control-plane/go.mod
→ (empty)
```

#### What this task does NOT claim

- That the interface is complete for a future native deployer. It is complete for phase 7. The
  lift may add methods; the suite shows what else must hold.
- That any Panacea behaviour matches the fake. Tasks 03 and 04 run the same suite against the
  connector; until then the fake is the only implementation.
- Anything about the Hub. The domain has no release, pin or brief type: those are the Hub's
  (`hubapi.Release`) and phase 4's (`provenance`).
