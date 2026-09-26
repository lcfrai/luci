// Package discoverhub is the control plane's Hub-backed brief service. It is
// the only phase 4 package that calls the Hub, and it calls it as the
// signed-in user through hubclient (ADR-H1 §5.4). It persists nothing: the
// only state it keeps is an in-memory cache of immutable revisions and a
// 60-second memo of each local project's binding.
package discoverhub

import (
	"container/heap"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// Hub is the part of hubclient this package uses. It is phase 4's own
// interface. HA4-2 reached the Hub in H0 (hubclient.Briefs declares List,
// hub 3df9fab8), so *hubclient.Client and *hubclient.Fake satisfy Hub
// unchanged; List is still declared here, identically, so a change to its
// signature in the Hub is a compile error in this package, the right place to
// learn it. Nothing in this package writes under internal/hub.
type Hub interface {
	hubclient.Briefs
	hubclient.Projects
	List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) // HA4-2
}

// RepoKey is one remote of a local repository, in the Hub's form.
type RepoKey struct {
	Key    string // githost.Remote.RepoKey() (phase 3 task 13)
	Host   string // "github" | "azure-devops"
	Remote string // the git remote's name: "origin", "upstream", …
}

// RepoIdentity lists a local repository's recognised remotes, in no order
// this package relies on (task 14 returns git's, which is alphabetical).
// Unrecognised hosts are left out, not errors. Task 14 wires it over phase
// 3's githost.ParseRemote and a read-only `git remote` /
// `git remote get-url` through project.Runner.
type RepoIdentity interface {
	RepoKeys(ctx context.Context, projectPath string) ([]RepoKey, error)
}

// Locator maps local ids to what this package needs. Task 14 adapts
// *project.Scanner to it.
type Locator interface {
	ProjectPath(ctx context.Context, projectID string) (string, error) // project.ErrUnknownProject when absent
	ProjectName(projectID string) string
	BranchName(branchID string) string // "evidence-upload"; "" for ""
}

type Config struct {
	Hub      Hub // nil: the Hub is not configured (PANACEA_APP_HUB_URL unset)
	Repos    RepoIdentity
	Locate   Locator
	Model    modelclient.Completer // nil: no model; follow-ups degrade
	Evaluate provenance.Evaluate   // nil: every criterion open
	Now      func() time.Time      // nil: time.Now
	NewID    func(prefix string) string
	Log      *slog.Logger // nil: discarded
	// CacheTTL bounds how long a resolved revision is held. Revisions are
	// immutable, but an org admin can ERASE one (ADR-H1 §5.6); until the
	// change-feed hook of ask HA4-8 exists, the TTL is how an erasure
	// reaches this process. Default 5 minutes; at most 4096 entries.
	CacheTTL time.Duration
}

// DefaultCacheTTL and MaxCacheEntries bound the revision cache (rule 4).
const (
	DefaultCacheTTL = 5 * time.Minute
	MaxCacheEntries = 4096
	// bindingMemo is how long Binding's answer is held per local project.
	bindingMemo = 60 * time.Second
)

// Service is the Hub-backed brief service. It is safe for concurrent use.
type Service struct {
	cfg   Config
	cache *revisionCache

	mu   sync.Mutex
	memo map[string]memoEntry // by LOCAL project id
}

type memoEntry struct {
	b       Binding
	expires time.Time
}

// New checks cfg. Repos and Locate are required; Hub may be nil (rule 6).
// Now defaults to time.Now, NewID to ids.Wide, Log to a discarding logger,
// CacheTTL to DefaultCacheTTL.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Repos == nil:
		return nil, errors.New("discoverhub: Config.Repos is required")
	case cfg.Locate == nil:
		return nil, errors.New("discoverhub: Config.Locate is required")
	case cfg.CacheTTL < 0:
		return nil, errors.New("discoverhub: Config.CacheTTL must not be negative")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.NewID == nil {
		cfg.NewID = ids.Wide
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = DefaultCacheTTL
	}
	return &Service{cfg: cfg, cache: newRevisionCache(MaxCacheEntries), memo: map[string]memoEntry{}}, nil
}

var (
	ErrHubOff   = errors.New("discoverhub: the Hub is not configured")
	ErrNotBound = errors.New("discoverhub: this project is not shared through the Hub")
	ErrNoRemote = errors.New("discoverhub: this repository has no GitHub or Azure DevOps remote")
	ErrNoBrief  = errors.New("discoverhub: no brief at this scope")
	// ErrPinUnreadable: a branch revision's pinned project revision is one the
	// Hub answers missing (task 07 as built, not in the part). Inheritable and
	// HeadRef return it rather than resolve the branch against anything but
	// its pin (rule 3); Resolve, which has no error, answers the slot
	// unavailable instead (rule 5).
	ErrPinUnreadable = errors.New("discoverhub: the project revision this branch revision pins cannot be read")
)

// ── the revision cache (rule 4) ─────────────────────────────────────────────

// piece is one resolved revision as this package holds it: decoded once,
// immutable, never erased (an erased answer evicts it).
type piece struct {
	rev       discover.Revision
	digest    string
	projectID string
	savedAt   string
	savedBy   string // display name
}

// revisionCache is a map plus a min-heap on expiry under one mutex; no
// goroutine (G8). An entry's expiry is set when it is stored and never
// extended.
type revisionCache struct {
	mu      sync.Mutex
	max     int
	entries map[string]*cacheEntry
	byTime  expiryHeap
}

type cacheEntry struct {
	key     string
	p       piece
	expires time.Time
	index   int // in byTime
}

func newRevisionCache(max int) *revisionCache {
	return &revisionCache{max: max, entries: map[string]*cacheEntry{}}
}

// get is key's piece while it has not expired; an expired entry is dropped.
func (c *revisionCache) get(key string, now time.Time) (piece, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if e == nil {
		return piece{}, false
	}
	if !now.Before(e.expires) {
		c.drop(e)
		return piece{}, false
	}
	return e.p, true
}

// put stores p until now+ttl unless key is already held and live, which it
// leaves as it is: same bytes, same expiry (rule 5). Expired entries go
// first, then the soonest to expire while the cache is full.
func (c *revisionCache) put(key string, p piece, now time.Time, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		if now.Before(e.expires) {
			return
		}
		c.drop(e)
	}
	for len(c.byTime) > 0 && !now.Before(c.byTime[0].expires) {
		c.drop(c.byTime[0])
	}
	for len(c.entries) >= c.max && len(c.byTime) > 0 {
		c.drop(c.byTime[0])
	}
	e := &cacheEntry{key: key, p: p, expires: now.Add(ttl)}
	c.entries[key] = e
	heap.Push(&c.byTime, e)
}

// evict removes key: the Hub answered it erased or missing (rule 5).
func (c *revisionCache) evict(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.drop(e)
	}
}

func (c *revisionCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// drop removes e from both structures; c.mu is held.
func (c *revisionCache) drop(e *cacheEntry) {
	delete(c.entries, e.key)
	heap.Remove(&c.byTime, e.index)
}

type expiryHeap []*cacheEntry

func (h expiryHeap) Len() int           { return len(h) }
func (h expiryHeap) Less(i, j int) bool { return h[i].expires.Before(h[j].expires) }
func (h expiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *expiryHeap) Push(x any) {
	e := x.(*cacheEntry)
	e.index = len(*h)
	*h = append(*h, e)
}
func (h *expiryHeap) Pop() any {
	old := *h
	e := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	return e
}
