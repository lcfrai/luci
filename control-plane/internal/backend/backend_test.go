package backend_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

func validSpec() backend.Spec {
	return backend.Spec{
		ID:          "s-3f2a91c0",
		SessionID:   "s-3f2a91c0",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		Args:        []string{"--model", "opus", "add tests, then refactor"},
		AttachToken: "a-secret",
		Resources:   backend.DefaultResources,
	}
}

// The read limit is the whole reason Dial exists as a shared function. If this
// constant is ever lowered below sandboxd's 256 KiB replay frame, re-attach
// breaks at a size light testing never reaches.
func TestMaxMessageBytesIsAboveTheLibraryDefaultAndTheReplayFrame(t *testing.T) {
	const libraryDefault = 32768
	const replayFrame = 256*1024 + 1
	if backend.MaxMessageBytes <= libraryDefault {
		t.Fatalf("MaxMessageBytes = %d, must exceed coder/websocket's %d default", backend.MaxMessageBytes, libraryDefault)
	}
	if backend.MaxMessageBytes <= replayFrame {
		t.Fatalf("MaxMessageBytes = %d, must exceed the %d-byte replay frame", backend.MaxMessageBytes, replayFrame)
	}
}

func TestResourceConversions(t *testing.T) {
	r := backend.Resources{CPUMillis: 2000, MemoryMiB: 4096}
	if got := r.NanoCPUs(); got != 2_000_000_000 {
		t.Errorf("NanoCPUs() = %d, want 2000000000", got)
	}
	if got := r.MemoryBytes(); got != 4294967296 {
		t.Errorf("MemoryBytes() = %d, want 4294967296", got)
	}
	if got := r.KubeCPU(); got != "2000m" {
		t.Errorf("KubeCPU() = %q, want %q", got, "2000m")
	}
	if got := r.KubeMemory(); got != "4096Mi" {
		t.Errorf("KubeMemory() = %q, want %q", got, "4096Mi")
	}
}

func TestDefaultResourcesAreTheDocumentedOnes(t *testing.T) {
	if backend.DefaultResources.CPUMillis != 2000 || backend.DefaultResources.MemoryMiB != 4096 {
		t.Fatalf("DefaultResources = %+v, want {CPUMillis:2000 MemoryMiB:4096}", backend.DefaultResources)
	}
}

// The comma is the case PANACEA_APP_ARGS_JSON exists for. sandboxd's legacy
// -args form splits on commas and would turn one opening prompt into two
// arguments.
func TestArgsJSONSurvivesACommaInTheOpeningPrompt(t *testing.T) {
	s := validSpec()
	want := `["--model","opus","add tests, then refactor"]`
	if got := s.ArgsJSON(); got != want {
		t.Fatalf("ArgsJSON() = %s, want %s", got, want)
	}
}

// "[]" and not "": sandboxd treats an empty PANACEA_APP_ARGS_JSON as "fall back
// to the comma-separated form", which is the form this field replaces.
func TestArgsJSONOfNoArgsIsAnEmptyJSONArray(t *testing.T) {
	var s backend.Spec
	if got := s.ArgsJSON(); got != "[]" {
		t.Fatalf("ArgsJSON() = %q, want %q", got, "[]")
	}
}

func TestEnvEntriesPutsTheContractFirstAndSortsTheRest(t *testing.T) {
	s := validSpec()
	s.Env = map[string]string{"ZED": "3", "ALPHA": "1", "MID": "2"}
	got := s.EnvEntries()
	want := []backend.EnvEntry{
		{Name: "PANACEA_APP_COMMAND", Value: "claude"},
		{Name: "PANACEA_APP_ARGS_JSON", Value: `["--model","opus","add tests, then refactor"]`},
		{Name: "PANACEA_APP_ATTACH_TOKEN", Value: "a-secret"},
		{Name: "ALPHA", Value: "1"},
		{Name: "MID", Value: "2"},
		{Name: "ZED", Value: "3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvEntries() = %#v,\nwant %#v", got, want)
	}
}

// The backstop half of the reserved-key rule (fix round 1, I1): EnvEntries
// must not render a caller-chosen PANACEA_APP_ATTACH_TOKEN even when
// ValidateSpec was never called. Calling ValidateSpec first would not
// exercise this path at all.
func TestEnvEntriesDropsAReservedKeyEvenWithoutValidateSpec(t *testing.T) {
	s := validSpec()
	s.Env = map[string]string{"PANACEA_APP_ATTACH_TOKEN": "attacker-chosen"}
	got := s.EnvEntries()

	var tokenEntries []backend.EnvEntry
	for _, e := range got {
		if e.Name == backend.EnvToken {
			tokenEntries = append(tokenEntries, e)
		}
	}
	if len(tokenEntries) != 1 || tokenEntries[0].Value != s.AttachToken {
		t.Fatalf("EnvEntries() carries %#v for %s, want exactly one entry with value %q",
			tokenEntries, backend.EnvToken, s.AttachToken)
	}
}

func TestValidateSpecAcceptsAGoodSpec(t *testing.T) {
	if err := backend.ValidateSpec(validSpec()); err != nil {
		t.Fatalf("ValidateSpec(valid) = %v, want nil", err)
	}
}

// A caller that could set PANACEA_APP_ATTACH_TOKEN through Spec.Env could
// replace the sandbox's secret with one it chose.
func TestValidateSpecRejectsAReservedEnvKey(t *testing.T) {
	s := validSpec()
	s.Env = map[string]string{"PANACEA_APP_ATTACH_TOKEN": "mine"}
	err := backend.ValidateSpec(s)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("ValidateSpec(reserved env) = %v, want an error wrapping ErrUnsupported", err)
	}
}

func TestValidateSpecRejectsUnusableFields(t *testing.T) {
	// A real absolute path for the host OS, used where only ContainerPath is
	// under test and HostPath just needs to not itself be the failure.
	absHostPath := t.TempDir()

	cases := map[string]func(*backend.Spec){
		"empty id":         func(s *backend.Spec) { s.ID = "" },
		"uppercase id":     func(s *backend.Spec) { s.ID = "S-3F2A91C0" },
		"id with a slash":  func(s *backend.Spec) { s.ID = "s-3f2a/91c0" },
		"over-long id":     func(s *backend.Spec) { s.ID = "s-000000000000000000000000000000000000000000" },
		"empty image":      func(s *backend.Spec) { s.Image = "" },
		"empty command":    func(s *backend.Spec) { s.Command = " " },
		"empty token":      func(s *backend.Spec) { s.AttachToken = "" },
		"zero cpu":         func(s *backend.Spec) { s.Resources.CPUMillis = 0 },
		"negative memory":  func(s *backend.Spec) { s.Resources.MemoryMiB = -1 },
		"relative workdir": func(s *backend.Spec) { s.WorkDir = "relative" },
		"mount with empty container path": func(s *backend.Spec) {
			s.Mounts = []backend.Mount{{HostPath: absHostPath, ContainerPath: ""}}
		},
		"mount with relative host path": func(s *backend.Spec) {
			s.Mounts = []backend.Mount{{HostPath: "relative/path", ContainerPath: "/workspace"}}
		},
	}
	for name, mangle := range cases {
		s := validSpec()
		mangle(&s)
		if err := backend.ValidateSpec(s); !errors.Is(err, backend.ErrUnsupported) {
			t.Errorf("%s: ValidateSpec = %v, want an error wrapping ErrUnsupported", name, err)
		}
	}
}

// idPattern must admit every id ids.New actually produces, for every
// documented prefix (fix round 1, M4): tightening the pattern to match Docker
// and Kubernetes naming rules must not reject a real session id.
func TestValidateSpecAcceptsEveryIDsNewPrefix(t *testing.T) {
	for _, prefix := range []string{"s", "p", "b", "w", "m"} {
		s := validSpec()
		s.ID = ids.New(prefix)
		if err := backend.ValidateSpec(s); err != nil {
			t.Errorf("ValidateSpec(ID=%q) = %v, want nil", s.ID, err)
		}
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	all := []error{
		backend.ErrNotFound, backend.ErrNotRunning, backend.ErrAlreadyExists,
		backend.ErrUnavailable, backend.ErrUnauthorized, backend.ErrUnsupported,
		backend.ErrNonLoopbackPublish,
	}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d and %d are not distinguishable: %v / %v", i, j, a, b)
			}
		}
	}
}
