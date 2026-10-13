import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { definitionApi } from "../../api/definition";
import { designPlanApi } from "../../api/designPlan";
import type { OrchestratorView } from "../../api/orchestrator";
import { ApiError } from "../../api/client";
import { CoordinatorContextReview } from "./CoordinatorContextReview";
import { PROJECT_CONTEXT_CHANGED } from "../../api/projectContextEvents";

const hash = "a".repeat(64);
const reviewView = (target: "specification" | "design-plan" = "specification") => ({
  context: { brief: { revision: 4 }, specification: { projectId: "p-1", revision: 2, status: target === "specification" ? "draft" : "approved", digest: "", purpose: "An accessible planner", requirements: [{ id: "R1", title: "Plan", acceptance: "Keyboard users can create a plan" }], questions: [] },
    designPlan: { projectId: "p-1", revision: 3, status: target === "design-plan" ? "draft" : "approved", digest: "", implementationNotes: "Keyboard-first design", screens: [{ id: "S1", name: "Planner", purpose: "Create plans" }] } },
}) as unknown as OrchestratorView;
const props = (target: "specification" | "design-plan" = "specification") => ({ projectId: "p-1", scope: "private-p-1", actorId: "alice", view: reviewView(target), canRead: true, canEdit: true, blocked: "", onReviewed: vi.fn() });

beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); });

it("reviews an exact definition in the conversation and refreshes the affected work context", async () => {
  const changed = vi.fn(); window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
  try {
  const approved = { projectId: "p-1", revision: 3, status: "approved", digest: hash };
  vi.spyOn(definitionApi, "approve").mockResolvedValue(approved as never);
  const input = props(); render(<CoordinatorContextReview {...input} />);
  await userEvent.click(screen.getByText("Saved definition · revision 2"));
  expect(screen.getAllByText(/Keyboard users can create a plan/).length).toBeGreaterThan(0);
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed this saved definition/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve saved definition" }));
  await waitFor(() => expect(input.onReviewed).toHaveBeenCalledTimes(1));
  expect(definitionApi.approve).toHaveBeenCalledExactlyOnceWith("p-1", 2, 4);
  expect(changed).toHaveBeenCalledTimes(1);
  expect((changed.mock.calls[0][0] as CustomEvent).detail).toEqual({ scope: "private-p-1", projectId: "p-1" });
  expect(localStorage.length).toBe(0);
  } finally { window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed); }
});

it("keeps an uncertain design review and confirms it by readback without replay", async () => {
  vi.spyOn(designPlanApi, "approve").mockRejectedValue(new ApiError("timeout", "Approval response was lost", 0));
  vi.spyOn(designPlanApi, "get").mockResolvedValue({ projectId: "p-1", revision: 4, status: "approved", digest: hash } as never);
  const input = props("design-plan"); render(<CoordinatorContextReview {...input} />);
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed this saved design/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve saved design" }));
  await screen.findByText(/An exact review request is retained/);
  expect(localStorage.length).toBe(1);
  await userEvent.click(screen.getByRole("button", { name: "Read saved review outcome" }));
  await waitFor(() => expect(input.onReviewed).toHaveBeenCalledTimes(1));
  expect(designPlanApi.approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  expect(localStorage.length).toBe(0);
});

it("leaves the retained request after losing edit authority before the approval response", async () => {
  let resolve!: (value: unknown) => void;
  vi.spyOn(definitionApi, "approve").mockImplementation(() => new Promise(resolvePromise => { resolve = resolvePromise; }) as never);
  const input = props(); const rendered = render(<CoordinatorContextReview {...input} />);
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed this saved definition/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve saved definition" }));
  rendered.rerender(<CoordinatorContextReview {...input} canEdit={false} />);
  await act(async () => { resolve({ projectId: "p-1", revision: 3, status: "approved", digest: hash }); });
  expect(input.onReviewed).not.toHaveBeenCalled();
  expect(localStorage.length).toBe(1);
});
