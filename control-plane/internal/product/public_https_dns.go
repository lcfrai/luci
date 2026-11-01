package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
)

const publicHTTPSDNSAPI = "2018-05-01"

var publicHTTPSZoneID = regexp.MustCompile(`(?i)^/subscriptions/([a-f0-9-]{36})/resourceGroups/([A-Za-z0-9_.()-]{1,90})/providers/Microsoft.Network/dnsZones/([a-z0-9.-]{3,253})$`)
var errPublicHTTPSAbsent = errors.New("the exact ARM DNS object is absent")

func canonicalPublicHTTPSZoneID(value string) string {
	parts := publicHTTPSZoneID.FindStringSubmatch(strings.TrimSpace(value))
	if len(parts) != 4 {
		return ""
	}
	return "/subscriptions/" + strings.ToLower(parts[1]) + "/resourceGroups/" + parts[2] + "/providers/Microsoft.Network/dnsZones/" + strings.ToLower(parts[3])
}

type publicHTTPSZone struct {
	ID         string `json:"id"`
	ETag       string `json:"etag"`
	Name       string `json:"name"`
	Properties struct {
		ZoneType    string   `json:"zoneType"`
		NameServers []string `json:"nameServers"`
	} `json:"properties"`
}
type publicHTTPSDNSRecord struct {
	ID         string `json:"id"`
	ETag       string `json:"etag"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Properties struct {
		TTL      int64 `json:"TTL"`
		ARecords []struct {
			IP string `json:"ipv4Address"`
		} `json:"ARecords"`
		Metadata       map[string]string `json:"metadata"`
		TargetResource any               `json:"targetResource"`
	} `json:"properties"`
}
type publicHTTPSBearerKey struct{}
type publicHTTPSBearerCache struct {
	rows    map[string]azureClusterBearer
	targets map[string]deploymentkube.Target
}

func publicHTTPSRequestContext(ctx context.Context) context.Context {
	ctx = WithKubernetesTargetResolution(ctx)
	if _, ok := ctx.Value(publicHTTPSBearerKey{}).(*publicHTTPSBearerCache); !ok {
		ctx = context.WithValue(ctx, publicHTTPSBearerKey{}, &publicHTTPSBearerCache{rows: map[string]azureClusterBearer{}, targets: map[string]deploymentkube.Target{}})
	}
	return ctx
}
func publicHTTPSHeader(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n\x00")
}
func (s *Server) publicHTTPSJob(row storedPublicHTTPS) (CloudJob, error) {
	s.mu.RLock()
	j, ok := s.jobs[row.Plan.AzureJobID]
	s.mu.RUnlock()
	if !ok || j.Kind != "aks" || !j.Owned || j.DeleteAttempted || j.Status != "resources_ready" || j.Cluster == nil || j.Cluster.ResourceUID != row.Plan.AzureClusterUID || j.Plan.Foundation == nil || j.Plan.Foundation.Cluster == nil || j.Plan.Foundation.Actor != row.Actor || j.Plan.Foundation.PlanDigest != row.Plan.AzureJobPlanDigest || !validFoundationPlan(j.Plan) {
		return CloudJob{}, ErrPublicHTTPS
	}
	return j, nil
}
func (s *Server) publicHTTPSARMBearer(ctx context.Context, row storedPublicHTTPS) (string, error) {
	j, e := s.publicHTTPSJob(row)
	if e != nil || s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return "", ErrPublicHTTPS
	}
	cache, _ := ctx.Value(publicHTTPSBearerKey{}).(*publicHTTPSBearerCache)
	key := j.Plan.Foundation.PlanDigest
	if cache != nil {
		if b, ok := cache.rows[key]; ok && time.Until(b.expires) > 2*time.Minute {
			return b.token, nil
		}
	}
	fit := j.Plan.Foundation.Cluster.Fit
	if _, e = s.azureClusterToken(ctx, row.Actor, fit.Spec, &fit.Principal); e != nil {
		return "", ErrPublicHTTPS
	}
	raw, e := s.runner(ctx, "az", []string{"account", "get-access-token", "--subscription", fit.Spec.SubscriptionID, "--resource", "https://management.azure.com/", "--output", "json"}, "")
	var v struct {
		AccessToken  string `json:"accessToken"`
		TokenType    string `json:"tokenType"`
		Tenant       string `json:"tenant"`
		Subscription string `json:"subscription"`
	}
	if e != nil || len(raw) > 64<<10 || json.Unmarshal([]byte(raw), &v) != nil || v.TokenType != "Bearer" || strings.ToLower(v.Tenant) != fit.Spec.TenantID || strings.ToLower(v.Subscription) != fit.Spec.SubscriptionID {
		return "", ErrPublicHTTPS
	}
	bearer, e := azureBearerClaimsForAudience(v.AccessToken, fit.Principal, time.Now(), "https://management.azure.com/")
	if e != nil || bearer.principal != fit.Principal || s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return "", ErrPublicHTTPS
	}
	if _, e = s.azureClusterToken(ctx, row.Actor, fit.Spec, &fit.Principal); e != nil {
		return "", ErrPublicHTTPS
	}
	if cache != nil {
		cache.rows[key] = bearer
	}
	return bearer.token, nil
}

// Exact authenticated status is needed: a sanitized CLI error is never 404.
// This reader has no mutating verb or caller-supplied URL/header/token surface.
func (s *Server) publicHTTPSARMRead(ctx context.Context, row storedPublicHTTPS, kind string, out any) error {
	endpoint := row.Plan.AzureDNSZoneID
	switch kind {
	case "zone":
	case "A", "AAAA", "CNAME":
		endpoint += "/" + kind + "/" + url.PathEscape(row.Plan.AzureDNSRecordName)
	default:
		return ErrPublicHTTPS
	}
	if !publicHTTPSZoneID.MatchString(row.Plan.AzureDNSZoneID) {
		return ErrPublicHTTPS
	}
	token, e := s.publicHTTPSARMBearer(ctx, row)
	if e != nil {
		return e
	}
	request, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://management.azure.com"+endpoint+"?api-version="+publicHTTPSDNSAPI, nil)
	if e != nil {
		return ErrPublicHTTPS
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	transport := &http.Transport{Proxy: nil, DialContext: researchDial, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableCompression: true, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(request)
	if e != nil {
		return ErrPublicHTTPS
	}
	raw, re := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
	ce := response.Body.Close()
	if re != nil || ce != nil || len(raw) > 256<<10 || s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return ErrPublicHTTPS
	}
	if response.StatusCode == 404 {
		return errPublicHTTPSAbsent
	}
	if response.StatusCode != 200 || decodeBoundedAzureJSON(string(raw), out) != nil {
		return ErrPublicHTTPS
	}
	return nil
}
func publicHTTPSZoneMatches(row storedPublicHTTPS, zone publicHTTPSZone) bool {
	parts := publicHTTPSZoneID.FindStringSubmatch(row.Plan.AzureDNSZoneID)
	return len(parts) == 4 && strings.EqualFold(zone.ID, row.Plan.AzureDNSZoneID) && zone.Name == parts[3] && zone.Properties.ZoneType == "Public" && publicHTTPSHeader(zone.ETag) && zone.ETag == row.Plan.AzureDNSZoneETag
}
func publicHTTPSDNSMetadata(row storedPublicHTTPS) map[string]string {
	return map[string]string{"luci-project": row.ProjectID, "luci-https": row.ID, "luci-service-uid": row.Plan.Destination.ServiceUID}
}
func publicHTTPSDNSMatches(row storedPublicHTTPS, record publicHTTPSDNSRecord) bool {
	if !row.DNSAcknowledged || !strings.EqualFold(record.ID, row.Plan.AzureDNSZoneID+"/A/"+row.Plan.AzureDNSRecordName) || record.ETag != row.DNSRecordETag || record.Name != row.Plan.AzureDNSRecordName || record.Type != "Microsoft.Network/dnsZones/A" || record.Properties.TTL != 300 || len(record.Properties.ARecords) != 1 || record.Properties.ARecords[0].IP != row.Address || record.Properties.TargetResource != nil || len(record.Properties.Metadata) != 3 {
		return false
	}
	for k, v := range publicHTTPSDNSMetadata(row) {
		if record.Properties.Metadata[k] != v {
			return false
		}
	}
	return true
}
func (s *Server) publicHTTPSDNSAbsent(ctx context.Context, row storedPublicHTTPS) error {
	for _, kind := range []string{"A", "AAAA", "CNAME"} {
		var record publicHTTPSDNSRecord
		if e := s.publicHTTPSARMRead(ctx, row, kind, &record); !errors.Is(e, errPublicHTTPSAbsent) {
			return ErrPublicHTTPS
		}
	}
	return nil
}
func (s *Server) publicHTTPSDNSMutation(ctx context.Context, row storedPublicHTTPS, remove bool) error {
	j, e := s.publicHTTPSJob(row)
	if e != nil {
		return e
	}
	endpoint := "https://management.azure.com" + row.Plan.AzureDNSZoneID + "/A/" + url.PathEscape(row.Plan.AzureDNSRecordName) + "?api-version=" + publicHTTPSDNSAPI
	method, header := "PUT", "If-None-Match=*"
	args := []string{}
	if remove {
		if !row.DNSAcknowledged || !publicHTTPSHeader(row.DNSRecordETag) {
			return ErrPublicHTTPS
		}
		method, header = "DELETE", "If-Match="+row.DNSRecordETag
	}
	args = []string{"rest", "--method", method, "--url", endpoint, "--headers", header, "--output", "json"}
	if !remove {
		body := map[string]any{"properties": map[string]any{"TTL": 300, "ARecords": []map[string]string{{"ipv4Address": row.Address}}, "metadata": publicHTTPSDNSMetadata(row)}}
		raw, _ := json.Marshal(body)
		args = append(args, "--body", string(raw))
	}
	raw, e := s.cloudJobCommandBefore(ctx, j, "az", args, "", func(c context.Context) error {
		if _, e := s.publicHTTPSMutationTarget(c, row); e != nil {
			return e
		}
		if !remove {
			var zone publicHTTPSZone
			if s.publicHTTPSARMRead(c, row, "zone", &zone) != nil || !publicHTTPSZoneMatches(row, zone) {
				return ErrPublicHTTPS
			}
		}
		// This late parent seal and the conditional child write are not an atomic
		// Azure transaction. Deletion instead retains its own original record ETag.
		return s.publicHTTPSGuard(c, row, !remove, false)
	})
	if e != nil {
		return ErrPublicHTTPS
	}
	p, e := s.resolveProject(row.ProjectID)
	if e != nil {
		return ErrPublicHTTPS
	}
	if remove {
		_, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			if !r.DNSDeleteAttempted || r.DNSRecordETag != row.DNSRecordETag {
				return ErrPublicHTTPS
			}
			r.DNSAbsentObserved = false
			return nil
		})
		return e
	}
	var record publicHTTPSDNSRecord
	if decodeBoundedAzureJSON(raw, &record) != nil || !publicHTTPSHeader(record.ETag) {
		return ErrPublicHTTPS
	}
	acknowledged := row
	acknowledged.DNSAcknowledged = true
	acknowledged.DNSRecordETag = record.ETag
	if !publicHTTPSDNSMatches(acknowledged, record) {
		return ErrPublicHTTPS
	}
	_, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
		if !r.DNSAttempted || r.DNSAcknowledged && r.DNSRecordETag != record.ETag {
			return ErrPublicHTTPS
		}
		r.DNSAcknowledged = true
		r.DNSRecordETag = record.ETag
		return nil
	})
	return e
}
