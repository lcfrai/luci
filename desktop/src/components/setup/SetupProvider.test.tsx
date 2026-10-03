import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SetupProvider, useSetup } from "./SetupProvider";
import { productApi, type SetupStatus } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";

const status = {
  product: "Luci",
  version: "0.4",
  platform: "windows",
  configured: true,
  config: {
    projectsDir: "C:/projects",
    mode: "local",
    cloudProvider: "",
    region: "",
    domain: "",
    monthlyBudgetUsd: 0,
  },
  checks: [],
  cloudProviders: [],
} as SetupStatus;
function Home() {
  const setup = useSetup();
  return (
    <>
      <h1>Project home</h1>
      <button onClick={() => setup?.openProject()}>New project</button>
      <button onClick={() => setup?.openSettings()}>Settings</button>
    </>
  );
}
beforeEach(() => {
  resetCockpit();
  vi.restoreAllMocks();
});
it("renders first-run onboarding as a dedicated portal, then returns to projects without claiming dependencies are ready", async () => {
  vi.spyOn(productApi, "status").mockResolvedValue({
    ...status,
    configured: false,
  });
  render(
    <SetupProvider onProjectsChanged={vi.fn()}>
      <Home />
    </SetupProvider>,
  );
  await screen.findByRole("heading", { name: "Choose your workspace" });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByRole("heading", { name: "Project home" })).toBeNull();
  expect(
    screen.getByRole("button", { name: "On this computer" }),
  ).not.toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Continue to projects" }),
  );
  expect(screen.getByRole("heading", { name: "Project home" })).not.toBeNull();
});
it("creates a project outside Settings and selects it only after the server succeeds", async () => {
  vi.spyOn(productApi, "status").mockResolvedValue(status);
  const create = vi
    .spyOn(productApi, "createProject")
    .mockResolvedValue({
      id: "p-new",
      name: "build-it",
      path: "C:/projects/build-it",
    });
  const refresh = vi.fn();
  render(
    <SetupProvider onProjectsChanged={refresh}>
      <Home />
    </SetupProvider>,
  );
  await waitFor(() => expect(productApi.status).toHaveBeenCalledOnce());
  await userEvent.click(screen.getByRole("button", { name: "New project" }));
  expect(
    screen.getByRole("dialog", { name: "Create a project" }),
  ).not.toBeNull();
  expect(
    screen.queryByRole("navigation", { name: "Settings sections" }),
  ).toBeNull();
  await userEvent.type(
    screen.getByRole("textbox", { name: "Project name" }),
    "build-it",
  );
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  expect(create).toHaveBeenCalledWith("build-it", undefined);
  expect(useCockpit.getState().selection.projectId).toBe("p-new");
  expect(useCockpit.getState().lens).toBe("discover");
});
