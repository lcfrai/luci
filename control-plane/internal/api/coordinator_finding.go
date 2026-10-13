package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Use the ordinary orchestrator admission and assistance job, including its
// exact idempotency key, private context snapshot and no-replay recovery. A
// finding is not a second inference engine or permission to apply its answer.
func (a *API) dispatchCoordinatorFinding(ctx context.Context, ref management.ExecutionActorRef, finding product.CoordinatorFindingDispatch) (string, error) {
	fresh, err := a.reauthorizeCoordinator(ctx, ref, finding.Harness, finding.AgentConnectionID != nil && *finding.AgentConnectionID != "")
	if err != nil || ref.UserID != finding.RecipientID || ref.ProjectID != finding.ProjectID {
		return "", management.ErrExecutionAuthority
	}
	if !a.deps.Product.CoordinatorFindingDispatchCurrent(finding.ProjectID, finding.RecipientID, finding.FindingID, finding.RouteRevision) {
		return "", errors.New("the automatic finding route or policy changed before model admission")
	}
	input, err := json.Marshal(orchestratorRequest{ExpectedRevision: finding.ExpectedRevision, ExpectedContextDigest: finding.ExpectedContextDigest, IdempotencyKey: finding.RequestKey, Message: finding.Message, Harness: finding.Harness, Model: finding.Model, AgentConnectionID: finding.AgentConnectionID})
	if err != nil {
		return "", err
	}
	req := httptest.NewRequestWithContext(fresh, "POST", "/v1/product/projects/"+finding.ProjectID+"/assist/orchestrator", bytes.NewReader(input))
	req.SetPathValue("id", finding.ProjectID)
	recorder := httptest.NewRecorder()
	a.handleOrchestratorCreate(recorder, req)
	if recorder.Code != 202 && recorder.Code != 200 {
		return "", errors.New("the exact automatic conversation turn was refused or its result is uncertain; inspect the retained finding and assistant history before another request")
	}
	var job assistance.Job
	if err = json.Unmarshal(recorder.Body.Bytes(), &job); err != nil || job.ProjectID != finding.ProjectID || job.Kind != "orchestrator" || job.OrchestratorTurnID == "" {
		return "", errors.New("the automatic conversation response could not be bound to its durable turn")
	}
	return job.OrchestratorTurnID, nil
}
