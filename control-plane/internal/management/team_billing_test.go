package management

import (
	"bytes"
	"context"
	"encoding/json"
	stripe "github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// This fixture drives the official SDK through HTTP; it records actual encoded
// Checkout requests and signs actual webhook bytes with the SDK test helper.
func TestStripeBillingHTTPCheckoutRecoveryWebhookPortalAndRestart(t *testing.T) {
	var mu sync.Mutex
	var forms []url.Values
	var keys []string
	complete := false
	var metadata map[string]string
	var teamID string
	stripeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer sk_test_fixture" {
			t.Error("missing merchant authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/account":
			json.NewEncoder(w).Encode(map[string]any{"id": "acct_fixture", "object": "account"})
		case "GET /v1/prices/price_fixture":
			json.NewEncoder(w).Encode(map[string]any{"id": "price_fixture", "object": "price", "active": true, "type": "recurring", "billing_scheme": "per_unit", "currency": "usd", "unit_amount": 2400, "unit_amount_decimal": "2400", "tax_behavior": "exclusive", "recurring": map[string]any{"interval": "month", "interval_count": 1, "usage_type": "licensed"}})
		case "POST /v1/checkout/sessions":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			forms = append(forms, r.PostForm)
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			metadata = map[string]string{}
			for _, key := range []string{"luci_team", "luci_reservation", "luci_config", "luci_account"} {
				metadata[key] = r.PostForm.Get("metadata[" + key + "]")
				if metadata[key] != r.PostForm.Get("subscription_data[metadata]["+key+"]") {
					t.Error("subscription metadata differs")
				}
			}
			if len(forms) == 1 {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"type": "api_error", "message": "fixture response lost after creation"}})
				return
			}
			fallthrough
		case "GET /v1/checkout/sessions/cs_fixture":
			status := "open"
			var sub any
			var customer any
			if complete {
				status = "complete"
				sub = "sub_fixture"
				customer = "cus_fixture"
			}
			json.NewEncoder(w).Encode(map[string]any{"id": "cs_fixture", "object": "checkout.session", "mode": "subscription", "status": status, "url": "https://checkout.stripe.com/c/pay/cs_fixture", "metadata": metadata, "client_reference_id": teamID, "subscription": sub, "customer": customer})
		case "GET /v1/subscriptions/sub_fixture":
			json.NewEncoder(w).Encode(map[string]any{"id": "sub_fixture", "object": "subscription", "customer": "cus_fixture", "status": "active", "collection_method": "charge_automatically", "metadata": metadata, "items": map[string]any{"object": "list", "has_more": false, "data": []any{map[string]any{"id": "si_fixture", "quantity": 1, "current_period_end": time.Now().Add(time.Hour).Unix(), "price": map[string]any{"id": "price_fixture"}}}}})
		case "POST /v1/billing_portal/sessions":
			r.ParseForm()
			if r.PostForm.Get("customer") != "cus_fixture" || r.PostForm.Get("return_url") != "https://app.example.com/?billing=return" {
				t.Error("portal customer or return URL mismatch")
			}
			json.NewEncoder(w).Encode(map[string]any{"id": "bps_fixture", "object": "billing_portal.session", "url": "https://billing.stripe.com/p/session/fixture"})
		default:
			t.Errorf("unexpected Stripe request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer stripeServer.Close()
	g, h, token, _ := fixture(t, nil)
	team := createTestTeam(t, h, token, "Billing test")
	teamID = team.ID
	opts := g.opts
	opts.ManagedTeamID = team.ID
	opts.PublicAppOrigin = "https://app.example.com"
	opts.StripeBilling = StripeBillingConfig{APIKey: "sk_test_fixture", WebhookSecret: "whsec_fixture", AccountID: "acct_fixture", PriceID: "price_fixture", TermsURL: "https://app.example.com/terms", PrivacyURL: "https://app.example.com/privacy"}
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(stripeServer.URL), HTTPClient: stripeServer.Client(), MaxNetworkRetries: stripe.Int64(0), LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}})
	opts.stripeProvider = &stripeSDKProvider{stripe.NewClient(opts.StripeBilling.APIKey, stripe.WithBackends(&stripe.Backends{API: backend}))}
	bound, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer bound.Close(context.Background())
	h = bound.Handler()
	base := "/v1/teams/" + team.ID + "/billing"
	bound.mu.Lock()
	allowed := bound.teamServiceAllowsExecutionLocked(team.ID)
	bound.mu.Unlock()
	if allowed {
		t.Fatal("unpaid managed team admitted execution")
	}
	request(t, h, "POST", base+"/quote", token, map[string]any{}, 200)
	read := func() *TeamBilling {
		bound.mu.Lock()
		defer bound.mu.Unlock()
		return cloneTeamBilling(bound.state.Teams[team.ID].Billing)
	}
	b := read()
	checkout := func(status int) {
		b = read()
		request(t, h, "POST", base+"/checkout", token, map[string]any{"quoteId": b.Quote.ID, "expectedRevision": b.Revision, "acknowledgePrice": true, "acknowledgeTerms": true}, status)
	}
	checkout(502)
	b = read()
	if b.Reservation == nil || b.Reservation.State != "reserved" {
		t.Fatal("ambiguous checkout reservation was lost")
	}
	persisted, err := os.ReadFile(filepath.Join(bound.dir, "identity.json"))
	if err != nil || !bytes.Contains(persisted, []byte(b.Reservation.ID)) {
		t.Fatal("reservation not durable")
	}
	checkout(200)
	mu.Lock()
	if len(forms) != 2 || keys[0] == "" || keys[0] != keys[1] || forms[0].Encode() != forms[1].Encode() {
		t.Error("ambiguous retry changed Stripe idempotency key or params")
	}
	if forms[0].Get("adaptive_pricing[enabled]") != "false" || forms[0].Get("line_items[0][quantity]") != "1" || forms[0].Get("success_url") != "https://app.example.com/?billing=return" {
		t.Error("checkout did not use fixed quantity and configured origin")
	}
	complete = true
	mu.Unlock()
	bound.mu.Lock()
	allowed = bound.teamServiceAllowsExecutionLocked(team.ID)
	bound.mu.Unlock()
	if allowed {
		t.Fatal("checkout URL granted authority before verification")
	}
	payload, _ := json.Marshal(map[string]any{"id": "evt_fixture", "object": "event", "type": "checkout.session.completed", "api_version": stripe.APIVersion, "created": time.Now().Unix(), "data": map[string]any{"object": map[string]any{"id": "cs_fixture"}}})
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: opts.StripeBilling.WebhookSecret})
	for range 2 {
		r := httptest.NewRequest("POST", "/v1/billing/stripe/webhook", bytes.NewReader(payload))
		r.Header.Set("Stripe-Signature", signed.Header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("webhook: %d %s", w.Code, w.Body.String())
		}
	}
	b = read()
	if b.Status != "active" || len(b.Events) != 1 || b.SubscriptionID != "sub_fixture" {
		t.Fatalf("subscription/replay not persisted: %+v", b)
	}
	bound.mu.Lock()
	allowed = bound.teamServiceAllowsExecutionLocked(team.ID)
	bound.mu.Unlock()
	if !allowed {
		t.Fatal("verified subscription unavailable")
	}
	request(t, h, "POST", base+"/portal", token, map[string]any{}, 200)
	// Rotating secrets retains the same merchant authority; removing configuration
	// or switching merchant account cannot silently turn paid access into OSS.
	opts.StripeBilling.APIKey = "sk_test_rotated"
	opts.StripeBilling.WebhookSecret = "whsec_rotated"
	reopened, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	if reopened.state.Teams[team.ID].Billing.SubscriptionID != "sub_fixture" {
		t.Fatal("secret rotation lost subscription")
	}
	changed := opts
	changed.StripeBilling.AccountID = "acct_foreign"
	if bad, e := New(changed); e == nil {
		bad.Close(context.Background())
		t.Fatal("merchant identity drift accepted")
	}
	changed.StripeBilling = StripeBillingConfig{}
	if bad, e := New(changed); e == nil {
		bad.Close(context.Background())
		t.Fatal("removing config bypassed billing")
	}
}
