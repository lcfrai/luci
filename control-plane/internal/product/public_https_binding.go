package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"time"
)

// Commissioning owns the proxy; ordinary app delivery only reuses its exact
// retained endpoint. It neither creates an Ingress nor copies ACME credentials.
func (s *Server) managedPublicHTTPSRow(ctx context.Context, actor management.ExecutionActorRef, blueprint deploymentplan.Blueprint, target deploymentkube.Target, ready bool) (storedPublicHTTPS, error) {
	r := blueprint.PublicRoute
	if r == nil || r.Profile != deploymentplan.ManagedPublicRouteProfile {
		return storedPublicHTTPS{}, ErrPublicHTTPS
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return storedPublicHTTPS{}, e
	}
	_, row, e := s.readPublicHTTPS(p, r.ManagedHTTPSID)
	if e != nil || row.Actor != actor {
		return row, ErrPublicHTTPS
	}
	if ready {
		if row.State != "ready" || !row.HTTPSVerified || !row.DNSObserved || row.RetireAttempted || row.DesiredReplicas != 1 {
			return row, ErrPublicHTTPS
		}
		expiry, e := time.Parse(time.RFC3339Nano, row.CertificateNotAfter)
		if e != nil || !time.Now().Before(expiry) {
			return row, ErrPublicHTTPS
		}
	}
	d := row.Plan.Destination
	if d.TargetID != target.ID || d.TargetFingerprint != target.Fingerprint || d.Namespace != target.Namespace || d.NamespaceUID != target.NamespaceUID || d.ServiceID != r.ServiceID || row.Plan.Hostname != r.Hostname || row.Plan.Path != r.Path || row.Plan.PathType != r.PathType {
		return row, ErrPublicHTTPS
	}
	matched := false
	for _, service := range blueprint.Services {
		if service.ID == d.ServiceID && service.Kind == "app" && service.Port == d.ServicePort {
			matched = true
		}
	}
	if !matched || s.kubernetesAuthority(ctx, actor, false, ready, 0) != nil {
		return row, ErrPublicHTTPS
	}
	return row, nil
}
func (s *Server) publicRouteForReview(ctx context.Context, actor management.ExecutionActorRef, target deploymentkube.Target, blueprint deploymentplan.Blueprint) (*deploymentkube.PublicRouteBinding, error) {
	if blueprint.PublicRoute == nil || blueprint.PublicRoute.Profile != deploymentplan.ManagedPublicRouteProfile {
		return deploymentkube.ReadPublicRouteBinding(ctx, target, blueprint, func(call context.Context) error { return s.kubernetesAuthority(call, actor, false, true, 0) })
	}
	row, e := s.managedPublicHTTPSRow(ctx, actor, blueprint, target, true)
	if e != nil {
		return nil, e
	}
	controller, lb := publicHTTPSResource(&row, "Deployment"), publicHTTPSResource(&row, "Service")
	if controller == nil || lb == nil || controller.UID == "" || lb.UID == "" {
		return nil, ErrPublicHTTPS
	}
	binding, e := deploymentkube.NewManagedPublicRouteBinding(blueprint, target, deploymentkube.ManagedPublicHTTPSBinding{CommissionID: row.ID, ControllerUID: controller.UID, LoadBalancerUID: lb.UID, AppServiceUID: row.Plan.Destination.ServiceUID, Address: row.Address, Spec: publicHTTPSBindingSpec(row)})
	if e != nil {
		return nil, e
	}
	request := deploymentkube.Request{Blueprint: blueprint, Identity: deploymentkube.Identity{TargetFingerprint: target.Fingerprint}, PublicRouteBinding: binding}
	e = deploymentkube.ReadManagedPublicHTTPSReferences(ctx, target, request, func(call context.Context) error {
		_, e := s.managedPublicHTTPSRow(call, actor, blueprint, target, true)
		return e
	})
	return binding, e
}
