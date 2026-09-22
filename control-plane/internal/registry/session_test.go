package registry_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// sampleSession is the contract's example document as a Go value. Ids are
// literals rather than ids.New calls: a random id cannot be compared against a
// golden document, and the prefixes are what this fixture is pinning.
func sampleSession() registry.Session {
	return registry.Session{
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
		Args:           []string{"--model", "opus", "--effort", "high", "add tests, then refactor"},
		Status:         registry.StatusRunning,
		ExitReason:     "",
		Backend:        "docker",
		SandboxID:      "s-3f2a91c0",
		SandboxRef:     "9c1f0b7ad3e2",
		Endpoint:       "127.0.0.1:52431",
		AttachToken:    "Qk9fV0lMTF9CRV9SRVBMQUNFRF9CWV8zMl9SQU5ET01f",
		CreatedAt:      time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 9, 18, 9, 14, 7, 0, time.UTC),
		LastOutputAt:   time.Date(2026, 9, 18, 9, 14, 6, 0, time.UTC),
		AttachCount:    1,
		ContextRef:     "",
	}
}

func TestSessionJSONKeySet(t *testing.T) {
	raw, err := json.Marshal(sampleSession())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := []string{
		"args", "attachCount", "attachToken", "backend", "branchId",
		"command", "contextRef", "createdAt", "effort", "endpoint",
		"exitReason", "harness", "id", "lastOutputAt", "machineId",
		"model", "openingPrompt", "permissionMode", "projectId", "sandboxId",
		"sandboxRef", "status", "title", "updatedAt", "worktreeId",
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("JSON keys =\n  %v\nwant\n  %v", keys, want)
	}

	// Empty in the fixture on purpose: their presence proves no field carries
	// omitempty, which is what lets the desktop type them as string.
	for _, k := range []string{"exitReason", "contextRef"} {
		if string(got[k]) != `""` {
			t.Fatalf("%s = %s, want \"\" — omitempty would drop the key entirely", k, got[k])
		}
	}
}

func TestSessionTimesSerialiseAsRFC3339UTC(t *testing.T) {
	raw, err := json.Marshal(sampleSession())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{
		`"createdAt":"2026-09-18T09:14:03Z"`,
		`"updatedAt":"2026-09-18T09:14:07Z"`,
		`"lastOutputAt":"2026-09-18T09:14:06Z"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("marshalled session does not contain %s\ngot: %s", want, raw)
		}
	}
}

func TestSessionRoundTripsThroughJSON(t *testing.T) {
	want := sampleSession()
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got registry.Session
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// reflect.DeepEqual is safe here only because every time in the fixture is
	// UTC and carries no monotonic reading. A fixture built from time.Now()
	// would fail this comparison; that is a property of time.Time rather than a
	// bug in Session, and the fix is a UTC fixture, not a looser assertion.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed the record:\n got %+v\nwant %+v", got, want)
	}
}

func TestStatusValues(t *testing.T) {
	// Four, and only four. The other three in docs/AGENTIC-UX.md cannot be
	// derived without reading agent output, which docs/PROGRAM.md forbids.
	for _, tc := range []struct {
		got  registry.Status
		want string
	}{
		{registry.StatusStarting, "starting"},
		{registry.StatusRunning, "running"},
		{registry.StatusIdle, "idle"},
		{registry.StatusExited, "exited"},
	} {
		if string(tc.got) != tc.want {
			t.Errorf("status = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestCloneDeepCopiesArgs(t *testing.T) {
	orig := sampleSession()
	clone := orig.Clone()

	clone.Args[0] = "--model=sonnet"
	if orig.Args[0] != "--model" {
		t.Fatalf("writing clone.Args[0] changed the original to %q — Clone shared the backing array", orig.Args[0])
	}

	clone.Args = append(clone.Args, "--extra")
	if len(orig.Args) != 5 {
		t.Fatalf("appending to clone.Args changed the original length to %d, want 5", len(orig.Args))
	}

	// A nil Args stays nil. Whether the REST surface emits null or [] is task
	// 11's projection to settle; the record does not invent a slice.
	var empty registry.Session
	if empty.Clone().Args != nil {
		t.Fatal("Clone fabricated an empty Args slice from a nil one")
	}
}

func TestCloneCopiesEveryOtherField(t *testing.T) {
	orig := sampleSession()
	if !reflect.DeepEqual(orig.Clone(), orig) {
		t.Fatal("Clone is not value-identical to its source — a field was dropped")
	}
}

func TestDefaultPathIsSessionsJSONUnderTheSessionsStateDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)

	want := filepath.Join(root, "sessions", "sessions.json")
	if got := registry.DefaultPath(); got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	all := []error{registry.ErrNotFound, registry.ErrExists, registry.ErrInvalidID}
	for i, a := range all {
		if a == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Fatalf("sentinels %d and %d are the same error; the API cannot then tell 400 from 404 from 409", i, j)
			}
		}
	}
}
