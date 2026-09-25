### Task 38: prototype, part 1: the Work lens and Plan in the app, behind `flag.compare`, on fixtures

Stream **WD1**, no seam, about 3 hours (an estimate, not a measurement). Depends on task 32
(`lib/flags.ts`, the dev toolbar), task 36 (SW.14, the thread) and task 37 (SW.15, the types and
`workSlice`). It also places task 28's `StateGlyph`, task 29's `AgentGlyph` and `MarkGlyph`, task
31's `ViewHeader`, task 20's `workScenario()` and `expectNoPrediction`, and registers with task 33's
`registerStressScreen`; the skeleton's row names 32, 36 and 37 only (a gap, named in the return).

`OD-1` keeps Phase W's review gate: the owner sees the work surfaces **in the app, in the new look**,
before tasks 40-45 are sized (05 §5.3 row 10; row 11 is "resized after row 10's review"). This is
a prototype, not the product. It reads task 20's fixtures, talks to no Hub and no replica, and lives
in its own dev-only directory, `desktop/src/prototype/work/`, which the skeleton's file map does not
list. Task 40 promotes what the owner kept into `components/work/` and deletes the rest.

**Decisions made here, so nobody re-derives them:**

1. **Three gates, all needed.** The prototype mounts when `lens === "work"`, `import.meta.env.DEV`
   holds, and the new flag `prototype` is `"on"`. The DEV gate is a `lazy()` behind a literal, as
   task 32's `components/devtools/mount.ts` does it, so a production build emits no chunk; the
   flag keeps the prototype out of a developer's way until they ask for it.
2. **How the owner gets there.** `G W` is task 54's. A dev-toolbar button, *Open the Work
   prototype*, sets the flag and calls task 37's `openWork(project, "feat/resumable-uploads")`.
   Inside the prototype one ghost button switches Work and Plan (the prototype's stand-in for
   `G W` and `G P`), so the review needs no keys.
3. **The world is task 20's, with overrides only.** Task 20's rule holds: no builder is declared
   here. `world.ts` takes `workScenario()` and adds, as override objects, what 05 §4.1's drawing
   needs and the scenario does not carry: which criterion each head serves (task 20 gives
   revisions for EP-142 and EP-164 only), EP-167 moved to `started` so the reader has a live
   agent, and one agent line. The drawing's issue numbers are 05's; the prototype uses the world's:

   | 05 §4.1's row | In the prototype | Why this one |
   |---|---|---|
   | EP-142, the reader's issue with `dylan · agent 1` | **EP-167**, `todo` → `started` (override) | the world's EP-142 is already `done` |
   | EP-150, a teammate's, *touching upload/\*.go* | **EP-163**, Alice's, `started` | the world's one teammate issue |
   | EP-139, held, `→ done · own evidence at 7c1f0a2 …` | **EP-162**, `proven_self_attested` | "own evidence" is exactly self-attested |
   | EP-131, settled | **EP-142**, `done`, proven | the world's proven issue |
   | EP-155, *not tied · chore* | **EP-165** (and EP-160, *not tied · imported*) | both not-tied heads |
   | EP-128, the departure ghost | **EP-168**, `cancelled` | the world's cancelled head |

4. **What is shown and what is not.** Cancelled and duplicate heads are not rows (EP-169 is
   absent; EP-168 is its ghost only). The convergence word per group is a fixture value here;
   task 45 computes it (`task-45.md:81`, `Convergence`). Group sizes are plain counts (Q1 A).
5. **Lit and selected, together.** At mount, three rows of the first group are selected through
   `workSlice.select` and the keyboard cursor sits on a row of the second group; focusing that row
   lights its criterion (task 36's `bind`, focus half only, on the rows; the full bind on the
   header). The three selected rows ring and carry a leading `MarkGlyph kind="selected"`; the lit
   group brightens only its gutter.
6. **`moves.hold`, faked honestly.** *Replay a land* moves EP-150 from `in_review` to `landed` in
   the prototype's own state. The list order is frozen at mount, so the row stays, takes its new
   glyph, sets `data-changed="unseen"` (task 34's flash or tick, the attribute task 41 also
   sets, `task-34.md:23-25`) and shows one mono cause line; the
   group header gains `· 1 moving`, and *Apply held changes* re-sorts. Task 41 builds the real
   rule; this only lets the owner see it.
7. **Plan** (05 §4.1, `G P`): Triage, Backlog, every criterion with its issues, and *My issues*.
   No cycles, features board or saved views (W2). Rows are the same row component as Work.
8. **Old and new look, graphite and paper** come from task 32's toolbar for free: the prototype
   writes only tokens and task 30's sans/mono rule, so `data-look` and `data-theme` restyle it.
   Both screens register with task 33's `stress.sets` by name.
9. **The node sits on the group header, once per criterion.** The drawing (05 §4.1) and the
   prototype HTML (`.gh .tnode`, `prototype/work-layer.html:128-129`) put the ring at the head of
   the line, and the rows hang from the line. A node per row would put a link inside every
   `role="option"`, an interactive element inside another, and a tab stop per row. The scope's
   "each with its `ThreadNode`" is read as each *group*; the owner can reverse it at task 39.

**Files** (the skeleton's file map has no row for any of these; a gap, named in the return):
- `desktop/src/prototype/work/world.ts` — the scenario plus its overrides (new)
- `desktop/src/prototype/work/mount.ts` — the DEV gate and the stress registration (new)
- `desktop/src/prototype/work/register.tsx` — the two `registerStressScreen` calls (new)
- `desktop/src/prototype/work/WorkPrototype.tsx`, `PlanPrototype.tsx` — the two screens (new)
- `desktop/src/prototype/work/Rows.tsx` — the row and the group, shared by both screens (new)
- `desktop/src/prototype/work/Launcher.tsx` — the toolbar's *Open the Work prototype* (new)
- `desktop/src/prototype/work/prototype.css` — owns `.pw-*` (new)
- `desktop/src/prototype/work/prototype.test.tsx` — ten tests (new)
- `desktop/src/lib/flags.ts` ⚑ (task 32) — `prototype: "off" | "on"` (modified)
- `desktop/src/components/devtools/DevToolbar.tsx` ⚑ (task 32) — mounts `Launcher` (modified)
- `desktop/src/components/AppShell.tsx` ⚑ — `LensRouter` gains its `"work"` arm (modified)

#### Interfaces — complete: the delta on ⚑ `lib/flags.ts`

```ts
export type PrototypeFlag = "off" | "on";

export interface Flags {
  look: Look;
  sans: SansCandidate;
  theme: ThemeName;
  /** Task 38: the dev-only Work and Plan prototype. Off is the production DOM. */
  prototype: PrototypeFlag;
}
// FLAG_DEFAULTS gains   prototype: "off",
// ATTRIBUTE gains       prototype: "data-prototype",
```

No stylesheet reads `data-prototype`; the attribute exists because task 32's rule is one attribute
per flag, and `resetFlags` clears it with the others.

#### Interfaces — complete: `desktop/src/prototype/work/mount.ts`

```ts
// desktop/src/prototype/work/mount.ts — task 38. The one import of the prototype from product
// code. Vite replaces import.meta.env.DEV with a literal, so in a production build both branches
// below are dead and no prototype chunk is emitted (the acceptance grep over dist/ holds that).
import { lazy } from "react";
import type { JSX, LazyExoticComponent } from "react";

export const WorkPrototype: LazyExoticComponent<() => JSX.Element> | null = import.meta.env.DEV
  ? lazy(() => import("./WorkPrototype"))
  : null;

if (import.meta.env.DEV) {
  // Registers "Work prototype" and "Plan prototype" with task 33's stress.sets.
  void import("./register").then((m) => m.register());
}
```

`register.ts` (new, six lines, in the same directory):

```ts
import { registerStressScreen } from "../../components/devtools/StressSets";
import WorkPrototype from "./WorkPrototype";
import { PlanPrototype } from "./PlanPrototype";

export function register(): void {
  registerStressScreen("Work prototype", () => <WorkPrototype />);
  registerStressScreen("Plan prototype", () => <PlanPrototype />);
}
```

It holds JSX, so its name is `register.tsx`; the `import("./register")` above resolves it.

#### Interfaces — complete: the delta on ⚑ `components/AppShell.tsx`

```tsx
import { Suspense } from "react";
import { useFlags } from "../lib/flags";
import { WorkPrototype } from "../prototype/work/mount";

// LensRouter, a new arm before `default` (AppShell.tsx:67). Task 40 replaces it with the lens.
    case "work":
      return <WorkArm />;

/** Nothing unless all three gates hold (decision 1). A hook, so it lives in its own component. */
function WorkArm(): JSX.Element {
  const { prototype } = useFlags();
  if (WorkPrototype === null || prototype !== "on") return <></>;
  return (
    <Suspense fallback={null}>
      <WorkPrototype />
    </Suspense>
  );
}
```

#### Interfaces — complete: `Launcher.tsx`, and the delta on ⚑ `DevToolbar.tsx`

```tsx
// desktop/src/prototype/work/Launcher.tsx — task 38, decision 2. Loaded only by the dev toolbar.
import type { JSX } from "react";
import { setFlag } from "../../lib/flags";
import { useCockpit } from "../../store/store";
import { WORK_IDS } from "../../test/workFixtures";

export function Launcher(): JSX.Element {
  const open = () => {
    setFlag("prototype", "on");
    useCockpit.getState().openWork(WORK_IDS.project, "feat/resumable-uploads");
  };
  return <button type="button" className="dt-reset" onClick={open}>Open the Work prototype</button>;
}
```

In `DevToolbar.tsx`, `import { Launcher } from "../../prototype/work/Launcher";` and
`<Launcher />` after `<FlagCompare />`. It borrows task 32's `.dt-reset` look, so the toolbar
gains no class.

#### Interfaces — complete: `desktop/src/prototype/work/world.ts`

```ts
// desktop/src/prototype/work/world.ts — task 38, decision 3. Task 20's world plus overrides; no
// builder is declared here (task 20's rule). Every override names the drawing row it stands in for.
import type { AgentStatus } from "../../components/glyphs/AgentGlyph";
import type { IssueHead, IssueRevision } from "../../api/work";
import { ALICE, DYLAN, WORK_IDS, makeIssueRevision, makeServe, workScenario } from "../../test/workFixtures";

export type ConvergenceWord = "settled" | "open" | "contested" | "drifting" | "uncovered";

export interface ProtoGroup {
  /** null is "Serves no criterion", always last. */
  criterionId: string | null;
  title: string;
  convergence: ConvergenceWord | null;
  issueIds: string[];
}
/** One agent line on the reader's own issue (05 §4.1, D C3). */
export interface ProtoAgent { issueId: string; who: string; meaning: string; status: AgentStatus; liveness: string }
/** A teammate's issue: the person and one sans line, no dot, no count (05 §4.4 rule 3). */
export interface ProtoTouch { issueId: string; line: string }

export interface ProtoWorld {
  projectKey: string; featureName: string; branch: string;
  reader: typeof DYLAN;
  heads: Record<string, IssueHead>;
  /** Each shown head's head revision: its title and its serves. */
  revisions: Record<string, IssueRevision>;
  briefKey: string;
  groups: ProtoGroup[];
  agent: ProtoAgent;
  touches: ProtoTouch[];
  /** EP-162's held cause line (the drawing's EP-139). */
  held: { issueId: string; cause: string };
  /** EP-168's departure (the drawing's EP-128). */
  departure: { alias: string; state: "cancelled" };
  /** The land the dev button replays (decision 6). */
  replay: { issueId: string; to: "landed"; cause: string };
  /** The initial multi-select and cursor (decision 5). */
  selected: string[]; cursor: string;
}

export function protoWorld(): ProtoWorld {
  const s = workScenario();
  const [resume, upload] = s.criteria;
  const heads = Object.fromEntries(s.issues.map((h) => [h.id, h])) as Record<string, IssueHead>;
  heads[WORK_IDS.ep167] = { ...heads[WORK_IDS.ep167], state: "started" };   // the reader's live issue
  // Each head's title and serves are its head revision's, as in the product. EP-142 and EP-164
  // carry task 20's revisions; the rest are override revisions (an override of task 20's builder,
  // not a builder of ours). In row order: the reader's live issue first, as the drawing has it.
  const rev = (issueId: string, title: string, criterionId: string | null) =>
    makeIssueRevision({ issueId, revision: 1, content: {
      title: { text: title, author: "H" }, description: { text: "", author: "H" },
      serves: criterionId === null ? [] : [makeServe({ criterionId })], implements: [] } });
  const serving: IssueRevision[] = [
    rev(WORK_IDS.ep167, "Resume offset per chunk record", resume.id),
    rev(WORK_IDS.ep163, "Split the restart fixture", resume.id),
    rev(WORK_IDS.ep150, "Chunked upload API", resume.id),
    rev(WORK_IDS.ep162, "Retry budget per chunk", resume.id),
    rev(WORK_IDS.ep161, "Resume after a browser refresh", resume.id),
    s.revisions.ep142,
    s.revisions.ep164,
    rev(WORK_IDS.ep166, "Image previews for uploaded evidence", upload.id),
    rev(WORK_IDS.ep160, "Imported: upload stalls on hotel wifi", null),
    rev(WORK_IDS.ep165, "Bump the chunk-store client", null),
  ];
  const revisions = Object.fromEntries(serving.map((r) => [r.issueId, r])) as Record<string, IssueRevision>;
  const under = (c: string) => serving.filter((r) => r.content.serves.some((x) => x.criterionId === c)).map((r) => r.issueId);
  return {
    projectKey: s.projectKey, featureName: "Resumable uploads", branch: "feat/resumable-uploads",
    reader: DYLAN, heads, revisions, briefKey: s.briefKey,
    groups: [
      { criterionId: resume.id, title: resume.text, convergence: "open", issueIds: under(resume.id) },
      { criterionId: upload.id, title: upload.text, convergence: "open", issueIds: under(upload.id) },
      { criterionId: null, title: "Serves no criterion", convergence: null,
        issueIds: serving.filter((r) => r.content.serves.length === 0).map((r) => r.issueId) },
    ],
    agent: { issueId: WORK_IDS.ep167, who: "dylan · agent 1", meaning: "writing the resume offset into store/chunks.go",
      status: "running", liveness: "live" },
    touches: [{ issueId: WORK_IDS.ep163, line: `${ALICE.displayName.split(" ")[0]} · touching restart_test.go` }],
    held: { issueId: WORK_IDS.ep162, cause: "→ done · own evidence at 4f1c2e9 (test run, exit 0) · 14:02" },
    departure: { alias: "EP-168", state: "cancelled" },
    replay: { issueId: WORK_IDS.ep150, to: "landed", cause: "→ landed · land commit 4f1c2e9 · 14:05" },
    selected: [WORK_IDS.ep167, WORK_IDS.ep150, WORK_IDS.ep161], cursor: WORK_IDS.ep164,
  };
}
```

`rev` calls task 20's `makeIssueRevision` with an override (`task-20.md:242-255`); the titles are
the prototype's words for the world's heads, which carry no title of their own (a head never
does; the title is revision content, `task-03.md:305-319`). `4f1c2e9` is the first seven of the world's `LAND`
(`task-20.md:295`). The times are the drawing's words, printed as given; the prototype formats
nothing.

#### Interfaces — complete: `desktop/src/prototype/work/Rows.tsx`

```tsx
// desktop/src/prototype/work/Rows.tsx — task 38. One row and one group, shared by Work and Plan.
import type { JSX, ReactNode } from "react";
import type { IssueHead } from "../../api/work";
import { AgentGlyph } from "../../components/glyphs/AgentGlyph";
import { MarkGlyph } from "../../components/glyphs/MarkGlyph";
import { StateGlyph } from "../../components/glyphs/StateGlyph";
import { ThreadLine, ThreadNode, useThreadLit } from "../../components/thread";
import type { ProtoAgent, ProtoGroup, ProtoTouch } from "./world";
import "./prototype.css";

export interface RowFacts {
  head: IssueHead; title: string; readerId: string;
  agent: ProtoAgent | null; touch: ProtoTouch | null; cause: string | null; unseen: boolean;
}

/** The row. Selected is the ring plus a leading check (a shape, 05 §4.1); lit never touches it. */
export function IssueRow(props: RowFacts & { selected: boolean; cursor: boolean; onFocus(): void }): JSX.Element {
  const { head, agent, touch } = props;
  const mine = head.assignee?.id === props.readerId;
  return (
    <div role="option" aria-selected={props.selected} tabIndex={props.cursor ? 0 : -1}
      className="pw-row" data-issue={head.id} data-changed={props.unseen ? "unseen" : undefined}
      onFocus={props.onFocus}>
      <div className="pw-ir">
        <span className="pw-sel">{props.selected ? <MarkGlyph kind="selected" size={14} /> : null}</span>
        <StateGlyph state={head.state} priority={head.priority} notProven={head.proof === "not_proven"} size={14} />
        <span className="pw-alias">{head.alias}</span>
        <span className="pw-title">{props.title}</span>
        <span className="pw-who">{head.assignee?.displayName.split(" ")[0] ?? "Unassigned"}</span>
      </div>
      {mine && agent !== null ? (
        <div className="pw-agent">
          <AgentGlyph status={agent.status} size={14} label={`${agent.who}, ${agent.status}`} />
          <span className="pw-agent-who">{agent.who}</span>
          <span className="pw-agent-meaning">{agent.meaning}</span>
          <span className="pw-live">{agent.liveness}</span>
        </div>
      ) : null}
      {!mine && touch !== null ? <div className="pw-touch">{touch.line}</div> : null}
      {head.threadState.kind === "not_tied" ? (
        <div className="pw-cause">not tied · {head.threadState.reason}</div>
      ) : null}
      {props.cause !== null ? <div className="pw-cause">{props.cause}</div> : null}
    </div>
  );
}

/** The group: the criterion in words (sans) and its plain size (mono), rows hanging from the
 *  line. The header binds lit fully; the rows bind focus only, so moving the pointer down a list
 *  does not light every group in turn (task 36, decision 2). */
export function Group(props: { group: ProtoGroup; briefKey: string; size: string; children: ReactNode }): JSX.Element {
  const { criterionId, title, convergence } = props.group;
  const { bind } = useThreadLit(criterionId);
  const rows = (
    <div role="presentation" onFocus={bind.onFocus} onBlur={bind.onBlur}>{props.children}</div>
  );
  return (
    <section className="pw-grp" aria-label={title} data-criterion={criterionId ?? "none"}>
      <div className="pw-gh" {...bind}>
        {criterionId === null ? <span className="pw-gh-gap" /> : (
          <ThreadNode criterionId={criterionId} href={`#peek/${props.briefKey}/${criterionId}`}
            label={`Brief criterion: ${title}`} />
        )}
        <span className="pw-gt">{title}</span>
        {convergence === null ? null : <span className="pw-conv">{convergence}</span>}
        <span className="pw-gs">{props.size}</span>
      </div>
      {criterionId === null ? rows : <ThreadLine criterionId={criterionId}>{rows}</ThreadLine>}
    </section>
  );
}
```

The first name only (`Dylan`, `Alice`) is the drawing's; a head with no assignee reads
*Unassigned*. `not tied · imported` and the cause lines are machine facts, so they are mono
(`.pw-cause`, task 30's rule); the agent's meaning and a teammate's line are sans.

#### Interfaces — complete: `desktop/src/prototype/work/WorkPrototype.tsx`

```tsx
// desktop/src/prototype/work/WorkPrototype.tsx — task 38. 05 §4.1's Work drawing on fixtures.
import { useEffect, useMemo, useRef, useState } from "react";
import type { JSX } from "react";
import type { IssueHead } from "../../api/work";
import { ViewHeader } from "../../components/common/ViewHeader";
import { useCockpit } from "../../store/store";
import { PlanPrototype, plainSize } from "./PlanPrototype";
import { Group, IssueRow } from "./Rows";
import { protoWorld } from "./world";
import type { ProtoWorld } from "./world";

/** The default export is the lazy target (mount.ts). One ghost button stands in for G W / G P. */
export default function WorkPrototype(): JSX.Element {
  const [screen, setScreen] = useState<"work" | "plan">("work");
  return screen === "work"
    ? <WorkScreen onPlan={() => setScreen("plan")} />
    : <PlanPrototype onWork={() => setScreen("work")} />;
}

export function WorkScreen(props: { onPlan?: () => void }): JSX.Element {
  const world: ProtoWorld = useMemo(protoWorld, []);
  const [heads, setHeads] = useState<Record<string, IssueHead>>(world.heads);
  const [groups, setGroups] = useState(world.groups);          // the order is frozen at mount
  const [moving, setMoving] = useState<Record<string, string>>({ [world.held.issueId]: world.held.cause });
  const [unseen, setUnseen] = useState<string[]>([]);
  const [ghost, setGhost] = useState<ProtoWorld["departure"] | null>(world.departure);
  const selection = useCockpit((s) => s.work.selection);
  const cursor = useCockpit((s) => s.work.cursor);
  const list = useRef<HTMLDivElement>(null);

  useEffect(() => {                                               // decision 5
    const { select, moveCursor } = useCockpit.getState();
    select(world.selected);
    moveCursor(world.cursor);
    list.current?.querySelector<HTMLElement>(`[data-issue="${world.cursor}"]`)?.focus();
  }, [world]);

  const replay = () => {                                          // decision 6: moves.hold, faked
    const { issueId, to, cause } = world.replay;
    setHeads((h) => ({ ...h, [issueId]: { ...h[issueId], state: to } }));
    setMoving((m) => ({ ...m, [issueId]: cause }));
    setUnseen((u) => [...u, issueId]);
  };
  const apply = () => {                                           // the stand-in for N
    // Each held row relocates to the end of its group. Where the product puts it is task 41's.
    const held = new Set(Object.keys(moving));
    setGroups((gs) => gs.map((g) => ({ ...g,
      issueIds: [...g.issueIds.filter((id) => !held.has(id)), ...g.issueIds.filter((id) => held.has(id))] })));
    setMoving({});
    setUnseen([]);
    setGhost(null);
  };

  return (
    <div className="pw-screen" data-screen="work">
      <ViewHeader title={`${world.projectKey} · ${world.featureName}`} route="global"
        badge={<span className="pw-branch">{world.branch}</span>}
        actions={<>
          <span className="pw-grouping">by criterion ▾</span>
          <kbd className="pw-kbd">Ctrl+K</kbd>
          {props.onPlan === undefined ? null : <button type="button" className="pw-ghost" onClick={props.onPlan}>Plan</button>}
        </>} />
      <div className="pw-dev" aria-label="Prototype controls">
        <button type="button" className="pw-ghost" onClick={replay}>Replay a land</button>
        <button type="button" className="pw-ghost" onClick={apply}>Apply held changes</button>
      </div>
      <div ref={list} role="listbox" aria-multiselectable="true" aria-label="Issues by criterion" className="pw-list">
        {groups.map((g) => {
          const held = g.issueIds.filter((id) => moving[id] !== undefined).length;
          const size = plainSize(g.issueIds.length, held);
          return (
            <Group key={g.criterionId ?? "none"} group={g} briefKey={world.briefKey} size={size}>
              {g.issueIds.map((id) => (
                <IssueRow key={id} head={heads[id]} title={world.revisions[id].content.title.text}
                  readerId={world.reader.id}
                  agent={world.agent.issueId === id ? world.agent : null}
                  touch={world.touches.find((t) => t.issueId === id) ?? null}
                  cause={moving[id] ?? null} unseen={unseen.includes(id)}
                  selected={selection.includes(id)} cursor={cursor === id}
                  onFocus={() => useCockpit.getState().moveCursor(id)} />
              ))}
            </Group>
          );
        })}
        {ghost === null ? null : (
          <div className="pw-ghostrow" role="presentation">
            <span>{ghost.alias} left this view · {ghost.state}</span><span className="pw-gs">N applies</span>
          </div>
        )}
      </div>
    </div>
  );
}
```

The dev strip is labelled *Prototype controls* and sits under the header, apart from the view's own
controls, so the owner never reads *Replay a land* as a product verb. `by criterion ▾` is a label,
not a control: display options are task 44's.

#### Interfaces — complete: `desktop/src/prototype/work/PlanPrototype.tsx`

```tsx
// desktop/src/prototype/work/PlanPrototype.tsx — task 38, decision 7. Plan (G P) on fixtures:
// Triage, Backlog, every criterion with its issues, My issues. No cycles, features board or views.
import { useMemo } from "react";
import type { JSX } from "react";
import { ViewHeader } from "../../components/common/ViewHeader";
import { useCockpit } from "../../store/store";
import { Group, IssueRow } from "./Rows";
import type { ProtoGroup } from "./world";
import { protoWorld } from "./world";

/** A plain size, Q1 A: a count of issues and, when any is held, of the held ones. Never "N of M". */
export function plainSize(issues: number, moving = 0): string {
  const n = `${issues} ${issues === 1 ? "issue" : "issues"}`;
  return moving === 0 ? n : `${n} · ${moving} moving`;
}

const CLOSED = new Set(["done", "cancelled", "duplicate"]);

export function PlanPrototype(props: { onWork?: () => void }): JSX.Element {
  const world = useMemo(protoWorld, []);
  const selection = useCockpit((s) => s.work.selection);
  const cursor = useCockpit((s) => s.work.cursor);
  const shown = Object.keys(world.revisions);                     // cancelled and duplicate are absent
  const inState = (st: string) => shown.filter((id) => world.heads[id].state === st);
  const bands: ProtoGroup[] = [
    { criterionId: null, title: "Triage", convergence: null, issueIds: inState("triage") },
    { criterionId: null, title: "Backlog", convergence: null, issueIds: inState("backlog") },
    ...world.groups.filter((g) => g.criterionId !== null),
    { criterionId: null, title: "My issues", convergence: null,
      issueIds: shown.filter((id) => world.heads[id].assignee?.id === world.reader.id && !CLOSED.has(world.heads[id].state)) },
  ];
  return (
    <div className="pw-screen" data-screen="plan">
      <ViewHeader title={`${world.projectKey} · Plan`} route="global"
        actions={props.onWork === undefined ? undefined
          : <button type="button" className="pw-ghost" onClick={props.onWork}>Work</button>} />
      <div role="listbox" aria-multiselectable="true" aria-label="Plan" className="pw-list">
        {bands.map((g) => (
          <Group key={`${g.title}`} group={g} briefKey={world.briefKey} size={plainSize(g.issueIds.length)}>
            {g.issueIds.map((id) => (
              <IssueRow key={`${g.title}/${id}`} head={world.heads[id]} title={world.revisions[id].content.title.text}
                readerId={world.reader.id} agent={world.agent.issueId === id ? world.agent : null}
                touch={world.touches.find((t) => t.issueId === id) ?? null} cause={null} unseen={false}
                selected={selection.includes(id)} cursor={cursor === id}
                onFocus={() => useCockpit.getState().moveCursor(id)} />
            ))}
          </Group>
        ))}
      </div>
    </div>
  );
}
```

An issue can sit in two bands (EP-167 is under its criterion and in *My issues*), so a row's React
key is band-scoped here. Task 45 decides whether the product shows it twice; the owner sees both
readings at task 39. Plan's Triage, Backlog and *My issues* use `Group` with `criterionId: null`, so
they draw no node and no line: only a criterion is on the thread.

#### Interfaces — complete: `desktop/src/prototype/work/prototype.css`

Tokens only (built: `--r-ctl`, `--r-chip`, `--mono`, `--sans`, `--ease`, `--dur-fast` in
`tokens.css`; task 26: `--row-h`, `--s-*`, `--t-*`, `--accent-tint`, `--inset`, `--surface`). No
hex, no `rgba()`, no `transition: all`; the flash and the tick are task 34's, keyed on
`data-changed="unseen"`, so this file draws no motion of its own.

```css
/* desktop/src/prototype/work/prototype.css — task 38. Owns .pw-* only. Deleted by task 40. */
.pw-screen { display: flex; flex-direction: column; min-height: 0; height: 100%; }
.pw-branch, .pw-kbd, .pw-alias, .pw-gs, .pw-agent-who, .pw-live, .pw-cause {
  font-family: var(--mono); font-size: var(--t-fact); line-height: var(--t-fact-lh);
  font-variant-numeric: tabular-nums;
}
.pw-branch { color: var(--ink-2); }
.pw-grouping { font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans); color: var(--ink-2); }
.pw-kbd { color: var(--ink-3); }
.pw-ghost {
  font: 500 var(--t-ui)/var(--t-ui-lh) var(--sans); color: var(--ink-2); background: none;
  border: 0; border-radius: var(--r-ctl); padding: var(--s-1) var(--s-2); cursor: default;
}
.pw-ghost:hover { background: var(--hover); color: var(--ink); }
.pw-ghost:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--accent-ring); }
.pw-dev { display: flex; gap: var(--s-2); padding: var(--s-1) var(--s-4); box-shadow: 0 1px 0 var(--line-soft); }
.pw-list { flex: 1; min-height: 0; overflow: auto; padding: var(--s-2) var(--s-4) var(--s-6); }

.pw-grp { margin-bottom: var(--s-3); }
.pw-gh { display: flex; align-items: center; gap: var(--s-2); padding: var(--s-2) var(--s-2) var(--s-1) var(--s-1);
  border-radius: var(--r-ctl); }
.pw-gh-gap { width: 14px; flex: none; }
.pw-gt { font: 600 var(--t-head)/var(--t-head-lh) var(--sans); color: var(--ink); min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pw-conv { font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans); color: var(--ink-2); }
.pw-gs { margin-left: auto; color: var(--ink-2); white-space: nowrap; }

.pw-row { position: relative; border-radius: var(--r-ctl); outline: none; }
.pw-ir { display: grid; grid-template-columns: 16px 16px 62px minmax(0, 1fr) auto; align-items: center;
  gap: var(--s-2); min-height: var(--row-h); padding: 0 var(--s-2); }
.pw-row:hover { background: var(--surface); }
.pw-row[tabindex="0"] { background: var(--inset); }
.pw-row:focus-visible { box-shadow: 0 0 0 2px var(--accent-ring); }
/* Selected is a ring and a check (decision 5); lit never reaches a row (task 36). */
.pw-row[aria-selected="true"] { background: var(--accent-tint); box-shadow: inset 0 0 0 1px var(--accent-ring); }
.pw-sel { display: inline-flex; color: var(--accent); }
.pw-alias { color: var(--ink-2); }
.pw-title { font: 400 var(--t-body)/var(--t-body-lh) var(--sans); color: var(--ink); min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pw-who { font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans); color: var(--ink-2); white-space: nowrap; }
.pw-agent { display: grid; grid-template-columns: 16px auto minmax(0, 1fr) auto; align-items: center;
  gap: var(--s-2); padding: 0 var(--s-2) var(--s-1) calc(16px + 16px + var(--s-2) * 2 + var(--s-2)); }
.pw-agent-who { color: var(--ink-2); }
.pw-agent-meaning, .pw-touch { font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans); color: var(--ink-2);
  min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pw-touch { padding: 0 var(--s-2) var(--s-1) calc(16px + 16px + 62px + var(--s-2) * 3 + var(--s-2)); }
.pw-live { color: var(--ink-3); }
.pw-cause { color: var(--ink-2); padding: 0 var(--s-2) var(--s-1) calc(16px + 16px + 62px + var(--s-2) * 3 + var(--s-2)); }
.pw-ghostrow { display: flex; justify-content: space-between; min-height: var(--row-h); align-items: center;
  padding: 0 var(--s-2) 0 calc(16px + 16px + var(--s-2) * 3); color: var(--ink-2);
  font: 400 var(--t-meta)/var(--t-meta-lh) var(--sans); }
```

`--t-head`, `--t-body`, `--t-meta` and `--t-ui` are task 26's; `.pw-gt` uses `--t-head` because a
criterion header is the list's heading. The selected ring is `inset`, so a selected row that is
also focused shows both rings, the outer one the focus.

#### Tests to write first — `desktop/src/prototype/work/prototype.test.tsx`

```tsx
// desktop/src/prototype/work/prototype.test.tsx — task 38. Each test names the mutant it kills.
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { resetFlags } from "../../lib/flags";
import { resetCockpit } from "../../store/store";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { WORK_IDS, workScenario } from "../../test/workFixtures";
import { PlanPrototype } from "./PlanPrototype";
import { WorkScreen } from "./WorkPrototype";

beforeEach(() => { localStorage.clear(); resetCockpit(); resetFlags(); });
afterEach(() => { vi.unstubAllEnvs(); vi.resetModules(); });

const groups = (c: HTMLElement) => [...c.querySelectorAll<HTMLElement>(".pw-grp")];
const row = (c: HTMLElement, id: string) => c.querySelector<HTMLElement>(`[data-issue="${id}"]`)!;
const agentGlyphs = (el: Element) => el.querySelectorAll('[data-glyph^="agent:"]');
const ids = (g: HTMLElement) => [...g.querySelectorAll<HTMLElement>(".pw-row")].map((r) => r.dataset.issue);

// Kills a %, a progress bar, an ETA or "1 of 3 proven" anywhere on either screen.
it("makes no prediction on either screen", () => {
  const work = render(<WorkScreen />);
  expectNoPrediction(work.container);
  work.unmount();
  expectNoPrediction(render(<PlanPrototype />).container);
});

// Kills grouping by workflow state (Linear's default; 05 §4.4 C13's structural check).
it("groups by criterion, not by state", () => {
  const { container } = render(<WorkScreen />);
  expect(groups(container).map((g) => g.dataset.criterion))
    .toEqual([WORK_IDS.criterionResume, WORK_IDS.criterionUpload, "none"]);
  // The StateGlyph is .pw-ir's own svg child; the selected mark sits inside .pw-sel.
  const states = ids(groups(container)[0]).map((id) => row(container, id!).querySelector(".pw-ir > svg")!.getAttribute("data-glyph"));
  expect(new Set(states).size).toBeGreaterThan(1);                 // one group, several states
});

// Kills sorting the groups by title ("Serves…" sorts before "the resume…" and "upload…").
it("puts Serves no criterion last", () => {
  const { container } = render(<WorkScreen />);
  const last = groups(container).at(-1)!;
  expect(last.getAttribute("aria-label")).toBe("Serves no criterion");
  expect(last.querySelector(".thr-node, .thr-line")).toBeNull();   // off the thread, so no node
});

// Kills a dot per agent, and an agent line on every row.
it("gives the reader's issue one agent line and one glyph", () => {
  const { container } = render(<WorkScreen />);
  const mine = row(container, WORK_IDS.ep167);
  expect(mine.querySelectorAll(".pw-agent")).toHaveLength(1);
  expect(agentGlyphs(mine)).toHaveLength(1);
  expect(agentGlyphs(container)).toHaveLength(1);                  // no other row has one
  expect(mine.querySelector(".pw-agent-who")!.textContent).toBe("dylan · agent 1");
});

// Kills "+2", a dot and a count on someone else's issue (05 §4.4 rule 3).
it("shows a teammate's issue with no dot and no count", () => {
  const { container } = render(<WorkScreen />);
  const theirs = row(container, WORK_IDS.ep163);
  expect(agentGlyphs(theirs)).toHaveLength(0);
  expect(theirs.querySelector(".pw-touch")!.textContent).toBe("Alice · touching restart_test.go");
  expect(theirs.textContent).not.toMatch(/\+\d|\d+\s*agents?/);
});

// Kills "2 agents" (or any counted noun) printed on a row; sizes belong to group headers only.
it("prints no count outside a group header", () => {
  const { container } = render(<WorkScreen />);
  for (const r of container.querySelectorAll(".pw-row")) {
    expect(r.textContent).not.toMatch(/\b\d+\s+(agents?|issues?|files?|moving|changes?)\b|\+\d+/);
  }
  expect(groups(container)[0].querySelector(".pw-gs")!.textContent).toBe("5 issues · 1 moving");
});
```

```tsx
// Kills lit drawn as selection (a ring or aria-selected on the lit row), and selection lighting a line.
it("shows lit and selected apart", () => {
  const { container } = render(<WorkScreen />);
  const selected = [...container.querySelectorAll('[aria-selected="true"]')].map((r) => r.getAttribute("data-issue"));
  expect(selected).toEqual([WORK_IDS.ep167, WORK_IDS.ep150, WORK_IDS.ep161]);
  for (const id of selected) expect(row(container, id!).querySelector(".pw-sel svg")).not.toBeNull();
  const lit = row(container, WORK_IDS.ep164);
  expect(document.activeElement).toBe(lit);                         // the cursor row has focus
  expect(lit.getAttribute("aria-selected")).toBe("false");
  expect(lit.querySelector(".pw-sel svg")).toBeNull();
  const [first, second] = groups(container);
  expect(second.querySelector<HTMLElement>(".thr-line")!.dataset.lit).toBe("true");
  expect(first.querySelector<HTMLElement>(".thr-line")!.dataset.lit).not.toBe("true");
});

// Kills a re-sort on arrival of the change (the row jumping), and a change with no cause line.
it("holds a replayed land in place until applied", () => {
  const { container } = render(<WorkScreen />);
  const before = ids(groups(container)[0]);
  fireEvent.click(screen.getByRole("button", { name: "Replay a land" }));
  expect(ids(groups(container)[0])).toEqual(before);
  const moved = row(container, WORK_IDS.ep150);
  expect(moved.dataset.changed).toBe("unseen");
  expect(moved.querySelector(".pw-ir > svg")!.getAttribute("data-glyph")).toBe("landed");
  expect(moved.querySelector(".pw-cause")!.textContent).toBe("→ landed · land commit 4f1c2e9 · 14:05");
  expect(groups(container)[0].querySelector(".pw-gs")!.textContent).toBe("5 issues · 2 moving");
  expect(container.querySelector(".pw-ghostrow")!.textContent).toBe("EP-168 left this view · cancelledN applies");
  fireEvent.click(screen.getByRole("button", { name: "Apply held changes" }));
  expect(ids(groups(container)[0]).slice(-2)).toEqual([WORK_IDS.ep150, WORK_IDS.ep162].sort(
    (a, b) => before.indexOf(a) - before.indexOf(b)));
  expect(groups(container)[0].querySelector(".pw-gs")!.textContent).toBe("5 issues");
  expect(container.querySelector(".pw-ghostrow")).toBeNull();
});

// Kills Plan drawn as Work (a missing band, or bands in another order) and cycles or views in W1.
it("draws Plan as Triage, Backlog, the criteria, then My issues", () => {
  const { container } = render(<PlanPrototype />);
  const [resume, upload] = workScenario().criteria;
  expect(groups(container).map((g) => g.getAttribute("aria-label")))
    .toEqual(["Triage", "Backlog", resume.text, upload.text, "My issues"]);
  expect(container.textContent).not.toMatch(/cycle|saved view/i);
});

// Kills shipping the prototype: an ungated lazy() or registration emits it into a production build.
it("does not exist in production", async () => {
  vi.stubEnv("DEV", false);
  vi.resetModules();
  const mount = await import("./mount");
  expect(mount.WorkPrototype).toBeNull();
});
```

The Plan test imports `workScenario` beside `WORK_IDS`; the criteria's words are task 20's
(`the resume test passes`, `task-20.md:227`; `upload works for PDF, XLSX and images`, `:300`), read
from the scenario so a fixture edit cannot silently pass. `vi.stubEnv` with a boolean for `DEV` is **UNVERIFIED** against vitest 5.0.1
(`desktop/package.json:35`); if it rejects a boolean, the test reads `mount.ts` as text through
`?raw` and asserts both uses of `lazy`/`import(` sit under `import.meta.env.DEV`, and the build
grep in Acceptance carries the claim. The ghost row's text has no space between its two spans,
which is why the assertion reads `cancelledN applies`.

