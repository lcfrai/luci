import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ExcalidrawProps } from "@excalidraw/excalidraw/types";
import { productApi, type Design, type DesignScene } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import {
  DEFAULT_APPEARANCE,
  updateAppearance,
} from "../../preferences/appearance";
import DesignEditor from "./DesignEditor";

const canvasState = vi.hoisted(() => ({ change: null as NonNullable<ExcalidrawProps["onChange"]> | null }));
vi.mock("@excalidraw/excalidraw", async () => {
  const { useEffect } = await import("react");
  return {
  Excalidraw: ({ theme, initialData, onChange }: { theme: string; initialData: DesignScene; onChange: NonNullable<ExcalidrawProps["onChange"]> }) => {
    canvasState.change = onChange;
    useEffect(() => {
      onChange(...[initialData.elements || [], { ...initialData.appState, theme, scrollX: 0, scrollY: 0, selectedElementIds: {} }, initialData.files || {}] as unknown as Parameters<typeof onChange>);
    }, []);
    return <div data-testid="canvas" data-theme={theme} data-paper={initialData.appState?.viewBackgroundColor as string} />;
  },
  MainMenu: Object.assign(() => null, {
    DefaultItems: {
      Export: () => null,
      SaveAsImage: () => null,
      ClearCanvas: () => null,
      ChangeCanvasBackground: () => null,
    },
  }),
  // The pinned adapter persists exactly these document fields; theme, viewport,
  // selection and drawing-tool state are deliberately outside its export.
  serializeAsJSON: (elements: Record<string, unknown>[], state: Record<string, unknown>, files: Record<string, unknown>) => JSON.stringify({ elements: elements.filter(element => !element.isDeleted), appState: Object.fromEntries(["gridSize", "gridStep", "gridModeEnabled", "viewBackgroundColor"].filter(key => key in state).map(key => [key, state[key]])), files }),
}; });
beforeEach(() => { resetCockpit(); canvasState.change = null; });
afterEach(() => {
  updateAppearance(DEFAULT_APPEARANCE);
  vi.restoreAllMocks();
  document.documentElement.style.removeProperty("--design-paper");
});

const shape = { id: "rectangle-1", type: "rectangle", x: 0, y: 0, width: 120, height: 80, version: 1 };
const paper = { viewBackgroundColor: "#ffffff", gridSize: 20, gridStep: 5, gridModeEnabled: false };
const scene = (): DesignScene => ({ elements: [shape], appState: paper, files: {} });
const design = (value = scene()): Design => ({ projectId: "p-1", revision: 3, briefRevision: 2, frames: [], scene: value, updatedAt: "now" });
function change(elements: unknown[], state: Record<string, unknown> = paper, files: Record<string, unknown> = {}) {
  act(() => canvasState.change?.(...[elements, state, files] as unknown as Parameters<NonNullable<ExcalidrawProps["onChange"]>>));
}

it("does not register hydration, appearance, viewport, selection or image cache changes as document drafts", async () => {
  const file = { id: "image-1", mimeType: "image/png", dataURL: "data:image/png;base64,cGl4ZWxz", created: 1, lastRetrieved: 1 };
  const picture = { ...shape, type: "image", fileId: "image-1" };
  vi.spyOn(productApi, "design").mockResolvedValue(design({ elements: [picture], appState: paper, files: { "image-1": file } }));
  const save = vi.spyOn(productApi, "saveDesign");
  const dirty = vi.fn();
  render(<DesignEditor projectId="p-1" onDirtyChange={dirty} />);
  await screen.findByTestId("canvas");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([]);
  change([picture], { ...paper, theme: "dark", zoom: { value: 2 }, scrollX: 100, scrollY: -40, selectedElementIds: { "rectangle-1": true }, activeTool: { type: "arrow" } }, { "image-1": { ...file, lastRetrieved: 20 } });
  act(() => updateAppearance({ theme: "dark" }));
  expect(screen.queryByText("Unsaved sketch changes.")).toBeNull();
  expect((screen.getByRole("button", { name: "Save design" }) as HTMLButtonElement).disabled).toBe(true);
  expect(dirty).not.toHaveBeenCalledWith(true);
  expect(save).not.toHaveBeenCalled();
});

it("protects an edited sketch from project navigation and saves its exact persisted scene", async () => {
  vi.spyOn(productApi, "design").mockResolvedValue(design());
  const edited = { ...shape, x: 75, version: 2 };
  const savedScene = { elements: [edited], appState: paper, files: {} };
  const save = vi.spyOn(productApi, "saveDesign").mockResolvedValue({ ...design(savedScene), revision: 4 });
  useCockpit.getState().selectProject("p-1");
  useCockpit.getState().setLens("design");
  render(<DesignEditor projectId="p-1" />);
  await screen.findByTestId("canvas");
  change([edited], { ...paper, selectedElementIds: { "rectangle-1": true }, scrollX: 80 });
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([{ projectId: "p-1", label: "Sketch", lens: "design" }]);
  act(() => useCockpit.getState().setLens("develop"));
  expect(useCockpit.getState().lens).toBe("design");
  expect(useCockpit.getState().pendingNavigation).not.toBeNull();
  const closing = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(closing);
  expect(closing.defaultPrevented).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Save design" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", { revision: 3, frames: [], scene: savedScene });
  await screen.findByText("Design saved.");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([]);
});

it("retains newer drawing changes while a save is pending and retries from the acknowledged revision", async () => {
  vi.spyOn(productApi, "design").mockResolvedValue(design());
  const first = { ...shape, x: 75, version: 2 }, later = { ...shape, x: 140, version: 3 };
  let finish!: (value: Design) => void;
  const save = vi.spyOn(productApi, "saveDesign").mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce({ ...design({ elements: [later], appState: paper, files: {} }), revision: 5 });
  render(<DesignEditor projectId="p-1" />);
  await screen.findByTestId("canvas");
  change([first]);
  await userEvent.click(screen.getByRole("button", { name: "Save design" }));
  const saving = screen.getByRole("button", { name: "Saving…" });
  expect(saving.getAttribute("aria-busy")).toBe("true");
  await userEvent.click(saving);
  expect(save).toHaveBeenCalledOnce();
  change([later]);
  await act(async () => finish({ ...design({ elements: [first], appState: paper, files: {} }), revision: 4 }));
  expect(screen.queryByText("Design saved.")).toBeNull();
  expect(screen.getByText("Unsaved sketch changes.")).not.toBeNull();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  await userEvent.click(screen.getByRole("button", { name: "Save design" }));
  expect(save.mock.calls[1]).toEqual(["p-1", { revision: 4, frames: [], scene: { elements: [later], appState: paper, files: {} } }]);
  await screen.findByText("Design saved.");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([]);
});

it("keeps a refused save as an unsaved sketch and offers loading recovery without silently saving", async () => {
  const load = vi.spyOn(productApi, "design").mockRejectedValueOnce(new Error("Workspace is offline")).mockResolvedValueOnce(design());
  const save = vi.spyOn(productApi, "saveDesign").mockRejectedValue(new Error("The saved sketch revision changed"));
  render(<DesignEditor projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Retry loading design" }));
  await screen.findByTestId("canvas");
  expect(load).toHaveBeenCalledTimes(2);
  expect(save).not.toHaveBeenCalled();
  change([{ ...shape, y: 50, version: 2 }]);
  await userEvent.click(screen.getByRole("button", { name: "Save design" }));
  expect((await screen.findByRole("alert")).textContent).toContain("saved sketch revision changed");
  expect(screen.getByText("Unsaved sketch changes.")).not.toBeNull();
  await waitFor(() => expect((screen.getByRole("button", { name: "Save design" }) as HTMLButtonElement).disabled).toBe(false));
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
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
