import { afterEach, describe, expect, it, vi } from "vitest";
import { clientKeyboardPlatform, shortcutLabel } from "./keyboardPlatform";

describe("client keyboard shortcut labels", () => {
  afterEach(() => vi.unstubAllGlobals());

  it.each([
    [{ userAgentData: { platform: "Windows" }, platform: "MacIntel" }, "other", "Ctrl+K"],
    [{ userAgentData: { platform: "macOS" }, platform: "Win32" }, "mac", "⌘K"],
    [{ platform: "Win32" }, "other", "Ctrl+K"],
    [{ platform: "Linux x86_64" }, "other", "Ctrl+K"],
    [{ platform: "MacIntel" }, "mac", "⌘K"],
    [{ platform: "iPad" }, "mac", "⌘K"],
    [{ platform: "", userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X)" }, "mac", "⌘K"],
    [{ platform: "" }, "other", "Ctrl+K"],
  ])("uses local navigator information %j", (navigator, platform, hint) => {
    vi.stubGlobal("navigator", navigator);
    expect(clientKeyboardPlatform()).toBe(platform);
    expect(shortcutLabel("K")).toBe(hint);
  });

  it("has a safe nonbrowser default without requiring a workspace connection", () => {
    vi.stubGlobal("navigator", undefined);
    expect(clientKeyboardPlatform()).toBe("other");
    expect(shortcutLabel("M", "other", true)).toBe("Ctrl+Shift+M");
    expect(shortcutLabel("M", "mac", true)).toBe("⌘⇧M");
  });
});
