### Task 133: *since you last looked* names issues: four delta classes, a handover that links every claim

W4, stream WD, AX §6.2-6.5 and 05 §2.5's example (*"EP-142 landed; EP-150 waits on your decision
since 23:40"*). Depends on task 123 (read cursors and the scoped `/v1/changes`, through task 125's
`WorkClient` members) and on Phase T's TM-54 (its rows share the band). Estimate 0.375 (3 h).
Closes **L-118** (with task 123) and **L-119** without audio.

After this task the Desk's *SINCE YOU LAST LOOKED* band says, in product words, what changed on the
issues that involve the person since they last looked: what landed, what waits on them and for how
long, what failed, and what was decided without asking them. Four lists, never merged. Every
sentence is built from fixed templates with no model call, and every claim in it is a link to the
event that makes it true. Opening an issue from the band moves only that issue's cursor and lands
its activity on the first event the person has not seen.

**Decisions made here, so nobody re-derives them:**

1. **Involvement is the filter, before any class.** An event is in scope only when its issue is
   one the person is assigned, reviews, is subscribed to (task 13), or is named in (a mention in a
   comment or a question addressed to them). Another member's issue that does not involve the
   person is absent, however busy the team is (no team firehose). A teammate's agent never appears
   by itself (TM-54's rule): its events reach the band only through an issue that involves the
   person.
2. **Four classes, never merged** (AX §6.2). `classify(events, me)` returns
   `{landed, waiting, failed, assumed}`, each a list:
   - *landed*: `issue.state` into `landed`; into `done` **with evidence** (task 10's `proof`
     `proven`); `issue.shipped` (task 71); a criterion the issue serves turning `settled`.
   - *waiting*: a question to the person; `in_review` with the person as reviewer; the person's
     issue blocking someone's running agent. Each carries its aging from the event's `at`
     (*"waiting 5h 51m"*, AX §6.7), measured on the Hub's clock against one read of `now`.
   - *failed*: a lane bound to the person's issue failing.
   - *assumed*: what was decided without asking: a DECIDED WITHOUT YOU agreement, a `done` moved by
     hand without evidence (*done, not proven*, §3.3), a criterion the issue serves drifting.
   An event that fits none is dropped, never forced into a class. A self-attested `done` is
   *assumed*, never *landed*.
3. **The handover is mechanical.** `handover(classes) → Sentence[]` fills one fixed template per
   item. The module imports no gateway client; a drafted version is task 139's. Each
   `Sentence {text, links [{ref, eventSeq}]}` carries the alias or key it names and the event it
   rests on, and the renderer turns every `ref` in `text` into a link to that event's span in the
   issue's activity (AX §6.4). A sentence with no link is a bug, and a test walks the text nodes.
4. **Time is aging, never *ago*.** Waiting items read *"waiting 5h 51m"*; landed and failed items
   read the clock time in mono (*"landed 14:02"*). Nothing reads *"ago"* (AX anti-pattern 9), so
   nothing changes meaning as the page sits.
5. **Cursors move by the person's act only.** Opening an issue from the band advances **only that
   issue's** cursor (task 125's `advanceReadCursor({scope: {kind: "issue", id}, seq})`), and the
   issue view scrolls to the first event after the old cursor with older events dimmed (AX §6.5).
   *Mark all seen* advances every project cursor in **one** `markSeen` call (task 123's one
   transaction). Rendering, scrolling and hovering move nothing.
6. **The band holds, like the Desk.** It is one of task 62's bands (`DeskBand "since"`), so it
   takes one snapshot on entry and never re-ranks while open; an event arriving while the Desk is
   open shows task 62's hold line, not a new sentence.
7. **Memory only.** Cursors and the classified events live in the band's snapshot, never in the
   replica's store or on disk (task 125 decision 3).
