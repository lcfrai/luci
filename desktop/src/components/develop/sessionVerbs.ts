// The session lifecycle verbs, shared by the rail's right-click menu and the pane's ⋯ menu.
// Contract §6.5 and §9's task 22 row. Two commit shapes, chosen per verb:
//
//   deferred  — the store changes now, the request is HELD for the Undo window, and Undo
//               cancels it. For verbs with no compensating request: Kill, Forget.
//   immediate — the store changes now, the request goes now, a refusal restores the
//               snapshot. For verbs the server can undo: Rename, Restart.
//
// Resume, Fork and Open-a-shell create rather than mutate and get no Undo at all. A single
// "optimistic" helper that sent everything immediately would offer an Undo on Forget that could
// not work, which is worse than no button.
import { useSyncExternalStore } from "react";
import { api } from "../../api";
import { getAttachState, retryAttach } from "../../api/attachRegistry";
import type { ApiError } from "../../api/client";
import { classify } from "../../api/problems";
import type { CreateSessionOptions } from "../../hooks/useCreateSession";
import { replaceById } from "../../lib/immutable";
import {
  __resetPendingSessions,
  isSessionPending as isPending,
  setSessionPending as setPending,
  subscribePending,
} from "../../lib/pendingSessions";
import type { MenuItem } from "../menu/ContextMenu";
import { selectHarness, selectSession, selectWorktrees } from "../../store/selectors";
import { TOAST_TTL_UNDO } from "../../store/slices/uiSlice";
import { useCockpit } from "../../store/store";
import type { Session, SessionId, Worktree, WorktreeId } from "../../store/types";

/** What a verb needs from whichever surface is rendering it. */
export interface VerbDeps {
  /** Task 18's useCreateSession() result. The one create path; this module does not add a second. */
  createSession: (worktreeId: WorktreeId, options?: CreateSessionOptions) => Promise<void>;
  /** Closes the host menu before the verb runs, so an Undo toast is not hidden behind it. */
  close: () => void;
  /** Arms the inline rename input. SessionMenu owns the input; the rail menu re-routes to it. */
  beginRename: (id: SessionId) => void;
}

/* ── the pending registry ────────────────────────────────────────────────────────────── */

/** The in-flight timers of deferred commits. The pending IDS moved to `lib/pendingSessions`
 *  in fix round 1, because `dataSlice.replaceSessions` has to read them and a store slice
 *  importing this module — which imports the store — would be a runtime cycle. The timers stay
 *  here: they are this module's machinery and nothing else reads them. */
const timers = new Set<ReturnType<typeof setTimeout>>();

export { isSessionPending } from "../../lib/pendingSessions";

/** One boolean per row. A primitive snapshot, so useSyncExternalStore never sees a new
 *  reference for an unchanged value and a rail of forty rows re-renders none of them.
 *
 *  The hook lives here rather than beside the Set so that `lib/pendingSessions` imports no
 *  React and stays a leaf that cannot participate in a cycle. */
export function useSessionPending(id: SessionId): boolean {
  return useSyncExternalStore(
    subscribePending,
    () => isPending(id),
    () => false,
  );
}

/** Test-only. vitest's restoreMocks does not clear module state, and a leaked pending id
 *  makes a later test fail in a file that did nothing wrong. Mirrors __resetHeartbeat. */
export function __resetSessionVerbs(): void {
  for (const timer of timers) clearTimeout(timer);
  timers.clear();
  __resetPendingSessions();
  raised = null;
}

/* ── the one place a failure becomes a sentence ──────────────────────────────────────── */

/** The prose is local because its tail names THIS act — nothing was changed — and the
 *  discrimination is `classify` because §4.2's non-collapse rule is one rule and four copies of
 *  it drift. Every other code, the sixteenth `non_loopback_publish` above all, lands in
 *  `server-said` and reaches the user as the server wrote it: that sandbox IS running, so a
 *  paraphrase into "not running" would be a lie in the one sentence this path exists to get right.
 *
 *  LOAD-BEARING SHAPE: declared return type, switch over `ProblemKind`, no `default:`. A fourth
 *  kind must fail this function with TS2366 under `strict: true` rather than fall through a
 *  catch-all. This is the fourth call site of that shape; the other three are
 *  `useCreateSession.ts`, `SessionPane.tsx` and `EmptyDevelop.tsx`. */
function sentenceFor(error: ApiError): string {
  switch (classify(error)) {
    case "control-plane-down":
      return "The Panacea control plane is not running, so nothing was changed.";
    case "backend-down":
      return "Docker is not running on this machine, so nothing was changed.";
    case "server-said":
      return error.message;
  }
}

/** The report this module last wrote to `lastError`. setError stores the object it is handed,
 *  so identity says whether the footer still shows a verb's report. */
let raised: { code: string; message: string } | null = null;

function report(error: ApiError): void {
  const state = useCockpit.getState();
  raised = { code: error.code, message: error.message };
  state.setError(raised);
  // No undo argument: by the time this runs the window has closed and the button is gone.
  // An Undo offered here would be a button with nothing to take back.
  state.pushToast(sentenceFor(error));
}

/** A verb succeeded. Clears the footer only if it still shows the report a verb raised.
 *
 *  AMENDED BY TASK 24's FIX ROUND 1 (review I1), a declared departure into task 22's file.
 *  Rename and Restart used to call `setError(null)` outright, so stopping a session in one
 *  project wiped another project's failed branch list off the footer while that project still
 *  showed no branches: an empty list with a clean footer, which is what ruling B (contract §4.7)
 *  exists to prevent. Contract §3.2: an operation clears only its own report. These verbs are
 *  one operation, because they share `report`, so all five success paths call this: the
 *  immediate commit, the deferred commit, Mark ready, Resume and Fork. Before round 1 only the
 *  first cleared at all, so a failed Kill stayed on the footer after a Kill that worked. Round 1
 *  wired the first three; round 2 (re-review I1) wired Resume and Fork, whose `harness_unavailable`
 *  sentence otherwise outlived the Resume that worked. A new path that calls `report` joins this
 *  operation, so it calls this on success and gets a row in the test file's `SUCCEEDS` table. */
function clearOwnReport(): void {
  if (raised !== null && useCockpit.getState().lastError === raised) useCockpit.getState().setError(null);
}

/* ── commit shape 1: deferred. The request is held for the Undo window. ──────────────── */

function runDeferred<T>(opts: {
  sessionId: SessionId;
  message: string;
  /** The optimistic patch. Applied synchronously, before anything is sent. */
  apply: () => void;
  /** The exact inverse of `apply`. Used by Undo, and by a late failure. */
  rollback: () => void;
  commit: () => Promise<T>;
  /** What to do with the server's answer, for the verbs that get one. Kill's `killSession`
   *  returns the `Session` it killed; Forget's `purgeSession` returns nothing, so it passes
   *  none. Generic rather than `unknown`, so a verb cannot quietly adopt the wrong shape. */
  adopt?: (result: T) => void;
}): void {
  const { sessionId, message, apply, rollback, commit, adopt } = opts;
  // Set the instant the request leaves. The Undo button is already off the screen by then, but
  // the store does not drop the toast until ToastRow's own timer fires, and a click landing in
  // that gap would roll back a change the server had already accepted — leaving the cockpit
  // showing a running row over a killed PTY until task 19's poller disagreed with it.
  let sent = false;

  apply();
  setPending(sessionId, true);

  /** ONE caller, the timer below, and that is what makes this whole shape safe to read.
   *
   *  An earlier version also carried an `undone` flag checked here. A mutation that deleted the
   *  check SURVIVED the suite, and it survived correctly rather than for want of a test: Undo
   *  clears the timer, so `settle` cannot run after Undo, and `sent` stops an Undo that lands
   *  after it. The flag was a third mechanism for an invariant two already held, and dead code
   *  on a destructive path is worse than none — it reads as protection while doing nothing.
   *
   *  What replaces it is the single-caller property above, which the commit step guards:
   *      grep -cE '^[[:space:]]*void settle\(\);' sessionVerbs.ts   # exactly 1
   *  A second caller would reach this without the timer's cancellation behind it, and the guard
   *  fails at the moment that decision is being made rather than afterwards. */
  async function settle(): Promise<void> {
    sent = true;
    try {
      const result = await commit();
      // Before `setPending(false)` in the finally below, so the poller suppression is still in
      // force while the server's own row lands — otherwise a tick between the two could take
      // the row back to the state this request just changed.
      adopt?.(result);
      clearOwnReport();
    } catch (caught) {
      // The window has closed. Put the row back, say why in plain text, and let task 19's
      // 2 s poller reconcile anything the rollback guessed wrong on its next tick.
      //
      // PENDING IS CLEARED FIRST, and the order is load-bearing rather than tidy. Forget's
      // rollback goes through `replaceSessions`, which now SUPPRESSES writes for pending ids —
      // so rolling back while still pending would have the store discard its own restore, and
      // the forgotten row would never come back. The Undo path above already clears before it
      // rolls back, for the same reason; this is the same ordering, on the failure path.
      // Nothing can interleave between the two: both are synchronous, with no await between.
      setPending(sessionId, false);
      rollback();
      report(caught as ApiError);
    } finally {
      setPending(sessionId, false);
    }
  }

  const timer = setTimeout(() => {
    timers.delete(timer);
    void settle();
  }, TOAST_TTL_UNDO);
  timers.add(timer);

  // The toast's own TTL is the same constant, so the button disappears at the moment the
  // request is sent. One constant, two timers, and they cannot drift apart.
  useCockpit.getState().pushToast(message, () => {
    if (sent) return;
    // THE cancellation. `settle` has no flag of its own any more, so this clearTimeout is the
    // only thing standing between an Undo and a request the user took back — see the note on
    // `settle`. Its own mutant (delete these two lines) is killed by "Undo inside the window
    // cancels the request and restores the session".
    clearTimeout(timer);
    timers.delete(timer);
    setPending(sessionId, false);
    rollback();
  });
}

/* ── commit shape 2: immediate. Sent now; a refusal restores the snapshot. ───────────── */

function runImmediate(opts: {
  sessionId: SessionId;
  apply: () => void;
  rollback: () => void;
  commit: () => Promise<Session>;
}): void {
  const { sessionId, apply, rollback, commit } = opts;
  apply();
  setPending(sessionId, true);
  void (async () => {
    try {
      // The server's object replaces the optimistic one outright: it carries updatedAt and
      // an attachPath the cockpit cannot derive, and guessing either is how a pane ends up
      // attached to a socket that no longer exists.
      useCockpit.getState().upsertSession(await commit());
      clearOwnReport();
    } catch (caught) {
      rollback();
      report(caught as ApiError);
    } finally {
      setPending(sessionId, false);
    }
  })();
}

/** Restores one session object exactly as it was. Captured before the optimistic patch. */
function restoreSession(snapshot: Session): () => void {
  return () => {
    useCockpit.getState().upsertSession(snapshot);
  };
}

/** Forget's rollback, and it cannot be `restoreSession`. `removeSession` drops the id from
 *  `sessionIdsByWorktree`, and contract §3.2 makes `upsertSession` PREPEND an id the store no
 *  longer holds — so the forgotten row would come back at the top of its worktree rather than
 *  where the user left it, which is a silent reorder of a list the server owns.
 *
 *  It drops three more things with it: the hand-placed Canvas tile, the pin, and the focus
 *  (`dataSlice.ts:160-191`). A rollback that restored only the session list would undo a Forget
 *  into a session whose tile placement had silently vanished — known defect 6 arriving through a
 *  side door, and on the COMMON path, because Undo is the button this task exists to offer.
 *
 *  The capture happens when this is CALLED, which is while the argument list of `runDeferred` is
 *  being evaluated — before `apply()` runs.
 *
 *  KNOWN AND ACCEPTED: `rows` is a wholesale snapshot up to six seconds old, which is the shape
 *  `markWorktreeReady` deliberately rejects one screen below — and sessions are the list that
 *  really is polled, so this one can genuinely be stale. It is accepted here and not there
 *  because the two rollbacks are not the same operation: marking a worktree replaces ONE element
 *  and `replaceById` can express that against the live list, while un-forgetting has to restore
 *  a row AT AN INDEX the live list no longer has a slot for. The cost is bounded — a session the
 *  poll added during the window is dropped from the index for at most one tick, and 2 s later
 *  `replaceSessions` puts it back from the server, which is authoritative for exactly this list.
 *  A rewrite would be an index-merge for a 2 s window, which is more machinery than the defect. */
function restoreWorktreeSessions(worktreeId: WorktreeId, sessionId: SessionId): () => void {
  const state = useCockpit.getState();
  const rows = (state.sessionIdsByWorktree[worktreeId] ?? [])
    .map((id) => state.sessions[id])
    .filter((session): session is Session => session !== undefined);
  const tile = state.tiles[sessionId];
  const pinIndex = state.pinned.indexOf(sessionId);
  const wasFocused = state.selection.sessionId === sessionId;

  return () => {
    // `replaceSessions` rebuilds the index in the order it is given, which is what puts the row
    // back at its position rather than at the top.
    useCockpit.getState().replaceSessions(rows, [worktreeId]);
    // setState rather than setTile/pinSession, because the forward actions are not inverses:
    // `pinSession` APPENDS, so it cannot restore a pin's place in Canvas order, and it pushes a
    // cap toast when the branch is already full — a toast about a pin the user never made.
    // Both writes are no-ops when there was nothing to restore, and neither clobbers a pin the
    // user made during the window.
    useCockpit.setState((s) => ({
      tiles: tile === undefined ? s.tiles : { ...s.tiles, [sessionId]: tile },
      pinned:
        pinIndex === -1 || s.pinned.includes(sessionId)
          ? s.pinned
          : [...s.pinned.slice(0, pinIndex), sessionId, ...s.pinned.slice(pinIndex)],
    }));
    // Only when nothing else took focus in the window: re-focusing would yank the user out of a
    // session they moved to while the toast was still up.
    if (wasFocused && useCockpit.getState().selection.sessionId === null) {
      useCockpit.getState().focusSession(sessionId);
    }
  };
}

/* ── the verbs ───────────────────────────────────────────────────────────────────────── */

/** Built from a store snapshot at the moment the menu opens. A momentary menu does not
 *  relabel itself mid-hover, and the alternative — subscribing the always-mounted
 *  ContextMenu to the whole session map — re-renders it on every 2 s poll. */
export function sessionVerbs(sessionId: SessionId, deps: VerbDeps): MenuItem[] {
  const state = useCockpit.getState();
  const session = selectSession(sessionId)(state);
  if (session === undefined) {
    // A purge that raced the render. Not a crash, and not an empty menu the user cannot
    // read: one row that says what happened.
    return [
      {
        id: "gone",
        label: "This session is no longer in the list",
        disabledReason: "It was forgotten, or the poll has not caught up yet.",
      },
    ];
  }

  const harness = selectHarness(session.harness)(state);
  // §3.7's selector for the read; the one-line exclusion is local because this module is not a
  // component and holds no subscription — it reads one snapshot per right-click, so the filtered
  // array never enters a render path and a `selectForkTargets` would put task 17's file into
  // this commit for a derivation with one caller.
  const siblings = selectWorktrees(session.branchId)(state).filter((w) => w.id !== session.worktreeId);
  const alive = session.status !== "exited";
  const act = (fn: () => void) => (): void => {
    deps.close();
    fn();
  };

  const items: MenuItem[] = [
    {
      id: "resume",
      label: "Resume",
      run:
        harness?.supportsResume === true
          ? act(() => {
              void (async () => {
                try {
                  const next = await api.resumeSession(session, harness);
                  useCockpit.getState().upsertSession(next);
                  useCockpit.getState().focusSession(next.id);
                  clearOwnReport();
                } catch (caught) {
                  report(caught as ApiError);
                }
              })();
            })
          : undefined,
      disabledReason:
        harness === undefined
          ? "This session's harness is not in the catalogue."
          : harness.supportsResume
            ? undefined
            : `${harness.name} cannot resume a session.`,
    },
  ];

  if (siblings.length === 0) {
    items.push({
      id: "fork",
      label: "Fork into…",
      disabledReason:
        "This branch has only one worktree, and Phase 2 cannot create another (contract §10.3).",
    });
  } else {
    for (const worktree of siblings) {
      items.push({
        id: `fork-${worktree.id}`,
        label: `Fork into ${worktree.name}`,
        run:
          harness === undefined
            ? undefined
            : act(() => {
                void (async () => {
                  try {
                    const next = await api.forkSession(session, worktree.id, harness);
                    useCockpit.getState().upsertSession(next);
                    useCockpit.getState().focusSession(next.id);
                    clearOwnReport();
                  } catch (caught) {
                    report(caught as ApiError);
                  }
                })();
              }),
        disabledReason:
          harness === undefined ? "This session's harness is not in the catalogue." : undefined,
      });
    }
  }

  return items.concat(tailVerbs(session, alive, deps, act));
}

/** The remaining verbs, split out so one function does not run past a screen and the two commit
 *  shapes stay visible side by side. */
function tailVerbs(
  session: Session,
  alive: boolean,
  deps: VerbDeps,
  act: (fn: () => void) => () => void,
): MenuItem[] {
  const id = session.id;
  // A deferred verb is already holding a request for this session. Both deferred verbs are gated
  // on it, because `session.status` here is the OPTIMISTICALLY PATCHED status: during a Kill
  // window the row already reads `exited`, which is exactly the condition that enables Forget. A
  // stacked Forget would then remove the row, and the Kill's own rollback would `upsertSession`
  // it back — prepending an id the store no longer holds, so the row returns at the TOP of its
  // worktree instead of where the user left it. One undoable change at a time removes the whole
  // family rather than the one pair that happens to collide.
  const inFlight = isPending(id);
  const inFlightReason = "Another change to this session has not finished yet.";

  /** Only while the ladder is spent, and RENDERED rather than disabled when it is not.
   *
   *  `failed` is terminal: nothing re-attempts on its own, and the pane's exit bar cannot offer
   *  the way out because it is absent by definition — the socket died while the session is NOT
   *  exited, so the Restart that answers `exited` never appears. Without this row a failed pane
   *  has no way forward from the pane itself.
   *
   *  Hidden rather than disabled because it is a repair for a transient transport state, not a
   *  capability the cockpit is withholding — §6.4 wants a reason on every disabled control, and
   *  "the attach is working" is not a reason a healthy session should carry on every right-click.
   *
   *  Reading the registry here does not make this a subscription: `sessionVerbs` takes one
   *  snapshot when the menu opens, and `getAttachState` is a synchronous snapshot read. That is
   *  why the verb lives here rather than in `SessionPane`, which would have needed a new surface
   *  in a file this task does not own. */
  const retryRow: MenuItem[] =
    getAttachState(id).phase === "failed"
      ? [
          {
            id: "retry-attach",
            label: "Retry attach",
            run: act(() => {
              if (retryAttach(id)) return;
              // It refuses when the pane has closed, the grace has run out, or a poll turned the
              // session exited under the open menu. Saying so beats a row that silently no-ops.
              useCockpit
                .getState()
                .pushToast("That pane is no longer attached, so there was nothing to retry.");
            }),
          },
        ]
      : [];

  return [
    ...retryRow,
    {
      id: "restart",
      // §6.5 says "Restart clean". Phase 2 has no worktree mutation endpoint (§10.3), so the
      // cockpit cannot promise a clean checkout and does not claim one. Same call as the pane's
      // exit bar — `api.restartSession` keeps the session id, so the transcript and the record
      // survive and nothing is orphaned. Not a create-with-copied-settings.
      label: "Restart",
      run: act(() =>
        runImmediate({
          sessionId: id,
          apply: () =>
            useCockpit.getState().upsertSession({ ...session, status: "starting", exitReason: "" }),
          rollback: restoreSession(session),
          commit: () => api.restartSession(id),
        }),
      ),
    },
    {
      id: "rename",
      label: "Rename…",
      // The ONE verb that is not wrapped in `act`, and the exception is load-bearing. `act`
      // closes the host menu before running, which is right for every verb that leaves — and
      // fatal for this one: in `SessionMenu` the host is the menu that is about to render the
      // rename input, so closing it first unmounts the surface the verb exists to show. A test
      // whose `onClose` is an inert spy cannot see that; the real `closeSessionMenu` nulls
      // `sessionMenuFor` and the input never appears at all.
      //
      // So the host decides what closing means. `SessionMenu` stays open and swaps its body;
      // `ContextMenu` closes itself and opens the ⋯ menu, inside its own `beginRename`.
      run: () => deps.beginRename(id),
    },
    {
      id: "shell",
      label: "Open a shell here",
      separatorBefore: true,
      run: act(() => void deps.createSession(session.worktreeId, { harness: "shell" })),
    },
    {
      id: "kill",
      label: "Kill",
      danger: true,
      separatorBefore: true,
      run:
        alive && !inFlight
          ? act(() =>
              runDeferred({
                sessionId: id,
                message: `Killed ${session.title}.`,
                apply: () =>
                  useCockpit
                    .getState()
                    .upsertSession({ ...session, status: "exited", exitReason: "killed" }),
                rollback: restoreSession(session),
                commit: () => api.killSession(id),
                // The server's own object, for the reason `runImmediate` argues at length: it
                // carries `updatedAt` and an `attachPath` the cockpit cannot derive. Discarding
                // it used to read as harmless only because the 2 s poller overwrote it — which
                // is precisely what this round's suppression now prevents.
                adopt: (killed) => useCockpit.getState().upsertSession(killed),
              }),
            )
          : undefined,
      disabledReason: inFlight
        ? inFlightReason
        : alive
          ? undefined
          : "This session has already exited.",
    },
    {
      id: "forget",
      label: "Forget",
      danger: true,
      run:
        alive || inFlight
          ? undefined
          : act(() =>
              runDeferred({
                sessionId: id,
                message: `Forgot ${session.title}.`,
                apply: () => useCockpit.getState().removeSession(id),
                // NOT restoreSession: see the comment on this helper. Forget is the one verb
                // whose rollback has to put a row back at an index, with its tile and its pin.
                rollback: restoreWorktreeSessions(session.worktreeId, id),
                commit: () => api.purgeSession(id),
              }),
            ),
      disabledReason: inFlight
        ? inFlightReason
        : alive
          ? "Kill it first — Forget removes the record, not the process."
          : undefined,
    },
    {
      id: "copy-session-id",
      label: "Copy session id",
      separatorBefore: true,
      run: act(() => copyId(id)),
    },
  ];
}

function copyId(id: SessionId): void {
  void navigator.clipboard?.writeText(id);
}

/* ── the rename runner, called by SessionMenu's inline input ─────────────────────────── */

/** Optimistic and immediate: the title is the user's own word, so showing it at once is not
 *  a guess. The client raises `bad_request` locally for an empty title (task 16), so a blank
 *  submit never reaches the network and still lands in the same failure path.
 *
 *  Contract §8 defect 7 is *"an edit of one of many splices the later ones"*, and the store is
 *  where that is settled: `sessions` is a `Record`, and §3.2 pins `upsertSession` to leave the
 *  position in `sessionIdsByWorktree` UNCHANGED for an id it already holds. So the rename does
 *  not need `replaceById` — there is no array here to splice — and the defect's regression test
 *  *"renaming the middle session leaves its neighbours unchanged"* asserts exactly that, in the
 *  file §8 names. `replaceById` is consumed below, where this task DOES hold an array. */
export function renameSessionTo(sessionId: SessionId, title: string): void {
  const snapshot = selectSession(sessionId)(useCockpit.getState());
  if (snapshot === undefined) return;
  runImmediate({
    sessionId,
    apply: () => useCockpit.getState().upsertSession({ ...snapshot, title: title.trim() }),
    rollback: restoreSession(snapshot),
    commit: () => api.renameSession(sessionId, title),
  });
}

/* ── contract §10.2's verb, and known defect 9 ───────────────────────────────────────── */

/** Optimistic and immediate: the flag is a boolean the user set, the server's answer is the
 *  same worktree object, and a refusal puts the tick back where it was. The list write goes
 *  through task 15's `replaceById` — contract §8 defect 7's helper, consumed here because
 *  "replacing one worktree in a list" is the other half of that defect's Phase 2 shape, and a
 *  hand-rolled `.map` is how the two spellings drift apart.
 *
 *  BOTH directions go by id, through the same `write`, and this task's plan had it backwards.
 *  The plan captured the branch's whole array up front and rolled back to it, arguing that
 *  rebuilding from the CURRENT list "would silently drop anything a poll added in between". It
 *  is the other way round: `setWorktrees` replaces the list wholesale, so restoring a snapshot
 *  taken before the request is exactly what discards a worktree the 2 s poll added while it was
 *  in flight. `replaceById` against the live list touches one element and keeps every other,
 *  which is the whole reason the forward path uses it.
 *
 *  It is also right at the other edge: `replaceById` is a no-op copy for an id the list no
 *  longer holds and never appends, so a worktree the server dropped mid-request stays dropped
 *  rather than being resurrected by a rollback. */
export function markWorktreeReady(worktree: Worktree): void {
  const branchId = worktree.branchId;
  const write = (next: Worktree): void => {
    const current = selectWorktrees(branchId)(useCockpit.getState());
    // `replaceById` returns a COPY for an id it cannot find (`lib/immutable.ts:31`), and
    // `selectWorktrees` returns the shared empty constant for a branch the store does not hold —
    // so writing unconditionally would create `worktreesByBranch[branchId] = []` where there was
    // no key at all, inventing an empty branch out of a failed lookup. Nothing to replace is not
    // an error here; it means the branch was never loaded, or a poll dropped the worktree while
    // the menu was open.
    if (!current.some((w) => w.id === next.id)) return;
    useCockpit.getState().setWorktrees(branchId, replaceById(current, next));
  };

  write({ ...worktree, ready: true });
  void (async () => {
    try {
      write(await api.setWorktreeReady(worktree.projectId, worktree.id, true));
      clearOwnReport();
    } catch (caught) {
      write(worktree);
      report(caught as ApiError);
    }
  })();
}

/** §10.2's verb as one row, built beside the runner that fills it rather than as JSX inside
 *  `ContextMenu`, so the test contract §8 defect 9 names can reach it — this task writes no
 *  `ContextMenu` test.
 *
 *  UNCONDITIONAL, and that is a correction to this task's own first draft. §10.2 LANDED: task 10
 *  shipped `worktreeJSON.Ready bool` with no `omitempty`, so the flag is always on the wire as
 *  true or false and there is no older-control-plane world to gate on. The draft returned `[]`
 *  for `ready === undefined` and §8 named a regression test for it; that test is **deleted**,
 *  not moved, because it asserted a shape the control plane cannot produce. `Worktree.ready`
 *  stays optional in the type only because that safely accepts an always-present boolean.
 *
 *  Already-ready is disabled rather than hidden: the row is a fact about the worktree, and
 *  Phase 2 has no un-mark to offer — the merge queue in Phase 3 owns that. */
export function worktreeReadyItems(worktree: Worktree, close: () => void): MenuItem[] {
  return [
    {
      id: "mark-ready",
      label: worktree.ready === true ? "Ready to merge" : "Mark ready",
      separatorBefore: true,
      run:
        worktree.ready === true
          ? undefined
          : () => {
              close();
              markWorktreeReady(worktree);
            },
      disabledReason:
        worktree.ready === true
          ? "Already marked ready. Phase 2 has no un-mark; the merge queue in Phase 3 owns that."
          : undefined,
    },
  ];
}
