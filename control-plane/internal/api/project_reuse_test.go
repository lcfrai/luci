package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type reuseRoundTrip func(*http.Request) (*http.Response, error)

func (f reuseRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReuseCatalogueQueriesCannotSelectAnEndpointOrIncludePrivateFields(t *testing.T) {
	p, err := reuseCatalogueParams(reuseCatalogueRequest{Action: "search", Query: "accessible React table"})
	if err != nil || p.Get("limit") != "12" || p.Get("page") != "1" {
		t.Fatal(p, err)
	}
	for _, r := range []reuseCatalogueRequest{{Action: "search", Query: "x", ID: "forged"}, {Action: "search", Query: "x\nsecret"}, {Action: "search", Query: strings.Repeat("x", 201)}, {Action: "inspect", ID: "bad", CatalogueHash: strings.Repeat("a", 64)}, {Action: "install"}} {
		if _, err := reuseCatalogueParams(r); err == nil {
			t.Fatal("accepted invalid query", r.Action)
		}
	}
}
func TestReuseCatalogueBindsSelectedSnapshotAndBoundsResponses(t *testing.T) {
	u := "https://github.com/example/component"
	sum := sha256.Sum256([]byte(u))
	id := hex.EncodeToString(sum[:])[:20]
	licence := "MIT"
	item := reuseCatalogueItem{ID: id, Name: "Component", URL: u, License: reuseCatalogueLicence{SPDX: &licence, Status: "known", Scope: "repository", Basis: "upstream-metadata"}, ObservedAt: "2026-10-30T00:00:00Z", Citations: []reuseCatalogueCitation{{RecordURL: u, SHA256: strings.Repeat("c", 64), ObservedAt: "2026-10-30T00:00:00Z"}}}
	page := reuseCataloguePage{GeneratedAt: "2026-10-30T01:00:00Z", ContentHash: strings.Repeat("a", 64), Item: &item}
	raw, _ := json.Marshal(page)
	calls := 0
	client := &http.Client{Transport: reuseRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "lcfr.ai" || r.URL.Path != "/api/catalogue" || r.Method != "GET" || r.Header.Get("Authorization") != "" || r.Body != nil {
			t.Fatal("request scope changed")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw))), Header: http.Header{}}, nil
	})}
	params := url.Values{"action": {"inspect"}, "id": {id}, "contentHash": {page.ContentHash}}
	got, digest, err := readReuseCatalogue(context.Background(), params, client)
	if err != nil {
		t.Fatal(err)
	}
	source, err := reuseSource(got, digest)
	if err != nil || source.URL != u || calls != 1 {
		t.Fatal(source, err, calls)
	}
	params.Set("contentHash", strings.Repeat("b", 64))
	if _, _, err = readReuseCatalogue(context.Background(), params, client); err == nil {
		t.Fatal("accepted wrong catalogue")
	}
	raw = []byte(strings.Repeat("x", glassesResponseLimit+1))
	if _, _, err = readReuseCatalogue(context.Background(), params, client); err == nil {
		t.Fatal("accepted oversized body")
	}
}

func TestReuseSearchValidatesMetadataAndDropsUninspectedCitations(t *testing.T) {
	u := "https://github.com/example/component"
	sum := sha256.Sum256([]byte(u))
	licence := "MIT"
	framework := "React"
	item := reuseCatalogueItem{ID: hex.EncodeToString(sum[:])[:20], Name: "Component", URL: u, Framework: &framework, License: reuseCatalogueLicence{SPDX: &licence, Status: "known", Scope: "repository", Basis: "upstream-metadata"}, ObservedAt: "2026-10-30T00:00:00Z", Citations: []reuseCatalogueCitation{{RecordURL: "uninspected upstream field"}}}
	page := reuseCataloguePage{GeneratedAt: "2026-10-30T01:00:00Z", ContentHash: strings.Repeat("a", 64), Page: 1, Limit: 12, Total: 1, Items: []reuseCatalogueItem{item}}
	client := &http.Client{Transport: reuseRoundTrip(func(r *http.Request) (*http.Response, error) {
		raw, _ := json.Marshal(page)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw))), Header: http.Header{}}, nil
	})}
	params := url.Values{"action": {"search"}, "query": {"table"}, "page": {"1"}, "limit": {"12"}}
	got, _, err := readReuseCatalogue(context.Background(), params, client)
	if err != nil || len(got.Items) != 1 || got.Items[0].Citations != nil {
		t.Fatal("summary leaked uninspected citations", got, err)
	}
	for _, alter := range []func(*reuseCatalogueItem){func(r *reuseCatalogueItem) { r.License.Scope = "unknown-scope" }, func(r *reuseCatalogueItem) { r.License.Basis = "guess" }, func(r *reuseCatalogueItem) { v := ""; r.License.SPDX = &v }, func(r *reuseCatalogueItem) { v := string([]byte{1}); r.Framework = &v }} {
		page.Items = []reuseCatalogueItem{item}
		alter(&page.Items[0])
		if _, _, err := readReuseCatalogue(context.Background(), params, client); err == nil {
			t.Fatal("accepted invalid search metadata")
		}
	}
}
