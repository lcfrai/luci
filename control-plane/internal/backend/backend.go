// Package backend is the seam between the control plane and whatever actually
// runs a sandbox. Two implementations exist: dockerbe (a laptop, and the
// product's genuine "local machine" feature, per docs/PROGRAM.md) and katabe
// (AKS pod sandboxing, dormant until Phase 8).
//
// Attach returns *websocket.Conn and not io.ReadWriteCloser, deliberately. The
// wire protocol is message-framed: a 32 KiB stdout frame split across two reads
// of a byte stream decodes as two frames, the second with a garbage kind byte.
// proxy.Relay already takes *websocket.Conn.
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// MaxMessageBytes raises coder/websocket's 32 KiB default read limit.
//
// SHARED RULE, THREE SITES. github.com/coder/websocket v1.8.12 caps a single
// inbound message at 32768 bytes (read.go:107). Two messages in this design
// exceed it: sandboxd's replay frame (up to 256 KiB of scrollback plus a kind
// byte) and a large paste, which arrives as one Stdin frame. Every
// *websocket.Conn on the Go side of the path must call
// SetReadLimit(MaxMessageBytes) immediately after Accept or Dial. The three
// sites are sandboxd's /attach, backend.Dial below, and the control plane's
// client Accept. Miss one and the pipe breaks at a size light testing never
// reaches. Browser WebSocket has no such limit, so the desktop needs no change.
const MaxMessageBytes = 4 << 20

// The sandbox's HTTP surface, as served by sandboxd.
const (
	AttachPath    = "/attach"
	HealthPath    = "/healthz"
	ContainerPort = 8080
)

// Environment variables injected into every sandbox. This is the downstream
// contract with sandboxd:
//   - EnvCommand   selects the harness binary
//   - EnvArgsJSON  is a JSON array of strings, because the legacy comma-joined
//     form corrupts any opening prompt containing a comma
//   - EnvToken     is the bearer token /attach requires; without it sandboxd
//     answers 401. /healthz stays unauthenticated so readiness works.
const (
	EnvCommand  = "PANACEA_APP_COMMAND"
	EnvArgsJSON = "PANACEA_APP_ARGS_JSON"
	EnvToken    = "PANACEA_APP_ATTACH_TOKEN"
)

// envReservedPrefix is the namespace a caller may not write into. Spec.Env is
// user-influenced; the three variables above are the security contract.
const envReservedPrefix = "PANACEA_APP_"

// Labels attached to every sandbox this control plane creates. Docker labels
// use dots by convention; the Kata pod labels already in internal/sandbox use
// slashes, because Kubernetes label keys do. Both are listed so neither backend
// invents a third spelling.
const (
	LabelManaged   = "panacea-app.managed"    // always "true"
	LabelSandboxID = "panacea-app.sandbox-id" // equals Spec.ID
	LabelSessionID = "panacea-app.session-id" // equals registry.Session.ID
	LabelCreatedAt = "panacea-app.created-at" // RFC3339, diagnostics only

	KubeLabelSandboxID = "panacea-app/sandbox-id"
	KubeLabelSessionID = "panacea-app/session-id"
	KubeLabelComponent = "panacea-app/component" // always "sandbox"
)

// Kind names a backend implementation. It is persisted in the registry and
// returned over the REST surface, so these strings are stable.
type Kind string

const (
	KindDocker Kind = "docker"
	KindKata   Kind = "kata"
)

// State is what the backend can observe about a sandbox. It is NOT the session
// status the user sees — registry.Status is. A backend knows nothing about
// idleness, because idleness is a fact about the relay, not about the
// container.
type State string

const (
	StateStarting State = "starting" // created, not yet answering /healthz
	StateRunning  State = "running"
	StateStopped  State = "stopped" // exists, process ended
	StateGone     State = "gone"    // no such sandbox
)

// Resources sizes a sandbox. Kata requires explicit CPU and memory because the
// guest VM is sized from them and under-requesting causes guest OOM; Docker
// takes the same numbers in different units, so the conversion lives here once.
type Resources struct {
	CPUMillis int // 1000 = one core
	MemoryMiB int
}

// DefaultResources is what a session gets when nothing overrides it.
var DefaultResources = Resources{CPUMillis: 2000, MemoryMiB: 4096}

// NanoCPUs renders CPUMillis for container.Resources.NanoCPUs.
func (r Resources) NanoCPUs() int64 { return int64(r.CPUMillis) * 1_000_000 }

// MemoryBytes renders MemoryMiB for container.Resources.Memory.
func (r Resources) MemoryBytes() int64 { return int64(r.MemoryMiB) << 20 }

// KubeCPU renders CPUMillis as a Kubernetes quantity, e.g. "2000m".
func (r Resources) KubeCPU() string { return strconv.Itoa(r.CPUMillis) + "m" }

// KubeMemory renders MemoryMiB as a Kubernetes quantity, e.g. "4096Mi".
func (r Resources) KubeMemory() string { return strconv.Itoa(r.MemoryMiB) + "Mi" }

// Mount is a host path exposed inside the sandbox. Phase 2 uses it for five
// things: giving a local Docker session the worktree it is supposed to work in;
// a primary worktree's .git/worktrees, read-only over itself, so nothing in the
// sandbox can prune or rewrite the host's linked-worktree metadata; giving a
// session whose harness has a conversation that worktree's conversation store,
// so a resume in a later sandbox can find the transcript; when the operator
// set PANACEA_APP_CA_BUNDLE, the extra root certificates a TLS-intercepting
// network needs, read-only; and the host git's global excludes and attributes
// files, read-only, each when the host's git reads one, so a commit in the
// sandbox holds what it would on the host. Kata ignores Mounts and returns
// ErrUnsupported if any are set, because a host path on an AKS node is
// meaningless — which makes the metadata mount, the conversation store, the CA
// bundle and the two git files four more things Phase 8 must provide another
// way (contract §2.6).
type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// Spec describes one sandbox to create. Every field is set by the caller; a
// backend never defaults anything except where documented.
type Spec struct {
	ID          string            // caller-chosen, unique, matches ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$
	SessionID   string            // for the label only; a backend never reads it back
	Image       string            // e.g. "panacea-app/sandbox:dev"
	Command     string            // becomes PANACEA_APP_COMMAND
	Args        []string          // JSON-encoded into PANACEA_APP_ARGS_JSON
	Env         map[string]string // extra environment; PANACEA_APP_* keys are rejected
	AttachToken string            // becomes PANACEA_APP_ATTACH_TOKEN; never logged
	Resources   Resources
	WorkDir     string  // container-side working directory, "" leaves the image default
	Mounts      []Mount // docker only
}

// EnvEntry is one environment variable, in order.
type EnvEntry struct {
	Name  string
	Value string
}

// ArgsJSON encodes Args for PANACEA_APP_ARGS_JSON.
//
// No arguments encodes as "[]" and never as "": sandboxd treats an empty
// PANACEA_APP_ARGS_JSON as "fall back to the legacy comma-separated form", and
// that form splits an opening prompt on its commas.
func (s Spec) ArgsJSON() string {
	if len(s.Args) == 0 {
		return "[]"
	}
	b, err := json.Marshal(s.Args)
	if err != nil {
		// json.Marshal of a []string cannot fail. The branch exists so that if
		// Args ever stops being a []string the failure is an empty argv rather
		// than a corrupted one.
		return "[]"
	}
	return string(b)
}

// EnvEntries returns the sandbox's environment in a fixed order: the three
// PANACEA_APP_* variables first, then every Spec.Env key sorted. The order is
// fixed so both backends produce identical environments and a test can assert
// on a slice instead of a set.
//
// ValidateSpec is the loud check: it rejects a Spec carrying a reserved
// PANACEA_APP_* key in Env, with an error a caller sees before anything runs.
// This function is the backstop: it silently drops any reserved key that
// reaches it anyway, so a backend that forgot to call ValidateSpec first still
// cannot have a caller-chosen value shadow the real attach token. Neither is
// redundant with the other — one is the message, one is the guarantee.
func (s Spec) EnvEntries() []EnvEntry {
	out := make([]EnvEntry, 0, 3+len(s.Env))
	out = append(out,
		EnvEntry{Name: EnvCommand, Value: s.Command},
		EnvEntry{Name: EnvArgsJSON, Value: s.ArgsJSON()},
		EnvEntry{Name: EnvToken, Value: s.AttachToken},
	)
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, envReservedPrefix) {
			continue // backstop: see the doc comment above.
		}
		out = append(out, EnvEntry{Name: k, Value: s.Env[k]})
	}
	return out
}

// idPattern is the sandbox id both backends accept. Docker container names
// must start with an alphanumeric; RFC 1123 requires a Kubernetes object name
// to start AND end alphanumeric. This pattern is the intersection of both, so
// a Spec that passes here cannot be the thing that turns "docker accepted it"
// into "kata rejected it" — a bare hyphen or a leading/trailing one would.
var idPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

// ValidateSpec rejects a Spec no backend can honour. Both implementations call
// it as the first statement of Create, so "docker accepted it and kata did not"
// cannot happen. Every failure wraps ErrUnsupported, which the API layer maps
// to 400.
func ValidateSpec(spec Spec) error {
	if !idPattern.MatchString(spec.ID) {
		return fmt.Errorf("spec.ID %q must match ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$: %w", spec.ID, ErrUnsupported)
	}
	if strings.TrimSpace(spec.Image) == "" {
		return fmt.Errorf("spec.Image is empty: %w", ErrUnsupported)
	}
	if strings.TrimSpace(spec.Command) == "" {
		return fmt.Errorf("spec.Command is empty: %w", ErrUnsupported)
	}
	if spec.AttachToken == "" {
		return fmt.Errorf("spec.AttachToken is empty: an unauthenticated /attach is a shell any local process can reach: %w", ErrUnsupported)
	}
	for k := range spec.Env {
		if strings.HasPrefix(k, envReservedPrefix) {
			return fmt.Errorf("spec.Env key %q is reserved: a caller that could set %s could replace the sandbox's secret: %w", k, EnvToken, ErrUnsupported)
		}
	}
	if spec.Resources.CPUMillis <= 0 || spec.Resources.MemoryMiB <= 0 {
		return fmt.Errorf("spec.Resources must be positive, got %d millicpu and %d MiB: %w", spec.Resources.CPUMillis, spec.Resources.MemoryMiB, ErrUnsupported)
	}
	if spec.WorkDir != "" && !strings.HasPrefix(spec.WorkDir, "/") {
		return fmt.Errorf("spec.WorkDir %q is not an absolute container path: %w", spec.WorkDir, ErrUnsupported)
	}
	for i, m := range spec.Mounts {
		if !strings.HasPrefix(m.ContainerPath, "/") {
			return fmt.Errorf("spec.Mounts[%d].ContainerPath %q is not an absolute container path: %w", i, m.ContainerPath, ErrUnsupported)
		}
		if !filepath.IsAbs(m.HostPath) {
			return fmt.Errorf("spec.Mounts[%d].HostPath %q is not an absolute host path: %w", i, m.HostPath, ErrUnsupported)
		}
	}
	return nil
}

// Sandbox is what a backend knows about one sandbox.
//
// Endpoint is host:port as reachable FROM THE CONTROL PLANE, not from the
// desktop. The desktop never learns it — it attaches through
// /v1/sessions/{id}/attach. For dockerbe it is 127.0.0.1:<published port>; for
// katabe it is <podIP>:8080.
type Sandbox struct {
	ID       string
	Kind     Kind
	Ref      string // container id, or pod name
	Endpoint string

	// NonLoopback is true when the daemon reports this sandbox published to any
	// address that is not loopback — 0.0.0.0, ::, the empty spelling Docker
	// reads as every interface, or a concrete LAN address. It is set only
	// by List, which must not fail a whole sweep over one bad container: Get and
	// Attach on the same id return ErrNonLoopbackPublish instead. A sandbox with
	// this set is a security defect the reaper destroys; Endpoint is always empty
	// when it is true, so nothing can attach to it in the meantime.
	NonLoopback bool

	State     State
	CreatedAt time.Time
	Labels    map[string]string
}

// Backend creates, finds, attaches to and destroys sandboxes.
//
// Attach takes the token explicitly rather than reading it back from the
// sandbox, so the secret lives in exactly one durable place (the registry) and
// is not recoverable by anything that can merely list containers.
type Backend interface {
	Kind() Kind
	Create(ctx context.Context, spec Spec) (Sandbox, error)
	Get(ctx context.Context, id string) (Sandbox, error)
	List(ctx context.Context) ([]Sandbox, error)
	Attach(ctx context.Context, id, token string) (*websocket.Conn, error)
	Destroy(ctx context.Context, id string) error
	Close() error
}

// Sentinel errors. Every implementation wraps these with %w; the API layer maps
// them to status codes with errors.Is and never string-matches.
var (
	// ErrNotFound: no sandbox with that id. Maps to 404 session_not_found when
	// the registry also has no record, or 409 sandbox_not_running when it does.
	ErrNotFound = errors.New("backend: sandbox not found")
	// ErrNotRunning: the sandbox exists but is not accepting connections. 409.
	ErrNotRunning = errors.New("backend: sandbox is not running")
	// ErrAlreadyExists: Spec.ID is taken. 409 conflict.
	ErrAlreadyExists = errors.New("backend: sandbox already exists")
	// ErrUnavailable: the backend itself is down (no Docker daemon, no
	// kubeconfig). 503 backend_unavailable.
	ErrUnavailable = errors.New("backend: backend is unavailable")
	// ErrUnauthorized: /attach answered 401, i.e. the stored token is wrong. 500.
	ErrUnauthorized = errors.New("backend: attach token rejected")
	// ErrUnsupported: this backend cannot honour part of the Spec. 400.
	ErrUnsupported = errors.New("backend: unsupported by this backend")
	// ErrNonLoopbackPublish: the sandbox port was published to something other
	// than 127.0.0.1. Refusing is the point: a published 0.0.0.0 port is a
	// shell on the LAN. 500, and the sandbox is destroyed.
	ErrNonLoopbackPublish = errors.New("backend: sandbox port is not bound to 127.0.0.1")
)
