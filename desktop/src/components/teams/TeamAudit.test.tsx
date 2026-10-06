import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { TeamAudit } from "./TeamAudit";
import { teamAuditApi, type TeamAuditResponse } from "../../api/teamAudit";
const result: TeamAuditResponse = { teamId: "team-one", events: [{ id: "event-one", at: "2026-10-06T00:00:00Z", actorId: "user-alice", action: "team.invitation.accept", target: "team-one", outcome: "success", actor: { id: "user-alice", displayName: "Alice" } }], limit: 200, installationRetentionLimit: 1000 };
beforeEach(() => { vi.restoreAllMocks(); vi.spyOn(teamAuditApi, "get").mockResolvedValue(result); });
it("reads on disclosure, names the actual actor and explains bounded retention", async () => {
  render(<TeamAudit teamId="team-one" actorId="admin" allowed />);
  expect(teamAuditApi.get).not.toHaveBeenCalled();
  await userEvent.click(screen.getByText("Team access and service history"));
  expect((await screen.findByText("Alice")).parentElement?.textContent).toContain("Joined through a team invitation");
  expect(screen.getByText(/1,000 retained management events/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh team activity" }));
  await screen.findByText("Alice"); expect(teamAuditApi.get).toHaveBeenCalledTimes(2);
});
it("hides revoked administrative access and ignores a late previous-team response", async () => {
  let complete!: (value: TeamAuditResponse) => void;
  vi.mocked(teamAuditApi.get).mockReturnValueOnce(new Promise(resolve => { complete = resolve; }));
  const view = render(<TeamAudit teamId="team-one" actorId="admin" allowed />);
  await userEvent.click(screen.getByText("Team access and service history"));
  view.rerender(<TeamAudit teamId="team-two" actorId="other-admin" allowed />);
  await act(async () => complete(result));
  expect(screen.queryByText("Alice")).toBeNull();
  view.rerender(<TeamAudit teamId="team-two" actorId="other-admin" allowed={false} />);
  expect(screen.queryByText("Team access and service history")).toBeNull();
});
it("reports a failed read without claiming the team has no activity", async () => {
  vi.mocked(teamAuditApi.get).mockRejectedValue(new Error("Your team access changed."));
  render(<TeamAudit teamId="team-one" actorId="admin" allowed />);
  await userEvent.click(screen.getByText("Team access and service history"));
  expect((await screen.findByRole("alert")).textContent).toContain("access changed");
  expect(screen.queryByText(/No retained management activity/)).toBeNull();
});
