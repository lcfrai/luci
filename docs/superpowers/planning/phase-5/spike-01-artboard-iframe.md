# Spike 01 — the script-less artboard iframe, measured in Edge

**Task:** phase 5 task 01 (`docs/superpowers/plans/2026-09-23-phase-5-design-canvas-parts/task-01.md`).
**Measured in:** Microsoft Edge 153.0.4234.48 (Chromium 153.0.8010.53, V8 15.3.12.7), Windows 11
25H2 build 26200.9448, devicePixelRatio 1.5, on this workstation, 2026-09-24. **Not WebView2**:
Edge and WebView2 share the Chromium runtime, and the same page in the Tauri shell is item 1 of
task 30's hands-on list (`OD-12`).
**Tools:** `playwright-core` 1.63.0 (Apache-2.0), `modern-screenshot` 4.7.0 (MIT), `html-to-image`
1.11.13 (MIT), installed in a scratch directory by the runner, never in the repo.
**Files:** `canvas/editor/spike/artboard-spike.html` (the page, the eleven checks, the fixtures),
`canvas/editor/spike/run.mjs` (the runner), `canvas/editor/spike/README.md`. Throwaway: nothing
imports them.

## Answer in one screen

- **The construct works for layout, hit-testing, overlays, keys and security.** The parent reads
  the artboard's DOM, boxes and computed styles. `elementFromPoint` hits the right node through
  `pointer-events: none`. A parent `<textarea>` wraps like the artboard's text. Keys stay in the
  parent. Nothing inside runs: script, `onerror`, `javascript:` and meta refresh are all blocked
  by the sandbox alone. The meta CSP stops every outbound load.
- **Capture: adopt `modern-screenshot`.** It renders five of the six fidelity features; both
  libraries miss `backdrop-filter`. `html-to-image` also draws the web font in the fallback face.
  The slowest 2× capture of a 1440×900 artboard is about half a second while the window is visible.
- **Three findings go to the orchestrator:**
  1. **The capture libraries bypass the artboard's CSP.** They fetch the artboard's URLs from the
     parent document, whose CSP is `null` in the Tauri shell (`desktop/src-tauri/tauri.conf.json:21`).
     Both libraries sent the two `https://` requests the artboard's own CSP had refused.
  2. **Capture in a minimised window is not dependable.** The layout half of a render request is
     right while hidden: 160 px. The capture half, across every recorded hidden run, took from
     about 0.4 s (a `toDataURL` capture) to more than 135 s (the main capture, unresolved), and page
     timers themselves run late (an in-page 10 s cap fired after 84.6 s). `html-to-image` never
     finishes while hidden: it waits on `requestAnimationFrame`.
  3. **Whether a minimised window hides its page at all varies with the machine's state.** On
     2026-09-24 the page went hidden in 19 of 20 recorded armed waits. On 2026-09-25 (fix round 1)
     it stayed `"visible"` every time, and so did a background tab. See "S11 across runs".
- **Parent listeners fire inside the scriptless document (S7).** Task 26 may edit text inside the
  iframe. Its default, a parent `<textarea>` over the text, also works (S5).

## How it was measured

`node run.mjs` does the following:

1. Serves the page and the two libraries from `http://127.0.0.1:8780/`, the first free port in
   8780–8789.
2. Starts Edge itself, replaces its first tab over raw CDP (departure 8), and attaches with
   Playwright's `connectOverCDP(url, { noDefaults: true })`. It checks the effective command line
   (below).
3. Loads the page. S1–S9 run in the page. The runner makes a real click and key press for S10.
   It then takes Edge's own screenshot of the S8 fidelity frame, as the classifier's control.
4. Minimises the window through CDP for S11 (`Browser.getWindowForTarget`, then
   `Browser.setWindowBounds`). It first calls `window.__armHidden()` and waits on it (bounded 5 s)
   before it sends the render request.
5. Restores the window and prints `window.__spike`.

`node run.mjs --negative` loads the same page with `?negative=1` and asserts that every negative
variant failed. `node run.mjs [--negative] --check <result.json>` applies the same assertions to a
recorded run, with no browser.

The iframe under test is the part's, attribute for attribute: `sandbox="allow-same-origin"`,
`tabindex="-1"`, `style="pointer-events:none;border:0"`, and a `srcdoc` whose head carries the meta
CSP `default-src 'none'; img-src blob: data:; font-src blob: data:; style-src 'unsafe-inline'`. Only
the body differs per check. Until fix round 1 the style also carried `display:block`, which made
this sentence false (review M3). No measured value depended on it, because every parent-side
position comes from `getBoundingClientRect`. It is gone, and the runner now fails a run whose frame
A differs from the part's block in any of the four.

### Effective command line (read from `edge://version`, the `#command_line` row)

```
"C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" --remote-debugging-port=0
  --user-data-dir="C:\Users\AU-DEV~1\AppData\Local\Temp\pa-p5-edge-<random>" --no-first-run
  --no-default-browser-check --edge-skip-compat-layer-relaunch --disable-features=msForceBrowserSignIn
  --flag-switches-begin --flag-switches-end --site-per-process about:blank
```

- **The three background switches are absent.** The runner exits 3 if any is present, or if no
  route yields the line.
- **The CDP route was refused.** `Browser.getBrowserCommandLine` answered "Command line not
  returned because --enable-automation not set". The `edge://version` fallback worked.
- **Chromium's OS sandbox is on:** there is no `--no-sandbox`, which Playwright's own launch adds.

## Departures from the part, each on the facts

1. **Edge is started by the runner, not by `chromium.launch`.** Playwright sends
   `Emulation.setFocusEmulationEnabled {enabled: true}` to every page it creates
   (`playwright-core` 1.63.0 `lib/coreBundle.js:37638`).
   - **The part's route, measured.** `node run.mjs --via-launch` at `36d3ceb` runs
     `chromium.launch` with the three switches removed exactly as the part asks; the command line
     confirmed them absent. Results:
     - the armed wait timed out with the minimised page still `"visible"` after 5 s;
     - `visibilityState` was `"visible"` at request and at answer;
     - the rAF probe fired only at 28,074 ms;
     - the capture missed its cap.
     The part's S11 negative would then have printed `not-applicable`.
   - A scratch probe with Playwright's default switches also saw `"visible"`, with rAF firing
     within 1.5 s.
   - **The runner's route.** Edge is started by the runner and attached with
     `connectOverCDP({ noDefaults: true })`, whose default context skips that override
     (`coreBundle.js:37634`). There the same page turned `"hidden"` and rAF stopped, on
     2026-09-24. Of the 17 armed waits the implementer recorded on this route
     (`scratchpad/p5/spike/out/`), 16 saw `"hidden"` after 0 to 2,500 ms, usually in under 60 ms.
     One, the control run c1, timed out `"visible"` after 5,000 ms. The reviewer's three more all
     saw `"hidden"`. So the runner's route does not always go hidden. On 2026-09-25 it never did
     (see "S11 across runs").
   - The part's route measures Playwright's focus emulation, not the renderer, and S11 exists to
     measure the renderer. It does show one case worth keeping: a page that says `"visible"` while
     rAF starves. Task 24's rule would pick rAF there, and only the responder's pre-measure `sync()`
     rescues the request. Whether WebView2 in a minimised Tauri window can be in that state is
     unverified (task 30).
   - The three switches are then simply never passed, and the check that they are absent still
     runs.
2. **`--disable-features=msForceBrowserSignIn` is passed.** This fleet's Edge policy opens
   `edge://force-signin/` at startup and then replaces the first tab, which closed pages under
   Playwright. Playwright's own launch disables the same feature.
3. **S6 counts requests that reached the network, not `request` events.** Playwright emits
   `request` for a load that the meta CSP blocks inside the renderer; it then fails with the
   reason `csp`. So "`page.on('request')` sees none" cannot hold even when the CSP works.
   - The runner fulfils `https://example.invalid/**` itself with a 1×1 PNG. "Reached the network"
     is that route being called. Without the CSP the image therefore loads (naturalWidth 1), rather
     than failing on DNS.
   - Both request events and their failure text are reported.
4. **S11 is bounded on the runner's clock, and records more than the part asks.**
   - A minimised page's own timers run late: an in-page 10 s cap fired after 84.6 s. So no cap
     inside the page bounds anything; the runner caps the request at 30 s on Node's clock.
   - The request's layout half (`window.__s11Partial`) stays readable even if the capture never
     answers.
   - `captured` requires the measured time to be at most 10 s, in the verdict **and in the field**
     (`window.__judgeS11`, fix round 1). Before, only the verdict applied it. A late resolve set
     `captured: true` (review M1: 54,864 ms in the reviewer's rev-pos, 67,875 ms in neg1). It is now
     recorded as `extras.s11.resolvedLate`. The runner feeds the judge three recorded answers as a
     self-test, and fails a result whose `s11.captured` is true above 10 s.
   - Extra facts, each capped on the runner's clock: `img` load + `decode()` of a blob: PNG and of
     an SVG `foreignObject` data URL; modern-screenshot's own timing log; `domToCanvas` with a
     synchronous `toDataURL`; the 1440×900 board; html-to-image.
5. **The negatives assert each leg, and add an S5 variant.**
   - The two predicted mutation survivors survived the part's four negatives (below).
   - S2's outcome now comes from one per-leg rule, and `--negative` asserts that every leg fails
     on its own.
   - S2 is measured three times. S2a is the artboard as task 24 builds it (sandbox + CSP). S2b
     drops the CSP, so the sandbox is measured alone. S2c (fix round 1) also drops the meta refresh.
   - **Why S2c.** Both S2a and S2b carry the refresh, so under `--negative` each failed on
     "document CHANGED" whatever the flags said. The "no flag was set" clause, the heart of S2, was
     then pinned by nothing (review I2: mutant R1 survived live). S2c's document cannot be
     navigated, so under `--negative` only the flags can fail it, and the runner asserts exactly
     that.
   - `--negative` also mis-sizes S5's overlay (1.25× font) and asserts that S5 fails.
   - The positive run asserts S11's layout half (width 160 while minimised).
6. **The web font is Consolas** (`C:/Windows/Fonts/consola.ttf`, `PANACEA_APP_SPIKE_FONT`),
   fetched by the parent and handed to the artboard as a `blob:` URL. Its `i` is about twice a serif
   `i` (175.9 px against 88.9 px for ten), which makes "rendered in the web font" and "rendered in
   the fallback" separable by pixels. It is used locally and not redistributed.
7. **The classifier has a control.** The same pixel classifier runs on Edge's own screenshot of the
   fidelity frame, which must read "rendered" for all six features; it does.
8. **The runner replaces Edge's first tab before Playwright attaches** (fix round 1, review M4).
   - **The hang.** From 23:43 on 2026-09-24, most runner starts hung in `connectOverCDP` after
     "ws connected": 10 of the reviewer's 13 starts, and both starts of fix round 1's first run.
   - **Its cause, measured with raw CDP** (`scratchpad/p5/fix1/rprobe.mjs`, `cprobe.mjs`):
     - the first tab of a fresh Edge answers no CDP command at all (`Runtime.evaluate` still
       unanswered after 150 s);
     - a second tab opened next to it answers in 46 to 118 ms;
     - Playwright initialises every page before `connectOverCDP` resolves. Its protocol log shows
       `Page.enable` and eight more commands sent to the first tab and never answered.
   - **Not the fix.** `--no-startup-window`, Playwright's own choice for `launch()`
     (`coreBundle.js:43324`), only moves the hang to the first tab opened, and so does a tab in a
     new window. Why the first tab of a window hangs is not established.
   - **The fix.** Over raw CDP, before Playwright attaches, the runner opens a second tab and
     closes every tab that was there. If a start still fails, it retries once on a fresh Edge with
     a fresh profile.
9. **Clean-up is by profile, and nothing is swallowed** (review M4).
   - Every `msedge.exe` whose command line names the run's `pa-p5-edge-<random>` profile is
     killed. Children carry `--user-data-dir` too, and some outlive their browser process, where
     `taskkill /T` cannot reach them.
   - Killed processes took from seconds to over a minute to go on this machine. The runner polls
     for about 30 s and then logs what is left, and whether the profile could not be removed. It
     does not fail the run for that.

## Results — positive run of record

`node run.mjs` at commit `36d3ceb`, exit 0: 11 checks and 8 `captureMs` rows. S1–S10 gave the
same outcomes in every positive run from `fdb28b5` on; S11 did not (see "S11 across runs").

| Id | Outcome | Evidence (the page's own sentence, as printed) |
|---|---|---|
| S1 | pass | contentDocument non-null; body.childElementCount 2, fixture has 2 |
| S2 | pass | S2a (sandbox + CSP): probes 4/4, flags [undefined, undefined, undefined], document unchanged; S2b (sandbox, no CSP): the same, after 1 s. Edge's console: "Blocked script execution in 'about:srcdoc' because the document's frame is sandboxed and the 'allow-scripts' permission is not set"; "Refused to execute the redirect specified via `<meta http-equiv='refresh'>` … The document is sandboxed" |
| S3 | pass | rect 24,24 120x40 (want 24,24 120x40 within 0.5 px); computed display "flex", background-color "oklch(0.62 0.19 250)" |
| S4 | pass | elementFromPoint(30, 30) is #n1 (want #n1) while the iframe's pointer-events is "none" |
| S5 | pass | paragraph 230 px wide wraps to 4 lines (height 96 / line-height 24); parent textarea styled from "16px Arial, sans-serif" wraps to 4 (scrollHeight 96) |
| S6 | pass | https img naturalWidth 0; requests to example.invalid that reached the network: 0; Playwright request events 2, failed with csp |
| S7 | pass | FIRES: a parent listener on contentDocument.body ran 1 time for a parent-realm MouseEvent and 1 for an iframe-realm one (either answer is acceptable) |
| S8 | partial | see the fidelity and time matrices below; adopted modern-screenshot; its slowest 2× capture 497 ms |
| S9 | pass | img src blob:… (parent-made from a PNG Uint8Array); naturalWidth 60 after 0 ms under the CSP |
| S10 | pass | a trusted pointerdown at 316,50 hit the parent's viewport; then the parent's activeElement is viewport; key "k" reached the parent's keydown with target viewport; contentDocument.hasFocus() false |
| S11 | fail | window minimized; armed wait saw hidden after 767 ms; visibilityState "hidden", width 160 (want 160), rAF probe did not fire within 2 s, modern-screenshot capture did not resolve within 10 s (the in-page 10 s cap itself fired only at 135.2 s) |

`window.__spike.s11` as printed: `{"visibilityState":"hidden","rafFiredWithin2s":false,"width":160,
"captured":false,"hiddenCaptureMs":null,"waitMs":767,"commandLine":"…"}` (the command line is the
one quoted above).

**Re-run after fix round 1** (acc-pos at `263275f`, 2026-09-25, exit 0; fin-pos at `422976a`
gave the same outcomes).
- S1–S10 have the same outcomes as above, and the same evidence values except S8's times.
- S2 adds S2c: 3/3 probes, no flag set, document unchanged.
- The iframe matches the part's block in all four recorded attributes, and the judge self-test
  passed.
- S11 reads `pass`, but for the visible case. The armed wait timed out `"visible"` after
  5,000 ms, `width` was 160, rAF fired, and the capture resolved in 98 ms.
- The table above stays the run of record for the hidden case, which this machine did not
  reproduce on 2026-09-25.
- The adopted library's slowest 2× on 1440×900 was 591 ms, against 497 ms above. That changes no
  decision: `max(30 s, 3 × 0.591 s)` is still 30 s.

### S8 — capture fidelity, per feature

| Feature (fixture region) | modern-screenshot | html-to-image | Edge's own rendering (control) |
|---|---|---|---|
| Web font from a `blob:` URL (`SpikeFont`) | rendered | **wrong**: the fallback serif face (run ends at x 97.5; the web font's at 183.5) | rendered |
| `linear-gradient` | rendered | rendered | rendered |
| `box-shadow` (6 px spread) | rendered | rendered | rendered |
| `filter: blur(2px)` | rendered | rendered | rendered |
| `backdrop-filter: blur(8px)` over stripes | **missing** (stripes crisp: 0..255) | **missing** (0..255) | rendered (108..128) |
| `border-radius` clipping a `blob:` image | rendered | rendered | rendered |

- **Adoption rule.** The adopted library is the one with fewer "missing" rows; ties go to
  modern-screenshot. Both have one "missing" row, so it is a tie, and **modern-screenshot is
  adopted**. html-to-image's extra "wrong" row points the same way.
- **Why html-to-image draws the fallback face.** It reads the web font rules from the parent's
  `document.styleSheets` (an inference from the result, not read in its source). The artboard's
  `@font-face` lives in the iframe's document.
- **`backdrop-filter` is not shown in screenshots** by either library. That is task 24's
  `capture.ts` doc-comment line and task 17's guide line.

### S8 — capture time, window visible (ms, call to `Blob` in hand, PNG encode included)

The 1440×900 artboard has 207 elements, the web font and two 320×200 `blob:` images. The 390×844
one has 63 elements and the same two images. Five runs each, in order.

| Library | Artboard | Scale | Median | Slowest | Five runs | PNG size |
|---|---|---|---|---|---|---|
| modern-screenshot | 1440x900 | 1 | 419 | 1330 | 1330, 403, 413, 425, 419 | 1440x900 |
| modern-screenshot | 1440x900 | 2 | 475 | **497** | 496, 497, 462, 469, 475 | 2880x1800 |
| modern-screenshot | 390x844 | 1 | 177 | 323 | 323, 171, 177, 172, 224 | 390x844 |
| modern-screenshot | 390x844 | 2 | 190 | 213 | 180, 195, 213, 190, 184 | 780x1688 |
| html-to-image | 1440x900 | 1 | 563 | 1266 | 1266, 548, 527, 638, 563 | 1440x900 |
| html-to-image | 1440x900 | 2 | 571 | 588 | 579, 571, 567, 558, 588 | 2880x1800 |
| html-to-image | 390x844 | 1 | 170 | 245 | 245, 163, 170, 170, 167 | 390x844 |
| html-to-image | 390x844 | 2 | 182 | 185 | 185, 171, 176, 185, 182 | 780x1688 |

Each library's very first capture is its slowest (1,330 and 1,266 ms): warm-up, including font
embedding. Over the
five positive runs whose figures were read (pos2, pos3, pos8, pos9, final-pos), the adopted
library's slowest 2× figure was 480 to 532 ms.

### S11 across runs (window minimised; the layout half and the capture half)

| Run (commit) | Armed wait | width | Main capture (fixture A, 2×) | Other facts while hidden |
|---|---|---|---|---|
| pos2 (working tree before `fdb28b5`) | 27 ms | 160 | **resolved, 5,954 ms** → S11 pass | — |
| pos3 (working tree before `fdb28b5`) | 22 ms | 160 | **resolved, 4,982 ms** → S11 pass | 1440×900 board at 2×: no answer within an in-page 60 s cap |
| pos7 (working tree before `ec303f0`) | 28 ms | 160 | no answer within 60 s (runner clock) | three modern-screenshot captures: no answer in 30 s each; html-to-image: none in 15 s |
| pos8 (working tree before `ec303f0`) | 23 ms | 160 | not within 10 s (the in-page cap fired at 84.6 s) | blob: PNG `<img>` load: no answer within 10 s; SVG `foreignObject` data URL: load 31 ms, `decode()` 0 ms; fixture A with modern-screenshot's timing log: 3,599 ms, of which **"canvas to blob" 3,439 ms**; both boards: no answer in 30 s |
| pos9 (`ec303f0`) | 27 ms | 160 | not within 10 s (cap fired at 22.6 s) | blob: PNG load 21 ms, decode 1 ms; fixture A timing log: 2,599 ms, "clone node" 448 ms, **"canvas to blob" 2,130 ms**; `domToCanvas` + synchronous `toDataURL`: no answer in 30 s on all three artboards |
| neg1 (`ec303f0`, rAF-only) | 51 ms | 120 (the negative) | resolved at 67,875 ms | `domToCanvas` + `toDataURL`: fixture A 14.0 s (encode 1.45 s), 390×844 board 10.2 s, 1440×900 board 3.7 s (encode 543 ms); 1440×900 `domToBlob` 17.0 s |
| c1 (`60f97e7`, rAF-only control) | **timed out `"visible"`, 5,000 ms** | 160 (rAF fired at 1 ms) | resolved, 163 ms, while `"visible"` | the runner printed `S11-negative: not-applicable` |
| final-pos (`36d3ceb`) | 767 ms | 160 | not within 10 s (cap fired at 135.2 s) | nothing answered: neither decode probe, nor a bare read of `window.__decodeState`, within 5–30 s |
| final-neg (`36d3ceb`, rAF-only) | 22 ms | 120 (the negative) | **resolved, 4,327 ms** | decode probes 2 ms and 0 ms; fixture A with the timing log 6,409 ms; `toDataURL`: fixture A 678 ms, 390×844 board 1,147 ms, 1440×900 board 1,341 ms; 1440×900 `domToBlob` 2,371 ms; html-to-image: none in 15 s |
| rev-pos (reviewer, `9505eac`) | 29 ms | 160 | resolved only at **54,864 ms**, so not within 10 s (the field said `captured: true` until fix round 1) | fixture A with the timing log 9,979 ms; fixture A `toDataURL` **405 ms**; the rest: no answer within their caps |
| rev-neg2 (reviewer, rAF-only) | 25 ms | 120 (the negative) | no answer within 30 s | nothing answered |
| r1d (reviewer, mutant R1) | 35 ms | **not measured**: neither the request nor `__s11Partial` answered | — | the S11 negative still passed at `9505eac` (review I1) |
| fix round 1, all 2026-09-25 runs (`289e74b`, `263275f`, `422976a`) | **timed out `"visible"`, 5,000 ms, every time** | 160 (rAF fired) | resolved in 98 to 159 ms, while `"visible"` (the seven unmutated runs) | a background tab in the same window also stayed `"visible"` (raw-CDP probe) |

html-to-image never answered while hidden, in any run: no answer within 15 s each time. Its image
step waits on rAF after `decode()` (`dist/html-to-image.js`:
`r.onload=function(){r.decode().then(function(){requestAnimationFrame(...)})}`), and a hidden
page runs no rAF.

**What this establishes:**

1. **The layout half holds.** Every positive run measured width 160 while hidden, so the
   schedulePatch rule and the pre-measure flush work.
2. **The capture half is not dependable** in a minimised Edge window. The main capture (fixture A,
   2×) was measured hidden in 16 runs: the table's 10 and the implementer's 6 mutant runs.
   - Within 10 s: 4 (pos2, pos3, final-neg, and m1b at 7,355 ms).
   - Late: 4 (m2 15.0 s, m4 32.3 s, rev-pos 54.9 s, neg1 67.9 s).
   - Never, within its cap: 8. pos4 and pos5 also hit the 9-minute watchdog while hidden.
   - Other hidden captures took from 405 ms up. Run to run, on the same machine and page, that is
     about 0.4 s to more than 135 s.
3. **The page's own timers run late while hidden.**
4. **The PNG encode (`canvas.toBlob`) is the slow stage when the capture does finish**, by
   modern-screenshot's own timing log.
5. **Whether minimising hides the page at all is not fixed.**
   - On 2026-09-24, 19 of 20 recorded armed waits on the runner's route saw `"hidden"`; c1 did
     not.
   - On 2026-09-25, from about 00:40, every run and probe stayed `"visible"` while minimised, with
     rAF firing and the capture resolving in about 135 ms. A background tab in the same window also
     stayed `"visible"`. The session was an active RDP session (session 2); LogonUI ran only in
     the console session (1), so the review's "workstation locked" does not follow from LogonUI.
   - What decides it is not established. For the product, a minimised cockpit may be hidden or
     visible, and the render path must be right either way (item 1 under "For the orchestrator").

The single cause is **not** established. The final run's unanswered bare `evaluate` suggests the
renderer ran no main-thread task at all for a while, but that is unverified.

### Capture egress (security finding)

Both libraries were run against the S6 frame, whose own CSP had refused its two `https://` URLs.
Each library then sent both URLs to the network from the parent (the runner's route was called):

- modern-screenshot: `https://example.invalid/x.png`, `https://example.invalid/bg.png`, in every
  run that recorded egress.
- html-to-image: the same two in every positive run. In every negative run it sent three, `x.png`
  twice (for example final-neg). There the S6 frame has no CSP and had already loaded `x.png`
  itself; why that adds a request is not established.

The capture code runs in the parent, so the parent's CSP governs it. In the Tauri shell that CSP is
`null` (`desktop/src-tauri/tauri.conf.json:21`). **The artboard CSP is not a line against capture
egress.**

The sanitiser (task 07) drops any `url()` or `src` that is not an `asset:` ref or a `data:` image
(`task-07.md:86`, `:138`). So this matters only if something outside the sanitiser puts a remote
URL into an artboard. It is still the third line missing for capture.

## The negative runs

`node run.mjs --negative` at `36d3ceb`: exit 0, "negatives: every variant failed as expected".

| Variant (what `?negative=1` changes) | Check | Outcome | What failed, as measured |
|---|---|---|---|
| S2a rebuilt with no `sandbox` (CSP kept) | S2 | fail | flags stay undefined (the CSP blocks script, `onerror` and `javascript:`), but the meta refresh navigated: "document CHANGED". Leg verdict `pass: false` |
| S2b rebuilt with no `sandbox` and no CSP | S2 | fail | flags `[1, 1, 1]`: the inline script, `onerror` and the `javascript:` link all ran; document CHANGED. Leg verdict `pass: false` |
| S2c (fix round 1) rebuilt with no `sandbox`, no CSP and no refresh | S2 | fail | acc-neg at `263275f`: flags `[1, 1, 1]`, document **unchanged**, leg verdict `pass: false`. The flags clause alone failed it |
| S6 rebuilt with no `<meta>` CSP | S6 | fail | naturalWidth 1; 2 requests reached the network (`x.png`, `bg.png`); no CSP failures |
| S4's node moved to 200,200 | S4 | fail | `elementFromPoint(30, 30)` is `#row`. S3 also fails (rect 200,200), which is expected |
| S11 rAF-only, no pre-measure flush (waits one frame; bounded by a 2 s page timer at `36d3ceb`, by the runner's release 2 s later since fix round 1) | S11 | fail | at `36d3ceb`: width 120 while hidden (armed wait saw hidden after 22 ms), so not-applicable was not needed. At `263275f` (acc-neg): the page stayed `"visible"` for 5 s, rAF fired, width 160, and the runner printed `S11-negative: not-applicable`, as the part provides. c1 at `60f97e7` did the same |
| S5's overlay mis-sized 1.25× (added, see departures) | S5 | fail | the paragraph wraps to 4 lines, the mis-sized textarea to 5 |

- **The S11 negative waits one frame instead of measuring at once.** Measuring immediately after
  a rAF-only schedule gives 120 even in a visible page, so it would prove nothing. Waiting one
  frame gives 160 when visible and 120 when rAF is suspended. So it fails exactly when the page is
  hidden, which is what it exists to catch.
- **One more departure:** the part says rAF-only with "no pre-measure flush". I read that as a
  responder that trusts the frame; this is my reading, declared.
- **What the S11 negative asserts since fix round 1** (review I1). At `36d3ceb` the runner
  asserted only that S11's outcome was `fail`. The capture half fails S11 by itself in most hidden
  runs, so that passed whatever the layout half did. It passed r1d, whose width was never
  measured, and mutant R2 (the negative flushes, width 160) on rev-neg2 and neg1 (desk run
  below). Now, whenever the armed wait saw `"hidden"`, the width must be the unflushed 120.
- **Why the release moved to the runner.** In r1d neither the request nor `__s11Partial`
  answered, and the wait's 2 s page timer is a candidate cause: hidden page timers ran up to minutes
  late here. The runner now calls `window.__s11Release()` 2 s after the request, on Node's clock.
  An evaluate is a task the runner sends, not a timer, and one answered while hidden in most runs
  (rev-neg2's `__s11Partial`). It is not a guarantee: final-pos's page answered no evaluate at all
  for a while. In that state nothing measures the layout half, and the negative now fails (exit 1)
  instead of passing.

## Mutation checks

Copies were built under `scratchpad/p5/mut/<tag>/` from `git archive HEAD canvas/editor/spike`
plus the uncommitted spike files. One mutant per copy, each applied exactly once (the script exits
2 otherwise, and printed "applied: yes" for every one). Each copy ran its own runner with
`PANACEA_APP_SPIKE_QUICK=1`, which skips only S11's extra hidden facts.

| Mutant | Named by | Run | Before the fix | After the fix |
|---|---|---|---|---|
| no `sandbox` on S2's frames | the part | `--negative` | killed: S2 fail (S2a navigated, S2b flags set) | killed |
| no meta CSP on S6's frame | the part | `--negative` | killed: S6 fail (reached 2, naturalWidth 1) | killed |
| S4's node at 200,200 | the part | `--negative` | killed: S4 fail (`#row`) | killed |
| rAF-only `schedulePatch`, no flush | the part | `--negative` | killed: S11 fail (width 120, hidden) | killed |
| **S5's comparison forced `true`** | predicted survivor 1 | `--negative` | **survived** (m1: exit 0, S5 pass) | **killed** (m1b at `60f97e7`: exit 1, "S5 did not fail with the overlay mis-sized 1.25x") |
| **S2 without its document-unchanged clause** | predicted survivor 2 | `--negative` | **survived** (m2: exit 0; S2b's flags masked it) | **killed** (m2b at `60f97e7`: exit 1, "S2 leg S2a did not fail on its own") |
| S11's pre-measure flush removed (positive path) | extra | positive | would survive: S11 already fails on capture | **killed** (m3 at `36d3ceb`: exit 1, "S11 layout half: width 120 while minimised, want 160") |
| S6 without its `naturalWidth === 0` clause | extra | `--negative` | survives (m4 at `36d3ceb`: exit 0) | **equivalent**: the route fulfils every request that leaves the renderer, so an image can load only if the route was hit, and the clause is implied |

Control copy c1 (no mutant, `60f97e7`, `--negative`): exit 0.

### Fix round 1 (2026-09-25)

Copies of `263275f`, and of `422976a` for mX1b, mX1c and X2, under `scratchpad/p5/fix1/mut/`.
Same method: one mutant per copy, "applied: yes" every time, `PANACEA_APP_SPIKE_QUICK=1`. The two
predicted survivors (P1, P2) were named in the stream ledger before any mutant ran. Every live run
below met the "visible while minimised" state, so no live run could exercise an S11 negative.

| Mutant | Named by | Run | Result |
|---|---|---|---|
| R1: `s2LegPass` without its flags clause | review (survived at `9505eac`, r1d exit 0) | `--negative` | **killed**: mR1 exit 1, "S2 leg S2c did not fail on its own" |
| R2: the S11 negative keeps the pre-measure flush | review (survives the old assertion on rev-neg2 and neg1) | `--negative` live; desk | live mR2: exit 0, **not exercised** (not-applicable, page visible). Desk, on three real hidden negatives: the old assertion let it survive on rev-neg2 and neg1; `--check` **kills it on all three**: "S11 negative: width 160 while hidden" |
| M1: the judge counts a late resolve as captured | this round | positive | **killed**: mM1 exit 1, "S11 judge self-test failed" |
| M3: the iframe style regains `display:block` | this round | positive | **killed**: mM3 exit 1, "frame A style is … display:block" |
| X1: the first-tab pre-flight removed | this round | positive | **survived** 3 of 3 runs at exit 0, but not by luck alone. mX1's first start connected. In mX1b and mX1c (batch 2, `422976a`) the first start hung and the retry on a fresh Edge connected. Both lines of M4's fix are real; X2 removes both |
| X2: X1 plus no retry | this round | positive | **killed** 2 of 2: mX2a and mX2b exit 1, "connectOverCDP: Timeout 30000ms exceeded" at their only start |
| **P1: the negative's release sent at once, not after 2 s** | predicted survivor 1 | `--negative` | **survived** (mP1 exit 0, not-applicable). Equivalent while hidden: rAF never fires, so the patch is stranded whenever the measure runs |
| **P2: the kill-by-profile sweep finds nothing** | predicted survivor 2 | positive | **survived** (mP2 exit 0). The profile was logged "NOT removed", as it is in unmutated runs, because the browser process outlived the ~30 s sweep there too. Nothing asserts clean-up, by design: a slow exit is not a failed measurement |
| none (control mC) | — | `--negative` | exit 0 |

**Batch 2, at `422976a`** (the pre-flight now records whether the first tab answered within 2 s):
fin-pos and fin-neg both exit 0, and in both the first tab did **not** answer. The pre-flight
replaced it, and each connected at its first start.

**Still unpinned: S1, S3, S7, S9 and S10.** They have no negative variant, so a "forced pass"
mutant of any of them would survive; none was run. They are measurements whose evidence sentences
carry the measured values.

## Decision table

| Question | Answer from | Answer | What follows |
|---|---|---|---|
| Can the renderer read layout? (S1, S3) | S1, S3 | **Yes.** `contentDocument` readable; n1's rect exactly 24,24 120×40; computed `display: flex` and `oklch(0.62 0.19 250)` | no stop; task 24 proceeds |
| Is the sandbox a real second line? (S2, S6) | S2, S6 | **Yes.** The sandbox alone (S2b, no CSP) blocks inline script, `onerror`, `javascript:` and meta refresh. The meta CSP lets 0 requests reach the network (S6) | both lines hold inside the artboard. **But capture code runs in the parent and fetches past the artboard CSP** (capture egress, above): for capture, the third line is missing |
| Text editing inside the iframe or over it? (S7) | S7 | **Either.** Parent listeners on nodes inside the scriptless document fire, for parent-realm and iframe-realm events alike. The parent overlay also lines up (S5: 4 lines each) | task 26 keeps its default (over it, measured by S5). Inside is open, but `contenteditable` and input events inside a scriptless document were not measured |
| Which capture library? (S8) | S8 | **modern-screenshot.** One "missing" row each, a tie, which goes to modern-screenshot (MIT, spec L14); html-to-image also draws the web font wrong | features neither renders: **`backdrop-filter`**, listed as "not shown in screenshots" in task 24's `capture.ts` and task 17's guide |
| Do `blob:` assets work? (S9) | S9 | **Yes.** A parent-made `blob:` PNG reaches naturalWidth 60 under the CSP | no `data:` fallback needed |
| Does the renderer answer while minimised? (S11) | S11 | **Layout yes, capture no.** Width 160 in every positive run. The capture resolved within 10 s in 2 of the 6 positive runs that finished (4,982 and 5,954 ms); 2 more (pos4, pos5) hit the runner's 9-minute watchdog while hidden. Final run: `visibilityState "hidden"`, `rafFiredWithin2s false`, `hiddenCaptureMs null`, `waitMs 767`. On 2026-09-25 (fix round 1) the minimised page stayed `"visible"` in every run, and captured in about 0.1 s: minimising does not always hide the page (S11 across runs) | **the "fail" route: to the orchestrator as a finding** (owner decision OQ-S11 below). Task 24 still builds `schedulePatch` and the pre-measure `sync()`; they hold (the layout half) |
| How long may one approval capture take? (S8 time) | S8 `captureMs`, the adopted library's slowest 2× row | **497 ms** (modern-screenshot, 1440x900, 2×) | `ApprovalRenderTimeout = max(30 s, 3 × 0.497 s) = 30 s`. The visible figure is far under 10 s, so there is no finding on it. **It does not bound a capture in a minimised window.** That capture missed 10 s in 4 of 6 positive runs, and had not resolved by 60 s (pos7), 84.6 s (pos8) or 135.2 s (final-pos) in three of them (the S11 row). Fix round 1's seven unmutated runs gave slowest 2× figures of 488 to 591 ms, which changes nothing |

## For the orchestrator

1. **OQ-S11, owner-visible (the part's S11 "fail" row).** In a minimised Edge window a render
   request's layout half works, but its capture half is not dependable: from about 0.4 s to more
   than 135 s, and html-to-image never finishes. Approval captures and `get_screenshot` from the
   hidden renderer are what this breaks; layout tools are unaffected. And a minimised window is not
   always hidden (finding 3), so the path must be right for both states.
   - **Option (a):** keep the shell's webview unthrottled. Tauri 2 has a Windows window option,
     `additionalBrowserArgs` (`tauri-utils` 2.9.3 `src/config.rs:2080-2083`, in the local cargo
     registry). **Unverified**: whether WebView2 honours the three switches through it, and whether
     they stop this stall. Playwright's own use of them points that way but proves nothing for
     WebView2.
   - **Option (b):** `renderer-unavailable` says *"restore the cockpit window"* for a capture while
     minimised.
   - **Recommended:** build (b) as the behaviour behind a seam; task 24's responder already
     distinguishes the halves. Try (a) in the Tauri shell. (b) is right even if (a) works, because
     the user can minimise at any moment.
   - **When (b) is hidden-only.** Fix round 1 measured a minimised page that stayed `"visible"`,
     with rAF running and the capture done in about 135 ms (S11 across runs). So (b) should key on
     `document.visibilityState === "hidden"` at request time, not on "minimised". A visible page
     captures as usual.
   - **What adopting (b) amends** (review M5). Each is unmade until the orchestrator rules:
     - **Task 30, hands-on item 1** (`task-30.md:121-126`). Its pass condition, "the PNG shows the
       write and no `renderer-unavailable` comes back", would fail by design under (b). It becomes:
       a PNG that shows the write, **or** `renderer-unavailable` with the restore-the-window
       sentence while the page is hidden. It must also say which one the shell gave.
     - **Task 30, hands-on item 1, gains the (a) trial.** Set `additionalBrowserArgs` to the three
       switches, minimise, and record whether a capture then resolves. As written, item 1 contains
       no such trial. This report said "(a) tried in task 30's hands-on item 1" until fix round 1.
     - **Task 13's causes** (`task-13.md:112-119`). There are three today: `ErrNoRenderer`,
       `ErrRendererDetached` and `ErrRenderTimeout`. (b) adds a fourth that wraps
       `ErrRendererUnavailable`, answered at once by the responder while hidden, whose reason
       sentence says *"restore the cockpit window"*. Task 21's approval loop, which tells the
       causes apart, then names it to the user instead of waiting out the 30 s.
2. **Capture egress, security.** Both libraries fetch artboard URLs from the parent document, past
   the artboard's CSP. The parent's policy governs those fetches, and in the Tauri shell it is
   `null` (`desktop/src-tauri/tauri.conf.json:21`). So there are two fixes, and the first is the
   root cause (corrected in fix round 1, review M6: this item first called the `fetchFn` the
   root-cause fix):
   - **Root cause: an app CSP** in `tauri.conf.json`, whose `img-src`, `font-src` and
     `connect-src` admit only what the cockpit needs. It bounds every parent-side fetch, whatever
     the library, and any other parent code.
   - **Defence in depth: modern-screenshot's `fetchFn`**, refusing everything except `blob:` and
     `data:`. It exists in 4.7.0: it is in the library's defaults, `fetchFn:null`. **Unverified**:
     that it intercepts every fetch the library makes (images, fonts, backgrounds).
   - **A consequence to check, not measured here.** By the HTML standard an `about:srcdoc`
     document inherits its parent's policy container. So an app CSP also applies inside every
     artboard, on top of the artboard's own. It must then admit `style-src 'unsafe-inline'` and
     `blob:`/`data:` images and fonts, or artboards break. Task 24 or task 30 should measure it.
   - The sanitiser already drops remote URLs (`task-07.md:86`, `:138`). So today this is a missing
     third line, not an open hole.
3. **The approval timeout.** 30 s stands on the visible figure (497 ms). The hidden-renderer
   approval path (P5-Q, `design.approving`) inherits item 1.

## Not claimed

- **WebView2 itself is not measured.** Everything here is Edge 153 on this workstation. The same
  page in the Tauri shell is item 1 of task 30's hands-on list (`OD-12`).
- **Not tried:** native WebView2 capture (`ICoreWebView2::CapturePreview`). It needs Rust in the
  shell and captures the whole webview, and it is the fallback if both libraries prove unusable.
- **S8's times are one machine's**: Edge on this workstation, window visible, idle main thread.
- **The S11 stall's single cause is not established.** Measured: page timers run late,
  modern-screenshot spends its hidden time in "canvas to blob", and one run answered no `evaluate`
  at all. Not measured: whether Edge froze the page, and whether a longer-hidden page behaves
  differently. The runs stayed under 5 minutes hidden.
- **Not established (fix round 1):** why the first tab of an Edge window answers no CDP, and why,
  on 2026-09-25, minimised pages and background tabs stayed `"visible"`. Both were measured, not
  explained. The S11 negative did not run live in fix round 1; its new assertion rests on desk
  runs over the recorded hidden runs of 2026-09-24.
- **The runner routes around two Playwright defaults.** It does not use `chromium.launch`
  (focus emulation) and does not use `page.exposeFunction`, which took 17 s on a fresh tab in a
  scratch probe. Both facts are from this tree's `playwright-core` 1.63.0.
- **Not measured: `contenteditable` or input events inside the scriptless document**, the
  in-iframe half of S7's choice.
- Nothing here is product code; nothing here is imported by the package.
