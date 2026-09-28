# Independent review — the decision register's gate, backwards audit and dependency frontier

Reviewed 2026-09-28, read-only, in the main tree on `feat/phase-2-cockpit`. Subject: commits
`1cb8be9a` (gate + frontier), `686e35c2` (backwards audit + *Holds up* lines), `7829f86d` (report).
Files read: `2026-09-23-owner-decisions.md`, `2026-09-25-owner-questions-pending.md`,
`07-register-report.md`, and the documents that cite them. Nothing was changed except this file.

**Verdict: one Critical, two Important, three Minor.** The gate is a real test — it gave me a clear
answer on both rows I tried it on cold, and on one of them it told me which file to go read. The
edges I checked are evidenced with one exception. The Critical is not in what was written but in
**where it was written**: inserting 36 lines at line 8 moved every decision down the file, and 41
line-anchored citations in 12 documents now point 36 lines early.

---

## C1 (Critical) — the gate section broke 41 inbound line citations

`2026-09-23-owner-decisions.md` lines 8–43 ("What earns an OD") were inserted **above the first
decision table**. Every line below shifted by 36. The programme cites this file by line range in 41
places across 12 documents, and those anchors were **exact** before the change.

Verified against `a109bf3e` (the commit before this work), same file:

| Cited as | Was, at `a109bf3e` | Is now, same line |
|---|---|---|
| `owner-decisions.md:13` (OD-2) | `\| **OD-2** \| 3-D1: what a worktree is in git …` | `are true.` (gate, limb list) |
| `owner-decisions.md:21` (OD-10) | `\| **OD-10** \| What Forget does to a persisted transcript …` | `Hub phase owns identity, and phase 7 consumes it.` |
| `owner-decisions.md:73-79` (the OD-8 rule) | the amended transcript rule, blockquote | `## The new phase: the Hub` and after |
| `owner-decisions.md:114` (team mode) | `## Team mode — owner decisions of 2026-09-24 …` | mid-blockquote of the OD-8 rule |
| `owner-decisions.md:140` (OD-19) | `\| **OD-19** \| How agents get inference …` | `- **OD-9**: the "Send shortcut keys…" toggle` |

Two of those are **binding** citations, not prose references:

- `2026-09-24-team-collaboration/03-techniques.md:45` — constraint K1, sourced to
  `owner-decisions.md:73-79`.
- `2026-09-24-team-collaboration/04-enterprise-constraints.md:121` — quotes *"no feature may require
  the adapter to function"* from `owner-decisions.md:73-79`.

Both now resolve into the gate's own "What the gate is not for" list. An agent that follows the
anchor rather than the quote reads the wrong rule.

This matters more because the new text asserts the opposite: *"It is not retrospective. Nothing
already numbered is deleted or renumbered — every `OD` id is cited across the programme."* The
citation-safety argument was made about **ids** and is correct about ids. It was not made about line
anchors, and the file's own preface at line 4 (*"Every plan written after this date cites it by
decision id"*) is what made that omission look safe. 41 citations say otherwise.

A twelfth file makes it worse: `08-context-report.md:38`, written today by the sibling chain, cites
`owner-decisions.md:50` for OD-2 — the **post-insert** line. The tree now holds two incompatible
anchor conventions for one file.

**Cheapest fix (restores 40 of 41, breaks 1):** move the whole "What earns an OD" section to sit
immediately before "The dependency frontier", near the end. Every pre-existing anchor returns to
correct; only `08-context-report.md:38` needs `50` → `14`. The gate reads no worse there — the
backwards audit that applies it is already at the end, and line 42's pointer to the frontier shows
the file already expects a reader to travel.

**Alternative (more edits, keeps the gate up front):** bump all 41 anchors by +36. More work, and
it locks the file against any future insertion above line 43.

---

## Q1 — is the gate a test, or a description? It is a test.

Three limbs, an explicit **"All three, not two"**, and — the part that makes it operable — each limb
carries a sentence naming *what it excludes*, so each one names the artifact that settles it. Plus
three stated non-purposes, which stop the usual drift into "everything important is an OD".

I applied it cold to two rows **the backwards audit does not mention**, to test the test rather than
re-read its author's answers.

**Trial 1 — OD-20 (local mode stays; cloud-first; ~90% never local).** PASS, in under a minute.
1. *Expensive to undo* — reversing it deletes the local Docker path and re-plans phase 6 and phase 8's
   relationship to it. Code and a re-plan, not a setting. Pass.
2. *Not derivable* — the tree shows local mode exists; it does not show the 90% target, and it
   certainly does not show the requirement that target imposes (the cloud must track the latest
   Claude Code CLI, with a bump pipeline). An agent would guess wrong. Pass.
3. *A real fork* — cloud-only now versus keep local, with the cost named in the row. Pass.

**Trial 2 — OD-27 (progress displays: proof per criterion, no predictions).** FAIL on limb 2, and
this is the trial that proves the gate is a test: the limb told me what evidence to fetch. It says
*"where the answer is legible from the source, the source is the record"* — so I went to the source.
`docs/AGENTIC-UX.md:77` already reads *"No percentages, no bars, no completion ticks, no '2 of 4
met'"* and `:237` *"No percent-complete, progress bar, ETA, or 'N of M met' anywhere in the
product."* That file was last touched `9f8c8c41`, **2026-09-17** — eight days before OD-27 was
answered. The row's own wording concedes it: *"**Still** banned"*. OD-27's only non-derivable
content is the four one-line amendments, which is a note about a doc edit.

A description would have let me nod and move on. The limb sent me to a file and the file answered.

**Where it is fuzzy, for the record.** Limb 1 did *not* settle OD-27 — a display convention that
every plan cites is reversible by editing one doc, yet costs code across every surface. Limb 2
settled it first. So the order of application matters and the file does not say to check the
cheapest limb first. Not a finding; a line worth adding if the gate is ever used in anger.

---

## Q2 — are the edges evidenced? Four of five yes, one not stated anywhere

No fabricated edge found. No edge points the wrong way. The five that carry the most ordering weight:

1. **OD-8 → OD-10 — CONFIRMED, verbatim.** The amended rule itself: *"Deleting a harness's file by
   the id the control plane minted (OD-10) is allowed. Rewriting its contents is not."* This is the
   strongest edge in the table; it names the child by id inside the parent's own text.
2. **OD-19, OD-21, OD-24 → OD-33 — CONFIRMED.** The OD-33 heading is literally *"phase 6 keeps
   LiteLLM (OD-21 stands)"*, and its defect 1 reads *"an OD-19/OD-24 project classified AU data
   only silently reaches Global"*. All three parents named in the child's own paragraph.
3. **OD-6, OD-13 → OD-30 — CONFIRMED, with a caveat (see M1).** OD-30's cell: *"the refresh token in
   Credential Manager (OD-6) makes later launches silent"* and *"the realm change is an ask for
   Panacea's update (OD-13 keeps Panacea separate)"*.
4. **OD-19 → OD-18 — CONFIRMED as text, asymmetric as a graph (see I1).** OD-18's cell: *"The one
   AWS advantage, Bedrock in Sydney, is reached from Azure through the inference gateway (OD-19)."*
   The implementer's own concern that this is a justification rather than a scheduling dependency is
   fair and I agree with it; it is still a stated link, so it stays.
5. **OD-15, OD-16 → OD-17 — NOT EVIDENCED.** This is the one edge I could not attach to a sentence.
   OD-17's cell (*"Design now, build next"*) names neither. Neither OD-15's nor OD-16's cell names
   OD-17. `05-design.md` cites OD-15 and OD-16 throughout as constraints on the design, but that
   document is the artifact OD-17 *scheduled*, so citing it is circular. All four were answered in
   one round on 2026-09-24, and round adjacency is not the stated bar (*"an edge is recorded only
   where this file, or a document it names, states the link"*). See I2.

One I checked because it looked thin and turned out fine: **OD-5 → OD-14**. Neither cell cites the
other by id — but `2026-09-24-team-collaboration/05-design.md:121` puts Fleet, the team control
plane, in *"our environment (OD-5)"*, and the register names that design folder. Stated in a
document the file names. Edge survives.

---

## I1 (Important) — the frontier is not self-consistent as an edge list

Four edges appear in **"Unblocked by"** only; the named parent's **"Unblocks"** cell does not name
the child. Read the table forwards and you get one graph; read it backwards and you get another.

| Edge | Child's row says | Parent's *Unblocks* cell says |
|---|---|---|
| OD-5 → OD-14 | `OD-5 (an environment to run them in)` | "Phase H; phase 8's growth…" — OD-14 absent |
| OD-19 → OD-18 | `OD-19 — Bedrock is reached across clouds…` | "OD-21; OD-24's…; OD-33; phase 6's re-scope…" — OD-18 absent |
| OD-15 → OD-17, OD-16 → OD-17 | `OD-14, OD-15, OD-16` | "Phase T's awareness surfaces" / "Phase T's Orchestrator channel" — OD-17 absent |
| OD-1 → OD-12, OD-31 → OD-12 | `Phases 3–7 being built (OD-1, ordered by OD-31)` | OD-31: "The paused work: … phases 3 to 7" — implies OD-12, names it nowhere |

The other ~18 edges are mirrored correctly, which is what makes these four read as slips rather than
a deliberate convention. The failure mode is concrete: an agent asked *"what does answering OD-5
release?"* reads OD-5's row, sees phase H and phase 8, and does not learn that the whole team-mode
cluster waits on it.

Fix: mirror the four, or state in the preamble that the two columns are independently sourced and
the union is the graph. Mirroring is ~4 cell edits.

## I2 (Important) — OD-17's two parents fail the file's own evidence bar

Detail in Q2 item 5. Not a fabrication — the dependency is structurally true, and nothing is
misordered by it, because all four were answered in the same half hour. But the file claims a bar
(*"only where this file, or a document it names, states the link"*) and this edge does not meet it,
which makes the bar unreliable for a reader who is trusting it on edges they cannot check. Either
cite something, or drop OD-15 and OD-16 from OD-17's row and keep OD-14.

## M1 (Minor) — the OD-6 → OD-30 edge carries two relations and distinguishes neither

The frontier excludes supersession by design and says so — yet OD-30 both **supersedes** OD-6
("directly against Entra") and **depends** on it (the Credential Manager store the silent relaunch
reuses). OD-30's row shows the bare pair `OD-6, OD-13`. OD-18's row models the fix: it carries its
reason inline. One clause on OD-30's cell ("OD-6 — the Credential Manager store, not the issuer,
which OD-30 supersedes") removes the ambiguity.

## M2 (Minor) — the audit undercounts: OD-27 also fails, on limb 2

Evidence in Q1 trial 2. The audit names six; OD-27 is a seventh by the audit's own standard, and its
row admits it with the word *"Still"*. The audit's framing ("the register is not uniform") survives —
this is completeness, not a false claim — but a reader who takes the six as the full list will
re-derive OD-27 the moment they apply the gate themselves, as I did on the first try.

## M3 (Minor) — OD-10 is the weakest of the six, and is asserted flatly

The audit says OD-10 fails *a real fork*: *"Forget deleting what it forgot is the only answer the
word allows."* The programme's own record disagrees in part. `2026-09-18-phase-2-go-contract.md:2259`
records the prior behaviour: *"Before, the purge left every…"* — so an alternative was **in force and
built**, and reversing it cost a fix round, a 500-and-keep-the-record failure path, and a seam handed
to phase 3. That reads like both a fork and an expensive undo. If OD-10 stays on the list, its row
should say *which* half is the note (the 2026-09-24 build status appended to it) rather than the
decision. The audit's structural observation 1 already makes exactly that distinction; the OD-10 row
just does not use it.

---

## Q3 — nothing renumbered, deleted or reworded. Ids are intact; lines are not.

- `git diff --numstat a109bf3e HEAD -- 2026-09-23-owner-decisions.md` → **114 insertions, 0
  deletions**. Count of removed content lines: **0**. Same for the pending-questions file: 43/0.
- Every `OD-n` id, heading and table row is byte-unchanged. Spot-checked the two most heavily cited:
  **OD-10** — eight citation sites in `2026-09-18-phase-2-go-contract.md` (§3.15, the `DELETE
  …?purge=true` row, the fix-round amendments) still describe exactly the row's text, including the
  `-p`-mode caveat and the phase-3 seam. **OD-8** — `PROGRAM.md`'s cross-cutting constraint and
  `03-techniques.md`'s K1 still match the amended rule word for word.
- Citations **by id**: all sound.
- Citations **by line**: all 41 broken. That is C1 above, and it is the only thing standing between
  this work and an approval.

## Q4 — the audit does not overstate

It names the limb each row misses and argues form, not substance. No row is re-litigated: OD-12 is
kept with its reason ("it defines what 'unverified' means in every report"), OD-28 is kept as worth
writing once, OD-35's real content is identified rather than dismissed. The closing disclaimer —
*"Not audited: whether any decision was correctly answered"* — is honest and is actually held
throughout. The two structural observations are the most valuable part of the section and are
correctly ranked above the six rows.

The only pressure on that verdict is M3 (OD-10 asserted too flatly) and, in the other direction, M2
(one row missed). Both are small. Nothing in the section relitigates an owner answer, which was the
risk worth checking.

## On the open question left to the owner

Correctly left open, and the three options are fairly drawn — (a) and (c) are both live and (b) is
honestly costed against itself. One observation the owner may want with it: **M2 raises the count.**
If OD-27 also fails, the "notes" set is at least seven of 35, and option (c)'s one-off pass is over
seven rows, not six. That does not change which option is right; it slightly raises the price of (a),
because the diluted fraction is a fifth of the register rather than a sixth.

---

**Task quality: Needs fixes**
