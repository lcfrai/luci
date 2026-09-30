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

### 3.1 The recipe seam holds; four details change the flow in R3

Source: `skill-guides/orca-per-workspace-env.md` and `docs/reference/headless-linux-server.md`.

1. **Orca names the lane branch, after pairing.** In Orca-server mode the guide says "let Orca create
   a linked worktree"; `provisioned-root` is "only ... direct SSH". So the control plane clones a
   primary checkout to `projectRoot`, and Orca runs `git worktree add` inside it later. `create`
   cannot know the branch, so the lane endpoint's hook must learn it from the plugin's
   `worktree.created` event, not fix it at create time.
2. **`create` gets less than R3 says.** It "runs locally from the repo root" with `ORCA_RECIPE_ID`
   and `ORCA_VM_INSTANCE_ID`. `ORCA_REPO_URL` and its siblings belong to the SSH `provisioned-root`
   contract. The repo URL comes from the laptop's anchor clone. The brief key has no field in Orca's
   new-workspace dialog, so the plugin must hold a pending brief that `create` reads.
3. **The warm pool saves the Kata boot, not Electron's start.** "Delete the runtime's user-data
   directory before re-snapshotting, or every workspace from this image shares one pairing
   identity." `orca serve` (Electron plus Xvfb) must start after the claim.
4. **The pairing route holds, and Orca asks for more than we give it.** `--pairing-address` takes "a
   complete reverse proxy URL such as `https://orca.example.com/runtime`". Orca's own advice: "Do not
   forward the Orca port directly to the public internet. Prefer ... an authenticated tunnel." 01's
   route is TLS plus a source-IP allowlist, which is short of that.

### 3.2 Push to the lane endpoint holds; "the control plane pushes as the user" does not

- **Holds.** Orca's Push is plain git: it "pushes the worktree's branch to `origin`, setting upstream
  the first time". Force is a separate `--force-with-lease` action (`review/commit-push.mdx:19-21`),
  so the hook must allow a lease-guarded rewrite of the lane's own branch, or an agent's rebase fails.
- **Does not hold.** The lane endpoint and the sandboxes are in the cloud, so R3's "control plane"
  that pushes and opens the PR is the cloud one. Four rules say it has no credential to do that:
  `OD-6` keeps the GitHub and ADO tokens "in Windows Credential Manager"; `OD-30` keeps ADO "a
  personal access token"; `OD-22` gives the cloud "a read-only credential" only, and "every push still
  uses a signed-in person's own credential"; phase 3 task 13 has the Hub serve that store "over Windows
  Credential Manager, per signed-in user". R6 prices landing at 1.5 days without this.
- **The fix inside the rules: land from the laptop.** The plugin's Open PR command runs
  `panacea lane land <lane>` on the laptop; the worker can spawn a process today (§2 row 3). The CLI
  fetches the lane branch into the anchor clone under `refs/panacea/lanes/…` (so `OD-2`'s "fetched
  back" holds as written), pushes through Git Credential Manager, then opens the PR and sets
  auto-merge or auto-complete with the stored token. The cloud never holds a push credential.
  **About 1 day more.** The other route, handing the token to the cloud per request or storing it
  there, changes `OD-6`, so it is the owner's call.

### 3.3 The plugin holds, and it can reach the Hub today

`terminal:send` types "into a terminal you can see (always a specific terminal)"; panels are sandboxed
frames. The worker is unsandboxed Node (§2 row 3), so the day-1 `net:fetch` spike is settled. The
risk moves: Orca says scoped `net:fetch` and `process:exec` "arrive in later phases". When they do,
the plugin must declare them, so the upgrade canary must run the plugin.

### 3.4 New: the agent shares its sandbox with `orca serve`

Orca starts the agent's PTY as its own child and, by default, as the same user. So the agent can read
the runtime's user data, including the pairing offer ("a capability containing a device credential
and E2EE material"). It can also kill the server and bind its port. The paired desktop then trusts
whatever that endpoint sends: files, diffs, status rows, notifications and review actions. That is a
far richer channel than phase 2's terminal bytes into xterm.js. Orca's remote model assumes the
runtime host is the user's own machine. The only threat model in the tree is
`agent-skill-sharing-threat-model.md`. **Not tested, and nobody has reviewed it. It needs a security
review before any customer tenancy.** The Kata boundary protects the cluster, not the laptop.

## 4. Landing: GitHub merge queue and Azure DevOps auto-complete

**Landing through the host is upheld.** It is the strongest part of the recommendation: phase 3's
concurrent land queue, forecast and undo become host features under the customer's own policy.

| Claim | Verdict | At source |
|---|---|---|
| GitHub's merge queue needs Enterprise Cloud on private repos | **Holds** | Docs: "available in any public repository owned by an organization, or in private repositories owned by organizations using GitHub Enterprise Cloud"; Actions "**must** use the `merge_group` event" |
| KPMG already has Enterprise Cloud | **Holds for github.com** | KPMG AU's GitHub is an EMU enterprise on **github.com** (org `kpmg-au-emu`, owner's memory note), and EMU is Enterprise Cloud. It stores code in the US, so AU residency means a separate GHE.com enterprise, which is the customer's call |
| The queue works the same on GHE.com | **Plausible** | GHE.com's feature-overview page lists what is missing (Marketplace, macOS runners, gists and more) and does not list the merge queue or auto-merge. That is absence of a limit, not a statement of parity |
| Price basis | **Holds** | github.com/pricing: Enterprise "$21 USD per user/month for the first 12 months"; Team "$4". The queue is the only reason to require Enterprise |
| ADO auto-complete carries the brief trailer | **Holds** | REST 7.1 `PATCH …/pullrequests/{id}` updates `CompletionOptions` and `AutoCompleteSetBy.Id`; `mergeCommitMessage`: "If set, this will be used as the commit message of the merge commit"; "Auto-complete always waits for required policies"; scope `vso.code_write` |
| ADO has no merge queue | **Holds (absence)** | Nothing in 2026 search results or release roundups. Copilot code review for Azure Repos is a limited preview (June 2026): Copilot, so not for AU projects |
| An ADO organisation can sit in Australia | **Holds** | Learn, *Data locations for Azure DevOps*: Australia is a listed geography, and data stays in it |
| Orca creates ADO PRs (the first run's correction) | **True, and moot for lanes** | Hosted review runs on the execution host (§2 row 4), which is the credential-less sandbox. Orca's ADO code only helps local workspaces |

### 4.1 Panacea previews: GitHub yes, Azure DevOps no, and ADO pushes do something else

- **GitHub: holds.** `selectPipeline` sends `pull_request_opened` to the preview pipeline and
  `pull_request_closed` to cleanup (`webhook-receiver/pipeline/trigger.go:1469-1472`).
  `providers/github.go:168-173` maps the PR actions.
- **ADO: absent, not "unverified".** The receiver's ADO provider "Supports `git.push`" only
  (`providers/azuredevops.go:108-116`). Worse for us, Panacea treats every ADO event as a Forge
  agent ship: `selectsAgentPipeline` returns true for `event.Provider == "azuredevops"`
  (`trigger.go:1512-1519`), and the production-branch and previews labels are "Skipped for Azure DevOps
  (Forge agent) events, which are not governed by these" (`:441-447`). **So registering a customer ADO
  repo in Panacea today would send every lane push to agent-deploy, not to a preview.** ADO previews
  are a Panacea-side change. Until then, v1 previews are GitHub only.
- **Session id: holds.** `X-Panacea-Session-Id` (`models/event.go:16`) is real and is what routes a
  Forge ship to agent-deploy and back. Copying the pattern for the lane and brief key is sound.

### 4.2 What this does to R6's landing claims

- "Landing through the host hands phase 3's hardest code to production systems": **upheld**.
- "Turns on the merge queue or auto-merge, or ADO auto-complete with `mergeCommitMessage`": **upheld**,
  run from the laptop CLI with the user's stored token (3.2), not from the cloud control plane.
- "Each lane's running app is reviewed in a Panacea PR preview": **GitHub only** for v1.
- The trailer surviving a squash under the merge queue is still one test per host (researcher's §8).

## 5. The two best alternatives, tried the same way

### 5.1 Phase 2 (ours): the backend holds; freeze the desktop, do not delete it

- **Claims hold at source.** `control-plane/internal/backend/backend.go:337-343` is the `Backend`
  interface (`Create`, `Get`, `List`, `Attach`, `Destroy`), with `dockerbe` and `katabe` beside it;
  `internal/{reaper,sandbox,worktree,proxy}` and `sandboxd/` exist; `MergeLens.tsx` and
  `FilesLens.tsx` are 15-line placeholders. Note: there is no `Suspend` or `Resume` on the
  interface, so Orca's `suspend` and `resume` scripts need 01's pause path added to it.
- **As a cockpit, it loses on depth and wins on risk.** Orca has an inbox, subagents, diff notes and
  a dashboard. Phase 2 has none of them, and matching them is 12 to 18 days (researcher's §3a).
  But phase 2 avoids four of Orca's problems:
  1. **`OD-8`:** its attention queue would come from Claude Code's typed hooks.
  2. **EDR:** the Tauri desktop is a thin client with no local shells, PTY daemon or process scans,
     so it should score far lower. Untested. It is also **unsigned today** (`tauri.conf.json` has no
     signing config), so it needs a KPMG code-signing certificate either way.
  3. **Trust:** the channel from the sandbox is terminal bytes only (3.4).
  4. **Churn:** there are no daily upstream releases to track.
- **Correction to R4.** "Retire the Tauri Develop views" should read **freeze them**: keep them
  building in CI, but not as the product surface. Orca's repo is six months old, a 25-person
  company makes it, and it says "no compatibility promises". The IT change (§6)
  or an `OD-8` ruling could each stop the Orca desktop, and freezing costs almost nothing, because
  the code is already built.
- **Verdict: integrate** (the backend, as R4 says) plus **keep as the fallback cockpit**.

### 5.2 Emdash (`generalaction/emdash`): still reference

- **Licence: Apache-2.0**, from the API (`LICENSE.md`).
- **Maintenance and install:** v1.2.7 on 2026-09-27, with weekly stable releases and a separate
  canary channel; 5,882 stars, 97 open issues. It ships an **MSI**, which suits Intune better than
  Orca's per-user `.exe`.
- **Status:** it "installs marker-tagged entries in the agent's user-level config", which fits
  `OD-8` better than Orca's OSC parse.
- **Why it still loses:** remotes are "SSH/SFTP" machines you connect by hand. The README shows no
  recipe seam that creates a sandbox per task, no plugin API and no Azure DevOps. A sandbox per lane
  would need a fork. **Reference**, and the fallback if Stably AI changes course.

### 5.3 Orca without the desktop: the fallback if IT says no

`orca serve` also pairs a "browser client" (`ways-to-run.mdx`), and `--recipe-json` returns
`pairing.webClientUrl`. Our control plane could start each lane itself, with no desktop recipe, and
the Hub could link to each lane's web client behind oauth2-proxy and Keycloak. That puts SSO in
front, as 01's verify wanted, and needs no Windows install, no Defender rule and no anchor clone.

The costs:
1. **One client per runtime** ("implicitly 1 client ↔ 1 server", #10687), so the fleet view becomes
   a Hub page instead of Orca's sidebar.
2. **Unknown parity:** the web client's features are undocumented, and plugins in serve mode are
   unverified.

**Not sized. Run a 0.5-day spike on day 1**: the web client behind a path-prefixed proxy (the
researcher found `vite.web.config.ts` `base: './'`). Promote this route only if KPMG IT has not
cleared the desktop by the end of week 1.

## 6. Integration days against our architecture

**7.5 days becomes about 10.5 (9.5 to 12), and first ship moves from 4.5 to about 6.5.** That is on
top of 01's corrected ~10 days for the sandbox. Three day-0 items decide the date more than any of
this, and none of them is engineering.

| R6 item | R6 | Sceptic | Why |
|---|---|---|---|
| Orca rollout package | 0.5 | 0.5 | Holds as engineering. No admin lock on version or settings was found (§2 row 10), so "pinned", "telemetry off" and "computer use off" are defaults a user can change |
| `panacea login` | 0.5 | 1 | No CLI and no PKCE code exist in the repo or the Hub worktree. Loopback PKCE, Credential Manager, refresh and the anchor clone are greenfield. The Keycloak client in Panacea's realm is an ask to the Panacea side (`OD-30`) |
| Lane git endpoint | 1.5 | 2 | Plus learning the branch from `worktree.created` (3.1), a lease-guarded force (3.2), and serving a fresh base branch through `OD-22`'s read-only fetch |
| Landing | 1.5 | 2.5 | Moves to the laptop CLI: fetch back, push, open PR, auto-merge or auto-complete (3.2). It is 2 if the CLI reuses phase 3 tasks 12 and 13's credential-helper code as planned |
| Panacea preview and provenance | 0.5 | 0.5 | GitHub only. ADO needs a Panacea-side change (4.1) |
| **First ship** | **4.5** | **6.5** | |
| Panacea plugin | 2 | 2.5 | Plus the pending-brief step before `create` (3.1) |
| Resume and fork | 0.5 | 1 | Claude Code keys transcripts by working directory, and Orca picks the worktree path. Starting `claude --resume … --fork-session` in Orca's terminal from `create` is unverified |
| AI first-pass review | 0.5 | 0.5 | Holds |
| **Total** | **7.5** | **10.5** | |

**Day 0, not counted above, and started today:**
1. **KPMG IT and SOC:** an MDE alert-suppression rule for Orca's process tree, an ASR allowlist for
   `%LOCALAPPDATA%\Programs\orca\` and `…\Orca\daemon-host\`, and any App Control or AppLocker rule.
   A publisher rule cannot target Orca (§2 row 12). This is a security team's decision and could
   take longer than the build. **The owner said "not in weeks", so it needs asking now, with the
   5.3 fallback ready.** On this dev laptop, per-user NSIS apps already run (Phren sits in
   `%LOCALAPPDATA%\Programs\`), so an owner pilot looks possible here. That says nothing about the
   standard fleet or a customer's.
2. **The owner's `OD-8` ruling** on Orca's OSC status (2.1). Zero days if the owner accepts it for
   display; otherwise an upstream change of unknown size and timing.
3. **A security review of 3.4** before any customer tenancy; for KPMG's own pilot, the owner's call.

**Architecture fit, checked:**
- **Go `Backend`:** fits, with `Suspend` and `Resume` added (5.1).
- **Keycloak:** fits for the Hub and the CLI, not for Orca's pairing (01 verify).
- **LiteLLM:** unchanged; the gateway token goes into the sandbox env.
- **Azure:** unchanged.
- **Tauri:** frozen as the fallback, not deleted (5.1).
- **"Replaces" arithmetic:** re-added from the phase 3 task table and correct (17 + 5.5 + 6 = 28.5 of
  71.5 hours; kept 31 hours, 27 still to build). Phase 8 tasks 22 and 26 and team-mode tasks 49, 61
  and 63 were not re-read.

## 7. Corrected recommendation

_To be written._

## 8. What this pass could not verify

_To be written._
