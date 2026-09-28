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

_(pending)_

## 5. The interview as shipped

_(pending)_

## 6. What would have to change

_(pending)_

---

## Closing

_(pending)_
