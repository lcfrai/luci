package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/mcprelay"
)

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

// mcpListenFlag defines -mcp-listen on fs. Its default is the address the
// harness is given (task 19), so the image's entrypoint passes nothing and
// every sandbox runs the relay; "off" leaves it out.
func mcpListenFlag(fs *flag.FlagSet) *string {
	return fs.String("mcp-listen", mcprelay.ListenAddr, `loopback address for the canvas MCP relay, or "off"`)
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
	mcpListen := mcpListenFlag(flag.CommandLine)
	flag.Parse()

	argv, err := parseArgs(*argsCSV, *argsJSON)
	if err != nil {
		slog.Error("bad arguments", "err", err)
		os.Exit(2)
	}

	srv, err := newServer(*command, argv, *token, *allowUnauthenticated, *mcpListen)
	if err != nil {
		slog.Error("could not start the session", "err", err)
		os.Exit(1)
	}

	// args_count, not args: argv routinely carries the user's opening prompt,
	// which has no business in container logs.
	slog.Info("sandboxd listening", "addr", *addr, "command", *command, "args_count", len(argv),
		"authenticated", *token != "", "allow_unauthenticated", *allowUnauthenticated,
		"mcp_listen", *mcpListen)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
