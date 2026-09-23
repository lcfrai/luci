# Task 30 — End to end, the phase 5 contract, and the amendments

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** all · **External:** Hub C3 (Hub plan tasks 37–38) for step 1's probe — already a hard dependency of tasks 20–22 and 27; Hub H4 on `compose.yaml` for step 2; the owner's batched hands-on session (`OD-12`) for step 5 · **Hours:** 2 (plus the owner's session)

## Why

Every earlier task proves its piece against fakes. This task proves the path **through the real
pieces** as far as an agent can drive it without the owner — a real sandbox, the real relay, the
real authority, the real Hub on `compose.yaml` — writes the contract other phases will build
against, makes the amendments the parts declared, and lists the owner's hands-on steps, each
**unverified** until the owner has done it (`OD-12`).

## Files

- `docs/superpowers/contracts/2026-09-23-phase-5-design-contract.md` (new)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` — §10.5 and §4.2 amendments
- `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` — the harness and `sandboxd` amendments
- `docs/PROGRAM.md` — phase 5's status line and bundle contents
- `docs/superpowers/planning/phase-5/e2e-report.md` (new)
- `control-plane/internal/designapi/e2e_test.go` (new, `//go:build e2e`)

## The contract document — what it gathers, each from its part

1. **Routes 1–27** with request and response JSON (tasks 20, 21, 22, 29) and the problem codes (task 20).
2. **The window stream** — format, topics, reconnect rule (task 20).
3. **The MCP server** — name, transport and wire table (task 14); all 35 tools with their input
   schemas and result fields (tasks 15–17); the error table (task 15); the instructions (task 17).
4. **The relay** — framing, golden frames, listener address, channel path (task 18); the two CLI flags
   and their placement (task 19).
5. **The document model and ops** as JSON (tasks 05, 06); the event shape (task 11).
6. **The review digest** — envelope and both vectors (task 08); **the bundle** — layout, manifest,
   `approval.json` (tasks 21, 22).
7. **What never changes** — the plan's G1–G13, restated as the contract's invariants.

## Amendments — each made here, once

| Document | Section | Change |
|---|---|---|
| UI contract | §10.5 | *"Requested for Phase 3: GET /v1/events as SSE"* → **exists** (phase 5): one stream per window, `canvas:` and `renderer:` topics; sessions still polled (seam S5-UI.1) |
| UI contract | §4.2 | the `ProblemCode` union's new members (task 27) |
| UI contract | §6.1 / the lens table | the `design` arm renders `DesignLens`; `DesignRenderers` is mounted in `AppShell` (task 28) |
| UI contract | the config files | `vite.config.ts`, `vitest.config.ts`, `tsconfig.json`, `package.json` amendments of task 03, with their reasons |
| Go contract | the harness package | `Request.CanvasMCP`, `Descriptor.SupportsMCPConfig`, the argv order (task 19) |
| Go contract | the sandbox surface | `/mcp-channel`, `-mcp-listen`, `127.0.0.1:8731` (task 18); `backend.EnvEntries` unchanged |
| `PROGRAM.md` | Phase 5 | *"Handoff bundle: tokens, component tree, screens"* → the bundle as built (task 21), and a status line pointing at this plan |

## Steps

1. **Relay end to end, scripted, no owner.** Build the sandbox image (`images/sandbox`), start the
   control plane, create a Claude Code session on a fixture repository's branch (it starts even
   before `/login`: the listener runs in `sandboxd`, not in the CLI). Then, as a test driver outside
   the CLI — `docker exec <sandbox> curl -s -X POST http://127.0.0.1:8731/mcp …` — send
   `initialize`, `tools/list`, `create_artboard`, `write_html` (a settings card), `get_html`, and
   `finish_working`. Assert: 35 tools listed; the artboard exists in the control plane's
   `GET /v1/design/…/document`; its `editedBy.sessionId` is **that session's id**, though no request
   named it. This is `e2e_test.go`, run with `go test -tags e2e ./control-plane/internal/designapi/...`.
   (`docker exec` drives the test; it does not wrap the CLI, which never runs a command here.)

   **Then the human-surface probe from inside the sandbox (review C1).** With `<port>` the control
   plane's port (8730 by default; the implementer's own range when agents share a machine) and
   `<id>` the canvas just created, `e2e_test.go` runs, through `docker exec <sandbox>`, each of:

   ```bash
   curl -s -o /tmp/probe.body -w "%{http_code}" -H "Host: 127.0.0.1:<port>" \
     -X POST http://host.docker.internal:<port>/v1/design/canvases/<id>/approvals -d '{"revision":1}'
   curl -s -o /tmp/probe.body -w "%{http_code}" -H "Host: 127.0.0.1:<port>" \
     -X POST http://host.docker.internal:<port>/v1/design/canvases/<id>/ops -d '{"ops":[],"label":"x","ifRevision":0}'
   curl -s -o /tmp/probe.body -w "%{http_code}" -H "Host: 127.0.0.1:<port>" \
     "http://host.docker.internal:<port>/v1/events?window=wn-000000000000"
   ```

   No `Origin`, a spoofed `Host`, no key — exactly the review's failure scenario. Assert, for each:
   **either** the connection is refused or times out (the host's loopback is not reachable from the
   sandbox), **or** the status is `401` and the body's code is `local_key_required`. Any other
   answer fails the test. Then, from the host **with** the key, assert the canvas's revision did not
   move and route 22 is not `approved`. The report records which of the two it was — **whether the
   host was reachable at all** from this fleet's Docker Desktop is the fact the review left
   unverified. If it was not reachable, the test also sends the same three requests keyless from
   the host itself and asserts each is `401 local_key_required`, so the key is proven either way.
2. **Hub end to end** (when Hub H4 has landed): `docker compose -f control-plane/cmd/hub/compose.yaml up`,
   sign in through `devissuer`, bind the fixture project (phase 4), write a complete brief (phase 4),
   then through the control plane: approve revision *n*; assert the Hub's `GET /v1/refs?key=design:…@n`
   resolves with the snapshot's blobs, and that `POST …/handoffs` writes a bundle whose every file
   matches the manifest. Then the hostile-teammate case (review C2): as a second `devissuer` user
   in the same org, post straight to the Hub API an approval for revision *n+1* whose manifest's
   blobs all match their digests but whose `files` lists `../../x.cmd`; `POST …/handoffs` for it →
   `422 handoff_snapshot_invalid` (`reason: "unsafe_path"`), and nothing new exists in the
   worktree or its parent directory. If Hub H4 has not landed, record it in the report as **not run**.
3. **The gates**: `go run ./tools/licencegate -root .` → 0; `go test ./canvas/... ./control-plane/...
   ./sandboxd/... ./tools/... -race -count=1`; `cd canvas/editor && npx vitest run && npx tsc --noEmit`;
   `cd desktop && npx vitest run && npx tsc --noEmit`; the G7 grep (task 24).
4. Write the contract document and make the amendments.
5. **The owner's hands-on list** — written into `e2e-report.md`, every item **unverified** until
   the owner does it in the batched session (`OD-12`):
   1. The spike page (task 01) inside the Tauri shell: WebView2's result for S1–S10.
   2. `/login` in a sandbox; prompt *"Design a settings page for this app on the canvas."* Watch the
      agent orient (`get_canvas_info`, `get_brief`), write, screenshot, `finish_working`,
      `request_review` — **with no permission prompt per canvas call** (OQ-P5-1).
   3. Two agents on two artboards never collide; a third writing into a claimed artboard is told
      `claimed` and works elsewhere.
   4. Hand-edit the agent's page (move, reorder, restyle, retype); undo your own edit and see the
      agent's work untouched; revert one agent's changes from the activity feed.
   5. Approve (see the Undo toast); edit after approval (*changed after approval* names you); edit
      the brief in Discover (*superseded*); re-review and approve again.
   6. *Start a Develop session from this design*; the Develop agent reads
      `.panacea/design/<canvas>@<rev>/manifest.json` and implements the screen; `git status` shows
      nothing under `.panacea/`.
   7. The visual pass: the Design lens against `prototype/phase-5-design.html` states 1–6 and
      `docs/DESIGN-SYSTEM.md`.
6. Commit by pathspec. **Do not merge** the branch (`OD-1`); the orchestrator pushes when asked.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test -tags e2e ./control-plane/internal/designapi/... -run E2E -count=1 -v 2>&1 | tail -5
```
Expected: `--- PASS: TestE2E_relay` and `ok`.

```bash
go run ./tools/licencegate -root . >/dev/null; echo "gate $?"; grep -c "unverified" docs/superpowers/planning/phase-5/e2e-report.md
```
Expected: `gate 0`; a count of at least `7` (one per owner item).

## Not claimed

- **Nothing the owner has not seen is claimed to work visually** (`CLAUDE.md`, `OD-12`).
- Kata (phase 8), other harnesses (S5-Hx.1), cross-person co-editing (v2) are not exercised.
- Step 2 depends on the Hub's H4 being built; the report says whether it ran.
