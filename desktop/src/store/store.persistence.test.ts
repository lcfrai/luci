import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { aProject } from "../test/fixtures";
import { createCockpitStore, PERSIST_KEY } from "./store";

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe("unavailable preference storage", () => {
  it.each(["QuotaExceededError", "SecurityError"])("keeps live edits and the explicit draft-loss guard when writes throw %s", name => {
    const store = createCockpitStore();
    store.getState().setProjects([aProject({ id: "p-1" })]);
    store.getState().selectProject("p-1", "discover");
    const saved = localStorage.getItem(PERSIST_KEY);
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new DOMException("unavailable", name); });
    const discard = vi.fn();

    expect(() => {
      store.setState({ layout: "canvas" });
      store.getState().setUnsavedProjectDraft("editor", {
        projectId: "p-1", label: "Unsaved settings", lens: "discover", retained: false, discard,
      });
      store.getState().setLens("design");
    }).not.toThrow();
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().lens).toBe("discover");
    expect(store.getState().pendingNavigation).toEqual({ kind: "lens", lens: "design" });
    expect(store.getState().unsavedProjectDrafts.editor.retained).toBe(false);
    expect(localStorage.getItem(PERSIST_KEY)).toBe(saved);
    expect(logged).toHaveBeenCalledExactlyOnceWith("[cockpit] could not save preferences; changes remain in memory only.");

    // A failed browser save cannot authorize the "keep drafts" escape route.
    expect(() => store.getState().discardDraftsAndNavigate(true)).not.toThrow();
    expect(store.getState().lens).toBe("discover");
    expect(discard).not.toHaveBeenCalled();
    expect(() => store.getState().discardDraftsAndNavigate()).not.toThrow();
    expect(discard).toHaveBeenCalledTimes(1);
    expect(store.getState().lens).toBe("design");
    expect(store.getState().unsavedProjectDrafts).toEqual({});
    expect(store.getState().pendingNavigation).toBeNull();
  });

  it("resumes real preference persistence after capacity recovers without persisting live drafts", () => {
    const store = createCockpitStore();
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new DOMException("full", "QuotaExceededError"); });
    store.getState().setLayout("canvas");
    store.getState().setUnsavedProjectDraft("editor", {
      projectId: "p-1", label: "Private live draft", lens: "discover", retained: false, discard: vi.fn(),
    });
    expect(createCockpitStore().getState().layout).toBe("columns");
    write.mockRestore();
    store.getState().toggleRail();

    const raw = localStorage.getItem(PERSIST_KEY)!;
    expect(JSON.parse(raw).state.layout).toBe("canvas");
    expect(raw).not.toContain("Private live draft");
    expect(JSON.parse(raw).state).not.toHaveProperty("unsavedProjectDrafts");
    const restarted = createCockpitStore().getState();
    expect(restarted.layout).toBe("canvas");
    expect(restarted.railCollapsed).toBe(true);
    expect(restarted.unsavedProjectDrafts).toEqual({});
    // Failure suppression ends on a successful write, rather than hiding future failures.
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new DOMException("full", "QuotaExceededError"); });
    store.getState().toggleRail();
    expect(logged).toHaveBeenCalledTimes(2);
  });

  it("keeps actions usable when the localStorage getter is denied and retries storage later", () => {
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const access = vi.spyOn(window, "localStorage", "get").mockImplementation(() => { throw new DOMException("denied", "SecurityError"); });
    const store = createCockpitStore();
    expect(() => store.getState().setLayout("canvas")).not.toThrow();
    expect(store.getState().layout).toBe("canvas");
    expect(logged).toHaveBeenCalled();
    access.mockRestore();
    store.getState().toggleRail();
    expect(createCockpitStore().getState().layout).toBe("canvas");
    expect(createCockpitStore().getState().railCollapsed).toBe(true);
  });

  it("reports a refused clear without claiming the old snapshot was removed", () => {
    const store = createCockpitStore();
    store.getState().setLayout("canvas");
    const saved = localStorage.getItem(PERSIST_KEY);
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const remove = vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new DOMException("denied", "SecurityError"); });
    expect(() => store.persist.clearStorage()).not.toThrow();
    expect(() => store.persist.clearStorage()).not.toThrow();
    expect(localStorage.getItem(PERSIST_KEY)).toBe(saved);
    expect(logged).toHaveBeenCalledExactlyOnceWith("[cockpit] could not clear saved preferences; the browser snapshot may still remain.");
    remove.mockRestore();
    store.persist.clearStorage();
    expect(localStorage.getItem(PERSIST_KEY)).toBeNull();
    expect(store.getState().layout).toBe("canvas");
  });
});
