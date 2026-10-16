package api

// A connection is an explicit copy of the official CLI's ChatGPT auth file
// from one stopped, owned sign-in session. It is never a mounted shared home.
// Each managed user has a separate API, registry and ConversationsDir.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const codexAuthLimit = 64 << 10
const codexAccountRoute = "/v1/setup/agents/codex/account"

type codexConnection struct {
	ID              string          `json:"id"`
	SourceSessionID string          `json:"sourceSessionId"`
	ActiveSessionID string          `json:"activeSessionId,omitempty"`
	Auth            json.RawMessage `json:"auth"`
}

type codexAccountJSON struct {
	Configured      bool   `json:"configured"`
	AuthMode        string `json:"authMode"`
	SourceSessionID string `json:"sourceSessionId,omitempty"`
	ActiveSessionID string `json:"activeSessionId,omitempty"`
	Message         string `json:"message"`
}

func (a *API) codexAccountStatus(c *codexConnection) codexAccountJSON {
	r := codexAccountJSON{AuthMode: "chatgpt", Message: "No saved ChatGPT sign-in. Complete device sign-in in Luci, stop that session, then save its sign-in."}
	if c != nil {
		r.Configured, r.SourceSessionID, r.ActiveSessionID = true, c.SourceSessionID, c.ActiveSessionID
		if c.ActiveSessionID != "" {
			// Keep the last lease privately for refresh-token reconciliation,
			// but don't tell the UI a finished sign-in is a running session.
			s, err := a.deps.Store.Get(c.ActiveSessionID)
			if errors.Is(err, registry.ErrNotFound) || (err == nil && s.Status == registry.StatusExited) {
				r.ActiveSessionID = ""
			}
		}
		r.Message = "ChatGPT sign-in saved for this Luci account. Subscription access is checked by Codex when used."
	}
	return r
}

func (a *API) registerCodexConnection(mux *routes) {
	mux.HandleFunc("GET "+codexAccountRoute, a.handleCodexAccount)
	mux.HandleFunc("POST "+codexAccountRoute, a.handleCodexAccount)
	mux.HandleFunc("DELETE "+codexAccountRoute, a.handleCodexAccount)
}

func (a *API) handleCodexAccount(w http.ResponseWriter, r *http.Request) {
	a.codexAccountMu.Lock()
	defer a.codexAccountMu.Unlock()
	c, err := a.readCodexConnection()
	if err != nil {
		codexAccountProblem(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		// No probing, rotation, mkdir or credential content on a GET.
		httpx.WriteJSON(w, 200, a.codexAccountStatus(c))
	case http.MethodDelete:
		if c != nil {
			if err := os.Remove(a.codexConnectionPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
				codexAccountProblem(w, err)
				return
			}
		}
		httpx.WriteJSON(w, 200, a.codexAccountStatus(nil))
	case http.MethodPost:
		var req struct {
			SessionID string `json:"sessionId"`
		}
		if !httpx.DecodeJSON(w, r, &req) {
			return
		}
		s, err := a.deps.Store.Get(req.SessionID)
		if err != nil {
			httpx.WriteProblem(w, 404, httpx.CodeSessionNotFound, "No sign-in session with that id in this Luci account.", nil)
			return
		}
		if s.Harness != "codex" || !s.AuthOnly || s.AuthMode != "chatgpt" {
			httpx.WriteProblem(w, 409, "codex_sign_in_required", "Choose a completed Codex device sign-in session created by this Luci account.", nil)
			return
		}
		if c != nil && c.SourceSessionID == s.ID {
			// A retry must not replace a refresh token rotated by later project
			// sessions with the original sign-in home's older credential.
			httpx.WriteJSON(w, 200, a.codexAccountStatus(c))
			return
		}
		if !sessionRestarts.acquire(s.ID) {
			codexAccountProblem(w, errCodexBusy)
			return
		}
		defer sessionRestarts.release(s.ID)
		if s.Status != registry.StatusExited {
			codexAccountProblem(w, errCodexSignInPending)
			return
		}
		if err := a.codexStopped(r.Context(), s); err != nil {
			codexAccountProblem(w, err)
			return
		}
		if c != nil && c.ActiveSessionID != s.ID {
			if err := a.codexAvailable(r.Context(), c, ""); err != nil {
				codexAccountProblem(w, err)
				return
			}
		}
		auth, err := a.codexSessionAuth(s)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, errCodexAuthIncomplete) {
				codexAccountProblem(w, errCodexSignInIncomplete)
			} else {
				codexAccountProblem(w, errCodexSignInUnreadable)
			}
			return
		}
		c = &codexConnection{ID: ids.UUID(), SourceSessionID: s.ID, ActiveSessionID: s.ID, Auth: auth}
		if err := a.writeCodexConnection(c); err != nil {
			codexAccountProblem(w, errCodexSaveFailed)
			return
		}
		if _, err = a.deps.Store.Update(s.ID, func(s *registry.Session) error { s.CodexConnectionID = c.ID; return nil }); err != nil {
			codexAccountProblem(w, errCodexSaveFailed)
			return
		}
		httpx.WriteJSON(w, 200, a.codexAccountStatus(c))
	}
}

var errCodexBusy = errors.New("codex account has an active session")
var errCodexDisconnected = errors.New("codex account was disconnected or replaced")
var errCodexAuthIncomplete = errors.New("no complete ChatGPT device credential recorded")
var errCodexSignInIncomplete = errors.New("selected codex device sign-in is incomplete")
var errCodexSignInPending = errors.New("codex sign-in has not stopped")
var errCodexSignInUnreadable = errors.New("codex sign-in state cannot be read safely")
var errCodexSaveFailed = errors.New("codex account save was not confirmed")

func codexAccountProblem(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	code, message := "codex_account_unavailable", "Luci could not access the saved Codex connection safely. Keep the sign-in session and refresh the connection when the workspace service and private storage are available."
	switch {
	case errors.Is(err, errCodexSignInIncomplete):
		code, message = "codex_sign_in_incomplete", "Codex has not recorded a complete device sign-in in this session. Account login alone does not approve the one-time Codex code. If the device sign-in was interrupted or failed, explicitly start a new device sign-in."
	case errors.Is(err, errCodexSignInPending):
		code, message = "codex_sign_in_pending", "This Codex sign-in session has not stopped. Finish the device sign-in, then stop the session before saving it."
	case errors.Is(err, errCodexSignInUnreadable):
		code, message = "codex_sign_in_state_unavailable", "Luci could not read this session's private Codex sign-in safely. Keep the session and check private storage availability before trying Save again; a new device sign-in is not yet indicated."
	case errors.Is(err, errCodexSaveFailed):
		status = http.StatusInternalServerError
		code, message = "codex_account_save_failed", "Luci could not confirm that saving the Codex connection finished. Keep the sign-in session and refresh the connection before trying Save again."
	}
	if errors.Is(err, errCodexBusy) {
		code, message = "codex_account_busy", "Stop the active Codex subscription session before starting another or saving a different sign-in."
	}
	if errors.Is(err, errCodexDisconnected) {
		code, message = "codex_account_required", "This session's saved ChatGPT connection was disconnected or replaced. Save a sign-in and start a new session; no API key was substituted."
	}
	// Deliberately never include upstream JSON, paths or credential parse errors.
	httpx.WriteProblem(w, status, code, message, nil)
}

func (a *API) codexConnectionPath() string {
	return filepath.Join(a.deps.ConversationsDir, "connections", "codex-account.json")
}

func codexRegularPath(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("invalid private state path")
	}
	cur := root
	for _, part := range append([]string{"."}, strings.Split(rel, string(filepath.Separator))...) {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("private state contains a link")
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	return hostpath.OneName(path)
}

func codexRead(root, path string, limit int64) ([]byte, error) {
	if err := codexRegularPath(root, path); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	rel, _ := filepath.Rel(root, path)
	f, err := r.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("invalid private state file")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("private state exceeds size limit")
	}
	return b, nil
}

func validateCodexAuth(auth []byte) error {
	var parsed struct {
		AuthMode string  `json:"auth_mode"`
		APIKey   *string `json:"OPENAI_API_KEY"`
		Tokens   struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if len(auth) > codexAuthLimit || json.Unmarshal(auth, &parsed) != nil {
		return errors.New("invalid private sign-in state")
	}
	if (parsed.AuthMode != "" && parsed.AuthMode != "chatgpt") || (parsed.APIKey != nil && *parsed.APIKey != "") || parsed.Tokens.Access == "" || parsed.Tokens.Refresh == "" {
		return errCodexAuthIncomplete
	}
	return nil
}

func (a *API) codexSessionAuth(s registry.Session) (json.RawMessage, error) {
	dir, err := a.nativeStore(s.WorktreeID, "codex", s.ConversationID)
	if err != nil {
		return nil, err
	}
	if _, err := hostpath.ResolveOutside(a.projectsDir(), a.deps.ConversationsDir); err != nil {
		return nil, err
	}
	b, err := codexRead(a.deps.ConversationsDir, filepath.Join(dir, "auth.json"), codexAuthLimit)
	if err == nil {
		err = validateCodexAuth(b)
	}
	return json.RawMessage(b), err
}

func (a *API) readCodexConnection() (*codexConnection, error) {
	b, err := codexRead(a.deps.ConversationsDir, a.codexConnectionPath(), codexAuthLimit+4096)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), a.deps.ConversationsDir); err != nil {
		return nil, err
	}
	var c codexConnection
	if json.Unmarshal(b, &c) != nil || c.ID == "" || c.SourceSessionID == "" {
		return nil, errors.New("invalid private connection record")
	}
	if err := validateCodexAuth(c.Auth); err != nil {
		return nil, err
	}
	return &c, nil
}

func codexAtomicPrivate(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".codex-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = codexPrivateFile(name); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

func (a *API) writeCodexConnection(c *codexConnection) error {
	if err := validateCodexAuth(c.Auth); err != nil {
		return err
	}
	if _, err := hostpath.ResolveOutside(a.projectsDir(), a.deps.ConversationsDir); err != nil {
		return err
	}
	dir := filepath.Dir(a.codexConnectionPath())
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := codexRegularPath(a.deps.ConversationsDir, dir); err != nil {
		return err
	}
	if err := codexPrivateDir(dir); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return codexAtomicPrivate(a.codexConnectionPath(), b)
}

func (a *API) codexStopped(ctx context.Context, s registry.Session) error {
	if s.Status != registry.StatusExited {
		return errCodexBusy
	}
	if s.SandboxID == "" || (s.Endpoint == "" && s.AttachToken == "") {
		return nil
	}
	sb, err := a.deps.Backend.Get(ctx, s.SandboxID)
	if errors.Is(err, backend.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if sb.State != backend.StateStopped && sb.State != backend.StateGone {
		return errCodexBusy
	}
	return nil
}

func (a *API) codexAvailable(ctx context.Context, c *codexConnection, except string) error {
	if c.ActiveSessionID == "" || c.ActiveSessionID == except {
		return nil
	}
	s, err := a.deps.Store.Get(c.ActiveSessionID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return a.codexStopped(ctx, s)
}

// Reconcile only the last session leased to this connection generation. An old
// session can never overwrite a newer refresh token or a replacement account.
func (a *API) reconcileCodexConnection(c *codexConnection) error {
	if c.ActiveSessionID == "" {
		return nil
	}
	s, err := a.deps.Store.Get(c.ActiveSessionID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.CodexConnectionID != c.ID {
		return nil
	}
	auth, err := a.codexSessionAuth(s)
	if err != nil {
		return err
	}
	c.Auth = auth
	return a.writeCodexConnection(c)
}

// Called under codexAccountMu before sandbox creation. A bound session cannot
// silently fall back to an API key after disconnect; auth-only always opts out.
func (a *API) selectCodexConnection(ctx context.Context, source registry.Session, authOnly bool, restarting string) (*codexConnection, string, error) {
	if authOnly {
		return nil, "chatgpt", nil
	}
	c, err := a.readCodexConnection()
	if err != nil {
		return nil, "", err
	}
	if source.CodexConnectionID != "" && (c == nil || source.CodexConnectionID != c.ID) {
		return nil, "", errCodexDisconnected
	}
	if c != nil {
		if err := a.codexAvailable(ctx, c, restarting); err != nil {
			return nil, "", err
		}
		// A restart reconciles only after its own container has stopped.
		if c.ActiveSessionID != restarting {
			if err := a.reconcileCodexConnection(c); err != nil {
				return nil, "", err
			}
		}
		return c, "chatgpt", nil
	}
	return nil, source.AuthMode, nil
}

func seedCodexConnection(dir string, c *codexConnection) error {
	if c == nil {
		return nil
	}
	// The new home is private and not yet mounted. Replace only the copied
	// auth file; conversation history stays in this session's separate snapshot.
	path := filepath.Join(dir, "auth.json")
	if _, err := os.Lstat(path); err == nil {
		if err := codexRegularPath(dir, path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return codexAtomicPrivate(path, c.Auth)
}

func (a *API) leaseCodexConnection(c *codexConnection, sessionID string) error {
	if c == nil {
		return nil
	}
	c.ActiveSessionID = sessionID
	return a.writeCodexConnection(c)
}

func codexConnectionID(c *codexConnection) string {
	if c == nil {
		return ""
	}
	return c.ID
}

func codexSubscriptionArgs(args []string, env map[string]string, authMode string) []string {
	if authMode != "chatgpt" {
		return args
	}
	delete(env, "OPENAI_API_KEY")
	delete(env, "CODEX_API_KEY")
	return append([]string{"-c", `forced_login_method="chatgpt"`, "-c", `model_provider="openai"`}, args...)
}
