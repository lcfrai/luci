import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ProjectUnderstanding } from "./ProjectUnderstanding";
import { context, view } from "./fixtures";

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
