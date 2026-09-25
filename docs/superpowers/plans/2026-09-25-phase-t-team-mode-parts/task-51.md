### Task 51: The prototype of every team surface, owner-reviewed

Nothing in team mode's cockpit is drawn yet (`05-design.md:939`, *"Nothing here is drawn yet."*),
and the working agreement is that **the owner reviews a prototype before any implementation task
starts** (`CLAUDE.md`, working agreements; `docs/DESIGN-SYSTEM.md` exists because the first
prototype *"failed by looking machine-generated"*). This task draws every team surface in one
self-contained file, `prototype/team-mode.html`, beside the existing `prototype/work-layer.html`
and `prototype/phase-4-discover.html`, with the same tokens so it reads as one product. Tasks
53–56 do not start until the owner has approved it; task 52's read model may run in parallel,
because the prototype's sample data is written **in ST.8's field names** (task 52) so the owner
reviews the data the cockpit will really receive, not a mock shaped for the drawing.

The surfaces, each from `05-design.md` §9 and the task scopes 53–56:

1. **The team strip** (§9.1): one band at the top of a team project's view, at most 5 rows, then
   *"+N more"*; per row one dot on the global clock, the name, the **assembled** line (brief section
   title · task title · agent count), up to three observed paths and a count, and on the right one
   link, *"ask mine ↗"*. Zero state: *"Only you on this project."* on one line.
2. **Click-in** (§9.2): the row expands in place into tasks with their brief-section links, agents
   as STATE / MEANING / NEXT cards (`docs/AGENTIC-UX.md:148`), files and leases with an open
   conflict marked, and one action, *"Ask my Orchestrator about this"*.
3. **The four Desk row kinds** (§9.3), each inside the Desk's existing section and nowhere else:
   HORIZON, NEEDS YOU, SINCE YOU LAST LOOKED, DECIDED WITHOUT YOU.
4. **The Orchestrator panel** (§9.4): the conversation, inline *"→ told <worker>: …"* directives,
   the open-thread list as typed state, one opened thread with the author class on every line,
   and DIVE (owner only).
5. **Share, route and spend** (§9.5): the Share popover's added line, the route badge in its three
   forms (`Global`, `AU`, `AU · required`), the ratchet action and its strip announcement, the
   data-class choice in the project-create dialog (OD-24), and the spend line in project settings.

It changes no product code. It is a picture the owner can open by double-click, with a prototype-
only state switcher so every state is one keystroke away.

**Files:**
- `prototype/team-mode.html` — the prototype, one file, no external request (new)
- `prototype/checks/team-mode.check.mjs` — a Node script, no dependencies, that reads the HTML as
  text and enforces the design rules below; exit 0 or 1 (new)
- `docs/verification/2026-09-25-phase-t-prototype-review.md` — the owner's verdict, in the owner's
  words, with the date and the file's commit sha (new, written after the review, not before)

**Read before drawing.** `docs/DESIGN-SYSTEM.md` in full (90 lines) and `docs/AGENTIC-UX.md` §5 and
the "Designing for many live processes" rules (`docs/DESIGN-SYSTEM.md:57-63`). The tokens block is
copied from `prototype/work-layer.html:14-44` (surfaces, the ink ramp, `--ink-label`), which itself
matches `prototype/app.html`; the clock period is phase 4's `--beat:2400ms`
(`prototype/phase-4-discover.html:50`). Nothing from any canvas, Linear or Plane source is opened
(OD-11); the drawing is from the design text and the ASCII in `05-design.md:946-952` only.

**Type.** OD-28 (`docs/superpowers/planning/2026-09-23-owner-decisions.md:165`): every id, path and
time in Commit Mono; all other text in Segoe UI Variable. The file makes no network request, so the
stacks name the installed faces and fall back:

```css
:root{
  --sans:"Segoe UI Variable Text","Segoe UI Variable","Segoe UI",system-ui,sans-serif;
  --mono:"Commit Mono","CommitMono","Cascadia Mono",Consolas,ui-monospace,monospace;
}
```

Whether Commit Mono is installed on the owner's machine is **UNVERIFIED**; the review record
says which face the owner actually saw (the prototype prints the resolved family of one mono
element in its footer, read with `document.fonts.check('12px "Commit Mono"')`).

**Interfaces:**
- Consumes: nothing at runtime. The sample data's **keys** are ST.8's (task 52's
  `control-plane/internal/api/team.go` and `desktop/src/store/types.ts` team block), written
  here as a JavaScript object so that a rename in task 52 is a visible diff in this file too.
- Produces: the approved picture tasks 53–56 build to, and the review record that gates them.

#### The sample data (complete; the prototype renders only this)

One project, `pj-sample`, whose viewer is Dylan. Every free-text value carries its author class
exactly as ST.8 does (`author`, `authorAgentId`, `label`), so the prototype can show the class
beside every agent-written line. The values are invented; no real name, repo or record.

```js
const T = (body, author, label, ownerUserId, authorAgentId = null) =>
  ({ body, author, authorAgentId, ownerUserId, at: "2026-09-25T04:12:09.000Z", label });

const SAMPLE = {
  strip: { v: 1, projectId: "pj-sample", team: true, generatedAt: "2026-09-25T04:12:10.000Z",
    route: { current: "global", required: false, raisedAt: null, canLower: false },
    people: [
      { userId: "u-alice", displayName: "Alice", you: false, status: "working", waitingOn: null,
        line: { briefSection: { ref: "bs-4f1c", title: "Sessions survive restart" },
                taskTitle: T("Persist session tokens across a restart", "agent",
                             "Alice's Orchestrator (agent)", "u-alice", "ag-a1orch"),
                agents: 2 },
        files: { recent: ["auth/session.go", "store/cache.go", "store/ttl.go"], more: 3,
                 heldByOthers: ["auth/session.go"] },
        changedAt: "2026-09-25T04:12:04.000Z" },
      { userId: "u-dylan", displayName: "Dylan", you: true, status: "working", waitingOn: null,
        line: { briefSection: { ref: "bs-77a0", title: "Mobile sign-in" }, taskTitle: null, agents: 1 },
        files: { recent: ["mobile/auth/login.ts"], more: 0, heldByOthers: [] },
        changedAt: "2026-09-25T04:09:51.000Z" },
      { userId: "u-priya", displayName: "Priya", you: false, status: "waiting",
        waitingOn: { kind: "human", name: "Priya",
                     words: T("Need a decision on the PDF font licence", "agent",
                              "Priya's export worker (agent)", "u-priya", "ag-p1w1") },
        line: { briefSection: { ref: "bs-9c02", title: "Evidence export" }, taskTitle: null, agents: 1 },
        files: { recent: ["export/pdf.go"], more: 0, heldByOthers: [] },
        changedAt: "2026-09-25T03:58:40.000Z" },
    ] },
  desk: { v: 1, generatedAt: "2026-09-25T04:12:10.000Z",
    horizon: [ { conflictId: "cf-3d9e", projectId: "pj-sample", tier: "H", state: "negotiating",
      paths: ["auth/session.go"], mine: { agentId: "ag-d1w1", name: "mobile worker" },
      theirs: { userId: "u-alice", displayName: "Alice", agentId: "ag-a1w2" },
      at: "2026-09-25T04:10:02.000Z" } ],
    needsYou: [ { escalationId: "es-51aa", projectId: "pj-sample", threadId: "th-8b21", fromAgentId: "ag-d1orch",
      question: T("Alice's side wants auth/session.go first. Let them go first?", "agent",
                  "Your Orchestrator (agent)", "u-dylan", "ag-d1orch"),
      options: [
        { label: T("Let Alice go first", "agent", "Your Orchestrator (agent)", "u-dylan", "ag-d1orch"),
          consequence: T("Your mobile worker pauses on auth/ until Alice's lane lands", "agent",
                         "Your Orchestrator (agent)", "u-dylan", "ag-d1orch") },
        { label: T("Keep going, both", "agent", "Your Orchestrator (agent)", "u-dylan", "ag-d1orch"),
          consequence: T("Both edit the file; the land review shows the overlap", "agent",
                         "Your Orchestrator (agent)", "u-dylan", "ag-d1orch") } ],
      default: 0, deadline: "2026-09-25T04:40:00.000Z", at: "2026-09-25T04:05:12.000Z" } ],
    sinceYouLastLooked: { since: "2026-09-24T22:41:00.000Z", items: [
      { kind: "inform", escalationId: "es-51ab", projectId: "pj-sample",
        question: T("Yielded store/cache.go to Alice's side; nothing lost.", "agent",
                    "Your Orchestrator (agent)", "u-dylan", "ag-d1orch"), at: "2026-09-25T01:02:11.000Z" },
      { kind: "answer", messageId: "ms-0c4f", threadId: "th-7a10", projectId: "pj-sample",
        to: { userId: "u-priya", displayName: "Priya" },
        text: T("Mobile sign-in does not touch export/.", "agent",
                "Your Orchestrator (agent)", "u-dylan", "ag-d1orch"), at: "2026-09-25T02:30:40.000Z" } ] },
    decidedWithoutYou: [ { escalationId: "es-51ac", conflictId: "cf-2b70", projectId: "pj-sample",
      resolution: "yield", other: { userId: "u-alice", displayName: "Alice" },
      globs: ["store/cache.go"], at: "2026-09-25T01:02:10.000Z", undo: { available: true },
      summary: "DECIDED WITHOUT YOU: your side: yielded store/cache.go to Alice (cf-2b70)." } ] },
};
```

The Orchestrator panel's sample, the same `T` helper, and the three settings rows:

```js
SAMPLE.orchestrator = { v: 1, projectId: "pj-sample",
  orchestrator: { agentId: "ag-d1orch", status: "idle", canDive: true },
  conversation: [
    { kind: "human", id: "hm-01aa", at: "2026-09-25T04:01:00.000Z",
      text: T("Ask Alice's side whether they need store/cache.go today.", "human", "You", "u-dylan") },
    { kind: "directive", id: "dv-01ab", at: "2026-09-25T04:01:31.000Z", directiveKind: "free",
      to: { agentId: "ag-d1w1", name: "mobile worker" }, params: { globs: [], leaseId: null },
      text: T("Leave store/ alone until I say otherwise.", "agent", "Your Orchestrator (agent)",
              "u-dylan", "ag-d1orch") },
    { kind: "reply", turnId: "ot-01ac", at: "2026-09-25T04:01:40.000Z", exit: "ok", truncated: false,
      text: T("I asked Alice's Orchestrator about store/cache.go and told your mobile worker to stay "
            + "out of store/. See [the thread](https://example.invalid) for their answer.", "agent",
              "Your Orchestrator (agent)", "u-dylan", "ag-d1orch") } ],
  threads: [ { id: "th-8b21", state: "open", other: { userId: "u-alice", displayName: "Alice" },
    subject: { kind: "conflict", id: "cf-3d9e" }, subjectLabel: "auth/session.go",
    round: 2, maxRounds: 3, deadline: "2026-09-25T04:40:00.000Z" } ] };

SAMPLE.thread = { thread: SAMPLE.orchestrator.threads[0], messages: [
  { id: "ms-0a01", kind: "propose", templated: false, at: "2026-09-25T04:02:10.000Z", tombstoned: false,
    sender: { agentId: "ag-a1orch", userId: "u-alice", label: "Alice's Orchestrator (agent)" },
    fields: { subject: null, inReplyTo: null, proposal: null, resolution: { kind: "sequence", first: "ag-a1w2", sides: {}, globs: [], base: null, paths: [] } },
    text: T("We finish auth/session.go first; ours lands within the hour.", "agent",
            "Alice's Orchestrator (agent)", "u-alice", "ag-a1orch") },
  { id: "ms-0a02", kind: "counter", templated: false, at: "2026-09-25T04:03:02.000Z", tombstoned: false,
    sender: { agentId: "ag-d1orch", userId: "u-dylan", label: "Your Orchestrator (agent)" },
    fields: { subject: null, inReplyTo: null, proposal: null, resolution: { kind: "split", first: null, globs: [], base: null, paths: [],
              sides: { "ag-a1w2": ["auth/session.go"], "ag-d1w1": ["mobile/**"] } } },
    text: null } ] };

SAMPLE.settings = { v: 1, projectId: "pj-sample", dataClass: "internal",
  route: { current: "global", required: false, raisedAt: null, canLower: false },
  spend: { day: "2026-09-25", workersCents: 1840, coordinationCents: 310, quotaCents: 5000 } };
```

The thread sample carries an agent's claim of time (*"within the hour"*) on purpose: the prototype
shows it as an agent's quoted words beside its author class, which is the only way an agent's
estimate may appear (OD-27 bans the product's own ETAs, not a quoted agent sentence).

#### The states the switcher shows

The switcher is a fixed strip at the bottom right, marked *"prototype only"*, and keys `1`–`9`
select a state. It is not a product element and task 53 does not build it.

| Key | State | Built from | What the owner checks |
|---|---|---|---|
| `1` | Solo project | `strip.team = false` | no band at all, and the header badge reads `Global` with *"Use AU for this project"* |
| `2` | Team, only you | `people` holding Dylan only | the band is one line, *"Only you on this project."* |
| `3` | Team, three people | `SAMPLE.strip` | the assembled line; the 1px ring on `auth/session.go`; Priya's `○` and *"waiting on Priya"* with her agent's words on hover |
| `4` | Six people | `people` × 6, generated | five rows, then *"+1 more"*; one clock, not six |
| `5` | Click-in on Alice | `SAMPLE.strip` + a detail | tasks with brief links; STATE / MEANING / NEXT; one action; no steer control |
| `6` | The Desk | `SAMPLE.desk` | the four rows inside the existing sections; answering in place; no toast |
| `7` | The Orchestrator panel | `SAMPLE.orchestrator` | inline *"→ told mobile worker: …"*; the markdown link shown as text; the thread list |
| `8` | A thread open | `SAMPLE.thread` | the author class on every line; the counter's typed split rendered from fields |
| `9` | Share, route, spend, create | `SAMPLE.settings` | the Share line; `AU · required` with no switch; the ratchet and its announcement; the data class at creation; the spend line in words and figures |

State `3` ticks: every 4 s the sample moves Alice's newest path to the front, which is the only
way the owner can see the ~600 ms flash and decay (`docs/DESIGN-SYSTEM.md:63`). It changes
`changedAt` and nothing else; the heartbeat clock keeps its phase.

#### `prototype/checks/team-mode.check.mjs` (complete)

The design rules the owner should not have to catch by eye, checked as text. It is the prototype's
test: it runs before the owner opens the file, and its `--self-test` proves every rule can fail.

```js
// Checks prototype/team-mode.html against the design rules as text. Exit 0 or 1.
// Usage: node prototype/checks/team-mode.check.mjs [file] | --self-test
import { readFileSync } from "node:fs";

const styleOf = (h) => [...h.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]).join("\n");
const scriptOf = (h) => [...h.matchAll(/<script[^>]*>([\s\S]*?)<\/script>/g)].map((m) => m[1]).join("\n");
const textOf = (h) => h.replace(/<style[\s\S]*?<\/style>|<script[\s\S]*?<\/script>|<[^>]+>/g, " ");

export const rules = [
  ["no transition: all", (h) => !/transition(-property)?\s*:\s*all\b/.test(styleOf(h))],
  ["transitions animate transform/opacity only", (h) =>
    [...styleOf(h).matchAll(/transition(?:-property)?\s*:\s*([^;}]+)/g)].every((m) =>
      m[1].split(",").every((p) => /^\s*(transform|opacity|none)\b/.test(p)))],
  ["keyframes touch transform/opacity/--beat-o only", (h) =>
    [...styleOf(h).matchAll(/@keyframes\s+[\w-]+\s*\{([\s\S]*?\})\s*\}/g)].every((m) =>
      [...m[1].matchAll(/([\w-]+)\s*:/g)].every((d) => ["transform", "opacity", "--beat-o"].includes(d[1])))],
  ["one heartbeat clock", (h) => (styleOf(h).match(/animation(-name)?\s*:[^;}]*\bbeat\b/g) || []).length === 1
    && /(:root|html)\s*\{[^}]*animation[^}]*\bbeat\b/.test(styleOf(h))],
  ["no spinner", (h) => !/spinner|rotate\(/i.test(h)],
  ["no gradient, no backdrop blur", (h) => !/(linear|radial|conic)-gradient|backdrop-filter/.test(styleOf(h))],
  ["tabular numerals", (h) => /font-variant-numeric\s*:\s*tabular-nums/.test(styleOf(h))],
  ["Commit Mono and Segoe UI Variable", (h) =>
    /--mono\s*:[^;]*Commit Mono/.test(h) && /--sans\s*:[^;]*Segoe UI Variable/.test(h)],
  ["no external request", (h) => !/(src|href)\s*=\s*["']?https?:|@import|url\(\s*["']?https?:|fetch\(/.test(h)],
  ["no innerHTML", (h) => !/\.innerHTML\s*=|insertAdjacentHTML|outerHTML\s*=/.test(scriptOf(h))],
  ["no percentage, bar, ETA or N of M in the markup", (h) => {
    const t = textOf(h) + "\n" + scriptOf(h).replace(/\/\/.*$/gm, "");
    return !/\d\s?%(?!\s*[;,)])/.test(textOf(h)) && !/<progress|role=["']progressbar/.test(h)
      && !/\bETA\b|\bremaining\b/i.test(textOf(h))
      && !/\b\d+ of \d+\b/.test(t.replace(/round \$?\{?[\w.]*\}? of \$?\{?[\w.]*\}?|round \d+ of \d+/g, ""));
  }],
];

const bad = {
  "no transition: all": "<style>a{transition:all 1s}</style>",
  "transitions animate transform/opacity only": "<style>a{transition:width 1s}</style>",
  "keyframes touch transform/opacity/--beat-o only": "<style>@keyframes x{to{height:2px}}</style>",
  "one heartbeat clock": "<style>.dot{animation:beat 2s infinite}</style>",
  "no spinner": "<div class=spinner></div>",
  "no gradient, no backdrop blur": "<style>a{background:linear-gradient(red,blue)}</style>",
  "tabular numerals": "<style>a{}</style>",
  "Commit Mono and Segoe UI Variable": "<style>:root{--mono:monospace;--sans:Arial}</style>",
  "no external request": '<link href="https://fonts.example/x.css">',
  "no innerHTML": "<script>el.innerHTML = x</script>",
  "no percentage, bar, ETA or N of M in the markup": "<p>3 of 5 done</p>",
};

function run(html) {
  const failed = rules.filter(([, ok]) => !ok(html)).map(([name]) => name);
  for (const n of failed) console.log("FAIL " + n);
  return failed;
}

if (process.argv[2] === "--self-test") {
  const missed = rules.filter(([name, ok]) => ok(bad[name])).map(([name]) => name);
  for (const n of missed) console.log("RULE CANNOT FAIL " + n);
  console.log(missed.length ? "self-test failed" : `self-test ok: ${rules.length} rules`);
  process.exit(missed.length ? 1 : 0);
}
const file = process.argv[2] || "prototype/team-mode.html";
const failed = run(readFileSync(file, "utf8"));
console.log(failed.length ? `${failed.length} rule(s) failed` : `ok: ${rules.length} rules`);
process.exit(failed.length ? 1 : 0);
```

The one `N of M` the prototype prints is the thread's *"round 2 of 3"*, and the check exempts
exactly that phrase. It is a hard cap Fleet enforces (task 28's limits), not a completion
prediction, which is how this plan reads OD-27; the review record asks the owner to confirm it.

Every timing function in the file is `var(--ease)`, never an inline `cubic-bezier(…)`, because the
transition rule splits a declaration on commas.

- [ ] **Step 1: Write the check first, and watch it fail**

Write `team-mode.check.mjs` as above. Each rule is a test; the mutant it kills is the drawing
mistake named in the table.

| Rule | Mutant it kills |
|---|---|
| no `transition: all` | a blanket transition added "for smoothness" (`CLAUDE.md` frontend rules) |
| transitions animate `transform`/`opacity` only | a row that animates `height` on click-in, which reflows every row below it |
| keyframes touch `transform`/`opacity`/`--beat-o` only | a flash that animates `background` or `width` |
| one heartbeat clock | a `.dot{animation:beat …}` per row, each phased by when it mounted (`docs/DESIGN-SYSTEM.md:61`) |
| no spinner | a spinner on the Orchestrator's *"thinking"* state |
| no gradient, no backdrop blur | the indigo hero and blanket glass the first prototype failed on |
| tabular numerals | deadlines and counts that jitter as they change |
| Commit Mono and Segoe UI Variable | OD-28's faces dropped for the old Berkeley-first stack |
| no external request | a Google Fonts link, which the corporate network may block and which breaks "one file" |
| no `innerHTML` | an agent's `text·A` rendered as HTML, so the sample's markdown link becomes live (§7.3) |
| no percentage, bar, ETA or N of M | HORIZON's *"~20 min"* (`05-design.md:992`) copied in, which OD-27 removes |

```bash
node prototype/checks/team-mode.check.mjs --self-test && node prototype/checks/team-mode.check.mjs
```

Expected: `self-test ok: 11 rules`, then `ENOENT` for the missing HTML. That is the red.

- [ ] **Step 2: The frame, the tokens and the clock**

`prototype/team-mode.html`: the tokens block, the two font stacks, and one registered property
animated on `:root`, which every live dot reads, so ten dots share one phase:

```css
@property --beat-o { syntax: "<number>"; inherits: true; initial-value: 1; }
:root { animation: beat var(--beat) var(--ease) infinite; }
@keyframes beat { 0%, 100% { --beat-o: 1; } 50% { --beat-o: .45; } }
.dot.live { opacity: var(--beat-o); }
@media (prefers-reduced-motion: reduce) { :root { animation: none; } }
```

The page frame is the cockpit's: the rail on the left with **nothing added** (`docs/AGENTIC-UX.md:144`),
a project header with the route badge, then the project view. Sample data is rendered by building
DOM nodes with `textContent`; one function, `authored(text)`, renders every `text·A`/`text·H`
value as `<span class="said">` plus its `label` in `--ink-label`, and is the only path an
agent's string takes to the screen.

- [ ] **Step 3: States 1–5, the strip and click-in**

The strip as §9.1's ASCII (`05-design.md:946-952`): `ON THIS PROJECT · 3` in caps, rows at
`--row-h`, the name in sans, paths and counts in mono. A path in `heldByOthers` gets
`box-shadow: 0 0 0 1px var(--line-strong)` and nothing else. `waitingOn.kind === "human"` prints
*"waiting on <name>"*, `lease` prints *"waiting on a lease"*, `peer` *"waiting on a thread"*.
The flash is a `::after` overlay whose `opacity` goes 1 → 0 over `--dur-flash` when `changedAt`
moves. Click-in uses `view-transition-name` where supported (`docs/DESIGN-SYSTEM.md:70`) and a
plain swap otherwise. Its one action opens state 7 with the subject shown as a chip.

- [ ] **Step 4: States 6–9**

The Desk renders each team row in its section: HORIZON in the horizon bar; NEEDS YOU as the
decide card (question, options with consequences, the safe default marked, the deadline as a clock
time `04:40` in mono, never a countdown); SINCE YOU LAST LOOKED as one batched list under the
section's own header; DECIDED WITHOUT YOU with `[Undo]` only where `undo.available`. The
Orchestrator panel, the thread, and state 9 follow the table above.

- [ ] **Step 5: Acceptance**

```bash
node prototype/checks/team-mode.check.mjs --self-test && node prototype/checks/team-mode.check.mjs
```

Expected: `self-test ok: 11 rules` and `ok: 11 rules`. Then the owner opens the file by
double-click in Edge and walks keys `1`–`9`. **Visual acceptance is the owner's eyes**, not this
check and not a screenshot (`CLAUDE.md`; headless screenshots are blocked on the corporate network).

- [ ] **Step 6: Commit, review, record**

```bash
git add prototype/team-mode.html prototype/checks/team-mode.check.mjs
git commit -F <msgfile> -- prototype/team-mode.html prototype/checks/team-mode.check.mjs
```

After the review, `docs/verification/2026-09-25-phase-t-prototype-review.md` records: the date,
the commit sha reviewed, which mono face the footer reported, the owner's verdict in the owner's
words, every change asked for, and the answers to three questions put to the owner: (1) is
*"round 2 of 3"* an allowed cap under OD-27; (2) should `waiting on a lease` name the holder;
(3) does the Desk's DECIDED WITHOUT YOU Undo belong on a `sequence` as well as a `yield`. Changes
asked for are made in the same file and re-reviewed. **Tasks 53–56 start only when the record says
"approved".**

**What this task does NOT claim.**
- That the surfaces work. The file renders sample data and talks to nothing; task 52's read model
  and tasks 53–56's components are the product, and task 57 is the first time two people see it.
- That the drawing is good. The check catches rule breaks, not taste; the owner decides.
- That Commit Mono rendered. The footer says which face did; the record keeps it.
- That the sample data's keys are final. They are ST.8's as task 52 writes them; if task 52's
  review renames a key, both files change in one commit.
