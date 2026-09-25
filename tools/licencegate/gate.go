// Command licencegate is OD-11 rule 3 as a CI step: every dependency that
// ships is MIT, Apache-2.0, BSD or ISC, and tldraw never ships. It reads only
// what is already on the machine (the Go module cache, node_modules, the Cargo
// registry cache), so it runs where module downloads do not.
package main

import (
	"fmt"
	"strings"
	"time"
)

// Allowed is OD-11 rule 3 as SPDX identifiers (plan decision P5-P). 0BSD is
// a BSD licence; the owner can drop it by deleting one line.
var Allowed = map[string]bool{
	"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"0BSD": true, "ISC": true,
}

// AllowedExceptions are the only SPDX "WITH" exceptions accepted.
var AllowedExceptions = map[string]bool{"LLVM-exception": true}

type Ecosystem string

const (
	Go    Ecosystem = "go"
	Npm   Ecosystem = "npm"
	Cargo Ecosystem = "cargo" // reported only in phase 5 (OQ-P5-2, review I10)
)

// CargoEnforced is the seam for OQ-P5-2: false builds the recommended option
// (report the Cargo tree). The owner's ruling flips it; nothing else changes.
const CargoEnforced = false

// Dep is one third-party package that ships, or (Enforced false) one that is
// only reported: an npm devDependency no shipped file reaches, or any Cargo
// crate while CargoEnforced is false (P5-P, OQ-P5-2).
type Dep struct {
	Ecosystem Ecosystem
	Name      string // module path, npm package name or crate name
	Version   string
	Declared  string // SPDX expression as found, or "" when none
	Source    string // where it was read: "LICENSE", "package.json", "Cargo.toml", or "none"
	Path      string // the directory read: .Module.Dir, the npm node's "path", the crate's manifest dir
	Enforced  bool
	Via       string // why enforced: "production", or "imported by desktop/src/x.tsx"; "" when reported
	From      string // "canvas", "control-plane", "sandboxd", "desktop", "canvas/editor", "desktop/src-tauri"
	// Why, when set by a source, is the sentence a failing verdict carries
	// instead of the licence's (a Go module with no licence file, an npm link
	// that is not a scanned project, an imported name that is not installed,
	// a crate's chain to the root). It never turns a fail into a pass.
	Why string
	// Broken, when set by a source, fails the dep whatever it declares: a
	// structural fault, not a licence (a link into -root that is not a
	// scanned project; an import with nothing installed).
	Broken bool
}

type Verdict string

const (
	Pass     Verdict = "pass"
	Fail     Verdict = "fail"
	Excepted Verdict = "excepted"
	Report   Verdict = "report" // a non-enforced dep that would have failed
)

type Finding struct {
	Dep     Dep
	Verdict Verdict
	Why     string // one sentence
}

// Exception is one reviewed dependency the evaluator could not pass alone —
// typically a LICENSE file that holds a code licence and a docs licence
// (spec L4). It pins an exact version, so an upgrade is re-reviewed.
type Exception struct {
	Ecosystem  Ecosystem `json:"ecosystem"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Declared   string    `json:"declared"` // what the gate saw
	Resolved   string    `json:"resolved"` // the SPDX expression a person read it as
	Reason     string    `json:"reason"`
	ReviewedBy string    `json:"reviewedBy"`
	ReviewedOn string    `json:"reviewedOn"` // YYYY-MM-DD
}

// deniedWhy is the sentence every denied package fails with.
const deniedWhy = "denied by OD-11 rule 3: tldraw's licence requires a commercial key in production; no exception can cover it"

// Denied fails whatever licence it declares, and cannot be excepted. OD-11
// rule 3: tldraw's licence requires a commercial key in production. The match
// is exact, never a string prefix (review I9): an npm name equal to "tldraw";
// an npm name whose scope, the part before the first "/", equals "@tldraw";
// a Go module path equal to "github.com/tldraw" or under "github.com/tldraw/".
// Names compare lower-cased: npm folds legacy upper-case names, and GitHub
// owners are case-insensitive, so "github.com/TLDraw/x" is the same repository.
func Denied(eco Ecosystem, name string) bool {
	n := strings.ToLower(name)
	switch eco {
	case Npm:
		if n == "tldraw" {
			return true
		}
		if strings.HasPrefix(n, "@") {
			scope, _, _ := strings.Cut(n, "/")
			return scope == "@tldraw"
		}
	case Go:
		return n == "github.com/tldraw" || strings.HasPrefix(n, "github.com/tldraw/")
	}
	return false
}

// checkException refuses an exception that could launder a licence: one that
// names a denied package, one whose Resolved expression is not itself allowed,
// and one missing the fields that make it a review.
func checkException(e Exception) error {
	key := fmt.Sprintf("%s %s@%s", e.Ecosystem, e.Name, e.Version)
	if Denied(e.Ecosystem, e.Name) {
		return fmt.Errorf("exception %s: %s", key, deniedWhy)
	}
	if e.Ecosystem == "" || e.Name == "" || e.Version == "" || e.Reason == "" || e.ReviewedBy == "" {
		return fmt.Errorf("exception %s: ecosystem, name, version, reason and reviewedBy are all required", key)
	}
	if _, err := time.Parse("2006-01-02", e.ReviewedOn); err != nil {
		return fmt.Errorf("exception %s: reviewedOn %q is not YYYY-MM-DD", key, e.ReviewedOn)
	}
	x, err := ParseSPDX(e.Resolved)
	if err != nil {
		return fmt.Errorf("exception %s: resolved: %w", key, err)
	}
	if !x.allowed() {
		return fmt.Errorf("exception %s: resolved %q is not allowed by OD-11 rule 3, so no exception can pass it", key, e.Resolved)
	}
	return nil
}

// Evaluate is pure: deps and exceptions in, findings out, one finding per
// dep in order. An exception whose Resolved expression is itself not allowed
// is refused, and an exception naming a denied package is refused. An
// exception covers a dep only on an exact ecosystem, name, version and
// declared match, so an upgrade or a changed licence is re-reviewed.
func Evaluate(deps []Dep, exceptions []Exception) (findings []Finding, err error) {
	for _, e := range exceptions {
		if err := checkException(e); err != nil {
			return nil, err
		}
	}
	for _, d := range deps {
		findings = append(findings, judge(d, exceptions))
	}
	return findings, nil
}

func judge(d Dep, exceptions []Exception) Finding {
	failed := func(why string) Finding {
		if d.Enforced {
			return Finding{Dep: d, Verdict: Fail, Why: why}
		}
		return Finding{Dep: d, Verdict: Report, Why: why}
	}
	if Denied(d.Ecosystem, d.Name) {
		return failed(deniedWhy)
	}
	if d.Broken {
		return failed(d.Why)
	}
	x, perr := ParseSPDX(d.Declared)
	if perr == nil && x.allowed() {
		return Finding{Dep: d, Verdict: Pass, Why: d.Declared + " is allowed"}
	}
	for _, e := range exceptions {
		if e.Ecosystem == d.Ecosystem && e.Name == d.Name && e.Version == d.Version && e.Declared == d.Declared {
			return Finding{Dep: d, Verdict: Excepted, Why: "reviewed by " + e.ReviewedBy + " on " + e.ReviewedOn + " as " + e.Resolved + ": " + e.Reason}
		}
	}
	switch {
	case d.Why != "":
		return failed(d.Why)
	case perr != nil:
		return failed(perr.Error())
	case d.Declared == "":
		return failed("no licence declared")
	}
	return failed(d.Declared + " is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3)")
}
