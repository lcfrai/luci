/** A bounded, untrusted projection of reports in a parent terminal, never task authority. */
export const HELPER_LINE_BYTES = 64 * 1024;
export const HELPER_FEED_BYTES = 256 * 1024;
export const HELPER_FEED_PARSES = 128;
export const HELPER_CALLS = 64;
export const HELPER_IDS = 8;
const TOOLS = ["spawn_agent", "send_input", "wait", "close_agent"] as const;
const STATES = ["pending_init", "running", "interrupted", "completed", "errored", "shutdown", "not_found"] as const;
type Tool = typeof TOOLS[number];
type AgentState = typeof STATES[number];
export interface ReportedHelperCall {
  readonly id: string;
  readonly tool: Tool;
  readonly phase: "item.started" | "item.completed";
  readonly sender: string;
  readonly receivers: readonly string[];
  readonly states: readonly Readonly<{ id: string; status: AgentState }>[];
}
export interface NativeHelperActivity {
  readonly calls: readonly ReportedHelperCall[];
  readonly partial: boolean;
  readonly disconnected: boolean;
}
export const EMPTY_HELPER_ACTIVITY: NativeHelperActivity = Object.freeze({
  calls: Object.freeze([]), partial: false, disconnected: false,
});
const object = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);
const id = (value: unknown): value is string =>
  typeof value === "string" && /^[\x21-\x7e]{1,128}$/.test(value);

export class NativeHelperActivityCapture {
  private line = new Uint8Array(HELPER_LINE_BYTES);
  private length = 0;
  private discarding = false;
  private value = EMPTY_HELPER_ACTIVITY;
  get snapshot(): NativeHelperActivity { return this.value; }

  clear(): void {
    this.length = 0;
    this.discarding = false;
    this.value = EMPTY_HELPER_ACTIVITY;
  }

  /** A transport gap cannot splice a partial old line into a new/replayed stream. */
  gap(): void {
    this.length = 0;
    this.discarding = false;
    this.set(this.value.calls, true, true);
  }

  feed(bytes: Uint8Array): void {
    let parses = 0;
    const end = Math.min(bytes.length, HELPER_FEED_BYTES);
    for (let offset = 0; offset < end; offset++) {
      const byte = bytes[offset];
      if (byte === 10) {
        if (!this.discarding && this.length > 0) {
          if (parses === HELPER_FEED_PARSES) {
            this.set(this.value.calls, true, this.value.disconnected);
            this.length = 0;
            this.discarding = true;
            return; // unexamined tail is never joined to the next feed
          }
          parses++;
          this.parseLine();
        }
        this.length = 0;
        this.discarding = false;
      } else if (!this.discarding) {
        if (this.length === HELPER_LINE_BYTES) {
          this.length = 0;
          this.discarding = true;
          this.set(this.value.calls, true, this.value.disconnected);
        } else this.line[this.length++] = byte;
      }
    }
    if (end !== bytes.length) {
      this.length = 0;
      this.discarding = true;
      this.set(this.value.calls, true, this.value.disconnected);
    }
  }

  private parseLine(): void {
    let event: unknown;
    try {
      const end = this.length - (this.line[this.length - 1] === 13 ? 1 : 0);
      const text = new TextDecoder("utf-8", { fatal: true }).decode(this.line.subarray(0, end));
      // Ordinary terminal output is not a malformed helper report.
      if (!text.startsWith("{")) return;
      event = JSON.parse(text);
    } catch {
      this.set(this.value.calls, true, this.value.disconnected);
      return;
    }
    if (!object(event) || (event.type !== "item.started" && event.type !== "item.completed") ||
        !object(event.item) || event.item.type !== "collab_tool_call") return;
    const item = event.item;
    if (!id(item.id) || !id(item.sender_thread_id) ||
        !TOOLS.includes(item.tool as Tool) || !Array.isArray(item.receiver_thread_ids) ||
        item.receiver_thread_ids.length > HELPER_IDS || !item.receiver_thread_ids.every(id) ||
        new Set(item.receiver_thread_ids).size !== item.receiver_thread_ids.length) {
      this.set(this.value.calls, true, this.value.disconnected);
      return;
    }
    const states: { id: string; status: AgentState }[] = [];
    if (item.agents_states !== undefined) {
      if (!object(item.agents_states)) { this.set(this.value.calls, true, this.value.disconnected); return; }
      // Read at most nine keys: a ninth refuses the entire report, without a large retained list.
      for (const key in item.agents_states) {
        if (!Object.prototype.hasOwnProperty.call(item.agents_states, key)) continue;
        const state = item.agents_states[key];
        if (states.length === HELPER_IDS || !id(key) || !object(state) || !STATES.includes(state.status as AgentState)) {
          this.set(this.value.calls, true, this.value.disconnected); return;
        }
        states.push({ id: key, status: state.status as AgentState });
      }
    }
    const next: ReportedHelperCall = Object.freeze({ id: item.id, tool: item.tool as Tool,
      phase: event.type, sender: item.sender_thread_id,
      receivers: Object.freeze([...item.receiver_thread_ids] as string[]),
      states: Object.freeze(states.map(state => Object.freeze(state))),
    });
    const calls = this.value.calls;
    const previous = calls.find(call => call.id === next.id);
    if (previous) {
      if (previous.tool !== next.tool || previous.sender !== next.sender ||
          JSON.stringify(previous.receivers) !== JSON.stringify(next.receivers)) {
        this.set(calls, true, this.value.disconnected); return;
      }
      // Replays do not regress a reported completed phase or inflate the bounded window.
      if (previous.phase === "item.completed" && next.phase === "item.started") return;
      if (JSON.stringify(previous) === JSON.stringify(next)) return;
      this.set(calls.map(call => call.id === next.id ? next : call), this.value.partial, this.value.disconnected);
    } else {
      this.set([...calls.slice(-(HELPER_CALLS - 1)), next], this.value.partial || calls.length === HELPER_CALLS, this.value.disconnected);
    }
  }

  private set(calls: readonly ReportedHelperCall[], partial: boolean, disconnected: boolean): void {
    if (calls === this.value.calls && partial === this.value.partial && disconnected === this.value.disconnected) return;
    this.value = Object.freeze({ calls: Object.freeze([...calls]), partial, disconnected });
  }
}
