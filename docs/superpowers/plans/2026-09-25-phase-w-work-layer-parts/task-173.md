### Task 173: admin on the desktop: apps, keys, webhooks, MCP connectors, the harness allow-list

W5, stream WD, about 3 hours (an estimate, not a measurement). Depends on task 169 (the W5
desktop wire: `App`, `APIKey`, `CreatedAPIKey`, `RateReading`, `Webhook`, `CreatedWebhook`,
`WebhookDelivery`, `MCPConnector`, `TeamAgentPolicy`, `HarnessId`, `PublishedPolicy`, the admin
members of `WorkIntakeClient`, and its decision 4: admin records are never replica kinds,
`task-169.md:35-44`, `:254-321`, `:467-483`). It also uses task 44's `ConfirmInPlace`
(`task-44.md:130-150`), task 20's `expectNoPrediction` and fixtures, task 170's `setupWork`
harness (`task-170.md:384-389`) and `desktop/src/lib/time.ts`.

Parity: closes, jointly with the Hub tasks named, **L-244** (159, 161), **L-245** and **L-252**
(160), **L-251** (156), **L-256** and **L-197** (161, 166), and **L-180** (166). Design: 05 §1 M
(L-180, L-197), P (L-244, L-245), Q (L-251, L-252), R (L-256); §3.6. OD-16 (an admin governs, never
acts as the person), OD-27 (facts, never a prediction without its rule and span).

**What the reader does.** Dylan, an org admin and the admin of team *Payments*, opens *Settings →
Admin*. *Apps* lists one pending app, *Release notes bot*, by Contoso Tools, asking for *read*;
he reads its redirect address, types *Used by the release team* and presses *Approve*. Under
*Keys* he creates *CI release*, team-scoped, expiring 25 Mar 2027: the secret appears once, in
mono, with *Copy*; he copies it and presses *Done*, and it is gone. Under the list reads *Your
own requests: used 3 · limit 20 · resets 14:05:03*. Under *Webhooks* a delivery reads *failed
· attempt 3 · 500*; he presses *Redeliver* once. Under *Agent policy* he moves *codex* below
*claude-code*, sets the skills library to a commit, and saves. Priya, a member, opens the same
page and sees *Keys* only: her own.

**Decisions made here, so nobody re-derives them:**

1. **Fetched on open, dropped on close.** Each section calls `WorkClient` directly when it mounts
   and holds the answer in `useState`; unmounting drops it. Nothing here sends an SW.16 op,
   writes `workSlice`, or touches `localStorage` (task 169 decision 4; §3.1, *no Hub data on
   disk*). A section re-fetches when the reader presses *Refresh* or after its own write; the
   admin feed kinds are skipped by the replica, so nothing live pushes into these lists.
2. **Who sees what comes from the Hub, not from a claim name.** Access is read from `GET /v1/me`
   (hub-contract §7.2, `:653-664`): `orgAdmin` and `teams[i].role` (`:625`). The desktop never
   parses a role string out of the token, because the org-admin role's name differs between
   the contract (`Hub.OrgAdmin`, `hub-contract.md:320`) and ADR-H1's later ruling
   (`hub.org_admin`, `ADR-H1-hub-architecture.md:789`); `/v1/me` answers the same boolean under
   both. An org admin sees **Apps** and **Connectors**; a team admin sees that team's
   **Webhooks**, **team keys** and **Agent policy**; everyone sees **their own keys**. A section
   the reader may not use is **absent**, not disabled; the Hub's 403 is the second check.
3. **A secret lives in one component, once.** `SecretOnce` receives the `secret` of a
   `CreatedAPIKey` or `CreatedWebhook` as a prop from the create call's answer, renders it in mono
   with *Copy* (`navigator.clipboard.writeText`, only on the press), and *Done* calls `onDone`,
   which drops the parent's state. No other component, store or log receives it; the create's
   answer is split at once into the listed record (kept) and the secret (handed to `SecretOnce`).
   The line reads *Shown once. Copy it now: it is not shown again.*
4. **Rates are facts, in words.** The one `RateReading` is the caller's **own** tightest bucket on
   the `apiKeys` response (task 160's headers; task 169 G8), so it is labelled *Your own
   requests*, never as a key's. It reads *used 3 · limit 20 · resets 14:05:03*: two numbers and a
   time, never *3 of 20*, never a bar or `<meter>`. Each key row shows *last used 14:02* or *never
   used*, and its expiry; no per-key used count until task 160 sends one (G8).
5. **A library is pinned by commit.** The skills form accepts a commit of exactly 40 lowercase
   hex characters; `main`, a tag or a short hash is refused in the form with *A full 40-character
   commit, never a branch*, before any call (task 166 refuses it too, 422).
6. **Every irreversible act confirms in place.** Revoke an app or a key, disable a webhook,
   rotate its secret, remove a connector: each first shows `ConfirmInPlace` with its consequence
   (*CI release stops working at once; anything using it gets 401*). One click never acts.
7. **One press, one call.** *Redeliver*, *Approve*, *Revoke*, *Save* and every create disable
   their button until the answer; a second press while one is out does nothing.
8. **Publish to sandboxes is absent until Phase T merges.** `publishRoute` (below) is `null` on
   this branch, so the button is not rendered at all. Once set, the section posts the fragment as
   the admin and shows the `version` and `digest` the route answered, verbatim, in mono; nothing
   in `admin/` hashes anything (no `crypto.subtle`), so the shown digest cannot be the desktop's.
9. **The MCP addresses are the Hub's, copied, never typed.** *Connectors* shows the Work MCP
   server's two addresses, `<hubBase>/mcp` and `<hubBase>/mcp/read` (task 161), built from the
   signed-in Hub base the desktop already holds, each with *Copy*. They are shown to everyone who
   sees *Connectors*; the allow-list under them is the org admin's.

**Files:**
- `desktop/src/components/admin/access.ts` (+ `access.test.ts`) — who sees which section (new)
- `desktop/src/components/admin/usePressOnce.ts` — one press, one call (decision 7) (new)
- `desktop/src/components/admin/SecretOnce.tsx` — the once-shown secret (decision 3) (new)
- `desktop/src/components/admin/policyForm.ts` (+ `policyForm.test.ts`) — the agent policy
  form's pure checks (decision 5) (new)
- `desktop/src/components/admin/{AdminHome,Apps,Keys,Webhooks,Deliveries,Connectors,AgentPolicy}.tsx`
  (+ one `.test.tsx` each, the seven task 174's sweep names, `task-174.md:281-287`) (new)
- `desktop/src/components/admin/testAdmin.ts` — the fake `WorkIntakeClient` and `Me` fixtures (new)
- `desktop/src/components/admin/admin.css` — tokens only, `DESIGN-SYSTEM.md` (new)
- ⚑ the Settings host: one *Admin* entry mounting `AdminHome` (task 96's settings surface, which
  task 176 extends with `Preferences.tsx`, `task-176.md:62-63`; the host file is **UNVERIFIED**
  until task 96 is built)

Tasks 181 (`SignedLand.tsx`) and 185 (`GroupLinks.tsx`) add their sections to this surface later
(`task-181.md:70-72`, `task-185.md:83-84`); `AdminHome` gives them a slot (below), and neither is
built here.

#### Interfaces — complete: `desktop/src/components/admin/access.ts`

```ts
// desktop/src/components/admin/access.ts — task 173. Who sees which admin section, read from
// GET /v1/me (hub-contract §7.2) and never from a token claim (decision 2).
import type { Org, Team, User } from "../../api/hub";   // H's wire types (UNVERIFIED module path)

/** The /v1/me answer, as hub-contract.md:660-664 gives it. Team.role is the CALLER's role. */
export interface Me { user: User; org: Org; orgAdmin: boolean; teams: Team[]; invitesBound: number }

export type AdminSection = "apps" | "keys" | "webhooks" | "connectors" | "agent-policy";

export interface AdminAccess {
  orgAdmin: boolean;
  /** Teams the caller is an admin of; the only teams whose webhooks, team keys and policy show. */
  adminTeams: Team[];
  sections: AdminSection[];   // in display order; absent means not rendered (decision 2)
}

export function accessOf(me: Me): AdminAccess {
  const adminTeams = me.teams.filter((t) => t.role === "admin");
  const sections: AdminSection[] = [];
  if (me.orgAdmin) sections.push("apps");
  sections.push("keys");                                   // everyone: their own keys
  if (adminTeams.length > 0) sections.push("webhooks");
  if (me.orgAdmin) sections.push("connectors");
  if (adminTeams.length > 0) sections.push("agent-policy");
  return { orgAdmin: me.orgAdmin, adminTeams, sections };
}

/** Task 160's rule (`task-160.md:16-17`, `:394`): the owner of a personal key, an admin of a team
 *  key's team, or an org admin, any key. The Hub decides; this only hides a Revoke it would refuse. */
export function mayRevoke(me: Me, key: { ownerId: string; teamId: string | null }): boolean {
  if (me.orgAdmin || key.ownerId === me.user.id) return true;
  return key.teamId !== null && me.teams.some((t) => t.id === key.teamId && t.role === "admin");
}
```

`orgAdmin` alone never shows a team's webhooks or policy: an org admin who is not that team's
admin governs apps and connectors, not the team's agents (OD-16). The org admin's view of every
team is task 185's and the Hub's, not this surface's.
