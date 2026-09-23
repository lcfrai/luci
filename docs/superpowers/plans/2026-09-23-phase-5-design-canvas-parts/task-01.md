# Task 01 — Spike: the script-less artboard iframe

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Gates:** 24, 25, 26 ·
**Hours:** 2

## Why

Every renderer and editor task assumes one construct the spec could not test (spec §7.5, §9.1
item 1): an artboard rendered as real DOM inside an `<iframe srcdoc>` whose `sandbox` is
`allow-same-origin` **without** `allow-scripts`, so the parent can read layout and nothing inside
can run. This task measures that construct in Chromium, the engine WebView2 is, before 77 hours of
work lean on it. It writes a throwaway page, a runner, and a report. It writes **no product code**.

## Files

- `canvas/editor/spike/artboard-spike.html` (new) — one page, the ten checks below, results into
  `window.__spike` as JSON and into a visible table.
- `canvas/editor/spike/run.mjs` (new) — drives the page in the installed Microsoft Edge through
  `playwright-core`, headed, and prints `window.__spike`.
- `canvas/editor/spike/README.md` (new) — three lines: throwaway, how to run it, the report's path.
- `docs/superpowers/planning/phase-5/spike-01-artboard-iframe.md` (new) — the report.

`canvas/editor/spike/` is excluded from the package's build and tests by task 03's `tsconfig.json`
(`"exclude": ["spike"]`) and `vitest.config.ts` (`include: ["src/**/*.test.{ts,tsx}"]`). If task 03
has not landed, this directory is created alone.

## Contracts

The page's result object — the runner prints it, the report transcribes it:

```ts
// window.__spike, set once when every check has settled
interface SpikeResult {
  userAgent: string;                 // navigator.userAgent
  checks: Array<{
    id: "S1" | "S2" | "S3" | "S4" | "S5" | "S6" | "S7" | "S8" | "S9" | "S10";
    outcome: "pass" | "fail" | "partial";
    evidence: string;                // one sentence: the values measured
  }>;
}
```

The iframe under test, exactly as task 24 will build it (only the body differs):

```html
<iframe sandbox="allow-same-origin" tabindex="-1" style="pointer-events:none;border:0"
  width="390" height="844"
  srcdoc="<!doctype html><html><head><meta charset=utf-8>
<meta http-equiv=Content-Security-Policy content=&quot;default-src 'none'; img-src blob: data:; font-src blob: data:; style-src 'unsafe-inline'&quot;>
<style>html,body{margin:0;padding:0}</style></head><body>…</body></html>"></iframe>
```

## The ten checks

| Id | Check | Pass when |
|---|---|---|
| S1 | The parent reads `iframe.contentDocument` | non-null, and `contentDocument.body.childElementCount` equals the fixture's |
| S2 | Nothing inside runs: an inline `<script>` setting `parent.__ran1`, an `<img src=x onerror>` setting `parent.__ran2`, `<a href="javascript:parent.__ran3=1">` clicked by the parent through `dispatchEvent`, `<meta http-equiv=refresh content="0;url=about:blank">` | all three flags `undefined` and the document is unchanged after 1 s |
| S3 | Layout is readable from the parent | `getBoundingClientRect()` of a fixture node equals its known box (a 120×40 flex child at 24,24) within 0.5 px; `getComputedStyle` of it (from `contentWindow`) returns `display: flex` and the resolved `oklch()` colour |
| S4 | Hit-testing without events | `contentDocument.elementFromPoint(30, 30)` is the fixture node while the iframe has `pointer-events: none` |
| S5 | A parent-side text overlay lines up | a `<textarea>` in the parent, positioned from S3's rect and styled from S3's computed font, wraps the fixture paragraph onto the same number of lines (compare `scrollHeight / lineHeight`) |
| S6 | The meta CSP holds | an `<img src="https://example.invalid/x.png">` stays `naturalWidth === 0`; a `url(https://…)` background yields no network request (Playwright's `page.on('request')` sees none) |
| S7 | Parent-registered listeners on nodes inside the sandboxed document | record whether `contentDocument.body.addEventListener('click', f)` from the parent fires on a synthetic click. **Either answer is acceptable**; it decides whether task 26 edits text inside the iframe or over it |
| S8 | PNG capture fidelity | `modern-screenshot`'s `domToPng` and `html-to-image`'s `toPng` of `contentDocument.documentElement`, against a fixture with a web font from a `blob:` URL, a `linear-gradient`, a `box-shadow`, `filter: blur(2px)`, `backdrop-filter: blur(8px)` and `border-radius` clipping an image. Record per feature: rendered, missing, or wrong |
| S9 | A parent-made `blob:` URL loads inside | an `<img src="blob:…">` created by the parent from a PNG `Uint8Array` reaches `naturalWidth > 0` under the CSP |
| S10 | Keys stay in the parent | after clicking the iframe's area, `document.activeElement` in the parent is the viewport element, not the iframe |

## Tests first

A spike's tests are its checks; each is written before the fixture it measures, and each is written
so that it can fail:

| Test | Asserts | Kills the mutant |
|---|---|---|
| S2 with the `sandbox` attribute removed | the runner's `--negative` flag rebuilds the iframe with no `sandbox` and **expects S2 to fail** | a check that cannot fail — a page whose flags are never set for another reason |
| S6 with the meta CSP removed | `--negative` also drops the `<meta>` and expects S6 to see the request | a CSP check that passes because the host is simply unreachable |
| S4 with the node moved | the fixture node moved to 200,200 makes S4 fail at 30,30 | an `elementFromPoint` that always returns `body` |

## Steps

1. Write `artboard-spike.html`: the fixture bodies, the ten checks, `window.__spike`. No framework.
   The two capture libraries are loaded from a `node_modules` path the runner serves locally — the
   page never fetches from the internet (`www.jsdelivr.com` is blocked on this network, spec §11.3).
2. Write `run.mjs`: `npm install --prefix <scratchpad>/spike playwright-core@1 modern-screenshot@4
   html-to-image@1` (the registry is reachable, spec §11.3); serve `canvas/editor/spike/` and those
   packages from `http://127.0.0.1:<free port>/` with Node's `http` module; launch
   `chromium.launch({ channel: "msedge", headless: false })` (headless screenshots are blocked on the
   corporate fleet, `CLAUDE.md`); open the page; wait for `window.__spike`; print it as JSON; exit
   non-zero if any check is missing. `--negative` runs the three negative variants.
3. Run it both ways. Record Edge's version from `userAgent`.
4. Write the report: the result table, the three negative runs, the capture fidelity matrix (S8),
   and the **decision table** below with every row filled.
5. Commit the four files by pathspec.

### The decision table the report must fill

| Question | Answer from | If the answer is "no" — the fallback later parts already carry |
|---|---|---|
| Can the renderer read layout? (S1, S3) | S1, S3 | **Stop.** Every layout tool fails; the report goes to the orchestrator before task 24 starts |
| Is the sandbox a real second line? (S2, S6) | S2, S6 | the sanitiser (task 07) and the renderer's `createElement` path (task 24) remain; the report says the third line is missing |
| Text editing inside the iframe or over it? (S7) | S7 | over it: a parent `<textarea>` positioned from S3 (task 26 builds this by default) |
| Which capture library? (S8) | S8 | the one with fewer "missing" rows; ties go to `modern-screenshot` (MIT, spec L14). Features neither renders are listed in task 24's `capture.ts` doc comment and in the guide (task 17) as *"not shown in screenshots"* |
| Do `blob:` assets work? (S9) | S9 | `data:` URLs (larger, but same-origin by construction) |

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor/spike && node run.mjs
```
Expected: one JSON object with ten `checks`, each with an `outcome`; exit 0.

```bash
node run.mjs --negative
```
Expected: S2 and S6 report `fail` for the unsandboxed and CSP-less variants, S4 reports `fail` for the
moved node; exit 0 (the runner asserts that the negatives failed).

```bash
grep -c "^| S" /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/docs/superpowers/planning/phase-5/spike-01-artboard-iframe.md
```
Expected: `10` or more.

## Not claimed

- **WebView2 itself is not measured.** Edge and WebView2 share the Chromium runtime; the report
  says *measured in Edge <version>*, and the same page in the Tauri shell is item 1 of task 30's
  hands-on list (`OD-12`).
- Native WebView2 capture (`ICoreWebView2::CapturePreview`) is not tried; it needs Rust in the
  shell and captures the whole webview. It is the fallback if S8 finds both libraries unusable.
- Nothing here is product code; nothing here is imported by the package.
