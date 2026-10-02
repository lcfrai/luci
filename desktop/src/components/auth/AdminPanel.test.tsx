import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, type AuthProfile } from "./AuthProvider";
import { RailFooter } from "../rail/RailFooter";
import { productApi, type SetupStatus } from "../../api/product";
import { authToken, setAuthToken } from "../../api/authSession";

const local: AuthProfile = {
  user: {
    id: "local-owner",
    username: "local",
    displayName: "Local owner",
    role: "super-admin",
    disabled: false,
  },
  permissions: {
    manageUsers: true,
    manageBacking: true,
    managePolicy: true,
    inspectAudit: true,
    runHostCommands: true,
  },
  policy: {
    allowHostCommands: false,
    allowCloudProvisioning: true,
    allowPublicPublishing: false,
    maxSessionsPerUser: 1,
    maxConcurrentChecksPerUser: 1,
    monthlyBudgetUsd: 30,
    allowedHarnesses: ["shell"],
    backingMode: "docker",
    instanceName: "My machine",
    sessionMemoryMiB: 1024,
    sessionCPUMillis: 1000,
    checkMemoryMiB: 1024,
    checkCPUMillis: 1000,
  },
};
const cloud: AuthProfile = {
  ...local,
  user: {
    ...local.user,
    id: "cloud-owner",
    username: "cloud",
    displayName: "Cloud owner",
  },
  policy: {
    ...local.policy,
    instanceName: "AWS workspace",
    backingMode: "aws",
    monthlyBudgetUsd: 45,
  },
};
const base = "/v1/setup/workspace/admin";
beforeEach(() => {
  setAuthToken("local-session");
  vi.spyOn(productApi, "status").mockResolvedValue({
    workspace: {
      mode: "cloud",
      connectionId: "ssh-1",
      host: "cloud.example",
      ready: true,
    },
  } as SetupStatus);
});
afterEach(() => {
  setAuthToken(null);
  vi.restoreAllMocks();
});
async function openAdmin() {
  render(
    <AuthProvider>
      <RailFooter />
    </AuthProvider>,
  );
  await userEvent.click(await screen.findByRole("button", { name: "Admin" }));
}
function publicResponse(path: string) {
  if (path === "/v1/control-plane/status")
    return {
      configured: true,
      bootstrapRequired: false,
      instanceName: "My machine",
      backing: "docker",
    };
  if (path === "/v1/auth/me") return local;
}

it("defaults to the selected backing, checks its identity first, and sends every admin operation to its bridge", async () => {
  let remote = structuredClone(cloud);
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path, body, method) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return remote;
      if (path === `${base}/users`)
        return body
          ? { ...cloud.user, id: "new-user" }
          : { users: [cloud.user] };
      if (path === `${base}/audit`)
        return {
          events: [
            {
              id: "a1",
              at: "2026-10-02T00:00:00Z",
              actorId: "cloud-owner",
              action: "cloud.audit",
              target: "cloud-owner",
              outcome: "success",
            },
          ],
        };
      if (path === `${base}/policy` && method === "PATCH") {
        remote = {
          ...remote,
          policy: { ...remote.policy, ...(body as object) },
        };
        return remote.policy;
      }
      if (path === `${base}/users/cloud-owner` && method === "PATCH")
        return cloud.user;
      if (path === `${base}/users/cloud-owner/runtime`)
        return { status: "running", message: "Cloud runtime is running" };
      if (path === `${base}/users/cloud-owner/runtime/stop`)
        return { status: "stopped", message: "Cloud runtime stopped" };
      throw new Error(`Unexpected ${method ?? "GET"} ${path}`);
    });
  await openAdmin();
  await screen.findByText("cloud.audit");
  expect(
    (screen.getByLabelText("Control plane") as HTMLSelectElement).value,
  ).toBe("cloud");
  expect(
    screen.getByText(/AWS workspace · Cloud owner · super-admin/),
  ).not.toBeNull();
  expect(screen.getByText(/cloud.example/)).not.toBeNull();
  const paths = request.mock.calls.map(([path]) => path);
  expect(paths.indexOf(`${base}/me`)).toBeLessThan(
    paths.indexOf(`${base}/users`),
  );
  await userEvent.clear(screen.getByLabelText("Sessions per user"));
  await userEvent.type(screen.getByLabelText("Sessions per user"), "2");
  await userEvent.click(
    screen.getByRole("button", { name: "Save workspace policy" }),
  );
  await screen.findByText("Workspace policy saved.");
  expect(request).toHaveBeenCalledWith(
    `${base}/policy`,
    expect.objectContaining({ maxSessionsPerUser: 2, backingMode: "aws" }),
    "PATCH",
  );
  expect(
    request.mock.calls.filter(([path]) => path === `${base}/me`),
  ).toHaveLength(2);
  expect(
    request.mock.calls.filter(([path]) => path === "/v1/auth/me"),
  ).toHaveLength(1);
  await userEvent.click(
    screen.getByRole("button", { name: "Inspect runtime for cloud" }),
  );
  await screen.findByText(/Cloud runtime is running/);
  await userEvent.click(
    screen.getByRole("button", { name: "Stop runtime for cloud" }),
  );
  await screen.findByText(/Cloud runtime stopped/);
  await userEvent.type(screen.getByLabelText("New username"), "person");
  await userEvent.type(screen.getByLabelText("New display name"), "A person");
  await userEvent.type(
    screen.getByLabelText("Initial password"),
    "private-test-password",
  );
  await userEvent.click(screen.getByRole("button", { name: "Create user" }));
  await screen.findByText(/User created/);
  expect(request).toHaveBeenCalledWith(
    `${base}/users`,
    expect.objectContaining({ username: "person", role: "member" }),
  );
  await userEvent.click(screen.getByText("super-admin · active"));
  await userEvent.click(screen.getByRole("button", { name: "Save access" }));
  await screen.findByText("User access updated.");
  expect(request).toHaveBeenCalledWith(
    `${base}/users/cloud-owner`,
    expect.objectContaining({ role: "super-admin" }),
    "PATCH",
  );
  expect(
    request.mock.calls.some(([path]) => path.startsWith("/v1/admin/")),
  ).toBe(false);
  expect(authToken()).toBe("local-session");
});

it("clears remote drafts and switches all reads and policy writes back to the explicitly selected machine", async () => {
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path, body, method) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return cloud;
      if (path.endsWith("/users")) return { users: [] };
      if (path.endsWith("/audit")) return { events: [] };
      if (path === "/v1/admin/policy" && method === "PATCH")
        return { ...local.policy, ...(body as object) };
      throw new Error(path);
    });
  await openAdmin();
  await screen.findByLabelText("Workspace name");
  await userEvent.type(
    screen.getByLabelText("Initial password"),
    "cloud-draft-secret",
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Control plane"),
    "local",
  );
  await waitFor(() => expect(request).toHaveBeenCalledWith("/v1/admin/users"));
  expect(
    (screen.getByLabelText("Initial password") as HTMLInputElement).value,
  ).toBe("");
  expect(
    (screen.getByLabelText("Workspace name") as HTMLInputElement).value,
  ).toBe("My machine");
  expect(
    screen.getByText(/My machine · Local owner · super-admin/),
  ).not.toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Save workspace policy" }),
  );
  await screen.findByText("Workspace policy saved.");
  expect(request).toHaveBeenCalledWith(
    "/v1/admin/policy",
    expect.objectContaining({ backingMode: "docker" }),
    "PATCH",
  );
  expect(
    request.mock.calls.some(
      ([path, , method]) => path.startsWith(base) && method === "PATCH",
    ),
  ).toBe(false);
});

it("gives a remote member no administrative controls even when the local account is Super Admin", async () => {
  const member: AuthProfile = {
    ...cloud,
    user: { ...cloud.user, role: "member" },
    permissions: {
      manageUsers: false,
      managePolicy: false,
      manageBacking: false,
      inspectAudit: false,
      runHostCommands: false,
    },
  };
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return member;
      throw new Error(path);
    });
  await openAdmin();
  await screen.findByText(
    /This backing account has no administrative permissions/,
  );
  expect(
    screen.getByText(/AWS workspace · Cloud owner · member/),
  ).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Create user" })).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Save workspace policy" }),
  ).toBeNull();
  expect(
    request.mock.calls
      .filter(([path]) => path.startsWith(base))
      .map(([path]) => path),
  ).toEqual([`${base}/me`]);
});

it("keeps an expired backing login distinct from local sign-in and retries without silently administering this machine", async () => {
  let expired = true;
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) {
        if (expired)
          throw new Error(
            "backing_sign_in_required: Sign in to this cloud backing again.",
          );
        return cloud;
      }
      if (path === `${base}/users`) return { users: [] };
      if (path === `${base}/audit`) return { events: [] };
      throw new Error(path);
    });
  await openAdmin();
  await screen.findByRole("alert");
  expect(
    (screen.getByLabelText("Control plane") as HTMLSelectElement).value,
  ).toBe("cloud");
  expect(screen.queryByLabelText("Workspace name")).toBeNull();
  expect(authToken()).toBe("local-session");
  expect(
    request.mock.calls.some(([path]) => path.startsWith("/v1/admin/")),
  ).toBe(false);
  expired = false;
  await userEvent.click(
    screen.getByRole("button", { name: "Retry backing connection" }),
  );
  await screen.findByLabelText("Workspace name");
  expect(screen.queryByRole("alert")).toBeNull();
});

it("ignores a late cloud identity response after the user chooses this machine", async () => {
  let resolveCloud!: (profile: AuthProfile) => void;
  const pending = new Promise<AuthProfile>((resolve) => {
    resolveCloud = resolve;
  });
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return pending;
      if (path === "/v1/admin/users") return { users: [] };
      if (path === "/v1/admin/audit") return { events: [] };
      throw new Error(path);
    });
  await openAdmin();
  await waitFor(() => expect(request).toHaveBeenCalledWith(`${base}/me`));
  await userEvent.selectOptions(
    screen.getByLabelText("Control plane"),
    "local",
  );
  await screen.findByLabelText("Workspace name");
  await act(async () => {
    resolveCloud(cloud);
    await pending;
  });
  expect(
    (screen.getByLabelText("Control plane") as HTMLSelectElement).value,
  ).toBe("local");
  expect(
    (screen.getByLabelText("Workspace name") as HTMLInputElement).value,
  ).toBe("My machine");
  expect(request.mock.calls.some(([path]) => path === `${base}/users`)).toBe(
    false,
  );
});

it("applies a backing role change before issuing further administration reads", async () => {
  let remote = structuredClone(cloud);
  let demoted = false;
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path, _body, method) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return remote;
      if (path === `${base}/users/cloud-owner` && method === "PATCH") {
        demoted = true;
        remote = {
          ...remote,
          user: { ...remote.user, role: "member" },
          permissions: {
            manageUsers: false,
            managePolicy: false,
            manageBacking: false,
            inspectAudit: false,
            runHostCommands: false,
          },
        };
        return remote.user;
      }
      if (!demoted && path === `${base}/users`) return { users: [cloud.user] };
      if (!demoted && path === `${base}/audit`) return { events: [] };
      throw new Error(`Forbidden after role change: ${path}`);
    });
  await openAdmin();
  await userEvent.click(await screen.findByText("super-admin · active"));
  await userEvent.selectOptions(
    screen.getByLabelText("Role for cloud"),
    "member",
  );
  await userEvent.click(screen.getByRole("button", { name: "Save access" }));
  await screen.findByText(
    /This backing account has no administrative permissions/,
  );
  expect(screen.queryByRole("alert")).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Save workspace policy" }),
  ).toBeNull();
  const mutation = request.mock.calls.findIndex(
    ([path, , method]) =>
      path === `${base}/users/cloud-owner` && method === "PATCH",
  );
  expect(request.mock.calls.slice(mutation + 1).map(([path]) => path)).toEqual([
    `${base}/me`,
  ]);
});

it("distinguishes loading administration from an empty audit", async () => {
  let finish!: (value: { users: AuthProfile["user"][] }) => void;
  const pending = new Promise<{ users: AuthProfile["user"][] }>((resolve) => {
    finish = resolve;
  });
  vi.spyOn(productApi, "request").mockImplementation(async (path) => {
    const response = publicResponse(path);
    if (response) return response;
    if (path === `${base}/me`) return cloud;
    if (path === `${base}/users`) return pending;
    if (path === `${base}/audit`) return { events: [] };
    throw new Error(path);
  });
  await openAdmin();
  await screen.findByText("Loading people and activity…");
  expect(screen.queryByText("No audit events returned.")).toBeNull();
  await act(async () => {
    finish({ users: [] });
    await pending;
  });
  await screen.findByText("No audit events returned.");
  expect(
    screen.getByRole("table", { name: "Recent workspace activity" }),
  ).not.toBeNull();
  expect(screen.queryByText("Loading people and activity…")).toBeNull();
});

it("names each person's runtime actions and prevents ordinary Admins from stopping administrator runtimes", async () => {
  const admin: AuthProfile = {
    ...cloud,
    user: {
      ...cloud.user,
      id: "cloud-admin",
      username: "operator",
      role: "admin",
    },
    permissions: {
      ...cloud.permissions,
      managePolicy: false,
      runHostCommands: false,
    },
  };
  const member = {
    ...cloud.user,
    id: "cloud-member",
    username: "gardener",
    role: "member" as const,
  };
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (path) => {
      const response = publicResponse(path);
      if (response) return response;
      if (path === `${base}/me`) return admin;
      if (path === `${base}/users`)
        return { users: [cloud.user, admin.user, member] };
      if (path === `${base}/audit`) return { events: [] };
      if (path === `${base}/users/cloud-member/runtime`)
        return { status: "running", message: "Member workspace is ready." };
      throw new Error(path);
    });
  await openAdmin();
  const protectedStop = await screen.findByRole("button", {
    name: "Stop runtime for cloud",
  });
  expect((protectedStop as HTMLButtonElement).disabled).toBe(true);
  expect(
    (
      screen.getByRole("button", {
        name: "Stop runtime for operator",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(
    (
      screen.getByRole("button", {
        name: "Stop runtime for gardener",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(false);
  await userEvent.click(protectedStop);
  expect(
    request.mock.calls.some(([path]) => path.endsWith("/runtime/stop")),
  ).toBe(false);
  await userEvent.click(
    screen.getByRole("button", { name: "Inspect runtime for gardener" }),
  );
  expect(
    (await screen.findByText(/Member workspace is ready/)).getAttribute("role"),
  ).toBe("status");
  expect(
    (screen.getByLabelText("Initial password") as HTMLInputElement).minLength,
  ).toBe(12);
});
