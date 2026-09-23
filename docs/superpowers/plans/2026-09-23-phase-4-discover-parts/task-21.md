### Task 21: Desktop — import a repository: the brief drafted from the code

Prototype state 7, and the old plan's task 13 (`:6018-6370`) reshaped: a project with code and no
project brief can have one **drafted** from a mechanical scan (task 13). Every drafted answer is
shown as drafted, lists what it was read from, and **is not inherited until a person accepts or
changes it** (`4-D2`). Then the person chooses where to go — and Discover never forces the common
case through the rare one (AGENTIC-UX §9 item 7).

**Read `docs/DESIGN-SYSTEM.md` first.** Held here: a drafted answer carries the fact sheet's
**`◌`** mark and the words *drafted by the model* — colour is never the only signal; the scan's
facts (`filesSeen`, language counts) are live-looking numbers and use `tabular-nums`, and they are
**facts, not scores** — no bar, no percentage; *Accept* and *Change* are the two controls per
answer, *Accept* never pre-focused; no spinner while the model drafts — the button reads
*Drafting* and is disabled. The owner's eyes are the visual acceptance (`OD-12`).

**Files:**
- `desktop/src/discover/ImportRepo.tsx` + `ImportRepo.test.tsx` (new)
- `desktop/src/discover/DiscoverLens.tsx` — offers *Draft a brief from the code* in state 1 on a
  project with no project brief (modify)

**Interfaces — complete source:**

```ts
// desktop/src/discover/ImportRepo.tsx
export type StartMode = "brief" | "design" | "develop";

export interface ImportRepoProps {
  projectId: ProjectId;
  /** Called once the person picks where to go. Discover does not navigate itself beyond
   *  switching the lens through phase 2's lens action; phase 5 owns what "design" opens. */
  onChoose(mode: StartMode): void;
}
export function ImportRepo(props: ImportRepoProps): JSX.Element;
```

**Flow:** *Draft a brief from the code* → `discoverApi.analyse(projectId)` → the view's step is
`review`; each pending answer shows its text, `◌ drafted by the model`, *read from:* its
`draftedFrom` names in mono, and **Accept** (op `accept`) / **Change** (op `edit`, which makes it the
person's). When the step becomes `complete` the three choices appear: *Keep refining the brief*,
*Open Design*, *Open Develop*. `draft: "model_off"` shows the response's `note` and the plain five
questions; `model_failed` says the draft could not be made and offers the same.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `drafted answers are marked and list their sources` | each shows `◌`, *drafted by the model*, and its `draftedFrom` names | a draft passed off as a person's answer (`4-D2`) |
| `accept sends accept; change sends edit` | the two ops, with the key | a change recorded as an accept, keeping the model as author |
| `the choices appear only when complete` | with one pending answer, no *Open Develop*; after the last accept, all three | starting agents on an unaccepted draft |
| `model off shows the note and the five questions` | `note` text; step 1 of 5 | an on-ramp that dead-ends without a model |
| `the scan's numbers are facts` | `filesSeen` and language counts rendered; no `%` anywhere in the component | a completeness score |
| `accept is not pre-focused` | `document.activeElement` is not an Accept button after render | accepting by pressing Enter on arrival |

**Steps:**

- [ ] **Step 1:** Open prototype state 7.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The component; the entry in `DiscoverLens`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. **Do not merge** (`OD-1`).

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
```

**Does NOT claim:** the quality of a real draft; what phase 5's *Open Design* does.
