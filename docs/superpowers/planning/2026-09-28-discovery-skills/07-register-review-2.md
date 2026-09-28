# Independent re-review — did the anchors actually come back?

Reviewed 2026-09-29, read-only, main tree on `feat/phase-2-cockpit`. Subject: the fix round's four
commits — `8e36b6e7` (the move), `a5a60ee3` (frontier mirrors + audit), `343a0276` (six anchor
bumps), `e365dbb4` (report) — against the first review's C1, I1, I2 and three Minors. Nothing was
changed except this file.

**Verdict: no Critical, no Important, five Minor.** C1 is fixed and the fix is provably complete —
not sampled-complete. I1 and I2 are done. Every Minor below is either a record-keeping slip in the
round's own report or a defect that pre-dates the round and is named here so it is not lost.

---

## 1. Anchors restored — CONFIRMED whole-file, not by sample

The implementer's stronger claim holds. `head -266` of the current file against `a109bf3e`'s copy:

```
58a41b56d5a895cb7e5f2deee1ca9c5e  a109bf3e:2026-09-23-owner-decisions.md   (266 lines)
58a41b56d5a895cb7e5f2deee1ca9c5e  HEAD, first 266 lines of 398
```

`diff` returns empty. So every pre-existing line anchor is exact by construction, not only the five
the review sampled. The eleven lines I was asked to print are identical character for character:

| Line | Old = new |
|---|---|
| 13 | `\| **OD-2** \| 3-D1: what a worktree is in git \| **A local clone per worktree** \|…` |
| 21 | `\| **OD-10** \| What Forget does to a persisted transcript \| **Forget deletes it** \|…` |
| 73–79 | the OD-8 blockquote, all seven lines, ending `> (OD-10) is allowed. Rewriting its contents is not.` |
| 114 | `## Team mode — owner decisions of 2026-09-24 (evening), OD-14 to OD-17` |
| 140 | `\| **OD-19** \| How agents get inference \|…` |

The review's offset figure was wrong and the report's correction is right: old line 12 (`OD-1`)
stood at 49 in `1cb8be9a`. The shift was **37**, not 36. Verified directly against both blobs.

## 2. Five citing sites re-opened — all resolve

Sampled across five documents, deliberately including the two **binding** citations and three the
sibling chain re-anchored today.

| Citing site | Cites | What the target line now says | |
|---|---|---|---|
| `03-techniques.md:45` — constraint K1, *"unmodified and unwrapped; nothing derives meaning from PTY bytes"* | `:73-79` | `:73` — *"**Harnesses run unmodified and unwrapped, and no feature derives meaning from PTY bytes.**"* | ✔ |
| `04-enterprise-constraints.md:121` — quotes *"no feature may require the adapter to function"* | `:73-79` | `:77-78` — *"No feature may *require* the adapter to function."* | ✔ |
| `05-design.md:3` — *"Revised 2026-09-25 for OD-18 to OD-20"* | `:130-141` | `:130` is the header *"## Cloud, inference and local mode — owner decisions of 2026-09-25, OD-18 to OD-20"*; `:139/140/141` are OD-18/19/20 | ✔ range exact at both ends |
| `04-internal-fit.md:585` — *"lettered for the reason Phase H was … no phase number moves"* | `:40-43` | *"It is lettered so that no existing phase number moves: the cockpit's copy and three prototypes cite phase numbers…"* | ✔ |
| `04-internal-fit.md:301` — *"no user steers another user's agent"* (OD-16) | `:127` | `:127` is OD-16 and contains the quoted clause | ✔ |
| `06-critique-feasibility.md:268` — *"Kata on EKS needs metal instances"* | `:139` | `:139` is OD-18 and contains the quote | ✔ |

The six anchors bumped today by `343a0276` also land: `CONTEXT.md:47` → `:13` = OD-2;
`CONTEXT.md:171` → `:14,16` = OD-3, OD-5; `CONTEXT.md:242-243` → `:152` = OD-24 and `:140` = OD-19;
`CONTEXT.md:255` → `:127` = OD-16; `08-context-report.md:38,48,49` likewise. Nothing in the tree now
carries a post-insert anchor.

## 3. OD-17 removed, four mirrors present — and the whole table re-walked

**I2 — removed, with its reasoning left behind.** `OD-17`'s row is now `| **OD-17** | OD-14 | Phase
T's plan (with OD-29)… |`. Neither OD-15's nor OD-16's *Unblocks* names OD-17 any more, and a
seven-line **"Considered and not recorded: OD-15, OD-16 → OD-17"** note under the table states the
bar it failed and why `05-design.md` cannot carry it. That is the right shape — an absence with a
reason does not get re-added by the next reader on intuition.

**I1 — all four mirrors exist.**

| Edge | Parent's *Unblocks* now reads |
|---|---|
| OD-5 → OD-14 | *"OD-14 — team agents need an environment to run in, and `…05-design.md:121` puts Fleet in 'our environment (OD-5)'"* |
| OD-19 → OD-18 | *"OD-18 (the gateway is why AWS's one advantage does not decide the cloud); OD-21; …"* |
| OD-1 → OD-12 | *"…; OD-12, whose one hands-on session waits on those builds; …"* |
| OD-31 → OD-12 | *"…; OD-12, which cannot happen until the phases it orders are built"* |

I did not stop at the four. I walked all 29 rows in both directions. Every *Unblocked by* entry has a
matching *Unblocks* entry and every OD named in an *Unblocks* cell appears as a child, with the one
exception at M2 below. The preamble's counts are internally consistent: 29 rows in the table, six
named as carrying none (OD-2, OD-4, OD-7, OD-9, OD-28, OD-35), 29 + 6 = 35.

## 4. Deletions: every one accounted for, nothing renumbered

Cumulative, `a109bf3e` → `HEAD`:

```
132  0  docs/superpowers/planning/2026-09-23-owner-decisions.md
 44  0  docs/superpowers/planning/2026-09-25-owner-questions-pending.md
```

**Zero deletions against the pre-round file.** Nothing that existed before this work was removed,
which — with the byte-identity above — settles "nothing renumbered, deleted or reworded" for every
pre-existing row without spot-checking any of them.

Within the round, the deletions are all replacements of the round's own new text:

- **`8e36b6e7` — 41 / 37.** Set-comparing removed against added lines: 32 of the 37 are the relocated
  gate lines, byte-identical. The other 5 are two deliberate rewrites — the 3-line *"It is not
  retrospective"* bullet, replaced by the 8-line ids-versus-lines version (this **is** the C1 fix),
  and the 2-line pointer *"recorded once, near the end of this file"*, replaced by the 1-line *"The
  dependency frontier, immediately below…"*, which is now literally true.
- **`a5a60ee3` — 23 / 9.** The nine: the OD-1, OD-5, OD-17, OD-19, OD-30 and OD-31 frontier rows
  (four mirrors, the OD-17 removal, M1's relation clause); *"Six rows"* → *"Seven rows"*; the OD-10
  audit row (M3); *"the six rows"* → *"the seven rows"*. The 14 net-new lines are the mirror-invariant
  paragraph, the OD-27 audit row and the not-recorded note. Plus 4 / 3 on the pending-questions file:
  six → seven twice, and the dilution restated as a fifth.
- **`343a0276` — 5 / 5 and 2 / 2.** Pure anchor bumps.
- **`e365dbb4` — 96 / 0.** Report only.

No file outside `docs/superpowers/planning/**`, `CONTEXT.md` and the round's own report was touched
by any of the four commits. `control-plane/**`, `sandboxd/**` and `desktop/**` appear in no diff.

---

## M1 (Minor) — the report's own verification table is wrong about the move

`07-register-report.md:135` reads *"`git diff --numstat` for the move | 37 / 37 — a pure relocation,
no content change"*. `git show --numstat 8e36b6e7` gives **41 / 37**, and content did change in that
same commit — the gate bullet was rewritten there, as the report itself says four paragraphs later.
The measurement was presumably taken before the bullet edit and committed together with it. The
artifact is correct; the row that a future auditor would trust is not. It should read 41 / 37, with
the four-line delta named as the rewritten bullet.

## M2 (Minor) — the new mirror invariant has one cell that does not literally hold

The preamble now asserts *"Every edge appears twice… Read forwards or backwards, the graph is the
same."* OD-29's *Unblocked by* is `OD-17, OD-25`. OD-25 mirrors it (*"OD-29's phase W half"*). OD-17's
*Unblocks* reads *"Phase T's plan (with OD-29)"* — which, under the file's own co-parent convention
(OD-3's *Unblocks* says *"Phase H, with OD-5"* while OD-5's *Unblocked by* is `—`), reads as OD-29
standing **beside** OD-17, not below it. One clause fixes it, or OD-29's parent list drops to OD-25.

Two adjacent reading hazards, same class, no fix needed because the preamble defuses them: OD-6's
*Unblocks* mentions *"OD-4's two host tokens"* and OD-26's and OD-29's mention OD-35, while the
preamble lists OD-4 and OD-35 as carrying no edges. A reader scanning the column for `OD-n` could
count either as an edge.

## M3 (Minor) — three anchor errors that pre-date this round

Found while sampling, all byte-identical at `a109bf3e`, so **not caused by this work** and not this
round's to fix. Recorded so they are not lost:

- `06-critique-feasibility.md:245` cites `owner-decisions.md:125` for *OD-15's "calm"*. `:125` is
  OD-14; OD-15 is `:126`.
- `2026-09-25-identity-keycloak-broker.md:4` — *"Source decision: `…owner-decisions.md:166` (OD-30)"*.
  `:166` is OD-31; OD-30 is `:167`.
- `2026-09-25-identity-keycloak-broker.md:571` cites `owner-decisions.md:186-188` for *"the PROGRAM.md
  bullet"*. `:186-188` is OD-32's open-source headline. This looks like a `PROGRAM.md` anchor written
  under the wrong filename.

## M4 (Minor) — "41 citations across 12 documents" undercounts the main tree

The gate's new sentence states the exposure as 41 across 12 documents, which is what the first review
counted under `docs/superpowers/planning/`. The main tree also carries **12 more line anchors in 7
files under `.superpowers/sdd/`** (`fork-carries-review.md`, the three `reconcile-25-26-*` files,
`od10-forget-review-c-2.md`, `task-24-review.md`, `facts-app-identity.md`). They were broken too and
are restored for free by the same fix. The direction is safe — the new forward rule protects every
one of them — but the number in the file is smaller than the hazard it describes.

## M5 (Minor) — one stale verification claim left in the sibling chain's review

`08-context-review.md` still records *"✔ OD-2 is line 50"* as a checked fact. It is a dated review
record, so leaving it is defensible, but it is the one place in the tree that still asserts the
post-insert convention as true. The sibling chain owns that file; a one-word strike-through or a
"(line 50 at review time; 13 after `8e36b6e7`)" parenthetical would close it.

---

## On the two concerns carried forward

The implementer's decision **not** to adopt the review's fuzzy-limb observation — apply the cheapest
limb first — is correct as a matter of scope. It changes how the gate is run, which is the owner's
call, and it is recorded rather than decided. I agree with leaving it.

The concern about sibling worktrees holding older copies of `owner-decisions.md` with the same
citations is real and is **not** made worse by this round: because the file came back byte-identical
below line 266, those branches' anchors resolve correctly against both their own copy and against
`feat/phase-2-cockpit` when they take it. The round closed the hazard rather than deferring it.

**Task quality: Approved**
