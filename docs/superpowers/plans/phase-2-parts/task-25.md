### Task 25: Prove the phase end to end on Docker, by hand, in a real window

Task 24 ends with the cockpit reading from a live control plane and a claim it is careful to limit: *"no screenshot was taken, no visual acceptance was performed"* (`task-24.md:1335`). Its step 11 is the only moment in Group D where a real window opens, and it checks three things — the rail lists a machine and projects, a project expands into branches, a branch fills the sub-rail (`task-24.md:1276`) — and says in bold that it is *"a functional check, not a visual one"* (`:1278`). **That is the whole of what task 24 observes in a real window.** Everything else Group D argued, it argued against jsdom, which *"hands back zero rects, never lays anything out"*, and against terminals that are *"the implementer's own model of xterm"* (`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-d-real-browser-carries.md:4-6`).

**This task is where those arguments become observations or become named gaps.** The master plan gives it one line — *"End-to-end proof on Docker by hand"* (`2026-09-18-phase-2-backend-and-cockpit.md:38`) — and the phase has spent six weeks of ledger filling in what that line has to cover. It inherits four things, and every one must end this task as a step or as a deferred item with an owner:

1. **Nine of the ten real-browser carries.** `group-d-real-browser-carries.md:13` says *"Task 24 is the only task with a real Chromium window. That is why it owns the observation of almost everything below"* — and then task 24's plan does not observe them. `grep -nE "scrollback|Retry attach|themeFromTokens|Playwright" task-24.md` finds the word *scrollback* once, as the title of defect 5 in the manifest (`:1019`), and nothing else. Carry 8 — *"there is no terminal visible in this application at all until task 24 mounts the cockpit"* (`group-d-real-browser-carries.md:89`) — is the one task 24 closes, by mounting. Carries 1 to 7, 9 and 10 land here. Step 19's table records the disposition of every one.
2. **Twelve open carries.** `group-d-open-carries.md` gives each an owner. Five name task 24 or 25 (5, 6, 7, 8 and — through its successor, real-browser carry 10 — 1). Carry 2, *"`resume` has no end-to-end proof"* (`:43`), names no task at all and is picked up here because this is the only task that has a real sandbox to resume in. The rest are dispositioned in step 19 without work.
3. **Two prerequisites, both verified this phase.** The sandbox image predates the behaviour it must demonstrate — built `2026-09-21T05:10:18Z`, while `382f056` (*"fail closed without a token…"*) landed `2026-09-22T08:43:47+10:00`, which is `2026-09-21T22:43:47Z` (`progress.md:5150-5155`). And the harness catalogue's evidence cites a CLI version that never runs: `harness.go:123` and `argv.go:6` say 2.1.267 *"on this machine"*, while `images/sandbox/Dockerfile:17` pins 2.1.274 (`group-d-open-carries.md:66-71`).
4. **Phase 1's proof, re-proven.** The UI contract says Phase 1's 9/9 *"is re-proven by hand in Group E"* (`2026-09-18-phase-2-ui-contract.md:276`), and Phase 1's own results end with five things it could not prove, the fifth being *"no human or agent has seen the native window render a terminal"* (`docs/verification/2026-09-17-phase-1-results.md:68`). Task 5 adds two more — Docker questions Q3 and Q4, deferred here if the image was absent (`task-05.md:1457`).

**The `max` effort is not changed by this task, because it is a product decision, not an omission.** Open carry 5 and the ruling behind it (`progress.md:5175-5182`) call it *"an omission, not a fault"*; the code says otherwise, in writing. `harness.go:37-40` reads *"The CLI also accepts "max"; the product does not offer it, so Build rejects it rather than passing through a level no picker can show"*, citing `docs/FUNCTIONAL-SPEC.md:27` (*"low / medium / high / xhigh (Claude Code only)"*). Two Go tests pin the absence on purpose — `TestClaudeCodeOffersExactlyTheFourProductEfforts` (`harness_test.go:150`) and `TestBuildRejectsAnEffortTheProductDoesNotOffer` (`argv_test.go:194`). Offering `max` would amend the functional spec, and that is the product owner's decision, not a plan's. **So task 25 records it as an open question for the owner** in the results document, and changes nothing about it. If the owner later says yes, one fact from this plan's drafting saves that change a defect: a Go-only edit is not enough, because `SessionPicker.tsx:27` is `const EFFORT_LABEL: Record<Effort, string>` over `types.ts:100`'s `Effort`, and `:112-114` renders `{EFFORT_LABEL[effort]}` for every effort the server sends — a fifth level added to the catalogue alone renders a picker row labelled `undefined`.

**What step 5 does change is evidence hygiene.** Both comments that cite the CLI's `--help` say it was read *"on this machine, version 2.1.267"*; the binary that runs is 2.1.274. `--permission-mode` was checked against 2.1.274 and is identical (`progress.md:5162-5166`), so the mapping stands and only the citation moves. Two comments, no behaviour.

#### Amended 2026-09-25: three owner decisions, and three fixes this task now runs against

**Owner decisions** (`docs/superpowers/planning/2026-09-23-owner-decisions.md`):

- **OD-12 batches every owner-observed step to the very end.** The owner does steps 16 to 18 in one hands-on session after phases 3 to 7 are built, not when this task runs. So this task **writes** the nineteen owner items and makes each reachable, and the results document carries every one as `PENDING OWNER`, **unverified**, until that session. No step waits for the owner, and no report may call an owner item done.
- **OD-19 makes org inference the cloud path**: a LiteLLM-style gateway, a scoped per-session token, no personal subscription. It retires "`/login` in every sandbox" **in the cloud**. **OD-20 keeps local mode**, which this phase is, so here `/login` is still the user's own, in every sandbox (step 18's note). Nothing in this task builds or assumes the gateway.

**Fixes on the branch this task proves** (reconcile record: `.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/task-25-26-reconcile.md`, items 1 to 12):

- **The Resume fix** (`b4be570`, round 1 `2e3db79`). It persists `/home/dev/.claude/projects` and nothing wider, resumes with `--resume <U> --fork-session --session-id <V>`, and refuses (409) to resume a session whose conversation has no transcript. `o2.resume` (step 14) seeds a real transcript and asserts the fork.
- **The sandbox git + CA fix** (`2d6d95b`…`4f6d698`, then its rounds through `31576b7`: line endings, no auto gc, the host's global excludes and attributes files, and X-1's host-owned-config-only resolvers). Git works on a primary worktree, as a configured identity (`PANACEA_APP_GIT_NAME` and `PANACEA_APP_GIT_EMAIL`, or the host's own `git var GIT_AUTHOR_IDENT`), and `PANACEA_APP_CA_BUNDLE` lets Claude Code through a TLS-intercepting proxy. Steps 1 and 6 set and check it; the new check `g.primary` (step 14) proves git.
- **The reaper-scope fix** (`a9c2cec`, `2cce576`). A control plane from HEAD reaps only its own instance's sandboxes. The Docker lock (`docker-daemon-lock.md`) still binds step 6, because an older build on the daemon still reaps everything.

The check count this changes is **nineteen** (it was eighteen); every restatement below says so.

#### Who can observe what, and why every step carries a label

`CLAUDE.md` binds this task more tightly than any other in the phase: *"A screenshot is not evidence that something works"*, and *"Corporate network blocks cube.computer and headless browser screenshots. Visual acceptance is the owner's eyes, not a self-report."* So every step below is labelled with exactly one of two kinds.

- **AGENT-PROVABLE.** Behaviour an implementer asserts and a script can re-run: the DOM state of a real Chromium window laid out by a real engine, the WebSocket frames it sends, what `stty size` reports from inside the PTY, what `docker inspect` says a container was started with, what the control plane logs. **Behaviour, not appearance.** A bounding rectangle inside the viewport is a number and is agent-provable; whether the thing in that rectangle looks right is not.
- **OWNER-OBSERVED.** Anything whose truth is *"does this look right"* — contrast, legibility, occlusion, whether a ring reads as focus, whether a burst renders acceptably. For these the plan writes a numbered checklist, each item saying exactly what to do and exactly what should be seen. The implementer's job is to make every item unambiguous and the app reachable in the state the item needs. **The implementer does not perform, pre-fill or summarise an owner item**, and the results document carries each one as `PENDING OWNER` until the owner writes the answer.

**A step that asks an agent to judge appearance is a defect in this plan.** If an implementer finds one, the fix is to move the judgement into the owner checklist, not to make it.

**The rule from `group-d-real-browser-carries.md:18-23` binds every step: do not close a carry with a stub.** This task is the first place a real engine lays anything out, so there is no longer any reason to write `getBoundingClientRect = () => …`; there is only a reason to read the real one. And its corollary, from Phase 1's own results: a real run can lie too. Phase 1 recorded *"a false positive"* — a `healthz: 200` from a stale `sandboxd.exe` on the host while the container had failed (`2026-09-17-phase-1-results.md:57`). Step 6 checks which process holds each port before anything is believed.

#### What drives the window, and why it is not the Tauri shell first

**The headed harness drives the Vite dev server in Playwright's bundled Chromium, at `http://localhost:1420`.** Three reasons, in order of weight:

1. **It is the same origin as `tauri dev`.** `desktop/src-tauri/tauri.conf.json:8` sets `devUrl` to `http://localhost:1420`, and the master plan lists that exact origin as the dev webview's (`2026-09-18-phase-2-backend-and-cockpit.md:77`). So the control plane's origin allowlist, its CORS grant and the attach relay treat the harness exactly as they treat the dev window — not a similar origin, the same one.
2. **Playwright controls a browser it launched completely**, including `page.on("websocket")` and every `framesent` / `framereceived`, which is how step 10 proves no socket re-opens on a layout flip and step 11 sees a resize frame leave the page.
3. **It is the only browser automation this machine permits.** Corporate policy kills an automation-controlled system Edge or Chrome within seconds, while Playwright's *bundled* Chromium runs headed, and the Playwright CDN is reachable (both observed on this machine on 2026-09-07). Headless screenshots do not work here at all, which suits this task: it takes none.

**What that leaves unproven is named, not hidden.** The native window is WebView2, not Chromium-of-Playwright's-build, and one module touches Tauri: `ContextMenu.tsx:44` dynamically imports `@tauri-apps/plugin-opener` for *reveal in folder*, which cannot work in a plain browser. So step 15 attaches to the real WebView2 over the DevTools protocol for a smaller smoke set, and the owner checklist opens the native window by hand. If corporate policy refuses the DevTools port, step 15 records that and the native window stays owner-observed only.

**A note on the expected-output blocks, as in task 24.** No command in this plan has been run. Where a transcript appears it is the shape the tool prints and the named value is what matters; where I am unsure of the chrome I say so. Do not edit an assertion to match a transcript in a document.

**Files:**
- `control-plane/internal/harness/harness.go` — the Claude Code descriptor's evidence comment (`:123-124`) re-cited to 2.1.274 in the image. Comment only (modify)
- `control-plane/internal/harness/argv.go` — the `--permission-mode` evidence comment (`:5-7`) re-cited the same way. Comment only (modify)
- `desktop/package.json`, `desktop/package-lock.json` — `playwright` as a pinned dev dependency and an `e2e` script (modify)
- `desktop/e2e/phase2.e2e.mjs` — the headed harness. `.mjs` and outside `src/`, so vitest's `src/**/*.test.{ts,tsx}` never collects it and `tsc --noEmit` never type-checks it (new)
- `desktop/e2e/fixture.sh` — builds the throwaway git project the control plane enumerates. Bash rather than PowerShell so it runs through the audited wrapper (new)
- `docs/verification/<run-date>-phase-2-results.md` — the sibling of `2026-09-17-phase-1-results.md`, in the same form, dated by the day the run happens (new)

**Interfaces:**
- Produces:
  - `npm run e2e` — runs `desktop/e2e/phase2.e2e.mjs` against a running stack. Reads `PANACEA_E2E_APP` (default `http://localhost:1420`), `PANACEA_E2E_API` (default `http://127.0.0.1:8730`), `PANACEA_E2E_CDP` (unset: skip the WebView2 smoke), `PANACEA_E2E_ONLY` (a comma-separated list of check ids, for re-running one), and — for the checks that read the daemon's log or stop and start it — `PANACEA_E2E_CP_LOG`, `PANACEA_E2E_CP_BIN` and `PANACEA_E2E_CP_PID`. It also reads the control plane's own `PANACEA_APP_GIT_NAME` and `PANACEA_APP_GIT_EMAIL`, which step 6 exports, as the identity `g.primary` expects on a commit. Prints one JSON line per check and exits non-zero if any agent-provable check fails
  - The phase 2 results document, which phases 4, 5 and 7 are gated on: `docs/superpowers/planning/2026-09-23-phases-4-5-7/dependency-graph-and-order.md:319` lists *"the whole substrate, as a claim"* as safe to build against *"after task 25's real-window E2E"*
- Consumes: task 24's `App.tsx` (catalogue load and poller); task 13's `cmd/api` and its `PANACEA_APP_*` configuration (`control-plane/cmd/api/config.go:22-50`); task 5's `dockerbe`; task 12's attach relay; task 19's attach registry (`GRACE_MS = 5_000`, `attachRegistry.ts:20`); task 21's Canvas (`CANVAS_MAX_TILES = 10`, `prefsSlice.ts:17`) and its `cv-resize` handle (`CanvasLayout.tsx:362-368`); task 22's *Retry attach* row (`sessionVerbs.ts:409`); the wire protocol's four kinds (`sandboxd/internal/wire/frame.go:15-18`) and its big-endian `ResizePayload` (`:41-60`)
- Required by: task 26 cites the results document as the evidence that each seam it names was running when it was named
- **Depends on the resume fix landing first**, for step 14's `o2.resume` and step 18. It has landed (`b4be570`, round 1 after it): it mints a UUID per Claude Code session, starts the CLI with `--session-id <uuid>`, resumes by that UUID **as a fork** (`--resume <U> --fork-session --session-id <V>`, `argv.go:115-118`), and persists `/home/dev/.claude/projects` — only that directory — so the transcript survives into the new sandbox. The control plane resolves the cockpit id to the Claude UUID (`sessionResolveResume`), so the desktop still sends the cockpit id on purpose. Step 1 (h) checks for it
- **Depends on the sandbox git + CA fix** (`2d6d95b`…`4f6d698`), for step 14's `g.primary` and for any Claude Code session on this network. It passes git's `safe.directory`, `user.useConfigOnly` and an identity in the environment (`GIT_CONFIG_*`, `GIT_AUTHOR_*`, `GIT_COMMITTER_*`), and mounts `PANACEA_APP_CA_BUNDLE` read-only at `/etc/panacea-app/ca-bundle.pem`, named by `NODE_EXTRA_CA_CERTS` (`sessionEnv`, `sessionMounts`, `api/sessions.go`). Step 1 (h) and (i) check for it
- **Changes no behaviour.** Every `desktop/` change is the harness and its dependency; every Go change is a comment; nothing amends a product document. If the harness finds a defect, **it is reported, not fixed here** — it belongs to the task that owns the file, and this task's commit records it as a failing check

#### The steps at a glance

| # | Kind | What it settles |
|---|---|---|
| 1 | AGENT-PROVABLE | Task 24 landed; every symbol this task drives exists |
| 2 | AGENT-PROVABLE | The sandbox image is newer than `382f056` |
| 3 | AGENT-PROVABLE | Fail-closed startup and token rejection, against that image |
| 4 | AGENT-PROVABLE | Phase 1's checks re-run on the rebuilt image; task 5's Q3 and Q4 |
| 5 | AGENT-PROVABLE | Harness flags re-verified against 2.1.274; the two evidence comments corrected |
| 6 | AGENT-PROVABLE | The stack is up, and each port is held by the process we think |
| 7 | AGENT-PROVABLE | The harness: Playwright, a fixture project, a first session |
| 8 | AGENT-PROVABLE | Real-browser carries 2 and 3: non-zero columns; theme from tokens |
| 9 | AGENT-PROVABLE | Carries 7 and 9 (the numeric half): the popover clamp, the exit row |
| 10 | AGENT-PROVABLE | Carry 1: scrollback survives Columns ↔ Canvas; the cap boundary |
| 11 | AGENT-PROVABLE | Carry 5: a tile resize reflows and the PTY is told |
| 12 | AGENT-PROVABLE | Carries 4 and 6: pinned scroll under real parse; the 10 Hz flush measured |
| 13 | AGENT-PROVABLE | Durability: detach past the grace, reload, control-plane restart |
| 14 | AGENT-PROVABLE | Carry 10: Retry attach on a genuinely failed attach; carry 9's overlay hit-test; open carry 2: resume works; git works on a primary worktree |
| 15 | AGENT-PROVABLE | The native WebView2 window, over the DevTools protocol, if policy allows |
| 16 | OWNER-OBSERVED | The native window and the terminal |
| 17 | OWNER-OBSERVED | The Develop lens around the terminal |
| 18 | OWNER-OBSERVED | Resume restores the agent's context |
| 19 | AGENT-PROVABLE | The results document, and a disposition for every carry |
| 20 | AGENT-PROVABLE | Guards, then commit |

**Seventeen agent-provable, three owner-observed.** The owner steps are few because each carries a checklist rather than a claim; together they hold nineteen numbered items. **Per OD-12, all nineteen are batched to the owner's one hands-on session at the very end**, after phases 3 to 7: this task writes them and leaves each `PENDING OWNER`, unverified. The harness's agent-provable checks number **nineteen** too (step 20 lists them); the two nineteens are different things.

**Every shell command below runs through the audited wrapper**, `~/.claude/tools/claudezsh/claudezsh '<command>'`, per the device-wide rule; the blocks show them bare so they can be read. The master plan's constraint that Docker runs under PowerShell because *"Git Bash mangles container paths"* (`2026-09-18-phase-2-backend-and-cockpit.md:52`) is honoured differently here: every `docker` command that names a container path is prefixed `MSYS_NO_PATHCONV=1`, which switches off exactly the conversion that mangles it, and keeps the command in the audit trail that PowerShell would bypass. **The pipe exit-code trap from the same line still applies** — `cmd | head` reports `head`'s status — so no block below decides pass or fail from a piped exit code.

- [ ] **Step 1: Check what this task assumes, before running anything** · AGENT-PROVABLE

Every one of these belongs to another task. If one is false, **stop and report it to the orchestrator rather than patching that task's file** — the same rule as `task-24.md:33`, and for the same reason: this is the last-but-one task in the phase, so a silent local patch here is a patch nobody reviews.

**Every guard is anchored**, per shared shape §3A: it matches a line whose first non-space character begins the code, so a comment that quotes the code cannot satisfy it. `task-24.md:35-42` records two guards that were red against a correct tree for exactly that reason.

```bash
cd desktop

# (a) Task 24 landed: the placeholder is gone, the loader exists, the stylesheet loads.
grep -c  "The cockpit mounts here in task 24"          src/App.tsx          # expect 0
grep -cE '^export function startCatalogue\b'           src/App.tsx          # expect 1
grep -cE '^import "\./styles/app\.css";'               src/main.tsx         # expect 1
test -f src/defects.test.tsx && echo "defects.test.tsx: present"

# (b) The constants and controls the harness drives. Each is read by a step below, and a
#     changed value changes what that step must see — so a mismatch is a stop, not a shrug.
grep -cE '^export const GRACE_MS = 5_000;'             src/api/attachRegistry.ts          # 1
grep -cE '^export const CANVAS_MAX_TILES = 10;'        src/store/slices/prefsSlice.ts     # 1
grep -cE '^\s+className="cv-resize"$'                  src/components/develop/CanvasLayout.tsx  # 1
grep -cE '^\s+label: "Retry attach",$'                 src/components/develop/sessionVerbs.ts   # 1
grep -cE '^\s+\{ id: "(columns|canvas)", label: "(Columns|Canvas)" \},$' \
  src/components/topbar/LayoutToggle.tsx                                    # expect 2

# (c) No renderer addon. The harness reads terminal text from xterm's DOM renderer
#     (`.xterm-rows`), which is the renderer xterm uses when nothing else is loaded.
#     Terminal.tsx:47-48 loads FitAddon and nothing else. A webgl or canvas addon would
#     paint rows into a <canvas> and every text assertion below would read nothing.
grep -rnE '@xterm/addon-(webgl|canvas)' src package.json                    # expect no output

# (d) The theme path this task observes for the first time.
grep -cE '^const TERM_BG_TOKEN = "--page";$'           src/components/Terminal.tsx        # 1
grep -cE '^const TERM_FG_TOKEN = "--ink";$'            src/components/Terminal.tsx        # 1

cd ..

# (e) The two stale citations step 5 corrects are where the carry says, and the product
#     decision step 5 must NOT touch is intact.
grep -nF '2.1.267' control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go
                                                                            # two lines: harness.go:123, argv.go:6
grep -cE '^\s+Efforts:\s+\[\]Effort\{EffortLow, EffortMedium, EffortHigh, EffortXHigh\},$' \
  control-plane/internal/harness/harness.go                                 # 1 — four, by design
grep -nE '^- \*\*Effort\*\* — low / medium / high / xhigh \(Claude Code only\)$' docs/FUNCTIONAL-SPEC.md
                                                                            # expect exactly: 27:- **Effort** — …

# (f) Nothing this task edits is dirty, so the commits in step 20 hold only this task's work.
git status --porcelain -- control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go \
  desktop/package.json desktop/package-lock.json desktop/e2e                # expect no output

# (g) Docker is up, and nothing is listening yet on the three ports this task uses.
docker version --format '{{.Server.Version}}'                               # a version, not an error
netstat -ano | grep -E ':(8730|1420|9222)\s' | grep LISTENING               # expect no output

# (h) The resume fix and the sandbox git + CA fix — both landed BEFORE this one (see
#     Interfaces). The facts each must have changed; their commits say the rest. Anchored so a
#     comment cannot answer. The desktop still sends the COCKPIT id (`resume: from.id`, count
#     1) on purpose: the control plane resolves it (UI contract §4.4's resume note, Go
#     contract §3.9). So resume is checked server-side, never by that client line.
grep -cE '^[^/]*"--session-id"'   control-plane/internal/harness/argv.go    # >= 1 — Claude starts with its own UUID
grep -cE '^[^/]*"--fork-session"' control-plane/internal/harness/argv.go    # >= 1 — a resume forks
grep -cE '^[^/]*sessionResolveResume\(' control-plane/internal/api/sessions.go          # >= 1 — cockpit id -> Claude UUID
grep -cE '^[^/]*sessionResumableConversation\(' control-plane/internal/api/sessions.go  # >= 1 — round 1: no transcript, 409
grep -cE '^const sessionConversationDir = "/home/dev/\.claude/projects"$' \
  control-plane/internal/api/sessions.go                                   # 1 — the ONLY persisted directory
grep -cE '^\s+env\["(GIT_CONFIG_COUNT|GIT_AUTHOR_NAME|GIT_COMMITTER_EMAIL|NODE_EXTRA_CA_CERTS)"\] = ' \
  control-plane/internal/api/sessions.go                                   # 4 — the git + CA fix's sessionEnv
grep -cE '^\s+env(GitName|GitEmail|CABundle)\s+= "PANACEA_APP_(GIT_NAME|GIT_EMAIL|CA_BUNDLE)"$' \
  control-plane/cmd/api/config.go                                          # 3 — the variables step 6 exports

# (i) Is this machine behind TLS interception? Node in the image decides, the same probe as
#     the git + CA fix's own proof. Exit 0 with a status line: no interception, and
#     PANACEA_APP_CA_BUNDLE may stay unset. Exit 1 with SELF_SIGNED_CERT_IN_CHAIN (or
#     UNABLE_TO_GET_ISSUER_CERT_LOCALLY): interception, and PANACEA_APP_CA_BUNDLE is REQUIRED.
#     Write $TEMP/tls-probe.js with the Write tool (a heredoc through the wrapper can end
#     early), holding exactly these two lines:
#       require("https").get("https://api.anthropic.com/", r => { console.log("HTTP", r.statusCode); process.exit(0); })
#         .on("error", e => { console.log("ERROR", e.code); process.exit(1); });
MSYS_NO_PATHCONV=1 docker run --rm --entrypoint node -v "$(cygpath -w "$TEMP/tls-probe.js"):/tmp/p.js:ro" \
  panacea-app/sandbox:dev /tmp/p.js > "$TEMP/tls-probe.out" 2>&1; echo "exit=$?" >> "$TEMP/tls-probe.out"
cat "$TEMP/tls-probe.out"
#     If it failed: the same probe, trusting the bundle the way a sandbox will. Must print
#     "HTTP <status>" and exit=0, or the bundle is the wrong one.
MSYS_NO_PATHCONV=1 docker run --rm --entrypoint node -e NODE_EXTRA_CA_CERTS=/etc/panacea-app/ca-bundle.pem \
  -v "$(cygpath -w "$PANACEA_APP_CA_BUNDLE"):/etc/panacea-app/ca-bundle.pem:ro" \
  -v "$(cygpath -w "$TEMP/tls-probe.js"):/tmp/p.js:ro" \
  panacea-app/sandbox:dev /tmp/p.js > "$TEMP/tls-probe-ca.out" 2>&1; echo "exit=$?" >> "$TEMP/tls-probe-ca.out"
cat "$TEMP/tls-probe-ca.out"
```

**(i) decides step 6's environment, and it fails with a sentence.** If the first probe exits 1 and `PANACEA_APP_CA_BUNDLE` is unset, stop: *"This machine intercepts TLS and PANACEA_APP_CA_BUNDLE is not set, so Claude Code cannot start in any sandbox (SELF_SIGNED_CERT_IN_CHAIN). Export the network's root certificate as PEM and set PANACEA_APP_CA_BUNDLE to its absolute path."* That is not a harness nicety: without it interactive `claude` exits 1 with `SELF_SIGNED_CERT_IN_CHAIN`, which kills `p1.claude` and `ctx.claude`, the source session of `o2.resume`. **Exit codes go through a file, never a pipe** (the master plan's `:52` trap). `claude -p hi` is no substitute: it answers `Not logged in · Please run /login` before touching the network, with or without the bundle (`sandbox-git-fix-report.md`), so it proves nothing about TLS.

**(g) is the Phase 1 lesson made mechanical.** A stale listener on 8730 would answer every `curl` in step 6 and every fetch the harness makes, and the run would prove a process that is not the one this task built. If (g) prints a line, its last column is a PID: `tasklist //FI "PID eq <pid>"` names it, and it is stopped before anything else runs.

**If (h) shows the resume fix has not landed**, `o2.resume` (step 14) will fail and step 18's resumed session will not remember — both for the known reason, which proves nothing new. **If (h) shows the git + CA fix has not landed**, `g.primary` (step 14) fails for the known reason, and so does every Claude Code session behind interception. Either way, ask the orchestrator whether to wait for the fix. Every other step is independent of both and can proceed. **(h) found both landed at `a38c268`**, so this is a guard against a wrong checkout, not an expected branch. **If (e) finds `2.1.267` already gone**, someone corrected the comments first; step 5b becomes a check rather than an edit.

- [ ] **Step 2: Rebuild the sandbox image, and prove it is newer than the behaviour it must show** · AGENT-PROVABLE

The image must contain `382f056`. **A timestamp is necessary and not sufficient**: an image built today from a stale checkout is newer than the commit and still lacks it. So this step proves the build time, and step 3 proves the content by demanding the behaviour.

```bash
# The tree the image is built from must be committed, or the image holds code no commit names.
git status --porcelain -- sandboxd images/sandbox                           # expect no output
git merge-base --is-ancestor 382f056 HEAD && echo "382f056: in HEAD"

docker image inspect panacea-app/sandbox:dev --format '{{.Created}}'        # before
git log -1 --format='%cI %h %s' 382f056

docker build -f images/sandbox/Dockerfile \
  --label "panacea-app.source-commit=$(git rev-parse HEAD)" \
  -t panacea-app/sandbox:dev .

docker image inspect panacea-app/sandbox:dev \
  --format '{{.Created}} {{index .Config.Labels "panacea-app.source-commit"}}'
```

**What you must see.** The commit: `2026-09-22T08:43:47+10:00 382f056 fix(sandboxd): fail closed without a token, pin the Exit-frame guard, atomic finish`, which is `2026-09-21T22:43:47Z`. After: a `Created` later than that instant, followed by the full hash of `HEAD`. **Record all three values verbatim for the results document.**

**Amended 2026-09-25: this image now carries both fixes, and `Created` is not a build time.** The orchestrator rebuilt `:dev` from `a38c268` under the Docker lock (`reconcile-25-26-report.md` §1): image `sha256:5d9a580e035b…` (was `469a07f6c43d`, the Resume fix's build), label `panacea-app.source-commit=a38c268db11fec6d9fb80140993bd0f2f25e22dc`, CLI `2.1.274 (Claude Code)`. So *before* is that id and label, not the `2026-09-21T05:10:18.955144265Z` of `progress.md:5150`. Its `Created` is still `2026-09-23T06:52:05.425529555Z`, because every step was a cache hit: the layers are the Resume fix's, byte for byte, and only the config (the label) changed. That is correct, not stale. The last commit touching `sandboxd/` or `images/sandbox/` is `b4be570`; the git + CA fix changed neither tree (it is control-plane only: env and mounts set at create). **So the source-commit label is the proof that ties the image to a commit, and `Created` only bounds it from below.** Add one line to the block above and record its output: `git log -1 --format=%h -- sandboxd images/sandbox`, which must print a commit that is an ancestor of the label's. "An image that carries both fixes" means this image plus a control plane built from `HEAD` (step 6).

If the build fails fetching `golang:1.23-bookworm` or `node:20-bookworm-slim`, the corporate proxy has blocked a base-image pull; both are normally already present from Phase 1's build. Do not substitute a different base — `task-05.md:1457` records the half-hour that goes into the wrong problem. If it fails at `go build`, `sandboxd/vendor/` has fallen behind `sandboxd/go.mod` (the master plan's `:47` constraint): report it, because re-vendoring is not this task's change.

- [ ] **Step 3: Prove fail-closed startup and token rejection against that image** · AGENT-PROVABLE

This is the behaviour the stale image could not show, and it is proven here against the container directly, **before** the control plane is in the path — so a pass cannot be the control plane supplying a token on the image's behalf.

```bash
# (1) No token, no flag: sandboxd must refuse to start. The entrypoint passes no
#     -allow-unauthenticated (images/sandbox/entrypoint.sh), so this is the default path.
docker run --rm --name pa-noauth panacea-app/sandbox:dev ; echo "exit=$?"

# (2) With a token: it starts, and says so.
docker run -d --rm --name pa-auth -p 127.0.0.1::8080 \
  -e PANACEA_APP_ATTACH_TOKEN=e2e-secret panacea-app/sandbox:dev
docker port pa-auth 8080
PORT=$(docker port pa-auth 8080 | grep -m1 -oE '127\.0\.0\.1:[0-9]+' | cut -d: -f2)
docker logs pa-auth 2>&1 | grep 'sandboxd listening'

# (3) Readiness stays open; /attach does not.
curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:$PORT/healthz"
curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:$PORT/attach"
curl -s -o /dev/null -w '%{http_code}\n' -H 'Authorization: Bearer wrong' "http://127.0.0.1:$PORT/attach"
curl -s -o /dev/null -w '%{http_code}\n' --max-time 2 \
  -H 'Authorization: Bearer e2e-secret' -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  "http://127.0.0.1:$PORT/attach"

docker rm -f pa-auth
```

**What you must see.** (1) The container exits within a couple of seconds with `exit=1`, and its only log line carries the message `sandboxd: no attach token configured; set AttachToken (PANACEA_APP_ATTACH_TOKEN / -attach-token) or pass -allow-unauthenticated to run without one` — that string is `sandboxd/internal/server/server.go:71-73` verbatim, logged by `main.go:57-58` under `could not start the session`. The line's prefix (timestamp, level) is slog's default handler and will differ; match on the message. **A container that stays up here is the stale image**, and step 2 did not do what it said. (2) One `127.0.0.1:<port>` line — and possibly a second `[::1]:<same>` — and **no `0.0.0.0`**; the log line carries `authenticated=true` and `allow_unauthenticated=false` (`main.go:64-65`). (3) `200`, then `401`, `401`, then `101`: the bearer comparison at `server.go:109-115` is constant-time and scheme-prefixed, and `/healthz` is deliberately unauthenticated so readiness works (`backend.go:52-53`). The `101` line may be followed by a curl timeout message once `--max-time` expires; the code printed is what matters.

- [ ] **Step 4: Re-run Phase 1's checks on the rebuilt image, and close task 5's Q3 and Q4** · AGENT-PROVABLE

Phase 1's table (`2026-09-17-phase-1-results.md:21-36`) has **twelve rows under a 9/9 headline** — the three container-level rows (`/healthz`, `claude --version` as uid 1001, the upgrade) were evidently counted outside its harness. This task reports every row separately, and splits them by where they can be proven: the container-level rows here, against the image alone; the PTY-fidelity rows in step 8, **through the whole path** — browser, control-plane relay, sandboxd, PTY — which is a stronger proof than Phase 1's direct socket and the one the phase actually needs.

```bash
# Phase 1 rows 1-2, re-run. --entrypoint because the image's ENTRYPOINT execs sandboxd and
# ignores CMD (phase-1-results.md:81).
docker run --rm --entrypoint bash panacea-app/sandbox:dev -lc 'id -u; claude --version'

# Task 5's Q4 — the published address — and Q3 — the two State strings stateFrom maps.
docker run -d --name panacea-app-probe -p 127.0.0.1::8080 -l panacea-app.managed=true \
  -e PANACEA_APP_ATTACH_TOKEN=e2e-secret panacea-app/sandbox:dev
docker port panacea-app-probe
docker ps -a --filter label=panacea-app.managed=true --format '{{.Names}} {{.State}} {{.Ports}}'
docker stop panacea-app-probe
docker ps -a --filter label=panacea-app.managed=true --format '{{.Names}} {{.State}} {{.Ports}}'
docker rm -f panacea-app-probe
```

**What you must see.** `1001` and `2.1.274 (Claude Code)`. From `docker port`: `8080/tcp -> 127.0.0.1:<ephemeral>`, possibly a `[::1]` twin, **never `0.0.0.0`** — `task-05.md:1453` makes a `0.0.0.0` line a blocker, not a footnote, because the whole security property of `dockerbe` rests on it. From the two `docker ps -a` lines: `running`, then `exited` — the two strings `stateFrom` maps to `StateRunning` and `StateStopped`. **The probe now carries a token**, which task 5's original commands did not: against a post-`382f056` image, a tokenless probe exits at once and `docker ps` would show `exited` before `docker stop` ever ran, which is Q3 observed wrongly. Record each observed string verbatim; if either `State` differs, stop and report it, because task 13's reaper decides what to destroy from exactly that field (`task-05.md:1455`).

- [ ] **Step 5: Re-verify the harness flags against 2.1.274, and correct the two evidence comments** · AGENT-PROVABLE

**5a — the evidence, read from the binary that runs.** `progress.md:5160-5173` already did this once, by hand, and found the four flags; this step does it again so the citation in the code points at a reading this commit can reproduce, not at a ledger paragraph. `--session-id` is read too, because the resume fix this task depends on (Interfaces block) starts Claude Code with it.

```bash
SCRATCH="$(mktemp -d)"
docker run --rm --entrypoint claude panacea-app/sandbox:dev --version
docker run --rm --entrypoint claude panacea-app/sandbox:dev --help > "$SCRATCH/claude-help.txt"
grep -nE -- '--(permission-mode|model|effort|resume|session-id)\b' "$SCRATCH/claude-help.txt"
```

**What you must see.** `2.1.274 (Claude Code)`. Then five option lines, whose wording is the CLI's and is not reproduced here because I have not run it; what they must *contain* is fixed by `progress.md:5162-5171`: `--permission-mode` with exactly the six choices `acceptEdits, auto, bypassPermissions, manual, dontAsk, plan`; `--model` naming aliases such as `fable`, `opus`, `sonnet`; `--resume` taking a session id; `--session-id` taking a UUID; and `--effort` with five levels, `low, medium, high, xhigh, max`. **If `--permission-mode` shows anything other than those six, stop**: `argv.go:9-14`'s argument that omitting the flag *is* the "ask" behaviour rests on the choice list having no `default`, and a changed list re-opens it. That is a finding for the orchestrator, not an edit.

**The fifth `--effort` level is recorded, not acted on.** The catalogue offers four by a written product decision (`harness.go:37-40`, citing `docs/FUNCTIONAL-SPEC.md:27`), and that comment is still true against 2.1.274 — *"The CLI also accepts "max"; the product does not offer it"*. So it is not edited. Step 19's results document carries `max` as an open question for the owner, with this `--help` line as its evidence.

**5b — correct the two citations.** Comments only; no line of code changes. In `control-plane/internal/harness/harness.go`, the `ClaudeCode` descriptor's two-line evidence comment at `:123-124` becomes:

```go
		// Verified against `claude --help` in panacea-app/sandbox:dev, which
		// images/sandbox/Dockerfile pins to @anthropic-ai/claude-code@2.1.274 —
		// the binary that actually runs. Re-read by Phase 2 task 25 step 5a.
```

And in `control-plane/internal/harness/argv.go`, the first sentence of the evidence at `:5-7`:

```go
// claudePermissionFlag maps the product's three-word vocabulary onto the CLI's
// --permission-mode choices. Evidence: `claude --help` 2.1.274, in the pinned
// sandbox image, lists acceptEdits, auto, bypassPermissions, manual, dontAsk
// and plan.
```

**Read both surrounding comments before replacing them.** The resume fix lands before this task and may have edited `argv.go` near these lines — it adds `--session-id` handling. Replace only the sentences that cite 2.1.267, and keep anything the fix added.

```bash
gofmt -l control-plane/internal/harness                                     # expect no output
grep -nF '2.1.267' control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go   # expect no output
git diff --stat -- control-plane/internal/harness                           # two files, comment lines only
go test ./sandboxd/... ./control-plane/... -race -count=1
```

**What you must see:** no `gofmt` output, no remaining `2.1.267`, a diff touching exactly `harness.go` and `argv.go`, and every package `ok`. `git diff -- control-plane/internal/harness` read by eye shows only `//` lines changed; a changed line of code means an edit slipped, and it is undone before going on.

- [ ] **Step 6: Bring up the stack, and prove which process holds each port** · AGENT-PROVABLE

**The fixture first.** The control plane enumerates *"subdirectories of root that contain a .git entry"* (`control-plane/internal/project/project.go:178`), and a linked worktree has `.git` as a *file* (`:207`) — so a linked worktree created inside the projects root would be listed as a second project. The fixture therefore keeps worktrees beside the root, not in it. Create `desktop/e2e/fixture.sh`:

```bash
#!/usr/bin/env bash
# Task 25. A throwaway project for the Phase 2 end-to-end run: one repository with two
# branches, and one linked worktree on the second. Linked worktrees live OUTSIDE the
# projects root, because project.go enumerates every child directory holding a .git
# entry and a linked worktree's .git is a file — inside the root it would be a project.
#
# Usage: bash desktop/e2e/fixture.sh <empty-directory>
# Prints the Windows form of the projects root, for PANACEA_APP_PROJECTS_DIR.
set -euo pipefail

root="${1:?usage: fixture.sh <empty-directory>}"
[ -z "$(ls -A "$root" 2>/dev/null)" ] || { echo "fixture: $root is not empty" >&2; exit 1; }

mkdir -p "$root/projects/e2e-demo" "$root/worktrees" "$root/state"
cd "$root/projects/e2e-demo"
git init -q -b main
git -c user.name=e2e -c user.email=e2e@invalid commit -q --allow-empty -m "e2e: root"
git branch feat/e2e
git worktree add -q "$root/worktrees/e2e-demo-feat" feat/e2e

cygpath -w "$root/projects"
```

```bash
E2E_ROOT="$(mktemp -d)"
bash desktop/e2e/fixture.sh "$E2E_ROOT"
git -C "$E2E_ROOT/projects/e2e-demo" worktree list
```

**What you must see:** a Windows path ending `\projects`, then two worktree lines — the clone on `[main]` and `…/worktrees/e2e-demo-feat` on `[feat/e2e]`. That is the whole shape the run needs: one project, two branches, each branch with exactly one worktree, so `TopBar.tsx:15-18`'s *"the selected worktree, else the branch's first"* resolves without a click in a sub-rail.

**Then the control plane — as a built binary, not `go run`.** A backgrounded `go run` leaves its compiled child running when the parent is stopped, and step 13 stops and restarts this process on purpose; a restart that leaves the old child holding 8730 would have the harness talk to the process it thinks it killed. Build once, start the binary, stop it by PID.

```bash
mkdir -p "$E2E_ROOT/bin"
( cd control-plane && go build -o "$E2E_ROOT/bin/panacea-api.exe" ./cmd/api )

# The Docker lock first (SDD docker-daemon-lock.md): mkdir the lock, write owner.txt, then
# the guard. Any managed container you did not create means: do not start, release, report.
docker ps -a --filter label=panacea-app.managed=true --format "{{.ID}} {{.Names}} {{.Status}}"

# Exported, not prefixed: the harness reads the identity (g.primary), and step 13 restarts
# this binary from the same shell. PANACEA_APP_CA_BUNDLE only if step 1 (i) found
# interception; it is an absolute host path to a PEM file, validated at startup (exit 2).
export PANACEA_APP_GIT_NAME="Panacea E2E" PANACEA_APP_GIT_EMAIL="e2e@invalid"
export PANACEA_APP_CA_BUNDLE="<absolute PEM path, or leave unset if (i) passed without it>"
export PANACEA_APP_REAP_INTERVAL=24h

PANACEA_APP_PROJECTS_DIR="$(cygpath -w "$E2E_ROOT/projects")" \
PANACEA_APP_STATE_DIR="$(cygpath -w "$E2E_ROOT/state")" \
PANACEA_APP_SESSION_CPU_MILLIS=250 \
PANACEA_APP_SESSION_MEMORY_MIB=512 \
PANACEA_APP_LOG_LEVEL=debug \
  "$E2E_ROOT/bin/panacea-api.exe" > "$E2E_ROOT/control-plane.log" 2>&1 &
echo "control plane pid: $!"
```

**Three more variables since the fixes, each with a reason** (`control-plane/cmd/api/config.go`, `envGitName`, `envGitEmail`, `envCABundle`). **The git pair is explicit**, so `g.primary` asserts a known author and not whatever the host's `git var GIT_AUTHOR_IDENT` answers; the pair must be complete, or startup exits 2 naming both. **`PANACEA_APP_CA_BUNDLE`** is set exactly when step 1 (i) found interception; startup refuses a relative path, a missing file, a directory, or a file with no `-----BEGIN CERTIFICATE-----` block, each with its own sentence and exit 2. **`PANACEA_APP_REAP_INTERVAL=24h`** is the lock's rule 3. A control plane from `HEAD` reaps only its own instance's sandboxes (`a9c2cec`), so the interval is no longer a safety measure against this build; the lock still binds, because an older build on the daemon still reaps everything (`docker-daemon-lock.md`, amended 2026-09-24). **Release the lock in step 20**, after its `docker ps` guard prints nothing.

Four choices in the rest of that environment, each with a reason. **`PANACEA_APP_STATE_DIR`** points the registry at the throwaway root (`config.go:248-252`), so this run cannot read or corrupt the owner's real `sessions.json`. **The two resource variables** (`config.go:32-33`) shrink each sandbox from the 2-core, 4 GiB default (`backend.go:110`): step 10 runs eleven sessions at once, and at the default that is 44 GiB of limit on a laptop. **`debug`** so the log has everything step 13 reads. **No `PANACEA_APP_ALLOWED_ORIGINS`**: the defaults already include `http://localhost:1420` (`control-plane/internal/httpx/origin.go:46`), and setting the variable *replaces* them (`config.go:225-228`).

**And the cockpit's dev server**, in a second background shell:

```bash
cd desktop && npm run dev > "$E2E_ROOT/vite.log" 2>&1 &
```

`vite.config.ts:17-18` pins port 1420 with `strictPort: true`, so if anything already holds it Vite exits rather than drifting to 1421 — which is the behaviour this task wants, and why step 1 (g) checked the port first.

**Now prove who is listening, before believing any answer.**

```bash
netstat -ano | grep -E ':(8730|1420)\s' | grep LISTENING
tasklist //FI "IMAGENAME eq panacea-api.exe"
curl -s http://127.0.0.1:8730/v1/machines
curl -s http://127.0.0.1:8730/v1/projects
curl -s http://127.0.0.1:8730/v1/harnesses | grep -o '"efforts":\[[^]]*\]'
grep -m1 'control plane listening' "$E2E_ROOT/control-plane.log"
```

**What you must see.** The 8730 line reads `127.0.0.1:8730` — **never `0.0.0.0:8730`** — and its PID is the one `tasklist` names as `panacea-api.exe`, and the one printed when it started. The 1420 line's PID is a `node.exe`. `/v1/machines` returns one machine; `/v1/projects` returns one project named `e2e-demo`. The `efforts` arrays read `["low","medium","high","xhigh"]` for `claude-code` — four, the product decision step 5 left alone — and `[]` for every other harness, never `null` (`harness.go:183-187`). The log line is `main.go:155`'s, and it prints the effective origin allowlist. **The binary is fresh by construction** — it was built a minute ago from `HEAD` into `$E2E_ROOT/bin` — and `tasklist` confirms it is the process on the port; no API response is needed to prove its age.

**CORS is not re-proven here.** `task-24.md:1240-1278` proves the grant, the preflight and the readable 403 against a running control plane; this task relies on that evidence and says so in the results document rather than running the same three `curl`s a second time.

- [ ] **Step 7: Install Playwright, build the fixture project, write the harness frame** · AGENT-PROVABLE

**The dependency is `playwright`, the library, not `@playwright/test`.** The runner would bring its own config, its own `*.spec.ts` discovery and its own assertion library, and this harness needs none of them: it is one script that runs checks in order against one long-lived page, because the checks share state on purpose — the session step 10 flips is the one step 8 typed into. **Pin the version exactly**, because a caret on a browser-automation library is a new Chromium on the next install.

```bash
cd desktop
npm view playwright version                        # record this value; call it <ver>
npm install --save-dev --save-exact playwright@<ver>
npx playwright install chromium                    # the BUNDLED build; see the preamble
```

Then add one script to `desktop/package.json`, after `"typecheck"`:

```json
    "typecheck": "tsc --noEmit",
    "e2e": "node e2e/phase2.e2e.mjs"
```

**What you must see:** `package.json` gains exactly one `devDependencies` entry and one script, and `package-lock.json` changes. `git diff --stat desktop/package.json` shows two added lines and nothing removed; a removed line means npm re-sorted or re-ranged something else, and that is undone before going on. `npx playwright install chromium` ends by naming a directory under `%LOCALAPPDATA%\ms-playwright\chromium-<build>`. **Do not pass `channel: "msedge"` or `channel: "chrome"` anywhere**: on this machine corporate policy closes an automation-controlled system browser within seconds, and the failure looks like a harness bug.

**Now the harness frame.** Create `desktop/e2e/phase2.e2e.mjs`. The checks themselves are appended by steps 8 to 15, each **inserted above the final `await main();` line**; this block is everything they share.

```js
// desktop/e2e/phase2.e2e.mjs
// Task 25. The headed end-to-end harness for Phase 2. It drives the cockpit's Vite dev
// server in Playwright's BUNDLED Chromium, against a real control plane and real Docker,
// and asserts on behaviour only: DOM state laid out by a real engine, the frames on the
// attach sockets, what the PTY reports about itself, and what the control plane logs.
//
// It takes no screenshot and judges no appearance. CLAUDE.md: "A screenshot is not
// evidence that something works", and visual acceptance "is the owner's eyes, not a
// self-report". What looks right is task 25's owner checklist, not this file.
//
// Run from desktop/ with the stack already up (task 25 step 6):   npm run e2e
// Prints one JSON line per check; exits 1 if any check failed.

import { readFileSync } from "node:fs";
import { chromium } from "playwright";

const APP = process.env.PANACEA_E2E_APP ?? "http://localhost:1420";
const API = process.env.PANACEA_E2E_API ?? "http://127.0.0.1:8730";
const CDP = process.env.PANACEA_E2E_CDP ?? "";
/** The control plane's log file, for the checks that read the daemon's own account. */
const CP_LOG = process.env.PANACEA_E2E_CP_LOG ?? "";
const ONLY = new Set((process.env.PANACEA_E2E_ONLY ?? "").split(",").filter(Boolean));

// sandboxd/internal/wire/frame.go:15-18 — one kind byte, then the payload.
const STDOUT = 0x00;
const RESIZE = 0x02;
const EXIT = 0x03;

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/** Polls `probe` until it returns something truthy, and returns that. Throws naming
 *  `what` when `ms` runs out, so a timeout says which condition never arrived. */
async function until(what, probe, ms = 10_000, every = 100) {
  const deadline = Date.now() + ms;
  for (;;) {
    const value = await probe();
    if (value) return value;
    if (Date.now() > deadline) throw new Error(`timed out after ${ms} ms waiting for ${what}`);
    await sleep(every);
  }
}

/** The control plane, from Node. Node's fetch sends no Origin header, which
 *  OriginPolicy.Allow treats as a non-browser client (control-plane/internal/httpx/origin.go:118-120). */
async function api(method, path, body) {
  const res = await fetch(API + path, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status} ${text}`);
  return text === "" ? null : JSON.parse(text);
}

function cpLogLines(pattern) {
  if (CP_LOG === "") throw new Error("PANACEA_E2E_CP_LOG is not set; this check reads the daemon's log");
  return readFileSync(CP_LOG, "utf8").split("\n").filter((line) => pattern.test(line));
}

/** Every attach socket the page opens, with its traffic decoded. The page is the only
 *  client, so this list IS the set of attaches the cockpit made — which is how a check
 *  proves a socket did NOT re-open, not merely that one is open now. */
function watchSockets(page, sockets) {
  page.on("websocket", (ws) => {
    const m = /\/v1\/sessions\/([^/?]+)\/attach/.exec(ws.url());
    if (m === null) return;
    const rec = {
      sessionId: decodeURIComponent(m[1]),
      opened: Date.now(),
      closed: null,
      stdout: "",
      resizes: [],
      exited: false,
    };
    const decoder = new TextDecoder();
    const bytes = (payload) => (typeof payload === "string" ? Buffer.from(payload) : payload);
    ws.on("framereceived", ({ payload }) => {
      const b = bytes(payload);
      if (b[0] === STDOUT) rec.stdout += decoder.decode(b.subarray(1), { stream: true });
      if (b[0] === EXIT) rec.exited = true;
    });
    ws.on("framesent", ({ payload }) => {
      const b = bytes(payload);
      // ResizePayload: big-endian uint16 cols, then rows (frame.go:41-60).
      if (b[0] === RESIZE && b.length >= 5) {
        rec.resizes.push({ cols: b.readUInt16BE(1), rows: b.readUInt16BE(3), at: Date.now() });
      }
    });
    ws.on("close", () => { rec.closed = Date.now(); });
    sockets.push(rec);
  });
}

const paneOf = (id) => `[data-session-id="${id}"]`;
const socketsFor = (h, id) => h.sockets.filter((s) => s.sessionId === id);
const stdoutOf = (h, id) => socketsFor(h, id).map((s) => s.stdout).join("");

/** The rows xterm's DOM renderer has ON SCREEN for one session: the viewport, not the
 *  buffer. xterm renders a space as U+00A0, normalised so a pattern can say " ". */
async function screen(page, id) {
  return page.$$eval(`${paneOf(id)} .xterm-rows > div`, (els) =>
    els.map((el) => (el.textContent ?? "").replace(/ /g, " ").trimEnd()));
}

/** Types into one session's terminal the way a user does: click it, type, press Enter. */
async function type(page, id, command) {
  await page.click(`${paneOf(id)} .xterm`);
  await page.keyboard.type(command);
  await page.keyboard.press("Enter");
}

/** Asks the PTY its own size. The command text contains no digit after "SIZE=", so the
 *  shell's echo of the command can never satisfy the pattern — only the answer can. */
async function ptySize(h, id) {
  const before = stdoutOf(h, id).length;
  await type(h.page, id, "stty size | sed 's/^/SIZE=/'");
  const m = await until(`stty size from ${id}`, () =>
    /SIZE=(\d+) (\d+)/.exec(stdoutOf(h, id).slice(before)));
  return { rows: Number(m[1]), cols: Number(m[2]) };
}

/** Shared across checks, deliberately. Filled by the "setup" check. */
const ctx = { machineId: "", projectId: "", branchId: "", worktreeId: "", claude: "", sh: "", created: [] };

async function createShell(title) {
  const s = await api("POST", "/v1/sessions", {
    machineId: ctx.machineId,
    worktreeId: ctx.worktreeId,
    harness: "shell",
    title,
  });
  ctx.created.push(s.id);
  return s;
}

const CHECKS = [];
/** `closes` names the carry or row the check is evidence for, so step 19's table can be
 *  built from this script's output rather than from memory. `always` runs even under
 *  PANACEA_E2E_ONLY, because other checks depend on what it leaves behind. */
function check(id, closes, fn, { always = false } = {}) {
  CHECKS.push({ id, closes, fn, always });
}

async function main() {
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
  const page = await context.newPage();
  const h = { browser, context, page, sockets: [] };
  watchSockets(page, h.sockets);

  let failed = 0;
  for (const c of CHECKS) {
    if (ONLY.size > 0 && !ONLY.has(c.id) && !c.always) continue;
    const started = Date.now();
    try {
      const observed = await c.fn(h);
      console.log(JSON.stringify({ id: c.id, closes: c.closes, ok: true, ms: Date.now() - started, observed }));
    } catch (error) {
      failed += 1;
      console.log(JSON.stringify({ id: c.id, closes: c.closes, ok: false, ms: Date.now() - started, error: String(error?.message ?? error) }));
      if (c.always) break;   // nothing after a failed setup can mean anything
    }
  }

  // Kill every session this run made. DELETE destroys the sandbox and keeps the record
  // (docs/FUNCTIONAL-SPEC.md:36 — finished sessions stay listed); the registry is the
  // throwaway PANACEA_APP_STATE_DIR, so the records go with it.
  for (const id of ctx.created) await api("DELETE", `/v1/sessions/${id}`).catch(() => {});
  await browser.close();
  process.exit(failed === 0 ? 0 : 1);
}
```

Then the one check everything depends on, and the last line:

```js
check("setup", "task 24 step 11, re-run: the catalogue loads and a session can be created from the UI", async (h) => {
  const machines = (await api("GET", "/v1/machines")).machines;
  const project = (await api("GET", "/v1/projects")).projects.find((p) => p.name === "e2e-demo");
  if (project === undefined) throw new Error("the fixture project is not enumerated; check PANACEA_APP_PROJECTS_DIR");
  const branch = (await api("GET", `/v1/projects/${project.id}/branches`)).branches.find((b) => b.name === "feat/e2e");
  const worktree = (await api("GET", `/v1/projects/${project.id}/worktrees?branchId=${branch.id}`)).worktrees[0];
  Object.assign(ctx, { machineId: machines[0].id, projectId: project.id, branchId: branch.id, worktreeId: worktree.id });

  // The user's path: open the project, pick the branch, press the split button's left half.
  await h.page.goto(APP);
  await h.page.click(`[data-kind="project"][data-id="${project.id}"]`);
  await h.page.click(`[data-kind="branch"][data-id="${branch.id}"]`);
  const known = new Set((await api("GET", "/v1/sessions")).sessions.map((s) => s.id));
  await until("the New session button to enable", () => h.page.isEnabled(".top-split-main"));
  await h.page.click(".top-split-main");
  const made = await until("the UI-created session to reach the control plane", async () =>
    (await api("GET", "/v1/sessions")).sessions.find((s) => !known.has(s.id)));
  ctx.claude = made.id;
  ctx.created.push(made.id);
  await until("a pane for the UI-created session", () => h.page.$(paneOf(ctx.claude)));
  await until("an attach socket for it", () => socketsFor(h, ctx.claude).length > 0, 30_000);

  // The shell every later check types into, made over REST: the poller must find it.
  ctx.sh = (await createShell("e2e-shell")).id;
  await until("the poller to render the REST-created session", () => h.page.$(paneOf(ctx.sh)), 10_000);
  await until("the shell's prompt", () => /\$ $/m.test(stdoutOf(h, ctx.sh)), 30_000);
  return { harness: made.harness, claude: ctx.claude, shell: ctx.sh };
}, { always: true });

await main();
```

**Two details in `setup` that are easy to "fix" wrongly.** The first session is created **through the UI** — rail row, branch row, `＋ New session` (`NewSessionButton.tsx:18-32`) — and every later one over REST. That split is the point: the UI path proves the create verb end to end once, and the REST path proves the *poller* finds a session the UI did not make, within its 2 s tick. Replacing the first with REST loses the only real-window proof of task 18's split button. And the shell prompt pattern `/\$ $/m` assumes bash's default prompt for a non-root user ends `$ `; `entrypoint.sh` runs a login shell as uid 1001 (`harness.go` builds `-l`, `argv.go:62-66`). If the image ever sets `PS1`, change the pattern, not the timeout.

```bash
cd desktop
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=setup npm run e2e
```

**What you must see:** a Chromium window opening on the cockpit, the rail expanding and a pane appearing, and one JSON line: `{"id":"setup",…,"ok":true,…,"observed":{"harness":"claude-code",…}}`. The harness is whatever `resolveDefaults` picked for the one-click path; `claude-code` is expected because the app-level default is `harness: "claude-code"` (`store/slices/prefsSlice.ts:20`) and `clampHarness` keeps it while the catalogue marks it available (`store/defaults.ts:39-44`). Anything else is recorded, not corrected. `ok:false` naming `the fixture project is not enumerated` means the control plane was started without `PANACEA_APP_PROJECTS_DIR`; naming `a pane for the UI-created session` means the create reached the server and the cockpit never rendered it — that is task 20's or task 24's defect and is reported, per the Interfaces block.

- [ ] **Step 8: The first real terminal — non-zero columns, and a theme from the tokens** · AGENT-PROVABLE

Three checks, and the first carries Phase 1's PTY-fidelity rows through the whole Phase 2 path. **Each assertion is built so the shell's echo of the typed command cannot satisfy it** — the box-drawing command is typed as octal escapes, the exit-status marker is split by quotes, the cursor check reads row 0 *after* a clear. A fidelity check that passes on the echo proves the keyboard, not the pipe; Phase 1 recorded two false negatives from exactly this family of harness mistake (`2026-09-17-phase-1-results.md:58`), and a false positive is the worse direction.

Insert above `await main();`:

```js
check("p1.fidelity", "phase-1 rows: ANSI, box drawing, cursor control, exit status, 500-line burst, Ctrl-C — through the relay", async (h) => {
  const id = ctx.sh;
  const since = () => stdoutOf(h, id).length;

  // ANSI colour: xterm's DOM renderer puts palette colour 1 on a span as class xterm-fg-1.
  await type(h.page, id, "printf '\\033[31mRED%s\\033[0m\\n' OK");
  await until("a red span reading REDOK", () => h.page.$$eval(`${paneOf(id)} .xterm-rows span`, (spans) =>
    spans.some((s) => /\bxterm-fg-1\b/.test(s.className) && (s.textContent ?? "").includes("REDOK"))));

  // Box drawing, typed as octal so the echo holds no box character: ╭─╮
  await type(h.page, id, "printf '\\342\\225\\255\\342\\224\\200\\342\\225\\256\\n'");
  await until("╭─╮ on screen", async () => (await screen(h.page, id)).some((r) => r.includes("╭─╮")));

  // Cursor control: clear and home, then print. Row 0 can only read TOPMARK if both worked.
  await type(h.page, id, "printf '\\033[2J\\033[HTOPMARK\\n'");
  await until("TOPMARK on row 0", async () => (await screen(h.page, id))[0] === "TOPMARK");

  // Exit status propagates.
  let from = since();
  await type(h.page, id, "false; echo \"R\"\"C=$?\"");
  await until("RC=1", () => stdoutOf(h, id).slice(from).includes("RC=1"));

  // 500 lines, every one, in order, as the PTY emits them (onlcr: \r\n).
  from = since();
  await type(h.page, id, "seq -f 'L%g' 1 500");
  const expected = Array.from({ length: 500 }, (_, i) => `L${i + 1}`).join("\r\n");
  await until("all 500 lines in order", () => stdoutOf(h, id).slice(from).includes(expected), 15_000);

  // Ctrl-C reaches the process and the shell survives it.
  await type(h.page, id, "sleep 30");
  await sleep(500);
  await h.page.keyboard.press("Control+C");
  from = since();
  await type(h.page, id, "echo \"ALI\"\"VE\"");
  await until("ALIVE after Ctrl-C", () => stdoutOf(h, id).slice(from).includes("ALIVE"), 5_000);
  return "six rows, all through browser -> control plane -> sandboxd -> PTY";
});

check("p1.claude", "phase-1 rows: Claude Code runs through the PTY, and its help renders", async (h) => {
  const from = stdoutOf(h, ctx.sh).length;
  await type(h.page, ctx.sh, "claude --help | head -3");
  await until("claude's usage line", () => /Usage: claude/.test(stdoutOf(h, ctx.sh).slice(from)), 15_000);
  // The UI-created session runs claude itself. Whatever its first screen is — login, theme
  // choice — it is a TUI, and a TUI draws with box characters (U+2500-U+257F).
  const rows = await until("claude's first screen to draw", async () => {
    const r = await screen(h.page, ctx.claude);
    return r.some((line) => /[─-╿]/.test(line)) ? r : null;
  }, 45_000);
  return { firstRows: rows.filter((r) => r.trim() !== "").slice(0, 6) };
});

check("c2.cols", "real-browser carry 2: mount after append yields a non-zero column count", async (h) => {
  const pty = await ptySize(h, ctx.sh);
  const sent = socketsFor(h, ctx.sh).flatMap((s) => s.resizes);
  const last = sent.at(-1);
  if (last === undefined) throw new Error("no resize frame was ever sent for the shell");
  if (pty.cols < 20 || pty.rows < 5) throw new Error(`the PTY reports ${pty.rows}x${pty.cols}`);
  if (last.cols !== pty.cols || last.rows !== pty.rows) {
    throw new Error(`last resize frame ${last.rows}x${last.cols} but the PTY reports ${pty.rows}x${pty.cols}`);
  }
  const box = await h.page.$eval(`${paneOf(ctx.sh)} .pane-body`, (el) => {
    const r = el.getBoundingClientRect();
    return { width: Math.round(r.width), height: Math.round(r.height) };
  });
  return { pty, resizeFrames: sent.length, paneBody: box };
});

check("c3.theme", "real-browser carry 3: themeFromTokens() produces real colours", async (h) =>
  h.page.evaluate((sel) => {
    // Normalise any CSS colour to RGBA bytes by painting it. An unparseable value leaves
    // the sentinel in place, so "the token did not parse" is distinguishable from "black".
    const g = Object.assign(document.createElement("canvas"), { width: 1, height: 1 }).getContext("2d");
    const rgba = (css) => {
      g.clearRect(0, 0, 1, 1);
      g.fillStyle = "rgb(1, 2, 3)";
      g.fillStyle = css;
      g.fillRect(0, 0, 1, 1);
      return Array.from(g.getImageData(0, 0, 1, 1).data).join(",");
    };
    const SENTINEL = "1,2,3,255";
    const root = getComputedStyle(document.documentElement);
    const page = rgba(root.getPropertyValue("--page").trim());
    const ink = rgba(root.getPropertyValue("--ink").trim());
    if (page === SENTINEL || ink === SENTINEL) throw new Error(`a token did not parse: --page ${page}, --ink ${ink}`);

    const term = document.querySelector(`${sel} .xterm`);
    if (term === null) throw new Error("no .xterm in the pane");
    const painted = [term, ...term.querySelectorAll("*")]
      .map((el) => rgba(getComputedStyle(el).backgroundColor))
      .filter((c) => !c.endsWith(",0"));
    const rows = term.querySelector(".xterm-rows");
    const fg = rows === null ? "" : rgba(getComputedStyle(rows).color);
    // xterm's own defaults are black on white-ish; either one here means the theme was dropped.
    if (!painted.includes(page)) throw new Error(`no terminal surface is painted --page (${page}); saw ${[...new Set(painted)]}`);
    if (fg !== ink) throw new Error(`.xterm-rows is ${fg}, --ink is ${ink}`);
    return { page, ink, surfacesPaintedPage: painted.filter((c) => c === page).length };
  }, paneOf(ctx.sh)));
```

```bash
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=p1.fidelity,p1.claude,c2.cols,c3.theme npm run e2e
```

**What you must see:** five lines, `setup` and these four, every one `"ok":true`. Read each `observed`, because two of them carry numbers the results document records: `c2.cols` reports the PTY's rows and columns and the pane body's pixel box — carry 9 asks *"whether an agent's output wraps legibly"* at `--pane-w: 520px` (`develop.css:28`), and the column count at that width is the agent-provable half of that question — and `p1.claude` reports the first six non-empty rows of Claude Code's first screen, which the owner will be looking at in step 16.

**What each failure would mean.** `c2.cols` reporting `0` columns, or throwing *no resize frame was ever sent*, is carry 2's failure exactly — `open()` on a detached node — and it would read to a user like a socket problem (`group-d-real-browser-carries.md:51-54`); it is task 19's, and it is reported. `c3.theme` throwing *a token did not parse* means Chromium's canvas rejected `oklch()` — it should not, the Tauri 2 webview clears Chromium 111 (`survey-phase-2-substrate.md:378-379`) and so does Playwright's build — while *no terminal surface is painted --page* means xterm received the theme and did not use it, or never received it. `themeFromTokens()` returns `undefined` when either token reads empty (`Terminal.tsx:26-32`), and xterm then keeps its defaults silently, which is the precise shape of carry 3. **Whether `--page` and `--ink` make a legible terminal is not asked here**: it is item 3 of step 16.

- [ ] **Step 9: The Develop lens's geometry that is a number** · AGENT-PROVABLE

Carries 7 and 9 each mix a measurable claim with a visual one, and this step takes only the measurable half. **A rectangle's coordinates are a fact; whether the thing inside it looks right is not**, and the second half of each carry is in step 17.

**Carry 7 — `Popover`'s viewport clamp** (`group-d-real-browser-carries.md:86-87`). Four surfaces mount `<Popover>`: `SessionMenu.tsx`, `ContextMenu.tsx`, `MachineChip.tsx` and `SessionPicker.tsx`. The machine chip is the one that must flip — it is `.rail-machine` in the rail's footer, at the bottom of the window, so a popover that opened *below* it would open off-screen. The check opens it at two window heights, and at the short one there is no room below at all.

**Carry 9's exit row** (`group-d-real-browser-carries.md:101-103`). `.pane` is `grid-template-rows: auto 1fr auto auto` (`develop.css:65`). Chromium reports a grid container's `grid-template-rows` as its **resolved track sizes in pixels**, so the third track can be read directly: it must be `0px` while the session runs, non-zero once it has exited, and the pane's own outer box must not move when it happens. A session that exits and pushes its neighbours is the layout shift `docs/AGENTIC-UX.md` §10 rule 7 bans.

Insert above `await main();`:

```js
check("c7.popover", "real-browser carry 7: Popover's viewport clamp, including the flip-above branch", async (h) => {
  const results = [];
  for (const height of [1000, 560]) {
    await h.page.setViewportSize({ width: 1600, height });
    await h.page.click(".rail-machine");
    const pop = await until("the machine popover", () => h.page.$(".pop"));
    const r = await pop.evaluate((el) => {
      const b = el.getBoundingClientRect();
      const chip = document.querySelector(".rail-machine").getBoundingClientRect();
      return { top: b.top, bottom: b.bottom, left: b.left, right: b.right,
               vw: innerWidth, vh: innerHeight, above: b.bottom <= chip.top + 1 };
    });
    await h.page.keyboard.press("Escape");   // hooks/useDismissable owns Escape (AppShell.tsx:78-79)
    await until("the popover to close", async () => (await h.page.$(".pop")) === null);
    if (r.top < 0 || r.left < 0 || r.bottom > r.vh || r.right > r.vw) {
      throw new Error(`at ${height}px the popover spills the viewport: ${JSON.stringify(r)}`);
    }
    results.push({ height, ...r });
  }
  await h.page.setViewportSize({ width: 1600, height: 1000 });
  if (!results.some((r) => r.above)) throw new Error("the popover never opened above the chip; the flip branch did not run");
  return results;
});

check("c9.exit-row", "real-browser carry 9: the exit row is zero-height until exit, and the pane does not move", async (h) => {
  const s = await createShell("e2e-exit");
  await until("the exit session's pane", () => h.page.$(paneOf(s.id)), 10_000);
  await until("its prompt", () => /\$ $/m.test(stdoutOf(h, s.id)), 30_000);
  const measure = () => h.page.$eval(paneOf(s.id), (el) => {
    const b = el.getBoundingClientRect();
    return { rows: getComputedStyle(el).gridTemplateRows, top: b.top, height: b.height,
             exitBar: el.querySelector(".pane-exit") !== null };
  });
  const before = await measure();
  if (before.exitBar) throw new Error("a running session already shows the exit bar");
  if (before.rows.split(" ")[2] !== "0px") throw new Error(`third track is ${before.rows} while running`);

  await type(h.page, s.id, "exit");
  await until("the exit bar", async () => (await measure()).exitBar, 10_000);
  const after = await measure();
  if (after.rows.split(" ")[2] === "0px") throw new Error(`exit bar present but its track is ${after.rows}`);
  if (Math.abs(after.top - before.top) > 0.5 || Math.abs(after.height - before.height) > 0.5) {
    throw new Error(`the pane moved: ${JSON.stringify({ before, after })}`);
  }
  return { before: before.rows, after: after.rows };
});
```

```bash
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=c7.popover,c9.exit-row npm run e2e
```

**What you must see:** both `"ok":true`. `c7.popover`'s `observed` has two entries; the 560 px one must have `"above":true`, and the check itself refuses to pass unless at least one does, because a clamp proven only at a height where it never had to act has proven nothing about the branch carry 7 is about. `c9.exit-row` reports the resolved track list before and after — something like `"31px 812px 0px 24px"` then `"31px 776px 36px 24px"`, though the numbers are the engine's and will differ; the shape is what is asserted.

**Two readings of a failure.** `c7.popover` failing with *the flip branch did not run* at 560 px is a real defect in the clamp, **unless** the rail is so short at that height that the chip is not at the bottom — read `observed` for the chip's position before reporting. And if Chromium resolves `grid-template-rows` to something other than a space-separated pixel list here, the assertion is wrong, not the pane: record the raw string and move the exit-row judgement to step 17 rather than rewriting the check to pass. That is the no-stub rule applied to the harness itself.

- [ ] **Step 10: Scrollback survives Columns ↔ Canvas, for the sessions both render** · AGENT-PROVABLE

This is **the single load-bearing claim of the whole design** (`group-d-real-browser-carries.md:29-33`) — the reason the attach registry lives at module scope outside React at all. `FakeTerm.open()` records its parent and does nothing else, so no unit suite could tell a preserved buffer from a blank one. Task 21's fix round 2 added the boundary the claim was first stated without: Canvas renders only the *pinned* sessions in scope (`selectors.ts:103-107`), at most `CANVAS_MAX_TILES = 10` of them, and a session dropped by that cap *"is rendered nowhere, so its `releaseAttach` has nothing re-acquiring it and `destroy()` runs at `+GRACE_MS`"* — **correct behaviour**, and the eleventh tile onward is a re-attach, not a preserved buffer (`group-d-real-browser-carries.md:35-42`). So there are two checks, one for each side of that line.

**Buffer contents alone cannot prove preservation, and the check is built around that.** A terminal destroyed and re-created would re-attach, and sandboxd would replay up to 256 KiB of scrollback into it (`backend.go:29-31`) — which would put the same lines back on screen. So preservation is proven by **three independent observers that a re-attach would each contradict**:

1. **The DOM node.** The check writes an expando property onto the pane's `.xterm` element before the flip and reads it back after. The registry re-parents `handle.element` rather than rebuilding it (`Terminal.tsx:117-120`), so the node in the Canvas tile must be the *same object*. A re-created terminal is a new node with no property. This observes; it changes nothing the app reads.
2. **The sockets.** `watchSockets` records every attach the page makes. A preserved session opens **zero** new sockets across the flip and its existing one never closes.
3. **The daemon.** The control plane logs `attach relay ended` at `attach.go:218` every time a relay closes. None may appear during the flip window.

And the buffer read is then what it can honestly be: proof that the re-parented terminal still *renders* its scrollback, rather than a blank box that holds it.

The window waited is **twice `GRACE_MS`**, ten seconds: long enough that a release the flip failed to re-acquire has already destroyed its terminal before the assertions run.

Insert above `await main();`:

```js
const GRACE_MS = 5_000;   // src/api/attachRegistry.ts:20, guarded in step 1 (b)

async function flipTo(h, label) {
  await h.page.click(`[role="group"][aria-label="Layout"] button:has-text("${label}")`);
  await until(`the ${label} layout`, () =>
    h.page.$(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]:has-text("${label}")`));
}
const tagTerm = (h, id) => h.page.$eval(`${paneOf(id)} .xterm`, (el, v) => { el.__e2eTag = v; }, id);
const tagOf = (h, id) => h.page.$eval(`${paneOf(id)} .xterm`, (el) => el.__e2eTag ?? null);
const relayEnds = () => cpLogLines(/attach relay ended/).length;
const liveIds = async () =>
  (await api("GET", "/v1/sessions")).sessions
    .filter((s) => ctx.created.includes(s.id) && s.status !== "exited").map((s) => s.id);

/** Every row the terminal can show, from the top of its scrollback to the bottom, read by
 *  scrolling the way a user does. Leaves the terminal scrolled to the bottom. */
async function readBuffer(h, id) {
  const box = await h.page.$eval(`${paneOf(id)} .xterm`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(box.x, box.y);
  await h.page.mouse.wheel(0, -1_000_000);
  await sleep(250);
  const seen = new Set();
  let previous = "";
  for (let i = 0; i < 3_000; i += 1) {
    const rows = await screen(h.page, id);
    for (const r of rows) seen.add(r);
    const key = rows.join("\n");
    if (key === previous) break;   // the viewport stopped moving: this is the bottom
    previous = key;
    await h.page.mouse.wheel(0, 300);
    await sleep(25);
  }
  return seen;
}

function missingMarks(seen, prefix, n) {
  const missing = [];
  for (let i = 1; i <= n; i += 1) if (!seen.has(`${prefix}${i}`)) missing.push(i);
  return missing;
}

check("c1.flip", "real-browser carry 1: scrollback survives Columns <-> Canvas for a session both render", async (h) => {
  await flipTo(h, "Columns");
  const from = stdoutOf(h, ctx.sh).length;
  await type(h.page, ctx.sh, "seq -f 'FLIP%g' 1 3000");
  await until("FLIP3000", () => stdoutOf(h, ctx.sh).slice(from).includes("FLIP3000\r\n"), 15_000);

  const live = await liveIds();
  for (const id of live) await tagTerm(h, id);
  const socketsBefore = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  const endsBefore = relayEnds();
  const log = [];

  for (const label of ["Canvas", "Columns"]) {
    await flipTo(h, label);
    await sleep(2 * GRACE_MS);
    for (const id of live) {
      const tag = await tagOf(h, id);
      if (tag !== id) throw new Error(`${label}: ${id}'s terminal is a different DOM node (tag ${tag})`);
      const opened = socketsFor(h, id).length - socketsBefore[id];
      if (opened !== 0) throw new Error(`${label}: ${id} opened ${opened} new attach socket(s)`);
      if (socketsFor(h, id).at(-1).closed !== null) throw new Error(`${label}: ${id}'s socket closed`);
    }
    const ends = relayEnds() - endsBefore;
    if (ends !== 0) throw new Error(`${label}: the control plane logged ${ends} relay end(s)`);
    const missing = missingMarks(await readBuffer(h, ctx.sh), "FLIP", 3000);
    if (missing.length > 0) throw new Error(`${label}: ${missing.length} FLIP lines not rendered, first ${missing[0]}`);
    log.push({ layout: label, liveSessions: live.length, newSockets: 0, relayEnds: 0, flipLinesRendered: 3000 });
  }
  return log;
});

check("c1.cap", "real-browser carry 1, the boundary: a session dropped by the Canvas cap is re-attached, not preserved", async (h) => {
  await flipTo(h, "Columns");
  for (let i = 1; i <= 9; i += 1) await createShell(`e2e-cap-${i}`);
  const live = await until("twelve sessions on the branch, all live but one", async () => {
    const ids = await liveIds();
    return ids.length >= 11 && ids.every((id) => socketsFor(h, id).length > 0) ? ids : null;
  }, 60_000);
  for (const id of live) await tagTerm(h, id);
  const socketsBefore = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));

  const flippedAt = Date.now();
  await flipTo(h, "Canvas");
  const tiles = await h.page.$$eval("[data-cv-tile]", (els) => els.map((el) => el.getAttribute("data-cv-tile")));
  if (tiles.length > 10) throw new Error(`Canvas rendered ${tiles.length} tiles; the cap is 10`);
  const dropped = live.filter((id) => !tiles.includes(id));
  const kept = live.filter((id) => tiles.includes(id));
  if (dropped.length === 0) throw new Error("nothing was dropped; the boundary was not exercised");

  await sleep(GRACE_MS + 3_000);
  for (const id of dropped) {
    const closed = socketsFor(h, id).at(-1).closed;
    if (closed === null) throw new Error(`dropped ${id} still has an open socket after the grace`);
    if (closed - flippedAt < GRACE_MS - 250) throw new Error(`dropped ${id} closed ${closed - flippedAt} ms after the flip, inside the grace`);
  }
  for (const id of kept) {
    if (socketsFor(h, id).length !== socketsBefore[id]) throw new Error(`kept ${id} re-attached`);
    if ((await tagOf(h, id)) !== id) throw new Error(`kept ${id} is a different DOM node`);
  }

  await flipTo(h, "Columns");
  for (const id of dropped) {
    await until(`a fresh socket for dropped ${id}`, () => socketsFor(h, id).length === socketsBefore[id] + 1, 15_000);
    if ((await tagOf(h, id)) !== null) throw new Error(`dropped ${id} kept its old DOM node; it should be a fresh terminal`);
  }
  return { live: live.length, tiles: tiles.length, dropped: dropped.length, droppedGraceMs: GRACE_MS };
});
```

```bash
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=c9.exit-row,c1.flip,c1.cap npm run e2e
```

`c9.exit-row` is included because it leaves an exited session on the branch, which makes `c1.cap`'s arithmetic the one written here: the UI session, the shell, the exited shell and nine more is **twelve sessions, eleven live**. Canvas pins new ids until it has ten (`CanvasLayout.tsx:108-120`), so two are dropped — in this run's creation order, the last two shells made. The check asserts only that *at least one live session* was dropped rather than naming which, because the pin order follows `selectPaneSessionIds`'s server order and whatever `c1.flip` already pinned, and a check whose pass depends on its neighbours' order is a check somebody reorders into a false red. Each harness run is a fresh browser context, so no pin survives from a previous run through `persist`.

**What you must see:** three `"ok":true`. `c1.flip`'s `observed` has two entries, one per layout, each with `"newSockets":0,"relayEnds":0,"flipLinesRendered":3000`. `c1.cap`'s has `"tiles":10` and `"dropped"` of 1 or 2.

**If `c1.flip` fails, the message says which observer disagreed, and they mean different things.** *A different DOM node* with no new socket means the registry rebuilt its element without re-attaching — an element that should be moved was re-created, and the scrollback left with it. *Opened N new attach socket(s)* means a release was not re-acquired inside the grace — the design's one load-bearing claim is false in a real engine, which is the most important thing this task could find, and it goes to the orchestrator with the full `observed` before anything else happens. *N FLIP lines not rendered* with every other observer clean means the buffer is intact and the re-parented terminal is not repainting it — a refresh after re-parenting is missing, and xterm's DOM renderer shows a blank viewport over a full buffer. **Do not add a wait to make a red turn green**: every timeout above is already past the grace window, and a longer one would only hide a destroy that happened.

- [ ] **Step 11: A tile resize reflows the terminal and tells the PTY** · AGENT-PROVABLE

Task 21's fix round 1 replaced a window `resize` listener with a `ResizeObserver` on the terminal's host node (`Terminal.tsx:102-130`), which made carry 5 **reachable**: the trigger is now a tile resize, a gesture that can be performed directly (`group-d-real-browser-carries.md:68-80`). Task 14's polyfill is a no-op class, so nothing in jsdom could ever close this, and nothing was faked to pretend otherwise. What the carry asks is three things in a chain — the terminal reflows to the new box, its columns and rows change, and the daemon receives the resize frame — and the check observes all three at three different places: the pixel box in the DOM, the frame leaving the page, and `stty size` answering from inside the PTY.

`attachRegistry` sends a resize frame from `term.onResize`, which xterm fires only when columns or rows actually change (`Terminal.tsx:122-124`), so the frame is itself evidence that the fit reflowed. The drag is done in ten pointer moves rather than one jump, because `CanvasLayout`'s gesture reads pointer deltas and a single teleporting move is not a gesture a user can make.

Insert above `await main();`:

```js
check("c5.resize", "real-browser carry 5: a tile resize reflows the terminal, and the PTY is told", async (h) => {
  await flipTo(h, "Canvas");
  const tile = `[data-cv-tile="${ctx.sh}"]`;
  if ((await h.page.$(tile)) === null) throw new Error("the shell has no Canvas tile; c1.flip should have pinned it");
  const before = await ptySize(h, ctx.sh);
  const framesBefore = socketsFor(h, ctx.sh).flatMap((s) => s.resizes).length;
  const boxBefore = await h.page.$eval(tile, (el) => el.getBoundingClientRect().toJSON());

  const grip = await h.page.$eval(`${tile} .cv-resize`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(grip.x, grip.y);
  await h.page.mouse.down();
  await h.page.mouse.move(grip.x + 240, grip.y + 160, { steps: 10 });
  await h.page.mouse.up();

  const boxAfter = await h.page.$eval(tile, (el) => el.getBoundingClientRect().toJSON());
  if (boxAfter.width <= boxBefore.width || boxAfter.height <= boxBefore.height) {
    throw new Error(`the tile did not grow: ${JSON.stringify({ boxBefore, boxAfter })}`);
  }
  const frame = await until("a resize frame after the drag", () => {
    const all = socketsFor(h, ctx.sh).flatMap((s) => s.resizes);
    return all.length > framesBefore ? all.at(-1) : null;
  }, 5_000);
  const after = await ptySize(h, ctx.sh);
  if (after.cols <= before.cols || after.rows <= before.rows) {
    throw new Error(`the PTY did not grow: ${before.rows}x${before.cols} -> ${after.rows}x${after.cols}`);
  }
  if (frame.cols !== after.cols || frame.rows !== after.rows) {
    throw new Error(`frame ${frame.rows}x${frame.cols} disagrees with the PTY's ${after.rows}x${after.cols}`);
  }
  ctx.resizedTile = { id: ctx.sh, width: boxAfter.width, height: boxAfter.height };
  return { before, after, tilePx: { from: [boxBefore.width, boxBefore.height], to: [boxAfter.width, boxAfter.height] } };
});
```

```bash
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=c1.flip,c5.resize npm run e2e
```

`c1.flip` is included because it is what pins the shell into Canvas; a fresh browser context has no pins until Canvas has been shown once.

**What you must see:** `"ok":true`, with `observed.before` and `observed.after` two different sizes, the second larger in both dimensions. `ctx.resizedTile` is left behind for step 13, which reloads the app and checks the tile kept its size — known defect 6, *"Canvas tile positions not persisted"* (`task-24.md:1035`), observed in a real window for the first time.

**The one failure that would mislead.** *The tile did not grow* with the pointer landing correctly means the drag gesture did not start — `.cv-resize` is `tabIndex={-1}` and pointer-only (`CanvasLayout.tsx:362-368`), so a click that lands on the tile's header instead begins a *move*. Check the grip's coordinates in the thrown message against the tile's box before calling it a defect. *The PTY did not grow* with a grown tile and no new frame is carry 5's failure exactly: the observer fired no fit, or the fit changed nothing.

- [ ] **Step 12: The pinned-scroll rule under real parse, and the flush measured under load** · AGENT-PROVABLE

**Carry 4** is a timing assumption. `FakeTerm.write` calls its callback synchronously; real xterm drains a parse queue first and moves `baseY` on its own schedule. Task 19's *ordering* is proven — a reviewer moved the measurement into the callback and two tests killed it — but the *timing assumption behind the callback* is not (`group-d-real-browser-carries.md:60-63`). `CLAUDE.md` states the rule the timing serves: log panes *"never scroll-jump when the user has scrolled up"*. So the check tests both directions under output that keeps arriving: **scrolled up, the viewport must not move; at the bottom, it must follow.**

**Carry 6** asks whether *"ten writes of a large chunk render acceptably at 4,000 lines/second"* (`:82-84`). "Acceptably" is a judgement, and it is the owner's — item 5 of step 16. What an agent *can* assert is that the application stays usable under that load, and it can **measure** the rest so the owner judges against numbers rather than an impression. The check generates exactly the carry's rate — 400 lines every 100 ms, for ten seconds — and records: how many times per second xterm's rows actually changed; the longest main-thread task; and how long a keystroke typed into a *different* pane took to echo back while the burst ran. **One hard assertion only**: that echo returns within two seconds. A cockpit that cannot echo a keystroke in two seconds is unusable whatever it looks like, and nothing looser is claimed.

The rendered-change rate is recorded, not asserted, and that is deliberate. The attach registry flushes at 10 Hz, but one large write can take xterm several frames to parse, so the DOM may change more than ten times a second while the flush is doing exactly its job. A threshold there would fail on correct code, and `task-24.md:94` names what happens to an audit that fails on correct code.

`awk` is in the image because Debian treats `mawk` as required, which is why the generator is written in it rather than in a language the image may not carry.

Insert above `await main();`:

```js
check("c4.pinned", "real-browser carry 4: the pinned-scroll rule holds against real xterm's asynchronous parse", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "while :; do echo \"TI\"\"CK $RANDOM\"; sleep 0.05; done");
  await until("ticks arriving", () => /TICK \d+/.test(stdoutOf(h, id)), 5_000);
  const centre = await h.page.$eval(`${paneOf(id)} .xterm`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(centre.x, centre.y);

  // Scrolled up: sample the viewport for three seconds of arriving output. It must not move.
  await h.page.mouse.wheel(0, -900);
  await sleep(300);
  const grown = stdoutOf(h, id).length;
  const pinned = await screen(h.page, id);
  for (let i = 0; i < 30; i += 1) {
    await sleep(100);
    const now = await screen(h.page, id);
    if (now.join("\n") !== pinned.join("\n")) throw new Error(`scrolled up, the viewport moved at sample ${i}`);
  }
  if (stdoutOf(h, id).length <= grown) throw new Error("no output arrived while pinned; the check proved nothing");

  // At the bottom: the last tick on screen must keep changing.
  await h.page.mouse.wheel(0, 1_000_000);
  await sleep(300);
  const lastTick = async () => (await screen(h.page, id)).filter((r) => /TICK \d+/.test(r)).at(-1);
  const first = await lastTick();
  await until("the viewport to follow new output", async () => (await lastTick()) !== first, 3_000);

  await h.page.keyboard.press("Control+C");
  return { pinnedSamples: 30, followed: true };
});

check("c6.load", "real-browser carry 6: 4,000 lines/s for 10 s — usability asserted, render rate measured", async (h) => {
  const id = ctx.sh;
  const other = ctx.created.find((x) => x !== id && x !== ctx.claude && socketsFor(h, x).at(-1)?.closed === null);
  if (other === undefined) throw new Error("no second live shell to echo into");

  await h.page.evaluate((sel) => {
    const rows = document.querySelector(`${sel} .xterm-rows`);
    window.__e2eLoad = { mutations: 0, longest: 0, over200: 0 };
    new MutationObserver(() => { window.__e2eLoad.mutations += 1; })
      .observe(rows, { childList: true, subtree: true, characterData: true });
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        window.__e2eLoad.longest = Math.max(window.__e2eLoad.longest, e.duration);
        if (e.duration > 200) window.__e2eLoad.over200 += 1;
      }
    }).observe({ type: "longtask" });
  }, paneOf(id));

  const started = Date.now();
  await type(h.page, id, "awk 'BEGIN{for(i=1;i<=40000;i++){print \"RATE\" i; if(i%400==0) system(\"sleep 0.1\")}}'");
  await sleep(4_000);   // mid-burst

  const echoFrom = stdoutOf(h, other).length;
  const typed = Date.now();
  await type(h.page, other, "echo \"PI\"\"NG\"");
  await until("the second pane's echo during the burst", () => stdoutOf(h, other).slice(echoFrom).includes("PING"), 2_000);
  const echoMs = Date.now() - typed;

  await until("RATE40000 on screen", async () => (await screen(h.page, id)).some((r) => r === "RATE40000"), 60_000);
  const seconds = (Date.now() - started) / 1_000;
  const load = await h.page.evaluate(() => window.__e2eLoad);
  return {
    burstSeconds: Number(seconds.toFixed(1)),
    rowMutationsPerSecond: Number((load.mutations / seconds).toFixed(1)),
    longestTaskMs: Math.round(load.longest),
    tasksOver200ms: load.over200,
    secondPaneEchoMs: echoMs,
  };
});
```

```bash
PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log" PANACEA_E2E_ONLY=c1.cap,c4.pinned,c6.load npm run e2e
```

`c1.cap` is included so `c6.load` has a second live shell to type into.

**What you must see:** `c4.pinned` `"ok":true`. `c6.load` `"ok":true`, and its `observed` — five numbers — goes into the results document verbatim and into step 16 item 5, where the owner watches the same burst with those numbers beside them. `burstSeconds` near 10 means the render kept up with the generator; much more than 10 means output queued behind the renderer, which is a finding for the owner to weigh, not a failure this check invents.

**What a `c4.pinned` failure means.** *Scrolled up, the viewport moved* is carry 4's failure and a `CLAUDE.md` rule broken: the measurement of "was the user at the bottom" is taken on the wrong side of xterm's parse. *The viewport to follow* timing out is the opposite failure — a user at the bottom is left behind by new output — and it is the same timing question answered wrongly in the other direction. Both belong to task 19. *No output arrived while pinned* is a harness failure: the loop did not start, and the sample proved nothing either way.

- [ ] **Step 13: Durability — detach past the grace, reload the app, restart the control plane** · AGENT-PROVABLE

The master plan's goal line promises that *"sessions survive both a detach and an app restart"* (`2026-09-18-phase-2-backend-and-cockpit.md:5`), and its first deliberate refinement is that **disconnect does not destroy** — *"Destroying on disconnect deletes the transcript the product is built around"* (`:60`). Every unit suite proves a piece of that. This step proves the sentence, three ways, each with the transcript as the witness:

1. **Detach.** Selecting another branch unmounts every pane on this one. Past `GRACE_MS` the registry destroys the terminal and closes the socket — correctly, since nothing shows it — and the session must still be running. Returning must open a fresh socket whose replay carries a marker written before the detach.
2. **App restart.** `page.reload()` is the whole cockpit process starting again: the store rehydrates `selection`, `layout`, `tiles` and `pinned` from `persist` (`store.ts:29-42`), the poller finds the sessions, and every live pane re-attaches without a click. Known defect 6's fix is checked here in passing — the tile step 11 resized must come back at its new size.
3. **Control-plane restart.** The Go process is stopped by PID and the same binary started again. The registry must recover every session (task 13's restart recovery), the **containers must be the same containers** — `docker ps` IDs unchanged, because a sandbox is not the control plane's to lose — and the cockpit must re-attach inside the retry ladder's 60-second budget with no user action (`group-d-real-browser-carries.md:141-142`).

The third needs the harness to stop and start the control plane itself, so the down window is measured rather than typed by hand. It does that with the binary step 6 built and **this process's environment** — so the harness must be run with the same `PANACEA_APP_*` variables the control plane was started with, exported in the same shell.

Extend the import block at the top of `desktop/e2e/phase2.e2e.mjs`:

```js
import { execFileSync, spawn } from "node:child_process";
import { openSync, readFileSync } from "node:fs";
import { chromium } from "playwright";
```

(`readFileSync` moves into the combined `node:fs` line; delete the old one.) Then insert above `await main();`:

```js
const CP_BIN = process.env.PANACEA_E2E_CP_BIN ?? "";
let cpPid = Number(process.env.PANACEA_E2E_CP_PID ?? "0");

const healthy = async () => { try { await api("GET", "/v1/healthz"); return true; } catch { return false; } };

async function stopControlPlane() {
  if (CP_BIN === "" || cpPid === 0) throw new Error("PANACEA_E2E_CP_BIN and PANACEA_E2E_CP_PID are required");
  execFileSync("taskkill", ["/PID", String(cpPid), "/F"]);
  await until("the control plane to stop answering", async () => !(await healthy()), 10_000);
}

async function startControlPlane() {
  const log = openSync(CP_LOG, "a");
  const child = spawn(CP_BIN, [], { detached: true, stdio: ["ignore", log, log], env: process.env });
  child.unref();
  cpPid = child.pid;
  await until("the control plane to answer again", healthy, 20_000);
  return cpPid;
}

const containerOf = (sessionId) =>
  execFileSync("docker", ["ps", "-q", "--filter", `label=panacea-app.session-id=${sessionId}`], { encoding: "utf8" }).trim();

check("d.detach", "durability: a pane unmounted past the grace re-attaches, and the replay carries the transcript", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "echo \"DETACH\"\"MARK\"");
  await until("DETACHMARK", () => stdoutOf(h, id).includes("DETACHMARK"));
  const socketsBefore = socketsFor(h, id).length;

  const main = (await api("GET", `/v1/projects/${ctx.projectId}/branches`)).branches.find((b) => b.name === "main");
  await h.page.click(`[data-kind="branch"][data-id="${main.id}"]`);
  await sleep(GRACE_MS + 2_000);
  if (socketsFor(h, id).at(-1).closed === null) throw new Error("the socket outlived the grace with no pane to show it");
  const status = (await api("GET", `/v1/sessions/${id}`)).status;
  if (status === "exited") throw new Error("detaching ended the session; disconnect must not destroy (master plan :60)");

  await h.page.click(`[data-kind="branch"][data-id="${ctx.branchId}"]`);
  const fresh = await until("a new socket on return", () =>
    socketsFor(h, id).length > socketsBefore ? socketsFor(h, id).at(-1) : null, 15_000);
  await until("the replay to carry DETACHMARK", () => fresh.stdout.includes("DETACHMARK"), 10_000);
  if (!(await readBuffer(h, id)).has("DETACHMARK")) throw new Error("replayed, but DETACHMARK is not rendered");
  return { statusWhileDetached: status, replayBytes: fresh.stdout.length };
});

check("d.reload", "durability: the cockpit restarts and every live session re-attaches; known defect 6 in a real window", async (h) => {
  const live = await liveIds();
  const before = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  const layoutBefore = await h.page.$eval(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]`, (b) => b.textContent);
  await h.page.reload();
  // No click: selection, layout, tiles and pinned are persisted (store.ts:29-42).
  for (const id of live) {
    await until(`${id} to re-attach after the reload`, () => socketsFor(h, id).length > before[id], 20_000);
  }
  const layoutAfter = await h.page.$eval(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]`, (b) => b.textContent);
  if (layoutAfter !== layoutBefore) throw new Error(`layout ${layoutBefore} came back as ${layoutAfter}`);
  let tile = null;
  if (ctx.resizedTile !== undefined) {
    await flipTo(h, "Canvas");
    tile = await h.page.$eval(`[data-cv-tile="${ctx.resizedTile.id}"]`, (el) => el.getBoundingClientRect().toJSON());
    if (Math.abs(tile.width - ctx.resizedTile.width) > 1 || Math.abs(tile.height - ctx.resizedTile.height) > 1) {
      throw new Error(`defect 6: the resized tile came back ${tile.width}x${tile.height}, was ${ctx.resizedTile.width}x${ctx.resizedTile.height}`);
    }
    await flipTo(h, layoutBefore);
  }
  return { reattached: live.length, layout: layoutAfter, tileKeptSize: tile !== null };
});

check("d.cp-restart", "durability: the control plane restarts; sessions and containers survive; the cockpit re-attaches unaided", async (h) => {
  const live = await liveIds();
  const containers = Object.fromEntries(live.map((id) => [id, containerOf(id)]));
  const before = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  await stopControlPlane();
  await sleep(10_000);   // well inside the ladder's 60 s budget
  const pid = await startControlPlane();

  const listed = (await api("GET", "/v1/sessions")).sessions;
  for (const id of live) {
    const s = listed.find((x) => x.id === id);
    if (s === undefined) throw new Error(`${id} is missing after the restart`);
    if (s.status === "exited") throw new Error(`${id} came back exited`);
    if (containerOf(id) !== containers[id]) throw new Error(`${id}'s container changed across the restart`);
  }
  for (const id of live) {
    await until(`${id} to re-attach unaided`, () => socketsFor(h, id).length > before[id], 60_000);
  }
  return { sessions: live.length, containersUnchanged: true, newControlPlanePid: pid };
});
```

```bash
export PANACEA_APP_PROJECTS_DIR="$(cygpath -w "$E2E_ROOT/projects")"
export PANACEA_APP_STATE_DIR="$(cygpath -w "$E2E_ROOT/state")"
export PANACEA_APP_SESSION_CPU_MILLIS=250 PANACEA_APP_SESSION_MEMORY_MIB=512 PANACEA_APP_LOG_LEVEL=debug
# Step 6's three, unchanged: the respawn inherits process.env, and a respawn without the
# CA bundle would start sandboxes in which Claude Code cannot reach the API.
export PANACEA_APP_GIT_NAME="Panacea E2E" PANACEA_APP_GIT_EMAIL="e2e@invalid" PANACEA_APP_REAP_INTERVAL=24h
export PANACEA_APP_CA_BUNDLE="<the same value as step 6, or unset>"
export PANACEA_E2E_CP_LOG="$E2E_ROOT/control-plane.log"
export PANACEA_E2E_CP_BIN="$(cygpath -w "$E2E_ROOT/bin/panacea-api.exe")"
export PANACEA_E2E_CP_PID=<the pid step 6 printed>
PANACEA_E2E_ONLY=c1.flip,c5.resize,d.detach,d.reload,d.cp-restart npm run e2e
```

**What you must see:** five `"ok":true`. `d.cp-restart`'s `observed.newControlPlanePid` **replaces** the PID step 6 printed — record it, because step 20 stops the control plane by that number, and a later check that bounces it again replaces it again.

**What each failure means.** `d.detach` throwing *disconnect must not destroy* is the master plan's first refinement broken in a real stack — a regression of the kind the whole phase is organised around, and it goes to the orchestrator first. `d.reload` timing out on one session and not the others usually means that session's pane is off-screen in Columns and was never mounted — check `observed` from `c1.cap` for how many panes the branch holds before calling it a defect. `d.cp-restart` throwing *container changed* means the reaper or restart recovery destroyed a sandbox it should have adopted: task 13's, and serious, because it is the transcript. *Missing after the restart* with an unchanged container means the registry did not survive its own restart — check that `PANACEA_APP_STATE_DIR` is exported in *this* shell, or the respawned control plane opened a different, empty registry and the check is measuring the harness.

- [ ] **Step 14: Retry attach on a genuinely failed attach, Resume end to end, and git on a primary worktree** · AGENT-PROVABLE

**Carry 10.** Every `retryAttach` test drives `FakeWebSocket`, *"which never performs a handshake"* (`group-d-real-browser-carries.md:119-125`). The gesture became performable in `07119d1`: *"fail an attach, open the `⋯` menu, click Retry attach"* (`:127-129`). The word that matters is **genuinely**. A `failed` pane is one whose retry ladder spent its 60-second budget while the session was *not* exited (`group-d-open-carries.md:30-33`), so the only honest way to produce one is to make the control plane unreachable for longer than that budget. The check stops it for as long as it takes the pane to report `failed` — `data-attach-phase="failed"` on the overlay (`SessionPane.tsx:91`) — then starts it again, and only then presses Retry.

**Carry 9's overlay hit-test rides on the same moment.** A failed pane shows the attach overlay in its *corner* presentation (`SessionPane.tsx:85-90`: only `connecting` is centred), and `develop.css:164-169` gives it `pointer-events: none` *"so it can never eat a selection drag in the terminal beneath it"*. `document.elementFromPoint` at the strip's centre answers that mechanically: it must return an element inside `.xterm`, not the strip. **Whether the strip occludes the transcript it sits over is not asked** — that is step 17's.

**Open carry 2 — `resume` has no end-to-end proof** (`group-d-open-carries.md:43`). Planning this task found why: resume could not have worked. The client sent `resume: from.id` (`desktop/src/api/client.ts:358`) — the **cockpit's** session id, `s-` plus eight hex characters (`control-plane/internal/ids/ids.go:17-25`) — to `argv.go:87-89`'s `--resume`, where the Go contract types it as *"a prior agent session id"* (`2026-09-18-phase-2-go-contract.md:1137`). And a resumed session is a **new sandbox** whose only mount was the worktree at `/home/dev/work` (`api/sessions.go:30,331`), so the old sandbox's `~/.claude` — where `claude --resume` reads its state (`docs/FUNCTIONAL-SPEC.md:36`) — was gone with it.

**The Resume fix fixed both, and has landed** (`b4be570`, round 1 `2e3db79`). It mints a UUID per Claude Code session and starts the CLI with `--session-id <uuid>` (which `claude --help` 2.1.274 offers, *"must be a valid UUID"*). It resumes that UUID **as a fork**, `--resume <U> --fork-session --session-id <V>`, because the CLI accepts no other pairing. It persists `/home/dev/.claude/projects` and nothing wider, from `<state>/conversations/local/<worktree>/`, mode 0o700. The desktop still sends the cockpit id, on purpose, and the control plane resolves it (`sessionResolveResume`). And it refuses, with a 409 at the create, to resume a session whose conversation has no transcript (`sessionResumableConversation`). **This check proves that fix works** — step 1 (h) found it at `a38c268`.

**What an agent can prove, and what it cannot.** Context continuing rests on two mechanical facts, and the check asserts both. **Identity**: the source sandbox was started with `--session-id <U>`, U a UUID, and the resumed sandbox with `--resume <U> --fork-session --session-id <V>`: the same U, not the cockpit's id, and V a new UUID. **The store**: the source's transcript `<U>.jsonl` is readable in the resumed sandbox, with the same line count. A session nobody spoke in has no transcript and cannot be resumed at all, so the check first seeds one: `claude -p <nonce> --session-id <U>` in the source sandbox, the way the Resume fix's own proof did (`resume-fix-report.md` §6 step 2). **The transcript is the marker.** It is a real file in the real persisted directory, so the synthetic `.e2e-resume-marker` of the first draft is gone; it proves the directory survives, which is exactly the property the fix adds. What no agent can prove is that the *agent remembers* — that needs a signed-in conversation, and signing in is the owner's (`CLAUDE.md`: users authenticate as themselves). That half is step 18, and the two together are the proof that Resume works.

Insert above `await main();`:

```js
check("c10.retry", "real-browser carry 10 + carry 9's overlay: Retry attach after a genuine failure; the strip lets the pointer through", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "echo \"RETRY\"\"MARK\"");
  await until("RETRYMARK", () => stdoutOf(h, id).includes("RETRYMARK"));
  await tagTerm(h, id);
  const socketsBefore = socketsFor(h, id).length;

  await stopControlPlane();
  const failedAt = await until("the pane to report failed", () =>
    h.page.$(`${paneOf(id)} [data-attach-phase="failed"]`), 120_000, 500);

  const hit = await failedAt.evaluate((strip) => {
    const r = strip.getBoundingClientRect();
    const under = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
    return { corner: strip.classList.contains("pane-attach--corner"),
             inTerminal: under?.closest(".xterm") !== null, onStrip: under?.closest(".pane-attach") !== null };
  });
  if (!hit.corner) throw new Error("a failed pane is not showing the corner presentation");
  if (!hit.inTerminal || hit.onStrip) throw new Error(`the strip takes the pointer: ${JSON.stringify(hit)}`);

  await startControlPlane();
  await sleep(3_000);   // failed is terminal: nothing reconnects on its own (open-carries :30-31)
  if (socketsFor(h, id).length !== socketsBefore) throw new Error("the pane reconnected without Retry; failed was not terminal");

  await h.page.click(`[data-session-menu-anchor="${id}"]`);
  await h.page.getByRole("menuitem", { name: "Retry attach" }).click();
  await until("a new socket after Retry", () => socketsFor(h, id).length > socketsBefore, 15_000);
  await until("the overlay to clear", async () => (await h.page.$(`${paneOf(id)} [data-attach-phase="failed"]`)) === null, 15_000);
  if ((await tagOf(h, id)) !== id) throw new Error("Retry rebuilt the terminal; the transcript did not survive it");
  if (!(await readBuffer(h, id)).has("RETRYMARK")) throw new Error("RETRYMARK is gone after Retry");
  const from = stdoutOf(h, id).length;
  await type(h.page, id, "echo \"AFTER\"\"RETRY\"");
  await until("stdin working on the new socket", () => stdoutOf(h, id).slice(from).includes("AFTERRETRY"));
  return { overlay: hit, sameTerminal: true };
});

/** The persisted part of Claude Code's store inside a sandbox, and ONLY that part: the resume
 *  fix mounts <state>/conversations/local/<worktree>/ here and nothing wider in ~/.claude
 *  (sessionConversationDir, control-plane/internal/api/sessions.go; the Dockerfile's
 *  `RUN mkdir -p /home/dev/.claude/projects`). A transcript is <this>/<cwd-slug>/<uuid>.jsonl,
 *  and the harness runs in /home/dev/work, so the slug is -home-dev-work. */
const CLAUDE_STORE = "/home/dev/.claude/projects";
/** Where the harness runs, so a seeded transcript lands in the same slug as the session's. */
const WORK = "/home/dev/work";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function argvOf(cid) {
  const env = execFileSync("docker", ["inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", cid], { encoding: "utf8" });
  const m = /^PANACEA_APP_ARGS_JSON=(.*)$/m.exec(env);
  if (m === null) throw new Error(`container ${cid} carries no PANACEA_APP_ARGS_JSON`);
  return JSON.parse(m[1]);
}
const flagValue = (argv, flag) => { const i = argv.indexOf(flag); return i === -1 ? null : argv[i + 1]; };
const inSandbox = (cid, script) => execFileSync("docker", ["exec", "-w", WORK, cid, "sh", "-c", script], { encoding: "utf8" }).trim();
/** Runs `script` in the sandbox and returns its REAL exit code and output. The code goes
 *  through a file, never a pipe: `cmd | tail` reports tail's status (the master plan's :52). */
function runInSandbox(cid, script) {
  const tag = `/tmp/e2e-${Date.now()}`;
  inSandbox(cid, `( ${script} ) > ${tag}.out 2>&1; echo $? > ${tag}.rc; true`);
  return { code: Number(inSandbox(cid, `cat ${tag}.rc`)), out: inSandbox(cid, `cat ${tag}.out; rm -f ${tag}.out ${tag}.rc`) };
}
/** `file` may be a glob: sh (dash) does not expand one in a redirection, so it is an argument. */
const lineCount = (cid, file) => Number(inSandbox(cid, `cat ${file} | wc -l`));

check("o2.resume", "open carry 2: Resume forks the same Claude conversation, from its persisted transcript, in a new sandbox", async (h) => {
  const source = ctx.claude;
  const sourceCid = containerOf(source);
  if (sourceCid === "") throw new Error("the source session has no running sandbox to resume from");
  const minted = flagValue(argvOf(sourceCid), "--session-id");
  if (minted === null || !UUID.test(minted)) {
    throw new Error(`the source was not started with --session-id <uuid> (got ${minted}); the resume fix has not landed — step 1 (h)`);
  }

  // Seed a real conversation. A session nobody spoke in has no transcript, and the control plane
  // refuses to resume it (409 "Nothing has been said…", sessionResumableConversation). `-p`
  // writes the user turn before the API call, even when not signed in (resume-fix-report §3), so
  // its exit code is recorded, not asserted: unsigned, it exits 1 "Not logged in". THE MARKER IS
  // THE TRANSCRIPT <U>.jsonl — a real file in the real persisted store, standing in for nothing.
  const nonce = `e2e-${Date.now()}`;
  const seed = runInSandbox(sourceCid, `claude -p ${nonce} --session-id ${minted}`);
  const transcript = `${CLAUDE_STORE}/*/${minted}.jsonl`;
  const seeded = Number(inSandbox(sourceCid, `ls ${transcript} 2>/dev/null | wc -l`));
  if (seeded !== 1) {
    throw new Error(`claude -p did not write ${transcript} (found ${seeded}; exit ${seed.code}: ${seed.out.slice(0, 200)})`);
  }
  if (!inSandbox(sourceCid, `cat ${transcript}`).includes(nonce)) throw new Error(`${minted}.jsonl does not hold ${nonce}`);
  const sourceLines = lineCount(sourceCid, transcript);

  const before = await api("GET", `/v1/sessions/${source}`);
  await api("DELETE", `/v1/sessions/${source}`);
  await until("the source to read exited", async () => (await api("GET", `/v1/sessions/${source}`)).status === "exited");
  await until("the source sandbox to be destroyed", () => containerOf(source) === "", 30_000);
  const known = new Set((await api("GET", "/v1/sessions")).sessions.map((s) => s.id));

  // The user's path: the exited pane's own menu.
  await h.page.click(`[data-session-menu-anchor="${source}"]`);
  await h.page.getByRole("menuitem", { name: "Resume" }).click();
  const resumed = await until("the resumed session", async () =>
    (await api("GET", "/v1/sessions")).sessions.find((s) => !known.has(s.id)), 15_000);
  ctx.created.push(resumed.id);
  const cid = await until("the resumed sandbox", () => containerOf(resumed.id), 30_000);

  // A resume FORKS: --resume <U> --fork-session --session-id <V> (argv.go, claudeArgv). The CLI
  // accepts that pairing and no other, and V names the fork so nothing reads the CLI's output.
  const argv = argvOf(cid);
  const resumedWith = flagValue(argv, "--resume");
  if (resumedWith !== minted) {
    throw new Error(`the resumed sandbox runs --resume ${resumedWith}; the source's Claude session id is ${minted}`);
  }
  if (!argv.includes("--fork-session")) throw new Error(`the resume does not fork: ${JSON.stringify(argv)}`);
  const fork = flagValue(argv, "--session-id");
  if (fork === null || !UUID.test(fork)) throw new Error(`the fork is not named by a UUID --session-id (got ${fork})`);
  if (fork === minted) throw new Error(`the fork reuses the source's id ${minted}; the CLI refuses a --session-id already in use`);
  for (const k of ["harness", "model", "effort", "permissionMode", "worktreeId"]) {
    if (resumed[k] !== before[k]) throw new Error(`resumed ${k} is ${JSON.stringify(resumed[k])}; the source's was ${JSON.stringify(before[k])}`);
  }
  // The store survived: the new sandbox reads <U>.jsonl, and the source is untouched by the fork.
  const readable = runInSandbox(cid, `cat ${transcript} > /dev/null`);
  if (readable.code !== 0) throw new Error(`the resumed sandbox cannot read ${transcript}: exit ${readable.code} ${readable.out}`);
  const resumedLines = lineCount(cid, transcript);
  if (resumedLines !== sourceLines) throw new Error(`${minted}.jsonl has ${resumedLines} lines after the resume; it had ${sourceLines}`);
  // Recorded, not asserted: whether the fork's own transcript exists yet. The interactive CLI
  // writes <V>.jsonl only once something is said, and nobody signed in to say it.
  const forkFiles = Number(inSandbox(cid, `ls ${CLAUDE_STORE}/*/${fork}.jsonl 2>/dev/null | wc -l`));
  return { claudeSessionId: minted, resumedWith, forkedAs: fork, sourceLines, resumedLines,
           seedExit: seed.code, forkTranscriptFiles: forkFiles };
});

/** The identity step 6 exported. The explicit pair wins over the host's git (sandbox fix §4). */
const GIT_NAME = process.env.PANACEA_APP_GIT_NAME ?? "";
const GIT_EMAIL = process.env.PANACEA_APP_GIT_EMAIL ?? "";
const CA_SET = (process.env.PANACEA_APP_CA_BUNDLE ?? "") !== "";

check("g.primary", "sandbox git + CA fix: git works in a session on a primary worktree, as the configured identity", async (h) => {
  if (GIT_NAME === "" || GIT_EMAIL === "") throw new Error("PANACEA_APP_GIT_NAME/EMAIL are not exported in this shell — step 6");
  // ctx.worktreeId is feat/e2e's LINKED worktree; this check needs main's, the clone itself.
  const main = (await api("GET", `/v1/projects/${ctx.projectId}/branches`)).branches.find((b) => b.name === "main");
  const wt = (await api("GET", `/v1/projects/${ctx.projectId}/worktrees?branchId=${main.id}`)).worktrees[0];
  if (wt?.isPrimary !== true) throw new Error(`main's worktree is not primary: ${JSON.stringify(wt)}`);
  const s = await api("POST", "/v1/sessions", { machineId: ctx.machineId, worktreeId: wt.id, harness: "shell", title: "e2e-git" });
  ctx.created.push(s.id);
  const cid = await until("the git session's sandbox", () => containerOf(s.id), 30_000);

  // Preflight (h)'s mechanical signal, in a created sandbox: the fix's names in docker inspect.
  const env = execFileSync("docker", ["inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", cid], { encoding: "utf8" });
  const want = [/^GIT_CONFIG_COUNT=\d+$/m, new RegExp(`^GIT_AUTHOR_NAME=${GIT_NAME}$`, "m"), new RegExp(`^GIT_COMMITTER_EMAIL=${GIT_EMAIL.replace(/\./g, "\\.")}$`, "m")];
  for (const re of want) if (!re.test(env)) throw new Error(`the sandbox's env lacks ${re}`);
  if (/^NODE_EXTRA_CA_CERTS=\/etc\/panacea-app\/ca-bundle\.pem$/m.test(env) !== CA_SET) throw new Error(`NODE_EXTRA_CA_CERTS does not follow PANACEA_APP_CA_BUNDLE (set: ${CA_SET})`);
  if (/^(SSL_CERT_FILE|NODE_TLS_REJECT_UNAUTHORIZED)=/m.test(env)) throw new Error("the sandbox replaces or disables TLS verification");

  // Real exit codes, through a file (runInSandbox), never a pipe.
  const status = runInSandbox(cid, `git -C ${WORK} status`);
  if (status.code !== 0) throw new Error(`git status on the primary worktree exited ${status.code}: ${status.out}`);
  const scope = runInSandbox(cid, `git -C ${WORK} config --show-scope --get-all safe.directory`);
  if (scope.code !== 0 || !/^command\s.*\/home\/dev\/work$/m.test(scope.out)) throw new Error(`safe.directory is not command-scope ${WORK}: ${scope.out}`);
  const commit = runInSandbox(cid, `git -C ${WORK} commit --allow-empty -q -m "e2e: g.primary"`);
  if (commit.code !== 0) throw new Error(`a commit on the primary worktree exited ${commit.code}: ${commit.out}`);
  const who = inSandbox(cid, `git -C ${WORK} log -1 --format='%an <%ae> / %cn <%ce>'`);
  const expect = `${GIT_NAME} <${GIT_EMAIL}> / ${GIT_NAME} <${GIT_EMAIL}>`;
  if (who !== expect) throw new Error(`the commit's author/committer is ${who}; want ${expect}`);
  return { worktree: wt.id, statusExit: status.code, commitExit: commit.code, identity: who, caMounted: CA_SET };
});
```

```bash
PANACEA_E2E_ONLY=c10.retry,o2.resume,g.primary npm run e2e
```

(Run in the same shell as step 13, with its exports still set.)

**What you must see.** `c10.retry` `"ok":true`, with `observed.overlay` reading `{"corner":true,"inTerminal":true,"onStrip":false}`; it takes a minute or more, because the ladder must genuinely spend its budget. `o2.resume` `"ok":true`, with `observed.claudeSessionId` a UUID U, `resumedWith` the same U, `forkedAs` a different UUID V, and `sourceLines` equal to `resumedLines` (the fork left the source untouched). `seedExit` is recorded, not asserted: `1` with *"Not logged in"* is expected, because `-p` writes the user turn before it fails. `forkTranscriptFiles` is recorded, not asserted, and is most likely `0`: the fork's interactive CLI writes `<V>.jsonl` only once something is said, and nobody signed in to say it. Step 18 is where a reply exists. `g.primary` `"ok":true`, with `statusExit: 0`, `commitExit: 0`, and `identity` reading `Panacea E2E <e2e@invalid> / Panacea E2E <e2e@invalid>`. Record every line whole.

**`o2.resume` failures, each naming a different half.** *The source was not started with `--session-id`* means the resume fix is absent — step 1 (h) should already have said so, and this is not a new finding. *`claude -p` did not write `<U>.jsonl`* means the seeding failed: read `seed.out` in the message. If it says the session id is already in use, the source's interactive CLI claimed U first, which is **unverified**: the Resume fix's own proof seeded a sandbox whose interactive CLI had already exited on TLS. That is a harness assumption failing, so report it and do not work around it. *`409 Nothing has been said in that session yet`* from the Resume click means the control plane found no `<U>.jsonl` in the **host** store (`sessionResumableConversation`, `2e3db79`): the seed wrote somewhere other than the persisted mount. *The resumed sandbox runs `--resume X`* with X the cockpit's `s-` id means the control plane did not resolve the cockpit id (`sessionResolveResume`). The desktop sends the cockpit id on purpose, so `client.ts` is not the suspect. *The resume does not fork*, or *the fork reuses the source's id*, means `argv.go`'s `claudeArgv` regressed, and the CLI would refuse that argv outright. *Cannot read `<U>.jsonl`*, or a line count that moved, means identity is fixed and persistence is not. The resumed agent would know which conversation to open and find nothing there, or a damaged one, which is the failure that looks like success. Each goes to the orchestrator against the resume fix, not fixed here.

**`g.primary` failures.** *The sandbox's env lacks …* means the git + CA fix is absent, or step 6's exports did not reach this binary: check `tasklist` against the PID before calling it a defect. *`git status` exited 128* with "dubious ownership" means `safe.directory` did not arrive. With "not a git repository", the worktree is a linked one: the check picked the wrong worktree. *The commit exited 128* with "Author identity unknown" means the identity did not arrive, and `user.useConfigOnly` refused, which is the fix's design working on a missing input. *The author is someone else* means the explicit pair lost to another source, and it goes to the orchestrator against the git fix.

**A further check the Resume fix's round 1 makes possible is recorded, not built** (reconcile item 12, the owner's call on the count): seed `<U>.jsonl`, Restart the source, then Resume it before anything is said in the restarted sandbox. The resumed argv must carry `--resume <U>`, the conversation the restart cleared (`earlierConversationIds`), not the restart's new id. It is not in the nineteen.

**`c10.retry` failures.** *The pane reconnected without Retry* means `failed` is not terminal, which contradicts `group-d-open-carries.md:30-31` and task 22's design; report it with the socket timings. *Retry rebuilt the terminal* is carry 10's failure — the same claim as carry 1, failing on a different path. A `getByRole("menuitem")` that finds nothing means the `⋯` menu's rows are not `role="menuitem"`; read `SessionMenu.tsx` and select by the row's text instead — that is a harness correction, not an app defect.

- [ ] **Step 15: The native window, over the DevTools protocol** · AGENT-PROVABLE

Phase 1's fifth unproven item is *"The Tauri desktop window … no human or agent has seen the native window render a terminal"* (`2026-09-17-phase-1-results.md:68`). Steps 8 to 14 ran in Chromium at the dev window's origin, which proves the cockpit and not the shell it ships in. WebView2 accepts `--remote-debugging-port` through the `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` environment variable, and Playwright's `connectOverCDP` attaches to it, so the agent half of Phase 1's gap can be closed from here: **the native engine mounts a terminal whose PTY reports a non-zero size.** The owner half is step 16.

**This is an attempt, and it may be refused.** Corporate policy already kills an automation-controlled system Edge; it may also refuse a WebView2 debugging port. If `curl` below prints nothing, **record *"DevTools port refused by policy"* in the results document, skip the check, and let step 16 carry the native window alone.** Do not look for a way around the policy.

`tauri dev` runs `npm run dev` itself (`desktop/src-tauri/tauri.conf.json:7`), and Vite's `strictPort` (`vite.config.ts:18`) refuses a second listener on 1420, so the dev server step 6 started must be stopped first — by the PID step 6's `netstat` named, not by image name, because `taskkill //IM node.exe` would take every Node process on the machine with it.

```bash
taskkill //PID <the node pid holding 1420> //F
cd desktop
WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=9222 npm run tauri dev > "$E2E_ROOT/tauri.log" 2>&1 &
# wait for the window, then:
curl -s --max-time 5 http://127.0.0.1:9222/json/version
```

Insert above `await main();`:

```js
check("w.webview2", "phase-1 unproven item 5, agent half: the native WebView2 window mounts a live terminal", async () => {
  if (CDP === "") return "skipped: PANACEA_E2E_CDP is unset";
  const native = await chromium.connectOverCDP(CDP);
  try {
    const pages = native.contexts().flatMap((c) => c.pages());
    const page = pages.find((p) => p.url().startsWith(APP));
    if (page === undefined) throw new Error(`no WebView2 page at ${APP}; saw ${pages.map((p) => p.url())}`);
    const engine = await page.evaluate(() => navigator.userAgent);
    await until("the project in the native rail", () => page.$(`[data-kind="project"][data-id="${ctx.projectId}"]`), 15_000);
    if ((await page.$(paneOf(ctx.sh))) === null) {
      await page.click(`[data-kind="project"][data-id="${ctx.projectId}"]`);
      await page.click(`[data-kind="branch"][data-id="${ctx.branchId}"]`);
    }
    await until("the shell's pane in the native window", () => page.$(paneOf(ctx.sh)), 15_000);
    await page.click(`${paneOf(ctx.sh)} .xterm`);
    await page.keyboard.type("stty size | sed 's/^/SIZE=/'");
    await page.keyboard.press("Enter");
    // Read off the native window's own screen: its sockets opened before this connection,
    // so watchSockets cannot see them. The PTY is shared, so the size is whichever client
    // resized last — only "non-zero" is asserted, which is carry 2's claim.
    const m = await until("stty size in the native window", async () => {
      for (const row of await screen(page, ctx.sh)) {
        const hit = /SIZE=(\d+) (\d+)/.exec(row);
        if (hit !== null) return hit;
      }
      return null;
    }, 10_000);
    if (Number(m[2]) === 0) throw new Error("the native window's PTY reports zero columns");
    return { engine, rows: Number(m[1]), cols: Number(m[2]) };
  } finally {
    await native.close();   // disconnects; does not close the app's window
  }
});
```

```bash
PANACEA_E2E_CDP=http://127.0.0.1:9222 PANACEA_E2E_ONLY=w.webview2 npm run e2e
```

**What you must see:** `curl` prints a JSON object whose `Browser` field names an `Edg/` version — that is WebView2. Then `"ok":true` with `observed.engine` a user-agent string containing `Edg/`, and non-zero `rows` and `cols`. **Leave `tauri dev` running**: step 16 is performed in this window. `native.close()` on a `connectOverCDP` browser disconnects Playwright and leaves the application alone; if the window disappears when the check ends, record it, because that would mean the harness closed the owner's window and step 16 must restart it.

- [ ] **Step 16: The owner checklist — the native window and the terminal** · OWNER-OBSERVED

**The implementer does not perform, pre-fill or summarise any item in steps 16 to 18.** Its job is to leave the app in the state each item needs, hand the owner this list, and copy the owner's answers into the results document word for word. If an item is ambiguous when the owner reads it, the plan is wrong: fix the wording with the owner, and record that it changed.

**State the implementer leaves before handing over.** The stack from step 6 is running; `tauri dev` from step 15 is open (or, if step 15 was refused, started without the environment variable); project `e2e-demo`, branch `feat/e2e` is selected; the layout is **Columns**; the shell `e2e-shell` exists and has run `p1.fidelity`'s commands, so its scrollback holds `REDOK`, `╭─╮` and `L1` to `L500`; and the UI-created Claude Code session is showing its first screen. Paste `c2.cols`' and `c6.load`'s `observed` lines at the top of the owner's copy — items 3 and 5 refer to them.

Each item says **what to do**, **what you should see**, and **what to answer**. Answer every item *yes*, *no* or *unsure*, with a sentence when it is not *yes*.

1. **The window is the cockpit.** *Do:* look at the native window. *You should see:* a narrow rail on the left listing one machine and the project `e2e-demo`, a top bar with a *Columns / Canvas* switch and a `＋ New session` button, and one pane per session to the right. *Answer:* is that what is on screen?
2. **Every pane shows a live terminal, not a blank box.** *Do:* look at each pane in turn. *You should see:* text in every one — a shell prompt in the shells, Claude Code's first screen in the Claude pane. *Answer:* does any pane show an empty or black rectangle?
3. **The terminal is legible** (real-browser carry 3). *Do:* read the last ten lines of `e2e-shell` at your normal distance from the screen. *You should see:* light text on a very dark, faintly warm ground; the `L` numbers and the prompt readable without leaning in. *Answer:* comfortable to read for an hour? The pane is `<cols from c2.cols>` columns wide at this window size — is that enough width for an agent's output?
4. **Colour and box drawing survived the pipe** (Phase 1's fidelity rows). *Do:* scroll `e2e-shell` up until you find `REDOK` and `╭─╮`. *You should see:* `REDOK` in red; `╭─╮` as a joined corner-dash-corner, not question marks, boxes or gaps. *Answer:* both as described?
5. **A burst is acceptable** (real-browser carry 6). *Do:* click into `e2e-shell`, type the command below, press Enter, and watch for the ten seconds it runs; while it runs, click into a *different* shell pane and type `echo hi` then Enter.
   `awk 'BEGIN{for(i=1;i<=40000;i++){print "RATE" i; if(i%400==0) system("sleep 0.1")}}'`
   *You should see:* the output stream for about ten seconds and end on `RATE40000`, and `hi` appear in the other pane promptly. `c6.load` measured this burst at `<rowMutationsPerSecond>` screen updates a second, a longest stall of `<longestTaskMs>` ms and an echo of `<secondPaneEchoMs>` ms. *Answer:* does it feel acceptable — no freezes you noticed, no stutter that would bother you in daily use?
6. **The window resizes cleanly** (real-browser carry 5, the window gesture). *Do:* drag the native window's bottom-right corner to make it much larger, then much smaller. *You should see:* the terminals' text re-wrap to fill their panes each time, with no band of empty space at the right or bottom of a terminal. *Answer:* re-wraps cleanly both ways?
7. **Flipping layout keeps what is on screen** (real-browser carry 1, as the user sees it). *Do:* in `e2e-shell` type `seq 1 2000` and Enter; scroll up a little so `1500`-ish is visible; click **Canvas**; then click **Columns**. *You should see:* the same terminal content in both layouts — at no point a blank terminal, and after returning to Columns the scrollback still there when you scroll. *Answer:* did any terminal go blank, even briefly?

- [ ] **Step 17: The owner checklist — the Develop lens around the terminal** · OWNER-OBSERVED

Real-browser carry 9 is listed separately from carry 3 *"because carry 3 is specifically `themeFromTokens()` resolving `--page`/`--ink` inside xterm — different elements, different tokens, and task 24 has no reason to look at them unless told"* (`group-d-real-browser-carries.md:93-97`). This is the telling. Steps 9 and 14 took the measurable half — the exit row's height, the overlay's hit-testing; these are the half that is only ever the owner's eyes.

**State the implementer leaves:** as step 16, plus at least four live sessions on the branch so a row of panes exists. For item 11 the implementer **acts during the item** and says so aloud: it stops the control plane when the owner is ready and starts it again when the owner has answered.

8. **The focus ring says where the keyboard is** (carry 9). *Do:* click inside the second pane's terminal, then look across the whole row of panes. Type a letter. *You should see:* a thin teal ring around that one pane and no other, and the letter appearing in the ringed pane. *Answer:* from the ring alone, with your eyes on a different pane, could you tell which pane will receive your typing? `develop.css:72` draws it as a 1px ring at 28% alpha; task 20's fix round made the ring the **only** indicator of where the caret is, so a *no* here is a defect, not a preference.
9. **The pane footer is readable** (carry 9). *Do:* read the footer line along the bottom of the Claude Code pane — harness, model, age, attach state. *You should see:* four facts separated by dots, in small monospace. *Answer:* can you read all four without leaning in? It is 11px in the lowest-contrast text colour that still passes 3:1 (`develop.css:252`).
10. **The pane width uses the window well** (carry 9). *Do:* in `e2e-shell` type `claude --help` and Enter; look at how the lines wrap, and at the space to the right of the last pane. *You should see:* help text wrapping at the pane's edge. *Answer:* is the wrapped text comfortable to read, and is the fixed 520px pane width (`develop.css:28`) a good use of this window, or does it waste space or cramp the text?
11. **The reconnect strip does not hide what you need** (carry 9). *Do:* tell the implementer you are ready; it stops the control plane. Within a few seconds each pane shows a small strip in one corner saying why it is reconnecting. Read the transcript behind the strip, then drag to select a line of text that runs underneath it. *You should see:* the strip leaving the transcript readable, and your selection working straight through the strip as if it were not there. *Answer:* did the strip cover anything you needed to read? Did the selection work? (The implementer then restarts the control plane.)
12. **An exited session is calm and costs the layout nothing** (carry 9). *Do:* in any shell pane other than `e2e-shell`, type `exit` and Enter. *You should see:* a neutral bar appear at the bottom of that pane with the exit reason and a Restart button — not red — and none of the *other* panes shift. *Answer:* both true?
13. **One heartbeat, no spinners** (`CLAUDE.md`, frontend rules). *Do:* look at the status dots beside the running sessions in the rail for a few seconds. *You should see:* every live dot pulsing together, in step, and no spinner anywhere in the window. *Answer:* in step, and spinner-free?
14. **A keyboard focus is visible on Canvas** (open carry 3, task 23's). *Do:* click **Canvas**; drag one tile so that it overlaps another; press **Ctrl+2** (⌘2 on macOS). *You should see:* the second session's tile come to the front and take the ring. *Answer:* did something visibly change? Open carry 3 records that it did not when it was found — *"a focus affordance with no visible effect is worse than none"* (`group-d-open-carries.md:47-50`) — and task 23 owns the fix, so a *no* here is reported against task 23, not fixed in this one.

- [ ] **Step 18: The owner checklist — resume restores the agent's context** · OWNER-OBSERVED

`docs/FUNCTIONAL-SPEC.md:36` defines the promise: **Resume** (`claude --resume`, *restores context*). Step 14's `o2.resume` proved the two mechanical halves the resume fix adds — the same Claude session id, and a conversation store that survives into the new sandbox. This step is the half no agent can do: the owner watching the promise itself, because a resumed agent that remembers — or does not — is the only evidence that settles it, and it needs a real sign-in. `CLAUDE.md` locks it that way: *"Users authenticate as themselves"*. **Like `o2.resume`, it depends on the resume fix having landed** (step 1 (h)); without it, item 19 cannot say PAPAYA and the step proves nothing new.

**What the owner should know before starting.** Signing in stores a credential in Claude Code's store inside the sandbox, and the resume fix persists that store across sandboxes so a resumed session can find its conversation. **Whether the credential persists with it, where it lives on this machine, and how it is removed, are the fix's decisions** — the implementer reads its commit and tells the owner all three before item 15. Nothing in this step pools or shares the subscription.

15. **Start a Claude Code session.** *Do:* with `feat/e2e` selected, click `＋ New session`. In the new pane, follow Claude Code's own sign-in with your own subscription. *You should see:* Claude Code's prompt, ready for input. *Answer:* signed in?
16. **Give it something to remember.** *Do:* type `Remember the word PAPAYA. Reply only OK.` and Enter. *You should see:* `OK`. *Answer:* did it reply?
17. **Kill it.** *Do:* open the pane's `⋯` menu and choose **Kill**. *You should see:* the exited bar in that pane. *Answer:* exited?
18. **Resume it.** *Do:* open the same pane's `⋯` menu and choose **Resume**. *You should see:* a new pane for a new session. *Answer:* write down the **first thing** the new pane shows — Claude Code's prompt, a sign-in screen, or an error message, word for word if it is an error.
19. **Ask.** *Do:* if the new pane reached a prompt, type `What word did I ask you to remember?` and Enter. *Answer:* write down its reply exactly.

**Resume working looks like this:** item 18 reaches Claude Code's prompt — directly if the fix persists the credential with the conversation, after a sign-in if it deliberately does not; the fix's commit says which — and item 19 answers PAPAYA. Anything else is a finding against the resume fix, not fixed here. **The owner's answer is the evidence either way**, and the results document records it in the owner's words beside `o2.resume`'s agent result, so a reader sees the mechanical proof and the lived one side by side.

- [ ] **Step 19: Write the results document, with a disposition for every carry** · AGENT-PROVABLE

First, the full run — every check, in order, once — so the document is built from one run and not stitched from partial ones:

```bash
cd desktop
npm run e2e > "$E2E_ROOT/e2e-results.jsonl"; echo "exit=$?"
cat "$E2E_ROOT/e2e-results.jsonl"
```

Redirected rather than piped through `tee`, because of the pipe exit-code trap: `npm run e2e | tee …` reports `tee`'s status, and the status is part of the evidence. **What you must see:** one line per check, `setup` first and `w.webview2` last, every one `"ok":true`, and `exit=0`. (`w.webview2` is `ok` with `observed: "skipped: …"` if step 15's port was refused.) If the resume fix had not landed and the orchestrator chose not to wait, `o2.resume` is `"ok":false` on its `--session-id` assertion, `exit=1`, and the document says so plainly. **Any other red is a finding**: it goes into the document as a failure with the check's `error` verbatim, and to the orchestrator. It is not re-run until green, and it is not fixed here.

Then create `docs/verification/<run-date>-phase-2-results.md`, where `<run-date>` is the day of the run in `YYYY-MM-DD`. It follows `2026-09-17-phase-1-results.md`'s form — a headline sentence, the environment, what was tested, the checks, blockers, false results, what is still unproven, how to reproduce — with **four additions** that Phase 2 needs and Phase 1 did not: the owner-observed items, the findings with owners, the open questions for the owner, and the carry disposition. Results are written `PASS` / `FAIL` / `SKIPPED` / `PENDING OWNER` in words.

**Its headline counts what its table holds.** Phase 1's reads *"9/9"* (`2026-09-17-phase-1-results.md:15-17`) over a check table of twelve rows (`:21-36`). Every `<n>/<n>` in this document is the number of rows in the table it heads, counted after the table is written. **The Phase 1 file is not edited** — it is a dated record of what was claimed on that day — and the discrepancy is noted once, in this document's *Phase 1, re-proven* section. The skeleton, with every `<…>` filled from the run:

```markdown
# Phase 2 verification results — <run-date>

<One sentence, written last: what the phase's defining claim was — scrollback survives a
re-parent, sessions survive detach and restart — and whether it was observed. Then one
sentence naming anything that failed.>

Run on the development machine (Windows) against Docker Desktop <version>, image
`panacea-app/sandbox:dev` created <Created> from commit <source-commit label>, control plane
built from <HEAD>. Browser automation: Playwright <ver>, bundled Chromium <build>, headed.
Native window: WebView2 <Edg/ version from w.webview2, or "DevTools port refused by policy">.

---

## What was tested

| Target | How | Result |
|---|---|---|
| The sandbox image alone | `docker run`, `curl` (steps 2–4) | <n>/<n> |
| The cockpit at the dev-window origin | headed Chromium, `desktop/e2e/phase2.e2e.mjs` | <n>/<n> |
| The native window | WebView2 over the DevTools protocol (step 15) | <PASS / SKIPPED: reason> |
| The owner's eyes | 19 numbered items (steps 16–18) | <n> answered, <n> PENDING OWNER |

## Prerequisites — the two this phase recorded

| Prerequisite | Evidence | Result |
|---|---|---|
| Image newer than `382f056` (2026-09-21T22:43:47Z) | before `2026-09-21T05:10:18Z`, after `<Created>` | PASS |
| Fail-closed startup | tokenless container exits 1 with `sandboxd: no attach token configured; …` | <PASS/FAIL> |
| Token rejection | `/healthz` 200, `/attach` 401 / 401 / 101 | <PASS/FAIL> |
| Harness flags vs 2.1.274 | `--permission-mode` the same six choices; both evidence comments re-cited to the image | <PASS/FAIL> |

## Phase 1, re-proven

<Twelve rows, one per row of 2026-09-17-phase-1-results.md:21-36, each saying where it was
re-proven — image alone (step 4) or through the relay (p1.fidelity, p1.claude) — and the result.>

Phase 1's results headline reads 9/9 over a table of twelve checks; the three container-level rows
appear to have been counted outside its harness. Recorded here rather than corrected there.

Task 5's Q3 and Q4: `docker port` printed <verbatim>; `State` read `<running>` then `<exited>`.

## The checks

| Check | Closes | Result | Observed |
|---|---|---|---|
<one row per JSON line in e2e-results.jsonl, in run order: id, closes, PASS/FAIL, and the
`observed` or `error` value, abbreviated only where it exceeds a line — never paraphrased>

## Owner-observed

<Items 1–19 from steps 16–18, numbered as there, each with the item's one-line title and the
owner's answer in the owner's words. Every item reads PENDING OWNER until the owner answers.
The implementer writes none of these.>

## Findings

<One row per FAIL and per owner "no": what failed, the evidence, and the task or decision that
owns it. A resume failure is owned by the resume fix, not by this task.>

## Open questions for the owner

1. **Should the product offer the `max` effort level?** `claude --help` in the pinned image
   (2.1.274) lists five levels — low, medium, high, xhigh, max. The product offers four, by a
   written decision: docs/FUNCTIONAL-SPEC.md:27, which control-plane/internal/harness/harness.go:37-40
   cites, and two tests pin. Saying yes amends FUNCTIONAL-SPEC.md, and the change is not Go-only:
   SessionPicker.tsx's EFFORT_LABEL is a Record<Effort, string>, so a fifth level added to the
   catalogue alone renders a picker row labelled `undefined`.
<Any further question the run raised for the owner, numbered on.>

## False results caught

<Anything the run first reported wrongly and why — a stale listener, a harness selector, a
check that passed on an echo. Phase 1 recorded two; record every one here, or "none".>

## Carry disposition

<The table below, with its Result column filled.>

## What is still unproven

1. Everything in 2026-09-17-phase-1-results.md:62-67 that needs an AKS cluster — Kata
   isolation, the RuntimeClass name, cold start, the read-only root in practice. Phase 8.
2. Contract §4.7's refresh half: declared as not delivered by task-24.md:28.
3. The CORS grant is task 24 step 11's evidence, relied on here and not re-run.
4. <anything a SKIPPED check leaves open>

## Reproducing this

<Steps 2, 6 and 7's commands, then `npm run e2e`, with the environment step 13 exports.>
```

**The carry disposition table.** It is written here in full so the implementer copies it rather than reconstructs it, and fills only the *Result* column. Every carry in both files appears exactly once. "Deferred" rows name an owner and are not work for this task.

| Carry | Source | Disposition in task 25 | Result |
|---|---|---|---|
| RB 1 — scrollback survives Columns ↔ Canvas | `group-d-real-browser-carries.md:29` | Step 10, `c1.flip` + `c1.cap`; step 16 item 7 | |
| RB 2 — non-zero columns after mount | `:51` | Step 8, `c2.cols`; step 15, `w.webview2` | |
| RB 3 — `themeFromTokens()` real colours | `:56` | Step 8, `c3.theme`; step 16 item 3 (legibility) | |
| RB 4 — pinned scroll vs async parse | `:60` | Step 12, `c4.pinned` | |
| RB 5 — fit reflows; resize reaches the daemon | `:65` | Step 11, `c5.resize`; step 16 item 6 | |
| RB 6 — 10 Hz under real load | `:82` | Step 12, `c6.load` (measured); step 16 item 5 (judged) | |
| RB 7 — `Popover` viewport clamp | `:86` | Step 9, `c7.popover` | |
| RB 8 — no terminal until task 24 mounts | `:89` | **Closed by task 24** step 6; `setup` depends on it | |
| RB 9 — Develop lens geometry, contrast, 11px footer, overlay | `:93` | Step 9 `c9.exit-row`, step 14 overlay hit-test; step 17 items 8–12 | |
| RB 10 — Retry attach on a real socket | `:119` | Step 14, `c10.retry` | |
| OC 1 — failed attach had no way forward | `group-d-open-carries.md:16` | Closed by `07119d1`; its observation is RB 10 | |
| OC 2 — `resume` has no end-to-end proof | `:43` | Step 14 `o2.resume` (identity and store); step 18 (the agent remembers). **Depends on the separate resume fix** — `--session-id <uuid>` at create, resume by it, the conversation store persisted — landing first | |
| OC 3 — Canvas `⌘1-9` does not raise the tile | `:47` | **Owner: task 23.** Step 17 item 14 observes the result | |
| OC 4 — palette exposes no session verbs | `:52` | Correction already made (`80342ee`); nothing to observe | |
| OC 5 — `max` effort (corrected at source: a product decision, not an omission) | `:60` | **Not changed.** Recorded as open question 1 for the owner; step 5a's `--help` reading is its evidence | |
| OC 6 — harness comments cite 2.1.267 | `:73` | Step 5a, 5b | |
| OC 7 — rebuild the sandbox image | `:73` | Step 2, proven by step 3 | |
| OC 8 — `Files` and `Merge` have no owning phase | `:77` | **Deferred to task 26**, which names the seam and the gap; the ownership decision is the product owner's | |
| OC 9 — `pinned` unbounded across branches | `:85` | **Deferred** to whichever task next opens `prefsSlice`; the cheapest prune is where the poller replaces sessions | |
| OC 10 — an unpin does not survive a flip or restart | `:91` | **Deferred** to the next persisted-schema bump (`PERSIST_VERSION`), which can tell *never pinned* from *unpinned* | |
| OC 11 — `observe` drops `dirty` before the write | `:99` | **Deferred to Phase 3**, as recorded | |
| OC 12 — `proxy.Relay` has no production caller | `:102` | **Deferred to task 26**, the end of the phase, which rules on it | |
| Contract §12 open question 1 — `@xterm/xterm@6` under jsdom never observed | `2026-09-18-phase-2-ui-contract.md:2355-2358` | Answered as it prescribes: real xterm is observed in steps 8–15 by Playwright, not by `canvas` in jsdom | |
| Phase 1 unproven item 5 — the native window | `2026-09-17-phase-1-results.md:68` | Step 15 (agent half), step 16 (owner half) | |
| Task 5's Q3 and Q4 | `task-05.md:1457` | Step 4 | |

- [ ] **Step 20: Guards, then commit** · AGENT-PROVABLE

**Stop what this task started, by PID, and prove nothing is left behind.** The control plane's PID is the last one a check reported (`d.cp-restart` or `c10.retry` replace it), not the one step 6 printed.

```bash
taskkill //PID <latest control-plane pid> //F
taskkill //PID <the tauri dev node pid> //T //F           # //T: tauri dev's own Vite goes with it
docker ps -a --filter label=panacea-app.managed=true --format '{{.Names}} {{.State}}'
netstat -ano | grep -E ':(8730|1420|9222)\s' | grep LISTENING
```

**What you must see:** the `docker ps` line prints nothing — the harness `DELETE`d every session it made, and `DELETE` destroys the sandbox. A leftover `panacea-app-*` container means a check threw before `main()` reached its cleanup loop; remove it by name with `docker rm -f`, after `docker inspect` confirms its `panacea-app.session-id` label is one this run created. The `netstat` line prints nothing. `$E2E_ROOT` is left in the temp directory for the owner to inspect and is not deleted by this task.

**Then the guards.** Anchored, per shared shape §3A.

```bash
cd desktop && npm test && npm run typecheck && cd ..
go test ./sandboxd/... ./control-plane/... -race -count=1

# The go directive did not move (master plan :46).
git diff --stat -- go.work sandboxd/go.mod control-plane/go.mod           # expect no output

# The evidence comments cite the image, and the product decision is untouched.
grep -c  '2\.1\.267' control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go      # 0 and 0
grep -c  '2\.1\.274' control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go      # >= 1 each
grep -cE '^\s+EffortMax\b'                              control-plane/internal/harness/harness.go   # 0
git status --porcelain -- docs/FUNCTIONAL-SPEC.md desktop/src/store/types.ts \
  desktop/src/components/topbar/SessionPicker.tsx control-plane/internal/harness/*_test.go  # expect no output

# The harness: exact pin, one script, never collected by vitest, no system-browser channel.
grep -cE '^\s+"playwright": "[0-9]+\.[0-9]+\.[0-9]+",?$'      desktop/package.json                       # 1
grep -cE '^\s+"e2e": "node e2e/phase2\.e2e\.mjs"$'            desktop/package.json                       # 1
grep -cE '^\s+channel:'                                       desktop/e2e/phase2.e2e.mjs                  # 0
grep -cE '^await main\(\);$'                                  desktop/e2e/phase2.e2e.mjs                  # 1
grep -cE '^check\("'                                          desktop/e2e/phase2.e2e.mjs                  # 18

# The results document exists and the implementer answered no owner item.
ls docs/verification/*-phase-2-results.md
grep -c 'PENDING OWNER' docs/verification/*-phase-2-results.md          # 19, unless the owner has answered

git status --porcelain | sort
```

**Eighteen checks**, in file order: `setup`, `p1.fidelity`, `p1.claude`, `c2.cols`, `c3.theme`, `c7.popover`, `c9.exit-row`, `c1.flip`, `c1.cap`, `c5.resize`, `c4.pinned`, `c6.load`, `d.detach`, `d.reload`, `d.cp-restart`, `c10.retry`, `o2.resume`, `w.webview2`. `w.webview2` is written even when step 15's port is refused — it returns `skipped` when `PANACEA_E2E_CDP` is unset — so the count does not depend on the policy.

`git status --porcelain | sort` must list exactly the seven paths of the Files block, the results document among them, and nothing else. A stray path is either an un-reverted edit or a file from the parallel work in this tree — the second is **not** staged.

**Two commits, not one**, because they are two different kinds of change: the first corrects what two comments claim about the code, and is reviewable in thirty seconds; the second is the harness and the evidence. Stage by name — never `git add -A`, `-u` or `.`, which in a tree shared with parallel agents would sweep up their work. **Each message body is written to a file with the file-writing tool and committed with `git commit -F`** — not `-m`, and not a heredoc: a multi-line heredoc through the audited wrapper can terminate early at its extra `bash -c` boundary and run the rest of the message as commands. The two blocks below are the file contents, then the commands.

**The trailer is `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`**, as the master plan's Global Constraints now state.

`$SCRATCH/c1.txt`:

```text
docs(harness): cite the CLI version that actually runs

Both evidence comments said claude --help was read "on this machine, version
2.1.267". The binary that runs is 2.1.274, pinned in images/sandbox/Dockerfile.
Re-read from the rebuilt image: --permission-mode lists the same six choices,
so the ask-means-no-flag mapping stands, and --model and --resume match. Only
the citations move; no code changes.

--effort in 2.1.274 lists five levels and the catalogue offers four. That is a
written product decision (FUNCTIONAL-SPEC.md:27, cited at harness.go:37-40),
so it is left alone and recorded as an open question for the owner in the
phase 2 results document.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

```bash
git add control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go
git commit -F "$SCRATCH/c1.txt"
```

`$SCRATCH/c2.txt`:

```text
test(e2e): prove phase 2 end to end on Docker, in a real window

A headed Playwright harness drives the cockpit at the dev-window origin against a
real control plane and real Docker, and asserts on behaviour only: DOM laid out
by a real engine, the frames on the attach sockets, what the PTY reports about
itself, and what the control plane logs. It takes no screenshot and judges no
appearance; nineteen owner-observed items carry every question of what looks
right, and the results document records them in the owner's words.

The sandbox image is rebuilt and proven to contain 382f056 by its behaviour: a
tokenless sandboxd now refuses to start, and /attach answers 401 without the
bearer token.

<If o2.resume passed: Resume is proven end to end on top of the separate
resume fix - the source sandbox runs with its own Claude session UUID, the
resumed one resumes that UUID, and the conversation store survives into the
new sandbox. Whether the agent remembers is the owner's step-18 observation.
If it failed, say which assertion, and that it is owned by the resume fix.>

<One paragraph per FAIL in the run, naming the check and the owner, or "No
agent-provable check failed.">

<The open question for the owner: whether to offer the max effort level.>

<Say whether w.webview2 ran or the DevTools port was refused.>

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

```bash
git add desktop/package.json desktop/package-lock.json \
        desktop/e2e/phase2.e2e.mjs desktop/e2e/fixture.sh \
        docs/verification/<run-date>-phase-2-results.md
git commit -F "$SCRATCH/c2.txt"
```

Fill the two `<…>` paragraphs from the run before committing; a commit body that still contains `<` is not finished. **Do not write that anything "looks right" in either message**: `task-24.md:1278` gives the reason, and this task has more occasions to break it than any other.

**What is not claimed.** No screenshot was taken. No visual acceptance was performed by an agent: the nineteen owner items stand as `PENDING OWNER` until the owner answers them, and the orchestrator commits those answers separately. What *is* claimed is exactly the list of `PASS` rows in the results document — each one a behaviour observed in a real engine, against a real control plane and a real sandbox, by a script anyone can re-run.
