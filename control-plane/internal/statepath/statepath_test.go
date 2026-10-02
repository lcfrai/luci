package statepath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/statepath"
)

// clearLadder empties every environment variable Dir reads, so a test starts
// from a known rung instead of from whatever the developer's machine exports.
// t.Setenv restores the previous value when the test ends and panics if the
// test has called t.Parallel, which is why nothing in this file is parallel.
func clearLadder(t *testing.T) {
	t.Helper()
	t.Setenv(statepath.EnvStateDir, "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_STATE_HOME", "")
}

// homeAt points os.UserHomeDir at dir on every platform. Go reads USERPROFILE
// on Windows and HOME everywhere else, and consults nothing else, so setting
// both makes this one test rather than two behind build tags.
func homeAt(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestEnvStateDirIsTheSharedVariableName(t *testing.T) {
	// Phase 4's discoverstore reads this exact string. Renaming it here without
	// renaming it there splits the user's state across two trees.
	if statepath.EnvStateDir != "LUCI_STATE_DIR" {
		t.Fatalf("EnvStateDir = %q, want %q", statepath.EnvStateDir, "LUCI_STATE_DIR")
	}
}

func TestStateDirEnvWinsOverEveryOtherRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	want := filepath.Join(root, "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestLocalAppDataRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	want := filepath.Join(local, "Luci", "state", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestXDGStateHomeRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	want := filepath.Join(xdg, "luci", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestHomeRung(t *testing.T) {
	clearLadder(t)
	home := t.TempDir()
	homeAt(t, home)

	want := filepath.Join(home, ".local", "state", "luci", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestTempFallbackWhenHomeIsUnknowable(t *testing.T) {
	clearLadder(t)
	// An empty HOME and USERPROFILE make os.UserHomeDir return an error —
	// "$HOME is not defined" on Unix, "%userprofile% is not defined" on Windows
	// — which is the only way to reach the last rung without deleting a real
	// user's profile.
	homeAt(t, "")

	want := filepath.Join(os.TempDir(), "luci", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestDirCreatesNothing(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := filepath.Join(t.TempDir(), "not-yet")
	t.Setenv(statepath.EnvStateDir, root)

	dir := statepath.Dir("sessions")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want a not-exist error — Dir must not create anything", dir, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want a not-exist error — Dir must not create its parent either", root, err)
	}
}

func TestDirEmptySubReturnsTheStateRoot(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)

	// filepath.Join drops the empty element and cleans the result, so the root
	// comes back with no trailing separator. Pinned so that a caller wanting the
	// root does not build a second ladder to get it.
	want := filepath.Clean(root)
	if got := statepath.Dir(""); got != want {
		t.Fatalf("Dir(\"\") = %q, want %q", got, want)
	}
}

func TestSessionsAndDiscoverAreSiblings(t *testing.T) {
	// Contract §7.1 as an assertion: Phase 2's registry and Phase 4's discover
	// store live under one parent, so one LUCI_STATE_DIR moves both.
	clearLadder(t)
	homeAt(t, t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	sessions := statepath.Dir("sessions")
	discover := statepath.Dir("discover")
	if filepath.Dir(sessions) != filepath.Dir(discover) {
		t.Fatalf("not siblings: %q and %q", sessions, discover)
	}
	if sessions == discover {
		t.Fatalf("Dir ignored its argument: both are %q", sessions)
	}
}
