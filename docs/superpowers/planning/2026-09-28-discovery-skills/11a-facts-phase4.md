# 11a — Fact sheet: phase 4 (Discover) as built

**What this is.** Facts about phase 4 as it stands in the phase-4 worktree today, for a designer.
It designs nothing and recommends nothing. Every fact carries a `file:line` in
`.claude/worktrees/phase-4/` (branch `feat/phase-4-discover`), abbreviated below as `p4/`.
Written 2026-09-29.

**Tree read.** `p4/` at `ca737d92` (*docs(plan): task 09 fix round 1, the part follows the code*).

## What could not be verified

- **No test was run for this sheet.** "Approved", "green" and every test count are what the review
  files, the parts and the ledger say, not a run made here.
- **The ledger is not a committed record.** `p4/.superpowers/sdd/p4/progress.md` is local and
  git-ignored (`:5-6`); it was read by grep only, as instructed.
- **Task 09's mutation round** is "running" in the ledger's last line (`progress.md:562`); its
  result lives in a scratchpad outside the tree and was not read. No re-review of 09 exists.
- **No brief has reached a real session.** The binary runs the interim Materialiser that writes
  nothing (section 4); what `contextwrite` writes is known from its code, tests and goldens only.
  Whether Claude Code reads `.claude/CLAUDE.md` is marked UNVERIFIED in the code itself
  (`contextwrite/materialise.go:47`, deferred to task 23).
- **The prototype was not opened.** Its 15 states are read from its `ORDER` array
  (`prototype/phase-4-discover.html:678`); how each looks was not seen, and headless screenshots
  are blocked here anyway (`CLAUDE.md` §Working agreements).
- **No model was called.** The follow-up's behaviour is from `assist.go` and its tests; the ledger
  notes the gateway's handling of `maxLength` is unverified until task 23 (`progress.md:362`).
- _(more added per section)_

---

## 1. The plan and what is built

**The plan.** `p4/docs/superpowers/plans/2026-09-23-phase-4-discover.md` (600 lines) plus 23 parts,
`p4/docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-01.md` … `task-23.md`. Its goal
(`:46-62`): five questions answered one at a time become the brief, stored in the Hub; when a
session starts, the control plane writes the brief's latest complete revision into the worktree's
`CLAUDE.md` before the agent's process starts and stamps the session with the key of what it wrote;
that key, `brief:bf-…@n`, is the start of the thread; the model "helps and never gates".

**The task table** (`:390-414`). Hours are the plan's estimates, not time spent. State is from the
review files in `p4/.superpowers/sdd/p4/` (their last `Task quality:` line) and the git log.

| # | Task (plan wording, shortened) | Depends | Est. | State today |
|---|---|---|---|---|
| 01 | Lift `writeAtomic` into `statepath` | — | 1 h | Approved (`task-t01-review.md`) |
| 02 | `modelclient` | — | 1.5 h | Approved (`task-t02-review-1.md`) |
| 03 | `discover`: content model and ops (`4-D2`, R6 ids, per-question skip) | — | 3 h | Approved (`task-t03-review-1.md`) |
| 04 | `discover`: the model follow-up, degrading to nothing | 02, 03 | 1.5 h | Approved (`task-t04-review-1.md`) |
| 05 | `discover.Resolve` (`4-D3`); `claudemd` Render, Merge, Strip, Inspect | 03 | 2.5 h | Approved (`task-t05-review-2.md`) |
| 06 | `provenance`: key grammar, `Ref`, `Build`, criterion digests | 03, 05 | 2.5 h | Approved (`task-t06-review-2.md`) |
| 07 | `discoverhub`: binding and reads | 03, 05, 06, Hub H0; HA4-2 for the real Hub | 3 h | Approved (`task-t07-review-1.md`) |
| 08 | `discoverhub`: writes, `4-D3` fan-out, archive, rebind | 07; HA4-2, HA4-9 | 3 h | Approved (`task-t08-review.md`) |
| 09 | `contextwrite`: the Materialiser | 01, 05, 07, 10 | 3 h | **In fix** (see below) |
| 10 | `api`: the `Materialiser` seam, `brief_not_written`, `RouteMounter`, interim | — | 3.5 h | Approved (`task-t10-review.md`) |
| 11 | `discoverapi`: brief routes, relay rule, route audit | 07, 08, 10 | 3 h | Unbuilt |
| 12 | `discoverapi`: refs, branch provenance, materialisations | 09, 11 | 2 h | Unbuilt |
| 13 | `repoanalysis` and the drafted brief | 02, 03, 11 | 2.5 h | Unbuilt |
| 14 | Wiring and configuration in `cmd/api` | 09–13; Hub C2; phase 3 task 13 | 1.5 h | Unbuilt |
| 15 | Desktop: Discover client, wire types, problem codes | 11, 12 | 2 h | Unbuilt |
| 16 | Desktop: `discoverSlice`, `provenanceSlice`, the one loader | 15 | 3 h | Unbuilt |
| 17 | Desktop: `ContextChip`, `DefinitionOfDone`, `ShippedManifest` | 16 | 3 h | Unbuilt |
| 18 | Desktop: the Discover lens — binding, asking, the brief born, follow-up | 16, 17 | 3 h | Unbuilt |
| 19 | Desktop: moved on, project changed, undo, past revision, missing, erased, hand-edited | 18 | 3 h | Unbuilt |
| 20 | Desktop: `4-D1` on every session start (six paths) | 15, 16 | 4 h | Unbuilt |
| 21 | Desktop: import a repository — drafted from the code | 13, 18 | 2 h | Unbuilt |
| 22 | Contract amendments (Go, UI, provenance) | 10, 12, 16 | 1.5 h | Unbuilt |
| 23 | End-to-end verification; the owner's batched steps | all | 2 h | Unbuilt |

**Counts.** 9 of 23 approved (01–08, 10): 21.5 of the plan's 57 estimated hours. One in fix (09,
3 h). Thirteen unbuilt (11–23, 32.5 h), every desktop task among them. The ledger says the same:
"9 of 23 tasks APPROVED" (`p4/.superpowers/sdd/p4/progress.md:550`) and the remaining order is 09's
fix and re-review, then 11 to 23 (`:554`).

**Task 09 in detail.** Built and committed (`5fb80e09` code and tests, ledger `:540`); its review
`task-t09-review.md` ends *Needs fixes*, 0 Critical / 2 Important / 8 Minor, both Importants
security (host git ran a sandbox-written `core.fsmonitor`; the Materialiser followed planted
symlinks) (ledger `:551`). Fix round 1 is committed as `6a354f69` (code) and `ca737d92` (part 09)
(`:560`), green on `contextwrite` and `statepath` (`:559`). The ledger's last line says a mutation
round over 18 mutants was **running** at `ca737d92` (`:562`). No `task-t09-review-1.md` exists.

**What "unbuilt" looks like on disk.** No `discoverapi` or `repoanalysis` package exists
(`ls p4/control-plane/internal/`). The only Discover file in the desktop is phase 2's placeholder,
`p4/desktop/src/components/lens/DiscoverLens.tsx:8-16`: it renders `LensPlaceholder` with the line
*"Turn a conversation into a spec this branch can be measured against."* (`:13`) — note it says
"spec", a word `CONTEXT.md` rules out. The binary still runs `materialiserUntilTask14`
(`p4/control-plane/cmd/api/materialiser_interim.go:18`), task 10's stand-in until task 14 wires
the real one.

**Order and the UI gate.** Three lanes: pure Go (01→06), seams (10 alone), desktop (15→21)
(`plan:416-424`). "UI branches (17–21) stay unmerged until the owner has looked" (`OD-1`,
`plan:427-428`). Critical path: Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23 (`plan:430`).

## 2. The brief content model (task 03)

Package `p4/control-plane/internal/discover`. Paths below are relative to it: `content.go` (403
lines) and `ops.go` (556 lines).

**It is a wire format.** `content.go:1-6`: "a revision is immutable and kept for ever - so this
file is a WIRE FORMAT. Field names, JSON tags and the rule 'every slice marshals as [], never null'
are part of every digest ever computed." `ContentVersion = 1`, the only version (`:17-19`). `Decode`
refuses a missing or sub-1 `v` and any `v` above 1, and fills nil slices with `[]` (`:250-292`).

**`Content`** (`:218-225`) — six fields, JSON names in brackets:

| Field | Type | Meaning |
|---|---|---|
| `V` (`v`) | int | always 1 |
| `Status` (`status`) | `open` \| `complete` | derived, never set by hand (`:40-47`); the Hub also accepts `skipped`, v1 never writes it (`:41`) |
| `Questions` (`questions`) | `[]Question` | the five base questions plus any follow-ups |
| `Answers` (`answers`) | `[]Answer` | at most one per question key |
| `Inherits` (`inherits`) | `*Inherit` | branch: the project's latest complete revision when written, or null; project: always null (`:223`) |
| `Change` (`change`) | `Change` | what produced this revision; "read by the history view and never rendered into CLAUDE.md" (`:209-210`) |

- **`Question`** (`:70-78`): `key`, `projectPrompt`, `branchPrompt`, `help`, `options []string`,
  `followUp bool`, `because` (a follow-up's reason; `""` on base questions). "The free-text escape
  is always available and is not modelled here" (`:68-69`). `Prompt(scope)` picks the project
  wording on a project brief, else the branch wording (`:83-88`).
- **`Answer`** (`:168-177`): `key`, `text` (`""` exactly when skipped), `skipped`, `source`
  (`picked` \| `typed` \| `drafted`, `:51-55`; `""` when skipped), `author` (`person` \|
  `model_drafted` \| `model_accepted`, `:62-66`), `criteria` (the `done` key only, `[]` otherwise),
  `draftedFrom` (the repo on-ramp's manifest and directory names, `[]` otherwise), `answeredAt`
  (RFC 3339 UTC, three fractional digits, `:399-403`).
- **Settled** (`:179-186`): skipped, or non-empty text whose author is `person` or `model_accepted`.
  A `model_drafted` answer "is shown, marked, and NOT settled, so a revision holding one is never
  complete and is never inherited, pinned or approved against" (`:57-59`).
- **Status** (`ops.go:407-419`): `complete` exactly when there is at least one question and every
  question has a settled answer. A content with no questions is never complete.
- **`Inherit`** (`:190-193`): `{key: "brief:bf-…@n", digest}` — the pinned project revision (`4-D3`).
- **Keys** (`:28-38`): five fixed — `purpose`, `audience`, `constraints`, `done`, `out_of_scope`; a
  follow-up's key is `f-` + 8 hex, minted when it is added.

**The ops** (`ops.go:15-25`) — nine, the wire shape of `POST …/brief/ops` (task 11, `:27-29`):

| Op | Does | Where |
|---|---|---|
| `record` | a person's answer; source must be `picked` or `typed` | `:164-168` |
| `edit` | replace a settled, non-skipped answer in place (always `typed`) | `:170-177` |
| `skip` | per-question skip: `{skipped: true, author: person, text: ""}` | `:179-185` |
| `accept` | `model_drafted` → `model_accepted`, text unchanged | `:187-196` |
| `add_follow_up` | append a follow-up question; the server mints its `f-` key | `:198-218` |
| `dismiss_follow_up` | remove a follow-up and its answer; base questions refuse | `:220-232` |
| `restore` | an earlier revision's questions and answers come back (server fills them) | `:234-240` |
| `reinherit` | branch only: move `Inherits` to the project's current pin (the `4-D3` fan-out) | `:242-246` |
| `draft` | the repo on-ramp writes `model_drafted` answers, never over a settled one | `:248-260` |

A tenth kind, `propose_follow_up`, is not a `discover` op: `discoverhub` expands it by asking the
model and writes nothing unless a question is proposed
(`p4/control-plane/internal/discoverhub/write.go:16-20`, `:124-125`).

**Skip is per question and includes `done`.** Any key may be skipped; nothing guards `done`
(`ops.go:179-185`). A skipped `done` has `criteria: []` and still completes the brief — pinned by
`TestSkipIsSettledAndCompletesTheBrief` (`ops_test.go:101`, the `done` row at `:122-124`).

**Criterion ids (R6).** `Criterion` is `{id: "c-" + 8 hex, text}` (`content.go:160-166`). Only the
`done` answer carries criteria (`ops.go:277-290`). The text is split by `SplitCriteria` —
newlines and semicolons, leading `- ` or `* ` stripped, empties dropped, no model call
(`content.go:380-397`). Each new item keeps the id of an unused previous criterion with **exactly**
the same text; every other item gets a fresh id never held before, so evidence never attaches to a
different sentence (`ops.go:295-330`). A `done` answer that splits into zero items is refused
(`ErrEmptyAnswer`, `ops.go:286-288`). Ids come from `Env.NewID` (`ids.New` in production; nil is a
server fault), retried up to 8 times on a clash (`ops.go:49`, `:81-83`, `:332-340`).

**Text rules on every string an op adds or changes** (`ops.go:449-534`):
- line endings become LF (`toLF`, `:421-424`); an answer is also trimmed (`:271`);
- a prompt, answer or criterion containing `panacea:discover:` — the managed-block marker prefix —
  is refused with `ReservedTextError{Key}` (`:121-124`, `:523-527`); help and options are not checked
  for it (`:462-469`);
- an answer over **4000** code points, or a prompt, help or option over **300**, is refused with
  `TooLongError{Key, Limit}` (`:93-96`, `:528-532`); criteria and `because` have no cap of their own.

**`Apply` is pure** (`ops.go:126-159`): it never mutates its inputs, sets `V`, the scope's
`Inherits`, `Change` and the derived `Status` on every result (`finish`, `:392-405`), and returns
`ErrNoChange` — "the caller writes no revision" — when questions, answers (ignoring `answeredAt`)
and `Inherits` are unchanged (`:536-556`). On any error the result is the zero `Content`.

**How a revision is produced.** `discoverhub.Service.Apply`
(`p4/control-plane/internal/discoverhub/write.go:111-181`, nine numbered steps): read the head
content; run `discover.Apply` with `Env.Inherits` set to the project's latest complete revision now
(branch scope only); `ErrNoChange` writes nothing; create the brief only once there is a change to
write (`:115-118`); then one `WriteRevisions` call carrying `BaseRevision` — "the head the person
saw" (`:25`, `:259`). A `revision_conflict` naming this brief goes back to the person unchanged
(`:152-153`). A project revision that becomes complete fans `reinherit` out to every live branch
brief whose head pins a different project key, open heads included (`:128-136`), in the same batch.
The Hub assigns the number; the service predicts `BaseRevision + 1` only to compute the pin's
digest before the write (`:263-270`) and checks the Hub's digest agrees after it (`:155-156`).
`NewContent` — five questions, no answers, `open` — "is never written itself - revision 1 is the
first op applied to it" (`content.go:227-237`).

**What is immutable.**
- A revision once written: "immutable and kept for ever" (`content.go:2-3`). An edit is a new
  revision; `restore` writes a new revision holding an old one's questions and answers, with
  `change.restores` naming the source (`content.go:214`, `ops.go:234-240`).
- The content's field names, JSON tags and `[]`-not-null rule (`content.go:4-6`).
- A criterion's id while its exact text survives (`content.go:160-162`).
- The five base questions' wording and options, "word for word from the 2026-09-18 plan", in a
  closed set: "'QUESTION 2 OF 5' is honest only because this list cannot grow mid-flow"
  (`content.go:90-95`). Follow-ups are appended and shown without a denominator (`:92-93`,
  `Step` sets `Ordinal` and `Total` to 0 for them, `:369-371`).

## 3. The provenance key grammar and `Ref` (task 06)

Package `p4/control-plane/internal/provenance`: `key.go` (109 lines), `ref.go` (444 lines). "The
package is pure: no I/O, no clock, no Hub" (`key.go:5-6`); it "owns the key grammar … nothing else
in the product may split a key" and owns `Ref`, "the one type Develop, Merge, Design and Deploy
render" (`key.go:1-3`).

**The three key forms** (`key.go:23-29`):

| Form | Pattern | Example |
|---|---|---|
| brief | `^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$` | `brief:bf-00000000000000a2@5` |
| design | `^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$` | (a canvas id, phase 5's) |
| none | `^none:(declined)$` | `none:declined` = `provenance.Declined` (`:40-43`) |

- brief and design are "the Hub's, verbatim (Hub contract §3.2, hub-contract.md:120-121 at hub
  da8c7da)"; none "is local to the control plane and never reaches the Hub (P4-C)" (`:23-25`).
- Revision: 1 to 9 digits, no leading zero (`:69-70`). A trailing line feed makes it not a key
  (`:31-32`). `""` is not a key — `Parse("")` is `ErrInvalidKey` — but `NoneRef("")` answers
  resolution `none`: no brief existed (`:54-55`, `ref.go:419-433`).
- `Key` holds `Kind`, `ID`, `Revision`, `NoneReason` (`key.go:33-38`). `Key.String` writes `""` for
  any key that would not parse back to itself (`:79-97`); `BriefKey(id, n)` panics on a bad id or
  revision — "a programming error, not input (the Hub mints both)" (`:99-109`).

**Which parsers compile the grammar.** Two compiled copies are in this tree, plus one look-alike:

1. `provenance/key.go:48-52` — all three forms, commented "They are the grammar's only copy"
   (`:47`). Its callers: `contextwrite/inspect.go:223`, `discoverhub/read.go:123`, `:241`, `:350`,
   `discoverhub/write.go:581`, and `provenance` itself.
2. `hub/hubapi/keys.go:56-59` — brief and design only, the Hub's own package (from Hub H0, merged
   into this branch, ledger `progress.md:486`), with `ParseBriefKey` (`:69`), `ParseDesignKey`
   (`:86`) and `ParseKey` (`:95-104`); used by the fake Hub (`hub/hubclient/fake_briefs.go:476`).
3. Not the grammar: `claudemd/render.go:287-299` reads the managed-block marker's
   `brief=<id> rev=<n>` with `discover.MarkerSafe` and a decimal check (section 4).

The desktop has no key parser: "the desktop, which never parses a key (P5)" (`ref.go:405-406`);
`git grep "brief:"` over non-test `desktop/src` finds nothing.

**What `Ref` carries** (`ref.go:71-103`; every slice `[]`, never null, `:77`):

- *Which revision:* `key`, `resolution`, `noneReason` (`""` \| `declined`), `present` (resolved or
  erased), `briefId`, `revision`, `digest`, `scope`, `projectId` (the Hub's `pj-…`), `branchRef`,
  `status` (the revision's content status — open revisions can be named by a history row).
- *The heads, the only time-varying part:* `headRevision`, `latestCompleteRevision` (`:71-72`).
- *The pin:* `inherits` = `InheritedRef{key, briefId, revision, digest, erased}` (`:61-69`).
- *The text:* `title`, `headline` (the purpose section's body), `audience`, `constraints`,
  `outOfScope`, `sections` (`[]discover.Section`), `criteria` (`[]{id, text, digest}`, `:43-47`).
- *Saved:* `savedAt`, `savedBy` (a display name), `erasedAt`.

Six resolutions (`ref.go:16-31`): `resolved`, `missing` (no such revision *or* one the caller
cannot read — "indistinguishable on purpose"), `erased` (key, digest, savedAt, savedBy remain, no
text; `:435-444`), `invalid`, `none`, `unavailable` (the Hub did not answer; never cached by the
desktop). **Not in a Ref:** criterion state — it "MOVES", so it is a `CriterionReading{id, state,
because}` served beside it (`:49-59`, `:73-74`); *superseded*, which the desktop derives from
`revision < latestCompleteRevision` (`:75-76`); any denominator (`TestRefHasNoDenominator`,
`ref_test.go:692`). Criterion state is `open` throughout phase 4 (`:57`); phase 3's evaluator fills
it through the `Evaluate` seam, nil meaning every criterion is open (`:131-139`).

**How the digests are built.**
- *Revision digest* — `"sha256:"` + hex SHA-256 of the canonical §3.4 envelope
  `{"v":1,"kind":"brief-revision","briefId","revision","projectId","scope","branchRef","content"}`,
  members sorted, content canonicalised (`discoverhub/digest.go:26-49`). The Hub computes it; phase
  4 builds the same bytes locally only because a branch revision in the same batch must pin the
  project revision's digest before the Hub has computed it (`:10-16`);
  `TestLocalEnvelopeIsTheHubsEnvelope` holds the two byte for byte (`:24`). `Ref.digest` is the
  Hub's (`ref.go:203`).
- *Criterion digest* — `"sha256:"` + hex SHA-256 of the text's UTF-8 bytes, "verbatim: not trimmed,
  not case-folded, not JSON-quoted (P4-2)" (`ref.go:306-311`), computed by `Build` for each item
  of the `done` section (`:291-304`).

**`Build(key, src, head)`** (`ref.go:167-223`) resolves through `discover.Resolve` (section 4) and
fills every field. Every error is `ErrBadSource` — "a caller fault (a 500), never the person's" —
and the Ref returned beside it is `UnavailableRef(key)`, "never one key on another revision's
text" (`:161-165`, `:185-195`).

## 4. `claudemd` (task 05) and the Materialiser (task 09)

Paths: `p4/control-plane/internal/claudemd/render.go` (443 lines), `…/discover/inherit.go` (334),
`…/contextwrite/materialise.go` (588) and `…/contextwrite/inspect.go` (241).

**What the running binary writes today: nothing.** `cmd/api` runs `materialiserUntilTask14`,
which "writes nothing" and logs once at start-up *"sessions inherit no brief until phase 4 task 14
wires contextwrite"* (`p4/control-plane/cmd/api/materialiser_interim.go:12-25`); it returns `""`,
or `none:declined` when the person declined (`:27-36`). No file outside the phase 4 packages
imports `discover`, `claudemd`, `provenance`, `discoverhub` or `contextwrite` (`git grep` of
`control-plane` and `sandboxd` at `ca737d92`). Everything below is what the built, tested,
**unwired** `contextwrite` would write once task 14 lands.

**Which file, and when.** `CLAUDE.md` at the worktree root — "the name is the contract with Claude
Code; not configurable" — or `.claude/CLAUDE.md` when git tracks `CLAUDE.md`, marked "UNVERIFIED
that the CLI reads it (task 23)" (`materialise.go:45-48`). Git tracking both refuses the start,
`tracked_elsewhere` (`:100-109`, message `:342`). The file is on disk "before the harness process
starts" — on create before the conversation store's `MkdirAll`, on restart before the old sandbox's
destroy (`:3-9`).

**Which revision.** `discoverhub.Inheritable`: "the latest complete branch revision, else the latest
complete project revision, else none" (`discoverhub/read.go:77-87`), read under a 5-second Hub
budget (`materialise.go:81-87`, `:136-146`). The session's `contextRef` is the key written, `""`
when there was nothing, `none:declined` when the person started without it (`:207-213`,
`:218-223`, `:254`).

**The bytes today** (`Render`, `render.go:62-111`), in order:

1. `<!-- panacea:discover:begin brief=<briefId> rev=<n> -->` (`:79`) — the id and number, not the
   literal key string.
2. `# <title>`: `Project brief`, or `Branch brief — <branch>` from the revision's own `branchRef`,
   never a current project or branch name (`inherit.go:26-30`, `:171-182`).
3. Two fixed lines (`:81-82`): *"Written by Discover in Panacea App. Every session started on this
   branch reads it first."* and *"Change it in Discover, not here: this block is regenerated and
   your edits inside it will be lost."*
4. With no section: *"No question in this brief has an answer."* (`:84-86`).
5. Per section: `## <heading>`; an origin line — `_Inherited from the project brief._`, or
   `_Set on this branch, replacing the project brief: <old text on one line>_`, or nothing for a
   branch-only answer (`:87-95`, origins `inherit.go:32-38`); then `- <text>` per criterion for
   `done`, else the answer's text (`:96-102`).
6. `---`, then `Brief <id>, revision <n>.` plus ` Inherits project brief <id>, revision <n>.` when a
   project revision is pinned (`:105-108`); then `<!-- panacea:discover:end -->` (`:109`).

- **Headings** (`inherit.go:75-87`): `What this is for`, `Who it is for`, `What must not break`,
  `Definition of done`, `Out of scope`, always in that order; then follow-ups, project first, then
  branch, in the order added, each headed by its prompt minus a final `?` (`:105-108`, `:286-333`).
- **Which answers appear** (`Resolve`, `inherit.go:89-103`): only settled, non-skipped answers; a
  `model_drafted` answer never; a skipped branch answer shows the project's; a skipped project
  answer with no branch answer is left out; every revision passed must be complete.
- **A project key marks every section inherited.** With no branch revision, "every section
  OriginProject" (`inherit.go:91`), so each carries `_Inherited from the project brief._`
  (`claudemd/testdata/golden-project.md:8`, `:13`, `:18`, `:23`, `:29`).
- **Never in the block:** a timestamp — "a timestamp would rewrite the file on every materialise"
  (`render.go:5-8`); the `change` record (`content.go:209-210`); criterion ids and digests (items
  print as `- <text>`, `render.go:98`); author, source, help, options, `because`, `savedBy`.
- **Neutralised:** every interpolated string has CR/CRLF made LF and each `<!--` written `<!-\-`
  (`render.go:47-54`), so a block holds exactly two `<!--`, its own markers (`:65-70`).

**No inline cap.** Nothing limits the block or the file: no length check in `Render`, and a grep of
`contextwrite`, `claudemd` and `inherit.go` (non-test) for `LimitReader`, `KiB`, `MiB`, `1 <<` finds
only `maxDiffCells` (`render.go:354`). The bounds that exist are upstream: 4000 runes per answer,
300 per prompt, help or option (`discover/ops.go:93-96`); 64 KiB per canonical revision envelope
(`hub/hubapi/envelope.go:31`, used at `discoverhub/write.go:49`). The one "400" is `DiffLines`' cap
on the hand-edit view's output lines (`render.go:348-363`), not on the block.

**The goldens that pin the block** (`claudemd/testdata/`, `.gitattributes` = `* -text`):

| Golden | Lines / bytes | Test | Shows |
|---|---|---|---|
| `golden-branch.md` | 37 / 1237 | `TestResolveBranchOverProjectGolden` (`render_test.go:105`) | a branch over a project: four overrides, one inherited, three criteria |
| `golden-project.md` | 36 / 856 | `TestResolveProjectAloneGolden` (`:111`) | a project key alone, all five sections |
| `golden-neutralised.md` | 30 / 812 | `TestRenderNeutralisesCommentOpeners` (`:320`) | `<!--` and a forged end marker in the title, an override note, answers, a criterion and a follow-up's heading |

The branch golden's failure message reads "fix the renderer, never the golden" (`render_test.go:107`).

**Merge, Strip, Inspect** (`render.go`). Everything outside the two markers "belongs to the person
and is never touched" (`:24-25`).
- `Merge(existing, block)` replaces the one block, or appends it after a blank line; any other
  marker shape (a begin with no end, a stray or early end, two ends, two begins) returns the file
  unchanged with `ErrUnterminatedBlock` or `ErrDuplicateBlock`, because "the guess would delete
  somebody's writing" (`:165-177`, `:189-210`). It writes in the file's own line ending, CRLF when
  the first line ending is CRLF (`:178-188`).
- `Strip` removes the block, the line ending after it, and — when the block is last — the blank
  line `Merge` put before it (`:212-244`).
- `Inspect` finds the block and reads its marker, "exactly `BeginPrefix + " brief=" + id + " rev=" +
  n + " -->"`", with the id passing `discover.MarkerSafe` (non-empty, no white space, no `<` or `>`,
  `inherit.go:235-251`) and `n` a decimal with no leading zero (`render.go:258-267`, `:281-320`).
  "Its only use is the drift check … NOTHING reads brief text back out of a file as a source"
  (`:266-267`). `SameText` is that check's only comparison, blind to CRLF and a lost final line
  ending (`:322-333`).

**The Materialiser's three rules** (`materialise.go:3-20`): the ordering rule (above); the commit
rule — "the managed block is never committed (phase 3's 3-D6)": written only to a file git does not
track, whose path is added to the clone's `info/exclude`, and a tracked file "is never opened for
writing, not even to strip a block from it"; the link rule — "nothing below the worktree is
followed" (`confine.go`, task 09's fix round). Before each write it adds `/CLAUDE.md` (or
`/.claude/CLAUDE.md`) and the temp file's `/.CLAUDE-*.md` to `info/exclude`, then checks git really
ignores the target, since a `.gitignore` `!CLAUDE.md` outranks `info/exclude` (`:169-184`). It
writes only when the bytes changed, with `statepath.WriteAtomic(…, Shared)`, and removes a file a
strip leaves holding only white space (`:185-191`).

**The drift check's seven states** (`inspect.go:18-47`): `absent` (no block, nothing to inherit),
`pending` (no block yet, the next start writes one), `current`, `superseded` (the block names
another key; also a right key whose pinned project revision was since erased), `edited` (right
key, other text; `diff` holds `DiffLines`), `stale` (a block, but nothing should be inherited now),
`broken` (the next start is refused `block_broken`). An `Inspection` also carries `markerKey`,
`expectedKey`, `diff`, `committed` (tracked files holding a block: repository text, never written),
`excluded` and `altTracked` (`:49-81`). The marker's key is rebuilt as `"brief:" + id + "@" + n`
and checked with `provenance.Parse` (`:218-228`).

## 5. The interview as shipped

**Shipped means server-side only.** The interview exists as Go data and logic in `discover`; no
desktop screen asks it yet (section 1: tasks 18–21 unbuilt; the lens is phase 2's placeholder).

**Five questions, a closed set, four options each** (`discover/content.go:95-158`), "word for word
from the 2026-09-18 plan (:872-935)" (`:90-91`). The free-text escape "is always available and is
not modelled here" (`:68-69`).

| # | Key | Branch wording (project wording) | Help line | Options |
|---|---|---|---|---|
| 1 | `purpose` | What is this branch for? (…project for?) | One sentence. The first thing every agent here reads. | Add a capability we do not have yet · Fix something broken · Change how something works · Try an idea we might throw away |
| 2 | `audience` | Who is this branch for? (…project for?) | Agents make different choices for an internal tool than a client-facing one. | Our own team · KPMG staff broadly · Client users · Nobody yet — a spike |
| 3 | `constraints` | What must this branch not break? (…project never break?) | Agents will refuse work that violates this instead of guessing. | The published API · Existing data and migrations · The deployed environment · Nothing — it is isolated |
| 4 | `done` | What has to be true before this branch can merge? (…before work here is finished?) | This becomes the definition of done, shown again at merge. | It has tests that pass · It does not break the published API · It works on mobile · Someone else has reviewed it |
| 5 | `out_of_scope` | What is explicitly out of scope for this branch? (…for this project?) | Naming this is how you stop an agent widening the job while you are away. | Performance work · Visual design · Migrations and data changes · Nothing is off the table |

**The flow's one counter.** `Content.Step` walks the questions in order and returns the first
without a settled answer or a pending draft (`content.go:342-378`). Four step kinds: `question`,
`follow_up`, `review` (only drafts remain) and `complete` (`:316-323`). `Total` is 5 on a
`question` step and 0 on every other; "the only legal rendering of it is the literal 'QUESTION n
OF m' (G5)" (`:325-329`).

**The follow-up (task 04)** — `discover/assist.go`, 344 lines.
- **When it is asked:** once per brief, after three base answers are settled, a skip counting
  (`:51-53`, `:141-149`); the plan's lens sends `propose_follow_up` automatically after the third
  settled answer (`task-18.md:142-144`). The server expands that op (`discoverhub/write.go:16-20`).
- **Where it appears:** appended after the five base questions (`ops.go:217`), so the flow still
  shows base question 4 and 5 first; the follow-up step comes after the fifth, with no
  denominator, and the brief stays `open` until it is answered, skipped or dismissed — pinned by
  `TestFollowUpsCarryNoDenominator` (`content_test.go:284-314`).
- **What it is:** at most one question, ≤ 200 characters, help ≤ 200, up to 4 options of ≤ 120
  each (`:55-66`); the model is told "Prefer false: every extra question costs the person time"
  (`:92-102`). Its `because` is composed by the **server** — "Suggested because of your answer to
  “<prompt>”." — never model prose (`:113-117`).
- **What the model sees:** the project and branch names and the answers holding a person's words,
  never file contents (`:15-20`, `:182-185`). One call, effort `medium`, 20-second budget
  (`:35-49`, `:68-71`).
- **Six outcomes, none an error** (`:22-33`): `proposed`, `model_off` (no key or base URL
  configured), `model_failed`, `nothing_to_ask`, `already_asked`, `not_yet`. With no model the
  interview is the plain five questions (plan `:60-62`).

**What the Discover lens tasks plan (all unbuilt).** The specification is
`p4/prototype/phase-4-discover.html` (698 lines; `task-18.md:3-5`), whose state picker lists 15
states: none, inherits, asking, born, followup, modeloff, drafted, superseded, projectmoved,
writefailed, handedited, past, missing, unavailable, chips (`prototype/phase-4-discover.html:678`).
- **18** — the lens itself: binding, asking, the brief born, the model paths, failures; prototype
  states 1–6 and 14 (`task-18.md:1-8`, states table `:122-140`). One question at a time, numbered
  options `1`–`4`, free text always on screen, a settled list with per-answer *Edit*; on a branch
  the skip control reads *Keep the project answer*, on a project *Skip this question* (`:133`);
  a follow-up is marked *drafted by the model* with *Not relevant — skip* (`:135`).
- **19** — the states where brief and inherited file "come apart": moved on, project changed,
  undo, past revision, missing, erased, hand-edited; prototype states 8, 9, 11, 12, 13 plus
  `erased` (`task-19.md:1-5`). *Undo* for five seconds writes a `restore` revision (`:7-9`).
- **20** — the `409 brief_not_written` refusal on all six session-start paths, with *Retry* and a
  deliberate *Start without the brief*; prototype state 10 (`task-20.md:1-20`); inline at the
  control that started it, "not a modal and not a toast" (`:27-32`).
- **21** — import a repository: a brief drafted from a mechanical scan, each answer marked `◌`
  *drafted by the model* with its sources, *Accept* / *Change*, then *Keep refining the brief*,
  *Open Design*, *Open Develop*; prototype state 7 (`task-21.md:1-14`, `:36-41`).

**A plan-versus-code disagreement, recorded as found.** Task 21 sends `edit` for *Change* on a
drafted answer (`task-21.md:38-39`, test row `:48`). The built `Apply` refuses `edit` on any answer
that is not settled, a drafted one included (`ops.go:170-177`), pinned by
`TestEditNeedsASettledTypedOrPickedAnswer` (`ops_test.go:74-82`, "none, skipped, drafted"). `record`
does replace a draft with a person's answer (`ops.go:164-168`).

## 6. What would have to change

_(pending)_

---

## Closing

_(pending)_
