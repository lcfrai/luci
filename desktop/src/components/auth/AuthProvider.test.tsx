import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, type AuthProfile } from "./AuthProvider";
import { RailFooter } from "../rail/RailFooter";
import { productApi, type SetupStatus } from "../../api/product";
import { authToken, setAuthToken } from "../../api/authSession";
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
  vi.spyOn(productApi, "status").mockResolvedValue({
    workspace: { mode: "local", connectionId: "", host: "", ready: true },
  } as SetupStatus);
});
afterEach(() => {
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
