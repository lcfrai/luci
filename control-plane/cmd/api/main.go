// Command api is the panacea-app control plane: one HTTP surface on loopback,
// one JSON registry on disk, one backend, one reaper. It owns no logic of its
// own beyond wiring, configuration and contract §5.5's restart recovery.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	// k8s.io/client-go/kubernetes and .../rest and nothing else from that
	// module. k8s.io/client-go/tools/clientcmd — the package that reads a
	// kubeconfig FILE — is deliberately absent: it pulls github.com/imdario/mergo
	// and github.com/spf13/pflag, and mergo is in neither go.mod nor go.sum, so
	// importing it makes every GOPROXY=off command fail with "updates to go.mod
	// needed". Group C ruling 8: no new module enters go.mod in this task. See
	// kubeClient below for what that costs.
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

func main() { os.Exit(run(os.Getenv, os.Stderr)) }

// run is main with its two impure edges as parameters, so that the exit-2 path
// is a unit test rather than a process. Exit codes: 2 is a configuration
// refusal the operator must fix, 1 is a runtime failure, 0 is a clean signal.
func run(getenv func(string) string, errOut io.Writer) int {
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintf(errOut, "panacea-app: %v\n", err)
		return 2
	}
	log := slog.New(slog.NewTextHandler(errOut, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logWarnings(log, cfg.Warnings)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := serve(ctx, cfg, log); err != nil {
		log.Error("control plane stopped", "err", err)
		return 1
	}
	return 0
}

func serve(ctx context.Context, cfg Config, log *slog.Logger) error {
	be, err := newBackend(cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = be.Close() }()

	// statepath.Dir creates nothing on purpose (contract §2.7) — the caller
	// knows whether a missing directory is an error. Here it is not.
	if err := os.MkdirAll(filepath.Dir(cfg.SessionsPath), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	store, err := registry.NewFileStore(cfg.SessionsPath)
	if err != nil {
		return fmt.Errorf("open session registry %s: %w", cfg.SessionsPath, err)
	}

	// Contract §5.5, once, before anything can serve a request from a stale
	// endpoint.
	recoverSessions(ctx, store, be, log)

	rp := reaper.New(reaperConfigFor(cfg, store, be, log))
	// The reaper gets its own context rather than ctx, so that a SIGINT drains
	// the HTTP server while the reaper is still sweeping a live backend; ctx is
	// already cancelled by the time Shutdown is called, and a reaper on ctx
	// would have stopped at the signal.
	reapCtx, stopReaper := context.WithCancel(context.Background())
	reaperDone := make(chan struct{})
	go func() {
		defer close(reaperDone)
		rp.Run(reapCtx)
	}()
	// Registered AFTER the backend's Close and therefore run BEFORE it: defers
	// unwind last-in-first-out, and be.Close() under an in-flight sweep is a
	// Destroy against a closed Docker client. This one defer covers every exit
	// from here down, including the serve-error path that returns without
	// reaching the shutdown block.
	defer func() {
		stopReaper()
		<-reaperDone
	}()

	gitRunner := project.NewExecRunner()
	scanner := project.NewScanner(cfg.ProjectsDir, gitRunner)
	// Contract §4: with no projects directory the provider is worktree.Static{},
	// so every session create fails 404 worktree_not_found. That is correct and
	// visible — no projects directory means no worktrees to work in.
	var worktrees worktree.Provider = worktree.Static{}
	if cfg.ProjectsDir != "" {
		worktrees = scanner
	}

	// Asked once here as well as at every create, so the user without an
	// identity hears it at startup, before any agent tries to commit. Asked
	// outside any worktree, this is the machine-wide answer; a repository that
	// sets its own identity is still asked at its own sessions' creates.
	gitIdentity := gitIdentityResolver(cfg.GitName, cfg.GitEmail, gitRunner)
	if _, err := gitIdentity(ctx, cfg.ProjectsDir); err != nil {
		log.Warn("no git identity", "err", err)
	}

	// api.New returns (*API, error): it validates every dependency it cannot
	// default and names the nil one, because a wiring mistake should stop the
	// process here with a sentence rather than panic on the first request.
	// There is no IdleAfter field to pass — the idle window belongs to the
	// reaper above, which is the only thing that derives idle.
	handler, err := api.New(api.Deps{
		Store:        store,
		Backend:      be,
		Worktrees:    worktrees,
		Projects:     scanner,
		Flags:        store, // registry.WorktreeFlagStore, implemented by *FileStore
		Origin:       httpx.NewOriginPolicy(cfg.AllowedOrigins, log),
		Image:        cfg.SandboxImage,
		Resources:    cfg.Resources(),
		ReadyTimeout: cfg.ReadyTimeout,
		BackendReady: rp.BackendReady,
		Log:          log,
		// Created per worktree, at the first session that needs it.
		ConversationsDir: cfg.ConversationsDir,
		GitIdentity:      gitIdentity,
		CABundle:         cfg.CABundle,
		// Interim until phase 4 task 14 wires contextwrite and deletes it.
		Materialiser: newMaterialiserUntilTask14(log),
	})
	if err != nil {
		return fmt.Errorf("build the HTTP surface: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}

	srv := &http.Server{
		Handler: handler,
		// ReadHeaderTimeout and nothing else. A ReadTimeout or a WriteTimeout
		// would kill /attach: the relay is a WebSocket that is idle for minutes
		// and then streams for an hour, and both timeouts apply to it.
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("control plane listening",
		"addr", ln.Addr().String(),
		"backend", be.Kind(),
		"sessions", cfg.SessionsPath,
		"conversations", cfg.ConversationsDir,
		"projects", cfg.ProjectsDir,
		"image", cfg.SandboxImage,
		"caBundle", cfg.CABundle,
	)

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Shutdown does not close hijacked connections, and coder/websocket hijacks
	// every /attach. Those relays end when the process does; the sandbox and its
	// PTY survive (Group A task 3), so the desktop reattaches with scrollback.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("graceful shutdown did not finish in time; closing what is left", "err", err)
		_ = srv.Close()
	}
	// The reaper is stopped and drained by the deferred function above, after
	// the HTTP server has finished and before the backend is closed.
	return nil
}

// reaperConfigFor is the Config-to-reaper.Config mapping, extracted from serve
// so that a test can see it. serve itself cannot be unit-tested — it builds a
// backend, and dockerbe.New pings a real daemon inside New — which would
// otherwise leave the one line below that is a decision rather than a copy
// asserted by nothing at all.
//
// CreateGrace is DERIVED from the ready timeout and is deliberately longer than
// it: reaper.CreateGraceFor owns the reasoning, and going through it means this
// call site and the reaper's own default cannot drift into disagreeing about how
// long a create may take. Passing cfg.ReadyTimeout here instead would compile,
// run, and quietly shorten the window that stops a mid-create session being
// marked exited.
func reaperConfigFor(cfg Config, store registry.Store, be backend.Backend, log *slog.Logger) reaper.Config {
	return reaper.Config{
		Store:       store,
		Backend:     be,
		IdleAfter:   cfg.IdleAfter,
		IdleTimeout: cfg.IdleTimeout,
		Interval:    cfg.ReapInterval,
		CreateGrace: reaper.CreateGraceFor(cfg.ReadyTimeout),
		Log:         log,
	}
}

func newBackend(cfg Config, log *slog.Logger) (backend.Backend, error) {
	switch cfg.Backend {
	case string(backend.KindDocker):
		// BindIP is deliberately not configurable: dockerbe defaults it to
		// 127.0.0.1 and refuses anything else, and there is no safe non-loopback
		// value to expose as an environment variable.
		return dockerbe.New(dockerbe.Config{
			Host:         cfg.DockerHost,
			ReadyTimeout: cfg.ReadyTimeout,
			Log:          log,
		})
	case string(backend.KindKata):
		client, err := kubeClient(cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kata backend: %w", err)
		}
		return katabe.New(katabe.Config{
			Manager:      sandbox.NewManager(client, cfg.Namespace),
			ReadyTimeout: cfg.ReadyTimeout,
		})
	default:
		// loadConfig already refused anything else; this arm exists so the
		// switch is total, not because a user can reach it.
		return nil, fmt.Errorf("unknown backend %q", cfg.Backend)
	}
}

// kubeClient builds a Kubernetes client for the Kata path from the in-cluster
// service account, and refuses PANACEA_APP_KUBECONFIG rather than honouring it.
//
// Reading a kubeconfig FILE needs k8s.io/client-go/tools/clientcmd, which drags
// github.com/imdario/mergo and github.com/spf13/pflag into go.mod — modules no
// other package here needs and that go.sum does not pin. Group C ruling 8 keeps
// them out: Kata is dormant in Phase 2, the Docker backend is what this task
// wires, and the real Kata deployment runs inside the cluster, where the
// service account is the credential anyway. So the variable is refused with a
// sentence naming what would have to change, which is honest, rather than
// ignored, which is not. loadConfig still parses it into Config.Kubeconfig for
// exactly that reason: the value has to reach here for the error to name it.
// Phase 8 turns Kata on and adds the module then, with contract §8.5's pinning
// review it deserves.
//
// This is the REFUSAL half of ruling 1, and it is deliberately louder than the
// warning half in loadConfig: set under docker the same variable earns one
// warning line and the process starts, because there it merely has no effect.
// Here it is a request this build cannot satisfy.
func kubeClient(kubeconfig string) (*kubernetes.Clientset, error) {
	if kubeconfig != "" {
		return nil, fmt.Errorf("%s is set, but Phase 2 reads Kubernetes credentials from the in-cluster service account only — a kubeconfig file needs k8s.io/client-go/tools/clientcmd, which this phase does not add to go.mod: %w", envKubeconfig, backend.ErrUnsupported)
	}
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		// Outside a cluster this is the expected state of every developer
		// laptop, so it is ErrUnavailable and not a panic.
		return nil, fmt.Errorf("%w: %v", backend.ErrUnavailable, err)
	}
	return kubernetes.NewForConfig(restCfg)
}

// recoverSessions is contract §5.5. It runs once, before the listener, and it
// is not part of the reaper on purpose: the reaper converges a system that is
// running, this reconciles one that was not. Orphans are left to the reaper's
// first sweep, which starts immediately.
func recoverSessions(ctx context.Context, store registry.Store, be backend.Backend, log *slog.Logger) {
	sessions, err := store.List()
	if err != nil {
		log.Warn("could not read the session registry at startup", "err", err)
		return
	}
	if len(sessions) == 0 {
		return
	}
	live, err := be.List(ctx)
	if err != nil {
		// THE failure mode this rule exists to prevent: marking everything
		// exited because Docker Desktop had not finished starting loses a
		// night's work, and exited is terminal (contract §5.6).
		log.Warn("backend did not answer at startup; every session is left exactly as it was and the reaper will converge",
			"sessions", len(sessions), "err", err)
		return
	}
	bySandbox := make(map[string]backend.Sandbox, len(live))
	for _, sb := range live {
		bySandbox[sb.ID] = sb
	}

	for _, s := range sessions {
		if s.SandboxID == "" {
			continue // nothing was ever created for this record
		}
		sb, alive := bySandbox[s.SandboxID]
		if !alive {
			if s.Status == registry.StatusExited {
				clearStaleAttachCount(store, log, s)
				continue
			}
			applyUpdate(store, log, s.ID, func(rec *registry.Session) {
				rec.Status = registry.StatusExited
				rec.ExitReason = "sandbox disappeared while the app was not running"
				rec.AttachCount = 0
			})
			continue
		}
		if s.Status == registry.StatusExited {
			// §5.6 has no transition out of exited, so the record stays
			// terminal — and the container is not destroyed here either, because
			// destroying at boot is the irreversible choice. DELETE
			// /v1/sessions/{id}?purge=1 owns that. The consequence is a
			// container the reaper will not collect while the record exists;
			// this line is the only warning anyone gets about it.
			log.Warn("a finished session still has a live sandbox; delete the session with purge to remove it",
				"session", s.ID, "sandbox", s.SandboxID, "state", sb.State)
			clearStaleAttachCount(store, log, s)
			continue
		}
		if s.Endpoint != sb.Endpoint {
			// Logged out here rather than inside the mutate function: the store
			// holds its write lock while that runs.
			log.Info("sandbox endpoint moved while the app was not running",
				"session", s.ID, "was", s.Endpoint, "now", sb.Endpoint)
		}
		applyUpdate(store, log, s.ID, func(rec *registry.Session) {
			// Overwritten unconditionally, including with "": a NonLoopback
			// sandbox reports an empty endpoint (task 5), and keeping the old
			// one would let a client attach to something on the LAN.
			rec.Endpoint = sb.Endpoint
			rec.SandboxRef = sb.Ref
			// AttachCount is a count of live WebSockets and every one of them
			// died with the previous process. Task 12's relay defer is what
			// would have decremented it, and that goroutine died too, so this
			// boot sweep is the only thing that can repair the field — nothing
			// else in the phase can tell a stale 3 from a real one.
			rec.AttachCount = 0
			switch sb.State {
			case backend.StateRunning:
				// The reaper re-derives idle on its first sweep.
				rec.Status = registry.StatusRunning
			case backend.StateStopped, backend.StateGone:
				rec.Status = registry.StatusExited
				rec.ExitReason = "process exited"
			case backend.StateStarting:
				// The one row this task adds to contract §5.5 step 3, which
				// names only StateRunning and StateStopped. A sandbox that is
				// still starting is neither: the control plane went down
				// between Create and the first /healthz answer. Reporting
				// running would fabricate a status, exited is terminal, and
				// starting is simply true. The reaper re-derives it within one
				// interval.
				rec.Status = registry.StatusStarting
			}
		})
	}
}

// clearStaleAttachCount repairs the one field nothing else can, on the two
// recovery branches that otherwise leave the record untouched: a session that
// was already exited. Both return early — §5.6 has no transition out of exited
// and this must not invent one — but AttachCount is not a status. It counts live
// WebSockets, all of which died with the previous process along with the relay
// defer that would have decremented them, so a session that was exited with a
// client attached would otherwise show that client as connected for ever.
//
// Guarded on the current value rather than written unconditionally: an Update is
// a full rewrite of the registry document under the write lock, and a laptop
// with fifty finished sessions would pay fifty of them at every boot to change
// nothing.
func clearStaleAttachCount(store registry.Store, log *slog.Logger, s registry.Session) {
	if s.AttachCount == 0 {
		return
	}
	applyUpdate(store, log, s.ID, func(rec *registry.Session) {
		rec.AttachCount = 0
	})
}

func applyUpdate(store registry.Store, log *slog.Logger, id string, apply func(*registry.Session)) {
	if _, err := store.Update(id, func(s *registry.Session) error {
		apply(s)
		return nil
	}); err != nil {
		log.Warn("could not update a session during recovery", "session", id, "err", err)
	}
}
