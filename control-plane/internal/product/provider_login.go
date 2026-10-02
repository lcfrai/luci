package product

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProviderLoginJob contains only user-facing sign-in instructions. Provider
// credentials remain in the provider CLI's own credential cache.
type ProviderLoginJob struct {
	ID              string `json:"id"`
	Provider        string `json:"provider"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	VerificationURL string `json:"verificationUrl,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
	ActionURL       string `json:"actionUrl,omitempty"`
	CreatedAt       string `json:"createdAt"`
	cancel          context.CancelFunc
}

type ProviderLoginCommand struct {
	Name      string
	Args, Env []string
}
type ProviderLoginProcess interface{ Run() error }
type ProviderLoginProcessFactory func(context.Context, ProviderLoginCommand, io.Writer) (ProviderLoginProcess, error)

func providerLoginCommand(provider string) (ProviderLoginCommand, error) {
	switch provider {
	case "aws":
		return ProviderLoginCommand{Name: "aws", Args: []string{"login", "--no-cli-pager"}, Env: []string{"AWS_PAGER=", "AWS_CLI_AUTO_PROMPT=off"}}, nil
	case "azure":
		return ProviderLoginCommand{Name: "az", Args: []string{"login", "--use-device-code", "--output", "none"}, Env: []string{"AZURE_CORE_LOGIN_EXPERIENCE_V2=off", "AZURE_CORE_ENABLE_BROKER_ON_WINDOWS=false", "PYTHONUNBUFFERED=1"}}, nil
	default:
		return ProviderLoginCommand{}, errors.New("choose AWS or Azure")
	}
}

func nativeProviderLoginProcess(ctx context.Context, command ProviderLoginCommand, output io.Writer) (ProviderLoginProcess, error) {
	path, err := exec.LookPath(command.Name)
	if err != nil {
		return nil, err
	}
	args := append([]string{}, command.Args...)
	environment := append(os.Environ(), command.Env...)
	// The official Azure MSI launcher simply invokes ../python.exe -IBm
	// azure.cli. Invoke that interpreter directly so cancellation owns the
	// actual CLI process and no command shell parses arguments.
	if runtime.GOOS == "windows" && command.Name == "az" && (strings.EqualFold(filepath.Ext(path), ".cmd") || strings.EqualFold(filepath.Ext(path), ".bat")) {
		python := filepath.Clean(filepath.Join(filepath.Dir(path), "..", "python.exe"))
		info, statErr := os.Stat(python)
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, errors.New("Azure CLI's bundled interpreter is unavailable")
		}
		path = python
		args = append([]string{"-IBm", "azure.cli"}, args...)
		environment = append(environment, "AZ_INSTALLER=MSI")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	hideProcess(cmd)
	cmd.Env = environment
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = 2 * time.Second
	return cmd, nil
}

func (s *Server) registerProviderLogin(mux Registrar) {
	mux.HandleFunc("POST /v1/setup/cloud/login", s.handleProviderLogin)
	mux.HandleFunc("GET /v1/setup/cloud/login", s.listProviderLogins)
}

func (s *Server) handleProviderLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	job, err := s.startProviderLogin(req.Provider, nativeProviderLoginProcess)
	if err != nil {
		problem(w, 400, "invalid_provider", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) startProviderLogin(provider string, factory ProviderLoginProcessFactory) (ProviderLoginJob, error) {
	command, err := providerLoginCommand(provider)
	if err != nil {
		return ProviderLoginJob{}, err
	}
	s.mu.Lock()
	if s.authJobs == nil {
		s.authJobs = map[string]*ProviderLoginJob{}
	}
	for _, job := range s.authJobs {
		if job.Provider == provider && (job.Status == "running" || job.Status == "waiting") {
			copy := *job
			s.mu.Unlock()
			return copy, nil
		}
	}
	// Only the current/recent attempt for each provider is needed. Removing a
	// completed instruction also avoids retaining an expired device code.
	for id, job := range s.authJobs {
		if job.Provider == provider {
			delete(s.authJobs, id)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	job := &ProviderLoginJob{ID: newID("login-"), Provider: provider, Status: "running", Message: "Starting provider sign-in.", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), cancel: cancel}
	s.authJobs[job.ID] = job
	s.mu.Unlock()
	writer := &providerLoginWriter{provider: provider, update: func(address, code string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if job.Status != "running" && job.Status != "waiting" {
			return
		}
		job.VerificationURL, job.UserCode = address, code
		job.Status = "waiting"
		if provider == "aws" {
			job.Message = "Finish signing in through the AWS browser window. If it did not open, use the sign-in link."
		} else {
			job.Message = "Open the Microsoft sign-in link and enter this device code."
		}
	}}
	process, err := factory(ctx, command, writer)
	if err != nil {
		cancel()
		s.mu.Lock()
		job.Status = "failed"
		job.Message = "The provider CLI could not start. Install or repair it, then try signing in again."
		if provider == "aws" {
			job.ActionURL = "https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
		} else {
			job.ActionURL = "https://learn.microsoft.com/en-us/cli/azure/install-azure-cli"
		}
		copy := *job
		s.mu.Unlock()
		return copy, nil
	}
	go func() {
		err := process.Run()
		s.mu.Lock()
		defer s.mu.Unlock()
		defer cancel()
		job.VerificationURL, job.UserCode = "", ""
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			job.Status = "expired"
			job.Message = "Sign-in expired after ten minutes. Start a new sign-in when ready."
		case errors.Is(ctx.Err(), context.Canceled):
			job.Status = "cancelled"
			job.Message = "Sign-in was cancelled."
		case err != nil:
			job.Status = "failed"
			job.Message = "Sign-in did not finish. Check the provider's browser instructions and try again."
		default:
			job.Status = "succeeded"
			job.Message = "The provider CLI completed sign-in. Check your cloud connection to confirm the selected account."
		}
	}()
	s.mu.RLock()
	copy := *job
	s.mu.RUnlock()
	return copy, nil
}

func (s *Server) providerLoginSnapshots() []ProviderLoginJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := []ProviderLoginJob{}
	for _, job := range s.authJobs {
		jobs = append(jobs, *job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	return jobs
}

func (s *Server) listProviderLogins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"jobs": s.providerLoginSnapshots()})
}

// closeProviderLogins must be called without holding s.mu.
func (s *Server) closeProviderLogins() {
	s.mu.RLock()
	cancels := []context.CancelFunc{}
	for _, job := range s.authJobs {
		if job.cancel != nil {
			cancels = append(cancels, job.cancel)
		}
	}
	s.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

var loginURLPattern = regexp.MustCompile(`https://[^\s<>"']+`)
var loginANSIPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
var loginCodePattern = regexp.MustCompile(`(?i)\b(?:enter|use)\s+(?:the\s+)?code\s+([A-Z0-9]{6,12}(?:-[A-Z0-9]{3,6})?)\b`)

type providerLoginWriter struct {
	mu       sync.Mutex
	provider string
	buffer   string
	update   func(string, string)
}

func (w *providerLoginWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Output can include account details or tokens on an unexpected CLI error.
	// Keep only a small transient parsing buffer; never persist or expose it.
	w.buffer += string(p)
	if len(w.buffer) > 16<<10 {
		w.buffer = w.buffer[len(w.buffer)-(16<<10):]
	}
	text := loginANSIPattern.ReplaceAllString(w.buffer, "")
	for _, match := range loginURLPattern.FindAllStringIndex(text, -1) {
		candidate := strings.TrimRight(text[match[0]:match[1]], ".,);")
		if !allowedProviderLoginURL(w.provider, candidate) {
			continue
		}
		code := ""
		if w.provider == "azure" {
			// The Azure device-code instruction follows its verification URL.
			// Do not pair it with unrelated earlier output containing "code".
			instruction := text[match[1]:]
			if len(instruction) > 512 {
				instruction = instruction[:512]
			}
			if codeMatch := loginCodePattern.FindStringSubmatch(instruction); len(codeMatch) == 2 {
				code = strings.ToUpper(codeMatch[1])
			}
		}
		w.update(candidate, code)
		break
	}
	return len(p), nil
}

func allowedProviderLoginURL(provider, address string) bool {
	if len(address) > 8192 {
		return false
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if provider == "azure" {
		if u.RawQuery != "" || u.RawPath != "" {
			return false
		}
		return ((host == "microsoft.com" || host == "www.microsoft.com" || host == "aka.ms") && u.Path == "/devicelogin") || (host == "login.microsoftonline.com" && u.Path == "/common/oauth2/deviceauth")
	}
	if provider != "aws" || (host != "signin.aws.amazon.com" && !strings.HasSuffix(host, ".signin.aws.amazon.com")) {
		return false
	}
	for key := range u.Query() {
		switch strings.ToLower(key) {
		case "access_token", "refresh_token", "id_token", "token", "password", "secret", "client_secret", "code":
			return false
		}
	}
	return true
}
