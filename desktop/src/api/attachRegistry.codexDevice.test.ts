import { afterEach, expect, it } from "vitest";
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, getCodexDeviceChallenge, setAttachPermission, updateAttachSession } from "./attachRegistry";
import { aSession } from "../test/fixtures";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { Kind, encode } from "../lib/wire";
import { CODEX_DEVICE_URL } from "./codexDeviceSignIn";

const text = `Welcome to Codex [v0.160.0]\nFollow these steps to sign in with ChatGPT using device code authorization\n${CODEX_DEVICE_URL}\nEnter this one-time code (expires in 15 minutes):\nAB12-C345D\n`;
afterEach(() => { __resetAttachRegistry(); FakeWebSocket.reset(); });
function open(authOnly = true) {
  const session = aSession({ harness: "codex", authOnly, authMode: "chatgpt", worktreeId: "setup-codex", projectId: "", branchId: "" });
  configureAttachRegistry({ client: { openAttach: () => new FakeWebSocket("ws://localhost/auth") as unknown as WebSocket, getSession: async () => session } });
  const handle = acquireAttach(session, { readOnly: false, createTerm: () => new FakeTerm() });
  handle.mount();
  const socket = FakeWebSocket.last()!; socket.acceptOpen();
  socket.emitBinary(encode(Kind.Stdout, new TextEncoder().encode(text)));
  return { session, socket };
}
it("captures only the auth utility stream, never a coding agent's matching prose", () => {
  const { session } = open(false);
  expect(getCodexDeviceChallenge(session.id)).toBeNull();
});
it("clears the code immediately on a typed process Exit without inferring auth success", () => {
  const { session, socket } = open();
  expect(getCodexDeviceChallenge(session.id)?.code).toBe("AB12-C345D");
  socket.emitBinary(encode(Kind.Exit, new Uint8Array()));
  expect(getCodexDeviceChallenge(session.id)).toBeNull();
});
it("hides the code after permission revocation or exited polling state", () => {
  const { session } = open();
  setAttachPermission(session.id, false);
  expect(getCodexDeviceChallenge(session.id)).toBeNull();
  updateAttachSession({ ...session, status: "exited" });
  expect(getCodexDeviceChallenge(session.id)).toBeNull();
});
