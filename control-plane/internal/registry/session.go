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
// A plain struct copy is not enough, and the reason is one field: Args is a
// slice, so two struct "copies" write through the same backing array — outside
// the store's lock, where nothing serialises them. Every Session crossing the
// store's boundary is cloned: in on Create, out of Get and List, in and out of
// Update. A nil Args stays nil; whether the REST surface emits null or [] is
// the projection's decision, not the record's.
func (s Session) Clone() Session {
	out := s
	if s.Args != nil {
		out.Args = make([]string, len(s.Args))
		copy(out.Args, s.Args)
	}
	return out
}
