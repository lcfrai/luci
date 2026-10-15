import type { Session } from "../store/types";

export const CODEX_DEVICE_URL = "https://auth.openai.com/codex/device";
export const DEVICE_PROMPT_BYTES = 8192;
export const DEVICE_CODE_MAX_AGE_MS = 15 * 60 * 1000;
export interface CodexDeviceChallenge {
  readonly code: string;
  readonly url: typeof CODEX_DEVICE_URL;
  readonly observedAt: number;
}

/** Only the API's project-free official device-login utility can supply this UI.
 * This recognizes display data, never authentication success or tool authority. */
export function isCodexDeviceSignIn(session: Session): boolean {
  return session.harness === "codex" && session.authOnly === true &&
    session.authMode === "chatgpt" && session.worktreeId === "setup-codex" &&
    session.projectId === "" && session.branchId === "";
}

export function parseCodexDevicePrompt(raw: string): string | null {
  if (raw.length > DEVICE_PROMPT_BYTES) return null;
  const text = raw
    .replace(/\x1b\][^\x07\x1b]{0,512}(?:\x07|\x1b\\)/g, "")
    .replace(/\x1b\[[0-?]{0,32}[ -/]{0,8}[@-~]/g, "");
  // Incomplete or unsupported escapes are not display instructions.
  if (text.includes("\x1b")) return null;
  const plain = text.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, "");
  const words = plain.replace(/\s+/g, " ");
  if (!words.includes("Welcome to Codex") ||
      !/sign in with ChatGPT using device code authorization/i.test(words)) return null;
  const urls = plain.match(/https?:\/\/[^\s<>"']+/g) ?? [];
  if (urls.length === 0 || urls.some(url => url !== CODEX_DEVICE_URL)) return null;
  const lines = plain.split(/\r?\n/);
  const label = lines.findIndex(line => /Enter this one-time code/i.test(line));
  if (label < 0) return null;
  // The pinned native prompt uses two uppercase alphanumeric groups (observed
  // 4+5). Keep both groups bounded without assuming a 4+4 vendor schema.
  const codes = lines.slice(label + 1, label + 5).map(line => line.trim())
    .filter(line => /^[A-Z0-9]{3,6}-[A-Z0-9]{3,6}$/.test(line));
  return codes.length === 1 ? codes[0] : null;
}

/** Ephemeral, allocation-owned capture. No persistence, log, fetch or input. */
export class CodexDevicePromptCapture {
  private decoder = new TextDecoder();
  private text = "";
  private bytes = 0;
  private blocked = false;
  challenge: CodexDeviceChallenge | null = null;

  observe(payload: Uint8Array, now = Date.now()): boolean {
    if (this.blocked || this.challenge) return false;
    this.bytes += payload.byteLength;
    if (this.bytes > DEVICE_PROMPT_BYTES) {
      this.clear();
      this.blocked = true;
      return false;
    }
    this.text += this.decoder.decode(payload, { stream: true });
    const code = parseCodexDevicePrompt(this.text);
    if (!code) return false;
    this.challenge = Object.freeze({ code, url: CODEX_DEVICE_URL, observedAt: now });
    this.text = "";
    return true;
  }

  clear(): void {
    this.challenge = null;
    this.text = "";
    this.bytes = 0;
    this.decoder = new TextDecoder();
  }
}
