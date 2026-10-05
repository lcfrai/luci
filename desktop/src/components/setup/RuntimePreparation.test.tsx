import { StrictMode } from "react";
import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuntimePreparation } from "./RuntimePreparation";
import type { ProductClient, SetupStatus } from "../../api/product";

const ready = {
  checks: [
    {
      id: "git",
      label: "Git",
      status: "ready",
      message: "Git available",
      action: "",
    },
    {
      id: "docker",
      label: "Docker",
      status: "ready",
      message: "Docker running",
      action: "",
    },
    {
      id: "sandbox-image",
      label: "Agent runtime",
      status: "missing",
      message: "Image not built",
      action: "Prepare runtime",
    },
  ],
} as SetupStatus;
it("starts the supported runtime preparation once and displays its actual job state", async () => {
  const installRuntime = vi
    .fn()
    .mockResolvedValue({
      status: "installing",
      message: "Building packaged runtime",
    });
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <StrictMode>
      <RuntimePreparation
        client={client}
        manageBacking
        automatic
        onStatus={vi.fn()}
      />
    </StrictMode>,
  );
  await screen.findByText(/Installing runtime… Building packaged runtime/);
  expect(installRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByText(/100%/)).toBeNull();
});
it("does not install when Docker is unavailable and lets the user rerun readiness", async () => {
  const check = vi
    .fn()
    .mockResolvedValue({
      ...ready,
      checks: ready.checks.map((item) =>
        item.id === "docker"
          ? { ...item, status: "missing", message: "Docker stopped" }
          : item,
      ),
    });
  const installRuntime = vi.fn();
  const client = {
    check,
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking
      automatic
      onStatus={vi.fn()}
    />,
  );
  await screen.findByText(/Open Docker Desktop and wait for its engine to start/);
  expect(installRuntime).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Check again" }));
  await waitFor(() => expect(check).toHaveBeenCalledTimes(2));
});
it("retains a real failure and requires an explicit retry rather than repeating installation", async () => {
  const installRuntime = vi
    .fn()
    .mockRejectedValueOnce(new Error("Docker ran out of disk space"))
    .mockResolvedValue({ status: "installing", message: "Building" });
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking
      automatic
      onStatus={vi.fn()}
    />,
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "disk space",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Prepare agent runtime" }),
  );
  await screen.findByText(/Installing runtime… Building/);
  expect(installRuntime).toHaveBeenCalledTimes(2);
});
it("never installs a runtime for an account without backing permission", async () => {
  const installRuntime = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking={false}
      automatic
      onStatus={vi.fn()}
    />,
  );
  await screen.findByText("Docker running");
  expect(installRuntime).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: "Prepare agent runtime" }),
  ).toBeNull();
});
