package api

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// Fork is Resume into another worktree. docs/FUNCTIONAL-SPEC.md: Fork "branch[es]
// the session from this point into a new worktree". Both verbs send `resume`,
// the cockpit id of the source, and both start the CLI the same way:
//
//	--resume <source conversation> --fork-session --session-id <new conversation>
//
// What differs is where the source's transcript is. The conversation store is
// per worktree, so a Resume's sandbox already mounts the store the source wrote
// to, and a Fork's does not: its store is the OTHER worktree's. So a fork's
// sandbox is shown one more file, the source's transcript, read-only, at the
// path the pinned CLI reads it from. The CLI then forks it into the fork's own
// store exactly as it does for a Resume — a whole copy, re-keyed to the new id
// (od10-forget-report.md §3, observed in `-p` mode) — and the source is never
// written.
//
// The file is shown from a STAGED copy, never from the source's store, and that
// is the security property of this file. A bind mount's host path is resolved
// by the Docker daemon on the host, following links, and the source's store is
// mounted read-write into the source's sandbox — which may still be running,
// because the cockpit offers Fork on a live session. An agent there could swap
// its own transcript for a link to any host file between a check here and the
// daemon's resolution, and the daemon would mount that file into the fork's
// sandbox. So the control plane copies the transcript, by the id it minted,
// into a directory no sandbox mounts, refusing anything but a regular file and
// checking that the file it opened is the file it looked at. The copy is the
// mount's source; no component of its path is one an agent can replace.
//
// OD-8 allows this and names its terms. The copy is read-only towards the
// harness's file and byte for byte: nothing is parsed, derived or rewritten. It
// depends on the file's location alone, which the canary pins
// (TestTheTranscriptLayoutIsPinnedToTheImagesCLI). A fork whose copy fails is
// refused, never started fresh: a fork that silently did not carry the
// conversation is the defect this file fixes.

// sessionResumeSource is a `resume` resolved against the registry: the cockpit
// session it named, the worktree that session ran in — whose store holds its
// transcript — and the conversation `--resume` names. The zero value is "no
// resume".
type sessionResumeSource struct {
	id           string
	worktreeID   string
	conversation string
}

// sessionCarry is step 3a of the create, for a fork: it stages the source's
// transcript under the new session's id, and returns the mount that shows it to
// the CLI and the staging directory, which the caller removes if the record is
// never written.
//
// It also makes the CLI's transcript directory in the fork's own store first,
// because that is where the mount point goes. Observed on Docker Desktop with
// panacea-app/sandbox:dev: a mount point's missing parent is made root-owned
// 0755 inside the sandbox, and the CLI, running as the agent user, then cannot
// write the fork beside the file it forks — the fork fails and writes nothing
// (fork-carries-report.md §3). Made by the host, it is as writable as the store.
func (a *API) sessionCarry(source sessionResumeSource, sessionID, conversationStore string) (backend.Mount, string, error) {
	if conversationStore == "" {
		return backend.Mount{}, "", errors.New("a fork needs its own conversation store to fork into")
	}
	sourceStore, err := a.sessionConversationStorePath(source.worktreeID)
	if err != nil {
		return backend.Mount{}, "", err
	}
	staging, err := a.sessionForkStagingPath(sessionID)
	if err != nil {
		return backend.Mount{}, "", err
	}
	if err := os.MkdirAll(filepath.Dir(sessionTranscriptPath(conversationStore, source.conversation)), 0o700); err != nil {
		return backend.Mount{}, "", fmt.Errorf("make the CLI's transcript directory in the fork's store: %w", err)
	}
	staged, err := sessionStageCarry(sourceStore, source.conversation, staging)
	if err != nil {
		return backend.Mount{}, "", err
	}
	return sessionCarriedMount(staged, source.conversation), staging, nil
}

// sessionForkStagingName is the directory, beside the worktree stores under
// <ConversationsDir>/<owner>, that holds forks' staged transcripts. A worktree
// id cannot begin with "." (sessionStoreSegment), so no store is ever named
// this, and no sandbox mounts anything under it read-write.
const sessionForkStagingName = ".forks"

// sessionForkStagingPath is where the fork sessionID's carried transcript is
// staged: <ConversationsDir>/<owner>/.forks/<session id>. Per session, so a
// Forget removes exactly its own, and a second fork of the same source never
// shares a file with the first.
func (a *API) sessionForkStagingPath(sessionID string) (string, error) {
	if !sessionStoreSegment.MatchString(sessionID) {
		return "", fmt.Errorf("session id %q cannot name a directory", sessionID)
	}
	return filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, sessionForkStagingName, sessionID), nil
}

// sessionTranscriptContainerPath is sessionTranscriptPath inside a sandbox:
// where the pinned CLI looks for conversation's transcript. Slash-separated
// whatever the host is, because it is a path in a Linux container.
func sessionTranscriptContainerPath(conversation string) string {
	return path.Join(sessionConversationDir, sessionTranscriptDirName, conversation+".jsonl")
}

// sessionCarriedMount is the one extra mount a fork's sandbox gets: the staged
// copy of the source's transcript, read-only, where the CLI will look for it.
func sessionCarriedMount(staged, conversation string) backend.Mount {
	return backend.Mount{HostPath: staged, ContainerPath: sessionTranscriptContainerPath(conversation), ReadOnly: true}
}

// sessionForgetCarried is Forget's part outside the stores: a fork's staged
// copy of its source's transcript goes with the fork. Only a fork has one — a
// record naming a CarriedConversationID — and it is found by the session's own
// id, under a directory only the control plane writes, so removing it whole
// reaches nothing else.
func (a *API) sessionForgetCarried(sess registry.Session) error {
	if sess.CarriedConversationID == "" {
		return nil
	}
	staging, err := a.sessionForkStagingPath(sess.ID)
	if err != nil {
		a.sessionLog().Error("forget: the session id names no staging directory; removed nothing by it",
			"sessionId", sess.ID, "err", err)
		return nil
	}
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("delete the fork's staged transcript %s: %w", staging, err)
	}
	return nil
}

// sessionStageCarry copies the transcript of conversation out of the source
// worktree's store into staging, and returns the copy's path. Anything but a
// regular file at the transcript's path is refused — a link, a directory, a
// device — and so is a file that is not the one looked at by the time it is
// open, which is how a swap between the look and the open shows. The regular-
// file check comes before the open for a reason of its own: opening a named
// pipe for reading blocks until something writes, so an agent that made its
// transcript a FIFO would hang this request for good. The staging directory
// must not exist yet, and the copy is written with O_EXCL, so nothing already
// there is followed, overwritten or removed. On any later failure the
// directory this made is removed, and nothing staged is left behind.
func sessionStageCarry(sourceStore, conversation, staging string) (string, error) {
	src := sessionTranscriptPath(sourceStore, conversation)
	// The directory first, as Forget checks it: it is inside the mount, and a
	// link there would have the look below follow it to wherever the agent chose.
	dir, err := sessionLstat(filepath.Dir(src))
	if err != nil {
		return "", fmt.Errorf("look at the transcript directory to carry from: %w", err)
	}
	if !dir.IsDir() {
		return "", fmt.Errorf("the transcript directory %s is a %v, not a directory; carried nothing", filepath.Dir(src), dir.Mode().Type())
	}
	looked, err := sessionLstat(src)
	if err != nil {
		return "", fmt.Errorf("look at the transcript to carry: %w", err)
	}
	if !looked.Mode().IsRegular() {
		return "", fmt.Errorf("the transcript %s is a %v, not a regular file; carried nothing", src, looked.Mode().Type())
	}
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open the transcript to carry: %w", err)
	}
	defer in.Close()
	opened, err := in.Stat()
	if err != nil {
		return "", fmt.Errorf("look at the opened transcript: %w", err)
	}
	if !os.SameFile(looked, opened) {
		return "", fmt.Errorf("the transcript %s was replaced while it was being opened; carried nothing", src)
	}

	// The session's own directory is made here or not at all: one already there
	// is another session's — ids.New collided — and nothing below may write into
	// it, or remove it on the way out.
	if err := os.MkdirAll(filepath.Dir(staging), 0o700); err != nil {
		return "", fmt.Errorf("make the forks' staging directory: %w", err)
	}
	if err := os.Mkdir(staging, 0o700); err != nil {
		return "", fmt.Errorf("make the fork's staging directory %s: %w", staging, err)
	}
	dst := filepath.Join(staging, conversation+".jsonl")
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", errors.Join(fmt.Errorf("create the staged transcript: %w", err), os.RemoveAll(staging))
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", errors.Join(fmt.Errorf("copy the transcript to carry: %w", err), os.RemoveAll(staging))
	}
	return dst, nil
}
