import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AdminInvitations, type Invitation } from "./AdminInvitations";
import { productApi } from "../../api/product";

afterEach(() => vi.restoreAllMocks());
const invitation: Invitation = {
  id: "invite-1",
  role: "member",
  createdBy: "owner",
  createdAt: "2026-10-02T00:00:00Z",
  expiresAt: "2099-10-04T00:00:00Z",
  revoked: false,
};
it("creates and revokes a one-time invitation on the selected backing and never fetches its code again", async () => {
  const base = "/v1/setup/workspace/admin";
  const request = vi
    .spyOn(productApi, "request")
    .mockImplementation(async (_path, body, method) => {
      if (method === "DELETE") return { revoked: true };
      if (body) return { invitation, inviteCode: "private-single-use-code" };
      return { invitations: [] };
    });
  const busy = vi.fn();
  render(
    <AdminInvitations base={base} canSuper disabled={false} onBusy={busy} />,
  );
  expect(request).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Manage invitations" }),
  );
  await screen.findByText("No invitations have been created.");
  await userEvent.click(
    screen.getByRole("button", { name: "Create invitation" }),
  );
  expect(
    (await screen.findByLabelText("New invitation code")).textContent,
  ).toBe("private-single-use-code");
  expect(request).toHaveBeenCalledWith(`${base}/invitations`, {
    role: "member",
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Revoke invitation invite-1" }),
  );
  await screen.findByText(
    "Invitation revoked. Its code can no longer create an account.",
  );
  expect(screen.queryByLabelText("New invitation code")).toBeNull();
  expect(request).toHaveBeenCalledWith(
    `${base}/invitations/invite-1`,
    undefined,
    "DELETE",
  );
  expect(
    request.mock.calls.some(([path]) => path.startsWith("/v1/admin/")),
  ).toBe(false);
  expect(busy.mock.calls).toEqual([[true], [false], [true], [false]]);
});
it("limits ordinary Admin invitations to members and keeps permission failures recoverable", async () => {
  let failed = true;
  vi.spyOn(productApi, "request").mockImplementation(async (_path, body) => {
    if (body) throw new Error("Only a Super Admin can grant this access.");
    if (failed) throw new Error("The backing is unavailable.");
    return { invitations: [{ ...invitation, role: "admin" }] };
  });
  render(
    <AdminInvitations
      base="/v1/admin"
      canSuper={false}
      disabled={false}
      onBusy={vi.fn()}
    />,
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Manage invitations" }),
  );
  await screen.findByRole("alert");
  const options = Array.from(
    (screen.getByLabelText("Invitation role") as HTMLSelectElement).options,
  ).map((option) => option.value);
  expect(options).toEqual(["member"]);
  failed = false;
  await userEvent.click(
    screen.getByRole("button", { name: "Reload invitations" }),
  );
  await waitFor(() =>
    expect(
      (
        screen.getByRole("button", {
          name: "Revoke invitation invite-1",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true),
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Create invitation" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Only a Super Admin",
  );
});
