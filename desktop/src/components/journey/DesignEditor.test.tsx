import { afterEach, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { productApi } from "../../api/product";
import {
  DEFAULT_APPEARANCE,
  updateAppearance,
} from "../../preferences/appearance";
import DesignEditor from "./DesignEditor";

vi.mock("@excalidraw/excalidraw", () => ({
  Excalidraw: ({ theme, initialData }: { theme: string; initialData: { appState?: { viewBackgroundColor?: string } } }) => (
    <div data-testid="canvas" data-theme={theme} data-paper={initialData.appState?.viewBackgroundColor} />
  ),
  MainMenu: Object.assign(() => null, {
    DefaultItems: {
      Export: () => null,
      SaveAsImage: () => null,
      ClearCanvas: () => null,
      ChangeCanvasBackground: () => null,
    },
  }),
  serializeAsJSON: vi.fn(),
}));
afterEach(() => {
  updateAppearance(DEFAULT_APPEARANCE);
  vi.restoreAllMocks();
  document.documentElement.style.removeProperty("--design-paper");
});
it("resolves the document paper token to a concrete canvas colour", async () => {
  vi.spyOn(productApi, "design").mockResolvedValue({ projectId: "p-1", revision: 0, briefRevision: 0, frames: [], updatedAt: "now" });
  document.documentElement.style.setProperty("--design-paper", "oklch(98% 0.0045 67)");
  const context = { fillStyle: "", fillRect: vi.fn(), getImageData: () => ({ data: new Uint8ClampedArray([249, 247, 244, 255]) }) };
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(context as unknown as CanvasRenderingContext2D);
  render(<DesignEditor projectId="p-1" />);
  expect((await screen.findByTestId("canvas")).getAttribute("data-paper")).toBe("#f9f7f4");
  expect(context.fillStyle).toBe("oklch(98% 0.0045 67)");
});
it("follows appearance without reloading or saving over the project design", async () => {
  const load = vi
    .spyOn(productApi, "design")
    .mockResolvedValue({
      projectId: "p-1",
      revision: 3,
      briefRevision: 2,
      frames: [],
      updatedAt: "now",
    });
  const save = vi.spyOn(productApi, "saveDesign");
  updateAppearance({ theme: "dark" });
  render(<DesignEditor projectId="p-1" />);
  expect((await screen.findByTestId("canvas")).getAttribute("data-theme")).toBe(
    "dark",
  );
  act(() => updateAppearance({ theme: "light" }));
  expect(screen.getByTestId("canvas").getAttribute("data-theme")).toBe("light");
  expect(load).toHaveBeenCalledOnce();
  expect(save).not.toHaveBeenCalled();
});
