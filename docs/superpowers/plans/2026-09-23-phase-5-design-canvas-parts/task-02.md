# Task 02 — The `canvas/` Go module

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Hours:** 1

## Why

`OD-11` rule 4: the canvas is its own package, shippable and testable alone. On the Go side that is
a module of its own in `go.work` (P5-O), with CI building, testing and vetting it beside the two
modules that exist. This task creates the empty module and the one file that keeps the npm package
out of it (below), so every later Go task has a home and no two lanes race to create it.

## Files

- `canvas/go.mod` (new)
- `canvas/doc/doc.go` (new) — the package comment only; task 05 adds the model.
- `canvas/editor/go.mod` (new) — a module boundary, not Go code (review M6). Task 03 fills the
  rest of `canvas/editor/` and never edits this file.
- `go.work` — one line.
- `azure-pipelines.yml` — three patterns.

## Contracts

```
// canvas/go.mod
module github.com/kpmg-au-emu/panacea-app/canvas

go 1.23
```

No `require` block: nothing imports a third-party package yet, and `go mod tidy` would delete one.
Task 07 adds `golang.org/x/net v0.26.0` (G1) when it first imports `golang.org/x/net/html`.

```
// go.work — after
go 1.23

use (
	./canvas
	./control-plane
	./sandboxd
)
```

```go
// canvas/doc/doc.go
// Package doc is the canvas document model: canvases, pages, nodes, tokens,
// assets, comment threads, and the op protocol that is the only way any of
// them changes. It imports nothing outside the standard library and nothing
// from the control plane or the desktop (OD-11 rule 4). See
// docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md.
package doc
```

```
// canvas/editor/go.mod
// Not Go code: a module boundary so ./canvas/... stops at the npm package (plan review M6).
module github.com/kpmg-au-emu/panacea-app/canvas/editor
```

**Why a second `go.mod` (review M6).** The npm package `canvas/editor` (task 03) sits inside this
module's directory, and a `./canvas/...` pattern walks every directory under `canvas/`, including
`canvas/editor/node_modules`: thousands of npm directories for every `go build`, `go test`,
`go vet`, the licence gate's `go list -deps -json ./canvas/...` (task 04) and `TestBoundary`'s
`go list` (task 05), and any `.go` file an npm package ships becomes a package of this module. The
`ignore` directive that would exclude a directory needs Go 1.25; this repository stays on `go 1.23`.
A directory holding a `go.mod` is another module, and the go command's pattern walk skips it, in
workspace mode and with `GOWORK=off` alike. `go.work` does **not** `use` it (it holds no Go code),
so no build ever loads it. Verified 2026-09-24 on go1.26.2 against a `go 1.23` module and a
`go.work`: with a stray `node_modules/stray/stray.go` of broken syntax under `editor/`,
`go list ./canvas/...` named `…/canvas/editor/node_modules/stray` and `go vet ./canvas/...` exited
1; with this file added, `go list` named only `…/canvas/doc`, and `go vet`, `go build`,
`go work sync` and `GOWORK=off go list ./...` in `canvas/` exited 0. Unverified: a go1.23
toolchain itself (none is installed here; the nested-module skip predates Go 1.23). Never run
`go work use -r .`: it would add this boundary to the workspace.

The alternative the review offered, moving the Go module to `canvas/core`, rewrites the canvas
paths in 15 part files (87 mentions, counted 2026-09-24) and the plan's file map for the same effect; this file is one line and is pinned by task 05's
`TestBoundary` (the file exists and names that module).

```yaml
# azure-pipelines.yml — the three steps, after
  - script: go build ./canvas/... ./sandboxd/... ./control-plane/...
    displayName: Build
  - script: go test ./canvas/... ./sandboxd/... ./control-plane/... -race -count=1
    displayName: Test
  - script: go vet ./canvas/... ./sandboxd/... ./control-plane/...
    displayName: Vet
```

**The module boundary, as a rule every later canvas task keeps:** no package under `canvas/`
imports `github.com/kpmg-au-emu/panacea-app/control-plane/...` or `.../sandboxd/...`. Task 05 adds
the test that enforces it (`canvas/boundary_test.go`), because this task has no package that could
break it.

## Tests first

None: an empty package has no behaviour. The acceptance commands are the test.

## Steps

1. Create `canvas/go.mod`, `canvas/doc/doc.go` and `canvas/editor/go.mod` exactly as above.
2. Add `./canvas` to `go.work`'s `use` block, first (the block is alphabetical). Not
   `./canvas/editor`.
3. Edit the three `azure-pipelines.yml` steps. Change nothing else in the file.
4. Run the acceptance commands; commit the five paths.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go build ./canvas/... ./sandboxd/... ./control-plane/... && echo ok
```
Expected: `ok`.

```bash
go vet ./canvas/... && go list ./canvas/...
```
Expected: `github.com/kpmg-au-emu/panacea-app/canvas/doc`, and no path under `…/canvas/editor`.

```bash
go test ./canvas/... -count=1
```
Expected: `?   	github.com/kpmg-au-emu/panacea-app/canvas/doc	[no test files]`.

```bash
grep -c "canvas" azure-pipelines.yml
```
Expected: `3`.

## Not claimed

- The sandbox image is unaffected and is not rebuilt: it copies only `sandboxd/` and builds with
  `GOFLAGS=-mod=vendor` and no workspace (`images/sandbox/Dockerfile:7-8`).
- The control plane does not import the canvas yet; task 19 adds its `require` and `replace`.
- CI is not run by this task; the pipeline edit is verified by the next CI run on a pushed branch,
  which is the orchestrator's (rule: never push).
