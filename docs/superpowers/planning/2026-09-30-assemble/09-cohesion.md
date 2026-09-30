# 09 - Cohesion: the assembled product as one app

**Status:** COMPLETE (2026-09-30). Written and committed a section at a time; §0 was added last.
**Date:** 2026-09-30. **Question:** how do Orca, Spec Kit, Penpot, the trackers, Teams, Panacea
and our own Hub surface, so that a person meets one product and not several merged into one?
**The owner, verbatim (2026-09-30):** *"Just make sure that even though we are pulling from a
bunch of different areas, it still all feels cohesive and a part of the same app. I hate when it
feels like they are all seperate apps merged into one..."*
**Standing:** a hard requirement on the whole plan, as binding as the licence rule. A pick that
cannot meet it changes (section 7). It is not excused.
**Builds on, does not re-argue:** 02-verify §7-8 (Orca shipped as our app from a thin build),
08-verify §7 (rules R1-R6, about 9 engineer-days counted once), 06-verify §6, 00-verify §8,
07-verify §6, and each cluster's corrected pick.
**Method:** the research files first. Then read-only at source: Penpot 2.18.0's UI style and OIDC
files, Orca at `59b746ff` (palette, plugin commands, settings, `main.css`), Panacea's share-link
handler, one Keycloak extension's licence, and our `desktop/src/styles/tokens.css`. Nothing was
installed, built or run. Section 8 lists what could not be verified.

## 0. The answer

**One window under our name, in our look, behind one sign-in, with one spine, one palette, one
queue and one set of words. No product pick changes; ten pieces change how they surface. That
costs about 12 new engineer-days (10 to 15), plus 2 pulled into the plan. The one-app layer comes
to about 23 days, and the whole product to about 117.**

1. **The shell** (§1): our build of the pinned Orca. A journey bar on every screen, Orca's
   workspace as Develop, and one app-level view where every Hub page renders, Penpot's editor
   included.
2. **Surfacing** (§2): headless for Spec Kit, the trackers, Panacea, Keycloak and LiteLLM. Embedded
   only for the frame, the terminal, Penpot's editor and the preview. Orca's own bell, feed and
   task sources are off, and Panacea's Portal is out of the path.
3. **One look** (§3): `tokens.css` generates a theme file for each foreign surface. Orca takes 125
   mapped variables. Penpot's editor takes about 42 semantic colours from an injected stylesheet,
   with no file changed. Its radius and font are compiled in: accept them, or patch under MPL-2.0.
4. **One sign-in** (§4): the plan misses a realm session inside the frame, so Penpot and previews
   would each ask for Microsoft. The fix is our own realm hand-off authenticator (1.5 days), with
   share-link previews meanwhile. Penpot's login page is skipped via its own OIDC route.
5. **One navigation** (§5): four equal lenses with no pipeline cues; the thread as records in
   every view; Orca's Cmd+J palette carrying every capability's commands; the Desk as the one
   queue, fed by agents, trackers, the git host, Penpot and Panacea. No cluster had costed the
   Desk: about 3 days.
6. **Proof** (§6): fourteen checks on one walk, with the owner's eyes as the last gate.

## 1. The one shell

**Ruling: the person lives in one desktop window, our build of the pinned Orca** (02-verify §8
item 1), under our name, icon, KPMG signature and update feed. The window has three parts and
nothing else:

1. **The journey bar**, across the top of every screen. It belongs to no product and no page.
2. **Develop**: Orca's own workspace (sidebar, terminals, editor, diff) in our tokens and our
   words, with the Panacea plugin's lane card in its right panel.
3. **One app-level view**, added by our build beside Orca's workspace. Every other surface is a
   Hub page rendered here: the Desk, Work, Define, Design (Penpot's editor inside it), Deploy and
   the running preview. It runs on the laptop's network, never through a sandbox (02-verify §7).

```
┌ <our name> ─────────────────────────────────────────────────────────────────────────────────┐
│ Desk  Work │ control-testing ▸ evidence-upload │ Define  Design  Develop  Deploy │ Brief r4 ▾ │ ⌘K │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  Develop selected: Orca's workspace, in our look         any other stop selected: the       │
│  ┌ sidebar ──┐┌ terminals · editor · diff ─┐┌ lane card ┐  app-level view shows that Hub     │
│  │ worktrees ││ unmodified Claude Code, in ││ brief r4  │  page in the same frame, under     │
│  │ sessions  ││ the lane's Kata sandbox    ││ PR · prev │  the same bar                      │
│  └───────────┘└────────────────────────────┘└───────────┘                                    │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Why this frame, against the alternatives** (the five checks are 02-verify §7's):

| Frame | One name | One sign-in | One spine | One look | Never leave | Verdict |
|---|---|---|---|---|---|---|
| Upstream Orca, our pages in its browser tabs | no | no | no | half | no | **Reject** (02-verify §7, at source) |
| **Our build of pinned Orca, a journey bar, one app-level view** | yes | yes, with §4's hand-off | yes | yes, generated CSS (§3) | yes | **Adopt** |
| Our Tauri desktop (phase 2) hosting Orca's web client per lane | yes | plausible, not checked | yes | yes | yes | **Frozen fallback.** Tauri cannot host Electron, so Develop becomes Orca's web client: one client per runtime (#10687), parity unknown, Orca's cross-lane sidebar lost |
| Browser only: the Hub as the shell in Edge, Orca's web client per lane (02-verify 5.3) | yes | yes, by construction: Edge signs in with Windows | yes | yes | yes | **Fallback if KPMG IT blocks Orca's desktop.** Same loss as the row above |

Orca's frame wins because Develop is where the hours go, and Orca's integrated whole is why it
was adopted (02 R3). Both stacks are Tailwind plus shadcn with Geist, so our look reaches Orca's
chrome through one generated file (§3). The same build also fixes the version lock, the signing
publisher and OSC status (02-verify §7).

**What the owner sees on first open:**

1. One icon with our name in the Start menu (pushed by Intune). One double-click.
2. No sign-in screen. On first run only, `panacea login` opens a system-browser tab that Windows
   signs in silently (auth-code + PKCE through the realm, `OD-30`). The tab shows our own "you can
   close this" page in our tokens (07-verify C6). After that, never again.
3. The Desk, in the app-level view (AGENTIC-UX §5). A new person sees its zero state and one entry
   that takes an idea, a repo or a ticket (AGENTIC-UX §9 item 7). A returning person sees "since
   you last looked", ranked.
4. The journey bar above it: Desk and Work, the project and branch, four lenses, the brief chip
   and ⌘K. Picking a lens swaps the region below. The bar never moves.
5. Develop opens Orca's workspace on that branch's lanes, in the same frame and look. The lane
   card carries the brief, the pull request, the preview and the release.

**What it never shows:** another product's name or logo, a second sign-in, an address bar on our
pages, Penpot's dashboard, Panacea's Portal, or a pasted token after the one-time Connect step
(and none at all once Panacea's P-3 lands, 00-verify §8 S3).

The name and icon are the owner's call (customer-visible, 02-verify call 5). This file writes
"our name" until then.

## 2. Surfacing: headless, embedded, or out of the path

Each piece gets exactly one mode:
- **Headless:** its engine runs behind our screens, through its API, CLI or MCP. The person never
  sees its UI.
- **Embedded:** its UI sits inside our frame, in our tokens, with our journey bar and chrome
  around it. Every embed is justified under the table.
- **Out:** it opens outside the frame. Allowed only where a row says so, and never for a daily
  act of the person building.

Default: headless for the trackers, Deploy and Define. Days are engineer-days. "Counted" means
another file already carries them; "new" means section 7 adds them.

| # | Piece (licence) | Mode | What the person meets | What it needs from the product | Days |
|---|---|---|---|---|---|
| 1 | **Orca** (MIT), the frame | **Embedded, as the frame itself**, rebuilt as ours | Develop: sidebar, terminals, editor, diff, in our look and words | Our thin build: name, build-time rename, token file, journey bar and app-level view, OSC off, cloud features off | 5, counted (02-verify §7); token file +0.5 new (§3) |
| 2 | Orca's own attention surfaces: bell, unread counts, per-agent OS notifications, the experimental Agents feed, the dashboard pop-out | **Off** | Nothing. Their facts reach the Desk (§5.4) | Our build's defaults: `notifications.enabled: false`, `experimentalActivity: false`, `experimentalAgentDashboardPopout: false` (`global-settings-types.ts:312`, `:470`, `:472`); the bell hidden by the build | 0.25 new |
| 3 | Orca's task sources: GitHub, GitLab, Linear, Jira | **Off** | Our Work page instead (row 9) | `visibleTaskProviders: []`, which "hides unused providers from Tasks chrome and sidebar shortcuts" (`:378-379`). Left on, they are a second tracker face that asks for a pasted personal token (05 §0.3) | 0 |
| 4 | Orca's browser pane | **Embedded, Develop only**, for the lane's own dev server | A browser tab beside the terminal, in our look | Never used for our pages or previews: it is per-worktree, has an address bar, and routes through the sandbox (02-verify §7 bar 5) | 0 |
| 5 | **Claude Code** and the other harnesses (unmodified, `OD-8`) | **Embedded**, as the terminal | The real CLI in a pane. That is the product's promise, not a seam (AGENTIC-UX §8) | Orca's custom terminal theme generated from our tokens; the CLI's own theme set to an ANSI-only option if the pinned version has one | 0.25 new |
| 6 | **Spec Kit** + aihero skills (MIT) | **Headless**, inside the Define session | Questions one at a time in the terminal. The brief itself on our Define page, which renders the exact pushed blob, Publish and drift (03-verify §7) | The `panacea` preset in our nouns: *brief* and *criteria*, never *spec* or *AC* (§5.5) | 10, counted (03-verify); wording +0.25 new |
| 7 | **Penpot 2.18** editor (MPL-2.0) | **Embedded**: the one product UI we embed | The editor, opened straight on the branch's file inside our Design page, with our bar and brief chip. Never Penpot's dashboard or login page | Same-site serving with `frame-ancestors` (04-verify 2.5 point 5); an injected theme file (§3); our OIDC-start shim (§4); flags off (§3) | 1, counted (08-verify §7.2); +0.75 new |
| 8 | Penpot's MCP server, one headless runner per active lane file | **Headless**, agents only | The agent's edits arrive live in the person's editor, as a named bot | Runners, bots and the relay (04-verify §5) | 13, counted (04-verify) |
| 9 | **Azure DevOps Boards**, **GitHub Issues/Projects** (Jira later) | **Headless**, behind our Work page | Tickets in one list, in our words. Start on a ticket opens a lane | The `tracker` interface; Start by a pending start plus the recipe (05-verify §5) | 20, counted (05-verify) |
| 10 | **GitHub and Azure DevOps pull requests**, merge queue, auto-complete | **Headless for the author; out for reviewers in v1** | The author lands from the lane card (`panacea lane land` on the laptop, 02-verify §8 item 4). A reviewer reads and answers comments on the host's own PR page | Nothing for the author. The reviewer exception is declared in §7 and closed later by pulling review comments into the Desk | 0 now |
| 11 | **Panacea's API and `panacea-mcp`** | **Headless**, behind our Deploy page and the lane card | Ship, preview, logs, scans, approve and roll back, as our buttons | The allowlist proxy (G4); our copy for Panacea's errors (00-verify S7) | 8.5, counted (06-verify) |
| 12 | Panacea previews: the person's own running app | **Embedded**, in the app-level view | The app itself under our bar, with Ship and Roll back beside it. No address bar | A short-lived share link or the realm session (§4) | 0.25 new |
| 13 | Panacea's Portal (Backstage) | **Out of the person's path** | Nothing. It is an operator console for Panacea's SuperAdmins (06-verify §6) | No link to it from any surface of ours (07-verify C5) | 0 |
| 14 | Panacea's key page, "Connect deploy", until P-3 | **Embedded once**, in the app-level view | One step at the first deploy, then gone | The deep link and a validating paste field (00-verify S3) | 0.25, counted |
| 15 | **Keycloak** (Apache-2.0) and **Entra** | **Headless** | Nothing on the normal path. Stock error pages remain until our own realm has a theme | The hand-off (§4); a Keycloakify theme later (07-verify C2) | §4; 0.5 later, counted |
| 16 | **LiteLLM**, our gateway (MIT) | **Headless** | A refusal is one sentence of ours, in the terminal and on the Desk | `gatekeeper` rewrites LiteLLM's own refusals (07-verify C3). Its admin UI is never linked | counted (07-verify) |
| 17 | **Teams**, with our Agents SDK bot (MIT) | **Out by nature**: a doorway back in | Our bot's cards, in our name and words, each linking to its exact Desk row or lane. The first-party Boards and GitHub channel apps carry tracker events only, never ours | Deep links into the frame (§5.3); decide cards mirror Desk rows both ways (§5.4) | 3, counted (05-verify); +0.5 new |
| 18 | **The Hub** (ours) | n/a | Every page in the app-level view | The shared page template (08-verify §7.2) | counted |

**The embeds, justified.** Each one is either the frame or something no headless form can give:
1. **Orca** is the frame. Its integrated Develop whole is why it was adopted, and our build makes
   it ours (row 1).
2. **Claude Code in a terminal** is the promise: genuine CLI output (AGENTIC-UX §8). Wrapping it
   would break `OD-8`.
3. **Penpot's editor**: a person cannot design through a headless canvas, and an approval needs
   the approver's own tab to save and pin the version (04-verify 2.3 row 11). Agents stay headless
   (row 8).
4. **The preview** is the customer's own product, the thing being built. It is not a vendor's
   face.
5. **Orca's browser pane** serves only the lane's own dev server, which is Develop's business.
6. **Panacea's key page** is one step, and P-3 removes it.

**Never linked from any surface of ours:** Panacea's Portal, LiteLLM's admin UI, Keycloak's
account console, Penpot's dashboard, and Orca's account, artifact-sharing and mobile screens
(switched off, 02-verify §7).

## 3. One design system across every surface

**One source, generated outward.** `desktop/src/styles/tokens.css` (48 variables) and
`docs/DESIGN-SYSTEM.md` are the only place a colour, radius, font or duration is decided. A small
generator writes one mapping file per foreign surface from them: `orca-theme.css`,
`penpot-theme.css`, the terminal theme, and later the Keycloak login theme. A test resolves every
mapped variable and fails when it no longer equals its source token, so no surface drifts when a
token changes.

**One type pairing, decided first.** Three sources disagree today. `tokens.css` ships Segoe UI
Variable and Cascadia Mono (`:151-152`), DESIGN-SYSTEM names Geist Sans plus one mono, and Orca
already ships Geist. **Use Geist Sans (OFL) and Commit Mono (OFL) on every surface**, and change
`--sans` and `--mono` to match. Berkeley Mono (about US$100 once, DESIGN-SYSTEM) is the paid
alternative for the mono.

### 3.1 Orca: chrome, panels, terminals

Orca's dark theme is **125 custom properties** in `.dark` (`src/renderer/src/assets/main.css:265-404`
at `59b746ff`), not the 20 its panels receive. They cover the shadcn set, a sidebar set, a
worktree-sidebar set, status, diff and git colours, terminal pane titles and an `--orca-security-*`
set. Most derive from about twelve of ours, so `orca-theme.css`, loaded after `main.css`, is one
generated file. 02-verify §7 priced five mappings at 0.5 day; at 125 it is about 1.

| Orca variable | Ours | Note |
|---|---|---|
| `--background`, `--sidebar`, `--worktree-sidebar` | `--page`, `--canvas` | Warm graphite, not Orca's `#0a0a0a` |
| `--card`, `--popover`, `--editor-surface` | `--surface` | |
| `--muted`, `--secondary`, `--input` | `--inset`, `--field` | |
| `--foreground` and every `*-foreground` on a surface | `--ink`, `--ink-2` | `--ink-3` is the floor for readable text |
| `--border`, `--sidebar-border`, `--worktree-sidebar-border` | `--line` | Hairlines carry structure |
| `--primary`, `--ring`, `--sidebar-ring`, `--worktree-sidebar-ring` | `--accent`, `--accent-ring` | Teal only on interactive and current state |
| `--accent`, `--sidebar-accent` | `--hover` | **The shadcn trap:** shadcn's `--accent` is a hover surface, ours is the teal. Map by meaning, never by name |
| `--destructive`, `--status-success`, `--status-warning`, `--workspace-status-*` | `--fail`, `--run`, `--need`, `--idle` | Each status keeps a shape as well as a colour |
| `--radius` | `--r-card` | Already 10px (`0.625rem`) |

**Settings our build sets:** `theme: 'dark'` for v1 (our light ramp is derived later, not
inverted); `appFontFamily` Geist; `terminalCustomThemes` generated from ours
(`global-settings-types.ts:80`, `:87`, `:143`). The `.titlebar-logo` (`main.css:900`) shows our
mark. Status dots stay static, one per row, in our colours and shapes. Orca's per-row animations
are switched off in our file, so the only live motion in the frame beats on `--beat` (§3.3).

**Panels inherit.** Orca hands plugin panels its allowlisted tokens "so panel documents can match
the app" (`plugin-panel-design-token-css.ts:3-8`). Once our file lands, the lane card is ours with
no further work.

**What stays Orca's without a renderer patch:** layout, spacing, component shapes, its own
transitions and shadows. That is accepted: they are the same shadcn family as ours. Tailwind v4
exposes shadows and radii as theme variables, so those may be mappable too (the `@theme inline`
block at `main.css:43` was not read).

### 3.2 Penpot's editor

At tag 2.18.0 Penpot's UI colour comes from about **42 semantic `--color-*` variables per
theme**, declared under a global `.default` (dark) and `.light` class in
`frontend/src/app/main/ui/ds/colors.scss` (84 declarations). The 339 component variables in
`frontend/resources/styles/common/refactor/design-tokens.scss` point at them. So colour can be
themed at runtime without touching a Penpot file:

- **How:** our ingress serves `penpot-theme.css` from our own origin and adds one `<link>` to
  Penpot's `index.html` (an nginx `sub_filter`, or a two-line wrapper image). The file redefines
  the semantic variables for `.default` and `.light`: `--color-background-primary` to
  `-quaternary` from `--page`, `--canvas`, `--surface`, `--inset`; `--color-foreground-primary`
  and `-secondary` from `--ink`, `--ink-2`; `--color-accent-primary` from our teal (Penpot's own
  dark accent is mint `#00d1b8`, its light accent purple); success, warning and error from
  `--run`, `--need`, `--fail`. G10's CSP (04-verify 2.5) must allow that one stylesheet.
- **Flags off.** These exist in `common/src/app/common/flags.cljc` at 2.18.0, but their effect
  was not checked: `onboarding`, `dashboard-templates-section` and `subscriptions` (Penpot's
  premium prompts). `telemetry` and `admin-console` are already off (04-verify §5).
- **What cannot be themed at runtime:**
  - **Radius:** Sass values compiled in (`$br-4`, `$br-6`, `$br-8`, `$br-12` in
    `ds/_borders.scss`). These are near our 6/8/10/14, so accept them.
  - **Font:** hard-coded as `font-family: "worksans", "vazirmatn", sans-serif` in
    `ds/typography.scss`. The fallback is an `@font-face` in our file that serves Geist under the
    family name `worksans`. That needs a spike: it must not change fonts inside designs, which
    Penpot loads separately.
  - **Icons, layout and motion** stay Penpot's. They sit inside our Design page, the way a
    document sits inside an editor.
  - **The canvas content** is the customer's design, and is never themed.
- **If a file must change** (the radius, the main-menu logo, a word): Penpot is MPL-2.0, so we
  may modify it. Every changed file that ships into a customer tenancy must have its source
  available (MPL-2.0 §3.2), so publish the changed files in a public repository beside the image.
  Injecting our own stylesheet changes no Penpot file, so it triggers no duty.

### 3.3 Every other surface, and the clock

| Surface | How it takes our tokens | What stays foreign |
|---|---|---|
| Hub pages: Desk, Work, Define, Design wrapper, Deploy, the thread drawer, the loopback page | `tokens.css` directly; shadcn/ui on Tailwind v4 (DESIGN-SYSTEM §Stack); the one page template (08-verify §7.2) | Nothing |
| Claude Code in the terminal | The ANSI palette of Orca's custom terminal theme, generated from ours | The CLI's own layout, by design |
| The preview | Not themed. It is the customer's app | The whole page, by design |
| Keycloak pages | Stock until our own realm carries a Keycloakify theme (MIT, 07-verify C2) | Error pages, rarely seen |
| Teams cards | Adaptive Cards take the Teams theme. Ours: the name, the words, one accent where the schema allows | Teams itself |
| The git host's PR page | Not themed (reviewers only, the v1 exception) | The page |

**One heartbeat.** `--beat` is 2400 ms, phase-aligned from a shared epoch (`tokens.css:139-142`).
Orca's per-row animations are off (§3.1), so no second phase appears in the frame. Penpot's
short UI transitions are not live indicators, so they stay. The house curve and durations
(`--ease`, `--dur-*`) apply to every surface we render.

**The fallback ladder**, where a product cannot be themed by variables:
1. Inject our stylesheet at our own edge.
2. Patch the file in our build. For MIT (Orca), keep the notice. For MPL-2.0 (Penpot), publish
   the changed files.
3. Keep it out of the person's path (the Portal).
4. Declare it as an exception in §7, for the owner's eyes.

## 4. One sign-in, one session

**The rule:** Entra once, through the realm, with zero clicks (`OD-30`), and no second sign-in
page anywhere in the frame. Today's plan covers two of the three places a person's session is
needed. It misses the third.

| # | Who needs a session | Where it comes from | Covered by |
|---|---|---|---|
| 1 | Our build of Orca, the plugin, the recipe scripts, the laptop CLI | `panacea login`: auth-code + PKCE through the realm, loopback, in the system browser, where Windows signs Entra in silently. The token lives in Windows Credential Manager (`OD-6`) | 02-verify §6 (1 day) |
| 2 | Hub pages in the app-level view | A one-time ticket bought with that token: 60 seconds, single use, bound to the caller, exchanged for an `HttpOnly` Hub cookie | 07-verify §6 C1 (1 day) |
| 3 | **Every other realm client inside the frame:** Penpot, previews behind oauth2-proxy, Panacea's key page | **A realm SSO session in the frame's own cookie store.** Electron's Chromium keeps its own store and does not use Windows sign-in, so the first of these shows Keycloak's page and then a Microsoft sign-in (00-verify 4.3 and S4; 02-verify §7 bar 2) | **Nobody. This section** |
| 4 | Agents in sandboxes | A per-session gateway token through `apiKeyHelper`; Penpot bot keys held by the relay. Never a person's session | 01, 04, 07 (counted) |
| 5 | Reviewers on the host's PR page | Their own GitHub or Azure DevOps sign-in, in their own browser | The v1 exception (§7) |

### 4.1 The fix: hand the realm session into the frame

1. **One partition.** Every web surface in the app-level view shares one Electron session
   partition, so the Hub, Penpot and previews share one cookie store: one session, one sign-out.
2. **The hand-off.** At start, our build loads the realm's authorisation endpoint in that
   partition for a dedicated client, `panacea-frame`, carrying a one-time hand-off ticket from the
   Hub (row 2's ticket kind, with audience "realm"). That client's browser flow is overridden to
   begin with **our own small authenticator**. It checks the ticket's signature against the Hub's
   keys, an expiry of 60 seconds or less, single use (Keycloak's single-use object store) and the
   user. The realm then sets its own SSO cookie in the frame's store. No page renders.
3. **Everything else is then standard SSO.** Penpot, oauth2-proxy previews and Panacea's pages
   complete silently, because a realm session now exists in that store.
4. **Penpot's own login screen never shows.** Penpot starts OIDC with `POST /api/auth/oidc`, which
   answers `{redirect-uri}` (`backend/src/app/auth/oidc.clj:1011-1026`, routes `:1162-1171`, at
   2.18.0). Our Design page makes that call and sends the frame to the answer. The same call
   carries an `invitation-token`, so the first visit also accepts the team invitation, which
   removes 04-verify 2.3 row 12's visible step.

**Licence and cost.** The authenticator is our own code on Keycloak's SPI (Keycloak is
Apache-2.0). `p2-inc/keycloak-magic-link` does this job but is **Elastic License 2.0** (its
`COPYING`, read today), which is restricted: it forbids providing the software as a hosted or
managed service and circumventing its licence-key functions, so we could not ship it into a
customer tenancy. The authenticator is about **1.5 days** (code and tests 1; the client, the flow
and the shell's call 0.5), plus a security review, because it turns a bearer ticket into a browser
session. The Penpot shim is **0.25**.

**Where it installs:** in a realm we can extend. On Panacea's dev Keycloak 25 it is a Panacea image
change, queued behind the cutover freeze (06-verify §2.4). On our own realm (07-verify 3.11, about
2 days, with IT's request filed on day 0) it is ours. **Recommend our own realm for it**, with
Penpot's OIDC client in that same realm, and the pilot on §4.2's bridges until it exists.

### 4.2 Until the hand-off exists

- **Previews: a share link, and no realm session at all.** Panacea mints "scoped, expiring share
  tokens" (`routes.go:1438-1442`): `POST /api/v1/projects/{name}/share-links {host, expiresIn}`,
  any positive duration up to 30 days, at most 50 active per project
  (`handlers/share_links.go:25`, `:52-58`). The control plane mints a 30-minute link per preview
  per person, reuses it while the preview is open, and revokes it after. Only for data classes
  `none` and `synthetic`: a `protected` preview waits for the realm session. About 0.25 day.
- **Penpot and the key page: a Windows sign-in bridge.** On Windows, browsers get Entra single
  sign-on from a primary-refresh-token cookie that Windows issues to them (Edge natively; Chrome
  and Firefox through the same Windows API). Our build's main process could attach that cookie to
  the frame's requests to `login.microsoftonline.com`, so Entra signs in silently and
  device-compliant inside the frame. This is from knowledge, not read today. It is Windows only,
  and a native module inside an app the EDR already scores, so it is a **pilot bridge: spike it
  first, about 1 day**. It is never the design.
- **If both fail:** the pilot shows one Microsoft sign-in inside the frame the first time Penpot
  opens. That is a declared seam (§7), not an accepted state.

**Sign-out is one act.** It deletes the Credential Manager token, clears the frame partition (Hub,
realm and Penpot cookies) and calls the realm's end-session endpoint. **A lapsed realm session**
re-runs the hand-off silently from the stored token. The person only signs in again if Windows
itself asks.

**What must never appear in the frame:** a Keycloak or Microsoft page on the normal path, Penpot's
login screen, an Orca account prompt, a pasted Linear or Jira token (Orca's task sources are off),
or a GitHub sign-in.

## 5. One navigation, one vocabulary

### 5.1 The journey bar: one spine, four lenses, no pipeline

| Slot | What it holds | Rule |
|---|---|---|
| Desk, Work | The two surfaces above a branch: the Desk (account level, the one queue, §5.4) and Work (a project's tickets and team strip) | The Desk is always the landing surface (AGENTIC-UX §5) |
| Project ▸ branch | One context chip, with a picker on each half | One context line, never repeated breadcrumbs (AGENTIC-UX §9 item 8) |
| Define · Design · Develop · Deploy | The four lenses on the current branch | Equal weight. No arrows, numbers, ticks, counts or "next" language. Any lens opens from any state (AGENTIC-UX §4 rule 12; §9 items 1 and 12). Only the current lens wears the accent |
| The brief chip | `brief:bf-…@n`, shown as "Brief r4", plus a drift mark when the current session started on an older revision | Opens the thread (§5.2). Never a count |
| ⌘K | The palette (§5.3) | The same palette on every surface |

**Context follows the person.** Switching lens keeps the project, branch and brief revision.
Choosing a worktree in Orca's sidebar moves the bar's branch, and choosing a branch in the bar
opens its lanes in Develop. A `lane` deep link on the registered scheme (02-verify §7) lets the
Desk, a Teams card or the thread land on the exact lane. Scroll and reading position persist per
surface (AGENTIC-UX rule 11).

**Two words the owner settles** (both customer-visible; `CONTEXT.md` open questions 1 and 2):
- **Define or Discover.** The code and the phase say Discover; `CLAUDE.md` and every owner message
  of 2026-09-30 say Define. One word must win everywhere. Recommend **Define**.
- **Four lenses or six.** Under Orca, Files and Merge stop being lenses. They become Develop's own
  explorer and source-control views, and landing is the lane card's Land button. So the bar
  carries four, which is what rule 12 says. Recommend **four**, and edit the `Lens` type and
  `CONTEXT.md` in the same change.

### 5.2 The thread: one element, in every view

**What it is:** the provenance thread drawn as records, never as progress (`CLAUDE.md` product
spine; AGENTIC-UX rule 10). One component renders it everywhere: the brief card from the shared
page template (08-verify §7.2), collapsed into the chip and expanded into a drawer.

| Record, in thread order | What it shows | Opens |
|---|---|---|
| Brief revision | Settled answers; each criterion as settled, open or contested, with its evidence sentence | The Define page at that revision |
| Design approval | The design revision, the brief revision it was approved against, the snapshot | The Design page at that version |
| Sessions | Each session's `contextRef`, with a drift mark when it is older than the latest revision | The lane in Develop, at the span |
| Lanes and pull requests | The lane, its PR, and the brief trailer on it | The lane card; the host PR page for a reviewer |
| Release | Reserved, confirmed, abandoned or expired; the pinned brief; both signatures with key ids (06-verify §6) | The Deploy page's release record |

**Where it shows:** the chip in the journey bar (every screen), the lane card in Orca's right
panel (Develop), the header of every Hub page, the release record, the PR description (through
the brief trailer), and one line on every Teams card. It is the same Hub data each time.

**What it never shows:** a percentage, a bar, a tick or "N of M met" (AGENTIC-UX anti-pattern 2).
A lens is never marked done because its records exist.

**Cost:** the chip and the card sit inside 02-verify's journey bar and 08-verify's template. The
drawer, with every record linking to its source, is **about 1 day new**.

### 5.3 One palette that reaches every capability

**The palette is Orca's Cmd+J palette, bound to ⌘K as well.** At `59b746ff` plugin commands
already appear in it. A selected action whose id starts with `plugin:` runs the plugin's command
(`use-worktree-jump-palette-selection-actions.ts:193`), and `AppCommandSource` is
`'plugin-keybinding' | 'plugin-palette'` (`lib/app-command-dispatch.ts:3`). So the Panacea
plugin's `commands` land in the one palette with no fork. They can also take keybindings
(`plugin:<key>/<commandId>`) and alias a closed list of built-in actions
(`shared/plugins/plugin-command-actions.ts`).

**What the plugin contributes, in our words:**

| Capability | Commands |
|---|---|
| Desk | Open the Desk · Quiet for 30 minutes · Pause everything |
| Define | Define from an idea, a repo or a ticket · Publish brief · Open the brief |
| Design | Open the design · Ask for design approval |
| Work | Start on a ticket · Open Work |
| Develop | New lane on this branch · Fork this session · Land this lane |
| Deploy | Ship a preview · Open the preview · Request promotion · Roll back |
| Thread | Open the thread · Go to the session behind this |

**Three pieces of glue make it one palette everywhere:**
1. **Lens moves are built-in actions.** Our build adds `panacea.lens.*` and `panacea.desk.open`
   to Orca's closed alias list, inside the journey-bar change, so the plugin can alias them.
2. **The chord works on every surface.** Keys typed inside the app-level view, including inside
   Penpot's frame, never reach Orca's renderer. Our build's main process catches ⌘K and Ctrl+K on
   that view with Electron's `before-input-event` and opens the palette (from knowledge, not
   tested). A Penpot shortcut on the same chord loses.
3. **Coverage is a test.** A registry lists every person-facing action on every Hub page and in
   the lane card. The build fails if any of them has no palette entry.

**Cost: about 1 day new.** The commands are thin calls to routes each cluster already builds. The
plugin worker can make them today, because it is unsandboxed (02-verify §2 row 3). When Orca ships
a scoped `net:fetch`, the manifest declares it and the pin-move canary runs the palette.

### 5.4 One attention queue: the Desk

**Only the Desk ranks, counts and asks** (AGENTIC-UX rule 4; anti-pattern 10). Orca's bell,
unread counts, Agents feed and per-agent notifications are off (§2 row 2). Orca's sidebar keeps
one static status dot per row, in our colours and shapes, and no numbers. This also retires the
queue 02 adopted from Orca, which reads OSC status out of PTY bytes that any terminal output can
forge (02-verify 2.1). The Desk takes typed facts only (`OD-8`).

| Source | How it reaches the Desk | Rows it makes (AGENTIC-UX's classes) |
|---|---|---|
| Agents in lanes | Claude Code's typed HTTP hooks, through `sandboxd`, to the Hub: 02 R6's "later" relay (1.5 days, counted), **moved to first ship** | NEEDS YOU (a permission prompt, a question), FAILED, DECIDED WITHOUT YOU, LANDED; RUNNING with its meaning and next lines |
| Trackers (ADO, GitHub) | Service hooks and webhooks, or polling (05-verify §8), through the control plane | NEEDS YOU: a ticket handed to you, proof contested |
| The git host | Review events on the lane's PR | NEEDS YOU: changes requested, with the host page as the one link out (v1) |
| Penpot | Webhooks (`enable-webhooks`, 04-verify 2.3 row 10) | NEEDS YOU: a mention, an approval asked |
| Panacea | The control plane's deploy and release polling | FAILED with the deterministic diagnosis (06-verify §6); NEEDS YOU for a promotion; LANDED when a preview or release is ready |
| Teams (our bot) | Not a source: a mirror | A decide card **is** a NEEDS YOU row. Answering in Teams resolves the Desk row, and the reverse. One record, two doorways |

**Answering.** Tracker, design and deploy rows are answered in place, through the routes behind
them. An agent's permission prompt is answered in place once a typed answer can reach the harness
(a hook that waits on the Hub is the likely route; not designed here). Until then, Enter dives
into the lane's terminal at that prompt (AGENTIC-UX's DIVE).

**Notifications are one digest.** At most one per person per 15 minutes, from the Hub, as one OS
notification from our build or one Teams message. Never a toast per agent, and never a modal from
a background session (AGENTIC-UX anti-patterns 8 and 11).

**The Desk itself is costed by no cluster.** Every "Desk" in this folder is a mention, not a
costed row, because 02 leaned on Orca's inbox. That inbox cannot take tracker, Penpot or Panacea
events: Orca's status store is built in, not pluggable (02 R2). A v1 Desk on the shared page
template (ingest, the four classes plus RUNNING, the rank reason, aging, a read cursor, in-place
answers) is **about 3 days**. The source adapters are **1.5** and the Teams mirror **0.5**. All
new.

### 5.5 One vocabulary: our words on every surface

`CONTEXT.md`'s nouns are the only nouns a person reads. Each product's own words are mapped where
the product surfaces:

| Product | Its word | Ours | Where the mapping happens |
|---|---|---|---|
| Orca | Orca | our name | Build-time rename over the English catalogue; the CLI, `orca.yaml` and `orca-plugin.json` excluded (02-verify §7) |
| Orca | workspace | worktree | The same rename list (`CONTEXT.md`: a workspace is nothing here; 08-verify R5) |
| Orca | repository, repo | project | The same list |
| Orca | agent, in the agent picker | harness | The same list. "Agent" stays for what runs in it |
| Orca | merge, "Merge when ready" | land | The same list. The host keeps "pull request" |
| Spec Kit | spec, specify, feature | brief | The `panacea` preset's templates and command text (03 §5: presets override terminology) |
| Spec Kit, and 03-verify's preset | `SC-`, `AC-`, acceptance criteria | criteria | The same preset. **A correction to 03-verify §7:** its `AC-` section contradicts `CONTEXT.md` ("not «acceptance criterion» abbreviated to «AC»") |
| Penpot | Penpot | nothing | The main-menu logo and page title, by the injected stylesheet or an MPL-2.0 patch (§3.2). Design words (board, layer, component) stay: they are the canvas's own domain, not our nouns |
| Panacea | innovation, secure-test, production | release rungs | Add them to `CONTEXT.md`, or map them, in the copy review (06-verify §6) |
| Panacea | deployment; raw error strings | release; one sentence of ours | The control plane maps them (00-verify S7) |
| Panacea | `none`, `synthetic`, `protected` | the three data classes | Add the values to `CONTEXT.md`'s Data classification entry |
| ADO, GitHub | work item, issue | ticket | Our Work page and the Desk (AGENTIC-UX §9 item 7 already says "ticket"); add it to `CONTEXT.md` |
| LiteLLM | its refusal text | one sentence of ours | `gatekeeper` (07-verify C3) |

**Held by tests, not by care:**
- **A banned-word scan in CI** over our build's catalogue, every Hub page's strings and the
  preset. The list is `CONTEXT.md`'s "Words we do not use" plus Orca, Penpot, Backstage, Keycloak,
  LiteLLM, spec, PRD, workspace and dashboard. They are allowed only in the About screen's
  open-source notices, which MIT and MPL-2.0 require.
- **A copy review** of each new page before merge.
- **Not ours to rename:** the harness's own output in the terminal, and the canvas's design terms.
  Both are tools doing their jobs, not product nouns.

**Cost: about 0.75 day new** (the scan 0.25, the copy review 0.25, the `CONTEXT.md` entries 0.25).
The preset's wording is §2 row 6's 0.25.

## 6. Cohesion acceptance criteria

**How they run.** Walk one journey on the pilot build, from a cold start on a managed KPMG
laptop, and check every criterion at every step. The automatable ones also run on every pin move
of Orca or Penpot (02-verify §8 item 1). They extend 08-verify's R6 journey test and 02-verify's
five checks, and replace neither.

**The walk:** J1 open the app. J2 the Desk. J3 Define a new project from an idea and publish the
brief. J4 Design: an agent drafts in Penpot, the person edits and approves. J5 Develop: start a
lane from a ticket, work in it, fork a session. J6 land the lane. J7 Deploy: open the preview,
ship, confirm the release. J8 come back the next morning to the Desk. J9 answer one decide card in
Teams.

| # | Criterion | Passes when | How it is tested |
|---|---|---|---|
| K1 | **One name** | No screen, window title, OS notification, installer or taskbar entry shows Orca, Penpot, Backstage, Keycloak, LiteLLM or Spec Kit, except the About screen's open-source notices | The CI scan (§5.5); a DOM text scan at each step; the installer by hand |
| K2 | **One sign-in** | From a cold start, J1-J8 render no sign-in page: none from `login.microsoftonline.com`, the realm, Penpot or Panacea's Portal. No token is pasted after the one-time Connect step, and none once P-3 lands | Record the frame's navigations during the walk; any interactive login document fails |
| K3 | **One frame** | Every step J1-J8 completes inside the window. The only exits are the first-run sign-in tab and a reviewer's host PR page | Count external opens during the walk: zero after first run |
| K4 | **One spine** | Every screen shows the project, the branch and the brief revision key. Every Hub page carries `brief:bf-…@n` in its header, and the lane card shows it in Develop | A DOM assertion per step |
| K5 | **The thread is whole** | From any screen, one click shows the same brief revision, design approval, sessions, lanes and PRs, and release, each opening its record. No `%`, bar, tick or "N of M" appears anywhere | Click through each record; scan text for `\d+ of \d+` and `%` |
| K6 | **Lenses, not a pipeline** | Four equal lenses: unnumbered, no arrows, no ticks. Each opens from each other lens on a fresh project | Visit all twelve lens pairs |
| K7 | **One palette** | ⌘K opens the same palette in Develop, on every Hub page and inside Penpot's editor, and every registered action is in it | The registry coverage test (§5.3); the chord pressed on each surface |
| K8 | **One queue** | One synthetic event per source (a permission prompt, a ticket handed over, a Penpot mention, a failed deploy, changes requested, a Teams answer) each makes exactly one ranked Desk row. No count or badge appears anywhere else | Inject the six events; scan every surface for badges |
| K9 | **One look** | Every surface we control renders our tokens: teal only on interactive or current state, warm graphite grounds, our status colours with shapes. None of Orca's `#0a0a0a` and `#171717`, Penpot's mint `#00d1b8` or its purple accent is visible | A computed-style probe of named elements per surface, then K14 |
| K10 | **One vocabulary** | No banned word (§5.5) on any surface we control. Panacea's errors arrive in our copy | The CI scan; a forced Panacea error during J7 |
| K11 | **One clock** | Every live indicator shares `--beat`'s phase. No spinner per row, and Orca's rows carry no animation of their own | Read animation names and delays from the DOM during an 8-lane run |
| K12 | **Quiet by default** | At most one OS notification or Teams digest per person per 15 minutes in an 8-lane run. No toast per agent, no modal from a background session | Count them over a timed 30-minute run |
| K13 | **Context survives** | Moving between lenses and back keeps the project, branch, brief revision and scroll position. A lane link from the Desk or a Teams card opens that exact lane | State assertions during the walk |
| K14 | **The owner's eyes** | The owner walks J1-J9 and answers one question: "Does this feel like one app?" A no fails the build, whatever K1-K13 say | Hands-on, batched as `OD-12` sets. Until then every visual claim in this file stays unverified: headless screenshots are blocked on the corporate network, and "visual acceptance is the owner's eyes" (`CLAUDE.md`) |

**Before the build, not after it:** the owner reviews a clickable prototype of the shell (the bar,
the Desk, one Hub page and the Develop frame), as the working agreements require (`CLAUDE.md`;
08-verify §8).

## 7. What it changes

**No product pick changes.** Every cluster's corrected choice stands: Orca, Kata sandboxes, Spec
Kit, Penpot, ADO Boards and GitHub, Teams, Panacea, our Hub and gateway. What changes is how
several of them surface. Ten pieces of the plan, as written, cannot pass the owner's ask:

| # | As planned | Why it fails the ask | What replaces it | New days |
|---|---|---|---|---|
| 1 | Orca under its own name, our pages in its browser tabs (02 §7 call 1; 08 §9 call 2) | Fails four of the five checks at source | The thin build (02-verify §8), already ruled | 0 (5 counted) |
| 2 | Orca's inbox and status store as the attention queue (02 R1 row 3) | A second queue with counts. It cannot take tracker, Penpot or Panacea events, and it reads PTY bytes | The Desk as the one queue (§5.4); Orca's own surfaces off (§2 row 2); the hook relay moves to first ship | 3 + 1.5 + 0.5 + 0.25 |
| 3 | Orca's task sources as Start for GitHub and Jira (05 §0.3) | A second tracker face, with pasted personal tokens | The Work page and the pending start (05-verify §5) | 0 |
| 4 | Penpot as delivered: dashboard, login screen, onboarding, mint and purple chrome | Another product's face in the Design lens | The editor alone on the branch's file, the injected theme, the OIDC shim, flags off (§3.2, §4.1) | 0.75 |
| 5 | Penpot and previews signed in by the embedded browser | A second Microsoft sign-in inside the frame | The realm hand-off (§4.1), share-link previews (§4.2), the Windows bridge as a pilot fallback | 1.5 + 0.25 |
| 6 | 03-verify's preset, with its `AC-` section and Spec Kit's "spec" | Words that are not ours | The preset in our nouns (§5.5) | 0.25 |
| 7 | Three type pairings across the surfaces | A different face on each surface | Geist Sans and Commit Mono everywhere (§3) | 0.25 |
| 8 | A palette that reaches only Orca's own actions | Two palettes, or none on our pages | Plugin commands, alias actions, the chord caught on every surface (§5.3) | 1 |
| 9 | The thread as a chip and a card only | Its records do not open from every view | The thread drawer (§5.2) | 1 |
| 10 | Each product's words as shipped; five theme mappings, not 125 | Several vocabularies; Orca's chrome only half ours | The rename list, the CI scan and the copy review (§5.5); the generated theme files (§3.1); the terminal theme | 0.75 + 0.5 + 0.25 |

**The days.**
- **New, from the table: about 12 engineer-days (10 to 15).** That is 11.75 before rounding.
  The Desk (3 days) is the largest piece, and no cluster had costed it.
- **Pulled into the plan: 2 days.** Our own realm, which 07-verify §5 kept as a costed fallback
  outside its total, now carries the hand-off (§4.1).
- **Contingent: 1 day.** The Windows bridge, only if the realm hand-off is not ready for the pilot.
- **Moved, not new:** 02 R6's hook relay (1.5 days) moves from "later" to first ship.
- **So the one-app layer is about 23 engineer-days:** 08-verify §7.2's 9, plus 12 new, plus the
  realm's 2. The whole-product sum moves from about 103 to **about 117**.
- **Calendar:** with five builders, about 2 to 3 working days more before the first app. The Desk
  and the hand-off sit on its path. The rest runs beside the other lanes.

**Declared v1 exceptions.** Each is small, named, and put to the owner's eyes rather than hidden:
1. **Reviewers on the host's PR page.** Authors never go there, because they land from the lane
   card. A reviewer reads and answers comments on GitHub or Azure DevOps. Closing it later means
   pulling review comments into the Desk and the lane card with reply in place: about 2 days,
   after v1.
2. **The first-run sign-in tab.** One system-browser tab showing our own page, once per laptop.
3. **Penpot's font and radii** (Work Sans; 4/6/8/12), unless the font-alias spike passes or we take
   an MPL-2.0 patch (§3.2).
4. **"Connect deploy"**: one paste until Panacea's P-3 lands (00-verify S3).
5. **Keycloak's stock error pages**, until our realm carries its theme (0.5 day, 07-verify C2).
6. **Spec Kit's file paths** (`specs/<n>-<name>/spec.md`) in Orca's editor, if the preset cannot
   rename them (not checked, §8).

**If the thin build cannot hold** (KPMG IT blocks Orca's desktop, or Orca's seams break): the
browser route (02-verify 5.3). The Hub is the shell in Edge, and each lane opens Orca's web
client. It passes one name, one sign-in and one spine by construction, and sign-in is simpler
there, because Edge already signs in with Windows and needs no hand-off. It loses Orca's
cross-lane sidebar. Spike it for 0.5 day on day 1, as 02-verify already plans. Phase 2's Tauri
desktop stays frozen as the other fallback.

**Owner calls (customer-visible or money only):**
1. **The app's name and icon** (customer-visible; 02-verify call 5).
2. **Define, not Discover, and four lenses**, with Files and Merge inside Develop
   (customer-visible; §5.1; `CONTEXT.md` open questions 1 and 2).
3. **Accept the six declared exceptions for v1** (customer-visible), each with its closing step.
4. **Our own realm for the hand-off** (customer-visible: sign-in moves off Panacea's dev estate;
   07-verify §8). IT's request for the redirect URI and second secret goes in on day 0.
5. **The mono face** (money): Commit Mono (free, OFL), recommended, or Berkeley Mono (about US$100
   once, DESIGN-SYSTEM).

## 8. What this could not verify

Nothing was installed, built, rendered or run. Every day figure is an estimate composed from other
estimates.

| Area | Not verified | How to close it |
|---|---|---|
| The whole | Whether the result *feels* like one app. Nothing was rendered, and headless screenshots are blocked on the corporate network | K14: the owner's eyes on a clickable shell prototype first, then on the pilot build |
| Orca | That ⌘K can also open the Cmd+J palette; that `before-input-event` catches keys inside Penpot's frame (from knowledge); where the bell lives in the renderer (not located); that `visibleTaskProviders: []` removes all task chrome; the `@theme inline` block (shadows, radii); that the app-level view can be a second web view on one shared partition | The journey-bar spike on the first pin (02-verify §9 item 11) |
| Penpot | That the injected stylesheet wins over the compiled `.default` and `.light` blocks, and which element carries the class; the font alias's effect on text inside designs; what the `onboarding`, `subscriptions` and `dashboard-templates-section` flags hide; that the OIDC-start round trip is silent and that its `invitation-token` joins the team | Inside 08-verify's 1-day Penpot wrap, plus this file's 0.75 |
| Sign-in | The hand-off authenticator is a design, not code, and its fit with Keycloak 25 and 26 is unchecked. The Windows bridge's API and behaviour are from knowledge, not read today. Whether KPMG laptops hold a primary refresh token, and KPMG's Conditional Access policy, are unknown | A day-1 spike of each on a managed laptop |
| Panacea share links | How a preview reads the share token (the bypass-token machinery was not traced); whether a 30-minute host scope suits the preview comments widget; the 50-link cap under per-person minting | A test on the dev estate |
| The Desk | No plan exists. Its 3 days are this file's estimate | A plan, reviewed before build |
| Harness theme | Whether the pinned Claude Code offers an ANSI-only theme, and how the sandbox image sets it | Read the CLI's settings for the pinned version |
| Spec Kit | Whether the preset can rename `specs/<n>-<name>/spec.md` | Read the preset schema at v1.0.13 |
| Teams | Whether a decide card and a Desk row can resolve each other both ways through the Agents SDK's `Action.Execute` | 05-verify's bot build |
| Days | 12 new, 2 pulled in, 1 contingent, all estimated | Re-sum when the plans exist |

## 9. Sources

Read 2026-09-30, read-only. Nothing installed.

- **This programme:** `CLAUDE.md`; `CONTEXT.md`; `docs/DESIGN-SYSTEM.md`; `docs/AGENTIC-UX.md`;
  `docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-12`);
  `desktop/src/styles/tokens.css`. In this folder: `02-fleet-cockpit.md`, `03-define.md` and
  `05-work-team.md`, and `00-`, `02-`, `03-`, `04-`, `05-`, `06-`, `07-` and `08-*-verify.md`, in
  the sections cited.
- **Orca** (`stablyai/orca` at `59b746ff`, through raw.githubusercontent.com and the GitHub tree
  API): `src/renderer/src/assets/main.css`; `src/shared/global-settings-types.ts`;
  `src/shared/notification-settings-types.ts`; `src/shared/plugins/plugin-command-actions.ts`;
  `src/renderer/src/lib/plugin-command-execution.ts` and `app-command-dispatch.ts`;
  `src/main/plugins/plugin-command-registry.ts`; `src/renderer/src/components/cmd-j/quick-actions.ts`
  and `palette-results.ts`; `use-worktree-jump-palette-selection-actions.ts`.
- **Penpot** (`penpot/penpot` at tag `2.18.0`): `frontend/src/app/main/ui/ds/colors.scss`,
  `_borders.scss` and `typography.scss`; `frontend/resources/styles/common/refactor/themes.scss`,
  `themes/default-theme.scss`, `color-defs.scss` and `design-tokens.scss`;
  `backend/src/app/auth/oidc.clj`; `common/src/app/common/flags.cljc`.
- **Panacea** (read-only): `services/panacea-api/routes/routes.go:1436-1444`;
  `services/panacea-api/handlers/share_links.go`.
- **Keycloak extension:** `p2-inc/keycloak-magic-link`, GitHub API metadata and `COPYING`
  (Elastic License 2.0).
