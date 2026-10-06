package diagnostics_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/diagnostics"
)

const nativeOperation = "prepare private provider state"

type testCause struct{ code string }

func (e testCause) Error() string {
	return "synthetic-secret:/private/provider/auth.json?token=never-record"
}
func (e testCause) DiagnosticCode() string { return e.code }

type testEvent struct {
	At        string `json:"at"`
	Operation string `json:"operation"`
	Cause     string `json:"cause"`
}

type testEnvelope struct {
	Version int         `json:"version"`
	Events  []testEvent `json:"events"`
}

func recorderPath(state string) string {
	return filepath.Join(state, diagnostics.Directory, diagnostics.Filename)
}

func openRecorder(t *testing.T, state string) *diagnostics.Recorder {
	t.Helper()
	r, err := diagnostics.New(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func readRecorder(t *testing.T, state string) (testEnvelope, []byte) {
	t.Helper()
	data, err := os.ReadFile(recorderPath(state))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > diagnostics.MaxBytes {
		t.Fatalf("diagnostic file has %d bytes, limit %d", len(data), diagnostics.MaxBytes)
	}
	var doc testEnvelope
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 || len(doc.Events) > diagnostics.MaxEvents {
		t.Fatalf("invalid diagnostic envelope: version %d, events %d", doc.Version, len(doc.Events))
	}
	for _, event := range doc.Events {
		if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
			t.Fatalf("invalid diagnostic timestamp %q: %v", event.At, err)
		}
	}
	return doc, data
}

func record(t *testing.T, r *diagnostics.Recorder, cause error) {
	t.Helper()
	if err := r.Record(nativeOperation, cause); err != nil {
		t.Fatal(err)
	}
}

func TestRecorderRestartPreservesPriorEventsAndPrivateModes(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	record(t, r, testCause{"native_state_link"})
	before, _ := readRecorder(t, state)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r = openRecorder(t, state)
	record(t, r, testCause{"native_state_changed"})
	after, _ := readRecorder(t, state)
	if len(before.Events) != 1 || len(after.Events) != 2 || after.Events[0] != before.Events[0] || after.Events[1].Cause != "native_state_changed" {
		t.Fatalf("restart lost or replaced prior diagnostics: %#v", after.Events)
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{filepath.Dir(recorderPath(state)): 0700, recorderPath(state): 0600} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Fatalf("private diagnostic mode = %o, want %o", info.Mode().Perm(), want)
			}
		}
	}
}

func TestRecorderClassifiesWithoutPersistingRawInputs(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	tests := []struct {
		cause error
		want  string
	}{
		{testCause{"native_state_link"}, "native_state_link"},
		{testCause{"native_state_special"}, "native_state_special"},
		{testCause{"native_state_hardlink"}, "native_state_hardlink"},
		{testCause{"native_state_limit"}, "native_state_limit"},
		{fmt.Errorf("synthetic-wrapper-secret: %w", testCause{"native_state_changed"}), "native_state_changed"},
		{&os.PathError{Op: "open", Path: "/synthetic-path-secret", Err: os.ErrNotExist}, "not_exist"},
		{fmt.Errorf("synthetic-permission-secret: %w", os.ErrPermission), "permission"},
		{&os.PathError{Op: "write", Path: "/synthetic-disk-secret", Err: syscall.ENOSPC}, "no_space"},
		{testCause{"synthetic-code-secret"}, "unclassified"},
		{errors.New("synthetic-error-secret"), "unclassified"},
	}
	for _, test := range tests {
		record(t, r, test.cause)
	}
	if err := r.Record(strings.Repeat("synthetic-operation-secret", 1<<12), errors.New(strings.Repeat("synthetic-large-secret", 1<<12))); err != nil {
		t.Fatal(err)
	}
	doc, data := readRecorder(t, state)
	if len(doc.Events) != len(tests)+1 {
		t.Fatalf("got %d events, want %d", len(doc.Events), len(tests)+1)
	}
	for i, test := range tests {
		if doc.Events[i].Operation != "native_state_prepare" || doc.Events[i].Cause != test.want {
			t.Fatalf("event %d = %#v, want native_state_prepare/%s", i, doc.Events[i], test.want)
		}
	}
	last := doc.Events[len(tests)]
	if last.Operation != "unclassified" || last.Cause != "unclassified" {
		t.Fatalf("unknown inputs were not reduced to safe categories: %#v", last)
	}
	for _, forbidden := range []string{"synthetic-", "auth.json", "never-record", "token=", "/private/"} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("diagnostics persisted raw input containing %q", forbidden)
		}
	}
}

func TestRecorderRetainsNewestBoundedEvents(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	for i := 0; i < 5; i++ {
		record(t, r, testCause{"native_state_link"})
	}
	for i := 0; i < diagnostics.MaxEvents; i++ {
		record(t, r, testCause{"native_state_limit"})
	}
	doc, _ := readRecorder(t, state)
	if len(doc.Events) != diagnostics.MaxEvents {
		t.Fatalf("retained %d events, want %d", len(doc.Events), diagnostics.MaxEvents)
	}
	for _, event := range doc.Events {
		if event.Cause != "native_state_limit" {
			t.Fatal("retention kept an old event instead of a newer one")
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r = openRecorder(t, state)
	record(t, r, testCause{"native_state_changed"})
	doc, _ = readRecorder(t, state)
	if len(doc.Events) != diagnostics.MaxEvents || doc.Events[len(doc.Events)-1].Cause != "native_state_changed" {
		t.Fatal("restart did not preserve bounded retention")
	}
}

func TestRecorderConcurrentCallsDoNotLoseEvents(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	const calls = 32
	errs := make(chan error, calls)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := "native_state_link"
			if i%2 == 0 {
				code = "native_state_changed"
			}
			errs <- r.Record(nativeOperation, testCause{code})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	doc, _ := readRecorder(t, state)
	counts := map[string]int{}
	for _, event := range doc.Events {
		counts[event.Cause]++
	}
	if len(doc.Events) != calls || counts["native_state_link"] != calls/2 || counts["native_state_changed"] != calls/2 {
		t.Fatalf("concurrent records were lost or duplicated: %#v", counts)
	}
}

func TestRecorderTenantStateDirectoriesStaySeparate(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	a, b := openRecorder(t, first), openRecorder(t, second)
	record(t, a, testCause{"native_state_link"})
	record(t, b, testCause{"native_state_special"})
	_, before := readRecorder(t, second)
	record(t, a, testCause{"native_state_changed"})
	aDoc, _ := readRecorder(t, first)
	bDoc, after := readRecorder(t, second)
	if len(aDoc.Events) != 2 || len(bDoc.Events) != 1 || bDoc.Events[0].Cause != "native_state_special" || !bytes.Equal(before, after) {
		t.Fatal("one tenant's records changed another tenant's diagnostics")
	}
}

func TestRecorderRejectsMissingOrRelativeStateDirectory(t *testing.T) {
	for _, state := range []string{"", ".", filepath.Join(t.TempDir(), "missing")} {
		r, err := diagnostics.New(state)
		if r != nil {
			_ = r.Close()
		}
		if err == nil {
			t.Fatalf("accepted invalid state directory %q", state)
		}
	}
}

func TestRecorderRejectsMalformedOrOversizedStateWithoutChangingIt(t *testing.T) {
	validEvent := `{"at":"2026-10-06T01:02:03.123456789Z","operation":"native_state_prepare","cause":"native_state_link"}`
	cases := map[string]string{
		"invalid-json":       `{`,
		"wrong-version":      `{"version":2,"events":[]}`,
		"unknown-envelope":   `{"version":1,"events":[],"secret":"synthetic-secret"}`,
		"unknown-event":      `{"version":1,"events":[{"at":"2026-10-06T01:02:03Z","operation":"native_state_prepare","cause":"native_state_link","secret":"synthetic-secret"}]}`,
		"unknown-operation":  `{"version":1,"events":[{"at":"2026-10-06T01:02:03Z","operation":"synthetic-secret","cause":"native_state_link"}]}`,
		"unknown-cause":      `{"version":1,"events":[{"at":"2026-10-06T01:02:03Z","operation":"native_state_prepare","cause":"synthetic-secret"}]}`,
		"invalid-timestamp":  `{"version":1,"events":[{"at":"synthetic-secret","operation":"native_state_prepare","cause":"native_state_link"}]}`,
		"missing-timestamp":  `{"version":1,"events":[{"operation":"native_state_prepare","cause":"native_state_link"}]}`,
		"trailing-document":  `{"version":1,"events":[]} {"secret":"synthetic-secret"}`,
		"too-many-events":    `{"version":1,"events":[` + strings.TrimSuffix(strings.Repeat(validEvent+",", diagnostics.MaxEvents+1), ",") + `]}`,
		"oversized-document": `{"version":1,"events":[]}` + strings.Repeat(" ", diagnostics.MaxBytes),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			path := recorderPath(state)
			if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := diagnostics.New(state)
			if r != nil {
				_ = r.Close()
			}
			if err == nil {
				t.Fatal("accepted invalid persisted diagnostics")
			}
			if strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("load error exposed persisted data")
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != input {
				t.Fatal("failed load changed the invalid source file")
			}
		})
	}
}

func TestRecorderRejectsLinksWithoutChangingOutsideFile(t *testing.T) {
	for _, kind := range []string{"ancestor", "state", "directory", "file", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			outsideFile := filepath.Join(outside, diagnostics.Filename)
			original := []byte(`{"version":1,"events":[]}`)
			if err := os.WriteFile(outsideFile, original, 0600); err != nil {
				t.Fatal(err)
			}
			state := filepath.Join(base, "state")
			if err := os.Mkdir(state, 0700); err != nil {
				t.Fatal(err)
			}
			var from, to string
			switch kind {
			case "ancestor":
				if err := os.Mkdir(filepath.Join(outside, "state"), 0700); err != nil {
					t.Fatal(err)
				}
				from, to = outside, filepath.Join(base, "alias")
				state = filepath.Join(to, "state")
			case "state":
				if err := os.Remove(state); err != nil {
					t.Fatal(err)
				}
				from, to = outside, state
			case "directory":
				from, to = outside, filepath.Join(state, diagnostics.Directory)
			default:
				if err := os.Mkdir(filepath.Join(state, diagnostics.Directory), 0700); err != nil {
					t.Fatal(err)
				}
				from, to = outsideFile, recorderPath(state)
			}
			var linkErr error
			if kind == "hardlink" {
				linkErr = os.Link(from, to)
			} else {
				linkErr = os.Symlink(from, to)
			}
			if linkErr != nil {
				t.Skipf("filesystem link unavailable: %v", linkErr)
			}
			r, err := diagnostics.New(state)
			if r != nil {
				_ = r.Close()
			}
			if err == nil {
				t.Fatal("accepted a linked diagnostic path")
			}
			after, err := os.ReadFile(outsideFile)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatal("rejected linked path altered outside bytes")
			}
			if kind == "ancestor" || kind == "state" {
				if _, err := os.Lstat(filepath.Join(state, diagnostics.Directory)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("rejected linked ancestor still created a diagnostic directory")
				}
			}
		})
	}
}

func TestRecorderRejectsReplacedPrivateDirectoryAndClosedRecorder(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	record(t, r, testCause{"native_state_link"})
	_, original := readRecorder(t, state)
	dir := filepath.Dir(recorderPath(state))
	old := dir + "-old"
	if err := os.Rename(dir, old); err != nil {
		t.Skipf("filesystem does not permit renaming an open directory: %v", err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := r.Record(nativeOperation, testCause{"native_state_changed"}); err == nil {
		t.Fatal("record accepted a replacement private directory")
	}
	after, err := os.ReadFile(filepath.Join(old, diagnostics.Filename))
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("failed record changed the original diagnostic file")
	}
	if _, err := os.Lstat(recorderPath(state)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed record wrote into the replacement directory")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Record(nativeOperation, testCause{"native_state_link"}); err == nil {
		t.Fatal("record succeeded after close")
	}
}
