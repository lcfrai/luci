package management

import (
	"context"
	"errors"
	stripe "github.com/stripe/stripe-go/v87"
	"net/http"
	"net/url"
	"time"
)

func billingRedirect(raw, host string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host == host && u.User == nil
}
func (g *Gateway) billingMetadata(reservationID string) map[string]string {
	return map[string]string{"luci_team": g.managedTeamID, "luci_reservation": reservationID, "luci_config": g.billing.configID, "luci_account": g.billing.config.AccountID}
}
func (g *Gateway) validBillingMetadata(meta map[string]string, b *TeamBilling) bool {
	if b.Reservation == nil {
		return false
	}
	for k, v := range g.billingMetadata(b.Reservation.ID) {
		if meta[k] != v {
			return false
		}
	}
	return true
}

// The return URL identifies a navigation destination only. Subscription access
// still requires the existing server-side Stripe receipt verification.
func (g *Gateway) billingReturnURL(kind string) string {
	return g.opts.PublicAppOrigin + "/?" + url.Values{"billing": {kind}, "billingTeam": {g.managedTeamID}}.Encode()
}
func (g *Gateway) checkoutParams(b *TeamBilling) *stripe.CheckoutSessionCreateParams {
	rr := b.Reservation
	successURL, cancelURL := g.opts.PublicAppOrigin+"/?billing=return", g.opts.PublicAppOrigin+"/?billing=cancel"
	if rr.ReturnURLVersion == 1 {
		successURL, cancelURL = g.billingReturnURL("return"), g.billingReturnURL("cancel")
	}
	p := &stripe.CheckoutSessionCreateParams{Mode: stripe.String("subscription"), ClientReferenceID: stripe.String(g.managedTeamID), Metadata: g.billingMetadata(rr.ID), SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{Metadata: g.billingMetadata(rr.ID)}, LineItems: []*stripe.CheckoutSessionCreateLineItemParams{{Price: stripe.String(rr.Quote.Price.ID), Quantity: stripe.Int64(1)}}, SuccessURL: stripe.String(successURL), CancelURL: stripe.String(cancelURL), ExpiresAt: stripe.Int64(rr.CreatedAt + 24*60*60), AllowPromotionCodes: stripe.Bool(false)}
	if b.CustomerID != "" {
		p.Customer = stripe.String(b.CustomerID)
	}
	p.AdaptivePricing = &stripe.CheckoutSessionCreateAdaptivePricingParams{Enabled: stripe.Bool(false)}
	p.SetIdempotencyKey("luci-team-checkout-" + rr.ID)
	return p
}
func (g *Gateway) checkoutTeamBilling(w http.ResponseWriter, r *http.Request) {
	var input struct {
		QuoteID          string `json:"quoteId"`
		ExpectedRevision int    `json:"expectedRevision"`
		AcknowledgePrice bool   `json:"acknowledgePrice"`
		AcknowledgeTerms bool   `json:"acknowledgeTerms"`
	}
	if !decode(w, r, &input) {
		return
	}
	b, err := g.billingAccess(r, true)
	if err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	if input.ExpectedRevision != b.Revision {
		fail(w, 409, "billing_changed", "Billing changed. Refresh and review the current price before continuing.")
		return
	}
	if !input.AcknowledgePrice || !input.AcknowledgeTerms {
		fail(w, 400, "billing_acknowledgement_required", "Review and acknowledge the subscription price, separate costs and terms before continuing.")
		return
	}
	if b.SubscriptionID != "" && b.Status != "canceled" {
		fail(w, 409, "subscription_exists", "An existing subscription must be managed through Stripe.")
		return
	}
	pending := b.Reservation != nil && b.Reservation.State != "expired" && b.Status != "canceled"
	var q *BillingQuote
	if pending {
		q = &b.Reservation.Quote
	} else {
		q = b.Quote
	}
	if q == nil || input.QuoteID != q.ID || q.ConfigID != g.billing.configID || (!pending && q.ExpiresAt < time.Now().Unix()) {
		fail(w, 409, "quote_expired", "Request and review a fresh price quote.")
		return
	}
	if pending && b.Reservation.CreatedAt < time.Now().Add(-23*time.Hour).Unix() {
		fail(w, 409, "checkout_reconciliation_required", "This checkout has an uncertain outcome beyond Stripe's safe retry window. Refresh its known session or ask the operator to reconcile the reservation before creating another.")
		return
	}
	price, err := g.verifiedPrice(r.Context())
	if err != nil {
		billingFailure(w)
		return
	}
	if price != q.Price {
		fail(w, 409, "price_changed", "The configured price changed. No checkout was created. Review a fresh price quote or ask the operator to reconcile the existing reservation.")
		return
	}
	if !pending {
		if b.Status == "canceled" {
			b.SubscriptionID = ""
			b.PeriodEnd = 0
			b.CancelAtPeriodEnd = false
			b.Status = "unsubscribed"
		}
		actor, _, _ := ActorFromContext(r.Context())
		b.Reservation = &BillingReservation{ID: randomToken(), CreatedAt: time.Now().Unix(), ActorID: actor.ID, Quote: *q, State: "reserved", ReturnURLVersion: 1}
		// This write precedes every potentially chargeable provider request.
		if err = g.saveBilling(r, b, "checkout-reserved"); err != nil {
			billingFailure(w)
			return
		}
	}
	if _, err = g.billingAccess(r, true); err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	var session *stripe.CheckoutSession
	if b.Reservation.SessionID != "" {
		session, err = g.billing.provider.Checkout(r.Context(), b.Reservation.SessionID)
	} else {
		session, err = g.billing.provider.CreateCheckout(r.Context(), g.checkoutParams(b))
	}
	if err != nil || session == nil {
		billingFailure(w)
		return
	}
	if err = g.applyCheckout(r.Context(), b, session); err != nil {
		billingFailure(w)
		return
	}
	if err = g.saveBilling(r, b, "checkout-saved"); err != nil {
		billingFailure(w)
		return
	}
	if b.Reservation.State != "open" || !billingRedirect(b.Reservation.CheckoutURL, "checkout.stripe.com") {
		g.getTeamBilling(w, r)
		return
	}
	jsonReply(w, 200, map[string]any{"url": b.Reservation.CheckoutURL, "revision": b.Revision})
}
func (g *Gateway) applyCheckout(ctx context.Context, b *TeamBilling, s *stripe.CheckoutSession) error {
	if s == nil || s.ID == "" || s.Livemode != g.billing.live || s.Mode != "subscription" || s.ClientReferenceID != g.managedTeamID || !g.validBillingMetadata(s.Metadata, b) || (b.Reservation.SessionID != "" && b.Reservation.SessionID != s.ID) {
		return errors.New("checkout binding mismatch")
	}
	if s.Customer != nil && s.Customer.ID != "" {
		if b.CustomerID != "" && b.CustomerID != s.Customer.ID {
			return errors.New("customer mismatch")
		}
		b.CustomerID = s.Customer.ID
	}
	b.Reservation.SessionID = s.ID
	switch s.Status {
	case "expired":
		if b.Reservation.State != "complete" {
			b.Reservation.State = "expired"
			b.Reservation.CheckoutURL = ""
		}
	case "open":
		if b.Reservation.State != "complete" {
			if !billingRedirect(s.URL, "checkout.stripe.com") {
				return errors.New("invalid checkout redirect")
			}
			b.Reservation.State = "open"
			b.Reservation.CheckoutURL = s.URL
		}
	case "complete":
		if s.Subscription == nil || s.Subscription.ID == "" || b.CustomerID == "" {
			return errors.New("completed checkout missing subscription")
		}
		sub, err := g.billing.provider.Subscription(ctx, s.Subscription.ID)
		if err != nil {
			return errors.New("subscription unavailable")
		}
		if err = g.applySubscription(ctx, b, sub); err != nil {
			return err
		}
		b.Reservation.State = "complete"
		b.Reservation.CheckoutURL = ""
	default:
		return errors.New("unknown checkout state")
	}
	return nil
}
func (g *Gateway) applySubscription(ctx context.Context, b *TeamBilling, s *stripe.Subscription) error {
	if s == nil || s.ID == "" || s.Customer == nil || s.Customer.ID == "" || s.Livemode != g.billing.live || !g.validBillingMetadata(s.Metadata, b) || (b.CustomerID != "" && b.CustomerID != s.Customer.ID) || (b.SubscriptionID != "" && b.SubscriptionID != s.ID) {
		return errors.New("subscription binding mismatch")
	}
	if err := g.verifyBillingMerchant(ctx); err != nil {
		return err
	}
	current := b.SubscriptionID == s.ID && b.CustomerID == s.Customer.ID
	downgrade := func() error {
		if !current {
			return errors.New("subscription plan mismatch")
		}
		b.Status = "plan_mismatch"
		b.PeriodEnd = 0
		b.CancelAtPeriodEnd = s.CancelAtPeriodEnd
		return nil
	}
	switch s.Status {
	case "active", "trialing", "past_due", "canceled", "unpaid", "incomplete", "incomplete_expired", "paused":
	default:
		return downgrade()
	}
	// A known subscription's cancellation/payment failure must revoke access even
	// when the merchant archives the old price or changes its subscription items.
	if current && s.Status != "active" && s.Status != "trialing" {
		b.Status = string(s.Status)
		b.PeriodEnd = 0
		b.CancelAtPeriodEnd = s.CancelAtPeriodEnd
		return nil
	}
	if s.CollectionMethod != "charge_automatically" || s.PauseCollection != nil || s.Items == nil || s.Items.HasMore || len(s.Items.Data) != 1 {
		return downgrade()
	}
	item := s.Items.Data[0]
	if item == nil || item.Deleted || item.Quantity != 1 || item.Price == nil || item.Price.ID != g.billing.config.PriceID || item.Price.Livemode != g.billing.live || (item.Subscription != "" && item.Subscription != s.ID) {
		return downgrade()
	}
	price, err := g.verifiedPrice(ctx)
	if err != nil {
		if errors.Is(err, errBillingPlan) {
			return downgrade()
		}
		return err
	}
	if price != b.Reservation.Quote.Price || (s.Currency != "" && string(s.Currency) != price.Currency) {
		return downgrade()
	}
	end := item.CurrentPeriodEnd
	if s.Status == "trialing" {
		if s.TrialEnd <= 0 {
			return downgrade()
		}
		if end == 0 || s.TrialEnd < end {
			end = s.TrialEnd
		}
	}
	if (s.Status == "active" || s.Status == "trialing") && end <= 0 {
		return downgrade()
	}
	b.CustomerID = s.Customer.ID
	b.SubscriptionID = s.ID
	b.Status = string(s.Status)
	b.PeriodEnd = end
	b.CancelAtPeriodEnd = s.CancelAtPeriodEnd
	b.Reservation.State = "complete"
	b.Reservation.CheckoutURL = ""
	return nil
}
func (g *Gateway) refreshTeamBilling(w http.ResponseWriter, r *http.Request) {
	b, err := g.billingAccess(r, true)
	if err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	if b.SubscriptionID != "" {
		s, e := g.billing.provider.Subscription(r.Context(), b.SubscriptionID)
		if e != nil {
			billingFailure(w)
			return
		}
		err = g.applySubscription(r.Context(), b, s)
	} else if b.Reservation != nil && b.Reservation.SessionID != "" {
		s, e := g.billing.provider.Checkout(r.Context(), b.Reservation.SessionID)
		if e != nil {
			billingFailure(w)
			return
		}
		err = g.applyCheckout(r.Context(), b, s)
	} else {
		g.getTeamBilling(w, r)
		return
	}
	if err != nil {
		billingFailure(w)
		return
	}
	if err = g.saveBilling(r, b, "refreshed"); err != nil {
		billingFailure(w)
		return
	}
	g.getTeamBilling(w, r)
}
func (g *Gateway) portalTeamBilling(w http.ResponseWriter, r *http.Request) {
	b, err := g.billingAccess(r, true)
	if err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	if b.CustomerID == "" || b.SubscriptionID == "" {
		fail(w, 409, "billing_customer_missing", "Complete and verify checkout before managing this subscription.")
		return
	}
	s, err := g.billing.provider.Subscription(r.Context(), b.SubscriptionID)
	if err != nil || s == nil || s.Customer == nil || s.Customer.ID != b.CustomerID || !g.validBillingMetadata(s.Metadata, b) || s.Livemode != g.billing.live {
		billingFailure(w)
		return
	}
	account, err := g.billing.provider.Account(r.Context())
	if err != nil || account == nil || account.ID != g.billing.config.AccountID {
		billingFailure(w)
		return
	}
	if _, err = g.billingAccess(r, true); err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	p, err := g.billing.provider.Portal(r.Context(), &stripe.BillingPortalSessionCreateParams{Customer: stripe.String(b.CustomerID), ReturnURL: stripe.String(g.billingReturnURL("portal"))})
	if err != nil || p == nil || !billingRedirect(p.URL, "billing.stripe.com") {
		billingFailure(w)
		return
	}
	if _, err = g.billingAccess(r, true); err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	jsonReply(w, 200, map[string]any{"url": p.URL})
}

// Recover a lost checkout response using its Stripe receipt ID. This cannot
// reset a reservation or trust supplied billing state: Stripe must return the
// exact existing team/reservation/config metadata before anything is saved.
func (g *Gateway) reconcileTeamBilling(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SessionID        string `json:"sessionId"`
		ExpectedRevision int    `json:"expectedRevision"`
	}
	if !decode(w, r, &input) {
		return
	}
	b, err := g.billingAccess(r, true)
	if err != nil {
		fail(w, 403, "billing_owner_required", err.Error())
		return
	}
	if b.Reservation == nil || b.Revision != input.ExpectedRevision || len(input.SessionID) < 4 || len(input.SessionID) > 256 || input.SessionID[:3] != "cs_" {
		fail(w, 409, "billing_changed", "Refresh billing and provide the Stripe Checkout session ID for this reservation.")
		return
	}
	s, err := g.billing.provider.Checkout(r.Context(), input.SessionID)
	if err != nil {
		billingFailure(w)
		return
	}
	if err = g.applyCheckout(r.Context(), b, s); err != nil {
		fail(w, 409, "billing_binding_mismatch", "This checkout does not match the team's durable reservation.")
		return
	}
	if err = g.saveBilling(r, b, "checkout-reconciled"); err != nil {
		billingFailure(w)
		return
	}
	g.getTeamBilling(w, r)
}
