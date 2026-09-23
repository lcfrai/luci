# Task 04 — The licence gate

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 02, 03 · **Hours:** 2.5

## Why

`OD-11` rule 3: *"Dependencies are MIT, Apache-2.0, BSD or ISC only, checked by a licence gate in
CI. tldraw is excluded."* The spec's done-when (§9.1 item 13): *CI fails on a deliberately added MPL
package.* The gate lands before any canvas task adds a dependency, so every later addition is
checked the day it arrives.

**Why our own program and not `google/go-licenses` plus `license-checker`** (spec L22, L23): both
must be downloaded, and the corporate proxy breaks module downloads (`images/sandbox/Dockerfile:3-6`),
so neither would run on the machines this is built on. The gate needs no download: `go list` reads
the module cache, `npm ls` reads `node_modules`, and the classification is a small, tested table.
A gate that runs only in CI is a gate developers meet after they push.

## Files

- `tools/licencegate/go.mod` (new module, `go 1.23`, standard library only) and `go.work` (`use ./tools/licencegate`)
- `tools/licencegate/main.go`, `gate.go`, `spdx.go`, `classify.go`, `sources.go` (new)
- `tools/licencegate/exceptions.json` (new)
- `tools/licencegate/*_test.go` and `testdata/` (new)
- `azure-pipelines.yml` — one step, after task 03's; the three Go patterns gain `./tools/...`

## Contracts

```go
// tools/licencegate/gate.go
package main

// Allowed is OD-11 rule 3 as SPDX identifiers (plan decision P5-P). 0BSD is
// a BSD licence; the owner can drop it by deleting one line.
var Allowed = map[string]bool{
	"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"0BSD": true, "ISC": true,
}

// AllowedExceptions are the only SPDX "WITH" exceptions accepted.
var AllowedExceptions = map[string]bool{"LLVM-exception": true}

// Denied names fail whatever licence they declare, and cannot be excepted.
// OD-11 rule 3: tldraw's licence requires a commercial key in production.
var DeniedNames = []string{"tldraw", "@tldraw/", "github.com/tldraw/"}

// FirstParty is never checked: it is this repository.
var FirstParty = []string{"github.com/kpmg-au-emu/panacea-app/", "@panacea-app/", "desktop"}

type Ecosystem string

const (
	Go  Ecosystem = "go"
	Npm Ecosystem = "npm"
)

// Dep is one third-party package that ships, or (Enforced false) one that is
// only reported: an npm devDependency (P5-P, OQ-P5-2).
type Dep struct {
	Ecosystem Ecosystem
	Name      string // module path or npm package name
	Version   string
	Declared  string // SPDX expression as found, or "" when none
	Source    string // where it was read: "LICENSE", "package.json", …
	Enforced  bool
	From      string // "canvas", "control-plane", "sandboxd", "desktop", "canvas/editor"
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
	Declared   string    `json:"declared"`   // what the gate saw
	Resolved   string    `json:"resolved"`   // the SPDX expression a person read it as
	Reason     string    `json:"reason"`
	ReviewedBy string    `json:"reviewedBy"`
	ReviewedOn string    `json:"reviewedOn"` // YYYY-MM-DD
}

// Evaluate is pure: deps and exceptions in, findings out. An exception whose
// Resolved expression is itself not allowed is refused, and an exception
// naming a denied package is refused.
func Evaluate(deps []Dep, exceptions []Exception) (findings []Finding, err error)
```

```go
// tools/licencegate/spdx.go

// Expr is a parsed SPDX license expression: an id, "X WITH E", or a
// parenthesised AND / OR of expressions. AND binds tighter than OR.
type Expr interface{ allowed() bool }

// ParseSPDX parses the forms npm and the classifier produce. "UNKNOWN",
// "UNLICENSED", "SEE LICENSE IN …", "Custom: …", "LicenseRef-…", a trailing
// "*" (a guess) and "" all parse to an Expr that is never allowed.
func ParseSPDX(s string) (Expr, error)
```

```go
// tools/licencegate/classify.go

// Classify reads the licence files of one Go module directory (LICENSE,
// LICENSE.md, LICENSE.txt, LICENCE, COPYING, LICENSE-MIT, LICENSE-APACHE) and
// returns an SPDX expression. Rules:
//   - each file is matched against fixed fingerprints of MIT, Apache-2.0,
//     BSD-2-Clause, BSD-3-Clause, ISC and 0BSD; copyleft markers (GNU GENERAL
//     PUBLIC, GNU LESSER, GNU AFFERO, Mozilla Public License, European Union
//     Public Licence, Server Side Public License, Business Source License)
//     classify as the named licence and fail;
//   - LICENSE-MIT with LICENSE-APACHE is "MIT OR Apache-2.0";
//   - any other set of files, or one file with two licences, is their AND;
//   - a file that matches nothing is "LicenseRef-unclassified".
func Classify(dir string) (expr string, source string, err error)
```

```go
// tools/licencegate/sources.go

// Runner runs a command in a directory and returns stdout; tests inject one.
type Runner func(dir string, name string, args ...string) ([]byte, error)

// GoDeps lists every module linked into the shipped packages:
//   go list -deps -json ./canvas/... ./control-plane/cmd/... ./sandboxd/cmd/...
// from the repository root, keeping each package's .Module (not the standard
// library, not first party), de-duplicated by path@version, and classifies
// each .Module.Dir. All Enforced.
func GoDeps(root string, run Runner) ([]Dep, error)

// NpmDeps reads one npm project:
//   npm ls --all --omit=dev --json    → Enforced deps
//   npm ls --all --include=dev --json → the rest, not Enforced
// and each package's package.json "license" (or the deprecated "licenses"
// array, read as the AND of its entries).
func NpmDeps(dir string, from string, run Runner) ([]Dep, error)
```

```jsonc
// tools/licencegate/exceptions.json — starts empty; every entry is a review
{ "exceptions": [] }
```

```
// command line
licencegate [-root <repo>] [-exceptions tools/licencegate/exceptions.json] [-json]
exit 0  every enforced dep is pass or excepted
exit 1  at least one enforced dep failed (each printed: FAIL <eco> <name> <version> <declared> — <why>)
exit 2  the gate could not run (a command failed, a file is unreadable)
```

```yaml
# azure-pipelines.yml — appended after "Canvas editor"
  - script: cd desktop && npm ci
    displayName: Desktop dependencies
  - script: go run ./tools/licencegate -root .
    displayName: Licence gate
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestParseSPDX` (table) | `MIT` ✓; `(MIT OR Apache-2.0)` ✓; `(MIT AND MPL-2.0)` ✗; `MPL-2.0 OR MIT` ✓; `Apache-2.0 WITH LLVM-exception` ✓; `GPL-2.0 WITH Classpath-exception-2.0` ✗; `MIT*` ✗; `UNKNOWN` ✗; `""` ✗; `(MIT` → error | AND evaluated as OR; a guessed `*` accepted |
| `TestEvaluate_mplFails` | an enforced npm dep `left-pad-mpl@1.0.0` declaring `MPL-2.0` → `fail`; `Evaluate` returns it | the spec's done-when: a gate that passes MPL |
| `TestEvaluate_tldrawDeniedWhateverItDeclares` | `@tldraw/editor@3.0.0` declaring `MIT` → `fail`, *why* names OD-11 | a deny list checked only against the licence |
| `TestEvaluate_exceptionCannotCoverDenied` | an exception for `tldraw` → `Evaluate` returns an error | an exception file that launders the deny list |
| `TestEvaluate_exceptionPinsVersion` | an exception for `x@1.0.0` does not cover `x@1.0.1` | a version-agnostic exception |
| `TestEvaluate_exceptionMustResolveAllowed` | an exception with `resolved: "MPL-2.0"` → error | an exception that simply overrides |
| `TestEvaluate_devDepReportsOnly` | a non-enforced `GPL-3.0` dep → `report`, exit code 0 | P5-P reversed silently |
| `TestClassify` (fixtures in `testdata/licenses/`) | the six allowed texts each classify to their id; a GPL-3.0 text → `GPL-3.0-only`; MPL text → `MPL-2.0`; Apache + a CC-BY-4.0 section in one file → `Apache-2.0 AND LicenseRef-unclassified`; `LICENSE-MIT` + `LICENSE-APACHE` → `MIT OR Apache-2.0` | a classifier that returns MIT for any permissive-looking text |
| `TestGoDeps` (injected `Runner` returning fixture JSON) | the standard library and `github.com/kpmg-au-emu/panacea-app/...` are skipped; a module appearing under two packages is one `Dep` | first-party code failing its own gate |
| `TestNpmDeps` (fixture `npm ls` JSON + fixture `package.json` files) | `--omit=dev` entries are enforced; dev-only are not; `licenses: [{type:"MIT"},{type:"ISC"}]` → `MIT AND ISC` | dev deps enforced by accident |
| `TestMain_exitCodes` | runs `run()` with fixtures: 0 when clean, 1 with the MPL fixture, 2 when the runner errors | a gate that exits 0 on a tool failure |

## Steps

1. Write the tests and fixtures (`testdata/licenses/*.txt` are the licences' canonical texts; the
   GPL, MPL and CC texts are fixtures, never code, and carry nothing copied from any canvas).
2. Implement `spdx.go`, `classify.go`, `gate.go`, `sources.go`, `main.go` until green.
3. Run it on the real repository. For each `fail` on an **existing** dependency (the control
   plane's Kubernetes and Docker trees predate `OD-11`), read the licence file, and either add a
   reviewed exception or record the dependency in the report below as a real violation for the
   orchestrator. Do not add an exception without reading the file.
4. Write `docs/superpowers/planning/phase-5/licence-gate-first-run.md`: the gate's output on the
   repository at this commit, every exception added and why, and every real violation found.
5. Edit `go.work` and `azure-pipelines.yml`; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./tools/licencegate/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/tools/licencegate`.

```bash
go run ./tools/licencegate -root . ; echo "exit $?"
```
Expected: a table of every enforced dependency, then `exit 0` — or `exit 1` with each real
violation named in the first-run report.

```bash
go run ./tools/licencegate -root ./tools/licencegate/testdata/repo-with-mpl ; echo "exit $?"
```
Expected: `FAIL npm mpl-fixture 1.0.0 MPL-2.0 — …` and `exit 1` — the spec's done-when.

## Not claimed

- **Cargo is not scanned.** The canvas adds no Rust crate. Tauri's own crate tree predates `OD-11`
  and is outside this phase (plan open question OQ-P5-2 covers dev tooling; Cargo is named there too).
- **Fonts are not scanned** (plan OQ-P5-5).
- The classifier is a fingerprint match, not a legal reading. An exception is a person's review;
  this is not legal advice.
- The first-run report may name existing violations; fixing them is not this task's.
