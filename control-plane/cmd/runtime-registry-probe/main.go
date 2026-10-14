// runtime-registry-probe makes a read-only request through the selected Docker
// Engine. It never reads credentials, creates tags, pulls or pushes images.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

var sourceCommit string // Set by the clean-source producer build, not the caller.

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var sessionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func validate(endpoint, engine, session string, port int) error {
	// Local supported transports only; never connect to a remote daemon or TLS
	// endpoint and never inherit DOCKER_HOST/auth environment configuration.
	if endpoint != "npipe:////./pipe/dockerDesktopLinuxEngine" && endpoint != "npipe:////./pipe/docker_engine" && endpoint != "unix:///var/run/docker.sock" {
		return errors.New("unsupported local Docker endpoint")
	}
	if engine == "" || len(engine) > 128 || !sessionPattern.MatchString(session) || port < 1024 || port > 65535 {
		return errors.New("invalid engine/session/loopback port")
	}
	return nil
}

func inspect(endpoint, engine, session string, port int) (map[string]any, error) {
	if err := validate(endpoint, engine, session, port); err != nil {
		return nil, err
	}
	cli, err := client.NewClientWithOpts(client.WithHost(endpoint), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.New("local Docker client unavailable")
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	before, err := cli.Info(ctx)
	if err != nil || before.ID != engine || before.OSType != "linux" {
		return nil, errors.New("selected Linux engine identity unavailable or changed")
	}
	ref := fmt.Sprintf("127.0.0.1:%d/luci/runtime-export-%s:payload", port, session)
	started := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = cli.DistributionInspect(ctx, ref, "") // Explicitly no registry auth.
	if !errdefs.IsNotFound(err) {
		return nil, errors.New("daemon distribution inspection did not return not-found")
	}
	after, err := cli.Info(ctx)
	if err != nil || after.ID != engine || after.OSType != "linux" {
		return nil, errors.New("engine changed during daemon distribution inspection")
	}
	return map[string]any{"version": 1, "sourceCommit": sourceCommit, "engineID": engine,
		"reference": ref, "session": session, "startedAt": started,
		"finishedAt": time.Now().UTC().Format(time.RFC3339Nano), "outcome": "not_found",
		"operation": "Engine.DistributionInspect", "registryAuthProvided": false,
		"tagOrPushPerformed": false}, nil
}

func main() {
	identity := flag.Bool("identity", false, "print compiled source identity without network effects")
	endpoint := flag.String("endpoint", "", "selected supported local Docker endpoint")
	engine := flag.String("engine", "", "exact expected engine ID")
	session := flag.String("session", "", "fresh registry session")
	port := flag.Int("port", 0, "literal loopback published port")
	flag.Parse()
	if flag.NArg() != 0 || !commitPattern.MatchString(sourceCommit) {
		fmt.Fprintln(os.Stderr, "clean compiled source identity required")
		os.Exit(1)
	}
	if *identity {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"version": 1, "sourceCommit": sourceCommit})
		return
	}
	result, err := inspect(*endpoint, *engine, *session, *port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error()) // Fixed reasons only; no headers/tokens.
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(result)
}
