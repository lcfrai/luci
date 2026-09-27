package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// exitLinger is how long the daemon stays alive after the process it runs has
// ended, so the Exit frame fanOut writes when the hub closes has time to leave.
// Two seconds because it is a loopback write of a few bytes to sockets that are
// already open — microseconds of work — and the only cost of being generous is
// two seconds of a container that is on its way out anyway. The only cost of
// being tight is the frame never arriving, which is a cockpit that never learns
// why its session ended.
const exitLinger = 2 * time.Second

// sandboxServer is what the entrypoint needs from the session: something to
// serve, and a channel that closes when the one process this container exists
// to run has ended.
//
// The second is F3's fix. main used to hold only the http.Handler, which erases
// the server, so the daemon had no way to know its process was gone and served
// until something killed it: a shell that ran `exit` left sandboxd as PID 1 in a
// container still Up, a session still `running`, and every attach getting an
// Exit frame and an immediate close that the cockpit answered by reconnecting
// (docs/verification/2026-09-27-phase-2-results.md, finding F3).
type sandboxServer interface {
	http.Handler
	Done() <-chan struct{}
	ExitReason() string
}

// serveSession serves until the listener fails or the session's process ends,
// and returns nil for the second. It takes functions rather than an
// *http.Server so the decision can be tested on a machine with no PTY — which
// is every Windows development machine, where the server package does not even
// build.
func serveSession(serve func() error, shutdown func(context.Context) error,
	processDone <-chan struct{}, linger time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- serve() }()

	select {
	case err := <-served:
		return err
	case <-processDone:
	}

	// Stop accepting first. A cockpit whose attach ladder has not noticed yet
	// gets a refused connection, which it backs off from, rather than a fresh
	// /attach that opens and closes at once — F3's storm was 634 of those.
	//
	// Shutdown returns as soon as the listener is closed: every /attach is a
	// hijacked connection, and net/http neither tracks nor waits for those. The
	// linger below is therefore the whole of the time fanOut has to put its Exit
	// frame on the wire before the process that writes it goes away.
	//
	// ONE deadline, not two timers. Shutdown used to get a linger-long context
	// and then the wait below started a fresh linger, so a /healthz in flight —
	// the one non-hijacked request that makes Shutdown block — cost 2 x linger,
	// twice what exitLinger's own comment promises and twice what the container
	// stop timeout was sized against. The deadline is computed once and the wait
	// takes what is left of it, so the post-exit window is exactly linger no
	// matter how long Shutdown takes.
	deadline := time.Now().Add(linger)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		slog.Warn("the listener did not shut down cleanly", "err", err)
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	<-timer.C
	return nil
}

// parseArgs resolves the process arguments. argsJSON wins when present: the
// CSV form cannot carry an opening prompt containing a comma, which is the
// common case the moment a user types one.
//
// A JSON "null" is rejected rather than quietly treated as no arguments.
// json.Unmarshal accepts it into a []string without error, leaving the slice
// nil, so a caller that meant to pass arguments and serialised them wrongly
// would get a silently argument-less agent. An empty array is the way to say
// "no arguments", and it still parses.
func parseArgs(argsCSV, argsJSON string) ([]string, error) {
	if strings.TrimSpace(argsJSON) != "" {
		var out []string
		if err := json.Unmarshal([]byte(argsJSON), &out); err != nil {
			return nil, fmt.Errorf("args-json must be a JSON array of strings: %w", err)
		}
		if out == nil {
			return nil, fmt.Errorf("args-json must be a JSON array of strings, got %q", argsJSON)
		}
		return out, nil
	}
	if argsCSV == "" {
		return nil, nil
	}
	return strings.Split(argsCSV, ","), nil
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	command := flag.String("command", "/bin/bash", "command to run in the PTY")
	argsCSV := flag.String("args", "", "comma-separated arguments (legacy; prefer -args-json)")
	argsJSON := flag.String("args-json", "", "arguments as a JSON array of strings")
	token := flag.String("attach-token", os.Getenv("PANACEA_APP_ATTACH_TOKEN"),
		"shared secret required on /attach")
	allowUnauthenticated := flag.Bool("allow-unauthenticated", false,
		"run without an attach token; local development only")
	flag.Parse()

	argv, err := parseArgs(*argsCSV, *argsJSON)
	if err != nil {
		slog.Error("bad arguments", "err", err)
		os.Exit(2)
	}

	srv, err := newServer(*command, argv, *token, *allowUnauthenticated)
	if err != nil {
		slog.Error("could not start the session", "err", err)
		os.Exit(1)
	}

	// args_count, not args: argv routinely carries the user's opening prompt,
	// which has no business in container logs.
	slog.Info("sandboxd listening", "addr", *addr, "command", *command, "args_count", len(argv),
		"authenticated", *token != "", "allow_unauthenticated", *allowUnauthenticated)
	httpSrv := &http.Server{Addr: *addr, Handler: srv}
	if err := serveSession(httpSrv.ListenAndServe, httpSrv.Shutdown, srv.Done(), exitLinger); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
	// The process this sandbox exists to run has ended, so the daemon ends with
	// it: PID 1 returning is what stops the container, and a stopped container
	// is how the control plane learns the session exited (Go contract §5.3
	// rule 3). Nothing else in the phase tells it.
	slog.Info("the session's process ended; sandboxd is exiting", "reason", srv.ExitReason())
}
