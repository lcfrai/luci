package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scope is whose repository Call.Dir is, which decides what §HGH H3 refuses. Every repository
// a host call may run in is host-private (ruling R4.1); a lane clone is never one (R4.2).
type Scope int

const (
	Unset   Scope = iota // the zero value: Exec refuses it with ErrCallShape and runs nothing
	Project              // the user's repository, Class Read, LocalWrite or Remote: refusal list
	None                 // `git version`, a Dir under OwnedRoot, or the clone lanes.Create is making (Call.NewLane): no check
)

// Hardening is what Exec adds to every call (§HGH). cmd/api builds it once at startup.
type Hardening struct {
	HooksDir   string   // an empty directory the control plane owns (H1)
	OwnedRoot  string   // the only root a Scope None call may run under
	Credential []string // "key=value", the machine's credential.*helper entries, system then global (H2)
	Proxy      string   // the machine's proxy; "" means none (H2)
	LaneRoot   string   // Machine.CloneRoot(): Exec refuses any call that runs in it (R4.2)
}

// UntrustedConfig is a repository config key §HGH H3 refuses. It carries the key, never the value,
// and shows a URL subsection's userinfo, query and fragment as "***": git writes the subsection into
// the key, so url."https://oauth2:<pat>@host/".insteadOf lists its token there (fix round 1, I-1).
// The API maps it to conflict + reason project_config_untrusted.
type UntrustedConfig struct {
	Dir   string
	Key   string
	Scope Scope
}

func (u *UntrustedConfig) Error() string {
	return "gitx: the repository config in " + u.Dir + " sets " + u.Key +
		", which the cockpit does not run git under; move it to your global config (git config --global)"
}

// ErrCallShape is a Call Exec will not run: Args[0] starting with "-", Scope Unset, a Remote call
// without RemoteURL, a Scope None call outside OwnedRoot whose Dir is not its NewLane, a
// NewLane set on a call that is not Scope None, or a NewLane not strictly under a non-empty
// Hardening.LaneRoot (fix round 1, review M-7).
var ErrCallShape = errors.New("gitx: call shape refused")

// ErrLaneDir is a call Exec refuses before it starts (ruling R4.2): its Dir, a `-C`,
// `--git-dir` or `--work-tree` argument, or a Call.Env GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR,
// GIT_OBJECT_DIRECTORY or GIT_ALTERNATE_OBJECT_DIRECTORIES value lies at or under
// Hardening.LaneRoot (either flag spelling, `=` or next argument; each list entry of the
// alternates value). Each path is made absolute against Dir and cleaned (filepath.Clean) before
// a filepath.Rel compare, which case-folds on Windows. A path that compare admits is still
// refused when it, an ancestor of it, or an ancestor of its filepath.EvalSymlinks form is
// LaneRoot's own directory (os.SameFile), so an 8.3 short name, a trailing-dot segment or a link
// in an ancestor cannot name a lane (fix round 1, review M-6). The one exception is Dir == Call.NewLane
// on a Scope None call. Exec checks it before ErrCallShape, so a lane path in a pinned Env key
// is ErrLaneDir. A positional argument (`fetch <path>`) is not parsed here; task 24's
// TestHostNeverRunsGitInALane pins that no call site passes one. A LaneRoot of "" checks
// nothing. Lane git runs in the sandbox, through task 24's runner, instead.
var ErrLaneDir = errors.New("gitx: host git never runs in a lane")

// prefix is §HGH H1 and H2: command-scope overrides between `git` and Args, on every call.
// Command scope outranks repository config, so no repository key can undo them.
func (x *Exec) prefix(class Class, scope Scope, remoteURL string) []string {
	h := x.Hardening
	p := []string{
		"-c", "core.hooksPath=" + h.HooksDir,
		"-c", "core.fsmonitor=false",
		"-c", "core.sshCommand=",
		"-c", "protocol.allow=never",
		"-c", "protocol.file.allow=always",
		// Fix round 1 (review M-8): git reads protocol.<name>.allow before protocol.allow, so a
		// config's protocol.ext.allow=always beat the default alone. Every named transport the
		// phase does not use is pinned by name too; https is pinned either way.
		"-c", "protocol.ext.allow=never",
		"-c", "protocol.ssh.allow=never",
		"-c", "protocol.git.allow=never",
		"-c", "protocol.http.allow=never",
	}
	if class == Remote {
		p = append(p, "-c", "protocol.https.allow=always")
	} else {
		p = append(p, "-c", "protocol.https.allow=never")
	}
	p = append(p,
		"-c", "core.askPass=",
		"-c", "commit.gpgSign=false",
		"-c", "tag.gpgSign=false",
		// b01-2 N1: a user's log.showSignature=true would run their gpg over an agent-authored
		// lane commit on task 16's `log -1`. No phase 3 --format string uses %G.
		"-c", "log.showSignature=false",
		"-c", "submodule.recurse=false",
		"-c", "fetch.recurseSubmodules=false",
		"-c", "credential.helper=",
	)
	if class == Remote || scope == None {
		for _, kv := range h.Credential {
			p = append(p, "-c", kv)
		}
	}
	if class == Remote {
		p = append(p,
			"-c", "http.sslVerify=true",
			"-c", "http.proxy="+h.Proxy,
			"-c", "http."+remoteURL+".sslVerify=true",
			"-c", "http."+remoteURL+".proxy="+h.Proxy,
		)
	}
	return append(p, "--literal-pathspecs")
}

// packPins puts --upload-pack after fetch and ls-remote and --receive-pack after push, so a
// repository's remote.<name>.uploadpack or .receivepack never runs as a local command.
func packPins(args []string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, args[0])
	switch args[0] {
	case "fetch", "ls-remote":
		out = append(out, "--upload-pack=git-upload-pack")
	case "push":
		out = append(out, "--receive-pack=git-receive-pack")
	}
	return append(out, args[1:]...)
}

// absDir is dir made absolute (against the process's working directory) and cleaned.
func absDir(dir string) string {
	if a, err := filepath.Abs(dir); err == nil {
		return a
	}
	return filepath.Clean(dir)
}

// absAgainst is p made absolute against base, then cleaned. A Windows path rooted without a
// volume (`\x`) takes base's volume, as git would take the working directory's.
func absAgainst(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if filepath.VolumeName(p) == "" && len(p) > 0 && os.IsPathSeparator(p[0]) {
		return filepath.Clean(filepath.VolumeName(base) + p)
	}
	return filepath.Join(base, p)
}

// under is whether p lies at or under root. Both are absolute and clean; filepath.Rel
// case-folds on Windows, and answers an error across volumes.
func under(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func samePath(a, b string) bool {
	rel, err := filepath.Rel(absDir(a), absDir(b))
	return err == nil && rel == "."
}

var laneEnvKeys = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"}

// laneRoot is Hardening.LaneRoot as a spelling and as a location (review M-6).
type laneRoot struct {
	path string      // absolute and clean
	fi   os.FileInfo // nil when the root cannot be stat'ed: the spelling compare alone applies
}

// holds is whether p, absolute and clean, lies at or under the root. The spelling compare comes
// first; a path it admits is still held when p or an ancestor of p, or of p's EvalSymlinks form,
// is the root's own directory. os.Stat follows a link and opens an 8.3 short name or a Win32
// trailing-dot segment as the directory it names, so os.SameFile compares locations.
func (r laneRoot) holds(p string) bool {
	if under(r.path, p) {
		return true
	}
	if r.fi == nil {
		return false
	}
	if r.isAncestorOf(p) {
		return true
	}
	if real, err := filepath.EvalSymlinks(p); err == nil && real != p {
		return under(r.path, real) || r.isAncestorOf(real)
	}
	return false
}

func (r laneRoot) isAncestorOf(p string) bool {
	for {
		if fi, err := os.Stat(p); err == nil && os.SameFile(fi, r.fi) {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// guardLane is ruling R4.2: no host call runs in a lane clone, save the one R4.5 admits.
func (x *Exec) guardLane(c Call) error {
	if x.Hardening.LaneRoot == "" {
		return nil
	}
	root := laneRoot{path: absDir(x.Hardening.LaneRoot)}
	if fi, err := os.Stat(root.path); err == nil {
		root.fi = fi
	}
	base := absDir(c.Dir)
	in := func(p string) bool { return root.holds(absAgainst(base, p)) }
	refuse := func(what string) error { return fmt.Errorf("%w: %s", ErrLaneDir, what) }
	newLane := c.Scope == None && c.NewLane != "" && samePath(c.Dir, c.NewLane)
	if !newLane && root.holds(base) {
		return refuse("Dir " + c.Dir)
	}
	for i, a := range c.Args {
		for _, flag := range []string{"-C", "--git-dir", "--work-tree"} {
			switch {
			case a == flag && i+1 < len(c.Args) && in(c.Args[i+1]):
				return refuse(flag + " " + c.Args[i+1])
			case flag != "-C" && strings.HasPrefix(a, flag+"=") && in(a[len(flag)+1:]):
				return refuse(a)
			}
		}
	}
	for k, v := range c.Env {
		switch {
		case envKeyIn(k, laneEnvKeys) && in(v):
			return refuse("Env " + k)
		case strings.EqualFold(k, "GIT_ALTERNATE_OBJECT_DIRECTORIES"):
			for _, p := range filepath.SplitList(v) {
				if p != "" && in(p) {
					return refuse("Env " + k)
				}
			}
		}
	}
	return nil
}

// newLaneUnderRoot is review M-7: root is non-empty and lane lies under it, not at it.
func newLaneUnderRoot(root, lane string) bool {
	if root == "" {
		return false
	}
	r, l := absDir(root), absDir(lane)
	return under(r, l) && !samePath(r, l)
}

// pinnedEnv are the keys H5's layout pin owns; a Call naming one is ErrCallShape.
var pinnedEnv = []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE"}

func shape(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCallShape}, a...)...)
}

// checkShape refuses a Call Exec will not run (ErrCallShape). Nothing has started.
func (x *Exec) checkShape(c Call) error {
	h := x.Hardening
	switch {
	case h.HooksDir == "":
		return shape("the Exec has no Hardening.HooksDir")
	case len(c.Args) == 0:
		return shape("no subcommand")
	case strings.HasPrefix(c.Args[0], "-"):
		return shape("Args[0] %q is an option; the prefix is Exec's alone", c.Args[0])
	case c.Scope != Project && c.Scope != None:
		return shape("the call names no Scope")
	case c.Class != Read && c.Class != LocalWrite && c.Class != Remote:
		return shape("unknown Class %d", c.Class)
	case c.Class == Remote && c.RemoteURL == "":
		return shape("a Remote call names no RemoteURL")
	case strings.ContainsAny(c.RemoteURL, "=\n\r\x00"):
		return shape("RemoteURL cannot be written into a -c key")
	case c.NewLane != "" && c.Scope != None:
		return shape("NewLane is set on a call that is not Scope None")
	case c.NewLane != "" && !newLaneUnderRoot(h.LaneRoot, c.NewLane):
		// review M-7: NewLane names the clone lanes.Create makes under CloneRoot, never some
		// other directory, and with no LaneRoot there is no lane to create.
		return shape("NewLane %s is not strictly under Hardening.LaneRoot %q", c.NewLane, h.LaneRoot)
	}
	for k := range c.Env {
		if envKeyIn(k, pinnedEnv) {
			return shape("Call.Env names %s, which Exec pins", k)
		}
	}
	if c.Scope == None {
		switch {
		case c.NewLane != "" && samePath(c.Dir, c.NewLane):
		case c.Dir == "" && len(c.Args) == 1 && c.Args[0] == "version":
		case h.OwnedRoot != "" && c.Dir != "" && under(absDir(h.OwnedRoot), absDir(c.Dir)):
		default:
			return shape("a Scope None call in %s, outside OwnedRoot and not its NewLane", c.Dir)
		}
	}
	return nil
}

// layoutPin is H5: a Project call whose <Dir>/.git is a directory runs with GIT_DIR and
// GIT_COMMON_DIR set to it, so a .git/commondir file cannot move the config git reads.
func layoutPin(dir string) []string {
	d := absDir(dir)
	gd := filepath.Join(d, ".git")
	if fi, err := os.Stat(gd); err != nil || !fi.IsDir() {
		return nil
	}
	return []string{"GIT_DIR=" + gd, "GIT_COMMON_DIR=" + gd, "GIT_WORK_TREE=" + d}
}

// checkConfig is H3 for a Project call: it lists the repository's own config, under H1's prefix
// and the layout pin, and refuses the call before git starts when a key is on the list.
func (x *Exec) checkConfig(ctx context.Context, c Call, pin []string) error {
	list := Call{Dir: c.Dir, Args: []string{"config", "--local", "--no-includes", "--list", "-z"}, Class: Read, Scope: Project}
	argv := append(x.prefix(Read, Project, ""), list.Args...)
	res, err := x.start(ctx, list, argv, environ(Read, nil, pin))
	if err != nil {
		return err
	}
	// A record is "key\nvalue\0", or "key\0" with no value: split on NUL only, and take the
	// key up to the first newline, so a value can neither forge a key nor hide one.
	for _, rec := range bytes.Split(res.Stdout, []byte{0}) {
		key, _, _ := strings.Cut(string(rec), "\n")
		if key != "" && refusedProjectKey(key, c.Class) {
			return &UntrustedConfig{Dir: c.Dir, Key: redactKey(key), Scope: Project}
		}
	}
	return nil
}

// redactKey is review I-1: git writes a URL subsection into the key, so a refused
// url."https://oauth2:<pat>@host/".insteadOf lists its token in the key, which reaches Error(),
// the API's details.key and every log line. The subsection (between the first and the last dot)
// loses its userinfo, up to its LAST "@", and then its query and fragment, each shown as "***".
// The last "@" anywhere, not only in the authority: a password holding an unencoded "/", "?" or
// "#" still ends at the "@" before the host, and a path "@" only costs the host's spelling. The
// section, the rest of the URL and the name stay, so the user can still find and move the key.
func redactKey(key string) string {
	first, last := strings.IndexByte(key, '.'), strings.LastIndexByte(key, '.')
	if first < 0 || first == last {
		return key
	}
	sub := key[first+1 : last]
	scheme := ""
	if i := strings.Index(sub, "://"); i >= 0 {
		scheme, sub = sub[:i+3], sub[i+3:]
	}
	if i := strings.LastIndexByte(sub, '@'); i >= 0 {
		sub = "***" + sub[i:]
	}
	if i := strings.IndexAny(sub, "?#"); i >= 0 {
		sub = sub[:i+1] + "***"
	}
	return key[:first+1] + scheme + sub + key[last:]
}

// refusedProjectKey is H3's Project row. Section and key names compare lowercased; the
// subsection is whatever lies between the first and the last dot.
func refusedProjectKey(key string, class Class) bool {
	k := strings.ToLower(key)
	first, last := strings.IndexByte(k, '.'), strings.LastIndexByte(k, '.')
	if first < 0 {
		return false
	}
	section, name, hasSub := k[:first], k[last+1:], first != last
	switch section {
	case "include", "includeif", "filter":
		return true
	case "diff":
		return (hasSub && (name == "textconv" || name == "command")) || (!hasSub && name == "external")
	case "merge":
		return hasSub && name == "driver"
	case "core":
		return !hasSub && (name == "gitproxy" || name == "alternaterefscommand")
	case "extensions":
		return !hasSub && name == "worktreeconfig"
	case "remote":
		return hasSub && (name == "pushurl" || name == "proxy" || name == "proxyauthmethod" || name == "vcs")
	case "url":
		return hasSub && (name == "insteadof" || name == "pushinsteadof")
	case "http":
		return class == Remote
	case "credential":
		return class == Remote && name != "helper" && name != "usehttppath"
	}
	return false
}

// MachineHardening creates the empty directory <root>/no-hooks, sets OwnedRoot to root, and reads
// H2's helpers and proxy from `git config --system`, then `--global`, run with Dir root and Scope
// None. Task 04 calls it once in main.go, with root statepath.Dir("git"), then sets LaneRoot to
// its Machine's CloneRoot(); MachineHardening leaves LaneRoot "".
func MachineHardening(ctx context.Context, root string) (Hardening, error) {
	root = absDir(root)
	hooks := filepath.Join(root, "no-hooks")
	if err := os.MkdirAll(hooks, 0o700); err != nil {
		return Hardening{}, fmt.Errorf("gitx: hooks directory: %w", err)
	}
	x, err := NewExec(Hardening{HooksDir: hooks, OwnedRoot: root})
	if err != nil {
		return Hardening{}, err
	}
	h := x.Hardening
	proxySet := false
	for _, file := range []string{"--system", "--global"} {
		// Exit 1 is "no such key" (and a missing file); anything else is a machine git
		// that cannot read its own config, which the user must see at startup.
		// --includes (review M-5 a): a named file does not follow [include] by default, and H1's
		// credential.helper= reset would then clear an included helper on every Remote call.
		res, err := x.Run(ctx, Call{Dir: root, Class: Read, Scope: None, OKExit: []int{1},
			Args: []string{"config", file, "--includes", "-z", "--get-regexp", `^credential\..*helper$`}})
		if err != nil {
			return Hardening{}, err
		}
		for _, rec := range bytes.Split(res.Stdout, []byte{0}) {
			if len(rec) == 0 {
				continue
			}
			key, value, _ := strings.Cut(string(rec), "\n")
			h.Credential = append(h.Credential, key+"="+value)
		}
		res, err = x.Run(ctx, Call{Dir: root, Class: Read, Scope: None, OKExit: []int{1},
			Args: []string{"config", file, "--includes", "--get", "http.proxy"}})
		if err != nil {
			return Hardening{}, err
		}
		// Exit 0 is a set key, even an empty one (review M-5 b): `http.proxy =` is git's "no
		// proxy at all" (http.c), so it wins over the system file and the environment. Global is
		// read last, so it wins, as it does in git.
		if res.ExitCode == 0 {
			h.Proxy, proxySet = strings.TrimSpace(string(res.Stdout)), true
		}
	}
	if !proxySet {
		// review M-5 c: all_proxy too, which curl reads (Linux; Windows' environment folds case).
		for _, k := range []string{"HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
			if v := os.Getenv(k); v != "" {
				h.Proxy = v
				break
			}
		}
	}
	return h, nil
}
