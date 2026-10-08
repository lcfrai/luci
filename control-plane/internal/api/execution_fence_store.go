package api

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Permanent host slots include interrupted intents. A retry keeps the same
// slot and identity; completed fences are never TTL-pruned to admit another.
type executionFenceRecord struct {
	Scope       string                             `json:"scope"`
	ProjectID   string                             `json:"projectId"`
	RunID       string                             `json:"runId"`
	OperationID string                             `json:"operationId"`
	ChildID     string                             `json:"childId"`
	QueueID     string                             `json:"queueId,omitempty"`
	Intent      product.ProjectSwarmRecoveryIntent `json:"intent"`
	Proof       product.ProjectSwarmRecoveryProof  `json:"proof,omitzero"`
	State       string                             `json:"state"`
	CreatedAt   time.Time                          `json:"createdAt"`
	UpdatedAt   time.Time                          `json:"updatedAt"`
}

func exactHex(value string, bytes int) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == bytes && value == strings.ToLower(value)
}
func boundedFenceText(value string, max int) bool {
	return value != "" && len(value) <= max && !strings.ContainsAny(value, "\x00\r\n")
}
func validExecutionFenceLease(lease executionLease) bool {
	if lease.FenceProtocol == 0 {
		return lease.FenceEngineID == "" && lease.FenceInstanceID == "" && lease.FenceIntentID == "" && lease.State != "fencing"
	}
	if !validFenceIdentity(backend.LaunchFenceIdentity{Protocol: lease.FenceProtocol, EngineID: lease.FenceEngineID, InstanceID: lease.FenceInstanceID}) || lease.LaunchGuard != 1 || lease.State == "reserved" || lease.State == "replacing" {
		return false
	}
	return lease.State == "fencing" && exactHex(lease.FenceIntentID, 16) || lease.State != "fencing" && lease.FenceIntentID == ""
}
func fenceRecordRequest(record executionFenceRecord) product.ProjectSwarmFenceRecoveryRequest {
	return product.ProjectSwarmFenceRecoveryRequest{ProjectSwarmRecoveryRequest: product.ProjectSwarmRecoveryRequest{ProjectID: record.ProjectID, RunID: record.RunID, OperationID: record.OperationID, ChildID: record.ChildID, QueueID: record.QueueID, UpdatedAt: record.Intent.OriginalUpdatedAt}, RequestDigest: record.Intent.RequestDigest, ImplementationDigest: record.Intent.ImplementationDigest}
}
func fenceRecordMatches(record executionFenceRecord, req product.ProjectSwarmFenceRecoveryRequest) bool {
	return record.ProjectID == req.ProjectID && record.RunID == req.RunID && record.OperationID == req.OperationID && record.ChildID == req.ChildID && record.QueueID == req.QueueID && (req.Intent == nil || record.Intent.OriginalUpdatedAt == req.UpdatedAt) && record.Intent.RequestDigest == req.RequestDigest && record.Intent.ImplementationDigest == req.ImplementationDigest
}
func fenceProofMatches(proof product.ProjectSwarmRecoveryProof, record executionFenceRecord) bool {
	request, fence := record.Intent.FenceRequest, proof.FenceProof
	ack, e1 := time.Parse(time.RFC3339Nano, fence.AcknowledgedAt)
	requested, e2 := time.Parse(time.RFC3339Nano, record.Intent.RequestedAt)
	return e1 == nil && e2 == nil && !ack.Before(requested) && product.ValidProjectSwarmRecoveryProof(proof, record.ChildID) && proof.Method == product.ProjectSwarmFenceRecoveryMethod && proof.FinalizedAt == "" && proof.LeaseID == record.Intent.LeaseID && proof.QueueID == record.QueueID &&
		fence.Protocol == record.Intent.Protocol && fence.EngineID == request.EngineID && fence.InstanceID == request.InstanceID && fence.SandboxID == request.SandboxID && fence.SessionID == request.SessionID && fence.IntentID == request.IntentID && fence.ScopeDigest == request.ScopeDigest && fence.OperationDigest == request.OperationDigest
}
func validExecutionFenceRecord(record executionFenceRecord) bool {
	intent, request := record.Intent, record.Intent.FenceRequest
	if !exactHex(record.Scope, 32) || !boundedFenceText(record.ProjectID, 200) || !boundedFenceText(record.RunID, 200) || !boundedFenceText(record.OperationID, 200) || !boundedFenceText(record.ChildID, 200) || record.QueueID != "" && !boundedFenceText(record.QueueID, 200) || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		return false
	}
	requested, e1 := time.Parse(time.RFC3339Nano, intent.RequestedAt)
	original, e2 := time.Parse(time.RFC3339Nano, intent.OriginalUpdatedAt)
	if e1 != nil || e2 != nil || requested.Before(original) || !record.CreatedAt.Equal(requested) || !exactHex(intent.ID, 16) || !boundedFenceText(intent.RequestedBy, 200) || !boundedFenceText(intent.LeaseID, 200) || intent.QueueID != record.QueueID || record.QueueID != "" && intent.LeaseID != record.QueueID || !exactHex(intent.RequestDigest, 32) || !exactHex(intent.ImplementationDigest, 32) || intent.ScopeDigest != record.Scope || request.ScopeDigest != record.Scope || request.IntentID != intent.ID || request.SessionID != record.ChildID || request.SandboxID != record.ChildID || request.OperationDigest != product.ProjectSwarmFenceOperationDigest(fenceRecordRequest(record)) || !validFenceIdentity(backend.LaunchFenceIdentity{Protocol: intent.Protocol, EngineID: request.EngineID, InstanceID: request.InstanceID}) || request.ExpectedOriginalRef != "" && !exactHex(request.ExpectedOriginalRef, 32) {
		return false
	}
	switch record.State {
	case "intent":
		return record.Proof == (product.ProjectSwarmRecoveryProof{})
	case "proved", "retired":
		return fenceProofMatches(record.Proof, record)
	default:
		return false
	}
}
func validateExecutionFenceRecords(ledger executionLeaseLedger) error {
	if len(ledger.Fences)+len(ledger.CheckFences) > backend.MaxLaunchFences {
		return errors.New("execution fence history exceeds its permanent host limit")
	}
	seen, children := map[string]bool{}, map[string]bool{}
	for _, record := range ledger.Fences {
		key := record.Scope + ":" + record.ChildID
		if !validExecutionFenceRecord(record) || seen[record.Intent.ID] || children[key] {
			return errors.New("execution fence ownership cannot be verified")
		}
		seen[record.Intent.ID], children[key] = true, true
		matches := 0
		for _, lease := range ledger.Leases {
			if lease.Scope != record.Scope || lease.ID != record.Intent.LeaseID {
				continue
			}
			if record.State == "retired" || lease.State != "fencing" || lease.FenceIntentID != record.Intent.ID || lease.ProjectID != record.ProjectID || lease.SessionID != record.ChildID || lease.SandboxID != record.ChildID || lease.FenceEngineID != record.Intent.FenceRequest.EngineID || lease.FenceInstanceID != record.Intent.FenceRequest.InstanceID {
				return errors.New("execution fence lease changed")
			}
			matches++
		}
		if record.State != "retired" && matches != 1 {
			return errors.New("execution fence requires its exact retained reservation")
		}
		if record.State == "retired" {
			retired := 0
			for _, receipt := range ledger.Retirements {
				if receipt.Scope == record.Scope && receipt.LeaseID == record.Intent.LeaseID && retiredExecutionMatches(receipt, fenceRecordRequest(record).ProjectSwarmRecoveryRequest) && receipt.Proof == record.Proof {
					retired++
				}
			}
			if retired != 1 {
				return errors.New("execution fence retirement is incomplete")
			}
		}
	}
	for _, lease := range ledger.Leases {
		if lease.State == "fencing" && !children[lease.Scope+":"+lease.SessionID] {
			return errors.New("execution fence intent is missing")
		}
	}
	for _, receipt := range ledger.Retirements {
		if receipt.Proof.Method == product.ProjectSwarmFenceRecoveryMethod && !children[receipt.Scope+":"+receipt.ChildID] {
			return errors.New("execution fence retirement has no permanent marker record")
		}
	}
	return nil
}

func (g *executionAdmission) readFenceLedger() (executionLeaseLedger, error) {
	if !filepath.IsAbs(g.path) {
		return executionLeaseLedger{}, errors.New("execution state path is not absolute")
	}
	if err := codexRegularPath(filepath.VolumeName(g.path)+string(filepath.Separator), g.path); err != nil {
		return executionLeaseLedger{}, err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), g.path); err != nil {
		return executionLeaseLedger{}, err
	}
	data, err := codexRead(filepath.Dir(g.path), g.path, 2<<20)
	if err != nil {
		return executionLeaseLedger{}, err
	}
	return decodeExecutionLedger(data)
}
