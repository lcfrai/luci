package deploymentkube

import (
	"context"
	"net"
	"reflect"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PublicRouteObservation struct {
	State                     string   `json:"state"` // pending, unknown, controller-reported
	URL                       string   `json:"url,omitempty"`
	IngressUID                string   `json:"ingressUid"`
	IngressVersion            string   `json:"ingressVersion"`
	ServiceUID                string   `json:"serviceUid"`
	ServiceName               string   `json:"serviceName"`
	ServiceVersion            string   `json:"serviceVersion"`
	ClassUID                  string   `json:"classUid"`
	TLSSecretUID              string   `json:"tlsSecretUid"`
	Addresses                 []string `json:"addresses"`
	ObservedAt                string   `json:"observedAt"`
	DNSVerified               bool     `json:"dnsVerified"`
	HTTPSVerified             bool     `json:"httpsVerified"`
	ApplicationHealthVerified bool     `json:"applicationHealthVerified"`
}

func safeIngressAnnotations(object *networkingv1.Ingress) bool {
	if object == nil {
		return false
	}
	allowed := map[string]bool{"luci.ai/plan-digest": true, "luci.ai/source-commit": true, operationAnnotation: true, artifactAnnotation: true, "traefik.ingress.kubernetes.io/router.entrypoints": true, "traefik.ingress.kubernetes.io/router.tls": true}
	for key := range object.Annotations {
		if !allowed[key] {
			return false
		}
	}
	return object.Annotations["traefik.ingress.kubernetes.io/router.entrypoints"] == "websecure" && object.Annotations["traefik.ingress.kubernetes.io/router.tls"] == "true"
}
func ingressMatches(expected, actual *networkingv1.Ingress) bool {
	if expected == nil || actual == nil || !safeIngressAnnotations(actual) || !reflect.DeepEqual(expected.Spec, actual.Spec) {
		return false
	}
	for key, value := range expected.Annotations {
		if actual.Annotations[key] != value {
			return false
		}
	}
	return true
}
func publicURL(request Request) string {
	r := request.Blueprint.PublicRoute
	if r == nil {
		return ""
	}
	return "https://" + r.Hostname + r.Path
}

func (c *Connector) observedPublicRoute(ctx context.Context, target Target, request Request, desired desiredResource, receipt ResourceReceipt, operation Operation) (*PublicRouteObservation, metav1.Object, error) {
	r, b := request.Blueprint.PublicRoute, request.PublicRouteBinding
	if r == nil || b == nil || desired.ingress == nil {
		return nil, nil, ErrRefused
	}
	view := &PublicRouteObservation{State: "pending", Addresses: []string{}, ClassUID: b.ClassUID, TLSSecretUID: b.TLSSecretUID, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	object, err := target.Client.NetworkingV1().Ingresses(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
	if err != nil || object == nil || !objectOwned(object, request, desired, true) || !ingressMatches(desired.ingress, object) {
		return view, object, ErrUnknown
	}
	// An uncertain Create with no positively retained UID cannot become original
	// ownership solely from labels. A new explicit reviewed existing-object action
	// can bind its current exact UID; this original attempt is never replayed.
	if receipt.ObservedUID == "" && receipt.UID == "" {
		view.State = "unknown"
		return view, object, ErrUnknown
	}
	if receipt.ObservedUID != "" && receipt.ObservedUID != string(object.UID) || receipt.UID != "" && receipt.UID != string(object.UID) {
		return view, object, ErrUnknown
	}
	var desiredService *corev1.Service
	var serviceReceipt *ResourceReceipt
	_, resources, err := prepare(request)
	if err != nil {
		return view, object, ErrRefused
	}
	for _, item := range resources {
		if item.service != nil && item.service.Labels["luci.ai/component"] == r.ServiceID {
			desiredService = item.service
		}
	}
	for i := range operation.Resources {
		item := &operation.Resources[i]
		if item.Kind == "Service" && item.Name == deploymentplan.ResourceName(request.Blueprint.ProjectID, r.ServiceID) {
			serviceReceipt = item
		}
	}
	if desiredService == nil || serviceReceipt == nil || !serviceReceipt.Attempted || serviceReceipt.ObservedUID == "" {
		return view, object, ErrUnknown
	}
	if c.authorize(ctx, request, false) != nil {
		return nil, nil, ErrRefused
	}
	service, err := target.Client.CoreV1().Services(target.Namespace).Get(ctx, desiredService.Name, metav1.GetOptions{})
	ds := desiredResource{kind: "Service", name: desiredService.Name, service: desiredService}
	for _, item := range resources {
		if item.kind == "Service" && item.name == desiredService.Name {
			ds = item
		}
	}
	if err != nil || service == nil || string(service.UID) != serviceReceipt.ObservedUID || !objectOwned(service, request, ds, true) || !serviceMatches(desiredService, service) || service.Spec.ClusterIP == "" || service.Spec.ClusterIP == "None" {
		return view, object, ErrUnknown
	}
	view.IngressUID, view.IngressVersion = string(object.UID), object.ResourceVersion
	view.ServiceUID, view.ServiceVersion, view.ServiceName = string(service.UID), service.ResourceVersion, service.Name
	if len(object.Status.LoadBalancer.Ingress) > 8 {
		return view, object, ErrUnknown
	}
	seen := map[string]bool{}
	for _, address := range object.Status.LoadBalancer.Ingress {
		value := address.IP
		if value == "" {
			value = address.Hostname
		}
		if value == "" || len(value) > 253 || !validIngressAddress(value) || seen[value] || address.IP != "" && address.Hostname != "" || len(address.Ports) > 1 {
			return view, object, ErrUnknown
		}
		for _, port := range address.Ports {
			if port.Port != 443 || port.Protocol != corev1.ProtocolTCP || port.Error != nil {
				return view, object, ErrUnknown
			}
		}
		seen[value] = true
		view.Addresses = append(view.Addresses, value)
	}
	sort.Strings(view.Addresses)
	if len(view.Addresses) > 0 {
		view.State = "controller-reported"
		view.URL = publicURL(request)
	}
	// Status is an attributed controller report, not a DNS lookup, TLS handshake,
	// public reachability check or application/model health result.
	return view, object, nil
}

func validatePublicRouteObservation(operation Operation) error {
	r, view := operation.Request.Blueprint.PublicRoute, operation.PublicRoute
	if r == nil {
		if view != nil {
			return ErrRefused
		}
		return nil
	}
	if view == nil {
		return nil
	} // a reserved/unknown original has no positive read yet
	if view.State != "pending" && view.State != "unknown" && view.State != "controller-reported" || view.DNSVerified || view.HTTPSVerified || view.ApplicationHealthVerified || len(view.Addresses) > 8 {
		return ErrRefused
	}
	if _, err := time.Parse(time.RFC3339Nano, view.ObservedAt); err != nil {
		return ErrRefused
	}
	if view.State != "controller-reported" {
		if view.URL != "" {
			return ErrRefused
		}
		return nil
	}
	b := operation.Request.PublicRouteBinding
	if b == nil || view.URL != publicURL(operation.Request) || view.ClassUID != b.ClassUID || view.TLSSecretUID != b.TLSSecretUID || len(view.Addresses) == 0 || operation.ReferenceState != "current" {
		return ErrRefused
	}
	ingress, service := false, false
	for _, row := range operation.Resources {
		if row.Kind == "Ingress" && row.Name == deploymentplan.PublicRouteName(operation.Request.Blueprint) && row.Attempted && row.Ready && row.ObservedUID == view.IngressUID && row.ObservedVersion == view.IngressVersion {
			ingress = true
		}
		if row.Kind == "Service" && row.Name == deploymentplan.ResourceName(operation.Request.Blueprint.ProjectID, r.ServiceID) && row.Attempted && row.ObservedUID == view.ServiceUID {
			service = true
		}
	}
	if !ingress || !service || view.IngressUID == "" || view.IngressVersion == "" || view.ServiceUID == "" || view.ServiceVersion == "" || view.ServiceName != deploymentplan.ResourceName(operation.Request.Blueprint.ProjectID, r.ServiceID) {
		return ErrRefused
	}
	for _, value := range view.Addresses {
		if value == "" || len(value) > 253 || !validIngressAddress(value) {
			return ErrRefused
		}
	}
	return nil
}
func validIngressAddress(value string) bool {
	return net.ParseIP(value) != nil || deploymentplan.ValidPublicHostname(value)
}
func (c *Connector) publicRouteStillObserved(ctx context.Context, target Target, operation Operation) error {
	view := operation.PublicRoute
	if view == nil || view.State != "controller-reported" {
		return nil
	}
	request, resources, err := prepare(operation.Request)
	if err != nil {
		return err
	}
	for i, desired := range resources {
		if desired.ingress != nil {
			next, _, err := c.observedPublicRoute(ctx, target, request, desired, operation.Resources[i], operation)
			if err != nil || next == nil {
				return ErrRefused
			}
			next.ObservedAt = view.ObservedAt
			if !reflect.DeepEqual(next, view) {
				return ErrRefused
			}
		}
	}
	return c.authorize(ctx, request, false)
}
