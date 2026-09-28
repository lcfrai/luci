// Package contextwrite puts a session's inherited brief into its worktree.
//
// THE ORDERING RULE: the file must be on disk before the harness process
// starts. api.handleSessionCreate and handleSessionRestart call Materialise
// after the request validates and before anything is recorded or created:
// on create, before the conversation store's MkdirAll too; on restart,
// before the old sandbox's destroy (ruling R1; review C1, M2). Called
// afterwards, the session inherits nothing, silently, and the feature looks
// built.
//
// THE COMMIT RULE: the managed block is never committed (phase 3's 3-D6). It
// is written only to a file the repository does not track, and that file's
// path is added to the clone's info/exclude. A file git tracks is never
// opened for writing, not even to strip a block from it (review I3): a
// tracked file that already holds a block is repository text, left
// byte-identical and listed in Inspect's Committed (re-review R-M1).
//
// THE LINK RULE (confine.go, review I-2): nothing below the worktree is
// followed. The sandbox writes that tree, links included, and the host
// would resolve them against its own filesystem.
package contextwrite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

const (
	FileName    = "CLAUDE.md"         // the name is the contract with Claude Code; not configurable
	AltFileName = ".claude/CLAUDE.md" // P4-D, when CLAUDE.md is tracked - UNVERIFIED that the CLI reads it (task 23)
)

// Source is discoverhub.Service's Inheritable.
type Source interface {
	Inheritable(ctx context.Context, t discoverhub.Target) (discoverhub.Inheritable, error)
}

// Materialiser is the one writer of the managed block (and of the exclude
// line). Calls for one worktree path are serialised; different worktrees
// run in parallel.
type Materialiser struct {
	src    Source
	git    Git
	log    *slog.Logger
	budget time.Duration

	mu    sync.Mutex
	locks map[string]*worktreeLock
}

type worktreeLock struct {
	mu    sync.Mutex
	users int // holders and waiters; the entry is dropped at zero
}

// New builds a Materialiser. A nil log discards.
func New(src Source, git Git, log *slog.Logger) *Materialiser {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Materialiser{src: src, git: git, log: log, budget: HubBudget, locks: map[string]*worktreeLock{}}
}

// HubBudget bounds step 4's Hub reads (review I4). A black-holed Hub costs
// hubclient ~31 s per read (Hub contract §9.2: 10 s per attempt, two
// retries), and the desktop's session start waits 10 s (client.ts:33), so
// without it 4-D1's refusal would reach the person as the desktop's timeout.
// 5 s leaves the other 5 s for the sandbox and the rest of the start.
// Tests shorten it with WithHubBudget.
const HubBudget = 5 * time.Second

// WithHubBudget sets the budget and returns m. Call it before first use.
func (m *Materialiser) WithHubBudget(d time.Duration) *Materialiser {
	m.budget = d
	return m
}

var _ api.Materialiser = (*Materialiser)(nil)

// Materialise, in order:
//  1. WorktreePath must be absolute and an existing directory, else a plain
//     error (500: the handler resolved it a moment ago).
//  2. Ask Git.Tracked of both candidates, once per call, and keep both
//     answers: the STRIPPABLE candidates are exactly the ones git does not
//     track (review I3). Choose the target: FileName if git does not track
//     it; else AltFileName if git does not track that; else
//     *api.MaterialiseError tracked_elsewhere (this step still runs before
//     step 3; the WithoutBrief dead end on that path is review N1, open for
//     its own batch). A directory git does not manage (Git reports
//     ErrNotARepo) has both candidates strippable, target FileName and no
//     exclude line. Any other Git error → a plain error (500), never a
//     guess at "untracked".
//  3. WithoutBrief: strip any managed block from every STRIPPABLE candidate
//     and return "none:declined". A tracked candidate is never opened. A
//     strip that fails refuses: a session labelled "started without the
//     brief" must not find an old brief in a file this package writes. A
//     block already committed in a tracked file is repository text that no
//     Panacea write may remove (3-D6): it is left byte-identical, the start
//     is not refused for it (the owner's OQ-7, plan §10: the recommended
//     option, with its provenance cost named there), and Inspect lists it
//     in Committed, which task 19 shows with the file's path (review I3,
//     re-review R-M1).
//     The reason follows the cause (re-review I2):
//     claudemd.ErrUnterminatedBlock / ErrDuplicateBlock → block_broken (the
//     markers are broken; the person repairs the file); a read or write
//     error → write_failed (the disk refused). Since the marker rule (plan
//     §4 Security, S3; task 03 rule 12, task 05's neutraliser) no brief text
//     can break the markers, so block_broken here is reached only by a hand
//     edit - including a stray or early end marker, which task 05 counts
//     as ErrUnterminatedBlock instead of stripping up to it and leaving the
//     brief's tail behind a none:declined stamp.
//  4. ProjectID "" is a worktree outside every project, which the handler
//     enumerated cleanly (task 10's sessionLocate; review I2): there is no
//     project, so nothing to inherit, and Source is NOT asked (a Locator
//     would answer project.ErrUnknownProject, a plain error, a 500 on
//     every start). An enumeration failure never reaches here as "": the
//     handler refuses it as project_unknown, a reason in api's closed set
//     that this package never returns.
//     Otherwise ask Source, under context.WithTimeout(ctx, HubBudget) (5 s,
//     review I4): every Hub read Inheritable makes shares that one budget.
//     hubclient.ErrUnavailable → hub_unavailable; the budget's OWN expiry
//     (the child ctx's Err() is DeadlineExceeded while the caller's ctx.Err()
//     is nil) → hub_unavailable too, whatever error type Source returned
//     (HA4-10: the Hub's task 33 retries even the caller's own expiry and
//     sleeps without ctx, so up to 1.25 s more can pass, about 6.25 s in
//     all, inside session start's 10 s; task 11); the caller's own ctx
//     cancelled (the desktop went away) → a plain error, never a refusal
//     reason. Only this step is budgeted: steps 1-3 and 5-7 are local git
//     and disk, and an expiry there would be misnamed as the Hub's.
//     discoverhub.ErrPinUnreadable (task 07 as built: the Hub answers the
//     branch revision's pinned project revision missing) → hub_unavailable:
//     a brief exists and the Hub cannot hand over what it builds on, which
//     4-D1 refuses, as it refuses an unreachable Hub (t07 review m3).
//     hubclient.ErrSignedOut, a relayed 401, or a relayed 403
//     tenant_not_enrolled → signed_out (a brief may exist and cannot be
//     read: 4-D1 refuses; the owner's OQ-11, plan §10, recommended option,
//     with Sign in first in task 20; review M10; task 20 offers no Sign in
//     for tenant_not_enrolled, re-review b09 M-1).
//     ErrHubOff, ErrNotBound, ErrNoRemote, a Hub 404, or none with no error
//     (a nil Hub, task 07 rule 6; re-review b05-2 N-c) → nothing to
//     inherit. Any other error → a plain error.
//  5. Nothing to inherit: strip every strippable candidate; return "".
//     Something: Merge(existing, Render(doc)) into the target, and strip
//     the other candidate only if it is strippable, so at most one managed
//     block exists across the files this package writes. A tracked other
//     candidate is not opened, even if it holds a block: the target is
//     still written and the key still returned, and Inspect lists that
//     file in the worktree's Committed (review I3, re-review R-M1).
//     claudemd.ErrUnterminatedBlock / ErrDuplicateBlock → block_broken.
//     Every new content is computed before anything is written, so a
//     broken marker in either file writes nothing.
//  6. The exclude lines FIRST (review M13): whenever the target will hold a
//     block, in a directory git manages, append to the path Git.ExcludePath
//     returns, each if absent, the target's line ("/CLAUDE.md" or
//     "/.claude/CLAUDE.md") and its temp file's ("/.CLAUDE-*.md" or
//     "/.claude/.CLAUDE-*.md": statepath.WriteAtomic's temp holds the whole
//     brief until its rename, and a crash leaves it; review m2). Asked on
//     every such start, the target's bytes unchanged included, so a line a
//     person removed comes back before the next write needs it. Then ask
//     Git.Ignored of the target (review m1): a .gitignore's "!CLAUDE.md"
//     outranks info/exclude, so false → write_failed, and an error → a plain
//     error. A Git.ExcludePath error is a plain error (500, step 2's Git rule); a
//     failed read or write of the exclude file → write_failed. Either way
//     the target is not opened for writing, so no brief text lands in a
//     file git would stage (3-D6: an agent's `git add -A` commits an
//     untracked, un-excluded CLAUDE.md). A strip that lands no brief text
//     needs no line. ErrNotARepo (step 2): no line, nothing to exclude from.
//  7. Write only if the bytes changed, with statepath.WriteAtomic(..., Shared),
//     the other candidate's strip before the target's write (review m7). A
//     file left holding only white space after a strip is removed. Every
//     write re-checks the link rule (confine.go) first. Any write error →
//     write_failed. A failure here leaves step 6's line in place: it names
//     a file that is absent or holds no new brief text, and step 6 would add
//     it again next start anyway ("if absent").
//  8. Return the key written.
//
// Calls for one worktree path are serialised; different worktrees run in
// parallel.
func (m *Materialiser) Materialise(ctx context.Context, req api.MaterialiseRequest) (api.MaterialiseResult, error) {
	dir, err := worktreeDir(req.WorktreePath) // step 1
	if err != nil {
		return api.MaterialiseResult{}, err
	}
	defer m.lock(dir)()

	c, err := m.choose(ctx, dir) // step 2
	if err != nil {
		return api.MaterialiseResult{}, err
	}
	if req.WithoutBrief { // step 3
		if err := m.stripAll(c); err != nil {
			return api.MaterialiseResult{}, err
		}
		m.log.Info("contextwrite: started without the brief", "worktree", req.WorktreeID)
		return api.MaterialiseResult{ContextRef: provenance.Declined}, nil
	}
	in, err := m.inherit(ctx, req) // step 4
	if err != nil {
		return api.MaterialiseResult{}, err
	}
	if in.Key == "" { // step 5, nothing to inherit
		if err := m.stripAll(c); err != nil {
			return api.MaterialiseResult{}, err
		}
		return api.MaterialiseResult{}, nil
	}

	// Step 5, something: every new content first, nothing written yet.
	var writes []pending
	if other := c.other(); c.strippable(other) {
		w, err := planStrip(dir, c.abs(other))
		if err != nil {
			return api.MaterialiseResult{}, err
		}
		writes = append(writes, w)
	}
	target := c.abs(c.target)
	existing, _, err := readCandidate(dir, target)
	if err != nil {
		return api.MaterialiseResult{}, err
	}
	merged, err := claudemd.Merge(existing, claudemd.Render(in.Document))
	if err != nil {
		return api.MaterialiseResult{}, markerRefusal(target, err)
	}
	writes = append(writes, pending{path: target, old: existing, new: merged})

	if c.repo { // step 6
		if err := m.ensureExcluded(ctx, dir, c.target); err != nil {
			return api.MaterialiseResult{}, err
		}
	}
	if err := apply(dir, writes); err != nil { // step 7
		return api.MaterialiseResult{}, err
	}
	m.log.Info("contextwrite: brief materialised", "worktree", req.WorktreeID, "key", in.Key, "file", c.target)
	return api.MaterialiseResult{ContextRef: in.Key}, nil // step 8
}

// ── step 1: the worktree ────────────────────────────────────────────────────

func worktreeDir(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("contextwrite: worktree path %q is not absolute", p)
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("contextwrite: worktree: %w", err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("contextwrite: worktree path %q is not a directory", p)
	}
	return filepath.Clean(p), nil
}

// lock serialises calls for one worktree and returns the unlock. The key is
// the cleaned path, case-folded on Windows, where two spellings name one
// directory.
func (m *Materialiser) lock(dir string) func() {
	key := dir
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	m.mu.Lock()
	l := m.locks[key]
	if l == nil {
		l = &worktreeLock{}
		m.locks[key] = l
	}
	l.users++
	m.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		m.mu.Lock()
		if l.users--; l.users == 0 {
			delete(m.locks, key)
		}
		m.mu.Unlock()
	}
}

// ── step 2: the target choice, shared with Inspect ──────────────────────────

// candidates, in their fixed order: FileName first.
var candidates = [2]string{FileName, AltFileName}

type choice struct {
	dir     string
	repo    bool    // git manages dir; false on ErrNotARepo
	tracked [2]bool // Git.Tracked's answers, by candidates' index
	target  string  // FileName or AltFileName
}

// choose asks Git.Tracked of both candidates, once each, and picks the
// target (step 2).
func (m *Materialiser) choose(ctx context.Context, dir string) (choice, error) {
	c := choice{dir: dir, repo: true}
	var errs [2]error
	for i, rel := range candidates {
		c.tracked[i], errs[i] = m.git.Tracked(ctx, dir, rel)
	}
	for i, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, ErrNotARepo):
			c.repo = false
		default:
			return choice{}, fmt.Errorf("contextwrite: git ls-files %s: %w", candidates[i], err)
		}
	}
	if !c.repo {
		c.tracked = [2]bool{}
		c.target = FileName
		return c, nil
	}
	switch {
	case !c.tracked[0]:
		c.target = FileName
	case !c.tracked[1]:
		c.target = AltFileName
	default:
		return choice{}, &api.MaterialiseError{
			Reason:  api.BriefReasonTracked,
			Message: "Git tracks both CLAUDE.md and .claude/CLAUDE.md in this worktree, so the brief has no file it can be written to without being committed.",
		}
	}
	return c, nil
}

func index(rel string) int {
	if rel == FileName {
		return 0
	}
	return 1
}

func (c choice) strippable(rel string) bool { return !c.tracked[index(rel)] }

func (c choice) other() string {
	if c.target == FileName {
		return AltFileName
	}
	return FileName
}

func (c choice) abs(rel string) string { return filepath.Join(c.dir, filepath.FromSlash(rel)) }

// excludeLine is step 6's line naming the target itself, "" outside git
// (Inspection.Excluded); step 6 also adds the temp file's (excludeLines).
func (c choice) excludeLine() string {
	if !c.repo {
		return ""
	}
	return excludeLines(c.target)[0]
}

// ── step 4: what the session inherits, shared with Inspect ──────────────────

// inherit is step 4: none for ProjectID "" without asking Source, else
// Source under the Hub budget, its errors mapped.
func (m *Materialiser) inherit(ctx context.Context, req api.MaterialiseRequest) (discoverhub.Inheritable, error) {
	if req.ProjectID == "" {
		return discoverhub.Inheritable{}, nil
	}
	bctx, cancel := context.WithTimeout(ctx, m.budget)
	defer cancel()
	in, err := m.src.Inheritable(bctx, discoverhub.Target{ProjectID: req.ProjectID, BranchID: req.BranchID})
	if err == nil {
		// The stamp is in.Key and the file's marker is in.Document's
		// BriefID@Revision (claudemd.Render): they must name one revision, or
		// the session's provenance and its file disagree (review m3).
		mk, _ := markerKey(claudemd.Marker{BriefID: in.Document.BriefID, Revision: in.Document.Revision})
		if in.Key != "" && (in.Document.Key != in.Key || mk != in.Key) {
			return discoverhub.Inheritable{}, fmt.Errorf("contextwrite: Source answered key %q with a document keyed %q whose marker names %q", in.Key, in.Document.Key, mk)
		}
		return in, nil
	}
	var he *hubclient.Error
	isStatus := func(s int) bool { return errors.As(err, &he) && he.Status == s }
	switch {
	case ctx.Err() != nil:
		return discoverhub.Inheritable{}, fmt.Errorf("contextwrite: the session start was abandoned: %w", err)
	case errors.Is(bctx.Err(), context.DeadlineExceeded):
		return discoverhub.Inheritable{}, m.refuse(req, api.BriefReasonHubUnavailable, "The Hub did not answer within "+m.budget.String()+", so the brief this session would inherit could not be read.", err)
	case errors.Is(err, hubclient.ErrUnavailable):
		return discoverhub.Inheritable{}, m.refuse(req, api.BriefReasonHubUnavailable, "The Hub did not answer, so the brief this session would inherit could not be read.", err)
	case errors.Is(err, discoverhub.ErrPinUnreadable):
		return discoverhub.Inheritable{}, m.refuse(req, api.BriefReasonHubUnavailable, "The Hub could not return the project brief revision this branch's brief builds on, so the brief could not be written.", err)
	case errors.Is(err, hubclient.ErrSignedOut), isStatus(401),
		hubclient.Code(err) == hubapi.CodeTenantNotEnrolled:
		return discoverhub.Inheritable{}, m.refuse(req, api.BriefReasonSignedOut, "You are not signed in to the Hub, so the brief this session would inherit could not be read.", err)
	case errors.Is(err, discoverhub.ErrHubOff), errors.Is(err, discoverhub.ErrNotBound),
		errors.Is(err, discoverhub.ErrNoRemote), isStatus(404):
		return discoverhub.Inheritable{}, nil
	}
	return discoverhub.Inheritable{}, fmt.Errorf("contextwrite: reading what the session inherits: %w", err)
}

// refuse is step 4's refusal. The cause is logged, not carried: the handler
// answers Reason and Message, and a Hub error's text is no sentence for the
// person.
func (m *Materialiser) refuse(req api.MaterialiseRequest, reason, msg string, cause error) error {
	m.log.Warn("contextwrite: brief not written", "worktree", req.WorktreeID, "reason", reason, "cause", cause)
	return &api.MaterialiseError{Reason: reason, Message: msg}
}

// ── steps 3, 5, 7: files ────────────────────────────────────────────────────

// pending is one file's planned content. new == old is no write.
type pending struct {
	path     string
	old, new string
	remove   bool // a strip left only white space: remove the file
}

// readCandidate reads a candidate under the link rule (confine.go); an
// absent file is "" and exists false. Any other error - a link, a directory
// where the file should be, a read the disk refused - is write_failed.
func readCandidate(dir, path string) (string, bool, error) {
	s, ok, err := readChecked(dir, path)
	if err != nil {
		return "", false, writeFailed(path, err)
	}
	return s, ok, nil
}

// planStrip is a strip of path's block, not yet written.
func planStrip(dir, path string) (pending, error) {
	existing, ok, err := readCandidate(dir, path)
	if err != nil || !ok {
		return pending{path: path}, err
	}
	stripped, err := claudemd.Strip(existing)
	if err != nil {
		return pending{}, markerRefusal(path, err)
	}
	p := pending{path: path, old: existing, new: stripped}
	p.remove = stripped != existing && strings.TrimSpace(stripped) == ""
	return p, nil
}

// stripAll strips every strippable candidate (steps 3 and 5): every strip
// planned first, so a broken marker in either writes nothing.
func (m *Materialiser) stripAll(c choice) error {
	var writes []pending
	for _, rel := range candidates {
		if !c.strippable(rel) {
			continue
		}
		w, err := planStrip(c.dir, c.abs(rel))
		if err != nil {
			return err
		}
		writes = append(writes, w)
	}
	return apply(c.dir, writes)
}

// apply writes, in order, every pending whose bytes changed, each path
// re-checked under the link rule first: git ran between the plan and now.
func apply(dir string, writes []pending) error {
	for _, w := range writes {
		if !w.remove && w.new == w.old {
			continue
		}
		if _, err := checkPath(dir, w.path); err != nil {
			return writeFailed(w.path, err)
		}
		switch {
		case w.remove:
			if err := os.Remove(w.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return writeFailed(w.path, err)
			}
		case w.new != w.old:
			if err := statepath.WriteAtomic(w.path, []byte(w.new), statepath.Shared); err != nil {
				return writeFailed(w.path, err)
			}
		}
	}
	return nil
}

// ── step 6: the exclude line ────────────────────────────────────────────────

// ensureExcluded is step 6: it appends target's exclude lines to the
// clone's info/exclude, each unless a line of the file is already exactly it
// (a trailing "\r" ignored: git ignores it too; leading white space is part
// of a git pattern, so "  /CLAUDE.md" is another line, review m8), and then
// asks git whether it now ignores target (review m1).
func (m *Materialiser) ensureExcluded(ctx context.Context, dir, target string) error {
	path, err := m.git.ExcludePath(ctx, dir)
	if err != nil {
		return fmt.Errorf("contextwrite: git rev-parse --git-path info/exclude: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	path = filepath.Clean(path)
	existing, _, err := readCandidate(dir, path)
	if err != nil {
		return err
	}
	var add string
	for _, line := range excludeLines(target) {
		if !hasLine(existing, line) {
			add += line + "\n"
		}
	}
	if add != "" {
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		if err := statepath.WriteAtomic(path, []byte(existing+add), statepath.Shared); err != nil {
			return writeFailed(path, err)
		}
	}
	ignored, err := m.git.Ignored(ctx, dir, target)
	if err != nil {
		return fmt.Errorf("contextwrite: git check-ignore -v -- %s: %w", target, err)
	}
	if !ignored {
		return &api.MaterialiseError{
			Reason: api.BriefReasonWriteFailed,
			Message: "Git would still list " + filepath.Join(dir, filepath.FromSlash(target)) +
				" although info/exclude names it: a .gitignore rule re-includes it (a line starting with !), and a .gitignore outranks info/exclude. " +
				"The brief is not written to a file an agent's git add would commit. Remove that rule, or start without the brief.",
		}
	}
	return nil
}

// excludeLines are step 6's lines for target: the file, and the temp file
// statepath.WriteAtomic writes beside it, which holds the whole brief until
// its rename and is what a crash between the two leaves (review m2).
// "CLAUDE.md" gives "/CLAUDE.md" and "/.CLAUDE-*.md"; ".claude/CLAUDE.md"
// gives "/.claude/CLAUDE.md" and "/.claude/.CLAUDE-*.md".
func excludeLines(target string) []string {
	i := strings.LastIndex(target, "/") + 1
	return []string{"/" + target, "/" + target[:i] + statepath.TempPattern(target[i:])}
}

func hasLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSuffix(l, "\r") == line {
			return true
		}
	}
	return false
}

// ── refusals ────────────────────────────────────────────────────────────────

// markerRefusal is block_broken for a broken marker, else write_failed.
func markerRefusal(path string, err error) error {
	if errors.Is(err, claudemd.ErrUnterminatedBlock) || errors.Is(err, claudemd.ErrDuplicateBlock) {
		return &api.MaterialiseError{
			Reason: api.BriefReasonBlockBroken,
			Message: path + " holds a Panacea brief marker with no partner, or two briefs. " +
				"Delete the Panacea block, every line from the first <!-- panacea:discover: line to the last, then start again.",
		}
	}
	return writeFailed(path, err)
}

func writeFailed(path string, err error) error {
	return &api.MaterialiseError{
		Reason:  api.BriefReasonWriteFailed,
		Message: "Could not read or write " + path + ": " + err.Error(),
	}
}
