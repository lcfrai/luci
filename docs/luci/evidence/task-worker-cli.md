# Native task CLI capability witness

Observed 2026-10-08 using the pinned candidate image
`sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b`.
The help-only container had no network, credentials or provider connection;
it ran as `dev`, with all capabilities dropped, `no-new-privileges`, 512 MiB,
one CPU and a 64-process limit. Commands were the installed binaries' version
and help commands, without installing or executing downloaded code:

```text
codex --version
codex exec --help
codex exec resume --help
claude --version
claude --help
opencode --version
opencode run --help
```

| Installed version | Verified task invocation | Relevant help evidence |
| --- | --- | --- |
| Codex 0.160.0 | `codex exec` | “Run Codex non-interactively”; supports model, sandbox, explicit approval bypass, config overrides and optional JSON output. `--ephemeral` is optional and Luci does not use it. |
| Claude Code 2.1.274 | `claude --print` | “Print response and exit”; accepts `--permission-mode bypassPermissions`, `--session-id` and `--append-system-prompt`. Print permission prompts require a host; `none` denies them automatically. |
| OpenCode 1.18.34 | `opencode run` | Interactive mode defaults to false; accepts `--pure`, `--agent`, `--model`, positional message and `--auto` (“auto-approve permissions that are not explicitly denied”). |

Luci keeps config overrides before Codex's `exec` subcommand, including the
provider and captured-context overrides. Its pinned parser replaces root `-c`
values when subcommand `-c` values are supplied; builders and allocator tests
guard that boundary. Native conversation mounts remain persistent. A swarm
attempt cannot resume or restart an old worker; continuation requires a fresh
reviewed operation. Ordinary terminal argv remains unchanged.

## Permission boundary

Codex tag `rust-v0.160.0` resolves to commit
[`a956835d020762cb2b570053af06f643a11c0ecc`](https://github.com/openai/codex/tree/a956835d020762cb2b570053af06f643a11c0ecc).
Its [exec implementation](https://github.com/openai/codex/blob/a956835d020762cb2b570053af06f643a11c0ecc/codex-rs/exec/src/lib.rs#L574)
sets `approval_policy: Some(AskForApproval::Never)` in headless mode, and its
server-request handler rejects interactive approval and user-input requests.
The available automatic-review option does not preserve interactive Ask consent.

Therefore Luci only enables autonomous tasks for an **explicitly saved Bypass**
route. Ask and Plan fail before swarm preparation, reservation or enqueue with
`task_permission_requires_interactive`. The app does not change the saved mode.
A native approval bridge remains necessary to support autonomous tasks with
interactive permission decisions. Process exit means **review required**;
neither exit nor provider text proves task correctness or approval completion.

## Receipt and limits

The exact owned container `luci-task-mode-help-20261008` exited with code 0 and
was removed by name. Filesystem free bytes were 7,088,590,848 before allocation,
2,353,303,552 before cleanup and 7,088,553,984 after cleanup, satisfying the
6 GiB preallocation and 1 GiB retained-floor guards. No broad cleanup occurred.

Development artifacts (not required to run the app):

- `artifacts/research/task-worker-lifecycle/pinned-cli-help.txt`, SHA-256
  `2f0fba696e5a010c911053b912074898987ff01ef263165ed602fd3b26d7a5aa`.
- `artifacts/research/task-worker-lifecycle/help-witness.json`, SHA-256
  `29690e480682f52a2be358b9ada3b36cfa554d3b5876d8f18202a344a32789bc`.

This witness establishes installed versions and documented command/permission
semantics. It does not establish live-provider quality, successful tool use,
native Windows compatibility or natural task exit in an end-to-end workflow.
Those require their separately recorded installed-runtime acceptance runs.

## Launch and capacity boundary

Each allocation holds its exact session's start/stop exclusion until the backend
call finishes. The durable `backend-dispatching` lease is written before the
backend call; `backend-settled` is written only after the entire successful
`Backend.Create` returns, including the Docker create and start RPCs. Capacity
release, inventory reconciliation and Git review cannot infer settlement from an
early missing container. The final launch guard also checks that the exact
registry child remains in its starting state.

A guarded claim that demonstrably never crossed the launch edge can use the
existing explicit missing-child retirement flow. A timed-out or interrupted
create/start call remains uncertain even when a later lookup and inventory are
empty: they cannot prove a late worker will never appear. Its lease and checkout
remain held across controller restart. If successful allocation cannot durably
record its acknowledgment, Luci attempts cleanup of only its originally bound
child and records an explicit failure, retaining the uncertain reservation.
This is not a claim of complete automatic recovery for ambiguous backend RPCs;
such recovery needs an authoritative backend request fence or equivalent proof.
