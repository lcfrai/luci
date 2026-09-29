# 03 · Define: the brief, the interview, the spec — adopt, do not build

**Date:** 2026-09-30 · **Status:** in progress (skeleton; sections land one commit at a time)
**Question:** which existing product or project can stand in for Define (phase 4, Discover, and the
OD-36 glossary and decision record) so that we build glue, not the capability?

## 1. Answer

_Pending: filled once the candidates below are assessed._

## 2. What Define has to do (the bar every candidate is scored against)

Seven needs, from the phase 4 plan (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md`
§1-§2), OD-3, OD-36 and the hard constraints. **N1-N3 are the capability a product can supply. N4-N5 are
the provenance thread, which is ours by definition. N6-N7 are the conditions for shipping it.**

| # | Need | Where it comes from |
|---|---|---|
| N1 | **Interview.** Turn an idea into a brief by asking one question at a time, with a recommended answer, from nothing (greenfield) or drafted from an existing repository (brownfield). The model helps and never gates | plan §1; tasks 03, 04, 13, 18, 21 |
| N2 | **Spec with acceptance criteria**, each with a stable id, in a form a coding agent reads and phase 3 can check before merge (the definition of done) | plan §1; tasks 03, 06, 17 |
| N3 | **Glossary and decision record beside the brief**, each its own record that references a brief revision; a decision is recorded only if it is hard to reverse, surprising, and a real trade-off | OD-36; `2026-09-28-discovery-skills/11-design-r2.md` §2-§3 |
| N4 | **Shared and revisioned server-side** (the Hub), so a teammate sees the same brief; a key such as `brief:bf-…@n` names immutable text and resolves to one `provenance.Ref` on every surface: pane chip, merge list, release | OD-3; plan §2 rules 1-3; tasks 06-08, 11-12 |
| N5 | **Inherited, not advised.** The brief (plus terms and decisions under OD-36) is written into the worktree's `CLAUDE.md` before the agent process starts, and the session is stamped with the key | plan §1; tasks 09, 10, 14 |
| N6 | **Enterprise conditions.** Azure (OD-18); Entra through Keycloak (OD-30); model calls only through the LiteLLM gateway, zero retention, AU path on Bedrock Sydney (OD-19, OD-24); permissive licence for anything deployed into a tenancy; nothing that makes an untrusted agent more trusted | owner decisions file |
| N7 | **Usability.** One obvious start, no wall of configuration; the whole Define step fits inside the cockpit the user already has | `CLAUDE.md` "Why this exists" |

**What is already built and stays.** Nine of phase 4's tasks: the Go packages `modelclient`, `discover`,
`claudemd`, `provenance`, `discoverhub` and `contextwrite` (about 17,000 lines with tests, in the phase-4
worktree) and the `api` Materialiser seam (task 10). They cover N4 and N5, the provenance thread.

**What is not built, and is the target of this study.** Tasks 11-23 (routes, repository analysis, wiring,
all seven desktop tasks, contracts, end-to-end): **32.5 h** in the plan's own table. OD-36's design round 2:
**45.25 h** in phase 4 (nine new tasks 24-32, seven amended) plus **31 h** in the Hub, **about 76.75 h**
before fix rounds (`11-design-r2.md` §7.5). **Together about 109 h before fix rounds, about 140 h with the
ledger's one fix round per task.** Most of it is N1-N3 and the screens that show them.

## 3. Candidates at a glance

_Pending._

## 4. Candidate assessments

_Pending._

## 5. Recommendation

_Pending._

## 6. What we still build (glue)

_Pending._

## 7. Calls only the owner can make

_Pending._

## 8. What this study could not verify

_Pending._

## 9. Sources

_Pending._
