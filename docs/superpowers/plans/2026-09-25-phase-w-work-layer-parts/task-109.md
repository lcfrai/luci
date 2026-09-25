### Task 109: guidance and the kill switch in settings; our published agent rules

Stream **W3** (agents on the work), about 2.5 hours (an estimate, not a measurement). Depends on
task 100 (SW.19 on the real Hub and its relay rows) and **TM-14** (Phase T's versioned policy store
and the version recorded on agent start). Uses SW.19's `Guidance`, `AgentSwitch` and the methods
`PutGuidance`, `ResolveGuidance`, `GetAgentSwitch`, `SetAgentSwitch` (task 99's scope,
`tasks-81-100.md:452-455`; the names and wire checked against task 100's part,
`task-100.md:176-231` and `:278-298`: `author` and `by` are `User` objects, a stale version is
409 `revision_conflict`, turning Orchestrators off needs a 1-500 character reason), task
100's `/v1/hub/work/*` relay, task 62's Desk frame (SW.17), and Phase T's human `pause`, which
*"is executed by Fleet directly, with no turn"* (TM-37, Phase T plan `:825`, read at `64ab2c4`;
**UNVERIFIED until merge**).

This part makes three things a person can reach and read:

1. **Guidance**, three layers (org → team → person), each a versioned record with its history,
   written only by a person and never marked `text·A` (05 §1 L-183, `05-design.md:350`).
2. **The org switch** that turns Orchestrators off (L-158, L-246, `05-design.md:320,428`), read by
   Fleet before every turn it would enqueue, and shown on every desktop path that would start one.
3. **Our published agent rules**, `docs/work/agent-rules.md`, each rule citing its source section,
   with a test that fails when a cited section does not exist (L-194, `05-design.md:361`).

**Decisions made here, so nobody re-derives them:**

1. **The switch's words.** SW.19's record is `AgentSwitch{orchestrators, by, at, reason}`. This
   part reads `orchestrators: false` as *the switch is on*, meaning Orchestrators are **off**. The
   screen never says "kill switch on"; it says *"Orchestrators are off for this org (set by <admin>,
   <time>)"* and *"Orchestrators are on"*. The word *kill* stays in code and this plan only.
2. **Only an org admin flips it.** The Hub refuses anyone else (task 100's matrix row); the
   desktop hides the control for a non-admin and shows the state as a fact line instead. Hiding is
   courtesy; the Hub's 403 is the rule.
3. **The switch stops the next turn, never a running session.** While it is on, Fleet enqueues no
   Orchestrator turn of any origin (`delegate`, `mention`, `relation`, `handover`, `blocker`,
   `ask`), and the Hub refuses every delegated write with `agents_disabled` (task 100). A worker
   already running keeps running; stopping it is *pause all*, a separate act per person.
4. **Fleet fails closed.** `killswitch.go` reads the switch from the Hub on every enqueue, with
   no cache. If the Hub read fails, the turn is **not** enqueued and the reason is recorded as
   `switch_unknown`. Turns are rare (TM-37's default budget is 30 an hour per Orchestrator), so one
   read each costs little; a cache would let a turn through after an admin turned Orchestrators off.
5. **One gate, one place.** The gate wraps TM-26's turn-queue `Enqueue` (Phase T plan `:337`), so
   every origin passes through it and no origin can bypass it. Tasks 103, 105 and 110 call the
   wrapped queue; none reads the switch itself.
6. **Guidance merges most local first.** The merged view for a person on a team shows person,
   then team, then org, in that order, each block labelled with its layer, version and author. It
   is **concatenation with provenance**, never a rewrite: TM-13's `Render` merges Claude Code
   managed-settings JSON across *org, then project* (Phase T plan `:546-557`), and a free-text
   layer does not fit that merge. Whether guidance joins TM-13's layers or stays a separate record
   is **UNVERIFIED** (task 99's open question to the Phase T owner), and **no design section says
   where guidance sits in the managed block**; this part renders the merged text and its version
   list only, and delivers nothing into a pod.
7. **A guidance edit is a new version.** Saving sends the whole new text; the Hub keeps every
   earlier version (task 99's append-only rule). The history lists each version with author and
   time in Commit Mono; an old version is read-only and can be copied into the editor, never
   restored in place.
8. ***Pause all* is the person's own agents only.** On a team project it calls Fleet's human
   `pause` for each of the caller's own agents (TM-37) through the local control plane; no
   teammate's agent is named in the request. **In local mode no pause verb exists**: a search of
   `control-plane/internal/api/*.go` and `desktop/src/api/client.ts` on 2026-09-25 finds none. So
   in local mode the button is disabled and says *"local sessions have no pause"*, a gap listed
   below, never a stop disguised as a pause.
9. **The rules page is ours.** Each rule is one sentence, then a source line naming a file and a
   section heading that exists: `AGENTIC-UX.md` or the team design (TM-05, read at `64ab2c4`). No
   Linear text is quoted or paraphrased (`OD-11`).

**Files:**
- `desktop/src/components/admin/AgentGuidance.tsx` — the three layers, the editor, the history (new)
- `desktop/src/components/admin/KillSwitch.tsx` — the org switch and its fact line (new)
- `desktop/src/components/admin/agentsOff.ts` — `agentsOffLine(sw)`, the one sentence every
  blocked path shows (new)
- `desktop/src/components/admin/admin.css` — `.ag-*` and `.ks-*` rules (new)
- `desktop/src/components/desk/Desk.tsx` ⚑ (task 62's) — *pause all* on the horizon bar (modified)
- `desktop/src/api/work.ts` ⚑ (task 37's; 83, 125) — the guidance, switch and pause calls (modified)
- `control-plane/internal/fleet/work/killswitch.go` (+ `_test.go`) — the gate on the turn queue
  (new; **path UNVERIFIED**: the skeleton's file map has no Fleet row for W3)
- `docs/work/agent-rules.md` — our published rules (new; file map row, task 109)
- `desktop/src/docs/agentRules.test.ts` — the doc test (new; see gap 3)
- `desktop/src/components/admin/{AgentGuidance,KillSwitch}.test.tsx` — the tests (new)
- Phase T's human-message path (TM-37, `control-plane/internal/fleet/budget`; **UNVERIFIED until
  merge**) ⚑ — one `gate.Allow` call, because *"a human's own message skips the budget and the
  queue"* (Phase T plan `:824-825`) and so would skip a gate that wraps only `Enqueue` (modified)

#### Interfaces — complete: `control-plane/internal/fleet/work/killswitch.go`

```go
// Package work holds Fleet's rules for agents on issues (Phase W, W3: tasks 103, 105, 109-111).
package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// SwitchReader is the one Hub read the gate needs: task 99's GetAgentSwitch, called as Fleet's
// own workload principal (the switch is org metadata, not a delegated write).
type SwitchReader interface {
	GetAgentSwitch(ctx context.Context, orgID string) (hubapi.AgentSwitch, error)
}

// Origin says why a turn would run. W3 adds these six; TM-26's own origins pass the gate too.
type Origin string

const (
	OriginDelegate Origin = "delegate" // task 102: Shift+O
	OriginMention  Origin = "mention"  // task 103: @me:orchestrator
	OriginRelation Origin = "relation" // task 105: a cross-owner blocks or duplicates
	OriginHandover Origin = "handover" // task 105: rule 2's wind-down
	OriginBlocker  Origin = "blocker"  // task 98: blocker.landed
	OriginAsk      Origin = "ask"      // task 110: the ask worker
)

// Turn is the part of TM-26's queued turn the gate reads. Step 1 replaces it with TM-26's type
// and keeps these reads; the gate never reads the turn's text.
type Turn struct {
	OrgID, ProjectID, OwnerID string
	Origin                    Origin
	SubjectID                 string // is-, ic-, th- or rq-; metadata only
}

// Enqueuer is TM-26's turn queue as the gate sees it.
type Enqueuer interface {
	Enqueue(ctx context.Context, t Turn) error
}

// Refusals is where a refused turn is recorded, as a metadata-only audit row (TM-10's store).
type Refusals interface {
	Refused(ctx context.Context, t Turn, code string)
}

var (
	ErrAgentsDisabled = errors.New("agents_disabled") // the code task 100's Hub guard returns
	ErrSwitchUnknown  = errors.New("switch_unknown")  // the Hub could not be read: fail closed
)

// Gate is the one place Fleet asks whether an Orchestrator turn may run (decision 5).
type Gate struct {
	Switch  SwitchReader
	Next    Enqueuer
	Refused Refusals
}

// Allow reads the switch on every call, with no cache (decision 4).
func (g Gate) Allow(ctx context.Context, t Turn) error {
	sw, err := g.Switch.GetAgentSwitch(ctx, t.OrgID)
	if err != nil {
		g.Refused.Refused(ctx, t, ErrSwitchUnknown.Error())
		return fmt.Errorf("%w: %v", ErrSwitchUnknown, err)
	}
	if !sw.Orchestrators {
		g.Refused.Refused(ctx, t, ErrAgentsDisabled.Error())
		return ErrAgentsDisabled
	}
	return nil
}

// Enqueue is TM-26's Enqueue behind the gate. Fleet's wiring hands every caller this, never Next.
func (g Gate) Enqueue(ctx context.Context, t Turn) error {
	if err := g.Allow(ctx, t); err != nil {
		return err
	}
	return g.Next.Enqueue(ctx, t)
}
```

`hubapi.AgentSwitch` is task 99's type; this part assumes it carries `Orchestrators bool`, `By`,
`At` and `Reason` as the scope lists them (`tasks-81-100.md:453`). If task 99 names the field
otherwise, the rename is in this file only.

#### Interfaces — complete: appended to `desktop/src/api/work.ts`

```ts
// W3 settings: guidance and the org switch (SW.19, task 99). Relay paths are task 100's
// /v1/hub/work/* rows; step 1 re-reads them and renames here only.
export type GuidanceScope = "org" | "team" | "person";

export interface Guidance {
  id: string;            // gd-…, task 99's proposed prefix
  scope: GuidanceScope;
  scopeId: string;       // or-…, tm-… or u-…
  text: string;          // always text·H: a person wrote it
  version: number;       // 1, 2, 3 …; never rewritten
  author: User;          // task 37's User {id, displayName, email}; task 100's wire sends a User
  at: string;            // RFC 3339
}

/** Current version per layer, most local first: person, team, org. A missing layer is absent. */
export interface ResolvedGuidance { layers: Guidance[] }

export interface AgentSwitch {
  orchestrators: boolean; // false: Orchestrators are off for the org
  by: User | null;        // the admin who last set it; null when never set
  at: string | null;
  reason: string;         // text·H; "" when never set (task 100's wire)
}

export interface WorkAgentSettingsApi {
  guidanceHistory(scope: GuidanceScope, scopeId: string): Promise<Guidance[]>; // newest first
  resolvedGuidance(teamId: string | null): Promise<ResolvedGuidance>;
  /** 409 `revision_conflict` {headVersion} when baseVersion is stale (task 100): nobody overwrites blind. */
  putGuidance(req: { scope: GuidanceScope; scopeId: string; text: string; baseVersion: number }): Promise<Guidance>;
  agentSwitch(orgId: string): Promise<AgentSwitch>;
  setAgentSwitch(orgId: string, req: { orchestrators: boolean; reason: string }): Promise<AgentSwitch>;
  /** Team projects only: Fleet's human pause for each of the caller's own agents (TM-37). */
  pauseMine(projectId: string): Promise<{ paused: string[] }>; // ag-… ids
}
```

#### Interfaces — complete: `desktop/src/components/admin/agentsOff.ts`

```ts
import type { AgentSwitch, User } from "../../api/work";

const localStamp = (iso: string): string =>
  new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" })
    .format(new Date(iso));

/** The one sentence every blocked path shows (Shift+O, Shift+A, @me:orchestrator, Ask…). */
export function agentsOffLine(
  sw: AgentSwitch,
  nameOf: (u: User) => string = (u) => u.displayName,
  stamp: (iso: string) => string = localStamp,
): string | null {
  if (sw.orchestrators) return null;
  const who = sw.by ? nameOf(sw.by) : "an org admin";
  const when = sw.at ? stamp(sw.at) : "time not recorded";
  return `Orchestrators are off for this org (set by ${who}, ${when})`;
}
```

`desktop/src/lib/time.ts` exports only `formatElapsed` (`:27`) and `formatWaiting` (`:49`) on
2026-09-25, neither a wall-clock stamp, so the stamp is local here and injectable for tests. The
caller prints the sentence with the time in Commit Mono.

#### Interfaces — complete: `desktop/src/components/admin/KillSwitch.tsx`

```tsx
import { useEffect, useState } from "react";
import type { AgentSwitch, User, WorkAgentSettingsApi } from "../../api/work";
import { agentsOffLine } from "./agentsOff";
import "./admin.css";

export interface KillSwitchProps {
  api: Pick<WorkAgentSettingsApi, "agentSwitch" | "setAgentSwitch">;
  orgId: string;
  isOrgAdmin: boolean;      // from the Hub's membership read; courtesy only (decision 2)
  nameOf: (u: User) => string;
}

export function KillSwitch({ api, orgId, isOrgAdmin, nameOf }: KillSwitchProps) {
  const [sw, setSw] = useState<AgentSwitch | null>(null);
  const [reason, setReason] = useState("");
  const [refused, setRefused] = useState<string | null>(null);
  useEffect(() => { void api.agentSwitch(orgId).then(setSw); }, [api, orgId]);
  if (!sw) return <p className="ks-fact">Reading the org switch…</p>;

  const line = agentsOffLine(sw, nameOf) ?? "Orchestrators are on";
  const flip = async () => {
    if (!reason.trim()) return;
    try {
      setSw(await api.setAgentSwitch(orgId, { orchestrators: !sw.orchestrators, reason: reason.trim() }));
      setReason(""); setRefused(null);
    } catch (e) {
      setRefused((e as { code?: string }).code ?? "refused");
    }
  };
  return (
    <section className="ks" aria-labelledby="ks-title">
      <h3 id="ks-title">Orchestrators for this org</h3>
      <p className="ks-fact" role="status">{line}</p>
      {sw.reason !== "" && <p className="ks-reason">{sw.reason}</p>}
      {isOrgAdmin && (
        <div className="ks-act">
          <label htmlFor="ks-why">Reason</label>
          <input id="ks-why" value={reason} onChange={(e) => setReason(e.target.value)} />
          <button type="button" disabled={!reason.trim()} onClick={() => void flip()}>
            {sw.orchestrators ? "Turn Orchestrators off" : "Turn Orchestrators on"}
          </button>
          <p className="ks-note">Stops the next turn. Running agents keep running; each person pauses their own.</p>
        </div>
      )}
      {refused && <p className="ks-refused" role="alert">Not changed: {refused}</p>}
    </section>
  );
}
```

A reason is required both ways, so the fact line always has a *why* beside *who* and *when*. The
button is a toggle between two named acts, never a switch control with an unlabelled state.

#### Interfaces — complete: `desktop/src/components/admin/AgentGuidance.tsx`

```tsx
import { useEffect, useState } from "react";
import type { Guidance, GuidanceScope, ResolvedGuidance, User, WorkAgentSettingsApi } from "../../api/work";
import "./admin.css";

const LAYER_WORD: Record<GuidanceScope, string> = { person: "yours", team: "team", org: "org" };

export interface AgentGuidanceProps {
  api: WorkAgentSettingsApi;
  teamId: string | null;
  /** The layers this person may write: always "person"; "team" for a team admin; "org" for an org admin. */
  writable: { scope: GuidanceScope; scopeId: string }[];
  nameOf: (u: User) => string;
}

/** The merged view, most local first (decision 6), then one editor and one history per layer. */
export function AgentGuidance({ api, teamId, writable, nameOf }: AgentGuidanceProps) {
  const [merged, setMerged] = useState<ResolvedGuidance | null>(null);
  const reload = () => void api.resolvedGuidance(teamId).then(setMerged);
  useEffect(reload, [api, teamId]);
  return (
    <section className="ag" aria-labelledby="ag-title">
      <h3 id="ag-title">Guidance your agents are given</h3>
      <ol className="ag-merged">
        {merged?.layers.map((g) => (
          <li key={g.id} data-layer={g.scope}>
            <p className="ag-meta">{LAYER_WORD[g.scope]} · v{g.version} · {nameOf(g.author)} · {g.at}</p>
            <pre className="ag-text">{g.text}</pre>
          </li>
        ))}
      </ol>
      {writable.map((w) => (
        <LayerEditor key={w.scope} api={api} scope={w.scope} scopeId={w.scopeId} nameOf={nameOf} onSaved={reload} />
      ))}
    </section>
  );
}

function LayerEditor(p: { api: WorkAgentSettingsApi; scope: GuidanceScope; scopeId: string;
  nameOf: (u: User) => string; onSaved: () => void }) {
  const [history, setHistory] = useState<Guidance[]>([]);
  const [draft, setDraft] = useState("");
  const [refused, setRefused] = useState<string | null>(null);
  const load = () => void p.api.guidanceHistory(p.scope, p.scopeId).then((h) => { setHistory(h); setDraft(h[0]?.text ?? ""); });
  useEffect(load, [p.api, p.scope, p.scopeId]);
  const current = history[0]?.version ?? 0;
  const save = async () => {
    try {
      await p.api.putGuidance({ scope: p.scope, scopeId: p.scopeId, text: draft, baseVersion: current });
      setRefused(null); load(); p.onSaved();
    } catch (e) { setRefused((e as { code?: string }).code ?? "refused"); }
  };
  return (
    <div className="ag-layer" data-scope={p.scope}>
      <label htmlFor={`ag-${p.scope}`}>Guidance, {LAYER_WORD[p.scope]}</label>
      <textarea id={`ag-${p.scope}`} value={draft} onChange={(e) => setDraft(e.target.value)} />
      <button type="button" disabled={draft === (history[0]?.text ?? "")} onClick={() => void save()}>
        Save as version {current + 1}
      </button>
      {refused && <p role="alert">Not saved: {refused}</p>}
      <ol className="ag-history" aria-label={`Versions, ${LAYER_WORD[p.scope]}`}>
        {history.map((g) => (
          <li key={g.version}>
            <span className="ag-meta">v{g.version} · {p.nameOf(g.author)} · {g.at}</span>
            <button type="button" onClick={() => setDraft(g.text)}>Copy into editor</button>
          </li>
        ))}
      </ol>
    </div>
  );
}
```

`.ag-meta` and `.ks-fact` set `font-family: var(--mono)` at `var(--t-fact)` with `tabular-nums`
(SW.11); ids, versions and times are facts. Neither component starts a timer.

#### Interfaces — complete: the edit to `desktop/src/components/desk/Desk.tsx` (task 62's)

Task 62's horizon bar gains one control. The props below are **added** to task 62's `DeskProps`;
nothing of task 62's is renamed.

```tsx
// Added to DeskProps (task 62):
  /** Team projects the caller is a member of; empty in local mode. */
  teamProjectIds: string[];
  pauseMine: (projectId: string) => Promise<{ paused: string[] }>;

// Added inside the horizon bar, after task 62's own controls:
<PauseAll teamProjectIds={props.teamProjectIds} pauseMine={props.pauseMine} />

// Added at the foot of Desk.tsx:
function PauseAll({ teamProjectIds, pauseMine }: Pick<DeskProps, "teamProjectIds" | "pauseMine">) {
  const [done, setDone] = useState<string | null>(null);
  if (teamProjectIds.length === 0) {
    return <button type="button" className="desk-pause" disabled title="local sessions have no pause">pause all</button>;
  }
  const run = async () => {
    const results = await Promise.all(teamProjectIds.map((p) => pauseMine(p)));
    const n = results.reduce((a, r) => a + r.paused.length, 0);
    setDone(n === 0 ? "none of your agents was running" : `${n} agents paused, yours only`);
  };
  return (
    <>
      <button type="button" className="desk-pause" onClick={() => void run()}>pause all</button>
      {done && <span className="desk-pause-fact" role="status">{done}</span>}
    </>
  );
}
```

*"3 agents paused, yours only"* is a count of a completed act, with no "of" in it, so no "N of M"
guard can read it as progress; the Desk test runs `expectNoPrediction` over it. The request names only a project; Fleet resolves *the caller's own* agents from the
caller's token (TM-37's human pause), so no teammate id can be sent.

#### Interfaces — complete: `desktop/src/components/admin/admin.css`

```css
.ks, .ag { display: grid; gap: var(--space-2); }
.ks-fact, .ag-meta, .desk-pause-fact {
  font-family: var(--mono); font-size: var(--t-fact); font-variant-numeric: tabular-nums; color: var(--ink-2);
}
.ks-refused { color: var(--danger-ink); }
.ag-text { font-family: var(--sans); white-space: pre-wrap; margin: 0; }
.ag-merged > li + li { border-top: 1px solid var(--line-1); padding-top: var(--space-2); }
.ag-history { font-size: var(--t-fact); }
```

`--ink-3` carries no fact (skeleton, *Faces*). Token names other than SW.11's are task 26's; step 1
renames here if task 26 named them otherwise.

#### Interfaces — complete: `docs/work/agent-rules.md`

Each rule is one line of ours, then `Source:` with a repository-relative file and a heading that
exists in it, in backticks; the doc test reads exactly that shape.

```markdown
# How agents behave in Panacea App

These are the rules every agent in Panacea App follows, and the product enforces them. Each names
where it comes from.

1. **Nobody steers another person's agent.** Your agents answer to you; a teammate's answer to
   them, and the only channel between people's agents is their Orchestrators.
   Source: `.claude/worktrees/team/docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` `## 4. The Orchestrator and its workers (OD-16)`
2. **Agents cannot claim progress.** No agent writes a percentage, a bar or "N of M"; what shows
   is what was observed.
   Source: `docs/AGENTIC-UX.md` `## 4. Principles`
3. **Agents propose evidence and people record it.** A proposal never counts until a person
   presses *Record*, with a reason, as themselves.
   Source: `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` `### 2.1 Agents that are real teammates on the issue`
4. **Agent text is marked.** Every field an agent wrote carries `text·A`; a person's carries `text·H`.
   Source: `.claude/worktrees/team/docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` `## 3. The data model`
5. **Agent text reaches another agent only quoted.** It arrives as tool output in a fixed envelope
   naming its author, never as instructions.
   Source: `.claude/worktrees/team/docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` `### 7.1 The chain OD-16 adds`
6. **No confidence number without its history.** An agent shows a confidence figure only where a
   calibration record exists.
   Source: `docs/AGENTIC-UX.md` `## 10. Anti-patterns`
7. **An org admin can turn Orchestrators off.** It stops every next turn; each person pauses
   their own running agents.
   Source: `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` `### L. Linear's own AI (L-155 to L-178)`
```

Rules 1, 4 and 5 cite the team design at its path **on `feat/team-mode`**; once that branch is
sync-merged the path becomes `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md`
and step 5 rewrites the three `Source:` lines (the doc test fails until it does). Rule 7's source
row is L-158's section; its words are ours.

#### Tests to write first — `control-plane/internal/fleet/work/killswitch_test.go`

```go
package work

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

type fakeSwitch struct {
	on    bool // Orchestrators on
	err   error
	reads int
}

func (f *fakeSwitch) GetAgentSwitch(context.Context, string) (hubapi.AgentSwitch, error) {
	f.reads++
	return hubapi.AgentSwitch{Orchestrators: f.on}, f.err
}

type countQueue struct{ got []Turn }

func (q *countQueue) Enqueue(_ context.Context, t Turn) error { q.got = append(q.got, t); return nil }

type refusals struct{ codes []string }

func (r *refusals) Refused(_ context.Context, _ Turn, code string) { r.codes = append(r.codes, code) }

var allOrigins = []Origin{OriginDelegate, OriginMention, OriginRelation, OriginHandover, OriginBlocker, OriginAsk}

func TestSwitchOffEnqueuesNoTurnForAnyOrigin(t *testing.T) {
	sw, q, r := &fakeSwitch{on: false}, &countQueue{}, &refusals{}
	g := Gate{Switch: sw, Next: q, Refused: r}
	for _, o := range allOrigins {
		if err := g.Enqueue(context.Background(), Turn{OrgID: "or-1", Origin: o}); !errors.Is(err, ErrAgentsDisabled) {
			t.Fatalf("%s: err = %v, want agents_disabled", o, err)
		}
	}
	if len(q.got) != 0 || len(r.codes) != len(allOrigins) {
		t.Fatalf("queued %d, refused %d; want 0 and %d", len(q.got), len(r.codes), len(allOrigins))
	}
}

func TestSwitchOnPassesEveryOrigin(t *testing.T) {
	q := &countQueue{}
	g := Gate{Switch: &fakeSwitch{on: true}, Next: q, Refused: &refusals{}}
	for _, o := range allOrigins {
		if err := g.Enqueue(context.Background(), Turn{OrgID: "or-1", Origin: o}); err != nil {
			t.Fatalf("%s: %v", o, err)
		}
	}
	if len(q.got) != len(allOrigins) {
		t.Fatalf("queued %d, want %d", len(q.got), len(allOrigins))
	}
}

func TestHubUnreadableFailsClosed(t *testing.T) {
	q, r := &countQueue{}, &refusals{}
	g := Gate{Switch: &fakeSwitch{on: true, err: errors.New("dial tcp: refused")}, Next: q, Refused: r}
	err := g.Enqueue(context.Background(), Turn{OrgID: "or-1", Origin: OriginMention})
	if !errors.Is(err, ErrSwitchUnknown) || len(q.got) != 0 || r.codes[0] != "switch_unknown" {
		t.Fatalf("err=%v queued=%d codes=%v", err, len(q.got), r.codes)
	}
}

func TestEveryCallReadsTheSwitchAgain(t *testing.T) {
	sw := &fakeSwitch{on: true}
	g := Gate{Switch: sw, Next: &countQueue{}, Refused: &refusals{}}
	_ = g.Enqueue(context.Background(), Turn{OrgID: "or-1", Origin: OriginAsk})
	sw.on = false // an admin turns Orchestrators off between two turns
	if err := g.Allow(context.Background(), Turn{OrgID: "or-1", Origin: OriginAsk}); !errors.Is(err, ErrAgentsDisabled) {
		t.Fatalf("second call passed after the switch went off: %v", err)
	}
	if sw.reads != 2 {
		t.Fatalf("reads = %d, want 2 (no cache)", sw.reads)
	}
}
```

The human-message path (TM-37) gets one row in Phase T's own budget test, added by this task:
with the switch off, a human's message to their Orchestrator starts no turn and the person sees
`agents_disabled` (**UNVERIFIED until merge**: the test file is TM-37's).

#### Tests to write first — `desktop/src/components/admin/KillSwitch.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { KillSwitch } from "./KillSwitch";
import { agentsOffLine } from "./agentsOff";

import type { AgentSwitch, User } from "../../api/work";

const PRIYA: User = { id: "u-admin", displayName: "Priya", email: "priya@example.test" };
const OFF: AgentSwitch = { orchestrators: false, by: PRIYA, at: "2026-09-25T04:10:00Z", reason: "incident 12" };
const ON: AgentSwitch = { orchestrators: true, by: null, at: null, reason: "" };
const nameOf = (u: User) => u.displayName;
const api = (sw: AgentSwitch, set = vi.fn()) => ({ agentSwitch: vi.fn().mockResolvedValue(sw), setAgentSwitch: set });

describe("KillSwitch", () => {
  it("names who turned Orchestrators off and when", () => {
    expect(agentsOffLine(OFF, nameOf, () => "25 Sep 14:10")).toBe(
      "Orchestrators are off for this org (set by Priya, 25 Sep 14:10)");
    expect(agentsOffLine(ON, nameOf)).toBeNull();
  });
  it("offers no control to a member who is not an org admin", async () => {
    const { container } = render(<KillSwitch api={api(ON)} orgId="or-1" isOrgAdmin={false} nameOf={nameOf} />);
    await screen.findByText("Orchestrators are on");
    expect(screen.queryByRole("button")).toBeNull();
    expectNoPrediction(container);
  });
  it("sends the flip with a reason, and never without one", async () => {
    const set = vi.fn().mockResolvedValue(OFF);
    render(<KillSwitch api={api(ON, set)} orgId="or-1" isOrgAdmin nameOf={nameOf} />);
    const btn = await screen.findByRole("button", { name: "Turn Orchestrators off" });
    expect((btn as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "incident 12" } });
    fireEvent.click(btn);
    await waitFor(() => expect(set).toHaveBeenCalledWith("or-1", { orchestrators: false, reason: "incident 12" }));
    await screen.findByText(/Orchestrators are off for this org \(set by Priya,/);
  });
  it("shows the Hub's refusal and keeps the old state", async () => {
    const set = vi.fn().mockRejectedValue({ code: "forbidden" });
    render(<KillSwitch api={api(ON, set)} orgId="or-1" isOrgAdmin nameOf={nameOf} />);
    fireEvent.change(await screen.findByLabelText("Reason"), { target: { value: "x" } });
    fireEvent.click(screen.getByRole("button", { name: "Turn Orchestrators off" }));
    await screen.findByText("Not changed: forbidden");
    expect(screen.getByRole("status").textContent).toBe("Orchestrators are on");
  });
});
```

#### Tests to write first — `desktop/src/components/admin/AgentGuidance.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { Guidance } from "../../api/work";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { AgentGuidance } from "./AgentGuidance";

const g = (scope: Guidance["scope"], version: number, text: string): Guidance =>
  ({ id: `gd-${scope}`, scope, scopeId: `x-${scope}`, text, version,
     author: { id: "u-d", displayName: "Dylan", email: "dylan@example.test" }, at: "2026-09-25T01:00:00Z" });

function fakeApi() {
  const store: Guidance[] = [g("person", 1, "prefer small commits")];
  return {
    store,
    guidanceHistory: vi.fn(async () => [...store].reverse()),
    resolvedGuidance: vi.fn(async () => ({ layers: [store[store.length - 1], g("team", 3, "team rule"), g("org", 7, "org rule")] })),
    putGuidance: vi.fn(async (r: { text: string; baseVersion: number }) => {
      if (r.baseVersion !== store.length) throw { code: "revision_conflict" };
      const next = g("person", store.length + 1, r.text); store.push(next); return next;
    }),
    agentSwitch: vi.fn(), setAgentSwitch: vi.fn(), pauseMine: vi.fn(),
  };
}

describe("AgentGuidance", () => {
  it("shows the person's layer first, then team, then org, each with its version", async () => {
    const { container } = render(<AgentGuidance api={fakeApi()} teamId="tm-1" writable={[]} nameOf={() => "Dylan"} />);
    await waitFor(() => expect(container.querySelectorAll(".ag-merged > li").length).toBe(3));
    const layers = [...container.querySelectorAll(".ag-merged > li")].map((li) => li.getAttribute("data-layer"));
    expect(layers).toEqual(["person", "team", "org"]);
    expect(container.querySelector(".ag-meta")?.textContent).toMatch(/^yours · v1 · Dylan/);
    expectNoPrediction(container);
  });
  it("saves an edit as a new version and keeps the old one", async () => {
    const api = fakeApi();
    render(<AgentGuidance api={api} teamId="tm-1" writable={[{ scope: "person", scopeId: "u-d" }]} nameOf={() => "Dylan"} />);
    fireEvent.change(await screen.findByLabelText("Guidance, yours"), { target: { value: "prefer tiny commits" } });
    fireEvent.click(screen.getByRole("button", { name: "Save as version 2" }));
    await waitFor(() => expect(api.store.map((x) => x.version)).toEqual([1, 2]));
    expect(api.putGuidance).toHaveBeenCalledWith(expect.objectContaining({ baseVersion: 1 }));
    expect(api.store[0].text).toBe("prefer small commits");
    await screen.findByText(/^v1 · Dylan/);
  });
});
```

#### Tests to write first — appended to task 62's `desktop/src/components/desk/Desk.test.tsx`

```tsx
describe("pause all (task 109)", () => {
  it("sends only project ids, never an agent or person id, and reports the count", async () => {
    const pauseMine = vi.fn().mockResolvedValue({ paused: ["ag-1", "ag-2"] });
    const { container } = renderDesk({ teamProjectIds: ["pj-a"], pauseMine });
    fireEvent.click(screen.getByRole("button", { name: "pause all" }));
    await screen.findByText("2 agents paused, yours only");
    expect(pauseMine.mock.calls).toEqual([["pj-a"]]);
    expectNoPrediction(container);
  });
  it("is disabled in local mode and says why", () => {
    renderDesk({ teamProjectIds: [], pauseMine: vi.fn() });
    const btn = screen.getByRole("button", { name: "pause all" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.title).toBe("local sessions have no pause");
  });
});
```

`renderDesk(extra)` is task 62's render helper with these two props added; if task 62 has none,
step 4 adds one at the top of this block.

#### Tests to write first — `desktop/src/docs/agentRules.test.ts`

```ts
import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const ROOT = resolve(__dirname, "../../..");
const RULES = readFileSync(resolve(ROOT, "docs/work/agent-rules.md"), "utf8");
const SOURCE = /^\s*Source: `([^`]+)` `([^`]+)`\s*$/;

/** Every rule line pairs with one Source line; returns [rule number, file, heading]. */
export function citations(md: string): [number, string, string][] {
  const out: [number, string, string][] = [];
  let rule = 0;
  for (const line of md.split(/\r?\n/)) {
    const n = /^(\d+)\. \*\*/.exec(line);
    if (n) rule = Number(n[1]);
    const s = SOURCE.exec(line);
    if (s) out.push([rule, s[1], s[2]]);
  }
  return out;
}

describe("docs/work/agent-rules.md", () => {
  const cites = citations(RULES);
  it("gives every rule exactly one source", () => {
    const rules = [...RULES.matchAll(/^(\d+)\. \*\*/gm)].map((m) => Number(m[1]));
    expect(cites.map((c) => c[0])).toEqual(rules);
  });
  it.each(cites)("rule %i cites a heading that exists in %s", (_n, file, heading) => {
    const path = resolve(ROOT, file);
    expect(existsSync(path)).toBe(true);
    const headings = readFileSync(path, "utf8").split(/\r?\n/).filter((l) => l.startsWith("#"));
    expect(headings).toContain(heading);
  });
  it("fails on a heading that does not exist (the mutant, run in-process)", () => {
    const bad = "1. **x.** y\n   Source: `docs/AGENTIC-UX.md` `## 99. Nowhere`\n";
    const [[, file, heading]] = citations(bad);
    const headings = readFileSync(resolve(ROOT, file), "utf8").split(/\r?\n/);
    expect(headings).not.toContain(heading);
  });
});
```

`ROOT` is three levels up from `desktop/src/docs/`, the repository root. The rules file is not
product UI, so no `expectNoPrediction` runs on it; rule 2 states the rule in words.

| Test | Mutant it kills |
|---|---|
| switch off, no origin enqueued | one path (`mention`, `relation`, `ask` …) bypasses the gate |
| switch on, every origin passes | the gate refuses everything and nobody notices it is miswired |
| Hub unreadable fails closed | a failed read treated as *on* |
| every call reads again | a cached switch lets a turn through after an admin turns it off |
| who and when | a bare *"agents disabled"* with no author or time |
| no control for a member | any member flips it (the Hub's 403 is task 100's row) |
| flip needs a reason | a switch flipped with no *why* |
| refusal keeps old state | the UI shows the new state before the Hub accepts it |
| person, team, org order | org wins, or the layers re-sorted |
| a new version, the old kept | `putGuidance` overwrites in place, or saves without `baseVersion` |
| pause sends project ids only | pauses the org, or a teammate's agent named in the request (OD-16) |
| pause disabled in local mode | a stop disguised as a pause |
| every rule has one source; each heading exists | a rule without a source, or a source section that moved |

#### Steps

- [ ] **Step 1: read on the sync base** task 99's `hubapi/work_agents.go` (the `AgentSwitch` field
  names, `Guidance`, the method names), task 100's relay rows in `internal/api/work.go`, task 62's
  `Desk.tsx` and `Desk.test.tsx`, task 26's token names, and on `feat/team-mode` TM-26's turn type
  and `Enqueue`, and TM-37's human-message and human-pause paths. Rename in this part's files only.
- [ ] **Step 2: the Go test first.** `go test ./control-plane/internal/fleet/work/ -race -count=1
  -run Switch\|Hub\|Every` fails on the missing `Gate`.
- [ ] **Step 3: `killswitch.go`**, then Fleet's wiring: every constructor that took TM-26's queue
  takes the `Gate` instead, and TM-37's human path calls `gate.Allow` first. A grep in step 6
  shows no caller holds the bare queue.
- [ ] **Step 4: the desktop tests first**, then `work.ts`'s append, `agentsOff.ts`, `KillSwitch.tsx`,
  `AgentGuidance.tsx`, `admin.css` and the `Desk.tsx` edit. Mount the two settings sections where
  task 62 or the app's settings surface puts org and personal settings (gap 1).
- [ ] **Step 5: `docs/work/agent-rules.md`**, then `agentRules.test.ts`. If `feat/team-mode` is
  merged, rewrite rules 1, 4 and 5's `Source:` paths to the merged path.
- [ ] **Step 6: run the acceptance commands.** Commit by path after steps 3, 4 and 5, each message
  ending with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/fleet/work/ -race -count=1 -run 'Switch|Hub|Every' -v
# expect: 4 tests, all PASS
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
npm --prefix desktop test -- admin desk/Desk docs/agentRules
# expect: every file passes; this part adds 3 files and 15 tests
#   (KillSwitch 4, AgentGuidance 2, agentRules 1 + 1 per rule + 1, i.e. 9 with seven rules)
#   plus 2 in Desk.test.tsx
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "\.Next\.Enqueue|queue\.Enqueue" control-plane/internal/fleet --include='*.go' | grep -v _test.go | grep -v fleet/work/killswitch.go
# expect: no output (only the gate calls the bare queue)
```

The counts are this part's plan, not a run. Visual acceptance of the settings sections and the
horizon bar is the owner's eyes (`CLAUDE.md`).

#### What this task does NOT claim

- That KPMG's managed policy leaves the switch reachable on a laptop: the switch is a Hub record
  set through the desktop, and whether a managed desktop can reach that settings route is untested.
- That guidance reaches an agent. This part stores, merges and shows it; delivery into a session
  (TM-13's layers or a managed-block section) is **UNVERIFIED** and undesigned (gap 2).
- That *pause all* pauses anything in local mode: no local pause verb exists (decision 8, gap 4).
- That Fleet's turn queue has no second entry point besides TM-26's `Enqueue` and TM-37's human
  path; step 6's grep checks Fleet's tree at the sync base only.
- That a running worker stops when Orchestrators go off. It does not; that is by design.

**Parity** (`05-design.md` rows). Closes **L-158** (`:320`: org, team and personal guidance; the
org switch; *pause all* on the horizon bar, in team mode), **L-183** (`:350`: records with history,
org → team → person), **L-194** (`:361`: our published rules) and **L-246** (`:428`, as L-158).
L-158's *pause all* half is closed for team projects only; the local-mode half is gap 4.

#### Gaps found while writing this part

1. **No settings surface is in the file map.** The skeleton gives `src/components/admin/*` to W5's
   tasks 169-173 only (`2026-09-25-phase-w-work-layer.md:314`); this task needs the row too, and a
   named mount point for org and personal settings, which no W1 task builds.
2. **Where guidance sits in an agent's context is undesigned.** TM-13 merges managed-settings JSON
   across org and project (Phase T plan `:546-557`); guidance is free text across org, team and
   person. Whether guidance joins TM-13's layers, TM-14's versioned store, or SW.7's managed block
   is a question for the owner and the Phase T owner.
3. **Docs have no test runner.** The scope asks for a doc test *beside* `agent-rules.md`; `docs/`
   has none, so the test lives at `desktop/src/docs/agentRules.test.ts`. The file map needs the row.
4. **Local mode has no pause.** Neither `control-plane/internal/api/*.go` nor
   `desktop/src/api/client.ts` has a pause verb on 2026-09-25. L-158's *pause all* for local
   sessions needs one (phase 2's owner), or the row is closed for team projects only.
5. **Fleet has no W3 row in the file map** (`control-plane/internal/fleet/work/*`), shared by tasks
   103, 105, 109 and 110.
6. **The turn origins are this part's words.** TM-26 owns the origin enum; the six W3 origins in
   `killswitch.go` must be added there, or mapped, when `feat/team-mode` is merged.

