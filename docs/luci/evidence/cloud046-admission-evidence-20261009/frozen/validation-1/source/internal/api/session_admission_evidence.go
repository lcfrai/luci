package api

import (
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Only the single-session GET uses this envelope. Lists, mutations and other
// session projections use sessionJSON, which cannot expose host telemetry.
type sessionDetailJSON struct {
	sessionJSON
	StartAdmissionEvidence *registry.StartAdmissionEvidence `json:"startAdmissionEvidence,omitempty"`
}

func sessionAdmissionEvidence(r *http.Request, row registry.Session) *registry.StartAdmissionEvidence {
	if !product.CanInspectHostResources(r) {
		return nil
	}
	if actor, _, ok := management.ActorFromContext(r.Context()); ok && actor.Disabled {
		return nil
	}
	if row.Status != registry.StatusExited || row.SandboxRef != "" || !row.StartAdmissionEvidence.ValidFor(row.StartFailure) {
		return nil
	}
	evidence := row.StartAdmissionEvidence
	return &evidence
}
