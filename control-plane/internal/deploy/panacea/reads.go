package panacea

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 03: every deploy.Identity and deploy.Reader read (Stream is task 12's), each on do
// (task 02) with its route (routes.go at 406d0da95) and its envelope (plan §4). Every read
// accepts only 200: each handler below answers nothing else on success.

var _ deploy.Identity = (*Client)(nil)

// branchUnreadable is EnvironmentFor's sentence for a refusal the access route's own chain never
// writes: a 404 (a router's, for a path whose %2F was decoded into an extra segment), or a 400
// whose body names no problem (an ingress's). See EnvironmentFor.
const branchUnreadable = "Panacea could not read this branch name. It may contain `/`, which the route cannot carry."

// segment is one path segment taken from input, url.PathEscape'd. "", "." and ".." are refused
// before any request: an empty segment names a different route, and a mux cleans a dot segment
// with a redirect, which do never follows, so the caller would read some other route's answer.
func segment(op, what, v string) (string, error) {
	if v == "" || v == "." || v == ".." {
		return "", &deploy.Error{Kind: deploy.KindRejected, Op: op,
			Cause: fmt.Errorf("panacea: %s %q cannot be a path segment", what, v)}
	}
	return url.PathEscape(v), nil
}

// projectPath is P, "/api/v1/projects/" + url.PathEscape(p) (plan §4).
func projectPath(op string, p deploy.ProjectRef) (string, error) {
	s, err := segment(op, "project", string(p))
	if err != nil {
		return "", err
	}
	return apiPrefix + "projects/" + s, nil
}

// get is do for a read: GET, answered 200.
func (c *Client) get(ctx context.Context, op, path string, out any) error {
	return c.do(ctx, op, http.MethodGet, path, nil, out, http.StatusOK)
}

// mapSlice maps each element; nil stays nil, so an absent list is not an empty one.
func mapSlice[T, U any](in []T, f func(T) U) []U {
	if in == nil {
		return nil
	}
	out := make([]U, len(in))
	for i, x := range in {
		out[i] = f(x)
	}
	return out
}

// parseTime reads an RFC 3339 instant; anything else is zero, never now.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Whoami reads GET /api/v1/me (routes.go:3038), a bare meDTO. Approved only when the answer says
// true: an absent or null approved reads as not approved (handlers/me.go:36-37).
func (c *Client) Whoami(ctx context.Context) (deploy.Account, error) {
	var dto meDTO
	if err := c.get(ctx, "whoami", apiPrefix+"me", &dto); err != nil {
		return deploy.Account{}, err
	}
	return deploy.Account{Username: dto.Username, Email: dto.Email, Admin: dto.IsAdmin,
		Approved: dto.Approved != nil && *dto.Approved}, nil
}

// Projects reads GET /api/v1/projects (routes.go:1018), a listDTO[projectDTO], in its order.
func (c *Client) Projects(ctx context.Context) ([]deploy.Project, error) {
	var dto listDTO[projectDTO]
	if err := c.get(ctx, "projects", apiPrefix+"projects", &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, func(p projectDTO) deploy.Project {
		return deploy.Project{Ref: deploy.ProjectRef(p.Name), DisplayName: p.DisplayName, RepoURL: p.RepoURL,
			DefaultBranch: p.DefaultBranch}
	}), nil
}

// Environments reads GET P/environments (routes.go:1126), a listDTO[environmentDTO]. Replicas is
// nil when the key is absent (an unread namespace, handlers/projects.go:2125-2140); Host is the
// subdomain; Branch is the row's label verbatim, Name for every non-base row
// (handlers/projects.go:2109-2118), so it is never a join key.
func (c *Client) Environments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
	P, err := projectPath("environments", p)
	if err != nil {
		return nil, err
	}
	var dto listDTO[environmentDTO]
	if err := c.get(ctx, "environments", P+"/environments", &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, func(e environmentDTO) deploy.Environment {
		env := deploy.Environment{Name: e.Name, Branch: e.Branch, Primary: e.Primary, Host: e.Subdomain, Status: e.Status}
		if e.Replicas != nil {
			env.Replicas = &deploy.Replicas{Ready: e.Replicas.Current, Desired: e.Replicas.Desired}
		}
		return env
	}), nil
}

// EnvironmentFor reads GET P/branches/{branch}/access (routes.go:1430), the branch
// url.PathEscape'd, a dataDTO[branchEnvDTO], and returns data.env verbatim: branchEnv ->
// mapBranchToEnvWithProd (handlers/branch_security.go:29-37, called at
// handlers/project_security.go:646, written at :668-677), with no existence check, so it answers
// for a branch never deployed. It is the ACCESS handler's mapping, not the deploy's: branchEnv
// reads the production branch from the production-branch label only (branch_security.go:32),
// while the deploy resolves label, then primary-branch annotation, then "main"
// (clients/pipeline.go:233-237). Nothing here corrects it (task 08 rule 5a; plan §12).
//
// The route's own chain never answers 404: authedProject (routes.go:3279-3285) runs
// RequireProjectAccess, which answers 401 or 403 (middleware/rbac.go:181-183, :221-228), and
// GetBranchAccessMode writes only 400, 503 or 200 (handlers/project_security.go:634-678). So a
// 404 is a router's for a path whose %2F was decoded into an extra segment, never "no such
// project" (b02 round 2 R2; re-review b02-2 M-2). It keeps KindNotFound and reads
// branchUnreadable. A 400 whose body names no problem is an ingress refusing the path, and reads
// the same sentence, KindRejected (re-review b02-2 M-3); the handler's own 400 is JSON, for a
// blank name or branch (:637-639), and keeps Panacea's sentence. Its 503 (:641-644) is the
// ordinary 5xx rule. An empty or absent data.env, which Panacea never writes (mapBranchToEnv and
// sanitizeBranchEnv never return "", handlers/projects.go:3783-3826), is KindUnavailable with
// Status 200, the status received (re-review M-4).
func (c *Client) EnvironmentFor(ctx context.Context, p deploy.ProjectRef, branch string) (string, error) {
	const op = "environmentFor"
	P, err := projectPath(op, p)
	if err != nil {
		return "", err
	}
	b, err := segment(op, "branch", branch)
	if err != nil {
		return "", err
	}
	var dto dataDTO[branchEnvDTO]
	err = c.get(ctx, op, P+"/branches/"+b+"/access", &dto)
	var e *deploy.Error
	if errors.As(err, &e) && (e.Status == http.StatusNotFound || (e.Status == http.StatusBadRequest && e.Message == "")) {
		cp := *e
		cp.Message = branchUnreadable
		return "", &cp
	}
	if err != nil {
		return "", err
	}
	if dto.Data.Env == "" {
		return "", &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
			Cause: fmt.Errorf("panacea: GET %s/branches/%s/access answered 200 with no data.env", P, b)}
	}
	return dto.Data.Env, nil
}

// Latest reads GET P/deploys/latest (routes.go:1025), a bare deploymentDTO, newest across ALL
// environments. Its 404 is "no deployments yet for project …" (handlers/projects.go:1539) and
// reads as (Deployment{}, false, nil). The watcher calls Environments first in the same cycle,
// so a project the caller cannot read has already surfaced as its own error (a 403, never 404).
func (c *Client) Latest(ctx context.Context, p deploy.ProjectRef) (deploy.Deployment, bool, error) {
	P, err := projectPath("latest", p)
	if err != nil {
		return deploy.Deployment{}, false, err
	}
	var dto deploymentDTO
	err = c.get(ctx, "latest", P+"/deploys/latest", &dto)
	var e *deploy.Error
	if errors.As(err, &e) && e.Status == http.StatusNotFound {
		return deploy.Deployment{}, false, nil
	}
	if err != nil {
		return deploy.Deployment{}, false, err
	}
	return deploymentOf(dto), true, nil
}

// Deployments reads GET P/deployments (routes.go:1024), a listDTO[deploymentDTO], in the order
// served: newest first (handlers/projects.go:1532-1536). Rows are kept verbatim: only the newest
// per environment carries liveReadyReplicas and scanStatus, and that is Panacea's doing.
func (c *Client) Deployments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Deployment, error) {
	P, err := projectPath("deployments", p)
	if err != nil {
		return nil, err
	}
	var dto listDTO[deploymentDTO]
	if err := c.get(ctx, "deployments", P+"/deployments", &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, deploymentOf), nil
}

// deploymentOf maps one models.Deployment. Version "" stays "" (never the commit); LiveReady
// keeps the pointer; Gates is nil unless a gate ran; CompletedAt is completedAt's.
func deploymentOf(d deploymentDTO) deploy.Deployment {
	out := deploy.Deployment{ID: d.ID, Environment: d.Environment, Branch: d.Branch, Commit: d.Commit,
		Version: d.Version, Image: d.Image, Build: buildState(d.Status), BuildRaw: d.Status,
		FailedStep: d.FailedStep, ErrorText: d.ErrorMessage, CreatedAt: d.CreatedAt,
		CompletedAt: completedAt(d.CreatedAt, d.Duration), DeployedBy: d.DeployedBy, Trigger: d.Trigger,
		LiveReady: d.LiveReadyReplicas, Reconstructed: d.Backfilled,
		Provisioning: mapSlice(d.Provisioning, func(x provisioningDTO) deploy.Provisioning {
			return deploy.Provisioning{Component: x.Component, Status: x.Status, Detail: x.Detail}
		})}
	// omitempty keeps an empty map off Panacea's wire (models/models.go:724-781), and an empty one
	// names no gate that ran, so both absent and empty are nil: "none ran", never "all clean".
	if len(d.ScanStatus) > 0 {
		out.Gates = make(map[string]deploy.Gate, len(d.ScanStatus))
		for k, g := range d.ScanStatus {
			out.Gates[k] = deploy.Gate{Verdict: g.Status, Criticals: g.Criticals, Highs: g.Highs, Findings: g.Findings}
		}
	}
	return out
}

// buildState is the status table, for Deployment.status (derivePipelineStatus,
// clients/pipeline.go:2073-2084; "Unknown" before any condition, clients/kubernetes.go:5662;
// "historical", clients/deployment_backfill.go:173) and PipelineRunStatus.status ("Unknown,
// Running, Succeeded, Failed", models/models.go:2390). Pending is a run whose condition is
// neither True nor False: still running, never terminal. Any other word is BuildUnknown, and
// the caller keeps the word in BuildRaw.
func buildState(word string) deploy.BuildState {
	switch word {
	case "Succeeded":
		return deploy.BuildSucceeded
	case "Failed":
		return deploy.BuildFailed
	case "Pending", "Running":
		return deploy.BuildRunning
	case "Unknown":
		return deploy.BuildPending
	case "historical":
		return deploy.BuildHistorical
	}
	return deploy.BuildUnknown
}

// completedAt is created plus the recorded duration (deploymentDTO.Duration). "", a duration that
// does not parse or is negative, and a zero created, are all zero: never a guess (review I3).
func completedAt(created time.Time, duration string) time.Time {
	if created.IsZero() || duration == "" {
		return time.Time{}
	}
	d, err := time.ParseDuration(duration)
	if err != nil || d < 0 {
		return time.Time{}
	}
	return created.Add(d)
}

// Run reads GET P/pipelines/{run} (routes.go:1290), a dataDTO[pipelineRunDTO]. Its 404 stays
// KindNotFound and takes the connector's own sentence, Panacea's logged at warn: the route writes
// "pipeline run not found: "+err.Error() for ANY error of a live Kubernetes Get
// (handlers/projects.go:3588-3592, clients/pipeline.go:1064-1068), so an API-server timeout would
// otherwise reach the user as "not found" holding an internal address, and the cross-project IDOR
// answer (:3595-3601) is not told from it without parsing (review t04 M2).
func (c *Client) Run(ctx context.Context, p deploy.ProjectRef, run string) (deploy.RunProgress, error) {
	P, err := projectPath("run", p)
	if err != nil {
		return deploy.RunProgress{}, err
	}
	r, err := segment("run", "run", run)
	if err != nil {
		return deploy.RunProgress{}, err
	}
	var dto dataDTO[pipelineRunDTO]
	if err := c.get(ctx, "run", P+"/pipelines/"+r, &dto); err != nil {
		return deploy.RunProgress{}, c.ownNotFound("run", err, "Panacea did not find run "+run+" in this project.",
			"project", string(p), "run", run)
	}
	d := dto.Data
	name := d.Name
	if name == "" {
		name = run // the run asked for: GetPipelineRun reads it by that name (clients/pipeline.go:1064)
	}
	return deploy.RunProgress{Run: name, Build: buildState(d.Status), BuildRaw: d.Status,
		StartedAt: parseTime(d.StartTime), CurrentStep: d.CurrentStep, CompletedSteps: d.CompletedSteps,
		FailedStep: d.FailedStep, ErrorText: d.ErrorMessage, IgnoredFailures: d.IgnoredFailures}, nil
}

// Scans reads GET P/security-status?deployId=&env= (routes.go:1043), a dataDTO[securityStatusDTO].
// deployId is a run name (handlers/security_status.go:211-214) and is required: without it
// Panacea answers the newest result per scan, which is not this deployment's report. env is the
// deployment's Environment, set with url.Values whenever it is non-empty and omitted otherwise,
// as the portal builds it (backstage/packages/app/src/plugins/SecurityScanReview.tsx:437-440).
// Without it resolveEnv reads "production" (security_status.go:192-201), so a branch release
// would show production's verdicts, or NOT_RUN rows, as its own (review C3). "prod" needs no
// special case: Panacea maps it to the same namespace (handlers/tier_store.go:106-113).
func (c *Client) Scans(ctx context.Context, p deploy.ProjectRef, env, deploymentID string) (deploy.ScanReport, error) {
	const op = "scans"
	P, err := projectPath(op, p)
	if err != nil {
		return deploy.ScanReport{}, err
	}
	if deploymentID == "" {
		return deploy.ScanReport{}, &deploy.Error{Kind: deploy.KindRejected, Op: op,
			Cause: errors.New("panacea: a scan report is one deployment's; the deployment id is empty")}
	}
	q := url.Values{"deployId": {deploymentID}}
	if env != "" {
		q.Set("env", env)
	}
	var dto dataDTO[securityStatusDTO]
	if err := c.get(ctx, op, P+"/security-status?"+q.Encode(), &dto); err != nil {
		return deploy.ScanReport{}, err
	}
	s := dto.Data
	note := func(n scanNoteDTO) deploy.ScanNote { return deploy.ScanNote{Scan: n.Scan, Reason: n.Reason} }
	return deploy.ScanReport{DeploymentID: s.DeployID, Overall: s.Overall,
		Blocking: mapSlice(s.Blocking, note), Incomplete: mapSlice(s.Incomplete, note),
		Scans: mapSlice(s.Scans, func(x scanResultDTO) deploy.Scan {
			return deploy.Scan{Name: x.Scan, Status: x.Status, Mode: x.Mode, Counts: x.Counts, Summary: x.Summary,
				SkipReason: x.SkipReason, ErrorReason: x.ErrorReason, Blocking: x.IsBlocking,
				LastScanAt: x.LastScanTime, FindingsSource: x.FindingsSource}
		}),
		DetailsRetained: s.DetailsRetained, DetailsSource: s.DetailsSource, UpdatedAt: s.LastUpdated}, nil
}

// envPath is P/environments/{env}/<leaf>, env url.PathEscape'd.
func envPath(op string, p deploy.ProjectRef, env, leaf string) (string, error) {
	P, err := projectPath(op, p)
	if err != nil {
		return "", err
	}
	e, err := segment(op, "environment", env)
	if err != nil {
		return "", err
	}
	return P + "/environments/" + e + "/" + leaf, nil
}

// Pods reads GET P/environments/{env}/pods (routes.go:1148), a listDTO[podDTO].
func (c *Client) Pods(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Pod, error) {
	path, err := envPath("pods", p, env, "pods")
	if err != nil {
		return nil, err
	}
	var dto listDTO[podDTO]
	if err := c.get(ctx, "pods", path, &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, func(x podDTO) deploy.Pod {
		return deploy.Pod{Name: x.Name, Phase: x.Phase, Ready: x.Ready, Restarts: x.Restarts, Reason: x.Reason, Message: x.Message}
	}), nil
}

// Events reads GET P/environments/{env}/events (routes.go:1150), a listDTO[eventDTO]. time is
// "RFC3339 (best available)" (clients/pod_introspection.go:96): unparseable reads as zero.
func (c *Client) Events(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Event, error) {
	path, err := envPath("events", p, env, "events")
	if err != nil {
		return nil, err
	}
	var dto listDTO[eventDTO]
	if err := c.get(ctx, "events", path, &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, func(x eventDTO) deploy.Event {
		return deploy.Event{Type: x.Type, Reason: x.Reason, Object: x.Object, Message: x.Message,
			LastSeen: parseTime(x.Time), Count: int(x.Count)}
	}), nil
}

// Diagnosis reads GET P/environments/{env}/diagnose (routes.go:1151), a dataDTO[diagnoseDTO],
// every string verbatim. Source "rules+ai" says a model was involved (handlers/diagnose.go:78).
func (c *Client) Diagnosis(ctx context.Context, p deploy.ProjectRef, env string) (deploy.Diagnosis, error) {
	path, err := envPath("diagnosis", p, env, "diagnose")
	if err != nil {
		return deploy.Diagnosis{}, err
	}
	var dto dataDTO[diagnoseDTO]
	if err := c.get(ctx, "diagnosis", path, &dto); err != nil {
		return deploy.Diagnosis{}, err
	}
	d := dto.Data
	return deploy.Diagnosis{Health: d.Health, Summary: d.Summary, Source: d.Source,
		Findings: mapSlice(d.Findings, func(f findingDTO) deploy.Finding {
			return deploy.Finding{Problem: f.Problem, LikelyCause: f.LikelyCause, SuggestedFix: f.SuggestedFix,
				Evidence: f.Evidence, Severity: f.Severity, Owner: f.Owner}
		})}, nil
}

// SameEnvironment implements deploy.Reader.SameEnvironment: exact equality plus the ONE
// equivalence Panacea itself applies — tenantNamespace treats "prod" and "production" as the
// base namespace (clients/kubernetes.go:4210-4217). deploywatch calls it through the interface,
// so the orchestration layer never imports this package.
// tenantNamespace sends "" to the base namespace as well (:4213), and that is NOT adopted: "" is
// a legacy run with no environment label, which Panacea's own environments list attributes by
// its branch label (:5396-5415), not to production. So "" is the same as no other name; task 08
// rule 1b joins it by label (review M5).
func (c *Client) SameEnvironment(a, b string) bool {
	return a == b || (isBaseEnvironment(a) && isBaseEnvironment(b))
}

func isBaseEnvironment(env string) bool { return env == "prod" || env == "production" }
