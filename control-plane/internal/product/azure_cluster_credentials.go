package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

const azureAKSTokenAudience = "6dae42f8-4368-4678-94ff-3960e28e3630"

var errAzureCluster = fmt.Errorf("The original Azure cluster, account, reviewed access or project context could not be confirmed. Read its retained status; an uncertain effect is never replayed.")

type azureAccountMetadata struct {
	ID              string `json:"id"`
	TenantID        string `json:"tenantId"`
	EnvironmentName string `json:"environmentName"`
	State           string `json:"state"`
	User            struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"user"`
}

// No serialization or persisted token cache: the bearer exists only for the
// current authenticated client operation, and every re-resolution is fresh.
type azureClusterBearer struct {
	token     string
	expires   time.Time
	principal deploymentplan.AzureClusterPrincipal
}

func azureAccountPrincipal(account azureAccountMetadata) (deploymentplan.AzureClusterPrincipal, error) {
	kind := ""
	switch account.User.Type {
	case "user":
		kind = "User"
	case "servicePrincipal":
		kind = "ServicePrincipal"
	default:
		return deploymentplan.AzureClusterPrincipal{}, errAzureCluster
	}
	if account.EnvironmentName != "AzureCloud" || account.State != "Enabled" || account.ID == "" || account.TenantID == "" || strings.TrimSpace(account.User.Name) == "" || len(account.User.Name) > 512 {
		return deploymentplan.AzureClusterPrincipal{}, errAzureCluster
	}
	return deploymentplan.AzureClusterPrincipal{SubscriptionID: strings.ToLower(account.ID), TenantID: strings.ToLower(account.TenantID), Kind: kind, AccountName: strings.ToLower(account.User.Name)}, nil
}

// These are correlation claims from the already authorized CLI response, not a
// local signature verification or an authorization grant. The exact TLS-bound
// Kubernetes server independently authenticates the bearer. A forged browser
// claim can never enter this function's CLI response or supply project rights.
func azureClusterBearerClaims(token string, principal deploymentplan.AzureClusterPrincipal, now time.Time) (azureClusterBearer, error) {
	if len(token) == 0 || len(token) > 16<<10 || strings.ContainsAny(token, "\r\n\x00") {
		return azureClusterBearer{}, errAzureCluster
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		return azureClusterBearer{}, errAzureCluster
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) > 12<<10 {
		return azureClusterBearer{}, errAzureCluster
	}
	var claims struct {
		Audience  string `json:"aud"`
		Tenant    string `json:"tid"`
		Object    string `json:"oid"`
		Issuer    string `json:"iss"`
		Expires   int64  `json:"exp"`
		NotBefore int64  `json:"nbf"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Audience != azureAKSTokenAudience || strings.ToLower(claims.Tenant) != principal.TenantID || (claims.Issuer != "https://sts.windows.net/"+principal.TenantID+"/" && claims.Issuer != "https://login.microsoftonline.com/"+principal.TenantID+"/v2.0") || claims.NotBefore > now.Add(time.Minute).Unix() || claims.Expires <= now.Add(2*time.Minute).Unix() || claims.Expires > now.Add(26*time.Hour).Unix() {
		return azureClusterBearer{}, errAzureCluster
	}
	principal.ObjectID = strings.ToLower(claims.Object)
	if !deploymentplan.ValidateAzureClusterPrincipal(principal) {
		return azureClusterBearer{}, errAzureCluster
	}
	return azureClusterBearer{token: token, expires: time.Unix(claims.Expires, 0), principal: principal}, nil
}

func (s *Server) azureClusterToken(ctx context.Context, actor management.ExecutionActorRef, spec deploymentplan.AzureClusterSpec, expected *deploymentplan.AzureClusterPrincipal) (azureClusterBearer, error) {
	if !azureSetupUUID.MatchString(spec.SubscriptionID) || !azureSetupUUID.MatchString(spec.TenantID) || expected != nil && !deploymentplan.ValidateAzureClusterPrincipal(*expected) {
		return azureClusterBearer{}, errAzureCluster
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	read := func(args ...string) (string, error) {
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", management.ErrExecutionAuthority
		}
		raw, err := s.runner(ctx, "az", args, "")
		if err != nil || len(raw) > 64<<10 || ctx.Err() != nil {
			return "", errAzureCluster
		}
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", management.ErrExecutionAuthority
		}
		return raw, nil
	}
	accountRaw, err := read("account", "show", "--subscription", spec.SubscriptionID, "--output", "json")
	if err != nil {
		return azureClusterBearer{}, err
	}
	var account azureAccountMetadata
	if json.Unmarshal([]byte(accountRaw), &account) != nil {
		return azureClusterBearer{}, errAzureCluster
	}
	principal, err := azureAccountPrincipal(account)
	if err != nil || principal.SubscriptionID != spec.SubscriptionID || principal.TenantID != spec.TenantID {
		return azureClusterBearer{}, errAzureCluster
	}
	raw, err := read("account", "get-access-token", "--subscription", spec.SubscriptionID, "--resource", azureAKSTokenAudience, "--output", "json")
	if err != nil {
		return azureClusterBearer{}, err
	}
	var response struct {
		AccessToken  string `json:"accessToken"`
		TokenType    string `json:"tokenType"`
		Tenant       string `json:"tenant"`
		Subscription string `json:"subscription"`
	}
	if json.Unmarshal([]byte(raw), &response) != nil || response.TokenType != "Bearer" || strings.ToLower(response.Tenant) != spec.TenantID || strings.ToLower(response.Subscription) != spec.SubscriptionID {
		return azureClusterBearer{}, errAzureCluster
	}
	bearer, err := azureClusterBearerClaims(response.AccessToken, principal, time.Now().UTC())
	if err != nil || expected != nil && bearer.principal != *expected {
		return azureClusterBearer{}, errAzureCluster
	}
	latestRaw, err := read("account", "show", "--subscription", spec.SubscriptionID, "--output", "json")
	if err != nil {
		return azureClusterBearer{}, err
	}
	var latest azureAccountMetadata
	if json.Unmarshal([]byte(latestRaw), &latest) != nil {
		return azureClusterBearer{}, errAzureCluster
	}
	current, e := azureAccountPrincipal(latest)
	principal.ObjectID = ""
	if e != nil || current != principal || !time.Now().Before(bearer.expires.Add(-2*time.Minute)) {
		return azureClusterBearer{}, errAzureCluster
	}
	return bearer, nil
}

// The provider envelope may contain unused users/contexts (including an exec
// stanza). They are inert parsed data, never selected, persisted or executed.
// Only ONE exact ARM-derived server/CA pair is admitted into the typed source.
func azureClusterServerCA(raw string, expectedFQDN string) (string, string, error) {
	if len(raw) > 256<<10 || expectedFQDN == "" || len(expectedFQDN) > 253 || strings.ContainsAny(expectedFQDN, "/:@\\ \r\n\t") {
		return "", "", errAzureCluster
	}
	var envelope struct {
		Kubeconfigs []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"kubeconfigs"`
	}
	if json.Unmarshal([]byte(raw), &envelope) != nil || len(envelope.Kubeconfigs) != 1 {
		return "", "", errAzureCluster
	}
	data, err := base64.StdEncoding.DecodeString(envelope.Kubeconfigs[0].Value)
	if err != nil || len(data) > 128<<10 {
		return "", "", errAzureCluster
	}
	var config clientcmdv1.Config
	if yaml.UnmarshalStrict(data, &config) != nil || config.Kind != "Config" || config.APIVersion != "v1" || len(config.Clusters) != 1 {
		return "", "", errAzureCluster
	}
	cluster := config.Clusters[0].Cluster
	u, err := url.Parse(cluster.Server)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), expectedFQDN) || u.Port() != "" && u.Port() != "443" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != "" && u.Path != "/" || cluster.InsecureSkipTLSVerify || cluster.ProxyURL != "" || cluster.CertificateAuthority != "" || cluster.TLSServerName != "" || len(cluster.CertificateAuthorityData) == 0 || len(cluster.CertificateAuthorityData) > 64<<10 {
		return "", "", errAzureCluster
	}
	server := "https://" + strings.ToLower(u.Hostname())
	ca := string(cluster.CertificateAuthorityData)
	// Pure validation; this placeholder is never returned as authentication or
	// used in a client. It only exercises the same strict explicit TLS contract.
	if _, err = kubernetesRESTConfig(server, KubernetesTargetCredentials{CA: ca, Token: "validation-only-not-an-access-token"}); err != nil {
		return "", "", errAzureCluster
	}
	return server, ca, nil
}

func decodeBoundedAzureJSON(raw string, out any) error {
	if len(raw) > 256<<10 {
		return errAzureCluster
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		return errAzureCluster
	}
	return nil
}
