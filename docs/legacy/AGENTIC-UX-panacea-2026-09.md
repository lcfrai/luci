# Panacea App — Design Brief: supervising many coding agents

**Status:** settled direction. Supersedes the layout of `prototype/index.html`, `prototype/v2.html`, `prototype/v3.html`. Does not supersede `docs/DESIGN-SYSTEM.md`, which still governs tokens, colour, motion and type.

---

## 1. The diagnosis

Every prototype so far is a **place you visit to find out what is true**, and the user's actual need is **a list of what changed while they were away, sorted by how expensive it is to be wrong**. The interface keeps asking "where do you want to go?" while the user is asking "what did I miss, and what is about to go wrong without me?" That gap cannot be closed by layout, which is exactly why the owner can see the problem and cannot name it: nothing is missing from the screen, so there is nothing to point at. What is missing is an *answer* — the interface shows the present tense to a person who was absent, shows state where change is the unit of interest, shows everything at equal weight because nothing in the model knows what matters, and has no zero state, so it never tells you you are finished. A dashboard you stare at feels wrong to someone who needed a desk they return to, with the post already sorted on it.

Two corollaries, both of which the owner already said out loud without the vocabulary:

- **"Too rigid"** = Define → Design → Develop → Deploy is static function allocation baked into the navigation. It asserts an order that real work does not follow, and renders the normal case (going backwards) as failure.
- **"False sense of hope"** = a progress bar makes a claim about a denominator that does not exist. Agent work is a search, not a march. The fix is a *convergence* display (what is settled, open, contested, drifting), not a better estimate.

---

## 2. What makes agentic UX different

Seven properties. Each has a design consequence that is checkable in review.

**1. The human is intermittently present; the work is continuously running.**
Time in the interface and time in the work are different clocks. Consequence: every surface is rendered relative to **a per-user read cursor**, not to `now`. "2m ago" is a present-tense idiom and is banned as a primary time display. "Since you last looked" and "waiting 6h 40m" are the primary idioms.

**2. Change is the unit of interest, not state.**
"What is true now" is near-worthless to someone with 22 agents who was last here four hours ago. Consequence: the primary surface is a **delta**, and the delta has exactly four classes that must never be merged into one activity stream: **LANDED, WAITING, FAILED, ASSUMED**.

**3. The dangerous event is invisible by construction.**
A blocked agent advertises itself: it stopped. The costly one hit an ambiguity, **guessed**, and kept going. Consequence: **ASSUMED** is a first-class object with its own surface and its own verbs (accept / reverse / rewind-to-here). Nothing in any prototype models it. This is the largest single share of the unnameable unease.

**4. Attention is the scarce resource, not screen space.**
At 10–30 agents the human is a scheduler and approver, not an observer. Consequence: the interface's job is **ranking and batching**, not display. Adding a pane is a cost, not a feature.

**5. Data availability is not observability.**
Three prototypes served perception ("here is agent 7, here is its output") and zero comprehension or projection. Consequence: every agent gets a generated **one-sentence meaning line** ("third attempt at the same test; last edit changed the test, not the implementation") and a **projection line** ("needs you in ~4m: migration next" / "autonomous for ~20m"). A log tail is not a status.

**6. Authority must be legislated in advance, not vetoed in real time.**
Continuous presence lets you veto as it happens; intermittent presence does not. Consequence: an explicit **pre-authorisation surface**, scoped per action class, changeable mid-flight, that shows its own predicted cost in interruptions per hour.

**7. The transcript is the artifact; everything else is a claim about it.**
A status dot asks you to trust the agent *and* the dashboard. Consequence, and this one is a hard review rule: **if you cannot name the transcript span that justifies a UI element, the element must not exist.** That rule deletes progress bars, health scores and summary counters automatically.

---

## 3. The core interaction loop

**Name it in the product and in the code: LEGISLATE → LEAVE → RETURN → ADJUDICATE.**

The product is not "run agents". It is **"delegate and be able to leave"**. Absence is the feature.

1. **LEGISLATE** — before leaving, set what each agent may do unsupervised: per action class, per branch, with defaults inherited from project. The screen tells you the consequence of the setting ("about 4 interruptions per hour; blast radius: own worktree only"). End-of-day is a first-class moment, not a settings page.
2. **LEAVE** — the human is gone. Agents run, publish intent, block where policy says block, record every unilateral decision, and accrue a delta against the user's read cursor. The system batches; it does not stream notifications.
3. **RETURN** — the user lands on a generated handover, not a dashboard. Four sections, in rank order. This is the surface the product lives or dies on (section 6).
4. **ADJUDICATE** — empty the queue: approve, answer, reverse an assumption, restart or abandon a failure, accept landed work in bulk. Each decision is answerable **in place**, without navigating. Then either legislate again and leave, or drop into the one escape hatch.

**The escape hatch — DIVE.** From any row, take the keyboard inside the real live session at the exact point of difficulty. Entering carries a "what happened here while you were gone" preamble; exiting captures "what I decided and what I left running". DIVE is roughly 10–20% of sessions and is the only rhythm every prototype was designed for, because it is the one that looks like an IDE.

**Build order, and this settles the argument:** RETURN first (it forces the runtime requirements that everything else depends on), then LEGISLATE, then DIVE, then merge-order convergence, then the new-work entry. Building the 10-second glance first gets you a badge with nothing behind it.

**Hard runtime dependency, not a UI task.** The agent runtime must emit, at every branch point: *decision, reason, alternatives rejected, confidence*; a continuously-updated one-line declared intent; a plan; and checkpoints. If the runtime does not emit these, the UI cannot invent them and RETURN stays unsolvable. This is the single most important non-UI implication in this brief and it must be scheduled before the rebuild, not after.

---

## 4. Principles

Twelve rules. Each is written so a design can fail it.

1. **Every screen answers "what happens if I look away for ten minutes?"** Fails if a screen shows only present state with no projection of demands on the human.
2. **Optimise seconds-to-resume, not agents-per-screen.** The measured budget is **30 seconds from cold to first correct decision**. Fails if reconstructing context requires reading raw output.
3. **Show meaning, not just state.** Every agent carries exactly three lines: STATE (machine-derived), MEANING (one regenerated sentence), NEXT (projection). Fails at four lines, fails at zero meaning line, fails if MEANING is a log tail.
4. **One ranked queue. No scattered badges.** Rank = cost of delay × blast radius × irreversibility, plus a bonus for agents blocking other agents. The rank reason is printed on the row. Fails if a count appears anywhere outside the queue.
5. **Always answer "what if I do nothing?"** Every queue row carries a consequence-of-inaction line and a deadline if one exists. Fails without it.
6. **Unilateral decisions are a first-class class.** ASSUMED is displayed, auditable, and reversible with a rewind, not a manual unpick. Fails if the only verbs in the product are approve and deny.
7. **No aggregate without provenance.** Every dot, number and row click-throughs to the exact transcript span that produced it — that line, not the top or bottom of the session. Fails if any element's justification cannot be named.
8. **Mechanically-derived status only, from a closed set of seven.** PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE, PAUSED, DRIFTING, DONE. Each maps to a fact (blocked on stdin, no output for N minutes, process exited). Fails on free-text status or authored health scores.
9. **No silent transitions.** Every mode change writes a timestamped row with a cause: "EXECUTING → BLOCKED-ON-YOU (rule: writes outside worktree require approval)". Every autonomous act names the rule the human set. Fails if "why did it do that?" requires reading a transcript.
10. **Replace progress with convergence.** Settled / open / contested / drifting. No percentages, no bars, no completion ticks, no "2 of 4 met". Fails on any denominator the system cannot defend.
11. **Never reorder or reflow under the user.** Position is a free spatial memory cue. Urgency lives in the queue, never in the sort order of the rail. Scroll position, expansion state and reading position persist per agent across visits. Fails if a poll moves anything the user is reading.
12. **Four lenses, no pipeline.** Discover, Design, Develop, Deploy are simultaneous views on a branch: no order, no arrows, no numbering, no completion states. Entry into any lens is always legal from any state. Fails if the UI implies sequence.

**Per-screen acceptance test** (apply in every review): is the agent **observable** (can I see what it is doing and why), **predictable** (can I anticipate what it does next), **directable** (can I change course right now without stopping it)? All three prototypes score zero on all three.

---

## 5. The primary surface: **The Desk**

Argued from the rhythms: the 10-second check, the 2-minute triage and the 4-hour return are the same mechanism at three zoom levels, and together they are ~80% of sessions. The Desk is **account-level, above projects**, because the user does not yet know which project the problem is in. It is the default landing surface, always. It is a surface you **empty**, not one you stare at.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  HORIZON · next 15 min   4 decisions expected  ·  2 agents converging on              │
│  services/api/routes.go  ·  1 will touch prod config ~14:22                           │
│                                        [ quiet 30m ]  [ pause all ]  [ ⌘K ]           │
├────────────┬─────────────────────────────────────────────────────────────────────────┤
│ PROJECTS   │  SINCE YOU LAST LOOKED — 08:41, 6h 12m ago      [ mark all seen ]        │
│            │                                                                          │
│ CT  ● ●    │  ┌── NEEDS YOU · 4 ───────────────────────────────────────────────────┐ │
│ ├ main     │  │ 1  test harness · CT/evidence-upload            waiting 5h 51m     │ │
│ ├ evidence │  │    wants: panacea migrate up --project control-testing             │ │
│ │  · uplo  │  │    ranked first: irreversible · 2 agents idle behind it            │ │
│ │  · test  │  │    if you do nothing: branch stalls until you return               │ │
│ │  · docs  │  │    ┌ last 12 lines of transcript ──────────────────────────────┐   │ │
│ └ preview  │  │    │ ⏺ Edit services/api/routes.go  (holding)                  │   │ │
│            │  │    │ + api.POST("/controls/:id/evidence", h.UploadEvidence)    │   │ │
│ VS  ●      │  │    └───────────────────────────────────────────────────────────┘   │ │
│ MF         │  │    [Approve]  [Always allow migrations on this branch]  [Deny]     │ │
│            │  │    [Ask one question]   [Defer ▾]              open session ↗      │ │
│ ＋ project │  │                                                                    │ │
│            │  │ 2  BATCH · 4 agents want to add a dependency in their own worktree │ │
│            │  │    approve pattern for all 4 →  [Approve pattern]  [Review each]   │ │
│            │  │ 3  …                                                               │ │
│            │  └────────────────────────────────────────────────────────────────────┘ │
│            │                                                                          │
│            │  ┌── DECIDED WITHOUT YOU · 3 ─────────────────────────────────────────┐ │
│            │  │ upload handler chose S3 presigned URLs over direct upload          │ │
│            │  │   because: mobile clients time out on 40MB bodies                  │ │
│            │  │   rejected: direct upload · chunked multipart                      │ │
│            │  │   [Accept]  [Do it the other way]  [Rewind to here]      span ↗    │ │
│            │  └────────────────────────────────────────────────────────────────────┘ │
│            │                                                                          │
│            │  ┌── FAILED · 1 ──────────────────────────────────────────────────────┐ │
│            │  │ scanner wiring · exit 1 at 02:14 · budget exhausted                │ │
│            │  │   [Restart from checkpoint]  [Abandon]                   logs ↗    │ │
│            │  └────────────────────────────────────────────────────────────────────┘ │
│            │                                                                          │
│            │  ┌── LANDED · 6 ──────────────────────────────────────────────────────┐ │
│            │  │ Uploads now work on mobile.       +412 −88 · 9 files   [Accept all]│ │
│            │  │ docs worktree finished, no conflicts.  ready to merge   review ↗   │ │
│            │  └────────────────────────────────────────────────────────────────────┘ │
│            │                                                                          │
│            │  ┌── RUNNING · 7 ─────────────────────────────────────────────────────┐ │
│            │  │ ▸ upload handler   EXECUTING   41m · last output 3s ago            │ │
│            │  │   writing quarantine notification; 3 edits applied, tests green    │ │
│            │  │   autonomous for ~20m                                              │ │
│            │  │ ▸ refactor-auth    DRIFTING    3rd attempt on same test            │ │
│            │  │   last edit changed the test, not the implementation               │ │
│            │  │   will need you in ~4m                                  steer ↗    │ │
│            │  └────────────────────────────────────────────────────────────────────┘ │
└────────────┴──────────────────────────────────────────────────────────────────────────┘
```

**Rules for the Desk.**

- **The rail is navigation only.** No counts, no dot clusters on projects and branches and worktrees. Attention lives in one place. The tree is a storage model, and a tree is a terrible attention model because the user's question cuts across it.
- **Zero state is the design target, and it must be reachable.** When the queue is empty: *"Nothing needs you. 7 agents running. Next decision expected ~14:22."* A surface that is usually empty is what makes the one non-empty day credible. If the primary surface has no zero state it will keep feeling wrong regardless of layout.
- **Section order is fixed** (NEEDS YOU → DECIDED WITHOUT YOU → FAILED → LANDED → RUNNING) and sections never resort under the cursor. Rank changes apply on explicit refresh or on returning to the surface.
- **Answering never navigates.** Approve, defer, answer-in-one-line and accept-in-bulk all resolve in place, optimistically, with an Undo window per `docs/DESIGN-SYSTEM.md`. Keyboard-traversable end to end: `j/k`, `a`, `d`, `?`, `Enter` to dive.
- **The agent card is a resumption cue,** designed as one. Three lines, always three: STATE / MEANING / NEXT. Plus, on expansion: the last decision **you** made and when, what changed since, and the one open question.
- **The horizon bar is the highest-leverage element in the product.** Nothing on the market has it. It projects the next 15 minutes of demands on the human, including collisions between agents, and it carries the two panic controls: quiet mode (hold all non-irreversible requests for N minutes and tell me what was held) and pause everything.

---

## 6. Answering "what did I miss?"

The mechanism, concretely. This is the product's differentiator and the thing no competitor has built.

**1. A read cursor per user, per scope.** Sessions, branches and projects each carry `last_seen_at` per user. Everything renders as a delta against it. "Mark all seen" moves it; entering a session moves it for that session only. Without this, nothing can be marked new, so everything is equally present-tense, so nothing stands out. This alone produces the "cluttered, I don't know what anything is" feeling.

**2. Four delta classes, never merged.** LANDED (reassuring, bulk-acceptable, low attention), WAITING (aging matters; an agent idle 9 hours is a wasted night), FAILED (restart or rethink), ASSUMED (highest risk, currently unmodelled anywhere). One activity stream mixing these is the same as no stream.

**3. The handover is generated in product language, not commit language.** "Uploads now work on mobile; the agent chose S3 presigned URLs over direct upload without asking; the migration has been waiting for your approval since 23:40." Not a commit list, not a log tail. The model here is a clinical or process-control shift handover: what happened, what state it is in, what I did about it, what I need from you.

**4. Every claim in the handover is a link to its span.** The summary is an *index over transcripts, never a replacement for them*. One click lands on the exact line, not the top of the session. An LLM summary as the primary content stacks two error surfaces; as a navigable index over mechanically-derived facts it is safe.

**5. Session entry lands on the watermark, not the bottom.** A "you were last here" rule, everything above dimmed, everything below is the delta. This is the single change that converts "re-scan 30 sessions" into "read the new part", and it is why email and pull requests survive volume while a wall of terminals does not.

**6. Exit state is captured.** On leaving a session: what you decided, what you left running, and an optional note-to-self on any agent ("check the retry logic when this comes back"). Without an exit state, every return is a cold start and the user carries the fleet in working memory between sessions. That specific cost is what makes 10–30 agents feel impossible rather than empowering.

**7. Aging is visible everywhere.** "waiting 5h 51m", not "5h ago". Wasted idle time must be legible or it is invisible.

**8. Notifications are a digest, never a stream.** One batched arrival summary. Twenty agents produce a firehose; 40 toasts and zero toasts are the same signal.

---

## 7. Autonomy and trust

**Authority is not one number and is not set once.**

**The grid.** Rows are action classes: read · edit in own worktree · edit outside worktree · run tests · install dependency · run migration · touch infra/config · commit · push · deploy. Columns are named discrete levels: **ASK FIRST · PROPOSE AND WAIT · ACT AND TELL ME · FULL**. Never a continuous slider; nobody can tell what 0.6 of authority means. Defaults inherit project → branch → agent, and the inherited value is shown greyed in place so overrides are visible.

**Show the consequence, not just the setting.** Under the grid: *"At this level, expect about 4 interruptions per hour from this agent. Blast radius: own worktree only."* That converts an abstract authority choice into a workload prediction, which is the thing the supervisor actually cares about.

**Authority moves both ways at runtime.** The agent may **request elevation** ("I need to add a dependency") and must **self-demote on uncertainty** ("confidence low, dropping to PROPOSE AND WAIT"). An agent that lowers its own leash when out of its depth is the strongest trust-building behaviour available and should be visibly logged as such.

**Trust is calibrated from the user's own history, per action class.** "You approved 23 of this agent's 25 test edits unchanged." That sentence beats any self-reported confidence score. Turn it into a reversible one-click proposal in both directions: *"Raise this agent to ACT AND TELL ME for test edits?"* and *"3 of your last 5 migration approvals were rewritten afterwards. Lower migration authority?"* **Never show a confidence number without its calibration record.** If you cannot compute the calibration, do not show the number.

**How approvals survive 30 agents without alarm flooding:**

1. **Structural batching.** "4 agents want to add a dependency to their own worktree — approve the pattern for all?" You reduce the number of decisions, not the speed of deciding. This is the only real answer to eight-at-once.
2. **Standing policy on every approval row.** "Always allow migrations on this branch." The queue must shrink structurally, not just today.
3. **Distinguish expensive waiting from free waiting.** "Blocked, holding a lock, 3 agents stalled" and "blocked, costs nothing, will keep until tomorrow" must not look alike. Making every block look urgent is precisely how the cry-wolf effect sets in.
4. **Defer with a reason is a first-class verb,** and deferred items age visibly rather than disappearing.
5. **Engagement sampling, as a visible user-set dial.** Even at high autonomy, route a small random fraction of decisions to the human for a fast yes/no. Deliberate friction with a purpose: it maintains situation awareness and calibrates trust. Visible and adjustable, never a hidden behaviour.
6. **Drift is an alert class alongside blocked and failed.** Detected mechanically as divergence between declared intent and observed action: files outside declared scope, third retry of the same test, scope growth, budget burn without progress.

---

## 8. Keeping the CLI real while aggregating many

The promise is that the live Claude Code output stays genuine. The mechanism that lets it also scale:

1. **The transcript is the only source of truth.** The aggregation layer is an index built exclusively from mechanically derivable facts: blocked-on-stdin, open permission prompt, output in last N seconds, process exited, exit code, files written, tokens spent.
2. **Span addressing.** Every transcript line has a stable id. Every aggregate element in the product carries the span id that justifies it and deep-links to that line. This is the enforcement mechanism for Principle 7.
3. **Structural compression, never prose compression.** Fold a transcript into tool-call steps; the collapsed row carries command, duration, exit code and line count: `pytest -q · 43s · exit 1 · 812 lines`. Cost stays legible, and 400 lines of test output still *means* something. A prose summary destroys that.
4. **Sub-agents are collapsed steps that keep their evidence.** Their own full transcript one click away, and their own row in the queue when they block. Claude Code today collapses sub-agents and drops their evidence, leaving a hole in the parent's audit chain. Do not inherit that bug.
5. **The diff is a co-equal second surface with back-links into the transcript.** Each hunk links to the span where it was written and why, constructed mechanically from timestamped file-write tool calls. Review at the level of "what changed about the product", with the diff as drill-down, because the user is deciding acceptance, not authorship. This is the missing piece in every PR-returning competitor and the strongest differentiator available.
6. **Working set, not wall.** Keep a canvas of live sessions, but bound it to the **5–10** the human has pinned by hand. The queue handles the other 25. Never auto-arrange, never re-sort, never move a tile a human placed.
7. **Take the keyboard.** From any row, drop into the real PTY at the exact point of difficulty. The human's keystrokes and reasoning stay in the session transcript, so the agent is not surprised either. Because this product aggregates real sessions rather than proxying them, this is available and dashboard-only competitors cannot copy it.
8. **Honest liveness only.** "Running 6m, last output 3s ago." Elapsed and last-activity are facts; remaining is a guess. One global heartbeat clock, 10Hz render throttle, no scroll-jump when the user has scrolled up, per `docs/DESIGN-SYSTEM.md`.

---

## 9. What to delete

Itemised against `prototype/v3.html`.

1. **The four module tabs with status counts** (lines 355–358: `Discover · done`, `Design · 3`, `Develop · 3`, `Deploy · live`). The counts make them stages with completion. Keep four lenses; delete `done`, delete the ordering, delete the numbers.
2. **"Definition of done — 2 of 4 met"** (line 616). A progress bar wearing a text disguise. Replace with convergence: what is settled, what is open, what is contested.
3. **The dot clusters on every rail level** (lines 310, 312–322, 329, 335). Project dots, branch dots and worktree dots are a flood generator. One queue, not thirty badges.
4. **The "Needs you · 4" footer counter** (lines 341–343). A count buried in a rail footer is not a queue. Promote to the primary surface, ranked, aging, answerable in place.
5. **Project card footer aggregates** (lines 391–393, 399, 404: "3 agents working / 2 need you / 3 branches"). Authored aggregates with no click-through to the fact that produced them.
6. **The three-column tile wall as the default view** (lines 533–588). Demote to the pinned working set inside DIVE. It is the rhythm-3 surface and rhythm 3 is 10–20% of sessions.
7. **The start card that routes all new work into Discover** (lines 377–382, hint "Starts a project and opens Discover"). Three distinct entries exist — I have an idea, I have a repo, I have a ticket — and forcing the common case (fix this) through the rare case (idea to app) is exactly the rigidity the owner named. One low-ceremony entry that accepts any of the three and infers the depth.
8. **Repeated breadcrumb chrome in every module bar** (lines 457–458, 521–522, 593–594) and the `Columns ▾` control (line 524). Chrome competing with content.
9. **Per-column "Type to this agent" inputs as the primary steering surface** (lines 554, 573, 586). Keep steer-by-message, but it queues to a running agent from the queue row; it is not three identical empty text fields.
10. **The bare approval card** (lines 567–571). Not deleted but rebuilt: it currently shows the command with no consequence-of-inaction, no rank reason, no aging, and no surrounding transcript. As shipped it becomes rubber-stamping inside a day.
11. **The coordination banner as a one-off** (lines 527–531). The content is right and it is one of the few genuinely good things in v3. Promote it into the horizon bar as a standing collision forecast, delete the bespoke strip.
12. **Everything that implies the pipeline**: arrows, numbering, ordered module lists, "next phase" language, and the Define → Design → Develop → Deploy framing in navigation. It survives as vocabulary in `CLAUDE.md`, not as a spine in the UI.

---

## 10. Anti-patterns

Enforceable, numeric where possible. Any one of these fails a design review.

1. **No UI element without a nameable transcript span.** Zero exceptions.
2. **No percent-complete, progress bar, ETA, or "N of M met"** anywhere in the product.
3. **No more than 3 lines per agent card** at rest (STATE / MEANING / NEXT).
4. **Exactly 7 statuses**, from the closed set. Any eighth requires deleting one.
5. **No more than 10 pinned tiles**, and tiles are hand-placed only. Spatial memory stops paying above roughly 7–12.
6. **No approval row without four things**: the ask, ≥12 lines of surrounding transcript, the consequence of inaction, and a standing-policy option.
7. **No layout shift from polling.** Zero pixels. Nothing reorders, reflows or re-sorts while the user is reading; rank changes apply on explicit refresh only.
8. **No more than 1 notification per user per 15 minutes**, and it is a digest. Never a per-agent toast.
9. **No "N ago" as a primary timestamp.** Primary is "since you last looked" or "waiting Xh Ym".
10. **No badge or count outside the queue.** The rail carries names, not numbers.
11. **No modal raised by a background session**, ever. Background events enter the queue.
12. **No confidence number without its calibration history.** If you cannot compute the second half, delete the first half.
13. **No collapsed-by-default tool output that hides its cost.** Collapsed rows carry command, duration, exit code, line count.
14. **No chat-bubble chrome over engineering output.** It reframes evidence as conversation and invites social trust in place of evidentiary trust.
15. **Time-to-first-correct-decision from cold: ≤ 30 seconds.** If a reviewer cannot do it with a stopwatch, the design fails.
16. **The primary surface must have a reachable zero state.** If it cannot be emptied, it is a dashboard and it is wrong.

**One-line restatement for the build team:** stop building a workspace you inhabit; build a desk you return to, with the post, the decisions, and the things that went wrong while you were out already sorted on it.