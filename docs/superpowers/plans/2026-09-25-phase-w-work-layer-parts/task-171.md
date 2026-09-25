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
