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
