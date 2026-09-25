### Task 57: the replica, part 3: only what the caller may read; resync on a gap; erase

Stream **WD1**, the visibility and recovery half of seam **SW.16** (the replica), about 2.5 hours
(an estimate, not a measurement). Depends on task 55 (`createReplica`, `ReplicaDeps`, `status`,
`cursor`, `get`, `byProject`). Task 55's part file was not on the tree when this was written, so
every task 55 name below is its **scope's** words (`tasks-41-60.md:457-490`), **UNVERIFIED** until
its part lands; task 55 wins any disagreement. It also reads task 56's `resume()` (the write queue,
`task-56.md:354-355`) and task 41's hold model (`reconcile`, `task-41.md:141`).

After this task the replica holds **only** Work records of projects the caller can read, drops them
the moment that stops being true, loads a project in full the moment it starts being true, recovers
from an outage or a restored Hub without a blank screen, and keeps no text an erase removed.

**What the reader sees** (05 §5.3, §3.6):

- Removed from the *Engagement Portal* team with EP-142 open: the list empties of EP's issues in
  the same frame, and the open issue view says one fact line in place of its body,
  *You no longer have access to EP*. No dialog, no error colour: it is a fact, not a failure.
- Added to a team: its project's issues appear in the list through the hold model (task 41), so
  nothing jumps under the reader's cursor; the held line says how many arrived.
- The Hub restored from a backup, or the relay back after an outage: the rows stay as they are, and
  the view header's fact line reads *Reloading · since 11:02* in `--ink-3` (the time in mono,
  `tabular-nums`) until the load settles. No spinner, no per-row state, no count of what is left.
- An erased issue: its row and every chip or link that names it say *erased* with its number, key
  and digest intact (05 §3.6); its title is gone from search (task 58) and from memory.

**Decisions made here, so nobody re-derives them:**

1. **The readable list is the Hub's answer, never the desktop's guess.** A project's access is
   exactly its team's membership (`hub-contract.md:333`), and `GET /v1/projects` answers *own
   teams' projects only* (`:347`). The replica keeps a `scope` map `projectId → ProjectScope` from
   that list and calls task 55's `listAll(projectId)` **only** for ids in it. `admit(record)`
   refuses any record whose `projectId` is outside `scope`: a defence for the case where the relay
   or a Work kind is not filtered, since 05 §5.3 marks the Hub-side filter for Work kinds
   **UNVERIFIED** (task 13 owns it).
2. **A subject the Hub will not give back is gone.** A non-member reading a project gets 404
   (`:349`); a viewer's write gets 403. Task 55's `fetchSubjects(ids)` answers the records it could
   read; this task requires that it **omits** an id answered 403 or 404 rather than throwing, and
   throws only for a transport failure. An id asked and not answered is dropped in the same
   `transact`, and its project is re-listed once (decision 3's fallback).
3. **Membership, both ways.** `team.member_removed` and `team.member_set` carry the team as
   `teamId` and the user as `subject` (`:1140-1141`). Removed, naming the caller: every record of
   every project of that team is dropped in **one** `transact`, and `accessLost(projectId)` answers
   `{projectKey, at}` for each. Set, naming the caller: re-list, and every project new to `scope`
   gets a **full `listAll`**, because its earlier events were filtered out of this caller's feed
   (`:1103-1104`) and will never arrive. A `member_set` that only changes a role finds nothing new
   and loads nothing. **Fallback**: the contract does not say whether the feed filter reads
   membership at write time or at read time; if read time, the caller never sees its own removal.
   So any dropped subject (decision 2) and every resync also re-list, and a project missing from
   the new list is dropped exactly as a removal is (a gap, named at the end).
4. **A gap is never read from `seq`.** The org's sequence is gapless (`:1099`) but each caller's
   feed is filtered (`:1103`), so holes are normal and the replica never checks contiguity. It
   resyncs on three signals only: (a) **restored** — a page whose `next` is below the last applied
   `seq` (a Hub restored from backup): full resync, cursor taken again with `after=latest` (`:1101`)
   *before* the load, as task 55's first load does; (b) **recovered** — a `changes` call that failed
   as `unavailable` (task 37's `workProblem`, which counts `upstream_unavailable` and
   `network_unavailable`, `task-37.md:472-475`) followed by one that succeeds: resume from the last
   applied cursor, **no** full load, then call task 56's `resume()`; (c) **membership** (decision 3).
