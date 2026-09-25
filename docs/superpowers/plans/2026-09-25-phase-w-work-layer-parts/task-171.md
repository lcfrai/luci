### Task 171: trackers on the desktop: connect Azure Boards, GitHub Issues or Jira; *not tied · imported*; the sync log

W5, stream WD, about 2.5 hours (an estimate, not a measurement). Depends on task 169 (the W5
desktop wire: `TrackerConnection`, `TrackerTarget`, `TrackerMode`, `ExternalLink`,
`UpstreamChange`, `SyncRun`, the tracker members of `WorkIntakeClient` and the `tracker.mode` op,
`task-169.md`). It also uses task 42's `threadWords` (`task-42.md:196-202`, which already renders
*not tied · imported*), task 44's `ConfirmInPlace` (`task-44.md:130-150`), task 20's
`expectNoPrediction` (`task-20.md:126-131`) and task 170's `testWork.ts` harness.

Parity: closes, jointly with tasks 150-153, **L-202** (GitHub Issues and Azure Boards), **L-215**
(Jira, two way for comments) and the W5 half of **L-220** (importers; *others later* keeps it
PARTIAL). Design: 05 §1 N (L-202, L-215, L-220), §3.3 (sync never moves a state; SW.3), §5.4 W5
(*"one way in, into `triage` as not tied · imported … Sync never writes a state, a fact or
`done`"*, quoted by task 150's scope, `tasks-141-160.md:419-421`).

**What the reader does.** A team admin opens *Team settings → Trackers*, picks *Jira*, types the
site `contoso.atlassian.net` and the project key `PAY`, and presses *Connect*. The form reads *One
way in, into Triage*. Rows arrive in the sync log as the first pass lands: *14:02:11 · in ·
PAY-381 · imported*. A week later one row reads *PAY-377 · closed in Jira · not applied: sync never
moves a state*. The admin turns on *comments both ways*; the switch asks first, because comment
text will leave the product.

**Decisions made here, so nobody re-derives them:**

1. **The surface never holds a tracker credential.** The connect form has no token, PAT, password
   or key field. How the Hub authorises is tasks 150-153's (a `secret_ref` or the Hub's workload
   identity). When `connectTracker` answers a `consentUrl` (a GitHub App installation, a Jira OAuth
   consent), the desktop opens it in the **system browser** through `@tauri-apps/plugin-opener`'s
   `openUrl` (the plugin is already a dependency: `desktop/package.json:17`, used at
   `desktop/src/components/menu/ContextMenu.tsx:44`), only when the URL is `https:`, and the
   connection reads *waiting for consent* until the Hub's next `tracker.connected` event. The
   webview never loads a tracker page.
2. **The target is validated before the Hub call, in the tracker's own terms.** `validateTarget`
   below checks each field against the tracker's naming rule as recorded here and names the field
   and the rule. The Hub's 422 stays a second check, never the only one.
3. **Direction is words.** One line under the tracker's name: *One way in, into Triage*. The
   *comments both ways* switch is **off** on every new connection, and turning it on shows
   `ConfirmInPlace` with *Comments people write here will be posted to Jira, under their name. Agent
   notes never leave.* (task 153's rule). Turning it off is immediate and needs no confirm.
4. **The log shows what the Hub recorded, no more.** A `SyncRun` gives counts and `errors`
   (task 169 decision 7), and an `ExternalLink` gives one imported item and its latest
   `UpstreamChange`. `syncLogRows` merges the two into rows newest first: one row per link imported
   in the window of that run (*imported*), one per error (*refused* with the code's reason), one per
   upstream change (*closed in Jira · not applied*), and one per-run summary row (*run · 14
   created, 3 existing*) that states the run's counts as facts of that pass. It never draws an
   *updated* or *skipped* row the Hub did not name (task 169 G5).
5. **Arrivals, never progress.** While a first import runs, new rows appear at the top as
   `external_link.created` events land (the replica's task 169 kinds). No bar, no *N of M*, no
   total to reach and no *importing… 40%*: the Hub does not know the total and neither do we
   (OD-27). The one heartbeat renders *last arrival 14:02:11*; nothing spins.
6. **Disconnect keeps history.** *Disconnect* confirms in place (*Stop syncing PAY? Imported issues
   and their links stay.*) and calls `disconnectTracker`; the links stay and still resolve (task
   150's test), so every imported row keeps *not tied · imported* and its external link.

**Files:**
- `desktop/src/components/intake/TrackerConnect.tsx` (+ `TrackerConnect.test.tsx`) — pick, target, connect (new)
- `desktop/src/components/intake/TrackerList.tsx` (+ `TrackerList.test.tsx`) — connections, mode switch, disconnect (new)
- `desktop/src/components/intake/SyncLog.tsx` (+ `SyncLog.test.tsx`) — the log and `syncLogRows` (new)
- `desktop/src/components/intake/trackerTarget.ts` (+ `trackerTarget.test.ts`) — `validateTarget` (new)
- `desktop/src/components/intake/intake.css` ⚑ (task 170) — `.ik-trk-*`, `.ik-log-*` (modified)

The three components mount under team settings (the File map's `src/components/{…,intake,…}`
row, skeleton `:314`). Where team settings lives is task 11's and task 53's; if no *Team settings*
host exists when this lands, `TrackerList` mounts on the team's Triage header menu (gap 1).

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 20, 42, 44, 169 and
170. There is **no part** for tasks 150-153; every route, field and event named from them is their
scopes' (`tasks-141-160.md:418-545`) and **UNVERIFIED**. The naming rules in `validateTarget` are
recorded from the trackers' public behaviour as this part understands it and are **UNVERIFIED**
against each tracker's current documentation; the tests fix what the desktop accepts.

#### Interfaces — complete: `desktop/src/components/intake/trackerTarget.ts`

```ts
// desktop/src/components/intake/trackerTarget.ts — task 171 (decision 2). Pure; no network.
import type { TrackerSource, TrackerTarget } from "../../api/work";   // task 169

export interface TargetProblem { field: string; rule: string }

const AZ_ORG = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,48}[A-Za-z0-9])?$/;       // 1-50, no edge hyphen
const AZ_PROJECT_BAD = /[\\/:*?"<>|;#$*{},+=[\]]/;                      // reserved characters
const GH_OWNER = /^[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$/; // 1-39, single hyphens
const GH_REPO = /^[A-Za-z0-9._-]{1,100}$/;
const JIRA_SITE = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.atlassian\.net$/;   // Jira Cloud
const JIRA_KEY = /^[A-Z][A-Z0-9_]{1,9}$/;                               // the default key rule

/** Every problem, in field order; [] means the Hub may be called. */
export function validateTarget(t: TrackerTarget): TargetProblem[] {
  const out: TargetProblem[] = [];
  const need = (field: string, v: string, ok: boolean, rule: string) => { if (!ok || v.trim() !== v) out.push({ field, rule }); };
  switch (t.source) {
    case "azure_boards":
      need("organisation", t.organisation, AZ_ORG.test(t.organisation), "letters, digits and inner hyphens, 1-50");
      need("project", t.project, t.project.length > 0 && t.project.length <= 64 && !AZ_PROJECT_BAD.test(t.project)
        && !t.project.startsWith("_") && !t.project.endsWith("."), "1-64 characters, none of \\ / : * ? \" < > | ; # $ { } , + = [ ]");
      if (t.areaPath !== null)
        need("areaPath", t.areaPath, t.areaPath === t.project || t.areaPath.startsWith(`${t.project}\\`),
          "starts with the project name, levels split by \\");
      break;
    case "github":
      need("owner", t.owner, GH_OWNER.test(t.owner), "letters, digits and single inner hyphens, 1-39");
      need("repository", t.repository, GH_REPO.test(t.repository) && t.repository !== "." && t.repository !== "..",
        "letters, digits, . _ -, 1-100");
      break;
    case "jira":
      need("site", t.site, JIRA_SITE.test(t.site), "a Jira Cloud site, name.atlassian.net");
      need("projectKey", t.projectKey, JIRA_KEY.test(t.projectKey), "2-10 capitals, digits or _, a capital first");
      break;
  }
  return out;
}

/** The tracker's own words for the form (decision 2): what each field is called there. */
export const TARGET_LABELS: Readonly<Record<TrackerSource, readonly [string, string][]>> = {
  azure_boards: [["organisation", "Organisation"], ["project", "Project"], ["areaPath", "Area path (optional)"]],
  github: [["owner", "Owner"], ["repository", "Repository"]],
  jira: [["site", "Site"], ["projectKey", "Project key"]],
};

export const SOURCE_NAME: Readonly<Record<TrackerSource, string>> = {
  azure_boards: "Azure Boards", github: "GitHub Issues", jira: "Jira",
};
```

`JIRA_KEY` is Jira's **default** project-key rule; a Jira admin can change the pattern, so a key
the site accepts may be refused here (gap 2). The refusal names the rule, so the person knows why,
and the Hub's own call is never reached with a key this form refused. Task 153 pins Jira Cloud;
a Jira Data Center site (not `*.atlassian.net`) is refused with the rule's words, which is the
scope's *Jira Cloud* claim and nothing more.

#### Interfaces — complete: `desktop/src/components/intake/SyncLog.tsx`, the exported half

```tsx
// desktop/src/components/intake/SyncLog.tsx — task 171 (decision 4). A view, not a wire type.
import type { JSX } from "react";
import type { ExternalLink, ProblemCode, SyncRun, TrackerSource } from "../../api/work";
import { SOURCE_NAME } from "./trackerTarget";

export type SyncResult = "imported" | "refused" | "upstream" | "run";
/** One log row. `direction` is "in" for every row W5 can write, "out" for a posted comment. */
export interface SyncLogEntry {
  at: string; direction: "in" | "out"; externalId: string | null;
  result: SyncResult; words: string;   // the row's reason or summary, in sans
  url: string | null;                  // the external item, https only
}

const REFUSAL: Partial<Record<ProblemCode, string>> = {
  principal_cannot_move_state: "not applied: sync never moves a state",
  validation_failed: "not imported: the item failed a check",
  forbidden: "not applied: sync may not write this",
};
const trackerName = (s: TrackerSource) => (s === "azure_boards" ? "Azure Boards" : s === "github" ? "GitHub" : "Jira");

/** An upstream change, as the fact it is: "closed in Jira · not applied: sync never moves a state". */
export function upstreamWords(field: string, source: TrackerSource): string {
  const act = field === "state" ? "closed" : field === "assignee" ? "reassigned" : `${field} changed`;
  return `${act} in ${trackerName(source)} · not applied: sync never moves a state`;
}

/** Runs newest first; links imported at or before a run and after the previous one belong to it. */
export function syncLogRows(runs: readonly SyncRun[], links: readonly ExternalLink[], source: TrackerSource): SyncLogEntry[] {
  const rows: SyncLogEntry[] = [];
  for (const run of runs) {
    rows.push({ at: run.at, direction: "in", externalId: null, result: "run", url: null,
      words: `run · ${run.created} created, ${run.existing} existing` +
        (run.commentsIn + run.commentsOut > 0 ? `, ${run.commentsIn} comments in, ${run.commentsOut} out` : "") });
    for (const e of run.errors)
      rows.push({ at: run.at, direction: "in", externalId: e.externalId, result: "refused", url: null,
        words: REFUSAL[e.code] ?? `not applied: ${e.code}` });
  }
  for (const l of links) {
    rows.push({ at: l.lastSeenAt, direction: "in", externalId: l.externalId, result: "imported", url: l.url, words: "imported" });
    if (l.upstream) rows.push({ at: l.upstream.at, direction: "in", externalId: l.externalId, result: "upstream",
      url: l.url, words: upstreamWords(l.upstream.field, source) });
  }
  return rows.sort((a, b) => (a.at < b.at ? 1 : a.at > b.at ? -1 : 0));
}

export function SyncLog(props: { connectionId: string; source: TrackerSource; now(): number }): JSX.Element;
```

The imported row's `at` is `lastSeenAt`, the time the Hub last saw the item, which is the import
time for an item not seen since; task 150 records no separate *importedAt* (gap 3). `SOURCE_NAME`
labels the log's header (*Jira · PAY*). Times and external ids render in mono (`--t-fact`,
`tabular-nums`); `words` in sans. The log pages through `syncRuns` and `trackerLinks` with a *Show
older* button, never infinite scroll that jumps; it throttles appends to about 10 Hz and never
scrolls when the reader has scrolled up (DESIGN-SYSTEM's log-pane rule).

#### Interfaces — complete: `TrackerConnect.tsx` and `TrackerList.tsx`, the exported halves

```tsx
// desktop/src/components/intake/TrackerConnect.tsx — task 171 (decisions 1-3).
export function TrackerConnect(props: {
  teamId: string;
  projects: readonly { id: string; key: string; name: string }[];   // the team's projects
  onConnected(c: TrackerConnection): void;
  onCancel(): void;
}): JSX.Element;
// Step 1: three buttons, Azure Boards · GitHub Issues · Jira. Step 2: the project it lands in,
// then TARGET_LABELS[source]'s fields (text inputs, no password type, no "token" anywhere), the
// line "One way in, into Triage", and Connect. Connect runs validateTarget; any problem is shown
// under its field and nothing is sent. Otherwise one connectTracker(projectId, {target, mode:
// "in"}) call; a consentUrl opens through openUrl (https only) and the row waits for consent.

// desktop/src/components/intake/TrackerList.tsx — task 171 (decisions 3, 6).
export function TrackerList(props: { teamId: string; canEdit: boolean; now(): number }): JSX.Element;
// One row per TrackerConnection: SOURCE_NAME, the target in its own terms (mono), the direction
// line, "last run 14:02" (mono; "not run yet" before the first), the comments-both-ways switch
// (role="switch", aria-checked), Disconnect, and the SyncLog under a disclosure. A member sees the
// same rows read-only. A disconnected row stays, reading "disconnected · links kept".
```

`mode` is always `"in"` at connect: the switch is on the row, not in the form, so a new connection
cannot start in `in_and_comments` (decision 3). The switch sends task 169's `tracker.mode` op with
`prev`, one Undo.
