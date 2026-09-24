### Task 18: Desktop — the governance panel

**Half:** 7a. **Hours:** 2.5. **Depends on:** 15, 11. **Uses:** task 16's `useHeldAct` contract —
see *The hold* below for which task lands the file.

Four controls for the selected branch, **drawn as the API really offers them** (brief `B1`,
corrected in plan §3.2): **deploy on push** and the **web address** are owner-only per-branch
switches; **who can reach it** is one of five modes per branch; **project members** are people with
a role, for the whole project. The panel decides no permission of its own. `view.ownerControls`
(task 07, plan 7P-9) draws owner controls disabled **before** a press, with the reason; Panacea
enforces every guard, and its refusal is shown verbatim.

**Read `docs/DESIGN-SYSTEM.md` first.** No spinner: a read in flight shows the control's label over
a static, fixed-height placeholder. A switch moves by `transform` only. `tabular-nums` on counts.
Every state is designed: loading, owner-only, refused, failed, stale. **Visual acceptance is the
owner's, batched (OD-12); unverified until then.**

**Files:**
- `desktop/src/deploy/GovernancePanel.tsx` (new) — the panel, collapsed by default, below the release table
- `desktop/src/deploy/PostureControl.tsx` (new) — five modes, the password field, the once-shown token
- `desktop/src/deploy/AccessList.tsx` (new) — the members; grant and revoke
- `desktop/src/deploy/useGovernance.ts` (new) — the three reads and six writes over `DeployApi`
- `desktop/src/deploy/useHeldAct.ts` (new **only if task 16 has not landed it**) — task 16's contract, unchanged
- `desktop/src/deploy/DeployLens.tsx` (modified, task 14's) — mounts `GovernancePanel` for the selected branch
- `desktop/src/deploy/deploy.css` (modified) — the panel
- `*.test.tsx` / `useGovernance.test.ts` beside each

#### Contracts — complete

```ts
// useGovernance.ts — the panel's only data source. It reads on OPEN, again after each write, and
// on Refresh — never on a timer and never on the cursor: none of the three is in the watcher's view.
import type { ApiError } from "../api/client";
import type { AccessGrant, AccessRole, AutoDeploy, Posture, PostureMode, PostureResult, UrlPrefix } from "./types";

export type Loaded<T> =
  | { state: "loading" }
  | { state: "ok"; value: T }
  | { state: "owner_only" }                  // 403 deploy_forbidden from an owner route
  | { state: "refused"; message: string }    // 409 deploy_conflict: Panacea's sentence, verbatim
  | { state: "failed"; error: ApiError };    // anything else; the message is shown with Try again

export interface Governance {
  autoDeploy: Loaded<AutoDeploy>;
  posture: Loaded<Posture>;
  access: Loaded<AccessGrant[]>;
  refresh(): void;
  setAutoDeploy(enabled: boolean): Promise<void>;
  setPosture(change: { mode: PostureMode; password?: string; rotateBypassToken?: boolean }): Promise<PostureResult>;
  grant(grantee: string, role: Exclude<AccessRole, "unknown">): Promise<void>;
  revoke(grantee: string): Promise<void>;
  setUrlPrefix(prefix: string): Promise<UrlPrefix>;
  clearUrlPrefix(): Promise<void>;
}

/** open false → no request at all. A branch change while open reads again for the new branch;
 *  an answer for the old branch that arrives late is dropped (compared by branch, not by time). */
export function useGovernance(projectId: string, branch: string, open: boolean): Governance;
```

```tsx
export function GovernancePanel(props: {
  projectId: string;
  branch: { id: string; name: string };
  env: EnvironmentView | null;          // the selected branch's environment, task 15's rule
  ownerControls: "yes" | "no" | "unknown";
  stale: boolean;
}): JSX.Element;

export function PostureControl(props: {
  posture: Loaded<Posture>;
  stale: boolean;
  onChange(change: { mode: PostureMode; password?: string; rotateBypassToken?: boolean }): Promise<PostureResult>;
}): JSX.Element;

export function AccessList(props: {
  access: Loaded<AccessGrant[]>;
  ownerControls: "yes" | "no" | "unknown";
  stale: boolean;
  onGrant(grantee: string, role: Exclude<AccessRole, "unknown">): Promise<void>;
  onRevoke(grantee: string): Promise<void>;
}): JSX.Element;
```

**The web address has no read of its own.** Panacea's routes for it are `PUT` and `DELETE` only
(`routes/routes.go:1313-1314`), so the current address is `env.host` from the view; after a claim,
the `PUT`'s `host` is shown until the next cursor brings the view up to date.

#### The hold — which acts wait five seconds, and why those

`useHeldAct` is task 16's (its contract, its four hold tests). Task 18 is 7a and usually lands
first: if `desktop/src/deploy/useHeldAct.ts` does not exist, this task creates it **exactly** as task
16 specifies, with those four tests, and task 16 finds it present. The `send` it wraps ignores the
Idempotency-Key — governance routes take none (task 11).

| Act | Held? | Because |
|---|---|---|
| posture → `none` or `password` | **held**, consequence printed first | the two modes Panacea itself treats as relaxing sign-in (`ssoEnforcedForMode`, `handlers/access_mode.go:84-91`) — the same pair its SSO lock refuses (`project_security.go:726-736`) |
| posture → `kpmg-sso`, `panacea-sso`, `bypass-token` | sent at once | each keeps the SSO gate (`access_mode.go:86`) |
| revoke a member | **held** | it takes a person's access to every branch at once |
| release the web address | **held** | once released, any branch in Panacea may claim it (`branch_url_prefix.go:22-24`) |
| deploy on push, grant, claim an address | sent at once | each is undone by its own opposite, and none opens the app to anyone new |

#### The four controls — what each says

**Deploy on push** (owner route, `routes.go:1308-1309`)

| State | Drawn |
|---|---|
| `ok` | a switch, *"Deploy `<branch>` automatically on every push"*; under it *"On."* / *"Off."* and, always, *"Every release a push makes will say “no brief is recorded for this release” — a push is not a Ship, so nothing is pinned."* `explicit null` adds *"Follows the project default (`on` / `off`)."*; `productionBranch` adds *"This is the production branch."* |
| `owner_only`, or `ownerControls "no"` | the switch disabled; *"Only a project owner can change this, with a key that is not restricted to one project. Panacea refuses it to developers and to every restricted key."* Both halves, always: the owner probe (7P-9) gets the same `403` for a developer and for a real owner holding a restricted key — every scoped key is capped at Developer (`models/models.go:547-559`) and `IsOwner` is false for one (`:247-249`) — and the watcher records only `"no"`: telling the two apart would mean matching Panacea's prose (task 03's `error-scope-owner-403.txt`; review I5) |
| `refused` | the switch disabled; Panacea's sentence verbatim (*"branch has no environment yet — deploy it once, or set the project default"*, `projects.go:4132-4134`) |

**Who can reach it** (`PostureControl`; developer route, `routes.go:1431`) — one radio per mode,
words from Panacea's own definitions (`handlers/access_mode.go:11-24`):

| Mode | Words | Consequence, before the press (held modes only) |
|---|---|---|
| `kpmg-sso` | *"Any signed-in KPMG identity. Panacea's default, and what an unknown mode falls back to."* | — |
| `panacea-sso` | *"Signed in, and on this project's own app-user allowlist."* | — |
| `bypass-token` | *"Signed in — or holding a bypass token, which lets automation through."* | — |
| `password` | *"One shared password instead of sign-in."* + a password field, write-only | *"Anyone holding the password will be able to open this branch, signed in or not."* |
| `none` | *"No gate at all."* | *"Anyone who can reach the address will be able to open this branch: Panacea removes every gate."* (plan Q-P7-4) |

- `modeKnown false` → `modeRaw` in mono and *"Panacea reports a mode this cockpit does not know.
  It treats it as `kpmg-sso`."* (`access_mode.go:19-20`).
- `passwordSet` → *"A password is set."*; `bypassTokenCount` → *"`N` bypass tokens are active."*,
  with **Rotate** (sends `rotateBypassToken: true`).
- A result with `bypassTokens` → each token once, in mono, with **Copy**, and *"Shown once. The
  cockpit does not keep it."* It lives in the component's state only, and is dropped on unmount.
- `enforced false` → the result's `message` verbatim, as a warning: the mode is recorded but the
  route has not caught up. That `message` is the connector's own sentence, never Panacea's, whose
  not-enforced text embeds an internal error (task 04, review M7).
- `409` → Panacea's SSO-lock sentence verbatim (*"access mode cannot be relaxed: …"*,
  `project_security.go:733-735`), and the radio returns to the mode Panacea still holds.

**Project members** (`AccessList`; list is membership, changes are owner routes,
`routes.go:1407-1409`) — *"For the whole project, every branch — Panacea has no per-branch access
list."* One row per grant: `grantee`, or *"a group binding"* and `group` in mono when `grantee` is
empty (legacy bindings, plan §3.2 B1); the role word, and `roleRaw` in mono when `role "unknown"`.
**Add a person**: one name or email and a role, with the line *"Gives `<grantee>` `<role>` on the
whole project."* before the press. **Remove** is held: *"Removes `<grantee>` from the whole project,
every branch."* A `422` shows *"No Panacea user matches that name or email."* (task 11).

**Web address** (owner routes, `routes.go:1313-1314`) — the current `env.host`, or *"No address
yet — the branch has not been deployed."*; a field and **Claim**; the hint *"Lowercase letters,
digits and hyphens; 1–63 characters; no hyphen at either end. Platform names such as `api` are
refused."* — words from `handlers/branch_url_prefix.go:32-64`, **no client-side check**: Panacea
decides, and its `409` (*"url prefix already taken: …"*) or `422` (the rule's sentence) is shown
verbatim. **Release** is held.

**Every control, when `stale`:** disabled, with *"Off while Panacea is not answering."*

#### Tests to write first (`vitest` + Testing Library; a fake `DeployApi`)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `closed panel reads nothing` | mounted collapsed → zero API calls; opened → one `autoDeploy`, one `posture`, one `access` | three reads per lens render, whether or not anyone looks |
| `no timer, no cursor` | open, then 60 s of fake time and three cursor changes → still one read each; **Refresh** → two | a governance poll — a second clock |
| `late answer for an old branch dropped` | switch branch while `posture` is pending; resolve the old one → the new branch's posture is shown | branch A's posture drawn under branch B's name |
| `owner-only before the press` | `ownerControls "no"` → the auto-deploy switch, **Add a person**, **Remove** and **Claim** disabled, each with the owner line, exactly, naming both the role and the restricted key; posture still enabled | a live 403 discovered instead of designed (brief R7), or posture wrongly owner-gated, or an owner line that tells a real owner holding a restricted key they are not one (review I5) |
| `403 is owner-only, not an error` | `autoDeploy` rejects `deploy_forbidden` → the owner line; no error banner | a developer shown a red failure for a control they were never offered |
| `none is held, with its consequence first` | idle radio on `none` shows the consequence sentence before any press; choose it → nothing sent for 4.9 s; Undo → nothing ever sent | a public branch one misclick away (Q-P7-4) |
| `kpmg-sso is not held` | choose `kpmg-sso` from `password` → one `setPosture` at once | a delay on tightening, which has no downside |
| `lock refusal verbatim` | `setPosture` rejects `409` with the lock sentence → that sentence, and the radio back on the old mode | a UI that shows the new mode Panacea refused |
| `token shown once` | a result with `bypassTokens ["t1"]` → `t1` visible; unmount and remount → absent; the slice never holds it | a secret kept in the store, or persisted |
| `password never kept` | after `setPosture` resolves or rejects, the password field is empty and the fake's logged request is the only place the value appeared | a password left in React state or the store |
| `unknown mode kept raw` | `modeKnown false, modeRaw "sso-v2"` → `sso-v2` in mono and the fallback sentence; no radio checked | an unknown mode drawn as `kpmg-sso` |
| `a group binding is not a person` | a grant with `grantee ""` → *"a group binding"* and its group | an empty row, or a group rendered as someone's name |
| `revoke is held` | **Remove** → nothing sent for 4.9 s; after 5 s one `revoke` with that grantee | an instant removal from every branch |
| `no client-side prefix rule` | `GovernancePanel.tsx` contains no `RegExp` / `.test(` / `.match(` on the prefix; `UPPER` is sent and Panacea's 422 sentence shown | a second copy of `validateURLPrefix` that drifts from Panacea's |

#### Steps

- [ ] Check first: `test -f desktop/src/deploy/useHeldAct.ts`. Absent → create it from task 16's
  contract with its four hold tests, and say so in the commit message.
- [ ] Tests first; `useGovernance`; `PostureControl`; `AccessList`; `GovernancePanel`; the
  `DeployLens` mount; the CSS.
- [ ] `npx tsc --noEmit`, `npx vitest run`; commit by pathspec with *"Visual acceptance pending the
  owner (OD-12)."*

#### Acceptance

```
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass
```
**Owner's eyes (batched):** open the panel as an owner and as a developer on the same project;
choose `none` and Undo; claim a taken address and read Panacea's sentence. **Unverified until
then.**

#### What this task does NOT claim

- That `none` exposes the branch to the whole internet. It removes Panacea's gate
  (`access_mode.go:24`); an edge allowlist in front of Panacea, if one is deployed, was not checked
  (plan §12).
- That `developer` role grants work as `editor` does — task 04 sends `editor` (plan §12).
- That the owner line is right for a person whose ownership changed since the last cursor.
  `ownerControls` is the watcher's last answer; Panacea's refusal is the truth, and is shown.
