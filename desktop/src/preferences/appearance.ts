import { useSyncExternalStore } from "react";

export type Appearance = {
  theme: "system" | "light" | "dark";
  accent: "teal" | "blue" | "rose";
  density: "comfortable" | "compact";
  terminalFontSize: number;
};
export const APPEARANCE_KEY = "luci.appearance.v1";
export const DEFAULT_APPEARANCE: Appearance = {
  theme: "system",
  accent: "teal",
  density: "comfortable",
  terminalFontSize: 14,
};
export function parseAppearance(value: unknown): Appearance {
  const candidate =
    value && typeof value === "object" ? (value as Partial<Appearance>) : {};
  return {
    theme:
      candidate.theme === "light" || candidate.theme === "dark"
        ? candidate.theme
        : "system",
    accent:
      candidate.accent === "blue" || candidate.accent === "rose"
        ? candidate.accent
        : "teal",
    density: candidate.density === "compact" ? "compact" : "comfortable",
    terminalFontSize:
      typeof candidate.terminalFontSize === "number" &&
      Number.isInteger(candidate.terminalFontSize) &&
      candidate.terminalFontSize >= 11 &&
      candidate.terminalFontSize <= 24
        ? candidate.terminalFontSize
        : 14,
  };
}
function stored(): Appearance {
  try {
    return parseAppearance(
      JSON.parse(localStorage.getItem(APPEARANCE_KEY) || "null"),
    );
  } catch {
    return { ...DEFAULT_APPEARANCE };
  }
}
let preference = stored();
let media: MediaQueryList | undefined;
let resolvedTheme: "light" | "dark" = "dark";
let snapshot: Appearance & { resolvedTheme: "light" | "dark" } = {
  ...preference,
  resolvedTheme,
};
const listeners = new Set<() => void>();
let initialized = false;
function apply() {
  resolvedTheme =
    preference.theme === "system"
      ? media?.matches
        ? "dark"
        : "light"
      : preference.theme;
  const root = document.documentElement;
  root.dataset.theme = resolvedTheme;
  root.dataset.accent = preference.accent;
  root.dataset.density = preference.density;
  snapshot = { ...preference, resolvedTheme };
  listeners.forEach((listener) => listener());
}
export function initializeAppearance() {
  if (initialized) return;
  initialized = true;
  media = window.matchMedia("(prefers-color-scheme: dark)");
  media.addEventListener("change", apply);
  window.addEventListener("storage", (event) => {
    if (event.key === APPEARANCE_KEY || event.key === null) {
      preference = stored();
      apply();
    }
  });
  apply();
}
export function updateAppearance(patch: Partial<Appearance>) {
  initializeAppearance();
  preference = parseAppearance({ ...preference, ...patch });
  try {
    localStorage.setItem(APPEARANCE_KEY, JSON.stringify(preference));
  } catch {
    /* The current window still works when storage is unavailable. */
  }
  apply();
}
export const getAppearance = () => snapshot;
export function useAppearance() {
  return useSyncExternalStore((listener) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }, getAppearance);
}
