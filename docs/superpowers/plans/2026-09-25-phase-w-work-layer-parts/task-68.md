### Task 68: create an issue from any list, a Desk row, a span or a hunk

Stream **WD2**, about 2.5 hours (an estimate, not a measurement). Depends on task 46 (the issue
view that opens the new issue, and the `Popover`-based pickers it reuses) and task 53 (the `C`
binding, `issue.create`, already in the key map at `task-53.md:178`), and through SW.16 on task 55
and 56 (`mutate`). Its entry points on task 60's peek and on phase 3's span and diff surfaces are
added only where those files exist on the sync base.

After this task `C` on a focused list opens a small composer beside the row, never a modal over
the work. Enter creates the issue where the team's policy says, with the text marked as mine, and
the new issue remembers where it came from: the Desk row's issue as a `relates` relation, or the
span or hunk as its origin.

**Decisions made here, so nobody re-derives them:**

1. **The key is already bound.** Task 53 maps `C` to `issue.create` in the `list` scope
   (`task-53.md:178`) and dispatches only from a list target (`task-53.md:500-511`). This task
   registers the verb's runner with `registerVerb("issue.create", …)` while a list is mounted and
   edits no key map row. So `C` in a text field types `c`, and in a terminal reaches xterm, by task
   53's rule (05 §4.3 rule 1), and test 1 proves it through the real dispatcher.
2. **One op kind joins SW.16.** Task 56's `WorkOp` has no create (`task-56.md:156-168`). This task
   adds `{ kind: "createIssue"; localId; projectId; req }`, shaped like `comment` (a `localId`
   names the painted row until the Hub answers, `task-56.md:165`, `:304`). `fieldsOf` is `[]`,
   `undoable` is `false` (a created issue is withdrawn by moving it to `canceled`, not by undo),
   `applyLocal` paints a ghost head, and a refusal removes it: the composer, not a row, shows the
   Hub's reason.
3. **The state is the team's, never ours.** The request sends `state: ""`, which the Hub reads as
   the team policy's `defaultState` (`task-37.md:181-182`); the ghost row paints
   `TeamPolicy.defaultState` from the replica (`task-37.md:143-146`) and the Hub's answer replaces
   it. Intake that is not a member lands in `triage` by the Hub's rule, which this task does not
   restate.
4. **The text is mine and says so.** Title and description go as `AuthoredText` with
   `author: "H"` (`task-37.md:112`), shown as `text·H` by task 46's `AuthorClass` (L-124). A
   span's or hunk's words are never pasted into the description: the origin is a reference, and
   the issue view renders it from the record it names.
5. **Serving is optional and one key away.** `T` in the composer opens task 42's
   `CriterionPicker` (the same picker Triage uses). With no criterion picked the composer shows
   `not tied · chore` as a visible, changeable choice (the request needs a reason when `serves` is
   empty, `task-37.md:184-185`); it is never a hidden default, and `N` cycles the four reasons.
6. **The link to the origin.** From a Desk row about an issue: one `relates` relation to that
   issue, sent as a second op after the create stands (task 56's `relation` op), so a refused
   relation never un-creates the issue. From a span or a hunk: `origin: { kind, ref }` on the
   create, where `ref` is task 60's own anchor ref (`session:<id>@<at>`, `task-60.md:123`; or
   `hunkRef(sha, h)`, `task-60.md:190`).
7. **`origin` is an added field** (WS0's rule): SW.1's `CreateIssueRequest` has no origin
   (`task-03.md`; `task-37.md:179-186`). This task adds `IssueOrigin` to `hubapi`, `origin` to
   the request and to `IssueHead` (set once at create, never patched), and the fake stores it
   verbatim. The Hub's store column and route are a WH task not in the skeleton (a reported gap);
   until it lands the real Hub drops the unknown field and the issue view says *origin not
   recorded*, never a pretend link.
8. **Beside the row.** The composer is `Popover` (`desktop/src/components/common/Popover.tsx:10`,
   `{anchor, onClose, children}`) anchored to the focused row's rect, so the list stays visible
   and does not scroll. `Esc` closes and keeps nothing; Enter with an empty title does nothing
   and says *A title is needed.* in the composer.

**Files:**
- `desktop/src/components/work/NewIssue.tsx` (+ `NewIssue.test.tsx`) — the composer and
  `useNewIssueVerb` (new)
- `desktop/src/work/replica/ops.ts` ⚑ (task 56) — the `createIssue` op (modified)
- `desktop/src/api/work.ts` ⚑ (task 37) — `IssueOrigin`, `CreateIssueRequest.origin`,
  `IssueHead.origin` (modified)
- `control-plane/internal/hub/hubapi/work.go` ⚑, `hubclient/fake_work.go` ⚑ — the added type and
  fields; the fake keeps `origin` (modified)
- `desktop/src/components/palette/commands.ts` ⚑ — *New issue* with its chord (after task 59;
  **no part for task 59 on the tree** at this writing, so its row shape is **UNVERIFIED**)
- `desktop/src/components/work/Peek.tsx` ⚑ (task 60) — `onCreateIssue` beside `onComment`
- `desktop/src/components/lens/files/SpanDrawer.tsx`, `PathDiff.tsx` ⚑ (P3-21) — one verb each,
  **only if on the sync base** (**UNVERIFIED until merge**)
- `desktop/src/components/issue/IssueOriginLine.tsx` — the origin line in the issue view (new;
  **not in the skeleton's file map**), mounted by task 46's `IssueHead` ⚑
- `desktop/src/components/desk/deskRows.tsx` ⚑ — nothing: Desk rows reach `C` through task 62's
  wrapper, which carries `data-issue-id` (`task-62.md:389`); the runner reads it (decision 6)

Task 59's palette row, task 60's peek and P3-21's surfaces call the same `openNewIssue(from)`;
none of them builds a request.

#### Interfaces — complete: the Hub delta (an added type and two added fields; WS0's rule)

**The Hub half is a gap.** The fake stores `origin`; the Hub's store column, its migration and the
route's decode are a WH task that is not in the skeleton's table (decision 7). Until it lands the
real Hub ignores the field and every head reads `origin: null`.

```go
// control-plane/internal/hub/hubapi/work.go ⚑ — appended (task 68; WS0: an added type and fields)

// IssueOrigin names the record an issue was created from (AX §8.2). Ref is the
// anchor ref the desktop already uses for a comment on that record
// ("session:<id>@<at>" for a span, "commit:<sha>:<path>:<oldStart>" for a hunk,
// task-60.md:123 and :189-191). The
// Hub keeps it verbatim and never reads the record it names.
type IssueOrigin struct {
	Kind string `json:"kind"` // "span" | "hunk"
	Ref  string `json:"ref"`  // 1..512 bytes
}

// ValidIssueOrigin is the create route's check; nil is valid (no origin).
func ValidIssueOrigin(o *IssueOrigin) bool {
	if o == nil {
		return true
	}
	return (o.Kind == "span" || o.Kind == "hunk") && o.Ref != "" && len(o.Ref) <= 512
}

// In type CreateIssueRequest, after NotTiedReason — absent or null is no origin:
//	Origin *IssueOrigin `json:"origin,omitempty"`

// In type IssueHead, after External — set once at create, never patched:
//	Origin *IssueOrigin `json:"origin"`
```

```go
// control-plane/internal/hub/hubclient/fake_work.go ⚑ — task 04's CreateIssue, two edits

	// in the validation switch, before the notTiedReason 422 (task-04.md:494-506):
	case !hubapi.ValidIssueOrigin(req.Origin):
		return zero, badRequest("origin", "An origin is a span or a hunk with its ref.")

	// after the fakeIssue literal (task-04.md:519-522), a copy so the caller's
	// pointer never aliases the stored head:
	if req.Origin != nil {
		o := *req.Origin
		is.head.Origin = &o
	}
```

`PatchIssueRequest` gains nothing, so no route can change an origin after create. The fake's event
payload (`task-04.md:530-532`) is unchanged: the origin is on the head, not in `issue.created`.

#### Interfaces — complete: the desktop wire (`desktop/src/api/work.ts` ⚑, task 37)

```ts
/** Task 68. The record an issue was created from; `ref` is task 60's anchor ref. */
export interface IssueOrigin { kind: "span" | "hunk"; ref: string }

// In CreateIssueRequest (task-37.md:179-186), after notTiedReason — omitted is no origin:
//   origin?: IssueOrigin;
// In IssueHead (task-37.md:121-128), after external — null when none, or when the real Hub has
// not yet got the WH task's column:
//   origin: IssueOrigin | null;
```

Task 37's client has no head normaliser (its `relay` hands back the JSON as sent), so a head from
a Hub without the column has no `origin` key at all. Every reader in this task writes
`head.origin ?? null`, and test 8 feeds such a head.

#### Interfaces — complete: the `createIssue` op on `desktop/src/work/replica/ops.ts` ⚑ (task 56)

Task 56's queue keys every op by `op.issueId` (`task-56.md:290-291`, `:368-372`), so the op carries
the ghost's id there as well as in `localId`: they are the same `lc-` string, and the queue for a
ghost holds only its own create.

```ts
// added to the WorkOp union (task-56.md:156-168):
  /** Task 68. `issueId` === `localId`: the ghost row's id until the Hub returns its is- id.
   *  `paintState` is the team policy's defaultState as the composer read it; paint only, never sent. */
  | { kind: "createIssue"; issueId: string; localId: string; projectId: string;
      paintState: IssueState; req: CreateIssueRequest }

// added to Settled (task-56.md:284-286):
  | { created: IssueWithRevision; replacesLocal: string }

// fieldsOf: a create touches no field of an existing issue.
    case "createIssue": return [];

// undoable (task-56.md:226-230), a second early line beside the comment's:
  if (op.kind === "createIssue") return false;

// inverseOf:
    case "createIssue": return null;

// sendOp (task-56.md:290-309):
    case "createIssue":
      return { created: await client.createIssue(op.projectId, op.req), replacesLocal: op.localId };
```

```ts
// applyLocal: the create's own arm, before the field loop. It paints a ghost head and revision
// in the one transact the batch already opens (task-56.md:368), and returns empty snapshots.
if (op.kind === "createIssue") {
  view.transact((w) => { w.putHead(ghostHead(view, op)); w.putRevision(ghostRevision(view, op)); });
  return { before: new Map(), after: new Map() };
}

// revertLocal: a refused create drops its ghost and nothing else. `dropIssue` is task 55's
// (`task-55.md:235`). It returns [] because no field of any real issue was written back.
if (op.kind === "createIssue") {
  view.transact((w) => w.dropIssue(op.localId));
  return [];
}

/** The painted head: the team's default state (decision 3), number 0 and alias "" (the row shows
 *  no alias until the Hub numbers it), and the person as creator. */
export function ghostHead(view: ReplicaReadView, op: Extract<WorkOp, { kind: "createIssue" }>): IssueHead {
  const now = new Date().toISOString();
  return {
    id: op.localId, projectId: op.projectId, number: 0, alias: "",
    state: op.req.state || op.paintState,
    stateReason: "", proof: "", landCommit: "", assignee: null,
    priority: op.req.priority || "none", labels: [...op.req.labels], relations: [],
    headRevision: 1, headKey: "", source: "person", external: null, origin: op.req.origin ?? null,
    threadState: op.req.serves.length > 0
      ? { kind: "on_thread", reason: "" } : { kind: "not_tied", reason: op.req.notTiedReason },
    createdAt: now, createdBy: view.me, updatedAt: now,
  };
}

export function ghostRevision(view: ReplicaReadView, op: Extract<WorkOp, { kind: "createIssue" }>): IssueRevision {
  return {
    issueId: op.localId, revision: 1, key: "", digest: "" as IssueRevision["digest"], projectId: op.projectId,
    content: { title: op.req.title, description: op.req.description,
               serves: [...op.req.serves], implements: [...op.req.implements] },
    writtenAt: new Date().toISOString(), writtenBy: view.me, restoredFrom: null, erasedAt: null,
  };
}
```

**Where `paintState` comes from.** Task 55's replica holds no team policy (its `ReplicaWriter`,
`task-55.md:227-236`, has no policy record), so decision 3's "from the replica" is corrected here:
the composer calls task 37's `teamPolicy(teamId)` (`task-37.md:339`) when it opens and shows the
answer as a fact, *lands in Backlog*. If that read fails the composer says *lands where the team's
policy says* and paints `triage`; the Hub's answer replaces the ghost within one round trip, so
the paint can be wrong for that long and never persists. `proof: ""` and `source: "person"` are
task 37's own literals (`task-37.md:102`, `:97`).

**On success** (`task-56.md:375-377`, step 4), the replica's success arm gains one case: for
`{created, replacesLocal}` it runs, in one `transact`, `dropIssue(replacesLocal)`,
`putHead(created.head)` and `putRevision(created.revision)`. Task 55 prints no rename notice, so a
ghost that the person selected (task 44's `selectIssues`) is dropped from the selection with it;
test 9 pins that. The `MutateHandle`'s `settled` then resolves `"applied"`, and the composer opens
the new issue with the cockpit's `openIssue(id)` (the store action task 40 reads,
`task-40.md:660`). `settled` carries no record, so `ReplicaWrites` (`task-56.md:345-352`) gains one
member, filled by the success arm and never cleared within a session:

```ts
  /** Task 68. The Hub's is- id for a ghost this replica created; null while unanswered or refused. */
  createdIdOf(localId: string): string | null;
```
