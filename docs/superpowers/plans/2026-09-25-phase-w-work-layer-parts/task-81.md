### Task 81: the digest builder and the outbound mail seam

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**Depends on** task 13 (the change feed and its Work kinds) and task 74 (the W2 tables). **Seam:**
a new one, `mailer.Mailer`, the Hub's only outbound-mail interface.

Held to two rules of `docs/AGENTIC-UX.md`, quoted from the scope and **verified on the tree**:
*"Notifications are a digest, never a stream"* (§6 item 8, `docs/AGENTIC-UX.md:171`) and *"No more
than 1 notification per user per 15 minutes"* (§10 anti-pattern 8, `:243`). The first shapes the
builder; the second shapes the sender, and the two are enforced in different places on purpose —
a builder that batches but a sender that fires per item would still be a firehose.

**Anchors, re-read on 2026-09-27:** `task-13.md` (the feed, `Scope`, the Work kinds);
`task-72.md:872-958` (`WorkPlan`); `task-77.md:22` (Hub task 24's `RunJanitor`, `Sweep`, and the
advisory lock), `:109` (`server/janitor.go`, whose pass list this task extends);
`task-80.md`'s `/p/issues/{id}` link. **UNVERIFIED until merge:** hub-contract §12.2's
*"No variable holds a secret in any deployed environment"* at `:2151-2152` on `feat/hub` at
`48351f8` (quoted from the scope, not re-read here), Hub task 24's exact `Sweep` signature, and
`hubapi.Me.Email`.

**A gap found in task 72 and reported, not patched here.** The scope says the builder reads
`DigestSettings{email: off·hourly·daily}` and a defer record from SW.18, *"and if task 72's part
does not carry them, that is a gap to close in 72, not here"*. It does not:
`grep -n "DigestSettings\|Deferral\|DigestItem" task-72.md` returns nothing. So **task 72 must add
`DigestSettings`, `Deferral` and `DigestItem` to `hubapi` and their four methods to `WorkPlan`**,
and **task 74 must add `digest_settings` and `deferrals`** (both keyed `(org_id, user_id[, issue_id])`,
both readable and writable only by their own person, like `drafts`). This part writes their shapes
below so neither task has to invent them, and names the dependency in its first step.

#### Decisions the scope left open, made here

**(a) The builder is pure and takes no clock.** `BuildDigest(events, subs, since)` reads only its
arguments. Every time is an argument or an event's; nothing calls `time.Now()`. A pure builder is
testable to the item, and the janitor is the only thing that knows what time it is.

**(b) Read-filtering happens before the builder, not inside it.** The store hands the builder only
events the person may read, by running task 13's feed query as that person (hub-contract §7.11,
project read, not team). The builder therefore cannot leak, because it never sees the row — the
alternative, a `canRead` callback inside the builder, is one forgotten call away from a leak.

**(c) One item per issue per digest, not one per event.** Forty events on one issue collapse to
one `DigestItem` whose `cause` is the **most recent** event's and whose `at` is that event's time.
A `count` field carries how many events it stands for, so *"12 changes"* is sayable without a
list. This is AX §6 item 8, made mechanical.

**(d) The digest carries no prose, ever.** A `DigestItem` holds the issue's alias, its head title,
a cause word, a time and a link. No description, no comment body, no request text, no revision.
The reason is not only volume: mail leaves the Hub's control and the tenant's retention rules,
and a comment can hold anything a person typed. A reader who wants the text follows the link.

**(e) The rate limit is per person, held by the sender.** `digest_settings.last_sent_at` is
checked and written in the same transaction as the send, under the janitor's advisory lock, and a
send inside 15 minutes of the last is refused by the **sender**, not skipped by the builder. The
builder still builds — the desktop route reads the same digest with no limit at all, because
reading is not notifying.

**(f) Idempotency is a key, not a memory.** `Message.IdempotencyKey` is
`sha256(userID + "\x00" + since.UTC().Format(time.RFC3339) + "\x00" + until)` — two replicas that
build the same window produce the same key, and `graph.go` sends it as
`client-request-id`. Combined with the advisory lock, the second replica does not send; if it ever
did, the key is the second line of defence.

**(g) No secret in a variable, so no SMTP.** `graph.go` acquires a token from the workload
identity's federated credential and calls Microsoft Graph `POST /users/{id}/sendMail`. There is
no password, no connection string and no API key in configuration — only a sender address, a
tenant and a client id, none of them secret (hub-contract §12.2).

**(h) Mail is off unless configured.** `PANACEA_APP_HUB_MAIL_MODE` is `off` (default), `fake` or
`graph`. `off` builds digests and sends nothing; the desktop route still works, so L-116's
desktop half (task 94) does not wait on a tenant grant.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/server/work_digest.go` | new: `BuildDigest` (pure) and `GET /v1/work/digest` |
| `control-plane/internal/hub/store/work_digest.go` | new: the per-person feed read, the settings read and the send record |
| `control-plane/internal/hub/mailer/mailer.go` | new: the seam — `Mailer`, `Message`, `ErrRateLimited` |
| `control-plane/internal/hub/mailer/graph.go` | new: Microsoft Graph `sendMail` under the workload identity |
| `control-plane/internal/hub/mailer/fake.go` | new: records messages, used by every test but `graph_test.go` |
| `control-plane/internal/hub/mailer/{mailer,graph}_test.go` | new: the tests below; `graph_test.go` runs against `httptest` only |
| `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's, task 77's passes) | one digest-send step, after task 77's cycle steps, in task order |
| `control-plane/internal/hub/server/work_digest_integration_test.go` | new: the route, the read filter and the two-replica test |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ | `workMatrixRows["81"]`; `"81"` in `workMatrixLive` |
| `control-plane/cmd/hub/config.go` ⚑ | `PANACEA_APP_HUB_MAIL_MODE`, `_SENDER`, `_TENANT_ID`, `_CLIENT_ID`, `_BASE_URL` |

#### The seam (`mailer/mailer.go`, complete)

```go
// Package mailer is the Hub's only outbound-mail seam. Nothing else in the Hub
// opens a socket to a mail service; nothing here knows what a digest is.
package mailer

import (
	"context"
	"errors"
)

// ErrRateLimited is returned when a message for this recipient was sent inside
// the window (AGENTIC-UX.md:243: one notification per user per 15 minutes).
// The caller records the refusal and does not retry inside the window.
var ErrRateLimited = errors.New("mailer: rate limited")

// Message is one mail. Text is required; HTML is optional and, when present,
// carries the same items and no more (decision (d): no prose either way).
type Message struct {
	To             string // one recipient; a digest is per person
	Subject        string
	Text           string
	HTML           string
	IdempotencyKey string // decision (f); sent as client-request-id
}

// Mailer sends one message. Every implementation is safe for concurrent use.
// Send is expected to be slow and is never called while a transaction is open.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}
```

#### The types this task needs, which task 72 must add (complete, so it need not invent them)

```go
// hubapi — reported to task 72 as a gap; written here so the shapes agree.

type DigestEmail string // "off" | "hourly" | "daily"

type DigestSettings struct {
	UserID     string      `json:"userId"`
	Email      DigestEmail `json:"email"`      // default "off" (decision (h))
	ViewIDs    []string    `json:"viewIds"`    // subscribed views, L-96
	LastSentAt *time.Time  `json:"lastSentAt"` // decision (e); server-written
}

// Deferral is "remind me about this later" (05 §3.7). Its own person's only.
type Deferral struct {
	IssueID string    `json:"issueId"`
	UntilAt time.Time `json:"untilAt"`
	Note    string    `json:"note"`
}

type DigestCause string // "assigned" | "mentioned" | "changed" | "due_soon" |
                        // "deferred_due" | "view_changed" | "blocked"

type DigestItem struct {
	Subject string      `json:"subject"` // "is-…"; the thread, not the event
	Alias   string      `json:"alias"`   // "EP-142"
	Title   string      `json:"title"`   // the head's title, or "erased"
	Cause   DigestCause `json:"cause"`   // the most recent event's (decision (c))
	At      time.Time   `json:"at"`
	Count   int         `json:"count"`   // events this item stands for
	Link    string      `json:"link"`    // task 80's https page link
}

type Digest struct {
	UserID string       `json:"userId"`
	Since  time.Time    `json:"since"`
	Until  time.Time    `json:"until"`
	Items  []DigestItem `json:"items"` // cause order, then At descending
}
```

#### The builder (`server/work_digest.go`, complete in its signature and its rules)

```go
// Subscriptions is everything the builder needs about one person, read by the
// store before the call. It holds no reader: decision (b) filtered already.
type Subscriptions struct {
	UserID    string
	IssueIDs  map[string]hubapi.DigestCause // subscribed issues -> why they care
	ViewIDs   map[string]string             // viewId -> name, L-96
	ViewHits  map[string][]string           // viewId -> issue ids that entered it
	DueSoon   map[string]time.Time          // issueId -> dueOn, within the window, L-37
	Deferred  map[string]time.Time          // issueId -> untilAt, now past
	Titles    map[string]string             // issueId -> head title, or "erased"
	Aliases   map[string]string             // issueId -> "EP-142"
	LinkBase  string                        // PANACEA_APP_HUB_MAIL_BASE_URL
}

// BuildDigest is pure: same arguments, same Digest, no clock, no I/O.
// Rules:
//  1. events are already read-filtered (decision (b)); the builder trusts them.
//  2. one item per issue (decision (c)): the newest event fixes Cause and At,
//     Count is how many events collapsed into it.
//  3. an issue in subs.DueSoon or subs.Deferred with no events still gets an
//     item, cause "due_soon" or "deferred_due" (L-37, 05 §3.7).
//  4. an issue that is only in a subscribed view gets cause "view_changed".
//  5. Title comes from subs.Titles, never from an event's data (decision (d)).
//  6. Link is subs.LinkBase + "/p/issues/" + issueID.
//  7. items sort by cause rank (assigned, mentioned, blocked, due_soon,
//     deferred_due, changed, view_changed) then At descending; the order is the
//     reading order and is asserted.
//  8. an event whose issue is in none of the maps is dropped, not guessed.
func BuildDigest(events []hubapi.Event, subs Subscriptions, since, until time.Time) hubapi.Digest
```

**The route.** `GET /v1/work/digest?since=<RFC3339>` (member; a person reads only their own — the
subject is the token's user and no `userId` parameter exists, so there is nothing to tamper with).
`since` absent means the last 24 hours; `since` more than 30 days back is `422 since_too_old`.
200 with `Digest`. It writes **no event and no `last_sent_at`**: reading is not notifying
(decision (e)).

#### The store (signatures complete)

```go
// store/work_digest.go
// ReadableWorkEvents runs task 13's feed query as this person, so RLS and the
// project-read rule do the filtering (hub-contract §7.11, decision (b)).
func (s *Store) ReadableWorkEvents(ctx context.Context, userID string, since, until time.Time) ([]hubapi.Event, error)
func (s *Store) SubscriptionsFor(ctx context.Context, userID string, since, until time.Time) (server.Subscriptions, error)
func (s *Store) DigestRecipients(ctx context.Context, mode hubapi.DigestEmail, before time.Time) ([]string, error)
// MarkDigestSent writes last_sent_at inside the caller's transaction and
// returns false when another replica already wrote a later one (decision (e)).
func (s *Store) MarkDigestSent(ctx context.Context, tx pgx.Tx, userID string, at time.Time, window time.Duration) (bool, error)
```

#### The janitor step (`server/janitor.go` ⚑), complete

```go
// Pass 10, after task 77's cycle passes, inside the same advisory lock the
// Hub's RunJanitor already holds (task 77:22). One pass, all recipients.
func (s *Server) sweepDigests(ctx context.Context, now time.Time, rep *SweepReport) error {
	if s.mail == nil { // MAIL_MODE=off (decision (h))
		return nil
	}
	for _, mode := range []hubapi.DigestEmail{"hourly", "daily"} {
		due := now.Add(-time.Hour)
		if mode == "daily" {
			due = now.Add(-24 * time.Hour)
		}
		users, err := s.store.DigestRecipients(ctx, mode, due)
		if err != nil { return err }
		for _, u := range users {
			sent, err := s.sendOneDigest(ctx, u, due, now)   // builds, marks, sends
			switch {
			case errors.Is(err, mailer.ErrRateLimited):
				rep.DigestsRateLimited++
			case err != nil:
				rep.DigestsFailed++                          // logged, never fatal
			case sent:
				rep.DigestsSent++
			default:
				rep.DigestsEmpty++                           // nothing to say, no mail
			}
		}
	}
	return nil
}
```

`sendOneDigest` marks first and sends second, inside one transaction for the mark and outside it
for the send: `MarkDigestSent` returning `false` means another replica won, and this one returns
without sending. An empty `Digest.Items` sends nothing at all — a mail that says *nothing changed*
is a toast with extra steps.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑), complete

One route, and its subject is the token, so every principal that can hold a token reads **their
own** digest. The viewer reads one too (a viewer can be mentioned); the sync principal is 403,
because a machine has no inbox.

```go
	"81": {
		workRow{"GET /v1/work/digest", row{name: "GET /v1/work/digest", method: "GET",
			path: "/v1/work/digest?since=2026-09-01T00:00:00Z",
			want: [6]int{200, 200, 200, 200, 200, 403}}},
	},
```

The other-team member's 200 is the point of the read filter: they get a digest with **no items**,
not a 403, and `TestADigestNeverCrossesAProjectRead` asserts the body is empty.

#### Tests to write first, each with the mutant it kills

`server/work_digest_test.go` is a unit file, no build tag: `BuildDigest` is pure (decision (a)).
The rest carry `//go:build integration`. `mailer/graph_test.go` runs against `httptest` and never
reaches Microsoft.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFortyEventsAreOneItem` (unit) | 40 events on one issue → `len(Items) == 1`, `Count == 40`, `Cause` the newest event's, `At` its time | one item per event — the firehose AX §6.8 forbids |
| `TestItemsSortInReadingOrder` (unit) | items of every cause → assigned, mentioned, blocked, due_soon, deferred_due, changed, view_changed, each group newest first | an order that buries what is addressed to you |
| `TestADueIssueWithNoEventsStillAppears` (unit) | an issue in `DueSoon` and in no event → one item, cause `due_soon`; the same for `Deferred` | a digest that only reports change, so a due date passes in silence (L-37) |
| `TestADigestCarriesNoProse` (unit) | events whose `data` holds a description, a comment body and a request body → no item field contains any of the three; `Title` comes from `subs.Titles` | a description rendered into mail (decision (d)) |
| `TestAnUnknownIssueIsDropped` (unit) | an event for an issue in none of the maps → no item, no panic | a guessed title, or a nil map read |
| `TestTheLinkIsThePageLink` (unit) | `Link` is `base + "/p/issues/" + id` for every item | a `panacea://` link in mail, which a browser cannot open (task 80) |
| `TestADigestNeverCrossesAProjectRead` | Vera, in another project, subscribed to nothing → `GET /v1/work/digest` is 200 with no items, while Linus's holds four | filtering by team instead of by project read (§7.11) |
| `TestTheRouteReadsOnlyYourOwn` | no `userId` parameter exists; `?userId=` is ignored; the body's `userId` is the token's | a digest addressable by id |
| `TestSinceTooOldIsRefused` | `since` 31 days back → `422 since_too_old`; 29 days → 200 | an unbounded feed scan |
| `TestASecondSendInsideTheWindowIsRefused` | `fake` mailer, two sweeps 5 minutes apart → one message; 16 minutes apart → two; two **different** people in one sweep → two messages | a limit that is per issue, or per process, instead of per person (`:243`) |
| `TestTwoReplicasSendOnce` | two `sweepDigests` goroutines against one database → exactly one message, one `last_sent_at`, and the loser reports neither sent nor failed | both replicas sending under the advisory lock |
| `TestAnEmptyDigestSendsNothing` | a person with no changes → zero messages, `DigestsEmpty` incremented | a nightly "nothing happened" mail |
| `TestTheIdempotencyKeyIsStable` | two builds of the same window for one person → the same key; a different window or person → a different key | a random key, which defeats the second line of defence |
| `TestGraphSendsUnderTheIdentity` | `httptest` server asserts `POST /users/{sender}/sendMail`, `Authorization: Bearer` from the token source, `client-request-id` the idempotency key, and a JSON body with one recipient | a secret in configuration; a missing idempotency header |
| `TestMailOffSendsNothing` | `MAIL_MODE` unset → `s.mail` nil, the sweep is a no-op, and `GET /v1/work/digest` still answers | mail wired on by default in a tenant with no grant |

#### Steps

- [ ] **Step 1: Close the task 72 and 74 gaps first.** `DigestSettings`, `Deferral`, `DigestItem`
  and `Digest` in `hubapi`; `digest_settings` and `deferrals` in the migration with the draft
  policy's shape. They are those tasks' commits, not this one's; if they have not landed, this
  task waits rather than writing them in the wrong file.
- [ ] **Step 2: The builder's six unit tests**, then `BuildDigest`. It is the whole risk of the
  task and it needs no database.
- [ ] **Step 3: The seam.** `mailer.go`, then `fake.go`, then the rate-limit test against the fake.
- [ ] **Step 4: The store and the route**, then the three route tests and the `"81"` matrix row.
- [ ] **Step 5: The janitor step** and the two-replica test under the advisory lock.
- [ ] **Step 6: `graph.go`** last, against `httptest` only, and the config keys.
- [ ] **Step 7: Commit by pathspec**, naming `server/janitor.go` and `cmd/hub/config.go` as
  touched and recording the open question of *Does not claim* in the message.

#### Acceptance

```bash
cd control-plane
go test ./internal/hub/server/ -race -count=1 -v \
  -run '^Test(FortyEventsAreOneItem|ItemsSortInReadingOrder|ADueIssueWithNoEventsStillAppears|ADigestCarriesNoProse|AnUnknownIssueIsDropped|TheLinkIsThePageLink)$' \
  2>&1 | grep -c '^--- PASS'                                                       # 6
go test ./internal/hub/mailer/ -race -count=1 -v 2>&1 | grep -c '^--- PASS'        # 3
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(ADigestNeverCrossesAProjectRead|TheRouteReadsOnlyYourOwn|SinceTooOldIsRefused|ASecondSendInsideTheWindowIsRefused|TwoReplicasSendOnce|AnEmptyDigestSendsNothing|TheIdempotencyKeyIsStable|MailOffSendsNothing)$' \
  2>&1 | grep -c '^--- PASS'                                                       # 8
go test ./internal/hub/... -tags integration -race -count=1                        # every package ok
grep -rn "time.Now()" internal/hub/server/work_digest.go | wc -l                   # 0 (decision (a))
grep -rniE "password|secret|api[_-]?key" internal/hub/mailer/*.go cmd/hub/config.go | grep -vc MAIL_MODE   # 0
grep -rn "smtp" internal/hub --include=*.go | wc -l                                # 0
```

Expected: `6`, `3`, `8`, every package `ok`, `0`, `0`, `0`. None has been run; no file exists yet
(OD-27). The three `0`s are the point: a pure builder, no secret, no SMTP.

#### What this task does not claim

- **Not that mail can be sent in any tenant.** Graph `Mail.Send` on the Hub's workload identity is
  a **tenant-admin grant nobody has made** (**UNVERIFIED**). `MAIL_MODE=off` is the default, and
  every test runs against the fake or `httptest`. A green suite here proves the code, not the
  permission.
- **An open question for the owner: may an issue title leave the Hub in mail at all?** The digest
  carries the head's title, which is business content leaving the tenant's application boundary
  for its mail estate. There is **no owner ruling**. If the answer is no, one constant changes:
  the item renders as its alias alone, and the tests that assert `Title` change with it. The
  question is recorded here, and in the commit message, rather than decided.
- **Not inbound mail** (task 149): nothing here reads a mailbox, and no address in `Message` is
  ever replied to. The `From` is a no-reply sender.
- **Not Teams** (task 165): L-116's Teams half is that task's.
- **Not the desktop digest** (task 94), which reads `GET /v1/work/digest` and owns what a person
  sees. L-116's email half is closed **jointly** with 94, not here.
- **Not a measured sweep.** One pass reads every hourly recipient in a loop. Nobody has run it
  against more than a handful of people, and no throughput is claimed.
- **Not per-view subscriptions as a route.** `DigestSettings.ViewIDs` is read here and written by
  task 72's method; this task adds no write route of its own.

#### Corrections to the scope

1. **Task 72 carries neither `DigestSettings` nor the defer record** (verified: `grep -n
   "DigestSettings\|Deferral\|DigestItem"` over `task-72.md` returns nothing). The scope
   anticipated this and sent it back to 72. **Reported as a gap in task 72**, with the shapes
   written above, and **in task 74**, which needs two more tables.
2. **The builder takes four arguments, not three.** The scope's `BuildDigest(events, subs, since)`
   cannot set `Digest.Until` or bound a `due_soon` window without `until`; a pure function may not
   read the clock (decision (a)).
3. **`DigestItem` gains `Count`**, so *"12 changes"* is sayable without listing twelve events —
   the mechanical form of AX §6 item 8.
4. **The rate limit lives in the sender, not the builder** (decision (e)), so the desktop route
   is not rate-limited. Reading is not notifying, and the scope's *"refused by the sender"* is
   read strictly.
5. **The matrix file is task 07's** `work_matrix_rows_integration_test.go`, not
   `server/matrix_integration_test.go` (task 75's correction).
6. **Five config keys and a `MAIL_MODE` of `off|fake|graph`**, which the scope's
   `PANACEA_APP_HUB_MAIL_*` implied but did not enumerate.

**Parity rows served:** **L-116**'s email half, closed jointly with task 94 (its Teams half is
task 165). It **serves** **L-37** (due soon), **L-96** (a subscribed view) and **L-117**, each
closed by tasks 89, 91 and 94 with W1's task 13. Task 97 records them.
