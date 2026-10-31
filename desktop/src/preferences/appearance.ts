import { useSyncExternalStore } from "react";

export type Appearance = {
  theme: "system" | "light" | "dark";
  accent: "teal" | "blue" | "rose";
  density: "comfortable" | "compact" | "spacious";
  contrast: "standard" | "strong";
  terminalFontSize: number;
};
export const APPEARANCE_KEY = "luci.appearance.v1";
export const DEFAULT_APPEARANCE: Appearance = {
  theme: "system",
  accent: "teal",
  density: "comfortable",
  contrast: "standard",
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
    density:
      candidate.density === "compact" || candidate.density === "spacious"
        ? candidate.density
        : "comfortable",
    contrast: candidate.contrast === "strong" ? "strong" : "standard",
    terminalFontSize:
      typeof candidate.terminalFontSize === "number" &&
      Number.isInteger(candidate.terminalFontSize) &&
      candidate.terminalFontSize >= 11 &&
      candidate.terminalFontSize <= 24
        ? candidate.terminalFontSize
        : 14,
  };
}
type Persistence = "persisted" | "window-only";
function stored(): { value?: Appearance; persistence: Persistence } {
  try {
    return {
      value: parseAppearance(JSON.parse(localStorage.getItem(APPEARANCE_KEY) || "null")),
      persistence: "persisted",
    };
  } catch {
    return { persistence: "window-only" };
  }
}
const initial = stored();
let preference = initial.value ?? { ...DEFAULT_APPEARANCE };
let persistence = initial.persistence;
let media: MediaQueryList | undefined;
let resolvedTheme: "light" | "dark" = "dark";
let snapshot: Appearance & { resolvedTheme: "light" | "dark"; persistence: Persistence } = {
  ...preference,
  resolvedTheme,
  persistence,
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
  root.dataset.contrast = preference.contrast;
  snapshot = { ...preference, resolvedTheme, persistence };
  listeners.forEach((listener) => listener());
}
export function initializeAppearance() {
  if (initialized) return;
  initialized = true;
  media = window.matchMedia("(prefers-color-scheme: dark)");
  media.addEventListener("change", apply);
  window.addEventListener("storage", (event) => {
    if (event.key === APPEARANCE_KEY || event.key === null) {
      const saved = stored();
      // A blocked or malformed read must not erase the current window's choices.
      if (saved.value) preference = saved.value;
      persistence = saved.persistence;
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
    persistence = "persisted";
  } catch {
    persistence = "window-only";
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
