import { useRef, useState } from "react";
import type { JSX, PointerEvent } from "react";
import { api } from "../../api";
import type { ApiError } from "../../api/client";
import { classify } from "../../api/problems";
import { useAttachState } from "../../api/attachRegistry";
import { useHeartbeat } from "../../hooks/heartbeat";
import { Terminal } from "../Terminal";
import { SessionPaneFooter, attachLabel } from "./SessionPaneFooter";
import { SessionPaneHeader } from "./SessionPaneHeader";
import { selectSession, selectWorktreeById } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";

/** The exited bar's Restart. `POST /v1/sessions/{id}/restart`, and NOT a create.
 *
 *  `control-plane/internal/api/sessions.go:723` settles which verb this is: restart means
 *  "same config, clean context", **it keeps the session id**, and resume is a different verb
 *  "because conflating the two makes restart lossy". A `createSession` carrying the dead
 *  session's settings is none of the client's four verbs — it produces an unrelated session
 *  that merely looks related and orphans the dead one beside it, which breaks the one thread
 *  the product spine is built on. §6.5's "Restart clean" is this same verb from the menu.
 *
 *  The two 409s are different sentences and must reach the user as written. Both are
 *  `CodeConflict`, so the `classify` branch below carries them verbatim: `sessions.go:733`
 *  "A restart is already in flight for that session." — §3.14's in-flight guard, because two
 *  concurrent restarts both destroy and both create — and `:751` "The worktree this session
 *  was created in no longer resolves.", deliberately a 409 rather than a 404 because a 404
 *  reads as "no such session" and sends the user after the wrong problem. */
async function restartSession(sessionId: SessionId): Promise<void> {
  try {
    const session = await api.restartSession(sessionId);
    // Same id, so this UPDATES the record rather than adding one. The transcript stays and
    // nothing is orphaned.
    useCockpit.getState().upsertSession(session);
  } catch (e) {
    useCockpit.getState().pushToast(sentenceForRestart(e as ApiError));
  }
}

/** The prose is local because its tail names THIS act; the discrimination is `classify` because
 *  §4.2's non-collapse rule is one rule and three copies of it drift. Both 409s land in
 *  `server-said` and arrive as the server wrote them.
 *
 *  LOAD-BEARING SHAPE: declared return type, switch over `ProblemKind`, no `default:`. A fourth
 *  kind must fail this function with TS2366 rather than fall through a catch-all. */
function sentenceForRestart(error: ApiError): string {
  switch (classify(error)) {
    case "control-plane-down":
      return "The control plane is not running, so the session was not restarted.";
    case "backend-down":
      return "Docker is not running, so the sandbox could not be rebuilt.";
    case "server-said":
      return error.message;
  }
}

/** Contract §7.7's connecting line. An overlay SIBLING of `<Terminal/>`, never a conditional
 *  render of it: unmounting the terminal to show a message about the terminal destroys the
 *  thing the message is about, and a dropped socket destroys nothing (§6.6).
 *
 *  ITS OWN COMPONENT, not inlined into SessionPane, and that is the whole point of the shape.
 *  It subscribes to BOTH `useAttachState` and the 1 Hz `useHeartbeat`. Holding those in
 *  SessionPane would re-render the header, the terminal wrapper and the exit bar every second
 *  — the same objection that kept this out of `Terminal`, arriving one component further in.
 *  As a leaf, the tick reaches only the element that displays the number.
 *
 *  The gate is the ATTACH PHASE and never the session status. `ladderStartedAt !== 0` is
 *  defined at `attachRegistry.ts:53` as exactly "this pane is retrying", which covers a first
 *  connect and a reconnect with one condition and self-clears on `onopen`. `failed` needs the
 *  second condition because `giveUp` sets the stamp back to 0 — the ladder is spent, so there
 *  is no elapsed left to render. There is no third condition. */
function AttachOverlay({ sessionId }: { sessionId: SessionId }): JSX.Element | null {
  const attach = useAttachState(sessionId);
  const nowMs = useHeartbeat();

  if (attach.phase !== "failed" && attach.ladderStartedAt === 0) return null;

  // Centred ONLY for `connecting`. It is the only phase whose body is provably empty — first
  // handshake, fresh terminal — so it is the only one that may take the middle of the pane.
  // Every other phase may have a transcript behind it, and covering an hour of output at the
  // moment the user most wants to re-read it would hide the artifact the registry exists to
  // protect. `failed` is the worse case rather than the lesser one: it is terminal, so a scrim
  // there would occlude permanently with nothing coming to clear it.
  const centred = attach.phase === "connecting";
  const failed = attach.phase === "failed";

  return (
    <div
      className={centred ? "pane-attach pane-attach--centre" : "pane-attach pane-attach--corner"}
      data-attach-phase={attach.phase}
      // A live region ONLY when the text is final. The retrying variants change once a second,
      // and a polite live region would read "connecting 1s", "connecting 2s" at a screen-reader
      // user for the whole ladder. `failed` renders once and is the sentence worth announcing.
      role={failed ? "status" : undefined}
    >
      {/* Verbatim, per attachRegistry.ts:63-64 — the reason is a sentence written for a human,
          and every `failed` publish in giveUp() carries one. Until now it reached the user
          only as the footer's `title`, which is hover-only on an 11px strip. */}
      {failed ? attach.reason : attachLabel(attach, nowMs)}
    </div>
  );
}

/** xterm.js renders a hidden `textarea.xterm-helper-textarea` and focuses it; that is the
 *  element a keystroke reaches. Two fallbacks, because the selector is a claim about a
 *  third-party's internals and the pane must still be keyboard-reachable if it changes. */
function focusTerminalIn(host: HTMLElement | null): void {
  if (host === null) return;
  const target =
    host.querySelector<HTMLElement>("textarea.xterm-helper-textarea") ??
    host.querySelector<HTMLElement>("textarea") ??
    host;
  target.focus();
}

export function SessionPane({
  sessionId,
  variant,
}: {
  sessionId: SessionId;
  variant: "column" | "tile";
}): JSX.Element {
  const session = useCockpit(selectSession(sessionId));
  // selectWorktreeById is task 18's (§3.7 + shared-shape §4); this task adds no second
  // one. Its parameter is a required WorktreeId, and `WorktreeId` is a string alias, so
  // the absent case passes "" — no worktree carries that id, the selector returns
  // undefined, and the header renders session.title. Same idiom as EmptyDevelop's
  // selectWorktrees(branch?.id ?? "").
  const worktree = useCockpit(selectWorktreeById(session?.worktreeId ?? ""));
  const focused = useCockpit((s) => s.selection.sessionId === sessionId);
  const focusSession = useCockpit((s) => s.focusSession);
  const bodyRef = useRef<HTMLDivElement>(null);
  const [restarting, setRestarting] = useState(false);

  const onPointerDown = (e: PointerEvent<HTMLElement>): void => {
    focusSession(sessionId);
    const target = e.target as HTMLElement | null;
    if (target === null) return;
    // Exception 1: an interactive control keeps the focus the browser is about to give it.
    if (target.closest("button, a, input, select, textarea") !== null) return;
    // Exception 2: inside the terminal, xterm owns DOM focus and the selection drag.
    if (bodyRef.current !== null && bodyRef.current.contains(target)) return;
    // Firing the compatibility mouse events is pointerdown's DEFAULT ACTION, so preventing
    // it suppresses the mousedown focus fix-up that would otherwise run after this line.
    // Without it the fix-up looks for a focusable ancestor of the pane chrome, finds none
    // — <section> is not focusable — and blurs to <body>, undoing the hand-off §6.8
    // requires. Only on this path: the two returns above deliberately leave the browser's
    // own focus behaviour alone, which is what keeps the ⋯ menu keyboard-drivable.
    e.preventDefault();
    focusTerminalIn(bodyRef.current);
  };

  const onRestart = (): void => {
    // MEASURED EQUIVALENCE, and when it expires. Dropping `|| restarting` survives the suite,
    // because the only path to this handler is a <button> that carries `disabled={restarting}`
    // and a disabled button fires no click. The guard is the second lock on the same door.
    // It stops being equivalent the moment anything reaches onRestart WITHOUT going through
    // that attribute — the button losing `disabled` in a restyle, or a caller invoking this
    // handler directly. Nothing planned does. Task 22's ⋯ menu calls `sessionVerbs.restart`,
    // a separate path that never reaches this function. Task 23's palette is not a trigger for
    // a stronger reason: it exposes NO session verbs at all (`task-23.md:50` — "a palette row
    // that killed a session would be a destructive act one keystroke from a filter change"),
    // and its only `sessionVerbs` caller is `⌘W`, which runs `kill`.
    // Cited twice wrongly before this: first naming the ⋯ menu as a trigger, then naming the
    // palette as a restart caller. Both were right about the conclusion and wrong about the
    // premise. Open the other task's file before writing a sentence about its behaviour.
    if (session === undefined || restarting) return;
    setRestarting(true);
    // The server's own in-flight guard returns 409 for a concurrent restart, so this local
    // one is a courtesy rather than the correctness boundary.
    void restartSession(session.id).finally(() => setRestarting(false));
  };

  // Exception 3. A purge that raced a render: say so, do not throw.
  if (session === undefined) {
    return <section className="pane pane--gone">This session is no longer listed.</section>;
  }

  const className = ["pane", variant === "tile" ? "pane--tile" : "", focused ? "pane--focused" : ""]
    .filter((c) => c !== "")
    .join(" ");

  return (
    <section
      className={className}
      data-variant={variant}
      data-session-id={session.id}
      aria-label={`${worktree?.name ?? session.title} — ${session.status}`}
      onPointerDown={onPointerDown}
    >
      <SessionPaneHeader session={session} worktree={worktree} variant={variant} />
      <div className="pane-body" ref={bodyRef} tabIndex={-1}>
        {/* Unconditional, for every status — known defect 3. readOnly is the ONLY thing
            `exited` changes, and even then the scrollback stays: the transcript is the
            artifact, and a dropped socket destroys nothing. */}
        <Terminal
          sessionId={session.id}
          readOnly={session.status === "exited"}
          focused={focused}
        />
        <AttachOverlay sessionId={session.id} />
      </div>
      {session.status === "exited" ? (
        <div className="pane-exit">
          <span className="pane-exit-why">
            {session.exitReason === "" ? "the process exited" : session.exitReason}
          </span>
          <button
            type="button"
            className="pane-exit-restart"
            onClick={onRestart}
            disabled={restarting}
          >
            Restart
          </button>
        </div>
      ) : null}
      <SessionPaneFooter session={session} />
    </section>
  );
}
