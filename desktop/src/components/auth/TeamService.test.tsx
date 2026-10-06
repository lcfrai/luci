import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { teamServiceApi, type TeamService, type TeamServiceRow } from "../../api/teamService";
import { AdminTeamServices } from "./AdminTeamServices";
import { TeamServiceStatus } from "./TeamServiceStatus";

const active: TeamService = { mode: "self-hosted", status: "active", revision: 0 };
const first: TeamServiceRow = { teamId: "team-aaaaaaaaaaaaaaaaaaaaaa", name: "Garden team", service: active };
const second: TeamServiceRow = { teamId: "team-bbbbbbbbbbbbbbbbbbbbbb", name: "Another team", service: { mode: "managed", status: "suspended", revision: 7 } };
const catalogue = (teams = [first, second]) => ({ teams, billingConnected: false as const, executionIsolationVerified: false as const });
afterEach(() => vi.restoreAllMocks());

it("loads service operations deliberately and saves an explicit revision with the selected team", async () => {
  const list = vi.spyOn(teamServiceApi, "list").mockResolvedValue(catalogue());
  const save = vi.spyOn(teamServiceApi, "save").mockResolvedValue({ mode: "managed", status: "suspended", revision: 1, reason: "Operator review" });
  const onBusy = vi.fn();
  render(<AdminTeamServices disabled={false} onBusy={onBusy} />);
  expect(list).not.toHaveBeenCalled();
  await userEvent.click(screen.getByText("Team service access on this machine"));
  await screen.findByRole("option", { name: "Garden team" });
  expect(screen.getByText(/they do not collect payment or verify a dedicated host/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Save service access" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.selectOptions(screen.getByLabelText("Service access"), "managed:suspended");
  await userEvent.type(screen.getByLabelText("Reason for service change"), "Operator review");
  await userEvent.click(screen.getByRole("button", { name: "Save service access" }));
  await screen.findByText("Service access saved for Garden team.");
  expect(save).toHaveBeenCalledExactlyOnceWith(first.teamId, { expectedRevision: 0, mode: "managed", status: "suspended", reason: "Operator review" });
  expect(screen.getByText(/Current access: Managed · suspended · revision 1/)).not.toBeNull();
  expect(onBusy).toHaveBeenLastCalledWith(false);
  expect((screen.getByLabelText("Reason for service change") as HTMLInputElement).value).toBe("");
});

it("retains a failed operation as an error and refreshes revisions without replaying a mutation", async () => {
  const list = vi.spyOn(teamServiceApi, "list").mockResolvedValue(catalogue());
  const save = vi.spyOn(teamServiceApi, "save").mockRejectedValue(new Error("team_service_changed"));
  render(<AdminTeamServices disabled={false} onBusy={() => {}} />);
  await userEvent.click(screen.getByText("Team service access on this machine"));
  await screen.findByLabelText("Team to operate");
  await userEvent.selectOptions(screen.getByLabelText("Team to operate"), second.teamId);
  await userEvent.selectOptions(screen.getByLabelText("Service access"), "managed:active");
  await userEvent.type(screen.getByLabelText("Reason for service change"), "Restore access");
  await userEvent.click(screen.getByRole("button", { name: "Save service access" }));
  expect((await screen.findByRole("alert")).textContent).toContain("team_service_changed");
  expect(screen.queryByText(/Service access saved/)).toBeNull();
  expect(save).toHaveBeenCalledWith(second.teamId, expect.objectContaining({ expectedRevision: 7 }));
  list.mockResolvedValue(catalogue([first, { ...second, service: { ...second.service, revision: 8 } }]));
  await userEvent.click(screen.getByRole("button", { name: "Reload service access" }));
  await screen.findByText(/revision 8/);
  expect(save).toHaveBeenCalledTimes(1);
  expect((screen.getByLabelText("Reason for service change") as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("Service access") as HTMLSelectElement).value).toBe("managed:suspended");
});

it("blocks duplicate submissions and ignores completion after the operator view closes", async () => {
  vi.spyOn(teamServiceApi, "list").mockResolvedValue(catalogue());
  let finish!: (value: TeamService) => void;
  const pending = new Promise<TeamService>(resolve => { finish = resolve; });
  const save = vi.spyOn(teamServiceApi, "save").mockReturnValue(pending);
  const onBusy = vi.fn();
  const view = render(<AdminTeamServices disabled={false} onBusy={onBusy} />);
  await userEvent.click(screen.getByText("Team service access on this machine"));
  await screen.findByLabelText("Reason for service change");
  await userEvent.type(screen.getByLabelText("Reason for service change"), "Keep local");
  await userEvent.dblClick(screen.getByRole("button", { name: "Save service access" }));
  expect(save).toHaveBeenCalledTimes(1);
  expect((screen.getByLabelText("Team to operate") as HTMLSelectElement).disabled).toBe(true);
  view.unmount();
  const calls = onBusy.mock.calls.length;
  await act(async () => { finish({ ...active, revision: 1 }); await pending; });
  expect(onBusy).toHaveBeenCalledTimes(calls);
});

it("shows useful suspended recovery actions and ignores a late response from another team", async () => {
  let finish!: (value: TeamService) => void;
  const pending = new Promise<TeamService>(resolve => { finish = resolve; });
  const read = vi.spyOn(teamServiceApi, "read").mockImplementation(async id => id === first.teamId ? pending : { ...second.service, reason: "Contact workspace operations" });
  const view = render(<TeamServiceStatus teamId={first.teamId} />);
  await waitFor(() => expect(read).toHaveBeenCalledWith(first.teamId));
  view.rerender(<TeamServiceStatus teamId={second.teamId} />);
  await screen.findByText("Managed execution is suspended");
  expect(screen.getByText(/export your work, or stop owned workloads/)).not.toBeNull();
  expect(screen.getByText("Operator note: Contact workspace operations")).not.toBeNull();
  await act(async () => { finish(active); await pending; });
  expect(screen.queryByText(/no subscription required/)).toBeNull();
  read.mockResolvedValue({ ...second.service, status: "active", revision: 8 });
  await userEvent.click(screen.getByRole("button", { name: "Refresh service access" }));
  await screen.findByText("Managed execution is active");
  expect(screen.queryByText(/New agent work/)).toBeNull();
});

it("keeps unreadable service access distinct from active access and offers retry", async () => {
  const read = vi.spyOn(teamServiceApi, "read").mockRejectedValue(new Error("Team access changed"));
  render(<TeamServiceStatus teamId={first.teamId} />);
  expect((await screen.findByRole("alert")).textContent).toContain("Team access changed");
  expect(screen.queryByText(/execution is active/)).toBeNull();
  read.mockResolvedValue(active);
  await userEvent.click(screen.getByRole("button", { name: "Refresh service access" }));
  await screen.findByText("Self-hosted team · no subscription required");
});
