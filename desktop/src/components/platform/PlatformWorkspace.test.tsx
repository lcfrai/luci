import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PlatformWorkspace } from "./PlatformWorkspace";
import { platformApi, type PlatformProject } from "../../api/platform";
import { productApi } from "../../api/product";
vi.mock("./DataWorkspace", () => ({ DataWorkspace: () => <div>Project data services</div> }));
const access = vi.hoisted(() => ({ loading: false, error: "", edit: true, deploy: true, secrets: true, execute: true }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({
  loading: access.loading, error: access.error, canRead: true,
  canEdit: !access.loading && !access.error && access.edit,
  canExecute: !access.loading && !access.error && access.edit && access.execute,
  permissions: { read: true, admin: false, ...access },
}) }));

vi.mock("../../api/platform", async (importOriginal) => ({
  ...await importOriginal<typeof import("../../api/platform")>(),
  platformApi: {
    status: vi.fn(),
    project: vi.fn(),
    connection: vi.fn(),
    projects: vi.fn(),
    deploy: vi.fn(),
    database: vi.fn(),
    restoreDatabase: vi.fn(),
    recoverPreviousDatabase: vi.fn(),
    connect: vi.fn(),
    disconnect: vi.fn(),
    mapProject: vi.fn(),
    deployPanacea: vi.fn(),
    refreshPanacea: vi.fn(),
    action: vi.fn(),
    logs: vi.fn(),
    downloadBackup: vi.fn(),
    prepareBrowser: vi.fn(),
  },
}));
vi.mock("../../api/product", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../api/product")>();
  return {
    ...actual,
    productApi: { baseUrl: actual.productApi.baseUrl, gitDelivery: vi.fn() },
    errorText: (value: unknown) => value instanceof Error ? value.message : "Request failed",
  };
});
const project: PlatformProject = {
  projectId: "owned",
  deployments: [],
  backups: [],
  database: {
    status: "not-created",
    engine: "PostgreSQL 16.15",
    message: "Create a private database.",
  },
};
beforeEach(() => {
  Object.assign(access, { loading: false, error: "", edit: true, deploy: true, secrets: true, execute: true });
  vi.mocked(platformApi.restoreDatabase).mockReset();
  vi.mocked(platformApi.prepareBrowser).mockReset();
  vi.mocked(platformApi.status).mockResolvedValue({
    dockerReady: true,
    scope: "Local persistent services.",
    limits: { memoryMiB: 512, cpuMillis: 500 },
    permissions: { connectPanacea: true, deployPanacea: true },
    supabase: {
      status: "not-installed",
      message: "Full Supabase is not installed.",
      upstream: "https://supabase.com",
    },
  });
  vi.mocked(platformApi.project).mockResolvedValue({
    project: structuredClone(project),
    busy: false,
  });
  vi.mocked(platformApi.connection).mockResolvedValue({
    configured: false,
    canManage: true,
  });
  vi.mocked(productApi.gitDelivery).mockResolvedValue({
    head: "a".repeat(40),
    branch: "main",
  });
});

it("distinguishes permission uncertainty from resolved missing grants without losing a deployment draft", async () => {
  const view = render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  const port = screen.getByLabelText("Application port") as HTMLInputElement;
  fireEvent.change(port, { target: { value: "8080" } });
  const start = () => screen.getByRole("button", { name: "Start persistent deployment" }) as HTMLButtonElement;
  const missingDeployment = /Deployment access is required to build/;
  const missingLogs = /Application logs require deployment and secrets access/;
  const missingPreview = /Opening private application previews requires execution access/;
  const checkUncertainDraft = () => {
    expect(screen.queryByText(missingDeployment)).toBeNull();
    expect(screen.queryByText(missingLogs)).toBeNull();
    expect(screen.queryByText(missingPreview)).toBeNull();
    expect(screen.getByLabelText("Application port")).toBe(port);
    expect(port.value).toBe("8080");
    expect(start().disabled).toBe(true);
    fireEvent.submit(port.closest("form")!);
    expect(platformApi.deploy).not.toHaveBeenCalled();
  };

  Object.assign(access, { loading: true, edit: false, deploy: false, secrets: false, execute: false });
  view.rerender(<PlatformWorkspace projectId="owned" />);
  expect(screen.getByText("Checking platform permissions…")).not.toBeNull();
  checkUncertainDraft();

  Object.assign(access, { loading: false, error: "The workspace connection is temporarily unavailable." });
  view.rerender(<PlatformWorkspace projectId="owned" />);
  expect(screen.queryByText("Checking platform permissions…")).toBeNull();
  expect(screen.getByText(/Platform permissions could not be confirmed/).textContent).toContain(access.error);
  checkUncertainDraft();

  access.error = "";
  view.rerender(<PlatformWorkspace projectId="owned" />);
  expect(screen.queryByText(/Platform permissions could not be confirmed/)).toBeNull();
  expect(screen.getByText(missingDeployment)).not.toBeNull();
  expect(screen.getByText(missingLogs)).not.toBeNull();
  expect(screen.getByText(missingPreview)).not.toBeNull();
  expect(start().disabled).toBe(true);

  Object.assign(access, { edit: true, deploy: true, secrets: true, execute: true });
  view.rerender(<PlatformWorkspace projectId="owned" />);
  expect(screen.queryByText(missingDeployment)).toBeNull();
  expect(screen.queryByText(missingLogs)).toBeNull();
  expect(screen.queryByText(missingPreview)).toBeNull();
  expect(screen.getByLabelText("Application port")).toBe(port);
  expect(port.value).toBe("8080");
  expect(start().disabled).toBe(false);
  expect(platformApi.deploy).not.toHaveBeenCalled();
});

it("shows accepted restore as progress, polls its actual result and never calls it completed early", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  try {
    const backup = { id: "backup-owned", createdAt: "2026-10-06T00:00:00Z", bytes: 2048, sha256: "a".repeat(64) };
    const stopped = { ...structuredClone(project), backups: [backup], database: { ...project.database, status: "stopped" } };
    const operation = { id: "restore-owned", action: "restore", status: "running" as const, stage: "restoring", backupId: backup.id, message: "Restoring into a separate candidate.", createdAt: "now", updatedAt: "now" };
    vi.mocked(platformApi.project)
      .mockResolvedValueOnce({ project: stopped, busy: false })
      .mockResolvedValueOnce({ project: { ...stopped, databaseOperation: operation }, busy: true })
      .mockResolvedValue({ project: { ...stopped, database: { ...stopped.database, status: "ready" }, databaseOperation: { ...operation, status: "succeeded", stage: "complete", message: "Candidate passed real health checks." } }, busy: false });
    vi.mocked(platformApi.restoreDatabase).mockResolvedValue(operation);
    render(<PlatformWorkspace projectId="owned" />);
    fireEvent.click(await screen.findByRole("button", { name: "Data" }));
    fireEvent.click(screen.getByText("Standalone PostgreSQL database"));
    fireEvent.change(screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }), { target: { value: backup.id } });
    fireEvent.click(screen.getByRole("checkbox", { name: /Switch to this backup/ }));
    fireEvent.click(screen.getByRole("button", { name: "Restore PostgreSQL backup" }));
    expect(await screen.findByText("Restoring into a separate candidate.")).not.toBeNull();
    expect(screen.queryByText("Restore PostgreSQL backup completed.")).toBeNull();
    expect(platformApi.restoreDatabase).toHaveBeenCalledExactlyOnceWith("owned", backup.id);
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); });
    expect(await screen.findByText("Candidate passed real health checks.")).not.toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Open application deployments" }));
    expect(screen.getByRole("button", { name: "Deployments" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: "Data" }).getAttribute("aria-pressed")).toBe("false");
    expect(platformApi.restoreDatabase).toHaveBeenCalledExactlyOnceWith("owned", backup.id);
    expect(platformApi.deploy).not.toHaveBeenCalled();
    expect(platformApi.action).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

it("refuses a foreign progress snapshot without replacing this project's retained services", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  try {
    const owned = { ...structuredClone(project), database: { ...project.database, status: "stopped", message: "Owned database retained." } };
    vi.mocked(platformApi.project).mockResolvedValueOnce({ project: owned, busy: true })
      .mockResolvedValue({ project: { ...owned, projectId: "foreign", database: { ...owned.database, message: "Foreign database." } }, busy: false });
    render(<PlatformWorkspace projectId="owned" />);
    fireEvent.click(await screen.findByRole("button", { name: "Data" }));
    fireEvent.click(screen.getByText("Standalone PostgreSQL database"));
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); });
    expect((await screen.findByRole("alert")).textContent).toContain("could not verify this project's operation progress");
    expect(screen.getByText("Owned database retained.")).not.toBeNull();
    expect(screen.queryByText("Foreign database.")).toBeNull();
    expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

const pendingBrowserDeployment = {
  id: "cloud-application", projectId: "owned", provider: "docker" as const,
  commit: "a".repeat(40), configurationRevision: 1, status: "ready",
  browserConnectionRequired: true, message: "Application is healthy on the workspace.",
  createdAt: "now", updatedAt: "now",
};

function pendingBrowserProject() {
  vi.mocked(platformApi.project).mockResolvedValue({ project: { ...structuredClone(project), deployments: [pendingBrowserDeployment] }, busy: false });
}

it("prepares a cloud application's isolated browser address only after Open, then retains a safe link", async () => {
  pendingBrowserProject();
  const address = `http://${"a".repeat(32)}.${"a".repeat(32)}.localhost:8731/`;
  let finish!: (value: { url: string; projectId: string; kind: "application"; resourceId: string }) => void;
  vi.mocked(platformApi.prepareBrowser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  expect(platformApi.prepareBrowser).not.toHaveBeenCalled();
  expect(screen.queryByRole("link", { name: "Open deployed application" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  expect(platformApi.prepareBrowser).toHaveBeenCalledExactlyOnceWith("owned", "application", "cloud-application");
  expect(open).not.toHaveBeenCalled();
  expect((screen.getByRole("button", { name: "Open deployed application" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  expect(platformApi.prepareBrowser).toHaveBeenCalledTimes(1);
  finish({ url: address, projectId: "owned", kind: "application", resourceId: "cloud-application" });
  const link = await screen.findByRole("link", { name: "Open deployed application" });
  expect(link.getAttribute("href")).toBe(address);
  expect(open).toHaveBeenCalledExactlyOnceWith(address, "_blank", "noopener,noreferrer");
  expect(screen.getByText("Browser connection ready. You can open the application below.")).not.toBeNull();
});

it.each([
  "http://user:password@127.0.0.1:45678/",
  `http://${"a".repeat(63)}.localhost:8731/`,
  `http://${"a".repeat(64)}.localhost.example.com:8731/`,
  "https://external.example.com/",
  "http://127.0.0.1:8730/",
  "http://localhost:3000/",
])("rejects an unsafe prepared browser address without navigation: %s", async (url) => {
  pendingBrowserProject();
  vi.mocked(platformApi.prepareBrowser).mockResolvedValue({ url, projectId: "owned", kind: "application", resourceId: "cloud-application" });
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  expect((await screen.findByRole("alert")).textContent).toContain("could not verify");
  expect(open).not.toHaveBeenCalled();
  expect(screen.queryByRole("link", { name: "Open deployed application" })).toBeNull();
  expect((screen.getByRole("button", { name: "Open deployed application" }) as HTMLButtonElement).disabled).toBe(false);
});

it("keeps failed cloud browser preparation recoverable and refuses another resource binding", async () => {
  pendingBrowserProject();
  vi.mocked(platformApi.prepareBrowser)
    .mockRejectedValueOnce(new Error("Reconnect your workspace in setup."))
    .mockResolvedValueOnce({ url: "http://127.0.0.1:45678/", projectId: "owned", kind: "application", resourceId: "other-application" });
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  expect((await screen.findByRole("alert")).textContent).toBe("Reconnect your workspace in setup.");
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("could not verify"));
  expect(platformApi.prepareBrowser).toHaveBeenCalledTimes(2);
  expect(open).not.toHaveBeenCalled();
});

it("does not open a late application browser response after leaving its project", async () => {
  pendingBrowserProject();
  let finish!: (value: { url: string; projectId: string; kind: "application"; resourceId: string }) => void;
  vi.mocked(platformApi.prepareBrowser).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  const view = render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  fireEvent.click(screen.getByRole("button", { name: "Open deployed application" }));
  view.unmount();
  finish({ url: "http://127.0.0.1:45678/", projectId: "owned", kind: "application", resourceId: "cloud-application" });
  await Promise.resolve();
  expect(open).not.toHaveBeenCalled();
});
it("starts with an honest capability overview and discloses real service controls by section", async () => {
  render(<PlatformWorkspace projectId="owned" />);
  expect(
    await screen.findByRole("heading", { name: "Your running services" }),
  ).not.toBeNull();
  expect(screen.queryByLabelText("Application port")).toBeNull();
  expect(screen.getByText("Full Supabase is not installed.")).not.toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Data" }));
  fireEvent.click(screen.getByText("Standalone PostgreSQL database"));
  expect(
    (
      screen.getByRole("button", {
        name: "Start PostgreSQL",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(false);
  expect(
    (
      screen.getByRole("button", {
        name: "Create database backup",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(screen.queryByLabelText("Scoped Panacea API key")).toBeNull();
});
it("records a queued deployment without saying the deployment is complete", async () => {
  vi.mocked(platformApi.deploy).mockResolvedValue({
    id: "release",
    projectId: "owned",
    provider: "docker",
    commit: "a".repeat(40),
    configurationRevision: 1,
    status: "building",
    message: "Building",
    createdAt: "now",
    updatedAt: "now",
  });
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  fireEvent.change(screen.getByLabelText("Application port"), {
    target: { value: "8080" },
  });
  fireEvent.click(
    screen.getByRole("button", { name: "Start persistent deployment" }),
  );
  await waitFor(() =>
    expect(platformApi.deploy).toHaveBeenCalledWith(
      "owned",
      "a".repeat(40),
      8080,
      "/",
    ),
  );
  expect(
    await screen.findByText(
      "Deployment accepted and running. Readiness has not been verified yet.",
    ),
  ).not.toBeNull();
  expect(
    screen.queryByText("Start persistent deployment completed."),
  ).toBeNull();
});
it("keeps Docker available when a connected Panacea account cannot be read", async () => {
  vi.mocked(platformApi.connection).mockResolvedValue({
    configured: true,
    canManage: true,
    baseUrl: "https://provider.example",
  });
  vi.mocked(platformApi.projects).mockRejectedValue(
    new Error("provider unavailable"),
  );
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(await screen.findByRole("button", { name: "Deployments" }));
  expect(
    (
      screen.getByRole("button", {
        name: "Start persistent deployment",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Hosting connection" }));
  expect(screen.getByRole("alert").textContent).toContain(
    "Your Docker services and database remain available",
  );
});
it("clears a submitted provider key immediately and never renders failed provider bodies", async () => {
  let reject!: (reason: Error) => void;
  vi.mocked(platformApi.connect).mockImplementation(
    () =>
      new Promise((_resolve, fail) => {
        reject = fail;
      }),
  );
  render(<PlatformWorkspace projectId="owned" />);
  fireEvent.click(
    await screen.findByRole("button", { name: "Hosting connection" }),
  );
  fireEvent.change(screen.getByLabelText("Panacea HTTPS origin"), {
    target: { value: "https://provider.example" },
  });
  fireEvent.change(screen.getByLabelText("Scoped Panacea API key"), {
    target: { value: "private-key-value-12345" },
  });
  fireEvent.click(
    screen.getByRole("button", { name: "Verify and save connection" }),
  );
  expect(
    (screen.getByLabelText("Scoped Panacea API key") as HTMLInputElement).value,
  ).toBe("");
  reject(new Error("Panacea could not verify the scoped API credential."));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "could not verify",
  );
  expect(document.body.textContent).not.toContain("private-key-value-12345");
});

it.each(["Restart deployment", "Verify deployment health"])("reports an HTTP-success unhealthy result honestly after %s", async label => {
  pendingBrowserProject();
  vi.mocked(platformApi.action).mockResolvedValue({ ...pendingBrowserDeployment, status: "failed", message: "Application did not pass its readiness probe." });
  render(<PlatformWorkspace projectId="owned" inspectDeploymentId={pendingBrowserDeployment.id} />);
  fireEvent.click(await screen.findByRole("button", { name: label }));
  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain(`${label} needs attention.`);
  expect(alert.textContent).toContain("Application did not pass its readiness probe.");
  expect(screen.queryByText(`${label} completed.`)).toBeNull();
  expect(platformApi.action).toHaveBeenLastCalledWith("owned", pendingBrowserDeployment.id, label === "Restart deployment" ? "restart" : "refresh");
});

it("opens an exact retained application and requires review before replacing it with current committed source", async () => {
  const old = { ...pendingBrowserDeployment, status: "failed", commit: "b".repeat(40) };
  vi.mocked(platformApi.project).mockResolvedValue({ project: { ...project, deployments: [old] }, busy: false });
  vi.mocked(platformApi.deploy).mockResolvedValue({ ...old, id: "replacement", status: "building", commit: "a".repeat(40) });
  const callsBefore = vi.mocked(platformApi.deploy).mock.calls.length;
  render(<PlatformWorkspace projectId="owned" inspectDeploymentId={old.id} />);
  const row = await screen.findByRole("article", { name: `Deployment ${old.id}` });
  await waitFor(() => expect(document.activeElement).toBe(row));
  fireEvent.click(screen.getByRole("button", { name: "Prepare replacement deployment" }));
  expect(screen.getByRole("heading", { name: "Review replacement deployment" })).not.toBeNull();
  expect(screen.getByText(/replacement needs new checks and review/)).not.toBeNull();
  const start = screen.getByRole("button", { name: "Start replacement deployment" });
  expect((start as HTMLButtonElement).disabled).toBe(true);
  expect(platformApi.deploy).toHaveBeenCalledTimes(callsBefore);
  fireEvent.click(screen.getByRole("checkbox", { name: "Create this replacement and retain the earlier deployment." }));
  fireEvent.click(start);
  await waitFor(() => expect(platformApi.deploy).toHaveBeenLastCalledWith("owned", "a".repeat(40), 3000, "/"));
  expect(screen.getByRole("article", { name: `Deployment ${old.id}` })).not.toBeNull();
  expect(await screen.findByText("Deployment accepted and running. Readiness has not been verified yet.")).not.toBeNull();
});

it("does not replace a missing inspected application with the newest row", async () => {
  pendingBrowserProject();
  render(<PlatformWorkspace projectId="owned" inspectDeploymentId="missing-deployment" />);
  expect((await screen.findByRole("alert")).textContent).toContain("selected application is no longer");
  expect(document.activeElement?.getAttribute("aria-label")).not.toBe(`Deployment ${pendingBrowserDeployment.id}`);
});

it("waits for platform readiness before selecting a retained row and supports inspecting it again", async () => {
  pendingBrowserProject();
  let finish!: (value: Awaited<ReturnType<typeof platformApi.status>>) => void;
  const status = await platformApi.status();
  vi.mocked(platformApi.status).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<PlatformWorkspace projectId="owned" inspectDeploymentId={pendingBrowserDeployment.id} inspectionRequest={1} />);
  await act(async () => {});
  expect(screen.queryByRole("alert")).toBeNull();
  await act(async () => finish(status));
  const row = await screen.findByRole("article", { name: `Deployment ${pendingBrowserDeployment.id}` });
  await waitFor(() => expect(document.activeElement).toBe(row));
  fireEvent.click(screen.getByRole("button", { name: "Overview" }));
  expect(screen.queryByRole("article", { name: `Deployment ${pendingBrowserDeployment.id}` })).toBeNull();
  view.rerender(<PlatformWorkspace projectId="owned" inspectDeploymentId={pendingBrowserDeployment.id} inspectionRequest={2} />);
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("article", { name: `Deployment ${pendingBrowserDeployment.id}` })));
});
