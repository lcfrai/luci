import { describe, expect, it } from "vitest";
import { CODEX_DEVICE_URL, CodexDevicePromptCapture, DEVICE_PROMPT_BYTES, isCodexDeviceSignIn, parseCodexDevicePrompt } from "./codexDeviceSignIn";
import { aSession } from "../test/fixtures";

export const prompt = `Welcome to Codex [v0.160.0]\nFollow these steps to sign in with ChatGPT using device code authorization\n1. Open this link in your browser:\n${CODEX_DEVICE_URL}\n2. Enter this one-time code (expires in 15 minutes):\n\nAB12-C345D\n`;

describe("official device prompt display data", () => {
  it("accepts observed 4+5 codes and ordinary ANSI split across frames", () => {
    const capture = new CodexDevicePromptCapture();
    expect(capture.observe(new TextEncoder().encode("\x1b[3"))).toBe(false);
    expect(capture.observe(new TextEncoder().encode("2m" + prompt + "\x1b[0m"), 10)).toBe(true);
    expect(capture.challenge).toEqual({ code: "AB12-C345D", url: CODEX_DEVICE_URL, observedAt: 10 });
    expect(capture.observe(new TextEncoder().encode(prompt), 50)).toBe(false);
    expect(capture.challenge?.observedAt).toBe(10);
  });
  it.each([
    prompt.replace(CODEX_DEVICE_URL, CODEX_DEVICE_URL + ".evil.example"),
    prompt.replace(CODEX_DEVICE_URL, "https://auth.openai.com.evil.example/codex/device"),
    prompt.replace(CODEX_DEVICE_URL, CODEX_DEVICE_URL + "?code=AB12-C345D"),
    prompt.replace(CODEX_DEVICE_URL, "http://auth.openai.com/codex/device"),
    prompt.replace("AB12-C345D", "abcd-efghi"),
    prompt.replace("AB12-C345D", "AB12-C34567890"),
    prompt.replace("AB12-C345D", "AB12-C345D\nEF12-G345H"),
    prompt.replace("Welcome to Codex", "The assistant says"),
    prompt.replace("device code authorization", "a tool call"),
    prompt + "\x1b]unclosed",
    "x".repeat(DEVICE_PROMPT_BYTES) + prompt,
  ])("rejects unrelated, ambiguous or unbounded text", text => {
    expect(parseCodexDevicePrompt(text)).toBeNull();
  });
  it("bounds total retained bytes and clears an allocation's challenge", () => {
    const capture = new CodexDevicePromptCapture();
    capture.observe(new Uint8Array(DEVICE_PROMPT_BYTES + 1));
    expect(capture.observe(new TextEncoder().encode(prompt))).toBe(false);
    expect(capture.challenge).toBeNull();
    const fresh = new CodexDevicePromptCapture();
    fresh.observe(new TextEncoder().encode(prompt)); fresh.clear();
    expect(fresh.challenge).toBeNull();
  });
  it("accepts only the server's project-free Codex ChatGPT sign-in utility", () => {
    const auth = aSession({ harness: "codex", authOnly: true, authMode: "chatgpt", worktreeId: "setup-codex", projectId: "", branchId: "" });
    expect(isCodexDeviceSignIn(auth)).toBe(true);
    for (const change of [{ authOnly: false }, { harness: "opencode" }, { authMode: "api" }, { projectId: "p-project" }, { worktreeId: "w-project" }]) {
      expect(isCodexDeviceSignIn({ ...auth, ...change })).toBe(false);
    }
  });
});
