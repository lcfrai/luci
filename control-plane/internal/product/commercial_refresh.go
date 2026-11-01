package product

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var errCommercialWindowLimit = errors.New("This collection reached its 100000-record operational bound. Save a shorter UTC window and start a new collection; partial totals are not published.")

type CommercialRefreshRequest struct {
	SourceKey       string `json:"sourceKey"`
	ResetCollection bool   `json:"resetCollection"`
}

func commercialDue(ledger commercialLedger, requested string, automatic bool, now time.Time) string {
	keys := commercialEnabledKeys(ledger)
	chosen := ""
	oldest := now.Add(time.Hour)
	for _, key := range keys {
		if requested != "" && key != requested {
			continue
		}
		var status *CommercialSourceStatus
		for i := range ledger.Status {
			if ledger.Status[i].SourceKey == key {
				status = &ledger.Status[i]
				break
			}
		}
		attempted := time.Time{}
		if status != nil && status.LastAttemptAt != "" {
			attempted, _ = time.Parse(time.RFC3339Nano, status.LastAttemptAt)
		}
		if automatic && status != nil && (status.State != "collecting" && now.Sub(attempted) < 15*time.Minute || status.State == "collecting" && now.Sub(attempted) < 10*time.Second) {
			continue
		}
		if chosen == "" || attempted.Before(oldest) {
			chosen, oldest = key, attempted
		}
	}
	return chosen
}
func (s *Server) commercialRouteGuard(ctx context.Context, p project.Project, ref management.ExecutionActorRef, revision int) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readContinuations(p, ref.UserID)
	if err != nil || rows.SavedRoute == nil || rows.SavedRoute.Revision != revision || !rows.SavedRoute.AutoDiscussFindings || rows.SavedRoute.Observations == nil || rows.SavedRoute.Observations.Remaining < 1 || !observationSelected(rows.SavedRoute.Observations.Kinds, "metrics") || observationOrigin(rows, ref) == "" || rows.RecipientRef.UserID != "" && rows.RecipientRef != ref {
		return management.ErrExecutionAuthority
	}
	return nil
}
func (s *Server) commercialReadGuard(ctx context.Context, p project.Project, ref management.ExecutionActorRef, key, digest string, automatic bool, routeRevision int) error {
	if s.commercialAccess(ctx, p, ref) != nil {
		return management.ErrExecutionAuthority
	}
	_, ledger, err := s.readCommercial(p)
	if err != nil || commercialSourceDigest(ledger, key) != digest || automatic && !ledger.Configuration.AutomaticReads {
		return ErrCommercial
	}
	if automatic && s.commercialRouteGuard(ctx, p, ref, routeRevision) != nil {
		return management.ErrExecutionAuthority
	}
	if strings.HasPrefix(key, "github:") {
		var repository string
		for _, r := range ledger.Configuration.Releases {
			if key == commercialReleaseKey(r.Repository, r.ReleaseID) {
				repository = r.Repository
			}
		}
		s.mu.RLock()
		root := s.config.ProjectsDir
		s.mu.RUnlock()
		bounded, cancel := context.WithTimeout(context.WithValue(ctx, journeyGitRootContextKey{}, root), 3*time.Second)
		defer cancel()
		raw, e := s.journeyGit(bounded, p.Path, "config", "--get-all", "remote.origin.url")
		actual, e2 := canonicalGitHubOrigin(strings.TrimSpace(raw))
		if e != nil || e2 != nil || repository == "" || actual != repository {
			return ErrCommercial
		}
	}
	return ctx.Err()
}
func newCommercialCollection(p project.Project, ledger commercialLedger, key string, now time.Time) (*commercialCollection, error) {
	salt, err := commercialSalt()
	if err != nil {
		return nil, err
	}
	row := &commercialCollection{Version: 1, ProjectID: p.ID, AccountScope: ledger.AccountScope, ProjectScope: ledger.ProjectScope, SourceKey: key, SourceDigest: commercialSourceDigest(ledger, key), DedupSalt: salt, SeenRecords: []string{}, SeenCustomers: []string{}}
	row.Snapshot = CommercialSnapshot{SourceKey: key, StartedAt: now.UTC().Format(time.RFC3339Nano), Currencies: []CommercialCurrency{}}
	if ledger.Configuration.Stripe != nil && key == commercialStripeKey(ledger.Configuration.Stripe.AccountID, ledger.Configuration.Stripe.Live, ledger.Configuration.StripeFilter) {
		source := ledger.Configuration.Stripe
		end := now.UTC().Truncate(24 * time.Hour)
		row.Snapshot.Provider = "stripe"
		row.Snapshot.Label = source.Name
		row.Snapshot.AccountID = source.AccountID
		row.Snapshot.Live = source.Live
		row.Snapshot.StripeFilter = ledger.Configuration.StripeFilter
		row.Snapshot.Attribution = "account-unallocated"
		if row.Snapshot.StripeFilter != nil {
			row.Snapshot.Attribution = "charge-metadata-filter"
		}
		row.Snapshot.PeriodStart = end.AddDate(0, 0, -ledger.Configuration.WindowDays).Format("2006-01-02")
		row.Snapshot.PeriodEndExclusive = end.Format("2006-01-02")
		row.Snapshot.Semantics = stripeCommercialSemantics
	} else {
		for _, source := range ledger.Configuration.Releases {
			if key == commercialReleaseKey(source.Repository, source.ReleaseID) {
				row.Snapshot.Provider = "github-release"
				row.Snapshot.Attribution = "original-release"
				row.Snapshot.Label = source.Label
				row.Snapshot.Repository = source.Repository
				row.Snapshot.ReleaseID = source.ReleaseID
				row.Snapshot.PublicationID = source.PublicationID
				row.Snapshot.ArtifactID = source.ArtifactID
				row.Snapshot.ArtifactDigest = source.ArtifactDigest
				row.Snapshot.Tag = source.Tag
				row.Snapshot.Semantics = githubCommercialSemantics
			}
		}
	}
	if row.SourceDigest == "" || !validCommercialSnapshot(row.Snapshot, false) {
		return nil, ErrCommercial
	}
	return row, nil
}
func commercialCompleted(row *commercialCollection) {
	row.Snapshot.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	row.Snapshot.Digest = commercialSnapshotDigest(row.Snapshot)
	row.Snapshot.ID = "commercial-" + row.Snapshot.Digest
}
func commercialCloneCollection(row *commercialCollection) commercialCollection {
	out := *row
	out.SeenRecords = append([]string{}, row.SeenRecords...)
	out.SeenCustomers = append([]string{}, row.SeenCustomers...)
	out.Snapshot.Currencies = append([]CommercialCurrency{}, row.Snapshot.Currencies...)
	if row.Snapshot.Reactions != nil {
		value := *row.Snapshot.Reactions
		out.Snapshot.Reactions = &value
	}
	if row.Snapshot.StripeFilter != nil {
		value := *row.Snapshot.StripeFilter
		out.Snapshot.StripeFilter = &value
	}
	return out
}
func commercialRetainSnapshot(ledger *commercialLedger, row CommercialSnapshot) {
	known := false
	for _, old := range ledger.Snapshots {
		known = known || old.ID == row.ID
	}
	if !known {
		ledger.Snapshots = append(ledger.Snapshots, row)
	}
	status := commercialStatus(ledger, row.SourceKey)
	status.LatestID = row.ID
	status.LastSuccessAt = time.Now().UTC().Format(time.RFC3339Nano)
	status.State = "ready"
	status.Records = row.Records
	commercialTrimHistory(ledger)
}
func commercialTrimHistory(ledger *commercialLedger) {
	latest := map[string]bool{}
	for _, s := range ledger.Status {
		latest[s.LatestID] = true
	}
	for len(ledger.Snapshots) > max(commercialHistoryLimit, len(ledger.Status)) {
		removed := false
		for i, old := range ledger.Snapshots {
			if !latest[old.ID] {
				ledger.Snapshots = append(ledger.Snapshots[:i], ledger.Snapshots[i+1:]...)
				ledger.PrunedSnapshots++
				removed = true
				break
			}
		}
		if !removed {
			break
		}
	}
}

// This is called by explicit UI reads and the existing authorized observation
// pass. It never creates a worker, timer, payment, publication or model request.
func (s *Server) refreshCommercial(ctx context.Context, ref management.ExecutionActorRef, input CommercialRefreshRequest, automatic bool, routeRevision int) (CommercialView, bool, error) {
	if s.feedbackAccess(ctx, ref, true) != nil {
		return CommercialView{}, false, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(ref.ProjectID)
	if err != nil {
		return CommercialView{}, false, err
	}
	parent := ctx
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ctx = bounded
	if !s.commercialRefreshMu.TryLock() {
		return CommercialView{}, false, ErrCommercial
	}
	defer s.commercialRefreshMu.Unlock()
	if !s.commercialMu.TryLock() {
		return CommercialView{}, false, ErrCommercial
	}
	locked := true
	defer func() {
		if locked {
			s.commercialMu.Unlock()
		}
	}()
	path, ledger, err := s.readCommercial(p)
	if err != nil {
		return CommercialView{}, false, err
	}
	if automatic && !ledger.Configuration.AutomaticReads {
		return commercialView(ledger), false, nil
	}
	key := commercialDue(ledger, input.SourceKey, automatic, time.Now().UTC())
	if key == "" {
		if input.SourceKey != "" {
			return CommercialView{}, false, ErrCommercial
		}
		return commercialView(ledger), false, nil
	}
	digest := commercialSourceDigest(ledger, key)
	guard := func(current context.Context) error {
		return s.commercialReadGuard(current, p, ref, key, digest, automatic, routeRevision)
	}
	if guard(ctx) != nil {
		return CommercialView{}, false, management.ErrExecutionAuthority
	}
	progressPath, collection, err := s.commercialProgress(p, ledger, key)
	if err != nil {
		return CommercialView{}, false, err
	}
	if collection != nil && collection.Complete {
		for _, previous := range ledger.Status {
			if previous.SourceKey == key && previous.State == "ready" && previous.LatestID == collection.Snapshot.ID {
				collection = nil
				break
			}
		}
	}
	if input.ResetCollection || collection == nil || collection.SourceKey != key || collection.SourceDigest != digest {
		collection, err = newCommercialCollection(p, ledger, key, time.Now())
		if err != nil {
			return CommercialView{}, false, err
		}
		if commercialWrite(progressPath, collection, 16<<20) != nil {
			return CommercialView{}, false, ErrCommercial
		}
	}
	status := commercialStatus(&ledger, key)
	status.LastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	status.State = "collecting"
	status.Records = collection.Snapshot.Records
	if guard(ctx) != nil || commercialWrite(path, ledger, 8<<20) != nil {
		return CommercialView{}, false, ErrCommercial
	}
	s.commercialMu.Unlock()
	locked = false
	ioctx := context.WithValue(context.WithValue(ctx, stripeCommercialGuardKey{}, guard), githubReleaseGuardKey{}, guard)
	var stripe stripeCommercialReader
	var github commercialGitHubReader
	var release CommercialReleaseSource
	if collection.Snapshot.Provider == "stripe" {
		stripe = s.stripeCommercialClient()
		account, e := stripe.Account(ioctx, ledger.StripeKey)
		if e != nil {
			err = e
		} else if account != collection.Snapshot.AccountID {
			err = ErrCommercial
		}
	} else {
		github = s.commercialGitHubClient()
		for _, r := range ledger.Configuration.Releases {
			if key == commercialReleaseKey(r.Repository, r.ReleaseID) {
				release = r
			}
		}
		if github == nil {
			err = ErrCommercial
		} else {
			actual, e := github.RepositoryID(ioctx, ledger.ReleaseCLI[key], release.Repository)
			if e != nil || actual != release.RepositoryID {
				err = ErrCommercial
			}
			if err == nil {
				commit, e := github.TagCommit(ioctx, ledger.ReleaseCLI[key], release.Repository, release.Tag)
				if e != nil || commit != release.Commit {
					err = ErrCommercial
				}
			}
		}
	}
	advanced := false
	// Page batches bound foreground and coordinator work. A crash before the
	// checkpoint may repeat read-only pages; totals/cursor commit together.
	for page := 0; err == nil && !collection.Complete && page < 10; page++ {
		deadline, _ := ctx.Deadline()
		if time.Until(deadline) < 2*time.Second {
			break
		}
		if guard(ioctx) != nil {
			err = management.ErrExecutionAuthority
			break
		}
		candidate := commercialCloneCollection(collection)
		if collection.Snapshot.Provider == "stripe" {
			start, _ := time.Parse("2006-01-02", collection.Snapshot.PeriodStart)
			end, _ := time.Parse("2006-01-02", collection.Snapshot.PeriodEndExclusive)
			value, e := stripe.Charges(ioctx, ledger.StripeKey, start, end, collection.Cursor)
			err = e
			if err == nil {
				err = collectStripePage(&candidate, value)
			}
		} else {
			value, e := github.CommercialPage(ioctx, ledger.ReleaseCLI[key], release, collection.Page+1)
			err = e
			if err == nil {
				err = collectGitHubCommercialPage(&candidate, value)
			}
		}
		if err != nil {
			break
		}
		collection = &candidate
		advanced = true
		if collection.Complete {
			commercialCompleted(collection)
		}
	}
	// The read sub-budget may expire before the caller's original budget. Only
	// that still-live original context can checkpoint completed pages; no renewal.
	checkpoint := ctx
	if ctx.Err() != nil {
		if parent.Err() != nil {
			return CommercialView{}, false, management.ErrExecutionAuthority
		}
		checkpoint = parent
		if err == nil {
			err = ctx.Err()
		}
	}
	if !s.commercialMu.TryLock() {
		return CommercialView{}, false, ErrCommercial
	}
	locked = true
	_, current, readErr := s.readCommercial(p)
	if readErr != nil || guard(checkpoint) != nil {
		return CommercialView{}, false, management.ErrExecutionAuthority
	}
	ledger = current
	// Only completed pages are checkpointed. Failed/cancelled bodies contribute
	// neither a cursor advance nor a public partial aggregate.
	if advanced || collection.Complete {
		if commercialWrite(progressPath, collection, 16<<20) != nil {
			return CommercialView{}, false, ErrCommercial
		}
	}
	status = commercialStatus(&ledger, key)
	status.Records = collection.Snapshot.Records
	if err != nil {
		status.State = "read-failed"
		if errors.Is(err, errCommercialWindowLimit) {
			status.State = "window-limit"
		}
		if commercialWrite(path, ledger, 8<<20) != nil {
			return CommercialView{}, false, ErrCommercial
		}
		return commercialView(ledger), advanced, err
	}
	if collection.Complete {
		if !validCommercialSnapshot(collection.Snapshot, true) {
			return CommercialView{}, false, ErrCommercial
		}
		commercialRetainSnapshot(&ledger, collection.Snapshot)
	}
	if commercialWrite(path, ledger, 8<<20) != nil {
		return CommercialView{}, false, ErrCommercial
	}
	if guard(checkpoint) != nil {
		return CommercialView{}, false, management.ErrExecutionAuthority
	}
	if !collection.Complete {
		s.WakeCoordinator()
	}
	return commercialView(ledger), advanced, nil
}
func (s *Server) RefreshCommercialSignals(ctx context.Context, ref management.ExecutionActorRef, input CommercialRefreshRequest) (CommercialView, error) {
	value, _, err := s.refreshCommercial(ctx, ref, input, false, 0)
	return value, err
}
func (s *Server) refreshCommercialForCoordinator(ctx context.Context, p project.Project, ref management.ExecutionActorRef, routeRevision int) bool {
	_, advanced, _ := s.refreshCommercial(ctx, ref, CommercialRefreshRequest{}, true, routeRevision)
	return advanced
}
