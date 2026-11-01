package product

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const stripeCommercialVersion = "2024-06-20"

var errCommercialStripeAccess = errors.New("Stripe refused this restricted key. Check its Account and Charges read permissions and the selected test/live account. No payment or Stripe setting was changed.")
var errCommercialStripeRate = errors.New("Stripe rate-limited this read. Completed snapshots and saved collection progress remain; continue later or let the configured observation route resume.")

const stripeCommercialSemantics = "Captured successful payments for charges created in this UTC window and matching the configured exact CHARGE metadata filter, when present. Unfiltered figures are account-wide and unallocated, never project outcomes. Charge metadata coverage is not proof of all project sales; Checkout/product metadata is not inferred. Refunds-to-date are read during collection. Amounts are Stripe integer minor units, separated by currency. Unique non-null paying customer references are counted across currencies. Not accounting revenue, payouts, profit, MRR, a period refund total or an atomic financial snapshot."

type stripeCommercialCharge struct {
	ID             string            `json:"id"`
	Object         string            `json:"object"`
	Created        int64             `json:"created"`
	Currency       string            `json:"currency"`
	CapturedAmount *int64            `json:"amount_captured"`
	RefundedAmount *int64            `json:"amount_refunded"`
	Paid           *bool             `json:"paid"`
	Captured       *bool             `json:"captured"`
	Disputed       *bool             `json:"disputed"`
	Live           *bool             `json:"livemode"`
	Status         string            `json:"status"`
	Customer       json.RawMessage   `json:"customer"`
	Metadata       map[string]string `json:"metadata"`
}
type stripeCommercialPage struct {
	Object string                   `json:"object"`
	URL    string                   `json:"url"`
	More   *bool                    `json:"has_more"`
	Data   []stripeCommercialCharge `json:"data"`
}
type stripeCommercialReader interface {
	Account(context.Context, string) (string, error)
	Charges(context.Context, string, time.Time, time.Time, string) (stripeCommercialPage, error)
}
type stripeCommercialGuardKey struct{}
type stripeCommercialHTTP struct{ transport http.RoundTripper }

func stripeKeyMode(key string) string {
	mode, prefix := "", ""
	if strings.HasPrefix(key, "rk_live_") {
		mode, prefix = "live", "rk_live_"
	} else if strings.HasPrefix(key, "rk_test_") {
		mode, prefix = "test", "rk_test_"
	}
	if mode == "" || len(key) < len(prefix)+16 || len(key) > 512 {
		return ""
	}
	for _, c := range strings.TrimPrefix(key, prefix) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return ""
		}
	}
	return mode
}
func (s *Server) stripeCommercialClient() stripeCommercialReader {
	if s.commercialStripe != nil {
		return s.commercialStripe
	}
	return stripeCommercialHTTP{transport: &http.Transport{Proxy: nil, DialContext: researchDial, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableCompression: true, DisableKeepAlives: true}}
}
func (n stripeCommercialHTTP) read(ctx context.Context, key, path string, query url.Values, out any) error {
	guard, ok := ctx.Value(stripeCommercialGuardKey{}).(func(context.Context) error)
	if !ok || guard(ctx) != nil || stripeKeyMode(key) == "" || path != "/v1/account" && path != "/v1/charges" {
		return ErrCommercial
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.stripe.com"+path+"?"+query.Encode(), nil)
	if err != nil {
		return ErrCommercial
	}
	request.SetBasicAuth(key, "")
	request.Header.Set("Stripe-Version", stripeCommercialVersion)
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Transport: n.transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return ErrCommercial
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || len(raw) > 4<<20 || guard(ctx) != nil {
		return ErrCommercial
	}
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return errCommercialStripeAccess
	}
	if response.StatusCode == 429 {
		return errCommercialStripeRate
	}
	if response.StatusCode != http.StatusOK {
		return ErrCommercial
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(out) != nil {
		return ErrCommercial
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrCommercial
	}
	return nil
}
func (n stripeCommercialHTTP) Account(ctx context.Context, key string) (string, error) {
	var value struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := n.read(ctx, key, "/v1/account", url.Values{}, &value); err != nil {
		return "", err
	}
	if value.Object != "account" || !strings.HasPrefix(value.ID, "acct_") || !commercialOpaque.MatchString(value.ID) {
		return "", ErrCommercial
	}
	return value.ID, nil
}
func (n stripeCommercialHTTP) Charges(ctx context.Context, key string, start, end time.Time, cursor string) (stripeCommercialPage, error) {
	q := url.Values{"limit": {"100"}, "created[gte]": {strconv.FormatInt(start.Unix(), 10)}, "created[lt]": {strconv.FormatInt(end.Unix(), 10)}}
	if cursor != "" {
		if !commercialOpaque.MatchString(cursor) {
			return stripeCommercialPage{}, ErrCommercial
		}
		q.Set("starting_after", cursor)
	}
	var page stripeCommercialPage
	if err := n.read(ctx, key, "/v1/charges", q, &page); err != nil {
		return stripeCommercialPage{}, err
	}
	if page.Object != "list" || page.URL != "/v1/charges" || page.More == nil || page.Data == nil || len(page.Data) > 100 || *page.More && len(page.Data) == 0 {
		return stripeCommercialPage{}, ErrCommercial
	}
	return page, nil
}
func commercialDedup(collection *commercialCollection, kind, value string) string {
	key, _ := hex.DecodeString(collection.DedupSalt)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(kind + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}
func commercialSeen(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, v := range values {
		result[v] = true
	}
	return result
}
func commercialAdd(a, b int64) (int64, error) {
	if a < 0 || b < 0 || b > commercialSafeInteger-a {
		return 0, ErrCommercial
	}
	return a + b, nil
}

// Only aggregate totals and salted dedup material survive this function. Raw
// charge/customer objects, names, emails and card fields never enter storage.
func collectStripePage(collection *commercialCollection, page stripeCommercialPage) error {
	start, e1 := time.Parse("2006-01-02", collection.Snapshot.PeriodStart)
	end, e2 := time.Parse("2006-01-02", collection.Snapshot.PeriodEndExclusive)
	if e1 != nil || e2 != nil || page.More == nil || page.Data == nil || len(page.Data) > 100 || *page.More && len(page.Data) == 0 {
		return ErrCommercial
	}
	seen, customers := commercialSeen(collection.SeenRecords), commercialSeen(collection.SeenCustomers)
	currencies := map[string]CommercialCurrency{}
	for _, v := range collection.Snapshot.Currencies {
		currencies[v.Currency] = v
	}
	last := ""
	newRecords := 0
	for _, row := range page.Data {
		if row.Object != "charge" || !commercialOpaque.MatchString(row.ID) || row.Created < start.Unix() || row.Created >= end.Unix() || len(row.Currency) != 3 || row.Currency != strings.ToLower(row.Currency) || !commercialCurrencyCode.MatchString(strings.ToUpper(row.Currency)) || row.Paid == nil || row.Captured == nil || row.Disputed == nil || row.Live == nil || *row.Live != collection.Snapshot.Live || row.CapturedAmount == nil || row.RefundedAmount == nil || *row.CapturedAmount < 0 || *row.CapturedAmount > commercialSafeInteger || *row.RefundedAmount < 0 || *row.RefundedAmount > *row.CapturedAmount || len(row.Customer) == 0 {
			return ErrCommercial
		}
		switch row.Status {
		case "succeeded", "pending", "failed":
		default:
			return ErrCommercial
		}
		last = row.ID
		id := commercialDedup(collection, "charge", row.ID)
		if seen[id] {
			continue
		}
		if collection.Snapshot.Records >= commercialRecordLimit {
			return errCommercialWindowLimit
		}
		seen[id] = true
		collection.SeenRecords = append(collection.SeenRecords, id)
		collection.Snapshot.Records++
		newRecords++
		filter := collection.Snapshot.StripeFilter
		if filter != nil && row.Metadata[filter.Key] != filter.Value {
			continue
		}
		collection.Snapshot.MatchedCharges++
		if !*row.Paid || !*row.Captured || row.Status != "succeeded" {
			continue
		}
		var customer *string
		if json.Unmarshal(row.Customer, &customer) != nil || customer != nil && !commercialOpaque.MatchString(*customer) {
			return ErrCommercial
		}
		currency := strings.ToUpper(row.Currency)
		bucket := currencies[currency]
		bucket.Currency = currency
		bucket.CapturedPayments++
		var err error
		bucket.CapturedMinor, err = commercialAdd(bucket.CapturedMinor, *row.CapturedAmount)
		if err != nil {
			return err
		}
		bucket.RefundedMinor, err = commercialAdd(bucket.RefundedMinor, *row.RefundedAmount)
		if err != nil {
			return err
		}
		if *row.Disputed {
			bucket.DisputedPayments++
		}
		if customer == nil {
			bucket.PaymentsWithoutCustomer++
		} else {
			key := commercialDedup(collection, "customer", *customer)
			if !customers[key] {
				customers[key] = true
				collection.SeenCustomers = append(collection.SeenCustomers, key)
				collection.Snapshot.PayingCustomerReferences++
			}
		}
		currencies[currency] = bucket
	}
	if *page.More && (last == "" || last == collection.Cursor || newRecords == 0) {
		return ErrCommercial
	}
	collection.Cursor = last
	collection.Page++
	collection.Snapshot.Currencies = commercialSortCurrencies(currencies)
	collection.Complete = !*page.More
	return nil
}
