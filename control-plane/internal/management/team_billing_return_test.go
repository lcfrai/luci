package management

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/form"
)

func assertBillingReturnURL(t *testing.T, raw, teamID, kind string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "app.example.com" || u.Path != "/" || u.User != nil || u.Fragment != "" {
		t.Errorf("billing return escaped configured application origin: %q", raw)
		return
	}
	query := u.Query()
	if len(query) != 2 || len(query["billing"]) != 1 || len(query["billingTeam"]) != 1 || query.Get("billing") != kind || query.Get("billingTeam") != teamID {
		t.Errorf("billing return lost exact team/intent: %q", raw)
	}
}

func TestBillingReturnNavigationUsesManagedTeamAndDistinctIntent(t *testing.T) {
	g := &Gateway{managedTeamID: "team-abcdefghijklmnopqrstuv", opts: Options{PublicAppOrigin: "https://app.example.com"}}
	for _, kind := range []string{"return", "cancel", "portal"} {
		assertBillingReturnURL(t, g.billingReturnURL(kind), g.managedTeamID, kind)
	}
}

func TestBillingReturnLegacyAmbiguousCheckoutRetainsOriginalParametersAfterRestart(t *testing.T) {
	f := newSecurityBillingFixture(t)
	request(t, f.h, "POST", f.base+"/quote", f.token, map[string]any{}, 200)
	b := f.read()
	// Reproduce an on-disk reservation written before scoped billing returns:
	// Stripe may have accepted the original request, but its response was lost.
	b.Reservation = &BillingReservation{ID: "legacy-reservation", CreatedAt: time.Now().Unix(), ActorID: f.owner.ID, Quote: *b.Quote, State: "reserved"}
	b.Revision++
	f.set(b)
	f.g.mu.Lock()
	err := f.g.persistLocked()
	f.g.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(filepath.Join(f.g.dir, "identity.json"))
	if err != nil || bytes.Contains(persisted, []byte("returnUrlVersion")) {
		t.Fatal("fixture does not represent a legacy reservation without a format marker")
	}
	// The full original form is independent of checkoutParams, so changing any
	// parameter while keeping this old idempotency key fails the regression.
	original := url.Values{
		"mode": {"subscription"}, "client_reference_id": {f.team.ID},
		"line_items[0][price]": {b.Quote.Price.ID}, "line_items[0][quantity]": {"1"},
		"success_url":           {"https://app.example.com/?billing=return"},
		"cancel_url":            {"https://app.example.com/?billing=cancel"},
		"expires_at":            {strconv.FormatInt(b.Reservation.CreatedAt+24*60*60, 10)},
		"allow_promotion_codes": {"false"}, "adaptive_pricing[enabled]": {"false"},
	}
	for key, value := range f.g.billingMetadata(b.Reservation.ID) {
		original.Set("metadata["+key+"]", value)
		original.Set("subscription_data[metadata]["+key+"]", value)
	}
	opts := f.g.opts
	if err = f.g.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.g, err = New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer f.g.Close(context.Background())
	f.h = f.g.Handler()
	f.p.create = func(params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
		values := &form.Values{}
		form.AppendTo(values, params)
		encoded, parseErr := url.ParseQuery(values.Encode())
		if parseErr != nil || stripe.StringValue(params.IdempotencyKey) != "luci-team-checkout-legacy-reservation" || encoded.Encode() != original.Encode() {
			t.Error("upgrade changed an ambiguous checkout's original Stripe key or parameters")
		}
		return &stripe.CheckoutSession{ID: "cs_legacy", Mode: "subscription", Status: "open", ClientReferenceID: f.team.ID, Metadata: f.g.billingMetadata(b.Reservation.ID), URL: "https://checkout.stripe.com/c/pay/cs_legacy"}, nil
	}
	f.checkout(t, 200)
	got := f.read()
	if f.p.creates != 1 || got.Reservation.ID != b.Reservation.ID || got.Reservation.ReturnURLVersion != 0 || got.Reservation.SessionID != "cs_legacy" || got.Reservation.State != "open" || f.allowed() {
		t.Fatal("legacy retry changed reservation identity, format or entitlement")
	}
}

func TestBillingReturnRejectsUnknownReservationFormat(t *testing.T) {
	f := newSecurityBillingFixture(t)
	b := f.reserve(t)
	for _, version := range []int{-1, 2} {
		b.Reservation.ReturnURLVersion = version
		if validateTeamBilling(b) == nil {
			t.Fatalf("unsupported return URL version %d accepted", version)
		}
	}
}
