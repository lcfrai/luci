import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import type { ProductClient, RuntimeAdmissionObservation, RuntimeJob, SetupStatus } from "../../api/product";
import { RuntimeAdmission } from "./RuntimeAdmission";
import { RuntimePreparation } from "./RuntimePreparation";
import { LocalTools } from "./LocalTools";

const observation: RuntimeAdmissionObservation = {
  stage: "initial", reasonCode: "resource_observation_unavailable", decidedAt: "2026-10-12T00:01:00Z",
  samplerAvailable: true, hostFresh: false, executionFresh: true,
  hostMemoryKnown: true, hostCPUKnown: true, executionMemoryKnown: true, executionCPUKnown: true,
  inventoryComplete: true, engineMatched: true,
};
const status = { checks: [
  { id: "docker", label: "Docker", status: "ready", message: "Docker available" },
  { id: "sandbox-image", label: "Runtime", status: "missing", message: "Runtime missing" },
] } as SetupStatus;
function clientFor(admission: RuntimeAdmissionObservation | undefined = observation) {
  const runtime: RuntimeJob = { id: "runtime-a", status: "blocked", image: "fixture", message: "Preparation paused", createdAt: observation.decidedAt, updatedAt: observation.decidedAt,
    execution: { jobId: "runtime-a", phase: "blocked", resourcesHeld: false, cleanupConfirmed: true, message: "Original decision", admission } };
  return { check: vi.fn().mockResolvedValue(status), runtime: vi.fn().mockResolvedValue(runtime),
    installRuntime: vi.fn(), cancelRuntime: vi.fn(), reconcileRuntime: vi.fn(),
    gitIdentity: vi.fn().mockResolvedValue({ name: "Fixture", email: "fixture@example.invalid" }),
    agents: vi.fn().mockResolvedValue({ providers: [] }) } as unknown as ProductClient;
}
function unchanged(client: ProductClient, baseline: number[]) {
  expect([vi.mocked(client.check).mock.calls.length, vi.mocked(client.runtime).mock.calls.length]).toEqual(baseline);
  expect(client.installRuntime).not.toHaveBeenCalled();
  expect(client.cancelRuntime).not.toHaveBeenCalled();
  expect(client.reconcileRuntime).not.toHaveBeenCalled();
}

it.each(["preparation", "local tools"])("reading %s refusal evidence does not start probes, retry or reconciliation", async view => {
  const client = clientFor();
  render(view === "preparation" ? <RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} /> : <LocalTools client={client} manageBacking />);
  if (view === "local tools") await userEvent.click(screen.getByText("Git identity, agent connection and runtime"));
  const summary = await screen.findByText("Why preparation paused");
  const baseline = [vi.mocked(client.check).mock.calls.length, vi.mocked(client.runtime).mock.calls.length];
  await userEvent.click(summary);
  expect(summary.closest("details")?.open).toBe(true);
  expect(screen.getByText("The computer capacity observation was missing, expired or ahead of the decision time.")).toBeTruthy();
  expect(screen.queryByText("The Docker workload inventory was incomplete.")).toBeNull();
  expect(screen.getByText(/saved decision, not current capacity/)).toBeTruthy();
  await userEvent.click(summary);
  unchanged(client, baseline);
});

it("hides diagnostics from backing viewers and drops the old workspace receipt before a new read settles", async () => {
  const old = clientFor();
  const current = clientFor();
  let resolve!: (value: RuntimeJob) => void;
  vi.mocked(current.runtime).mockReturnValue(new Promise(done => { resolve = done; }));
  const view = render(<RuntimePreparation client={old} manageBacking onStatus={vi.fn()} />);
  await screen.findByText("Why preparation paused");
  view.rerender(<RuntimePreparation client={old} manageBacking={false} onStatus={vi.fn()} />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  view.rerender(<RuntimePreparation client={current} manageBacking onStatus={vi.fn()} />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  await act(async () => { resolve({ id: "runtime-b", status: "idle", image: "fixture", message: "Current workspace", createdAt: observation.decidedAt, updatedAt: observation.decidedAt }); });
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  expect(old.installRuntime).not.toHaveBeenCalled();
  expect(current.installRuntime).not.toHaveBeenCalled();
});

it("distinguishes absent monitoring from a CPU refusal and the final build check", async () => {
  const view = render(<RuntimeAdmission observation={{ ...observation, samplerAvailable: false, hostMemoryKnown: false }} />);
  await userEvent.click(screen.getByText("Why preparation paused"));
  expect(screen.getByText("The capacity monitor had no observation available.")).toBeTruthy();
  expect(screen.queryByText("Available computer memory could not be measured.")).toBeNull();
  view.rerender(<RuntimeAdmission observation={{ ...observation, hostFresh: true, stage: "before-build", reasonCode: "cpu_pressure" }} />);
  expect(screen.getByText("The observed CPU load exceeded the configured headroom limit.")).toBeTruthy();
  expect(screen.getByText(/at the final check before building/)).toBeTruthy();
  expect(screen.queryByText(/missing, expired/)).toBeNull();
});

it("does not invent a historical cause for older or malformed receipts", () => {
  const view = render(<RuntimeAdmission />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  view.rerender(<RuntimeAdmission observation={{ ...observation, decidedAt: "not-a-date" }} />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  view.rerender(<RuntimeAdmission observation={{ ...observation, stage: "future-stage" as "initial" }} />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
  view.rerender(<RuntimeAdmission observation={{ ...observation, hostFresh: true, reasonCode: "unrecognized" }} />);
  expect(screen.queryByText("Why preparation paused")).toBeNull();
});
