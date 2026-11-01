package product

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"math"
	"os"
	"regexp"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var errPublicHTTPSRetireCluster = errors.New("Retire the owned HTTPS endpoint before deleting this cluster; paused endpoints still retain a load balancer and DNS. For an unconfirmed original Create, remove only its exact retained name through the operator, then Observe its absence. No replacement is adopted and no resource is deleted automatically.")
var ErrPublicHTTPS = errors.New("The original HTTPS review, owned resource or current access changed. Read the retained operation; an unconfirmed effect is never replayed.")
var ErrPublicHTTPSDomain = errors.New("Choose a public DNS hostname inside the selected existing Azure DNS zone. The zone must already be delegated publicly; use a clean Exact or Prefix path.")
var ErrPublicHTTPSStorage = errors.New("Choose an available existing storage class on this target. HTTPS needs a writable 1 GiB persistent filesystem for certificate/account recovery; Immediate or WaitForFirstConsumer binding is supported.")
var ErrPublicHTTPSApplication = errors.New("Deploy an app service to this target first, then select its retained deployment and service. A positively owned current Service UID and declared port are required; inference/telemetry ports are not exposed.")
var ErrPublicHTTPSInput = errors.New("Complete the reviewed HTTPS settings and acknowledge Let's Encrypt registration/terms, private DNS credential storage and continuing infrastructure/DNS costs until pause or retirement.")
var ErrPublicHTTPSCapacity = errors.New("Retained HTTPS capacity is full. Active, paused or unresolved endpoints are never hidden. Retire owned resources or let unused non-applied reviews expire before creating another review.")
var publicHTTPSKey = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
var publicHTTPSID = regexp.MustCompile(`^https-[a-f0-9]{32}$`)

type publicHTTPSIndex struct {
	Version int      `json:"version"`
	IDs     []string `json:"ids"`
}
type publicHTTPSActive struct {
	Version int                    `json:"version"`
	Rows    []publicHTTPSActiveRef `json:"rows"`
}
type publicHTTPSActiveRef struct {
	ProjectID  string `json:"projectId"`
	ID         string `json:"id"`
	Hostname   string `json:"hostname"`
	PlanDigest string `json:"planDigest"`
}

func publicHTTPSPlanDigest(p PublicHTTPSPlan) string { p.Digest = ""; return reviewDigest(p) }
func publicHTTPSReviewID(actor management.ExecutionActorRef, key string) string {
	return "https-" + reviewDigest(struct {
		Actor management.ExecutionActorRef
		Key   string
	}{actor, key})[:32]
}
func (s *Server) publicHTTPSPath(p project.Project, id string) (string, error) {
	if !publicHTTPSID.MatchString(id) {
		return "", ErrPublicHTTPS
	}
	s.mu.RLock()
	root := s.config.ProjectsDir
	s.mu.RUnlock()
	return privateApplicationStatePathForRoot(root, s.stateDir, "public-https-"+p.ID+"-"+id)
}
func publicHTTPSState(v string) bool {
	switch v {
	case "reviewed", "provisioning", "pending-address", "pending-dns", "pending-certificate", "ready", "paused", "retiring", "retired", "unknown", "access-changed":
		return true
	}
	return false
}
func validStoredPublicHTTPS(row storedPublicHTTPS, p project.Project) bool {
	created, e1 := time.Parse(time.RFC3339Nano, row.Plan.CreatedAt)
	expires, e2 := time.Parse(time.RFC3339Nano, row.Plan.ExpiresAt)
	if row.Version != 1 || row.ProjectID != p.ID || row.Actor.ProjectID != p.ID || row.Actor.Local || row.Actor.UserID == "" || row.Revision < 1 || row.Revision > 1e9 || !publicHTTPSKey.MatchString(row.ReviewKey) || row.ID != publicHTTPSReviewID(row.Actor, row.ReviewKey) || row.Plan.ID != row.ID || row.Plan.ProjectID != p.ID || row.Plan.Profile != deploymentkube.PublicHTTPSProfile || row.Plan.Digest != publicHTTPSPlanDigest(row.Plan) || !publicHTTPSState(row.State) || e1 != nil || e2 != nil || !expires.Equal(created.Add(10*time.Minute)) || row.Resources == nil || len(row.Resources) != 5 || row.Plan.StorageGiB != 1 || row.ApplicationHealthVerified || !hexDigest(row.ReviewRequestDigest) || !hexDigest(row.SourceBlueprintDigest) || !hexDigest(row.SourceContextDigest) || !hexDigest(row.SourceRunDigest) || !hexDigest(row.SourceCheckDigest) || row.DesiredReplicas < 0 || row.DesiredReplicas > 1 {
		return false
	}
	name := deploymentkube.PublicHTTPSName(p.ID, row.ID)
	seen := map[string]bool{}
	for _, r := range row.Resources {
		if seen[r.Kind] || r.Name != name || r.UID != "" && !validConnectionToken(r.UID, 128, false) || r.UID != "" && !r.Attempted || r.AbsentObserved && !r.DeleteAttempted && !(row.RetireAttempted && (r.UID == "" || r.Kind == "PersistentVolumeClaim")) {
			return false
		}
		seen[r.Kind] = true
		switch r.Kind {
		case "PersistentVolumeClaim", "Secret", "ConfigMap", "Deployment", "Service":
		default:
			return false
		}
		switch r.State {
		case "planned", "unknown", "observed", "pending", "ready", "deleted", "retained":
		default:
			return false
		}
	}
	if row.DNSAcknowledged && (!row.DNSAttempted || row.DNSRecordETag == "") || row.DNSObserved && !row.DNSAcknowledged || row.DNSAbsentObserved && !row.DNSDeleteAttempted && !(row.RetireAttempted && !row.DNSAcknowledged) || row.HTTPSVerified && (!row.DNSObserved || row.State != "ready" || row.Address == "" || row.CertificateNotAfter == "" || !hexDigest(row.CertificateSHA256)) || row.URL != "" && (!row.HTTPSVerified || row.URL != "https://"+row.Plan.Hostname+row.Plan.Path) {
		return false
	}
	if row.State != "retired" && (row.DNSCredential.ClientID != row.Plan.DNSClientID || !azureSetupUUID.MatchString(row.DNSCredential.ClientID) || !azureSetupUUID.MatchString(row.DNSCredential.TenantID) || len(row.DNSCredential.ClientSecret) < 16 || len(row.DNSCredential.ClientSecret) > 4096) {
		return false
	}
	if row.Actions == nil || len(row.Actions) > 32 || len(row.Message) > 2048 || len(row.Plan.ACMEEmail) > 254 || len(row.Plan.AzureDNSZoneID) > 1024 || row.Plan.Destination.ServicePort < 1 || row.Plan.Destination.ServicePort > 65535 || !hexDigest(row.Plan.Destination.TargetFingerprint) || row.Plan.Destination.NamespaceUID == "" || row.Plan.Destination.ServiceUID == "" || !deploymentplan.ValidPublicHostname(row.Plan.Hostname) || !publicHTTPSPathGrammar.MatchString(row.Plan.Path) || len(row.Plan.Path) > 256 || row.Plan.PathType != "Exact" && row.Plan.PathType != "Prefix" || row.Plan.MonthlyBudgetUSD <= 0 || math.IsNaN(row.Plan.MonthlyBudgetUSD) || math.IsInf(row.Plan.MonthlyBudgetUSD, 0) {
		return false
	}
	keys := map[string]bool{}
	for _, a := range row.Actions {
		if !publicHTTPSKey.MatchString(a.RequestKey) || keys[a.RequestKey] || !hexDigest(a.RequestDigest) || !publicHTTPSActionKind(a.Action) || a.StartedRevision < 1 || a.StartedRevision >= row.Revision || a.State != "attempted" && a.State != "settled" && a.State != "unknown" || a.State == "settled" && (a.SettledRevision <= a.StartedRevision || a.SettledRevision > row.Revision) || a.State != "settled" && a.SettledRevision != 0 {
			return false
		}
		keys[a.RequestKey] = true
	}
	credential := publicHTTPSCredentialData(row)
	if row.State == "retired" {
		credential = map[string][]byte{"tenant-id": []byte("retired"), "client-id": []byte("retired"), "client-secret": []byte("retired")}
	}
	if _, e := deploymentkube.PublicHTTPSObjects(publicHTTPSSpec(row), credential); e != nil {
		return false
	}
	return true
}
func (s *Server) readPublicHTTPS(p project.Project, id string) (string, storedPublicHTTPS, error) {
	path, e := s.publicHTTPSPath(p, id)
	if e != nil {
		return "", storedPublicHTTPS{}, e
	}
	var row storedPublicHTTPS
	if e = externalReadJSON(path, 128<<10, &row); e != nil {
		return path, row, e
	}
	s.mu.RLock()
	scope, e := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if e != nil || row.AccountScope != scope.AccountScope || row.ProjectScope != scope.ProjectScope || !validStoredPublicHTTPS(row, p) {
		return "", storedPublicHTTPS{}, ErrPublicHTTPS
	}
	return path, row, nil
}
func publicHTTPSWrite(path string, value any, limit int) error {
	raw, e := json.MarshalIndent(value, "", "  ")
	if e != nil || len(raw)+1 > limit {
		return ErrPublicHTTPS
	}
	return durableRunJSON(path, value)
}

// Caller controls mutation admission. The transaction never performs remote I/O.
// It is also used to retain a positively acknowledged original UID after a late
// revocation/cancellation; that bookkeeping grants no further effect authority.
func (s *Server) publicHTTPSMutate(p project.Project, id string, actor management.ExecutionActorRef, fn func(*storedPublicHTTPS) error) (storedPublicHTTPS, error) {
	s.publicHTTPSMu.Lock()
	defer s.publicHTTPSMu.Unlock()
	path, row, e := s.readPublicHTTPS(p, id)
	if e != nil || row.Actor != actor {
		return row, ErrPublicHTTPS
	}
	if e = fn(&row); e != nil {
		return row, e
	}
	row.Revision++
	if !validStoredPublicHTTPS(row, p) || publicHTTPSWrite(path, row, 128<<10) != nil {
		return row, ErrPublicHTTPS
	}
	return row, nil
}
func (s *Server) publicHTTPSIndexPath(projectID string) (string, error) {
	s.mu.RLock()
	root := s.config.ProjectsDir
	s.mu.RUnlock()
	return privateApplicationStatePathForRoot(root, s.stateDir, "public-https-index-"+projectID)
}
func (s *Server) readPublicHTTPSIndex(projectID string) (string, publicHTTPSIndex, error) {
	path, e := s.publicHTTPSIndexPath(projectID)
	empty := publicHTTPSIndex{Version: 1, IDs: []string{}}
	if e != nil {
		return "", empty, e
	}
	var v publicHTTPSIndex
	e = externalReadJSON(path, 128<<10, &v)
	if errors.Is(e, os.ErrNotExist) {
		return path, empty, nil
	}
	if e != nil || v.Version != 1 || v.IDs == nil || len(v.IDs) > 128 {
		return "", empty, ErrPublicHTTPS
	}
	seen := map[string]bool{}
	for _, id := range v.IDs {
		if !publicHTTPSID.MatchString(id) || seen[id] {
			return "", empty, ErrPublicHTTPS
		}
		seen[id] = true
	}
	return path, v, nil
}
func (s *Server) publicHTTPSRememberLocked(p project.Project, id string) error {
	path, index, e := s.readPublicHTTPSIndex(p.ID)
	if e != nil {
		return e
	}
	for _, old := range index.IDs {
		if old == id {
			return nil
		}
	}
	if len(index.IDs) >= 128 {
		removed := false
		for i, old := range index.IDs {
			_, r, e := s.readPublicHTTPS(p, old)
			if e != nil {
				return e
			}
			expiry, _ := time.Parse(time.RFC3339Nano, r.Plan.ExpiresAt)
			if r.State == "retired" && !r.PVCDataRetained || !r.ApplyAttempted && time.Now().After(expiry) {
				index.IDs = append(index.IDs[:i], index.IDs[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return ErrPublicHTTPSCapacity
		}
	}
	index.IDs = append(index.IDs, id)
	return publicHTTPSWrite(path, index, 128<<10)
}
func (s *Server) publicHTTPSActivePath() (string, error) {
	s.mu.RLock()
	root := s.config.ProjectsDir
	s.mu.RUnlock()
	return privateApplicationStatePathForRoot(root, s.stateDir, "public-https-active")
}
func (s *Server) readPublicHTTPSActive() (string, publicHTTPSActive, error) {
	path, e := s.publicHTTPSActivePath()
	empty := publicHTTPSActive{Version: 1, Rows: []publicHTTPSActiveRef{}}
	if e != nil {
		return "", empty, e
	}
	var v publicHTTPSActive
	e = externalReadJSON(path, 128<<10, &v)
	if errors.Is(e, os.ErrNotExist) {
		return path, empty, nil
	}
	if e != nil || v.Version != 1 || v.Rows == nil || len(v.Rows) > 512 {
		return "", empty, ErrPublicHTTPS
	}
	seen := map[string]bool{}
	for _, r := range v.Rows {
		if !coolifyID.MatchString(r.ProjectID) || !publicHTTPSID.MatchString(r.ID) || r.Hostname == "" || !hexDigest(r.PlanDigest) || seen[r.ProjectID+"/"+r.ID] {
			return "", empty, ErrPublicHTTPS
		}
		seen[r.ProjectID+"/"+r.ID] = true
	}
	return path, v, nil
}
func (s *Server) publicHTTPSActivateLocked(p project.Project, row storedPublicHTTPS) error {
	id := row.ID
	path, index, e := s.readPublicHTTPSActive()
	if e != nil {
		return e
	}
	for _, r := range index.Rows {
		if r.ProjectID == p.ID && r.ID == id {
			if r.PlanDigest != row.Plan.Digest {
				return ErrPublicHTTPS
			}
			return nil
		}
		if r.Hostname == row.Plan.Hostname {
			return ErrPublicHTTPS
		}
	}
	if len(index.Rows) >= 512 {
		return ErrPublicHTTPSCapacity
	}
	index.Rows = append(index.Rows, publicHTTPSActiveRef{p.ID, id, row.Plan.Hostname, row.Plan.Digest})
	return publicHTTPSWrite(path, index, 128<<10)
}
func (s *Server) PublicHTTPSList(ctx context.Context, actor management.ExecutionActorRef, key string) ([]PublicHTTPSStatus, error) {
	if s.kubernetesAuthority(ctx, actor, false, false, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return nil, e
	}
	s.publicHTTPSMu.Lock()
	defer s.publicHTTPSMu.Unlock()
	ids := []string{}
	if key != "" {
		if !publicHTTPSKey.MatchString(key) {
			return nil, ErrPublicHTTPS
		}
		ids = []string{publicHTTPSReviewID(actor, key)}
	} else {
		_, index, e := s.readPublicHTTPSIndex(p.ID)
		if e != nil {
			return nil, e
		}
		ids = index.IDs
	}
	out := []PublicHTTPSStatus{}
	for _, id := range ids {
		_, row, e := s.readPublicHTTPS(p, id)
		if key != "" && errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if row.Actor == actor {
			out = append(out, row.PublicHTTPSStatus)
		}
	}
	if s.kubernetesAuthority(ctx, actor, false, false, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	return out, nil
}
func (s *Server) PublicHTTPSGet(ctx context.Context, actor management.ExecutionActorRef, id string) (PublicHTTPSStatus, error) {
	if s.kubernetesAuthority(ctx, actor, false, false, 0) != nil {
		return PublicHTTPSStatus{}, management.ErrExecutionAuthority
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	_, row, e := s.readPublicHTTPS(p, id)
	if e != nil || row.Actor != actor || s.kubernetesAuthority(ctx, actor, false, false, 0) != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	return row.PublicHTTPSStatus, nil
}

func (s *Server) publicHTTPSDeactivateLocked(p project.Project, row storedPublicHTTPS) error {
	path, index, e := s.readPublicHTTPSActive()
	if e != nil {
		return e
	}
	kept := []publicHTTPSActiveRef{}
	for _, r := range index.Rows {
		if r.ProjectID == p.ID && r.ID == row.ID {
			if r.PlanDigest != row.Plan.Digest || row.State != "retired" {
				return ErrPublicHTTPS
			}
			continue
		}
		kept = append(kept, r)
	}
	index.Rows = kept
	return publicHTTPSWrite(path, index, 128<<10)
}
