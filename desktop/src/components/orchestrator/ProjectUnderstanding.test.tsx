import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ProjectUnderstanding } from "./ProjectUnderstanding";
import { context, view } from "./fixtures";
import type { OrchestratorRunSummary } from "../../api/orchestrator";

it("targets the implementation plan for its next action even when the reader cannot edit", async () => {
  const edit = vi.fn(), navigate = vi.fn(), ask = vi.fn();
  render(<ProjectUnderstanding view={view({ nextAction: { id: "plan", label: "Review implementation tasks", reason: "Review the plan against the current definition.", lens: "develop" } })} onAsk={ask} onReply={vi.fn()} onNavigate={navigate} onEdit={edit} canEdit={false} />);
  await userEvent.click(screen.getByRole("button", { name: "Review implementation tasks" }));
  expect(edit).toHaveBeenCalledExactlyOnceWith("plan", undefined);
  expect(navigate).not.toHaveBeenCalled(); expect(ask).not.toHaveBeenCalled();
});

it("falls back to Develop for a plan review when no context destination handler is provided", async () => {
  const navigate = vi.fn();
  render(<ProjectUnderstanding view={view({ nextAction: { id: "plan", label: "Review implementation tasks", reason: "Review the current tasks.", lens: "develop" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={navigate} />);
  await userEvent.click(screen.getByRole("button", { name: "Review implementation tasks" }));
  expect(navigate).toHaveBeenCalledExactlyOnceWith("develop");
});

it("routes each manual question to its own definition answer while keeping model questions separate", async () => {
  const snapshot = context(), edit = vi.fn(), ask = vi.fn(), reply = vi.fn();
  snapshot.specification.questions = [{ id: "join", question: "How do members join?", answer: "" }, { id: "reassign", question: "Who can reassign tasks?", answer: "" }];
  render(<ProjectUnderstanding view={view({ context: snapshot, blockingQuestions: [{ turnId: "model-question", question: "Which storage region?", rationale: "Data location", options: [] }] })} onAsk={ask} onReply={reply} onNavigate={vi.fn()} onEdit={edit} />);
  const buttons = screen.getAllByRole("button", { name: "Answer in definition" });
  await userEvent.click(buttons[1]);
  expect(edit).toHaveBeenCalledExactlyOnceWith("definition", "reassign");
  expect(ask).not.toHaveBeenCalled(); expect(reply).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Work through the decisions" }));
  expect(reply).toHaveBeenCalledExactlyOnceWith("model-question");
});

it("keeps saved answers behind a deliberate disclosure and labels approval without promoting draft answers", async () => {
  const snapshot = context(), edit = vi.fn();
  snapshot.specification.questions = [{ id: "join", question: "How do members join?", answer: "Coordinators invite members." }];
  snapshot.specification.status = "draft";
  const props = { onAsk: vi.fn(), onReply: vi.fn(), onNavigate: vi.fn(), onEdit: edit };
  const mounted = render(<ProjectUnderstanding view={view({ context: snapshot })} {...props} />);
  const disclosure = screen.getByText("Saved answers").closest("details")!;
  expect(disclosure.open).toBe(false);
  expect(screen.queryByRole("region", { name: "Decisions to resolve" })).toBeNull();
  await userEvent.click(screen.getByText("Saved answers"));
  expect(within(disclosure).getByText("These saved answers need review with the current definition.")).not.toBeNull();
  expect(within(disclosure).getByText("Coordinators invite members.")).not.toBeNull();
  await userEvent.click(within(disclosure).getByRole("button", { name: "Edit answer in definition" }));
  expect(edit).toHaveBeenCalledExactlyOnceWith("definition", "join");
  snapshot.specification = { ...snapshot.specification, status: "approved", revision: 8 };
  mounted.rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} {...props} />);
  expect(within(disclosure).getByText("Part of reviewed definition 8.")).not.toBeNull();
  expect(screen.getAllByText("Coordinators invite members.")).toHaveLength(1);
  snapshot.specification = { ...snapshot.specification, status: "outdated" };
  mounted.rerender(<ProjectUnderstanding view={view({ context: { ...snapshot } })} {...props} canEdit={false} />);
  expect(within(disclosure).queryByText("Part of reviewed definition 8.")).toBeNull();
  await userEvent.click(within(disclosure).getByRole("button", { name: "Edit answer in definition" }));
  expect(edit).toHaveBeenCalledOnce();
});

it("does not offer a provider detour or editable answer to a viewer of manual questions", async () => {
  const snapshot = context(), edit = vi.fn(), ask = vi.fn();
  snapshot.specification.questions = [{ id: "join", question: "How do members join?", answer: "" }];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={ask} onReply={vi.fn()} onNavigate={vi.fn()} onEdit={edit} canEdit={false} />);
  expect(screen.queryByRole("button", { name: "Work through the decisions" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Answer in definition" }));
  expect(edit).not.toHaveBeenCalled(); expect(ask).not.toHaveBeenCalled();
});

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
  expect(within(build).getByText(/Saved scope: reviewed definition 2 and design 3/)).not.toBeNull();
  expect(screen.getByText(/Draft definition/)).not.toBeNull();
  expect(within(build).getByText(/does not prove the current application is ready/)).not.toBeNull();
});

it("makes amendment and clarification paths explicit without starting an operation", async () => {
  const ask = vi.fn(), navigate = vi.fn();
  render(<ProjectUnderstanding view={view({ blockingQuestions: [{ turnId: "q", question: "May guests see the customer list?", rationale: "This changes the privacy boundary.", options: [] }] })} onAsk={ask} onReply={vi.fn()} onNavigate={navigate} />);
  expect(screen.getByRole("region", { name: "Decisions to resolve" })).not.toBeNull();
  expect(screen.getByText("This changes the privacy boundary.")).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  expect(ask).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Edit outcomes" })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Discuss a change with Luci" }));
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
  expect(within(ongoing).getByText(/Saved scope: reviewed definition 2 and design 3/)).not.toBeNull();
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

it("keeps one cancellation notice beside the exact scope and reveals technical reasons only on request", async () => {
  const snapshot = context();
  const reason = "Future dispatch is cancelled. Use the child's explicit stop control.";
  const activityReason = "A coding child was last recorded as running. Future dispatch is cancelled.";
  snapshot.activeRuns = [{ id: "active", state: "cancelled", valid: false, reason, activityReason, activity: "running", dispatchCancelled: true,
    tasks: [{ id: "task", title: "Build private guest checkout", status: "selected" }], requirements: [{ id: "outcome", title: "Guest privacy", acceptance: "Only the guest sees their booking" }],
    specificationRevision: 2, designRevision: 3, operations: [{ id: "op", childId: "session", status: "running", updatedAt: "2026-10-06T14:00:00Z" }] }];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  const details = within(build).getByText("Recorded coding activity").closest("details")!;
  expect(details.open).toBe(false);
  expect(within(details).getByText(reason)).not.toBeNull();
  expect(within(details).getByText(activityReason)).not.toBeNull();
  expect(within(build).getByText("Build private guest checkout").closest("details")).toBeNull();
  expect(within(build).getByText(/Saved scope: reviewed definition 2 and design 3/).closest("details")).toBeNull();
  expect(within(build).getAllByText("Further work cancelled.")).toHaveLength(1);
  expect(within(build).getByText("Recorded status. Open the run to reconcile its current outcome.")).not.toBeNull();
  expect(within(build).queryByText(/does not prove the current application is ready/)).toBeNull();
  await userEvent.click(within(build).getByText("Recorded coding activity"));
  expect(details.open).toBe(true);
  expect(within(details).getByText("Session session")).not.toBeNull();
});

it("keeps a blocked run's actionable reason and verification warning visible when no coding work is pending", () => {
  const snapshot = context();
  snapshot.currentRun = { id: "blocked", state: "blocked", valid: false, reason: "The selected provider was disabled. Choose an enabled connection and prepare a new run.", activity: "none", operations: [] };
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText(snapshot.currentRun.reason).closest("details")).toBeNull();
  expect(within(build).getByText(/does not prove the current application is ready/)).not.toBeNull();
});

it("keeps older recorded review obligations visible beside a newer prepared build without claiming activity", async () => {
  const snapshot = context(), inspect = vi.fn();
  snapshot.currentRun = { id: "newer-ready", state: "ready", valid: false, reason: "Prepared", activity: "none" };
  snapshot.activeRuns = [];
  snapshot.needsAttention = [
    { id: "older-unintegrated", state: "awaiting-checks", kind: "task-review", contextStatus: "current" },
    { id: "older-verification", state: "awaiting-checks", kind: "verification", contextStatus: "changed" },
    { id: "older-release", state: "awaiting-release", kind: "release-review", contextStatus: "unavailable" },
    { id: "older-blocked", state: "blocked", kind: "blocked", contextStatus: "current" },
  ];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} onInspect={inspect} canEdit={false} />);
  const review = screen.getByRole("region", { name: "Recorded review work" });
  expect(within(review).getByText(/4 builds need review/)).toBeTruthy();
  expect(screen.queryByRole("region", { name: "Active and unconfirmed work" })).toBeNull();
  expect(screen.getByText("Build prepared")).toBeTruthy();
  expect(within(review).getAllByRole("button", { name: /^Inspect / })).toHaveLength(3);
  await userEvent.click(within(review).getByRole("button", { name: "Inspect older-unintegrated: Task changes to review" }));
  expect(inspect).toHaveBeenCalledExactlyOnceWith("older-unintegrated");
  await userEvent.click(within(review).getByRole("button", { name: "Show more review work (1 remaining)" }));
  await userEvent.click(within(review).getByRole("button", { name: "Inspect older-blocked: Blocked work to inspect" }));
  expect(inspect).toHaveBeenLastCalledWith("older-blocked");
});

it("requires task review before advertising a current build as ready for independent checks", () => {
  const snapshot = context();
  snapshot.currentRun = { id: "needs-integration", state: "awaiting-checks", valid: false, reason: "Recorded task exit", activity: "none" };
  snapshot.needsAttention = [{ id: "needs-integration", state: "awaiting-checks", kind: "task-review", contextStatus: "current" }];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  expect(screen.getByText("Task changes await review")).toBeTruthy();
  expect(screen.queryByText("Ready for independent checks")).toBeNull();
  expect(screen.getByText(/Inspect retained task changes before integration and independent checks/)).toBeTruthy();
});

it("keeps cancelled dispatch distinct from retained task changes awaiting review", () => {
  const snapshot = context();
  snapshot.currentRun = { id: "cancelled-parent", state: "cancelled", valid: false, reason: "Future dispatch cancelled", activity: "none", dispatchCancelled: true };
  snapshot.needsAttention = [{ id: "cancelled-parent", state: "cancelled", kind: "task-review", contextStatus: "current" }];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Task changes await review")).toBeTruthy();
  expect(within(build).getByText("Further dispatch is cancelled. Retained task changes still need review.")).toBeTruthy();
  expect(screen.queryByRole("region", { name: "Active and unconfirmed work" })).toBeNull();
});

it("never repeats the acceptance invitation from an accepted run's historical valid observation", () => {
  const snapshot = context();
  const staleInvitation = "Checks and the freshly observed release match. Review the application and accept the selected outcome.";
  snapshot.currentRun = { id: "accepted", state: "complete", valid: true, reason: staleInvitation, outcome: "preview" };
  render(<ProjectUnderstanding view={view({ context: snapshot, nextAction: { id: "maintain", runId: "accepted", label: "Inspect releases and health", reason: "Historical acceptance does not guarantee uptime.", lens: "deploy" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Run accepted")).not.toBeNull();
  expect(within(build).queryByText(staleInvitation)).toBeNull();
  expect(within(build).getByText("This outcome was accepted. Historical acceptance does not guarantee current release health.")).not.toBeNull();
  expect(screen.getByRole("button", { name: "Inspect releases and health" })).not.toBeNull();
  expect(build.textContent).not.toMatch(/is live|accept the selected outcome/i);
});

it.each([
  "The accepted release no longer passes its health check. Inspect the existing release.",
  "The project definition changed. Review the current definition before starting new work.",
])("retains accepted history and the actionable current invalidity: %s", async reason => {
  const snapshot = context(), inspect = vi.fn();
  snapshot.currentRun = { id: "accepted-invalid", state: "complete", valid: false, reason, outcome: "preview" };
  render(<ProjectUnderstanding view={view({ context: snapshot, nextAction: { id: "inspect-run", runId: "accepted-invalid", label: "Review current run evidence", reason, lens: "develop" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} onInspect={inspect} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Run accepted")).not.toBeNull();
  expect(within(build).getByText(reason).closest("details")).toBeNull();
  expect(within(build).getByText(/recorded progress does not prove the current application is ready/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Review current run evidence" }));
  expect(inspect).toHaveBeenCalledExactlyOnceWith("accepted-invalid");
});


it.each(["awaiting-checks", "awaiting-release"])("labels changed context separately from the recorded %s state", state => {
  const snapshot = context();
  // Wire data is deliberately assigned via a variable so the regression also
  // runs against the earlier client interface which lacked this extension.
  const recorded = { id: "original-build", state, valid: true, reason: "Recorded progress for plan A", activity: "none" as const,
    contextFreshness: { status: "changed" as const, reason: "The saved implementation plan changed. Review the current plan." },
    tasks: [{ id: "original", title: "Implement saving", status: "selected" }] };
  snapshot.currentRun = recorded;
  snapshot.plan.tasks = [{ id: "original", title: "Save with a visible audit trail", status: "todo" }];
  render(<ProjectUnderstanding view={view({ context: snapshot, nextAction: { id: "plan", label: "Review current plan", reason: recorded.contextFreshness.reason, lens: "develop" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Context changed since this build")).not.toBeNull();
  expect(within(build).queryByText(/^Ready for (release|independent checks)$/)).toBeNull();
  expect(within(build).getByText("Implement saving")).not.toBeNull();
  expect(within(build).queryByText("Save with a visible audit trail")).toBeNull();
  expect(within(build).getByText(recorded.contextFreshness.reason)).not.toBeNull();
  expect(within(build).getByText("Last recorded: Recorded progress for plan A")).not.toBeNull();
});

it.each(["preview", "publish"] as const)("opens the exact verified %s build from its deployment review step", async outcome => {
  const snapshot = context(), inspect = vi.fn(), navigate = vi.fn();
  snapshot.currentRun = { id: "checked-build", state: "awaiting-release", valid: true, reason: "Recorded checks passed", outcome };
  render(<ProjectUnderstanding view={view({ context: snapshot, nextAction: { id: "deploy", runId: "checked-build", label: "Review verified build", reason: "Review this build and its checks before preparing a release.", lens: "deploy" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={navigate} onInspect={inspect} canEdit={false} />);
  await userEvent.click(screen.getByRole("button", { name: "Review verified build" }));
  expect(inspect).toHaveBeenCalledExactlyOnceWith("checked-build");
  expect(navigate).not.toHaveBeenCalled();
});


it.each(["complete", "cancelled", "coding"])("preserves recorded %s history alongside changed current inputs", state => {
  const snapshot = context();
  const run: OrchestratorRunSummary = { id: "historical-build", state, valid: true, reason: "Saved observation", activity: state === "coding" ? "running" : "none", contextFreshness: { status: "changed", reason: "Review the amended task plan before new work." }, tasks: [{ id: "task-a", title: "Original task A", status: "selected" }] };
  snapshot.currentRun = run;
  snapshot.activeRuns = state === "coding" ? [run] : [];
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText(state === "complete" ? "Run accepted" : state === "cancelled" ? "Further work cancelled" : "Last recorded: Running")).not.toBeNull();
  expect(within(build).getByText("Context changed since this build.")).not.toBeNull();
  expect(build.textContent).toContain("Review the amended task plan before new work.");
  expect(within(build).getByText("Original task A")).not.toBeNull();
  if (state === "complete") expect(within(build).getByText(/Historical acceptance does not guarantee current release health/)).not.toBeNull();
  if (state === "coding") expect(within(build).getByText(/Open the run to reconcile its current outcome/)).not.toBeNull();
});

it("does not claim readiness when the current input comparison is unavailable", () => {
  const snapshot = context();
  snapshot.currentRun = { id: "unavailable-build", state: "awaiting-release", valid: true, reason: "Checks were recorded", contextFreshness: { status: "unavailable", reason: "Check the project's context storage, then refresh history." } };
  render(<ProjectUnderstanding view={view({ context: snapshot })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={vi.fn()} />);
  const build = screen.getByRole("region", { name: "In this build" });
  expect(within(build).getByText("Current context unavailable")).not.toBeNull();
  expect(within(build).queryByText("Ready for release")).toBeNull();
  expect(within(build).getByText("Check the project's context storage, then refresh history.")).not.toBeNull();
});

it("keeps accepted-release health navigation distinct from an exact build review", async () => {
  const navigate = vi.fn(), inspect = vi.fn();
  render(<ProjectUnderstanding view={view({ nextAction: { id: "maintain", runId: "accepted", label: "Inspect releases and health", reason: "Inspect current health.", lens: "deploy" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={navigate} onInspect={inspect} />);
  await userEvent.click(screen.getByRole("button", { name: "Inspect releases and health" }));
  expect(navigate).toHaveBeenCalledExactlyOnceWith("deploy");
  expect(inspect).not.toHaveBeenCalled();
});

it("falls back to build review rather than generic deployment when no exact-run handler is supplied", async () => {
  const navigate = vi.fn();
  render(<ProjectUnderstanding view={view({ nextAction: { id: "deploy", runId: "checked-build", label: "Review verified build", reason: "Inspect the recorded checks first.", lens: "deploy" } })} onAsk={vi.fn()} onReply={vi.fn()} onNavigate={navigate} />);
  await userEvent.click(screen.getByRole("button", { name: "Review verified build" }));
  expect(navigate).toHaveBeenCalledExactlyOnceWith("develop");
});
