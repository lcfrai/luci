/* ADDED BY TASK 23's FIX ROUND 1, as a DECLARED DEPARTURE from contract §1.5, which keeps the
 * real xterm out of unit tests. The ruling that allows it has one condition: it must be stable.
 * It ran five consecutive times with identical results before it was committed; if it ever
 * flakes, delete this file and keep the handler-level tests, which is what that ruling says.
 *
 * WHY IT EXISTS. The Critical it pins was invisible to every other test in the tree, for one
 * reason: they all dispatched on `window`, and none on the focused element. Real xterm handles
 * keydown on its own helper textarea FIRST, and for any key it maps to data it sends the bytes
 * to the agent and cancels the event, stopPropagation included — so ⌘3 went down the PTY as ESC
 * (the agent's interrupt) and ⌘⏎ as CR (submitting its half-typed prompt), and the keyboard map
 * never saw either. Terminal.test.tsx pins that makeTerm installs a handler and which way it
 * points, against FakeTerm. This file pins the end state the user sees, through the real thing:
 *
 *   - a chord pressed on the focused terminal, with Ctrl or with Meta (fix round 2), reaches the
 *     map, and the agent receives nothing;
 *   - Escape, shift+Tab and Ctrl+C pressed the same way reach the agent, and the map does nothing.
 *
 * §1.5's reason for keeping xterm out still holds for what it names — RENDERING. Nothing here
 * draws or measures a glyph. xterm's key path is plain DOM event handling, which jsdom runs. */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, renderHook } from "@testing-library/react";
import { Terminal } from "../components/Terminal";
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, releaseAttach } from "../api/attachRegistry";
import { FakeWebSocket } from "../test/fakes";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";
import { resetCockpit, useCockpit } from "../store/store";
import type { Session } from "../store/types";
import { __resetOverlays } from "./useDismissable";
import { useKeyboardMap } from "./useKeyboardMap";

// The verbs and the create path are other tasks'; what they do is not the question here, only
// whether the chord reached them. @xterm/xterm is deliberately NOT mocked — that is the point.
const { killMock, createSessionMock } = vi.hoisted(() => ({ killMock: vi.fn(), createSessionMock: vi.fn() }));
vi.mock("../components/develop/sessionVerbs", () => ({
  sessionVerbs: (sessionId: string) => [{ id: "kill", label: "Kill", run: () => killMock(sessionId) }],
}));
vi.mock("./useCreateSession", () => ({ useCreateSession: () => createSessionMock }));

/** Nine sessions, so every digit chord has a session to land on and "the map acted" is a
 *  selection change for all nine, not a no-op past the end that reads the same either way. */
const IDS = Array.from({ length: 9 }, (_, i) => `s-${i + 1}`);

const session = (id: string): Session =>
  aSession({ id, projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "running", attachPath: `/v1/sessions/${id}/attach` });

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  FakeWebSocket.reset();
  __resetAttachRegistry();
  killMock.mockReset();
  createSessionMock.mockReset().mockResolvedValue(undefined);
  configureAttachRegistry({
    client: {
      openAttach: (s: Session) => new FakeWebSocket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => session("s-1"),
    },
  });
  useCockpit.setState({
    projects: [aProject({ id: "p-1" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", isHead: false })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" })] },
    sessions: Object.fromEntries(IDS.map((id) => [id, session(id)])),
    sessionIdsByWorktree: { "w-1": IDS },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
  });
  renderHook(() => useKeyboardMap());
});

afterEach(() => {
  __resetAttachRegistry();
});

/** Mount the real Terminal, focused, and return its helper textarea — which must be the element
 *  holding DOM focus, or nothing below is dispatched where a real keystroke lands — and every
 *  byte xterm hands the agent from here on. */
function focusedTerminal(): { textarea: HTMLTextAreaElement; sent: string[] } {
  render(<Terminal sessionId="s-1" readOnly={false} focused />);
  const handle = acquireAttach(session("s-1"), {
    createTerm: () => { throw new Error("the component should already have built a terminal"); },
    readOnly: false,
  });
  releaseAttach("s-1");
  const sent: string[] = [];
  handle.term.onData((data) => sent.push(data));
  const textarea = handle.element.querySelector<HTMLTextAreaElement>("textarea.xterm-helper-textarea");
  expect(textarea).not.toBeNull();
  expect(document.activeElement).toBe(textarea);
  return { textarea: textarea as HTMLTextAreaElement, sent };
}

/** xterm reads `keyCode`, which KeyboardEventInit cannot set; a real keystroke carries both. */
function keydownOn(target: HTMLElement, key: string, keyCode: number, init: KeyboardEventInit): KeyboardEvent {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  Object.defineProperty(event, "keyCode", { value: keyCode });
  target.dispatchEvent(event);
  return event;
}

interface ChordRow {
  name: string;
  key: string;
  keyCode: number;
  init: KeyboardEventInit;
  /** selection.sessionId before the press. */
  focus: string | null;
  /** What the map doing its job looks like, for this chord. */
  acted: () => boolean;
}

const state = () => useCockpit.getState();
const digit = (n: number): ChordRow => ({
  name: `⌘${n}`, key: `${n}`, keyCode: 48 + n, init: { ctrlKey: true }, focus: null,
  acted: () => state().selection.sessionId === `s-${n}`,
});

/** The six the review's probe watched real xterm consume, and what each sent the agent then. */
const CONSUMED_BEFORE_THE_FIX: ChordRow[] = [
  { name: "⌘K (was \\x0b)", key: "k", keyCode: 75, init: { ctrlKey: true }, focus: null, acted: () => state().paletteOpen },
  { name: "⌘W (was \\x17)", key: "w", keyCode: 87, init: { ctrlKey: true }, focus: "s-2",
    acted: () => JSON.stringify(killMock.mock.calls) === JSON.stringify([["s-2"]]) },
  { name: "⌘B (was \\x02)", key: "b", keyCode: 66, init: { ctrlKey: true }, focus: null, acted: () => state().railCollapsed },
  { ...digit(3), name: "⌘3 (was \\x1b, the agent's interrupt)" },
  { name: "⌘\\ (was \\x1c)", key: "\\", keyCode: 220, init: { ctrlKey: true }, focus: "s-2",
    acted: () => state().selection.sessionId === "s-3" },
  { name: "⌘⏎ (was \\r, submitting the agent's prompt)", key: "Enter", keyCode: 13, init: { ctrlKey: true }, focus: null,
    acted: () => JSON.stringify(createSessionMock.mock.calls) === JSON.stringify([["w-1"]]) },
];

/** The rest of the table. xterm let ⌘1, ⌘2, ⌘9 and ⌘⇧M past already; ⌘4–⌘8 fall under the
 *  same Ctrl-digit mapping as ⌘3. Pinned here so no future key path starts eating them. */
const THE_REST_OF_THE_TABLE: ChordRow[] = [
  digit(1), digit(2), digit(4), digit(5), digit(6), digit(7), digit(8), digit(9),
  { name: "⌘⇧M", key: "M", keyCode: 77, init: { ctrlKey: true, shiftKey: true }, focus: null,
    acted: () => state().machinesOpen },
];

/** ADDED BY TASK 23's FIX ROUND 2: the whole table again, pressed with Meta and with Ctrl
 *  released. §6.9's ⌘ is metaKey || ctrlKey, and every row above presses Ctrl, so a key handler
 *  narrowed to `!(event.ctrlKey && isCockpitChord(event))` passed the suite. Under it, ⌘⏎ on the
 *  Meta key is the row that goes red here: xterm maps Enter to CR whatever the modifiers, so the
 *  agent's half-typed prompt is submitted and no session is created. On a non-Mac platform xterm
 *  maps no other Meta chord in the table to data, so those rows reach the map whatever the
 *  handler says. They are here because each carries its own `acted`, which pins the other half:
 *  a map branch that stopped acting on Meta would go red in its own row. */
const THE_TABLE_ON_THE_META_KEY: ChordRow[] = [...CONSUMED_BEFORE_THE_FIX, ...THE_REST_OF_THE_TABLE].map((row) => ({
  ...row,
  name: `${row.name.split(" (")[0]} on the Meta key`,
  init: { ...row.init, ctrlKey: false, metaKey: true },
}));

describe("a chord pressed on a focused real xterm reaches the keyboard map, and not the agent", () => {
  it.each([...CONSUMED_BEFORE_THE_FIX, ...THE_REST_OF_THE_TABLE, ...THE_TABLE_ON_THE_META_KEY])("$name", (row) => {
    if (row.focus !== null) state().focusSession(row.focus);
    const { textarea, sent } = focusedTerminal();
    keydownOn(textarea, row.key, row.keyCode, row.init);
    expect({ acted: row.acted(), sent }).toEqual({ acted: true, sent: [] });
  });
});

/** The keys §6.9 says the agent owns. Each is sent by xterm on keydown alone, so the bytes are
 *  observable here without synthesising a keypress. The map must do nothing with any of them. */
const THE_AGENTS_KEYS: Array<[string, string, number, KeyboardEventInit, string]> = [
  ["Escape, the agent's interrupt", "Escape", 27, {}, "\x1b"],
  ["shift+Tab, the agent's permission cycle", "Tab", 9, { shiftKey: true }, "\x1b[Z"],
  ["Ctrl+C", "c", 67, { ctrlKey: true }, "\x03"],
];

describe("the agent's own keys still reach the agent through a real xterm", () => {
  it.each(THE_AGENTS_KEYS)("%s", (_name, key, keyCode, init, bytes) => {
    state().focusSession("s-2");
    const { textarea, sent } = focusedTerminal();
    const before = JSON.stringify([state().selection, state().paletteOpen, state().railCollapsed, state().machinesOpen]);
    keydownOn(textarea, key, keyCode, init);
    const after = JSON.stringify([state().selection, state().paletteOpen, state().railCollapsed, state().machinesOpen]);
    expect({ sent, mapUnchanged: after === before, killed: killMock.mock.calls.length, created: createSessionMock.mock.calls.length })
      .toEqual({ sent: [bytes], mapUnchanged: true, killed: 0, created: 0 });
  });
});

/* OWNER DECISION OD-9, "Keep, plus a toggle", through the real xterm. Everything above runs with
 * the setting at its default, off, and is the off half. This is the on half.
 *
 * On, a chord pressed on a focused terminal is the agent's, and it takes TWO guards to make it
 * so, one per kind of chord. A chord xterm maps to data (⌘3 is ESC) is kept by terminalKeepsKey,
 * sent, and cancelled by xterm, so the map never sees it. A chord xterm maps to nothing (⌘1)
 * bubbles to the window whatever the handler says, and only the map's "inside a terminal"
 * return stops it moving focus. So each row asserts the bytes AND that the map did nothing, and
 * the counts at the bottom prove both kinds are present. */
const ALL_CHORD_ROWS: ChordRow[] = [...CONSUMED_BEFORE_THE_FIX, ...THE_REST_OF_THE_TABLE, ...THE_TABLE_ON_THE_META_KEY];

/** What xterm 6 sends for a chord it is allowed to keep, off a Mac. Ctrl-letter is its C0 byte,
 *  Ctrl-3..8 are ESC, FS, GS, RS, US and DEL, Ctrl-\ is FS, and Enter is CR on any modifier.
 *  Ctrl-1, Ctrl-2, Ctrl-9, Ctrl-Shift-M and every other Meta chord map to nothing. */
const CTRL_BYTES: Record<string, string> = {
  k: "\x0b", w: "\x17", b: "\x02", "\\": "\x1c", Enter: "\r",
  "3": "\x1b", "4": "\x1c", "5": "\x1d", "6": "\x1e", "7": "\x1f", "8": "\x7f",
};
function bytesWhenKept(row: ChordRow): string[] {
  if (row.key === "Enter") return ["\r"];
  if (row.init.metaKey === true || row.init.shiftKey === true) return [];
  const bytes = CTRL_BYTES[row.key];
  return bytes === undefined ? [] : [bytes];
}

const mapSnapshot = () =>
  JSON.stringify([state().selection, state().paletteOpen, state().railCollapsed, state().machinesOpen, state().pickerOpen]);

describe("OD-9 on: a chord pressed on a focused real xterm reaches the agent, and not the keyboard map", () => {
  it.each(ALL_CHORD_ROWS)("$name", (row) => {
    state().setSendChordsToTerminal(true);
    if (row.focus !== null) state().focusSession(row.focus);
    const { textarea, sent } = focusedTerminal();
    const before = mapSnapshot();
    const event = keydownOn(textarea, row.key, row.keyCode, row.init);
    const expected = bytesWhenKept(row);
    expect({
      sent,
      mapUnchanged: mapSnapshot() === before,
      killed: killMock.mock.calls.length,
      created: createSessionMock.mock.calls.length,
      // xterm cancels what it sends, which is what keeps ⌘W from closing the window; what it
      // does not send, nobody cancels, because the map ignores it rather than claiming it.
      cancelled: event.defaultPrevented,
    }).toEqual({ sent: expected, mapUnchanged: true, killed: 0, created: 0, cancelled: expected.length > 0 });
  });

  it("covers both kinds of chord: the ones xterm sends, and the ones only the map's return stops", () => {
    const sentKinds = ALL_CHORD_ROWS.map((row) => bytesWhenKept(row).length > 0);
    expect({ rows: ALL_CHORD_ROWS.length, sent: sentKinds.filter(Boolean).length, unsent: sentKinds.filter((k) => !k).length })
      .toEqual({ rows: 30, sent: 12, unsent: 18 });
  });

  it("a terminal built with the setting off obeys it the moment it is turned on, and off again", () => {
    const { textarea, sent } = focusedTerminal();
    state().setSendChordsToTerminal(true);
    keydownOn(textarea, "3", 51, { ctrlKey: true });
    expect({ sent, focus: state().selection.sessionId }).toEqual({ sent: ["\x1b"], focus: null });
    state().setSendChordsToTerminal(false);
    keydownOn(textarea, "3", 51, { ctrlKey: true });
    expect({ sent, focus: state().selection.sessionId }).toEqual({ sent: ["\x1b"], focus: "s-3" });
  });

  it("⌘K pressed outside the terminal still opens the palette, with a terminal on the page", () => {
    const { sent } = focusedTerminal();
    state().setSendChordsToTerminal(true);
    const event = keydownOn(document.body, "k", 75, { ctrlKey: true });
    expect({ open: state().paletteOpen, cancelled: event.defaultPrevented, sent }).toEqual({ open: true, cancelled: true, sent: [] });
  });
});

/** The agent's keys, plus "/", which xterm sends on keydown too. The same bytes, and the same
 *  silence from the map, in both modes: the setting moves no key that is not a chord. */
const THE_AGENTS_KEYS_AND_SLASH: Array<[string, string, number, KeyboardEventInit, string]> = [
  ...THE_AGENTS_KEYS,
  ["/, the agent's slash commands", "/", 191, {}, "/"],
];

describe("OD-9: the agent's own keys behave the same through a real xterm with the setting off and on", () => {
  const cases = THE_AGENTS_KEYS_AND_SLASH.flatMap(([name, key, keyCode, init, bytes]) =>
    [false, true].map((on) => ({ name, key, keyCode, init, bytes, on })));
  it.each(cases)(
    "$name, setting on: $on",
    ({ key, keyCode, init, bytes, on }) => {
      state().setSendChordsToTerminal(on);
      state().focusSession("s-2");
      const { textarea, sent } = focusedTerminal();
      const before = mapSnapshot();
      keydownOn(textarea, key, keyCode, init);
      expect({ sent, mapUnchanged: mapSnapshot() === before, killed: killMock.mock.calls.length, created: createSessionMock.mock.calls.length })
        .toEqual({ sent: [bytes], mapUnchanged: true, killed: 0, created: 0 });
    },
  );
});
