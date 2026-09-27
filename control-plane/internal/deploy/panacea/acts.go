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

// notStartedSentence is Cancel's own sentence for a run Panacea would not cancel because it had
// not started (review t04 I2): plain text, no internal detail, and it says the build goes on.
const notStartedSentence = "The build had not started yet, so Panacea could not stop it, and it will go ahead. " +
	"Try Cancel again in a few seconds."

// Ship posts P/deploy (routes.go:1286), a shipRequestDTO, and reads TriggerDeploy's 202
// dataDTO[shipResponseDTO] (handlers/projects.go:2998-3005). It refuses before any request a
// commit that is not 40 lowercase hex (a symbolic ref would ship whatever the branch holds at
// clone time) and an empty branch. noCache is sent only as true: nil and false both leave
// Panacea's default (models/models.go:2349-2352). Its 402s, the budget and risk gates
// (projects.go:2856, :2916-2918), are KindConflict with the sentence by the status table
// (errors.go); its 500 (:2991) is KindUnavailable with none. Its 404 answers ANY error of
// K8s.GetProject, a live namespace Get, as "project not found: "+err.Error()
// (projects.go:2953-2957, clients/kubernetes.go:1528-1533), so an API-server timeout arrives as
// a 404 holding an internal address: the kind stays KindNotFound and the sentence is the
// connector's own, Panacea's logged at warn (review t04 M2).
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
		return deploy.ShipResult{}, c.ownNotFound(op, err, "Panacea did not find the project "+string(p)+".",
			"project", string(p), "branch", req.Branch)
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
// dataDTO[cancelResponseDTO] (handlers/projects.go:3669-3687): "cancelled" is Cancelled with
// Panacea's sentence, and any word this code does not know is KindUnavailable with the 200
// received. "already_completed" is NOT taken at its word: CancelDeploy cancels only a Pending or
// Running run (:3669) and pipelineRunStatusFromUnstructured starts every run at "Unknown" until
// Tekton writes a condition (clients/pipeline.go:1097-1100), so a Cancel in a run's first seconds
// stops nothing and says already_completed while the build goes on to deploy. One re-read of the
// run confirms it (cancelAlreadyCompleted). Its 404 (:3660, :3664) answers ANY error of a live
// GetPipelineRun as "pipeline run not found: "+err.Error() (:3657-3661,
// clients/pipeline.go:1064-1068), so the kind stays KindNotFound and the sentence is the
// connector's own, Panacea's logged at warn (review t04 M2).
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
		return deploy.CancelResult{}, c.ownNotFound(op, err, "Panacea did not find run "+run+" in this project.",
			"project", string(p), "run", run)
	}
	switch dto.Data.Status {
	case "cancelled":
		return deploy.CancelResult{Outcome: deploy.Cancelled, Message: dto.Data.Message}, nil
	case "already_completed":
		return c.cancelAlreadyCompleted(ctx, p, run, dto.Data.Message)
	}
	c.log.Warn("panacea answered a cancel with a word this code does not know", "op", op, "project", string(p),
		"run", run, "word", dto.Data.Status, "status", http.StatusOK)
	return deploy.CancelResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
		Cause: fmt.Errorf("panacea: POST %s/pipelines/%s/cancel answered 200 with status %q", P, r, dto.Data.Status)}
}

// cancelAlreadyCompleted confirms Panacea's already_completed with ONE re-read of the run through
// the connector's own Run (GET P/pipelines/{run}, a retried READ, so re-reading is safe where
// re-sending the cancel would not be). It reads structure, never Panacea's sentence, so plan
// 7P-12's single parsed sentence stays single:
//
//	terminal (Succeeded, Failed, historical) → AlreadyFinished with Panacea's sentence, no error
//	pending or running                       → KindConflict, Status 200, notStartedSentence
//	a state word this code does not know     → KindUnavailable, Status 200
//	the re-read failed                       → KindUnavailable, Status 200
//
// The Status is the cancel's own 200 throughout: that is the answer this error is about. Every
// outcome but the terminal one is logged at warn, naming the run: the build is still going and
// nothing stopped it (review t04 I2).
func (c *Client) cancelAlreadyCompleted(ctx context.Context, p deploy.ProjectRef, run, sentence string) (deploy.CancelResult, error) {
	const op = "cancel"
	rp, err := c.Run(ctx, p, run)
	if err != nil {
		c.log.Warn("panacea refused a cancel as already finished and the run could not be re-read", "op", op,
			"project", string(p), "run", run, "status", http.StatusOK, "cause", err.Error())
		return deploy.CancelResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
			Cause: fmt.Errorf("panacea: a cancel of %q answered already_completed and the run could not be re-read: %w", run, err)}
	}
	if rp.Build.Terminal() {
		return deploy.CancelResult{Outcome: deploy.AlreadyFinished, Message: sentence}, nil
	}
	if rp.Build == deploy.BuildUnknown {
		c.log.Warn("panacea refused a cancel as already finished for a run in a state this code does not know", "op", op,
			"project", string(p), "run", run, "state", rp.BuildRaw, "status", http.StatusOK)
		return deploy.CancelResult{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
			Cause: fmt.Errorf("panacea: a cancel of %q answered already_completed for a run reading %q", run, rp.BuildRaw)}
	}
	c.log.Warn("panacea refused a cancel of a run that has not finished; nothing was stopped", "op", op,
		"project", string(p), "run", run, "state", rp.BuildRaw, "status", http.StatusOK)
	return deploy.CancelResult{}, &deploy.Error{Kind: deploy.KindConflict, Op: op, Status: http.StatusOK,
		Message: notStartedSentence,
		Cause:   fmt.Errorf("panacea: a cancel of %q answered already_completed while the run reads %q", run, rp.BuildRaw)}
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
