package panacea

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 04: deploy.Actor. Every act is a write, so do sends it ONCE and never retries it (task
// 02): a retried deploy is a second deploy. Each accepts only the status its handler writes on
// success, and an answer that does not hold what the act needs is KindUnavailable with the
// status received, logged, never a guess (re-review M-4).

var _ deploy.Actor = (*Client)(nil)

// noRunNamed is Ship's log line, and its error's cause, for a 202 with no run name. Task 09
// step 6's "Accepted, no run" abandons the reservation on it.
const noRunNamed = "Panacea accepted the deploy but did not name the run"

// Ship posts P/deploy (routes.go:1286), a shipRequestDTO, and reads TriggerDeploy's 202
// dataDTO[shipResponseDTO] (handlers/projects.go:2998-3005). It refuses before any request a
// commit that is not 40 lowercase hex (a symbolic ref would ship whatever the branch holds at
// clone time) and an empty branch. noCache is sent only as true: nil and false both leave
// Panacea's default (models/models.go:2349-2352). Its 402s, the budget and risk gates
// (projects.go:2856, :2916-2918), are KindConflict with the sentence by the status table
// (errors.go); its 500 (:2991) is KindUnavailable with none.
func (c *Client) Ship(ctx context.Context, p deploy.ProjectRef, req deploy.ShipRequest) (deploy.ShipResult, error) {
	const op = "ship"
	if req.Branch == "" || !deploy.ValidCommit(req.Commit) {
		return deploy.ShipResult{}, &deploy.Error{Kind: deploy.KindRejected, Op: op,
			Cause: fmt.Errorf("panacea: a Ship names a branch and a 40-hex commit, not branch %q, commit %q", req.Branch, req.Commit)}
	}
	P, err := projectPath(op, p)
	if err != nil {
		return deploy.ShipResult{}, err
	}
	body := shipRequestDTO{Branch: req.Branch, Commit: req.Commit}
	if req.NoCache != nil && *req.NoCache {
		yes := true
		body.NoCache = &yes
	}
	var dto dataDTO[shipResponseDTO]
	if err := c.do(ctx, op, http.MethodPost, P+"/deploy", body, &dto, http.StatusAccepted); err != nil {
		return deploy.ShipResult{}, err
	}
	if dto.Data.PipelineRun == "" {
		c.log.Warn(noRunNamed, "op", op, "project", string(p), "branch", req.Branch, "commit", req.Commit,
			"status", http.StatusAccepted)
		return deploy.ShipResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusAccepted,
			Cause: fmt.Errorf("panacea: POST %s/deploy answered 202 with no data.pipelineRun: %s", P, noRunNamed)}
	}
	return deploy.ShipResult{Run: dto.Data.PipelineRun}, nil
}

// Cancel posts P/pipelines/{run}/cancel (routes.go:1291) and reads CancelDeploy's
// dataDTO[cancelResponseDTO] (handlers/projects.go:3669-3687): "cancelled" is Cancelled,
// "already_completed" is AlreadyFinished, not an error, each with Panacea's sentence. CancelDeploy
// cancels only a Pending or Running run (:3669), so a run with no condition yet ("Unknown",
// clients/pipeline.go GetPipelineRun) also answers already_completed. Any other word is
// KindUnavailable with the 200 received. Its 404 (:3660, :3664) keeps Panacea's sentence.
func (c *Client) Cancel(ctx context.Context, p deploy.ProjectRef, run string) (deploy.CancelResult, error) {
	const op = "cancel"
	P, err := projectPath(op, p)
	if err != nil {
		return deploy.CancelResult{}, err
	}
	r, err := segment(op, "run", run)
	if err != nil {
		return deploy.CancelResult{}, err
	}
	var dto dataDTO[cancelResponseDTO]
	if err := c.do(ctx, op, http.MethodPost, P+"/pipelines/"+r+"/cancel", nil, &dto, http.StatusOK); err != nil {
		return deploy.CancelResult{}, err
	}
	switch dto.Data.Status {
	case "cancelled":
		return deploy.CancelResult{Outcome: deploy.Cancelled, Message: dto.Data.Message}, nil
	case "already_completed":
		return deploy.CancelResult{Outcome: deploy.AlreadyFinished, Message: dto.Data.Message}, nil
	}
	c.log.Warn("panacea answered a cancel with a word this code does not know", "op", op, "project", string(p),
		"run", run, "word", dto.Data.Status, "status", http.StatusOK)
	return deploy.CancelResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
		Cause: fmt.Errorf("panacea: POST %s/pipelines/%s/cancel answered 200 with status %q", P, r, dto.Data.Status)}
}

// Rollback posts P/deployments/{id}/rollback (routes.go:1026) and reads Rollback's
// dataDTO[rollbackResponseDTO] (handlers/projects.go:1636-1639). It creates no run and no
// deployment (7P-4): SourceDeploymentID is data.id, the deployment rolled back TO
// (clients/kubernetes.go:5982, :6030), so an answer for any other id is KindUnavailable with the
// 200 received and a log line, because 7P-4's premise no longer holds and task 10's Hub confirm
// must not guess. Image is the target's (:1619). Warmth is "warm" or "cold"; any other word is
// WarmthUnknown. ColdStartSeconds is carried for cold only: Panacea writes 0 for warm
// (:5963-5971). Its 409 (:1603-1604), 400 (:1575) and 404 (:1570) keep Panacea's sentence.
func (c *Client) Rollback(ctx context.Context, p deploy.ProjectRef, deploymentID string) (deploy.RollbackResult, error) {
	const op = "rollback"
	P, err := projectPath(op, p)
	if err != nil {
		return deploy.RollbackResult{}, err
	}
	id, err := segment(op, "deployment", deploymentID)
	if err != nil {
		return deploy.RollbackResult{}, err
	}
	var dto dataDTO[rollbackResponseDTO]
	if err := c.do(ctx, op, http.MethodPost, P+"/deployments/"+id+"/rollback", nil, &dto, http.StatusOK); err != nil {
		return deploy.RollbackResult{}, err
	}
	d := dto.Data
	if d.ID != deploymentID {
		c.log.Warn("panacea answered a rollback for another deployment", "op", op, "project", string(p),
			"asked", deploymentID, "answered", d.ID, "status", http.StatusOK)
		return deploy.RollbackResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
			Cause: fmt.Errorf("panacea: POST %s/deployments/%s/rollback answered 200 for deployment %q", P, id, d.ID)}
	}
	res := deploy.RollbackResult{SourceDeploymentID: deploymentID, Image: d.Image, Warmth: warmthOf(d.RollbackWarmth)}
	if res.Warmth == deploy.WarmthCold {
		res.ColdStartSeconds = max(d.ColdStartSeconds, 0)
	}
	return res, nil
}

// warmthOf reads models.Deployment.RollbackWarmth (models/models.go:695-704): "warm" or "cold".
func warmthOf(word string) deploy.Warmth {
	switch word {
	case "warm":
		return deploy.WarmthWarm
	case "cold":
		return deploy.WarmthCold
	}
	return deploy.WarmthUnknown
}
