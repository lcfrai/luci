import { afterEach, expect, it, vi } from "vitest";
import {
  APPEARANCE_KEY,
  DEFAULT_APPEARANCE,
  getAppearance,
  parseAppearance,
  updateAppearance,
} from "./appearance";

afterEach(() => { vi.restoreAllMocks(); updateAppearance(DEFAULT_APPEARANCE); });
it("ignores invalid saved settings and bounds terminal text size", () => {
  expect(
    parseAppearance({
      theme: "invalid",
      accent: "invisible",
      density: "tiny",
      terminalFontSize: 500,
    }),
  ).toEqual(DEFAULT_APPEARANCE);
  expect(parseAppearance({ terminalFontSize: 11.5 }).terminalFontSize).toBe(14);
  expect(parseAppearance(null)).toEqual(DEFAULT_APPEARANCE);
});
it("persists changes and applies the resolved appearance to the document", () => {
  updateAppearance({
    theme: "light",
    accent: "rose",
    density: "compact",
    contrast: "standard",
    terminalFontSize: 20,
  });
  expect(getAppearance()).toEqual({
    theme: "light",
    resolvedTheme: "light",
    persistence: "persisted",
    accent: "rose",
    density: "compact",
    contrast: "standard",
    terminalFontSize: 20,
  });
  expect(document.documentElement.dataset).toMatchObject({
    theme: "light",
    accent: "rose",
    density: "compact",
    contrast: "standard",
  });
  expect(JSON.parse(localStorage.getItem(APPEARANCE_KEY)!)).toEqual({
    theme: "light",
    accent: "rose",
    density: "compact",
    contrast: "standard",
    terminalFontSize: 20,
  });
});
it("reflects another window's saved preference without requiring an app restart", () => {
  updateAppearance({ theme: "light" });
  localStorage.setItem(
    APPEARANCE_KEY,
    JSON.stringify({
      ...DEFAULT_APPEARANCE,
      theme: "dark",
      terminalFontSize: 18,
    }),
  );
  window.dispatchEvent(new StorageEvent("storage", { key: APPEARANCE_KEY }));
  expect(getAppearance().resolvedTheme).toBe("dark");
  expect(getAppearance().terminalFontSize).toBe(18);
});
it("restores legacy choices while independently defaulting new contrast settings", () => {
  expect(parseAppearance({ theme: "dark", accent: "blue", density: "compact", terminalFontSize: 18 })).toEqual({
    theme: "dark", accent: "blue", density: "compact", terminalFontSize: 18, contrast: "standard",
  });
  expect(parseAppearance({ density: "huge", contrast: "invisible" })).toEqual(DEFAULT_APPEARANCE);
});
it("restores spacious and stronger contrast across windows without replacing other choices", () => {
  updateAppearance({ theme: "light", accent: "blue", terminalFontSize: 19 });
  updateAppearance({ density: "spacious", contrast: "strong" });
  expect(getAppearance()).toMatchObject({ theme: "light", accent: "blue", terminalFontSize: 19, density: "spacious", contrast: "strong" });
  expect(document.documentElement.dataset).toMatchObject({ density: "spacious", contrast: "strong", theme: "light", accent: "blue" });
  const saved = JSON.parse(localStorage.getItem(APPEARANCE_KEY)!);
  updateAppearance(DEFAULT_APPEARANCE);
  localStorage.setItem(APPEARANCE_KEY, JSON.stringify(saved));
  window.dispatchEvent(new StorageEvent("storage", { key: APPEARANCE_KEY }));
  expect(getAppearance()).toMatchObject(saved);
  expect(document.documentElement.dataset).toMatchObject({ density: "spacious", contrast: "strong" });
});
it("keeps active choices when saving fails and reports persistence again after a successful reset", () => {
  updateAppearance({ theme: "dark", accent: "rose", terminalFontSize: 20 });
  const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Storage unavailable"); });
  updateAppearance({ density: "spacious", contrast: "strong" });
  expect(getAppearance()).toMatchObject({ theme: "dark", accent: "rose", terminalFontSize: 20, density: "spacious", contrast: "strong", persistence: "window-only" });
  expect(document.documentElement.dataset).toMatchObject({ density: "spacious", contrast: "strong" });
  write.mockRestore();
  updateAppearance(DEFAULT_APPEARANCE);
  expect(getAppearance()).toMatchObject({ ...DEFAULT_APPEARANCE, persistence: "persisted" });
});
it("does not replace active preferences on a blocked storage event and follows a later valid event or clear", () => {
  updateAppearance({ density: "spacious", contrast: "strong" });
  const read = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("Storage unavailable"); });
  window.dispatchEvent(new StorageEvent("storage", { key: APPEARANCE_KEY }));
  expect(getAppearance()).toMatchObject({ density: "spacious", contrast: "strong", persistence: "window-only" });
  read.mockRestore();
  localStorage.setItem(APPEARANCE_KEY, JSON.stringify({ ...DEFAULT_APPEARANCE, accent: "blue" }));
  window.dispatchEvent(new StorageEvent("storage", { key: APPEARANCE_KEY }));
  expect(getAppearance()).toMatchObject({ ...DEFAULT_APPEARANCE, accent: "blue", persistence: "persisted" });
  localStorage.removeItem(APPEARANCE_KEY);
  window.dispatchEvent(new StorageEvent("storage", { key: null }));
  expect(getAppearance()).toMatchObject({ ...DEFAULT_APPEARANCE, persistence: "persisted" });
});
