import { useRef, useState } from "react";
import type { JSX, PointerEvent } from "react";
import { api } from "../../api";
import type { ApiError } from "../../api/client";
import { Terminal } from "../Terminal";
import { SessionPaneFooter } from "./SessionPaneFooter";
import { SessionPaneHeader } from "./SessionPaneHeader";
import { toCreateRequest } from "../../store/defaults";
import { selectSession, selectWorktreeById } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionDefaults, SessionId, WorktreeId } from "../../store/types";

/** The one create path in Group D's develop surface: this pane's Restart and EmptyDevelop's
 *  two named actions. It is here rather than in a module of its own because contract §9
 *  fixes this task's file list at seven, and here rather than in EmptyDevelop because both
 *  directions of import have to terminate.
 *
 *  It is the SECOND `api.createSession` call site in the tree and that is deliberate, not an
 *  oversight: task 18's `hooks/useCreateSession` RE-RESOLVES its arguments against the live
 *  defaults, which is right for "＋ New session" and wrong for a Restart — the user chose
 *  this session's harness, model, effort and permission mode once, and a restart that
 *  silently upgrades the model is the class of surprise this product exists to avoid. Task
 *  24's audit counts on there being exactly these two.
 *
 *  Not permanent: it belongs in `sessionVerbs.ts` alongside Resume/Fork/Restart/Kill. No
 *  Phase 2 task moves it — task 22 creates that module but rewrites neither call site —
 *  so whoever moves it leaves a re-export here or updates both call sites together. */
export async function startSessionOnWorktree(
  worktreeId: WorktreeId,
  defaults: SessionDefaults,
): Promise<void> {
  const before = useCockpit.getState();
  const harness = before.harnesses.find((h) => h.id === defaults.harness);
  if (harness === undefined) {
    // Not a crash and not silence: the catalogue has not loaded, or the harness was removed
    // server-side between the render and the click.
    before.pushToast(`The ${defaults.harness} harness is not in the catalogue.`);
    return;
  }

  try {
    // toCreateRequest strips every field this harness does not accept (§3.6); sending
    // `effort` to a shell is a 400 the client could have avoided.
    const session = await api.createSession(toCreateRequest(worktreeId, defaults, harness));
    const after = useCockpit.getState();
    after.upsertSession(session);
    after.focusSession(session.id);
  } catch (e) {
    // Every client method rejects with ApiError and nothing else (§4). These two codes are
    // different sentences and must not be collapsed: the first means our process is down,
    // the second means Docker is. Every other code carries the server's own message —
    // including the sixteenth, non_loopback_publish, whose sandbox is RUNNING.
    const err = e as ApiError;
    const message =
      err.code === "network_unavailable"
        ? "The control plane is not running. Start it, then try again."
        : err.code === "backend_unavailable"
          ? "Docker is not running, so a sandbox cannot be started."
          : err.message;
    useCockpit.getState().pushToast(message);
  }
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
    // It stops being equivalent the moment anything reaches onRestart without going through
    // that attribute: a keyboard verb in task 23's map, task 22's ⋯ menu offering Restart, or
    // the button losing `disabled` in a restyle. Any of those and a double fire creates two
    // sessions on one worktree from one intent.
    if (session === undefined || restarting) return;
    setRestarting(true);
    void startSessionOnWorktree(session.worktreeId, {
      harness: session.harness,
      model: session.model,
      effort: session.effort,
      permissionMode: session.permissionMode,
    }).finally(() => setRestarting(false));
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
        <Terminal sessionId={session.id} readOnly={session.status === "exited"} />
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
