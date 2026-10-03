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
import {
  TERMINAL_HOST_ATTRIBUTE,
  terminalKeepsKey,
} from "../hooks/useKeyboardMap";
import { selectSession } from "../store/selectors";
import { useCockpit } from "../store/store";
import type { SessionId } from "../store/types";
import { getAppearance, useAppearance } from "../preferences/appearance";

/** Checked against src/styles/tokens.css by the greps above; task 14 owns the names. */
const TERM_BG_TOKEN = "--page";
const TERM_FG_TOKEN = "--ink";

/** The fit addon belongs to the terminal, and the terminal outlives this component, so the
 *  pairing cannot live in a ref. Weak, so a disposed terminal takes its addon with it. */
const fitAddons = new WeakMap<TermLike, FitAddon>();
const terminals = new WeakMap<TermLike, XTerm>();

/** No literal hex in this file. If the tokens do not resolve — jsdom, where nothing is
 *  rendered anyway — no theme is passed and xterm keeps its own defaults. */
function themeFromTokens():
  { background: string; foreground: string } | undefined {
  const styles = getComputedStyle(document.documentElement);
  // xterm accepts CSS hex/rgb colours, not every browser-supported colour space.
  // Resolve tokens through a canvas so OKLCH themes become concrete sRGB values.
  const resolve = (token: string) => {
    const value = styles.getPropertyValue(token).trim();
    if (!value) return "";
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 1;
    const context = canvas.getContext("2d");
    if (!context) return value;
    context.fillStyle = value;
    context.fillRect(0, 0, 1, 1);
    const [r, g, b] = context.getImageData(0, 0, 1, 1).data;
    return `rgb(${r}, ${g}, ${b})`;
  };
  const background = resolve(TERM_BG_TOKEN);
  const foreground = resolve(TERM_FG_TOKEN);
  if (background === "" || foreground === "") return undefined;
  return { background, foreground };
}

/** Called by the registry at most once per session id. */
function makeTerm(): TermLike {
  const term = new XTerm({
    fontFamily: '"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace',
    fontSize: getAppearance().terminalFontSize,
    theme: themeFromTokens(),
    cursorBlink: true,
    // §7.2. xterm's default is 1 000 lines, which makes scrolling up not worth doing — and
    // the pinned-scroll rule only pays for itself if there is something up there to read.
    scrollback: 10_000,
    // Let the agent CLI own the screen; we never reinterpret its output.
    convertEol: false,
  });
  /* EDITED BY TASK 23's FIX ROUND 1, as a declared departure — task 19 is closed and this file
   * is its own. The change is this one call, and nothing else.
   *
   * WHY. xterm handles keydown on its own focused helper textarea, and for any key it maps to
   * data it sends the bytes to the agent and CANCELS the event, stopPropagation included. So
   * with a terminal focused — the state the cockpit lives in — six of useKeyboardMap's chords
   * never reached its window listener and went to the PTY as control bytes instead: ⌘3 as ESC,
   * which interrupts the agent, and ⌘⏎ as CR, which submits its half-typed prompt. Returning
   * false makes xterm skip the event WITHOUT cancelling it, so it bubbles to the map.
   *
   * The predicate is the map's own, so the two cannot disagree about what a chord is. Escape,
   * shift+Tab, "/", AltGr and every bare key fall outside it and still reach the agent (§6.9).
   *
   * EDITED FOR OWNER DECISION OD-9. The handler is terminalKeepsKey, which is `!isCockpitChord`
   * while "Send shortcut keys to the terminal" is off, and true for every key while it is on.
   * It reads the setting per keystroke, because this terminal is built once and outlives any
   * toggle. The host div below carries TERMINAL_HOST_ATTRIBUTE, which is how the map knows a
   * keydown came from in here. */
  term.attachCustomKeyEventHandler(terminalKeepsKey);
  const fit = new FitAddon();
  term.loadAddon(fit);
  fitAddons.set(term, fit);
  terminals.set(term, term);
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
  const appearance = useAppearance();
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
    handle.mount(); // AFTER the append: xterm measures a
    fitAddons.get(handle.term)?.fit(); // rendered element, never a detached one

    /* EDITED BY TASK 21's FIX ROUND 1, as a declared departure — task 19 is closed and this file
     * is its own. The change is the resize trigger, and nothing else.
     *
     * WHY. A `window` resize listener was the ONLY refit trigger in the tree, so a Canvas tile
     * dragged by its corner grew its pixel box while xterm kept its old cols and rows until the
     * user happened to resize the window — a letterboxed grid and a PTY never told. Nothing
     * objected, because nothing pinned that listener either: the refit path was unpinned before
     * this change, so the defect was not a wrong listener but an absent objection to no listener
     * at all. Task 14's `test/setup.ts` already reads "the fit addon and the Canvas tiles both
     * construct one", so this is the wiring that was planned for and then not made.
     *
     * The observer SUBSUMES the window listener rather than sitting beside it: a host in a flex
     * or grid layout changes box when the window does, and where it does not, cols and rows
     * would not have changed anyway. So the listener is gone, not kept.
     *
     * Observe `node`, NOT `handle.element`. The registry owns that element, it outlives this
     * component, and it is RE-PARENTED between Columns and Canvas — an observer on it would have
     * a different lifetime from the observer's owner. `node` lives exactly as long as this
     * effect, which is the observer's life.
     *
     * No debounce, and it is self-limiting rather than merely cheap: attachRegistry sends a
     * resize frame from `term.onResize`, and xterm fires that only when cols or rows ACTUALLY
     * change, so a fit per layout frame is at most one frame per dimension change.
     *
     * `observe()` fires once immediately with the initial box, so `fit()` runs twice at mount.
     * The second is a no-op at the wire for the same reason. Do NOT delete the explicit fit
     * above as redundant: it is ordered against the append, which is not the observer's job. */
    const ro = new ResizeObserver(() => fitAddons.get(handle.term)?.fit());
    ro.observe(node);

    return () => {
      ro.disconnect();
      if (handle.element.parentNode === node) node.removeChild(handle.element);
      handleRef.current = null;
      releaseAttach(sessionId);
    };
  }, [sessionId, hasSession]);

  useEffect(() => {
    const handle = handleRef.current;
    const term = handle && terminals.get(handle.term);
    if (!handle || !term?.options) return;
    term.options.fontSize = appearance.terminalFontSize;
    term.options.theme = themeFromTokens();
    fitAddons.get(handle.term)?.fit();
  }, [appearance, sessionId, hasSession]);

  // The condition. Runs after the effect above on mount, because effects fire in declaration
  // order, so the handle is already in the ref. Covers BOTH halves with one mechanism: a pane
  // that mounts focused takes the caret, and a pane focused later — a rail-row click, ⌘1-9,
  // the palette — takes it on the false→true transition.
  useEffect(() => {
    if (focused) handleRef.current?.focus();
  }, [focused, sessionId]);

  // OD-9: the marker useKeyboardMap's isInsideTerminal looks for. Spread from the constant, so
  // the attribute is named once.
  return (
    <div
      ref={host}
      {...{ [TERMINAL_HOST_ATTRIBUTE]: "" }}
      style={{ width: "100%", height: "100%" }}
    />
  );
}

/** memo'd per §8 row 1: a parent re-render with unchanged props never reaches the effects.
 *  No custom comparator, deliberately — the default shallow compare covers every prop, so
 *  `focused` is included by construction and a fourth prop would be too. A hand-written
 *  comparator listing `sessionId` and `readOnly` is exactly how `focused` would have been
 *  dropped in silence. */
export const Terminal = memo(TerminalImpl);
