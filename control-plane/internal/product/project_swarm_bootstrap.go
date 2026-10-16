package product

import (
	"errors"
	"runtime"
	"strings"
	"unicode/utf8"
)

var errSwarmBootstrapPath = errors.New("Windows Git cannot initialize this task workspace at the configured Projects path. Choose a shorter Projects path for new work. Existing work was not moved.")

// This is a conservative ordinary-path bootstrap policy, not a claim that
// enabling core.longpaths can open an already-too-long initial config. Keep it
// on new allocation: existing history, inspection and retirement remain valid.
func swarmBootstrapFeasibility(destination string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	return windowsSwarmBootstrapFeasibility(destination)
}

func windowsSwarmBootstrapFeasibility(destination string) error {
	if !utf8.ValidString(destination) || strings.ContainsRune(destination, 0) || !windowsBootstrapUnitsFit(destination, "", 248) {
		return errSwarmBootstrapPath
	}
	ordinary := strings.ReplaceAll(destination, `\`, "/")
	drive := len(ordinary) >= 3 && ((ordinary[0] >= 'A' && ordinary[0] <= 'Z') || (ordinary[0] >= 'a' && ordinary[0] <= 'z')) && ordinary[1:3] == ":/"
	unc := strings.HasPrefix(ordinary, "//") && !strings.HasPrefix(ordinary, "//?/") && !strings.HasPrefix(ordinary, "//./")
	parts := strings.Split(ordinary, "/")
	if unc {
		unc = len(parts) >= 5 && parts[2] != "" && parts[3] != ""
	}
	if !drive && !unc {
		return errSwarmBootstrapPath
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return errSwarmBootstrapPath
		}
	}
	// Managed init already disables templates. Reserve the ordinary bootstrap
	// metadata directories and files before any local longpaths setting exists.
	for _, suffix := range []string{".git", ".git/objects/info", ".git/objects/pack", ".git/refs/heads", ".git/refs/tags"} {
		if !windowsBootstrapUnitsFit(ordinary, suffix, 248) {
			return errSwarmBootstrapPath
		}
	}
	for _, suffix := range []string{".git/config", ".git/config.lock", ".git/HEAD", ".git/description", ".git/info/exclude"} {
		if !windowsBootstrapUnitsFit(ordinary, suffix, 260) {
			return errSwarmBootstrapPath
		}
	}
	return nil
}

func windowsBootstrapUnitsFit(base, suffix string, maximum int) bool {
	units := 2 + len(suffix) // fixed ASCII suffix, separator and terminating NUL
	for _, r := range base {
		units++
		if r > 0xffff {
			units++
		}
		if units > maximum {
			return false
		}
	}
	return units <= maximum
}
