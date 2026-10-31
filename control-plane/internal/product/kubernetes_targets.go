package product

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrKubernetesReview = errors.New("The cluster target, reviewed deployment or current project access changed. Read its retained status and review again; an unconfirmed operation is never repeated automatically.")

// Only these explicit credentials are admitted. No kubeconfig, plugin, ambient
// proxy, environment authentication or system trust fallback is consulted.
type KubernetesTargetCredentials struct {
	CA          string `json:"ca"`
	Token       string `json:"token,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	Key         string `json:"key,omitempty"`
}
type KubernetesTargetDescriptor struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	Server       string `json:"server"`
	Namespace    string `json:"namespace"`
	NamespaceUID string `json:"namespaceUid"`
	Revision     int    `json:"revision"`
	Fingerprint  string `json:"fingerprint"`
}
type KubernetesTargetSave struct {
	ID               string                      `json:"id"`
	ExpectedRevision int                         `json:"expectedRevision"`
	Name             string                      `json:"name"`
	Server           string                      `json:"server"`
	Namespace        string                      `json:"namespace"`
	Credentials      KubernetesTargetCredentials `json:"credentials"`
}
type privateKubernetesTarget struct {
	KubernetesTargetDescriptor
	Credentials KubernetesTargetCredentials `json:"credentials"`
}
type kubernetesTargetLedger struct {
	Version      int                       `json:"version"`
	ProjectID    string                    `json:"projectId"`
	AccountScope string                    `json:"accountScope"`
	ProjectScope string                    `json:"projectScope"`
	Targets      []privateKubernetesTarget `json:"targets"`
}

func kubeOpaqueID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func kubernetesRESTConfig(server string, credentials KubernetesTargetCredentials) (*rest.Config, error) {
	u, err := url.Parse(server)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || len(server) > 2048 {
		return nil, ErrKubernetesReview
	}
	if len(credentials.CA) == 0 || len(credentials.CA) > 64<<10 || len(credentials.Token) > 16<<10 || len(credentials.Certificate) > 64<<10 || len(credentials.Key) > 64<<10 || strings.ContainsAny(credentials.Token, "\r\n\x00") {
		return nil, ErrKubernetesReview
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(credentials.CA)) {
		return nil, ErrKubernetesReview
	}
	if credentials.Token != "" {
		if credentials.Certificate != "" || credentials.Key != "" {
			return nil, ErrKubernetesReview
		}
	} else if _, err := tls.X509KeyPair([]byte(credentials.Certificate), []byte(credentials.Key)); err != nil {
		return nil, ErrKubernetesReview
	}
	return &rest.Config{Host: strings.TrimSuffix(server, "/"), BearerToken: credentials.Token,
		TLSClientConfig: rest.TLSClientConfig{CAData: []byte(credentials.CA), CertData: []byte(credentials.Certificate), KeyData: []byte(credentials.Key)},
		Timeout:         10 * time.Second, QPS: 5, Burst: 5,
		Proxy: func(*http.Request) (*url.URL, error) { return nil, nil }}, nil
}

func kubernetesClient(target privateKubernetesTarget) (kubernetes.Interface, error) {
	config, err := kubernetesRESTConfig(target.Server, target.Credentials)
	if err != nil {
		return nil, ErrKubernetesReview
	}
	client, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, ErrKubernetesReview
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("Cluster redirects are refused.") }
	return kubernetes.NewForConfigAndClient(config, client)
}

// false still requires current Edit+Deploy. Only new effects additionally need
// current allocation/provisioning policy and a positive reviewed budget. These
// ClusterIP workloads create no public endpoint, so public publishing is separate.
func (s *Server) kubernetesAuthority(ctx context.Context, actor management.ExecutionActorRef, mutation, secrets bool, budget float64) error {
	if ctx.Err() != nil || actor.Local || actor.UserID == "" || s.coordinatorDeploymentAuthorize == nil {
		return management.ErrExecutionAuthority
	}
	currentCaller, err := management.CaptureDeploymentActor(ctx, actor.ProjectID)
	if err != nil || currentCaller != actor {
		return management.ErrExecutionAuthority
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, actor, mutation)
	if err != nil {
		return management.ErrExecutionAuthority
	}
	fresh := management.WithExecutionAuthority(ctx, authority)
	if captured, err := management.CaptureDeploymentActor(fresh, actor.ProjectID); err != nil || captured != actor {
		return management.ErrExecutionAuthority
	}
	if secrets && actor.TeamID != "" && (authority.Team == nil || !authority.Team.Permissions.Secrets) {
		return management.ErrExecutionAuthority
	}
	if mutation && (!authority.Policy.AllowCloudProvisioning || budget <= 0 || math.IsNaN(budget) || math.IsInf(budget, 0) || budget > authority.Policy.MonthlyBudgetUSD) {
		return management.ErrExecutionAuthority
	}
	return ctx.Err()
}

func (s *Server) readKubernetesTargets(projectID string) (string, kubernetesTargetLedger, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", kubernetesTargetLedger{}, err
	}
	s.mu.RLock()
	scope, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", kubernetesTargetLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "kubernetes-targets-"+projectID)
	empty := kubernetesTargetLedger{Version: 1, ProjectID: projectID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Targets: []privateKubernetesTarget{}}
	if err != nil {
		return "", empty, err
	}
	var saved kubernetesTargetLedger
	err = externalReadJSON(path, 4<<20, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != projectID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Targets == nil || len(saved.Targets) > 32 {
		return "", empty, ErrKubernetesReview
	}
	ids := map[string]bool{}
	for _, target := range saved.Targets {
		copy := target
		copy.Fingerprint = ""
		if !validKubeTarget(target) || ids[target.ID] || target.ProjectID != projectID || target.Fingerprint != reviewDigest(copy) {
			return "", empty, ErrKubernetesReview
		}
		if _, err := kubernetesRESTConfig(target.Server, target.Credentials); err != nil {
			return "", empty, ErrKubernetesReview
		}
		ids[target.ID] = true
	}
	return path, saved, nil
}

func validKubeTarget(target privateKubernetesTarget) bool {
	return strings.HasPrefix(target.ID, "cluster-") && len(target.ID) == 40 && validConnectionToken(target.ID, 40, false) && target.Name != "" && len(target.Name) <= 120 && len(kubevalidation.IsDNS1123Label(target.Namespace)) == 0 && target.NamespaceUID != "" && len(target.NamespaceUID) <= 128 && target.Revision >= 1 && target.Revision <= 1_000_000_000
}

func (s *Server) ListKubernetesTargets(ctx context.Context, actor management.ExecutionActorRef) ([]KubernetesTargetDescriptor, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, false, 0); err != nil {
		return nil, err
	}
	_, ledger, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	rows := []KubernetesTargetDescriptor{}
	for _, row := range ledger.Targets {
		rows = append(rows, row.KubernetesTargetDescriptor)
	}
	if err := s.kubernetesAuthority(ctx, actor, false, false, 0); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Server) SaveKubernetesTarget(ctx context.Context, actor management.ExecutionActorRef, input KubernetesTargetSave) (KubernetesTargetDescriptor, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	if input.Name == "" || len(input.Name) > 120 || input.ExpectedRevision < 0 || input.ExpectedRevision >= 1_000_000_000 || len(kubevalidation.IsDNS1123Label(input.Namespace)) != 0 {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	if _, err := kubernetesRESTConfig(input.Server, input.Credentials); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	id := input.ID
	if id == "" {
		if input.ExpectedRevision != 0 {
			return KubernetesTargetDescriptor{}, ErrKubernetesReview
		}
		var err error
		id, err = kubeOpaqueID("cluster-")
		if err != nil {
			return KubernetesTargetDescriptor{}, err
		}
	}
	row := privateKubernetesTarget{KubernetesTargetDescriptor: KubernetesTargetDescriptor{ID: id, ProjectID: actor.ProjectID, Name: input.Name, Server: strings.TrimSuffix(input.Server, "/"), Namespace: input.Namespace, Revision: input.ExpectedRevision + 1}, Credentials: input.Credentials}
	client, err := kubernetesClient(row)
	if err != nil {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	namespace, err := client.CoreV1().Namespaces().Get(ctx, row.Namespace, metav1.GetOptions{})
	if err != nil || namespace == nil || namespace.UID == "" || namespace.DeletionTimestamp != nil {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	row.NamespaceUID = string(namespace.UID)
	if !validKubeTarget(row) {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	row.Fingerprint = reviewDigest(row)
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	// A separate lock avoids recursive s.mu acquisition during private path reads.
	s.kubernetesTargetMu.Lock()
	defer s.kubernetesTargetMu.Unlock()
	path, ledger, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	found := -1
	for index, target := range ledger.Targets {
		if target.ID == id {
			found = index
			if target.Revision != input.ExpectedRevision {
				return KubernetesTargetDescriptor{}, ErrKubernetesReview
			}
		}
	}
	if found < 0 && (input.ID != "" || input.ExpectedRevision != 0 || len(ledger.Targets) >= 32) {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	if found < 0 {
		ledger.Targets = append(ledger.Targets, row)
	} else {
		ledger.Targets[found] = row
	}
	if raw, err := json.MarshalIndent(ledger, "", "  "); err != nil || len(raw)+1 > 4<<20 {
		return KubernetesTargetDescriptor{}, ErrKubernetesReview
	}
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	if err := durableRunJSON(path, ledger); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return KubernetesTargetDescriptor{}, err
	}
	return row.KubernetesTargetDescriptor, nil
}

func (s *Server) resolveKubernetesTarget(ctx context.Context, identity deploymentkube.Identity) (deploymentkube.Target, error) {
	_, ledger, err := s.readKubernetesTargets(identity.ProjectID)
	if err != nil {
		return deploymentkube.Target{}, ErrKubernetesReview
	}
	for _, target := range ledger.Targets {
		if target.ID != identity.TargetID || target.Fingerprint != identity.TargetFingerprint {
			continue
		}
		client, err := kubernetesClient(target)
		if err != nil {
			return deploymentkube.Target{}, ErrKubernetesReview
		}
		return deploymentkube.Target{ID: target.ID, Fingerprint: target.Fingerprint, Namespace: target.Namespace, NamespaceUID: target.NamespaceUID, Client: client}, nil
	}
	return deploymentkube.Target{}, ErrKubernetesReview
}
