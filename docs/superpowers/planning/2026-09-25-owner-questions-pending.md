# Owner questions, pending (collected 2026-09-25)

None of these blocks a build: each stream built its recommended default. They are waiting for the
owner's next question round. Answers go into `2026-09-23-owner-decisions.md` as OD rows.

Each question carries a **Holds up** line: what stays open, unwritten or unverified until it is
answered. It is the same idea as the *unblocks* column on the decision register, for questions that
have no decision yet, so the next round can be ordered by what is waiting on it rather than by which
stream asked first. "Holds up: nothing" is a real and common answer here — it means the default that
was built stands, and an answer would only confirm or replace it.

## From the Hub (phase H), run `wf_b4bd0092-bcc`

Several of these were written against direct Entra. OD-30's ruling
(`2026-09-25-identity-keycloak-broker.md`) may already answer them; check it before asking.

5. **Step-up sign-in for one non-Hub scope** (Azure DevOps via Entra's claims challenge) is not
   built in v1. Recommended: decide after task 43's R-8 evidence. Under OD-30, ADO is a personal
   access token, which may make this moot.
   - *Holds up:* nothing built. Only a later "IT granted ADO consent" path, which needs task 43's
     R-8 evidence first. Under OD-30 the PAT stands.
6. **Task 40 against phase 2's SX.2 gate.** `types.ts` must be byte-identical to UI contract §2.
   Recommended: amend §2 in task 40's own commit, rather than moving the types to their own file.
   - *Holds up:* the shape of task 40's own commit, and nothing else. Either answer keeps one copy
     of the types; the question is which file the copy lives in, and the gate checks it either way.
7. **Q12a: accept two edits to ADR-H1 (your text)?**
   - §5.3's invite row: an invite binds by the email claim only for a member, or for a guest whose
     domain is verified.
   - §9 item 3: IT configures the optional claims email, acct and xms_edov.
   - Recommended: yes, but OD-30 moves the claims to Keycloak mappers.
   - *Holds up:* ADR-H1's final text, and the realm-configuration ask that goes to Panacea's own
     update under OD-13 — the mappers are in Panacea's realm until phase 8 runs its own Keycloak.
8. **Q12b: someone who cannot bind (a B2B guest with no verified domain, or no email) has no
   in-product way into a team in v1.** Is that acceptable for the single KPMG tenant? Recommended:
   yes, with a later admin list of the org's users.
   - *Holds up:* whether the Hub's invite surface needs a second path at all. A "no" adds an admin
     user list to a later slice; nothing in flight waits on it.
9. **Q13: phase H's two UI surfaces have no prototype** (the sign-in states in the rail footer, and
   the share popover). Recommended: write `prototype/phase-h-hub.html` before task 41's component
   code, and hold `feat/hub`'s onward merge until you have looked (OD-12 batching).
   - *Holds up:* task 41's component code, and `feat/hub`'s onward merge. This is the one Hub
     question with work behind it — it rides OD-12's batched hands-on session.

The Hub's sibling asks of phase 8 (`Member` and `EmailVerified` claims, and `TRUSTED_HOPS`) are
routed to the orchestrator; they are not questions for you.

## From phase 7 (the deploy connector), run `wf_63db912c-42f`

1. **Q-P7-7: which `pnc_` key does the connect panel tell users to mint?** Phase 7 recommends, and
   has built, an **unrestricted** key behind the `KEY_ADVICE` constant.
   - The cost: the key carries every right its holder has, including platform admin for an
     administrator, on every project.
   - The alternative is a key restricted to one project at Deploy, which means one project per
     cockpit and no owner controls.
   - One key per project is not built.
   - OD-30 may change this: phase 7's ruling section says whether the realm token from the new
     sign-in can replace the pasted key.
   - *Holds up:* the connect panel's final copy (`KEY_ADVICE`) and the owner-controls story at
     Deploy. **The largest of the nine**, because a restricted key would also change phase 7's
     one-project-per-cockpit shape. OD-30 may dissolve it before the owner has to answer.
2. **O-2: a Panacea defect. Raise it with the Panacea team?**
   - `projectAccessGroups` treats `project-<name>-` as a prefix (`clients/keycloak.go:483-490`
     at `b02424691`). So for a project called `app` it also picks up `app-two`'s groups.
   - The effects: `app`'s member list shows `app-two`'s members; a role change on `app` drops the
     person's `app-two` roles; a revoke on `app` removes them from `app-two` too.
   - Phase 7 records it under "does not claim" (task 18).
   - It belongs in Panacea's own update (OD-13).
   - *Holds up:* nothing in this repo. It holds up a fix in Panacea, and until that lands, any
     project whose name is a prefix of another's shows the wrong members through phase 7.
3. **Q-P7-6: should Panacea's `branchEnv` resolve the production branch the same way the deploy
   does?**
   - `branchEnv` is at `handlers/branch_security.go:29-37`. The deploy's order is label, then the
     primary-branch annotation, then `main`, via `ResolveProjectProductionBranch`
     (`clients/pipeline.go:236`).
   - Until it does, phase 7 flags a release recorded against another environment (task 08
     rule 5a) and corrects nothing.
   - This is a Panacea change.
   - *Holds up:* nothing in this repo. Until it lands, phase 7 flags a release recorded against
     another environment and corrects nothing, which is the built behaviour either way.
4. **HA7-6 (for the record only): do you accept answer (b)?** The Hub keys Azure DevOps
   repositories as phase 3's `azure-devops/<org>/<project>/<repo>`. Both sibling plans already
   follow it, and phase 7 works under either answer.
   - *Holds up:* nothing. Both sibling plans already key repositories this way; an answer confirms
     the record. It is the clearest case for the register's "a real fork" limb rejecting a row.

## From the register work (2026-09-28), run in the main tree

1. **Where does an owner answer go when it fails the three-part gate?** The register now carries a
   test for what earns an `OD` (expensive to undo · not derivable from the tree · a real fork), and
   a backwards audit naming six existing rows that would not pass it. But the register is also the
   only record of what you said in a question round, and some answers are genuinely notes — OD-28
   chose two fonts, OD-7 recorded where the backup lives. Under a strict gate they have nowhere to
   live, and a second store is what the gate itself forbids.
   - (a) **Gate the register, tolerate the notes.** Every answer stays an `OD`; the audit section
     tells a reader which rows are notes. Free, slightly diluted.
   - (b) **Gate strictly.** A failing answer goes into the plan or task it governs and gets no
     number. Sharp register; an owner answer buried in a part file is not where anyone looks.
   - (c) **One register, two row kinds.** A `note` marker on the row itself, visible where the row
     is, not only in an audit at the end. One pass over six rows, plus a convention each round.
   - No recommendation: this is the owner's call about their own record. Detail in
     `2026-09-28-discovery-skills/07-register-report.md` §5.
   - *Holds up:* nothing built. It holds up the next question round's write-up convention only.
