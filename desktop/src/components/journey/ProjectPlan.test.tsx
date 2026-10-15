import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { productApi, type Brief, type ImplementationPlan, type RequirementCoverage } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import type { ImplementationContext } from "../../store/types";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";
import { implementationPrompt, ProjectPlan } from "./ProjectPlan";
import { DraftNavigationDialog } from "../common/DraftNavigationDialog";
import { planDraftValid } from "../../hooks/projectDraftValidation";

const { createSession, reviewedContext, draftIdentity } = vi.hoisted(() => ({ createSession: vi.fn(), reviewedContext: vi.fn(), draftIdentity: { value: "plan-permission-test" } }));
vi.mock("../../api", () => ({ api: { createSession, getSessionReviewedContext: reviewedContext } }));
vi.mock("../../hooks/useProjectDraft", async original => ({ ...await original<typeof import("../../hooks/useProjectDraft")>(), useProjectDraftScope: (projectId: string) => `${draftIdentity.value}:${projectId}` }));
const brief: Brief = { projectId: "p-1", revision: 2, purpose: "Journal", audience: "Writers", constraints: "Offline", criteria: [], updatedAt: "" };
const plan: ImplementationPlan = { projectId: "p-1", revision: 3, briefRevision: 2, specificationRevision: 6, tasks: [{ id: "t-1", title: "Build editor", status: "todo" }, { id: "t-2", title: "Save entries", status: "doing" }, { id: "t-3", title: "Set up project", status: "done" }], updatedAt: "" };
const coverage: RequirementCoverage = { projectId: "p-1", specification: { revision: 6, status: "approved", digest: "spec-6" }, design: { revision: 8, status: "approved", digest: "design-8" }, source: { commit: "a".repeat(40), sourceDigest: "source-a", dirty: false, available: true }, rows: [] };
const snapshot = (over: Partial<ImplementationContext> = {}): ImplementationContext => ({ projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["t-1"], briefDigest: "brief-2", planDigest: "plan-3", specificationDigest: "spec-6", designPlanDigest: "design-8", worktreeId: "w-1", sourceCommit: "a".repeat(40), sourceDigest: "source-a", sourceDirty: false, taskTitles: ["Build editor"], requirementIds: [], ...over });

beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); createSession.mockReset(); reviewedContext.mockReset();
  draftIdentity.value = "plan-permission-test";
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

async function openScope(taskNumber: number) {
  await userEvent.click(await screen.findByText(new RegExp(`^Scope and dependencies for task ${taskNumber} ·`)));
  return await screen.findByRole("group", { name: `Task ${taskNumber} waits for` });
}

it("saves task-specific acceptance, concrete file scope and dependencies without changing progress or dispatching", async () => {
  const save = vi.spyOn(productApi, "savePlan").mockImplementation(async (_id, _revision, tasks) => ({ ...plan, tasks, revision: 4 }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const dependencies = await openScope(1);
  fireEvent.change(screen.getByRole("textbox", { name: "How to verify task 1" }), { target: { value: "Write an entry, reload, and recover it after an offline save." } });
  fireEvent.change(screen.getByRole("textbox", { name: "Intended files and folders for task 1" }), { target: { value: "src/editor\ntests/editor.test.ts\n" } });
  await userEvent.click(within(dependencies).getByRole("checkbox", { name: "Set up project · Task 3" }));
  expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan saved as revision 4.");
  const task = save.mock.calls[0][2][0];
  expect(task).toEqual({ ...plan.tasks[0], acceptance: "Write an entry, reload, and recover it after an offline save.", writePaths: ["src/editor", "tests/editor.test.ts"], dependsOn: ["t-3"] });
  expect(save.mock.calls[0][2].slice(1)).toEqual(plan.tasks.slice(1));
  expect(browserDrafts()).toEqual({}); expect(createSession).not.toHaveBeenCalled();
  const prompt = implementationPrompt(brief, { ...plan, tasks: [task, ...plan.tasks.slice(1)] }, task);
  expect(prompt).toContain("Task acceptance: Write an entry, reload");
  expect(prompt).toContain("Set up project (t-3)");
  expect(prompt).toContain("src/editor, tests/editor.test.ts");
  expect(prompt).toContain("Stop and explain if the task needs changes beyond this scope.");
});

it("keeps new coordination fields through browser recovery and a read-only downgrade without silent edits", async () => {
  const save = vi.spyOn(productApi, "savePlan");
  let mounted = render(<ProjectPlan projectId="p-1" brief={brief} />);
  const dependencies = await openScope(1);
  fireEvent.change(screen.getByLabelText("How to verify task 1"), { target: { value: "Keep my unfinished acceptance\n" } });
  fireEvent.change(screen.getByLabelText("Intended files and folders for task 1"), { target: { value: "src/editor\n" } });
  await userEvent.click(within(dependencies).getByRole("checkbox", { name: "Set up project · Task 3" }));
  const retained = browserDrafts();
  mounted.unmount();
  mounted = render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByRole("region", { name: "Implementation plan draft recovery" });
  await openScope(1);
  expect((screen.getByLabelText("How to verify task 1") as HTMLTextAreaElement).value).toBe("");
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  const acceptance = screen.getByLabelText("How to verify task 1") as HTMLTextAreaElement;
  const paths = screen.getByLabelText("Intended files and folders for task 1") as HTMLTextAreaElement;
  expect(acceptance.value).toBe("Keep my unfinished acceptance\n"); expect(paths.value).toBe("src/editor\n");
  expect((within(screen.getByRole("group", { name: "Task 1 waits for" })).getByRole("checkbox", { name: "Set up project · Task 3" }) as HTMLInputElement).checked).toBe(true);
  teamAccess.permissions.edit = false;
  mounted.rerender(<ProjectPlan projectId="p-1" brief={brief} />);
  expect(acceptance.disabled).toBe(true); expect(paths.disabled).toBe(true);
  fireEvent.change(acceptance, { target: { value: "Unauthorized overwrite" } });
  expect(acceptance.value).toBe("Keep my unfinished acceptance\n");
  expect(JSON.parse(Object.values(browserDrafts())[0]!).value).toEqual(JSON.parse(Object.values(retained)[0]!).value);
  expect(save).not.toHaveBeenCalled(); expect(createSession).not.toHaveBeenCalled();
});

it("merges a collaborator's file scope with draft dependencies and explicitly resolves overlapping acceptance", async () => {
  const original: ImplementationPlan = { ...plan, tasks: plan.tasks.map((task, index) => index === 0 ? { ...task, acceptance: "Initial acceptance", writePaths: ["src/editor"] } : task) };
  vi.mocked(productApi.plan).mockResolvedValue(original);
  const save = vi.spyOn(productApi, "savePlan").mockRejectedValueOnce(new Error("The saved plan changed. Reload it before saving."));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const dependencies = await openScope(1);
  fireEvent.change(screen.getByLabelText("How to verify task 1"), { target: { value: "Offline recovery preserves the last saved entry" } });
  await userEvent.click(within(dependencies).getByRole("checkbox", { name: "Set up project · Task 3" }));
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("The saved plan changed. Reload it before saving.");
  const current = { ...original, revision: 5, tasks: original.tasks.map((task, index) => index === 0 ? { ...task, acceptance: "A teammate's acceptance", writePaths: ["src/editor", "tests/recovery.test.ts"] } : task) };
  vi.mocked(productApi.plan).mockResolvedValue(current);
  save.mockImplementation(async (_id, _revision, tasks) => ({ ...current, tasks, revision: 6 }));
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  await userEvent.click(await screen.findByRole("radio", { name: "Use my draft for Tasks · Build editor · How to verify this task" }));
  await userEvent.click(screen.getByText("Review combined draft"));
  const combined = await screen.findByText("Waits for tasks");
  expect(within(screen.getByRole("list", { name: "Waits for tasks" })).getByText("Set up project")).not.toBeNull();
  expect(within(combined.closest(".draft-document-review")!).getByText("tests/recovery.test.ts")).not.toBeNull();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version and these recovery choices/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("How to verify task 1") as HTMLTextAreaElement).value).toBe("Offline recovery preserves the last saved entry");
  expect((screen.getByLabelText("Intended files and folders for task 1") as HTMLTextAreaElement).value).toBe("src/editor\ntests/recovery.test.ts");
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan saved as revision 6.");
  expect(save.mock.calls[1][1]).toBe(5);
  expect(save.mock.calls[1][2][0]).toEqual({ ...current.tasks[0], acceptance: "Offline recovery preserves the last saved entry", dependsOn: ["t-3"] });
});

it("requires deliberate removal of a missing dependency and prevents a dependency loop before saving", async () => {
  vi.mocked(productApi.plan).mockResolvedValue({ ...plan, tasks: plan.tasks.map((task, index) => index === 0 ? { ...task, dependsOn: ["t-3"] } : task) });
  const save = vi.spyOn(productApi, "savePlan");
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.click(await screen.findByRole("button", { name: "Remove task 3" }));
  await screen.findByText(/A task still depends on a removed task/);
  expect((screen.getByRole("button", { name: "Save implementation plan" }) as HTMLButtonElement).disabled).toBe(true);
  const first = await openScope(1);
  await userEvent.click(within(first).getByRole("checkbox", { name: /t-3 · Unavailable task/ }));
  expect(screen.queryByText(/A task still depends on a removed task/)).toBeNull();
  await userEvent.click(within(first).getByRole("checkbox", { name: "Save entries · Task 2" }));
  const second = await openScope(2);
  await userEvent.click(within(second).getByRole("checkbox", { name: "Build editor · Task 1" }));
  await screen.findByText(/These dependencies form a loop/);
  expect((screen.getByRole("button", { name: "Save implementation plan" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(within(second).getByRole("checkbox", { name: "Build editor · Task 1" }));
  expect(screen.queryByText(/These dependencies form a loop/)).toBeNull();
  expect((screen.getByRole("button", { name: "Save implementation plan" }) as HTMLButtonElement).disabled).toBe(false);
  expect(save).not.toHaveBeenCalled(); expect(createSession).not.toHaveBeenCalled();
});

it("shows saved dependencies and shared scope without presenting human progress as runtime or verification", async () => {
  const graphed: ImplementationPlan = { ...plan, graph: { waves: [["t-3"], ["t-1", "t-2"]], readyTaskIds: ["t-1"], blocked: [], conflicts: [{ taskIds: ["t-1", "t-2"], paths: ["src/editor", "src/editor/store.ts"] }], conflictsOmitted: 2, unscopedTaskIds: [], recordedDoneTaskIds: ["t-3"] } };
  vi.mocked(productApi.plan).mockResolvedValue(graphed);
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const graph = await screen.findByRole("region", { name: "Saved task coordination" });
  expect(within(graph).getByRole("list", { name: "Dependency waves" }).children).toHaveLength(2);
  expect(within(graph).getByText("3 potential file overlaps")).not.toBeNull();
  expect(within(graph).getByText("src/editor/store.ts")).not.toBeNull();
  await userEvent.click(within(graph).getByText(/Recorded progress ·/));
  expect(within(graph).getByText(/It does not mean an agent ran, changes were integrated, or checks passed/)).not.toBeNull();
  fireEvent.change(screen.getByLabelText("Task 1"), { target: { value: "Unsaved replacement title" } });
  expect(within(graph).getByText(/This is the saved plan/)).not.toBeNull();
  expect(within(graph).queryByText("Unsaved replacement title")).toBeNull();
  expect(createSession).not.toHaveBeenCalled();
});

it("accepts legacy and extended browser drafts and safely rejects malformed optional scope fields", () => {
  expect(planDraftValid({ tasks: plan.tasks, title: "" })).toBe(true);
  expect(planDraftValid({ tasks: [{ ...plan.tasks[0], acceptance: "Expected result", dependsOn: ["t-3"], writePaths: ["src/editor", ""] }], title: "" })).toBe(true);
  for (const invalid of [{ dependsOn: "t-3" }, { writePaths: [1] }, { acceptance: {} }, { requirementIds: {} }]) {
    expect(planDraftValid({ tasks: [{ ...plan.tasks[0], ...invalid }], title: "" })).toBe(false);
  }
});

it("requires current definition and design approvals and provides their review paths", async () => {
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: [], coverage: { ...coverage, specification: { ...coverage.specification, status: "draft" } } });
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByText("Approve the current definition in Discover before starting implementation.");
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
  teamAccess.permissions.edit = false; teamAccess.permissions.execute = false; teamAccess.permissions.secrets = false;
  const originalContext = snapshot({ briefRevision: 1, planRevision: 2, taskTitles: ["Original editor title"], sourceDirty: true });
  useCockpit.getState().upsertSession(aSession({ id: "s-old", title: "Prior editor run", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "exited", exitReason: "Process exited with code 0", implementationContext: originalContext, reviewedContextDelivery: { status: "captured", version: 1, digest: "a".repeat(64) } }));
  reviewedContext.mockResolvedValue({ sessionId: "s-old", projectId: "p-1", worktreeId: "w-1", version: 1, digest: "a".repeat(64), implementationContext: originalContext, document: { kind: "definition", sha256: "b".repeat(64), bytes: 40, value: { purpose: "Original editor scope" } } });
  useCockpit.getState().upsertSession(aSession({ id: "s-foreign", title: "Foreign run", projectId: "p-2", implementationContext: snapshot() }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const history = await screen.findByRole("region", { name: "Implementation sessions" });
  expect(within(history).getByText("Prior editor run · Ended")).not.toBeNull();
  expect(within(history).getByText(/Brief 1 · Plan 2.*Earlier saved context/)).not.toBeNull();
  expect(within(history).getByText("Original editor title")).not.toBeNull();
  expect(within(history).getByText(/Completion and verification are not inferred/)).not.toBeNull();
  expect(screen.queryByText(/Foreign run/)).toBeNull();
  expect((screen.getByLabelText("Progress for task 1") as HTMLSelectElement).value).toBe("todo");
  await userEvent.click(within(history).getByRole("button", { name: "View reviewed context" }));
  const dialog = await screen.findByRole("dialog", { name: "Reviewed context" });
  await within(dialog).findByText("Original editor scope");
  expect(useCockpit.getState().selection.sessionId).toBeNull();
  expect(reviewedContext).toHaveBeenCalledExactlyOnceWith("s-old", "definition");
  expect(createSession).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole("button", { name: "Close reviewed context" }));
  await userEvent.click(within(history).getByRole("button", { name: "Open session" }));
  expect(useCockpit.getState().selection.sessionId).toBe("s-old");
});

it("explains a reviewed coding attempt that ended without linked candidate or checks and offers a deliberate retry", async () => {
  useCockpit.getState().upsertSession(aSession({ id: "s-no-edit", title: "Orchard task", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "opencode", status: "exited", exitReason: "process exited", implementationContext: snapshot(), implementationOutcome: { state: "ended", evidence: "session-only" } }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const history = await screen.findByRole("region", { name: "Implementation sessions" });
  expect(within(history).getByText(/No candidate or passing checks are linked to this coding session/)).not.toBeNull();
  await userEvent.click(within(history).getByRole("button", { name: "Try this task again" }));
  expect(screen.getByRole("dialog", { name: "New session" })).not.toBeNull();
  expect(createSession).not.toHaveBeenCalled();
});

it("keeps a quiet native agent and an uncertain lost runtime distinct from a completed attempt", async () => {
  useCockpit.getState().upsertSession(aSession({ id: "s-quiet", title: "Agent awaiting input", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "idle", implementationContext: snapshot(), implementationOutcome: { state: "quiet", evidence: "session-only" } }));
  useCockpit.getState().upsertSession(aSession({ id: "s-unknown", title: "Lost coding runtime", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "exited", exitReason: "sandbox disappeared", implementationContext: snapshot(), implementationOutcome: { state: "unknown", evidence: "session-only" } }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const history = await screen.findByRole("region", { name: "Implementation sessions" });
  expect(within(history).getByText(/may still be working or waiting for Ask permission/)).not.toBeNull();
  expect(within(history).getByText(/resolve its runtime state before starting another attempt/)).not.toBeNull();
  expect(within(history).queryByRole("button", { name: "Try this task again" })).toBeNull();
  expect(createSession).not.toHaveBeenCalled();
});

it("shows an owned OpenCode final turn with only reads while its Ask process stays alive", async () => {
  useCockpit.getState().upsertSession(aSession({ id: "s-native-finish", title: "Orchard task", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "opencode", status: "idle", implementationContext: snapshot(), implementationOutcome: { state: "quiet", evidence: "session-only", nativeTurnId: "msg_67d5b111", nativeToolClass: "reads-only" } }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const history = await screen.findByRole("region", { name: "Implementation sessions" });
  expect(within(history).getByText(/latest OpenCode turn ended while this Ask session remains open/)).not.toBeNull();
  expect(within(history).getByText(/no implementation or checks are linked/)).not.toBeNull();
  expect(within(history).getByRole("button", { name: "Open session" })).not.toBeNull();
  expect(within(history).queryByRole("button", { name: "Try this task again" })).toBeNull();
  expect(createSession).not.toHaveBeenCalled();
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

it("does not lose an early recovery attempt when a manual saved-plan read finishes later", async () => {
  let finish!: (value: ImplementationPlan) => void;
  const save = vi.spyOn(productApi, "savePlan").mockRejectedValue(new Error("The saved plan changed. Reload it before saving."));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  fireEvent.change(await screen.findByLabelText("Task 1"), { target: { value: "Keep my edited task" } });
  fireEvent.change(screen.getByLabelText("New task"), { target: { value: "Keep my unfinished title" } });
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("The saved plan changed. Reload it before saving.");
  vi.mocked(productApi.plan).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  await act(async () => finish(plan));
  // A pending read must keep recovery available, rather than consuming the
  // draft and then overwriting its restored values with the delayed response.
  const restore = screen.queryByRole("button", { name: "Restore draft" });
  if (restore) await userEvent.click(restore);
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Keep my edited task");
  expect((screen.getByLabelText("New task") as HTMLInputElement).value).toBe("Keep my unfinished title");
  fireEvent.change(screen.getByLabelText("New task"), { target: { value: "Keep my unfinished title and this next edit" } });
  const stored = JSON.parse(Object.values(browserDrafts())[0]!);
  expect(stored.value.tasks[0].title).toBe("Keep my edited task");
  expect(stored.value.title).toBe("Keep my unfinished title and this next edit");
  expect(save).toHaveBeenCalledOnce(); expect(createSession).not.toHaveBeenCalled();
});

function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (cause: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

it("keeps the exact draft locked after a failed reload, then merges and saves against the retried current revision", async () => {
  const reload = deferred<ImplementationPlan>(), saved = deferred<ImplementationPlan>(), prepare = vi.fn();
  const save = vi.spyOn(productApi, "savePlan").mockRejectedValueOnce(new Error("Plan revision conflict")).mockReturnValueOnce(saved.promise);
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={prepare} />);
  fireEvent.change(await screen.findByLabelText("Task 1"), { target: { value: "My offline editor" } });
  fireEvent.change(screen.getByLabelText("New task"), { target: { value: "My unfinished next task" } });
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan revision conflict");
  const retained = browserDrafts();
  vi.mocked(productApi.plan).mockReturnValueOnce(reload.promise);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  expect(screen.getByText("Loading saved plan… Your draft is kept while the current revision is checked.")).not.toBeNull();
  for (const name of ["Reload saved plan", "Restore draft", "Discard browser draft", "Save implementation plan", "Start implementation session", "Prepare project run", "Add task"]) {
    const button = screen.getByRole("button", { name }) as HTMLButtonElement;
    expect(button.disabled).toBe(true); fireEvent.click(button);
  }
  fireEvent.change(screen.getByLabelText("Task 1"), { target: { value: "Must not replace the task" } });
  fireEvent.change(screen.getByLabelText("New task"), { target: { value: "Must not replace the title" } });
  fireEvent.submit(screen.getByLabelText("New task").closest("form")!);
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("My offline editor");
  expect((screen.getByLabelText("New task") as HTMLInputElement).value).toBe("My unfinished next task");
  expect(productApi.plan).toHaveBeenCalledTimes(2);
  await act(async () => reload.reject(new Error("Saved plan temporarily unavailable")));
  expect(screen.getByText("Reload the saved plan before editing or recovering your draft. Your browser draft is unchanged.")).not.toBeNull();
  for (const name of ["Restore draft", "Discard browser draft", "Save implementation plan", "Start implementation session", "Prepare project run"]) expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByLabelText("New task") as HTMLInputElement).disabled).toBe(true);
  expect(browserDrafts()).toEqual(retained);
  expect(save).toHaveBeenCalledOnce(); expect(prepare).not.toHaveBeenCalled(); expect(createSession).not.toHaveBeenCalled();

  const current = { ...plan, revision: 9, tasks: plan.tasks.map((task, index) => ({ ...task, title: index === 0 ? "Peer editor change" : index === 1 ? "Peer durable entry recovery" : task.title })) };
  vi.mocked(productApi.plan).mockResolvedValueOnce(current);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  await userEvent.click(await screen.findByRole("radio", { name: /Use my draft for Tasks.*Title/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed the saved version and these recovery choices/ }));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("My offline editor");
  expect((screen.getByLabelText("Task 2") as HTMLInputElement).value).toBe("Peer durable entry recovery");
  expect((screen.getByLabelText("New task") as HTMLInputElement).value).toBe("My unfinished next task");
  await userEvent.click(screen.getByRole("button", { name: "Add task" }));
  const saveButton = screen.getByRole("button", { name: "Save implementation plan" });
  await userEvent.click(saveButton);
  fireEvent.click(saveButton);
  expect(save).toHaveBeenCalledTimes(2);
  expect(save.mock.calls[1]).toEqual(["p-1", 9, expect.any(Array), 6]);
  expect(save.mock.calls[1][2].map(task => task.title)).toEqual(["My offline editor", "Peer durable entry recovery", "Set up project", "My unfinished next task"]);
  expect((screen.getByRole("button", { name: "Prepare project run" }) as HTMLButtonElement).disabled).toBe(true);
  await act(async () => saved.resolve({ ...current, revision: 10, tasks: save.mock.calls[1][2] }));
  await screen.findByText("Plan saved as revision 10.");
  expect(browserDrafts()).toEqual({});
  await waitFor(() => expect((screen.getByRole("button", { name: "Prepare project run" }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(screen.getByRole("button", { name: "Prepare project run" }));
  expect(prepare).toHaveBeenCalledExactlyOnceWith({ worktreeId: "w-1", implementationContext: { projectId: "p-1", briefRevision: 2, planRevision: 10, specificationRevision: 6, designPlanRevision: 8, taskIds: ["t-1", "t-2", save.mock.calls[1][2][3].id] } });
  expect(createSession).not.toHaveBeenCalled();
});

it("does not admit a second reload in the same event turn", async () => {
  const pending = deferred<ImplementationPlan>();
  vi.mocked(productApi.plan).mockRejectedValueOnce(new Error("Initial read unavailable")).mockReturnValueOnce(pending.promise);
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  const reload = await screen.findByRole("button", { name: "Reload saved plan" });
  act(() => { reload.click(); reload.click(); });
  expect(productApi.plan).toHaveBeenCalledTimes(2);
  await act(async () => pending.resolve(plan));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Build editor");
});

it.each([
  ["account/workspace", "success"], ["account/workspace", "error"],
  ["project", "success"], ["project", "error"],
])("ignores a delayed manual-read %s scope response ending in %s", async (change, outcome) => {
  const pending = deferred<ImplementationPlan>();
  vi.spyOn(productApi, "savePlan").mockRejectedValue(new Error("Old plan conflict"));
  const view = render(<ProjectPlan projectId="p-1" brief={brief} />);
  fireEvent.change(await screen.findByLabelText("Task 1"), { target: { value: "Old private task draft" } });
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Old plan conflict");
  vi.mocked(productApi.plan).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  const projectId = change === "project" ? "p-2" : "p-1";
  if (change === "account/workspace") draftIdentity.value = "different-account-and-workspace";
  vi.mocked(productApi.plan).mockResolvedValueOnce({ ...plan, projectId, tasks: [{ ...plan.tasks[0], title: "Current scope task" }] });
  view.rerender(<ProjectPlan projectId={projectId} brief={{ ...brief, projectId }} />);
  await screen.findByDisplayValue("Current scope task");
  await act(async () => outcome === "error"
    ? pending.reject(new Error("Private error from old workspace"))
    : pending.resolve({ ...plan, tasks: [{ ...plan.tasks[0], title: "Late saved task from old workspace" }] }));
  expect(screen.queryByText(/Private error|Old plan conflict/)).toBeNull();
  expect(screen.queryByDisplayValue("Late saved task from old workspace")).toBeNull();
  expect(screen.queryByRole("region", { name: "Implementation plan draft recovery" })).toBeNull();
  fireEvent.change(screen.getByLabelText("Task 1"), { target: { value: "Current scope edit" } });
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Current scope edit");
  const stored = Object.values(browserDrafts()).map(raw => JSON.parse(raw!));
  expect(stored.map(row => row.value.tasks[0].title).sort()).toEqual(["Current scope edit", "Old private task draft"]);
  expect(createSession).not.toHaveBeenCalled();
});

it("rejects a wrong-project saved plan without enabling recovery against it", async () => {
  vi.mocked(productApi.plan).mockResolvedValueOnce({ ...plan, projectId: "p-foreign", tasks: [{ ...plan.tasks[0], title: "Foreign private task" }] });
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByText("The saved plan belongs to another project. Reload the current project's plan.");
  expect(screen.queryByDisplayValue("Foreign private task")).toBeNull();
  expect(screen.queryByRole("button", { name: "Start implementation session" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  await screen.findByDisplayValue("Build editor");
});

it("does not accept a pending saved read after confirmed read revocation and requires a fresh read after access returns", async () => {
  const pending = deferred<ImplementationPlan>();
  vi.spyOn(productApi, "savePlan").mockRejectedValue(new Error("Plan conflict"));
  const view = render(<ProjectPlan projectId="p-1" brief={brief} />);
  fireEvent.change(await screen.findByLabelText("Task 1"), { target: { value: "Retain this task" } });
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan conflict");
  vi.mocked(productApi.plan).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  const retained = browserDrafts();
  teamAccess.permissions.read = false;
  view.rerender(<ProjectPlan projectId="p-1" brief={brief} />);
  await act(async () => pending.resolve({ ...plan, revision: 8, tasks: [{ ...plan.tasks[0], title: "Response after access was removed" }] }));
  expect(screen.queryByDisplayValue("Response after access was removed")).toBeNull();
  expect((screen.getByRole("button", { name: "Reload saved plan" }) as HTMLButtonElement).disabled).toBe(true);
  expect(browserDrafts()).toEqual(retained);
  teamAccess.permissions.read = true;
  view.rerender(<ProjectPlan projectId="p-1" brief={brief} />);
  expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).disabled).toBe(true);
  expect(productApi.plan).toHaveBeenCalledTimes(2);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved plan" }));
  await waitFor(() => expect((screen.getByRole("button", { name: "Restore draft" }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(screen.getByRole("button", { name: "Restore draft" }));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Retain this task");
  expect(productApi.plan).toHaveBeenCalledTimes(3); expect(createSession).not.toHaveBeenCalled();
});
