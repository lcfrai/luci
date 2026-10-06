import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { orchestratorApi } from "./orchestrator";
import { context, job, turn, view } from "../components/orchestrator/fixtures";
afterEach(() => vi.restoreAllMocks());

it("keeps encoded routes and explicit CAS/idempotency for send/apply, with observation separate", async () => {
  const projectId = "project/space", result = view({ projectId, context: context(projectId) });
  const request = vi.spyOn(productApi, "request").mockResolvedValue(result);
  await orchestratorApi.get(projectId);
  await orchestratorApi.get(projectId, 21);
  request.mockResolvedValue(job({ projectId, orchestratorContext: context(projectId) }));
  const input = { expectedRevision: 3, expectedContextDigest: "digest", idempotencyKey: "request-1", message: "My idea", harness: "codex" as const, agentConnectionId: "saved", replyToTurnId: "turn-1" };
  await orchestratorApi.send(projectId, input);
  await orchestratorApi.job(projectId, "job/1");
  await orchestratorApi.cancel(projectId, "job/1");
  await orchestratorApi.reconcile(projectId, "job/1");
  request.mockResolvedValue(result);
  await orchestratorApi.apply(projectId, "turn/1", { expectedRevision: 4, expectedContextDigest: "reviewed" });
  expect(request.mock.calls).toEqual([
    ["/v1/product/projects/project%2Fspace/orchestrator"], ["/v1/product/projects/project%2Fspace/orchestrator?before=21"],
    ["/v1/product/projects/project%2Fspace/assist/orchestrator", input], ["/v1/product/projects/project%2Fspace/assist/job%2F1"],
    ["/v1/product/projects/project%2Fspace/assist/job%2F1/cancel", {}], ["/v1/product/projects/project%2Fspace/assist/job%2F1/reconcile", {}],
    ["/v1/product/projects/project%2Fspace/orchestrator/turns/turn%2F1/apply", { expectedRevision: 4, expectedContextDigest: "reviewed" }],
  ]);
});
it("exports the private ledger and requires its reviewed storage digest for rotation", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue({ version: 1, projectId: "p-1", storageDigest: "saved", ledger: {} });
  await orchestratorApi.export("p-1");
  request.mockResolvedValue(view());
  await orchestratorApi.rotate("p-1", { expectedRevision: 3, expectedStorageDigest: "saved", acknowledgeExportedHistory: true });
  expect(request.mock.calls).toEqual([["/v1/product/projects/p-1/orchestrator/export"], ["/v1/product/projects/p-1/orchestrator/rotate", { expectedRevision: 3, expectedStorageDigest: "saved", acknowledgeExportedHistory: true }]]);
  request.mockResolvedValue({ version: 1, projectId: "other", storageDigest: "saved", ledger: {} });
  await expect(orchestratorApi.export("p-1")).rejects.toThrow("could not be verified");
});
it.each([
  view({ projectId: "other" }), view({ context: context("other") }), view({ turns: [turn({ projectId: "other" })] }),
  view({ turns: [turn({ context: context("other") })] }), view({ turns: [turn(), turn()] }),
])("rejects foreign or duplicate conversation content before rendering", async value => {
  vi.spyOn(productApi, "request").mockResolvedValue(value);
  await expect(orchestratorApi.get("p-1")).rejects.toThrow("could not be verified");
});
it("rejects jobs from another project or assistance kind", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(job({ projectId: "other" }));
  await expect(orchestratorApi.job("p-1", "job-2")).rejects.toThrow("different conversation");
  request.mockResolvedValue({ ...job(), kind: "interview" });
  await expect(orchestratorApi.job("p-1", "job-2")).rejects.toThrow("different conversation");
});
