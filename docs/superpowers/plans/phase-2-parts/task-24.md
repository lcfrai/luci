### Task 24: Wire the cockpit to the live control plane, and prove the ten

This is the last task of Phase 2 and the first one that reads from the control plane. Everything before it built a substrate that renders whatever is in the store, and **nothing before it ever loads a catalogue into it**: `grep -rnE "\bapi\.[a-zA-Z]+\(" src` across tasks 14 to 23 finds `api.createSession` twice — in task 18's `hooks/useCreateSession.ts` and in task 20's `startSessionOnWorktree` — and the session verbs in task 22's `sessionVerbs.ts` (resume, fork, restart, rename, kill, purge, `setWorktreeReady`), plus task 19's poller, which takes its client injected and fetches sessions and nothing else. (The character class has to be `[a-zA-Z]`: every method on the client is camel-case, so a lowercase-only class matches none of them.) Every one of those acts on a row the user already has in front of them. **Nothing anywhere *lists* a machine, a project, a branch, a worktree or a harness.** So as the tree stands after task 23 the rail renders an empty store forever, the session picker offers no harness, and the split New-session button has no worktree to create against. **The catalogue load is this task's, it is not optional, and it is the reason the title says "wire" before it says "prove".** §9's one-line scope for this row says "`App.tsx` rewritten to mount `AppShell` + start the poller + load the catalogue"; the third clause is doing more work than the other two.

The second half is the phase's evidence, in two files. `src/defects.test.tsx` carries the ten known defects of contract §8 — a manifest that fails if any of the ten fixes is removed from the implementation, and a set of composition tests that assert in the assembled app what the unit suites can only assert in isolation. `src/styles/designsystem.test.ts` is a **mechanical** audit of the whole source tree: no literal hex, no `transition: all`, no banned hue, no colour outside the token layer. Mechanical is the entire point. `docs/DESIGN-SYSTEM.md` exists because the first prototype failed by looking machine-generated, and a human eye cannot grep a tree for the one `#8b5cf6` that undoes it. The audit is also what deletes `App.css`: that file is still Vite's React template, 116 lines and 18 hex literals, two of which — `#646cff` and `#535bf2` — are indigo and violet, the same family as the pair `docs/DESIGN-SYSTEM.md` bans by name in its first line, though not literally that pair. Nothing imports it. It has been sitting in the repository since scaffold, and the audit written in step 2 is what finally makes its presence a failing test rather than a thing nobody noticed.

**This is also where a missing CORS grant would have surfaced, and it will not, because task 9 shipped it.** `contract-go.md` §2.12 specified an `OriginPolicy` that rejects; task 9's plan resolved that the middleware must **grant** as well, echoing the request's `Origin` verbatim with `Vary: Origin` and answering preflight `OPTIONS` with 204 before the mux. A Tauri webview is a browser and every build is cross-origin against `http://127.0.0.1:8730`. WebSockets are exempt from both preflight and the `Access-Control-Allow-Origin` requirement, which is exactly why Phase 1's attach relay worked with none of this and why the omission would not have shown itself until the first `fetch` — which is `startCatalogue`, in step 6 of this task. Step 11 proves the grant against a running daemon with `curl`, because no jsdom test can: the client's `fetch` is injected and never crosses an origin.

**A note on the expected-output blocks in this file, read it before you judge one.** As in task 19, no command in this plan has been run: `desktop/node_modules` contains no vitest until task 14 installs it, and the corporate network blocks the registry from this machine. Where a transcript appears below it is the *shape* the tooling prints, and the named assertion is the part that matters. Where I am not confident of the chrome I say so in prose instead of inventing it. Do not edit a test to match a transcript in a document.

Facts from Groups B and C override the UI contract, and every one of them is load-bearing here. `GET /v1/harnesses` returns `[]` and never `null` — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test — so this task writes no defensive `?? []` and the audit greps for one. The error-code union is **eighteen** members: `contract-go.md` §3.2's fifteen (the UI contract reproduces them in §4.2), Group C's ruling-7 sixteenth `non_loopback_publish`, and the two the client raises itself, `network_unavailable` and `timeout`. And `SessionStatus` has four members, not `docs/AGENTIC-UX.md` §4 rule 8's seven; UI contract §0 keeps only the mechanically derived ones, because five of the seven need the agent-runtime signal that document itself calls a hard runtime dependency. The last block of `defects.test.tsx` pins each of them, because this is the file a later phase will read to find out what Phase 2 actually guaranteed.

**Files:**
- `desktop/src/App.tsx` — rewritten: mounts `<AppShell/>`, starts the one poller, and owns every catalogue fetch — machines, projects and harnesses once, branches and worktrees on navigation (modify)
- `desktop/src/main.tsx` — imports `./styles/app.css`, which nothing imports today (modify)
- `desktop/src/App.css` — **deleted.** Vite's React template: 116 lines, 18 hex literals — two of them indigo and violet, though not the literal pair the tell grep names, which is step 3's point — a `transition: 0.75s` with no property and an `rgba()` box-shadow. Nothing imports it (delete)
- `desktop/src/defects.test.tsx` — the ten-defect manifest, the assembled-cockpit composition tests, and the three cross-group facts the UI contract predates (new)
- `desktop/src/styles/designsystem.test.ts` — the mechanical design-system audit over the whole `src` tree (new)

**Interfaces:**
- Produces:
  - `App` (default export) — `(): JSX.Element`. Renders `<AppShell/>` and nothing of its own
  - `App.startCatalogue(client, store) → stop()` — **an addition the contract does not name**, argued in step 6: the once-per-mount load of machines, projects and harnesses, plus a store subscription that fetches branches and worktrees on navigation, per §4.7's "fetched on navigation and on an explicit refresh, not on the poll"
- Consumes: task 14's `stripCssComments` / `findHexLiterals` from `src/test/fakes.ts`, and `FakeTerm` / `FakeWebSocket` from the same file; the six builders `aMachine` / `aProject` / `aBranch` / `aWorktree` / `aSession` / `aHarness`, which **task 15** fills into task 14's `src/test/fixtures.ts` at the point task 14 reserves (shared shape §3.1 — not task 14, which cannot type them, and not task 17, which runs after the task that first imports them). This task declares none of the six and needs no seventh; task 16's `api` singleton, `CockpitClient` and `ApiError`; task 17's `useCockpit`, `resetCockpit` and `CockpitState`; task 18's `AppShell`; task 19's `startPolling`, `configureAttachRegistry` and `__resetAttachRegistry`; tasks 20 to 23 transitively, through `AppShell`
- Required by: nothing. This is the last task in Group D and in Phase 2
- **Adds no selector and no fixture.** Shared-shape §3 and §4 govern both: `fixtures.ts` and `selectors.ts` belong to tasks 14 and 17, neither is in §9's row for this task, and everything this task reads is already declared. The navigation loader reads `expandedProjects`, `expandedBranches` and `selection` straight off `getState()` inside a subscription rather than through a selector, because it is not a render path — a selector exists to keep two components from deriving the same thing differently, and there is no second consumer
- **Amends contract §4.7**, which says the poller "starts on mount of `AppShell`". §9's row for this task puts it in `App.tsx`. They are the same lifetime — `App` renders `AppShell` and nothing else, so an effect in `App` starts when `AppShell` mounts and stops when it unmounts — and `App.tsx` is the one of the two files this task may modify. No behaviour differs
- **Declares one gap against contract §4.7**, which says the catalogue is "fetched on navigation and on an explicit refresh, not on the poll". This task implements the navigation half and **not** the refresh half, and that is a declared departure rather than an oversight: no surface in Group D ships a refresh affordance — §6 declares none on the rail, the sub-rail, the top bar or the palette — so there is nothing to wire it to, and adding one would mean editing a file task 18 owns, which §9's row and shared shape §1 both forbid this task. It is the one clause of §4.7 Phase 2 does not deliver, and it is named here rather than left for a reader to find. The loader is shaped to take it later: it asks "is this list absent from the store", so a Phase 3 `invalidate(key)` action that deletes the key from `branchesByProject` / `worktreesByBranch` **and** drops it from the failure memo is the whole of what a refresh button needs behind it
- **Departs from contract §6's file tree**, which annotates `main.tsx` as "unchanged". §9's row for this task says `M desktop/src/main.tsx (imports app.css)`, and §9 is authoritative for the file list. Task 14 created `src/styles/app.css` and no file imports it; without this one line the entire reset, the scrollbars and the body type never load

- [ ] **Step 1: Check what this task assumes, before writing a line**

Every one of these belongs to another task. If one is false, **stop and report it to the orchestrator rather than patching that task's file** — shared-shape §1 exists because Group C produced nine Criticals by doing the opposite, and this task is the last one in the phase, so a silent local patch here is a patch nobody reviews.

**Every guard below is anchored, per shared shape §3A**, and (b) and (d) were not in this
file's first draft. Both were red against a correct tree. (b) counted task 14's six *commented*
builder signatures alongside task 15's six real ones and returned **12**, not 6 (task 14's
reserved point spells each one out as `//   export function aMachine(over?: Partial<Machine>):
Machine { ... }`). (d) was satisfied by the first line of `app.css` itself, which is
`/* desktop/src/styles/app.css`, over a tree in which nothing imports the file. An anchored
guard matches a line whose first non-space character begins the code, so no comment can stand
in for it.

```bash
cd desktop

# (a) The symbols this task imports, each from the task that owns it.
grep -cE '^export function AppShell\b'     src/components/AppShell.tsx       # expect 1
grep -cE '^export function startPolling\b' src/api/poller.ts                 # expect 1
grep -cE '^export const api\b'             src/api/index.ts                  # expect 1
grep -cE '^export type ProblemCode\b'      src/api/client.ts                 # expect 1
grep -c  'non_loopback_publish'            src/api/client.ts                 # expect >= 1

# (b) The six fixture builders of shared shape §3.2, which TASK 15 fills into task 14's file
#     at the point task 14 reserves. defects.test.tsx uses all six and §3.2 forbids a seventh.
#     Anchored at column 0, or task 14's reserving comment is counted a second time.
grep -cE '^export function a(Machine|Project|Branch|Worktree|Session|Harness)\(' \
  src/test/fixtures.ts                                                       # expect 6

# (c) Task 14's two CSS helpers, which the audit imports rather than re-implementing.
grep -cE '^export function (stripCssComments|findHexLiterals)\b' src/test/fakes.ts  # expect 2

# (d) app.css exists and no module imports it yet. Both halves matter: the first is task 14's
#     deliverable, the second is what step 7 fixes. The second grep matches the IMPORT
#     STATEMENT rather than the path, for the reason above.
test -f src/styles/app.css && echo "app.css: present"
grep -rnE '^import "\./styles/app\.css";' src --include=*.ts --include=*.tsx  # expect no output

# (e) Nothing loads the catalogue yet. This is the claim the first paragraph makes; if it
#     prints a listMachines or listProjects call, another task took the work and this
#     task's step 6 must be re-scoped rather than duplicating it.
grep -rnE "\.(listMachines|listProjects|listBranches|listWorktrees|listHarnesses)\(" src \
  --include=*.ts --include=*.tsx | grep -v "\.test\." | grep -v "src/api/client.ts"
                                                                             # expect no output

# (f) Shared shape §3B: every stylesheet but the token layer imports the token layer. Step 2's
#     audit asserts the strict form (first non-comment line); this is the cheap version, run
#     here so a §3B miss is caught before step 3 spends a red run on it. App.css is excluded
#     because it is the file step 7 deletes and step 3 is where its missing import is the
#     point. Anything else this names belongs to the task that wrote that stylesheet, not to
#     this one — report it, per the paragraph at the top of this step.
find src -name '*.css' ! -name 'tokens.css' ! -name 'App.css' \
  -exec grep -L '^@import "' {} +                                            # expect no output
```

Read these rather than run them. `src/App.tsx` should be task 19 step 12's eleven-line placeholder whose comment names this task — if it is still Phase 1's `ws://` harness, task 19 is incomplete and `<Terminal/>` still takes a URL. And `src/App.css` should still be Vite's template; step 3's failing transcript is transcribed from the real file and will not match if someone has already tidied it.

- [ ] **Step 2: Write the failing design-system audit**

**It reads from disk, and that is a decision with a reason.** A `?raw` import is static, so the set of stylesheets this audit covers would be a hand-written import list, and a stylesheet a later phase adds is audited only if somebody remembers to add a line for it — an audit that silently covers less than the tree is worse than no audit at all. `readdirSync` walks `src` instead, so a new stylesheet is in the audit the moment it exists, and the bytes asserted on are the bytes a reviewer would open, with no transform in between. The blanking hazard that makes `?raw` fragile is real, and it is task 14's rather than this task's: vitest's `css-disable` plugin rewrites any id matching `\.css(?:$|\?)` to `export default ""`, and task 14's step 2 measured that and set `css: { include: [/\.css\?raw$/] }` — scoped to the query rather than named after one file, so every `?raw` CSS import arrives with its real bytes. Nothing here needs that widened, and nothing here may widen it: `vitest.config.ts` belongs to task 14 and is not in §9's row for this task.

**It reports files, not line numbers.** Producing a line number means blanking comments while preserving newline count, which is a second implementation of "what is a comment" alongside task 14's `stripCssComments` — and two implementations of one rule is precisely the drift task 14's step 5 wrote those helpers to prevent. A failure prints `src/App.css  #646cff`; one `grep` finds the line.

**Which rules stay with the reviewer rather than the grep is itself the point.** One is the property allowlist: `docs/DESIGN-SYSTEM.md` says `transform`/`opacity` only, but contract §6.1 makes `grid-template-columns` the one allowed exception and task 18 ships `transition: background-color var(--dur-fast) var(--ease)` on a rail row. A mechanical allowlist would fail already-approved CSS on its first run, and an audit that fails on correct code is an audit somebody deletes. The other is token-only durations: task 18's `.dot-flash` is `animation: flash 600ms var(--ease) 1`, a one-shot with no token, and it is right. What the grep *does* enforce is the rule underneath both — `transition: all` is banned, and so is a transition that names no property at all, which is the same defect wearing the shorthand.

Create `desktop/src/styles/designsystem.test.ts`:

```ts
// Contract §9 (task 24) and §5. The mechanical half of the design system: the rules a
// reviewer cannot hold in their head across ten stylesheets and forty components.
// docs/DESIGN-SYSTEM.md exists because the first prototype failed by looking
// machine-generated, and every rule below is one of that document's non-negotiables
// expressed as a grep over the tree rather than as an instruction nobody re-reads.
//
// Files are read from disk with node:fs rather than imported with `?raw`: a `?raw` import
// is static, so the set of stylesheets audited would be a hand-written list that every
// later stylesheet has to be added to. This walks the tree instead. The hex and comment
// regexes come from src/test/fakes.ts — there is one implementation of "what is a hex
// literal" in this repository and task 14 owns it.

import { readFileSync, readdirSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { findHexLiterals, stripCssComments } from "../test/fakes";

/** `desktop/src`. This file lives at `desktop/src/styles/designsystem.test.ts`. */
const SRC = fileURLToPath(new URL("..", import.meta.url));

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

/** Repo-relative and always forward-slashed, so a failure message reads the same on
 *  Windows as it does in CI. */
function rel(file: string): string {
  return `src/${relative(SRC, file).split(sep).join("/")}`;
}

const ALL = walk(SRC).sort();
const CSS = ALL.filter((f) => f.endsWith(".css"));

/** Non-test TypeScript. Test files are excluded deliberately: task 14's tokens.test.ts
 *  carries all nineteen documented hexes as data and this file names the two banned ones.
 *  An audit that may not name the thing it bans is an audit nobody can read. */
const TS = ALL.filter(
  (f) => /\.tsx?$/.test(f) && !/\.test\.tsx?$/.test(f) && !rel(f).startsWith("src/test/"),
);

const TEXT = new Map<string, string>(
  [...CSS, ...TS].map((f) => [f, readFileSync(f, "utf8")]),
);

function read(file: string): string {
  const text = TEXT.get(file);
  if (text === undefined) throw new Error(`not collected: ${file}`);
  return text;
}

function source(relPath: string): string {
  const hit = [...TEXT.keys()].find((f) => rel(f) === relPath);
  if (hit === undefined) throw new Error(`missing: ${relPath}`);
  return read(hit);
}

const TOKENS = "src/styles/tokens.css";

/* ── the rules, as predicates ───────────────────────────────────────────────────────── */

/** The indigo/violet arc, in OKLCH degrees. docs/DESIGN-SYSTEM.md bans the slop pair
 *  #6366f1 / #8b5cf6 outright; contract §5.2 places them at ~276° and ~303° and records
 *  that nothing in the palette lands in the arc except `--drift`, at 316.7°. */
const BANNED_HUE = { min: 250, max: 330 };
const DRIFT_HUE = 316.7;

function oklchHues(css: string): number[] {
  const out: number[] = [];
  for (const match of css.matchAll(/oklch\(([^)]*)\)/g)) {
    const parts = match[1].split("/")[0].trim().split(/\s+/);
    const hue = Number.parseFloat(parts[2] ?? "");
    if (Number.isFinite(hue)) out.push(hue);
  }
  return out;
}

function inBannedArc(hue: number): boolean {
  const h = ((hue % 360) + 360) % 360;
  return h >= BANNED_HUE.min && h < BANNED_HUE.max;
}

/** Every value of one property, whitespace-collapsed. Matched against the declaration
 *  rather than the line: at least one transition in the tree is written across three. */
function declarations(css: string, property: string): string[] {
  const re = new RegExp(`${property}\\s*:\\s*([^;}]*)`, "g");
  return [...css.matchAll(re)].map((m) => m[1].replace(/\s+/g, " ").trim());
}
```

Append the eleven assertions to the same file:

```ts
describe("the design-system audit", () => {
  it("walks the whole source tree and reads real bytes", () => {
    const names = CSS.map(rel);
    expect(names).toContain(TOKENS);
    expect(names).toContain("src/styles/app.css");
    // Ten stylesheets ship in Phase 2: tokens, app, shell, rail, subrail, topbar, common,
    // menu, develop, palette. Fewer means the walk missed a directory and every assertion
    // under it is vacuous — the failure mode task 14's step 2 measured on `?raw`.
    expect(CSS.length).toBeGreaterThanOrEqual(10);
    expect(TS.length).toBeGreaterThanOrEqual(20);
    expect(source(TOKENS).length).toBeGreaterThan(2000);
  });

  it("uses no literal hex in any stylesheet", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      for (const hex of findHexLiterals(stripCssComments(read(file)))) {
        offenders.push(`${rel(file)}  ${hex}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("uses no literal hex in any non-test source file", () => {
    // Task 19 already lives by this rule: xterm cannot read a CSS variable, so its theme is
    // resolved from the token layer at construction rather than hard-coded. Any red here is
    // fixed the same way, never by widening the exclusion list.
    const offenders: string[] = [];
    for (const file of TS) {
      for (const hex of findHexLiterals(read(file))) offenders.push(`${rel(file)}  ${hex}`);
    }
    expect(offenders).toEqual([]);
  });

  it("contains the indigo/violet tell nowhere at all", () => {
    const tell = ["#6366f1", "#8b5cf6"];
    const offenders = [...TEXT.keys()]
      .filter((f) => tell.some((t) => read(f).toLowerCase().includes(t)))
      .map(rel);
    expect(offenders).toEqual([]);
  });

  it("lands no colour in the indigo/violet arc, bar the one status token Phase 2 never emits", () => {
    // --drift is defined so Phase 3 does not invent a second purple, and contract §5.2 says
    // so. It is the single permitted member of the arc, and the next test proves it is unused.
    expect(oklchHues(stripCssComments(source(TOKENS))).filter(inBannedArc)).toEqual([DRIFT_HUE]);

    const offenders: string[] = [];
    for (const file of CSS) {
      if (rel(file) === TOKENS) continue;
      for (const hue of oklchHues(stripCssComments(read(file))).filter(inBannedArc)) {
        offenders.push(`${rel(file)}  oklch hue ${hue}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("defines --drift and references it nowhere, because Phase 2 cannot derive drift", () => {
    expect(stripCssComments(source(TOKENS))).toMatch(/^\s*--drift\s*:/m);
    expect([...TEXT.keys()].filter((f) => read(f).includes("var(--drift)")).map(rel)).toEqual([]);
  });

  it("keeps every colour function inside the token layer", () => {
    const colourFn = /\b(?:rgba?|hsla?|lab|lch|oklab|oklch|color-mix)\(/g;
    const offenders: string[] = [];
    for (const file of CSS) {
      if (rel(file) === TOKENS) continue;
      for (const m of stripCssComments(read(file)).matchAll(colourFn)) {
        offenders.push(`${rel(file)}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("has every stylesheet but the token layer import the token layer first", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      if (rel(file) === TOKENS) continue;
      const first = stripCssComments(read(file))
        .split("\n")
        .map((line) => line.trim())
        .find((line) => line !== "");
      if (first === undefined || !/^@import "(?:\.\/|(?:\.\.\/)+styles\/)tokens\.css";$/.test(first)) {
        offenders.push(`${rel(file)}  ${first ?? "<empty file>"}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("never writes `transition: all`, nor a transition that names no property", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      for (const value of declarations(stripCssComments(read(file)), "transition")) {
        // `transition: 0.75s` is `transition: all` with the word left out.
        if (/^all\b/.test(value) || !/^[a-z-]+[\s,]/.test(`${value} `)) {
          offenders.push(`${rel(file)}  transition: ${value}`);
        }
      }
    }
    for (const file of TS) {
      for (const m of read(file).matchAll(/transition\s*:\s*["'`]\s*all\b/g)) {
        offenders.push(`${rel(file)}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("builds every shadow from a token or from currentColor", () => {
    // docs/DESIGN-SYSTEM.md: elevation is a 1px ring plus a layered shadow, never a flat
    // border — and never a colour invented at the call site.
    const offenders: string[] = [];
    for (const file of CSS) {
      if (rel(file) === TOKENS) continue;
      for (const value of declarations(stripCssComments(read(file)), "box-shadow")) {
        if (!value.includes("var(--") && !value.includes("currentColor")) {
          offenders.push(`${rel(file)}  box-shadow: ${value}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it("ships no gradient anywhere", () => {
    // Tell #1 in docs/DESIGN-SYSTEM.md, and Phase 2 needs none: the grain layer is an SVG
    // noise `url()`, not a gradient. A surface that needs one changes the design system
    // first and this test second, in that order.
    const offenders: string[] = [];
    for (const file of [...CSS, ...TS]) {
      for (const m of stripCssComments(read(file)).matchAll(/\b(?:linear|radial|conic)-gradient\(/g)) {
        offenders.push(`${rel(file)}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});
```

- [ ] **Step 3: Run the audit and watch five of its eleven go red on one file**

```bash
cd desktop
npm test -- src/styles/designsystem.test.ts
```

**What you must see: eleven tests, five failing, and every failure naming `src/App.css`.** That number is not a guess — it is read off the file, which is still Vite's React template at 116 lines:

The assertions are named, not numbered, because a twelfth rule would renumber them and this
table would then point at the wrong ones.

| Assertion | Why it fails | Evidence in `src/App.css` |
|---|---|---|
| uses no literal hex in any stylesheet | 18 hex literals, 13 distinct | `#747bff`, `#61dafb`, `#646cff`, `#535bf2`, `#396cd8` ×2, `#24c8db` ×2, `#0f0f0f` ×2, `#0f0f0f98`, `#0f0f0f69`, `#f6f6f6` ×2, `#ffffff` ×2, `#e8e8e8`, `#2f2f2f` |
| keeps every colour function inside the token layer | one `rgba()` | `box-shadow: 0 2px 2px rgba(0, 0, 0, 0.2);` |
| has every stylesheet but the token layer import the token layer first | the first line is a selector | `.logo.vite:hover {` |
| never writes `transition: all`, nor a transition that names no property | line 37 | `transition: 0.75s;` |
| builds every shadow from a token or from currentColor | the same `rgba()` shadow | line 74 |

**Five, and every one of them naming `src/App.css` — that is the check.** Shared shape §3B binds
tasks 18, 20, 21, 22 and 23 to rules this audit greps for: the token import first in
every stylesheet, no colour function outside the token layer, no gradient, and no
`transition: all`. If a failure here names a component stylesheet — `src/components/**/*.css`,
`src/styles/palette.css` — then that stylesheet did not land §3B, and **the fix belongs to the
task that wrote it**. Report it to the orchestrator exactly as step 1 says, and do not edit the
stylesheet: §9's row gives this task five files and not one of them is a component stylesheet.
The audit is not wrong when that happens. It is doing the only job it has.

The six that pass are as interesting as the five that do not. **The `#6366f1`/`#8b5cf6` test passes**, because Vite's template uses `#646cff` and `#535bf2` — the same indigo/violet family, not the literal pair `docs/DESIGN-SYSTEM.md` names. That is the whole argument for the hex rule being the primary gate and the named-pair test being a second, narrower one: a tell grep alone would have found nothing here. The arc test passes too, because it reads `oklch()` and this file has none.

The reporter chrome below is approximate — no command in this plan has been run, for the reason at the top of the file. The five `×` lines and the file name are what matter; the durations, the ordering and the diff formatting will differ.

```
 ❯ src/styles/designsystem.test.ts (11 tests | 5 failed)
   × the design-system audit > uses no literal hex in any stylesheet
     → expected [ 'src/App.css  #747bff', …17 more ] to deeply equal []
   × the design-system audit > keeps every colour function inside the token layer
   × the design-system audit > has every stylesheet but the token layer import the token layer first
   × the design-system audit > never writes `transition: all`, nor a transition that names no property
   × the design-system audit > builds every shadow from a token or from currentColor

 Test Files  1 failed (1)
      Tests  5 failed | 6 passed (11)
```

Two failures that would mean something else. `Tests  no tests` with a loader error means the file was not collected — check `include: ["src/**/*.test.{ts,tsx}"]`. And `walks the whole source tree and reads real bytes` failing on `expected 3 to be greater than or equal to 10` means tasks 18 to 23 have not landed and this task is being run out of order; §9 lists it as depending on 21, 22 and 23.

One Windows detail that is easy to lose an hour to: this repository is checked out with CRLF endings, so `split("\n")` leaves a trailing `\r` on every line. Every line-wise assertion above calls `.trim()` before comparing, which removes it. If you add a twelfth rule, do the same.

- [ ] **Step 4: Write the failing composition suite**

This half of `defects.test.tsx` asserts the things that are only true once the app is assembled. The unit suites already prove each mechanism in isolation — task 19 proves the registry does not dispose a terminal, task 20 proves a pane scrolls into view — and none of them proves that `App` wires the poller to the store at all, which is the single point of failure for every one of them. These eight tests are the integration seam, and the defect-1 test in particular is the one no unit suite can write: it drives three real poll ticks through the real store into the real `DevelopLens` and asserts the terminal object is the same object afterwards.

Everything crosses one stub of `./api`, which is the module `App` imports and also the module `useCreateSession` and the attach registry default to, so one mock covers the whole tree. The xterm mocks are task 19's, unchanged.

Create `desktop/src/defects.test.tsx`:

```tsx
// Contract §9 (task 24) and §8. Phase 2's evidence file, in two halves. This half asserts in
// the ASSEMBLED app what the unit suites can only assert in isolation: that App mounts the
// shell, loads the catalogue exactly once, polls sessions and nothing else, and destroys no
// terminal while it does. Step 9 appends the second half — the manifest over §8's ten.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { FakeTerm, FakeWebSocket } from "./test/fakes";
import { aBranch, aHarness, aMachine, aProject, aSession, aWorktree } from "./test/fixtures";
import {
  __resetAttachRegistry,
  acquireAttach,
  configureAttachRegistry,
  releaseAttach,
} from "./api/attachRegistry";
import { POLL_INTERVAL_MS } from "./api/poller";
import { resetCockpit, useCockpit } from "./store/store";
import type { Session, SessionId } from "./store/types";
import App from "./App";

vi.mock("@xterm/xterm", () => ({ Terminal: FakeTerm }));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class { fit() {} activate() {} dispose() {} },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

/** The control plane, as a table. `vi.hoisted` because the mock factory below is hoisted
 *  above every import and a plain `const` would not exist yet when it runs. */
const server = vi.hoisted(() => ({
  machines: [] as unknown[],
  projects: [] as unknown[],
  harnesses: [] as unknown[],
  branches: new Map<string, unknown[]>(),
  worktrees: new Map<string, unknown[]>(),
  sessions: [] as unknown[],
  /** Set to make every endpoint reject — the control plane not running. */
  fail: null as Error | null,
  calls: { machines: 0, projects: 0, harnesses: 0, branches: 0, worktrees: 0, sessions: 0 },
}));

vi.mock("./api", async () => {
  const { FakeWebSocket: Socket } = await import("./test/fakes");
  const answer = async <T,>(key: keyof typeof server.calls, value: T): Promise<T> => {
    server.calls[key] += 1;
    if (server.fail !== null) throw server.fail;
    return value;
  };
  return {
    api: {
      listMachines: () => answer("machines", server.machines),
      listProjects: () => answer("projects", server.projects),
      listHarnesses: () => answer("harnesses", server.harnesses),
      listBranches: (projectId: string) =>
        answer("branches", server.branches.get(projectId) ?? []),
      listWorktrees: (_projectId: string, branchId: string) =>
        answer("worktrees", server.worktrees.get(branchId) ?? []),
      listSessions: () => answer("sessions", server.sessions),
      openAttach: (s: Session) =>
        new Socket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => server.sessions[0] as Session,
    },
  };
});

/** Fake timers plus a real microtask flush. `advanceTimersByTimeAsync` drains the promise
 *  queue between timer callbacks, which `advanceTimersByTime` does not, so a fetch resolved
 *  inside a tick has actually been written to the store by the time this returns. */
async function settle(ms = 0): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
    // Three drains, not one. A catalogue load resolves, its store write wakes the
    // navigation subscription, and THAT starts a second fetch: branches, then worktrees.
    // One drain returns before the second round has begun.
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(0);
  });
}

function seedCatalogue(): void {
  server.machines = [aMachine({ id: "m-1" })];
  server.projects = [aProject({ id: "p-1" })];
  server.harnesses = [aHarness({ id: "claude" })];
  server.branches.set("p-1", [aBranch({ id: "b-1", projectId: "p-1" })]);
  server.worktrees.set("b-1", [
    aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" }),
  ]);
}

/** Task 19's trick: reach the registry entry the pane already built. The factory throws if
 *  it is ever called, so reaching one IS the assertion that one exists. */
function grabTerm(id: SessionId): FakeTerm {
  const handle = acquireAttach(useCockpit.getState().sessions[id], {
    createTerm: () => {
      throw new Error(`no terminal exists for ${id}; the pane never mounted`);
    },
    readOnly: false,
  });
  releaseAttach(id);
  return handle.term as unknown as FakeTerm;
}
```

Append the harness and seven of the eight tests to the same file — the eighth, the one no unit suite can write, goes in its own block below:

```tsx
beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.reset();
  __resetAttachRegistry();
  resetCockpit();
  server.machines = [];
  server.projects = [];
  server.harnesses = [];
  server.sessions = [];
  server.branches.clear();
  server.worktrees.clear();
  server.fail = null;
  for (const key of Object.keys(server.calls) as (keyof typeof server.calls)[]) {
    server.calls[key] = 0;
  }
  configureAttachRegistry({
    client: {
      openAttach: (s: Session) =>
        new FakeWebSocket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => server.sessions[0] as Session,
    },
  });
});

afterEach(() => {
  __resetAttachRegistry();
  vi.useRealTimers();
});

describe("the assembled cockpit", () => {
  it("mounts the shell and its grain layer, and adds no chrome of its own", async () => {
    const { container } = render(<App />);
    await settle();
    const shell = container.querySelector(".shell");
    expect(shell).not.toBeNull();
    expect(shell?.classList.contains("grain")).toBe(true);
    // App renders <AppShell/> and nothing else: no wrapper div, no banner, no router.
    expect(container.firstElementChild).toBe(shell);
  });

  it("loads machines, projects and harnesses exactly once on mount", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.machines).toBe(1);
    expect(server.calls.projects).toBe(1);
    expect(server.calls.harnesses).toBe(1);
    const state = useCockpit.getState();
    expect(state.machines).toHaveLength(1);
    expect(state.projects).toHaveLength(1);
    expect(state.harnesses).toHaveLength(1);
  });

  it("polls sessions on the interval and never refetches the catalogue", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.sessions).toBe(1);   // §4.7: the first tick does not wait 2 s
    await settle(POLL_INTERVAL_MS * 3);
    expect(server.calls.sessions).toBe(4);
    // Machines, projects and harnesses change when the user acts, not when an agent does.
    expect(server.calls.machines).toBe(1);
    expect(server.calls.projects).toBe(1);
    expect(server.calls.harnesses).toBe(1);
  });

  it("stops the interval when the app unmounts", async () => {
    const { unmount } = render(<App />);
    await settle();
    const before = server.calls.sessions;
    unmount();
    await settle(POLL_INTERVAL_MS * 5);
    expect(server.calls.sessions).toBe(before);
  });

  it("loads branches when a project opens, worktrees when a branch is selected", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.branches).toBe(0);   // nothing is open yet

    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(server.calls.branches).toBe(1);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);

    act(() => useCockpit.getState().selectBranch("p-1", "b-1"));
    await settle();
    expect(server.calls.worktrees).toBe(1);
    expect(useCockpit.getState().worktreesByBranch["b-1"]).toHaveLength(1);

    // Loaded once. The poll does not touch either, and neither does a re-render.
    await settle(POLL_INTERVAL_MS * 2);
    expect(server.calls.branches).toBe(1);
    expect(server.calls.worktrees).toBe(1);
  });

  it("leaves the shell standing when the control plane is not running, and names the failure", async () => {
    server.fail = new Error("fetch failed");
    const { container } = render(<App />);
    await settle();
    expect(container.querySelector(".shell")).not.toBeNull();
    const error = useCockpit.getState().lastError;
    expect(error).not.toBeNull();
    expect(error?.message).toContain("fetch failed");
  });

  it("does not retry a failed load on every store write", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    server.fail = new Error("fetch failed");
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(server.calls.branches).toBe(1);

    // Any unrelated store write re-runs the subscription. A loader with no memo of what
    // already failed turns that into a request per keystroke.
    act(() => useCockpit.getState().toggleRail());
    act(() => useCockpit.getState().toggleRail());
    await settle();
    expect(server.calls.branches).toBe(1);
  });
});
```

And the one test the unit suites cannot write, in its own block:

```tsx
describe("defect 1, in the assembled app", () => {
  it("three poll ticks do not touch the terminal the user is typing into", async () => {
    seedCatalogue();
    const base = aSession({
      id: "s-1",
      projectId: "p-1",
      branchId: "b-1",
      worktreeId: "w-1",
      status: "running",
      attachPath: "/v1/sessions/s-1/attach",
    });
    server.sessions = [base];
    useCockpit.setState({
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
      lens: "develop",
    });

    render(<App />);
    await settle();

    // A zero here is not a defect-1 failure: it means the pane never rendered. Check that
    // the worktree load resolved, that `lens` is still "develop", and that AppShell's
    // Develop arm returns <DevelopLens/> rather than task 18's reserved LensPlaceholder.
    expect(FakeWebSocket.instances).toHaveLength(1);
    const term = grabTerm("s-1");
    term.write("the user is typing");

    for (const status of ["running", "idle", "running"] as const) {
      server.sessions = [{ ...base, status, updatedAt: new Date().toISOString() }];
      await settle(POLL_INTERVAL_MS);
    }

    // The store moved — so the poll really ran — and the terminal did not.
    expect(useCockpit.getState().sessions["s-1"].status).toBe("running");
    const after = grabTerm("s-1");
    expect(after).toBe(term);
    // Identity AND content, both read off the handle re-acquired AFTER the three ticks.
    // Asserting on `term` here would prove nothing the line above has not already proved;
    // asserting on `after` is what catches a refactor that put a FRESH Terminal under the
    // same key, because a fresh one has an empty `written`. FakeTerm.written is a string[]
    // (task 14), one entry per write, so join("") is everything the pane has ever shown.
    expect(after.written.join("")).toContain("the user is typing");
    expect(after.disposed).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });
});
```

`term.write` is called for one reason: a terminal that is re-created loses what was written to it, and asserting object identity alone would still pass if some future refactor swapped a fresh instance in under the same key. Identity plus surviving content is the pair that cannot be faked — which is why the assertions read `after.written`, off the handle re-acquired after the three ticks, and not the `term` reference the test has been holding all along. If `FakeTerm` has no `write` that records, task 19 grew task 14's copy in its step 1 and it does — check there before adding one here, because shared-shape §3 says `fakes.ts` is task 14's file and this task may not touch it.

- [ ] **Step 5: Run the composition suite and watch it fail on an app that does nothing**

```bash
cd desktop
npm test -- src/defects.test.tsx
```

**What you must see: eight tests, seven of them failing.** `src/App.tsx` is still task 19 step 12's eleven-line placeholder — a centred `<p>` that says "The cockpit mounts here in task 24" — so there is no shell, no poller and no catalogue. The green one is *stops the interval when the app unmounts*, for the reason step 10 gives: against an app that never starts an interval it counts zero ticks before the unmount and zero after, so it cannot go red here and proves nothing on its own. Of the seven that do fail, the first assertion is the failing one in every case but *loads branches when a project opens, worktrees when a branch is selected*, whose opening `expect(server.calls.branches).toBe(0)` passes against a placeholder that fetches nothing; that one fails an assertion later, on the `expect(server.calls.branches).toBe(1)` that follows the toggle. The ones worth recognising are:

- *mounts the shell and its grain layer* fails at `expected null not to be null`, because `container.querySelector(".shell")` finds nothing. That is the honest first failure of this task.
- *three poll ticks do not touch the terminal* fails at `expected [] to have length 1`, not inside `grabTerm`. Ordering matters here: if it fails inside `grabTerm` with `no terminal exists for s-1`, the socket assertion above it was removed.

This is a red-before-green with the one exception named above: every test whose subject is behaviour `App.tsx` does not yet have is red, and the single green one is green because a no-op app and a correctly stopped interval look identical to it — which is why step 10 keeps that test out of the falsification table too. The reporter chrome is approximate for the reason at the top of this file; `Tests  7 failed | 1 passed (8)` is the line to check.

- [ ] **Step 6: Rewrite `src/App.tsx` in full**

**The catalogue loader is an addition, and here is the argument for it.** §9's one-line scope says "load the catalogue" and names no function. §4.7 says machines, projects, branches, worktrees and harnesses are "fetched on navigation and on an explicit refresh, not on the poll", and assigns that to nobody. Step 1's grep (e) establishes that nobody took it: task 18's rail renders `state.projects` and never asks for them. So it lands here, and the shape follows from one constraint — **this task may not edit a rail file.** §9's row lists `App.tsx` and `main.tsx`, and shared-shape §1's table lists the same two. Putting a `useEffect` in `ProjectRow` would mean editing a file task 18 owns and task 22 also edits, which is the exact collision shape the shared-shape document was written to stop.

The navigation signal is already in the store — `expandedProjects`, `expandedBranches` and `selection` — so the loader subscribes to the store instead of living in a component. That has consequences worth stating. Every network read in the app is in one file, which is what makes step 4's call counters meaningful. The components stay pure readers, which is what §3's store design is for. And the loader is idempotent by construction: it asks "is this list absent from the store and not already in flight", so running it on every store write costs an object lookup.

**It memoises failure, and that is not an optimisation.** The subscription fires on every store write, including its own `setLoading`. Without a set of keys whose last attempt threw, a control plane that is not running turns one expanded project into a fetch per keystroke — and since the failure path also calls `setError`, which is a store write, it would be a self-feeding loop. Step 4's last test is that memo's regression test.

Replace `desktop/src/App.tsx` in full:

> **Superseded in part, 2026-09-23 (ledger, "Task 24 rulings"; UI contract §4.7, "Two
> amendments").** The implementer found two catalogue-error defects in the block below, and both
> are fixed in the shipped file. First, a list that failed or never arrived was never fetched again.
> Second, the poller's next good tick wiped the catalogue's `lastError`. So `startCatalogue` now
> returns `{ stop, retry }` rather than a stop function. App passes `catalogue.retry` to
> `startPolling` as its third argument. The loader clears only the error it raised itself. The
> step-10 table's "Comment out `startPolling(api, useCockpit)`" row therefore targets
> `startPolling(api, useCockpit, catalogue.retry)`. `desktop/src/App.tsx` is the reference.

```tsx
// Contract §9 (task 24) and §4.7. This file renders nothing of its own: it mounts
// <AppShell/>, it starts the one poller, and it owns every catalogue fetch.
//
// The catalogue is here because nothing else fetches it. Every api.* call site across tasks 14
// to 23 is a create (task 18's useCreateSession, task 20's startSessionOnWorktree) or a session
// verb (task 22's sessionVerbs.ts), plus the poller, which fetches sessions. Every one of them
// acts on a row the user already has.
// Nothing LISTS a machine, a project, a harness, a branch or a worktree, so without this file
// the rail renders an empty store for ever.

import { useEffect } from "react";
import type { JSX } from "react";
import { api } from "./api";
import { ApiError } from "./api/client";
import type { CockpitClient } from "./api/client";
import { startPolling } from "./api/poller";
import { AppShell } from "./components/AppShell";
import { useCockpit } from "./store/store";
import type { CockpitState } from "./store/store";
import type { BranchId, ProjectId } from "./store/types";

type Store = typeof useCockpit;

/** Derived rather than imported: `LoadKey` is declared in store/slices/dataSlice.ts and this
 *  file has no other reason to reach inside a slice. Deriving it also means a change to the
 *  key vocabulary is a compile error here rather than a silently unmatched string. */
type LoadKey = Parameters<CockpitState["setLoading"]>[0];

function asError(error: unknown): { code: string; message: string } {
  return error instanceof ApiError
    ? { code: error.code, message: error.message }
    : { code: "internal", message: String(error) };
}

/** Every project whose branches the user has asked to see: expanded in the rail, or
 *  selected. Both, because selecting a project from the palette does not expand it. */
function openProjects(state: CockpitState): ProjectId[] {
  const ids = Object.keys(state.expandedProjects).filter((id) => state.expandedProjects[id]);
  const selected = state.selection.projectId;
  return selected !== null && !ids.includes(selected) ? [...ids, selected] : ids;
}

function openBranches(state: CockpitState): BranchId[] {
  const ids = Object.keys(state.expandedBranches).filter((id) => state.expandedBranches[id]);
  const selected = state.selection.branchId;
  return selected !== null && !ids.includes(selected) ? [...ids, selected] : ids;
}

/** `listWorktrees` is keyed by project AND branch (§4.4), so a branch cannot be fetched
 *  until its project's branch list has arrived. Null means "not yet"; the store write that
 *  delivers the branch list wakes the subscription and this runs again. */
function projectOfBranch(state: CockpitState, branchId: BranchId): ProjectId | null {
  for (const [projectId, branches] of Object.entries(state.branchesByProject)) {
    if (branches.some((branch) => branch.id === branchId)) return projectId;
  }
  return null;
}
```

…and the rest of the same file:

```tsx
/**
 * Loads the catalogue and keeps loading it as the user navigates. Returns a stop function
 * with the same contract as `startPolling`: after it is called nothing further is written.
 */
export function startCatalogue(client: CockpitClient, store: Store): () => void {
  let stopped = false;
  /** Keys whose last attempt threw. A failed load is not retried until the app restarts or
   *  the user acts, because `sync` runs on EVERY store write. */
  const failed = new Set<string>();

  const run = async (key: LoadKey, load: () => Promise<void>): Promise<void> => {
    if (stopped || failed.has(key) || store.getState().loading[key] === true) return;
    store.getState().setLoading(key, true);
    try {
      await load();
    } catch (error) {
      failed.add(key);
      if (!stopped) store.getState().setError(asError(error));
    } finally {
      // Cleared even when stopped. React's StrictMode mounts this effect twice: the first
      // loader sets the flag, the second sees it and skips, and if the first then declined
      // to clear it the key would read "loading" for the life of the process.
      store.getState().setLoading(key, false);
    }
  };

  const sync = (): void => {
    if (stopped) return;
    const state = store.getState();

    for (const projectId of openProjects(state)) {
      if (state.branchesByProject[projectId] !== undefined) continue;
      void run(`branches:${projectId}`, async () => {
        store.getState().setBranches(projectId, await client.listBranches(projectId));
      });
    }

    for (const branchId of openBranches(state)) {
      if (state.worktreesByBranch[branchId] !== undefined) continue;
      const projectId = projectOfBranch(state, branchId);
      if (projectId === null) continue;
      void run(`worktrees:${branchId}`, async () => {
        const worktrees = await client.listWorktrees(projectId, branchId);
        store.getState().setWorktrees(branchId, worktrees);
      });
    }
  };

  // The three lists that belong to no selection. Fetched in parallel — they are independent
  // and serialising them would put three round trips in front of the first paint.
  void Promise.all([
    run("machines", async () => store.getState().setMachines(await client.listMachines())),
    run("projects", async () => store.getState().setProjects(await client.listProjects())),
    run("harnesses", async () => store.getState().setHarnesses(await client.listHarnesses())),
  ]).then(sync);

  const unsubscribe = store.subscribe(sync);
  sync();   // a rehydrated selection is navigation that already happened

  return () => {
    stopped = true;
    unsubscribe();
  };
}

export default function App(): JSX.Element {
  // §4.7 says the poller starts on mount of AppShell; §9 puts it here. Same lifetime: this
  // component renders AppShell and nothing else, so this effect IS AppShell's lifetime.
  useEffect(() => {
    const stopCatalogue = startCatalogue(api, useCockpit);
    const stopPolling = startPolling(api, useCockpit);
    return () => {
      stopCatalogue();
      stopPolling();
    };
  }, []);

  return <AppShell />;
}
```

What follows will look wrong to a reviewer and is not.

**`` `branches:${projectId}` `` typechecks against `LoadKey` without a cast.** `LoadKey` includes `` `branches:${string}` ``, and TypeScript contextually types a template-literal *expression* against a template-literal *type* — so the argument is inferred as `` `branches:${string}` `` rather than widened to `string`. If your compiler disagrees, do not widen `run`'s parameter to `string`: add `as LoadKey` at the call site and leave the key vocabulary closed.

**`sync` is called three times on startup** — once directly, once by the subscription's first firing after `setLoading`, and once when the parallel load resolves. That is deliberate, not a leftover. The direct call covers a rehydrated `selection` restored by `persist` before any fetch has run; the third covers the case where `selection.projectId` is only satisfiable once `projects` has arrived. All three are idempotent.

**The effect's dependency array is empty and `api`/`useCockpit` are module singletons**, so it runs once per mount and the linter has nothing to complain about. Do not add them to the array: they are not reactive values, and listing them invites someone to "fix" a re-subscription that is not happening.

**Nothing here renders an error.** `setError` writes the store; contract §7.7's error-surface table gives the rail footer and the main-area bar to `RailFooter` and `AppShell`, both task 18's. An error banner in `App.tsx` would be a second owner of the same state.

- [ ] **Step 7: Load the stylesheet, and delete the one nothing loads**

Two changes that are the same change. `desktop/src/styles/app.css` has existed since task 14 and no module imports it, so the reset, the scrollbars, the page colours and `tabular-nums` on `<body>` have never been applied — every surface tasks 18 to 23 built has been rendering against the browser's default ground. `desktop/src/App.css` is Vite's template, imports nothing and is imported by nothing, and step 3 is the first thing in the repository's history to notice.

Replace `desktop/src/main.tsx` in full:

```tsx
// The token layer and the document ground load FIRST, before any component stylesheet.
// Vite emits CSS in module-graph order, so importing this after "./App" would put every
// component sheet ahead of the reset that is supposed to sit under it.
import "./styles/app.css";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Then delete the template stylesheet, and prove nothing referenced it:

```bash
cd desktop
rm src/App.css
test ! -e src/App.css && echo "App.css: deleted"
# Anchored on the IMPORT, not on the string (shared shape §3A). `app.css`'s own header says
# "Task 24 imports this from main.tsx and deletes the Vite starter's App.css", so
# `grep -rn "App.css" src` finds that sentence over a tree in which nothing imports the file.
grep -rnE '^import "\./App\.css";' src --include=*.ts --include=*.tsx   # expect no output
grep -nE "App\.css" index.html                                          # expect no output
```

`<React.StrictMode>` stays. Task 19 built the attach registry's ref-counting and grace window specifically so the double mount is survivable (§1.7), and step 6's `finally` clears `loading` unconditionally for the same reason. Removing StrictMode would hide exactly the class of bug those two were written to expose.

- [ ] **Step 8: Run both files, then the whole suite, then the typechecker**

```bash
cd desktop
npm test -- src/styles/designsystem.test.ts
npm test -- src/defects.test.tsx
npm test
npm run typecheck
```

**What you must see.** The audit is 11 passed — all five of step 3's failures were in the file step 7 deleted, and nothing else in the tree ever carried a literal hex, because task 19's step 12 already greps its own output for one. The composition suite is 8 passed. The full run is every test tasks 14 to 23 left behind plus these nineteen, with zero failures and, in particular, **zero unhandled rejections**: a `PromiseRejectionHandledWarning` or an `Unhandled Rejection` line means a `run(...)` call lost its `catch`, and the only way that happens is if `load()` was made to throw outside the `try`.

`npm run typecheck` must be silent. When it is not, every diagnostic carries four things — file, position, `TS` code, message — in one of two layouts: the flat `src/App.tsx(NN,CC): error TS2345: …` when stdout is piped, and TypeScript 6's pretty `src/App.tsx:NN:CC - error TS2345: …` with the source line and a caret under it when it sees a TTY. `NN` and `CC` are written as placeholders on purpose: no run produced them, and they would move anyway. **Match on the code and the message; the line and column move whenever anything above them does.** These are predictable enough to name, because each means one specific thing rather than "the build is broken":

- `error TS2345: Argument of type 'string' is not assignable to parameter of type 'LoadKey'.` on the `` `branches:${projectId}` `` call — the contextual typing of the template literal did not fire. Add `as LoadKey` at the call site; do not widen `run`'s parameter.
- `error TS2339: Property 'subscribe' does not exist on type …` — `useCockpit` was imported as the hook rather than the store object. Zustand's `create` returns a hook *with* `getState`, `setState` and `subscribe` hung off it; an intermediate `const store = useCockpit()` inside a component would lose them.
- `error TS6133: 'aHarness' is declared but its value is never read.` in `defects.test.tsx` — `noUnusedLocals` is on and the fixture is used by `seedCatalogue`; if this fires, the seed function was dropped during transcription.

None of them came out of a run, for the reason at the top of this file. The codes are the ones `tsc` emits for those errors and the two layouts are TypeScript 6's; the wording is the compiler's phrasing of the shape described, not a transcript. Treat the code as the signal.

- [ ] **Step 9: Append the ten-defect manifest**

**The manifest keys on implementation anchors, not on test titles, and that is the decision this step turns on.** Contract §8's last column names a regression test per defect by *title* — and titles are prose that authors legitimately reword. They already have: task 15 writes *"does not collide on fast successive calls — known defect 2"* where §8 says *"10 000 successive makeId calls are unique"*; task 19 writes *"a store update does not dispose the terminal or re-open the socket (defect 1)"*; task 20 writes *"a newly created session is focused and scrolled into view"* where §8 says *"a newly focused session is scrolled into view and takes focus"*. A manifest that greps for §8's exact strings would be red on its first run for a reason that is not a regression, and a gate that cries wolf is a gate somebody deletes.

So each row carries two things that do not move. The **test file** §8 names must exist and contain tests — that catches a suite being deleted or renamed out of existence. And an **anchor in the implementation**: the call, the export or the absence that the fix cannot survive without. `ColumnsLayout` without `scrollIntoView` is defect 4 again whatever its tests are called.

Extend the import list at the top of `desktop/src/defects.test.tsx`:

```tsx
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
```

Then append:

```tsx
/* ── the ten known defects — contract §8 ────────────────────────────────────────────── */

const SRC = fileURLToPath(new URL(".", import.meta.url));
const at = (relPath: string): string => join(SRC, relPath.replace(/^src\//, ""));
const text = (relPath: string): string => readFileSync(at(relPath), "utf8");

/** The same source with block and line comments blanked, newlines kept. Shared shape §3A, and
 *  defect 8 is the shape it describes: an `absent` anchor bans a construct the file's own prose
 *  can legitimately DISCUSS. `useKeyboardMap.ts` exists partly to explain why it does NOT
 *  handle shift+tab, and the first draft of its header named both the key and the field while
 *  explaining it — which made a raw /permissionMode/ red against a correct file, permanently.
 *  Task 23 has since reworded that header to name neither, so the raw form passes today too.
 *  This strips anyway: a guard that only holds while another task keeps a particular sentence
 *  out of a particular comment is a guard that breaks on a rewrite nobody thought was risky.
 *
 *  Task 14 exports `stripCssComments` and the audit uses it; TypeScript's `//` form has no
 *  equivalent there, and this is the only place in Group D that needs one, so it lives here
 *  rather than growing another task's file. It is deliberately naive — it blanks a `//` inside
 *  a string literal too — which is why it is used for `absent` ONLY, over one file that
 *  contains no URL and no regex. */
function codeOnly(source: string): string {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .replace(/\/\/[^\n]*/g, "");
}

interface Anchor {
  file: string;
  /** Code the fix cannot exist without. Matched against the RAW source: every one was read
   *  against the file that ships it and every one is a real declaration or call, never a
   *  comment. If one ever becomes satisfiable only by prose, move it to `codeOnly` too. */
  present?: RegExp;
  /** Code whose PRESENCE would be the defect. Defect 8 is the only one shaped this way, and it
   *  is matched against `codeOnly(...)` for the reason above. */
  absent?: RegExp;
}

interface Defect {
  n: number;
  /** docs/FUNCTIONAL-SPEC.md's wording, as contract §8 quotes it. */
  spec: string;
  /** §8's "Fixed by" column. */
  fixedBy: number[];
  /** §8's "Regression test" column: the files, not the titles. */
  tests: string[];
  anchors: Anchor[];
}

const DEFECTS: Defect[] = [
  {
    n: 1,
    spec: "Live tick destroys in-progress typing",
    fixedBy: [19],
    tests: ["src/api/attachRegistry.test.ts", "src/components/Terminal.test.tsx"],
    anchors: [
      // Ref-counted entries keyed by session id, at module scope, outside React.
      { file: "src/api/attachRegistry.ts", present: /\brefs\b/ },
      { file: "src/components/Terminal.tsx", present: /\bmemo\s*\(/ },
    ],
  },
  {
    n: 2,
    spec: "uid() collides on fast successive calls",
    fixedBy: [15],
    tests: ["src/lib/ids.test.ts"],
    anchors: [{ file: "src/lib/ids.ts", present: /randomUUID\s*\(/ }],
  },
  {
    n: 3,
    spec: "Denying a permission dead-ends the session — composer must always render",
    fixedBy: [20],
    tests: ["src/components/develop/SessionPane.test.tsx"],
    anchors: [
      // Rendered once, unconditionally. readOnly gates stdin and nothing else.
      { file: "src/components/develop/SessionPane.tsx", present: /^\s*<Terminal\b/m },
      { file: "src/components/develop/SessionPane.tsx", present: /readOnly/ },
    ],
  },
  {
    n: 4,
    spec: "New session created off-screen with no focus",
    fixedBy: [20],
    tests: ["src/components/develop/ColumnsLayout.test.tsx"],
    anchors: [
      { file: "src/components/develop/ColumnsLayout.tsx", present: /scrollIntoView\s*\(/ },
      { file: "src/components/develop/ColumnsLayout.tsx", present: /focusSession/ },
    ],
  },
  {
    n: 5,
    spec: "Auto-scroll steals the scrollback",
    fixedBy: [19],
    tests: ["src/api/attachRegistry.test.ts"],
    anchors: [
      { file: "src/api/attachRegistry.ts", present: /pinned/ },
      { file: "src/api/attachRegistry.ts", present: /scrollToBottom\s*\(/ },
    ],
  },
  // … the array continues in the next block. It is one literal; do not close it here.
```

The rest of the same array. Defects 7, 9 and 10 are the ones §8 says live in a surface Phase 2 does not build — Discover is Phase 4, Merge is Phase 3 — and each is fixed as a *class* in the surface where the same mistake would otherwise be made, which is why their anchors sit in `lib/immutable.ts`, `store/types.ts`, `sessionVerbs.ts` and `CommandPalette.tsx` rather than anywhere named Merge:

```tsx
  {
    n: 6,
    spec: "Canvas tile positions not persisted",
    fixedBy: [21],
    tests: [
      "src/components/develop/CanvasLayout.test.tsx",
      "src/store/store.test.ts",
    ],
    anchors: [
      { file: "src/store/slices/prefsSlice.ts", present: /\btiles\b/ },
      { file: "src/store/store.ts", present: /partialize/ },
    ],
  },
  {
    n: 7,
    spec: "editAnswer splices all later answers",
    fixedBy: [15, 22],
    tests: [
      "src/lib/immutable.test.ts",
      "src/components/develop/sessionVerbs.test.ts",
    ],
    anchors: [
      { file: "src/lib/immutable.ts", present: /export function replaceAt\b/ },
      { file: "src/lib/immutable.ts", present: /export function replaceById\b/ },
    ],
  },
  {
    n: 8,
    spec: "\"Don't ask again for X\" flips the whole session to bypass",
    fixedBy: [23],
    tests: ["src/hooks/useKeyboardMap.test.ts"],
    anchors: [
      // The only inverted row. The cockpit must not own a permission model at all: shift+tab
      // belongs to the agent's own TUI, so the keyboard map may not name Tab, and nothing
      // outside the create path may write permissionMode. Both run against codeOnly(...), so
      // the row asserts that no BRANCH here reads or writes either one — which is the thing
      // that matters — and stays green if a future header sentence names them while
      // explaining the rule. Shared shape §3A.
      { file: "src/hooks/useKeyboardMap.ts", absent: /["']Tab["']/ },
      { file: "src/hooks/useKeyboardMap.ts", absent: /permissionMode/ },
    ],
  },
  {
    n: 9,
    spec: "Nothing ever sets `ready`, so user-created worktrees can never merge",
    fixedBy: [22],
    tests: ["src/components/develop/sessionVerbs.test.ts"],
    anchors: [
      // Contingent on contract §10.2 landing in the Go surface. Either way the code has to
      // KNOW about the flag: if §10.2 landed, the verb PATCHes it; if it did not, the verb
      // is withheld while `Worktree.ready` is undefined. Both spell `ready`.
      { file: "src/store/types.ts", present: /\bready\?:/ },
      { file: "src/components/develop/sessionVerbs.ts", present: /\bready\b/ },
    ],
  },
  {
    n: 10,
    spec: "Merge reorder indexes the unfiltered array",
    fixedBy: [15, 23],
    tests: [
      "src/lib/immutable.test.ts",
      "src/components/palette/CommandPalette.test.tsx",
    ],
    anchors: [
      { file: "src/lib/immutable.ts", present: /export function moveWithinVisible\b/ },
      { file: "src/lib/immutable.ts", present: /export function idAtVisiblePosition\b/ },
      // The rule the helpers encode: never pass an index across a filter boundary.
      { file: "src/components/palette/CommandPalette.tsx", present: /idAtVisiblePosition/ },
    ],
  },
];

describe("the ten known defects", () => {
  it("covers all ten, numbered 1 to 10, with no gap and no duplicate", () => {
    expect(DEFECTS.map((d) => d.n)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
  });

  it("names a regression test file for each, and every one of them exists and holds tests", () => {
    const missing: string[] = [];
    for (const defect of DEFECTS) {
      for (const file of defect.tests) {
        if (!existsSync(at(file))) {
          missing.push(`defect ${defect.n}: ${file} does not exist`);
          continue;
        }
        if (!/\bit\s*\(/.test(text(file))) {
          missing.push(`defect ${defect.n}: ${file} contains no tests`);
        }
      }
    }
    expect(missing).toEqual([]);
  });

  it("still has every fix in the implementation", () => {
    const regressions: string[] = [];
    for (const defect of DEFECTS) {
      for (const anchor of defect.anchors) {
        if (!existsSync(at(anchor.file))) {
          regressions.push(`defect ${defect.n}: ${anchor.file} does not exist`);
          continue;
        }
        const source = text(anchor.file);
        if (anchor.present !== undefined && !anchor.present.test(source)) {
          regressions.push(`defect ${defect.n} (${defect.spec}): ${anchor.file} lost ${anchor.present}`);
        }
        if (anchor.absent !== undefined && anchor.absent.test(codeOnly(source))) {
          regressions.push(`defect ${defect.n} (${defect.spec}): ${anchor.file} gained ${anchor.absent}`);
        }
      }
    }
    expect(regressions).toEqual([]);
  });
});
```

And the last block, which is why the import list grew a directory walk. These are not defects; they are the facts Groups B and C settled *after* the UI contract was written, and this is the file a later phase will read to find out what Phase 2 actually guaranteed:

```tsx
/* ── the facts the UI contract predates ─────────────────────────────────────────────── */

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) out.push(full);
  }
  return out;
}

const SOURCES = walk(SRC);
const relName = (file: string): string => `src/${relative(SRC, file).split(sep).join("/")}`;

describe("what Groups B and C settled after the UI contract was written", () => {
  it("treats the harness arrays as always present, because /v1/harnesses returns [] not null", () => {
    // Task 8's fix round made the Go side allocate unconditionally and pinned it with a
    // marshalled-bytes test, precisely because null.map() throws where [].map() returns [].
    // A defensive `?? []` on this side is dead code that would hide a regression on the
    // other side of the wire instead of reporting it.
    const defensive = /\b(?:harnesses|models|efforts|permissionModes)\s*\?\?\s*\[\s*\]/g;
    const offenders: string[] = [];
    for (const file of SOURCES) {
      for (const m of readFileSync(file, "utf8").matchAll(defensive)) {
        offenders.push(`${relName(file)}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("declares eighteen error codes: contract-go §3.2's fifteen, ruling 7's sixteenth, and the client's two", () => {
    const union = /export type ProblemCode\s*=([\s\S]*?);/.exec(text("src/api/client.ts"));
    expect(union).not.toBeNull();
    const codes = [...(union?.[1] ?? "").matchAll(/"([a-z_]+)"/g)].map((m) => m[1]);
    expect(codes).toContain("non_loopback_publish");   // Group C's ruling 7
    expect(codes).toContain("network_unavailable");    // client-raised: nothing answered
    expect(codes).toContain("timeout");                // client-raised: timeoutMs elapsed
    expect(codes).toHaveLength(18);
    expect(new Set(codes).size).toBe(18);
  });

  it("can render only the four statuses the server can send", () => {
    // docs/AGENTIC-UX.md §4 rule 8 lists seven; contract §0 keeps only the mechanically
    // derived ones, because the rest need the agent-runtime signal that document itself calls
    // a hard runtime dependency. §0.1 rules `need` and `drift` out of the prototype's own
    // vocabulary for the same reason: neither can be derived without reading PTY bytes. A
    // fifth member here would be a status the cockpit can display and the control plane can
    // never produce.
    const union = /export type SessionStatus\s*=([\s\S]*?);/.exec(text("src/store/types.ts"));
    const members = [...(union?.[1] ?? "").matchAll(/"([a-z]+)"/g)].map((m) => m[1]).sort();
    expect(members).toEqual(["exited", "idle", "running", "starting"]);
  });
});
```

`non_loopback_publish` earns its line here for a reason worth spelling out. Task 12 emits it when the daemon reports a sandbox published off loopback, and **that sandbox is running** — so a cockpit that fell through to `sandbox_not_running` would tell the user something false about a live machine. Nothing in Group D switches on error codes (the rail footer renders `lastError.message` verbatim), so the sixteenth code has no exhaustive switch to fall out of; the count assertion is what stops it being quietly dropped from the union by a later tidy-up.

- [ ] **Step 10: Run the manifest, then prove it can fail**

```bash
cd desktop
npm test -- src/defects.test.tsx
npm run typecheck
```

**What you must see: 14 passed** — the eight composition tests, the three manifest tests and the three cross-group facts — and a silent typecheck.

**A regression suite that has never been red proves nothing, and this one is green on its first run by construction**, because every fix it names already landed in tasks 15 to 23. So falsify it. Make each change, run the one command, confirm the named test goes red for the named reason, then **undo the change before making the next one**.

| Mutation | Test that must go red | Failure it must print |
|---|---|---|
| Delete the `scrollIntoView(` call from `src/components/develop/ColumnsLayout.tsx` | *still has every fix in the implementation* | `defect 4 (New session created off-screen with no focus): src/components/develop/ColumnsLayout.tsx lost /scrollIntoView\s*\(/` |
| Add a `case "Tab":` to `src/hooks/useKeyboardMap.ts` | *still has every fix in the implementation* | `defect 8 … gained /["']Tab["']/` |
| Rename `src/api/attachRegistry.test.ts` to `.bak` | *names a regression test file for each* | two lines, defects 1 and 5, both `does not exist` |
| Comment out `startPolling(api, useCockpit)` in `src/App.tsx` | *polls sessions on the interval and never refetches the catalogue* | `expected 0 to be 1` on its first assertion |

*stops the interval when the app unmounts* is deliberately **not** in that table and must not be added to it. With the poller commented out it counts zero ticks before the unmount and zero after, so it stays green — which is exactly why it is written beside the interval test rather than in place of it: on its own it cannot tell a stopped interval from one that never started.

Do the same for the audit, once: put `color: #ff0000;` into any component stylesheet and confirm *uses no literal hex in any stylesheet* prints `src/…  #ff0000`. If a mutation does **not** go red, that row of the manifest is decorative and the anchor is wrong — fix the anchor, not the mutation.

Then confirm you left nothing behind:

```bash
cd ..
git status --porcelain -- desktop/src | sort
```

Five lines and no others: a modification for `App.tsx` and `main.tsx`, a deletion for `App.css`, and two additions. Anything else is an un-reverted mutation.

- [ ] **Step 11: Prove the CORS grant against a running control plane**

**No test in this repository can prove this one.** Task 16's client takes an injected `fetch`, jsdom does not enforce the same-origin policy the way a browser does, and WebSockets — which is all Phase 1 ever exercised — need neither a preflight nor an `Access-Control-Allow-Origin` header. That is precisely why a missing grant would have stayed invisible until this task's first REST call. Task 9 shipped it; this step is the evidence, and it runs entirely from the shell: the daemon, the `curl`s below, and the cockpit itself.

Running the daemon is neither `go test` nor an edit to a `.go` file, so it is inside Group D's rules. In one terminal:

```bash
cd control-plane
go run ./cmd/api            # binds 127.0.0.1:8730; task 13 refuses anything else
```

In another:

```bash
# 1. The grant on a simple GET. The origin is ECHOED, never `*`.
curl -s -i -H "Origin: http://localhost:1420" http://127.0.0.1:8730/v1/machines | head -12

# 2. The preflight every POST/PATCH/DELETE with a JSON body triggers.
curl -s -i -X OPTIONS -H "Origin: http://localhost:1420" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  http://127.0.0.1:8730/v1/sessions | head -12

# 3. A rejected origin must be a readable 403 JSON body, not an opaque CORS failure.
curl -s -i -H "Origin: http://evil.example" http://127.0.0.1:8730/v1/machines | head -12
```

**What you must see.** (1) `200`, `Access-Control-Allow-Origin: http://localhost:1420` and `Vary: Origin`. (2) `204`, `Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS`, `Access-Control-Allow-Headers: Content-Type`, `Access-Control-Max-Age: 600`, **and no `Access-Control-Allow-Credentials` at all** — with an echoed origin that header would make the allowlist the only thing between a rejected page and an authenticated one. (3) `403` with `{"error":"forbidden_origin",…}`.

Then the cockpit itself:

```bash
cd desktop
npm run tauri dev
```

The rail should list the machine and the local projects within a second of the window appearing, a project should expand into branches, and a branch should populate the sub-rail. A rail that stays empty while the `curl`s above pass means the catalogue loader is not running — check the effect, not the CORS.

**This is a functional check, not a visual one.** `CLAUDE.md` is explicit: the corporate network blocks headless screenshots and visual acceptance is the product owner's eyes, not a self-report. Do not write "the cockpit looks right" into the commit message.

- [ ] **Step 12: Guards, then commit**

Every guard below is anchored per shared shape §3A. Guards in this file's first draft that a comment somewhere else in the tree could satisfy on its own are fixed: step 1's (b), which counted task 14's six *commented* builder signatures as well as task 15's six real ones; step 1's (d), which `app.css`'s own first line answered; and this block's `App.css` scan, which `app.css`'s header — "deletes the Vite starter's App.css" — answered. The placeholder guard, `grep -c "The cockpit mounts here in task 24" src/App.tsx`, is the rule-3 analogue for this task: it consumes no reserved insertion point, because step 6 replaces `App.tsx` whole rather than inserting into it, so the check is that task 19's placeholder is **gone** rather than that a line was added. It greps the placeholder's visible `<p>` text and not its reserving comment, because the comment is prose about this task that a reviewer might reasonably quote back, and the `<p>` is the thing a user would still be looking at if step 6 never ran.

```bash
cd desktop
npm test
npm run typecheck

test ! -e src/App.css && echo "App.css: deleted"
grep -rnE '^import "\./App\.css";' src --include=*.ts --include=*.tsx  # expect no output
grep -nE "App\.css" index.html                                        # expect no output
grep -c "The cockpit mounts here in task 24" src/App.tsx              # expect 0
grep -cE '^import "\./styles/app\.css";' src/main.tsx                 # expect 1
grep -rnE "#[0-9a-fA-F]{3,8}\b" src --include=*.css --include=*.ts --include=*.tsx \
  | grep -v "src/styles/tokens.css" | grep -v "\.test\."              # expect no output

cd ..
git status --porcelain -- '*.go'                                   # expect no output
git status --porcelain -- desktop/src | sort                       # expect exactly five lines
```

The last `grep` duplicates the audit on purpose: it is the one rule cheap enough to check from the shell before the suite runs, and it catches a hex added to a file the walk somehow missed.

Then commit, staging by name — shared-shape §6, never `git add -A`, `-u` or `.`. `git add` on a path whose file is gone records the deletion, which is how `App.css` leaves:

```bash
git add desktop/src/App.tsx \
        desktop/src/main.tsx \
        desktop/src/App.css \
        desktop/src/defects.test.tsx \
        desktop/src/styles/designsystem.test.ts

git commit -m "feat(desktop): wire the cockpit to the control plane and prove the ten" -m "App.tsx is rewritten: it mounts AppShell, starts the one poller and owns every
catalogue fetch. Nothing before this task listed a machine, a project, a branch, a
worktree or a harness, so the rail rendered an empty store; branches and worktrees
are loaded on navigation via a store subscription, per contract 4.7, which keeps
every network read in one file and leaves task 18's components pure readers.

main.tsx imports styles/app.css, which has existed unimported since task 14, and
App.css is deleted: Vite's template, 116 lines, 18 hex literals, never imported.

Two evidence files. defects.test.tsx carries the assembled-cockpit tests the unit
suites cannot write, a manifest over contract section 8's ten known defects keyed
on implementation anchors rather than test titles, and the three facts Groups B
and C settled after the UI contract was written. designsystem.test.ts is the
mechanical audit: no literal hex, no colour outside the token layer, no gradient,
no transition without a property, no hue in the indigo/violet arc.

The CORS grant task 9 shipped is exercised for the first time here, by the
catalogue load, and verified against a running control plane.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

That closes Group D and Phase 2. What is **not** claimed: no screenshot was taken, no visual acceptance was performed, and the five lenses that route to `LensPlaceholder` are still placeholders by design (§0.2). What is claimed is that the cockpit reads from a live control plane, that the ten defects have anchors that fail when the fix is removed, and that the design system is enforced by a grep rather than by hope.
