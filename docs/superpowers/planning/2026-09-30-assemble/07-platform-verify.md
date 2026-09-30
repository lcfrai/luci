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
- **Relaunch 2** (after the cohesion step was added): re-read `07-platform.md` in three chunks
  (1-125, 125-235, 235-365). Added to the test list: the user's request that the product feel
  like one app, so each platform piece is also checked for a screen, sign-in or console of its
  own that a user would meet (§6).
- Panacea at `f0a258d8e` (2026-09-30), grep and `sed -n` only. `deploy/main.tf:928` is
  `replicas: 1`, and `:954-957` start Keycloak with `--cache=local`. The comment above them
  records an SSO outage on 2026-09-21: the pod moved twice in 30 minutes, and each move logged
  every user out. No image tag is set, so "25.0.0" is the chart's default (the ruling's U-3
  says the same: "pod not inspected").

## 2. The recommendation, claim by claim

The recommendation under test: Panacea's realm now; our LiteLLM from the MIT tree with
`keysmith` and `gatekeeper`; the Go Hub on Flexible Server; our AKS from Panacea's `.tf` under
OpenTofu. GitHub API figures were read at 12:44 on 2026-09-30; Panacea is at `f0a258d8e`.

| # | Claim in `07-platform.md` | What the source says | Verdict |
|---|---|---|---|
| I1 | Keycloak is Apache-2.0, 26.7.4 of 2026-09-16 | API: `Apache-2.0`, 37,061 stars, `26.7.4` at 2026-09-16T14:44Z | **holds** |
| I2 | Panacea runs Keycloak 25.0.0 on one replica (`main.tf:928`, `:954-957`) | `replicas: 1` and `--cache=local`. No image tag is set, so 25.0.0 is the chart default. The comment above records an SSO outage on 2026-09-21: two pod moves in 30 minutes, and each logged every user out | **holds, and has already failed once** |
| I3 | Keycloak 26 persists user sessions by default | keycloak.org 26.0.0 notes: "In Keycloak 26, this feature is enabled by default" | **holds** |
| I4 | One realm change, no IT ticket | Holds per the ruling (K1-K4). But the issuer is Panacea's **dev** realm, `https://auth.dev.panacea.kpmg/realms/panacea` (ruling line 94), not production. The research never says so | **holds; name the estate** |
| I5 | (not claimed) Is Keycloak 25 still patched? | `SECURITY.md`, "Supported Versions": fixes land "in the current `major.minor` release ... or ... the following". Longer support is only the Red Hat build (a paid subscription) | **new risk**: our sign-in rests on an IdP that gets no fixes |
| G1 | LiteLLM: MIT root, `enterprise/` carve-out, GitHub `NOASSERTION` | Root `LICENSE`: `enterprise/` is under `enterprise/LICENSE`, and the rest is MIT. That licence allows production use only with a subscription "for the correct number of user seats"; dev and test copies are free | **holds**; the price basis is **per user seat** (the docs only say "deployment size") |
| G2 | v1.103.1 of 2026-09-30; the plan pins v1.102.1 | API: `v1.103.1` at 2026-09-30T00:59Z | **holds** |
| G3 | The paid tier holds IP lists, JWT auth, audit logs, SSO over 5 users, tag budgets, team models, key rotation, secret managers | docs.litellm.ai/docs/enterprise lists each one ("SSO is free for up to 5 users"), plus per-model budgets per key. Virtual keys, users, teams and Prometheus are open source | **holds**. One wording error: §0 item 2 says the paid tier "keeps pod binding". No tier binds a key to a pod; 3.16 has it right |
| G4 | Panacea's gateway cannot serve agents | Stronger at source. `allow-vvah-to-gateway.yaml:82` says Panacea's pinned proxy, v1.40.0, "serves no /v1/messages route at all". Claude Code speaks only the Messages format through `ANTHROPIC_BASE_URL` | **holds, stronger** |
| G5 | The cited Panacea lines: `llm_vkeys.go` `:79`, `:209`, `:263-340`, `:458`; `llm_env_keys.go:100-110`; `ai_settings.go:218`; `deploy_analysis.go:30-31`; v1.63.11 and v1.40.0; no `turn_off_message_logging` | Each is at the cited line. The two versions are in `litellm-values.yaml:31` and the `main-v1.40.0` Dockerfile `FROM`. No `turn_off_message_logging` appears under `deploy/` or `kubernetes/` | **holds** |
| G6 | Refusing `count_tokens` closes the AU leak (task 26) | Claude Code's gateway guide: token counting is "the only optional" endpoint; without it, Claude Code "falls back to a character-based estimate" | **holds**: the refusal costs only exact `/context` numbers |
| G7 | (not claimed) Is the LiteLLM supply chain safe? | BerriAI's post of 2026-03-24: PyPI `litellm==1.82.7` and `1.82.8` were backdoored for about 40 minutes, through the Trivy step in its CI. The official Docker image was not affected. Images are cosign-signed from v1.83.0 | **new risk**, which the plan's build-from-source and digest pin already answer. Add: build from a tagged commit and lock the Python dependencies by hash |
| H1 | The Hub schema needs no superuser and no extension | `roles.sql` is 11 lines: two `NOLOGIN` roles and one `GRANT`. `0001_init.sql` (693 lines) uses built-in `sha256()`, PL/pgSQL, `SECURITY DEFINER`, triggers and row-level security on `current_setting`. It has no `CREATE EXTENSION`, event trigger or `ALTER SYSTEM` | **holds** (read, not applied) |
| H2 | Burstable B2ms costs US$0.208 an hour in Australia East | Retail Prices API: `B2ms vCore`, 0.208 USD per hour | **holds** |
| H3 | Panacea's `postgres-flexible.tf` gives HA, PITR and geo-backup (`:255-285`) | Geo-backup is at `:255-256`, private endpoint only at `:263`, HA at `:268-271`. The HA block is unconditional and "Requires a General Purpose / Memory Optimized SKU" | **holds**, with one trap: P2's Burstable dev server cannot copy the file unchanged |
| E1 | OpenTofu is MPL-2.0, v1.12.6 | API: `MPL-2.0`, `v1.12.6` at 2026-08-19 | **holds** |
| E2 | Panacea's `.tf` runs on OpenTofu | 22 files. They use `required_version = ">= 1.6.0"`, `backend "azurerm"` and five `check` blocks, all in OpenTofu 1.6+. There is no `cloud` block, ephemeral resource, write-only argument or provider function | **holds** |
| E3 | The ISM cites: ISM-0459 37 times, ISM-1416 and ISM-1277 12 each | Recounted: 37, 12, 12 | **holds** |

**What survives:** every licence and version figure, and every Panacea line cited. **What
changes:** sign-in rests on Panacea's *dev* Keycloak 25. That release is out of upstream
support, and it has already logged everyone out once (I2, I4, I5). LiteLLM's March breach
should be named, even though the plan's controls answer it (G7).

## 3. Alternative 1: Supabase, self-hosted (and Coolify)

**Verdict: the rejection holds (reference, not the Hub).** One stated reason is wrong, and four
stronger ones were missed.

| Check | What the source says | Effect on 3.2 / 3.20 |
|---|---|---|
| Licence | `supabase/supabase` Apache-2.0, `supabase/auth` MIT, PostgREST MIT, the chart Apache-2.0 (API). Realtime, Storage, Supavisor, postgres-meta and Logflare are Apache-2.0 (raw `LICENSE` files; the API was rate-limited by then) | **holds**: all permissive |
| Self-hosting limits | The vendor page, verbatim: Docker is "the fastest and recommended way"; the Helm charts are "community-driven"; "branching, advanced metrics beyond logs, managed backups and PITR, analytics and vector buckets, ETL, and the platform management API are unavailable"; "Studio doesn't support multiple organizations or projects"; "Self-hosted Supabase is community-supported" | **holds** |
| Trusting Keycloak | Keys are static values in `.env` (`JWT_SECRET`, `JWT_KEYS`, `JWT_JWKS`). No remote JWKS is described | Holds for trusting realm tokens as they are. **But "only by a pasted key list" is wrong.** Supabase Auth has a built-in `keycloak` provider (README `:436`, `:467`). That path makes it a third hop that issues its own JWT, so each user holds two sessions: Supabase's for data, and the realm's for the gateway and fleet (K2). That breaks the ruling's single token profile |
| Operator console | Studio sits behind one shared `DASHBOARD_USERNAME` / `DASHBOARD_PASSWORD` (`docker/.env.example:58-59`), enforced by Envoy `basic_auth` filters (`lds.template.yaml:71`) | **new**: no SSO for operators. It fails the Entra rule unless we put oauth2-proxy in front |
| What KPMG can buy | Self-hosting has no paid tier and is "community-supported" | **new**: no support contract is available for our system of record |
| Maintenance | `supabase/supabase` was pushed on 2026-09-30 (110,907 stars). The Kubernetes chart is community-run (818 stars, `supabase-0.8.0` of 2026-09-05). The compose file pins PostgREST `v14.17`; upstream is `v16.4` | The AKS path rests on a community chart that lags upstream |
| Services to run | The current compose file has 11 images. The API gateway is now `envoyproxy/envoy:v1.39.1`, kept under the alias `kong` | "About twelve" holds. "Behind Kong" is out of date |
| Days (13 against 10) | `hubapi.Canonical` is RFC 8785 JCS (`canonical.go:14-21`). Postgres `jsonb` does not keep key order or the input's exact form (Postgres manual, JSON types; from knowledge, not fetched today), so a cast cannot reproduce the golden vectors. The org scope is a server-set `hub.org_id`, and the realm issues no org claim | **Plausible, and a floor**: a hand-written JCS in PL/pgSQL or Deno, plus a claim mapper or auth hook, are both new work |

**Coolify (3.20) at source.** `LICENSE` is Apache-2.0; `v4.3.23` of 2026-09-18, 62,411
stars (API, 12:44). Its dashboard signs in through GitHub, GitLab, Google, Azure, Bitbucket,
Discord or Infomaniak, and through Authentik, Clerk or Zitadel by guide
([coolify.io OAuth](https://coolify.io/docs/knowledge-base/oauth)). **Keycloak and generic
OIDC are not listed.** So operators would reach Entra only through a new app registration,
which is the IT ticket `OD-30` exists to avoid. On Kubernetes, a search summary of Coolify's
own scalability page and its v5 milestone says support is "planned" with no date, and that v5
builds its own Docker-based multi-server layer (a summary only; the page was not fetched). The
rejection holds: no Kata, a second platform to run, and a dashboard outside our sign-in.

**Cohesion (the user's ask).** Taking Supabase and Coolify would add three consoles a person
could meet: Studio, Coolify's dashboard, and a Supabase-issued session beside the realm's.
Each has its own look and its own sign-in. That is the "separate apps merged into one" result
the user asked us to avoid, and one more reason against them.

## 4. Alternative 2: the Hub hosted as a Panacea project

**Verdict: it stays the fallback only, and it costs more than 1.5 days.** Every Panacea line
3.4 cites is at source. The glue is larger than stated, and the failure domain is live today.

| Check | What the source says | Effect on 3.4 |
|---|---|---|
| The routes | `POST /api/v1/databases` (`routes.go:1325`); realtime (`:2911-2912`); the Data API `rest/{table}`, where reads need membership and writes a developer (`:2915-2918`) | **holds** |
| Roles | The tenant runtime role is `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS` (`db_roles_ensure.go:100-101`). Its grants cover schema `public` only (`db_roles_ops.go:113-124`) | **holds**. The Hub cannot create `hub_owner` or `hub_app` there |
| The size of that rework | `0001_init.sql` names `hub_app` 23 times and `hub_owner` 3 times. The migrate package's three integration test files hold 40 tests | **new**: the schema's grants must be templated onto Panacea's generated role names and re-proven. Otherwise the Panacea stream must create the roles for us, which is another ask during its cutover |
| Builds from a subdirectory (3.4's open question, §9.2) | `panacea-build` resolves a build context per component (`manifest/types.go:79-84`, `:111-132`, "defaulted to `./<name>`") | **answered by reading**: `control-plane/cmd/hub` can be a component. Not run |
| Serving without SSO | Namespace access modes include `panacea-sso`, `kpmg-sso` and `none` (`access_mode_inherit_test.go:25-83`). The Hub validates its own bearer tokens, so it needs `none` | **holds**: a posture exception |
| The shared failure domain | Two recent production incidents. On 2026-09-21, Keycloak's pod moves logged everyone out (`main.tf` comment, §2 I2). And the owner's memory note of 2026-09-28 (not re-verified today) records every cert-manager renewal dead since mid-August. That was fixed by a hotfix which Argo CD "REVERTS" when it reconnects, and Keycloak's TLS secret there has no `Certificate`, so it never renews | **stronger than stated**: our system of record would share an estate that is actively firefighting |

**Days.** 3.4 says about 1.5 engineer-days of hosting glue. Templating 26 role references and
re-running 40 tests, plus the posture exception, the egress allow-list and the DSN wiring, come
to **about 2.5 to 3 days** (our estimate). The Hub's own 10 days are unchanged under either
host.

**When it is right.** Only if the Azure subscription is late, as 3.4 says. Even then, the
cheaper bridge may be Flexible Server in any KPMG subscription that exists today, with the Hub
as one container beside it. That avoids the role rework entirely (not costed; §8).

## 5. The days, against our architecture

_Pending._

## 6. Cohesion: does the platform layer feel like one app?

_Pending._

## 7. Missed candidates

_Pending._

## 8. The corrected recommendation

_Pending._

## 9. What I could not verify

_Pending._

## 10. Sources

_Pending._
