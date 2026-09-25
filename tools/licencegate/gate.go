// Command licencegate is OD-11 rule 3 as a CI step: every dependency that
// ships is MIT, Apache-2.0, BSD or ISC, and tldraw never ships. It reads only
// what is already on the machine (the Go module cache, node_modules, the Cargo
// registry cache), so it runs where module downloads do not.
package main

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

// Denied is a red stub.
func Denied(eco Ecosystem, name string) bool { return false }

// Evaluate is a red stub.
func Evaluate(deps []Dep, exceptions []Exception) (findings []Finding, err error) {
	return nil, nil
}
