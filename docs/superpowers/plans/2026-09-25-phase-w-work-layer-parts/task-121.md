### Task 121: the janitor, W4: recurring issues, update reminders, triage rotation

W4, stream WH; no seam. Depends on task 78 (issue templates `tp-`) and task 116 (the W4 tables).
It extends the Hub's janitor, Hub task 24's `server/janitor.go` (`phase-h-hub.md:306`; its part
`2026-09-23-phase-h-hub-parts/task-24.md`, cited `H24:N`, read on `feat/hub` at `1271ddb`), after
task 77's W2 passes, which it leaves alone. Task 77's and task 78's parts are not written at this
writing; names taken from them are their scopes' (`tasks-61-80.md`) and **UNVERIFIED**. The wire is
task 114's part (`task-114.md`, in progress, cited `T114` by type name). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1).

Three things the Hub now does on its own, on a clock, for a policy a person set: make an issue
from a template on a schedule (L-32), ask a feature's lead for a status update when the last one is
stale (L-77), and hand triage duty to the next person in a rotation (L-50). None of them decides
anything: each carries out a stored rule and says, in its event, whose rule it was.

Seven decisions the scope left open, made here.

1. **One clock, one lock, the Hub's rules.** The W4 passes run inside Hub task 24's `Sweep`, after
   task 77's, under the same `pg_try_advisory_lock(hashtext('panacea-app-hub-janitor'))` and the
   same per-org transaction with `set_config('hub.org_id', org, true)` and `store.LockHead`
   (`H24:80-101`). `now` is `cfg.Now()`, so a test moves time without sleeping. Hub task 24 already
   exports `Sweep` *"for the tests"* (`H24:104-105`); task 124 asks for a named one-run hook
   (`task-124.md`, *Gaps* 4), which is `RunJanitorOnce` below, a wrapper in a `_test.go` file of
   package `server`, so it never ships in the binary.
2. **A schedule needs an anchor, and T114's `TemplateSchedule` has none.** `{every, at, tz}` says
   *how often* but not *which Monday* or *which day of the month*. **Gap G1, fixed in 114:**
   `TemplateSchedule` gains `startsOn` (`YYYY-MM-DD`, a local date in `tz`; optional in the
   request, defaulting to the local date the schedule is set). Occurrence *k* is `startsOn` plus
   *k·n* days, weeks or months, at `at` local; a month with no such day uses its last day, so *the
   31st* runs on 30 September. Local wall time becomes an instant through `time.Date(…, loc)`; an
   `at` inside the hour a daylight-saving start skips runs at the first minute after it (Go leaves
   that choice unspecified, so the code normalises it and a test pins it).
3. **Setting a schedule never backfills.** `PUT …/schedule` stores `last_run_at` as the latest
   occurrence at or before `now`, so the first issue comes at the **next** occurrence; a person who
   sets a weekly schedule on a Wednesday does not get last Monday's issue.
4. **Missed periods make one issue.** When the Hub was down across several occurrences, the pass
   creates **one** issue, for the latest occurrence passed, and its run event says `skipped: n`
   (the occurrences not created). The run key is `(scheduleId, occurrenceAt)`: the pass moves
   `last_run_at` to the occurrence in the same transaction as the create, so a second run finds
   nothing due.
5. **One event per changed record, and whose rule it was.** A recurring create changes two
   records, the issue and the schedule, so it appends two events: task 13's `issue.created`
   (`source: "recurring"`, `actor: null`) and `template.schedule_ran {templateId, issueId,
   occurrenceAt, skipped, by}` on the schedule, where `by` is the person who set it. The scope asks
   for *"the author in `data`"* of the created issue's event; `IssueCreatedData` is W1's
   (`task-13.md:142-148`) and has no such field, so the author rides on the schedule's event
   instead of changing a W1 type. A reminder changes the policy (`last_reminded_at`) and appends
   `update.reminder`; a handover changes the rotation (`last_handover_on`) and appends
   `triage.handover`. Every janitor event has `actor: null` (the Hub's rule, `H24:92`).
6. **Days are whole 24-hour spans for reminders, local dates for the rotation.** `daysSince` is
   `floor((now − since) / 24 h)`, with `since` the subject's latest update (task 119's
   `LatestUpdateAt`, erased updates included) or the policy's creation when there is none. A
   rotation's period boundaries are local midnights; T114's `TriageRotation` has no zone, so
   **G1** also adds `tz` (optional, default `"UTC"`).
7. **Duty is derived on read over current members.** `members` filtered to those still in the team,
   order kept; duty is `current[floor(days(startsOn → today) / periodDays) mod len(current)]`. The
   only write is the handover event at a boundary; a boundary crossed while the Hub was down gives
   one event for the latest boundary, `skipped: n`, as decision 4.

**The one thing this task must not get wrong** is a burst. A janitor that restarts after a weekend
and creates twelve issues, or reminds a lead on every sixty-second sweep, teaches a team to ignore
everything it makes. Decisions 3 and 4 and the per-period `last_*` columns are the guard, and
`TestTwoJanitorsCreateOneIssue` and `TestASameDayRerunRemindsNobody` hold it.
