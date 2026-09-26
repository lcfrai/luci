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

#### Interfaces — complete: `usePressOnce.ts`, `SecretOnce.tsx`, `policyForm.ts`

```ts
// desktop/src/components/admin/usePressOnce.ts — task 173, decision 7. While a call is out the
// button is disabled and a second press is a no-op; the answer (or error) re-enables it.
import { useCallback, useRef, useState } from "react";

export function usePressOnce<A extends unknown[], R>(call: (...a: A) => Promise<R>) {
  const out = useRef(false);
  const [busy, setBusy] = useState(false);
  const press = useCallback(async (...a: A): Promise<R | undefined> => {
    if (out.current) return undefined;   // a ref, not state: two presses in one tick see it
    out.current = true; setBusy(true);
    try { return await call(...a); } finally { out.current = false; setBusy(false); }
  }, [call]);
  return { press, busy };
}
```

```tsx
// desktop/src/components/admin/SecretOnce.tsx — task 173, decision 3. The only holder of a
// secret. It never logs, never stores, never forwards; Done drops the parent's state.
import type { JSX } from "react";
import "./admin.css";

export function SecretOnce(props: { label: string; secret: string; onDone(): void }): JSX.Element {
  return (
    <div className="adm-secret" role="group" aria-label={props.label}>
      <p>Shown once. Copy it now: it is not shown again.</p>
      <code className="adm-mono" data-secret>{props.secret}</code>
      <button type="button" onClick={() => void navigator.clipboard.writeText(props.secret)}>Copy</button>
      <button type="button" onClick={props.onDone}>Done</button>
    </div>
  );
}
```

```ts
// desktop/src/components/admin/policyForm.ts — task 173, decision 5. Pure; the form calls
// checkPolicy before PUT and shows the first refusal under its field. Task 166 refuses the same.
import type { HarnessId, PutAgentPolicyBody } from "../../api/work";

export const HARNESS_IDS: readonly HarnessId[] = ["claude-code", "codex", "opencode", "gemini-cli", "shell"];
export const DEFAULT_HARNESSES: HarnessId[] = ["claude-code"];   // task 166: unset means this
const COMMIT = /^[0-9a-f]{40}$/;

export type PolicyRefusal =
  | { field: "harnesses"; text: "Allow at least one harness" | "A harness is listed twice" }
  | { field: "commit"; text: "A full 40-character commit, never a branch" }
  | { field: "repo"; text: "Name the library's repository" };

export function checkPolicy(b: PutAgentPolicyBody): PolicyRefusal | null {
  if (b.harnesses.length === 0) return { field: "harnesses", text: "Allow at least one harness" };
  if (new Set(b.harnesses).size !== b.harnesses.length) return { field: "harnesses", text: "A harness is listed twice" };
  if (b.skills.source === "library") {
    if (!b.skills.library || b.skills.library.repoKey.trim() === "") return { field: "repo", text: "Name the library's repository" };
    if (!COMMIT.test(b.skills.library.commit)) return { field: "commit", text: "A full 40-character commit, never a branch" };
  }
  return null;
}

/** Move one harness up or down; the order is the fallback order task 166 stores. */
export function move(list: HarnessId[], id: HarnessId, by: -1 | 1): HarnessId[] {
  const i = list.indexOf(id), j = i + by;
  if (i < 0 || j < 0 || j >= list.length) return list;
  const next = list.slice(); [next[i], next[j]] = [next[j], next[i]]; return next;
}
```

`SecretOnce` has no `useEffect`, no `console`, no store import and no prop but the secret; the
test greps the file for `workSlice`, `localStorage`, `console.` and `sessionStorage` and expects
none. `HARNESS_IDS` is `harness.Catalogue()`'s set as task 169 quotes it
(`control-plane/internal/harness/harness.go:19-25`, the five `ID` constants, read on
`feat/work-layer` on 2026-09-26).

#### Interfaces — complete: `AdminHome.tsx` and every section's props

```tsx
// desktop/src/components/admin/AdminHome.tsx — task 173. Settings → Admin. Renders only the
// sections accessOf(me) names (decision 2); each section fetches on mount (decision 1).
import type { JSX, ReactNode } from "react";
import type { WorkIntakeClient } from "../../api/work";
import { accessOf, type Me } from "./access";
import { Apps } from "./Apps";
import { Keys } from "./Keys";
import { Webhooks } from "./Webhooks";
import { Connectors } from "./Connectors";
import { AgentPolicy } from "./AgentPolicy";
import "./admin.css";

/** Phase T's policy route, once merged (decision 8). null on this branch: no Publish button. */
export type PublishRoute = ((teamId: string, fragment: unknown) => Promise<{ version: number; digest: string }>) | null;
export const publishRoute: PublishRoute = null;

export interface AdminProps {
  me: Me;
  client: Pick<WorkIntakeClient,
    "apps" | "decideApp" | "createAPIKey" | "apiKeys" | "revokeAPIKey" | "createWebhook" |
    "webhooks" | "disableWebhook" | "rotateWebhookSecret" | "deliveries" | "redeliver" |
    "mcpConnectors" | "allowMCPConnector" | "removeMCPConnector" | "agentPolicy" | "putAgentPolicy">;
  hubBase: string;               // the signed-in Hub's https base, for the two MCP addresses
  timeZone: string;              // the person's; times read through desktop/src/lib/time.ts
  publish?: PublishRoute;        // tests inject; production reads publishRoute
  /** Slots for later sections on the same surface: task 181's SignedLand, task 185's GroupLinks. */
  teamExtras?: (teamId: string) => ReactNode;
}

export function AdminHome(p: AdminProps): JSX.Element {
  const a = accessOf(p.me);
  const has = (s: (typeof a.sections)[number]) => a.sections.includes(s);
  return (
    <div className="adm-home">
      <h1>Admin</h1>
      {has("apps") && <Apps client={p.client} timeZone={p.timeZone} />}
      <Keys me={p.me} client={p.client} adminTeams={a.adminTeams} timeZone={p.timeZone} />
      {has("webhooks") && a.adminTeams.map((t) => (
        <Webhooks key={t.id} team={t} client={p.client} timeZone={p.timeZone} />
      ))}
      {has("connectors") && <Connectors client={p.client} hubBase={p.hubBase} timeZone={p.timeZone} />}
      {has("agent-policy") && a.adminTeams.map((t) => (
        <section key={t.id} aria-label={`${t.name}: agents`}>
          <AgentPolicy team={t} client={p.client} publish={p.publish ?? publishRoute} timeZone={p.timeZone} />
          {p.teamExtras?.(t.id)}
        </section>
      ))}
    </div>
  );
}
```

The sections' props, each complete (the bodies follow the decisions; `Keys` is given in full
below because the secret's path is the one thing here that must not be got wrong):

```ts
type C = AdminProps["client"];
export interface AppsProps { client: C; timeZone: string }
export interface KeysProps { me: Me; client: C; adminTeams: Team[]; timeZone: string }
export interface WebhooksProps { team: Team; client: C; timeZone: string }
export interface DeliveriesProps { webhookId: string; client: C; timeZone: string }   // inside Webhooks
export interface ConnectorsProps { client: C; hubBase: string; timeZone: string }
export interface AgentPolicyProps { team: Team; client: C; publish: PublishRoute; timeZone: string }
```

- **Apps** lists `apps()`; a `pending` row shows the manifest's `name`, `publisher`, `contact`,
  each `redirectUris` entry verbatim in mono, and its scopes in words (`work.read` → *read*,
  `work.write` → *read and write*); *Approve* and *Decline* send `decideApp(id, {approve, reason})`
  with the typed reason. An `approved` row's *Revoke* goes through `ConfirmInPlace` (*Release notes
  bot stops at once; its users' next call gets 401*) and sends `{approve: false, reason}`.
- **Webhooks** lists `webhooks(team.id)` with URL, kinds and state; *New webhook* posts
  `createWebhook` and hands the answer's `secret` to `SecretOnce`; *Rotate secret* and *Disable*
  confirm in place first. Each row opens **Deliveries**: `deliveries(id)` with *status · attempt n
  · code* and the time, `nextAttemptAt` read *retry at 14:07:00*, and *Redeliver* per failed row.
- **Connectors** shows the two addresses (decision 9) and the allow-list; *Allow* sends
  `allowMCPConnector({url, name})`, *Remove* confirms in place and sends `removeMCPConnector(url)`.
- **AgentPolicy** reads `agentPolicy(team.id)`; an unset policy (`updatedAt` null) reads
  *Claude Code only (the default)*. Up and down move a harness (`move`); the skills source is
  *In the sandbox image* or *A library*, with repository, path and commit; *Save* runs
  `checkPolicy`, then `putAgentPolicy`. *Publish to sandboxes* renders only when `publish` is set.

#### Interfaces — complete: `desktop/src/components/admin/Keys.tsx`

```tsx
// desktop/src/components/admin/Keys.tsx — task 173. Your own keys, and each admin team's keys.
import { useCallback, useEffect, useState, type JSX } from "react";
import type { APIKey, RateReading } from "../../api/work";
import type { Team } from "../../api/hub";
import { ConfirmInPlace } from "../common/ConfirmInPlace";
import { mayRevoke } from "./access";
import type { KeysProps } from "./AdminHome";
import { SecretOnce } from "./SecretOnce";
import { usePressOnce } from "./usePressOnce";

/** A clock time in the person's zone, 24-hour with seconds: "14:05:03". time.ts has no such
 *  formatter (it holds formatElapsed and formatWaiting only), so it lives here. */
export const clock = (iso: string, timeZone: string) =>
  new Intl.DateTimeFormat("en-AU", { timeZone, hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" }).format(new Date(iso));
const day = (iso: string, timeZone: string) =>
  new Intl.DateTimeFormat("en-AU", { timeZone, day: "numeric", month: "short", year: "numeric" }).format(new Date(iso));

/** Decision 4: two numbers and a time, the caller's own bucket, never "N of M". */
export const rateLine = (r: RateReading, tz: string) =>
  `Your own requests: used ${r.used} · limit ${r.limit} · resets ${clock(r.resetsAt, tz)}`;

function KeyList(p: KeysProps & { team: Team | null }): JSX.Element {
  const teamId = p.team?.id ?? null;
  const [keys, setKeys] = useState<APIKey[] | null>(null);
  const [rate, setRate] = useState<RateReading | null>(null);
  const [secret, setSecret] = useState<string | null>(null);   // decision 3: here and nowhere else
  const [confirm, setConfirm] = useState<string | null>(null);
  const [name, setName] = useState(""); const [expires, setExpires] = useState("");
  const load = useCallback(async () => {
    const r = await p.client.apiKeys(teamId); setKeys(r.keys); setRate(r.rate);
  }, [p.client, teamId]);
  useEffect(() => { void load(); }, [load]);   // decision 1: fetched on open
  const create = usePressOnce(async () => {
    const made = await p.client.createAPIKey(teamId, { name, expiresAt: `${expires}T00:00:00Z` });
    setSecret(made.secret);                    // split at once: the secret to SecretOnce ...
    setKeys((k) => [made.key, ...(k ?? [])]);  // ... the record to the list
    setName(""); setExpires("");
  });
  const revoke = usePressOnce(async (id: string) => { await p.client.revokeAPIKey(id); setConfirm(null); await load(); });
  const title = p.team ? `${p.team.name}: team keys` : "Your keys";
  return (
    <section aria-label={title}>
      <h2>{title}</h2>
      {secret !== null && <SecretOnce label="New key" secret={secret} onDone={() => setSecret(null)} />}
      <ul>{(keys ?? []).map((k) => (
        <li key={k.id} data-key={k.id}>
          <span>{k.name}</span> <code className="adm-mono">{k.shown}</code>
          <span>{k.lastUsedAt ? `last used ${clock(k.lastUsedAt, p.timeZone)}` : "never used"}</span>
          <span>{k.revokedAt ? `revoked ${day(k.revokedAt, p.timeZone)}` : `expires ${day(k.expiresAt, p.timeZone)}`}</span>
          {!k.revokedAt && mayRevoke(p.me, k) && (confirm === k.id
            ? <ConfirmInPlace consequence={`${k.name} stops working at once; anything using it gets 401.`}
                confirmLabel={`Revoke ${k.name}`} onConfirm={() => void revoke.press(k.id)} onCancel={() => setConfirm(null)} />
            : <button type="button" disabled={revoke.busy} onClick={() => setConfirm(k.id)}>Revoke</button>)}
        </li>))}
      </ul>
      <form onSubmit={(e) => { e.preventDefault(); void create.press(); }}>
        <label>Name <input value={name} onChange={(e) => setName(e.target.value)} required /></label>
        <label>Expires <input type="date" value={expires} onChange={(e) => setExpires(e.target.value)} required /></label>
        <button type="submit" disabled={create.busy}>Create key</button>
      </form>
      {p.team === null && rate && <p className="adm-rate">{rateLine(rate, p.timeZone)}</p>}
      <button type="button" onClick={() => void load()}>Refresh</button>
    </section>
  );
}

export function Keys(p: KeysProps): JSX.Element {
  return <>{[null, ...p.adminTeams].map((t) => <KeyList key={t?.id ?? "me"} {...p} team={t} />)}</>;
}
```

The rate line sits under **Your keys** only: the reading is the caller's own bucket on whichever
list call answered (decision 4), so repeating it under a team list would read as the team's.
`expiresAt` is the picked date at `T00:00:00Z`, task 160's example shape (`task-160.md:377`);
task 160 refuses more than 365 days on and names the field in its 400 (`:380`), which the form
shows under *Expires*.

#### Tests to write first — the fixtures, `testAdmin.ts`

```ts
// desktop/src/components/admin/testAdmin.ts — task 173. A fake client whose every member is a
// vi.fn over fixed answers, and three people: Dylan (org admin, admin of Payments), Priya (a
// member of Payments), Sam (admin of Payments, not an org admin).
import { vi } from "vitest";
import type { APIKey, CreatedAPIKey } from "../../api/work";
import type { AdminProps } from "./AdminHome";
import type { Me } from "./access";

const u = (id: string, displayName: string) => ({ id, displayName, email: `${id}@kpmg.com.au` });
const org = { id: "org-1", displayName: "KPMG Australia", tenantId: "t-1" };
const pay = (role: "admin" | "member") =>
  ({ id: "tm-pay", name: "Payments", role, createdAt: "2026-09-01T00:00:00Z", createdBy: u("u-d", "Dylan") });
export const dylan: Me = { user: u("u-d", "Dylan"), org, orgAdmin: true, teams: [pay("admin")], invitesBound: 0 };
export const priya: Me = { user: u("u-p", "Priya"), org, orgAdmin: false, teams: [pay("member")], invitesBound: 0 };
export const sam: Me = { user: u("u-s", "Sam"), org, orgAdmin: false, teams: [pay("admin")], invitesBound: 0 };

export const key = (over: Partial<APIKey> = {}): APIKey => ({
  id: "ak-1", scope: "personal", ownerId: "u-d", teamId: null, name: "CI release", shown: "pak_3f9c",
  createdAt: "2026-09-26T03:00:00Z", expiresAt: "2027-03-25T00:00:00Z",
  lastUsedAt: "2026-09-26T04:02:00Z", revokedAt: null, revokedBy: null, ...over });
export const SECRET = "pak_3f9c2a1b7e4d4c0a9b8e7f6a5d4c3b2a";
export const created: CreatedAPIKey = { key: key({ id: "ak-2" }), secret: SECRET };

export function fakeClient(over: Partial<AdminProps["client"]> = {}): AdminProps["client"] {
  return {
    apps: vi.fn(async () => []), decideApp: vi.fn(),
    apiKeys: vi.fn(async () => ({ keys: [key()], rate: { used: 3, limit: 20, resetsAt: "2026-09-26T04:05:03Z" } })),
    createAPIKey: vi.fn(async () => created), revokeAPIKey: vi.fn(async () => key({ revokedAt: "2026-09-26T04:10:00Z" })),
    createWebhook: vi.fn(), webhooks: vi.fn(async () => []), disableWebhook: vi.fn(),
    rotateWebhookSecret: vi.fn(), deliveries: vi.fn(async () => ({ items: [], next: null })), redeliver: vi.fn(),
    mcpConnectors: vi.fn(async () => []), allowMCPConnector: vi.fn(), removeMCPConnector: vi.fn(),
    agentPolicy: vi.fn(async () => ({ teamId: "tm-pay", harnesses: ["claude-code"], skills: { source: "image", library: null }, updatedBy: null, updatedAt: null })),
    putAgentPolicy: vi.fn(async (_t, b) => ({ teamId: "tm-pay", ...b, updatedBy: null, updatedAt: "2026-09-26T04:00:00Z" })),
    ...over,
  } as AdminProps["client"];
}
export const base = { hubBase: "https://hub.example.test", timeZone: "Australia/Sydney" };
```

`Page` is `{ items, next }` as task 156's list answers (`task-156.md:538`); the fixtures' times are
UTC and read in Sydney (UTC+10 on 2026-09-26), so `04:05:03Z` renders *14:05:03*.

#### Tests to write first — `Keys.test.tsx` and `AdminHome.test.tsx`

```tsx
// desktop/src/components/admin/Keys.test.tsx
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { useWorkStore } from "../../state/workSlice";   // task 37's store (UNVERIFIED export name)
import { AdminHome } from "./AdminHome";
import { base, dylan, fakeClient, priya, SECRET } from "./testAdmin";

async function createKey(client = fakeClient()) {
  const user = userEvent.setup();
  const view = render(<AdminHome me={dylan} client={client} {...base} />);
  const mine = await screen.findByRole("region", { name: "Your keys" });
  await user.type(within(mine).getByLabelText("Name"), "CI release");
  await user.type(within(mine).getByLabelText("Expires"), "2027-03-25");
  return { user, view, mine, client };
}

describe("Keys", () => {
  it("shows the secret once, then it is gone from the DOM, the store and storage", async () => {
    const { user, view, mine } = await createKey();
    await user.click(within(mine).getByRole("button", { name: "Create key" }));
    expect(await screen.findByText(SECRET)).toBeVisible();
    expect(screen.getByText("Shown once. Copy it now: it is not shown again.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(view.container.innerHTML).not.toContain(SECRET);
    expect(JSON.stringify(useWorkStore.getState())).not.toContain(SECRET);
    expect(JSON.stringify({ ...localStorage })).not.toContain(SECRET);
  });

  it("sends one create for a double press", async () => {
    const { user, mine, client } = await createKey();
    const btn = within(mine).getByRole("button", { name: "Create key" });
    await Promise.all([user.click(btn), user.click(btn)]);
    await waitFor(() => expect(client.createAPIKey).toHaveBeenCalledTimes(1));
  });

  it("reads the rate as facts: two numbers and a time, never N of M", async () => {
    const { view } = await createKey();
    expect(await screen.findByText("Your own requests: used 3 · limit 20 · resets 14:05:03")).toBeVisible();
    expect(view.container.textContent).not.toMatch(/\d+\s+of\s+\d+|%/);
    expect(view.container.querySelector("meter, progress, [role=progressbar]")).toBeNull();
    expectNoPrediction(view.container);
  });

  it("revokes only after the in-place confirm", async () => {
    const { user, mine, client } = await createKey();
    await user.click(await within(mine).findByRole("button", { name: "Revoke" }));
    expect(client.revokeAPIKey).not.toHaveBeenCalled();
    expect(screen.getByText("CI release stops working at once; anything using it gets 401.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Revoke CI release" }));
    await waitFor(() => expect(client.revokeAPIKey).toHaveBeenCalledWith("ak-1"));
  });

  it("holds the secret in SecretOnce's props only", () => {
    const src = readFileSync(resolve(__dirname, "SecretOnce.tsx"), "utf8");
    expect(src).not.toMatch(/workSlice|localStorage|sessionStorage|console\.|useEffect/);
  });
});
```

```tsx
// desktop/src/components/admin/AdminHome.test.tsx (the access rows)
it("a member sees her own keys and no other section", async () => {
  render(<AdminHome me={priya} client={fakeClient()} {...base} />);
  expect(await screen.findByRole("region", { name: "Your keys" })).toBeVisible();
  for (const name of ["Apps", "Connectors", "Payments: webhooks", "Payments: agents", "Payments: team keys"])
    expect(screen.queryByRole("region", { name })).toBeNull();
});
it("an org admin who is no team's admin sees Apps and Connectors, never a team's policy", async () => {
  render(<AdminHome me={{ ...dylan, teams: [{ ...dylan.teams[0], role: "member" }] }} client={fakeClient()} {...base} />);
  expect(await screen.findByRole("region", { name: "Apps" })).toBeVisible();
  expect(screen.queryByRole("region", { name: "Payments: agents" })).toBeNull();
});
it("renders no Publish button while publishRoute is null", async () => {
  const { container } = render(<AdminHome me={dylan} client={fakeClient()} {...base} />);
  await screen.findByRole("region", { name: "Payments: agents" });
  expect(screen.queryByRole("button", { name: /publish/i })).toBeNull();
  expectNoPrediction(container);
});
```
