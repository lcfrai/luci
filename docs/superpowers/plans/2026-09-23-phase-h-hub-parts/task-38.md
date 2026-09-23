### Task 38: the local caller key — desktop side: one Tauri command, one header on every call

**Lane D. Depends on: 37 (land together). About 2 hours. Contract task C3.**

The renderer learns the key task 37 minted through **one** Tauri command, keeps it **in memory
only**, and `client.ts` sends it on every request (contract §9.4). A control plane restarted under
a running window has a new key; the client re-reads it once on 401 `local_key_required` and
retries. The attach WebSocket is unchanged: it cannot carry a header, and the control plane exempts
it (task 37).

**Files:**
- `desktop/src-tauri/src/lib.rs` — `local_key` command and its path ladder (modify ⚑)
- `desktop/src/api/localKey.ts` + `localKey.test.ts` (new)
- `desktop/src/api/client.ts` + `client.test.ts` — the header, the retry, the new codes, the comment at `:6-8` (modify ⚑)
- `desktop/src/api/index.ts` — construct `api` with the key source (modify)

#### Contract — complete

```rust
// desktop/src-tauri/src/lib.rs — added. `greet` stays (it is phase 2's scaffold).

use std::path::PathBuf;

/// The local caller key's file (Hub plan task 37): PANACEA_APP_LOCAL_KEY_FILE, else
/// statepath's ladder (control-plane/internal/statepath/statepath.go:30-45) + "auth/local-key":
///   $PANACEA_APP_STATE_DIR/auth/local-key
///   %LOCALAPPDATA%\PanaceaApp\state\auth\local-key
///   $XDG_STATE_HOME/panacea-app/auth/local-key
///   ~/.local/state/panacea-app/auth/local-key
/// Pure over its inputs, so the test pins the same example paths as the Go side.
fn key_path(getenv: impl Fn(&str) -> Option<String>, home: Option<PathBuf>) -> Option<PathBuf>

/// Reads the key. The error names the path and the cause, never the file's content.
#[tauri::command]
fn local_key() -> Result<String, String>

// in run(): .invoke_handler(tauri::generate_handler![greet, local_key])
```

An app-defined command needs no capability entry in Tauri 2 (phase 2's `greet` has none and
`capabilities/default.json` lists only `core:default` and `opener:default`); **unverified by a run
here** — Step 4 checks it in the real window.

```ts
// desktop/src/api/localKey.ts

/** The local caller key (Hub plan task 37). Held in memory only: never in the store, never in
 *  localStorage, never in a log (contract §9.4; ADR-H1 §5.4 widened A5). */
export interface LocalKeySource {
  /** The cached key, reading it once on first use. */
  get(): Promise<string>;
  /** Forget the cached key and read it again — after a 401 local_key_required. */
  refresh(): Promise<string>;
}

/** Reads through the Tauri command. `invoke` is injected so tests need no Tauri. */
export function tauriLocalKey(
  invoke?: (cmd: "local_key") => Promise<unknown>,
): LocalKeySource;
```

```ts
// desktop/src/api/client.ts — the changes, complete.

export interface ClientOptions {
  // …phase 2's four fields, unchanged…
  /** Hub plan task 38. Absent only in phase 2's own tests, which pre-date the key. */
  localKey?: LocalKeySource;
}

// ProblemCode gains the control plane's four new codes (Hub plan task 32) — added to the union,
// nothing removed: "signed_out" | "hub_not_configured" | "local_key_required" | "upstream_unavailable"
```

`send` (`client.ts:242-289`) changes in exactly three ways: every request carries
`Authorization: Bearer <await localKey.get()>` when a source is set (so `headers` is always an
object); a `401` whose body is `local_key_required` calls `localKey.refresh()` and retries **once**;
and the comment at `:6-8` becomes: *"Every REST call carries the local caller key (Hub plan task
38; contract §9.4). The attach WebSocket carries none: the control plane exempts it, and it reaches
a local PTY, never the Hub."* `attachUrl`, `openAttach` and `problems.ts`'s `classify` are
**unchanged**: the new codes are `server-said`, rendered verbatim (four call sites switch on
`classify` — `EmptyDevelop.tsx:57`, `SessionPane.tsx:48`, `sessionVerbs.ts:85`,
`useCreateSession.ts:38` — and none needs a new branch).

#### Tests to write first

`client.test.ts` (added) and `localKey.test.ts`, with an injected `fetch` and a fake `invoke`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `sends the key on every verb` | `health`, `listSessions`, `createSession`, `renameSession`, `killSession`, `purgeSession` each carry `Authorization: Bearer k1` | one verb forgotten, and 401 on it in the real app |
| `reads the key once` | ten calls → one `invoke("local_key")` | a Tauri round trip per request |
| `re-reads the key once after a restart` | 401 `local_key_required` then 200 → `invoke` twice, the call succeeds, the second request carries `k2` | a window that stays broken until reloaded after the control plane restarts |
| `gives up after one retry` | 401 twice → `ApiError` with code `local_key_required` | an infinite retry |
| `a plain 401 is not retried` | 401 `signed_out` → no refresh; `ApiError` `signed_out` | the key re-read on every sign-out |
| `the attach socket carries no key` | `attachUrl(s)` has no query or credential; `openAttach` adds nothing | a key in a URL, where logs and history keep it |
| `the key is never in an error` | every `ApiError` above: `message` and `details` do not contain `k1` or `k2` | the key surfacing in the rail footer's error line (`RailFooter.tsx:74-76`) |
| `the key is not in the store` | after calls, `JSON.stringify(useCockpit.getState())` and `localStorage` contain neither key | a persisted key that outlives the process |

Rust (`cargo test` in `desktop/src-tauri`): `key_path_follows_statepath` — the four ladder rungs
and the override, with the example paths `C:\Users\u\AppData\Local\PanaceaApp\state\auth\local-key`
and `/tmp/s/auth/local-key` (`PANACEA_APP_STATE_DIR=/tmp/s`), which task 37's Go test also pins.

#### Steps

- [ ] **Step 1:** Re-read contract §9.4; `client.ts` in full; `lib.rs`.
- [ ] **Step 2:** Tests; they fail. Then `localKey.ts`, the three `send` changes, the codes, the
  comment, `index.ts`, `lib.rs`.
- [ ] **Step 3:** `cd desktop && npm test && npm run typecheck`; `cd desktop/src-tauri && cargo test`.
- [ ] **Step 4:** With task 37's control plane running: `npm run tauri dev`; the rail loads
  projects and sessions (every call authorised); a terminal attaches; restart the control plane —
  the next poll recovers without reloading the window. **Record what was seen**, and that this is
  the implementer's observation, not the owner's (`OD-12`).
- [ ] **Step 5:** Commit by pathspec, together with task 37's merge.

#### Acceptance

```bash
cd desktop && npm test 2>&1 | tail -3           # all passed
cd desktop && npm run typecheck                  # no output
cd desktop/src-tauri && cargo test key_path      # 1 passed
git grep -n "No Authorization header\|no Authorization header" -- desktop/src/api/client.ts   # no output
```

#### What this task does not claim

- That the command works in a packaged build: checked in `tauri dev` only; the packaged MSI is
  phase 9's.
- Protection against script in the webview reading the key: the webview holds it by design, as it
  holds the power to call the control plane today (contract T7).
