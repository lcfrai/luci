package binding

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// DefaultHubReadTimeout bounds ONE Hub lookup. It exists because ResolveMany
// runs inside the watcher's cycle: with a Hub that never answers, unbounded
// lookups spend the whole cycle budget and starve every other read in it
// (carried b09-2 I-1). The other half of that fix is in resolveAll: after the
// first lookup that fails as a blip, the rest of the cycle's lookups are
// skipped, because they would all fail the same way.
const DefaultHubReadTimeout = 5 * time.Second

type Config struct {
	Projects         LocalProjects             // gives the local project's Path
	Remotes          Remotes                   // gives its origin URL
	Deployer         deploy.Identity           // connection.Manager.Current()
	ParseRemote      ParseRemote               // phase 3's githost, through main.go's adapter
	PanaceaOrigin    func() string             // the address a stored choice is scoped to
	Hub              HubProjects               // nil until the Hub is configured: HubProjectID stays ""
	HubKeyAccepted   func(string) bool         // the Hub's own key rule; required when Hub is set
	ClassifyHubError func(error) HubErrorClass // what a Hub failure means; nil = every failure is a blip
	HubReadTimeout   time.Duration             // per lookup; 0 takes DefaultHubReadTimeout
	ChoicesDir       string                    // statepath.Dir("deploy")
	Now              func() time.Time
	Log              *slog.Logger
}

// Resolver is the one binding for the whole process: the watcher (task 07) and
// the read routes (tasks 09, 11, 12) hold the same pointer, so they cannot
// disagree about which project deploys a repository.
type Resolver struct {
	cfg     Config
	mu      sync.Mutex
	cache   map[string]Binding
	choices *choiceFile
}

func New(cfg Config) (*Resolver, error) {
	switch {
	case cfg.Projects == nil:
		return nil, errors.New("binding: Config.Projects is required")
	case cfg.Remotes == nil:
		return nil, errors.New("binding: Config.Remotes is required")
	case cfg.Deployer == nil:
		return nil, errors.New("binding: Config.Deployer is required")
	case cfg.ParseRemote == nil:
		return nil, errors.New("binding: Config.ParseRemote is required")
	case cfg.ChoicesDir == "":
		return nil, errors.New("binding: Config.ChoicesDir is required")
	// A Hub with no rule wired would report every miss as not_linked, which
	// reads as "this repository is not a shared project" about keys the Hub
	// can never hold. Refusing here makes that a startup failure, not a
	// sentence the user is shown for months.
	case cfg.Hub != nil && cfg.HubKeyAccepted == nil:
		return nil, errors.New("binding: Config.HubKeyAccepted is required when Hub is set")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.HubReadTimeout <= 0 {
		cfg.HubReadTimeout = DefaultHubReadTimeout
	}
	if cfg.PanaceaOrigin == nil {
		cfg.PanaceaOrigin = func() string { return "" }
	}
	ch, err := loadChoices(cfg.ChoicesDir, cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("binding: %w", err)
	}
	return &Resolver{cfg: cfg, cache: map[string]Binding{}, choices: ch}, nil
}

// Cached is the last computed binding, or (Binding{}, false).
func (r *Resolver) Cached(projectID string) (Binding, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.cache[projectID]
	return b, ok
}
