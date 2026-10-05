import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DependencySetup } from "./DependencySetup";
import type { DependencyClient, DependencyStatus } from "../../api/dependencies";

const snapshot = (): DependencyStatus => ({
  scope: "control-plane-host", platform: "windows", canInstall: true,
  installer: { id: "winget", status: "ready", message: "Microsoft source verified" },
  dependencies: [{ id: "docker", name: "Docker Desktop", packageId: "Docker.DockerDesktop", supported: true, reason: "",
    check: { status: "missing", message: "Docker is not installed" }, actionRequired: "none", job: null }],
});
function client(value = snapshot()): DependencyClient {
  return { get: vi.fn().mockResolvedValue(value), check: vi.fn().mockResolvedValue(value), install: vi.fn().mockResolvedValue({
    id: "install-1", dependencyId: "docker", status: "action_required", actionRequired: "restart_app",
    message: "Reopen Luci to use the installed tool", createdAt: "2026-10-04T00:00:00Z", updatedAt: "2026-10-04T00:00:01Z",
  }) };
}
beforeEach(() => vi.restoreAllMocks());

it("loads cached availability and installs only the selected tool after explicit agreement", async () => {
  const api = client();
  render(<DependencySetup client={api} canManage />);
  const install = await screen.findByRole("button", { name: "Install Docker Desktop" });
  expect((install as HTMLButtonElement).disabled).toBe(true);
  expect(api.check).not.toHaveBeenCalled();
  expect(api.install).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("checkbox", { name: /I accept the publisher/ }));
  await userEvent.click(install);
  await screen.findByText("Reopen Luci to use the installed tool");
  expect(api.install).toHaveBeenCalledExactlyOnceWith("docker", true);
  expect(screen.queryByRole("button", { name: "Install Docker Desktop" })).toBeNull();
  expect(screen.getByText(/Choose “Check tools again”/)).not.toBeNull();
});

it("requires a new check before retrying a failed installation and clears old consent", async () => {
  const value = snapshot();
  value.dependencies[0].job = { id: "failed-1", dependencyId: "docker", status: "failed", actionRequired: "none", message: "Installer failed", createdAt: "2026-10-04T00:00:00Z", updatedAt: "2026-10-04T00:00:01Z" };
  const api = client(value);
  vi.mocked(api.check).mockResolvedValue(snapshot());
  render(<DependencySetup client={api} canManage />);
  await screen.findByText("Installation failed");
  expect(screen.queryByRole("button", { name: "Install Docker Desktop" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Check tools again" }));
  const install = await screen.findByRole("button", { name: "Install Docker Desktop" });
  expect((install as HTMLButtonElement).disabled).toBe(true);
  expect(api.check).toHaveBeenCalledOnce();
  expect(api.install).not.toHaveBeenCalled();
});

it("explains the host boundary to a cloud member without exposing installation controls", async () => {
  const api = client();
  render(<DependencySetup client={api} canManage={false} cloudWorkspace automatic />);
  await screen.findByRole("heading", { name: "Docker Desktop" });
  expect(screen.getByText(/do not change your selected cloud workspace/)).not.toBeNull();
  expect(screen.getByText(/Your administrator installs tools/)).not.toBeNull();
  expect(screen.queryByRole("checkbox")).toBeNull();
  expect(api.check).not.toHaveBeenCalled();
  expect(api.install).not.toHaveBeenCalled();
});

it("checks unchecked tools during local onboarding without installing them", async () => {
  const value = snapshot();
  value.dependencies[0].check.status = "unchecked";
  const api = client(value);
  vi.mocked(api.check).mockResolvedValue(snapshot());
  const checked = vi.fn();
  render(<DependencySetup client={api} canManage automatic onChecked={checked} />);
  await screen.findByRole("button", { name: "Install Docker Desktop" });
  await waitFor(() => expect(api.check).toHaveBeenCalledOnce());
  expect(checked).toHaveBeenCalledOnce();
  expect(api.install).not.toHaveBeenCalled();
});
