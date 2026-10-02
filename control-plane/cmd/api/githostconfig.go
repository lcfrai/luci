package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
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
const hostGitPattern = "luci-hostgit-"

// newHostGit is the hostGit serve() wires: the OS temp directory, made
// absolute because git ignores a relative ceiling, and the runner that keeps
// git's repository search inside the directory each git is given.
func newHostGit() (hostGit, error) {
	root, err := filepath.Abs(os.TempDir())
	if err != nil {
		return hostGit{}, fmt.Errorf("find the directory host git runs in: %w", err)
	}
	run := &project.ExecRunner{Timeout: project.HostConfigTimeout, Env: hostGitEnv(root)}
	return hostGit{run: run, root: root, getenv: os.Getenv}, nil
}

// hostConfigReadBudget bounds a WHOLE host-config read — every git one
// resolver runs — and it is a DIFFERENT decision from project.HostConfigTimeout,
// which bounds ONE git. Review C-1: they were the same number, so the four
// gits of a line-endings read shared 30 s and got 7.5 s each, below the 10 s
// F2 proved too short. Six concurrent creates then turned a test the
// pre-change bounds pass 3/3 into a failure — the change made the thing F2
// complained about MORE likely, not less.
//
// Sized from the READ, and from two numbers it has to sit between:
//
//   - Above: before there was a budget the same read had 4 × 10 s, 5 × 10 s on
//     its raw-autocrlf fallback. 60 s is more wall clock than it ever had, so
//     no slow-but-succeeding read that used to finish now fails, and one git
//     may stall its full 30 s with the 30 s the rest of the read used to have
//     still in hand.
//   - Below: it is NOT the arithmetic 5 × project.HostConfigTimeout (150 s),
//     because POST /v1/sessions runs these reads in SEQUENCE — line endings,
//     then global files, then the identity — and the cockpit abandons a create
//     at 150 s in total (desktop/src/api/client.ts §4.8). 60 + 60 +
//     hostIdentityReadBudget = 130 s, which leaves the create the rest.
const hostConfigReadBudget = 60 * time.Second

// hostIdentityReadBudget bounds the WHOLE identity read and is deliberately
// NOT the budget above. Review I-3: the identity read's failure is not fatal —
// internal/api's sessionGitIdentity logs a warning and starts the session with
// an empty identity — so a stalled `git var GIT_AUTHOR_IDENT` must not make
// the user wait the budget that exists because a failure destroys the create.
// It is one git, its answer is thrown away on failure, and the same resolver
// runs at startup before api.New (main.go), where it delays the control plane
// coming up. So it gets the shorter wall clock, and inherits nothing from a
// constant's name.
const hostIdentityReadBudget = 10 * time.Second

// read runs one host-config read under its own budget, and says whose deadline
// ended it when one did. Review M-2: from project.ExecRunner's side this
// budget IS the caller, so a spent budget and an HTTP request that was already
// over produced the same sentence, and only GitError.Elapsed told them apart,
// by inference. The distinction is made here, where it is knowable — the
// budget's context is done and the request's is not — because F2's whole
// complaint was an operator who could not tell a stalled host from anything
// else.
func (h hostGit) read(ctx context.Context, budget time.Duration, read func(context.Context) error) error {
	inner, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	err := read(inner)
	if err != nil && ctx.Err() == nil && inner.Err() != nil {
		return fmt.Errorf("this host-config read spent its whole %s budget: %w", budget, err)
	}
	return err
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
const hostGitUnset = "(luci: not set in the global scope)"

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

// valueWithOrigin is key's value in scope AND the config file git took it from,
// read together with --show-origin so a value an [include] pulled in is paired
// with the file that actually set it, not the top-level file (X-3 review 3 I-C).
// --includes is on for the same reason config() sets it: naming a scope turns
// include.path off, and the host's git follows it. An unset key answered by
// --default has the origin "command line", which parseShowOrigin reports as no
// file.
//
// -z (X-3 review 4 I-C): without it git C-quotes an origin name holding a
// backslash, a double quote, a control byte or a non-ASCII byte — a Windows
// path, an include spelled `sub\x.inc`, a projects directory under a name like
// Zoë — and a quoted name resolves to nothing, so the provenance walk never
// ran. With -z git writes the name raw and ends each field with a NUL, which
// no path and no config value can hold, so there is no quoting to undo.
func (h hostGit) valueWithOrigin(ctx context.Context, scope, key string, args ...string) (value, origin string, err error) {
	full := append([]string{"config", "-z", "--show-origin", "--get", scope, "--includes"}, args...)
	out, err := h.git(ctx, append(full, key)...)
	if err != nil {
		return "", "", err
	}
	return parseShowOrigin(string(out))
}

// parseShowOrigin reads `git config -z --show-origin --get` output: exactly one
// record, `<origin type>:<origin name>` NUL `<value>` NUL (git's own format,
// measured on git 2.53.0.windows.2), the value byte for byte. A "file" origin
// returns its name; --default's answer, the origin "command line" with no name,
// returns "" because no file set it. It FAILS CLOSED: any other shape or origin
// type (a blob, standard input, a nameless file, a second record, the unquoted
// form git prints without -z) is an error, and the error fails the create. An
// origin this parse cannot name is an origin no walk checked, and under X-3's
// premise the file behind it is the agent's.
func parseShowOrigin(out string) (value, origin string, err error) {
	fields := strings.Split(out, "\x00")
	if len(fields) != 3 || fields[2] != "" {
		return "", "", fmt.Errorf("git config -z --show-origin answered %q, which is not one <origin> NUL <value> NUL record, so where the value came from is not known", out)
	}
	src, value := fields[0], fields[1]
	if src == "command line:" {
		return value, "", nil
	}
	if f, ok := strings.CutPrefix(src, "file:"); ok && f != "" {
		return value, f, nil
	}
	return "", "", fmt.Errorf("git config -z --show-origin named the origin %q, which is neither a file nor --default's command line, so where the value came from cannot be checked", src)
}
