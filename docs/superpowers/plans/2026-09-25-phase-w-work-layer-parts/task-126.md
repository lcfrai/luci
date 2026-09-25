### Task 126: cross-team sharing in the feature's header

W4, stream WD, about 2.5 hours (an estimate, not a measurement). The desktop half of task 117
(Q12: a **project-scoped** read-and-comment grant; per-object sharing stays SKIP, L-86). Depends on
task 125 (`ProjectGrant`, `grantProject`, `revokeProjectGrant`, `projectGrants`, the `grant.*` ops
and `ProjectScope.access`, `task-125.md`). Reads task 31's `ViewHeader` (`task-31.md:93-117`,
SW.13), task 56's `inverseOf` and `useUndoEntry` (`task-56.md:45-77`, SW.11's `--undo-window`),
task 53's `KEYMAP`, `KeyBinding` and `registerVerb` (its scope, `…-scopes/tasks-41-60.md:390-427`;
**no part on this tree**, so UNVERIFIED), task 85's `FeatureView` (scope only, UNVERIFIED) and task
20's `expectNoPrediction` (SW.8). Closes **L-67** (its W4 half, with task 117; its W2 half is tasks
76 and 85).

**What the reader sees** (05 §5.4 W4 *cross-team planning*, §4.1):

- **A team admin of the owning team** sees *Share* in the feature's header actions. It opens a
  picker of the org's other teams; each row says what the grant is, in full: *"Payments can read
  and comment on everything in EP, not only this feature"*. The copy names the whole project
  because the grant is the project's. Below the picker, the teams already granted are listed, each
  with *Revoke*.
- **A member of the owning team who is not an admin** sees the same list, read-only: *"Shared with
  Payments"* or *"Not shared with another team"*. No *Share* control, no *Revoke*.
- **A member of a granted team** sees one line under the title, *"Checkout's feature. You can read
  and comment"*, in `--ink-3`. There is no create, state, relation, *Start* or bulk control, and the
  chords for those verbs do nothing on that project: no toast, no refusal, nothing. A refusal would
  be the Hub's 403, and the desktop does not offer what the Hub refuses.
- **The Brief tab, for a granted reader**, reads *"The brief is team-only until the Hub allows a
  granted read"* while task 117's Hub-contract proposal is open. It never renders a partial brief,
  a title, or a criterion count.
- **Revoke** applies at once (the grant leaves the list in the same frame) with an Undo chip inside
  `--undo-window`. Undo is a **fresh grant**, written as the person, never a restore.

**Decisions made here, so nobody re-derives them:**

1. **Who is an admin is the Hub's word, read from `/v1/me`.** `hubapi.Me.Teams[]` carries the
   caller's `role` per team (`feat/hub` `hubapi/types.go:108-114`, `:208-214`; roles
   `viewer·member·admin` at `:23-25`). *Share* renders only when the caller's role in the feature's
   owning team is `admin`. The Hub checks again (task 117, 403 for a non-admin); the desktop's check
   is there so the control is not offered, not as the guard.
2. **Access is one function, `accessOf(projectId)`, over task 125's replica scope.** It answers
   `"member"` for a project of one of the caller's teams, `"granted"` for a project held with
   `ProjectScope.access === "granted"`, and `"none"` otherwise. Every surface in this task reads it;
   none reads grants directly.
3. **The keyboard path is closed at one choke point, not per surface.** Task 53's dispatcher runs
   every binding through `registerVerb`'s registry. This task adds `writes?: true` to the write rows
   of `KEYMAP` and one gate: `pushWriteGate(allow)` returns a pop, and the dispatcher drops a
   `writes` binding while any pushed gate answers `false`. `FeatureView` pushes a gate for a granted
   project while it is mounted. The palette's commands go through the same registry (task 53's
   scope), so the palette is closed by the same line.
4. **Undo of a revoke is `grant.create` as the person** (task 125's op and task 56's
   `inverseOf`): if the Hub refuses it (the person is no longer an admin), the chip shows task 56's
   refusal line, and the grant stays revoked. Nothing is restored behind the person's back.
5. **Revocation reaches a granted reader through the feed**, not through this surface: task 125's
   `classifyGrant` drops the project on `grant.revoked`. This surface only re-renders; when its
   feature leaves the replica, task 57's *"You no longer have access to EP"* fact replaces the view.

**Files** (the scope names the first three; the access module and the keymap edit are the
mechanism decision 3 needs, and the scope's file list omits them, a gap named at the end):
- `desktop/src/components/plan/FeatureSharing.tsx` — the header control and the lists (new)
- `desktop/src/components/plan/FeatureSharing.test.tsx` — the tests below (new)
- `desktop/src/components/plan/FeatureView.tsx` ⚑ (task 85) — mounts it in `<ViewHeader actions>`,
  the granted line, the Brief tab's line, the write gate (modified)
- `desktop/src/components/plan/plan.css` ⚑ — the sharing popover and the granted line (modified)
- `desktop/src/work/access/projectAccess.ts` — `accessOf`, `useProjectAccess`, `useIsTeamAdmin`,
  `pushWriteGate`, `writesAllowed` (new)
- `desktop/src/work/keys/keymap.ts` ⚑ (task 53) — `writes?: true` on the write rows; the
  dispatcher consults `writesAllowed()` (modified)

#### Interfaces — complete: `desktop/src/work/access/projectAccess.ts`

```ts
// desktop/src/work/access/projectAccess.ts — who may do what on a project, as the desktop shows it.
// The Hub decides (task 117's workAccess); this module only decides what to offer.
import { useWork } from "../replica/useWork";            // SW.16, task 55
import { useMe } from "../../api/useMe";                  // H's /v1/me hook (UNVERIFIED name)
import type { ProjectScope } from "../replica/visibility";

export type ProjectAccess = "member" | "granted" | "none";

/** Pure: the caller's access to one project, from the replica's scope (task 125's ProjectScope). */
export function accessOf(projectId: string, scope: readonly ProjectScope[]): ProjectAccess {
  const s = scope.find((p) => p.projectId === projectId);
  if (!s) return "none";
  return s.access === "granted" ? "granted" : "member";
}

export function useProjectAccess(projectId: string): ProjectAccess {
  return useWork((r) => accessOf(projectId, r.scope()));
}

/** True when the caller's role in teamId is admin (hubapi.Me.Teams[].role, types.go:108-114). */
export function useIsTeamAdmin(teamId: string): boolean {
  const me = useMe();
  return me?.teams.some((t) => t.id === teamId && t.role === "admin") ?? false;
}

// ── the write gate (decision 3) ──
type Gate = () => boolean;
const gates = new Set<Gate>();

/** While pushed, the gate may veto every `writes` binding. Returns the pop. A surface pushes one
 *  in an effect and pops it in the cleanup, so a gate never outlives its surface. */
export function pushWriteGate(allow: Gate): () => void {
  gates.add(allow);
  return () => { gates.delete(allow); };
}

/** The dispatcher's question: may a `writes` binding run now? */
export function writesAllowed(): boolean {
  for (const g of gates) if (!g()) return false;
  return true;
}

/** Test seam: the number of live gates, so a test can prove a pop happened. */
export const liveWriteGates = (): number => gates.size;
```

`useMe` is the H-plan's desktop hook over `GET /v1/me` (**UNVERIFIED name** until H's desktop
tasks merge); if it is named otherwise, this import follows. `r.scope()` is task 57's
`ReplicaVisibility.scope()` (`task-57.md:184-191`), which task 125 widened with `access`.

#### Interfaces — complete: the delta on ⚑ `desktop/src/work/keys/keymap.ts` (task 53)

```ts
// KeyBinding (task 53's scope) gains one optional field:
  /** The verb writes a record. Such a binding is dropped while writesAllowed() is false. */
  writes?: true;

// In KEYMAP, every row whose verb creates, moves, relates, assigns, labels, prioritises, starts,
// delegates or bulk-edits gains `writes: true` (05 §4.3's table). Read verbs (open, move focus,
// search, the `?` sheet, copy a key) do not.

// In the dispatcher, before a matched binding runs (task 53's matchListKey → registry call):
import { writesAllowed } from "../access/projectAccess";
  const b = matchListKey(event, scope);
  if (b === null) return;
  if (b.writes && !writesAllowed()) return;   // nothing happens; the Hub would refuse it (task 126)
  runVerb(b.verb, event);
```

`runVerb` stands for task 53's registry call (the scope names `registerVerb(verb, run)` and no
call-side name, so the merged name is taken at build). The check sits **before** the call and
after the match, so `preventDefault` still runs for a matched chord: a vetoed `Shift+R` never
reaches the terminal or the page, and it never starts anything.

#### Interfaces — complete: `desktop/src/components/plan/FeatureSharing.tsx`

```tsx
// desktop/src/components/plan/FeatureSharing.tsx — the header's Share control (task 126).
import { useState } from "react";
import { Popover } from "../common/Popover";                    // task 44's (UNVERIFIED path)
import { useWork, mutate } from "../../work/replica/useWork";   // SW.16: mutate(op) → MutateHandle
import { useIsTeamAdmin, useProjectAccess } from "../../work/access/projectAccess";
import type { Team } from "../../api/hub";                       // H's desktop Team (UNVERIFIED)

export interface FeatureSharingProps {
  projectId: string; projectKey: string;   // e.g. "EP"
  owningTeamId: string;
  /** The org's teams other than the owner, for the picker (a gap: see the end). */
  otherTeams: readonly Team[];
}

/** The picker row's words: the grant is the project's, so the copy says the whole project. */
export const grantCopy = (team: string, projectKey: string): string =>
  `${team} can read and comment on everything in ${projectKey}, not only this feature`;

export function FeatureSharing(p: FeatureSharingProps): JSX.Element | null {
  const access = useProjectAccess(p.projectId);
  const isAdmin = useIsTeamAdmin(p.owningTeamId);
  const grants = useWork((r) => [...r.roadmap().grants.values()].filter((g) => g.projectId === p.projectId));
  const [open, setOpen] = useState(false);
  if (access !== "member") return null;          // a granted reader sees GrantedLine instead
  const name = (id: string) => p.otherTeams.find((t) => t.id === id)?.name ?? id;
  const granted = new Set(grants.map((g) => g.teamId));

  const list = grants.length === 0
    ? <p className="fs-none">Not shared with another team</p>
    : <ul className="fs-list" aria-label="Shared with">
        {grants.map((g) => (
          <li key={g.teamId} className="fs-row">
            <span>{name(g.teamId)}</span>
            {isAdmin && (
              <button type="button" className="fs-revoke" onClick={() =>
                // task 56's ledger takes the entry; its UndoChip offers Undo inside --undo-window,
                // and Undo sends inverseOf = a fresh grant.create as the person (decision 4)
                mutate({ kind: "grant.revoke", projectId: p.projectId, teamId: g.teamId })
              }>Revoke</button>
            )}
          </li>
        ))}
      </ul>;

  if (!isAdmin) {
    return grants.length === 0 ? list
      : <p className="fs-shared">Shared with {grants.map((g) => name(g.teamId)).join(", ")}</p>;
  }
  return (
    <Popover open={open} onOpenChange={setOpen}
      trigger={<button type="button" className="fs-share">Share</button>}>
      <div className="fs-pop" role="dialog" aria-label={`Share ${p.projectKey}`}>
        <ul className="fs-pick" role="listbox" aria-label="Teams">
          {p.otherTeams.filter((t) => !granted.has(t.id)).map((t) => (
            <li key={t.id} role="option" aria-selected={false} tabIndex={0} className="fs-opt"
                onClick={() => { mutate({ kind: "grant.create", projectId: p.projectId, teamId: t.id }); setOpen(false); }}>
              <span className="fs-opt-name">{t.name}</span>
              <span className="fs-opt-copy">{grantCopy(t.name, p.projectKey)}</span>
            </li>
          ))}
        </ul>
        {list}
      </div>
    </Popover>
  );
}

/** One line under the title for a member of a granted team. No write verb is rendered with it. */
export function GrantedLine({ owningTeamName }: { owningTeamName: string }): JSX.Element {
  return <p className="fs-granted">{owningTeamName}'s feature. You can read and comment</p>;
}

/** The Brief tab's whole body for a granted reader, while task 117's proposal is open. */
export const BRIEF_TEAM_ONLY = "The brief is team-only until the Hub allows a granted read";
```

`mutate(op) → MutateHandle` is SW.16's (`task-56.md:340-357`): it paints in one `transact`, pushes
one ledger entry when `undoable(op, before)` holds, and task 56's `UndoChip` reads the ledger. So
this component writes no undo code: task 125's `RoadmapOp` joins task 56's `WorkOp` union, and its
`inverseOf` for `grant.revoke` is `grant.create` on the same pair. The picker lists only teams
not yet granted, so `grant_exists` (409) is not reachable from the UI; if a race makes it so, task
56's refusal line shows the Hub's message and the replica's grant list is unchanged.

#### Interfaces — complete: the deltas on ⚑ `FeatureView.tsx` (task 85) and ⚑ `plan.css`

```tsx
// desktop/src/components/plan/FeatureView.tsx — task 126's additions, inside task 85's component.
import { useEffect } from "react";
import { FeatureSharing, GrantedLine, BRIEF_TEAM_ONLY } from "./FeatureSharing";
import { pushWriteGate, useProjectAccess } from "../../work/access/projectAccess";

  const access = useProjectAccess(feature.projectId);
  const granted = access === "granted";
  // Decision 3: while a granted project's feature is on screen, no `writes` binding runs.
  useEffect(() => (granted ? pushWriteGate(() => false) : undefined), [granted]);

  <ViewHeader
    title={feature.name}
    route={route}
    actions={granted ? undefined : (
      <>
        <FeatureSharing projectId={feature.projectId} projectKey={project.key}
          owningTeamId={project.teamId} otherTeams={otherTeams} />
        {/* task 85's own actions (Start, New issue, the state menu) follow, members only */}
      </>
    )}
  />
  {granted && <GrantedLine owningTeamName={owningTeam.name} />}

  // The Brief tab (task 85's tab strip): a granted reader gets one line and nothing else.
  {tab === "brief" && (granted
    ? <p className="fv-brief-team-only">{BRIEF_TEAM_ONLY}</p>
    : <BriefPane feature={feature} />)}

  // Task 85's issue list inside the feature: for a granted reader it renders with
  // `readOnly` (no create row, no state menu, no relation picker, no bulk bar, no Start).
  <FeatureIssues feature={feature} readOnly={granted} />
```

`FeatureIssues`, `BriefPane`, `tab`, `route`, `project`, `owningTeam` and `otherTeams` are task
85's names as its scope implies them (**UNVERIFIED**, no part on this tree); `readOnly` is a prop
this task adds to task 85's issue list, and a gap if task 85's list has no single place to hide its
write controls. The gate is pushed in an effect so it pops on unmount and on a change of feature.

```css
/* desktop/src/components/plan/plan.css — task 126 */
.fs-share, .fs-revoke { font: inherit; font-size: var(--t-ui); border-radius: var(--r-ctl); }
.fs-pop { display: grid; gap: var(--s-2); padding: var(--s-3); min-width: 22rem;
  background: var(--surface); border-radius: var(--r-card);
  box-shadow: var(--e-overlay);             /* ring plus layered shadow, never a flat border */ }
.fs-pick { display: grid; gap: var(--s-1); }
.fs-opt { display: grid; gap: 2px; padding: var(--s-2); border-radius: var(--r-ctl); cursor: pointer; }
.fs-opt:focus-visible, .fs-opt:hover { background: var(--hover); }
.fs-opt-name { font-family: var(--sans); font-size: var(--t-ui); line-height: var(--t-ui-lh); color: var(--ink); }
.fs-opt-copy { font-family: var(--sans); font-size: var(--t-meta); line-height: var(--t-meta-lh); color: var(--ink-3); }
.fs-list { display: grid; gap: var(--s-1); }
.fs-row { display: flex; justify-content: space-between; align-items: center; min-height: var(--row-h); }
.fs-none, .fs-shared, .fs-granted, .fv-brief-team-only {
  font-family: var(--sans); font-size: var(--t-meta); line-height: var(--t-meta-lh); color: var(--ink-3); margin: 0; }
```

No transition, no colour but the ink scale: sharing is a fact, not a status, so it takes no hue.
The popover's face follows `docs/DESIGN-SYSTEM.md`'s elevation rule through `--e-overlay`. Token
names verified against `desktop/src/styles/tokens.css` on this tree (`--e-overlay`, `--r-card`,
`--r-ctl`, `--surface`, `--hover`, `--ink`, `--ink-3`, `--sans`) and task 26's part (`--s-*`,
`--t-ui`, `--t-meta`, `--row-h`, `task-26.md:89-109`), which adds the latter set.

#### Tests to write first — `desktop/src/components/plan/FeatureSharing.test.tsx`

Each test renders `FeatureView` (or `FeatureSharing`) over task 125's fake-Hub replica
(`roadmap.test.ts`'s harness) and task 20's Work fixtures: project `EP` of team *Checkout*
(`tm-ck`), a second team *Payments* (`tm-pay`). `me` is set per test through the `useMe` test
provider. Every test ends with `expectNoPrediction(container)` (SW.8).

```tsx
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";     // task 20
import { renderFeature, seedGrant, meAs, keyDown, feedEvent } from "../../test/roadmapHarness";
import { grantCopy, BRIEF_TEAM_ONLY } from "./FeatureSharing";
import { liveWriteGates } from "../../work/access/projectAccess";

describe("task 126: sharing in the feature's header", () => {
  it("a non-admin member sees who it is shared with and no Share control", async () => {
    meAs({ teams: [{ id: "tm-ck", role: "member" }] });
    seedGrant("pj-ep", "tm-pay");
    const { container } = await renderFeature("ft-ep");
    expect(screen.queryByRole("button", { name: "Share" })).toBeNull();     // kills "members grant"
    expect(screen.queryByRole("button", { name: "Revoke" })).toBeNull();
    expect(screen.getByText("Shared with Payments")).toBeTruthy();
    expectNoPrediction(container);
  });

  it("the picker's copy names the whole project", async () => {
    meAs({ teams: [{ id: "tm-ck", role: "admin" }] });
    const { container } = await renderFeature("ft-ep");
    fireEvent.click(screen.getByRole("button", { name: "Share" }));
    const opt = within(screen.getByRole("listbox", { name: "Teams" })).getByRole("option");
    // kills "implies per-feature sharing"
    expect(opt.textContent).toContain(grantCopy("Payments", "EP"));
    expect(opt.textContent).toContain("everything in EP, not only this feature");
    expectNoPrediction(container);
  });

  it("a granted reader has no write control, and the write chords do nothing", async () => {
    meAs({ teams: [{ id: "tm-pay", role: "admin" }] });
    seedGrant("pj-ep", "tm-pay");
    const { container, hub } = await renderFeature("ft-ep");
    for (const name of [/new issue/i, /start/i, /state/i, /relation/i, /share/i])
      expect(screen.queryByRole("button", { name })).toBeNull();
    expect(container.querySelector("[data-bulk-bar]")).toBeNull();
    keyDown({ code: "KeyR", key: "R", shiftKey: true });      // task 61's Start
    keyDown({ code: "KeyC", key: "c" });                      // task 53's create
    // kills "a keyboard path around the missing button"
    expect(hub.writes()).toEqual([]);
    expect(screen.getByText("Checkout's feature. You can read and comment")).toBeTruthy();
    expectNoPrediction(container);
  });

  it("the Brief tab shows the team-only line and nothing of the brief", async () => {
    meAs({ teams: [{ id: "tm-pay", role: "member" }] });
    seedGrant("pj-ep", "tm-pay");
    const { container } = await renderFeature("ft-ep", { tab: "brief" });
    expect(screen.getByText(BRIEF_TEAM_ONLY)).toBeTruthy();
    // kills "brief leaks through the tab": no criterion text, no brief key, no count
    expect(container.textContent).not.toMatch(/bf-|criteri|\d+ of \d+/i);
    expectNoPrediction(container);
  });

  it("a revoke removes the feature from a granted reader's replica on the next feed event", async () => {
    meAs({ teams: [{ id: "tm-pay", role: "member" }] });
    seedGrant("pj-ep", "tm-pay");
    const { container, unmount } = await renderFeature("ft-ep");
    await feedEvent({ kind: "grant.revoked", projectId: "pj-ep", data: { teamId: "tm-pay" } });
    // kills "stale shared view"
    expect(screen.getByText("You no longer have access to EP")).toBeTruthy();   // task 57's fact
    expect(screen.queryByText(/You can read and comment/)).toBeNull();
    unmount();
    expect(liveWriteGates()).toBe(0);    // kills "a gate that outlives its surface"
    expectNoPrediction(container);
  });
});
```

`roadmapHarness` is this task's thin test module over task 125's fake-Hub replica: `renderFeature`
mounts `FeatureView` with the replica and a `useMe` provider; `hub.writes()` lists every write the
fake received; `keyDown` dispatches on the focused list host (`[data-work-list]`, task 53's
target). `grant.revoked`'s kind name is task 125's list and **UNVERIFIED** until task 115 lands.

A sixth test sits in task 53's `keymap.test.ts`: every `KEYMAP` row whose verb id ends in
`.create`, `.start`, `.state`, `.relate`, `.assign`, `.label`, `.priority`, `.delegate` or `.bulk`
carries `writes: true`, and no row whose verb id starts `open.`, `focus.`, `search.` or `keys.`
does. **Kills** "a write row the gate does not see". The verb-id grammar is task 53's scope's
(`VerbId`); if its ids differ, the test's lists follow them.

One more file the tests need: `desktop/src/test/roadmapHarness.tsx` (new), the helpers above.
Tasks 127 and 128 reuse it.

#### Steps

1. Write `roadmapHarness.tsx` and the five tests; run them. They fail: no `FeatureSharing`, no
   `projectAccess`.
2. Write `projectAccess.ts`; add `writes?: true` and the dispatcher line to `keymap.ts`; write the
   sixth test in `keymap.test.ts` and mark the write rows until it passes.
3. Write `FeatureSharing.tsx` and the `FeatureView.tsx` and `plan.css` deltas; the five tests pass.
4. `npm --prefix desktop run typecheck`, then the Acceptance commands.
5. `git add` the new files; commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- src/components/plan/FeatureSharing.test.tsx src/work/keys/keymap.test.ts
# expect: FeatureSharing.test.tsx 5 passed; keymap.test.ts one more passed than before; no failed
npm --prefix desktop run typecheck
# expect: exit 0
```

**Visual acceptance is the owner's eyes** (CLAUDE.md, *Working agreements*): the popover's face,
the granted line's weight and the Brief tab's line are judged by the owner in the running app, not
by this task's report. No screenshot is claimed as evidence.

#### What this task does NOT claim

- That a granted team can read the brief: task 117's Hub-contract proposal decides that, and until
  it does the Brief tab says so.
- Sharing narrower than a project, or with a person: per-object sharing is SKIP (L-86).
- That the gate covers a write path outside `KEYMAP` and the palette (a drag, a context menu): each
  such surface must render nothing for a granted project, as task 85's `readOnly` does for its list.
- That a restarted desktop still holds a granted project (task 125's gap 1).

#### Gaps found while writing this part

1. **The scope's file list** names three files; decision 3 needs `projectAccess.ts` and a
   `keymap.ts` edit, and the tests need `roadmapHarness.tsx`. The skeleton's File map should list
   them under task 126.
2. **The team picker needs the org's teams.** H's `GET /v1/teams` is not known to list teams the
   caller is not in (`hub-contract.md` §7.3, **UNVERIFIED** here); a team admin cannot grant to a
   team they cannot name. H or task 117 must fix a directory read; until then `otherTeams` is the
   teams the caller belongs to other than the owner, which is a narrower picker, and the part says
   so in the UI by offering only those.
3. **Task 53's `KeyBinding.slice`** is `"W1" | "W2" | "W3"`; W4 rows (task 128's zoom chords) need
   `"W4"`. Not this task's edit, named for 128.
4. **Task 85** has no part; `FeatureView`'s tab strip, header actions and issue list are its
   scope's, and `readOnly` is a prop this task asks of it.
