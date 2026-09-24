### Task 41: the sign-in surface in the rail footer — UI

**Lane D. Depends on: 40. About 4 hours (3, plus 1 for the prototype, review M14). Contract task C4.**

The person lives in the rail footer, where every prototype puts them (`prototype-fact-sheet.md:26`:
*"The person — Dylan Mackie — rail footer"*). Four states from `GET /v1/auth/status` (contract
§9.5): **signed out** (a *Sign in* control), **signing in** (*"Finish signing in in your
browser"*, with *Cancel*), **signed in** (the name, with *Sign out* in its menu), and **needs you
to sign in again** (the same control, and a sentence saying why). With no Hub configured the footer
is exactly phase 2's: the product works on one laptop without anyone signing in.

**Read `docs/DESIGN-SYSTEM.md` first.** Its non-negotiables, as they bind this surface:
- **No spinner.** The signing-in state's only motion is a `StatusDot` with `live`, which is the only
  component allowed to call `beatDelayStyle()` (`components/common/StatusDot.tsx:2`) — so it
  breathes on the one global heartbeat with every other live indicator.
- **Motion explains causality or is deleted**; `transform`/`opacity` only; never `transition: all`;
  a `prefers-reduced-motion` variant for anything that moves.
- **The accent is for interactive and current state only** — the *Sign in* control may use it; the
  name, the sentences and the menu may not.
- **Elevation** is a 1px ring plus layered shadow (`--e-btn`, `--e-card`), never a flat border;
  **radius** by size (`--r-ctl` for the control, `--r-card` for the menu). No literal colour: tokens
  only (`styles/designsystem.test.ts` audits every CSS file).
- **Focus rings are designed**; the control and the menu are fully keyboard-operable.
- **Every state designed**, including the unconfigured one (nothing new shown) and each failure.

**Visual acceptance is the owner's eyes, batched to the end (`OD-12`).** This task's tests prove
behaviour, not appearance; its report says *unverified by the owner* for everything visual.

**A prototype comes first (owner question Q13, review M14).** `OD-1` waives prototype-before-plan
for phases 3, 4, 5 and 7 only (`owner-decisions.md:12`), so phase H's two surfaces, this one and
task 42's share popover, have no waiver and had no prototype. Step 0 writes one: a static,
self-contained `prototype/phase-h-hub.html`, beside the phases' own prototypes, that shows this
control's four states and the no-Hub footer, and task 42's popover in each state its tests name.
It is committed alone, before any component code, and the components follow it. The owner's look
at it is batched with the rest (`OD-12`), so lane D does not wait. **`feat/hub` is not merged
onward until the owner has looked** (plan, "Order and parallel lanes"). If the owner answers Q13
with *no prototype*, Step 0 is skipped and the hour is returned.

**Files:**
- `prototype/phase-h-hub.html` (new; Step 0)
- `desktop/src/components/rail/AccountControl.tsx` + `AccountControl.test.tsx` (new)
- `desktop/src/components/rail/RailFooter.tsx` — render `AccountControl` (modify ⚑: two lines)
- `desktop/src/components/rail/rail.css` — the control's rules, appended (modify ⚑)

#### Contract — complete

```tsx
// desktop/src/components/rail/AccountControl.tsx

/** The signed-in person, or the way to sign in. Reads store.auth (task 40); acts through the
 *  client; opens the browser through @tauri-apps/plugin-opener's openUrl, lazily imported the way
 *  components/menu/ContextMenu.tsx:44 imports revealItemInDir. Renders null while auth is null
 *  (before the first status read) and when !auth.hubConfigured. */
export function AccountControl(props: {
  client: Pick<CockpitClient, "signIn" | "cancelSignIn" | "signOut" | "authStatus">;
  /** Injected for tests; defaults to the plugin's openUrl. */
  openUrl?: (url: string) => Promise<void>;
}): JSX.Element | null;
```

**The copy, exactly** (the control plane's own `message` is shown verbatim for an `ApiError`;
these are the surface's sentences for states it renders itself):

| State (`auth.state`, `auth.reason`) | What is shown |
|---|---|
| `signed_out`, `""` | a button **Sign in** |
| `signing_in` | `StatusDot status="starting" live label="Waiting for your browser"`, the sentence **Finish signing in in your browser.**, a button **Cancel**, and a quiet link **Open the sign-in page again** (re-opens the last `authorizeUrl`, held in component state only) |
| `signed_in` | the account's **name**; activating it opens a menu (the existing `common/Popover`) holding the username and **Sign out** |
| `signed_out`, `reauth_required` | **Your sign-in has expired. Sign in again to see your team's shared work.** and **Sign in** |
| `signed_out`, `timeout` | **The sign-in wasn't finished in time.** and **Sign in** |
| `signed_out`, `entra_error` or `exchange_failed` | **Microsoft sign-in didn't complete.** and **Sign in** |
| `signed_out`, `tenant_not_enrolled` | **Your organisation isn't set up for shared work yet.** — and no button: signing in again cannot change it |
| a `signIn()` call that rejects | the `ApiError`'s `message`, verbatim (e.g. the control plane's sentence for 503 `upstream_unavailable`, `entra`) |

After **Sign in**: `client.signIn()` → `openUrl(authorizeUrl)` → `store.setSignInExpiresAt(expiresAt)`;
the state then moves by the `Panacea-Auth-State` header (task 40), not by a timer in this component.
**Sign out** acts at once — no confirm dialog (DESIGN-SYSTEM's interaction standard) — and the
footer shows **Sign in** on the next status read.

#### Tests to write first — `AccountControl.test.tsx` (RTL, fake client, fake `openUrl`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `nothing new without a Hub` | `auth.hubConfigured: false` → the component renders null; `RailFooter` still reads *This machine* | a sign-in offered that cannot work |
| `sign in opens the browser` | click **Sign in** → `signIn` once, `openUrl` with its URL, `signInExpiresAt` set | a URL opened in the webview instead of the system browser (ADR-H1 §5.2) |
| `signing in breathes on the heartbeat, not a spinner` | in `signing_in`: exactly one element with class `dot-live`; no element whose class names `spin` or whose style animates `transform` | a per-row spinner (DESIGN-SYSTEM, *"never a spinner per row"*) |
| `cancel cancels` | **Cancel** → `cancelSignIn` once | — |
| `open again re-opens the same URL` | the link → `openUrl` with the first URL, no second `signIn` | a second sign-in that cancels the first |
| `signed in shows the name and signs out` | `signed_in` → the name; open the menu; **Sign out** → `signOut` once, no dialog | a confirm dialog on a non-destructive act |
| `each reason has its sentence` | one row per line of the table above | a generic "error" where the user needs to know what to do |
| `tenant not enrolled offers no button` | no **Sign in** in that state | a loop the user cannot win |
| `a failed sign-in shows the server's sentence` | `signIn` rejects with `ApiError("upstream_unavailable", "Microsoft sign-in can't be reached right now.", 503)` → that sentence | a paraphrase that loses the cause |
| `it is keyboard-operable` | Tab reaches the control; Enter activates it; Escape closes the menu (the existing `useDismissable`) | a mouse-only surface |
| `no token or id in the DOM` | in every state, the rendered HTML contains neither an object id nor anything token-shaped (`eyJ`) | an identifier leaking into the renderer |

`styles/designsystem.test.ts` (phase 2's audit) must still pass over the appended `rail.css`.

#### Steps

- [ ] **Step 0: the prototype.** Read `docs/DESIGN-SYSTEM.md` in full and one of the phases' own
  prototypes (`prototype/phase-4-discover.html`) for the conventions. Write
  `prototype/phase-h-hub.html`: this task's four states and the no-Hub footer, then task 42's
  popover states, each labelled, with a plain state switcher and no other script. No network, no
  font or script from outside the file. Commit it alone, by pathspec, marked *unverified by the
  owner* (`OD-12`).
- [ ] **Step 1:** Read `docs/DESIGN-SYSTEM.md` in full (if Step 0 was skipped), and Step 0's
  prototype, which the component follows; `RailFooter.tsx`, `rail.css`,
  `common/StatusDot.tsx`, `common/Popover.tsx`, `hooks/useDismissable.ts`.
- [ ] **Step 2:** Tests; they fail. Then the component, the two `RailFooter` lines, the CSS.
- [ ] **Step 3:** `cd desktop && npm test && npm run typecheck`.
- [ ] **Step 4:** In `npm run tauri dev` against `devstack.sh up` and a control plane with the four
  variables: sign in as Grace through `devissuer`'s page; see the name; sign out. **Record what was
  seen as the implementer's observation**, and mark the visual design *unverified by the owner*
  (`OD-12`).
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
cd desktop && npx vitest run src/components/rail/AccountControl.test.tsx 2>&1 | tail -2   # 11 passed
cd desktop && npm test 2>&1 | tail -3                                                      # all passed, designsystem.test.ts included
cd desktop && npm run typecheck                                                            # no output
```

#### What this task does not claim

- **That it looks right.** Only the owner's eyes accept a surface (`CLAUDE.md`, working
  agreements), batched to the end (`OD-12`).
- That Entra's page opens in the owner's default browser under KPMG's policies: task 43's
  real-tenant checklist.
- Any surface that needs the Hub (Discover, Design, Deploy): those lenses show their own
  `signed_out` or `unavailable` states (contract §9.5); none blocks the Develop lens.
