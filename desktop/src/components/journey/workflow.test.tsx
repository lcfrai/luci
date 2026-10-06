import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { productApi, type Brief, type CheckRun } from "../../api/product";
import { ProjectChecks } from "./ProjectChecks";
import { ProjectPlan } from "./ProjectPlan";
import { DraftNavigationDialog } from "../common/DraftNavigationDialog";
import { resetCockpit, useCockpit } from "../../store/store";
import {
  aBranch,
  aHarness,
  aProject,
  aSession,
  aWorktree,
} from "../../test/fixtures";
const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));
const brief: Brief = {
  projectId: "p-1",
  revision: 2,
  purpose: "Offline journal",
  audience: "Writers",
  constraints: "Private",
  criteria: ["Keep notes offline"],
  updatedAt: "",
};
const run: CheckRun = {
  id: "check-1",
  projectId: "p-1",
  commit: "123456789",
  dirty: true,
  status: "failed",
  commands: [
    {
      label: "Tests",
      executable: "npm",
      args: ["test"],
      status: "failed",
      exitCode: 1,
      output: "Expected notes to persist",
      truncated: false,
    },
  ],
  createdAt: "2026-10-02T00:00:00Z",
  updatedAt: "",
  message: "Tests failed",
};
beforeEach(() => {
  vi.restoreAllMocks();
  resetCockpit();
  createSession.mockReset();
  vi.spyOn(productApi, "checks").mockResolvedValue({ runs: [], suggested: [], coverage: { projectId: "p-1", specification: { revision: 6, status: "approved", digest: "spec-6" }, design: { revision: 8, status: "approved", digest: "design-8" }, source: { commit: "a".repeat(40), sourceDigest: "source-a", dirty: false, available: true }, rows: [] } });
  vi.spyOn(productApi, "agentConnections").mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
  vi.spyOn(productApi, "agents").mockResolvedValue({ providers: [] });
});

it("requires reviewed host trust before running real checks and shows a failing command honestly", async () => {
  vi.spyOn(productApi, "checks").mockResolvedValue({
    runs: [],
    suggested: [{ label: "Tests", executable: "npm", args: ["test"] }],
  });
  const start = vi.spyOn(productApi, "runChecks").mockResolvedValue(run);
  render(<ProjectChecks projectId="p-1" />);
  await screen.findByText("Tests");
  expect(start).not.toHaveBeenCalled();
  expect(
    (
      screen.getByRole("button", {
        name: "Run project checks",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(
    screen.getByRole("button", { name: "Run project checks" }),
  );
  expect(await screen.findByText("Tests failed")).not.toBeNull();
  expect(start).toHaveBeenCalledWith(
    "p-1",
    [{ label: "Tests", executable: "npm", args: ["test"] }],
    true,
    "docker",
  );
  expect(screen.getByText(/includes uncommitted changes/)).not.toBeNull();
  await userEvent.click(screen.getByText("Tests · failed · exit 1"));
  expect(screen.getByText("Expected notes to persist")).not.toBeNull();
});

it("retains edited tasks when saving encounters a revision conflict", async () => {
  vi.spyOn(productApi, "plan").mockResolvedValue({
    projectId: "p-1",
    revision: 3,
    briefRevision: 2,
    tasks: [{ id: "t1", title: "Build editor", status: "todo" }],
    updatedAt: "",
  });
  const save = vi
    .spyOn(productApi, "savePlan")
    .mockRejectedValue(new Error("The plan changed. Reload saved plan."));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.selectOptions(
    await screen.findByLabelText("Progress for task 1"),
    "doing",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save implementation plan" }),
  );
  await screen.findByRole("alert");
  expect(
    (screen.getByLabelText("Progress for task 1") as HTMLSelectElement).value,
  ).toBe("doing");
  expect(save).toHaveBeenCalledWith("p-1", 3, [
    { id: "t1", title: "Build editor", status: "doing" },
  ], 6);
  expect(screen.queryByText(/Plan saved as revision/)).toBeNull();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([
    { projectId: "p-1", label: "Implementation plan", lens: "develop" },
  ]);
});

const savedPlan = {
  projectId: "p-1", revision: 3, briefRevision: 2, specificationRevision: 6,
  tasks: [{ id: "t1", title: "Build editor", status: "todo" as const }], updatedAt: "",
};
function selectPlanProject() {
  useCockpit.setState({
    projects: [aProject({ id: "p-1" }), aProject({ id: "p-2" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", isPrimary: true })] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
    lens: "develop",
  });
}
function PlanNavigation() {
  const selection = useCockpit(state => state.selection);
  const lens = useCockpit(state => state.lens);
  return <>
    {selection.projectId === "p-1" && lens === "develop"
      ? <ProjectPlan projectId="p-1" brief={brief} />
      : <p>Destination {selection.projectId} · {lens}</p>}
    <DraftNavigationDialog />
  </>;
}

it("keeps an implementation-task edit on phase navigation and explicitly discards to the chosen project's Files", async () => {
  selectPlanProject();
  vi.spyOn(productApi, "plan").mockResolvedValue(savedPlan);
  const save = vi.spyOn(productApi, "savePlan");
  render(<PlanNavigation />);
  await userEvent.clear(await screen.findByLabelText("Task 1"));
  await userEvent.type(screen.getByLabelText("Task 1"), "Keep this unsaved task");
  act(() => useCockpit.getState().setLens("design"));
  expect(useCockpit.getState().lens).toBe("develop");
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "lens", lens: "design" });
  await screen.findByRole("dialog", { name: "Keep your changes?" });
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Keep this unsaved task");
  expect(useCockpit.getState().pendingNavigation).toBeNull();
  act(() => useCockpit.getState().selectProject("p-2", "files"));
  expect(useCockpit.getState().selection.projectId).toBe("p-1");
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "project", projectId: "p-2", lens: "files" });
  await userEvent.click(screen.getByRole("button", { name: "Discard edits and continue" }));
  expect(screen.getByText("Destination p-2 · files")).not.toBeNull();
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  expect(save).not.toHaveBeenCalled();
});

it("guards an unadded task title and refuses a handoff until it is cleared or added", async () => {
  selectPlanProject();
  vi.spyOn(productApi, "plan").mockResolvedValue(savedPlan);
  const { unmount } = render(<ProjectPlan projectId="p-1" brief={brief} />);
  await screen.findByLabelText("Task 1");
  await userEvent.type(screen.getByLabelText("New task"), "Pending title");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([
    { projectId: "p-1", label: "Implementation plan", lens: "develop" },
  ]);
  expect((screen.getByRole("button", { name: "Implement task" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(true);
  act(() => useCockpit.getState().selectBranch("p-1", "b-2"));
  expect(useCockpit.getState().selection.branchId).toBe("b-1");
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "branch", projectId: "p-1", branchId: "b-2" });
  act(() => useCockpit.getState().cancelNavigation());
  expect((screen.getByLabelText("New task") as HTMLInputElement).value).toBe("Pending title");
  await userEvent.clear(screen.getByLabelText("New task"));
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  expect((screen.getByRole("button", { name: "Implement task" }) as HTMLButtonElement).disabled).toBe(false);
  await userEvent.type(screen.getByLabelText("New task"), "Retained on unmount only in this editor");
  unmount();
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  expect(createSession).not.toHaveBeenCalled();
});

it("a successful saved task does not clear a separate unsubmitted title or its navigation guard", async () => {
  selectPlanProject();
  vi.spyOn(productApi, "plan").mockResolvedValue(savedPlan);
  const save = vi.spyOn(productApi, "savePlan").mockResolvedValue({ ...savedPlan, revision: 4, tasks: [{ ...savedPlan.tasks[0], status: "doing" }] });
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.selectOptions(await screen.findByLabelText("Progress for task 1"), "doing");
  await userEvent.type(screen.getByLabelText("New task"), "Not yet added");
  await userEvent.click(screen.getByRole("button", { name: "Save implementation plan" }));
  await screen.findByText("Plan saved as revision 4.");
  expect(save).toHaveBeenCalledWith("p-1", 3, [{ id: "t1", title: "Build editor", status: "doing" }], 6);
  expect((screen.getByLabelText("New task") as HTMLInputElement).value).toBe("Not yet added");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  expect((screen.getByRole("button", { name: "Start implementation session" }) as HTMLButtonElement).disabled).toBe(true);
  act(() => useCockpit.getState().setLens("design"));
  expect(useCockpit.getState().lens).toBe("develop");
  act(() => useCockpit.getState().cancelNavigation());
  await userEvent.clear(screen.getByLabelText("New task"));
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  act(() => useCockpit.getState().setLens("design"));
  expect(useCockpit.getState().lens).toBe("design");
});

it("hands a saved task and project context to an actual session only after reviewing the picker", async () => {
  useCockpit.setState({
    harnesses: [
      aHarness({
        id: "claude-code",
        name: "Claude Code",
        command: "claude",
        available: true,
        installed: true,
        flagsVerified: true,
        supportsOpeningPrompt: true,
      }),
    ],
    projects: [aProject({ id: "p-1" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", isPrimary: true, ready: true })],
    },
    selection: {
      projectId: "p-1",
      branchId: "b-1",
      worktreeId: "w-1",
      sessionId: null,
    },
  });
  vi.spyOn(productApi, "plan").mockResolvedValue({
    projectId: "p-1",
    revision: 3,
    briefRevision: 2,
    specificationRevision: 6,
    tasks: [{ id: "t1", title: "Build editor", status: "todo" }],
    updatedAt: "",
  });
  createSession.mockResolvedValue(aSession({ id: "s-new", worktreeId: "w-1" }));
  render(<ProjectPlan projectId="p-1" brief={brief} />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Implement task" }),
  );
  const prompt = screen.getByRole("textbox", {
    name: "Opening prompt",
  }) as HTMLTextAreaElement;
  expect(prompt.value).toContain("Work on this task: Build editor");
  expect(prompt.value).toContain(".luci/design.json");
  expect(createSession).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Create session" }));
  await waitFor(() => expect(createSession).toHaveBeenCalled());
  expect(createSession.mock.calls[0][0]).toMatchObject({
    worktreeId: "w-1",
    openingPrompt: prompt.value,
    implementationContext: { projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["t1"] },
  });
});
