package api

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// Fork was shipped starting a fresh session in the other worktree: the same
// configuration, none of the conversation — against docs/FUNCTIONAL-SPEC.md's
// "branch the session from this point". These tests are what Fork owes now:
// the CLI starts on the source's conversation, forked, in the other worktree,
// and the source's file reaches the fork's sandbox only as a staged, read-only
// copy that no agent can redirect.

// forkTarget is the second worktree on the env's branch, the one a Fork goes
// into.
const forkTarget = "w-5b88c2d0"

// forkTranscript is a source transcript's bytes. Distinctive, so a staged copy
// that holds them is a copy of THIS file and not an empty or invented one.
var forkTranscript = []byte("{\"type\":\"user\",\"text\":\"remember the word pelican\"}\n")

// forkNewEnv is sessionsNewEnv with forkTarget added beside the env's worktree:
// resolvable, and enumerated on the same branch, as the cockpit's Fork into…
// menu finds it.
func forkNewEnv(t *testing.T) *sessionsEnv {
	t.Helper()
	return forkAddTarget(t, sessionsNewEnv(t))
}

// forkNewEnvWithLog is forkNewEnv with a log a test can read.
func forkNewEnvWithLog(t *testing.T) (*sessionsEnv, *sandboxEnvLog) {
	t.Helper()
	env, sink := forgetNewEnvWithLog(t)
	return forkAddTarget(t, env), sink
}

// forkAddTarget adds forkTarget to env.
func forkAddTarget(t *testing.T, env *sessionsEnv) *sessionsEnv {
	t.Helper()
	dir := filepath.Join(filepath.Dir(env.dir), "evidence-upload-2")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("make the second worktree: %v", err)
	}
	env.wt[forkTarget] = dir
	env.loc.worktrees["p-8c1d0e42"] = append(env.loc.worktrees["p-8c1d0e42"], project.Worktree{
		ID: forkTarget, ProjectID: "p-8c1d0e42", BranchID: "b-8c1d0e42-evidence-upload",
		Name: "evidence-upload-2", Path: dir,
	})
	return env
}

// forkBody is the Fork verb as the desktop now sends it: resume names the
// source, worktreeId names the other worktree.
func forkBody(sourceID string) string {
	return `{"worktreeId":"` + forkTarget + `","harness":"claude-code","model":"opus","resume":"` + sourceID + `"}`
}

// forkStore is a worktree's conversation store, and forkPath is a transcript in
// it, both spelled out like resumeSpeak so a layout change fails here.
func forkStore(env *sessionsEnv, worktreeID string) string {
	return filepath.Join(env.conversations, "local", worktreeID)
}

func forkPath(env *sessionsEnv, worktreeID, conversation string) string {
	return filepath.Join(forkStore(env, worktreeID), "-home-dev-work", conversation+".jsonl")
}

// forkWrite writes a file, making its directory.
func forkWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// forkStaged is where the fork session id's copy of conversation is staged.
func forkStaged(env *sessionsEnv, sessionID, conversation string) string {
	return filepath.Join(env.conversations, "local", ".forks", sessionID, conversation+".jsonl")
}

// forkCreate creates a fork of s-0000000a, which has spoken in the env's
// worktree, and returns the fork's record.
func forkCreate(t *testing.T, env *sessionsEnv) registry.Session {
	t.Helper()
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	forkWrite(t, forkPath(env, "w-4a77b1c9", sessionsConversation), forkTranscript)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", forkBody("s-0000000a")), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	fork, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return fork
}

// THE DEFECT, pinned: a fork's CLI starts on the source's conversation, forked
// under a new id, in the other worktree — the argv a Resume gets, word for word.
func TestAForkIntoAnotherWorktreeStartsTheCLIOnTheSourcesConversation(t *testing.T) {
	env := forkNewEnv(t)
	fork := forkCreate(t, env)

	spec := env.be.spec(t, 0)
	want := []string{"--model", "opus", "--resume", sessionsConversation, "--fork-session", "--session-id", fork.ConversationID}
	if resumeJoin(spec.Args) != resumeJoin(want) {
		t.Fatalf("Args = %#v\nwant   %#v", spec.Args, want)
	}
	if fork.WorktreeID != forkTarget {
		t.Errorf("the fork's worktree = %q, want %q", fork.WorktreeID, forkTarget)
	}
	if fork.ConversationID == sessionsConversation || !resumeUUID.MatchString(fork.ConversationID) {
		t.Errorf("the fork's conversationId = %q, want a freshly minted UUID", fork.ConversationID)
	}
	if fork.CarriedConversationID != sessionsConversation {
		t.Errorf("carriedConversationId = %q, want the source's %q, so Forget can find what carrying left", fork.CarriedConversationID, sessionsConversation)
	}
	// The fork writes into ITS worktree's store, not the source's.
	if spec.Mounts[1] != (backend.Mount{HostPath: forkStore(env, forkTarget), ContainerPath: "/home/dev/.claude/projects"}) {
		t.Errorf("Mounts[1] = %#v, want the fork's own worktree store", spec.Mounts[1])
	}
}

// The source's transcript reaches the fork's sandbox as ONE file, read-only, at
// the path the pinned CLI reads, and from a staged copy of its exact bytes —
// never from the source's store, which an agent in the source's sandbox can
// rewrite while the fork is created.
func TestAForkIsShownAStagedReadOnlyCopyOfTheSourcesTranscript(t *testing.T) {
	env := forkNewEnv(t)
	fork := forkCreate(t, env)

	spec := env.be.spec(t, 0)
	staged := forkStaged(env, fork.ID, sessionsConversation)
	want := backend.Mount{
		HostPath:      staged,
		ContainerPath: "/home/dev/.claude/projects/-home-dev-work/" + sessionsConversation + ".jsonl",
		ReadOnly:      true,
	}
	if len(spec.Mounts) != 3 || spec.Mounts[2] != want {
		t.Fatalf("Mounts = %#v, want the worktree, its store, and then %#v", spec.Mounts, want)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read the staged copy: %v", err)
	}
	if !bytes.Equal(got, forkTranscript) {
		t.Errorf("the staged copy holds %q, want the source's bytes %q", got, forkTranscript)
	}
	for _, store := range []string{forkStore(env, "w-4a77b1c9"), forkStore(env, forkTarget)} {
		if rel, err := filepath.Rel(store, staged); err == nil && !strings.HasPrefix(rel, "..") {
			t.Errorf("the staged copy %s is inside the store %s, which a sandbox mounts read-write", staged, store)
		}
	}
	// Made by the host before the sandbox: Docker would make it root-owned 0755,
	// and the CLI could not write the fork into it.
	if info, err := os.Stat(filepath.Join(forkStore(env, forkTarget), "-home-dev-work")); err != nil || !info.IsDir() {
		t.Errorf("the CLI's transcript directory in the fork's store was not made before the sandbox: %v", err)
	}
	// The source is read, never written: its file and its record.
	if b, _ := os.ReadFile(forkPath(env, "w-4a77b1c9", sessionsConversation)); !bytes.Equal(b, forkTranscript) {
		t.Errorf("forking changed the source's transcript to %q", b)
	}
	if src, _ := env.st.Get("s-0000000a"); src.ConversationID != sessionsConversation || src.WorktreeID != "w-4a77b1c9" {
		t.Errorf("forking touched the source record: %+v", src)
	}
}

// The carried id is a server-side fact like the fork's own.
func TestAForksResponseNamesNoConversation(t *testing.T) {
	env := forkNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	forkWrite(t, forkPath(env, "w-4a77b1c9", sessionsConversation), forkTranscript)

	rec := env.do(t, http.MethodPost, "/v1/sessions", forkBody("s-0000000a"))
	sessionsDecodeOne(t, rec, http.StatusCreated)
	sessionsAwaitWatch(t, done)
	if strings.Contains(rec.Body.String(), sessionsConversation) {
		t.Errorf("the carried conversation reached the wire: %s", rec.Body.String())
	}
}

// A Resume in its own worktree needs nothing carried: its store is the one the
// source wrote to. No staged copy, no extra mount, nothing for Forget.
func TestAResumeInItsOwnWorktreeCarriesNothing(t *testing.T) {
	env := forkNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	resumeSpeak(t, env, sessionsConversation)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody("s-0000000a")), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	if spec := env.be.spec(t, 0); len(spec.Mounts) != 2 {
		t.Errorf("Mounts = %#v, want the worktree and its store alone", spec.Mounts)
	}
	if rec, _ := env.st.Get(got.ID); rec.CarriedConversationID != "" {
		t.Errorf("carriedConversationId = %q on a resume in its own worktree", rec.CarriedConversationID)
	}
	if forkExists(t, filepath.Join(env.conversations, "local", ".forks")) {
		t.Errorf("a resume in its own worktree staged a copy")
	}
}

// forkExists is forgetExists under this file's name.
func forkExists(t *testing.T, path string) bool {
	t.Helper()
	return forgetExists(t, path)
}

// Fork is refused exactly as Resume is: the one sentence, from the one place.
// The sentence is compared between the two verbs rather than to a literal, so
// a fork with a refusal of its own fails here.
func TestAForkOfASessionNobodySpokeInIsRefusedWithResumesSentence(t *testing.T) {
	message := func(body func(string) string) (int, string, *sessionsEnv) {
		env := forkNewEnv(t)
		sessionsSeed(t, env.st, resumeSource("s-0000000a"))
		rec := env.do(t, http.MethodPost, "/v1/sessions", body("s-0000000a"))
		return rec.Code, resumeProblemMessage(t, rec.Body.Bytes()), env
	}
	resumeCode, resumeSaid, _ := message(resumeBody)
	forkCode, forkSaid, env := message(forkBody)

	if forkCode != http.StatusConflict || resumeCode != forkCode {
		t.Errorf("fork status = %d and resume status = %d, want both 409", forkCode, resumeCode)
	}
	if !strings.Contains(forkSaid, "Nothing has been said in that session yet") || forkSaid != resumeSaid {
		t.Errorf("the fork said %q and the resume said %q; want Resume's sentence, the same", forkSaid, resumeSaid)
	}
	if n := len(env.be.specs); n != 0 {
		t.Errorf("backend.Create was called %d times for a fork that cannot work", n)
	}
	if list, _ := env.st.List(); len(list) != 1 {
		t.Errorf("a refused fork wrote a record: %d sessions", len(list))
	}
	if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused fork created %s (stat err = %v)", env.conversations, err)
	}
}

// forkRefusal is a fork that must be refused at staging: 500, no record, no
// sandbox, nothing staged.
func forkRefusal(t *testing.T, env *sessionsEnv) {
	t.Helper()
	rec := env.do(t, http.MethodPost, "/v1/sessions", forkBody("s-0000000a"))
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	if n := len(env.be.specs); n != 0 {
		t.Errorf("backend.Create was called %d times for a fork whose transcript could not be carried", n)
	}
	if list, _ := env.st.List(); len(list) != 1 {
		t.Errorf("a fork that carried nothing wrote a record: %d sessions", len(list))
	}
	staging := filepath.Join(env.conversations, "local", ".forks")
	entries, err := os.ReadDir(staging)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read %s: %v", staging, err)
	}
	if len(entries) != 0 {
		t.Errorf("a fork that carried nothing left %d staging entries", len(entries))
	}
}

// Only a regular file is carried. The store is the source agent's
// ~/.claude/projects, so what sits at the transcript's path is the agent's
// choice, and a link there is a host file of its choosing.
func TestAForkCarriesNothingButARegularFile(t *testing.T) {
	t.Run("the transcript is a directory", func(t *testing.T) {
		env, sink := forkNewEnvWithLog(t)
		sessionsSeed(t, env.st, resumeSource("s-0000000a"))
		if err := os.MkdirAll(forkPath(env, "w-4a77b1c9", sessionsConversation), 0o700); err != nil {
			t.Fatal(err)
		}
		forkRefusal(t, env)
		// Refused for what it IS, before it is opened. Opening a directory and
		// failing to read it refuses too, but the same path with a named pipe
		// there would block in the open (sessions_fork_fifo_test.go).
		if !strings.Contains(sink.String(), "not a regular file") {
			t.Errorf("the log does not say the transcript was not a regular file:\n%s", sink.String())
		}
	})
	t.Run("the transcript is a link to a host file", func(t *testing.T) {
		env := forkNewEnv(t)
		sessionsSeed(t, env.st, resumeSource("s-0000000a"))
		victim := filepath.Join(filepath.Dir(env.conversations), "victim.txt")
		forkWrite(t, victim, []byte("a host file the agent does not own\n"))
		link := forkPath(env, "w-4a77b1c9", sessionsConversation)
		forkWrite(t, filepath.Join(filepath.Dir(link), "placeholder"), nil)
		if err := os.Symlink(victim, link); err != nil {
			t.Skipf("this platform will not make a file symlink here: %v", err)
		}
		forkRefusal(t, env)
	})
	t.Run("the transcript directory is a link", func(t *testing.T) {
		env := forkNewEnv(t)
		sessionsSeed(t, env.st, resumeSource("s-0000000a"))
		elsewhere := filepath.Join(filepath.Dir(env.conversations), "elsewhere")
		forkWrite(t, filepath.Join(elsewhere, sessionsConversation+".jsonl"), forkTranscript)
		if err := os.MkdirAll(forkStore(env, "w-4a77b1c9"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, filepath.Join(forkStore(env, "w-4a77b1c9"), "-home-dev-work")); err != nil {
			t.Skipf("this platform will not make a directory symlink here: %v", err)
		}
		forkRefusal(t, env)
	})
}

// A file swapped between the look and the open is not the file looked at, and
// is not carried. The swap is simulated by the look answering for another file:
// no filesystem swaps on demand between two calls.
func TestAForkCarriesNothingWhenTheFileChangesBetweenTheLookAndTheOpen(t *testing.T) {
	env := forkNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	src := forkPath(env, "w-4a77b1c9", sessionsConversation)
	forkWrite(t, src, forkTranscript)
	decoy := filepath.Join(filepath.Dir(env.conversations), "decoy.jsonl")
	forkWrite(t, decoy, forkTranscript)

	prev := sessionLstat
	sessionLstat = func(name string) (fs.FileInfo, error) {
		if name == src {
			return os.Lstat(decoy)
		}
		return prev(name)
	}
	t.Cleanup(func() { sessionLstat = prev })

	forkRefusal(t, env)
}

// The copy is staged before the record, so a record that cannot be written
// leaves a copy nothing names. The create removes it on the way out.
func TestAForkWhoseRecordCannotBeWrittenLeavesNoStagedCopy(t *testing.T) {
	env := forkNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	forkWrite(t, forkPath(env, "w-4a77b1c9", sessionsConversation), forkTranscript)
	env.st.state.createErr = errors.New("the disk is full")

	rec := env.do(t, http.MethodPost, "/v1/sessions", forkBody("s-0000000a"))
	if rec.Code < 400 {
		t.Fatalf("status = %d, want a failure", rec.Code)
	}
	staging := filepath.Join(env.conversations, "local", ".forks")
	entries, err := os.ReadDir(staging)
	if err != nil {
		t.Fatalf("read %s: %v — the staging step never ran, so this proves nothing", staging, err)
	}
	if len(entries) != 0 {
		t.Errorf("a fork that was never recorded left %d staged copies", len(entries))
	}
}

// ids.New can collide, however rarely, and a staging directory already there
// is another session's. Staging refuses it and touches nothing in it — neither
// writes into it nor removes it on the way out.
func TestStagingNeverWritesIntoOrRemovesADirectoryItDidNotMake(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "store")
	forkWrite(t, filepath.Join(source, "-home-dev-work", sessionsConversation+".jsonl"), forkTranscript)
	staging := filepath.Join(root, ".forks", "s-0000000c")
	theirs := filepath.Join(staging, "theirs.jsonl")
	forkWrite(t, theirs, []byte("another session's staged copy\n"))

	if _, err := sessionStageCarry(source, sessionsConversation, staging); err == nil {
		t.Fatal("staging wrote into a directory that was already there")
	}
	if !forkExists(t, theirs) {
		t.Fatal("staging removed another session's staged copy")
	}
	if forkExists(t, filepath.Join(staging, sessionsConversation+".jsonl")) {
		t.Error("staging put a copy into another session's directory")
	}
}

// OD-10 for a fork: Forget deletes the fork's own transcript, the staged copy,
// and the empty mount point Docker left for it in the fork's store — and not
// the source's transcript, which is the source's to forget.
func TestForgettingAForkDeletesWhatCarryingLeftAndNothingElse(t *testing.T) {
	env := forkNewEnv(t)
	fork := forkCreate(t, env)
	// What the sandbox left: the CLI's fork, Docker's empty mount point for the
	// staged copy, and another session's transcript beside them.
	forkWrite(t, forkPath(env, forkTarget, fork.ConversationID), forkTranscript)
	forkWrite(t, forkPath(env, forkTarget, sessionsConversation), nil)
	forkWrite(t, forkPath(env, forkTarget, forgetOther), forkTranscript)

	if code := forgetPurge(t, env, fork.ID); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204", code)
	}
	if forkExists(t, forkStaged(env, fork.ID, sessionsConversation)) || forkExists(t, filepath.Join(env.conversations, "local", ".forks", fork.ID)) {
		t.Errorf("Forget left the fork's staged copy of its source's transcript")
	}
	if forkExists(t, forkPath(env, forkTarget, sessionsConversation)) {
		t.Errorf("Forget left the mount point named after the source's conversation in the fork's store")
	}
	if forkExists(t, forkPath(env, forkTarget, fork.ConversationID)) {
		t.Errorf("Forget left the fork's own transcript")
	}
	if !forkExists(t, forkPath(env, forkTarget, forgetOther)) {
		t.Errorf("Forget deleted another session's transcript in the fork's store")
	}
	if b, err := os.ReadFile(forkPath(env, "w-4a77b1c9", sessionsConversation)); err != nil || !bytes.Equal(b, forkTranscript) {
		t.Errorf("forgetting the fork reached its source's transcript: %q, %v", b, err)
	}
	if _, err := env.st.Get("s-0000000a"); err != nil {
		t.Errorf("forgetting the fork removed its source: %v", err)
	}
}

// Forget of a session that is not a fork removes no staging directory, even
// one its id would name: only a fork has one.
func TestForgetOfASessionThatIsNotAForkLeavesStagingAlone(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	stray := filepath.Join(env.conversations, "local", ".forks", "s-0000000b", "keep.jsonl")
	forkWrite(t, stray, forkTranscript)

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204", code)
	}
	if !forkExists(t, stray) {
		t.Errorf("Forget of a session with no carried conversation removed a staging directory")
	}
}

// The staging directory is the session id joined onto a path, and Forget
// removes it whole. A record whose id is not one segment — a hand-edited
// registry — must remove nothing: ".." joined on names the owner directory,
// every worktree's store in it.
func TestForgetRemovesNoStagingUnderASessionIDThatCannotNameOne(t *testing.T) {
	env, sink := forkNewEnvWithLog(t)
	store := forkPath(env, "w-4a77b1c9", forgetOther)
	forkWrite(t, store, forkTranscript)

	err := env.api.sessionForgetCarried(registry.Session{ID: "..", CarriedConversationID: sessionsConversation})
	if err != nil {
		t.Errorf("err = %v; an id that names no staging directory has none to fail on", err)
	}
	if !forkExists(t, store) {
		t.Fatalf("Forget of a record with the id \"..\" deleted %s", store)
	}
	if !strings.Contains(sink.String(), "names no staging directory") {
		t.Errorf("the refusal was not logged:\n%s", sink.String())
	}
}

// A restart is "same config, clean context", for a fork as for anything: no
// --resume, no carried file. The carried id stays on the record, because the
// mount point and the staged copy are still there for Forget to delete.
func TestARestartOfAForkCarriesNothing(t *testing.T) {
	env := forkNewEnv(t)
	fork := forkCreate(t, env)
	done := sessionsWatchDone(t)

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/"+fork.ID+"/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)

	spec := env.be.spec(t, 1)
	if valueAfter(spec.Args, "--resume") != "" {
		t.Errorf("the restarted fork resumed: %#v", spec.Args)
	}
	if len(spec.Mounts) != 2 {
		t.Errorf("Mounts = %#v, want the worktree and its store alone", spec.Mounts)
	}
	after, _ := env.st.Get(fork.ID)
	if after.CarriedConversationID != sessionsConversation {
		t.Errorf("carriedConversationId = %q after a restart, want %q kept for Forget", after.CarriedConversationID, sessionsConversation)
	}
}
