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
- `tools/licencegate/main.go`, `gate.go`, `spdx.go`, `classify.go`, `sources.go`, `imports.go` (new).
  *As built:* `sources.go` is three files, `gosource.go` (`Runner`, `GoDeps`), `npmsource.go`
  (`NpmDeps`) and `cargosource.go` (`CargoDeps`), and the printing is `output.go`; `canon` itself
  is `canon.go`
- `tools/licencegate/testdata/npm/firstparty/` (a real npm 11.12.1 run) and
  `testdata/repo-with-mpl/` (the done-when's fixture repository, committed with its
  `desktop/node_modules`, so `.gitignore` gains one `!` line for it)
- `tools/licencegate/canon_windows.go` and `canon_other.go` (new): `canon`'s one platform line,
  `syscall.GetLongPathName` on Windows, nothing elsewhere (re-review RR-M1)
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

// First party is decided by structure, never by name (review I9), and it means
// "a project this gate scans", never "a path inside -root" (re-review RR-I9-2:
// a vendored third-party copy lives inside the repository too). There is no
// name list. A package is first party, and never checked, only when:
//   - Go: its package's .Module.Main is true. Under go.work every workspace
//     module is Main (verified 2026-09-24: control-plane and sandboxd both
//     report "Main": true), so every first-party Go module is a go.work
//     member. A directory replace, inside -root or outside it, is classified
//     from its directory like any module (neither go.mod has a replace today,
//     read 2026-09-24);
//   - npm: it is the root node of a project in the scan list (the command
//     line's "scans"), or a link whose target is exactly the root of such a
//     project. A link is read from what npm states about structure, never
//     from npm ls's "resolved" (re-review RR-I9-1): the project's
//     package-lock.json entry for the node's location has "link": true, and
//     its "resolved" is the target relative to the project directory
//     (verified on npm 11.12.1: "node_modules/@panacea-app/b": {"resolved":
//     "../b", "link": true} and "node_modules/c": {"resolved": "sub/c",
//     "link": true}). npm ls --long's own "resolved" is relative to a
//     different base per kind ("file:../../../b" from the link's parent for a
//     link; for a tarball an absolute "file:C:\…\vendor\g-1.0.0.tgz", measured
//     by task 04's run, not the "file:vendor/g-1.0.0.tgz" first written
//     here), so the gate never reads it. A file: tarball has no "link" and installs as a real
//     copy: it is third party and checked. A link to a directory inside -root
//     that is not a scanned project fails with "first-party link <path> is not
//     a scanned project: add it to the scan list, or install a vendored copy
//     as a file: tarball, which is checked like a registry package". A link
//     outside -root is third party: classified from its target's package.json,
//     and descended;
//   - Cargo: its id is in "workspace_members". A path crate that is not a
//     member is reported like any crate.
// Paths are compared after canon (re-review RR-M1: npm prints "path" in the
// long form even under an 8.3 working directory, and -root may arrive short,
// as in the program's own $TEMP mutation copies): filepath.Abs, then, on
// Windows only (canon_windows.go), syscall.GetLongPathName, then
// filepath.Clean. GetLongPathName expands short elements and keeps junctions
// as they are; verified 2026-09-24 with go1.26.2 on Windows: C:\Users\AU-DEV~1\
// AppData\Local\Temp became C:\Users\au-dev-dmackie1\AppData\Local\Temp, and
// a path through a junction j kept "\j\". canon is never filepath.EvalSymlinks:
// a worktree's desktop/node_modules is a junction into the main tree (the
// program's rule 5), so resolving links would move every node's location
// out of its project, and on the same probe EvalSymlinks of a path through
// the junction returned an error, "The system cannot find the path
// specified". (Carried b05-2 RR2-1: the gate does not read a junctioned
// node_modules at all. There, npm ls exits 1 ELSPROBLEMS once a file:
// dependency is installed, and Node and Vite resolve the main tree's
// canvas/editor, so a gate that took npm's JSON anyway would clear source
// that is not what ships. NpmDeps Lstats <dir>/node_modules first; a
// ModeSymlink or ModeIrregular (a junction, go1.26.2: measured) is exit 2:
// "<dir>/node_modules is a link into <target>: the gate reads this project's
// own installed tree; remove the link (never rm -rf through it) and run npm
// ci in <dir>". canon still keeps junctions: a real install holds one, the
// file:../canvas/editor link node.) On Windows, samePath and insideRoot compare with
// strings.EqualFold (filepath.Rel already does, per element). "Inside -root"
// is filepath.Rel of the two canon forms with no leading "..". How go list
// prints Module.Dir under a short working directory is unverified; canon
// makes it not matter. A registry package named "desktop-notify",
// "@panacea-app/x" or "github.com/kpmg-au-emu/panacea-app/x" (a published copy
// with Main false) is third party.
// canon calls os.Stat on every platform before the platform line (carried
// b05-2 RR2-M1: GetLongPathName alone made this Windows-only, and on ubuntu
// CI a broken in-root link failed as "not a scanned project"), so canon of a
// missing path (a link whose target is gone) is an error, and the run exits
// 2, never a skip, on every platform. TestCanon has the row. The
// module stays standard-library only: syscall is the standard library.
// Each source in sources.go applies its line of this rule through these:
func canon(p string) (string, error)
func samePath(a, b string) bool   // canon forms equal
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
// returns an SPDX expression. The names match case-insensitively, over the
// entries os.ReadDir returns (strings.EqualFold), never by opening a fixed
// name (re-review RR-I9-3): github.com/josharian/intern v1.0.0, in the shipped
// closure, holds only "license.md". Opening "LICENSE.md" finds it on Windows,
// whose filesystem ignores case, and misses it on the ubuntu-latest CI agent,
// so an exact-name gate passes locally and fails the first push. source
// carries the entry's real name, "license.md", which is how a test on Windows
// tells a directory read from an open of a fixed name. Rules:
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
// de-duplicated by path@version, and classifies .Module.Replace.Dir when
// .Module.Replace is set (a directory replace included, wherever it points:
// re-review RR-I9-2), else .Module.Dir. All Enforced. A module whose
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
// other node is reported. Links are read from <dir>/package-lock.json, keyed
// by each node's location, filepath.ToSlash(filepath.Rel(canon(dir),
// canon(path))). No lockfile is exit 2. A node with no entry is not a link,
// so it is checked: only a positive "link": true can exempt anything. The project
// root, and a link whose target is a scanned project's root (the scanned
// argument: the command line's list), are skipped and not descended, because
// that project is scanned as its own From; an imported name that resolves to
// one is skipped the same way. Every other link follows gate.go's rule: in
// -root, it fails "not a scanned project"; outside, it is classified.
func NpmDeps(root, dir, from string, scanned []string, run Runner) ([]Dep, error)

// CargoDeps reports one Cargo workspace (review I10):
//   cargo metadata --offline --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc
// run in desktop/src-tauri. --locked refuses to rewrite Cargo.lock and
// --offline refuses the network; the filter keeps the Windows target, the one
// the owner's machines run. Each package in "packages" whose id is not in
// "workspace_members" (today one member, desktop@0.1.0: read 2026-09-24)
// becomes a Dep with Declared = its "license" (or "" with Source
// "license_file: <path>" when only license_file is set), Enforced =
// CargoEnforced. Cargo's deprecated "/" separator is read as OR, with or
// without spaces around it, here only, because Cargo documents it so;
// ParseSPDX is unchanged. The tree holds both forms today (read 2026-09-24):
// "Unlicense/MIT" (2 crates), "MIT/Apache-2.0" (11), "BSD-3-Clause/MIT"
// (brotli-decompressor 5.0.3) and "Apache-2.0 / MIT" (fnv 1.0.7; re-review
// RR-M2), so a split that misses the spaced form reports fnv falsely today and
// fails it once CargoEnforced flips.
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
// The walk starts at every .ts/.tsx/.mts/.cts/.js/.jsx/.mjs/.cjs/.css file
// (Vite bundles all eight script forms; re-review RR-M4) under srcDir that
// is not a test file (the name matches *.test.* or *.spec.*, or the file lies
// under a directory named "test"), and follows relative imports from a walked
// file to a fixed point, so a test helper that shipped code imports is walked
// too. Specifiers come from imports.cjs for script files and from CSS
// `@import "x"` / `@import url("x")` (after /* */ comments are removed) for
// .css. It skips `import type` / `export type` statements (erased before
// bundling; `import { type A, b }` is kept, because b ships) and specifiers
// starting ".", "/", "node:", "virtual:" or "\x00". "@s/n/sub" names "@s/n",
// "n/sub" names "n", and a "?query" suffix is dropped.
// As built (task 04): a Node builtin ("fs", marked by imports.cjs through
// module.isBuiltin) names no package unless one of that name is installed
// under <dir>/node_modules ("events"), which is how Vite resolves it; a "#x"
// subpath import is kept as a name, so it fails "not installed" (fail closed:
// package.json "imports" can map it to any package; neither project uses one).
// imports.cjs is embedded (go:embed) and written to a temporary file, and
// files go 40 per node call (the Windows command-line cap).
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
        This list is also the npm first-party list (gate.go): a link is exempt only when its
        target is one of these project roots. A new first-party package is added here, in one
        line, or the gate fails its link as "not a scanned project".
exit 0  every enforced dep is pass or excepted
exit 1  at least one enforced dep failed (each printed: FAIL <eco> <name> <version> <declared> — <why>)
exit 2  the gate could not run (a command failed, a file is unreadable, a node_modules is a
        link into another tree (carried RR2-1), an exception is refused, no scanned project
        exists under -root)
every non-enforced dep that would have failed prints REPORT <eco> <name> <version> <declared> — <why>,
and never changes the exit code; the Cargo lines print whether or not cargo ran (CargoDeps)
As built: every enforced finding prints "<VERDICT> <eco> <name> <version> <declared> — <why>
(<via>, from <from>)", "(none)" for no licence and "-" for no version; a Cargo line always
prints ("cargo: N crates scanned, reported only …" or "REPORT cargo: not scanned: …"), then a
summary. A scanned project absent from -root prints "SKIP go: …" or "SKIP npm <dir>: …" and
changes nothing (the done-when's fixture holds only a desktop); none present is exit 2, so a
mistyped -root never passes vacuously. -exceptions defaults to
<root>/tools/licencegate/exceptions.json, none when absent; a named file must exist; unknown
fields are refused. An npm alias of a denied package ("my-canvas": "npm:tldraw@…") fails as
denied under its installed name.
```

```yaml
# azure-pipelines.yml — appended after "Canvas editor"
  - script: cd desktop && npm ci
    displayName: Desktop dependencies
  - script: |
      if command -v cargo >/dev/null; then cargo fetch --locked --manifest-path desktop/src-tauri/Cargo.toml; else echo "cargo not on PATH: the licence gate prints REPORT cargo: not scanned"; fi
    displayName: Cargo registry cache (licence report only)
    continueOnError: true
  - script: node --test tools/licencegate/imports.test.cjs
    displayName: Licence gate import parser
  - script: go run ./tools/licencegate -root .
    displayName: Licence gate
```

*As built:* the `node --test` step is added (task 04), so the parser test that step 2 runs
locally also runs in CI, after `desktop`'s `npm ci` has installed `typescript`.

The gate needs `node` (for `npm ls` and `imports.cjs`, already on the agent for the desktop steps)
and, for the Cargo report only, `cargo` **and a populated registry cache**, because `--offline`
reads only the cache (re-review RR-M3). A fresh hosted `ubuntu-latest` agent starts with an empty
cache, so without the `cargo fetch --locked` step every CI run would print `REPORT cargo: not
scanned` whether or not `cargo` is installed. The fetch step fills it; `continueOnError` keeps a
report-only step from failing the build, and the gate's own line still says when the report did
not run. Whether the hosted agent has `cargo`, and whether its fetch gets through, are
**unverified**; either way the run prints `REPORT cargo: not scanned: …` and still exits on the
enforced ecosystems alone, so a missing report is visible, never silent.

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
| `TestShippedImports` (hole 1; table, injected `Runner` for `imports.cjs`) | `import type` skipped, `import { type A, b }` kept; `@s/n/sub` → `@s/n`, `n/sub` → `n`, `x?raw` → `x`; `./`, `node:`, `virtual:` skipped; `a.test.tsx`, `a.spec.ts` and `src/test/x.ts` not walked; `src/test/helper.ts` **is** walked when a shipped file imports it; `src/latest/x.ts` and `src/contest.tsx` **are** walked (a directory named exactly `test`, not a substring); a bare import in each of `a.mts`, `a.cts` and `a.cjs` is found (RR-M4); CSS `@import "pkg/a.css"` → `pkg`, and one inside `/* */` is not; *as built:* a builtin `fs` dropped, `events` kept when `node_modules/events` exists, `#internal` kept as a name | a walk that trusts file names alone; a test helper that ships unscanned |
| `imports.test.cjs` (`node --test`, real `typescript` from the project) | a fixture holding a comment `// import x from "c"`, JSX text `<p>Can't "d"</p>`, a regex `/"/g` and five real import forms yields exactly the five | a regex scanner substituted for the parser |
| `TestNpmDeps_firstPartyIsStructural` (hole 2; RR-I9-1, RR-I9-2) | every fixture's `npm ls --long` JSON and `package-lock.json` is copied from a real npm run, and the test's comment names the npm version and the command, because a fixture composed from the rule's own words agrees with a wrong rule. Registry packages named `desktop-notify` and `@panacea-app/x` (`resolved: "https://registry…"`) are checked, and a `GPL-3.0` one fails; a link (lockfile `"link": true`) whose target is a scanned project's root is skipped and not descended; a link into `<root>/vendor/x`, not a scanned project, declaring `GPL-3.0`, fails "not a scanned project"; a link into `<root>-fork/x` declaring `UNLICENSED` fails (npm ls prints `resolved: "file:../../../<root>-fork/x"` for it, which joined to the node's `path` lands inside `-root`); a `file:vendor/g-1.0.0.tgz` tarball declaring `MIT`, no `link` in the lockfile, is checked and **passes** (carried b05-2 RR2-M2: declaring `GPL-3.0` it failed under the mutant "any `file:` is a link" too, as "not a scanned project", so the row could not tell them apart); a `GPL-3.0` devDependency tarball no shipped file imports is `report`; a node with no lockfile entry is checked; the project root is skipped | first party matched by name or prefix; `insideRoot` as a string-prefix test; the link target joined to `path` (or to `filepath.Dir(path)`) from npm ls's `resolved`; any `file:` read as a link; "inside -root" read as "ours" |
| `TestCanon` (RR-M1) | `samePath` of a path and its case-flipped form is true on Windows, false elsewhere; `insideRoot("<r>", "<r>-fork/x")` false; on Windows, when `os.TempDir()` holds a `~` (it does on the owner's machine: `C:\Users\AU-DEV~1\…`), `canon` of it has no `~` and `insideRoot(short, long form of a child)` is true; the case is skipped, and says so, where TEMP has no short element; on Windows, a junction `j` made under `t.TempDir()` (`mklink /J`, skipped with a message if it cannot be made) → `canon` of `j/f` still ends `\j\f`; on **every** platform, `canon` of a missing path returns an error and `insideRoot` of a missing child is false (carried RR2-M1) | a root given in 8.3 form putting every in-root link outside the root; `canon` as `filepath.EvalSymlinks`, which moves a worktree's junctioned `node_modules` into the main tree |
| `TestGoDeps_firstPartyIsStructural` (hole 2; RR-I9-2) | `Main: true` skipped; `github.com/kpmg-au-emu/panacea-app/x` with `Main: false` is checked; a directory `replace` into `<root>/third_party/gpl-lib` holding a `GPL-3.0` text is classified from `Replace.Dir` and fails, and one outside `-root` is classified too; `.Standard` skipped; a module under two packages is one `Dep` | first party matched by module path; a vendored fork inside the repository exempted as first party; first-party code failing its own gate |
| `TestClassify_noLicenceFile` + `TestGoDeps_noLicenceFileFails` (hole 3) | an empty module dir → `("", "none", nil)`; that module's `Dep` fails with why "no licence file in …", exit 1, not 2 | a module with no licence passing, or aborting the run |
| `TestNpmDeps_readsEachCopyAtItsPath` (hole 4) | `node_modules/x` 1.0.0 `MIT` and `node_modules/y/node_modules/x` 2.0.0 `GPL-3.0` are two `Dep`s, each `license` read from its own `path`; the nested one fails; a node printed twice at one path, once without children, keeps its children | a `node_modules/<name>` guess; one copy shadowing another |
| `TestCargoDeps` (I10; fixture `cargo metadata` JSON) | every non-first-party package is a `Dep` with `Enforced` = `CargoEnforced` (false); `MPL-2.0` → `report`, exit 0; `Unlicense/MIT` read as `Unlicense OR MIT` → pass, and so do `Apache-2.0 / MIT` and `BSD-3-Clause/MIT` (RR-M2); the one id in `workspace_members` skipped, and a path crate (`source: null`) inside `-root` that is not a member is a `Dep`; why carries the normal-edge chain, and "(proc-macro)" for a proc-macro-only crate; the runner failing → `notScanned` set, `err` nil | Cargo enforced by accident; a missing cargo aborting the gate, or skipped with no line |
| `TestClassify` (fixtures in `testdata/licenses/`) | the six allowed texts each classify to their id; a GPL-3.0 text → `GPL-3.0-only`; MPL text → `MPL-2.0`; Apache + a CC-BY-4.0 section in one file → `Apache-2.0 AND LicenseRef-unclassified`; `LICENSE-MIT` + `LICENSE-APACHE` → `MIT OR Apache-2.0`; a directory holding only `license.md` (the MIT text, as in `josharian/intern` v1.0.0) → `MIT` with source `license.md`, the entry's real name (RR-I9-3) | a classifier that returns MIT for any permissive-looking text; names matched exactly, by `==` over the entries or by opening `LICENSE.md` (on Windows the open finds the file, but source then reads `LICENSE.md`, so the assertion on source kills it there too) |
| `TestNpmDeps` (fixture `npm ls --long` JSON + fixture `package.json` files) | `--omit=dev` entries are enforced; dev-only entries no shipped file imports are not; `licenses: [{type:"MIT"},{type:"ISC"}]` → `MIT AND ISC` | dev deps enforced by accident |
| `TestMain_exitCodes` | runs `run()` with fixtures: 0 when clean, 1 with the MPL fixture, 2 when the Go or npm runner errors; 0 with an `MPL-2.0` crate in the Cargo fixture (a `REPORT cargo` line printed); 0 with the cargo runner failing (a `REPORT cargo: not scanned` line printed); 2 when `desktop/node_modules` is a junction (Windows, `mklink /J`) or a symlink (elsewhere) into another tree, stderr naming "node_modules is a link into" (carried RR2-1; skipped with a message where no link can be made); 2 for an exception naming `tldraw`; 2 with no scanned project under `-root`; every run that is not 2 prints a Cargo line | a gate that exits 0 on a tool failure; Cargo flipping the exit code before the owner rules; a gate that reads a junctioned worktree's main-tree packages |
| `TestNpmDeps_linkedNodeModules` (carried RR2-1; added by task 04) | `isLinkMode` true for `ModeSymlink` and `ModeIrregular`, false for a directory; a real junction (Windows) or symlink (elsewhere) as `<dir>/node_modules` → error naming the link, "never rm -rf through it" and "run npm ci in", and npm is never run. Logs the `Lstat` mode: `?rw-rw-rw-` (`ModeIrregular`) on go1.26.2, Windows | a check of `ModeSymlink` alone, which a junction does not carry |
| `TestMain_repoWithMPL` (added by task 04) | the real runner on `testdata/repo-with-mpl`: exit 1 and `FAIL npm mpl-fixture 1.0.0 MPL-2.0 — …`; skipped where `npm` is not on PATH | the spec's done-when, as a test and not only an acceptance command |

## Steps

1. Write the tests and fixtures (`testdata/licenses/*.txt` are the licences' canonical texts; the
   GPL, MPL and CC texts are fixtures, never code, and carry nothing copied from any canvas).
2. Implement `spdx.go`, `classify.go`, `gate.go`, `sources.go`, `imports.go`, `imports.cjs`,
   `main.go` until green; then `node --test tools/licencegate/imports.test.cjs` from the repository
   root, with `desktop/node_modules` present, until green. `node --test` needs only
   `typescript`, so a junction serves it; the gate run in step 3 needs a **real** install in
   each scanned project, because a junctioned `node_modules` is refused (exit 2, carried RR2-1).
3. Run it on the real repository. For each `fail` on an **existing** dependency (the control
   plane's Kubernetes and Docker trees predate `OD-11`), read the licence file, and either add a
   reviewed exception or record the dependency in the report below as a real violation for the
   orchestrator. Do not add an exception without reading the file. An exception cannot cover an
   `MPL-2.0` crate (its `resolved` must be allowed, `TestEvaluate_exceptionMustResolveAllowed`), so
   the Cargo `REPORT` lines are recorded, not excepted: they are the owner's (OQ-P5-2).
4. Write `docs/superpowers/planning/phase-5/licence-gate-first-run.md`: the gate's output on the
   repository at this commit, every exception added and why, every real violation found, and a
   **Cargo** section with every `REPORT cargo` line. That section names, at minimum, the five
   `MPL-2.0` crates this plan already knows of, each with its chain as the gate prints it — the
   facts below were read on 2026-09-24 with `cargo metadata --offline --locked --format-version 1
   --filter-platform x86_64-pc-windows-msvc` in `desktop/src-tauri` (263 packages) and each crate's
   `license` in the registry's `Cargo.toml`; re-read them, and report any difference:

   | Crate | `Cargo.lock` | Reached (normal edges) | Links into the binary? |
   |---|---|---|---|
   | `cssparser 0.36.0` | `:587` | `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0` | a `lib`: *unverified* |
   | `cssparser-macros 0.6.1` | `:600` | `… → dom_query 0.27.0 → cssparser 0.36.0` | a proc-macro: runs at compile time, does not link |
   | `selectors 0.36.1` | `:2923` | `tauri → tauri-utils → dom_query 0.27.0` | a `lib` (with a build script): *unverified* |
   | `dtoa-short 0.3.5` | `:850` | `… → dom_query → cssparser 0.36.0` | a `lib`: *unverified* |
   | `option-ext 0.2.0` | `:2382` | `tauri 2.11.5 → dirs 6.0.0 → dirs-sys 0.5.0` | a `lib`: *unverified* |

   The same read found 18 crates under `Unicode-3.0` alone (`icu_*`, `zerovec`, `yoke`, `tinystr`,
   `litemap`, …), `unicode-ident 1.0.25` as `(MIT OR Apache-2.0) AND Unicode-3.0`, `foldhash 0.2.0`
   and `zlib-rs 0.6.8` as `Zlib`, and `same-file 1.0.6` and `walkdir 2.5.0` as `Unlicense/MIT` (the
   last two pass once `/` reads as OR). `Unicode-3.0` and `Zlib` are permissive but are not among
   `OD-11`'s four families, so they report too. The review counted four `MPL-2.0` crates; the read
   found a fifth, `cssparser-macros`.
5. Edit `go.work` and `azure-pipelines.yml`; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./tools/licencegate/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/tools/licencegate`.

```bash
go run ./tools/licencegate -root . ; echo "exit $?"
```
Expected: a table of every enforced dependency, the `REPORT` lines (among them `REPORT cargo
cssparser 0.36.0 MPL-2.0 — …` and the other crates of step 4's table, or one `REPORT cargo: not
scanned: …` line), then `exit 0` — or `exit 1` with each real violation named in the first-run
report. The Cargo lines never change the exit code while `CargoEnforced` is false.

```bash
go run ./tools/licencegate -root ./tools/licencegate/testdata/repo-with-mpl ; echo "exit $?"
```
Expected: `FAIL npm mpl-fixture 1.0.0 MPL-2.0 — …` and `exit 1` — the spec's done-when.

## Not claimed

- **Cargo is reported, not enforced** (review I10). The canvas adds no Rust crate, but the canvas
  ships inside the Tauri binary, and that crate tree already holds five `MPL-2.0` crates and
  permissive licences outside `OD-11`'s four families (step 4's table). Whether any of them may
  ship is the owner's ruling, OQ-P5-2, which now states them; `CargoEnforced` is the one-line seam.
  Which `lib` crates actually link into the release binary is *unverified*: the chain comes from
  `cargo metadata`, which unifies features across build and normal use.
- **Build plugins are not scanned.** A Vite plugin can inject runtime code into the bundle. By its
  documented behaviour (not read in this tree), `@vitejs/plugin-react` injects `react/jsx-runtime`,
  from `react`, a production dependency, and the refresh runtime in dev only. A plugin added later
  that injects a package no shipped file imports and no production list names would pass
  unenforced; the reviewer of that plugin's task owns it.
- **Fonts are not scanned** (plan OQ-P5-5).
- The classifier is a fingerprint match, not a legal reading. An exception is a person's review;
  this is not legal advice.
- The first-run report may name existing violations; fixing them is not this task's.
