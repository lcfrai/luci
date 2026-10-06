import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { productApi, type Brief, type ImplementationPlan, type RequirementCoverage } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import type { ImplementationContext } from "../../store/types";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";
import { implementationPrompt, ProjectPlan } from "./ProjectPlan";
import { DraftNavigationDialog } from "../common/DraftNavigationDialog";

const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));
vi.mock("../../hooks/useProjectDraft", async original => ({ ...await original<typeof import("../../hooks/useProjectDraft")>(), useProjectDraftScope: (projectId: string) => `plan-permission-test:${projectId}` }));
const brief: Brief = { projectId: "p-1", revision: 2, purpose: "Journal", audience: "Writers", constraints: "Offline", criteria: [], updatedAt: "" };
const plan: ImplementationPlan = { projectId: "p-1", revision: 3, briefRevision: 2, specificationRevision: 6, tasks: [{ id: "t-1", title: "Build editor", status: "todo" }, { id: "t-2", title: "Save entries", status: "doing" }, { id: "t-3", title: "Set up project", status: "done" }], updatedAt: "" };
const coverage: RequirementCoverage = { projectId: "p-1", specification: { revision: 6, status: "approved", digest: "spec-6" }, design: { revision: 8, status: "approved", digest: "design-8" }, source: { commit: "a".repeat(40), sourceDigest: "source-a", dirty: false, available: true }, rows: [] };
const snapshot = (over: Partial<ImplementationContext> = {}): ImplementationContext => ({ projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["t-1"], briefDigest: "brief-2", planDigest: "plan-3", specificationDigest: "spec-6", designPlanDigest: "design-8", worktreeId: "w-1", sourceCommit: "a".repeat(40), sourceDigest: "source-a", sourceDirty: false, taskTitles: ["Build editor"], requirementIds: [], ...over });

beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); createSession.mockReset();
  useCockpit.setState({ projects: [aProject({ id: "p-1" })], branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] }, worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", isPrimary: true })] }, selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null }, harnesses: [aHarness({ id: "claude-code", name: "Claude Code", command: "claude", available: true, supportsOpeningPrompt: true })] });
  vi.spyOn(productApi, "plan").mockResolvedValue(plan);
  vi.spyOn(productApi, "checks").mockResolvedValue({ runs: [], suggested: [], coverage });
  vi.spyOn(productApi, "agents").mockResolvedValue({ providers: [] });
  vi.spyOn(productApi, "agentConnections").mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
  createSession.mockImplementation(async request => aSession({ id: "s-bound", title: "Bound implementation", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "starting", implementationContext: snapshot({ ...request.implementationContext, taskTitles: request.implementationContext.taskIds.map((id: string) => plan.tasks.find(task => task.id === id)!.title) }) }));
});

it("binds only unfinished saved tasks and canonical handoff revisions after picker review, without changing progress", async () => {
  const save = vi.spyOn(productApi, "savePlan");
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const start = await screen.findByRole("button", { name: "Start implementation session" });
  await waitFor(() => expect((start as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(start);
  const prompt = screen.getByLabelText("Opening prompt") as HTMLTextAreaElement;
  expect(prompt.value).toContain("Use the captured reviewed context supplied by Luci for this session");
  expect(prompt.value).toContain("Read the captured specification for the exact criteria");
  expect(prompt.value).toContain("do not substitute them for the captured scope");
  expect(prompt.value).not.toMatch(/LUCI\.md|\.luci\//);
  expect(prompt.value).toContain("brief is revision 2"); expect(prompt.value).toContain("implementation plan is revision 3");
  expect(prompt.value).toContain("no task-specific requirement links");
  expect(createSession).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Create session" }));
  await screen.findByText("Bound implementation · Starting");
  expect(createSession.mock.calls[0][0].openingPrompt).toBe(prompt.value);
  expect(createSession.mock.calls[0][0].implementationContext).toEqual({ projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["t-1", "t-2"] });
  expect((screen.getByLabelText("Progress for task 1") as HTMLSelectElement).value).toBe("todo");
  expect((screen.getByLabelText("Progress for task 2") as HTMLSelectElement).value).toBe("doing");
  expect(save).not.toHaveBeenCalled();
  expect(useCockpit.getState().sessions["s-bound"].implementationContext?.sourceDigest).toBe("source-a");
});

it("keeps an unbound prompt honest about live checkout context without claiming captured inputs", () => {
  const task = { ...plan.tasks[0], requirementIds: ["R-EDIT"] };
  const prompt = implementationPrompt(brief, plan, task);
  expect(prompt).toContain("Read LUCI.md, .luci/plan.json");
  expect(prompt).toContain(".luci/design-handoff.json and .luci/design-handoff.md");
  expect(prompt).toContain("Read .luci/specification.json for the exact criteria.");
  expect(prompt).toContain("Work on this task: Build editor");
  expect(prompt).toContain("This task links: R-EDIT.");
  expect(prompt).not.toContain("captured");
});

it("requires current definition and design approvals and provides their review paths", async () => {
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: [], coverage: { ...coverage, specification: { ...coverage.specification, status: "draft" } } });
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByText("Approve the current definition in Define before starting implementation.");
  expect((screen.getAllByRole("button", { name: "Implement task" })[0] as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Review current definition" }));
  expect(useCockpit.getState().lens).toBe("discover");
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: [], coverage: { ...coverage, design: { ...coverage.design, status: "outdated" } } });
  await userEvent.click(screen.getByRole("button", { name: "Refresh approved requirements" }));
  await screen.findByText("Approve the current design handoff in Design before starting implementation.");
  await userEvent.click(screen.getByRole("button", { name: "Review current design" }));
  expect(useCockpit.getState().lens).toBe("design"); expect(createSession).not.toHaveBeenCalled();
});

it("re-saves an unchanged stale plan against current approved definition without inventing requirement links", async () => {
  const save = vi.spyOn(productApi, "savePlan").mockResolvedValue({ ...plan, revision: 4, briefRevision: 3 });
  render(<ProjectPlan projectId="p-1" brief={{ ...brief, revision: 3 }} />);
  await screen.findByText("The brief or definition changed. Review the tasks and save the plan against the current revisions.");
  expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan saved as revision 4.");
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", 3, plan.tasks, 6);
  await waitFor(() => expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(false));
  expect(save.mock.calls[0][2].every(task => !task.requirementIds)).toBe(true);
});

it("lets a viewer open immutable ended history and excludes foreign sessions without inferring completion", async () => {
  teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  useCockpit.getState().upsertSession(aSession({ id: "s-old", title: "Prior editor run", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "exited", exitReason: "Process exited with code 0", implementationContext: snapshot({ briefRevision: 1, planRevision: 2, taskTitles: ["Original editor title"], sourceDirty: true }) }));
  useCockpit.getState().upsertSession(aSession({ id: "s-foreign", title: "Foreign run", projectId: "p-2", implementationContext: snapshot() }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const history = await screen.findByRole("region", { name: "Implementation sessions" });
  expect(within(history).getByText("Prior editor run · Ended")).not.toBeNull();
  expect(within(history).getByText(/Brief 1 · Plan 2.*Earlier saved context/)).not.toBeNull();
  expect(within(history).getByText("Original editor title")).not.toBeNull();
  expect(within(history).getByText(/Completion and verification are not inferred/)).not.toBeNull();
  expect(screen.queryByText(/Foreign run/)).toBeNull();
  expect((screen.getByLabelText("Progress for task 1") as HTMLSelectElement).value).toBe("todo");
  await userEvent.click(within(history).getByRole("button", { name: "Open session" }));
  expect(useCockpit.getState().selection.sessionId).toBe("s-old");
});

it("closes a reviewed picker when approval revisions change rather than silently replacing its context", async () => {
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const task = (await screen.findAllByRole("button", { name: "Implement task" }))[0];
  await waitFor(() => expect((task as HTMLButtonElement).disabled).toBe(false)); await userEvent.click(task);
  expect(screen.getByRole("dialog", { name: "New session" })).not.toBeNull();
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: [], coverage: { ...coverage, design: { ...coverage.design, revision: 9 } } });
  await act(async () => window.dispatchEvent(new CustomEvent("luci-requirement-evidence-changed", { detail: "p-1" })));
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "New session" })).toBeNull());
  expect(screen.getByText("The saved handoff changed. Review the current definition, design and plan before starting.")).not.toBeNull();
  expect(createSession).not.toHaveBeenCalled();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it("lets an editor save tasks and prepare a run without granting coding execution", async () => {
  teamAccess.permissions.execute = false;
  const prepare = vi.fn(), save = vi.spyOn(productApi, "savePlan").mockImplementation(async (_id, _revision, tasks) => ({ ...plan, tasks, revision: 4 }));
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={prepare} />);
  const prepareButton = await screen.findByRole("button", { name: "Prepare project run" });
  await waitFor(() => expect((prepareButton as HTMLButtonElement).disabled).toBe(false));
  expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(prepareButton); expect(prepare).toHaveBeenCalledOnce(); expect(createSession).not.toHaveBeenCalled();
  await userEvent.type(screen.getByLabelText("Task 1"), " revised");
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await waitFor(() => expect(save).toHaveBeenCalledOnce()); expect(save.mock.calls[0][2][0].title).toContain("revised");
});

function browserDrafts() {
  return Object.fromEntries(Object.keys(localStorage).filter(key => key.startsWith("luci.project-draft.v1:")).map(key => [key, localStorage.getItem(key)]));
}

it("does not let a viewer create an unsaveable task draft or navigation guard", async () => {
  teamAccess.permissions.edit = false;
  useCockpit.setState({ lens: "develop" });
  const save = vi.spyOn(productApi, "savePlan");
  render(<><ProjectPlan projectId="p-1" brief={brief} /><DraftNavigationDialog /></>);
  const title = await screen.findByRole("textbox", { name: "New task" }) as HTMLInputElement;
  expect(title.disabled).toBe(true);
  await userEvent.type(title, "A task I cannot save");
  fireEvent.submit(title.closest("form")!);
  expect(title.value).toBe("");
  expect(screen.queryByLabelText("Task 4")).toBeNull();
  expect((screen.getByRole("button", { name: "Add task" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Save implementation plan" }) as HTMLButtonElement).disabled).toBe(true);
  expect(browserDrafts()).toEqual({});
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  act(() => useCockpit.getState().setLens("design"));
  expect(useCockpit.getState().lens).toBe("design");
  expect(screen.queryByRole("dialog", { name: "Keep your changes?" })).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(createSession).not.toHaveBeenCalled();
});

it("preserves an editor's task draft through a permission downgrade and only resumes editing after access returns", async () => {
  useCockpit.setState({ lens: "develop" });
  const save = vi.spyOn(productApi, "savePlan").mockImplementation(async (_id, _revision, tasks) => ({ ...plan, tasks, revision: 4 }));
  const mounted = render(<><ProjectPlan projectId="p-1" brief={brief} /><DraftNavigationDialog /></>);
  await userEvent.type(await screen.findByLabelText("Task 1"), " with autosave");
  const title = screen.getByRole("textbox", { name: "New task" }) as HTMLInputElement;
  await userEvent.type(title, "Keep existing entries");
  const retained = browserDrafts();
  expect(Object.keys(retained)).toHaveLength(1);
  teamAccess.permissions.edit = false;
  mounted.rerender(<><ProjectPlan projectId="p-1" brief={brief} /><DraftNavigationDialog /></>);
  expect(title.disabled).toBe(true);
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByLabelText("Progress for task 1") as HTMLSelectElement).disabled).toBe(true);
  await userEvent.type(title, " cannot be added now");
  fireEvent.submit(title.closest("form")!);
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  expect(title.value).toBe("Keep existing entries");
  expect(screen.queryByLabelText("Task 4")).toBeNull();
  expect(browserDrafts()).toEqual(retained);
  expect(save).not.toHaveBeenCalled();
  act(() => useCockpit.getState().setLens("design"));
  expect(screen.getByRole("dialog", { name: "Keep your changes?" })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(useCockpit.getState().lens).toBe("develop");
  expect(title.value).toBe("Keep existing entries");
  teamAccess.permissions.edit = true;
  mounted.rerender(<><ProjectPlan projectId="p-1" brief={brief} /><DraftNavigationDialog /></>);
  expect(title.disabled).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Add task" }));
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan saved as revision 4.");
  expect(save).toHaveBeenCalledOnce();
  expect(save.mock.calls[0][2].map(task => task.title)).toEqual(["Build editor with autosave", "Save entries", "Set up project", "Keep existing entries"]);
  expect(browserDrafts()).toEqual({});
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  expect(createSession).not.toHaveBeenCalled();
});

it("locks a pending recovered task title until the person explicitly restores it", async () => {
  const mounted = render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.type(await screen.findByRole("textbox", { name: "New task" }), "Recover this title");
  const retained = browserDrafts();
  mounted.unmount();
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByRole("region", { name: "Implementation plan draft recovery" });
  const title = await screen.findByRole("textbox", { name: "New task" }) as HTMLInputElement;
  expect(title.disabled).toBe(true);
  await userEvent.type(title, "Do not replace recovery");
  fireEvent.submit(title.closest("form")!);
  expect(title.value).toBe("");
  expect(browserDrafts()).toEqual(retained);
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect(title.disabled).toBe(false);
  expect(title.value).toBe("Recover this title");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
});

it("keeps an unadded task title unchanged while another task edit is being saved", async () => {
  let finish!: (value: ImplementationPlan) => void;
  const save = vi.spyOn(productApi, "savePlan").mockReturnValue(new Promise(resolve => { finish = resolve; }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.type(await screen.findByLabelText("Task 1"), " with autosave");
  const title = screen.getByRole("textbox", { name: "New task" }) as HTMLInputElement;
  await userEvent.type(title, "A separate next task");
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  expect(title.disabled).toBe(true);
  await userEvent.type(title, " not accepted during save");
  fireEvent.submit(title.closest("form")!);
  expect(title.value).toBe("A separate next task");
  expect(screen.queryByLabelText("Task 4")).toBeNull();
  await act(async () => finish({ ...plan, tasks: save.mock.calls[0][2], revision: 4 }));
  expect(title.disabled).toBe(false);
  expect(title.value).toBe("A separate next task");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  expect(JSON.parse(Object.values(browserDrafts())[0]!).value.title).toBe("A separate next task");
  expect(save).toHaveBeenCalledOnce(); expect(createSession).not.toHaveBeenCalled();
});
