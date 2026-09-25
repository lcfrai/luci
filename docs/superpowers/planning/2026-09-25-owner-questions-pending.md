# Owner questions, pending (collected 2026-09-25)

None of these blocks a build: each stream built its recommended default. They are waiting for the
owner's next question round. Answers go into `2026-09-23-owner-decisions.md` as OD rows.

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
