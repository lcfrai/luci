# Luci application contract

Version 0.3, 2 October 2026. JSON is camelCase. Routes below are additions to the retained session API at /v1. Errors are `{error: string, message: string}`. Requests reject unknown JSON fields. A control API read never starts a session/provisioning job or creates a resource. Bounded read-only Git subprocesses are allowed. Application preview requests are forwarded to the selected application's own HTTP surface.

## Setup

GET /v1/setup/status returns:

```json
{
  "product": "Luci", "version": "0.3.0", "platform": "windows", "configured": false,
  "workspace": {"mode": "local", "connectionId": "", "host": "", "ready": true},
  "config": {"projectsDir": "absolute path", "mode": "local", "cloudProvider": "", "region": "", "domain": "", "monthlyBudgetUsd": 30},
  "checks": [{"id": "git", "label": "Git", "status": "unknown", "message": "Run the readiness check", "action": ""}],
  "cloudProviders": [{"id": "aws", "name": "AWS", "strategy": "Lightsail", "description": "One Linux machine, fixed bundle pricing"}, {"id": "azure", "name": "Azure", "strategy": "Virtual machine", "description": "One Linux machine, no Kubernetes"}]
}
```

Checks include git, docker, sandbox-image, aws and azure. Status is unknown, ready, missing or error. POST /v1/setup/check runs bounded readiness probes and returns the same status shape. Docker daemon and Luci sandbox image are checked separately. AWS/Azure are optional.

POST /v1/setup/config accepts `{projectsDir, mode, cloudProvider, region, domain, monthlyBudgetUsd}` and returns setup/status. Mode is local or cloud; cloudProvider is empty, aws or azure. ProjectsDir must be absolute. Server stores configuration atomically. Local starts without a cloud account. Saving switches the scanner to the chosen folder immediately.

POST /v1/setup/cloud/plan accepts `{provider, name, region, domain, size, monthlyBudgetUsd, sshPublicKey}`. Provider is aws or azure; name is a lowercase slug; domain is a DNS hostname without a scheme or path; size is small or standard; sshPublicKey is a valid SSH public key. Returns:

```json
{"id": "cp-...", "provider": "aws", "name": "luci", "region": "ap-southeast-2", "domain": "luci.example.com", "resources": ["One Linux virtual machine", "One persistent disk", "A public address"], "estimate": {"currency": "USD", "monthly": null, "verified": false, "explanation": "A live provider quote is required"}, "steps": [{"id": "sign-in", "title": "Connect your account", "detail": "..."}], "files": [{"name": "template.json", "content": "...", "mediaType": "application/json"}], "commands": [{"label": "Preview", "executable": "aws", "args": ["..."]}], "warnings": [], "createdAt": "RFC3339"}
```

Plans are deployable template bundles, not successful deployment claims. No resources are created by planning. Quote failures remain explicit. POST /v1/setup/cloud/create accepts `{planId, acknowledgeCosts, acknowledgeUnverifiedPrice}`. Both acknowledgements are required when the quote is incomplete; verified estimates above the reviewed budget are refused. Creation records a job before issuing provider operations. GET /v1/setup/cloud/jobs returns cached `{jobs}`; POST /v1/setup/cloud/jobs/{jobId}/refresh explicitly checks the provider. POST /v1/setup/cloud/jobs/{jobId}/delete requires `{confirmName}` matching an owned resource, while POST /v1/setup/cloud/jobs/{jobId}/dns checks whether the planned domain resolves to the recorded public IP. Provider acceptance of a request is not readiness or proof of completed deletion.

POST /v1/setup/cloud/login accepts `{provider: "aws" | "azure"}` and returns an asynchronous `{id, provider, status, message, verificationUrl?, userCode?, actionUrl?, createdAt}` job. GET on the same route returns cached `{jobs}`. AWS uses its CLI's browser login; Azure uses device-code login. Only allowlisted sign-in URLs and user-facing device codes are exposed. Raw CLI output and credentials are not returned, and credentials remain with the provider CLI. Active duplicate requests share the existing job; login expires after ten minutes and is cancelled when Luci closes.

## Project journey

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

For a persistent personal API connection, GET /v1/setup/agents returns `{providers: [{id: "anthropic", name: "Claude Code", configured: false}]}`. POST /v1/setup/agents/anthropic accepts `{apiKey: "..."}` and returns the same status-only shape. An empty key disconnects. A nonempty key must be 20–8,192 printable ASCII characters without whitespace. The explicit save stores the key only in Luci's private host state, with mode 0600 on Unix and a protected owner-only DACL on Windows. It is injected as `ANTHROPIC_API_KEY` only into new or restarted Claude sandboxes, never into shell sessions, project files, session records, GET responses or command arguments. Clearing applies to future sessions; stop existing containers to remove their existing environment. Saving a key does not make a provider call or claim that the provider accepted it. API usage is billed by Anthropic under the person's API account; a Claude subscription is a separate sign-in option. Startup and settings refuse any projects folder that contains the private state folder.

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
