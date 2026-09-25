### Task 13: subscriptions; the change feed's Work kinds, filtered per caller

W1, stream WH, seam **SW.5** (the Work feed kinds and their metadata-only payloads; consumed by the
replica 55, the Desk 62, Fleet 98, W4's handover 123 and W5's webhooks 156). Depends on tasks 11
and 12 (the last W1 writers before import) and Hub task H26 (the change feed, route 43,
hub-contract §7.11). Hub anchors were read in the `feat/hub` worktree at `9f2fa31`: the contract
(cited `hub-contract:N`), Hub task 14's part (`H14:N`) and Hub task 26's part
`docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-26.md` (cited `H26:N`). `server/` and
`store/` are **planned, not on `feat/hub`** at `9f2fa31`; `server/changes.go` and
`store/changes.go` are Hub task 26's files and are **UNVERIFIED until merge**.

This task does two things that belong together because both are about who hears what.

**First, it freezes the Work kinds.** Tasks 08 to 12 emitted their events under draft names. This
task moves every name into one file, `hubapi/workevents.go`, with **one Go struct per kind** for
its `data`, so the Hub's writers and the desktop's types (task 37) read one list. A reflection test
refuses any `data` field that is not a string, an `int64` or a `bool` (hub-contract:1166; `Event.Data`
holds "strings, int64s and bools only", `H14:72`), and a second test fails when the Hub writes a
Work-looking kind that the file does not declare. Drift between a writer and the desktop then fails
a test instead of silently dropping an event (consumers ignore unknown kinds, hub-contract:1135).

**Second, it adds subscriptions** (L-117, pulled into W1 by 05 §5.3 row 4): who is following an
issue. The rules are few and strict. The creator on create, the assignee on assign, each mentioned
user on mention and the commenter on comment are subscribed **in the same transaction** as the
write that caused it. A person who said `subscribed: false` stays unsubscribed: no later trigger
turns it back on. A person who leaves the team keeps the row and stops seeing it, because every
read is filtered by what the caller can read now. There is **no delivery** here: no mail, no
toast, no badge. The Desk digest is W2's (tasks 81 onward), and AGENTIC-UX §10's anti-pattern 8
(*"No more than 1 notification per user per 15 minutes, and it is a digest"*,
`docs/AGENTIC-UX.md:243`) is why.

**The one per-caller rule.** The feed already shows an event only to readers of its project or
members of its team (hub-contract:1103-1104; `H26:26-28`). A subscription is private: that Grace
muted EP-142 is nobody else's business. So `subscription.set` is shown **only to its subject**,
the user it is about. The rule is declared on the kind (`Visibility: SubjectOnly`) and applied in
the feed query as one additive predicate; W2 marks its draft kinds (`df-`) the same way. Nothing
else in the feed changes.

**The one thing this task must not get wrong** is that no text reaches the feed. A title, a
description, a comment body, an import's URL or author name: none of them may appear in any event
`data`, because the feed is the audit log filtered (`H26:7-9`) and the audit log is hash-chained, so
text written there can never be erased (task 14). The canary test writes one marked string into
every W1 text field through every W1 route and then reads every page of `/v1/changes` as every
principal.

Three decisions the scope left open, made here.

1. **Team-scoped kinds carry `teamId` and a `null` `projectId`.** The scope asks for non-null
   `teamId` and `projectId` on every Work kind, but `label.created`, `label.archived` and
   `team.work_policy_set` are about a team, not a project, exactly as the Hub's own `team.*`
   kinds are (hub-contract:1139-1144, `projectId` —). Giving them a project would be inventing
   one. So each kind declares its scope (`ScopeProject` or `ScopeTeam`); the test asserts
   `teamId` non-null on every Work event and `projectId` non-null on every `ScopeProject` kind.
   Listed as a correction to the scope in the return.
2. **A trigger never overwrites a row.** The trigger is `INSERT … ON CONFLICT (issue_id,
   user_id) DO NOTHING`: an existing row, subscribed or not, keeps its value **and its first
   cause**. Only `PUT /v1/issues/{issueId}/subscription` changes a row, with cause `manual`.
3. **A trigger that inserts a row appends a `subscription.set` event; one that finds a row appends
   nothing.** One event per changed record (hub-contract:1133). So a create now changes two records
   (the issue and the creator's subscription) and appends two events. Task 08's
   one-event-per-create test is re-read by kind in this task (see *Files*), and task 12's
   comment count already counts `comment.*` kinds only.

