### Task 39: prototype, part 2: the issue view and the Desk; **the owner's review**, recorded

Stream **WD1**, no seam, about 3 hours (an estimate, not a measurement). Depends on task 38 (the
prototype directory, `world.ts`, `Rows.tsx`, `mount.ts`). It also places task 28's `StateGlyph`
(its `done_not_proven` key), task 29's `AgentGlyph`, task 31's `ViewHeader`, task 36's
`ThreadNode` and `useThreadLit`, and task 20's builders and `expectNoPrediction`; the skeleton's
row names 38 only (a gap, named in the return).

This is the `OD-1` gate for Phase W's desktop work: the owner looks at the work surfaces **in the
app, in the new look**, before tasks 40-45 are sized (05 §5.3 rows 10 and 12). It adds the last two
screens and the file that records what the owner said. It writes no verdict: until the owner has
looked, the file says *not yet reviewed*, and `feat/work-layer` is not merged onward (`OD-1`,
`OD-12`). Like task 38's screens, these read fixtures, talk to no Hub, and are deleted by the
task that builds the real surface (46-48 for the issue view, 62 for the Desk frame).

**Decisions made here, so nobody re-derives them:**

1. **One switch for four screens.** Task 38's default export switched Work and Plan with one ghost
   button in each header. It becomes a strip of four, *Work · Plan · Issue · Desk*
   (`aria-pressed` on the shown one), above the screen, labelled *Prototype screens*, so the owner
   moves between them without keys. The header buttons go (both props were optional for this).
2. **The issue view shows the world's live issue, EP-167, and EP-161.** 05 §2.1 draws EP-142 at
   `started` with an agent on it; task 38's decision 3 already stands EP-167 in for that row, so
   the Work row and the issue view agree. EP-161 is the world's `done` issue with
   `proof: "not_proven"` and a `stateReason` (`task-20.md:309`): it is the second fixture issue that
   reads ***done, not proven*** with its one-line reason. Two ghost buttons in the issue header
   switch between them.
3. **The overrides are in a new `issueWorld.ts`**, on top of `protoWorld()`, and declare no
   builder (task 20's rule): EP-167's revisions 2 and 3 (task 38's revision 1 stays first), its
   label, the files the lane was observed to change, and one *observed evidence* fact. Every value
   names the drawing line it stands in for.
4. **Nothing from W3.** No *proposed evidence* line, no *Record* button, no Orchestrator note: 05
   §2.1's drawing has all three, and W3 builds them (05 §5.3). The owner is told so in the review
   file, not on the screen.
5. **The serves line prints the fixture's key in full.** `brief:bf-00000000000000a2@2 ›
   c-91e2a4b0` in mono, then the criterion's words in sans, then the `ThreadNode`. The drawing's
   `bf-3a…@6 › done ›` abbreviates and names a section; task 20's `Serve` is `{ briefKey,
   criterionId }` (`task-20.md:175`) and has no section, and how the product shortens ids is task
   47's. The line binds `useThreadLit` fully, as task 36 decision 2 says the `serves` line does.
6. **The Desk's five sections are drawn in one fixed order, always,** NEEDS YOU → DECIDED WITHOUT
   YOU → FAILED → LANDED → RUNNING (05 §4.2, AX §5). An empty section keeps its place and says
   *None* in one line, so the order is visible even on a quiet day; DECIDED WITHOUT YOU is empty in
   W1, since only an Orchestrator agreement fills it (W3). Whether the product hides empty sections
   is task 62's.
7. **RUNNING folds.** One agent changed since the reader last looked (EP-150's agent 2, idle with a
   question, quiet 12m) gets the three-line card; the steady one folds into *1 steady, all live*,
   which expands in place. The number is a group size (Q1 A). No liveness word changes while the
   Desk is shown: the prototype has no clock at all, so this is true by construction, and task 62
   builds the rule.
8. **The zero state is a prototype control**, *Show the zero state*, which swaps the five sections
   for the one fact line of the demo's last frame: *Nothing needs you. EP-142 landed and proven at
   14:02.* No illustration, no projection. 05 §4.2's longer line adds `3 agents live on 2 issues`;
   the scope and the skeleton (`:29`) end at *14:02*, and this task follows them.
9. **The review file is a template of facts.** The implementer fills what a machine or a reading
   can establish (date, commit, scaling, themes, screens shown) before the session, and the rest
   only from the session itself: each check's result is the owner's call, written as the owner
   said it. The owner's words go in `>` quotes, dated and attributed. A file with no `>` line is
   the honest *not yet reviewed* state, not a failure and not a pass.

**Files** (the skeleton's file map has no row for any of these, as for task 38; named in the return):
- `desktop/src/prototype/work/issueWorld.ts` — the issue view's overrides (new)
- `desktop/src/prototype/work/IssuePrototype.tsx` — the issue view on fixtures (new)
- `desktop/src/prototype/work/DeskPrototype.tsx` — the Desk on fixtures (new)
- `desktop/src/prototype/work/IssueDesk.test.tsx` — seven tests (new)
- `desktop/src/prototype/work/WorkPrototype.tsx` ⚑ (task 38) — the four-way switch (modified)
- `desktop/src/prototype/work/register.tsx` ⚑ (task 38) — two more stress screens (modified)
- `desktop/src/prototype/work/prototype.css` ⚑ (task 38) — `.pw-switch`, `.pw-iv-*`, `.pw-desk-*` (modified)
- `docs/verification/2026-09-25-phase-w-prototype-review.md` — the review record (new; the
  directory exists and holds `2026-09-17-phase-1-results.md` and `phase-1-operator-runbook.md`)

#### Interfaces — complete: `desktop/src/prototype/work/issueWorld.ts`

```ts
// desktop/src/prototype/work/issueWorld.ts — task 39, decision 3. Overrides on protoWorld(); no
// builder is declared here (task 20's rule). Times and words are the drawing's, printed as given.
import type { IssueHead, IssueRevision, Label } from "../../api/work";
import { WORK_IDS, makeIssueRevision, makeLabel, workScenario } from "../../test/workFixtures";
import { protoWorld } from "./world";
import type { ProtoAgent, ProtoWorld } from "./world";

/** One line of the revision list, in the prototype's words (task 47 computes them). Newest first. */
export interface ProtoRevisionLine { revision: number; who: string; words: string; time: string }
/** The platform watched this proof (05 §2.1 rule 4): an observation, so no click records it. */
export interface ProtoObserved { criterionId: string; commit: string; what: string; by: string; time: string }

export interface ProtoIssue {
  head: IssueHead;
  /** The head revision: the title, the description and what the issue serves. */
  revision: IssueRevision;
  labels: Label[];
  /** The words of the criterion the issue serves; null when it serves none. */
  criterionText: string | null;
  agent: ProtoAgent | null;
  /** Paths the lane was observed to change, in the order seen (05 §2.1's `files` line). */
  files: string[];
  observed: ProtoObserved | null;
  revisions: ProtoRevisionLine[];
  /** "started 09:12"; null when the issue is not started. */
  startedWords: string | null;
}

export interface IssueWorld { world: ProtoWorld; live: ProtoIssue; notProven: ProtoIssue }

export function issueWorld(): IssueWorld {
  const world = protoWorld();
  const [resume] = workScenario().criteria;
  const r1 = world.revisions[WORK_IDS.ep167];                    // task 38's revision 1
  const r3 = makeIssueRevision({ issueId: WORK_IDS.ep167, revision: 3, content: { ...r1.content,
    description: { text: "Record the offset each chunk reached, so a resumed upload starts there.", author: "H" } } });
  const live: ProtoIssue = {
    head: { ...world.heads[WORK_IDS.ep167], labels: [WORK_IDS.label] },   // headRevision is 3 (task 20)
    revision: r3,
    labels: [makeLabel()],
    criterionText: resume.text,
    agent: world.agent,
    files: ["store/chunks.go", "store/chunks_test.go", "services/api/upload.go"],
    // 7c1f0a2 is the drawing's lane commit: EP-167 has not landed, so the world's LAND is not it.
    observed: { criterionId: resume.id, commit: "7c1f0a2", what: "resume test, exit 0", by: "run by the platform", time: "14:01" },
    revisions: [
      { revision: 3, who: "Dylan", words: "edited the description", time: "11:40" },
      { revision: 2, who: "Dylan", words: "wrote the description", time: "09:20" },
      { revision: 1, who: "Dylan", words: "created it, serving the resume test", time: "09:05" },
    ],
    startedWords: "started 09:12",
  };
  const notProven: ProtoIssue = {
    head: world.heads[WORK_IDS.ep161],                            // done, not_proven, with its stateReason
    revision: world.revisions[WORK_IDS.ep161],
    labels: [], criterionText: resume.text, agent: null, files: [], observed: null,
    revisions: [{ revision: 1, who: "Dylan", words: "created it, serving the resume test", time: "09:03" }],
    startedWords: null,
  };
  return { world, live, notProven };
}
```

Revision 2 has no content object here: the list shows it as a line, and nothing in this task
reads its body; restore and revision-pinned reading are task 47's. `makeLabel()`'s default name is
`resume` (`task-20.md:267-269`).

#### Interfaces — complete: `desktop/src/prototype/work/IssuePrototype.tsx`

```tsx
// desktop/src/prototype/work/IssuePrototype.tsx — task 39. 05 §2.1's issue drawing on fixtures,
// without W3 (decision 4): no proposed evidence, no Record, no Orchestrator note.
import { useMemo, useState } from "react";
import type { JSX } from "react";
import { ViewHeader } from "../../components/common/ViewHeader";
import { AgentGlyph } from "../../components/glyphs/AgentGlyph";
import { StateGlyph } from "../../components/glyphs/StateGlyph";
import { ThreadNode, useThreadLit } from "../../components/thread";
import { issueWorld } from "./issueWorld";
import type { ProtoIssue } from "./issueWorld";
import "./prototype.css";

export function IssuePrototype(): JSX.Element {
  const iw = useMemo(issueWorld, []);
  const [which, setWhich] = useState<"live" | "notProven">("live");
  const pick = (k: "live" | "notProven") => (
    <button type="button" className="pw-ghost" aria-pressed={which === k} onClick={() => setWhich(k)}>
      {iw[k].head.alias}
    </button>
  );
  return (
    <div className="pw-screen" data-screen="issue">
      <ViewHeader title={`${iw.world.projectKey} · ${iw.world.featureName}`} route="global"
        badge={<span className="pw-branch">{iw.world.branch}</span>} actions={<>{pick("live")}{pick("notProven")}</>} />
      <IssueBody issue={iw[which]} />
    </div>
  );
}

export function IssueBody(props: { issue: ProtoIssue }): JSX.Element {
  const { head, revision, labels, agent, files, observed, revisions, startedWords } = props.issue;
  const serve = revision.content.serves[0] ?? null;
  const notProven = head.state === "done" && head.proof === "not_proven";
  return (
    <article className="pw-iv" aria-label={`${head.alias} ${revision.content.title.text}`}>
      <div className="pw-iv-head">
        <StateGlyph state={head.state} priority={head.priority} notProven={notProven} size={16} />
        <span className="pw-alias">{head.alias}</span>
        <h2 className="pw-iv-title">{revision.content.title.text}</h2>
        <span className="pw-who">
          {head.assignee?.displayName.split(" ")[0] ?? "Unassigned"}{startedWords === null ? "" : ` · ${startedWords}`}
        </span>
        {labels.map((l) => <span key={l.id} className="pw-iv-label">{l.name}</span>)}
      </div>
      {notProven ? (
        <p className="pw-iv-np"><span className="pw-iv-np-mark">done, not proven</span>
          <span className="pw-iv-np-reason">{head.stateReason}</span></p>
      ) : null}
      {serve === null ? null
        : <ServesLine briefKey={serve.briefKey} criterionId={serve.criterionId} text={props.issue.criterionText} />}
      {agent === null ? null : (
        <div className="pw-agent">
          <AgentGlyph status={agent.status} size={14} label={`${agent.who}, ${agent.status}`} />
          <span className="pw-agent-who">{agent.who}</span>
          <span className="pw-agent-meaning">{agent.meaning}</span>
          <span className="pw-live">{agent.liveness}</span>
        </div>
      )}
      {files.length === 0 ? null : (
        <div className="pw-iv-line"><span className="pw-iv-k">files</span>
          {files.map((f) => <span key={f} className="pw-iv-mono">{f}</span>)}</div>
      )}
      {observed === null ? null : (
        <div className="pw-iv-line"><span className="pw-iv-k">observed evidence</span>
          <span className="pw-iv-mono">{`${observed.criterionId} at ${observed.commit}`}</span>
          <span className="pw-iv-words">{`${observed.what}, ${observed.by} ${observed.time}`}</span></div>
      )}
      <section className="pw-iv-revs" aria-label="Revisions">
        <h3 className="pw-iv-k">Revisions</h3>
        <ol>{revisions.map((r) => (
          <li key={r.revision}><span className="pw-iv-mono">r{r.revision}</span>
            <span className="pw-iv-words">{`${r.who} ${r.words}`}</span><span className="pw-iv-mono">{r.time}</span></li>
        ))}</ol>
      </section>
    </article>
  );
}

/** The key in mono, the criterion's words in sans, then the node; binds lit fully (task 36, 2). */
function ServesLine(props: { briefKey: string; criterionId: string; text: string | null }): JSX.Element {
  const { bind } = useThreadLit(props.criterionId);
  return (
    <div className="pw-iv-line pw-iv-serves" {...bind}>
      <span className="pw-iv-k">serves</span>
      <span className="pw-iv-key">{`${props.briefKey} › ${props.criterionId}`}</span>
      {props.text === null ? null : <span className="pw-iv-words">{props.text}</span>}
      <ThreadNode criterionId={props.criterionId} href={`#peek/${props.briefKey}/${props.criterionId}`}
        label={`Brief criterion: ${props.text ?? props.criterionId}`} />
    </div>
  );
}
```

*done, not proven* is sans, because it is the product's word for a state, and its reason sits on
the same line: the mark never appears without why (05 §2.1 rule 4, `task-20.md:309`'s
`stateReason`). The criterion id inside *observed evidence* is mono, as every machine id is.

#### Interfaces — complete: `desktop/src/prototype/work/DeskPrototype.tsx`

```tsx
// desktop/src/prototype/work/DeskPrototype.tsx — task 39. 05 §4.2's Desk on fixtures: the fixed
// sections in their fixed order (decision 6), RUNNING folded (decision 7), the zero state (8).
import { useMemo, useState } from "react";
import type { JSX } from "react";
import { ViewHeader } from "../../components/common/ViewHeader";
import { AgentGlyph } from "../../components/glyphs/AgentGlyph";
import type { AgentStatus } from "../../components/glyphs/AgentGlyph";
import { StateGlyph } from "../../components/glyphs/StateGlyph";
import { WORK_IDS, makeComment } from "../../test/workFixtures";
import { protoWorld } from "./world";
import "./prototype.css";

/** AX §5's order. Never re-ranked by content; a section with no rows keeps its place. */
export const DESK_ORDER = ["NEEDS YOU", "DECIDED WITHOUT YOU", "FAILED", "LANDED", "RUNNING"] as const;
export type DeskSection = (typeof DESK_ORDER)[number];

/** One issue row: why it is here, in one sans line. Override words, each naming its 05 §4.2 cell. */
interface DeskRow { issueId: string; why: string }
/** AX's three-line card, for an agent that changed since the reader last looked. */
interface AgentCard { issueId: string; who: string; status: AgentStatus; meaning: string; fact: string }

export const ZERO_LINE = "Nothing needs you. EP-142 landed and proven at 14:02.";

export function DeskPrototype(): JSX.Element {
  const world = useMemo(protoWorld, []);
  const [zero, setZero] = useState(false);
  const [unfolded, setUnfolded] = useState(false);
  const ask = makeComment();                                       // Alice's question to Dylan on EP-163
  const rows: Record<Exclude<DeskSection, "RUNNING">, DeskRow[]> = {
    "NEEDS YOU": [{ issueId: ask.issueId, why: `Alice asked you: ${ask.body.text}` }],     // "a comment naming you with a question"
    "DECIDED WITHOUT YOU": [],                                                          // W3 fills it
    FAILED: [{ issueId: WORK_IDS.ep160, why: "imported from CSV; no intake rule routed it" }], // "an intake rule could not route a request"
    LANDED: [{ issueId: WORK_IDS.ep142, why: "landed and proven at 14:02" },
             { issueId: WORK_IDS.ep164, why: "landed; its evidence is not in yet" }],     // "your issue landed"
  };
  const changed: AgentCard = { issueId: WORK_IDS.ep150, who: "dylan · agent 2", status: "idle",
    meaning: "asks which chunk size the brief means", fact: "idle · quiet 12m" };
  const steady = [world.agent];                                    // EP-167's agent 1, live
  const row = (r: DeskRow) => {
    const head = world.heads[r.issueId];
    return (
      <li key={r.issueId} className="pw-desk-row" data-issue={r.issueId}>
        <StateGlyph state={head.state} priority={head.priority} notProven={head.proof === "not_proven"} size={14} />
        <span className="pw-alias">{head.alias}</span>
        <span className="pw-title">{world.revisions[r.issueId].content.title.text}</span>
        <span className="pw-desk-why">{r.why}</span>
      </li>
    );
  };
  return (
    <div className="pw-screen" data-screen="desk">
      <ViewHeader title="Desk" route="global" />
      <div className="pw-dev" aria-label="Prototype controls">
        <button type="button" className="pw-ghost" aria-pressed={zero} onClick={() => setZero(!zero)}>Show the zero state</button>
      </div>
      {zero ? <p className="pw-desk-zero">{ZERO_LINE}</p> : (
        <div className="pw-list">
          {DESK_ORDER.map((name) => (
            <section key={name} className="pw-desk-sec" aria-label={name} data-section={name}>
              <h2 className="pw-desk-h">{name}</h2>
              {name !== "RUNNING" ? (
                rows[name].length === 0 ? <p className="pw-desk-none">None</p> : <ul>{rows[name].map(row)}</ul>
              ) : (<>
                <div className="pw-desk-card" data-issue={changed.issueId}>
                  <div><AgentGlyph status={changed.status} size={14} label={`${changed.who}, ${changed.status}`} />
                    <span className="pw-agent-who">{changed.who}</span>
                    <span className="pw-alias">{world.heads[changed.issueId].alias}</span></div>
                  <div className="pw-agent-meaning">{changed.meaning}</div>
                  <div className="pw-cause">{changed.fact}</div>
                </div>
                <button type="button" className="pw-ghost pw-desk-fold" aria-expanded={unfolded}
                  onClick={() => setUnfolded(!unfolded)}>{`${steady.length} steady, all live`}</button>
                {unfolded ? steady.map((a) => (
                  <div key={a.issueId} className="pw-agent"><AgentGlyph status={a.status} size={14} label={`${a.who}, ${a.status}`} />
                    <span className="pw-agent-who">{a.who}</span><span className="pw-agent-meaning">{a.meaning}</span></div>
                )) : null}
              </>)}
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
```

`makeComment()` is task 20's one comment (`task-20.md:257-265`): Alice to Dylan on EP-163, so the
NEEDS YOU row reads a fixture, not an invented line. The other `why` words are overrides.

