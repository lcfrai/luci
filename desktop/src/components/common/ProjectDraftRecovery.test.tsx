import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { DefinitionWorkspace } from "../definition/DefinitionWorkspace";
import DesignWorkspace from "../design/DesignWorkspace";
import DesignEditor from "../journey/DesignEditor";
import { ProjectPlan } from "../journey/ProjectPlan";
import { DraftNavigationDialog } from "./DraftNavigationDialog";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { productApi, type Brief, type DesignScene, type ImplementationPlan } from "../../api/product";
import { selectTeamScope } from "../../api/teamScope";
import { resetCockpit, useCockpit } from "../../store/store";

const identity = vi.hoisted(() => ({ actor: "alice", workspace: "local", remoteUser: "alice", root: "/projects/alice" }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => identity.actor ? { profile: { user: { id: identity.actor } } } : null }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ workspace: { mode: identity.workspace === "local" ? "local" : "cloud", connectionId: identity.workspace, host: identity.workspace }, status: { config: { projectsDir: identity.root }, access: { user: { username: identity.remoteUser } } } }) }));
const canvas = vi.hoisted(() => ({ change: null as ((elements: unknown[], state: object, files: object) => void) | null }));
vi.mock("@excalidraw/excalidraw", async () => {
  const { useEffect } = await import("react");
  return { Excalidraw: ({ initialData, onChange }: { initialData: DesignScene; onChange: typeof canvas.change }) => {
    canvas.change = onChange;
    useEffect(() => { onChange?.(initialData.elements, initialData.appState, initialData.files); }, []);
    return <div aria-label="Sketch document">{JSON.stringify(initialData.elements)}</div>;
  }, serializeAsJSON: (elements: unknown[], appState: object, files: object) => JSON.stringify({ elements, appState, files }),
  MainMenu: Object.assign(() => null, { DefaultItems: { Export: () => null, SaveAsImage: () => null, ClearCanvas: () => null, ChangeCanvasBackground: () => null } }) };
});
const specification = (): ProjectSpecification => ({ projectId: "p-1", version: 1, revision: 2, briefRevision: 3, startingPoint: "new", purpose: "Saved idea", audience: "People", constraints: "Offline", outOfScope: [], requirements: [{ id: "AC-1", title: "Save work", priority: "must", acceptance: "Reopened work returns" }], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: "draft", digest: "", updatedAt: "" });
const designPlan = (): DesignPlan => ({ projectId: "p-1", revision: 4, briefRevision: 3, specificationRevision: 2, sketchRevision: 1, screens: [{ id: "home", name: "Saved screen", route: "/", purpose: "Save work", states: ["Loading"], widths: [390], requirementIds: ["AC-1"], accessibility: "Keyboard" }], tokens: [], assets: [], implementationNotes: "Saved notes", prototypeReleaseId: "", prototypeCommit: "", status: "draft", digest: "", updatedAt: "" });
const brief: Brief = { projectId: "p-1", revision: 3, purpose: "Saved idea", audience: "People", constraints: "", criteria: [], updatedAt: "" };
const plan: ImplementationPlan = { projectId: "p-1", revision: 5, briefRevision: 3, tasks: [{ id: "build", title: "Saved task", status: "todo" }], updatedAt: "" };
const scene: DesignScene = { elements: [{ id: "shape", x: 1 }], appState: {}, files: {} };
function edit(name: string, value: string) { fireEvent.change(screen.getByRole("textbox", { name }), { target: { value } }); }
function storedDrafts() { return Object.keys(localStorage).filter(key => key.startsWith("luci.project-draft.v1:")); }

beforeEach(() => {
  vi.restoreAllMocks(); selectTeamScope(""); localStorage.clear(); resetCockpit();
  Object.assign(identity, { actor: "alice", workspace: "local", remoteUser: "alice", root: "/projects/alice" });
  vi.spyOn(definitionApi, "get").mockImplementation(async id => ({ ...specification(), projectId: id }));
  vi.spyOn(definitionApi, "listAssistance").mockResolvedValue({ jobs: [] });
  vi.spyOn(productApi, "agentConnections").mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
  vi.spyOn(productApi, "agents").mockResolvedValue({ providers: [] });
  vi.spyOn(designPlanApi, "get").mockResolvedValue(designPlan());
  vi.spyOn(productApi, "prototypeReleases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "plan").mockResolvedValue(plan);
  vi.spyOn(productApi, "checks").mockResolvedValue({ runs: [], suggested: [] });
  vi.spyOn(productApi, "design").mockResolvedValue({ projectId: "p-1", revision: 2, briefRevision: 3, frames: [], scene, updatedAt: "" });
});
afterEach(() => { cleanup(); selectTeamScope(""); localStorage.clear(); vi.restoreAllMocks(); });

it("recovers a definition only after an explicit restore, keeps its stable requirement IDs, and clears it after a successful save", async () => {
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, value) => ({ ...value, revision: 3, briefRevision: 4 }));
  const view = render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea"); edit("The idea", "My unsaved idea"); edit("Acceptance scenario 1", "Recover work after reload");
  expect(storedDrafts()).toHaveLength(1); view.unmount();
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea");
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Saved idea");
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("My unsaved idea");
  expect(screen.getByText(/Recovered draft · kept in this browser/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await waitFor(() => expect(storedDrafts()).toHaveLength(0));
  expect(save.mock.calls[0][1].requirements[0]).toMatchObject({ id: "AC-1", acceptance: "Recover work after reload" });
  expect(save.mock.calls[0].slice(2)).toEqual([2, 3]);
});

it("retains conflict edits while comparing the newer saved definition, and requires review before rebasing a manual save", async () => {
  const save = vi.spyOn(definitionApi, "save").mockRejectedValueOnce(new Error("Revision conflict: another person saved this definition"));
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea"); edit("The idea", "My conflicting draft");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await screen.findByText(/Revision conflict:/);
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("My conflicting draft");
  vi.mocked(definitionApi.get).mockResolvedValue({ ...specification(), purpose: "Colleague saved idea", revision: 3, briefRevision: 4 });
  await userEvent.click(screen.getByRole("button", { name: "Reload saved definition" }));
  await waitFor(() => expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Colleague saved idea"));
  expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByText("Compare draft with saved version"));
  const recovery = screen.getByRole("region", { name: "Definition draft recovery" });
  expect(recovery.textContent).toContain("Colleague saved idea"); expect(recovery.textContent).toContain("My conflicting draft");
  expect((screen.getByRole("checkbox", { name: /I reviewed the saved version/ }) as HTMLInputElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("radio", { name: /Use my draft for The idea/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect(save).toHaveBeenCalledTimes(1);
  save.mockImplementation(async (_id, value) => ({ ...value, revision: 4, briefRevision: 5 }));
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(save.mock.calls[1][1].purpose).toBe("My conflicting draft"); expect(save.mock.calls[1].slice(2)).toEqual([3, 4]);
});

it("blocks definition saved-read recovery until the fresh revision is available, then merges and saves explicitly", async () => {
  const save = vi.spyOn(definitionApi, "save").mockRejectedValueOnce(new Error("Revision conflict"));
  const approve = vi.spyOn(definitionApi, "approve");
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("Boundaries and constraints"); edit("Boundaries and constraints", "Alice requires keyboard access");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await screen.findByText(/Revision conflict/);
  let finish!: (value: ProjectSpecification) => void;
  vi.mocked(definitionApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const key = storedDrafts()[0], retained = localStorage.getItem(key);
  const reload = screen.getByRole("button", { name: "Reload saved definition" });
  await userEvent.click(reload);
  const restore = screen.getByRole("button", { name: "Restore draft" });
  expect((restore as HTMLButtonElement).disabled).toBe(true);
  expect((reload as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Discard browser draft" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("Boundaries and constraints").closest("fieldset") as HTMLFieldSetElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Save definition" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText("Loading saved definition… Your draft is kept while the current revision is checked.")).not.toBeNull();
  await userEvent.click(reload); await userEvent.click(restore);
  expect(definitionApi.get).toHaveBeenCalledTimes(2);
  expect(save).toHaveBeenCalledOnce(); expect(approve).not.toHaveBeenCalled();
  expect(localStorage.getItem(key)).toBe(retained);
  const bob = { ...specification(), revision: 3, briefRevision: 4, requirements: [{ ...specification().requirements[0], title: "Bob's agreed outcome" }] };
  await act(async () => finish(bob));
  expect((restore as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(restore);
  expect((screen.getByLabelText("Boundaries and constraints") as HTMLTextAreaElement).value).toBe("Alice requires keyboard access");
  expect((screen.getByLabelText("Requirement 1") as HTMLInputElement).value).toBe("Bob's agreed outcome");
  save.mockImplementation(async (_id, value) => ({ ...value, revision: 4, briefRevision: 5 }));
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(save.mock.calls[1][1]).toMatchObject({ constraints: "Alice requires keyboard access", requirements: bob.requirements });
  expect(save.mock.calls[1].slice(2)).toEqual([3, 4]);
  expect(storedDrafts()).toHaveLength(0);
});

it("keeps definition saved-read recovery blocked after failure and retains the draft for an explicit retry", async () => {
  vi.spyOn(definitionApi, "save").mockRejectedValueOnce(new Error("Revision conflict"));
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea"); edit("The idea", "My conflict draft");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await screen.findByText(/Revision conflict/);
  const key = storedDrafts()[0], retained = localStorage.getItem(key);
  vi.mocked(definitionApi.get).mockRejectedValueOnce(new Error("Saved definition is temporarily unavailable"));
  await userEvent.click(screen.getByRole("button", { name: "Reload saved definition" }));
  await screen.findByText(/Saved definition is temporarily unavailable/);
  expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Save definition" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Reload saved definition" }) as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("My conflict draft");
  expect(localStorage.getItem(key)).toBe(retained);
  vi.mocked(definitionApi.get).mockResolvedValueOnce({ ...specification(), revision: 3, briefRevision: 4, audience: "Bob's audience" });
  await userEvent.click(screen.getByRole("button", { name: "Reload saved definition" }));
  await screen.findByRole("checkbox", { name: /I reviewed the saved version/ });
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("My conflict draft");
  expect((screen.getByLabelText("Who it is for") as HTMLTextAreaElement).value).toBe("Bob's audience");
  expect(screen.queryByRole("alert")).toBeNull();
});

it("blocks definition approval during a saved-read retry even without an edited browser draft", async () => {
  const approve = vi.spyOn(definitionApi, "approve").mockRejectedValueOnce(new Error("Definition changed before approval"));
  render(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Approve definition" }));
  await screen.findByText(/Definition changed before approval/);
  let finish!: (value: ProjectSpecification) => void;
  vi.mocked(definitionApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Reload saved definition" }));
  const action = screen.getByRole("button", { name: "Approve definition" });
  expect((action as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("The idea").closest("fieldset") as HTMLFieldSetElement).disabled).toBe(true);
  await userEvent.click(action); expect(approve).toHaveBeenCalledOnce();
  expect(storedDrafts()).toHaveLength(0);
  await act(async () => finish({ ...specification(), revision: 3, briefRevision: 4 }));
  expect((action as HTMLButtonElement).disabled).toBe(false);
  approve.mockResolvedValueOnce({ ...specification(), revision: 3, briefRevision: 4, status: "approved" });
  await userEvent.click(action);
  expect(approve).toHaveBeenLastCalledWith("p-1", 3, 4);
});

it.each(["success", "error"] as const)("ignores superseded definition saved-read %s after effect restart and a newer edit", async outcome => {
  let finish!: (value: ProjectSpecification) => void, reject!: (error: Error) => void;
  vi.mocked(definitionApi.get).mockImplementationOnce(() => new Promise((resolve, fail) => { finish = resolve; reject = fail; }))
    .mockResolvedValueOnce({ ...specification(), revision: 3, briefRevision: 4 });
  render(<StrictMode><DefinitionWorkspace projectId="p-1" /></StrictMode>);
  await screen.findByLabelText("The idea"); edit("The idea", "Newer local edit");
  const retained = localStorage.getItem(storedDrafts()[0]);
  await act(async () => { if (outcome === "success") finish(specification()); else reject(new Error("Obsolete saved-read failure")); });
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Newer local edit");
  expect(screen.getByText("Unsaved changes")).not.toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(localStorage.getItem(storedDrafts()[0])).toBe(retained);
});

it("recovers Alice's constraint without overwriting Bob's newer outcome and saves against the current revision", async () => {
  const save = vi.spyOn(definitionApi, "save").mockRejectedValueOnce(new Error("Revision conflict"));
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("Boundaries and constraints"); edit("Boundaries and constraints", "Alice requires keyboard access");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await screen.findByText(/Revision conflict/);
  const bob = { ...specification(), revision: 3, briefRevision: 4, requirements: [{ ...specification().requirements[0], title: "Save a note with its garden area", acceptance: "Reopened notes retain the chosen garden area" }] };
  vi.mocked(definitionApi.get).mockResolvedValue(bob);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved definition" }));
  await waitFor(() => expect((screen.getByLabelText("Requirement 1") as HTMLInputElement).value).toBe(bob.requirements[0].title));
  expect(screen.queryByRole("radio")).toBeNull();
  expect(screen.getByText(/Edits to different fields are combined/)).not.toBeNull();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Boundaries and constraints") as HTMLTextAreaElement).value).toBe("Alice requires keyboard access");
  expect((screen.getByLabelText("Acceptance scenario 1") as HTMLTextAreaElement).value).toBe(bob.requirements[0].acceptance);
  save.mockImplementation(async (_id, value) => ({ ...value, revision: 4, briefRevision: 5 }));
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(save.mock.calls[1][1]).toMatchObject({ constraints: "Alice requires keyboard access", requirements: bob.requirements });
  expect(save.mock.calls[1].slice(2)).toEqual([3, 4]);
});

it("requires explicit field choices for legacy stale drafts instead of treating a newer saved document as their baseline", async () => {
  const save = vi.spyOn(definitionApi, "save");
  const first = render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("Boundaries and constraints"); edit("Boundaries and constraints", "Alice requires keyboard access"); first.unmount();
  const key = storedDrafts()[0], legacy = JSON.parse(localStorage.getItem(key)!);
  delete legacy.original; localStorage.setItem(key, JSON.stringify(legacy));
  const bob = { ...specification(), revision: 3, briefRevision: 4, purpose: "Bob's updated idea" };
  vi.mocked(definitionApi.get).mockResolvedValue(bob);
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  expect(screen.getByText(/older browser draft does not include its original/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("radio", { name: /Keep saved version for The idea/ }));
  await userEvent.click(screen.getByRole("radio", { name: /Use my draft for Constraints/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe(bob.purpose);
  expect((screen.getByLabelText("Boundaries and constraints") as HTMLTextAreaElement).value).toBe("Alice requires keyboard access");
  expect(save).not.toHaveBeenCalled();
});

it("makes a merged journey's removed outcome link visible and requires an explicit remedy without dropping the teammate's journey", async () => {
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, value) => ({ ...value, revision: 4, briefRevision: 5 }));
  const first = render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("button", { name: "Remove requirement 1" })); first.unmount();
  const replacement = { id: "AC-2", title: "Save with a garden area", acceptance: "The garden area is retained", priority: "must" as const };
  vi.mocked(definitionApi.get).mockResolvedValue({ ...specification(), revision: 3, briefRevision: 4, requirements: [...specification().requirements, replacement], journeys: [{ id: "bob-journey", name: "Bob's note workflow", actor: "Gardener", steps: "Save, reopen and review the note", requirementIds: ["AC-1"] }] });
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect(screen.getByRole("alert").textContent).toContain("outcome that was removed");
  expect((screen.getByRole("button", { name: "Save definition" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("Journey name") as HTMLInputElement).value).toBe("Bob's note workflow");
  await userEvent.click(screen.getByRole("checkbox", { name: /AC-2 · Save with a garden area/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: "Remove unavailable outcome AC-1 from journey 1" }));
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  expect(save.mock.calls[0][1].journeys).toEqual([{ id: "bob-journey", name: "Bob's note workflow", actor: "Gardener", steps: "Save, reopen and review the note", requirementIds: ["AC-2"] }]);
});

it("isolates drafts when accounts, remote accounts, workspaces, project roots, API endpoints, or projects change", async () => {
  const view = render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea"); edit("The idea", "Private Alice draft");
  for (const [key, next] of [["actor", "bob"], ["workspace", "other-host"], ["remoteUser", "remote-bob"], ["root", "/new-root"]] as const) {
    const previous = identity[key]; identity[key] = next;
    view.rerender(<DefinitionWorkspace projectId="p-1" />);
    await screen.findByLabelText("The idea");
    expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull();
    expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Saved idea");
    identity[key] = previous;
  }
  const endpoint = vi.spyOn(productApi, "baseUrl", "get").mockReturnValue("https://other-service.invalid");
  view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull(); endpoint.mockRestore();
  view.rerender(<DefinitionWorkspace projectId="p-2" />); await screen.findByLabelText("The idea");
  expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull();
  view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Private Alice draft");
});

it("retains structured design edits through remount and discards a browser draft without saving it", async () => {
  const save = vi.spyOn(designPlanApi, "save");
  const view = render(<DesignWorkspace projectId="p-1" />);
  await screen.findByLabelText("Screen name"); edit("Screen name", "Unsaved screen"); view.unmount();
  const recovered = render(<DesignWorkspace projectId="p-1" />); await screen.findByLabelText("Screen name");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Screen name") as HTMLInputElement).value).toBe("Unsaved screen"); recovered.unmount();
  render(<DesignWorkspace projectId="p-1" />); await screen.findByLabelText("Screen name");
  await userEvent.click(screen.getByRole("button", { name: "Discard browser draft" }));
  expect((screen.getByLabelText("Screen name") as HTMLInputElement).value).toBe("Saved screen");
  expect(storedDrafts()).toHaveLength(0); expect(save).not.toHaveBeenCalled();
});

it("recovers task edits and an unfinished new task without starting implementation", async () => {
  const save = vi.spyOn(productApi, "savePlan");
  const view = render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByLabelText("Task 1"); edit("Task 1", "Unsaved implementation task");
  fireEvent.change(screen.getByPlaceholderText("Describe a concrete outcome"), { target: { value: "Not added yet" } }); view.unmount();
  render(<ProjectPlan projectId="p-1" brief={brief} />); await screen.findByLabelText("Task 1");
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Saved task");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Unsaved implementation task");
  expect((screen.getByPlaceholderText("Describe a concrete outcome") as HTMLInputElement).value).toBe("Not added yet");
  expect(save).not.toHaveBeenCalled();
});

it("preserves a teammate's newly added task when recovering an independently edited task", async () => {
  const save = vi.spyOn(productApi, "savePlan");
  const first = render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByLabelText("Task 1"); edit("Task 1", "Alice's revised implementation task"); first.unmount();
  vi.mocked(productApi.plan).mockResolvedValue({ ...plan, revision: 6, tasks: [...plan.tasks, { id: "test-area", title: "Bob's garden-area regression test", status: "todo" }] });
  render(<ProjectPlan projectId="p-1" brief={brief} />); await screen.findByLabelText("Task 2");
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Alice's revised implementation task");
  expect((screen.getByLabelText("Task 2") as HTMLInputElement).value).toBe("Bob's garden-area regression test");
  expect(save).not.toHaveBeenCalled();
});

it("recovers a normalized sketch without counting hydration as an edit or automatically saving it", async () => {
  const save = vi.spyOn(productApi, "saveDesign");
  const view = render(<DesignEditor projectId="p-1" />);
  await screen.findByLabelText("Sketch document"); expect(storedDrafts()).toHaveLength(0);
  act(() => canvas.change?.([{ id: "shape", x: 99 }], {}, {})); view.unmount();
  render(<DesignEditor projectId="p-1" />); await screen.findByLabelText("Sketch document");
  expect(screen.getByLabelText("Sketch document").textContent).toContain('"x":1');
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect(screen.getByLabelText("Sketch document").textContent).toContain('"x":99');
  expect((screen.getByRole("button", { name: "Save design" }) as HTMLButtonElement).disabled).toBe(false);
  expect(save).not.toHaveBeenCalled();
});

it("keeps a draft through guarded navigation and explicitly deletes it on discard navigation", async () => {
  useCockpit.getState().selectProject("p-1"); useCockpit.getState().setLens("discover");
  const view = render(<><DefinitionWorkspace projectId="p-1" /><DraftNavigationDialog /></>);
  await screen.findByLabelText("The idea"); edit("The idea", "Navigate with this draft");
  act(() => useCockpit.getState().setLens("develop"));
  await userEvent.click(screen.getByRole("button", { name: "Keep drafts and continue" }));
  expect(useCockpit.getState().lens).toBe("develop"); view.unmount();
  const next = render(<><DefinitionWorkspace projectId="p-1" /><DraftNavigationDialog /></>); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  act(() => useCockpit.getState().setLens("design"));
  await userEvent.click(screen.getByRole("button", { name: "Discard edits and continue" }));
  next.unmount(); expect(storedDrafts()).toHaveLength(0);
});

it("reports unavailable storage truthfully and offers no keep-and-leave action", async () => {
  const write = Storage.prototype.setItem;
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (this: Storage, key, value) { if (key.startsWith("luci.project-draft.v1:")) throw new DOMException("Full", "QuotaExceededError"); write.call(this, key, value); });
  useCockpit.getState().selectProject("p-1");
  render(<><DefinitionWorkspace projectId="p-1" /><DraftNavigationDialog /></>);
  await screen.findByLabelText("The idea"); edit("The idea", "Still in memory");
  expect((await screen.findByRole("alert")).textContent).toContain("Browser storage is unavailable or full");
  act(() => useCockpit.getState().setLens("design"));
  expect(screen.queryByRole("button", { name: "Keep drafts and continue" })).toBeNull();
});

it("does not persist unidentified accounts or oversized drafts", async () => {
  identity.actor = "";
  const view = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "Unidentified draft");
  expect(storedDrafts()).toHaveLength(0); view.unmount(); identity.actor = "alice";
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "x".repeat(1024 * 1024 + 1));
  expect((await screen.findByRole("alert")).textContent).toContain("too large"); expect(storedDrafts()).toHaveLength(0);
});

it("preserves a newer draft written in another tab when a late save clears this editor", async () => {
  let finish!: (value: ProjectSpecification) => void;
  vi.spyOn(definitionApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "First tab");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  const key = storedDrafts()[0], record = JSON.parse(localStorage.getItem(key)!); record.value.purpose = "Second tab"; localStorage.setItem(key, JSON.stringify(record));
  await act(async () => finish({ ...specification(), purpose: "First tab", revision: 3, briefRevision: 4 })); view.unmount();
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByText("Compare draft with saved version", { selector: "summary" }));
  expect(within(screen.getByRole("region", { name: "Definition draft recovery" })).getByText(/Second tab/)).not.toBeNull();
});

it("does not offer an expired browser draft and never evicts a fresh draft to make room", async () => {
  const first = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "Expired work"); first.unmount();
  const expiredKey = storedDrafts()[0], expired = JSON.parse(localStorage.getItem(expiredKey)!);
  expired.updatedAt = Date.now() - 31 * 24 * 60 * 60 * 1000; localStorage.setItem(expiredKey, JSON.stringify(expired));
  const second = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull(); expect(storedDrafts()).toHaveLength(0);
  // Other fresh documents already consume this feature's bounded budget.
  const fresh = JSON.stringify({ ...expired, updatedAt: Date.now(), value: { ...specification(), purpose: "z".repeat(1050000) } });
  localStorage.setItem("luci.project-draft.v1:other-document-1", fresh);
  localStorage.setItem("luci.project-draft.v1:other-document-2", fresh);
  edit("The idea", "Keep this edit in memory when browser recovery is full");
  expect((await screen.findByRole("alert")).textContent).toContain("Browser draft storage is full");
  expect(localStorage.getItem("luci.project-draft.v1:other-document-1")).toBe(fresh);
  expect(localStorage.getItem("luci.project-draft.v1:other-document-2")).toBe(fresh);
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toContain("Keep this edit"); second.unmount();
});

it("keeps an account's unsaved draft when its save finishes after switching accounts", async () => {
  let finish!: (value: ProjectSpecification) => void;
  vi.spyOn(definitionApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "Alice pending save");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  identity.actor = "bob"; view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await act(async () => finish({ ...specification(), purpose: "Alice pending save", revision: 3, briefRevision: 4 }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Saved idea");
  expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull();
  identity.actor = "alice"; view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByText("Compare draft with saved version", { selector: "summary" }));
  expect(screen.getByRole("region", { name: "Definition draft recovery" }).textContent).toContain("Alice pending save");
});

it("keeps personal and selected team drafts separate through the actual API workspace prefix", async () => {
  const view = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea"); edit("The idea", "Personal-only draft");
  selectTeamScope("team-aaaaaaaaaaaaaaaaaaaaaa"); view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  expect(screen.queryByRole("button", { name: "Restore draft" })).toBeNull(); edit("The idea", "Shared workspace draft");
  selectTeamScope(""); view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Personal-only draft");
  selectTeamScope("team-aaaaaaaaaaaaaaaaaaaaaa"); view.rerender(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Shared workspace draft");
});
