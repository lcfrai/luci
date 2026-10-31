package product

import (
	"encoding/base64"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"strings"
	"testing"
	"time"
)

func azureClusterTokenFixture(now time.Time, change func(map[string]any)) string {
	claims := map[string]any{"aud": azureAKSTokenAudience, "tid": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "oid": "cccccccc-cccc-cccc-cccc-cccccccccccc", "iss": "https://sts.windows.net/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/", "nbf": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix()}
	if change != nil {
		change(claims)
	}
	raw, _ := json.Marshal(claims)
	return "controller-cli-header." + base64.RawURLEncoding.EncodeToString(raw) + ".signature-not-locally-verified"
}
func TestAzureClusterTokenCorrelationRejectsForeignAudienceTenantAndExpiredIdentity(t *testing.T) {
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	principal := deploymentplan.AzureClusterPrincipal{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Kind: "User", AccountName: "operator@example.com"}
	token := azureClusterTokenFixture(now, nil)
	value, err := azureClusterBearerClaims(token, principal, now)
	if err != nil || value.principal.ObjectID != "cccccccc-cccc-cccc-cccc-cccccccccccc" || value.token != token {
		t.Fatal("original CLI correlation unavailable", err)
	}
	for _, change := range []func(map[string]any){func(c map[string]any) { c["aud"] = "foreign" }, func(c map[string]any) { c["tid"] = "dddddddd-dddd-dddd-dddd-dddddddddddd" }, func(c map[string]any) { c["exp"] = now.Add(-time.Minute).Unix() }, func(c map[string]any) { c["iss"] = "https://untrusted.example/" }, func(c map[string]any) { c["oid"] = "not-an-object-id" }} {
		if _, err := azureClusterBearerClaims(azureClusterTokenFixture(now, change), principal, now); err == nil {
			t.Fatal("foreign or expired correlation accepted")
		}
	}
}
func TestAzureClusterUserEnvelopeExtractsOnlyExactServerCAWithoutFollowingExec(t *testing.T) {
	ca := kubeTestCA(t)
	config := map[string]any{"apiVersion": "v1", "kind": "Config", "clusters": []any{map[string]any{"name": "unused-name", "cluster": map[string]string{"server": "https://owned.example:443", "certificate-authority-data": base64.StdEncoding.EncodeToString([]byte(ca))}}}, "users": []any{map[string]any{"name": "unused-user", "user": map[string]any{"exec": map[string]any{"apiVersion": "client.authentication.k8s.io/v1beta1", "command": "MUST-NOT-EXECUTE", "args": []string{"PRIVATE-UNUSED-ARG"}}}}}, "contexts": []any{map[string]any{"name": "unused-context", "context": map[string]string{"cluster": "unused-name", "user": "unused-user"}}}, "current-context": "unused-context"}
	raw, _ := json.Marshal(config)
	envelope, _ := json.Marshal(map[string]any{"kubeconfigs": []any{map[string]string{"name": "clusterUser", "value": base64.StdEncoding.EncodeToString(raw)}}})
	server, received, err := azureClusterServerCA(string(envelope), "owned.example")
	if err != nil || server != "https://owned.example" || received != ca || strings.Contains(server, "PRIVATE") {
		t.Fatal("strict CA/server extraction failed", err)
	}
	if _, _, err = azureClusterServerCA(string(envelope), "foreign.example"); err == nil {
		t.Fatal("foreign ARM endpoint accepted")
	}
	config["clusters"] = append(config["clusters"].([]any), config["clusters"].([]any)[0])
	raw, _ = json.Marshal(config)
	envelope, _ = json.Marshal(map[string]any{"kubeconfigs": []any{map[string]string{"value": base64.StdEncoding.EncodeToString(raw)}}})
	if _, _, err = azureClusterServerCA(string(envelope), "owned.example"); err == nil {
		t.Fatal("ambiguous cluster configuration accepted")
	}
}
