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

it("accepts an older review obligation as the exact next action without inventing activity", async () => {
  const snapshot = context();
  snapshot.currentRun = { id: "latest-ready", state: "ready", valid: false, reason: "Prepared", activity: "none" };
  snapshot.activeRuns = [];
  snapshot.needsAttention = [{ id: "older-review", state: "awaiting-checks", kind: "task-review", contextStatus: "changed" }];
  const nextAction = { id: "inspect-run", label: "Review earlier task changes", reason: "Recorded obligation", lens: "develop" as const, runId: "older-review" };
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot, nextAction }));
  const result = await orchestratorApi.get("p-1");
  expect(result.nextAction.runId).toBe("older-review");
  expect(result.context.activeRuns).toEqual([]);
  expect(result.context.needsAttention).toEqual(snapshot.needsAttention);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator");
  request.mockResolvedValue(view({ context: snapshot, nextAction: { ...nextAction, runId: "outside-project-projection" } }));
  await expect(orchestratorApi.get("p-1")).rejects.toThrow("could not be verified");
});
it("retains unintegrated task output after future dispatch is cancelled", async () => {
  const snapshot = context();
  snapshot.needsAttention = [{ id: "cancelled-parent", state: "cancelled", kind: "task-review", contextStatus: "current" }];
  vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot }));
  expect((await orchestratorApi.get("p-1")).context.needsAttention).toEqual(snapshot.needsAttention);
});
it.each(["complete", "cancelled"])("retains unanswered agent questions from a %s build as exact review navigation", async state => {
  const snapshot = context();
  snapshot.needsAttention = [{ id: "retained-question-run", state, kind: "task-question", contextStatus: "changed" }];
  vi.spyOn(productApi, "request").mockResolvedValue(view({ context: snapshot, nextAction: { id: "inspect-run", label: "Review reported task questions", reason: "Retained reported uncertainty", lens: "develop", runId: "retained-question-run" } }));
  const result = await orchestratorApi.get("p-1");
  expect(result.nextAction.runId).toBe("retained-question-run");
  expect(result.context.needsAttention).toEqual(snapshot.needsAttention);
});
const reviewObligation = { id: "earlier", state: "awaiting-checks", kind: "verification", contextStatus: "current" };
it.each([
  null, [null], [reviewObligation, reviewObligation], [{ ...reviewObligation, id: "" }],
  [{ ...reviewObligation, state: "complete" }], [{ ...reviewObligation, state: "cancelled" }],
  [{ ...reviewObligation, kind: "running" }], [{ ...reviewObligation, contextStatus: "verified" }],
  Array.from({ length: 201 }, (_, index) => ({ ...reviewObligation, id: String(index) })),
])("rejects invalid review projections without treating their identities as navigation authority", async needsAttention => {
  vi.spyOn(productApi, "request").mockResolvedValue(view({ context: { ...context(), needsAttention } as never }));
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

it("resolves only the original scoped request and rejects a mismatched or unsupported receipt", async () => {
  const input = { expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: "retained-original-key", message: "Original", harness: "codex" as const, model: "original-model" };
  const request = vi.spyOn(productApi, "request").mockResolvedValue({ projectId: "p-1", status: "accepted", job: job(), message: "Already accepted" });
  expect((await orchestratorApi.resolveRequest("p-1", input)).job?.id).toBe(job().id);
  expect(request).toHaveBeenCalledWith("/v1/product/projects/p-1/orchestrator/resolve", input);
  for (const value of [
    { projectId: "other", status: "not-sent", message: "Wrong scope" },
    { projectId: "p-1", status: "absent", message: "Unfenced absence is not safe" },
    { projectId: "p-1", status: "accepted", message: "Missing receipt" },
    { projectId: "p-1", status: "accepted", job: job({ projectId: "other" }), message: "Wrong job" },
  ]) { request.mockResolvedValue(value); await expect(orchestratorApi.resolveRequest("p-1", input)).rejects.toThrow(); }
});
