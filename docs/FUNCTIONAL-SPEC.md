# Panacea App — functional spec

Written 2026-09-17. Supersedes assumptions baked into `prototype/app.html`.

## Object model

| Object | Cardinality | User sees the word? |
|---|---|---|
| **Machine** | many per user; one cloud by default, plus "this Mac" | Yes — it is the thing they name and switch between |
| **Sandbox** | one per running session, ephemeral Kata microVM | No — surfaced only as "isolated" and as blast radius |
| **Project** | many per machine; = one git repo | Yes |
| **Branch** | many per project; each separately deployable | Yes |
| **Worktree** | many per branch; an isolated checkout | Yes — the users are engineers, they know the term |
| **Session** | **many per worktree** — agent or shell | Yes |
| **Sub-agent** | many per agent session | Yes, as nested steps |
| **Environment** | one per deployed branch, arbitrary name | Yes |

The change that matters: **a worktree holds many sessions.** Previously one agent per worktree, which is wrong — you routinely want a shell open beside an agent, or two agents on different parts of the same checkout.

## Starting a session — the owner's question

**One click for the common case.** The `＋` on a worktree starts the project's default harness immediately with inherited defaults. No dialog.

**The split button's caret opens the picker** for when you need something else:
- **Harness** — Claude Code, Codex, OpenCode, Gemini CLI, or **Shell** (a plain terminal, no agent)
- **Model** — filtered to what that harness supports
- **Effort** — low / medium / high / xhigh (Claude Code only)
- **Permission mode** — bypass / ask / plan
- **Initial prompt** — optional; blank just opens the session

**Defaults inherit** project → branch → session, so a team standardises once and nobody re-chooses.

**Power users never open it**: `⌘⏎` starts the default harness in the focused worktree; `⌘K` → "new claude in evidence-upload" does it by name.

### Session lifecycle
Every session carries a `⋯` menu: **Resume** (`claude --resume`, restores context), **Fork** (branch the session from this point into a new worktree), **Restart** (same config, clean context), **Rename**, **Duplicate**, **Kill**. Finished sessions stay listed and readable — the transcript is the artifact.

## Capacity

Sessions spend the signed-in user's **own** subscriptions, rotating as each 5-hour or weekly window exhausts, with metered API as backstop. The footer of every session shows which account it is on. A dedicated Capacity view shows each account's utilisation, reset time, and which sessions are on it. Never pooled across users — that violates Anthropic's terms.

## Machines

One cloud machine exists at first sign-in with no create form — claimed from a warm pool. States: `provisioning · running · idle · sleeping · stopped · error`. **Wake is implicit**: typing into a sleeping machine's session wakes it, with inline "waking… 6s". Sizing is only ever asked for a *second* machine, or after an OOM. The local machine appears in the same list with an honest list of what it cannot do — notably that local agents are bounded by your OS user, not a microVM, so it is badged `unsandboxed`.

## Navigation

- **Rail** — machine chip at top, then projects → branches → worktrees → sessions. Right-click any row for its verbs.
- **Sub-rail (in a branch)** — Discover, Design, Develop, Files, Merge, Deploy.
- **Top bar (Develop)** — layout toggle, Authority, Run locally, and the split New-session button.
- **Command palette `⌘K`** — everything, including things with no button.

## Keyboard

`⌘K` palette · `⌘⏎` new default session · `⌘⇧M` machines · `⌘1-9` focus session n · `⌘\` next session · `shift+tab` cycle permission mode in the focused session · `esc` interrupt · `⌘W` close session · `⌘B` toggle rail · `/` slash commands inside a session.

With a terminal focused, the `⌘` chords go to the cockpit, as in VS Code. The palette's **"Send shortcut keys to the terminal"** toggle hands every one of them, `⌘K` included, to the agent instead; `⌘K` pressed outside a terminal still opens the palette. `shift+tab`, `esc` and `/` go to the agent either way. *(Owner decision OD-9, 2026-09-24.)*

## Known defects being fixed in this pass

1. Live tick destroys in-progress typing — preserve focus and draft text across re-render.
2. `uid()` collides on fast successive calls — use a counter.
3. Denying a permission dead-ends the session — composer must always render.
4. New session created off-screen with no focus — scroll it into view.
5. Auto-scroll steals the scrollback — only auto-scroll panes already pinned to the bottom.
6. Canvas tile positions not persisted — store on the session.
7. `editAnswer` splices all later answers — splice one.
8. "Don't ask again for X" flips the whole session to bypass — keep a scoped allow-list instead.
9. Nothing ever sets `ready`, so user-created worktrees can never merge.
10. Merge reorder indexes the unfiltered array.
