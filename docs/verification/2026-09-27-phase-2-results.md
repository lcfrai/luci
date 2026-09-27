# Phase 2 verification results — 2026-09-27

**All nineteen checks ran, once, in full: 10 PASS, 9 FAIL, 0 NOT RUN.** `e2e exit=1`, started
`2026-09-27T12:01:32Z`, ended `12:12:21Z` (22:01:32 to 22:12:21 local). This is the first run of
the phase in which no check was skipped. What is now observed end to end: a session created from
the UI, Phase 1's PTY-fidelity rows through the whole path, Claude Code running in that PTY,
columns after mount, real theme colours, the popover clamp including its flip-above branch, a tile
resize reaching the PTY, the cockpit restarting with every session re-attached, the control plane
restarting under a live cockpit, and git in a sandbox on a primary worktree. What is still not
observed: scrollback across a re-parent, the Canvas cap boundary, the exit row, pinned scroll, the
10 Hz flush under load, Retry attach, detach, resume end to end, and a live terminal in the native
window. Nine reds, five product defects (F1-F5), none fixed here, and every red attributed to one
of them.

Run on the development machine (Windows 11) against Docker Desktop 29.4.3, image
`panacea-app/sandbox:dev`
(`sha256:243189316a3e640ae0dee00f0d72bb5174a78ff86bec91be0b840602e5cc2402`) created
`2026-09-23T06:52:05.425529555Z` from commit `dbaf2ad6aed86885096c29de4827e194086f0a88` (its
`panacea-app.source-commit` label). The image was **not** rebuilt, and step 2 proved it need not
be: `git rev-list --count dbaf2ad6..HEAD -- sandboxd images/sandbox` is **0**, the last commit
touching either tree (`b4be5703`) is an ancestor of the label, and both trees are clean. Control
plane built from HEAD `1e4fb7ff` (`go build`, exit 0, a 64,879,104-byte binary); `git diff --stat
502f0751 HEAD -- control-plane sandboxd go.work` is empty, so its Go is `502f0751`'s. Browser
automation: Playwright 1.63.0, bundled Chromium build 1243, headed. Native window: WebView2 reached
over its DevTools port, and `w.webview2` **ran** with `PANACEA_E2E_CDP` set — `connectOverCDP`
attached (`phase2.e2e.mjs:894`) and the project rail (`:900`) and the shell's pane (`:905`) were
found in that window before the check failed at `:912`. That is this run's own, indirect proof that
the port was allowed; **no file in `e2e-run4/` records a `/json/version` read.** The version string
`Edg/153.0.4234.48` belongs to the *earlier* session's step 15 on port 8815
(`task-25-report.md:216`, in the git-ignored SDD workspace — workspace evidence, which will not
survive this branch), and is not quoted here as run 4's.

**The host was quiet, and that is what separates this run from the three before it.** Five
`git config --get --system --includes core.eol` spawns, timed immediately before the run (21:27
local): **1,049 / 1,131 / 1,037 / 623 / 881 ms**. Five more immediately after it, with the stack
still up (22:14): **1,458 / 1,181 / 1,384 / 1,837 / 1,631 ms**. Every one is far under the 4 s
stop-bound the earlier session declared, and run 3's comparable samples were 7.6 to 86.5 s. The
host stayed quiet for the whole run. That matters twice: it is why `setup` passed, and it is what
turns the typed-input symptom the earlier runs carried as an open observation into finding F5.

**Ports.** The plan's own: 8730 (API), 1420 (Vite), 9222 (WebView2 DevTools).
`PANACEA_APP_ALLOWED_ORIGINS` was left **unset**, deliberately: `DefaultOrigins` already carries
`http://localhost:1420` and `http://127.0.0.1:1420`
(`control-plane/internal/httpx/origin.go:46-47`), and setting the variable *replaces* the
defaults. So the harness origin is the dev window's own origin, byte for byte — the deviation run
3 had to explain is gone.

Nothing was fixed in this run, no bound, timeout or assertion was loosened, and no red was re-run
until it went green. Each red below is a result.

---

## What was tested

| Target | How | Result |
|---|---|---|
| The image and this task's premises | anchored greps, `docker run`, `curl`, `openssl` (steps 1–5) | 8/8 PASS (*Prerequisites*) |
| The cockpit at the dev-window origin | headed Chromium, `desktop/e2e/phase2.e2e.mjs` | 10/19 PASS, 9 FAIL, 0 NOT RUN (*The checks*) |
| The native window | WebView2 over the DevTools protocol (step 15), then `w.webview2` | Port allowed (CDP attached, rail and pane found in the native window); `w.webview2` FAIL, at its typing step |
| The owner's eyes | 19 numbered items (steps 16–18), batched to the owner's own session (OD-12) | 0 answered, 19 PENDING OWNER, unverified |

## Prerequisites — the eight this run proved before the harness ran

| Prerequisite | Evidence | Result |
|---|---|---|
| Task 24 landed; the constants; no renderer addon; the theme tokens | step 1 (a)–(d): 0 / 1 / 1 / present; 1, 1, 1, 1, 2; no addon match (exit 1); 1, 1 | PASS |
| Docker up, and 8730 / 1420 / 9222 free | step 1 (g): `29.4.3`; no `LISTENING` line | PASS |
| The image is tied to a commit, and need not be rebuilt | step 2: label `dbaf2ad6aed8…`; `git rev-list --count dbaf2ad6..HEAD -- sandboxd images/sandbox` = **0**; last image-tree commit `b4be5703`, an ancestor of the label; `382f056: in HEAD` | PASS |
| The Resume fix and the git + CA fix landed | step 1 (h): 1 / 1 / 2 / 2 / 1 / 4 / 3, every count at or above expectation | PASS |
| TLS interception, and the CA bundle | step 1 (i): without it `ERROR SELF_SIGNED_CERT_IN_CHAIN`, exit 1; with the re-derived root (`CN=caadmin.netskope.com`, SHA-1 `05:20:B9:…:17:27`, exported by `openssl s_client -showcerts`) `HTTP 404`, exit 0 | PASS |
| Fail-closed startup | step 3, tokenless container exit 1: `sandboxd: no attach token configured; set AttachToken (PANACEA_APP_ATTACH_TOKEN / -attach-token) or pass -allow-unauthenticated to run without one` | PASS |
| Token rejection | step 3: `/healthz` 200, `/attach` 401 / 401 / 101; published `127.0.0.1:51372` only; `authenticated=true allow_unauthenticated=false` | PASS |
| Harness flags vs 2.1.274 | step 5a: `--permission-mode` choices exactly acceptEdits, auto, bypassPermissions, manual, dontAsk, plan; `--session-id <uuid>` *"(must be a valid UUID)"*; step 5b: `harness.go:140`, `:143`, `argv.go:9`, `:92` all cite 2.1.274, and `2.1.267` appears nowhere | PASS |

## Phase 1, re-proven

| Phase 1 row (`2026-09-17-phase-1-results.md:25-36`) | Re-proven where | Result |
|---|---|---|
| Container starts, `/healthz` | image alone, step 3: `200` | PASS |
| `claude --version` as uid 1001 | image alone, step 4: `1001`, `2.1.274 (Claude Code)` | PASS |
| WebSocket upgrade on `/attach` | image alone, step 3: `101` with the bearer | PASS |
| Resize observed by the process | through the relay, `c2.cols`: the PTY reports 54×71, and the last resize frame sent matches it | PASS |
| ANSI colour passes through | through the relay, `p1.fidelity` | PASS |
| Box drawing (UTF-8) intact | through the relay, `p1.fidelity` | PASS |
| Cursor control sequences intact | through the relay, `p1.fidelity` | PASS |
| Exit status propagates | through the relay, `p1.fidelity` | PASS |
| 500-line burst not truncated | through the relay, `p1.fidelity` | PASS |
| Claude Code runs through the PTY | through the relay, `p1.claude` | PASS |
| Claude Code help renders | through the relay, `p1.claude` | PASS |
| Ctrl-C interrupts, shell survives | through the relay, `p1.fidelity` | PASS |

Phase 1's results headline reads 9/9 over a table of twelve checks; the three container-level rows
appear to have been counted outside its harness. Recorded here rather than corrected there.

Task 5's Q3 and Q4 (step 4): `docker port` printed `8080/tcp -> 127.0.0.1:52711`, with no
`0.0.0.0` line and no `[::1]` twin; `State` read `running`, then `exited` after `docker stop`.
## The checks

This document's run: the committed harness at HEAD `1e4fb7ff`, run once in full with
`PANACEA_E2E_CDP` set, from a fresh empty fixture and state directory, no `PANACEA_E2E_ONLY`,
redirected to a file and never piped, so the exit code is the harness's own. `e2e exit=1`. All
nineteen ran, in file order; `setup` is declared `{ always: true }` (`desktop/e2e/phase2.e2e.mjs:215`,
the flag that breaks the loop at `:166`) and it passed,
so nothing was stopped by design.

| Check | Closes | Result | Observed |
|---|---|---|---|
| `setup` | task 24 step 11, re-run: the catalogue loads and a session can be created from the UI | PASS | 24,296 ms: `{"harness":"claude-code","claude":"s-9a9bd81e","shell":"s-7ab0d23a"}` |
| `p1.fidelity` | phase-1 rows: ANSI, box drawing, cursor control, exit status, 500-line burst, Ctrl-C — through the relay | PASS | 2,891 ms: `"six rows, all through browser -> control plane -> sandboxd -> PTY"` |
| `p1.claude` | phase-1 rows: Claude Code runs through the PTY, and its help renders | PASS | 9,725 ms: `firstRows[0]` = `"Welcome to Claude Code v2.1.274"` |
| `c2.cols` | real-browser carry 2: mount after append yields a non-zero column count | PASS | 834 ms: `{"pty":{"rows":54,"cols":71},"resizeFrames":1,"paneBody":{"width":520,"height":820}}` |
| `c3.theme` | real-browser carry 3: themeFromTokens() produces real colours | PASS | 13 ms: `{"page":"10,9,8,255","ink":"242,237,231,255","surfacesPaintedPage":1}` |
| `c7.popover` | real-browser carry 7: Popover's viewport clamp, including the flip-above branch | PASS | 601 ms: three measurements; the third `{"contextMenu":"branch","anchorY":125.375,"height":104.53125,"top":14.46875,"bottom":119,"vh":235,"above":true}` — the flip-above branch ran |
| `c9.exit-row` | real-browser carry 9: the exit row is zero-height until exit, and the pane does not move | FAIL | 21,364 ms: `"timed out after 10000 ms waiting for the exit bar"` — F3 |
| `c1.flip` | real-browser carry 1: scrollback survives Columns <-> Canvas for a session both render | FAIL | 10,975 ms: `"Canvas: s-c9078dff opened 9 new attach socket(s)"` — F3's reconnect storm, false result 6 recurring |
| `c1.cap` | real-browser carry 1, the boundary: a session dropped by the Canvas cap is re-attached, not preserved | FAIL | 108,514 ms: `"kept s-c9078dff re-attached"` — F3 |
| `c5.resize` | real-browser carry 5: a tile resize reflows the terminal, and the PTY is told | PASS | 1,084 ms: `{"before":{"rows":11,"cols":63},"after":{"rows":23,"cols":97},"tilePx":{"from":[460,320],"to":[700,480]}}` |
| `c4.pinned` | real-browser carry 4: the pinned-scroll rule holds against real xterm's asynchronous parse | FAIL | 8,221 ms: `"timed out after 3000 ms waiting for the viewport to follow new output"` — F5 |
| `c6.load` | real-browser carry 6: 4,000 lines/s for 10 s — usability asserted, render rate measured | FAIL | 64,909 ms: `"timed out after 60000 ms waiting for RATE40000 on screen"` — F5 |
| `d.detach` | durability: a pane unmounted past the grace re-attaches, and the replay carries the transcript | FAIL | 10,337 ms: `"timed out after 10000 ms waiting for DETACHMARK"` — F5 |
| `d.reload` | durability: the cockpit restarts and every live session re-attaches; known defect 6 in a real window | PASS | 6,244 ms: `{"reattached":12,"layout":"Columns","tileKeptSize":true,"defect6":{"id":"s-7ab0d23a","width":700,"height":480}}` |
| `d.cp-restart` | durability: the control plane restarts; sessions and containers survive; the cockpit re-attaches unaided | PASS | 61,595 ms: `{"sessions":12,"containersUnchanged":true,"newControlPlanePid":23364}` |
| `c10.retry` | real-browser carry 10 + carry 9's overlay: Retry attach after a genuine failure; the strip lets the pointer through | FAIL | 10,323 ms: `"timed out after 10000 ms waiting for RETRYMARK"` — F5 |
| `o2.resume` | open carry 2: Resume forks the same Claude conversation, from its persisted transcript, in a new sandbox | FAIL | 71,171 ms: `"timed out after 15000 ms waiting for the resumed session"` — F2, on the resume's own create (OO-1) |
| `g.primary` | sandbox git + CA fix: git works in a session on a primary worktree, as the configured identity | PASS | 67,995 ms: `{"worktree":"w-48e94ad6","statusExit":0,"commitExit":0,"identity":"Panacea E2E <e2e@invalid> / Panacea E2E <e2e@invalid>","caMounted":true}` |
| `w.webview2` | phase-1 unproven item 5, agent half: the native WebView2 window mounts a live terminal | FAIL | 20,622 ms: `"timed out after 10000 ms waiting for stty size in the native window"` — F5 |

**Summary: 19 checks, 10 PASS, 9 FAIL, 0 NOT RUN.**

**`d.reload` measured defect 6 rather than skipping into a PASS.** That is what HEAD `1e4fb7ff`
changed, after the round-1 review found the check could return `ok` with `tileKeptSize:false` when
`c5.resize` had not passed (false result 8 below). Its `defect6` field names session `s-7ab0d23a`
at 700×480, and the coupling to `c5.resize` — which passed — held.

**`w.webview2` got further than its verdict says.** The check finds the WebView2 page at the app
origin, waits for the project in the native rail, waits for the session's pane, and only then
types. It timed out at the *typing* step, so the native window had already loaded the cockpit and
mounted a pane. The native engine rendering a pane is observed; the pane carrying live PTY output
is not. Phase 1's unproven item 5 is half closed by measurement, and the other half is not
claimed.

**Why no second run.** The plan runs step 19 once and never re-runs a red until it goes green
(`task-25.md:1584`). Nine reds stand as the result.

## Owner-observed

1. **The window is the cockpit**: PENDING OWNER
2. **Every pane shows a live terminal, not a blank box**: PENDING OWNER
3. **The terminal is legible**: PENDING OWNER
4. **Colour and box drawing survived the pipe**: PENDING OWNER
5. **A burst is acceptable**: PENDING OWNER
6. **The window resizes cleanly**: PENDING OWNER
7. **Flipping layout keeps what is on screen**: PENDING OWNER
8. **The focus ring says where the keyboard is**: PENDING OWNER
9. **The pane footer is readable**: PENDING OWNER
10. **The pane width uses the window well**: PENDING OWNER
11. **The reconnect strip does not hide what you need**: PENDING OWNER
12. **An exited session is calm and costs the layout nothing**: PENDING OWNER
13. **One heartbeat, no spinners**: PENDING OWNER
14. **A keyboard focus is visible on Canvas**: PENDING OWNER
15. **Start a Claude Code session**: PENDING OWNER
16. **Give it something to remember**: PENDING OWNER
17. **Kill it**: PENDING OWNER
18. **Resume it**: PENDING OWNER
19. **Ask**: PENDING OWNER

No agent has performed, pre-filled or summarised one of these, and this document judges no
appearance, legibility or beauty anywhere. Each item's *do* and *you should see* are in
`docs/superpowers/plans/phase-2-parts/task-25.md` steps 16-18. The session's preparation — bringing
the stack up, the state each step needs, and which findings bear on which item — is in the task's
owner checklist in the SDD workspace (`task-25-owner-checklist.md`, git-ignored), which points at
*Reproducing this* below for the stack.
## Findings

Five product defects. F1 to F4 were measured in the earlier runs and the Go suite and are carried
here unchanged; F5 is new, and this run is what made it a finding rather than an observation.
None is fixed here (task 25 reports; the task that owns the file fixes). The control-plane log for
this whole run holds **exactly one** `level=ERROR` line, and it is F2's.

| # | What failed | Evidence | Owner |
|---|---|---|---|
| F1 | **A create the cockpit gives up on is cancelled half-made, and leaks a container.** The cockpit aborts `POST /v1/sessions` at `DEFAULT_TIMEOUT_MS` = 10 s (`desktop/src/api/client.ts:33`); under load a create took about 15 s. The abort cancels the request context, which the create's Docker calls run on. | Measured in the earlier runs, not this one: `dockerbe: start panacea-app-s-12993e6c: … context canceled` (08:09:58); `dockerbe: create panacea-app-s-e19633bd: … context canceled` (08:58:34); the cockpit's own text *"The control plane did not answer POST …/v1/sessions within 10000 ms."*; the record `exited` / *"the sandbox could not be created"* yet still naming `sandboxId s-e19633bd`, and that container `Created` more than an hour later, through a restart's startup sweep: the reaper counts an exited record as tracked (`reaper.go:372-377`) and rules 2 and 3 skip exited records (`:435`, `:459`). **This run: `grep -c "context canceled"` over its control-plane log = 0.** | `control-plane/internal/api/sessions.go` (a create must not run on the caller's context) with the reaper (task 13); the 10 s budget is `client.ts`'s (task 20) |
| F2 | **A timed-out host-git call reads as `exit status 1:` with nothing after it.** `project.GitTimeout` = 10 s (`control-plane/internal/project/gitrunner.go:17`); on Windows the kill is `TerminateProcess(…, 1)`, so the timeout reports as a plain exit 1 with empty output and the cause is never named. | This run fired it **exactly once**, and it is the run's only `level=ERROR` line, verbatim: `time=2026-09-27T22:08:42.762+10:00 level=ERROR msg="api: read how the host's git converts line endings, for worktree w-41581457 failed" method=POST path=/v1/sessions err="read core.eol: git config --get --system --includes --default native core.eol in C:\\Users\\AU-DEV~1\\AppData\\Local\\Temp\\panacea-app-hostgit-1400797516: exit status 1: "` — the bare `exit status 1: ` with nothing after the colon. `grep -c "context deadline exceeded"` = 0. **It lands inside `o2.resume`, the check it broke** — see the attribution below. Earlier evidence, unchanged: 10 such lines in the earlier logs, one `exec: canceling Cmd: TerminateProcess: Access is denied.`, and the Go suite hitting it on unchanged Go (`githostconfig_reap_test.go:122`, `gitglobalfiles_test.go:177`), where **three of four errors read `context deadline exceeded: `** and one `exit status 1: `. Go returns a killed process's own non-zero status (`os/exec/exec.go:261-268`, go1.26.2); `context deadline exceeded` is the other path. So the cause is sometimes named, never with its bound. **Step 20's Go suite, run once at this HEAD on the same quiet host, is green** — exit 0, 19 packages `ok`, `control-plane/cmd/api` 169.3 s — which is consistent with F2 being load-triggered and does not refute it. | `gitrunner.go` (the error must say it timed out, and after how long); the 10 s bound itself is worth re-deciding with it |
| F3 | **sandboxd outlives the process it runs, so an exited session never becomes `exited`.** Every attach then gets an Exit frame and a close *"session ended"* (`sandboxd/internal/server/server.go:151-154`) and the cockpit reconnects, because its ladder stops only on `status === "exited"` (`desktop/src/api/attachRegistry.ts:503`). | **This run's largest single cause: it owns three of the nine failures.** Counted over the run's own control-plane log: **634** `attach relay ended` lines out of the **646** the control plane logged in an 11-minute run — **98.1%**. The **594** `session ended` lines are not a second population: `session ended` is the close reason carried *inside* a relay-end line, and every one of the 594 is one of the 634 (`grep "session ended" | grep -vc "attach relay ended"` = 0). The two counts do not add. `c9.exit-row` never saw an exit bar; `c1.flip` and `c1.cap` both name the same session `s-c9078dff` and both failed on the storm's sockets, not on the property under test. Earlier evidence, unchanged: a shell that ran `exit` still showed only `sandboxd` (PID 1) 26 min later, its container `Up`, `GET /v1/sessions` `running`, with 312 relay-end lines in 7 min. `sandboxd/cmd/sandboxd/main.go:66` serves until killed; the reaper's rule 3 (Go contract §5.3, with §2.2's `StateStopped`) assumes the container stops when the process does. | sandboxd (task 5), against the reaper's rule 3 (task 13) |
| F4 | **A failed `rev-parse` silently drops the checked-out branch's marker.** `Scanner.Branches` tolerates *every* failure of `git rev-parse --abbrev-ref HEAD` (`control-plane/internal/project/project.go:245-250`), though its comment gives one reason, a repository with no commits. So a timed-out rev-parse (F2's bound) returns the list with no branch `isHead`, and no error and no log line say why. | Found by the Go suite, not the harness: `TestScannerAgainstARealRepositoryAndARealLinkedWorktree` failed (41.43 s) with `gitrunner_test.go:166: main is not marked IsHead in the primary worktree's repository`, on a repository that has commits. That the rev-parse timed out is inferred, not observed: its error is thrown away, which is the defect. **This run neither confirmed nor refuted it**: branch enumeration worked throughout (`setup` resolved a branch, `g.primary` found `main`'s primary worktree, `d.reload` re-attached 12 sessions), so no marker was seen to drop. The round-1 review re-ran that package and it passed, and step 20's Go suite at this HEAD passed it again (`control-plane/internal/project` `ok`, 51.7 s, on the quiet host), so **F4 is observed once and has not been reproduced**; neither green refutes it, because the marker is dropped only when the rev-parse fails. | `project.go` (the project enumeration, `44d6b27c`): tolerate only an unborn HEAD; return or log anything else |
| F5 | **Typed input draws no output, on a host that is not loaded.** Five checks failed waiting for output that never arrived, each with its own marker and its own bound: `c4.pinned` (*"the viewport to follow new output"*, 3,000 ms), `c6.load` (*"RATE40000 on screen"*, 60,000 ms), `d.detach` (*"DETACHMARK"*, 10,000 ms), `c10.retry` (*"RETRYMARK"*, 10,000 ms) and `w.webview2` (*"stty size in the native window"*, 10,000 ms). | **Measured**: typed output *worked* earlier in the same run — `p1.fidelity` drove six PTY-fidelity rows through the whole path, `p1.claude` read `Welcome to Claude Code v2.1.274` off the screen, `c2.cols` read a `stty size` of 54×71, and `c9.exit-row` waited successfully for a fresh shell's prompt — and then stopped working. **Measured**: the first failure of the class in this run is `c4.pinned` — but not *immediately* after `c1.cap`: `c5.resize` sits between them and PASSED in 1,084 ms. The two runs agree on the boundary and not on the ordering. In run 1 the first of the class was `c5.resize` itself, the very next check after `c1.cap` (`e2e-run4/run1-results.jsonl` — that run's own results, copied into this run's evidence directory because the scratch they were written to will not survive: `c1.cap` `ok:true` in 178,645 ms, then `c5.resize` `ok:false`, *"timed out after 10000 ms waiting for stty size from s-470f5bbe"*). So in both runs the class begins within two checks of `c1.cap` putting eleven sessions on one branch. **What that does not say**: "no member of the class fails before `c1.cap`" is true by construction in this run, because the class is *defined* by which checks failed after it. Run 1 holds an earlier red of the same outward shape — `p1.claude`, 47,982 ms, *"timed out after 45000 ms waiting for claude's first screen to draw"* — which the class excludes only on its "typed input" qualifier, nothing having been typed into that one. The two runs agree on where the class *starts*; they are not evidence that nothing of its shape happened earlier. **Measured**: F3's reconnect storm runs throughout, from `c9.exit-row` onward. **Measured**: the host was quiet before and after (1.0–1.9 s per git spawn), which is what run 3's document named as the one thing that would settle it. **Not asserted**: the mechanism. One hypothesis was checked and discarded — that `c9.exit-row` killed the shell the later checks type into — because `c9.exit-row` creates its own shell (`createShell("e2e-exit")`, `phase2.e2e.mjs:369`) while the later checks use `ctx.sh` = `s-7ab0d23a`. | **Unassigned.** Candidates: `desktop/src/api/attachRegistry.ts` (the attach ladder under many concurrent sessions), the control plane's relay, or sandboxd (F3's owner), since the storm is the loudest thing happening when it starts. The cheapest next diagnostic is one short run: `PANACEA_E2E_ONLY=p1.fidelity,c1.cap,c4.pinned` — does `c4.pinned` fail with `c1.cap` in front of it and pass without it? |

**F2's one line is inside `o2.resume`, and it cost this run the phase's only end-to-end proof of
Resume.** Four anchors in this run's own files put it there, each re-read against the raw log:

1. **That `POST /v1/sessions` *is* the resume's create, and that it made nothing.** Its worktree
   `w-41581457` is the source session `s-9a9bd81e`'s — but that alone identifies little: **12 of
   the run's 13 sessions carry that worktree** (`e2e-run4/sessions-after.json`), so it narrows the
   create to the run, not to a check. What does narrow it is that **no session exists at that
   instant**: the newest create before the error is `s-d1c31bac` at 12:04:42Z and the next is
   `s-a93b83d2` at 12:09:14Z — `g.primary`'s, on a *different* worktree `w-48e94ad6`. Nothing was
   created at 12:08:42Z, which is the run's own proof that the 500 answered a create that made
   nothing. Its site is
   `control-plane/internal/api/sessions.go:883-886` — `a.internal(w, r, "read how the host's git
   converts line endings, …", err)` then `return`, and `a.internal` writes
   `http.StatusInternalServerError` (`api.go:442-447`), so the request answered **500 and no
   session was created**. The UI's Resume is that same route: `resumeSession` calls
   `createSession({…, resume: from.id})` on `from.worktreeId` (`desktop/src/api/client.ts:345-361`).
2. **The restart it was blamed on is 95 s earlier.** `e2e-run4/control-plane.log:353`, 22:07:07.026,
   is the **only** `control plane listening` line after start-up (21:51:00.870), and its process is
   `d.cp-restart`'s own `newControlPlanePid` 23364. That check ran 61,595 ms, so even the widest
   window the line permits, 22:06:05–22:08:09, excludes 22:08:42.
3. **22:08:27–29 is `o2.resume` destroying its source.** `control-plane.log:460` and `:464` are
   `attach failed: upstream dial` for `s-9a9bd81e`, the second reading `dockerbe: sandbox
   s-9a9bd81e is stopped with endpoint "": backend: sandbox is not running`. `s-9a9bd81e` is
   `setup`'s Claude session, and the only check that `DELETE`s it and waits for its sandbox to go
   is `o2.resume` (`phase2.e2e.mjs:818-820`).
4. **The arithmetic closes, to about ten seconds.** `o2.resume` throws at `until("the resumed
   session", …, 15_000)` (`phase2.e2e.mjs:826-827`), 15 s after the `Resume` click at `:825` —
   and a Playwright click resolves on event dispatch, not when the create it triggers answers, so
   the create's own 10 s host-git bound runs *inside* that 15 s. The check therefore threw at
   **≈22:08:48–22:08:58**, depending on how much of the 15 s the create consumed, and started
   ≈22:07:37–22:07:47 once its own 71,171 ms is subtracted. Every value in that range leaves room
   for `c10.retry`'s 10,323 ms after `d.cp-restart`, and every one is consistent with
   `g.primary`'s session appearing at 12:09:14Z (`sessions-after.json`). These two instants are
   *derived*; the only clock value in this anchor is 22:08:42.762.

**What that means.** F2 did not cost nothing. One flaky host-git read — `git config --get
--system --includes --default native core.eol`, on a host measured quiet before and after — refused
the resume create with a bare `exit status 1: `, and with it went **the only end-to-end proof of
Resume this phase has**: open carry OC 2 and seam SX.10 in `docs/SEAMS.md` both rest on
`o2.resume`, and both stay unproven. It does not change the verdict: `o2.resume` is still FAILED,
the fork was still never reached, and the mechanical half credited below — minted UUID, seeded
transcript, source destroyed — is still measured, because anchor 3 *is* that destroy. The fix is to
attribute, not to re-run.

**That the read timed out is inferred, not observed** — exactly as F4's rev-parse is, and said
here because F2's own definition is that in this form the cause is never named. What the log holds
is `exit status 1: ` with nothing after the colon. Two things support the inference and neither
measures it: that same command, run on this host on 2026-09-28, prints `native` and exits 0, so an
exit 1 with empty output is not the missing-key path; and of the four errors of this shape the Go
suite produced, three read `context deadline exceeded: ` and one reads `exit status 1: `. What is
measured is the 500 and where it landed.

**A cosmetic expectation of the plan was not met, and is recorded rather than fixed.** Step 6 says
the `control plane listening` line prints the effective origin allowlist. It does not: the line
carries `addr`, `backend`, `instance`, `sessions`, `conversations`, `projects`, `image` and
`caBundle`, and `grep -i origin` over the whole log finds nothing. The allowlist was confirmed from
source (`control-plane/internal/httpx/origin.go:41-49`) instead.

## Open observations

Three reds from the diagnostic run had no disposition when the round-1 review read this document.
They are carried here as OO-1, OO-2 and OO-3, each with its verbatim error, and each is now given
what this run's evidence does to it.

| # | The observation, verbatim from the diagnostic run | What this run does to it |
|---|---|---|
| OO-1 | `o2.resume` got through every mechanical step — the `--session-id` minted, the transcript seeded, the source session `DELETE`d and its sandbox destroyed (`phase2.e2e.mjs:794-822`) — and then failed in the resume UI: `locator.click: Timeout 30000ms exceeded.` / `Call log:` / `- waiting for getByRole('menuitem', { name: 'Resume' })`, after clicking the exited pane's `⋯` anchor. | **Explained and closed, and a new failure takes its place.** In this run the same click succeeded: the failure is thrown 15 s *later*, by `until("the resumed session", …, 15_000)` at `phase2.e2e.mjs:826`, which is only reached after `getByRole('menuitem', { name: 'Resume' }).click()` at `:825` resolves. So the `⋯` menu opened, the `Resume` item existed and was clickable. `o2.resume` still **FAILED**, at the next step: the click produced no new session within 15 s. **That failure is F2's**, not F1's (0 `context canceled` lines): the run's one `level=ERROR` line, 22:08:42.762, is a 500 on this check's own resume create — `POST /v1/sessions` for `w-41581457`, `s-9a9bd81e`'s worktree — refused by a host-git read that failed without saying why. The four anchors are in the findings section above. So OO-1 closes too: the red is attributed, and the check that carries OC 2 and seam SX.10 was lost to one `git config` call that failed without saying why — F2, whose 10 s bound is inferred, not observed. |
| OO-2 | `c7.popover`: `timed out after 10000 ms waiting for the context menu`. | **Explained and closed.** `c7.popover` PASSED here in 601 ms, and its third measurement is the context menu itself — `{"contextMenu":"branch",…,"above":true}` — so the menu opened and the flip-above branch ran. |
| OO-3 | `c5.resize`: `the tile did not grow: {"boxBefore":{"x":950,"y":7.5104169845581055,"width":460,"height":320,"top":7.5104169845581055,"right":1410,"bottom":327.5104169845581,"left":950},"boxAfter":{"x":950,"y":7.5104169845581055,"width":460,"height":320,"top":7.5104169845581055,"right":1410,"bottom":327.5104169845581,"left":950}}` — byte-identical before and after the drag. | **Explained and closed.** `c5.resize` PASSED here in 1,084 ms: the tile went 460×320 → 700×480 and the PTY was told, 11×63 → 23×97. |

**The shared-cause candidate is discharged.** The round-1 review offered one candidate, not a
finding: that the `⋯` menu never opened at all, since
`desktop/src/components/develop/sessionVerbs.ts` always renders a `Resume`
item — disabled, never absent — so Playwright's *"waiting for getByRole"* means the element was
never there. This run contradicts it in both places at once: the context menu opened for
`c7.popover`, and the `⋯` menu's `Resume` item was found and clicked for `o2.resume`. The
candidate is recorded as discharged, and no finding was ever raised on it.
## Earlier runs the same day — on record, not this document's result

Listed so no red is hidden and no green is borrowed. **None of these lines is claimed above**: every
PASS in *The checks* is this run's own.

- **Run 1**, harness `953d14db` (before its four premise fixes), a state directory polluted by
  probes. Exit 1. PASS: `setup` (a false pass, false result 1), `p1.fidelity`, `c2.cols` (PTY
  53×71, one resize frame), `c3.theme`, `c1.cap`, `d.reload` (11 re-attached), `d.cp-restart`
  (11 sessions, containers unchanged), `g.primary` (status 0, commit 0, identity `Panacea E2E
  <e2e@invalid>`, CA mounted); `w.webview2` skipped. FAIL: `p1.claude`, `c7.popover`,
  `c9.exit-row`, `c1.flip`, `c5.resize`, `c4.pinned`, `c6.load`, `d.detach`, `c10.retry`,
  `o2.resume` — three on harness premises since fixed (`c7`, `c9`, `c1.flip`), two downstream of
  F1 (`p1.claude`, `o2.resume`), and five on typed input that drew no output (`c5`, `c4`, `c6`,
  `d.detach`, `c10`), which is F5's symptom on a loaded host.
- **Run 2**, harness `83660ac2`, fresh state. Exit 1: `setup` FAIL, 22,460 ms, *"timed out after
  10000 ms waiting for the UI-created session to reach the control plane"*. **Both 10 s bounds
  were in play and both are named**: the harness's own wait for the record expired, which is F1's
  shape, while the control plane logged that create's host-git read killed at 08:54:21 with F2's
  bare `exit status 1:` and no output. Which fired first is not determinable from the record.
- **A diagnostic variant** (a scratch copy of the harness that substitutes a REST create when the
  UI create fails; **not** the committed harness). Exit 1. The UI create itself failed (F1), so its
  `setup` is no proof. PASS: `p1.fidelity`, `p1.claude` (`Welcome to Claude Code v2.1.274`),
  `c2.cols`, `c3.theme`, `d.reload`, `d.cp-restart`. FAIL, eleven: `c9.exit-row` and `c1.flip`
  (F3), `c1.cap` and `g.primary` (F2), `c4`, `c6`, `d.detach`, `c10` (F5's symptom), `c5.resize`
  (OO-3), `c7.popover` (OO-2), and `o2.resume` (OO-1).
- **Run 3**, harness `83660ac2`, on a host saturated by other work (five git spawns 7.6-86.5 s).
  Exit 1, one line printed: `setup` FAIL, 22,024 ms, `GET /v1/projects/p-b9ffb3be/branches -> 500`,
  the log reading `git for-each-ref --format=%(refname:short) refs/heads in …\e2e-demo: exit status
  1: ` — F2. `setup` is `always`, so the other eighteen checks never ran. That document's run is
  the one this document replaces.

## Open questions for the owner

1. **Should the product offer the `max` effort level?** `claude --help` in the pinned image
   (2.1.274) lists five levels — low, medium, high, xhigh, max. The product offers four, by a
   written decision: docs/FUNCTIONAL-SPEC.md:27, which control-plane/internal/harness/harness.go:37-40
   cites, and two tests pin. Saying yes amends FUNCTIONAL-SPEC.md, and the change is not Go-only:
   SessionPicker.tsx's EFFORT_LABEL is a Record<Effort, string>, so a fifth level added to the
   catalogue alone renders a picker row labelled `undefined`.

This run raised no further question for the owner: findings F1 to F5 are engineering defects with
a root-cause fix each, not product choices.

## False results caught

1. **A setup that passed on a dead session** (run 1). The UI-created record appeared, so `setup`
   passed, and its sandbox then failed to start (F1). `p1.claude` and `o2.resume` failed for a
   reason `setup` had already seen. The harness now requires the session to leave `starting`
   alive (`83660ac2`).
2. **"2985 FLIP lines not rendered"** (run 1, `c1.flip`). One wheel event moves xterm five rows
   whatever its delta (measured: rows W349→W344→W339 for −1,000,000 and −300 alike), so the one
   "wheel to the top" read only the visible rows. The DOM-node, socket and relay-end observers
   were all clean. Harness premise, fixed in `83660ac2`.
3. **"third track … while running"** (run 1, `c9.exit-row`). The plan read the pane's third grid
   track as the exit bar's, `0px` while running. `.pane-exit` is rendered only on exit, and on a
   linked worktree the third child is the git note (tracks `38px 809.76px 48px 25.3958px`).
   Harness premise, fixed in `83660ac2` without changing what carry 9 asks.
4. **"the flip branch did not run"** (run 1, `c7.popover`). `.rail-machine` is the rail's first
   row (top 0), so a menu anchored to it can never flip. Harness premise, fixed in `83660ac2` —
   and this run measured the flip-above branch actually taking, `above:true` at `vh:235`.
5. **"dropped 7 of 11"** (run 1, `c1.cap`). The state directory held earlier probes' exited
   records on the same branch, and Canvas pinned them first. Every later run, this one included,
   used a fresh empty state directory.
6. **"opened 10 new attach socket(s)"** (the diagnostic run, `c1.flip`). This was F3's reconnect
   storm on an exited shell, not the flip. **It recurred in this run**, as *"opened 9 new attach
   socket(s)"*, for the same reason: `c1.flip`'s red is F3's, and carry 1 is unobserved either
   way, not refuted.
7. **A PID that did not hold the port** (step 6). `/proc/$!/winpid` named `nohup.exe`, not the
   listener; every PID in this document is read from `netstat` on the port instead.
8. **A `d.reload` that could PASS without checking defect 6.** Found by the round-1 review, not by
   a run: `ctx.resizedTile` is set only when `c5.resize` fully passes, so when `c5.resize` failed
   or was filtered out the defect-6 block was skipped and the check still returned `ok` with
   `tileKeptSize:false` — a field that reads as "the tile lost its size" but meant "not checked".
   It happened twice: run 1 and the diagnostic run both printed `"ok":true … "tileKeptSize":false`
   with `c5.resize` failed. Fixed at HEAD `1e4fb7ff`: `d.reload` now measures defect 6 or fails.
   In this run it measured it, and `defect6` names `s-7ab0d23a` at 700×480.
## Carry disposition

The table is task 25's (`task-25.md:1696-1723` — header `:1696`, 26 rows `:1698-1723`), copied;
only *Result* is filled, from this run.

| Carry | Source | Disposition in task 25 | Result |
|---|---|---|---|
| RB 1 — scrollback survives Columns ↔ Canvas | `group-d-real-browser-carries.md:29` | Step 10, `c1.flip` + `c1.cap`; step 16 item 7 | **Not observed either way**: `c1.flip` and `c1.cap` both FAILED on F3's reconnect storm, before reaching the property; item 7 PENDING OWNER |
| RB 2 — non-zero columns after mount | `:51` | Step 8, `c2.cols`; step 15, `w.webview2` | PASS (`c2.cols`, 54×71, the last resize frame matching); `w.webview2` FAIL (F5) |
| RB 3 — `themeFromTokens()` real colours | `:56` | Step 8, `c3.theme`; step 16 item 3 (legibility) | PASS (`c3.theme`, `--page` painted `10,9,8,255`); item 3 PENDING OWNER |
| RB 4 — pinned scroll vs async parse | `:60` | Step 12, `c4.pinned` | FAIL (`c4.pinned`, F5) |
| RB 5 — fit reflows; resize reaches the daemon | `:65` | Step 11, `c5.resize`; step 16 item 6 | PASS (`c5.resize`, 11×63 → 23×97 for a tile 460×320 → 700×480); item 6 PENDING OWNER |
| RB 6 — 10 Hz under real load | `:82` | Step 12, `c6.load` (measured); step 16 item 5 (judged) | FAIL (`c6.load`, F5: the load never reached the screen, so no rate was measured); item 5 PENDING OWNER |
| RB 7 — `Popover` viewport clamp | `:86` | Step 9, `c7.popover` | PASS (`c7.popover`, including the flip-above branch, `above:true` at `vh:235`) |
| RB 8 — no terminal until task 24 mounts | `:89` | **Closed by task 24** step 6; `setup` depends on it | Closed by task 24; consistent here — `setup` passed and `p1.fidelity` read xterm's own DOM spans |
| RB 9 — Develop lens geometry, contrast, 11px footer, overlay | `:93` | Step 9 `c9.exit-row`, step 14 overlay hit-test; step 17 items 8–12 | FAIL (`c9.exit-row`, F3); the overlay hit-test was never reached (`c10.retry`, F5); items 8–12 PENDING OWNER; F3 bears on item 12 |
| RB 10 — Retry attach on a real socket | `:119` | Step 14, `c10.retry` | FAIL (`c10.retry`, F5) |
| OC 1 — failed attach had no way forward | `group-d-open-carries.md:16` | Closed by `07119d1`; its observation is RB 10 | Closed by `07119d1`; its observation (RB 10) FAIL (F5) |
| OC 2 — `resume` has no end-to-end proof | `:43` | Step 14 `o2.resume` (identity, the fork, and the persisted transcript); step 18 (the agent remembers; PENDING OWNER per OD-12) | **Still unproven end to end.** `o2.resume` FAILED, and its cause is F2: a host-git read answered the resume's own `POST /v1/sessions` with a 500 (OO-1), and that the read hit its 10 s bound is inferred from the bare `exit status 1: `, not observed. What its earlier assertions did measure before the failure: the source sandbox ran with a UUID `--session-id`, `claude -p` wrote that UUID's `.jsonl` into the persisted store and it held the nonce, and the source was `DELETE`d, read `exited` and had its sandbox destroyed. The fork itself — `--resume <U> --fork-session --session-id <V>` in the new sandbox, and the store readable there — was **never reached**. Step 18 PENDING OWNER |
| Sandbox git + CA fix — git and Claude Code in a sandbox on this network | `sandbox-git-fix-report.md` | Step 1 (h) and (i); step 14 `g.primary` | PASS: step 1 (h) and (i), and `g.primary` (status 0, commit 0, committed as `Panacea E2E <e2e@invalid>`, CA mounted) |
| OC 3 — Canvas `⌘1-9` does not raise the tile | `:47` | **Owner: task 23.** Step 17 item 14 observes the result | PENDING OWNER (item 14) |
| OC 4 — palette exposes no session verbs | `:52` | Correction already made (`80342ee`); nothing to observe | Closed by `80342ee`; nothing observed here |
| OC 5 — `max` effort (corrected at source: a product decision, not an omission) | `:60` | **Not changed.** Recorded as open question 1 for the owner; step 5a's `--help` reading is its evidence | Not changed; open question 1; `--effort` lists `low, medium, high, xhigh, max` in 2.1.274 |
| OC 6 — harness comments cite 2.1.267 | `:73` | Step 5a, 5b | PASS (landed in `502f0751`; step 5b re-checked it and found no `2.1.267`) |
| OC 7 — rebuild the sandbox image | `:73` | Step 2, proven by step 3 | PASS, **without a rebuild**: step 2 proved no commit between the image's label and HEAD touches `sandboxd` or `images/sandbox`, and step 3 proved the behaviour anyway |
| OC 8 — `Files` and `Merge` have no owning phase | `:77` | **Named by task 26** as seam S3.5 in `docs/SEAMS.md`, with the gap pinned; the ownership decision is the product owner's | Named by task 26 (S3.5); nothing observed here |
| OC 9 — `pinned` unbounded across branches | `:85` | **Deferred** to whichever task next opens `prefsSlice` | Deferred |
| OC 10 — an unpin does not survive a flip or restart | `:91` | **Deferred** to the next persisted-schema bump (`PERSIST_VERSION`) | Deferred |
| OC 11 — `observe` drops `dirty` before the write | `:99` | **Deferred to Phase 3**, as recorded | Deferred to Phase 3 |
| OC 12 — `proxy.Relay` has no production caller | `:102` | **Ruled by task 26**: deleted, after an anchored grep re-proved no production caller; `proxy.RelayHooked` is the one relay (`docs/SEAMS.md` S8.7) | Ruled by task 26; nothing observed here |
| Contract §12 open question 1 — `@xterm/xterm@6` under jsdom never observed | `2026-09-18-phase-2-ui-contract.md:2355-2358` | Answered as it prescribes: real xterm is observed in steps 8–15 by Playwright, not by `canvas` in jsdom | **Answered**: a real xterm mounted and was read by Playwright — `c2.cols` off its PTY, `p1.fidelity` off its DOM spans, `c3.theme` off its painted colours |
| Phase 1 unproven item 5 — the native window | `2026-09-17-phase-1-results.md:68` | Step 15 (agent half), step 16 (owner half) | Agent half, **half closed**: the DevTools port was allowed (`connectOverCDP` attached, `phase2.e2e.mjs:894`), `w.webview2` ran, and the native window loaded the cockpit and mounted a pane; live PTY output in that pane was NOT observed (F5). Owner half (step 16) PENDING OWNER |
| Task 5's Q3 and Q4 | `task-05.md:1457` | Step 4 | PASS |

## What is still unproven

1. Everything in 2026-09-17-phase-1-results.md:62-67 that needs an AKS cluster — Kata
   isolation, the RuntimeClass name, cold start, the read-only root in practice. Phase 8.
2. Contract §4.7's refresh half: declared as not delivered by task-24.md:28.
3. The CORS grant is task 24 step 11's evidence, relied on here and not re-run (step 6 proved
   only that the allowlisted origin gets 200 and another gets 403).
4. All nineteen owner items: batched to the owner's end-of-build session (OD-12), unverified
   until then. That includes whether a resumed agent remembers (step 18).
5. Org inference (OD-19) is the cloud path and is not built in this phase. Local mode (OD-20),
   which this run exercised, still needs `/login` in every sandbox.
6. **Carry 1, both halves** — scrollback across a re-parent, and the Canvas cap boundary. Both
   checks failed on F3's storm without reaching the property.
7. **Carry 9's exit row** (F3), and **carries 4, 6 and 10 and detach** (F5). The 10 Hz flush under
   load has no measured rate at all, because the load never reached the screen.
8. **Resume end to end** (OC 2): the fork was never reached. `o2.resume`'s failure is F2's — a
   host-git read 500'd the resume's own create (OO-1) — so the phase's only end-to-end
   proof of Resume, and of seam SX.10, was lost to one `git config` call on a quiet host that failed
   without saying why — F2, whose 10 s bound is inferred, not observed.
9. **A live terminal in the native window**: the pane mounted; output in it did not arrive.
10. **F4**, which this run did not exercise either way.

## Reproducing this

In Git Bash, every command through the audited wrapper, after taking the Docker lock and running
its guard. `$E2E` is an **empty** directory under `$TEMP`; `$CA` is the proxy root as a PEM file
(step 1 (i)). The ports are the plan's own, and `PANACEA_APP_ALLOWED_ORIGINS` is **left unset** —
`DefaultOrigins` already carries `http://localhost:1420`, and setting the variable replaces the
defaults instead of adding to them.

1. The image, as task 25 step 2 writes it — or, if `git rev-list --count <label>..HEAD -- sandboxd
   images/sandbox` is 0 and both trees are clean, skip the rebuild and prove the content with
   steps 3 and 4's probes, as this run did.
2. The fixture and the control plane:
   `bash desktop/e2e/fixture.sh "$E2E"` and `(cd control-plane && go build -o "$E2E/bin/panacea-api.exe" ./cmd/api)`.
3. One environment for everything after this, exported in the shell that runs the harness:
   `PANACEA_APP_ADDR=127.0.0.1:8730`, `PANACEA_APP_PROJECTS_DIR=$E2E/projects`,
   `PANACEA_APP_STATE_DIR=$E2E/state`, `PANACEA_APP_CA_BUNDLE=$CA`,
   `PANACEA_APP_REAP_INTERVAL=24h`, `PANACEA_APP_GIT_NAME="Panacea E2E"`,
   `PANACEA_APP_GIT_EMAIL=e2e@invalid`, `PANACEA_APP_SESSION_CPU_MILLIS=250`,
   `PANACEA_APP_SESSION_MEMORY_MIB=512`, `PANACEA_APP_LOG_LEVEL=debug`,
   `PANACEA_E2E_APP=http://localhost:1420`, `PANACEA_E2E_API=http://127.0.0.1:8730`,
   `PANACEA_E2E_CP_LOG=$E2E/control-plane.log`, `PANACEA_E2E_CP_BIN=$E2E/bin/panacea-api.exe`,
   and `PANACEA_E2E_CP_PID` = the PID `netstat` shows on 8730 once it is up (Windows paths via
   `cygpath -w`). Do **not** set `PANACEA_APP_ALLOWED_ORIGINS`.
4. Start the control plane (`"$E2E/bin/panacea-api.exe" >> "$E2E/control-plane.log" 2>&1 &`) and,
   from `desktop/`, Vite (`node node_modules/vite/bin/vite.js --port 1420 --strictPort`).
5. The native window, from `desktop/`: `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=9222
   npx tauri dev --config '{"build":{"devUrl":"http://localhost:1420","beforeDevCommand":null}}'`
   — `beforeDevCommand: null` because Vite is already running. Cargo recompiles the desktop crate
   the first time (3m 31s here).
6. The run, from `desktop/`: `PANACEA_E2E_CDP=http://127.0.0.1:9222 npm run e2e > "$E2E/results.jsonl"; echo "exit=$?" > "$E2E/results.exit"`
   — redirected, not piped, so the exit code is the harness's own.
7. Afterwards: kill the control plane and Vite **by the PID `netstat` shows on the port**, not by
   `$!`; `taskkill //PID <desktop.exe> //T //F` for the native window; then release the Docker
   lock.

**Guards on this document.** Step 20's anchored list is in `task-25.md`. Three checks read this
document against `results.jsonl` and must stay with it: `doc-vs-run.mjs` (every check in *The
checks* matches the run's own line, and no PASS is printed for a check the run did not pass);
a tables check (no row of *Phase 1, re-proven* or *Carry disposition* may say PASS for a check the
run did not pass, and the summary row's arithmetic must match the run); and a rule that a NOT RUN
row's *Observed* cell is an em dash and nothing else.
