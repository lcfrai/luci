import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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
