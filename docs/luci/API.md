# Luci application contract

[PRODUCT-CONTRACT.md](PRODUCT-CONTRACT.md) controls product scope and the active owner audit gate. This file documents implemented routes; their existence does not establish the required per-person conversational coordinator, autonomous team reconciliation or cloud microVM execution. CLOUD-085 policy drafts in the working tree are uncommitted/unverified, not a supported API. New implementation remains paused until the local audit explicitly lifts the gate.

Local Develop uses isolated Docker and existing upstream harnesses. Cloud Develop requires a real supported microVM backend (CLOUD-084, missing). Existing `backing` labels (`docker`, `aws`, `azure`), SSH workspace selection and Coolify release routes identify management/compatibility or app-deployment locations, not microVM capability. Runtime image installation on an SSH host installs the existing container runtime only. Keep **Run on** and **Deploy to** distinct. Future coordinator automation must call current typed operations under agreed authority/review/cost/deployment policy; optional export/return does not replace that primary path.

Version 0.5, 6 October 2026. JSON is camelCase. Routes below are additions to the retained session API at /v1. Errors are `{error: string, message: string}`. Requests reject unknown JSON fields. A control API read never starts a session/provisioning job or creates a resource. Bounded read-only Git subprocesses are allowed. Application preview requests are forwarded to the selected application's own HTTP surface.


Current cross-phase contracts are documented separately in [project understanding and reviewed amendments](ORCHESTRATION.md) and [durable project runs](PROJECT-RUN-CONTRACT.md). Team, billing and hosting contracts are in [team service operations](TEAM-SERVICE-OPERATIONS.md), [Stripe billing](STRIPE-BILLING.md) and [public HTTPS ingress](PUBLIC-HTTPS-INGRESS.md).

CLOUD-088 adds required elastic cloud task agents: each receives a separate newly allocated isolated microVM with exact durable task/attempt ownership and bounded provider/resource/spend admission. Current Docker session/scheduler endpoints are reuse foundations, not an implemented cloud swarm API. Per-person Lucis, team peers and task agents have distinct identities.

## Setup

### Identity and management

The control-plane gateway authenticates the API before it reaches a user's execution service. `GET /v1/control-plane/status` is public and returns `{product: "Luci", version, configured, instanceName, backing: "docker" | "aws" | "azure", bootstrapRequired, bootstrapTokenPath?}`. First setup reads the owner-only `StateDir/control-plane/bootstrap-token` file. `POST /v1/auth/bootstrap` accepts `{bootstrapToken, username, displayName, password}` and returns `201 {token, user}` for the first Super Admin. The bootstrap file is removed after success. `POST /v1/auth/login` accepts `{username, password}` and returns `{token, user}`. Passwords require 12–1,024 characters and use the Go project's Argon2id implementation with random salts; derivation concurrency and login attempts are bounded.

API requests use `Authorization: Bearer TOKEN`. Sessions expire after twelve hours; only SHA-256 token hashes are persisted. `GET /v1/auth/me` returns `{user, permissions, policy}`. User fields are `{id, username, displayName, role: "member" | "admin" | "super-admin", disabled}`. Permissions are `{manageUsers, manageBacking, managePolicy, inspectAudit, runHostCommands}`. `POST /v1/auth/logout {}` revokes the current session. `POST /v1/auth/password {currentPassword, newPassword}` revokes existing sessions and returns `{signInRequired: true}`.

`POST /v1/auth/socket-ticket {path}` accepts a canonical session attach path and returns `{ticket, expiresAt}`. A ticket is bound to its user, bearer session and exact path, expires in at most thirty seconds, and is consumed once by the WebSocket request's `?ticket=` parameter. It is removed before forwarding. Already-upgraded sockets close when their session expires or is revoked, including the initial owner's inline terminal. Application preview query parameters named `ticket` remain application data; preview bridges authenticate separately with a bearer token and preserve their isolated application's cookies.

Session responses may include `executionGeneration`, an opaque 64-character hexadecimal allocation fingerprint. Compare it only for equality within the same session/project/worktree/attach scope; it is not an ordered revision, credential or authorization. A restart changes the fingerprint even though the session ID stays the same. Legacy records and terminal records whose private allocation token has been erased may omit it; omission must not clear an already observed terminal Exit. The fingerprint exposes no raw attach token, sandbox identity or endpoint. Existing read permissions still govern the whole session response.

Session responses may also include `attachReady`. `true` means this known nonterminal allocation has a recorded endpoint; it does not assert health or authorization. `false` means the responding API is currently preparing this exact execution generation without a recorded endpoint. The client displays “preparing” and waits without consuming its connection retry budget. Omission means readiness is unknown, including a reopened controller or legacy identity, and preserves the existing backend-based attach checks. Readiness never grants access, replaces a Restart acknowledgement, or clears terminal-generation fences.

The terminal relay drains final output and preserves an upstream normal WebSocket close before canceling its sibling read. A typed upstream binary Exit ends that allocation's terminal display without an automatic reconnect and asynchronously records the same exact private allocation as exited. The next status poll exposes Restart without waiting for the periodic reaper. The observation rejects replaced generations and cleared tokens, retains allocation identity and execution reservations, and does not delay socket close while persisting. It does not establish successful work, verified output, cleanup or integration. A distinct current allocation fingerprint permits attachment after restart; missing fingerprints on older servers require an explicit reconnect when the session is still live. Overload and abnormal disconnects remain distinct from a clean terminal close.

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

### Full product definition and design handoff

`GET /v1/product/projects/{id}/specification` returns the version-1 structured definition: revision, briefRevision, startingPoint (`new`/`existing`), purpose/audience/constraints, exclusions, stable requirements (`id,title,priority,acceptance`), journeys linked to requirement IDs, roles/access, entities/privacy, integrations, glossary, decisions, questions, status, digest and timestamp. Legacy briefs derive an unsaved definition in memory; reads never create files. `POST` accepts `{expectedRevision,expectedBriefRevision,specification}` with optimistic concurrency, persists an immutable revision, updates the compatible brief and exports full agent context to `.luci/specification.md` and `.json`. Client status/digest cannot grant approval. `POST .../specification/approve {revision,briefRevision}` records a digest-bound approved snapshot only with a purpose, audience, measurable requirements and answered questions. History is `GET .../specification/history`. Later brief changes make approval outdated until reconciled and saved.

`GET /v1/product/projects/{id}/design-plan` returns the revisioned screen/state handoff, linked definition/brief/sketch revisions, responsive widths, requirement links, tokens, asset provenance, implementation notes and an optional verified prototype release/commit. `POST` accepts `{expectedRevision,expectedBriefRevision,expectedSpecificationRevision,plan}`. `POST .../design-plan/approve {revision}` binds the current approved definition and sketch after checking screen purpose, states and responsive targets. History is `GET .../design-plan/history`. Saving the sketch does not approve this handoff. Exports are `.luci/design-handoff.md` and `.json`. Subsequent definition/sketch edits make approval outdated. Restore rebinds the new project identity, removes unavailable prototype references and requires fresh approval.

`GET /v1/product/projects/{id}/prototype-releases` returns `{releases: Release[]}` for Design. It combines the project's verified local previews, cloud and GitHub Pages releases with its owned running native Docker applications. Only ready project-owned records with valid provenance are admitted; foreign, malformed or ambiguous history is refused. Native application rows have `target:"application"`, a blank `url` and `browserConnectionRequired:true`. Opening first reads the existing platform project DTO and requires a nonbusy matching project and exactly one ready Docker deployment with the same release ID, commit and configuration revision. Local applications reuse that deployment's existing isolated gateway URL. Only a deployment marked as requiring a cloud browser connection uses `POST /v1/setup/workspace/platform/browser {projectId,kind:"application",resourceId}`, with the returned resource binding and separate origin checked. Selected SSH workspaces use the same catalogue and their explicit browser bridge. Saving a design resolves its prototype against this catalogue and records the backend release's exact commit; caller-supplied `prototypeCommit` is not trusted. This catalogue does not add native application rows to the separate preview/cloud deployment history or change its stop/rollback routing. Cloud/Pages ready history retains its original verification and does not continuously monitor the public site; native application eligibility includes a current owned-container health check.

`POST /v1/setup/agents/codex/sign-in {}` starts an official auth-only session in a private setup workspace, independent of project/worktree selection. It returns the ordinary Session DTO for terminal attach; saving the completed account connection still uses the existing explicit account endpoint. `POST /v1/product/projects/{id}/assist {expectedRevision,message,harness?,model?,agentConnectionId?}` starts a durable structured-assistance job through the selected connection described below. Omitted harness defaults to Codex. GET of `.../assist/{jobId}` reads cached progress/result; POST of `.../cancel` cancels the owned run. Results contain one question/options/rationale and an optional proposed definition. They never save or approve project content. A named subscription connection never falls back to an API account. No host-profile credential import is used.

### Git delivery and project backups

`POST /v1/setup/github/check` runs bounded `gh` availability/account checks and returns `{available, signedIn, login?, message, actionUrl?}`. `POST /v1/setup/github/login` accepts `{}` and returns the existing asynchronous login job shape with `provider: "github"`. `GET` on that login route returns `{jobs}` for GitHub only. The owned CLI process uses GitHub's browser/device flow; raw output and tokens are never returned. Credentials remain with GitHub CLI on the current execution workspace. This operation does not copy an account to another machine.

`GET /v1/product/projects/{id}/git/delivery` returns `{origin?, branch, head, publication?, message?}` from local Git and cached delivery provenance. `publication` is `{repository, visibility?, status, branch?, commit?, verified, updatedAt}`. A missing or unsafe origin is omitted. This read does not contact or mutate a remote.

`POST .../{id}/git/remote` accepts `{url}` and attaches a credential-free HTTPS origin, returning a publication record. It refuses to replace an existing different origin. `POST .../{id}/git/github` accepts `{owner?, name, visibility?: "private" | "public", acknowledgePublic?: boolean}`. It defaults to private, requires `acknowledgePublic: true` for public, creates through `gh`, verifies repository identity/visibility, and attaches the origin. Success is `201` with the publication record. Creation does not push code. An ambiguous creation is persisted and blocks a blind retry; the user can check GitHub and attach the existing repository explicitly.

`POST .../{id}/git/push` accepts `{commit, branch?}`. It requires the current full HEAD, the current named branch, a clean worktree, and the saved brief in the reviewed commit. It pushes that immutable commit without force/mirror/tag side effects and verifies `ls-remote` returns the exact commit for the target branch. Success is `200` with `status: "pushed"` and `verified: true`. Remote operations are bounded to two minutes and return sanitized errors; the UI should show a busy state. Repository-local executable credential helpers and URL rewrite rules are not used; trusted host helpers and GitHub CLI remain supported.

`POST .../{id}/backups` downloads `application/zip` with an attachment filename. The manifest hashes every included entry. It carries a Git bundle, current tracked/non-ignored working files (including dirty and untracked files), and saved brief/design revision history. Known credential-like current files, ignored/generated files, Git configuration/hooks, host credentials and sessions are excluded. **Git history can still contain previously committed secrets; treat the archive as private.** It is not an application database or runtime-volume backup. Submodules and symbolic/special working files require separate handling and are refused.

`POST /v1/product/projects/restore?name=NEW_NAME` accepts raw `application/zip` and returns a new `{id, name, path}` project with status `201`. It validates inventory, SHA-256, ZIP CRC, paths, case-insensitive duplicates, entry types and limits before reserving a new folder. Limits are 512 MiB compressed, 1 GiB expanded, 20,000 entries, 64 MiB per working file and 8 MiB journey metadata. Existing folders are never overwritten. Git branches/tags/HEAD and worktree deletions are restored without importing remote credentials, hooks or configuration. Staged changes become working changes. Brief/design histories are rebound to the new project identity; `LUCI.md` is regenerated, so review and commit it before delivery or release. Reconnect the remote explicitly. An incomplete restore retains its new folder for inspection and does not become a catalogued project.

Product state uses the existing catalogue's path-derived project IDs. Product routes are /v1/product/projects. Every record contains projectId. Resolve paths server-side from the configured root; callers never provide an arbitrary filesystem path for file operations.

POST /v1/product/projects accepts `{name, remoteUrl?, importAttemptId?}`. Creates a new repository in configured projectsDir or imports an HTTPS remote. Returns `{id, name, path}` as in the existing catalogue (`201`, or `200` for an already completed matching import attempt). Never overwrites an existing file, empty directory, project or symlink. Git commits use the configured person's identity; creation does not invent it.

For an import, the browser saves an `importAttemptId` (`imp-` followed by 24 lowercase hexadecimal characters) before sending the request. It is scoped to the signed-in account, selected API/team/workspace and projects folder. Repository URLs are limited to 2,048 characters and must not contain embedded credentials, queries or fragments. The private server receipt binds the exact actor, projects folder and its native identity, name and remote URL. Repeating an attempt never starts Git again: it returns the retained project or the existing failed/running/interrupted/unconfirmed outcome. A different request under the same identifier returns `409 import_conflict`. Older clients may omit the identifier; Luci generates one, included in any retained failure response.

Cloning remains synchronous with a two-minute request budget. Git works in a nested same-filesystem staging directory, `<projects>/.li/<attempt suffix>/r`, whose immediate container has no Git directory. Partial imports therefore appear in neither project catalogue and do not reserve the final name after failure. Luci checks the configured root and original directory identities, then uses a native no-replace directory move to publish the completed repository. Unsupported safe-move filesystems fail closed. Concurrent imports cannot replace one another or a folder created while Git was running. No failed stage is recursively deleted, including when its path has been replaced. Partial files remain available for deliberate inspection; this API does not automatically prune them.

`GET /v1/product/project-imports/{attemptId}` returns `{id,name,status,message,retainedPath?,project?}` only to the attempt's authenticated actor in the same workspace. `status` is `running`, `failed`, `complete`, `interrupted` or `unconfirmed`; `retainedPath` is relative to the original projects folder and `project` is present only for a verified completed result. A lost response after publication can be resolved from the private publication intent and matching native project/Git directory identities, including after restart. A replaced folder is never adopted by matching its name or remote. GET performs no Git commands, retries, promotion, reconciliation writes or resource allocation. Unknown or foreign attempts return `404`; this is not proof that a delayed original request was never received.

An admitted Git failure returns `422 {error:"git_failed",message,attempt}` with the same public attempt shape. Messages give bounded network/URL/credential or cancellation guidance without exposing Git stderr or credential-helper output. After a confirmed failure or pre-publication interruption, the person can edit the import details and retry with a fresh identifier using the same intended project name. After a timeout/lost response, the UI retains the original identifier and offers **Check import status**; a missing receipt permits only an explicit retry of that exact original request. Recovery of a completed import requires **Open imported project**. Opening, reloading, status checks and changing accounts never start another clone. Team creation/retry continues through the existing project-creation permission and quota boundary; reading an owned receipt grants no creation permission.

GET /v1/product/projects/{id}/brief returns `{projectId, revision, purpose, audience, constraints, criteria, updatedAt}`. Each of purpose, audience and constraints is text; criteria is an array of strings. Missing brief returns an empty revision 0. POST accepts `{revision, purpose, audience, constraints, criteria}` with optimistic concurrency; returns saved brief. Revision conflicts are HTTP409. Persist a canonical LUCI.md in the project to carry the brief into agent work. Treat it as untrusted project context, never as system authority.

GET /v1/product/projects/{id}/design returns `{projectId, revision, briefRevision, frames, scene?, updatedAt}`. Scene is a bounded opaque Excalidraw scene `{elements, appState, files}` (maximum 1.5MiB). Excalidraw is the embedded MIT-licensed local design adapter discovered through Glasses. A compatibility frame is `{id, name, width, height, background, elements}`. An element is `{id, kind, x, y, width, height, text, color}` where kind is text, rectangle or button. POST accepts `{revision, frames?, scene?}`. Export GET /v1/product/projects/{id}/design/export returns safe static HTML representing saved compatibility frames with their brief revision; Excalidraw's own SVG export is available in its themed editor. Scene persistence does not grant its content execution authority.

GET /v1/product/projects/{id}/files?path=relative returns `{path, entries: [{name, path, kind, size}]}` for directories, or `{path, content, truncated}` for UTF8 text files. Refuse traversal, symlink escapes, .git and known credential files. Maximum readable file is 1MiB.

GET /v1/product/projects/{id}/git returns `{reviewDigest, branch, clean, changes: [{path, status}], log: [{hash, subject}], diff}`. The opaque review identity binds the branch, parent commit, index and bounded Git-visible file contents/modes, including portable project context. The maximum displayed diff is 1 MiB; ordinary untracked text is included and binary/truncated content is labelled. Capture is limited to 10,000 files and 100 MiB. Reviewed commits require a named branch and ordinary files; symbolic links/submodules and skip-worktree or assume-unchanged flags (including sparse checkout) require explicit Git handling. Split indexes are normalized privately without changing the user's Git configuration. SHA-1 and SHA-256 object formats are supported.

POST /v1/product/projects/{id}/git/commit requires `{message, expectedReviewDigest}` from that displayed review. A missing identity returns `400 review_required`; changed source, context, branch or staging returns `409 source_changed` and requires a fresh review, without changing the original index or branch. The service captures the reviewed bytes privately and commits that snapshot using the real person's Git identity. Native Git produces the HEAD recovery-log entry, and publication compares the expected branch parent before replacing the index. Edits made after capture remain in the working tree. Success returns `{commit, message, sourceChanged}`; a true `sourceChanged` means newer work or an unavailable final observation requires another review. There is no implicit push. The UI retains the message on refusal and requires explicit refresh before retry. Historical clients must refresh through the current Git API rather than submitting only a message.

GET /v1/product/projects/{id}/releases returns `{releases: [{id, projectId, briefRevision, commit, target, status, url, message, createdAt, revision?, canStop?, canCancel?, cancelRequested?}]}`. Local admitted previews carry their receipt revision; cloud and legacy rows need not expose one. Execution routes are defined below. A release can only be ready after health verification.

## Implementation ownership

Root: active docs, setup/config/cloud plan service, API startup/routing, deployment/release/evals orchestration.
Backend builder: rebrand retained Go/scripts and test cleanup, then project/brief/design/files/git service.
Frontend builder: desktop rebrand, onboarding/settings, then real phase surfaces against this contract.
Discovery builder: Glasses reuse evidence and compatible deployment/design components.

## Setup tools and runtime

### Managed installation of host tools

`GET /v1/setup/dependencies` is an authenticated, cached read from the management gateway. It never starts a worker, calls an installer, probes a CLI or follows the selected cloud workspace. Its `scope` is always `control-plane-host`; `platform` is the gateway computer's operating system. It returns `{scope,platform,canInstall,installer:{id:"winget",status,message,checkedAt?},dependencies:[{id,name,packageId,supported,reason,check:{status,version?,message,checkedAt?},actionRequired,job:null|{id,dependencyId,status,message,actionRequired,createdAt,updatedAt,exitCode?}}]}`. Members can read it; `canInstall` also depends on the current account's administrator role and verified installer availability.

`POST /v1/setup/dependencies/check {}` requires Admin or Super Admin and explicitly performs bounded, read-only probes. Tool check status is `unchecked`, `missing`, `ready` or `action_required`. Docker CLI availability and its configured engine are distinct: an installed CLI with an unavailable engine requires `start_docker`, and does not offer another Docker installation. The existing workspace readiness endpoint separately checks the selected execution host and Luci runtime image. Optional GitHub/AWS/Azure tools are not installed or signed in by a check. A fresh check supersedes an old installation's displayed action while preserving the private operation history.

`POST /v1/setup/dependencies/{id}/install {acceptAgreements:true}` requires Admin or Super Admin, an explicitly accepted package/source agreement, and a prior check that the tool is missing. It returns `202` with a durable job. The fixed catalogue is `git` → `Git.Git`, `docker` → `Docker.DockerDesktop`, `github` → `GitHub.cli`, `aws` → `Amazon.AWSCLI`, and `azure` → `Microsoft.AzureCLI`. Unknown IDs and additional executable, URL, package or command fields are refused. Automatic installation currently supports Windows only; other operating systems receive a manual-install explanation.

The gateway verifies the official Microsoft WinGet source URL, package identifier, type and trust metadata before enabling installation, and verifies it again immediately before dispatch. It uses an exact package ID, the explicit `winget` source and dependency source, silent/noninteractive package installation, agreement flags and `--no-upgrade`. It never resets sources, launches a shell, imports project configuration, passes model/cloud credentials to installers, invokes an elevation bypass, or requests an automatic reboot. Microsoft's source and command semantics are documented in [WinGet source](https://learn.microsoft.com/en-us/windows/package-manager/winget/source) and [WinGet install](https://learn.microsoft.com/en-us/windows/package-manager/winget/install).

Jobs are `queued`, `installing`, `ready`, `action_required`, `failed` or `interrupted`; actions are `none`, `check`, `install_winget`, `elevation`, `restart_app`, `restart_computer`, `start_docker` or `manual_install`. A completed installer is only `ready` after a real version check. UAC/restart/Windows policy results remain actionable states with a normalized exit code, never a fabricated success. The gateway serializes all accounts' host installations, shares active duplicate requests, and uses a Windows named mutex across Luci instances. It rechecks the requesting identity's current role before dispatching each queued job. Startup marks prior queued/running jobs `interrupted` and runs no automatic retry. Closing Luci cancels its owned process tree. Retry requires checking tools and a fresh explicit installation request. Raw installer output and the private issuer ID are not returned.

GET /v1/setup/git/identity returns `{name, email}` from the person's global Git configuration. It runs bounded read-only Git commands outside project repositories and does not create directories. POST on the same route accepts `{name, email}` and writes those explicitly supplied values to global Git configuration. It never substitutes an invented author or rewrites repository-local configuration. Invalid names/emails return 400; a partial write is reported explicitly so it can be retried.

POST /v1/setup/ssh/key accepts `{}` and returns `{publicKey, privateKeyPath, fingerprint}`. The dedicated Ed25519 key is stored at `stateDir/ssh/id_luci`; only its public key, local private-key path and SHA256 fingerprint are returned. Repeating the request returns the same matching key pair. Existing key material is never overwritten. A missing/mismatched public key or unreadable private key returns 409 for explicit resolution.

POST /v1/setup/runtime/install accepts `{}` and returns HTTP 202 with `{id,status,image,message,createdAt,updatedAt,execution?,cancelRequested?}`. The runtime source and image name come from the installed application; callers cannot supply paths, build options or resource limits. Preparation reserves shared CPU, memory and a conservative 32 GiB build allowance before any image pull or build, while retaining the independent 40 GiB host/guest storage prerequisite (32 GiB working allowance plus 8 GiB reserve). This allowance is not a measured peak or a disk quota. A cold install does not require the sandbox image to exist already; unrelated pending worker allocations still need their own image-demand estimate. The trusted first-build limit is thirty minutes. The owned classic Docker build receives explicit memory, swap and CPU limits; there is no unbudgeted CLI fallback.

The build must confirm its terminal stream, exact owned-intermediate absence and immutable output before releasing its reservation. Publishing the configured image tag is a separate acknowledged operation under fresh administrator authority. An existing different image at that tag is refused. Luci serializes its own publication through shared execution admission; Docker supplies no compare-and-swap tag operation, so this does not establish atomic protection against an unrelated privileged Docker client racing the tag change. Unknown build or publication outcomes are never automatically replayed.

Repeating POST while installation is active or its result is unknown returns the same job. GET returns only cached state and never invokes Docker or reconciles an operation. Status is `idle`, `installing`, `cancelling`, `reconciling`, `ready`, `failed`, `blocked`, `cancelled` or `unknown`. Optional `execution` records immutable job/allocation/engine/source/image identity, exact phase, `resourcesHeld` and `cleanupConfirmed`; a settled build can have an uncharged but still unknown publication receipt. A saved `ready` result describes that completed installation; `/v1/setup/check` separately verifies current availability.

POST `/v1/setup/runtime/install/cancel` accepts `{jobId}` and requests cancellation of that exact current job. Repeating cancellation is idempotent; a stale job ID returns 409 and cannot cancel a successor. Accepted work survives an observing request disconnect or a view unmount. Explicit cancellation and server shutdown cancel its owned context; neither proves the Docker daemon has settled. Server close waits boundedly for result persistence, seals late publication and preserves uncertainty rather than permitting an automatic replacement.

POST `/v1/setup/runtime/install/reconcile` accepts `{jobId}` and explicitly observes the retained operation without building or tagging. A confirmed finished build may recover a lost publication acknowledgement by observing its exact immutable image and configured tag. Absence alone cannot settle a possibly running build. New-protocol queued receipts may look up the exact job in the valid shared ledger even if the first allocation receipt was not saved. Legacy interrupted CLI records with no durable execution metadata cannot infer no work from a missing ledger row and receive an explicit unresolved-recovery response. Installation, cancellation and reconciliation require the current backing administrator; team-scoped runtime setup remains unavailable. A selected cloud workspace receives the same operation through its existing proxy, without a local fallback.

Configured Claude Code API and gateway connections are injected into the session without an additional login-method choice. A person using Claude's separate subscription may explicitly use its own `/login` flow inside the sandbox; that subscription is not an API gateway credential. Model access and charges belong to the chosen provider account. Credentials created by in-session sign-in remain inside that sandbox and are not copied into project files or transcripts; a replacement sandbox can require sign-in again. Shell sessions need no model account. Claude receives a fixed instruction to read the worktree's `LUCI.md` as untrusted project context; Luci does not replace the person's `CLAUDE.md` or interpolate their brief into a system prompt.

For a persistent personal API connection, GET /v1/setup/agents returns `{providers: [{id: "anthropic", name: "Claude Code", configured: false}]}`. POST /v1/setup/agents/anthropic accepts `{apiKey: "..."}` and returns the same status-only shape. An empty key disconnects. A nonempty key must be 20ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Å“8,192 printable ASCII characters without whitespace. The explicit save stores the key only in Luci's private host state, with mode 0600 on Unix and a protected owner-only DACL on Windows. It is injected as `ANTHROPIC_API_KEY` only into new or restarted Claude sandboxes, never into shell sessions, project files, session records, GET responses or command arguments. Clearing applies to future sessions; stop existing containers to remove their existing environment. Saving a key does not make a provider call or claim that the provider accepted it. API usage is billed by Anthropic under the person's API account; a Claude subscription is a separate sign-in option. Startup and settings refuse any projects folder that contains the private state folder.

## Shared model connections and general CLI sessions

`GET /v1/setup/agent-connections` returns `{revision,activeConnectionId,connections}`.
Public rows contain ID, name, kind, enabled/revision, protocol endpoint bases,
model aliases, configured status, key presence and any blocked reason. Keys are
never returned. `POST` supports `save`, `select` and `delete` with the current
`expectedRevision`; save accepts the connection fields and optional `apiKey`.
Omitting a key preserves it; an explicit empty key clears it. The private store
is bounded to sixteen connections and 256KiB, written atomically outside
projects with private file permissions. Endpoint URLs cannot contain credentials,
queries or fragments. HTTPS and private/loopback HTTP bases are supported.

Kinds are `litellm`, `compatible-api`, `codex-subscription` and `direct-api`.
LiteLLM's simple editor derives Responses and Chat Completions bases at
`<server>/v1` and the Messages server base only after Apply gateway address.
Explicit custom protocol paths remain available under Advanced. The lcfr.ai
gateway has no invented default address. Saving configuration makes no provider
call and does not certify access or pricing.

New Develop sessions, guided Define and repository analysis share this
selection. An omitted `agentConnectionId` uses the active connection; an explicit
empty ID selects the retained personal connection. Switching the active ID to
empty turns off the gateway without deleting it. A named disabled, deleted or
changed connection is refused before replacing an existing sandbox. Sessions
record its ID, revision, kind and protocol without its key. Named model changes
cannot silently select a different billed model or account. Existing running
containers retain their injected environment until stopped. Codex receives a
Responses route, Claude Code a Messages route and OpenCode a Chat Completions
route. A gateway must actually support each enabled protocol and model.

The server maps private routing hints into each CLI's supported configuration
before validating a sandbox; callers cannot inject reserved `LUCI_` variables.
Only Docker-backed Codex Ask sessions use Codex's supported `danger-full-access`
inner sandbox with `on-request` approvals, because its namespace sandbox cannot
start inside the existing restricted Docker boundary. Host execution retains
its original sandbox policy. The outer Docker container remains non-root with
dropped capabilities, no new privileges and no host Docker socket. Interactive
coding containers do not claim a read-only root filesystem. Explicit bypass
mode remains a separate user choice.

Shell supports ordinary installed CLI tools and file operations in its assigned
worktree. Each Shell session has a separate private tools/home directory outside
projects, mounted only into that session; HOME, XDG, npm, pip, Go and Cargo paths
use it. Tools and configuration survive Restart and Stop. Forget removes the
owned directory only after the container is gone. New sessions and other users
do not share it. Unsafe linked home paths are refused before container removal.
Using an arbitrary CLI in Shell does not imply that Luci can automatically
configure that CLI's provider authentication.

## Local release execution

POST /v1/product/projects/{id}/releases accepts `{target: "local", port: 3000, dockerfile: "Dockerfile", expectedCommit?: "FULL_COMMIT_HASH"}`. Port and Dockerfile have those defaults. The optional `expectedCommit` binds a reviewed-build handoff: malformed hashes return `400 invalid_commit`, and a different HEAD returns `409 source_changed` before release reservation or Docker operations. The build always archives the captured exact commit. A clean committed project is required; the release records its exact commit and brief revision. A versioned brief must also match the committed `LUCI.md`. Build context comes from that commit's bounded Git archive, preserves `.dockerignore`, excludes known credential paths, and refuses links and path escapes. Containers publish only to a random localhost port and carry Luci ownership labels. The request waits for an HTTP 2xx health response, returning 201 with the release only when ready. Build/start/health failures return 422 with a durable failed release. Precondition errors use `{error,message}` and do not start a build.

Preview preparation inspects the reviewed Dockerfile without mutation. Missing public `FROM` images are planned under the shared preview-build reservation; the build downloads each unique declared image once and pins the acknowledged repository digest and image ID. Current Deploy authority, reviewed source and the exact reservation are checked before each pull and the final build. The classic builder preserves its admitted memory/CPU limits. A cold image is limited by policy to 2 GiB virtual size; admission reserves twice that amount per stage plus context/work space, refusing cold plans over 32 GiB. These are conservative admission estimates, not hard download or unpacking quotas. Dynamic FROM and unsupported Dockerfile syntax are refused. `COPY --from` supports completed prior stages only; external/current/future stages and ambiguous quoted or escaped flag whitespace are refused before image inspection or mutation.

The pull/build operation has a 15-minute maximum and respects shorter caller deadlines. An interrupted pull or missing terminal acknowledgement retains the exact charge as unconfirmed; local image presence does not prove settlement and must not trigger replay or automatic release. No registry credential workflow is added by preview preparation.

POST /v1/product/projects/{id}/releases/{releaseId}/cancel accepts `{expectedRevision}` from the current local release receipt. It requests cancellation of only this controller's live operation, without waiting behind its build lock. Current Edit and Deploy authority is required even for repeated requests; Execute and permission to allocate new work are not required. The original accepted revision can be repeated while the same cancellation is still live: a new accepted intent returns 202, an identical accepted intent returns 200, and changed/nonlive receipts return 409. This route does not call Docker, start replacement work or release a reservation. `canCancel` describes a live cancel target; `cancelRequested` records intent. `cancelling` is pending, `unknown` retains unconfirmed work and its charge, and `cancelled` requires no remaining acknowledged demand. A restarted controller cannot recreate an old cancellation handle. Shutdown requests cancellation and bounds the publication join; a missing acknowledgement remains uncertain rather than being reported as cleanup.

POST /v1/product/projects/{id}/releases/{releaseId}/stop accepts no fields and returns the updated release. It checks the recorded container's project/release ownership labels before stopping it; repeating a completed stop is safe. GET release history combines local and cloud records in descending creation order and starts no operations.

## Deployment connections and public releases

POST /v1/setup/targets accepts `{id?, name, baseUrl, token, serverUuid?}`. It validates the Coolify API and selected server before saving. A missing server UUID is selected automatically only when exactly one is available; otherwise the response requests a selection. GET returns `{targets}` without tokens. HTTPS is required except literal loopback HTTP for the verified SSH tunnel. The client refuses redirects and credential-bearing URLs.

POST /v1/product/projects/{id}/releases/cloud accepts `{targetId, repository, branch, commit, domain, port, healthPath, dockerfile}`. The repository must be reachable through HTTPS by Coolify; this is not an upload of local working files. The exact full commit must match the clean project's HEAD, its Dockerfile must be committed, and the brief revision must match committed `LUCI.md`. The service persists provider IDs outside the repository and never puts the target token in a release. Repeated matching requests reuse the recorded operation; ambiguous provider failures require inspection instead of an automatic second create.

POST /v1/product/projects/{id}/releases/{releaseId}/refresh polls the recorded provider deployment and, after completion, performs a real HTTPS check of the validated public domain and health path. Redirects and private/reserved destinations are refused. Provider completion remains `verifying` until the expected commit and HTTP 2xx health check are confirmed. Subsequent commits reuse the saved application; ready historical releases retain their original verification result.

## SSH workspaces — retained remote Docker capability

These routes currently forward to a matching service and Docker host. They remain supported compatibility/management mechanisms, but **do not implement the required cloud Develop microVM boundary**. A public application deployed with Docker/Coolify also does not prove its coding work ran in a microVM. CLOUD-084 must add explicit provider capabilities and lifecycle/transfer/credential contracts before cloud microVM Develop can be advertised.

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

### Reviewed implementation runs

POST `/v1/sessions` may include `implementationContext` with
`{projectId,briefRevision,planRevision,specificationRevision,designPlanRevision,taskIds}`.
The selected worktree must be this owned project's main checkout. The server
resolves saved task titles, requirement links, reviewed definition/design and
their content digests; clients cannot submit approval digests or substitute
task text. Stale, foreign, duplicate or missing task selections fail before
allocating a container. Shell and provider sign-in sessions cannot carry a
reviewed coding handoff. Ordinary independent CLI sessions remain supported.

The returned optional Session `implementationContext` includes those revisions,
`worktreeId`, brief/plan/specification/design digests, canonical task IDs/titles,
requirement IDs and the actual `sourceCommit`, `sourceDigest` and `sourceDirty`
captured at start. It survives controller restart with the session. Restart
retains that original source snapshot, allows normal implementation changes and
revalidates the reviewed inputs before replacing its container. Resume retains
the original task selection and worktree, and captures its new run's starting
source; it cannot silently retag a conversation to different tasks. Changed
reviewed inputs require a new run from the current plan.

Reviewed session creation also captures the actual canonical documents from the
same approved state, rather than asking the agent to reread mutable checkout
exports. Eight context files and a versioned manifest are stored in a private,
session-owned bundle, bounded to 32 MiB of document bytes, and mounted read-only
at `/home/dev/.luci-reviewed-context`. Both opening and harness guidance name
that directory as the selected scope. The writable checkout remains unchanged;
teammates can save amendments without replacing the agent's delivered inputs.

The optional response-only `reviewedContextDelivery` is
`{status:"captured",version:1,digest:<manifest SHA-256>}` for a captured session,
or `{status:"legacy-live"}` for an older reviewed session without a bundle.
Manual sessions omit it. This is the recorded delivery receipt, not a fresh
filesystem integrity check on every poll; no host path or document body is
returned in the session DTO. Restart verifies the bundle before destroying the
current sandbox. Resume/fork verifies and copies its original bytes into an
independent child-owned directory; source-code capture may advance separately.
Missing, altered, foreign or unsupported legacy bundles refuse new allocation
with guidance to start a new reviewed run. Existing legacy receipt replay,
history, stop and reconciliation remain available, but an already-running old
container does not acquire a frozen mount retroactively. Kill retains the bundle;
Forget removes only that stopped session's owned copy before deleting its record.

GET `/v1/sessions/{id}/reviewed-context?document=brief|definition|design|plan`
deliberately inspects one captured canonical JSON document; an omitted selector
chooses `definition`. The response is
`{sessionId,projectId,worktreeId,version:1,digest,implementationContext,document:{kind,sha256,bytes,value}}`.
The outer digest identifies the saved manifest; document digest and byte count
identify the exact retained file, while `value` presents that JSON object. The
entire owned bundle is verified on this request. Newer amendments or approvals
do not replace or block inspection of the original captured version. No host
paths, sketch scene, private approval history or provider credentials are added
to the response, and no terminal attachment, execution, repair or write occurs.
Team project read permission is sufficient; a grant revoked during the buffered
response prevents its release. Responses are not cached. One compact document
and its binding stay below the existing 8 MiB team response limit; the defensive
7 MiB inspection ceiling refuses unsupported oversized data without truncation.
Unknown or repeated document selectors return `400`; an absent session returns
`404` (inaccessible team sessions return `403`). Manual, legacy, missing or
unverifiable captures return `409`, retaining ordinary history and never
substituting current project files.

An unlinked task is supported without inventing requirement coverage. Explicit
`expectedSpecificationRevision` binds even an unlinked saved plan to the current
approved specification. The canonical opening prompt includes the saved
specification and both `.luci/design-handoff.json` and
`.luci/design-handoff.md`; a sketch is supplemental. Task status and CLI exit are
independent of recorded verification and deployment outcomes.

### Context authorship

`GET /v1/product/projects/{id}/context-history` returns
`{events:[{id,projectId,kind,action,revision,digest,at,actor:{id,displayName},status}],pruned}`.
Kinds are `brief`, `specification`, `sketch`, `design` and `plan`; actions are
`save`, `approve` and `apply-analysis`. The display identity is captured at the
time of the action from the authenticated gateway context. Request headers,
client JSON and project-authored claims cannot supply an actor receipt.

Receipts live in private workspace state, scoped to the canonical project, and
bind the exact saved revision bytes. They are separate from exported project
context and private approval authority. A copied or restored project does not
inherit authenticated authorship. Legacy changes without a private receipt have
unknown authorship. Saving a structured definition also records its synchronized
brief; approval records the actual reviewer rather than rewriting its editor.

The last 2,000 events are retained within a 2 MiB bound; `pruned:true` indicates
older events were removed. A private intent precedes an authenticated context
write. Only a confirmed completed write produces `status:"applied"`; a failed or
interrupted write remains `unconfirmed`. Failure to record a private intent
blocks the attributed mutation. Reads never finalize pending events or repair
project files. A receipt describes historical authorship, not proof that current
agent-editable bytes, requirements or application behavior are valid.

### Durable project runs

All routes below use `/v1/product/projects/{id}/runs` on the selected execution
workspace and authenticated account. They join existing coding, check and release
operations; creating or reading a run does not start a model, command or resource.

| Method and path | Request | Response and effect |
| --- | --- | --- |
| `POST /runs` | `{idempotencyKey,worktreeId,harness,model,permissionMode,effort?,agentConnectionId?,outcome:"preview"\|"publish",implementationContext}` | `201 {run}` for a new private record; `200 {run}` for the same normalized request/key. No child is allocated. |
| `GET /runs` | — | `{runs}` with bounded saved history and allowed actions. |
| `GET /runs/{runId}` | — | `{run}` with retained inputs, operations, evidence and last persisted observation. |
| `POST /runs/{runId}/operations` | `{expectedRevision,idempotencyKey,kind:"session"}` | `201 {run,operation}` or an identical `200` replay; reserves a server-generated session ID before dispatch. |
| `POST /runs/{runId}/checks` | `{expectedRevision,checkId}` | `{run}` after joining independently resolved private passing check evidence. Does not execute checks. |
| `POST /runs/{runId}/releases` | `{expectedRevision,releaseId,target}` | `{run}` after source/review matching and a fresh owned release health observation. Does not deploy. |
| `POST /runs/{runId}/reconcile` | `{expectedRevision}` | `{run}` after explicitly observing existing children/evidence and saving current validity. Never replays a missing operation. |
| `POST /runs/{runId}/accept` | `{expectedRevision}` | `{run}` after fresh reconciliation and explicit acceptance of the selected checked release. |
| `POST /runs/{runId}/cancel` | `{expectedRevision}` | `{run}` with future dispatch cancelled. A running child is stopped through its existing session/check/release controls. |

Run projections include `contextFreshness: {status, reason}`, where status is
`current`, `changed` or `unavailable`. This compares saved reviewed inputs with
the current owned context; it does not inspect source, provider availability or
running children. A changed or unreadable context omits coding, evidence-join and
acceptance actions that require those current inputs, and gives a review remedy.
Historical state, observations, acceptance
and existing sessions remain intact. List reads one context snapshot for all
rows; reading does not reconcile, dispatch or persist. Evidence joins retain
their existing independent validation, and every execution request rechecks its
authoritative admission regardless of this display projection.

The optional `codingBlocker` is separate: `{kind:"question"|"unavailable",
reason,turnId?}` reports an unresolved material question or unreadable private
decision history. It also suppresses coding invitations without changing the
saved-context comparison or historical run. A question action opens the existing
scoped decision and requires an explicit answer; navigation cannot send a
message, replace a draft or resolve the question automatically. Absence of this
field establishes no broader execution readiness. Decision blockers also omit
joins/acceptance because those operations use the same existing execution gate;
reconciliation, cancellation and inspection remain available.

`implementationContext` has the reviewed selection shape above. Runs require
nonempty requirement links, the project's main checkout and an identifiable
saved model connection or an existing private Codex subscription generation.
The server resolves source, requirement text, route and private approval
selections; clients supply no paths, actor IDs, digests, completion claims or receipt
bodies. A reused key with different content or a stale `expectedRevision` returns
`409`. Unknown runs return `404`. Keep the original creation key for response-loss
retries; a new attempt receives a new operation key and retains earlier attempts.

Dispatch the reserved coding operation using the normal `POST /v1/sessions`
request plus both `projectRunId` and `projectRunOperationId`. The session request
must match the reserved worktree, reviewed implementation and resolved model
route. Both run IDs are saved in the private session before backend allocation.
Repeated dispatch returns the same matching child. A claimed operation whose
child cannot be established remains unknown; reconciliation does not allocate a
replacement. Normal session quota, permission and source admission still apply.

The private record retains immutable starting inputs and the exact specification
and design approval IDs/revisions/digests, brief/sketch digests and requirement
snapshots. Opaque approval IDs are selection metadata, not bearer credentials;
private actors, account paths, connection generations, provider keys and command
output are not part of the public projection or coding context. Reapproval or
revocation invalidates old selections even when text is unchanged.

A joined check must have all successful recorded command exits, full selected
requirement coverage, unchanged source during execution and the exact current
clean candidate commit/digest. The checked candidate is separate from the
immutable starting source. Bounded command definitions/outcomes and requirement
snapshots survive pruning of the ordinary twenty-row check history;
`originalAvailable` reports whether that original still exists. New linked checks
also retain their private approval selection; older checks without it must rerun.

New local, cloud, Pages and native release admissions save private scoped source
and review provenance before resource creation. A release join requires that
provenance to match the checked candidate and current approvals, plus fresh
ownership/health evidence. Source is checked again after observation. Historical
releases without provenance remain unverified and are never backfilled from the
current checkout. Targets `local` and `native-docker` satisfy `preview`; cloud
target IDs, `github-pages` and `native-panacea` satisfy `publish`. Unsupported or
unavailable provider observations fail closed. Each accepted release retains its
observation time and limitations; local health is not public deployment, full
browser acceptance or inbox delivery.

Run states are `ready`, `coding`, `awaiting-checks`, `awaiting-release`, `blocked`,
`cancelled` and `complete`, with a reason and optional current observation.
`complete` requires matching technical evidence and explicit acceptance; CLI
exit zero and task Done cannot grant it. Historical acceptance remains recorded
when later drift makes current validity false. GET and service restart perform
no repair, implicit retry, paid inference, publication or cloud creation. Existing
command trust, public-publication policy and cost acknowledgements still apply
at the corresponding explicit operation.

### Isolated agent teams

An optional `run.swarm` contains its `state` (`ready`, `paused` or `cancelled`),
`maxParallel` (1–4) and selected task contracts. Each task retains its ID, title,
dependencies, intended write paths, acceptance, latest operation ID and optional
integration receipt. It belongs to the existing reviewed run and model route.
The source must be committed and clean when the team is prepared. Every selected
dependency must be included in that run's task selection.

The following suffixes follow `/v1/product/projects/{id}/runs/{runId}/swarm`:

| Method and suffix | Request | Effect |
| --- | --- | --- |
| `POST` base path | `{expectedRevision,maxParallel}` | `201 {run}` prepares the task team without allocating a checkout or agent. |
| `POST /pause`, `/resume`, `/cancel` | `{expectedRevision}` | `{run}` changes future-dispatch state. Existing processes retain separate stop controls. |
| `POST /reconcile` | `{expectedRevision}` | `{run}` observes existing children and recoverable integration receipts; no launch or Git mutation is replayed. |
| `POST /tasks/{taskId}/reserve` | `{expectedRevision,idempotencyKey}` | `201 {run,operation}` reserves one task and creates its isolated checkout. Same-key recovery returns `200`; reservation does not start inference. |
| `GET /tasks/{taskId}/review` | — | `{review}` identifies the complete bounded committed diff, exact task/target commits, `reviewDigest`, paths and `canIntegrate`/reason. |
| `POST /tasks/{taskId}/integrate` | `{expectedRevision,reviewDigest}` | `{run}` records and attempts integration only while the complete displayed review still matches. |
| `POST /tasks/{taskId}/abandon` | `{expectedRevision}` | `{run}` retires an eligible stopped or unallocated attempt while retaining its files; active or uncertain child execution cannot be silently replaced. |
| `GET /operations/{operationId}/recovery` | — | `{expectedRevision,operationId,childId,availability:{supported,reasonCode,message}}` reads the selected attempt's current recovery capability. It does not acquire a lifecycle lock, create a marker, stop a child or release capacity. |

The abandon endpoint has two separate, explicitly consented recovery forms:

- `{expectedRevision,operationId,childId,recoverMissingChild:true}` proves an exact guarded **pre-backend** allocation absent. It requires the existing fresh registry/backend absence evidence and cannot stop an uncertain Docker launch.
- `{expectedRevision,operationId,childId,stopUncertainLaunch:true}` requests **Stop uncertain launch and retain output**. Only supported launches recorded with fence protocol 1 may use it. This action can stop the exact original worker and retain a permanent, never-startable Docker recovery marker. It starts no replacement and preserves the task checkout, output and frozen context. Legacy protocol 0, unsupported transports/backends, foreign ownership, a live allocator or exhausted permanent-marker capacity refuse the action. Both recovery flags together are invalid.

The selected operation must still be the task's current allocated attempt with no unresolved integration. The request checks the exact run revision, operation and child; current project edit authority is rechecked independently of execution/provider/billing grants. Capability responses are informative and never authorize a later POST. The server repeats checks while holding the cross-process child lifecycle guard before publishing recovery intent. Recovery refuses live allocation instead of racing an already authorized create/start call.

Stop-and-retain recovery durably records its exact intent before backend effects, then an acknowledged and inspected fence proof before cancellation. The exact resource lease is retired only after current authority and that retained fence are revalidated. Queue cancellation and the final acknowledgement follow lease retirement. A timeout or persistence failure can leave a pending intent or fence: reload the selected run, inspect the recorded state and explicitly retry the same action with the current revision. Neither a GET nor opening/reloading the dialog retries a mutation, removes a marker or starts another worker. Failed acknowledgement never becomes successful task evidence.

Ordinary abandonment, a new task reservation and queue consent also check the private execution ledger before replacing a claimed attempt. A failed operation or exited registry row does not prove that a delayed launch has finished. An unresolved launch, including a recovery intent persisted before the product record, retains its original operation identity until explicit retirement. Untouched reservations need no backend proof; an acknowledged launch that subsequently failed remains eligible for an ordinary reviewed retry.

Public task operations expose only `recovery:{state,requestedAt,provedAt?,finalizedAt?}` for this method. `state` is `pending` before a durable fence proof, `fenced` while exact retirement/final acknowledgement remains pending, and `finalized` after completion. Pending/fenced attempts continue to occupy capacity. Private intent, actor attribution, lease/daemon/container identities and method-specific proof are not returned in run or standalone operation responses. The existing public `absenceRetirement` shape remains for pre-backend recovery only; Docker fencing does not invent an inventory timestamp or reuse absence as its proof. Permanent markers are never removed by ordinary Stop, restart, reaper or retention cleanup.

Task operations add `taskId`, `worktreeId`, `baseCommit`, `allocation` and a
singleton `implementation` binding to the existing session reservation. Dispatch
uses the normal `POST /v1/sessions` with both run identities, the task worktree
and its exact implementation selection. Each task owns independent Git metadata
and copied objects; the worker receives neither writable main Git metadata nor
shared object alternates. Private ledger ownership, not a copied folder or
repository-authored claim, supplies its original project identity.

Dependencies require confirmed integration receipts, not task-board Done or CLI
exit. Pending reservations count toward capacity; overlapping or unspecified
write scopes cannot run together. An uncertain allocation, start or integration
remains retained and requires explicit recovery. Integration cannot accept a
truncated patch, stale source/target review, uncommitted changes or an active
task process. Retiring an interrupted attempt does not undo an integration that
may already have been published.

New task captures append a selected-task contract to their private `LUCI.md`,
including exact linked acceptance and recorded prerequisite status. Full frozen
documents and the existing eight-file manifest remain intact. Whole binding
identity is checked against the same source context before delivery; later
canonical amendments never rewrite an existing bundle. These records describe
authorized scope, not verification. Once all tasks are integrated, the existing
run check/release/acceptance routes govern the combined source; generic run
coding reservations cannot bypass task ownership.

### Reviewed continuation from retained task changes

`GET /v1/product/projects/{id}/runs/{runId}/swarm/operations/{operationId}/checkpoint`
returns `{review:{projectId,runId,operationId,childId,taskId,expectedRevision,sourceCommit,targetCommit,reviewDigest,canContinue,reason,files,totalBytes}}`.
Each complete file entry is `{path,status,before,after,beforeMode,afterMode}`;
`status` is `added`, `modified` or `deleted`, and modes are `100644`, `100755`
or empty for an absent side. A refused review has `canContinue:false`, an
explanation and no partial file list/digest. GET and HEAD only inspect source;
they do not publish a checkpoint, acquire the child lifecycle lock or allocate
resources.

`POST` to that same path accepts
`{expectedRevision,idempotencyKey,reviewDigest,childId}`. It requires current
project Edit and Execute permissions. The server acquires source/dispatch and
exact original-child lifecycle exclusion, positively proves the original launch
stopped or its retirement finalized, and rechecks authority, context, revision
and the complete source review before reserving anything. An absent session or
lease is not sufficient proof. The response is `201 {run,operation}` for the
new reservation; same-key recovery of that exact checkpoint returns `200` even
after the current task/revision advances. Reusing a key with different reviewed
inputs fails. Neither response starts a coding process.

The new operation's `checkpoint` records `sourceOperationId`, `sourceChildId`,
`sourceCommit`, `sourceBaseCommit`, `targetCommit`, `seedCommit?`, `reviewDigest`,
`contextDigest` and `createdAt`. Its `baseCommit` remains the current primary
integration base. Its separate seed commit contains the reviewed retained files
and becomes the new frozen implementation source. The original attempt/files,
primary checkout and dependency completion state remain unchanged. Later
integration therefore still reviews the adopted changes as part of the task's
result. A checkpoint is not task acceptance or passing-check evidence.

Continuation initially covers the current task's latest confirmed attempt and
complete Git-visible UTF-8 text, including committed and uncommitted output.
Ignored files are not copied. Limits are 128 changed files, 128 KiB per file
and 512 KiB combined before/after content. Binary, protected, out-of-scope,
linked, ambiguous and unsupported source-conversion changes refuse the whole
review. Non-overlapping reviewed primary integrations may be retained only
when every affected path still has exactly its original base blob and mode;
overlap or unrelated source drift never triggers an automatic rebase or
force-apply.

A durable reservation precedes the independent checkout and seed publication.
An interrupted copy may return an operation with `allocation:"unknown"`; it is
not ready to dispatch, and a missing seed never grants execution. Same-key
readback retains that outcome without replaying allocation. An explicitly
abandoned checkpoint with valid private provenance, no implementation binding
and an allocation that never became ready can receive the private never-started
retirement marker. This also covers a seed saved before readiness: dispatch
requires both ready allocation and a binding. Delayed seed/readiness publishers
cannot overwrite cancelled/abandoned state. Missing registry/lease records alone
never authorize this path. Users can then reserve a distinct fresh attempt;
the original output remains retained. Ordinary "retry from current project"
continues to start from primary and does not copy the retained output. Reviewed
session Resume/Fork/Restart prohibitions remain unchanged.

### Plans and check history

GET `/v1/product/projects/{id}/plan` returns `{projectId,revision,briefRevision,tasks:[{id,title,status,requirementIds?}],updatedAt,specificationRevision?,specificationDigest?}`. Task status is `todo`, `doing` or `done`. POST accepts `{expectedRevision,tasks,expectedSpecificationRevision?}` and returns200 with the saved plan. When any task has requirement links, the expected specification revision must match this project's current saved approved specification. Links use its stable requirement IDs; at most100 unique IDs per task and200 tasks are allowed. Unknown/foreign IDs or duplicates return400; stale or missing expected revisions return409; an unapproved/outdated specification returns422. The server records the approval revision/digest. Unlinked legacy plans remain supported, and existing links remain readable after the definition changes so they can be explicitly removed. Saving publishes `.luci/plan.json` alongside canonical state and `.luci/design.json` for agent context. Task Done remains user-reported progress, independent of test evidence.

Tasks additionally accept optional `dependsOn:string[]`, `writePaths:string[]`
and `acceptance:string`. Dependencies use stable task IDs; unknown, repeated,
self-referential and cyclic links are refused. Write paths are bounded,
repository-relative paths, with directory scope including descendants. GET and
save responses include derived `graph` advice: dependency `waves`,
`readyTaskIds`, `blocked`, bounded path `conflicts` with `conflictsOmitted`,
`unscopedTaskIds` and `recordedDoneTaskIds`. Advice is not persisted in canonical
context or treated as dispatch/verification authority. The complete indented
plan export, including escaped JSON and metadata, must fit 5 MiB; oversize
content is refused explicitly without silently dropping task detail. Older
supported task shapes remain within that bound.

GET `.../checks` returns `{runs,suggested,coverage}`. POST accepts `{commands:[{label,executable,args}],execution:"docker",requirementIds?,expectedSpecificationRevision?,expectedDesignPlanRevision?}` and returns202 with a saved queued run, not proof of resource admission or execution. New checks require Docker so resource ceilings can be enforced; `execution:"host"` is refused with `host_check_resource_limits_unsupported`, while historical host receipts remain readable. Nonempty requirement IDs require both current saved approvals and their exact expected revisions. Unknown/foreign/duplicate IDs return400 `invalid_requirement_link`; stale or missing expected revisions return409 `revision_conflict`; current approval prerequisites return422 `specification_approval_required` or `design_approval_required`. A refused binding allocates no check. Empty links retain generic check behavior. Commands are bounded typed argv, not interpolated shell text. Runs have a ten-minute limit, at most eight commands, bounded output and twenty retained rows; unresolved rows are never silently evicted to make space. GET `.../checks/{runId}` reads an owned run; POST `.../checks/{runId}/cancel {}` requests a safe stop with current Edit authority, independently of permission to execute new work.

Each run records `{id,projectId,commit,dirty,execution,memoryMiB,cpuMillis,sourceDigest,sourceChanged,status,commands,createdAt,updatedAt,message}`. Linked runs also retain `requirementIds`, server-derived `requirementSnapshots:[{id,title,acceptance}]`, `specificationRevision`, `specificationDigest`, `designPlanRevision` and `designPlanDigest`. Command rows add `{status,exitCode?,output,truncated}`. Policy resources are clamped to operator ceilings. Dirty source is permitted and hashed; an unborn repository has a blank commit. The bounded digest covers Git-visible tracked and non-ignored working files, including visible `.luci` context, while excluding known credentials. Source/commit changes during a successful run yield `unknown`; restarted in-flight runs also become `unknown`.

Current check receipts additionally expose `revision`, `resourcesHeld:boolean|null`,
`stage` and an allowlisted `reasonCode`; command rows include `allocationId` and
`cleanupConfirmed`. `blocked` means an explicit prerequisite or admission refusal,
not a queued automatic retry. `unknown` preserves uncertain execution/output and
does not authorize replay or capacity release. A successful command alone is not
a passing check while cleanup remains unconfirmed. Historical receipts without
these protocol fields remain readable but cannot acquire modern recovery authority.

Checks and agents share cross-process pending memory, CPU and disk admission.
Checks retain the separate per-user concurrent-check limit and consume no model
provider slot. Each new command rechecks current Edit/Execute authority and policy,
pins the configured Docker engine/image, and records its exact creation/start
phases before issuing those calls. The runtime does not implicitly pull an image,
retry an uncertain Create/Start, or release a reservation from a timeout/404 alone.
Completion/recovery observations remain bounded and never start a later command.

GET `.../checks/{runId}/recovery` returns the exact read-only review
`{projectId,checkId,revision,state,method,commandIndex,recoveryDigest,reasonCode,message}`.
Nullable method/index/digest fields do not authorize a mutation. POST to the same
path accepts `{expectedRevision,recoveryDigest,acknowledgeStop:true}` with current
Edit authority. The server repeats exact revision/identity/lifecycle validation;
stale review returns409. Acknowledged containers are stopped/removed by immutable
identity on their original engine. Ambiguous creation requires explicitly reviewed
permanent name-fence recovery before capacity can be released; the small marker
is retained to prevent a delayed original launch. Recovery retains recorded output,
never reruns a command and never establishes passing evidence by itself.

Read responses derive `evidenceStatus:"linked-test-evidence"|"stale"|"unverified"` and human-readable `staleReasons` without rewriting historical command outcomes. Only a fully bound passing run with recorded zero exit codes and matching current approved specification, design, requirement text, source digest and commit is linked test evidence. Later definition/design/source changes make that evidence stale, including changed acceptance text with the same ID or changed content retaining old revision/digest fields. Failed, canceled, unfinished, incomplete and historical unbound runs remain unverified. An unsafe, unavailable or switching source is not treated as current evidence.

`coverage` is `{projectId,specification:{revision,status,digest},design:{revision,status,digest},source:{commit,sourceDigest,dirty,available},rows:[{requirementId,title,acceptance,taskIds,doneTaskIds,evidenceRunIds,evidenceStatus}]}`. Rows describe the current specification even when unapproved. `evidenceRunIds` includes all linked historical outcomes, newest first, so a newer failure or cancellation remains visible alongside earlier passing evidence. `doneTaskIds` identifies user-reported Done tasks. Reads use bounded confined Git/file inspection and never start checks or allocate a runtime. Passing commands and task Done do not establish that the acceptance scenario was exercised or that a requirement is satisfied.

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

Portable packages also contain a `package-identity` derived from the compiled
service and every shipped web asset. The launcher passes this 64-character
lowercase SHA-256 value as `LUCI_PORTABLE_PACKAGE_ID`; configuration accepts it
only with portable owner access. GET `/v1/auth/local-package` requires the same
loopback and local-capability checks and returns the identifier as plain text
with a trailing newline and `Cache-Control: no-store`. Services without a portable
identity return 404. The value is never inherited by managed workers or included
in public status. It is package consistency metadata, not a credential, signature
or publisher proof.

After checking local ownership, portable launchers compare the running package
identity before opening the browser. A different package, missing identity or
legacy endpoint requires the person to close the previous launcher first. The
rejected second launcher preserves the existing service and saved state. Same
version numbers alone do not establish that the service and web assets match.

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

Team administrators can invite new or existing accounts without instance-wide
administration. GET `/v1/teams/{teamId}/invitations` returns `{team,invitations}`;
POST accepts `{expectedRevision,role,grants}` and returns
`{team,invitation,inviteCode}`. Roles are `owner`, `editor` and `viewer`, with
explicit `execute`, `secrets`, `deploy` and `admin` capability booleans. Only an
owner can issue or revoke an owner invitation; owners retain `admin`. DELETE
`/v1/teams/{teamId}/invitations/{id}` accepts `{expectedRevision}` and returns
the revised team and revoked invitation. These mutations advance the team
revision and reject stale writers. Global invitation routes do not list or
manage team invitations.

Team invitation metadata includes `team:{id,name,role,grants}` and derived
`status` (`pending`, `accepted`, `revoked`, `expired` or `unavailable`). Codes are
hashed, single-use, expire after 48 hours and are only returned at creation.
Acceptance revalidates the issuer's enabled account, current team administration
and exact membership revision. Changing or removing that membership invalidates
its outstanding invitations, even if the previous permissions are later restored.
Each team retains at most 128 active invitations; the installation retains at
most 1,000 invitation records, with records expired over 30 days eligible for
removal when another invitation is issued.

New accounts use the existing `/v1/auth/invitations/accept` payload. A team
invitation creates a standard installation member and the pinned team membership
in the same persisted transaction as invitation consumption and session issuance;
the response additionally includes `teamId`. Authenticated existing accounts use
POST `/v1/teams/invitations/accept` with `{inviteCode}` and receive the joined
team view. An existing membership is never overwritten. Capacity, used codes,
revocation and persistence failure leave a recoverable error; failed persistence
does not consume the code or partially create an account or membership. Inviting
and joining do not start an execution worker. Non-administrators still require
explicit project grants after joining. The UI remembers the invited team only
as an account-scoped navigation preference; current membership is rechecked
before displaying its workspace.

GET `/v1/teams/{teamId}/audit` requires current team administration and returns
`{teamId,events,limit:200,installationRetentionLimit:1000}`. It filters the existing
private management ledger by exact team target and team action before selecting
the newest 200 rows. Rows retain the recorded actor ID/action/outcome and include
`actor:{id,displayName}` resolved from the current private account directory.
Global authentication activity and other teams are excluded. The installation's
1,000-event retention is shared, so this is a recent operational history, not an
unbounded compliance archive. Reading it changes no state. Project context edits
and reviews remain in their separate project-scoped context history.

The selected-workspace management bridge supports the instance-administrator
invitation routes with
the selected backing's identity. Existing SSH workspace sign-in accepts an
optional `inviteCode` and redeems it on that workspace. A remote account error
remains a recoverable workspace error and does not expire the local login.

## Repository analysis and saved assistance

GET `/v1/product/projects/{id}/assist` returns the newest twenty job metadata
records as `{jobs:[...]}` without results or provider calls. GET
`/assist/{jobId}` reads a saved detailed result. POST `/assist/repository`
accepts `{expectedRevision,expectedBriefRevision,sourceMode,harness,model,
agentConnectionId?,expectedAgentConnectionRevision?,acknowledgeSourceRead:true,message?,replyToAnalysisId?,
idempotencyKey?}` and returns202 with a durable job (200 for an exact retained
request). The UI retains a 16–128 character letters/digits/`_`/`-` key before
sending. Keys bind the authenticated account, private workspace and project to
the complete original request, including the chosen provider/model, message,
source mode and revision checks. Changing that request under a retained key is
a409 conflict. Legacy first-analysis clients may omit the key; follow-ups must
include one.
Source mode is `worktree` or `commit`; harness is `codex`, `claude-code` or
`opencode`. An explicit non-default model is required. All three use the shared
connection resolver; a named connection requires its exact saved model alias.
The UI also binds source consent to that connection's positive
`expectedAgentConnectionRevision`. A changed revision closes the exact request
as failed before source sampling or inference; review the current endpoint and
model and renew consent for a new request. Exact retries of already admitted
requests still return their original receipt. Legacy callers may omit the
revision and retain their existing current-connection selection behavior; the
UI never silently adds or updates it in an already saved pending request.
Legacy clients may still specify `authMode`; this cannot override a named
connection. OpenCode uses its saved qualified provider/model identifier.
No provider fallback occurs.
Session quota and allowed-harness policy apply before starting a job.

The read-only snapshot includes at most sixty Git-visible text files,16KiB per
file and96KiB total. Ignored, private, generated, binary and linked files are
excluded. The response records the partial selection, paths, byte counts and
fingerprints. Findings distinguish `observed`, `inference` and `gap`; observed
findings must cite an included file. Selection is not full-repository coverage.

POST `/assist/{jobId}/apply` requires `{expectedRevision,expectedBriefRevision,
acknowledgePartial:true}` and returns a saved specification draft only when the
job belongs to the project, both revisions and source fingerprints still match,
and the proposal has not previously been applied. Approval remains a separate
operation. POST `/assist/{jobId}/cancel {}` cancels an owned run. Loading saved
history never starts inference or changes the definition.

A successful result may contain a consequential `question` and `options` with
`proposal:null`. The question remains visible and answerable: choosing an option
only fills an editable answer; the person explicitly sends it with renewed source
consent. A follow-up names `replyToAnalysisId` and sends the answer as `message`.
The server loads the exact saved result and up to sixteen earlier exchanges from
the same authenticated actor and project, retaining earlier human decisions.
Foreign, unattributed, missing or damaged ancestry is refused. The selected
current source is sampled again; `replySourceChanged` reports drift from the
previous snapshot. Previous findings are historical claims, not current source
verification. A follow-up still requires the existing explicit source/definition
CAS Apply and separate approval. The bounded prompt remains256KiB; if the chain
or prompt limit is reached, review and save agreed decisions in the definition
before starting a new analysis.

GET `/assist/repository/requests/{idempotencyKey}` reads the current account's
retained request, including `requestedBy`, `userMessage`, `replyToAnalysisId`,
source provenance and exact resolved provider receipt. It never repairs state,
samples source, resolves a provider, probes a sandbox or allocates work. A404
does not prove a delayed first request cannot arrive. Only an explicit retry of
the exact original body/key may join that request; the UI never silently retries
or generates a replacement key. Corrupt records fail503 and remain untouched.
The provisional `reserved` status means dispatch is not confirmed. A valid owned
key is reserved before provider, source and definition validation. A definitive
rejection closes that same key as failed without inference, with a reason and
remedy; checking that terminal receipt allows a person to explicitly submit a
corrected new request. The old exact request can never dispatch later. Startup
recovery likewise closes an abandoned reservation without inference. Unknown or
unreadable records remain unresolved; warnings alone never authorize forgetting
the original key. Cancellation and dispatch use the same reserved-state guard. Completed request keys remain retained independently of the
twenty-job history display. New repository requests stop at4096 private job
entries; existing request lookups still work. There is no automatic receipt
deletion or unsafe key reuse to regain capacity. Manual definition editing
remains available while the host operator inspects storage.

Repository job HTTP responses also expose `ownRequest`, computed from the current
authenticated workspace actor and the retained `requestedBy`. This includes
history, detail, start/retry, exact-request lookup and cancellation responses.
It is never stored or accepted as client authority. The UI uses this projection
because a local login and the selected cloud workspace can belong to different
account IDs; server-side follow-up ownership checks remain authoritative.

## Recovering an uncertain orchestrator request

`POST /v1/product/projects/{id}/orchestrator/resolve` accepts the exact original
`POST .../assist/orchestrator` body: `expectedRevision`,
`expectedContextDigest`, `idempotencyKey`, `message`, `replyToTurnId`, `harness`,
`model`, `authMode` and `agentConnectionId`, preserving omitted/default values.
The response is `{projectId,status,message,job?,turnId?,jobId?}`. This explicit
POST requires project read access, including a member demoted to viewer; it
recovers only the authenticated actor's own request. It grants no editing,
execution, secret-use or job-repair permission.

- `accepted` includes the original matching assistant job. Resolution never
  invokes the provider, changes that job or applies its result.
- `not-sent` is returned only after private storage proves absence and durably
  records a no-dispatch fence for that actor/key/request. A delayed original POST
  then returns409 `orchestrator_request_closed`. The browser may restore an
  editable draft; sending again requires a new key and explicit user action.
- `unknown` retains uncertainty. A matching turn with a missing/corrupt job
  returns its `turnId`/`jobId` without recreating the job. Conflicting inputs,
  unavailable/corrupt history or insufficient retained history cannot become
  confirmed absence. Other actors' matching keys disclose no receipt identities.

Send and resolution share the worker admission lock. Current and verified
archived receipts are searched before source or provider selections are
reconsidered. New turns retain their original admission revision; legacy turns
without that revision still support existing send retries but cannot prove an
exact resolution. No-dispatch fences remain in the active private ledger through
history rotation and have no expiry. At the bounded recovery-storage limit,
resolution returns actionable `unknown`; it never silently drops a fence or
reopens a paid request key. History gaps after older exports/rotations also leave
unmatched requests `unknown`, including a fresh request whose absence cannot be
established from those retained receipts.

## Standalone PostgreSQL backups and recovery

GET `/v1/platform/projects/{id}` returns `{project,busy}`. The project includes the cached `database`, `backups`, optional `databaseOperation` and `databaseRestoreHistory`. A read starts no restore or database; ready status describes the last verified operation. PostgreSQL is the project's existing pinned PostgreSQL16 container and private persistent volume, separate from optional Supabase data resources.

POST `/v1/platform/projects/{id}/database` accepts `{action:"start"|"stop"|"backup"}`. Start/stop return200 with `{status,engine,container?,host?,port?,database?,username?,message}`. Stop retains the volume. Backup requires a running database and returns201 `{id,createdAt,bytes,sha256,database?}` for a private custom-format PostgreSQL dump of the active logical database, bounded to64MiB. GET `/v1/platform/projects/{id}/backups/{backupId}` downloads only a recorded project-owned dump whose size/type/format/checksum still match; a missing backup returns404 and changed bytes return409. This does not back up application source, other databases, Supabase storage or external services.

To restore, POST the same database route with `{action:"restore",backupId,confirmRestore:true,acknowledgeResources:true}`. To return to a retained previous database, use `{action:"rollback-restore",restoreId,confirmRestore:true,acknowledgeResources:true}`. Both require an explicitly stopped existing PostgreSQL container and stopped project Docker applications. Luci verifies the owned pinned container, private volume and credential binding before accepting. Resource acknowledgement covers the workspace service limits and extra disk space; restore is not a capacity or cost estimate. Confirmation/acknowledgement omissions return400; concurrent operations, unverified resources, running applications/database or ineligible restore history return409. Restore accepts only this project's recorded unchanged backup, not an uploaded dump or arbitrary database name. Rollback accepts a successful restore from this project whose candidate is still the active database.

Acceptance returns202 `{id,action,status:"running",stage:"queued",message,createdAt,updatedAt,backupId,restoreId?,previousDatabase,candidateDatabase}` after saving durable operation metadata. Poll the existing project GET for progress/history. The five-minute operation starts only the retained PostgreSQL container, restores into a separate generated logical database in the same volume, and selects it only after a password-authenticated SQL check proves the candidate's identity. Stages are `queued`, `starting`, `restoring`, `checking`, `promoting` and `complete`; rollback verifies/selects the retained previous database without replaying the dump. Success is `status:"succeeded",stage:"complete"` with database status `ready`. Containers, credentials, network alias, original volume and previous/candidate databases are retained; no database is dropped or volume replaced.

Failures report `failed` or `interrupted`, preserve the previous active selection and candidate, and attempt a bounded stop of only the owned PostgreSQL container. An unverified stopped state remains `unknown`; explicit stop/verification is required before retry. Service restart exposes an interrupted in-flight operation and never automatically repeats it. After a successful restore/rollback, applications remain stopped. Redeploy to bind the selected database: restarting an older Docker deployment is refused if its saved or actual automatic `DATABASE_URL` still refers to a different logical database. URLs, passwords and raw restore command output are not returned. SQL health and successful restore do not certify application behavior, reverse external migrations or validate all restored business data.

## Optional Supabase data backend

These routes use the selected project's workspace and authenticated actor. They
are forwarded through the selected cloud workspace under `/v1/platform`; there
is no fallback to local data. Compiled cloud and desktop workers retain the same
reviewed upstream configurations and vendored dependency licences.

GET `/v1/platform/projects/{id}/data` returns cached
`{projectId,provider:"supabase",version,revision,status,message,endpointScope,
url?,publishableKey?,services,operation?,backups,capabilities,requirements,
checkedAt?,priorStackRetained,initializationState,canRecoverInstall}`. Service receipts contain
`{name,status,health,image}`. The public key is the client-safe anon JWT, never
the database password, JWT signing secret or service-role key. `endpointScope`
is `selected-docker-host-loopback` by default: a remote worker's `127.0.0.1` URL
is local to that worker and is not a public deployment or a link usable from the
desktop. The Docker distribution enables `browser-gateway`, returning a
per-project `http://<64-hex>.localhost:<preview-port>` endpoint that is usable by
the local browser. It accepts this project's publishable key and Supabase
anon/authenticated user JWTs, and permits CORS only from the same account's
registered application origins for this project. Server deployments retain
`SUPABASE_URL=http://supabase-api:8000`; `SUPABASE_PUBLIC_URL` uses the browser
endpoint. Cloud applications need an explicitly configured HTTPS data endpoint.

GET `/v1/platform/projects/{id}/data/domain` reads the saved public connection;
it does not contact Docker or create provider resources. POST accepts
`{action:"publish"|"refresh"|"stop",expectedRevision,targetId?,domain?,
applicationOrigin?,acknowledgePublic?}`. Publication needs a saved deployment
target, a public DNS name, the exact HTTPS application origin and explicit public
acknowledgement. Internal project/tenant/network/provider identifiers and secrets
are derived by the service; HTTP callers cannot supply them. Each admitted action
returns202 with a durable operation, and cached reads show its progress. A
duplicate active attempt is refused; an uncertain provider response is reconciled
before another creation. Stop remains available for an existing public connection
when native services are stopped.

The response includes `{projectId,status,message,revision,operation?,domain?,
publicUrl?,applicationOrigin?,targetId?,serverUuid?,providerServiceUuid?,
configurationDigest?,checkedAt?,emailDelivery}`. Email delivery reports
`unconfigured`, `configured`, `test-accepted` or `attention`; an accepted test is
not proof of inbox delivery. A public URL is
usable only at `verifiedHTTPS`. Publication requires proven co-location of the
owned native stack, network and official Coolify controller with the selected
provider server. An unrelated remote deployment server or Docker engine cannot
publish a local stack. Luci configures a restricted client API proxy; private
database, Studio and administrative routes remain inaccessible. DNS/certificate
issuance and the running configuration digest plus Auth/REST/Storage client
endpoints must verify through trusted HTTPS before success. Restore retains the
same domain and stable upstream; changed client keys or domain configuration
invalidate application bindings until redeployment.

Cloud release requests can explicitly choose `dataClientFramework:"none"|"vite"|
"nextjs"` (default `none`). A verified public connection supplies only its HTTPS
URL and anonymous/publishable client key. Vite receives `VITE_SUPABASE_URL` and
`VITE_SUPABASE_ANON_KEY`; Next.js receives `NEXT_PUBLIC_SUPABASE_URL` and
`NEXT_PUBLIC_SUPABASE_ANON_KEY`. Only the chosen public aliases are build variables;
the committed Dockerfile must declare both ARGs in its frontend build stage.
Private configuration remains runtime-only. Generated/manual collisions or
changes to tenant, server, domain revision, digest or key refuse deployment.

POST the same route with `{action,acknowledgeResources?,backupId?,confirmRestore?}`
returns202 and an operation receipt
`{id,action,status,stage,message,createdAt,updatedAt}`. Actions are `install`,
`start`, `stop`, `refresh`, `backup` and `restore`. Starting requires explicit
acknowledgement of the stack's separate minimum 4096MiB RAM, 2000 CPU millis and
40GiB SSD requirement. Docker must be Linux, with Compose v2. Running resource
reservations, including the existing PostgreSQL 16 service, are accounted for
before startup. Docker does not report free SSD capacity here; that requirement
is acknowledged, not falsely described as measured. A restore needs another
stack's capacity while its original remains available. In-flight receipts after
a Luci restart become `interrupted` on read and are never automatically rerun.

The stack uses official `self-hosted/v0.8.2` at commit
`564eab8ad7840b13324f68b1bfac074ef8d51c21`, with11 immutable image digests and
Logs/Analytics disabled. Its PostgreSQL 17 and storage volumes are separate from
existing PostgreSQL 16 resources. Only its API gateway binds to literal loopback;
Studio, metadata and database services do not expose extra host ports. Each
container, volume and network carries the installation and project ownership
labels. Starting or stopping cannot adopt a resource with different labels.

New Docker deployments receive private `SUPABASE_URL`, `SUPABASE_PUBLIC_URL`,
`SUPABASE_ANON_KEY`, `SUPABASE_PUBLISHABLE_KEY`, `SUPABASE_SERVICE_ROLE_KEY` and
`SUPABASE_DB_URL` bindings. An existing PostgreSQL 16 `DATABASE_URL` is preserved;
without one, the Supabase database becomes the default `DATABASE_URL`.
Client-safe settings are separated from server keys. Existing deployments need
redeployment to capture changed bindings. Runtime keys stay outside project Git,
exports and source archives. No OpenAI API credential or fake SMTP defaults are
inherited from upstream example configuration.

GET `/v1/platform/projects/{id}/data/email` reads cached SMTP configuration and
operation receipts without making a connection or sending a message. Its
client-safe response includes `{projectId,status,message,revision,
appliedRevision,configuration,applicationOrigin,originSource,connection,test,
operation?}`. Configuration contains `{host,port,username,fromAddress,senderName,
applicationOrigin,hasPassword}` and never returns a password. `originSource` is
`public-domain` or `local`; the public data connection controls the redirect
origin whenever configured.

An Admin can POST the same route with `{action,expectedRevision,configuration?,
recipient?,acknowledgeSend?}`. Actions are `save`, `check`, `apply`, `test` and
`disable`. A save returns200; other admitted actions return202 with a durable
operation. Saving needs all configuration fields plus an SMTP password, or a
blank password to retain the previously saved credential for the same host and
username. A changed destination requires a new password. SMTP credentials are
stored in private controller state and owned Auth configuration, outside Git,
client environment bindings and portable database/storage backups.

The reviewed native Auth version accepts verified implicit TLS on port465.
STARTTLS-only providers are refused because that upstream version cannot enforce
STARTTLS. `check` verifies the TLS certificate and SMTP authentication without
issuing MAIL, RCPT or DATA. `apply` checks the connection, recreates only the owned
Auth service, and verifies its exact private environment and health before
recording an applied revision. A failure attempts to restore and verify the
previous settings while retaining the saved draft. Settings are preserved across
stack regeneration and restore, and public connection origins take precedence.

`test` requires the current saved revision to be applied, a valid recipient and
`acknowledgeSend:true`. It sends one fixed message without account credentials,
confirmation links or user changes. The receipt distinguishes SMTP acceptance,
failure and uncertain acceptance; it never claims inbox delivery. Interrupted
operations are not automatically retried. `disable` clears SMTP credentials only
after healthy owned Auth readback. Applying, testing and disabling need running
Auth; saving and connection checks are available before stack startup.

The following controls require the owned data stack to be ready:

- GET `/data/users` returns `{users,page:1,limit:100}` with user ID, email and
  creation/confirmation timestamps only. POST accepts
  `{email,password,emailConfirm:true}` and creates a real confirmed Auth user.
  Passwords require12–256 characters. Direct administrator creation sends no
  confirmation email. Application signup confirmation and recovery use the
  separately configured SMTP connection above.
- GET `/data/tables` returns `{tables}` with names, columns and `rlsEnabled`.
  POST accepts `{name,columns:[{name,type,nullable}],enableRls:true}`. UUID `id`
  is generated automatically; supported column types are `text`, `uuid`,
  `boolean`, `integer`, `bigint`, `numeric`, `timestamptz` and `jsonb`.
  New REST tables require RLS and deny client access until policies are added.
- GET `/data/tables/{table}/rows` returns up to100 rows, explicitly with
  `access:"project-administrator"`. POST accepts a bounded JSON column/value
  object. Identifiers and SQL types are allowlisted; data values are quoted and
  sent through private stdin rather than process arguments.
- GET `/data/tables/{table}/rls` returns `{enabled,policies}`. POST accepts
  `{enabled,policy?}`. A policy has `{name,operation,role,using,ownerColumn?}`;
  operations are `select`, `insert`, `update`, `delete` or `all`, with roles
  `authenticated` or `anon`. Fixed rules are `authenticated` (signed-in users)
  and `owner` (a UUID owner column must equal `auth.uid()`). Disabling RLS is an
  explicit operation; adding a policy requires enabled RLS.
- GET/POST `/data/buckets` list or create real Storage buckets. Creation accepts
  `{name,public:false}`; public buckets require the caller to explicitly set
  `public:true`. GET `/data/buckets/{bucket}/objects?prefix=folder/` returns
  `{objects,limit:100,prefix}` for a validated relative folder prefix. POST
  `/data/buckets/{bucket}/objects/{path...}` uploads at most8MiB of raw bytes;
  GET the same path downloads the object. Absolute, parent, empty and backslash
  path segments are refused. Stored objects use the real Storage API.
- GET `/data/realtime` returns `{tables,publication,rlsRequired:true}`. POST
  `{table,enabled}` adds or removes an existing public table in the real
  `supabase_realtime` publication. Application subscriptions still obey RLS.
- GET `/data/functions` is a read-only catalogue:
  `{projectId,revision,canManage,customPublishing,functions,operation?,message}`.
  Summaries include name/path/sourceDigest/authMode/workingChanged/builtin,
  active release and retained history. Draft source is available before service
  installation. The reserved `hello` example remains read-only.
- GET `/data/functions/{name}` reads the confined project's function source,
  files, digest and current Git commit/committedDigest. POST `.../{name}/source`
  saves `{expectedDigest,authMode,files:[{path,content}]}` with an optimistic
  source guard. A new source uses the empty expected digest. The fixed entry is
  `supabase/functions/{name}/index.ts`; source is limited to32 local files,
  64KiB each and1MiB total, without links, private paths or remote imports.
  The generated `luci.function.json` retains the selected access policy.
- POST `/data/functions/{name}/action` uses
  `{action,expectedRevision,commit?,releaseId?,acknowledgeExecute?,acknowledgeRemove?}`
  for publish/rollback/remove/refresh. Publish requires verified committed source,
  healthy owned data services and explicit code-execution acknowledgement.
  Rollback requires an earlier release and the same acknowledgement; it restores
  code/policy rather than database contents. Remove unroutes the endpoint while
  retaining source/releases. Accepted work returns202 with durable progress.
  Immutable artifacts are compiled/probed by the pinned native Edge Runtime
  before route promotion. A failed candidate must preserve the prior route or
  report an uncertain/degraded recovery; it cannot claim a healthy activation.
- POST `/data/functions/{name}/invoke` accepts
  `{input:JSONobject,applicationAccessToken?}` up to64KiB, returning
  `{function,releaseId,statusCode,result,durationMs,contentType?}` from the actual
  service. Authenticated is the default access policy: a real application-user
  session is required and verified. Explicit project-key functions allow the
  public project key. Luci/control or service-role credentials are not accepted.
  Legacy unwrapped `hello` requests remain supported. A timed-out request reports
  an uncertain outcome and is never retried automatically.
  Worker code receives client-safe project settings, limited data-network access
  and no private server credentials or host-command/filesystem authority.

Backups contain a real custom-format PostgreSQL dump of public/Auth/Storage
schemas and a storage file archive. Their metadata is
`{id,createdAt,bytes,sha256,includesStorage:true}`. GET
`/data/backups/{backupId}` downloads the recorded ZIP only after its checksum is
verified. The combined limit is64MiB. Signing/encryption restore keys remain in
private companion state and are not included in the downloaded ZIP. API services
briefly pause during backup; direct database clients must separately pause writes
for cross-service consistency. The UI must not call this a zero-downtime backup.

Restore requires a recorded project-owned backup, `confirmRestore:true` and the
resource acknowledgement. It validates ZIP entries, checksums, expanded limits
and storage paths, restores into fresh owned volumes, and checks the new services
before switching private application bindings. The previous stack is stopped and
retained. A failed candidate is stopped by verified ownership and retained for
diagnosis; it never overwrites the old volume. Redeploy applications to update
client-facing URLs after the switch. Backup imports, automatic volume deletion,
arbitrary migrations and public data-domain provisioning are separate operations
and are not implied by these controls.

The explicit `recover-install` data action requires `confirmFreshInstall:true`
and the resource acknowledgement. It is limited to a stopped incomplete first
installation, with no successful initialization, prior restore or backup. Both
legacy unknown state and a pending state with existing containers require owned
database logs proving the reviewed initial SQL permission failure; an absent
success receipt never proves an empty database. The cached `canRecoverInstall`
flag offers the UI action, while native preflight revalidates eligibility before
allocation. Recovery creates a fresh candidate with the same credentials and
storage identity, checks all services and database/Auth/REST/Storage, then
promotes it with verified network handover. Original volumes remain retained.
Failure stops the owned candidate or reports a degraded cleanup outcome; it does
not report a successful installation or silently reset the original database.

## Public static deployment

POST `/v1/product/projects/{id}/releases/pages` accepts `{commit,branch,acknowledgePublic:true}`. The repository comes only from the project's verified GitHub origin, and the published directory is fixed to `/docs`. The project must have a clean current HEAD matching the requested full commit and current/default branch, a matching committed brief, Dockerfile, `docs/index.html` and `docs/.nojekyll`. The GitHub repository must be public and its remote branch must match the requested commit. Public-publication policy and the explicit acknowledgement apply. This route uses the signed-in GitHub CLI; it accepts no provider token or arbitrary command/path.

The durable release has `target:"github-pages"`. Its provider record is outside project Git and contains repository/source configuration, commit, brief revision, index digest, provider build and verification time. `ready` requires the recorded provider commit, a public HTTPS response and matching committed index bytes. Queued/building/verifying outcomes are not readiness. Common history and refresh routes include Pages releases; stop unpublishes only unchanged configuration owned by that release. Pages rollback is not supported. Later default-branch pushes can republish `/docs` automatically; an older ready history entry describes its original verification and is not continuous site monitoring. Pages hosts static public output and receives no private runtime values.

## Team browser access

Team registrations live in the dedicated worker's private
`control-plane/teams/{teamId}/runtime/state/sessions/browser-previews` directory.
The browser gateway discovers these alongside personal registrations, but a
known team preview hostname alone grants no access. Every team target requires
the management access controller; missing wiring fails closed.

POST `/v1/teams/{teamId}/projects/{projectId}/browser-access` accepts `{url}` for
an already registered isolated browser URL and returns `{url,expiresAt}`. It
requires the current Luci login, project membership, editing and execution
permissions, and an active team service. Data origins additionally require
deployment and secret permissions. The requested origin must belong to that
exact team and project; it cannot supply an arbitrary upstream. An intentionally
public HTTPS deployment retains its own application authentication and does not
use this private browser endpoint.

The returned URL is a single-use invitation, valid for at most 60 seconds. Its
`/.well-known/luci-team-access` endpoint sets a host-only Secure, HttpOnly,
SameSite=None, partitioned browser cookie and removes the invitation from the
visible application address. Open a fresh invitation for a new top-level tab;
do not reuse the URL consumed by an embedded preview. Invitation/cookie values
stay out of project context, release records and application requests. The
application cannot overwrite Luci's reserved browser cookie or set a cookie
domain spanning sibling preview origins.

When the caller also has data access, the application handoff first connects
registered data origins from the same project using bounded hidden handoffs in
the same browser partition. The generated client example in Data → Connect uses
`@supabase/supabase-js` with a fetch wrapper setting `credentials:'include'` only
for its configured data origin. Existing browser clients need this option.
Credentialed CORS admits only an isolated application origin from the same
private project; preflight returns CORS metadata without authorizing a data
request. Application JWT validation and row/storage policies remain in force.
Realtime WebSockets use the same project-authorized browser cookie.

Each request revalidates its login session, exact team/project grant revisions,
permissions and service state. Logout, permission removal, project revocation
and service suspension cancel active requests and close upgraded connections.
They preserve shared running children and data for an authorized explicit stop
or cleanup. Browser grants are memory-only; a service restart requires opening
the preview again. A copied clean URL in a different browser partition does not
authenticate. Direct team control-origin preview/remote-workspace bridges remain
outside the team route allowlist.

`node scripts/team-browser-eval.mjs --browser /path/to/chromium` validates actual
Chromium iframe authentication, isolated cookies, cross-origin data API and
realtime behavior, copied-URL denial and live revocation through production
gateway handlers. It uses deterministic app/data upstream fixtures, creates no
paid resources, and writes a sanitized report under `artifacts/uat`. This is
browser transport evidence, not proof of a real Supabase installation or a
remotely hosted team's TLS/network boundary.

## Owner data-deletion requests

GET `/v1/teams/{teamId}/data-lifecycle` requires a current team owner and returns
`{team,dataLifecycle}`. Initially the lifecycle is `{revision:0,status:"none"}`.
POST to the same path plus `/request` accepts
`{expectedRevision,acknowledgeDeletion:true}`; `/cancel` accepts
`{expectedRevision,requestId}` for the exact pending request. The revision is the
current **team** revision. Both operations advance it and the independent
lifecycle revision; stale revisions, duplicate pending requests and wrong request
IDs return409. Nonowners receive403, including administrators without ownership.

A saved lifecycle is `{revision,status:"requested"|"cancelled",requestId,
requestedBy,requestedAt,updatedAt,cancelledBy?,cancelledAt?}`. The gateway supplies
identity and timestamps from authenticated authority; clients cannot supply a
completed status or private receipt fields. Request/cancel audit entries are
saved atomically with the receipt. A failed persistence write rolls back both.
Cancelling preserves the original request identity; a later request gets a new ID.
Neither operation starts workers, stops workloads, changes billing/access or
removes any project or state files.

GET `/v1/admin/team-data-requests` requires a current Super Admin and returns
`{requests:[{team,dataLifecycle}]}` for pending requests, restricted to the bound
team on a managed host. It is an operator queue read, not automatic processing.
The separate operator tool requires the exact pending request, a still-enabled
owner, stopped state writers and a current private backup before unlinking the
verified bound host's files. Its scoped receipt never claims removal of external
resources, backup archives or provider records. See
[managed data lifecycle](MANAGED-DATA-LIFECYCLE.md) for the review/command contract.

## Task starts refused before allocation

A session and its exact project-run operation may include optional
`startFailure: {stage, reasonCode, message}` after a positively recorded refusal
before `Backend.Create`. The public diagnostic uses fixed allowlisted text;
underlying authorization, filesystem and backend errors stay in private logs.
It records what failed, not permission to retry or proof that a resource lease
can be released. A later explicit ordinary-session restart clears the earlier
diagnostic for the new allocation generation.

The project swarm queue includes terminal state `failed` and `counts.failed`.
Failed starts do not count as completed/review-required work and cannot use
pending-item resume or priority actions to replay a launch. The exact child,
context and retained checkout remain inspectable. An actual failed worker may
have reviewable output; a refused start has no invented worker report. A failed
entry can still hold an execution reservation until the existing exact lifecycle
release proof succeeds. Its safe reason code remains separate from that hold.

Project overview `execution.failed` is the recorded queue count when available,
or null when unknown. Older responses omitting it are treated as unknown by the
client. Failed work can direct attention to its exact retained run. Backend
Create/Start acknowledgement failures remain unknown/recoverable because an
error response does not prove that no worker was created. Neither opening Home
nor observing these diagnostics allocates, retries or integrates work.

## Optional external context interoperability

This is a supplementary path; primary Develop boots the chosen upstream harness
inside isolated Docker locally; cloud Develop requires a supported isolated
microVM backend, which remains missing. No external-handoff route launches a
session, imports credentials, executes report commands or changes application
source. Handoff reads require project Read; creation and return recording require
current Edit, rechecked after waiting and before publication. Execute is not
required for these context-only operations.

Routes under `/v1/product/projects/{id}/external-handoffs`:

- `GET` lists bounded handoff summaries.
- `POST {selection,idempotencyKey}` captures canonical approved context for the
  existing `ImplementationContextRequest` (project, brief/plan/specification/design
  revisions and selected task IDs). It binds current private review authority
  and source. The stable request key supports exact readback, never a changed
  assignment. It returns201 for a new capture or200 for the same retained one.
- `GET /{handoffId}` reads the immutable assignment and retained unverified returns.
- `GET /{handoffId}/bundle` verifies retained files and downloads an inert ZIP with
  `manifest.json`, captured Markdown/JSON context, `source-files.json`,
  `HANDOFF.md` and `report-template.json`. No source code, private approval receipt,
  user agent-file replacement or credential is exported. Extract into a separate
  context directory; never over the working project.
- `GET /{handoffId}/review` observes current context and actual Git-visible source,
  returning `contextCurrent`, its explanation, changed-file hashes and a
  `reviewDigest`. Expected implementation changes are reviewable; changed context
  requires a fresh approved assignment. Reads do not save or execute anything.
- `POST /{handoffId}/returns {expectedReviewDigest,idempotencyKey,report}` rechecks
  current authority, source and approved context before retaining the report.
  `report` requires exact `projectId`, `handoffId`, `handoffDigest`, nonempty
  `summary`, `checks:[{command,outcome,output}]` and `notes`. The template directs
  the tool to copy identity from the manifest. Foreign identity and stale reviews
  refuse; a request key cannot name a different report. Exact repeats return the
  original receipt. All retained returns have `evidenceStatus:"unverified"`.

Reports are bounded to128KiB,50checks and20returns per handoff. Per-project
handoff history is bounded to100 records. Reaching a bound refuses new writes;
there is no automatic evidence deletion. Historical bundles remain inspectable
and byte-stable after restart or amendments. The source inventory is bounded by
the existing Git review limits. Saving a return does not integrate source,
approve context, pass checks, mark requirements complete or deploy. Existing
source review, independently executed checks and release controls remain the
next actions. Download success means transfer started, not proof that a user
retained it. A lost write acknowledgement requires explicit saved-record readback
before another mutation; the UI retains a bounded project-scoped report draft.
