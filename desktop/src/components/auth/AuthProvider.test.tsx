import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, type AuthProfile } from "./AuthProvider";
import { RailFooter } from "../rail/RailFooter";
import { productApi, type SetupStatus } from "../../api/product";
import { authToken, setAuthToken } from "../../api/authSession";
const native = vi.hoisted(() => ({ invoke: vi.fn() }));
vi.mock("@tauri-apps/api/core", () => native);
const member: AuthProfile = {
  user: {
    id: "u-1",
    username: "member",
    displayName: "A member",
    role: "member",
    disabled: false,
  },
  permissions: {
    manageUsers: false,
    manageBacking: false,
    managePolicy: false,
    inspectAudit: false,
    runHostCommands: false,
  },
  policy: {
    allowHostCommands: false,
    allowCloudProvisioning: false,
    allowPublicPublishing: false,
    maxSessionsPerUser: 2,
    maxConcurrentChecksPerUser: 1,
    monthlyBudgetUsd: 30,
    allowedHarnesses: ["shell"],
    backingMode: "docker",
    instanceName: "My workspace",
  },
};
beforeEach(() => {
  native.invoke.mockReset();
  vi.spyOn(productApi, "status").mockResolvedValue({
    workspace: { mode: "local", connectionId: "", host: "", ready: true },
  } as SetupStatus);
});
afterEach(() => {
  Reflect.deleteProperty(window, "__TAURI_INTERNALS__");
  Reflect.deleteProperty(window, "__LUCI_API__");
  setAuthToken(null);
  window.history.replaceState(null, "", "/");
  vi.restoreAllMocks();
});

it("mounts project content only after sign-in and gives members account controls without Admin", async () => {
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      if (path.endsWith("/status"))
        return {
          configured: true,
          bootstrapRequired: false,
          instanceName: "My workspace",
          backing: "docker",
        };
      if (path.endsWith("/login"))
        return { token: "test-token", user: member.user };
      if (path.endsWith("/me")) return member;
      return {};
    });
  render(
    <AuthProvider>
      <p>Project content</p>
      <RailFooter />
    </AuthProvider>,
  );
  await screen.findByRole("button", { name: "Sign in" });
  expect(screen.queryByText("Project content")).toBeNull();
  await userEvent.type(screen.getByLabelText("Username"), "member");
  await userEvent.type(screen.getByLabelText("Password"), "secret-for-test");
  await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
  await screen.findByText("Project content");
  expect(authToken()).toBe("test-token");
  expect(screen.queryByRole("button", { name: "Admin" })).toBeNull();
  expect(
    request.mock.calls.some(([path]) => path.startsWith("/v1/admin")),
  ).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "A member" }));
  await userEvent.click(screen.getByRole("button", { name: "Sign out" }));
  await waitFor(() => expect(screen.queryByText("Project content")).toBeNull());
  expect(authToken()).toBeNull();
});

it("autofills a portable one-time token from the fragment, removes it immediately, and never submits it before account creation", async () => {
  const token = "b".repeat(43);
  window.history.replaceState(null, "", `/#bootstrap=${token}`);
  const request = vi.spyOn(productApi, "request").mockResolvedValue({
    configured: false,
    bootstrapRequired: true,
    instanceName: "Luci",
    backing: "docker",
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  const input = (await screen.findByLabelText(
    /^Bootstrap token/,
  )) as HTMLInputElement;
  expect(input.value).toBe(token);
  expect(window.location.hash).toBe("");
  expect(
    request.mock.calls.every(([path]) => path === "/v1/control-plane/status"),
  ).toBe(true);
  expect(screen.queryByText("Private projects")).toBeNull();
});
it("clears portable bootstrap fragments without using them for an already configured workspace", async () => {
  window.history.replaceState(null, "", `/#bootstrap=${"a".repeat(43)}`);
  vi.spyOn(productApi, "request").mockResolvedValue({
    configured: true,
    bootstrapRequired: false,
    instanceName: "Luci",
    backing: "docker",
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  await screen.findByRole("button", { name: "Sign in" });
  expect(window.location.hash).toBe("");
  expect(screen.queryByLabelText(/^Bootstrap token/)).toBeNull();
});
it("does not accept an invalid bootstrap fragment", async () => {
  window.history.replaceState(null, "", "/#bootstrap=not-a-token");
  vi.spyOn(productApi, "request").mockResolvedValue({
    configured: false,
    bootstrapRequired: true,
    instanceName: "Luci",
    backing: "docker",
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  expect(
    ((await screen.findByLabelText(/^Bootstrap token/)) as HTMLInputElement)
      .value,
  ).toBe("");
  expect(window.location.hash).toBe("");
});
it("lets Super Admins save operating policy and inspect audit without claiming the budget caps provider billing", async () => {
  setAuthToken("admin-test");
  const owner: AuthProfile = {
    ...member,
    user: { ...member.user, role: "super-admin" },
    policy: {
      ...member.policy,
      sessionMemoryMiB: 1024,
      sessionCPUMillis: 1000,
      checkMemoryMiB: 1024,
      checkCPUMillis: 1000,
    },
    permissions: {
      manageUsers: true,
      manageBacking: true,
      managePolicy: true,
      inspectAudit: true,
      runHostCommands: true,
    },
  };
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path, body, method) => {
      if (path.endsWith("/status"))
        return {
          configured: true,
          bootstrapRequired: false,
          instanceName: "Luci",
          backing: "docker",
        };
      if (path.endsWith("/me")) return owner;
      if (path.endsWith("/users")) return { users: [owner.user] };
      if (path.endsWith("/audit"))
        return {
          events: [
            {
              id: "a1",
              at: "2026-10-02T00:00:00Z",
              actorId: "u-1",
              action: "auth.login",
              target: "u-1",
              outcome: "success",
            },
          ],
        };
      if (path.endsWith("/policy") && method === "PATCH")
        return { ...owner.policy, ...(body as object) };
      throw new Error(path);
    });
  render(
    <AuthProvider>
      <RailFooter />
    </AuthProvider>,
  );
  await userEvent.click(await screen.findByRole("button", { name: "Admin" }));
  await screen.findByText("auth.login");
  expect(
    screen.getByText(/does not cap cloud-provider or model billing/),
  ).not.toBeNull();
  expect(
    (screen.getByLabelText(/^Agent memory/) as HTMLInputElement).value,
  ).toBe("1024");
  await userEvent.clear(screen.getByLabelText(/^Agent memory/));
  await userEvent.type(screen.getByLabelText(/^Agent memory/), "2048");
  await userEvent.clear(screen.getByLabelText("Sessions per user"));
  await userEvent.type(screen.getByLabelText("Sessions per user"), "4");
  await userEvent.click(
    screen.getByRole("button", { name: "Save workspace policy" }),
  );
  await screen.findByText("Workspace policy saved.");
  expect(
    request.mock.calls.find(
      ([path, , method]) => path.endsWith("/policy") && method === "PATCH",
    )?.[1],
  ).toMatchObject({
    maxSessionsPerUser: 4,
    backingMode: "docker",
    sessionMemoryMiB: 2048,
    checkMemoryMiB: 1024,
  });
});

it("keeps an ordinary Admin's workspace policy read-only and only offers member creation", async () => {
  setAuthToken("ordinary-admin-test");
  const admin: AuthProfile = {
    ...member,
    user: { ...member.user, role: "admin" },
    permissions: {
      manageUsers: true,
      manageBacking: true,
      managePolicy: false,
      inspectAudit: true,
      runHostCommands: false,
    },
  };
  vi.spyOn(productApi, "request").mockImplementation(async (path) => {
    if (path.endsWith("/status"))
      return {
        configured: true,
        bootstrapRequired: false,
        instanceName: "Luci",
        backing: "docker",
      };
    if (path.endsWith("/me")) return admin;
    if (path.endsWith("/users")) return { users: [admin.user] };
    if (path.endsWith("/audit")) return { events: [] };
    throw new Error(path);
  });
  render(
    <AuthProvider>
      <RailFooter />
    </AuthProvider>,
  );
  await userEvent.click(await screen.findByRole("button", { name: "Admin" }));
  await screen.findByLabelText("Workspace name");
  expect(
    (
      screen.getByRole("button", {
        name: "Save workspace policy",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(
    (screen.getByLabelText("Workspace name") as HTMLInputElement).disabled,
  ).toBe(true);
  expect(
    Array.from(
      (screen.getByLabelText("New user role") as HTMLSelectElement).options,
    ).map((option) => option.value),
  ).toEqual(["member"]);
});

it("explains first-account requirements before submission and protects an in-flight account draft", async () => {
  window.history.replaceState(null, "", `/#bootstrap=${"b".repeat(43)}`);
  let finish!: (value: { token: string; user: AuthProfile["user"] }) => void;
  const pending = new Promise<{ token: string; user: AuthProfile["user"] }>(
    (resolve) => {
      finish = resolve;
    },
  );
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      if (path === "/v1/control-plane/status")
        return {
          configured: false,
          bootstrapRequired: true,
          instanceName: "My workspace",
          backing: "docker",
        };
      if (path === "/v1/auth/bootstrap") return pending;
      if (path === "/v1/auth/me") return member;
      throw new Error(path);
    });
  render(
    <AuthProvider>
      <p>Your project space</p>
    </AuthProvider>,
  );
  const displayName = await screen.findByLabelText("Display name");
  expect(document.activeElement).toBe(displayName);
  expect(screen.queryByRole("button", { name: "Retry connection" })).toBeNull();
  const username = screen.getByLabelText("Username") as HTMLInputElement;
  const password = screen.getByLabelText("Password") as HTMLInputElement;
  expect(
    document.getElementById(username.getAttribute("aria-describedby")!)
      ?.textContent,
  ).toContain("3–64");
  expect(
    document.getElementById(password.getAttribute("aria-describedby")!)
      ?.textContent,
  ).toContain("at least 12");
  expect(password.minLength).toBe(12);
  await userEvent.type(displayName, "Garden owner");
  await userEvent.type(username, "bad name");
  expect(username.checkValidity()).toBe(false);
  await userEvent.clear(username);
  await userEvent.type(username, "garden.owner");
  await userEvent.type(password, "a long private password");
  await userEvent.type(
    screen.getByLabelText("Confirm password"),
    "a long private password",
  );
  const createAccount = screen.getByRole("button", { name: "Create account" }) as HTMLButtonElement;
  expect(createAccount.type).toBe("submit");
  await userEvent.click(createAccount);
  await screen.findByText("Creating your workspace account…");
  expect(createAccount.getAttribute("aria-busy")).toBe("true");
  expect(createAccount.disabled).toBe(true);
  await userEvent.click(createAccount);
  expect(username.disabled).toBe(true);
  expect(password.disabled).toBe(true);
  expect((displayName as HTMLInputElement).disabled).toBe(true);
  expect(
    request.mock.calls.filter(([path]) => path === "/v1/auth/bootstrap"),
  ).toHaveLength(1);
  await act(async () => {
    finish({ token: "created-session", user: member.user });
    await pending;
  });
  await screen.findByText("Your project space");
});

it("recovers from a connection failure and removes the retry action once sign-in is ready", async () => {
  vi.spyOn(productApi, "request")
    .mockRejectedValueOnce(new Error("The control plane is offline."))
    .mockResolvedValue({
      configured: true,
      bootstrapRequired: false,
      instanceName: "My workspace",
      backing: "docker",
    });
  render(
    <AuthProvider>
      <p>Your project space</p>
    </AuthProvider>,
  );
  expect((await screen.findByRole("alert")).textContent).toContain("offline");
  expect(
    screen.getByRole("heading", { name: "Connect to Luci." }),
  ).not.toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Retry connection" }),
  );
  await screen.findByRole("button", { name: "Sign in" });
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByRole("button", { name: "Retry connection" })).toBeNull();
  expect(document.activeElement).toBe(screen.getByLabelText("Username"));
});

it("keeps account keyboard focus inside the dialog and returns it to the account trigger", async () => {
  setAuthToken("member-session");
  vi.spyOn(productApi, "request").mockImplementation(async (path) => {
    if (path === "/v1/control-plane/status")
      return {
        configured: true,
        bootstrapRequired: false,
        instanceName: "My workspace",
        backing: "docker",
      };
    if (path === "/v1/auth/me") return member;
    throw new Error(path);
  });
  render(
    <AuthProvider>
      <RailFooter />
    </AuthProvider>,
  );
  const trigger = await screen.findByRole("button", { name: "A member" });
  await userEvent.click(trigger);
  const dialog = screen.getByRole("dialog", { name: "Your account" });
  expect(document.activeElement).toBe(dialog);
  await userEvent.tab({ shift: true });
  expect(document.activeElement).toBe(
    screen.getByRole("button", { name: "Sign out" }),
  );
  await userEvent.tab();
  expect(document.activeElement).toBe(
    screen.getByRole("button", { name: "Close" }),
  );
  expect(
    (screen.getByLabelText("New password") as HTMLInputElement).minLength,
  ).toBe(12);
  await userEvent.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});

function ownedNative() {
  Object.defineProperty(window, "__TAURI_INTERNALS__", {
    configurable: true,
    value: {},
  });
}
const configured = {
  configured: true,
  bootstrapRequired: false,
  instanceName: "Saved garden",
  backing: "docker",
};
async function fillAccount() {
  await userEvent.type(screen.getByLabelText("Display name"), "Garden owner");
  await userEvent.type(screen.getByLabelText("Username"), "garden.owner");
  await userEvent.type(
    screen.getByLabelText("Password", { exact: true }),
    "a long private password",
  );
  await userEvent.type(
    screen.getByLabelText("Confirm password"),
    "a long private password",
  );
}
it("shows first setup despite a stale session and still obtains the owned bootstrap token", async () => {
  ownedNative();
  setAuthToken("obsolete-session");
  native.invoke.mockImplementation(async (command) =>
    command === "local_access_info"
      ? { available: true, owner: null }
      : "b".repeat(43),
  );
  const request = vi.spyOn(productApi, "request").mockResolvedValue({
    ...configured,
    configured: false,
    bootstrapRequired: true,
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  expect(
    ((await screen.findByLabelText("Bootstrap token")) as HTMLInputElement)
      .value,
  ).toBe("b".repeat(43));
  expect(authToken()).toBeNull();
  expect(request.mock.calls.some(([path]) => path === "/v1/auth/me")).toBe(
    false,
  );
  expect(native.invoke).toHaveBeenCalledWith("bootstrap_token");
  expect(
    screen
      .getByText("This workspace is ready for first setup")
      .closest("details")?.open,
  ).toBe(false);
});
it("offers sign-in after account creation and sign-out rather than repeating bootstrap", async () => {
  let fresh = true;
  window.history.replaceState(null, "", `/#bootstrap=${"b".repeat(43)}`);
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      if (path.endsWith("/status"))
        return { ...configured, configured: !fresh, bootstrapRequired: fresh };
      if (path === "/v1/auth/bootstrap") {
        fresh = false;
        return { token: "first-session", user: member.user };
      }
      if (path.endsWith("/me")) return member;
      return {};
    });
  render(
    <AuthProvider>
      <RailFooter />
    </AuthProvider>,
  );
  await screen.findByLabelText("Display name");
  await fillAccount();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await userEvent.click(
    await screen.findByRole("button", { name: "A member" }),
  );
  await userEvent.click(screen.getByRole("button", { name: "Sign out" }));
  await screen.findByRole("button", { name: "Sign in" });
  expect(screen.queryByLabelText("Bootstrap token")).toBeNull();
  expect(
    request.mock.calls.filter(([path]) => path === "/v1/auth/bootstrap"),
  ).toHaveLength(1);
});
it("creates a separate owned local account through IPC and never sends ownership credentials through browser HTTP", async () => {
  ownedNative();
  native.invoke.mockImplementation(async (command) =>
    command === "local_access_info"
      ? { available: true, owner: member.user }
      : { token: "new-local-session", user: member.user },
  );
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) =>
      path.endsWith("/status") ? configured : member,
    );
  render(
    <AuthProvider>
      <p>Separate workspace</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Create account" }),
  );
  expect(
    screen.getByText(/Existing accounts and their projects stay intact/),
  ).not.toBeNull();
  expect(screen.queryByLabelText("Invitation code")).toBeNull();
  await fillAccount();
  await userEvent.click(screen.getByLabelText("Show passwords"));
  expect(
    (screen.getByLabelText("Password", { exact: true }) as HTMLInputElement)
      .type,
  ).toBe("text");
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await screen.findByText("Separate workspace");
  expect(native.invoke).toHaveBeenCalledWith("create_local_account", {
    username: "garden.owner",
    displayName: "Garden owner",
    password: "a long private password",
  });
  expect(
    request.mock.calls.every(
      ([path]) => path === "/v1/control-plane/status" || path === "/v1/auth/me",
    ),
  ).toBe(true);
});
it("recovers only the saved owner through IPC after matching password confirmation", async () => {
  ownedNative();
  native.invoke.mockImplementation(async (command) =>
    command === "local_access_info"
      ? { available: true, owner: member.user }
      : { token: "recovered-session", user: member.user },
  );
  vi.spyOn(productApi, "request").mockImplementation(async (path) =>
    path.endsWith("/status") ? configured : member,
  );
  render(
    <AuthProvider>
      <p>Original projects</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Forgot sign-in?" }),
  );
  expect(
    screen.getByText(/keeps the original account, projects and settings/),
  ).not.toBeNull();
  await userEvent.type(
    screen.getByLabelText("New password"),
    "a long private password",
  );
  await userEvent.type(
    screen.getByLabelText("Confirm password"),
    "different private password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Recover owner account" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "do not match",
  );
  expect(
    native.invoke.mock.calls.some(
      ([command]) => command === "recover_local_owner",
    ),
  ).toBe(false);
  await userEvent.clear(screen.getByLabelText("Confirm password"));
  await userEvent.type(
    screen.getByLabelText("Confirm password"),
    "a long private password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Recover owner account" }),
  );
  await screen.findByText("Original projects");
  expect(native.invoke).toHaveBeenCalledWith("recover_local_owner", {
    newPassword: "a long private password",
  });
});
it("requires an invitation for browser account creation and keeps an invalid code recoverable", async () => {
  let invalid = true;
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      if (path.endsWith("/status")) return configured;
      if (path.endsWith("/accept")) {
        if (invalid)
          throw new Error(
            "This invitation is invalid, expired or already used.",
          );
        return { token: "invited-session", user: member.user };
      }
      return member;
    });
  render(
    <AuthProvider>
      <p>Invited workspace</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Create account" }),
  );
  await userEvent.type(
    screen.getByLabelText("Invitation code"),
    "one-time-private-code",
  );
  await fillAccount();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "already used",
  );
  expect((screen.getByLabelText("Username") as HTMLInputElement).value).toBe(
    "garden.owner",
  );
  expect(screen.queryByRole("button", { name: "Retry connection" })).toBeNull();
  invalid = false;
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await screen.findByText("Invited workspace");
  expect(request).toHaveBeenCalledWith(
    "/v1/auth/invitations/accept",
    expect.objectContaining({
      inviteCode: "one-time-private-code",
      username: "garden.owner",
    }),
  );
  expect(native.invoke).not.toHaveBeenCalled();
});
it("never offers local recovery for an external API even in a native window", async () => {
  ownedNative();
  native.invoke.mockResolvedValue({ available: false });
  vi.spyOn(productApi, "request").mockResolvedValue(configured);
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Forgot sign-in?" }),
  );
  expect(screen.getByText(/Ask this workspace’s administrator/)).not.toBeNull();
  expect(
    screen.queryByRole("button", { name: "Recover owner account" }),
  ).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  expect(screen.getByLabelText("Invitation code")).not.toBeNull();
  expect(native.invoke.mock.calls.map(([command]) => command)).toEqual([
    "local_access_info",
  ]);
});
it("recovers if another client completes first setup while the form is open", async () => {
  let configuredElsewhere = false;
  window.history.replaceState(null, "", `/#bootstrap=${"b".repeat(43)}`);
  vi.spyOn(productApi, "request").mockImplementation(async (path) => {
    if (path.endsWith("/status"))
      return { ...configured, bootstrapRequired: !configuredElsewhere };
    configuredElsewhere = true;
    throw Object.assign(new Error("Already configured"), {
      status: 403,
      code: "bootstrap_denied",
    });
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  await screen.findByLabelText("Display name");
  await fillAccount();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await screen.findByRole("button", { name: "Sign in" });
  expect((await screen.findByRole("alert")).textContent).toContain(
    "already has an account",
  );
  expect(screen.queryByLabelText("Bootstrap token")).toBeNull();
});

it("keeps manual first-setup code entry open while the user types", async () => {
  vi.spyOn(productApi, "request").mockResolvedValue({
    ...configured,
    configured: false,
    bootstrapRequired: true,
    bootstrapTokenPath: "C:/Private/control-plane/bootstrap-token",
  });
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  const input = await screen.findByLabelText("Bootstrap token");
  await userEvent.type(input, "b".repeat(43));
  expect((input as HTMLInputElement).value).toBe("b".repeat(43));
  expect(input.closest("details")?.open).toBe(true);
});
it("preserves local account drafts and offers a capability retry only for native connection failures", async () => {
  ownedNative();
  let attempt = 0;
  native.invoke.mockImplementation(async (command) => {
    if (command === "local_access_info")
      return { available: true, owner: member.user };
    attempt++;
    throw attempt === 1
      ? "That username already exists."
      : "Luci's local service is unavailable. Retry the connection.";
  });
  vi.spyOn(productApi, "request").mockResolvedValue(configured);
  render(
    <AuthProvider>
      <p>Private projects</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Create account" }),
  );
  await fillAccount();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "already exists",
  );
  expect(
    screen.queryByRole("button", { name: "Retry local account access" }),
  ).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await screen.findByRole("button", { name: "Retry local account access" });
  expect(
    (screen.getByLabelText("Display name") as HTMLInputElement).value,
  ).toBe("Garden owner");
  expect(
    (screen.getByLabelText("Password", { exact: true }) as HTMLInputElement)
      .value,
  ).toBe("a long private password");
  await userEvent.click(
    screen.getByRole("button", { name: "Retry local account access" }),
  );
  await screen.findByRole("button", { name: "Create account" });
  expect((screen.getByLabelText("Username") as HTMLInputElement).value).toBe(
    "garden.owner",
  );
});

it("consumes portable owner proof immediately, creates a separate account and never persists or forwards the proof to normal APIs", async () => {
  const proof = "o".repeat(43);
  window.history.replaceState(null, "", `/#owner=${proof}&view=entry`);
  const fetcher = vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(
      async (_url, init) =>
        new Response(
          JSON.stringify(
            init?.method === "POST"
              ? { token: "portable-created-session", user: member.user }
              : { owner: member.user },
          ),
        ),
    );
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) =>
      path.endsWith("/status") ? configured : member,
    );
  render(
    <AuthProvider>
      <p>Portable account projects</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Create account" }),
  );
  expect(window.location.hash).toBe("#view=entry");
  expect(screen.queryByLabelText("Invitation code")).toBeNull();
  await fillAccount();
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
  await screen.findByText("Portable account projects");
  expect(fetcher).toHaveBeenCalledWith(
    `${window.location.origin}/v1/auth/local-account`,
    expect.objectContaining({
      headers: {
        "X-Luci-Local-Access-Token": proof,
        "Content-Type": "application/json",
      },
    }),
  );
  expect(JSON.stringify(request.mock.calls)).not.toContain(proof);
  expect(JSON.stringify(window.sessionStorage)).not.toContain(proof);
  expect(JSON.stringify(window.localStorage)).not.toContain(proof);
});
it("recovers the original portable owner with the in-memory proof and preserves a network-failed draft", async () => {
  window.history.replaceState(null, "", `/#owner=${"o".repeat(43)}`);
  let offline = true;
  const fetcher = vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (_url, init) => {
      if (init?.method === "POST" && offline)
        throw new Error("Connection refused");
      return new Response(
        JSON.stringify(
          init?.method === "POST"
            ? { token: "recovered-portable", user: member.user }
            : { owner: member.user },
        ),
      );
    });
  vi.spyOn(productApi, "request").mockImplementation(async (path) =>
    path.endsWith("/status") ? configured : member,
  );
  render(
    <AuthProvider>
      <p>Saved portable work</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Forgot sign-in?" }),
  );
  await userEvent.type(
    screen.getByLabelText("New password"),
    "a long private password",
  );
  await userEvent.type(
    screen.getByLabelText("Confirm password"),
    "a long private password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Recover owner account" }),
  );
  await screen.findByRole("button", { name: "Retry local account access" });
  expect(
    (screen.getByLabelText("New password") as HTMLInputElement).value,
  ).toBe("a long private password");
  offline = false;
  await userEvent.click(
    screen.getByRole("button", { name: "Recover owner account" }),
  );
  await screen.findByText("Saved portable work");
  expect(
    fetcher.mock.calls.filter(([url]) =>
      String(url).endsWith("/local-owner/recover"),
    ),
  ).toHaveLength(2);
  expect(native.invoke).not.toHaveBeenCalled();
});
it("clears and ignores owner fragments for an external API and for native windows", async () => {
  const proof = "o".repeat(43);
  const fetcher = vi.spyOn(globalThis, "fetch");
  window.__LUCI_API__ = "https://team.example";
  window.history.replaceState(null, "", `/#owner=${proof}`);
  vi.spyOn(productApi, "request").mockResolvedValue(configured);
  const first = render(
    <AuthProvider>
      <p>Private work</p>
    </AuthProvider>,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Create account" }),
  );
  expect(screen.getByLabelText("Invitation code")).not.toBeNull();
  expect(window.location.hash).toBe("");
  expect(fetcher).not.toHaveBeenCalled();
  first.unmount();
  Reflect.deleteProperty(window, "__LUCI_API__");
  ownedNative();
  native.invoke.mockResolvedValue({ available: false });
  window.history.replaceState(null, "", `/#owner=${proof}`);
  render(
    <AuthProvider>
      <p>Private work</p>
    </AuthProvider>,
  );
  await screen.findByRole("button", { name: "Sign in" });
  expect(window.location.hash).toBe("");
  expect(fetcher).not.toHaveBeenCalled();
});
