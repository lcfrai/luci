# Managed team subscriptions

Self-hosted Luci remains MIT open source with no required subscription. The
optional dedicated managed installation supports one fixed-price Stripe
subscription per bound team. Model usage and infrastructure are billed
separately. This implementation is not evidence of a live merchant account,
completed charge, tax registration or dedicated host isolation.

## Operator configuration

First commission exactly one team and bind the installation using
`LUCI_MANAGED_TEAM_ID`, following [team operations](TEAM-SERVICE-OPERATIONS.md).
Configure the exact HTTPS `LUCI_PUBLIC_APP_ORIGIN`, then these startup values in
the management service's private environment (or a private Compose environment):

| Variable | Required value |
| --- | --- |
| `LUCI_STRIPE_API_KEY` | Merchant `sk_test_…` during commissioning, then deliberate live configuration |
| `LUCI_STRIPE_WEBHOOK_SECRET` | The endpoint's `whsec_…` signing secret |
| `LUCI_STRIPE_ACCOUNT_ID` | The merchant's exact `acct_…` account ID |
| `LUCI_STRIPE_PRICE_ID` | An active fixed recurring `price_…` ID |
| `LUCI_BILLING_TERMS_URL` | Published HTTPS terms, reviewed for this service/operator |
| `LUCI_BILLING_PRIVACY_URL` | Published HTTPS privacy notice, reviewed for this service/operator |

No price amount is supplied by Luci. The owner reviews the amount fetched from
Stripe before creating checkout. The initial supported currencies are USD, EUR,
GBP, CAD, AUD, NZD, SGD, CHF, SEK, NOK and DKK (two decimal minor units). Other
currencies fail closed before checkout. Plans must be licensed, per-unit,
quantity one, monthly or yearly with interval count one; tiered, metered,
transformed or custom-price plans are rejected. Promotions and additional
checkout items are not enabled. Adaptive Pricing is explicitly disabled to
retain the acknowledged plan currency. Luci does not invent a trial.

Configure Stripe Customer Portal to permit cancellation of this plan. Review
merchant tax/invoice settings and the final Stripe checkout total. An exclusive
price is displayed as exclusive; Luci does not claim that its quote calculates
all taxes. Publish the terms/privacy documents with actual operator identity,
contact, service scope and applicable legal/tax decisions before live charging.
The repository's legal drafts require this operator review.

Register the HTTPS endpoint
`POST https://<exact-app-origin>/v1/billing/stripe/webhook` for
`checkout.session.completed`, `checkout.session.expired`,
`customer.subscription.created`, `customer.subscription.updated` and
`customer.subscription.deleted`. Use the API version of the pinned official
`stripe-go/v87` SDK: `2026-09-30.endive`.
The SDK validates API-version compatibility, signatures and a five-minute
signed timestamp tolerance. Forward the original body and `Stripe-Signature`
header without rewriting them. Keep system time synchronized.

Secrets are never sent to team workers or exposed through the billing API.
Existing private billing identity pins the merchant account, test/live mode,
price, app origin and legal document URLs. Rotating API and webhook secrets for
the same identity is supported; removing configuration or changing that
identity fails startup. A test installation must be a separate private state
from a production live installation; test-mode state is not production proof.
Archive/reconcile existing billing state before any deliberate plan migration.
There is no silent plan or merchant migration endpoint.

## Owner workflow and recovery

Open the team's **Service and data** pane. **Review current subscription price**
fetches the current merchant price. A quote lasts fifteen minutes and includes
currency, billing interval, tax behavior and the exact terms/privacy URLs. The
owner explicitly acknowledges the recurring price, terms and separate model and
infrastructure costs before **Prepare Stripe checkout**. Open the returned
Stripe link and complete the provider's checkout. A checkout return URL grants
no access. **Verify subscription with Stripe** reconciles the current provider
subscription; signed webhooks perform the same verification automatically.

Checkout reserves its unique ID, exact quote, owner acknowledgement and timestamp
in private durable state before contacting Stripe. A network error retains this
reservation; **Resume reserved checkout** reuses its exact provider idempotency
key and parameters. Never start a second subscription to work around an unknown
response. Automated retry stops after 23 hours, within Stripe's normal 24-hour
idempotency retention. The operator can locate the Checkout session in the
merchant dashboard by `luci_team` and `luci_reservation` metadata, then give the
owner its `cs_…` receipt ID. **Recover a checkout receipt** retrieves and verifies
that exact session against the existing reservation. A confirmed expired session
permits a fresh quote; an unknown outcome does not.

**Manage or cancel subscription** opens the customer-bound Stripe portal and
remains available during operator suspension. Cancellation retains team context
and data. Active cancellation-at-period-end retains paid authority only until the
verified period deadline. Confirmed changes to the current subscription plan revoke access as `plan_mismatch`
until corrected and verified; archiving a price cannot prevent a cancellation
from revoking authority. Failed/unpaid/paused/canceled states block new
execution while preserving context access and permitted cleanup. Cached paid
authority expires even if webhooks stop; existing terminal/browser connections
close at that deadline. Existing owned containers are not automatically deleted
or stopped, and their infrastructure costs can continue until explicitly stopped.

Operator suspension is independent. Payment success can never override a manual
suspension. Setting the service's label to self-hosted cannot bypass billing on a
configured managed installation. Membership and project capabilities continue
to apply before billing admission; a paid subscription does not grant membership.

## Data, verification and remaining commissioning gates

Private identity state retains merchant/config digests, customer/subscription/
checkout IDs, status and verified period, the accepted price/terms receipt,
reservation identity and owner, and up to 128 verified event IDs/body digests.
It does not retain card numbers, billing addresses, provider email or full webhook
payloads. Backups therefore contain private billing authority and must use the
managed-host backup protections. Deleting an application data request is not
Stripe subscription cancellation or payment-record erasure.

The source tests drive the official SDK through a local HTTP provider fixture,
sign genuine-format webhook payloads, exercise ambiguous-response recovery,
exact idempotency and price bindings, replay, suspension and secret rotation.
They make no real charges. Production commissioning still requires a real
merchant account, published reviewed legal documents, reachable trusted HTTPS,
correct portal/webhook/tax configuration, successful test-mode checkout,
webhook/renewal/cancellation/recovery evidence, and a separately authorized live
payment verification. No live Stripe credentials or charges were used for this
implementation.
