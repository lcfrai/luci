import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ProjectUnderstanding } from "./ProjectUnderstanding";
import { context, view } from "./fixtures";
import type { OrchestratorRunSummary } from "../../api/orchestrator";

it("explains the saved scope and next stage while allowing a different stage to be inspected", async () => {
  const current = view(), navigate = vi.fn(), ask = vi.fn();
  render(<ProjectUnderstanding view={current} onAsk={ask} onReply={vi.fn()} onNavigate={navigate} />);
  const nav = screen.getByRole("navigation", { name: "Project stages" });
  expect(within(nav).getByRole("button", { name: "Open Design stage" }).getAttribute("aria-current")).toBe("step");
  expect(screen.getByText(current.context.specification.purpose)).not.toBeNull();
  expect(screen.getByRole("heading", { name: "Agreed outcomes" })).not.toBeNull();
  await userEvent.click(within(nav).getByRole("button", { name: "Open Develop stage" }));
  expect(navigate).toHaveBeenCalledExactlyOnceWith("develop");
  expect(within(nav).getByRole("button", { name: "Open Design stage" }).getAttribute("aria-current")).toBe("step");
  expect(ask).not.toHaveBeenCalled();
});

it("shows what the active build was actually asked to do when the current draft has changed", () => {
  const snapshot = context();
  snapshot.specification.purpose = "The amended application";
  snapshot.specification.status = "draft";
  snapshot.plan.tasks = [{ id: "later", title: "A newly proposed task", status: "todo" }];
  snapshot.currentRun = { id: "run", state: "coding", valid: false, reason: "The definition changed", outcome: "preview", specificationRevision: 2, designRevision: 3,
    tasks: [{ id: "selected", title: "Implement the reviewed guest checkout", status: "selected" }],
    requirements: [{ id: "AC-old", title: "Guest checkout", acceptance: "A guest can complete checkout without an account" }] };
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Implement the reviewed guest checkout")).not.toBeNull();
  expect(within(build).queryByText("A newly proposed task")).toBeNull();
  expect(within(build).getByText(/Uses reviewed definition 2 and design 3/)).not.toBeNull();
  expect(screen.getByText(/Draft definition/)).not.toBeNull();
  expect(within(build).getByText(/does not prove the current application is ready/)).not.toBeNull();
});

it("makes amendment and clarification paths explicit without starting an operation", async () => {
  const ask = vi.fn(), navigate = vi.fn();
  render(<ProjectUnderstanding view={view({ blockingQuestions: [{ turnId: "q", question: "May guests see the customer list?", rationale: "This changes the privacy boundary.", options: [] }] })} onAsk={ask} onReply={vi.fn()} onNavigate={navigate} />);
  expect(screen.getByRole("region", { name: "Decisions to resolve" })).not.toBeNull();
  expect(screen.getByText("This changes the privacy boundary.")).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  expect(ask).toHaveBeenCalledExactlyOnceWith();
  expect(navigate).not.toHaveBeenCalled();
});

it("describes an accepted preview without claiming a public live deployment", () => {
  const snapshot = context(); snapshot.currentRun = { id: "run", state: "complete", valid: true, reason: "Verified preview", outcome: "preview" };
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Run accepted")).not.toBeNull();
  expect(build.textContent).toContain("Preview target");
  expect(build.textContent).not.toMatch(/public.*live|is live/i);
});

it("binds the unresolved decision when an answer is the next step", async () => {
  const ask = vi.fn(), reply = vi.fn(), navigate = vi.fn();
  render(<ProjectUnderstanding view={view({ blockingQuestions: [{ turnId: "privacy-decision", question: "Can guests see other bookings?", rationale: "Privacy boundary", options: [] }], nextAction: { id: "answer-question", label: "Answer the open question", reason: "The privacy boundary needs a decision.", lens: "discover" } })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  await userEvent.click(within(screen.getByRole("region", { name: "Next step" })).getByRole("button", { name: "Answer the open question" }));
  expect(reply).toHaveBeenCalledExactlyOnceWith("privacy-decision");
  expect(ask).not.toHaveBeenCalled();
  expect(navigate).not.toHaveBeenCalled();
});

it("keeps an older coding scope visible through newer preparation, parent cancellation and unconfirmed recovery", async () => {
  const active: OrchestratorRunSummary = {
    id: "run-a", state: "coding", valid: false, reason: "The selected session is working", activity: "running", dispatchCancelled: false,
    operations: [{ id: "operation-a", childId: "session-a", status: "running", updatedAt: "2026-10-06T14:00:00Z" }],
    tasks: [{ id: "task-a", title: "Implement private guest booking", status: "selected" }],
    requirements: [{ id: "REQ-a", title: "Private recovery", acceptance: "Only the guest may recover their booking" }],
    specificationRevision: 2, designRevision: 3, outcome: "preview",
  };
  const snapshot = context(); snapshot.currentRun = active; snapshot.activeRuns = [active];
  const navigate = vi.fn(), ask = vi.fn(), reply = vi.fn();
  const { rerender } = render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  expect(screen.getAllByText("Implement private guest booking")).toHaveLength(1);
  expect(screen.queryByRole("region", { name: "Latest prepared build" })).toBeNull();

  const prepared: OrchestratorRunSummary = {
    id: "run-b", state: "ready", valid: false, reason: "Prepared only", activity: "none", dispatchCancelled: false, operations: [],
    tasks: [{ id: "task-b", title: "Add calendar export", status: "selected" }], specificationRevision: 4, designRevision: 5, outcome: "preview",
  };
  snapshot.currentRun = prepared;
  rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  let ongoing = screen.getByRole("region", { name: "Active and unconfirmed work" });
  const latest = screen.getByRole("region", { name: "Latest prepared build" });
  expect(within(ongoing).getByText("Implement private guest booking")).not.toBeNull();
  expect(within(ongoing).queryByText("Add calendar export")).toBeNull();
  expect(within(ongoing).getByText("Only the guest may recover their booking")).not.toBeNull();
  expect(within(ongoing).getByText(/Uses reviewed definition 2 and design 3/)).not.toBeNull();
  expect(within(latest).getByText("Add calendar export")).not.toBeNull();
  expect(within(latest).queryByText("Implement private guest booking")).toBeNull();

  const cancelled = { ...active, state: "cancelled", dispatchCancelled: true, reason: "Future dispatch is cancelled." };
  snapshot.activeRuns = [cancelled];
  rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  ongoing = screen.getByRole("region", { name: "Active and unconfirmed work" });
  expect(within(ongoing).getByText("Last recorded: Running")).not.toBeNull();
  expect(within(ongoing).getByText("Further work cancelled.")).not.toBeNull();
  expect(ongoing.textContent).toContain("This does not stop an agent already running");
  expect(ongoing.textContent).not.toMatch(/\bstopped\b/i);
  await userEvent.click(within(ongoing).getByRole("button", { name: "Inspect build and checks" }));
  expect(navigate).toHaveBeenCalledExactlyOnceWith("develop");
  expect(ask).not.toHaveBeenCalled(); expect(reply).not.toHaveBeenCalled();

  snapshot.activeRuns = [{ ...cancelled, activity: "unknown", operations: [{ ...active.operations![0], status: "unknown" }] }];
  rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  ongoing = screen.getByRole("region", { name: "Active and unconfirmed work" });
  expect(within(ongoing).getByText("Last recorded: Outcome unconfirmed")).not.toBeNull();
  expect(within(ongoing).getByText("Implement private guest booking")).not.toBeNull();
  expect(ongoing.textContent).not.toMatch(/\bstopped\b/i);

  snapshot.activeRuns = [];
  rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} onAsk={ask} onReply={reply} onNavigate={navigate} />);
  expect(screen.queryByRole("region", { name: "Active and unconfirmed work" })).toBeNull();
  expect(within(screen.getByRole("region", { name: "In this build" })).getByText("Add calendar export")).not.toBeNull();
  expect(screen.queryByText("Implement private guest booking")).toBeNull();
});

it("keeps all recorded pending builds distinct and never describes reservation as running", () => {
  const snapshot = context();
  snapshot.activeRuns = ["running", "unknown", "pending"].map((activity, index) => ({
    id: `run-${index}`, state: "coding", valid: false, reason: "Recorded only", activity: activity as "running" | "unknown" | "pending", dispatchCancelled: false,
    operations: [{ id: `op-${index}`, childId: `session-${index}`, status: index === 2 ? "reserved" : index === 1 ? "unknown" : "running", updatedAt: "2026-10-06T14:00:00Z" }],
    tasks: [{ id: `task-${index}`, title: `Scope ${index}`, status: "selected" }],
  }));
  snapshot.currentRun = snapshot.activeRuns[2];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  for (let index = 0; index < 3; index++) {
    expect(within(screen.getByRole("region", { name: `Build ${index + 1}` })).getByText(`Scope ${index}`)).not.toBeNull();
    expect(screen.getAllByText(`Scope ${index}`)).toHaveLength(1);
  }
  expect(within(screen.getByRole("region", { name: "Build 3" })).getByText("Last recorded: Reserved, not started")).not.toBeNull();
});

it("does not claim cancellation stopped a child in older snapshots without activity data", () => {
  const snapshot = context(); snapshot.currentRun = { id: "old-run", state: "cancelled", valid: false, reason: "Inspect any previously allocated session." };
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Further work cancelled")).not.toBeNull();
  expect(build.textContent).not.toMatch(/\bstopped\b/i);
});

it("discloses omitted scope and activity with exact counts and inspection of that specific build", async () => {
  const snapshot = context(), inspect = vi.fn(), navigate = vi.fn();
  snapshot.plan.tasks = [{ id: "mutable-task", title: "Unrelated current plan task", status: "todo" }];
  snapshot.activeRuns = [{ id: "large-older-build", state: "cancelled", reason: "", valid: false, activity: "unknown", dispatchCancelled: true,
    scopeOmitted: true, taskCount: 80, requirementCount: 42, operationsOmitted: true, operationCount: 3, tasks: [], requirements: [], operations: [] }];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={navigate} onInspect={inspect} />);
  const active = screen.getByRole("region", { name: "Active and unconfirmed work" });
  expect(within(active).getByText("80 selected tasks / 42 outcomes; inspect this build for its full saved scope.")).not.toBeNull();
  expect(within(active).getByText("3 unfinished coding attempts; inspect this build for their full recorded activity.")).not.toBeNull();
  expect(within(active).queryByText("Unrelated current plan task")).toBeNull();
  expect(within(active).getByText("Last recorded: Outcome unconfirmed")).not.toBeNull();
  await userEvent.click(within(active).getByRole("button", { name: "Inspect build and checks" }));
  expect(inspect).toHaveBeenCalledExactlyOnceWith("large-older-build");
  expect(navigate).not.toHaveBeenCalled();
});
