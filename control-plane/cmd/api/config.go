package main

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// Every environment variable this process reads, in one block. Contract §4
// forbids any other package calling os.Getenv at use time, so a misspelling is
// visible here and a test constructs a Config instead of editing the
// environment it is running in.
const (
	envAddr                   = "PANACEA_APP_ADDR"
	envAllowPublicBind        = "PANACEA_APP_ALLOW_PUBLIC_BIND"
	envBackend                = "PANACEA_APP_BACKEND"
	envSandboxImage           = "PANACEA_APP_SANDBOX_IMAGE"
	envProjectsDir            = "PANACEA_APP_PROJECTS_DIR"
	envAllowedOrigins         = "PANACEA_APP_ALLOWED_ORIGINS"
	envIdleAfter              = "PANACEA_APP_IDLE_AFTER"
	envIdleTimeout            = "PANACEA_APP_IDLE_TIMEOUT"
	envReapInterval           = "PANACEA_APP_REAP_INTERVAL"
	envSessionCPUMillis       = "PANACEA_APP_SESSION_CPU_MILLIS"
	envSessionMemoryMiB       = "PANACEA_APP_SESSION_MEMORY_MIB"
	envDockerHost             = "PANACEA_APP_DOCKER_HOST"
	envKubeconfig             = "PANACEA_APP_KUBECONFIG"
	envNamespace              = "PANACEA_APP_NAMESPACE"
	envLogLevel               = "PANACEA_APP_LOG_LEVEL"
	envHarnessAllowUnverified = "PANACEA_APP_HARNESS_ALLOW_UNVERIFIED"

	// envReadyTimeout is NOT in contract §4's table. Task 5 records it as a
	// cross-task dependency of this task: dockerbe.Config.ReadyTimeout exists
	// but nothing sets it, so production always gets backend.ReadyTimeout and an
	// operator whose first pull of the sandbox image is cold has no way to wait
	// longer. This task adds exactly this one row to that table.
	envReadyTimeout = "PANACEA_APP_READY_TIMEOUT"

	// envGitName and envGitEmail are the commit identity every sandbox's git
	// uses, as a pair. Unset, each session takes the identity the host's own git
	// would stamp on a commit in that worktree (gitidentity.go). Added by the
	// sandbox git fix, with envCABundle; contract §4 carries all three rows.
	envGitName  = "PANACEA_APP_GIT_NAME"
	envGitEmail = "PANACEA_APP_GIT_EMAIL"

	// envCABundle is an absolute host path to a PEM bundle every sandbox trusts
	// IN ADDITION to its own roots — the root certificate of a TLS-intercepting
	// proxy, without which Claude Code cannot reach its API from any sandbox on
	// such a network. Unset mounts nothing and sets nothing.
	envCABundle = "PANACEA_APP_CA_BUNDLE"

	// envStateDir is statepath's own constant, not a second spelling of it.
	// statepath.Dir reads this variable (contract §2.7) and this file reports
	// it; a rename in either place must break the build, not the ladder.
	envStateDir = statepath.EnvStateDir
)

const (
	defaultAddr         = "127.0.0.1:8730"
	defaultBackend      = string(backend.KindDocker)
	defaultSandboxImage = "panacea-app/sandbox:dev"
	defaultNamespace    = "panacea-app"
	defaultIdleAfter    = 90 * time.Second
	defaultReapInterval = 30 * time.Second
	minReapInterval     = 5 * time.Second
)

// backendScoped is contract §4's three backend-specific variables and the one
// backend each is read by.
//
// It is a table and one loop rather than three conditionals because of what
// happens otherwise: warning about one and staying silent about the others is
// arbitrary in a way the next reader cannot tell from an oversight, which is
// exactly how PANACEA_APP_KUBECONFIG came to be parsed and then dropped on the
// floor. The rule is that no PANACEA_APP_* variable is ever silently ignored.
//
// PANACEA_APP_KUBECONFIG set under kata is deliberately NOT a warning: it takes
// kubeClient's refusal instead, which says "this build cannot do what you asked"
// rather than "this has no effect here". The two must not be confused.
var backendScoped = []struct {
	name      string
	appliesTo string
}{
	{envDockerHost, string(backend.KindDocker)},
	{envKubeconfig, string(backend.KindKata)},
	{envNamespace, string(backend.KindKata)},
}

// Config is the whole of this process's configuration, parsed once at startup.
type Config struct {
	Addr            string
	AllowPublicBind bool
	Backend         string // "docker" or "kata"; validated here, mapped in main.go
	SandboxImage    string
	StateDir        string // "" means the statepath ladder decides
	SessionsPath    string // the registry document, derived from StateDir
	// ConversationsDir is the root every worktree's conversation store lives
	// under, derived from StateDir beside the registry. Absolute always: it
	// becomes a bind mount's host path.
	ConversationsDir string
	ProjectsDir      string // "" means no projects, and therefore no worktrees
	AllowedOrigins   []string
	IdleAfter        time.Duration // 0 disables idle derivation
	IdleTimeout      time.Duration // 0 means never destroy an idle sandbox
	ReapInterval     time.Duration
	ReadyTimeout     time.Duration
	CPUMillis        int
	MemoryMiB        int
	DockerHost       string
	Kubeconfig       string // parsed so kubeClient can refuse it by name
	Namespace        string
	LogLevel         slog.Level

	HarnessAllowUnverified bool

	// GitName and GitEmail are both set or both empty; loadConfig refuses one
	// without the other. Empty means gitIdentityResolver asks the host's git.
	GitName  string
	GitEmail string
	// CABundle is an absolute path to a file loadConfig has read and found to
	// hold at least one PEM certificate, or "" for none.
	CABundle string

	// Warnings are the complaints that must not stop the process. They are
	// collected rather than printed because the logger does not exist yet —
	// its level is one of the things being parsed — and main emits them as the
	// first lines it logs.
	Warnings []string
}

// Resources renders the two size variables for backend.Spec.
func (c Config) Resources() backend.Resources {
	return backend.Resources{CPUMillis: c.CPUMillis, MemoryMiB: c.MemoryMiB}
}

// logWarnings emits everything loadConfig collected, one line each. It is a
// function rather than three lines inside run so that a test can assert on what
// an operator actually sees, which is the log line and not the slice.
func logWarnings(log *slog.Logger, warnings []string) {
	for _, w := range warnings {
		log.Warn(w)
	}
}

// loadConfig parses the environment through getenv. Taking the lookup as a
// parameter rather than calling os.Getenv is what makes every test in
// config_test.go parallel-safe: nothing here mutates process state.
func loadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	// raw is the value exactly as the operator set it, so "set but the same as
	// the default" is distinguishable from "unset" — which is the difference
	// PANACEA_APP_NAMESPACE's warning turns on.
	raw := func(name string) string { return strings.TrimSpace(getenv(name)) }
	get := func(name, def string) string {
		if v := raw(name); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Addr:         get(envAddr, defaultAddr),
		Backend:      strings.ToLower(get(envBackend, defaultBackend)),
		SandboxImage: get(envSandboxImage, defaultSandboxImage),
		StateDir:     get(envStateDir, ""),
		ProjectsDir:  get(envProjectsDir, ""),
		DockerHost:   get(envDockerHost, ""),
		Kubeconfig:   get(envKubeconfig, ""),
		Namespace:    get(envNamespace, defaultNamespace),
		LogLevel:     slog.LevelInfo,
	}
	cfg.AllowPublicBind = parseFlag(raw(envAllowPublicBind))
	cfg.HarnessAllowUnverified = parseFlag(raw(envHarnessAllowUnverified))

	if cfg.Backend != string(backend.KindDocker) && cfg.Backend != string(backend.KindKata) {
		return Config{}, fmt.Errorf("%s=%q is not a backend: use %q or %q", envBackend, cfg.Backend, backend.KindDocker, backend.KindKata)
	}

	// The backend is selected, so anything scoped to the other one is a mistake
	// in the operator's environment — and a mistake they are told about rather
	// than a value that vanishes. Not an error: an inapplicable variable is no
	// reason to refuse to run.
	for _, v := range backendScoped {
		if raw(v.name) != "" && cfg.Backend != v.appliesTo {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("%s is set but only the %s backend reads it, and %s=%s is selected; it has no effect", v.name, v.appliesTo, envBackend, cfg.Backend))
		}
	}

	var err error
	// A malformed duration exits rather than falling back. PANACEA_APP_IDLE_AFTER=90
	// is not a Go duration; treating it as 90s guesses, and treating it as the
	// default ignores an operator who plainly meant something.
	if cfg.IdleAfter, err = parseDuration(envIdleAfter, raw(envIdleAfter), defaultIdleAfter); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = parseDuration(envIdleTimeout, raw(envIdleTimeout), 0); err != nil {
		return Config{}, err
	}
	if cfg.ReapInterval, err = parseDuration(envReapInterval, raw(envReapInterval), defaultReapInterval); err != nil {
		return Config{}, err
	}
	if cfg.ReadyTimeout, err = parseDuration(envReadyTimeout, raw(envReadyTimeout), backend.ReadyTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadyTimeout <= 0 {
		return Config{}, fmt.Errorf("%s=%q: a sandbox given no time to answer %s can never start", envReadyTimeout, raw(envReadyTimeout), backend.HealthPath)
	}
	if cfg.CPUMillis, err = parsePositiveInt(envSessionCPUMillis, raw(envSessionCPUMillis), backend.DefaultResources.CPUMillis); err != nil {
		return Config{}, err
	}
	if cfg.MemoryMiB, err = parsePositiveInt(envSessionMemoryMiB, raw(envSessionMemoryMiB), backend.DefaultResources.MemoryMiB); err != nil {
		return Config{}, err
	}

	// Contract §4 gives a 5s minimum without saying how it is enforced.
	// Clamping is also the only reading that does not crash: time.NewTicker
	// panics on a non-positive duration, so 0 cannot reach the reaper.
	if cfg.ReapInterval < minReapInterval {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("%s=%v is below the %v minimum; sweeping every %v instead", envReapInterval, cfg.ReapInterval, minReapInterval, minReapInterval))
		cfg.ReapInterval = minReapInterval
	}

	// Contract §4 makes an unknown log level a warning, not an exit: a typo
	// here must not stop the cockpit from starting.
	if level := raw(envLogLevel); level != "" {
		switch strings.ToLower(level) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("%s=%q is not one of debug, info, warn, error; logging at info", envLogLevel, level))
		}
	}

	// The allowlist REPLACES httpx.DefaultOrigins rather than extending it
	// (contract §4), so a value that parses to nothing is an error: silently
	// restoring the defaults it was set to replace is the opposite of what was
	// asked for.
	if origins := raw(envAllowedOrigins); origins != "" {
		for _, part := range strings.Split(origins, ",") {
			if o := strings.TrimSpace(part); o != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
			}
		}
		if len(cfg.AllowedOrigins) == 0 {
			return Config{}, fmt.Errorf("%s=%q contains no origin; unset it to use the defaults", envAllowedOrigins, origins)
		}
	}

	if cfg.HarnessAllowUnverified {
		cfg.Warnings = append(cfg.Warnings, envHarnessAllowUnverified+" is set, but harness.Build as landed in task 8 accepts no such option: unverified harnesses stay unavailable until one is added to images/sandbox/Dockerfile and its flags are checked against --help (contract §8.2)")
	}

	// filepath.Join(StateDir, "sessions") is byte-for-byte what statepath.Dir
	// returns when PANACEA_APP_STATE_DIR is set (contract §2.7), so the two
	// cannot disagree; going through registry.DefaultPath when it is unset
	// keeps the rest of the ladder in the one package that owns it.
	if cfg.StateDir != "" {
		cfg.SessionsPath = filepath.Join(cfg.StateDir, "sessions", "sessions.json")
		cfg.ConversationsDir = filepath.Join(cfg.StateDir, "conversations")
	} else {
		cfg.SessionsPath = registry.DefaultPath()
		cfg.ConversationsDir = statepath.Dir("conversations")
	}
	// The registry is happy with a relative path and a bind mount is not: a
	// relative PANACEA_APP_STATE_DIR would otherwise fail every session create
	// with a Docker error that names neither variable.
	conversations, err := filepath.Abs(cfg.ConversationsDir)
	if err != nil {
		return Config{}, fmt.Errorf("%s: make the conversation store %q absolute: %w", envStateDir, cfg.ConversationsDir, err)
	}
	cfg.ConversationsDir = conversations

	// A pair or nothing. Half a pair has no reading that is not a guess: the
	// other half from the host would stamp commits with two sources' idea of
	// one person, and the half alone fails every commit in a way that names
	// neither variable.
	cfg.GitName, cfg.GitEmail = raw(envGitName), raw(envGitEmail)
	switch {
	case cfg.GitName != "" && cfg.GitEmail == "":
		return Config{}, fmt.Errorf("%s is set but %s is not: set both, or neither to commit as the identity this machine's git uses", envGitName, envGitEmail)
	case cfg.GitName == "" && cfg.GitEmail != "":
		return Config{}, fmt.Errorf("%s is set but %s is not: set both, or neither to commit as the identity this machine's git uses", envGitEmail, envGitName)
	}

	if cfg.CABundle = raw(envCABundle); cfg.CABundle != "" {
		if err := validateCABundle(cfg.CABundle); err != nil {
			return Config{}, err
		}
	}

	if err := validateAddr(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validateAddr enforces contract §4's loopback rule. It runs last so that an
// operator with two problems is told about the parse error first and the bind
// refusal second, rather than fixing the bind and discovering the parse error
// on the next run.
//
// The predicate is httpx.LoopbackHost — the same one task 9's middleware uses on
// every request's Host header — rather than a second list of spellings kept in
// this file. Two loopback predicates in one binary is the shape that drifts, and
// the drift would be silent: the bind would be refused for an address the
// running guard then accepted, or the reverse. LoopbackHost takes a HOST HEADER
// and not an address, so the port comes off first.
//
// That predicate is one step wider than contract §4's literal three spellings:
// it accepts the whole 127.0.0.0/8 range, so PANACEA_APP_ADDR=127.0.0.2:8730
// binds where §4's text alone would refuse. The widening is deliberate and it is
// in the safe direction — every address it adds is loopback, which no other host
// on the network can route to — and it buys one predicate instead of two.
func validateAddr(cfg *Config) error {
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return fmt.Errorf("%s=%q is not host:port: %w", envAddr, cfg.Addr, err)
	}
	if port == "" {
		return fmt.Errorf("%s=%q has no port", envAddr, cfg.Addr)
	}
	// An empty host is not "the default" — it is every interface — and
	// LoopbackHost refuses it, which is why ":8730" reaches the refusal below.
	if cfg.AllowPublicBind || httpx.LoopbackHost(host) {
		return nil
	}
	return fmt.Errorf("refusing to bind %s: a sandbox attach socket on a non-loopback address is a shell on the LAN. Set %s=1 to override.", cfg.Addr, envAllowPublicBind)
}

// validateCABundle is the startup check on PANACEA_APP_CA_BUNDLE, and it
// refuses rather than warns. A bundle that is missing or holds no certificate
// is mounted into every sandbox anyway, Node ignores it with a warning nobody
// sees, and the user's first sign is Claude Code failing its connection check
// in a terminal — far from the variable that caused it.
//
// "At least one certificate" means a CERTIFICATE block x509 can parse, not any
// PEM block: a private key or a CSR is valid PEM and trusts nothing.
//
// Each refusal wraps one of the errCABundle* values below, so a test asks
// errors.Is which refusal it got. A test that matched text could not tell a
// refusal from the generic "cannot be read" fallback on Linux, whose errno
// strings are "no such file or directory" and "is a directory".
func validateCABundle(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s=%q %w", envCABundle, path, errCABundleNotAbsolute)
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s=%q: %w", envCABundle, path, errCABundleMissing)
	case err != nil:
		return fmt.Errorf("%s=%q cannot be read: %w", envCABundle, path, err)
	case info.IsDir():
		return fmt.Errorf("%s=%q %w", envCABundle, path, errCABundleIsADirectory)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s=%q cannot be read: %w", envCABundle, path, err)
	}
	for rest := data; ; {
		var block *pem.Block
		if block, rest = pem.Decode(rest); block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s=%q %w", envCABundle, path, errCABundleNoCertificate)
}

// The four refusals of validateCABundle, each the rest of the sentence after
// the variable and its value.
var (
	errCABundleNotAbsolute   = errors.New("is not an absolute path; it is mounted into every sandbox, so give the full path to the PEM file")
	errCABundleMissing       = errors.New("no such file. Export your network's root certificate as PEM to that path (contract §4 names the command), or unset the variable")
	errCABundleIsADirectory  = errors.New("is a directory; it must name one PEM file")
	errCABundleNoCertificate = errors.New("holds no PEM certificate: it needs at least one -----BEGIN CERTIFICATE----- block (a binary .cer export is DER and must be converted), or unset the variable")
)

// parseFlag implements contract §4's "any value other than 1 or true is also
// treated as unset", and it means ANY other value: "TRUE" is not "true".
// PANACEA_APP_ALLOW_PUBLIC_BIND is the one variable that disarms a security
// rule, so this comparison is exact rather than EqualFold — a case-insensitive
// widening is a change to a locked document, and the refusal message names the
// spelling that works.
func parseFlag(v string) bool {
	return v == "1" || v == "true"
}

func parseDuration(name, value string, def time.Duration) (time.Duration, error) {
	if value == "" {
		return def, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a Go duration such as 90s or 5m: %w", name, value, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s=%q is negative", name, value)
	}
	return d, nil
}

func parsePositiveInt(name, value string, def int) (int, error) {
	if value == "" {
		return def, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a whole number: %w", name, value, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero", name, value)
	}
	return n, nil
}
