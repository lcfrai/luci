package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPlatformRunnerDiagnosticHelper(t *testing.T) {
	for n, arg := range os.Args {
		if arg != "--platform-diagnostic-fixture" || n+1 >= len(os.Args) {
			continue
		}
		messages := map[string]string{"registry": "unauthorized: Bearer private-test-credential must never be exposed", "permission": "permission denied: private-test-credential", "socket": "Cannot connect to the Docker daemon at private-test-credential", "compose": "invalid compose configuration: private-test-credential", "manifest": "manifest unknown: private-test-credential"}
		fmt.Fprintln(os.Stderr, messages[os.Args[n+1]])
		os.Exit(42)
	}
}
func TestPlatformRunnerFailureClassifiesWithoutExposingStderr(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for mode, kind := range map[string]string{"registry": "registry-access", "permission": "permission", "socket": "docker-connection", "compose": "compose-configuration", "manifest": "registry-manifest"} {
		_, err := commandRunner("")(context.Background(), exe, []string{"-test.run=^TestPlatformRunnerDiagnosticHelper$", "--", "--platform-diagnostic-fixture", mode}, t.TempDir(), nil)
		var failure *platformCommandFailure
		if !errors.As(err, &failure) || failure.kind != kind || failure.exitCode != 42 || strings.Contains(err.Error(), "private-test-credential") || strings.Contains(dataError(err), "private-test-credential") {
			t.Fatalf("unsafe/wrong diagnostic for %s: %v", mode, err)
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, e = commandRunner("")(ctx, exe, nil, t.TempDir(), nil)
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("command deadline cause was lost")
	}
	_, e = commandRunner("")(context.Background(), "luci-missing-command-fixture-702d", nil, t.TempDir(), nil)
	var failure *platformCommandFailure
	if !errors.As(e, &failure) || failure.kind != "start-failed" || failure.processStatus != "not-started" {
		t.Fatal("spawn failure was not distinguished")
	}
}
