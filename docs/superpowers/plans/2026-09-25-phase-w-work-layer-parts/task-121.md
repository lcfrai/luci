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

**Files:**

- `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's file; task 77's passes before
  this): one call, `sweepWorkW4(ctx, tx, org, now, &report)`, after task 77's, and three counters on
  `SweepReport`
- `control-plane/internal/hub/server/janitor_w4.go` (new): the three passes
- `control-plane/internal/hub/server/recur.go`, `recur_test.go` (new): the pure occurrence rule,
  and `import _ "time/tzdata"`, so an IANA zone loads in an image with no `/usr/share/zoneinfo`
  (whether the Hub image carries one is **UNVERIFIED**; the import makes it not matter, at about
  450 KB of binary, per the `time/tzdata` package doc)
- `control-plane/internal/hub/store/work_schedules.go`, `server/work_schedules.go` (new): the
  policy store and its five routes
- `control-plane/internal/hub/server/janitor_w4_export_test.go` (new): `RunJanitorOnce`
- `control-plane/internal/hub/server/janitor_w4_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: the `"121"` rows

**Interfaces:**

- Produces: routes `PUT` and `DELETE /v1/templates/{templateId}/schedule`, `PUT
  /v1/reminder-policies`, `PUT` and `GET /v1/teams/{teamId}/triage-rotation` (T114's route table);
  `server.RunJanitorOnce` (tests only); the kinds of G2 below; `SweepReport.Recurring`,
  `.Reminded`, `.HandedOver`
- Consumes: T114's `TemplateSchedule`, `Every`, `ReminderPolicy`, `TriageRotation`,
  `TriageRotationView`, `SubjectRef`; task 116's `work_template_schedules`,
  `work_reminder_policies`, `work_triage_rotations`; task 78's issue template row and its prefill
  copy; task 08's issue create; task 119's `store.LatestUpdateAt`; task 76's feature `lead_id` and
  task 118's initiative `owner`; task 13's `emitWork`; Hub task 24's `Sweep`, `SweepReport`,
  `JanitorConfig.Now`
- Consumed by: task 81's digest builder (it reads `update.reminder` into the lead's Desk digest),
  task 124 (the methods over HTTP; `RunJanitorOnce` in its end to end), task 127 (the reminder
  row), task 134 (the settings surfaces)

**Gaps.** **G1, task 114:** `TemplateSchedule.StartsOn` and `TriageRotation.TZ` (decisions 2 and
6). **G2, task 114:** the seven kinds, none in T114 at this writing (task 119 names the same gap):

| Kind | Scope | `subject` | `data` (scalars only) |
|---|---|---|---|
| `template.schedule_set` | team | `tp-` | `{every, at, tz, startsOn}` (`every` as `"2 week"`) |
| `template.schedule_cleared` | team | `tp-` | `{}` |
| `template.schedule_ran` | team | `tp-` | `{issueId, occurrenceAt, skipped, by}` |
| `reminder.policy_set` | project, or team for an initiative | `ft-` or `in-` | `{subjectKind, everyDays}` |
| `update.reminder` | project, or team for an initiative | `ft-` or `in-` | `{subjectKind, lead, daysSince, by}` |
| `triage.rotation_set` | team | `tm-` | `{members, periodDays, startsOn, tz}` (`members` a count) |
| `triage.handover` | team | `tm-` | `{from, to, skipped, by}` |

**G3, task 116:** the three tables must carry `work_template_schedules.last_run_at timestamptz`
(the scope says so), `work_reminder_policies.last_reminded_at timestamptz NULL` and
`created_at`, and `work_triage_rotations.last_handover_on date NULL`. If 116's part omits them,
116 adds them; this task appends no migration.

#### The occurrence rule (`server/recur.go`, complete)

```go
package server

import (
	"time"
	_ "time/tzdata" // IANA zones without the host's zoneinfo
)

// Occurrence k of a schedule: startsOn plus k·n units, at hh:mm local in loc.
// A month without the anchor day uses its last day. A wall time that a
// daylight-saving start skips moves forward to the first instant after the gap.
func occurrence(startsOn time.Time, n int, unit string, hh, mm int, loc *time.Location, k int) time.Time {
	y, m, d := startsOn.Date()
	switch unit {
	case "day":
		y, m, d = time.Date(y, m, d+k*n, 12, 0, 0, 0, time.UTC).Date()
	case "week":
		y, m, d = time.Date(y, m, d+7*k*n, 12, 0, 0, 0, time.UTC).Date()
	case "month":
		first := time.Date(y, m+time.Month(k*n), 1, 12, 0, 0, 0, time.UTC)
		last := first.AddDate(0, 1, -1).Day()
		y, m = first.Year(), first.Month()
		if d > last {
			d = last
		}
	}
	t := time.Date(y, m, d, hh, mm, 0, 0, loc)
	if h, mi, _ := t.Clock(); h == hh && mi == mm {
		return t
	}
	// The gap: step the asked wall time forward a minute at a time until Go
	// returns the wall time asked for; that is the first minute after the gap.
	for i := 1; i <= 180; i++ {
		w := time.Date(y, m, d, hh, mm+i, 0, 0, loc)
		want := (hh*60 + mm + i) % (24 * 60)
		if h, mi, _ := w.Clock(); h*60+mi == want {
			return w
		}
	}
	return t // unreachable for every zone in the IANA database (gaps are at most 2 h)
}

// dueOccurrence is decision 4: the latest occurrence in (after, now], and how
// many earlier ones in that window it stands for. ok is false when none is due.
func dueOccurrence(after, now, startsOn time.Time, n int, unit string, hh, mm int, loc *time.Location) (at time.Time, skipped int, ok bool) {
	for k := 0; k < 10000; k++ { // a bound: 10,000 daily occurrences is 27 years
		t := occurrence(startsOn, n, unit, hh, mm, loc, k)
		if t.After(now) {
			break
		}
		if t.After(after) {
			if ok {
				skipped++
			}
			at, ok = t, true
		}
	}
	return at, skipped, ok
}
```

The date arithmetic runs at noon UTC so a day never crosses a boundary; only the final
`time.Date(…, loc)` is local. The gap rule compares the wall clock Go returned with the one asked
for, which is the documented way to detect the non-existent time (`time.Date`'s doc: *"the choice
of time zone, and therefore the time, is not guaranteed"*); the forward walk then lands on the
first real minute, 03:00 on 2026-10-04 in Sydney for an `at` of `02:30`. That a zone's gaps are at
most two hours is from knowledge of the IANA data, **UNVERIFIED** here; the loop's bound is three. The linear scan from `k = 0` is simple and bounded;
an index from `after` is a later optimisation if a profile asks.
