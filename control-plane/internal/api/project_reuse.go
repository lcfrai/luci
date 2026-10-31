package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

const glassesCatalogueEndpoint = "https://lcfr.ai/api/catalogue"
const glassesResponseLimit = 1 << 20

type reuseCatalogueRequest struct {
	Action        string `json:"action"`
	Query         string `json:"query"`
	ID            string `json:"id"`
	CatalogueHash string `json:"catalogueHash"`
}
type reuseCatalogueLicence struct {
	SPDX   *string `json:"spdx"`
	Status string  `json:"status"`
	Scope  string  `json:"scope"`
	Basis  string  `json:"basis"`
}
type reuseCatalogueCitation struct {
	RecordURL  string `json:"recordUrl"`
	SHA256     string `json:"sha256"`
	ObservedAt string `json:"observedAt"`
}
type reuseCatalogueItem struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	URL         string                   `json:"url"`
	Description string                   `json:"description"`
	Framework   *string                  `json:"framework"`
	License     reuseCatalogueLicence    `json:"license"`
	ObservedAt  string                   `json:"observedAt"`
	Citations   []reuseCatalogueCitation `json:"citations,omitempty"`
}
type reuseCataloguePage struct {
	GeneratedAt string               `json:"generatedAt"`
	ContentHash string               `json:"contentHash"`
	Total       int                  `json:"total"`
	Page        int                  `json:"page"`
	Limit       int                  `json:"limit"`
	Items       []reuseCatalogueItem `json:"items"`
	Item        *reuseCatalogueItem  `json:"item"`
}

func reuseHash(value string, n int) bool {
	if len(value) != n {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
func reuseCatalogueParams(req reuseCatalogueRequest) (url.Values, error) {
	p := url.Values{"action": {req.Action}}
	switch req.Action {
	case "search":
		if strings.TrimSpace(req.Query) == "" || len(req.Query) > 200 || !utf8.ValidString(req.Query) || strings.IndexFunc(req.Query, func(r rune) bool { return r < 32 || r == 127 }) >= 0 || req.ID != "" || req.CatalogueHash != "" {
			return nil, errors.New("Enter a public catalogue search of up to 200 UTF-8 bytes")
		}
		p.Set("query", req.Query)
		p.Set("limit", "12")
		p.Set("page", "1")
	case "inspect":
		if req.Query != "" || !reuseHash(req.ID, 20) || !reuseHash(req.CatalogueHash, 64) {
			return nil, errors.New("Select an exact candidate from the returned catalogue snapshot")
		}
		p.Set("id", req.ID)
		p.Set("contentHash", req.CatalogueHash)
	default:
		return nil, errors.New("Choose search or inspect")
	}
	return p, nil
}
func reuseCatalogueText(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && strings.IndexFunc(value, func(r rune) bool { return r < 32 && r != '\t' && r != '\n' && r != '\r' }) < 0
}
func reuseCatalogueItemValid(row reuseCatalogueItem) bool {
	sum := sha256.Sum256([]byte(row.URL))
	if row.ID != hex.EncodeToString(sum[:])[:20] || !product.TaskReusePublicURL(row.URL) || strings.TrimSpace(row.Name) == "" || !reuseCatalogueText(row.Name, 200) || !reuseCatalogueText(row.Description, 1200) || row.Framework != nil && !reuseCatalogueText(*row.Framework, 100) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, row.ObservedAt); err != nil {
		return false
	}
	licence := row.License
	if licence.Status != "known" && licence.Status != "restricted" && licence.Status != "unknown" || (licence.SPDX == nil) != (licence.Status == "unknown") {
		return false
	}
	if licence.Scope != "repository" && licence.Scope != "registry-provider" && licence.Scope != "registry-item" && licence.Scope != "source-artifact" && licence.Scope != "public-page" {
		return false
	}
	if licence.Basis != "upstream-metadata" && licence.Basis != "source-declaration" {
		return false
	}
	return licence.SPDX == nil || strings.TrimSpace(*licence.SPDX) != "" && reuseCatalogueText(*licence.SPDX, 100)
}

// Only this fixed public read is allowed. No source URL, credentials, redirects,
// private task text, remote code or catalogue distribution is fetched implicitly.
func readReuseCatalogue(ctx context.Context, params url.Values, client *http.Client) (reuseCataloguePage, string, error) {
	var out reuseCataloguePage
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, glassesCatalogueEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return out, "", err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return out, "", errors.New("The public catalogue could not be read")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return out, "", errors.New("The public catalogue is unavailable or its snapshot changed; search again")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, glassesResponseLimit+1))
	if err != nil || len(raw) > glassesResponseLimit || !utf8.Valid(raw) {
		return out, "", errors.New("Catalogue response exceeded its read bound")
	}
	if json.Unmarshal(raw, &out) != nil || !reuseHash(out.ContentHash, 64) {
		return out, "", errors.New("Unrecognized catalogue response")
	}
	if _, err := time.Parse(time.RFC3339Nano, out.GeneratedAt); err != nil {
		return out, "", errors.New("Catalogue observation date is missing")
	}
	if params.Get("action") == "search" {
		if out.Items == nil || len(out.Items) > 12 || out.Total < len(out.Items) || out.Total > 100000 || out.Page != 1 || out.Limit != 12 || out.Item != nil {
			return out, "", errors.New("Invalid catalogue page")
		}
		seen := map[string]bool{}
		for i, row := range out.Items {
			if !reuseCatalogueItemValid(row) || seen[row.ID] {
				return out, "", errors.New("Invalid catalogue candidate")
			}
			seen[row.ID] = true
			out.Items[i].Citations = nil
		}
	}
	if params.Get("action") == "inspect" && (out.Item == nil || out.Item.ID != params.Get("id") || out.ContentHash != params.Get("contentHash") || !reuseCatalogueItemValid(*out.Item)) {
		return out, "", errors.New("The selected catalogue source changed")
	}
	sum := sha256.Sum256(raw)
	return out, hex.EncodeToString(sum[:]), nil
}
func reuseSource(page reuseCataloguePage, responseDigest string) (product.TaskReuseSource, error) {
	if page.Item == nil {
		return product.TaskReuseSource{}, errors.New("Missing candidate")
	}
	row := page.Item
	source := product.TaskReuseSource{ID: row.ID, Name: row.Name, URL: row.URL, Description: row.Description, LicenceStatus: row.License.Status, LicenceScope: row.License.Scope, LicenceBasis: row.License.Basis, ObservedAt: row.ObservedAt, CatalogueHash: page.ContentHash, CatalogueGeneratedAt: page.GeneratedAt, ResponseDigest: responseDigest, CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), Citations: []product.TaskReuseCitation{}, CitationCount: len(row.Citations)}
	if row.Framework != nil {
		source.Framework = *row.Framework
	}
	if row.License.SPDX != nil {
		source.Licence = *row.License.SPDX
	}
	seen := map[string]bool{}
	for _, c := range row.Citations {
		key := c.RecordURL + ":" + c.SHA256
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(source.Citations) == 3 {
			break
		}
		source.Citations = append(source.Citations, product.TaskReuseCitation{URL: c.RecordURL, Digest: c.SHA256, ObservedAt: c.ObservedAt})
	}
	source.Digest = product.TaskReuseSourceDigest(source)
	return source, product.ValidateTaskReuseSource(source)
}
func (a *API) handleProjectReuse(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := management.ActorFromContext(r.Context())
	if !ok || actor.Disabled || a.deps.Product == nil {
		httpx.WriteProblem(w, 403, "reuse_unavailable", "Sign in to this project first.")
		return
	}
	ref := management.ExecutionActorRef{UserID: actor.ID, ProjectID: r.PathValue("id")}
	if team, scoped := management.TeamAccessFromContext(r.Context()); scoped {
		ref.TeamID = team.Team.ID
	}
	// The existing read-only branch resolves exact current actor/team/project grants.
	if a.authorizeProjectFeedback(r.Context(), ref, false) != nil {
		httpx.WriteProblem(w, 403, "reuse_unavailable", "Current project read access is required.")
		return
	}
	var req reuseCatalogueRequest
	if !portfolioGoalsDecode(w, r, &req) {
		return
	}
	params, err := reuseCatalogueParams(req)
	if err != nil {
		httpx.WriteProblem(w, 400, "reuse_query_invalid", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Catalogue redirects are not accepted") }}
	page, digest, err := readReuseCatalogue(ctx, params, client)
	if err != nil {
		httpx.WriteProblem(w, 502, "reuse_catalogue_unavailable", err.Error())
		return
	}
	if a.authorizeProjectFeedback(r.Context(), ref, false) != nil {
		httpx.WriteProblem(w, 403, "reuse_unavailable", "Project access changed while reading the catalogue.")
		return
	}
	if req.Action == "search" {
		httpx.WriteJSON(w, 200, map[string]any{"projectId": ref.ProjectID, "catalogueHash": page.ContentHash, "generatedAt": page.GeneratedAt, "query": req.Query, "total": page.Total, "items": page.Items})
		return
	}
	source, err := reuseSource(page, digest)
	if err != nil {
		httpx.WriteProblem(w, 502, "reuse_source_invalid", "This candidate's source metadata is incomplete; nothing has been added.")
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"projectId": ref.ProjectID, "source": source})
}
