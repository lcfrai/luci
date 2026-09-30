# 08 - Whole products that already do most of it

**Status:** IN PROGRESS (skeleton written first; sections fill in one at a time).
**Date:** 2026-09-30. **Cluster:** 08-whole-product. **Feeds:** the assemble plan, alongside 00-06.
**Question:** is there an end-to-end product that already covers most of Define -> Design ->
Develop -> Deploy with parallel cloud agents for an enterprise, which we could build on
(white-label, extend, embed) or buy instead of building PanaceaApp's phases, and what would still
be ours (the provenance thread, the cockpit UX, KPMG compliance)?

**Direction this was written under (2026-09-30, owner):** "focus on using third party solutions,
and solving the main problem statement and building the tool. I need this tool ASAP, not in
weeks." Second direction, same day: Orca (`stablyai/orca`, MIT) is the cockpit pick; Panacea is the
strong default for platform services and the best fully-fledged solution wins ("Panacea is fine to
use majority of, if we want to use Coolify plus Supabase, and enhance with snippets from Panacea,
that is fine as well. As long as we get the best solution that is fully fledged."). OD-13 relaxed:
Panacea snippets may be copied with attribution in the file header.

**Method:** vendor pages and docs (WebSearch, WebFetch), the GitHub API for stars, licences and
releases, the GLASSES catalogue (untrusted discovery data), and the sibling clusters 00-06 in this
folder. Nothing installed or run. Every claim that could not be checked is marked unverified.

---

## 1. Answer first

_(pending)_

## 2. The bar a whole product has to clear

A "whole product" here means one vendor's product that already runs **parallel coding agents in
the cloud for a team** and covers at least two of Define, Design, Develop and Deploy end to end.
Libraries and single parts are the other clusters' business (01-06). Each candidate is scored on
the ten tests below; B1-B7 are hard rules from the owner decisions, B8-B10 are the owner's
direction of today.

| # | Test | Source | What fails it |
|---|---|---|---|
| B1 | **Runs on Azure, in our own environment first, in a customer's own Azure later** | `OD-5`, `OD-18` | SaaS with no self-hosted or bring-your-own-cloud mode; a self-hosted mode on AWS or GCP only |
| B2 | **An AU data-residency option** for code, transcripts and inference | `OD-19`, `OD-24` | A vendor region list without Australia; inference that leaves the region |
| B3 | **Entra sign-in through our Keycloak broker** (OIDC or SAML to Keycloak), zero clicks | `OD-30` | Sign-in only through a git host or the vendor's own accounts |
| B4 | **Inference through our LiteLLM gateway, zero data retention**, attributed per user, team and project; no personal subscriptions | `OD-19`, `OD-21`, `OD-33` | The vendor's own model hosting, or a seat that bundles inference we cannot route |
| B5 | **Unmodified agent CLIs** (Claude Code, Codex, Gemini CLI), each session in its own sandbox on its own clone; nothing reads PTY bytes for meaning | `OD-2`, `OD-8`, `CLAUDE.md` | The vendor's own agent only, or a third-party CLI wrapped behind the vendor's chat |
| B6 | **Untrusted-code isolation**: microVM class (Kata on AKS in our plan) | `CLAUDE.md` "Isolation" | Shared-kernel containers on a shared host |
| B7 | **Both Azure DevOps and GitHub** for repos and pull requests | `OD-4` | GitHub only, or GitHub, GitLab and Bitbucket without Azure Repos |
| B8 | **Licence**: permissive (MIT, Apache-2.0, BSD, ISC, MPL-2.0) for anything we deploy into a customer tenancy, or a commercial product KPMG can buy | task brief; `OD-32` | AGPL, GPL, SSPL, BUSL, fair-code, Elastic, PolyForm and other source-available terms, unless bought |
| B9 | **Fits beside Orca**, the owner's cockpit pick | second direction | A product whose value is its own cockpit, so adopting it means a second cockpit |
| B10 | **Days, not weeks** to a first real use by a KPMG team | first direction | A procurement, a platform migration or a git-host move before first use |

**What no vendor will cover, whatever it is.** The provenance thread (the brief written in Define,
referenced in Design, carried into Develop, attached to the release), "done means proven"
(`OD-27`), KPMG's compliance evidence per release, and the cockpit's journey shell. Every
candidate below is judged for what it removes from our plan, never for those four.

**How a candidate is counted as covering a phase.** *Define*: turns an idea into a written,
reviewable spec or brief that later steps read. *Design*: a canvas or UI prototyping step whose
output the build uses. *Develop*: parallel agents in cloud sandboxes, each on its own branch.
*Deploy*: a running URL per branch, promote, rollback, on the customer's infrastructure.

## 3. The candidates at a glance

_(pending)_

## 4. Candidates in detail

_(pending)_

## 5. Which can run in a customer's own Azure tenancy

_(pending)_

## 6. Revised for the owner's second direction

_(pending)_

## 7. The recommendation

_(pending)_

## 8. What we still build (glue)

_(pending)_

## 9. Owner calls

_(pending)_

## 10. What this run could not verify

_(pending)_

## 11. Sources

_(pending)_
