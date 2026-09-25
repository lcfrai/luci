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

**Files:**

| Path | Change |
|---|---|
| `desktop/src/components/issue/AgentByline.tsx`, `AgentByline.test.tsx` | new: decision (f) |
| `desktop/src/components/issue/Comments.tsx` ⚑ (task 51) | an agent-kind comment renders `AgentByline` above its body; a `note` reads *note* |
| `desktop/src/components/issue/commentModel.ts` ⚑ (task 51) | `ORCH_HREF`, `routesToOwnOrchestrator(markdown)` |
| `desktop/src/components/issue/mentionPlugin.ts` ⚑ (task 51) | the *my Orchestrator* picker row; the `@me:orchestrator` input rule |
| `desktop/src/components/issue/CommentComposer.tsx` ⚑ (task 51) | sends the flag; the switch-off sentence |
| `desktop/src/api/work.ts` ⚑ (task 37) | `Comment` gains `kind` and `attribution`; `CreateCommentRequest` gains the flag |
| `control-plane/internal/hub/hubapi/work.go` ⚑ (task 12's additions) | `CreateCommentRequest.RouteToOwnOrchestrator` |
| `control-plane/internal/hub/server/work_comments.go` ⚑ (task 12) | the match check; the event field |
| `control-plane/internal/hub/hubclient/fake_work_social.go` ⚑ (task 05) | the same check and field in the fake |
| `control-plane/internal/fleet/work/mention.go`, `mention_test.go` | new: the intake of decision (d) |
| `control-plane/internal/fleet/model/event.go` ⚑ (TM-09) | one `EventKind`, `issue.mentioned` |

The scope named task 50's `editor/*` for the token; mentions are task 51's (`mentionPlugin.ts`,
`commentModel.ts`), so the token goes there. The three Hub files are this task's Hub delta, under
rule H1 (an added field and an added event key; nothing renamed). `fleet/work/` is a new Fleet
package the file map does not name (gap 1).

#### The wire, complete

```go
// hubapi/work.go ⚑ — task 12's CreateCommentRequest gains one field
	// RouteToOwnOrchestrator is true exactly when Body holds the link
	// [@me:orchestrator](panacea://orchestrator/me) (task 103). The Hub checks the match;
	// it never routes anything itself.
	RouteToOwnOrchestrator bool `json:"routeToOwnOrchestrator,omitempty"`

// OrchestratorMeHref is the one link the check looks for.
const OrchestratorMeHref = "panacea://orchestrator/me"
```

```jsonc
// POST /v1/issues/{issueId}/comments, as task 12, with the field
{ "body": { "text": "[@me:orchestrator](panacea://orchestrator/me) can you check the 8 MB guess?",
            "author": "H" },
  "mentions": [], "routeToOwnOrchestrator": true }
// 400 invalid_request {field:"routeToOwnOrchestrator"} when the flag and the link disagree
// event comment.created, task 13's data plus one key, only when true:
{ "issueId": "is-…", "kind": "comment", "routeToOwnOrchestrator": true }
// the event's actor is the writer (task 13's envelope); no text reaches data
```

```go
// fleet/model/event.go ⚑ (TM-09): one kind. Ref is the issue id; the comment id travels as
// the gate's Turn.SubjectID (task 109), so an Orchestrator's inbox shows "mentioned on EP-142".
const EventIssueMentioned EventKind = "issue.mentioned"
```

`issue.mentioned` is a non-thread event, so it coalesces with the Orchestrator's other events for
up to 60 s (TM-05 §4.2, as task 98's two kinds do). It carries no text: the Orchestrator reads the
comment with `panacea issue show`, as data.

#### `control-plane/internal/fleet/work/mention.go`, complete in shape

```go
package work

// Mentions turns a routed comment into one Orchestrator turn for its writer (decision (d)).
// It is registered with TM-11's Feed.Also, and "comment.created" joins the kinds list TM-12's
// fleetd passes to Feed.Poll. It reads only the event's metadata: never the comment's text.
type Mentions struct {
	Orchs  Orchestrators // the actor's live Orchestrator in the project, from TM-10's store
	Gate   Enqueuer      // task 109's Gate, which wraps TM-26's queue; the only way in
	Seen   Seen          // comment ids already enqueued (idempotence across a replayed page)
	Refuse Refusals      // task 109's metadata-only refusal rows
}

// Orchestrators finds the owner's running Orchestrator agent for a project.
type Orchestrators interface {
	Live(ctx context.Context, org, project string, owner model.Owner) (model.AgentID, bool, error)
}

// Seen is a set keyed by (org, comment id), in TM-10's store so a restart keeps it.
type Seen interface {
	Has(ctx context.Context, org, commentID string) (bool, error)
	Mark(ctx context.Context, org, commentID string) error
}

// Enqueuer and Refusals are task 109's types, restated so this file compiles against them.
type Enqueuer interface {
	Enqueue(ctx context.Context, t killswitch.Turn) error
}
type Refusals interface {
	Refused(ctx context.Context, t killswitch.Turn, code string)
}

// Apply is the Feed.Also consumer. Every other event returns nil untouched. The rule order:
//  1. kind "comment.created" and data.routeToOwnOrchestrator == true, else nil;
//  2. the owner is the event's actor (ev.Actor.ID), never the issue's assignee;
//  3. Seen.Has(org, subject) → nil (already enqueued or refused once);
//  4. Orchs.Live for (org, project, actor): none → Refused(t, "no_orchestrator"), Mark, nil;
//  5. Gate.Enqueue(Turn{OrgID, ProjectID, OwnerID: actor, Origin: OriginMention,
//     SubjectID: the comment id}); the gate records its own refusal while Orchestrators are
//     off; then Mark.
// An Enqueue error other than the gate's refusal is returned without Mark, so TM-11's Poll
// re-reads the page and the mention is tried again, once.
func (m *Mentions) Apply(ctx context.Context, ev hubapi.Event) error
```

A mention made while Orchestrators are off is refused **and marked**: turning them back on does
not replay old mentions into turns nobody asked for at that time. The package name `killswitch` is task 109's
(`task-109.md:84-131`; its file path is **UNVERIFIED**, step 1 re-reads it).

#### Desktop interfaces, complete

```ts
// commentModel.ts ⚑ (task 51)
export const ORCH_HREF = "panacea://orchestrator/me";
export const orchMentionMarkdown = "[@me:orchestrator](panacea://orchestrator/me)";
/** True when the markdown holds the link; the composer sends it as routeToOwnOrchestrator. */
export function routesToOwnOrchestrator(markdown: string): boolean;
/** The chip's words, relative to the writer (decision (b)). */
export function orchMentionLabel(writer: User, reader: User): string; // "your Orchestrator" | "Alice's Orchestrator"
```

```ts
// mentionPlugin.ts ⚑ (task 51): the picker's first row on team projects, and an input rule
export interface MentionPluginOptions {
  people: () => readonly User[];
  team: () => boolean;             // TM-52's strip team flag for the project
}
/** Matches exactly "@me:orchestrator" then a space or one of ,.;:!? — nothing else. */
export const ORCH_INPUT_RULE: RegExp; // /(^|\s)@me:orchestrator(?=[\s,.;:!?])$/
```

```ts
// work.ts ⚑ (task 37): the comment grows the W3 fields task 104 already declared
export interface Comment {
  /* …task 37's fields… */
  kind: CommentKind;                      // task 104's type
  attribution: AgentAttribution | null;   // non-null exactly for note, evidence_proposal, negotiation
}
export interface CreateCommentRequest {
  /* …task 37's fields… */
  routeToOwnOrchestrator?: true;          // sent only when true
}
```

```tsx
// AgentByline.tsx
export function AgentByline(props: {
  attribution: AgentAttribution;
  owner: User | null;       // looked up by attribution.ownerId; null → "former member"
  reader: User;
  role: "orchestrator" | "worker";   // from TM-52's agent view; a worker should never appear (gap 3)
  onOpenSession?: (sessionId: string) => void; // present only when the session is the reader's own
}): JSX.Element;
// renders: <span class="who">you · orchestrator (agent)</span> · claude-opus-4-8 · 3f2a91c0
// all Commit Mono at --t-fact, --ink-2; the session id a button only with onOpenSession
```
