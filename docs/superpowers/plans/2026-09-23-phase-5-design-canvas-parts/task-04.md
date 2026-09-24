# Task 04 — The licence gate

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 02, 03 · **Hours:** 4 (2.5
before fix batch b05, which adds the shipped-import scan, the Cargo report and structural first
party: review I9, I10)

## Why

`OD-11` rule 3: *"Dependencies are MIT, Apache-2.0, BSD or ISC only, checked by a licence gate in
CI. tldraw is excluded."* The spec's done-when (§9.1 item 13): *CI fails on a deliberately added MPL
package.* The gate lands before any canvas task adds a dependency, so every later addition is
checked the day it arrives.

**Why our own program and not `google/go-licenses` plus `license-checker`** (spec L22, L23): both
must be downloaded, and the corporate proxy breaks module downloads (`images/sandbox/Dockerfile:3-6`),
so neither would run on the machines this is built on. The gate needs no download: `go list` reads
the module cache, `npm ls` reads `node_modules`, `cargo metadata --offline` reads the Cargo registry
cache, and the classification is a small, tested table. A gate that runs only in CI is a gate
developers meet after they push.

**What "ships" means here (review I9).** Vite bundles whatever a shipped source file imports,
whichever `package.json` list names it, and `canvas/editor` ships **source** (`task-03.md:57-60`).
So the npm line between enforced and reported is not `dependencies` versus `devDependencies`: it is
*reached from shipped source* versus *not*. A package is enforced when it is a production dependency,
or when a non-test file under `desktop/src` or `canvas/editor/src` imports it, together with that
package's own production closure. The review's first wording, "enforce every dependency of
`canvas/editor`", is wrong on one fact: `canvas/editor`'s dev list holds `vitest`
(`task-03.md:52`); `vitest` 5.0.1 takes `vite` as a peer dependency
(`desktop/node_modules/vitest/package.json:124`); the installed `vite` 8.3.0 declares `lightningcss`
as a dependency (`vite/package.json:59`), whose licence is `MPL-2.0` (`lightningcss/package.json:4`;
all three verified 2026-09-24 in the main tree's `desktop/node_modules`). Enforcing test tooling fails
the gate on day one on a build tool that never ships, and a gate that fails on noise gets excepted
into silence. The import rule catches the review's case, `html-to-image` in a dev list and imported
by the export code, and passes `vitest`.

## Files

- `tools/licencegate/go.mod` (new module, `go 1.23`, standard library only) and `go.work` (`use ./tools/licencegate`)
- `tools/licencegate/main.go`, `gate.go`, `spdx.go`, `classify.go`, `sources.go`, `imports.go` (new)
- `tools/licencegate/imports.cjs` and `imports.test.cjs` (new): the one Node helper, which asks the
  project's own `typescript` parser for each file's import specifiers (review I9, hole 1)
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

// Denied fails whatever licence it declares, and cannot be excepted. OD-11
// rule 3: tldraw's licence requires a commercial key in production. The match
// is exact, never a string prefix (review I9): an npm name equal to "tldraw";
// an npm name whose scope, the part before the first "/", equals "@tldraw";
// a Go module path equal to "github.com/tldraw" or under "github.com/tldraw/".
// "tldraw-helpers" is not denied by name; its licence is checked like any
// other, and tldraw's own licence text classifies to a LicenseRef, which no
// rule allows.
func Denied(eco Ecosystem, name string) bool

// First party is decided by structure, never by name (review I9). There is no
// name list. A package is first party, and never checked, when:
//   - Go: its package's .Module.Main is true (under go.work every workspace
//     module is Main: verified 2026-09-24, control-plane and sandboxd both
//     report "Main": true), or .Module.Replace is a directory replacement
//     (Replace.Version "") whose Replace.Dir lies inside -root;
//   - npm: it is the project's root node, or it is a link node, "resolved"
//     starting "file:", whose target lies inside -root. The target is the rest
//     of "resolved" joined to the node's "path" (verified on npm 11.12.1: a
//     link at a/node_modules/@panacea-app/b carries "resolved": "file:../../../b",
//     which is a sibling b/);
//   - Cargo: a package with "source": null whose manifest_path lies inside -root.
// "Inside -root" is filepath.Rel with no leading "..", after filepath.Abs on
// both. A registry package named "desktop-notify", "@panacea-app/x" or
// "github.com/kpmg-au-emu/panacea-app/x" (a published copy with Main false) is
// third party; so is a file: link or a replace directory outside -root.
// Each source in sources.go applies its line of this rule, through one helper:
func insideRoot(root, dir string) bool

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
//   - a file that matches nothing is "LicenseRef-unclassified";
//   - a directory with none of these files returns ("", "none", nil). "" never
//     parses to an allowed Expr, so a Go module with no licence file fails,
//     with the why "no licence file in <dir>" (review I9, hole 3). It is not
//     an error: exit 2 is for a gate that could not run, and this one ran.
func Classify(dir string) (expr string, source string, err error)
```

```go
// tools/licencegate/sources.go

// Runner runs a command in a directory and returns stdout; tests inject one.
type Runner func(dir string, name string, args ...string) ([]byte, error)

// GoDeps lists every module linked into the shipped packages:
//   go list -deps -json ./canvas/... ./control-plane/cmd/... ./sandboxd/cmd/...
// from the repository root, keeping each package's .Module (not the standard
// library, .Standard true; not first party by the structural rule in gate.go),
// de-duplicated by path@version, and classifies each .Module.Dir, or the
// .Module.Replace.Dir of a non-local replacement. All Enforced. A module whose
// directory holds no licence file fails (Classify's "none").
func GoDeps(root string, run Runner) ([]Dep, error)

// NpmDeps reads one npm project, with --long so every node carries "path"
// (verified on npm 11.12.1; plain --json carries only version, resolved and
// overridden, so it cannot say which package.json an entry is — review I9,
// hole 4):
//   npm ls --all --omit=dev --json --long    → the production closure: Enforced, Via "production"
//   npm ls --all --include=dev --json --long → every installed node
// Nodes are keyed by "path", never by name: two copies of one name at two
// paths are two Deps, each read from its own <path>/package.json "license"
// (or the deprecated "licenses" array, read as the AND of its entries). A node
// seen at one path under two parents has its children unioned, so a closure
// is right whether or not npm prints a deduplicated node's children
// (unverified which it does). Then, for each package ShippedImports returns,
// the node at <dir>/node_modules/<name>, the directory Node resolution reaches
// first from a file under src/ (no src/ directory has a node_modules), and its
// production closure are Enforced, Via "imported by <file>". An imported name
// with no node there fails: "imported by <file> but not installed". Every
// other node is reported. The project root and in-root file: links are skipped
// (gate.go); a first-party link is not descended, because its own project is
// scanned as its own From.
func NpmDeps(root, dir, from string, run Runner) ([]Dep, error)

// CargoDeps reports one Cargo workspace (review I10):
//   cargo metadata --offline --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc
// run in desktop/src-tauri. --locked refuses to rewrite Cargo.lock and
// --offline refuses the network; the filter keeps the Windows target, the one
// the owner's machines run. Each package in "packages" that is not first party
// becomes a Dep with Declared = its "license" (or "" with Source
// "license_file: <path>" when only license_file is set), Enforced =
// CargoEnforced. Cargo's deprecated "/" separator (e.g. "Unlicense/MIT") is
// read as OR, here only, because Cargo documents it so; ParseSPDX is unchanged.
// Why names the crate's first chain to the root through normal (not build,
// not dev) edges in resolve.nodes, e.g. "tauri 2.11.5 → tauri-utils 2.9.3 →
// dom_query 0.27.0", and "(proc-macro)" when the crate's only target is a
// proc-macro, which runs at compile time and does not link. The chain is
// approximate: cargo metadata unifies features across build and normal use.
// If cargo is not on PATH, or the command fails, CargoDeps returns no deps and
// notScanned = the error text, and err nil, while CargoEnforced is false: the
// run prints "REPORT cargo: not scanned: <error>" and the exit code does not
// change. With CargoEnforced true the same failure is err, so exit 2. A
// silent skip is not an outcome: the line always prints.
func CargoDeps(root, dir string, run Runner) (deps []Dep, notScanned string, err error)
```

```go
// tools/licencegate/imports.go

// ShippedImports returns each bare package name the shipped source of one npm
// project imports, with the first file that imports it (review I9, hole 1).
// The walk starts at every .ts/.tsx/.js/.jsx/.mjs/.css file under srcDir that
// is not a test file (the name matches *.test.* or *.spec.*, or the file lies
// under a directory named "test"), and follows relative imports from a walked
// file to a fixed point, so a test helper that shipped code imports is walked
// too. Specifiers come from imports.cjs for script files and from CSS
// `@import "x"` / `@import url("x")` (after /* */ comments are removed) for
// .css. It skips `import type` / `export type` statements (erased before
// bundling; `import { type A, b }` is kept, because b ships) and specifiers
// starting ".", "/", "node:", "virtual:" or "\x00". "@s/n/sub" names "@s/n",
// "n/sub" names "n", and a "?query" suffix is dropped.
func ShippedImports(dir, srcDir string, run Runner) (map[string]string, error)
```

```js
// tools/licencegate/imports.cjs <projectDir> <file>…
// Loads <projectDir>/node_modules/typescript, parses each file with
// ts.createSourceFile (ScriptKind TSX for .tsx/.jsx), and prints JSON
// [{file, specifier, typeOnly}] for ImportDeclaration, ExportDeclaration,
// import("x") and require("x") with a string-literal argument. Why a parser
// and not a regular expression: desktop/src/components/develop/CanvasLayout.tsx:97,
// a shipped file, has a comment reading `from "this id is merely absent"`,
// which a from-"x" pattern reads as an import; JSX text holds apostrophes and
// a regex literal such as /"/g holds a quote, so a hand tokenizer is wrong too.
// Verified 2026-09-24 with typescript 6.0.3: a probe over a fixture holding
// all three, and over CanvasLayout.tsx, returned exactly the real imports.
```

```jsonc
// tools/licencegate/exceptions.json — starts empty; every entry is a review
{ "exceptions": [] }
```

```
// command line
licencegate [-root <repo>] [-exceptions tools/licencegate/exceptions.json] [-json]
scans   Go: GoDeps from -root. npm: NpmDeps and ShippedImports for desktop (src: desktop/src) and
        for canvas/editor (src: canvas/editor/src). Cargo: CargoDeps in desktop/src-tauri.
exit 0  every enforced dep is pass or excepted
exit 1  at least one enforced dep failed (each printed: FAIL <eco> <name> <version> <declared> — <why>)
exit 2  the gate could not run (a command failed, a file is unreadable)
every non-enforced dep that would have failed prints REPORT <eco> <name> <version> <declared> — <why>,
and never changes the exit code; the Cargo lines print whether or not cargo ran (CargoDeps)
```

```yaml
# azure-pipelines.yml — appended after "Canvas editor"
  - script: cd desktop && npm ci
    displayName: Desktop dependencies
  - script: go run ./tools/licencegate -root .
    displayName: Licence gate
```

The gate needs `node` (for `npm ls` and `imports.cjs`, already on the agent for the desktop steps)
and, for the Cargo report only, `cargo` with a populated registry cache. Whether the CI agent has
`cargo` is **unverified**; without it the run prints `REPORT cargo: not scanned: …` and still exits
on the enforced ecosystems alone, so a missing tool is visible, never silent.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestParseSPDX` (table) | `MIT` ✓; `(MIT OR Apache-2.0)` ✓; `(MIT AND MPL-2.0)` ✗; `MPL-2.0 OR MIT` ✓; `Apache-2.0 WITH LLVM-exception` ✓; `GPL-2.0 WITH Classpath-exception-2.0` ✗; `MIT*` ✗; `UNKNOWN` ✗; `""` ✗; `(MIT` → error | AND evaluated as OR; a guessed `*` accepted |
| `TestEvaluate_mplFails` | an enforced npm dep `left-pad-mpl@1.0.0` declaring `MPL-2.0` → `fail`; `Evaluate` returns it | the spec's done-when: a gate that passes MPL |
| `TestEvaluate_tldrawDeniedWhateverItDeclares` | `@tldraw/editor@3.0.0` declaring `MIT` → `fail`, *why* names OD-11 | a deny list checked only against the licence |
| `TestEvaluate_exceptionCannotCoverDenied` | an exception for `tldraw` → `Evaluate` returns an error | an exception file that launders the deny list |
| `TestEvaluate_exceptionPinsVersion` | an exception for `x@1.0.0` does not cover `x@1.0.1` | a version-agnostic exception |
| `TestEvaluate_exceptionMustResolveAllowed` | an exception with `resolved: "MPL-2.0"` → error | an exception that simply overrides |
| `TestEvaluate_devDepReportsOnly` | a non-enforced `GPL-3.0` dep (a devDependency no shipped file reaches) → `report`, exit code 0 | P5-P reversed silently |
| `TestEvaluate_denyMatchesExactly` (table) | `tldraw` ✗ and `@tldraw/editor` ✗ whatever they declare; `tldraw-helpers` declaring `MIT` ✓; `@tldrawx/y` declaring `MIT` ✓; Go `github.com/tldraw/tldraw-go` ✗; Go `github.com/tldrawx/y` declaring `MIT` ✓ | a deny list matched by string prefix, or by substring |
| `TestNpmDeps_importedDevDepIsEnforced` (hole 1; fixture tree + injected `Runner`) | `html-to-image` declared only in `devDependencies`, declaring `GPL-3.0`, imported by `src/export/png.ts` → `Enforced`, `Via` names that file, verdict `fail`; its production child is `Enforced` too; `vitest`, imported only by `src/a.test.ts` and `src/test/setup.ts`, stays `report`; an imported name with no `node_modules/<name>` node → `fail` "not installed" | the review's case: a bundled dev import only reported; a closure that stops at the direct package |
| `TestShippedImports` (hole 1; table, injected `Runner` for `imports.cjs`) | `import type` skipped, `import { type A, b }` kept; `@s/n/sub` → `@s/n`, `n/sub` → `n`, `x?raw` → `x`; `./`, `node:`, `virtual:` skipped; `a.test.tsx`, `a.spec.ts` and `src/test/x.ts` not walked; `src/test/helper.ts` **is** walked when a shipped file imports it; CSS `@import "pkg/a.css"` → `pkg`, and one inside `/* */` is not | a walk that trusts file names alone; a test helper that ships unscanned |
| `imports.test.cjs` (`node --test`, real `typescript` from the project) | a fixture holding a comment `// import x from "c"`, JSX text `<p>Can't "d"</p>`, a regex `/"/g` and five real import forms yields exactly the five | a regex scanner substituted for the parser |
| `TestNpmDeps_firstPartyIsStructural` (hole 2) | registry packages named `desktop-notify` and `@panacea-app/x` (`resolved: "https://registry…"`) are checked, and a `GPL-3.0` one fails; a `file:` link inside `-root` is skipped and not descended; a `file:` link outside `-root` declaring `UNLICENSED` fails; the project root is skipped | first party matched by name or prefix |
| `TestGoDeps_firstPartyIsStructural` (hole 2) | `Main: true` skipped; `github.com/kpmg-au-emu/panacea-app/x` with `Main: false` is checked; a directory `replace` inside `-root` skipped, one outside checked; `.Standard` skipped; a module under two packages is one `Dep` | first party matched by module path; first-party code failing its own gate |
| `TestClassify_noLicenceFile` + `TestGoDeps_noLicenceFileFails` (hole 3) | an empty module dir → `("", "none", nil)`; that module's `Dep` fails with why "no licence file in …", exit 1, not 2 | a module with no licence passing, or aborting the run |
| `TestNpmDeps_readsEachCopyAtItsPath` (hole 4) | `node_modules/x` 1.0.0 `MIT` and `node_modules/y/node_modules/x` 2.0.0 `GPL-3.0` are two `Dep`s, each `license` read from its own `path`; the nested one fails; a node printed twice at one path, once without children, keeps its children | a `node_modules/<name>` guess; one copy shadowing another |
| `TestCargoDeps` (I10; fixture `cargo metadata` JSON) | every non-first-party package is a `Dep` with `Enforced` = `CargoEnforced` (false); `MPL-2.0` → `report`, exit 0; `Unlicense/MIT` read as `Unlicense OR MIT` → pass; a `source: null` member inside `-root` skipped; why carries the normal-edge chain, and "(proc-macro)" for a proc-macro-only crate; the runner failing → `notScanned` set, `err` nil | Cargo enforced by accident; a missing cargo aborting the gate, or skipped with no line |
| `TestClassify` (fixtures in `testdata/licenses/`) | the six allowed texts each classify to their id; a GPL-3.0 text → `GPL-3.0-only`; MPL text → `MPL-2.0`; Apache + a CC-BY-4.0 section in one file → `Apache-2.0 AND LicenseRef-unclassified`; `LICENSE-MIT` + `LICENSE-APACHE` → `MIT OR Apache-2.0` | a classifier that returns MIT for any permissive-looking text |
| `TestNpmDeps` (fixture `npm ls --long` JSON + fixture `package.json` files) | `--omit=dev` entries are enforced; dev-only entries no shipped file imports are not; `licenses: [{type:"MIT"},{type:"ISC"}]` → `MIT AND ISC` | dev deps enforced by accident |
| `TestMain_exitCodes` | runs `run()` with fixtures: 0 when clean, 1 with the MPL fixture, 2 when the Go or npm runner errors; 0 with an `MPL-2.0` crate in the Cargo fixture (a `REPORT cargo` line printed); 0 with the cargo runner failing (a `REPORT cargo: not scanned` line printed) | a gate that exits 0 on a tool failure; Cargo flipping the exit code before the owner rules |

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
