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

const pendingBuild = () => ({ id: "older-active", state: "cancelled", valid: false, reason: "Future dispatch cancelled", activity: "running" as const, dispatchCancelled: true,
  tasks: [{ id: "task-a", title: "Private booking", status: "selected" }], operations: [{ id: "op-a", childId: "session-a", status: "running" as const, updatedAt: "2026-10-06T14:00:00Z" }] });
it("retains an older pending build separately from the newest prepared run without observing or changing it", async () => {
  const snapshot = context(); snapshot.currentRun = { id: "new-ready", state: "ready", valid: false, reason: "Prepared", activity: "none", dispatchCancelled: false, operations: [] };
  snapshot.activeRuns = [pendingBuild()];
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot }));
  const result = await orchestratorApi.get("p-1");
  expect(result.context.currentRun?.id).toBe("new-ready");
  expect(result.context.activeRuns).toEqual([pendingBuild()]);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator");
});
it("preserves bounded scope and activity omission explicitly while retaining run identity", async () => {
  const snapshot = context();
  snapshot.activeRuns = [{ ...pendingBuild(), scopeOmitted: true, taskCount: 80, requirementCount: 42, tasks: [], requirements: [], operationsOmitted: true, operationCount: 3, operations: [] }];
  vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot }));
  expect((await orchestratorApi.get("p-1")).context.activeRuns).toEqual(snapshot.activeRuns);
});
it("accepts the authoritative active-run next action and rejects a target absent from this project's projection", async () => {
  const snapshot = context(); snapshot.activeRuns = [pendingBuild()];
  const nextAction = { id: "inspect-run", label: "Inspect active builds", reason: "Recorded running activity", lens: "develop" as const, runId: pendingBuild().id };
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot, nextAction }));
  expect((await orchestratorApi.get("p-1")).nextAction.runId).toBe(pendingBuild().id);
  request.mockResolvedValue(view({ context: snapshot, nextAction: { ...nextAction, runId: "unrelated-run" } }));
  await expect(orchestratorApi.get("p-1")).rejects.toThrow("could not be verified");
});
it.each([
  null, [pendingBuild(), pendingBuild()], [{ ...pendingBuild(), activity: "none" }],
  [{ ...pendingBuild(), dispatchCancelled: undefined }], [{ ...pendingBuild(), operations: [] }],
  [{ ...pendingBuild(), operations: [null] }],
  [{ ...pendingBuild(), operations: [{ ...pendingBuild().operations[0], status: "succeeded" }] }],
  [{ ...pendingBuild(), operations: [{ ...pendingBuild().operations[0], updatedAt: "invalid" }] }],
])("rejects malformed active activity instead of presenting misleading execution state", async activeRuns => {
  vi.spyOn(productApi, "request").mockResolvedValue(view({ context: { ...context(), activeRuns } as never }));
  await expect(orchestratorApi.get("p-1")).rejects.toThrow("could not be verified");
});
