package registry

import "time"

// Status is the session status the user sees.
//
// PHASE 2 SHIPS FOUR, AND ONLY FOUR. docs/AGENTIC-UX.md §4 rule 8 specifies a
// closed set of seven (PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE,
// PAUSED, DRIFTING, DONE). Five of those cannot be derived without reading agent
// output, which docs/PROGRAM.md forbids absolutely. The four below are the facts
// the control plane can actually observe. BLOCKED-ON-YOU needs the agent-runtime
// signal docs/AGENTIC-UX.md §3 calls "a hard runtime dependency, not a UI task".
// It is not invented here and must not be guessed from PTY bytes.
type Status string

const (
	// StatusStarting: the sandbox has been created and has not yet answered
	// /healthz.
	StatusStarting Status = "starting"
	// StatusRunning: the sandbox answers /healthz and its process is alive.
	StatusRunning Status = "running"
	// StatusIdle: running, but no output has crossed the relay for
	// Config.IdleAfter. Derived from a byte COUNT, never from byte content.
	StatusIdle Status = "idle"
	// StatusExited: the process ended, the sandbox was killed, or the sandbox
	// disappeared. Terminal. The record is kept — docs/FUNCTIONAL-SPEC.md:
	// "Finished sessions stay listed and readable — the transcript is the
	// artifact."
	StatusExited Status = "exited"
)

// Session is one row of the registry: everything the control plane must survive
// its own restart with.
//
// AttachToken and Endpoint are persisted and are NEVER serialised onto the REST
// surface. api.sessionJSON exists to make that a compile-time fact rather than a
// review question.
//
// No field carries omitempty, deliberately. The desktop types every one of them
// as present, and a key that disappears while a session has not exited yet is a
// key the TypeScript side would have to defend against for no reason.
type Session struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	MachineID  string `json:"machineId"`
	ProjectID  string `json:"projectId"`
	BranchID   string `json:"branchId"`
	WorktreeID string `json:"worktreeId"`

	Harness        string   `json:"harness"` // harness.ID
	Model          string   `json:"model"`
	Effort         string   `json:"effort"`
	PermissionMode string   `json:"permissionMode"`
	OpeningPrompt  string   `json:"openingPrompt"`
	Command        string   `json:"command"` // resolved argv[0], for restart
	Args           []string `json:"args"`    // resolved argv[1:], for restart

	// ConversationID is the HARNESS's id for this session's conversation — the
	// UUID the control plane minted and passed as `claude --session-id` — or ""
	// for a harness with no conversation, such as a shell. It is what a later
	// resume passes to `claude --resume`; the cockpit's own id, ID above, means
	// nothing to the CLI. Persisted and NEVER on the REST surface: it is a
	// harness implementation detail, resolved server-side from the session a
	// resume names.
	ConversationID string `json:"conversationId"`

	// EarlierConversationIDs are this session's own conversations from before
	// its latest restart, oldest first, each one the CLI wrote a transcript for.
	// A restart is "same config, clean context", so it starts the sandbox in a
	// new conversation — and without this list the one it left would still be
	// on disk with no record naming it. A resume falls back to the newest of
	// these when ConversationID has no transcript yet, which is exactly the
	// state a restart leaves until someone speaks. Same rules as ConversationID:
	// persisted, and never on the REST surface.
	EarlierConversationIDs []string `json:"earlierConversationIds"`

	// CarriedConversationID is set on a Fork only: a session whose `resume`
	// named a session in ANOTHER worktree. It is that source's conversation,
	// which the fork's first sandbox was shown read-only so the CLI could fork
	// it (`--resume` this id). It is NOT this session's conversation and nothing
	// resumes it: the fork's own is ConversationID, a whole copy the CLI wrote.
	// It is kept for Forget alone, which deletes by it what carrying left behind
	// — the staged copy, and the empty mount point Docker makes for the file in
	// this worktree's store. Same rules as ConversationID: persisted, and never
	// on the REST surface.
	CarriedConversationID string `json:"carriedConversationId"`

	Status     Status `json:"status"`
	ExitReason string `json:"exitReason"`

	Backend     string `json:"backend"`     // backend.Kind, held as a string so
	SandboxID   string `json:"sandboxId"`   // registry imports no other package
	SandboxRef  string `json:"sandboxRef"`  // container id / pod name
	Endpoint    string `json:"endpoint"`    // host:port, revalidated on every boot
	AttachToken string `json:"attachToken"` // secret; see the type comment

	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LastOutputAt time.Time `json:"lastOutputAt"`
	AttachCount  int       `json:"attachCount"` // currently-connected clients

	// ContextRef is the Phase 4 brief this session inherited, or "". Phase 2
	// stores and returns it and never interprets it.
	ContextRef string `json:"contextRef"`
}

// Clone returns a copy that shares nothing with s.
//
// A plain struct copy is not enough, and the reason is two fields: Args and
// EarlierConversationIDs are slices, so two struct "copies" write through the
// same backing array — outside the store's lock, where nothing serialises them.
// Every Session crossing the store's boundary is cloned: in on Create, out of
// Get and List, in and out of Update. A nil slice stays nil; whether the REST
// surface emits null or [] is the projection's decision, not the record's.
func (s Session) Clone() Session {
	out := s
	out.Args = cloneStrings(s.Args)
	out.EarlierConversationIDs = cloneStrings(s.EarlierConversationIDs)
	return out
}

// cloneStrings is a copy of s that shares no backing array with it, and nil
// for nil.
func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
