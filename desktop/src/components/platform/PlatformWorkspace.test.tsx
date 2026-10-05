import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PlatformWorkspace } from "./PlatformWorkspace";
import { platformApi, type PlatformProject } from "../../api/platform";
import { productApi } from "../../api/product";
vi.mock("./DataWorkspace", () => ({ DataWorkspace: () => <div>Project data services</div> }));

vi.mock("../../api/platform", () => ({
  platformApi: {
    status: vi.fn(),
    project: vi.fn(),
    connection: vi.fn(),
    projects: vi.fn(),
    deploy: vi.fn(),
    database: vi.fn(),
    connect: vi.fn(),
    disconnect: vi.fn(),
    mapProject: vi.fn(),
    deployPanacea: vi.fn(),
    refreshPanacea: vi.fn(),
    action: vi.fn(),
    logs: vi.fn(),
    downloadBackup: vi.fn(),
  },
}));
vi.mock("../../api/product", () => ({
  productApi: { gitDelivery: vi.fn() },
  errorText: (value: unknown) =>
    value instanceof Error ? value.message : "Request failed",
}));
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
