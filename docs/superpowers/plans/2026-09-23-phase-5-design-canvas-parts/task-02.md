# Task 02 — The `canvas/` Go module

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Hours:** 1

## Why

`OD-11` rule 4: the canvas is its own package, shippable and testable alone. On the Go side that is
a module of its own in `go.work` (P5-O), with CI building, testing and vetting it beside the two
modules that exist. This task creates the empty module and nothing else, so every later Go task
has a home and no two lanes race to create it.

## Files

- `canvas/go.mod` (new)
- `canvas/doc/doc.go` (new) — the package comment only; task 05 adds the model.
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

1. Create `canvas/go.mod` and `canvas/doc/doc.go` exactly as above.
2. Add `./canvas` to `go.work`'s `use` block, first (the block is alphabetical).
3. Edit the three `azure-pipelines.yml` steps. Change nothing else in the file.
4. Run the acceptance commands; commit the four paths.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go build ./canvas/... ./sandboxd/... ./control-plane/... && echo ok
```
Expected: `ok`.

```bash
go vet ./canvas/... && go list ./canvas/...
```
Expected: `github.com/kpmg-au-emu/panacea-app/canvas/doc`.

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
