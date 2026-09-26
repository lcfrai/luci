### Task 155: outside PRs, part 2: linkbacks and autolinks; a PR-body magic word files an issue; copy `ep-142-slug`, the first push is `started`

W5, streams WH and WP, about 3 hours (an estimate, not a measurement). Depends on task 154 (the
ingress, `HostEvent`, `HostEvent.CreatedRef`, `apply`, `hostFact`, `ParseRefs`, `PersonMapper`,
`ProjectKeysEver`, `ResolveRef`, `task-154.md:292-321`, `:609-727`). Reads task 152's
`GitHubAppToken` and `worksync.TokenFunc` (`task-152.md:354-404`, `task-151.md:108`), task 21's
`lanes.IssueBranchName` (`task-21.md:202-236`), task 24's `workstart.DataClass`
(`task-24.md:161-165`), task 16's relay file and `Deps.Work` (`task-16.md:496-563`), task 37's
`WORK_LOCAL` (`task-37.md:301-310`), task 46's `IssueHead` (`task-46.md:554-633`), task 146's
intake create path (`task-146.md:575-629`), task 143's `OutsidePRLink` and `WordBranch`
(`task-143.md:291-311`), task 20's `expectNoPrediction` (`task-20.md:41`), and three tasks with
**no part on this tree**: 80 (`server/work_pages.go`, the `/p/` page and its browser session,
scope `tasks-61-80.md:665-700`), 59 (`palette/commands.ts`, scope `tasks-41-60.md:579`) and 145
(the `host` guard, scope `tasks-141-160.md:230-276`). Every name taken from those three is
**UNVERIFIED** until their parts exist.

Closes **L-177** (05 §1: *"Magic word in a PR body files an issue … on both hosts"*), **L-203**
(*"PR linkback comments, autolinks … Both hosts"*: GitHub in full; on Azure DevOps the linkback
only, so **PARTIAL** there, decision 5), and, with task 154, the W5 halves of **L-198** (*"For work
outside the product, copy `ep-142-slug`; the first push is the `started` fact"*), **L-199**,
**L-200** and **L-201**.

**What this task must not get wrong:** nothing the Hub writes on a customer's remote carries an
issue's text. A linkback is an alias and a link, never a title, because an *AU data only* title
must not reach a remote the Hub's erase cannot follow (`OD-24`, Q8). A branch name for an AU
project is the alias alone for the same reason. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **The Hub never calls a host inside a transaction.** `apply` claims a linkback in the database
   (`linkback_state` `NULL` → `pending`, one conditional `UPDATE`) and returns it; the ingress posts
   it after commit and records `posted` or `failed` in a second, one-statement transaction. A host
   that is slow or down therefore cannot hold a row lock or roll back a link, and two deliveries
   racing on the same PR claim each link once: the second `UPDATE` matches no row.
2. **One linkback per `(PR, issue)`, ever, and one comment per delivery.** A delivery that links
   EP-142 and EP-143 for the first time posts one comment with two lines. A later `edited` event
   that names them again claims nothing and posts nothing. A failed post is not retried: it stays
   `failed`, visible in task 172's admin list (**UNVERIFIED** that 172 shows it), because a retry
   loop that posts twice is worse than a missing comment.
3. **The linkback text is built from constants, the alias and the id link, and nothing else.**
   Each line is `Linked to EP-142 in PanaceaApp: https://<hub>/p/issues/is-…`. The link is the
   id form, not the alias form, so it survives every key change (skeleton rule 1). The builder
   takes no title parameter at all, so no future caller can pass one in by mistake.
4. **The filing word is one line, `issue: new`,** alone on a line of the PR body, any case, with
   spaces allowed around the colon (our own word, not a host's). It files one issue per PR, ever,
   recorded in `work_outside_pr_filings` whether it filed or not, so an `edited` event that still
   carries the line files nothing more and an unmatched author is told once. The issue: title the
   PR title cut to SW.1's bound, description `Filed from <PR URL>` (the PR body is not copied),
   state `triage`, `source: intake`, `not tied: support`, created by the **mapped PR author**
   (`HostPR.Author`, added here). Its PR link is `non_closing`: a filed issue is intake, and a
   person accepts it out of `triage` (05 §3.5), so a merge does not land it.
5. **Autolinks are GitHub only.** At registration, and whenever the project's current key differs
   from the key the hook last registered (`work_host_hooks.autolink_key`), the Hub sends one
   `POST /repos/{owner}/{repo}/autolinks` with prefix `EP-` and the template
   `https://<hub>/p/issues/by-alias/EP-<num>?p=pj-…`. The janitor step compares the two columns
   each tick, so a key change is caught without a hook on task 08's route; the old prefix's
   autolink is left on the repository, so an old `EP-142` in a GitHub comment still resolves.
   Azure DevOps: this part found no free-prefix autolink for repositories (**UNVERIFIED** against
   the current Azure DevOps documentation; the implementer checks on the day and, if one exists,
   raises a follow-up rather than widening this task).
6. **The by-alias page needs its project.** `GET /p/issues/by-alias/{alias}?p={projectId}`
   resolves the alias at request time through the project's current key and every earlier key
   (task 154's `ProjectKeysEver`), then answers `302` to `/p/issues/{issueId}`. A missing `p`, an
   alias that resolves to nothing, or an issue the signed-in reader cannot read is the same `404`.
   The scope's path had no project; an alias alone is ambiguous once a key is freed and reused,
   and the autolink, the only producer of these URLs, always knows the project.
7. **The first push is a created branch, parsed, never guessed.** Task 154's normalisers already
   set `CreatedRef` (GitHub `created: true`; Azure DevOps an all-zero old object id). `ParseBranch`
   reads `^([a-z][a-z0-9]{1,9})-([1-9][0-9]{0,8})(-|$)`, upper-cases the key, keeps it only if the
   project has ever had it, and resolves `KEY-n` through `ResolveRef`. An issue in `triage`,
   `backlog` or `todo` moves to `started`, cause `outside_push`, **as the mapped pusher**; one
   `OutsideCommitLink` with word `branch` records the push's head commit. An issue at or past
   `started`, a push that created no branch, a name that parses to nothing, or an alias naming no
   issue writes nothing and fails nothing. A branch deleted and pushed again while its issue was
   moved back to `todo` by a person is a new first push, and moves it again: the rule reads state,
   not history.
8. **Unmatched people get the same treatment as in task 154.** An unmatched pusher: the branch
   link, no fact, one `issue.outside_unmatched`. That event's data gains `kind` (`"pr"` or
   `"push"`) beside task 154's `{issueId, host, prNumber}`, and `prNumber` is `0` for a push. An
   unmatched PR author with `issue: new`: nothing filed, a filing row with no issue, and the
   linkback line `could not file: we cannot match your account`. Neither carries a login.
9. **The branch name is computed at the press, in the control plane, by one function.** `GET
   /v1/work/issues/{issueId}/branch-name` reads the issue through `Deps.Work.Issue` (task 16's
   re-hash included) and the class through task 24's `DataClass`, and returns
   `lanes.IssueBranchName(alias, title, auOnly)`. An error from the class, or no class wired, is
   `auOnly = true`, as task 24's Start does (`task-24.md:262`): a missing class loses a slug, never
   leaks one. The desktop never caches the answer, so a renamed issue copies its new slug.
10. **The filing and the first push write as `host`.** Both set `hub.work_actor_kind = 'host'` and
    `hub.work_actor_user` to the mapped person for their writes only, through task 154's
    `SetWorkActor` / `ClearWorkActor` pair. The filed issue's `INSERT` is not state-guarded (task
    07's `guard_issue_initial_state` admits `triage`, `task-07.md:235-240`), and task 145's `host`
    branch admits `started` with `outside_push` and a mapped person; nothing here needs a new guard.

**Files:**

- `control-plane/internal/hub/server/work_linkback.go` — `Linkback`, `LinkbackText`, the GitHub and
  Azure DevOps implementations, `Autolinker` (new; **not in the File map**: a gap)
- `control-plane/internal/hub/server/work_linkback_test.go` — pure and `httptest` rows (new)
- `control-plane/internal/hub/server/work_hooks_words.go` ⚑ (task 154) — `ParseBranch`,
  `WantsFiling`, `WordBranch` (modified)
- `control-plane/internal/hub/server/work_hooks_words_test.go` ⚑ — their rows (modified)
- `control-plane/internal/hub/server/work_hooks.go` ⚑ (task 154) — `HostPR.Author`; `apply`'s
  filing, first push and linkback claims; the post after commit; the autolink at registration
  (modified)
- `control-plane/internal/hub/store/work_hooks.go` ⚑ (task 154) — `ClaimLinkback`,
  `MarkLinkback`, `InsertFiling`, `HooksNeedingAutolink`, `SetAutolinkKey` (modified)
- `control-plane/internal/hub/server/work_pages.go` ⚑ (task 80) — the by-alias redirect (modified)
- `control-plane/internal/hub/server/janitor.go` ⚑ — the autolink step beside task 154's delivery
  expiry (modified; the file name is the Hub plan's, **UNVERIFIED**)
- `control-plane/internal/hub/server/work_hooks_integration_test.go` ⚑ — nine rows (modified)
- `control-plane/internal/api/work.go` ⚑ (task 16) — the branch-name route (modified)
- `control-plane/internal/api/work_branch_test.go` — its rows (new)
- `desktop/src/api/work.ts` ⚑ (task 37) — one `WORK_LOCAL` row and `issueBranchName` (modified;
  **not in the scope's list**: a gap)
- `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46), `IssueHead.test.tsx` ⚑ — *Copy branch
  name* (modified)
- `desktop/src/components/palette/commands.ts` ⚑ (task 59) — the same verb, no chord (modified)

**Interfaces:**

- Produces: `server.Linkback`, `server.LinkbackText`, `server.GitHubLinkback`,
  `server.ADOLinkback`, `server.Autolinker`, `server.ParseBranch`, `server.WantsFiling`; `GET
  /p/issues/by-alias/{alias}?p=`; `GET /v1/work/issues/{issueId}/branch-name` → `{"name": …}`;
  the desktop's `WorkClient.issueBranchName(issueId)`; the palette verb `issue.copyBranchName`
- Consumes: task 154's `HostEvent`, `apply`, `hostFact`, `SetWorkActor`, `ClearWorkActor`,
  `ProjectKeysEver`, `ResolveRef`, `UpsertOutsidePRLink`; task 152's `GitHubAppToken`; task 151's
  `TokenFunc` under the Hub's workload identity; task 146's intake create sequence; task 21's
  `IssueBranchName`; task 24's `DataClass`; task 20's `expectNoPrediction`
- Consumed by: task 168 (end to end), task 172 (the admin list shows `linkback_state`)
