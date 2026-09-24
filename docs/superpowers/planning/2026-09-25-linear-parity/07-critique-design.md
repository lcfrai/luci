# 07 — Adversarial design critique of `05-design.md`

**Verdict: not yet better than Linear.** As specified, the work layer reads as a disciplined
Linear clone in teal. It is not generated slop: the craft floor is high and the built tokens pass
the detector clean. Five Critical fixes turn it into something Linear structurally cannot be.
The first two matter most: make the brief's criteria the default layout of the Work surface (C1),
and stop rows moving under the reader (C2). **5 Critical · 10 Important · 10 Minor.**

Reviewer stance: a product-design lead asked one question. *Put the Work surface next to Linear.
Does it look and feel better, like a clone, or like generated slop?* Everything below is judged
against `02-linear-design-language.md` (02), `docs/DESIGN-SYSTEM.md` (DS), `docs/AGENTIC-UX.md`
(AX) and the owner decisions (OD-14 to OD-20).

**Scope.** The UX and design-language parts of 05: §4.1 to §4.6, the Desk rows in §4.2, the key
table in §4.3, the uplift table in §4.4, and every part of §2 and §5 that decides what the user
sees first. The object model, parity tally and delivery sizes are out of scope unless they
change what is on screen.

**Method.** Read 05 §4 in full; 02 Part C in full; DS in full; AX §4 to §10. Cross-checked the
claims 05 inherits from 02 against DS and AX. Applied the impeccable skill's critique lens
(visual hierarchy, typography, colour, motion, keyboard, accessibility, anti-patterns). No UI
exists for the work layer, so **nothing here is a visual observation**: every finding is about
the specification. Visual acceptance stays with the owner's eyes (`CLAUDE.md`).

**Clean room.** No Linear asset, copy or code was opened. Where a finding compares us with
Linear's visible conventions (key letters, priority icons), the comparison is from general
product knowledge and is marked **UNVERIFIED**; 02 D4 records that Linear's shortcut page
returned 404, and this review did not re-fetch it.

`DEGRADED: single-context` (the impeccable critique normally splits a design review and a
detector pass into two isolated sub-agents; this run's orchestrator forbids sub-agents, and the
target is a written spec, so there is no page to inspect. The detector was run once over the
built tokens as the only incumbent visual truth; the Detector section records what it found.
Questions skipped: this is a return value to an orchestrator, and the owner questions this
critique raises are in the fixes.)

---

## Critical: fix before the W1 prototype is drawn

### C1. The Work surface is Linear's list view in teal

**Where:** 05 §4.1 (the drawn Work surface), §4.4 (C13 adopted as written), §5.3 row 8.

**What is wrong.** Take the teal, the grain and the mono away and the drawn surface is Linear's
default issue list: a cycle header, rows grouped by workflow state (`IN REVIEW 2`, `STARTED 3`,
`TODO 4`), a team-prefixed id (`EP-142`), a title, an assignee name, meta on the right, `⌘K`, `G`
sequences and number-key triage. The only structural differences on a row are a six-character
mono hash (`┊c-91e2`) and an agent line. From two metres away, the reader sees Linear.

The not-a-clone test in 02 C13 cannot catch this, because it checks the **skin** (face, accent,
grain, a filling icon, glass) and never the **skeleton**. A screen can pass all ten checks and
still be a copy. That is the worst of both outcomes: it invites the comparison and then loses it,
because Linear has had six years to polish that exact layout.

**Why it matters for "better than Linear".** Our one structural advantage is the brief. Linear
has no criteria to group by, so a surface whose layout *is* the provenance thread is the one thing
Linear cannot copy without becoming a different product (02 C2 says the same about the signature,
then 05 shrinks the signature to a chip).

**Fix.**
1. Make **"by criterion"** the default grouping of the Work surface and the Work lens. The group
   header is the criterion in sans, in words (*"Uploads survive a dropped connection"*), with its
   convergence word from AX Principle 10 (`settled` · `open` · `contested` · `drifting`). Its
   issues sit under it, joined by `thread.line`. Issues that serve no criterion collect in one
   plain group at the end, *"Serves no criterion"*, which is itself a useful fact.
2. The workflow state becomes a glyph column inside the row, not the grouping.
3. "Current cycle, by state" survives as a saved view for people who want Linear's shape. It is
   not the default and it is not what the prototype shows first.
4. Add one **structural** line to C13's second list: *"a work screen whose default grouping is
   workflow state"* fails the review.
5. Name the criterion in words wherever it appears on a row. `c-91e2` is a machine id; by 05's
   own `voice.split` it belongs in peek and in mono, not as the row's only link to the brief.

### C2. Rows will jump under the reader, at exactly the moment the demo happens

**Where:** 05 §4.1 (grouped by state), §4.4 C7 row, §5.3 (the W1 demo).

**What is wrong.** A list grouped by state is a list whose rows **change group** whenever a state
changes. In Linear a person makes most changes, so a jump is usually their own. Here agents and
teammates change state all day: `started → in_review → landed → done` happen without the reader,
and §5.3's lovable demo is literally *"the issue turns `done` by itself"*. 05 covers two cases:
new rows (`arrivals.hold`) and an explicit re-sort (FLIP). It covers neither a row that **moves**
between groups nor a row that **leaves** the view (done issues dropping out of a cycle list).
AX Principle 11 and anti-pattern 7 allow zero pixels of layout shift from polling.

**Fix.** Name a third move beside `arrivals.hold`:
- **`moves.hold`:** a row whose group changes stays where it is. It takes its new glyph, flashes
  over `--dur-flash`, and shows one mono line of cause (`→ done · criterion settled · 14:02`). It
  relocates only on explicit refresh (`N`, which then means "apply held changes"), on leaving the
  view, or when the reader's focus is not in that section.
- **Departures** leave a one-line ghost in place (`EP-142 left this view · done`) until refresh.
- The group header's plain size (Q1 option A) shows what is held: `IN REVIEW 2 · 1 moving`.
- Put it in the W1 prototype, because the demo in §5.3 exercises it on its first run.

### C3. The agent cell on a Work row breaks the rules it cites

**Where:** 05 §4.1 mockup (`● 2 agents · last 3s`, `● 1 agent · Alice's`), §4.2 RUNNING row,
§4.4 C10 row.

**What is wrong,** checked against the documents 05 itself adopts:
- **No meaning line.** AX Principle 3: every agent carries STATE, MEANING and NEXT, and *"fails at
  zero meaning line"*. The drawn cell is state only. 05 §4.4 says the cell is `row.three`
  *"compressed to one line"*, but what survived the compression is STATE, the one line that tells
  a person least.
- **A count outside the queue.** `2 agents` is a count on a row. 05 §4.4 rule 3 says *"no count
  outside a group header or the Desk"*; AX anti-pattern 10 says the same more strictly.
- **An aggregate with no single source.** AX Principle 7: every dot and number click-throughs to
  the span that produced it. A dot standing for two agents has two sources and so none.
- **`who.label` is not applied.** 02 C10 requires `alice · worker 2 (agent)` in mono on every
  agent string. The mockup prints `Alice` in the assignee column and `Alice's` in the agent cell:
  the same name twice, and still no class.
- **The Desk disagrees with AX.** §4.2 gives RUNNING *"one line each"*; AX §5 says the card is
  *"three lines, always three"*. 05 does not say which wins.

**Fix.**
1. A Work row carries **one** agent line: the lead agent's MEANING in sans, truncated at the row
   end, prefixed by `who.label` in mono. Other agents on the issue are reached by `Space`
   (`peek.span`), not counted on the row.
2. The dot beside it is that one agent's state, so it has one span.
3. State in 05 which shape the Desk uses. Recommend: three-line cards for agents that changed
   since you last looked; one line (STATE plus MEANING) for the steady ones. Record it as an
   AX §5 amendment for the owner, because it changes a binding rule.

### C4. With reduced motion on, "running" and "idle" differ by colour alone

**Where:** 05 §4.1 (`●` on every Work row), §4.4 C7 and C10 (`fleet.beat`, `--dur-flash`);
built code `desktop/src/styles/tokens.css:100-104` and `:173-183`.

**What is wrong.** The built tokens say it plainly: idle and running are **both discs**, and
*"that pair is carried by colour … and by motion: idle is still, running breathes"*
(tokens.css:102-104). The reduced-motion floor (tokens.css:174-183) sets every animation to
0.01ms and one iteration, so the breath stops. What remains is `--run` `#43b95e` against `--idle`
`#6c645d`. Computed from those hex values, their luminance ratio is about **2.3:1**, under the
3:1 that WCAG's techniques ask of a difference carried by colour. For a user who has reduced
motion on and red-green colour blindness, which is not a rare pairing, every agent dot on the
Work surface reads the same. The same floor also removes the `--dur-flash` state-change flash,
so a state change becomes silent.

This is the product's core signal: "is my agent alive". 05 builds its whole answer to OD-15's
many-live-agents problem on it (`fleet.beat`) and puts the dot on every issue row.

**Fix.**
1. Give **running its own shape**, for example a disc with a 1px open ring around it, so shape
   separates all four of running, idle, exited and failed. DESIGN-SYSTEM already requires *"each
   carries a shape as well as a colour"*; the built set falls short of it for one pair.
2. Under reduced motion, replace the flash with a **static mark** (a 2px leading tick in `--ink`)
   that stays until the row is seen. Seen-ness already exists as the read cursor (AX §6.1).
3. Add *reduced motion × deuteranopia simulation* to `stress.sets` (02 C12), and make it one of
   the prototype's review screens.

The contrast figure is computed, not measured on a display: **UNVERIFIED** until checked in
WebView2 with a colour-blindness simulator.

### C5. W1's Desk is calm because it hides failures

**Where:** 05 §5.3 row 13 (*"a minimal Desk with NEEDS YOU and LANDED"*), §4.2, §5.6.

**What is wrong.** The calm human summary is the product's answer to the owner's "magic". AX §6.2
names four delta classes that must never merge: LANDED, WAITING, FAILED and ASSUMED. W1's Desk
carries two of them. A lane bound to your issue that fails in W1 shows only as a `--fail` tint on
a Work row you may not be looking at, and there is no RUNNING section, so the zero state *"7
agents on 4 issues"* has nothing behind it. A Desk that says "nothing needs you" while a lane has
failed is not calm. It is wrong, and the first time a user catches it, they stop trusting the
empty state, which AX §5 calls the design target.

There is a second effect. With the Desk cut down to two sections, the first thing a W1 user
lives in is the Work surface, which is the Linear-shaped part (C1). The demo in §5.3 also ends on
the list, not the Desk.

**Fix.**
1. Put **FAILED** into W1's Desk. It is one row type, with *Restart from checkpoint* and
   *Abandon*, and the existing lane-failure event feeds it. Estimate about 0.5 d.
2. Put **RUNNING** in as one line per agent (C3's shape). Estimate about 0.5 d.
3. End the §5.3 demo on the Desk: *"Nothing needs you. EP-142 shipped to production at 14:02."*
   That is the moment Linear cannot show, and it should be the last frame the owner sees.

---

## Important: fix in the prototype or in 05 before W1's plan

### I1. The lit thread and a selected row look the same

**Where:** 05 §4.1 (*"hovering it lights every issue, frame and lane on the same criterion"*),
02 C2 `thread.lit` (*"rings every artifact … in `--accent-ring`"*); built
`tokens.css:68` (`--accent-ring` is *"the ring on a selected row"*).

Hover `┊c-91e2` and four rows gain the ring that means *selected*. With `X` and `Shift+X` building
a selection for the bulk bar, the user cannot tell which rows the bulk action will touch. One
token carries two meanings, which is the failure DESIGN-SYSTEM names for colour.

**Fix.** `thread.lit` brightens the `thread.node` and draws `thread.line` in the row's leading
gutter; it never rings a row. Selection keeps the ring and gains a leading check glyph, so it is a
shape as well. Test both at once in the prototype: three rows selected, a fourth lit.

### I2. The Work surface shows teammates' agents, where OD-15 asks for a calm person summary

**Where:** 05 §4.1 (`● 1 agent · Alice's` on Alice's row), §1 row L-182 (*"filter Alice's
agents"*).

OD-15 splits the audience: agents get full awareness; **humans** get *"who is working, a one-line
summary of each person's work, and the files being touched, without clutter"*. As drawn, every
teammate's issue carries a live dot and its agent count. With eight teammates at three agents
each, the Work surface breathes with 24 dots, 21 of which the reader cannot act on (OD-16: nobody
steers another user's agent). That is agent state shown to a human, the thing OD-15 says not to
do.

**Fix.** On the Work surface, **your** issues show your agent line (C3). A **teammate's** issue
shows the person and one sans line of what they are doing, with no dot and no count:
*"Alice · chunked upload API · touching `upload/*.go`"*. Their fleet is one `Space` away in
`peek.person`. The heartbeat then carries only what the reader can direct, which is also what
makes "calm" true.

### I3. The keyboard model: one sound rule, four holes

**Where:** 05 §4.3.

*"Lower case edits the record; `Shift` hands work to your agents"* is the best idea in §4.3.
A reader can tell from the key alone whether an agent will run. Four things undercut it.

1. **`Shift+X` breaks the rule.** It is range select, a record act. Move range select to
   `X` then `↓`/`↑` extend, or to `Shift+↓`/`Shift+↑`, which is the platform convention anyway.
2. **`S` and `Shift+S` are one modifier apart**, and one of them bills a cloud session (OD-19's
   org inference). The handler must match `event.code === "KeyS"` **and** `event.shiftKey`, never
   `event.key === "S"`, or Caps Lock turns *pick state* into *start an agent*. State it in 05.
   §4.6 already makes Start reversible until the first write; make that visible as the Undo chip
   on the row, so a slip costs one keystroke.
3. **`G` and `O` sequences fire "outside a terminal"**, which contradicts rule 1 (*"only when
   focus is in a list"*) and also reaches text fields. WCAG 2.1.4 (Character Key Shortcuts,
   level A) requires single-character shortcuts to be turn-off-able or remappable unless they
   are active only while their component has focus. List-scoped keys pass; global sequences do
   not. Add a setting that turns single-key shortcuts off, and scope `G`/`O` to non-text focus.
   Speech-input users are the people this protects.
4. **There is no assignee key.** §4.6 lists assignee among the 16ms acts; the table has `I`
   (assign to me) only, because `A` is taken by approve. Give assignee a letter, or say that it
   is palette-only.

Also, be honest about the letters. §4.3 says *"we … write our own letters"*, but `C` create,
`X` select, `S` `P` `L` `E` properties, `I` assign to me, and `G`/`O` sequences are, from general
product knowledge, Linear's own public letters (**UNVERIFIED**; 02 D4 got a 404 on its shortcut
page). Converging is **fine and probably right**: key letters are function, not expression, and
muscle memory is what a migrating team brings. Say it is a deliberate choice, check the table
against Linear's public help page once it is reachable, and decide rather than drift.

### I4. Amber for "urgent" spends the one colour that means "needs you"

**Where:** 05 §4.4 work-specific rule 2 (*"urgent, which is `--need` only on the assignee's own
rows"*).

Two problems.
1. **Amber gets two meanings.** `--need` means *an agent is waiting on you* (DESIGN-SYSTEM's
   locked palette). An urgent issue may need nobody yet. 02 C1 says *"status must mean the same
   thing on every screen"*, and 02 C10's `lease.ring` rule exists precisely to keep `--need` to
   one meaning. Rule 2 undoes both.
2. **It reproduces Linear's most recognisable glyph set.** Priority as signal-bar shapes with an
   orange urgent mark is, from general product knowledge, Linear's own convention
   (**UNVERIFIED**). It is the first thing a Linear user's eye goes to.

**Fix.** Urgent is a **shape in ink** (our drawn set, 02 C6), not a colour. If an urgent issue
really needs the person, it enters NEEDS YOU on the Desk, and amber is earned there. Draw priority
as something other than bars, for example a notch depth on the state glyph's leading edge, so the
row needs one glyph, not two.

### I5. The most repeated things on the screen are still placeholders

**Where:** 05 §4.1 (*"state glyphs … are placeholders"*), 02 C6 (icon family **UNVERIFIED**),
§6 Q10 (mono licence open), `tokens.css:144-152` (Geist and the licensed mono are not shipped;
the app runs on Segoe UI Variable and Cascadia Mono), §5.3 row 7 (prototype, 1.5 d).

The state glyph appears on every row. The type sets every word. Neither is decided, and the W1
prototype is budgeted at 1.5 days. A prototype judged on *"prettier than Linear"* while wearing
placeholder glyphs and fallback fonts repeats the first prototype's failure: it will look
generated because its most visible parts are generic.

**Fix.** Gate the prototype, not the code:
1. Q10 answered (the mono), and the sans chosen (I6).
2. The ~12 domain glyphs from 02 C6 drawn first, the state set and priority notch included,
   at 14px and 16px, in both grounds.
3. Fonts bundled as files, as 02 C3 already requires.
4. Raise row 7 to **about 3 d**, and review it on a KPMG laptop at 125% scaling, in `graphite`
   and `paper`, behind `flag.compare`.

### I6. Geist is no longer the safe choice; it is the generator's default

**Where:** 02 C3 (*"keep the DESIGN-SYSTEM pairing, Geist Sans + one licensed mono"*), 05 §4.4.

DESIGN-SYSTEM already warns that *"Inter and Geist are both drifting toward the new default"*.
By late 2024, Geist and Geist Mono were, from general knowledge, the default fonts in
`create-next-app` and common in v0 output (**UNVERIFIED** this session). That is the toolchain
behind most generated interfaces in 2026. C13 bans Inter, the older tell, and blesses the newer
one. The mono then has to carry the whole typographic difference at 11px (I7).

**Options, recommendation first.** Put two of them side by side in `flag.compare` and let the
owner's eyes decide:
1. **Segoe UI Variable on Windows** (already shipping in `tokens.css:151`, native, optically sized,
   nobody's generator default) with the licensed mono. It costs nothing and matches the platform
   most users are on. The macOS fallback needs a choice too.
2. **An OFL sans with its own character.** Atkinson Hyperlegible Next is one candidate: it was
   drawn for low-vision legibility, so it answers the accessibility brief as well as the
   differentiation one. Licence and weight range to be checked: **UNVERIFIED**.
3. Geist, as 02 proposes, accepting the resemblance.

### I7. The evidence is set smallest

**Where:** 02 C3 `voice.split` and the `--t-micro` row (11/14, mono), adopted by 05 §4.4 C3.

`voice.split` is right: claims in sans, facts in mono. But it puts **every** machine fact
(criterion ids, costs, durations, exit codes, span refs, route badges) at 11px, the smallest size
in the scale. Linear can shrink an id because an id is secondary there. Here the facts **are** the
product: *"done means proven"* is argued in mono. At 100% scaling on WebView2, 11px mono is about
8pt, and it will be the least legible text on the screen, sitting next to the claim it proves.

**Fix.** Machine facts at 12px mono (the `--t-meta` size, a new `--t-fact` token). Keep 11px for
`kbd` hints only. Judge it at 100% scaling, where it is hardest, and set `ink-2` as the floor for
any fact (`--ink-2` is 7.63:1; `--ink-3` at 3.43:1 is under WCAG 1.4.3's 4.5:1 for text and must
never carry a fact).

### I8. Windows contrast themes will erase the rings, and one escalation is colour only

**Where:** 05 §4.4 C1 (four app themes), 02 C10 `lease.ring`; built `tokens.css:118-122` (every
`--e-*` elevation is a `box-shadow`). A grep of `desktop/src` finds no `forced-colors` rule.

KPMG laptops run Windows. When a user turns on a Windows contrast theme, WebView2 applies
`forced-colors: active`, which drops `box-shadow`. Every elevation ring, the `thread.node` (drawn
with `--e-hair` geometry), the `lease.ring` and the selected-row ring vanish. The app's own `-hc`
themes do not replace the OS setting, because the user chose the OS one. Separately, `lease.ring`
*"turns `--need` only when a `decide` escalation exists"*: a colour change with no change of shape,
which DESIGN-SYSTEM forbids for status.

**Fix.**
1. A `@media (forced-colors: active)` layer: rings become `outline` or `border` in system colours
   (`CanvasText`, `Highlight`), glyphs are SVG in `currentColor`, and every state keeps its shape.
   The built focus ring is already an `outline` (`tokens.css:187-191`), so it survives. Keep it.
2. Give the escalated lease a shape (the needs-you shape on the chip), not only amber.
3. Add a Windows contrast theme to `stress.sets` item 2.

### I9. No model for assistive technology on a live fleet

**Where:** 05 §4.1, §4.4 C7 and C10, §4.6 (`--undo-window` 6000ms); none of 05 or 02 names a
screen reader.

Thirty agents on one heartbeat, flashes, held arrivals and an Undo that lapses in six seconds:
the spec never says what a screen reader hears. Left to default, a live region per row
announces 30 streams at once, which is AX §6.8's firehose delivered by voice. Six seconds is also
short for a user who must first hear that the Undo exists.

**Fix.**
1. The heartbeat and the flash are `aria-hidden`. **One** polite live region per window carries
   a digest, on AX §6.8's own rate (at most one per 15 minutes), plus direct answers to the
   user's own acts (*"EP-142 deferred. Undo, U"*).
2. Lists are ARIA grids or listboxes with `aria-rowcount`, because they are virtualised over 50
   rows (DESIGN-SYSTEM floor). Document that `Space` means peek, since it overrides the listbox
   default.
3. The Undo countdown pauses while the row has focus or hover. After it lapses, the act stays
   reversible from the issue's revision history. Say so in §4.6, so the six seconds is a fast
   path, not the only path.

### I10. The liveness text and the density budget will not hold at 30 agents

**Where:** 05 §4.1 (`last 3s` on rows), §4.4 C4; 02 C4 (*"a 30-agent RUNNING section at
`--row-h` is 960px, one screen on a 1080p laptop"*).

1. **Thirty numbers ticking every second** is motion, whatever the dots do. `last 3s → 4s → 5s`
   on every row puts per-row churn back into the calm view that `fleet.beat` was built to quiet.
   **Fix:** liveness text changes only on the beat and coarsens. Under 10 seconds it reads
   `live`; after that `quiet 2m`, `quiet 14m`. The quiet rows are the news, so they are the only
   ones whose text changes.
2. **The 960px budget is wrong.** A 1080p screen at 125% scaling, which 02 C12 lists for KPMG
   laptops, is 864 CSS px tall; at 150% it is 720. Take off the window chrome, the horizon bar,
   the `ViewHeader` and four other section headers, and 30 rows at 32px do not fit. **Fix:** state
   budgets in CSS px at 125%. When the reader arrives at the Desk (never live, per AX §5's
   fixed-order rule), RUNNING lists the agents that changed since they last looked, and folds the
   steady ones into one line that expands in place (*"23 steady, all quiet under 10m"*). The
   folded line's number is a group size under Q1 option A.

---

## Minor

| # | Where | Finding | Fix |
|---|---|---|---|
| M1 | §4.1 mockup | Draws `CYCLE 14` and a scope log, which are W2. The W1 prototype will be built from this picture | Draw the W1 Work surface without cycles, grouped by criterion (C1) |
| M2 | §4.1 mockup | `queued 2nd` on EP-139 is undefined: queued for what, behind whom? | Name the cause in mono (`review · behind EP-142`) or delete it |
| M3 | §4.1 `⇄` row | The negotiation outcome sits under the issue with no rule for when it goes. After a week, most rows carry one, and rows stop being one line | Shows until seen (read cursor), then collapses to a `⇄` glyph on the row; the typed thread stays one `Space` away |
| M4 | §4.1 | `thread.lit` is described on hover only. 02 C2 says hover **or focus** | Say focus, so keyboard users get the signature |
| M5 | §4.4 C13 | *"Five things on every screen"* cannot hold: grain is dark-only, so `paper` fails its own test; settings and first run carry no heartbeat and no thread | Rewrite as "on every work screen, in every theme", and give `paper` its own fifth mark or drop grain from the list |
| M6 | `tokens.css:163-171` | Grain is a fixed overlay at `z-index: 9999` over virtualised, scrolling lists. At 150% scaling on WebView2 its compositing cost is unmeasured | Measure scroll frame time with and without; turn grain off in the `-hc` themes and under `forced-colors` |
| M7 | §4.1 | Work becomes the **seventh** lens tab. AX §9 item 1 deleted module tabs because tabs read as stages | Make Work the project's home surface, reached by `G W`, rather than another tab in the lens row |
| M8 | §4.2, 02 C10 | Projections (`~14:22`, `~20 min`, `~4m`) sit beside AX anti-pattern 2's *"no … ETA"*. AX itself does this, and 05 inherits it | Define a projection as a stated rule plus its source span (*"expected ~14:22 · rule: migration gate"*), so it is not a bare ETA |
| M9 | §4.4 C7 | A batch land or a bulk edit by a teammate flashes 20 rows at once | Coalesce: over 3 simultaneous changes in one section, flash the section header once and mark the rows statically |
| M10 | §4.3 | `N` now means "show held arrivals". With C2, it should also apply held moves | Name it once: `N` = apply held changes |

---

## What is genuinely better than Linear, and must survive the fixes

1. **`Shift` means an agent will run.** A reader can predict the blast radius of a key from the
   key. Linear has no reason to need this; we do. (§4.3 rule 2)
2. **Two optimism classes.** `optimistic.undo` for reversible acts, `confirm.inplace` inside the
   row for irreversible ones. Copying Linear's "optimistic everywhere" onto deploy and push would
   have been the most dangerous clone move, and 05 does not make it. (§4.6)
3. **`keys.list`.** Single keys never fire in a terminal. It is correct, and it is built on the
   existing `isCockpitChord` carve-out. (§4.3 rule 1)
4. **"Shipped to production, 14:02" on the issue.** Done means proven, shown as a fact with its
   release. This is the most beautiful sentence in the spec, and it is not decoration.
5. **Peek shows evidence, not fields**, and empty states are one fact, one projection, one verb,
   with no illustration. (§4.2; 02 C9, C11)

---

## The four questions, answered

| Question | Answer |
|---|---|
| Better than Linear, a clone, or slop? | **Not slop.** DESIGN-SYSTEM's floor is strong and the built tokens pass the detector clean. **A clone by structure** (C1): the skin differs, the skeleton does not. **Better** only where agents and the thread show, and the drawn surface shrinks both to a chip and a count |
| Density | 32/28px rows are right. The 30-agent budget is wrong at KPMG scaling (I10), and the evidence is set smallest (I7) |
| Typography | Two voices is the right idea. The sans is now a generator default (I6); the mono at 11px carries too much (I7) |
| Motion | Disciplined: one curve, `transform`/`opacity`, one heartbeat. Missing: held **moves** (C2), a reduced-motion substitute (C4), flash coalescing (M9) |
| Colour | Signal discipline is good. Two tokens have two meanings each: `--need` (I4) and `--accent-ring` (I1) |
| Keyboard | The strongest part. Four holes (I3) |
| Many live agents, calm summary | `fleet.beat` and the fixed-section Desk are the right mechanisms. The Work rows undo them with per-row counts, teammates' dots and ticking seconds (C3, I2, I10), and W1's Desk omits FAILED (C5) |
| Accessibility | The weakest area: colour-only liveness under reduced motion (C4), `forced-colors` (I8), no screen-reader model (I9), global single-key sequences (I3.3) |

## Heuristic scores (Nielsen, 0 to 4, scored on the spec, not on a build)

| # | Heuristic | Score | Why |
|---|---|---|---|
| 1 | Visibility of system status | 2 | Heartbeat and agent line exist; liveness fails under reduced motion (C4); failures hidden in W1 (C5) |
| 2 | Match with the real world | 2 | The criterion appears as `c-91e2`; `queued 2nd` is undefined (C1, M2) |
| 3 | User control and freedom | 3 | Undo, rewind and in-row confirm; the Undo window is the only path (I9) |
| 4 | Consistency and standards | 2 | Two tokens with two meanings; the Shift rule broken by `Shift+X` (I1, I3, I4) |
| 5 | Error prevention | 2 | Caps Lock and `Shift+S`; bulk select confused by lit rows (I1, I3) |
| 6 | Recognition rather than recall | 3 | Chords in the palette and menus, the `?` sheet; the criterion needs recall (C1) |
| 7 | Flexibility and efficiency | 4 | Key grammar, palette prefixes, peek on evidence |
| 8 | Aesthetic and minimalist design | 2 | Clean in principle; counts, dots and negotiation rows accumulate on rows (C3, I2, M3) |
| 9 | Recognise and recover from errors | 2 | Cause lines are good; FAILED absent from W1's Desk (C5) |
| 10 | Help and documentation | 3 | `?` sheet and the real first-run landing |
| | **Total** | **25 / 40** | 62%, "acceptable": the craft floor is high, and the structure is where it loses |

## Detector

`node ~/.claude/skills/impeccable/scripts/detect.mjs --json desktop/src/styles/tokens.css
desktop/src/components/common/common.css`, run once on 2026-09-25: **`[]`, no findings.** The
built tokens carry none of the mechanical slop tells. That supports "not slop" and says nothing
about C1 to C5, which are structural and not detectable in CSS.

## Sources

No web page was fetched for this critique, and no URL is relied on. Every claim about Linear is
either taken from 02 (which records its own URLs and dates) or is general product knowledge
marked **UNVERIFIED**. Files read on 2026-09-25:

- `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` §4 (lines 750-909), §5.1 to
  §5.3 (911-969), and grep over §1 and §6
- `docs/superpowers/planning/2026-09-25-linear-parity/02-linear-design-language.md` Part C
  (lines 180-355)
- `docs/DESIGN-SYSTEM.md` (all); `docs/AGENTIC-UX.md` §4 to §10 (lines 64-252)
- `docs/superpowers/planning/2026-09-23-owner-decisions.md` OD-14 to OD-17 (lines 114-128)
- `desktop/src/styles/tokens.css` lines 46-191; grep over `desktop/src` for `forced-colors`,
  `aria-live`, `prefers-reduced-motion`, `:focus-visible`

