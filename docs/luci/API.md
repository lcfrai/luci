# Luci application contract

Version 0.4, 2 October 2026. JSON is camelCase. Routes below are additions to the retained session API at /v1. Errors are `{error: string, message: string}`. Requests reject unknown JSON fields. A control API read never starts a session/provisioning job or creates a resource. Bounded read-only Git subprocesses are allowed. Application preview requests are forwarded to the selected application's own HTTP surface.

## Setup

### Identity and management

The control-plane gateway authenticates the API before it reaches a user's execution service. `GET /v1/control-plane/status` is public and returns `{product: "Luci", version, configured, instanceName, backing: "docker" | "aws" | "azure", bootstrapRequired, bootstrapTokenPath?}`. First setup reads the owner-only `StateDir/control-plane/bootstrap-token` file. `POST /v1/auth/bootstrap` accepts `{bootstrapToken, username, displayName, password}` and returns `201 {token, user}` for the first Super Admin. The bootstrap file is removed after success. `POST /v1/auth/login` accepts `{username, password}` and returns `{token, user}`. Passwords require 12ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Å“1,024 characters and use the Go project's Argon2id implementation with random salts; derivation concurrency and login attempts are bounded.

API requests use `Authorization: Bearer TOKEN`. Sessions expire after twelve hours; only SHA-256 token hashes are persisted. `GET /v1/auth/me` returns `{user, permissions, policy}`. User fields are `{id, username, displayName, role: "member" | "admin" | "super-admin", disabled}`. Permissions are `{manageUsers, manageBacking, managePolicy, inspectAudit, runHostCommands}`. `POST /v1/auth/logout {}` revokes the current session. `POST /v1/auth/password {currentPassword, newPassword}` revokes existing sessions and returns `{signInRequired: true}`.

`POST /v1/auth/socket-ticket {path}` accepts a canonical session attach path and returns `{ticket, expiresAt}`. A ticket is bound to its user, bearer session and exact path, expires in at most thirty seconds, and is consumed once by the WebSocket request's `?ticket=` parameter. It is removed before forwarding. Already-upgraded sockets close when their session expires or is revoked, including the initial owner's inline terminal. Application preview query parameters named `ticket` remain application data; preview bridges authenticate separately with a bearer token and preserve their isolated application's cookies.

Administrators can `GET /v1/admin/users` (`{users}`), `POST /v1/admin/users {username, displayName, password, role}` (`201 User`), and `PATCH /v1/admin/users/{id} {role?, disabled?, password?}` (`User`). Admins manage member accounts; only Super Admins create or modify administrators. Role/password/disable changes revoke sessions and stop an affected child runtime. The last enabled Super Admin cannot be disabled or demoted. `GET /v1/admin/audit` returns the most recent thousand `{events: [{id, at, actorId, action, target, outcome}]}` without credentials or raw command output.

`GET /v1/admin/policy` returns the policy; only Super Admins may `PATCH` it with supported fields. The shape is `{allowHostCommands, allowCloudProvisioning, allowPublicPublishing, maxSessionsPerUser, maxConcurrentChecksPerUser, sessionMemoryMiB, sessionCPUMillis, checkMemoryMiB, checkCPUMillis, monthlyBudgetUsd, allowedHarnesses, backingMode, instanceName}`. Defaults allow guided cloud provisioning and explicit public publishing, disable host commands, and permit one concurrent agent session and one check per user. Each defaults to 1,024 MiB memory and 1,000 millicores. Memory bounds are 128ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Å“32,768 MiB; CPU bounds are 100ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Å“16,000 millicores. Existing policy files without those fields receive the safe defaults. These per-process limits do not guarantee that every user's combined workload fits the host.

Supported harness IDs are `claude-code`, `codex`, `opencode`, and `shell`. Members cannot provision backing infrastructure or change their assigned project root. Native host execution requires both Super Admin and the enabled host-command policy. Session limits are checked against actual running/starting/idle session records; check limits are enforced inside the execution service. The budget is an admission setting, not a provider billing cap. Changing a policy backing label does not itself provision or move a server.

The initial owner retains the original project folder and inline execution service. Additional users receive an owned child process with separate `state`, `projects`, `home`, Git identity, and provider configuration under `StateDir/control-plane/users/{id}/runtime`. Workers bind literal loopback on an ephemeral port, require a private random bearer secret, and receive an immutable user identity. Model/cloud tokens and other users' CLI profiles are not inherited. The gateway replaces actor headers with trusted context; public callers cannot select another user's worker. Windows workers use a hidden process and a kill-on-close Job Object; Unix workers use an owned process group. These are separate application workspaces on one host, not separate operating-system accounts. Docker remains the default code-execution boundary.

`GET /v1/admin/users/{id}/runtime` returns cached `{status, startedAt?, message}`. `POST` to the same path plus `/stop` stops that user's child; the next permitted request starts a new child with existing private state. Disable the account to prevent restart. The initial owner's inline runtime stops with Luci itself and its per-user stop route returns `409`. The native app has a separate owner-token lifecycle shutdown route; this is not an anonymous management API.

GET /v1/setup/status returns:

```json
{
  "product": "Luci", "version": "0.4.2", "platform": "windows", "configured": false,
  "access": {"user": {"username": "owner", "role": "super-admin"}, "manageBacking": true, "projectsDirManaged": false},
  "workspace": {"mode": "local", "connectionId": "", "host": "", "ready": true},
  "config": {"projectsDir": "absolute path", "mode": "local", "cloudProvider": "", "region": "", "domain": "", "monthlyBudgetUsd": 30},
  "checks": [{"id": "git", "label": "Git", "status": "unknown", "message": "Run the readiness check", "action": ""}],
  "cloudProviders": [{"id": "aws", "name": "AWS", "strategy": "Lightsail", "description": "One Linux machine, fixed bundle pricing"}, {"id": "azure", "name": "Azure", "strategy": "Virtual machine", "description": "One Linux machine, no Kubernetes"}]
}
```

Checks include git, docker, sandbox-image, aws and azure. Status is unknown, ready, missing or error. POST /v1/setup/check runs bounded readiness probes and returns the same status shape. Docker daemon and Luci sandbox image are checked separately. AWS/Azure are optional.

`access` describes the active execution account, derived only from the authenticated gateway or verified worker actor. It is omitted when no trusted actor exists. When a cloud workspace is selected, status/config/check responses carry the remote account's access, never the local desktop administrator's authority. `manageBacking` permits that account's administrator operations; `projectsDirManaged` independently means its project folder is assigned and cannot be changed, including for separate local Admin/Super Admin accounts. Local connection switching still uses the local account's permissions.

Non-owner managed workers start with `configured: true` because the control plane already prepared their private, fixed projects directory. They can immediately create projects and save briefs/designs without a first-use configuration write or infrastructure permission. This includes newly invited Members and additional local accounts. `configured` does not claim Docker, model credentials or cloud deployment readiness; those remain separate checks. Initial installation owners still choose their local/cloud/existing workspace normally. No completion endpoint or broader folder permissions are introduced.

POST /v1/setup/config accepts `{projectsDir, mode, cloudProvider, region, domain, monthlyBudgetUsd}` and returns setup/status. Mode is local or cloud; cloudProvider is empty, aws or azure. ProjectsDir must be absolute. Server stores configuration atomically. Local starts without a cloud account. Saving switches the scanner to the chosen folder immediately.

POST /v1/setup/cloud/plan accepts `{provider, name, region, domain, size, monthlyBudgetUsd, sshPublicKey}`. Provider is aws or azure; name is a lowercase slug; domain is a DNS hostname without a scheme or path; size is small or standard; sshPublicKey is a valid SSH public key. Returns:

```json
{"id": "cp-...", "provider": "aws", "name": "luci", "region": "ap-southeast-2", "domain": "luci.example.com", "resources": ["One Linux virtual machine", "One persistent disk", "A public address"], "estimate": {"currency": "USD", "monthly": null, "verified": false, "explanation": "A live provider quote is required"}, "steps": [{"id": "sign-in", "title": "Connect your account", "detail": "..."}], "files": [{"name": "template.json", "content": "...", "mediaType": "application/json"}], "commands": [{"label": "Preview", "executable": "aws", "args": ["..."]}], "warnings": [], "createdAt": "RFC3339"}
```

Plans are deployable template bundles, not successful deployment claims. No resources are created by planning. Quote failures remain explicit. POST /v1/setup/cloud/create accepts `{planId, acknowledgeCosts, acknowledgeUnverifiedPrice}`. Both acknowledgements are required when the quote is incomplete; verified estimates above the reviewed budget are refused. Creation records a job before issuing provider operations. GET /v1/setup/cloud/jobs returns cached `{jobs}`; POST /v1/setup/cloud/jobs/{jobId}/refresh explicitly checks the provider. POST /v1/setup/cloud/jobs/{jobId}/delete requires `{confirmName}` matching an owned resource, while POST /v1/setup/cloud/jobs/{jobId}/dns checks whether the planned domain resolves to the recorded public IP. Provider acceptance of a request is not readiness or proof of completed deletion.

POST /v1/setup/cloud/login accepts `{provider: "aws" | "azure"}` and returns an asynchronous `{id, provider, status, message, verificationUrl?, userCode?, actionUrl?, createdAt}` job. GET on the same route returns cached `{jobs}`. AWS uses its CLI's browser login; Azure uses device-code login. Only allowlisted sign-in URLs and user-facing device codes are exposed. Raw CLI output and credentials are not returned, and credentials remain with the provider CLI. Active duplicate requests share the existing job; login expires after ten minutes and is cancelled when Luci closes.

## Project journey

### Git delivery and project backups

`POST /v1/setup/github/check` runs bounded `gh` availability/account checks and returns `{available, signedIn, login?, message, actionUrl?}`. `POST /v1/setup/github/login` accepts `{}` and returns the existing asynchronous login job shape with `provider: "github"`. `GET` on that login route returns `{jobs}` for GitHub only. The owned CLI process uses GitHub's browser/device flow; raw output and tokens are never returned. Credentials remain with GitHub CLI on the current execution workspace. This operation does not copy an account to another machine.

`GET /v1/product/projects/{id}/git/delivery` returns `{origin?, branch, head, publication?, message?}` from local Git and cached delivery provenance. `publication` is `{repository, visibility?, status, branch?, commit?, verified, updatedAt}`. A missing or unsafe origin is omitted. This read does not contact or mutate a remote.

`POST .../{id}/git/remote` accepts `{url}` and attaches a credential-free HTTPS origin, returning a publication record. It refuses to replace an existing different origin. `POST .../{id}/git/github` accepts `{owner?, name, visibility?: "private" | "public", acknowledgePublic?: boolean}`. It defaults to private, requires `acknowledgePublic: true` for public, creates through `gh`, verifies repository identity/visibility, and attaches the origin. Success is `201` with the publication record. Creation does not push code. An ambiguous creation is persisted and blocks a blind retry; the user can check GitHub and attach the existing repository explicitly.

`POST .../{id}/git/push` accepts `{commit, branch?}`. It requires the current full HEAD, the current named branch, a clean worktree, and the saved brief in the reviewed commit. It pushes that immutable commit without force/mirror/tag side effects and verifies `ls-remote` returns the exact commit for the target branch. Success is `200` with `status: "pushed"` and `verified: true`. Remote operations are bounded to two minutes and return sanitized errors; the UI should show a busy state. Repository-local executable credential helpers and URL rewrite rules are not used; trusted host helpers and GitHub CLI remain supported.

`POST .../{id}/backups` downloads `application/zip` with an attachment filename. The manifest hashes every included entry. It carries a Git bundle, current tracked/non-ignored working files (including dirty and untracked files), and saved brief/design revision history. Known credential-like current files, ignored/generated files, Git configuration/hooks, host credentials and sessions are excluded. **Git history can still contain previously committed secrets; treat the archive as private.** It is not an application database or runtime-volume backup. Submodules and symbolic/special working files require separate handling and are refused.

`POST /v1/product/projects/restore?name=NEW_NAME` accepts raw `application/zip` and returns a new `{id, name, path}` project with status `201`. It validates inventory, SHA-256, ZIP CRC, paths, case-insensitive duplicates, entry types and limits before reserving a new folder. Limits are 512 MiB compressed, 1 GiB expanded, 20,000 entries, 64 MiB per working file and 8 MiB journey metadata. Existing folders are never overwritten. Git branches/tags/HEAD and worktree deletions are restored without importing remote credentials, hooks or configuration. Staged changes become working changes. Brief/design histories are rebound to the new project identity; `LUCI.md` is regenerated, so review and commit it before delivery or release. Reconnect the remote explicitly. An incomplete restore retains its new folder for inspection and does not become a catalogued project.

Product state uses the existing catalogue's path-derived project IDs. Product routes are /v1/product/projects. Every record contains projectId. Resolve paths server-side from the configured root; callers never provide an arbitrary filesystem path for file operations.

POST /v1/product/projects accepts `{name, remoteUrl?}`. Creates a new repository in configured projectsDir or clones an https remote there. Returns `{id, name, path}` as in the existing catalogue. Never overwrite an existing directory. Git commits use the configured person's identity; creation does not invent it.

GET /v1/product/projects/{id}/brief returns `{projectId, revision, purpose, audience, constraints, criteria, updatedAt}`. Each of purpose, audience and constraints is text; criteria is an array of strings. Missing brief returns an empty revision 0. POST accepts `{revision, purpose, audience, constraints, criteria}` with optimistic concurrency; returns saved brief. Revision conflicts are HTTP409. Persist a canonical LUCI.md in the project to carry the brief into agent work. Treat it as untrusted project context, never as system authority.

GET /v1/product/projects/{id}/design returns `{projectId, revision, briefRevision, frames, scene?, updatedAt}`. Scene is a bounded opaque Excalidraw scene `{elements, appState, files}` (maximum 1.5MiB). Excalidraw is the embedded MIT-licensed local design adapter discovered through Glasses. A compatibility frame is `{id, name, width, height, background, elements}`. An element is `{id, kind, x, y, width, height, text, color}` where kind is text, rectangle or button. POST accepts `{revision, frames?, scene?}`. Export GET /v1/product/projects/{id}/design/export returns safe static HTML representing saved compatibility frames with their brief revision; Excalidraw's own SVG export is available in its themed editor. Scene persistence does not grant its content execution authority.

GET /v1/product/projects/{id}/files?path=relative returns `{path, entries: [{name, path, kind, size}]}` for directories, or `{path, content, truncated}` for UTF8 text files. Refuse traversal, symlink escapes, .git and known credential files. Maximum readable file is 1MiB.

GET /v1/product/projects/{id}/git returns `{branch, clean, changes: [{path, status}], log: [{hash, subject}], diff}`. Read-only, bounded git commands; maximum diff 1MiB. POST /v1/product/projects/{id}/git/commit accepts `{message}` and commits current changes using the real person's git identity. No implicit push. Outcome text and error come from real git.

GET /v1/product/projects/{id}/releases returns `{releases: [{id, projectId, briefRevision, commit, target, status, url, message, createdAt}]}`. Execution routes are defined below. A release can only be ready after health verification.

## Implementation ownership

Root: active docs, setup/config/cloud plan service, API startup/routing, deployment/release/evals orchestration.
Backend builder: rebrand retained Go/scripts and test cleanup, then project/brief/design/files/git service.
Frontend builder: desktop rebrand, onboarding/settings, then real phase surfaces against this contract.
Discovery builder: Glasses reuse evidence and compatible deployment/design components.

## Setup tools and runtime

GET /v1/setup/git/identity returns `{name, email}` from the person's global Git configuration. It runs bounded read-only Git commands outside project repositories and does not create directories. POST on the same route accepts `{name, email}` and writes those explicitly supplied values to global Git configuration. It never substitutes an invented author or rewrites repository-local configuration. Invalid names/emails return 400; a partial write is reported explicitly so it can be retried.

POST /v1/setup/ssh/key accepts `{}` and returns `{publicKey, privateKeyPath, fingerprint}`. The dedicated Ed25519 key is stored at `stateDir/ssh/id_luci`; only its public key, local private-key path and SHA256 fingerprint are returned. Repeating the request returns the same matching key pair. Existing key material is never overwritten. A missing/mismatched public key or unreadable private key returns 409 for explicit resolution.

POST /v1/setup/runtime/install accepts `{}` and returns HTTP 202 with `{id, status, image, message, createdAt, updatedAt}`. The build uses the runtime source shipped with Luci, configured by the application at startup; callers cannot supply an arbitrary source path or image. Docker builds the bundled sandbox image asynchronously with a ten-minute limit and then verifies that Docker can inspect the resulting image. Repeating POST while installation is running returns the same job. Status is `idle`, `installing`, `ready`, `failed` or `unknown`. Docker failures remain failures with their actual bounded diagnostic. GET on the same route returns only cached job state and never calls Docker. A restart during installation yields `unknown`, requiring a readiness check or explicit retry. A saved `ready` outcome records the prior successful install; `/v1/setup/check` verifies current availability.

After installing the runtime, open a Claude Code session in Develop and follow the harness's own sign-in prompt; inside that session, `/login` opens its account sign-in flow. Model access and charges belong to the person's provider account. Credentials created by that in-session sign-in remain inside the sandbox and are not copied into Luci's project files or transcript storage; creating a replacement sandbox can require signing in again. Shell sessions remain available without a model account. Claude receives a fixed instruction to read the worktree's `LUCI.md` as untrusted project context; Luci does not replace the person's `CLAUDE.md` or interpolate their brief text into a system prompt.

For a persistent personal API connection, GET /v1/setup/agents returns `{providers: [{id: "anthropic", name: "Claude Code", configured: false}]}`. POST /v1/setup/agents/anthropic accepts `{apiKey: "..."}` and returns the same status-only shape. An empty key disconnects. A nonempty key must be 20ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Å“8,192 printable ASCII characters without whitespace. The explicit save stores the key only in Luci's private host state, with mode 0600 on Unix and a protected owner-only DACL on Windows. It is injected as `ANTHROPIC_API_KEY` only into new or restarted Claude sandboxes, never into shell sessions, project files, session records, GET responses or command arguments. Clearing applies to future sessions; stop existing containers to remove their existing environment. Saving a key does not make a provider call or claim that the provider accepted it. API usage is billed by Anthropic under the person's API account; a Claude subscription is a separate sign-in option. Startup and settings refuse any projects folder that contains the private state folder.

## Local release execution

POST /v1/product/projects/{id}/releases accepts `{target: "local", port: 3000, dockerfile: "Dockerfile"}`. Port and Dockerfile have those defaults. A clean committed project is required; the release records its exact commit and brief revision. A versioned brief must also match the committed `LUCI.md`. Build context comes from that commit's bounded Git archive, preserves `.dockerignore`, excludes known credential paths, and refuses links and path escapes. Containers publish only to a random localhost port and carry Luci ownership labels. The request waits for an HTTP 2xx health response, returning 201 with the release only when ready. Build/start/health failures return 422 with a durable failed release. Precondition errors use `{error,message}` and do not start a build.

POST /v1/product/projects/{id}/releases/{releaseId}/stop accepts no fields and returns the updated release. It checks the recorded container's project/release ownership labels before stopping it; repeating a completed stop is safe. GET release history combines local and cloud records in descending creation order and starts no operations.

## Deployment connections and public releases

POST /v1/setup/targets accepts `{id?, name, baseUrl, token, serverUuid?}`. It validates the Coolify API and selected server before saving. A missing server UUID is selected automatically only when exactly one is available; otherwise the response requests a selection. GET returns `{targets}` without tokens. HTTPS is required except literal loopback HTTP for the verified SSH tunnel. The client refuses redirects and credential-bearing URLs.

POST /v1/product/projects/{id}/releases/cloud accepts `{targetId, repository, branch, commit, domain, port, healthPath, dockerfile}`. The repository must be reachable through HTTPS by Coolify; this is not an upload of local working files. The exact full commit must match the clean project's HEAD, its Dockerfile must be committed, and the brief revision must match committed `LUCI.md`. The service persists provider IDs outside the repository and never puts the target token in a release. Repeated matching requests reuse the recorded operation; ambiguous provider failures require inspection instead of an automatic second create.

POST /v1/product/projects/{id}/releases/{releaseId}/refresh polls the recorded provider deployment and, after completion, performs a real HTTPS check of the validated public domain and health path. Redirects and private/reserved destinations are refused. Provider completion remains `verifying` until the expected commit and HTTP 2xx health check are confirmed. Subsequent commits reuse the saved application; ready historical releases retain their original verification result.

## Cloud workspaces

The desktop control process stays local. An active cloud workspace places projects, Git worktrees, agent sessions, files and application previews on the selected SSH machine. Provider account sign-in, cloud resource creation and the SSH connection controls remain local. Setting `config.mode` alone does not activate a remote runtime. `workspace` in setup status identifies the active execution location; an unavailable cloud connection does not silently fall back to local project state.

POST /v1/setup/ssh/inspect accepts `{host, port, user, privateKeyPath}` and returns observed fingerprints and a scan ID. A fingerprint must be compared with a trusted source. POST /v1/setup/ssh/connect accepts `{scanId, acceptedFingerprint}` and opens loopback tunnels using that exact accepted host key and strict checking. GET /v1/setup/ssh/connections returns cached connections. POST /v1/setup/ssh/connections/{connectionId}/stop closes the selected tunnel.

POST /v1/setup/ssh/connections/{connectionId}/install-luci accepts `{acknowledgeInstall: true}`. It transfers this build's bundled Linux runtime by SCP over verified SSH, verifies its SHA-256 on the selected server, installs a versioned runtime and systemd service, and checks that a matching Luci version responds through the tunnel. It requires an existing connection, bundled runtime and appropriate remote sudo rights; it does not create a cloud machine. GET /v1/setup/workspace/install returns cached `{jobs}`. An installation accepted or transferred is not yet a ready workspace.

POST /v1/setup/workspace/activate accepts `{connectionId}` and requires a responding matching-version remote Luci service. Selection and the accepted SSH host key persist outside project repositories. Startup attempts reconnection using the retained host key; a changed key is not silently accepted. POST /v1/setup/workspace/local selects the local workspace. Neither selection copies projects or deletes the previous workspace's data. Switching back locally does not stop cloud billing.

While a cloud workspace is active, project/session APIs, Git identity, model API settings, runtime installation and deployment targets use the remote service. Configure the agent account and install the sandbox runtime for that active location explicitly. Local provider CLI credentials and local model keys are not implicitly copied to the server. A local release means a Docker preview local to the active execution machine; public deployment remains the explicit Coolify release flow.

The internal preview route supports GET (including HEAD), POST, PUT, PATCH, DELETE and OPTIONS at /v1/product/projects/{id}/releases/{releaseId}/preview/{path...}. It forwards only a recorded `target=local`, `status=ready` release with a literal `http://127.0.0.1:<port>` origin. It preserves application paths, query, body, app cookies and WebSocket traffic, while stripping control authorization and bridge headers. It never accepts an arbitrary upstream URL. Calls require `X-Luci-Preview-Bridge: 1` and no browser fetch metadata; this marker separates server bridge traffic and is not an authentication secret. Browser previews use a separate loopback origin per release, supporting root-relative application URLs without giving application JavaScript access to Luci's control origin.

POST /v1/setup/workspace/preview accepts `{projectId, releaseId}` for an active cloud workspace. It first checks the remote release history for that project's ready local preview, then creates or reuses its separate local bridge and returns `{url}`. This allocation is an explicit POST. Remote local-preview URLs are hidden from ordinary release history until the bridge exists; unmodified remote loopback URLs must never be opened on the desktop. Root-relative assets, application API calls and WebSockets all use the fixed release behind that origin. Returning to local mode or closing the connection closes owned bridges; it does not stop the remote application container.

## Additional agent connections

GET `/v1/setup/agents` now returns `anthropic`, `openai`, and `local` provider rows, with `{id,name,configured,authMode,model?,baseUrl?,signInCommand?}`. Presence means saved configuration, not successful provider acceptance. POST `/v1/setup/agents/openai` accepts `{apiKey}` with the same private-storage validation as Anthropic. POST `/v1/setup/agents/local` accepts `{baseUrl,model,apiKey?}`; an empty connection disconnects. HTTP endpoints must be loopback or private IP addresses; other endpoints require HTTPS. Embedded credentials, queries, fragments, control characters and incomplete model configurations are rejected. GET never includes API keys.

POST `/v1/sessions` accepts optional `authOnly:true` with `harness:"codex"`, a worktree ID, and no resume or opening prompt. Attach to complete the official device sign-in flow; restart that session to start normal Codex with retained auth and clean context. API-key sessions receive a documented environment-backed Codex provider configuration. OpenCode requires an explicit saved endpoint/model before starting; it enables only that provider and uses the same model for smaller tasks. Loopback model endpoints are translated to the Docker host's `host.docker.internal` alias. That host must actually expose the model service to its Docker network.

GET `/v1/setup/agents/codex/account` returns `{configured,authMode:"chatgpt",sourceSessionId?,activeSessionId?,message}`. POST accepts only `{sessionId}` and explicitly retains the native credential from this user's stopped Codex device-sign-in session. It accepts neither credential values nor host paths. DELETE disconnects future account use while existing sessions keep their private credential copies. Saved presence is not a successful account/model test. New Codex sessions prefer the saved subscription, receive distinct private homes, and record `authMode:"chatgpt"` so restart/resume cannot substitute an API key. One subscription session is active at a time; competing use returns 409 `codex_account_busy`. Status and ordinary session metadata expose `authOnly` and `authMode`, never native credential contents.

Disconnected or replaced connections cannot be resurrected through restart or fork of a previously bound session: reconnect explicitly. Account-status GET is read-only; an exited lease remains private for credential reconciliation and is not reported as active. Saving the already-connected sign-in is idempotent and cannot overwrite refreshed credentials with its old native cache.

Codex/OpenCode resume requires a stopped source, returning 409 while it runs or changes. A bounded private state snapshot is mounted in a new sandbox and forked with native CLI flags; the source is unchanged. Restart retains only the native credential file and keeps earlier histories available for resume. Forget removes the recorded state without removing independent forks. See [provider scope and evidence](PROVIDERS.md).

## Private cloud source and maintenance

POST `/v1/product/projects/{id}/releases/deploy-key` accepts `{targetId,repositoryHTTPSGithub}`. It uses that workspace's signed-in GitHub CLI account, which needs repository administration permission, to add a read-only deploy key. Luci generates an Ed25519 key, registers its private half with the selected Coolify target, and uploads only its public half to GitHub. The response includes `{status,privateKeyUuid?,publicKey,message}` plus operation identity. No private key is returned, logged, or retained in local project/state files. A durable public operation record precedes remote writes. Repeats reconcile the exact public key and operation name rather than generating duplicate keys after ambiguous failures. If the original private-key upload was interrupted and no matching remote key exists, manual resolution or an existing Coolify key UUID is required. Remote key removal is detected on a subsequent explicit request.

POST `.../releases/cloud` additionally accepts `{source:"public"|"private",privateKeyUuid?}`; omitted source retains public behavior. Private source currently supports HTTPS GitHub provenance, translated to `git@github.com:owner/repository.git` only for Coolify's deploy-key API. An existing manually configured Coolify key UUID is accepted. Source/key changes cannot silently repurpose an existing application. The published commit must still match clean local HEAD and the committed brief.

POST `.../releases/{releaseId}/stop` dispatches to the owned local preview or cloud application. A cloud stop persists intent, submits `docker_cleanup=false`, and returns 202 with `status:"stopping"`. Repeats do not submit another stop. Refresh observes the application's actual stopped/exited status before recording `stopped`. No volumes or networks are pruned. A historical release cannot stop a newer release's application; pending/uncertain deployments must be resolved first.

POST `.../releases/{releaseId}/rollback` accepts `{}` and creates a new release from that historical release's exact commit and brief revision, preserving its owned target, application, repository access and domain. Current local HEAD is intentionally irrelevant to this explicit historical rebuild. It returns 202 queued, with `rollbackOf` identifying the source. Refresh must still confirm the requested provider commit and successful public HTTPS health; a queued rebuild is never marked ready. Repeated requests reconcile an in-flight/latest rebuild rather than silently creating another. Changed application settings or uncertain deployment state return 409 for resolution. These operations rebuild application code; they do not reverse database migrations or restore application data.

## Task plans and check outcomes

GET `/v1/product/projects/{id}/plan` returns `{projectId,revision,briefRevision,tasks:[{id,title,status}],updatedAt}`. Task status is `todo`, `doing` or `done`. POST accepts `{expectedRevision,tasks}` and returns the saved plan; stale revisions return409. Plans contain at most200 tasks. Saving publishes `.luci/plan.json` alongside canonical state and `.luci/design.json` for agent context. Manual task status is not execution evidence.

GET `.../checks` returns `{runs,suggested}`. POST accepts `{commands:[{label,executable,args}],execution:"docker"|"host",acknowledgeHostExecution?}` and returns202. Docker is the default; host mode requires Super Admin, enabled policy and an explicit acknowledgement. Commands are bounded typed argv, not interpolated shell text. Runs have a ten-minute limit, at most eight commands, bounded output and retained history. GET `.../checks/{runId}` reads a run; POST `.../checks/{runId}/cancel {}` requests cancellation.

Each run records `{id,projectId,commit,dirty,execution,memoryMiB,cpuMillis,sourceDigest,sourceChanged,status,commands,createdAt,updatedAt,message}`. Command rows add `{status,exitCode?,output,truncated}`. Policy resources are clamped to operator ceilings. Only an unchanged source snapshot with successful commands can pass; changed/unverifiable source yields `unknown`. Restarted in-flight runs become `unknown`. Checks do not automatically mark tasks complete or claim acceptance criteria are satisfied.

## Cloud backing identity

GET `/v1/setup/workspace/auth?connectionId=...` returns `{authenticated,user?,bootstrapRequired,bootstrapTokenAvailable,message}` for an existing verified SSH connection. POST accepts `{connectionId,username,password,bootstrapToken?,displayName?}` to sign in or create the first remote Super Admin. If bootstrap is required and no code is supplied, Luci reads only the fixed installer-owned bootstrap file over verified SSH. It refuses a different product/version. DELETE with the connection query revokes the saved remote session.

Responses never contain the remote bearer token. The private cache is bound to SSH host, user and accepted host key, so a changed fingerprint cannot reuse it. Activation requires authenticated remote readiness. Project forwarding replaces local credentials with that remote session; a remote session socket receives a new single-use remote ticket. Preview bridges retain application cookies/query/paths while stripping control credentials before reaching the application. Missing/revoked backing authentication returns `409 backing_sign_in_required`, leaving the independent local login and Settings available for reconnection. Local session expiry still returns401. There is no local workspace fallback.

## Selected backing administration

The Admin screen explicitly selects this machine or the active cloud backing. Cloud management uses `/v1/setup/workspace/admin`: GET `/me`, GET/POST `/users`, PATCH `/users/{id}`, GET/PATCH `/policy`, GET `/audit`, GET `/users/{id}/runtime`, and POST `/users/{id}/runtime/stop`. Shapes/status match the management routes above. The local caller must be Admin or Super Admin; the backing independently enforces its own account permissions. The screen obtains the backing identity before requesting privileged data and shows that account and role.

The bridge uses only the selected, connected, pinned SSH backing and its private bearer. Callers cannot override the target, path or credentials. Bodies are bounded to1MiB; requests carry no local actor/policy/cookie headers. Public response DTOs omit credentials. Remote401 becomes recoverable409, transport failure503; neither causes local fallback. Switching the management surface clears drafts and prevents late responses from changing the new selection.

## Private application runtime configuration

GET `/v1/product/projects/{id}/configuration` returns `{revision,variables:[{name}],updatedAt}`. It never returns values. POST accepts `{expectedRevision,updates:[{name,value?,remove?}]}` and returns the new metadata; stale revisions return409. Names follow environment-variable syntax. At most100 variables are stored, values are bounded to16KiB and the whole configuration to256KiB; NUL and line breaks are rejected. An explicit removal deletes the stored value.

Values remain in owner-protected state outside the project. They are excluded from project files, Git, design/brief context and project backup. Local Docker releases capture an immutable configuration snapshot and pass an owner-only temporary env-file by pathname, removing it after startup. The build receives no runtime secrets. Release metadata records `configurationRevision` without values. Cloud deployment uses Coolify's runtime environment API before submission and reconciles only the keys Luci manages; unrelated provider/manual configuration is retained. A changed configuration creates a new deployment even at the same Git commit.

Rebuilding historical code uses the current runtime configuration. It does not restore old secrets, database migrations or application data. A project restored from backup needs its runtime values re-entered.

## Account entry, local recovery and invitations

The public control-plane status describes whether a workspace needs its first
account. It never enumerates users or exposes local account authority. Successful
first-account creation changes the entry state to sign-in, including after
sign-out in the same application session. Obsolete browser sessions do not block
the launcher's one-time first-setup token.

The native launcher creates a per-process `LUCI_LOCAL_ACCESS_TOKEN` for its owned
loopback sidecar. It remains in Rust and the service; it is not returned to the
WebView, written to state, or inherited by managed workers. External `LUCI_API`
connections have no such authority. The following routes require both loopback
and `X-Luci-Local-Access-Token`; a loopback address alone grants nothing:

- GET `/v1/auth/local-owner` returns `{owner: User | null}`.
- POST `/v1/auth/local-account` accepts `{username,displayName,password}` and
  creates a separate local administrator account with its own execution workspace.
  It preserves the original owner and all saved users, projects and policy.
- POST `/v1/auth/local-owner/recover` accepts `{newPassword}` and restores only
  the original installation owner's access. It preserves that account's ID and
  saved work, ends its prior sessions/connections and records an audit event.

The desktop invokes these through `local_access_info`, `create_local_account` and
`recover_local_owner`. Account creation and recovery return `{token,user}` through
IPC, without disclosing the local capability. Native calls require an owned live
child and refuse external addresses. Credential mutations are persisted before
success and roll back on a failed save.

The portable launchers explicitly enable `LUCI_PORTABLE_OWNER_ACCESS=1` for their
loopback service. This mode generates a separate rotating proof in the private
`control-plane/owner-access-token` file. The launcher verifies that proof against
the service before opening the browser with an `#owner=...` fragment. The page
immediately removes the fragment and uses the proof only for same-origin local
account routes; it never includes it in general API headers. The service removes
its own proof on exit. Native authority remains exclusively in Rust, and neither
mode is available on public bindings or managed workers. Reopening the portable
launcher restores this computer's account controls after a service restart.

Administrators can issue an invitation using POST `/v1/admin/invitations` with
`{role}`. The result is `{invitation,inviteCode}`; GET returns
`{invitations: [...]}` without codes, and DELETE `/v1/admin/invitations/{id}`
revokes a code. Metadata includes `id`, `role`, `createdBy`, `createdAt`,
`expiresAt`, `revoked` and optional `acceptedAt`/`acceptedBy`. Invitations expire
after 48 hours, are stored hashed and can be accepted only once. An ordinary
Admin can invite members; only a Super Admin can invite an administrator.

POST `/v1/auth/invitations/accept` accepts
`{inviteCode,username,displayName,password}` and returns `{token,user}`. Role comes
from the invitation; callers cannot choose one. Disabled or demoted issuers,
revoked/expired codes and reused codes are rejected. Creating an account does not
grant access to another account's projects.

The selected-workspace management bridge supports these invitation routes with
the selected backing's identity. Existing SSH workspace sign-in accepts an
optional `inviteCode` and redeems it on that workspace. A remote account error
remains a recoverable workspace error and does not expire the local login.

## Public static deployment

POST `/v1/product/projects/{id}/releases/pages` accepts `{commit,branch,acknowledgePublic:true}`. The repository comes only from the project's verified GitHub origin, and the published directory is fixed to `/docs`. The project must have a clean current HEAD matching the requested full commit and current/default branch, a matching committed brief, Dockerfile, `docs/index.html` and `docs/.nojekyll`. The GitHub repository must be public and its remote branch must match the requested commit. Public-publication policy and the explicit acknowledgement apply. This route uses the signed-in GitHub CLI; it accepts no provider token or arbitrary command/path.

The durable release has `target:"github-pages"`. Its provider record is outside project Git and contains repository/source configuration, commit, brief revision, index digest, provider build and verification time. `ready` requires the recorded provider commit, a public HTTPS response and matching committed index bytes. Queued/building/verifying outcomes are not readiness. Common history and refresh routes include Pages releases; stop unpublishes only unchanged configuration owned by that release. Pages rollback is not supported. Later default-branch pushes can republish `/docs` automatically; an older ready history entry describes its original verification and is not continuous site monitoring. Pages hosts static public output and receives no private runtime values.
