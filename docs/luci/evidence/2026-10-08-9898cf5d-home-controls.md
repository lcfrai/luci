# Installed Home controls at 9898cf5d

**Passed for this scope; the local milestone remains open.** The installed Linux
web application and its real backend completed the Home queue/navigation checks
on 8 October 2026. The separate autonomous swarm witness found the missing
HTTP catalogue task-capability projection (CLOUD-006); this Home result does not
override that failure or establish agent execution.

[Sanitized machine receipt](2026-10-08-9898cf5d-home-controls.json) records source
`9898cf5d4a5302e431c8e42f5b3612c0a95e7061`, Chromium `151.0.7922.173`,
and all **968 matching package members**. The package archive SHA-256 is
`e3262a041983caac3fdc26de2d0da7b64cb016601c978a7f0698e7982781edf6`.

## Actual installed behavior

- The browser paused four queue entries, reprioritized four, and cancelled one.
  The helper compared actual queue IDs and durable revisions before and after.
- After an owner API revoked execution access for one selected project, a
  two-project browser resume produced one actual mutation and one visible
  permission failure. The revoked entry stayed unchanged; no hidden retry ran.
- Selecting an older cancelled run, returning Home, and opening the next action
  requested and displayed the exact latest run.

Accounts, grants, reviewed project context, saved Bypass run inputs and initial
queue entries were explicitly seeded through API scaffolding. Browser controls
performed the interventions. These fixture setup actions are not end-user
plan-creation or autonomous task-preparation acceptance.

## Controlled overview responses (CLOUD-003 UI scope)

Only one project’s overview GET was intercepted after an actual successful
baseline read. All queue and permission requests continued to the real backend.

- Injected HTTP409 `branch_busy` retained the prior goal and stage with explicit
  “Last recorded” and recorded-count labels; the stale next action disappeared.
- With the injected 409 still active, a browser priority change advanced the
  real queue entry revision from 3 to 4.
- Injected HTTP403 and HTTP500 cleared the private goal, stage, counters,
  context details and next action. Successful backend reads restored the summary.

The `controlledOverviewResponses` receipt distinguishes these injected replies
from actual queue mutations. **This does not prove a real server source lock or
overview permission revocation.** Native Windows slow-clone/source-preparation
behavior and actual backend lock availability remain separate acceptance scopes.

## Isolation, artifacts and reproduction

Dispatch remained globally paused. Docker used an absent private Unix socket;
no model/cloud credentials were inherited. The loopback provider trap received
zero requests. Final reads found zero reserved operations and zero child IDs.
Owned service/browser processes stopped and the private fixture was removed.
No shared runtime fixture or Docker daemon was used.

The preserved first attempt also passed. A second fresh run only corrected
screenshot targeting to show the affected third project inside the app’s scroll
container. No product source changed between the two Home witnesses. Desktop
1440×1080 and phone 390×844 screenshots were inspected; the phone state exposes
the warning, saved goal, recorded stage and usable queue without horizontal
document overflow. Screenshots supplement the checked DOM/API outcomes.

Artifact directory: `artifacts/quality-swarm/home-controls-9898cf5d-attempt2/`.
The original result SHA-256 (identical to the linked JSON) is
`4611a78c62af07d4d980d7e824b90760de4b973b36952b7938f70f2106028a2e`.
The exact executed helper SHA-256 is `ccb5a25f1da585ead6317dc17c155f69d6f51fcc4c8d075544b96031d3caab55`.
Its copied verifier SHA-256 is `e4754d0d9db7424abdd76826c390e55bcddd5b6f55fd5ad716c3cfb5af8befd5`.

| Supplementary screenshot | SHA-256 |
| --- | --- |
| `bulk-partial-desktop.png` | `1b7e577f4d2ed1ea8b34ac838aa23b84c0d431c1a22238b7692fe2673794ecf2` |
| `bulk-partial-phone.png` | `b107361bc7447cf261ffb2512450366551abd8e323a11494706d418b3a6b7e02` |
| `exact-run-desktop.png` | `328aa783c01e789a3258e6cf8f27a1a16aadfcfcfeba893f7e3421c58dc9dfa3` |
| `controlled-overview-busy-phone.png` | `894ad128bb24449c70de1bf2cb6817f2e51f365784e118ec854d74b75631b3d2` |
| `controlled-overview-busy-card-phone.png` | `850b17a67809019b115b31ddc879e8eef9d58740c52c665201584424fa65f81b` |
| `controlled-overview-denied-desktop.png` | `41c32ef00c5c17093f22eeba3a6edd35bb9f6bba4b23c14092f4c819859fe0b9` |

Use the [Home acceptance procedure](../../../scripts/acceptance/four-project-queue/HOME-CONTROLS.md)
with the matching clean package/provenance and a fresh evidence directory.
The result establishes neither live-model output quality nor task-worker
execution, parallel speed, native Windows, installation or cloud acceptance.
