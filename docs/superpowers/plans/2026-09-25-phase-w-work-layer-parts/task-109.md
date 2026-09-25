### Task 109: guidance and the kill switch in settings; our published agent rules

Stream **W3** (agents on the work), about 2.5 hours (an estimate, not a measurement). Depends on
task 100 (SW.19 on the real Hub and its relay rows) and **TM-14** (Phase T's versioned policy store
and the version recorded on agent start). Uses SW.19's `Guidance`, `AgentSwitch` and the methods
`PutGuidance`, `ResolveGuidance`, `GetAgentSwitch`, `SetAgentSwitch` (task 99's scope,
`tasks-81-100.md:452-455`; **their exact names are task 99's**, and step 1 re-reads them), task
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
  author: string;        // u-…
  at: string;            // RFC 3339
}

/** Current version per layer, most local first: person, team, org. A missing layer is absent. */
export interface ResolvedGuidance { layers: Guidance[] }

export interface AgentSwitch {
  orchestrators: boolean; // false: Orchestrators are off for the org
  by: string | null;      // u-… of the admin who last set it
  at: string | null;
  reason: string | null;  // text·H
}

export interface WorkAgentSettingsApi {
  guidanceHistory(scope: GuidanceScope, scopeId: string): Promise<Guidance[]>; // newest first
  resolvedGuidance(teamId: string | null): Promise<ResolvedGuidance>;
  /** 409 `stale_version` when baseVersion is not the current one: nobody overwrites blind. */
  putGuidance(req: { scope: GuidanceScope; scopeId: string; text: string; baseVersion: number }): Promise<Guidance>;
  agentSwitch(orgId: string): Promise<AgentSwitch>;
  setAgentSwitch(orgId: string, req: { orchestrators: boolean; reason: string }): Promise<AgentSwitch>;
  /** Team projects only: Fleet's human pause for each of the caller's own agents (TM-37). */
  pauseMine(projectId: string): Promise<{ paused: string[] }>; // ag-… ids
}
```

#### Interfaces — complete: `desktop/src/components/admin/agentsOff.ts`

```ts
import type { AgentSwitch } from "../../api/work";

const localStamp = (iso: string): string =>
  new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" })
    .format(new Date(iso));

/** The one sentence every blocked path shows (Shift+O, Shift+A, @me:orchestrator, Ask…). */
export function agentsOffLine(
  sw: AgentSwitch,
  nameOf: (u: string) => string,
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
