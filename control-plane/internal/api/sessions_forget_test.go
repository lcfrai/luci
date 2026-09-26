package api

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
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

// forgetCarried is the conversation a fork carried from its source: the
// source's id, which a Forget of the fork deletes by name from the fork's own
// store (the empty mount point Docker made there) and from its staging.
const forgetCarried = "c0ffee00-0000-4000-8000-0000ca441ed0"

// forgetEverything seeds rec, a Claude Code session in the env's worktree, as
// a record naming every kind of id Forget deletes by — current, earlier,
// carried — and puts on disk every file a Forget of it deletes. It returns
// those files' paths, so a test can say which verbs must leave all of them.
func forgetEverything(t *testing.T, env *sessionsEnv, rec registry.Session) []string {
	t.Helper()
	rec.ConversationID = sessionsConversation
	rec.EarlierConversationIDs = []string{forgetEarlier}
	rec.CarriedConversationID = forgetCarried
	sessionsSeed(t, env.st, rec)
	paths := []string{
		forgetTranscript(env, sessionsConversation),
		forgetTranscript(env, forgetEarlier),
		forgetTranscript(env, forgetCarried),
		forkStaged(env, rec.ID, forgetCarried),
	}
	for _, p := range paths {
		forkWrite(t, p, []byte("{}\n"))
	}
	return paths
}

// Kill is not Forget. A plain DELETE ends the sandbox and keeps every file a
// Resume, a Restart or a later Forget reads — docs/FUNCTIONAL-SPEC.md "Session
// lifecycle", contract §5.2 — so the forget sits below the purge check, and
// this is what holds it there: above it, every Kill would erase the
// conversation Resume needs, for good.
func TestAKillDeletesNoneOfWhatAForgetWould(t *testing.T) {
	env := sessionsNewEnv(t)
	paths := forgetEverything(t, env, sessionsStarting("s-0000000b"))

	if code := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b", "").Code; code != http.StatusOK {
		t.Fatalf("kill status = %d, want 200", code)
	}
	for _, p := range paths {
		if !forgetExists(t, p) {
			t.Errorf("a Kill deleted %s", p)
		}
	}

	// The killed record still names all of it, so the Forget that comes later
	// finds all of it.
	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("the Forget after the Kill = %d, want 204", code)
	}
	for _, p := range paths {
		if forgetExists(t, p) {
			t.Errorf("the Forget after a Kill left %s", p)
		}
	}
}

// A Forget whose destroy fails has not ended the sandbox. The CLI in it is
// probably still running and still writing, and the record is kept still
// saying so — and the files are kept with it, so the forget sits below the
// destroy. Above it, a 503 would come back over a live session whose
// conversation was already gone.
func TestAForgetWhoseDestroyFailsDeletesNothing(t *testing.T) {
	env := sessionsNewEnv(t)
	paths := forgetEverything(t, env, sessionsStarting("s-0000000b"))
	env.be.destroyErr = backend.ErrUnavailable

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusServiceUnavailable {
		t.Fatalf("purge status = %d, want 503", code)
	}
	for _, p := range paths {
		if !forgetExists(t, p) {
			t.Errorf("a Forget whose destroy failed deleted %s", p)
		}
	}
	if stored, err := env.st.Get("s-0000000b"); err != nil || stored.Status != registry.StatusStarting {
		t.Errorf("the record after a failed destroy = %+v, %v; want it kept as it was", stored, err)
	}

	// The daemon back, the same Forget deletes all of it.
	env.be.destroyErr = nil
	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Fatalf("the retried purge status = %d, want 204", code)
	}
	for _, p := range paths {
		if forgetExists(t, p) {
			t.Errorf("the retried Forget left %s", p)
		}
	}
}

// Forget is one pass over every file, and a deletion that fails does not stop
// it: each of the others is tried, and the failures come back together. The
// record is kept either way, so nothing is lost by stopping — but the pass is
// what the handler's comment and the report promise, and this holds it, in both
// halves: the store's loop, and the staged copy after it.
func TestAForgetThatCannotDeleteOneFileStillDeletesTheRest(t *testing.T) {
	env := sessionsNewEnv(t)
	paths := forgetEverything(t, env, resumeSource("s-0000000b"))
	// The first file the pass reaches, made undeletable: a non-empty directory.
	blocked := paths[0]
	if err := os.Remove(blocked); err != nil {
		t.Fatalf("clear the file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(blocked, "held"), 0o700); err != nil {
		t.Fatalf("make the blocker: %v", err)
	}

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusInternalServerError {
		t.Fatalf("purge status = %d, want 500", code)
	}
	for _, p := range paths[1:] {
		if forgetExists(t, p) {
			t.Errorf("one failed deletion stopped the Forget before %s", p)
		}
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("a Forget with a failed deletion removed the record: %v", err)
	}
}

// A resume forks: the new session gets a conversation of its own, and the CLI
// writes that fork as a whole copy of the source's history re-keyed to the new
// id (observed in panacea-app/sandbox:dev, claude 2.1.274, in `-p` mode;
// od10-forget-report.md §3; the interactive fork is unverified until OD-12). So
// forgetting the source leaves the fork's file, and the fork can still be
// resumed.
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

// forgetLink makes link a link to the directory target, the way an agent's
// swap would leave it, and removes the link itself — never what it points at —
// before the test's directories go. A junction on Windows, which needs no
// privilege; a symlink elsewhere. It reports whether it could.
func forgetLink(t *testing.T, link, target string) bool {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Logf("mklink /J %s: %v: %s", link, err, out)
			return false
		}
	} else if err := os.Symlink(target, link); err != nil {
		t.Logf("symlink %s: %v", link, err)
		return false
	}
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("remove the link %s: %v", link, err)
		}
	})
	return true
}

// The look at the directory and the deletes are two steps, and the store is
// live between them: it is mounted into every other session's sandbox in the
// worktree. An agent there that swaps the directory for a link after the look
// must get no deletion through it. The name a deletion carries is not only the
// session's own id: a fork's record names its SOURCE's conversation
// (carriedConversationId), so a link to the source worktree's transcript
// directory would have deleted the source's conversation. The swap here lands
// exactly in that gap.
func TestForgetDeletesNothingThroughALinkSwappedInAfterItsLook(t *testing.T) {
	env := sessionsNewEnv(t)
	probe := filepath.Join(t.TempDir(), "probe")
	if !forgetLink(t, probe, t.TempDir()) {
		t.Skip("this platform will not make a directory link here")
	}
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	resumeSpeak(t, env, sessionsConversation)
	dir := filepath.Dir(forgetTranscript(env, sessionsConversation))
	elsewhere := filepath.Join(filepath.Dir(env.conversations), "elsewhere")
	victim := filepath.Join(elsewhere, sessionsConversation+".jsonl")
	forkWrite(t, victim, []byte("{}\n"))

	prev := sessionLstat
	swapped := false
	sessionLstat = func(name string) (os.FileInfo, error) {
		info, err := prev(name)
		if name == dir && !swapped {
			swapped = true
			if err := os.Rename(dir, dir+"-moved"); err != nil {
				t.Errorf("move the real directory aside: %v", err)
			} else if !forgetLink(t, dir, elsewhere) {
				t.Errorf("swap the link in")
			}
		}
		return info, err
	}
	t.Cleanup(func() { sessionLstat = prev })

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if !swapped {
		t.Fatalf("Forget never looked at %s", dir)
	}
	if !forgetExists(t, victim) {
		t.Errorf("Forget deleted %s through a link swapped in after its look", victim)
	}
	if !forgetExists(t, filepath.Join(dir+"-moved", sessionsConversation+".jsonl")) {
		t.Errorf("the real transcript went although Forget refused")
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("a Forget that refused to delete removed the record anyway: %v", err)
	}
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
}

// A link in the directory's place is refused even when nothing behind it
// carries the name: the look refuses it, not the deletes. Otherwise the
// deletes would find no file, call the transcript already gone and answer 204
// — over a transcript the agent only moved out of the way.
func TestForgetRefusesALinkInTheStoreWithNothingBehindIt(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	store := filepath.Join(env.conversations, "local", "w-4a77b1c9")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatalf("make the store: %v", err)
	}
	if !forgetLink(t, filepath.Join(store, "-home-dev-work"), t.TempDir()) {
		t.Skip("this platform will not make a directory link here")
	}

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("a Forget that refused a link removed the record anyway: %v", err)
	}
}

// A link at a transcript's own name is the link, not a way through: it is
// unlinked and what it points at is left alone.
func TestForgetUnlinksALinkAtTheTranscriptsNameAndLeavesItsTarget(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	target := filepath.Join(t.TempDir(), "precious.txt")
	forkWrite(t, target, []byte("keep\n"))
	link := forgetTranscript(env, sessionsConversation)
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatalf("make the transcript dir: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform will not make a file symlink here: %v", err)
	}

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Errorf("purge status = %d, want 204", code)
	}
	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the link at the transcript's name is still there: %v", err)
	}
	if !forgetExists(t, target) {
		t.Fatalf("Forget deleted the link's target %s", target)
	}
}

// What os.Remove deleted, Forget still deletes: a transcript made read-only,
// and an empty directory where a transcript would be. Either left would make a
// Forget that fails on every retry.
func TestForgetDeletesAReadOnlyTranscriptAndAnEmptyDirectoryInOnesPlace(t *testing.T) {
	env := sessionsNewEnv(t)
	rec := resumeSource("s-0000000b")
	rec.EarlierConversationIDs = []string{forgetEarlier}
	sessionsSeed(t, env.st, rec)
	resumeSpeak(t, env, sessionsConversation)
	readOnly := forgetTranscript(env, sessionsConversation)
	if err := os.Chmod(readOnly, 0o400); err != nil {
		t.Fatalf("make the transcript read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o600) })
	empty := forgetTranscript(env, forgetEarlier)
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatalf("make the empty directory: %v", err)
	}

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Errorf("purge status = %d, want 204", code)
	}
	if forgetExists(t, readOnly) {
		t.Errorf("Forget left the read-only transcript")
	}
	if forgetExists(t, empty) {
		t.Errorf("Forget left the empty directory in a transcript's place")
	}
}

// A store deep enough that its transcripts' paths run past Windows' MAX_PATH
// (260) is forgotten like any other. os.Remove reached such a file; a deletion
// that could not would find "no such path", call the transcript already gone,
// and answer 204 over it.
func TestForgetReachesATranscriptPastMaxPath(t *testing.T) {
	st := sessionsMustStore(t)
	deep := filepath.Join(t.TempDir(), strings.Repeat("d", 120), strings.Repeat("e", 60))
	env := sessionsNewEnvOver(t, deep, st, st)
	env.st = st
	sessionsSeed(t, env.st, resumeSource("s-0000000b"))
	resumeSpeak(t, env, sessionsConversation)
	transcript := forgetTranscript(env, sessionsConversation)
	if len(transcript) <= 260 {
		t.Fatalf("the transcript's path is %d long; the test needs one past 260", len(transcript))
	}

	if code := forgetPurge(t, env, "s-0000000b"); code != http.StatusNoContent {
		t.Errorf("purge status = %d, want 204", code)
	}
	if forgetExists(t, transcript) {
		t.Errorf("Forget answered but left the transcript %d characters deep", len(transcript))
	}
}

// The look at whether a transcript was written joins the record's id onto the
// store too, and Build's spelling rule holds there as it does for Forget: an id
// no mint produces names no transcript, whatever its join would find. "../x"
// joined on is <store>/x.jsonl.
func TestAnIDNoMintProducesNamesNoTranscriptForTheLook(t *testing.T) {
	store := t.TempDir()
	forkWrite(t, filepath.Join(store, "x.jsonl"), []byte("{}\n"))
	written, err := sessionTranscriptState(store, "../x")
	if written || err != nil {
		t.Errorf("sessionTranscriptState(../x) = %v, %v; want certainly absent", written, err)
	}
}

// So a resume of a record naming such an id finds nothing to continue and says
// so, before any sandbox starts — rather than choosing whatever the join
// reached and failing later.
func TestAResumeOfARecordNamingAnIDNoMintProducesContinuesNothing(t *testing.T) {
	env := sessionsNewEnv(t)
	src := resumeSource("s-0000000b")
	src.ConversationID = "../x"
	src.Args = []string{"--model", "opus"}
	sessionsSeed(t, env.st, src)
	forkWrite(t, filepath.Join(env.conversations, "local", "w-4a77b1c9", "x.jsonl"), []byte("{}\n"))

	rec := env.do(t, http.MethodPost, "/v1/sessions", resumeBody("s-0000000b"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("resume status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	env.be.mu.Lock()
	created := len(env.be.specs)
	env.be.mu.Unlock()
	if created != 0 {
		t.Errorf("%d sandboxes were created for a resume with nothing to continue", created)
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
