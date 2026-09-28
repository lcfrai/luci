# OD-36 folded into the owner-decision register: report

**Verdict: done.** OD-36 is in `docs/superpowers/planning/2026-09-23-owner-decisions.md`, with its
frontier row and the mirror in OD-3's row. No cited line moved, and lines 1-266 are still
byte-identical to `a109bf3e`.

The decision: Discover gains a project glossary and a project decision record, and each is its own
Hub record that references a brief revision, not a field of the brief's content. The source is
`10-decision-terms-beside-the-brief.md`.

## 1. Where OD-36 went, and why nothing cited moved

- **Placement:** a new last section, *The project glossary and decision record — 2026-09-28,
  OD-36* (register `:401-430`), plus one frontier row (`:359`). The register's own rule at `:303`
  says anything added later goes below the tables.
- **Inbound citations:** `grep -o "owner-decisions.md:[0-9]*"` over the main tree finds none above
  `:186`. In the worktrees the highest is `phase-6` tasks 26 and 27, at `:203-222` (OD-33). Nothing
  cites line 267 or below, by number or by range. I compared every distinct cited line number at
  HEAD with the edited file: all unchanged.
- **What did shift:** HEAD lines 359-398 (the *Considered and not recorded* paragraph and the
  backwards audit) are now 360-399, byte-identical, one line down. Nothing cites them.
- **Not after OD-35 (`:253`):** that would move 255-266, which nothing cites, but it would break the
  266-line identity that `2af32e54` verified. **Not before *What earns an OD* (`:267`):** allowed,
  but then every later OD would push the gate, frontier and audit down again. At the end of the
  file, the next decision can also go in without moving anything.

## 2. The row, in the register's own format

- It uses the four-column `Id | Question | Answer | Consequence` row. I read OD-29 to OD-31
  (`:166-168`, the latest table rows) and OD-32 to OD-35 (`:189-253`) first.
- The first sentence of the Answer cell is the decision itself (*"Beside it."*), as the backwards
  audit's observation 1 (`:390`) asks of new rows.
- **Left out on purpose:** the example key `term:tm-7f2a@2` (`10-…:14`). Its id has 4 hex digits,
  where a brief key has 16 (`CONTEXT.md:107`; `11b-facts-hub.md`, in flight, notes the same). The
  key grammar belongs to the Hub contract, and the form this key takes is for design round 2 to
  settle, so the register does not fix it.

## 3. The three-limb gate

Recorded in the register at `:411-425`. OD-36 is the first row admitted through the gate. The
backwards audit covered the 35 earlier rows and does not include it.

1. **Expensive to undo:** reversing it after it is built folds records back into brief content, each
   fold minting a brief revision, and three built parts (the Materialiser, `Ref` and the Discover
   lens) change a second time. Before it is built, reversing it re-plans design round 2.
2. **Not derivable:** no file says where a project's vocabulary lives, and the obvious guess was
   wrong. `05-design.md`, a whole design round, put the terms inside the content.
3. **A real fork:** inside the content was defensible (one object, one revision history). Beside
   gives that up for a brief revision *plus* digests, and built phase 4 code moves, at a cost the
   owner accepted.

## 4. The dependency frontier, both directions

- **OD-36 row (`:359`):** Unblocked by OD-3. It unblocks Discover's design round 2 and, after that,
  the rework of phase 4 tasks 09, 06 and 18 to 21.
- **Mirror (`:331`):** OD-3's *Unblocks* cell now ends with OD-36. OD-36's *Unblocks* side names no
  `OD`, so no second mirror is owed.
- **Considered and not recorded:** OD-5 → OD-36 (reason at `:427-430`). OD-36 depends on the brief
  being shared (OD-3), not on where the Hub is hosted (OD-5).
- **Counts kept true, each edited in place:** `:316` now reads "30 of the 36". `:270` now says
  thirty-five decisions *were* on the page before the gate existed, because "are" would be false at
  36. `:303-305` point to the last section and no longer say the audit is "at the end of this file".

## 5. The status line of `10-decision-terms-beside-the-brief.md`

Lines 3-6 were rewritten in place to say the decision is recorded as OD-36. It is still four lines,
and the file is 85 lines before and after. Lines 14 and 54, which `11b-facts-hub.md:83` and `:50`
cite, are unchanged.

## 6. Verification

The script ran through `claudezsh`, with these results. The first 266 lines `cmp` equal to
`git show a109bf3e:…`. Diff hunks against HEAD: in place at 270, 303-305, 316 and 331; inserted
at 359; appended at 400-430. HEAD 359-398 is identical to 360-399 now. No cited line changed.

## 7. What is unverified, and what is left open

- The owner's words are quoted as `10-…` records them. I did not see the owner's original message.
- "Discover captures no language today" is the decision note's claim ("verified twice"). I did not
  re-check it against the phase 4 code.
- The phase 4 task numbers were checked against plan headings only (the phase-4 worktree's
  `task-09.md` heading, `task-06.md:123` `type Ref struct`, and the task 18-21 headings), not
  against built code.
- **Not done, and no owner question:** `CONTEXT.md` has no entry for *project glossary* or *project
  decision record*, and `CONTEXT.md:303` maps "ADR directory" to `OD-n`. When design round 2 names
  these records, `CONTEXT.md` needs an entry that keeps a per-project decision record apart from
  this register. I did not add one now, because only OD-36 names them so far.
