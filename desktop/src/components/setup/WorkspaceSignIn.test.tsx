import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkspaceSignIn } from "./WorkspaceSignIn";
import { createProductClient } from "../../api/product";

it("redeems a remote invitation through the existing verified workspace route and offers administrator recovery only", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(
      async (_url, init) =>
        new Response(
          JSON.stringify({
            authenticated: init?.method === "POST",
            bootstrapRequired: false,
            message:
              init?.method === "POST"
                ? "Remote account created"
                : "Sign in required",
          }),
        ),
    );
  const received = vi.fn();
  render(
    <WorkspaceSignIn
      client={createProductClient("http://localhost:8730", fetcher)}
      connectionId="ssh-1"
      onAuthenticated={received}
    />,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Forgot workspace sign-in?" }),
  );
  expect(screen.getByText(/Ask this workspace’s administrator/)).not.toBeNull();
  expect(
    screen.queryByRole("button", { name: "Recover owner account" }),
  ).toBeNull();
  await userEvent.click(
    screen.getByRole("button", { name: "Join with an invitation" }),
  );
  await userEvent.type(
    screen.getByLabelText("Remote invitation code", { exact: true }),
    "one-time-remote-code",
  );
  await userEvent.type(
    screen.getByLabelText("Remote display name"),
    "Remote person",
  );
  await userEvent.type(
    screen.getByLabelText("Remote username", { exact: true }),
    "remote.person",
  );
  await userEvent.type(
    screen.getByLabelText("Remote password", { exact: true }),
    "a long private password",
  );
  await userEvent.type(
    screen.getByLabelText("Confirm remote password"),
    "a long private password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Create workspace account" }),
  );
  await screen.findByText("Remote account created");
  const [url, init] = fetcher.mock.calls.find(
    ([, options]) => options?.method === "POST",
  )!;
  expect(url).toBe("http://localhost:8730/v1/setup/workspace/auth");
  expect(JSON.parse(init!.body as string)).toEqual({
    connectionId: "ssh-1",
    username: "remote.person",
    password: "a long private password",
    displayName: "Remote person",
    inviteCode: "one-time-remote-code",
  });
  expect(received).toHaveBeenLastCalledWith(true);
  expect(screen.queryByLabelText("Remote invitation code")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Disconnect remote sign-in" }));
  await screen.findByRole("button", { name: "Sign in to workspace" });
  expect(screen.queryByLabelText("Remote invitation code")).toBeNull();
});
it("validates remote confirmation and preserves a failed invitation draft for correction", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (_url, init) =>
      init?.method === "POST"
        ? new Response(
            JSON.stringify({
              error: "invalid_invitation",
              message:
                "The invitation expired. Ask your administrator for a new code.",
            }),
            { status: 409 },
          )
        : new Response(
            JSON.stringify({
              authenticated: false,
              bootstrapRequired: false,
              message: "Sign in required",
            }),
          ),
    );
  render(
    <WorkspaceSignIn
      client={createProductClient("http://localhost:8730", fetcher)}
      connectionId="ssh-1"
      onAuthenticated={vi.fn()}
    />,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Join with an invitation" }),
  );
  await userEvent.type(
    screen.getByLabelText("Remote invitation code", { exact: true }),
    "old-code",
  );
  await userEvent.type(
    screen.getByLabelText("Remote display name"),
    "Remote person",
  );
  await userEvent.type(
    screen.getByLabelText("Remote username", { exact: true }),
    "remote.person",
  );
  await userEvent.type(
    screen.getByLabelText("Remote password", { exact: true }),
    "a long private password",
  );
  await userEvent.type(
    screen.getByLabelText("Confirm remote password"),
    "does not match",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Create workspace account" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "do not match",
  );
  expect(fetcher.mock.calls.some(([, init]) => init?.method === "POST")).toBe(
    false,
  );
  await userEvent.clear(screen.getByLabelText("Confirm remote password"));
  await userEvent.type(
    screen.getByLabelText("Confirm remote password"),
    "a long private password",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Create workspace account" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain("expired");
  expect(
    (
      screen.getByLabelText("Remote username", {
        exact: true,
      }) as HTMLInputElement
    ).value,
  ).toBe("remote.person");
  await userEvent.click(
    screen.getByRole("button", { name: "Back to workspace sign-in" }),
  );
  expect(screen.queryByLabelText("Remote invitation code")).toBeNull();
  expect(
    (
      screen.getByLabelText("Remote password", {
        exact: true,
      }) as HTMLInputElement
    ).value,
  ).toBe("");
});
