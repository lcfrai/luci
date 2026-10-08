/** Shortcuts belong to the client keyboard, even when its workspace runs on another OS. */
export type KeyboardPlatform = "mac" | "other";

export function clientKeyboardPlatform(): KeyboardPlatform {
  if (typeof navigator === "undefined") return "other";
  const client = navigator as Navigator & { userAgentData?: { platform?: string } };
  const platform = client.userAgentData?.platform || client.platform || client.userAgent;
  return /Mac|iPhone|iPad|iPod/i.test(platform) ? "mac" : "other";
}

export function shortcutLabel(key: string, platform = clientKeyboardPlatform(), shift = false): string {
  return platform === "mac" ? `⌘${shift ? "⇧" : ""}${key}` : `Ctrl+${shift ? "Shift+" : ""}${key}`;
}
