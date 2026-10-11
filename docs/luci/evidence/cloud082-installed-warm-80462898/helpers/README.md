# Prepared warm multistage preview acceptance

Status: prepared only. No Luci controller, Docker build/pull/tag/container or application server was launched while preparing this directory. Root will run the helper only after the exact 80462898735a191cceaa4cb33e486b42b0a35f3c package is independently verified, with no concurrent performance measurement.

This is a handwritten Node calculator fixture using the already cached immutable node:22-alpine image. It is not model-generated work, a cold-base test, the Luci runtime installer, a passing project check, Windows evidence or full product acceptance.

## Intended execution

Use the activated Node 24 toolchain. Fill a **new** reviewed JSON configuration using `run-config.template.json`. Do not substitute an older binary or lower the production policy. The template deliberately lacks executable identities and has `acknowledgeFixtureMutation: false` and `acknowledgeWarmBaseOnly: false`.

```
node /workspace/.luci-tools/cloud082-warm-preview-80462898/preview-smoke.mjs /absolute/path/to/reviewed-run-config.json
```

The normal controller/API helper creates a fresh private state, project and bootstrap owner below `evidenceParent`. It never imports provider credentials. Its API workflow saves the brief, specification and design and approves both, writes the source fixture, commits through reviewed Git API, sets project configuration and verifies stale-commit/dirty-source refusal. It then requests exactly one preview with the reviewed commit. The release request is bounded to 195 seconds; work has a 7 minute 15 second deadline with a 45 second cleanup reserve and an 8 minute hard-stop receipt. No request replay is added.

The daemon host is fixed to `unix:///var/run/docker.sock`; current engine identity, driver and immutable cached image ID must match the reviewed config. No helper pull/tag command exists. The Dockerfile uses the exact image ID twice, so absent image IDs must refuse in production. Do not present this as independent captured Docker-wire proof that no ImagePull was called: the evidence is source-bound selection plus an unchanged existing base identity/tag/digest inventory. Root may add an external observation wrapper if needed without changing the fixture.

## Actual build oracle

Only the four `.mjs` sources, Dockerfile and a fresh nonce/source-hash manifest are committed. Neither `dist`, TAP nor `build-proof.json` exists in the reviewed context. The first stage executes:

```
RUN node --test --test-reporter=tap calculator.test.mjs > test-output.tap && node build.mjs
```

Ten tests cover requested decimal arithmetic, signed/zero amounts, missing/empty/invalid/nonfinite/nondecimal input and overflow. On test success only, the build script checks the source hashes and completed TAP, records read-only cgroup observations and creates `dist`. The second stage copies only `dist` from the completed prior stage. A per-run nonce and production `NoCache` setting prevent reuse of an old fixture proof; no host compile/build output is copied into this context.

The app's `/build-proof` must match the fresh nonce, all four source hashes, Node 22/Linux architecture and zero-exit TAP. It must expose observed RUN limits of 1 GiB memory, one CPU (100000/100000 quota/period) and disabled swap using actual v2 or v1 cgroup files. Unavailable or different observations fail this fixture; missing values are not fabricated. This reads limits, not peak resource usage or an atomic host admission snapshot. It does not assert a build cgroup PID cap: production uses the `nproc` ulimit, distinct from the serving cgroup's PID limit.

The helper samples only this project's existing lease rows at 50 ms while the single preview POST is pending. It must actually observe one charged build allocation with the exact release/commit/source/resources, no duplicate build identity, and at most one active build-or-serving allocation per captured snapshot. The sampling is not exhaustive and cannot prove every transition timestamp. Raw private ledger SHA and allowlisted row snapshots are retained. No observer mutates a reservation.

Normal image/container inspections verify the build labels, immutable output ID and serving 1 GiB/one CPU/256 PID limits, disabled swap, dropped capabilities, no-new-privileges, non-root UID, no mounts and loopback bindings. HTTP must return the calculator form, `12.5 + 7.25 = 19.75`, the project greeting, no model key and UID 65532; malformed input must return 400.

Normal project Stop must settle exactly the preview build/serving lineage, establish owned container absence and clear only this fixture's lease rows. The reusable image/cache remains; no global cleanup or manual ledger modification occurs. On failure, a single known release is stopped through its normal API. Lost response only permits a read-only exact release lookup; no replay. Retained unknown work remains explicit. Controller shutdown verifies captured exact PID/start identities; root must still perform an independent closure check, preserving zombies/unreadable observations literally.

## Preparation checks

- `node --check preview-smoke.mjs` and `node --check fixture/build.mjs`: pass.
- Local host Node 24 fixture tests: 10/10 pass. An initial default spec reporter output is retained separately; a subsequent explicit TAP invocation produced `local-fixture-tests.tap`. These are **not Docker RUN evidence**.
- Exact extracted helper proof-validator tests: 9/9 pass using synthetic identity/cgroup observations and the local TAP. Covers nonce/source/TAP/nonzero test mismatch, unavailable/unlimited memory/CPU, enabled swap and equivalent exact v1 readings. This is controlled helper logic only.

`source-bindings.json` and `prepared-manifest.json` bind the prepared bytes. Config placeholders must be replaced with verified current identities; the final reviewed run config must be retained separately. The original C080 helper is preserved byte-for-byte for diff/review.
