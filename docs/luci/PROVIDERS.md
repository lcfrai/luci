# Agent providers and runtime evidence

Luci delegates execution and conversation formats to upstream CLIs. The sandbox image pins Claude Code `2.1.274`, OpenAI Codex `0.160.0`, and OpenCode `1.18.34`. Gemini remains unavailable. Go 1.26, Node 24, and Debian Bookworm's Python 3 plus `venv` are included for project checks; the runtime smoke records the actual installed versions. Image tags for base toolchains receive maintenance patches and are not digest-pinned reproducible releases.

## Connections

- Claude uses the saved personal Anthropic API key, or its own interactive `/login`. Existing Claude transcript behavior is preserved. Its interactive sign-in credentials remain container-local; the saved API connection is the durable option.
- Codex accepts a saved OpenAI API key through its documented environment-backed provider configuration. The key is never placed in command arguments. Alternatively, an `authOnly` Codex session runs the official `codex login --device-auth` flow. Follow its terminal instructions, then restart that session to begin normal work with its retained account credential. Device sign-in availability and billing depend on the person's OpenAI account/workspace. Luci does not implement a substitute OAuth flow.
- OpenCode uses an explicitly configured local or OpenAI-compatible endpoint and model ID. The selected model is also used for small-model tasks, and `enabled_providers` allows only that connection. There is no fallback to a paid model. Ollama's compatible endpoint is commonly `http://localhost:11434/v1`; the model must already exist on that server. A remote endpoint can charge for its usage. Configuring a connection neither contacts nor downloads a model.

OpenCode's upstream default allows most tools, so Luci explicitly sets its documented permission environment for Ask, Plan and Bypass, selects the build/plan agent, and disables project configuration with the pinned CLI's verified flag. The fixture checks that a project config requesting permissive tools cannot override those settings. These CLI approval controls supplement Docker isolation; they are not a separate security sandbox.

Settings stores API credentials in private state outside the projects folder. GET returns connection presence, endpoint and model, never the key. `configured` means saved configuration, not a successful account/model acceptance test. Empty credentials disconnect future sessions; stop already running sessions to remove the environment they received.

Container `localhost` refers to the container. For explicitly configured loopback model endpoints, Luci translates the host to `host.docker.internal` and supplies Docker's `host-gateway` alias. The model server must listen on an interface reachable from Docker; a Linux server bound only to host loopback will require a deliberate interface/firewall adjustment. On a cloud workspace this is the cloud host, not the laptop. There is no automatic exposure of a laptop model port or installation of models on a small cloud host.

## Durable sessions

Every Codex/OpenCode session receives its own writable home mounted at `/home/dev/.luci-agent`. The host directory is outside project Git and outside Claude's transcript mount. Native histories are copied only from stopped sessions, then continued with the upstream fork option. The source database is never edited by Luci. Copies reject links, special files, and hard links, and are bounded to 4,096 files/256 MiB. Large or malformed histories return an explicit error.

Restart means clean context and copies only the native credential file into a fresh home. Earlier histories remain resumable until Forget. Forget deletes only the recorded private homes; a previously forked session remains independent. OpenCode and Codex use their own session IDs internally. Luci's UUID names its private home and is never substituted as a native conversation ID.

Codex's database contains absolute rollout paths. Every Docker sandbox therefore uses the same in-container `CODEX_HOME` path even when its backing host directory differs. A future native-host implementation must address this constraint; moving a copied Codex home to an arbitrary host path does not work. Native host execution is not an automatic fallback or an isolation guarantee.

## Verified scope

On 2 October 2026, the actual pinned Windows CLI executables completed local fixture requests and forked copied state: Codex used the Responses API with an environment-supplied fixture key; OpenCode used streamed Chat Completions with the explicitly selected fixture model. No real account/model credential, paid inference, or cloud resource was used. Codex's fixture emitted unknown-model metadata and optional plugin-cache warnings; the completion and fork both exited successfully.

The fixture server supplies deterministic responses and tool calls; no language model runs. The actual CLI must perform the file writes: Codex uses its advertised command tool and OpenCode its advertised write tool. The test checks the resulting Node app and verification file, starts that generated app, and requires loopback HTTP 200. OpenCode's two real writes, authenticated fixture requests, generated HTTP 200 and independent history fork passed on Windows. Codex's Windows completion/fork passed, but its host command was refused by policy; no host development success is claimed.

`scripts/provider-smoke.py` repeats those checks in the Linux image. Its development turn explicitly selects Luci's supported **Bypass** mode inside Docker and refuses to run outside Docker. `scripts/runtime-smoke.mjs` supplies the outer boundary: networking disabled, read-only root filesystem, all capabilities dropped, no-new-privileges and a disposable writable directory. Ask/configuration checks are separate from this explicitly permitted write turn.

Evidence records the actual CLI/toolchain versions, fixture authorization, tool outcomes, app hash/HTTP status and unchanged source history after a fork. Partial outcomes are retained when a later provider fails. A fixture pass proves the tested configuration, protocol, persistence and tool execution; paid-model quality, real account sign-in and interactive approval UI need separate acceptance. The current commit's successful Docker CI result is required to claim the full Linux image journey passed.

Upstream references: [Codex authentication](https://developers.openai.com/codex/auth/), [Codex configuration](https://developers.openai.com/codex/config-reference/), [OpenCode CLI](https://opencode.ai/docs/cli/), [OpenCode configuration](https://opencode.ai/docs/config/), and [OpenCode providers](https://opencode.ai/docs/providers/). Glasses discovery was performed before implementation; no unverified proxy or credential bridge was adopted. The official CLIs provide the required functionality. OpenAI Codex is Apache-2.0 and OpenCode is MIT; upstream grants remain distinct from Luci's own MIT license.
