import { StrictMode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, cleanup, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { DefinitionWorkspace } from "./DefinitionWorkspace";
import { resetCockpit, useCockpit } from "../../store/store";
import { productApi } from "../../api/product";
import { requestProjectContextEdit, takeProjectContextEdit } from "../../api/projectContextEvents";

vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: "question-owner" } } }) }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ status: { access: { user: { username: "question-owner" } }, config: { projectsDir: "/question-projects" } } }) }));
const questionScope = () => JSON.stringify([productApi.baseUrl, "question-owner", "local", "", "", "question-owner", "/question-projects", "p-1"]);

afterEach(() => { cleanup();vi.restoreAllMocks(); });
beforeEach(() => { localStorage.clear(); resetCockpit(); vi.spyOn(definitionApi,"listAssistance").mockResolvedValue({jobs:[]}); vi.spyOn(productApi,"agentConnections").mockResolvedValue({revision:0,activeConnectionId:"",connections:[]}); vi.spyOn(productApi,"agents").mockResolvedValue({providers:[]}); });
const fixture = (): ProjectSpecification => ({projectId:"p-1",version:1,revision:2,briefRevision:3,startingPoint:"new",purpose:"A complete app",audience:"People",constraints:"Offline",outOfScope:[],requirements:[{id:"AC-001",title:"Save work",priority:"must",acceptance:"Saved work survives reopening"}],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:"draft",digest:"",updatedAt:""});
const questionFixture = (): ProjectSpecification => ({ ...fixture(), questions: [{ id: "storage", question: "Where do records live?", answer: "" }, { id: "membership", question: "How do members join?", answer: "" }] });
function retainQuestionDraft(value: ProjectSpecification) {
  const original = questionFixture();
  localStorage.setItem("luci.project-draft.v1:" + JSON.stringify([questionScope(), "definition"]), JSON.stringify({ version: 1, base: JSON.stringify({ revision: original.revision, briefRevision: original.briefRevision }), original, value, updatedAt: Date.now() }));
}
it("opens the exact question answer after a delayed load and focuses only once without saving or invoking an agent", async () => {
  let finish!: (value: ProjectSpecification) => void;
  vi.spyOn(definitionApi, "get").mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const save = vi.spyOn(definitionApi, "save"), assist = vi.spyOn(definitionApi, "assist");
  const cancel = requestProjectContextEdit(questionScope(), "p-1", "definition", "membership");
  render(<DefinitionWorkspace projectId="p-1" />);
  const spec = questionFixture(); spec.questions.reverse();
  await act(async () => finish(spec));
  const answer = within(await screen.findByRole("group", { name: "Open question 1" })).getByRole("textbox", { name: "Answer" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(screen.getByRole("tab", { name: "Specification" }).getAttribute("aria-selected")).toBe("true");
  await userEvent.type(answer, "Coordinators invite members.");
  const idea = screen.getByRole("textbox", { name: "The idea" }); await userEvent.click(idea); await userEvent.type(idea, " with retained context");
  expect(document.activeElement).toBe(idea);
  expect((answer as HTMLTextAreaElement).value).toBe("Coordinators invite members.");
  expect(takeProjectContextEdit(questionScope(), "p-1", "definition")).toBe(false);
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled(); cancel();
});
it("waits for explicit recovery before focusing a reordered question and preserves the recovered answers", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(questionFixture());
  const retained = questionFixture(); retained.purpose = "Keep this unrelated draft"; retained.questions.reverse(); retained.questions[0].answer = "Invite only";
  retainQuestionDraft(retained);
  const cancel = requestProjectContextEdit(questionScope(), "p-1", "definition", "membership");
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByRole("region", { name: "Definition draft recovery" });
  const restore = screen.getByRole("button", { name: "Restore draft" });
  expect(document.activeElement?.tagName).not.toBe("TEXTAREA");
  await userEvent.click(restore);
  const answer = within(await screen.findByRole("group", { name: "Open question 1" })).getByRole("textbox", { name: "Answer" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect((answer as HTMLTextAreaElement).value).toBe("Invite only");
  expect((screen.getByRole("textbox", { name: "The idea" }) as HTMLTextAreaElement).value).toBe("Keep this unrelated draft"); cancel();
});
it("reports a question removed by recovery without focusing or overwriting a different answer", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(questionFixture());
  const retained = questionFixture(); retained.questions = [retained.questions[0]]; retained.questions[0].answer = "Local persistent storage";
  retainQuestionDraft(retained);
  const save = vi.spyOn(definitionApi, "save");
  const cancel = requestProjectContextEdit(questionScope(), "p-1", "definition", "membership");
  render(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Restore draft" }));
  await screen.findByText(/This question is no longer in the current definition draft/);
  const other = screen.getByRole("textbox", { name: "Answer" });
  expect(document.activeElement).not.toBe(other);
  expect((other as HTMLTextAreaElement).value).toBe("Local persistent storage");
  expect(save).not.toHaveBeenCalled(); cancel();
});
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


it("aborts an abandoned definition GET while fencing late ignored-abort content and reading a fresh baseline on return", async () => {
  const reads: { id: string; signal: AbortSignal; finish(value: ProjectSpecification): void }[] = [];
  vi.spyOn(definitionApi, "get").mockImplementation((id, signal) => new Promise(finish => {
    expect(signal).toBeInstanceOf(AbortSignal); reads.push({ id, signal: signal!, finish });
  }));
  const mounted = render(<DefinitionWorkspace projectId="p-1" />);
  expect(reads).toHaveLength(1);
  mounted.rerender(<DefinitionWorkspace projectId="p-2" />);
  expect(reads[0].signal.aborted).toBe(true); expect(reads[1].signal.aborted).toBe(false);
  expect(reads[1].signal).not.toBe(reads[0].signal);
  await act(async () => reads[0].finish({ ...fixture(), purpose: "Late private A" }));
  expect(screen.queryByDisplayValue("Late private A")).toBeNull();
  await act(async () => reads[1].finish({ ...fixture(), projectId: "p-2", purpose: "Fresh project B" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("Fresh project B");
  mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(reads).toHaveLength(3); expect(screen.queryByLabelText("The idea")).toBeNull();
  expect(reads[2].signal.aborted).toBe(false);
  mounted.unmount(); expect(reads[2].signal.aborted).toBe(true);
});

it("gives StrictMode's live definition read a new controller and ignores the canceled first effect", async () => {
  const reads: { signal: AbortSignal; finish(value: ProjectSpecification): void }[] = [];
  vi.spyOn(definitionApi, "get").mockImplementation((_id, signal) => new Promise(finish => reads.push({ signal: signal!, finish })));
  const mounted = render(<StrictMode><DefinitionWorkspace projectId="p-1" /></StrictMode>);
  expect(reads).toHaveLength(2); expect(reads[0].signal.aborted).toBe(true); expect(reads[1].signal.aborted).toBe(false);
  await act(async () => reads[0].finish({ ...fixture(), purpose: "Canceled strict effect" }));
  expect(screen.queryByDisplayValue("Canceled strict effect")).toBeNull();
  await act(async () => reads[1].finish(fixture()));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe(fixture().purpose);
  mounted.unmount();
});

it("preserves definition drafts through Read revocation while canceling and refreshing a pending saved baseline", async () => {
  const reads: { signal: AbortSignal; finish(value: ProjectSpecification): void }[] = [];
  vi.spyOn(definitionApi, "get").mockResolvedValueOnce(fixture()).mockImplementation((_id, signal) => new Promise(finish => reads.push({ signal: signal!, finish })));
  const mounted = render(<DefinitionWorkspace projectId="p-1" />);
  const idea = await screen.findByLabelText("The idea"); await userEvent.type(idea, " retained answer");
  const draftKey = Object.keys(localStorage).find(key => key.startsWith("luci.project-draft.v1:"))!;
  const savedDraft = localStorage.getItem(draftKey);
  teamAccess.permissions.read = false; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(screen.queryByLabelText("The idea")).toBeNull(); expect(localStorage.getItem(draftKey)).toBe(savedDraft);
  teamAccess.permissions.read = true; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(reads).toHaveLength(1); expect(reads[0].signal.aborted).toBe(false);
  expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  teamAccess.permissions.read = false; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(reads[0].signal.aborted).toBe(true);
  teamAccess.permissions.read = true; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(reads).toHaveLength(2);
  await act(async () => reads[0].finish({ ...fixture(), purpose: "Revoked read must not win" }));
  expect(screen.queryByDisplayValue("Revoked read must not win")).toBeNull();
  await act(async () => reads[1].finish(fixture()));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe("A complete app retained answer");
});

it("aborts an explicit definition reload on departure without reporting a late service error", async () => {
  let signal!: AbortSignal; let fail!: (cause: unknown) => void;
  vi.spyOn(definitionApi, "get").mockRejectedValueOnce(new Error("Initial read failed")).mockImplementation((_id, selected) => new Promise((_resolve, reject) => { signal = selected!; fail = reject; }));
  const mounted = render(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Reload saved definition" }));
  expect(signal.aborted).toBe(false); mounted.unmount(); expect(signal.aborted).toBe(true);
  await act(async () => fail(new Error("Late abandoned failure")));
  expect(screen.queryByText("Late abandoned failure")).toBeNull();
});

it("does not cancel a readable definition or refetch it for Edit-only permission changes", async () => {
  let signal!: AbortSignal; let finish!: (value: ProjectSpecification) => void;
  vi.spyOn(definitionApi, "get").mockImplementation((_id, selected) => new Promise(resolve => { signal = selected!; finish = resolve; }));
  const mounted = render(<DefinitionWorkspace projectId="p-1" />);
  teamAccess.permissions.edit = false; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(signal.aborted).toBe(false); expect(definitionApi.get).toHaveBeenCalledOnce();
  await act(async () => finish(fixture()));
  expect((screen.getByLabelText("The idea").closest("fieldset") as HTMLFieldSetElement).disabled).toBe(true);
});


it("queues the restored-Read baseline behind a pending definition save without canceling or replaying the write", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(fixture());
  let finish!: (value: ProjectSpecification) => void;
  const save = vi.spyOn(definitionApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const mounted = render(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.type(await screen.findByLabelText("The idea"), " accepted saved change");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  teamAccess.permissions.read = false; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  teamAccess.permissions.read = true; mounted.rerender(<DefinitionWorkspace projectId="p-1" />);
  expect(definitionApi.get).toHaveBeenCalledOnce(); expect(save).toHaveBeenCalledOnce();
  const saved = { ...save.mock.calls[0][1], revision: 3, briefRevision: 4 };
  vi.mocked(definitionApi.get).mockResolvedValue(saved);
  await act(async () => finish(saved));
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalledTimes(2));
  expect((screen.getByLabelText("The idea") as HTMLTextAreaElement).value).toBe(saved.purpose);
  expect((screen.getByRole("button", { name: "Approve definition" }) as HTMLButtonElement).disabled).toBe(false);
  expect(save.mock.calls[0]).toHaveLength(4); expect(save).toHaveBeenCalledOnce();
});


function populatedGroups(projectId = "p-1"): ProjectSpecification {
  return {
    ...twoOutcomes(), projectId, purpose: `Purpose of ${projectId}`,
    roles: [{ id: `${projectId}-role`, name: `${projectId} reviewer`, canDo: "Review changes", cannotDo: "Deploy" }],
    entities: [{ id: `${projectId}-entity`, name: "Result", fields: "Evidence", privacy: "Project only" }],
    integrations: [{ id: `${projectId}-integration`, name: "Provider", purpose: "Build", constraint: "Budget" }],
    glossary: [{ id: `${projectId}-term`, term: "Verified", meaning: "Check passed" }],
    decisions: [{ id: `${projectId}-decision`, title: "Local", decision: "Local first", reason: "Control", alternatives: "Cloud" }],
    questions: [{ id: `${projectId}-question`, question: "Who reviews?", answer: "" }],
  };
}
function group(title: string): HTMLDetailsElement {
  return screen.getByText(title, { selector: "summary > span" }).closest("details")!;
}
async function disclose(details: HTMLDetailsElement, open = true) {
  await act(async () => { details.open = open; fireEvent(details, new Event("toggle")); });
}
it("defers every initially closed record and journey editor while retaining the visible counts", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(populatedGroups());
  render(<DefinitionWorkspace projectId="p-1" />);
  await screen.findByLabelText("The idea");
  for (const title of ["User journeys", "Access roles", "Data entities", "Integrations", "Glossary terms", "Decisions", "Open questions"]) {
    const details = group(title);
    expect(details.open).toBe(false);
    expect(details.querySelector("summary")?.textContent).toContain("1");
    expect(details.querySelector(".definition-group-body")).toBeNull();
    expect(details.querySelectorAll("input, textarea, button")).toHaveLength(0);
  }
});
it("mounts a disclosed record once and keeps edited values and controls through close and reopen", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(populatedGroups());
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, value) => ({ ...value, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  const details = group("Access roles");
  expect(details.querySelector("textarea")).toBeNull();
  await disclose(details);
  const role = screen.getByLabelText("Role name");
  await userEvent.type(role, " amended");
  await disclose(details, false);
  expect(details.querySelector("textarea")).toBe(role);
  await disclose(details);
  expect(screen.getByLabelText("Role name")).toBe(role);
  expect((role as HTMLTextAreaElement).value).toBe("p-1 reviewer amended");
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  expect(save.mock.calls[0][1].roles[0]).toEqual({ ...populatedGroups().roles[0], name: "p-1 reviewer amended" });
});
it("defers the journey requirement matrix then retains its edits while collapsed", async () => {
  vi.spyOn(definitionApi, "get").mockResolvedValue(populatedGroups());
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, value) => ({ ...value, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  const details = group("User journeys");
  expect(details.querySelectorAll('input[type="checkbox"]')).toHaveLength(0);
  await disclose(details);
  const journey = screen.getByLabelText("Journey name");
  await userEvent.type(journey, " amended");
  await userEvent.click(screen.getByRole("checkbox", { name: "AC-002 · Staff block availability" }));
  await disclose(details, false); await disclose(details);
  expect(screen.getByLabelText("Journey name")).toBe(journey);
  expect((journey as HTMLInputElement).value).toBe("Booking amended");
  expect((screen.getByRole("checkbox", { name: "AC-002 · Staff block availability" }) as HTMLInputElement).checked).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  expect(save.mock.calls[0][1].journeys[0]).toMatchObject({ name: "Booking amended", requirementIds: ["AC-001"] });
});
it("reveals usable missing-link repair before saving without requiring a disclosure click", async () => {
  const value = populatedGroups(); value.journeys[0].requirementIds.push("AC-removed");
  vi.spyOn(definitionApi, "get").mockResolvedValue(value);
  const save = vi.spyOn(definitionApi, "save").mockImplementation(async (_id, spec) => ({ ...spec, revision: 3 }));
  render(<DefinitionWorkspace projectId="p-1" />);
  const removed = await screen.findByRole("checkbox", { name: "Remove unavailable outcome AC-removed from journey 1" });
  expect(group("User journeys").open).toBe(true);
  expect(screen.getByRole("alert").textContent).toContain("outcome that was removed");
  await userEvent.type(screen.getByLabelText("Journey name"), " preserved");
  expect((screen.getByRole("button", { name: "Save definition" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(removed);
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  expect(save.mock.calls[0][1].journeys[0]).toMatchObject({ name: "Booking preserved", requirementIds: ["AC-001", "AC-002"] });
});
it("scopes deferred group state and retained edits to each project while respecting fresh read and edit grants", async () => {
  vi.spyOn(definitionApi, "get").mockImplementation(async id => populatedGroups(id));
  const save = vi.spyOn(definitionApi, "save");
  const view = render(<DefinitionWorkspace projectId="p-1" />); await screen.findByLabelText("The idea");
  await disclose(group("Access roles"));
  await userEvent.type(screen.getByLabelText("Role name"), " retained");
  view.rerender(<DefinitionWorkspace projectId="p-2" />);
  await screen.findByDisplayValue("Purpose of p-2");
  expect(group("Access roles").querySelector("textarea")).toBeNull();
  await disclose(group("Access roles"));
  expect((screen.getByLabelText("Role name") as HTMLTextAreaElement).value).toBe("p-2 reviewer");
  teamAccess.permissions.edit = false; view.rerender(<DefinitionWorkspace projectId="p-2" />);
  await userEvent.type(screen.getByLabelText("Role name"), " forbidden");
  expect((screen.getByLabelText("Role name") as HTMLTextAreaElement).value).toBe("p-2 reviewer");
  teamAccess.permissions.read = false; view.rerender(<DefinitionWorkspace projectId="p-2" />);
  expect(screen.queryByLabelText("Role name")).toBeNull();
  teamAccess.permissions.read = true; teamAccess.permissions.edit = true;
  view.rerender(<DefinitionWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Restore draft" }));
  await disclose(group("Access roles"));
  expect((screen.getByLabelText("Role name") as HTMLTextAreaElement).value).toBe("p-1 reviewer retained");
  expect(save).not.toHaveBeenCalled();
});
