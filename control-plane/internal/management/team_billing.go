package management

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
	"github.com/shopspring/decimal"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TeamBilling is private merchant authority, distinct from the operator's
// independent service suspension. It contains no card data or Stripe secrets.
type TeamBilling struct {
	ConfigID          string                `json:"configId"`
	Revision          int                   `json:"revision"`
	CustomerID        string                `json:"customerId,omitempty"`
	SubscriptionID    string                `json:"subscriptionId,omitempty"`
	Status            string                `json:"status"`
	PeriodEnd         int64                 `json:"periodEnd,omitempty"`
	CancelAtPeriodEnd bool                  `json:"cancelAtPeriodEnd,omitempty"`
	Quote             *BillingQuote         `json:"quote,omitempty"`
	Reservation       *BillingReservation   `json:"reservation,omitempty"`
	Events            []billingEventReceipt `json:"events,omitempty"`
}
type BillingPrice struct {
	ID            string `json:"priceId"`
	UnitAmount    int64  `json:"unitAmount"`
	Currency      string `json:"currency"`
	Interval      string `json:"interval"`
	IntervalCount int64  `json:"intervalCount"`
	TaxBehavior   string `json:"taxBehavior"`
}
type BillingQuote struct {
	ID         string       `json:"id"`
	ConfigID   string       `json:"configId"`
	ExpiresAt  int64        `json:"expiresAt"`
	Price      BillingPrice `json:"price"`
	TermsURL   string       `json:"termsUrl"`
	PrivacyURL string       `json:"privacyUrl"`
}
type BillingReservation struct {
	ID          string       `json:"id"`
	CreatedAt   int64        `json:"createdAt"`
	ActorID     string       `json:"actorId"`
	Quote       BillingQuote `json:"quote"`
	SessionID   string       `json:"sessionId,omitempty"`
	CheckoutURL string       `json:"checkoutUrl,omitempty"`
	State       string       `json:"state"`
	// Missing means the original unscoped return URLs. A reserved checkout must
	// retain identical provider parameters when its idempotency key is retried.
	ReturnURLVersion int `json:"returnUrlVersion,omitempty"`
}
type billingEventReceipt struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}
type teamBillingRuntime struct {
	config     StripeBillingConfig
	configID   string
	live       bool
	provider   stripeBillingProvider
	operations chan struct{}
	expiry     *time.Timer
}

func cloneTeamBilling(b *TeamBilling) *TeamBilling {
	if b == nil {
		return nil
	}
	out := *b
	if b.Quote != nil {
		q := *b.Quote
		out.Quote = &q
	}
	if b.Reservation != nil {
		r := *b.Reservation
		out.Reservation = &r
	}
	out.Events = append([]billingEventReceipt(nil), b.Events...)
	return &out
}
func validateTeamBilling(b *TeamBilling) error {
	if b == nil {
		return nil
	}
	if len(b.ConfigID) != 64 || b.Revision < 1 || len(b.Events) > 128 {
		return errors.New("invalid private team billing state")
	}
	if b.Reservation != nil && (b.Reservation.ID == "" || b.Reservation.CreatedAt <= 0 || b.Reservation.ActorID == "" || b.Reservation.Quote.ConfigID != b.ConfigID) {
		return errors.New("invalid checkout reservation")
	}
	if b.Reservation != nil && (b.Reservation.ReturnURLVersion < 0 || b.Reservation.ReturnURLVersion > 1) {
		return errors.New("unsupported checkout return URL version")
	}
	return nil
}

// The initial managed plan supports these two-decimal currencies. Rejecting
// others prevents a misleading quote (for example JPY divided by 100).
func billingCurrencySupported(currency string) bool {
	switch currency {
	case "usd", "eur", "gbp", "cad", "aud", "nzd", "sgd", "chf", "sek", "nok", "dkk":
		return true
	}
	return false
}
func legalURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}
func (g *Gateway) initializeTeamBilling() error {
	c := g.opts.StripeBilling
	enabled := c != (StripeBillingConfig{})
	for _, row := range g.state.Teams {
		if row.Billing != nil && !enabled {
			return errors.New("configured team billing cannot be disabled by removing merchant configuration")
		}
	}
	if !enabled {
		return nil
	}
	if g.managedTeamID == "" || previewgateway.ValidatePublicAppOrigin(g.opts.PublicAppOrigin) != nil || !strings.HasPrefix(c.AccountID, "acct_") || !strings.HasPrefix(c.PriceID, "price_") || (!strings.HasPrefix(c.APIKey, "sk_test_") && !strings.HasPrefix(c.APIKey, "sk_live_")) || !strings.HasPrefix(c.WebhookSecret, "whsec_") || !legalURL(c.TermsURL) || !legalURL(c.PrivacyURL) {
		return errors.New("Stripe requires a bound managed team, exact HTTPS app origin, account, API key, webhook secret, recurring price and HTTPS terms/privacy URLs")
	}
	live := strings.HasPrefix(c.APIKey, "sk_live_")
	identity, _ := json.Marshal([]any{c.AccountID, live, c.PriceID, g.opts.PublicAppOrigin, c.TermsURL, c.PrivacyURL})
	digest := sha256.Sum256(identity)
	configID := hex.EncodeToString(digest[:])
	p := g.opts.stripeProvider
	if p == nil {
		p = newStripeProvider(c.APIKey)
	}
	g.billing = &teamBillingRuntime{config: c, configID: configID, live: live, provider: p, operations: make(chan struct{}, 1)}
	row := g.state.Teams[g.managedTeamID]
	if row.Billing != nil {
		if row.Billing.ConfigID != configID {
			return errors.New("Stripe merchant, mode, plan, origin or legal terms differ from the durable billing binding")
		}
		return nil
	}
	row.Billing = &TeamBilling{ConfigID: configID, Revision: 1, Status: "unsubscribed"}
	g.state.Teams[g.managedTeamID] = row
	return g.persistLocked()
}
func (g *Gateway) billingAllowsExecutionLocked(teamID string) bool {
	if g.billing == nil {
		return true
	}
	if teamID != g.managedTeamID {
		return false
	}
	b := g.state.Teams[teamID].Billing
	return b != nil && b.ConfigID == g.billing.configID && (b.Status == "active" || b.Status == "trialing") && b.SubscriptionID != "" && b.CustomerID != "" && b.PeriodEnd > time.Now().Unix()
}
func (g *Gateway) registerTeamBilling(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams/{teamId}/billing", g.getTeamBilling)
	mux.HandleFunc("POST /v1/teams/{teamId}/billing/quote", g.billingAction(g.quoteTeamBilling))
	mux.HandleFunc("POST /v1/teams/{teamId}/billing/checkout", g.billingAction(g.checkoutTeamBilling))
	mux.HandleFunc("POST /v1/teams/{teamId}/billing/reconcile", g.billingAction(g.reconcileTeamBilling))
	mux.HandleFunc("POST /v1/teams/{teamId}/billing/refresh", g.billingAction(g.refreshTeamBilling))
	mux.HandleFunc("POST /v1/teams/{teamId}/billing/portal", g.billingAction(g.portalTeamBilling))
	mux.HandleFunc("POST /v1/billing/stripe/webhook", g.stripeWebhook)
}
func (g *Gateway) billingAccess(r *http.Request, owner bool) (*TeamBilling, error) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	a, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil || r.Context().Err() != nil || (owner && a.Member.Role != "owner") || !g.billingSessionCurrentLocked(r, actor.ID) {
		return nil, errors.New("Only a current team owner can manage this subscription.")
	}
	return cloneTeamBilling(g.state.Teams[a.Team.ID].Billing), nil
}
func (g *Gateway) billingAction(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := g.billingAccess(r, true); err != nil {
			fail(w, 403, "billing_owner_required", err.Error())
			return
		}
		if g.billing == nil || r.PathValue("teamId") != g.managedTeamID {
			fail(w, 409, "billing_disabled", "Subscriptions are not configured for this installation.")
			return
		}
		select {
		case g.billing.operations <- struct{}{}:
			defer func() { <-g.billing.operations }()
		case <-r.Context().Done():
			return
		default:
			fail(w, 409, "billing_busy", "A billing operation is in progress. Refresh before retrying.")
			return
		}
		registered, finish, err := g.registerBillingRequest(r)
		if err != nil {
			fail(w, 403, "billing_owner_required", err.Error())
			return
		}
		defer finish()
		fn(w, registered)
	}
}
func (g *Gateway) billingViewLocked(teamID, actorID string) map[string]any {
	row := g.state.Teams[teamID]
	b := row.Billing
	out := map[string]any{"enabled": g.billing != nil, "owner": row.Members[actorID].Role == "owner", "executionAllowed": g.teamServiceAllowsExecutionLocked(teamID), "separateCosts": "Model usage and infrastructure are billed separately from the fixed team subscription."}
	if b == nil {
		return out
	}
	out["revision"] = b.Revision
	out["status"] = b.Status
	out["periodEnd"] = b.PeriodEnd
	out["cancelAtPeriodEnd"] = b.CancelAtPeriodEnd
	out["hasCustomer"] = b.CustomerID != ""
	out["canManageSubscription"] = b.CustomerID != "" && b.SubscriptionID != ""
	if row.Members[actorID].Role == "owner" {
		out["quote"] = b.Quote
		if b.Reservation != nil {
			out["checkoutState"] = b.Reservation.State
			out["checkoutRecoverable"] = b.Reservation.CreatedAt > time.Now().Add(-23*time.Hour).Unix()
		}
	}
	return out
}
func (g *Gateway) getTeamBilling(w http.ResponseWriter, r *http.Request) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	a, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil || !g.billingSessionCurrentLocked(r, actor.ID) {
		fail(w, 404, "team_unavailable", errTeamAccess.Error())
		return
	}
	jsonReply(w, 200, g.billingViewLocked(a.Team.ID, actor.ID))
}
func (g *Gateway) billingSessionCurrentLocked(r *http.Request, actorID string) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	session, ok := g.state.Sessions[hashToken(token)]
	return token != "" && token != r.Header.Get("Authorization") && ok && session.UserID == actorID && time.Now().Before(session.ExpiresAt)
}
func (g *Gateway) registerBillingRequest(r *http.Request) (*http.Request, func(), error) {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	a, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), actor.ID, "")
	if err != nil || a.Member.Role != "owner" || !g.billingSessionCurrentLocked(r, actor.ID) {
		return nil, nil, errors.New("Current team owner access is required.")
	}
	ctx, cancel := context.WithCancel(r.Context())
	scope := &teamRequest{teamSocketScope: teamSocketScope{TeamID: a.Team.ID, UserID: actor.ID, MemberRevision: a.Member.Revision}, SessionHash: hashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))}
	if g.teamRequests == nil {
		g.teamRequests = map[*teamRequest]context.CancelFunc{}
	}
	g.teamRequests[scope] = cancel
	var once sync.Once
	finish := func() { once.Do(func() { cancel(); g.mu.Lock(); delete(g.teamRequests, scope); g.mu.Unlock() }) }
	return r.WithContext(ctx), finish, nil
}
func (g *Gateway) saveBilling(r *http.Request, b *TeamBilling, action string) error {
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.managedTeamID
	if r.PathValue("teamId") != "" {
		a, err := g.resolveTeamAccessLocked(id, actor.ID, "")
		if err != nil || a.Member.Role != "owner" || r.Context().Err() != nil || !g.billingSessionCurrentLocked(r, actor.ID) {
			return errors.New("team owner access changed")
		}
	}
	if g.closed {
		return errors.New("billing service closed")
	}
	wasAllowed := g.billingAllowsExecutionLocked(id)
	before := g.state
	g.state = cloneIdentityState(before)
	row := g.state.Teams[id]
	b.Revision = row.Billing.Revision + 1
	row.Billing = cloneTeamBilling(b)
	g.state.Teams[id] = row
	g.auditLocked(actor.ID, "team.billing."+action, id, "success")
	if err := g.persistLocked(); err != nil {
		g.state = before
		return err
	}
	if wasAllowed && !g.billingAllowsExecutionLocked(id) {
		g.revokeTeamAccessLocked(id, "", "")
	}
	g.scheduleBillingExpiryLocked()
	return nil
}
func billingFailure(w http.ResponseWriter) {
	fail(w, 502, "billing_unavailable", "Stripe billing could not be verified. No new execution access was granted. Refresh or retry the existing reservation; contact the operator if this persists.")
}

var errBillingPlan = errors.New("invalid fixed recurring price")

func (g *Gateway) verifyBillingMerchant(ctx context.Context) error {
	account, err := g.billing.provider.Account(ctx)
	if err != nil || account == nil || account.ID != g.billing.config.AccountID {
		return errors.New("merchant unavailable or mismatched")
	}
	return nil
}
func (g *Gateway) verifiedPrice(ctx context.Context) (BillingPrice, error) {
	if err := g.verifyBillingMerchant(ctx); err != nil {
		return BillingPrice{}, err
	}
	p, err := g.billing.provider.Price(ctx, g.billing.config.PriceID)
	if err != nil {
		return BillingPrice{}, errors.New("price unavailable")
	}
	if p == nil || p.ID != g.billing.config.PriceID || !p.Active || p.Deleted || p.Livemode != g.billing.live || p.Type != "recurring" || p.BillingScheme != "per_unit" || p.Recurring == nil || p.Recurring.UsageType != "licensed" || p.Recurring.Meter != "" || p.Recurring.IntervalCount != 1 || (p.Recurring.Interval != "month" && p.Recurring.Interval != "year") || p.CustomUnitAmount != nil || p.TransformQuantity != nil || p.TiersMode != "" || len(p.Tiers) != 0 || p.UnitAmount <= 0 || !billingCurrencySupported(string(p.Currency)) || (!p.UnitAmountDecimal.IsZero() && !p.UnitAmountDecimal.Equal(decimal.NewFromInt(p.UnitAmount))) {
		return BillingPrice{}, errBillingPlan
	}
	return BillingPrice{p.ID, p.UnitAmount, string(p.Currency), string(p.Recurring.Interval), p.Recurring.IntervalCount, string(p.TaxBehavior)}, nil
}
func (g *Gateway) quoteTeamBilling(w http.ResponseWriter, r *http.Request) {
	b, err := g.billingAccess(r, true)
	if err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	if b.Reservation != nil && b.Reservation.State != "expired" && b.Status != "canceled" {
		fail(w, 409, "checkout_exists", "Refresh or resume the existing checkout before requesting another price.")
		return
	}
	if b.SubscriptionID != "" && b.Status != "canceled" {
		fail(w, 409, "subscription_exists", "Manage the existing subscription through Stripe.")
		return
	}
	price, err := g.verifiedPrice(r.Context())
	if err != nil {
		billingFailure(w)
		return
	}
	b.Quote = &BillingQuote{ID: randomToken(), ConfigID: g.billing.configID, ExpiresAt: time.Now().Add(15 * time.Minute).Unix(), Price: price, TermsURL: g.billing.config.TermsURL, PrivacyURL: g.billing.config.PrivacyURL}
	if err = g.saveBilling(r, b, "quoted"); err != nil {
		billingFailure(w)
		return
	}
	g.getTeamBilling(w, r)
}
