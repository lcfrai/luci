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
- Produces: `QuotaAdmin` (props `{ projectId, client }`), mounted by `SpendLine` inside phase 2's
  `Popover`, which owns closing;
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
  if (status === 409) return `${STALE_REVISION} ${NOT_SAVED}`;
  if (status === 403) return "Only a team admin can change these quotas.";
  return "The quotas could not be saved. Try again.";
}

/** Dollars typed as "25" or "25.00" → cents; anything else → null (the field says so). */
export function parseUsd(text: string): number | null {
  const m = /^\s*(\d{1,6})(?:\.(\d{1,2}))?\s*$/.exec(text);
  if (m === null) return null;
  const cents = Number(m[1]) * 100 + Number((m[2] ?? "0").padEnd(2, "0"));
  return cents > 0 ? cents : null;
}
```

`parseUsd` refuses `0`, a negative sign and a third decimal, because each would reach the Hub as a
value it refuses or a value the admin did not mean; its refusal is the field's own sentence,
*"Enter an amount in US dollars, like 25.00."*, and **Save** stays enabled so the Hub's word is the
last one. Six digits of dollars is above the Hub's 10,000,000-cent bound, so the Hub still refuses
the absurd.

#### `QuotaAdmin.tsx` (complete)

```tsx
import { useEffect, useState } from "react";
import type { JSX } from "react";
import type { ProjectId, QuotaView } from "../../store/types";
import type { TeamClient } from "../../api/team";
import { ApiError } from "../../api/client";
import * as W from "./quotaWords";

type Save = { state: "idle" } | { state: "saving" } | { state: "saved" } | { state: "refused"; line: string };

export function QuotaAdmin(props: { projectId: ProjectId; client: TeamClient }): JSX.Element {
  const { projectId, client } = props;
  const [view, setView] = useState<QuotaView | null>(null);
  const [failed, setFailed] = useState(false);
  const [member, setMember] = useState("");
  const [project, setProject] = useState("");
  const [save, setSave] = useState<Save>({ state: "idle" });

  const load = async (keepTyped: boolean): Promise<void> => {
    try {
      const v = await client.quotas(projectId);
      setView(v);
      setFailed(false);
      if (!keepTyped) {
        setMember((v.memberDaily.dailyCents / 100).toFixed(2));
        setProject((v.project.dailyCents / 100).toFixed(2));
      }
    } catch {
      setFailed(true);
    }
  };
  useEffect(() => { void load(false); }, [projectId]); // eslint-disable-line react-hooks/exhaustive-deps

  if (failed && view === null) return <section className="quota-admin"><p>The quotas could not be read.</p></section>;
  if (view === null || !view.team) return <section className="quota-admin" aria-busy={view === null} />;

  const onSave = async (): Promise<void> => {
    const m = W.parseUsd(member);
    const p = W.parseUsd(project);
    if (m === null || p === null) {
      setSave({ state: "refused", line: "Enter an amount in US dollars, like 25.00." });
      return;
    }
    setSave({ state: "saving" });
    try {
      const v = await client.putQuotas(projectId, { revision: view.revision, memberDailyCents: m, projectDailyCents: p });
      setView(v);
      setSave({ state: "saved" });
    } catch (e) {
      const line = e instanceof ApiError ? W.refusalLine(e.status, e.details) : W.refusalLine(0, {});
      setSave({ state: "refused", line });
      if (e instanceof ApiError && e.status === 409) await load(true); // keep what was typed
    }
  };

  return (
    <section className="quota-admin" aria-label={W.QUOTAS_TITLE}>
      <h2 className="quota-title">{W.QUOTAS_TITLE}</h2>
      <p className="quota-asof quota-mono">{W.asOfLine(view.asOf)}</p>

      <p className="quota-figure">{W.memberQuotaLine(view)}</p>
      <Binds line={W.ceilingBindsLine("member", view.memberDaily.dailyCents, view.ceiling.memberDailyCents)} />
      <p className="quota-figure">{W.projectQuotaLine(view)}</p>
      <Binds line={W.ceilingBindsLine("project", view.project.dailyCents, view.ceiling.projectDailyCents)} />
      <p className="quota-figure">{W.projectTodayLine(view)}</p>

      {view.canEdit && (
        <form className="quota-edit" onSubmit={(ev) => { ev.preventDefault(); void onSave(); }}>
          <Field label="Each member, a day (US$)" value={member} onChange={setMember}
                 hint={W.ceilingHint(view.ceiling.memberDailyCents)} />
          <Field label="The project, a day (US$)" value={project} onChange={setProject}
                 hint={W.ceilingHint(view.ceiling.projectDailyCents)} />
          <button type="submit" disabled={save.state === "saving"}>Save</button>
          {save.state === "saved" && <p role="status">{W.SAVED}</p>}
          {save.state === "refused" && <p role="alert">{save.line}</p>}
        </form>
      )}

      <ul className="quota-members">
        {view.members.map((m) => (
          <li key={m.userId} className="quota-member">
            <p className="quota-figure">{W.memberTodayLine(m)}</p>
            {W.reachedLine(m) !== null && <p className="quota-reached">{W.reachedLine(m)}</p>}
            {m.byCounterpart.length > 0 && (
              <ul className="quota-counterparts">
                {m.byCounterpart.map((c) => <li key={c.userId} className="quota-figure">{W.counterpartLine(c)}</li>)}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

function Binds(props: { line: string | null }): JSX.Element | null {
  return props.line === null ? null : <p className="quota-binds">{props.line}</p>;
}

function Field(props: { label: string; value: string; hint: string; onChange: (s: string) => void }): JSX.Element {
  const bad = props.value !== "" && W.parseUsd(props.value) === null;
  return (
    <label className="quota-field">
      <span>{props.label}</span>
      <input inputMode="decimal" value={props.value} aria-invalid={bad}
             onChange={(e) => props.onChange(e.target.value)} />
      <span className="quota-hint">{props.hint}</span>
    </label>
  );
}
```

The input is `inputMode="decimal"` with `type` left as text: a `type="number"` field draws a
spinner control and accepts `1e3`, both of which this view does not want.

#### `SpendLine.tsx`, the hunk (task 56's file)

```tsx
// added imports: useState; Popover (../common/Popover); QuotaAdmin (./QuotaAdmin);
// QUOTAS_TITLE (./quotaWords); type TeamClient (../../api/team)
export function SpendLine(props: { settings: TeamSettings | null; client?: TeamClient }): JSX.Element | null {
  const [anchor, setAnchor] = useState<DOMRect | null>(null); // before the guard: hooks run every render
  if (props.settings === null) return null; // task 56's guard, unchanged (task-56.md:292)
  return (
    <>
      <p className="team-spend tabular-nums">{spendLineSentence(props.settings.spend)}</p>
      {props.client !== undefined && (
        <button type="button" className="team-quotas-link"
                onClick={(e) => setAnchor(e.currentTarget.getBoundingClientRect())}>
          {QUOTAS_TITLE}
        </button>
      )}
      <Popover anchor={anchor} onClose={() => setAnchor(null)}>
        {props.client !== undefined && <QuotaAdmin projectId={props.settings.projectId} client={props.client} />}
      </Popover>
    </>
  );
}
```

`SpendLine` renders only on a team project (task 56 mounts it there), so the link exists only
where quotas exist. The `client` prop is optional so task 56's own tests pass unchanged; the
popover that mounts `SpendLine` passes `createTeamClient()`.

Figures follow phase 6's `formatUsd` whole-number rule (`task-56.md:297-298`): US$25 a day, not
US$25.00; this part's sentences with `.00` are illustrative, and the tests expect `formatUsd`'s
output. The input fields are the one place `.00` is written, since they hold what the admin types.

#### `team-settings.css`, the added rules (tokens only)

```css
.quota-admin { display: grid; gap: var(--space-3); max-inline-size: 36rem; }
.quota-title { font: var(--text-title); margin: 0; }
.quota-figure, .quota-counterparts, .quota-edit input { font-variant-numeric: var(--numeric); }
.quota-mono { font-family: var(--mono); }
.quota-asof, .quota-hint, .quota-binds { color: var(--text-muted); }
.quota-reached { color: var(--text-strong); }
.quota-members, .quota-counterparts { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--space-2); }
.quota-counterparts { padding-inline-start: var(--space-4); }
.quota-field { display: grid; grid-template-columns: 14rem 8rem 1fr; align-items: baseline; gap: var(--space-2); }
```

The token names (`--space-*`, `--text-*`) are **UNVERIFIED** against `desktop/src/styles/tokens.css`
beyond `--numeric` (`:156`) and `--mono` (`:152`); the implementer maps each to the nearest
existing token and adds none. No rule sets `width` from data, no `transition`, and no
`::-webkit-progress-*` or `meter` style exists, so nothing can be drawn as a bar. The `font`
shorthand is avoided on the figures because it resets `font-variant-numeric`
(`desktop/src/components/develop/develop.css:389`).

#### `prototype/team-mode.html`, state `0` (task 51's file)

Key `0` on the switcher shows the popover with `QuotaAdmin`'s layout, from one added sample:

```js
SAMPLE.quotas = { v: 1, projectId: "pj-sample", team: true, day: "2026-09-25",
  asOf: "2026-09-25T04:20:00Z", currency: "USD", canEdit: true, revision: 4,
  ceiling: { memberDailyCents: 5000, projectDailyCents: 50000 },
  memberDaily: { dailyCents: 2500, enforcedCents: 2500 },
  project: { dailyCents: 20000, enforcedCents: 20000, spentCents: 6120, reached: false },
  members: [
    { userId: "u-alice", name: "Alice", enforcedCents: 2500, workersCents: 1840, coordinationCents: 310,
      outsideFleetCents: 0, reached: false, byCounterpart: [
        { userId: "u-bob", name: "Bob", costCents: 190, inputTokens: null, outputTokens: null },
        { userId: "", name: "", costCents: 120, inputTokens: null, outputTokens: null } ] },
    { userId: "u-bob", name: "Bob", enforcedCents: 2500, workersCents: 2500, coordinationCents: 0,
      outsideFleetCents: 0, reached: true, byCounterpart: [] } ] };
```

A second key, `Shift+0`, shows the same sample with `canEdit: false`, so the owner sees the
member's read-only view beside the admin's.

#### The tests, first (each names the mutant it kills)

`quotaWords.test.ts` (pure) and `QuotaAdmin.test.tsx` (React Testing Library over a fake
`TeamClient`; the desktop already pins `vitest` 5.0.1 and `@testing-library/react` 16.3.3,
`desktop/package.json:26-35`):

| Test | Asserts | Mutant killed |
|---|---|---|
| `non-admin sees read-only values` | `canEdit: false`: every sentence renders; `queryAllByRole("textbox")`, `queryByRole("button", {name: "Save"})` are empty, and no element has `disabled` | the scope's test; a disabled form shown to a member, or none of the values shown |
| `no element renders a progress bar` | for both samples: no `progress`, `meter` or `[role=progressbar]`/`[role=meter]` element; no inline `style` with `width`; no text node contains `%` | the scope's test; a bar, a meter, or a percentage (OD-27) |
| `no builder writes a ratio` | every `quotaWords` builder over the sample: no `/` between figures and no match of `/US\$[\d,.]+\s+of\s+US\$/` | an "N of M" sentence such as "US$18 of US$25" |
| `admin edits within the ceiling` | `canEdit: true`: typing `30.00` and `250` and Save sends `{revision: 4, memberDailyCents: 3000, projectDailyCents: 25000}` once; the new view's sentences render and `Saved.` shows | dollars sent as cents or cents as dollars; the revision dropped |
| `ceiling refusal is shown in words` | the fake rejects `ApiError(bad_request, …, 422, {reason: "above_org_ceiling", ceilingCents: 5000})`: the alert reads `Above your organisation's ceiling of US$50.` | a client-side clamp; a generic error for the one refusal an admin can act on |
| `stale revision keeps what was typed` | the fake rejects 409 once: `quotas` is read again, the fields still hold the typed values, and the alert names both sentences | a reload that discards the admin's input; a silent overwrite |
| `null is not zero` | `workersCents: null`: `workers not known yet`; `asOf: null`: `No usage posted today yet.`; tokens `null`: no `tokens` word at all | an unknown figure printed as US$0 or 0 tokens |
| `tokens appear when known` | `inputTokens: 30000, outputTokens: 11000`: `coordination with Bob: US$1.90 today, 41k tokens` | tokens never shown after the gateway starts reading them |
| `the ceiling that binds is said` | `memberDaily.dailyCents: 25000`, ceiling 10000: the `Set to … your organisation's ceiling of … applies.` line renders; equal values: it does not | a quota shown that the gateway does not enforce |
| `figures are tabular, times mono` | each `.quota-figure` resolves `font-variant-numeric: tabular-nums` (computed from the stylesheet the test imports), the `As of` line carries `quota-mono` | proportional digits; a time in the text face (OD-28) |
| `parseUsd refuses what it must` | `"0"`, `"-1"`, `"1.234"`, `"1e3"`, `""` → null; `"25"` → 2500, `"25.5"` → 2550 | a zero quota sent as "off"; a third decimal silently rounded |
| `the link is team-only` | `SpendLine` with `settings: null`: no *Team quotas* button; with settings and a client: the button opens the popover | an admin surface on a solo project |

The computed-style assertion depends on jsdom applying the imported stylesheet; if the desktop's
vitest setup does not load CSS (**UNVERIFIED**; task 56's `figures are tabular` test checks the
class, `task-56.md:189`), the test checks the class instead, as task 56 does.

#### Steps

1. Add state `0` and `Shift+0` to `prototype/team-mode.html` with `SAMPLE.quotas` above, in task
   51's markup and tokens. Commit. **Stop for the owner's review.** A changed sentence goes into
   `quotaWords.ts`'s constants before step 3.
2. Add the `QuotaView` block to `store/types.ts` and the two methods to `api/team.ts`; run
   `npm run typecheck`.
3. Write both test files; run `npm test -- quota`; each fails (no module).
4. Write `quotaWords.ts`, then `QuotaAdmin.tsx`, then the `SpendLine` hunk and the CSS. Run the
   tests until green, then the whole suite.
5. Commit the desktop half with the paths above.

#### Acceptance

```bash
cd desktop
npm run typecheck                         # expect exit 0, no output past the banner
npx vitest run src/components/team/quotaWords.test.ts src/components/team/QuotaAdmin.test.tsx
# expect: Test Files 2 passed (2); Tests 12 passed (12)
npm test                                  # expect every file passed, task 56's SpendLine tests included
grep -rn "progress\|meter\|%" src/components/team/QuotaAdmin.tsx src/components/team/quotaWords.ts
# expect no output
grep -n "transition" src/components/team/team-settings.css   # expect no output from the added rules
```

Visual acceptance is the owner's eyes against state `0` and `Shift+0` of the approved prototype,
not this list; the corporate network blocks headless screenshots (`CLAUDE.md`). The live read
against a real Hub and gateway is deferred to task 69 item 6.

#### What this task does NOT claim

- **That the figures are live.** They are as of the gateway's last post, up to ten minutes old
  (task 66 decision f), and the view says when.
- **That a member's own coordination split here equals task 56's.** Task 56's line is task 37's
  exact split from Fleet; this view is the Hub's roll-up. They agree once the day's last post lands.
- **An org-ceiling editor.** Route 81 is the org admin's; no desktop surface for it is built here.
- **The compliance holder's "move back to Global".** Task 56 says *"The admin surface for it is
  task 67's"* (`task-56.md:53-55`); this task's scope names quotas only, so it is not drawn (gap 2).

#### Gaps, raised here and not resolved alone

1. **No prototype state existed for this view.** Step 1 adds it; the build waits for the owner.
2. **Task 56 hands the compliance lowering control to task 67** (`task-56.md:53-55`), and the
   skeleton's scope for 67 does not carry it. Either task 67's scope grows by one row (*"Move this
   project back to Global"*, shown only to a `Hub.Compliance` holder, through route 79) or another
   task owns it; the skeleton should say which.
3. **Task 56's `spendLineSentence` says *"no daily quota set"*** for a `null` quota
   (`task-56.md:287`). After task 66 every team project has a non-zero quota (OD-24), so that
   branch now means "not known yet", and its words should say so.
4. **Task 52's `Spend` comment says the Australia/Sydney day** (`task-52.md:576`); this view and
   task 66 use the UTC day.

