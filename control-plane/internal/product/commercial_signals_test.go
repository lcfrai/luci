package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func commercialBool(v bool) *bool  { return &v }
func commercialInt(v int64) *int64 { return &v }
func commercialTestCharge(id string, end time.Time) stripeCommercialCharge {
	return stripeCommercialCharge{ID: id, Object: "charge", Created: end.Add(-time.Hour).Unix(), Currency: "usd", CapturedAmount: commercialInt(1000), RefundedAmount: commercialInt(100), Paid: commercialBool(true), Captured: commercialBool(true), Disputed: commercialBool(false), Live: commercialBool(false), Status: "succeeded", Customer: json.RawMessage(`"cus_private_one"`), Metadata: map[string]string{"project": "one", "private_customer_note": "must-not-persist"}}
}
func commercialTestCollection(t *testing.T) *commercialCollection {
	t.Helper()
	s, p, _, actor, _, _, _ := feedbackFixture(t)
	_ = s
	_ = actor
	ledger := commercialLedger{AccountScope: "a", ProjectScope: "p", Configuration: CommercialConfiguration{Stripe: &CommercialStripeConnection{ID: "stripe-one", Name: "Product receipts", AccountID: "acct_one", Revision: 1}, StripeEnabled: true, StripeFilter: &CommercialStripeFilter{Key: "project", Value: "one"}, WindowDays: 30, Releases: []CommercialReleaseSource{}}, StripeKey: "rk_test_" + strings.Repeat("a", 32)}
	if err := commercialAssignGenerations(&ledger, commercialLedger{}); err != nil {
		t.Fatal(err)
	}
	row, err := newCommercialCollection(p, ledger, commercialStripeKey("acct_one", false, ledger.Configuration.StripeFilter), time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return row
}
func TestCommercialStripeCohortAttributionAndPageAtomicity(t *testing.T) {
	row := commercialTestCollection(t)
	end, _ := time.Parse("2006-01-02", row.Snapshot.PeriodEndExclusive)
	first := commercialTestCharge("charge_one", end)
	other := commercialTestCharge("charge_other_project", end)
	other.Metadata["project"] = "two"
	second := commercialTestCharge("charge_two", end)
	second.Currency = "eur"
	if err := collectStripePage(row, stripeCommercialPage{More: commercialBool(false), Data: []stripeCommercialCharge{first, other, second}}); err != nil {
		t.Fatal(err)
	}
	if row.Snapshot.Records != 3 || row.Snapshot.MatchedCharges != 2 || row.Snapshot.PayingCustomerReferences != 1 || len(row.Snapshot.Currencies) != 2 || row.Snapshot.Currencies[0].CapturedMinor != 1000 {
		t.Fatal("project attribution/currency/customer cohort changed", row.Snapshot)
	}
	raw, _ := json.Marshal(row.Snapshot)
	if strings.Contains(string(raw), "cus_private") || strings.Contains(string(raw), "must-not-persist") {
		t.Fatal("customer metadata leaked into public snapshot")
	}
	original := commercialCloneCollection(row)
	candidate := commercialCloneCollection(row)
	late := commercialTestCharge("charge_late", end)
	broken := commercialTestCharge("charge_broken", end)
	broken.CapturedAmount = commercialInt(-1)
	if err := collectStripePage(&candidate, stripeCommercialPage{More: commercialBool(false), Data: []stripeCommercialCharge{late, broken}}); err == nil {
		t.Fatal("malformed later row accepted")
	}
	if !reflect.DeepEqual(*row, original) {
		t.Fatal("discarded partial page mutated original cursor/totals")
	}
	candidate.SeenRecords[0] = "changed"
	candidate.Snapshot.Currencies[0].CapturedMinor = 0
	candidate.Snapshot.StripeFilter.Value = "other"
	if !reflect.DeepEqual(*row, original) {
		t.Fatal("page clone shares mutable source data")
	}
}

type commercialRoundTrip func(*http.Request) (*http.Response, error)

func (f commercialRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCommercialStripeFixedReadBoundaryAndLateGrant(t *testing.T) {
	key := "rk_test_" + strings.Repeat("a", 32)
	allowed := true
	calls := 0
	client := stripeCommercialHTTP{transport: commercialRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		user, password, ok := r.BasicAuth()
		if r.Method != "GET" || r.URL.Scheme != "https" || r.URL.Host != "api.stripe.com" || r.URL.Path != "/v1/charges" || r.URL.Query().Get("starting_after") != "charge_cursor" || r.URL.Query().Get("limit") != "100" || r.Header.Get("Stripe-Version") != stripeCommercialVersion || !ok || user != key || password != "" {
			t.Fatal("fixed provider boundary changed")
		}
		allowed = false
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"object":"list","url":"/v1/charges","has_more":false,"data":[]}`)), Header: make(http.Header)}, nil
	})}
	ctx := context.WithValue(context.Background(), stripeCommercialGuardKey{}, func(context.Context) error {
		if !allowed {
			return management.ErrExecutionAuthority
		}
		return nil
	})
	if _, err := client.Charges(ctx, key, time.Now().Add(-24*time.Hour), time.Now(), "charge_cursor"); err == nil || calls != 1 {
		t.Fatal("late revocation or request count ignored")
	}
	if stripeKeyMode("sk_test_"+strings.Repeat("a", 32)) != "" {
		t.Fatal("unrestricted Stripe key accepted")
	}
}

type commercialStripeFixture struct {
	Calls       int
	During      func()
	EndAfter    int
	Revise      int64
	FirstCursor string
}

func (f *commercialStripeFixture) Account(context.Context, string) (string, error) {
	return "acct_one", nil
}
func (f *commercialStripeFixture) Charges(_ context.Context, _ string, _ time.Time, end time.Time, cursor string) (stripeCommercialPage, error) {
	f.Calls++
	if f.Calls == 1 {
		f.FirstCursor = cursor
	}
	index := 1
	if cursor != "" {
		value, err := strconv.Atoi(strings.TrimPrefix(cursor, "charge_"))
		if err != nil {
			return stripeCommercialPage{}, err
		}
		index = value + 1
	}
	if f.During != nil {
		f.During()
	}
	row := commercialTestCharge(fmt.Sprintf("charge_%d", index), end)
	row.CapturedAmount = commercialInt(1000 + f.Revise)
	return stripeCommercialPage{More: commercialBool(index < f.EndAfter), Data: []stripeCommercialCharge{row}}, nil
}
func configureCommercialFixture(t *testing.T, s *Server, ctx context.Context, actor management.ExecutionActorRef, automatic bool) CommercialView {
	t.Helper()
	view, err := s.SaveCommercialStripe(ctx, actor, CommercialStripeSave{ExpectedRevision: 0, Name: "Project Stripe", APIKey: "rk_test_" + strings.Repeat("a", 32), AcknowledgePrivateStorage: true})
	if err != nil {
		t.Fatal(err)
	}
	view, err = s.SaveCommercialConfiguration(ctx, actor, CommercialConfigurationSave{ExpectedRevision: view.Configuration.Revision, StripeEnabled: true, StripeFilter: &CommercialStripeFilter{Key: "project", Value: "one"}, WindowDays: 30, PublicationIDs: []string{}, AutomaticReads: automatic, AcknowledgeSharing: true})
	if err != nil {
		t.Fatal(err)
	}
	return view
}
func TestCommercialResumesPersistedCursorAndCommitsOnlyExhaustedCollection(t *testing.T) {
	s, p, ctx, actor, _, _, _ := feedbackFixture(t)
	first := &commercialStripeFixture{EndAfter: 11}
	s.commercialStripe = first
	view := configureCommercialFixture(t, s, ctx, actor, false)
	key := commercialStripeKey(view.Configuration.Stripe.AccountID, false, view.Configuration.StripeFilter)
	view, err := s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{SourceKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if first.Calls != 10 || len(view.Snapshots) != 0 || view.Status[0].State != "collecting" {
		t.Fatal("partial cohort published or batch bound changed", first.Calls, view)
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil {
		t.Fatal(err)
	}
	_, stored, err := s.commercialProgress(p, ledger, key)
	if err != nil || stored.Cursor != "charge_10" {
		t.Fatal("durable cursor missing", err)
	}
	// A new reader has no in-memory cursor: the persisted original owns resume.
	second := &commercialStripeFixture{EndAfter: 11}
	s.commercialStripe = second
	view, err = s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{SourceKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if second.FirstCursor != "charge_10" || second.Calls != 1 || len(view.Snapshots) != 1 || view.Snapshots[0].MatchedCharges != 11 || view.Snapshots[0].Currencies[0].CapturedMinor != 11000 {
		t.Fatal("resumed cohort wrong", view)
	}
	before := view.Snapshots[0].Digest
	changed := view.Snapshots[0]
	changed.StartedAt = "2026-01-01T00:00:00Z"
	changed.ObservedAt = "2026-01-02T00:00:00Z"
	if commercialSnapshotDigest(changed) != before {
		t.Fatal("observation timestamps created a false new financial result")
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "rk_test_") || strings.Contains(string(raw), "dedupSalt") || strings.Contains(string(raw), "cus_private_one") {
		t.Fatal("private material entered view")
	}
}
func TestCommercialPauseDuringProviderReadWithdrawsSnapshotWithoutOverwritingConfig(t *testing.T) {
	s, p, ctx, actor, _, _, _ := feedbackFixture(t)
	provider := &commercialStripeFixture{EndAfter: 1}
	s.commercialStripe = provider
	view := configureCommercialFixture(t, s, ctx, actor, false)
	provider.During = func() {
		_, err := s.SaveCommercialConfiguration(ctx, actor, CommercialConfigurationSave{ExpectedRevision: view.Configuration.Revision, StripeEnabled: false, WindowDays: 30, PublicationIDs: []string{}})
		if err != nil {
			t.Fatal("pause blocked behind provider I/O", err)
		}
	}
	if _, err := s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{}); err == nil {
		t.Fatal("late pause accepted old snapshot")
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil || ledger.Configuration.StripeEnabled || len(ledger.Snapshots) != 0 {
		t.Fatal("paused configuration overwritten", err)
	}
}
func TestCommercialCurrentReadSecretRevocationAndUnallocatedExclusion(t *testing.T) {
	s, p, ctx, actor, _, _, allowed := feedbackFixture(t)
	provider := &commercialStripeFixture{EndAfter: 1}
	s.commercialStripe = provider
	configureCommercialFixture(t, s, ctx, actor, false)
	provider.During = func() { *allowed = false }
	if _, err := s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{}); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("late Secrets revocation did not withdraw read", err)
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil || len(ledger.Snapshots) != 0 {
		t.Fatal("revoked read published", err)
	}
	row := commercialTestCollection(t)
	row.Snapshot.StripeFilter = nil
	row.Snapshot.Attribution = "account-unallocated"
	row.SourceKey = commercialStripeKey("acct_one", false, nil)
	row.Snapshot.SourceKey = row.SourceKey
	row.Complete = true
	commercialCompleted(row)
	ledger.Configuration.StripeFilter = nil
	ledger.Configuration.StripeEnabled = true
	ledger.Status = []CommercialSourceStatus{{SourceKey: row.SourceKey, LatestID: row.Snapshot.ID, State: "ready"}}
	ledger.Snapshots = []CommercialSnapshot{row.Snapshot}
	captured := commercialContextForLedger(ledger)
	if captured == nil || captured.UnallocatedOmitted != 1 || len(captured.Records) != 0 {
		t.Fatal("account-wide totals became project evidence")
	}
}
func TestCommercialCoordinatorSourceConsentAndFairResumableCadence(t *testing.T) {
	ledger := commercialLedger{Configuration: CommercialConfiguration{Stripe: &CommercialStripeConnection{AccountID: "acct_one"}, StripeEnabled: true, WindowDays: 30, Releases: []CommercialReleaseSource{}}}
	key := commercialStripeKey("acct_one", false, nil)
	now := time.Now().UTC()
	ledger.Status = []CommercialSourceStatus{{SourceKey: key, State: "collecting", LastAttemptAt: now.Add(-11 * time.Second).Format(time.RFC3339Nano)}}
	if commercialDue(ledger, "", true, now) != key {
		t.Fatal("pending collection incorrectly waited15minutes")
	}
	ledger.Status[0].State = "ready"
	if commercialDue(ledger, "", true, now) != "" {
		t.Fatal("completed-source interval ignored")
	}
	if commercialDue(ledger, key, false, now) != key {
		t.Fatal("explicit refresh blocked by automatic cadence")
	}
	// An enabled read source alone is not model/automatic-route authority.
	s, _, ctx, actor, _, _, _ := feedbackFixture(t)
	provider := &commercialStripeFixture{EndAfter: 1}
	s.commercialStripe = provider
	configureCommercialFixture(t, s, ctx, actor, true)
	if _, _, err := s.refreshCommercial(ctx, actor, CommercialRefreshRequest{}, true, 1); err == nil || provider.Calls != 0 {
		t.Fatal("missing original saved metrics route still read provider")
	}
}
func TestCommercialExistingObservationPassRefreshesAndRetainsOneFinding(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	original := coordinatorReadyFixture(t, s, p, ctx, req)
	s.SetProjectFeedbackAuthorizer(func(_ context.Context, ref management.ExecutionActorRef, _ bool) error {
		if ref != original.ExecutionActor {
			return management.ErrExecutionAuthority
		}
		return nil
	})
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	provider := &commercialStripeFixture{EndAfter: 1}
	s.commercialStripe = provider
	configureCommercialFixture(t, s, ctx, original.ExecutionActor, true)
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), AutoDiscussFindings: true, Observations: &CoordinatorObservationRoute{Kinds: []string{"metrics"}, TelemetryIDs: []string{}, Remaining: 2, Last: map[string]string{"metrics": strings.Repeat("b", 64)}}}
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	changed, err := s.refreshCoordinatorObservations(ctx, p, rows, coordinatorFixtureReauthorize)
	if err != nil || !changed || provider.Calls != 1 {
		t.Fatal("existing authorized pass did not complete source read", changed, err, provider.Calls)
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil || len(ledger.Snapshots) != 1 {
		t.Fatal("complete provider snapshot not retained", err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Findings) != 1 || rows.Findings[0].Observation == nil || rows.Findings[0].Observation.Kind != "metrics" || rows.SavedRoute.Observations.Remaining != 1 {
		t.Fatal("ordinary metrics finding/budget join missing", rows, err)
	}
	if changed, err = s.refreshCoordinatorObservations(ctx, p, rows, coordinatorFixtureReauthorize); err != nil || changed || provider.Calls != 1 {
		t.Fatal("same interval bought another read or finding", changed, err, provider.Calls)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || view.Context.CommercialSignals == nil || len(view.Context.CommercialSignals.Records) != 1 {
		t.Fatal("normalized snapshot missing from captured project context", err)
	}
}

func TestCommercialChangedWindowWithdrawsCurrentCohortAndRetainsHistory(t *testing.T) {
	s, p, ctx, actor, _, _, _ := feedbackFixture(t)
	s.commercialStripe = &commercialStripeFixture{EndAfter: 1}
	view := configureCommercialFixture(t, s, ctx, actor, false)
	view, err := s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{})
	if err != nil || len(view.Snapshots) != 1 {
		t.Fatal(err)
	}
	before := view.Snapshots[0].ID
	view, err = s.SaveCommercialConfiguration(ctx, actor, CommercialConfigurationSave{ExpectedRevision: view.Configuration.Revision, StripeEnabled: true, StripeFilter: &CommercialStripeFilter{Key: "project", Value: "one"}, WindowDays: 7, PublicationIDs: []string{}, AcknowledgeSharing: true})
	if err != nil || len(view.Snapshots) != 1 || view.Snapshots[0].ID != before || len(view.Status) != 0 {
		t.Fatal("changed window hid history or retained current binding", view, err)
	}
	_, ledger, err := s.readCommercial(p)
	captured := commercialContextForLedger(ledger)
	if err != nil || captured == nil || len(captured.Records) != 0 || captured.PendingSources != 1 {
		t.Fatal("previous cohort remained current project evidence", captured, err)
	}
}

func TestCommercialOriginalReleaseAssetPagesRemainCountersNotRevenue(t *testing.T) {
	row := commercialTestCollection(t)
	row.Snapshot = CommercialSnapshot{SourceKey: "github:owner/repo:77", Provider: "github-release", Label: "Product launch", Repository: "owner/repo", ReleaseID: 77, PublicationID: "gh-release-one", ArtifactID: "launch-one", ArtifactDigest: strings.Repeat("a", 64), Tag: "v1.0.0", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Currencies: []CommercialCurrency{}, Attribution: "original-release", Semantics: githubCommercialSemantics}
	row.SourceKey = row.Snapshot.SourceKey
	if err := collectGitHubCommercialPage(row, commercialGitHubPage{Assets: []commercialGitHubAsset{{ID: 11, Downloads: commercialInt(5)}}, More: true}); err != nil {
		t.Fatal(err)
	}
	if err := collectGitHubCommercialPage(row, commercialGitHubPage{Assets: []commercialGitHubAsset{{ID: 11, Downloads: commercialInt(8)}, {ID: 12, Downloads: commercialInt(3)}}, Reactions: commercialInt(0)}); err != nil {
		t.Fatal(err)
	}
	commercialCompleted(row)
	if !row.Complete || row.Snapshot.Records != 2 || row.Snapshot.AssetDownloads != 8 || row.Snapshot.Reactions == nil || *row.Snapshot.Reactions != 0 || len(row.Snapshot.Currencies) != 0 || !validCommercialSnapshot(row.Snapshot, true) {
		t.Fatal("asset overlap was double counted or release counters acquired money semantics", row.Snapshot)
	}
	if err := collectGitHubCommercialPage(row, commercialGitHubPage{Assets: []commercialGitHubAsset{}, More: true}); err == nil {
		t.Fatal("nonadvancing release page accepted")
	}
}

func TestCommercialReselectionCannotPromoteOldCompletedProgress(t *testing.T) {
	t.Run("Stripe disabled then re-enabled", func(t *testing.T) {
		s, _, ctx, actor, _, _, _ := feedbackFixture(t)
		provider := &commercialStripeFixture{EndAfter: 1}
		s.commercialStripe = provider
		view := configureCommercialFixture(t, s, ctx, actor, false)
		view, err := s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{})
		if err != nil {
			t.Fatal(err)
		}
		original := view.Snapshots[0]
		input := CommercialConfigurationSave{ExpectedRevision: view.Configuration.Revision, StripeEnabled: false, StripeFilter: &CommercialStripeFilter{Key: "project", Value: "one"}, WindowDays: 30, PublicationIDs: []string{}}
		view, err = s.SaveCommercialConfiguration(ctx, actor, input)
		if err != nil {
			t.Fatal(err)
		}
		input.ExpectedRevision = view.Configuration.Revision
		input.StripeEnabled = true
		input.AcknowledgeSharing = true
		view, err = s.SaveCommercialConfiguration(ctx, actor, input)
		if err != nil || len(view.Status) != 0 {
			t.Fatal("reselection kept old current result", err)
		}
		provider.Revise = 10
		view, err = s.RefreshCommercialSignals(ctx, actor, CommercialRefreshRequest{})
		if err != nil || provider.Calls != 2 || len(view.Snapshots) != 2 || view.Snapshots[1].ID == original.ID {
			t.Fatal("old complete cursor was promoted instead of rereading", view, err, provider.Calls)
		}
	})
	t.Run("release removed then re-added", func(t *testing.T) {
		source := CommercialReleaseSource{PublicationID: "gh-release-one", Repository: "owner/repo", RepositoryID: 1, ReleaseID: 77, AuthorID: 2, Tag: "v1.0.0", Commit: strings.Repeat("a", 40), ArtifactID: "launch-one", ArtifactDigest: strings.Repeat("b", 64), Label: "Product launch"}
		key := commercialReleaseKey(source.Repository, source.ReleaseID)
		initial := commercialLedger{Configuration: CommercialConfiguration{Releases: []CommercialReleaseSource{source}}, ReleaseCLI: map[string]GitHubIssueCLIIdentity{key: {Path: "fixture-gh", Bytes: 1, SHA256: strings.Repeat("c", 64)}}}
		if err := commercialAssignGenerations(&initial, commercialLedger{}); err != nil {
			t.Fatal(err)
		}
		before := commercialSourceDigest(initial, key)
		removed := initial
		removed.Configuration.Releases = []CommercialReleaseSource{}
		if err := commercialAssignGenerations(&removed, initial); err != nil {
			t.Fatal(err)
		}
		restored := initial
		if err := commercialAssignGenerations(&restored, removed); err != nil {
			t.Fatal(err)
		}
		if before == commercialSourceDigest(restored, key) || len(removed.CollectionGenerations) != 0 || len(restored.CollectionGenerations) != 1 {
			t.Fatal("re-added release adopted old private collection generation")
		}
		unchanged := restored
		unchanged.Configuration.AutomaticReads = true
		if err := commercialAssignGenerations(&unchanged, restored); err != nil || commercialSourceDigest(unchanged, key) != commercialSourceDigest(restored, key) {
			t.Fatal("unrelated setting restarted pending source", err)
		}
	})
}
