package product

// GitHub CLI remains responsible for OAuth and credential storage. Luci keeps
// only publication provenance, and never returns raw Git/gh command output.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type GitHubAccount struct {
	Available bool   `json:"available"`
	SignedIn  bool   `json:"signedIn"`
	Login     string `json:"login,omitempty"`
	Message   string `json:"message"`
	ActionURL string `json:"actionUrl,omitempty"`
}
type GitPublication struct {
	Repository string `json:"repository"`
	Visibility string `json:"visibility,omitempty"`
	Status     string `json:"status"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Verified   bool   `json:"verified"`
	UpdatedAt  string `json:"updatedAt"`
}
type GitDelivery struct {
	Origin      string          `json:"origin,omitempty"`
	Branch      string          `json:"branch"`
	Head        string          `json:"head"`
	Publication *GitPublication `json:"publication,omitempty"`
	Message     string          `json:"message,omitempty"`
}

var githubOwner = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var githubRepo = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,99}$`)
var deliverySHA = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
var deliveryBranch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

func (s *Server) registerDelivery(mux Registrar) {
	mux.HandleFunc("POST /v1/setup/github/check", s.checkGitHub)
	mux.HandleFunc("POST /v1/setup/github/login", s.loginGitHub)
	mux.HandleFunc("GET /v1/setup/github/login", s.listGitHubLogins)
	mux.HandleFunc("GET /v1/product/projects/{id}/git/delivery", s.getGitDelivery)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/remote", s.attachGitRemote)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/github", s.createGitHubRepository)
	mux.HandleFunc("POST /v1/product/projects/{id}/git/push", s.pushGitDelivery)
}

func (s *Server) checkGitHub(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	result := GitHubAccount{Message: "Install GitHub CLI to connect your account.", ActionURL: "https://cli.github.com/"}
	if _, err := s.runner(ctx, "gh", []string{"--version"}, s.stateDir); err == nil {
		result.Available = true
		result.Message = "Sign in to GitHub on this workspace."
		result.ActionURL = ""
		login, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "user", "--jq", ".login"}, s.stateDir)
		if err == nil && githubOwner.MatchString(strings.TrimSpace(login)) {
			result.Login, result.SignedIn, result.Message = strings.TrimSpace(login), true, "GitHub account connected on this workspace."
		}
	}
	writeJSON(w, 200, result)
}

func (s *Server) loginGitHub(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !readJSON(w, r, &req) {
		return
	}
	writeJSON(w, 202, s.startGitHubLogin(nativeProviderLoginProcess))
}
func (s *Server) listGitHubLogins(w http.ResponseWriter, r *http.Request) {
	jobs := []ProviderLoginJob{}
	for _, job := range s.providerLoginSnapshots() {
		if job.Provider == "github" {
			jobs = append(jobs, job)
		}
	}
	writeJSON(w, 200, map[string]any{"jobs": jobs})
}
func (s *Server) startGitHubLogin(factory ProviderLoginProcessFactory) ProviderLoginJob {
	s.mu.Lock()
	if s.authJobs == nil {
		s.authJobs = map[string]*ProviderLoginJob{}
	}
	for _, job := range s.authJobs {
		if job.Provider == "github" && (job.Status == "running" || job.Status == "waiting") {
			copy := *job
			s.mu.Unlock()
			return copy
		}
	}
	for id, job := range s.authJobs {
		if job.Provider == "github" {
			delete(s.authJobs, id)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	job := &ProviderLoginJob{ID: newID("login-"), Provider: "github", Status: "running", Message: "Starting GitHub sign-in.", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), cancel: cancel}
	s.authJobs[job.ID] = job
	s.mu.Unlock()
	writer := &githubLoginWriter{update: func(code string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if job.Status == "running" || job.Status == "waiting" {
			job.Status = "waiting"
			job.VerificationURL = "https://github.com/login/device"
			job.UserCode = code
			job.Message = "Open GitHub and enter this one-time code."
		}
	}}
	command := ProviderLoginCommand{Name: "gh", Args: []string{"auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web", "--skip-ssh-key"}, Env: []string{"GH_PROMPT_DISABLED=1", "GH_BROWSER="}}
	process, err := factory(ctx, command, writer)
	if err != nil {
		cancel()
		s.mu.Lock()
		job.Status = "failed"
		job.Message = "GitHub CLI could not start. Install or repair it, then try again."
		job.ActionURL = "https://cli.github.com/"
		copy := *job
		s.mu.Unlock()
		return copy
	}
	go func() {
		err := process.Run()
		s.mu.Lock()
		defer s.mu.Unlock()
		defer cancel()
		job.UserCode, job.VerificationURL = "", ""
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			job.Status = "expired"
			job.Message = "GitHub sign-in expired. Try again when ready."
		case ctx.Err() != nil:
			job.Status = "cancelled"
			job.Message = "GitHub sign-in was cancelled."
		case err != nil:
			job.Status = "failed"
			job.Message = "GitHub sign-in did not finish. Retry using the browser instructions."
		default:
			job.Status = "succeeded"
			job.Message = "GitHub CLI completed sign-in. Check the connection to confirm your account."
		}
	}()
	s.mu.RLock()
	copy := *job
	s.mu.RUnlock()
	return copy
}

var githubDeviceCode = regexp.MustCompile(`(?i)one-time code:\s*([A-Z0-9]{4}-[A-Z0-9]{4})\b`)

type githubLoginWriter struct {
	mu     sync.Mutex
	buffer string
	update func(string)
}

func (w *githubLoginWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer += string(p)
	if len(w.buffer) > 16384 {
		w.buffer = w.buffer[len(w.buffer)-16384:]
	}
	clean := loginANSIPattern.ReplaceAllString(w.buffer, "")
	if match := githubDeviceCode.FindStringSubmatch(clean); len(match) == 2 {
		w.update(strings.ToUpper(match[1]))
	}
	return len(p), nil
}

func safeDeliveryURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Port() != "" || strings.ContainsAny(raw, "\r\n\x00\\ \t") || !regexp.MustCompile(`^[A-Za-z0-9.-]+$`).MatchString(u.Host) {
		return "", errors.New("Use an HTTPS Git URL without credentials, a port, query, fragment or escaped path.")
	}
	if u.Path == "" || u.Path == "/" {
		return "", errors.New("Include the repository path in the HTTPS URL.")
	}
	for _, part := range strings.Split(strings.TrimPrefix(u.Path, "/"), "/") {
		if part == "" || part == "." || part == ".." || !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(part) {
			return "", errors.New("The repository URL contains an unsupported path.")
		}
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}
func (s *Server) reserveDelivery(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deliveryBusy == nil {
		s.deliveryBusy = map[string]bool{}
	}
	if s.deliveryBusy[id] {
		return false
	}
	s.deliveryBusy[id] = true
	return true
}
func (s *Server) releaseDelivery(id string) { s.mu.Lock(); delete(s.deliveryBusy, id); s.mu.Unlock() }
func (s *Server) publicationPath(id string) string {
	return filepath.Join(s.stateDir, "git-delivery", id+".json")
}
func (s *Server) publication(id string) (*GitPublication, error) {
	f, err := os.Open(s.publicationPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var record GitPublication
	if err = json.NewDecoder(io.LimitReader(f, 16384)).Decode(&record); err != nil {
		return nil, err
	}
	if _, err = safeDeliveryURL(record.Repository); err != nil {
		return nil, err
	}
	return &record, nil
}
func (s *Server) savePublication(id string, record *GitPublication) error {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return atomicJSON(s.publicationPath(id), record)
}
func (s *Server) rawOrigin(ctx context.Context, dir string) (string, error) {
	raw, err := s.journeyGit(ctx, dir, "config", "--get-all", "remote.origin.url")
	if err != nil && raw == "" {
		return "", nil
	}
	if err != nil || strings.Contains(strings.TrimSpace(raw), "\n") {
		return "", errors.New("Origin must have exactly one HTTPS URL.")
	}
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return safeDeliveryURL(strings.TrimSpace(raw))
}
func (s *Server) getGitDelivery(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result := GitDelivery{}
	result.Origin, _ = s.rawOrigin(ctx, p.Path)
	if result.Origin == "" {
		result.Message = "Attach a safe HTTPS origin or create a GitHub repository."
	}
	branch, _ := s.journeyGit(ctx, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	result.Branch = strings.TrimSpace(branch)
	head, _ := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if deliverySHA.MatchString(strings.TrimSpace(head)) {
		result.Head = strings.TrimSpace(head)
	}
	result.Publication, _ = s.publication(p.ID)
	writeJSON(w, 200, result)
}
func (s *Server) attachGitRemote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	address, err := safeDeliveryURL(req.URL)
	if err != nil {
		problem(w, 400, "invalid_remote", err.Error())
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery(p.ID) {
		problem(w, 409, "delivery_busy", "Another delivery action is running for this project.")
		return
	}
	defer s.releaseDelivery(p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	origin, err := s.rawOrigin(ctx, p.Path)
	if err != nil || (origin != "" && origin != address) {
		problem(w, 409, "origin_exists", "The project already has an origin. Review and change it in Git before attaching a different repository.")
		return
	}
	if origin == "" {
		if _, err = s.journeyGit(ctx, p.Path, "remote", "add", "origin", address); err != nil {
			problem(w, 422, "attach_failed", "Git could not attach this origin.")
			return
		}
	}
	record := GitPublication{Repository: address, Status: "attached"}
	if err = s.savePublication(p.ID, &record); err != nil {
		problem(w, 500, "history_failed", "Origin is attached, but delivery history could not be saved.")
		return
	}
	writeJSON(w, 200, record)
}
func (s *Server) createGitHubRepository(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Owner             string `json:"owner"`
		Name              string `json:"name"`
		Visibility        string `json:"visibility"`
		AcknowledgePublic bool   `json:"acknowledgePublic"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if !githubRepo.MatchString(req.Name) || req.Name == "." || req.Name == ".." || (req.Owner != "" && !githubOwner.MatchString(req.Owner)) || (req.Visibility != "private" && req.Visibility != "public") {
		problem(w, 400, "invalid_repository", "Choose a GitHub repository name, optional owner, and private or public visibility.")
		return
	}
	if req.Visibility == "public" && !req.AcknowledgePublic {
		problem(w, 400, "public_acknowledgement_required", "Confirm that creating this repository publicly is intended.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery(p.ID) {
		problem(w, 409, "delivery_busy", "Another delivery action is running for this project.")
		return
	}
	defer s.releaseDelivery(p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	origin, err := s.rawOrigin(ctx, p.Path)
	if err != nil || origin != "" {
		problem(w, 409, "origin_exists", "This project already has an origin. Use Push to deliver to it.")
		return
	}
	previous, err := s.publication(p.ID)
	if err != nil || (previous != nil && previous.Status == "creation_unconfirmed") {
		problem(w, 409, "creation_unconfirmed", "A previous creation may have reached GitHub. Check GitHub and attach that repository's HTTPS URL before retrying.")
		return
	}
	if req.Owner == "" {
		login, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "user", "--jq", ".login"}, s.stateDir)
		if err != nil || !githubOwner.MatchString(strings.TrimSpace(login)) {
			problem(w, 409, "github_signin_required", "Connect your GitHub account on this workspace first.")
			return
		}
		req.Owner = strings.TrimSpace(login)
	}
	name := req.Owner + "/" + req.Name
	record := GitPublication{Repository: "https://github.com/" + name + ".git", Visibility: req.Visibility, Status: "creation_unconfirmed"}
	if err = s.savePublication(p.ID, &record); err != nil {
		problem(w, 500, "history_failed", "Could not save the creation request. No repository was created.")
		return
	}
	if _, err = s.runner(ctx, "gh", []string{"repo", "create", name, "--" + req.Visibility}, s.stateDir); err != nil {
		problem(w, 422, "creation_unconfirmed", "GitHub did not confirm repository creation. Check the repository in GitHub; attach its HTTPS URL if it exists. Credentials or organisation permissions may need attention.")
		return
	}
	output, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "repos/" + name}, s.stateDir)
	var verified struct {
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
		CloneURL string `json:"clone_url"`
	}
	if err != nil || json.Unmarshal([]byte(output), &verified) != nil || !strings.EqualFold(verified.FullName, name) || verified.Private != (req.Visibility == "private") || verified.CloneURL != record.Repository {
		problem(w, 422, "creation_unconfirmed", "GitHub creation completed but repository visibility and URL could not be verified. Check GitHub before attaching it.")
		return
	}
	if _, err = s.journeyGit(ctx, p.Path, "remote", "add", "origin", record.Repository); err != nil {
		problem(w, 422, "attach_failed", "GitHub repository exists, but Git could not attach it. Attach its HTTPS URL manually.")
		return
	}
	record.Status = "created"
	if err = s.savePublication(p.ID, &record); err != nil {
		problem(w, 500, "history_failed", "Repository exists and origin is attached, but delivery history could not be saved.")
		return
	}
	writeJSON(w, 201, record)
}

// Read helpers from trusted machine-level configuration, never repository-local
// shell helpers. A fixed gh helper enables the account connected by this flow.
func (s *Server) deliveryNetworkGit(ctx context.Context, dir, address string, args ...string) (string, error) {
	rewrites, err := s.journeyGit(ctx, dir, "config", "--get-regexp", `^url\..*\.(insteadof|pushinsteadof)$`)
	if err == nil && strings.TrimSpace(rewrites) != "" {
		return "", errors.New("URL rewrites must be removed before guided delivery")
	}
	flags := []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "http.followRedirects=false", "-c", "credential.helper="}
	for _, scope := range []string{"--system", "--global"} {
		helpers, err := s.journeyGit(ctx, dir, "config", scope, "--get-urlmatch", "credential.helper", address)
		if err == nil {
			for _, helper := range strings.Split(strings.TrimSpace(helpers), "\n") {
				if helper != "" {
					flags = append(flags, "-c", "credential.helper="+strings.TrimSpace(helper))
				}
			}
		}
	}
	u, _ := url.Parse(address)
	if u.Host == "github.com" {
		flags = append(flags, "-c", "credential.helper=!gh auth git-credential")
	}
	return s.journeyGit(ctx, dir, append(flags, args...)...)
}
func (s *Server) pushGitDelivery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Commit string `json:"commit"`
		Branch string `json:"branch"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !deliverySHA.MatchString(req.Commit) {
		problem(w, 400, "invalid_commit", "Choose the complete commit ID shown in this project.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery(p.ID) {
		problem(w, 409, "delivery_busy", "Another delivery action is running for this project.")
		return
	}
	defer s.releaseDelivery(p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	address, err := s.rawOrigin(ctx, p.Path)
	if err != nil || address == "" {
		problem(w, 409, "origin_required", "Attach a safe HTTPS origin before pushing.")
		return
	}
	if _, policy, managed := management.ActorFromContext(r.Context()); managed && !policy.AllowPublicPublishing {
		if !s.privateGitHubDestination(ctx, address) {
			problem(w, 403, "private_repository_required", "Instance policy permits only a verified private GitHub destination. Connect GitHub and use a private repository, or ask a Super Admin to allow public publishing.")
			return
		}
	}
	branch, _ := s.journeyGit(ctx, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	branch = strings.TrimSpace(branch)
	if req.Branch == "" {
		req.Branch = branch
	}
	if req.Branch != branch || !deliveryBranch.MatchString(req.Branch) {
		problem(w, 409, "branch_changed", "Choose the project's current named branch before pushing.")
		return
	}
	if _, err = s.journeyGit(ctx, p.Path, "check-ref-format", "refs/heads/"+req.Branch); err != nil {
		problem(w, 400, "invalid_branch", "The branch name is invalid.")
		return
	}
	head, err := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(head) != req.Commit {
		problem(w, 409, "commit_changed", "The project commit changed. Refresh Git and review it before pushing.")
		return
	}
	status, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || status != "" {
		problem(w, 409, "clean_commit_required", "Commit all project changes before pushing.")
		return
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 409, "brief_unavailable", "The saved brief could not be read.")
		return
	}
	if state.Brief.Revision > 0 {
		committed, err := s.journeyGit(ctx, p.Path, "show", req.Commit+":LUCI.md")
		if err != nil || committed != string(renderBrief(state.Brief)) {
			problem(w, 409, "brief_not_committed", "Commit the current brief before pushing.")
			return
		}
	}
	// Push the reviewed immutable commit and explicit HTTPS URL. Ignore local
	// pushurl, default refspecs and configured force/mirror behaviour.
	if _, err = s.deliveryNetworkGit(ctx, p.Path, address, "push", "--porcelain", "--no-force", "--no-mirror", "--no-follow-tags", "--recurse-submodules=no", address, req.Commit+":refs/heads/"+req.Branch); err != nil {
		problem(w, 422, "push_failed", "Git did not confirm this push. Check repository access and branch rules, then refresh before retrying. No force push was requested.")
		return
	}
	output, err := s.deliveryNetworkGit(ctx, p.Path, address, "ls-remote", "--heads", address, "refs/heads/"+req.Branch)
	want := req.Commit + "\trefs/heads/" + req.Branch
	if err != nil || strings.TrimSpace(output) != want {
		problem(w, 422, "remote_unverified", "Push returned, but the remote branch's exact commit could not be verified. Refresh before retrying.")
		return
	}
	record := GitPublication{Repository: address, Status: "pushed", Branch: req.Branch, Commit: req.Commit, Verified: true}
	if prior, _ := s.publication(p.ID); prior != nil && prior.Repository == address {
		record.Visibility = prior.Visibility
	}
	if err = s.savePublication(p.ID, &record); err != nil {
		problem(w, 500, "history_failed", "The exact commit is on the remote, but delivery history could not be saved.")
		return
	}
	writeJSON(w, 200, record)
}

func (s *Server) privateGitHubDestination(ctx context.Context, address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Host != "github.com" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) != 2 {
		return false
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	if !githubOwner.MatchString(parts[0]) || !githubRepo.MatchString(parts[1]) {
		return false
	}
	output, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "repos/" + strings.Join(parts, "/")}, s.stateDir)
	var metadata struct {
		Private  bool   `json:"private"`
		CloneURL string `json:"clone_url"`
	}
	return err == nil && json.Unmarshal([]byte(output), &metadata) == nil && metadata.Private && strings.TrimSuffix(metadata.CloneURL, ".git") == strings.TrimSuffix(address, ".git")
}
