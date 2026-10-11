import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { type ProductClient, type RuntimeJob, type SetupStatus } from "../../api/product";
import { RuntimePreparation } from "./RuntimePreparation";
import { LocalTools } from "./LocalTools";

afterEach(() => vi.restoreAllMocks());

const status = { checks: [
  { id: "docker", label: "Docker", status: "ready", message: "Docker available" },
  { id: "sandbox-image", label: "Runtime", status: "missing", message: "Runtime missing" },
] } as SetupStatus;

function job(state: string, changes: Partial<RuntimeJob> = {}): RuntimeJob {
  return { id: "runtime-a", status: state, image: "luci/sandbox:dev", message: `Observed ${state}`, createdAt: "2026-10-11T00:00:00Z", updatedAt: "2026-10-11T00:00:00Z", ...changes };
}
function retained() {
  return { jobId: "runtime-a", allocationId: "allocation-a", imageId: "sha256:verified", phase: "unknown", resourcesHeld: false, cleanupConfirmed: true, message: "Image publication is unconfirmed" };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}
function clientFor(initial: RuntimeJob) {
  return {
    check: vi.fn().mockResolvedValue(status),
    runtime: vi.fn().mockResolvedValue(initial),
    installRuntime: vi.fn(),
    cancelRuntime: vi.fn(),
    reconcileRuntime: vi.fn(),
    gitIdentity: vi.fn().mockResolvedValue({ name: "Saved", email: "saved@example.invalid" }),
    agents: vi.fn().mockResolvedValue({ providers: [] }),
  } as unknown as ProductClient;
}
function controlledInterval() {
  let tick = () => {};
  const actualInterval = globalThis.setInterval;
  vi.spyOn(globalThis, "setInterval").mockImplementation((callback, delay, ...args) => {
    // Testing Library also uses intervals. Only replace the component's exact
    // poll; retain a real, harmless handle so its normal cleanup stays valid.
    if (delay === 3000) {
      tick = callback as () => void;
      return actualInterval(() => {}, 60_000);
    }
    return actualInterval(callback, delay, ...args);
  });
  return () => tick();
}

it("cancels the exact installation once and keeps polling until an unconfirmed publication is observed", async () => {
  const tick = controlledInterval();
  const client = clientFor(job("installing"));
  const cancelled = job("cancelling", { cancelRequested: true });
  vi.mocked(client.cancelRuntime).mockResolvedValue(cancelled);
  render(<RuntimePreparation client={client} manageBacking onStatus={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "Cancel runtime preparation" }));
  expect(client.cancelRuntime).toHaveBeenCalledExactlyOnceWith("runtime-a");
  expect((await screen.findByRole("button", { name: "Cancellation requested" })).hasAttribute("disabled")).toBe(true);
  expect(screen.queryByRole("button", { name: "Prepare agent runtime" })).toBeNull();
  vi.mocked(client.runtime).mockResolvedValue(job("unknown", { execution: retained() }));
  await act(async () => { tick(); });
  await screen.findByRole("button", { name: "Check interrupted installation" });
  expect(screen.queryByRole("button", { name: "Prepare agent runtime" })).toBeNull();
  expect(client.installRuntime).not.toHaveBeenCalled();
});

it("ignores a pre-cancellation poll that arrives after the cancellation response", async () => {
  const tick = controlledInterval();
  const stale = deferred<RuntimeJob>();
  const client = clientFor(job("installing"));
  vi.mocked(client.cancelRuntime).mockResolvedValue(job("cancelling", { cancelRequested: true }));
  render(<RuntimePreparation client={client} manageBacking onStatus={vi.fn()} />);
  await screen.findByRole("button", { name: "Cancel runtime preparation" });
  vi.mocked(client.runtime).mockReturnValueOnce(stale.promise);
  await act(async () => { tick(); });
  await userEvent.click(screen.getByRole("button", { name: "Cancel runtime preparation" }));
  await screen.findByRole("button", { name: "Cancellation requested" });
  await act(async () => { stale.resolve(job("installing")); await stale.promise; });
  expect(screen.getByRole("button", { name: "Cancellation requested" }).hasAttribute("disabled")).toBe(true);
  expect(screen.queryByRole("button", { name: "Cancel runtime preparation" })).toBeNull();
});

it("requires explicit reconciliation of an uncharged unknown receipt and never automatically rebuilds it", async () => {
  const client = clientFor(job("unknown", { execution: retained() }));
  vi.mocked(client.reconcileRuntime).mockResolvedValue(job("failed", { message: "Publication was not confirmed", execution: { ...retained(), phase: "failed" } }));
  render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "Check interrupted installation" }));
  expect(client.reconcileRuntime).toHaveBeenCalledExactlyOnceWith("runtime-a");
  await screen.findByRole("button", { name: "Retry runtime preparation" });
  expect(client.installRuntime).not.toHaveBeenCalled();
});

it("does not let cancellation from the previous workspace replace the newly selected workspace", async () => {
  const pending = deferred<RuntimeJob>();
  const old = clientFor(job("installing"));
  const current = clientFor(job("installing", { id: "runtime-b", message: "Current workspace build" }));
  vi.mocked(old.cancelRuntime).mockReturnValue(pending.promise);
  const view = render(<RuntimePreparation client={old} manageBacking onStatus={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "Cancel runtime preparation" }));
  view.rerender(<RuntimePreparation client={current} manageBacking onStatus={vi.fn()} />);
  await screen.findByText(/Current workspace build/);
  await act(async () => { pending.resolve(job("unknown", { execution: retained() })); await pending.promise; });
  expect(screen.queryByRole("button", { name: "Check interrupted installation" })).toBeNull();
  expect(screen.getByText(/Current workspace build/)).toBeTruthy();
  view.unmount();
  expect(old.cancelRuntime).toHaveBeenCalledTimes(1);
  expect(current.cancelRuntime).not.toHaveBeenCalled();
});

it("hides backing mutations from a viewer while retaining the observed installer state", async () => {
  const client = clientFor(job("installing"));
  render(<RuntimePreparation client={client} manageBacking={false} automatic onStatus={vi.fn()} />);
  await screen.findByText(/Observed installing/);
  expect(screen.queryByRole("button", { name: "Cancel runtime preparation" })).toBeNull();
  expect(client.installRuntime).not.toHaveBeenCalled();
  expect(client.cancelRuntime).not.toHaveBeenCalled();
});

it("the local tools view blocks a second build while an uncharged publication receipt awaits reconciliation", async () => {
  const client = clientFor(job("unknown", { execution: retained() }));
  render(<LocalTools client={client} manageBacking />);
  await userEvent.click(screen.getByText("Git identity, agent connection and runtime"));
  await screen.findByRole("button", { name: "Check interrupted installation" });
  const install = screen.getByRole("button", { name: "Install agent runtime" });
  await waitFor(() => expect(install.hasAttribute("disabled")).toBe(true));
  await userEvent.click(install);
  expect(client.installRuntime).not.toHaveBeenCalled();
});

it("removes the old backing's control targets before the new backing finishes its first observation", async () => {
  const old = clientFor(job("installing"));
  const pending = deferred<RuntimeJob>();
  const current = clientFor(job("installing", { id: "runtime-b" }));
  vi.mocked(current.runtime).mockReturnValue(pending.promise);
  const view = render(<RuntimePreparation client={old} manageBacking onStatus={vi.fn()} />);
  await screen.findByRole("button", { name: "Cancel runtime preparation" });
  view.rerender(<RuntimePreparation client={current} manageBacking onStatus={vi.fn()} />);
  expect(screen.queryByRole("button", { name: "Cancel runtime preparation" })).toBeNull();
  expect(current.cancelRuntime).not.toHaveBeenCalled();
  await act(async () => { pending.resolve(job("installing", { id: "runtime-b" })); await pending.promise; });
  await screen.findByRole("button", { name: "Cancel runtime preparation" });
});

it("refreshes actual image readiness after a direct ready reconciliation response", async () => {
  const client = clientFor(job("unknown", { execution: retained() }));
  const installed = { ...status, checks: status.checks.map((check) => check.id === "sandbox-image" ? { ...check, status: "ready", message: "Verified current image" } : check) } as SetupStatus;
  vi.mocked(client.check).mockResolvedValueOnce(status).mockResolvedValue(installed);
  vi.mocked(client.reconcileRuntime).mockResolvedValue(job("ready", { execution: { ...retained(), phase: "ready" } }));
  const onStatus = vi.fn();
  render(<RuntimePreparation client={client} manageBacking onStatus={onStatus} />);
  await userEvent.click(await screen.findByRole("button", { name: "Check interrupted installation" }));
  await screen.findByText("Verified current image");
  expect(client.check).toHaveBeenCalledTimes(2);
  expect(onStatus).toHaveBeenLastCalledWith(installed);
  expect(screen.queryByRole("button", { name: "Prepare agent runtime" })).toBeNull();
});
