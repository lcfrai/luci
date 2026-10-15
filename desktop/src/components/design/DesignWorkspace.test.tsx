import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { definitionApi } from "../../api/definition";
import { productApi, type Release } from "../../api/product";
import * as browserAccess from "../../api/browserAccess";
import { platformApi, type PlatformProject } from "../../api/platform";
import DesignWorkspace from "./DesignWorkspace";
import { resetCockpit, useCockpit } from "../../store/store";
import { DEFAULT_BASE_URL } from "../../api/client";
import * as projectDraft from "../../hooks/useProjectDraft";
import { notifyProjectContextChanged } from "../../api/projectContextEvents";

vi.mock("../journey/DesignEditor", () => ({ default: ({ onDirtyChange, onBusyChange }: { onDirtyChange?(dirty: boolean): void; onBusyChange?(busy: boolean): void }) => <div>Drawing tools<button onClick={() => onDirtyChange?.(true)}>Edit sketch fixture</button><button onClick={() => onDirtyChange?.(false)}>Save sketch fixture</button><button onClick={() => onBusyChange?.(true)}>Begin sketch save fixture</button><button onClick={() => onBusyChange?.(false)}>Complete sketch save fixture</button></div> }));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
beforeEach(() => resetCockpit());
const fixture = (): DesignPlan => ({ projectId:"p-1", revision:2, briefRevision:1, specificationRevision:2, sketchRevision:0, screens:[{id:"screen-1",name:"Home",route:"/",purpose:"Save work",states:["Empty","Saved"],widths:[390,1440],requirementIds:["AC-001"],accessibility:"Keyboard"}],tokens:[],assets:[],implementationNotes:"Saved notes",prototypeReleaseId:"",prototypeCommit:"",status:"draft",digest:"",updatedAt:"" });
async function openScreenPlan() {
  const tab = await screen.findByRole('tab', {name: 'Screens and states'});
  await waitFor(() => expect(tab.getAttribute('aria-disabled')).not.toBe('true'));
  if (tab.getAttribute('aria-selected') !== 'true') await userEvent.click(tab);
}
async function screenPlanField(name: string) { await openScreenPlan(); return screen.findByRole('textbox', {name}); }
it("refreshes a clean Design editor after same-window definition approval before new edits", async () => {
  ready();
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue("design-context-clean");
  const initial = vi.mocked(definitionApi.get).getMockImplementation()!;
  vi.mocked(definitionApi.get).mockImplementationOnce(initial).mockImplementation(async projectId => ({ ...await initial(projectId), revision: 3 }));
  vi.mocked(designPlanApi.get).mockResolvedValueOnce(fixture()).mockResolvedValue({ ...fixture(), status: "outdated" });
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, plan) => ({ ...plan, revision: 3 }));
  render(<DesignWorkspace projectId="p-1" />);
  await screenPlanField('Screen name');
  await act(async () => notifyProjectContextChanged("design-context-clean", "p-1"));
  await screen.findByText("Definition 3 · approved");
  expect(designPlanApi.get).toHaveBeenCalledTimes(2);
  const name = screen.getByRole("textbox", { name: "Screen name" });
  await userEvent.type(name, " refreshed");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  expect(save.mock.calls[0][3]).toBe(1);
  expect(save.mock.calls[0][4]).toBe(3);
});

it("keeps a dirty Design draft on a context event and preserves stale-save refusal", async () => {
  ready();
  const scope = "design-context-dirty";
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue(scope);
  const save = vi.spyOn(designPlanApi, "save").mockRejectedValue(new Error("The definition/design changed. Reload saved design before saving."));
  try {
    render(<DesignWorkspace projectId="p-1" />);
    const name = await screenPlanField('Screen name') as HTMLInputElement;
    await userEvent.type(name, " with my draft");
    await act(async () => notifyProjectContextChanged(scope, "p-1"));
    expect(designPlanApi.get).toHaveBeenCalledTimes(1);
    expect(name.value).toBe("Home with my draft");
    await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
    await screen.findByRole("button", { name: "Reload saved design" });
    expect(save).toHaveBeenCalledExactlyOnceWith("p-1", expect.objectContaining({ screens: [expect.objectContaining({ name: "Home with my draft" })] }), 2, 1, 2);
    expect(name.value).toBe("Home with my draft");
  } finally { localStorage.removeItem(`luci.project-draft.v1:${JSON.stringify([scope, "design-plan"])}`); }
});
it("retains newly typed handoff notes when the sketch refresh completes late", async () => {
  let finish!: (value: DesignPlan) => void;
  vi.spyOn(designPlanApi,"get").mockResolvedValueOnce(fixture()).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  vi.spyOn(definitionApi,"get").mockResolvedValue({projectId:"p-1",version:1,revision:2,briefRevision:1,startingPoint:"new",purpose:"Save work",audience:"People",constraints:"",outOfScope:[],requirements:[{id:"AC-001",title:"Save work",priority:"must",acceptance:"Reopened work returns"}],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:"approved",digest:"a".repeat(64),updatedAt:""});
  vi.spyOn(productApi,"prototypeReleases").mockResolvedValue({releases:[]});
  render(<DesignWorkspace projectId="p-1"/>);
  await screenPlanField('Screen name');
  await userEvent.click(screen.getByRole("tab",{name:"Sketch"}));
  await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("tab",{name:"Implementation handoff"}));
  const notes = screen.getByRole("textbox",{name:"Implementation notes"});
  await userEvent.clear(notes); await userEvent.type(notes,"Keep my unsaved keyboard behavior");
  await act(async () => { finish({...fixture(),status:"outdated"}); });
  expect((notes as HTMLTextAreaElement).value).toBe("Keep my unsaved keyboard behavior");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

function ready(plan = fixture()) {
  vi.spyOn(designPlanApi, "get").mockResolvedValue(plan);
  vi.spyOn(definitionApi, "get").mockResolvedValue({ projectId:"p-1", version:1, revision:2, briefRevision:1, startingPoint:"new", purpose:"Save work", audience:"People", constraints:"", outOfScope:[], requirements:[{id:"AC-001", title:"Save work", priority:"must", acceptance:"Reopened work returns"}], journeys:[], roles:[], entities:[], integrations:[], glossary:[], decisions:[], questions:[], status:"approved", digest:"a".repeat(64), updatedAt:"" });
  vi.spyOn(productApi, "prototypeReleases").mockResolvedValue({ releases: [] });
}

it("keeps recovery and edits blocked until the saved design reload completes, then merges against its fresh revision", async () => {
  ready();
  const scope = "design-reload-recovery";
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue(scope);
  const key = `luci.project-draft.v1:${JSON.stringify([scope, "design-plan"])}`;
  const save = vi.spyOn(designPlanApi, "save").mockRejectedValueOnce(new Error("The saved design changed. Reload before saving."))
    .mockImplementation(async (_id, plan) => ({ ...plan, revision: 4 }));
  let finish!: (value: DesignPlan) => void;
  const user = userEvent.setup();
  try {
    render(<DesignWorkspace projectId="p-1" />);
    await user.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
    const notes = screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement;
    await user.clear(notes); await user.type(notes, "Keep my keyboard recovery steps");
    await user.click(screen.getByRole("button", { name: "Save design plan" }));
    const reload = await screen.findByRole("button", { name: "Reload saved design" });
    vi.mocked(designPlanApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    await user.click(reload);
    const restore = screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement;
    const restoreWasBlocked = restore.disabled;
    expect(screen.getByText("Loading saved design… Your draft stays here while the saved version is checked.")).not.toBeNull();
    expect(notes.matches(":disabled")).toBe(true);
    expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
    const retainedDuringRead = localStorage.getItem(key);
    await user.click(reload);
    expect(designPlanApi.get).toHaveBeenCalledTimes(2);
    await user.click(restore);
    await user.type(notes, " should not be admitted during reload");
    expect(localStorage.getItem(key)).toBe(retainedDuringRead);
    const peer = { ...fixture(), revision: 3, implementationNotes: "Teammate navigation rules", screens: [{ ...fixture().screens[0], name: "Teammate home" }] };
    await act(async () => finish(peer));
    expect(screen.queryByRole("region", { name: "Design plan draft recovery" }), `Recovery must remain offered after the read; visible notes: ${notes.value}`).not.toBeNull();
    expect(restoreWasBlocked).toBe(true);
    expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByRole("radio", { name: "Use my draft for Implementation notes" }));
    await user.click(screen.getByRole("checkbox", { name: "I reviewed the saved version and these recovery choices." }));
    await user.click(screen.getByRole("button", { name: "Restore draft" }));
    expect(notes.value).toBe("Keep my keyboard recovery steps");
    await user.click(screen.getByRole("tab", { name: "Screens and states" }));
    expect((screen.getByRole("textbox", { name: "Screen name" }) as HTMLInputElement).value).toBe("Teammate home");
    await user.click(screen.getByRole("button", { name: "Save design plan" }));
    expect(save).toHaveBeenCalledTimes(2);
    expect(save.mock.calls[1]).toEqual(["p-1", { ...peer, implementationNotes: "Keep my keyboard recovery steps", status: "draft", digest: "" }, 3, 1, 2]);
    expect(localStorage.getItem(key)).toBeNull();
  } finally { localStorage.removeItem(key); }
});

it("retains a design draft through a failed saved read and requires successful retry before recovery", async () => {
  ready();
  const scope = "design-reload-failed";
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue(scope);
  const key = `luci.project-draft.v1:${JSON.stringify([scope, "design-plan"])}`;
  vi.spyOn(designPlanApi, "save").mockRejectedValueOnce(new Error("The saved design changed."));
  const user = userEvent.setup();
  try {
    render(<DesignWorkspace projectId="p-1" />);
    await user.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
    const notes = screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement;
    await user.type(notes, " with local recovery instructions");
    const retained = localStorage.getItem(key);
    await user.click(screen.getByRole("button", { name: "Save design plan" }));
    vi.mocked(designPlanApi.get).mockRejectedValueOnce(new Error("Saved design is temporarily unavailable"));
    await user.click(await screen.findByRole("button", { name: "Reload saved design" }));
    await screen.findByText("Reload the saved design before editing or recovering your draft. Your existing edits are kept.");
    expect(screen.getByRole("alert").textContent).toContain("Saved design is temporarily unavailable");
    expect(localStorage.getItem(key)).toBe(retained);
    expect(notes.value).toBe("Saved notes with local recovery instructions");
    expect(notes.matches(":disabled")).toBe(true);
    for (const name of ["Restore draft", "Discard browser draft", "Save design plan"]) expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
    const retry = screen.getByRole("button", { name: "Reload saved design" }) as HTMLButtonElement;
    expect(retry.disabled).toBe(false);
    await user.click(retry);
    const restore = screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement;
    await waitFor(() => expect(restore.disabled).toBe(false));
    await user.click(restore);
    expect(notes.value).toBe("Saved notes with local recovery instructions");
    expect(notes.matches(":disabled")).toBe(false);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(designPlanApi.save).toHaveBeenCalledTimes(1);
  } finally { localStorage.removeItem(key); }
});

it("blocks edits and approval while a clean design reload is pending", async () => {
  ready();
  const approve = vi.spyOn(designPlanApi, "approve").mockRejectedValueOnce(new Error("The handoff changed. Reload before approval."));
  let finish!: (value: DesignPlan) => void;
  const user = userEvent.setup();
  render(<DesignWorkspace projectId="p-1" />);
  await openScreenPlan();
  await user.click(await screen.findByRole("button", { name: "Approve design handoff" }));
  vi.mocked(designPlanApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await user.click(await screen.findByRole("button", { name: "Reload saved design" }));
  const approval = screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement;
  const name = screen.getByRole("textbox", { name: "Screen name" }) as HTMLInputElement;
  expect(approval.disabled).toBe(true); expect(name.matches(":disabled")).toBe(true);
  await user.click(approval); await user.type(name, "Unadmitted name");
  expect(approve).toHaveBeenCalledTimes(1);
  expect(name.value).toBe("Home");
  await act(async () => finish({ ...fixture(), revision: 3, screens: [{ ...fixture().screens[0], name: "Freshly reviewed home" }] }));
  expect(name.value).toBe("Freshly reviewed home");
  expect(name.matches(":disabled")).toBe(false); expect(approval.disabled).toBe(false);
  expect(approve).toHaveBeenCalledTimes(1);
});

it.each(["success", "error"])("ignores an old saved design read's %s after workspace scope changes", async outcome => {
  ready();
  const scope = vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue("design-read-old-workspace");
  let finish!: (value: DesignPlan) => void, fail!: (cause: Error) => void;
  vi.mocked(designPlanApi.get).mockImplementationOnce(() => new Promise((resolve, reject) => { finish = resolve; fail = reject; }))
    .mockResolvedValue({ ...fixture(), screens: [{ ...fixture().screens[0], name: "Current workspace screen" }] });
  const view = render(<DesignWorkspace projectId="p-1" />);
  scope.mockReturnValue("design-read-current-workspace");
  view.rerender(<DesignWorkspace projectId="p-1" />);
  const name = await screenPlanField('Screen name') as HTMLInputElement;
  expect(name.value).toBe("Current workspace screen");
  await act(async () => { if (outcome === "success") finish(fixture()); else fail(new Error("Old workspace read failed")); });
  expect(name.value).toBe("Current workspace screen");
  expect(name.matches(":disabled")).toBe(false);
  expect(screen.queryByRole("alert")).toBeNull();
});

it("starts an unfinished screen without invented behavior or coverage, allows a named draft and blocks approval until described", async () => {
  ready({ ...fixture(), revision: 0, screens: [] });
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, plan) => ({ ...plan, revision: 1, status: "draft" }));
  const approve = vi.spyOn(designPlanApi, "approve");
  const user = userEvent.setup();
  render(<DesignWorkspace projectId="p-1" />);
  await openScreenPlan(); await user.click(await screen.findByRole("button", { name: "Describe the first screen" }));
  const name = screen.getByRole("textbox", { name: "Screen name" });
  expect(document.activeElement).toBe(name);
  for (const label of ["Screen name", "Route", "What this screen helps someone do", "States and recovery", "Accessibility and interaction"]) expect((screen.getByRole("textbox", { name: label }) as HTMLInputElement).value).toBe("");
  expect((screen.getByRole("checkbox", { name: "AC-001 · Save work" }) as HTMLInputElement).checked).toBe(false);
  expect(screen.getByText("Name screen 1 so you can save this draft.")).not.toBeNull();
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Continue to Develop" })).toBeNull();
  await user.type(name, "Garden observations");
  await user.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 1 saved. Implementation context updated.");
  const screenDraft = save.mock.calls[0][1].screens[0];
  expect(screenDraft).toMatchObject({ name: "Garden observations", route: "", purpose: "", states: [], requirementIds: [], accessibility: "", widths: [390, 1440] });
  expect((screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole("region", { name: "Design decisions still needed" }).textContent).toContain("what people do here and how their access differs");
  await user.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).not.toHaveBeenCalled();
});

it("shows actual saved roles and acceptance, then separately saves and approves explicitly authored role behavior", async () => {
  ready({ ...fixture(), revision: 0, screens: [] });
  const definition = await definitionApi.get("p-1");
  vi.mocked(definitionApi.get).mockResolvedValue({ ...definition, audience: "Garden coordinators and members", roles: [
    { id: "role-coordinator", name: "Coordinator", canDo: "Assign shared garden tasks", cannotDo: "Change another member's private notes" },
    { id: "role-member", name: "Member", canDo: "Mark assigned tasks complete", cannotDo: "Assign tasks to other members" },
  ] });
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, plan) => ({ ...plan, revision: 1, status: "draft" }));
  const approve = vi.spyOn(designPlanApi, "approve").mockImplementation(async () => ({ ...save.mock.calls[0][1], revision: 2, status: "approved" }));
  const user = userEvent.setup();
  render(<DesignWorkspace projectId="p-1" />);
  await user.click(await screen.findByText("People, permissions and outcomes from your definition"));
  const guidance = screen.getByRole("region", { name: "Plan the experience" });
  for (const text of ["Garden coordinators and members", "Assign shared garden tasks", "Change another member's private notes", "Mark assigned tasks complete", "Assign tasks to other members", "Reopened work returns"]) expect(guidance.textContent).toContain(text);
  await openScreenPlan(); await user.click(screen.getByRole("button", { name: "Describe the first screen" }));
  await user.type(screen.getByRole("textbox", { name: "Screen name" }), "Garden tasks");
  const purpose = "Coordinators assign tasks; members see and complete their assigned tasks without assignment controls. Saved completion remains when either role returns.";
  const states = "No assigned tasks: members see who to contact\nSaved completion: task remains completed after reopening\nSave failed: retain the member's changes and offer retry\nMember opens assignment route: explain coordinator access is required";
  await user.click(screen.getByRole("textbox", { name: "What this screen helps someone do" })); await user.paste(purpose);
  await user.click(screen.getByRole("textbox", { name: "States and recovery" })); await user.paste(states);
  await user.click(screen.getByRole("checkbox", { name: "AC-001 · Save work" }));
  await user.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 1 saved. Implementation context updated.");
  expect(save.mock.calls[0][1].screens[0]).toMatchObject({ purpose, states: states.split("\n"), requirementIds: ["AC-001"] });
  expect(approve).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 1);
  await screen.findByRole("button", { name: "Continue to Develop" });
});

it("opens existing Luci planning help with a scoped unsent request and requires resolving local edits first", async () => {
  ready({ ...fixture(), revision: 0, screens: [] });
  const receive = vi.fn(); window.addEventListener("luci:orchestrator-open", receive);
  const save = vi.spyOn(designPlanApi, "save"), approve = vi.spyOn(designPlanApi, "approve");
  try {
    const user = userEvent.setup(); render(<DesignWorkspace projectId="p-1" />);
    const help = await screen.findByRole("button", { name: "Ask Luci to help plan the experience" });
    await user.click(help);
    expect(receive).toHaveBeenCalledOnce();
    const detail = (receive.mock.calls[0][0] as CustomEvent).detail;
    expect(detail.projectId).toBe("p-1"); expect(detail.message).toContain("each person or role"); expect(detail.message).toContain("design-plan draft for my review");
    expect(detail.message).toContain("editable scene with frames and components");
    expect(screen.getByText(/Opens an unsent message. Sending needs an available agent connection/)).not.toBeNull();
    expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
    await openScreenPlan(); await user.click(screen.getByRole("button", { name: "Describe the first screen" }));
    expect((help as HTMLButtonElement).disabled).toBe(true);
    await user.click(help); expect(receive).toHaveBeenCalledOnce();
    expect(screen.getByText(/Save or resolve your draft first so the conversation uses the context you intend/)).not.toBeNull();
  } finally { window.removeEventListener("luci:orchestrator-open", receive); }
});

it("keeps typed comma delimiters and only saves parsed responsive widths after the edit is complete", async () => {
  ready();
  const original = fixture();
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, plan) => ({ ...plan, revision: 3, status: "draft" }));
  const approve = vi.spyOn(designPlanApi, "approve");
  const user = userEvent.setup();
  const view = render(<DesignWorkspace projectId="p-1" />);
  const input = await screenPlanField('Responsive widths');
  await user.click(input);
  await user.keyboard("{End},");
  expect((input as HTMLInputElement).value).toBe("390, 1440,");
  expect(input.getAttribute("aria-invalid")).toBe("true");
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  await user.keyboard(" 768");
  expect((input as HTMLInputElement).value).toBe("390, 1440, 768");
  expect(input.getAttribute("aria-invalid")).toBe("false");
  expect(save).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 3 saved. Implementation context updated.");
  const expected = { ...original, screens: [{ ...original.screens[0], widths: [390, 1440, 768] }] };
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", expected, 2, 1, 2);
  expect(approve).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "Continue to Develop" })).toBeNull();
  view.unmount();
  vi.mocked(designPlanApi.get).mockResolvedValue({ ...expected, revision: 3 });
  render(<DesignWorkspace projectId="p-1" />);
  expect(((await screenPlanField('Responsive widths')) as HTMLInputElement).value).toBe("390, 1440, 768");
  expect(screen.getByRole("button", { name: "Approve design handoff" })).not.toBeNull();
});

it("retains exact invalid width text through blur, tabs, reload and explicit browser draft recovery", async () => {
  ready();
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue("design-width-recovery");
  const save = vi.spyOn(designPlanApi, "save");
  const approve = vi.spyOn(designPlanApi, "approve");
  const user = userEvent.setup();
  const view = render(<DesignWorkspace projectId="p-1" />);
  const input = await screenPlanField('Responsive widths');
  const raw = "390, tablet, 1440, ";
  await user.clear(input); await user.type(input, raw); await user.tab();
  expect((input as HTMLInputElement).value).toBe(raw);
  expect(screen.getByText(/Enter whole-number widths from 320 to 3840/)).not.toBeNull();
  await user.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect(screen.getByText("Unsaved changes")).not.toBeNull();
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  await user.click(screen.getByRole("tab", { name: "Screens and states" }));
  expect((screen.getByRole("textbox", { name: "Responsive widths" }) as HTMLInputElement).value).toBe(raw);
  view.unmount();
  const reload = render(<DesignWorkspace projectId="p-1" />);
  await screen.findByRole("region", { name: "Design plan draft recovery" });
  const restore = screen.getByRole("button", { name: "Restore draft" });
  await waitFor(() => expect((restore as HTMLButtonElement).disabled).toBe(false));
  await user.click(restore);
  await openScreenPlan(); expect((screen.getByRole("textbox", { name: "Responsive widths" }) as HTMLInputElement).value).toBe(raw);
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
  await expect(vi.mocked(designPlanApi.get).mock.results[0].value).resolves.toEqual(fixture());
  reload.unmount();
  localStorage.removeItem('luci.project-draft.v1:["design-width-recovery","design-plan"]');
});

it("requires a recovery choice when unfinished widths and a teammate's saved widths both changed", async () => {
  ready();
  vi.spyOn(projectDraft, "useProjectDraftScope").mockReturnValue("design-width-conflict");
  const save = vi.spyOn(designPlanApi, "save");
  const user = userEvent.setup();
  const view = render(<DesignWorkspace projectId="p-1" />);
  const input = await screenPlanField('Responsive widths');
  await user.click(input); await user.keyboard("{End}, ");
  view.unmount();
  const peer = { ...fixture(), revision: 3, implementationNotes: "Keep the teammate's keyboard requirement", screens: [{ ...fixture().screens[0], widths: [390, 1024] }] };
  vi.mocked(designPlanApi.get).mockResolvedValue(peer);
  const reload = render(<DesignWorkspace projectId="p-1" />);
  const choice = await screen.findByRole("radio", { name: "Use my draft for Screens · Home · Responsive widths" });
  const restore = screen.getByRole("button", { name: "Restore draft" });
  expect((restore as HTMLButtonElement).disabled).toBe(true);
  await user.click(choice);
  await user.click(screen.getByRole("checkbox", { name: "I reviewed the saved version and these recovery choices." }));
  await user.click(restore);
  await openScreenPlan(); expect((screen.getByRole("textbox", { name: "Responsive widths" }) as HTMLInputElement).value).toBe("390, 1440, ");
  await user.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect((screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement).value).toBe(peer.implementationNotes);
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  expect(save).not.toHaveBeenCalled();
  reload.unmount();
  localStorage.removeItem('luci.project-draft.v1:["design-width-conflict","design-plan"]');
});

it("exposes removed requirement links and preserves all screen content until explicit unlink, save and review", async () => {
  const original: DesignPlan = { ...fixture(), status: "outdated", screens: [{ ...fixture().screens[0], requirementIds: ["AC-001", "AC-removed"] }] };
  ready(original);
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, plan) => ({ ...plan, revision: 3, status: "draft" }));
  const approve = vi.spyOn(designPlanApi, "approve").mockImplementation(async () => ({ ...fixture(), revision: 4, status: "approved" }));
  const user = userEvent.setup();
  render(<DesignWorkspace projectId="p-1" />);
  await user.click(await screen.findByRole("tab", { name: "Screens and states" }));
  const removed = await screen.findByRole("checkbox", { name: "Remove unavailable outcome AC-removed from screen 1" });
  expect((removed as HTMLInputElement).checked).toBe(true);
  expect(screen.getByText(/An outcome linked to a screen was removed/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(true);
  await user.click(screen.getByRole("button", { name: "Save design plan" }));
  expect(save).not.toHaveBeenCalled();
  expect(original.screens[0].requirementIds).toEqual(["AC-001", "AC-removed"]);
  await user.click(removed);
  expect(screen.queryByRole("checkbox", { name: "Remove unavailable outcome AC-removed from screen 1" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 3 saved. Implementation context updated.");
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", { ...original, status: "draft", digest: "", screens: [{ ...original.screens[0], requirementIds: ["AC-001"] }] }, 2, 1, 2);
  expect(original.screens[0].requirementIds).toEqual(["AC-001", "AC-removed"]);
  expect(approve).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "Continue to Develop" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  await screen.findByRole("button", { name: "Continue to Develop" });
});

it("keeps saved design usable after optional prototype history fails and retries without blocking approval", async () => {
  ready();
  vi.mocked(productApi.prototypeReleases).mockRejectedValue(new Error("History unavailable"));
  const save = vi.spyOn(designPlanApi, "save").mockResolvedValue({ ...fixture(), revision: 3, implementationNotes: "Keep saved work" });
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...fixture(), revision: 4, implementationNotes: "Keep saved work", status: "approved" });
  render(<DesignWorkspace projectId="p-1" />);
  await screenPlanField('Screen name');
  expect(screen.queryByRole("alert")).toBeNull();
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  await userEvent.clear(screen.getByLabelText("Implementation notes")); await userEvent.type(screen.getByLabelText("Implementation notes"), "Keep saved work");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 3 saved. Implementation context updated.");
  expect(save).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  expect((await screen.findByRole("alert")).textContent).toContain("History unavailable");
  expect(screen.queryByText("No verified prototype yet")).toBeNull();
  let finish!: (value: { releases: Release[] }) => void;
  vi.mocked(productApi.prototypeReleases).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Retry prototype history" }));
  await screen.findByText(/Refreshing prototype history/);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect((screen.getByLabelText("Implementation notes") as HTMLTextAreaElement).value).toBe("Keep saved work");
  expect((screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement).disabled).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  await act(async () => finish({ releases: [] }));
  expect(screen.getByRole("button", { name: "Continue to Develop" })).not.toBeNull();
  expect(designPlanApi.get).toHaveBeenCalledOnce();
});

it("links design tabs to manually activated panels without opening sketch on keyboard focus", async () => {
  ready();
  render(<DesignWorkspace projectId="p-1" />);
  const selected = await screen.findByRole("tab", { name: "Screens and states" });
  const user = userEvent.setup();
  expect(screen.getByRole('tab', {name: 'Canvas'}).getAttribute('aria-selected')).toBe('true');
  await openScreenPlan();
  selected.focus();
  await user.keyboard("{ArrowRight}");
  expect(screen.queryByText("Drawing tools")).toBeNull();
  expect(selected.getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Enter}");
  await screen.findByText("Drawing tools");
  const sketch = screen.getByRole("tab", { name: "Sketch" });
  const panel = screen.getByRole("tabpanel", { name: "Sketch" });
  expect(panel.id).toBe(sketch.getAttribute("aria-controls"));
  expect(panel.getAttribute("aria-labelledby")).toBe(sketch.id);
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
});

it("retains the mounted sketch on attempted navigation and only discards it explicitly", async () => {
  ready();
  const save = vi.spyOn(productApi, "saveDesign");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  const canvas = await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Edit sketch fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  expect((await screen.findByRole("alert")).textContent).toContain("unsaved changes");
  expect(screen.getByRole("tab", { name: "Sketch" }).getAttribute("aria-selected")).toBe("true");
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  expect(designPlanApi.get).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  await userEvent.click(screen.getByRole("button", { name: "Discard sketch and continue" }));
  expect(screen.queryByText("Drawing tools")).toBeNull();
  expect(screen.getByRole("textbox", { name: "Implementation notes" })).not.toBeNull();
  await waitFor(() => expect(designPlanApi.get).toHaveBeenCalledTimes(2));
  expect(save).not.toHaveBeenCalled();
});

it("requires re-saving the handoff against the changed sketch before approving the exact new plan revision", async () => {
  ready();
  let finishRead!: (value: DesignPlan) => void;
  vi.mocked(designPlanApi.get).mockResolvedValueOnce(fixture()).mockImplementationOnce(() => new Promise(resolve => { finishRead = resolve; }));
  const rebound: DesignPlan = { ...fixture(), revision: 3, sketchRevision: 7 };
  const save = vi.spyOn(designPlanApi, "save").mockResolvedValue(rebound);
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...rebound, revision: 4, status: "approved" });
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Edit sketch fixture" }));
  await userEvent.click(screen.getByRole("button", { name: "Save sketch fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  const staleApprove = screen.getByRole("button", { name: "Approve design handoff" });
  expect((staleApprove as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(staleApprove);
  expect(approve).not.toHaveBeenCalled();
  await act(async () => finishRead({ ...fixture(), status: "outdated" }));
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", { ...fixture(), status: "outdated" }, 2, 1, 2);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect(screen.getByText("Sketch", { selector: "dt" }).nextElementSibling?.textContent).toBe("7");
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  expect(screen.getByRole("button", { name: "Continue to Develop" })).not.toBeNull();
});

it("does not unmount the sketch during its explicit save operation", async () => {
  ready();
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  const canvas = await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Begin sketch save fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(designPlanApi.get).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("button", { name: "Complete sketch save fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  await screen.findByRole("textbox", { name: "Screen name" });
});

it("requires saved design notes and explicit approval before continuing to Develop", async () => {
  ready();
  let finish!: (value: DesignPlan) => void;
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...fixture(), revision: 3, status: "approved", digest: "b".repeat(64) });
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Implementation notes" }), " and real recovery");
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
  const action = screen.getByRole("button", { name: "Save design plan" });
  await userEvent.click(action);
  expect(action.getAttribute("aria-busy")).toBe("true");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(action);
  expect(save).toHaveBeenCalledOnce();
  expect(approve).not.toHaveBeenCalled();
  await act(async () => finish({ ...save.mock.calls[0][1], revision: 3 }));
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  await userEvent.click(await screen.findByRole("button", { name: "Continue to Develop" }));
  expect(useCockpit.getState().lens).toBe("develop");
});

it("retains the design draft and save action after a revision refusal", async () => {
  ready();
  vi.spyOn(designPlanApi, "save").mockRejectedValue(new Error("The sketch changed. Review the current saved design before retrying."));
  const approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
  const notes = screen.getByRole("textbox", { name: "Implementation notes" });
  await userEvent.type(notes, " with unsaved recovery steps");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  expect((await screen.findByRole("alert")).textContent).toContain("The sketch changed");
  expect((notes as HTMLTextAreaElement).value).toBe("Saved notes with unsaved recovery steps");
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(false);
  expect(screen.queryByRole("button", { name: "Continue to Develop" })).toBeNull();
  expect(approve).not.toHaveBeenCalled();
});

it("preserves the local isolated prototype gateway and rejects control-origin or credential URLs", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const release = { id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url: `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`, message: "Verified", createdAt: "" };
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(release.url);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  view.unmount();
  for (const url of [window.location.origin + "/", DEFAULT_BASE_URL + "/", "http://account:password@localhost:8731/", `http://${"a".repeat(64)}.localhost:8731/`]) {
    vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ ...release, url }] });
    const attempt = render(<DesignWorkspace projectId="p-1" />);
    await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Open prototype" })).not.toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
    expect((await screen.findByRole("alert")).textContent).toContain("verified separate application origin");
    expect(screen.queryByTitle("Design prototype")).toBeNull();
    attempt.unmount();
  }
});

const applicationRelease = (): Release => ({
  id: "deploy-owned", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40),
  configurationRevision: 4,
  target: "application", status: "ready", url: "", browserConnectionRequired: true,
  message: "Verified application", createdAt: "",
});

const nativeProject = (cloud = false): { project: PlatformProject; busy: boolean } => ({
  project: { projectId: "p-1", deployments: [{
    id: "deploy-owned", projectId: "p-1", provider: "docker", commit: "a".repeat(40),
    configurationRevision: 4, status: "ready", message: "Owned application", createdAt: "", updatedAt: "",
    browserConnectionRequired: cloud,
    url: cloud ? undefined : `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`,
  }], database: { status: "not-created", engine: "postgres", message: "" }, backups: [] }, busy: false,
});

it("opens a local Docker prototype through its existing owned gateway without requiring a cloud account", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  const local = nativeProject();
  const read = vi.spyOn(platformApi, "project").mockResolvedValue(local);
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(local.project.deployments[0].url);
  expect(read).toHaveBeenCalledExactlyOnceWith("p-1");
  expect(prepare).not.toHaveBeenCalled();
});

it.each([
  "project", "deployment-project", "missing", "duplicate", "commit", "configuration", "busy", "stopped", "gateway",
])("rejects changed local prototype metadata: %s", async (changed) => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  const local = nativeProject();
  const deployment = local.project.deployments[0];
  if (changed === "project") local.project.projectId = "another-project";
  if (changed === "deployment-project") deployment.projectId = "another-project";
  if (changed === "missing") local.project.deployments = [];
  if (changed === "duplicate") local.project.deployments.push({ ...deployment });
  if (changed === "commit") deployment.commit = "b".repeat(40);
  if (changed === "configuration") deployment.configurationRevision++;
  if (changed === "busy") local.busy = true;
  if (changed === "stopped") deployment.status = "stopped";
  if (changed === "gateway") deployment.url = DEFAULT_BASE_URL;
  vi.spyOn(platformApi, "project").mockResolvedValue(local);
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  await screen.findByRole("alert");
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect(prepare).not.toHaveBeenCalled();
});

it("opens a cloud Docker prototype through its owned isolated browser connection", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  const release = applicationRelease();
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  const connection = { projectId: "p-1", kind: "application" as const, resourceId: release.id,
    url: `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/` };
  const prepare = vi.spyOn(platformApi, "prepareBrowser").mockResolvedValue(connection);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  expect(screen.getByRole("option", { name: /Application · aaaaaaaa/ })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  expect(prepare).toHaveBeenCalledExactlyOnceWith("p-1", "application", release.id);
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(connection.url);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "Preview width" }), "390");
  expect((frame as HTMLIFrameElement).style.width).toBe("390px");
});

it.each([
  { projectId: "another-project", kind: "application" as const, resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "data" as const, resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "application" as const, resourceId: "another-release", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "application" as const, resourceId: "deploy-owned", url: DEFAULT_BASE_URL },
])("refuses a changed prototype/browser binding: %j", async (connection) => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  vi.spyOn(platformApi, "prepareBrowser").mockResolvedValue(connection);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  expect((await screen.findByRole("alert")).textContent).toContain("verify this prototype's browser connection");
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect((screen.getByRole("button", { name: "Open prototype" }) as HTMLButtonElement).disabled).toBe(false);
});

it("makes a stopped saved prototype recoverable and never offers a neighbouring project's release", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [
    { ...applicationRelease(), status: "stopped" },
    { ...applicationRelease(), id: "foreign-release", projectId: "other-project" },
  ] });
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  expect(screen.getByText(/This prototype has stopped or is no longer available/)).not.toBeNull();
  expect(screen.getByRole("option", { name: "Saved prototype unavailable" })).not.toBeNull();
  expect(screen.queryByRole("option", { name: /foreign-release/ })).toBeNull();
  const open = screen.getByRole("button", { name: "Open prototype" });
  expect((open as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(open);
  expect(prepare).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Go to Deploy" }));
  expect(useCockpit.getState().lens).toBe("deploy");
});

it("does not show a previous project's late application connection after navigation", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(designPlanApi.get).mockImplementation(async (id) => ({ ...fixture(), projectId: id, prototypeReleaseId: id === "p-1" ? "deploy-owned" : "" }));
  vi.mocked(productApi.prototypeReleases).mockImplementation(async (id) => ({ releases: id === "p-1" ? [applicationRelease()] : [] }));
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  let finish!: (connection: { projectId: string; kind: "application"; resourceId: string; url: string }) => void;
  vi.spyOn(platformApi, "prepareBrowser").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  view.rerender(<DesignWorkspace projectId="p-2" />);
  await screen.findByText("No verified prototype yet");
  await act(async () => { finish({ projectId: "p-1", kind: "application", resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" }); });
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect((screen.getByRole("button", { name: "Open prototype" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("alert")).toBeNull();
});

it("does not prepare an old cloud browser after its deployment read finishes in another project", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(designPlanApi.get).mockImplementation(async (id) => ({ ...fixture(), projectId: id, prototypeReleaseId: id === "p-1" ? "deploy-owned" : "" }));
  vi.mocked(productApi.prototypeReleases).mockImplementation(async (id) => ({ releases: id === "p-1" ? [applicationRelease()] : [] }));
  let finish!: (value: ReturnType<typeof nativeProject>) => void;
  vi.spyOn(platformApi, "project").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  view.rerender(<DesignWorkspace projectId="p-2" />);
  await screen.findByText("No verified prototype yet");
  await act(async () => { finish(nativeProject(true)); });
  expect(prepare).not.toHaveBeenCalled();
  expect(screen.queryByTitle("Design prototype")).toBeNull();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it("keeps saved design readable for viewers without offering screen edits or approval", async () => {
  ready(); teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  const save = vi.spyOn(designPlanApi, "save"), approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  const name = await screenPlanField('Screen name');
  expect((name.closest("fieldset.definition-fields") as HTMLFieldSetElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.type(name, " forbidden");
  expect((name as HTMLInputElement).value).toBe("Home"); expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
});


it("prepares a fresh one-use handoff for a separate tab and keeps the embedded ticket unchanged", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/app#review`;
  const release = { id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" };
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  const prepare = vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValueOnce("https://isolated.example/.well-known/luci-team-access?ticket=embedded").mockResolvedValueOnce("https://isolated.example/.well-known/luci-team-access?ticket=separate");
  const tab = { opener: window, location: { href: "about:blank" }, close: vi.fn() };
  vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  await userEvent.click(screen.getByRole("button", { name: "Open prototype in a separate tab" }));
  await waitFor(() => expect(tab.location.href).toContain("ticket=separate"));
  expect(prepare.mock.calls).toEqual([["p-1", url], ["p-1", url]]);
  expect(frame.getAttribute("src")).toContain("ticket=embedded"); expect(tab.opener).toBeNull(); expect(tab.close).not.toHaveBeenCalled();
});

it("closes a pending separate tab when execution access is revoked and does not reuse the embedded ticket", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" }] });
  let finish!: (url: string) => void;
  vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValueOnce(url).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const tab = { opener: window, location: { href: "about:blank" }, close: vi.fn() };
  vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" })); await screen.findByTitle("Design prototype");
  await userEvent.click(screen.getByRole("button", { name: "Open prototype in a separate tab" }));
  teamAccess.permissions.execute = false; view.rerender(<DesignWorkspace projectId="p-1" />);
  await act(async () => finish("https://isolated.example/.well-known/luci-team-access?ticket=stale"));
  expect(tab.close).toHaveBeenCalledOnce(); expect(tab.location.href).toBe("about:blank"); expect(screen.queryByTitle("Design prototype")).toBeNull();
  teamAccess.permissions.execute = true; view.rerender(<DesignWorkspace projectId="p-1" />);
  expect(screen.queryByTitle("Design prototype")).toBeNull();
});

it("reviews a planned screen against its real source and preserves an unverified observation through amendment and save", async () => {
  const plan = { ...fixture(), status: "approved" as const, prototypeReleaseId: "verified", prototypeCommit: "a".repeat(40), screens: [fixture().screens[0], { ...fixture().screens[0], id: "screen-detail", name: "Saved note", route: "/notes?filter=saved#detail", purpose: "Reopen a saved note", states: ["Saved note appears", "Failed load offers Retry"], widths: [320, 768] }] };
  ready(plan);
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" }] });
  const prepare = vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValueOnce(url + ".well-known/luci-team-access?ticket=first").mockResolvedValueOnce(url + ".well-known/luci-team-access?ticket=fresh");
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(async (_id, value) => ({ ...value, revision: 3 }));
  const approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "Screen to review" }), "screen-detail");
  const review = screen.getByRole("region", { name: "Screen review" });
  expect(within(review).getByText("Failed load offers Retry")).not.toBeNull();
  expect(within(review).getByText("Reopened work returns")).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Record an observation" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open planned screen" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(prepare).toHaveBeenCalledExactlyOnceWith("p-1", url + "notes?filter=saved#detail");
  expect((frame as HTMLIFrameElement).style.width).toBe("320px");
  await userEvent.click(screen.getByRole("button", { name: "Record an observation" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Observation or requested change" }), "Retry should keep the note title visible.");
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
  expect(screen.getByText("Unsaved changes")).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Amend this screen" }));
  const names = screen.getAllByRole("textbox", { name: "Screen name" });
  expect(document.activeElement).toBe(names[1]);
  expect((names[1] as HTMLInputElement).value).toBe("Saved note");
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  const notes = (screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement).value;
  expect(notes).toContain("Saved notes");
  expect(notes).toContain("human note; not verification");
  expect(notes).toContain("Release: verified");
  expect(notes).toContain("Source commit: " + "a".repeat(40));
  expect(notes).toContain("Screen: Saved note (screen-detail)");
  expect(notes).toContain("Viewport: 320 px");
  expect(notes).toContain("Retry should keep the note title visible.");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", { ...plan, status: "draft", digest: "", implementationNotes: notes }, 2, 1, 2);
  expect(approve).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect(prepare).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("button", { name: "Open planned screen" }));
  expect((await screen.findByTitle("Design prototype")).getAttribute("src")).toContain("ticket=fresh");
  expect(prepare).toHaveBeenCalledTimes(2);
});

it("discards a late browser invitation after leaving Prototype and prepares a fresh one on return", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" }] });
  let finish!: (value: string) => void;
  const prepare = vi.spyOn(browserAccess, "prepareBrowserAccess").mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(url + ".well-known/luci-team-access?ticket=fresh");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  await act(async () => finish(url + ".well-known/luci-team-access?ticket=abandoned"));
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  expect((await screen.findByTitle("Design prototype")).getAttribute("src")).toContain("ticket=fresh");
  expect(prepare).toHaveBeenCalledTimes(2);
});

it("keeps a replaced release's observations bound to the source that was actually opened", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified", prototypeCommit: "a".repeat(40) });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  const release = { id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" };
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  vi.spyOn(browserAccess, "prepareBrowserAccess").mockImplementation(async (_id, address) => address);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  await userEvent.click(await screen.findByRole("button", { name: "Record an observation" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Observation or requested change" }), "Old source has no retry.");
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ ...release, commit: "b".repeat(40) }] });
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  await screen.findByRole("option", { name: /bbbbbbbb/ });
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect(screen.queryByRole("button", { name: "Record an observation" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  await userEvent.click(await screen.findByRole("button", { name: "Record an observation" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Observation or requested change" }), "New source keeps the title.");
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  const notes = (screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement).value;
  expect(notes).toContain("Source commit: " + "a".repeat(40));
  expect(notes).toContain("Old source has no retry.");
  expect(notes).toContain("Source commit: " + "b".repeat(40));
  expect(notes).toContain("New source keeps the title.");
});

it("leaves a finished prototype observation through the handoff tab without saving or replaying its browser invitation", async () => {
  ready({ ...fixture(), status: "approved", prototypeReleaseId: "verified", prototypeCommit: "a".repeat(40) });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" }] });
  const prepare = vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValue(url + ".well-known/luci-team-access?ticket=first");
  const save = vi.spyOn(designPlanApi, "save"), approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open planned screen" }));
  await screen.findByTitle("Design prototype");
  await userEvent.click(screen.getByRole("button", { name: "Record an observation" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Observation or requested change" }), "Keep the entered note after a failed save.");
  await userEvent.click(screen.getByRole("button", { name: "Done writing" }));
  expect(screen.queryByRole("textbox", { name: "Observation or requested change" })).toBeNull();
  const handoff = screen.getByRole("tab", { name: "Implementation handoff" });
  await userEvent.click(handoff);
  expect(handoff.getAttribute("aria-selected")).toBe("true");
  const notes = (screen.getByRole("textbox", { name: "Implementation notes" }) as HTMLTextAreaElement).value;
  expect(notes).toContain("Saved notes");
  expect(notes).toContain("human note; not verification");
  expect(notes).toContain("Keep the entered note after a failed save.");
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect(prepare).toHaveBeenCalledOnce();
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
});
