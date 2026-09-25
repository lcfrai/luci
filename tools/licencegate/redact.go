package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// npm passes every string it prints through @npmcli/redact (4.0.0 in npm
// 11.12.1, read 2026-09-26): a UUID becomes "***" and an npm token
// "npm_***". A node's "path" under a directory named by a UUID therefore
// names nothing (this program's scratchpad is one). These are its patterns.
var (
	npmRedactUUID  = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	npmRedactToken = regexp.MustCompile(`(?i)\b(npms?_)[a-zA-Z0-9]{36,48}\b`)
)

func npmRedact(s string) string {
	return npmRedactUUID.ReplaceAllString(npmRedactToken.ReplaceAllString(s, "npm_***"), "***")
}

// unredact restores each element of p that holds "***" from the filesystem:
// the one entry of its parent directory whose name npm would print the same.
// None, or more than one, is an error: the gate never guesses a path.
func unredact(p string) (string, error) {
	if !strings.Contains(p, "***") {
		return p, nil
	}
	clean := filepath.Clean(p)
	vol := filepath.VolumeName(clean)
	elems := strings.Split(strings.TrimPrefix(clean[len(vol):], string(filepath.Separator)), string(filepath.Separator))
	cur := vol + string(filepath.Separator)
	for _, e := range elems {
		if strings.Contains(e, "***") {
			entries, err := os.ReadDir(cur)
			if err != nil {
				return "", fmt.Errorf("npm printed the redacted path %s: %w", p, err)
			}
			var hits []string
			for _, de := range entries {
				if n := de.Name(); npmRedact(n) == e {
					hits = append(hits, n)
				}
			}
			if len(hits) != 1 {
				return "", fmt.Errorf("npm printed the redacted path %s, and %d entries of %s match %q", p, len(hits), cur, e)
			}
			e = hits[0]
		}
		cur = filepath.Join(cur, e)
	}
	return cur, nil
}
