import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectEntry } from "./ProjectEntry";
import { ProductApiError, type ProductClient } from "../../api/product";
import { loadPendingImport, projectImportsApi, savePendingImport, type PendingProjectImport } from "../../api/projectImports";

const identity = vi.hoisted(() => ({ scope: "account-a-workspace-a" as string | null }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: () => identity.scope }));
beforeEach(() => { localStorage.clear(); identity.scope = "account-a-workspace-a"; });
afterEach(() => vi.restoreAllMocks());

const repository = "https://github.com/example/existing.git";
const imported = { id: "imported", name: "existing", path: "/projects/existing" };
const original: PendingProjectImport = { id: `imp-${"a".repeat(24)}`, name: "existing", repository };
async function fillImport() {
  await userEvent.type(screen.getByRole("textbox", { name: "Project name" }), "existing");
  await userEvent.type(screen.getByRole("textbox", { name: /Repository URL/ }), repository);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(within(screen.getByRole("dialog")).getAllByRole("button", { name: "Import repository" }).find(button => button.closest("form"))!);
}

it.each(["new", "import"] as const)("explains an invalid %s folder name before any request and preserves the entered spelling", async mode => {
  const project = { id: "p-atlas", name: "Acceptance_Atlas", path: "/projects/Acceptance_Atlas" };
  const createProject = vi.fn().mockResolvedValue(project), created = vi.fn();
  render(<ProjectEntry mode={mode} client={{ createProject } as unknown as ProductClient} onCreated={created} onClose={() => {}} />);
  const name = screen.getByRole("textbox", { name: "Project name" });
  const form = name.closest("form")!;
  if (mode === "import") {
    await userEvent.type(screen.getByRole("textbox", { name: /Repository URL/ }), repository);
    await userEvent.click(screen.getByRole("checkbox"));
  }
  const submit = within(form).getByRole("button", { name: mode === "new" ? "Create project" : "Import repository" });
  for (const invalid of ["Acceptance Atlas", "CON", "a".repeat(65)]) {
    fireEvent.change(name, { target: { value: invalid } });
    expect(name).toHaveProperty("value", invalid);
    expect(name.getAttribute("aria-invalid")).toBe("true");
    expect(document.getElementById(name.getAttribute("aria-describedby")!)?.textContent).toContain(invalid === "CON" ? "reserved by Windows" : "No spaces");
    expect(submit).toHaveProperty("disabled", true);
    fireEvent.submit(form);
    expect(createProject).not.toHaveBeenCalled();
    expect(loadPendingImport(identity.scope)).toBeNull();
  }
  fireEvent.change(name, { target: { value: " Acceptance_Atlas " } });
  expect(name.getAttribute("aria-invalid")).toBe("false");
  expect(submit).toHaveProperty("disabled", false);
  await userEvent.click(submit);
  await waitFor(() => expect(created).toHaveBeenCalledWith(project));
  expect(createProject).toHaveBeenCalledTimes(1);
  expect(createProject.mock.calls[0][0]).toBe("Acceptance_Atlas");
});

it("keeps the project name after a failure and opens only the successfully created project", async () => {
  const project = { id: "p-new", name: "my-app", path: "/projects/my-app" };
  const createProject = vi
    .fn()
    .mockRejectedValueOnce(new Error("Git is not installed"))
    .mockResolvedValueOnce(project);
  const created = vi.fn(),
    close = vi.fn();
  render(
    <ProjectEntry
      client={{ createProject } as unknown as ProductClient}
      onCreated={created}
      onClose={close}
    />,
  );
  await userEvent.type(
    screen.getByRole("textbox", { name: "Project name" }),
    "my-app",
  );
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Git is not installed",
  );
  expect(
    (screen.getByRole("textbox", { name: "Project name" }) as HTMLInputElement)
      .value,
  ).toBe("my-app");
  expect(close).not.toHaveBeenCalled();
  expect(created).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  expect(created).toHaveBeenCalledWith(project);
});
it("requires a trusted HTTPS repository and forwards it through the real project import API", async () => {
  const createProject = vi
    .fn()
    .mockResolvedValue({
      id: "imported",
      name: "existing",
      path: "/projects/existing",
    });
  render(
    <ProjectEntry
      mode="import"
      client={{ createProject } as unknown as ProductClient}
      onCreated={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.type(
    screen.getByRole("textbox", { name: "Project name" }),
    "existing",
  );
  await userEvent.type(
    screen.getByRole("textbox", { name: /Repository URL/ }),
    "https://github.com/example/existing.git",
  );
  const submit = screen
    .getAllByRole("button", { name: "Import repository" })
    .find(
      (button) =>
        button.getAttribute("type") !== "button" && button.closest("form"),
    ) as HTMLButtonElement;
  expect(submit.disabled).toBe(true);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(submit);
  expect(createProject).toHaveBeenCalledWith(
    "existing",
    "https://github.com/example/existing.git",
    expect.stringMatching(/^imp-[a-f0-9]{24}$/),
  );
});

it("retains a failed import and retries its name with a fresh identity only on request", async () => {
  const createProject = vi.fn().mockImplementationOnce((name: string, _repository: string, id: string) => Promise.reject(new ProductApiError("internal", "raw Git output with a secret", 422, {
    error: "git_failed", attempt: { id, name, status: "failed", message: "The clone failed. Check the repository URL and Git credentials before retrying.", retainedPath: "/projects/.li/stage/r" },
  }))).mockResolvedValue(imported);
  const created = vi.fn();
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={created} onClose={vi.fn()} />);
  await fillImport();
  expect(await screen.findByText(/repository could not be imported/)).not.toBeNull();
  expect(screen.queryByText(/raw Git output/)).toBeNull();
  expect(createProject).toHaveBeenCalledTimes(1);
  expect(created).not.toHaveBeenCalled();
  const first = createProject.mock.calls[0][2];
  expect(loadPendingImport(identity.scope)?.id).toBe(first);
  await userEvent.click(screen.getByRole("button", { name: "Retry import" }));
  await waitFor(() => expect(created).toHaveBeenCalledWith(imported));
  expect(createProject.mock.calls[1].slice(0, 2)).toEqual(["existing", repository]);
  expect(createProject.mock.calls[1][2]).not.toBe(first);
  expect(loadPendingImport(identity.scope)).toBeNull();
});

it("checks a lost response after reload without cloning, then opens a completed import explicitly", async () => {
  const createProject = vi.fn().mockRejectedValue(new Error("raw secret from Git"));
  const client = { createProject } as unknown as ProductClient;
  const created = vi.fn();
  const first = render(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  await fillImport();
  expect((await screen.findByRole("alert")).textContent).toContain("result could not be confirmed");
  expect(screen.queryByText(/raw secret/)).toBeNull();
  const request = loadPendingImport(identity.scope)!;
  first.unmount();
  const status = vi.spyOn(projectImportsApi, "status").mockResolvedValue({ ...request, status: "running", message: "Running" });
  render(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  expect(await screen.findByText(/import is still running/)).not.toBeNull();
  expect(status).toHaveBeenCalledWith(request.id, client);
  expect(createProject).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "Retry import" })).toBeNull();
  status.mockResolvedValue({ ...request, status: "complete", message: "Ready", project: imported });
  await userEvent.click(screen.getByRole("button", { name: "Check import status" }));
  const open = await screen.findByRole("button", { name: "Open imported project" });
  expect(created).not.toHaveBeenCalled();
  await userEvent.click(open);
  expect(created).toHaveBeenCalledExactlyOnceWith(imported);
  expect(createProject).toHaveBeenCalledTimes(1);
  expect(loadPendingImport(identity.scope)).toBeNull();
});

it("keeps a missing receipt ambiguous and reuses its exact request only after explicit retry", async () => {
  savePendingImport(identity.scope, original);
  const status = vi.spyOn(projectImportsApi, "status").mockRejectedValue(new ProductApiError("project_not_found", "Not found", 404, {}));
  const createProject = vi.fn().mockResolvedValue(imported);
  const created = vi.fn();
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={created} onClose={vi.fn()} />);
  const retry = await screen.findByRole("button", { name: "Retry original request" });
  expect((screen.getByRole("alert")).textContent).toContain("may still be in transit");
  expect((retry as HTMLButtonElement).disabled).toBe(true);
  expect(createProject).not.toHaveBeenCalled();
  expect(status).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(retry);
  await waitFor(() => expect(created).toHaveBeenCalledWith(imported));
  expect(createProject).toHaveBeenCalledExactlyOnceWith(original.name, original.repository, original.id);
});

it("fences a late import completion when the account or workspace changes", async () => {
  let finish!: (value: typeof imported) => void;
  const createProject = vi.fn().mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const client = { createProject } as unknown as ProductClient;
  const created = vi.fn();
  const view = render(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  await fillImport();
  const firstScope = identity.scope;
  const request = loadPendingImport(firstScope)!;
  identity.scope = "account-b-workspace-b";
  view.rerender(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  expect((screen.getByRole("textbox", { name: "Project name" }) as HTMLInputElement).value).toBe("");
  expect(loadPendingImport(identity.scope)).toBeNull();
  await act(async () => finish(imported));
  expect(created).not.toHaveBeenCalled();
  expect(loadPendingImport(firstScope)).toEqual(request);
  expect(screen.queryByRole("button", { name: "Open imported project" })).toBeNull();
});

it("retains a request when its screen unmounts before success", async () => {
  let finish!: (value: typeof imported) => void;
  const createProject = vi.fn().mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const created = vi.fn();
  const view = render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={created} onClose={vi.fn()} />);
  await fillImport();
  const request = loadPendingImport(identity.scope);
  view.unmount();
  await act(async () => finish(imported));
  expect(created).not.toHaveBeenCalled();
  expect(loadPendingImport(identity.scope)).toEqual(request);
});

it("discards a delayed status response from a previous account", async () => {
  savePendingImport(identity.scope, original);
  let finish!: (value: { id: string; name: string; status: "complete"; message: string; project: typeof imported }) => void;
  vi.spyOn(projectImportsApi, "status").mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const created = vi.fn();
  const client = { createProject: vi.fn() } as unknown as ProductClient;
  const view = render(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  identity.scope = "account-b";
  view.rerender(<ProjectEntry mode="import" client={client} onCreated={created} onClose={vi.fn()} />);
  await act(async () => finish({ ...original, status: "complete", message: "Ready", project: imported }));
  expect(screen.queryByRole("button", { name: "Open imported project" })).toBeNull();
  expect(created).not.toHaveBeenCalled();
  expect(client.createProject).not.toHaveBeenCalled();
});

it("blocks an import before sending when its recovery identity cannot be scoped or persisted", async () => {
  identity.scope = null;
  const createProject = vi.fn();
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  await fillImport();
  expect((await screen.findByRole("alert")).textContent).toContain("account and workspace are known");
  expect(createProject).not.toHaveBeenCalled();
});

it("never stores or sends repository URL credentials", async () => {
  const createProject = vi.fn();
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  await userEvent.type(screen.getByRole("textbox", { name: "Project name" }), "existing");
  await userEvent.type(screen.getByRole("textbox", { name: /Repository URL/ }), "https://user:password@example.com/repo.git");
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getAllByRole("button", { name: "Import repository" }).find(button => button.closest("form"))!);
  expect((await screen.findByRole("alert")).textContent).toContain("without credentials");
  expect(createProject).not.toHaveBeenCalled();
  expect(loadPendingImport(identity.scope)).toBeNull();
});

it.each([
  [400, "invalid_name"], [400, "invalid_remote"], [409, "project_exists"], [409, "setup_required"],
])("allows editing after definitive pre-admission rejection %s %s", async (status, error) => {
  const createProject = vi.fn().mockRejectedValue(new ProductApiError("internal", "raw secret", status, { error }));
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  await fillImport();
  await screen.findByRole("alert");
  expect((screen.getByRole("textbox", { name: "Project name" }) as HTMLInputElement).disabled).toBe(false);
  expect(screen.queryByText(/raw secret/)).toBeNull();
  expect(loadPendingImport(identity.scope)).toBeNull();
  expect(createProject).toHaveBeenCalledTimes(1);
});

it("retains identity after a server error without an authoritative attempt", async () => {
  const createProject = vi.fn().mockRejectedValue(new ProductApiError("internal", "raw secret", 500, { error: "import_unavailable" }));
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  await fillImport();
  expect((await screen.findByRole("alert")).textContent).toContain("result could not be confirmed");
  expect(loadPendingImport(identity.scope)?.id).toBe(createProject.mock.calls[0][2]);
  expect(screen.queryByRole("button", { name: "Edit import details" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Retry original request" })).toBeNull();
  expect(createProject).toHaveBeenCalledTimes(1);
});

it("lets an interrupted import correct its details without deleting retained files or automatically retrying", async () => {
  savePendingImport(identity.scope, original);
  vi.spyOn(projectImportsApi, "status").mockResolvedValue({ ...original, status: "interrupted", message: "Interrupted", retainedPath: "/projects/.li/old/r" });
  const createProject = vi.fn().mockResolvedValue({ ...imported, name: "corrected" });
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "Edit import details" }));
  expect(createProject).not.toHaveBeenCalled();
  expect(loadPendingImport(identity.scope)).toBeNull();
  const name = screen.getByRole("textbox", { name: "Project name" });
  await userEvent.clear(name); await userEvent.type(name, "corrected");
  const url = screen.getByRole("textbox", { name: /Repository URL/ });
  await userEvent.clear(url); await userEvent.type(url, "https://example.com/correct.git");
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getAllByRole("button", { name: "Import repository" }).find(button => button.closest("form"))!);
  expect(createProject).toHaveBeenCalledTimes(1);
  expect(createProject.mock.calls[0].slice(0, 2)).toEqual(["corrected", "https://example.com/correct.git"]);
  expect(createProject.mock.calls[0][2]).not.toBe(original.id);
});

it("keeps new projects and backup restore independent from a saved import", async () => {
  savePendingImport(identity.scope, original);
  const status = vi.spyOn(projectImportsApi, "status").mockResolvedValue({ ...original, status: "running", message: "Running" });
  render(<ProjectEntry client={{ createProject: vi.fn() } as unknown as ProductClient} onCreated={vi.fn()} onClose={vi.fn()} />);
  expect((screen.getByRole("textbox", { name: "Project name" }) as HTMLInputElement).value).toBe("");
  expect(status).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Restore backup" }));
  expect(screen.getByLabelText("Luci backup ZIP")).not.toBeNull();
  expect(status).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Import repository" }));
  expect(await screen.findByText(/import is still running/)).not.toBeNull();
  expect(status).toHaveBeenCalledTimes(1);
});

it("shows the verified workspace-change remedy in import details without retrying or losing the request", async () => {
  savePendingImport(identity.scope, original);
  const message = "The projects folder changed during import. Restore the original projects folder in workspace settings before checking this attempt again.";
  const retainedPath = "/projects/.li/original/r";
  vi.spyOn(projectImportsApi, "status").mockResolvedValue({ ...original, status: "unconfirmed", message, retainedPath });
  const createProject = vi.fn(), created = vi.fn();
  render(<ProjectEntry mode="import" client={{ createProject } as unknown as ProductClient} onCreated={created} onClose={vi.fn()} />);
  await screen.findByText(/import outcome is not yet confirmed/);
  const details = screen.getByText("Import details").closest("details")!;
  expect(details.open).toBe(false);
  await userEvent.click(screen.getByText("Import details"));
  expect(details.open).toBe(true);
  expect(within(details).getByText(message)).not.toBeNull();
  expect(within(details).getByText(original.id)).not.toBeNull();
  expect(within(details).getByText(retainedPath)).not.toBeNull();
  expect(loadPendingImport(identity.scope)).toEqual(original);
  expect(createProject).not.toHaveBeenCalled(); expect(created).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "Retry import" })).toBeNull();
});
it("has a separate backup restore route without opening workspace Settings", async () => {
  render(
    <ProjectEntry
      client={{} as ProductClient}
      onCreated={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(screen.getByRole("button", { name: "Restore backup" }));
  expect(
    screen.getByRole("heading", { name: "Restore a project" }),
  ).not.toBeNull();
  expect(screen.getByLabelText("Luci backup ZIP")).not.toBeNull();
  expect(screen.queryByRole("textbox", { name: "Projects folder" })).toBeNull();
});
