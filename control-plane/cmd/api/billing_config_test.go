package main

import "testing"

func TestBillingConfigIsExplicitManagementOnly(t *testing.T) {
	values := map[string]string{envStateDir: t.TempDir(), envStripeAPIKey: "sk_test_fixture", envStripeWebhookSecret: "whsec_fixture", envStripeAccountID: "acct_fixture", envStripePriceID: "price_fixture", envBillingTermsURL: "https://app.example.com/terms", envBillingPrivacyURL: "https://app.example.com/privacy"}
	cfg, err := loadConfig(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StripeAPIKey != values[envStripeAPIKey] || cfg.StripeWebhookSecret != values[envStripeWebhookSecret] || cfg.StripeAccountID != values[envStripeAccountID] || cfg.StripePriceID != values[envStripePriceID] || cfg.BillingTermsURL != values[envBillingTermsURL] || cfg.BillingPrivacyURL != values[envBillingPrivacyURL] {
		t.Fatal("merchant configuration not parsed exactly")
	}
	values[envWorker] = "1"
	if _, err = loadConfig(env(values)); err == nil {
		t.Fatal("worker accepted merchant credentials")
	}
}
