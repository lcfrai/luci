# 02 — The agent-fleet cockpit and git lanes: the sceptic's verification

**Status:** IN PROGRESS (skeleton, 2026-09-30).
**Date:** 2026-09-30. **Verifies:** `02-fleet-cockpit.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (Orca as the cockpit, lanes in our Kata
sandbox through a recipe, landing through the git host) and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

_To be written._

## 1. What was checked, and how

_To be written._

## 2. Orca: licence, maintenance, and the claims at source

Source: the blobless clone at `59b746ff` (2026-09-29) already in this session's scratchpad, blobs
read with `git show`; the GitHub API (it answered this pass); onorca.dev; the YC company page.

**Licence: MIT, holds.** `GET /repos/stablyai/orca/license` returns `spdx_id: MIT`, path
`LICENSE`, "Copyright (c) 2026 Lovecast Inc." `package.json` has no `license` field. The only
other licence files in the 30,735-file tree are vendored (ripgrep's set,
`mobile/packages/expo-two-way-audio/LICENSE`) and the Geist font (SIL OFL 1.1, in
`docs/site/THIRD_PARTY_NOTICES.md`). No copyleft file found. MIT lets us redistribute the signed
build by Intune and put the Linux build in our sandbox image, with the notice kept.

**Maintenance: very active, very young, a small company.** v1.4.217 on 2026-09-29; ten releases
2026-09-22 to 09-29 (releases feed); last push 2026-09-30 01:05Z; 81,648 stars, 5,292 forks.
**3,638 open pull requests and 3,442 open issues** (search API), with 273 issues closed in the last
seven days. Stably AI is YC W22, founded 2022, **25 people**, San Francisco, active. The repo was
created 2026-03-17: six months old.

**Paid tiers: none found.** No price anywhere; the enterprise page offers "Rollout guidance",
"AICPA SOC 2 Readiness" (readiness, not an attestation) and "talk to our team". Mobile needs an
Orca sign-in to Orca's hosted relay. No SSO: issue #10687 (open since 2026-07-26, **0 comments**)
says "The pairing link is currently the entire credential: whoever holds it has a full shell on
that runtime."

| # | Researcher's claim | Verdict | At source |
|---|---|---|---|
| 1 | Recipes run from the desktop only | **Holds** | `ephemeral-vm-recipe-context.ts:44-51`: a repo with a `connectionId` gets "Ephemeral VM recipes run on the local desktop host in v1." A local, non-folder git repo is required |
| 2 | A plugin can ship the recipe, so no `orca.yaml` in customer repos | **Holds, with a catch** | `combineEphemeralVmRecipes` (`:140-146`) merges plugin recipes, but "Project-owned recipes ... shadow same-id global plugin recipes". A repo's `orca.yaml` with our recipe id replaces our `create` script on the laptop. Use a namespaced id and check it in `create` |
| 3 | Plugin API: closed capability set, no `net:fetch` yet | **Holds, but the worker is not sandboxed** | `plugin-capabilities.ts:15-23` is the closed set. The worker is `fork(entry, [], { execArgv: [] })` with `ELECTRON_RUN_AS_NODE=1` and a plain `import()` of the plugin (`plugin-host-process.ts:88-95`, `plugin-host-runtime.ts:56`). Capabilities gate the host API, not the process, so a worker can `fetch` and spawn today. That settles the day-1 spike in our favour, but it rests on a boundary Orca says will tighten |
| 4 | Hosted review runs on the execution host | **Holds** | `hosted-review-execution-host.ts:21-26`: a `runtime:` host throws, "that environment's server runs its own git" |
| 5 | ADO PR creation exists; no auto-complete | **Holds** | `src/main/azure-devops/pull-request-creation.ts`: PAT or bearer from `ORCA_AZURE_DEVOPS_TOKEN` / `_PAT` / `_ACCESS_TOKEN` env vars. No `completionOptions` or `autoComplete` anywhere in `src/main/azure-devops/` |
| 6 | Resume only from local workspaces; no fork | **Holds** | `session-history.mdx:24,48-51` |
| 7 | Telemetry to PostHog US, `ORCA_TELEMETRY_DISABLED=1` | **Holds** | `telemetry.mdx:14,57`; also `DO_NOT_TRACK=1` and an in-app toggle |
| 8 | Six MDE incidents in eight days | **Holds, and understated** | `windows-edr-posture.md:4-5`. See rows 11 and 12 |
| 9 | OSC title events are "display only for us" | **Does not hold** | See 2.1 |
| 10 | "Pin one version" | **Only by request** | electron-updater polls `github.com/stablyai/orca/releases/latest/download` with `autoDownload = false` and a nudge (`updater-setup.ts:146-165`); a user-chosen pinned build exists. No admin lock was found. The NSIS config sets no `perMachine`, so it is a per-user install and a user updates without admin rights |
| 11 | *(new)* Where it runs on Windows | **Risk** | Per-user in `%LOCALAPPDATA%\Programs\orca\`, and the terminal daemon runs from a copy in `%LOCALAPPDATA%\Orca\daemon-host\<version>\`. Orca's own doc: "execution from a non-standard user-writable location is scored on its own" |
| 12 | *(new)* Who signed it | **Risk** | Publisher is `CN=SignPath Foundation`, "a free OV certificate ... shared across many OSS projects" (`windows-edr-posture.md:347-352`). An App Control or AppLocker publisher rule cannot single Orca out, and a hash rule breaks with every daily release |
| 13 | *(new)* Desktop and sandbox versions drift | **Holds, with a limit** | "Mixed versions are the normal state" (`remote-wire-compatibility.md:3-5`); protocol 3, minimum compatible 2. The cross-version test runs the working tree against the newest release tag only, so a skew of more than one release is untested |

### 2.1 Orca's attention queue reads PTY bytes, and that is the feature we adopt it for

`src/shared/agent-status-osc.ts` is a "Stateful OSC 9999 parser for PTY streams". The status store
takes "hook HTTP posts, the WSL and SSH relay receivers, and main's own OSC parse" down one
`applyNormalizedStatus` path (`agent-status-store.md:69-71`). Its only guard is narrow: a Claude row
whose main agent is `done` while a child works refuses OSC (`:213-215`). For Claude Code, Orca
installs "a status-line hook that emits OSC title events Orca uses for state dots"
(`agents/claude-code.mdx:15`).

`OD-8`'s rule is "no feature derives meaning from PTY bytes", and its reason is that a terminal
stream "can be forged by anything the agent prints". An untrusted agent can print
`ESC ] 9999 ; {…} BEL` and move its own row to done or waiting. The researcher calls this display
only, but the attention queue is the reason Orca replaces `BLOCKED-ON-YOU` (R4). **So adopting Orca
needs an owner ruling on `OD-8` before first ship, or an upstream option that turns OSC status off
for hook-capable agents.** The researcher's 0.5 day for that option is a guess. Orca also resolves
a row's terminal handle only through the OSC parse (`agent-status-store.md:353-355`), so the change
is not a one-line setting, and upstream may decline it.

## 3. The lane path: recipe, plugin, credential-free push

_To be written._

## 4. Landing: GitHub merge queue and Azure DevOps auto-complete

_To be written._

## 5. Phase 2 (ours) as the alternative

_To be written._

## 6. Integration days against our architecture

_To be written._

## 7. Corrected recommendation

_To be written._

## 8. What this pass could not verify

_To be written._
