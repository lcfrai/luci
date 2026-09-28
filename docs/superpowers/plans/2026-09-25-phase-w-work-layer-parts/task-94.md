### Task 94: the digest, defer with a reason (`H`), CSV export

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WD2.
**dependsOn:** 65 (the Desk's answers) and 81 (the digest builder), after a sync-merge of
`feat/work-layer` (rule H1). **Seams:** consumes **SW.17** (task 62's `registerDeskRow(kind,
section, render, rank)`, skeleton `:391`) and **SW.18**'s `DigestItem` and `Deferral` through task
83; adds **no** seam. It registers one new Desk row kind and binds one key.

Three things that all say the same thing: **the Desk does not shout**. The digest is one line, not
a section and not a stream (AX §6 item 8, `docs/AGENTIC-UX.md:171`; anti-pattern 8, `:243`).
Deferring is a **verb with a reason attached**, and what you deferred stays where it was, ageing
in front of you, rather than vanishing until a date (AX §7 item 4, `docs/AGENTIC-UX.md:192`:
*"Defer with a reason is a first-class verb, and deferred items age visibly rather than
disappearing"*). Export leaves through the browser, because the desktop writes no Hub row to disk.

The janitor's stale-closure batch (task 77) also lands on the Desk here, in NEEDS YOU, as *close 6
issues untouched for 90 days?* (L-11) — one row, one answer, six closes written **as the person**.
No other W2 desktop task owns that row, so it is registered in this one.

#### Anchors, re-read on 2026-09-27

In this tree, verified: `desktop/package.json` lists `@tauri-apps/plugin-opener` at `:17`;
`desktop/src/components/` has **no** `desk` directory yet (`ls`, 2026-09-27), so task 62's frame
is not merged here; `docs/AGENTIC-UX.md:192` is AX §7 item 4, quoted above, word for word.

In this branch's parts (**UNVERIFIED until merge**): task 62's Desk frame, its five fixed sections
and `registerDeskRow` (SW.17, skeleton `:391`, scope `tasks-61-80.md:67`); task 65's *answer in
place* and reassign *decide* (`tasks-61-80.md:170`); task 66's one live region
(`tasks-61-80.md:211`); task 81's `GET /v1/work/digest?since=`, its `DigestItem{subject, alias,
title, cause, at, link}` and `DigestSettings{email: off·hourly·daily}` (`tasks-81-100.md:35-58`);
task 80's `GET /v1/projects/{projectId}/issues.csv?viewId=` and `/features.csv`
(`tasks-61-80.md:674-675`); task 77's closure proposals and `POST
/v1/closure-proposals/{proposalId}/answer` (`task-77.md`, decision (g)); SW.18's `Deferral`.

**Task 81's scope flags its own gap**: the defer record and `DigestSettings` must exist in task
72's `hubapi`, *"and if task 72's part does not carry them, that is a gap to close in 72, not
here"*. This task depends on the same two, and repeats the flag rather than working around it.

Design: `05-design.md` §5.4 W2's digest row, §4.2 (*"No new Desk section"*, *never live while you
are on it*) and §4.3 (*"`H` defers on both"*).

#### Decisions the scope left open, made here

**(a) The digest line is a summary and a door, not a feed.** One line above the five sections:
`Since yesterday: 14 changes across 6 issues · 2 due soon` with the counts in `tabular-nums`, and
it opens a popover holding the items grouped by subject, never by event. Fourteen events on one
issue are **one** entry reading *EP-142 — 14 changes*. The popover is the only place digest items
are listed; nothing of the digest is ever injected into a section.

**(b) It does not change while you are looking at it.** The Desk is frozen while open (task 62's
rule, 05 §4.2). The digest inherits that **and adds a floor**: even across a refresh it changes at
most once per 15 minutes, matching the Hub's send ceiling (AX anti-pattern 8). So the sequence is:
the Desk is opened, `since` is the last-read mark, the line is computed once; a re-open inside 15
minutes re-renders the **same** line from the same response. The mark advances only when the
person opens the popover — reading is an act, not a side effect of a render.

**(c) `H` is the same verb on two surfaces, with one dialog.** `H` on a Desk row and `H` on an
issue row both open `DeferMenu`: a reason field, four presets (*waiting on legal*, *waiting on a
customer*, *needs a decision*, *not this cycle*) and an optional *until* date. **The reason is
required**, the presets are text and not codes, and there is no *defer without a reason* path —
that is what "first-class verb" means. Empty or whitespace-only is refused in the client and
again by the Hub.

**(d) A deferred row stays put and ages in words.** It does not leave its section, does not grey
out and does not collapse. It gains a `--ink-3` suffix: `deferred 3d · waiting on legal`, and when
an *until* date passes, `deferred 12d · was due 3d ago` in `--need`. Ageing is rendered from the
one global heartbeat clock (DESIGN-SYSTEM), so a hundred deferred rows tick together and none owns
a timer.

**(e) The closure batch is answered as the person, once.** The NEEDS YOU row renders the
proposal's six issues as a checkable list, all checked by default, with **Close checked** and
**Keep all**. The answer is one `POST …/answer` carrying the checked ids; the Hub writes each
close as the person with cause `hand` (task 77's decision (g)). The client never closes an issue
itself and never sends `close` for an id the person unchecked. *Keep all* is `close: []`, which is
a real answer and stops the proposal returning next sweep.

**(f) Export opens a URL; it never builds a file.** *Export as CSV* calls
`opener.openUrl(toHubUrl(...))` on the Hub's `issues.csv` route (task 80), with the current view's
id when a view is open. The browser downloads it under the person's own browser session. There is
**no `Blob`, no `URL.createObjectURL`, no `download` attribute and no `fs` plugin call** anywhere
in the webview, and a test greps for each, because every one of those is a Hub row written to disk
by us (ADR-H1). The `@tauri-apps/plugin-opener` already in `package.json:17` is the whole
mechanism; nothing new is installed.

#### Files

| File | New / ⚑ | What it gains |
|---|---|---|
| `desktop/src/components/desk/DeskDigest.tsx` | new | decision (a)'s line and popover |
| `desktop/src/components/desk/DeskDigest.test.tsx` | new | the digest suite |
| `desktop/src/components/desk/DeferMenu.tsx` | new | decision (c)'s one dialog |
| `desktop/src/components/desk/DeferMenu.test.tsx` | new | the defer suite |
| `desktop/src/work/defer.ts` | new | `deferLabel`, `validateReason`, `DEFER_PRESETS` |
| `desktop/src/work/defer.test.ts` | new | the label/validation suite |
| `desktop/src/work/exportCsv.ts` | new | `exportIssuesCsv`, `exportFeaturesCsv` |
| `desktop/src/work/exportCsv.test.ts` | new | the no-file-write suite |
| `desktop/src/components/desk/Desk.tsx` | ⚑ (task 62's) | mounts `DeskDigest` above the sections |
| `desktop/src/components/desk/deskRows.tsx` | ⚑ (task 62's) | registers `closure.proposed` in NEEDS YOU |
| `desktop/src/components/desk/ClosureBatchRow.tsx` | new | decision (e)'s row |
| `desktop/src/components/desk/ClosureBatchRow.test.tsx` | new | the batch suite |
| `desktop/src/components/common/LiveRegion.tsx` | ⚑ (task 66's) | announces the digest, not events |
| `desktop/src/work/keys/keymap.ts` | ⚑ | `H` on a Desk row and on an issue row |

#### `desktop/src/work/defer.ts`, complete

```ts
// desktop/src/work/defer.ts
//
// Defer is a first-class verb (AGENTIC-UX.md:192). A reason is required, and what you defer
// stays where it was and ages in front of you. There is no "snooze until" that hides a row.

import type { Deferral } from "../api/work";   // SW.18, via task 83

/** Text, not codes: a preset fills the field and the person may edit it. */
export const DEFER_PRESETS: readonly string[] = [
  "waiting on legal",
  "waiting on a customer",
  "needs a decision",
  "not this cycle",
] as const;

export const REASON_MAX = 140;

export type ReasonProblem = "empty" | "too-long";

/** Client-side; the Hub checks again. Whitespace-only is empty. */
export function validateReason(raw: string): ReasonProblem | null;

/**
 * The `--ink-3` suffix on a deferred row (decision d). `now` is the heartbeat's tick, passed in,
 * so a hundred rows share one clock and none owns a timer.
 *   deferLabel({at, reason, until: null}, now)     → "deferred 3d · waiting on legal"
 *   deferLabel({at, reason, until: past}, now)     → "deferred 12d · was due 3d ago"
 *   deferLabel({at, reason, until: future}, now)   → "deferred 3d · until 14 Oct · waiting on legal"
 */
export function deferLabel(d: Deferral, now: number): string;

/** True when the row should carry `--need` rather than `--ink-3`: an `until` that has passed. */
export function deferOverdue(d: Deferral, now: number): boolean;
```

`deferLabel` never returns a countdown and never returns a bare date without the elapsed part: the
point of L-115 and AX §7 item 4 is that the **age** is visible. Ages under a day read `deferred
today`; there is no `3h` form, because the Desk is not a stopwatch.

#### `DeskDigest.tsx`'s contract, complete

```ts
// desktop/src/components/desk/DeskDigest.tsx
export interface DeskDigestProps {
  /** Fetched once per Desk open by task 62's frame, never by this component (decision b). */
  readonly digest: Digest | null;          // SW.18, via task 83
  /** The last-read mark. Advanced ONLY by onRead (decision b). */
  readonly since: string;
  readonly onRead: () => void;
  /** Task 93's toHubUrl, injected so this file knows no origin. */
  readonly openLink: (item: DigestItem) => void;
}

/** The one line. Returns null — no line at all — when there is nothing since `since`. */
export function DeskDigest(p: DeskDigestProps): JSX.Element | null;

/** Pure, exported for the test: groups by subject and counts, never lists events. */
export function summariseDigest(items: readonly DigestItem[]): {
  changes: number; subjects: number; dueSoon: number;
  groups: readonly { subject: string; alias: string; title: string; count: number; last: string }[];
};
```

`DIGEST_MIN_INTERVAL_MS = 15 * 60 * 1000` is exported and is the floor of decision (b). The
component renders **no** element that updates on a timer; ages inside the popover come from the
heartbeat, as everywhere else.

#### The Desk row and the key binding

`deskRows.tsx` ⚑ gains exactly one registration, in task order after task 65's:

```ts
registerDeskRow("closure.proposed", "needs-you", ClosureBatchRow, 40);
```

`keymap.ts` ⚑ gains `H` in two scopes — `desk.row` and `issue.row` — both dispatching
`defer.open`, which is 05 §4.3's *"`H` defers on both"* held to one handler so the two cannot
drift. `H` on a row that is already deferred opens the same dialog with the current reason filled
and a **Clear deferral** action, so the verb is reversible by the same key.

#### `exportCsv.ts`, complete

```ts
// desktop/src/work/exportCsv.ts
import { openUrl } from "@tauri-apps/plugin-opener";   // already a dependency (package.json:17)

export interface ExportDeps {
  readonly openUrl: (url: string) => Promise<void>;
  readonly hubOrigin: string;
}

/** Opens the Hub's CSV route in the system browser. Builds NO file (decision f). */
export function exportIssuesCsv(
  deps: ExportDeps, projectId: string, viewId: string | null,
): Promise<void>;

export function exportFeaturesCsv(deps: ExportDeps, projectId: string): Promise<void>;

export const tauriExportDeps = { openUrl };
```

Both compose `${hubOrigin}/v1/projects/${projectId}/issues.csv` with `?viewId=` when a view is
open, and refuse a non-`https` origin exactly as `toHubUrl` does. The person's browser carries the
Hub page's own session (task 80); if it does not, the browser shows a sign-in, which is the
**UNVERIFIED** part below.

#### Tests to write first, each with the mutant it kills

Vitest + `@testing-library/react` (`desktop/package.json:26-28`), with the heartbeat and the relay
faked. `defer.test.ts` and the `summariseDigest` case are pure. Each is seen failing first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `defer refuses an empty reason` | `""`, `"  "`, `"\n"` → `"empty"`; the dialog's confirm is disabled and no relay call is made; 141 chars → `"too-long"` | a defer with no reason, which makes the verb a snooze |
| `a deferred row stays in its section and ages` | a `todo` row deferred → still in the same section at the same rank, with `deferred 3d · waiting on legal`; at `now + 9d` past an `until` → `was due 3d ago` in `--need` | hidden until due; a row that greys out or collapses |
| `no deferred row owns a timer` | after mount, `setInterval` and `setTimeout` spies are never called by `DeferMenu` or the row; advancing the injected heartbeat re-renders every label at once | a per-row clock (DESIGN-SYSTEM's one heartbeat) |
| `the digest is one line, grouped by subject` | 40 events on one issue plus 3 on another → `3 changes across 2 issues`… and two popover groups, `EP-142 — 40 changes` | one item per event (the firehose AX §6 item 8 forbids) |
| `the digest does not change while the Desk is open` | 12 feed events arrive after mount → the line's text is identical and the component re-renders zero times; a re-open at `+5 min` → the same line | a re-render per event; a live digest |
| `the read mark advances only on open` | mount, wait, unmount → `onRead` never called; open the popover → called once with the newest `at` | reading as a side effect of rendering |
| `nothing of the digest enters a section` | with a digest of 9 items, the five sections' row counts are unchanged | a sixth section, or digest rows injected (05 §4.2) |
| `the live region announces the digest, not each event` | 40 events → exactly one `polite` announcement, whose text is the summary line | a screen reader read the firehose |
| `the closure batch closes only what is checked` | 6 proposed, 2 unchecked → one `POST …/answer` with the 4 checked ids; *Keep all* → `close: []`; the client issues no state call of its own | the client closing directly; all six closed |
| `the batch cites the policy` | the row's text names the policy's author and the threshold (*set by Grace · 90 days*) | an anonymous "the system closed these" |
| `no Hub row is written to disk` | `grep` over `desktop/src` finds no `createObjectURL`, no `download=`, no `new Blob(`, no `@tauri-apps/plugin-fs`; `exportIssuesCsv` calls `openUrl` once with an `https` URL | a Blob-URL download |
| `export carries the open view` | with view `vw-9` open → the URL ends `issues.csv?viewId=vw-9`; with none → no query | the export ignoring the filter the person is looking at |
| `H is the same verb on both surfaces` | `H` on a Desk row and `H` on an issue row open the same component with the same props shape; on an already-deferred row it offers **Clear deferral** | two divergent dialogs; an irreversible defer |

#### Steps

- [ ] **Step 1: Check the base.** Task 62's frame and `registerDeskRow` as merged; task 65's answer
  in place; task 81's digest route reachable through the fake; task 77's `answer` route; **and the
  two records task 81 flagged** (`Deferral`, `DigestSettings`) present in task 72's `hubapi` and
  task 83's types. If either is missing, that is task 72's gap to close; raise it and stop.
- [ ] **Step 2: The tests.** All thirteen. `npm --prefix desktop test`: they fail.
- [ ] **Step 3: The pure modules**, `defer.ts` then `exportCsv.ts` then `summariseDigest`, green
  before any component is written.
- [ ] **Step 4: The components**, `DeferMenu`, `DeskDigest`, `ClosureBatchRow`, then the four ⚑
  wirings (`Desk.tsx`, `deskRows.tsx`, `LiveRegion.tsx`, `keymap.ts`).
- [ ] **Step 5: Green and commit by pathspec**, the fourteen files of *Files*, naming the four ⚑
  as touched.

#### Acceptance

```bash
npm --prefix desktop test -- --run src/work/defer.test.ts src/work/exportCsv.test.ts 2>&1 | tail -5
npm --prefix desktop test -- --run src/components/desk 2>&1 | tail -6
npm --prefix desktop test 2>&1 | tail -5
npm --prefix desktop run typecheck
grep -rn 'createObjectURL\|new Blob(\|download=\|plugin-fs' desktop/src | wc -l     # 0
grep -rn 'setInterval' desktop/src/components/desk | wc -l                          # 0
grep -c 'registerDeskRow("closure.proposed"' desktop/src/components/desk/deskRows.tsx  # 1
```

Expected: `2 passed` then the Desk files passing with no failures; the whole suite passes;
`typecheck` silent, exit 0; the greps print `0`, `0`, `1`. **None has been run; none of these
files exists and `desktop/src/components/desk` does not exist yet** (OD-27).

#### What this task does not claim

- **Not that the Hub's export page signs in through the browser.** **UNVERIFIED** until task 80's
  page lands: the browser may show a sign-in before the download. If it does, that is task 80's
  session to fix, not a reason to build a file here.
- **Not the email half of the digest.** Sending is task 81's mailer under the Hub's workload
  identity. This task renders what the route returns and owns the desktop's *email: off · hourly ·
  daily* control only in so far as it writes the setting task 81 reads.
- **Not Teams delivery**, which is task 165. L-116's desktop half only.
- **Not a subscription editor.** What a person is subscribed to is task 81's `Subscriptions`;
  changing it is not in any W2 desktop task, and that is a gap listed below.
- **Not a due-date engine.** *Due soon* is a cause the Hub sets; the desktop does not compute it.
- **Not a measured 15-minute ceiling in the wild.** The floor is enforced in the component and
  tested with an injected clock; nobody has watched a real fortnight.
- **Not intake CSV** (import is W5).

#### Corrections to the scope

1. **`ClosureBatchRow.tsx` is a file the scope does not name.** The scope has this task *register*
   the janitor's closure batch through `registerDeskRow`, which needs a component to register. It
   is added here with its test.
2. **`desktop/src/work/defer.ts` and `exportCsv.ts` are not in the scope's file list either.** The
   scope names only the two components. Putting `deferLabel`, `validateReason` and the export URL
   in components would make them untestable without a DOM and would hide the no-file-write rule
   inside JSX; both are pure modules here, and the grep test is the stronger for it.
3. **Nobody owns *change my digest settings*.** Task 81 reads `DigestSettings{email:
   off·hourly·daily}` per person and no desktop task writes it. Task 96's preferences screen is
   the natural home, but its scope lists exactly four keys and digest settings are **Hub** data,
   not a device preference, so they cannot go in the preference file. **Raised as a skeleton gap**;
   the fallback is one control in this task's digest popover, which is where a person notices they
   want it.
4. **The read mark needs somewhere to live.** Decision (b)'s `since` is per person and must
   survive a restart, so it is **Hub** state (`GET /v1/work/digest?since=` implies the caller
   remembers). Task 81's scope does not say who stores it. This task sends `since` from the last
   `readAt` the Hub returns, and if task 81 does not return one, that is task 81's gap.

**Parity rows served:** **L-115** (defer with a reason; PARTIAL, as priced), **L-116**'s desktop
half with task 81, **L-153** (export) with task 80, **L-11**'s Desk half with task 77, and the
digest halves of **L-37** and **L-96**. Task 97 proves them.
