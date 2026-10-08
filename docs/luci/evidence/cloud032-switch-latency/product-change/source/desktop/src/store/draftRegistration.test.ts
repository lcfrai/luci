import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { aProject } from "../test/fixtures";
import { createCockpitStore, PERSIST_KEY } from "./store";
import type { UnsavedProjectDraft } from "./slices/uiSlice";

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

function projectStore() {
  const store = createCockpitStore();
  store.getState().setProjects([aProject({ id: "A" }), aProject({ id: "B" })]);
  store.getState().selectProject("A", "discover");
  return store;
}

it("does not notify subscribers or write storage when clean editor lifecycles clear absent drafts", () => {
  const store = projectStore(), before = store.getState();
  const changed = vi.fn(), unsubscribe = store.subscribe(changed);
  const write = vi.spyOn(Storage.prototype, "setItem");
  const remove = vi.spyOn(Storage.prototype, "removeItem");
  try {
    for (const id of ["clean-definition", "clean-definition", "clean-design", "toString"]) {
      store.getState().setUnsavedProjectDraft(id, null);
    }
    expect(store.getState()).toBe(before);
    expect(changed).not.toHaveBeenCalled();
    expect(write).not.toHaveBeenCalled();
    expect(remove).not.toHaveBeenCalled();
  } finally { unsubscribe(); }
});

it("still publishes actual draft additions, updates and removals without persisting the draft", () => {
  const store = projectStore();
  const changed = vi.fn(), unsubscribe = store.subscribe(changed);
  const write = vi.spyOn(Storage.prototype, "setItem");
  const draft: UnsavedProjectDraft = { projectId: "A", label: "Definition", lens: "discover", retained: false };
  const updated = { ...draft, retained: true, discard: vi.fn() };
  try {
    store.getState().setUnsavedProjectDraft("editor", draft);
    expect(store.getState().unsavedProjectDrafts.editor).toBe(draft);
    store.getState().setUnsavedProjectDraft("editor", updated);
    expect(store.getState().unsavedProjectDrafts.editor).toBe(updated);
    store.getState().setUnsavedProjectDraft("editor", null);
    expect(store.getState().unsavedProjectDrafts).not.toHaveProperty("editor");
    expect(changed).toHaveBeenCalledTimes(3);
    expect(write.mock.calls.filter(([key]) => key === PERSIST_KEY)).toHaveLength(3);
    expect(JSON.parse(localStorage.getItem(PERSIST_KEY)!).state).not.toHaveProperty("unsavedProjectDrafts");
    expect(updated.discard).not.toHaveBeenCalled();
  } finally { unsubscribe(); }
});

it("keeps a real dirty editor's navigation guard when unrelated clean editors unmount", () => {
  const store = projectStore(), discard = vi.fn();
  const draft: UnsavedProjectDraft = { projectId: "A", label: "Definition", lens: "discover", retained: true, discard };
  store.getState().setUnsavedProjectDraft("dirty-editor", draft);
  store.getState().selectProject("B");
  const guarded = store.getState();
  const write = vi.spyOn(Storage.prototype, "setItem");
  store.getState().setUnsavedProjectDraft("clean-editor", null);
  expect(store.getState()).toBe(guarded);
  expect(store.getState().selection.projectId).toBe("A");
  expect(store.getState().pendingNavigation).toEqual({ kind: "project", projectId: "B" });
  expect(store.getState().unsavedProjectDrafts["dirty-editor"]).toBe(draft);
  expect(write).not.toHaveBeenCalled();
  expect(discard).not.toHaveBeenCalled();

  store.getState().discardDraftsAndNavigate();
  expect(discard).toHaveBeenCalledOnce();
  expect(store.getState().selection.projectId).toBe("B");
  expect(store.getState().pendingNavigation).toBeNull();
  expect(store.getState().unsavedProjectDrafts).toEqual({});
  const navigated = store.getState();
  write.mockClear();
  store.getState().setUnsavedProjectDraft("dirty-editor", null);
  expect(store.getState()).toBe(navigated);
  expect(write).not.toHaveBeenCalled();
});
