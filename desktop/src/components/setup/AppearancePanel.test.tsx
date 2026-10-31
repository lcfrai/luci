import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { APPEARANCE_KEY, DEFAULT_APPEARANCE, updateAppearance } from "../../preferences/appearance";
import { AppearancePanel } from "./AppearancePanel";

afterEach(() => { cleanup(); vi.restoreAllMocks(); updateAppearance(DEFAULT_APPEARANCE); });

it("exposes three densities and independent contrast, and saves selections without resetting colour or terminal choices", () => {
  updateAppearance({ theme: "dark", accent: "rose", terminalFontSize: 20 });
  render(<AppearancePanel />);
  const density = screen.getByRole("combobox", { name: "Interface density" });
  expect(within(density).getAllByRole("option").map(option => option.textContent)).toEqual(["Comfortable", "Compact", "Spacious"]);
  fireEvent.change(density, { target: { value: "spacious" } });
  fireEvent.change(screen.getByRole("combobox", { name: "Surface contrast" }), { target: { value: "strong" } });
  expect(JSON.parse(localStorage.getItem(APPEARANCE_KEY)!)).toEqual({ theme: "dark", accent: "rose", terminalFontSize: 20, density: "spacious", contrast: "strong" });
  expect(document.documentElement.dataset).toMatchObject({ density: "spacious", contrast: "strong", accent: "rose", theme: "dark" });
  expect(screen.getByRole("region", { name: "Appearance preview" })).not.toBeNull();
});

it("reset restores every appearance choice and the visible controls", () => {
  updateAppearance({ theme: "light", accent: "blue", terminalFontSize: 24, density: "compact", contrast: "strong" });
  render(<AppearancePanel />);
  fireEvent.click(screen.getByRole("button", { name: "Reset appearance" }));
  expect(JSON.parse(localStorage.getItem(APPEARANCE_KEY)!)).toEqual(DEFAULT_APPEARANCE);
  expect((screen.getByRole("combobox", { name: "Interface density" }) as HTMLSelectElement).value).toBe("comfortable");
  expect((screen.getByRole("combobox", { name: "Surface contrast" }) as HTMLSelectElement).value).toBe("standard");
});

it("shows current-window-only changes without discarding them, then confirms a successful reset", () => {
  updateAppearance(DEFAULT_APPEARANCE);
  render(<AppearancePanel />);
  const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Storage unavailable"); });
  fireEvent.change(screen.getByRole("combobox", { name: "Interface density" }), { target: { value: "spacious" } });
  expect(screen.getByRole("status").textContent).toContain("this window only");
  expect((screen.getByRole("combobox", { name: "Interface density" }) as HTMLSelectElement).value).toBe("spacious");
  write.mockRestore();
  fireEvent.click(screen.getByRole("button", { name: "Reset appearance" }));
  expect(screen.getByRole("status").textContent).toBe("Appearance saved on this device.");
});
