import { memo, useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import {
  acquireAttach,
  releaseAttach,
  setAttachReadOnly,
  updateAttachSession,
} from "../api/attachRegistry";
import type { AttachHandle, TermLike } from "../api/attachRegistry";
import { selectSession } from "../store/selectors";
import { useCockpit } from "../store/store";
import type { SessionId } from "../store/types";

/** Checked against src/styles/tokens.css by the greps above; task 14 owns the names. */
const TERM_BG_TOKEN = "--page";
const TERM_FG_TOKEN = "--ink";

/** The fit addon belongs to the terminal, and the terminal outlives this component, so the
 *  pairing cannot live in a ref. Weak, so a disposed terminal takes its addon with it. */
const fitAddons = new WeakMap<TermLike, FitAddon>();

/** No literal hex in this file. If the tokens do not resolve — jsdom, where nothing is
 *  rendered anyway — no theme is passed and xterm keeps its own defaults. */
function themeFromTokens(): { background: string; foreground: string } | undefined {
  const styles = getComputedStyle(document.documentElement);
  const background = styles.getPropertyValue(TERM_BG_TOKEN).trim();
  const foreground = styles.getPropertyValue(TERM_FG_TOKEN).trim();
  if (background === "" || foreground === "") return undefined;
  return { background, foreground };
}

/** Called by the registry at most once per session id. */
function makeTerm(): TermLike {
  const term = new XTerm({
    fontFamily: '"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace',
    fontSize: 12,
    theme: themeFromTokens(),
    cursorBlink: true,
    // §7.2. xterm's default is 1 000 lines, which makes scrolling up not worth doing — and
    // the pinned-scroll rule only pays for itself if there is something up there to read.
    scrollback: 10_000,
    // Let the agent CLI own the screen; we never reinterpret its output.
    convertEol: false,
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  fitAddons.set(term, fit);
  return term;
}

/** EDITED BY TASK 20's FIX ROUND 1, as a declared departure — task 19 is closed and this file
 *  is its own. The change is the `focused` prop and the condition it gates, and nothing else.
 *
 *  WHY. `handle.focus()` below used to run unconditionally on mount. React flushes passive
 *  effects children-before-parents in mount order, so opening Columns with nine panes left the
 *  NINTH terminal's helper textarea holding DOM focus, while the `pane--focused` ring follows
 *  `selection.sessionId`, which is null on a cold open. No ring anywhere and keystrokes going
 *  to the last pane in server order — the exact symptom §6.8's three-owner split exists to
 *  prevent. A prop rather than a caller-side fix because mount order then stops mattering:
 *  pane 9 simply sees `focused === false`. §6.8 is unchanged — `uiSlice.focusSession` still
 *  holds WHICH pane is focused; this file is merely told whether it is the one. */
function TerminalImpl({
  sessionId,
  readOnly,
  focused,
}: {
  sessionId: SessionId;
  readOnly: boolean;
  focused: boolean;
}) {
  const host = useRef<HTMLDivElement>(null);
  const session = useCockpit(selectSession(sessionId));
  const hasSession = session !== undefined;
  /** The handle, so the focus effect below can reach it. `focused` must NOT join the mount
   *  effect's dependency array — a focus change would re-acquire and re-append the terminal,
   *  which is known defect 1 — so the two cannot be one effect. */
  const handleRef = useRef<AttachHandle | null>(null);

  // Two pushes into the registry. Neither acquires, so neither can re-create anything.
  useEffect(() => {
    if (session) updateAttachSession(session);
  }, [session]);
  useEffect(() => {
    setAttachReadOnly(sessionId, readOnly);
  }, [sessionId, readOnly]);

  useEffect(() => {
    const node = host.current;
    // Read imperatively: a dependency on the session object would re-run this effect on every
    // 2 s poll, which is known defect 1 wearing a component's clothes.
    const current = useCockpit.getState().sessions[sessionId];
    if (!node || !current) return;

    const handle = acquireAttach(current, { createTerm: makeTerm, readOnly });
    handleRef.current = handle;
    node.appendChild(handle.element);
    handle.mount();                              // AFTER the append: xterm measures a
    fitAddons.get(handle.term)?.fit();           // rendered element, never a detached one

    const onWindowResize = () => fitAddons.get(handle.term)?.fit();
    window.addEventListener("resize", onWindowResize);

    return () => {
      window.removeEventListener("resize", onWindowResize);
      if (handle.element.parentNode === node) node.removeChild(handle.element);
      handleRef.current = null;
      releaseAttach(sessionId);
    };
  }, [sessionId, hasSession]);

  // The condition. Runs after the effect above on mount, because effects fire in declaration
  // order, so the handle is already in the ref. Covers BOTH halves with one mechanism: a pane
  // that mounts focused takes the caret, and a pane focused later — a rail-row click, ⌘1-9,
  // the palette — takes it on the false→true transition.
  useEffect(() => {
    if (focused) handleRef.current?.focus();
  }, [focused, sessionId]);

  return <div ref={host} style={{ width: "100%", height: "100%" }} />;
}

/** memo'd per §8 row 1: a parent re-render with unchanged props never reaches the effects.
 *  No custom comparator, deliberately — the default shallow compare covers every prop, so
 *  `focused` is included by construction and a fourth prop would be too. A hand-written
 *  comparator listing `sessionId` and `readOnly` is exactly how `focused` would have been
 *  dropped in silence. */
export const Terminal = memo(TerminalImpl);
