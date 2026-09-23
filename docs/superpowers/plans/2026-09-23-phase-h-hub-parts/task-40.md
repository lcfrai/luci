### Task 40: the desktop's auth, share, credential and feed client; `authSlice`; `hubFeed` (seam S-FEED, desktop half)

**Lane D. Depends on: 36, 38, 39 (and 35 for the share routes). About 3 hours. Contract task C4.**

Everything the two UI tasks (41, 42) and phases 3, 4, 5 and 7's desktop work need from the control
plane's new routes, as typed client functions — and the desktop half of the change feed: the
poller reads two response headers off the sessions poll it already makes (task 36), and **only
when one changes** does the desktop make one more request — `GET /v1/hub/changes` for the cursor,
`GET /v1/auth/status` for the auth state. Nothing here is persisted (ADR-H1 §5.4: nothing
Hub-derived on the user's machine).

**Files:**
- `desktop/src/store/types.ts` — the new wire types (modify ⚑: appended)
- `desktop/src/api/client.ts` + `client.test.ts` — the functions; the header capture (modify ⚑)
- `desktop/src/api/hubFeed.ts` + `hubFeed.test.ts` (new)
- `desktop/src/api/poller.ts` + `poller.test.ts` — the header hook (modify ⚑)
- `desktop/src/store/slices/authSlice.ts` (new); `desktop/src/store/store.ts` + `store.test.ts` — compose it, never persist it (modify ⚑)
- `desktop/src/App.tsx` — pass the hook to `startPolling` (modify ⚑: one line)

#### Contract — complete

```ts
// desktop/src/store/types.ts — appended. Field for field the control plane's JSON (tasks 32, 35,
// 36, 39) and the Hub's (contract §7.1, §7.3, §7.11), which the control plane relays unchanged.

export type AuthState = "signed_out" | "signing_in" | "signed_in";

export interface AuthStatus {
  state: AuthState;
  /** "" | "timeout" | "entra_error" | "exchange_failed" | "reauth_required" | "tenant_not_enrolled" */
  reason: string;
  account: { tenantId: string; name: string; username: string } | null;
  hubConfigured: boolean;
}

export interface SignInStart { authorizeUrl: string; expiresAt: string }

export interface HubUser { id: string; displayName: string; email: string }
export type HubRole = "viewer" | "member" | "admin";
export interface HubTeam { id: string; name: string; role: HubRole; createdAt: string; createdBy: HubUser }
export interface HubMember { user: HubUser; role: HubRole; addedAt: string; addedBy: HubUser | null }
export interface HubTeamDetail { team: HubTeam; members: HubMember[] }
export interface HubInvite {
  id: string; teamId: string; email: string; role: HubRole;
  state: "pending" | "bound" | "revoked"; createdAt: string; createdBy: HubUser; boundUser: HubUser | null;
}
export interface HubRemote { repoKey: string; host: "github" | "azure-devops"; addedAt: string }
export interface HubProject { id: string; teamId: string; name: string; remotes: HubRemote[]; createdAt: string; createdBy: HubUser }

export type HubLinkState = "linked" | "unlinked" | "no_remote";
export interface HubLink {
  state: HubLinkState;
  project: HubProject | null;
  repoKeys: { repoKey: string; host: "github" | "azure-devops"; remote: string }[];
}

export type CredentialKind = "github" | "panacea";
/** Metadata only. There is no token field, by design: no route returns one (Hub plan task 39). */
export interface CredentialEntry { kind: CredentialKind | "entra"; scope: string; subject: string; savedAt: string }

export interface HubEvent {
  seq: number; at: string; kind: string;
  teamId: string | null; projectId: string | null; subject: string;
  actor: HubUser | null; data: Record<string, string | number | boolean>;
}
export interface HubChangesPage { events: HubEvent[]; next: string; more: boolean }
```

```ts
// desktop/src/api/client.ts — CockpitClient gains, and createClient implements:

  /** The headers the control plane sets on every response (Hub plan task 36), as last seen on
   *  ANY response of this client. `cursors` holds one entry per `Panacea-<Name>-Cursor` header,
   *  keyed by the LOWERCASED <Name> ("hub", "deploy": `Headers` iteration lowercases names) —
   *  the generic mechanism phase 7 asked for (HA7-4, task 00). A name absent from a response
   *  keeps its last value; authState is null until a response carried it. */
  lastHubHeaders(): HubHeaders;

  authStatus(): Promise<AuthStatus>;                                   // GET  /v1/auth/status
  signIn(): Promise<SignInStart>;                                      // POST /v1/auth/sign-in
  cancelSignIn(): Promise<void>;                                       // POST /v1/auth/cancel      (204)
  signOut(): Promise<void>;                                            // POST /v1/auth/sign-out    (204)

  hubLink(projectId: ProjectId): Promise<HubLink>;                     // GET  /v1/projects/{id}/hub-link
  shareProject(projectId: ProjectId, teamId: string, name?: string): Promise<HubLink>; // POST same
  hubTeams(): Promise<HubTeam[]>;                                      // GET  /v1/hub/teams
  createHubTeam(name: string): Promise<HubTeam>;                       // POST /v1/hub/teams
  hubTeam(teamId: string): Promise<HubTeamDetail>;                     // GET  /v1/hub/teams/{id}
  hubInvites(teamId: string): Promise<HubInvite[]>;                    // GET  /v1/hub/teams/{id}/invites
  inviteToTeam(teamId: string, email: string, role: HubRole): Promise<HubInvite>; // POST same
  revokeInvite(teamId: string, inviteId: string): Promise<void>;       // DELETE …/invites/{inviteId}

  listCredentials(): Promise<CredentialEntry[]>;                       // GET    /v1/credentials
  storeCredential(kind: CredentialKind, scope: string, token: string, subject?: string): Promise<void>; // POST /v1/credentials/{kind}
  deleteCredential(kind: CredentialKind, scope: string): Promise<void>; // DELETE /v1/credentials/{kind}?scope=

  hubChanges(after: string): Promise<HubChangesPage>;                  // GET  /v1/hub/changes?after=
```

`storeCredential` holds the token only for the duration of the request: it is never assigned to a
module variable, the store or an error. `send` captures the headers from every response
(`res.headers.forEach`, matching `^panacea-([a-z]{1,32})-cursor$` and `panacea-auth-state`, read
while the timer is armed, as the body is).

```ts
// desktop/src/api/hubFeed.ts (exported type)
export type HubHeaders = { cursors: Readonly<Record<string, string>>; authState: AuthState | null };
```

```ts
// desktop/src/api/hubFeed.ts

/** The desktop half of seam S-FEED. One instance per window (constructed beside `api`). */
export interface HubFeed {
  /** Handlers for events whose kind starts with kindPrefix: "brief.", "design.", "release.",
   *  "project.", "team.", or "" for all. Phase 4's provenanceSlice, phase 5's approvals and phase
   *  7's releases subscribe here (contract §9.3). Returns the unsubscribe. */
  subscribe(kindPrefix: string, fn: (e: HubEvent) => void): () => void;
  /** The poller calls this after every good tick with lastHubHeaders(); it reads
   *  h.cursors.hub and h.authState, and ignores every other cursor. */
  noteHeaders(h: HubHeaders): void;
  /** Any caller that got ApiError "signed_out" calls this; it re-reads the auth status once. */
  noteSignedOut(): void;
}

export function createHubFeed(client: CockpitClient, store: typeof useCockpit): HubFeed;
```

**`noteHeaders`' rules:** the **first** `cursors.hub` seen is remembered and nothing is fetched — every
cache starts empty, so there is nothing to invalidate. A **changed** cursor → one
`hubChanges(after = last)`, each event delivered to matching subscribers in `seq` order, `last` =
`next`; while `more`, fetch again (at most 5 pages per change). **One read in flight at a time**: a
change seen during a read is remembered and read after it. A failed read keeps `last` and is retried
at the next change. A **changed** auth state → one `authStatus()` → `store.setAuth(…)`.

```ts
// desktop/src/store/slices/authSlice.ts

export interface AuthSlice {
  /** null until the first status read. NEVER persisted: store.ts's `partialize` names what
   *  survives a restart, and this is not in it (Hub plan task 40; ADR-H1 §5.4). */
  auth: AuthStatus | null;
  /** Set while a sign-in is pending, for the "finish in your browser" state (task 41). */
  signInExpiresAt: string | null;
  setAuth(status: AuthStatus): void;
  setSignInExpiresAt(at: string | null): void;
}
```

```ts
// desktop/src/api/poller.ts — the signature gains one optional parameter, after onGoodTick:
export function startPolling(
  client: CockpitClient,
  store: typeof useCockpit,
  onGoodTick?: () => void,
  onHubHeaders?: (h: HubHeaders) => void,
): () => void;
// onHubHeaders runs after a good tick only, with client.lastHubHeaders(), outside the try (as
// onGoodTick does, poller.ts:74-76), so a throw from it is never the poller's failure. One
// parameter serves every consumer: App.tsx passes one function that calls hubFeed.noteHeaders
// and, when phase 7 lands, its deploy feed's — each reads its own cursors entry (HA7-4, task 00).
// DECLARED DIFFERENCE (task 00, 44a §9.3): phase 7 asked for onCursor(name, value) per changed
// header. That is NOT built. There is one hook, keys are the LOWERCASED name (cursors.hub,
// cursors.deploy; cursors.Deploy is always undefined), and each consumer compares its own entry
// with the last value it saw. Phase 7 adds no second hook here.
```

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `client: each new function calls its route` | one row per function: method, path, query, JSON body, and the 204 functions resolve `undefined` | a path typo the compiler cannot see |
| `client: storeCredential never keeps the token` | after a call that succeeds and one that fails, neither `ApiError`'s `message` nor `details` holds the token | a token in the rail footer's error line |
| `client: headers are captured from any response` | a `listSessions` whose response carries `Panacea-Hub-Cursor`, `Panacea-Deploy-Cursor` and `Panacea-Auth-State` → `lastHubHeaders()` returns `cursors: {hub, deploy}` and the state; a response without them leaves the last values, per name | a feed that never fires, or phase 7's cursor dropped (HA7-4) |
| `hubFeed: the first cursor fetches nothing` | `noteHeaders({cursors:{hub:"5"}, authState:null})` → no `hubChanges` call; a changed `deploy` alone → none either | a start-up replay of history, or the Hub feed read on another source's tick |
| `hubFeed: a change reads once and delivers in order` | cursor 5 → 8 → one `hubChanges("5")`; a `brief.` subscriber gets the `brief.*` events in seq order; a `release.` subscriber none | an invalidation missed or misrouted |
| `hubFeed: one read in flight` | 8 → 9 → 10 while the first read is pending → two reads in total, the second from the first's `next` | parallel reads delivering events twice or out of order |
| `hubFeed: more is followed, bounded` | `more: true` three times → four reads; `more` six times → five, the rest at the next change | an unbounded loop in the render thread |
| `hubFeed: a failed read is retried` | the read rejects → the next change reads from the same `last` | events lost to one network blip |
| `hubFeed: an auth change reads status once` | `authState` `signing_in` → `signed_in` → one `authStatus()`; `store.auth` updated | a sign-in that the window never notices finished |
| `hubFeed: noteSignedOut re-reads status` | → one `authStatus()` | a stale "signed in" after the refresh token died |
| `poller: the header hook runs after good ticks only` | a good tick → hook called with the headers; a failed tick → not | a hook fed stale headers after an outage |
| `store: auth is never persisted` | after `setAuth`, `partialize(state)` has no `auth` key and `localStorage[PERSIST_KEY]` has no `"auth"` | Hub-derived data on disk (A5 widened) |
| `types: no token field` | a source-text test: `types.ts` declares no property named `token` in the appended block | a type that invites putting a secret in state |

#### Steps

- [ ] **Step 1:** Re-read contract §9.3; tasks 32, 35, 36, 39's route tables; `client.ts`,
  `poller.ts`, `store.ts` as they stand.
- [ ] **Step 2:** Tests; they fail. Then the types, the functions, `hubFeed.ts`, `authSlice.ts`,
  the poller parameter, the store composition, and the one `App.tsx` line
  (`startPolling(api, useCockpit, catalogue.retry, hubFeed.noteHeaders)`).
- [ ] **Step 3:** `cd desktop && npm test && npm run typecheck`. Commit by pathspec.

#### Acceptance

```bash
cd desktop && npm test 2>&1 | tail -3      # all passed
cd desktop && npm run typecheck             # no output
git grep -n "token" -- desktop/src/store/slices/authSlice.ts   # no output
```

#### What this task does not claim

- Any pixel: tasks 41 and 42.
- Phase 4's `provenanceSlice` invalidation: phase 4 subscribes to `hubFeed` (seam S-FEED) and
  owns the UI contract §4.7 amendment (contract §9.3).
