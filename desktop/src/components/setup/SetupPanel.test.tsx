import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SetupPanel } from "./SetupPanel";
import { createProductClient, type SetupStatus } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";

// Dependency installation has its own consent, cached-check and retry outcome tests.
vi.mock("./DependencySetup", () => ({ DependencySetup: () => null }));

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

const memberCloud: SetupStatus = {
  ...status,
  workspace: {
    mode: "cloud",
    connectionId: "cloud-1",
    host: "team.example.invalid",
    ready: true,
  },
  access: {
    user: { username: "invited-member", role: "member" },
    manageBacking: false,
    projectsDirManaged: true,
  },
  config: { ...status.config, projectsDir: "/srv/luci/users/member/projects" },
};

it("takes an already selected cloud member straight to their assigned projects without choosing a new backing", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
    new Response(
      JSON.stringify({
        id: "member-project",
        name: "my-work",
        path: "/srv/luci/users/member/projects/my-work",
      }),
      { status: 201 },
    ),
  );
  const close = vi.fn(),
    refresh = vi.fn();
  render(
    <SetupPanel
      initialStatus={memberCloud}
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={refresh}
      onClose={close}
    />,
  );
  expect(
    screen.getByRole("heading", { name: "Your cloud workspace" }),
  ).not.toBeNull();
  expect(screen.getByText(/Signed in as invited-member/)).not.toBeNull();
  expect(
    screen.queryByRole("group", { name: "Workspace setup options" }),
  ).toBeNull();
  expect(screen.queryByRole("button", { name: "Local" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
  expect(
    (
      screen.getByRole("textbox", {
        name: /Projects folder/,
      }) as HTMLInputElement
    ).readOnly,
  ).toBe(true);
  expect(
    screen.getByRole("button", { name: "Return to this machine" }),
  ).not.toBeNull();
  await userEvent.click(
    screen.getByRole("button", {
      name: "Create your first project",
    }),
  );
  expect(
    screen.getByRole("heading", { name: "Create a cloud project" }),
  ).not.toBeNull();
  await userEvent.type(
    screen.getByRole("textbox", { name: "Project name" }),
    "my-work",
  );
  await userEvent.click(screen.getByRole("button", { name: "Create project" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(fetcher).toHaveBeenCalledOnce();
  expect(fetcher.mock.calls[0][0]).toBe(
    "http://localhost:8730/v1/product/projects",
  );
  expect(JSON.parse(fetcher.mock.calls[0][1]?.body as string)).toEqual({
    name: "my-work",
  });
  expect(refresh).toHaveBeenCalledOnce();
  expect(useCockpit.getState().selection.projectId).toBe("member-project");
});

it("uses remote member permission for runtime controls while retaining personal Git setup and local connection recovery", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    const path = new URL(String(url)).pathname;
    if (path.endsWith("/git/identity"))
      return new Response(
        JSON.stringify({ name: "Member", email: "member@example.invalid" }),
      );
    if (path.endsWith("/runtime/install"))
      return new Response(
        JSON.stringify({ status: "ready", message: "Runtime ready" }),
      );
    if (path.endsWith("/agents"))
      return new Response(JSON.stringify({ providers: [] }));
    if (path.endsWith("/ssh/connections"))
      return new Response(
        JSON.stringify({
          connections: [
            {
              id: "cloud-1",
              host: "team.example.invalid",
              status: "connected",
              localUrl: "http://127.0.0.1:8123",
              message: "Connected",
            },
          ],
        }),
      );
    if (path.endsWith("/workspace/auth"))
      return new Response(
        JSON.stringify({
          authenticated: true,
          bootstrapRequired: false,
          user: { username: "invited-member", role: "member" },
          message: "Connected",
        }),
      );
    throw new Error(`Unexpected request ${path}`);
  });
  render(
    <SetupPanel
      initialStatus={{ ...memberCloud, configured: true }}
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await waitFor(() =>
    expect(
      (
        screen.getByRole("button", {
          name: "Save Git identity",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(false),
  );
  expect(
    screen.queryByRole("button", { name: "Install agent runtime" }),
  ).toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Connection and account" }),
  );
  await screen.findByRole("button", { name: "Disconnect remote sign-in" });
  expect(
    screen.queryByRole("group", { name: "Workspace setup options" }),
  ).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Install Luci workspace" }),
  ).toBeNull();
  expect(
    screen.queryByRole("heading", {
      name: "Finish your deployment connection",
    }),
  ).toBeNull();
  expect(
    fetcher.mock.calls.some(([url]) =>
      /\/cloud\/|\/setup\/targets/.test(String(url)),
    ),
  ).toBe(false);
});

it("keeps assigned local administrator folders read-only without a disabled Save dead end", () => {
  render(
    <SetupPanel
      initialStatus={{
        ...status,
        configured: true,
        access: {
          user: { username: "second-admin", role: "admin" },
          manageBacking: true,
          projectsDirManaged: true,
        },
      }}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(
    (
      screen.getByRole("textbox", {
        name: /Projects folder/,
      }) as HTMLInputElement
    ).readOnly,
  ).toBe(true);
  expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
  expect(
    screen.queryByRole("button", {
      name: "Projects folder managed by administrator",
    }),
  ).toBeNull();
  expect(
    screen.getByRole("button", { name: /Create your first project/ }),
  ).not.toBeNull();
});

it("fails closed for remote resource controls when a cloud service does not supply active-account access", () => {
  render(
    <SetupPanel
      initialStatus={{ ...memberCloud, access: undefined }}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
  expect(
    (
      screen.getByRole("textbox", {
        name: /Projects folder/,
      }) as HTMLInputElement
    ).readOnly,
  ).toBe(true);
  expect(
    screen.getByRole("button", { name: "Connection and account" }),
  ).not.toBeNull();
});

it("retains deployment-target configuration for an authorized active cloud administrator without restarting backing onboarding", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    const path = new URL(String(url)).pathname;
    if (path.endsWith("/ssh/connections"))
      return new Response(JSON.stringify({ connections: [] }));
    if (path.endsWith("/setup/targets"))
      return new Response(JSON.stringify({ targets: [] }));
    throw new Error(`Unexpected request ${path}`);
  });
  render(
    <SetupPanel
      initialStatus={{
        ...memberCloud,
        configured: true,
        access: {
          user: { username: "remote-admin", role: "admin" },
          manageBacking: true,
          projectsDirManaged: true,
        },
      }}
      initialSection="cloud"
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(
    screen.getByRole("heading", { name: "Finish your deployment connection" }),
  ).not.toBeNull();
  expect(
    screen.getByRole("button", { name: "Verify and save connection" }),
  ).not.toBeNull();
  expect(
    screen.queryByRole("group", { name: "Workspace setup options" }),
  ).toBeNull();
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
});

it("keeps remote sign-in recovery reachable when the expired account cannot load deployment targets", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    const path = new URL(String(url)).pathname;
    if (path.endsWith("/ssh/connections"))
      return new Response(
        JSON.stringify({
          connections: [
            {
              id: "cloud-1",
              host: "team.example.invalid",
              status: "connected",
              localUrl: "http://127.0.0.1:8123",
              message: "Connected",
            },
          ],
        }),
      );
    if (path.endsWith("/setup/targets"))
      return new Response(
        JSON.stringify({
          error: "backing_sign_in_required",
          message: "Sign in to the workspace again.",
        }),
        { status: 409 },
      );
    if (path.endsWith("/workspace/auth"))
      return new Response(
        JSON.stringify({
          authenticated: false,
          bootstrapRequired: false,
          message: "Sign in to this workspace.",
        }),
      );
    throw new Error(`Unexpected request ${path}`);
  });
  render(
    <SetupPanel
      initialStatus={{
        ...memberCloud,
        configured: true,
        access: {
          user: { username: "remote-admin", role: "admin" },
          manageBacking: true,
          projectsDirManaged: true,
        },
      }}
      initialSection="cloud"
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await screen.findByRole("button", { name: "Sign in to workspace" });
  expect(screen.getByRole("alert").textContent).toContain(
    "Sign in to the workspace again.",
  );
  expect(
    screen.getByRole("textbox", { name: "Remote username" }),
  ).not.toBeNull();
});

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
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={changed}
      onProjectsChanged={refresh}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Local" }),
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
  expect(changed).toHaveBeenCalledTimes(2); // preference receipt, then configured folder
  expect(changed.mock.calls[0][0].setupChoice.mode).toBe('local');
  expect(screen.getByText("Docker is not running")).not.toBeNull();
});

it("preserves an unsuccessful new project and selects a successful project in Discover", async () => {
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
      client={setupClient("http://localhost:8730", fetcher)}
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
      initialStatus={{ ...status, configured: true }}
      initialSection="cloud"
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "AWS" }),
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
      client={setupClient("http://localhost:8730", fetcher)}
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
    await screen.findByRole("button", { name: "Local" }),
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
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "AWS" }),
  );
  await userEvent.type(
    screen.getByLabelText(/SSH public key/),
    "ssh-ed25519 AAAATEST",
  );
  const previewPlan = screen.getByRole("button", { name: "Preview cloud plan" }) as HTMLButtonElement;
  expect(previewPlan.type).toBe("submit");
  await userEvent.click(previewPlan);
  expect(previewPlan.getAttribute("aria-busy")).toBe("true");
  expect(previewPlan.disabled).toBe(true);
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
  expect(previewPlan.hasAttribute("aria-busy")).toBe(false);
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
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Local" }),
  ).not.toBeNull();
  expect(
    screen.getByRole("button", { name: "Bring your own cloud" }),
  ).not.toBeNull();
  expect(
    screen.getByRole("button", { name: "Connect to an existing workspace" }),
  ).not.toBeNull();
  expect(screen.getByText(/Recommended · no cloud hosting bill/)).not.toBeNull();
  expect(screen.queryByLabelText("Provider")).toBeNull();
  expect(screen.queryByLabelText("Server address")).toBeNull();
  expect(fetcher).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Local" }),
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
    screen.getByRole("button", { name: "Local" }),
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
      client={setupClient("http://localhost:8730", fetcher)}
      onStatus={vi.fn()}
      onProjectsChanged={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "not available",
  );
  expect(
    screen.queryByRole("button", { name: "AWS" }),
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
      client={setupClient("http://localhost:8730", fetcher)}
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

it("hands automatic preparation to the backend before leaving the portal and permits planning while the build runs", async () => {
  let finishCheck!: (value: Response) => void;
  let acceptInstall!: (value: Response) => void;
  const check = new Promise<Response>(resolve => { finishCheck = resolve; });
  const acceptance = new Promise<Response>(resolve => { acceptInstall = resolve; });
  const current: SetupStatus = { ...status, configured: true, checks: [
    { id: "docker", label: "Docker", status: "ready", message: "Docker running", action: "" },
    { id: "docker-backing-storage", label: "Docker backing storage", status: "ready", message: "Confirmed host and guest capacity is sufficient for preparation.", action: "" },
    { id: "sandbox-image", label: "Agent runtime", status: "missing", message: "Image not built", action: "" },
  ] };
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
    const route = new URL(String(url)).pathname;
    if (route.endsWith("/setup/check")) return check;
    if (route.endsWith("/runtime/install"))
      return init?.method === "POST" ? acceptance : new Response(JSON.stringify({ status: "idle" }));
    if (route.endsWith("/git/identity")) return new Response(JSON.stringify({ name: "Owner", email: "owner@example.invalid" }));
    throw new Error("Unexpected portal request");
  });
  const close = vi.fn(), openProject = vi.fn();
  const { unmount } = render(<SetupPanel presentation="portal" initialStatus={current} client={setupClient("http://localhost:8730", fetcher)} onStatus={vi.fn()} onProjectsChanged={vi.fn()} onClose={close} onOpenProject={openProject} />);
  await userEvent.click(screen.getByRole("button", { name: "Local" }));
  const proceed = screen.getByRole("button", { name: "Continue to projects" }) as HTMLButtonElement;
  expect(proceed.disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Choose another setup option" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Connect a coding agent" }) as HTMLButtonElement).disabled).toBe(true);
  const create = screen.getByRole("button", { name: /Create your first project/ }) as HTMLButtonElement;
  expect(create.disabled).toBe(true);
  await userEvent.click(proceed);
  await userEvent.click(create);
  expect(close).not.toHaveBeenCalled();
  expect(openProject).not.toHaveBeenCalled();
  await act(async () => { finishCheck(new Response(JSON.stringify(current))); await check; });
  await waitFor(() => expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith("/runtime/install") && init?.method === "POST")).toHaveLength(1));
  expect(proceed.disabled).toBe(true);
  await act(async () => { acceptInstall(new Response(JSON.stringify({ status: "installing", message: "Backend build continues" }), { status: 202 })); await acceptance; });
  await screen.findByText(/Installing runtime… Backend build continues/);
  await waitFor(() => expect(proceed.disabled).toBe(false));
  expect(create.disabled).toBe(false);
  await userEvent.click(proceed);
  expect(close).toHaveBeenCalledOnce();
  unmount();
  expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith("/runtime/install") && init?.method === "POST")).toHaveLength(1);
});

it("allows the portal to continue after a real unavailable-Docker check without starting a runtime", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    const route = new URL(String(url)).pathname;
    if (route.endsWith("/setup/check")) return new Response(JSON.stringify({ ...status, configured: true }));
    if (route.endsWith("/runtime/install")) return new Response(JSON.stringify({ status: "idle" }));
    if (route.endsWith("/git/identity")) return new Response(JSON.stringify({ name: "Owner", email: "owner@example.invalid" }));
    throw new Error("Unexpected portal request");
  });
  const close = vi.fn();
  render(<SetupPanel presentation="portal" initialStatus={{ ...status, configured: true }} client={setupClient("http://localhost:8730", fetcher)} onStatus={vi.fn()} onProjectsChanged={vi.fn()} onClose={close} />);
  await userEvent.click(screen.getByRole("button", { name: "Local" }));
  await screen.findByText(/Open Docker Desktop and wait for its engine to start/);
  const proceed = screen.getByRole("button", { name: "Continue to projects" }) as HTMLButtonElement;
  await waitFor(() => expect(proceed.disabled).toBe(false));
  await userEvent.click(proceed);
  expect(close).toHaveBeenCalledOnce();
  expect(fetcher.mock.calls.some(([url, init]) => String(url).endsWith("/runtime/install") && init?.method === "POST")).toBe(false);
});

// Mode persistence has dedicated network/CAS tests. These older regressions
// isolate storage, project and legacy deployment controls behind a valid receipt.
function setupClient(baseUrl: string, fetcher: typeof fetch) {
  return createProductClient(baseUrl, (input, init) => {
    if (new URL(String(input)).pathname === '/v1/setup/choice') {
      const request = JSON.parse(String(init?.body));
      return Promise.resolve(new Response(JSON.stringify({revision: request.expectedRevision + 1, mode: request.mode})));
    }
    return fetcher(input, init);
  });
}
