import { expect, it, vi } from "vitest";
import { ApiError } from "./client";
import { createProductClient } from "./product";

it("cancels an in-flight readiness response without reporting a service failure", async () => {
  let transportSignal: AbortSignal | undefined;
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    transportSignal = init?.signal ?? undefined;
    return new Promise<Response>((_resolve, reject) => {
      transportSignal?.addEventListener("abort", () => reject(transportSignal?.reason), { once: true });
    });
  });
  const selected = new AbortController();
  const result = createProductClient("http://localhost", fetcher).check(selected.signal);
  const outcome = result.catch((cause: unknown) => cause);
  selected.abort();
  expect(await outcome).toBe(selected.signal.reason);
  expect(transportSignal?.aborted).toBe(true);
  expect(fetcher).toHaveBeenCalledOnce();
});

it("does not dispatch a runtime observation for an already abandoned workspace", async () => {
  const fetcher = vi.fn();
  const selected = new AbortController();
  selected.abort();
  await expect(createProductClient("http://localhost", fetcher).runtime(selected.signal))
    .rejects.toBe(selected.signal.reason);
  expect(fetcher).not.toHaveBeenCalled();
});

it("cancels a stalled response body as well as its initial request", async () => {
  let observed!: AbortSignal;
  let body!: ReadableStream<Uint8Array>;
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    observed = init!.signal!;
    body = new ReadableStream<Uint8Array>({
      start(stream) {
        observed.addEventListener("abort", () => stream.error(observed.reason), { once: true });
      },
    });
    return Promise.resolve(new Response(body, { status: 200 }));
  });
  const selected = new AbortController();
  const result = createProductClient("http://localhost", fetcher).check(selected.signal);
  const outcome = result.catch((cause: unknown) => cause);
  await Promise.resolve();
  expect(body.locked).toBe(true);
  selected.abort();
  expect(await outcome).toBe(selected.signal.reason);
  expect(observed.aborted).toBe(true);
});

it("retains its timeout and releases the caller listener when the service stalls", async () => {
  vi.useFakeTimers();
  const selected = new AbortController();
  const remove = vi.spyOn(selected.signal, "removeEventListener");
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
    init?.signal?.addEventListener("abort", () => reject(init.signal?.reason), { once: true });
  }));
  try {
    const result = createProductClient("http://localhost", fetcher).check(selected.signal);
    const timedOut = expect(result).rejects.toMatchObject({ code: "timeout" } satisfies Partial<ApiError>);
    await vi.advanceTimersByTimeAsync(120_000);
    await timedOut;
    expect(selected.signal.aborted).toBe(false);
    expect(remove).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  } finally {
    vi.useRealTimers();
  }
});
