### Task 108: `step.fold` on the issue; one ephemeral line; session, lane and preview links; the first-activity budget

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 107 (`whoLabel`, the card an agent line opens). Uses task 48's `IssueAgentLine`
(`task-48.md:317-363`), task 40's `AgentLine`, task 61's 10-second first sign on the row, the one
heartbeat (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts:64`, `BEAT_MS = 2400` at `:13`),
phase 3's OD-8 transcript adapter (`internal/transcript/claudecode`, phase 3 plan `:144`, its
task 17; **UNVERIFIED until merge**) and SW.8.

Three things an issue view says about each agent on it, all from facts:

1. **What it did, folded.** 02 `step.fold` (`02:320`) and AX §8.3 (`AGENTIC-UX.md:204`): *"the
   collapsed row carries command, duration, exit code and line count: `pytest -q · 43s · exit 1 ·
   812 lines`"*. Never the raw output, never a prose summary.
2. **Whether it is alive, on the beat.** One ephemeral line (02 `row.three`, `02:319`) whose
   liveness coarsens to `live` under 10 s, then `quiet 2m`, `quiet 14m`, and changes only when the
   one heartbeat ticks (05 §4.1, D I10; AX §8.8 *"Honest liveness only"*).
3. **The first-activity budget.** 02 `budget.ack` (`02:285`): 10 s from launch to the first sign
   of work, else the line says so as a fact, *"no output yet · 10s"*. Task 61 built it for Start
   on the row; this task gives every agent line on the issue view the same budget.

**Decisions made here, so nobody re-derives them:**

1. **Steps come only from the adapter, and the adapter is never required.** The fold reads a new
   control-plane read, `GET /v1/sessions/{id}/steps`, which asks phase 3's adapter for the
   session's shell tool calls. When the adapter declines (`*claudecode.Unreadable`: not pinned,
   missing, over size, another harness), the read answers `tier: "commits"` with the lane's
   commits and the session's exit, and the fold shows those; there is no error state (`OD-8`).
2. **The adapter delta is phase 3's file, extended.** `Adapter.Steps` is added beside
   `ToolCalls` (`task-17.md:119`) under the same pin, size and 1% rules, reading `Bash` tool calls
   and their results. Rule H2: if phase 3's adapter has not landed, this task waits. It is not in
   the skeleton's delta table (gap 1).
3. **A step is four fields and a command cut at 80 characters.** `command` (the tool call's own
   command string, cut with `…`), `durationMs` (the result's timestamp minus the call's),
   `exitCode` when the result states one, `lines` (the result's line count). The result's text is
   read to count lines and is never returned.
4. **The first sign is the earlier of the first step and the first observed output.** The control
   plane's `Session.lastOutputAt` (`desktop/src/store/types.ts:157`) is an observed timestamp, a
   mechanical fact AX §8.1 allows; it counts as a sign only once it is later than `createdAt`.
5. **Liveness words, one rule.** Under 10 s since the last sign: `live · last output 3s ago`;
   10-59 s: `quiet · under 1m`; then `quiet 2m`, `quiet 14m`, `quiet 1h 05m`. Computed from the
   beat's time, so the text can only change on a beat. No spinner, no `progressbar` role.
6. **A link renders only where its fact exists.** *open session* for the reader's own sessions
   (task 48's three store actions); the **worktree** link (the screen word for a lane, 05 §4.1)
   when the session has a `worktreeId`; *preview* only where phase 7's session record carries a
   URL (**UNVERIFIED** that it exposes one per session).
7. **Delegated agents get the line, not the fold.** A delegated agent is Fleet's managed session
   (TM-05); its liveness comes from TM-52's `PersonDetail.agents[].status` through task 107's card
   model, and it has no step fold, because the adapter runs beside a local transcript only (gap 2).

**Files:**
- `desktop/src/components/issue/StepFold.tsx` (+ `StepFold.test.tsx`) — the fold, `formatStep` (new)
- `desktop/src/components/issue/SessionLinks.tsx` (+ `SessionLinks.test.tsx`) — the links, the
  ephemeral line, `livenessWords`, `firstSign` (new)
- `desktop/src/components/issue/IssueAgentLine.tsx` ⚑ (task 48) — mounts both per agent (modified)
- `desktop/src/api/client.ts` ⚑ — `getSessionSteps(sessionId)` (modified)
- `control-plane/internal/transcript/claudecode/steps.go` (+ test) ⚑ (phase 3 task 17's package) —
  `Adapter.Steps` (new file in a sibling package; gap 1)
- `control-plane/internal/api/sessions.go` ⚑ (phase 2, P4 task 10), `routes_audit_test.go` ⚑ — the
  steps read and its catalog row (modified; gap 1)

#### Interfaces — complete: `control-plane/internal/transcript/claudecode/steps.go`

```go
package claudecode

import (
	"context"
	"time"
)

// MaxCommand is how much of a shell command a step keeps; the rest is cut with "…".
const MaxCommand = 80

// Step is one shell tool call, folded (02 step.fold). It never carries the call's output: the
// result is read to count its lines and to find its exit code, and nothing else leaves here.
type Step struct {
	ToolUseID  string    `json:"toolUseId"`
	Command    string    `json:"command"`            // input.command, first line, cut at MaxCommand
	At         time.Time `json:"at"`                 // the assistant line's timestamp
	DurationMs int64     `json:"durationMs"`         // the result line's timestamp minus At; 0 while pending
	ExitCode   *int      `json:"exitCode,omitempty"` // only when the result states one
	Lines      int       `json:"lines"`              // the result's line count, 0 while pending
	Pending    bool      `json:"pending"`            // no result line yet
}

// Steps reads every Bash tool call and its result, oldest first. It declines exactly as ToolCalls
// does (*Unreadable: gone, over MaxBytes, another pinned version, over 1% unreadable lines) and
// shares its (path, size, mtime) cache; a call whose result is missing is Pending, never dropped.
// The exit code is read only from the result's structured field for it, when the pinned version
// writes one; a result whose text mentions an exit code does not count (no meaning from prose).
func (a *Adapter) Steps(ctx context.Context, worktreeID, conversationID string) ([]Step, error)
```

#### Interfaces — complete: the steps read on the control plane

```text
GET /v1/sessions/{id}/steps
  200 {"tier": "steps",   "steps": [Step, …]}                          adapter read the transcript
  200 {"tier": "commits", "reason": "<Unreadable.Reason>",
       "commits": [{"sha": "7c1f0a2…", "at": "…", "files": 3}, …],
       "exit": {"code": 1, "at": "…"} | null}                          adapter declined (decision 1)
  404 {"code": "session_not_found"}
```

The handler lives beside phase 2's session routes (`internal/api/sessions.go`, P4 task 10 adds
`IssueRef` to it); it maps the session to its worktree and conversation id the way phase 3's
`getTranscriptSpan` route does (phase 3 `task-18.md:238`), and on `*Unreadable` reads the lane's
commits from phase 3's lane index (`internal/converge`, P3 tasks 07-09) and the session's exit
from the session record. **A commit's subject is not returned**: it is agent-written text, and a
fact line carries sha, time and file count only. Any other error is a 500 with the phase 2 problem
shape, and the desktop keeps the last answer.

#### Tests to write first — `steps_test.go` (package `claudecode`) and the route rows

| Test | Fixture → expected | Mutant it kills |
|---|---|---|
| `TestStepsFoldsABashCall` | one `Bash` call `pytest -q`, a result 43 s later with 812 lines and exit 1 → one Step `{pytest -q, 43000, 1, 812}` | the raw output returned; the duration from the wrong line |
| `TestStepsCutsALongCommand` | a 200-character command with a newline at 120 → the first line, 80 characters, ending `…` | a full script copied into the fold |
| `TestStepsKeepsAPendingCall` | a call with no result line → `Pending: true`, `Lines: 0`, no `exitCode` | an unfinished step dropped, so the fold hides work in progress |
| `TestStepsExitCodeOnlyFromTheField` | a result whose text says `exit code 0` but whose field says 2 → 2; no field → omitted | meaning derived from output text |
| `TestStepsDeclinesLikeToolCalls` | the same three fixtures `TestVersionlessLinesAreIgnored` and the pin tests use → the same `*Unreadable` reasons | a second, looser parser |
| `TestStepsRouteDegradesToCommits` (`internal/api`) | a session whose transcript is missing → 200 `tier: "commits"` with the lane's two commits and no `subject` key | "the adapter is required" (`OD-8`); agent text in a fact line |
| `TestStepsRouteIsCatalogued` (`routes_audit_test.go`) | the route has its catalog row | an unlisted route |

#### Interfaces — complete: `desktop/src/components/issue/StepFold.tsx`

```tsx
// Typed steps only. This file imports nothing from the terminal, the attach registry, xterm or a
// PTY stream (OD-8); a test holds that by source. It starts no timer.
import type { JSX } from "react";

export interface Step { command: string; durationMs: number; exitCode?: number; lines?: number; pending?: boolean; at?: string }
export interface CommitFact { sha: string; at: string; files: number }
export type StepsAnswer =
  | { tier: "steps"; steps: Step[] }
  | { tier: "commits"; reason: string; commits: CommitFact[]; exit: { code: number; at: string } | null };

/** "43s", "2m 05s", "1h 02m": a duration, never a remaining time. */
export function formatDuration(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}

/** 02 step.fold: `pytest -q · 43s · exit 1 · 812 lines`. A field that is absent is left out. */
export function formatStep(st: Step): string {
  if (st.pending) return `${st.command} · running`;
  const parts = [st.command, formatDuration(st.durationMs)];
  if (st.exitCode !== undefined) parts.push(`exit ${st.exitCode}`);
  if (st.lines !== undefined) parts.push(`${st.lines} ${st.lines === 1 ? "line" : "lines"}`);
  return parts.join(" · ");
}

/** The last `max` steps, oldest first; the count of earlier ones is a plain count, not a ratio. */
export function StepFold(props: { answer: StepsAnswer; max?: number }): JSX.Element | null {
  const { answer, max = 5 } = props;
  if (answer.tier === "commits") {
    if (answer.commits.length === 0 && answer.exit === null) return null;
    return (
      <ul className="iv-steps" data-tier="commits" aria-label="Commits from this agent" title={answer.reason}>
        {answer.commits.map((c) => (
          <li key={c.sha} className="iv-step iv-fact">commit {c.sha.slice(0, 7)} · {c.files} {c.files === 1 ? "file" : "files"}</li>
        ))}
        {answer.exit && <li className="iv-step iv-fact">exited · exit {answer.exit.code}</li>}
      </ul>
    );
  }
  if (answer.steps.length === 0) return null;
  const shown = answer.steps.slice(-max);
  const earlier = answer.steps.length - shown.length;
  return (
    <ul className="iv-steps" data-tier="steps" aria-label="Steps this agent ran">
      {earlier > 0 && <li className="iv-step-earlier">{earlier} earlier {earlier === 1 ? "step" : "steps"}</li>}
      {shown.map((st, i) => <li key={i} className="iv-step iv-fact" data-exit={st.exitCode ?? ""}>{formatStep(st)}</li>)}
    </ul>
  );
}
```

`.iv-step` is Commit Mono at `--t-fact` with `tabular-nums` (skeleton *Faces*, `OD-28`); a
non-zero exit is marked by the word, never by colour alone. The adapter's decline reason is not
shown as an error: the commits tier carries it in the list's `title` for the curious, and the
line itself reads as facts.

#### Interfaces — complete: `desktop/src/components/issue/SessionLinks.tsx`

```tsx
// The ephemeral line and the links. Every word here is computed from the beat's time, so it can
// only change when the one heartbeat ticks; this file starts no timer (DESIGN-SYSTEM.md).
import type { JSX } from "react";
import { useHeartbeat } from "../../hooks/heartbeat";
import { useCockpit } from "../../store/store";
import type { Session } from "../../store/types";
import type { StepsAnswer } from "./StepFold";

export const BUDGET_MS = 10_000;                 // 02 budget.ack

/** Decision 4: the earlier of the first step and the first observed output after launch. */
export function firstSign(s: Session, steps: StepsAnswer | null): number | null {
  const out = Date.parse(s.lastOutputAt) > Date.parse(s.createdAt) ? Date.parse(s.lastOutputAt) : null;
  const st = steps?.tier === "steps" && steps.steps.length > 0 ? Date.parse(steps.steps[0].at ?? "") : NaN;
  const cands = [out, Number.isNaN(st) ? null : st].filter((x): x is number => x !== null);
  return cands.length === 0 ? null : Math.min(...cands);
}

const secs = (ms: number): number => Math.max(0, Math.floor(ms / 1000));

/** Decision 5's words, from the last sign and the beat's time. */
export function livenessWords(lastSignMs: number, beatMs: number): string {
  const s = secs(beatMs - lastSignMs);
  if (s < 10) return `live · last output ${s}s ago`;
  if (s < 60) return "quiet · under 1m";
  const m = Math.floor(s / 60);
  return m < 60 ? `quiet ${m}m` : `quiet ${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}

/** The one ephemeral line (02 row.three): starting, the budget fact, liveness, or exited. */
export function ephemeralLine(s: Session, steps: StepsAnswer | null, beatMs: number): string {
  if (s.status === "exited") return "exited";
  const sign = firstSign(s, steps);
  if (sign === null) {
    const waited = beatMs - Date.parse(s.createdAt);
    return waited < BUDGET_MS ? "starting" : `no output yet · ${secs(waited) < 60 ? `${secs(waited)}s` : `${Math.floor(secs(waited) / 60)}m`}`;
  }
  return livenessWords(Math.max(sign, Date.parse(s.lastOutputAt)), beatMs);
}

export function SessionLinks(props: { session: Session; mine: boolean; previewUrl: string | null }): JSX.Element {
  const { session: s, mine, previewUrl } = props;
  const st = useCockpit.getState;
  return (
    <span className="iv-links">
      {mine && <button type="button" className="iv-link" onClick={() => { st().selectWorktree(s.worktreeId); st().focusSession(s.id); st().setLens("develop"); }}>open session</button>}
      {s.worktreeId && <button type="button" className="iv-link" onClick={() => st().selectWorktree(s.worktreeId)}>worktree</button>}
      {previewUrl && <a className="iv-link" href={previewUrl} target="_blank" rel="noreferrer">preview</a>}
    </span>
  );
}

export function EphemeralLine(props: { session: Session; steps: StepsAnswer | null }): JSX.Element {
  const beat = useHeartbeat();
  // aria-hidden: liveness is not announced (task 66's rule); the row's status word carries meaning.
  return <span className="iv-ephemeral iv-fact" aria-hidden="true">{ephemeralLine(props.session, props.steps, beat)}</span>;
}
```

`Step.at` is the adapter's timestamp for the call, carried through the steps read. Phase 7's
preview URL arrives as a prop from the session's deploy record where one exists; with none the
link is simply absent.

#### Interfaces — complete: the edit to task 48's `IssueAgentLine.tsx`

```tsx
// Inside each own-agent <li>, after task 40's AgentLine and replacing task 48's lone
// "open session" button:
<EphemeralLine session={s} steps={steps[s.id] ?? null} />
<SessionLinks session={s} mine previewUrl={previewOf(s.id)} />
<StepFold answer={steps[s.id] ?? { tier: "steps", steps: [] }} />
// `steps` is one getSessionSteps read per own session, again when the session's lastOutputAt
// moves (at most once a beat); delegated agents (decision 7) get EphemeralLine from task 107's
// card model and no StepFold.
```

#### Tests to write first — `StepFold.test.tsx` and `SessionLinks.test.tsx`

```tsx
// StepFold.test.tsx
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { StepFold, formatStep } from "./StepFold";
import src from "./StepFold.tsx?raw";

describe("StepFold", () => {
  it("prints the four fields in mono, as 02 step.fold does", () => {
    expect(formatStep({ command: "pytest -q", durationMs: 43_000, exitCode: 1, lines: 812 })).toBe("pytest -q · 43s · exit 1 · 812 lines");
    const { container } = render(<StepFold answer={{ tier: "steps", steps: [{ command: "pytest -q", durationMs: 43_000, exitCode: 1, lines: 812 }] }} />);
    expect(container.querySelector(".iv-step.iv-fact")!.textContent).toBe("pytest -q · 43s · exit 1 · 812 lines");
  });
  it("has no field for raw output, so none can be shown", () => {
    const leaky = { command: "ls", durationMs: 10, lines: 2, output: "SECRET_TOKEN=abc" } as never;
    const { container } = render(<StepFold answer={{ tier: "steps", steps: [leaky] }} />);
    expect(container.textContent).not.toContain("SECRET_TOKEN");
  });
  it("degrades to commit facts, with no error banner and no commit subject", () => {
    const { container } = render(<StepFold answer={{ tier: "commits", reason: "the transcript is gone", commits: [{ sha: "7c1f0a2e5b9d", at: "2026-09-25T10:41:00Z", files: 3 }], exit: { code: 0, at: "2026-09-25T10:42:00Z" } }} />);
    expect(container.textContent).toBe("commit 7c1f0a2 · 3 filesexited · exit 0");
    expect(container.querySelector("[role='alert'], .error, .banner")).toBeNull();
  });
  it("imports nothing from the terminal or a PTY, and predicts nothing", () => {
    expect(src).not.toMatch(/attachRegistry|xterm|Terminal|pty|openAttach|setInterval|setTimeout/i);
    const steps = Array.from({ length: 8 }, (_, i) => ({ command: `step ${i}`, durationMs: 1000, lines: 1 }));
    const { container } = render(<StepFold answer={{ tier: "steps", steps }} />);
    expect(container.textContent).toContain("3 earlier steps");
    expectNoPrediction(container);
  });
});
```

```tsx
// SessionLinks.test.tsx
import { act, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { __resetHeartbeat, BEAT_MS } from "../../hooks/heartbeat";
import { aSession } from "../../test/fixtures";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { EphemeralLine, SessionLinks, ephemeralLine, livenessWords } from "./SessionLinks";
import src from "./SessionLinks.tsx?raw";

const T0 = Date.parse("2026-09-25T10:00:00Z");
afterEach(() => { vi.useRealTimers(); __resetHeartbeat(); });

describe("the ephemeral line", () => {
  it("states the budget as a fact when nothing has come in 10 s", () => {
    const s = aSession({ createdAt: "2026-09-25T10:00:00Z", lastOutputAt: "2026-09-25T10:00:00Z", status: "running" });
    expect(ephemeralLine(s, null, T0 + 9_000)).toBe("starting");
    expect(ephemeralLine(s, null, T0 + 10_000)).toBe("no output yet · 10s");
  });
  it("coarsens liveness: live, then quiet under 1m, then minutes", () => {
    expect(livenessWords(T0, T0 + 3_000)).toBe("live · last output 3s ago");
    expect(livenessWords(T0, T0 + 30_000)).toBe("quiet · under 1m");
    expect(livenessWords(T0, T0 + 14 * 60_000)).toBe("quiet 14m");
  });
  it("changes only on the heartbeat, with no timer of its own and no spinner", () => {
    vi.useFakeTimers({ now: T0 + 2_000 });
    const s = aSession({ createdAt: "2026-09-25T09:59:00Z", lastOutputAt: "2026-09-25T10:00:00Z", status: "running" });
    const { container } = render(<EphemeralLine session={s} steps={null} />);
    const before = container.textContent;
    act(() => { vi.advanceTimersByTime(BEAT_MS - 1); });
    expect(container.textContent).toBe(before);
    act(() => { vi.advanceTimersByTime(1); });
    expect(container.textContent).not.toBe(before);
    expect(src).not.toMatch(/setInterval|setTimeout|requestAnimationFrame/);
    expect(container.querySelector("[role='progressbar'], .spinner, progress")).toBeNull();
  });
  it("renders a link only where its fact exists", () => {
    const s = aSession({ worktreeId: "", status: "running" });
    const { container, rerender } = render(<SessionLinks session={s} mine={false} previewUrl={null} />);
    expect(container.querySelectorAll("button, a")).toHaveLength(0);
    rerender(<SessionLinks session={aSession({ worktreeId: "w-1" })} mine previewUrl="https://preview.example.test/s-1" />);
    expect([...container.querySelectorAll("button, a")].map((e) => e.textContent)).toEqual(["open session", "worktree", "preview"]);
    expectNoPrediction(container);
  });
});
```

| Test | Mutant it kills |
|---|---|
| the four fields in mono | a prose summary or a reordered fold (AX §8.3) |
| no field for raw output | "raw output shown" |
| degrades to commit facts | "the adapter is required" (`OD-8`); an error banner; an agent-written commit subject in a fact line |
| nothing from the terminal, predicts nothing | "meaning from PTY bytes"; an "N of M steps" line (`OD-27`) |
| the budget as a fact | "waits forever": no word when nothing arrives |
| coarsens liveness | a seconds counter that re-renders every second |
| only on the heartbeat | "a timer per row"; "spinner per row" (`DESIGN-SYSTEM.md`) |
| a link only where its fact exists | "dead link"; *open session* on another person's agent |

#### Steps

- [ ] **Step 1: read on the sync base** phase 3's `claudecode` adapter (`ToolCalls`, the pin, the
  cache, its fixtures: does the pinned version write a Bash result's exit code as a field?), its
  `getTranscriptSpan` route, phase 3's lane index, phase 2's `sessions.go`, `heartbeat.ts` (how
  `useHeartbeat` schedules its beat, so the fake-timer test advances it correctly), task 48's
  `IssueAgentLine`, task 40's `AgentLine` and the `aSession` fixture. Rename in this part's files
  only; phase 3's adapter not landed means wait (rule H2).
- [ ] **Step 2: the Go tests**; `go test ./control-plane/internal/transcript/... ./control-plane/internal/api/... -race -count=1`
  fails on the missing `Steps` and route.
- [ ] **Step 3: `steps.go`, the route and its catalog row**; the Go tests pass.
- [ ] **Step 4: the desktop tests**; `npm --prefix desktop test -- StepFold SessionLinks` fails
  on the missing modules.
- [ ] **Step 5: `StepFold.tsx`, `SessionLinks.tsx`, `getSessionSteps` in `client.ts`, the
  `IssueAgentLine` edit**, and the `.iv-steps`, `.iv-ephemeral`, `.iv-links` rules in `issue.css`.
- [ ] **Step 6: the acceptance commands.** Commit by path after steps 3 and 5, each message ending
  with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/transcript/... ./control-plane/internal/api/... -race -count=1
# expect: ok; steps_test.go runs 5 tests; the api package gains 2 rows
npm --prefix desktop test -- StepFold SessionLinks issue/IssueAgentLine
# expect: every file passes; StepFold 4 tests, SessionLinks 4; task 48's IssueAgentLine tests pass
#   (its "no timer of its own" test now also covers the two new files through the import)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "setInterval|setTimeout|role=.progressbar|spinner" desktop/src/components/issue/StepFold.tsx desktop/src/components/issue/SessionLinks.tsx | wc -l
# expect: 0
```

Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- That the adapter reads any given harness version: its canary decides (**UNVERIFIED**), and
  Codex, OpenCode and Gemini CLI get the commits tier only (phase 3 3-D7 c, `task-17.md:237`).
- That the pinned transcript format records a shell call's exit code as a field; if it does not,
  every step omits `exit` rather than guess it from the text.
- A step fold for delegated agents (decision 7, gap 2), or a preview link on any session today
  (phase 7, **UNVERIFIED**).
- That *no output yet* means the agent is stuck: it states that nothing has arrived, and when.

**Parity.** Closes **L-185**, **L-186**, **L-188** and **L-190** (05 §1's rows as the scope names
them; task 113 records the proof).

#### Gaps found while writing this part

1. **Not in the skeleton's delta table.** The fold needs phase 3's adapter extended with
   `Steps` (`internal/transcript/claudecode`, P3 task 17) and a control-plane steps read beside
   phase 2's session routes; the scope lists desktop files only. The skeleton's *Deltas on
   sibling code* table should gain a phase 3 row for task 108, and the file map a control-plane row.
2. **Delegated agents have no step source.** A Fleet session's transcript lives in its sandbox,
   not beside the desktop; a fold for delegated agents needs Fleet (or the sandbox) to run the same
   adapter and serve steps, an ask to Phase T. TM-52 also carries no first-output time, so the
   budget for a delegated agent reads from its status only.
3. **Phase 7's per-session preview URL** has no named field on any session record this plan reads.
4. **`Session` has no exit code** (`desktop/src/store/types.ts:134-157`); the commits tier's `exit`
   comes from the control plane's session record, which this read adds to its answer.
