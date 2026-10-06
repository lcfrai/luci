import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DataWorkspace } from "./DataWorkspace";
import { DataUsers, DataStorage, DataTables, DataRealtime } from "./DataControls";
import { dataApi, type DataStatus } from "../../api/data";
import { platformApi } from "../../api/platform";
const access = vi.hoisted(() => ({ read: true, edit: true, deploy: true, secrets: true, execute: true, loading: false, error: null as string | null, scope: "first" }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: access.loading, error: access.error, canRead: access.read,
  canEdit: !access.loading && !access.error && access.read && access.edit, canExecute: !access.loading && !access.error && access.edit && access.execute,
  permissions: { ...access } }) }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${access.scope}:${id}` }));
vi.mock("./DataEmail", () => ({ DataEmail: () => null }));
vi.mock("./DataClientSetup", () => ({ DataClientSetup: () => null }));
vi.mock("./DataPublicDomain", () => ({ DataPublicDomain: () => null }));
vi.mock("../../api/platform", async actual => ({ ...await actual<typeof import("../../api/platform")>(), platformApi: { prepareBrowser: vi.fn() } }));
vi.mock("../../api/data", () => ({ dataApi: { status: vi.fn(), action: vi.fn(), users: vi.fn(), createUser: vi.fn(), tables: vi.fn(), createTable: vi.fn(), buckets: vi.fn(), createBucket: vi.fn(), realtime: vi.fn(), updateRealtime: vi.fn(), downloadBackup: vi.fn() } }));
const ready: DataStatus = { projectId: "owned", provider: "supabase", version: "fixture", revision: 1, status: "ready", message: "Saved service status.", services: [], backups: [{ id: "backup", createdAt: "2026-10-06T00:00:00Z", bytes: 100, sha256: "a".repeat(64), includesStorage: true }], capabilities: {}, requirements: { memoryMiB: 4096, cpuMillis: 2000, diskGiB: 40 }, priorStackRetained: false };
beforeEach(() => {
  vi.clearAllMocks(); Object.assign(access, { read: true, edit: true, deploy: true, secrets: true, execute: true, loading: false, error: null, scope: "first" });
  vi.mocked(dataApi.status).mockResolvedValue(structuredClone(ready));
  vi.mocked(dataApi.users).mockResolvedValue({ users: [], page: 1, limit: 100 });
  vi.mocked(dataApi.tables).mockResolvedValue({ tables: [] });
  vi.mocked(dataApi.buckets).mockResolvedValue([]);
  vi.mocked(dataApi.realtime).mockResolvedValue({ tables: [], publication: "supabase_realtime", rlsRequired: true });
});
function disabled(name: string) { return (screen.getByRole("button", { name }) as HTMLButtonElement).disabled; }

it.each(["read", "deploy", "secrets"] as const)("does not request private data without the %s grant", async permission => {
  access[permission] = false;
  render(<><DataWorkspace projectId="owned" dockerReady /><DataUsers projectId="owned" /><DataTables projectId="owned" /><DataStorage projectId="owned" /><DataRealtime projectId="owned" /></>);
  await act(async () => {});
  expect(dataApi.status).not.toHaveBeenCalled(); expect(dataApi.users).not.toHaveBeenCalled(); expect(dataApi.tables).not.toHaveBeenCalled(); expect(dataApi.buckets).not.toHaveBeenCalled();
  expect(screen.getAllByText(/Ask a team owner for project read, deployment and secrets/).length).toBeGreaterThan(0);
  expect(screen.queryByRole("button", { name: "Install data services" })).toBeNull();
});
it("allows a private reader to inspect/download without enabling health or database writes", async () => {
  access.edit = false;
  render(<DataWorkspace projectId="owned" dockerReady />);
  await screen.findByText("Saved service status.");
  expect(disabled("Check service health")).toBe(true); expect(disabled("Stop data services")).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Refresh saved data status" }));
  await waitFor(() => expect(dataApi.status).toHaveBeenCalledTimes(2));
  fireEvent.click(screen.getByRole("button", { name: "Backups" }));
  expect(disabled("Create data backup")).toBe(true); expect(disabled("Restore selected backup")).toBe(true);
  await waitFor(() => expect(disabled("Download archive")).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "Download archive" }));
  await waitFor(() => expect(dataApi.downloadBackup).toHaveBeenCalledWith("owned", ready.backups[0], expect.any(Function)));
  expect(dataApi.action).not.toHaveBeenCalled();
});
it("allows private user reads and refresh but rejects even a synthetic create submit for a read-only member", async () => {
  access.edit = false;
  render(<DataUsers projectId="owned" />);
  await screen.findByText("No application users yet.");
  fireEvent.click(screen.getByText("Create a confirmed user"));
  expect(disabled("Create confirmed user")).toBe(true); expect(disabled("Refresh users")).toBe(false);
  fireEvent.submit(screen.getByLabelText("User email").closest("form")!);
  fireEvent.click(screen.getByRole("button", { name: "Refresh users" }));
  await waitFor(() => expect(dataApi.users).toHaveBeenCalledTimes(2));
  expect(dataApi.createUser).not.toHaveBeenCalled();
});
it("keeps the same user draft across routine permission refresh, then clears it on confirmed denial", async () => {
  const view = render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Authentication" }));
  await screen.findByText("No application users yet."); fireEvent.click(screen.getByText("Create a confirmed user"));
  const email = screen.getByLabelText("User email") as HTMLInputElement;
  const password = screen.getByLabelText("Initial password") as HTMLInputElement;
  fireEvent.change(email, { target: { value: "draft@example.test" } }); fireEvent.change(password, { target: { value: "pending-private-password" } });
  access.loading = true; access.deploy = false; access.secrets = false;
  view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  expect(screen.getByLabelText("User email")).toBe(email); expect(email.value).toBe("draft@example.test"); expect(password.value).toBe("pending-private-password"); expect(disabled("Create confirmed user")).toBe(true);
  access.loading = false; access.deploy = true; access.secrets = true;
  view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  expect(screen.getByLabelText("Initial password")).toBe(password); expect(disabled("Create confirmed user")).toBe(false);
  access.secrets = false; view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  expect(screen.queryByLabelText("Initial password")).toBeNull(); expect(screen.queryByText("Saved service status.")).toBeNull();
  access.secrets = true; view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Authentication" }));
  expect((await screen.findByLabelText("Initial password") as HTMLInputElement).value).toBe("");
});
it("discards delayed user mutations after edit revocation even if editing is restored", async () => {
  let finish!: (value: Awaited<ReturnType<typeof dataApi.createUser>>) => void;
  vi.mocked(dataApi.createUser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DataUsers projectId="owned" />);
  await screen.findByText("No application users yet."); fireEvent.click(screen.getByText("Create a confirmed user"));
  fireEvent.change(screen.getByLabelText("User email"), { target: { value: "draft@example.test" } }); fireEvent.change(screen.getByLabelText("Initial password"), { target: { value: "pending-password" } });
  fireEvent.click(screen.getByRole("button", { name: "Create confirmed user" }));
  access.edit = false; view.rerender(<DataUsers projectId="owned" />); access.edit = true; view.rerender(<DataUsers projectId="owned" />);
  await act(async () => finish({ id: "user", email: "draft@example.test", created_at: "now" }));
  expect(screen.queryByText("Create user completed.")).toBeNull(); expect((screen.getByLabelText("User email") as HTMLInputElement).value).toBe("draft@example.test");
  expect(dataApi.users).toHaveBeenCalledTimes(1); expect(disabled("Create confirmed user")).toBe(false);
});
it("rejects delayed browser grants after denial and regrant without changing the saved API address", async () => {
  vi.mocked(dataApi.status).mockResolvedValue({ ...ready, endpointScope: "workspace-browser-pending" });
  let finish!: (value: Awaited<ReturnType<typeof platformApi.prepareBrowser>>) => void;
  vi.mocked(platformApi.prepareBrowser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DataWorkspace projectId="owned" dockerReady />);
  fireEvent.click(await screen.findByRole("button", { name: "Connect" })); fireEvent.click(screen.getByRole("button", { name: "Connect browser API" }));
  access.execute = false; view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  access.execute = true; view.rerender(<DataWorkspace projectId="owned" dockerReady />);
  await act(async () => finish({ projectId: "owned", kind: "data", resourceId: "data-owned", url: "http://data.fixture.localhost:1234/" }));
  expect(screen.queryByLabelText("API address for your browser")).toBeNull(); expect(disabled("Connect browser API")).toBe(false);
});
it("hides private data on permission errors and fences a same-project account switch", async () => {
  let finish!: (value: Awaited<ReturnType<typeof dataApi.users>>) => void;
  vi.mocked(dataApi.users).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DataUsers projectId="owned" />);
  access.error = "Membership denied"; access.read = false; view.rerender(<DataUsers projectId="owned" />);
  expect(screen.getByText(/Data access unavailable/)).not.toBeNull(); expect(screen.queryByLabelText("Initial password")).toBeNull();
  access.error = null; access.read = true; access.scope = "second"; view.rerender(<DataUsers projectId="owned" />);
  await screen.findByText("No application users yet.");
  await act(async () => finish({ users: [{ id: "old", email: "old-private@example.test", created_at: "now" }], page: 1, limit: 100 }));
  expect(screen.queryByText("old-private@example.test")).toBeNull();
});
