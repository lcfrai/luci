import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { orchestratorApi } from "../../api/orchestrator";
import { ConversationStorage } from "./ConversationStorage";
import { view } from "./fixtures";
const onView = vi.fn(), onBusy = vi.fn();
const props = () => ({ projectId: "p-1", view: view(), canEdit: true, busy: false, onView, onBusy });
beforeEach(() => {
  vi.restoreAllMocks(); onView.mockReset(); onBusy.mockReset();
  vi.spyOn(orchestratorApi, "export").mockResolvedValue({ version: 1, projectId: "p-1", storageDigest: "private-storage-digest", ledger: { turns: ["private contents"] } });
  vi.spyOn(orchestratorApi, "get").mockResolvedValue(view());
  vi.spyOn(orchestratorApi, "rotate").mockResolvedValue(view({ revision: 4, storageDigest: "rotated", archivePresent: true }));
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:private-export"), revokeObjectURL: vi.fn() }));
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
});
afterEach(() => vi.unstubAllGlobals());

it("requires a real private-history download and explicit acknowledgement before exact CAS cleanup", async () => {
  render(<ConversationStorage {...props()} />);
  await userEvent.click(screen.getByText("Conversation history and storage"));
  const rotate = screen.getByRole("button", { name: "Free conversation space" }) as HTMLButtonElement;
  expect(rotate.disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Download private conversation" }));
  await screen.findByText(/Download started/);
  expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
  expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledTimes(1);
  expect(rotate.disabled).toBe(true);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(rotate);
  expect(orchestratorApi.rotate).toHaveBeenCalledWith("p-1", { expectedRevision: 3, expectedStorageDigest: "private-storage-digest", acknowledgeExportedHistory: true });
  expect(onView).toHaveBeenLastCalledWith(expect.objectContaining({ revision: 4 }), true);
  expect(rotate.disabled).toBe(true);
});

it("rejects a download whose snapshot changed in flight, and invalidates an acknowledgement on later shared edits", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValueOnce(view({ storageDigest: "new-activity" }));
  const mounted = render(<ConversationStorage {...props()} />);
  await userEvent.click(screen.getByText("Conversation history and storage"));
  await userEvent.click(screen.getByRole("button", { name: "Download private conversation" }));
  await screen.findByText(/conversation changed during download/);
  expect((screen.getByRole("checkbox") as HTMLInputElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Download private conversation" }));
  await screen.findByText(/Download started/);
  await userEvent.click(screen.getByRole("checkbox"));
  mounted.rerender(<ConversationStorage {...props()} view={view({ storageDigest: "changed-again" })} />);
  expect((screen.getByRole("button", { name: "Free conversation space" }) as HTMLButtonElement).disabled).toBe(true);
  expect(orchestratorApi.rotate).not.toHaveBeenCalled();
});

it("preserves a failed cleanup as an explicit error and requires a fresh download", async () => {
  vi.mocked(orchestratorApi.rotate).mockRejectedValue(new Error("A colleague changed the conversation."));
  render(<ConversationStorage {...props()} />);
  await userEvent.click(screen.getByText("Conversation history and storage"));
  await userEvent.click(screen.getByRole("button", { name: "Download private conversation" }));
  await screen.findByText(/Download started/);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getByRole("button", { name: "Free conversation space" }));
  await screen.findByRole("alert");
  expect((screen.getByRole("checkbox") as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole("button", { name: "Free conversation space" }) as HTMLButtonElement).disabled).toBe(true);
});

it("never starts a private download after the project panel unmounts", async () => {
  let finish!: (value: Awaited<ReturnType<typeof orchestratorApi.export>>) => void;
  vi.mocked(orchestratorApi.export).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const mounted = render(<ConversationStorage {...props()} />);
  await userEvent.click(screen.getByText("Conversation history and storage"));
  await userEvent.click(screen.getByRole("button", { name: "Download private conversation" }));
  mounted.unmount();
  await act(async () => finish({ version: 1, projectId: "p-1", storageDigest: "private-storage-digest", ledger: {} }));
  await waitFor(() => expect(HTMLAnchorElement.prototype.click).not.toHaveBeenCalled());
  expect(onView).not.toHaveBeenCalled();
});
