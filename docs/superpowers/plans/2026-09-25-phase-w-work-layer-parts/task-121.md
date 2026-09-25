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

#### The passes (`server/janitor_w4.go`, contract complete)

```go
package server

// sweepWorkW4 runs in the org's janitor transaction, after task 77's passes,
// with the org's audit head already locked (H24:86-90). Each pass appends its
// events with Actor nil and the policy's author in data (decision 5).
func sweepWorkW4(ctx context.Context, tx *Tx, org string, now time.Time, r *SweepReport) error {
	if err := passRecurring(ctx, tx, now, r); err != nil {
		return err
	}
	if err := passReminders(ctx, tx, now, r); err != nil {
		return err
	}
	return passHandovers(ctx, tx, now, r)
}

// passRecurring: for each schedule of a live issue template, dueOccurrence
// after last_run_at. If due: task 78's prefill copy of the template into task
// 08's create, state "triage", source "recurring", created_by the schedule's
// author, Actor nil; emit issue.created; UPDATE last_run_at = the occurrence;
// emit template.schedule_ran{issueId, occurrenceAt, skipped, by}. A template
// archived since the schedule was set is skipped, and the schedule stays.
func passRecurring(ctx context.Context, tx *Tx, now time.Time, r *SweepReport) error

// passReminders: for each policy, since = LatestUpdateAt(subject) or the
// policy's created_at; daysSince = floor(now−since / 24h). Due when
// daysSince >= everyDays and (last_reminded_at is null or last_reminded_at <
// since or now − last_reminded_at >= everyDays·24h). If due: UPDATE
// last_reminded_at = now; emit update.reminder{subjectKind, lead, daysSince, by}.
// The lead is the feature's lead (task 76) or the initiative's owner (task 118);
// a subject that is done or cancelled is skipped, and its policy stays.
func passReminders(ctx context.Context, tx *Tx, now time.Time, r *SweepReport) error

// passHandovers: for each rotation, the period index p = floor(localDays(
// startsOn → today in tz) / periodDays), and the index of last_handover_on the
// same way. If p is greater: from = duty at the recorded period, to = duty at p,
// both over current members (decision 7); UPDATE last_handover_on = the
// boundary date; emit triage.handover{from, to, skipped: p − last − 1, by}.
// A rotation whose current members are empty is skipped and reads onDuty null.
func passHandovers(ctx context.Context, tx *Tx, now time.Time, r *SweepReport) error

// SweepReport (Hub task 24's struct) gains, additively:
//	Recurring, Reminded, HandedOver int
```

`created_by` on the recurring issue is the schedule's author, so the issue's own record says whose
rule made it and the list shows *"Ada's weekly template"*, not a system user; the event's `actor`
stays `null`, which is how the feed tells a person's act from the janitor's (05 §3.6, as cited in
the scope). Whether task 08's create path accepts a `created_by` different from the principal is
**UNVERIFIED** until its part lands; if it does not, the janitor calls a store function beside it
with the same checks and no principal, and the part for task 08 is not changed.

```go
// server/janitor_w4_export_test.go (package server; never in the binary)

// RunJanitorOnce is task 124's hook: one Sweep at now, the W4 passes included.
func RunJanitorOnce(ctx context.Context, st *store.Store, blobs blobstore.Store, now time.Time) (SweepReport, error) {
	return Sweep(ctx, st, blobs, JanitorConfig{
		Interval: time.Minute, Retention: 24 * time.Hour,
		Now: func() time.Time { return now }, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}
```

Task 124 names it `server.RunJanitorOnce(ctx, clk.Now())`; its end to end must be in package
`server` to call it, or pass `st` and `blobs` as this signature needs. That is a note for 124.

#### The store (`store/work_schedules.go`, signatures complete)

```go
package store

type ScheduleRow struct {
	TemplateID, TeamID, By string
	N                      int
	Unit, At, TZ, StartsOn string
	LastRunAt              time.Time
}
func PutSchedule(ctx context.Context, tx pgx.Tx, s ScheduleRow) error // upsert on template_id
func DeleteSchedule(ctx context.Context, tx pgx.Tx, templateID string) (deleted bool, err error)
func DueSchedules(ctx context.Context, q Querier) ([]ScheduleRow, error) // every row of the org; the pass filters
func SetLastRun(ctx context.Context, tx pgx.Tx, templateID string, at time.Time) error

type ReminderRow struct {
	SubjectKind, SubjectID, TeamID, ProjectID, By string // ProjectID "" for an initiative
	EveryDays                                     int
	CreatedAt                                     time.Time
	LastRemindedAt                                *time.Time
}
func PutReminderPolicy(ctx context.Context, tx pgx.Tx, r ReminderRow) error // upsert on the subject
func ReminderPolicies(ctx context.Context, q Querier) ([]ReminderRow, error)
func SetLastReminded(ctx context.Context, tx pgx.Tx, subjectKind, subjectID string, at time.Time) error

type RotationRow struct {
	TeamID, By, StartsOn, TZ string
	Members                  []string
	PeriodDays               int
	LastHandoverOn           *string
}
func PutRotation(ctx context.Context, tx pgx.Tx, r RotationRow) error // upsert on team_id
func RotationOf(ctx context.Context, q Querier, teamID string) (RotationRow, bool, error)
func Rotations(ctx context.Context, q Querier) ([]RotationRow, error)
func SetLastHandover(ctx context.Context, tx pgx.Tx, teamID, on string) error
```

#### The routes (`server/work_schedules.go`)

```go
mux.HandleFunc("PUT /v1/templates/{templateId}/schedule", s.putSchedule)
mux.HandleFunc("DELETE /v1/templates/{templateId}/schedule", s.deleteSchedule)
mux.HandleFunc("PUT /v1/reminder-policies", s.putReminderPolicy)
mux.HandleFunc("PUT /v1/teams/{teamId}/triage-rotation", s.putRotation)
mux.HandleFunc("GET /v1/teams/{teamId}/triage-rotation", s.getRotation)
```

- **`putSchedule`**, in `s.Write`: the template found and readable (404 `template_not_found`, task
  78's code, **UNVERIFIED** name); an org template or a brief template → 422 `field:
  "templateId"`, `reason` `org_template` or `not_issue_template` (a recurring issue needs one team
  and an issue shape); the caller a team admin (403 `required: admin`); `TemplateSchedule` decoded,
  `templateId` absent or equal to the path; T114's `Validate`; `time.LoadLocation(tz)` succeeds
  (else 422 `field: "tz"`); `startsOn` defaulted (decision 2); `last_run_at` set by decision 3;
  `PutSchedule`; one `template.schedule_set`; 200 `TemplateSchedule`. **`deleteSchedule`**: the
  same read and admin checks; deleted → one `template.schedule_cleared`, 204; absent → 204, no
  event.
- **`putReminderPolicy`**: the subject readable as a member (404 `feature_not_found` or
  `initiative_not_found`); the caller the feature's lead, the initiative's owner or a team admin
  (403 `required: lead`); `everyDays` 1-90 (T114); `PutReminderPolicy` (an existing policy is
  replaced and its `last_reminded_at` kept); one `reminder.policy_set`; 200 `ReminderPolicy`.
  There is no delete route in T114's table; a policy is paused by setting `everyDays: 90`, and a
  delete route is a gap listed below.
- **`putRotation`**: team admin; every member a **current** member of the team (422
  `field: "members[n]"`); T114's ranges; `tz` loads; `last_handover_on` set to the current period's
  boundary, so setting a rotation never announces a handover; one `triage.rotation_set`; 200
  `TriageRotationView`. **`getRotation`**: any team member, viewer included; no rotation → 404
  `rotation_not_found` (**G1**: 114 appends the code); `onDuty` and `handoverOn` per decision 7,
  `handoverOn` the next boundary date, which the stored policy fixes.

**G1, completed:** T114's `TriageRotationView.OnDuty` is a `User`; decision 7's empty-rotation case
needs `*User` (`null` when every listed member has left), and the code `rotation_not_found`.

#### Tests to write first, part 1 — `server/recur_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWeeklyOccurrencesKeepLocalTime` | weekly from Monday 2026-09-28 at 09:00 `Australia/Sydney`: occurrence 0 is 23:00Z on 27 September, occurrence 1 is 22:00Z on 4 October (after the change of 2026-10-04) | a fixed offset; UTC for `tz` |
| `TestTheThirtyFirstFallsBackToTheLastDay` | monthly from 2026-08-31: occurrences on 31 August, 30 September, 31 October, 30 November | a month without the day skipped, or rolled into the next |
| `TestAGapTimeRunsAtTheFirstRealMinute` | daily at 02:30 Sydney: the occurrence on 2026-10-04 is 03:00 local (+11:00), on 2026-10-05 02:30 local | Go's unspecified choice left in; a skipped day |
| `TestDueOccurrenceCountsTheSkipped` | `after` three weekly occurrences back → the latest one, `skipped: 2`; `after` equal to the latest → not due | a burst; a run key that re-runs |

#### Tests to write first, part 2 — `server/janitor_w4_integration_test.go` (`//go:build integration`)

The cast is task 11's (Ada, Portal's admin; Grace and Linus as members of Portal's team for the
rotation; Xavier of another org). Time moves through `RunJanitorOnce(…, now)` only.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAWeeklyScheduleRunsOncePerOccurrence` | a weekly 09:00 `Australia/Sydney` schedule set on Sunday 2026-09-27 with `startsOn` 2026-09-28; a run at Monday 08:59 local creates nothing, at 09:01 one issue in `triage`, a second run at 09:02 nothing | no run key; UTC for `tz` |
| `TestSettingAScheduleNeverBackfills` | set on Wednesday 2026-09-30 with `startsOn` 2026-09-28 → runs until Monday 2026-10-05 09:00 local create nothing | a first run that backfills |
| `TestMissedWeeksMakeOneIssue` | no run for three weekly occurrences → one issue; `template.schedule_ran` `skipped: 2` | a burst of three |
| `TestAJanitorActHasANullActor` | the `issue.created` and `template.schedule_ran` events have `actor: null`; `schedule_ran.data.by` is Ada; the issue's `createdBy` is Ada | a system user as actor; an anonymous rule |
| `TestATemplateEditChangesNoRecurringIssue` | PATCH the template's title after a run → the created issue's head is unchanged; the next run uses the new title | a live link to the template |
| `TestTwoJanitorsCreateOneIssue` | two `RunJanitorOnce` at once on one due schedule → one issue; one report says `Locked: false` | no advisory lock |
| `TestAStaleFeatureGetsOneReminder` | `everyDays: 7`, last update 9 days ago → one `update.reminder`, `daysSince: 9`, `lead` FT-1's lead, `by` the policy's setter | reminders ignored or mis-addressed |
| `TestASameDayRerunRemindsNobody` | a second run an hour later → none; seven days later with no update → one more | a reminder every sweep |
| `TestAnUpdateResetsTheReminder` | Grace posts an update (task 119) on day 9 → no reminder until day 16 | the reminder clock ignoring updates |
| `TestTheRotationHandsOverAtTheBoundary` | three members, `periodDays: 7`, `startsOn` day 1: `GET` on day 7 reads member 1, on day 8 member 2 with `handoverOn` day 15; one `triage.handover` `{from: 1, to: 2}` | an off-by-one boundary |
| `TestALeaverIsSkipped` | member 2 leaves the team before day 8 → day 8 reads member 3, and the handover names member 3 | duty given to a non-member |
| `TestWhoSetsPolicies` | Grace (member) → 403 on the schedule and rotation routes, 403 on a reminder for a feature she does not lead, 200 on one she leads; Xavier → 404 everywhere | a member sets the team's policies |
| `TestEachPolicyWriteAppendsOneEvent` | each PUT → one event; a DELETE of an absent schedule → none | a write without its event |
