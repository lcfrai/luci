// Package connection holds the signed-in user's own Panacea credential and address, and
// reports honestly whether they work. It is the only writer of the credential store's
// PanaceaApp/panacea/* rows (Hub contract 9.1).
//
// The token enters through one route and never leaves the control plane: no response carries
// it, no log line prints it, it never reaches the Hub or a sandbox.
package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// Secrets is the subset of credstore.Secrets (Hub contract 9.1) this package uses. It is
// declared here so phase 7 compiles before Hub task C1 lands; credstore.Secrets satisfies it.
type Secrets interface {
	Get(target string) ([]byte, error)
	Put(target, userName string, blob []byte) error
	Delete(target string) error
}

// The token row is the one Hub contract 9.1 reserves (PanaceaApp/panacea/<api host>). The
// origin is not a secret and 9.1 lists no row for it, so it is NOT in the credential store:
// it is configuration, the file Config.StateDir/connection.json, {"origin","savedAt"},
// written with statepath.WriteAtomic. A credstore row named "current" would also share the
// <api host> namespace with a token row.
const (
	targetTokenBase = "PanaceaApp/panacea/" // + lowercased host
	originFile      = "connection.json"     // in Config.StateDir
)

// ErrOriginLocked is Connect's refusal of an origin that differs from the administrator's
// PANACEA_APP_PANACEA_URL. The PUT handler maps it to 400 bad_request before writeDeployError
// runs, because it is the CALLER's mistake, not Panacea's refusal.
var ErrOriginLocked = errors.New("connection: the Panacea address is set by PANACEA_APP_PANACEA_URL")

// OriginLockedMessage is the sentence the PUT answers with.
const OriginLockedMessage = "This cockpit's Panacea address is set by PANACEA_APP_PANACEA_URL. Send the key alone."

type State string

const (
	NotConnected     State = "not_connected"     // no address, or no token
	Connected        State = "connected"         // the token works and the account is approved
	AwaitingApproval State = "awaiting_approval" // the token works; Panacea says approved is not true
	TokenRejected    State = "token_rejected"    // Panacea answered 401 to this token
	Unreachable      State = "unreachable"       // the last check could not reach Panacea
)

type Status struct {
	State        State
	Origin       string // "" when none
	OriginLocked bool   // PANACEA_APP_PANACEA_URL is set: the connect form shows it read-only
	APIVersion   string // from GET /api/v1/version; "" when never reached
	Account      *deploy.Account
	Message      string    // Panacea's sentence for TokenRejected; "" otherwise
	CheckedAt    time.Time // zero = never checked
}

type Config struct {
	Secrets    Secrets
	IsNotFound func(error) bool // errors.Is(err, credstore.ErrNotFound), wired in main.go
	// StateDir is statepath.Dir("deploy"), the directory task 06's bindings.json shares. The
	// origin file lives here; an absent file is "no origin", a malformed one is logged at warn
	// and treated as absent (the user re-enters the address; the token row is untouched).
	StateDir string
	// LockedOrigin is PANACEA_APP_PANACEA_URL after ParseOrigin; "" = the user chooses.
	LockedOrigin string
	CAFile       string
	Timeout      time.Duration
	UserAgent    string
	Now          func() time.Time
	Log          *slog.Logger
	// NewClient builds a Deployer for an origin. main.go passes a closure over panacea.New;
	// tests pass one over deploytest.Fake or panaceatest.Server.
	NewClient func(origin string, tokens panacea.TokenSource) (deploy.Deployer, error)
	// Version is panacea.Version, injectable for tests.
	Version func(ctx context.Context, origin string) (string, error)
}

type Manager struct {
	cfg     Config
	current *Current

	mu     sync.Mutex
	status Status
}

// originDoc is connection.json. It carries no token and never will: the file is not a secret
// store, and a reader who can read it can read the state directory.
type originDoc struct {
	Origin  string    `json:"origin"`
	SavedAt time.Time `json:"savedAt"`
}

// staticToken is the TokenSource of the throwaway client Connect proves a candidate token
// with. It is never installed as Current's delegate.
type staticToken string

func (s staticToken) PanaceaToken(context.Context) (string, error) { return string(s), nil }

// targetFor is the credential-store row for an origin: the one name Hub contract 9.1
// reserves. The host carries its port, so two Panacea instances on one machine - which is
// exactly what a test server is - do not share a row.
func targetFor(origin string) string {
	return targetTokenBase + strings.ToLower(strings.TrimPrefix(origin, "https://"))
}

// New loads the stored origin (or the locked one) and builds the current Deployer. It makes
// NO network call: startup never blocks on Panacea. Status starts as NotConnected or, when a
// token is stored, as Connected-unchecked (CheckedAt zero) until Check runs.
//
// It errors only on a Config it cannot use: StateDir "", Secrets or NewClient nil, or an
// origin file that cannot be READ for a reason other than not existing (permission, ENOTDIR).
// A file that reads but does not parse is absent, never an error. main.go treats the error as
// fatal at startup.
func New(cfg Config) (*Manager, error) {
	switch {
	case cfg.StateDir == "":
		return nil, errors.New("connection: Config.StateDir is empty; there is nowhere to keep the Panacea address")
	case cfg.Secrets == nil:
		return nil, errors.New("connection: Config.Secrets is nil; there is nowhere to keep the Panacea token")
	case cfg.NewClient == nil:
		return nil, errors.New("connection: Config.NewClient is nil; no Panacea client can be built")
	}
	if cfg.IsNotFound == nil {
		cfg.IsNotFound = func(error) bool { return false }
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Version == nil {
		cfg.Version = func(ctx context.Context, origin string) (string, error) {
			hc, err := panacea.BuildHTTPClient(cfg.CAFile, cfg.Timeout)
			if err != nil {
				return "", err
			}
			return panacea.Version(ctx, hc, origin)
		}
	}
	m := &Manager{cfg: cfg, current: &Current{}}

	origin := cfg.LockedOrigin
	if origin == "" {
		stored, err := m.readOrigin()
		if err != nil {
			return nil, err
		}
		origin = stored
	}
	m.status = Status{State: NotConnected, Origin: origin, OriginLocked: cfg.LockedOrigin != ""}
	if origin == "" {
		return m, nil
	}
	// A stored token makes this start "connected, unchecked". The FIRST Check decides whether
	// that is true; saying not_connected here would ask a user who is connected to connect.
	if _, err := m.tokenFor(origin); err != nil {
		return m, nil
	}
	c, err := cfg.NewClient(origin, m)
	if err != nil {
		// The address parses (it came from ParseOrigin) but no client can be built for it -
		// a CA file that vanished, say. The user is not connected, and the reason is logged.
		cfg.Log.Warn("connection: could not build a Panacea client for the stored address",
			"origin", origin, "err", err)
		return m, nil
	}
	m.current.set(c)
	m.status.State = Connected
	return m, nil
}

// readOrigin returns the stored origin, or "" when there is none to read. Only an error that
// is neither "absent" nor "unparseable" is returned: a torn file is debris, not a fatal
// start, and the token row is left exactly where it is (the user re-enters the address).
func (m *Manager) readOrigin() (string, error) {
	path := filepath.Join(m.cfg.StateDir, originFile)
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("connection: read %s: %w", path, err)
	}
	var doc originDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		m.cfg.Log.Warn("connection: the stored Panacea address is unreadable and was ignored",
			"path", path, "bytes", len(raw), "err", err)
		return "", nil
	}
	origin, err := panacea.ParseOrigin(doc.Origin)
	if err != nil {
		m.cfg.Log.Warn("connection: the stored Panacea address is not an address and was ignored",
			"path", path, "err", err)
		return "", nil
	}
	return origin, nil
}

// writeOrigin publishes the address. It runs LAST in Connect, after the token Put: a crash
// between the two must leave no origin pointing at a token that is not there.
func (m *Manager) writeOrigin(origin string) error {
	raw, err := json.Marshal(originDoc{Origin: origin, SavedAt: m.cfg.Now().UTC()})
	if err != nil {
		return fmt.Errorf("connection: encode the Panacea address: %w", err)
	}
	return statepath.WriteAtomic(filepath.Join(m.cfg.StateDir, originFile), append(raw, '\n'))
}

// tokenFor reads the row for origin. A not-found is deploy.ErrNotConnected; any other read
// failure is reported as it is, because "the credential store is broken" is not "you have not
// connected" and the two send a user to different places.
func (m *Manager) tokenFor(origin string) (string, error) {
	if origin == "" {
		return "", deploy.ErrNotConnected
	}
	blob, err := m.cfg.Secrets.Get(targetFor(origin))
	switch {
	case err != nil && m.cfg.IsNotFound(err):
		return "", deploy.ErrNotConnected
	case err != nil:
		return "", &deploy.Error{Kind: deploy.KindNotConnected, Op: "token",
			Message: "The Panacea key could not be read from Windows Credential Manager.", Cause: err}
	case len(blob) == 0:
		return "", deploy.ErrNotConnected
	}
	return string(blob), nil
}

// PanaceaToken implements panacea.TokenSource. deploy.ErrNotConnected when none is stored.
func (m *Manager) PanaceaToken(context.Context) (string, error) {
	m.mu.Lock()
	origin := m.status.Origin
	m.mu.Unlock()
	return m.tokenFor(origin)
}

// Current is the Deployer every other phase 7 package holds. It is stable for the process's
// life; Connect and Disconnect swap what it delegates to.
func (m *Manager) Current() *Current { return m.current }

// Status is what the lens draws.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status.clone()
}

// clone copies the one pointer Status carries, so a caller cannot mutate the Manager's
// account through the value it was handed.
func (s Status) clone() Status {
	if s.Account != nil {
		a := *s.Account
		s.Account = &a
	}
	return s
}

// resolveOrigin answers which address a Connect is for, or refuses.
//
// A locked origin is an administrator's decision, not a default. An origin that is EQUAL to
// it after ParseOrigin is not an override and is accepted, because the connect form shows the
// locked address and a form that shows a value may send it back.
func (m *Manager) resolveOrigin(origin string) (string, error) {
	if m.cfg.LockedOrigin == "" {
		return panacea.ParseOrigin(origin)
	}
	if strings.TrimSpace(origin) == "" {
		return m.cfg.LockedOrigin, nil
	}
	got, err := panacea.ParseOrigin(origin)
	if err != nil {
		return "", err
	}
	if got != m.cfg.LockedOrigin {
		return "", fmt.Errorf("%w: %s", ErrOriginLocked, OriginLockedMessage)
	}
	return m.cfg.LockedOrigin, nil
}

// Connect proves origin and token before storing either. Nothing is stored unless Whoami
// answered 200: a stored token that does not work shows "connected" at the next start.
func (m *Manager) Connect(ctx context.Context, origin, token string) (Status, error) {
	o, err := m.resolveOrigin(origin)
	if err != nil {
		return m.Status(), err
	}
	version, err := m.cfg.Version(ctx, o)
	if err != nil {
		// The address is not Panacea, or Panacea cannot be reached. Either way the token has
		// not left this process, which is the point of checking the address first. Status is
		// NOT touched: a refused ATTEMPT says nothing about the credential already stored,
		// and the route answers this error to the caller who is looking at it.
		return m.Status(), err
	}
	probe, err := m.cfg.NewClient(o, staticToken(token))
	if err != nil {
		return m.Status(), err
	}
	account, err := probe.Whoami(ctx)
	if err != nil {
		// TokenRejected is reserved for the STORED token. This candidate was never stored,
		// so the state stays whatever it was and the 401's sentence goes to the caller.
		return m.Status(), err
	}
	// The token row first, the address last.
	if err := m.cfg.Secrets.Put(targetFor(o), account.Username, []byte(token)); err != nil {
		return m.Status(), fmt.Errorf("connection: store the Panacea key: %w", err)
	}
	if err := m.writeOrigin(o); err != nil {
		return m.Status(), err
	}

	client, err := m.cfg.NewClient(o, m)
	if err != nil {
		return m.Status(), err
	}
	m.current.set(client)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = Status{
		State: stateFor(account), Origin: o, OriginLocked: m.cfg.LockedOrigin != "",
		APIVersion: version, Account: &account, CheckedAt: m.cfg.Now().UTC(),
	}
	return m.status.clone(), nil
}

func stateFor(a deploy.Account) State {
	if a.Approved {
		return Connected
	}
	return AwaitingApproval
}

// Disconnect deletes the token row (absent is not an error) and swaps Current to
// not-connected. The origin file is KEPT, so reconnecting asks only for a token.
func (m *Manager) Disconnect(context.Context) error {
	m.mu.Lock()
	origin := m.status.Origin
	m.mu.Unlock()

	m.current.set(nil)
	if origin != "" {
		if err := m.cfg.Secrets.Delete(targetFor(origin)); err != nil && !m.cfg.IsNotFound(err) {
			return fmt.Errorf("connection: remove the Panacea key: %w", err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = Status{State: NotConnected, Origin: origin, OriginLocked: m.cfg.LockedOrigin != ""}
	return nil
}

// Check re-runs Version and Whoami with the STORED token and updates Status. It is called at
// start in a goroutine, by POST .../check, and by the watcher after it observes
// KindUnauthorized or recovers from KindUnavailable.
func (m *Manager) Check(ctx context.Context) Status {
	m.mu.Lock()
	origin := m.status.Origin
	m.mu.Unlock()
	if origin == "" {
		return m.Status()
	}
	if _, err := m.tokenFor(origin); err != nil {
		m.setState(NotConnected, "", nil)
		return m.Status()
	}
	version, err := m.cfg.Version(ctx, origin)
	if err != nil {
		m.Observe(err)
		return m.Status()
	}
	client := m.current.get()
	if client == nil {
		if client, err = m.cfg.NewClient(origin, m); err != nil {
			m.Observe(err)
			return m.Status()
		}
		m.current.set(client)
	}
	account, err := client.Whoami(ctx)
	if err != nil {
		m.Observe(err)
		return m.Status()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = Status{
		State: stateFor(account), Origin: origin, OriginLocked: m.cfg.LockedOrigin != "",
		APIVersion: version, Account: &account, CheckedAt: m.cfg.Now().UTC(),
	}
	return m.status.clone()
}

// Observe lets the watcher report what it saw without a second request: KindUnauthorized sets
// TokenRejected with the error's Message, KindUnavailable sets Unreachable, and nil clears
// neither by itself - the caller schedules a Check, which is the only thing that can say
// "connected" truthfully.
func (m *Manager) Observe(err error) {
	if err == nil {
		return
	}
	var msg string
	var state State
	switch deploy.KindOf(err) {
	case deploy.KindUnauthorized:
		state = TokenRejected
		var e *deploy.Error
		if errors.As(err, &e) {
			msg = e.Message
		}
	case deploy.KindUnavailable:
		state = Unreachable
	default:
		// KindRejected and the rest say nothing about the STORED credential: a refused
		// address is not a refused key, and overwriting the state here would tell a
		// connected user they are not.
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = state
	m.status.Message = msg
	m.status.CheckedAt = m.cfg.Now().UTC()
}

// setState is the small update Check makes when the token row has gone.
func (m *Manager) setState(s State, msg string, account *deploy.Account) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State, m.status.Message, m.status.Account = s, msg, account
	m.status.CheckedAt = m.cfg.Now().UTC()
}
