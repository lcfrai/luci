# 07 — Desktop, terminal and UI primitives

*Research pass, 2026-09-28, for the OSS-adoption question raised at ~11% of the
programme built (≈53 of ≈486 tasks). Scope: the Tauri + React cockpit and every
UI primitive it needs across all phases — terminal rendering, log/list
virtualisation, command palette, keyboard engine, pane layout, charts, headless
component primitives, and Tauri desktop plugins.*

**Read-only pass.** No source was changed, no test run, nothing started. Nothing
in `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas source was
opened (`OD-11`, clean room).

**How licences were verified.** Every SPDX id below was read from the project's
own `LICENSE` file, the GitHub licence API (`api.github.com/repos/<r>` →
`license.spdx_id`), or the npm registry's `license` field
(`registry.npmjs.org/<pkg>/latest`), on **2026-09-28**. Where the corporate
proxy blocked a source, that is stated in the candidate row and listed again at
the end. Nothing here is from memory or from a badge.

---

## The one-paragraph answer

**There is real, unambiguous savings in three places and nowhere else.**

1. **Terminal.** We already depend on `@xterm/xterm` v6 and `@xterm/addon-fit`.
   We are using 2 of 13 MIT addons from the same repository. `addon-webgl`,
   `addon-serialize`, `addon-search`, `addon-unicode11`, `addon-web-links` and
   `@xterm/headless` each delete a named chunk of phase 2/3 work at zero licence
   risk and near-zero integration cost. **This is the single highest
   savings-per-hour item in the whole report.**
2. **Tauri desktop plumbing.** The official `tauri-apps/plugins-workspace`
   (MIT **or** Apache-2.0, our choice) ships `deep-link`, `single-instance`,
   `updater`, `window-state`, `store`, `log`, `global-shortcut`, `os`,
   `process`, `notification` — every desktop concern phase 2/6/9 has on its
   list, written by the Tauri team, versioned with Tauri itself. We currently
   depend on exactly one (`opener`). Adopting the rest removes most of phase 9's
   distribution work and phase 2's window/deep-link work.
3. **Layout and virtualisation.** `react-resizable-panels` (MIT) and a virtual
   list (`virtua` MIT, or TanStack Virtual MIT) replace hand-written geometry
   that is already 478 lines and will grow.

**Everywhere else the answer is "no", and for a reason that is not squeamishness
about licences.** The design system is the product's differentiator; the whole
rejected-prototype story is in `docs/DESIGN-SYSTEM.md`. A component library that
brings its own look is a permanent tax, not a saving. And the two primitives
people reach for first — a command palette and a keyboard engine — are *already
built* here (`CommandPalette.tsx` 145 lines, `useKeyboardMap.ts` 243 lines,
including a hard-won xterm key-routing fix documented in `Terminal.tsx`).
Replacing working code with a dependency is negative progress.

---
## What is already in the tree (the baseline this is measured against)

Read 2026-09-28 from `desktop/package.json`, `desktop/src-tauri/Cargo.toml` and
`desktop/src/` (108 files, 20,413 lines of TS/TSX including tests).

| Concern | Today | Lines |
|---|---|---|
| Terminal | `@xterm/xterm` ^6.0.0 + `@xterm/addon-fit` ^0.11.0 | `Terminal.tsx` 178 |
| Attach/reconnect | hand-written `api/attachRegistry.ts` | — |
| Command palette | hand-written `components/palette/` | 145 + commands |
| Keyboard map | hand-written `hooks/useKeyboardMap.ts` | 243 |
| Pane layout | hand-written `CanvasLayout.tsx` + `ColumnsLayout.tsx` | 405 + 73 |
| Popover / context menu / toasts | hand-written `components/common/`, `components/menu/` | — |
| Heartbeat clock | hand-written `hooks/heartbeat.ts` | — |
| State | `zustand` 5.0.15 | — |
| Tauri plugins | `@tauri-apps/plugin-opener` **only** | — |
| Charts | none yet | — |
| Virtualisation | none yet | — |

Two things follow from that table. The **cockpit's interaction layer is already
written and tested**, so most "obvious" adoptions are now *replacements*, not
gap-fills — a worse trade. The **desktop-shell layer is nearly empty**, so
almost everything the Tauri workspace offers is a pure gap-fill.

---

## Candidate table

Licence column = SPDX id read on 2026-09-28 from the source named in the notes.

| # | Candidate | Licence (verified) | Maturity | Replaces | Verdict |
|---|---|---|---|---|---|
| 1 | `@xterm/addon-webgl` | MIT (npm 0.19.0) | xterm.js 21.2k★, pushed 2026-09-13 | render perf work, phase 2 §7 | **adopt** |
| 2 | `@xterm/addon-serialize` | MIT (npm 0.14.0) | same repo | scrollback snapshot/restore, phase 2 resume + phase 3 transcript back-links | **adopt** |
| 3 | `@xterm/addon-search` | MIT (npm 0.16.0) | same repo | in-terminal find | **adopt** |
| 4 | `@xterm/addon-unicode11` + `addon-web-links` + `addon-clipboard` | MIT (npm 0.9.0 / 0.12.0 / 0.2.0) | same repo | width bugs, link handling, clipboard | **adopt** |
| 5 | `@xterm/headless` | MIT (npm 6.0.0) | same repo | server/Node-side terminal state for replay & screen snapshots | **adopt (conditional — Node, not Go)** |
| 6 | `tauri-apps/plugins-workspace` (`deep-link`, `single-instance`, `updater`, `window-state`, `store`, `log`, `global-shortcut`, `os`, `process`, `notification`) | npm metadata `"MIT OR Apache-2.0"`; GitHub's classifier reports `Apache-2.0` for the repo — **dual, take MIT** | official Tauri org, 1,817★, pushed 2026-09-28 | phase 2 desktop plumbing, phase 9 update channel | **adopt** |
| 7 | `keyring-rs` (`keyring` crate) | MIT **or** Apache-2.0, dual | 768★, maintained | `OD-6` Windows Credential Manager token storage, phase 2/H | **adopt** |
| 8 | `react-resizable-panels` | MIT (npm 4.14.1) | 5,377★, pushed 2026-09-27 | *gap-fill, not a deletion* — resizable rail and any future split layout. `--rail-w` is a fixed `248px` today | **adopt (when the feature is asked for)** |
| 9 | `virtua` | MIT (npm) | 3.7k★, active | log pane + rail virtualisation, phase 2 §7.2, phase 3 diff lists | **adopt** |
| 10 | `@tanstack/react-virtual` | MIT (npm 3.14.13) | 7,123★, pushed 2026-09-21 | same as 9 — the alternative | **adopt (pick one of 9/10)** |
| 11 | `uPlot` | MIT (10,518★, pushed 2026-09-27) | very active | phase 6 spend/quota charts, phase 2 live meters | **adopt** |
| 12 | `@floating-ui/react` | MIT (32.8k★) | very active | positioning inside our own `Popover.tsx` / `ContextMenu.tsx` | **adopt (narrow)** |
| 13 | `tauri-plugin-oauth` (FabianLars) | MIT **or** Apache-2.0, dual | 215★, Tauri v2 branch | loopback redirect for `OD-30` PKCE sign-in | **vendor** |
| 14 | `@xterm/addon-ligatures` | MIT (npm 0.10.0) | same repo | nothing we need; pulls a font-parsing dep | **reject** |
| 15 | `cmdk` | MIT | 13k★, 52 open issues | `CommandPalette.tsx` — **already built** | **reject** |
| 16 | `kbar` | MIT (5,255★, pushed 2026-08-10) | slowing | same | **reject** |
| 17 | `tinykeys` | MIT (4,099★, pushed 2026-09-25) | active, ~650 B | `useKeyboardMap.ts` — **already built, with an xterm fix a generic engine does not have** | **study-only** |
| 18 | `dockview` | MIT core (npm 8.3.1) — **`dockview-enterprise` is proprietary** | 3.4k★ | full docking; overshoots Columns/Canvas | **reject** |
| 19 | `allotment` | MIT (npm 1.20.5, 1,260★) | active | split views — but ships VS Code's look as CSS | **reject** |
| 20 | `react-mosaic-component` | Apache-2.0 (LICENSE read; GitHub API says `NOASSERTION`) | 4,801★ | tiling windows | **reject** |
| 21 | `react-virtuoso` | MIT per npm — **no LICENSE file in the repo root**; sibling `VirtuosoMessageList` is **commercial** | 6,460★, very active | virtualisation | **reject (use 9 or 10)** |
| 22 | `@melloware/react-logviewer` | **MPL-2.0** (fork of Mozilla `react-lazylog`, MPL-2.0) | 169★ | log pane | **CONDITIONAL — study-only** |
| 23 | `react-arborist` | MIT | 3.7k★, 65 open issues | `FilesLens` tree | **study-only** |
| 24 | Base UI (`@base-ui-components/react`) | MIT, 11k★ | by the Radix/Floating-UI/MUI people | our own primitives | **study-only** |
| 25 | Radix Primitives | MIT (19,338★, pushed 2026-08-08; now stewarded by WorkOS) | mature | same | **study-only** |
| 26 | `visx` | MIT (21,064★) — **pushed 2026-06-22, 3 months quiet** | slowing | charts | **study-only** |
| 27 | Observable Plot | ISC (5,390★, pushed 2026-09-01) | active | charts — but grammar-of-graphics defaults carry a look | **reject** |
| 28 | `recharts` | MIT (27,593★) | very active | charts — SVG per point, own look | **reject** |
| 29 | `libghostty` / `libghostty-vt` | MIT | **no versioned API; signatures "still in flux"** | a native terminal core | **study-only** |
| 30 | `shiki` | MIT | 13.8k★ | syntax highlighting in diff review (phase 3) and canvas code (phase 5) | **adopt (phase 3, not now)** |

---
## 1. Terminal — the xterm.js addons we are not using

**Licence.** `LICENSE` at `github.com/xtermjs/xterm.js` is MIT (read 2026-09-28
via `raw.githubusercontent.com`; copyright "2017-2026 The xterm.js authors;
2014-2017 SourceLair; 2012-2013 Christopher Jeffrey"). Every addon is published
from the **same repository under the same licence** — confirmed package by
package from `registry.npmjs.org/<pkg>/latest`, `"license":"MIT"`. Repo:
21,227★, last push 2026-09-13, not archived. We already ship `@xterm/xterm`
6.0.0 and `@xterm/addon-fit` 0.11.0, so **adopting more of them adds no new
licence surface, no new vendor and no new supply-chain root at all.** That is
rare and it is why this item is first.

The thirteen published addons, and which ones matter:

| Addon | ver | What it gives us | Verdict |
|---|---|---|---|
| `addon-webgl` | 0.19.0 | GPU renderer (`webgl2` canvas) | **adopt** |
| `addon-serialize` | 0.14.0 | Serialises the live buffer to **VT sequences or HTML** | **adopt** |
| `addon-search` | 0.16.0 | Find in scrollback, with decorations | **adopt** |
| `addon-unicode11` | 0.9.0 | Correct character widths | **adopt** |
| `addon-web-links` | 0.12.0 | URL detection + click handling | **adopt** |
| `addon-clipboard` | 0.2.0 | OSC 52 clipboard from inside the PTY | **adopt** |
| `addon-progress` | — | `OSC 9;4` progress reporting from the process | **evaluate** |
| `addon-web-fonts` | — | Waits for a webfont before measuring | **evaluate** |
| `addon-canvas` | 0.7.0 | 2D-canvas fallback renderer | **conditional** |
| `addon-attach` | 0.12.0 | Naïve WebSocket attach | **reject** |
| `addon-image` | 0.9.0 | Sixel/iTerm image protocol | **reject for now** |
| `addon-ligatures` | 0.10.0 | Font ligatures | **reject** |
| `addon-unicode-graphemes` | — | Grapheme clustering (**experimental**) | **reject for now** |

### Why `addon-webgl` is not optional for this product

The cockpit's stated hard case is *many concurrent live processes* — Columns
renders up to nine panes, Canvas caps at ten tiles (`CANVAS_MAX_TILES`), each an
independent terminal taking PTY bytes. xterm's default renderer draws every cell
as DOM. Nine DOM-rendered terminals under load is where this class of app dies,
and the design system's "log panes throttle to ~10 Hz" rule is a *throughput*
rule that a DOM renderer will still lose. `addon-webgl` moves glyph rasterising
into a `webgl2` canvas with a glyph atlas.

**Integration cost: ~1 hour.** `term.loadAddon(new WebglAddon())` inside the
existing `makeTerm()` in `Terminal.tsx`, plus a `contextLoss` handler that falls
back to `addon-canvas` and then to DOM. That fallback chain is the only real
work, and it is ~20 lines.

**Risk, stated honestly:** WebGL inside a WebView2 (Tauri on Windows) can hit
driver-dependent context loss on a KPMG SOE fleet with mixed GPU drivers, and
the corporate fleet is exactly the environment where that bites. So the
three-step fallback is mandatory, not a nicety, and the acceptance test must
include "force a context loss and confirm the terminal keeps rendering". The
xterm README does **not** declare WebGL mandatory or canvas deprecated — I read
the README on 2026-09-28 and it calls the GPU renderer "optional". Treat that as
support for keeping the fallback, not as a reason to skip the addon.

### Why `addon-serialize` is the sleeper

It serialises a terminal's whole buffer **to VT sequences** — i.e. to something
you can write straight back into a fresh terminal and get the identical screen.
That is exactly the primitive behind three separate pieces of planned work:

- **Phase 2 session resume.** Reattaching to a session currently means replaying
  whatever the server buffered. Serialise-on-detach / write-on-attach gives an
  exact screen restore, including scrollback, colours and cursor, with no
  server-side replay protocol to design.
- **Phase 3 diff review with transcript back-links.** A back-link wants a
  *rendered* snippet of what the agent's screen said at a commit. `serialize`
  has an **HTML** mode, so the snapshot is a styled fragment, not a screenshot —
  and it carries a provenance id as an attribute, which a screenshot cannot.
  *This is the provenance-thread argument, and it is why this addon beats a
  screenshot approach.*
- **Phase 9 audit trail.** An exact, replayable terminal state is far better
  evidence than a byte log, and it is produced without parsing PTY bytes —
  which the cross-cutting constraint forbids. Serialising xterm's own buffer is
  not "deriving meaning from PTY bytes"; it is asking the emulator what its
  screen is.

**Integration cost: ~half a day** for detach/attach snapshotting;
the HTML-snapshot path is a phase 3 item, not now.

### `@xterm/headless` — and the honest caveat

`@xterm/headless` 6.0.0 (MIT) is xterm.js with the DOM stripped, for Node. Pair
it with `addon-serialize` and the **server** can hold authoritative terminal
state: "what does session X's screen look like right now" answered without a
client attached. That is what makes an agent-fleet overview grid possible at all
— ten live thumbnails with no ten live WebSockets.

**The caveat that decides it: our control plane is Go, not Node.**
(`control-plane/go.mod`, `go 1.23`.) Adopting `@xterm/headless` means putting a
Node process in the server path, which is an architecture change and should not
be done casually. Two honest alternatives:

1. Run it in the **sandbox** alongside `sandboxd` (which is Go, using
   `creack/pty` v1.1.24) and have `sandboxd` shell out for snapshots — ugly.
2. Use a **Go** terminal emulator instead. I did not verify one to the standard
   this report requires, so I am not recommending one; the search terms are
   `hinshun/vt10x` and `charmbracelet/x/vt`. **Treat that as an open item, not a
   finding.**

Recommendation: **adopt `@xterm/headless` only if the snapshot feature is built
client-side first** (the desktop already has xterm; a hidden headless instance
per session is free there). Defer the server-side question.

### Terminal cores we are not going to use

- **libghostty / libghostty-vt** (ghostty-org/ghostty, **MIT**, 61,617★, pushed
  2026-09-27). A zero-dependency C+Zig terminal library with a C ABI, explicitly
  designed for embedding. Technically superb and licence-clean. **Rejected as a
  dependency today for one stated reason: there is no versioned API.** The
  project's own words on 2026-09-28: "API signatures are still in flux."
  Binding a Rust/Tauri app to an unversioned C ABI, then shipping it to a
  regulated customer's fleet, is a maintenance liability we would carry alone.
  **Verdict: study-only. Re-open when libghostty-vt tags a 1.0.**
- **wezterm's `termwiz` / `portable-pty`, Alacritty's `alacritty_terminal`** —
  real, permissively licensed crates, but they solve the *native* terminal
  problem. Our renderer is a WebView. Adopting them means rendering terminal
  cells ourselves in Rust and pushing frames to the WebView, which throws away
  xterm.js and everything built on it. **Reject.**

---
## 2. Tauri desktop plumbing — the biggest pure gap-fill

`src-tauri/Cargo.toml` declares exactly three Tauri crates: `tauri`,
`tauri-build`, `tauri-plugin-opener`. Everything else a shipped desktop app
needs is unwritten. The official `tauri-apps/plugins-workspace` (1,817★, pushed
**2026-09-28** — i.e. today; not archived) has it.

**Licence.** Every plugin's npm package declares `"license": "MIT OR
Apache-2.0"` (read from `registry.npmjs.org/<pkg>/latest`, 2026-09-28 —
`deep-link` 2.5.0, `updater` 2.13.0, `window-state` 2.5.0, `store` 2.5.0, `log`
2.10.0, `global-shortcut` 2.4.0, `notification` 2.5.0, `os` 2.4.0, `process`
2.4.0, `dialog` 2.8.0, `stronghold` 2.4.0). GitHub's licence classifier reports
`Apache-2.0` for the repository as a whole, which is the classifier picking one
arm of a dual pair — **we take the MIT arm and record both.** Same licence
family as Tauri itself, which we already ship.

| Plugin | Phase it serves | What it replaces |
|---|---|---|
| `deep-link` | 2 / H | `panaceaapp://` callback for the Keycloak PKCE round trip (`OD-30`) and "open this project/session" links from the Hub. Registering a URL scheme per-OS by hand is days. |
| `single-instance` | 2 | Second launch focuses the running cockpit instead of opening a second window with a second set of WebSocket attaches. Directly protects `attachRegistry`'s one-terminal-per-session invariant. |
| `window-state` | 2 | Window size / position / maximised across restarts. Small, but it is on every "feels unfinished" list. |
| `updater` | **9** | Signed update channel — the phase 9 line item "signed MSI and update channel for the KPMG fleet". The single largest phase 9 saving. Ships signature verification and a Windows NSIS/MSI install path. |
| `store` | 2 | Typed persisted key-value on disk. The cockpit persists through zustand into `localStorage` today (see `CanvasLayout.tsx`'s note about "a JSON serialise of the cockpit into localStorage"). A native store removes the WebView storage-quota and clear-site-data failure modes. |
| `log` | 2 / 9 | Structured logs to file with rotation — a prerequisite for the phase 9 audit trail and for supporting a fleet you cannot attach a debugger to. |
| `global-shortcut` | 2 | OS-level hotkey to summon the cockpit. Distinct from `useKeyboardMap`, which only sees in-window keys. |
| `os`, `process` | 2 / 9 | Platform facts and restart-self; needed by `updater`. |
| `notification` | 2 | "Agent needs you" when the window is not focused. `AGENTIC-UX.md`'s "assume the user is intermittently present" rule has no delivery mechanism today. |
| `dialog` | 2 / 3 | Native file / folder pickers for "open a local clone" (`OD-2`). |

**Integration cost: 1–2 days for the set**, most of it `capabilities/*.json`
permission wiring, which Tauri v2 requires per plugin. **No architecture is
forced** — these are additive crates plus a JS binding each.

**Risks, honestly:**

- `updater` needs a signing key and an update-manifest endpoint. `PROGRAM.md`
  already lists "Tauri distribution at KPMG — code signing and update channel
  need platform-team engagement early". The plugin removes the *code*, not the
  *KPMG platform conversation*. Start that conversation now; it is the long pole.
- `deep-link` on Windows writes registry entries at install time; on a locked
  SOE that is an MSI/packaging question, not a code question.
- Each plugin is a capability the security review must scope. `shell` and `fs`
  should **not** be adopted by reflex — `sandboxd` owns execution and there is no
  reason to hand the WebView a shell.

### `stronghold` vs `keyring` for `OD-6` token storage

`OD-6` says tokens live in **Windows Credential Manager**.
`tauri-plugin-stronghold` (MIT OR Apache-2.0, 2.4.0) is **not** that: it is IOTA
Stronghold, an encrypted file database with its own password. Adopting it would
quietly contradict a locked decision.

The right part is the **`keyring` crate** (crates.io `keyring` **4.2.0**,
published **2026-08-29**, 27,418,178 all-time downloads; crates.io metadata says
`MIT OR Apache-2.0`, GitHub's classifier says `Apache-2.0` — dual, take MIT;
repo `hwchen/keyring-rs` 768★, pushed 2026-09-15). One API over Windows
Credential Manager, macOS Keychain and Linux secret services. Since v4 the core
API is split into `keyring-core` with credential stores as separate crates, so
we can depend on **only the Windows store** and keep the surface small.

**Verdict: adopt `keyring`, reject `stronghold`.** Cost: a few hours behind a
small `TokenStore` trait, which is also what keeps the macOS/Linux door open.

### OAuth loopback (`OD-30`)

`FabianLars/tauri-plugin-oauth` (215★, pushed 2026-09-26, v2 branch; dual MIT /
Apache-2.0 — GitHub's classifier reports `Apache-2.0`) spawns a temporary
localhost server to catch an OAuth redirect, for providers that will not accept
a custom URI scheme. Its docs do **not** mention PKCE; it captures the redirect
and hands you the query string. PKCE is ours either way.

**Verdict: vendor, not adopt.** It is roughly "bind a loopback port, serve one
response, shut down". For a sign-in path a regulated customer's security team
will read line by line, a small vendored file with attribution beats a 215★
third-party crate in the auth critical path. Prefer `deep-link` with a
registered scheme where Entra/Keycloak allows it; keep loopback as the fallback.

---

## 3. Lists, logs and the no-scroll-jump rule

The design system is specific: *"Log panes batch appends at ~10Hz and never
scroll-jump when the user has scrolled up"* and *"Virtualise any list over 50
rows."* Nothing in the tree virtualises anything today.

Two clean options, both MIT, both read from npm on 2026-09-28:

**`virtua` 0.52.7** (MIT; `inokawa/virtua`, 3,746★, pushed 2026-09-27; **zero
production dependencies** — the manifest declares only peer and dev deps). Its
distinguishing features are exactly our problem: **reverse scroll** with correct
iOS/Safari behaviour, shift-preserving insertion above the viewport,
bi-directional infinite scroll, built-in scroll restoration. A log pane is a
reverse-scrolled list with items arriving at the bottom, and "does not jump when
content is inserted above the viewport" is the hard part virtua advertises
solving. ~3 kB gzipped.
*Risk: pre-1.0 (0.52.x); effectively a single principal maintainer.*

**`@tanstack/react-virtual` 3.14.13** (MIT; `TanStack/virtual`, 7,123★, pushed
2026-09-21). Headless — it returns measurements and you render. Stable major,
larger maintainer org, same family as TanStack Table if we later need a data
grid. Stick-to-bottom is ours to write (~30 lines: only auto-scroll while
`scrollHeight - scrollTop - clientHeight` is under a threshold).

**Recommendation: `@tanstack/react-virtual`.** Not because it is better at the
job — virtua is — but because "headless, and we own the 30 lines of
stick-to-bottom" matches the design system's insistence on owning behaviour, and
a stable major from a larger org is the right risk profile for software shipped
to regulated customers. If the team would rather not write that logic at all,
virtua is a defensible swap; both are MIT and moving between them is a day.

**Rejected here, with reasons:**

- **`react-virtuoso` 4.18.15.** Two problems.
  **(a) Provenance.** `api.github.com/repos/petyosi/react-virtuoso/license`
  returns **404**, and the repository root has **no LICENSE / LICENCE / COPYING
  file** (root listing read 2026-09-28). The MIT claim exists only in
  `package.json`. That is thinner evidence than a KPMG legal review should accept
  when two equally good MIT alternatives *do* ship a licence file.
  **(b) It is the exact permissive-core / commercial-add-on trap this review was
  told to look for.** virtuoso.dev states the core list, grid, table and masonry
  components are MIT while **`VirtuosoMessageList` — the chat / human-AI
  conversation component, i.e. precisely the one an agent cockpit reaches for —
  is under a commercial licence with its own EULA.** Adopting the MIT core puts
  the paid component one import away from a developer who does not know where the
  boundary is.
- **`@melloware/react-logviewer` 6.6.0 — MPL-2.0** (confirmed on npm and via the
  GitHub licence API; 169★, pushed 2026-09-27), a maintained fork of Mozilla's
  `react-lazylog` 4.5.3, also **MPL-2.0**. MPL-2.0 is **file-level copyleft**: we
  may ship it, but any modification to *its* files must be published under
  MPL-2.0. A log viewer is the one component we are guaranteed to modify (10 Hz
  batching, our heartbeat clock, our tokens, our provenance ids), so the
  obligation fires on day one and the usual "leave it unmodified" mitigation is
  not available. **CONDITIONAL; recommendation study-only** — read its
  follow/scroll logic, write our own on TanStack Virtual.
- **ANSI-to-HTML converters** (`anser` MIT 2.3.5, `ansi-to-html` MIT 0.7.2,
  `fancy-ansi` Apache-2.0 0.1.3) are permissive and fine, and we should still not
  need them. Terminal output goes to xterm. Reaching for an ANSI parser is the
  first symptom of something starting to derive meaning from PTY bytes, which the
  cross-cutting constraint forbids. **Reject on architecture, not licence.**

---
## 4. Command palette and keyboard engine — the two everyone assumes we should adopt

We should not. Both are already built, tested, and carry knowledge a generic
library does not have.

**Command palette.** `cmdk` (MIT, 12,991★) is the obvious candidate: genuinely
headless, styled entirely through `[cmdk-root]` / `[cmdk-item]` attribute
selectors, with filtering, ranking, groups and nested pages. It is a good
library. Three reasons it is still a reject here:

1. **`CommandPalette.tsx` + `commands.ts` already exist and are under test.**
   Swapping them is a lateral move that costs days and buys nothing.
2. **`cmdk`'s last push was 2025-10-29** (GitHub API, read 2026-09-28) —
   eleven months quiet, with 52 open issues. Compare `tinykeys` (pushed
   2026-09-25) or `virtua` (2026-09-27). Adding an eleven-month-stale dependency
   to a product with a five-year enterprise support horizon is the wrong
   direction.
3. The design system requires *"⌘K palette scoped to the current selection.
   Stable ranking; rows never reflow while filtering."* **Stable ranking is a
   design requirement that fights a general-purpose fuzzy ranker's defaults.** We
   would be fighting `cmdk`'s scorer to get a property we already have.

`kbar` (MIT, 5,255★, pushed 2026-08-10) has the same problems plus a heavier
opinion about how actions are registered.

**Keyboard engine.** `tinykeys` (MIT, 4,099★, pushed 2026-09-25, ~650 bytes) is
excellent and would be the right answer on a greenfield. Here it is
**study-only** for a concrete reason worth writing down:
`useKeyboardMap.ts` is not a generic chord matcher. It exists because of a bug
that a generic engine would reintroduce. `Terminal.tsx` documents it:

> xterm handles keydown on its own focused helper textarea, and for any key it
> maps to data it sends the bytes to the agent and CANCELS the event,
> stopPropagation included. So with a terminal focused … six of useKeyboardMap's
> chords never reached its window listener and went to the PTY as control bytes
> instead: ⌘3 as ESC, which interrupts the agent, and ⌘⏎ as CR, which submits its
> half-typed prompt.

The fix is `term.attachCustomKeyEventHandler(terminalKeepsKey)` where
`terminalKeepsKey` is *the map's own predicate*, so the two cannot disagree —
plus the `OD-9` "send shortcut keys to the terminal" toggle read per keystroke.
A window-level library like `tinykeys` would sit exactly where the cancelled
events never arrive. **Adopting it would delete the fix and the bug would come
back.** Worth reading `tinykeys`' chord parser (it is tiny and clean) if we ever
want sequence chords; not worth adopting.

`react-hotkeys-hook` (MIT 5.3.3, 3,503★), `hotkeys-js` (MIT 4.0.8),
`@github/hotkey` (MIT 3.1.4) — same verdict, more weight.

---

## 5. Pane layout — the one where the obvious answer is wrong

The task brief asked about "resizable/dockable pane layouts and tiling grids".
I read both layout files before answering, and the honest finding is that
**none of the layout libraries replace what is there**, because what is there is
not a split view.

- **`ColumnsLayout.tsx` (73 lines)** is a horizontally scrolling strip of
  fixed-width panes, with two focus effects: a new session takes focus, and the
  focused pane is scrolled into view (`inline: center`, `block: nearest`). There
  is no resizing at all. A splitter library replaces none of it.
- **`CanvasLayout.tsx` (405 lines)** is a free-form 2400×1600 plane of
  absolutely-positioned tiles with drag-move, corner-resize, a z-stack and
  persisted geometry. Its central design decision is that **React does not own a
  tile's geometry** — transform/width/height are written through a ref during a
  gesture so the 2 s poller landing mid-drag cannot snap the tile back. Every
  React layout library owns geometry through state. Adopting one means giving up
  that property and reintroducing the snap-back.

So:

| Library | Licence (verified 2026-09-28) | Verdict and why |
|---|---|---|
| `react-resizable-panels` 4.14.1 | MIT (npm) | **adopt later, as a gap-fill.** `--rail-w` is a hard-coded `248px` in `shell.css`; a resizable rail and any future split lens are real features it would serve. Caveat found in its own docs: *"The default inline styles cannot be overridden, except for overflow"* — acceptable for a splitter, not for a component library. |
| `dockview` 8.3.1 | **Mixed monorepo.** I fetched `LICENCE.md` at `raw.githubusercontent.com/dockview/dockview/master/LICENCE.md`: `dockview-core`, `dockview`, `dockview-react`, `dockview-vue`, `dockview-angular` are MIT; **`dockview-enterprise` is "Proprietary — commercial licence"**. GitHub's classifier reports `NOASSERTION` for the repo because of that mixture. | **reject.** Overshoots the product (we do not want VS Code docking), ships its own themes, and is exactly the permissive-core/commercial-add-on shape this review was told to catch. If it is ever reconsidered, the CI licence gate must pin the *package*, not the repo. |
| `allotment` 1.20.5 | MIT (npm; 1,260★, pushed 2026-09-27) | **reject.** Derived from VS Code's split view and ships `allotment/dist/style.css`. It would import VS Code's visual language into a product whose differentiator is not looking like everything else. |
| `react-mosaic-component` 7.1.0 | **Apache-2.0** — read from the repo `LICENSE`: *"Copyright 2019 Kevin Verdieck, originally developed at Palantir Technologies"*. GitHub's classifier says `NOASSERTION`; the file is plain Apache-2.0 with no extra clauses. | **reject.** Tiling-window model, not our canvas model; brings Blueprint-flavoured chrome. |
| `rc-dock` 4.0.2 | Apache-2.0 (npm) | **reject**, same reasons as dockview. |
| `flexlayout-react` 0.11.1 | MIT (npm) | **reject**, same. |
| `@dnd-kit/core` 6.3.1 | MIT (npm) | **reject for Canvas** — it owns transforms through React state, which is the exact property `CanvasLayout` was written to avoid. Reconsider only for *list* reordering (rail, merge queue), where `Motion`'s `layout` (already in the design system stack) is likely enough. |
| `@atlaskit/pragmatic-drag-and-drop` 4.0.0 | Apache-2.0 (npm) | **study-only.** It is the one drag library that does *not* own your geometry — it is an event/adapter layer over native HTML5 drag-and-drop. That matches `CanvasLayout`'s philosophy, but Canvas already works. Worth remembering for phase 3's merge-ordering UI. |

**Net: the layout section saves nothing today, and saying so is the finding.**

---

## 6. Charts

Nothing charts today; phase 6 (quota, spend, per-session attribution) and the
phase 2 live meters will need it. The design system is hostile to chart
libraries: colour is signal not decoration, the accent is reserved for
interactive state and **"never on cards, icons, borders or chart fills"**, live
numbers are `tabular-nums`, motion must explain causality.

**Adopt `uPlot` 1.6.32** (MIT; `leeoniya/uPlot` 10,518★, pushed 2026-09-27).
~50 kB minified, Canvas 2D, built for streaming time series — its own benchmark
claims 166,650 points in 25 ms cold and roughly 100k points/ms after, and it
beats Chart.js and ECharts on CPU and memory during live update. It imposes no
visual language: you pass stroke, fill, width and dash per series, so our tokens
go straight in. Streaming update and multi-series-with-toggle are first-class.
*Risks:* the API is imperative and terse (a thin React wrapper is ~60 lines, and
several exist); the docs are demo-driven rather than prose; accessibility is
ours to add. Those are acceptable for meters and spend charts. **It is also the
only charting option whose performance profile survives ten live panes.**

**Study-only: `visx` 4.0.0** (MIT; `airbnb/visx` 21,064★) — genuinely unstyled
D3 primitives, modular packages, deliberately no built-in animation. The right
shape for us. The problem is maintenance: **last push 2026-06-22**, three months
quiet at the time of writing, against uPlot's same-week activity. Use it as a
reference for scale/axis maths; do not make it a dependency until it moves again.

**Reject: Observable Plot** 0.6.17 (ISC; 5,390★, pushed 2026-09-01) — excellent
library, wrong job: a grammar of graphics carries opinionated defaults (its own
type scale, tick style, legend chrome) that we would spend the savings undoing.
**Reject: `recharts`** 3.10.1 (MIT; 27,593★) — SVG node per data point, which is
the wrong performance shape for live meters, and a strong house style.

---

## 7. Headless component primitives — and why we are not adopting a component library

We already hand-wrote `Popover.tsx`, `ContextMenu.tsx`, `Toasts.tsx` and
`StatusDot.tsx`. The design system already names the intended stack: Tailwind v4
+ shadcn/ui (MIT, "we own the code"), our own token layer, Motion v13, Kibo UI
for composites. That decision is made; this section only says what *else* is
worth knowing.

| Candidate | Licence (verified) | Verdict |
|---|---|---|
| `@floating-ui/react` 0.27.20 | MIT (npm; `floating-ui/floating-ui` 32,753★, pushed 2026-09-23) | **adopt, narrowly.** Positioning only: collision detection, flip, shift, arrow, and the interaction hooks. It imposes no styling and no DOM. Our `Popover.tsx` and `ContextMenu.tsx` keep their markup and their tokens and stop owning viewport-edge maths, which is the part that is always subtly wrong. This is a real saving with no design cost. |
| Base UI `@base-ui-components/react` **1.0.0-rc.0** | MIT (npm; `mui/base-ui` 11,007★, pushed 2026-09-27) | **study-only.** Fully unstyled, built by the people behind Radix, Floating UI and MUI, and it is where that community is heading. But the npm version is a **release candidate**, and we do not put an RC on the critical path of a regulated-enterprise product. Re-evaluate at 1.0 stable. |
| Radix Primitives | MIT, "Copyright © 2022-present WorkOS" (19,338★, pushed 2026-08-08) | **study-only.** Mature and unstyled. Note the stewardship change — Radix is now maintained by **WorkOS**, and its principal authors are on Base UI. Not a reason to avoid it; a reason not to build the design system's foundations on it now. shadcn/ui (which we do use, as copied code) sits on Radix anyway, so we inherit it by that route and own the copy. |
| Ark UI `@ark-ui/react` 5.39.2 | MIT (npm; `chakra-ui/ark` 5,398★, pushed 2026-09-27) | **reject.** Unstyled and good, but it is a state-machine framework (Zag.js) underneath — a second architecture for component behaviour alongside zustand. A wall of configuration is exactly what the product's wedge says we must not have. |
| `react-aria-components` 1.21.1 | **Apache-2.0** (npm; `adobe/react-spectrum` 15,894★, pushed 2026-09-27) | **study-only.** Apache-2.0 is allowed. The best accessibility work in the ecosystem, and the design system commits to keyboard as a primary input. But it brings Adobe's interaction semantics wholesale. **Read `react-aria`'s focus-management and `:focus-visible` behaviour and copy the ideas.** |
| `@headlessui/react` 2.2.10 | MIT (npm) | **reject.** Narrower than Base UI, and tied to Tailwind Labs' release cadence. |
| `sonner` 2.0.8 (MIT), `vaul` 1.1.2 (MIT) | MIT (npm) | **reject.** Both are opinionated *visual* components (toast stack, drawer). `Toasts.tsx` exists. A toast library that brings its own motion curve fights the house curve `cubic-bezier(0.23, 1, 0.32, 1)`. |
| `react-arborist` 3.16.0 | MIT (npm; `brimdata/react-arborist` 3,746★, 65 open issues) | **study-only** for the `FilesLens` tree. Virtualised, drag-drop, inline rename, multi-select, keyboard — the full list. But it is **not** headless: it ships default renderers and you opt out of them component by component. Given `FilesLens` is an unbuilt placeholder, revisit when phase 3 gets there; it is the strongest candidate in its category and the only reason it is not an adopt is that we have not designed the tree yet. |
| `geist` font package | npm metadata reads `"SIL OPEN FONT LICENSE"` (OFL) — matches the design system's "Geist Sans (OFL, free)" | **adopt** when typography lands. Note the npm `version` field for this package is malformed in the registry (`"changeset version"`), so pin by a real published version after checking, not by `latest`. |

**The general rule this section is defending:** the design system exists because
the first prototype "failed by looking machine-generated". Every component
library that ships CSS is a machine-generated look, chosen by someone else. We
adopt *behaviour* (positioning, virtualisation, terminal emulation) and we write
*appearance*. That is not purism; it is the product's only differentiator.

---

## 8. Forward-looking: phase 3 and phase 5 primitives (not needed yet)

Verified now so the licence question is settled before anyone is in a hurry.

- **`shiki` 4.4.3** — MIT (npm; `shikijs/shiki` 13,832★, pushed 2026-09-11).
  TextMate-grammar syntax highlighting, used for diff review (phase 3) and code
  frames on the canvas (phase 5). Its own `LICENSE` is MIT (Pine Wu 2021, Anthony
  Fu 2023). **Caveat I could not close: Shiki bundles a large set of third-party
  TextMate grammars and themes, and those carry their own licences.** The root
  `LICENSE` I read does not enumerate them. Before adoption, enumerate the
  grammars actually loaded (we need perhaps six languages, not two hundred) and
  check each — a bundle-size win and a licence-hygiene win in the same pass.
- **`@codemirror/view` 6.43.13 and `@codemirror/merge` 6.12.2** — MIT (npm).
  `@codemirror/merge` is a real unified/split diff view with editable merge. For
  phase 3's "diff review with transcript back-links", this is the strongest
  permissive option, and CodeMirror is modular enough that we do not import an
  IDE. Monaco (MIT) is the alternative and is far heavier.
- **`diff2html` (MIT)** — registry metadata for `latest` is corrupted (the
  `version` field literally reads `"git add -A package.json"`), so **pin an
  explicit version and re-verify** rather than trusting `latest`. Flagged, not
  rejected.
- **`react-diff-viewer-continued` 4.4.0** — MIT (npm). Simpler than CodeMirror;
  ships its own styles. Study-only.
- **`@tanstack/react-table`** — MIT (npm). Headless table logic for the
  provenance ledger and release lists in phase H. Adopt if a table with sorting,
  grouping and column sizing is needed; it renders nothing, which is the point.

---
## 9. Can any of this let us DELETE existing phase 2 code?

The brief asked specifically. The honest answer is **very little, and that is a
good sign, not a bad one** — it means phase 2 built the parts that do not exist
off the shelf.

| Existing code | Could a library delete it? | Answer |
|---|---|---|
| `Terminal.tsx` (178) | partly | **No deletion, but a rewrite of ~20 lines pays for itself.** The `ResizeObserver`→`fit()` wiring stays. The renderer choice becomes `loadAddon(new WebglAddon())` + fallback chain. Net line count goes *up* slightly; net capability goes up a lot. |
| `api/attachRegistry.ts` | partly | **Shrinks.** `@xterm/addon-serialize` removes any hand-rolled screen-restore path on reattach. It does **not** replace the registry itself — one-terminal-per-session, read-only mode and re-parenting between Columns and Canvas are ours. **Do not adopt `@xterm/addon-attach`:** it is a naïve "pipe the socket into the terminal" helper with no reconnect, no read-only mode and no backpressure. It would be a downgrade. |
| `CommandPalette.tsx` (145) | yes, technically | **No.** See §4. Lateral swap onto an 11-month-stale dependency. |
| `useKeyboardMap.ts` (243) | yes, technically | **No.** See §4. The xterm `attachCustomKeyEventHandler` fix is the reason this file exists, and a window-level library sits where the cancelled events never arrive. |
| `ColumnsLayout.tsx` (73) | no | It is a scroll strip with focus effects, not a split view. |
| `CanvasLayout.tsx` (405) | no | Its core property — React does not own tile geometry during a gesture — is one every React layout/drag library violates. |
| `components/common/Popover.tsx` | **partly — yes** | **The only genuine deletion in the list.** Hand-rolled viewport-edge/flip/shift maths goes to `@floating-ui/react`; the markup and tokens stay. Same for `ContextMenu.tsx`'s positioning. |
| `hooks/heartbeat.ts` | no | One global heartbeat clock is a design-system invention. No library has it, and that is the point. |

**Interpretation.** At ~11% built, the parts that *are* built are mostly
product-specific. The savings are in the ~89% not yet written — desktop
plumbing, virtualisation, charts, terminal features — which is exactly where
this report points.

---

## 10. Copyleft and non-permissive projects found — recorded, not read

Listed so the record shows we looked. **No source from any of these was opened.**

| Project | Licence | Relevance | Why excluded |
|---|---|---|---|
| Paperdesign (`~/Projects/Paperdesign`) | **AGPL-3.0-only** | design canvas (phase 5) | Already withdrawn by the owner (`OD-11`). §13 Remote Network Interaction fires under a hosted model. Not opened. |
| `kgoedecke/doop` | AGPL (per `OD-11`) | design canvas | Same. Not opened. |
| tldraw | **Custom commercial licence** — GitHub's classifier reports `NOASSERTION` (50,608★, pushed 2026-09-27), i.e. it matches no OSI licence template | canvas | Named as excluded in `CLAUDE.md` and `DESIGN-SYSTEM.md`. Confirmed independently: a `NOASSERTION` on a 50k-star project is the signature of a hand-written licence, which is always a field-of-use question. |
| Excalidraw | **MIT** (133,105★, pushed 2026-09-27) — *permissive, and therefore NOT excluded* | canvas | Listed here only to correct a likely assumption: Excalidraw is MIT and is a legitimate phase 5 reference. Its scene model is hand-drawn-shape-oriented rather than real-HTML-frame-oriented, so it does not fit `OD-11`'s brief — but reading it is allowed. |
| coss.com/ui (formerly Origin UI) | **AGPL-3.0** | component library | Already flagged "blocked pending legal" in `DESIGN-SYSTEM.md`. |
| `dockview-enterprise` | **Proprietary, commercial** | docking layout | Sibling of the MIT `dockview` packages in one monorepo. Verified by reading `LICENCE.md` at the repo root. |
| `VirtuosoMessageList` (virtuoso.dev) | **Commercial, own EULA** | AI-conversation list | Permissive-core / paid-component pattern; the paid part is the one an agent cockpit wants. |
| `@melloware/react-logviewer`, `react-lazylog` | **MPL-2.0** | log viewer | CONDITIONAL, not forbidden. Excluded because we will modify the files, which triggers the file-level obligation. |
| kitty | GPL-3.0 | terminal emulator | GPL. Recorded only. |
| asciinema | GPL-3.0 (CLI historically; verify per component) | terminal session recording | GPL. Recorded only. The **asciicast file format** is a published spec, not code — a format is not a licence risk, and writing our own asciicast writer is permitted and cheap. |
| Aceternity UI, React Bits | non-OSI | component libraries | Already excluded in `DESIGN-SYSTEM.md`; their visual signature *is* the AI-generated look. |
| GSAP | non-OSI for the parts we would want | animation | Already excluded in `DESIGN-SYSTEM.md`. |
| AG Grid | GitHub classifier `NOASSERTION` (15,617★, pushed 2026-09-26) — MIT community packages and a **commercial enterprise** tier in one repository | data grid | Same shape as dockview. If ever reconsidered, gate on the package. Not needed: `@tanstack/react-table` is MIT and headless. |
| Highcharts | GitHub classifier `NOASSERTION` (12,492★) — free for non-commercial, **paid for commercial** | charts | Field-of-use restriction. Forbidden by the licence rule. |
| Handsontable | non-commercial-only free tier | data grid | Field-of-use restriction. |
| Apache ECharts | **Apache-2.0** (67,404★) — *permissive, not excluded* | charts | Recorded for completeness: it is legally fine. Rejected in §6 on design and weight, not licence. |

**Two further notes for the CI licence gate `OD-11` calls for:**

1. **Gate on the package, not the repository.** dockview proves the case: the
   repo classifier says `NOASSERTION` and the published `dockview` package says
   MIT, and both are correct. A repo-level gate would either block a usable MIT
   package or wave through a proprietary sibling.
2. **Gate must fail on a missing licence file, not only on a bad one.**
   react-virtuoso proves that case: `package.json` says MIT and there is no
   licence file in the repository at all.

---

## 11. Fetches the corporate proxy or the API blocked

Stated so nothing here reads as verified when it was not.

| Target | Outcome |
|---|---|
| `https://www.npmjs.com/package/@xterm/addon-webgl` (and npmjs.com generally) | **HTTP 403 Forbidden** through the fetch tool. Worked around via `registry.npmjs.org/<pkg>/latest`, which returns the same `license` field from the registry itself. |
| `api.github.com/repos/...` unauthenticated | **HTTP 403 rate-limited** after roughly 25 repositories. Worked around with the authenticated `gh` CLI. Rows in my scratch file after that point are blank; every figure quoted in this document comes from a call that returned data. |
| `raw.githubusercontent.com/petyosi/react-virtuoso/master/LICENSE` | **404** — and this is a *finding*, not a proxy failure: the repository genuinely has no licence file (root listing confirms it). |
| `raw.githubusercontent.com/mathuo/dockview/master/LICENSE` | **404** — the file is `LICENCE.md` (British spelling) and lives at `dockview/dockview`. Retrieved successfully via `gh api repos/mathuo/dockview/license`. |
| `curl` to `raw.githubusercontent.com` from the shell | Works, but the first TLS handshake through the intercepting proxy took **over two minutes**. Every shell fetch in this pass had to be backgrounded. Budget for that in any future licence-audit tooling; do not assume a 30 s timeout is enough on this network. |

## 12. What I could NOT verify

- **Shiki's bundled grammar and theme licences.** The root `LICENSE` is MIT and
  enumerates no third-party notices. Shiki ships a large set of TextMate grammars
  from many upstreams. **Unresolved. Must be closed before phase 3 adopts it.**
- **A Go server-side terminal emulator.** `@xterm/headless` is MIT and proven,
  but the control plane is Go. I did not verify `hinshun/vt10x` or
  `charmbracelet/x/vt` to this report's standard. **Open item.**
- **uPlot's latest release date.** npm reports 1.6.32; the repository was pushed
  2026-09-27 and has 10,518★. I did not read a dated release tag, so "1.6.x, repo
  active this week" is what the evidence supports, not "released recently".
- **`@xterm/addon-web-fonts` and `addon-unicode-graphemes` versions/licences.**
  Both are listed in the xterm.js README as published addons from the same MIT
  repository; I did not fetch their individual npm records. `addon-progress`
  0.2.0 MIT **was** verified.
- **Bundle sizes.** Every size figure here is the project's own claim (uPlot
  "~50 kB min", virtua "~3 kB gzipped"). I measured nothing.
- **Whether Base UI's RC has since gone stable.** npm `latest` on 2026-09-28 is
  `1.0.0-rc.0`.
- **`vercel/geist-font`'s repository licence.** The npm `geist` package declares
  `"SIL OPEN FONT LICENSE"`, and its registry `version` field is corrupt
  (`"changeset version"`). Pin a real version and read the repo's OFL text before
  shipping the font.

---

## 13. Recommended order of work

Ranked by saving-per-hour, most valuable first. Nothing here is a decision; the
owner reviews it.

1. **xterm addons — `webgl` (+ canvas/DOM fallback), `serialize`, `search`,
   `unicode11`, `web-links`, `clipboard`.** ~1–2 days total. No new vendor, no
   new licence, immediate capability. Do `webgl` first and make the
   context-loss fallback part of its acceptance test.
2. **Tauri plugins — `single-instance`, `window-state`, `deep-link`, `log`,
   `store`, `notification`, `os`, `process`.** ~1–2 days. Pure gap-fill. In
   parallel, open the **code-signing and update-channel conversation with the
   KPMG platform team**, because `updater` cannot ship without it and that is a
   people-lead-time item, not a code item.
3. **`keyring` crate behind a `TokenStore` trait** for `OD-6`. Half a day. Also
   settles that `stronghold` is the wrong plugin before someone adopts it.
4. **`@tanstack/react-virtual`** (or virtua) for the log pane and the rail,
   with our own 30-line stick-to-bottom. ~1 day, and it is a design-system
   requirement that currently has no implementation anywhere in the tree.
5. **`@floating-ui/react` inside the existing `Popover.tsx` / `ContextMenu.tsx`.**
   Half a day, and it is the one place where adopting a library genuinely deletes
   fiddly code we own.

Then, when the phase needs it: `uPlot` (phase 6), `@codemirror/merge` + `shiki`
(phase 3, after closing the grammar-licence question), `react-resizable-panels`
(whenever a resizable rail is asked for), `react-arborist` (phase 3 `FilesLens`).

**Do not do:** swap the command palette, swap the keyboard map, adopt any
docking/tiling library, adopt any styled component library, adopt
`@xterm/addon-attach`, or adopt `tauri-plugin-stronghold`.

---

*End of 07-ui-primitives.md. Licence facts are as at 2026-09-28 and should be
re-verified at adoption time; a licence can change between a research pass and a
`package.json` line.*

---

## Appendix A — adjacent whole products, and why none of them is a shortcut

The search deliberately looked for someone who has already built this, because
adopting a working cockpit would beat adopting ten primitives. Metadata below
read via the authenticated `gh` CLI on 2026-09-28.

| Project | Licence | ★ | Created | Last push | Verdict |
|---|---|---|---|---|---|
| `Surething-io/cockpit` ("OpenCockpit", the open-source Claude Code GUI — parallel multi-project sessions, multi-engine BYOK, terminal) | MIT | 37 | 2026-02-06 | 2026-09-23 | **study-only** |
| `timwukp/agent-terminal` (crash-proof PTY reattach for long-running agents: daemon holds PTY + screen state, client reattaches with exact restore) | MIT | 0 | 2026-08-04 | 2026-09-24 | **study-only** |
| `kochul2000/laymux` (terminal multiplexer for agentic coding; `portable-pty` + xterm.js 6, ratio-based grid) | **none declared** (`license: null`) | 2 | 2026-03-25 | 2026-09-27 | **reject — no licence means no permission** |
| `awaysu/AwayTerminal2` (Tauri 2 + Rust + xterm.js terminal) | `NOASSERTION` (classifier could not match) | 0 | 2026-09-26 | 2026-09-27 | **reject** |

**None of these is adoptable, and the reason is the same for all four.** They
are small, young, single-author projects (0–37 stars), and more importantly they
solve the *cockpit* half while having no answer at all to the parts that make
this product a product: the Define→Design→Develop→Deploy journey, the provenance
thread, Kata microVM isolation, Keycloak/Entra identity, or a control plane a
regulated customer can audit. Adopting one would be adopting the easy half.

Two are worth *reading*:

- **`agent-terminal`** states the same problem `attachRegistry.ts` solves —
  "daemon holds PTY + screen state, client reattaches with exact restore". If
  its approach differs from ours, that is a free second opinion on a piece of
  phase 2 that is hard to test. MIT, so reading it carries no licence risk.
- **`laymux`** uses `portable-pty` with xterm.js 6 and a ratio-based grid.
  **Do not read it**: it declares no licence at all, which means no rights are
  granted, and "no licence" is legally more restrictive than GPL, not less.
  Recorded here so nobody rediscovers it and assumes public equals permissive.

### Permissively-licensed terminal emulators worth reading, not adopting

Verified 2026-09-28: **Alacritty** Apache-2.0 (65,836★, pushed 2026-08-31),
**Zellij** MIT (35,567★, pushed 2026-09-25), **Rio** MIT (7,554★, pushed
2026-09-27), **Ghostty** MIT (61,617★, pushed 2026-09-27), **wezterm**
(classifier reports `NOASSERTION`; 29,042★, pushed 2026-09-21 — read its
`LICENSE.md` before using any of it). All are native renderers, so none replaces
xterm.js in a WebView. Their value to us is as references for VT edge cases,
scrollback data structures and glyph-atlas design — and Zellij in particular for
pane-multiplexing semantics, which is the closest analogue to Columns/Canvas
that exists in a mature codebase.

**kitty is GPL-3.0** (35,096★) and **asciinema is GPL-3.0** (17,841★). Recorded
by name only; no source read.
