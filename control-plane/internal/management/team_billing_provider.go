package management

import (
	"context"
	stripe "github.com/stripe/stripe-go/v87"
	"net/http"
	"time"
)

// StripeBillingConfig is startup-only merchant configuration. Secrets never enter
// team state, worker environments, audit events or API responses.
type StripeBillingConfig struct {
	APIKey, WebhookSecret, AccountID, PriceID, TermsURL, PrivacyURL string
}

type stripeBillingProvider interface {
	Account(context.Context) (*stripe.Account, error)
	Price(context.Context, string) (*stripe.Price, error)
	CreateCheckout(context.Context, *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	Checkout(context.Context, string) (*stripe.CheckoutSession, error)
	Subscription(context.Context, string) (*stripe.Subscription, error)
	Portal(context.Context, *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error)
}
type stripeSDKProvider struct{ client *stripe.Client }

func newStripeProvider(key string) stripeBillingProvider {
	cfg := &stripe.BackendConfig{HTTPClient: &http.Client{Timeout: 20 * time.Second}, MaxNetworkRetries: stripe.Int64(0), LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}}
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, cfg)
	return &stripeSDKProvider{stripe.NewClient(key, stripe.WithBackends(&stripe.Backends{API: backend}))}
}
func (p *stripeSDKProvider) Account(ctx context.Context) (*stripe.Account, error) {
	return p.client.V1Accounts.Retrieve(ctx, &stripe.AccountRetrieveParams{})
}
func (p *stripeSDKProvider) Price(ctx context.Context, id string) (*stripe.Price, error) {
	return p.client.V1Prices.Retrieve(ctx, id, &stripe.PriceRetrieveParams{})
}
func (p *stripeSDKProvider) CreateCheckout(ctx context.Context, v *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	return p.client.V1CheckoutSessions.Create(ctx, v)
}
func (p *stripeSDKProvider) Checkout(ctx context.Context, id string) (*stripe.CheckoutSession, error) {
	return p.client.V1CheckoutSessions.Retrieve(ctx, id, &stripe.CheckoutSessionRetrieveParams{})
}
func (p *stripeSDKProvider) Subscription(ctx context.Context, id string) (*stripe.Subscription, error) {
	return p.client.V1Subscriptions.Retrieve(ctx, id, &stripe.SubscriptionRetrieveParams{})
}
func (p *stripeSDKProvider) Portal(ctx context.Context, v *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	return p.client.V1BillingPortalSessions.Create(ctx, v)
}
