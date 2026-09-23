package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// sessionBasePath is the canonical prefix every session URL is built from.
// Contract §3.1 also serves the whole surface under /api/v1, but that alias is
// task 10's rewrite and the paths this package MINTS — Location and attachPath
// — use the canonical form so the desktop has one shape to match.
const sessionBasePath = "/v1"

// sessionWorkDir is where the worktree is mounted inside every sandbox and the
// working directory the harness starts in. Contract §5.1 step 5.
const sessionWorkDir = "/home/dev/work"

// sessionConversationDir is the one part of the agent user's ~/.claude that
// outlives a sandbox. Without it Resume has nothing to resume: every sandbox
// is new, so the transcript `claude --resume` reads died with the old one.
//
// Observed, not assumed — claude 2.1.274 in panacea-app/sandbox:dev, started
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
// Per worktree because resume is same-worktree by design, so nothing ever needs
// to read another worktree's transcripts. 0o700 because a transcript holds
// whatever the agent read, and on Windows that mode is advisory: the boundary
// there is that the directory sits under the user's own state directory.
func (a *API) sessionConversationStore(worktreeID string) (string, error) {
	if !sessionStoreSegment.MatchString(worktreeID) {
		return "", fmt.Errorf("worktree id %q cannot name a directory", worktreeID)
	}
	dir := filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, worktreeID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sessionHasConversation is whether a session on this harness is given a
// conversation id and a conversation store. SupportsResume is the test because
// resuming is the only thing either is for; a shell has no conversation, and
// mounting it one would only put transcripts in reach of a terminal.
func sessionHasConversation(d harness.Descriptor) bool {
	return d.Available() && d.SupportsResume
}

// sessionMounts is everything of the host a sandbox sees: its worktree, and its
// worktree's conversation store when the harness has a conversation.
func sessionMounts(worktreePath, conversationStore string) []backend.Mount {
	mounts := []backend.Mount{{HostPath: worktreePath, ContainerPath: sessionWorkDir}}
	if conversationStore != "" {
		mounts = append(mounts, backend.Mount{HostPath: conversationStore, ContainerPath: sessionConversationDir})
	}
	return mounts
}

// sessionJSON is the session object of contract §3.10, and it is a separate
// struct from registry.Session for one reason: AttachToken, Endpoint,
// SandboxID, SandboxRef, Command and Args must never reach the wire, and they
// cannot be tagged json:"-" on the record because they must reach the registry
// file. A separate struct makes the omission a compile-time fact instead of a
// review question. sessions_test.go proves it by searching the marshalled
// bytes, not by checking a field it remembered.
type sessionJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	MachineID  string `json:"machineId"`
	ProjectID  string `json:"projectId"`
	BranchID   string `json:"branchId"`
	WorktreeID string `json:"worktreeId"`

	Harness        string `json:"harness"`
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	PermissionMode string `json:"permissionMode"`
	OpeningPrompt  string `json:"openingPrompt"`

	Status     string `json:"status"`
	ExitReason string `json:"exitReason"`

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
	return sessionJSON{
		ID:             s.ID,
		Title:          s.Title,
		MachineID:      s.MachineID,
		ProjectID:      s.ProjectID,
		BranchID:       s.BranchID,
		WorktreeID:     s.WorktreeID,
		Harness:        s.Harness,
		Model:          s.Model,
		Effort:         s.Effort,
		PermissionMode: s.PermissionMode,
		OpeningPrompt:  s.OpeningPrompt,
		Status:         string(s.Status),
		ExitReason:     s.ExitReason,
		Backend:        s.Backend,
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
var sessionErrMoved = errors.New("api: the session moved on while the watch was in flight")

// sessionCreateRequest is the body of POST /v1/sessions, contract §3.9. Every
// field but worktreeId is optional and defaulted below.
type sessionCreateRequest struct {
	MachineID      string `json:"machineId"`
	WorktreeID     string `json:"worktreeId"`
	Harness        string `json:"harness"`
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	PermissionMode string `json:"permissionMode"`
	OpeningPrompt  string `json:"openingPrompt"`
	Title          string `json:"title"`
	Resume         string `json:"resume"`
	ContextRef     string `json:"contextRef"`
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
	return "panacea-app/sandbox:dev"
}

// sessionResources refuses a half-filled Resources: Kata sizes a guest VM from
// both numbers and under-requesting either causes a guest OOM.
func (a *API) sessionResources() backend.Resources {
	if a.deps.Resources.CPUMillis > 0 && a.deps.Resources.MemoryMiB > 0 {
		return a.deps.Resources
	}
	return backend.DefaultResources
}

func (a *API) sessionReadyTimeout() time.Duration {
	if a.deps.ReadyTimeout > 0 {
		return a.deps.ReadyTimeout
	}
	return backend.ReadyTimeout
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

	req.WorktreeID = strings.TrimSpace(req.WorktreeID)
	if req.WorktreeID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "worktreeId is required.", nil)
		return
	}

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
	// it — unavailable, or cannot resume — in the contract's order.
	resume := strings.TrimSpace(req.Resume)
	var conversation, conversationStore string
	if sessionHasConversation(desc) {
		if resume != "" {
			var ok bool
			if resume, ok = a.sessionResolveResume(w, r, resume, harnessID, req.WorktreeID); !ok {
				return
			}
		}
		// Minted here and TOLD to the CLI with --session-id. The alternative,
		// learning the id the CLI picked, means reading its output.
		conversation = ids.UUID()
	}

	command, argv, err := harness.Build(harness.Request{
		Harness:       harnessID,
		Model:         model,
		Effort:        effort,
		Permission:    mode,
		OpeningPrompt: req.OpeningPrompt,
		Conversation:  conversation,
		Resume:        resume,
	})
	if err != nil {
		a.writeHarnessProblem(w, r, harnessID, desc, err)
		return
	}
	if conversation != "" {
		if conversationStore, err = a.sessionConversationStore(req.WorktreeID); err != nil {
			a.internal(w, r, "prepare the conversation store for worktree "+req.WorktreeID, err)
			return
		}
	}

	// 3. Ids and the secret.
	sessionID := ids.New("s")
	token := ids.Token()

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
	projectID, branchID := a.sessionLocate(ctx, req.WorktreeID)

	// 4. The record exists before the sandbox does. SandboxID is written here,
	// before backend.Create, so a crash between the two leaves a record the
	// reaper can match a container against. Task 13 owns the grace condition
	// that stops a sweep landing in this window killing a live create.
	if err := a.deps.Store.Create(registry.Session{
		ID:             sessionID,
		Title:          title,
		MachineID:      machineID,
		ProjectID:      projectID,
		BranchID:       branchID,
		WorktreeID:     req.WorktreeID,
		Harness:        string(harnessID),
		Model:          model,
		Effort:         string(effort),
		PermissionMode: string(mode),
		OpeningPrompt:  req.OpeningPrompt,
		Command:        command,
		Args:           argv,
		ConversationID: conversation,
		Status:         registry.StatusStarting,
		Backend:        string(a.deps.Backend.Kind()),
		SandboxID:      sessionID,
		AttachToken:    token,
		ContextRef:     req.ContextRef,
	}); err != nil {
		// Task 7's decision 3 fixes these two: an empty id is ErrInvalidID and
		// answers 400, not 409 and not 500. Neither is reachable while
		// sessionID comes from ids.New over crypto/rand, and both are mapped
		// anyway — the first time a defensive branch fires is the worst moment
		// to find out it answered 500 to a client error.
		switch {
		case errors.Is(err, registry.ErrInvalidID):
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"The registry rejected the session id.", nil)
		case errors.Is(err, registry.ErrExists):
			httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
				"A session with that id already exists.", map[string]string{"id": sessionID})
		default:
			a.internal(w, r, "record session "+sessionID, err)
		}
		return
	}

	// 5. The sandbox.
	sb, err := a.deps.Backend.Create(ctx, backend.Spec{
		ID:          sessionID,
		SessionID:   sessionID,
		Image:       a.sessionImage(),
		Command:     command,
		Args:        argv,
		AttachToken: token,
		Resources:   a.sessionResources(),
		WorkDir:     sessionWorkDir,
		Mounts:      sessionMounts(path, conversationStore),
	})
	if err != nil {
		// The reason on the record is written for the user; the raw error goes
		// to the log, because it can carry a host path.
		a.sessionLog().Error("create the sandbox", "sessionId", sessionID, "err", err)
		a.markSessionExited(sessionID, "the sandbox could not be created")
		a.writeBackendProblem(w, r, "create the sandbox", err)
		return
	}

	stored, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		sess.SandboxID = sb.ID
		sess.SandboxRef = sb.Ref
		sess.Endpoint = sb.Endpoint
		return nil
	})
	if err != nil {
		a.internal(w, r, "record the sandbox for session "+sessionID, err)
		return
	}

	// 7. The readiness watch runs on a background context, not r.Context():
	// the request's context is cancelled the moment the 201 is written.
	go a.watchSessionReady(sessionID, sb.ID, sb.Endpoint)

	// 6. Return immediately. The desktop opens the attach socket, which retries
	// while the status is starting.
	w.Header().Set("Location", sessionBasePath+"/sessions/"+sessionID)
	httpx.WriteJSON(w, http.StatusCreated, toJSON(stored, sessionBasePath))
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
func (a *API) sessionResolveResume(w http.ResponseWriter, r *http.Request, sourceID string, harnessID harness.ID, worktreeID string) (string, bool) {
	details := map[string]string{"resume": sourceID}
	src, err := a.deps.Store.Get(sourceID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"The session to resume is no longer in the list, so there is no conversation to continue. Start a new session instead.",
				details)
			return "", false
		}
		a.internal(w, r, "read the session to resume", err)
		return "", false
	}
	if src.Harness != string(harnessID) {
		name := src.Harness
		if d, ok := harness.Lookup(harness.ID(src.Harness)); ok {
			name = d.Name
		}
		details["harness"] = src.Harness
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"A session resumes on the harness it ran, and that one ran "+name+".", details)
		return "", false
	}
	// Same work, same worktree — and the conversation store is per worktree, so
	// a resume anywhere else would find no conversation to continue.
	if src.WorktreeID != worktreeID {
		details["worktreeId"] = src.WorktreeID
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"A session resumes in the worktree it ran in, because that is where its conversation is kept.", details)
		return "", false
	}
	if src.ConversationID == "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"That session has no saved conversation to continue: it was started before the app recorded one. Start a new session instead.",
			details)
		return "", false
	}
	return src.ConversationID, true
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
		out = append(out, toJSON(sess, sessionBasePath))
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
	httpx.WriteJSON(w, http.StatusOK, toJSON(sess, sessionBasePath))
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
	httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
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
			"A restart is already in flight for that session.", map[string]string{"id": id})
		return
	}
	defer sessionRestarts.release(id)

	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

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
	if desc, known := harness.Lookup(harness.ID(sess.Harness)); known && sessionHasConversation(desc) {
		conversation = ids.UUID()
		command, args, err = harness.Build(harness.Request{
			Harness:       desc.ID,
			Model:         sess.Model,
			Effort:        harness.Effort(sess.Effort),
			Permission:    harness.PermissionMode(sess.PermissionMode),
			OpeningPrompt: sess.OpeningPrompt,
			Conversation:  conversation,
		})
		if err != nil {
			a.writeHarnessProblem(w, r, desc.ID, desc, err)
			return
		}
		if conversationStore, err = a.sessionConversationStore(sess.WorktreeID); err != nil {
			a.internal(w, r, "prepare the conversation store for worktree "+sess.WorktreeID, err)
			return
		}
	}

	// One of contract §5.2's three permitted destroys.
	if sess.SandboxID != "" {
		if err := a.deps.Backend.Destroy(ctx, sess.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
			a.sessionLog().Error("restart: destroy the old sandbox",
				"sessionId", id, "sandboxId", sess.SandboxID, "err", err)
			a.writeBackendProblem(w, r, "destroy the old sandbox", err)
			return
		}
	}

	// A FRESH sandbox id, not the old one. Reusing it races a Destroy the
	// daemon has not finished and comes back as ErrAlreadyExists; minting a new
	// one also means a container that survived the destroy stops matching any
	// record, so task 13's reaper sees an orphan and removes it.
	sandboxID := ids.New("s")
	token := ids.Token()

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
	// argv and the conversation go in the same write for the same reason: a
	// resume must find the conversation the new sandbox is running.
	if _, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.SandboxID = sandboxID
		sess.SandboxRef = ""
		sess.Endpoint = ""
		sess.AttachToken = token
		sess.Command = command
		sess.Args = args
		sess.ConversationID = conversation
		sess.Status = registry.StatusStarting
		sess.ExitReason = ""
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

	sb, err := a.deps.Backend.Create(ctx, backend.Spec{
		ID:          sandboxID,
		SessionID:   sess.ID,
		Image:       a.sessionImage(),
		Command:     command,
		Args:        args,
		AttachToken: token,
		Resources:   a.sessionResources(),
		WorkDir:     sessionWorkDir,
		Mounts:      sessionMounts(path, conversationStore),
	})
	if err != nil {
		a.sessionLog().Error("restart: create the sandbox", "sessionId", id, "err", err)
		a.markSessionExited(id, "the sandbox could not be recreated")
		a.writeBackendProblem(w, r, "create the sandbox", err)
		return
	}

	// Second write, and it carries only what the create told us: the id and the
	// token are already on the record. Mirrors create's own second write.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.SandboxRef = sb.Ref
		sess.Endpoint = sb.Endpoint
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "record the restarted sandbox", err)
		return
	}

	go a.watchSessionReady(id, sb.ID, sb.Endpoint)
	httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
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
	ctx := r.Context()
	purge := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("purge")), "true")

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

	if !purge {
		httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
		return
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
