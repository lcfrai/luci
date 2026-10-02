import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PagesDeployForm } from "./PagesDeployForm";
import { ProjectDeploy } from "../lens/DeployLens";
import { productApi, type GitDelivery, type Release } from "../../api/product";

const commit = "a".repeat(40);
const delivery: GitDelivery = {
  origin: "https://github.com/example/static-demo.git",
  branch: "main",
  head: commit,
  publication: {
    repository: "https://github.com/example/static-demo.git",
    visibility: "public",
    status: "pushed",
    branch: "main",
    commit,
    verified: true,
    updatedAt: "2026-10-03T00:00:00Z",
  },
};
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue(delivery);
  vi.spyOn(productApi, "configuration").mockResolvedValue({
    revision: 0,
    variables: [],
    updatedAt: "",
  });
});
it("requires public acknowledgement and submits the exact pushed revision", async () => {
  const deploy = vi.fn();
  render(<PagesDeployForm projectId="p-1" busy="" onDeploy={deploy} />);
  await screen.findByText(delivery.origin!);
  const button = screen.getByRole("button", { name: "Deploy to GitHub Pages" });
  expect((button as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(
    screen.getByRole("checkbox", {
      name: "I understand that this site and its repository are public.",
    }),
  );
  await userEvent.click(button);
  expect(deploy).toHaveBeenCalledWith({
    commit,
    branch: "main",
    acknowledgePublic: true,
  });
  expect(
    screen.getByText(/Server code, databases and private runtime variables/),
  ).toBeTruthy();
});
it("does not submit an unpushed replacement commit", async () => {
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({
    ...delivery,
    head: "b".repeat(40),
  });
  const deploy = vi.fn();
  render(<PagesDeployForm projectId="p-1" busy="" onDeploy={deploy} />);
  await screen.findByText(/Push the reviewed commit/);
  await userEvent.click(screen.getByRole("checkbox"));
  expect(
    (
      screen.getByRole("button", {
        name: "Deploy to GitHub Pages",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(deploy).not.toHaveBeenCalled();
});
it("offers a deployed HTTPS site and unpublish, without unsupported rollback or an embedded preview", async () => {
  const ready: Release = {
    id: "pages-1",
    projectId: "p-1",
    briefRevision: 1,
    commit,
    target: "github-pages",
    status: "ready",
    url: "https://example.github.io/static-demo/",
    message: "Verified exact commit and HTTPS content",
    createdAt: "2026-10-03T00:00:00Z",
  };
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [ready] });
  const stop = vi
    .spyOn(productApi, "stopRelease")
    .mockResolvedValue({ ...ready, status: "stopped", url: "" });
  render(<ProjectDeploy projectId="p-1" />);
  const link = await screen.findByRole("link", { name: "Open deployed site" });
  expect(link.getAttribute("href")).toBe(ready.url);
  expect(screen.queryByRole("button", { name: "Open preview" })).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Roll back to this release" }),
  ).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Unpublish site" }));
  await waitFor(() => expect(stop).toHaveBeenCalledWith("p-1", "pages-1"));
});
it("keeps queued Pages releases unlinked until provider and HTTPS checks succeed", async () => {
  vi.spyOn(productApi, "releases").mockResolvedValue({
    releases: [
      {
        id: "pages-1",
        projectId: "p-1",
        briefRevision: 1,
        commit,
        target: "github-pages",
        status: "queued",
        url: "",
        message: "Waiting for verification",
        createdAt: "2026-10-03T00:00:00Z",
      },
    ],
  });
  render(<ProjectDeploy projectId="p-1" />);
  await screen.findByText("Waiting for verification");
  expect(screen.queryByRole("link", { name: "Open deployed site" })).toBeNull();
});
