# Luci reuse decisions and evidence

Reviewed 2 October 2026. This is discovery and integration guidance, not evidence that a remote deployment has succeeded. No upstream application source was imported during this review.

## Decision

Keep the existing Tauri/React/xterm cockpit and Go local service. Embed Excalidraw as the first Design canvas. Integrate Coolify's complete deployment engine behind Luci's Deploy interface for users who choose a remote server. Keep the local path independent of that server. Reuse these working capabilities instead of writing a new canvas or PaaS. Retain one Luci project, navigation, terminology, theme, and release history.

For Define, retain the local brief and use Spec Kit as a possible recipe reference. For Develop, retain the working cockpit and BYO agent connections; Orca is a credible whole-product alternative, but replacing the current foundation adds migration and packaging work. For local storage, retain the existing Go persistence rather than introduce a second backend. Penpot, PocketBase and LiteLLM are optional future adapters with specific needs, not default dependencies.

## Glasses discovery actually performed

Both the installed `glasses_local` MCP and public [lcfr.ai MCP](https://lcfr.ai/api/mcp) were queried. The public endpoint exposed `glasses_status`, `glasses_search`, `glasses_inspect`, and `glasses_evidence`; its snapshot contained 70,064 entries, generated `2026-10-01T02:10:17.356Z`, content hash `75166d21aee6ed89fed9ff37476b997ac6347030c528a48f95d894f16542c778`. The local catalogue contained 73,413 entries. Searches included exact candidate names and deployment/canvas/backend capabilities. Promising entries were inspected. Catalogue classifications are leads, not proof of compatibility, current licensing, or production readiness.

The local service initially was unreachable. Its existing installation was started normally with background scouts disabled for this process; no package installation, source modification, cloud creation, or paid inference was required. It remains available at `http://127.0.0.1:4317`.

| Candidate and Glasses entry ID | Source and licence evidence | Decision and cost implications |
| --- | --- | --- |
| Mission Control `c2fef5ebe44fb6767872` | [Repository](https://github.com/builderz-labs/mission-control), [MIT licence](https://github.com/builderz-labs/mission-control/blob/e28edf8b28c85a32f1fd55ac577d325df6b16e4a/LICENSE) and [README at inspected revision](https://github.com/builderz-labs/mission-control/blob/e28edf8b28c85a32f1fd55ac577d325df6b16e4a/README.md). Glasses retained licence evidence `e35c109ea6319429ddf1` and README evidence `7ae77cd8d35de9f601d3`; original licence SHA256 `e1fb39bdbcefc6a2a27d29ec54f1be67b19356d7562d367f742566addc1b22b0`. | Inspected the whole solution before adding managed accounts. Its Next.js/SQLite operator application already offers roles, tasks, runtime adapters, audits and spend views, but declares alpha status and explicitly distinguishes itself from managed multi-tenant SaaS. Defer adoption: running or replacing Luci with a second control application adds a Node service, persistence and workspace migration. Keep the existing Go/Tauri application and isolated user runtimes; no Mission Control code, assets or binaries were imported. Its licence does not itself prove Luci's identity and execution boundaries. |
| GitHub CLI `440ac9df5ef1fbd34f29` | [Official repository](https://github.com/cli/cli), [MIT licence at inspected revision](https://github.com/cli/cli/blob/fc4b137cdef0a6bd28fd461b7cf9c84a5812a8cd/LICENSE). Glasses search and inspect retained README evidence `1c09e4a2d481b6dc85dd`; primary CLI help and [sign-in](https://cli.github.com/manual/gh_auth_login)/[repository creation](https://cli.github.com/manual/gh_repo_create) contracts checked. | Reuse the installed whole CLI for account sign-in, private-by-default repository creation, and Git credential lookup. Luci does not implement OAuth or copy tokens into project state. Existing Git provides immutable-commit push, remote verification and [bundle history](https://git-scm.com/docs/git-bundle) for backups. No CLI source or binary is bundled by this adapter. |
| Excalidraw `e25d5aae975b96dbca02` | [Repository](https://github.com/excalidraw/excalidraw), [MIT licence at inspected revision](https://github.com/excalidraw/excalidraw/blob/1919728724a1b71af73cb7e6d2d1a418a1415b1c/LICENSE). Glasses README citation `6f326e9fe4b49ef8741b`; retained licence evidence `ed2b25a26175bd7ce7aa`. | Adopt embedded React editor. No service or account required for local scene editing. Luci supplies durable persistence and local assets. Wireframes and diagrams, not a complete design-to-production-code system. |
| Coolify `d9575f3020769f3b284b` | [Repository](https://github.com/coollabsio/coolify), [Apache-2.0 licence at proposed release](https://github.com/coollabsio/coolify/blob/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/LICENSE). Glasses README citation `167324e662d341838f16`; retained licence evidence `764c469cdbf6a3e5c0fc`. | Adopt optional remote deployment engine through API. Self-hosting has no platform subscription; server, storage, backups, network, domain and operations still cost money. |
| Orca `0a8884baa5f55aa6602b` | [Repository](https://github.com/stablyai/orca), [MIT licence](https://github.com/stablyai/orca/blob/main/LICENSE). Glasses metadata citation `908b301c4a6f2e3b7199`; inspected source revision `3727100cc9dbcea6201f8a3e506676a3c4b53b18`. | Keep as future whole-cockpit alternative. Desktop and headless modes exist; adopting now displaces useful Tauri/Go work. Agent subscriptions or API usage remain separate. |
| Penpot `c0d4875dd605f77caf57` | [Repository and MPL-2.0 licence](https://github.com/penpot/penpot). Glasses README citation `a832c7932cdebfe53618`; prior licence evidence `626be444747ce7051878`. | Optional richer design/prototype service later. More services and integration work than an embedded local canvas. Preserve its licence boundaries and attribution if adopted. |
| PocketBase `d8cd1778e5067988f4fc` | [Repository and MIT licence](https://github.com/pocketbase/pocketbase). Inspected catalogue revision `5cec579da984436a258602a46a96302fbd31f77c`. | Complete small Go backend if auth/realtime/admin becomes necessary. Do not introduce a second persistence system just to reuse something. |
| LiteLLM `dab892839df45099e3fc` | [Repository](https://github.com/BerriAI/litellm), [MIT licence with enterprise-directory exception](https://github.com/BerriAI/litellm/blob/main/LICENSE). Glasses README citation `00c4f2276bcd5181dcbe`. | Optional team provider gateway. Keep local BYO harness/API/model paths simpler; enterprise code is not covered by the root MIT grant. Inference is separately billed. |
| Spec Kit `f4e0adf84666b0c986e7` | [Repository and MIT licence](https://github.com/github/spec-kit). Catalogue revision `d2ddd910266ad14afe79c5414c88ee316aca6e9a`. | Reusable specification workflow ideas for Define. No hosted service needed. |
| Dokploy `c0f70d661d1e7f729404` | [Repository](https://github.com/Dokploy/dokploy), [current licence](https://github.com/Dokploy/dokploy/blob/canary/LICENSE.MD): Apache-2.0 outside `/proprietary`, separately licensed proprietary directories. | Credible complete deployment alternative, but do not operate two PaaS adapters initially. Coolify is selected. |

Glasses explicitly marked the current Excalidraw and Coolify licence state `unverified-current-source`: newer README revisions existed than the retained licence checks. This was not treated as a licence grant. The exact current/proposed upstream licence links above were opened separately. `glasses_evidence` returned the retained original licence responses: Coolify response SHA256 `284cf6c516b68dfd5e1c3285229fbcf7c9654237234beade75dfda2319e41cb1`; Excalidraw response SHA256 `e269449f6fb69dfdc7f79c1237b749d85d938298ca4b901f926f279abb8f062a`. These hashes describe retained API responses, not downloaded package archives. Audit the exact locked dependency versions and bundled fonts/assets before distribution.

## Embedded Design integration

Use `@excalidraw/excalidraw` in the current React application. Persist scene elements, app state and referenced files under the same Luci project, with revision/brief linkage. Preserve export/import and undo behavior. Hosted Excalidraw collaboration and persistence are not automatically provided by the editor package.

The implementation copies the used font families to local application assets and sets `window.EXCALIDRAW_ASSET_PATH` before loading the editor. Excalidraw 0.18.1 also appends CDN fallback URLs even with that setting, so `desktop/vite.config.ts` applies a narrow build transformation to keep those font fallbacks on the application's `/design-assets/` origin. The installed upstream package remains unchanged; the shipped JavaScript includes this documented adaptation. Eight font families are distributed with their original grants and byte hashes in the licence inventory. The server-side-only Liberation family is excluded. See [official installation and asset guidance](https://docs.excalidraw.com/docs/@excalidraw/excalidraw/installation). Offline save/reopen/export remains a release acceptance check.

## Coolify integration contract

Use the user's trusted HTTPS Coolify origin and `/api/v1`. The local adapter also permits HTTP to a literal loopback IP when an independently authenticated SSH tunnel supplies the remote transport; it rejects HTTP to remote hosts and to DNS names. All protected calls carry `Authorization: Bearer <complete-token>`; JSON bodies carry `Content-Type: application/json`. Validate the connection using `GET /teams/current`, then list servers/projects. A self-hosted administrator must enable API access. Tokens are scoped to a user/team and permissions. Keep credentials in the local secret store, redact logs, and never redirect an authenticated request to a different origin. See [authorization](https://coolify.io/docs/api/authorization) and [API overview](https://coolify.io/docs/api/overview).

Luci should own the themed connection/token-entry form, resource selection, readiness checks, domain configuration, progress, errors and health verification. No embedded vendor administration screen is needed for daily work. Public API discovery did not establish an unauthenticated first-admin/API-token creation endpoint. Fully automated first-time token provisioning therefore remains an explicit bootstrap integration task; do not invent such an endpoint or claim a token can be created without authorization.

The following shapes were checked against the official endpoint documentation and the [OpenAPI file pinned to v4.3.23](https://github.com/coollabsio/coolify/blob/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/openapi.json):

| Operation | Method/path and useful request fields |
| --- | --- |
| Create project | `POST /projects` with `{"name":"luci-example","description":"Managed by Luci"}`; response includes `uuid`. [Docs](https://coolify.io/docs/api/endpoints/projects/create-project) |
| Create environment if absent | `POST /projects/{project_uuid}/environments` with `{"name":"production"}`. [Docs](https://coolify.io/docs/api/endpoints/projects/create-environment) |
| Find server | `GET /servers`; retain a validated server UUID. Self-hosted installations have a localhost server. Creating a separate server uses `POST /servers` with `name`, `ip`, `port`, `user`, `private_key_uuid`, `instant_validate`, and optionally `proxy_type`. |
| Application from Git | `POST /applications/public`; example below. Private Git uses the supported deploy-key/Git-app route and credentials. [Docs](https://coolify.io/docs/api/endpoints/applications/create-public-application) |
| Prebuilt image | `POST /applications/dockerimage` with `project_uuid`, `server_uuid`, `environment_name`, `docker_registry_image_name`, `docker_registry_image_tag`, `ports_exposes`, `domains`, `instant_deploy:false`. Prefer an immutable release tag. [Docs](https://coolify.io/docs/api/endpoints/applications/create-dockerimage-application) |
| Compose service | `POST /services` with project/server/environment, `docker_compose_raw` containing base64 Compose content, `urls:[{"name":"web","url":"https://app.example.com"}]`, and `instant_deploy:false`. Response includes `uuid` and `domains`. [Docs](https://coolify.io/docs/api/endpoints/services/create-service) |
| Trigger release | `POST /deploy?uuid={resource_uuid}&force=false`; response contains `deployments[].deployment_uuid`. [Docs](https://coolify.io/docs/api/endpoints/deployments/deploy-by-tag-or-uuid) |
| Observe deployment | `GET /deployments/{deployment_uuid}` returns status, commit and logs. Also verify the intended public HTTPS health URL before recording a successful Luci release. [Docs](https://coolify.io/docs/api/endpoints/deployments/get-deployment-by-uuid) |

```json
{
  "project_uuid": "<resolved-project-uuid>",
  "server_uuid": "<validated-server-uuid>",
  "environment_name": "production",
  "name": "luci-example",
  "git_repository": "https://github.com/your-account/your-project",
  "git_branch": "main",
  "git_commit_sha": "<release-commit>",
  "build_pack": "dockerfile",
  "dockerfile_location": "/Dockerfile",
  "ports_exposes": "3000",
  "domains": "https://app.example.com",
  "instant_deploy": false,
  "is_auto_deploy_enabled": false,
  "health_check_enabled": true,
  "health_check_path": "/health"
}
```

Use either `environment_name` or `environment_uuid`; the schema lists both as required but its descriptions state that one is sufficient. Keep this in adapter contract tests. Example values are illustrative, not existing user resources.

**Local-source transfer is separate.** The raw `/applications/dockerfile` endpoint has no repository build context; `COPY` cannot read Luci's local files. Use reachable Git, publish a prebuilt image with appropriate credentials, or implement an explicit tested source-transfer/build path. Creating an API resource does not upload source or prove deployment. [Upstream Dockerfile explanation](https://coolify.io/docs/applications/builds/dockerfile).

For Git Compose applications use `build_pack:"dockercompose"`, `docker_compose_location`, and `docker_compose_domains:[{"name":"web","domain":"https://app.example.com"}]`. The service endpoint's `urls[].url` shape differs. Do not interchange them. Treat HTTP 409 domain conflicts as a decision to resolve, not an instruction to force override. Store created UUIDs in Luci state so retries do not duplicate projects/resources after an unknown response.

## Versioned bootstrap and operating cost

The latest stable release returned by the upstream GitHub API on the review date was [v4.3.23](https://github.com/coollabsio/coolify/releases/tag/v4.3.23), published 18 September 2026, commit `e2e2d4010bcd590084b66d6f748f3eec8e2bbee9`. The following is a **reviewed command template**, not a command executed during this research. Run it only on the user's selected fresh Linux server after Luci's cost acknowledgement and host checks:

```bash
set -euo pipefail
workdir=$(mktemp -d)
trap 'rm -f -- "$workdir/install.sh"; rmdir -- "$workdir"' EXIT
curl --fail --show-error --silent --location \
  https://raw.githubusercontent.com/coollabsio/coolify/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/scripts/install.sh \
  --output "$workdir/install.sh"
printf '%s  %s\n' \
  '8ef02dce49339208f5abc247bff0277c73d04538d7a36dcfd21331e314e0f2cd' \
  "$workdir/install.sh" | sha256sum --check --status
sudo env AUTOUPDATE=false bash "$workdir/install.sh" 4.3.23
```

The SHA256 was calculated over the raw script bytes fetched at that commit. The [script](https://github.com/coollabsio/coolify/blob/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/scripts/install.sh) accepts the release number as its first argument and honors `AUTOUPDATE=false`. This pins the installer and main application version, **not the full dependency supply chain**: it still obtains Compose/upgrade assets from the Coolify CDN, helper versions and OS packages at execution time. A fully reproducible installer needs separately pinned/verified dependencies; do not describe this template as hermetic.

Bootstrap must protect first-admin setup, retain generated secrets/backup material, and restrict management access. The official installer supports predefined root account environment variables, but these are credentials: they must not appear in committed cloud-init, process logs, copied command previews or chat. The platform installation requires at least **2 CPU cores, 2 GB RAM and 10 GB disk**, before allowing headroom for builds and applications. See [self-hosted requirements and setup](https://coolify.io/docs/start-with-self-hosted).

Consequently, do not recommend an undersized cheapest VM simply because it can run one small container. Offer existing remote Coolify, a suitably sized BYO server, or the simpler local path. A single server avoids mandatory Kubernetes, NAT gateways, load balancers and managed databases, but creates a shared failure domain. Backups and restore checks remain necessary. Cloud quotes must come from the chosen provider/region at setup time and include disk, public IPv4, transfer, backup and tax/currency assumptions. A plan is not a verified quote.

For a lightweight direct-Compose path, Caddy is a useful separate reverse-proxy option. Its [automatic HTTPS](https://caddyserver.com/docs/automatic-https) requires correct DNS, reachable ports and persistent certificate data; wildcard certificates require DNS challenge support. Do not add a competing proxy in front of a Coolify-managed server without a deliberate routing design.

## Acceptance evidence still required

`control-plane/internal/product/coolify.go` implements the standalone client for token/team checks, server/project listing, unambiguous project reuse, commit-pinned public Git application creation, deployment triggering and status polling. It does not own target configuration, SSH tunnels, cloud consent or persistence. Its `httptest` coverage exercises the API journey, duplicate-project reuse, input validation, redirect refusal, secret-safe errors, cancellation, malformed/oversized responses, and the requirement for health verification after provider completion. Mock-server tests are not a real remote-deployment result.

`remote_deploy.go` integrates that client with the product service. `POST /v1/product/projects/{id}/releases/cloud` accepts a connected target, accessible public HTTPS Git repository, branch, current HEAD commit, domain, port, Dockerfile and health path. It verifies the clean local commit and committed brief, reserves a release outside the repository, saves each provider identifier, reuses the existing application for later commits, and does not automatically repeat an ambiguous mutation. `POST /v1/product/projects/{id}/releases/{releaseId}/refresh` checks the provider's commit/status and public HTTPS health. Remote DNS must resolve to public addresses; redirects are refused. Local and cloud records share the existing release listing. Tests cover restart recovery, duplicate requests, later commits, provenance failures, uncertain outcomes, and actual local TLS test-server responses. An unresolved ambiguous provider outcome still requires reconciliation before another mutation.

`provider_login.go` supplies explicitly initiated local sign-in jobs. AWS uses the CLI's [native `aws login` browser flow](https://docs.aws.amazon.com/cli/latest/reference/login/); Azure uses [device-code sign-in](https://learn.microsoft.com/en-us/cli/azure/authenticate-azure-cli-interactively) with process-only settings that suppress the hidden subscription selector and Windows broker. The jobs expose only allowlisted provider verification links and Azure's user code, never raw CLI output. Credentials remain in the provider CLI cache. Jobs are limited to ten minutes and cancelled when Luci closes. Tests inject processes; they do not authenticate a real account. On Windows, Azure's [official MSI launcher](https://github.com/Azure/azure-cli/blob/dev/build_scripts/windows/scripts/az_msi.cmd) delegates to its adjacent Python runtime; Luci invokes that same runtime directly to avoid shell argument parsing and to own process cancellation.

Validation completed for these additions: `go test ./internal/product -run 'Coolify|CloudRelease|ProviderLogin' -count=1` passed on the development host. This is local integration evidence, not a clean-install or cloud acceptance result.

Test an actual clean-machine install and the embedded canvas save/reopen/export path with networking disabled. Against the pinned Coolify version, test connection failures, insufficient token permissions, repeated creation, deployment failure, domain conflicts, DNS/TLS readiness, release health, restart recovery and cleanup. Test both cloud-provider provisioning adapters with separately acknowledged real resources before marking them production-ready. None of those remote acceptance results is implied by this discovery document.
