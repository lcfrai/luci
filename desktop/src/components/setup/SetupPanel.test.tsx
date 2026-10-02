import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
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
  await userEvent.click(
    screen.getByRole("button", { name: "On this computer" }),
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
  await userEvent.click(
    screen.getByRole("button", { name: "Set up a cloud workspace" }),
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

it("shows initial connection progress before offering retry and keeps settings keyboard focus contained", async () => {
  let finish!: (value: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const fetcher = vi.fn<typeof fetch>().mockReturnValue(pending);
  render(
    <SetupPanel
      initialStatus={null}
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(screen.getByText("Loading your workspace settings…")).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Retry connection" })).toBeNull();
  await act(async () => {
    finish(new Response(JSON.stringify(status)));
    await pending;
  });
  await userEvent.click(
    await screen.findByRole("button", { name: "On this computer" }),
  );
  await screen.findByLabelText(/Projects folder/);
  await userEvent.click(screen.getByRole("button", { name: "Close settings" }));
  await userEvent.tab({ shift: true });
  expect(document.activeElement).toBe(
    screen.getByRole("button", { name: "Back to project" }),
  );
  await userEvent.tab();
  expect(document.activeElement).toBe(
    screen.getByRole("button", { name: "Close settings" }),
  );
});

it("holds the cloud-plan inputs and section in place until the displayed plan matches the submitted draft", async () => {
  let finish!: (value: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url) =>
      String(url).endsWith("/plan")
        ? pending
        : new Response(
            JSON.stringify({ jobs: [], targets: [], connections: [] }),
          ),
    );
  render(
    <SetupPanel
      initialStatus={{ ...status, configured: true }}
      initialSection="cloud"
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Set up a cloud workspace" }),
  );
  await userEvent.type(
    screen.getByLabelText(/SSH public key/),
    "ssh-ed25519 AAAATEST",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Preview cloud plan" }),
  );
  expect(
    (screen.getByLabelText("Provider") as HTMLSelectElement).disabled,
  ).toBe(true);
  expect((screen.getByLabelText("Domain") as HTMLInputElement).disabled).toBe(
    true,
  );
  expect(
    (screen.getByLabelText(/SSH public key/) as HTMLTextAreaElement).disabled,
  ).toBe(true);
  expect(
    (screen.getByRole("button", { name: "New project" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  expect(screen.getByText("Planning…")).not.toBeNull();
  await act(async () => {
    finish(
      new Response(
        JSON.stringify({
          id: "plan-1",
          provider: "aws",
          name: "luci",
          region: status.config.region,
          domain: status.config.domain,
          resources: [],
          estimate: {
            currency: "USD",
            monthly: null,
            verified: false,
            explanation: "Quote pending",
          },
          steps: [],
          files: [],
          commands: [],
          warnings: [],
          createdAt: "2026-10-02T00:00:00Z",
        }),
      ),
    );
    await pending;
  });
  await screen.findByText(
    "Plan generated. No cloud resources have been created.",
  );
  expect((screen.getByLabelText("Domain") as HTMLInputElement).disabled).toBe(
    false,
  );
  expect(
    (screen.getByRole("button", { name: "New project" }) as HTMLButtonElement)
      .disabled,
  ).toBe(false);
});

it("offers three clear first-run routes before loading cloud tools and allows backtracking without losing a local folder draft", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValue(
      new Response(JSON.stringify({ jobs: [], targets: [], connections: [] })),
    );
  render(
    <SetupPanel
      initialStatus={status}
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(
    screen.getByRole("button", { name: "On this computer" }),
  ).not.toBeNull();
  expect(
    screen.getByRole("button", { name: "Set up a cloud workspace" }),
  ).not.toBeNull();
  expect(
    screen.getByRole("button", { name: "Connect to an existing workspace" }),
  ).not.toBeNull();
  expect(screen.getByText(/Recommended · lowest ongoing cost/)).not.toBeNull();
  expect(screen.queryByLabelText("Provider")).toBeNull();
  expect(screen.queryByLabelText("Server address")).toBeNull();
  expect(fetcher).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "On this computer" }),
  );
  const folder = screen.getByRole("textbox", { name: /Projects folder/ });
  await userEvent.clear(folder);
  await userEvent.type(folder, "C:\\GardenProjects");
  await userEvent.click(
    screen.getByRole("button", { name: "Choose another setup option" }),
  );
  expect(document.activeElement).toBe(
    screen.getByRole("heading", { name: "Choose your workspace" }),
  );
  await userEvent.click(
    screen.getByRole("button", { name: "On this computer" }),
  );
  expect(
    (
      screen.getByRole("textbox", {
        name: /Projects folder/,
      }) as HTMLInputElement
    ).value,
  ).toBe("C:\\GardenProjects");
  expect(fetcher).not.toHaveBeenCalled();
});

it("recovers failed initial settings loading into the route choices without showing cloud fields prematurely", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockRejectedValueOnce(new Error("Local service unavailable"))
    .mockResolvedValueOnce(new Response(JSON.stringify(status)));
  render(
    <SetupPanel
      initialStatus={null}
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "not available",
  );
  expect(
    screen.queryByRole("button", { name: "Set up a cloud workspace" }),
  ).toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry connection" }),
  );
  await screen.findByRole("button", {
    name: "Connect to an existing workspace",
  });
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByLabelText("Provider")).toBeNull();
  expect(screen.queryByLabelText("SSH public key")).toBeNull();
});

it("connects an existing workspace through host verification and its account without cloud purchase or reinstall controls", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) => {
      const path = new URL(String(url)).pathname;
      if (path.endsWith("/ssh/inspect"))
        return new Response(
          JSON.stringify({
            id: "scan-1",
            host: "team.example",
            fingerprints: ["SHA256:verified"],
            createdAt: "now",
          }),
        );
      if (path.endsWith("/ssh/connect"))
        return new Response(
          JSON.stringify({
            id: "ssh-1",
            host: "team.example",
            localUrl: "http://127.0.0.1:55100",
            status: "connected",
            message: "Team connection ready",
          }),
        );
      if (path.endsWith("/workspace/auth"))
        return new Response(
          JSON.stringify({
            authenticated: init?.method === "POST",
            bootstrapRequired: false,
            message:
              init?.method === "POST"
                ? "Team account connected"
                : "Sign in to your team account",
          }),
        );
      return new Response(JSON.stringify({ connections: [] }));
    });
  render(
    <SetupPanel
      initialStatus={status}
      client={createProductClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Connect to an existing workspace" }),
  );
  expect(screen.queryByLabelText("Provider")).toBeNull();
  expect(screen.queryByLabelText("Server API token")).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Create cloud resources" }),
  ).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Install Luci workspace" }),
  ).toBeNull();
  await userEvent.type(screen.getByLabelText("Server address"), "team.example");
  await userEvent.click(
    screen.getByRole("button", { name: "Inspect server fingerprint" }),
  );
  await screen.findByLabelText("Server fingerprint");
  await userEvent.selectOptions(
    screen.getByLabelText("Server fingerprint"),
    "SHA256:verified",
  );
  await userEvent.click(screen.getByLabelText(/I compared this fingerprint/));
  await userEvent.click(
    screen.getByRole("button", { name: "Trust and connect" }),
  );
  await screen.findByText("Sign in to your team account");
  const activate = screen.getByRole("button", {
    name: "Use this cloud workspace",
  }) as HTMLButtonElement;
  expect(activate.disabled).toBe(true);
  await userEvent.type(screen.getByLabelText("Remote username"), "gardener");
  await userEvent.type(
    screen.getByLabelText("Remote password"),
    "team-private-password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Sign in to workspace" }),
  );
  await screen.findByText("Team account connected");
  expect(activate.disabled).toBe(false);
  expect(
    fetcher.mock.calls.some(([url]) =>
      /\/cloud\/(jobs|create|plan)|\/targets|install-luci/.test(String(url)),
    ),
  ).toBe(false);
});
