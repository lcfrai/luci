# Astra readiness review — 6 October 2026

Luci has substantial working product infrastructure, but the full product is not
ready for an unrestricted completion claim. This review found a reproducible
default Codex execution failure and several disconnected model-access paths.
Those defects matter more than adding another autonomous agent or visual layer.

The review baseline is `eda1d061d679e4051e720b7ff98f67a760c270dc` / 0.4.16.
Line references below describe that baseline; concurrent fixes may move them.
The latest owner-approved scope is [GOAL.md](GOAL.md), including optional lcfr.ai
LiteLLM access, personal subscription/API choices, and general CLI file editing.
This is a bounded review of each core area, not a claim to have inspected every
line or found every defect. Source inspection, existing receipts, new fixture
execution and missing external acceptance are distinguished throughout.

## Verified findings

No P0 defect was established in this review. That is not a security certification.

| ID | Severity | Verified problem and effect | Implementation / acceptance requirement |
| --- | --- | --- | --- |
| CLI-01 | P1 | Normal Codex Ask uses `workspace-write` (`internal/harness/argv.go:89`). Its Linux sandbox cannot create its namespace inside Luci's non-root, cap-dropped Docker boundary (`internal/backend/dockerbe/docker.go:223`). A real pinned CLI fixture invoked its command tool, failed with the bwrap namespace error, created no application, and nevertheless exited **0**. | Keep the outer Docker boundary. Apply a Docker-specific supported inner-sandbox setting while retaining the selected approval policy; do not globally relax host execution or enable privileged Docker. The successful fixture used `--sandbox danger-full-access --ask-for-approval on-request`, without the combined dangerous-bypass flag. Test file bytes and application behavior, not just CLI exit. |
| CONN-01 | P1 | There is no shared configured gateway connector at baseline. `product/agent_credentials.go:164` injects separate direct Anthropic/OpenAI keys; its compatible endpoint is only passed to OpenCode. `api/sessions_native.go:227` hardcodes the OpenAI API endpoint. Selecting Codex or Claude cannot reuse a saved LiteLLM/compatible connection. | Add a per-account explicit active connection, supported protocol/model bindings and private credentials. Use the same resolver for all coding/assistance entry points. Enable, disable and switch deliberately; never silently substitute a subscription or paid provider. No live lcfr.ai endpoint has been supplied or verified. |
| CONN-02 | P1 | Define repository analysis exposes an explicit Codex connection-method selector (`RepositoryAnalysisPanel.tsx:119`) and requires `authMode`. The guided Define assistant instead requires a saved Codex ChatGPT connection (`api/assist.go:174`, especially 205–211; `assistance/assistance.go:233`). Therefore a working Claude/OpenCode/API connection does not power the complete Define experience. | Resolve connection and protocol on the server from the selected harness/model. Remove the everyday login-method selector. Keep official subscription sign-in as a genuine separate external protocol. Update `management/policy.go:123` admission too: it currently identifies every guided helper as Codex. Preserve per-harness policy and quota enforcement. |
| MODEL-01 | P1 | Develop cannot faithfully select/preserve all models already accepted by its backend. The Codex catalogue offers only `default`; `Descriptor.freeTextModel` is private, `store/defaults.ts:53` clamps unknown IDs to the default, and `SessionPicker.tsx:119` only renders a select. A configured gateway alias or explicit Codex model is lost or unavailable. | Export truthful model-selection capabilities or a resolved connection model catalogue. Preserve the chosen supported model through project defaults, new runs, restart and assistance; report the actual resolved model in run provenance. Do not infer availability from an arbitrary typed ID. |
| CLI-02 | P2 | Shell is real and its project directory is writable, but supported harnesses remain a fixed catalogue. Gemini is explicitly unavailable. Shell/custom CLI homes are not durable: `harness.go:210` gives private native state only to Codex/OpenCode; `sessionMounts` only mounts project/transcript/selected native state. A CLI installed or configured under the shell's home can disappear on recreation. | A general shell already supports ordinary project commands; do not describe that as arbitrary persistent harness support. A custom CLI profile needs a bounded executable/argv contract, capability declaration, private durable home, explicit connector opt-in and installation/version verification. Never grant a profile host execution or inject every credential into Shell. |
| GIT-01 | P2 | Linked-worktree Git is deliberately unavailable inside CLI containers: the `.git` indirection points to host metadata absent from the sandbox. `LinkedWorktreeNote.tsx:14` explains this, and `sessions.go:630` protects the primary checkout's linked metadata. Main-checkout branch switching is the existing recovery path. | Retain the warning and working main-checkout route. Full parallel Git-agent claims require a tested, confined linked-worktree metadata solution; mounting the entire host repository administration tree is not an acceptable shortcut. |

Backend paths in the table are shorthand for `control-plane/internal/` (or
`control-plane/` when already prefixed with `internal/`). Frontend paths are
under `desktop/src/`; component basenames above are in
`components/definition`, `components/topbar`, or `components/develop`.
The coordinator has assigned the connector/resolver, assisted Define, UI model
selection and Docker Ask corrections. Assignment is not evidence of completion.

## New execution evidence

Two explicitly authorized disposable containers used the installed sandbox image
`sha256:8949aaa1b80e52bcb325feef29866c0cb54e1f2e3b393f3a9bb5975b5db1afe1`
and Codex CLI 0.160.0. Both used UID/GID 1001, dropped all capabilities, enabled
no-new-privileges, retained default Docker seccomp, disabled networking except
in-container loopback, and used a new private writable tmpfs project. Neither
mounted host files, installed instances, account state or retained volumes.
Cleanup removed only the exact new container after matching its ownership label.

- `artifacts/astra-review-20261006/cli-boundary-probe.json`: Bash created a nested
  project file with exact verified bytes. Explicit Codex Linux sandbox execution
  failed before creating a file.
- `artifacts/astra-review-20261006/codex-ask-fixture.json`: a deterministic local
  Responses fixture caused the actual pinned CLI to call `exec_command`.
  `workspace-write` failed with no output artifact. The Docker-contained
  `danger-full-access` alternative retained `on-request` and produced the exact
  application and verification files; the produced application answered HTTP 200.
  A system-directory write was denied, the process was non-root, and the Docker
  socket was absent.

This is protocol/tool execution evidence, not paid inference, an official
subscription benchmark, or an interactive approval-prompt test. The earlier
`scripts/provider-smoke.py:242` write fixture explicitly used Bypass; it did not
cover the default Ask defect. Preserve both successful and failed receipts.

## Coverage across the complete product

| Goal area | Source paths inspected and conclusion | Remaining acceptance |
| --- | --- | --- |
| Account, onboarding and recovery | `management/auth.go:138`, `local_access.go:145`, `runtime.go`, `product/server.go:413`, `setup_tools.go`, `SetupPanel.tsx`. Authentication checks expiry/disabled state; owner recovery revokes existing access; workers have allowlisted environments and private account roots. Readiness probes require successful nonempty output and installation does not itself claim execution ready. | Matching installed first launch, returning account, recovery, dependency preparation/failure/retry and remote workspace reconnect. The current native offscreen-control failure belongs to the active native-driver recovery work. No newly proved auth bypass was found here. |
| Projects and navigation | `AppShell.tsx`, `EmptyDevelop.tsx`, `ProjectPlan.tsx`, `product/backup.go:122/371`. Project/source revision changes remount phase views; Projects home and create/import/restore exist; backup entries are bounded and path-validated. Backups intentionally exclude current credentials and runtime databases. | Installed restart and account/project/source switching, unsaved-edit recovery, real import/restore and appearance persistence. A project backup must not be presented as a full database/runtime backup. |
| Define | `api/assist.go`, `repository_analysis.go`, `assistance/repository.go`, `DefinitionWorkspace.tsx`, `RepositoryAnalysisPanel.tsx`. Repository analysis uses a bounded source snapshot, separate utility workspace, disabled tools and reviewed draft application. | Fix CONN-02/MODEL-01; prove chosen connection/model, failed connection recovery, stale-source refusal, cancellation/restart and cross-account isolation with actual supported providers. |
| Design | `product/design_plan.go:157/230`, `DesignWorkspace.tsx`, `prototype_releases.go`. Screens, states, responsive widths, tokens, assets, linked requirements, sketch revision and explicit handoff are stored. Prototype inspection uses an actual recorded release. Design is currently manual planning/sketching plus the running prototype; it has no model design assistant. | An installed meaningful interaction prototype and accurate implementation handoff, not only a saved drawing. Approval is deliberately not proof that implementation works. A model-driven design assistant or generated asset is not already present. |
| Develop, CLI, Git and verification | `harness/*`, `api/sessions*.go`, `sandboxd/internal/ptysession`, `backend/dockerbe`, `ProjectPlan.tsx`, `product/project_checks.go`. Real PTYs, isolated project mounts, native provider history, requirements linkage and source-change invalidation exist. Fix CLI-01 and MODEL-01; retain CLI-02/GIT-01 limitations. | Default Ask command/file creation, edit/delete within an owned disposable project, restart/reconnect, same-account project isolation, account isolation, Git and source-linked checks. The required actual Codex-subscription implementation must be made by that agent. |
| Application/data platform | `platform/data_function_runtime.go:38/53/359`, `data_install_recovery.go`, `database_restore.go`, `product/app_config.go`, `cloud_app_config.go`, platform UI. Public function environment is explicitly allowlisted; private administrator/database credentials are excluded; function network access and resource ownership are checked. Public client settings and private runtime values are distinct. | Current installed database/Auth/RLS/Storage/Realtime/functions/email journeys and negative access cases. Existing 0.4.16 receipts are scoped evidence, not newly repeated by this review. Custom functions intentionally exclude arbitrary external imports/packages; SMTP configuration supports verified TLS on 465, not STARTTLS-only providers. |
| Deploy and cloud | `product/releases.go:142`, `remote_deploy.go`, `cloud.go`, `cloud_jobs.go:91`, `workspace.go:492`, `DeployLens.tsx`. Local builds require a clean exact commit; cloud records reserve IDs before mutation, preserve uncertain outcomes and verify configuration before deploying. Workspace APIs route together through the selected connection. Cost acknowledgements, plan expiry and current policy checks precede cloud creation. | Live AWS and Azure credentials, fresh regional quote/resource acknowledgement, SSH trust, owned server installation, DNS/TLS, real application/data health and owned-resource cleanup. No actual cloud account/domain proof was performed. A Vercel deployment adapter or Hermes/Vercel feature parity is not implied by the existing Coolify/local/Pages paths. |
| Maintenance | `platform/database_restore.go:39`, `data_install_recovery.go:74`, `data_backup.go`, `product/backup.go`, `cloud_maintenance.go`, `cloud_jobs.go`. Backup bytes are admitted by digest/ownership; interrupted restore retains candidates; fresh-install recovery refuses known complete/unknown unsafe states and keeps original data. | Installed restart, failed-first-install recovery, exact restored records/files, stopped-service recovery, supported rollback and stop, stale/uncertain remote outcomes and no accidental volume removal. Root-held historical/current receipts must remain bound to their actual package. |
| UX and accessibility | `AppShell.tsx`, `SetupPanel.tsx`, Definition/Design/Develop/Deploy views, vendored Fluid Button/Tabs. There is a coherent project route and meaningful empty/error/recovery copy. The manual login selector, disappearing model choice and unsupported-harness developer instructions are concrete friction points. | One bounded installed visual/keyboard/responsive/accessibility pass after the fixes. This source review does not certify visual polish or replace the UI owner's actual rendered inspection. Prefer existing Fluid controls and calm connection summaries; reveal provider/runtime detail deliberately. |
| Distribution | `.github/workflows/luci-ci.yml`, `scripts/build-release.ps1:45/166`, `prepare-docker-release.py:54/115`. Release preparation checks clean matching source, archive hashes, required assets and licence manifests. | Matching native/portable/Docker packages, supported clean-host installs and upgrade/restart retention. Public download, signing and a genuinely published matching build are still external release gates. Build success is not native install success or public availability. |

Command checks use a read-only container root filesystem but a writable project
bind (`project_checks.go:346`). They are explicitly trusted project commands and
record source changes. Repository-analysis tools are separately restricted.
Do not confuse either path with the normal writable coding workspace or promise
that arbitrary test commands cannot alter their own project.
Interactive `dockerbe.Create` does not set `ReadonlyRootfs`: its image layer is
writable according to the unprivileged user's permissions and is ephemeral.
The durable terminal-home fix preserves that existing boundary rather than
claiming that interactive sessions now have a read-only root filesystem.

## Terminal-home remediation

`api/sessions_shell.go` adds an account-private, compact home derived from the
worktree and session identities outside project files. A restart retains that
home; a new terminal gets a separate home. Kill retains tools and configuration,
and Forget removes the home only after the owning container has stopped.
Host-selected state roots reject symlinks/reparse points and project locations;
root-anchored removal cannot follow child links outside the home. Unix mode 0700
and the existing Windows private ACL helper protect the host directory.

The terminal sets HOME, XDG and package-manager user/cache locations to its
assigned mount. A user-editable Bash profile is created exclusively on first
creation, so installed executables are on PATH without replacing a user's
profile at restart. The image bootstrap preserves only this fixed terminal
HOME after mapping the host UID; it never copies host profiles or credentials.
The resolver supplies no model credentials to Shell. Installing or invoking a
CLI there does not turn it into a supported connector-injected harness.

Focused Windows helper and integrated HTTP lifecycle tests passed for retained tool/config bytes, separate
sessions/worktrees/accounts, invalid identities, project-root refusal, junction
escape and child-link-safe removal. The lifecycle tests exercised the HTTP
create/restart/kill/forget route and proved that unsafe home validation fails
before destroying the running terminal. The production
Bash helper and installed executable PATH test runs on Linux; Windows skips that
Linux-image test explicitly. Release-level container execution and installed
acceptance remain separate gates.

## Connection and security contract for the fixes

Keep connection identity, kind, revision, protocol and resolved model in public
run receipts; keep keys only in private account state/environment. New and
restarted sessions must resolve intentionally. Missing, disabled or incompatible
explicit selections must fail with a recovery action, never fall through to a
different billed account. Changing a saved key cannot revoke an environment
already delivered to a running container; the UI must state the effective scope
or stop/restart affected sessions through an explicit product operation.

LiteLLM is the owner-selected gateway, so no extra gateway service is required.
Codex requires the Responses protocol, Claude requires an Anthropic-compatible
gateway interface, and the pinned OpenCode adapter currently uses a chat-compatible
interface. A configured URL/key is not a verified tool-capable connection.
Azure OpenAI/Bedrock behind LiteLLM remain gateway-side choices; Luci must use the
gateway's supported aliases/protocols without inventing upstream compatibility.
[LiteLLM client setup](https://docs.litellm.ai/docs/proxy/client_setup/overview)
and [Codex setup](https://docs.litellm.ai/docs/proxy/client_setup/codex_cli)
document these client connections. Current documentation may describe features
newer than Luci's pinned clients; check the pinned binary/source before enabling.

The pinned Codex CLI/config enum accepts read-only, workspace-write and
danger-full-access; `external-sandbox` is not a valid CLI/config mode for this
version even though a lower-level protocol type has that name.
[Pinned source](https://raw.githubusercontent.com/openai/codex/rust-v0.160.0/codex-rs/protocol/src/config_types.rs).
For Claude, gateway credentials and personal subscription access are distinct;
pointing a URL at a proxy does not create a universal reusable login.
[Official gateway documentation](https://code.claude.com/docs/en/llm-gateway).

Connector acceptance must include per-account isolation; no credentials in
session/job DTOs, argv, project/Git files or diagnostics; key rotation/removal;
connection disable/switch/restart; incompatible harness/protocol refusal;
localhost resolution inside Docker; and an actual CLI tool round trip for each
advertised protocol. No host CLI auth cache should be silently imported.

## Orchestration and reuse decision

`ProjectPlan.tsx:131` currently creates a handoff prompt and opens the session
picker. Tasks are human-recorded; individual operations are durable, but there
is no model-driven cross-phase project coordinator. A stronger planner is useful
for hard decomposition and review, but a particular model brand must not become
a mandatory dependency or security authority.

A useful next coordinator is optional and persists project/source/specification/
design revisions, connection receipt, stage, child sessions, check runs, release
IDs and retry/cancel state. It invokes existing typed operations and reconciles
their real results. It cannot self-approve cloud cost, publish through a hidden
fallback, or treat an agent's text as evidence that tests/deployment succeeded.
Fix connector consistency and ordinary CLI execution before adding that loop.

Glasses discovery/inspection considered:

- **Hermes**, catalogue `ec2b8bd43eefd65f9cb1`, source revision
  `6b2fe92af66a95ea6a5caa309e05c5643cdd79de`: classified as a replacement agent /
  CLI candidate, with a needs-review assessment. The catalogue's MIT claim is
  metadata only; exact licence text must be checked before adoption. An optional
  pinned harness inside Luci's existing container is a plausible fit. Do not
  mount the host Docker socket or reuse shared homes across Luci users.
  [Hermes configuration](https://hermes-agent.nousresearch.com/docs/user-guide/configuration)
  describes its own compatible providers, terminal backends, persistence and
  approval rules; these do not replace Luci's policy boundary.
- **GoModel**, catalogue `fbad765ae7c0ca2f4313`, was inspected as a possible API
  gateway. Adding it alongside the chosen LiteLLM service would duplicate the
  immediate need. No gateway or agent package was installed/adopted by this review.
- Existing **Fluid Functionalism** controls remain the appropriate UI foundation;
  there is no reason to replace the shell with a new cosmetic template.

## Completion gates

The next package must demonstrate resolved connection/model continuity through
Define and Develop; actual default-mode coding; retained source/history after
restart; honest failure/retry; and account/project isolation. Then execute the
complete installed requirement → design/prototype → agent implementation →
verified commit → deployment/maintenance journey. Preserve current evidence
against exact source/package identities in [FULL-PRODUCT.md](FULL-PRODUCT.md).

The real Codex subscription, supplied LiteLLM endpoint, live AWS/Azure accounts,
owned domain, external SMTP recipient/inbox and public release/signing remain
distinct external prerequisites. Their absence is neither a code defect nor a
pass. The fixtures above do not satisfy those live acceptance outcomes.
