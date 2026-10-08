import { StrictMode } from "react";
import { expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuntimePreparation } from "./RuntimePreparation";
import { createProductClient, type ProductClient, type RuntimeJob, type SetupStatus } from "../../api/product";

const ready = {
  checks: [
    {
      id: "git",
      label: "Git",
      status: "ready",
      message: "Git available",
      action: "",
    },
    {
      id: "docker",
      label: "Docker",
      status: "ready",
      message: "Docker running",
      action: "",
    },
    {
      id: "sandbox-image",
      label: "Agent runtime",
      status: "missing",
      message: "Image not built",
      action: "Prepare runtime",
    },
  ],
} as SetupStatus;
it("starts the supported runtime preparation once and displays its actual job state", async () => {
  const installRuntime = vi
    .fn()
    .mockResolvedValue({
      status: "installing",
      message: "Building packaged runtime",
    });
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <StrictMode>
      <RuntimePreparation
        client={client}
        manageBacking
        automatic
        onStatus={vi.fn()}
      />
    </StrictMode>,
  );
  await screen.findByText(/Installing runtime… Building packaged runtime/);
  expect(installRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByText(/100%/)).toBeNull();
});
it("does not install when Docker is unavailable and lets the user rerun readiness", async () => {
  const check = vi
    .fn()
    .mockResolvedValue({
      ...ready,
      checks: ready.checks.map((item) =>
        item.id === "docker"
          ? { ...item, status: "missing", message: "Docker stopped" }
          : item,
      ),
    });
  const installRuntime = vi.fn();
  const client = {
    check,
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking
      automatic
      onStatus={vi.fn()}
    />,
  );
  await screen.findByText(/Open Docker Desktop and wait for its engine to start/);
  expect(installRuntime).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Check again" }));
  await waitFor(() => expect(check).toHaveBeenCalledTimes(2));
});
it("retains a real failure and requires an explicit retry rather than repeating installation", async () => {
  const installRuntime = vi
    .fn()
    .mockRejectedValueOnce(new Error("Docker ran out of disk space"))
    .mockResolvedValue({ status: "installing", message: "Building" });
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking
      automatic
      onStatus={vi.fn()}
    />,
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "disk space",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Prepare agent runtime" }),
  );
  await screen.findByText(/Installing runtime… Building/);
  expect(installRuntime).toHaveBeenCalledTimes(2);
});
it("never installs a runtime for an account without backing permission", async () => {
  const installRuntime = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  render(
    <RuntimePreparation
      client={client}
      manageBacking={false}
      automatic
      onStatus={vi.fn()}
    />,
  );
  await screen.findByText("Docker running");
  expect(installRuntime).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: "Prepare agent runtime" }),
  ).toBeNull();
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

it("does not dispatch late automatic work after changing the selected client", async () => {
  const oldCheck = deferred<SetupStatus>();
  const oldInstall = vi.fn();
  const previous = {
    check: vi.fn().mockReturnValue(oldCheck.promise),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: oldInstall,
  } as unknown as ProductClient;
  const nextInstall = vi.fn().mockResolvedValue({ status: "installing", message: "Selected workspace build" });
  const selected = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: nextInstall,
  } as unknown as ProductClient;
  const { rerender } = render(<RuntimePreparation client={previous} manageBacking automatic onStatus={vi.fn()} />);
  rerender(<RuntimePreparation client={selected} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText(/Installing runtime… Selected workspace build/);
  await act(async () => { oldCheck.resolve(ready); await oldCheck.promise; });
  expect(oldInstall).not.toHaveBeenCalled();
  expect(nextInstall).toHaveBeenCalledOnce();
});

it("tracks automatic preparation separately for each selected client", async () => {
  const first = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "installing", message: "First workspace build" }),
  } as unknown as ProductClient;
  const second = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "installing", message: "Second workspace build" }),
  } as unknown as ProductClient;
  const { rerender } = render(<RuntimePreparation client={first} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText(/Installing runtime… First workspace build/);
  rerender(<RuntimePreparation client={second} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText(/Installing runtime… Second workspace build/);
  expect(first.installRuntime).toHaveBeenCalledOnce();
  expect(second.installRuntime).toHaveBeenCalledOnce();
});

it("does not start late automatic work after backing permission is revoked", async () => {
  const check = deferred<SetupStatus>();
  const installRuntime = vi.fn();
  const client = {
    check: vi.fn().mockReturnValue(check.promise),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime,
  } as unknown as ProductClient;
  const { rerender } = render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  rerender(<RuntimePreparation client={client} manageBacking={false} automatic onStatus={vi.fn()} />);
  await act(async () => { check.resolve(ready); await check.promise; });
  await screen.findByText("Docker running");
  expect(installRuntime).not.toHaveBeenCalled();
});

it("keeps a shared install handoff pending across a new readiness check and unlocks before its build completes", async () => {
  const acceptance = deferred<RuntimeJob>();
  const pending = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: vi.fn().mockReturnValue(acceptance.promise),
  } as unknown as ProductClient;
  const { rerender } = render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} onPendingChange={pending} />);
  await waitFor(() => expect(client.installRuntime).toHaveBeenCalledOnce());
  const originalSignal = vi.mocked(client.check).mock.calls[0][0]!;
  rerender(<RuntimePreparation client={client} manageBacking automatic refreshToken={1} onStatus={vi.fn()} onPendingChange={pending} />);
  await waitFor(() => expect(client.check).toHaveBeenCalledTimes(2));
  expect(originalSignal.aborted).toBe(true);
  expect(vi.mocked(client.check).mock.calls[1][0]!.aborted).toBe(false);
  expect(pending).toHaveBeenLastCalledWith(true);
  await act(async () => { acceptance.resolve({ status: "installing", message: "Backend build continues" } as RuntimeJob); await acceptance.promise; });
  await screen.findByText(/Installing runtime… Backend build continues/);
  expect(client.installRuntime).toHaveBeenCalledOnce();
  expect(client.installRuntime).toHaveBeenCalledWith();
  expect(pending).toHaveBeenLastCalledWith(false);
});

it("aborts abandoned HTTP readiness and runtime observations when the workspace changes", async () => {
  const abandoned: AbortSignal[] = [];
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    const signal = init!.signal!;
    abandoned.push(signal);
    return new Promise<Response>((_resolve, reject) => {
      signal.addEventListener("abort", () => reject(signal.reason), { once: true });
    });
  });
  const previous = createProductClient("http://localhost", fetcher);
  const onStatus = vi.fn();
  const selected = {
    check: vi.fn().mockResolvedValue(ready),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: vi.fn(),
  } as unknown as ProductClient;
  const view = render(<RuntimePreparation client={previous} manageBacking={false} onStatus={onStatus} />);
  expect(abandoned).toHaveLength(2);
  view.rerender(<RuntimePreparation client={selected} manageBacking={false} onStatus={onStatus} />);
  await screen.findByText("Docker running");
  expect(abandoned.every(signal => signal.aborted)).toBe(true);
  expect(onStatus).toHaveBeenCalledOnce();
  expect(onStatus).toHaveBeenLastCalledWith(ready);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(selected.installRuntime).not.toHaveBeenCalled();
});

it("cancels an in-flight completed-job observation on unmount without cancelling its build", async () => {
  vi.useFakeTimers();
  const imageProof = deferred<SetupStatus>();
  const onStatus = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValueOnce(ready).mockReturnValue(imageProof.promise),
    runtime: vi.fn().mockResolvedValueOnce({ status: "idle" }).mockResolvedValue({ status: "ready" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "installing", message: "Backend build" }),
  } as unknown as ProductClient;
  const view = render(<RuntimePreparation client={client} manageBacking automatic onStatus={onStatus} />);
  try {
    await act(async () => {});
    await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
    const pollSignal = vi.mocked(client.check).mock.calls[1][0]!;
    expect(pollSignal.aborted).toBe(false);
    view.unmount();
    expect(pollSignal.aborted).toBe(true);
    await act(async () => { imageProof.resolve(ready); await imageProof.promise; });
    expect(onStatus).toHaveBeenCalledOnce();
    expect(client.installRuntime).toHaveBeenCalledWith();
    expect(vi.getTimerCount()).toBe(0);
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});

it("cancels an older job poll when a fresh readiness check replaces it", async () => {
  vi.useFakeTimers();
  const oldImage = deferred<SetupStatus>();
  const current = { ...ready, checks: ready.checks.map(item => item.id === "sandbox-image"
    ? { ...item, status: "ready", message: "Fresh image verified" } : item) } as SetupStatus;
  const onStatus = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValueOnce(ready).mockReturnValueOnce(oldImage.promise).mockResolvedValue(current),
    runtime: vi.fn().mockResolvedValueOnce({ status: "idle" }).mockResolvedValue({ status: "ready" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "installing", message: "Backend build" }),
  } as unknown as ProductClient;
  const view = render(<RuntimePreparation client={client} manageBacking automatic onStatus={onStatus} />);
  try {
    await act(async () => {});
    await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
    const oldPoll = vi.mocked(client.check).mock.calls[1][0]!;
    view.rerender(<RuntimePreparation client={client} manageBacking automatic refreshToken={1} onStatus={onStatus} />);
    await act(async () => {});
    expect(oldPoll.aborted).toBe(true);
    expect(screen.getByText("Fresh image verified")).not.toBeNull();
    await act(async () => { oldImage.resolve(ready); await oldImage.promise; });
    expect(onStatus).toHaveBeenCalledTimes(2);
    expect(onStatus).toHaveBeenLastCalledWith(current);
    expect(screen.getByText("Fresh image verified")).not.toBeNull();
    expect(client.installRuntime).toHaveBeenCalledOnce();
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});

it.each([true, false])("checks actual image readiness once when the job finished after its first snapshot (image exists: %s)", async (imageExists) => {
  const current = {
    ...ready,
    checks: ready.checks.map(item => item.id === "sandbox-image" ? {
      ...item, status: imageExists ? "ready" : "missing", message: imageExists ? "Actual image verified" : "Actual image still missing",
    } : item),
  } as SetupStatus;
  const onStatus = vi.fn();
  const client = {
    check: vi.fn().mockResolvedValueOnce(ready).mockResolvedValue(current),
    runtime: vi.fn().mockResolvedValue({ status: "ready" }),
    installRuntime: vi.fn(),
  } as unknown as ProductClient;
  render(<RuntimePreparation client={client} manageBacking automatic onStatus={onStatus} />);
  await screen.findByText(imageExists ? "Actual image verified" : "Actual image still missing");
  expect(client.check).toHaveBeenCalledTimes(2);
  expect(client.installRuntime).not.toHaveBeenCalled();
  expect(onStatus).toHaveBeenLastCalledWith(current);
  expect(!!screen.queryByRole("button", { name: "Prepare agent runtime" })).toBe(!imageExists);
});

it("checks actual image availability after an install request reports ready", async () => {
  const current = { ...ready, checks: ready.checks.map(item => item.id === "sandbox-image" ? { ...item, status: "ready", message: "Installed image verified" } : item) } as SetupStatus;
  const client = {
    check: vi.fn().mockResolvedValueOnce(ready).mockResolvedValue(current),
    runtime: vi.fn().mockResolvedValue({ status: "idle" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "ready" }),
  } as unknown as ProductClient;
  render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText("Installed image verified");
  expect(client.check).toHaveBeenCalledTimes(2);
  expect(client.installRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByRole("button", { name: "Prepare agent runtime" })).toBeNull();
});

it("applies the final actual image check before the completed-job poll tears down", async () => {
  vi.useFakeTimers();
  const imageProof = deferred<SetupStatus>();
  const verified = { ...ready, checks: ready.checks.map(item => item.id === "sandbox-image" ? { ...item, status: "ready", message: "Completed image verified" } : item) } as SetupStatus;
  const client = {
    check: vi.fn().mockResolvedValueOnce(ready).mockReturnValue(imageProof.promise),
    runtime: vi.fn().mockResolvedValueOnce({ status: "idle" }).mockResolvedValue({ status: "ready" }),
    installRuntime: vi.fn().mockResolvedValue({ status: "installing", message: "Backend build" }),
  } as unknown as ProductClient;
  const view = render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  try {
    await act(async () => {});
    expect(screen.getByText(/Installing runtime… Backend build/)).not.toBeNull();
    await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
    expect(client.check).toHaveBeenCalledTimes(2);
    expect(screen.getByText(/Installing runtime… Backend build/)).not.toBeNull();
    await act(async () => { await vi.advanceTimersByTimeAsync(6000); });
    expect(client.check).toHaveBeenCalledTimes(2);
    expect(client.runtime).toHaveBeenCalledTimes(2);
    await act(async () => { imageProof.resolve(verified); await imageProof.promise; });
    expect(screen.getByText("Completed image verified")).not.toBeNull();
    expect(screen.queryByText(/Installing runtime…/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Prepare agent runtime" })).toBeNull();
    await act(async () => { await vi.advanceTimersByTimeAsync(6000); });
    expect(client.check).toHaveBeenCalledTimes(2);
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});

it('keeps planning available while Windows storage blocks automatic and manual new runtime preparation', async () => {
  const storage = { id: 'docker-backing-storage', label: 'Docker backing storage', status: 'missing', message: 'Confirm the current storage layout in Settings.', action: 'Review Docker storage.' };
  const blocked = { ...ready, platform: 'windows', checks: [...ready.checks, storage] } as SetupStatus;
  const client = { check: vi.fn().mockResolvedValue(blocked), runtime: vi.fn().mockResolvedValue({ status: 'idle' }), installRuntime: vi.fn().mockResolvedValue({ status: 'installing', message: 'Preparing after storage passed.' }) } as unknown as ProductClient;
  const view = render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText('Docker backing storage');
  expect(client.installRuntime).not.toHaveBeenCalled();
  expect((screen.getByRole('button', { name: 'Prepare agent runtime' }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/you can keep planning while new runtime preparation waits/)).toBeTruthy();
  vi.mocked(client.check).mockResolvedValue({ ...blocked, checks: blocked.checks.map(item => item.id === storage.id ? { ...item, status: 'ready', message: 'Fresh confirmed capacity observed.' } : item) });
  view.rerender(<RuntimePreparation client={client} manageBacking automatic refreshToken={1} onStatus={vi.fn()} />);
  await screen.findByText(/Installing runtime… Preparing after storage passed/);
  expect(client.installRuntime).toHaveBeenCalledOnce();
});
it('does not treat a missing Windows storage readiness check as authorization to install', async () => {
  const client = { check: vi.fn().mockResolvedValue({ ...ready, platform: 'windows' }), runtime: vi.fn().mockResolvedValue({ status: 'idle' }), installRuntime: vi.fn() } as unknown as ProductClient;
  render(<RuntimePreparation client={client} manageBacking automatic onStatus={vi.fn()} />);
  await screen.findByText(/Check Docker storage readiness before preparing/);
  expect(client.installRuntime).not.toHaveBeenCalled();
});

it('invalidates an older readiness response without probing again or starting runtime preparation', async () => {
  let complete!: (value:SetupStatus)=>void;
  const storageReady={...ready,platform:'windows',checks:[...ready.checks,{id:'docker-backing-storage',label:'Storage',status:'ready',message:'Earlier storage check.',action:''}]} as SetupStatus;
  const stale={...storageReady,checks:storageReady.checks.map(check=>check.id==='docker-backing-storage'?{...check,status:'unknown' as const,message:'Storage changed; check readiness explicitly.'}:check)};
  const client={check:vi.fn().mockImplementationOnce(()=>new Promise(resolve=>{complete=resolve;})).mockResolvedValue(storageReady),runtime:vi.fn().mockResolvedValue({status:'idle'}),installRuntime:vi.fn().mockResolvedValue({status:'installing',message:'Explicit fresh readiness accepted.'})} as unknown as ProductClient;
  const onStatus=vi.fn(),pending=vi.fn();
  const view=render(<RuntimePreparation client={client} manageBacking automatic storageRevision={0} statusSnapshot={storageReady} onStatus={onStatus} onPendingChange={pending}/>);
  view.rerender(<RuntimePreparation client={client} manageBacking automatic storageRevision={1} statusSnapshot={stale} onStatus={onStatus} onPendingChange={pending}/>);
  await act(async()=>complete(storageReady));
  expect(client.check).toHaveBeenCalledOnce();expect(client.installRuntime).not.toHaveBeenCalled();expect(onStatus).not.toHaveBeenCalled();
  expect(pending).toHaveBeenLastCalledWith(false);
  expect(screen.getByRole('button',{name:'Prepare agent runtime'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Check again'}));
  await waitFor(()=>expect(client.check).toHaveBeenCalledTimes(2));
  expect(onStatus).toHaveBeenCalledWith(storageReady,1);expect(client.installRuntime).toHaveBeenCalledOnce();
});

it('honors parent invalidation before new storage props have rendered', async () => {
  let complete!:(value:SetupStatus)=>void,currentStorageRevision=0;
  const client={check:vi.fn().mockImplementation(()=>new Promise(resolve=>{complete=resolve;})),runtime:vi.fn().mockResolvedValue({status:'idle'}),installRuntime:vi.fn()} as unknown as ProductClient;
  const receive=vi.fn((_status:SetupStatus,checkedRevision?:number)=>checkedRevision===currentStorageRevision);
  render(<RuntimePreparation client={client} manageBacking automatic storageRevision={0} onStatus={receive}/>);
  currentStorageRevision=1;
  await act(async()=>complete(ready));
  expect(receive).toHaveBeenCalledWith(ready,0);
  expect(client.check).toHaveBeenCalledOnce();expect(client.installRuntime).not.toHaveBeenCalled();
});
