import { afterEach, expect, it } from "vitest";
import {
  APPEARANCE_KEY,
  DEFAULT_APPEARANCE,
  getAppearance,
  parseAppearance,
  updateAppearance,
} from "./appearance";

afterEach(() => updateAppearance(DEFAULT_APPEARANCE));
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
    terminalFontSize: 20,
  });
  expect(getAppearance()).toEqual({
    theme: "light",
    resolvedTheme: "light",
    accent: "rose",
    density: "compact",
    terminalFontSize: 20,
  });
  expect(document.documentElement.dataset).toMatchObject({
    theme: "light",
    accent: "rose",
    density: "compact",
  });
  expect(JSON.parse(localStorage.getItem(APPEARANCE_KEY)!)).toEqual({
    theme: "light",
    accent: "rose",
    density: "compact",
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
