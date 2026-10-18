import { describe, expect, it } from "vitest";
import { EMPTY_HELPER_ACTIVITY, HELPER_CALLS, HELPER_FEED_BYTES, HELPER_FEED_PARSES, HELPER_LINE_BYTES, NativeHelperActivityCapture } from "./nativeHelperActivity";

const bytes = (text: string) => new TextEncoder().encode(text);
const report = (over: Record<string, unknown> = {}, phase = "item.started") => JSON.stringify({
  type: phase, item: { id: "call-1", type: "collab_tool_call", tool: "spawn_agent",
    sender_thread_id: "parent", receiver_thread_ids: ["child"], ...over },
}) + "\n";

describe("native helper reports", () => {
  it("frames split UTF-8 and CRLF without retaining prompts or messages", () => {
    const capture = new NativeHelperActivityCapture();
    const input = bytes(report({ prompt: "private é", agents_states: { child: { status: "running", message: "secret" } } }).replace("\n", "\r\n"));
    const split = input.indexOf(0xc3) + 1;
    capture.feed(input.slice(0, split));
    expect(capture.snapshot).toBe(EMPTY_HELPER_ACTIVITY);
    capture.feed(input.slice(split));
    expect(capture.snapshot.calls[0]).toEqual({ id: "call-1", tool: "spawn_agent", phase: "item.started", sender: "parent", receivers: ["child"], states: [{ id: "child", status: "running" }] });
    expect(JSON.stringify(capture.snapshot)).not.toMatch(/private|secret|prompt|message/);
    expect(Object.isFrozen(capture.snapshot.calls[0].states[0])).toBe(true);
  });

  it.each(["spawn_agent", "send_input", "wait", "close_agent"])("retains only the allowlisted reported action %s", tool => {
    const capture = new NativeHelperActivityCapture(); capture.feed(bytes(report({ tool })));
    expect(capture.snapshot.calls[0].tool).toBe(tool);
  });

  it("ignores ordinary output and unrelated events without creating a report", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes('plain terminal\n{"type":"thread.started"}\n'));
    expect(capture.snapshot).toBe(EMPTY_HELPER_ACTIVITY);
  });

  it.each([
    { tool: "interrupt_agent" }, { id: "\u001bhidden" }, { sender_thread_id: "x".repeat(129) },
    { receiver_thread_ids: Array.from({ length: 9 }, (_, i) => `id-${i}`) },
    { receiver_thread_ids: ["same", "same"] }, { agents_states: { child: { status: "success" } } },
    { agents_states: Object.fromEntries(Array.from({ length: 9 }, (_, i) => [`id-${i}`, { status: "running" }])) },
  ])("refuses malformed or excessive report fields %#", fields => {
    const capture = new NativeHelperActivityCapture(); capture.feed(bytes(report(fields)));
    expect(capture.snapshot.calls).toHaveLength(0); expect(capture.snapshot.partial).toBe(true);
  });

  it("bounds a line and resynchronizes only at a later LF", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes("x".repeat(HELPER_LINE_BYTES + 1)));
    capture.feed(bytes("ignored tail\n" + report()));
    expect(capture.snapshot.calls).toHaveLength(1); expect(capture.snapshot.partial).toBe(true);
  });

  it("refuses invalid UTF-8 and malformed JSON without contaminating later reports", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(new Uint8Array([123, 0xff, 10])); capture.feed(bytes('{broken\n' + report()));
    expect(capture.snapshot.calls).toHaveLength(1); expect(capture.snapshot.partial).toBe(true);
  });

  it("caps parsing work per feed and never joins the skipped tail", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes('{}\n'.repeat(HELPER_FEED_PARSES) + report()));
    expect(capture.snapshot.calls).toHaveLength(0);
    capture.feed(bytes("discarded\n" + report()));
    expect(capture.snapshot.calls).toHaveLength(1); expect(capture.snapshot.partial).toBe(true);
  });

  it("caps observed bytes per feed while preserving later resynchronization", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes("x".repeat(HELPER_FEED_BYTES) + report()));
    expect(capture.snapshot.calls).toHaveLength(0);
    capture.feed(bytes("\n" + report()));
    expect(capture.snapshot.calls).toHaveLength(1); expect(capture.snapshot.partial).toBe(true);
  });

  it("deduplicates replay without regressing a completed reported phase", () => {
    const capture = new NativeHelperActivityCapture(); capture.feed(bytes(report({}, "item.completed")));
    const before = capture.snapshot;
    capture.feed(bytes(report({}, "item.completed") + report()));
    expect(capture.snapshot).toBe(before);
    capture.feed(bytes(report({ sender_thread_id: "impostor" })));
    expect(capture.snapshot.calls[0].sender).toBe("parent"); expect(capture.snapshot.partial).toBe(true);
  });

  it("updates a started report to its reported completed phase without interpreting call status", () => {
    const capture = new NativeHelperActivityCapture(); capture.feed(bytes(report()));
    capture.feed(bytes(report({ status: "not-a-trusted-success" }, "item.completed")));
    expect(capture.snapshot.calls).toHaveLength(1);
    expect(capture.snapshot.calls[0].phase).toBe("item.completed");
    expect(JSON.stringify(capture.snapshot)).not.toContain("not-a-trusted-success");
  });

  it("discovers a spawn receiver once on completion and retains the exact identity through replay", () => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes(report({ receiver_thread_ids: [] })));
    expect(capture.snapshot.calls[0].receivers).toEqual([]);
    const completed = report({ agents_states: { child: { status: "running" } } }, "item.completed");
    capture.feed(bytes(completed));
    expect(capture.snapshot.calls[0].phase).toBe("item.completed");
    expect(capture.snapshot.calls[0].receivers).toEqual(["child"]);
    expect(capture.snapshot.partial).toBe(false);
    const before = capture.snapshot;
    capture.feed(bytes(completed));
    expect(capture.snapshot).toBe(before);
    capture.feed(bytes(report({ receiver_thread_ids: [] })));
    expect(capture.snapshot).toBe(before);
    expect(capture.snapshot.calls[0].phase).toBe("item.completed");
    expect(capture.snapshot.calls[0].receivers).toEqual(["child"]);
    capture.feed(bytes(report({ receiver_thread_ids: ["different-child"] }, "item.completed")));
    expect(capture.snapshot.calls[0].receivers).toEqual(["child"]);
    expect(capture.snapshot.partial).toBe(true);
  });

  it.each([
    { tool: "wait", receiver_thread_ids: ["child"] },
    { tool: "spawn_agent", receiver_thread_ids: ["child", "second-child"] },
    { tool: "spawn_agent", receiver_thread_ids: ["child"], sender_thread_id: "different-parent" },
  ])("refuses receiver discovery outside the exact spawn transition %#", fields => {
    const capture = new NativeHelperActivityCapture();
    capture.feed(bytes(report({ tool: fields.tool, receiver_thread_ids: [] })));
    capture.feed(bytes(report(fields, "item.completed")));
    expect(capture.snapshot.calls[0].phase).toBe("item.started");
    expect(capture.snapshot.calls[0].receivers).toEqual([]);
    expect(capture.snapshot.partial).toBe(true);
  });

  it("retains a bounded immutable window and records loss instead of lifetime counts", () => {
    const capture = new NativeHelperActivityCapture();
    for (let i = 0; i <= HELPER_CALLS; i++) capture.feed(bytes(report({ id: `call-${i}` })));
    expect(capture.snapshot.calls).toHaveLength(HELPER_CALLS);
    expect(capture.snapshot.calls[0].id).toBe("call-1"); expect(capture.snapshot.partial).toBe(true);
    expect(Object.isFrozen(capture.snapshot.calls)).toBe(true);
  });

  it("discards a partial transport line and clears all observations on owner reset", () => {
    const capture = new NativeHelperActivityCapture(); capture.feed(bytes(report()));
    capture.feed(bytes('{"type":')); capture.gap(); capture.feed(bytes('"item.completed"}\n'));
    expect(capture.snapshot.calls[0].phase).toBe("item.started"); expect(capture.snapshot.disconnected).toBe(true);
    capture.clear(); expect(capture.snapshot).toBe(EMPTY_HELPER_ACTIVITY);
  });
});
