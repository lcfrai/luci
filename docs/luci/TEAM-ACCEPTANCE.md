# Team acceptance checks

These checks use generated accounts and private temporary state. They do not call external model providers, create cloud resources, or establish production host isolation.

Build and exercise the actual main service and its supervised team workers:

```sh
cd control-plane
go build -o ../artifacts/luci-team-acceptance ./cmd/api
cd ..
python3 scripts/team-main-smoke.py --binary artifacts/luci-team-acceptance
```

On Windows, build the executable with an `.exe` suffix and invoke the script with `python`. Use `--output` to choose the JSON receipt location; its default is `artifacts/team-main-smoke.json`. The receipt records the binary SHA256 and each assertion. Windows process handling is implemented, but the current runner acceptance was executed on Linux.

The runner starts its own service on an available loopback port, creates an owner and editor, verifies explicit per-project access, shared context, stale-save rejection, real author attribution, personal workspace separation, restart persistence, and membership removal. It shuts down only its own process group and removes its generated fixture. The environment excludes inherited provider, cloud, GitHub, and SSH-agent credentials. Git and the compiled service must be available; this context-only journey requires no running containers.

`--fixture-parent` chooses where to create a fresh private temporary directory. Existing state is never reused. `--keep-fixture` retains diagnostic logs and state containing generated test credentials; keep that directory private and remove it when diagnosis is complete. `--docker-host` is optional and does not cause this runner to create containers.

Exercise actual browser handoffs, isolated app/data origins, authenticated fetches, realtime sockets and revocation separately:

```sh
node scripts/team-browser-eval.mjs --browser /path/to/chromium
```

That runner requires the repository Go toolchain, installed desktop dependencies and a Playwright-compatible Chromium. Its application and data upstreams are deterministic fixtures behind the production management and preview handlers. It does not substitute for deploying and verifying a real managed host, TLS, backup recovery, or hostile workload isolation.

Exercise a reviewed implementation through the actual team worker and installed Codex CLI, then independently check and release its output:

```sh
node scripts/team-run-smoke.mjs --binary artifacts/luci-team-acceptance --image luci/sandbox:dev
```

This Linux check requires the ordinary Docker-enabled host user and an explicitly selected, already installed trusted runtime image. Optionally pass `--release-image node@sha256:<verified-digest>` for an already installed small official Node image; this keeps the same generated app while reducing Docker storage during release. The receipt records both immutable image IDs and the release image repository digest. The runner never pulls either image. It creates two accounts, records definition/design approvals by different people, reserves a project run, and dispatches its normal session API. A deterministic Responses fixture listens only on the local Docker bridge and receives no real model key. The actual CLI must receive the bound task prompt and read the exact reviewed context bytes in its assigned worktree before its tool creates the fixture application. The runner acknowledges only the trust prompt for that newly generated fixture folder. It then runs an independent Docker HTTP acceptance check, joins a local release from the same clean candidate commit, explicitly accepts the run, and verifies access/socket revocation and owned resource stop. This tests protocol, orchestration and execution; it does not evaluate model reasoning or claim public deployment.

Use `--report` to choose the sanitized JSON receipt and `--keep-fixture` to retain private diagnostic state. Failure retains the private fixture automatically. Cleanup inspects exact recorded session/release labels before removing containers; it never deletes unrelated Docker resources or reuses existing application state. No caller model, cloud, SSH, GitHub or registry credential is imported.
