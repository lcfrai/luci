package deploymentkube

import (
	"context"
	"encoding/json"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Derived by the server from the retained original commissioning journal,
// never accepted as a browser's assertion of ownership or deployment authority.
type ManagedPublicHTTPSBinding struct {
	CommissionID    string                  `json:"commissionId"`
	ControllerUID   string                  `json:"controllerUid"`
	LoadBalancerUID string                  `json:"loadBalancerUid"`
	AppServiceUID   string                  `json:"appServiceUid"`
	Address         string                  `json:"address"`
	Spec            PublicHTTPSResourceSpec `json:"spec"`
}

func NewManagedPublicRouteBinding(blueprint deploymentplan.Blueprint, target Target, m ManagedPublicHTTPSBinding) (*PublicRouteBinding, error) {
	if blueprint.PublicRoute == nil {
		return nil, ErrRefused
	}
	b := &PublicRouteBinding{Version: 2, RouteDigest: digest(*blueprint.PublicRoute), NamespaceUID: target.NamespaceUID, TargetFingerprint: target.Fingerprint, Managed: &m}
	b.MetadataDigest = publicBindingDigest(*b)
	if validateManagedPublicRouteBinding(Request{Blueprint: blueprint, Identity: Identity{TargetFingerprint: target.Fingerprint}, PublicRouteBinding: b}) != nil {
		return nil, ErrRefused
	}
	return b, nil
}
func validateManagedPublicRouteBinding(request Request) error {
	r, b := request.Blueprint.PublicRoute, request.PublicRouteBinding
	if r == nil || r.Profile != deploymentplan.ManagedPublicRouteProfile || b == nil || b.Version != 2 || b.Managed == nil || b.MetadataDigest != publicBindingDigest(*b) || b.RouteDigest != digest(*r) || b.NamespaceUID == "" || b.TargetFingerprint != request.Identity.TargetFingerprint || b.ClassName != "" || b.ClassUID != "" || b.ClassVersion != "" || b.Controller != "" || b.TLSSecretName != "" || b.TLSSecretUID != "" || b.TLSSecretVersion != "" {
		return ErrRefused
	}
	m := b.Managed
	s := m.Spec
	if m.CommissionID != r.ManagedHTTPSID || s.ID != m.CommissionID || s.ProjectID != request.Blueprint.ProjectID || s.Namespace != request.Blueprint.Infrastructure.Namespace || s.Hostname != r.Hostname || s.Path != r.Path || s.PathType != r.PathType || s.ServiceName != deploymentplan.ResourceName(request.Blueprint.ProjectID, r.ServiceID) || s.ServiceUID != m.AppServiceUID || m.ControllerUID == "" || m.LoadBalancerUID == "" || m.Address == "" || s.Replicas != 1 {
		return ErrRefused
	}
	found := false
	for _, app := range request.Blueprint.Services {
		if app.ID == r.ServiceID && app.Kind == "app" && app.Port == s.ServicePort {
			found = true
		}
	}
	if !found {
		return ErrRefused
	}
	if _, e := PublicHTTPSObjects(s, map[string][]byte{"tenant-id": []byte("validation-only"), "client-id": []byte("validation-only"), "client-secret": []byte("validation-only")}); e != nil {
		return e
	}
	return nil
}
func ReadManagedPublicHTTPSReferences(ctx context.Context, target Target, request Request, authorize func(context.Context) error) error {
	if authorize == nil || validateManagedPublicRouteBinding(request) != nil || authorize(ctx) != nil || PublicHTTPSNamespace(ctx, target) != nil {
		return ErrRefused
	}
	b := request.PublicRouteBinding
	m := b.Managed
	if target.Fingerprint != b.TargetFingerprint || target.NamespaceUID != b.NamespaceUID {
		return ErrRefused
	}
	objects, e := PublicHTTPSObjects(m.Spec, map[string][]byte{"tenant-id": []byte("validation-only"), "client-id": []byte("validation-only"), "client-secret": []byte("validation-only")})
	if e != nil {
		return e
	}
	for _, object := range objects {
		if object.Kind != "Deployment" && object.Kind != "Service" {
			continue
		}
		if authorize(ctx) != nil {
			return ErrRefused
		}
		var got metav1.Object
		if object.Kind == "Deployment" {
			var row appsv1.Deployment
			if PublicHTTPSResourceRPC(ctx, target, "GET", object.Kind, object.Name, nil, &row) != nil || !PublicHTTPSOwned(m.Spec, &row, m.ControllerUID) || !PublicHTTPSMatches(object.Object, &row) {
				return ErrRefused
			}
			got = &row
		} else {
			var row corev1.Service
			if PublicHTTPSResourceRPC(ctx, target, "GET", object.Kind, object.Name, nil, &row) != nil || !PublicHTTPSOwned(m.Spec, &row, m.LoadBalancerUID) || !PublicHTTPSMatches(object.Object, &row) || len(row.Status.LoadBalancer.Ingress) != 1 || row.Status.LoadBalancer.Ingress[0].IP != m.Address {
				return ErrRefused
			}
			got = &row
		}
		if got.GetDeletionTimestamp() != nil || authorize(ctx) != nil {
			return ErrRefused
		}
	}
	service, e := target.Client.CoreV1().Services(target.Namespace).Get(ctx, m.Spec.ServiceName, metav1.GetOptions{})
	if e != nil || service == nil || string(service.UID) != m.AppServiceUID || !PublicHTTPSDestinationMatches(request.Blueprint, service) || authorize(ctx) != nil || PublicHTTPSNamespace(ctx, target) != nil {
		return ErrRefused
	}
	return nil
}

// Match the current blueprint's Service spec while preserving the original UID.
// New application images/releases may update the same Service without changing
// this dedicated route or recreating certificate state/controller resources.
func PublicHTTPSDestinationMatches(blueprint deploymentplan.Blueprint, service *corev1.Service) bool {
	plan, e := deploymentplan.Compile(blueprint)
	if e != nil || service == nil {
		return false
	}
	for _, a := range plan.Artifacts {
		var meta struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal([]byte(a.Content), &meta) != nil {
			return false
		}
		if meta.Kind != "Service" {
			continue
		}
		var expected corev1.Service
		if json.Unmarshal([]byte(a.Content), &expected) != nil {
			return false
		}
		if expected.Name == service.Name {
			return serviceMatches(&expected, service) && service.Labels["app.kubernetes.io/managed-by"] == "luci-blueprint" && service.Labels["luci.ai/project"] == projectLabel(blueprint.ProjectID)
		}
	}
	return false
}
