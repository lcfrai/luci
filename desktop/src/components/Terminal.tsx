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
import type { TermLike } from "../api/attachRegistry";
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

function TerminalImpl({ sessionId, readOnly }: { sessionId: SessionId; readOnly: boolean }) {
  const host = useRef<HTMLDivElement>(null);
  const session = useCockpit(selectSession(sessionId));
  const hasSession = session !== undefined;

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
    node.appendChild(handle.element);
    handle.mount();                              // AFTER the append: xterm measures a
    fitAddons.get(handle.term)?.fit();           // rendered element, never a detached one
    handle.focus();

    const onWindowResize = () => fitAddons.get(handle.term)?.fit();
    window.addEventListener("resize", onWindowResize);

    return () => {
      window.removeEventListener("resize", onWindowResize);
      if (handle.element.parentNode === node) node.removeChild(handle.element);
      releaseAttach(sessionId);
    };
  }, [sessionId, hasSession]);

  return <div ref={host} style={{ width: "100%", height: "100%" }} />;
}

/** memo'd per §8 row 1: a parent re-render with unchanged props never reaches the effects. */
export const Terminal = memo(TerminalImpl);
