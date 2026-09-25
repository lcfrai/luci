### Task 67: The quotas admin view

**Slice:** TM2. **Focused time:** 2.5 h. **dependsOn:** 56 (the spend line and its popover, the
`usd` formatting it uses), 66 (`QuotaView`, the two local routes, the `above_org_ceiling`
refusal); 51 (the prototype) and 52 (the team client, ST.8's `types.ts` block) through them.
**Seam:** ST.8, extended by one view.

A team admin needs one place to answer three questions: *how much may each person and the project
spend a day, how much went on today, and where did coordination go?* A member needs the same
answers about themselves, and no way to change them. Task 66 serves all of it as one `QuotaView`
(`task-66.md`, *api/teamquotas.go*); this task renders it in words and figures and lets a team
admin change the two quotas, within the org's ceiling, through task 66's `PUT`.

It follows OD-27 to the letter: **no percentage, bar, meter, projection or "N of M"** anywhere in
it. A quota is a sentence (*"Each member may spend up to US$25.00 a day on this project."*); a
day's spend is a sentence (*"Today, Alice: workers US$18.40, coordination US$3.10."*); a reached
quota is a fact (*"Alice has reached today's quota."*), said once, in words. Figures are
`tabular-nums`; times are in the monospace face (OD-28).

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) The view opens from the spend line, in phase 2's `Popover`.** There is no project-settings
view in the desktop; task 56 made the route badge's popover the project's only settings surface
and put the spend line in it (`task-56.md:71-77`). On a team project the spend line gains one link,
*"Team quotas"*, that opens `QuotaAdmin` in phase 2's `Popover` (`common/Popover.tsx:10-15`,
`anchor`, `onClose`, `children`), anchored on the link. No new dialog primitive is added. On a
solo project the link is absent.

**(b) The prototype has no quotas state yet, so this task adds one before it builds.** Task 51's
switcher has states `1`–`9` (`task-51.md:177-190`); state `9` shows the spend line, not an admin
view. `CLAUDE.md` requires the owner to review a prototype before its implementation, so step 1
adds state `0`, *"Team quotas"*, to `prototype/team-mode.html` in task 51's language and sample
shape, and **the build steps wait for the owner's review**. If the owner changes the wording, the
constants in `quotaWords.ts` follow the owner, and this part's sentences are the draft.

**(c) The sentences are pure functions in `quotaWords.ts`.** The component only arranges them. The
tests read the sentences, not the markup, so wording changes stay one-file edits, and a sentence
that could carry a ratio (a slash, a percent sign, the word *of* between two figures) is caught by
one test over every builder.

**(d) Tokens are shown only when the gateway knows them.** Task 66 posts `null` tokens until P6-09
reads them (`task-66.md` gap 3). The scope's example is *"coordination with Alice: 41k tokens
today"*; this view says *"coordination with Alice: US$0.41 today"* while tokens are `null`, and
appends *", 41k tokens"* when they are known. It never prints `0 tokens` for `null`.

**(e) The edit is two fields and one button, for a team admin only.** `canEdit` comes from task
66's view, which takes it from the Hub's role; the Hub still decides. A non-admin sees the same
sentences and **no input, no button and no disabled control**: a control nobody may use is noise
(phase 6's own rule, `task-56.md:53-55`). Inputs take US dollars with cents (`25.00`), each with its
ceiling said beside it (*"up to US$50.00, your organisation's ceiling"*). The client parses and
sends; it does not re-check the ceiling, so the Hub's refusal is the only one and its words are
shown as they come: *"Above your organisation's ceiling of US$50.00."*, from `ceilingCents`.

**(f) A stale revision reloads, it does not merge.** A 409 `conflict` means another admin saved
first. The view says *"Another admin changed these quotas. The current values are shown."*,
re-reads, and leaves the user's typed values in the fields so nothing typed is lost, marked as not
saved.

**(g) Freshness is said, not animated.** The view reads once when opened and once after a save;
it does not poll. `asOf` is shown as *"As of 14:20 UTC"* in the monospace face, and *"No usage
posted today yet."* when it is `null`. Task 66's figures can be ten minutes behind the gateway;
saying the time is the honest answer, and a spinner per row is forbidden anyway.

**Files:**
- `prototype/team-mode.html` — state `0`, *"Team quotas"*, and `SAMPLE.quotas` (modify, task 51's
  file; owner-reviewed before step 3)
- `desktop/src/store/types.ts` — the `QuotaView` block, after task 52's team block (modify)
- `desktop/src/api/team.ts` — `quotas(projectId)` and `putQuotas(projectId, body)` on
  `TeamClient` (modify, task 52's file, two methods)
- `desktop/src/components/team/quotaWords.ts` — every sentence, pure (new)
- `desktop/src/components/team/QuotaAdmin.tsx` — the view and the admin's edit (new)
- `desktop/src/components/team/SpendLine.tsx` — the *"Team quotas"* link on a team project
  (modify, task 56's file, one prop and one element)
- `desktop/src/components/team/team-settings.css` — the `.quota-*` rules, tokens only (modify,
  task 56's file)
- `desktop/src/components/team/{quotaWords,QuotaAdmin}.test.ts(x)` (new)

**Interfaces:**
- Produces: `QuotaAdmin` (props `{ projectId, client, onClose }`), mounted by `SpendLine`;
  `quotaWords`' builders; `TeamClient.quotas` and `.putQuotas`.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 66: `QuotaView`, `QuotaCeiling`, `ProjectQuota`, `MemberQuota`, `CounterpartSpend`,
    `PutQuotasRequest`, `GET`/`PUT /v1/projects/{projectId}/team/quotas`, `details.reason =
    "above_org_ceiling"` with `details.ceilingCents` (`task-66.md`, *api/teamquotas.go* and the
    route-75 hunk);
  - task 52: `createTeamClient`, its `send` through `createTransport` (`task-52.md:945-980`),
    `ProjectId` (`store/types.ts`);
  - task 56: `SpendLine(props: { settings })` and its `usd(cents)` helper over phase 6's
    `formatUsd` (`task-56.md:279-293`; `formatUsd` is phase 6 task 22's, **UNVERIFIED until
    merge**);
  - phase 2: `Popover` (`desktop/src/components/common/Popover.tsx:10-15`), `ApiError` with
    `status` and `details` (`desktop/src/api/client.ts:57-69`), `--numeric`
    (`desktop/src/styles/tokens.css:156`) and `--mono` (`:152`).

#### `store/types.ts`, the `QuotaView` block (complete)

```ts
/** ST.8, task 66: GET /v1/projects/{projectId}/team/quotas. US cents only.
 *  No percentage, remainder or "used" field exists, on purpose (OD-27). */
export interface QuotaView {
  v: 1;
  projectId: ProjectId;
  team: boolean;
  day: string;            // UTC, "2026-09-25"
  asOf: string | null;    // RFC 3339; null: no usage posted today
  currency: "USD";
  canEdit: boolean;
  revision: number;
  ceiling: { memberDailyCents: number; projectDailyCents: number };
  memberDaily: { dailyCents: number; enforcedCents: number };
  project: { dailyCents: number; enforcedCents: number; spentCents: number | null; reached: boolean };
  members: MemberQuota[];
}

export interface MemberQuota {
  userId: string;
  name: string;
  enforcedCents: number;
  workersCents: number | null;
  coordinationCents: number | null;
  outsideFleetCents: number | null;
  byCounterpart: CounterpartSpend[];
  reached: boolean;
}

/** userId "" is coordination with no single teammate. */
export interface CounterpartSpend {
  userId: string;
  name: string;
  costCents: number;
  inputTokens: number | null;
  outputTokens: number | null;
}

export interface PutQuotasRequest {
  revision: number;
  memberDailyCents: number;
  projectDailyCents: number;
}
```

#### `api/team.ts`, the two methods

```ts
  /** GET …/team/quotas. A solo project answers { team: false }. */
  quotas(projectId: ProjectId): Promise<QuotaView>;
  /** PUT …/team/quotas. Rejects with ApiError: 422 (details.reason "above_org_ceiling",
   *  details.ceilingCents), 409 conflict (stale revision), 403 (not a team admin). */
  putQuotas(projectId: ProjectId, body: PutQuotasRequest): Promise<QuotaView>;
```

Both use task 52's `send` unchanged, with `encodeURIComponent(projectId)` in the path. `ApiError`'s
`details` is typed `Record<string, string>` (`client.ts:62`), but the Hub sends `ceilingCents` as
a JSON number; `quotaWords` reads it with `Number(details.ceilingCents)` and treats `NaN` as *"not
said"*, so the refusal still shows without a figure.

#### `quotaWords.ts` (complete)

```ts
import type { CounterpartSpend, MemberQuota, QuotaView } from "../../store/types";
import { formatUsd } from "../inference/format";

export const usd = (cents: number): string => formatUsd(cents / 100);

/** 41k, 1.2M: tokens are said compactly; exact figures are the gateway's. */
export function compactTokens(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${Math.round(n / 1000)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

export const QUOTAS_TITLE = "Team quotas";
export const NO_POST_YET = "No usage posted today yet.";
export const STALE_REVISION = "Another admin changed these quotas. The current values are shown.";
export const NOT_SAVED = "Not saved.";
export const SAVED = "Saved.";

export function asOfLine(asOf: string | null): string {
  if (asOf === null) return NO_POST_YET;
  const d = new Date(asOf);
  const hh = String(d.getUTCHours()).padStart(2, "0");
  const mm = String(d.getUTCMinutes()).padStart(2, "0");
  return `As of ${hh}:${mm} UTC`;
}

export function memberQuotaLine(v: QuotaView): string {
  return `Each member may spend up to ${usd(v.memberDaily.enforcedCents)} a day on this project.`;
}

export function projectQuotaLine(v: QuotaView): string {
  return `The project may spend up to ${usd(v.project.enforcedCents)} a day.`;
}

/** Said only when the org's ceiling is what binds: the admin set more. */
export function ceilingBindsLine(kind: "member" | "project", setCents: number, ceilingCents: number): string | null {
  if (setCents <= ceilingCents) return null;
  const who = kind === "member" ? "each member" : "the project";
  return `Set to ${usd(setCents)} for ${who}; your organisation's ceiling of ${usd(ceilingCents)} applies.`;
}

export function ceilingHint(ceilingCents: number): string {
  return `up to ${usd(ceilingCents)}, your organisation's ceiling`;
}

function fig(cents: number | null, what: string): string {
  return cents === null ? `${what} not known yet` : `${what} ${usd(cents)}`;
}

export function memberTodayLine(m: MemberQuota): string {
  const parts = [fig(m.workersCents, "workers"), fig(m.coordinationCents, "coordination")];
  if (m.outsideFleetCents !== null && m.outsideFleetCents > 0) {
    parts.push(`outside the cloud ${usd(m.outsideFleetCents)}`);
  }
  return `Today, ${m.name}: ${parts.join(", ")}.`;
}

export function counterpartLine(c: CounterpartSpend): string {
  const who = c.userId === "" ? "coordination with no single teammate" : `coordination with ${c.name}`;
  const tokens = c.inputTokens !== null && c.outputTokens !== null
    ? `, ${compactTokens(c.inputTokens + c.outputTokens)} tokens` : "";
  return `${who}: ${usd(c.costCents)} today${tokens}`;
}

export function reachedLine(m: MemberQuota): string | null {
  return m.reached ? `${m.name} has reached today's quota.` : null;
}

export function projectTodayLine(v: QuotaView): string {
  const p = v.project;
  if (p.spentCents === null) return NO_POST_YET;
  return p.reached ? `The project has reached today's quota.` : `The project has spent ${usd(p.spentCents)} today.`;
}

export function refusalLine(status: number, details: Record<string, string>): string {
  if (status === 422 && details.reason === "above_org_ceiling") {
    const c = Number(details.ceilingCents);
    return Number.isNaN(c) ? "Above your organisation's ceiling." : `Above your organisation's ceiling of ${usd(c)}.`;
  }
  if (status === 409) return STALE_REVISION;
  if (status === 403) return "Only a team admin can change these quotas.";
  return "The quotas could not be saved. Try again.";
}
```

