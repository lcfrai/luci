# Owner decision, 2026-09-28: terms and decisions sit BESIDE the brief, not inside it

**Status: decided by the owner. To be folded into
`docs/superpowers/planning/2026-09-23-owner-decisions.md` as OD-36** once the register edit now in
flight has landed (the `od-register` chain of run `wf_a4e50f19-3d1` is editing that file, so this
was written here instead of fighting it).

## What was decided

Discover gains a **project glossary** and a **project decision record**, and both are **their own
Hub records that reference a brief revision**, rather than fields inside the brief's content.

```
term:tm-7f2a@2  ->  brief:bf-9c1d@4
  word:       lane
  definition: a branch an agent works in, fetched back under refs/panacea/lanes/...
  settled:    revision 4, by owner
  supersedes: term:tm-7f2a@1
```

The Materialiser renders both into the per-branch `CLAUDE.md`, so an agent inherits the project's
vocabulary and its decisions along with the brief it is working to.

## The gap this closes, verified twice

Discover carries **intent** and no **language**. The brief's answers and criteria are captured,
versioned and provable; the project's own word for a thing is not captured anywhere. Every `ADR`
in the phase 4 plan is `ADR-H1`, the Hub's own architecture record, not a per-project artifact.
So today the thread knows what was asked for and never learns what the team means by *lane*,
*rail*, *machine* or *seam*.

## Why beside, and NOT inside

The owner's note when deciding was *"we are still in build, so if you need to redo some work, it is
what it is"*, which removes rework cost as the deciding factor. Beside is still correct, on three
grounds that have nothing to do with cost:

1. **Lifecycle.** Vocabulary changes at a different rate from requirements. If terms were fields of
   brief content, defining a term would mint a new brief revision, and *revision 31* would stop
   meaning anything. That is the same defect the sceptic found in the interview design, which put
   `Grill{Rounds}` inside immutable content: process state and content state look alike right up
   until one of them is made immutable.
2. **Scope.** *Lane* is a project word, not a brief word. Ten briefs in one project share it. Inside
   brief content there would be ten copies free to disagree, which is exactly the cross-scope
   contradiction `06-sceptic.md` found between a branch dissenting from a project decision and
   `Apply` only ever seeing one content object.
3. **Supersede.** A definition often improves while the requirement does not change. Beside the
   brief that is expressible (`term:tm-7f2a@2 supersedes @1`); inside it, it is not.

## What it does NOT change, and this is the point

- The brief content model, which is **built** (phase 4 tasks 03 to 06 and 09: about 10,940 lines,
  98+ test functions, 4 goldens).
- The provenance key grammar, which the **Hub contract** owns, two parsers compile, and 64 plan
  files across six worktrees quote. No re-anchoring.

## What it DOES change, and the owner has authorised the rework

Where the correct design needs built phase 4 code to move, it moves:
- the **Materialiser** (task 09) renders terms and decisions into the per-branch `CLAUDE.md`;
- **`Ref`** (task 06) carries term and decision digests so a session's inheritance is provable;
- the **Discover lens** (tasks 18 to 21) shows a term as it settles and a decision as it is taken.

## Rejected with it

The large in-content design in `05-design.md` is **not adopted**. `06-sceptic.md` returned
**Needs fixes** on eight grounds, four structural: it priced built code as paper; it edited a
grammar owned by a contract it never opened; it put three mandatory checkboxes and a disabled Save
in front of a user `AGENTIC-UX.md` calls intermittently present, which is the wall-of-configuration
bar `CLAUDE.md` says we fail; and past 40 terms it handed an agent the word without the definition
while still stamping the session with the key, so the provenance claim would have certified a
truncation.

**A design round 2 is required before any of this is built**, and it must carry the sceptic's
findings as inputs. It is queued behind a workflow slot; the owner's standing limit is three
concurrent workflows.

## Provenance of this decision

Source: `https://www.aihero.dev/skills-grill-with-docs` and the catalogue at
`https://www.aihero.dev/skills`, studied on 2026-09-28. The upstream repository
(`mattpocock/skills`) states an MIT licence; its LICENSE file text was not read, and must be read
before any of its text is used. Nothing of theirs has been copied into this repository: what was
taken is the method, and specifically the three-part test for what earns a decision record, which
is adopted separately for our own owner-decision register.
