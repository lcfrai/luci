package product

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func kubeTestCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fixture only"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestKubernetesTargetTLSIsExplicitAndHasNoAmbientProxyOrCredentialFallback(t *testing.T) {
	credentials := KubernetesTargetCredentials{CA: kubeTestCA(t), Token: "fixture-only-not-a-real-token"}
	config, err := kubernetesRESTConfig("https://cluster.example:6443", credentials)
	if err != nil || config.Insecure || config.TLSClientConfig.CAFile != "" || config.BearerTokenFile != "" || config.ExecProvider != nil || config.AuthProvider != nil {
		t.Fatal("explicit TLS configuration refused or widened", err)
	}
	request, _ := http.NewRequest("GET", "https://cluster.example", nil)
	if proxy, err := config.Proxy(request); err != nil || proxy != nil {
		t.Fatal("ambient proxy inherited")
	}
	for _, server := range []string{"http://cluster.example", "https://user:password@cluster.example", "https://cluster.example/path", "https://cluster.example?token=bad"} {
		if _, err := kubernetesRESTConfig(server, credentials); err == nil {
			t.Fatal("unsafe endpoint accepted", server)
		}
	}
	credentials.Certificate = "also-a-certificate"
	credentials.Key = "also-a-key"
	if _, err := kubernetesRESTConfig("https://cluster.example", credentials); err == nil {
		t.Fatal("mixed credential sources accepted")
	}
	credentials = KubernetesTargetCredentials{Token: "fixture-only"}
	if _, err := kubernetesRESTConfig("https://cluster.example", credentials); err == nil {
		t.Fatal("system trust fallback accepted")
	}
}

func TestKubernetesPrivateWorkloadsDoNotRequirePublicPublishingButRetainAllocationAndBudgetGates(t *testing.T) {
	s, p, _, ctx := azureBlueprintFixture(t)
	actor, policy, _ := management.ActorFromContext(ctx)
	policy.AllowPublicPublishing = false
	policy.AllowCloudProvisioning = true
	policy.MonthlyBudgetUSD = 100
	denied := false
	s.coordinatorDeploymentAuthorize = func(_ context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		if ref.UserID != actor.ID || ref.ProjectID != p.ID || allocate && denied {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	ref, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.kubernetesAuthority(ctx, ref, true, false, 50); err != nil {
		t.Fatal("private ClusterIP workload incorrectly requires public publishing", err)
	}
	policy.AllowCloudProvisioning = false
	if s.kubernetesAuthority(ctx, ref, true, false, 50) == nil {
		t.Fatal("provisioning policy ignored")
	}
	policy.AllowCloudProvisioning = true
	if s.kubernetesAuthority(ctx, ref, true, false, 101) == nil || s.kubernetesAuthority(ctx, ref, true, false, 0) == nil {
		t.Fatal("authority budget ignored")
	}
	denied = true
	if s.kubernetesAuthority(ctx, ref, true, false, 50) == nil {
		t.Fatal("current allocation refusal ignored")
	}
	if s.kubernetesAuthority(ctx, ref, false, false, 0) != nil {
		t.Fatal("retained observation incorrectly needs allocation")
	}
	foreign := ref
	foreign.UserID = "another-user"
	if s.kubernetesAuthority(ctx, foreign, false, false, 0) == nil {
		t.Fatal("forged actor adopted")
	}
}

func TestKubernetesReviewRefusesUnsavedTopologyAndMutableImageBeforeAnyTargetRead(t *testing.T) {
	s, p, _, ctx := azureBlueprintFixture(t)
	actor, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReviewKubernetesDeployment(ctx, actor, KubernetesReviewInput{TargetID: "cluster-" + strings.Repeat("a", 32), BlueprintRevision: 1, BlueprintDigest: strings.Repeat("b", 64), RunID: "run-" + strings.Repeat("a", 24), CheckID: "check-one", MonthlyBudgetUSD: 10})
	if err == nil {
		t.Fatal("unsaved blueprint authorized cluster inspection")
	}
	if kubeImmutableImage.MatchString("registry/app:latest") || kubeImmutableImage.MatchString("registry/app@sha256:abc") || !kubeImmutableImage.MatchString("registry/app@sha256:"+strings.Repeat("b", 64)) {
		t.Fatal("immutable image reference boundary changed")
	}
}
