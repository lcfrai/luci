### Task 103: Orchestrator notes on the issue, one voice per owner; agent comments name owner, agent, model, session; `@me:orchestrator`

**Slice:** W3. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WD3
(desktop) with one Fleet file. **dependsOn:** 100. **Uses** SW.19 (the agent comment kinds and
attribution, tasks 99-100) and SW.5 (the change feed, task 13). Phase T paths are **UNVERIFIED
until merge** (rule H2).

An Orchestrator's note (task 101's `panacea issue note`) lands on the issue as a comment. This task
draws it so nobody mistakes it for a person: the owner, the agent, the model and the session, in
mono, on every agent comment, with the `text·A` mark. And it gives a person one way to call an
Orchestrator from a comment: `@me:orchestrator`, which reaches **the writer's own** Orchestrator
and nobody else's (OD-16).

**Anchors, re-read on 2026-09-25.** 05-design: §2.1 point 5 (`:549-550`: *"only the Orchestrator
writes notes on the issue … Five workers do not produce five comment streams"*), the matrix rows
L-156 (`:318`: *"`@me:orchestrator` in a comment routes to **your own** Orchestrator only"*) and
L-173 (`:335`: *"Every agent comment names owner, agent, model and session; team mode always shows
the owner"*). 02's `who.label` (`02-linear-design-language.md:321`: *"`alice · orchestrator
(agent)` … the label is the disclosure"*). 04 §4.5 (`04-internal-fit.md:355-372`: workers
*"`panacea report` … to their own Orchestrator"*). Phase W: task 51's mention rule and files
(`task-51.md:28-34`: a mention is the link `[@Priya](panacea://user/u-…)`, `mentions` derived from
it; `:58-60`: an `A` body renders `<AuthorClass author="A"/>` and *text·A · agent*; `:66-80`:
`commentModel.ts`, `mentionPlugin.ts`, `CommentComposer.tsx`, `Comments.tsx`); task 12's
`CreateCommentRequest` and `MaxMentions` (`task-12.md:141`); task 100's agent comment wire and
guards (decisions (b), (c)); task 101's `issue.note` and its `InTurn` read grant (decision (b));
task 104's TypeScript `AgentAttribution` (`task-104.md:358-362`); task 109's `Gate` and
`OriginMention` (`task-109.md:100-140`). Phase T at `64ab2c4`: TM-09's `Event` and `Turn`
(`task-09.md:240-270`), TM-11's per-org feed reader (`task-11.md:36-42`), TM-26's turn queue
(Phase T plan `:337`, via task 109).

**Not on any branch, so UNVERIFIED:** everything of Phase T; task 99's types; task 109's gate code.

Six decisions the scope left open, made here.

**(a) `@me:orchestrator` is a link, like every mention.** Task 51 makes a mention a link in the
text. So the token is `[@me:orchestrator](panacea://orchestrator/me)`, inserted by task 51's `@`
picker (one extra first row, *"my Orchestrator"*, on team projects only) or by typing the literal
`@me:orchestrator` followed by a space or punctuation (an input rule). **`@dylan:orchestrator` has
no rule and no picker row**: it stays plain text and routes nowhere, so no person can address
another person's Orchestrator by typing its name.

**(b) The link is relative to the writer, and so is its chip.** `me` is the comment's author, not
the reader. Dylan reading Alice's comment sees *"@Alice's Orchestrator"*; Alice sees *"@your
Orchestrator"*. The stored text never names an Orchestrator's owner, so it cannot be edited into
naming another one.

**(c) The route is one added field and one event field.** Task 12's `CreateCommentRequest` gains
`routeToOwnOrchestrator: bool` (an added field; SW.2's freeze allows it, and task 99 has not added
it, per its scope). The Hub checks it matches the text: `true` needs the link in the body, and the
link in the body needs `true` (400 `invalid_request`, `field: routeToOwnOrchestrator`). The flag is
not stored in a column: the body holds the link, and task 13's `comment.created` data gains
`routeToOwnOrchestrator: true` (metadata only; absent when false). No migration.

**(d) Fleet enqueues for the writer's Orchestrator, once.** A new Fleet intake reads
`comment.created` with the flag from TM-11's per-org feed and enqueues one turn for the **actor's**
Orchestrator, under the actor's own delegation, through task 109's `Gate` with
`OriginMention` and `SubjectID` = the comment id. The issue's assignee is never consulted. It is
idempotent by comment id (a replayed feed enqueues nothing twice). With no live Orchestrator
delegation for the actor in that project, nothing is enqueued and a metadata-only refusal is
recorded (`no_orchestrator`).

**(e) The Orchestrator answers where it may.** On the writer's **own** issue it may write a note
(task 100's assignee rule). On **another person's** issue it may read (task 101's `InTurn` grant)
but not write: the Hub refuses an agent comment on an issue its owner is not assigned. So Alice's
Orchestrator, called from Dylan's issue, answers Alice in her Orchestrator panel (TM-55), never on
Dylan's issue. One voice per owner, and the owner's voice stays on the owner's work.

**(f) The byline is a fact line, not a name tag.** `AgentByline` renders, in Commit Mono at
`--t-fact`: `dylan · orchestrator (agent)` (02 `who.label`), then ` · ` and the model as the
session record states it, then ` · ` and the session id's first 8 characters. The id is a link that
opens the session only when it is the reader's own (their Orchestrator's); for a teammate's agent
it is text, with the full id in `title`. The owner is always shown, as L-173 requires, even when
the reader is the owner (`you · orchestrator (agent)`).
