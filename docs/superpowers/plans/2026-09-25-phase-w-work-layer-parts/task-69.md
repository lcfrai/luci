### Task 69: the import dialog; `first.landing`'s rehearsal as the real issue EP-1

Stream **WI**, about 3 hours (an estimate, not a measurement). Depends on task 25 (the control
plane's importer: `POST /v1/work/import/preview` and `POST /v1/work/import`, one body, the
`Outcome` of `created`, `unchanged`, `refused` and `unmatchedLabels`, `task-25.md:99-110`,
`:189-196`, routes at `:433-434`) and task 42 (Triage, and the *not tied* reasons task 15
wrote). Uses task 56's `mutate` for the rehearsal's create and task 46's issue view for EP-1.

After this task a person with work in Linear or a CSV can put it in Triage in one dialog, and a
person with nothing at all can make their first real issue and land it themselves.

**What the reader sees** (05 §5.3's demo, first frame; §4.4's C11 row; 02 C11):

- **Import.** A dialog with two sources. *Linear* asks for a personal API key in a password
  field and an optional team key; *CSV* takes a file. Then a target project, then *Preview*,
  which says *"44 ready · 2 refused"* with the two refusals named by line, and then *Import*.
- While it runs, one line: *"imported 400 so far"*, and nothing else — no total, no *N of M*,
  no bar, no estimate (OD-27).
- It ends with the plain counts: *"142 created · 3 already imported"*, the refusals listed, the
  labels that matched nothing, and one verb, *Open Triage* (`G T`).
- Every imported item is in `triage` and reads *not tied · imported*.
- **The rehearsal.** In a project with no issues, the Work lens's zero state offers one verb,
  *Rehearse on EP-1*. Pressing it creates a real issue, EP-1, whose description names the
  rehearsal task in one line. Nothing starts: the person's own `Shift+R` starts it, and their
  first Desk item is a real LANDED row with a real span and a real hunk.
- **On this tree the verb is disabled**, and its label is the reason: *Rehearse on EP-1 — the
  starter repo is not built yet*. It is never a fake agent (AX Principle 7).

**Decisions made here, so nobody re-derives them:**

1. **The key lives in one `useState` and nowhere else.** It is never in the zustand store, never
   in `localStorage`, never in a log, never in a URL, and never in a React key or a `data-`
   attribute. `ImportDialog` holds it, passes it to one `fetch`, and `onClose` sets it to `""`
   before it unmounts. Test 3 spies on `localStorage.setItem`, the store's `setState` and the
   log, and test 4 reopens the dialog and reads the field. This is the whole reason the source
   step is a component and not a store slice.
2. **The count on screen is a count of what happened, never of what will.** The dialog shows
   `created + unchanged` so far and the word *so far*; it never divides, never renders a
   `progressbar` role, and never puts the word *of* beside a number. A CSV's row count is
   known before the post and is still not shown as a denominator: knowing the total is not a
   licence to predict the end (OD-27, and 02's rule that a number on screen is an observation).
3. **The counts in the result come from the response, never from the file.** *"142 created · 3
   already imported"* is `outcome.created` and `outcome.unchanged`, which the Hub returned by
   `(source, externalId)` (SW.6's idempotency). A CSV of 145 rows that imports 142 and finds 3
   is the same sentence as one of 145 rows where 3 were refused, and only the response can tell
   them apart, which is why test 1 asserts the source of each number and not its value.
4. **Preview reads nothing from the Hub.** Task 25's `/preview` does no Hub write and no Hub
   read of existing pairs (`task-25.md:109`), so *ready* is a count of parseable items and the
   dialog says *ready*, not *new*. Calling them *new* would be a prediction that they are not
   already imported, which only the import can know.
5. **The running count needs a channel task 25 does not have.** `POST /v1/work/import` answers
   once, at the end (`task-25.md:99-106`): there is no progress route and no stream, so a
   desktop that only posts can show no *"imported 400 so far"* at all. This part asks task 25
   for the smallest addition that carries it — **the same route, streaming
   `application/x-ndjson` when the request carries `Accept: application/x-ndjson`, one line per
   chunk, `{"created":N,"unchanged":M}`, and the `Outcome` as the last line** — and, until that
   exists, the dialog shows *"importing…"* with the elapsed time and no count. Elapsed time is
   an observation; a count that is not there is not invented (gap 1, **UNVERIFIED**).
6. **Imported items land in `triage` as *not tied · imported*, and the desktop does not put them
   there.** SW.6's batch writes the state and `notTiedReason: "imported"` (task 15's value);
   the dialog asserts what it sees in Triage after the import and sets nothing itself. A team
   whose policy sends intake elsewhere is the Hub's business.
7. **The rehearsal creates a real issue, and only that.** *Rehearse on EP-1* is one
   `createIssue` through task 68's op: a real Hub write, in the person's own project, with a
   one-line description that names the rehearsal task. It starts no session, opens no terminal
   and writes no fake event. The first `Shift+R` is the person's (task 61).
8. **The verb's disabled label is the fact.** The starter repo the rehearsal needs is not in
   this tree (searched `docs/`, `fixtures/` and the repo root, 2026-09-27; task 70 lists it as
   not claimed, `task-70.md:595`). So the button renders `disabled` with the label *Rehearse on
   EP-1 — the starter repo is not built yet*, and `REHEARSAL_REPO` is the one constant that
   turns it on: set it, and the same component enables the same verb. No mock repository, no
   scripted agent, no recorded transcript (AX Principle 7).

**Files:**
- `desktop/src/components/work/ImportDialog.tsx` (+ `ImportDialog.test.tsx`) — the dialog (new)
- `desktop/src/api/work.ts` ⚑ (task 37) — `importPreview`, `importBatch` and their types
- `desktop/src/work/firstRun/rehearsal.ts` (+ `rehearsal.test.ts`) — `shouldOfferRehearsal`,
  `rehearsalIssue`, `REHEARSAL_REPO` (new)
- `desktop/src/components/work/WorkZero.tsx` ⚑ (task 40's zero state) — the one verb
- `desktop/src/components/work/import.css` — the dialog's rules (new)

#### Interfaces — complete: the wire (`desktop/src/api/work.ts` ⚑), mirroring task 25

```ts
// desktop/src/api/work.ts ⚑ — task 25's body and answer, field for field (task-25.md:99-110).
export interface ImportRequest {
  projectId: string;
  source: "linear" | "csv";
  linear?: { apiKey: string; teamKey?: string };
  csv?: { content: string };
}
export interface ImportRefusal { at: string; reason: string }   // "line 17" | "ENG-212"
export interface ImportOutcome {
  created: number;
  unchanged: number;
  refused: readonly ImportRefusal[];        // [] when none
  unmatchedLabels: readonly string[];       // [] when none
}
/** /preview answers the same shape with `ready` set and the three counts null. */
export interface ImportPreview extends Partial<ImportOutcome> {
  ready: number;
  refused: readonly ImportRefusal[];
}

export async function importPreview(req: ImportRequest): Promise<ImportPreview>;
export async function importBatch(
  req: ImportRequest,
  onProgress?: (seen: { created: number; unchanged: number }) => void,
): Promise<ImportOutcome>;
```

`onProgress` is called once per ndjson line when the control plane streams (decision 5) and
never otherwise; a caller that passes it and gets no call shows no count, which is exactly what
happens on today's control plane. Neither function logs its argument: `importBatch`'s error path
re-throws `classify(e)` and the request body never reaches a log line, because the key is in it.

#### Interfaces — complete: `ImportDialog.tsx`, part 1: the state and the key

```tsx
// desktop/src/components/work/ImportDialog.tsx
type Step = "source" | "previewing" | "ready" | "running" | "done";

export function ImportDialog({ onClose }: { onClose(): void }) {
  const [source, setSource] = useState<"linear" | "csv">("linear");
  const [apiKey, setApiKey] = useState("");          // decision 1: here and nowhere else
  const [teamKey, setTeamKey] = useState("");
  const [csv, setCsv] = useState("");
  const [projectId, setProjectId] = useState(useCockpit.getState().selection.projectId ?? "");
  const [step, setStep] = useState<Step>("source");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [outcome, setOutcome] = useState<ImportOutcome | null>(null);
  const [seen, setSeen] = useState<{ created: number; unchanged: number } | null>(null);
  const [failed, setFailed] = useState<string | null>(null);

  const close = () => { setApiKey(""); setCsv(""); onClose(); };   // test 4

  const req = (): ImportRequest => ({
    projectId, source,
    ...(source === "linear" ? { linear: { apiKey, ...(teamKey ? { teamKey } : {}) } }
                            : { csv: { content: csv } }),
  });

  const run = async () => {
    setStep("running"); setSeen(null); setFailed(null);
    try {
      const got = await importBatch(req(), setSeen);        // decision 5
      setOutcome(got); setStep("done");
    } catch (e) { setFailed(classify(e).detail); setStep("ready"); }
    finally { setApiKey(""); }                              // the key dies with the run
  };
```

`setApiKey("")` in `finally` is the second of the two places the key is cleared, and it is the
one that matters: a run that throws, a window that is closed by the OS, and a successful import
all end with the field empty. The key is passed to `importBatch` by value inside `req()`, which
builds a fresh object per call and keeps no reference of its own.

#### Interfaces — complete: `ImportDialog.tsx`, part 2: what it renders

```tsx
  return (
    <div className="im" role="dialog" aria-modal="true" aria-label="Import issues">
      {step === "source" && (
        <>
          <fieldset>
            <label><input type="radio" checked={source === "linear"}
              onChange={() => setSource("linear")} /> Linear</label>
            <label><input type="radio" checked={source === "csv"}
              onChange={() => setSource("csv")} /> CSV</label>
          </fieldset>
          {source === "linear" ? (
            <>
              <label>Personal API key
                <input type="password" autoComplete="off" spellCheck={false}
                  value={apiKey} onChange={(e) => setApiKey(e.target.value)} /></label>
              <p className="im-note">The key is used for this import and is not stored.</p>
              <label>Team key (optional)
                <input value={teamKey} onChange={(e) => setTeamKey(e.target.value)} /></label>
            </>
          ) : (
            <input type="file" accept=".csv,text/csv"
              onChange={async (e) => setCsv(await (e.target.files?.[0]?.text() ?? ""))} />
          )}
          <ProjectPicker value={projectId} onChange={setProjectId} />
          <button type="button" disabled={!projectId} onClick={doPreview}>Preview</button>
        </>
      )}

      {step === "ready" && preview && (
        <>
          <p className="im-ready">{preview.ready} ready{preview.refused.length > 0
            ? ` · ${preview.refused.length} refused` : ""}</p>
          <RefusalList items={preview.refused} />
          {failed && <p className="im-fail">{failed}</p>}
          <button type="button" onClick={run}>Import</button>
        </>
      )}
```

```tsx
      {step === "running" && (
        <p className="im-running" aria-live="polite">
          {seen ? `imported ${seen.created + seen.unchanged} so far` : "importing…"}
          {" · "}<span className="tnum">{formatElapsed(startedAt, beat)}</span>
        </p>
      )}

      {step === "done" && outcome && (
        <>
          <p className="im-done">
            {outcome.created} created · {outcome.unchanged} already imported
          </p>
          <RefusalList items={outcome.refused} />
          {outcome.unmatchedLabels.length > 0 && (
            <p className="im-labels">matched no label: {outcome.unmatchedLabels.join(", ")}</p>
          )}
          <button type="button" onClick={() => { close(); goTo("triage"); }}>
            Open Triage <Kbd chord="G T" />
          </button>
        </>
      )}
    </div>
  );
}
```

Three rules hold across every branch above, and each has a test: no element carries
`role="progressbar"`; the word *of* never appears beside a number; and the only elapsed value is
`formatElapsed` on the one heartbeat, in `tabular-nums`. The `aria-live="polite"` region
announces the running count as it changes, which is the one place a screen reader hears
progress — and it hears the same observation the screen shows, never a projection.

#### Interfaces — complete: `desktop/src/work/firstRun/rehearsal.ts`

```ts
// desktop/src/work/firstRun/rehearsal.ts — pure; no React, no fetch.
import type { CreateIssueRequest } from "../../api/work";

/** Decision 8. Null until the starter repo exists; setting it enables the verb, nothing else. */
export const REHEARSAL_REPO: { url: string; ref: string } | null = null;

export const REHEARSAL_DISABLED_LABEL =
  "Rehearse on EP-1 — the starter repo is not built yet";
export const REHEARSAL_LABEL = "Rehearse on EP-1";

/** 02 C11 as amended by 05 §4.4: offered only in a project with no issues at all. */
export function shouldOfferRehearsal(issueCount: number, projectId: string | null): boolean {
  return projectId !== null && issueCount === 0;
}

/** The one real issue. No fixture, no seeded events: a create like any other (decision 7). */
export function rehearsalIssue(projectId: string): CreateIssueRequest {
  return {
    projectId,
    title: { text: "Add a line to README and land it", author: "H" },
    description: { text:
      "A rehearsal: open this issue, press Shift+R to start an agent in a worktree, let it " +
      "add one line to README.md, then land it. The Desk will then hold your first landed " +
      "row, with the span and the hunk that produced it.", author: "H" },
    state: "",                       // the team policy's default (task 68's decision 3)
    serves: [],
    notTiedReason: "chore",
  };
}
```

`author: "H"` is the person's own class (task 46's `AuthorClass`, L-124): the words are written
here, by a person, once, and the issue carries no claim that an agent wrote them. `state: ""`
means the team decides where a created issue lands, so a team that sends creates to `triage`
gets EP-1 in Triage and the zero state is answered either way.

```tsx
// desktop/src/components/work/WorkZero.tsx ⚑ (task 40) — the one verb, and nothing else new.
{shouldOfferRehearsal(heads.length, projectId) && (
  <button type="button"
    disabled={REHEARSAL_REPO === null}
    onClick={() => mutate({ kind: "createIssue", localId: newLocalId(),
                            projectId: projectId!, req: rehearsalIssue(projectId!) })}>
    {REHEARSAL_REPO === null ? REHEARSAL_DISABLED_LABEL : REHEARSAL_LABEL}
  </button>
)}
```

The disabled button is still rendered, and its label is the reason: a person who wonders where
the rehearsal went reads why it is not there. AX Principle 7's alternative — hiding it — would
leave the zero state saying nothing about the one thing 02 C11 promised.

#### Tests to write first, part 1 — `ImportDialog.test.tsx` (the dialog)

```tsx
// The fetch layer is stubbed at importPreview/importBatch, so no network and no control plane.
// 1. THE COUNTS COME FROM THE RESPONSE. A CSV of 145 rows whose externalId repeats; the stub
//    answers {created: 142, unchanged: 3, refused: [], unmatchedLabels: []} → the screen reads
//    "142 created · 3 already imported". Then the same 145-row file with the stub answering
//    {created: 145, unchanged: 0} reads "145 created · 0 already imported".
//    Mutant it kills: the row count rendered as created, which is right exactly once and wrong
//    on every re-import — the case idempotency exists for.
// 2. NO PREDICTION WHILE IT RUNS. A paged Linear import: the stub calls onProgress four times
//    (100, 200, 300, 400) before resolving. At each tick the screen holds "imported N so far",
//    there is no element with role="progressbar", and the rendered text never matches /\bof\b/
//    beside a number. expectNoPrediction(dialog) at each tick and at the end.
//    Mutant: "400 of 1000", which claims a total the control plane never sent.
// 3. THE KEY IS NOWHERE BUT THE FIELD. Spy on localStorage.setItem, useCockpit.setState and
//    the log sink. Type a key, preview, import → no spy received a string containing the key,
//    and JSON.stringify(useCockpit.getState()) does not contain it. Also assert the input's
//    type is "password" and autoComplete is "off".
//    Mutant: the key kept in workSlice "so the next import is quicker", which puts a personal
//    credential in a persisted store.
// 4. CLOSING CLEARS IT. Type a key, close, reopen → the field is empty. And a failed import
//    (the stub throws) leaves the field empty too, while the dialog stays open and shows
//    classify(e).detail. Mutant: the key kept for the next open.
// 5. PREVIEW SAYS READY, NOT NEW. The stub answers {ready: 44, refused: [2 refusals]} → the
//    screen reads "44 ready · 2 refused" and both refusals are listed with their `at` values
//    ("line 17", "ENG-212"). The word "new" appears nowhere. Mutant: "44 new", a claim the
//    preview cannot make (decision 4).
// 6. IMPORTED ITEMS READ AS IMPORTED. After the import, the Triage fixture rendered with the
//    Hub's answer shows every item in triage with "not tied · imported"; none is in backlog.
//    Mutant: the desktop setting state itself, or the reason left blank.
// 7. OPEN TRIAGE. The verb closes the dialog and navigates to Triage once, and its chord reads
//    "G T" from chordFor, not from a literal (task 54). Mutant: a hand-written chord.
```

#### Tests to write first, part 2 — `rehearsal.test.ts` and the zero state

```tsx
// 8.  shouldOfferRehearsal: 0 issues and a project → true; 1 issue → false; no project → false.
//     Mutant: offered on every visit to an empty FILTER, not an empty project, which would put
//     a rehearsal button over a filtered list of real work.
// 9.  rehearsalIssue: state is "", serves is [], notTiedReason is "chore", and both texts are
//     author "H". Mutant: state "todo" hard-coded, which ignores the team policy.
// 10. THE VERB CREATES ONE ISSUE AND STARTS NOTHING. With REHEARSAL_REPO stubbed non-null,
//     pressing it calls mutate exactly once with kind "createIssue", and api.startSession,
//     api.createSession and api.restartSession were not called.
//     Mutant: an auto-start, which is the one thing 02 C11 and AX Principle 7 both forbid —
//     the person's first agent must be the person's own Shift+R.
// 11. WITH NO STARTER REPO THE VERB IS DISABLED AND SAYS WHY. On the tree as it is
//     (REHEARSAL_REPO null), the button is disabled and its label is
//     REHEARSAL_DISABLED_LABEL; pressing it calls nothing. Mutant: enabled, which starts a
//     rehearsal that cannot finish, or hidden, which loses C11 silently.
// 12. A project with issues renders no rehearsal button at all (the pair to test 8, in the DOM).
// 13. expectNoPrediction(zeroState): the label holds no "~", no "%", no minutes.
```

#### Steps

- [ ] Add `ImportRequest`, `ImportRefusal`, `ImportOutcome`, `ImportPreview`, `importPreview`
      and `importBatch` to `api/work.ts`, mirroring task 25's JSON names field for field. Run
      the typecheck; task 37's suite passes unchanged.
- [ ] Write test 3 first, then `ImportDialog.tsx` part 1. Mutant: put the key in the store.
- [ ] Write tests 5 and 1, then the source, preview and done steps. Mutants: "44 new" (5); the
      row count as created (1).
- [ ] Write test 2, then the running step and `onProgress`. Mutants: a progressbar; a
      denominator; a percentage.
- [ ] Write tests 4, 6 and 7, then `close()`, the Triage assertion and the *Open Triage* verb.
- [ ] Write tests 8 and 9, then `rehearsal.ts`. Mutants: offer on an empty filter (8); a
      hard-coded state (9).
- [ ] Write tests 10-13, then the zero-state button. Mutants: auto-start (10); enabled with no
      repo (11); hidden (11); the button on a project with issues (12).
- [ ] Ask task 25's owner for the ndjson progress line (decision 5, gap 1). Until it exists,
      leave `onProgress` unpassed in production and keep the `"importing…"` branch.
- [ ] Tasks 25, 37, 42, 46, 56 and 68's suites pass. Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/work/ImportDialog.test.tsx src/work/firstRun'
# expected: 2 files, 13 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "apiKey" desktop/src/store/store.ts desktop/src/work/replica/replica.ts'
# expected: no output (decision 1: the key is never in the store or the replica)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "progressbar|of \\$\\{|percent|Math.round\\(" desktop/src/components/work/ImportDialog.tsx'
# expected: no output (decision 2, OD-27)
~/.claude/tools/claudezsh/claudezsh 'grep -n "startSession\|createSession" desktop/src/components/work/WorkZero.tsx desktop/src/work/firstRun/rehearsal.ts'
# expected: no output (decision 7: the rehearsal starts nothing)
~/.claude/tools/claudezsh/claudezsh 'grep -n "REHEARSAL_REPO" desktop/src/work/firstRun/rehearsal.ts'
# expected: one declaration, null on this tree (decision 8)
```

#### What this task does NOT claim

- **Jira, Azure Boards or GitHub Issues import.** W5's tasks 150-153.
- **Linear's comments, attachments, relations, cycles or sub-issues.** SW.6's batch carries
  title, description, priority, labels, author name and created time (`task-25.md:88-89`) and
  nothing else; what is not carried is not mentioned on screen either.
- **A measured import speed.** No rate, no throughput, no estimate, at any point.
- **A running count on today's control plane.** Decision 5: without the ndjson line the dialog
  says *importing…* and an elapsed time.
- **That the rehearsal can be completed.** The starter repo is not in this tree (searched
  2026-09-27), so the verb is disabled with that as its label; task 70 lists it as not claimed
  (`task-70.md:595`).
- **A cloud rehearsal.** W1 is local mode (OD-20): the agent the person starts runs on their
  machine, in a worktree.
- **That the key never reaches disk at all.** It is in the process's memory and in the request
  body; what the OS does with that memory is outside this part. The claim is narrower and
  testable: it is in no store, no `localStorage`, no log and no URL.
- **Visual acceptance:** the owner's eyes (CLAUDE.md, working agreements).
- **Parity:** closes L-220's W1 half (Linear and CSV into `triage`); serves L-46.

#### Gaps found while writing this part

1. **No progress channel.** `POST /v1/work/import` answers once (`task-25.md:99-106`), so 05
   §5.3's *"imported 400 so far"* cannot be drawn today. The smallest fix is decision 5's
   ndjson line on the same route, and it belongs to task 25. **UNVERIFIED** until then, and the
   dialog is written so the count appears the day it lands, with no other edit.
2. **The starter repo does not exist.** Searched `docs/`, `fixtures/` and the repo root on
   2026-09-27: nothing. `first.landing` needs a tiny repository with a README and a test, and
   no task in the skeleton owns building it. Either add one (an hour of work) or 02 C11 stays
   promised and undelivered.
3. **`notTiedReason: "imported"` is the batch's, not the desktop's.** Task 15 defines the
   reasons and task 25 sends the batch; whether SW.6's `ImportItem` carries the reason or the
   Hub sets it on every imported row is not printed in either part. Test 6 asserts what Triage
   shows, which is true either way, but one of the two should say it.
4. **The CSV's column set is not printed anywhere on the desktop side.** Task 25 names
   `Title, Description, Priority, Labels, AuthorName, CreatedAt` (`:88`); the dialog should
   show that header list before the file picker, and 05 §5.3 does not say so. A one-line
   addition to 05.
5. **`ProjectPicker` has no owner.** The dialog needs a project chooser; task 42's Triage takes
   its project as a prop and task 40's lens reads the selection. The name here is this part's
   and is **UNVERIFIED**.
6. **`goTo("triage")` and `newLocalId()`** are this part's names for task 41's navigation and
   task 56's local-id maker; both are **UNVERIFIED** until those parts are read side by side.
7. The skeleton's file map has no `work/firstRun/rehearsal.ts` and no `components/work/
   import.css`; the scope names the first (`tasks-61-80.md:296-297`).
8. **EP-1 is not guaranteed to be EP-1.** The alias comes from the team's next number, so in a
   project whose team has numbered issues elsewhere the rehearsal issue is not number 1. 05
   §4.4 calls it EP-1 throughout; the part's copy says *the rehearsal issue*, and the label is
   the only place the literal appears. Worth one sentence in 05.
