import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SetupProvider, useSetup } from "./SetupProvider";
import { productApi, type SetupStatus } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import { completeOnboardingVisit, onboardingKey } from "../../preferences/onboarding";
import { aProject } from "../../test/fixtures";

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
  localStorage.clear();
  resetCockpit();
  useCockpit.getState().setProjects([]);
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
  for (const name of ["Local", "Bring your own cloud", "Luci Cloud"])
    expect(screen.getByRole("button", { name })).not.toBeNull();
  expect(screen.getByRole("button", { name: "Connect to an existing workspace" })).not.toBeNull();
  expect(screen.getByText(/Azure first · cloud and model charges are separate/)).not.toBeNull();
  expect(screen.getByText(/Not available yet · choose Local or Bring your own cloud/)).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Prepare Azure instructions" })).toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Continue to projects" }),
  );
  expect(screen.getByRole("heading", { name: "Project home" })).not.toBeNull();
});
it("creates a project outside Settings and selects it only after the server succeeds", async () => {
  completeOnboardingVisit(onboardingKey(status));
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
  expect(create).toHaveBeenCalledExactlyOnceWith("build-it");
  expect(useCockpit.getState().selection.projectId).toBe("p-new");
  expect(useCockpit.getState().lens).toBe("discover");
});

it("shows the location chooser for a new account with an assigned, configured folder", async () => {
  vi.spyOn(productApi, "status").mockResolvedValue({ ...status,
    access: { user: { username: "admin", role: "admin" }, manageBacking: true, projectsDirManaged: true },
  });
  render(<SetupProvider onProjectsChanged={vi.fn()}><Home /></SetupProvider>);
  await screen.findByRole("heading", { name: "Choose your workspace" });
  for (const name of ["Local", "Bring your own cloud", "Luci Cloud", "Connect to an existing workspace"])
    expect(screen.getByRole("button", { name })).not.toBeNull();
  expect(screen.getByText(/Choosing a mode saves a preference/)).not.toBeNull();
  expect(screen.getByText(/Not available yet · choose Local or Bring your own cloud/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Continue to projects" }));
  expect(screen.getByRole("heading", { name: "Project home" })).not.toBeNull();
  expect(useCockpit.getState().projects).toEqual([]);
});

it("remembers a completed visit without changing workspace readiness", async () => {
  const snapshot = { ...status, configured: false };
  vi.spyOn(productApi, "status").mockResolvedValue(snapshot);
  const first = render(<SetupProvider onProjectsChanged={vi.fn()}><Home /></SetupProvider>);
  await screen.findByRole("heading", { name: "Choose your workspace" });
  await userEvent.click(screen.getByRole("button", { name: "Continue to projects" }));
  first.unmount();
  render(<SetupProvider onProjectsChanged={vi.fn()}><Home /></SetupProvider>);
  await screen.findByRole("heading", { name: "Project home" });
  expect(screen.queryByRole("heading", { name: "Choose your workspace" })).toBeNull();
  expect(snapshot.configured).toBe(false);
});

it("keeps a returning account in its existing project when no visit preference exists", async () => {
  const project = aProject({ id: "p-existing" });
  useCockpit.getState().setProjects([project]);
  useCockpit.getState().selectProject(project.id);
  vi.spyOn(productApi, "status").mockResolvedValue(status);
  render(<SetupProvider onProjectsChanged={vi.fn()}><Home /></SetupProvider>);
  await screen.findByRole("heading", { name: "Project home" });
  expect(screen.queryByRole("heading", { name: "Choose your workspace" })).toBeNull();
  expect(useCockpit.getState().selection.projectId).toBe(project.id);
});
