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
