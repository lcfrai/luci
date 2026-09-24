### Task 42: share a project with a team — UI (owner question Q8)

**Lane D. Depends on: 40, 41 (and 35's routes). About 3 hours. Contract task C4 (an addition: the contract names no share surface).**

`OD-3` makes a brief, an approval and a pin shared **from day one** — but only once a local project
**is** a Hub project in some team. Without a way to do that, phases 4, 5 and 7 write nothing the
team can see. This is the smallest surface that closes the gap, built on owner question Q8's
recommendation: **explicit** sharing from the project's menu, **automatic** linking when the
repository is already shared, and a one-step default — *a new team named after the project*.

**Read `docs/DESIGN-SYSTEM.md` first.** Its non-negotiables, as they bind this surface:
- The popover is a card: `--e-card` (1px ring plus layered shadow, never a flat border), `--r-card`;
  its fields and buttons `--r-ctl` and `--e-btn`. **No gradient, no glassmorphism, no three-card
  grid** — it is a form, laid out as one.
- **The accent is for interactive and current state only**: the primary action (*Share* /
  *Invite*) and the focused field. Team names, member names and state sentences are ink, not accent.
- **Every state designed**: loading (a skeleton that mirrors the final geometry exactly — one
  pixel of layout shift is worse than none), empty (no teams yet), unlinked, linked, no remote,
  signed out, shared elsewhere, error.
- **Motion** only where it explains causality — the popover's open/close is `opacity`/`transform`,
  under 300 ms, the house curve `--ease`, with a `prefers-reduced-motion` variant; nothing else moves.
- **No spinner**: a pending action disables its button and shows its label changing, nothing more.
- **Keyboard first**: focus moves into the popover on open, Escape closes it and returns focus to
  the project row, every control is reachable by Tab, focus rings are the designed ones.

**Visual acceptance is the owner's eyes, batched to the end (`OD-12`)**; everything visual in this
task's report is *unverified by the owner*. `OD-1`'s prototype waiver does not name phase H
(`owner-decisions.md:12`), so this popover's look is drawn first, in task 41's Step 0 prototype
(`prototype/phase-h-hub.html`, owner question Q13, review M14), and this task follows it.
**`feat/hub` is not merged onward until the owner has looked** at both surfaces.

**Files:**
- `desktop/src/components/share/SharePopover.tsx` + `SharePopover.test.tsx` (new)
- `desktop/src/components/share/share.css` (new)
- `desktop/src/components/menu/ContextMenu.tsx` — one project item, shown only with a Hub (modify ⚑: `projectItems`, `:98-110`, and one `useCockpit` read beside the others at `:48-54`)
- `desktop/src/store/slices/authSlice.ts` — `shareFor` (modify)
- `desktop/src/components/AppShell.tsx` — mount the popover once (modify ⚑: one line)

#### Contract — complete

```ts
// desktop/src/store/slices/authSlice.ts — added (never persisted, like the rest of the slice).
  /** The local project whose share popover is open, or null. */
  shareFor: ProjectId | null;
  openShare(projectId: ProjectId): void;
  closeShare(): void;
```

```tsx
// desktop/src/components/share/SharePopover.tsx

/** Renders nothing while store.shareFor is null. Reads GET /v1/projects/{id}/hub-link when it
 *  opens (and on a "project." or "team." hub feed event while open — task 40's hubFeed), and
 *  acts through the client functions of task 40. Holds no Hub data after it closes. */
export function SharePopover(props: {
  client: Pick<CockpitClient,
    "hubLink" | "shareProject" | "hubTeams" | "createHubTeam" | "hubTeam" |
    "hubInvites" | "inviteToTeam" | "revokeInvite">;
  feed: Pick<HubFeed, "subscribe">;
}): JSX.Element | null;
```

The project menu gains **Share with a team…** after *Reveal in Explorer*, calling `openShare`,
**only when a Hub is configured**: `const hubConfigured = useCockpit((s) => s.auth?.hubConfigured
=== true)`, read unconditionally with the other hooks, and the item appended only when it is
true. With `PANACEA_APP_HUB_URL` unset (`hubConfigured: false`), or before the first status read
(`auth` is `null`, task 40), the project menu is phase 2's three items exactly. Task 32 promises
that without a Hub nothing changes for the user, and task 41's footer hides its sign-in on the same
flag; an item whose every call answers 503 `hub_not_configured` would break both (review M6).

**The states, and exactly what each shows:**

| State | Shown |
|---|---|
| loading | a skeleton of the unlinked layout's geometry, `aria-busy="true"` |
| `no_remote` | **This project has no GitHub or Azure DevOps remote, so it can't be shared yet.** |
| signed out (`ApiError` `signed_out`) | **Sign in to share this project.** (the rail footer holds the control, task 41) |
| `unlinked`, no teams | a text field **Team name**, prefilled with the project's name, and **Share** → `createHubTeam(name)` then `shareProject(projectId, team.id)` |
| `unlinked`, teams | the caller's teams where their role is `member` or `admin`, as a radio list, plus **New team…** (the field above); **Share** → `shareProject(projectId, chosen)` |
| `linked` | **Shared with <team name>**, the members by name and role, the pending invites; for an **admin**, an **Invite** form — email field and a role select (`viewer`, `member` — the default — `admin`) — and **Revoke** beside each pending invite; for a non-admin, **Only a team admin can invite people.** |
| shared elsewhere (409 `repo_already_linked`) | the relayed message, verbatim: *"Another team already shares this repository. Ask someone in that team to invite you."* (task 17 fixes that sentence) |
| any other `ApiError` | its `message`, verbatim |

An invite sends nothing by email (contract §7.3): the linked view says **They'll join when they
next sign in with their work account in this organisation.** under the form. The qualifier is
not decoration: binding needs a member account or a verified email domain (A15, task 16a), and a
guest or an account with no email stays `pending` with no remedy in v1 (task 16a's stated
limitation, plan re-review b05 m2). The wording is the owner's to accept (OD-12).

#### Tests to write first — `SharePopover.test.tsx` (RTL, fake client, fake feed)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `the project menu opens it` | `auth.hubConfigured: true`; right-click a project → **Share with a team…** → the popover for that project | a surface nobody can reach |
| `the menu item is absent without a Hub` | `auth: null`, then `auth.hubConfigured: false` → right-click a project → the menu is exactly *New branch*, *Copy path*, *Reveal in Explorer*; no **Share with a team…** | an unconditional item, whose every call answers 503 `hub_not_configured` (review M6) |
| `a first share creates a team named after the project` | `unlinked`, no teams → the field holds the project's name; **Share** → `createHubTeam("Engagement Portal")` then `shareProject(p, tm)`; the linked view follows | a share that needs a separate team-setup step first (the wedge: no wall of configuration) |
| `sharing into an existing team` | two teams, one where the caller is a `viewer` → only the other is offered; choosing it → `shareProject(p, that team)` | a share into a team the caller cannot write to, answered 403 |
| `linked shows the team and its people` | the team name, members with roles, pending invites | — |
| `an admin invites and revokes` | fill `vera@example.test`, role `viewer`, **Invite** → `inviteToTeam(tm, "vera@example.test", "viewer")`; the invite appears; **Revoke** → `revokeInvite` | — |
| `a member cannot invite` | role `member` → no form, the sentence | a form that always answers 403 |
| `no remote and signed out say why` | each → its sentence and no **Share** | a button that cannot succeed |
| `shared elsewhere shows the Hub's sentence` | `shareProject` rejects with `new ApiError("repo_already_linked", "<the sentence>", 409)` → that message verbatim. The code is a `HubProblemCode`, which task 38 adds and `ApiError.code` admits (review M7), so the fake compiles under `npm run typecheck` | a paraphrase |
| `a feed event refreshes it` | while open, a `team.invite_bound` event → one more `hubTeam` read; the new member appears | a popover showing yesterday's team |
| `escape closes and returns focus` | Escape → `closeShare`; focus on the project row | a keyboard user stranded |
| `loading mirrors the final layout` | while `hubLink` is pending, `aria-busy="true"` and the skeleton's rows match the unlinked layout's count | a skeleton that shifts the layout when data arrives |

`styles/designsystem.test.ts` must still pass over `share.css`.

#### Steps

- [ ] **Step 1:** Read `docs/DESIGN-SYSTEM.md`; the popover states in `prototype/phase-h-hub.html`
  (task 41, Step 0), if it was written; `ContextMenu.tsx`, `common/Popover.tsx`,
  `AppShell.tsx`; task 35's route table.
- [ ] **Step 2:** Tests; they fail. Then the slice fields, the component, the CSS, the menu item,
  the mount.
- [ ] **Step 3:** `cd desktop && npm test && npm run typecheck`.
- [ ] **Step 4:** In `npm run tauri dev` against `devstack.sh up`, signed in as Grace: share this
  repository's own project into a new team; invite `vera@example.test`; sign out; sign in as Vera
  through `devissuer`; open the popover — Vera is a member. **Record what was seen as the
  implementer's observation**; the design is *unverified by the owner* (`OD-12`).
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
cd desktop && npx vitest run src/components/share/SharePopover.test.tsx 2>&1 | tail -2   # 12 passed
cd desktop && npm test 2>&1 | tail -3                                                     # all passed
cd desktop && npm run typecheck                                                           # no output
```

#### What this task does not claim

- **That it looks right**: the owner's eyes only (`OD-12`).
- Role changes and removal of members from the desktop: the Hub routes exist (8, 9); v1's surface
  does not offer them.
- A "shared" mark on the project row in the rail: later, and phase 4's Discover lens may carry its
  own.
- Q8 itself: the recommendation is built; the question stays open for the owner.
