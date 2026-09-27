package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// sampleSession is deliberately fully populated: every field §5.5 needs to
// re-attach to a live sandbox after a control-plane restart must survive the
// round trip, and a zero value would let a dropped field pass unnoticed.
func sampleSession() Session {
	at := time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC)
	return Session{
		ID:             "s-3f2a91c0",
		Title:          "evidence upload",
		MachineID:      "local",
		ProjectID:      "p-8c1d0e42",
		BranchID:       "b-8c1d0e42-evidence-upload",
		WorktreeID:     "w-4a77b1c9",
		Harness:        "claude-code",
		Model:          "opus",
		Effort:         "high",
		PermissionMode: "ask",
		OpeningPrompt:  "add tests, then refactor",
		Command:        "claude",
		Args:           []string{"--model", "opus", "--session-id", "0f3c2b1a-9d8e-4f7a-8b6c-5d4e3f2a1b0c", "add tests, then refactor"},
		ConversationID: "0f3c2b1a-9d8e-4f7a-8b6c-5d4e3f2a1b0c",
		Status:         StatusRunning,
		ExitReason:     "",
		Backend:        "docker",
		SandboxID:      "s-3f2a91c0",
		SandboxRef:     "9c1f0b7ad3e2",
		Endpoint:       "127.0.0.1:52431",
		AttachToken:    "not-a-real-token-but-shaped-like-one",
		CreatedAt:      at,
		UpdatedAt:      at.Add(4 * time.Second),
		LastOutputAt:   at.Add(3 * time.Second),
		AttachCount:    1,
		ContextRef:     "",
	}
}

func newStore(t *testing.T, path string) *FileStore {
	t.Helper()
	st, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore(%s): %v", path, err)
	}
	return st
}

func TestNewFileStoreOnAMissingFileIsAnEmptyStoreNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions on first run, want 0", len(list))
	}
	// Opening must not create the file: nothing has been mutated yet, and a
	// stray empty document would make "has this app ever run" unanswerable.
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat after open = %v, want the file to still be absent", err)
	}
}

func TestSessionsSurviveAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	want := sampleSession()

	first := newStore(t, path)
	if err := first.Create(want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := newStore(t, path).Get(want.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}

	if got.AttachToken != want.AttachToken {
		t.Fatalf("attachToken = %q, want it persisted so a restart can re-attach", got.AttachToken)
	}
	if got.Endpoint != want.Endpoint || got.SandboxRef != want.SandboxRef || got.Backend != want.Backend {
		t.Fatalf("sandbox identity lost: endpoint=%q ref=%q backend=%q", got.Endpoint, got.SandboxRef, got.Backend)
	}
	// Contents, not just length: this is the test whose whole subject is that
	// the argv survives, and a store that persisted three of the wrong strings
	// would restart the agent with the wrong model.
	if got.Command != want.Command || !slices.Equal(got.Args, want.Args) {
		t.Fatalf("restart argv lost: command=%q args=%v, want %q args=%v",
			got.Command, got.Args, want.Command, want.Args)
	}
	// The one field a resume after a control-plane restart depends on. Lose it
	// and every session created before the restart becomes one nothing can
	// resume, with no error until somebody tries.
	if want.ConversationID == "" {
		t.Fatal("the fixture carries no conversation id, so this assertion proves nothing")
	}
	if got.ConversationID != want.ConversationID {
		t.Fatalf("conversationId = %q after a reopen, want %q", got.ConversationID, want.ConversationID)
	}
	if got.Status != want.Status || got.AttachCount != want.AttachCount {
		t.Fatalf("status/attachCount = %q/%d, want %q/%d", got.Status, got.AttachCount, want.Status, want.AttachCount)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.LastOutputAt.Equal(want.LastOutputAt) {
		t.Fatalf("timestamps lost: createdAt=%s lastOutputAt=%s", got.CreatedAt, got.LastOutputAt)
	}
}

func TestDocumentUsesTheContractFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := newStore(t, path).Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	var doc struct {
		SchemaVersion int                        `json:"schemaVersion"`
		SavedAt       string                     `json:"savedAt"`
		Sessions      []map[string]any           `json:"sessions"`
		WorktreeFlags map[string]map[string]bool `json:"worktreeFlags"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the document this build wrote does not parse: %v\n%s", err, raw)
	}

	if doc.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", doc.SchemaVersion)
	}
	if doc.SavedAt == "" {
		t.Fatal("savedAt is empty; it is diagnostic but it is not optional")
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("sessions has %d entries, want 1", len(doc.Sessions))
	}
	// These spellings are a contract with the TypeScript side. A rename here is
	// a silent break there, so it fails here first.
	for _, key := range []string{
		"id", "title", "machineId", "projectId", "branchId", "worktreeId",
		"harness", "model", "effort", "permissionMode", "openingPrompt",
		"command", "args", "conversationId", "status", "exitReason", "backend", "sandboxId",
		"sandboxRef", "endpoint", "attachToken", "createdAt", "updatedAt",
		"lastOutputAt", "attachCount", "contextRef",
	} {
		if _, ok := doc.Sessions[0][key]; !ok {
			t.Errorf("session object is missing %q", key)
		}
	}
}

func corruptSiblings(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func TestUnparseableDocumentIsQuarantinedAndTheStoreStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	garbage := []byte(`{"schemaVersion":1,"sessions":[{"id":"s-3f2a91c0"`) // truncated mid-write
	if err := os.WriteFile(path, garbage, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions from an unparseable document, want 0", len(list))
	}
	aside := corruptSiblings(t, path)
	if len(aside) != 1 {
		t.Fatalf("found %d quarantine files, want exactly 1: %v", len(aside), aside)
	}
	kept, err := os.ReadFile(aside[0])
	if err != nil {
		t.Fatalf("read quarantine file: %v", err)
	}
	if string(kept) != string(garbage) {
		t.Fatalf("quarantine file = %q, want the original bytes verbatim", kept)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s = %v, want the unreadable document moved, not copied", path, err)
	}
}

func TestSchemaVersionFromTheFutureIsAHardErrorAndTouchesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	future := []byte(`{"schemaVersion":2,"savedAt":"2026-09-18T09:14:07Z","sessions":[]}`)
	if err := os.WriteFile(path, future, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st, err := NewFileStore(path)
	if err == nil {
		t.Fatalf("NewFileStore on a newer document returned a store (%v), want an error", st)
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("error = %v, want it to wrap ErrSchemaTooNew", err)
	}
	// The contract requires the message to name both versions, because the only
	// useful action is "run the newer build", and the user needs to know which.
	msg := err.Error()
	if !strings.Contains(msg, "2") || !strings.Contains(msg, "1") {
		t.Fatalf("error = %q, want it to name both the document and the binary version", msg)
	}
	// Nothing is moved aside: the file is not corrupt, it is from the future,
	// and a newer build must find it exactly where it left it.
	if aside := corruptSiblings(t, path); len(aside) != 0 {
		t.Fatalf("a newer document was quarantined: %v", aside)
	}
	still, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(still) != string(future) {
		t.Fatalf("document = %q, want it untouched", still)
	}
}

func TestSchemaVersionBelowOneIsQuarantined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte(`{"sessions":[]}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions, want 0", len(list))
	}
	if aside := corruptSiblings(t, path); len(aside) != 1 {
		t.Fatalf("found %d quarantine files, want exactly 1: %v", len(aside), aside)
	}
}

func TestLoadIgnoresALeftoverTempFile(t *testing.T) {
	// A crash between statepath.WriteAtomic's Write and its Rename leaves a temp file next
	// to an intact document. The next start must read the document and ignore
	// the debris: the cost of the crash is the newest mutation, never the file.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := newStore(t, path).Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sessions-4172.json"), []byte(`{"schemaVersion":1,`), 0o600); err != nil {
		t.Fatalf("seed debris: %v", err)
	}

	list, err := newStore(t, path).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}
	if aside := corruptSiblings(t, path); len(aside) != 0 {
		t.Fatalf("the intact document was quarantined because of debris: %v", aside)
	}
}

func TestWorktreeReadyFlagSurvivesAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	got, err := newStore(t, path).SetWorktreeReady("w-4a77b1c9", true)
	if err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}
	if !got.Ready {
		t.Fatal("SetWorktreeReady returned ready=false")
	}

	flags := newStore(t, path).WorktreeFlagsSnapshot()
	if !flags["w-4a77b1c9"].Ready {
		t.Fatalf("after reopen flags = %v, want w-4a77b1c9 ready", flags)
	}
	// Clearing it persists too, or "mark ready" would be a one-way door.
	if _, err := newStore(t, path).SetWorktreeReady("w-4a77b1c9", false); err != nil {
		t.Fatalf("SetWorktreeReady(false): %v", err)
	}
	if newStore(t, path).WorktreeFlagsSnapshot()["w-4a77b1c9"].Ready {
		t.Fatal("ready=false did not persist")
	}
}

func TestUnknownWorktreeIsNotReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	st := newStore(t, path)
	if _, err := st.SetWorktreeReady("w-4a77b1c9", true); err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}

	// The property is the store's, not Go's: the snapshot holds a row for every
	// worktree the registry has been told about and for no other. That is what
	// makes the zero value a truthful answer — decision 5, "a worktree the
	// registry has never seen is not an error, it is a worktree that is not
	// ready". A store that pre-seeded rows, or defaulted a miss to true, would
	// fail here rather than in the API.
	snap := st.WorktreeFlagsSnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot holds %d rows, want only the one that was set: %v", len(snap), snap)
	}
	if _, present := snap["w-never-seen"]; present {
		t.Fatalf("the store invented a row for a worktree it was never told about: %v", snap)
	}
	if snap["w-never-seen"].Ready {
		t.Fatal("an unknown worktree reported ready")
	}
}

func TestWorktreeFlagsSnapshotIsACopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	st := newStore(t, path)
	if _, err := st.SetWorktreeReady("w-4a77b1c9", true); err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}

	snap := st.WorktreeFlagsSnapshot()
	snap["w-4a77b1c9"] = WorktreeFlags{Ready: false}
	snap["w-injected"] = WorktreeFlags{Ready: true}

	again := st.WorktreeFlagsSnapshot()
	if !again["w-4a77b1c9"].Ready {
		t.Fatal("mutating the snapshot changed the store")
	}
	if _, ok := again["w-injected"]; ok {
		t.Fatal("the snapshot shares its map with the store")
	}
}

func TestSetWorktreeReadyRejectsAnEmptyID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	if _, err := newStore(t, path).SetWorktreeReady("", true); err == nil {
		t.Fatal("SetWorktreeReady(\"\") returned nil, want an error")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a rejected call still wrote the document: %v", err)
	}
}

func TestSessionsAndWorktreeFlagsShareOneDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	st := newStore(t, path)
	if err := st.Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.SetWorktreeReady("w-4a77b1c9", true); err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	var doc struct {
		Sessions      []map[string]any           `json:"sessions"`
		WorktreeFlags map[string]map[string]bool `json:"worktreeFlags"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1: writing a flag dropped the sessions", len(doc.Sessions))
	}
	if !doc.WorktreeFlags["w-4a77b1c9"]["ready"] {
		t.Fatalf("worktreeFlags = %v, want w-4a77b1c9 ready", doc.WorktreeFlags)
	}
}
