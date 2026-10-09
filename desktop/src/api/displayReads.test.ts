import { afterEach, expect, it, vi } from "vitest";
import { definitionApi } from "./definition";
import { orchestratorApi } from "./orchestrator";
import { createProductClient, productApi } from "./product";
import { context, view } from "../components/orchestrator/fixtures";

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });
const displayRead = (kind: "definition" | "orchestrator", signal: AbortSignal) => kind === "definition"
  ? definitionApi.get("project/space", signal)
  : orchestratorApi.get("project/space", 21, signal);

it("forwards display cancellation without changing the definition route or the optional legacy request shape", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(context("project/space").specification);
  const controller = new AbortController();
  await definitionApi.get("project/space", controller.signal);
  await definitionApi.get("project/space");
  expect(request.mock.calls).toEqual([
    ["/v1/product/projects/project%2Fspace/specification", undefined, "GET", 120_000, controller.signal],
    ["/v1/product/projects/project%2Fspace/specification"],
  ]);
});

it("preserves orchestrator pagination and project validation when an optional display signal is supplied", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view({ projectId: "project/space", context: context("project/space") }));
  const controller = new AbortController();
  await orchestratorApi.get("project/space", 21, controller.signal);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/project%2Fspace/orchestrator?before=21", undefined, "GET", 120_000, controller.signal);
  request.mockResolvedValue(view());
  await expect(orchestratorApi.get("project/space", undefined, controller.signal)).rejects.toThrow("could not be verified");
});

it.each(["definition", "orchestrator"] as const)("cancels the actual %s transport after headers while its body is stalled", async kind => {
  let observed!: AbortSignal; let body!: ReadableStream<Uint8Array>;
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    observed = init!.signal!;
    body = new ReadableStream<Uint8Array>({ start(stream) {
      observed.addEventListener("abort", () => stream.error(observed.reason), { once: true });
    } });
    return Promise.resolve(new Response(body, { status: 200 }));
  });
  vi.spyOn(productApi, "request").mockImplementation(createProductClient("http://localhost", fetcher).request);
  const controller = new AbortController(); const remove = vi.spyOn(controller.signal, "removeEventListener");
  const pending = displayRead(kind, controller.signal); const outcome = pending.catch((cause: unknown) => cause);
  await Promise.resolve(); expect(body.locked).toBe(true);
  controller.abort(); expect(await outcome).toBe(controller.signal.reason);
  expect(observed.aborted).toBe(true); expect(fetcher).toHaveBeenCalledOnce(); expect(remove).toHaveBeenCalledOnce();
});

it.each(["definition", "orchestrator"] as const)("does not start an already abandoned %s read", async kind => {
  const fetcher = vi.fn();
  vi.spyOn(productApi, "request").mockImplementation(createProductClient("http://localhost", fetcher).request);
  const controller = new AbortController(); controller.abort();
  await expect(displayRead(kind, controller.signal)).rejects.toBe(controller.signal.reason);
  expect(fetcher).not.toHaveBeenCalled();
});

it.each(["definition", "orchestrator"] as const)("keeps the original 120-second %s timeout independent from caller cancellation", async kind => {
  vi.useFakeTimers();
  const fetcher = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
    init!.signal!.addEventListener("abort", () => reject(init!.signal!.reason), { once: true });
  }));
  vi.spyOn(productApi, "request").mockImplementation(createProductClient("http://localhost", fetcher).request);
  const controller = new AbortController(); const remove = vi.spyOn(controller.signal, "removeEventListener");
  const pending = expect(displayRead(kind, controller.signal)).rejects.toMatchObject({ code: "timeout", status: 0 });
  await vi.advanceTimersByTimeAsync(120_000); await pending;
  expect(controller.signal.aborted).toBe(false); expect(remove).toHaveBeenCalledOnce(); expect(vi.getTimerCount()).toBe(0);
});
