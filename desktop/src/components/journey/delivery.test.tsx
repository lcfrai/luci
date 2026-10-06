import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { productApi, type GitState } from "../../api/product";
import { ProjectDelivery } from "./ProjectDelivery";
import { ProjectRestore } from "./ProjectBackup";
import { CloudDeployForm } from "./CloudDeployForm";
import { resetCockpit, useCockpit } from "../../store/store";
import { aProject } from "../../test/fixtures";
const head = "1234567890abcdef";
const git: GitState = {
  branch: "main",
  clean: true,
  changes: [],
  diff: "",
  log: [{ hash: head, subject: "Verified work" }],
};
beforeEach(() => {
  vi.restoreAllMocks();
  resetCockpit();
  useCockpit.setState({ projects: [aProject({ id: "p-1", name: "journal" })] });
});

it("creates a private repository separately from pushing the exact reviewed commit", async () => {
  vi.spyOn(productApi, "gitDelivery")
    .mockResolvedValueOnce({ branch: "main", head })
    .mockResolvedValue({
      origin: "https://github.com/owner/journal.git",
      branch: "main",
      head,
    });
  const create = vi
    .spyOn(productApi, "githubCreate")
    .mockResolvedValue({
      repository: "https://github.com/owner/journal.git",
      visibility: "private",
      status: "connected",
      verified: false,
      updatedAt: "",
    });
  const push = vi
    .spyOn(productApi, "gitPush")
    .mockResolvedValue({
      repository: "https://github.com/owner/journal.git",
      status: "pushed",
      commit: head,
      verified: true,
      updatedAt: "",
    });
  render(<ProjectDelivery projectId="p-1" git={git} />);
  await userEvent.click(screen.getByText("Create a GitHub repository"));
  expect(
    (screen.getByLabelText("Repository visibility") as HTMLSelectElement).value,
  ).toBe("private");
  await userEvent.click(
    screen.getByRole("button", { name: "Create repository" }),
  );
  await waitFor(() =>
    expect(create).toHaveBeenCalledWith("p-1", {
      name: "journal",
      visibility: "private",
      acknowledgePublic: false,
    }),
  );
  expect(push).not.toHaveBeenCalled();
  await screen.findByText(
    /repository connected. Push your reviewed commit next/,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Push reviewed commit" }),
  );
  expect(
    await screen.findByText("Remote commit verified: 12345678."),
  ).not.toBeNull();
  expect(push).toHaveBeenCalledWith("p-1", head, "main");
});
it("requires an explicit public visibility acknowledgment and blocks dirty pushes", async () => {
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({
    branch: "main",
    head,
  });
  const create = vi.spyOn(productApi, "githubCreate");
  render(<ProjectDelivery projectId="p-1" git={{ ...git, clean: false }} />);
  await userEvent.click(screen.getByText("Create a GitHub repository"));
  await userEvent.selectOptions(
    screen.getByLabelText("Repository visibility"),
    "public",
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Create repository",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(create).not.toHaveBeenCalled();
  expect(
    (
      screen.getByRole("button", {
        name: "Push reviewed commit",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
});
it("uploads a trusted ZIP unchanged and preserves the restore form when its new name already exists", async () => {
  const restore = vi
    .spyOn(productApi, "restoreProject")
    .mockRejectedValue(new Error("Project already exists"));
  const restored = vi.fn();
  render(<ProjectRestore client={productApi} onRestored={restored} />);
  const zip = new File([new Uint8Array([80, 75, 3, 4])], "backup.zip", {
    type: "application/zip",
  });
  await userEvent.upload(screen.getByLabelText("Luci backup ZIP"), zip);
  await userEvent.type(
    screen.getByLabelText("Restored project name"),
    "restored-journal",
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Restore as new project",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.click(screen.getByRole("checkbox"));
  expect(
    (
      screen.getByRole("button", {
        name: "Restore as new project",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(false);
  // happy-dom does not reconcile FileList with required-file form validity. Submit the trusted form directly.
  fireEvent.submit(
    screen
      .getByRole("button", { name: "Restore as new project" })
      .closest("form")!,
  );
  await screen.findByText("Project already exists");
  expect(restore).toHaveBeenCalledWith("restored-journal", zip);
  expect(restored).not.toHaveBeenCalled();
  expect(
    (screen.getByLabelText("Restored project name") as HTMLInputElement).value,
  ).toBe("restored-journal");
});
it("prepares read-only access before deploying a private repository and carries its reviewed commit", async () => {
  vi.spyOn(productApi, "targets").mockResolvedValue({
    targets: [
      {
        id: "target-1",
        name: "Server",
        serverUuid: "srv-1",
        baseUrl: "https://control.example",
      },
    ],
  });
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({
    origin: "https://github.com/owner/journal.git",
    branch: "main",
    head,
    publication: {
      repository: "https://github.com/owner/journal.git",
      visibility: "private",
      status: "pushed",
      verified: true,
      updatedAt: "",
    },
  });
  const key = vi
    .spyOn(productApi, "prepareDeployKey")
    .mockResolvedValue({
      status: "ready",
      privateKeyUuid: "owned-key",
      message: "Read-only access ready",
    });
  const deploy = vi.fn();
  render(
    <CloudDeployForm
      projectId="p-1"
      busy=""
      onSetup={vi.fn()}
      onDeploy={deploy}
    />,
  );
  await screen.findByRole("option", { name: "Server" });
  await userEvent.selectOptions(
    screen.getByLabelText("Deployment server"),
    "target-1",
  );
  await userEvent.type(
    screen.getByLabelText("Application domain"),
    "journal.example.com",
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Deploy to cloud",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.click(
    screen.getByRole("button", { name: "Prepare read-only GitHub access" }),
  );
  await screen.findByText("Read-only access ready");
  expect(key).toHaveBeenCalledWith(
    "p-1",
    "target-1",
    "https://github.com/owner/journal.git",
  );
  expect(deploy).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Deploy to cloud" }),
  );
  expect(deploy).toHaveBeenCalledWith(
    expect.objectContaining({
      source: "private",
      privateKeyUuid: "owned-key",
      commit: head,
      branch: "main",
      targetId: "target-1",
    }),
  );
});
it.each(["none", "vite", "nextjs"] as const)("carries the explicit %s browser data choice into the reviewed cloud release", async (framework) => {
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [{ id: "target-1", name: "Server", serverUuid: "srv-1", baseUrl: "https://control.example" }] });
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ origin: "https://github.com/owner/journal.git", branch: "main", head });
  const deploy = vi.fn();
  render(<CloudDeployForm projectId="p-1" busy="" onSetup={vi.fn()} onDeploy={deploy} />);
  await screen.findByRole("option", { name: "Server" });
  fireEvent.change(screen.getByLabelText("Deployment server"), { target: { value: "target-1" } });
  fireEvent.change(screen.getByLabelText("Application domain"), { target: { value: "journal.example.com" } });
  fireEvent.change(screen.getByLabelText("Repository access"), { target: { value: "public" } });
  fireEvent.change(screen.getByLabelText("Browser data variables"), { target: { value: framework } });
  fireEvent.submit(screen.getByRole("button", { name: "Deploy to cloud" }).closest("form")!);
  expect(deploy).toHaveBeenCalledTimes(1);
  const input = deploy.mock.calls[0][0];
  expect(input.commit).toBe(head);
  expect(input.dataClientFramework).toBe(framework === "none" ? undefined : framework);
  expect(input.privateKeyUuid).toBeUndefined();
});
it("does not submit another deployment while a release is running", async () => {
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [{ id: "target-1", name: "Server", serverUuid: "srv-1", baseUrl: "https://control.example" }] });
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ origin: "https://github.com/owner/journal.git", branch: "main", head });
  const deploy = vi.fn();
  const props = { projectId: "p-1", onSetup: vi.fn(), onDeploy: deploy };
  const view = render(<CloudDeployForm {...props} busy="" />);
  await screen.findByRole("option", { name: "Server" });
  fireEvent.change(screen.getByLabelText("Deployment server"), { target: { value: "target-1" } });
  fireEvent.change(screen.getByLabelText("Repository access"), { target: { value: "public" } });
  view.rerender(<CloudDeployForm {...props} busy="Deploying…" />);
  expect((screen.getByLabelText("Application domain") as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByLabelText("Cloud app port") as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByLabelText("Health check path") as HTMLInputElement).disabled).toBe(true);
  fireEvent.submit(screen.getByRole("button", { name: "Deploying…" }).closest("form")!);
  expect(deploy).not.toHaveBeenCalled();
});

it("shows server loading separately from a verified empty account and preserves member setup permissions", async () => {
  let finish!: (value: Awaited<ReturnType<typeof productApi.targets>>) => void;
  vi.spyOn(productApi, "targets").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ branch: "main", head });
  const setup = vi.fn(), deploy = vi.fn();
  render(<CloudDeployForm projectId="p-1" busy="" canManageBacking={false} onSetup={setup} onDeploy={deploy} />);
  expect(screen.getByText(/Checking your connected deployment servers/)).not.toBeNull();
  expect(screen.queryByText(/No deployment server connected yet/)).toBeNull();
  await act(async () => finish({ targets: [] }));
  expect(screen.getByText(/Ask your administrator to connect/)).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Set up your cloud" })).toBeNull();
  expect(setup).not.toHaveBeenCalled();
  expect(deploy).not.toHaveBeenCalled();
});
it("reveals a collapsed cloud option for browser validation without starting a deployment", async () => {
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [] });
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ branch: "main", head });
  const deploy = vi.fn();
  render(<CloudDeployForm projectId="p-1" busy="" onSetup={vi.fn()} onDeploy={deploy} />);
  await screen.findByText(/No deployment server connected yet/);
  const details = screen.getByText("Build and health options", { selector: "summary" }).parentElement as HTMLDetailsElement;
  const port = screen.getByLabelText("Cloud app port");
  fireEvent.change(port, { target: { value: 0 } });
  fireEvent.invalid(port);
  expect(details.open).toBe(true);
  expect(deploy).not.toHaveBeenCalled();
});

it("keeps private repository credential preparation separate from deployment permission", async () => {
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [{ id: "target-1", name: "Server", serverUuid: "srv-1", baseUrl: "https://control.example" }] });
  vi.spyOn(productApi, "git").mockResolvedValue(git);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ origin: "https://github.com/owner/journal.git", branch: "main", head });
  const key = vi.spyOn(productApi, "prepareDeployKey");
  const deploy = vi.fn();
  render(<CloudDeployForm projectId="p-1" busy="" canPrepareAccess={false} onSetup={vi.fn()} onDeploy={deploy} />);
  await screen.findByRole("option", { name: "Server" });
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "Deployment server" }), "target-1");
  const prepare = screen.getByRole("button", { name: "Prepare read-only GitHub access" });
  expect((prepare as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(prepare);
  expect(key).not.toHaveBeenCalled();
  expect(screen.getByText(/Ask a team owner for secrets access/)).not.toBeNull();
  expect(deploy).not.toHaveBeenCalled();
});
