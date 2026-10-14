### Task 87: the cycle view — convergence per criterion, the scope log, cadence settings, `Y`

Stream **WD2**, about 3 focused hours (an estimate, not a measurement). Depends on task 83 (the
W2 wire types and replica kinds), and through it on task 55 (the replica) and 56 (`mutate`). The
Hub half is task 77, whose routes this task reads; a sync-merge of `feat/hub` precedes the first
green run (rule H1).

After this task a team has a cycle view that answers *what is this cycle converging on* without
ever predicting when it will finish. Linear answers that question with a burn-up chart and a
velocity figure; 05 §1 L-63 replaces both with **convergence per criterion**, and L-62 (capacity)
is a SKIP whose own stated substitute — the last three cycles' closed counts as plain facts — is
built here. `Y` picks a cycle, `G V` opens the current one, `O C` opens any cycle by name.

**Anchors, re-read on 2026-09-27.** On the tree: `desktop/src/components/rail/Rail.tsx` and
`rail.css` exist; `desktop/src/work/` and `desktop/src/components/plan/` do **not** yet (they are
created by tasks 53-58 and 84), so every path under them is this phase's own and **UNVERIFIED
until those parts merge**. In this branch's parts: task 77's wire (`task-77.md:170-183`), its
`PatchCycleRequest`, `PutCadenceRequest` and `SetCycleRequest` (`:196-214`), `IssuePlan.ArchivedAt`
(`:228-230`), its eight event kinds (`:235`), and its *does not claim* line handing convergence to
this task (`:585-587`). Task 83's type list, `Cycle`, `Cadence`, `CustomState` and the rest, is the
scope's (`tasks-81-100.md:90-103`) because `task-83.md` is not written at this writing —
**UNVERIFIED**; a different spelling there is a rename here. In the parity docs: 05 §1 rows 57-65
and §4.1's cycle paragraph, §3.3's five words, §3.6's `actor: null` rule, §4.3's key table.

#### Decisions the scope left open, made here

1. **Convergence is derived on the desktop, never stored.** Task 77 stores the scope log and the
   cycle's membership; it stores no per-criterion roll-up (`task-77.md:585-587`). This view reads
   the cycle's issues from the replica, reads each issue's `serves` (criterion ids), asks phase
   4's criterion read model for each criterion's word, and groups. Nothing is cached to disk: the
   replica is the cache (task 55's rule), and a stale roll-up would be a number nobody wrote.
2. **The five words are phase 4's, not ours.** `settled`, `open`, `contested`, `drifting`,
   `uncovered` (05 §3.3). This task renders them; it does not compute them and does not add a
   sixth. A criterion whose word the read model cannot supply renders as `not read` in mono — a
   stated absence, never a guessed word.
3. **A count per word, and nothing that divides.** Each row is *word · N issues*. No percentage,
   no ratio, no bar whose length is a fraction of a total, because a fraction of a total is a
   completion claim. `expectNoPrediction` (task 34's matcher, **UNVERIFIED** until task 34 merges)
   guards the whole subtree, and test 1 below is the one that fails when somebody adds a bar.
4. **The last three cycles are facts with their dates.** L-62's substitute is *closed in EP-cycle
   12: 14 · 11: 9 · 10: 12*, each with the cycle's own start and end dates beside it, so nobody
   reads three numbers as a trend. They are rendered in `tabular-nums` (DESIGN-SYSTEM.md's live
   numbers rule). No average is shown, because an average of three is a velocity.
5. **The scope log names a person for every entry.** Task 77's `ScopeEntry` carries the actor;
   where the actor is the janitor the line reads the **policy's author** with the policy named
   (*"rolled over by Priya's rollover policy"*), by 05 §3.6's rule that an automatic act is
   written as the person whose policy caused it. `actor: null` renders as *the system* nowhere;
   an entry that arrives with a null actor and no policy author renders as `cause not recorded`
   and test 3 pins that.
6. **An issue started mid-cycle is held until `N`.** Task 77's Hub puts a `started` issue into the
   current cycle. On the list that would make the row jump under the reader's cursor, so the row
   is held in place by task 56's `moves.hold` and released on `N` (next change), the same
   mechanism the Desk uses. The held row shows a mono *moved to this cycle* line while held.
7. **`Y` is a picker, not a setter.** `Y` on a focused issue row opens the cycle picker and sets
   the issue's cycle through `PUT /v1/issues/{issueId}/cycle`; `Y` with the cycle view focused and
   no row focused does nothing. Task 53's dispatcher already refuses a verb outside its scope, so
   this is one `list` scope row, not a second handler.
8. **Cadence settings are a team-admin surface and say so before it refuses.** A member sees the
   cadence read-only with a mono *team admins set the cadence* line; the controls are not rendered
   at all, so there is no control that answers with a 403. A pause is a gap, entered as a cooldown
   for the next turn, not a `paused` flag (L-65, task 77's model).
9. **The iCal URL is copied, never fetched here.** The view shows the URL task 77 mints and a
   *Copy* button. This task does not fetch it, does not parse it, and claims nothing about what a
   calendar client does with it.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/plan/CycleView.tsx` (+ `.test.tsx`) | new: the cycle page, the held-row rule, the last three cycles |
| `desktop/src/components/plan/CycleConvergence.tsx` (+ `.test.tsx`) | new: criterion rows, their words and counts |
| `desktop/src/components/plan/ScopeLog.tsx` (+ `.test.tsx`) | new: the log, one line per entry, always a person |
| `desktop/src/components/plan/CadenceSettings.tsx` (+ `.test.tsx`) | new: length, cooldown, ahead, names, dates, pause, iCal |
| `desktop/src/components/plan/cycle.css` | new: the three surfaces' rules (tokens only, DESIGN-SYSTEM.md) |
| `desktop/src/work/cycles.ts` (+ `.test.ts`) | new: `convergenceOf`, `scopeLine`, `closedCounts` — pure, no React |
| `desktop/src/work/keys/keymap.ts` ⚑ (task 53) | one row: `Y` → `cycle.pick`, scope `list` |
| `desktop/src/work/keys/sequences.ts` ⚑ (task 53) | two rows: `G V` → current cycle, `O C` → open a cycle |
| `desktop/src/api/client.ts` ⚑ (task 83) | `cadence`, `putCadence`, `cycles`, `patchCycle`, `setIssueCycle`, `scopeLog` |

`desktop/src/work/cycles.ts` holds every derivation so the tests that matter run without a DOM and
without the replica; the four components render what it returns.

#### Interfaces — complete: `desktop/src/work/cycles.ts`

```ts
import type { Cycle, IssueHead, ScopeEntry } from "../api/work";   // task 83 (UNVERIFIED)

/** Phase 4's word for a criterion. The desktop never computes one; it is read. */
export type CriterionWord = "settled" | "open" | "contested" | "drifting" | "uncovered";
export const CRITERION_WORDS: readonly CriterionWord[] =
  ["settled", "open", "contested", "drifting", "uncovered"];

export type CriterionRef = { briefKey: string; criterionId: string };

/** What the criterion read model hands us. `word: null` means "not read", never a guess. */
export type CriterionState = CriterionRef & { title: string; word: CriterionWord | null };

export type ConvergenceRow = {
  criterion: CriterionState;
  issueIds: readonly string[];      // this cycle's issues that serve it, in list order
};

/** Every criterion this cycle's issues serve, once each, in the order first served.
 *  Issues that serve nothing are collected under `untied` and are NOT given a word. */
export function convergenceOf(
  issues: readonly IssueHead[],
  states: readonly CriterionState[],
): { rows: readonly ConvergenceRow[]; untied: readonly string[] };

/** Counts per word across `rows`. A criterion whose word is null is counted in `notRead`.
 *  No total, no ratio: the caller renders "word · N", and there is nothing to divide by. */
export function wordCounts(
  rows: readonly ConvergenceRow[],
): Record<CriterionWord, number> & { notRead: number };

/** The cycle's closed count. `closedStates` is the team's closed set (task 84's custom states
 *  after `landed` are not closed unless the team marked them so). */
export function closedCount(
  issues: readonly IssueHead[],
  closedStates: ReadonlySet<string>,
): number;

/** One scope-log line, always attributed to a person (decision 5). */
export type ScopeLine = {
  at: string;                       // RFC 3339, from the entry
  text: string;                     // "3 added on day 4"
  by: string;                       // a person's display name — never "the system"
  via: string | null;               // "Priya's rollover policy", when a policy caused it
  unattributed: boolean;            // true only when neither actor nor policy author is known
};

export function scopeLine(e: ScopeEntry, cycle: Cycle, nameOf: (id: string) => string): ScopeLine;
```

`scopeLine`'s rule, in full, because it is the one place a null actor could leak a *the system*
line: when `e.actorId` is set the line is that person; when it is null and `e.policyAuthorId` is
set the line is that person with `via` naming the policy; when both are null `unattributed` is
true and the component renders `cause not recorded` in mono. There is no fourth branch, and test 3
asserts the function has no string `"the system"` anywhere in its module (a `grep` assertion in
the unit test, the same shape task 55's no-persistence grep uses).

#### Interfaces — complete: `CycleConvergence.tsx`

```tsx
import { convergenceOf, wordCounts, type ConvergenceRow, type CriterionState } from "../../work/cycles";
import type { IssueHead } from "../../api/work";

export function CycleConvergence(p: {
  issues: readonly IssueHead[];
  states: readonly CriterionState[];
  onOpenCriterion(ref: { briefKey: string; criterionId: string }): void;
}): JSX.Element {
  const { rows, untied } = convergenceOf(p.issues, p.states);
  const counts = wordCounts(rows);
  return (
    <section className="cv-conv" aria-label="Convergence by criterion">
      <ul className="cv-counts">
        {(["settled", "open", "contested", "drifting", "uncovered"] as const).map((w) => (
          <li key={w} className="cv-count">
            <span className="cv-word" data-word={w}>{w}</span>
            <span className="cv-n">{counts[w]}</span>
          </li>
        ))}
        {counts.notRead > 0
          ? <li className="cv-count"><span className="cv-word cv-mono">not read</span>
              <span className="cv-n">{counts.notRead}</span></li>
          : null}
      </ul>
      <ul className="cv-rows">
        {rows.map((r: ConvergenceRow) => (
          <li key={`${r.criterion.briefKey} ${r.criterion.criterionId}`} className="cv-row">
            <button type="button" className="cv-crit"
              onClick={() => p.onOpenCriterion(r.criterion)}>{r.criterion.title}</button>
            <span className="cv-word" data-word={r.criterion.word ?? "unread"}>
              {r.criterion.word ?? "not read"}</span>
            <span className="cv-n">{r.issueIds.length}</span>
          </li>
        ))}
      </ul>
      {untied.length > 0
        ? <p className="cv-untied">{untied.length} in this cycle serve no criterion</p>
        : null}
    </section>
  );
}
```

`.cv-n` is `tabular-nums`; `.cv-word` is sans with `data-word` driving a token colour **and** the
word itself, so colour is never the only signal (task 34's `forced-colors` test covers it). There
is no element whose width is a function of a count.

#### Interfaces — complete: `ScopeLog.tsx` and `CycleView.tsx`'s head

```tsx
export function ScopeLog(p: {
  entries: readonly ScopeEntry[]; cycle: Cycle; nameOf(id: string): string;
}): JSX.Element {
  const lines = p.entries.map((e) => scopeLine(e, p.cycle, p.nameOf));
  return (
    <ol className="cv-log" aria-label="Scope log">
      {lines.map((l, i) => (
        <li key={`${l.at}-${i}`} className="cv-logline">
          <span className="cv-when">{dayOf(l.at, p.cycle)}</span>
          <span className="cv-what">{l.text}</span>
          {l.unattributed
            ? <span className="cv-mono">cause not recorded</span>
            : <span className="cv-who">by {l.by}{l.via ? <span className="cv-mono"> · {l.via}</span> : null}</span>}
        </li>
      ))}
      {lines.length === 0 ? <li className="cv-logline cv-mono">nothing added or removed</li> : null}
    </ol>
  );
}
```

```tsx
export function CycleView(p: { cycleId: string }): JSX.Element {
  const r = useReplica();                                   // SW.16, task 55
  const cycle = r.cycle(p.cycleId);
  const issues = r.issuesInCycle(p.cycleId);                // held rows included, in list order
  const held = useHeld(issues);                             // task 56's moves.hold; released on N
  const states = useCriterionStates(issues);                // phase 4's read model; null word when absent
  const last3 = r.closedCycles(cycle?.teamId, 3);
  if (!cycle) return <p className="cv-mono">cycle not on this device yet</p>;
  return (
    <article className="cv">
      <header className="cv-head">
        <h1>{cycle.name}</h1>
        <p className="cv-dates"><time dateTime={cycle.startsOn}>{cycle.startsOn}</time>
          {" to "}<time dateTime={cycle.endsOn}>{cycle.endsOn}</time></p>
      </header>
      <CycleConvergence issues={held.rows} states={states} onOpenCriterion={openCriterion} />
      <section className="cv-last" aria-label="Closed in the last three cycles">
        <ul>{last3.map((c) => (
          <li key={c.id}><span className="cv-cyc">{c.name}</span>
            <span className="cv-n">{closedCount(r.issuesInCycle(c.id), r.closedStates(cycle.teamId))}</span>
            <span className="cv-mono">{c.startsOn} to {c.endsOn}</span></li>))}</ul>
      </section>
      <ScopeLog entries={r.scopeLog(p.cycleId)} cycle={cycle} nameOf={r.personName} />
      <IssueList rows={held.rows} heldIds={held.heldIds} />
    </article>
  );
}
```

`useHeld` is task 56's hold, used as the Desk uses it (`moves.hold` while the list has focus,
released by `N`); a held row carries `data-held="1"` and one mono line *moved to this cycle*, and
nothing else about it differs. `useCriterionStates` reads phase 4's read model where it is
mounted and returns `word: null` for every criterion when it is not — the view degrades to *not
read*, never to an invented word. Both are **UNVERIFIED until phase 4 merges**.

#### Interfaces — complete: `CadenceSettings.tsx` (the shape; the body is a form over it)

```tsx
export type CadenceDraft = {
  lengthDays: number;      // 0 (off) or 7-56, task 77's PutCadenceRequest
  cooldownDays: number;    // 0-14 — a pause is a longer cooldown, not a flag (L-65)
  ahead: number;           // 1-15
  firstStartsOn: string;   // ValidDate
};

export function CadenceSettings(p: {
  teamId: string; canEdit: boolean;             // canEdit false → read-only, no controls at all
  cadence: Cadence; cycles: readonly Cycle[];   // upcoming cycles, renamable and re-datable
  icalUrl: string | null;                       // task 77's feed; null until a token is minted
}): JSX.Element;
```

The three numbers are `<input type="number">` with the Hub's own bounds as `min`/`max`, so a bad
value is refused in place before a request; a Hub refusal still rolls the form back and names the
code (task 56's `onRefused`), because the bounds may differ after a Hub change. *Copy* writes
`icalUrl` to the clipboard and says *copied* in place — no toast (AX §10 anti-pattern 8).

#### Interfaces — complete: the key rows

```ts
// desktop/src/work/keys/keymap.ts ⚑ — one row, in task 53's table shape
{ key: "Y", scope: "list", verb: "cycle.pick", label: "Cycle…" },

// desktop/src/work/keys/sequences.ts ⚑ — two rows, in task 53's sequence shape
{ seq: ["G", "V"], verb: "cycle.current", label: "Current cycle" },
{ seq: ["O", "C"], verb: "cycle.open",    label: "Open a cycle…" },
```

`cycle.pick` is registered by the list that owns the focused row, exactly as task 68 registers
`issue.create`; `cycle.current` and `cycle.open` are registered by the lens. No other key map row
changes, and `Y` in a text field types `y` by task 53's rule (05 §4.3 rule 1).

#### Tests to write first, each with the mutant it kills

1. `expectNoPrediction(<CycleView/>)` over the rendered subtree, with a fixture of 20 issues.
   **Mutant:** a target line, a velocity figure, a *60 % done*, or any element whose inline width
   is a percentage of a count. (Task 34's matcher; **UNVERIFIED** until 34 merges.)
2. `wordCounts` returns a count per word and `notRead`, and its return has no `total` key.
   **Mutant:** a `total` added, which the view would then divide by.
3. `scopeLine` names a person for every fixture entry, and the module contains no `"the system"`
   string. **Mutant:** a null actor rendered as *the system* (05 §3.6).
4. A rolled-over issue's entry shows the policy's author with the policy named.
   **Mutant:** the janitor rendered as the actor.
5. An issue `started` mid-cycle appears in the current cycle's list **in its held position** and
   moves only after `N`. **Mutant:** the row jumps under the cursor on arrival.
6. `Y` with a focused list row opens the picker; `Y` typed into the cadence name field types `y`
   and opens nothing. **Mutant:** a global key listener.
7. A criterion whose word the read model does not supply renders `not read`.
   **Mutant:** it renders `open`, the most common word, as a default.
8. With `canEdit: false` the cadence form renders no `input` and no `button` except *Copy*.
   **Mutant:** disabled controls that still exist and can be re-enabled in devtools.
9. `forced-colors: active` keeps the five words distinguishable (each renders its own text).
   **Mutant:** colour alone distinguishes `open` from `contested`.

#### Steps

- [ ] **Step 1: Check the base.** Task 83's types on the branch, task 53's key tables, task 55/56's
  replica, and whether phase 4's criterion read model is mounted on the sync base. Where it is
  not, `useCriterionStates` returns `word: null` and tests 1-2 and 7 still run.
- [ ] **Step 2: The nine tests.** Run them: they fail.
- [ ] **Step 3: `work/cycles.ts`** — `convergenceOf`, `wordCounts`, `closedCount`, `scopeLine`.
  Tests 2, 3, 4 and 7 go green with no component written.
- [ ] **Step 4: The four components and `cycle.css`**, then the three key rows.
- [ ] **Step 5: Green**, then commit by pathspec, naming the two key files as touched.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/cycles.test.ts src/components/plan/CycleView.test.tsx \
  src/components/plan/CycleConvergence.test.tsx src/components/plan/ScopeLog.test.tsx \
  src/components/plan/CadenceSettings.test.tsx --reporter=dot 2>&1 | tail -3   # 9 passed
grep -rn "the system" desktop/src/work/cycles.ts | wc -l                       # 0
grep -rnE "velocity|burn-?up|% done|projected" desktop/src/components/plan/ | wc -l   # 0
cd desktop && npx tsc --noEmit                                                 # no output
```

Expected: `9 passed`, `0`, `0`, no output. None has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not a burn-up, a velocity or a capacity dial** (L-62 and L-63 are the SKIP and its substitute).
- **Not the proposed next cycle** (L-66, task 140, W4).
- **Not that a calendar client subscribes to the iCal feed behind Entra** — task 77 owns the
  feed's auth and marks it **UNVERIFIED**; this view copies a URL and asserts nothing about it.
- **Not phase 4's words.** They are read. If the read model is absent the view says *not read*.
- **No measured render cost** at 9,000 issues; the derivation is O(issues + criteria) and has not
  been timed.

#### Gaps reported to the skeleton

1. **The criterion read model has no named seam in this phase.** `useCriterionStates` is invented
   here against phase 4's words. If phase 4 exposes a different call, this task's one hook changes.
2. **Task 83's type list does not include `ScopeEntry`** (`tasks-81-100.md:92-93` lists `Cycle`
   and `Cadence` but not the log's row). Either 83 adds it or this task declares it locally; asked
   of 83 here.
