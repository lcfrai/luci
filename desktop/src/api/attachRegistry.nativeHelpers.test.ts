import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, FLUSH_INTERVAL_MS, getNativeHelperActivity, GRACE_MS, MAX_OUTPUT_BYTES, releaseAttach, setAttachPermission, subscribeAttach, updateAttachSession } from "./attachRegistry";
import { EMPTY_HELPER_ACTIVITY } from "./nativeHelperActivity";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { aSession } from "../test/fixtures";
import { encode, Kind } from "../lib/wire";
import type { Session } from "../store/types";

const session = (over: Partial<Session> = {}) => aSession({ id: "helper-parent", harness: "codex", status: "running", attachPath: "/v1/sessions/helper-parent/attach", executionGeneration: "a".repeat(64), ...over });
const text = JSON.stringify({ type: "item.started", item: { id: "call-1", type: "collab_tool_call", tool: "spawn_agent", sender_thread_id: "parent", receiver_thread_ids: ["child"] } }) + "\n";
const emit = (socket: FakeWebSocket, value = text) => socket.emitBinary(encode(Kind.Stdout, new TextEncoder().encode(value)));
const mount = (current = session()) => {
  const term = new FakeTerm();
  const handle = acquireAttach(current, { createTerm: () => term, readOnly: false });
  const socket = FakeWebSocket.last()!; socket.acceptOpen();
  return { term, handle, socket, current };
};
beforeEach(() => {
  vi.useFakeTimers(); FakeWebSocket.reset(); __resetAttachRegistry();
  configureAttachRegistry({ client: { openAttach: s => new FakeWebSocket(`ws://localhost${s.attachPath}`) as unknown as WebSocket, getSession: async () => session() } });
});
afterEach(() => { __resetAttachRegistry(); vi.useRealTimers(); });

describe("attach helper observation", () => {
  it("subscribes without attaching and shares the single existing byte owner", () => {
    const changed = vi.fn(); const off = subscribeAttach(session().id, changed);
    expect(FakeWebSocket.instances).toHaveLength(0);
    const { current, term } = mount();
    expect(acquireAttach(current, { createTerm: () => { throw new Error("duplicate terminal"); }, readOnly: false }).term).toBe(term);
    expect(FakeWebSocket.instances).toHaveLength(1); off();
  });

  it("observes admitted stdout without modifying raw xterm bytes or keyboard ownership", () => {
    const { socket, current, term } = mount(); emit(socket);
    expect(getNativeHelperActivity(current).calls).toHaveLength(1);
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual([text]);
    const sent = socket.sent.length; term.type("normal input"); expect(socket.sent).toHaveLength(sent + 1);
  });

  it.each([{ authOnly: true }, { harness: "shell" as const }, { executionGeneration: undefined }])("excludes auth, non-Codex and unidentified-allocation streams %#", over => {
    const { socket, current } = mount(session(over)); emit(socket);
    expect(getNativeHelperActivity(current)).toBe(EMPTY_HELPER_ACTIVITY);
  });

  it("rejects oversized output before observation and never parses synthetic exit banners", () => {
    const { socket, current } = mount();
    emit(socket, text + "x".repeat(MAX_OUTPUT_BYTES));
    expect(getNativeHelperActivity(current).calls).toHaveLength(0);
    expect(socket.closedWith?.code).toBe(4008);
    __resetAttachRegistry();
    const fresh = mount(); fresh.socket.emitBinary(encode(Kind.Exit, new Uint8Array()));
    expect(getNativeHelperActivity(fresh.current).calls).toHaveLength(0);
  });

  it.each([{ projectId: "other-project" }, { attachPath: "/v1/sessions/other/attach" }, { executionGeneration: "b".repeat(64) }])("hides old reports synchronously on same-ID scope or generation mismatch %#", over => {
    const { socket, current } = mount(); emit(socket);
    const next = { ...current, ...over }; updateAttachSession(next);
    expect(getNativeHelperActivity(next)).toBe(EMPTY_HELPER_ACTIVITY);
  });

  it("clears permission loss and refuses stale callbacks after a change and return", () => {
    const { socket, current } = mount(); emit(socket);
    const late = socket.onmessage!;
    setAttachPermission(current.id, false); setAttachPermission(current.id, true);
    expect(getNativeHelperActivity(current)).toBe(EMPTY_HELPER_ACTIVITY);
    const data = encode(Kind.Stdout, new TextEncoder().encode(text));
    late({ data: data.buffer as ArrayBuffer });
    expect(getNativeHelperActivity(current)).toBe(EMPTY_HELPER_ACTIVITY);
  });

  it("resets allocation reports and ignores the old socket after replacement", () => {
    const { socket, current } = mount(); emit(socket);
    const next = { ...current, executionGeneration: "b".repeat(64) }; updateAttachSession(next);
    emit(socket);
    expect(getNativeHelperActivity(next)).toBe(EMPTY_HELPER_ACTIVITY);
    const replacement = FakeWebSocket.last()!; replacement.acceptOpen(); emit(replacement);
    expect(getNativeHelperActivity(next).calls).toHaveLength(1);
    expect(getNativeHelperActivity(current)).toBe(EMPTY_HELPER_ACTIVITY);
  });

  it("discloses reconnect gaps and discards a transport's partial line", () => {
    const { socket, current } = mount(); emit(socket); emit(socket, '{"type":'); socket.emitClose(1006);
    expect(getNativeHelperActivity(current).disconnected).toBe(true);
    vi.advanceTimersByTime(250);
    const next = FakeWebSocket.last()!; next.acceptOpen(); emit(next, '"item.completed"}\n');
    expect(getNativeHelperActivity(current).calls).toHaveLength(1);
    expect(getNativeHelperActivity(current).calls[0].phase).toBe("item.started");
    expect(getNativeHelperActivity(current).partial).toBe(true);
  });

  it("destroys observation with its original terminal and does not recreate through a getter", () => {
    const { socket, current, term } = mount(); emit(socket); releaseAttach(current.id);
    vi.advanceTimersByTime(GRACE_MS); emit(socket);
    expect(term.disposed).toBe(true); expect(getNativeHelperActivity(current)).toBe(EMPTY_HELPER_ACTIVITY);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });
});
