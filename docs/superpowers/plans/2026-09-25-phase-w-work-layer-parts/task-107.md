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
    files: d.files.filter((f) => !f.heldByOthers).map((f) => ({ path: f.path, lastEdit: f.lastEdit })),
    leases: d.leases.filter((l) => l.agentId === agentId).map(({ id, globs, mode, expiresAt }) => ({ id, globs, mode, expiresAt })),
    tasks: d.tasks.filter((t) => d.agents.some((x) => x.agentId === agentId && x.taskId === t.id)).map((t) => ({ id: t.id, state: t.state })),
  };
}
```
