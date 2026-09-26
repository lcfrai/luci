package gitx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// MinVersion is the oldest host git phase 3 runs on: merge-tree --write-tree needs 2.38,
// merge-tree --merge-base (revert, task 10) needs 2.40, and a fetch from a bundle file runs
// fsck over its objects only from 2.48.0 (RelNotes/2.48.0.adoc: "git bundle --unbundle" and
// "git clone" on a bundle "learned to trigger fsck"; read, not run). Task 24's FetchLane
// relies on that fsck, so 2.48.0 is the floor (review r4-1 M-4).
const MinVersion = "2.48.0"

// versionCall is the one call CheckVersion makes. Scope None with Dir "" is admitted for it
// alone (§HGH "Call-site scopes": None is for `git version`); `git version` reads no repository.
func versionCall() Call {
	return Call{Args: []string{"version"}, Class: Read, Scope: None}
}

// CheckVersion runs `git version`, parses "git version 2.53.0.windows.2", and returns the
// version string, or an error whose message is the sentence the user acts on.
func CheckVersion(ctx context.Context, r Runner) (string, error) {
	res, err := r.Run(ctx, versionCall())
	if err != nil {
		if ge, ok := AsError(err); ok {
			return "", fmt.Errorf("git could not be run to check its version (%s): install Git %s or newer and put it on PATH", ge.Sentence(), MinVersion)
		}
		return "", fmt.Errorf("git could not be run to check its version: %w", err)
	}
	line := strings.TrimSpace(string(res.Stdout))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	f := strings.Fields(line)
	if len(f) < 3 || f[0] != "git" || f[1] != "version" {
		return "", fmt.Errorf("git answered %q to `git version`, which is not a version: install Git %s or newer", line, MinVersion)
	}
	have, ok := parseVersion(f[2])
	if !ok {
		return "", fmt.Errorf("git answered %q to `git version`, which is not a version: install Git %s or newer", line, MinVersion)
	}
	want, _ := parseVersion(MinVersion)
	if less(have, want) {
		return "", fmt.Errorf("git %s is older than %s, the oldest git this app runs on: install Git %s or newer", f[2], MinVersion, MinVersion)
	}
	return f[2], nil
}

// parseVersion reads the leading "major.minor[.patch]" of "2.53.0.windows.2" as numbers, so
// 2.9 < 2.40 (a lexical compare would say otherwise).
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return v, false
	}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			if i == 2 {
				break // "2.48.rc0"-style: major and minor are enough
			}
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
