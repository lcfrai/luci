// Package statepath answers one question: where on this machine does the control
// plane keep state. It exists because two packages need the same answer —
// registry (Phase 2) and discoverstore (Phase 4) — and two copies of an OS
// ladder drift the moment one of them gains a case.
package statepath

import (
	"os"
	"path/filepath"
)

// EnvStateDir overrides every other rule.
const EnvStateDir = "LUCI_STATE_DIR"

// Dir returns the directory for a named kind of state, e.g. Dir("sessions") or
// Dir("discover"). The ladder, in order:
//
//	$LUCI_STATE_DIR/<sub>
//	%LOCALAPPDATA%\Luci\state\<sub>        (Windows)
//	$XDG_STATE_HOME/luci/<sub>
//	~/.local/state/luci/<sub>
//	<os.TempDir()>/luci/<sub>             (only if the home directory is unknowable)
//
// The rungs are chosen by environment rather than by runtime.GOOS on purpose:
// %LOCALAPPDATA% is simply unset off Windows, so the same code walks the same
// ladder everywhere and one test covers every platform.
//
// Dir does not create anything. The caller does, because the caller knows
// whether a missing directory is an error.
func Dir(sub string) string {
	if dir := os.Getenv(EnvStateDir); dir != "" {
		return filepath.Join(dir, sub)
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "Luci", "state", sub)
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "luci", sub)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "luci", sub)
	}
	return filepath.Join(home, ".local", "state", "luci", sub)
}
