package product

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/platform"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

var prototypeReleaseID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// PlatformPrototypeReleases closes over the admitted service instance. Client
// DTOs contain neither a native loopback URL nor an image, port or container.
func PlatformPrototypeReleases(resolve func() *platform.Service) func(context.Context, string) ([]Release, error) {
	return func(ctx context.Context, id string) ([]Release, error) {
		if resolve == nil || resolve() == nil {
			return nil, errors.New("the native application integration is unavailable")
		}
		rows, err := resolve().ApplicationPrototypeReleases(ctx, id)
		if err != nil {
			return nil, err
		}
		result := []Release{}
		for _, row := range rows {
			result = append(result, Release{ID: row.ID, ProjectID: row.ProjectID, Commit: row.Commit, ConfigurationRevision: row.ConfigurationRevision, Target: "application", Status: "ready", BrowserConnectionRequired: true, Message: "Verified owned Docker application. Connect its browser address to open the prototype.", CreatedAt: row.CreatedAt})
		}
		return result, nil
	}
}

func validPrototypeURL(row Release, registry ...*previewgateway.Registry) bool {
	if row.Target == "application" {
		return row.BrowserConnectionRequired && row.URL == ""
	}
	u, err := url.Parse(row.URL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(row.URL, "\\\x00\r\n") {
		return false
	}
	if row.Target == "local" {
		if len(registry) > 0 && registry[0] != nil && registry[0].AcceptsOrigin(u.Scheme+"://"+u.Host) {
			return true
		}
		host := strings.ToLower(u.Hostname())
		port, err := strconv.Atoi(u.Port())
		return err == nil && port > 0 && port <= 65535 && (u.Scheme == "http" || u.Scheme == "https") && (host == "127.0.0.1" || host == "localhost" || previewgateway.IsCapabilityHostname(host))
	}
	// Cloud releases retain the selected deployment target ID in this DTO.
	return prototypeReleaseID.MatchString(row.Target) && u.Scheme == "https"
}

// The catalogue and design save share this source. Call before acquiring a
// journey write lock: provider histories and the platform resolver take locks.
func (s *Server) prototypeReleases(ctx context.Context, projectID string) ([]Release, error) {
	s.mu.RLock()
	local, err := s.readReleases(projectID)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	rows := []Release{}
	for _, row := range s.observeLocalReleases(ctx, local) {
		rows = append(rows, s.browserRelease(row))
	}
	cloud, err := s.cloudReleases(projectID)
	if err != nil {
		return nil, err
	}
	rows = append(rows, cloud...)
	pages, err := s.pagesReleases(projectID)
	if err != nil {
		return nil, err
	}
	rows = append(rows, pages...)
	if s.nativePrototypeReleases != nil {
		native, err := s.nativePrototypeReleases(ctx, projectID)
		if err != nil {
			return nil, err
		}
		rows = append(rows, native...)
	}
	result := []Release{}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Status != "ready" {
			continue
		}
		validCommit := deliverySHA.MatchString(row.Commit)
		if row.Target != "local" && row.Target != "application" {
			validCommit = coolifyCommit.MatchString(row.Commit)
		}
		if row.ProjectID != projectID || !prototypeReleaseID.MatchString(row.ID) || seen[row.ID] || !validCommit || !validPrototypeURL(row, s.browserRegistry) {
			return nil, errors.New("prototype release provenance is invalid or ambiguous")
		}
		seen[row.ID] = true
		result = append(result, row)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result, nil
}

func (s *Server) listPrototypeReleases(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	rows, err := s.prototypeReleases(r.Context(), p.ID)
	if err != nil {
		problem(w, 500, "prototypes_unavailable", "Prototype history could not be verified. Check the project's deployments and retry.")
		return
	}
	writeJSON(w, 200, map[string]any{"releases": rows})
}
