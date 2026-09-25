### Task 107: the agent's task-card page; *my agents* and *Alice's agents* filters; `who.label`

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on task
98 (`task.issueId`) and Phase T's TM-52 (the team read model and client: `PersonDetail`,
`PersonRow`, `AgentCard`, `createTeamClient().person`, `teamSlice`; Phase T `task-52.md:842-876`,
**UNVERIFIED until merge**). Uses task 44's display menu, task 91's view filter terms, task 60's
`peek.person`, task 59's palette, the replica (SW.16) for issue → criterion → brief, and SW.8.

TM-05 §0 calls it a *task card*: what one agent is doing, and why, in five fields. This part
draws it as a page for **any** agent on a project the reader belongs to, with one rule that
decides everything else on it: **you may look at another person's agent, never steer it**
(`OD-16`), and you never see its terminal, transcript or prompt (`OD-15`).

**Decisions made here, so nobody re-derives them:**

1. **`whoLabel` is the one place an agent is named.** It prints only the screen words of 05 §4.1's
   table (`05-design.md:855-859`): `dylan · agent 2`, `alice · orchestrator`, `you · agent 1`.
   The internal words *task* and *worker* never reach the screen, so 02's `you · worker 2 (agent)`
   (`02:321`) is superseded (P I5). `isMe` prints *you*; callers that list only the reader's own
   agents (task 48's issue line) pass the reader's handle instead, as 05's table shows.
2. **The five fields.** The task (its id in mono, never its title as the headline), its **reason**
   as one link that reads *task → issue → criterion → brief*, the status, the files observed, and
   the leases. The headline is the issue's title with its author class, because the task's own
   title is agent-written planning text and the issue is what a person asked for (05 §3.4).
3. **The reason resolves through the key, not the title.** From task 98's `issueId`, the replica's
   head revision gives the `issue:is-…@r` key and its `serves`; each served criterion links to its
   brief (`criterionHref`, task 47). A task with no `issueId` reads *not tied to an issue* and
   shows no invented reason.
4. **Status is a closed type of seven.** `AgentStatus7` is AX principle 8's set (`AGENTIC-UX.md:75`):
   `planning`, `executing`, `blocked_on_you`, `blocked_on_machine`, `paused`, `drifting`, `done`.
   TM-52 types `status` as `string`, so `statusOf(s)` maps its known words and answers
   `unknown` (printed *status not known*) for anything else, never passing a free string through.
5. **One action on another person's card.** *Ask my Orchestrator about this*, which opens Phase
   T's Orchestrator panel with `{kind: "agent", ref: ag-…}` as the subject (TM-52's `AskSubject`,
   `task-52.md:950`; TM-05 §9.2). On the reader's own card, *open session* is added (task 108's
   link). No pause, stop, message or steer control exists on any card.
6. **`agentsOf` is a view term, evaluated on the desktop.** Agents are Fleet state, not Hub state,
   so `agentsOf: "me" | u-…` is matched against the team read model at render time and never
   stored on an issue (skeleton rule 3); a saved view keeps only the term. *My agents* is
   `agentsOf: "me"`; *Alice's agents* is `agentsOf: u-alice`.
7. **A list, never a ratio.** The card lists the agent's tasks one per line; no *3 of 7*, no bar
   (L-187's *don't copy*; `OD-27`).

**Files:**
- `desktop/src/lib/whoLabel.ts` (+ `whoLabel.test.ts`) — `whoLabel`, `handleOf` (new)
- `desktop/src/components/work/AgentTaskCard.tsx` (+ `AgentTaskCard.test.tsx`) — the card page,
  `statusOf`, `AgentStatus7` (new)
- `desktop/src/components/work/DisplayMenu.tsx` ⚑ (task 44) — the *My agents* / *Alice's agents*
  filter entries (modified)
- task 91's view filter terms ⚑ (W2; the file is read on the sync base) — `agentsOf` (modified)
- `desktop/src/components/work/Peek.tsx` ⚑ (task 60) — `peek.person` opens the card (modified)
- `desktop/src/components/palette/commands.ts` ⚑ (tasks 59, 102) — *Open agent card…* (modified;
  not in the scope's list, a gap)
- `desktop/src/store/slices/workSlice.ts` ⚑ (task 37) — `openAgentCard(agentId)`, `agentCardId`
  (modified)

#### Interfaces — complete: `desktop/src/lib/whoLabel.ts`

```ts
// The one namer of an agent on screen (05 §4.1's words table). "task" and "worker" are internal
// words and never appear in what this returns; a test holds that over every input.
export type AgentRole = "orchestrator" | "agent";

export interface WhoInput {
  ownerHandle: string;   // handleOf(displayName): "dylan"
  isMe: boolean;         // true prints "you" (decision 1)
  role: AgentRole;       // TM-52's "worker" is passed as "agent" by roleOf
  ordinal: number;       // 1-based, stable per owner and project; ignored for the orchestrator
}

/** "Dylan Example" → "dylan": the first word, lower case, as task 48's line already prints it. */
export function handleOf(displayName: string): string {
  return (displayName.trim().split(/\s+/)[0] ?? displayName).toLowerCase();
}

export function roleOf(fleetRole: "orchestrator" | "worker"): AgentRole {
  return fleetRole === "orchestrator" ? "orchestrator" : "agent";
}

export function whoLabel(w: WhoInput): string {
  const who = w.isMe ? "you" : w.ownerHandle;
  if (w.role === "orchestrator") return `${who} · orchestrator`;
  if (!Number.isInteger(w.ordinal) || w.ordinal < 1) throw new RangeError(`whoLabel: ordinal ${w.ordinal}`);
  return `${who} · agent ${w.ordinal}`;
}
```

`ordinal` is the agent's place among its owner's workers on the project, in the order TM-52's
`PersonDetail.agents` lists them. Phase T does not state that order (gap 2); if it is not start
order, a finished agent could renumber the rest, which task 48 forbids for its own line.

#### Interfaces — complete: `desktop/src/components/work/AgentTaskCard.tsx`, part 1: the model

```tsx
// Five fields from TM-52's PersonDetail and the replica; no terminal, attach, transcript or prompt
// import anywhere in this file (OD-8, OD-15). Only the reader's own card carries "open session".
import type { JSX } from "react";
import type { PersonDetail } from "../../store/types";            // TM-52's
import { handleOf, roleOf, whoLabel } from "../../lib/whoLabel";

export type AgentStatus7 =
  | "planning" | "executing" | "blocked_on_you" | "blocked_on_machine" | "paused" | "drifting" | "done";
export const STATUS_WORDS: Readonly<Record<AgentStatus7 | "unknown", string>> = {
  planning: "planning", executing: "executing", blocked_on_you: "blocked on you",
  blocked_on_machine: "blocked on the machine", paused: "paused", drifting: "drifting", done: "done",
  unknown: "status not known",
};
const KNOWN = new Set<string>(Object.keys(STATUS_WORDS).filter((k) => k !== "unknown"));

/** TM-52's status is a string; only the seven words pass (decision 4). */
export function statusOf(s: string): AgentStatus7 | "unknown" {
  const k = s.trim().toLowerCase().replace(/[\s-]+/g, "_");
  return KNOWN.has(k) ? (k as AgentStatus7) : "unknown";
}

export interface CardReason {                  // task → issue → criterion → brief (decision 3)
  issueId: string | null; issueKey: string | null; alias: string | null;
  serves: { briefKey: string; criterionId: string; text: string | null }[];
}
export interface CardModel {
  agentId: string; who: string; mine: boolean; ownerId: string; status: AgentStatus7 | "unknown";
  taskId: string; headline: { text: string; author: "H" | "A" } | null;  // the issue's title, else null
  reason: CardReason; files: { path: string; lastEdit: string }[];
  leases: { id: string; globs: string[]; mode: "intent" | "hold" | "bulk"; expiresAt: string }[];
  tasks: { id: string; state: string }[];                               // a list, never a ratio
}

/** Pure: the detail, the agent's id, the reader's id, and a replica lookup for an issue's head
 *  revision. `issueIdOf` reads task 98's issueId from the detail's task row (gap 1). */
export function cardModel(d: PersonDetail, agentId: string, meId: string,
  issueIdOf: (taskId: string) => string | null,
  head: (issueId: string) => { key: string; alias: string; title: { text: string; author: "H" | "A" };
    serves: { briefKey: string; criterionId: string; text: string | null }[] } | null): CardModel | null {
  const workers = d.agents.filter((a) => a.role === "worker");
  const a = d.agents.find((x) => x.agentId === agentId);
  if (!a) return null;
  const mine = d.userId === meId;
  const who = whoLabel({ ownerHandle: handleOf(d.displayName), isMe: mine, role: roleOf(a.role),
    ordinal: a.role === "worker" ? workers.indexOf(a) + 1 : 1 });
  const issueId = issueIdOf(a.taskId);
  const h = issueId ? head(issueId) : null;
  return {
    agentId, who, mine, ownerId: d.userId, status: statusOf(a.status), taskId: a.taskId,
    headline: h?.title ?? null,
    reason: { issueId, issueKey: h?.key ?? null, alias: h?.alias ?? null, serves: h?.serves ?? [] },
    // PersonDetail.files is per person, not per agent (gap 3): the card says "files this person's
    // agents touched" until TM-52 carries an agentId on each file.
    files: d.files.map((f) => ({ path: f.path, lastEdit: f.lastEdit })),
    leases: d.leases.filter((l) => l.agentId === agentId).map(({ id, globs, mode, expiresAt }) => ({ id, globs, mode, expiresAt })),
    tasks: d.tasks.filter((t) => d.agents.some((x) => x.agentId === agentId && x.taskId === t.id)).map((t) => ({ id: t.id, state: t.state })),
  };
}
```

#### Interfaces — complete: `AgentTaskCard.tsx`, part 2: the page

```tsx
import { useCockpit } from "../../store/store";
import { AuthorClass } from "../issue/AuthorClass";
import { criterionHref } from "../issue/issueData";   // task 47's

function askMyOrchestrator(projectId: string, agentId: string): void {
  // TM-55's panel with the agent as the question's subject (TM-52 AskSubject); nothing is sent
  // until the person types and sends it there.
  useCockpit.getState().openOrchestratorPanel(projectId, { kind: "agent", ref: agentId });
}

export function AgentTaskCard(props: { projectId: string; model: CardModel; onOpenSession?(): void }): JSX.Element {
  const m = props.model;
  return (
    <article className="atc" aria-label={`${m.who}, what it is doing and why`}>
      <header className="atc-head">
        <span className="iv-fact atc-who">{m.who}</span>
        <span className="atc-status" data-status={m.status}>{STATUS_WORDS[m.status]}</span>
      </header>
      {m.headline
        ? <h2 className="atc-title"><AuthorClass author={m.headline.author} />{m.headline.text}</h2>
        : <h2 className="atc-title">not tied to an issue</h2>}
      <dl className="atc-fields">
        <dt>task</dt><dd className="iv-fact">{m.taskId}</dd>
        <dt>why</dt>
        <dd className="atc-reason">
          {m.reason.issueKey === null ? "not tied to an issue" : <>
            <span className="iv-fact">{m.reason.alias}</span> · <span className="iv-fact">{m.reason.issueKey}</span>
            {m.reason.serves.map((s) => (
              <span key={`${s.briefKey} ${s.criterionId}`}> · serves{" "}
                <a href={criterionHref(s.briefKey, s.criterionId)} className="iv-fact">{s.criterionId}</a>
                {s.text ? ` ${s.text}` : ""} in <span className="iv-fact">{s.briefKey}</span></span>
            ))}
          </>}
        </dd>
        <dt>tasks</dt>
        <dd><ul className="atc-tasks">{m.tasks.map((t) => <li key={t.id}><span className="iv-fact">{t.id}</span> {t.state}</li>)}</ul></dd>
        <dt>files</dt>
        <dd><ul className="atc-files">{m.files.map((f) => <li key={f.path} className="iv-fact">{f.path}</li>)}</ul></dd>
        <dt>leases</dt>
        <dd><ul className="atc-leases">{m.leases.map((l) => <li key={l.id}><span className="iv-fact">{l.globs.join(" ")}</span> · {l.mode}</li>)}</ul></dd>
      </dl>
      <footer className="atc-actions">
        {m.mine && props.onOpenSession && <button type="button" className="iv-link" onClick={props.onOpenSession}>open session</button>}
        <button type="button" onClick={() => askMyOrchestrator(props.projectId, m.agentId)}>Ask my Orchestrator about this</button>
      </footer>
    </article>
  );
}
```

`criterionHref` is task 47's hash link (`#<briefKey>/<criterionId>`), the one way a criterion is
linked on Work surfaces, so the reason's last hop reaches the brief the same way the issue view's
*serves* row does. The card is a page in the Work lens (`workSlice.agentCardId`), opened by
`openAgentCard(agentId)` from an agent line (task 40's `AgentLine` gains an `onClick` that calls
it), from `peek.person` (task 60: each agent line in the peek), and from the palette.

#### Interfaces — complete: the `agentsOf` term

```ts
// Task 44's DisplayOptions gains one optional field; task 91's view terms gain the same key, so a
// saved view keeps the term and never a list of issues (decision 6).
export type AgentsOf = "me" | `u-${string}`;
//   DisplayOptions { …; agentsOf?: AgentsOf | null }         task 44, null = no agent filter
//   ViewFilter term { kind: "agentsOf"; value: AgentsOf }     task 83's union, one added member

/** Evaluated at render time against the team read model; `me` is resolved to the reader's id
 *  here, never stored resolved. An owner whose detail is not loaded matches nothing. */
export function issuesWithAgentsOf(term: AgentsOf, meId: string,
  detailOf: (userId: string) => PersonDetail | undefined, issueIdOf: (taskId: string) => string | null): Set<string> {
  const d = detailOf(term === "me" ? meId : term);
  const out = new Set<string>();
  for (const a of d?.agents ?? []) {
    const id = issueIdOf(a.taskId);
    if (id !== null) out.add(id);
  }
  return out;
}
```

`DisplayMenu` lists *My agents* and one *<Name>'s agents* entry per person on the project's team
strip (`TeamStrip.people`, TM-52), names only and no counts (AX rule 10). In solo mode the team
read model is empty and the section is not shown; *My agents* in solo mode is the reader's own
local sessions with an `issueRef` (task 19), matched the way task 48's line is.

`AgentsOf` and `issuesWithAgentsOf` are exported from `AgentTaskCard.tsx`, beside the model they
read; the two type lines are the edits to tasks 44 and 83's files.

#### Tests to write first — `desktop/src/lib/whoLabel.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { handleOf, roleOf, whoLabel } from "./whoLabel";

describe("whoLabel", () => {
  it("prints 05 §4.1's three goldens", () => {
    expect(whoLabel({ ownerHandle: "dylan", isMe: false, role: "agent", ordinal: 2 })).toBe("dylan · agent 2");
    expect(whoLabel({ ownerHandle: "alice", isMe: false, role: "orchestrator", ordinal: 1 })).toBe("alice · orchestrator");
    expect(whoLabel({ ownerHandle: "dylan", isMe: true, role: "agent", ordinal: 1 })).toBe("you · agent 1");
  });
  it("never prints an internal word, for any role Fleet sends", () => {
    for (const r of ["orchestrator", "worker"] as const) {
      for (const isMe of [true, false]) {
        const s = whoLabel({ ownerHandle: handleOf("Alice Example"), isMe, role: roleOf(r), ordinal: 3 });
        expect(s).not.toMatch(/worker|task|session|lane/);
      }
    }
  });
  it("refuses an ordinal that is not a positive whole number", () => {
    expect(() => whoLabel({ ownerHandle: "dylan", isMe: false, role: "agent", ordinal: 0 })).toThrow(RangeError);
  });
});
```

#### Tests to write first — `desktop/src/components/work/AgentTaskCard.test.tsx`

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { BRIEF_KEY, WORK_IDS } from "../../test/workFixtures";
import { AgentTaskCard, cardModel, issuesWithAgentsOf, statusOf, type AgentStatus7 } from "./AgentTaskCard";
import src from "./AgentTaskCard.tsx?raw";

const alice = {
  v: 1, projectId: "pj-1", userId: WORK_IDS.alice, displayName: "Alice Example", you: false,
  tasks: [{ id: "tk-1", state: "running", title: null, briefSection: null }, { id: "tk-2", state: "queued", title: null, briefSection: null }],
  agents: [{ agentId: "ag-o", name: "o", role: "orchestrator", harness: "claude", taskId: "", status: "executing", card: { state: "", meaning: "", next: "" } },
           { agentId: "ag-w1", name: "w1", role: "worker", harness: "claude", taskId: "tk-1", status: "Executing", card: { state: "", meaning: "", next: "" } }],
  files: [{ path: "src/upload.ts", lastEdit: "2026-09-25T10:00:00Z", heldByOthers: false }],
  leases: [{ id: "ls-1", agentId: "ag-w1", globs: ["src/upload/**"], mode: "intent", state: "held", expiresAt: "2026-09-25T12:00:00Z" }],
  conflicts: [],
} as never;
const issueIdOf = (tk: string): string | null => (tk === "tk-1" ? WORK_IDS.ep150 : null);
const head = () => ({ key: `issue:${WORK_IDS.ep150}@3`, alias: "EP-150", title: { text: "Resume uploads after restart", author: "H" as const },
  serves: [{ briefKey: BRIEF_KEY, criterionId: WORK_IDS.criterionUpload, text: "an upload survives a restart" }] });
const model = () => cardModel(alice, "ag-w1", WORK_IDS.dylan, issueIdOf, head)!;

describe("AgentTaskCard", () => {
  it("offers no steer on another person's agent: only Ask my Orchestrator", () => {
    render(<AgentTaskCard projectId="pj-1" model={model()} onOpenSession={() => undefined} />);
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["Ask my Orchestrator about this"]);
  });
  it("has no terminal, transcript or prompt link, in the DOM or the source", () => {
    const { container } = render(<AgentTaskCard projectId="pj-1" model={model()} />);
    for (const a of container.querySelectorAll("a[href]")) expect(a.getAttribute("href")).toMatch(/^#brief:/);
    expect(src).not.toMatch(/attachRegistry|xterm|Terminal|transcript|openAttach|prompt/i);
  });
  it("resolves the reason through the issue key to the criterion and its brief", () => {
    const { container } = render(<AgentTaskCard projectId="pj-1" model={model()} />);
    expect(container.querySelector(".atc-reason")!.textContent).toContain(`issue:${WORK_IDS.ep150}@3`);
    expect(container.querySelector(`.atc-reason a[href='#${BRIEF_KEY}/${WORK_IDS.criterionUpload}']`)).not.toBeNull();
    expect(container.querySelector(".atc-title")!.textContent).toContain("Resume uploads after restart");
  });
  it("names the agent by who.label and lists tasks without a ratio", () => {
    const { container } = render(<AgentTaskCard projectId="pj-1" model={model()} />);
    expect(container.querySelector(".atc-who")!.textContent).toBe("alice · agent 1");
    expect(container.querySelectorAll(".atc-tasks li")).toHaveLength(1);
    expectNoPrediction(container);
  });
  it("passes only the seven statuses and types them closed", () => {
    expect(statusOf("Executing")).toBe("executing");
    expect(statusOf("thinking hard")).toBe("unknown");
    // @ts-expect-error a free-text status is not an AgentStatus7
    const s: AgentStatus7 = "thinking hard";
    expect(s).toBeTruthy();
  });
  it("finds Alice's agents' issues from the read model, never from a stored field", () => {
    expect([...issuesWithAgentsOf(WORK_IDS.alice as never, WORK_IDS.dylan, () => alice, issueIdOf)]).toEqual([WORK_IDS.ep150]);
    expect(issuesWithAgentsOf("me", WORK_IDS.dylan, () => undefined, issueIdOf).size).toBe(0);
  });
});
```

| Test | Mutant it kills |
|---|---|
| `whoLabel` goldens | "internal word shown"; *you* dropped for the reader |
| never an internal word | `worker 2` (02's superseded form) for any Fleet role |
| refuses a bad ordinal | `agent 0` or `agent NaN` from an agent missing from the list |
| no steer on another person's agent | "a steer button" (`OD-16`, TM-05 §9.2); *open session* on someone else's agent |
| no terminal, transcript or prompt link | "transcript leaks" (`OD-15`, `OD-8`) |
| the reason resolves through the key | "reason is the task title"; a criterion link that skips the brief |
| who.label, tasks without a ratio | "3 of 7" (L-187's *don't copy*, `OD-27`) |
| only the seven statuses | "free-text status" (AX principle 8), at runtime and at typecheck |
| Alice's agents from the read model | `agentsOf` stored on the issue (skeleton rule 3); `me` stored resolved |

The display menu's own test (`DisplayMenu.test.tsx` ⚑, task 44's file, two rows appended): *My
agents* sets `agentsOf: "me"` and the list shows only issues in `issuesWithAgentsOf` (mutant: the
filter ignored); in solo mode no *<Name>'s agents* entry is rendered (mutant: an empty team
section with nobody in it).

#### Steps

- [ ] **Step 1: read on the sync base** TM-52's `PersonDetail` (does each task row carry task 98's
  `issueId`? does each file carry an `agentId`? in what order are `agents` listed?), TM-55's panel
  action, task 44's `DisplayOptions`, task 83's `ViewFilter`, task 60's `Peek`, task 59's
  `commands.ts` and task 40's `AgentLine`. Rename in this part's files only.
- [ ] **Step 2: the tests**; `npm --prefix desktop test -- whoLabel AgentTaskCard DisplayMenu`
  fails on the missing modules.
- [ ] **Step 3: `whoLabel.ts`**; its test passes.
- [ ] **Step 4: `AgentTaskCard.tsx`**, the `workSlice` fields, and the edits to `DisplayMenu`,
  the view term, `Peek`, `AgentLine` and `commands.ts`; the `.atc*` rules in `work.css`.
- [ ] **Step 5: the acceptance commands.** Commit by path after steps 3 and 4, each message ending
  with the `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- whoLabel AgentTaskCard DisplayMenu work/Peek
# expect: every file passes; whoLabel 3 tests, AgentTaskCard 6, DisplayMenu gains 2;
#   tasks 40, 44 and 60's tests stay green
npm --prefix desktop run typecheck
# expect: exit 0 (the @ts-expect-error line holds: a free-text status does not compile)
grep -rnE "worker [0-9]|· worker" desktop/src/components desktop/src/lib | grep -v test | wc -l
# expect: 0
```

Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- That the read model's fields are live in the cloud (TM1, **UNVERIFIED**), or that any card field
  is fresher than TM-52's last read.
- That the files listed are this agent's own: TM-52 lists them per person (gap 3).
- Any steering: the only act is opening my own Orchestrator's panel; what it asks is the person's.
- A person's page (task 91) or the agent's transcript (never).

**Parity.** Closes **L-182**.

#### Gaps found while writing this part

1. **TM-52's `PersonDetail.tasks` has no `issueId`** (`task-52.md:869`); task 98 adds it to Fleet's
   task, so the read model must carry it too, an added field and an ask to Phase T.
2. **Agent order is unstated.** `whoLabel`'s ordinal needs `PersonDetail.agents` in start order (or
   a `startedAt` field); otherwise an agent renumbers when an earlier one finishes.
3. **Files are per person, not per agent** (`task-52.md:872`); the card's *files observed* needs an
   `agentId` on each file row.
4. **Task 107's dependsOn misses W2.** It edits task 44's `DisplayOptions` and task 83/91's view
   terms; a saved shared view with `agentsOf` also needs W2's Hub view filter (task 72, SW.18) to
   accept the added term. The row should read `98, 83, 91; TM-52`.
5. **The palette edit (`commands.ts`)** is not in the scope's file list, though the scope says the
   card opens *from the palette*.
6. **`whoLabel` versus task 48's own-agent line**: task 48 prints the reader's handle; this part
   keeps that for single-owner lists and prints *you* where people are mixed. The skeleton should
   record the one rule.
