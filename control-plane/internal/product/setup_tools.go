package product

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type SSHKey struct {
	PublicKey      string `json:"publicKey"`
	PrivateKeyPath string `json:"privateKeyPath"`
	Fingerprint    string `json:"fingerprint"`
}

type RuntimeJob struct {
	OperationKind string `json:"operationKind,omitempty"`
	InputProof *runtimeinstall.DebianInputProofOutcome `json:"inputProof,omitempty"`
	ID              string                  `json:"id"`
	Status          string                  `json:"status"`
	Image           string                  `json:"image"`
	Message         string                  `json:"message"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
	Execution       *runtimeinstall.Outcome `json:"execution,omitempty"`
	CancelRequested bool                    `json:"cancelRequested,omitempty"`
}

// A first build downloads two base images, operating-system packages and the
// pinned agent CLIs. It has one bounded budget, with no automatic retries.
const runtimeBuildTimeout = 30 * time.Minute

// Only trusted runtime preparation code can grant the longer Docker build
// budget. Request bodies, headers and other commands cannot select it.
type runtimeBuildBudgetKey struct{}
type runtimeBuildBudget struct{}

func commandExecutionBudget(ctx context.Context, executable string, args []string) time.Duration {
	if executable == "docker" && len(args) > 0 && args[0] == "build" {
		if _, trusted := ctx.Value(runtimeBuildBudgetKey{}).(runtimeBuildBudget); trusted {
			return runtimeBuildTimeout
		}
	}
	if _, bounded := ctx.Deadline(); bounded {
		return 10 * time.Minute
	}
	return 2 * time.Minute
}

func runtimeBuildFailure(ctx context.Context, err error) string {
	prefix := "Runtime installation failed: "
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		prefix = "Runtime installation timed out after 30 minutes. Check the connection and Docker build error before trying again: "
	}
	// Retain both the initial operation and the final Docker failure. Keeping
	// only the beginning hides the cause behind ordinary cold-download output.
	detail := strings.ToValidUTF8(err.Error(), "?")
	const limit = 8000
	if len(prefix)+len(detail) <= limit {
		return prefix + detail
	}
	const marker = "\n[earlier build output omitted]\n"
	const headBytes = 1024
	tailBytes := limit - len(prefix) - headBytes - len(marker)
	head := strings.ToValidUTF8(detail[:headBytes], "")
	tail := strings.ToValidUTF8(detail[len(detail)-tailBytes:], "")
	return prefix + head + marker + tail
}

// These short setup operations must not hold the server's state lock while a
// child process runs. Serialization also prevents two first-run key requests
// from racing to publish different public/private pairs.
var identityWriteMu sync.Mutex
var sshKeyWriteMu sync.Mutex

func stateOutsideProjects(projectsDir, stateDir string) error {
	if _, err := hostpath.ResolveOutside(projectsDir, stateDir); err != nil {
		return errors.New("choose a projects folder that does not contain Luci's private state folder")
	}
	return nil
}

func (s *Server) initSetupTools() error {
	if err := os.MkdirAll(s.stateDir, 0700); err != nil {
		return err
	}
	if err := s.initAgentCredentials(); err != nil {
		return err
	}
	s.runtimeJob = RuntimeJob{Status: "idle", Image: s.sandboxImage, Message: "Install the bundled agent runtime when Docker is ready."}
	file, err := os.Open(filepath.Join(s.stateDir, "runtime-install.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32<<10+1))
	if err != nil {
		return err
	}
	if len(data) > 32<<10 {
		return errors.New("runtime install state exceeds its limit")
	}
	if err = decodeRuntimeJobState(data, &s.runtimeJob); err != nil {
		return err
	}
	if runtimeInstallActive(s.runtimeJob.Status) {
		s.runtimeJob.Status = "unknown"
		s.runtimeJob.Message = "Luci restarted during installation. Check the retained operation before preparing again."
	}
	return nil
}

func (s *Server) registerSetupTools(mux Registrar) {
	s.registerAgentCredentials(mux)
	mux.HandleFunc("GET /v1/setup/git/identity", s.getGitIdentity)
	mux.HandleFunc("POST /v1/setup/git/identity", s.setGitIdentity)
	mux.HandleFunc("POST /v1/setup/ssh/key", s.createSSHKey)
	mux.HandleFunc("GET /v1/setup/runtime/install", s.getRuntimeInstall)
	mux.HandleFunc("POST /v1/setup/runtime/install", s.installRuntime)
	mux.HandleFunc("POST /v1/setup/runtime/install/cancel", s.cancelRuntimeInstall)
	mux.HandleFunc("POST /v1/setup/runtime/install/reconcile", s.reconcileRuntimeInstall)
}

// Setup commands run in host-owned state, never in a repository where local
// config or hooks could influence the identity that Luci displays or writes.
func (s *Server) setupHostDir(create bool) (string, error) {
	dir, err := hostpath.ResolveOutside(s.projectRoot(), s.stateDir)
	if err != nil {
		return "", errors.New("Luci state must be outside the projects folder for host setup")
	}
	if !create {
		return dir, nil
	}
	toolsDir, err := guardedProjectPath(dir, "tools", true)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(toolsDir, 0700); err != nil {
		return "", err
	}
	return guardedProjectPath(dir, "tools", false)
}

func (s *Server) globalIdentity(ctx context.Context, dir string) (GitIdentity, error) {
	var result GitIdentity
	for key, dst := range map[string]*string{"user.name": &result.Name, "user.email": &result.Email} {
		value, err := s.runner(ctx, "git", []string{"config", "--global", "--get", key}, dir)
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				continue
			}
			return result, err
		}
		*dst = strings.TrimSpace(value)
	}
	return result, nil
}

func (s *Server) getGitIdentity(w http.ResponseWriter, r *http.Request) {
	dir, err := s.setupHostDir(false)
	if err != nil {
		problem(w, 409, "host_state_required", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	identity, err := s.globalIdentity(ctx, dir)
	if err != nil {
		problem(w, 422, "git_identity_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, identity)
}

func (s *Server) setGitIdentity(w http.ResponseWriter, r *http.Request) {
	var req GitIdentity
	if !readJSON(w, r, &req) {
		return
	}
	req.Name, req.Email = strings.TrimSpace(req.Name), strings.TrimSpace(req.Email)
	address, err := mail.ParseAddress(req.Email)
	if req.Name == "" || len(req.Name) > 200 || len(req.Email) > 254 || strings.ContainsAny(req.Name, "<>") || strings.IndexFunc(req.Name+req.Email, unicode.IsControl) >= 0 || err != nil || address.Address != req.Email {
		problem(w, 400, "invalid_identity", "Enter your name and a plain email address for Git commits.")
		return
	}
	dir, err := s.setupHostDir(true)
	if err != nil {
		problem(w, 409, "host_state_required", err.Error())
		return
	}
	identityWriteMu.Lock()
	defer identityWriteMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if _, err = s.runner(ctx, "git", []string{"config", "--global", "--replace-all", "user.name", req.Name}, dir); err != nil {
		problem(w, 422, "git_identity_failed", err.Error())
		return
	}
	if _, err = s.runner(ctx, "git", []string{"config", "--global", "--replace-all", "user.email", req.Email}, dir); err != nil {
		problem(w, 422, "git_identity_partial", "Git saved your name but could not save your email. Retry to finish: "+err.Error())
		return
	}
	writeJSON(w, 200, req)
}

func (s *Server) createSSHKey(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !readJSON(w, r, &req) {
		return
	}
	hostDir, err := s.setupHostDir(true)
	if err != nil {
		problem(w, 409, "host_state_required", err.Error())
		return
	}
	sshKeyWriteMu.Lock()
	defer sshKeyWriteMu.Unlock()
	keyDir, err := guardedProjectPath(s.stateDir, "ssh", true)
	if err == nil {
		err = os.MkdirAll(keyDir, 0700)
	}
	if err != nil {
		problem(w, 409, "ssh_key_unavailable", err.Error())
		return
	}
	keyPath, err := guardedProjectPath(s.stateDir, "ssh/id_luci", true)
	if err != nil {
		problem(w, 409, "ssh_key_unavailable", err.Error())
		return
	}
	pubPath, err := guardedProjectPath(s.stateDir, "ssh/id_luci.pub", true)
	if err != nil {
		problem(w, 409, "ssh_key_unavailable", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if _, err = os.Lstat(keyPath); errors.Is(err, os.ErrNotExist) {
		if _, publicErr := os.Lstat(pubPath); !errors.Is(publicErr, os.ErrNotExist) {
			problem(w, 409, "ssh_key_exists", "A public key already exists without its private key. Existing key material will not be overwritten.")
			return
		}
		tmp, tmpErr := os.MkdirTemp(keyDir, ".key-*")
		if tmpErr != nil {
			problem(w, 500, "ssh_key_failed", tmpErr.Error())
			return
		}
		tmpKey := filepath.Join(tmp, "id_luci")
		defer os.Remove(tmp)
		defer os.Remove(tmpKey + ".pub")
		defer os.Remove(tmpKey)
		_, err = s.runner(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-C", "luci", "-f", tmpKey}, hostDir)
		if err == nil {
			err = restrictPrivateFile(tmpKey)
		}
		// A hard-link publication is atomic and refuses an existing destination
		// on every platform; rename would replace an existing key on Unix.
		if err == nil {
			err = os.Link(tmpKey, keyPath)
		}
		if err == nil {
			err = os.Link(tmpKey+".pub", pubPath)
		}
		if err != nil {
			problem(w, 422, "ssh_key_failed", "Could not create the dedicated Luci key; existing keys were retained: "+err.Error())
			return
		}
		// Drop temporary links before the ownership check below.
		_ = os.Remove(tmpKey)
		_ = os.Remove(tmpKey + ".pub")
	} else if err != nil {
		problem(w, 409, "ssh_key_unavailable", err.Error())
		return
	}
	if err = hostpath.OneName(keyPath); err != nil {
		problem(w, 409, "ssh_key_unavailable", err.Error())
		return
	}
	derived, err := s.runner(ctx, "ssh-keygen", []string{"-y", "-P", "", "-f", keyPath}, hostDir)
	if err != nil {
		problem(w, 409, "ssh_key_unavailable", "The existing private key cannot be read; it will not be overwritten: "+err.Error())
		return
	}
	public, err := os.ReadFile(pubPath)
	if err != nil || len(public) > 8192 {
		problem(w, 409, "ssh_key_unavailable", "The public key is missing or invalid; existing key material will not be overwritten.")
		return
	}
	fields, derivedFields := strings.Fields(string(public)), strings.Fields(derived)
	if len(fields) < 2 || len(derivedFields) < 2 || fields[0] != "ssh-ed25519" || fields[0] != derivedFields[0] || fields[1] != derivedFields[1] {
		problem(w, 409, "ssh_key_mismatch", "The existing public and private keys do not match; neither was overwritten.")
		return
	}
	fingerprint, err := s.runner(ctx, "ssh-keygen", []string{"-l", "-E", "sha256", "-f", pubPath}, hostDir)
	if err != nil {
		problem(w, 422, "ssh_key_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, SSHKey{PublicKey: strings.TrimSpace(string(public)), PrivateKeyPath: keyPath, Fingerprint: strings.TrimSpace(fingerprint)})
}

func (s *Server) getRuntimeInstall(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	job := s.runtimeJob
	s.mu.RUnlock()
	writeJSON(w, 200, job)
}

func (s *Server) runtimeSource() (string, error) {
	if !filepath.IsAbs(s.runtimeDir) {
		return "", errors.New("this installation does not contain the bundled runtime source; reinstall a complete Luci package")
	}
	dir, err := hostpath.ResolveOutside(s.projectRoot(), s.runtimeDir)
	if err != nil {
		return "", errors.New("bundled runtime must be outside the projects folder")
	}
	for _, relative := range []string{"images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh", "sandboxd/go.mod", "sandboxd/vendor/modules.txt"} {
		path, err := guardedProjectPath(dir, relative, false)
		if err != nil {
			return "", fmt.Errorf("bundled runtime is incomplete: %s", relative)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("bundled runtime file is unavailable: %s", relative)
		}
	}
	return dir, nil
}
