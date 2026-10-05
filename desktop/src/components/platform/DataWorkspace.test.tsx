import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DataWorkspace } from "./DataWorkspace";
import { dataApi, type DataStatus } from "../../api/data";
import { platformApi } from "../../api/platform";
vi.mock("../../api/platform", async (importOriginal) => ({ ...await importOriginal<typeof import("../../api/platform")>(), platformApi: { prepareBrowser: vi.fn() } }));
vi.mock("../../api/data", () => ({ dataApi: { status: vi.fn(), action: vi.fn(), users: vi.fn(), createUser: vi.fn(), tables: vi.fn(), rows: vi.fn(), rls: vi.fn(), buckets: vi.fn(), realtime: vi.fn(), functions: vi.fn(), downloadBackup: vi.fn() } }));
const initial: DataStatus = { projectId: "owned", provider: "supabase", version: "self-hosted/v0.8.2", revision: 0, status: "not-installed", message: "Not installed.", services: [], backups: [], capabilities: {}, requirements: { memoryMiB: 4096, cpuMillis: 2000, diskGiB: 40 }, priorStackRetained: false };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(platformApi.prepareBrowser).mockReset(); vi.mocked(dataApi.status).mockResolvedValue(structuredClone(initial)); });

it("requires resource acknowledgement and keeps accepted installation pending until its real result", async () => {
  const accepted: DataStatus = { ...initial, revision: 1, operation: { id: "operation", action: "install", status: "running", stage: "pulling", message: "Downloading reviewed images", createdAt: "now", updatedAt: "now" } };
  vi.mocked(dataApi.action).mockResolvedValue(accepted);
  render(<DataWorkspace projectId="owned" dockerReady />);
  const install = await screen.findByRole("button", { name: "Install data services" });
  expect((install as HTMLButtonElement).disabled).toBe(true);
  expect(dataApi.action).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("I have capacity for these services on the selected Docker host."));
  fireEvent.click(install);
  await waitFor(() => expect(dataApi.action).toHaveBeenCalledWith("owned", "install", { acknowledgeResources: true }));
  expect(await screen.findByText("Downloading reviewed images")).not.toBeNull();
  expect(screen.queryByText(/installation completed/i)).toBeNull();
  expect((screen.getByRole("button", { name: "Install data services" }) as HTMLButtonElement).disabled).toBe(true);
});

it("keeps restore disabled until a specific backup, capacity and replacement confirmation are selected", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "stopped", backups: [{ id: "backup-1", createdAt: "2026-10-04T00:00:00Z", bytes: 100, sha256: "a".repeat(64), includesStorage: true }] });
  vi.mocked(dataApi.action).mockResolvedValue({ ...initial, status: "installing" });
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Backups" }));
  const restore = screen.getByRole("button", { name: "Restore selected backup" });
  expect((restore as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText("Backup to restore"), { target: { value: "backup-1" } });
  fireEvent.click(screen.getByLabelText("I have capacity for a fresh data stack."));
  expect((restore as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText("Restore this backup and replace the active stack."));
  fireEvent.click(restore);
  await waitFor(() => expect(dataApi.action).toHaveBeenCalledWith("owned", "restore", { acknowledgeResources: true, backupId: "backup-1", confirmRestore: true }));
});

it("clears an initial user password immediately and retains actionable service failure", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready" });
  vi.mocked(dataApi.users).mockResolvedValue({ users: [], page: 1, limit: 100 });
  let fail!: (reason: Error) => void;
  vi.mocked(dataApi.createUser).mockImplementation(() => new Promise((_resolve, reject) => { fail = reject; }));
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Authentication" }));
  await screen.findByText("No application users yet.");
  fireEvent.click(screen.getByText("Create a confirmed user"));
  fireEvent.change(screen.getByLabelText("User email"), { target: { value: "person@example.com" } });
  fireEvent.change(screen.getByLabelText("Initial password"), { target: { value: "test-password-12345" } });
  fireEvent.click(screen.getByRole("button", { name: "Create confirmed user" }));
  expect((screen.getByLabelText("Initial password") as HTMLInputElement).value).toBe("");
  fail(new Error("Refresh service health and retry."));
  expect((await screen.findByRole("alert")).textContent).toBe("Refresh service health and retry.");
  expect(document.body.textContent).not.toContain("test-password-12345");
});

it("labels a remote worker loopback address without presenting it as a public site", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready", url: "http://127.0.0.1:45678", endpointScope: "selected-docker-host-loopback", publishableKey: "anonymous-public-fixture" });
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  expect((screen.getByLabelText("API address on the selected Docker host") as HTMLInputElement).value).toBe("http://127.0.0.1:45678");
  expect(screen.queryByRole("link")).toBeNull();
  expect(screen.getByText(/A cloud workspace needs an HTTPS proxy and domain/)).not.toBeNull();
});

it("connects a running cloud data API only after explicit browser preparation", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready", endpointScope: "workspace-browser-pending", publishableKey: "anonymous-public-fixture" });
  let finish!: (value: { url: string; projectId: string; kind: "data"; resourceId: string }) => void;
  vi.mocked(platformApi.prepareBrowser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  expect(platformApi.prepareBrowser).not.toHaveBeenCalled();
  expect(screen.queryByText("An API address will appear after installation.")).toBeNull();
  expect(screen.queryByLabelText("API address for your browser")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  expect(platformApi.prepareBrowser).toHaveBeenCalledExactlyOnceWith("owned", "data");
  expect((screen.getByRole("button", { name: "Connecting browser API…" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Connecting browser API…" }));
  expect(platformApi.prepareBrowser).toHaveBeenCalledTimes(1);
  const address = `http://${"b".repeat(64)}.localhost:8731/`;
  finish({ url: address, projectId: "owned", kind: "data", resourceId: "stable-data-identity" });
  expect((await screen.findByLabelText("API address for your browser") as HTMLInputElement).value).toBe(address);
  expect((screen.getByLabelText(/Public anonymous key/) as HTMLTextAreaElement).value).toBe("anonymous-public-fixture");
  expect(screen.queryByRole("button", { name: "Connect browser API" })).toBeNull();
  expect(screen.getByText(/active Luci workspace connection/)).not.toBeNull();
});

it("keeps cloud data browser failure retryable and rejects control origins or another project", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready", endpointScope: "workspace-browser-pending" });
  vi.mocked(platformApi.prepareBrowser)
    .mockRejectedValueOnce(new Error("The workspace is disconnected. Reconnect it in setup."))
    .mockResolvedValueOnce({ url: "http://127.0.0.1:8730/", projectId: "owned", kind: "data", resourceId: "data-identity" })
    .mockResolvedValueOnce({ url: "http://127.0.0.1:45678/", projectId: "other-project", kind: "data", resourceId: "data-identity" });
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  expect((await screen.findByRole("alert")).textContent).toContain("workspace is disconnected");
  fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("could not verify"));
  fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  await waitFor(() => expect(platformApi.prepareBrowser).toHaveBeenCalledTimes(3));
  await waitFor(() => expect((screen.getByRole("button", { name: "Connect browser API" }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.queryByLabelText("API address for your browser")).toBeNull();
});

it("ignores a late cloud API connection after switching project", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready", endpointScope: "workspace-browser-pending" });
  let finish!: (value: { url: string; projectId: string; kind: "data"; resourceId: string }) => void;
  vi.mocked(platformApi.prepareBrowser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, projectId: "other", status: "ready", endpointScope: "workspace-browser-pending" });
  view.rerender(<DataWorkspace projectId="other" dockerReady />);
  finish({ url: "http://127.0.0.1:45678/", projectId: "owned", kind: "data", resourceId: "data-identity" });
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  expect(screen.queryByLabelText("API address for your browser")).toBeNull();
  expect(screen.getByRole("button", { name: "Connect browser API" })).not.toBeNull();
});

it("retains an existing local browser data endpoint without allocating a cloud connection", async () => {
  const address = `http://${"c".repeat(64)}.localhost:8731/`;
  vi.mocked(dataApi.status).mockResolvedValue({ ...initial, status: "ready", url: address, endpointScope: "browser-gateway" });
  render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" }));
  expect((screen.getByLabelText("API address for your browser") as HTMLInputElement).value).toBe(address);
  expect(platformApi.prepareBrowser).not.toHaveBeenCalled();
});
