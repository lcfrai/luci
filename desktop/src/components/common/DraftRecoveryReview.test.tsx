import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ProjectDraft, ProjectDraftState } from "../../hooks/useProjectDraft";
import { DraftRecovery } from "./DraftRecovery";

function recovery<T>(value: T, original?: T): ProjectDraftState<T> {
  const pending: ProjectDraft<T> = { version: 1, base: "old", value, original, updatedAt: 1700000000000 };
  return { pending, retained: true, restored: false, error: "", take: vi.fn(() => pending), clear: vi.fn(), offerRecovery: vi.fn() };
}
function details(title: string): HTMLDetailsElement {
  return screen.getByText(title, { selector: "summary" }).closest("details")!;
}
async function expand(title: string): Promise<HTMLDetailsElement> {
  const element = details(title);
  await userEvent.click(within(element).getByText(title, { selector: "summary" }));
  return element;
}
const outcome = (id: string, title: string) => ({ id, title, priority: "must", acceptance: "Reopening shows the saved note" });
const definition = () => ({
  projectId: "p-1", revision: 2, version: 1, briefRevision: 1, status: "draft", digest: "saved-fingerprint", updatedAt: "2026-10-06T12:00:00Z",
  startingPoint: "new", purpose: "Keep garden notes", audience: "Gardeners", constraints: "Works offline", outOfScope: ["Public profiles"],
  requirements: [outcome("AC-1", "Save a note")],
  journeys: [{ id: "j-1", name: "Record a harvest", actor: "Gardener", steps: "Write, save, then reopen", requirementIds: ["AC-1"] }],
  roles: [{ id: "r-1", name: "Editor", canDo: "Edit garden notes", cannotDo: "Read another team's notes" }],
  entities: [{ id: "e-1", name: "Garden note", fields: "Text and garden area", privacy: "Kept until the owner removes it" }],
  integrations: [{ id: "i-1", name: "Weather", purpose: "Attach conditions", constraint: "Never send note text" }],
  glossary: [{ id: "g-1", term: "Bed", meaning: "A garden growing area" }],
  decisions: [{ id: "d-1", title: "Local data", decision: "Save locally", reason: "Offline access", alternatives: "Cloud-only storage" }],
  questions: [{ id: "q-1", question: "How long are notes kept?", answer: "Until manually removed" }],
});

describe("structured draft recovery review", () => {
  it("presents the combined definition as readable outcomes and context, retaining all meaningful fields", async () => {
    const baseline = definition();
    const draft = recovery({ ...baseline, constraints: "Alice needs keyboard access" }, baseline);
    const saved = { ...baseline, revision: 3, requirements: [{ ...baseline.requirements[0], acceptance: "Bob requires the garden area to survive reopening" }] };
    const restore = vi.fn();
    const view = render(<DraftRecovery merge draft={draft} label="Definition" base="current" saved={saved} onRestore={restore} />);
    expect(view.container.querySelectorAll("pre")).toHaveLength(0);
    expect(screen.queryByRole("radio")).toBeNull();
    const combined = await expand("Review combined draft");
    await within(combined).findByText("Alice needs keyboard access");
    for (const text of ["A new idea", "Keep garden notes", "Gardeners", "Public profiles", "Save a note", "Must have", "Bob requires the garden area to survive reopening", "Record a harvest", "Write, save, then reopen", "Editor", "Edit garden notes", "Read another team's notes", "Garden note", "Text and garden area", "Kept until the owner removes it", "Weather", "Attach conditions", "Never send note text", "Bed", "A garden growing area", "Local data", "Save locally", "Offline access", "Cloud-only storage", "How long are notes kept?", "Until manually removed"]) {
      expect(within(combined).getAllByText(text).length).toBeGreaterThan(0);
    }
    expect(within(combined).getByText("Acceptance scenario", { selector: "dt" })).not.toBeNull();
    expect(within(combined).getByRole("list", { name: "Related requirements" }).textContent).toContain("Save a note AC-1");
    expect(combined.querySelector("pre")).toBeNull();
    expect(within(combined).queryByText("saved-fingerprint")).toBeNull();
    await expand("Recorded revision details");
    expect(await within(combined).findByText("saved-fingerprint")).not.toBeNull();
    expect(within(combined).getByText("Saved revision").nextElementSibling?.textContent).toBe("3");
    await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
    await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
    expect(restore).toHaveBeenCalledWith({ ...saved, constraints: "Alice needs keyboard access" });
  });

  it("compares readable versions first and exposes exact JSON only through a separate disclosure", async () => {
    const baseline = definition();
    const local = { ...baseline, purpose: "Alice's idea" };
    const saved = { ...baseline, purpose: "Bob's idea", revision: 3 };
    const view = render(<DraftRecovery merge draft={recovery(local, baseline)} label="Definition" base="current" saved={saved} onRestore={vi.fn()} />);
    const comparison = await expand("Compare draft with saved version");
    await within(comparison).findByText("Alice's idea");
    expect(within(comparison).getByText("Bob's idea")).not.toBeNull();
    expect(within(comparison).getAllByText("The idea")).toHaveLength(2);
    expect(view.container.querySelectorAll("pre")).toHaveLength(0);
    await expand("Original technical values");
    const raw = comparison.querySelectorAll("pre");
    expect(raw).toHaveLength(2);
    expect(JSON.parse(raw[0].textContent!)).toEqual(saved);
    expect(JSON.parse(raw[1].textContent!)).toEqual(local);
  });

  it("explains removal versus an edited record and retains explicit choice and consent", async () => {
    const baseline = { tasks: [{ id: "task-1", title: "Save work", status: "todo" }] };
    const local = { tasks: [] as typeof baseline.tasks };
    const saved = { tasks: [{ ...baseline.tasks[0], title: "Save notes offline", status: "doing" }] };
    const restore = vi.fn();
    render(<DraftRecovery merge draft={recovery(local, baseline)} label="Implementation plan" base="current" saved={saved} onRestore={restore} />);
    expect(screen.getByText("Removed in your draft")).not.toBeNull();
    expect(screen.getByText("Save notes offline")).not.toBeNull();
    expect(screen.getByText("In progress")).not.toBeNull();
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
    await expand("Before these edits: Tasks · Save work");
    expect(await screen.findByText("Not started")).not.toBeNull();
    await userEvent.click(screen.getByRole("radio", { name: "Use my draft for Tasks · Save work" }));
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
    await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
    await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
    expect(restore).toHaveBeenCalledWith(local);
  });

  it("names records in competing order choices and includes additions in the recovered plan", async () => {
    const tasks = ["Write a note", "Save a note", "Reopen a note"].map((title, index) => ({ id: `task-${index}`, title, status: "todo" }));
    const baseline = { tasks, title: "" };
    const local = { tasks: [tasks[1], tasks[0], tasks[2], { id: "alice", title: "Alice's added check", status: "todo" }], title: "Draft task not added yet" };
    const saved = { tasks: [tasks[0], tasks[2], tasks[1], { id: "bob", title: "Bob's added check", status: "todo" }], title: "" };
    const restore = vi.fn();
    render(<DraftRecovery merge draft={recovery(local, baseline)} label="Implementation plan" base="current" saved={saved} onRestore={restore} />);
    const orders = screen.getAllByRole("list", { name: "Record order" });
    expect(within(orders[0]).getAllByRole("listitem").map(item => item.textContent)).toEqual(["Write a note task-0", "Reopen a note task-2", "Save a note task-1", "Bob's added check bob"]);
    expect(within(orders[1]).getAllByRole("listitem")[0].textContent).toBe("Save a note task-1");
    await userEvent.click(screen.getByRole("radio", { name: "Use my draft for Tasks · Order" }));
    const combined = await expand("Review combined draft");
    await within(combined).findByText("Alice's added check");
    expect(within(combined).getByText("Bob's added check")).not.toBeNull();
    expect(within(combined).getByText("Task being added").nextElementSibling?.textContent).toBe("Draft task not added yet");
    await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
    await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
    expect(restore.mock.calls[0][0].tasks.map((task: { id: string }) => task.id).sort()).toEqual(["alice", "bob", "task-0", "task-1", "task-2"]);
  });

  it("renders screen states, widths, design tokens, assets and prototype provenance without losing fields", async () => {
    const baseline = {
      screens: [{ id: "home", name: "Garden", route: "/garden", purpose: "Read and write notes", states: ["Loading", "Offline recovery"], widths: [390, 1440], requirementIds: ["AC-1"], accessibility: "Keyboard focus stays visible" }],
      tokens: [{ name: "surface", value: "#101010", purpose: "Reading background" }],
      assets: [{ id: "logo", path: "assets/logo.svg", purpose: "Garden identity", licence: "MIT, created by the team" }],
      implementationNotes: "Use saved local notes while offline", prototypeReleaseId: "release-7", prototypeCommit: "abc123", customBoundary: { enabled: false, note: "Preserve this additional context" },
    };
    const local = { ...baseline, implementationNotes: "Alice's implementation notes" };
    const saved = { ...baseline, screens: [{ ...baseline.screens[0], accessibility: "Bob requires keyboard and zoom support" }] };
    render(<DraftRecovery merge draft={recovery(local, baseline)} label="Design plan" base="current" saved={saved} onRestore={vi.fn()} />);
    const combined = await expand("Review combined draft");
    await within(combined).findByText("Alice's implementation notes");
    for (const text of ["Garden", "/garden", "Read and write notes", "Loading", "Offline recovery", "390 px", "1440 px", "AC-1", "Bob requires keyboard and zoom support", "surface", "#101010", "Reading background", "assets/logo.svg", "Garden identity", "MIT, created by the team", "release-7", "abc123", "No", "Preserve this additional context"]) {
      expect(within(combined).getByText(text)).not.toBeNull();
    }
    expect(combined.querySelector("pre")).toBeNull();
  });

  it("keeps legacy differences explicit and invalidates choices and consent when saved context changes", async () => {
    const local = { purpose: "Older idea", constraints: "Offline" };
    const saved = { purpose: "New idea", constraints: "Online" };
    const draft = recovery(local);
    const restore = vi.fn();
    const view = render(<DraftRecovery merge draft={draft} label="Definition" base="current" saved={saved} onRestore={restore} />);
    expect(screen.getByText(/older browser draft does not include its original/)).not.toBeNull();
    const combined = await expand("Review combined draft");
    expect(await within(combined).findByText(/this preview shows the saved version for undecided differences/)).not.toBeNull();
    await userEvent.click(screen.getByRole("radio", { name: "Keep saved version for The idea" }));
    await userEvent.click(screen.getByRole("radio", { name: "Use my draft for Constraints" }));
    await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(false);
    view.rerender(<DraftRecovery merge draft={draft} label="Definition" base="newer" saved={{ ...saved, purpose: "Another saved idea" }} onRestore={restore} />);
    expect((screen.getByRole("checkbox", { name: /I reviewed the saved version/ }) as HTMLInputElement).checked).toBe(false);
    expect(screen.getAllByRole("radio").every(radio => !(radio as HTMLInputElement).checked)).toBe(true);
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
    expect(restore).not.toHaveBeenCalled();
  });

  it("keeps read-only choices disabled and opaque sketch comparison as a raw fallback", async () => {
    const local = { purpose: "Mine" }, saved = { purpose: "Saved" };
    const view = render(<DraftRecovery merge readOnly draft={recovery(local)} label="Definition" base="current" saved={saved} onRestore={vi.fn()} />);
    expect(screen.getByRole("radio", { name: "Use my draft for The idea" }).closest("fieldset")?.disabled).toBe(true);
    const scene = { elements: [{ id: "shape", x: 10 }], appState: {}, files: {} };
    const sketchDraft = recovery(scene);
    view.rerender(<DraftRecovery draft={sketchDraft} label="Sketch" base="current" saved={{ ...scene, elements: [] }} onRestore={vi.fn()} />);
    const compare = await expand("Compare draft with saved version");
    expect(compare.querySelectorAll("pre")).toHaveLength(2);
    expect(compare.textContent).toContain('"x": 10');
    expect(screen.queryByText("Review combined draft")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Discard browser draft" }));
    expect(sketchDraft.clear).toHaveBeenCalledOnce();
  });

  it("does not claim an unknown-baseline record was deleted when it is merely absent", () => {
    const local = { tasks: [] as { id: string; title: string }[] };
    const saved = { tasks: [{ id: "new", title: "A teammate's task" }] };
    render(<DraftRecovery merge draft={recovery(local)} label="Implementation plan" base="current" saved={saved} onRestore={vi.fn()} />);
    expect(screen.getByText("Not present in your draft")).not.toBeNull();
    expect(screen.queryByText("Removed in your draft")).toBeNull();
    expect(screen.queryByText(/Before these edits:/)).toBeNull();
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
