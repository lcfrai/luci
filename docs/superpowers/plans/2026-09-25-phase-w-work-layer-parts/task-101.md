### Task 101: the verbs: `panacea issue show`, `panacea issue note`, `panacea evidence propose`; `panacea team` names each agent's issue

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WF (Fleet).
**dependsOn:** 100; TM-18, TM-19, TM-22. **Seams:** consumes **SW.7** (task 19's managed block)
and **SW.19** (tasks 99-100). Every Phase T path is `cp/`→`control-plane/`, `sbx/`→`sandboxd/`
(Phase T plan `:190-191`) and is **UNVERIFIED until merge** (rule H2: the owner phase's file wins).

An agent on a delegated issue needs three things it cannot do today: read the issue as data, say
one thing on it, and propose evidence for a criterion it serves. This task adds them as three verbs
on Phase T's `panacea` control wire (ST.1), served by Fleet, and makes `panacea team` name the
issue each agent works on. The verbs are **our binary** (`sandboxd/cmd/panacea`, TM-18): no agent
CLI is parsed or modified (OD-8).

**Anchors, re-read on 2026-09-25 at `feat/team` `64ab2c4`:** TM-09's verb table (`task-09.md:275-325`:
`model.Verb{Name, Classes, Local}`, `ClassMay`) and code list (`:420-455`: `CodeForbiddenClass`,
`CodeAgentNotOwner`, `CodeNotFound`, `CodeTextRefused`, `CodeRateLimited`, `CodeUnavailable`);
TM-19's dispatch (`task-19.md:84-145`: `Caller{Org, Project, Agent, Owner, Class}`, `Handler`,
`Handle(verb, h, opts...)`, which panics on a verb *"that no class may call in task 09's table"*,
`Decode[T]`); TM-22's `team` result (`task-22.md:166-245`: `TeamWorker`, `WorkerAgent`,
`TeamOrchestrator`, `OrchCard`, `Quoted{Text, Class, By, Owner}`; *"Order is fixed so fixtures
compare byte for byte"*); TM-18's CLI (`task-18.md:36-60`: four exit codes, exit `3` prints
*"team view unavailable since <t>"*, `--args '<json>'` for every verb; `:249-272`: `verb{name,
usage, build}`, `table[argv[0]]`); TM-11's `delegation.Client.Ctx(ctx, k, route)` and
`hubclient.WithDelegation` (`task-11.md:36-42`, `:460-479`); TM-41's
`fleet/instructions/orchestrator.md` (`task-41.md:39`). Phase W: task 19's golden
(`task-19.md:200-232`) and its Q3 rule that comments are read *"only as data, through `panacea
issue show` (task 101)"* (`:22-24`); task 23's done evaluator; task 100's routes and route names;
04 §4.5 (`04-internal-fit.md:355-372`) and §4.6 steps 1-2 (`:374-383`).

**Not on any branch, so UNVERIFIED:** all of Phase T's code; task 99's `hubclient.WorkAgents` and
`hubapi` W3 types (this part uses task 100's restated names); task 98's Fleet store rows for
`issue.delegated` and `task.issueId`.

Seven decisions the scope left open, made here.

**(a) Wire names are dotted, like `turn.end`.** `issue.show`, `issue.note`, `evidence.propose`.
The CLI's `table[argv[0]]` lookup (TM-18) gains one step: with two or more argv words it tries
`argv[0] + "." + argv[1]` first, so `panacea issue show EP-142` sends `issue.show`. `panacea
issue` alone is exit `2` with the usage of both sub-verbs.

**(b) What "delegated" means to Fleet.** An issue is in the caller's scope when its owner
delegated it (a stored `issue.delegated` event with `ownerId` = the caller's owner, task 98), or
one of the owner's tasks is bound to it (`task.issueId`, task 98, which the Hub checked is the
owner's). Anything else is `agent_not_owner`, one code for "not yours" and "no such issue"
(TM-09's rule). `issue.show` has a third, read-only way in: the subject issue of the Orchestrator's
current `mention` turn (task 103), so it can read the comment it was called to. The Hub holds the
write half underneath: an agent comment needs the owner to be the assignee (task 100 (c)).

**(c) Class rules live in TM-09's table.** `issue.show` is Orchestrator and worker (a worker
reads its task's issue as data); `issue.note` and `evidence.propose` are Orchestrator only. Three
rows are appended to `model.Verbs`; TM-19's dispatch then refuses a worker with
`forbidden_class` before any handler runs.

**(d) A note is ≤ 600 characters, one per issue per owner per 5 minutes.** 04 §4.5 says
*"≤ 600, attributed, rate-limited per issue"* and gives no rate; five minutes is this part's
choice, a constant (`NoteEvery`), so a chatty Orchestrator cannot flood a person's issue. A refusal
is `rate_limited` with `retryAt`. Text goes through TM-02's `teamtext.Clean` first; a refused
string is `text_refused`.

**(e) `evidence.propose` names its issue.** The scope's args are `{criterion, commit, because}`;
a criterion may be served by two issues (task 02's `serves` is many-to-many), so `issue` is
required. The criterion must be one the issue's **head revision** serves; Fleet fills `briefKey`
from that `serves` row, so the agent never types a brief key.

**(f) `issue.show` answers the key, never writes the alias.** It accepts `is-…` or the display
alias (`EP-142`); the alias is resolved with `IssueByNumber` at call time, and the result carries
`key: "issue:is-…@r"` (skeleton rule 1). Comments come back as **data**: kind, author, class,
body, inside TM-18's quoted envelope, never rendered into a managed block (05 §3.4).

**(g) `panacea team` names issues on the Orchestrator's view only.** `OrchCard` gains `issue
{key, alias, title, titleClass}` and `criterion {briefKey, criterionId}`; the title is the
issue's, `titleClass: "H"` where a person wrote it, shown instead of the task's. The worker view
is **unchanged**: TM-22 pins its keys to task 16's `team.json` snapshot, whose decode refuses an
unknown field (`task-22.md:168-171`), so one added key would break every live pod's snapshot. A
worker reads its own task's issue with `panacea issue show` and no argument.
