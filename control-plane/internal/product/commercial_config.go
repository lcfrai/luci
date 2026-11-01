package product

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type CommercialStripeSave struct {
	ExpectedRevision          int    `json:"expectedRevision"`
	Name                      string `json:"name"`
	APIKey                    string `json:"apiKey"`
	Remove                    bool   `json:"remove"`
	AcknowledgePrivateStorage bool   `json:"acknowledgePrivateStorage"`
}
type CommercialConfigurationSave struct {
	ExpectedRevision   int                     `json:"expectedRevision"`
	StripeEnabled      bool                    `json:"stripeEnabled"`
	StripeFilter       *CommercialStripeFilter `json:"stripeFilter"`
	WindowDays         int                     `json:"windowDays"`
	PublicationIDs     []string                `json:"publicationIds"`
	AutomaticReads     bool                    `json:"automaticReads"`
	AcknowledgeSharing bool                    `json:"acknowledgeSharing"`
}

func (s *Server) commercialAccess(ctx context.Context, p project.Project, actor management.ExecutionActorRef) error {
	if s.feedbackAccess(ctx, actor, true) != nil {
		return management.ErrExecutionAuthority
	}
	current, err := s.resolveProject(actor.ProjectID)
	if err != nil || current.ID != p.ID || current.Path != p.Path {
		return ErrCommercial
	}
	return nil
}
func (s *Server) SaveCommercialStripe(ctx context.Context, actor management.ExecutionActorRef, input CommercialStripeSave) (CommercialView, error) {
	if s.feedbackAccess(ctx, actor, true) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return CommercialView{}, err
	}
	if !s.commercialMu.TryLock() {
		return CommercialView{}, ErrCommercial
	}
	defer s.commercialMu.Unlock()
	path, ledger, err := s.readCommercial(p)
	if err != nil || input.ExpectedRevision != ledger.Configuration.Revision {
		return CommercialView{}, ErrCommercial
	}
	previous := ledger
	if input.Remove {
		if input.APIKey != "" {
			return CommercialView{}, ErrCommercial
		}
		ledger.Configuration.Stripe = nil
		ledger.StripeKey = ""
		ledger.Configuration.StripeEnabled = false
		ledger.Configuration.StripeFilter = nil
	} else {
		input.Name = strings.TrimSpace(input.Name)
		mode := stripeKeyMode(input.APIKey)
		if !input.AcknowledgePrivateStorage || mode == "" || input.Name == "" || len(input.Name) > 120 || !validArtifactStoredText(input.Name) {
			return CommercialView{}, ErrCommercial
		}
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		ioctx := context.WithValue(bounded, stripeCommercialGuardKey{}, func(c context.Context) error { return s.commercialAccess(c, p, actor) })
		account, readErr := s.stripeCommercialClient().Account(ioctx, input.APIKey)
		if readErr != nil {
			return CommercialView{}, readErr
		}
		if s.commercialAccess(ctx, p, actor) != nil {
			return CommercialView{}, management.ErrExecutionAuthority
		}
		id, revision := "", 1
		if ledger.Configuration.Stripe != nil {
			id = ledger.Configuration.Stripe.ID
			revision = ledger.Configuration.Stripe.Revision + 1
		} else {
			id, err = kubeOpaqueID("stripe-")
			if err != nil {
				return CommercialView{}, err
			}
		}
		ledger.Configuration.Stripe = &CommercialStripeConnection{ID: id, Name: input.Name, AccountID: account, Live: mode == "live", Revision: revision}
		ledger.StripeKey = input.APIKey
		// A different credential/account is shown before charges are enabled.
		ledger.Configuration.StripeEnabled = false
	}
	ledger.Configuration.Revision++
	if commercialAssignGenerations(&ledger, previous) != nil {
		return CommercialView{}, ErrCommercial
	}
	commercialInvalidateChangedSources(&ledger, previous)
	commercialPruneStatus(&ledger)
	if !validCommercialConfig(ledger.Configuration, ledger.StripeKey) || s.commercialAccess(ctx, p, actor) != nil {
		return CommercialView{}, ErrCommercial
	}
	if err = commercialWrite(path, ledger, 8<<20); err != nil {
		return CommercialView{}, err
	}
	if s.commercialAccess(ctx, p, actor) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	return commercialView(ledger), nil
}
func (s *Server) SaveCommercialConfiguration(ctx context.Context, actor management.ExecutionActorRef, input CommercialConfigurationSave) (CommercialView, error) {
	if s.feedbackAccess(ctx, actor, true) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return CommercialView{}, err
	}
	if input.PublicationIDs == nil || len(input.PublicationIDs) > commercialSourceSafetyLimit || !validCommercialStripeFilter(input.StripeFilter) || input.WindowDays < 1 || input.WindowDays > 90 || (input.StripeEnabled || len(input.PublicationIDs) > 0) && !input.AcknowledgeSharing {
		return CommercialView{}, ErrCommercial
	}
	if !s.commercialMu.TryLock() {
		return CommercialView{}, ErrCommercial
	}
	defer s.commercialMu.Unlock()
	path, ledger, err := s.readCommercial(p)
	if err != nil || input.ExpectedRevision != ledger.Configuration.Revision || input.StripeEnabled && ledger.Configuration.Stripe == nil {
		return CommercialView{}, ErrCommercial
	}
	previous := ledger
	ids := append([]string{}, input.PublicationIDs...)
	sort.Strings(ids)
	seen := map[string]bool{}
	sources := []CommercialReleaseSource{}
	identities := map[string]GitHubIssueCLIIdentity{}
	runner := s.commercialGitHubClient()
	for _, id := range ids {
		if !validGitHubPublicationID(id) || seen[id] || runner == nil {
			return CommercialView{}, ErrCommercial
		}
		seen[id] = true
		source := CommercialReleaseSource{}
		for _, old := range ledger.Configuration.Releases {
			if old.PublicationID == id {
				source = old
			}
		}
		if source.PublicationID == "" {
			source, err = s.commercialReleaseBinding(p, actor, id)
			if err != nil {
				return CommercialView{}, err
			}
		}
		cli, e := runner.Identity(ctx)
		if e != nil || s.commercialAccess(ctx, p, actor) != nil {
			return CommercialView{}, ErrCommercial
		}
		sources = append(sources, source)
		identities[commercialReleaseKey(source.Repository, source.ReleaseID)] = cli
	}
	ledger.Configuration.Revision++
	ledger.Configuration.StripeEnabled = input.StripeEnabled
	ledger.Configuration.StripeFilter = input.StripeFilter
	ledger.Configuration.WindowDays = input.WindowDays
	ledger.Configuration.Releases = sources
	ledger.Configuration.AutomaticReads = input.AutomaticReads && (input.StripeEnabled || len(sources) > 0)
	ledger.ReleaseCLI = identities
	if commercialAssignGenerations(&ledger, previous) != nil {
		return CommercialView{}, ErrCommercial
	}
	commercialInvalidateChangedSources(&ledger, previous)
	commercialPruneStatus(&ledger)
	if !validCommercialConfig(ledger.Configuration, ledger.StripeKey) || s.commercialAccess(ctx, p, actor) != nil {
		return CommercialView{}, ErrCommercial
	}
	if commercialWrite(path, ledger, 8<<20) != nil {
		return CommercialView{}, ErrCommercial
	}
	if s.commercialAccess(ctx, p, actor) != nil {
		return CommercialView{}, management.ErrExecutionAuthority
	}
	s.WakeCoordinator()
	return commercialView(ledger), nil
}
func commercialEnabledKeys(ledger commercialLedger) []string {
	keys := []string{}
	if ledger.Configuration.StripeEnabled && ledger.Configuration.Stripe != nil {
		r := ledger.Configuration.Stripe
		keys = append(keys, commercialStripeKey(r.AccountID, r.Live, ledger.Configuration.StripeFilter))
	}
	for _, r := range ledger.Configuration.Releases {
		keys = append(keys, commercialReleaseKey(r.Repository, r.ReleaseID))
	}
	return keys
}
func commercialKnownKeys(ledger commercialLedger) map[string]bool {
	keys := map[string]bool{}
	if ledger.Configuration.Stripe != nil {
		r := ledger.Configuration.Stripe
		keys[commercialStripeKey(r.AccountID, r.Live, ledger.Configuration.StripeFilter)] = true
	}
	for _, r := range ledger.Configuration.Releases {
		keys[commercialReleaseKey(r.Repository, r.ReleaseID)] = true
	}
	return keys
}
func commercialPruneStatus(ledger *commercialLedger) {
	keys := commercialKnownKeys(*ledger)
	out := []CommercialSourceStatus{}
	for _, r := range ledger.Status {
		if keys[r.SourceKey] {
			out = append(out, r)
		}
	}
	ledger.Status = out
	commercialTrimHistory(ledger)
}
func commercialSourceDescriptorDigest(ledger commercialLedger, key string) string {
	if ledger.Configuration.StripeEnabled && ledger.Configuration.Stripe != nil {
		r := ledger.Configuration.Stripe
		if key == commercialStripeKey(r.AccountID, r.Live, ledger.Configuration.StripeFilter) {
			return reviewDigest(struct {
				Connection CommercialStripeConnection
				Key        string
				Days       int
				Filter     *CommercialStripeFilter
			}{*r, ledger.StripeKey, ledger.Configuration.WindowDays, ledger.Configuration.StripeFilter})
		}
	}
	for _, r := range ledger.Configuration.Releases {
		if key == commercialReleaseKey(r.Repository, r.ReleaseID) {
			return reviewDigest(struct {
				Source CommercialReleaseSource
				CLI    GitHubIssueCLIIdentity
			}{r, ledger.ReleaseCLI[key]})
		}
	}
	return ""
}

// Completed older cohorts remain in history. A changed source/filter/window or
// credential identity cannot remain the current context while a new read waits.
func commercialInvalidateChangedSources(ledger *commercialLedger, previous commercialLedger) {
	out := []CommercialSourceStatus{}
	for _, status := range ledger.Status {
		old := commercialSourceDigest(previous, status.SourceKey)
		current := commercialSourceDigest(*ledger, status.SourceKey)
		if old != "" && old == current {
			out = append(out, status)
		}
	}
	ledger.Status = out
}

// A fresh selection cannot resurrect an old completed private cursor. Generations
// are private, bounded to enabled sources and published atomically with consent.
// Unrelated config edits preserve a continuously enabled descriptor's generation.
func commercialAssignGenerations(ledger *commercialLedger, previous commercialLedger) error {
	values := map[string]string{}
	for _, key := range commercialEnabledKeys(*ledger) {
		generation := previous.CollectionGenerations[key]
		if !hexDigest(generation) || commercialSourceDescriptorDigest(*ledger, key) != commercialSourceDescriptorDigest(previous, key) {
			var err error
			generation, err = commercialSalt()
			if err != nil {
				return err
			}
		}
		values[key] = generation
	}
	ledger.CollectionGenerations = values
	return nil
}
func commercialSourceDigest(ledger commercialLedger, key string) string {
	descriptor := commercialSourceDescriptorDigest(ledger, key)
	generation := ledger.CollectionGenerations[key]
	if descriptor == "" || !hexDigest(generation) {
		return ""
	}
	return reviewDigest(struct {
		Descriptor string
		Generation string
	}{descriptor, generation})
}
