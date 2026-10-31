package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
)

const kubernetesOperationLimit = 512 << 10

type kubernetesOperationEnvelope struct {
	Version      int                      `json:"version"`
	AccountScope string                   `json:"accountScope"`
	ProjectScope string                   `json:"projectScope"`
	Operation    deploymentkube.Operation `json:"operation"`
}

type kubernetesOperationJournal struct{ server *Server }

// NewKubernetesDeploymentConnector is an explicit integration seam, not a public
// route or an enabled target. The controller must supply a private target/client
// resolver and a fresh original-actor/source/check/Deploy/effect-cost authorizer.
// No ambient kubeconfig, cluster credential, provider or subprocess is opened.
func (s *Server) NewKubernetesDeploymentConnector(resolve deploymentkube.Resolver, authorize deploymentkube.Authorizer) *deploymentkube.Connector {
	return deploymentkube.New(resolve, authorize, &kubernetesOperationJournal{server: s})
}

func (j *kubernetesOperationJournal) path(identity deploymentkube.Identity) (string, kubernetesOperationEnvelope, error) {
	p, err := j.server.resolveProject(identity.ProjectID)
	if err != nil {
		return "", kubernetesOperationEnvelope{}, err
	}
	j.server.mu.RLock()
	scope, err := j.server.approvalScopes(p.Path, p.ID)
	j.server.mu.RUnlock()
	if err != nil {
		return "", kubernetesOperationEnvelope{}, err
	}
	// Actor/team are checked inside the retained record, not in the filename:
	// changing caller identity must never create a second copy of an operation.
	key := reviewDigest([]string{identity.ProjectID, identity.TargetID, identity.OperationID})
	path, err := guardedProjectPath(j.server.stateDir, "kubernetes-workload-"+key+".json", true)
	return path, kubernetesOperationEnvelope{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope}, err
}

func readKubernetesOperation(path string, scope kubernetesOperationEnvelope, id deploymentkube.Identity) (deploymentkube.Operation, error) {
	var record kubernetesOperationEnvelope
	if err := externalReadJSON(path, kubernetesOperationLimit, &record); err != nil {
		return deploymentkube.Operation{}, err
	}
	if record.Version != 1 || record.AccountScope != scope.AccountScope || record.ProjectScope != scope.ProjectScope || record.Operation.Request.Identity != id || deploymentkube.ValidateOperation(record.Operation) != nil {
		return deploymentkube.Operation{}, deploymentkube.ErrRefused
	}
	return record.Operation, nil
}

func writeKubernetesOperation(path string, scope kubernetesOperationEnvelope, operation deploymentkube.Operation) error {
	if deploymentkube.ValidateOperation(operation) != nil {
		return deploymentkube.ErrRefused
	}
	scope.Operation = operation
	raw, err := json.MarshalIndent(scope, "", "  ")
	if err != nil || len(raw)+1 > kubernetesOperationLimit {
		return deploymentkube.ErrRefused
	}
	return durableRunJSON(path, scope)
}

func (j *kubernetesOperationJournal) Reserve(ctx context.Context, operation deploymentkube.Operation) (deploymentkube.Operation, bool, error) {
	if ctx.Err() != nil || operation.Revision != 1 || operation.State != "reserved" || deploymentkube.ValidateOperation(operation) != nil {
		return deploymentkube.Operation{}, false, deploymentkube.ErrRefused
	}
	path, scope, err := j.path(operation.Request.Identity)
	if err != nil {
		return deploymentkube.Operation{}, false, err
	}
	j.server.mu.Lock()
	defer j.server.mu.Unlock()
	saved, err := readKubernetesOperation(path, scope, operation.Request.Identity)
	if err == nil {
		return saved, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return deploymentkube.Operation{}, false, err
	}
	if ctx.Err() != nil {
		return deploymentkube.Operation{}, false, ctx.Err()
	}
	if err = writeKubernetesOperation(path, scope, operation); err != nil {
		return deploymentkube.Operation{}, false, err
	}
	return operation, true, nil
}

func (j *kubernetesOperationJournal) Load(ctx context.Context, id deploymentkube.Identity) (deploymentkube.Operation, error) {
	if ctx.Err() != nil {
		return deploymentkube.Operation{}, ctx.Err()
	}
	path, scope, err := j.path(id)
	if err != nil {
		return deploymentkube.Operation{}, err
	}
	j.server.mu.RLock()
	defer j.server.mu.RUnlock()
	return readKubernetesOperation(path, scope, id)
}

func (j *kubernetesOperationJournal) CompareAndSwap(ctx context.Context, operation deploymentkube.Operation, expected int) error {
	if ctx.Err() != nil || operation.Revision != expected+1 || deploymentkube.ValidateOperation(operation) != nil {
		return deploymentkube.ErrRefused
	}
	path, scope, err := j.path(operation.Request.Identity)
	if err != nil {
		return err
	}
	j.server.mu.Lock()
	defer j.server.mu.Unlock()
	before, err := readKubernetesOperation(path, scope, operation.Request.Identity)
	if err != nil || before.Revision != expected || before.RequestDigest != operation.RequestDigest || before.NamespaceUID != operation.NamespaceUID || len(before.Resources) != len(operation.Resources) {
		return deploymentkube.ErrRefused
	}
	for index, old := range before.Resources {
		row := operation.Resources[index]
		if old.ExpectedResource != row.ExpectedResource || old.DesiredSHA256 != row.DesiredSHA256 || old.Attempted && !row.Attempted || old.ObservedUID != "" && old.ObservedUID != row.ObservedUID {
			return deploymentkube.ErrRefused
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return writeKubernetesOperation(path, scope, operation)
}
