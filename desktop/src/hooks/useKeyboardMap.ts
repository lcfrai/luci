// Contract §6.9 (the chord table) and §6.8 ("the keydown listener" row): this is the ONE
// window keydown listener in the application, mounted once by AppShell. No component adds
// another.
//
// THREE KEYS ARE DELIBERATELY NOT HANDLED HERE. Do not "fix" them:
//   • shift+tab — Claude Code cycles its own agent-side permission state on it, inside its
//     own TUI. A cockpit that intercepted it, wrote that field in the store and told the
//     agent nothing would desynchronise the two permanently. That is known defect 8 (§8).
//     This file names neither the key nor the field on purpose: task 24's defect suite
//     strips comments and asserts that NEITHER string appears in what is left, which is a
//     cheap mechanical proof that no branch here can be reading or writing either one.
//   • /        — slash commands belong to the agent CLI. Eating the key makes /compact
//     untypeable.
//   • Escape   — task 18's useDismissable owns it, as a capture-phase listener attached only
//     while an overlay is open. With nothing open there is no listener, xterm sends \x1b down
//     the PTY and the agent interrupts itself (§6.9). This hook READS hasOpenOverlay() and
//     CALLS closeTopOverlay(); it never re-derives either.
import { useEffect } from "react";
import { sessionVerbs } from "../components/develop/sessionVerbs";
import { useCreateSession } from "./useCreateSession";
import { closeTopOverlay, hasOpenOverlay } from "./useDismissable";
import { idAtVisiblePosition } from "../lib/immutable";
import { selectPaneSessionIds, selectTargetWorktreeId } from "../store/selectors";
import { useCockpit } from "../store/store";

/** Two of task 22's three VerbDeps are unreachable from a keyboard chord: there is no host
 *  menu to close and no rename input to arm. They are no-ops rather than throws, because the
 *  one verb this file runs (Kill) calls deps.close() before it commits anything. */
const NOOP = (): void => {};

export function useKeyboardMap(): void {
  const createSession = useCreateSession();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent): void {
      // ⌘ is metaKey || ctrlKey — the app runs on Windows, where the physical key is Ctrl.
      // Everything without it leaves here, which is where the three non-interceptions live.
      if (!(event.metaKey || event.ctrlKey)) return;
      // AltGr on several European layouts reports ctrlKey AND altKey; hijacking it would eat
      // composed characters the user is typing into the agent.
      if (event.altKey) return;

      // Shifted letters arrive as "M", not "m", so the table is matched in lower case.
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      const state = useCockpit.getState();

      // ⌘W FIRST, and preventDefault BEFORE every guard: without it WebView2 closes the
      // window, and "no session is focused" must still swallow the key (§6.9).
      if (key === "w" && !event.shiftKey) {
        event.preventDefault();
        if (hasOpenOverlay()) return;
        const focused = state.selection.sessionId;
        if (focused === null) return;
        // Optimistic kill plus the Undo window: task 22's, not reimplemented here. The verb
        // list is a plain function over a store snapshot, so it is built here in the handler
        // rather than at render. `run` is undefined on a session that has already exited —
        // task 22 gates Kill on `alive` — so `?.run?.()` is an honest no-op and NOT a second
        // gate written here, which would be a second answer to the same question.
        sessionVerbs(focused, { createSession, close: NOOP, beginRename: NOOP })
          .find((verb) => verb.id === "kill")
          ?.run?.();
        return;
      }

      // ⌘K works from anywhere, including a focused terminal and an open overlay. The palette
      // is a peer of the picker and the menus, never a child, so the topmost one closes.
      if (key === "k" && !event.shiftKey) {
        event.preventDefault();
        if (state.paletteOpen) {
          state.closePalette();
          return;
        }
        // ONCE, not in a loop. closeTopOverlay() calls close() and returns true; the stack is
        // popped by useDismissable's effect cleanup, i.e. after React unmounts the overlay,
        // i.e. after this handler returns. A loop would therefore call close() repeatedly on
        // the same entry and never reach a second one — it would read as draining and drain
        // nothing. A second overlay under this one stays open and its own Escape still works.
        closeTopOverlay();
        state.openPalette();
        return;
      }

      // §6.9 marks ⌘B "Always", and it can be honoured literally: the rail is beside the
      // palette, not behind it, so toggling it while an overlay is open changes something the
      // user can see. This branch is therefore ABOVE the overlay bail-out.
      if (key === "b" && !event.shiftKey) {
        event.preventDefault();
        state.toggleRail();
        return;
      }

      // Every remaining chord acts on the surface an overlay COVERS, so an open overlay takes
      // them: the palette owns its own arrows and Enter, and the picker owns its own fields.
      if (hasOpenOverlay()) return;

      // DECLARED DEPARTURE from §6.9, which marks ⌘⇧M "Always". It is dropped while an
      // overlay is open because openMachines() opens a SECOND overlay and .pal-scrim is
      // z-index 60 over the whole viewport: obeying the table literally would put the
      // machines popover under the palette's backdrop, where the user cannot see or click
      // it and Escape closes the wrong surface. Step 7's test pins this.
      if (key === "m" && event.shiftKey) {
        event.preventDefault();
        state.openMachines();
        return;
      }
      if (event.shiftKey) return;

      if (key === "Enter") {
        event.preventDefault();
        // "Requires a selected branch; falls back to the branch's first worktree" (§6.9) is
        // one selector, shared with the palette's New-session row.
        const worktreeId = selectTargetWorktreeId(state);
        if (worktreeId === null) return;
        void createSession(worktreeId);
        return;
      }

      if (key === "\\") {
        event.preventDefault();
        const ids = selectPaneSessionIds(state);
        if (ids.length === 0) return;
        const focused = state.selection.sessionId;
        // 1-based, and 0 when the focused session is not in the pane list — both land on the
        // first session, which is the only sensible "next" from outside the list.
        const position = focused === null ? 0 : ids.indexOf(focused) + 1;
        const next = idAtVisiblePosition(ids, position >= ids.length ? 1 : position + 1);
        if (next !== null) state.focusSession(next);
        return;
      }

      if (/^[1-9]$/.test(key)) {
        event.preventDefault();
        // §3.9's rule: never index an array across a filter boundary, map the position to an
        // id. Past the end this returns null and the chord is a no-op, per §6.9.
        const id = idAtVisiblePosition(selectPaneSessionIds(state), Number(key));
        if (id !== null) state.focusSession(id);
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [createSession]);
}
