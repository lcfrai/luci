import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { productApi, type Brief, type CheckRun } from "../../api/product";
import { ProjectChecks } from "./ProjectChecks";
import { ProjectPlan } from "./ProjectPlan";
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
  ]);
  expect(screen.queryByText(/Plan saved as revision/)).toBeNull();
});

it("hands a saved task and project context to an actual session only after reviewing the picker", async () => {
  useCockpit.setState({
    harnesses: [
      aHarness({
        available: true,
        installed: true,
        flagsVerified: true,
        supportsOpeningPrompt: true,
      }),
    ],
    projects: [aProject({ id: "p-1" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", branchId: "b-1", ready: true })],
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
  });
});
