import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import * as setup from "../setup/SetupProvider";
import { DEFAULT_BASE_URL } from "../../api/client";

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
const gatewayURL = "http://" + "a".repeat(32) + "." + "a".repeat(32) + ".localhost:8731/";
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
  expect(
    screen.getByTitle("Project preview").getAttribute("sandbox"),
  ).toContain("allow-same-origin");
  await userEvent.click(screen.getByRole("button", { name: "Stop preview" }));
  await waitFor(() =>
    expect(screen.queryByTitle("Project preview")).toBeNull(),
  );
  expect(stop).toHaveBeenCalledWith("p-1", "rel-1");
});

it.each([window.location.origin + "/", DEFAULT_BASE_URL + "/"])("does not embed either Luci control origin as a project preview: %s", async url => {
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [{ ...ready, url }],
  });
  render(<ProjectDeploy projectId="p-1" />);
  await screen.findByText("Health check passed");
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
  expect(screen.queryByTitle("Project preview")).toBeNull();
});

it("retires an open preview when refreshed release health is no longer ready", async () => {
  vi.spyOn(productApi, "releases").mockResolvedValueOnce({ releases: [ready] }).mockResolvedValue({ releases: [{ ...ready, status: "failed", message: "Health verification failed" }] });
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Open preview" }));
  expect(screen.getByTitle("Project preview")).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("Health verification failed");
  await waitFor(() => expect(screen.queryByTitle("Project preview")).toBeNull());
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
});

it("changes deployment destination only on explicit tab activation", async () => {
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  const delivery = vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ branch: "main", head: "" });
  const targets = vi.spyOn(productApi, "targets");
  render(<ProjectDeploy projectId="p-1" />);
  const user = userEvent.setup();
  screen.getByRole("tab", { name: "On this machine" }).focus();
  await user.keyboard("{ArrowRight}{ArrowRight}");
  expect(delivery).not.toHaveBeenCalled();
  expect(targets).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Build local preview" })).not.toBeNull();
  await user.keyboard("{Enter}");
  await screen.findByRole("button", { name: "Deploy to GitHub Pages" });
  const tab = screen.getByRole("tab", { name: "GitHub Pages" });
  const panel = screen.getByRole("tabpanel", { name: "GitHub Pages" });
  expect(tab.getAttribute("aria-controls")).toBe(panel.id);
  expect(panel.getAttribute("aria-labelledby")).toBe(tab.id);
  expect(delivery).toHaveBeenCalledExactlyOnceWith("p-1");
  expect(targets).not.toHaveBeenCalled();
});
it("uses the default build without expanding technical options and retains truthful pending release state", async () => {
  const building = { ...ready, status: "building", url: "", message: "Building the selected source" };
  vi.spyOn(productApi, "releases").mockResolvedValueOnce({ releases: [] }).mockResolvedValue({ releases: [building] });
  const release = vi.spyOn(productApi, "release").mockResolvedValue(building);
  render(<ProjectDeploy projectId="p-1" />);
  const summary = screen.getByText("Preview build options", { selector: "summary" });
  expect((summary.parentElement as HTMLDetailsElement).open).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  expect(release).toHaveBeenCalledExactlyOnceWith("p-1", 3000, "Dockerfile");
  await screen.findByText("Building the selected source");
  expect(screen.getByText("building")).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
  expect(screen.queryByTitle("Project preview")).toBeNull();
  await userEvent.click(summary);
  expect(screen.getByRole("spinbutton", { name: /App port/ })).not.toBeNull();
});
it("reveals a collapsed preview option when the browser reports invalid input", () => {
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  const release = vi.spyOn(productApi, "release");
  render(<ProjectDeploy projectId="p-1" />);
  const details = screen.getByText("Preview build options", { selector: "summary" }).parentElement as HTMLDetailsElement;
  const port = screen.getByLabelText(/^App port/);
  fireEvent.change(port, { target: { value: 0 } });
  expect(details.open).toBe(false);
  fireEvent.invalid(port);
  expect(details.open).toBe(true);
  expect(release).not.toHaveBeenCalled();
});
it.each([gatewayURL, `https://p-${"b".repeat(51)}a.preview.projects.test/`])("opens an isolated verified Docker gateway preview with application origin permission: %s", async address => {
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [{ ...ready, url: address }],
  });
  const prepare = vi.spyOn(productApi, "prepareWorkspacePreview");
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Open preview" }));
  const frame = screen.getByTitle("Project preview");
  expect(frame.getAttribute("src")).toBe(address);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  expect(prepare).not.toHaveBeenCalled();
});

it.each([
  "http://" + "a".repeat(64) + ".localhost:8731/",
  "http://" + "a".repeat(31) + "." + "b".repeat(33) + ".localhost:8731/",
  "http://extra." + "a".repeat(32) + "." + "b".repeat(32) + ".localhost:8731/",
  "http://" + "a".repeat(63) + ".localhost:8731/",
  "http://" + "a".repeat(65) + ".localhost:8731/",
  "http://" + "g".repeat(64) + ".localhost:8731/",
  "http://" + "a".repeat(64) + ".localhost.example:8731/",
  "http://extra." + "a".repeat(64) + ".localhost:8731/",
  "http://localhost.example:8731/",
  "http://127.0.0.1.example:8731/",
  "https://" + "a".repeat(64) + ".localhost:8731/",
  "http://user:password@" + "a".repeat(64) + ".localhost:8731/",
  "http://:password@localhost:8731/",
])("keeps an invalid or credential-bearing local address out of the preview: %s", async (url) => {
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [{ ...ready, url }],
  });
  render(<ProjectDeploy projectId="p-1" />);
  await screen.findByText("Health check passed");
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
  expect(screen.queryByTitle("Project preview")).toBeNull();
});

it.each(["https://user:password@app.example/", "https://:password@app.example/"])(
  "does not link to a credential-bearing public deployment: %s",
  async (url) => {
    vi.spyOn(productApi, "releases").mockResolvedValue({
      releases: [{ ...ready, target: "github-pages", url }],
    });
    render(<ProjectDeploy projectId="p-1" />);
    await screen.findByText("Health check passed");
    expect(screen.queryByRole("link", { name: "Open deployed site" })).toBeNull();
    expect(screen.queryByTitle("Project preview")).toBeNull();
  },
);

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

it.each(["http://127.0.0.1:54411/", gatewayURL])("opens a cloud workspace preview through its prepared isolated address only after a click: %s", async (url) => {
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
    .mockResolvedValue({ url });
  render(<ProjectDeploy projectId="p-1" />);
  const button = await screen.findByRole("button", { name: "Open preview" });
  expect(prepare).not.toHaveBeenCalled();
  expect(screen.queryByTitle("Project preview")).toBeNull();
  await userEvent.click(button);
  expect(
    (await screen.findByTitle("Project preview")).getAttribute("src"),
  ).toBe(url);
  expect(screen.getByTitle("Project preview").getAttribute("sandbox")).toContain("allow-same-origin");
  expect(prepare).toHaveBeenCalledWith("p-1", "rel-1");
});

it.each([
  "https://unexpected.example",
  "http://" + "a".repeat(64) + ".localhost.example:8731/",
  "http://" + "a".repeat(63) + ".localhost:8731/",
  "http://user:password@" + "a".repeat(64) + ".localhost:8731/",
])("keeps an invalid remote preview address out of the embedded frame: %s", async (url) => {
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
    url,
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
