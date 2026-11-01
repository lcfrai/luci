package product

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var ErrCommercial = errors.New("Commercial source, saved version or current access changed. Read saved results before retrying; no payment or publication was performed.")

const commercialHistoryLimit = 100
const commercialRecordLimit = 100_000
const commercialSourceSafetyLimit = 1024
const commercialSafeInteger int64 = 9_007_199_254_740_991

var commercialOpaque = regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`)
var commercialCurrencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
var commercialMetadataKey = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,40}$`)

type CommercialStripeConnection struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccountID string `json:"accountId"`
	Live      bool   `json:"live"`
	Revision  int    `json:"revision"`
}
type CommercialStripeFilter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type CommercialReleaseSource struct {
	PublicationID  string `json:"publicationId"`
	Repository     string `json:"repository"`
	RepositoryID   int64  `json:"repositoryId"`
	ReleaseID      int64  `json:"releaseId"`
	AuthorID       int64  `json:"authorId"`
	Tag            string `json:"tag"`
	Commit         string `json:"commit"`
	ArtifactID     string `json:"artifactId"`
	ArtifactDigest string `json:"artifactDigest"`
	Label          string `json:"label"`
}
type CommercialConfiguration struct {
	Revision       int                         `json:"revision"`
	Stripe         *CommercialStripeConnection `json:"stripe"`
	StripeEnabled  bool                        `json:"stripeEnabled"`
	StripeFilter   *CommercialStripeFilter     `json:"stripeFilter"`
	WindowDays     int                         `json:"windowDays"`
	Releases       []CommercialReleaseSource   `json:"releases"`
	AutomaticReads bool                        `json:"automaticReads"`
}
type CommercialCurrency struct {
	Currency                string `json:"currency"`
	CapturedPayments        int64  `json:"capturedPayments"`
	CapturedMinor           int64  `json:"capturedMinor"`
	RefundedMinor           int64  `json:"refundedMinor"`
	DisputedPayments        int64  `json:"disputedPayments"`
	PaymentsWithoutCustomer int64  `json:"paymentsWithoutCustomer"`
}
type CommercialSnapshot struct {
	ID                       string                  `json:"id"`
	Digest                   string                  `json:"digest"`
	SourceKey                string                  `json:"sourceKey"`
	Provider                 string                  `json:"provider"`
	Label                    string                  `json:"label"`
	AccountID                string                  `json:"accountId,omitempty"`
	Live                     bool                    `json:"live"`
	Repository               string                  `json:"repository,omitempty"`
	ReleaseID                int64                   `json:"releaseId,omitempty"`
	PublicationID            string                  `json:"publicationId,omitempty"`
	ArtifactID               string                  `json:"artifactId,omitempty"`
	ArtifactDigest           string                  `json:"artifactDigest,omitempty"`
	Tag                      string                  `json:"tag,omitempty"`
	PeriodStart              string                  `json:"periodStart,omitempty"`
	PeriodEndExclusive       string                  `json:"periodEndExclusive,omitempty"`
	StartedAt                string                  `json:"startedAt"`
	ObservedAt               string                  `json:"observedAt"`
	Records                  int                     `json:"records"`
	MatchedCharges           int                     `json:"matchedCharges"`
	StripeFilter             *CommercialStripeFilter `json:"stripeFilter"`
	Attribution              string                  `json:"attribution"`
	Currencies               []CommercialCurrency    `json:"currencies"`
	PayingCustomerReferences int64                   `json:"payingCustomerReferences"`
	AssetDownloads           int64                   `json:"assetDownloads"`
	Reactions                *int64                  `json:"reactions"`
	Semantics                string                  `json:"semantics"`
}
type CommercialSourceStatus struct {
	SourceKey     string `json:"sourceKey"`
	LatestID      string `json:"latestId"`
	LastAttemptAt string `json:"lastAttemptAt"`
	LastSuccessAt string `json:"lastSuccessAt"`
	State         string `json:"state"`
	Records       int    `json:"records"`
}
type CommercialView struct {
	ProjectID             string                   `json:"projectId"`
	Configuration         CommercialConfiguration  `json:"configuration"`
	Status                []CommercialSourceStatus `json:"status"`
	Snapshots             []CommercialSnapshot     `json:"snapshots"`
	PrunedSnapshots       int                      `json:"prunedSnapshots"`
	RefreshMinutes        int                      `json:"refreshMinutes"`
	CollectionRecordLimit int                      `json:"collectionRecordLimit"`
}
type commercialLedger struct {
	Version               int                               `json:"version"`
	ProjectID             string                            `json:"projectId"`
	AccountScope          string                            `json:"accountScope"`
	ProjectScope          string                            `json:"projectScope"`
	Configuration         CommercialConfiguration           `json:"configuration"`
	StripeKey             string                            `json:"stripeKey"`
	ReleaseCLI            map[string]GitHubIssueCLIIdentity `json:"releaseCli"`
	CollectionGenerations map[string]string                 `json:"collectionGenerations"`
	Status                []CommercialSourceStatus          `json:"status"`
	Snapshots             []CommercialSnapshot              `json:"snapshots"`
	PrunedSnapshots       int                               `json:"prunedSnapshots"`
}
type commercialCollection struct {
	Version       int                `json:"version"`
	ProjectID     string             `json:"projectId"`
	AccountScope  string             `json:"accountScope"`
	ProjectScope  string             `json:"projectScope"`
	SourceKey     string             `json:"sourceKey"`
	SourceDigest  string             `json:"sourceDigest"`
	Snapshot      CommercialSnapshot `json:"snapshot"`
	Cursor        string             `json:"cursor"`
	Page          int                `json:"page"`
	DedupSalt     string             `json:"dedupSalt"`
	SeenRecords   []string           `json:"seenRecords"`
	SeenCustomers []string           `json:"seenCustomers"`
	Complete      bool               `json:"complete"`
	Reset         bool               `json:"reset,omitempty"`
}

func commercialSnapshotDigest(row CommercialSnapshot) string {
	row.ID, row.Digest, row.StartedAt, row.ObservedAt = "", "", "", ""
	return reviewDigest(row)
}
func validCommercialCurrency(v CommercialCurrency) bool {
	return commercialCurrencyCode.MatchString(v.Currency) && v.CapturedPayments >= 0 && v.CapturedPayments <= commercialRecordLimit && v.CapturedMinor >= 0 && v.CapturedMinor <= commercialSafeInteger && v.RefundedMinor >= 0 && v.RefundedMinor <= v.CapturedMinor && v.DisputedPayments >= 0 && v.DisputedPayments <= v.CapturedPayments && v.PaymentsWithoutCustomer >= 0 && v.PaymentsWithoutCustomer <= v.CapturedPayments
}
func validCommercialSnapshot(v CommercialSnapshot, complete bool) bool {
	if v.SourceKey == "" || len(v.SourceKey) > 300 || !validArtifactStoredText(v.Label) || v.Label == "" || len(v.Label) > 200 || !feedbackTimestamp(v.StartedAt) || v.Records < 0 || v.Records > commercialRecordLimit || v.MatchedCharges < 0 || v.MatchedCharges > v.Records || v.Currencies == nil || len(v.Currencies) > 180 || v.PayingCustomerReferences < 0 || v.PayingCustomerReferences > int64(v.MatchedCharges) || v.AssetDownloads < 0 || v.AssetDownloads > commercialSafeInteger || v.Reactions != nil && (*v.Reactions < 0 || *v.Reactions > commercialSafeInteger) {
		return false
	}
	if complete && (!feedbackTimestamp(v.ObservedAt) || v.Digest != commercialSnapshotDigest(v) || v.ID != "commercial-"+v.Digest) {
		return false
	}
	if v.Provider == "stripe" {
		start, e1 := time.Parse("2006-01-02", v.PeriodStart)
		end, e2 := time.Parse("2006-01-02", v.PeriodEndExclusive)
		if !strings.HasPrefix(v.AccountID, "acct_") || !commercialOpaque.MatchString(v.AccountID) || !validCommercialStripeFilter(v.StripeFilter) || e1 != nil || e2 != nil || !end.After(start) || end.Sub(start) > 90*24*time.Hour || v.SourceKey != commercialStripeKey(v.AccountID, v.Live, v.StripeFilter) || v.Repository != "" || v.ReleaseID != 0 || v.AssetDownloads != 0 || v.Reactions != nil || v.Semantics != stripeCommercialSemantics || v.StripeFilter == nil && v.Attribution != "account-unallocated" || v.StripeFilter != nil && v.Attribution != "charge-metadata-filter" {
			return false
		}
	} else if v.Provider == "github-release" {
		if !validFeedbackRepository(v.Repository) || v.ReleaseID < 1 || v.ReleaseID > commercialSafeInteger || v.SourceKey != commercialReleaseKey(v.Repository, v.ReleaseID) || !validGitHubPublicationID(v.PublicationID) || !prototypeReleaseID.MatchString(v.ArtifactID) || !hexDigest(v.ArtifactDigest) || !githubReleaseTag.MatchString(v.Tag) || len(v.Currencies) != 0 || v.PayingCustomerReferences != 0 || v.MatchedCharges != 0 || v.StripeFilter != nil || v.Attribution != "original-release" || v.AccountID != "" || v.PeriodStart != "" || v.PeriodEndExclusive != "" || v.Semantics != githubCommercialSemantics {
			return false
		}
	} else {
		return false
	}
	previous := ""
	captured := int64(0)
	for _, c := range v.Currencies {
		if !validCommercialCurrency(c) || c.Currency <= previous {
			return false
		}
		previous = c.Currency
		captured += c.CapturedPayments
	}
	if captured > int64(v.MatchedCharges) {
		return false
	}
	return true
}
func validCommercialStripeFilter(v *CommercialStripeFilter) bool {
	return v == nil || commercialMetadataKey.MatchString(v.Key) && v.Value != "" && len(v.Value) <= 200 && strings.TrimSpace(v.Value) == v.Value && !strings.ContainsAny(v.Value, "\r\n\t") && validArtifactStoredText(v.Value)
}
func commercialStripeKey(account string, live bool, filter *CommercialStripeFilter) string {
	mode := "test"
	if live {
		mode = "live"
	}
	suffix := "account"
	if filter != nil {
		suffix = reviewDigest(filter)[:16]
	}
	return "stripe:" + account + ":" + mode + ":" + suffix
}
func commercialReleaseKey(repo string, id int64) string {
	return "github:" + repo + ":" + fmtCommercialID(id)
}
func validCommercialConfig(v CommercialConfiguration, key string) bool {
	if v.Revision < 0 || v.Revision > 1_000_000_000 || v.WindowDays < 1 || v.WindowDays > 90 || v.Releases == nil || len(v.Releases) > commercialSourceSafetyLimit || !validCommercialStripeFilter(v.StripeFilter) || v.StripeEnabled && v.Stripe == nil {
		return false
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil || len(raw)+1 > 128<<10 {
		return false
	}
	if v.Stripe == nil {
		if key != "" {
			return false
		}
	} else {
		r := v.Stripe
		if !prototypeReleaseID.MatchString(r.ID) || !strings.HasPrefix(r.ID, "stripe-") || r.Name == "" || len(r.Name) > 120 || !validArtifactStoredText(r.Name) || !strings.HasPrefix(r.AccountID, "acct_") || !commercialOpaque.MatchString(r.AccountID) || r.Revision < 1 || r.Revision > 1_000_000_000 || stripeKeyMode(key) != map[bool]string{true: "live", false: "test"}[r.Live] {
			return false
		}
	}
	seen := map[string]bool{}
	for _, r := range v.Releases {
		k := commercialReleaseKey(r.Repository, r.ReleaseID)
		if seen[k] || !validGitHubPublicationID(r.PublicationID) || !validFeedbackRepository(r.Repository) || r.RepositoryID < 1 || r.RepositoryID > commercialSafeInteger || r.ReleaseID < 1 || r.ReleaseID > commercialSafeInteger || r.AuthorID < 1 || r.AuthorID > commercialSafeInteger || !githubReleaseTag.MatchString(r.Tag) || !githubReleaseSHA.MatchString(r.Commit) || !prototypeReleaseID.MatchString(r.ArtifactID) || !hexDigest(r.ArtifactDigest) || r.Label == "" || len(r.Label) > 200 || !validArtifactStoredText(r.Label) {
			return false
		}
		seen[k] = true
	}
	return true
}
func (s *Server) readCommercialLocked(p project.Project) (string, commercialLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", commercialLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "commercial-"+p.ID)
	empty := commercialLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Configuration: CommercialConfiguration{WindowDays: 30, Releases: []CommercialReleaseSource{}}, ReleaseCLI: map[string]GitHubIssueCLIIdentity{}, CollectionGenerations: map[string]string{}, Status: []CommercialSourceStatus{}, Snapshots: []CommercialSnapshot{}}
	if err != nil {
		return "", empty, err
	}
	var saved commercialLedger
	err = externalReadJSON(path, 8<<20, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != p.ID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || !validCommercialConfig(saved.Configuration, saved.StripeKey) || saved.CollectionGenerations == nil || len(saved.CollectionGenerations) != len(commercialEnabledKeys(saved)) || saved.ReleaseCLI == nil || len(saved.ReleaseCLI) != len(saved.Configuration.Releases) || saved.Snapshots == nil || len(saved.Snapshots) > max(commercialHistoryLimit, len(saved.Status)) || saved.Status == nil || len(saved.Status) > commercialSourceSafetyLimit+1 || saved.PrunedSnapshots < 0 {
		return "", empty, ErrCommercial
	}
	for _, key := range commercialEnabledKeys(saved) {
		if !hexDigest(saved.CollectionGenerations[key]) {
			return "", empty, ErrCommercial
		}
	}
	for _, r := range saved.Configuration.Releases {
		cli, ok := saved.ReleaseCLI[commercialReleaseKey(r.Repository, r.ReleaseID)]
		if !ok || cli.Path == "" || cli.Bytes < 1 || !hexDigest(cli.SHA256) {
			return "", empty, ErrCommercial
		}
	}
	known := map[string]bool{}
	snapshotSources := map[string]string{}
	for _, r := range saved.Snapshots {
		if !validCommercialSnapshot(r, true) || known[r.ID] {
			return "", empty, ErrCommercial
		}
		known[r.ID] = true
		snapshotSources[r.ID] = r.SourceKey
	}
	seen := map[string]bool{}
	for _, r := range saved.Status {
		if r.SourceKey == "" || len(r.SourceKey) > 300 || seen[r.SourceKey] || r.LatestID != "" && (!known[r.LatestID] || snapshotSources[r.LatestID] != r.SourceKey) || r.LastAttemptAt != "" && !feedbackTimestamp(r.LastAttemptAt) || r.LastSuccessAt != "" && !feedbackTimestamp(r.LastSuccessAt) || r.Records < 0 || r.Records > commercialRecordLimit {
			return "", empty, ErrCommercial
		}
		switch r.State {
		case "collecting", "ready", "read-failed", "window-limit", "access-changed":
		default:
			return "", empty, ErrCommercial
		}
		seen[r.SourceKey] = true
	}
	return path, saved, nil
}
func commercialWrite(path string, value any, limit int) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > limit {
		return ErrCommercial
	}
	return durableRunJSON(path, value)
}
func (s *Server) readCommercial(p project.Project) (string, commercialLedger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readCommercialLocked(p)
}
func commercialProgressSlot(ledger commercialLedger, key string) string {
	if ledger.Configuration.Stripe != nil && key == commercialStripeKey(ledger.Configuration.Stripe.AccountID, ledger.Configuration.Stripe.Live, ledger.Configuration.StripeFilter) {
		return "stripe"
	}
	for i, r := range ledger.Configuration.Releases {
		if key == commercialReleaseKey(r.Repository, r.ReleaseID) {
			return "release-" + fmtCommercialID(int64(i))
		}
	}
	return ""
}
func (s *Server) commercialProgress(p project.Project, ledger commercialLedger, key string) (string, *commercialCollection, error) {
	slot := commercialProgressSlot(ledger, key)
	if slot == "" {
		return "", nil, ErrCommercial
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "commercial-collection-"+p.ID+"-"+slot)
	if err != nil {
		return "", nil, err
	}
	var row commercialCollection
	err = externalReadJSON(path, 16<<20, &row)
	if errors.Is(err, os.ErrNotExist) {
		return path, nil, nil
	}
	if err != nil {
		return "", nil, ErrCommercial
	}
	if row.Version == 1 && row.ProjectID == p.ID && row.AccountScope == ledger.AccountScope && row.ProjectScope == ledger.ProjectScope && row.Reset && row.SourceKey == "" && row.SourceDigest == "" && row.Snapshot.ID == "" && row.Cursor == "" && row.DedupSalt == "" && row.SeenRecords == nil && row.SeenCustomers == nil {
		return path, nil, nil
	}
	if row.Version != 1 || row.ProjectID != p.ID || row.AccountScope != ledger.AccountScope || row.ProjectScope != ledger.ProjectScope || !hexDigest(row.SourceDigest) || row.SourceKey != row.Snapshot.SourceKey || !validCommercialSnapshot(row.Snapshot, false) || len(row.Cursor) > 255 || row.Cursor != "" && !commercialOpaque.MatchString(row.Cursor) || row.Page < 0 || row.Page > 1001 || !hexDigest(row.DedupSalt) || row.SeenRecords == nil || row.SeenCustomers == nil || len(row.SeenRecords) > commercialRecordLimit || len(row.SeenCustomers) > commercialRecordLimit {
		return "", nil, ErrCommercial
	}
	for _, set := range [][]string{row.SeenRecords, row.SeenCustomers} {
		seen := map[string]bool{}
		for _, id := range set {
			if !hexDigest(id) || seen[id] {
				return "", nil, ErrCommercial
			}
			seen[id] = true
		}
	}
	if row.Snapshot.Records != len(row.SeenRecords) || row.Snapshot.PayingCustomerReferences != int64(len(row.SeenCustomers)) {
		return "", nil, ErrCommercial
	}
	if row.Complete && !validCommercialSnapshot(row.Snapshot, true) {
		return "", nil, ErrCommercial
	}
	return path, &row, nil
}
func commercialSalt() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func commercialView(ledger commercialLedger) CommercialView {
	return CommercialView{ProjectID: ledger.ProjectID, Configuration: ledger.Configuration, Status: ledger.Status, Snapshots: ledger.Snapshots, PrunedSnapshots: ledger.PrunedSnapshots, RefreshMinutes: 15, CollectionRecordLimit: commercialRecordLimit}
}
func (s *Server) CommercialSignals(ctx context.Context, actor management.ExecutionActorRef) (CommercialView, error) {
	if s.feedbackAccess(ctx, actor, false) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return CommercialView{}, err
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil {
		return CommercialView{}, err
	}
	if s.feedbackAccess(ctx, actor, false) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	return commercialView(ledger), nil
}
func commercialStatus(ledger *commercialLedger, key string) *CommercialSourceStatus {
	for i := range ledger.Status {
		if ledger.Status[i].SourceKey == key {
			return &ledger.Status[i]
		}
	}
	ledger.Status = append(ledger.Status, CommercialSourceStatus{SourceKey: key, State: "collecting"})
	return &ledger.Status[len(ledger.Status)-1]
}
func commercialSortCurrencies(values map[string]CommercialCurrency) []CommercialCurrency {
	out := make([]CommercialCurrency, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}
