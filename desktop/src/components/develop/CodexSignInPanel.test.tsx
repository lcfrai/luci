import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CodexSignInPanel } from "./CodexSignInPanel";
import { CODEX_DEVICE_URL } from "../../api/codexDeviceSignIn";
import type { ProductClient } from "../../api/product";
import { aSession } from "../../test/fixtures";
import { resetCockpit, useCockpit } from "../../store/store";
import { ApiError } from "../../api/client";

const mocks = vi.hoisted(() => ({ kill: vi.fn(), challenge: { code: "AB12-C345D", url: "https://auth.openai.com/codex/device", observedAt: Date.now() }, phase: "attached" }));
vi.mock("../../api", () => ({ api: { killSession: mocks.kill } }));
vi.mock("../../api/attachRegistry", () => ({ useCodexDeviceChallenge: () => mocks.challenge, useAttachState: () => ({ phase: mocks.phase }) }));
beforeEach(() => { resetCockpit(); mocks.phase = "attached"; mocks.challenge.observedAt = Date.now(); });
function setup(exited = false, canControl = true) {
  const session = aSession({ id: "auth", harness: "codex", authOnly: true, authMode: "chatgpt", worktreeId: "setup-codex", projectId: "", branchId: "", status: exited ? "exited" : "running" });
  const stopped = { ...session, status: "exited" as const };
  const fresh = { ...session, id: "fresh-auth", status: "starting" as const };
  mocks.kill.mockResolvedValue(stopped);
  const save = vi.fn().mockResolvedValue({ configured: true });
  const start = vi.fn().mockResolvedValue(fresh);
  useCockpit.setState({ sessions: { auth: session } });
  render(<CodexSignInPanel session={session} canControl={canControl} client={{ connectCodexAccount: save, startCodexSignIn: start } as unknown as ProductClient} />);
  return { save, start, session, fresh };
}
it("copies outside the terminal without stopping or authenticating anything", async () => {
  const user = userEvent.setup(); setup();
  const copy = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
  await user.click(screen.getByRole("button", { name: "Copy code" }));
  expect(copy).toHaveBeenCalledExactlyOnceWith("AB12-C345D");
  expect(mocks.kill).not.toHaveBeenCalled();
  expect(await screen.findByRole("status")).toHaveProperty("textContent", "Code copied. Paste it on OpenAI’s sign-in page.");
});
it("opens only the fixed official URL and waits for explicit save", async () => {
  const user = userEvent.setup(); const { save, start } = setup();
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  await user.click(screen.getByRole("button", { name: "Open OpenAI sign-in" }));
  expect(open).toHaveBeenCalledWith(CODEX_DEVICE_URL, "_blank", "noopener,noreferrer");
  expect(save).not.toHaveBeenCalled(); expect(start).not.toHaveBeenCalled(); expect(mocks.kill).not.toHaveBeenCalled();
});
it("normal-stops then validates sign-in only after explicit Save", async () => {
  const user = userEvent.setup(); const { save } = setup();
  expect(save).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Save Codex sign-in" }));
  await screen.findByText(/Codex sign-in saved for this Luci workspace/);
  expect(mocks.kill).toHaveBeenCalledExactlyOnceWith("auth"); expect(save).toHaveBeenCalledExactlyOnceWith("auth");
  expect(mocks.kill.mock.invocationCallOrder[0]).toBeLessThan(save.mock.invocationCallOrder[0]);
});
it("retains ended sign-in guidance and starts a fresh utility rather than generic Restart", async () => {
  const user = userEvent.setup(); const { start, save, fresh } = setup(true);
  expect(screen.queryByRole("button", { name: "Copy code" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Restart" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Start new device sign-in" }));
  await waitFor(() => expect(useCockpit.getState().sessions[fresh.id]).toEqual(fresh));
  expect(start).toHaveBeenCalledOnce(); expect(save).not.toHaveBeenCalled(); expect(mocks.kill).not.toHaveBeenCalled();
});
it("does not allocate a fresh utility if normal retirement fails", async () => {
  const user = userEvent.setup(); mocks.phase = "ended"; const { start } = setup();
  mocks.kill.mockRejectedValue(new Error("Docker retirement is uncertain. Retry stop."));
  await user.click(screen.getByRole("button", { name: "Start new device sign-in" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "Docker retirement is uncertain. Retry stop.");
  expect(start).not.toHaveBeenCalled();
});
it("requires a terminal stop witness before allocating another utility", async () => {
  const user = userEvent.setup(); mocks.phase = "ended"; const { start, session } = setup();
  mocks.kill.mockResolvedValue(session);
  await user.click(screen.getByRole("button", { name: "Start new device sign-in" }));
  await screen.findByText("Sign-in has not stopped safely. Retry stop before continuing.");
  expect(start).not.toHaveBeenCalled();
});
it("preserves adoption failure inline and leaves explicit fresh sign-in available", async () => {
  const user = userEvent.setup(); const { save, start } = setup(true);
  save.mockRejectedValue(new Error("No completed ChatGPT credential. Start a new device sign-in."));
  await user.click(screen.getByRole("button", { name: "Save Codex sign-in" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "No completed ChatGPT credential. Start a new device sign-in.");
  expect(screen.getByRole("button", { name: "Start new device sign-in" })).toBeDefined(); expect(start).not.toHaveBeenCalled();
});
it("withholds controls when account authority is unavailable", () => {
  setup(true, false);
  expect((screen.getByRole("button", { name: "Save Codex sign-in" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Start new device sign-in" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("button", { name: "Copy code" })).toBeNull();
});

it("keeps a timed-out Save unconfirmed without allocating another sign-in", async () => {
  const user = userEvent.setup(); const { save, start } = setup(true);
  save.mockRejectedValue(new ApiError("timeout", "request timed out", 0));
  await user.click(screen.getByRole("button", { name: "Save Codex sign-in" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Saving this sign-in is unconfirmed");
  expect(screen.queryByText(/Codex sign-in saved for this Luci workspace/)).toBeNull();
  expect(start).not.toHaveBeenCalled();
  expect(useCockpit.getState().sessions.auth).toBeDefined();
});
