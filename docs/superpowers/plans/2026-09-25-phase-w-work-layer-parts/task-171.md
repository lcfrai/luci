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
   consent), the desktop opens it in the **system browser** through the shell's external-open
   (as sign-in does, OD-6), and the connection reads *waiting for consent* until the Hub's next
   `tracker.connected` event. The webview never loads a tracker page.
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
