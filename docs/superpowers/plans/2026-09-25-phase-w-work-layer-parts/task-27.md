### Task 27: faces: Commit Mono bundled with its licence; Segoe UI Variable as the sans

Stream **WD1**, the face half of seam **SW.11** (`--mono`, `--sans` and two optical-size stacks),
about 2 hours (an estimate, not a measurement). Depends on task 26 (the `--t-*` steps these faces
are set in). `OD-28` decides both faces (Q10, Q14): **Commit Mono** for every machine fact, bundled,
because OFL 1.1 allows it and `OD-11` lists it; **Segoe UI Variable** for human language, never
bundled, because it is the Windows system face and its licence does not allow redistribution.

Two things make this more than a one-line stack edit.

**The terminal measures its cell once.** xterm sizes its grid from a measured character cell. A
terminal that opens before Commit Mono has loaded measures the fallback's cell, fits its columns to
that, and the grid is then wrong by the difference between two faces' advance widths for as long
as the pane lives. Today `Terminal.tsx` also ignores `--mono` altogether: it hard-codes
`'"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace'` at `:38`, although the comment in
`tokens.css:149-150` says "the --mono stack is the one xterm.js is configured with". Both are fixed
here: the terminal reads `--mono` from the token layer (the way `themeFromTokens()` at `:27-33`
already reads `--page` and `--ink`), and it waits, at most one second, for the face before it opens
and fits. The wait gates `handle.mount()` (the registry's `term.open`, `attachRegistry.ts:184-188`)
and the first `fit()` (`:120`) together, because the scope's "before its first fit" is not enough
if `open()` is where the cell is measured. Where xterm 6 measures is **UNVERIFIED** against its
source in this tree; gating both is right either way.

**The font is an asset with provenance.** A bundled face is third-party code in the installer. It
arrives with its licence text beside it, verbatim, and with a manifest recording the release it came
from and each file's SHA-256, so a later reviewer can tell the file on disk from the file published.

**Corrections to the scope, made here so nobody re-derives them:**

1. **`fonts.css` is imported second, not first.** `tokens.test.ts:85-90` pins `@import
   "./tokens.css";` as the first line of `app.css`. `fonts.css` goes directly after it, which is
   still before any rule (CSS requires every `@import` to precede rules anyway), so nothing shifts.
2. **The licence and manifest tests read the tree with `import.meta.glob`, not `node:fs`.** The
   desktop package has no `@types/node`, so a `node:fs` import fails the typecheck (TS2591);
   `designsystem.test.ts:8-16` records the same finding. A `?raw` glob lists the files; the hashes
   are checked by the acceptance command, not by a test, because a binary read through `?raw` is
   decoded as text and its hash would be meaningless.
3. **The terminal's stack comes from `--mono`, with a literal fallback.** jsdom resolves no custom
   property, so the literal (Commit Mono first) is what the tests see and what a webview that fails
   to load the token layer gets.

**Which files the Commit Mono release ships is UNVERIFIED** (static weights or one variable file;
the planning agent did not fetch it). The part is written for static 400 and 500 WOFF2 files, the
two weights the tokens use. If the release ships only a variable WOFF2, step 1 bundles that single
file with `font-weight: 400 700` in one `@font-face`, and the manifest records one entry; nothing
else in this part changes. If it ships no WOFF2 at all, stop and report: converting a TTF is a
modification the OFL allows only under a different name (the Reserved Font Name clause), which is
an owner call.

**Files:**
- `desktop/src/assets/fonts/CommitMono-400.woff2`, `CommitMono-500.woff2` — the release's files,
  renamed on import only (new; binary)
- `desktop/src/assets/fonts/OFL-CommitMono.txt` — the release's licence text, verbatim (new)
- `desktop/src/assets/fonts/fonts.lock.json` — release tag, upstream URL, each file's upstream
  name, our name and SHA-256 (new)
- `desktop/src/styles/fonts.css` — the `@font-face` rules (new)
- `desktop/src/styles/fonts.test.ts` — the licence, bundle and stack tests (new)
- `desktop/src/styles/app.css` ⚑ (82 lines; `@import "./tokens.css";` at `:10`) — one import (modified)
- `desktop/src/styles/tokens.css` ⚑ (`--sans`/`--mono` at `:151-152`, the comment above at
  `:144-150`) — the two stacks, two optical-size stacks, the comment (modified)
- `desktop/src/components/Terminal.tsx` ⚑ (178 lines; the literal stack at `:38`, the mount and
  first fit at `:119-120`, the observer at `:149-150`, the focus effect at `:164-166`) (modified)
- `desktop/src/components/Terminal.test.tsx` ⚑ (`describe("Terminal")` at `:67`) — four tests (modified)

The Tauri CSP is `null` (`desktop/src-tauri/tauri.conf.json:21`, verified), so local fonts need no
CSP change. An implementer who finds a CSP there at the sync base adds `font-src 'self'` to it and
says so.

#### Interfaces — complete: `desktop/src/styles/fonts.css`

```css
/* desktop/src/styles/fonts.css
 *
 * Bundled faces (Phase W task 27, OD-28). Local files only: the app is offline-first and a face
 * fetched from a network is a layout shift on a corporate laptop. Vite fingerprints each url()
 * into dist/assets. Every family here has an OFL-<Family>.txt beside its files in
 * src/assets/fonts/, and fonts.lock.json records where each file came from.
 *
 * font-display: block. A fact set in a fallback for 100ms and then re-set in Commit Mono moves
 * every column it sits in; a short invisible period is the lesser failure for local files, which
 * load in one frame. The terminal does not rely on this rule: it waits for the face itself.
 *
 * Segoe UI Variable is NOT here and must never be: it is the Windows system face and its licence
 * does not allow redistribution. fonts.test.ts fails on any @font-face or file naming Segoe.
 *
 * The token import below is required, not decorative: designsystem.test.ts:648-650
 * (importNotFirst, TOKEN_IMPORT at :238) fails any stylesheet but tokens.css whose first line is
 * not it. app.css imports tokens.css too; Vite's CSS import resolution inlines a file once. */
@import "./tokens.css";

@font-face {
  font-family: "Commit Mono";
  src: url("../assets/fonts/CommitMono-400.woff2") format("woff2");
  font-weight: 400;
  font-style: normal;
  font-display: block;
}

@font-face {
  font-family: "Commit Mono";
  src: url("../assets/fonts/CommitMono-500.woff2") format("woff2");
  font-weight: 500;
  font-style: normal;
  font-display: block;
}
```

#### Interfaces — complete: `app.css` and `tokens.css`

`app.css`, one line added directly after `:10`:

```css
@import "./tokens.css";
@import "./fonts.css";
```

`tokens.css:144-152`, the type comment and the two stacks, replaced by:

```css
  /* ── Faces (OD-28; Phase W task 27) ───────────────────────────────────────────────
   * Human language is sans; every machine fact is mono (05 §4.4 C3, voice.split).
   * Commit Mono is bundled (OFL 1.1, src/assets/fonts/, declared in fonts.css). Segoe UI
   * Variable is the Windows system face and is never bundled; on macOS and Linux the sans
   * stacks fall to the platform face. The --mono stack is the one Terminal.tsx reads, so it
   * must end in real monospace faces on every platform: xterm lays out a grid, and a
   * proportional fallback would break it. */
  --sans: "Segoe UI Variable Text", "Segoe UI", -apple-system, system-ui, sans-serif;
  --sans-small: "Segoe UI Variable Small", var(--sans);     /* --t-meta: 12px optical size */
  --sans-display: "Segoe UI Variable Display", var(--sans); /* --t-head: 16px optical size */
  --mono: "Commit Mono", "Cascadia Mono", ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono", monospace;
```

`var()` inside a `font-family` list is legal: the custom property substitutes its token list,
so `--sans-small` resolves to `"Segoe UI Variable Small", "Segoe UI Variable Text", "Segoe UI",
…`. `--sans` is unchanged from `:151`; it is restated so the block reads whole.

#### Interfaces — complete: `desktop/src/assets/fonts/fonts.lock.json`

The shape, with the values step 1 fills in. Nothing reads it at run time; `fonts.test.ts` checks
that every bundled file has an entry, and the acceptance command checks the hashes.

```json
{
  "commit-mono": {
    "family": "Commit Mono",
    "licence": "OFL-1.1",
    "licenceFile": "OFL-CommitMono.txt",
    "release": "<the release tag, e.g. v1.143 — UNVERIFIED until fetched>",
    "source": "https://github.com/eigilnikolajsen/commit-mono/releases/tag/<tag>",
    "files": [
      { "ours": "CommitMono-400.woff2", "upstream": "<name in the release>", "sha256": "<64 hex>" },
      { "ours": "CommitMono-500.woff2", "upstream": "<name in the release>", "sha256": "<64 hex>" }
    ]
  }
}
```

The upstream repository and the `v1.143` example tag are **UNVERIFIED** (from memory, not fetched);
step 1 confirms both before anything is committed.

#### Interfaces — complete: `Terminal.tsx`

Added after `TERM_FG_TOKEN` (`:19`):

```ts
/** Task 27 (OD-28). The mono stack comes from the token layer, like the theme below. The literal
 *  is what jsdom and a webview without tokens.css get; it leads with the same face. */
const TERM_MONO_TOKEN = "--mono";
export const TERM_MONO_FALLBACK = '"Commit Mono", "Cascadia Mono", ui-monospace, Menlo, Consolas, monospace';

/** The face the grid is measured in, at the size makeTerm sets (fontSize 12). */
export const MONO_FACE = '12px "Commit Mono"';
/** How long a terminal waits for that face before it opens on whatever it has. Bounded, so a
 *  missing file costs one second of an empty pane, never a pane that never opens. */
export const MONO_WAIT_MS = 1000;

export function termFontFamily(): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(TERM_MONO_TOKEN).trim();
  return value === "" ? TERM_MONO_FALLBACK : value;
}

/** True when the grid can be measured now: the face is loaded, or there is no FontFaceSet to ask
 *  (jsdom), in which case nothing is rendered anyway. */
function monoFaceReady(): boolean {
  const fonts = document.fonts as FontFaceSet | undefined;
  return fonts === undefined || fonts.check(MONO_FACE);
}

/** Resolves when the face loads, when its load fails, or after MONO_WAIT_MS, whichever is first.
 *  Never rejects: every outcome means "open now". */
function whenMonoFace(): Promise<void> {
  return Promise.race([
    document.fonts.load(MONO_FACE).then(() => undefined, () => undefined),
    new Promise<void>((resolve) => { setTimeout(resolve, MONO_WAIT_MS); }),
  ]);
}
```

In `makeTerm`, `:38` becomes `fontFamily: termFontFamily(),`. Nothing else in `makeTerm` changes.

In `TerminalImpl`, one ref beside `handleRef`, kept current on every render, so a terminal that
opens late still takes the caret it was given while it waited:

```ts
  const focusedRef = useRef(focused);
  focusedRef.current = focused;
```

The mount effect's body from `:118` to the end of its cleanup becomes:

```ts
    node.appendChild(handle.element);

    /* Task 27. Open, fit and observe only once the grid can be measured in Commit Mono. On the
     * ready path (the face is cached, or jsdom) this runs synchronously, exactly as before, so
     * the ordering tests above this change still hold. On the waiting path `live` stops a
     * terminal from opening into a pane that has since unmounted. */
    let live = true;
    let ro: ResizeObserver | null = null;
    const open = (): void => {
      if (!live) return;
      handle.mount();                              // AFTER the append: xterm measures a
      fitAddons.get(handle.term)?.fit();           // rendered element, never a detached one
      ro = new ResizeObserver(() => fitAddons.get(handle.term)?.fit());
      ro.observe(node);
      if (focusedRef.current) handle.focus();      // the focus effect ran before we opened
    };
    if (monoFaceReady()) open();
    else void whenMonoFace().then(open);

    return () => {
      live = false;
      ro?.disconnect();
      if (handle.element.parentNode === node) node.removeChild(handle.element);
      handleRef.current = null;
      releaseAttach(sessionId);
    };
```

The two long `EDITED BY TASK 21's FIX ROUND 1` comments stay where they are, above the observer
line, which moves inside `open`. A comment `EDITED BY PHASE W TASK 27, as a declared departure`
opens the new block, in the file's own convention for a closed task's file.

`handle.mount()` is idempotent (`attachRegistry.ts:184-188`, `if (held.opened) return;`), so a
terminal re-parented between Columns and Canvas after its face loaded opens once, as before.

#### Tests to write first — `desktop/src/styles/fonts.test.ts` (new)

```ts
import { describe, expect, it } from "vitest";
import appCss from "./app.css?raw";
import fontsCss from "./fonts.css?raw";
import tokensCss from "./tokens.css?raw";
import lock from "../assets/fonts/fonts.lock.json";
import { stripCssComments } from "../test/fakes";

/** Every file in assets/fonts, keyed by name. Only names and the .txt bodies are read: a woff2
 *  decoded as text is meaningless, so its hash is checked by the acceptance command instead. */
const RAW = import.meta.glob<string>("/src/assets/fonts/*", { query: "?raw", import: "default", eager: true });
const FILES = new Map(Object.entries(RAW).map(([k, v]) => [k.replace("/src/assets/fonts/", ""), v]));
const FACES = [...FILES.keys()].filter((n) => n.endsWith(".woff2")).sort();
const stack = (name: string): string[] => {
  const m = new RegExp(`\\n\\s*${name}\\s*:\\s*([^;]+);`).exec(stripCssComments(tokensCss));
  return m ? m[1].split(",").map((s) => s.trim()) : [];
};

describe("the bundled faces", () => {
  it("bundles at least one face", () => {
    expect(FACES.length).toBeGreaterThan(0);   // guards every loop below from passing on nothing
  });

  // Kills: a font with no licence beside it.
  it("ships a licence beside every bundled face", () => {
    for (const family of new Set(FACES.map((n) => n.split("-")[0]))) {
      const licence = FILES.get(`OFL-${family}.txt`);
      expect(licence, `OFL-${family}.txt`).toBeDefined();
      expect(licence).toContain("SIL Open Font License");
      expect(licence).toContain("Version 1.1");
    }
  });

  // Kills: redistributing a system font.
  it("bundles no Segoe face", () => {
    expect([...FILES.keys()].filter((n) => /segoe/i.test(n))).toEqual([]);
    expect(stripCssComments(fontsCss)).not.toMatch(/segoe/i);
  });

  // Kills: a face with no recorded origin, or a placeholder left in the manifest.
  it("records every bundled file in fonts.lock.json", () => {
    const entries = Object.values(lock).flatMap((f) => f.files);
    expect(entries.map((e) => e.ours).sort()).toEqual(FACES);
    for (const e of entries) expect(e.sha256).toMatch(/^[0-9a-f]{64}$/);
    expect(JSON.stringify(lock)).not.toMatch(/UNVERIFIED|<[^>]*>/);
  });

  // Kills: a network font, and a url() naming a file that is not there.
  it("loads only local files, each blocking", () => {
    const css = stripCssComments(fontsCss);
    const urls = [...css.matchAll(/url\("([^"]+)"\)/g)].map((m) => m[1]);
    expect(urls.length).toBe(FACES.length);
    for (const u of urls) expect(FACES).toContain(u.replace("../assets/fonts/", ""));
    expect(css.match(/@font-face/g)?.length).toBe(css.match(/font-display: block;/g)?.length);
  });

  // Kills: Berkeley (the prototype's first entry) left first, and a chain ending in a proportional face.
  it("puts Commit Mono first in --mono and keeps a monospace fallback", () => {
    const mono = stack("--mono");
    expect(mono[0]).toBe('"Commit Mono"');
    expect(mono[mono.length - 1]).toBe("monospace");
    expect(mono.join(",")).not.toMatch(/Berkeley|sans-serif|system-ui/);
  });

  // Kills: an optical-size stack that does not fall back to the sans.
  it("keeps the sans on the system face, with its optical sizes", () => {
    expect(stack("--sans")[0]).toBe('"Segoe UI Variable Text"');
    expect(stack("--sans-small")).toEqual(['"Segoe UI Variable Small"', "var(--sans)"]);
    expect(stack("--sans-display")).toEqual(['"Segoe UI Variable Display"', "var(--sans)"]);
  });

  // Kills: a late import that shifts layout, and a fonts import that displaces the token layer.
  it("imports fonts.css before any rule", () => {
    const lines = stripCssComments(appCss).split("\n").map((l) => l.trim()).filter(Boolean);
    expect(lines.slice(0, 2)).toEqual(['@import "./tokens.css";', '@import "./fonts.css";']);
  });
});
```

`stack()` matches a declaration at the start of a line, so `--sans` does not match inside
`--sans-small`'s `var(--sans)`.

#### Tests to write first — `desktop/src/components/Terminal.test.tsx` (appended inside `:67`)

Two imports join the file's head: `import { FitAddon } from "@xterm/addon-fit";` (the mocked class
at `:28-30`) and `MONO_WAIT_MS, TERM_MONO_FALLBACK, termFontFamily` from `./Terminal`. One helper and
one `afterEach` go above `describe("Terminal")`:

```ts
/** Task 27: jsdom has no FontFaceSet. This installs one whose face is not yet loaded, and hands
 *  back the switch that loads it. Removed after each test, so the older tests keep jsdom's
 *  "no FontFaceSet" path, which is the synchronous one. */
function unloadedFace(): { load: ReturnType<typeof vi.fn>; loadNow: () => void } {
  let done: (() => void) | null = null;
  const load = vi.fn(() => new Promise<FontFace[]>((resolve) => { done = () => resolve([]); }));
  Object.defineProperty(document, "fonts", { configurable: true, value: { check: () => false, load } });
  return { load, loadNow: () => done?.() };
}
afterEach(() => { delete (document as { fonts?: unknown }).fonts; });
```

```ts
  // Kills: a fit measured on the fallback face (the terminal opens and fits before the face loads).
  it("fits after the mono face loads", async () => {
    const fit = vi.spyOn(FitAddon.prototype, "fit");
    const face = unloadedFace();
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    const term = grab().term as unknown as FakeTerm;
    expect(face.load).toHaveBeenCalledWith('12px "Commit Mono"');
    expect([term.opens, fit.mock.calls.length]).toEqual([0, 0]);
    await act(async () => { face.loadNow(); });
    expect(term.opens).toBe(1);
    expect(term.openedWhileConnected).toBe(true);
    expect(fit).toHaveBeenCalled();
  });

  // Kills: an unbounded wait, which leaves a pane blank forever when the file is missing.
  it("opens after one second when the face never loads", async () => {
    vi.useFakeTimers();
    try {
      unloadedFace();
      seed(session());
      render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
      const term = grab().term as unknown as FakeTerm;
      await act(async () => { vi.advanceTimersByTime(MONO_WAIT_MS - 1); });
      expect(term.opens).toBe(0);
      await act(async () => { vi.advanceTimersByTime(1); });
      expect(term.opens).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  // Kills: opening into a pane that unmounted while it waited.
  it("never opens after it unmounted mid-wait", async () => {
    const face = unloadedFace();
    seed(session());
    const view = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    const term = grab().term as unknown as FakeTerm;
    view.unmount();
    await act(async () => { face.loadNow(); });
    expect(term.opens).toBe(0);
  });

  // Kills: the caret lost because the focus effect ran before the late open.
  it("takes the caret once it opens, when it mounted focused", async () => {
    const face = unloadedFace();
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused />);
    const term = grab().term as unknown as FakeTerm;
    const before = term.focused;
    await act(async () => { face.loadNow(); });
    expect(term.focused).toBe(before + 1);
  });

  // Kills: the hard-coded Cascadia-first stack left in place (Terminal.tsx:38 today).
  it("sets its face from --mono, with Commit Mono first in the fallback", () => {
    expect(TERM_MONO_FALLBACK.startsWith('"Commit Mono",')).toBe(true);
    expect(termFontFamily()).toBe(TERM_MONO_FALLBACK);   // jsdom resolves no custom property
  });
```

The `grab()` helper (`:57-64`) is the file's own way to reach the entry without re-creating it.
`restoreMocks: true` (`vitest.config.ts:15`) restores the `fit` spy after each test.

#### Steps

1. **Fetch and record the face.** From the Commit Mono project's GitHub releases, take the latest
   release's WOFF2 files for weights 400 and 500 and its licence file. Confirm the licence is SIL
   OFL 1.1 and read its Reserved Font Name line: if a name is reserved, the files are bundled
   byte-for-byte and only *renamed on disk*, never modified (renaming a file is not modifying the
   font). Copy them into `desktop/src/assets/fonts/` under our names, the licence verbatim as
   `OFL-CommitMono.txt`, and fill `fonts.lock.json` with the tag, the release URL, each upstream
   name and `sha256sum` of each file. The corporate network may need `curl --ssl-no-revoke`; the
   files are fetched by a person or an agent with network access, never at build time.
2. Write `fonts.test.ts` and the five Terminal tests. Run them: the stack, import and Terminal
   tests fail; the licence and manifest tests pass only if step 1 is complete (that is their job).
3. Write `fonts.css`; add its import to `app.css`; replace the stacks in `tokens.css`.
4. Change `Terminal.tsx` as above. Run `npm --prefix desktop test`: all pass, the older Terminal
   tests included, because jsdom takes the synchronous path.
5. Mutate by hand and watch the named test go red, then revert: put `"Berkeley Mono"` first in
   `--mono`; move the fonts import below the `*` reset; set `MONO_WAIT_MS` to `Infinity`; drop the
   `live` check; drop the `focusedRef` line; delete `OFL-CommitMono.txt`.
6. `npm --prefix desktop run build`, then the acceptance commands. `git add` the five new files and
   the two binaries; commit with `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test
# expect: every test passes, the eight in fonts.test.ts and the five new Terminal tests included

npm --prefix desktop run typecheck
# expect: exit 0

npm --prefix desktop run build && ls desktop/dist/assets | grep -c woff2
# expect: the build succeeds; the count is 2 (1 if the release shipped one variable file)

cd desktop/src/assets/fonts && sha256sum CommitMono-*.woff2
# expect: each hash equals its "sha256" in fonts.lock.json

ls desktop/src/assets/fonts desktop/dist/assets | grep -ci segoe
# expect: 0 (no Segoe file bundled; the CSS bundle names the stack, which is not a face)
```

#### What this task does not claim

- **Not the sans comparison.** Task 32 sets Segoe UI Variable beside the two alternatives 05 §4.4
  C3 names, for the owner's eyes. This task only wires the owner's choice (`OD-28`).
- **Not how the faces render at 125 % and 150 %** (task 33), nor on macOS or Linux, where the sans
  falls to the platform face and nobody has looked.
- **Not a legal review of the licence.** OFL 1.1 is on `OD-11`'s allowed list; that Commit Mono's
  release carries OFL 1.1 is **UNVERIFIED** until step 1 reads it.
- **Not which files the release ships**, its tag or its URL: all **UNVERIFIED** until fetched.
- **Not that the token layer is inlined once.** `fonts.css` must open with the token import (the
  audit's rule); that Vite drops the second copy under `app.css` is **UNVERIFIED**. A duplicate
  would be harmless (identical declarations) and larger; step 6 can check `dist/assets/*.css`
  for one `--t-fact:` occurrence.
- **Not that xterm measures at `open()`.** The gate covers `open()` and the first `fit()` so the
  claim does not matter; which of the two measures in xterm 6 is **UNVERIFIED**.
- **Not a visible change the owner has accepted.** Every fact in the terminal and in mono CSS
  changes face on this task; whether that reads better is the owner's eyes at task 39.
- Design: 05 §4.4 C3 (faces, revision 2); 02 C3; `OD-28`, `OD-11`. Parity: no 01 row names a
  face; it serves **L-234**'s look with task 26.

**Interfaces:**
- Produces (seam **SW.11**, the face half):
  - `--mono` (Commit Mono first, monospace last), `--sans`, `--sans-small`, `--sans-display`
  - `@font-face "Commit Mono"` 400 and 500, `font-display: block`, from `src/styles/fonts.css`
  - `Terminal.tsx`: `TERM_MONO_FALLBACK`, `MONO_FACE`, `MONO_WAIT_MS`, `termFontFamily()`
  - `src/assets/fonts/fonts.lock.json`, the manifest shape above; any later bundled face (a
    Geist, if the owner ever changes `OD-28`) adds an entry and an `OFL-<Family>.txt`
- Consumes: task 26's `--t-*` steps (named in the comments; no rule here sets a size);
  `attachRegistry`'s idempotent `mount()`; `desktop/src/test/fakes.ts`'s `FakeTerm` and
  `stripCssComments`
- Amends: nothing in the Hub. No wire shape changes.
