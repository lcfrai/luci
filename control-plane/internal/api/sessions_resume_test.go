package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Resume was shipped unable to resume anything, and the test that should have
// caught it mocked the client and asserted it sent `resume: from.id` — true,
// and meaningless, because nothing checked what that id meant to the CLI. These
// tests are where the meaning is: what the control plane hands `claude`, and
// where the conversation it names lives.

// resumeUUID is the shape ids.UUID mints and `claude --session-id` requires.
var resumeUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// resumeSource is a finished Claude Code session in the env's worktree whose
// record carries sessionsConversation — a literal no mint can reproduce, so an
// argv carrying it proves the value was READ from this record.
func resumeSource(id string) registry.Session {
	s := sessionsStarting(id)
	s.Status = registry.StatusExited
	s.ExitReason = "killed"
	s.Endpoint, s.AttachToken = "", ""
	s.Args = []string{"--model", "opus", "--session-id", sessionsConversation}
	s.ConversationID = sessionsConversation
	return s
}

func resumeBody(sourceID string) string {
	return `{"worktreeId":"w-4a77b1c9","harness":"claude-code","model":"opus","resume":"` + sourceID + `"}`
}

func resumeJoin(argv []string) string { return strings.Join(argv, "\x00") }

// valueAfter returns the argv element after flag, or "" when flag is absent or
// last.
func valueAfter(argv []string, flag string) string {
	i := slices.Index(argv, flag)
	if i < 0 || i+1 >= len(argv) {
		return ""
	}
	return argv[i+1]
}

// resumeSpeak is someone saying something in a conversation: the CLI writing
// its transcript into the env's worktree store. The path is spelled out rather
// than taken from sessionTranscriptPath, so a change to the layout that
// function computes fails here instead of moving both sides together. The
// contents are never read; any byte will do.
func resumeSpeak(t *testing.T, env *sessionsEnv, conversation string) {
	t.Helper()
	dir := filepath.Join(env.conversations, "local", "w-4a77b1c9", "-home-dev-work")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("make the transcript dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, conversation+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write the transcript: %v", err)
	}
}

// resumeProblemMessage is the sentence a refusal carries.
func resumeProblemMessage(t *testing.T, body []byte) string {
	t.Helper()
	var p struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p.Message
}

func TestCreateNamesTheConversationItMints(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	got := sessionsDecodeOne(t, rec, http.StatusCreated)
	sessionsAwaitWatch(t, done)

	stored, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !resumeUUID.MatchString(stored.ConversationID) {
		t.Fatalf("conversationId = %q, want a version 4 UUID; the CLI rejects anything else", stored.ConversationID)
	}
	spec := env.be.spec(t, 0)
	if valueAfter(spec.Args, "--session-id") != stored.ConversationID {
		t.Errorf("Args = %#v, want --session-id %s: the record must name the conversation the sandbox actually runs", spec.Args, stored.ConversationID)
	}
	if slices.Contains(spec.Args, "--resume") || slices.Contains(spec.Args, "--fork-session") {
		t.Errorf("Args = %#v; a fresh session resumes nothing", spec.Args)
	}
	if !slices.Equal(spec.Args, stored.Args) {
		t.Errorf("the sandbox ran %#v and the record says %#v", spec.Args, stored.Args)
	}
	// Server-side only. The desktop sends the cockpit id and the control plane
	// resolves it; a desktop that could see this would be invited to send it.
	if strings.Contains(rec.Body.String(), stored.ConversationID) {
		t.Errorf("the conversation id reached the wire: %s", rec.Body.String())
	}
}

// Two sessions never share a conversation id: the store is per worktree, and
// two sessions in one worktree writing one transcript is the failure a fork
// exists to prevent.
func TestTwoCreatesMintTwoConversations(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	a := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	b := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	sessionsAwaitWatch(t, done)

	ra, _ := env.st.Get(a.ID)
	rb, _ := env.st.Get(b.ID)
	if ra.ConversationID == "" || ra.ConversationID == rb.ConversationID {
		t.Fatalf("conversation ids %q and %q; each session needs its own", ra.ConversationID, rb.ConversationID)
	}
}

// THE SECURITY PIN. The store is ~/.claude/projects and nothing wider: /login
// writes .credentials.json into ~/.claude itself, and a mount of ~/.claude
// would carry a user's credentials into a directory that outlives the sandbox.
func TestCreateMountsOnlyThisWorktreesConversationStore(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	spec := env.be.spec(t, 0)
	want := backend.Mount{
		HostPath:      filepath.Join(env.conversations, sessionConversationOwner, "w-4a77b1c9"),
		ContainerPath: "/home/dev/.claude/projects",
	}
	if len(spec.Mounts) != 2 || spec.Mounts[1] != want {
		t.Fatalf("Mounts = %#v, want the worktree and then %#v", spec.Mounts, want)
	}
	for _, m := range spec.Mounts {
		for _, forbidden := range []string{"/home/dev", "/home/dev/.claude"} {
			if strings.TrimRight(m.ContainerPath, "/") == forbidden {
				t.Errorf("a mount covers %s, which is where /login writes credentials", m.ContainerPath)
			}
		}
	}
	// Created before the sandbox, because Docker creates a missing host path as
	// root on Linux and the agent could then write nothing into it.
	if info, err := os.Stat(want.HostPath); err != nil || !info.IsDir() {
		t.Fatalf("the store %s was not created before the sandbox: %v", want.HostPath, err)
	}
	// Not inside the worktree, where it would be committed with the code.
	if rel, err := filepath.Rel(env.dir, want.HostPath); err == nil && !strings.HasPrefix(rel, "..") {
		t.Errorf("the store %s is inside the worktree %s", want.HostPath, env.dir)
	}
}

// A shell keeps only its own private tool home: no provider conversation id,
// flag, transcript store or credential configuration reaches the terminal.
func TestCreateGivesAShellNoConversation(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"shell"}`), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	stored, _ := env.st.Get(got.ID)
	if stored.ConversationID != "" {
		t.Errorf("conversationId = %q for a shell", stored.ConversationID)
	}
	spec := env.be.spec(t, 0)
	if !slices.Equal(spec.Args, []string{"-l"}) {
		t.Errorf("Args = %#v, want [-l]", spec.Args)
	}
	home := resumeAssertShellHomeIsolation(t, env, stored, spec)
	if !slices.Equal(spec.Mounts, []backend.Mount{{HostPath: env.dir, ContainerPath: sessionWorkDir}, {HostPath: home, ContainerPath: shellHomeDir}}) {
		t.Errorf("Mounts = %#v, want exactly the worktree and this session's private home", spec.Mounts)
	}
}

// Assert the real HTTP/backend result against the recorded session identity.
// The control-plane-owned h/ home may exist while provider transcript roots
// remain absent; mounting their shared parent would expose another session.
func resumeAssertShellHomeIsolation(t *testing.T, env *sessionsEnv, stored registry.Session, spec backend.Spec) string {
	t.Helper()
	if stored.ConversationID != "" || len(stored.EarlierConversationIDs) != 0 || stored.CarriedConversationID != "" {
		t.Fatal("terminal record acquired provider conversation state")
	}
	home, err := env.api.shellHomePath(stored.WorktreeID, stored.ID)
	if err != nil || filepath.Dir(home) != filepath.Join(env.conversations, "h") {
		t.Fatal("terminal home is not bound to its own private h/ identity")
	}
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		t.Fatal("terminal's own home was not prepared before sandbox creation")
	}
	for _, providerRoot := range []string{sessionConversationOwner, "t"} {
		if _, err := os.Stat(filepath.Join(env.conversations, providerRoot)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("terminal created a provider transcript root")
		}
	}
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == sessionConversationDir || mount.ContainerPath == nativeAgentDir || mount.HostPath == env.conversations || mount.HostPath == filepath.Dir(home) {
			t.Fatal("terminal mounts shared or provider state")
		}
	}
	if spec.Env["HOME"] != shellHomeDir {
		t.Fatal("terminal does not use its private mounted home")
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "OPENAI_API_KEY", "OPENAI_BASE_URL", "CODEX_API_KEY", "CODEX_HOME", "OPENCODE_CONFIG_CONTENT", "OPENCODE_LOCAL_API_KEY"} {
		if spec.Env[key] != "" {
			t.Fatalf("terminal received provider configuration %s", key)
		}
	}
	return home
}

// THE DEFECT, pinned where it was: the cockpit id went to `claude --resume`.
// The value on --resume must be the one READ from the source's record — the
// literal sessionsConversation, which no mint can produce — and the cockpit id
// must appear nowhere in the argv at all.
func TestResumePassesTheSourcesConversationNotTheCockpitID(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	src := sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	resumeSpeak(t, env, sessionsConversation)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(src.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	spec := env.be.spec(t, 0)
	fork, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := []string{"--append-system-prompt", harness.ProjectBriefPrompt, "--model", "opus", "--resume", sessionsConversation, "--fork-session", "--session-id", fork.ConversationID}
	if resumeJoin(spec.Args) != resumeJoin(want) {
		t.Fatalf("Args = %#v\nwant   %#v", spec.Args, want)
	}
	if slices.Contains(spec.Args, src.ID) {
		t.Errorf("the cockpit id %s reached the CLI: %#v", src.ID, spec.Args)
	}
	// A fork, so a NEW id — the same value again would be two sandboxes
	// appending to one transcript.
	if fork.ConversationID == sessionsConversation || !resumeUUID.MatchString(fork.ConversationID) {
		t.Errorf("the resumed session's conversationId = %q, want a freshly minted UUID", fork.ConversationID)
	}
	// Same worktree, same store: the fork is written beside what it forked.
	if spec.Mounts[1].HostPath != filepath.Join(env.conversations, sessionConversationOwner, src.WorktreeID) {
		t.Errorf("the resumed sandbox mounts %s, not the store its source wrote to", spec.Mounts[1].HostPath)
	}
	// The source is read, never written.
	after, _ := env.st.Get(src.ID)
	if after.ConversationID != sessionsConversation || !after.UpdatedAt.Equal(src.UpdatedAt) {
		t.Errorf("resuming touched the source record: %+v", after)
	}
}

// A resumed session stores the id of ITS conversation, the fork, so resuming
// it continues the fork and not the original.
func TestAResumedSessionCanItselfBeResumed(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	src := sessionsSeed(t, env.st, resumeSource("s-0000000a"))
	resumeSpeak(t, env, sessionsConversation)

	first := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(src.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	firstRec, _ := env.st.Get(first.ID)
	resumeSpeak(t, env, firstRec.ConversationID)
	second := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(first.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	secondRec, _ := env.st.Get(second.ID)
	spec := env.be.spec(t, 1)
	if got := valueAfter(spec.Args, "--resume"); got != firstRec.ConversationID {
		t.Fatalf("the second resume passed --resume %q, want the first resume's own conversation %q", got, firstRec.ConversationID)
	}
	if secondRec.ConversationID == firstRec.ConversationID || secondRec.ConversationID == sessionsConversation {
		t.Errorf("the second fork reused an id: %q", secondRec.ConversationID)
	}
}

// Every refusal is a sentence the user can act on, and none of them costs a
// sandbox or a record: the alternative is a terminal that prints one line and
// exits.
func TestResumeRefusesWithASentenceTheUserCanActOn(t *testing.T) {
	legacy := resumeSource("s-0000000c")
	legacy.ConversationID = ""
	// A source in another worktree is no longer refused: that is a Fork, and it
	// carries the conversation (sessions_fork_test.go).
	shell := resumeSource("s-0000000e")
	shell.Harness, shell.Command, shell.Args, shell.ConversationID, shell.Model = "shell", "/bin/bash", []string{"-l"}, "", ""
	// A conversation the CLI never wrote, and a restart's earlier one that it
	// never wrote either: `claude --resume` answers both with "No conversation
	// found" and an exit, inside the terminal.
	unspoken := resumeSource("s-0000000f")
	unspoken.EarlierConversationIDs = []string{"aaaaaaaa-1111-4111-8111-111111111111"}

	for name, tc := range map[string]struct {
		seed       *registry.Session
		body       string
		wantStatus int
		wantCode   string
		wantPhrase string
	}{
		"the source is gone": {
			body: resumeBody("s-00000009"), wantStatus: http.StatusNotFound,
			wantCode: "session_not_found", wantPhrase: "Start a new session",
		},
		"the source predates conversation ids": {
			seed: &legacy, body: resumeBody(legacy.ID), wantStatus: http.StatusConflict,
			wantCode: "conflict", wantPhrase: "no saved conversation",
		},
		"the source ran on another harness": {
			seed: &shell, body: resumeBody(shell.ID), wantStatus: http.StatusBadRequest,
			wantCode: "bad_request", wantPhrase: "ran Shell",
		},
		"a shell cannot resume": {
			seed: &shell, body: `{"worktreeId":"w-4a77b1c9","harness":"shell","resume":"` + shell.ID + `"}`,
			wantStatus: http.StatusBadRequest, wantCode: "unsupported_option", wantPhrase: "cannot resume",
		},
		"nothing was ever said in the source": {
			seed: &unspoken, body: resumeBody(unspoken.ID), wantStatus: http.StatusConflict,
			wantCode: "conflict", wantPhrase: "Nothing has been said in that session yet",
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			if tc.seed != nil {
				sessionsSeed(t, env.st, *tc.seed)
			}
			before, _ := env.st.List()

			rec := env.do(t, http.MethodPost, "/v1/sessions", tc.body)
			if code := sessionsProblemCode(t, rec, tc.wantStatus); code != tc.wantCode {
				t.Errorf("error = %q, want %q", code, tc.wantCode)
			}
			var p struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !strings.Contains(p.Message, tc.wantPhrase) {
				t.Errorf("message = %q, want it to say %q", p.Message, tc.wantPhrase)
			}
			if n := len(env.be.specs); n != 0 {
				t.Errorf("backend.Create was called %d times for a resume that cannot work", n)
			}
			if after, _ := env.st.List(); len(after) != len(before) {
				t.Errorf("a record was written: %d sessions, had %d", len(after), len(before))
			}
			// Not even an empty store: looking for a transcript is not a
			// reason to create the directory it would be in.
			if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a refused resume created %s (stat err = %v)", env.conversations, err)
			}
		})
	}
}

// Restart is "same config, clean context". With a store that outlives the
// sandbox, replaying the stored argv would hand the CLI a --session-id that
// already exists ("Session ID … is already in use") and, for a resumed session,
// fork the original again. So a restart mints a new conversation and drops the
// resume.
func TestRestartMintsANewConversationAndDropsTheResume(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	resumed := sessionsStarting("s-0000000b")
	resumed.OpeningPrompt = "add tests, then refactor"
	resumed.Args = []string{"--model", "opus", "--resume", "aaaaaaaa-1111-4111-8111-111111111111", "--fork-session", "--session-id", sessionsConversation, "add tests, then refactor"}
	resumed.ConversationID = sessionsConversation
	sessionsSeed(t, env.st, resumed)

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)

	after, _ := env.st.Get("s-0000000b")
	if after.ConversationID == sessionsConversation || !resumeUUID.MatchString(after.ConversationID) {
		t.Fatalf("conversationId = %q after a restart, want a new UUID", after.ConversationID)
	}
	want := []string{"--append-system-prompt", harness.ProjectBriefPrompt, "--model", "opus", "--session-id", after.ConversationID, "add tests, then refactor"}
	spec := env.be.spec(t, 0)
	if resumeJoin(spec.Args) != resumeJoin(want) {
		t.Errorf("the restarted sandbox ran %#v, want %#v", spec.Args, want)
	}
	if resumeJoin(after.Args) != resumeJoin(spec.Args) {
		t.Errorf("the record says %#v and the sandbox ran %#v", after.Args, spec.Args)
	}
	if len(spec.Mounts) != 2 {
		t.Errorf("Mounts = %#v, want the worktree and its conversation store", spec.Mounts)
	}
}

// Everything without a conversation still REPLAYS. The stored argv here is
// one Build would never produce, so a rebuild cannot pass by coincidence.
func TestRestartReplaysAShellsStoredArgv(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sh := sessionsStarting("s-0000000b")
	sh.Harness, sh.Model, sh.Command, sh.Args = "shell", "", "/bin/bash", []string{"-l", "-i"}
	sessionsSeed(t, env.st, sh)

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)

	spec := env.be.spec(t, 0)
	if spec.Command != "/bin/bash" || !slices.Equal(spec.Args, []string{"-l", "-i"}) {
		t.Errorf("restart ran %q %#v, want the stored /bin/bash [-l -i]", spec.Command, spec.Args)
	}
	after, _ := env.st.Get("s-0000000b")
	home := resumeAssertShellHomeIsolation(t, env, after, spec)
	if !slices.Equal(spec.Mounts, []backend.Mount{{HostPath: env.dir, ContainerPath: sessionWorkDir}, {HostPath: home, ContainerPath: shellHomeDir}}) {
		t.Errorf("Mounts = %#v, want exactly the worktree and the retained terminal's private home", spec.Mounts)
	}
}

// Task 13's recovery: the control plane restarts, a new process opens the same
// registry file, and a resume issued THEN still finds the conversation. Real
// FileStore on both sides, so the JSON round trip is in the path.
func TestAResumeAfterAControlPlaneRestartStillFindsTheConversation(t *testing.T) {
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "sessions.json")

	before, err := registry.NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	first := sessionsNewEnvOver(t, dir, before, before)
	created := sessionsDecodeOne(t, first.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	conversation := valueAfter(first.be.spec(t, 0).Args, "--session-id")
	if conversation == "" {
		t.Fatal("the first session was started without a conversation id")
	}
	resumeSpeak(t, first, conversation)

	after, err := registry.NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	second := sessionsNewEnvOver(t, dir, after, after)
	sessionsDecodeOne(t, second.do(t, http.MethodPost, "/v1/sessions", resumeBody(created.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	if got := valueAfter(second.be.spec(t, 0).Args, "--resume"); got != conversation {
		t.Fatalf("after a restart the resume passed --resume %q, want %q", got, conversation)
	}
}

// sessionConversationDir works only because the image creates it first, as the
// agent user. Without that, Docker creates ~/.claude as root to hold the mount
// point, and /login can no longer write credentials — observed, in the image as
// it was before this line existed. This pins the two files to each other.
func TestTheImageCreatesTheConversationDirAsTheAgentUser(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "images", "sandbox", "Dockerfile"))
	if err != nil {
		t.Fatalf("read the sandbox Dockerfile: %v", err)
	}
	user := ""
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "USER" {
			user = fields[1]
		}
		if len(fields) >= 4 && fields[0] == "RUN" && fields[1] == "mkdir" && fields[2] == "-p" && slices.Contains(fields[3:], sessionConversationDir) {
			if user != "dev" {
				t.Fatalf("the Dockerfile creates %s as %q, want dev: a root-owned ~/.claude breaks /login", sessionConversationDir, user)
			}
			return
		}
	}
	t.Fatalf("images/sandbox/Dockerfile never creates %s; Docker will make ~/.claude root-owned to hold the mount", sessionConversationDir)
}

// The worktree id becomes a host path segment. project.WorktreeID only mints
// safe ones; this is what stops a hand-built provider's id walking the store
// out of its root.
func TestTheConversationStoreRefusesAnIDThatIsNotOneSegment(t *testing.T) {
	env := sessionsNewEnv(t)
	for _, id := range []string{"", "..", "../w-4a77b1c9", `w-1\..\x`, "w-1/x", "W-4A77B1C9", "-w", "w-" + strings.Repeat("a", 64)} {
		if dir, err := env.api.sessionConversationStore(id); err == nil {
			t.Errorf("worktree id %q made the store %s", id, dir)
		}
	}
	if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused id still created %s (stat err = %v)", env.conversations, err)
	}
	if dir, err := env.api.sessionConversationStore("w-4a77b1c9"); err != nil || dir != filepath.Join(env.conversations, "local", "w-4a77b1c9") {
		t.Errorf("sessionConversationStore(w-4a77b1c9) = %q, %v", dir, err)
	}
}

// resumeRestart restarts s-0000000b and waits for its readiness watch.
func resumeRestart(t *testing.T, env *sessionsEnv, done <-chan string) registry.Session {
	t.Helper()
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)
	after, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return after
}

// resumeOf resumes s-0000000b and returns the --resume the new sandbox got.
func resumeOf(t *testing.T, env *sessionsEnv, done <-chan string) string {
	t.Helper()
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody("s-0000000b")), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	return valueAfter(env.be.spec(t, len(env.be.specs)-1).Args, "--resume")
}

// resumeRestartable is a running Claude Code session whose record says what
// create would have written for it.
func resumeRestartable() registry.Session {
	s := sessionsStarting("s-0000000b")
	s.Args = []string{"--model", "opus", "--session-id", sessionsConversation}
	s.ConversationID = sessionsConversation
	return s
}

// The rebuild is now the only thing that keeps a plan-mode session in plan
// mode across a restart: before the resume fix restart replayed argv byte for
// byte, so it could not lose a flag. Every other restart fixture leaves both
// fields empty, which is how deleting either line from the rebuild passed the
// whole suite. Both values differ from the defaults Build falls back to.
func TestRestartKeepsThePermissionModeAndTheEffort(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	planned := resumeRestartable()
	planned.Effort, planned.PermissionMode = "high", "plan"
	planned.Args = []string{"--model", "opus", "--effort", "high", "--permission-mode", "plan", "--session-id", sessionsConversation}
	sessionsSeed(t, env.st, planned)

	after := resumeRestart(t, env, done)

	want := []string{"--append-system-prompt", harness.ProjectBriefPrompt, "--model", "opus", "--effort", "high", "--permission-mode", "plan", "--session-id", after.ConversationID}
	if spec := env.be.spec(t, 0); resumeJoin(spec.Args) != resumeJoin(want) {
		t.Fatalf("the restarted sandbox ran %#v\nwant %#v", spec.Args, want)
	}
	if after.PermissionMode != "plan" || after.Effort != "high" {
		t.Errorf("the record came back as permissionMode %q, effort %q", after.PermissionMode, after.Effort)
	}
}

// A restart clears the context, not the artifact. It points the record at a
// conversation nobody has spoken in, and before this a Resume issued then
// passed `--resume <that id>` and the CLI printed "No conversation found" and
// exited — while the conversation the restart cleared sat on disk with no
// record naming it. Now the record names every conversation its restarts left,
// and a resume continues the newest one the CLI wrote.
func TestAResumeAfterARestartContinuesTheConversationTheRestartCleared(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeRestartable())
	resumeSpeak(t, env, sessionsConversation)

	first := resumeRestart(t, env, done)
	if !slices.Equal(first.EarlierConversationIDs, []string{sessionsConversation}) {
		t.Fatalf("earlierConversationIds = %v after a restart, want the conversation it cleared, %s", first.EarlierConversationIDs, sessionsConversation)
	}

	// Nobody has spoken since the restart, so the one with a transcript is the
	// one the restart cleared.
	if got := resumeOf(t, env, done); got != sessionsConversation {
		t.Fatalf("a resume before anyone spoke after the restart passed --resume %q, want the cleared conversation %q", got, sessionsConversation)
	}

	// Once someone speaks in the restarted sandbox, that is the conversation.
	resumeSpeak(t, env, first.ConversationID)
	if got := resumeOf(t, env, done); got != first.ConversationID {
		t.Fatalf("a resume after speaking in the restarted sandbox passed --resume %q, want %q", got, first.ConversationID)
	}

	// And a second restart keeps both, oldest first, so none is ever orphaned.
	second := resumeRestart(t, env, done)
	if want := []string{sessionsConversation, first.ConversationID}; !slices.Equal(second.EarlierConversationIDs, want) {
		t.Fatalf("earlierConversationIds = %v after two restarts, want %v", second.EarlierConversationIDs, want)
	}
	// Newest first on the way back: the second restart's cleared conversation,
	// not the first one's.
	if got := resumeOf(t, env, done); got != first.ConversationID {
		t.Errorf("after two restarts the resume passed --resume %q, want the newest written conversation %q", got, first.ConversationID)
	}
}

// A conversation nobody spoke in has no transcript, so a restart does not list
// it — one dead id per restart otherwise — and a resume of a session that was
// only ever restarted is refused before anything is created.
func TestARestartOfAnUnspokenConversationLeavesNothingToResume(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeRestartable())

	after := resumeRestart(t, env, done)
	if len(after.EarlierConversationIDs) != 0 {
		t.Fatalf("earlierConversationIds = %v; the restart listed a conversation nobody spoke in", after.EarlierConversationIDs)
	}

	rec := env.do(t, http.MethodPost, "/v1/sessions", resumeBody("s-0000000b"))
	if code := sessionsProblemCode(t, rec, http.StatusConflict); code != "conflict" {
		t.Errorf("error = %q, want conflict", code)
	}
	if msg := resumeProblemMessage(t, rec.Body.Bytes()); !strings.Contains(msg, "Nothing has been said in that session yet") {
		t.Errorf("message = %q", msg)
	}
	if n := len(env.be.specs); n != 1 {
		t.Errorf("backend.Create was called %d times, want 1 — the restart's — and none for a resume that cannot work", n)
	}
}

// The restart looks for the old transcript AFTER the old sandbox is gone. Until
// then its CLI can still write the first line — the user typed just before
// pressing Restart — and a look taken before the destroy would call that
// conversation unspoken and orphan it. The hook is that last write.
func TestARestartLooksForTheOldTranscriptOnlyOnceTheOldSandboxIsGone(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, resumeRestartable())
	env.be.onDestroy = func(string) { resumeSpeak(t, env, sessionsConversation) }

	after := resumeRestart(t, env, done)
	if !slices.Equal(after.EarlierConversationIDs, []string{sessionsConversation}) {
		t.Fatalf("earlierConversationIds = %v; the transcript the old sandbox wrote as it was destroyed is orphaned", after.EarlierConversationIDs)
	}
}

// OD-8: nothing may need the look at a harness's file to work. A look that
// fails — anything but "certainly absent" — lets the resume through for the
// CLI to judge, and makes the restart keep the id it could not see.
func TestALookAtATranscriptThatFailsDegradesInsteadOfRefusing(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	prev := sessionStat
	sessionStat = func(name string) (os.FileInfo, error) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrPermission}
	}
	t.Cleanup(func() { sessionStat = prev })
	sessionsSeed(t, env.st, resumeRestartable())

	if got := resumeOf(t, env, done); got != sessionsConversation {
		t.Fatalf("a resume whose look failed passed --resume %q, want the recorded %q", got, sessionsConversation)
	}
	after := resumeRestart(t, env, done)
	if !slices.Equal(after.EarlierConversationIDs, []string{sessionsConversation}) {
		t.Errorf("earlierConversationIds = %v; a restart that could not look dropped a conversation that may exist", after.EarlierConversationIDs)
	}
}

// "A restart that cannot build its argv must fail while the old sandbox is
// still alive", and the same for the store it mounts. The store's parent here
// is a regular file, so MkdirAll fails on every platform. Moved after the
// destroy, that failure would kill a live session and answer 500.
func TestARestartThatCannotPrepareTheStoreLeavesTheOldSandboxAlive(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the blocker: %v", err)
	}
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, dir, st, st, func(d *Deps) {
		d.ConversationsDir = filepath.Join(blocker, "conversations")
	})
	env.st = st
	before := sessionsSeed(t, st, resumeRestartable())

	rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
	if code := sessionsProblemCode(t, rec, http.StatusInternalServerError); code != "internal" {
		t.Errorf("error = %q, want internal", code)
	}
	if n := env.be.destroyCount(); n != 0 {
		t.Fatalf("Destroy was called %d times; a restart that cannot prepare its store destroyed the live sandbox first", n)
	}
	if n := len(env.be.specs); n != 0 {
		t.Errorf("backend.Create was called %d times", n)
	}
	after, _ := st.Get("s-0000000b")
	if after.SandboxID != before.SandboxID || after.ConversationID != before.ConversationID || after.AttachToken != before.AttachToken {
		t.Errorf("the failed restart rewrote the record: %+v", after)
	}
}

// 0o700 is requested for the store because a transcript holds whatever the
// agent read. Windows ignores the mode bits (the boundary there is the
// profile's ACL), so this is skipped here and bites on Linux CI and in phase 8.
func TestTheConversationStoreIsPrivateToItsUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Go's MkdirAll ignores mode bits on Windows; the store's boundary there is the user profile's ACL")
	}
	env := sessionsNewEnv(t)
	dir, err := env.api.sessionConversationStore("w-4a77b1c9")
	if err != nil {
		t.Fatalf("sessionConversationStore: %v", err)
	}
	for _, d := range []string{filepath.Dir(dir), dir} {
		info, err := os.Stat(d)
		if err != nil {
			t.Fatalf("stat %s: %v", d, err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("%s has mode %o, want 700: another local user could read the transcripts", d, perm)
		}
	}
}

// OD-8's guard for the one look the control plane takes at a harness's file.
// The transcript's place was observed on one CLI version; an image that
// installs another has to be observed again before anything trusts that place.
func TestTheTranscriptLayoutIsPinnedToTheImagesCLI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "images", "sandbox", "Dockerfile"))
	if err != nil {
		t.Fatalf("read the sandbox Dockerfile: %v", err)
	}
	m := regexp.MustCompile(`@anthropic-ai/claude-code@(\S+)`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("images/sandbox/Dockerfile no longer installs a pinned @anthropic-ai/claude-code, so the transcript layout is pinned to nothing")
	}
	if got := string(m[1]); got != sessionTranscriptCLIVersion {
		t.Fatalf("the image installs claude-code %s and the transcript layout was observed on %s.\n"+
			"Observe it again in the new image — where does a conversation started in %s with --session-id <id> write <id>.jsonl? —\n"+
			"then update sessionTranscriptPath and sessionTranscriptCLIVersion together.",
			got, sessionTranscriptCLIVersion, sessionWorkDir)
	}
	if got, want := sessionTranscriptPath("store", "U"), filepath.Join("store", "-home-dev-work", "U.jsonl"); got != want {
		t.Errorf("sessionTranscriptPath = %q, want %q, the place observed on %s", got, want, sessionTranscriptCLIVersion)
	}
}
