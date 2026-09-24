package api

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// OD-10, in the owner's words: "Forget deletes it". Forget — DELETE
// ?purge=true — used to remove the record and leave the session's transcript in
// the worktree's conversation store with nothing left naming it, kept forever.
// These tests are what Forget now owes: the session's own transcripts go, by
// the ids the control plane minted and by nothing else, and a failure to delete
// one is said out loud rather than swallowed.

// forgetEarlier is a conversation a restart left behind: the session's own, and
// as much Forget's to delete as its current one.
const forgetEarlier = "c0ffee00-0000-4000-8000-0000ea411e00"

// forgetOther is another session's conversation in the same worktree store. It
// must survive every Forget of a session that is not its own.
const forgetOther = "0badf00d-0000-4000-8000-000000000001"

// forgetTranscript is where the pinned CLI writes conversation's transcript in
// the env's worktree store — spelled out, like resumeSpeak, so a change to the
// layout sessionTranscriptPath computes fails here instead of moving both sides.
func forgetTranscript(env *sessionsEnv, conversation string) string {
	return filepath.Join(env.conversations, "local", "w-4a77b1c9", "-home-dev-work", conversation+".jsonl")
}

// forgetExists is whether path is there, failing the test on any answer other
// than yes or certainly not.
func forgetExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("stat %s: %v", path, err)
	return false
}

// forgetPurge is the Forget verb exactly as the desktop sends it.
func forgetPurge(t *testing.T, env *sessionsEnv, id string) int {
	t.Helper()
	return env.do(t, http.MethodDelete, "/v1/sessions/"+id+"?purge=true", "").Code
}

// forgetNewEnvWithLog is sessionsNewEnv with a log a test can read.
func forgetNewEnvWithLog(t *testing.T) (*sessionsEnv, *sandboxEnvLog) {
	t.Helper()
	sink := &sandboxEnvLog{}
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Log = slog.New(slog.NewTextHandler(sink, nil))
	})
	env.st = st
	return env, sink
}

func TestForgetDeletesTheSessionsOwnTranscriptsAndNoOtherFile(t *testing.T) {
	env := sessionsNewEnv(t)
	src := resumeSource("s-0000000b")
	src.EarlierConversationIDs = []string{forgetEarlier}
	sessionsSeed(t, env.st, src)
	resumeSpeak(t, env, sessionsConversation)
	resumeSpeak(t, env, forgetEarlier)
	resumeSpeak(t, env, forgetOther)

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204", code)
	}
	if forgetExists(t, forgetTranscript(env, sessionsConversation)) {
		t.Errorf("Forget left the session's transcript %s", sessionsConversation)
	}
	if forgetExists(t, forgetTranscript(env, forgetEarlier)) {
		t.Errorf("Forget left %s, the conversation the session's restart cleared: it is the session's own, and once the record is gone nothing names it", forgetEarlier)
	}
	if !forgetExists(t, forgetTranscript(env, forgetOther)) {
		t.Errorf("Forget deleted %s, another session's transcript", forgetOther)
	}
	if !forgetExists(t, filepath.Join(env.conversations, "local", "w-4a77b1c9")) {
		t.Errorf("Forget removed the worktree's store; only removing the worktree may do that")
	}
	if _, err := env.st.Get("s-0000000b"); !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("the record survived the purge: err = %v", err)
	}
}

// A conversation nobody spoke in has no transcript, and neither does a session
// whose store was never made. Forget is still a clean 204, and it does not
// create the store just to find it empty.
func TestForgetOfAConversationNobodySpokeInIsNotAnError(t *testing.T) {
	env := sessionsNewEnv(t)
	src := resumeSource("s-0000000b")
	src.EarlierConversationIDs = []string{forgetEarlier}
	sessionsSeed(t, env.st, src)

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204: a transcript that was never written is already forgotten", code)
	}
	if _, err := env.st.Get("s-0000000b"); !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("the record survived the purge: err = %v", err)
	}
	if forgetExists(t, env.conversations) {
		t.Errorf("Forget created %s to delete nothing from it", env.conversations)
	}
}

// The commonest missing file is not a store that was never made but one
// conversation in a store that exists: a restart starts a new conversation,
// and until someone speaks in it the CLI has written nothing for it. Forget
// then deletes the conversation the restart cleared and is a clean 204 — the
// current one being absent is not a failure.
func TestForgetAfterARestartNobodySpokeInDeletesTheConversationItCleared(t *testing.T) {
	env := sessionsNewEnv(t)
	src := resumeSource("s-0000000b")
	src.EarlierConversationIDs = []string{forgetEarlier}
	sessionsSeed(t, env.st, src)
	resumeSpeak(t, env, forgetEarlier) // the current one, sessionsConversation, was never spoken in

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204: the current conversation's missing transcript is already forgotten", code)
	}
	if forgetExists(t, forgetTranscript(env, forgetEarlier)) {
		t.Errorf("Forget left %s, the conversation the restart cleared", forgetEarlier)
	}
	if _, err := env.st.Get("s-0000000b"); !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("the record survived the purge: err = %v", err)
	}
}

// A session with no conversation — a shell, or a record from before the
// resume fix — has nothing to forget, and that is not an anomaly: the error
// log is for refusals, and a clean Forget writes nothing to it.
func TestForgetOfASessionWithNoConversationLogsNoError(t *testing.T) {
	env, sink := forgetNewEnvWithLog(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("purge status = %d, want 204", code)
	}
	if log := sink.String(); strings.Contains(log, "level=ERROR") {
		t.Errorf("a clean Forget of a session with no conversation logged an error:\n%s", log)
	}
}

// Every id Forget deletes by is one the control plane minted, and ids.UUID
// mints one spelling. A record carrying anything else was edited or corrupted,
// could never have started a CLI (Build refuses it), and so names no transcript
// of ours — but joined onto the store it names somewhere else. The uppercase
// case is the quiet one: on Windows' case-insensitive filesystem it is another
// conversation's file.
func TestForgetNeverDeletesByAnIDTheControlPlaneCouldNotHaveMinted(t *testing.T) {
	type victim struct {
		name string
		// id goes in the record; at is where joining it onto the store lands.
		id string
		at func(env *sessionsEnv) string
		// earlier puts the id in EarlierConversationIDs, behind a good current one.
		earlier bool
	}
	cases := []victim{
		{name: "up to the owner dir", id: "../../victim",
			at: func(env *sessionsEnv) string { return filepath.Join(env.conversations, "local", "victim.jsonl") }},
		{name: "out of the conversations root", id: "../../../../victim",
			at: func(env *sessionsEnv) string { return filepath.Join(filepath.Dir(env.conversations), "victim.jsonl") }},
		{name: "another conversation by case", id: strings.ToUpper(forgetOther),
			at: func(env *sessionsEnv) string { return forgetTranscript(env, forgetOther) }},
		{name: "an earlier id, behind a good current one", id: "../../victim", earlier: true,
			at: func(env *sessionsEnv) string { return filepath.Join(env.conversations, "local", "victim.jsonl") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, sink := forgetNewEnvWithLog(t)
			src := resumeSource("s-0000000b")
			if c.earlier {
				src.EarlierConversationIDs = []string{c.id}
				resumeSpeak(t, env, sessionsConversation)
			} else {
				src.ConversationID = c.id
			}
			sessionsSeed(t, env.st, src)
			target := c.at(env)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatalf("make the victim's dir: %v", err)
			}
			if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
				t.Fatalf("write the victim: %v", err)
			}

			code := forgetPurge(t, env, "s-0000000b")
			if !forgetExists(t, target) {
				t.Fatalf("Forget of a record carrying %q deleted %s", c.id, target)
			}
			// Declared ruling: an id no mint can produce names no transcript of
			// ours, so there is nothing of the session's left to delete and the
			// purge completes — refusing it would leave a record nobody can
			// ever forget. The refusal is logged at error, never silent.
			if code != http.StatusNoContent {
				t.Errorf("purge status = %d, want 204", code)
			}
			if c.earlier && forgetExists(t, forgetTranscript(env, sessionsConversation)) {
				t.Errorf("a refused earlier id stopped Forget deleting the session's good current transcript")
			}
			if log := sink.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "s-0000000b") {
				t.Errorf("the refusal left no error naming the session in the log:\n%s", log)
			}
		})
	}
}

// A deletion that fails is reported, and it is reported by NOT forgetting: the
// record stays, so it still names the file, and Forget can be asked again. The
// alternative — record gone, file kept — is exactly the unnamed, unretained
// transcript OD-10 exists to end, with a 204 saying it had ended.
func TestAForgetThatCannotDeleteTheTranscriptKeepsTheRecordAndSaysSo(t *testing.T) {
	env, sink := forgetNewEnvWithLog(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	// A non-empty directory where the transcript goes: os.Remove refuses it on
	// every platform, which is a real deletion failure with no test hook.
	blocked := forgetTranscript(env, sessionsConversation)
	if err := os.MkdirAll(filepath.Join(blocked, "held"), 0o700); err != nil {
		t.Fatalf("make the blocker: %v", err)
	}

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	stored, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("a Forget whose deletion failed removed the record anyway: %v", err)
	}
	if stored.Status != registry.StatusExited || stored.ConversationID != sessionsConversation {
		t.Errorf("the kept record = %q/%q, want exited and still naming %q", stored.Status, stored.ConversationID, sessionsConversation)
	}
	if log := sink.String(); !strings.Contains(log, "transcript") || !strings.Contains(log, sessionsConversation) {
		t.Errorf("the log does not say which transcript could not be deleted:\n%s", log)
	}

	// The obstacle cleared, the same Forget completes.
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatalf("clear the blocker: %v", err)
	}
	resumeSpeak(t, env, sessionsConversation)
	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("the retried purge status = %d, want 204", code)
	}
	if forgetExists(t, forgetTranscript(env, sessionsConversation)) {
		t.Errorf("the retried Forget left the transcript")
	}
}

// A resume forks: the new session gets a conversation of its own, and the CLI
// writes that fork as a whole copy of the source's history re-keyed to the new
// id (observed in panacea-app/sandbox:dev, claude 2.1.274; od10-forget-report.md
// §3). So forgetting the source leaves the fork's file, and the fork can still
// be resumed.
func TestForgettingTheSourceLeavesAResumeOfItWhole(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	src := sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	resumeSpeak(t, env, sessionsConversation)

	fork := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(src.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	forkRec, err := env.st.Get(fork.ID)
	if err != nil {
		t.Fatalf("Get the fork: %v", err)
	}
	resumeSpeak(t, env, forkRec.ConversationID)

	if code := forgetPurge(t, env, src.ID); code != http.StatusNoContent {
		t.Fatalf("purge of the source status = %d, want 204", code)
	}
	if forgetExists(t, forgetTranscript(env, sessionsConversation)) {
		t.Errorf("Forget of the source left its transcript")
	}
	if !forgetExists(t, forgetTranscript(env, forkRec.ConversationID)) {
		t.Fatalf("Forget of the source deleted the fork's own transcript %s", forkRec.ConversationID)
	}
	if _, err := env.st.Get(fork.ID); err != nil {
		t.Fatalf("Forget of the source removed the fork's record: %v", err)
	}

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(fork.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	if got := valueAfter(env.be.spec(t, 1).Args, "--resume"); got != forkRec.ConversationID {
		t.Errorf("the fork's resume after its source was forgotten passed --resume %q, want the fork's own %q", got, forkRec.ConversationID)
	}
}

// The store is mounted as the agent's ~/.claude/projects, so the agent can
// swap the directory the CLI keeps transcripts in for a link to anywhere on the
// host. Forget must not delete through it: that is a host file deleted at the
// agent's choosing. Refusing is a deletion failure like any other — reported,
// and the record kept.
func TestForgetRefusesToDeleteThroughALinkTheAgentPutInTheStore(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	elsewhere := filepath.Join(filepath.Dir(env.conversations), "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatalf("make the link's target: %v", err)
	}
	victim := filepath.Join(elsewhere, sessionsConversation+".jsonl")
	if err := os.WriteFile(victim, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write the victim: %v", err)
	}
	store := filepath.Join(env.conversations, "local", "w-4a77b1c9")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatalf("make the store: %v", err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(store, "-home-dev-work")); err != nil {
		t.Skipf("this platform will not make a directory symlink here: %v", err)
	}

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	if !forgetExists(t, victim) {
		t.Fatalf("Forget deleted %s through a link in the store", victim)
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("a Forget that refused to delete removed the record anyway: %v", err)
	}
}

// The worktree id is joined onto a path too. A record whose worktree id cannot
// name a store never had one made — sessionConversationStore refuses it — so
// Forget deletes nothing under it and completes, and says so at error. ".."
// joined on would put "the store" at the conversations root itself.
func TestForgetDeletesNothingUnderAWorktreeIDThatCannotNameAStore(t *testing.T) {
	env, sink := forgetNewEnvWithLog(t)
	src := resumeSource("s-0000000b")
	src.WorktreeID = ".."
	sessionsSeed(t, env.st, src)
	victim := filepath.Join(env.conversations, "-home-dev-work", sessionsConversation+".jsonl")
	if err := os.MkdirAll(filepath.Dir(victim), 0o700); err != nil {
		t.Fatalf("make the victim's dir: %v", err)
	}
	if err := os.WriteFile(victim, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write the victim: %v", err)
	}

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Errorf("purge status = %d, want 204", code)
	}
	if !forgetExists(t, victim) {
		t.Fatalf("Forget of a record whose worktree id is %q deleted %s", src.WorktreeID, victim)
	}
	if log := sink.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "s-0000000b") {
		t.Errorf("the refusal left no error naming the session in the log:\n%s", log)
	}
}

// Only "certainly absent" means nothing to delete. A look at the transcript
// directory that fails any other way leaves Forget unable to say the file is
// gone, so it is a failure: 500, record kept.
func TestAForgetWhoseLookAtTheStoreFailsKeepsTheRecord(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	resumeSpeak(t, env, sessionsConversation)
	prev := sessionLstat
	sessionLstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { sessionLstat = prev })

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("a Forget that could not look removed the record anyway: %v", err)
	}
}

// OD-10's other half: removing a worktree deletes its store. No product action
// removes a worktree in phase 2, so this pins the seam phase 3's verb calls:
// that worktree's store goes whole, a neighbour's stays, gone already is fine,
// and an id that is not one path segment removes nothing — ".." is the whole
// owner directory.
func TestRemovingAWorktreesStoreDeletesThatStoreAndNoOther(t *testing.T) {
	env := sessionsNewEnv(t)
	resumeSpeak(t, env, sessionsConversation)
	neighbour := filepath.Join(env.conversations, "local", "w-00000002", "-home-dev-work", forgetOther+".jsonl")
	if err := os.MkdirAll(filepath.Dir(neighbour), 0o700); err != nil {
		t.Fatalf("make the neighbour's store: %v", err)
	}
	if err := os.WriteFile(neighbour, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write the neighbour's transcript: %v", err)
	}

	for _, id := range []string{"", ".", "..", "../w-4a77b1c9", `w-1\..\x`, "w-1/x", "W-4A77B1C9"} {
		if err := env.api.sessionForgetWorktreeStore(id); err == nil {
			t.Errorf("worktree id %q was accepted", id)
		}
	}
	if !forgetExists(t, forgetTranscript(env, sessionsConversation)) || !forgetExists(t, neighbour) {
		t.Fatalf("a refused worktree id removed a store")
	}

	if err := env.api.sessionForgetWorktreeStore("w-4a77b1c9"); err != nil {
		t.Fatalf("remove the store: %v", err)
	}
	if forgetExists(t, filepath.Join(env.conversations, "local", "w-4a77b1c9")) {
		t.Errorf("the worktree's store survived its removal")
	}
	if !forgetExists(t, neighbour) {
		t.Errorf("removing one worktree's store deleted a neighbour's transcript")
	}
	if err := env.api.sessionForgetWorktreeStore("w-4a77b1c9"); err != nil {
		t.Errorf("removing a store that is already gone = %v, want nil", err)
	}
}
