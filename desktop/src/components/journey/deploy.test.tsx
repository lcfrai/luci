import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import * as setup from "../setup/SetupProvider";

const ready: Release = {
  id: "rel-1",
  projectId: "p-1",
  briefRevision: 3,
  commit: "abc123",
  target: "local",
  status: "ready",
  url: "http://127.0.0.1:54211",
  message: "Health check passed",
  createdAt: "2026-10-02T00:00:00Z",
};
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(productApi, "configuration").mockResolvedValue({
    revision: 0,
    variables: [],
    updatedAt: "",
  });
});
it("offers an embedded preview only for a verified local release and removes it after stop", async () => {
  vi.spyOn(productApi, "releases")
    .mockResolvedValueOnce({ releases: [ready] })
    .mockResolvedValue({ releases: [{ ...ready, status: "stopped" }] });
  const stop = vi
    .spyOn(productApi, "stopRelease")
    .mockResolvedValue({ ...ready, status: "stopped" });
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Open preview" }),
  );
  expect(screen.getByTitle("Project preview").getAttribute("src")).toBe(
    "http://127.0.0.1:54211/",
  );
  await userEvent.click(screen.getByRole("button", { name: "Stop preview" }));
  await waitFor(() =>
    expect(screen.queryByTitle("Project preview")).toBeNull(),
  );
  expect(stop).toHaveBeenCalledWith("p-1", "rel-1");
});
it("records a build failure without exposing an unverified preview", async () => {
  vi.spyOn(productApi, "releases")
    .mockResolvedValueOnce({ releases: [] })
    .mockResolvedValue({
      releases: [
        { ...ready, status: "failed", message: "Docker build failed" },
      ],
    });
  vi.spyOn(productApi, "release").mockRejectedValue(
    new Error("Docker build failed"),
  );
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(
    screen.getByRole("button", { name: "Build local preview" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Docker build failed",
  );
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
});

it("opens a cloud workspace preview through a newly prepared local origin only after a click", async () => {
  vi.spyOn(setup, "useSetup").mockReturnValue({
    workspace: {
      mode: "cloud",
      connectionId: "ssh-1",
      host: "workspace.example",
      ready: true,
    },
  } as ReturnType<typeof setup.useSetup>);
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [{ ...ready, url: "" }],
  });
  const prepare = vi
    .spyOn(productApi, "prepareWorkspacePreview")
    .mockResolvedValue({ url: "http://127.0.0.1:54411/" });
  render(<ProjectDeploy projectId="p-1" />);
  const button = await screen.findByRole("button", { name: "Open preview" });
  expect(prepare).not.toHaveBeenCalled();
  expect(screen.queryByTitle("Project preview")).toBeNull();
  await userEvent.click(button);
  expect(
    (await screen.findByTitle("Project preview")).getAttribute("src"),
  ).toBe("http://127.0.0.1:54411/");
  expect(prepare).toHaveBeenCalledWith("p-1", "rel-1");
});

it("keeps an invalid remote preview address out of the embedded frame", async () => {
  vi.spyOn(setup, "useSetup").mockReturnValue({
    workspace: {
      mode: "cloud",
      connectionId: "ssh-1",
      host: "workspace.example",
      ready: true,
    },
  } as ReturnType<typeof setup.useSetup>);
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [{ ...ready, url: "" }],
  });
  vi.spyOn(productApi, "prepareWorkspacePreview").mockResolvedValue({
    url: "https://unexpected.example",
  });
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Open preview" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "valid local preview address",
  );
  expect(screen.queryByTitle("Project preview")).toBeNull();
});

it("requests rollback for the selected historical release and keeps an unverified replacement out of preview", async () => {
  const cloud = {
    ...ready,
    target: "server-1",
    url: "https://app.example.com",
    status: "stopped",
  };
  vi.spyOn(productApi, "releases")
    .mockResolvedValueOnce({ releases: [cloud] })
    .mockResolvedValue({
      releases: [
        {
          ...cloud,
          id: "replacement",
          status: "attention",
          url: "",
          message: "Reconcile provider IDs",
        },
        cloud,
      ],
    });
  const rollback = vi.spyOn(productApi, "rollbackRelease").mockResolvedValue({
    ...cloud,
    id: "replacement",
    status: "attention",
    url: "",
  });
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Roll back to this release" }),
  );
  await screen.findByText("Reconcile provider IDs");
  expect(rollback).toHaveBeenCalledWith("p-1", "rel-1");
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
});
