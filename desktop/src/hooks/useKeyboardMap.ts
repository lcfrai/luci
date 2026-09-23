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
//
// All three are kept out by isCockpitChord below, which is ALSO what the terminal consults
// before it handles a key (Terminal.tsx, makeTerm, through terminalKeepsKey). One list, two
// readers: a chord this map claims is a chord xterm lets past, and anything else is xterm's to
// send to the agent.
//
// OWNER DECISION OD-9 ("Keep, plus a toggle") adds ONE setting on top, and it moves no key in
// or out of the list. prefsSlice.sendChordsToTerminal, "Send shortcut keys to the terminal", is
// off by default, and off is everything above: the cockpit wins, as in VS Code. On, a keydown
// that comes from inside a terminal is the agent's, whatever it is: terminalKeepsKey tells xterm
// to keep it, and the map's second return below ignores it when xterm maps it to nothing and it
// bubbles here anyway. The same chord pressed OUTSIDE a terminal is the cockpit's in both modes,
// which is how ⌘K still opens the palette with the setting on. The three keys above are outside
// the list, so xterm keeps them and this map ignores them in both modes, exactly as before.
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

/** §6.9's unshifted chord keys, and its shifted ones (⌘⇧M alone), in the lower case chordKey
 *  yields. Between them these two lists ARE the predicate's claim: it reads no key outside them,
 *  and a test pins isCockpitChord's source so that it cannot start to.
 *
 *  Exported, frozen, for one reader besides isCockpitChord. The keys are still written twice,
 *  here and in the branch chain below, and a key listed here with no branch is one xterm skips
 *  and nothing acts on, lost to both the agent and the cockpit. useKeyboardMap.test.ts reads
 *  these lists and checks every entry, on Ctrl and on Meta, reaches a branch. EDITED BY TASK
 *  23's FIX ROUND 2: before, the only guard sampled the keyboard, and "⌘← claimed, no branch"
 *  fell outside the sample and passed. */
export const CHORD_KEYS: readonly string[] = Object.freeze(["w", "k", "b", "Enter", "\\", "1", "2", "3", "4", "5", "6", "7", "8", "9"]);
export const SHIFTED_CHORD_KEYS: readonly string[] = Object.freeze(["m"]);

/** Shifted letters arrive as "M", not "m", so the table is matched in lower case. */
function chordKey(event: KeyboardEvent): string {
  return event.key.length === 1 ? event.key.toLowerCase() : event.key;
}

/** True for exactly the chords this map handles, and for nothing else.
 *
 *  TWO readers, and that is why it is exported. The map below returns on false. Terminal.tsx
 *  hands `!isCockpitChord(e)` to xterm's attachCustomKeyEventHandler, because xterm handles
 *  keydown on its own focused textarea first and CANCELS what it maps to data: Ctrl-3 is ESC,
 *  which interrupts the agent, and Ctrl-Enter is CR, which submits its prompt, and neither ever
 *  bubbled to this map while a terminal had focus. Returning false there makes xterm skip the
 *  event without cancelling it. So a key added to the two lists above is a key the agent
 *  stops receiving. */
export function isCockpitChord(event: KeyboardEvent): boolean {
  // ⌘ is metaKey || ctrlKey — the app runs on Windows, where the physical key is Ctrl.
  if (!(event.metaKey || event.ctrlKey)) return false;
  // AltGr on several European layouts reports ctrlKey AND altKey; claiming it would eat
  // composed characters the user is typing into the agent.
  if (event.altKey) return false;
  const key = chordKey(event);
  return (event.shiftKey ? SHIFTED_CHORD_KEYS : CHORD_KEYS).includes(key);
}

/** OD-9. The attribute Terminal.tsx puts on the element that hosts an xterm, and the only
 *  thing isInsideTerminal reads. Ours rather than one of xterm's own class names, so that what
 *  counts as "inside a terminal" is decided in this repo and not by an xterm upgrade. */
export const TERMINAL_HOST_ATTRIBUTE = "data-terminal-host";

/** True when a keydown's target sits inside a Terminal's host: in practice, xterm's focused
 *  helper textarea. Anything else, including the palette's input and the document body, is
 *  outside, so a chord pressed there is the cockpit's in both modes. */
export function isInsideTerminal(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest(`[${TERMINAL_HOST_ATTRIBUTE}]`) !== null;
}

/** What makeTerm hands to xterm's attachCustomKeyEventHandler: true means xterm handles the
 *  key, false means it skips the event WITHOUT cancelling it, so it bubbles to the map.
 *
 *  Off (the default), xterm skips exactly the chords, as task 23's fix round 1 made it. On,
 *  xterm keeps everything, so ⌘3 reaches the agent as ESC and ⌘W as ETB. The store is read
 *  HERE, at event time, and not when the terminal is built: the registry builds one terminal
 *  per session and keeps it, so a value captured in makeTerm would ignore every later toggle. */
export function terminalKeepsKey(event: KeyboardEvent): boolean {
  return useCockpit.getState().sendChordsToTerminal || !isCockpitChord(event);
}

export function useKeyboardMap(): void {
  const createSession = useCreateSession();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent): void {
      // Everything that is not a chord leaves here, which is where the three
      // non-interceptions live — and it is the same test the terminal applies. Do not give
      // this line a list of its own again: a copy is equivalent the day it is written and
      // drifts silently after, because xterm reads only the predicate.
      //
      // What pins what, in useKeyboardMap.test.ts (corrected by task 23's fix round 2; this
      // comment used to say the whole-keyboard test notices any drift, and it sees only its
      // own keys). "every key the two lists name ... reaches a branch" reads CHORD_KEYS and
      // SHIFTED_CHORD_KEYS, so a listed key with no branch fails it whatever the key is. "the
      // predicate names no key of its own" pins isCockpitChord and chordKey as source, so a
      // claim written into either, outside the two lists, fails it whatever the key is. The
      // whole-keyboard test presses a SAMPLE of 74 keys with every modifier the predicate
      // reads, and checks behaviour rather than source, for the keys in its sample. The other
      // direction, a branch for a key the predicate does not claim, loses no key: every branch
      // is below this line, so it is dead code, and the chord table's own test for that chord
      // is what fails.
      if (!isCockpitChord(event)) return;

      const state = useCockpit.getState();
      // OD-9, the map's second return. With the setting on, a chord from inside a terminal is
      // the agent's. xterm has already sent and cancelled every chord it maps to data, so what
      // arrives here is one it maps to nothing (⌘1, ⌘2, ⌘9, ⌘⇧M, and the Meta chords off a
      // Mac), and acting on it would move focus out of the pane the user just typed into. No
      // preventDefault either: ignoring means ignoring, and xterm cancelled ⌘W itself (ETB).
      // useKeyboardMap.xterm.test.tsx measures both kinds, row by row.
      if (state.sendChordsToTerminal && isInsideTerminal(event.target)) return;

      const key = chordKey(event);

      // ⌘W FIRST, and preventDefault BEFORE every guard: without it WebView2 closes the
      // window, and "no session is focused" must still swallow the key (§6.9).
      if (key === "w") {
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

      // ⌘K works from anywhere, including an open overlay, and from a focused terminal unless
      // OD-9's setting is on (the return above): then ⌘K inside a terminal is the agent's, and
      // the palette opens from anywhere outside one, the palette's own input included, so it
      // always closes itself. The palette is a peer of the picker and the menus, never a
      // child, so the topmost one closes.
      if (key === "k") {
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
      if (key === "b") {
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
      // it and Escape closes the wrong surface. Step 7's test pins this. Only the SHIFTED m
      // gets this far — isCockpitChord decides shift for every chord, so no branch here does.
      if (key === "m") {
        event.preventDefault();
        state.openMachines();
        return;
      }

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
