package project_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// ExecRunner.Env wins over this process's own environment. cmd/api's
// host-config runner relies on it: its GIT_CEILING_DIRECTORIES must hold even
// when the control plane was started with one of its own (X-1). The variable
// observed here is command-scope config, because git prints it back.
func TestExecRunnerEnvWinsOverTheProcesssOwn(t *testing.T) {
	requireGit(t)
	cfg := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(cfg, "gitconfig"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "luci.test")
	t.Setenv("GIT_CONFIG_VALUE_0", "from-the-process")

	run := &project.ExecRunner{Env: []string{"GIT_CONFIG_VALUE_0=from-the-runner"}}
	out, err := run.Run(context.Background(), cfg, "config", "--get", "luci.test")
	if err != nil {
		t.Fatalf("git config: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "from-the-runner" {
		t.Errorf("git saw %q, want the runner's own entry %q", got, "from-the-runner")
	}

	// Nil adds nothing: the process's own value stands.
	out, err = (&project.ExecRunner{}).Run(context.Background(), cfg, "config", "--get", "luci.test")
	if got := strings.TrimSpace(string(out)); err != nil || got != "from-the-process" {
		t.Errorf("with no Env git saw %q, %v; want the process's own %q", got, err, "from-the-process")
	}
}
