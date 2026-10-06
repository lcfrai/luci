import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { DefinitionWorkspace } from "./DefinitionWorkspace";
import { resetCockpit, useCockpit } from "../../store/store";
import { productApi } from "../../api/product";

afterEach(() => { cleanup();vi.restoreAllMocks(); });
beforeEach(() => { resetCockpit(); vi.spyOn(definitionApi,"listAssistance").mockResolvedValue({jobs:[]}); vi.spyOn(productApi,"agentConnections").mockResolvedValue({revision:0,activeConnectionId:"",connections:[]}); vi.spyOn(productApi,"agents").mockResolvedValue({providers:[]}); });
const fixture = (): ProjectSpecification => ({projectId:"p-1",version:1,revision:2,briefRevision:3,startingPoint:"new",purpose:"A complete app",audience:"People",constraints:"Offline",outOfScope:[],requirements:[{id:"AC-001",title:"Save work",priority:"must",acceptance:"Saved work survives reopening"}],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:"draft",digest:"",updatedAt:""});
it("keeps requirement identities while changing acceptance words",async()=>{
  vi.spyOn(definitionApi,"get").mockResolvedValue(fixture());
  const save=vi.spyOn(definitionApi,"save").mockImplementation(async(_id,spec)=>({...spec,revision:3,briefRevision:4}));
  render(<DefinitionWorkspace projectId="p-1"/>);
  const acceptance=await screen.findByRole("textbox",{name:"Acceptance scenario 1"});
  await userEvent.clear(acceptance);await userEvent.type(acceptance,"Given saved work, when reopened offline, then all entries return");
  await userEvent.click(screen.getByRole("button",{name:"Save definition"}));
  await waitFor(()=>expect(save).toHaveBeenCalledOnce());
  expect(save.mock.calls[0][1].requirements[0].id).toBe("AC-001");
  expect(save.mock.calls[0][1].requirements[0].acceptance).toContain("offline");
});
it("activates linked definition panels manually without discarding edited answers", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  const history = vi.spyOn(definitionApi, "history").mockResolvedValue({ revisions: [] });
  render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByRole("textbox", { name: "The idea" });
  await userEvent.type(idea, " with an edited outcome");
  const specification = screen.getByRole("tab", { name: "Specification" });
  const user = userEvent.setup();
  specification.focus();
  await user.keyboard("{ArrowRight}{ArrowRight}");
  expect(history).not.toHaveBeenCalled();
  expect(specification.getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Enter}");
  const selected = screen.getByRole("tab", { name: "Revision history" });
  const panel = screen.getByRole("tabpanel", { name: "Revision history" });
  expect(panel.id).toBe(selected.getAttribute("aria-controls"));
  expect(panel.getAttribute("aria-labelledby")).toBe(selected.id);
  await waitFor(() => expect(history).toHaveBeenCalledExactlyOnceWith("p-1"));
  await user.click(specification);
  expect((screen.getByRole("textbox", { name: "The idea" }) as HTMLTextAreaElement).value).toBe("A complete app with an edited outcome");
  expect(screen.getByRole("button", { name: "Save definition" })).not.toBeNull();
});

it("advances the primary action only after a saved revision and explicit approval", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  let finish!: (value: ProjectSpecification) => void;
  const save = vi.spyOn(definitionApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const approved = { ...fixture(), revision: 3, briefRevision: 4, status: "approved" as const, digest: "a".repeat(64) };
  const approve = vi.spyOn(definitionApi, "approve").mockResolvedValue(approved);
  render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByRole("textbox", { name: "The idea" });
  expect(screen.queryByRole("button", { name: "Save definition" })).toBeNull();
  await userEvent.type(idea, " with recovery");
  expect(screen.queryByRole("button", { name: "Approve definition" })).toBeNull();
  const action = screen.getByRole("button", { name: "Save definition" });
  await userEvent.click(action);
  expect(action.getAttribute("aria-busy")).toBe("true");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(action);
  expect(save).toHaveBeenCalledOnce();
  expect(approve).not.toHaveBeenCalled();
  await act(async () => finish({ ...save.mock.calls[0][1], revision: 3, briefRevision: 4 }));
  await userEvent.click(screen.getByRole("button", { name: "Approve definition" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3, 4);
  await userEvent.click(await screen.findByRole("button", { name: "Continue to Design" }));
  expect(useCockpit.getState().lens).toBe("design");
});

it("retains a rejected approval as a draft and lets the user correct it", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  vi.spyOn(definitionApi, "approve").mockRejectedValue(new Error("Resolve the unanswered acceptance question before approval."));
  render(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Approve definition" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Resolve the unanswered acceptance question");
  expect(screen.queryByRole("button", { name: "Continue to Design" })).toBeNull();
  expect((screen.getByRole("textbox", { name: "The idea" }) as HTMLTextAreaElement).value).toBe(fixture().purpose);
  await userEvent.type(screen.getByRole("textbox", { name: "The idea" }), " with a clear acceptance scenario");
  expect(screen.getByRole("button", { name: "Save definition" })).not.toBeNull();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it("allows viewers to read shared definition while disabling edits, approval and provider execution", async () => {
  teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  const save = vi.spyOn(definitionApi, "save"), approve = vi.spyOn(definitionApi, "approve"), assist = vi.spyOn(definitionApi, "assist");
  render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByLabelText("The idea");
  expect((idea.closest("fieldset") as HTMLFieldSetElement).disabled).toBe(true);
  expect(screen.getByText(/Read-only access/)).not.toBeNull();
  await userEvent.type(idea, " forbidden");
  await userEvent.click(screen.getByRole("button", { name: "Approve definition" }));
  expect((idea as HTMLTextAreaElement).value).toBe(fixture().purpose);
  expect(screen.queryByRole("button", { name: "Ask Luci" })).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled();
});
it("retains edited answers while project permissions refresh and then become read-only", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  const view = render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByLabelText("The idea"); await userEvent.type(idea, " with unsaved edits");
  teamAccess.loading = true; view.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toContain("with unsaved edits");
  expect((screen.getByRole("button", { name: "Save definition" }) as HTMLButtonElement).disabled).toBe(true);
  teamAccess.loading = false; teamAccess.permissions.edit = false; view.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toContain("with unsaved edits");
  expect(definitionApi.get).toHaveBeenCalledOnce();
});

it("continues the same project conversation without starting another provider job", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  const assist = vi.spyOn(definitionApi, "assist"), save = vi.spyOn(definitionApi, "save");
  const events: unknown[] = [];
  const handle = (event: Event) => events.push((event as CustomEvent).detail);
  window.addEventListener("luci:orchestrator-open", handle);
  try {
    render(<DefinitionWorkspace projectId="p-1" />);
    await screen.findByLabelText("The idea");
    await userEvent.click(screen.getByRole("button", { name: "Discuss or request a change" }));
    expect(events).toEqual([{ projectId: "p-1" }]);
    expect(assist).not.toHaveBeenCalled(); expect(save).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("Definition assistant")).toBeNull();
  } finally { window.removeEventListener("luci:orchestrator-open", handle); }
});
it("retains an amendment draft before returning to the saved-context conversation", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByLabelText("The idea");
  await userEvent.type(idea, " with changed intent");
  expect((screen.getByRole("button", { name: "Discuss or request a change" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/Save or resolve this draft first/)).not.toBeNull();
  expect((idea as HTMLTextAreaElement).value).toContain("changed intent");
});

async function interviewOutcomes() {
  await screen.findByRole("textbox", { name: "The idea" });
  await userEvent.click(screen.getByRole("tab", { name: "Guided interview" }));
  for (let step = 0; step < 4; step++) await userEvent.click(screen.getByRole("button", { name: "Next question" }));
  await screen.findByRole("textbox", { name: "How to check outcome 1" });
}
function twoOutcomes() {
  const value = fixture();
  value.requirements.push({ id: "AC-002", title: "Staff block availability", priority: "should", acceptance: "Unavailable slots cannot be booked" });
  value.journeys = [{ id: "customer", name: "Booking", actor: "Customer", steps: "Choose and confirm", requirementIds: ["AC-001", "AC-002"] }];
  return value;
}
it("changes only the intended acceptance when revisiting the interview, preserving titles, IDs, priorities and other answers", async () => {
  const original = twoOutcomes();
  vi.spyOn(definitionApi, "get").mockResolvedValue(original);
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, spec) => ({ ...spec, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />); await interviewOutcomes();
  const answer = screen.getByRole("textbox", { name: "How to check outcome 1" });
  await userEvent.clear(answer); await userEvent.type(answer, "When reopened offline, every saved entry returns.");
  await userEvent.click(screen.getByRole("button", { name: "Review specification" }));
  expect((screen.getByRole("textbox", { name: "Requirement 1" }) as HTMLInputElement).value).toBe("Save work");
  expect((screen.getByRole("textbox", { name: "Requirement 2" }) as HTMLInputElement).value).toBe("Staff block availability");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  expect(save.mock.calls[0][1].requirements).toEqual([{ ...original.requirements[0], acceptance: "When reopened offline, every saved entry returns." }, original.requirements[1]]);
  expect(save.mock.calls[0][1].journeys).toEqual(original.journeys);
});
it("reorders and removes the chosen outcome without reassigning identities or retaining its journey link", async () => {
  const original = twoOutcomes();
  vi.spyOn(definitionApi, "get").mockResolvedValue(original);
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, spec) => ({ ...spec, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />); await interviewOutcomes();
  await userEvent.click(screen.getByRole("button", { name: "Move outcome 2 up" }));
  expect((screen.getByRole("textbox", { name: "Outcome 1" }) as HTMLInputElement).value).toBe(original.requirements[1].title);
  await userEvent.click(screen.getByRole("button", { name: "Remove outcome 2" }));
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  expect(save.mock.calls[0][1].requirements).toEqual([original.requirements[1]]);
  expect(save.mock.calls[0][1].journeys[0].requirementIds).toEqual(["AC-002"]);
});
it("keeps an empty or multiline answer on its own outcome and adds a distinct outcome without recreating existing records", async () => {
  const original = twoOutcomes();
  vi.spyOn(definitionApi, "get").mockResolvedValue(original);
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, spec) => ({ ...spec, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />); await interviewOutcomes();
  await userEvent.clear(screen.getByRole("textbox", { name: "How to check outcome 1" }));
  await userEvent.click(screen.getByRole("button", { name: "Add an outcome" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Outcome 3" }), "Customer cancellation");
  await userEvent.type(screen.getByRole("textbox", { name: "How to check outcome 3" }), "Cancel a booking.\nThe slot becomes available.");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  const rows = save.mock.calls[0][1].requirements;
  expect(rows).toHaveLength(3);
  expect(rows[0]).toEqual({ ...original.requirements[0], acceptance: "" });
  expect(rows[1]).toEqual(original.requirements[1]);
  expect(rows[2]).toMatchObject({ title: "Customer cancellation", acceptance: "Cancel a booking.\nThe slot becomes available.", priority: "must" });
  expect(new Set(rows.map(row => row.id)).size).toBe(3);
});
