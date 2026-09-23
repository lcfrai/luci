package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
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

// A shell has no conversation: no id, no flag, no store, and no transcripts in
// reach of a terminal.
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
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %#v, want the worktree alone", spec.Mounts)
	}
	if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a shell session created the conversation store root (stat err = %v)", err)
	}
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

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(src.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	spec := env.be.spec(t, 0)
	fork, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := []string{"--model", "opus", "--resume", sessionsConversation, "--fork-session", "--session-id", fork.ConversationID}
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

	first := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(src.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	second := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", resumeBody(first.ID)), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	firstRec, _ := env.st.Get(first.ID)
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
	elsewhere := resumeSource("s-0000000d")
	elsewhere.WorktreeID = "w-99999999"
	shell := resumeSource("s-0000000e")
	shell.Harness, shell.Command, shell.Args, shell.ConversationID, shell.Model = "shell", "/bin/bash", []string{"-l"}, "", ""

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
		"the source ran in another worktree": {
			seed: &elsewhere, body: resumeBody(elsewhere.ID), wantStatus: http.StatusBadRequest,
			wantCode: "bad_request", wantPhrase: "worktree it ran in",
		},
		"the source ran on another harness": {
			seed: &shell, body: resumeBody(shell.ID), wantStatus: http.StatusBadRequest,
			wantCode: "bad_request", wantPhrase: "ran Shell",
		},
		"a shell cannot resume": {
			seed: &shell, body: `{"worktreeId":"w-4a77b1c9","harness":"shell","resume":"` + shell.ID + `"}`,
			wantStatus: http.StatusBadRequest, wantCode: "unsupported_option", wantPhrase: "cannot resume",
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
	want := []string{"--model", "opus", "--session-id", after.ConversationID, "add tests, then refactor"}
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
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %#v, want the worktree alone", spec.Mounts)
	}
	if after, _ := env.st.Get("s-0000000b"); after.ConversationID != "" {
		t.Errorf("a shell restart minted conversation %q", after.ConversationID)
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
