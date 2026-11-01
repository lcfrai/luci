package deploymentplan

import (
	"encoding/json"
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const PublicRouteProfile = "traefik-websecure"

var publicManagedID = regexp.MustCompile(`^https-[a-f0-9]{32}$`)

const ManagedPublicRouteProfile = "luci-managed-https"
const PublicRouteController = "traefik.io/ingress-controller"

// PublicRoute references an operator-owned controller, DNS name and TLS Secret.
// It contains no certificate, private key, arbitrary annotation or DNS account.
type PublicRoute struct {
	Profile          string `json:"profile" enum:"traefik-websecure,luci-managed-https"`
	ManagedHTTPSID   string `json:"managedHTTPSId,omitempty"`
	ServiceID        string `json:"serviceId"`
	IngressClassName string `json:"ingressClassName"`
	Hostname         string `json:"hostname"`
	Path             string `json:"path"`
	PathType         string `json:"pathType" enum:"Exact,Prefix"`
	TLSSecretName    string `json:"tlsSecretName"`
}

func ValidPublicHostname(value string) bool {
	if len(value) > 253 || !strings.Contains(value, ".") || value != strings.ToLower(value) || len(kubevalidation.IsDNS1123Subdomain(value)) != 0 || net.ParseIP(value) != nil {
		return false
	}
	last := value[strings.LastIndex(value, ".")+1:]
	if strings.Trim(last, "0123456789") == "" {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal"} {
		if strings.HasSuffix(value, suffix) {
			return false
		}
	}
	return true
}
func validatePublicRoute(b Blueprint) error {
	r := b.PublicRoute
	if r == nil {
		return nil
	}
	if b.Infrastructure.Kind != "kubernetes" || (r.Profile != PublicRouteProfile && r.Profile != ManagedPublicRouteProfile) || !validName(r.ServiceID) || !ValidPublicHostname(r.Hostname) || (r.PathType != "Prefix" && r.PathType != "Exact") || r.Path == "" || len(r.Path) > 256 || !strings.HasPrefix(r.Path, "/") || path.Clean(r.Path) != r.Path || strings.ContainsAny(r.Path, "\\%?#") {
		return fmt.Errorf("public HTTPS routing needs one exact app, existing Traefik class/TLS Secret, DNS hostname and clean Exact/Prefix path")
	}
	if r.Profile == PublicRouteProfile {
		if r.ManagedHTTPSID != "" || len(r.IngressClassName) > 253 || len(kubevalidation.IsDNS1123Subdomain(r.IngressClassName)) != 0 || len(r.TLSSecretName) > 253 || len(kubevalidation.IsDNS1123Subdomain(r.TLSSecretName)) != 0 {
			return fmt.Errorf("manual public routing needs existing controller and TLS Secret, without a managed endpoint")
		}
	} else if !strings.HasPrefix(r.ManagedHTTPSID, "https-") || len(r.ManagedHTTPSID) != 38 || !publicManagedID.MatchString(r.ManagedHTTPSID) || r.IngressClassName != "" || r.TLSSecretName != "" {
		return fmt.Errorf("select one retained managed HTTPS endpoint; no duplicate Ingress or TLS Secret is generated")
	}
	for _, c := range r.Path {
		if c < 33 || c > 126 {
			return fmt.Errorf("public route path must be visible ASCII without query or credentials")
		}
	}
	found := false
	for _, s := range b.Services {
		if s.ID == r.ServiceID && s.Kind == "app" && s.Collector == nil {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("public route must select an existing app service; inference and Collector ports are not publicly exposed by this profile")
	}
	return nil
}
func PublicRouteName(b Blueprint) string {
	if b.PublicRoute == nil {
		return ""
	}
	return ResourceName(b.ProjectID, b.PublicRoute.ServiceID+"-public")
}
func publicIngressArtifact(b Blueprint, planDigest string) (Artifact, error) {
	r := b.PublicRoute
	if r == nil {
		return Artifact{}, fmt.Errorf("no public route")
	}
	name := PublicRouteName(b)
	port := 0
	for _, s := range b.Services {
		if ResourceName(b.ProjectID, s.ID) == name {
			return Artifact{}, fmt.Errorf("public route resource name collision")
		}
		if s.ID == r.ServiceID {
			port = s.Port
		}
	}
	if port < 1 || port > 65535 {
		return Artifact{}, fmt.Errorf("public app requires its declared Service port")
	}
	value := map[string]any{
		"apiVersion": "networking.k8s.io/v1", "kind": "Ingress",
		"metadata": map[string]any{"name": name, "namespace": b.Infrastructure.Namespace,
			"labels":      map[string]string{"app.kubernetes.io/managed-by": "luci-blueprint", "luci.ai/project": digest([]byte(b.ProjectID))[:12], "luci.ai/component": r.ServiceID},
			"annotations": map[string]string{"luci.ai/plan-digest": planDigest, "luci.ai/source-commit": b.Source.Commit, "traefik.ingress.kubernetes.io/router.entrypoints": "websecure", "traefik.ingress.kubernetes.io/router.tls": "true"}},
		"spec": map[string]any{"ingressClassName": r.IngressClassName,
			"tls":   []any{map[string]any{"hosts": []string{r.Hostname}, "secretName": r.TLSSecretName}},
			"rules": []any{map[string]any{"host": r.Hostname, "http": map[string]any{"paths": []any{map[string]any{"path": r.Path, "pathType": r.PathType, "backend": map[string]any{"service": map[string]any{"name": ResourceName(b.ProjectID, r.ServiceID), "port": map[string]int{"number": port}}}}}}}}},
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Artifact{}, err
	}
	raw = append(raw, '\n')
	return Artifact{Name: name + ".ingress.json", MediaType: "application/json", SHA256: digest(raw), Content: string(raw)}, nil
}
