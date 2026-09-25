# Owner questions, pending (collected 2026-09-25)

None of these blocks a build: each stream built its recommended default. They are waiting for the
owner's next question round. Answers go into `2026-09-23-owner-decisions.md` as OD rows.

## From the Hub (phase H), run `wf_b4bd0092-bcc`

Several of these were written against direct Entra. OD-30's ruling
(`2026-09-25-identity-keycloak-broker.md`) may already answer them; check it before asking.

5. **Step-up sign-in for one non-Hub scope** (Azure DevOps via Entra's claims challenge) is not
   built in v1. Recommended: decide after task 43's R-8 evidence. Under OD-30, ADO is a personal
   access token, which may make this moot.
6. **Task 40 against phase 2's SX.2 gate.** `types.ts` must be byte-identical to UI contract §2.
   Recommended: amend §2 in task 40's own commit, rather than moving the types to their own file.
7. **Q12a: accept two edits to ADR-H1 (your text)?**
   - §5.3's invite row: an invite binds by the email claim only for a member, or for a guest whose
     domain is verified.
   - §9 item 3: IT configures the optional claims email, acct and xms_edov.
   - Recommended: yes, but OD-30 moves the claims to Keycloak mappers.
8. **Q12b: someone who cannot bind (a B2B guest with no verified domain, or no email) has no
   in-product way into a team in v1.** Is that acceptable for the single KPMG tenant? Recommended:
   yes, with a later admin list of the org's users.
9. **Q13: phase H's two UI surfaces have no prototype** (the sign-in states in the rail footer, and
   the share popover). Recommended: write `prototype/phase-h-hub.html` before task 41's component
   code, and hold `feat/hub`'s onward merge until you have looked (OD-12 batching).

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
2. **O-2: a Panacea defect. Raise it with the Panacea team?**
   - `projectAccessGroups` treats `project-<name>-` as a prefix (`clients/keycloak.go:483-490`
     at `b02424691`). So for a project called `app` it also picks up `app-two`'s groups.
   - The effects: `app`'s member list shows `app-two`'s members; a role change on `app` drops the
     person's `app-two` roles; a revoke on `app` removes them from `app-two` too.
   - Phase 7 records it under "does not claim" (task 18).
   - It belongs in Panacea's own update (OD-13).
3. **Q-P7-6: should Panacea's `branchEnv` resolve the production branch the same way the deploy
   does?**
   - `branchEnv` is at `handlers/branch_security.go:29-37`. The deploy's order is label, then the
     primary-branch annotation, then `main`, via `ResolveProjectProductionBranch`
     (`clients/pipeline.go:236`).
   - Until it does, phase 7 flags a release recorded against another environment (task 08
     rule 5a) and corrects nothing.
   - This is a Panacea change.
4. **HA7-6 (for the record only): do you accept answer (b)?** The Hub keys Azure DevOps
   repositories as phase 3's `azure-devops/<org>/<project>/<repo>`. Both sibling plans already
   follow it, and phase 7 works under either answer.
