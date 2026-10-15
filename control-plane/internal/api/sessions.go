package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// sessionBasePath is the canonical prefix every session URL is built from.
// Contract §3.1 also serves the whole surface under /api/v1, but that alias is
// task 10's rewrite and the paths this package MINTS — Location and attachPath
// — use the canonical form so the desktop has one shape to match.
const sessionBasePath = "/v1"

// sessionWorkDir is where the worktree is mounted inside every sandbox and the
// working directory the harness starts in. Contract §5.1 step 5.
const sessionWorkDir = "/home/dev/work"

// sessionWorktreeAdminDir is where a primary worktree's linked-worktree
// metadata sits inside the sandbox, mounted read-only over itself. See
// sessionWorktreeAdmin.
const sessionWorktreeAdminDir = sessionWorkDir + "/.git/worktrees"

// sessionConversationDir is the one part of the agent user's ~/.claude that
// outlives a sandbox. Without it Resume has nothing to resume: every sandbox
// is new, so the transcript `claude --resume` reads died with the old one.
//
// Observed, not assumed — claude 2.1.274 in luci/sandbox:dev, started
// in sessionWorkDir, writes ~/.claude/projects/-home-dev-work/<conversation
// id>.jsonl, and a fresh container given only this directory resumed and
// forked that conversation with its history intact.
//
// ONLY this directory, and the reason is credentials. /login writes
// .credentials.json and .device-keys.json into ~/.claude itself, beside
// projects/, and mounting ~/.claude whole would carry a user's Claude
// credentials onto the host, into a directory that outlives every sandbox.
// images/sandbox/Dockerfile creates this path as the agent user: Docker makes a
// mount point's missing parents root-owned, which would leave ~/.claude
// unwritable and break /login.
const sessionConversationDir = "/home/dev/.claude/projects"

// sessionConversationOwner is the first path segment of every conversation
// store, between Deps.ConversationsDir and the worktree id. Phase 2 has one
// user, named the way the one machine is. Phase 8's multi-user backend puts the
// signed-in user's id here, so two users who share a worktree never share a
// transcript.
const sessionConversationOwner = "local"

// sessionStoreSegment is what a worktree id must look like to name a host
// directory. project.WorktreeID mints "w-" plus eight hex characters; this is
// the belt to that braces, because an id with a separator or ".." in it would
// put the store somewhere else on the host.
var sessionStoreSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// sessionConversationStore returns the host directory that holds one
// worktree's conversations, creating it if absent:
//
//	<Deps.ConversationsDir>/<owner>/<worktree id>
//
// Per worktree because a Resume continues in the worktree it ran in, and its
// sandbox needs no other worktree's transcripts. The one crossing is a Fork —
// a resume into another worktree — and it is given one file, a staged copy of
// its source's transcript, read-only, never another store (sessions_fork.go).
// 0o700 because a transcript holds
// whatever the agent read. On Windows that mode is advisory — Go's MkdirAll
// ignores it, and Docker Desktop shows the bind as root 777 inside the sandbox
// — so the boundary there is the ACL of the user's own profile, under which the
// state directory sits.
func (a *API) sessionConversationStore(worktreeID string) (string, error) {
	dir, err := a.sessionConversationStorePath(worktreeID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sessionConversationStorePath is where sessionConversationStore puts a
// worktree's store, without creating it: a resume that is refused must leave
// nothing behind, not even an empty directory.
func (a *API) sessionConversationStorePath(worktreeID string) (string, error) {
	if !sessionStoreSegment.MatchString(worktreeID) {
		return "", fmt.Errorf("worktree id %q cannot name a directory", worktreeID)
	}
	return filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, worktreeID), nil
}

// sessionTranscriptCLIVersion is the claude-code version the transcript layout
// below was observed on. OD-8 lets the control plane look at a harness's own
// file by an id the control plane minted, read-only, on condition that the
// reading is pinned to the CLI version in the image and a canary fails when
// that moves: TestTheTranscriptLayoutIsPinnedToTheImagesCLI fails the moment
// images/sandbox/Dockerfile installs any other version, and says to observe
// the layout again.
const sessionTranscriptCLIVersion = "2.1.274"

// sessionTranscriptPath is where the pinned CLI writes conversation id's
// transcript inside a worktree's store: the store is ~/.claude/projects, and
// the CLI names the directory under it after the working directory with every
// "/" made "-". Observed in luci/sandbox:dev, and recorded in the
// go-contract's verification table: ~/.claude/projects/-home-dev-work/<uuid>.jsonl.
func sessionTranscriptPath(store, conversation string) string {
	return filepath.Join(store, sessionTranscriptDirName, conversation+".jsonl")
}

// sessionTranscriptDirName is that directory's name: sessionWorkDir with every
// "/" made "-", which is how the pinned CLI names it.
var sessionTranscriptDirName = strings.ReplaceAll(sessionWorkDir, "/", "-")

// sessionStat is os.Stat, a variable only so a test can make the stat fail in
// a way this platform's filesystem cannot be made to.
var sessionStat = os.Stat

// sessionLstat is os.Lstat, a variable for the same reason: Forget's look at
// the transcript directory can fail in ways other than "absent", and no
// filesystem fails that way on demand on every platform.
var sessionLstat = os.Lstat

// sessionTranscriptState is the ONE look the control plane takes at a
// harness's own files: does the transcript for this conversation exist. It
// opens nothing and reads no byte of it, so it cannot come to depend on the
// format — only on where the file is, which the canary above pins.
//
// written is false only when the file is certainly absent. Any other failure
// to stat comes back as err, and every caller degrades on it rather than
// refusing, as OD-8 requires: nothing may need this look to work.
//
// A conversation id not spelled the one way ids.UUID mints is certainly
// absent, and nothing is looked at for it. Build refuses such an id, so no CLI
// ever wrote a transcript under it, and joined onto the store it would name
// some other file — "../x" climbs out — whose existence would then pass for a
// conversation: a resume would pick it, and a restart would list it.
func sessionTranscriptState(store, conversation string) (written bool, err error) {
	if !harness.IsConversationID(conversation) {
		return false, nil
	}
	if _, err := sessionStat(sessionTranscriptPath(store, conversation)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// sessionForgetTranscripts is OD-10, "Forget deletes it": it deletes the
// transcript of every conversation sess's record names as its own — its
// current one and the ones its restarts left — and nothing else. OD-8 lets the
// control plane delete a harness's own file by an id the control plane minted;
// it opens and rewrites nothing, and it never lists the store, so no other
// session's file can be reached from here. A transcript that was never written
// is already forgotten, and so is a store that was never made: neither is an
// error, and nothing is created to find that out.
//
// A fork does not depend on its source's file. `--resume A --fork-session
// --session-id B` writes B as a whole copy of A's history re-keyed to B
// (od10-forget-report.md §3), and a resume resolves only the resumed record's
// own ids (sessionResumableConversation), so forgetting A leaves B resumable.
// Observed in `-p` mode; the interactive fork before its first turn is
// unverified until OD-12's hands-on session (go-contract §3.15).
//
// Two refusals, both about an id joined onto a path:
//
//   - An id not spelled the one way ids.UUID mints is refused and logged at
//     error, and the rest carry on. Build refuses such an id, so no CLI ever
//     wrote a transcript under it; joined onto the store it names somewhere
//     else — "../../x" climbs out, and an uppercase copy of another id is that
//     other conversation's file on Windows. The same goes for a worktree id
//     that cannot name a store: sessionConversationStore refuses it, so no
//     store exists under it.
//   - The directory the CLI keeps transcripts in is inside the mount, so the
//     agent can replace it with a link. Deleting through a link deletes on the
//     host at the agent's choosing, so anything there but a real directory
//     fails the Forget. The look is not enough on its own: the store is
//     mounted into every OTHER live session in the worktree too, so an agent
//     there can swap the directory for a link after the look. So each file is
//     deleted by sessionRemoveIn, which deletes only the file actually in the
//     store's directory and fails otherwise. The names deleted are not all
//     this session's own — carriedConversationId is the fork's SOURCE's id —
//     so without it a well-timed link would delete the source's transcript.
//
// A fork into another worktree left two more things, and both go too: the
// staged copy of its source's transcript (sessionForgetCarried), and the empty
// file Docker made in this store as the mount point for that copy — named by
// the source's conversation id, CarriedConversationID, and deleted by it here
// like the session's own. The source's transcript, in the source's store, is
// the source's to forget, and nothing here can reach it.
//
// Any failure to delete comes back as the error, and the caller keeps the
// record that names the file.
func (a *API) sessionForgetTranscripts(sess registry.Session) error {
	if sess.Harness == string(harness.Shell) {
		return a.forgetShellHome(sess)
	}
	if nativeHarness(sess.Harness) {
		return a.forgetNativeState(sess)
	}
	return errors.Join(a.sessionForgetStoreTranscripts(sess), a.sessionForgetCarried(sess))
}

// sessionForgetStoreTranscripts is the half of sessionForgetTranscripts inside
// the session's worktree store.
func (a *API) sessionForgetStoreTranscripts(sess registry.Session) error {
	var ids []string
	named := append([]string{sess.ConversationID}, sess.EarlierConversationIDs...)
	for _, id := range append(named, sess.CarriedConversationID) {
		if id == "" {
			continue
		}
		if !harness.IsConversationID(id) {
			a.sessionLog().Error("forget: the record names a conversation id no mint produces; deleted nothing by it",
				"sessionId", sess.ID, "conversationId", id)
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	store, err := a.sessionConversationStorePath(sess.WorktreeID)
	if err != nil {
		a.sessionLog().Error("forget: the record's worktree id names no conversation store; deleted no transcript",
			"sessionId", sess.ID, "err", err)
		return nil
	}

	dir := filepath.Dir(sessionTranscriptPath(store, ids[0]))
	info, err := sessionLstat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("look at the transcript directory %s: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("the transcript directory %s is a %v, not a directory; deleted nothing through it", dir, info.Mode().Type())
	}

	// Every id is tried, a failure or not, and the failures come back
	// together: the record is kept either way, so the retry finds the rest.
	var errs []error
	for _, id := range ids {
		if err := sessionRemoveIn(store, sessionTranscriptDirName, id+".jsonl"); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("delete the transcript of conversation %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// sessionForgetWorktreeStore is OD-10's other half: removing a worktree from
// the product deletes that worktree's conversation store, every transcript in
// it included. No product action removes a worktree in phase 2 — the catalog
// enumerates the worktrees git already has, and the one worktree verb is PATCH
// ready (catalog.go) — so nothing calls this yet. It is the seam phase 3's
// worktree-removal verb calls, and that verb owes one thing first: every
// session in the worktree killed, so no CLI is writing into the store it
// removes. A store that does not exist is not an error. A worktree id that
// cannot name one is refused before anything is removed: ".." would name the
// whole owner directory.
func (a *API) sessionForgetWorktreeStore(worktreeID string) error {
	store, err := a.sessionConversationStorePath(worktreeID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(store); err != nil {
		return fmt.Errorf("delete the conversation store of worktree %s: %w", worktreeID, err)
	}
	return nil
}

// sessionHasConversation is whether a session on this harness is given a
// conversation id and a conversation store. SupportsResume is the test because
// resuming is the only thing either is for; a shell has no conversation, and
// mounting it one would only put transcripts in reach of a terminal.
func sessionHasConversation(d harness.Descriptor) bool {
	return d.Available() && d.SupportsResume && !d.NativeState()
}

// sessionCABundlePath is where Deps.CABundle appears inside every sandbox, and
// the value NODE_EXTRA_CA_CERTS names. Outside /home/dev so nothing the agent
// user owns sits over it, and read-only because a trust anchor the agent could
// rewrite is not one.
const sessionCABundlePath = "/etc/luci/ca-bundle.pem"

// sessionGitExcludesPath and sessionGitAttributesPath are where the host's
// global excludes and attributes files appear inside a sandbox, when the host's
// git reads one, and what core.excludesFile and core.attributesFile name there.
// Outside /home/dev like the CA bundle, and read-only: they are the host's
// files, and the host's git reads them too.
const (
	sessionGitExcludesPath   = "/etc/luci/git/ignore"
	sessionGitAttributesPath = "/etc/luci/git/attributes"
)

// sessionGitNoFile is what core.excludesFile or core.attributesFile names in a
// sandbox whose host's git reads no such file: an empty file, so the sandbox's
// git reads nothing there either. Leaving the key unset would not do that: git
// would read the sandbox's own ~/.config/git default, which the host never
// reads and an agent can write.
const sessionGitNoFile = "/dev/null"

// sessionGitFiles is the host's global excludes and attributes files a sandbox
// sees.
type sessionGitFiles struct {
	excludes   sessionGitFile
	attributes sessionGitFile
}

// sessionGitFile is one of them: mount, a host file mounted read-only at its
// container path; or inRepo, a relative path inside the repository, which the
// host's git reads against the repository's top and the sandbox's git reads
// against the same repository in its mount, so nothing is mounted (X-1 review
// m-4); or neither, when the host's git reads no such file.
type sessionGitFile struct {
	mount  string
	inRepo string
}

// sessionMounts is everything of the host a sandbox sees: its worktree; a
// primary worktree's linked-worktree metadata, read-only over itself, when
// sessionWorktreeAdmin found one; its worktree's conversation store when the
// harness has a conversation; for a fork's first sandbox, the staged copy of
// its source's transcript, read-only, inside that store (carried, see
// sessions_fork.go; the zero Mount otherwise); the operator's CA bundle,
// read-only, when there is one; and the host's global git excludes and
// attributes files, read-only, each when the host's git reads one.
func sessionMounts(worktreePath, worktreeAdmin, conversationStore string, carried backend.Mount, caBundle string, git sessionGitFiles) []backend.Mount {
	mounts := []backend.Mount{{HostPath: worktreePath, ContainerPath: sessionWorkDir}}
	if worktreeAdmin != "" {
		mounts = append(mounts, backend.Mount{HostPath: worktreeAdmin, ContainerPath: sessionWorktreeAdminDir, ReadOnly: true})
	}
	if conversationStore != "" {
		mounts = append(mounts, backend.Mount{HostPath: conversationStore, ContainerPath: sessionConversationDir})
	}
	if carried.HostPath != "" {
		mounts = append(mounts, carried)
	}
	if caBundle != "" {
		mounts = append(mounts, backend.Mount{HostPath: caBundle, ContainerPath: sessionCABundlePath, ReadOnly: true})
	}
	if git.excludes.mount != "" {
		mounts = append(mounts, backend.Mount{HostPath: git.excludes.mount, ContainerPath: sessionGitExcludesPath, ReadOnly: true})
	}
	if git.attributes.mount != "" {
		mounts = append(mounts, backend.Mount{HostPath: git.attributes.mount, ContainerPath: sessionGitAttributesPath, ReadOnly: true})
	}
	return mounts
}

// sessionEnv is the environment every sandbox gets beyond sandboxd's own three
// variables, for every harness: a shell user runs git and node as much as an
// agent does.
//
// Git's configuration travels as GIT_CONFIG_COUNT/KEY/VALUE (git 2.31+; the
// image has 2.39) rather than a file in the image, so sessionWorkDir stays the
// ONE place the mount point is written. Environment config is command scope,
// which outranks every config file, so neither a repository's config nor an
// agent's `git config --global` can change these. Eight settings:
//
//   - safe.directory = sessionWorkDir. A bind mount's owner is not the image's
//     dev user (Docker Desktop shows root), so without it every git command in
//     the worktree dies with "detected dubious ownership". This one path and
//     never "*", which would switch the protection off for every repository.
//     Environment config is command-scope, which git counts as protected, so
//     the setting is honoured; a repository's own config cannot widen it.
//   - user.useConfigOnly = true. With no identity, git would otherwise invent
//     one from the user and host name — dev@<container> — and on a host with a
//     domain it would succeed. Commits are made as the user or not at all.
//   - gc.worktreePruneExpire = never. Every linked worktree looks prunable from
//     the sandbox (see sessionWorktreeAdmin), so a gc would try to prune
//     them. The read-only mount already stops the deletion; this stops gc
//     trying.
//   - gc.auto = 0. With that metadata read-only, a gc in the sandbox cannot
//     lock a linked worktree's HEAD to expire its reflog, and exits 128
//     ("fatal: failed to run reflog", measured). A DETACHED auto gc that
//     fails writes .git/gc.log, and the host's own `git gc --auto` then
//     prints it and does nothing until the file is a day old (git-gc(1),
//     gc.logExpiry; documented, not measured here). So the sandbox never
//     starts one, and the host's git, which can write all of its metadata,
//     keeps the repository. A `git gc` an agent runs by hand still fails,
//     loudly, and harms nothing.
//   - core.autocrlf and core.eol = the host's own, from Deps.GitLineEndings.
//     The host's system and global config are invisible here, and Git for
//     Windows sets core.autocrlf=true at system scope. Without them a CRLF
//     checkout over LF blobs reads as every text file modified, and commit -a
//     writes CRLF blobs for files the agent never touched. The answer is from
//     the host's global and system config only, never the worktree's
//     .git/config, which the agent writes (X-1); these pairs outrank that
//     file's own core.autocrlf and core.eol in the sandbox. The repository's
//     .gitattributes is in the mount and applies as it does on the host.
//     core.eol matters when core.autocrlf is false and a file
//     is text by attribute: "native" is CRLF there and LF here, which is why
//     the resolver sends the host's answer and never "native".
//   - core.excludesFile and core.attributesFile = the host's own files, as
//     the host's global and system config name them and never a worktree's
//     (X-1: a worktree's core.excludesFile named an SSH key, and it was
//     mounted), from Deps.GitGlobalFiles, mounted read-only at sessionGitExcludesPath and
//     sessionGitAttributesPath, or the host's own relative value when it
//     names a path inside the repository, with nothing mounted (X-1 review
//     m-4), or sessionGitNoFile when the host's git reads
//     none (gitfix-rereview I-A). Git reads ~/.config/git/ignore even with
//     core.excludesFile unset, and this HOME has none, so without the first an
//     agent's `git add -A` commits what the host's git ignores:
//     .claude/settings.local.json on the owner's host, measured. The second is
//     the same class as core.autocrlf: a global `* text=auto` changes the
//     blobs a commit writes.
//
// The identity goes in as GIT_AUTHOR_* and GIT_COMMITTER_*, which git prefers
// over any config, so what Deps.GitIdentity resolved is what every commit says.
//
// NODE_EXTRA_CA_CERTS ADDS to Node's trust store. SSL_CERT_FILE would replace
// the system bundle, and verification is never disabled under any setting.
func sessionEnv(id GitIdentity, eol GitLineEndings, git sessionGitFiles, caBundle string) map[string]string {
	env := map[string]string{}
	pairs := [][2]string{
		{"safe.directory", sessionWorkDir},
		{"user.useConfigOnly", "true"},
		{"gc.worktreePruneExpire", "never"},
		{"gc.auto", "0"},
		{"core.autocrlf", eol.AutoCRLF},
		{"core.eol", eol.EOL},
		{"core.excludesFile", sessionGitFileIn(git.excludes, sessionGitExcludesPath)},
		{"core.attributesFile", sessionGitFileIn(git.attributes, sessionGitAttributesPath)},
	}
	for i, kv := range pairs {
		env["GIT_CONFIG_KEY_"+strconv.Itoa(i)] = kv[0]
		env["GIT_CONFIG_VALUE_"+strconv.Itoa(i)] = kv[1]
	}
	env["GIT_CONFIG_COUNT"] = strconv.Itoa(len(pairs))
	if id.Name != "" {
		env["GIT_AUTHOR_NAME"] = id.Name
		env["GIT_COMMITTER_NAME"] = id.Name
	}
	if id.Email != "" {
		env["GIT_AUTHOR_EMAIL"] = id.Email
		env["GIT_COMMITTER_EMAIL"] = id.Email
	}
	if caBundle != "" {
		env["NODE_EXTRA_CA_CERTS"] = sessionCABundlePath
	}
	return env
}

// sessionGitLineEndings asks Deps.GitLineEndings how the host's own config
// says git converts line endings, and refuses an answer the sandbox's git would
// read differently: anything but the three core.autocrlf words, and any
// core.eol but lf or crlf. "native" is the one that matters, because it means
// CRLF to Git for Windows and LF to the sandbox's git. An error, or a refused
// answer, fails the create or restart: there is no safe default.
func (a *API) sessionGitLineEndings(ctx context.Context) (GitLineEndings, error) {
	eol, err := a.deps.GitLineEndings(ctx)
	if err != nil {
		return GitLineEndings{}, err
	}
	if eol.AutoCRLF != "true" && eol.AutoCRLF != "false" && eol.AutoCRLF != "input" {
		return GitLineEndings{}, fmt.Errorf("the line-ending resolver answered core.autocrlf %q, which is not true, false or input", eol.AutoCRLF)
	}
	if eol.EOL != "lf" && eol.EOL != "crlf" {
		return GitLineEndings{}, fmt.Errorf("the line-ending resolver answered core.eol %q, which is not lf or crlf", eol.EOL)
	}
	return eol, nil
}

// sessionGitFileIn is the path a sandbox's git reads for one of the host's
// global files: where it is mounted when the host has it, the relative path
// itself when it is inside the repository, else sessionGitNoFile.
func sessionGitFileIn(f sessionGitFile, containerPath string) string {
	switch {
	case f.mount != "":
		return containerPath
	case f.inRepo != "":
		return f.inRepo
	}
	return sessionGitNoFile
}

// sessionGitGlobalFiles asks Deps.GitGlobalFiles where the host's own config
// says git looks for its global excludes and attributes files, and keeps each
// one git would read patterns from. The worktree is not asked about (X-1): its
// .git/config is the agent's, and a file it named would be mounted. An error
// fails the create or restart, like the line endings: a sandbox that guessed
// "none" commits what the host ignores.
func (a *API) sessionGitGlobalFiles(ctx context.Context, worktreeID string) (sessionGitFiles, error) {
	files, err := a.deps.GitGlobalFiles(ctx)
	if err != nil {
		return sessionGitFiles{}, err
	}
	if err := a.sessionGitConfigOrigins(files.Origins); err != nil {
		return sessionGitFiles{}, err
	}
	excludes, err := a.sessionGitGlobalFile(worktreeID, "core.excludesFile", files.Excludes)
	if err != nil {
		return sessionGitFiles{}, err
	}
	attributes, err := a.sessionGitGlobalFile(worktreeID, "core.attributesFile", files.Attributes)
	if err != nil {
		return sessionGitFiles{}, err
	}
	return sessionGitFiles{excludes: excludes, attributes: attributes}, nil
}

// sessionGitConfigOrigins fails the create when a host config file that SET a
// mounted global-file value is reached through the projects directory (X-3
// review 3 I-C). sessionGitGlobalFile below refuses a NAMED file an agent can
// swap (m-5); this refuses the LINE that names it. --includes is on by design
// (X-1), so under m-5's own premise — a dotfiles repository that is also a
// project — the [include]d file that sets core.excludesFile can live inside a
// project and name any host file, and that file's own path is nowhere near the
// projects directory. The origin is checked with the same walk m-5 uses, so a
// link or a junction on the way to it is caught the same way, and then by its
// link count, because a hard link is a second name that no walk of this one
// can see (hostpath.OneName).
//
// It FAILS CLOSED (X-3 review 4 I-C): the create goes on only when every
// origin is PROVED outside, and every other answer refuses — an origin that is
// empty or not absolute, one the walk cannot follow to a file, one that is
// gone, one whose link count cannot be read. git read that file an instant
// ago, so a name that does not resolve now is a check that did not run.
// Refusing only on ErrInside was the hole: git's C-quoted origin resolved to
// nothing, was logged, and the value it set was mounted.
func (a *API) sessionGitConfigOrigins(origins []string) error {
	for _, origin := range origins {
		if !filepath.IsAbs(origin) {
			return fmt.Errorf("the host's git reads a global-file setting from %q, which is not an absolute path, so where that file is cannot be checked and this session is not started", origin)
		}
		resolved, err := hostpath.ResolveOutside(a.projectsDir(), origin)
		if errors.Is(err, hostpath.ErrInside) {
			return fmt.Errorf("the host's git reads a global-file setting from %q, which is inside the projects directory where an agent can write it, so this session is not started: move that git config out of the projects directory (%v)", origin, err)
		}
		if err != nil {
			return fmt.Errorf("the host's git reads a global-file setting from %q, and that file cannot be followed to prove it is outside the projects directory, so this session is not started (%v)", origin, err)
		}
		if err := hostpath.OneName(resolved); err != nil {
			return fmt.Errorf("the host's git reads a global-file setting from %q, which has another name, a hard link that may be inside the projects directory where an agent can write it, so this session is not started: make it a file of its own (%v)", origin, err)
		}
	}
	return nil
}

// sessionGitGlobalFile is hostPath, with every link resolved, when the host's
// git reads patterns from it, which is when it is a regular file (Stat,
// following a link as git does), and "" otherwise, the way git itself treats
// it. Nothing there: git skips the path silently. Something git cannot read,
// or a directory: git reads no patterns from it and at most warns, so the
// sandbox reads none either, and the cause goes to the log.
//
// An absolute path reached through the projects directory fails the create
// (X-3). It is not downgraded to "no patterns", because that is the failure
// this resolver exists to prevent: without the host's excludes an agent's
// `git add -A` commits every file the host's git ignores, .claude/
// settings.local.json among them. A quiet /dev/null would leak more, more
// often, than the attack it was closing; the 500 names the one-line fix.
//
// A relative path is never mounted: git reads it against each repository's
// top, so it names a file in the worktree, which the agent writes, and not
// one of the host's (X-1). One that stays inside the repository goes to the
// sandbox as it is, in slash form, and the sandbox's git reads the same file
// in its mount (X-1 review m-4). One that leaves the repository (`..`, or a
// Windows drive- or root-relative path) names a file no sandbox can see, so
// it fails the create, with the fix in the sentence.
func (a *API) sessionGitGlobalFile(worktreeID, key, hostPath string) (sessionGitFile, error) {
	if hostPath == "" {
		return sessionGitFile{}, nil
	}
	if !filepath.IsAbs(hostPath) {
		if filepath.IsLocal(hostPath) {
			return sessionGitFile{inRepo: filepath.ToSlash(filepath.Clean(hostPath))}, nil
		}
		return sessionGitFile{}, fmt.Errorf("the host's git config sets %s to %q, a relative path that leaves the repository, so no sandbox can read the file the host's git reads there: set it to an absolute path, or to a path inside the repository", key, hostPath)
	}
	// The mount's source is the file, not a link to it. Dotfile managers link
	// ~/.config/git/ignore, and what a Docker host does with a link as a mount
	// source (Docker Desktop's share would see a Windows target) is then never
	// the question. Not measured with a link here.
	//
	// The walk is what makes that link safe to follow. It refuses as soon as
	// any step of the resolution is inside the projects directory, because a
	// step there is the agent's to swap: link ~/.config/git/ignore into a
	// dotfiles repository under the projects directory, as a dotfile manager
	// does, and an agent running in that repository points it at ~/.ssh/id_rsa
	// instead. Both ends of that chain are outside the projects directory, so
	// checking the named path or the resolved path lets the key through (X-1
	// review m-5, and review-2 m-4's likelier case). R4 rule 6: the host never
	// trusts anything a sandbox can write.
	resolved, err := hostpath.ResolveOutside(a.projectsDir(), hostPath)
	if err != nil {
		if errors.Is(err, hostpath.ErrInside) {
			return sessionGitFile{}, fmt.Errorf("the host's git config sets %s to %q, and reaching it passes through the projects directory, where an agent can swap it for a link to any file on this host, so it is not mounted: set it to a path outside the projects directory (%v)", key, hostPath, err)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			a.sessionLog().Warn("the host's git cannot read this file either, so the sandbox's git reads none",
				"worktreeId", worktreeID, "key", key, "path", hostPath, "err", err)
		}
		return sessionGitFile{}, nil
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			a.sessionLog().Warn("the host's git cannot read this file either, so the sandbox's git reads none",
				"worktreeId", worktreeID, "key", key, "path", resolved, "err", err)
		}
		return sessionGitFile{}, nil
	}
	if !info.Mode().IsRegular() {
		return sessionGitFile{}, nil
	}
	return sessionGitFile{mount: resolved}, nil
}

// sessionWorktreeAdmin returns the host path of a primary worktree's
// linked-worktree metadata, <worktree>/.git/worktrees, making the directory
// when there is none yet, and "" for a worktree whose .git is a regular file. A
// linked worktree's .git is a file, and has no such metadata. A .git that is a
// reparse point (a symlink or a junction) fails the create instead (m-D).
//
// sessionMounts mounts it read-only over itself (I-1 of the sandbox git fix's
// review). Each .git/worktrees/<id>/gitdir names a host path the sandbox
// cannot see, so the sandbox's git calls every linked worktree prunable, and a
// `git worktree prune` there would delete the host's metadata: after that,
// host git says "not a git repository" in the linked worktree. No setting
// stops an explicit prune; a read-only mount does, and it stops repair, move,
// remove and add too, each of which would write a Linux path into the host's
// metadata. The sandbox's git can still READ it, so it still knows which
// branches the linked worktrees hold.
//
// The directory is made rather than skipped when absent because a bind mount
// of a directory shows what is added to it later: a linked worktree created
// while the session runs is covered too. Git makes the same directory itself
// at the first `git worktree add`.
//
// Lstat, so a .git that is a symlink OR A JUNCTION fails the create rather than
// being followed to decide what to overmount (X-3 review 3 m-D): an agent in this
// worktree can aim such a link at any directory on this host. A linked
// worktree's .git is a regular FILE, not a reparse point, and still returns ""
// (it has no admin metadata of its own). Reaching this needs the real .git moved
// first — the recorded mv residue — so it is defence in depth, closed rather than
// left as O-2's "mounting nothing" for .git itself.
//
// AMENDED by X-3 (review O-2; was "making the directory when there is none yet"
// with nothing said about what is already there): .git/worktrees is Lstatted
// too, and a link OR A JUNCTION there fails the create. See below for why it
// fails rather than mounting nothing, and for the race this does not close.
//
// AMENDED by the X-3 fix round (x3-rails-review I-1): the Lstat now refuses any
// reparse point (os.ModeSymlink OR os.ModeIrregular), not only a symlink. On
// Windows a junction (mklink /J, no privilege) is a reparse point that
// os.Lstat reports as irregular but NOT a symlink, and filepath.EvalSymlinks
// does not resolve it — so the old symlink-only Lstat let a junction through
// and Confine below could not catch it either (it compares the junction's own
// location, which is inside the worktree). The Lstat is the load-bearing guard
// and is now pinned by tests for BOTH the in-worktree symlink and the junction.
func sessionWorktreeAdmin(worktreePath string) (string, error) {
	gitDir := filepath.Join(worktreePath, ".git")
	info, err := os.Lstat(gitDir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// m-D: a .git that is a reparse point fails closed, the same mask this
	// function applies to .git/worktrees below. A regular file (a linked
	// worktree's .git) falls through to the "" return, unaffected.
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return "", fmt.Errorf("%s is a link or a junction, and the host will not follow it to decide what to mount: an agent in this worktree can aim it at any directory on this host, so delete it and let git write the real .git", gitDir)
	}
	if !info.IsDir() {
		return "", nil
	}
	admin := filepath.Join(gitDir, "worktrees")
	// O-2 of the X-1 review: .git is Lstatted, .git/worktrees was not, and
	// MkdirAll Stats — which follows a link and answers nil for a directory.
	// So an agent that replaced .git/worktrees with a link to /etc got /etc
	// bind-mounted into its own sandbox. The agent writes this directory, so
	// the link is the agent's to plant, and the mount is the host's to refuse.
	// A junction (a reparse point that is not a symlink) is the agent's to
	// plant just the same and needs no privilege on Windows, so the mask below
	// refuses os.ModeIrregular as well: Confine cannot back this up, because
	// EvalSymlinks leaves a junction unresolved and reads it as in-worktree.
	//
	// It fails the create rather than mounting nothing: mounting nothing puts
	// the sandbox's git back where the read-only bind was added to stop it,
	// free to `git worktree prune` the HOST's metadata, and a session that
	// quietly lost a safety control is worse than one that did not start.
	if info, err := os.Lstat(admin); err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return "", fmt.Errorf("%s is a link or a junction, and the host will not mount what it points at: an agent in this worktree can aim it at any directory on this host, so delete it and let git make the directory again", admin)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(admin, 0o755); err != nil {
		return "", err
	}
	// R4 rule 6 as the rule rather than as two Lstats: whatever is mounted has
	// to be in the worktree it belongs to. The remaining hole is the race —
	// the agent can swap the directory for a link between this check and
	// Docker's mount, and a fork on the same worktree runs while it is live.
	// No check closes that; R4 rule 1 does, by never mounting the project
	// repository at all (phase 3). X-3 records it as the residue.
	if _, err := hostpath.Confine(worktreePath, admin); err != nil {
		return "", fmt.Errorf("%s does not stay inside the worktree, so it is not mounted: %w", admin, err)
	}
	return admin, nil
}

// sessionGitIdentity asks Deps.GitIdentity who the user's commits are by; the
// worktree is named only in the log. It never fails a session: an agent with
// no identity can still read, edit and run everything, and only a commit is
// refused. The refusal's cause goes to the log as the resolver wrote it,
// because that sentence names the fix.
func (a *API) sessionGitIdentity(ctx context.Context, worktreeID string) GitIdentity {
	if a.deps.GitIdentity == nil {
		return GitIdentity{}
	}
	id, err := a.deps.GitIdentity(ctx)
	if err != nil {
		a.sessionLog().Warn("the sandbox has no git identity, so a commit in it will fail",
			"worktreeId", worktreeID, "err", err)
		return GitIdentity{}
	}
	return id
}

// sessionJSON is the session object of contract §3.10, and it is a separate
// struct from registry.Session for one reason: AttachToken, Endpoint,
// SandboxID, SandboxRef, Command and Args must never reach the wire, and they
// cannot be tagged json:"-" on the record because they must reach the registry
// file. A separate struct makes the omission a compile-time fact instead of a
// review question. sessions_test.go proves it by searching the marshalled
// bytes, not by checking a field it remembered.
type sessionJSON struct {
	// Opaque allocation identity for terminal display continuity, never authorization.
	ExecutionGeneration string `json:"executionGeneration,omitempty"`
	// Optional recorded allocation readiness; never authorization or a health check.
	AttachReady *bool  `json:"attachReady,omitempty"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	MachineID   string `json:"machineId"`
	ProjectID   string `json:"projectId"`
	BranchID    string `json:"branchId"`
	WorktreeID  string `json:"worktreeId"`

	Harness                 string                          `json:"harness"`
	Model                   string                          `json:"model"`
	Effort                  string                          `json:"effort"`
	PermissionMode          string                          `json:"permissionMode"`
	OpeningPrompt           string                          `json:"openingPrompt"`
	AuthOnly                bool                            `json:"authOnly,omitempty"`
	AuthMode                string                          `json:"authMode,omitempty"`
	AgentConnectionID       string                          `json:"agentConnectionId,omitempty"`
	AgentConnectionRevision int                             `json:"agentConnectionRevision,omitempty"`
	AgentConnectionKind     string                          `json:"agentConnectionKind,omitempty"`
	AgentConnectionProtocol string                          `json:"agentConnectionProtocol,omitempty"`
	ImplementationContext   *registry.ImplementationContext `json:"implementationContext,omitempty"`
	ImplementationOutcome   *implementationOutcome          `json:"implementationOutcome,omitempty"`
	ReviewedContextDelivery *reviewedContextDelivery        `json:"reviewedContextDelivery,omitempty"`
	ProjectRunID            string                          `json:"projectRunId,omitempty"`
	ProjectRunOperationID   string                          `json:"projectRunOperationId,omitempty"`

	Status       string                       `json:"status"`
	ExitReason   string                       `json:"exitReason"`
	StartFailure registry.SessionStartFailure `json:"startFailure,omitzero"`

	Backend   string `json:"backend"`
	Isolation string `json:"isolation"`

	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	LastOutputAt string `json:"lastOutputAt"`
	AttachCount  int    `json:"attachCount"`
	AttachPath   string `json:"attachPath"`

	ContextRef string `json:"contextRef"`
}

// toJSON renders one session for the wire. base is the URL prefix attachPath is
// built from; production passes sessionBasePath.
func toJSON(s registry.Session, base string) sessionJSON {
	generation := sessionExecutionGeneration(s)
	return sessionJSON{
		ExecutionGeneration:     generation,
		AttachReady:             sessionAttachReady(s, generation),
		ID:                      s.ID,
		Title:                   s.Title,
		MachineID:               s.MachineID,
		ProjectID:               s.ProjectID,
		BranchID:                s.BranchID,
		WorktreeID:              s.WorktreeID,
		Harness:                 s.Harness,
		Model:                   s.Model,
		Effort:                  s.Effort,
		PermissionMode:          s.PermissionMode,
		OpeningPrompt:           s.OpeningPrompt,
		AuthOnly:                s.AuthOnly,
		AuthMode:                s.AuthMode,
		AgentConnectionID:       s.AgentConnectionID,
		AgentConnectionRevision: s.AgentConnectionRevision,
		AgentConnectionKind:     s.AgentConnectionKind,
		AgentConnectionProtocol: s.AgentConnectionProtocol,
		ImplementationContext:   s.ImplementationContext.Clone(),
		ImplementationOutcome:   implementationOutcomeFor(s),
		ReviewedContextDelivery: sessionReviewedContextDelivery(s),
		ProjectRunID:            s.ProjectRunID,
		ProjectRunOperationID:   s.ProjectRunOperationID,
		Status:                  string(s.Status),
		ExitReason:              s.ExitReason,
		StartFailure:            publicSessionStartFailure(s),
		Backend:                 s.Backend,
		// isolationFor is task 10's and is the package's only mapping of
		// contract §3.4's enum — group-C ruling 6. registry.Session.Backend is
		// a plain string because task 7 keeps registry free of a backend
		// import, so the call converts.
		Isolation:    isolationFor(backend.Kind(s.Backend)),
		CreatedAt:    sessionTime(s.CreatedAt),
		UpdatedAt:    sessionTime(s.UpdatedAt),
		LastOutputAt: sessionTime(s.LastOutputAt),
		AttachCount:  s.AttachCount,
		AttachPath:   base + "/sessions/" + s.ID + "/attach",
		ContextRef:   s.ContextRef,
	}
}

// sessionTime renders a timestamp for the wire: UTC, seconds, and "" for the
// zero value.
//
// A string and not a time.Time, because time.Time marshals with whatever
// sub-second precision it holds: the same session would serialise as …:03Z from
// the registry and …:03.412Z after a round-trip through a stamping Update, and
// the desktop would see a changed field on every poll.
func sessionTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// sessionWatchGrace is the slack the readiness watch's own context gets over
// the timeout it passes to WaitReady, so the context is not the thing that
// cancels the wait.
const sessionWatchGrace = 5 * time.Second

// sessionDestroyTimeout bounds a Destroy issued from a background goroutine,
// which has no request context to inherit a deadline from.
const sessionDestroyTimeout = 30 * time.Second

// sessionWaitReady is backend.WaitReady, indirected so a test can drive the
// readiness watch without standing up an HTTP server. Production never
// reassigns it; a test that does must not call t.Parallel.
var sessionWaitReady = backend.WaitReady

// sessionReadyDone is called when a readiness watch has finished and the store
// has been updated. It is nil in production and exists because the watch is a
// goroutine and API is declared in api.go, which this task does not own, so
// there is nowhere to hang a sync.WaitGroup. Task 13 may replace it with one
// when it adds graceful shutdown.
var sessionReadyDone func(sessionID string)

// sessionErrMoved aborts a Store.Update whose precondition no longer holds.
// Returning an error from mutate discards the change, which is exactly the
// behaviour wanted: a watch whose sandbox has been replaced must write nothing.
//
// AMENDED by the f1-f3 fix round (review I-4): three writers use it now, not
// one. The two second writes of a create — the ones that record the endpoint —
// carry the same precondition, because since F1 they can land a whole create
// grace after the caller went away, onto a record a DELETE has since taken
// terminal. The message says "the watch" because that is where it started.
var sessionErrMoved = errors.New("api: the session moved on while the watch was in flight")

// sessionCreateRequest is the body of POST /v1/sessions, contract §3.9. Every
// field but worktreeId is optional and defaulted below.
type sessionCreateRequest struct {
	MachineID             string                                `json:"machineId"`
	WorktreeID            string                                `json:"worktreeId"`
	Harness               string                                `json:"harness"`
	Model                 string                                `json:"model"`
	Effort                string                                `json:"effort"`
	PermissionMode        string                                `json:"permissionMode"`
	OpeningPrompt         string                                `json:"openingPrompt"`
	Title                 string                                `json:"title"`
	Resume                string                                `json:"resume"`
	ContextRef            string                                `json:"contextRef"`
	AuthOnly              bool                                  `json:"authOnly"`
	AgentConnectionID     *string                               `json:"agentConnectionId"`
	ImplementationContext *product.ImplementationContextRequest `json:"implementationContext,omitempty"`
	ProjectRunID          string                                `json:"projectRunId,omitempty"`
	ProjectRunOperationID string                                `json:"projectRunOperationId,omitempty"`
}

// registerSessions mounts the six verbs. Go 1.22's ServeMux matches on method
// and path wildcards, so there is no hand-rolled router and no switch on
// r.Method.
//
// It takes *routes and not *http.ServeMux — group-C §1A. The recorder is how
// contract §10a's audit sees a route at all, and the two read-only patterns
// below are named in catalogReadOnlyRoutes with the reason a GET is safe on
// them; the audit fails until they are.
func (a *API) registerSessions(mux *routes) {
	mux.HandleFunc("POST "+sessionBasePath+"/sessions", a.handleSessionCreate)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions", a.handleSessionList)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions/{id}", a.handleSessionGet)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions/{id}/native-turn", a.handleSessionNativeTurn)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions/{id}/reviewed-context", a.handleSessionReviewedContext)
	mux.HandleFunc("PATCH "+sessionBasePath+"/sessions/{id}", a.handleSessionPatch)
	mux.HandleFunc("DELETE "+sessionBasePath+"/sessions/{id}", a.handleSessionDelete)
	mux.HandleFunc("POST "+sessionBasePath+"/sessions/{id}/restart", a.handleSessionRestart)
}

func (a *API) sessionLog() *slog.Logger {
	if a.deps.Log != nil {
		return a.deps.Log
	}
	return slog.Default()
}

func (a *API) sessionImage() string {
	if a.deps.Image != "" {
		return a.deps.Image
	}
	return "luci/sandbox:dev"
}

// sessionResources refuses a half-filled Resources: Kata sizes a guest VM from
// both numbers and under-requesting either causes a guest OOM.
func (a *API) sessionResources(ctx context.Context) backend.Resources {
	resources := backend.DefaultResources
	if a.deps.Backend != nil && a.deps.Backend.Kind() == backend.KindDocker {
		resources = backend.DefaultLocalResources
	}
	if a.deps.Resources.CPUMillis > 0 && a.deps.Resources.MemoryMiB > 0 {
		resources = a.deps.Resources
	}
	if _, policy, ok := management.ActorFromContext(ctx); ok {
		resources = a.applySessionResourcePolicy(resources, policy)
	}
	return resources
}

func (a *API) applySessionResourcePolicy(resources backend.Resources, policy management.Policy) backend.Resources {
	if a.deps.SessionMemoryUsesAccountPolicy && policy.SessionMemoryMiB > 0 {
		resources.MemoryMiB = policy.SessionMemoryMiB
	}
	if a.deps.SessionCPUUsesAccountPolicy && policy.SessionCPUMillis > 0 {
		resources.CPUMillis = policy.SessionCPUMillis
	}
	return clampSessionResources(resources, policy)
}

func clampSessionResources(resources backend.Resources, policy management.Policy) backend.Resources {
	if policy.SessionMemoryMiB > 0 {
		resources.MemoryMiB = min(resources.MemoryMiB, policy.SessionMemoryMiB)
	}
	if policy.SessionCPUMillis > 0 {
		resources.CPUMillis = min(resources.CPUMillis, policy.SessionCPUMillis)
	}
	return resources
}

func (a *API) sessionReadyTimeout() time.Duration {
	if a.deps.ReadyTimeout > 0 {
		return a.deps.ReadyTimeout
	}
	return backend.ReadyTimeout
}

// sessionDestroyBudget bounds the cleanup of a sandbox a create failed to
// finish. Short on purpose: it runs inside the request that is about to answer
// with the failure, and a daemon that cannot destroy a container in ten seconds
// is one the reaper's rule 3a should take instead of one this handler should
// wait for.
const sessionDestroyBudget = 10 * time.Second

// sessionCreateContext detaches the sandbox create from the caller, and F1 is
// why it exists (docs/verification/2026-09-27-phase-2-results.md).
// handleSessionCreate ran backend.Create on r.Context(), so the cockpit's own
// abort at DEFAULT_TIMEOUT_MS cancelled ContainerCreate or ContainerStart
// mid-flight and left a container `Created` that no rule would take. A create
// is not a read: it makes state outside this process, and state outside this
// process must not be owned by a connection that is free to close.
//
// WithoutCancel rather than Background: the caller's values — a request id, a
// deadline set by something upstream of the cockpit — stay, and only the
// cancellation goes.
//
// Bounded rather than open-ended, and bounded by exactly reaper.CreateGraceFor:
// that is how long the sweep spares a session whose create is still in flight,
// so a create that runs past it is one the reaper has already stopped
// protecting, and two numbers that must agree are derived from one function
// rather than written down twice.
func (a *API) sessionCreateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), reaper.CreateGraceFor(a.sessionReadyTimeout()))
}

// destroyAfterFailedCreate removes whatever a failed create left behind. F1's
// second part: backend.Create can fail after ContainerCreate has succeeded — a
// start that fails, a sandbox that never answers /healthz, a deadline — and the
// container it made is then named by a record that is about to become exited,
// which is terminal and which every reaper rule but 3a skips.
//
// Its own context: the failure the caller is handling may BE createCtx's
// deadline, and a Destroy on a context that is already done destroys nothing.
// Best effort by design — the reaper's rule 3a is the backstop, and the log
// line says so rather than pretending this was the last chance.
func (a *API) destroyAfterFailedCreate(sandboxID, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionDestroyBudget)
	defer cancel()
	if err := a.deps.Backend.Destroy(ctx, sandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
		a.sessionLog().Warn("could not destroy the sandbox of a create that failed; the reaper will reclaim it",
			"sessionId", sessionID, "sandbox", sandboxID, "err", err)
	}
}

// destroyAbandonedSandbox is destroyAfterFailedCreate's twin for a create that
// SUCCEEDED and whose record moved on under it. Same best-effort shape, its own
// log line, because "the create failed" and "the session was killed while it
// ran" are different operational stories and an operator reads the difference.
func (a *API) destroyAbandonedSandbox(sandboxID, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionDestroyBudget)
	defer cancel()
	if err := a.deps.Backend.Destroy(ctx, sandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
		a.sessionLog().Warn("could not destroy the sandbox of a session that moved on mid-create; the reaper will reclaim it",
			"sessionId", sessionID, "sandbox", sandboxID, "err", err)
	}
}

// sessionMovedOnMidCreate answers the second write of a create or of a restart
// — the write that records the endpoint the backend just handed back — when the
// record no longer wants the container that create made. It reports whether it
// answered the request.
//
// The record can move under a detached create three ways: a DELETE takes it
// terminal, a DELETE ...?purge=true removes it, or another restart points it at
// a different sandbox. F1 part 1 widened that window from the instant of the
// caller's abort to the whole create grace, so it is an interleave a user
// reaches by killing a session while it is starting.
//
// Two things follow, and they are the two watchSessionReady already does for
// its own write. Write nothing: an endpoint on an `exited` record contradicts
// Go contract §5.3, is permanent — markExited refuses an already-exited record
// and rule 3a leaves the record alone by design — and defeats
// handleSessionDelete's alreadyTerminal test, which is what stops a daemon
// outage trapping a record that can never be purged. And destroy the container:
// the reaper is the backstop, not the plan, and neither of its rules for this
// shape runs sooner than a sweep away.
func (a *API) sessionMovedOnMidCreate(w http.ResponseWriter, r *http.Request, err error, sandboxID, sessionID string) bool {
	moved := errors.Is(err, sessionErrMoved)
	gone := errors.Is(err, registry.ErrNotFound)
	if !moved && !gone {
		return false
	}
	a.destroyAbandonedSandbox(sandboxID, sessionID)
	if gone {
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
			"No session with that id.", map[string]string{"id": sessionID})
		return true
	}
	// 409, not the 201 or the 200 the happy path writes: nothing the caller
	// asked for is true any more, and the body would otherwise be a terminal
	// record answering a verb that is supposed to have started something.
	httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
		"The session was ended while its sandbox was being created.", map[string]string{"id": sessionID})
	return true
}

// handleSessionCreate is contract §5.1, in its order, and the order is the
// point: resolve before creating so a bad id costs nothing, record before the
// sandbox so a failure leaves evidence rather than an unnameable orphan.
func (a *API) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var req sessionCreateRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	if !a.validateProjectRunSessionRequest(w, &req) {
		return
	}

	req.WorktreeID = strings.TrimSpace(req.WorktreeID)
	if req.WorktreeID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "worktreeId is required.", nil)
		return
	}
	// bindSessionImplementation appends the reviewed prompt below. Keep the
	// original request for exact duplicate admission after concurrent creates.
	projectRunRequest := req

	// 1. The worktree, first.
	path, err := a.deps.Worktrees.Path(ctx, req.WorktreeID)
	if err != nil {
		if errors.Is(err, worktree.ErrUnknownID) || errors.Is(err, worktree.ErrNotADir) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeWorktreeNotFound,
				"No worktree with that id.", map[string]string{"worktreeId": req.WorktreeID})
			return
		}
		a.internal(w, r, "resolve worktree "+req.WorktreeID, err)
		return
	}

	// 2. The harness.
	releaseBranch, ok := a.sessionBranchAdmission(w, r, req.WorktreeID, "")
	if !ok {
		return
	}
	defer releaseBranch()
	// A primary checkout may have switched while initial ownership was read.
	// Re-resolve beneath admission before context or sandbox work begins.
	if path, err = a.deps.Worktrees.Path(ctx, req.WorktreeID); err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "The worktree changed before the session could start. Refresh the project and try again.", nil)
		return
	}
	projectID, branchID := a.sessionLocate(ctx, req.WorktreeID)
	if !a.validateSwarmWorktreeSession(w, r, projectID, req) {
		return
	}
	if a.reuseProjectRunSession(w, r, req, projectID) {
		return
	}
	if req.ProjectRunID != "" {
		queueID, err := a.deps.Product.SwarmQueueIDForOperation(projectID, req.ProjectRunID, req.ProjectRunOperationID)
		if err != nil {
			a.internal(w, r, "resolve queued operation ownership", err)
			return
		}
		if queueID != "" {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "This task is owned by the durable queue. Use its queue controls; its exact child will start when admitted.", nil)
			return
		}
	}

	harnessID := harness.ID(strings.TrimSpace(req.Harness))
	if harnessID == "" {
		harnessID = harness.ClaudeCode
	}
	desc, known := harness.Lookup(harnessID)
	if !known {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnknownHarness,
			"No harness with that id.", map[string]string{"harness": string(harnessID)})
		return
	}
	if req.AuthOnly && (harnessID != harness.Codex || req.Resume != "" || req.OpeningPrompt != "") {
		httpx.WriteProblem(w, 400, httpx.CodeConflict, "Device sign-in requires a fresh Codex session without an opening prompt.", nil)
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = desc.DefaultModel
	}
	mode := harness.PermissionMode(strings.TrimSpace(req.PermissionMode))
	if mode == "" {
		mode = harness.PermissionAsk
	}
	effort := harness.Effort(strings.TrimSpace(req.Effort))

	// 2a. The conversation. `resume` names a COCKPIT session, which is all the
	// desktop has; the harness needs that session's own conversation id, and
	// sessionResolveResume looks it up on the record. For a harness with no
	// conversation the raw value goes to Build untranslated, and Build refuses
	// it — unavailable, or cannot resume — in the contract's order. The source
	// may be in this worktree (Resume) or another (Fork); step 3a is the
	// difference.
	resume := strings.TrimSpace(req.Resume)
	if desc.NativeState() && resume != "" {
		if !sessionRestarts.acquire(resume) {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "The source session is already changing; try after it stops.", nil)
			return
		}
		defer sessionRestarts.release(resume)
	}
	var conversation, conversationStore string
	var source sessionResumeSource
	if harnessID == harness.Codex {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
		// A concurrent create may have persisted this exact reserved child
		// while this request waited for the subscription account lock.
		if a.reuseProjectRunSession(w, r, req, projectID) {
			return
		}
	}
	if sessionHasConversation(desc) || desc.NativeState() {
		if resume != "" {
			var ok bool
			if source, ok = a.sessionResolveResume(w, r, resume, harnessID); !ok {
				return
			}
			resume = source.conversation
		}
		// Minted here and TOLD to the CLI with --session-id. The alternative,
		// learning the id the CLI picked, means reading its output.
		conversation = ids.UUID()
	}

	var account *codexConnection
	var authMode string
	var sourceSession registry.Session
	if source.id != "" {
		sourceSession, err = a.deps.Store.Get(source.id)
		if err != nil {
			codexAccountProblem(w, err)
			return
		}
		if !validateProjectRunResume(w, req, sourceSession) {
			return
		}
	}
	selection := req.AgentConnectionID
	reviewedCapture, contextOK := a.bindSessionImplementation(w, r, &req, path, sourceSession)
	if !contextOK {
		return
	}
	var implementationContext *registry.ImplementationContext
	if reviewedCapture != nil {
		implementationContext = reviewedCapture.Binding
	}
	if req.AuthOnly {
		if selection != nil && *selection != "" {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "Device sign-in cannot use an API or gateway route. Start a fresh subscription sign-in instead.", nil)
			return
		}
		empty := ""
		selection = &empty
	}
	agentConnection, agentEnv, boundModel, err := a.resolveSessionAgent(string(harnessID), selection, sourceSession, req.Model)
	if err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
		return
	}
	if boundModel != "" {
		model = boundModel
	}
	authMode = agentConnection.AuthMode
	if harnessID == harness.Codex && (agentConnection.Kind == "legacy" || agentConnection.Kind == "codex-subscription") {
		account, authMode, err = a.selectCodexConnection(ctx, sourceSession, req.AuthOnly, "")
		if err != nil {
			codexAccountProblem(w, err)
			return
		}
		if agentConnection.Kind == "codex-subscription" && account == nil {
			codexAccountProblem(w, errCodexDisconnected)
			return
		}
	}
	buildHarness := harness.Build
	taskWorker := false
	if req.ProjectRunID != "" {
		ownedRun, ownedOperation, ownerErr := a.deps.Product.ProjectSwarmWorktreeOwner(ctx, projectID, req.WorktreeID)
		if ownerErr != nil {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, ownerErr.Error(), nil)
			return
		}
		if ownedRun != "" {
			if ownedRun != req.ProjectRunID || ownedOperation != req.ProjectRunOperationID {
				httpx.WriteProblem(w, 409, httpx.CodeConflict, "This task checkout belongs to another reviewed operation.", nil)
				return
			}
			buildHarness = harness.BuildTask
			taskWorker = true
		}
	}
	command, argv, err := buildHarness(harness.Request{
		DockerBoundary:  a.deps.Backend.Kind() == backend.KindDocker,
		ProjectBrief:    true,
		ReviewedContext: implementationContext != nil,
		Harness:         harnessID,
		Model:           model,
		Effort:          effort,
		Permission:      mode,
		OpeningPrompt:   req.OpeningPrompt,
		Conversation:    conversation,
		Resume:          resume,
	})
	if err != nil {
		a.writeHarnessProblem(w, r, harnessID, desc, err)
		return
	}
	allocatedResources := a.sessionResources(ctx)
	var executionLeaseID string
	var executionActor management.ExecutionActorRef
	var reviewedContractRun *product.ProjectRun
	if req.ProjectRunID != "" && a.deps.Product != nil {
		retained, readErr := a.deps.Product.GetProjectSwarmRun(projectID, req.ProjectRunID)
		if readErr != nil {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "The reviewed run is unavailable. Inspect its saved status before starting work.", nil)
			return
		}
		if retained.Input.OrchestrationContract != nil {
			reviewedContractRun = &retained
		}
	}
	if reviewedContractRun != nil && a.deps.ResourceSnapshot == nil {
		httpx.WriteProblem(w, 503, httpx.CodeConflict, "Current execution authority is unavailable. The reviewed contract cannot start a session yet.", nil)
		return
	}
	if a.deps.ResourceSnapshot != nil {
		if a.deps.Product == nil {
			httpx.WriteProblem(w, 503, httpx.CodeConflict, "Execution policy is not available. Try again after the workspace finishes starting.", nil)
			return
		}
		executionActor, err = a.captureQueuedActor(ctx, projectID)
		if err != nil {
			httpx.WriteProblem(w, 403, httpx.CodeConflict, err.Error(), nil)
			return
		}
		authority, authErr := a.authorizeExecution(ctx, executionActor, string(harnessID), agentConnection.ID != "")
		if authErr != nil {
			httpx.WriteProblem(w, 403, httpx.CodeConflict, authErr.Error(), nil)
			return
		}
		if reviewedContractRun != nil && !product.OrchestrationAuthorityMatches(*reviewedContractRun, authority) {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "The current team grant or execution policy differs from this reviewed contract. Prepare a new contract before starting work.", nil)
			return
		}
		policy, policyErr := a.deps.Product.SwarmSchedulerPolicy()
		if policyErr != nil {
			a.internal(w, r, "read execution policy", policyErr)
			return
		}
		provider, limit := providerCapacity(product.ProjectRunRoute{Harness: string(harnessID), Connection: agentConnection}, policy)
		executionLeaseID = ids.New("manual")
		decision, reserveErr := a.execution.reserve(ctx, executionLeaseID, projectID, provider, limit, executionActor, authority, policy)
		if reserveErr != nil {
			a.internal(w, r, "reserve execution capacity", reserveErr)
			return
		}
		if decision.State != "admitted" {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, decision.Message, map[string]string{"reasonCode": decision.Code})
			return
		}
		allocatedResources = decision.Resources
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.execution.release(cleanup, executionLeaseID); err != nil && !errors.Is(err, ErrExecutionLeaseHeld) {
				a.sessionLog().Warn("retain execution capacity after unresolved cleanup", "err", err)
			}
		}()
	}
	if conversation != "" && !desc.NativeState() {
		if conversationStore, err = a.sessionConversationStore(req.WorktreeID); err != nil {
			a.internal(w, r, "prepare the conversation store for worktree "+req.WorktreeID, err)
			return
		}
	}
	// 2b. The sandbox's environment and mounts. Resolved here, before the
	// record, because both run git or write on the host, the window between
	// the record and its sandbox is the one the reaper has to tolerate, and a
	// failure here must leave nothing behind.
	lineEndings, err := a.sessionGitLineEndings(ctx)
	if err != nil {
		a.internal(w, r, "read how the host's git converts line endings, for worktree "+req.WorktreeID, err)
		return
	}
	gitFiles, err := a.sessionGitGlobalFiles(ctx, req.WorktreeID)
	if err != nil {
		a.internal(w, r, "find the host git's global excludes and attributes files for worktree "+req.WorktreeID, err)
		return
	}
	worktreeAdmin, err := sessionWorktreeAdmin(path)
	if err != nil {
		a.internal(w, r, "prepare the linked-worktree metadata of worktree "+req.WorktreeID, err)
		return
	}
	sandboxEnv := sessionEnv(a.sessionGitIdentity(ctx, req.WorktreeID), lineEndings, gitFiles, a.deps.CABundle)
	for key, value := range agentEnv {
		sandboxEnv[key] = value
	}
	if harnessID == harness.OpenCode && sandboxEnv["OPENCODE_CONFIG_CONTENT"] == "" {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "Configure an explicit local or compatible model in Settings before starting OpenCode.", nil)
		return
	}
	argv = codexSubscriptionArgs(argv, sandboxEnv, authMode)
	if a.deps.Backend.Kind() == backend.KindDocker {
		nativeDockerEndpoint(sandboxEnv)
	}
	if harnessID == harness.OpenCode {
		if implementationContext != nil {
			if err := clearReviewedOpenCodeInstructions(sandboxEnv); err != nil {
				httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
				return
			}
		}
		if err := prepareLocalOpenCodeCodingContext(ctx, sandboxEnv, a.deps.Backend); err != nil {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
			return
		}
	}
	argv = nativeArgs(string(harnessID), argv, sandboxEnv, req.AuthOnly)
	nativeSandboxEnvironment(sandboxEnv)

	// 3. Ids and the secret.
	sessionID := ids.New("s")
	var runAdmission product.ProjectRunSessionAdmission
	if req.ProjectRunID != "" {
		var ok bool
		runAdmission, ok = a.beginProjectRunSession(w, r, projectRunRequest, registry.Session{
			ProjectID: projectID, WorktreeID: req.WorktreeID, BranchID: branchID,
			Harness: string(harnessID), Model: model, Effort: string(effort), PermissionMode: string(mode),
			AuthMode: authMode, CodexConnectionID: codexConnectionID(account),
			AgentConnectionID: agentConnection.ID, AgentConnectionRevision: agentConnection.Revision,
			AgentConnectionKind: agentConnection.Kind, AgentConnectionProtocol: agentConnection.Protocol,
			ImplementationContext: implementationContext, ProjectRunID: req.ProjectRunID,
			ProjectRunOperationID: req.ProjectRunOperationID,
		})
		if !ok {
			return
		}
		defer runAdmission.Release()
		sessionID = runAdmission.SessionID
	}
	if executionLeaseID != "" {
		if err := a.execution.bind(ctx, executionLeaseID, sessionID); err != nil {
			a.internal(w, r, "bind execution capacity", err)
			return
		}
	}
	token := ids.Token()
	if harnessID == harness.OpenCode {
		// This authenticates the upstream loopback server to sandboxd. It is
		// private to this sandbox generation and never enters the REST view.
		sandboxEnv["OPENCODE_SERVER_PASSWORD"] = token
	}

	// 3a. A Fork carries its source's conversation into this worktree. Last
	// before the record, so the one failure left that must undo it is the
	// record's own. sessions_fork.go says why it is a staged copy.
	var carried backend.Mount
	var carriedConversation, staging string
	if source.conversation != "" && source.worktreeID != req.WorktreeID && !desc.NativeState() {
		if carried, staging, err = a.sessionCarry(source, sessionID, conversationStore); err != nil {
			a.internal(w, r, "carry the conversation of session "+source.id+" into worktree "+req.WorktreeID, err)
			return
		}
		carriedConversation = source.conversation
	}
	sandboxMounts := sessionMounts(path, worktreeAdmin, conversationStore, carried, a.deps.CABundle, gitFiles)
	if harnessID == harness.Shell {
		home, err := a.prepareShellHome(req.WorktreeID, sessionID)
		if err != nil {
			a.internal(w, r, "prepare private terminal home", err)
			return
		}
		shellEnvironment(sandboxEnv)
		sandboxMounts = shellMount(sandboxMounts, home)
	}
	if desc.NativeState() {
		providerStore, err := a.prepareNativeState(req.WorktreeID, string(harnessID), conversation, source, false)
		if err != nil {
			a.internal(w, r, "prepare private provider state", err)
			return
		}
		if err := seedCodexConnection(providerStore, account); err != nil {
			codexAccountProblem(w, err)
			return
		}
		sandboxMounts = nativeMount(sandboxMounts, providerStore)
		nativeEnvironment(string(harnessID), sandboxEnv, mode)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = desc.Name + " · " + filepath.Base(filepath.Clean(path))
	}
	// Contract §3.4 and task 10's LocalMachineID: the one machine Phase 2 has
	// is the literal "local". A second default here would be a second answer.
	machineID := strings.TrimSpace(req.MachineID)
	if machineID == "" {
		machineID = LocalMachineID
	}
	if !a.validateSessionImplementation(w, projectID, req.WorktreeID, path, implementationContext) {
		return
	}
	var reviewedDescriptor *registry.ReviewedContext
	if taskWorker {
		mount, outputErr := a.prepareTaskHandoffOutput(sessionID)
		if outputErr != nil {
			a.internal(w, r, "prepare isolated task handoff output", outputErr)
			return
		}
		sandboxMounts = append(sandboxMounts, mount)
	}
	if reviewedCapture != nil {
		var mount backend.Mount
		reviewedDescriptor, mount, err = a.prepareReviewedContext(sessionID, reviewedCapture)
		if err != nil {
			a.internal(w, r, "capture the reviewed session context", err)
			return
		}
		sandboxMounts = append(sandboxMounts, mount)
	}

	var revalidateExecution func(context.Context) error
	var markExecutionStart func(context.Context) error
	var markExecutionSettled func(context.Context, string) error
	if executionLeaseID != "" {
		markExecutionStart = func(ctx context.Context) error {
			return a.execution.markBackendStarting(ctx, executionLeaseID, sessionID, sessionID)
		}
		markExecutionSettled = func(ctx context.Context, sandboxID string) error {
			return a.execution.markBackendSettled(ctx, executionLeaseID, sessionID, sandboxID)
		}
		revalidateExecution = func(ctx context.Context) error {
			current, err := a.authorizeExecution(ctx, executionActor, string(harnessID), agentConnection.ID != "")
			if err != nil {
				return admissionFailure("execution_authority_changed", err)
			}
			if reviewedContractRun != nil && !product.OrchestrationAuthorityMatches(*reviewedContractRun, current) {
				return admissionFailure("execution_contract_authority_changed", errors.New("The current team grant or execution policy differs from this reviewed contract."))
			}
			if reviewedContractRun != nil {
				if err := a.deps.Product.ValidateRunExecutionContract(projectID, *reviewedContractRun); err != nil {
					return admissionFailure("execution_contract_changed", err)
				}
			}
			resources := a.sessionResources(ctx)
			if executionActor.UserID != "" {
				resources = a.applySessionResourcePolicy(resources, current.Policy)
			}
			if resources != allocatedResources {
				return admissionFailure("resource_policy_changed", errors.New("The worker resource policy changed before allocation."))
			}
			policy, err := a.deps.Product.SwarmSchedulerPolicy()
			if err != nil {
				return admissionFailure("resource_policy_unavailable", err)
			}
			provider, limit := providerCapacity(product.ProjectRunRoute{Harness: string(harnessID), Connection: agentConnection}, policy)
			decision, err := a.execution.reserve(ctx, executionLeaseID, projectID, provider, limit, executionActor, current, policy)
			if err != nil {
				return admissionContextFailure(ctx, err)
			}
			if decision.State != "admitted" {
				return admissionDecisionFailure(decision)
			}
			return nil
		}
	}
	// The typed allocator is shared with durable queued dispatch. It records
	// ownership before backend effects and retains uncertain outcomes.
	stored, err := a.allocatePreparedSession(ctx, preparedSessionAllocation{
		Record: registry.Session{
			ID:                      sessionID,
			Title:                   title,
			MachineID:               machineID,
			ProjectID:               projectID,
			BranchID:                branchID,
			WorktreeID:              req.WorktreeID,
			Harness:                 string(harnessID),
			Model:                   model,
			Effort:                  string(effort),
			PermissionMode:          string(mode),
			OpeningPrompt:           req.OpeningPrompt,
			AuthOnly:                req.AuthOnly,
			AuthMode:                authMode,
			CodexConnectionID:       codexConnectionID(account),
			AgentConnectionID:       agentConnection.ID,
			AgentConnectionRevision: agentConnection.Revision,
			AgentConnectionKind:     agentConnection.Kind,
			AgentConnectionProtocol: agentConnection.Protocol,
			Command:                 command,
			Args:                    argv,
			ConversationID:          conversation,
			Status:                  registry.StatusStarting,
			Backend:                 string(a.deps.Backend.Kind()),
			SandboxID:               sessionID,
			AttachToken:             token,
			ContextRef:              req.ContextRef,
			ImplementationContext:   implementationContext,
			ReviewedContext:         reviewedDescriptor,
			ProjectRunID:            req.ProjectRunID,
			ProjectRunOperationID:   req.ProjectRunOperationID,

			CarriedConversationID: carriedConversation,
		},
		Spec:    backend.Spec{ID: sessionID, SessionID: sessionID, Image: a.sessionImage(), Command: command, Args: argv, Env: sandboxEnv, AttachToken: token, Resources: allocatedResources, WorkDir: sessionWorkDir, Mounts: sandboxMounts},
		Account: account, Revalidate: runAdmission.Revalidate, BeforeAllocate: revalidateExecution, BeforeBackendStart: markExecutionStart, AfterBackendCreate: markExecutionSettled, Staging: staging, HasReviewedContext: reviewedDescriptor != nil, ExecutionLeaseID: executionLeaseID,
	})
	if err != nil {
		a.writeSessionAllocationProblem(w, r, err)
		return
	}

	// 6. Return immediately. The desktop opens the attach socket, which retries
	// while the status is starting.
	w.Header().Set("Location", sessionBasePath+"/sessions/"+sessionID)
	httpx.WriteJSON(w, http.StatusCreated, a.sessionJSON(stored, sessionBasePath))
}

// sessionLocate finds the project and branch a worktree belongs to, through
// task 10's Enumerator. Both ids are empty when the worktree is not under one
// of the enumerated projects; an empty id is honest, a guessed one is not.
// New rejects a nil Projects, so the nil check is for a hand-built API only.
func (a *API) sessionLocate(ctx context.Context, worktreeID string) (projectID, branchID string) {
	loc := a.deps.Projects
	if loc == nil {
		return "", ""
	}
	projects, err := loc.Projects(ctx)
	if err != nil {
		a.sessionLog().Warn("enumerate projects while creating a session", "err", err)
		return "", ""
	}
	for _, p := range projects {
		worktrees, err := loc.Worktrees(ctx, p.ID)
		if err != nil {
			a.sessionLog().Warn("enumerate worktrees while creating a session", "projectId", p.ID, "err", err)
			continue
		}
		for _, wt := range worktrees {
			if wt.ID == worktreeID {
				return wt.ProjectID, wt.BranchID
			}
		}
	}
	return "", ""
}

// sessionResolveResume turns the `resume` field — the id of the cockpit session
// being resumed — into the conversation id the harness knows it by, which is on
// that session's record. The desktop sends the cockpit id and always has; what
// was wrong is that this file then passed it to `claude --resume` unchanged,
// and the CLI has never heard of s-3f2a91c0.
//
// Every refusal is a sentence the user can act on. The alternative is worse
// than an error: a resume the CLI cannot honour starts a sandbox whose
// terminal prints one line and exits.
//
// Resume and Fork both come through here, and are refused alike: a Fork is a
// resume whose request names another worktree. That used to be refused, when
// the per-worktree store left a fork nothing to read; step 3a of the create
// now carries the source's transcript across (sessions_fork.go), so where the
// source ran is returned instead of checked.
func (a *API) sessionResolveResume(w http.ResponseWriter, r *http.Request, sourceID string, harnessID harness.ID) (sessionResumeSource, bool) {
	details := map[string]string{"resume": sourceID}
	src, err := a.deps.Store.Get(sourceID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"The session to resume is no longer in the list, so there is no conversation to continue. Start a new session instead.",
				details)
			return sessionResumeSource{}, false
		}
		a.internal(w, r, "read the session to resume", err)
		return sessionResumeSource{}, false
	}
	if src.Harness != string(harnessID) {
		name := src.Harness
		if d, ok := harness.Lookup(harness.ID(src.Harness)); ok {
			name = d.Name
		}
		details["harness"] = src.Harness
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"A session resumes on the harness it ran, and that one ran "+name+".", details)
		return sessionResumeSource{}, false
	}
	if src.ConversationID == "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"That session has no saved conversation to continue: it was started before the app recorded one. Start a new session instead.",
			details)
		return sessionResumeSource{}, false
	}
	if nativeHarness(src.Harness) && src.Status != registry.StatusExited {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"Stop this provider session before resuming or forking it so its saved database can be copied consistently.", details)
		return sessionResumeSource{}, false
	}
	conversation, found, err := a.sessionResumableConversation(src)
	if err != nil {
		a.internal(w, r, "find the conversation of the session to resume", err)
		return sessionResumeSource{}, false
	}
	if !found {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"Nothing has been said in that session yet, so there is no conversation to continue. Start a new session instead.",
			details)
		return sessionResumeSource{}, false
	}
	return sessionResumeSource{id: src.ID, worktreeID: src.WorktreeID, conversation: conversation}, true
}

// sessionResumableConversation is the conversation a resume of src continues:
// the newest of src's own conversations the CLI has written a transcript for.
// Its current one first; then, newest first, the ones its restarts left, so a
// resume after a restart and before anyone speaks continues the conversation
// the restart cleared instead of one that does not exist yet.
//
// found is false only when every one of them is certainly absent — the state
// `claude --resume` answers with "No conversation found with session ID" and an
// exit, which is why the caller refuses before any sandbox starts. A
// conversation whose transcript cannot be looked at is taken as it is, and the
// CLI is left to judge: refusing on a failed look would make resume depend on
// it, which OD-8 forbids.
func (a *API) sessionResumableConversation(src registry.Session) (conversation string, found bool, err error) {
	if nativeHarness(src.Harness) {
		return a.nativeResumable(src)
	}
	store, err := a.sessionConversationStorePath(src.WorktreeID)
	if err != nil {
		return "", false, err
	}
	candidates := []string{src.ConversationID}
	for i := len(src.EarlierConversationIDs) - 1; i >= 0; i-- {
		candidates = append(candidates, src.EarlierConversationIDs[i])
	}
	for _, id := range candidates {
		written, err := sessionTranscriptState(store, id)
		if err != nil {
			a.sessionLog().Warn("resume: cannot tell whether a transcript was written; letting the CLI decide",
				"sessionId", src.ID, "err", err)
			return id, true, nil
		}
		if written {
			return id, true, nil
		}
	}
	return "", false, nil
}

// markSessionExited is the one place a session becomes terminal without a
// sandbox to destroy. It clears the endpoint and the token: the port is about
// to be handed back to the OS and may belong to another container within
// seconds, and a token for a sandbox that no longer exists is only a liability.
// An existing reason is kept, because the first reason is the true one.
func (a *API) markSessionExited(sessionID, reason string) {
	if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		if sess.ExitReason == "" {
			sess.ExitReason = reason
		}
		sess.Endpoint = ""
		sess.AttachToken = ""
		return nil
	}); err != nil && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Error("mark the session exited", "sessionId", sessionID, "err", err)
	}
}

// watchSessionReady is contract §5.1 step 7. It owns exactly one sandbox — the
// one whose id it was given — and it writes to the record only while the record
// still points at that sandbox. Without that guard a slow watch left over from
// before a restart marks the NEW sandbox running with the OLD endpoint, or
// destroys it outright.
//
// A sandbox with no endpoint gets its own path and not the timeout path. See
// the comment on that branch: they are different failures and only one of them
// is this task's to resolve.
func (a *API) watchSessionReady(sessionID, sandboxID, endpoint string) {
	defer func() {
		if done := sessionReadyDone; done != nil {
			done(sessionID)
		}
	}()

	// NO ENDPOINT IS NOT "NOT READY YET", AND THE DIFFERENCE IS A FABRICATED
	// STATUS. backend.Sandbox documents Endpoint as blank whenever NonLoopback
	// is set, so this is the LAN-published container: it is RUNNING, the
	// control plane simply cannot reach it. Waiting anyway would have
	// backend.WaitReady build http:///healthz, fail every poll for the whole
	// timeout, and hand this function a readiness error it would read as "the
	// sandbox never became ready" — destroying a healthy container and telling
	// the user their agent finished when it never started, with the real cause
	// (the daemon published it off loopback) lost.
	//
	// That is the fifth thing in this phase to refuse to translate one fact
	// into a different one: task 5's List keeps such a container in the slice
	// with an empty Endpoint rather than faking StateStopped, and group-C
	// ruling 10 hands it to task 13's reaper, which destroys it and logs that
	// destruction distinctly so "why did my sandbox vanish" has an answer.
	// Inventing "exited" here is the same lie one layer up.
	//
	// So: destroy nothing, write nothing, and leave the record at starting —
	// which is true, the sandbox has not answered /healthz — for the reaper to
	// resolve. The log line is the durable record of why, and it is tagged
	// SECURITY because a sandbox published off loopback is a shell on the LAN.
	if endpoint == "" {
		a.sessionLog().Error("SECURITY: the sandbox has no loopback endpoint, so nothing can attach to it",
			"sessionId", sessionID, "sandboxId", sandboxID,
			"detail", "the record stays at starting; group-C ruling 10 gives this container to the reaper, which destroys it and says so")
		return
	}

	timeout := a.sessionReadyTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout+sessionWatchGrace)
	defer cancel()

	if err := sessionWaitReady(ctx, endpoint, timeout); err != nil {
		a.sessionLog().Warn("the sandbox never became ready",
			"sessionId", sessionID, "sandboxId", sandboxID, "timeout", timeout, "err", err)

		// Destroying is right regardless of what the record now says: a sandbox
		// that never answered /healthz has no user and would be reaped anyway.
		// This is one of the three places contract §5.2 allows a Destroy.
		dctx, dcancel := context.WithTimeout(context.Background(), sessionDestroyTimeout)
		if derr := a.deps.Backend.Destroy(dctx, sandboxID); derr != nil && !errors.Is(derr, backend.ErrNotFound) {
			a.sessionLog().Error("destroy a sandbox that never became ready", "sandboxId", sandboxID, "err", derr)
		}
		dcancel()

		if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
			if sess.SandboxID != sandboxID {
				return sessionErrMoved
			}
			sess.Status = registry.StatusExited
			// The contract quotes this string in §5.1 step 7. It is the text
			// the cockpit renders, so it is transcribed, not paraphrased.
			sess.ExitReason = "sandbox never became ready"
			sess.Endpoint = ""
			sess.AttachToken = ""
			return nil
		}); err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
			a.sessionLog().Error("mark the session exited", "sessionId", sessionID, "err", err)
		}
		return
	}

	if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		// starting is the only status this transition is allowed to leave. A
		// session killed while the watch was in flight stays exited.
		if sess.SandboxID != sandboxID || sess.Status != registry.StatusStarting {
			return sessionErrMoved
		}
		sess.Status = registry.StatusRunning
		sess.Endpoint = endpoint
		sess.ExitReason = ""
		sess.StartFailure = registry.SessionStartFailure{}
		sess.StartAdmissionEvidence = registry.StartAdmissionEvidence{}
		return nil
	}); err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Error("mark the session running", "sessionId", sessionID, "err", err)
	}
}

// writeHarnessProblem keeps the three harness failures distinct: 400 means the
// request was wrong, 409 means the request was reasonable and the image is not
// ready for it, and the cockpit renders them differently.
func (a *API) writeHarnessProblem(w http.ResponseWriter, r *http.Request, id harness.ID, desc harness.Descriptor, err error) {
	details := map[string]string{"harness": string(id)}
	switch {
	case errors.Is(err, harness.ErrTaskPermission):
		httpx.WriteProblem(w, http.StatusConflict, "task_permission_requires_interactive", harness.TaskPermissionNotice, nil)
	case errors.Is(err, harness.ErrUnknownHarness):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnknownHarness, "No harness with that id.", details)
	case errors.Is(err, harness.ErrUnavailable):
		if desc.Note != "" {
			details["note"] = desc.Note
		}
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeHarnessUnavailable,
			"That harness is not installed in the sandbox image.", details)
	case errors.Is(err, harness.ErrUnsupportedOption), errors.Is(err, harness.ErrUnknownModel):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnsupportedOption, err.Error(), details)
	default:
		a.internal(w, r, "build the argv for harness "+string(id), err)
	}
}

// writeBackendProblem dispatches with errors.Is and never string-matches, which
// is the whole reason task 4 maps every non-101 handshake to a sentinel.
func (a *API) writeBackendProblem(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, backend.ErrUnavailable):
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"The sandbox backend is unavailable.", nil)
	case errors.Is(err, backend.ErrUnsupported):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnsupportedOption,
			"The backend cannot honour part of that request.", nil)
	case errors.Is(err, backend.ErrAlreadyExists):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "That sandbox already exists.", nil)
	case errors.Is(err, backend.ErrNotRunning), errors.Is(err, backend.ErrNotFound):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning, "The sandbox is not running.", nil)
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		// Loud, and never 200: a published 0.0.0.0 port is a shell on the LAN.
		// The extra line is deliberate duplication — a.internal logs at the
		// same moment, but this one is greppable as SECURITY and names the op.
		a.sessionLog().Error("SECURITY: a sandbox port is not bound to loopback", "op", op, "err", err)
		a.internal(w, r, op, err)
	default:
		a.internal(w, r, op, err)
	}
}

// sessionPatchRequest is the body of PATCH /v1/sessions/{id}. Title is a
// pointer so an absent field and an empty string are distinguishable: the
// decoder rejects any OTHER field, so a body with no title at all is a request
// that asks for nothing and gets 400 rather than a silent 200.
type sessionPatchRequest struct {
	Title *string `json:"title"`
}

// sessionListResponse is the envelope of contract §3.11. An envelope rather
// than a bare array because a top-level JSON array cannot grow a field later.
type sessionListResponse struct {
	Sessions []sessionJSON `json:"sessions"`
}

// sessionStatuses is the closed set Phase 2 can actually derive. The other
// three in docs/AGENTIC-UX.md §4 rule 8 need agent output, which this phase
// does not read, so a filter naming one of them is a bad request rather than a
// silently empty rail.
var sessionStatuses = map[registry.Status]bool{
	registry.StatusStarting: true,
	registry.StatusRunning:  true,
	registry.StatusIdle:     true,
	registry.StatusExited:   true,
}

func (a *API) handleSessionList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	worktreeID := strings.TrimSpace(query.Get("worktreeId"))

	wanted := map[registry.Status]bool{}
	for _, raw := range query["status"] {
		status := registry.Status(strings.TrimSpace(raw))
		if !sessionStatuses[status] {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"status must be one of starting, running, idle or exited.",
				map[string]string{"status": raw})
			return
		}
		wanted[status] = true
	}

	all, err := a.deps.Store.List()
	if err != nil {
		a.internal(w, r, "list sessions", err)
		return
	}

	// Never nil: null and [] are different values to the desktop's decoder.
	out := make([]sessionJSON, 0, len(all))
	for _, sess := range all {
		if worktreeID != "" && sess.WorktreeID != worktreeID {
			continue
		}
		if len(wanted) > 0 && !wanted[sess.Status] {
			continue
		}
		out = append(out, a.sessionJSON(sess, sessionBasePath))
	}

	// Descending, per §3.11, sorted here rather than by reversing the store's
	// ascending order: the API's contract must not depend on the store's. The
	// keys are the rendered strings, which sort correctly because every one is
	// RFC 3339 in UTC and so lexicographic order is chronological order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})

	httpx.WriteJSON(w, http.StatusOK, sessionListResponse{Sessions: out})
}

// sessionByID is the 404 every single-session verb shares.
func (a *API) sessionByID(w http.ResponseWriter, r *http.Request) (registry.Session, bool) {
	id := r.PathValue("id")
	sess, err := a.deps.Store.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return registry.Session{}, false
		}
		a.internal(w, r, "read the session", err)
		return registry.Session{}, false
	}
	return sess, true
}

func (a *API) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sessionDetailJSON{sessionJSON: a.sessionJSON(sess, sessionBasePath), StartAdmissionEvidence: sessionAdmissionEvidence(r, sess)})
}

func (a *API) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	var req sessionPatchRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Title == nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"title is the only patchable field and it is required.", nil)
		return
	}
	title := strings.TrimSpace(*req.Title)
	if title == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "title cannot be empty.", nil)
		return
	}
	id := r.PathValue("id")
	// Renaming an exited session is allowed, per §3.13: the record outlives the
	// sandbox and the user is naming a transcript.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.Title = title
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "rename the session", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.sessionJSON(updated, sessionBasePath))
}

// sessionIDSet serialises one verb per session id. It is a package-level value
// rather than a field on API because api.go belongs to task 10 and this task
// adds only Deps fields to it. Session ids come from ids.New over crypto/rand,
// so two APIs in one process cannot collide on a real id; two tests that
// hand-write the same id would serialise, which is the correct behaviour
// anyway.
type sessionIDSet struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (s *sessionIDSet) acquire(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The init runs BEFORE the read, not after it. Reading a nil map is legal
	// and returns false, so the other order looks harmless — but it is the
	// write on the next line that panics, and the zero value is then only safe
	// by accident of how the package variable below happens to be declared.
	if s.ids == nil {
		s.ids = map[string]bool{}
	}
	if s.ids[id] {
		return false
	}
	s.ids[id] = true
	return true
}

func (s *sessionIDSet) release(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, id)
}

// sessionRestarts is §3.14's "409 conflict if a restart is already in flight".
// Two concurrent restarts would otherwise both destroy and both create, and the
// loser's sandbox becomes an orphan holding the session's only live endpoint.
//
// Declared as the zero value, with no map literal: acquire initialises it, and
// a second initialisation here would make that branch dead code that nothing
// ever exercises — which is how a nil-map panic survives a green suite.
var sessionRestarts = &sessionIDSet{}

// handleSessionRestart is §3.14: same config, clean context. It keeps the
// session id, rebuilds the sandbox from the STORED command and args — except
// that a harness with a conversation gets a new one, see below — and mints a
// new attach token. Resume is a different verb — POST /v1/sessions with resume
// set — because resuming continues an old transcript and conflating the two
// makes restart lossy.
func (a *API) handleSessionRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionRestarts.acquire(id) {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"This session is already starting or restarting. Wait for its current operation before retrying.", map[string]string{"id": id})
		return
	}
	defer sessionRestarts.release(id)

	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	// Managed restarts share the exact child's durable lifecycle fence with
	// trusted capacity observation. Read the current row again under that fence
	// before reserving or replacing any generation.
	if a.execution != nil && a.deps.ResourceSnapshot != nil {
		unlock, err := a.execution.lockChildLifecycle(r.Context(), id)
		if err != nil {
			httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This session's current allocation is being observed or changed. Retry after its current operation.", nil)
			return
		}
		defer unlock()
		sess, ok = a.sessionByID(w, r)
		if !ok {
			return
		}
	}
	if sess.ProjectRunID != "" || sess.ProjectRunOperationID != "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "Reserve a new coding operation in the project run before retrying. The original attempt remains in the run history.", nil)
		return
	}
	ctx := r.Context()
	if a.deps.Product != nil && sess.BranchID == "" {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "This older session has no saved branch. Start a fresh session on the selected branch instead.", nil)
		return
	}
	releaseBranch, admitted := a.sessionBranchAdmission(w, r, sess.WorktreeID, sess.BranchID)
	if !admitted {
		return
	}
	defer releaseBranch()
	var account *codexConnection
	authMode := sess.AuthMode
	selection := sess.AgentConnectionID
	agentConnection, agentEnv, model, err := a.resolveSessionAgent(sess.Harness, &selection, sess, sess.Model)
	if err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
		return
	}
	if agentConnection.AuthMode != "" {
		authMode = agentConnection.AuthMode
	}
	restartAdmission, err := a.prepareSessionRestart(ctx, sess, agentConnection)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, management.ErrExecutionAuthority) {
			status = http.StatusForbidden
		}
		httpx.WriteProblem(w, status, httpx.CodeConflict, err.Error(), nil)
		return
	}
	defer restartAdmission.release()
	if sess.Harness == "codex" && (agentConnection.Kind == "legacy" || agentConnection.Kind == "codex-subscription") {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
		var err error
		account, authMode, err = a.selectCodexConnection(ctx, sess, false, sess.ID)
		if err != nil {
			codexAccountProblem(w, err)
			return
		}
		if agentConnection.Kind == "codex-subscription" && account == nil {
			codexAccountProblem(w, errCodexDisconnected)
			return
		}
	}

	// The spec is rebuilt, not replayed, so the mount has to be re-resolved.
	path, err := a.deps.Worktrees.Path(ctx, sess.WorktreeID)
	if err != nil {
		// 409 and not 404: the SESSION exists, it simply cannot be restarted
		// where it was. A 404 here would read as "no such session" and send the
		// user looking for the wrong problem.
		a.sessionLog().Warn("restart: the worktree no longer resolves",
			"sessionId", id, "worktreeId", sess.WorktreeID, "err", err)
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"The worktree this session was created in no longer resolves.",
			map[string]string{"worktreeId": sess.WorktreeID})
		return
	}

	if !a.validateSessionImplementation(w, sess.ProjectID, sess.WorktreeID, path, sess.ImplementationContext) {
		return
	}
	var reviewedMount backend.Mount
	if sess.ImplementationContext != nil {
		if _, err = a.readReviewedContext(sess.ID, sess.ImplementationContext, sess.ReviewedContext); err != nil {
			a.reviewedContextProblem(w, sess.ID, err)
			return
		}
		dir, _ := a.reviewedContextPath(sess.ID)
		reviewedMount = backend.Mount{HostPath: dir, ContainerPath: reviewedContextContainerDir, ReadOnly: true}
	}
	// A harness with a conversation is REBUILT, not replayed, and restart is
	// the one place this file does that. The stored args name the old
	// conversation with --session-id, and now that the store outlives the
	// sandbox that conversation still exists: the pinned CLI answers a replay
	// with "Session ID <id> is already in use" and exits. So restart mints a
	// new conversation and builds argv from the stored configuration — which
	// also drops a resumed session's --resume, because a restart is "same
	// config, clean context" (docs/FUNCTIONAL-SPEC.md) and replaying the fork
	// would start it inside the conversation it was forked from. Everything
	// else still replays the stored command and args exactly.
	//
	// Before the destroy, like the worktree above: a restart that cannot build
	// its argv must fail while the old sandbox is still alive.
	command, args, conversation := sess.Command, sess.Args, sess.ConversationID
	var conversationStore string
	if desc, known := harness.Lookup(harness.ID(sess.Harness)); known && (sessionHasConversation(desc) || desc.NativeState()) {
		conversation = ids.UUID()
		command, args, err = harness.Build(harness.Request{
			DockerBoundary:  a.deps.Backend.Kind() == backend.KindDocker,
			ProjectBrief:    true,
			ReviewedContext: sess.ImplementationContext != nil,
			Harness:         desc.ID,
			Model:           model,
			Effort:          harness.Effort(sess.Effort),
			Permission:      harness.PermissionMode(sess.PermissionMode),
			OpeningPrompt:   sess.OpeningPrompt,
			Conversation:    conversation,
		})
		if err != nil {
			a.writeHarnessProblem(w, r, desc.ID, desc, err)
			return
		}
		if !desc.NativeState() {
			if conversationStore, err = a.sessionConversationStore(sess.WorktreeID); err != nil {
				a.internal(w, r, "prepare the conversation store for worktree "+sess.WorktreeID, err)
				return
			}
		}
	}
	// The environment is rebuilt like the mounts, and before the destroy like
	// them: the identity is asked for again, so a user who gave their git an
	// identity after the first create gets a sandbox that can commit, and so
	// are the line endings, so a failure to read them leaves the old sandbox
	// alive.
	lineEndings, err := a.sessionGitLineEndings(ctx)
	if err != nil {
		a.internal(w, r, "read how the host's git converts line endings, for worktree "+sess.WorktreeID, err)
		return
	}
	gitFiles, err := a.sessionGitGlobalFiles(ctx, sess.WorktreeID)
	if err != nil {
		a.internal(w, r, "find the host git's global excludes and attributes files for worktree "+sess.WorktreeID, err)
		return
	}
	worktreeAdmin, err := sessionWorktreeAdmin(path)
	if err != nil {
		a.internal(w, r, "prepare the linked-worktree metadata of worktree "+sess.WorktreeID, err)
		return
	}
	sandboxEnv := sessionEnv(a.sessionGitIdentity(ctx, sess.WorktreeID), lineEndings, gitFiles, a.deps.CABundle)
	for key, value := range agentEnv {
		sandboxEnv[key] = value
	}
	if sess.Harness == string(harness.OpenCode) && sandboxEnv["OPENCODE_CONFIG_CONTENT"] == "" {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "Configure an explicit local or compatible model in Settings before starting OpenCode.", nil)
		return
	}
	args = codexSubscriptionArgs(args, sandboxEnv, authMode)
	if a.deps.Backend.Kind() == backend.KindDocker {
		nativeDockerEndpoint(sandboxEnv)
	}
	if sess.Harness == string(harness.OpenCode) {
		if sess.ImplementationContext != nil {
			if err := clearReviewedOpenCodeInstructions(sandboxEnv); err != nil {
				httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
				return
			}
		}
		if err := prepareLocalOpenCodeCodingContext(ctx, sandboxEnv, a.deps.Backend); err != nil {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
			return
		}
	}
	args = nativeArgs(sess.Harness, args, sandboxEnv, false)
	nativeSandboxEnvironment(sandboxEnv)
	// No carried transcript: a restart starts clean, so even a fork's restart
	// resumes nothing and needs no source to read.
	sandboxMounts := sessionMounts(path, worktreeAdmin, conversationStore, backend.Mount{}, a.deps.CABundle, gitFiles)
	if reviewedMount.HostPath != "" {
		sandboxMounts = append(sandboxMounts, reviewedMount)
	}
	if sess.Harness == string(harness.Shell) {
		home, err := a.prepareShellHome(sess.WorktreeID, sess.ID)
		if err != nil {
			a.internal(w, r, "prepare private terminal home for restart", err)
			return
		}
		shellEnvironment(sandboxEnv)
		sandboxMounts = shellMount(sandboxMounts, home)
	}

	// One of contract §5.2's three permitted destroys.
	if err := restartAdmission.revalidate(ctx); err != nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, err.Error(), nil)
		return
	}
	if sess.SandboxID != "" {
		if err := a.deps.Backend.Destroy(ctx, sess.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
			a.sessionLog().Error("restart: destroy the old sandbox",
				"sessionId", id, "sandboxId", sess.SandboxID, "err", err)
			a.writeBackendProblem(w, r, "destroy the old sandbox", err)
			return
		}
	}
	if nativeHarness(sess.Harness) {
		source := sessionResumeSource{worktreeID: sess.WorktreeID, conversation: sess.ConversationID}
		if account != nil && account.ActiveSessionID == sess.ID {
			if err := a.reconcileCodexConnection(account); err != nil {
				a.markSessionExited(id, "ChatGPT sign-in could not be retained after stopping")
				codexAccountProblem(w, err)
				return
			}
		}
		providerStore, err := a.prepareNativeState(sess.WorktreeID, sess.Harness, conversation, source, true)
		if err != nil {
			_, _ = a.deps.Store.Update(id, func(s *registry.Session) error {
				s.Status = registry.StatusExited
				s.ExitReason = "provider state could not be prepared after stopping"
				s.Endpoint = ""
				return nil
			})
			a.internal(w, r, "prepare private provider state for restart", err)
			return
		}
		if err := seedCodexConnection(providerStore, account); err != nil {
			a.markSessionExited(id, "ChatGPT sign-in could not be prepared after stopping")
			codexAccountProblem(w, err)
			return
		}
		sandboxMounts = nativeMount(sandboxMounts, providerStore)
		nativeEnvironment(sess.Harness, sandboxEnv, harness.PermissionMode(sess.PermissionMode))
	}

	// The conversation the restart leaves stays resumable. A restart clears the
	// context, not the artifact: the old transcript is still on disk, and with
	// the record naming only the new conversation nothing would name it again.
	// So it joins the list a resume falls back to — if the CLI wrote it. A
	// conversation nobody spoke in has no transcript, and listing it would grow
	// the record by one dead id per restart.
	//
	// After the destroy, not before with the rest: until the old sandbox is gone
	// its CLI can still write the first line, and a look taken before that would
	// drop a conversation that exists. A look that fails keeps the id, because
	// dropping one that exists is the loss this is here to prevent and listing
	// one that does not costs a resume only a skipped candidate.
	//
	// A plain append: sess is Get's clone and Update clones again on the way
	// in, so nothing this appends into is shared with the store.
	earlier := sess.EarlierConversationIDs
	if nativeHarness(sess.Harness) && sess.ConversationID != "" {
		earlier = append(earlier, sess.ConversationID)
	}
	if conversationStore != "" && sess.ConversationID != "" {
		written, err := sessionTranscriptState(conversationStore, sess.ConversationID)
		if err != nil {
			a.sessionLog().Warn("restart: cannot tell whether the old conversation was written; keeping it",
				"sessionId", id, "err", err)
		}
		if written || err != nil {
			earlier = append(earlier, sess.ConversationID)
		}
	}

	// A FRESH sandbox id, not the old one. Reusing it races a Destroy the
	// daemon has not finished and comes back as ErrAlreadyExists; minting a new
	// one also means a container that survived the destroy stops matching any
	// record, so task 13's reaper sees an orphan and removes it.
	sandboxID := ids.New("s")
	token := ids.Token()
	if sess.Harness == string(harness.OpenCode) {
		sandboxEnv["OPENCODE_SERVER_PASSWORD"] = token
	}
	finishPreparation := a.beginSessionPreparation(registry.Session{ID: id, SandboxID: sandboxID, AttachToken: token})
	defer finishPreparation()

	// THE RECORD NAMES THE NEW SANDBOX BEFORE THE SANDBOX EXISTS, which is
	// create's ordering (see handleSessionCreate step 4) and is here for the
	// same reason: the record is the only thing that names a container, and
	// task 13's reaper matches live containers against SandboxID.
	//
	// Written after Create instead, as this did, two sweeps break a restarting
	// session in two different ways for the whole duration of the create — which
	// is a readiness timeout, not an instant:
	//
	//  1. The new container is live and named by no record, so the orphan sweep
	//     destroys the sandbox the user just asked for.
	//  2. The record still names the sandbox the Destroy above removed, so the
	//     disappeared-sandbox rule writes the session terminal — exited,
	//     "sandbox disappeared" — while it is restarting normally. A fabricated
	//     status, on a session that is fine.
	//
	// Task 13 carries a grace for the create window as defence in depth; a grace
	// tolerates a window, and writing first removes it. What remains between the
	// Destroy above and this write is two id mints and one store update, not a
	// container start.
	//
	// The endpoint is cleared rather than left pointing at the sandbox that was
	// just destroyed, and the token is replaced in the same write as the id, so
	// the record never names one sandbox while holding another's secret. The
	// argv, the conversation and the earlier conversations go in the same write
	// for the same reason: a resume must find the conversation the new sandbox
	// is running, and the one it left.
	if _, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.SandboxID = sandboxID
		sess.SandboxRef = ""
		sess.Endpoint = ""
		sess.AttachToken = token
		sess.Command = command
		sess.Args = args
		sess.AuthOnly = false
		sess.AuthMode = authMode
		sess.CodexConnectionID = codexConnectionID(account)
		sess.AgentConnectionID = agentConnection.ID
		sess.AgentConnectionRevision = agentConnection.Revision
		sess.AgentConnectionKind = agentConnection.Kind
		sess.AgentConnectionProtocol = agentConnection.Protocol
		sess.ConversationID = conversation
		sess.EarlierConversationIDs = earlier
		sess.Status = registry.StatusStarting
		sess.ExitReason = ""
		sess.StartFailure = registry.SessionStartFailure{}
		sess.StartAdmissionEvidence = registry.StartAdmissionEvidence{}
		// AttachCount is deliberately untouched. Task 12's relay defer
		// decrements it when the dropped socket notices; zeroing it here would
		// take the count negative.
		return nil
	}); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "record the restart", err)
		return
	}

	if err := a.leaseCodexConnection(account, id); err != nil {
		a.markSessionExited(id, "the saved ChatGPT connection could not be reserved")
		codexAccountProblem(w, err)
		return
	}
	if err := restartAdmission.bind(ctx, sandboxID); err != nil {
		a.markSessionExited(id, "execution access or capacity changed before restart allocation")
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, err.Error(), nil)
		return
	}
	// Its own context, for the same reason create's is: F1's abort cancels a
	// restart's Docker calls exactly as it cancels a create's.
	createCtx, endCreate := a.sessionCreateContext(ctx)
	defer endCreate()
	sb, err := a.deps.Backend.Create(createCtx, backend.Spec{
		ID:          sandboxID,
		SessionID:   sess.ID,
		Image:       a.sessionImage(),
		Command:     command,
		Args:        args,
		Env:         sandboxEnv,
		AttachToken: token,
		Resources:   restartAdmission.resources,
		WorkDir:     sessionWorkDir,
		Mounts:      sandboxMounts,
	})
	if err != nil {
		a.sessionLog().Error("restart: create the sandbox", "sessionId", id, "err", err)
		// Pinned by TestARestartThatCannotCreateDestroysTheContainerItJustMade:
		// deleting this line left the package green until that test existed.
		a.destroyAfterFailedCreate(sandboxID, id)
		a.markSessionExited(id, "the sandbox could not be recreated")
		a.writeBackendProblem(w, r, "create the sandbox", err)
		return
	}

	// A successful Create acknowledges both backend creation and startup. Record
	// that boundary independently of the HTTP caller before publishing its result.
	var settledErr error
	if sb.ID != sandboxID {
		settledErr = errors.New("backend acknowledgment does not match the exact restarted sandbox")
	} else {
		settledCtx, endSettled := context.WithTimeout(context.Background(), 5*time.Second)
		settledErr = restartAdmission.complete(settledCtx, sandboxID)
		endSettled()
	}
	if settledErr != nil {
		a.destroyAfterFailedCreate(sandboxID, id)
		a.markSessionExited(id, "execution launch acknowledgment could not be recorded")
		a.internal(w, r, "record the restarted sandbox launch", settledErr)
		return
	}

	// Second write, and it carries only what the create told us: the id and the
	// token are already on the record. Mirrors create's own second write,
	// precondition included — this path is the more exposed of the two, because
	// it runs on a record that was `exited` moments before.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		if sess.SandboxID != sandboxID || sess.Status != registry.StatusStarting {
			return sessionErrMoved
		}
		sess.SandboxRef = sb.Ref
		sess.Endpoint = sb.Endpoint
		return nil
	})
	if err != nil {
		if a.sessionMovedOnMidCreate(w, r, err, sb.ID, id) {
			return
		}
		a.internal(w, r, "record the restarted sandbox", err)
		return
	}

	go a.watchSessionReady(id, sb.ID, sb.Endpoint)
	httpx.WriteJSON(w, http.StatusOK, a.sessionJSON(updated, sessionBasePath))
}

// handleSessionDelete is §3.15 and it is one of the three places in this phase
// that is allowed to destroy a sandbox. Without purge the record stays: the
// transcript is the artifact and docs/FUNCTIONAL-SPEC.md requires finished
// sessions stay listed and readable.
func (a *API) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	id := sess.ID
	if !sessionRestarts.acquire(id) {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "The session is starting, restarting or being copied; wait before stopping or forgetting it.", nil)
		return
	}
	defer sessionRestarts.release(id)
	ctx := r.Context()
	if sess.Harness == "codex" {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
	}
	purge := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("purge")), "true")
	if purge {
		if err := a.requireTaskHandoffBeforePurge(ctx, sess); err != nil {
			httpx.WriteProblem(w, http.StatusConflict, "task_handoff_required", err.Error(), nil)
			return
		}
	}

	// A session a previous verb already took terminal has no sandbox left to
	// destroy, and re-issuing the Destroy is what let a daemon outage trap the
	// record forever: this verb deliberately KEEPS SandboxID after a kill — it
	// is how the reaper tells a known container from an orphan — so a second
	// DELETE ...?purge=true would ask the backend again, get 503, and never
	// remove a record whose sandbox is already gone.
	//
	// exited WITH both the endpoint and the token cleared is the state only a
	// COMPLETED kill, a completed watch failure or the reaper leaves. A destroy
	// that failed leaves the record untouched — that is the rule below — so it
	// does not match this and still gets its retry, and no container is ever
	// orphaned by a record being purged out from under it.
	alreadyTerminal := sess.Status == registry.StatusExited && sess.Endpoint == "" && sess.AttachToken == ""

	if sess.SandboxID != "" && !alreadyTerminal {
		// Already gone is success. An explicit kill and the reaper race by
		// design, and a delete that errors on "already gone" makes the cockpit
		// show a failure for work that completed.
		if err := a.deps.Backend.Destroy(ctx, sess.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
			// The record is NOT marked exited here. The container is probably
			// still running, and reporting it exited would be a fabricated
			// status.
			a.sessionLog().Error("destroy the sandbox", "sessionId", id, "sandboxId", sess.SandboxID, "err", err)
			a.writeBackendProblem(w, r, "destroy the sandbox", err)
			return
		}
	}

	// Marked exited even on the purge path, so a purge that fails half-way
	// leaves an honest record rather than one that still claims to be running.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		// An existing reason is kept: the first reason is the true one, and
		// killing an already-exited session must not rewrite its history.
		if sess.ExitReason == "" {
			sess.ExitReason = "killed"
		}
		sess.Endpoint = ""
		sess.AttachToken = ""
		// SandboxID is kept. It is how task 13's reaper tells a container that
		// belongs to a known session from an orphan, and it is the only
		// diagnostic left once the endpoint is cleared.
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "record the kill", err)
		return
	}

	if a.deps.ResourceSnapshot != nil {
		if err := a.execution.releaseSession(ctx, id); err != nil {
			if purge {
				httpx.WriteProblem(w, 409, httpx.CodeConflict, "The sandbox exit and capacity release are not yet confirmed. Its session record remains available; retry Forget after recovery.", nil)
				return
			}
			a.sessionLog().Warn("retain session capacity after stop", "sessionId", id, "err", err)
		}
	}
	if sess.Harness == "codex" && sess.CodexConnectionID != "" {
		account, err := a.readCodexConnection()
		if err != nil {
			codexAccountProblem(w, err)
			return
		}
		if account != nil && account.ID == sess.CodexConnectionID && account.ActiveSessionID == id {
			if err := a.reconcileCodexConnection(account); err != nil {
				codexAccountProblem(w, err)
				return
			}
			account.ActiveSessionID = ""
			if err := a.writeCodexConnection(account); err != nil {
				codexAccountProblem(w, err)
				return
			}
		}
	}
	if !purge {
		httpx.WriteJSON(w, http.StatusOK, a.sessionJSON(updated, sessionBasePath))
		return
	}

	// OD-10, "Forget deletes it": the session's own transcripts go before its
	// record does. That order is the reporting. A deletion that fails leaves
	// the record — the one thing that names the file — so the 500 is true, the
	// cockpit puts the row back, and asking again finds the file again. The
	// other order would answer 204 over a transcript nothing names any more,
	// kept forever: the state OD-10 exists to end. This session's sandbox is
	// destroyed by now, so its CLI is not writing the file this removes. The
	// worktree's other sessions still have the store mounted, which is why the
	// deletes cannot be steered through a link (sessionForgetTranscripts).
	//
	// Below the purge check and below the destroy, both on purpose and both
	// pinned: above the first, every Kill would erase the conversation Resume
	// needs; above the second, a 503 would come back over a live session whose
	// conversation was already gone.
	if err := a.sessionForgetTranscripts(updated); err != nil {
		a.internal(w, r, "delete the session's transcript", err)
		return
	}
	if updated.ReviewedContext != nil {
		if err := a.removeReviewedContext(updated.ID); err != nil {
			a.internal(w, r, "delete the session's reviewed context", err)
			return
		}
	}

	if err := a.deps.Store.Delete(id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		// The sandbox is already gone by this point, so the detail an operator
		// needs is "destroyed, not purged" — and it goes in the log, where
		// a.internal puts it, rather than into a 500 body that would then be
		// the only 500 on this surface that reads differently from the
		// catalog's.
		a.internal(w, r, "purge the session record", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
