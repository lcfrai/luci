package deploymentkube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"reflect"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Values never enter receipts: this records only original class/Secret identity
// and the closed route selection. Browser trust, DNS and reachability are unknown.
type PublicRouteBinding struct {
	Version           int    `json:"version"`
	RouteDigest       string `json:"routeDigest"`
	NamespaceUID      string `json:"namespaceUid"`
	TargetFingerprint string `json:"targetFingerprint"`
	ClassName         string `json:"className"`
	ClassUID          string `json:"classUid"`
	ClassVersion      string `json:"classVersion"`
	Controller        string `json:"controller"`
	TLSSecretName     string `json:"tlsSecretName"`
	TLSSecretUID      string `json:"tlsSecretUid"`
	TLSSecretVersion  string `json:"tlsSecretVersion"`
	MetadataDigest    string `json:"metadataDigest"`
}

func publicBindingDigest(row PublicRouteBinding) string { row.MetadataDigest = ""; return digest(row) }
func validatePublicRouteBinding(request Request) error {
	r, b := request.Blueprint.PublicRoute, request.PublicRouteBinding
	if r == nil {
		if b != nil {
			return ErrRefused
		}
		return nil
	}
	if b == nil || b.Version != 1 || b.RouteDigest != digest(*r) || b.NamespaceUID == "" || len(b.NamespaceUID) > 128 || b.TargetFingerprint != request.Identity.TargetFingerprint || b.ClassName != r.IngressClassName || b.Controller != deploymentplan.PublicRouteController || b.TLSSecretName != r.TLSSecretName || b.MetadataDigest != publicBindingDigest(*b) {
		return ErrRefused
	}
	for _, id := range []string{b.ClassUID, b.ClassVersion, b.TLSSecretUID, b.TLSSecretVersion} {
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "\x00\r\n") {
			return ErrRefused
		}
	}
	return nil
}

func ReadPublicRouteBinding(ctx context.Context, target Target, blueprint deploymentplan.Blueprint, authorize func(context.Context) error) (*PublicRouteBinding, error) {
	r := blueprint.PublicRoute
	if r == nil {
		return nil, nil
	}
	if target.Client == nil || authorize == nil || target.Namespace != blueprint.Infrastructure.Namespace || target.NamespaceUID == "" {
		return nil, ErrRefused
	}
	namespace := func() error {
		if authorize(ctx) != nil {
			return ErrRefused
		}
		row, e := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
		if e != nil || row == nil || string(row.UID) != target.NamespaceUID || row.DeletionTimestamp != nil || authorize(ctx) != nil {
			return ErrRefused
		}
		return nil
	}
	if namespace() != nil {
		return nil, ErrRefused
	}
	if authorize(ctx) != nil {
		return nil, ErrRefused
	}
	class, err := target.Client.NetworkingV1().IngressClasses().Get(ctx, r.IngressClassName, metav1.GetOptions{})
	if err != nil || class == nil || class.Name != r.IngressClassName || class.Namespace != "" || class.DeletionTimestamp != nil || class.Spec.Controller != deploymentplan.PublicRouteController || class.Spec.Parameters != nil || authorize(ctx) != nil {
		return nil, ErrRefused
	}
	if authorize(ctx) != nil {
		return nil, ErrRefused
	}
	secret, err := target.Client.CoreV1().Secrets(target.Namespace).Get(ctx, r.TLSSecretName, metav1.GetOptions{})
	if err != nil || secret == nil || secret.Name != r.TLSSecretName || secret.Namespace != target.Namespace || secret.DeletionTimestamp != nil || secret.Type != corev1.SecretTypeTLS || len(secret.Data[corev1.TLSCertKey]) == 0 || len(secret.Data[corev1.TLSCertKey]) > 256<<10 || len(secret.Data[corev1.TLSPrivateKeyKey]) == 0 || len(secret.Data[corev1.TLSPrivateKeyKey]) > 64<<10 || authorize(ctx) != nil {
		return nil, ErrRefused
	}
	// Parse only transiently. No CA/private key material, certificate body or
	// parser/provider error is returned, hashed into evidence or logged here.
	pair, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil || len(pair.Certificate) == 0 {
		return nil, ErrRefused
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	now := time.Now()
	if err != nil || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) || leaf.VerifyHostname(r.Hostname) != nil {
		return nil, ErrRefused
	}
	serverUsage := len(leaf.ExtKeyUsage) == 0
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth || usage == x509.ExtKeyUsageAny {
			serverUsage = true
		}
	}
	if !serverUsage {
		return nil, ErrRefused
	}
	row := &PublicRouteBinding{Version: 1, RouteDigest: digest(*r), NamespaceUID: target.NamespaceUID, TargetFingerprint: target.Fingerprint, ClassName: class.Name, ClassUID: string(class.UID), ClassVersion: class.ResourceVersion, Controller: class.Spec.Controller, TLSSecretName: secret.Name, TLSSecretUID: string(secret.UID), TLSSecretVersion: secret.ResourceVersion}
	row.MetadataDigest = publicBindingDigest(*row)
	request := Request{Blueprint: blueprint, Identity: Identity{TargetFingerprint: target.Fingerprint}, PublicRouteBinding: row}
	if validatePublicRouteBinding(request) != nil || namespace() != nil {
		return nil, ErrRefused
	}
	return row, nil
}

// One dedicated hostname is supported. Refuse even a different path or class
// for the same host rather than silently sharing another owner's routing space.
// This bounded read is not an atomic cluster-wide hostname reservation.
func (c *Connector) publicHostnameAvailable(ctx context.Context, target Target, request Request, mutation bool) error {
	r := request.Blueprint.PublicRoute
	if r == nil {
		return nil
	}
	if c.authorize(ctx, request, mutation) != nil {
		return ErrRefused
	}
	rows, err := target.Client.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 129})
	if err != nil || rows == nil || rows.Continue != "" || len(rows.Items) > 128 || c.authorize(ctx, request, mutation) != nil {
		return ErrRefused
	}
	name := deploymentplan.PublicRouteName(request.Blueprint)
	ownUID := ""
	for _, expected := range request.Expected {
		if expected.Kind == "Ingress" && expected.Name == name {
			ownUID = expected.UID
		}
	}
	for _, row := range rows.Items {
		overlaps := false
		for _, rule := range row.Spec.Rules {
			host := strings.ToLower(rule.Host)
			if host == "" || host == r.Hostname || strings.HasPrefix(host, "*.") && strings.HasSuffix(r.Hostname, host[1:]) {
				overlaps = true
			}
		}
		if !overlaps {
			continue
		}
		// A known original resource may be updated; an unknown Create is never
		// certified as the original merely because its labels look the same.
		ours := row.Namespace == target.Namespace && row.Name == name && row.DeletionTimestamp == nil && row.Labels["app.kubernetes.io/managed-by"] == "luci-blueprint" && row.Labels["luci.ai/project"] == projectLabel(request.Identity.ProjectID) && len(row.OwnerReferences) == 0
		if !ours || !(ownUID != "" && string(row.UID) == ownUID || row.Annotations[operationAnnotation] == operationLabel(request.Identity)) {
			return ErrRefused
		}
	}
	return nil
}
func (c *Connector) validatePublicRouteReferences(ctx context.Context, target Target, request Request, service *deploymentplan.Service, mutation bool) error {
	r := request.Blueprint.PublicRoute
	if r == nil {
		return validatePublicRouteBinding(request)
	}
	if service != nil && service.ID != r.ServiceID {
		return nil
	}
	actual, err := ReadPublicRouteBinding(ctx, target, request.Blueprint, func(ctx context.Context) error { return c.authorize(ctx, request, mutation) })
	if err != nil {
		return ErrRefused
	}
	if !reflect.DeepEqual(actual, request.PublicRouteBinding) {
		return ErrReferenceChanged
	}
	return c.publicHostnameAvailable(ctx, target, request, mutation)
}
