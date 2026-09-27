package lanes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

var (
	ErrUnknownBranch = errors.New("lanes: the project has no such branch")
	ErrNameTaken     = errors.New("lanes: that worktree name is taken, or its directory exists")
	ErrBadName       = errors.New("lanes: a worktree name is 1-40 of a-z 0-9 . _ - and starts with a letter or digit")
)

// CreateRequest is one worktree to create. Project comes from the enumerator, so its Path is
// the scanner's own answer, never a path a client supplied.
type CreateRequest struct {
	Project project.Project
	Branch  string // must resolve as refs/heads/<Branch> in Project.Path
	Name    string // "" derives "<last segment of Branch, slugged, ≤ 24>-<n>", lowest free n ≥ 1
}

// Creator makes lane clones on one machine. Every create of its Store's lanes goes through one
// Creator (task 04 builds one), because step 3 holds the create's name in the Creator from the
// check to the end of Create: two creates at once in one project never clone under one name, a
// derived name skips a name another create holds, and the loser of an explicit name is
// ErrNameTaken before it clones (fix round 1, review I-1). A Creator is not copied after first use.
type Creator struct {
	Machine gitx.Machine
	Store   Store
	Now     func() time.Time // nil means time.Now
	// Nonce gives step 3's directory suffix: nil means 6 lowercase hex from crypto/rand. Tests pin
	// it to make a collision. It is never derived from the name, the time or a counter.
	Nonce func() string

	mu      sync.Mutex
	pending map[string]map[string]bool // ProjectID → the names creates in flight hold
}

// NoFetchURL and NoPushURL are a lane clone's origin (step 4a, review Minor 14). Each is a path
// that exists on neither machine and names who does the job, so the agent's `git pull` or
// `git push` in the sandbox fails with a sentence that tells it why. Task 01's
// TestSandboxCloneCannotFetch records that sentence verbatim in task 01's results document.
const (
	NoFetchURL = "/panacea-app/no-remote/the-cockpit-fetches-origin-and-upstream-for-this-worktree"
	NoPushURL  = "/panacea-app/no-remote/the-cockpit-lands-this-worktree-and-pushes-the-branch"
)

// NoFilterEnv is step 4's Call.Env: GIT_CONFIG_COUNT=3 with filter.lfs.smudge="",
// filter.lfs.process="" and filter.lfs.required=false, at command scope, so a machine's Git LFS
// (Git for Windows' installer can define filter.lfs.* in the system config, and `git lfs
// install` writes it to the global one; command scope beats both) neither runs nor smudges
// during the checkout (review Minor 13). The load-bearing keys are process= and required=false
// (carried t01-D11): git skips smudge and clean whenever filter.<d>.process is set, an empty
// process runs nothing, and without required=false a required driver that runs nothing fails
// the checkout. The empty smudge is kept for a driver defined with smudge alone, and is
// redundant wherever process is set. It is env, not `--config`, because env is command scope for
// this call only, so no `filter.*` key lands in the config the sandbox's git reads.
// Create passes a copy, so a Runner that edits Call.Env cannot change it.
var NoFilterEnv = map[string]string{
	"GIT_CONFIG_COUNT":   "3",
	"GIT_CONFIG_KEY_0":   "filter.lfs.smudge",
	"GIT_CONFIG_VALUE_0": "",
	"GIT_CONFIG_KEY_1":   "filter.lfs.process",
	"GIT_CONFIG_VALUE_1": "",
	"GIT_CONFIG_KEY_2":   "filter.lfs.required",
	"GIT_CONFIG_VALUE_2": "false",
}

// cloneArgv is step 4's argv, built in one place: it is the seam plan Q9's answer changes
// (--no-hardlinks), and no configuration switches it back.
func cloneArgv(branch, src, dst string) []string {
	return []string{"clone", "--no-hardlinks",
		"--config", "core.autocrlf=false", "--config", "core.eol=lf",
		"--config", "core.checkStat=minimal", "--config", "core.trustctime=false",
		"--config", "core.longpaths=true", "--config", "core.symlinks=false",
		"--branch", branch, "--origin", "origin", "--", src, dst}
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)

// windowsDevice is a Win32 reserved device name. A lane directory is <Name>-<Nonce>, so only a
// Name with a dot can reach one ("nul.x" makes "nul.x-aaaaaa", whose stem is NUL). validName
// also refuses a bare "nul" or "con", whose directory "nul-aaaaaa" is no device: stricter than
// it needs to be, and kept so, because a name that is a device name anywhere is refused
// everywhere (review m-3).
var windowsDevice = regexp.MustCompile(`^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`)

func validName(n string) bool {
	if !nameRE.MatchString(n) {
		return false
	}
	stem, _, _ := strings.Cut(n, ".")
	return !windowsDevice.MatchString(stem)
}

// validBranch refuses a branch name that git's revision syntax could read as more than a ref
// (`main~1`, `a..b`, `x@{u}`), or that a refspec could split (`:`), before any git runs.
func validBranch(b string) bool {
	return b != "" && !strings.HasPrefix(b, "-") && !strings.HasPrefix(b, "/") && !strings.HasSuffix(b, "/") &&
		!strings.Contains(b, "..") && !strings.Contains(b, "@{") && !strings.Contains(b, "//") &&
		!strings.ContainsAny(b, " ~^:?*[\\\x7f") && !strings.ContainsFunc(b, func(r rune) bool { return r < 0x20 })
}

func defaultNonce() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("lanes: crypto/rand: " + err.Error()) // rand.Read does not fail on supported platforms
	}
	return hex.EncodeToString(b[:])
}

// slugLast is the last segment of branch, lowercased, every run outside [a-z0-9] one "-",
// trimmed of "-", at most 24 characters; "lane" when nothing is left.
func slugLast(branch string) string {
	seg := branch[strings.LastIndex(branch, "/")+1:]
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(seg) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 24 {
		s = strings.TrimRight(s[:24], "-")
	}
	if s == "" {
		s = "lane"
	}
	return s
}

// Create runs task 03's steps in this order, and the order is the point:
//  0. FSOf(c.Machine) (seam S8-P3.6): ErrNoMachineFS, and nothing on disk;
//  1. validate a non-empty Name (ErrBadName); an empty one is derived at step 3; a Branch git's
//     revision syntax could read as more than a ref is ErrUnknownBranch before any git runs;
//  2. `rev-parse --verify --quiet refs/heads/<Branch>^{commit}` in the project: ErrUnknownBranch;
//  3. reserve, under c.mu: a name is taken when the index has it for this project or another
//     create in flight on c holds it; "" derives the lowest free n, a taken Name is ErrNameTaken,
//     and the name is held until Create returns (review I-1). Path = m.Join(m.CloneRoot(),
//     ProjectID, Name+"-"+Nonce()), drawn again while fs.Exists answers exists, at most 4 draws;
//  4. cloneArgv in the project (LocalWrite, Scope Project), Call.Env a copy of NoFilterEnv;
//     4a. origin's url and pushurl become NoFetchURL and NoPushURL (NewLane);
//     4b. the project's Upstream(<Branch>), when it has one, is fetched from Project.Path by its
//     literal path into refs/remotes/upstream/<Branch> (NewLane; carried b05-2 issue 1), so a lane
//     created after the last refresh can merge upstream/<Branch> from its first session;
//  5. `var GIT_AUTHOR_IDENT` in the project; when ParseIdent accepts it, user.name and
//     user.email in the clone (NewLane). No identity is not an error. The identity copied is
//     the one the project's git derives, phase 2's rule for who the user is (carried t01-D1);
//  6. `config --local --get core.hooksPath` in the project; a value CopyableHooksPath accepts is
//     written into the clone byte for byte (NewLane), then CopyGeneratedHooks;
//     6a. fs.Seal(ctx, Path): R4.5's window ends after the last NewLane write;
//  7. Store.Add. Its ErrExists, when the index now has Name for this project (another writer on
//     the same index), is ErrNameTaken, which task 04 answers as 409 `name_taken` (review I-1).
//
// A failure in 4-7 removes Path with fs.RemoveTree(CloneRoot, Path), under a context that the
// caller's cancel does not end; a failure in 0-3 has touched nothing. Steps 2, 4, 4b's and 5's
// reads and 6's `--get` run in the project with Scope Project; 4a, 4b's fetch, 5's and 6's
// writes and CopyGeneratedHooks' `ls-files` run with Dir Path, Scope None and NewLane Path.
// Step 4's clone names Path only as its positional destination and carries no NewLane.
func (c *Creator) Create(ctx context.Context, req CreateRequest) (Lane, error) {
	fsys, err := FSOf(c.Machine)
	if err != nil {
		return Lane{}, err
	}
	if req.Name != "" && !validName(req.Name) {
		return Lane{}, fmt.Errorf("%w: %q", ErrBadName, req.Name)
	}
	if !validBranch(req.Branch) {
		return Lane{}, fmt.Errorf("%w: %q", ErrUnknownBranch, req.Branch)
	}
	p := req.Project
	res, err := c.Machine.Git().Run(ctx, gitx.Call{Dir: p.Path, Class: gitx.Read, Scope: gitx.Project, OKExit: []int{1},
		Args: []string{"rev-parse", "--verify", "--quiet", "refs/heads/" + req.Branch + "^{commit}"}})
	if err != nil {
		return Lane{}, err
	}
	if res.ExitCode != 0 {
		return Lane{}, fmt.Errorf("%w: %q in %s", ErrUnknownBranch, req.Branch, p.ID)
	}
	name, release, err := c.reserve(p.ID, req.Branch, req.Name)
	if err != nil {
		return Lane{}, err
	}
	defer release() // after Store.Add or the cleanup below
	path, err := c.draw(ctx, fsys, p.ID, name)
	if err != nil {
		return Lane{}, err
	}
	l, err := c.build(ctx, fsys, req, name, path)
	if err != nil {
		if rerr := fsys.RemoveTree(context.WithoutCancel(ctx), c.Machine.CloneRoot(), path); rerr != nil {
			return Lane{}, errors.Join(err, rerr)
		}
		return Lane{}, err
	}
	return l, nil
}

// build is steps 4 to 7. Every call after the clone is in Path, the one R4.5 window.
func (c *Creator) build(ctx context.Context, fsys gitx.MachineFS, req CreateRequest, name, path string) (Lane, error) {
	m, p := c.Machine, req.Project
	git := m.Git()
	env := make(map[string]string, len(NoFilterEnv))
	for k, v := range NoFilterEnv {
		env[k] = v
	}
	if _, err := git.Run(ctx, gitx.Call{Dir: p.Path, Args: cloneArgv(req.Branch, p.Path, path),
		Class: gitx.LocalWrite, Scope: gitx.Project, Env: env}); err != nil {
		return Lane{}, err
	}
	inLane := func(args ...string) error {
		_, err := git.Run(ctx, gitx.Call{Dir: path, Args: args, Class: gitx.LocalWrite, Scope: gitx.None, NewLane: path})
		return err
	}
	if err := inLane("config", "remote.origin.url", NoFetchURL); err != nil {
		return Lane{}, err
	}
	if err := inLane("config", "remote.origin.pushurl", NoPushURL); err != nil {
		return Lane{}, err
	}
	up, err := Upstream(ctx, git, p.Path, req.Branch)
	if err != nil {
		return Lane{}, err
	}
	if up != "" {
		if err := inLane("fetch", "--no-tags", "--", p.Path, "+"+up+":refs/remotes/upstream/"+req.Branch); err != nil {
			return Lane{}, err
		}
	}
	ident, err := git.Run(ctx, gitx.Call{Dir: p.Path, Args: []string{"var", "GIT_AUTHOR_IDENT"},
		Class: gitx.Read, Scope: gitx.Project, OKExit: []int{128}})
	if err != nil {
		return Lane{}, err
	}
	if id, ok := gitx.ParseIdent(string(ident.Stdout)); ok && ident.ExitCode == 0 {
		if err := inLane("config", "user.name", id.Name); err != nil {
			return Lane{}, err
		}
		if err := inLane("config", "user.email", id.Email); err != nil {
			return Lane{}, err
		}
	}
	hooks, err := git.Run(ctx, gitx.Call{Dir: p.Path, Args: []string{"config", "--local", "--get", "core.hooksPath"},
		Class: gitx.Read, Scope: gitx.Project, OKExit: []int{1}})
	if err != nil {
		return Lane{}, err
	}
	if v := strings.TrimSuffix(strings.TrimSuffix(string(hooks.Stdout), "\n"), "\r"); hooks.ExitCode == 0 && CopyableHooksPath(v) {
		if err := inLane("config", "core.hooksPath", v); err != nil {
			return Lane{}, err
		}
		if _, err := CopyGeneratedHooks(ctx, m, p.Path, path, v); err != nil {
			return Lane{}, err
		}
	}
	if err := fsys.Seal(ctx, path); err != nil {
		return Lane{}, err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	l := Lane{ID: project.WorktreeID(path), ProjectID: p.ID, Branch: req.Branch, Name: name, Path: path,
		MachineID: m.ID(), CreatedAt: now().UTC()}
	if err := c.Store.Add(l); err != nil {
		if errors.Is(err, ErrExists) && indexHas(c.Store, p.ID, name) {
			return Lane{}, fmt.Errorf("%w: %q in %s: %w", ErrNameTaken, name, p.ID, err)
		}
		return Lane{}, err
	}
	return l, nil
}

// indexHas reports whether the index has a lane named name in the project.
func indexHas(s Store, projectID, name string) bool {
	for _, l := range s.ByProject(projectID) {
		if l.Name == name {
			return true
		}
	}
	return false
}

// Upstream is <branch>@{upstream} in the project when that is a remote-tracking ref, else "".
// It is the one reading of a lane's upstream: Create's step 4b and task 09's RefreshClones share
// it (carried b05-2 issue 1). No upstream configured, and one that is not fetched, exit 1.
func Upstream(ctx context.Context, git gitx.Runner, projectPath, branch string) (string, error) {
	res, err := git.Run(ctx, gitx.Call{Dir: projectPath, Class: gitx.Read, Scope: gitx.Project, OKExit: []int{1},
		Args: []string{"rev-parse", "--verify", "-q", "--symbolic-full-name", branch + "@{upstream}"}})
	if err != nil {
		return "", err
	}
	up := strings.TrimSpace(string(res.Stdout))
	if res.ExitCode != 0 || !strings.HasPrefix(up, "refs/remotes/") || strings.ContainsAny(up, ": \t\r\n") {
		return "", nil
	}
	return up, nil
}

// reserve is step 3's name, under c.mu, so the check and the hold are one step: a name is
// taken when the index has it for the project or another create in flight on c holds it. An
// empty name derives the lowest free n; a taken one is ErrNameTaken. release drops the hold;
// Create defers it, so the name stays held through Store.Add or the cleanup (review I-1).
func (c *Creator) reserve(projectID, branch, name string) (string, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	held := c.pending[projectID]
	taken := make(map[string]bool, len(held))
	for n := range held {
		taken[n] = true
	}
	for _, l := range c.Store.ByProject(projectID) {
		taken[l.Name] = true
	}
	if name == "" {
		name = deriveName(branch, taken)
	} else if taken[name] {
		return "", nil, fmt.Errorf("%w: %q in %s", ErrNameTaken, name, projectID)
	}
	if held == nil {
		if c.pending == nil {
			c.pending = map[string]map[string]bool{}
		}
		held = map[string]bool{}
		c.pending[projectID] = held
	}
	held[name] = true
	return name, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.pending[projectID], name)
		if len(c.pending[projectID]) == 0 {
			delete(c.pending, projectID)
		}
	}, nil
}

// maxDraws bounds step 3's nonce draws.
const maxDraws = 4

func (c *Creator) draw(ctx context.Context, fsys gitx.MachineFS, projectID, name string) (string, error) {
	nonce := c.Nonce
	if nonce == nil {
		nonce = defaultNonce
	}
	m := c.Machine
	for i := 0; i < maxDraws; i++ {
		path := m.Join(m.CloneRoot(), projectID, name+"-"+nonce())
		exists, _, err := fsys.Exists(ctx, path)
		if err != nil {
			return "", err
		}
		if !exists {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: %q: %d directories were taken", ErrNameTaken, name, maxDraws)
}

// deriveName is "<slug>-<n>", the lowest n ≥ 1 no lane of the project uses.
func deriveName(branch string, taken map[string]bool) string {
	base := slugLast(branch)
	for n := 1; ; n++ {
		if name := base + "-" + strconv.Itoa(n); !taken[name] {
			return name
		}
	}
}

// CopyableHooksPath reports whether a project's local core.hooksPath is copied into a lane
// clone: it is non-empty, relative (no leading `/` or `\`, no leading `~`), has no `..`
// segment, and holds no `\`, no `:`, no NUL and no CR or LF (review m-2). The first three keep
// the value inside the lane's working tree, where the sandbox's git resolves it; the sandbox's
// Linux git reads `\` as part of a file name, so a Windows-style value would name no directory
// there. Step 6 reads the value as `config --get`'s one line, so a value holding NUL, CR or LF
// is not read back intact. `:` refuses every drive form
// (`C:/hooks`, `C:hooks`) and an NTFS stream name (`.git::$INDEX_ALLOCATION`, which Windows opens
// as `.git`; carried b03-2 Minor 2). Husky writes `.husky/_`.
func CopyableHooksPath(v string) bool {
	if v == "" || strings.ContainsAny(v, "\\:\x00\r\n") || v[0] == '/' || v[0] == '~' {
		return false
	}
	for _, seg := range strings.Split(v, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// CopyGeneratedHooks copies <projectTop>/<rel> into <cloneTop>/<rel> when the project holds a
// generated hooks directory there, and reports how many files it copied (task 03 step 6).
// It refuses a rel with a `.git` segment here, first (compared case-insensitively after trimming
// the trailing dots and spaces Windows drops), then FSOf(m).CopyGeneratedHooks runs the guards
// and the copy (LocalFS: the clone side's existing parents and the project side's components
// are real directories, none of the project's is its .git by os.SameFile, the destination does
// not exist, only regular files, at most 64 and 1 MiB, SameFile at open). After a copy,
// `ls-files --others --exclude-standard -- <rel>` in the clone (NewLane) must print nothing;
// otherwise FSOf(m).RemoveTree(cloneTop, made) takes the copy back and it returns 0, so a user's
// unignored `.githooks/` never reaches an agent's `git add -A`. made is the first component of
// <cloneTop>/<rel> that did not exist before the copy (by fs.Exists), so the take-back removes
// the parents the copy created as well (review m-4).
func CopyGeneratedHooks(ctx context.Context, m gitx.Machine, projectTop, cloneTop, rel string) (int, error) {
	if hasGitSegment(rel) {
		return 0, nil
	}
	fsys, err := FSOf(m)
	if err != nil {
		return 0, err
	}
	made, err := firstMissing(ctx, fsys, m, cloneTop, rel)
	if err != nil {
		return 0, err
	}
	n, err := fsys.CopyGeneratedHooks(ctx, projectTop, cloneTop, rel)
	if err != nil || n == 0 {
		return 0, err
	}
	res, err := m.Git().Run(ctx, gitx.Call{Dir: cloneTop, Args: []string{"ls-files", "--others", "--exclude-standard", "--", rel},
		Class: gitx.Read, Scope: gitx.None, NewLane: cloneTop})
	if err != nil {
		return 0, err
	}
	if len(strings.TrimSpace(string(res.Stdout))) != 0 {
		if err := fsys.RemoveTree(ctx, cloneTop, made); err != nil {
			return 0, err
		}
		return 0, nil
	}
	return n, nil
}

// firstMissing is the first component of <cloneTop>/<rel> that fs.Exists answers absent. When
// every component exists (the copy then refuses its existing destination and copies nothing),
// it is m.Join(cloneTop, rel), the path the take-back named before review m-4.
func firstMissing(ctx context.Context, fsys gitx.MachineFS, m gitx.Machine, cloneTop, rel string) (string, error) {
	p := cloneTop
	for _, seg := range relSegments(rel) {
		p = m.Join(p, seg)
		exists, _, err := fsys.Exists(ctx, p)
		if err != nil {
			return "", err
		}
		if !exists {
			return p, nil
		}
	}
	return m.Join(cloneTop, rel), nil
}
