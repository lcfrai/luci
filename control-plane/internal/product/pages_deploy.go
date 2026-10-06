package product

// Pages deliberately publishes committed static files only. GitHub CLI owns
// authentication; no token is read, persisted in a release, or sent to the site.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type PagesReleaseRequest struct {
	Commit            string `json:"commit"`
	Branch            string `json:"branch"`
	AcknowledgePublic bool   `json:"acknowledgePublic"`
}

type pagesRelease struct {
	Release
	Repository        string `json:"repository"`
	Branch            string `json:"branch"`
	IndexSHA256       string `json:"indexSha256"`
	SiteURL           string `json:"siteUrl"`
	BuildURL          string `json:"buildUrl,omitempty"`
	VerifiedAt        string `json:"verifiedAt,omitempty"`
	OwnsConfiguration bool   `json:"ownsConfiguration"`
}
type pagesSite struct {
	HTMLURL   string `json:"html_url"`
	CNAME     string `json:"cname"`
	BuildType string `json:"build_type"`
	Source    struct {
		Branch string `json:"branch"`
		Path   string `json:"path"`
	} `json:"source"`
}
type pagesBuild struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Commit string `json:"commit"`
}

func (s *Server) registerPagesReleases(mux Registrar) {
	mux.HandleFunc("POST /v1/product/projects/{id}/releases/pages", s.createPagesRelease)
}
func (s *Server) refreshRelease(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.PathValue("releaseId"), "pages-") {
		s.pagesReleaseOperation(w, r, false)
		return
	}
	s.refreshCloudRelease(w, r)
}
func (s *Server) pagesReleaseFile(projectID string) string {
	return filepath.Join(s.stateDir, "pages-releases", projectID+".json")
}
func (s *Server) readPagesReleases(projectID string) ([]pagesRelease, error) {
	f, err := os.Open(s.pagesReleaseFile(projectID))
	if errors.Is(err, os.ErrNotExist) {
		return []pagesRelease{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return nil, errors.New("Pages history could not be read")
	}
	var rows []pagesRelease
	if json.Unmarshal(data, &rows) != nil {
		return nil, errors.New("Pages history is invalid")
	}
	for _, row := range rows {
		if row.ProjectID != projectID || row.Target != "github-pages" || !validPagesRepository(row.Repository) {
			return nil, errors.New("Pages history has invalid provenance")
		}
	}
	return rows, nil
}
func (s *Server) storePagesRelease(row pagesRelease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readPagesReleases(row.ProjectID)
	if err != nil {
		return err
	}
	found := false
	for i := range rows {
		if rows[i].ID == row.ID {
			rows[i] = row
			found = true
			break
		}
	}
	if !found {
		rows = append([]pagesRelease{row}, rows...)
	}
	if len(rows) > 5000 {
		return errors.New("Pages history is full")
	}
	return atomicJSON(s.pagesReleaseFile(row.ProjectID), rows)
}
func (s *Server) pagesReleases(projectID string) ([]Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readPagesReleases(projectID)
	result := []Release{}
	for _, row := range rows {
		result = append(result, row.Release)
	}
	return result, err
}
func validPagesRepository(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && githubOwner.MatchString(parts[0]) && githubRepo.MatchString(parts[1]) && parts[1] != "." && parts[1] != ".." && !strings.HasSuffix(strings.ToLower(parts[1]), ".github.io")
}
func pagesAddress(repo string) string {
	parts := strings.Split(repo, "/")
	return "https://" + strings.ToLower(parts[0]) + ".github.io/" + parts[1] + "/"
}
func pagesSiteMatches(site pagesSite, row pagesRelease) bool {
	return site.CNAME == "" && (site.BuildType == "" || site.BuildType == "legacy") && site.Source.Branch == row.Branch && site.Source.Path == "/docs" && site.HTMLURL == pagesAddress(row.Repository)
}

// --include gives an explicit HTTP status, including 404. Command errors without
// an HTTP response cannot be mistaken for a missing site. Raw output stays here.
func (s *Server) pagesAPI(ctx context.Context, method, endpoint string, fields ...string) (int, []byte, error) {
	args := []string{"api", "--hostname", "github.com", "--method", method, "--include", "-H", "Accept: application/vnd.github+json", "-H", "X-GitHub-Api-Version: 2022-11-28", endpoint}
	for _, field := range fields {
		args = append(args, "-f", field)
	}
	out, runErr := s.runner(ctx, "gh", args, s.stateDir)
	if len(out) > 1<<20 {
		return 0, nil, errors.New("GitHub response exceeded its bound")
	}
	head, body, ok := strings.Cut(strings.ReplaceAll(out, "\r\n", "\n"), "\n\n")
	first := strings.Fields(strings.SplitN(head, "\n", 2)[0])
	if !ok || len(first) < 2 || !strings.HasPrefix(first[0], "HTTP/") {
		return 0, nil, errors.New("GitHub did not confirm the request; check the GitHub connection")
	}
	status, err := strconv.Atoi(first[1])
	if err != nil || status < 100 || status > 599 {
		return 0, nil, errors.New("GitHub returned an invalid response")
	}
	if runErr != nil && status >= 200 && status < 300 {
		return 0, nil, errors.New("GitHub response was interrupted")
	}
	return status, []byte(body), nil
}
func (s *Server) pagesSite(ctx context.Context, repo string) (pagesSite, int, error) {
	status, data, err := s.pagesAPI(ctx, "GET", "repos/"+repo+"/pages")
	var site pagesSite
	if err == nil && status == 200 && json.Unmarshal(data, &site) != nil {
		err = errors.New("GitHub Pages returned invalid site metadata")
	}
	return site, status, err
}
func pagesAllowed(w http.ResponseWriter, r *http.Request) bool {
	if _, policy, managed := management.ActorFromContext(r.Context()); managed && !policy.AllowPublicPublishing {
		problem(w, 403, "public_publishing_disabled", "Instance policy does not permit public publishing.")
		return false
	}
	return true
}
func (s *Server) createPagesRelease(w http.ResponseWriter, r *http.Request) {
	var req PagesReleaseRequest
	if !readJSON(w, r, &req) || !pagesAllowed(w, r) {
		return
	}
	if !req.AcknowledgePublic || !deliverySHA.MatchString(req.Commit) || !deliveryBranch.MatchString(req.Branch) {
		problem(w, 400, "invalid_pages_request", "Choose the complete committed revision and branch, and acknowledge public publication of /docs.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("pages:" + p.ID) {
		problem(w, 409, "release_busy", "Another Pages operation is in progress.")
		return
	}
	defer s.releaseDelivery("pages:" + p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	head, err := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	branch, branchErr := s.journeyGit(ctx, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	status, statusErr := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || branchErr != nil || statusErr != nil || strings.TrimSpace(head) != req.Commit || strings.TrimSpace(branch) != req.Branch || status != "" {
		problem(w, 409, "clean_commit_required", "Refresh and commit all changes on the selected branch before deploying.")
		return
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 409, "brief_unavailable", "The saved brief could not be read.")
		return
	}
	if err := s.validateReviewedCommit(ctx, p, state, req.Commit); err != nil {
		problem(w, 409, "review_approval_required", err.Error())
		return
	}
	tree, err := s.journeyGit(ctx, p.Path, "ls-tree", "-r", "-z", req.Commit, "--", "docs", "Dockerfile")
	files := map[string]bool{}
	for _, entry := range strings.Split(tree, "\x00") {
		if entry == "" {
			continue
		}
		mode, path, exists := strings.Cut(entry, "\t")
		if !exists || (!strings.HasPrefix(mode, "100644 blob ") && !strings.HasPrefix(mode, "100755 blob ")) {
			err = errors.New("links are not supported")
		}
		files[path] = true
		base := strings.ToLower(filepath.Base(path))
		if base == "cname" || base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
			err = errors.New("unsupported public file")
		}
	}
	if err != nil || !files["docs/index.html"] || !files["docs/.nojekyll"] || !files["Dockerfile"] {
		problem(w, 409, "static_site_required", "Commit docs/index.html, docs/.nojekyll and the Dockerfile. Pages accepts static output only, without links, CNAME or credential files.")
		return
	}
	index, err := s.journeyGit(ctx, p.Path, "show", req.Commit+":docs/index.html")
	if err != nil || len(index) == 0 || len(index) > 1<<20 {
		problem(w, 409, "static_site_required", "The committed index.html must be nonempty and at most 1 MiB.")
		return
	}
	origin, err := s.rawOrigin(ctx, p.Path)
	if err != nil {
		problem(w, 409, "github_origin_required", "Attach and push this project to a public GitHub repository first.")
		return
	}
	u, parseErr := url.Parse(origin)
	repo := ""
	if parseErr == nil && u.Host == "github.com" && u.Scheme == "https" {
		repo = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	}
	if !validPagesRepository(repo) {
		problem(w, 409, "github_origin_required", "Use a public GitHub project repository, not an account-level github.io site.")
		return
	}
	code, data, err := s.pagesAPI(ctx, "GET", "repos/"+repo)
	var repository struct {
		FullName      string `json:"full_name"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
	}
	if err != nil || code != 200 || json.Unmarshal(data, &repository) != nil || !strings.EqualFold(repository.FullName, repo) || repository.Private || repository.DefaultBranch != req.Branch {
		problem(w, 409, "public_repository_required", "GitHub must confirm this public repository and its default branch. Private repository Pages and non-default branches are outside this target.")
		return
	}
	code, data, err = s.pagesAPI(ctx, "GET", "repos/"+repo+"/git/ref/heads/"+url.PathEscape(req.Branch))
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err != nil || code != 200 || json.Unmarshal(data, &ref) != nil || ref.Object.SHA != req.Commit {
		problem(w, 409, "push_required", "Push this exact commit to GitHub before deploying.")
		return
	}
	digest := sha256.Sum256([]byte(index))
	row := pagesRelease{Release: Release{ID: newID("pages-"), ProjectID: p.ID, BriefRevision: state.Brief.Revision, Commit: req.Commit, Target: "github-pages", Status: "preparing", Message: "Preparing public static deployment.", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, Repository: repo, Branch: req.Branch, IndexSHA256: hex.EncodeToString(digest[:]), SiteURL: pagesAddress(repo)}
	s.mu.RLock()
	prior, err := s.readPagesReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 500, "release_history_unavailable", "Pages history could not be read.")
		return
	}
	for _, old := range prior {
		if old.Repository == repo && old.Branch == req.Branch && old.OwnsConfiguration && old.Status != "stopped" {
			row.OwnsConfiguration = true
			break
		}
	}
	site, code, err := s.pagesSite(ctx, repo)
	if err != nil || (code != 200 && code != 404) {
		problem(w, 502, "pages_unavailable", "GitHub did not confirm Pages settings. Check account access and repository policy.")
		return
	}
	if code == 200 && (!row.OwnsConfiguration || !pagesSiteMatches(site, row)) {
		problem(w, 409, "pages_configuration_exists", "This repository already has a Pages configuration that Luci does not own or that has changed. Nothing was replaced.")
		return
	}
	for _, old := range prior {
		if old.Repository == repo && old.Branch == req.Branch && old.Commit == req.Commit && old.Status != "stopped" {
			writeJSON(w, 202, old.Release)
			return
		}
	}
	if code == 404 {
		row.OwnsConfiguration = false
	}
	if err = s.RecordProjectRunReleaseProvenance(ctx, p.ID, row.ID, row.Target, row.Commit); err != nil {
		problem(w, 409, "release_provenance_unavailable", err.Error())
		return
	}
	if err = s.storePagesRelease(row); err != nil {
		problem(w, 500, "release_record_failed", "The deployment request could not be saved. Nothing was published.")
		return
	}
	fail := func(message string) {
		row.Status = "attention"
		row.Message = message
		_ = s.storePagesRelease(row)
		writeJSON(w, 202, row.Release)
	}
	if code == 404 {
		code, data, err = s.pagesAPI(ctx, "POST", "repos/"+repo+"/pages", "build_type=legacy", "source[branch]="+req.Branch, "source[path]=/docs")
		if err != nil || code != 201 || json.Unmarshal(data, &site) != nil || !pagesSiteMatches(site, row) {
			fail("GitHub did not confirm creation. Inspect the repository's Pages settings before retrying; no duplicate create is attempted.")
			return
		}
		row.OwnsConfiguration = true
		if s.storePagesRelease(row) != nil {
			problem(w, 500, "release_record_failed", "Pages was created but its ownership receipt could not be saved. Inspect GitHub before retrying.")
			return
		}
	}
	code, _, err = s.pagesAPI(ctx, "POST", "repos/"+repo+"/pages/builds")
	if err != nil || code != 201 {
		fail("Pages is configured but the build request was not confirmed. Check deployment status before retrying.")
		return
	}
	row.Status = "queued"
	row.Message = "GitHub Pages build requested. The site is not ready until the commit and public HTTPS content are verified."
	if s.storePagesRelease(row) != nil {
		problem(w, 500, "release_record_failed", "The build was requested but its receipt could not be saved.")
		return
	}
	writeJSON(w, 202, row.Release)
}

func (s *Server) pagesReleaseOperation(w http.ResponseWriter, r *http.Request, stop bool) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("pages:" + p.ID) {
		problem(w, 409, "release_busy", "Another Pages operation is in progress.")
		return
	}
	defer s.releaseDelivery("pages:" + p.ID)
	s.mu.RLock()
	rows, err := s.readPagesReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 500, "release_history_unavailable", "Pages history could not be read.")
		return
	}
	var row pagesRelease
	for _, candidate := range rows {
		if candidate.ID == r.PathValue("releaseId") {
			row = candidate
			break
		}
	}
	if row.ID == "" {
		problem(w, 404, "release_missing", "This project has no Pages release with that ID.")
		return
	}
	if row.Status == "stopped" || (!stop && (row.Status == "ready" || row.Status == "failed")) {
		writeJSON(w, 200, row.Release)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	site, code, err := s.pagesSite(ctx, row.Repository)
	if err != nil || (code != 200 && code != 404) {
		problem(w, 502, "pages_unavailable", "GitHub Pages status could not be checked.")
		return
	}
	if stop {
		if !row.OwnsConfiguration || (code == 200 && !pagesSiteMatches(site, row)) {
			problem(w, 409, "pages_ownership_changed", "Pages settings do not match Luci's creation receipt. Nothing was removed.")
			return
		}
		for _, newer := range rows {
			if newer.Repository == row.Repository && newer.CreatedAt > row.CreatedAt && newer.Status != "stopped" {
				problem(w, 409, "newer_release_exists", "Stop the latest Pages release for this repository.")
				return
			}
		}
		if code == 200 {
			code, _, err = s.pagesAPI(ctx, "DELETE", "repos/"+row.Repository+"/pages")
			if err != nil || code != 204 {
				problem(w, 502, "stop_unconfirmed", "GitHub did not confirm unpublishing. Refresh before retrying.")
				return
			}
		}
		_, code, err = s.pagesSite(ctx, row.Repository)
		if err != nil || code != 404 {
			problem(w, 502, "stop_unconfirmed", "Waiting for GitHub to confirm that Pages is disabled.")
			return
		}
		row.Status = "stopped"
		row.URL = ""
		row.Message = "GitHub confirmed that this Pages site is unpublished. The repository and release history are retained."
	} else {
		if code != 200 || !row.OwnsConfiguration || !pagesSiteMatches(site, row) {
			problem(w, 409, "pages_configuration_changed", "The recorded Pages configuration is absent or changed. Check GitHub before continuing.")
			return
		}
		code, data, e := s.pagesAPI(ctx, "GET", "repos/"+row.Repository+"/pages/builds/latest")
		var build pagesBuild
		if e != nil || code != 200 || json.Unmarshal(data, &build) != nil {
			problem(w, 502, "pages_build_unavailable", "GitHub has not supplied a build status yet. Check again shortly.")
			return
		}
		row = observePagesBuild(ctx, row, build, verifyPublicPagesIndex)
	}
	if s.storePagesRelease(row) != nil {
		problem(w, 500, "release_record_failed", "The observed Pages status could not be saved.")
		return
	}
	writeJSON(w, 200, row.Release)
}
func observePagesBuild(ctx context.Context, row pagesRelease, build pagesBuild, health func(context.Context, string, string) error) pagesRelease {
	row.URL = ""
	if build.Commit != row.Commit {
		row.Status = "attention"
		row.Message = "GitHub's latest Pages build does not match this release commit. No readiness is claimed."
		return row
	}
	expected := "https://api.github.com/repos/" + row.Repository + "/pages/builds/"
	if !strings.HasPrefix(build.URL, expected) || strings.TrimPrefix(build.URL, expected) == "" || strings.ContainsAny(strings.TrimPrefix(build.URL, expected), "/?#") {
		row.Status = "attention"
		row.Message = "GitHub did not return a valid build identifier."
		return row
	}
	row.BuildURL = build.URL
	switch build.Status {
	case "errored":
		row.Status = "failed"
		row.Message = "GitHub Pages reported a build failure. Inspect the repository's Pages build."
	case "queued", "building":
		row.Status = build.Status
		row.Message = "GitHub Pages is building the recorded commit."
	case "built":
		row.Status = "verifying"
		row.Message = "GitHub built this commit; waiting for public HTTPS to serve the committed index.html."
		if health(ctx, row.SiteURL, row.IndexSHA256) == nil {
			row.Status = "ready"
			row.URL = row.SiteURL
			row.VerifiedAt = time.Now().UTC().Format(time.RFC3339Nano)
			row.Message = "GitHub confirmed the exact commit and public HTTPS served its committed static site."
		}
	default:
		row.Status = "attention"
		row.Message = "GitHub returned an unrecognised Pages build status."
	}
	return row
}
func verifyPublicPagesIndex(ctx context.Context, address, digest string) error {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || !strings.HasSuffix(u.Hostname(), ".github.io") || !publicReleaseHost(u.Hostname()) {
		return errors.New("invalid public Pages address")
	}
	repository := strings.TrimSuffix(u.Hostname(), ".github.io") + "/" + strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	if !validPagesRepository(repository) || address != pagesAddress(repository) {
		return errors.New("invalid public Pages site path")
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("Pages DNS is unavailable")
		}
		for _, ip := range ips {
			if !publicReleaseIP(ip.IP) {
				return nil, errors.New("Pages DNS is not public")
			}
		}
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	defer transport.CloseIdleConnections()
	return probePagesIndex(ctx, address, digest, &http.Client{Timeout: 8 * time.Second, Transport: transport})
}
func probePagesIndex(ctx context.Context, address, digest string, client *http.Client) error {
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cache-Control", "no-cache")
	response, err := copyClient.Do(req)
	if err != nil {
		return errors.New("Pages HTTPS request did not succeed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("Pages returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return errors.New("Pages response exceeded its bound")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return errors.New("Pages is not serving this committed index yet")
	}
	return nil
}
