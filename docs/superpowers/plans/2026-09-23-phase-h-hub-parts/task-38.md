### Task 38: the local caller key — desktop side: one Tauri command, one header on every call, one subprotocol on attach (review I3)

**Lane D. Depends on: 37 (land together). About 3.5 hours (1.5 more than first planned: the attach
subprotocol, review I3, and the pending-read guards, re-review N1). Contract task C3.**

The renderer learns the key task 37 minted through **one** Tauri command, keeps it **in memory
only**, and `client.ts` sends it on every request (contract §9.4). A control plane restarted under
a running window has a new key; the client re-reads it once on 401 `local_key_required` and
retries. **The attach WebSocket carries the key too** (review I3): it cannot set a header, so
`openAttach` opens `new WebSocket(url, ["panacea.v1", "panacea.key." + key])`, and task 37's
control plane compares the second entry and echoes only `panacea.v1`. The key is never in the URL.

**Files:**
- `desktop/src-tauri/src/lib.rs` — `local_key` command and its path ladder (modify ⚑)
- `desktop/src/api/localKey.ts` + `localKey.test.ts` (new)
- `desktop/src/api/client.ts` + `client.test.ts` — the header, the retry, the new codes, the comment at `:6-8` (modify ⚑)
- `desktop/src/api/index.ts` — construct `api` with the key source (modify)
- `desktop/src/api/attachRegistry.ts` + `attachRegistry.test.ts` — `connect` (`:385`) awaits `openAttach`, and a pending key read counts as in flight at `connect`, `retryAttach` and `updateAttachSession` (modify ⚑; review I3, re-review N1)
- The fake clients whose `openAttach` returns a socket synchronously, each changed only to return
  it from an `async` function: `attachRegistry.test.ts:26`, `components/Terminal.test.tsx:45`,
  `defects.test.tsx:92` and `:176`, `hooks/useKeyboardMap.xterm.test.tsx:55` (modify; Step 1
  re-derives the list with `git grep -n "openAttach" -- desktop/src`)

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
  // …baseUrl, fetch, timeoutMs: unchanged…
  /** Injected for the same reason: jsdom's WebSocket opens a real socket. Hub plan task 38 adds
   *  `protocols` (review I3); the default is `(url, protocols) => new WebSocket(url, protocols)`. */
  webSocket?: (url: string, protocols: string[]) => WebSocket;
  /** Hub plan task 38. Absent only in phase 2's own tests, which pre-date the key. */
  localKey?: LocalKeySource;
}

// CockpitClient.openAttach (client.ts:164) becomes async — it must read the key first:
openAttach(session: Session): Promise<WebSocket>;
// With a localKey source: protocols = ["panacea.v1", "panacea.key." + await localKey.refresh()].
// Without one (phase 2's own tests): protocols = [], exactly today's handshake.

// ProblemCode gains the control plane's four new codes (Hub plan task 32) — added to the union,
// nothing removed: "signed_out" | "hub_not_configured" | "local_key_required" | "upstream_unavailable"
```

`send` (`client.ts:242-289`) changes in exactly three ways: every request carries
`Authorization: Bearer <await localKey.get()>` when a source is set (so `headers` is always an
object); a `401` whose body is `local_key_required` calls `localKey.refresh()` and retries **once**;
and the comment at `:6-8` becomes: *"Every REST call carries the local caller key as
`Authorization: Bearer` (Hub plan task 38; contract §9.4). The attach WebSocket cannot set a
header, so it offers the key as its second subprotocol, never in its URL; the control plane echoes
only `panacea.v1`."*

**`openAttach` (`client.ts:385-391`) changes (review I3).** It reads the key with
`localKey.refresh()`, not `get()`: **every handshake reads the key afresh**. A browser reports a
refused handshake as a bare close (1006) with no status, so the socket has no 401 to react to; a
fresh read on every rung of the registry's retry ladder is what lets a terminal reattach after the
control plane restarts with a new key. It is one Tauri call per handshake, and a handshake is
already a round trip. `refresh()` also updates the REST cache, which is the same file. It then
opens `makeSocket(attachUrl(baseUrl, session), protocols)` and sets `binaryType` before returning,
as today. A key that cannot be read rejects the promise with the command's error, which names the
path and never the content. `attachUrl` stays unchanged: no key, no query.

**`attachRegistry.ts` `connect` (`:385`).** `ws = deps.client.openAttach(entry.session)` at `:409`
becomes an `await`, inside the existing `try`, whose `catch` already schedules a retry (`:410-413`).
The comment at `:407-408` (*"No Authorization header … before Phase 8 (§4.5)"*) becomes: *"The
local caller key rides in the subprotocol (Hub plan task 38; UI contract §4.5 as amended by task
44a)."*

**The pending key read is "in flight" too (plan re-review b02 N1).** Phase 2 reads
`entry.ws !== null` as *"a socket is open or a handshake is in flight"*. An `await` leaves
`entry.ws` null for the whole key read, so that one test loses its meaning at three sites:
`connect`'s top guard (`:386`) would let a stale rung or any other caller open a second socket;
`retryAttach`'s guard (`:259`) would pass and zero a ladder the pending call still runs on; and
`updateAttachSession` (`:212`) would publish `ended` for a handshake that then publishes over it.
`entry.attempts` cannot tell the calls apart either: `retryAttach` (`:276`) and `onopen` (`:421`)
both reset it to 0, so a superseding call can land on the same value (ABA) and leak the first
socket, a live attach to the PTY that nothing reads. The fix, complete:

1. `Entry` (`:95-110`) gains two fields. `openSeq: number` is a handshake generation that only
   ever increases, never reset by anything. `opening: number` is the generation of the key read
   now pending, and 0 when none is. Both start at 0 in the entry literal (`:139-159`).
2. One predicate, `inFlight(entry) = entry.ws !== null || entry.opening !== 0`, replaces the bare
   `entry.ws` test at `:386`, `:259` and `:212`, and nowhere else: the socket handlers' own
   `entry.ws !== ws` guards (`:417`, `:429`, `:437`) are about *which* socket, not whether one
   exists, and stay as they are. The comment at `:254-258` and mechanism 2 of `:266-267` now name
   the pending read beside the live socket.
3. `connect`, just before the `await`: `const gen = ++entry.openSeq; entry.opening = gen;`.
4. After the `await` (both the resolve and the `catch`), the call first checks
   `entry.opening === gen && !entry.destroyed`. When that fails, a resolved socket is closed at
   once, nothing is published and nothing is scheduled. When it holds, `entry.opening = 0`, and
   then: a rejected read runs the existing `scheduleRetry(entry)`; a resolved read whose session
   exited during the read (the poller's `updateAttachSession` left it alone, by step 2) closes the
   socket and runs `scheduleRetry(entry)`, whose exited branch (`:503`) publishes `ended`; any
   other resolved read sets `entry.ws = ws` and wires the handlers, exactly as today.
5. `destroy()` (`:643`) also sets `entry.opening = 0`, so a read that settles after the pane closed
   fails step 4's first check on two counts.

**Declared equivalent mutant.** With step 2's three guards in place, nothing can start a second
`connect` while a read is pending, so dropping the `entry.opening === gen` half of step 4 (keeping
`!entry.destroyed`) survives every test. It is the check that keeps the file safe the day one of
those guards is removed, which is exactly how this defect arose. The guards themselves are each
killed by a named test below.

`problems.ts`'s `classify` is **unchanged**: the new codes are `server-said`, rendered verbatim (four call sites switch on
`classify` — `EmptyDevelop.tsx:57`, `SessionPane.tsx:48`, `sessionVerbs.ts:85`,
`useCreateSession.ts:38` — and none needs a new branch).

#### Tests to write first

`client.test.ts` (added) and `localKey.test.ts`, with an injected `fetch` and a fake `invoke`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `sends the key on every verb` | `health`, `listSessions`, `createSession`, `renameSession`, `killSession`, `purgeSession` each carry `Authorization: Bearer k1` | one verb forgotten, and 401 on it in the real app |
| `reads the key once` | ten REST calls → one `invoke("local_key")` (attach handshakes re-read on purpose; see below) | a Tauri round trip per request |
| `re-reads the key once after a restart` | 401 `local_key_required` then 200 → `invoke` twice, the call succeeds, the second request carries `k2` | a window that stays broken until reloaded after the control plane restarts |
| `gives up after one retry` | 401 twice → `ApiError` with code `local_key_required` | an infinite retry |
| `a plain 401 is not retried` | 401 `signed_out` → no refresh; `ApiError` `signed_out` | the key re-read on every sign-out |
| `the attach key rides in the subprotocol, never the URL` (review I3; replaces `the attach socket carries no key`) | the injected `webSocket` is called with `attachUrl(s)` unchanged (no query, no credential) and protocols exactly `["panacea.v1", "panacea.key.k1"]`, in that order | a key in a URL, where logs and history keep it; the entries swapped, which the control plane refuses |
| `every attach handshake reads the key afresh` | `invoke` answers `k1`, then `k2`; two `openAttach` calls → the second socket's protocols carry `panacea.key.k2`, and a REST call after it sends `Bearer k2` | a terminal that never reattaches after a control-plane restart, because the refused handshake gave it no 401 to react to |
| `a key that cannot be read fails the open, not the app` | `invoke` rejects → `openAttach` rejects with the command's message; the registry schedules a retry (`attachRegistry.test.ts`) | an unhandled rejection; a pane stuck on "connecting" |
| `a pane closed while the key is read opens nothing` (`attachRegistry.test.ts`) | `openAttach` held pending, the entry destroyed, then resolved → the socket is closed at once and no phase is published | a socket leaked by the new `await` |
| `a retry while the key is read opens one socket` (`attachRegistry.test.ts`; re-review N1) | `openAttach` held pending on the first `connect`; `retryAttach("s-1")` → returns `false` and `openAttach` is not called again; resolve → exactly one socket exists and it is `entry.ws`, and the published `attempts` was never zeroed | `retryAttach`'s guard testing `entry.ws` alone (`:259`): the second call lands on the same `attempts`, both publish, and the first socket leaks |
| `a stale rung firing during the key read opens nothing` (`attachRegistry.test.ts`; re-review N1) | the mid-ladder shape of `:810-827`: a close arms a 250 ms rung; at t=100 `retryAttach` starts a `connect` whose `openAttach` is held pending; advance to t=250, where the stale rung fires → `openAttach` was called exactly twice in all (the first socket, the retry), not three times; resolve → one live socket | `connect`'s top guard testing `entry.ws` alone (`:386`): the stale rung opens a second socket off the replaced ladder |
| `a session that exits while the key is read ends once` (`attachRegistry.test.ts`; re-review N1) | `openAttach` held pending; `updateAttachSession` with `status: "exited"` → no `ended` is published yet; resolve → the socket is closed at once, `ended` is published once, and `attached` never is | `updateAttachSession`'s guard testing `entry.ws` alone (`:212`): `ended` published while the handshake is pending, and the handshake publishing over it |
| `the key is never in an error` | every `ApiError` above: `message` and `details` do not contain `k1` or `k2` | the key surfacing in the rail footer's error line (`desktop/src/components/rail/RailFooter.tsx:19-21`) |
| `the key is not in the store` | after calls, `JSON.stringify(useCockpit.getState())` and `localStorage` contain neither key | a persisted key that outlives the process |

Rust (`cargo test` in `desktop/src-tauri`): `key_path_follows_statepath` — the four ladder rungs
and the override, with the example paths `C:\Users\u\AppData\Local\PanaceaApp\state\auth\local-key`
and `/tmp/s/auth/local-key` (`PANACEA_APP_STATE_DIR=/tmp/s`), which task 37's Go test also pins.

#### Steps

- [ ] **Step 1:** Re-read contract §9.4; `client.ts` in full; `lib.rs`; `attachRegistry.ts:375-446`
  (`openIfLive` and `connect`), and the other sites re-review N1 touches: `Entry` `:95-110`, its
  literal `:139-159`, `updateAttachSession` `:204-221`, `retryAttach` `:244-276`, `destroy`
  `:643`. Re-derive the fake-client list in Files with its `git grep`.
- [ ] **Step 2:** Tests; they fail. Then `localKey.ts`, the three `send` changes, the codes, the
  comment, the async `openAttach` and its subprotocols, `connect`'s `await`, the `openSeq` /
  `opening` generation and the `inFlight` predicate at its three sites (re-review N1),
  the fakes (each only made `async`), `index.ts`, `lib.rs`. A phase-2 test that reads a fake socket
  in the same tick as the attach now awaits one microtask first; that is the only other change a
  test may get (how many such tests exist is unverified until Step 2 runs them).
- [ ] **Step 3:** `cd desktop && npm test && npm run typecheck`; `cd desktop/src-tauri && cargo test`.
- [ ] **Step 4:** With task 37's control plane running: `npm run tauri dev`; the rail loads
  projects and sessions (every call authorised); a terminal attaches (so WebView2 accepted the
  `panacea.v1` echo, review I3) and takes keystrokes; restart the control plane — the next poll
  recovers, and the terminal reattaches, without reloading the window. **Record what was seen**,
  and that this is the implementer's observation, not the owner's (`OD-12`).
- [ ] **Step 5:** Commit by pathspec, together with task 37's merge.

#### Acceptance

```bash
cd desktop && npm test 2>&1 | tail -3           # all passed
cd desktop && npm run typecheck                  # no output
cd desktop/src-tauri && cargo test key_path      # 1 passed
git grep -n "No Authorization header\|no Authorization header" -- desktop/src/api/client.ts desktop/src/api/attachRegistry.ts   # no output
git grep -n "panacea.key." -- desktop/src/api/client.ts   # the one site that builds the subprotocol
```

#### What this task does not claim

- That the command works in a packaged build: checked in `tauri dev` only; the packaged MSI is
  phase 9's.
- Protection against script in the webview reading the key: the webview holds it by design, as it
  holds the power to call the control plane today (contract T7).
