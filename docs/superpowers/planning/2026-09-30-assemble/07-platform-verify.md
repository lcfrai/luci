# 07 Platform (shared state, identity, AI gateway, environment): the sceptic's verification

**Status:** IN PROGRESS (2026-09-30). Skeleton; sections are filled one at a time.
**Date:** 2026-09-30. **Verifies:** `07-platform.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run; nothing written in `Projects/Panacea`.

## 0. Verdict first

_Pending._

## 1. What was checked, and how

_Working notes (relaunch after the 11:59 stop), filled as the read proceeds._

- Read `07-platform.md` §0-§3.8 (lines 1-120). The recommendation under test: Panacea's
  Keycloak realm now (3.10); our own LiteLLM from the MIT tree plus `keysmith` and
  `gatekeeper` (3.14); our Go Hub on Azure Database for PostgreSQL Flexible Server (3.1);
  our AKS in Australia East with Panacea's Terraform copied into OpenTofu (3.18, 3.21).
  The two alternatives under test: self-hosted Supabase, with Coolify (3.2, 3.20), and the
  Hub hosted as a Panacea project (3.4).
- Read §3.8-§4 (lines 120-229). Facts to test at source: Keycloak Apache-2.0 and 26.7.x;
  LiteLLM's MIT root, `enterprise/` carve-out, v1.103.1, its paid-tier list and price
  basis; Supabase's self-hosting limits and JWKS handling; Coolify's licence, version and
  Kubernetes gap; PostgREST, Electric, Temporal, Bifrost, Portkey licences; APIM's
  Anthropic pass-through; Foundry's Australia East gap; AVM and OpenTofu.
- Read §4-§9 and Sources (lines 229-365). The headline: about 19.2 engineer-days to first
  use (Hub 10, gateway 8.2, environment 1, realm 0), about 1.25 of it new glue. Claims to
  test in Panacea (grep only): Keycloak 25.0.0 on one replica (`deploy/main.tf:928`,
  `:954-957`), `NOCREATEROLE` (`clients/db_roles_ensure.go:97-109`), the key code
  (`handlers/llm_vkeys.go`), Bedrock removed (`handlers/ai_settings.go:218`); and whether
  Panacea's `.tf` uses anything OpenTofu lacks. In our repo: `roles.sql` on `feat/hub`.

## 2. The recommendation, claim by claim

_Pending._

## 3. Alternative 1: Supabase, self-hosted (and Coolify)

_Pending._

## 4. Alternative 2: the Hub hosted as a Panacea project

_Pending._

## 5. The days, against our architecture

_Pending._

## 6. Missed candidates

_Pending._

## 7. The corrected recommendation

_Pending._

## 8. What I could not verify

_Pending._

## 9. Sources

_Pending._
