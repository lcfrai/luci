import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectEntry } from "./ProjectEntry";
import type { ProductClient } from "../../api/product";

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
  );
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
