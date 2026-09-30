# 09 - Cohesion: the assembled product as one app

**Status:** IN PROGRESS (2026-09-30), written and committed a section at a time.
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
