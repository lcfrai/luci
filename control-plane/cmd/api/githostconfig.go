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
// the answer the session then mounted. So every git here runs in a directory
// made for that one git (see git) that nothing else writes, outside every
// repository, and never with -C, --git-dir or --work-tree. A config read also
// names its scope, --global or --system, so a repository's own scope is not
// read even if one were found.
type hostGit struct {
	run project.Runner
	// root is the directory each git's own directory is made in, the OS temp
	// directory as newHostGit found it. The runner's GIT_CEILING_DIRECTORIES
	// is root, so git looks for a repository in that one directory alone.
	root   string
	getenv func(string) string
}

// hostGitPattern is os.MkdirTemp's pattern for the directory one git runs in.
const hostGitPattern = "panacea-app-hostgit-"

// newHostGit is the hostGit serve() wires: the OS temp directory, made
// absolute because git ignores a relative ceiling, and the runner that keeps
// git's repository search inside the directory each git is given.
func newHostGit() (hostGit, error) {
	root, err := filepath.Abs(os.TempDir())
	if err != nil {
		return hostGit{}, fmt.Errorf("find the directory host git runs in: %w", err)
	}
	run := &project.ExecRunner{Timeout: project.GitTimeout, Env: hostGitEnv(root)}
	return hostGit{run: run, root: root, getenv: os.Getenv}, nil
}

// hostGitEnv is what a host-config git gets on top of this process's own
// environment: root as the ceiling, so git stops before root and searches
// only the directory made in it.
func hostGitEnv(root string) []string {
	return []string{"GIT_CEILING_DIRECTORIES=" + filepath.Clean(root)}
}

// git runs one git with args in a directory made for it alone in h.root, and
// removes that directory when the git is done. A directory made once at
// startup would be the OS temp directory's oldest untouched entry, which temp
// reapers remove (systemd-tmpfiles, macOS's /var/folders cleaner, Windows
// Storage Sense), and every create after that would fail its chdir with a 500
// until a restart (X-1 review I-1). os.MkdirTemp makes the directory itself,
// mode 0700, under a name nobody else made first, so nothing but this process
// can have put a .git in it.
func (h hostGit) git(ctx context.Context, args ...string) ([]byte, error) {
	dir, err := os.MkdirTemp(h.root, hostGitPattern)
	if err != nil {
		return nil, fmt.Errorf("make the directory host git runs in: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return h.run.Run(ctx, dir, args...)
}

// hostGitUnset is the --default the scope probe passes: a value nobody sets,
// so reading it back means the global scope does not set the key.
const hostGitUnset = "(panacea-app: not set in the global scope)"

// config runs `git config --get <scope> --includes <args...>` through h.git
// and returns its one line. --includes because naming a scope turns
// include.path off, and the host's git follows it.
func (h hostGit) config(ctx context.Context, scope string, args ...string) (string, error) {
	out, err := h.git(ctx, append([]string{"config", "--get", scope, "--includes"}, args...)...)
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
