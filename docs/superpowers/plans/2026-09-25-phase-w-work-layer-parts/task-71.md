### Task 71: shipped: issues in the release manifest; *shipped to production, 14:02*

**Slice:** W1b. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WD2 with
phase 7's files. **Depends on** task 22 (the `landed` move and `IssueHead.landCommit`), task 64
(the Desk's LANDED section), task 67 (the issue chip), and phase 7 tasks **17** (the release
drawer) and **19** (sessions from trailers) on the sync base. A delta on phase 7 (rule H2:
phase 7's files win, and this task waits for P7-17 and P7-19). **Seams:** SW.9 (the trailer, read
here), SW.17 (one Desk row kind).

**Shipped is derived, never a state and never stored on the issue** (05 §3.3; rule 3). The
release manifest lists the issues whose `Panacea-Issue` trailers sit in the release's shipped
range, read from the local clone the way phase 7 task 19 reads `Panacea-Session`. The issue view
and the Desk read *"shipped to production, 14:02"* from that, and nothing is written to the Hub.

**Anchors, re-read on 2026-09-26** (phase 7 at `.claude/worktrees/phase-7`, `40219eee`):
P7 `task-19.md:29-92` (`sessions.Range`, `Result`, `Reader.Read` and its three git commands),
`:108-171` (the route's *previous release* rule, `deploywatch.CurrentAt`), `:175-192` (the
`{P}/sessions` wire and its catalog line); P7 `task-12.md:52` (`{P}` is
`/v1/projects/{projectId}/deploy`); P7 `task-17.md:67-76` (`ReleaseDrawer` and its `sessions`
slot); P7 `task-13.md:68-89` (`EnvironmentView.primary`, `current`), `:94-109` (`ReleasePin`:
`state`, `briefKey`, `confirmedAt`), `:111-138` (`ReleaseRow`); the P7 plan's file map
(`2026-09-23-phase-7-deploy.md:588-601`). This branch's parts: task 21's `gitx.TrailerIssue` and
`ValidIssueTrailer` (`task-21.md:81-104`; land stamps one trailer per distinct issue on the land
commit, `:268-284`); task 03's `IssueHead.LandCommit` (`task-03.md:290`); task 65's
`registerDeskSource` and `registerDeskRow` idiom (`task-65.md:784-794`); the prototype's RELEASE
fact row (`prototype/work-layer.html:687-689`). **UNVERIFIED until merge:** every phase 7 name
(none is code on this branch), and tasks 62 and 64's snapshot members (no part on the tree).

#### Decisions made here, so nobody re-derives them

1. **One walk, two trailers.** `sessions.Reader`'s private `walk` runs `Read`'s three git
   commands once with both trailers in the format, and returns each commit's session ids and
   issue ids. `Read` (P7-19) keeps its wire byte for byte; `ReadIssues` is new and answers
   `IssuesResult`. A route reads once; the two blocks of one drawer read twice, bounded by
   `MaxCommits`.
2. **Two routes, not one.** `GET {P}/issues?deploymentId=` is the manifest block, beside
   `{P}/sessions`, with the same range rule (the one function, not a copy). `GET
   {P}/issue-shipped?issueId=&landCommit=` is the issue's own derivation: per environment, the
   earliest confirmed release whose manifest lists the issue. The scope names one route; the
   issue view cannot read a manifest per release per environment, so the second is recorded
   under *Corrections*.
3. **"Lists it" is decided by ancestry, and the two routes are tested to agree.** Task 21 stamps
   the issue's trailer on the land commit, so a release's range `(From, Commit]` holds a commit
   carrying it exactly when `landCommit` is an ancestor of `Commit` and not of `From`. The
   per-issue route checks `git merge-base --is-ancestor` per release row, and a test holds that
   the release it names is one whose `ReadIssues` lists the issue.
4. **Only a confirmed release counts.** A row counts when its build is `succeeded` and its
   `release.state` is `confirmed`; the time is the pin's `confirmedAt` (the Hub's confirm), shown
   `HH:MM` in mono. A reserved, abandoned or expired pin, or `releaseLoaded: false`, never reads as
   shipped.
5. **The environment is named, never assumed.** The line uses the environment's `name` verbatim.
   The head shows the `primary` environment's fact first, then the others by name. A release to
   `staging` reads *"shipped to staging"*, never *"production"*.
6. **A rollback is said.** When an environment's current release does not hold `landCommit`, the
   fact reads *"was on production from 14:02; not serving now"*. It is never dropped to
   *"not shipped"*, which would erase a release that happened.
7. **Ids only, never titles** (05 §3.6). The manifest carries `issueId` and commit shas; the
   drawer resolves each id to its alias and chip from the replica (task 67), and an id the
   replica cannot read shows as the bare id in mono.
