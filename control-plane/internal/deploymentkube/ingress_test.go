package deploymentkube

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"io"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"math/big"
	"strings"
	"testing"
	"time"
)

func publicIngressFixture(t *testing.T) (*Connector, *fake.Clientset, *memoryJournal, Request) {
	t.Helper()
	c, client, journal, request := workloadFixture(t)
	request.Blueprint.PublicRoute = &deploymentplan.PublicRoute{Profile: deploymentplan.PublicRouteProfile, ServiceID: "web", IngressClassName: "traefik", Hostname: "app.example.com", Path: "/", PathType: "Prefix", TLSSecretName: "site-tls"}
	plan, err := deploymentplan.Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.Identity.PlanDigest = plan.Digest
	request.Expected = nil
	for _, a := range plan.Artifacts {
		var row struct {
			Kind     string
			Metadata struct{ Name string }
		}
		if json.Unmarshal([]byte(a.Content), &row) != nil {
			t.Fatal("invalid artifact")
		}
		request.Expected = append(request.Expected, ExpectedResource{Kind: row.Kind, Name: row.Metadata.Name})
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fixture"}, DNSNames: []string{"app.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	class := &networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: "traefik", UID: "class-one", ResourceVersion: "1"}, Spec: networkingv1.IngressClassSpec{Controller: deploymentplan.PublicRouteController}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "site-tls", Namespace: "apps", UID: "tls-one", ResourceVersion: "1"}, Type: corev1.SecretTypeTLS, Data: map[string][]byte{corev1.TLSCertKey: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), corev1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})}}
	if err = client.Tracker().Add(class); err != nil {
		t.Fatal(err)
	}
	if err = client.Tracker().Add(secret); err != nil {
		t.Fatal(err)
	}
	target, err := c.target(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PublicRouteBinding, err = ReadPublicRouteBinding(context.Background(), target, request.Blueprint, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return c, client, journal, request
}
func ingressWrites(client *fake.Clientset) int {
	n := 0
	for _, a := range client.Actions() {
		if a.GetResource().Resource == "ingresses" && (a.GetVerb() == "create" || a.GetVerb() == "update" || a.GetVerb() == "patch") {
			n++
		}
	}
	return n
}
func setIngressReport(t *testing.T, client *fake.Clientset, request Request) {
	t.Helper()
	row, err := client.NetworkingV1().Ingresses("apps").Get(context.Background(), deploymentplan.PublicRouteName(request.Blueprint), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	row.ResourceVersion = "2"
	row.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "8.8.8.8"}}
	if err = client.Tracker().Update(schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, row, "apps"); err != nil {
		t.Fatal(err)
	}
}

func TestPublicIngressRequiresOriginalControllerReportWithoutDNSOrHealthClaims(t *testing.T) {
	c, client, _, request := publicIngressFixture(t)
	op, err := c.Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if ingressWrites(client) != 1 || op.PublicRoute == nil || op.PublicRoute.URL != "" || op.PublicRoute.State != "pending" {
		t.Fatal("unobserved endpoint was presented", op.PublicRoute)
	}
	setIngressReport(t, client, request)
	op, err = c.Observe(context.Background(), request.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if op.PublicRoute == nil || op.PublicRoute.State != "controller-reported" || op.PublicRoute.URL != "https://app.example.com/" || op.PublicRoute.ServiceUID == "" || op.PublicRoute.IngressUID == "" || op.PublicRoute.DNSVerified || op.PublicRoute.HTTPSVerified || op.PublicRoute.ApplicationHealthVerified || op.ApplicationHealthVerified {
		t.Fatal("attribution or qualification missing", op.PublicRoute)
	}
	row, _ := client.NetworkingV1().Ingresses("apps").Get(context.Background(), deploymentplan.PublicRouteName(request.Blueprint), metav1.GetOptions{})
	row.Annotations["traefik.ingress.kubernetes.io/router.middlewares"] = "foreign-auth@kubernetescrd"
	if err = client.Tracker().Update(schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, row, "apps"); err != nil {
		t.Fatal(err)
	}
	op, err = c.Observe(context.Background(), request.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if op.PublicRoute != nil && op.PublicRoute.URL != "" {
		t.Fatal("mutated routing kept a public endpoint")
	}
}
func TestPublicIngressUnknownCreateCannotReplayOrPromoteLabelsToOwnership(t *testing.T) {
	c, client, _, request := publicIngressFixture(t)
	client.PrependReactor("create", "ingresses", func(a ktesting.Action) (bool, runtime.Object, error) {
		object := a.(ktesting.CreateAction).GetObject().DeepCopyObject()
		meta := object.(metav1.Object)
		meta.SetUID(types.UID("created-but-reply-lost"))
		meta.SetResourceVersion("1")
		if err := client.Tracker().Create(a.GetResource(), object, a.GetNamespace()); err != nil {
			t.Fatal(err)
		}
		return true, nil, io.ErrUnexpectedEOF
	})
	if _, err := c.Apply(context.Background(), request); !errors.Is(err, ErrUnknown) {
		t.Fatal("uncertain create was not retained", err)
	}
	setIngressReport(t, client, request)
	for i := 0; i < 2; i++ {
		_, _ = c.Apply(context.Background(), request)
		op, err := c.Observe(context.Background(), request.Identity)
		if err != nil {
			t.Fatal(err)
		}
		if op.PublicRoute != nil && op.PublicRoute.URL != "" {
			t.Fatal("labels-only object became original ownership")
		}
	}
	if ingressWrites(client) != 1 {
		t.Fatal("unknown create replayed")
	}
}
func TestPublicIngressRechecksTLSAndAuthorityAfterAttemptedMarker(t *testing.T) {
	for _, mode := range []string{"secret-revision", "grant"} {
		t.Run(mode, func(t *testing.T) {
			c, client, journal, request := publicIngressFixture(t)
			denied := false
			c.authorize = func(context.Context, Request, bool) error {
				if denied {
					return ErrRefused
				}
				return nil
			}
			journal.afterSave = func() {
				for _, row := range journal.operation.Resources {
					if row.Kind == "Ingress" && row.Attempted {
						journal.afterSave = nil
						if mode == "grant" {
							denied = true
						} else {
							secret, err := client.CoreV1().Secrets("apps").Get(context.Background(), "site-tls", metav1.GetOptions{})
							if err != nil {
								t.Fatal(err)
							}
							secret.ResourceVersion = "2"
							if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), secret, "apps"); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
			if _, err := c.Apply(context.Background(), request); err == nil {
				t.Fatal("late changed admission accepted")
			}
			if ingressWrites(client) != 0 {
				t.Fatal("Ingress written after changed admission")
			}
			if journal.operation == nil {
				t.Fatal("attempt was lost")
			}
			attempted := false
			for _, row := range journal.operation.Resources {
				if row.Kind == "Ingress" {
					attempted = row.Attempted
				}
			}
			if !attempted {
				t.Fatal("unknown attempt marker was erased")
			}
		})
	}
}
func TestPublicIngressRefusesForeignHostnameBeforeAnyMutation(t *testing.T) {
	c, client, _, request := publicIngressFixture(t)
	foreign := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "someone-else", Namespace: "other", UID: "foreign"}, Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "app.example.com"}}}}
	if err := client.Tracker().Add(foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Apply(context.Background(), request); err == nil || writes(client) != 0 {
		t.Fatal("conflicting host admitted or mutated", err)
	}
	raw, _ := json.Marshal(request.PublicRouteBinding)
	if strings.Contains(string(raw), "PRIVATE KEY") || strings.Contains(string(raw), "CERTIFICATE") {
		t.Fatal("TLS material entered retained metadata")
	}
}
