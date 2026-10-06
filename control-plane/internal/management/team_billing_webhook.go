package management

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/stripe/stripe-go/v87/webhook"
	"io"
	"net/http"
	"time"
)

func (g *Gateway) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if g.billing == nil {
		fail(w, 404, "billing_disabled", "Stripe billing is not configured.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		fail(w, 400, "invalid_webhook", "Invalid Stripe webhook.")
		return
	}
	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), g.billing.config.WebhookSecret)
	if err != nil || event.ID == "" || event.Livemode != g.billing.live || (event.Account != "" && event.Account != g.billing.config.AccountID) {
		fail(w, 400, "invalid_webhook", "Invalid Stripe webhook signature, timestamp or merchant mode.")
		return
	}
	switch event.Type {
	case "checkout.session.completed", "checkout.session.expired", "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
	default:
		jsonReply(w, 200, map[string]bool{"received": true})
		return
	}
	// Provider calls are serialized across checkout, refresh and webhook handling.
	// A delivery arriving before checkout's response is saved retries against the
	// already durable reservation; its metadata can safely recover that outcome.
	select {
	case g.billing.operations <- struct{}{}:
		defer func() { <-g.billing.operations }()
	default:
		fail(w, 503, "billing_busy", "Billing is reconciling another operation. Retry this delivery.")
		return
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	g.mu.Lock()
	b := cloneTeamBilling(g.state.Teams[g.managedTeamID].Billing)
	g.mu.Unlock()
	for _, receipt := range b.Events {
		if receipt.ID == event.ID {
			if receipt.SHA256 != digest {
				fail(w, 409, "webhook_conflict", "The event ID was reused with different content.")
				return
			}
			jsonReply(w, 200, map[string]bool{"received": true})
			return
		}
	}
	var object struct {
		ID string `json:"id"`
	}
	if event.Data == nil || json.Unmarshal(event.Data.Raw, &object) != nil || object.ID == "" {
		fail(w, 400, "invalid_webhook", "Stripe event object is missing.")
		return
	}
	switch event.Type {
	case "checkout.session.completed", "checkout.session.expired":
		s, e := g.billing.provider.Checkout(r.Context(), object.ID)
		if e != nil {
			billingFailure(w)
			return
		}
		err = g.applyCheckout(r.Context(), b, s)
	default:
		s, e := g.billing.provider.Subscription(r.Context(), object.ID)
		if e != nil {
			billingFailure(w)
			return
		}
		err = g.applySubscription(r.Context(), b, s)
	}
	if err != nil {
		fail(w, 409, "billing_binding_mismatch", "This Stripe event does not match the team's durable checkout and plan binding.")
		return
	}
	b.Events = append(b.Events, billingEventReceipt{event.ID, digest})
	if len(b.Events) > 128 {
		b.Events = b.Events[len(b.Events)-128:]
	}
	if err = g.saveBilling(r, b, "webhook-verified"); err != nil {
		billingFailure(w)
		return
	}
	jsonReply(w, 200, map[string]bool{"received": true})
}

// Existing terminals and browser streams must end at the last verified billing
// deadline, even if every subsequent provider webhook is lost.
func (g *Gateway) scheduleBillingExpiryLocked() {
	if g.billing == nil {
		return
	}
	if g.billing.expiry != nil {
		g.billing.expiry.Stop()
		g.billing.expiry = nil
	}
	b := g.state.Teams[g.managedTeamID].Billing
	if b == nil || (b.Status != "active" && b.Status != "trialing") || b.PeriodEnd <= time.Now().Unix() {
		return
	}
	deadline := b.PeriodEnd
	g.billing.expiry = time.AfterFunc(time.Until(time.Unix(deadline, 0)), func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.closed {
			return
		}
		if !g.billingAllowsExecutionLocked(g.managedTeamID) {
			g.revokeTeamAccessLocked(g.managedTeamID, "", "")
		}
	})
}
