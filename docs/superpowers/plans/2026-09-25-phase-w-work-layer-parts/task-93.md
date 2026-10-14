### Task 93: https links and the prefill deep link

**Slice:** W2. **Size:** 2 focused hours (an estimate, not a measurement). **Stream:** WD2.
**dependsOn:** 80 (the Hub's read-only page, the link target) and 83 (the W2 wire types), after a
sync-merge of `feat/work-layer` (rule H1). **Seams:** consumes **SW.1**'s `issue:` key, its
`DisplayAlias` and the `issue-revision` canonical form, and **SW.1**'s bounded lengths for the
prefill; adds **no** seam. Task 91's ad hoc list gains one caller.

Every link a person copies out of this app is **`https://`**, to the Hub's read-only page. If the
desktop is installed the page hands off to it; if it is not — a phone, a reviewer outside the
pilot, a Teams channel read on the web — the page is the answer on its own. `panacea://` stays
**internal**: it is how the page and the OS hand a route to a running window, never what a person
sees or shares (05 §5.4 W2, P I10). That is the whole of this task's rule, and both of its halves
are testable: what `Copy link` produces, and what the scheme handler accepts.

The handler is deliberately narrow. A URL may **navigate and prefill**. A URL may not **act**. A
link that starts an agent, sends a comment, moves a state or creates an issue is a link somebody
can put in an email, so `panacea://issue/new?title=…` opens the composer filled and focused and
**never submits**; every parameter outside a short allow-list is ignored in silence, not
best-effort-honoured.

#### Anchors, re-read on 2026-09-27

In this tree, verified: `desktop/src-tauri/Cargo.toml` lists `tauri-plugin-opener = "2"` at
`:22` and **no other** `tauri-plugin-*` (`grep`, 2026-09-27); `desktop/package.json` lists
`@tauri-apps/plugin-opener` at `:17` and no deep-link package; `desktop/src-tauri/tauri.conf.json`
carries `"productName": "desktop"` (`:3`) and `"identifier": "com.au-dev-dmackie1.desktop"`
(`:5`) — both the scaffold's defaults — and **no `plugins` block and no `schemes` key**;
`desktop/src-tauri/capabilities/default.json` is the only capability file; `grep -rn "panacea://"
desktop/src desktop/src-tauri` finds nothing. `desktop/src/lib/` holds `ids.ts`, `immutable.ts`,
`pendingSessions.ts`, `time.ts` and `wire.ts`; there is no `links.ts`.

In this branch's parts (**UNVERIFIED until merge**): task 80's page `GET /p/issues/{issueId}[@{rev}]`,
outside `/v1`, server-rendered, no script, with an *Open in Panacea* button pointing at
`panacea://issue/…` (`tasks-61-80.md:673-681`); task 91's ad hoc list; task 83's types; task 46's
issue head; SW.1's lengths (skeleton `:375`).

**The licences of the two new dependencies are UNVERIFIED.** `tauri-plugin-deep-link` and
`@tauri-apps/plugin-deep-link` are expected MIT OR Apache-2.0, like the rest of the Tauri plugin
workspace, but the corporate network blocks the registry fetch from this session, so nobody has
read the crate's own `license` field here. Step 1 reads it and records the string; if it is not
MIT/Apache/BSD/ISC the task stops and the fallback below ships instead.

Design: `05-design.md` §5.4 W2 and §1 L-30, L-98, L-111.

#### Decisions the scope left open, made here

**(a) Parsing is a total function that returns a refusal, never throws.** `parsePanaceaUrl`
answers `Route | Refused`, and `Refused` carries a `reason` the window logs and, for a URL that
arrived from the OS, shows as one line: *That link asked for something a link may not do.* A
thrown parse in a deep-link handler is a crash on a hostile input, which is exactly the input a
URL scheme invites.

**(b) The allow-list is four keys, and it is a list, not a filter.** `title`, `description`,
`project` and `labels` are read; everything else is **dropped without comment**. Not warned about,
not echoed back — a URL that carries `&start=1` behaves exactly as one that does not, so probing
the handler tells an attacker nothing. Each value is bounded by SW.1's lengths before it reaches
React: over-long input is **truncated**, not refused, so a long paste still opens a usable
composer.

**(c) An alias is resolved by the Hub, never by parsing.** `EP-142` is a display alias (skeleton
rule 1). The handler sends the whole string to `GET /v1/issues/resolve?ref=` (task 83's relay
call) and takes back a key. That is what makes a link survive a project rename: `EP-142` copied
in March still resolves in June after the key became `UPL-142`, because the Hub keeps the old
alias as a redirect. Splitting on `-` and treating the prefix as a project key would break on
exactly that case, and is the mutant a test kills.

**(d) A revision-pinned link opens that revision, read-only, and says so.** `issue:is-…@3` on a
five-revision issue opens the issue with a `--mono` line above the body: `@3 of 5 · viewing an
earlier revision` and a *Go to current* button. The body is not editable at `@3`. This is L-98 —
*what did they read?* — made answerable: the link a person shared names the text they saw.

**(e) The prefill focuses the title and selects nothing.** *New issue* opens with `title` filled,
the caret at the end, and the description below it. It does not submit, it does not pick a team
beyond `project`'s, and it does not set an assignee. Creating the issue is always a person's
keystroke.

**(f) A second instance hands off rather than opening a second window.** The deep-link plugin's
single-instance behaviour is used: a `panacea://` URL with the app already running routes into
the running window. With the app **not** running, the URL is queued and applied after the session
is restored, so a cold-start link does not land on the sign-in screen and vanish.

**(g) The fallback, if the deep-link licence or the managed device says no.** The Hub page keeps
its *Open in Panacea* button, but it copies `issue:is-…@3` to the clipboard and tells the person
to press `Ctrl` `K` and paste. Everything in `links.ts` — `toHubUrl`, the parser, the allow-list,
the alias resolution — is unchanged and still used, because the palette feeds the same parser.
So the fallback costs the two dependencies and the scheme registration, nothing else.

#### Files

| File | New / ⚑ | What it gains |
|---|---|---|
| `desktop/src/lib/links.ts` | new | `toHubUrl`, `parsePanaceaUrl`, `PREFILL_KEYS` |
| `desktop/src/lib/links.test.ts` | new | the unit suite (11 tests) |
| `desktop/src/work/deepLink.ts` | new | the OS handler: subscribe, queue, apply |
| `desktop/src/work/deepLink.test.ts` | new | the handler suite |
| `desktop/src-tauri/Cargo.toml` | ⚑ | `tauri-plugin-deep-link = "2"` |
| `desktop/src-tauri/src/lib.rs` | ⚑ | `.plugin(tauri_plugin_deep_link::init())` |
| `desktop/src-tauri/tauri.conf.json` | ⚑ | `plugins.deep-link.desktop.schemes: ["panacea"]`; a real identifier |
| `desktop/src-tauri/capabilities/default.json` | ⚑ | the plugin's two permissions |
| `desktop/package.json` | ⚑ | `@tauri-apps/plugin-deep-link` |
| `desktop/src/components/issue/IssueHead.tsx` | ⚑ (task 46's) | *Copy link* → `toHubUrl` |
| `desktop/src/components/plan/FeatureView.tsx` | ⚑ (task 86's) | *Copy link* → `toHubUrl` |
| `desktop/src/components/work/ViewHeader.tsx` | ⚑ (task 91's) | *Copy link* → `toHubUrl` |
| `desktop/src/components/palette/…` | ⚑ | *Copy link* and *Open a link…* |

**The file map names none of the four `src-tauri` files or `package.json` for this task**, so they
are added here. The scope says so in as many words.

#### `desktop/src/lib/links.ts`, complete

```ts
// desktop/src/lib/links.ts
//
// Two directions, one file.
//   OUT: toHubUrl — every link a person copies is https:// to the Hub's read-only page.
//   IN:  parsePanaceaUrl — panacea:// is internal, navigates and prefills, and never acts.
// A URL may not cause a write. If you are adding a route here that has a verb in it, stop.

/** What a link points at. `rev` pins a revision; absent means "current". */
export type LinkRef =
  | { readonly kind: "issue"; readonly issueId: string; readonly rev?: number }
  | { readonly kind: "feature"; readonly featureId: string }
  | { readonly kind: "view"; readonly viewId: string }
  | { readonly kind: "issues"; readonly issueIds: readonly string[] };

/** The routes a panacea:// URL may reach. Every one is read-only or a prefill. */
export type Route =
  | { readonly kind: "issue"; readonly ref: string; readonly rev: number | null }
  | { readonly kind: "issues"; readonly refs: readonly string[] }
  | { readonly kind: "feature"; readonly ref: string }
  | { readonly kind: "view"; readonly viewId: string }
  | { readonly kind: "new-issue"; readonly prefill: Prefill };

export interface Prefill {
  readonly title: string | null;
  readonly description: string | null;
  readonly project: string | null;
  readonly labels: readonly string[];
}

export type Refused =
  | { readonly kind: "refused"; readonly reason: "scheme" }
  | { readonly kind: "refused"; readonly reason: "unknown-route" }
  | { readonly kind: "refused"; readonly reason: "malformed" }
  | { readonly kind: "refused"; readonly reason: "too-many"; readonly limit: number };

/** The ONLY query keys ever read. Adding one is a reviewed change (decision b). */
export const PREFILL_KEYS = ["title", "description", "project", "labels"] as const;

/** SW.1's lengths, mirrored so truncation happens before React sees the value. */
export const PREFILL_LIMITS = {
  title: 256,
  description: 16_384,
  project: 16,
  labels: 20,        // count, not length
  labelName: 64,
} as const;

/** The ad hoc list's ceiling (task 91). Over it, the URL is refused rather than truncated. */
export const MAX_IDS = 200;
```

```ts
/**
 * The Hub's public page for a ref. `origin` comes from the session (task 55's snapshot), never
 * from a constant, because dev, the pilot and a customer-hosted Hub are three origins.
 *   toHubUrl({kind:"issue", issueId:"is-7Q3FK2QW9N", rev:3}, origin)
 *     → "https://hub.panacea.kpmg/p/issues/is-7Q3FK2QW9N@3"
 *   toHubUrl({kind:"issues", issueIds:[...]}, origin)
 *     → "https://hub.panacea.kpmg/p/issues?ids=is-…,is-…"
 * Throws on a non-https origin, so a misconfigured session cannot leak an http link.
 */
export function toHubUrl(ref: LinkRef, origin: string): string;

/** Total. Never throws. Never returns a route that writes. */
export function parsePanaceaUrl(url: string): Route | Refused;

/** Shared by the parser and the palette's "Open a link…" box, so both refuse identically. */
export function parseRef(raw: string): { ref: string; rev: number | null } | null;
```

**The four routes, complete.** Anything else is `unknown-route`.

| URL | Route | Row |
|---|---|---|
| `panacea://issue/EP-142` | `{kind:"issue", ref:"EP-142", rev:null}` | L-98 |
| `panacea://issue/is-7Q3FK2QW9N@3` | `{kind:"issue", ref:"is-7Q3FK2QW9N", rev:3}` | L-98 |
| `panacea://issues?ids=EP-1,EP-4` | `{kind:"issues", refs:["EP-1","EP-4"]}` | L-111 |
| `panacea://issue/new?title=…&project=EP` | `{kind:"new-issue", prefill:{…}}` | L-30 |
| `panacea://feature/ft-…`, `panacea://view/vw-…` | the two read routes | — |

`parseRef` accepts both an alias and an `is-` key, with an optional `@<n>`, `n` a positive integer
under 10,000. `issue:is-…@3` — the canonical form a person may paste from a copied Markdown head
line (task 92) — is accepted with its `issue:` prefix stripped, so the two forms are one code
path. A `rev` of `0`, `-1`, `1e3`, `03` or `9999999` is `malformed`, not clamped.

#### The three `src-tauri` changes, complete

```toml
# desktop/src-tauri/Cargo.toml — under [dependencies], beside tauri-plugin-opener (:22)
tauri-plugin-deep-link = "2"
```

```json
// desktop/src-tauri/tauri.conf.json — a new top-level "plugins" block
"plugins": {
  "deep-link": {
    "desktop": {
      "schemes": ["panacea"]
    }
  }
}
```

```json
// desktop/src-tauri/capabilities/default.json — added to "permissions"
"deep-link:default",
"deep-link:allow-get-current"
```

```rust
// desktop/src-tauri/src/lib.rs — one line in the builder chain, before .run()
.plugin(tauri_plugin_deep_link::init())
```

**And the identifier changes.** Windows registers a URL scheme against the bundle identifier, and
this tree still carries the scaffold's `com.au-dev-dmackie1.desktop` (`tauri.conf.json:5`) with
`"productName": "desktop"` (`:3`). Registering `panacea://` under a personal-looking identifier is
wrong on a managed device and wrong in a signed installer. This task sets
`"identifier": "com.kpmg.panacea.app"` and `"productName": "Panacea"`, and names the change
loudly, because it also changes the app's data directory: **any existing local state under the old
identifier is orphaned** (today that is the window layout in `store.ts` and the device id in
`ids.ts`, neither of which is Hub data, so nothing of value is lost).

#### `desktop/src/work/deepLink.ts`, complete

```ts
// desktop/src/work/deepLink.ts
import { onOpenUrl, getCurrent } from "@tauri-apps/plugin-deep-link";
import { parsePanaceaUrl, type Route, type Refused } from "../lib/links";

export interface DeepLinkDeps {
  /** Injected so the whole file is testable with no Tauri runtime. */
  readonly subscribe: (fn: (urls: string[]) => void) => Promise<() => void>;
  readonly pending: () => Promise<string[] | null>;
  readonly navigate: (route: Route) => void;
  readonly refuse: (r: Refused) => void;
  /** Resolves while the session is not yet restored (decision f). */
  readonly ready: () => boolean;
}

/** Starts listening. Returns a stop. Applies a cold-start URL once `ready()` turns true. */
export function startDeepLinks(deps: DeepLinkDeps): Promise<() => void>;

export const tauriDeps = { subscribe: onOpenUrl, pending: getCurrent };
```

**Only the first URL of a batch is applied.** The OS may hand over several at once (a
multi-select drag onto the icon); the rest are refused as `too-many` with `limit: 1`, because a
single keystroke opening six windows is a denial of service somebody can put in an email.

#### Tests to write first, each with the mutant it kills

`links.test.ts` and `deepLink.test.ts` under vitest, both pure: `deepLink.test.ts` drives
`DeepLinkDeps` by hand and never loads `@tauri-apps/plugin-deep-link`. Each is seen failing first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `a copied link is https to the Hub` | every `toHubUrl` output begins `https://` and contains `/p/`; `grep` of the four ⚑ surfaces finds no `panacea://` literal | a `panacea://` link shared out |
| `a non-https origin throws` | `toHubUrl(ref, "http://hub…")` throws | an http link leaked from a misconfigured session |
| `the prefill fills and never submits` | `panacea://issue/new?title=Hi` → the composer renders with `Hi`, focused, and the create call is **not** made; no `POST` on the fake | a link that creates an issue |
| `only four keys are read` | `…&start=1&assignee=me&state=done&title=Hi` → prefill is `{title:"Hi"}` and nothing else; no agent call; the answer is identical with and without the extra keys | `&start=1` starts an agent; a probe that leaks which keys exist |
| `over-long prefill is truncated, not refused` | a 300-char title → 256 chars, a route, no refusal; 30 labels → 20 | a refusal that loses the person's paste |
| `an alias resolves through the Hub` | `panacea://issue/EP-142` calls `resolve?ref=EP-142`; the fake answers a key whose project is now `UPL` and the issue opens; **no** `split("-")` appears in `links.ts` | `EP-` parsed as a project key (rule 1) |
| `a revision-pinned link opens that revision` | `panacea://issue/is-…@3` on a 5-revision issue → the body is revision 3, read-only, with a `--mono` `@3 of 5` line and a *Go to current* | `@3` ignored; an editable old revision |
| `a malformed revision is refused` | `@0`, `@-1`, `@03`, `@1e3`, `@99999` → `{kind:"refused", reason:"malformed"}`; never a clamp | a clamp that opens the wrong text |
| `an id list opens the ad hoc view` | `panacea://issues?ids=EP-1,EP-4` → task 91's list with two rows, in the URL's order; 201 ids → `too-many` | the list re-sorted; an unbounded list |
| `a foreign scheme is refused` | `http://…`, `file://…`, `javascript:…`, `panacea:/../..`, `panacea://issue/new/../../start` → a refusal, never a throw | a thrown parse (a crash on hostile input); path traversal |
| `only the first of a batch is applied` | `subscribe` fires with three URLs → one `navigate`, two `too-many` refusals | three windows from one drop |
| `a cold-start URL waits for the session` | `pending()` answers a URL while `ready()` is false → nothing; when `ready()` turns true → one `navigate` | the link lands on the sign-in screen and is lost |

#### Steps

- [ ] **Step 1: Read the two licences and record the strings.** `cargo metadata` for
  `tauri-plugin-deep-link` and `npm view @tauri-apps/plugin-deep-link license`. If either is not
  MIT, Apache-2.0, BSD or ISC, **stop and ship decision (g)'s fallback**; `links.ts` and its
  eleven tests are unchanged either way, so the task still closes L-98's parse half.
- [ ] **Step 2: The tests.** All twelve. `npm --prefix desktop test`: they fail.
- [ ] **Step 3: `links.ts`.** Pure, no Tauri, no React; green before anything else starts.
- [ ] **Step 4: The four `src-tauri` changes and `package.json`**, then `deepLink.ts`, then its
  test. Build once — `npm --prefix desktop run tauri build --debug` — to prove the Rust side
  compiles and the capability file is accepted.
- [ ] **Step 5: The four `Copy link` surfaces** point at `toHubUrl`, and the palette gains *Open a
  link…* feeding the same parser.
- [ ] **Step 6: Green and commit by pathspec**, the thirteen files of *Files*, naming the four ⚑
  React surfaces as touched, and recording both licence strings in the commit body.

#### Acceptance

```bash
npm --prefix desktop test -- --run src/lib/links.test.ts 2>&1 | tail -5
npm --prefix desktop test -- --run src/work/deepLink.test.ts 2>&1 | tail -5
npm --prefix desktop run typecheck
grep -rn 'panacea://' desktop/src/components/ | wc -l                                  # 0
grep -rn 'split("-")\|split(.-.)' desktop/src/lib/links.ts | wc -l                     # 0
grep -c '"schemes": \["panacea"\]' desktop/src-tauri/tauri.conf.json                   # 1
grep -c 'com.au-dev-dmackie1' desktop/src-tauri/tauri.conf.json                        # 0
cargo metadata --manifest-path desktop/src-tauri/Cargo.toml --format-version 1 \
  | grep -o '"name":"tauri-plugin-deep-link","[^}]*"license":"[^"]*"'
```

Expected: both suites `1 passed` with no failures; `typecheck` silent, exit 0; the greps print
`0`, `0`, `1`, `0`; the last prints the crate's own licence string, which is what step 1 records.
**None has been run; `links.ts` does not exist and the plugin is not installed** (OD-27).

#### What this task does not claim

- **Not that a KPMG-managed device lets a per-user installer register a URL scheme.**
  **UNVERIFIED.** Windows writes the scheme under `HKCU\Software\Classes`, which a per-user MSI
  normally may do, but this estate's application-control and installer policy has not been read by
  anyone here. The packaging owner checks it. Decision (g) is the fallback and costs no rework.
- **Not the licences.** Both are **UNVERIFIED** until step 1 runs (the registry fetch is blocked
  from this session).
- **Not the Hub page**, its sign-in, or its *Open in Panacea* button: all task 80's. This task
  only fixes the shape of the URL the button carries.
- **Not macOS or Linux registration.** The `schemes` key is written under `desktop`, which covers
  all three, but only Windows is a target today and only Windows would be tried.
- **Not a browser extension, and no `https://` deep-link association** (Windows app URI handlers
  need a signed domain-verification file on the Hub's origin). A shared `https://` link opens the
  page; the page hands off.
- **No measured cold-start latency** for a queued URL.

#### Corrections to the scope

1. **Four files, not three.** The scope names `Cargo.toml`, `tauri.conf.json` and `package.json`.
   `src-tauri/src/lib.rs` (the `.plugin()` line) and `capabilities/default.json` (the plugin's
   permissions) are also required; Tauri 2 refuses a plugin command that has no capability entry.
2. **The bundle identifier must change with the scheme.** `com.au-dev-dmackie1.desktop` is the
   scaffold's, and a URL scheme is registered against it. Named here because it orphans the local
   app-data directory; it is not a Hub-data change.
3. **`resolve?ref=` is needed and no part names it.** Decision (c) requires a Hub route that turns
   an alias — including a **stale** alias from before a rename — into a key. Task 80 owns views
   and drafts; task 08 owns numbers and `project.key`. The route is small and additive; **this
   task's step 1 confirms it exists or raises it with the WH stream.**
4. **`MAX_IDS` is this task's, not task 91's.** Task 91's scope gives the ad hoc list no ceiling.
   A URL is the one place an unbounded list arrives from outside, so the bound is set here at 200
   and task 91 reads the constant.

**Parity rows served:** **L-30** (the prefill deep link) and **L-98**'s W2 part (a link names the
revision a person read), jointly with task 80; and **L-111**'s URL half with task 91. Task 97
proves them.
