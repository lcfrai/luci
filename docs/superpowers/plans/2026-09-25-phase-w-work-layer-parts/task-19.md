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

