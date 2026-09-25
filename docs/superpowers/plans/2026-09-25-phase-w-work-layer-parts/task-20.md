### Task 20: desktop test kit — `expectNoPrediction` and the Work fixtures

W1, stream WD1, seam **SW.8** (called by every desktop surface task). No dependency: it is pure
test code over the DOM and plain data, so it can start on day one.

`OD-27` is the owner's ruling on progress: show **which** criteria are proven and plain counts per
group; still banned are percentages, progress bars, ETAs and "N of M" completion figures. It
restates `AGENTIC-UX.md`'s anti-pattern 2 (*"No percent-complete, progress bar, ETA, or "N of M
met" anywhere in the product"*, `docs/AGENTIC-UX.md:237`). A rule that every surface must keep is
only kept if every surface test checks it, the same way, with the same exemptions. This task
writes that check once, `expectNoPrediction(container)`, and the fixtures every Work surface
test renders, so a surface task's test is two lines: render the fixture, call the check.

**The one thing this task must not get wrong is the exemptions.** A check that bans `50%` in a
quoted diff, or bans *"waiting 2h 10m"*, will be switched off by the first surface that trips it,
and then it guards nothing. So the check exempts by **explicit attribute only**, never by a
heuristic: `data-verbatim` skips an element whole (transcript, file or diff text, which may
legitimately say `50%`), and `data-projection-rule` allows a time projection that names its rule
(05's answer to Q1, `05-design.md:1358`, *"A projection that names its rule and source span
(next decision ~14:22 · rule: migration gate) is allowed"*, one of Q13's amendments, which scope
20 numbers as the fourth — **UNVERIFIED** which number `AGENTIC-UX.md` will give it) while still checking that
element for percentages, bars and "N of M". Plain counts (*"4 issues"*), clock times
(*"14:02"*), aging (*"waiting 2h 10m"*) and ids (*"EP-142"*) always pass.

**What is on the tree today (verified 2026-09-25).** `desktop/src/test/` holds `fakes.ts`,
`fixtures.ts` and `setup.ts`. `fixtures.ts:1-9` sets the kit's rule: builders are declared
**once**, a variant is an override object, never a near-duplicate or a local builder. `FIXED_NOW`
(`1789722000000`, `2026-09-18T09:00:00.000Z`) and `FIXED_NOW_ISO` are at `fixtures.ts:25-26`.
`desktop/vitest.config.ts:11` runs `./src/test/setup.ts` for every test, in `jsdom`, with
`globals: false`, so every test imports `describe`/`it`/`expect` from `vitest`.

**Files:**
- `desktop/src/test/expectNoPrediction.ts` — the check (new)
- `desktop/src/test/expectNoPrediction.test.ts` — the rows below (new)
- `desktop/src/test/workFixtures.ts` — `WORK_IDS`, the local mirror types, eight builders and
  `workScenario()` (new)
- `desktop/src/test/workFixtures.test.ts` — the grammar, coverage and self-check rows (new)
- nothing else

**Interfaces:**
- Produces (**SW.8**): `expectNoPrediction(container: HTMLElement): void`;
  `findPredictions(container: HTMLElement): Prediction[]` (the same scan, returning instead of
  throwing, for tests that assert a specific finding); `WORK_IDS`; `makeIssueHead`,
  `makeIssueRevision`, `makeServe`, `makeCriterion`, `makeComment`, `makeLabel`,
  `makeTeamPolicy`, `makeWorkEvent`; `workScenario()`; `WORK_STATES`, `PROOF_WORDS`.
- Consumes: `FIXED_NOW`, `FIXED_NOW_ISO` from `fixtures.ts`; task 03's JSON field names
  (`task-03.md:279-398`) and task 02's (`task-02.md:226-244`), mirrored by hand here.
- Required by: every desktop surface task of W1-W5 (tasks 26 onward), task 37 (which replaces the
  local mirror types with imports from `src/api/work.ts` and whose typecheck proves they agree).

#### Contract — `expectNoPrediction.ts`, complete

```ts
/**
 * desktop/src/test/expectNoPrediction.ts — SW.8, task 20.
 * OD-27 as a test: no percentage, no bar or meter, no "N of M", no ETA. Exemptions are by
 * explicit attribute only: data-verbatim (skipped whole) and data-projection-rule (ETA rules
 * waived, every other rule still applied). Plain counts, clock times, aging and ids pass.
 */
export type PredictionRule = "percentage" | "progress-role" | "progress-element" | "valuenow" | "n-of-m" | "eta";
export interface Prediction { rule: PredictionRule; text: string; path: string }

const PERCENT = /\d+(\.\d+)?\s?%/;
const N_OF_M = [/\b\d+\s+of\s+\d+\b/i, /\b\d+\s*\/\s*\d+\s+(done|complete|proven|met|closed)\b/i];
const ETA = [
  /\bETA\b/,
  /\b(\d+\s?(s|m|min|h|d)|\d+\s+(minutes?|hours?|days?))\s+(left|remaining|to go)\b/i,
  /\b(done|ready|finished|complete) by\b/i,
  /\bestimated\b/i,
];
const TEXT_ATTRS = ["aria-label", "aria-valuetext", "title"] as const;
const PHRASING = new Set(["B", "I", "EM", "STRONG", "SPAN", "CODE", "SMALL", "MARK", "ABBR", "TIME", "DATA", "SUB", "SUP", "KBD", "S", "U"]);

function cssPath(el: Element, root: Element): string {
  const parts: string[] = [];
  for (let e: Element | null = el; e && e !== root.parentElement; e = e.parentElement) {
    const same = e.parentElement ? Array.from(e.parentElement.children).filter((c) => c.tagName === e!.tagName) : [e];
    const nth = same.length > 1 ? `:nth-of-type(${same.indexOf(e) + 1})` : "";
    parts.unshift(e.tagName.toLowerCase() + nth);
    if (e === root) break;
  }
  return parts.join(" > ");
}

function textRules(s: string, projection: boolean): PredictionRule[] {
  const hits: PredictionRule[] = [];
  if (PERCENT.test(s)) hits.push("percentage");
  if (N_OF_M.some((r) => r.test(s))) hits.push("n-of-m");
  if (!projection && ETA.some((r) => r.test(s))) hits.push("eta");
  return hits;
}

/** The scan, returning every finding. expectNoPrediction throws when this is non-empty. */
export function findPredictions(container: HTMLElement): Prediction[] {
  const out = new Map<string, Prediction>();
  const add = (rule: PredictionRule, text: string, el: Element) => {
    const p = { rule, text: text.trim(), path: cssPath(el, container) };
    out.set(`${p.rule}|${p.path}|${p.text}`, p);
  };
  const elements = [container, ...Array.from(container.querySelectorAll("*"))];
  for (const el of elements) {
    if (el.closest("[data-verbatim]")) continue;
    const projection = el.closest("[data-projection-rule]") !== null;
    const role = el.getAttribute("role");
    if (role === "progressbar" || role === "meter") add("progress-role", `role=${role}`, el);
    if (el.tagName === "PROGRESS" || el.tagName === "METER") add("progress-element", `<${el.tagName.toLowerCase()}>`, el);
    if (el.hasAttribute("aria-valuenow")) add("valuenow", `aria-valuenow=${el.getAttribute("aria-valuenow")}`, el);
    for (const a of TEXT_ATTRS) {
      const v = el.getAttribute(a);
      if (v) for (const r of textRules(v, projection)) add(r, `${a}="${v}"`, el);
    }
    // Own text nodes, then the joined text of a phrasing-only element, so "<b>3</b> of 5" is
    // caught once, at the element that holds the whole phrase.
    for (const n of Array.from(el.childNodes)) {
      if (n.nodeType === Node.TEXT_NODE && n.textContent) for (const r of textRules(n.textContent, projection)) add(r, n.textContent, el);
    }
    const kids = Array.from(el.children);
    if (kids.length > 0 && kids.every((k) => PHRASING.has(k.tagName) && !k.hasAttribute("data-verbatim"))) {
      const joined = el.textContent ?? "";
      for (const r of textRules(joined, projection)) add(r, joined, el);
    }
  }
  return Array.from(out.values());
}

export function expectNoPrediction(container: HTMLElement): void {
  const found = findPredictions(container);
  if (found.length === 0) return;
  const lines = found.map((p) => `  ${p.rule}: "${p.text}" at ${p.path}`);
  throw new Error(`expectNoPrediction (OD-27, AGENTIC-UX anti-pattern 2) found ${found.length}:\n${lines.join("\n")}`);
}
```

Two properties are deliberate. A phrasing element whose own child is `data-verbatim` is not
joined, so a verbatim span inside a sentence cannot drag its `50%` into the sentence's check. And
the ETA rules are waived under `data-projection-rule` but the percentage, bar and "N of M" rules
are not: a projection may say *when*, never *how far*.

#### Contract — `workFixtures.ts`, part 1: ids and the local mirror types

```ts
/**
 * desktop/src/test/workFixtures.ts — SW.8, task 20.
 * The Work fixtures. fixtures.ts's rule holds: each builder is declared ONCE, here; a variant is
 * an override object; no test declares a local builder. The types below mirror task 03's JSON
 * names by hand until task 37 (SW.15) replaces them with imports from src/api/work.ts.
 */
import { FIXED_NOW } from "./fixtures";

const hex16 = (n: number): string => n.toString(16).padStart(16, "0");
const at = (minutesAfterNow: number): string => new Date(FIXED_NOW + minutesAfterNow * 60_000).toISOString();

/** Hub-shaped ids: prefix + "-" + 16 lower-case hex (hubapi.ValidID); criteria "c-" + 8 hex
 *  (discover.Criterion). Stable, so a failure reads the same in every suite. */
export const WORK_IDS = {
  team: "tm-" + hex16(0xe5), project: "pj-" + hex16(0xa1), brief: "bf-00000000000000a2",
  ep142: "is-3f2a91c07b1e4d2a", ep150: "is-" + hex16(0x150), ep160: "is-" + hex16(0x160),
  ep161: "is-" + hex16(0x161), ep162: "is-" + hex16(0x162), ep163: "is-" + hex16(0x163),
  ep164: "is-" + hex16(0x164), ep165: "is-" + hex16(0x165), ep166: "is-" + hex16(0x166),
  ep167: "is-" + hex16(0x167), ep168: "is-" + hex16(0x168), ep169: "is-" + hex16(0x169),
  comment: "ic-" + hex16(0xc1), label: "lb-" + hex16(0x1b),
  criterionResume: "c-91e2a4b0", criterionUpload: "c-40aa17c3",
  dylan: "u-" + hex16(0xd1), alice: "u-" + hex16(0xa11ce),
} as const;
export const BRIEF_KEY = `brief:${WORK_IDS.brief}@2`;

export type IssueState = "triage" | "backlog" | "todo" | "started" | "in_review" | "landed" | "done" | "cancelled" | "duplicate";
export const WORK_STATES: readonly IssueState[] = ["triage", "backlog", "todo", "started", "in_review", "landed", "done", "cancelled", "duplicate"];
export type Proof = "" | "proven" | "proven_self_attested" | "not_proven" | "no_criteria" | "waiting";
export const PROOF_WORDS: readonly Proof[] = ["proven", "proven_self_attested", "not_proven", "no_criteria", "waiting"];
export type Priority = "none" | "urgent" | "high" | "medium" | "low";

export interface User { id: string; displayName: string; email: string }
export interface AuthoredText { text: string; author: "H" | "A" }
export interface Serve { briefKey: string; criterionId: string }
export interface Criterion { id: string; text: string; digest: string }
export interface ThreadState { kind: "on_thread" | "project_level" | "not_tied"; reason: "" | "chore" | "spike" | "imported" | "support" }
export interface Relation { kind: "blocks" | "duplicates" | "relates"; issueId: string }
export interface ExternalRef { source: string; externalId: string; url: string; authorName: string }
export interface IssueHead {
  id: string; projectId: string; number: number; alias: string; state: IssueState; stateReason: string;
  proof: Proof; landCommit: string; assignee: User | null; priority: Priority; labels: string[];
  relations: Relation[]; headRevision: number; headKey: string; threadState: ThreadState;
  source: "person" | "orchestrator" | "intake"; external: ExternalRef | null;
  createdAt: string; createdBy: User; updatedAt: string;
}
export interface IssueRevision {
  issueId: string; revision: number; key: string; digest: string; projectId: string;
  content: { title: AuthoredText; description: AuthoredText; serves: Serve[]; implements: string[] };
  writtenAt: string; writtenBy: User; restoredFrom: number | null; erasedAt: string | null;
}
export interface Reaction { emoji: string; users: User[] }
export interface Comment {
  id: string; issueId: string; parentId: string | null; kind: "comment"; body: AuthoredText;
  anchor: null; mentions: string[]; reactions: Reaction[]; resolvedAt: string | null;
  resolvedBy: User | null; createdAt: string; createdBy: User; erasedAt: string | null;
}
export interface Label { id: string; teamId: string; name: string; color: string }
export interface TeamPolicy {
  teamId: string; defaultState: IssueState; priorityToLeaveTriage: boolean;
  stopAtLandedWithoutServes: boolean; updatedAt: string | null; updatedBy: User | null;
}
export interface WorkEvent {
  seq: number; at: string; kind: string; teamId: string | null; projectId: string | null;
  subject: string; actor: User | null; data: Record<string, string | number | boolean>;
}
```

`Criterion` is phase 4's `RefCriterion` (`task-15.md:154`, `{id, text, digest}`), not a Hub type:
a criterion lives in a brief. `Comment.anchor` is typed `null` only, because W1's fixtures pin no
anchor; the anchor surface task widens it through task 37's real type. `WorkEvent` is
`hubapi.Event` (`hubapi/types.go:536-545`) with `data` narrowed to task 13's scalar-only rule.

#### Contract — `workFixtures.ts`, part 2: the eight builders

```ts
export const DYLAN: User = { id: WORK_IDS.dylan, displayName: "Dylan Example", email: "dylan@example.test" };
export const ALICE: User = { id: WORK_IDS.alice, displayName: "Alice Example", email: "alice@example.test" };
const DIGEST = "sha256:" + "0".repeat(62) + "a2";
const issueKey = (id: string, rev: number): string => `issue:${id}@${rev}`;

export function makeServe(over: Partial<Serve> = {}): Serve {
  return { briefKey: BRIEF_KEY, criterionId: WORK_IDS.criterionResume, ...over };
}

export function makeCriterion(over: Partial<Criterion> = {}): Criterion {
  return { id: WORK_IDS.criterionResume, text: "the resume test passes", digest: DIGEST, ...over };
}

export function makeIssueHead(over: Partial<IssueHead> = {}): IssueHead {
  const id = over.id ?? WORK_IDS.ep142;
  const rev = over.headRevision ?? 3;
  return {
    id, projectId: WORK_IDS.project, number: 142, alias: "EP-142", state: "todo", stateReason: "",
    proof: "", landCommit: "", assignee: DYLAN, priority: "high", labels: [], relations: [],
    headRevision: rev, headKey: issueKey(id, rev), threadState: { kind: "on_thread", reason: "" },
    source: "person", external: null, createdAt: at(0), createdBy: DYLAN, updatedAt: at(0),
    ...over,
  };
}

export function makeIssueRevision(over: Partial<IssueRevision> = {}): IssueRevision {
  const issueId = over.issueId ?? WORK_IDS.ep142;
  const revision = over.revision ?? 3;
  return {
    issueId, revision, key: issueKey(issueId, revision), digest: DIGEST, projectId: WORK_IDS.project,
    content: {
      title: { text: "Resume a session after a restart", author: "H" },
      description: { text: "A restarted sandbox must pick up the same conversation.", author: "H" },
      serves: [makeServe()], implements: [],
    },
    writtenAt: at(0), writtenBy: DYLAN, restoredFrom: null, erasedAt: null,
    ...over,
  };
}

export function makeComment(over: Partial<Comment> = {}): Comment {
  return {
    id: WORK_IDS.comment, issueId: WORK_IDS.ep163, parentId: null, kind: "comment",
    body: { text: "Dylan, the fixture needs your restart case.", author: "H" }, anchor: null,
    mentions: [WORK_IDS.dylan], reactions: [], resolvedAt: null, resolvedBy: null,
    createdAt: at(30), createdBy: ALICE, erasedAt: null,
    ...over,
  };
}

export function makeLabel(over: Partial<Label> = {}): Label {
  return { id: WORK_IDS.label, teamId: WORK_IDS.team, name: "resume", color: "accent", ...over };
}

export function makeTeamPolicy(over: Partial<TeamPolicy> = {}): TeamPolicy {
  return {
    teamId: WORK_IDS.team, defaultState: "triage", priorityToLeaveTriage: true,
    stopAtLandedWithoutServes: false, updatedAt: null, updatedBy: null, ...over,
  };
}

let seq = 0;
export function makeWorkEvent(over: Partial<WorkEvent> = {}): WorkEvent {
  seq += 1;
  return {
    seq, at: at(0), kind: "issue.state", teamId: WORK_IDS.team, projectId: WORK_IDS.project,
    subject: WORK_IDS.ep142, actor: DYLAN, data: {}, ...over,
  };
}
```

`makeWorkEvent`'s `seq` counts up across calls in one test file, so two events built in a row are
ordered; a test that needs fixed numbers passes `seq` in the override. `label.color` is a token
name, never a hex (task 03).

#### Contract — `workFixtures.ts`, part 3: `workScenario()`, the demo's world (05 §5.3)

```ts
const LAND = "4f1c2e9a7b3d5c6e8f0a1b2c3d4e5f6a7b8c9d0e"; // 40 hex
const head = (n: number, id: string, over: Partial<IssueHead>): IssueHead =>
  makeIssueHead({ id, number: n, alias: `EP-${n}`, headRevision: 1, headKey: issueKey(id, 1), ...over });

export function workScenario() {
  const criteria = [makeCriterion(), makeCriterion({ id: WORK_IDS.criterionUpload, text: "upload works for PDF, XLSX and images" })];
  const issues: IssueHead[] = [
    makeIssueHead({ state: "done", proof: "proven", landCommit: LAND, updatedAt: at(302) }), // EP-142, 14:02
    head(150, WORK_IDS.ep150, { state: "in_review", relations: [{ kind: "blocks", issueId: WORK_IDS.ep142 }] }),
    head(160, WORK_IDS.ep160, {
      state: "triage", priority: "none", assignee: null, source: "intake",
      threadState: { kind: "not_tied", reason: "imported" },
      external: { source: "csv", externalId: "row-160", url: "", authorName: "Import" },
    }),
    head(161, WORK_IDS.ep161, { state: "done", proof: "not_proven", stateReason: "Checked by hand on staging; the CI job was down." }),
    head(162, WORK_IDS.ep162, { state: "done", proof: "proven_self_attested", landCommit: LAND }),
    head(163, WORK_IDS.ep163, { state: "started", assignee: ALICE, createdBy: ALICE }),
    head(164, WORK_IDS.ep164, { state: "landed", proof: "waiting", landCommit: LAND }),
    head(165, WORK_IDS.ep165, { state: "done", proof: "no_criteria", threadState: { kind: "not_tied", reason: "chore" } }),
    head(166, WORK_IDS.ep166, { state: "backlog", priority: "low" }),
    head(167, WORK_IDS.ep167, { state: "todo" }),
    head(168, WORK_IDS.ep168, { state: "cancelled", stateReason: "Superseded by the upload rework." }),
    head(169, WORK_IDS.ep169, { state: "duplicate", relations: [{ kind: "duplicates", issueId: WORK_IDS.ep142 }] }),
  ];
  const revisions = {
    ep142: makeIssueRevision(), // revision 3, serving c-91e2a4b0
    ep164: makeIssueRevision({ issueId: WORK_IDS.ep164, revision: 1,
      content: { title: { text: "Upload evidence files", author: "H" }, description: { text: "PDF, XLSX and images.", author: "H" },
        serves: [makeServe({ criterionId: WORK_IDS.criterionUpload })], implements: [] } }),
    ep163Proposal: makeIssueRevision({ issueId: WORK_IDS.ep163, revision: 2, writtenBy: ALICE,
      content: { title: { text: "Split the restart fixture", author: "A" }, description: { text: "Drafted by an agent.", author: "A" },
        serves: [], implements: [] } }),
  };
  const comments = [makeComment({ reactions: [{ emoji: "\u{1F44D}", users: [DYLAN] }] })];
  const events: WorkEvent[] = [
    makeWorkEvent({ seq: 1, at: at(280), kind: "issue.state", subject: WORK_IDS.ep142, data: { to: "landed", cause: "land_trailer" } }),
    makeWorkEvent({ seq: 2, at: at(302), kind: "issue.state", subject: WORK_IDS.ep142, data: { to: "done", cause: "evidence_observed" } }),
    makeWorkEvent({ seq: 3, at: at(30), kind: "comment.created", subject: WORK_IDS.ep163, actor: ALICE, data: { commentId: WORK_IDS.comment } }),
    makeWorkEvent({ seq: 4, at: at(30), kind: "comment.mentioned", subject: WORK_IDS.dylan, actor: ALICE, data: { commentId: WORK_IDS.comment, issueId: WORK_IDS.ep163 } }),
  ];
  return {
    projectKey: "EP", briefKey: BRIEF_KEY, criteria, issues, revisions, comments, events,
    labels: [makeLabel()], policy: makeTeamPolicy(), users: [DYLAN, ALICE],
  };
}
```

**What the world holds, in words** (so a surface task can name its fixture without reading code):
EP-142 serves *"the resume test passes"* at revision 3, landed at 13:40 and done with **proven**
at 14:02; EP-150 **blocks** EP-142 and is in review, not landed; EP-160 is an imported `triage`
item reading *not tied · imported*; EP-161 is a hand-set `done`, **not proven**, with its reason;
EP-162 is **proven, self-attested**; EP-163 is Alice's, started, with her comment mentioning
Dylan and an agent's unaccepted revision 2 (`text·A`, for task 19's refusal and the accept
surface); EP-164 is landed and **waiting** on *"upload works for PDF, XLSX and images"*; EP-165 is
a chore done with **no criteria**; EP-166 to EP-169 are `backlog`, `todo`, `cancelled` and
`duplicate`. The `data` payloads follow task 13's scalar-only rule; their exact keys are task 13's
data structs and task 37 takes them from there (**UNVERIFIED** here beyond the two kinds shown).
Times are ISO instants in UTC (`FIXED_NOW` is 09:00Z); a surface that prints *14:02* formats in
the time zone its test pins, which is that surface task's choice, not this kit's.

#### Tests to write first

`expectNoPrediction.test.ts` renders each case into a fresh `div` with `innerHTML` (no React: the
check is about the DOM, whoever built it).

| Test | Input | Asserts | Mutant it kills |
|---|---|---|---|
| `throws on a percentage in text` | `<p>40%</p>` | throws; the message names `percentage`, `"40%"` and `div > p` | a missing rule; a message with no path |
| `throws on a percentage in an attribute` | `<span aria-label="60% done"></span>`, and `title="12.5 %"` | each throws, naming the attribute | a text-only check |
| `throws on a bar by role` | `<div role="progressbar"></div>`, `<div role="meter"></div>` | each throws `progress-role` | a check that looks only at text |
| `throws on a bar by element` | `<progress></progress>`, `<meter></meter>` | each throws `progress-element` | the native elements forgotten |
| `throws on aria-valuenow` | `<div aria-valuenow="3"></div>` | throws `valuenow` | a custom slider-shaped bar |
| `throws on N of M` | `3 of 5 criteria`; `<b>3</b> of 5`; `4/5 proven` | each throws `n-of-m`; the split one once at its `p` | a per-text-node check that misses a split phrase |
| `throws on an ETA` | `ETA 14:22`; `about 5 min left`; `2 hours remaining`; `done by Friday`; `estimated 16:00` | each throws `eta` | each missing rule |
| `passes the facts` | `EP-142`; `14:02`; `4 issues`; `waiting 2h 10m`; `c-91e2 still open (EP-150 not landed)` | no throw | an over-broad rule that would ban facts, and so get switched off |
| `verbatim is skipped whole` | `<pre data-verbatim>coverage 50% · 3 of 5 · ETA 2m</pre>` | no throw | a verbatim exemption that checks anyway |
| `a verbatim span does not leak into its sentence` | `<p>Log says <code data-verbatim>50%</code> today</p>` | no throw | the phrasing join dragging verbatim text in |
| `a projection may say when` | `<span data-projection-rule>estimated ~14:22 · rule: migration gate</span>` | no throw; the same text **without** the attribute throws `eta` | an exemption that does nothing, or one that is always on |
| `a projection may not say how far` | `<span data-projection-rule>40% · rule: migration gate</span>` | throws `percentage` | an exemption that disables every rule |
| `findPredictions returns, never throws` | `<p>40%</p>` | one finding, rule `percentage` | two code paths that disagree |

`workFixtures.test.ts`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `every id matches the Hub's grammar` | each `WORK_IDS` value matches `^[a-z]{2,3}-[0-9a-f]{16}$` for Hub ids, `^c-[0-9a-f]{8}$` for criteria, `^u-[0-9a-f]{16}$` for users; `BRIEF_KEY` and each `headKey` match the Hub's `brief:` and `issue:` grammars | a fixture id the real Hub would refuse |
| `every state and proof word is in the world` | the set of `workScenario().issues[].state` equals `WORK_STATES`; the set of non-empty `proof` values equals `PROOF_WORDS` | a fixture set that hides a glyph or a state from surface tests |
| `the world tells the demo's story` | EP-142 is `done`, `proven`, served criterion `c-91e2a4b0`, `updatedAt` 14:02Z; EP-150 `blocks` EP-142 and is not `landed`; EP-160 is `not_tied` / `imported`; EP-161's `stateReason` is non-empty | a scenario that drifts from 05 §5.3 |
| `no fixture string is a prediction` | every string reachable in `workScenario()`, rendered into one `<p>` each, passes `expectNoPrediction` | a fixture that would force exemptions on every surface |
| `builders are single` | a source scan of `desktop/src/**/*.test.ts(x)` finds no `function make(IssueHead|IssueRevision|Serve|Criterion|Comment|Label|TeamPolicy|WorkEvent)` outside this file | a local near-duplicate builder (`fixtures.ts:6-8`'s rule) |

#### Steps

- [ ] **Step 1: write `expectNoPrediction.test.ts`.** `npm --prefix desktop test -- src/test/expectNoPrediction.test.ts`
  fails: the module does not exist.
- [ ] **Step 2: write `expectNoPrediction.ts`** as above. Re-run; every row passes.
- [ ] **Step 3: prove two rows bite.** Delete the `data-projection-rule` waiver line; the
  `a projection may say when` row fails. Delete the `PHRASING` join; the split `N of M` row fails.
  Restore both.
- [ ] **Step 4: write `workFixtures.test.ts`, then `workFixtures.ts`.** Run
  `npm --prefix desktop test -- src/test/`; every test passes, the kit's existing ones included.
- [ ] **Step 5: typecheck.** `npm --prefix desktop run typecheck` exits 0.
- [ ] **Step 6: commit** the four files by path.

#### Acceptance

```bash
npm --prefix desktop test -- src/test/
# expect: every file under src/test passes; the two new files list 13 and 5 tests
npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc
```

#### What this task does NOT claim

- That a bar drawn by CSS widths alone is caught: a `div` whose `width` grows is invisible to a DOM
  text scan. The owner's review of the prototype (task 39) covers the look.
- That every surface calls the check: each surface task's own test does.
- That the local mirror types are right: task 37's typecheck is what proves them against
  `src/api/work.ts`.

Parity rows served: none closed alone. It enforces `OD-27` on every surface task that closes one.

#### Gaps found while writing this part

1. **Row 20 has no dependency, but its types are task 03's and task 37's.** As the scopes' gap 6
   says, this part mirrors task 03's JSON names by hand (`task-03.md:279-398`) and task 37 swaps
   in the real types.
2. **Event `data` keys** are task 13's per-kind structs; the fixtures use two kinds' keys only.

