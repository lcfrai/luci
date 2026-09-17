## State & lifecycle defects — `prototype/app.html`

### 1. HIGH — the live tick destroys whatever the user is typing into an agent composer
`setInterval` at **1214–1231**. Every 6th tick (~8.4 s) any `status==='run'` worktree gets new tty lines and the handler calls `renderMain()` (1229), which does `main.innerHTML = …`. The composer at **1048–1057** is `<input placeholder="">` with no `value` attribute and its text lives nowhere in `S`.

- **User does:** opens Develop on `feature/evidence-upload`, starts typing a prompt to the `docs` worktree (takes >8 s).
- **Happens:** mid-sentence the whole main pane is rebuilt, focus is lost and every character typed is gone. It fires even when the running agent is on a *different* project — the guard at 1227 only checks `view`/`mod`, not which branch changed.
- **Fix:** before `renderMain()` capture `document.activeElement` id/value/selectionStart and restore after; or hold composer text in `w.draft` and emit `value="${esc(w.draft)}"`; or skip the re-render entirely while `document.activeElement` is an `input`/`textarea` inside `#main`.

### 2. HIGH — `uid()` collides, so an imported project gets two branches with the same id
`uid` at **297**: `p + '-' + Math.floor(performance.now()*1000%1e9).toString(36)`. `doImport` (**541–557**) calls `uid('b')` at line 542 and again at line 551, microseconds apart. Browsers clamp `performance.now()` (Chrome ~100 µs, Firefox 1 ms), so both calls return the same number and both branches get the identical `b-xxxx` id.

- **User does:** "I have a repo instead" → pick a repo → "Bring it in", then clicks `staging` in the rail.
- **Happens:** `brOf()` (452) uses `find`, so `staging` resolves to `main`. Both rail rows highlight as selected simultaneously (`S.ui.bid===b.id` at 819), and `toggleBranch`/`openBranch` on staging operate on main. The staging branch is unreachable for the life of the session.
- **Fix:** `let _n=0; const uid = p => p+'-'+(++_n).toString(36);` (or append a counter to the timestamp).

### 3. HIGH — denying a permission dead-ends the worktree permanently
`answerPerm` choice 2 at **633–635** pushes "Tell me what to do instead" and sets `w.status='need'`. But `viewDevelop` **1033** renders the composer only when `w.status!=='need'`, and the canvas layout (1022–1027) never renders one at all.

- **User does:** on `test harness`, clicks "No, and tell Claude what to do differently".
- **Happens:** the agent asks for direction and there is no input box to answer with, and no action changes `status` back off `need`. The only escape is closing the worktree (losing it). `cycleMode` is also unreachable because `.cc-mode` lives inside the same hidden block.
- **Fix:** use a distinct status (e.g. `'idle'`/`'need-input'`) for "awaiting your words" and gate the composer on "a pending `perm` line exists in `w.tty`" rather than on `status==='need'`.

### 4. HIGH — `S.ui.wid` is write-only; a newly created worktree is never brought into view
`createWorktree` sets `S.ui.wid=id` at **614**, and `SEED` declares `wid`/`focusCol` at **443**. Neither is ever read — no render function references `S.ui.wid`, and the `.col.focus` style (CSS line 137) is never applied.

- **User does:** on a branch that already has 3 worktrees, creates a 4th.
- **Happens:** columns are `min-width:420px` inside `.cols{overflow-x:auto}`, so the new column is created off-screen to the right with no scroll and no focus ring. The toast fires but the screen looks unchanged. (Same class of dead pointer: `closeWorktree`/`mergeOne` never clear `S.ui.wid`, so it also outlives the object it names — harmless today only because nothing reads it.)
- **Fix:** in `afterDevelop()`, if `S.ui.wid` matches a column, `scrollIntoView()` it and add `.focus`; clear `S.ui.wid` in `closeWorktree` and `mergeOne`.

### 5. MEDIUM — terminal history cannot be read while any agent runs
`afterDevelop` **1118** unconditionally sets `t.scrollTop=t.scrollHeight` for every `.tty`. The tick at **1228** builds a `scroll` map of "was this pane at the bottom?" and then never uses it — the intended preservation is dead code.

- **User does:** scrolls up in a column to re-read an earlier diff.
- **Happens:** within ~8.4 s the tick re-renders and yanks every pane back to the bottom. Also fires on unrelated renders (`cycleMode`, `toggleProject`, a toast-producing action).
- **Fix:** actually consume the map — record stick-to-bottom per `t.id` before `renderMain()`, pass it into `afterDevelop()`, and only auto-scroll panes that were already pinned.

### 6. MEDIUM — dragged canvas tiles snap back
`afterDevelop` **1122–1128**: the `up` handler only removes listeners; unlike the Design canvas (**987–989**, which writes `s.x`/`s.y` back to state), tile positions are never stored. `viewDevelop` recomputes them from the loop index at **1024**.

- **User does:** switches to Canvas layout, arranges the tiles.
- **Happens:** the next live tick re-renders and all tiles jump back to the 2-column grid.
- **Fix:** store `w.cx`/`w.cy` on pointerup and render `left:${w.cx ?? default}px`.

### 7. MEDIUM — `editAnswer` silently deletes the answer you clicked plus every answer after it
**696**: `b.discover.answers.splice(i)` (no delete-count) removes from `i` to the end, sets `done=false`, and nothing pre-fills the old text.

- **User does:** on `feature/evidence-upload`, clicks "edit" on the first settled answer to fix a typo.
- **Happens:** all three answers vanish, the branch drops out of "Discover complete", and the re-asked questions are the generic `A.QUESTIONS` wording ("What is this branch for?"), not the branch's original questions ("Purpose of this branch"). On seeded branches whose answer count is less than `QUESTIONS.length` (e.g. `ct-main`, 1 answer) the user is now forced through 3 unrelated questions to get back to `done`.
- **Fix:** `splice(i,1)` and seed the free-text box with the old answer, or re-ask only question `i` and splice it back at the same index.

### 8. MEDIUM — approving "don't ask again for migrate commands" silently turns off all permissions
`answerPerm` **636**: `if(choice===1){ w.mode='bypass'; }`.

- **User does:** picks option 2, "Yes, and don't ask again for **panacea migrate** commands".
- **Happens:** the worktree's global mode flips to bypass (footer now reads "bypass permissions on"), granting far more than the scoped consent; if the worktree was in `plan` mode that is silently lost too.
- **Fix:** keep `w.allow = w.allow||[]` and push the command prefix; leave `w.mode` alone.

### 9. MEDIUM — nothing ever sets `ready`, so user-created worktrees can never be merged
`ready:false` is written at **609** and only ever read (**852, 1040, 1135–1136**). No action in `A` sets it true.

- **User does:** cuts a worktree, talks to the agent until it says it is done, opens Merge.
- **Happens:** it is always under "STILL WORKING"; only the seeded `docs` worktree is merge-able, forever. Related: the "Merge in this order" button at **1140** only renders when `ready.length>1` yet calls `mergeOne(ready[0].id)` — it merges one item despite promising a batch, and is hidden in the single-ready case where it is not needed anyway.
- **Fix:** set `w.ready=true` when the agent's turn ends (e.g. in `sendTo`/the tick when a worktree transitions to `done`), and make the batch button loop over `ready`.

### 10. LOW — `reorder` indexes the unfiltered array while Merge renders the filtered one
**738–742** swaps inside `b.worktrees`; `viewMerge` renders `ready = b.worktrees.filter(w=>w.ready)` (**1135**) and the ↑/↓ buttons (**1153–1154**) are per-ready-item.

- **User does:** with ready worktrees at indices 0 and 2 and a non-ready one at 1, clicks ↑ on the second ready item.
- **Happens:** it swaps with the invisible non-ready worktree; the visible order does not change, so the button looks broken.
- **Fix:** compute the neighbour within the filtered list and swap those two positions in `b.worktrees`.

### 11. LOW — shift+Tab (the documented shortcut) throws away the prompt you were typing
Key handler **1237–1241** → `cycleMode` → `render()` → full `innerHTML` replacement, same data loss as #1. It also does nothing in Canvas layout because tiles carry `.tile`, not `.col` (`closest('.col')` at 1238). The unused `const b=curB();` at 1239 is dead.

- **Fix:** restore the input value/caret after `cycleMode`, and match `.col,.tile`.

### 12. LOW — `/clear` does not actually reset context to the shown value
`sendTo` **653** sets `w.ctx=2`, then line **661** runs unconditionally and bumps it to 4. Move the `w.ctx=Math.min(99,w.ctx+2)` into the non-slash-command branch.

### 13. LOW — a whitespace-only project name produces an unnamed card
`createProject` **493**: `($('#npName').value||'Untitled project').trim()` — `" "` is truthy, so the fallback is skipped and `name` becomes `''`, giving an empty rail row and empty `ini`. Trim first, then fall back.

---

### Traced and clean (no defect found)

- **`answerPerm`'s captured index (1106) is not stale in practice.** The only mutator of a `perm`-bearing worktree's `tty` is the tick, which skips anything not `status==='run'`; the seeded perm sits on a `need` worktree, and `sendTo` is unreachable there because the composer is hidden. Every re-render regenerates `i` from the current array. The `line.t!=='perm'` guard (627) would also fail closed. It is fragile — it is the only index-addressed handler in the file, and a second perm line or any future tty mutation would make the click silently no-op — but it is not a live bug today. Worth converting to an id on the perm object anyway.
- **No dangling `S.ui.pid`/`S.ui.bid`.** Nothing in `A` deletes a project or a branch, so those pointers cannot outlive their objects. (`goHome` nulls them explicitly; `curB()` null is handled at 844 and 866.)
- **Newly created objects are fully initialised.** `createProject` (497–499), `createBranch` (572–573) and `doImport` (545–553) all set `discover{done,answers}`, `screens`, `design`, `worktrees`; `createWorktree` sets `files:[]`. The seeded branches that lack `design` are covered by the `b.design||[]` guards at 709 and 960, and `w.files` by `(w.files||[])` at 1060 and 1151.

=====AGENT BREAK=====

# Prototype control audit — `prototype/app.html`

All 71 inline handlers traced. 12 real defects, 3 of them make a control or a whole state unusable. The command palette and modal behaviour are, with two exceptions noted, correct.

---

## Defects, ranked

### 1. Denying a permission prompt strands the worktree with no way to reply — hard dead end
`viewDevelop()` line **1033**: `${w.status==='need' ? '' : ccInput(p,b,w)}` — a worktree in `need` state renders **no composer**. `A.answerPerm` (line **632–634**) on `choice===2` deletes the prompt, prints *"Understood — I will not run that. Tell me what to do instead."*, and sets `w.status='need'`.

User: open `Control Testing → feature/evidence-upload → Develop`, click option **3. No, and tell Claude what to do differently** on the *test harness* column.
Result: the prompt disappears, the agent asks you to say what to do, and there is no input, no mode line, no button. The column is permanently inert; the rail keeps showing "1 waiting on you" forever. Only exit is `×` (destroy the worktree).

Fix: render `ccInput` always, or at minimum whenever `status==='need'` and there is no live `perm` line in `w.tty`. The `need`→hide rule should key off "a perm prompt is currently showing", not off status.

### 2. First click on "Deploy this branch" / "Rollback" is swallowed after typing an env name
Deploy env field, line **1181**: `onchange="A.setEnv(this.value)"`. `A.setEnv` (line **720**) calls `render()`, which replaces all of `#main`.

User: open Deploy on an undeployed branch (e.g. a newly created project's `main`), type `staging`, click **Deploy this branch**.
Result: mousedown blurs the input → `change` fires → `render()` destroys the button mid-click → no `click` event is ever dispatched. The env name applies, the deploy does not. You must click the button a second time. Same for Rollback and for the Other-branches cards.

Fix: in `setEnv`, only re-render the parts that changed (or `queueMicrotask`/`requestAnimationFrame` the render), or switch the field to `oninput` with a debounce and no full re-render.

### 3. The live tick wipes whatever you are typing into an agent
Lines **1214–1231**: every 6th tick (≈8.4 s) `renderMain()` is called, replacing `#main` wholesale. The composer `<input>` is a plain uncontrolled input inside that subtree.

User: type a message longer than ~8 seconds into any running agent's composer.
Result: text and focus vanish mid-sentence. Same tick also re-runs `afterDevelop()` (line **1118**) which forces every `.tty` to `scrollTop=scrollHeight`, so scrolling up to read an agent's history is undone within 8 seconds.

Note the author already intended to fix this: line **1228** builds a `scroll` map of which panes were at the bottom and then never uses it.

Fix: on tick, patch only `#tty-<id>.innerHTML` for changed worktrees, and honour that `scroll` map instead of unconditional auto-scroll.

### 4. Canvas layout removes every way to talk to an agent, and drags reset
Lines **1023–1026**: the canvas tile renders `colHeader + tty + colFooter` only — `ccInput` is never called. So switching **Columns → Canvas** silently removes the composer and the mode line from every agent; the only controls left are `merge` and `×`.
`afterDevelop` (line **1122–1128**) drags tiles by mutating `t.style` and its `up` handler saves nothing, while the tick re-renders `viewDevelop`, which recomputes `left/top` from the loop index.

User: switch to Canvas, drag a tile, wait.
Result: within ~8 s the tile snaps back to its grid slot. Also, the `.tile .rs-h` resize handle is styled (line **182**) but no `.rs-h` element is ever emitted, and `.canvas` is `overflow:hidden`, so with enough worktrees tiles are unreachable.

Fix: include `ccInput(p,b,w)` in the tile; persist `x/y` onto the worktree in the pointerup handler and read them back in `viewDevelop`.

### 5. Shift+Tab cycles the mode exactly once, then stops
Keyboard handler lines **1237–1241**: it finds the column via `document.activeElement.closest('.col')`, then calls `A.cycleMode`, which calls `render()` and destroys the focused input. On the second Shift+Tab `document.activeElement` is `<body>`, `closest('.col')` is `null`, nothing is prevented, and focus just tabs backwards out of the app.

The UI advertises it: line **1056** renders `(shift+tab to cycle)`. It also never works for a `need` worktree (no input to focus) and never works in Canvas (no `.col`, no input).

Fix: capture `col.dataset.id` before render and re-focus that column's input afterwards, e.g. `const id=col.dataset.id; A.cycleMode(...); $('#tty-'+id)?.parentElement.querySelector('input')?.focus();`. Line **1239**'s `const b=curB()` is unused dead code.

### 6. Merge ↑ / ↓ buttons appear to do nothing (and silently reorder Develop instead)
`A.reorder` lines **738–742** swaps inside the full `b.worktrees` array, but the list it renders (line **1135**) is `b.worktrees.filter(w=>w.ready)`.

User: Merge module, click ↑ on the *docs* worktree (the only `ready` one in seed data).
Result: `w3` swaps with `w2` in the underlying array. The READY list still shows one item labelled `1`. Nothing changes on screen — but the Develop column order silently changed. With two ready items separated by a non-ready one, ↑/↓ also move the wrong pair.

Fix: reorder within the filtered ready list, then write that order back — find the neighbour's index by `b.worktrees.indexOf(readyList[k+dir])` and swap those two.

### 7. Four controls that look actionable and have no handler at all
- **line 1155** `<button class="btn ghost">Review diff</button>` — Merge module, no `onclick`. Dead.
- **line 1189** `<button class="btn ghost">Change access</button>` — Deploy module, in a card with no `onclick` either. Dead.
- **line 781** Authority modal: each of the 7 rows renders `ask / wait / tell / full` as `<span class="kbd" style="cursor:pointer;…">` with **no handler**. They carry a pointer cursor and a highlighted "current" value, so they read as a working permission matrix. Clicking any of them does nothing; the modal is read-only.
- **line 1153/1154** ↑/↓ — see defect 6.

(For contrast, the handler-less `Open` buttons at lines **919** and **1199** *do* work, via bubbling to the parent `.card.click`. Those are fine.)

Fix: wire `Review diff` to a diff modal, `Change access` to an access modal, and make the authority chips call something like `A.setAuthority(rowIndex, level)` backed by state.

### 8. Palette: the highlight never moves and arrow keys do nothing
`A.palette` lines **756–767**. `.pitem.on` is baked in at `i===0` (line **758**) and is never recalculated. `oninput` (line **762**) only toggles `display`. `onkeydown` (line **765**) handles `Enter` only.

User: press ⌘K, type `verity`, press ↓ then Enter.
Result: ↓ does nothing (no handler), and the visible highlight is still on the now-hidden first row while Enter runs the *first visible* row. Filtering and Enter both work correctly in isolation; the highlight just lies about which item Enter will run. Empty result sets give no feedback.

Fix: add ArrowUp/ArrowDown handling that moves `.on` across visible items, and make Enter run the `.on` item rather than "first visible".

**Apostrophe/`eval` check — clean today, but fragile.** Line **758** writes `data-run="${esc(it.run)}"` and Enter does `eval(first.dataset.run)`. `esc` (line **448**) escapes `& < > "` but **not** `'`. That is currently safe only because every `run` string interpolates IDs (`p-xxx`, `b-xxx` from `uid()`), never user-typed names — names appear only in the escaped display text. The moment anyone puts `x.w.name` or `p.name` into a `run` string, a project named `Dylan's app` breaks the attribute and throws a SyntaxError from `eval`. Fix: drop `eval` and store `{fn, args}` on the element via a closure or a `data-idx` into the `items` array.

### 9. "edit" on a settled Discover answer deletes it and everything after it
`A.editAnswer` line **695–697**: `b.discover.answers.splice(i)` — one-argument splice truncates from `i` to the end.

User: on `feature/evidence-upload` → Discover, click **edit** on the first settled answer.
Result: all three recorded answers are destroyed, `done` flips to false, and the branch is thrown back to "QUESTION 1 OF 3" — asking `A.QUESTIONS[0]` ("What is this branch for?"), which is not the question that was answered. No undo, no pre-filled text.

Fix: keep the answer, seed the free-text box with it, and replace in place: `b.discover.answers.splice(i,1)` at minimum, or better, an inline edit that preserves `q`.

### 10. `ready` is never set by any action — the Merge module is effectively read-only
`ready:true` appears only once, in the seed (`w3`, line **380**). No action sets it. There is no "mark done", and `A.sendTo` / `A.answerPerm` only ever set `status` to `run`/`need`.

Consequence: merge `docs`, and the Merge module reads "Nothing is finished yet." for the rest of the session, no matter how many worktrees you create or how much they do. The `merge` button in `colHeader` (line **1040**) can likewise never appear on a worktree you created.

Related: line **1140** `Merge in this order` only renders when `ready.length>1` but calls `A.mergeOne(ready[0].id)` — it merges one item, not the queue. Unreachable with seed data; wrong as soon as it is reachable.

Fix: have the tick or `sendTo` flip a worktree to `ready` when it goes idle, and make "Merge in this order" iterate the ready list.

### 11. Rollback is live on a branch that was never deployed
`A.rollback` line **729** toasts unconditionally; `viewDeploy` line **1172** always renders the button.
User: new project → Deploy → Rollback. Result: toast reads `Rolled back  to the previous version` (empty `b.env`). Fix: `disabled` when `!b.env || !b.version`, and interpolate a fallback.

### 12. Clicking a specific agent in the rail does not take you to that agent
Line **826**: every `.wrow` fires the identical `A.openBranch(pid,bid,'develop')`. `.wrow.sel` is styled (line **68**) but never applied; `A.createWorktree` writes `S.ui.wid` (line **614**) and nothing ever reads it; `S.ui.focusCol` and `.col.focus` (line **137**) are likewise dead.
With 3+ columns, clicking *docs* in the rail lands you on the leftmost column with no indication of which agent you picked. Fix: set `S.ui.wid` on click, mark the matching `.col` with `focus`, and `scrollIntoView` it in `afterDevelop`.

---

## Smaller, still real

- **No Enter-to-submit in any modal.** `#npName` (479), `#nbName` (564), `#nwTask` (583) have no `onkeydown`; every creation flow requires a mouse trip to the primary button.
- **`(esc to interrupt · 18s)`** is printed by `renderTty` line **1095** but Escape (line **1236**) only ever closes modals. The stated affordance is decorative.
- **`A.sendTo` loses focus after every message** (line **662** `render()`), so the composer needs a re-click per message. Same root cause as defect 3.
- **`A.setEnv` leaves a stale URL**: line **720** only sets `b.url` when it is falsy, so renaming `staging`→`preview` keeps `staging.…panacea.kpmg` displayed at line **1184**.
- **`A.openMerge` (line 732) is never referenced** by any handler — dead code.
- **Canvas cannot scroll**: `.canvas` is `overflow:hidden` (line **177**) while tiles are laid out at `24 + floor(i/2)*330` px, so the 5th worktree onwards is unreachable in that layout.
- **`A.createProject` accepts a blank name**: line **493** `(value||'Untitled project').trim()` trims *after* the fallback, so a whitespace-only name yields `''` and an empty `ini` badge. Swap to `.trim() || 'Untitled project'`.

---

## Verified working (no defect)

- **Modal behaviour is correct.** Scrim click closes (`closeModal` line **746** guards `e.target.id!=='scrim'`); clicks inside are stopped by the `onclick="event.stopPropagation()"` on `.modal` (line **745**); footer buttons call `A.closeModal()` with no arg, which bypasses the guard and closes; Escape closes via the synthetic `{target:{id:'scrim'}}` (line **1236**). No case where an inside click wrongly closes.
- **Every `A.*` referenced from markup exists** with matching arity. I checked all 71 handlers: `goHome, palette, closeModal, importRepo, createProject, analyseRepo, pickStart, doImport, newProject, newBranch, createBranch, newWorktree, createWorktree, closeWorktree, answerPerm, sendTo, cycleMode, answerDiscover, discoverFree, editAnswer, skipDiscover, addScreen, designSend, handToDevelop, setEnv, deployNow, toggleAuto, rollback, mergeOne, reorder, setMod, setLayout, openProject, openBranch, toggleProject, toggleBranch, authority, runLocally` — no missing function, no wrong-argument call.
- **Permission-prompt options are clickable and correctly wired.** Line **1106** passes `(p.id, b.id, w.id, i, j)` where `i` is the true `w.tty` index and `j` the option index; `answerPerm` maps `j=1`→`mode='bypass'` and `j=2`→deny, matching the option text. The apostrophe in *"Yes, and don't ask again…"* is safe — it lands in element text via `esc(o)`, not in the attribute.
- **Discover option escaping works**: line **945** `esc(o).replace(/'/g,"\\'")` correctly backslash-escapes for the single-quoted JS string inside a double-quoted attribute (no current option contains one).
- **Navigation is fully reversible** apart from defect 1: home → project → branch → module, back via the sub-rail `← project` (line **856**) and the rail header `A.goHome()` (line **279**). Every module is reachable from the sub-rail at all times, including from a half-finished Discover.

=====AGENT BREAK=====

## Requirement scorecard

| Requirement | Status | Evidence |
|---|---|---|
| Rail: projects > branches > worktrees | **Real** | `renderRail()` L806-838. Tree, thread lines, status dots, inline `＋ worktree` / `＋ branch`. |
| Live sessions streamed into columns or canvas | **Half** | `viewDevelop()` L996 renders both layouts; `setInterval` L1214 appends fake chatter. Canvas mode is a downgrade, not a peer (see D2, D4). |
| Faithful Claude Code TUI (own perm prompt, tool rendering, input box, mode line) | **Mostly real, one dead end** | `renderTty()` L1066 is the strongest part of the file: `⏺`/`⎿` gutters, right-aligned line numbers, add/del hunk backgrounds, `☐/☒` todos, `✻ Deliberating… (esc to interrupt · 18s)`, ASCII permission box, `╭──╮` prompt frame, `⏵⏵ bypass permissions on (shift+tab to cycle)`. No custom Approve/Deny buttons — the rule was respected. Broken by D1/D3/D5. |
| Four modules, not a rigid pipeline | **Real (five)** | `renderSub()` L848-854 adds Merge. Defensible, but the module list is hard-coded against CLAUDE.md L5 "do not hard-code the assumption that there are exactly four". |
| Branch separately deployable, arbitrary env name | **Real** | `viewDeploy()` L1167, `A.setEnv` L720, free-text field. Good: it is a plain name, not a dropdown of dev/staging/prod. |
| ...with its own access settings | **Faked** | `Change access` button L1189 has no `onclick`. Access is a display-only string in seed data. |
| Multiple worktrees per branch that converge and merge | **Faked at the seams** | `viewMerge()` L1133 exists, but see D6, D7, D8. |
| Agents coordinate so they don't disrupt each other | **Dead in the demo** | `collisions()` L1058 never fires on seed data. See D1. |
| Provenance thread (Define → Design → Develop → Deploy) | **Missing entirely** | `b.discover` is referenced only inside Discover itself (grep: L849, L928-949) plus one cosmetic `Read CLAUDE.md` line at L612. See D2. |
| No progress bars / percent-complete | **Violated** | `colFooter()` L1045 renders `<div class="meter"><i style="width:${w.ctx}%">` plus a literal `38%`. CSS L172-174. |
| One accent, interactive state only | **Mostly held, three leaks** | `.disc .step` L186 (`--accent-hi` on a static "QUESTION 1 OF 3" label), `.advice` L239 (accent ring as decoration), `.wm` L1097 watermark. |
| Status colours separate from accent | **Held, one leak** | `.meter i{background:var(--run)}` L173 uses running-green for context fill — status green as decoration. |
| Elevation = 1px ring + shadow | **Held** | `--e-hair` for hairline level, ring+layered shadow on `.tile`/`.modal`/`.toast`. |
| No counts outside the queue | **Violated, and there is no queue** | Sub-rail ticks L849-853, home card count row L891-897, project rows L916-918. Meanwhile no queue surface exists at all (D3). |

---

## Defects, ranked

**D1 — The coordination story is invisible: `collisions()` can never fire on the seeded data.**
`collisions()` L1058-1063 intersects `w.files`. Seed: `w1.files = ['services/api/scanner.go','services/api/upload.go']` (L336), `w2.files = ['services/api/routes.go','services/api/upload_test.go']` (L366), `w3.files = ['docs/api/upload.md','docs/evidence.md']` (L381). Zero overlap, on every branch in the file.
*User does:* opens Develop on `feature/evidence-upload` — the flagship demo branch.
*Happens:* the `⚠ two worktrees want routes.go` banner (L1009) never renders, the Merge `REPO AGENT` advice card (L1142) never renders, `.mg-item.conflict` styling never applies, and Merge labels every item `· no overlap` (L1152). Worse, the transcript actively contradicts the state: w1 says *"routes.go is locked by another worktree on this branch"* (L360) while w1 does not list `routes.go` and w2 — which does hold it — shows no lock. The single hardest requirement in the brief renders as nothing.
*Fix:* add `services/api/routes.go` to `w1.files`. One token. Then also surface the lock inside the column header (a `🔒 routes.go — held by test harness` chip), not only in a page-level banner.

**D2 — No provenance thread. Discover's output is never referenced again anywhere.**
CLAUDE.md L13 makes this the spine: *"Any feature that breaks that thread is wrong regardless of how good it is in isolation."* Grep confirms `b.discover` appears in `viewDiscover` only. `viewDesign` L958, `viewDevelop` L996, `viewMerge` L1133, `viewDeploy` L1167 contain no reference to the brief, the answers, or the definition of done.
*Concretely:* the Discover question at L678 promises *"This becomes the definition of done, checked at merge."* The branch's answer is *"Evidence is virus-scanned before storage; every upload carries a data classification."* Merge shows nothing about it. `viewDeploy` attaches no spec to `v1.5.0-rc3`.
*Fix:* (a) sub-rail: one line of the branch purpose under `.sbr` instead of the generic `2 worktrees · live at client-preview`; (b) Merge: render the Done-when answers as a checklist above READY TO MERGE, each attributable to the worktree that satisfied it; (c) Deploy: a "what shipped" row citing the Discover answer + the merged worktrees; (d) Develop column header: a small `◇` chip that pops the inherited context.

**D3 — Nothing that needs you is reachable. There is no queue.**
`needCount()` L456 is computed and then printed as flat text in the home subtitle (L878) and per-card (L895). There is no click target. `S.ui.wid` is set in `createWorktree` L614 and **never read**. `.wrow.sel` (CSS L68) and `.col.focus` (CSS L137) and `S.ui.focusCol` (L443) are all defined and never applied. Clicking a worktree in the rail (L826) calls `A.openBranch(p,b,'develop')` — identical to clicking the branch.
*User does:* sees "1 waiting on you" on the home screen, clicks it.
*Happens:* nothing — it is not a button. They must guess which project, expand the branch, open Develop, then horizontally scroll the 420px-min columns to find the amber one. For a fleet cockpit this is the core loop.
*Fix:* make the rail worktree row select and scroll-into-view the matching column (`S.ui.wid` → `.col.focus` ring, `scrollIntoView({inline:'center'})`), and make every "N waiting on you" a link that jumps straight to that agent. That also gives the design system its one legal home for counts.

**D4 — Canvas layout: tiles reset every 8.4s, and you cannot talk to any agent in it.**
`afterDevelop()` L1120-1129 drags tiles by mutating `style.left/top` but the `up()` handler L1126 never writes back to state (contrast `afterDesign()` L988-989, which does persist `s.x/s.y`). Tile positions are recomputed from array index at L1024. The live tick re-renders main every 6 ticks (L1219, ~8.4s).
*User does:* arranges four agent tiles into a layout during a demo.
*Happens:* within ~8 seconds every tile snaps back to the index grid. Separately, the canvas branch (L1023-1026) renders `colHeader + tty + colFooter` and **omits `ccInput`** — no input box, no mode line, so Canvas violates "its own input box and mode line" outright. `.tile .rs-h` resize handle CSS (L182) is defined and never emitted.
*Fix:* persist `w.x/w.y/w.w/w.h` on pointerup; include `ccInput` in the tile; emit the `.rs-h` handle and wire it.

**D5 — Denying a permission traps the agent with no way to give it direction.**
`ccInput` is suppressed whenever `w.status==='need'` (L1033). `answerPerm` choice 2 (L632-635) pushes *"Understood — I will not run that. Tell me what to do instead."* and sets `w.status='need'`.
*User does:* clicks option 3, "No, and tell Claude what to do differently (esc)".
*Happens:* the agent asks for direction and the input box is gone. Permanent dead end — the only escape is closing the worktree. This is also unfaithful to the real TUI, where the prompt lives *in* the input frame and the box is always present.
*Fix:* always render `ccInput`; when a perm prompt is pending, render it inside the input frame and let `1`/`2`/`3`/`Esc` answer it from the keyboard (currently the options are mouse-only, the `❯` marker is hardcoded to `j===0` at L1105 and never moves, and global `Escape` L1236 only closes modals despite the on-screen `(esc)` and `esc to interrupt` affordances at L1095).

**D6 — "Merge in this order" merges exactly one worktree.**
L1140: `${ready.length>1?`<button ... onclick="A.mergeOne('${ready[0].id}')">Merge in this order</button>`}`. `mergeOne` L733 merges one and deletes it from state.
*User does:* has three ready worktrees, clicks the button that promises a sequenced landing — the "converge and merge" requirement.
*Happens:* one merges; two remain; the button label lied. Also `Review diff` (L1155) is dead, and merging **deletes** the worktree from `b.worktrees` with no record and no undo (design system L69: optimistic mutation *with an Undo window*, not deletion). Same for `closeWorktree` L617.
*Fix:* sequence all ready worktrees with a per-item state (merged / rebasing / blocked), keep merged items visible as landed, add an Undo toast.

**D7 — Merge reorder arrows silently do nothing on the seeded branch.**
`reorder()` L738-742 swaps within `b.worktrees`, but the list rendered is the filtered `ready` subset (L1135). In `feature/evidence-upload` the only ready worktree is `w3` at index 2.
*User does:* clicks ↑ on a ready item.
*Happens:* it swaps with the non-ready `w2` behind the scenes; the READY list is unchanged. Looks broken.
*Fix:* reorder within the filtered subset, or merge-sort the swap back into the full array by ready-index.

**D8 — Every live tick destroys what the user is typing and force-scrolls every terminal.**
The tick at L1214 calls `renderMain()` (L1229), which replaces `innerHTML`; `afterDevelop()` L1118 then does `t.scrollTop = t.scrollHeight` on **every** `.tty`. The tick even captures a `scroll` map at L1228 recording who is scrolled up — and then never uses it. Dead code that shows the intent was known.
*User does:* scrolls back through an agent's transcript, or starts typing a message into one column.
*Happens:* within ~8 seconds every pane snaps to the bottom and the half-typed input is wiped with focus lost. Same loss on `A.cycleMode` (shift+tab, L664) and on any `render()`. This breaks CLAUDE.md L42 and DESIGN-SYSTEM L65 verbatim ("never scroll-jump when the user has scrolled up").
*Fix:* append to the existing `.tty` node instead of re-rendering main; preserve input value + selection across renders; honour the already-captured scroll-at-bottom map.

**D9 — Renaming an environment leaves the old URL live.**
`setEnv` L720: `if(b.env && !b.url) b.url = ...`. The URL is only derived when absent.
*User does:* renames `client-preview` → `uat` in the Deploy field.
*Happens:* the URL card (L1184) still reads `client-preview.ctt.panacea.kpmg`. Directly undercuts "arbitrary user-chosen environment name". Also `rollback()` L729 is a toast that changes no state — version and `deployedAgo` stay put.
*Fix:* recompute `b.url` whenever `b.env` changes (and show the old URL as a redirect note); make rollback decrement version and stamp `deployedAgo`.

**D10 — Editing one Discover answer silently discards every answer after it.**
`editAnswer(i)` L695-696: `b.discover.answers.splice(i)` truncates the tail and flips `done=false`.
*User does:* clicks `edit` on "Purpose of this branch" to fix a typo.
*Happens:* "Who it is for" and "Done when" vanish without warning; the branch drops out of complete. Nothing is recoverable.
*Fix:* edit in place — replace `answers[i]` and keep `done`.

**D11 — Dead controls that a stakeholder will click.**
`Authority` modal L773-786: the seven `ask|wait|tell|full` chips are `<span class="kbd" style="cursor:pointer">` with no handler — the entire "agents coordinate / blast radius" control surface is a picture. `Change access` L1189, `Review diff` L1155, and the `Open` buttons at L919/L1199 (inert children of an already-clickable card) are all no-ops. `Run locally` L769 is a toast only. The rail's `LOCAL · this Mac` header L836 has nothing under it — and says "this Mac" in a Windows-estate, AKS-hosted enterprise product.

**D12 — Keyboard is not a primary input, contrary to the design system.**
No `:focus-visible` rule anywhere in the stylesheet (grep: 0 matches; only `.fld:focus` L255). No rail keyboard navigation. No `1/2/3` on Discover picks despite the numbered `.k` chips at L946 implying it. `shift+Tab` (L1237) only cycles mode when focus is already inside a `.col` and then destroys that focus by re-rendering. The palette (L747) is global, not "scoped to the current selection" as the design system specifies, and uses `eval(first.dataset.run)` at L766.

---

## The 8 highest-value things to do next, ranked by demo impact

1. **Make the file-lock collision real (D1) and put it in the column header.** One-line seed fix unlocks the banner, the Merge advice card and the conflict styling. This is the requirement no competitor demo has; right now it renders as nothing. ~15 minutes.
2. **Fix the terminal tick: no scroll-jump, no lost input (D8).** Everything else in Develop is undermined by it — a stakeholder typing into an agent will hit it within ten seconds. Append-in-place + preserve input. ~1 hour.
3. **Wire the provenance thread (D2).** Branch purpose in the sub-rail, the Done-when checklist in Merge, the spec cited on the release in Deploy, the inherited-context chip on each column. This is the brief's stated spine and its absence is the biggest gap between the prototype and the product. ~2 hours.
4. **Build the "waiting on you" queue and make the rail select an agent (D3).** Home queue → click → the exact column focused and scrolled into view. Turns a tree browser into a fleet cockpit, and gives the counts a legal home. ~2 hours.
5. **Always show the input box; make the permission prompt keyboard-answerable (D5).** Fixes the deny dead end and makes the TUI actually faithful — `1/2/3`, `Esc`, moving `❯`. ~1 hour.
6. **Bring Canvas to parity (D4): persistent tile positions, input box, working resize handle.** Right now Canvas is a demo trap — it looks like the cube.computer money shot and falls apart on contact. ~2 hours.
7. **Make Merge converge properly (D6, D7): sequenced multi-merge, real reorder, landed history, Undo.** "Multiple worktrees which must converge" is currently a delete button. ~2 hours.
8. **Replace the `ctx%` progress meter with a compliant signal, and kill the dead controls (D11).** The meter is a literal progress bar in green, breaking two stated rules on the most-visible chrome in the app; a sparkline or a three-state hairline (`fresh / filling / near limit`) says the same thing. Then either wire Authority's chips or remove the button. ~1 hour.

Deliberately not in the top 8: film grain (DESIGN-SYSTEM L53), `view-transition-name` drill-in (L70), OKLCH tokens, and heartbeat phase-locking (`.live` at 2.4s vs `.cursor` at 1.05s are two clocks, not one). All real gaps against the design system, none of which a stakeholder will notice before the eight above.