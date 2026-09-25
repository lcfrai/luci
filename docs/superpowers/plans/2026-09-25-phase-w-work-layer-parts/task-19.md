### Task 19: phase 4 delta — `Session.IssueRef`; the Materialiser's issue section; the loader's second key

W1, stream WP, seam **SW.7** (the managed block's issue section: what an agent is told, byte for
byte; consumed by Start, task 24, and W3's Delegate, task 101). Depends on task 05 (the fake's
`IssueRevisionAt`), task 18 (`issue:` keys, `Ref.Kind`, `BuildIssue`) and phase 4 tasks 09
(`contextwrite/materialise.go`), 10 (the `api.Materialiser` seam in create and restart) and 16
(the desktop loader and its key sources).

This task decides **what an agent is told about the issue it is working on**. Phase 4 already
writes one managed block into the worktree's `CLAUDE.md`: the brief, rendered deterministically,
between two markers, stamped on the session as `contextRef`. This task adds one section to that
block — the issue's title, its description and the criteria it serves — and stamps the issue key
it rendered as `Session.IssueRef`, beside `contextRef`, by the same rule: **from what the
Materialiser actually wrote, never from what the request asked for** (phase 4's P6).

**The one thing this task must not get wrong is whose words go into an agent's context.** An
issue revision's title and description each carry an author, `H` or `A` (05 §3.2; task 02's
`AuthoredText`). A revision an agent wrote is a *proposal* until a person accepts it (05 §3.4).
Rendering `text·A` into another agent's context is how one agent's hallucination becomes a second
agent's instruction. So the section renders **only** a revision whose title and description are
both `H`; anything else refuses with a new reason, `issue_not_adoptable`, and the session does not
start. Comments and requests are **never** rendered, even when a person wrote them: 05 §3.4 and
§3.5 class them as coordination and customer content, which an agent reads only as data, through
`panacea issue show` (task 101). Q3 is ruled: *"issue text goes into the agent's context, with
guards"* (owner decisions `:168`, under OD-1). The guards are this task: `H` authorship only, the
session's own Hub project only, the comment-opener neutraliser, and no comment or request text.
TM-05's injection corpus, which 05 §3.4 also waits on, is not merged; the hostile-string test
below uses a local fixture until it is.

**What is on the tree today (verified 2026-09-25).**
- **Built on this branch's base:** the wire `Session.ContextRef` (`internal/api/sessions.go:619`),
  the create request's `ContextRef` (`:706`), the stamp from the request (`:921`, which P4-10
  changes to the result); `registry.Session.ContextRef` (`internal/registry/session.go:102-104`);
  the desktop's `contextRef` in `desktop/src/store/types.ts:163` and `desktop/src/api/client.ts:97`.
- **Built on `feat/phase-4` (`.claude/worktrees/phase-4` at `f700613`):** `claudemd.Render(doc
  discover.Document) string` (`control-plane/internal/claudemd/render.go:76-110`), its unexported
  `clean` neutraliser (`:51-53`, `<!--` → `` `<!-\-` ``) and the footer line
  `"\n---\n\nBrief %s, revision %d."` (`:105-108`); `discover.Criterion{ID "c-"+8 hex, Text}`
  (`discover/content.go:163-166`) and `discover.Section.Items` (`discover/inherit.go:40-47`).
- **Planned, UNVERIFIED until merge:** P4-09's `contextwrite/` (P4 plan file map), P4-10's
  `internal/api/materialiser.go` (`task-10.md:85-135`: `MaterialiseRequest`, `MaterialiseResult`,
  `MaterialiseError`, the closed reason list), P4-07's `discoverhub/read.go` `Resolve(keys)`, and
  P4-16's `desktop/src/provenance/loader.ts` with `keySources` (`task-16.md:150-159`).

**The decision scope 19 left to this part: the section is a parameter of the renderer, not a
splice.** `Render` is merged and `clean` is unexported, so a splice in `contextwrite` would either
duplicate the neutraliser or search the rendered bytes for `"\n---\n"`, which a brief's own text
could contain. Instead `claudemd` gains `RenderWithIssue(doc, *Issue)`, and `Render(doc)` becomes
`RenderWithIssue(doc, nil)`: one line in the owner's function, byte-identical output for every
brief, which P4-05's own goldens prove unchanged.

**Files:**
- `control-plane/internal/claudemd/render.go` ⚑ (P4-05's, built) — `Issue`, `IssueServe`,
  `RenderWithIssue`; `Render` delegates
- `control-plane/internal/claudemd/render_test.go` ⚑ — the issue golden, the neutraliser rows
- `control-plane/internal/claudemd/testdata/golden-issue.md` — SW.7's bytes (new)
- `control-plane/internal/contextwrite/issue.go` and `issue_test.go` — `adoptIssue`: fetch, the
  four adoption rules, criterion text, the `claudemd.Issue` (new)
- `control-plane/internal/contextwrite/testdata/hostile-issue-strings.json` — ten hostile strings,
  until TM-05's corpus is merged (new)
- `control-plane/internal/contextwrite/materialise.go` ⚑ (P4-09's) — calls `adoptIssue` when
  `req.IssueKey != ""`, renders through `RenderWithIssue`, returns `IssueRef`
- `control-plane/internal/api/materialiser.go` ⚑ (P4-10's) — `IssueKey`, `IssueRef`, the reasons
  `issue_not_adoptable` and `issue_needs_brief`
- `control-plane/internal/api/sessions.go` ⚑ — the create request's `issueKey`, the wire
  `Session.IssueRef`, the stamp from the result on create and restart
- `control-plane/internal/registry/session.go` ⚑ — `Session.IssueRef` beside `ContextRef`
- `control-plane/internal/discoverhub/read.go` ⚑ (P4-07's) — `Resolve`'s `issue:` branch, calling
  `provenance.BuildIssue` / `ErasedIssueRef` (task 18's gap 1, assigned here)
- `desktop/src/store/types.ts` ⚑, `desktop/src/api/client.ts` ⚑ — `issueRef` beside `contextRef`
- `desktop/src/store/selectors.ts` ⚑ — `selectSessionIssueRefs`
- `desktop/src/provenance/loader.ts` ⚑ (P4-16's) — one entry appended to `keySources`

**Interfaces:**
- Produces (**SW.7**): `claudemd.Issue`, `claudemd.IssueServe`, `claudemd.RenderWithIssue` and the
  golden's bytes; `api.MaterialiseRequest.IssueKey`, `api.MaterialiseResult.IssueRef`,
  `api.BriefReasonIssueNotAdoptable`, `api.BriefReasonIssueNeedsBrief`; the wire
  `Session.issueRef` and the create request's `issueKey`; `registry.Session.IssueRef`;
  `selectSessionIssueRefs`.
- Consumes: task 16's `hubclient.Work` (`Issue`, `IssueRevisionAt`) — the fake from task 05 in
  every test here; H34's `Briefs.Resolve` for a `serves` pair naming another brief revision;
  task 18's `provenance.Parse`, `KindIssue`, `BuildIssue`, `ErasedIssueRef`; P4-09's Materialiser,
  P4-10's seam and 409 writer, P4-16's `keySources`.
- Required by: task 24 (Start sends `issueKey`), task 101 (Delegate, W3), task 67 (the issue chip
  reads `session.issueRef`), task 70 (the demo).

#### Contract — the server types

```go
// control-plane/internal/api/materialiser.go ⚑ — additions to P4-10's types
type MaterialiseRequest struct {
	// …P4-10's fields unchanged…

	// IssueKey is the issue:is-…@r key the person adopted (Start, task 24;
	// Delegate, task 101), or "". It is a request, not a fact: the handler
	// stamps what the Materialiser returns in IssueRef, never this.
	IssueKey string
}

type MaterialiseResult struct {
	ContextRef string // P4-10's, unchanged
	// IssueRef is the issue key whose section the Materialiser wrote into the
	// managed block, verbatim, or "". Non-empty only when the block on disk
	// holds that section; the handler stamps it and never interprets it.
	IssueRef string
}

// Two reasons join P4-10's closed list; the desktop switches on them (task 24).
const (
	// The issue revision cannot go into an agent's context: a text is
	// author "A", the issue is in another Hub project, the key does not
	// resolve, or a serves pair names a criterion its brief does not hold.
	BriefReasonIssueNotAdoptable = "issue_not_adoptable"
	// An issue section lives inside the brief's managed block, and this
	// worktree has no brief to inherit, or the person started without it.
	BriefReasonIssueNeedsBrief = "issue_needs_brief"
)
```

```go
// control-plane/internal/api/sessions.go ⚑ — the wire Session, after ContextRef (:619)
	IssueRef string `json:"issueRef"`
// the create request, after ContextRef (:706)
	IssueKey string `json:"issueKey"`
// the stamp (:921, as P4-10 leaves it): from the result, never from req
	ContextRef: res.ContextRef,
	IssueRef:   res.IssueRef,

// control-plane/internal/registry/session.go ⚑ — after ContextRef (:102-104)
	// IssueRef is the W1 issue key whose section the Materialiser wrote, or
	// "". Stamped from the result, like ContextRef; never interpreted here.
	IssueRef string `json:"issueRef"`
```

The create request keeps P4's strict decoder, so a request carrying `"issueRef"` is refused as an
unknown field (400), which is stronger than ignoring it; the test below pins that. **Restart**
re-materialises with `IssueKey = record.IssueRef` (the key already adopted, not a new head), so a
restarted agent reads the same bytes; P4-10's rule that a Hub outage mid-restart leaves the
running agent running (`task-10.md:273`) applies unchanged.

#### Contract — the renderer, and SW.7's bytes

```go
// control-plane/internal/claudemd/render.go ⚑

// Issue is the one issue section of a managed block: an adopted revision,
// already checked by contextwrite (H authorship, same project). Render never
// checks; it only neutralises and lays out.
type Issue struct {
	Key         string // issue:is-…@r
	Alias       string // "EP-142", display only
	WrittenBy   string // the revision's writtenBy.displayName
	Title       string
	Description string
	Serves      []IssueServe // in the revision's order; may be empty
}

// IssueServe is one serves pair with its criterion text, looked up by id.
type IssueServe struct {
	CriterionID string
	Text        string
}

// Render is P4-05's, unchanged in bytes: it is RenderWithIssue(doc, nil).
func Render(doc discover.Document) string { return RenderWithIssue(doc, nil) }

// RenderWithIssue is Render with one more section after the brief's last
// section and before the "---" footer, and one more sentence in the footer.
// Every issue string passes through clean, as every brief string does, so
// strings.Count(block, "<!--") == 2 still holds.
func RenderWithIssue(doc discover.Document, iss *Issue) string
```

What `RenderWithIssue` writes when `iss != nil`, between the last section's line and
`"\n---\n\n"`:

```go
	fmt.Fprintf(&b, "\n## The issue you are working on\n\n")
	fmt.Fprintf(&b, "%s · %s · written by %s\n\n", clean(iss.Alias), clean(iss.Key), oneLine(clean(iss.WrittenBy)))
	fmt.Fprintf(&b, "### %s\n\n", oneLine(clean(iss.Title)))
	if d := clean(iss.Description); d != "" {
		fmt.Fprintf(&b, "%s\n\n", d)
	}
	if len(iss.Serves) == 0 {
		b.WriteString("Serves: no criterion of this brief.\n")
	} else {
		b.WriteString("Serves:\n")
		for _, s := range iss.Serves {
			fmt.Fprintf(&b, "- %s — %s\n", clean(s.CriterionID), oneLine(clean(s.Text)))
		}
	}
```

and the footer's line gains `fmt.Fprintf(&b, " Issue %s.", clean(iss.Key))` after the brief (and
inherited brief) sentences. The description is written **verbatim** (after `clean`, which also
normalises `\r\n` to `\n`), so a person's Markdown reaches the agent as they wrote it; the title,
the author and each criterion go through P4-05's `oneLine` so none can open a new heading.

**The golden** — `testdata/golden-issue.md`, P4-05's branch golden (`task-05.md:262-304`) plus
EP-142 at revision 3. The test builds the Document and the `Issue` in Go and compares bytes;
**fix the renderer, never the golden file**. Only the tail differs from P4's golden; it reads:

```markdown
## Out of scope
_Set on this branch, replacing the project brief: Billing and invoicing_

Bulk upload; the reviewer side of evidence

## The issue you are working on

EP-142 · issue:is-3f2a91c07b1e4d2a@3 · written by Dylan Example

### Resume a session after a restart

A restarted sandbox must pick up the same conversation.
Keep the transcript store; do not re-run the opening prompt.

Serves:
- c-91e2a4b0 — the resume test passes

---

Brief bf-00000000000000a2, revision 5. Inherits project brief bf-00000000000000b7, revision 5. Issue issue:is-3f2a91c07b1e4d2a@3.
<!-- panacea:discover:end -->
```

The begin marker is **unchanged** (`brief=… rev=…`): P4-05's `parseMarker` and `Inspect` know
nothing of issues, and must not need to, because the block is still the brief's. The criterion
`c-91e2a4b0` is added to the golden's Definition of done as a fourth item, so the served criterion
is one the rendered brief really holds.

