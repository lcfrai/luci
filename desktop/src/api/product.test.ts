import { expect, it, vi } from "vitest";
import { ApiError } from "./client";
import { createProductClient } from "./product";
import { checkRunFixture } from "../test/checkFixtures";

it("binds a local commit to the exact source review and retains newer-source information", async () => {
  const result = { commit: "c".repeat(40), message: "Reviewed application", sourceChanged: true };
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(result), { status: 200 }));
  const client = createProductClient("http://localhost", fetcher);
  const expectedReviewDigest = "a".repeat(64);
  await expect(client.commit("p-1", result.message, expectedReviewDigest)).resolves.toEqual(result);
  expect(fetcher.mock.calls[0][0]).toBe("http://localhost/v1/product/projects/p-1/git/commit");
  expect(fetcher.mock.calls[0][1].method).toBe("POST");
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ message: result.message, expectedReviewDigest });
});

it("binds a checked local preview to its exact commit while preserving independent preview requests", async () => {
  const fetcher = vi.fn().mockImplementation(() => Promise.resolve(new Response("{}", { status: 200 })));
  const client = createProductClient("http://localhost", fetcher);
  const commit = "a".repeat(40);
  await client.release("p-1", 3000, "Dockerfile", commit);
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ target: "local", port: 3000, dockerfile: "Dockerfile", expectedCommit: commit });
  await client.release("p-1", 3000, "Dockerfile");
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ target: "local", port: 3000, dockerfile: "Dockerfile" });
});

it("sends exact requirement revision bindings for tasks and checks without changing generic check requests", async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response("{}", { status:200 }));
  const client = createProductClient("http://localhost", fetcher);
  await client.savePlan("p-1",3,[{id:"t1",title:"Offline",status:"todo",requirementIds:["R-OFFLINE"]}],6);
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({expectedRevision:3,tasks:[{id:"t1",title:"Offline",status:"todo",requirementIds:["R-OFFLINE"]}],expectedSpecificationRevision:6});
  fetcher.mockResolvedValue(new Response(JSON.stringify(checkRunFixture()), {status:200}));
  await client.runChecks("p-1",[{label:"Tests",executable:"npm",args:["test"]}],true,"docker",{requirementIds:["R-OFFLINE"],expectedSpecificationRevision:6,expectedDesignPlanRevision:8});
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toMatchObject({requirementIds:["R-OFFLINE"],expectedSpecificationRevision:6,expectedDesignPlanRevision:8,execution:"docker",acknowledgeHostExecution:true});
  fetcher.mockResolvedValue(new Response(JSON.stringify(checkRunFixture()), {status:200}));
  await client.runChecks("p-1",[{label:"Tests",executable:"npm",args:["test"]}],true);
  const generic = JSON.parse(fetcher.mock.calls[2][1].body);
  expect(generic).not.toHaveProperty("requirementIds");
  expect(generic).not.toHaveProperty("expectedSpecificationRevision");
});

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


it.each([202, 200])("binds cancellation to the exact observed revision and accepts explicit %s intent only", async status => {
  const response = { id: "release/a", projectId: "project/a", target: "local", revision: 7, cancelRequested: true, status: "cancelling" };
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), { status }));
  await expect(createProductClient("http://localhost", fetcher).cancelRelease("project/a", "release/a", 4)).resolves.toEqual(response);
  expect(fetcher).toHaveBeenCalledOnce();
  expect(fetcher.mock.calls[0][0]).toBe("http://localhost/v1/product/projects/project%2Fa/releases/release%2Fa/cancel");
  expect(fetcher.mock.calls[0][1].method).toBe("POST");
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ expectedRevision: 4 });
});
it.each([0, -1, 1.5, Number.NaN, Number.MAX_SAFE_INTEGER + 1])("refuses invalid cancellation revision %s before dispatch", async revision => {
  const fetcher = vi.fn();
  await expect(createProductClient("http://localhost", fetcher).cancelRelease("p", "r", revision)).rejects.toThrow(/Refresh this exact build/);
  expect(fetcher).not.toHaveBeenCalled();
});
it.each([{ projectId: "foreign" }, { id: "foreign" }, { target: "cloud" }, { revision: 4 }, { revision: null }, { cancelRequested: false }])("does not acknowledge mismatched cancellation result %j", async changed => {
  const response = { id: "r", projectId: "p", target: "local", revision: 5, cancelRequested: true, ...changed };
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), { status: 202 }));
  await expect(createProductClient("http://localhost", fetcher).cancelRelease("p", "r", 4)).rejects.toThrow(/could not be confirmed/);
  expect(fetcher).toHaveBeenCalledOnce();
});
