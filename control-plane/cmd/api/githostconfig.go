package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// hostGit is how the resolvers ask the HOST's git about the HOST's own config
// and nothing else (phase 3 ruling R4 rule 6; X-1). A worktree's .git/config
// is in its sandbox's mount and the agent writes it, so a question asked there
// lets the agent choose the answer: core.excludesFile naming an SSH key was
// the answer the session then mounted. So every git here runs in dir, a
// directory this control plane made for the purpose and nothing else writes,
// outside every repository, and never with -C, --git-dir or --work-tree. A
// config read also names its scope, --global or --system, so a repository's
// own scope is not read even if one were found.
type hostGit struct {
	run    project.Runner
	dir    string
	getenv func(string) string
}

// newHostGit makes the directory host git runs in, and the runner that keeps
// git's repository search inside it: GIT_CEILING_DIRECTORIES is its parent,
// so git looks for a repository in dir alone, and dir is empty. The cleanup
// removes the directory; serve() defers it.
func newHostGit() (hostGit, func(), error) {
	dir, err := os.MkdirTemp("", "panacea-app-hostgit-")
	if err != nil {
		return hostGit{}, nil, fmt.Errorf("make the directory host git runs in: %w", err)
	}
	run := &project.ExecRunner{Timeout: project.GitTimeout, Env: hostGitEnv(dir)}
	return hostGit{run: run, dir: dir, getenv: os.Getenv}, func() { _ = os.RemoveAll(dir) }, nil
}

// hostGitEnv is what a host-config git gets on top of this process's own
// environment.
func hostGitEnv(dir string) []string {
	return []string{"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir)}
}

// hostGitUnset is the --default the scope probe passes: a value nobody sets,
// so reading it back means the global scope does not set the key.
const hostGitUnset = "(panacea-app: not set in the global scope)"

// config runs `git config --get <scope> --includes <args...>` in h.dir and
// returns its one line. --includes because naming a scope turns include.path
// off, and the host's git follows it.
func (h hostGit) config(ctx context.Context, scope string, args ...string) (string, error) {
	out, err := h.run.Run(ctx, h.dir, append([]string{"config", "--get", scope, "--includes"}, args...)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// scope is the host-owned scope git takes key from, in git's own order: the
// global scope when it sets key, because git reads it after the system scope;
// else the system scope. The global scope answers too when git is told to skip
// the system file, and then its --default is the answer.
func (h hostGit) scope(ctx context.Context, key string) (string, error) {
	probe, err := h.config(ctx, "--global", "--default", hostGitUnset, key)
	if err != nil {
		return "", err
	}
	if probe != hostGitUnset || h.noSystem() {
		return "--global", nil
	}
	return "--system", nil
}

// noSystem is GIT_CONFIG_NOSYSTEM as git reads it: a boolean, where the empty
// string is false. git config --system reads the system file regardless, so
// it is honoured here. A value git refuses is read as false.
func (h hostGit) noSystem() bool {
	v := strings.ToLower(strings.TrimSpace(h.getenv("GIT_CONFIG_NOSYSTEM")))
	switch v {
	case "true", "yes", "on":
		return true
	}
	n, err := strconv.Atoi(v)
	return err == nil && n != 0
}

// value is key as the host's own config sets it: `git config --get` with
// args (--type, --default) in the scope git takes it from.
func (h hostGit) value(ctx context.Context, key string, args ...string) (string, error) {
	scope, err := h.scope(ctx, key)
	if err != nil {
		return "", err
	}
	return h.config(ctx, scope, append(args, key)...)
}
