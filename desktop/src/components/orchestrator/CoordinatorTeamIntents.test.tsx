import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { coordinatorIntentsApi, type CoordinatorIntent, type CoordinatorIntentsView } from "../../api/coordinatorIntents";
import { ApiError } from "../../api/client";
import { ProductApiError } from "../../api/product";
import { CoordinatorTeamIntents, type CoordinatorTeamIntentsProps } from "./CoordinatorTeamIntents";

const hash = "a".repeat(64), otherHash = "b".repeat(64);
function intent(patch: Partial<CoordinatorIntent> = {}): CoordinatorIntent { return { id: "intent-a", turnId: "turn-a", actor: { id: "alice", displayName: "Alice" }, luciId: `luci-${hash}`, baseContextDigest: hash, proposalDigest: hash, target: "design-plan", requestedChange: "Use blue buttons", changedPaths: ["palette"], requirementIds: ["AC-colour"], taskIds: ["task-buttons"], affectsAllTasks: false, dependsOn: [], status: "pending", createdAt: "2026-10-12T00:00:00Z", ...patch }; }
function view(patch: Partial<CoordinatorIntentsView> = {}): CoordinatorIntentsView { return { projectId: "p-1", revision: 2, contextDigest: hash, intents: [intent(), intent({ id: "intent-b", turnId: "turn-b", actor: { id: "bob", displayName: "Bob" }, requestedChange: "Use red buttons" })], events: [], conflicts: [{ intentId: "intent-a", otherIntentId: "intent-b", paths: ["palette"], reason: "opposing-intent" }], allowedActions: ["propose-own-turn", "supersede-own-intent"], ...patch }; }
const props = (patch: Partial<CoordinatorTeamIntentsProps> = {}): CoordinatorTeamIntentsProps => ({ projectId: "p-1", scope: "account-workspace-project", actorId: "alice", contextDigest: hash, canRead: true, canEdit: true, ...patch });
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (value: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
function saved(reason = "Accessible red has been agreed.") { return intent({ status: "superseded", supersededBy: "intent-b", resolutionReason: reason }); }
function resolvedView() { return view({ revision: 3, intents: [saved(), view().intents[1]], conflicts: [] }); }
async function openDecision() {
  await screen.findByText("2 pending · 1 coordination issue");
  await userEvent.click(screen.getByText("Requests and context history (2)"));
  await userEvent.click(screen.getByRole("button", { name: "Resolve my request" }));
  await userEvent.selectOptions(screen.getByLabelText("Replacement request"), "intent-b");
  fireEvent.change(screen.getByLabelText("Why this resolves your request"), { target: { value: "Accessible red has been agreed." } });
  await userEvent.click(screen.getByRole("checkbox", { name: /Supersede only my request/ }));
}
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear();
  vi.spyOn(coordinatorIntentsApi, "get").mockResolvedValue(view());
  vi.spyOn(coordinatorIntentsApi, "resolve").mockResolvedValue(saved());
  vi.spyOn(coordinatorIntentsApi, "record").mockResolvedValue(intent());
});

it("shows exact shared authors and conflicting scope without claiming consensus or mutating on reads", async () => {
  render(<CoordinatorTeamIntents {...props()} />);
  await screen.findByText("2 pending · 1 coordination issue");
  await userEvent.click(screen.getByText("Requests and context history (2)"));
  expect(screen.getByText("Alice’s Luci · Design")).not.toBeNull(); expect(screen.getByText("Bob’s Luci · Design")).not.toBeNull();
  expect(screen.getAllByText(/Decision needed: these requests/)).toHaveLength(2);
  expect(screen.getAllByRole("button", { name: "Resolve my request" })).toHaveLength(1);
  expect(screen.getByText(/A saved context revision is separate/)).not.toBeNull();
  expect(coordinatorIntentsApi.record).not.toHaveBeenCalled(); expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
});

it("observes a teammate's new intent on return even when canonical context did not change", async () => {
  render(<CoordinatorTeamIntents {...props()} />);
  await screen.findByText("2 pending · 1 coordination issue");
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(view({ revision: 3, intents: [...view().intents,
    intent({ id: "intent-c", turnId: "turn-c", actor: { id: "charlie", displayName: "Charlie" }, requestedChange: "Add keyboard access", changedPaths: ["accessibility"] })],
  }));
  fireEvent.focus(window);
  await screen.findByText("3 pending · 1 coordination issue");
  expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
  expect(coordinatorIntentsApi.record).not.toHaveBeenCalled();
});
it("requires fresh consent and exact replacement/reason for only the person's own intent", async () => {
  render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  fireEvent.change(screen.getByLabelText("Why this resolves your request"), { target: { value: "Updated reason" } });
  expect((screen.getByRole("button", { name: "Record my decision" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText("Why this resolves your request"), { target: { value: "Accessible red has been agreed." } });
  await userEvent.click(screen.getByRole("checkbox", { name: /Supersede only my request/ }));
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(resolvedView());
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" }));
  await screen.findByText(/Your request was superseded/);
  expect(coordinatorIntentsApi.resolve).toHaveBeenCalledExactlyOnceWith("p-1", "alice", "intent-a", { expectedRevision: 2, expectedContextDigest: hash, supersededBy: "intent-b", reason: "Accessible red has been agreed." });
  expect(screen.queryByRole("form", { name: "Resolve my team request" })).toBeNull();
  expect(localStorage.length).toBe(0);
});
it("retains a timed-out decision across remount and only exact readback releases its barrier", async () => {
  vi.mocked(coordinatorIntentsApi.resolve).mockRejectedValue(new ApiError("timeout", "Response lost", 0));
  const mounted = render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" })); await screen.findByText(/Response lost/);
  const raw = localStorage.getItem(localStorage.key(0)!); mounted.unmount();
  render(<CoordinatorTeamIntents {...props()} />); await screen.findByText(/A team decision is awaiting/);
  expect(localStorage.getItem(localStorage.key(0)!)).toBe(raw);
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" }));
  await waitFor(() => expect((screen.getByRole("button", { name: "Refresh coordination" }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.getByText(/A team decision is awaiting/)).not.toBeNull(); expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(resolvedView());
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" })); await screen.findByText(/Your recorded decision is confirmed/);
  expect(localStorage.length).toBe(0); expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
});
it.each(["success", "timeout"])('keeps the mutation barrier through revoke/restore even when the old %s arrives after a fresh empty read', async outcome => {
  const pending = deferred<CoordinatorIntent>(); vi.mocked(coordinatorIntentsApi.resolve).mockReturnValue(pending.promise);
  const mounted = render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" }));
  mounted.rerender(<CoordinatorTeamIntents {...props({ canEdit: false })} />); await act(async () => {});
  mounted.rerender(<CoordinatorTeamIntents {...props({ canEdit: true })} />); await act(async () => {});
  await act(async () => { if (outcome === "success") pending.resolve(saved()); else pending.reject(new ApiError("timeout", "Late timeout", 0)); });
  expect(screen.getByText(/A team decision is awaiting/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Record my decision" }) as HTMLButtonElement).disabled).toBe(true);
  expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
});
it("hides revoked private context and fences late reads across project and actor switches", async () => {
  const read = deferred<CoordinatorIntentsView>(); vi.mocked(coordinatorIntentsApi.get).mockReturnValueOnce(read.promise);
  const mounted = render(<CoordinatorTeamIntents {...props()} />);
  mounted.rerender(<CoordinatorTeamIntents {...props({ canRead: false })} />);
  await act(async () => read.resolve(view())); expect(screen.queryByRole("region", { name: "Team coordination" })).toBeNull();
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(view({ projectId: "p-2", intents: [], conflicts: [] }));
  mounted.rerender(<CoordinatorTeamIntents {...props({ projectId: "p-2", actorId: "bob", scope: "other-project" })} />);
  await screen.findByText(/No shared requests recorded/); expect(screen.queryByText("Use blue buttons")).toBeNull();
});
it("retains drafts per person/project and never carries consent into reopening", async () => {
  const mounted = render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  mounted.unmount(); render(<CoordinatorTeamIntents {...props()} />); await screen.findByRole("form", { name: "Resolve my team request" });
  expect((screen.getByLabelText("Why this resolves your request") as HTMLTextAreaElement).value).toBe("Accessible red has been agreed.");
  expect((screen.getByRole("checkbox", { name: /Supersede only my request/ }) as HTMLInputElement).checked).toBe(false);
  expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
});
it("refuses decisions based on stale conversation context or an unsaved parent draft", async () => {
  const refresh = vi.fn(); render(<CoordinatorTeamIntents {...props({ contextDigest: otherHash, blocked: "Save your context draft first.", onRefreshContext: refresh })} />);
  await screen.findByText(/Shared context changed/); await userEvent.click(screen.getByText("Requests and context history (2)"));
  expect((screen.getByRole("button", { name: "Resolve my request" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Refresh conversation" })); expect(refresh).toHaveBeenCalledOnce();
  expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
});
it("fails closed if another window replaced the recovery record before dispatch", async () => {
  render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  localStorage.setItem(localStorage.key(0)!, "another-window");
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" }));
  await screen.findByText(/Another window changed/); expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
  expect(localStorage.getItem(localStorage.key(0)!)).toBe("another-window");
});
it("requires a fresh read and renewed consent after an authoritative stale revision refusal", async () => {
  vi.mocked(coordinatorIntentsApi.resolve).mockRejectedValue(new ProductApiError("conflict", "Revision changed", 409, { error: "conflict" }));
  render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" })); await screen.findByText(/Revision changed/);
  expect(screen.queryByText(/A team decision is awaiting/)).toBeNull();
  expect((screen.getByRole("button", { name: "Record my decision" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" })); await act(async () => {});
  expect((screen.getByRole("checkbox", { name: /Supersede only my request/ }) as HTMLInputElement).checked).toBe(false);
  expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
});
it("does not mistake a client workspace-change 409 for a confirmed server refusal", async () => {
  vi.mocked(coordinatorIntentsApi.resolve).mockRejectedValue(new ApiError("conflict", "The workspace changed before this response arrived.", 409));
  render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" })); await screen.findByText(/workspace changed before this response/);
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" }));
  await waitFor(() => expect((screen.getByRole("button", { name: "Refresh coordination" }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.getByText(/A team decision is awaiting/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Record my decision" }) as HTMLButtonElement).disabled).toBe(true);
  expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
});
it("shows confirmed context application separately from delivery and sends discussion only through the parent", async () => {
  const discuss = vi.fn(); vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(view({ intents: [intent({ status: "applied", appliedRevision: 5 })], conflicts: [] }));
  render(<CoordinatorTeamIntents {...props({ onDiscuss: discuss })} />); await screen.findByText("0 pending · 0 coordination issues");
  await userEvent.click(screen.getByText("Requests and context history (1)"));
  expect(screen.getByText("Context saved · revision 5")).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Resolve my request" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open this conversation" })); expect(discuss).toHaveBeenCalledExactlyOnceWith("turn-a");
  expect(coordinatorIntentsApi.resolve).not.toHaveBeenCalled();
});
it("does not mark an independent prerequisite blocked by its dependent's waiting state", async () => {
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(view({ conflicts: [{ intentId: "intent-a", otherIntentId: "intent-b", paths: [], reason: "dependency-unresolved" }] }));
  render(<CoordinatorTeamIntents {...props()} />); await screen.findByText("2 pending · 1 coordination issue");
  await userEvent.click(screen.getByText("Requests and context history (2)"));
  expect(screen.getAllByText("Needs coordination")).toHaveLength(1);
  expect(screen.getAllByText("Pending context application")).toHaveLength(1);
});
it("omits different document targets and replacements waiting on the withdrawn intent", async () => {
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(view({ intents: [...view().intents,
    intent({ id: "intent-c", turnId: "turn-c", target: "specification", requestedChange: "Different document" }),
    intent({ id: "intent-d", turnId: "turn-d", dependsOn: ["intent-a"], requestedChange: "Dependent replacement" }),
  ] }));
  render(<CoordinatorTeamIntents {...props()} />); await screen.findByText("4 pending · 1 coordination issue");
  await userEvent.click(screen.getByText("Requests and context history (4)"));
  await userEvent.click(screen.getAllByRole("button", { name: "Resolve my request" })[0]);
  expect(screen.queryByRole("option", { name: /Different document/ })).toBeNull();
  expect(screen.queryByRole("option", { name: /Dependent replacement/ })).toBeNull();
  expect(screen.getByRole("option", { name: /Bob: Use red buttons/ })).not.toBeNull();
});
it.each([401, 403])("hides cached private intents after read denial %s without losing an unknown decision", async status => {
  vi.mocked(coordinatorIntentsApi.resolve).mockRejectedValue(new ApiError("timeout", "Original outcome unknown", 0));
  render(<CoordinatorTeamIntents {...props()} />); await openDecision();
  await userEvent.click(screen.getByRole("button", { name: "Record my decision" })); await screen.findByText(/Original outcome unknown/);
  const raw = localStorage.getItem(localStorage.key(0)!);
  vi.mocked(coordinatorIntentsApi.get).mockRejectedValue(new ProductApiError("forbidden_origin", "Read access refused", status, { error: "forbidden" }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" }));
  await screen.findByRole("alert");
  await waitFor(() => expect((screen.getByRole("button", { name: "Refresh coordination" }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.queryByText("Alice’s Luci · Design")).toBeNull();
  expect(screen.queryByLabelText("Why this resolves your request")).toBeNull();
  expect(localStorage.getItem(localStorage.key(0)!)).toBe(raw);
  vi.mocked(coordinatorIntentsApi.get).mockResolvedValue(resolvedView());
  await userEvent.click(screen.getByRole("button", { name: "Refresh coordination" }));
  await screen.findByText(/Your recorded decision is confirmed/);
  expect(coordinatorIntentsApi.resolve).toHaveBeenCalledTimes(1);
});
