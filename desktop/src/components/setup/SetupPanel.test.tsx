import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SetupPanel } from "./SetupPanel";
import { createProductClient, type SetupStatus } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";

const status: SetupStatus = {
  product: "Luci",
  version: "0.3.0",
  platform: "windows",
  configured: false,
  config: {
    projectsDir: "C:\\Projects",
    mode: "local",
    cloudProvider: "",
    region: "ap-southeast-2",
    domain: "luci.example.com",
    monthlyBudgetUsd: 30,
  },
  checks: [
    {
      id: "docker",
      label: "Docker",
      status: "missing",
      message: "Docker is not running",
      action: "Start Docker Desktop",
    },
  ],
  cloudProviders: [
    {
      id: "aws",
      name: "AWS",
      strategy: "Lightsail",
      description: "One Linux machine",
    },
  ],
};
beforeEach(() => resetCockpit());

it("saves local configuration, shows the server result and refreshes the project catalogue", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
    new Response(JSON.stringify({ ...status, configured: true }), {
      status: 200,
    }),
  );
  const refresh = vi.fn(),
    changed = vi.fn();
  render(
    <SetupPanel
      initialStatus={status}
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={changed}
      onProjectsChanged={refresh}
      onClose={vi.fn()}
    />,
  );
  const folder = screen.getByRole("textbox", { name: /Projects folder/ });
  await userEvent.clear(folder);
  await userEvent.type(folder, "C:\\MyProjects");
  await userEvent.click(screen.getByRole("button", { name: "Save settings" }));
  await screen.findByText("Settings saved. Your projects folder is ready.");
  expect(fetcher.mock.calls[0][0]).toBe(
    "http://localhost:8730/v1/setup/config",
  );
  expect(JSON.parse(fetcher.mock.calls[0][1]?.body as string).projectsDir).toBe(
    "C:\\MyProjects",
  );
  expect(refresh).toHaveBeenCalledOnce();
  expect(changed).toHaveBeenCalledOnce();
  expect(screen.getByText("Docker is not running")).not.toBeNull();
});

it("preserves an unsuccessful new project and selects a successful project in Define", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: "conflict",
          message: "That project already exists.",
        }),
        { status: 409 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: "p-new",
          name: "garden",
          path: "C:\\Projects\\garden",
        }),
        { status: 201 },
      ),
    );
  const close = vi.fn(),
    refresh = vi.fn();
  render(
    <SetupPanel
      initialStatus={status}
      initialSection="project"
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={refresh}
      onClose={close}
    />,
  );
  await userEvent.type(
    screen.getByRole("textbox", { name: "Project name" }),
    "garden",
  );
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "already exists",
  );
  expect(close).not.toHaveBeenCalled();
  expect(refresh).not.toHaveBeenCalled();
  expect(
    (screen.getByRole("textbox", { name: "Project name" }) as HTMLInputElement)
      .value,
  ).toBe("garden");
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(useCockpit.getState().selection.projectId).toBe("p-new");
  expect(useCockpit.getState().lens).toBe("discover");
});

it("labels an unverified cloud plan honestly and never creates resources while planning", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) =>
    String(url).endsWith("/plan")
      ? new Response(
          JSON.stringify({
            id: "cp-1",
            provider: "aws",
            name: "luci",
            region: "ap-southeast-2",
            domain: "luci.example.com",
            resources: ["One virtual machine"],
            estimate: {
              currency: "USD",
              monthly: null,
              verified: false,
              explanation: "Check provider pricing before creating resources",
            },
            steps: [],
            files: [],
            commands: [],
            warnings: [],
            createdAt: "2026-10-02T00:00:00Z",
          }),
        )
      : new Response(
          JSON.stringify({ jobs: [], targets: [], connections: [] }),
        ),
  );
  render(
    <SetupPanel
      initialStatus={status}
      initialSection="cloud"
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.type(
    screen.getByRole("textbox", { name: /SSH public key/ }),
    "ssh-ed25519 AAAATEST",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Preview cloud plan" }),
  );
  expect(await screen.findByText("Monthly cost not verified")).not.toBeNull();
  expect(
    screen.getByText("Plan generated. No cloud resources have been created."),
  ).not.toBeNull();
  expect(
    fetcher.mock.calls
      .filter((call) => call[1]?.method === "POST")
      .map((call) => call[0]),
  ).toEqual(["http://localhost:8730/v1/setup/cloud/plan"]);
});
