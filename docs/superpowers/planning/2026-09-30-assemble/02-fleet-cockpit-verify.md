# 02 — The agent-fleet cockpit and git lanes: the sceptic's verification

**Status:** COMPLETE (2026-09-30). The relaunch added §7, the one-app check, and carried it into
§0, §8 and §9.
**Date:** 2026-09-30. **Verifies:** `02-fleet-cockpit.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (Orca as the cockpit, lanes in our Kata
sandbox through a recipe, landing through the git host) and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop, Keycloak, LiteLLM, Azure). Then
test it against the user's one ask: that the assembled parts feel like one app (§7).
Desk research only: nothing installed or run.

## 0. Verdict first

**UPHELD, with corrections.** Orca as the cockpit, lanes in our Kata sandbox through a plugin recipe,
and landing through the host all survive at source. "Unforked, under its own name" does not: it
fails the user's one-app ask, so Orca ships as our app from a thin build of the pinned upstream (§7).

- **Why it survives:** the `LICENSE` is MIT at source, with no copyleft in the tree. Orca is very
  active (v1.4.217 on 2026-09-29, a release a day) and has no paid tier. Every seam the plan needs is
  real: desktop-run recipes, plugin-shipped recipes, Push to any `origin`, and a pairing address
  behind a proxy. GitHub's merge queue and ADO auto-complete hold as documented, and KPMG's GitHub is
  already Enterprise Cloud (EMU on github.com).
- **What does not survive:**
  1. **One app (§7).** Unforked Orca keeps its own name, palette and update feed. A plugin gets only
     right-sidebar panels that cannot navigate. The Hub pages would sit in a per-worktree browser
     pane where Entra's zero-click sign-in is not shown to work, and whose lane traffic runs through
     the sandbox. Fix: our build (name, signature, feed, tokens, a journey bar with a session
     hand-off, cloud features off), about 5 days. It also closes the version lock, the shared
     certificate and the OSC problem below.
  2. **The cloud control plane cannot push or open PRs "with the user's credential".** `OD-6`,
     `OD-22` and `OD-30` keep that credential on the laptop. Landing moves to the laptop CLI, which
     also makes `OD-2`'s "fetched back" literally true (3.2).
  3. **Orca's attention queue reads PTY bytes** (an OSC 9999 parser that needs no token). So any
     terminal output can forge a row's status and prompt text. That queue is the feature we adopt
     Orca for, so this is not "display only". It needs an `OD-8` ruling before first ship, or our
     build's OSC patch (2.1, §7).
  4. **Panacea's ADO previews do not exist.** Worse, Panacea routes every ADO push to its Forge
     agent-deploy pipeline, so v1 previews are GitHub only (4.1).
  5. **7.5 days becomes about 15.5: 10.5 for the corrections (§6) plus 5 for one app (§7).** First
     ship moves from 4.5 to about 6.5 calendar days if our build runs as its own stream. A day-0
     KPMG IT and SOC change (MDE suppression rule, ASR allowlist) could outlast the build (§6).
  6. **New risk:** the untrusted agent shares its sandbox, and by default its user, with
     `orca serve`. That puts a rich channel into the laptop. It needs a security review before any
     customer tenancy (3.4).
- **In our favour:** the plugin worker is unsandboxed Node, so the Hub fetch works today (it is a
  boundary Orca says will tighten).
- **Alternatives:** freeze phase 2's desktop as the fallback; do not retire it (5.1). Emdash stays
  reference: Apache-2.0 and an MSI, but no sandbox seam (5.2). Orca's browser client is the fallback
  if IT says no (5.3), and the most cohesive one, because the Hub is its shell (§7).

## 1. What was checked, and how

All reads on 2026-09-30. Nothing installed, built or run.

- **The report under test:** `02-fleet-cockpit.md`, all of it (609 lines).
- **Orca at source:** the blobless clone at `59b746ff` in this session's scratchpad, blobs read with
  `git show`. Files: the recipe, plugin, updater, hosted-review, Azure DevOps and OSC-status sources
  cited above; `config/electron-builder.config.cjs`; the recipe skill guide; docs on headless
  servers, remote servers, wire compatibility, the status store, EDR posture, telemetry, session
  history, commit and push, and Claude Code.
- **GitHub API:** Orca's `/license`, metadata, releases, issue #10687 and search counts; Emdash's
  `/license`, metadata and releases. Also the releases feed and Emdash's README.
- **Vendor pages:** onorca.dev/enterprise; the YC company page; GitHub merge-queue docs and pricing;
  the GHE.com feature overview; ADO REST 7.1 PR update; ADO data locations. Searches covered Orca
  pricing, an ADO merge queue, and ADO geographies.
- **Our side, read-only:** `OD-2`, `OD-6`, `OD-8`, `OD-22` and `OD-30`; the phase 3 task table and
  task 13; `control-plane/internal/backend/backend.go`; the desktop lens files and `tauri.conf.json`;
  `01-sandboxes.md` and `01-sandboxes-verify.md` by grep.
- **Panacea, read-only:** `webhook-receiver/pipeline/trigger.go`,
  `providers/{github,azuredevops}.go`, `models/event.go` and `panacea-api/handlers/webhook_ado.go`.
- **Cohesion (§7), on the relaunch:** from the same clone, the browser docs (`overview.mdx`,
  `profiles.mdx`), the CLI reference, `plugin-manifest.ts`, `plugin-host-api.ts`,
  `plugin-panel-navigation-guard.ts`, `plugin-panel-shell.ts`, `plugin-panel-design-token-css.ts`,
  `plugin-language-pack-registry.ts`, `document-theme.ts`, `global-settings-types.ts`, `main.css`,
  `en.json` (counted, not read), `skill-share-link.ts` and the builder config's brand lines. Ours:
  `desktop/src/styles/tokens.css`, `docs/DESIGN-SYSTEM.md`, and 08's §7 and §9. Web: Cypress issue
  #33836 (Electron and Entra device compliance), Microsoft's Azure Artifact Signing page, and a
  devclass report on where it is available.

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
stream "can be forged by anything the agent prints". **That forgery works here.** The payload needs
only a valid `state`, with no token or nonce: `normalizeAgentStatusObject`
(`agent-status-types.ts:305-317`), reached from `parseAgentStatusPayload` (`:365`). So
`ESC ] 9999 ; {"state":"waiting",…} BEL` printed from anywhere in the terminal sets the row. That
includes a file the agent `cat`s, a test log, or a package's install script. The same payload also
carries `interactivePrompt` and `lastAssistantMessage` text (`:330-337`) into the row. That row goes
to every reader: the sidebar, the dashboard, `orca worktree ps` and mobile
(`agent-status-store.md:21-22`); how each one renders the text was not checked. It is a spoofing
channel into the cockpit, not a cosmetic glitch. The researcher calls this display only, but the
attention queue is the reason Orca replaces `BLOCKED-ON-YOU` (R4). **So adopting Orca needs an
owner ruling on `OD-8` before first ship, or an upstream option that turns OSC status off for
hook-capable agents.** The researcher's 0.5 day for that option is a guess. Orca also resolves
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
  (`trigger.go:1504-1525`), and the production-branch and previews labels are "Skipped for Azure DevOps
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

## 7. Cohesion: does it feel like one app?

**Not as the researcher ships it.** The user's one ask for this run: "even though we are pulling
from a bunch of different areas, it still all feels cohesive and a part of the same app. I hate when
it feels like they are all seperate apps merged into one." 02 ships Orca "under its own name"
(02 §7 call 1) and joins Define, Design and Deploy as "Hub web pages, opened in Orca's browser tabs"
(02 R4, last row). 08 recommends accepting "several products' faces" for v1 (08 §9 call 2). The
user's words answer both calls: one app.

**The bar, as five yes-or-no checks on the pilot build:**
1. **One name and icon:** the user starts one thing.
2. **One sign-in:** Entra once, zero clicks (`OD-30`), and no second prompt inside the app.
3. **One journey spine:** Define, Design, Develop and Deploy reachable from every screen, with the
   brief in view (the provenance thread, `CLAUDE.md`).
4. **One look:** our tokens (`desktop/src/styles/tokens.css`, 48 variables) on every surface the
   user works in.
5. **No leaving the app for a core step:** hand-offs between phases are in-app links, both ways.

**What unforked Orca allows, at source (`59b746ff`):**

| Bar | Unforked Orca | At source |
|---|---|---|
| 1 | **No.** The name is in the build | `config/electron-builder.config.cjs`: `appId: 'com.stablyai.orca'` (77), `productName: 'Orca'` (175), `executableName: 'Orca'` (430), update feed `stablyai/orca` (679-682). "Orca" is on 789 of the 18,586 lines of `src/renderer/src/i18n/locales/en.json` (844 times). A plugin language pack is a separate locale the user must choose (`plugin:<key>/<locale>`, `plugin-language-pack-registry.ts:69-76`), not a rename |
| 2 | **No, inside Orca's browser.** Each browser profile "has its own storage partition", and "Platform passkeys stored by the operating system are not available in Orca yet" (`browser/profiles.mdx:23,31`). Electron's embedded Chromium "does not participate in WAM", and Entra's device-compliance check then fails with "AADSTS53000 / AADSTS53003" (Cypress issue #33836, open since 2026-05-19) | Reasoned for Orca: its browser code was not read, and KPMG's Conditional Access policy is unknown |
| 3 | **No.** A plugin contributes right-sidebar panels, commands, events, language packs, keybindings, VM recipes and agents, and nothing else (`plugin-manifest.ts:97-119`, `.strict()`): no top-level view, bar or menu. A panel blocks every navigation after its first document (`plugin-panel-navigation-guard.ts:23-40`), and the v0 host API has no URL or browser method (`plugin-host-api.ts:122` on). The `orca:` scheme is registered (builder config line 176), but the only consumers found are skill shares and settings, so the Hub cannot send the user to a lane | Deep links from file names and `skill-share-link.ts` only |
| 4 | **Partly.** The chrome takes `system`, `dark` or `light` (`document-theme.ts:46-57`), a left-sidebar tint, the app font and terminal themes (`global-settings-types.ts:80-157`). Its palette is stock shadcn neutrals (`main.css`: `--primary: #171717`, dark `--background: #0a0a0a`), and panels are handed Orca's 20 tokens "so panel documents can match the app" (`plugin-panel-design-token-css.ts:3-8`) | Our teal accent, warm graphite and radius scale cannot reach Orca's chrome |
| 5 | **No.** The browser pane is "a real Chromium window — address bar, history, devtools", and "Tabs are scoped to the worktree" (`browser/overview.mdx:7`). So Define, Design and Deploy would be web pages inside a code tool, swapped out when the worktree changes. Opened from a lane, a page's "HTTP(S), WebSocket, DNS, and loopback traffic still go through the remote host" (`:29`): it travels through the untrusted sandbox, whose egress would have to open to the Hub and Entra | The plugin can open that tab only by having its worker run `orca tab create --url` (`cli/reference.mdx:184`), which rests on the unsandboxed worker (§2 row 3) |

**So "Orca under its own name, with Hub pages in its browser tabs" fails bars 1, 2, 3 and 5, and
half of 4.** That is the "separate apps merged into one" the user described.

**What survives: Orca as the engine, shipped as our app.** MIT allows a renamed build if Stably's
notice is kept and the Orca name is not used. Both stacks are Tailwind plus shadcn, so the change
set is small and mostly configuration:

| Change to our build of the pinned Orca | Bar | Also closes | Days |
|---|---|---|---|
| Name, app id, icon, KPMG signature and our update feed (the builder lines above) | 1 | §2 row 10: a real version lock (our feed, our pin). §2 row 12: a KPMG publisher that App Control can single out | 0.5 |
| Rename at build time, not in source: a transform over the English catalog with an exclusion list (the `orca` CLI, `orca.yaml`, `orca-plugin.json`) | 1 | No rebase conflicts in `en.json` | 0.5 |
| One CSS file mapping our tokens onto Orca's shadcn set: `--background` from `--page`, `--card` from `--surface`, `--foreground` from `--ink`, `--border` from `--line`, `--ring` from `--accent-ring`. `--radius` is already 10px, our `--r-card` (`tokens.css:112`) | 4 | Panels then inherit our palette through Orca's own token hand-off | 0.5 |
| A journey bar: Define · Design · Develop · Deploy plus the brief chip, hosting Hub pages in one app-level view on the laptop's network, not in a worktree tab. The desktop hands the Hub a one-time session from `panacea login`, so Electron never meets Entra. Add a `lane` deep link on the registered scheme | 2, 3, 5 | Keeps the sandbox's egress shut to the Hub and Entra | 2, plus 0.5 in the Hub |
| OSC status off for hook-capable agents | none | 2.1: the `OD-8` question becomes our patch, not an upstream request | 0.5 to 1 |
| Telemetry, the mobile relay, "Share as artifact" (a public link "through your signed-in Orca account", `browser/overview.mdx:52`) and computer use off by default | 1 | No Orca account prompts. §2 row 7 | 0.25 |

**About 5 days once, plus about 0.5 each time the pin moves.** These are this pass's estimates from
the files named. The journey bar was not sized against Orca's layout code, so its 2 days is the
softest number.

**The researcher's three reasons not to fork (R3), re-weighed:**
1. **"Our own Windows code signing":** needed anyway, because the Tauri fallback is unsigned today
   (5.1). Azure Artifact Signing is "$9.99/month for up to 5,000 signatures" (Basic, Microsoft's
   product page). That page states no country rule. One trade report says it is generally available
   in the USA, Canada and Europe, so an Australian entity may need an OV certificate in Key Vault.
2. **"A fresh Defender prevalence reset on every build":** upstream already resets it daily, under a
   shared SignPath certificate (§2 rows 11 and 12). Our build resets it only when we move the pin,
   under a publisher IT can allowlist. That shrinks the §6 day-0 ask; the MDE suppression rule stays.
3. **"A daily rebase":** no. We pin anyway (§8 item 1), so the patch rebases when the pin moves,
   inside the canary. It stays cheap only while it stays in the six places above. A feature built
   inside Orca's renderer is the point where this becomes a real fork.

**The browser route (5.3) passes bars 1 to 3 by construction:** the Hub is the shell, in Edge, and
Orca's look shows only inside each lane. It stays the fallback, not the plan, because it drops Orca's
cross-lane sidebar and inbox (5.3, cost 1).

**Still open, owned elsewhere:**
- **Two more faces:** Penpot's editor (04) and Panacea Portal pages (06). The journey bar is where
  they appear. Whether each can take our tokens was not checked here.
- **The git host's pull request page** stays a reviewer's surface. The author lands from the lane
  strip (§8 item 4), which sets auto-merge or auto-complete, so landing needs no visit. Reading a
  reviewer's comments still happens on the host page in v1, because Orca's hosted review is dark
  inside lanes (§2 row 4).

## 8. Corrected recommendation

**Upheld, with corrections. Adopt Orca (`stablyai/orca`, MIT) as the cockpit engine, and ship it as
our app: a thin build of the pinned upstream, not Orca under its own name, and not a feature fork
(§7).**

1. **One app, from one pinned build** (§7). Our name, icon, KPMG signature and update feed, with the
   rename applied at build time; our tokens on Orca's shadcn variables; a journey bar (Define,
   Design, Develop, Deploy and the brief chip) that hosts Hub pages in one app-level view, signed in
   from `panacea login`; a lane deep link; Orca's cloud features off. The same version runs on the
   desktop and in the sandbox image. Every pin move is a canary that runs the plugin, a
   mixed-version pairing and the five checks in §7.
2. **Each lane runs in our Kata sandbox** through one Panacea recipe, shipped by the Panacea plugin
   under a namespaced id, because a repo's `orca.yaml` shadows a recipe with the same id (§2 row 2).
3. **The sandbox holds no git-host credential.** Orca's Push goes to a lane-scoped endpoint on the
   cloud control plane, and that endpoint allows a lease-guarded force on the lane branch.
4. **Landing runs on the laptop, not in the cloud** (3.2). The plugin's Open PR command runs
   `panacea lane land`, which does four things:
   - fetches the lane back into the anchor clone under `refs/panacea/lanes/…`;
   - pushes with the user's own stored credential (`OD-6`, `OD-22`, `OD-30`);
   - opens the PR with the brief trailer;
   - sets GitHub auto-merge or the merge queue, or ADO auto-complete with `mergeCommitMessage`.
5. **Panacea PR previews are for GitHub lanes only in v1.** ADO previews wait for a Panacea-side
   change, and until then no customer ADO repo is registered in Panacea (4.1).
6. **Freeze phase 2's Tauri Develop views as the fallback cockpit; do not delete them** (5.1). The
   control plane, `Backend` (plus `Suspend`/`Resume`), `katabe`, the transcript store and the
   gateway token carry on as R4 says.
7. **Two gates before first ship:** KPMG IT is asked **today** for the MDE suppression rule and ASR
   allowlist, now for our KPMG-signed build, and a signing route is chosen (owner call 6). `OD-8`
   (2.1) stops being a gate once our build's OSC patch lands; until then the owner's ruling stands.
   The browser-only route (5.3) is the fallback.
8. **One gate before any customer tenancy:** a security review of the agent and `orca serve`
   sharing one sandbox (3.4).

**Days:** about **15.5 (14 to 18)**: §6's 10.5 plus §7's 5, beside 01's ~10, plus the day-0 items
(§6). **First ship stays at about 6.5 calendar days only if our build runs as its own stream from
day 1.** Done in series, it is about 11.5.

**Replaces:** as R6 (arithmetic checked). One change: phase 3 tasks 12 and 13 (push through the
machine's credential helper, the credential-store seam) now serve the laptop CLI. They were designed
for that job.

**Owner calls, changed or new:**
1. **`OD-8` and Orca's OSC status** (new). Recommend accepting it for KPMG's own pilot, for display
   only, and letting nothing we build (Hub, team mode, provenance) read it. Before a customer
   tenancy, our build turns OSC status off for hook-capable agents (§7), and we offer the change
   upstream. Any terminal output can forge a status and its prompt text (2.1).
2. **The Defender and ASR prerequisite** (the researcher's call 2). Unchanged, but ask on day 0.
3. **Land from the laptop, or hand the token to the cloud** (new). Recommend the laptop: it keeps
   `OD-6` as written.
4. **ADO previews in v1** (new). Recommend GitHub-only previews in v1, and ask Panacea for ADO
   PR events.
5. **One app, not Orca under its own name** (customer-visible, new). This replaces 02's §7 call 1
   and answers 08's §9 call 2. The user's request settles the direction (§7). What is left for the
   owner: the desktop's name and icon.
6. **Code signing** (money, new). Recommend Azure Artifact Signing, US$9.99 a month on Basic, if an
   Australian entity qualifies. Otherwise use an OV code-signing certificate in Key Vault, which
   KPMG may already hold. The Tauri fallback needs one either way (5.1).

Money: GitHub Enterprise Cloud at US$21 per user per month, for queued landing on private repos, as
before, plus code signing (call 6).

## 9. What this pass could not verify

Nothing was installed, built or run. Every claim is from upstream source or docs, the GitHub API,
vendor pages, or a read-only grep of Panacea and this repo.

1. **Whether Orca runs on KPMG's standard fleet at all.** MDE, ASR and App Control policy are
   unknown. The six incidents are Orca's report from another tenant. The Phren evidence is from this
   dev laptop only.
2. **An admin lock for Orca's version and settings.** Only `updater-setup.ts` and the NSIS config
   were read; a policy mechanism elsewhere in the 30,735 files is not ruled out.
3. **The shared-sandbox attack path (3.4)** is reasoned from Orca's architecture and its "full shell"
   pairing model, not tested. Whether Orca can start PTYs as a second user was not searched.
4. **Orca's browser client**: its parity with the desktop, plugins in serve mode, and behaviour
   behind a path-prefixed proxy (5.3).
5. **The upstream `OD-8` fix**: whether Stably would accept an OSC-off option for hook-capable agents,
   and how large it is.
6. **Resume and fork through a recipe**: whether `create` can start an agent command inside Orca's
   terminal, and where Orca places the worktree path that Claude Code keys transcripts by.
7. **GHE.com merge-queue parity** (no listed limit only), and **the trailer surviving** a squash under
   the queue and ADO's `mergeCommitMessage` under auto-complete. That is one test per host.
8. **Not re-read:** the researcher's phase 8 (22, 26) and team-mode (49, 61, 63) dispositions; Orca's
   `references/windows-scripts.md`; Emdash beyond its README and the API; the ADO Basic price.
9. **What an ADO push does in Panacea today**: `selectsAgentPipeline` was read, but the agent-deploy
   pipeline it would start for a customer repo was not traced.
