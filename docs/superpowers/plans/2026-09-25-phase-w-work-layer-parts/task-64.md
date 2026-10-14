### Task 64: the Desk: FAILED with *Restart from checkpoint* and *Abandon*; LANDED; RUNNING's fold

Stream **WD2**, about 3 hours (an estimate, not a measurement). Depends on task 62 (SW.17:
`registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskRank`, `DeskSnapshot` with
`lastVisitAt` and `sessions`, `task-62.md:126-157`, `:198-208`), and through their seams on task
19 (`Session.issueRef`, `task-19.md:300-303`), task 24 (SW.10's discard, `task-24.md:112-114`,
`:287-291`) and task 10 (SW.4's `IssueHead.proof`, `task-10.md:47-54`, `task-37.md:102`). Uses
task 63's block in `deskRows.tsx` (its `hhmm` and `stood`, `task-63.md:330-335`), task 61's
`firstSign` (`task-61.md:185-194`), task 40's `whoLabel` (`task-40.md:432-434`), task 37's
`WorkClient.discardStart` (`task-37.md:357-358`), task 29's `MarkGlyph`, task 56's `mutate`, the
built `api.restartSession` (`desktop/src/api/client.ts:153`, `:338`), `classify`
(`desktop/src/api/problems`), `formatElapsed` (`desktop/src/lib/time.ts:27`) and the heartbeat
(`hooks/heartbeat.ts:64`).

After this task the Desk's FAILED, LANDED and RUNNING sections hold rows: a failed agent on the
person's issue with the two answers that matter, what of theirs landed and how it was proven, and
their agents, with the ones that changed since they last looked drawn as three-line cards and the
rest folded into one line.

**What the reader sees** (05 §4.2's FAILED, LANDED and RUNNING rows and *RUNNING's shape*,
`05-design.md:938-948`; §3.3, `:736-753`; AX §5, `docs/AGENTIC-UX.md:148`;
`prototype/work-layer.html:767-785`):

- **A FAILED row**: `dylan · agent 1` and *EP-142*, its title, then *"exit at 02:14 · process
  exited"* in mono, the reason as the control plane wrote it. Under it *"ranked first: the agent on
  EP-142 stopped and nothing is working on it"* and *"if you do nothing: the worktree stays as it
  is and nothing resumes"*. Two buttons: *Restart from checkpoint* and *Abandon*.
- ***Restart from checkpoint*** answers in place: *"Restarting in the same worktree"*, then
  *"Restarted · running"* or the control plane's own sentence. The Desk does not move to Develop.
- ***Abandon*** asks first: the first press shows *"Abandon deletes the worktree and its
  uncommitted changes: upload/resume.go, upload/chunk.go"* and an *Abandon* button in `--need`;
  only the second press discards. The session record stays listed in Develop.
- **A LANDED row**: the `done` glyph, *EP-142*, and the word SW.4's result gives, never another:
  *"landed and proven at 14:02"*, *"proven, self-attested at 14:02"*, *"done, not proven at
  14:02"* in the *done, not proven* glyph, *"done, serves no criterion at 14:02"*, or, still
  `landed`, *"landed at 14:02, evidence waiting"*. When a sibling serving the same criterion is not
  landed, a second clause: *"done · c-91e2 still open (EP-150 not landed)"* (05 §3.3).
  A *Mark seen* button, with *Undo* for the undo window (6 s).
- **RUNNING**: one card per agent that **changed since the last visit**, exactly three lines:
  `dylan · agent 2 · idle · EP-160` with *"running 41m · last output 11m ago"*; the issue's title
  in `text·H`; and the next fact it waits on, as a rule: *"Waits for you: the agent is idle"*.
  Under the cards, one line: *"23 steady, all live"* (or *"21 steady, 2 quiet"*) with *Expand*;
  pressing it lists each steady agent in place, one line each.
- No row shows `~`, a `%`, a bar or a countdown (`OD-27`): every number is an observation.

**Decisions made here, so nobody re-derives them:**

1. **Five kinds, one pure file.** `work/desk/agents.ts` holds the selectors over a `DeskSnapshot`
   and returns `DeskItem`s; no React, no store, no clock (`snap.takenAt` is the now). The kinds:
   `lane.failed` (FAILED), `issue.landed` and `issue.done` (LANDED), `agent.changed` and
   `agent.steady` (RUNNING). `deskRows.tsx` registers each as a row and a source (task 62's
   decision 2), in a block after task 63's. Task 71 adds `issue.shipped` to LANDED beside these.
2. **An agent is on my issue when its `issueRef` starts `issue:<id>@` and the head is mine.** The
   prefix test is task 40's and task 60's (`task-40.md:426`, `task-60.md:106`): the desktop builds
   the prefix from a head's id and compares; it never splits the key (`task-19.md:301-302`). *Mine*
   is `head.assignee.id === snap.me`, as task 63's decision 2. A session with `issueRef === ""` is
   not on the Desk: Develop shows it.
3. **Failed means exited with a reason that is not my own stop.** `status === "exited"`,
   `exitReason !== ""` and `exitReason !== "killed"`. `"killed"` is the one reason the kill route
   writes (`control-plane/internal/api/sessions.go:1723-1726`), which is the person's own stop;
   the reaper's `"process exited"`, `"idle timeout"` and `"sandbox disappeared"`
   (`reaper/reaper.go:45-48`) are failures, and so is any other non-empty reason, shown verbatim.
   The exit time is the record's `updatedAt`: the registry stamps it on every write
   (`registry/filestore.go:321`) and `markExited` refuses to touch an exited record
   (`reaper.go:632-634`), so it is the exit's write unless a later rename touched it (gap 3).
   A failed session whose issue has a newer live session is not FAILED: the person already
   restarted or started again, and RUNNING shows that agent.
4. **Landed says only what SW.4's result says.** The five wordings come from one function over
   `IssueHead.state` and `IssueHead.proof` (task 10's result, `task-37.md:102`): `landed` with
   `proof.kind === "observed"` is *"landed and proven at HH:MM"*; `done` with
   `proof.kind === "observed"` is the same word *proven*; `done` with
   `proof.kind === "self"` is *"proven, self-attested at HH:MM"*; `done` with no proof and a
   `serves` is *"done, not proven at HH:MM"* in the *done, not proven* glyph; `done` with an
   empty `serves` is *"done, serves no criterion at HH:MM"*; `landed` with no proof yet is
   *"landed at HH:MM, evidence waiting"*. There is no sixth branch and no fallback sentence: an
   unmapped combination renders the state word alone, so a new proof kind never reads as proven.
5. **The sibling clause needs phase 4's loader, so it is conditional.** *"done · c-91e2 still
   open (EP-150 not landed)"* is drawn only when `snap.briefCriteria(...)` answers something
   other than `null` and the criterion's other servers are readable heads. With `null` (no P4 on
   the tree) the clause is absent and the row is still correct: it states less, never something
   unchecked. P4-16's loader is **UNVERIFIED until merge** (rule H2).
6. **Changed since the last visit is one predicate, and it is observational.** An agent of mine
   is `agent.changed` when any of: its `status` differs from what it was at `lastVisitAt`
   (from the session record's `statusChangedAt`, later than `lastVisitAt`); its status is
   `idle`; or `snap.takenAt - lastOutputAt > 10 * 60_000`. Every input is a fact the control
   plane wrote; nothing here infers what the agent is doing from what it printed (OD-8). On the
   first visit `lastVisitAt` is `null` and **every** live agent of mine is `agent.changed`: the
   person has not looked yet, so nothing is steady to them.
7. **The fold expands in place and counts what it says.** One `agent.steady` item carries the
   whole steady set, so the fold is one row in the model and one `aria-expanded` button in the
   DOM; expanding renders the set under it without a fetch and without moving the sections
   above. Its words are counted from the set, not from the total: *"23 steady, all live"* only
   when no member is `idle`, else *"21 steady, 2 idle"* with the idle count. 05's *quiet* and
   this plan's `idle` are the same word for the session's `idle` status; the row uses the
   status's own word, `idle`.
8. **The three lines are exactly three, and NEXT is a rule.** AX §5's card is STATE, MEANING,
   NEXT. STATE is the status word and `"running 41m · last output 3s ago"` from
   `formatElapsed`. MEANING is the issue's alias and title, marked `text·H`, because a person
   wrote it; W1 has no gateway to compose a sentence (W4 does). NEXT is chosen from a table of
   four facts — *Waits for you: the agent is idle*, *Waits for you: a comment is unanswered*,
   *Waits for the Hub: the lane is not fetched back*, *Nothing waits on you* — and never a time
   or a fraction (OD-27). A card with no fourth line is a test, not a convention.

**Files:**
- `desktop/src/work/desk/agents.ts` (+ `agents.test.ts`) — the five selectors, pure (new)
- `desktop/src/components/desk/rows/FailedRow.tsx` (+ test) — the FAILED row and its two verbs
- `desktop/src/components/desk/rows/LandedRow.tsx` — the five wordings and *Mark seen*
- `desktop/src/components/desk/rows/RunningCard.tsx` — AX §5's three lines
- `desktop/src/components/desk/rows/RunningFold.tsx` — the one-line fold and its expansion
- `desktop/src/components/desk/deskRows.tsx` ⚑ (task 62's file) — task 64's block, after 63's
- `desktop/src/components/desk/desk.css` ⚑ — the `.dk-fail`, `.dk-land`, `.dk-run` rules
- `desktop/src/components/desk/rows/RunningRows.test.tsx` — the card and the fold together

Nothing here reads another person's sessions: `snap.sessions` is this machine's own registry
(OD-15, and task 63's decision 4). Nothing imports `src/api/attachRegistry.ts` (OD-8).

#### Interfaces — complete: `desktop/src/work/desk/agents.ts`, part 1: the payloads and the tests

```ts
// desktop/src/work/desk/agents.ts — pure over a DeskSnapshot. No React, no store, no clock.
import type { DeskItem, DeskSnapshot } from "../../components/desk/deskModel";
import type { IssueHead } from "../../api/work";
import type { Session } from "../../api/client";

export interface FailedPayload {
  session: Session;
  head: IssueHead;
  exitReason: string;      // the control plane's own words, shown verbatim
  exitedAt: string;        // session.updatedAt (decision 3, gap 3)
  dirtyPaths: readonly string[];  // what Abandon deletes; [] when unknown (see below)
}
export type LandedWord =
  | "landed_proven" | "done_proven" | "done_self" | "done_unproven"
  | "done_no_criterion" | "landed_waiting";
export interface LandedPayload {
  head: IssueHead;
  word: LandedWord;
  at: string;                    // the state move's time, from the head
  openSibling: { criterionId: string; issueAlias: string } | null;  // decision 5
}
export interface AgentPayload {
  session: Session;
  head: IssueHead;
  why: "status" | "idle" | "quiet" | "first-visit";   // why it counts as changed
  next: NextFact;
}
export type NextFact =
  | { kind: "you-idle" } | { kind: "you-comment"; commentId: string }
  | { kind: "hub-lane" } | { kind: "none" };
export interface FoldPayload { steady: readonly AgentSteady[]; idle: number }
export interface AgentSteady { session: Session; head: IssueHead }

/** Sessions of mine bound to a head of mine. The prefix test is task 40's and 60's. */
export function myAgents(snap: DeskSnapshot): readonly { s: Session; h: IssueHead }[] {
  const out: { s: Session; h: IssueHead }[] = [];
  for (const s of snap.sessions) {
    if (!s.issueRef) continue;                       // Develop lists it, not the Desk
    const at = s.issueRef.indexOf("@");
    if (!s.issueRef.startsWith("issue:") || at < 0) continue;
    const id = s.issueRef.slice("issue:".length, at);
    const h = snap.headOf(id);
    if (!h || !snap.me || h.assignee?.id !== snap.me) continue;
    out.push({ s, h });
  }
  return out;
}

const REAPER_STOP = "killed";    // sessions.go:1723-1726, the person's own stop

export function failedItems(snap: DeskSnapshot): readonly DeskItem<FailedPayload>[] {
  const live = new Set(
    myAgents(snap).filter(({ s }) => s.status !== "exited").map(({ h }) => h.id));
  return myAgents(snap)
    .filter(({ s, h }) =>
      s.status === "exited" && s.exitReason !== "" &&
      s.exitReason !== REAPER_STOP && !live.has(h.id))          // decision 3
    .map(({ s, h }) => ({
      kind: "lane.failed",
      id: s.id,
      payload: { session: s, head: h, exitReason: s.exitReason, exitedAt: s.updatedAt,
                 dirtyPaths: s.dirtyPaths ?? [] },
      arrivedAt: s.updatedAt,
      rankReason: `ranked first: the agent on ${h.alias} stopped and nothing is working on it`,
      ifNothing: "if you do nothing: the worktree stays as it is and nothing resumes",
      issueId: h.id,
    }));
}
```

`Session.dirtyPaths` is **UNVERIFIED**: the registry record carries the worktree path
(`registry/session.go`), and task 24's discard names the uncommitted files in its own confirm
(`task-24.md:287-291`). Where the member is absent the confirm line names the worktree and not
the files, which is still a true consequence (gap 2).

#### Interfaces — complete: `agents.ts`, part 2: LANDED's word and RUNNING's two selectors

```ts
/** Decision 4. One table, no fallback sentence. */
export function landedWord(h: IssueHead): LandedWord | null {
  const proven = h.proof?.kind === "observed";
  if (h.state === "landed") return proven ? "landed_proven" : "landed_waiting";
  if (h.state !== "done") return null;
  if (proven) return "done_proven";
  if (h.proof?.kind === "self") return "done_self";
  return (h.serves?.length ?? 0) === 0 ? "done_no_criterion" : "done_unproven";
}

export function landedItems(snap: DeskSnapshot): readonly DeskItem<LandedPayload>[] {
  const out: DeskItem<LandedPayload>[] = [];
  for (const h of snap.heads.values()) {
    if (!snap.me || h.assignee?.id !== snap.me) continue;
    const word = landedWord(h);
    if (!word) continue;
    const at = h.stateChangedAt;
    if (snap.lastVisitAt !== null && Date.parse(at) < snap.lastVisitAt - SEEN_WINDOW) continue;
    out.push({
      kind: h.state === "landed" ? "issue.landed" : "issue.done",
      id: h.id, payload: { head: h, word, at, openSibling: openSibling(snap, h) },
      arrivedAt: at,
      rankReason: "ranked last: it is done and nothing is waiting on you",
      ifNothing: "if you do nothing: it stays on this list until you mark it seen",
      issueId: h.id,
    });
  }
  return out;
}
const SEEN_WINDOW = 24 * 60 * 60_000;   // a landing older than a day and already visited drops off

/** Decision 5: only where phase 4's loader answers. */
function openSibling(snap: DeskSnapshot, h: IssueHead): LandedPayload["openSibling"] {
  for (const c of h.serves ?? []) {
    let anyOpen: IssueHead | undefined;
    for (const other of snap.heads.values()) {
      if (other.id === h.id || !(other.serves ?? []).includes(c.criterionId)) continue;
      if (other.state !== "landed" && other.state !== "done") { anyOpen = other; break; }
    }
    if (anyOpen) return { criterionId: c.criterionId, issueAlias: anyOpen.alias };
  }
  return null;
}

/** Decision 6. Every input is a written fact; nothing is read from output bytes (OD-8). */
const QUIET_MS = 10 * 60_000;
export function changedAgents(snap: DeskSnapshot): readonly DeskItem<AgentPayload>[] {
  return myAgents(snap)
    .filter(({ s }) => s.status !== "exited")
    .map(({ s, h }) => ({ s, h, why: whyChanged(snap, s) }))
    .filter((r): r is { s: Session; h: IssueHead; why: AgentPayload["why"] } => r.why !== null)
    .map(({ s, h, why }) => ({
      kind: "agent.changed", id: s.id,
      payload: { session: s, head: h, why, next: nextFact(snap, s, h) },
      arrivedAt: s.statusChangedAt ?? s.updatedAt,
      rankReason: why === "idle" ? "ranked second: the agent is idle and waiting on you"
                                 : "ranked second: it changed since you last looked",
      ifNothing: "if you do nothing: it stays as it is",
      issueId: h.id,
    }));
}
function whyChanged(snap: DeskSnapshot, s: Session): AgentPayload["why"] | null {
  if (snap.lastVisitAt === null) return "first-visit";
  if (s.status === "idle") return "idle";
  const changed = Date.parse(s.statusChangedAt ?? s.updatedAt);
  if (changed > snap.lastVisitAt) return "status";
  const out = s.lastOutputAt ? Date.parse(s.lastOutputAt) : NaN;
  if (!Number.isNaN(out) && snap.takenAt - out > QUIET_MS) return "quiet";
  return null;
}
```

`Session.statusChangedAt` and `Session.lastOutputAt` are the registry's members as task 62's
snapshot passes them through; `lastOutputAt` is the time a byte was last written, a fact about
the stream and not about its content (OD-8). Both are **UNVERIFIED** against
`registry/session.go` at this writing (gap 1).

**Checked, and one of the two is not there.** `control-plane/internal/registry/session.go` holds
`ExitReason` (`:89`), `UpdatedAt` (`:98`) and `LastOutputAt` (`:99`), and **no
`StatusChangedAt`** (grepped 2026-09-27). So `whyChanged`'s `s.statusChangedAt ?? s.updatedAt`
resolves to `updatedAt` on today's record, which the registry stamps on every write
(`registry/filestore.go:321`): a status move is one of those writes, but so is a rename, so the
`"status"` branch can fire on a write that changed nothing a person cares about. That is a
false *changed*, never a false *steady*, so the card is over-offered and never missed. Adding
`StatusChangedAt` to the record is gap 1, and it is one field and one assignment in the writer.

```ts
/** Decision 8's table of four. Never a time, never a fraction (OD-27). */
function nextFact(snap: DeskSnapshot, s: Session, h: IssueHead): NextFact {
  if (s.status === "idle") return { kind: "you-idle" };
  const mine = (h.comments ?? []).find(
    (c) => !c.resolvedAt && c.mentions?.some((m) => m.id === snap.me));
  if (mine) return { kind: "you-comment", commentId: mine.id };
  if (h.state === "in_review" && !h.laneFetchedAt) return { kind: "hub-lane" };
  return { kind: "none" };
}

/** Decision 7. One item carries the whole steady set. */
export function steadyFold(snap: DeskSnapshot): readonly DeskItem<FoldPayload>[] {
  const changed = new Set(changedAgents(snap).map((i) => i.id));
  const steady = myAgents(snap)
    .filter(({ s }) => s.status !== "exited" && !changed.has(s.id))
    .map(({ s, h }) => ({ session: s, head: h }));
  if (steady.length === 0) return [];
  const idle = steady.filter(({ session }) => session.status === "idle").length;
  return [{
    kind: "agent.steady", id: "agent.steady",
    payload: { steady, idle },
    arrivedAt: new Date(snap.takenAt).toISOString(),
    rankReason: "ranked last: nothing about these changed since you last looked",
    ifNothing: "if you do nothing: they keep running",
  }];
}

export function foldLine(p: FoldPayload): string {
  return p.idle === 0
    ? `${p.steady.length} steady, all live`
    : `${p.steady.length - p.idle} steady, ${p.idle} idle`;
}
```

`foldLine` counts the idle out of the steady total, so *"21 steady, 2 idle"* and *"23 steady,
all live"* both add to 23: the reader can check the arithmetic on screen, which is the point of
the line. `h.comments`, `h.laneFetchedAt` and `h.stateChangedAt` are task 37's members as task
63 reads them (`task-63.md:119-126`); `laneFetchedAt` is **UNVERIFIED** and where it is absent
the `hub-lane` branch never fires and NEXT falls to *Nothing waits on you* (gap 4).

#### Interfaces — complete: `rows/FailedRow.tsx` and `rows/LandedRow.tsx`

```tsx
// desktop/src/components/desk/rows/FailedRow.tsx
export function FailedRow({ item }: { item: DeskItem<FailedPayload> }) {
  const { session: s, head: h, exitReason, exitedAt, dirtyPaths } = item.payload;
  const [armed, setArmed] = useState(false);          // confirm.inplace, decision 3 of task 62
  const [said, setSaid] = useState<string | null>(null);
  const restart = async () => {
    setSaid("Restarting in the same worktree");
    try { await api.restartSession(s.id); setSaid("Restarted · running"); }
    catch (e) { setSaid(classify(e).detail); }         // the control plane's own sentence
  };
  const abandon = async () => {
    if (!armed) { setArmed(true); return; }
    try { await discardStart(s.id); setSaid("Abandoned · the worktree is gone"); }
    catch (e) { setSaid(classify(e).detail); }
  };
  return (
    <div className="dk-fail" data-issue-id={h.id}>
      <div className="dk-fail-who">{whoLabel(s)} · {h.alias}</div>
      <div className="dk-fail-title">{h.title.text}</div>
      <div className="dk-fail-fact mono">exit at {hhmm(exitedAt)} · {exitReason}</div>
      <div className="dk-why">{item.rankReason}</div>
      <div className="dk-why">{item.ifNothing}</div>
```
      <div className="dk-verbs">
        <button type="button" onClick={restart}>Restart from checkpoint</button>
        <button type="button" className={armed ? "need" : undefined} onClick={abandon}>
          Abandon</button>
      </div>
      {armed && !said && (
        <div className="dk-consequence">
          Abandon deletes the worktree and its uncommitted changes
          {dirtyPaths.length > 0 ? `: ${dirtyPaths.join(", ")}` : ""}
        </div>
      )}
      {said && <div className="dk-said"><Said>{said}</Said></div>}
    </div>
  );
}
```

*Restart from checkpoint* calls the built route and nothing else: it does not navigate, it does
not touch the store, and the Desk stays where it is. The words *from checkpoint* are true
because the route resumes in the same worktree, so the session's own commits are the checkpoint;
there is no separate checkpoint object and the row never implies one.

```tsx
// desktop/src/components/desk/rows/LandedRow.tsx
const WORDS: Record<LandedWord, (t: string) => string> = {
  landed_proven:     (t) => `landed and proven at ${t}`,
  done_proven:       (t) => `landed and proven at ${t}`,
  done_self:         (t) => `proven, self-attested at ${t}`,
  done_unproven:     (t) => `done, not proven at ${t}`,
  done_no_criterion: (t) => `done, serves no criterion at ${t}`,
  landed_waiting:    (t) => `landed at ${t}, evidence waiting`,
};

export function LandedRow({ item }: { item: DeskItem<LandedPayload> }) {
  const { head: h, word, at, openSibling } = item.payload;
  const seen = () => mutate({ kind: "markSeen", issueId: h.id });   // optimistic.undo, U
  return (
    <div className="dk-land" data-issue-id={h.id}>
      <MarkGlyph state={h.state} proof={h.proof} />
      <span className="dk-land-alias mono">{h.alias}</span>
      <span className="dk-land-word">{WORDS[word](hhmm(at))}</span>
      {openSibling && (
        <span className="dk-land-open">
          {" · "}{openSibling.criterionId} still open ({openSibling.issueAlias} not landed)
        </span>
      )}
      <button type="button" className="dk-seen" onClick={seen}>Mark seen</button>
    </div>
  );
}
```

`MarkGlyph` is task 29's, which already draws *done, not proven* differently from *proven*;
this row passes the head's own `state` and `proof` and adds no glyph logic of its own. The
criterion id is mono because it is a machine id (05 §4.1). Task 71 registers `issue.shipped`
in this same section with the same shape.

#### Interfaces — complete: `rows/RunningCard.tsx` and `rows/RunningFold.tsx`

```tsx
// desktop/src/components/desk/rows/RunningCard.tsx — AX §5: exactly three lines.
const NEXT_WORDS: Record<NextFact["kind"], string> = {
  "you-idle":    "Waits for you: the agent is idle",
  "you-comment": "Waits for you: a comment is unanswered",
  "hub-lane":    "Waits for the Hub: the lane is not fetched back",
  "none":        "Nothing waits on you",
};

export function RunningCard({ item }: { item: DeskItem<AgentPayload> }) {
  const { session: s, head: h, next } = item.payload;
  const beat = useHeartbeat();                       // hooks/heartbeat.ts:64, one global clock
  return (
    <div className="dk-run" data-issue-id={h.id} data-beat={beat}>
      <div className="dk-run-state">
        {whoLabel(s)} · {s.status} · {h.alias}
        {" · "}<span className="tnum">running {formatElapsed(s.startedAt, beat)}</span>
        {" · "}<span className="tnum">last output {formatElapsed(s.lastOutputAt, beat)} ago</span>
      </div>
      <div className="dk-run-meaning"><AuthorClass author="H">{h.title.text}</AuthorClass></div>
      <div className="dk-run-next">{NEXT_WORDS[next.kind]}</div>
    </div>
  );
}
```

Three children, and the test counts them. The times are `tabular-nums` and both come from the
one heartbeat (DESIGN-SYSTEM.md: one global clock, never a timer per row); `formatElapsed` takes
the beat as its now, so twenty cards re-render together or not at all.

```tsx
// desktop/src/components/desk/rows/RunningFold.tsx
export function RunningFold({ item }: { item: DeskItem<FoldPayload> }) {
  const [open, setOpen] = useState(false);
  const p = item.payload;
  return (
    <div className="dk-fold">
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)}>
        {foldLine(p)}
      </button>
      {open && (
        <ul className="dk-fold-list">
          {p.steady.map(({ session: s, head: h }) => (
            <li key={s.id} data-issue-id={h.id}>
              {whoLabel(s)} · {s.status} · {h.alias} — {h.title.text}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

Expanding renders from the payload already in the model: no fetch, no store write, and the
sections above do not move, because the list grows below the button inside the same container.

#### Interfaces — complete: task 64's block in `deskRows.tsx` ⚑, and the CSS

```tsx
// desktop/src/components/desk/deskRows.tsx ⚑ — appended after task 63's block.
registerDeskRow<FailedPayload>("lane.failed", "failed",
  (i) => <FailedRow item={i} />, () => ({ tier: 0, waitingSinceMs: 0 }));
registerDeskSource("lane.failed", failedItems);

for (const kind of ["issue.landed", "issue.done"] as const) {
  registerDeskRow<LandedPayload>(kind, "landed",
    (i) => <LandedRow item={i} />, (i, s) => ({
      tier: 2, waitingSinceMs: s.takenAt - Date.parse(i.arrivedAt) }));
}
registerDeskSource("issue.landed", (s) => landedItems(s).filter((i) => i.kind === "issue.landed"));
registerDeskSource("issue.done",   (s) => landedItems(s).filter((i) => i.kind === "issue.done"));

registerDeskRow<AgentPayload>("agent.changed", "running",
  (i) => <RunningCard item={i} />, (i, s) => ({
    tier: i.payload.next.kind === "none" ? 2 : 1,
    waitingSinceMs: s.takenAt - Date.parse(i.arrivedAt) }));
registerDeskSource("agent.changed", changedAgents);

registerDeskRow<FoldPayload>("agent.steady", "running",
  (i) => <RunningFold item={i} />, () => ({ tier: 2, waitingSinceMs: 0 }));
registerDeskSource("agent.steady", steadyFold);
```

`"failed"`, `"landed"` and `"running"` are task 62's `DeskSection` names
(`task-62.md:112-118`). The two LANDED kinds share one render and one ranker and differ only in
the head's state, which is what keeps the five wordings in one place. A failed row is tier 0
with `waitingSinceMs: 0` because nothing is working: it sorts above every waiting row.

```css
/* desktop/src/components/desk/desk.css ⚑ — appended */
.dk-fail-fact, .dk-land-alias { font-family: var(--font-mono); }   /* OD-28: Commit Mono */
.dk-run-state .tnum { font-variant-numeric: tabular-nums; }
.dk-fail .need { box-shadow: var(--ring-need); }                    /* 1px ring, not a border */
.dk-consequence { color: var(--fg-need); margin-top: var(--sp-1); }
.dk-fold > button { display: block; width: 100%; text-align: left; }
.dk-fold-list { margin: var(--sp-1) 0 0; padding-left: var(--sp-3); }
.dk-run { transition: opacity var(--dur-row), transform var(--dur-row); }
```

No `transition: all`, no per-row spinner, no gradient (DESIGN-SYSTEM.md). The `--ring-need`,
`--fg-need`, `--sp-*` and `--dur-row` tokens are phase 2's; `--font-mono` is OD-28's Commit
Mono, which task 44 adds to the token file.

#### Tests to write first, part 1 — `desktop/src/work/desk/agents.test.ts` (the pure selectors)

```ts
// A DeskSnapshot literal per test: no React, no store, no timers.
// 1. An exited session whose exitReason is "killed" makes no FAILED row; one whose reason is
//    "process exited", "idle timeout", "sandbox disappeared" or "cgroup oom" each make one.
//    Mutant it kills: `status === "exited"` alone, so the person's own stop is called a failure
//    and the Desk asks them to restart what they deliberately killed.
// 2. A failed session whose issue has a second, live session makes no FAILED row (decision 3).
//    Mutant: the live-session filter dropped, so a restarted issue shows both.
// 3. landedWord covers the five: (landed, observed) → landed_proven; (done, observed) →
//    done_proven; (done, self) → done_self; (done, none, serves:[c]) → done_unproven;
//    (done, none, serves:[]) → done_no_criterion; (landed, none) → landed_waiting; and
//    (started, anything) → null. A head with proof.kind "attested-by-agent" (an invented kind)
//    → done_unproven, never proven.
//    Mutant: a fallback branch that returns done_proven for anything with a proof object.
// 4. openSibling is null when briefCriteria answers null, and names the open sibling when two
//    heads serve c-91e2 and one is started.
//    Mutant: the clause drawn from the head alone, which would claim a criterion is open with
//    no reading of the brief.
// 5. changedAgents: lastVisitAt null → every live agent of mine is changed, why "first-visit";
//    lastVisitAt set and nothing moved → none; status "idle" → changed, why "idle";
//    lastOutputAt 11 minutes before takenAt → changed, why "quiet"; 3 seconds before → not.
//    Mutant: the 10-minute test written against `updatedAt`, so a heartbeat write makes every
//    agent look changed forever.
// 6. steadyFold: with 25 of mine and 2 changed, the fold's payload holds 23 and no changed one;
//    with 2 of those idle, foldLine is "21 steady, 2 idle"; with none idle, "23 steady, all
//    live"; with 0 steady the selector returns [].
//    Mutant: foldLine says "all live" whatever the idle count, which is a lie on the screen.
// 7. A session of mine bound to an issue assigned to Alice makes no row of any kind, and a
//    session with issueRef "" makes none either.
//    Mutant: the assignee test dropped, so my Desk fills with Alice's work.
// 8. nextFact: idle → you-idle; an unresolved comment mentioning me → you-comment; in_review
//    with no laneFetchedAt → hub-lane; else none. Mutant: a time in the NEXT line.
```

#### Tests to write first, part 2 — the rows (`FailedRow.test.tsx`, `RunningRows.test.tsx`)

```tsx
// FailedRow.test.tsx, with api.restartSession and discardStart spied.
// 9.  Restart calls api.restartSession(s.id) exactly once, and the router's location is
//     unchanged after it resolves. Mutant: the row navigates to Develop, which loses the Desk.
// 10. Abandon needs two presses: after the first, discardStart has not been called and the
//     consequence line is on screen with the two dirty paths; after the second it is called
//     once. Mutant: one press discards, and an uncommitted worktree is gone on a mis-key.
// 11. A refused restart shows the control plane's own sentence from classify(e).detail and the
//     row stays. Mutant: a written-here sentence, so the real reason never reaches the person.
//
// RunningRows.test.tsx
// 12. A card has exactly three element children (STATE, MEANING, NEXT):
//     expect(card.children).toHaveLength(3). Mutant: a fourth line, which is where AX's
//     "will need you in ~4m" was copied in the prototype.
// 13. An agent with output 11 minutes old renders a card; one with output 3 seconds old and no
//     status move is inside the fold, not a card. Mutant: no fold at all, 25 cards on entry.
// 14. The fold's button has aria-expanded=false, and pressing it lists 23 <li> and leaves the
//     Desk's scrollTop unchanged. Mutant: aria-expanded missing (task 66's floor), or a
//     navigation to a separate agents screen.
// 15. Both times are in elements with font-variant-numeric: tabular-nums, and the card holds no
//     element with role="progressbar". Mutant: a progress bar for "how far through" a run.
// 16. expectNoPrediction(desk) over a fixture with all four kinds: no "~", no "%", no bar, no
//     countdown, no "will". Mutant: any AX §5 sentence copied verbatim with its estimate.
// 17. MarkGlyph draws the "done, not proven" glyph for done_unproven and the proven glyph for
//     done_proven, from the same fixture set as test 3. Mutant: one glyph for every landed row.
```

#### Steps

- [ ] Write `agents.test.ts` tests 1-8; it fails to import `./agents`. Write `agents.ts` until
      the eight pass. Mutants, one at a time, then revert: drop the `exitReason` tests (1); drop
      the live-session filter (2); add a `proof ? done_proven` fallback (3); draw openSibling
      without `briefCriteria` (4); test `updatedAt` for quiet (5); hard-code *all live* (6);
      drop the assignee test (7); put an elapsed time in NEXT (8).
- [ ] Write `FailedRow.tsx` and `FailedRow.test.tsx` (9-11). Mutants: navigate after restart
      (9); discard on the first press (10); a written-here refusal sentence (11).
- [ ] Write `LandedRow.tsx`; extend test 3's fixtures into test 17 for the glyph.
- [ ] Write `RunningCard.tsx`, `RunningFold.tsx` and `RunningRows.test.tsx` (12-16). Mutants: a
      fourth line (12); no fold (13); no `aria-expanded` (14); a progressbar (15); an AX
      sentence with its estimate (16).
- [ ] Append task 64's block to `deskRows.tsx` and the rules to `desk.css`. Task 62's twelve
      tests and task 63's eleven still pass unchanged.
- [ ] Run the whole desktop suite. Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/desk/agents.test.ts src/components/desk'
# expected: 5 files, 40 tests, all passing (17 here, 12 from task 62, 11 from task 63)
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "Date\.now|setInterval|setTimeout|from \"react\"" desktop/src/work/desk/agents.ts'
# expected: no output (decision 1: pure over the snapshot, one global clock)
~/.claude/tools/claudezsh/claudezsh 'grep -n "attachRegistry" desktop/src/components/desk/rows/RunningCard.tsx desktop/src/work/desk/agents.ts'
# expected: no output (OD-8: no meaning from PTY bytes)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "transition: all|linear-gradient" desktop/src/components/desk/desk.css'
# expected: no output (DESIGN-SYSTEM.md)
~/.claude/tools/claudezsh/claudezsh 'grep -c "WORDS\[" desktop/src/components/desk/rows/LandedRow.tsx'
# expected: 1 — one table, one lookup, no second branch (decision 4)
```

#### What this task does NOT claim

- **The issue's state after *Abandon*.** 05 names no fact that moves a `started` issue back when
  its worktree is discarded, so the issue keeps the state it had and the Desk says nothing about
  it (gap 5, reported).
- **A written MEANING line.** W1 has no gateway; MEANING is the person's own title, marked
  `text·H`. A sentence about what the agent is doing is W4's.
- **That 30 agents fit one screen at 125 %.** Measured in task 70, not asserted here.
- **That a restart resumes the agent's thinking.** It resumes a session in the same worktree;
  what the CLI does on resume is the CLI's, and nothing is typed into it (OD-8).
- **A teammate's agents.** `snap.sessions` is this machine's registry (OD-15). Team mode's Desk
  rows are TM-54's.
- **An intake-rule failure in FAILED.** W5's task 170 adds `request.unrouted` to this section.
- **Visual acceptance:** the owner's eyes (CLAUDE.md, working agreements).
- **Parity:** closes L-114's FAILED, LANDED and RUNNING sections with 62, 63 and 65; serves
  L-113, L-184 and L-189.

#### Gaps found while writing this part

1. **`StatusChangedAt` is not on the session record.** `registry/session.go` has `ExitReason`
   (`:89`), `UpdatedAt` (`:98`) and `LastOutputAt` (`:99`) and no status-change time (grepped
   2026-09-27), so decision 6's *status* branch reads `updatedAt` and can fire on a write that
   moved nothing. One field and one assignment in the writer closes it.
2. **`Session.dirtyPaths` is **UNVERIFIED**.** Task 24's discard names the uncommitted files in
   its own confirm; if the member is not on the record the consequence line names the worktree
   only, which is still true but less useful.
3. **The exit time is `updatedAt`.** The reaper refuses to touch an exited record
   (`reaper.go:632-634`), so it is the exit's own write unless a later rename touched it. An
   `ExitedAt` on the record would remove the *unless*.
4. **`IssueHead.laneFetchedAt` is **UNVERIFIED*** in task 37's type; without it `nextFact` never
   returns `hub-lane` and the card says *Nothing waits on you* for an unfetched lane.
5. **No fact moves an abandoned issue's state** (05 §3). Worth a sentence in 05: either the
   issue returns to `todo` on discard, or the person is asked.
6. The skeleton's file map has no `work/desk/agents.ts` and no `components/desk/rows/*`; the
   scope names them (`tasks-61-80.md:134-135`), and tasks 63, 64 and 65 all add to that folder.
7. `SEEN_WINDOW` (24 h) is this part's number: 05 says a landed row is marked seen, not how long
   an unseen one persists. If the Hub ever carries a `seenAt`, the window goes.
