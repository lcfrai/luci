### Task 14: Desktop — the lens frame, its connection states, and the binding

**Half:** 7a. **Hours:** 2.5. **Depends on:** 05, 13. **Elsewhere:** Hub C3 (the local key on every
request — the lens works without it).

The Deploy lens replaces its placeholder (`AppShell.tsx:60-67`, already `phase={7}`) and draws the
states a person meets **before** any environment: not connected, awaiting approval, token rejected,
unreachable, and the six binding answers. Everything past a bound, connected project is task 15.

**Read `docs/DESIGN-SYSTEM.md` before writing a line** (`CLAUDE.md`, *Frontend rules*). This task
holds, and its tests check where a test can: no indigo/violet, no gradient hero or three-card grid,
no blanket glass (§*The tells*); elevation is `--e-card`/`--e-raised`, never a flat border (§*Token
structure*); radius by size — `--r-ctl` on inputs and buttons, `--r-card` on the connect card
(`tokens.css:110-113`); `transform`/`opacity` only, never `transition: all` (§*Floor*); every state
designed (§*Floor*: loading, empty, partial, stale, offline, unauthorized, error). **Visual
acceptance is the owner's eyes, batched to the end (OD-12); until then this task's look is
unverified**, and its report says so.

**Files:**
- `desktop/src/deploy/DeployLens.tsx` (new) — the frame; picks one of the states below
- `desktop/src/deploy/ConnectPanel.tsx` (new) — address + token
- `desktop/src/deploy/BindingPanel.tsx` (new) — the six unbound answers and the one choice
- `desktop/src/deploy/deploy.css` (new) — `@import "../styles/tokens.css";` first (`designsystem.test.ts:299`)
- `desktop/src/deploy/DeployLens.test.tsx`, `ConnectPanel.test.tsx`, `BindingPanel.test.tsx` (new)
- `desktop/src/components/AppShell.tsx` (modified) — `case "deploy": return <DeployLens />;`. If
  Hub task 42 has merged when this task is built, the line is `<DeployLens onShare={openShare} />`,
  with `openShare` read from the store by a selector in `AppShell`; if not, it passes nothing, and
  adding `onShare` is the one-line wiring recorded in plan §10's Hub C2/C4 row (plan re-review b10 M-5)

#### Component contracts — complete

```tsx
/** The Deploy lens. Reads the selected project from the store and its view from the deploy slice
 *  (task 13). On mount, and when the selected project changes while mounted, it calls
 *  loadDeployView ONCE — the only fetch it starts; every later refresh is the cursor's (task 13).
 *  onShare is the seam to the Hub's share screen (review M17; plan re-review b10 M-5): the lens
 *  never reads openShare from the store itself, so it type-checks before and after Hub task 42.
 *  The host passes it; absent → no Share button (the no_hub_project row below). */
export function DeployLens(props: { onShare?: (projectId: ProjectId) => void }): JSX.Element;

/** The first-run form. Posts once; clears both fields on every outcome; never stores the token in
 *  state beyond the input's own value, and never in the zustand store. */
export function ConnectPanel(props: {
  status: ConnectionStatus;             // origin prefill and originLocked
  onConnected(status: ConnectionStatus): void;
}): JSX.Element;

export function BindingPanel(props: {
  binding: Binding;
  onChoose(panaceaProject: string): Promise<void>;
}): JSX.Element;
```

#### States and their words — each one a test

Copy is from `prototype/phase-7-deploy.html:806-838` where it was right, and **corrected where
OD-3 or this plan changed a fact** (marked †).

| When | Drawn | Words |
|---|---|---|
| no project selected | the lens's empty state | *"Choose a project in the rail to see where it is deployed."* |
| first load in flight | a skeleton that **mirrors task 15's geometry exactly** (DESIGN-SYSTEM *Interaction standards*: a skeleton that shifts one pixel is worse than none); no spinner | — |
| `not_connected` | `ConnectPanel` in a card | heading *"Connect this cockpit to Panacea"*; the three numbered steps of the prototype (`:810-815`), with step 2 naming **which** key (review I5): *"Go to Account → API keys and create a key. Leave* Restrict to a project? *on* No restriction. *It starts with `pnc_`."* — and under the steps, *"A key restricted to one project sees only that project and cannot change owner settings. Restricted to Read only, it cannot Ship, roll back, rebuild, cancel or change who can open a branch."* (each of those is `authedProjectDeveloper` in Panacea: `routes/routes.go:1286`, `:1026`, `:1291`, `:1431`; plan re-review b06 M-2) The portal's words, verified at `b33d15acb` (`backstage/packages/app/src/plugins/AccountPage.tsx:1345-1355`: the field *Restrict to a project?*, default *No restriction — full access, same as you*; `:993-996` and `:1060`: once a project is picked the capability defaults to *Read only*). Step 2 and that line are one constant, `KEY_ADVICE` in `ConnectPanel.tsx`, the seam for the owner's answer to plan Q-P7-7; † the hint under the address reads *"Your Panacea's address, e.g. https://dev.panacea.kpmg."* — the old *"two written sources disagree"* was settled by measurement (`orchestrator-rulings.md`) |
| address check failed | the field's own error, verbatim from the problem | e.g. *"That address sends you to a sign-in page, so it is the portal, not the API. Try https://dev.panacea.kpmg."* |
| `awaiting_approval` | a card, no form | *"Panacea accepted your token, but you have no access yet."* … *"A project owner approves new members in the Panacea portal. This screen checks again on its own."* (true: task 07 step 0) |
| `token_rejected` | a card with *Paste a new token* | *"Panacea rejected the token."* + `status.message` verbatim (e.g. *"api key revoked or expired"*) |
| `unreachable` and **no** view yet | a card | *"Panacea could not be reached."* + † *"If the corporate proxy intercepts TLS, the fix is to add its root certificate (PANACEA_APP_PANACEA_CA_FILE) — never to turn certificate checks off."* |
| `unreachable` **with** a view | task 15's lens, dimmed, with the stale banner (task 15) | — |
| binding `no_origin` · `unknown_host` · `no_match` · `not_connected` · `unavailable` | `BindingPanel`, one sentence | `binding.message` verbatim (task 06's table) |
| binding `ambiguous` · `no_repo_info` | `BindingPanel` with a radio list of `candidates` (display name, then `ref` in mono, then `repoUrl` in `--ink-3`) and one *Use this project* button | `binding.message` verbatim |
| binding `bound` | task 15 | — |
| `releasesState: "no_hub_project"` | a quiet line above task 15's content, and one **Share with a team…** button (review M17) | *"This repository is not a shared project in the Hub yet, so releases show no brief and Ship is off. Share it with a team to turn Ship on."* The button calls the lens's `onShare(projectId)` prop, which the host wires to the store's `openShare` (Hub task 42), which opens the Hub's `SharePopover` (Hub task 42, the project menu's own *Share with a team…*). That popover links the project through `POST /v1/projects/{projectId}/hub-link` `{teamId, name}` (Hub task 35, seam S-LINK), which calls `CreateProject` (Hub contract §7.4, `:705` at 6b873d0), or links to the team's existing project for this repository. Phase 7 creates no Hub project itself and writes no second share form. If Hub task 42 has not landed, the host passes no `onShare`, the button is not drawn and the sentence ends *"…Ship is off. Sharing a project arrives with the Hub's share screen."*, which is true and makes no promise. After a share, the watcher re-resolves on the Hub's `project.*` event (task 07 `HubEvents`), so Ship turns on without a restart |
| `releasesState: "hub_key_spelling"` | the same line | *"The Hub cannot hold this repository yet — the cockpit names it in a form the Hub does not accept — so releases show no brief and Ship is off."* No host is named: the cause is any key `hubapi.HostOfRepoKey` refuses, GitHub or Azure DevOps (HA7-6, review I6; plan re-review b05 R-3) |
| `releasesState: "signed_out"` | the same line | *"Sign in to see which brief each release was shipped against."* — the rail footer (Hub C4) holds the control |

**Token handling in the form.** `type="password"`, `autocomplete="off"`, `spellcheck={false}`; the
value is read from the input's ref at submit and passed straight to `DeployApi.connect`, then the
input is cleared whatever the outcome. **No `useState` holds the token** (a React devtools snapshot
would show it). A pasted value not starting `pnc_` shows *"A Panacea API key starts with pnc_."*
under the field before submit; the server checks again (task 05).

#### Tests to write first (`vitest` + `@testing-library/react`, jsdom)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `renders the placeholder no longer` | `AppShell` with `lens: "deploy"` renders `DeployLens`, and the text *"Ship the branch, with the spec attached"* is gone | the lens wired but unreachable |
| `one load on mount` | mounting with a selected project calls `loadDeployView` once; re-rendering with the same project does not call it again | a `useEffect` without deps that fetches per render (P8) |
| `each connection state` | one test per row of the table, asserting its words and **only** its control | two states sharing one screen, which `AGENTIC-UX.md` §2 forbids |
| `token never in the store` | after typing `pnc_secret` and submitting, `JSON.stringify(useCockpit.getState())` lacks `pnc_secret`, and the input is empty | a token held in renderer state (ADR-H1 §6.5) |
| `connect names the key` | the `not_connected` panel's step 2 and the line under the steps render `KEY_ADVICE` exactly: *No restriction* is named, and so is what a restricted and a *Read only* key cannot do; the line contains *"cannot Ship, roll back, rebuild, cancel or change who can open a branch"* word for word (review I5; re-review b06 M-2) | a panel that says only *"create a key"*, so the user takes the portal's scoped path, gets a one-project *Read only* key, and meets *"No Panacea project your key can see"* and a refused Ship with no warning |
| `token cleared on failure` | `connect` rejects `deploy_token_rejected` → the input is empty and the problem's message is shown | a failed paste left sitting in the field |
| `locked origin read-only` | `originLocked: true` → the address input is `readOnly` with the origin | a user override of an administrator's address |
| `ambiguous chooses once` | two candidates; choosing one calls `bind` with its `ref`; the button is disabled while it runs | a double submit binding twice |
| `not shared names the way forward` | `releasesState "no_hub_project"` with an `onShare` spy passed as the prop → a **Share with a team…** button; clicking it calls `onShare("p-1")` once and nothing else (no `DeployApi` call, no Hub route); with no `onShare` prop → no button (absence injected through the seam, no cast; plan re-review b10 M-5), the *"arrives with the Hub's share screen"* sentence; `"hub_key_spelling"` → no button either, because no share can fix a key the Hub refuses (review M17) | a Ship that is off with no way forward; a second share form in the Deploy lens; a Share offered for a key no Hub project can ever hold |
| `binding message verbatim` | each non-bound state shows `binding.message` exactly | rewording the control plane's sentence in the desktop |
| `no spinner` | the loading state's DOM contains no element with `role="progressbar"` or a class matching `/spin/` | a per-component spinner (DESIGN-SYSTEM *many live processes*) |
| `design-system guard` | the existing `styles/designsystem.test.ts` passes over `deploy.css` | a literal hex, `transition: all`, a missing token import |

#### Steps

- [ ] Read `docs/DESIGN-SYSTEM.md` in full and `prototype/phase-7-deploy.html:806-838`.
- [ ] Tests first; then the three components and `deploy.css`; then the one-line `AppShell` change.
- [ ] `npx tsc --noEmit` and `npx vitest run`; commit by pathspec.
- [ ] Record in the commit message: *"Visual acceptance pending the owner (OD-12)."*

#### Acceptance

```
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass, including styles/designsystem.test.ts
```
**Owner's eyes (batched, OD-12):** open the Deploy lens with no token stored; paste a real `pnc_`
token; see *Connected* become the lens. Until then: **unverified**.

#### What this task does NOT claim

- How it looks. A passing test is not evidence of that (`CLAUDE.md`, *Working agreements*).
- The Hub sign-in surface. That is Hub C4; this lens only reports `signed_out` in a line.
