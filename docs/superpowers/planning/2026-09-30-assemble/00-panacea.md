# 00 - What Panacea already provides (inventory for PanaceaApp)

**Status:** IN PROGRESS (skeleton written first; sections fill in one at a time).
**Date:** 2026-09-30. **Cluster:** 00-panacea. **Feeds:** every other cluster (01-06) and the plan.
**Scope:** a read-only inventory of every Panacea capability PanaceaApp can call today instead of
building or buying it: the entry point (panacea-api route with file:line, or panacea-mcp tool),
what it does, how a caller from another product authenticates, and its gaps for our use. Then the
candidates side by side (Panacea, Coolify + Supabase with Panacea snippets, others) and ONE
recommendation.

**Direction this was written under (2026-09-30, owner):** "focus on using third party solutions ...
I need this tool ASAP, not in weeks"; Panacea is the strong default and the best fully-fledged
solution wins ("Panacea is fine to use majority of, if we want to use Coolify plus Supabase, and
enhance with snippets from Panacea, that is fine as well"). OD-13 relaxed: Panacea snippets may be
copied with attribution in the file header. Orca is the cockpit pick (not re-argued here).

**Method:** grep only, scoped per directory, in `Projects/Panacea` (read-only; nothing built, run
or written there). Every claim below cites `path:line` in that repo as of today's working tree.

---

## 1. Answer first

_(pending)_

## 2. The inventory at a glance

_(pending)_

## 3. Deploy, per-branch previews, promote, rollback, versions and branches

_(pending)_

## 4. panacea-mcp: the tools

_(pending)_

## 5. Identity: the Keycloak realm, Entra brokering, and how another product calls in

_(pending)_

## 6. AI: the LiteLLM gateway and its token model

_(pending)_

## 7. Platform services: Backstage, Vault, Postgres, object storage

_(pending)_

## 8. Builds and security: Tekton, Trivy, Falco and the other scanners

_(pending)_

## 9. Isolation: tenant namespaces, and Forge (agent loop and hardened preview pods)

_(pending)_

## 10. Candidates side by side

_(pending)_

## 11. Recommendation

_(pending)_

## 12. The glue we still build

_(pending)_

## 13. Owner calls

_(pending)_

## 14. Revised for the owner's second direction

_(pending)_

## 15. What was not verified

_(pending)_

## 16. Sources

_(pending)_
