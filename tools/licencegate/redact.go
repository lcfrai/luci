package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// splitPath is p's volume and its elements, after filepath.Clean.
func splitPath(p string) (vol string, elems []string) {
	clean := filepath.Clean(p)
	vol = filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean[len(vol):], string(filepath.Separator))
	if rest == "" {
		return vol, nil
	}
	return vol, strings.Split(rest, string(filepath.Separator))
}

// npmPlace is one directory the gate knows unredacted under which npm puts
// nodes (fix round 1, review I1): form is a spelling npm may print it in (the
// path as the gate was handed it, or its canon form), place its canon form.
type npmPlace struct{ form, place string }

// npmRestorer turns a path npm ls printed back into the real path.
type npmRestorer []npmPlace

// newRestorer knows the places one npm project's nodes live: the project
// directory, where npm installs, and each link target its package-lock.json
// names (resolved relative to the project directory, never redacted: the
// lockfile is read from disk, not from npm's output), under which the
// target's own dependencies print. -root is not one: nothing is installed at
// it, and a link target inside it is named by the lockfile already.
func newRestorer(dir, cdir string, locks map[string]lockEntry) npmRestorer {
	bases := []string{cdir}
	if abs, err := filepath.Abs(dir); err == nil && filepath.Clean(abs) != cdir {
		bases = append(bases, filepath.Clean(abs))
	}
	var r npmRestorer
	for _, b := range bases {
		r = append(r, npmPlace{b, cdir})
	}
	locs := make([]string, 0, len(locks))
	for loc, e := range locks {
		if e.Link {
			locs = append(locs, loc)
		}
	}
	sort.Strings(locs)
	for _, loc := range locs {
		rel := filepath.FromSlash(locks[loc].Resolved)
		for _, b := range bases {
			form := filepath.Join(b, rel)
			place := form
			if c, err := canon(form); err == nil {
				place = c // a target that is gone stays as written: state() exits 2 on it
			}
			r = append(r, npmPlace{form, place})
			if place != form {
				r = append(r, npmPlace{place, place})
			}
		}
	}
	return r
}

// sameElem compares one element npm printed with one the gate knows.
func sameElem(printed, known string) bool {
	if caseInsensitivePaths {
		return strings.EqualFold(printed, npmRedact(known))
	}
	return printed == npmRedact(known)
}

// restore maps p to the real path. A path under exactly one known place, or
// under several nested in one another, is that place plus the rest; a path
// under two known places neither of which holds the other is an error; a
// path under none is restored from the filesystem by unredact.
func (r npmRestorer) restore(p string) (string, error) {
	if !strings.Contains(p, "***") {
		return p, nil
	}
	pv, pe := splitPath(p)
	var hits []npmPlace
	best, bestN := npmPlace{}, -1
	for _, a := range r {
		av, ae := splitPath(a.form)
		if len(ae) > len(pe) || !sameElem(pv, av) {
			continue
		}
		match := true
		for i := range ae {
			if !sameElem(pe[i], ae[i]) {
				match = false
				break
			}
		}
		if match {
			hits = append(hits, a)
			if len(ae) > bestN {
				best, bestN = a, len(ae)
			}
		}
	}
	if len(hits) == 0 {
		return unredact(p)
	}
	for _, h := range hits {
		if !insideCanon(h.place, best.place) {
			return "", fmt.Errorf("npm printed the redacted path %s, which names both %s and %s: the gate never guesses a path", p, h.place, best.place)
		}
	}
	return unredactUnder(best.form, pe[bestN:], p)
}

// unredact restores each element of p that holds "***" from the filesystem:
// the one entry of its parent directory whose name npm would print the same.
// None, or more than one, is an error: the gate never guesses a path.
func unredact(p string) (string, error) {
	if !strings.Contains(p, "***") {
		return p, nil
	}
	vol, elems := splitPath(p)
	return unredactUnder(vol+string(filepath.Separator), elems, p)
}

// unredactUnder is unredact of the elements below cur; p is what npm printed.
func unredactUnder(cur string, elems []string, p string) (string, error) {
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
