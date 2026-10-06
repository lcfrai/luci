import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { contextHistoryApi, type ContextChangeEvent, type ContextHistoryResponse } from "../../api/contextHistory";
import { ContextHistory } from "./ContextHistory";

const identity = vi.hoisted(() => ({ actor: "owner", connection: "", root: "/projects/owner" }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => identity.actor ? { profile: { user: { id: identity.actor } } } : null }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({
  workspace: { mode: identity.connection ? "cloud" : "local", connectionId: identity.connection, host: identity.connection },
  status: { access: { user: { username: identity.actor } }, config: { projectsDir: identity.root } },
}) }));

const event = (patch: Partial<ContextChangeEvent> = {}): ContextChangeEvent => ({
  id: `context-${"a".repeat(24)}`, projectId: "p-1", kind: "specification", action: "save", revision: 2,
  digest: "a".repeat(64), at: "2026-10-06T13:15:00Z", actor: { id: "person-alice", displayName: "Alice" }, status: "applied", ...patch,
});

beforeEach(() => {
  vi.restoreAllMocks(); identity.actor = "owner"; identity.connection = ""; identity.root = "/projects/owner";
  vi.spyOn(contextHistoryApi, "list").mockResolvedValue({ events: [], pruned: false });
});

it("shows the human who saved and the human who reviewed an exact revision and refreshes by reading only", async () => {
  vi.mocked(contextHistoryApi.list).mockResolvedValue({ events: [
    event({ id: `context-${"b".repeat(24)}`, action: "approve", actor: { id: "person-bob", displayName: "Bob" } }), event(),
  ], pruned: false });
  render(<ContextHistory projectId="p-1" />);
  expect((await screen.findByText("Bob")).parentElement?.textContent).toBe("Bob approved the definition");
  expect(screen.getByText("Alice").parentElement?.textContent).toBe("Alice saved the definition");
  expect(screen.getAllByText(/Revision 2/)).toHaveLength(2);
  expect(screen.getByText(/Earlier activity does not grant approval/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh activity" }));
  await waitFor(() => expect(contextHistoryApi.list).toHaveBeenCalledTimes(2));
  expect(vi.mocked(contextHistoryApi.list).mock.calls).toEqual([["p-1"], ["p-1"]]);
});

it("keeps interrupted review requests explicitly unconfirmed", async () => {
  vi.mocked(contextHistoryApi.list).mockResolvedValue({ events: [event({ action: "approve", status: "unconfirmed", kind: "design" })], pruned: false });
  render(<ContextHistory projectId="p-1" />);
  expect((await screen.findByText("Alice")).parentElement?.textContent).toBe("Alice requested approval of the design handoff");
  expect(screen.getByText(/Outcome not confirmed.*Reload the document/)).not.toBeNull();
  expect(screen.queryByText("approved the design handoff", { exact: false })).toBeNull();
});

it("discloses unknown imported authorship and omitted older history without fabricating people", async () => {
  vi.mocked(contextHistoryApi.list).mockResolvedValue({ events: [], pruned: true });
  render(<ContextHistory projectId="p-1" />);
  await screen.findByText(/Imported or earlier revisions have unknown authorship/);
  expect(screen.getByText(/Older activity is no longer retained/)).not.toBeNull();
  expect(screen.queryByRole("list")).toBeNull();
});

it("renders bounded recent activity and lets people read older retained changes", async () => {
  vi.mocked(contextHistoryApi.list).mockResolvedValue({ events: Array.from({ length: 13 }, (_, index) => event({
    id: `context-${index.toString(16).padStart(24, "0")}`, revision: 13 - index,
  })), pruned: false });
  render(<ContextHistory projectId="p-1" />);
  await screen.findByRole("list");
  expect(screen.getAllByRole("listitem")).toHaveLength(12);
  await userEvent.click(screen.getByRole("button", { name: "Show older activity" }));
  expect(screen.getAllByRole("listitem")).toHaveLength(13);
  expect(screen.queryByRole("button", { name: "Show older activity" })).toBeNull();
  expect(contextHistoryApi.list).toHaveBeenCalledTimes(1);
});

it("shows a failed read honestly and recovers with an explicit refresh", async () => {
  vi.mocked(contextHistoryApi.list).mockRejectedValueOnce(new Error("Private context attribution could not be verified."));
  render(<ContextHistory projectId="p-1" />);
  expect((await screen.findByRole("alert")).textContent).toContain("could not be verified");
  expect(screen.queryByText(/unknown authorship/)).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh activity" }));
  await screen.findByText(/unknown authorship/);
  expect(screen.queryByRole("alert")).toBeNull();
});

it.each(["actor", "connection", "root", "project"] as const)("discards late attribution after the selected %s changes", async change => {
  let complete!: (value: ContextHistoryResponse) => void;
  vi.mocked(contextHistoryApi.list).mockReturnValueOnce(new Promise(resolve => { complete = resolve; }));
  const view = render(<ContextHistory projectId="p-1" />);
  if (change !== "project") identity[change] = "other";
  view.rerender(<ContextHistory projectId={change === "project" ? "p-2" : "p-1"} />);
  await screen.findByText(/unknown authorship/);
  await act(async () => complete({ events: [event()], pruned: false }));
  expect(screen.queryByText("Alice")).toBeNull();
  expect(contextHistoryApi.list).toHaveBeenCalledTimes(2);
});

it("clears visible people immediately on sign out and never reads an anonymous history", async () => {
  vi.mocked(contextHistoryApi.list).mockResolvedValue({ events: [event()], pruned: false });
  const view = render(<ContextHistory projectId="p-1" />);
  await screen.findByText("Alice");
  identity.actor = "";
  view.rerender(<ContextHistory projectId="p-1" />);
  expect(screen.queryByText("Alice")).toBeNull();
  expect(screen.getByText(/account and workspace are connected/)).not.toBeNull();
  expect(contextHistoryApi.list).toHaveBeenCalledTimes(1);
});
