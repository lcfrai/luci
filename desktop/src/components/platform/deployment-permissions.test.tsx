import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlatformWorkspace } from "./PlatformWorkspace";
import { platformApi, type PlatformDeployment, type PlatformProject } from "../../api/platform";
import { productApi } from "../../api/product";

const grants = vi.hoisted(() => ({ edit: false, deploy: false, secrets: false, execute: false }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: false, error: null, canRead: true, canEdit: grants.edit, canExecute: grants.edit && grants.execute, permissions: { read: true, ...grants } }) }));
vi.mock("./DataWorkspace", () => ({ DataWorkspace: () => null }));
vi.mock("./StandaloneDatabase", () => ({ StandaloneDatabase: () => null }));
const deployment = { id: "deployment-owned", provider: "docker", commit: "a".repeat(40), status: "ready", message: "Last observed healthy", url: "http://127.0.0.1:4567/", browserConnectionRequired: true } as PlatformDeployment;
const project = { projectId: "owned", deployments: [deployment], backups: [], database: { status: "not-created", engine: "PostgreSQL", message: "No database" } } as PlatformProject;
beforeEach(() => {
  Object.assign(grants, { edit: false, deploy: false, secrets: false, execute: false });
  vi.spyOn(platformApi, "status").mockResolvedValue({ dockerReady: true, scope: "Local service", limits: { memoryMiB: 512, cpuMillis: 500 }, permissions: { connectPanacea: true, deployPanacea: true }, supabase: { status: "not-installed", message: "Not installed", upstream: "https://supabase.com" } });
  vi.spyOn(platformApi, "project").mockResolvedValue({ project, busy: false });
  vi.spyOn(platformApi, "connection").mockResolvedValue({ configured: false, canManage: false });
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ head: deployment.commit, branch: "main" });
  vi.spyOn(platformApi, "deploy").mockResolvedValue(deployment);
  vi.spyOn(platformApi, "action").mockResolvedValue(deployment);
  vi.spyOn(platformApi, "logs").mockResolvedValue({ text: "private application output", notice: "Sensitive application output" });
});
afterEach(() => vi.restoreAllMocks());

it.each([
  { edit: false, deploy: false, secrets: false, execute: false },
  { edit: true, deploy: false, secrets: false, execute: true },
])("explains denied platform mutations while permitting read refresh %j", async permissions => {
  Object.assign(grants, permissions);
  render(<PlatformWorkspace projectId="owned" />);
  await userEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  for (const name of ["Start persistent deployment", "Verify deployment health", "Stop deployment", "Restart deployment", "Read application logs"]) {
    const button = screen.getByRole("button", { name }) as HTMLButtonElement;
    expect(button.disabled).toBe(true); fireEvent.click(button);
  }
  expect(screen.getByText(/Ask a team owner for editing and deployment access/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh platform" }));
  await waitFor(() => expect(platformApi.project).toHaveBeenCalledTimes(2));
  expect(platformApi.deploy).not.toHaveBeenCalled(); expect(platformApi.action).not.toHaveBeenCalled(); expect(platformApi.logs).not.toHaveBeenCalled();
});

it("allows deployment without execution or secrets but does not expose logs or a private browser", async () => {
  Object.assign(grants, { edit: true, deploy: true });
  render(<PlatformWorkspace projectId="owned" />);
  await userEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  for (const name of ["Start persistent deployment", "Verify deployment health", "Stop deployment", "Restart deployment"]) expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByRole("button", { name: "Read application logs" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Open deployed application" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Verify deployment health" }));
  expect(platformApi.action).toHaveBeenCalledExactlyOnceWith("owned", "deployment-owned", "refresh");
});

it("permits read-only logs with deployment and secrets grants, and clears them when that grant is removed", async () => {
  Object.assign(grants, { deploy: true, secrets: true });
  const view = render(<PlatformWorkspace projectId="owned" />);
  await userEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  const logs = screen.getByRole("button", { name: "Read application logs" });
  expect((logs as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByRole("button", { name: "Stop deployment" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(logs);
  await screen.findByText("private application output");
  grants.secrets = false;
  view.rerender(<PlatformWorkspace projectId="owned" />);
  await waitFor(() => expect(screen.queryByText("private application output")).toBeNull());
  expect((logs as HTMLButtonElement).disabled).toBe(true);
  expect(platformApi.logs).toHaveBeenCalledExactlyOnceWith("owned", "deployment-owned");
});

it("discards in-flight log output when its secrets grant is removed", async () => {
  Object.assign(grants, { deploy: true, secrets: true });
  let resolve!: (value: { text: string; notice: string }) => void;
  vi.mocked(platformApi.logs).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const view = render(<PlatformWorkspace projectId="owned" />);
  await userEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  await userEvent.click(screen.getByRole("button", { name: "Read application logs" }));
  grants.secrets = false;
  view.rerender(<PlatformWorkspace projectId="owned" />);
  await act(async () => resolve({ text: "late sensitive output", notice: "Private" }));
  expect(screen.queryByText("late sensitive output")).toBeNull();
});

it("does not open a delayed application browser after execution access is revoked", async () => {
  Object.assign(grants, { edit: true, execute: true });
  let finish!: (value: { url: string; projectId: string; kind: "application"; resourceId: string }) => void;
  vi.spyOn(platformApi, "prepareBrowser").mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  const view = render(<PlatformWorkspace projectId="owned" />);
  await userEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  await userEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  grants.execute = false; view.rerender(<PlatformWorkspace projectId="owned" />);
  await act(async () => finish({ url: `http://${"a".repeat(32)}.${"a".repeat(32)}.localhost:8731/`, projectId: "owned", kind: "application", resourceId: deployment.id }));
  expect(open).not.toHaveBeenCalled();
  expect(screen.queryByText("Browser connection ready. You can open the application below.")).toBeNull();
  expect((screen.getByRole("button", { name: "Open deployed application" }) as HTMLButtonElement).disabled).toBe(true);
});
