package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

func TestTeamBillingMerchantConfigurationNeverEntersExecutionWorker(t *testing.T) {
	base := []string{
		"PATH=/tools", "LUCI_STRIPE_API_KEY=merchant-api-key",
		"LUCI_STRIPE_SECRET_KEY=merchant-secret-key", "LUCI_STRIPE_WEBHOOK_SECRET=merchant-webhook-proof",
		"STRIPE_SECRET_KEY=other-merchant-key", "STRIPE_WEBHOOK_SECRET=other-merchant-proof",
		"LUCI_STRIPE_ACCOUNT_ID=acct_merchant", "LUCI_STRIPE_PRICE_ID=price_merchant",
	}
	root := filepath.Join(t.TempDir(), "runtime")
	worker := isolatedTeamEnvironment(base, root, "private-worker-token", filepath.Join(root, "address.json"), "team-abcdefghijklmnopqrstuv", User{ID: "member", Role: "member"})
	for _, entry := range worker {
		if strings.Contains(strings.ToUpper(entry), "STRIPE") || strings.Contains(entry, "merchant-") {
			t.Fatal("execution worker inherited merchant authority")
		}
	}
}

// This independent provider fixture intentionally supports malformed responses
// and ambiguous failures that the normal SDK/HTTP acceptance fixture cannot.
type securityBillingProvider struct {
	account                       *stripe.Account
	price                         *stripe.Price
	sub                           *stripe.Subscription
	session                       *stripe.CheckoutSession
	create                        func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	priceHook                     func()
	calls, creates, subscriptions int
}

func (p *securityBillingProvider) Account(context.Context) (*stripe.Account, error) {
	p.calls++
	return p.account, nil
}
func (p *securityBillingProvider) Price(context.Context, string) (*stripe.Price, error) {
	p.calls++
	if p.priceHook != nil {
		p.priceHook()
	}
	return p.price, nil
}
func (p *securityBillingProvider) CreateCheckout(_ context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	p.calls++
	p.creates++
	if p.create != nil {
		return p.create(params)
	}
	return p.session, nil
}
func (p *securityBillingProvider) Checkout(context.Context, string) (*stripe.CheckoutSession, error) {
	p.calls++
	return p.session, nil
}
func (p *securityBillingProvider) Subscription(context.Context, string) (*stripe.Subscription, error) {
	p.calls++
	p.subscriptions++
	return p.sub, nil
}
func (p *securityBillingProvider) Portal(context.Context, *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	p.calls++
	return &stripe.BillingPortalSession{URL: "https://billing.stripe.com/p/session/fixture"}, nil
}

type securityBillingFixture struct {
	g           *Gateway
	h           http.Handler
	p           *securityBillingProvider
	team        Team
	owner       User
	token, base string
}

func newSecurityBillingFixture(t *testing.T) *securityBillingFixture {
	t.Helper()
	g, h, token, owner := fixture(t, nil)
	team := createTestTeam(t, h, token, "Independent billing security")
	opts := g.opts
	if err := g.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	p := &securityBillingProvider{
		account: &stripe.Account{ID: "acct_fixture"},
		price:   &stripe.Price{ID: "price_fixture", Active: true, Type: "recurring", BillingScheme: "per_unit", Currency: "usd", UnitAmount: 2900, Recurring: &stripe.PriceRecurring{Interval: "month", IntervalCount: 1, UsageType: "licensed"}},
	}
	opts.ManagedTeamID = team.ID
	opts.PublicAppOrigin = "https://app.example.com"
	opts.StripeBilling = StripeBillingConfig{APIKey: "sk_test_fixture", WebhookSecret: "whsec_fixture", AccountID: "acct_fixture", PriceID: "price_fixture", TermsURL: "https://app.example.com/terms", PrivacyURL: "https://app.example.com/privacy"}
	opts.stripeProvider = p
	g, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := g.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return &securityBillingFixture{g: g, h: g.Handler(), p: p, team: team, owner: owner, token: token, base: "/v1/teams/" + team.ID + "/billing"}
}
func (f *securityBillingFixture) read() *TeamBilling {
	f.g.mu.Lock()
	defer f.g.mu.Unlock()
	return cloneTeamBilling(f.g.state.Teams[f.team.ID].Billing)
}
func (f *securityBillingFixture) set(b *TeamBilling) {
	f.g.mu.Lock()
	defer f.g.mu.Unlock()
	row := f.g.state.Teams[f.team.ID]
	row.Billing = cloneTeamBilling(b)
	f.g.state.Teams[f.team.ID] = row
}
func (f *securityBillingFixture) reserve(t *testing.T) *TeamBilling {
	t.Helper()
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 200)
	f.p.create = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return nil, errors.New("response lost")
	}
	f.checkout(t, 502)
	f.p.create = nil
	return f.read()
}
func (f *securityBillingFixture) checkoutBody() map[string]any {
	b := f.read()
	q := b.Quote
	if b.Reservation != nil && b.Reservation.State != "expired" && b.Status != "canceled" {
		q = &b.Reservation.Quote
	}
	return map[string]any{"quoteId": q.ID, "expectedRevision": b.Revision, "acknowledgePrice": true, "acknowledgeTerms": true}
}
func (f *securityBillingFixture) checkout(t *testing.T, status int) {
	t.Helper()
	request(t, f.h, "POST", f.base+"/checkout", f.token, f.checkoutBody(), status)
}
func (f *securityBillingFixture) subscription() *stripe.Subscription {
	b := f.read()
	return &stripe.Subscription{ID: "sub_fixture", Customer: &stripe.Customer{ID: "cus_fixture"}, Status: "active", CollectionMethod: "charge_automatically", Metadata: f.g.billingMetadata(b.Reservation.ID), Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{ID: "si_fixture", Quantity: 1, Subscription: "sub_fixture", CurrentPeriodEnd: time.Now().Add(time.Hour).Unix(), Price: &stripe.Price{ID: "price_fixture"}}}}}
}
func (f *securityBillingFixture) payload(t *testing.T, eventID, eventType string, extra map[string]any) []byte {
	t.Helper()
	body := map[string]any{"id": eventID, "object": "event", "api_version": stripe.APIVersion, "livemode": false, "type": eventType, "data": map[string]any{"object": map[string]any{"id": "sub_fixture", "status": "active"}}}
	for k, v := range extra {
		body[k] = v
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func (f *securityBillingFixture) webhook(t *testing.T, payload []byte, secret string, at time.Time, status int) {
	t.Helper()
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: at})
	r := httptest.NewRequest("POST", "/v1/billing/stripe/webhook", bytes.NewReader(payload))
	r.Header.Set("Stripe-Signature", signed.Header)
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("webhook status %d want %d: %s", w.Code, status, w.Body.String())
	}
}
func (f *securityBillingFixture) allowed() bool {
	f.g.mu.Lock()
	defer f.g.mu.Unlock()
	return f.g.teamServiceAllowsExecutionLocked(f.team.ID)
}

func TestTeamBillingCanceledReplacementRetainsOneDurableReservation(t *testing.T) {
	f := newSecurityBillingFixture(t)
	b := f.reserve(t)
	oldID := b.Reservation.ID
	b.CustomerID = "cus_fixture"
	b.SubscriptionID = "sub_old"
	b.Status = "canceled"
	b.PeriodEnd = time.Now().Add(time.Hour).Unix()
	b.CancelAtPeriodEnd = true
	b.Reservation.State = "complete"
	f.set(b)
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 200)
	var keys []string
	f.p.create = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		keys = append(keys, stripe.StringValue(params.IdempotencyKey))
		b := f.read()
		if b.SubscriptionID != "" || b.Status != "unsubscribed" || b.PeriodEnd != 0 || b.CancelAtPeriodEnd || b.CustomerID != "cus_fixture" {
			t.Error("replacement retained canceled subscription authority")
		}
		data, err := os.ReadFile(filepath.Join(f.g.dir, "identity.json"))
		if err != nil || !bytes.Contains(data, []byte(b.Reservation.ID)) {
			t.Error("provider called before durable replacement reservation")
		}
		return nil, errors.New("ambiguous provider outcome")
	}
	f.checkout(t, 502)
	first := f.read()
	if first.Reservation.ID == oldID {
		t.Fatal("replacement reused canceled generation")
	}
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 409)
	request(t, f.h, "POST", f.base+"/refresh", f.token, map[string]any{}, 200)
	f.checkout(t, 502)
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] || first.Reservation.ID != f.read().Reservation.ID {
		t.Fatal("ambiguous replacement retry created another chargeable reservation")
	}
	if f.p.subscriptions != 0 || f.allowed() {
		t.Fatal("old subscription was fetched or restored execution")
	}
	// A delayed event from the canceled generation cannot replace this pending one.
	f.p.sub = f.subscription()
	f.p.sub.ID = "sub_old"
	f.p.sub.Metadata = f.g.billingMetadata(oldID)
	payload := f.payload(t, "evt_old", "customer.subscription.updated", nil)
	f.webhook(t, payload, "whsec_fixture", time.Now(), 409)
	if !reflect.DeepEqual(first, f.read()) {
		t.Fatal("old subscription event changed replacement state")
	}
}

func TestTeamBillingRejectsUntrustedWebhookBeforeProviderAccess(t *testing.T) {
	f := newSecurityBillingFixture(t)
	baseline := f.read()
	for _, tc := range []struct {
		name, secret string
		at           time.Time
		extra        map[string]any
	}{
		{"wrong signature", "whsec_foreign", time.Now(), nil},
		{"expired signature", "whsec_fixture", time.Now().Add(-10 * time.Minute), nil},
		{"wrong mode", "whsec_fixture", time.Now(), map[string]any{"livemode": true}},
		{"wrong merchant", "whsec_fixture", time.Now(), map[string]any{"account": "acct_foreign"}},
		{"wrong api version", "whsec_fixture", time.Now(), map[string]any{"api_version": "2010-01-01"}},
		{"missing object", "whsec_fixture", time.Now(), map[string]any{"data": map[string]any{"object": map[string]any{}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.webhook(t, f.payload(t, "evt_invalid", "customer.subscription.updated", tc.extra), tc.secret, tc.at, 400)
		})
	}
	if f.p.calls != 0 || !reflect.DeepEqual(baseline, f.read()) || f.allowed() {
		t.Fatal("unverified webhook reached provider or changed entitlement")
	}
}

func TestTeamBillingWebhookBindsSubscriptionAndAcknowledgedPlan(t *testing.T) {
	f := newSecurityBillingFixture(t)
	baseline := f.reserve(t)
	for _, tc := range []struct {
		name   string
		mutate func(*stripe.Subscription)
	}{
		{"foreign team", func(s *stripe.Subscription) { s.Metadata["luci_team"] = "team-foreign" }},
		{"foreign reservation", func(s *stripe.Subscription) { s.Metadata["luci_reservation"] = "old-reservation" }},
		{"foreign configuration", func(s *stripe.Subscription) { s.Metadata["luci_config"] = "foreign" }},
		{"foreign merchant", func(s *stripe.Subscription) { s.Metadata["luci_account"] = "acct_foreign" }},
		{"live mode", func(s *stripe.Subscription) { s.Livemode = true }},
		{"missing customer", func(s *stripe.Subscription) { s.Customer = nil }},
		{"foreign price", func(s *stripe.Subscription) { s.Items.Data[0].Price.ID = "price_other" }},
		{"extra quantity", func(s *stripe.Subscription) { s.Items.Data[0].Quantity = 2 }},
		{"extra item", func(s *stripe.Subscription) { s.Items.Data = append(s.Items.Data, s.Items.Data[0]) }},
		{"paginated items", func(s *stripe.Subscription) { s.Items.HasMore = true }},
		{"foreign item subscription", func(s *stripe.Subscription) { s.Items.Data[0].Subscription = "sub_other" }},
		{"invoice collection", func(s *stripe.Subscription) { s.CollectionMethod = "send_invoice" }},
		{"paused collection", func(s *stripe.Subscription) { s.PauseCollection = &stripe.SubscriptionPauseCollection{} }},
		{"missing paid deadline", func(s *stripe.Subscription) { s.Items.Data[0].CurrentPeriodEnd = 0 }},
		{"missing trial deadline", func(s *stripe.Subscription) { s.Status = "trialing"; s.TrialEnd = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.p.sub = f.subscription()
			tc.mutate(f.p.sub)
			f.webhook(t, f.payload(t, "evt_bound", "customer.subscription.updated", nil), "whsec_fixture", time.Now(), 409)
			if !reflect.DeepEqual(baseline, f.read()) || f.allowed() {
				t.Fatal("mismatched subscription changed durable entitlement or receipt")
			}
		})
	}
	// A changed provider quote cannot silently authorize an unacknowledged price.
	f.p.sub = f.subscription()
	f.p.price.UnitAmount++
	f.webhook(t, f.payload(t, "evt_price", "customer.subscription.updated", nil), "whsec_fixture", time.Now(), 409)
	if !reflect.DeepEqual(baseline, f.read()) {
		t.Fatal("unacknowledged price persisted")
	}
}

func TestTeamBillingWebhookUsesCurrentProviderStateAndPreservesManualSuspension(t *testing.T) {
	f := newSecurityBillingFixture(t)
	f.reserve(t)
	f.p.sub = f.subscription()
	f.p.sub.Status = "canceled"
	stale := f.payload(t, "evt_stale_active", "customer.subscription.updated", nil)
	f.webhook(t, stale, "whsec_fixture", time.Now(), 200)
	if f.read().Status != "canceled" || f.allowed() {
		t.Fatal("stale event payload overrode authoritative canceled subscription")
	}
	before := f.read()
	calls := f.p.calls
	f.webhook(t, stale, "whsec_fixture", time.Now(), 200)
	if f.p.calls != calls || !reflect.DeepEqual(before, f.read()) {
		t.Fatal("identical replay repeated provider calls or writes")
	}
	conflict := f.payload(t, "evt_stale_active", "customer.subscription.updated", map[string]any{"created": 123})
	f.webhook(t, conflict, "whsec_fixture", time.Now(), 409)
	if f.p.calls != calls || !reflect.DeepEqual(before, f.read()) {
		t.Fatal("conflicting replay changed authority")
	}
	f.g.mu.Lock()
	row := f.g.state.Teams[f.team.ID]
	row.Service = TeamService{Mode: "managed", Status: "suspended", Revision: 1, Reason: "operator pause", UpdatedBy: f.owner.ID, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	f.g.state.Teams[f.team.ID] = row
	f.g.mu.Unlock()
	f.p.sub = f.subscription()
	f.webhook(t, f.payload(t, "evt_new_active", "customer.subscription.updated", nil), "whsec_fixture", time.Now(), 200)
	if f.read().Status != "active" || f.allowed() {
		t.Fatal("payment webhook overrode operator suspension")
	}
}

func TestTeamBillingPersistenceFailureDoesNotChargeOrConsumeWebhook(t *testing.T) {
	f := newSecurityBillingFixture(t)
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 200)
	before := f.read()
	restore := blockIdentityCommit(t, f.g)
	f.checkout(t, 502)
	if f.p.creates != 0 || !reflect.DeepEqual(before, f.read()) {
		t.Fatal("failed reservation write reached Stripe or changed memory")
	}
	restore()
	f.reserveAfterQuote(t)
	before = f.read()
	f.p.sub = f.subscription()
	restore = blockIdentityCommit(t, f.g)
	payload := f.payload(t, "evt_retryable_write", "customer.subscription.updated", nil)
	f.webhook(t, payload, "whsec_fixture", time.Now(), 502)
	if !reflect.DeepEqual(before, f.read()) || f.allowed() {
		t.Fatal("failed webhook commit retained entitlement or consumed receipt")
	}
	restore()
	f.webhook(t, payload, "whsec_fixture", time.Now(), 200)
	if !f.allowed() || len(f.read().Events) != 1 {
		t.Fatal("retry could not recover failed webhook commit")
	}
}
func (f *securityBillingFixture) reserveAfterQuote(t *testing.T) {
	t.Helper()
	f.p.create = func(*stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		return nil, errors.New("response lost")
	}
	f.checkout(t, 502)
	f.p.create = nil
}

func TestTeamBillingRevokedOwnerCannotRetryPendingCheckout(t *testing.T) {
	f := newSecurityBillingFixture(t)
	f.reserve(t)
	other := addUser(t, f.h, f.token, "billing-other-owner", "member")
	otherToken := loginAs(t, f.h, other.Username)
	base := "/v1/teams/" + f.team.ID
	team := teamFromResponse(t, request(t, f.h, "POST", base+"/members", f.token, teamMemberBody(f.team.Revision, other.Username, "owner", TeamGrants{Admin: true, Execute: true, Secrets: true, Deploy: true}), 200))
	creates := f.p.creates
	f.p.priceHook = func() {
		request(t, f.h, "PUT", base+"/members/"+f.owner.ID, otherToken, teamMemberBody(team.Revision, "", "viewer", TeamGrants{}), 200)
	}
	data, _ := json.Marshal(f.checkoutBody())
	r := httptest.NewRequest("POST", f.base+"/checkout", bytes.NewReader(data))
	r.Header.Set("Authorization", "Bearer "+f.token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	if w.Code < 400 || f.p.creates != creates {
		t.Fatalf("revoked owner reached chargeable retry: status=%d before=%d after=%d", w.Code, creates, f.p.creates)
	}
}

func TestTeamBillingAdminGrantDoesNotImplySubscriptionOwnership(t *testing.T) {
	f := newSecurityBillingFixture(t)
	u := addUser(t, f.h, f.token, "billing-admin", "member")
	token := loginAs(t, f.h, u.Username)
	request(t, f.h, "POST", "/v1/teams/"+f.team.ID+"/members", f.token, teamMemberBody(f.team.Revision, u.Username, "editor", TeamGrants{Admin: true}), 200)
	for _, action := range []string{"quote", "checkout", "refresh", "portal", "reconcile"} {
		request(t, f.h, "POST", f.base+"/"+action, token, map[string]any{}, 403)
	}
	view := request(t, f.h, "GET", f.base, token, nil, 200)
	if f.p.calls != 0 || strings.Contains(view.Body.String(), "quote") || strings.Contains(view.Body.String(), "acct_fixture") || strings.Contains(view.Body.String(), "whsec_") {
		t.Fatal("team admin gained billing ownership or merchant details")
	}
}

func TestTeamBillingExpiredIdempotencyCannotStartAnotherCheckout(t *testing.T) {
	f := newSecurityBillingFixture(t)
	b := f.reserve(t)
	b.Reservation.CreatedAt = time.Now().Add(-25 * time.Hour).Unix()
	f.set(b)
	before := f.read()
	calls := f.p.calls
	f.checkout(t, 409)
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 409)
	if f.p.calls != calls || !reflect.DeepEqual(before, f.read()) {
		t.Fatal("expired provider idempotency window reset pending reservation")
	}
	// Reconciliation only accepts the original provider binding.
	f.p.session = &stripe.CheckoutSession{ID: "cs_foreign", Mode: "subscription", Status: "open", ClientReferenceID: f.team.ID, Metadata: f.g.billingMetadata("other-reservation"), URL: "https://checkout.stripe.com/c/pay/foreign"}
	request(t, f.h, "POST", f.base+"/reconcile", f.token, map[string]any{"sessionId": "cs_foreign", "expectedRevision": before.Revision}, 409)
	if !reflect.DeepEqual(before, f.read()) {
		t.Fatal("foreign receipt replaced expired pending reservation")
	}
}

func TestTeamBillingDeadlineRevokesAlreadyAdmittedRequests(t *testing.T) {
	f := newSecurityBillingFixture(t)
	b := f.reserve(t)
	b.Status = "active"
	b.SubscriptionID = "sub_fixture"
	b.CustomerID = "cus_fixture"
	b.PeriodEnd = time.Now().Add(time.Second).Unix()
	b.Reservation.State = "complete"
	f.set(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.g.mu.Lock()
	f.g.teamRequests = map[*teamRequest]context.CancelFunc{{teamSocketScope: teamSocketScope{TeamID: f.team.ID, UserID: f.owner.ID}}: cancel}
	f.g.scheduleBillingExpiryLocked()
	f.g.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("billing expiry left existing request admitted")
	}
	if f.allowed() {
		t.Fatal("past paid deadline still permits execution")
	}
}

func TestTeamBillingCurrentSubscriptionDowngradesRevokeCachedEntitlement(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		mutate       func(*securityBillingProvider)
	}{
		{"canceled archived price", "canceled", func(p *securityBillingProvider) { p.sub.Status = "canceled"; p.price.Active = false }},
		{"changed current quantity", "plan_mismatch", func(p *securityBillingProvider) { p.sub.Items.Data[0].Quantity = 2 }},
		{"invalid current plan", "plan_mismatch", func(p *securityBillingProvider) { p.price.Currency = "jpy" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSecurityBillingFixture(t)
			f.reserve(t)
			f.p.sub = f.subscription()
			f.webhook(t, f.payload(t, "evt_paid", "customer.subscription.updated", nil), "whsec_fixture", time.Now(), 200)
			if !f.allowed() {
				t.Fatal("fixture did not establish paid entitlement")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.g.mu.Lock()
			f.g.teamRequests = map[*teamRequest]context.CancelFunc{{teamSocketScope: teamSocketScope{TeamID: f.team.ID, UserID: f.owner.ID}}: cancel}
			f.g.mu.Unlock()
			tc.mutate(f.p)
			f.webhook(t, f.payload(t, "evt_downgrade", "customer.subscription.updated", nil), "whsec_fixture", time.Now(), 200)
			b := f.read()
			if b.Status != tc.status || b.PeriodEnd != 0 || f.allowed() || ctx.Err() == nil || len(b.Events) != 2 {
				t.Fatalf("known subscription downgrade retained paid access: status=%s period=%d canceled=%v events=%d", b.Status, b.PeriodEnd, ctx.Err() != nil, len(b.Events))
			}
		})
	}
}
