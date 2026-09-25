### Task 56: Route badge, Share line, spend line; the data class at creation

**Slice:** TM1. **Focused time:** 2.5 h. **dependsOn:** 43 (role-to-verb, the *compliance* role
that alone lowers the ratchet), 52 (the team read model and the desktop client, ST.8), P6-22
(phase 6's inference panel state, **UNVERIFIED until merge**: phase 6's plan is still being
written); 51's prototype, owner-approved, through 52.

This task puts three small facts where a person meets them, and makes a fourth a question the
project-create flow always asks:

1. **The route badge** in the project header says where inference goes: `Global` or `AU`
   (`05-design.md` §9.5, OD-19). A project whose data class is *AU data only* says `AU · required`
   and has **no switch**. Otherwise the badge is the user's trigger: on a solo project *"Use AU for
   this project"*; on a team project *"Use AU for everyone on this project"*, a **one-way
   ratchet** the team strip announces, lowered only by a member holding *compliance* (§2.5,
   task 43).
2. **The Share line**: the Hub's Share popover (Hub task 42, **UNVERIFIED until merge**) gains one
   sentence, *"Team agents run in Panacea's cloud. Your teammates' agents will see your agents'
   tasks and files."* Consent is informed at the moment of sharing.
3. **The spend line**, in project settings only, never on the Desk: *workers* and *coordination*
   against the user's quota, in words and figures, **never a bar** (§4.4, §9.5, OD-27).
4. **The data class is chosen when the project is created** (OD-24) and can be changed later. The
   create flow has no default that skips the question.

The badge and the ratchet reuse phase 6's inference panel state (P6-22); they do not add a second
source of truth for the route. The route itself is decided server-side (phase 6's gateway and the
project's class); this task only shows it and sends the user's one allowed change.

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) The badge is phase 6's chip, not a second chip.** Phase 6 task 22 already puts a route chip
in the top bar, after `<LayoutToggle />`, and says why: the top bar has no project name
(`phase-6 …/task-22.md:26`). "The project header" in `05-design.md` §9.5 is therefore that chip's
place. `RouteBadge` replaces `RouteChip` **inside** phase 6's `InferencePanel`; it takes the same
props plus `required`, so the fetch, the popover, and the rule that the chip changes only on the
server's answer (`task-22.md:5`) stay phase 6's. Two chips would be two sources of truth.

**(b) `AU · required` is computed from the class, never from the route.** It shows when
`dataClass.residency === "au-only"`, which is phase 6's `Floor` (`au-only` → `au`,
`phase-6 …/2026-09-25-phase-6-gateway.md:216`). A project that is `AU` because someone raised it
says `AU`, and a project whose class forbids Global says `AU · required`. The popover of a
`required` project offers no action at all.

**(c) The raise has two wordings and two paths.** Solo: *"Use AU for this project"*, through phase
6's `patchInference(projectId, {route: "au"})`. Team (`strip.team === true`, ST.8): *"Use AU for
everyone on this project"*, through task 52's team action `raiseTeamRoute(projectId)`, which the
Hub's team policy stores (task 43) and Fleet's feed announces to every member's strip. A team raise
through phase 6's local PATCH would change one member's file and nobody else's route, which is
exactly the silent split §2.5 forbids. The confirmation sentence names the consequence: *"Every
teammate's agents on this project will use AU from their next request. Moving it back needs your
organisation's compliance role."* Once the route is `au`, **the action is absent**, for everyone.

**(d) Lowering is never offered in this task.** A *compliance* holder lowers a route through task
43's audited Hub route; this surface does not draw a control that every other person would be
refused (phase 6's own rule, `task-22.md:3`). The admin surface for it is task 67's.

**(e) There is no desktop project-create dialog today, so "at creation" means two moments.**
Verified: phase 2 lists projects from a directory and has no create route
(`control-plane/internal/api/catalog.go:30`, `:258`); no worktree's `desktop/src` holds a create
dialog (grepped 2026-09-25). The moments at which a project first becomes a record are:
1. **The first share** (Hub task 42's `unlinked` state), which creates the Hub's project link. The
   popover gains a required `DataClassField`; **Share** is disabled until a class is chosen, and
   the class is sent with the share (task 43 stores it on the team policy).
2. **A local project with no class** (phase 6's *Not classified* state, `task-22.md:28`). The
   popover offers **Choose a data class** and the same field, sent as phase 6's
   `patchInference(projectId, {dataClass})`. This closes phase 6's gap 2 without a second picker.

When phase 4's create flow exists (phase 6 seam S6.8, ask P4-1), it mounts `DataClassField`
unchanged. "Changed later" is the same field, opened from the popover's *Data class* row.

**(f) The spend line lives in the badge's popover, the project's only settings surface.** No
project-settings view exists in the desktop (`desktop/src/components` holds `common`, `develop`,
`lens`, `menu`, `palette`, `rail`, `subrail`, `topbar`, listed 2026-09-25). The popover already
holds phase 6's spend, per dimension; on a team project `SpendLine` adds one line under it,
*"Today: workers US$18.40, coordination US$3.10, of your US$50.00 quota."* It is never on the Desk
and never a bar, a percentage or an "N of M" (OD-27). A figure phase 6 cannot answer is said, not
zeroed: *"coordination not known yet"* (task 37's `null`, `task-37.md:456-459`).

**(g) The Share line is a component, mounted once.** `ShareLine` renders the fixed sentence as
plain text in both the `unlinked` and `linked` states of Hub task 42's popover, above the
**Share** button, so consent is informed before the act and is still visible after it.

**Files:**
- `desktop/src/components/team/RouteBadge.tsx` — the three forms and `routeBadgeLabel` (new)
- `desktop/src/components/team/ShareLine.tsx` — the fixed sentence (new)
- `desktop/src/components/team/SpendLine.tsx` — workers and coordination against the quota (new)
- `desktop/src/components/team/DataClassField.tsx` — the class choice, required, no default (new)
- `desktop/src/components/team/{RouteBadge,SpendLine,DataClassField}.test.tsx` (new)
- `desktop/src/components/team/team-settings.css` — tokens only (new)
- `desktop/src/components/inference/InferencePanel.tsx` — `RouteBadge` in place of `RouteChip`;
  the team wording and path; the class field in the *Not classified* state; `SpendLine` under
  `Spend` on a team project (modify, **phase 6 task 22's file, UNVERIFIED until merge**)
- `desktop/src/components/share/SharePopover.tsx` — `ShareLine` in two states; `DataClassField`
  in `unlinked`; the class passed to the share (modify, **Hub task 42's file, UNVERIFIED until
  merge**)

**Interfaces:**
- Produces: `RouteBadge`, `routeBadgeLabel`, `ShareLine`, `SHARE_LINE`, `SpendLine`,
  `spendLineSentence`, `DataClassField`, `DATA_CLASS_CHOICES`.
- Consumes, **written against these names; where the owning task spells one differently, it wins
  and this part is renamed mechanically**:
  - phase 6 task 22: `ProjectInference`, `DataClass`, `Route`, `InferencePatch`,
    `api.getInference`, `api.patchInference`, `formatUsd`, `ROUTE_LABEL`, `RouteChip`'s props
    (`phase-6 …/task-22.md:55-119`, `:330-405`);
  - task 52 (**not yet written; required of task 52**): `TeamSettings` as the prototype's
    `SAMPLE.settings` (`task-51.md:165-167`), the strip's `team` and `route` fields
    (`task-51.md:80-81`), the selector `selectTeamSettings(state, projectId)` and the action
    `api.raiseTeamRoute(projectId): Promise<TeamSettings>`;
  - task 37: `SplitView.workersCents` and `coordinationCents`, each `null` when not known
    (`task-37.md:433-441`), which task 52 relays as `settings.spend`;
  - task 43 (**not yet written; required of task 43**): the team policy's data class on the share,
    the route raise by any member and the lowering by *compliance* only;
  - Hub task 42: `SharePopover`'s states and `shareProject(projectId, teamId)`
    (`hub …/task-42.md:55-82`), which gains an optional third argument `{ dataClass }`.

#### The contract types (complete; appended to `desktop/src/store/types.ts` by task 52)

This part does not own these types; it states the shape it renders so a reviewer can check both
parts against one block. If task 52's block differs, task 52 wins.

```ts
/** ST.8: one project's team settings, as the prototype's SAMPLE.settings. */
export interface TeamSettings {
  v: 1;
  projectId: ProjectId;
  /** Phase 6's class object; the prototype's bare string is the level alone. */
  dataClass: DataClass | null;
  route: {
    current: Route;
    /** true when dataClass.residency is "au-only": no switch is drawn. */
    required: boolean;
    /** When the team route was raised to AU, RFC 3339; null while Global. */
    raisedAt: string | null;
    /** true only for a holder of the org compliance role (task 43). */
    canLower: boolean;
  };
  spend: {
    /** UTC day, "2026-09-25". */
    day: string;
    /** null: phase 6 cannot answer yet. Never 0 for "unknown" (task 37). */
    workersCents: number | null;
    coordinationCents: number | null;
    /** The viewer's own daily quota on this project; null: none set. */
    quotaCents: number | null;
  };
}
```

`dataClass` here is phase 6's `DataClass` object, not the prototype's string (`task-51.md:165`
shows `"internal"`). The object is what phase 6's panel already reads, so one type serves both
surfaces; this is listed as a gap for task 52.

- [ ] **Step 1: Preflight**

```bash
cd desktop
test -f src/components/inference/InferencePanel.tsx && echo ok || echo "STOP: phase 6 task 22 not merged"
test -f src/components/share/SharePopover.tsx && echo ok || echo "STOP: Hub task 42 not merged"
grep -c "export interface TeamSettings" src/store/types.ts            # expect 1 (task 52 merged)
grep -c "raiseTeamRoute" src/api/team.ts                              # expect at least 1
test -d src/components/team && ls src/components/team                 # task 53's files; none of ours
```

- [ ] **Step 2: Write the failing tests**

Vitest and React Testing Library, in phase 6 task 22's shape (`vi.hoisted` fakes, `resetCockpit`,
`useCockpit.setState`; no `jest-dom`, so a `getBy…` that throws is the presence check,
`phase-6 …/task-22.md:125`).

`RouteBadge.test.tsx`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `an AU-only project renders no switch` | `dataClass.residency: "au-only"`, route `au` → label `AU · required`; open the popover → no button whose name matches `/use au/i` and no `/global/i` control | a switch drawn from the route instead of the class (decision (b)) |
| `the ratchet button is absent once AU is set` | a team project, `route.current: "au"`, residency `any` → label `AU`; popover → no `/use au/i` button | a raise offered on a raised project, whose every click is a no-op the person reads as "not yet AU" |
| `a team project says everyone` | `strip.team: true`, route `global` → the button reads *Use AU for everyone on this project* | the solo wording on a team project, hiding that the act changes teammates' routes |
| `a solo project says this project` | no team settings → *Use AU for this project* | the reverse |
| `the team raise goes through the team action` | team, confirm → `raiseTeamRoute("pj-1")` called once and `patchInference` never | a team raise written to one member's local file (decision (c)) |
| `the badge changes only on the server's answer` | `raiseTeamRoute` rejects with `ApiError` → the label is still `Global` and the error is shown | an optimistic flip that reports AU when the route is Global |
| `no lowering control for anyone` | `canLower: true`, route `au` → no control whose name matches `/global|lower|move back/i` | a lowering drawn here, not on task 67's audited surface (decision (d)) |

`SpendLine.test.tsx`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `words and figures` | 1840, 310, 5000 → *Today: workers US$18.40, coordination US$3.10, of your US$50 quota.* | — |
| `unknown is said, not zeroed` | `coordinationCents: null` → the sentence holds *coordination not known yet* and no `US$0.00` | a `?? 0` that reports free coordination |
| `no bar, no percentage` | the rendered HTML has no `progress`, `meter`, `role="progressbar"` and no `%` | a gauge (OD-27, §9.5) |
| `figures are tabular` | the sentence's element carries the `tabular-nums` class | proportional digits that jitter as spend moves |
| `solo projects render nothing` | no team settings → `container.firstChild === null` | a coordination figure on a project that has no Orchestrator |

`DataClassField.test.tsx`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `no default` | first render → no radio is checked; `onChange` not called | a pre-checked class that lets a person skip the question (OD-24) |
| `a choice reports the class` | pick *Confidential, AU only* → `onChange({level: "confidential", residency: "au-only"})` | a level sent without its residency |
| `the share waits for a class` | `SharePopover`, `unlinked`, teams → **Share** is disabled until a class is picked; then `shareProject(p, tm, {dataClass})` | a share that creates the Hub project unclassified |
| `the share line is shown before the act` | `unlinked` and `linked` each contain `SHARE_LINE` verbatim | consent shown only after sharing |
| `an unclassified local project can choose` | phase 6 panel, `dataClass: null` → **Choose a data class**, pick, save → `patchInference(p, {dataClass})`; the label follows the answer | phase 6's dead end (its gap 2) |

Run them; all seventeen fail (the modules do not exist).

- [ ] **Step 3: Write `RouteBadge.tsx` and `ShareLine.tsx`**

```tsx
// desktop/src/components/team/RouteBadge.tsx — task 56. Phase 6's chip, with the third form.
// Neutral, like RouteChip: the accent is for the focus ring only (docs/DESIGN-SYSTEM.md).
import type { JSX } from "react";
import type { DataClass, Route } from "../../store/types";
import { ROUTE_LABEL } from "../inference/format";

/** Decision (b): "AU · required" comes from the class, never from the route. */
export function routeBadgeLabel(route: Route | null, dataClass: DataClass | null): string {
  if (route === null || dataClass === null) return "Not classified";
  if (dataClass.residency === "au-only") return "AU · required";
  return ROUTE_LABEL[route];
}

export function RouteBadge(props: {
  projectName: string;
  route: Route | null;
  dataClass: DataClass | null;
  expanded: boolean;
  onOpen: (anchor: DOMRect) => void;
}): JSX.Element {
  const { projectName, route, dataClass, expanded, onOpen } = props;
  const label = routeBadgeLabel(route, dataClass);
  return (
    <button
      type="button"
      className="top-btn inf-chip"
      data-route={route ?? "none"}
      data-required={dataClass?.residency === "au-only" ? "true" : "false"}
      aria-label={`Inference for ${projectName}: ${label}`}
      aria-haspopup="dialog"
      aria-expanded={expanded}
      onClick={(e) => onOpen(e.currentTarget.getBoundingClientRect())}
    >
      {label}
    </button>
  );
}

/** The one action's wording (decision (c)); null when no action may be drawn. */
export function raiseLabel(route: Route | null, dataClass: DataClass | null, team: boolean): string | null {
  if (route !== "global" || dataClass === null || dataClass.residency === "au-only") return null;
  return team ? "Use AU for everyone on this project" : "Use AU for this project";
}

export const RAISE_CONFIRM_TEAM =
  "Every teammate's agents on this project will use AU from their next request. " +
  "Moving it back needs your organisation's compliance role.";
export const RAISE_CONFIRM_SOLO =
  "Keep this project's inference in Australia? Moving it back needs your organisation's compliance role.";
```

`RAISE_CONFIRM_SOLO` is phase 6's sentence verbatim (`task-22.md:30`), so a solo project reads as
it did before this task.

```tsx
// desktop/src/components/team/ShareLine.tsx — task 56, decision (g). Plain text only.
import type { JSX } from "react";

export const SHARE_LINE =
  "Team agents run in Panacea's cloud. Your teammates' agents will see your agents' tasks and files.";

export function ShareLine(): JSX.Element {
  return <p className="share-line">{SHARE_LINE}</p>;
}
```

- [ ] **Step 4: Write `SpendLine.tsx` and `DataClassField.tsx`**

```tsx
// desktop/src/components/team/SpendLine.tsx — task 56, decision (f). Words and figures only.
import type { JSX } from "react";
import type { TeamSettings } from "../../store/types";
import { formatUsd } from "../inference/format";

const usd = (cents: number): string => formatUsd(cents / 100);

/** "Today: workers US$18.40, coordination US$3.10, of your US$50.00 quota." */
export function spendLineSentence(s: TeamSettings["spend"]): string {
  const w = s.workersCents === null ? "workers not known yet" : `workers ${usd(s.workersCents)}`;
  const c = s.coordinationCents === null ? "coordination not known yet" : `coordination ${usd(s.coordinationCents)}`;
  const q = s.quotaCents === null ? "no daily quota set" : `of your ${usd(s.quotaCents)} quota`;
  return `Today: ${w}, ${c}, ${q}.`;
}

export function SpendLine(props: { settings: TeamSettings | null }): JSX.Element | null {
  if (props.settings === null) return null; // solo project: no Orchestrator, no split
  return <p className="team-spend tabular-nums">{spendLineSentence(props.settings.spend)}</p>;
}
```

`formatUsd(18.4)` gives `US$18.40` and `formatUsd(50)` gives `US$50` (phase 6's whole-number rule,
`task-22.md:338-341`), so the test expects `US$50`; decision (f)'s `US$50.00` is illustrative.
Cents are task 37's unit, dollars phase 6's, and this division is the one place the two meet.

```tsx
// desktop/src/components/team/DataClassField.tsx — task 56, decision (e). No default (OD-24).
import { useId, useState } from "react";
import type { JSX } from "react";
import type { DataClass } from "../../store/types";

/** Phase 6 task 01's proposed levels (owner question Q1): one table, never switched on. */
export const DATA_CLASS_LEVELS = ["public", "internal", "confidential", "restricted"] as const;
const LEVEL_LABEL: Record<(typeof DATA_CLASS_LEVELS)[number], string> = {
  public: "Public", internal: "Internal", confidential: "Confidential", restricted: "Restricted",
};

export function DataClassField(props: {
  value: DataClass | null;
  onChange: (c: DataClass) => void;
}): JSX.Element {
  const id = useId();
  const [level, setLevel] = useState<string | null>(props.value?.level ?? null);
  const [res, setRes] = useState<DataClass["residency"] | null>(props.value?.residency ?? null);
  const pick = (l: string | null, r: DataClass["residency"] | null): void => {
    setLevel(l);
    setRes(r);
    if (l !== null && r !== null) props.onChange({ level: l, residency: r });
  };
  return (
    <fieldset className="dc-field">
      <legend>Data class</legend>
      {DATA_CLASS_LEVELS.map((l) => (
        <label key={l}>
          <input type="radio" name={`${id}-level`} checked={level === l} onChange={() => pick(l, res)} />
          {LEVEL_LABEL[l]}
        </label>
      ))}
      <label>
        <input type="radio" name={`${id}-res`} checked={res === "any"} onChange={() => pick(level, "any")} />
        May use Global inference
      </label>
      <label>
        <input type="radio" name={`${id}-res`} checked={res === "au-only"} onChange={() => pick(level, "au-only")} />
        Australia only
      </label>
    </fieldset>
  );
}
```

Both questions start unanswered, and `onChange` fires only when both are answered, so neither a
level nor a residency can be defaulted into the record. The test `a choice reports the class`
picks the level and then *Australia only*.

- [ ] **Step 5: Wire the two sibling surfaces (anchored hunks)**

In phase 6's `InferencePanel.tsx` (`phase-6 …/task-22.md:409-513`), four hunks:

```tsx
// 1. Imports and the team read, beside the file's other useCockpit reads.
import { RouteBadge, raiseLabel, RAISE_CONFIRM_SOLO, RAISE_CONFIRM_TEAM } from "../team/RouteBadge";
import { SpendLine } from "../team/SpendLine";
import { DataClassField } from "../team/DataClassField";
import { selectTeamSettings } from "../../store/selectors"; // task 52
const team = useCockpit((s) => (projectId === null ? null : selectTeamSettings(s, projectId)));

// 2. The chip: <RouteChip … /> becomes
<RouteBadge projectName={projectName} route={eff} dataClass={inf.dataClass}
  expanded={anchor !== null} onOpen={open} />

// 3. The raise: the action's label is raiseLabel(eff, inf.dataClass, team !== null); when it
//    is null nothing is drawn. The confirm sentence is RAISE_CONFIRM_TEAM or _SOLO. raise():
const next = team !== null
  ? (await api.raiseTeamRoute(projectId), await api.getInference(projectId)) // server's answer
  : await api.patchInference(projectId, { route: "au" });
setInf(next);

// 4. Under <Spend inf={inf} />, and the class choice in the eff === null branch:
<SpendLine settings={team} />
{eff === null && <ClassChooser onSave={(c) => api.patchInference(projectId, { dataClass: c }).then(setInf)} />}
```

`ClassChooser` is a six-line local component in the same file: a **Choose a data class** button
that expands in place into `DataClassField` and a **Save** button enabled once a class is picked,
with `setError` on a refusal, as the raise does. After a team raise the panel re-reads phase 6's
route rather than trusting task 52's answer, so the badge still changes only on the route source's
own word (decision (a)).

In Hub task 42's `SharePopover.tsx` (`hub …/task-42.md:71-82`), three hunks: `<ShareLine />`
above **Share** in the `unlinked` layout and under the team name in `linked`; `<DataClassField>`
above **Share** in `unlinked`, with **Share** `disabled` until it has reported a class; and the
share call `shareProject(projectId, teamId, { dataClass })`, the optional third argument that
task 43's share route stores on the team policy. Hub task 42's twelve tests still pass: none of
them asserts that **Share** is enabled with no class; the two that press it gain one class pick
first, which is the only edit to its test file (**UNVERIFIED until merge**: the test file is
planned, not built).

`team-settings.css` holds `.team-spend`, `.share-line`, `.dc-field` in tokens only: no hex, no
`transition: all`; the `fieldset` is a 1px ring (`--e-btn`), not a flat border.

- [ ] **Step 6: Acceptance**

```bash
cd desktop
npx vitest run src/components/team/RouteBadge.test.tsx src/components/team/SpendLine.test.tsx \
  src/components/team/DataClassField.test.tsx 2>&1 | tail -2      # 17 passed
npx vitest run src/components/inference src/components/share 2>&1 | tail -2   # phase 6's 11 and Hub 42's 12 still pass
npm test 2>&1 | tail -3                                                       # all passed
npm run typecheck                                                             # exit 0
npx vitest run src/styles/designsystem.test.ts 2>&1 | tail -2                 # passes over team-settings.css
grep -rn "progress\|role=\"progressbar\"\|%" src/components/team/SpendLine.tsx   # no output
```

Visual acceptance is the owner's eyes against the approved prototype's state `9`
(`task-51.md:189`), in `npm run tauri dev`; the implementer records what was seen as an
observation, not as evidence.

- [ ] **Step 7: Commit** by pathspec: the seven new files and the two modified ones.

**What this task does NOT claim:**
- That the team route raise reaches every member within any stated time. Task 52's action and
  task 43's record carry it; the gatekeeper sees it within phase 6's 30-second route cache
  (`phase-6 …/2026-09-25-phase-6-gateway.md:244`). Task 57 criterion (6) measures it.
- That a lowering works. It is task 43's route and task 67's surface.
- That the data-class levels are KPMG's. They are phase 6's proposed list (owner question Q1).
- That there is a create dialog. Decision (e) names the two real moments; phase 4 owns the dialog.
- That the spend figures are exact. They are task 37's split, with its own stated limits
  (`task-37.md:660-665`).
- Visual acceptance, which is the owner's (`CLAUDE.md`, working agreements).

**Gaps found in the skeleton and in sibling parts:**
1. Task 52 must export `TeamSettings`, `selectTeamSettings` and `api.raiseTeamRoute`, and carry
   phase 6's `DataClass` object, not the prototype's bare level string (`task-51.md:165`).
2. Hub task 42's `shareProject` needs an optional `{ dataClass }` argument, and task 43's share
   route must store it; neither the skeleton nor task 43's scope says the class arrives *with the
   share*.
3. The skeleton places the badge in "the project header"; none exists. Decision (a) uses phase 6's
   top-bar chip.
