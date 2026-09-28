// Package deploywatch polls the deploy target for every bound local project,
// holds the newest reading in memory, writes the Hub deployment join, and
// exposes a cursor that changes when any reading does. It persists nothing
// (plan 7P-13).
//
// It is ONE goroutine and ONE ticker. The desktop's one-clock rule is about the
// renderer; the control plane already runs its own timers (the reaper, the
// Hub's change-feed poll) and this is one more - the only thing that polls
// Panacea.
package deploywatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
)

// The default cadences (plan §6).
const (
	DefaultInterval        = 15 * time.Second
	DefaultActiveInterval  = 3 * time.Second
	DefaultHistoryInterval = 60 * time.Second
	// DefaultHubReadTimeout is the floor for an unset HubReadTimeout; the real
	// default is the larger window CycleDeadline leaves (see New).
	DefaultHubReadTimeout = 5 * time.Second
	// RecheckFactor makes the recheck interval 4 x Interval (60 s at the
	// defaults). A recheck is measured in TIME since the last one, never
	// counted in cycles or polls.
	RecheckFactor = 4
	// ownerProbeInterval is 7P-9's "once per binding and then every 10 minutes".
	ownerProbeInterval = 10 * time.Minute
	// maxWantedBranches is the per-project cap on WantBranch; the least
	// recently wanted is dropped from BOTH branch maps (carried b02-2 M-5).
	maxWantedBranches = 16
	// targetedPinReadsPerCycle bounds step 6's beyond-the-page reads (review M11).
	targetedPinReadsPerCycle = 20
	// stopsBeforeFront is how many consecutive budget stops move a project to
	// the front of the rotation, so its next poll has the whole budget ahead of
	// it (carried b09-2 I-1, point 4).
	stopsBeforeFront = 2
)

// Bindings is the part of binding.Resolver the watcher holds.
type Bindings interface {
	ResolveMany(ctx context.Context, projectIDs []string) (map[string]binding.Binding, error)
	Cached(projectID string) (binding.Binding, bool)
}

// Connection is the part of connection.Manager the watcher holds.
type Connection interface {
	Observe(err error)
	Status() connection.Status
	Check(ctx context.Context) connection.Status
}

type Config struct {
	Deployer   deploy.Reader
	Governor   deploy.Governor // the owner probe only (7P-9)
	Projects   binding.LocalProjects
	Bindings   Bindings
	Connection Connection
	// Releases nil = no Hub configured: ReleasesState "no_hub".
	Releases Releases
	Me       Me
	// ClassifyHubError says what a Hub failure MEANS. nil = every failure is a
	// blip, which reads as "unavailable" - never as "signed out", because
	// telling a user to sign in when the Hub is merely down wastes their time
	// in the other direction.
	ClassifyHubError func(error) binding.HubErrorClass
	// Interval, ActiveInterval, HistoryInterval: plan §6. Zero takes the defaults.
	Interval, ActiveInterval, HistoryInterval time.Duration
	// ReadTimeout is the Panacea client's per-attempt timeout; main.go passes
	// the same value it gives panacea.Config.Timeout. HubReadTimeout bounds one
	// Hub read and is applied by the watcher itself.
	ReadTimeout, HubReadTimeout time.Duration
	// CycleDeadline is one cycle's budget, every project's reads together.
	CycleDeadline time.Duration
	Now           func() time.Time
	Log           *slog.Logger
}

// Watcher is the one poller. Every field but cursor is under mu.
type Watcher struct {
	cfg    Config
	mu     sync.Mutex
	states map[string]*state
	order  []string // rotation order, by local project id
	// startAt is the index in order the next cycle's rotation begins at: the
	// project a budget stop came in, else the one after the last polled.
	startAt int
	// hubFirst makes the next cycle run the Hub phase before the Panacea one,
	// so a stop starves neither phase.
	hubFirst bool
	// hubDownUntil is the Hub failure rule's retry floor: while it is in the
	// future the Hub phase is not attempted (carried b09-2 I-1, point 2).
	hubDownUntil time.Time
	// meID is the signed-in Hub principal, read once and re-read when it fails.
	meID     string
	lastGate time.Time // the last Connection.Check, for the gate's recheck
	cursor   atomic.Uint64
}

func New(cfg Config) (*Watcher, error) {
	switch {
	case cfg.Deployer == nil:
		return nil, errors.New("deploywatch: Config.Deployer is required")
	case cfg.Projects == nil:
		return nil, errors.New("deploywatch: Config.Projects is required")
	case cfg.Bindings == nil:
		return nil, errors.New("deploywatch: Config.Bindings is required")
	case cfg.Connection == nil:
		return nil, errors.New("deploywatch: Config.Connection is required")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.ActiveInterval <= 0 {
		cfg.ActiveInterval = DefaultActiveInterval
	}
	if cfg.HistoryInterval <= 0 {
		cfg.HistoryInterval = DefaultHistoryInterval
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.CycleDeadline <= 0 {
		cfg.CycleDeadline = cfg.ReadTimeout + cfg.Interval
	}
	// The budget must hold one whole attempt, so a read that hangs ends as its
	// own timeout - Panacea's failure - and never as a budget stop.
	if cfg.CycleDeadline < cfg.ReadTimeout+cfg.ActiveInterval {
		return nil, fmt.Errorf("deploywatch: CycleDeadline %s must be at least ReadTimeout plus ActiveInterval (%s)",
			cfg.CycleDeadline, cfg.ReadTimeout+cfg.ActiveInterval)
	}
	if err := setHubReadTimeout(&cfg); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Watcher{cfg: cfg, states: map[string]*state{}}, nil
}

// setHubReadTimeout keeps HubReadTimeout + ReadTimeout within the budget, so a
// Hub read can never leave the cycle with no room for one Panacea attempt
// (carried b09-2 I-1, point 3).
func setHubReadTimeout(cfg *Config) error {
	room := cfg.CycleDeadline - cfg.ReadTimeout
	if cfg.HubReadTimeout <= 0 {
		cfg.HubReadTimeout = room - cfg.ActiveInterval
		if cfg.HubReadTimeout < DefaultHubReadTimeout {
			cfg.HubReadTimeout = DefaultHubReadTimeout
		}
		if cfg.HubReadTimeout > room {
			cfg.HubReadTimeout = room
		}
		return nil
	}
	if cfg.HubReadTimeout > room {
		return fmt.Errorf("deploywatch: HubReadTimeout %s is above CycleDeadline minus ReadTimeout (%s)",
			cfg.HubReadTimeout, room)
	}
	return nil
}

func (w *Watcher) recheckInterval() time.Duration { return RecheckFactor * w.cfg.Interval }

// CursorName and Cursor implement api.CursorSource.
func (w *Watcher) CursorName() string { return "Deploy" }

// Cursor is a decimal uint64 that increases whenever ANY project's
// Snapshot.Version does; "" before the first cycle.
func (w *Watcher) Cursor() string {
	n := w.cursor.Load()
	if n == 0 {
		return ""
	}
	return strconv.FormatUint(n, 10)
}

// Snapshot is the newest reading for one local project; ok false before the
// first cycle that reached it.
func (w *Watcher) Snapshot(projectID string) (Snapshot, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	st, ok := w.states[projectID]
	if !ok || !st.published {
		return Snapshot{}, false
	}
	return st.snap.clone(), true
}

// SameEnvironment is its Reader's SameEnvironment, unchanged: the rule is the
// target's, and a rule of the watcher's own would be a second one (b02 R4).
func (w *Watcher) SameEnvironment(a, b string) bool {
	return w.cfg.Deployer.SameEnvironment(a, b)
}
