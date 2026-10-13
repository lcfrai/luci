import { afterEach, expect, it, vi } from "vitest";
import { coordinatorPolicyApi, type CoordinatorPolicy } from "./coordinatorPolicy";
import { productApi } from "./product";
import { orchestratorApi } from "./orchestrator";
import { view } from "../components/orchestrator/fixtures";
const policy = (): CoordinatorPolicy => ({ projectId: "p-1", revision: 1, mode: "review", allowedTargets: ["specification"], executionMode: "manual", deploymentMode: "manual", actor: { id: "alice", displayName: "Alice" }, updatedAt: "2026-10-12T00:00:00Z" });
afterEach(() => vi.restoreAllMocks());
it("retains exact CAS and omits unrelated execution/deployment policy fields when saving draft policy", async () => {
  const response = { ...policy(), projectId: "p/one", revision: 5, mode: "auto-drafts" as const, executionMode: "reviewed-tasks" as const };
  const request = vi.spyOn(productApi, "request").mockResolvedValue(response);
  const input = { expectedRevision: 4, mode: "auto-drafts" as const, allowedTargets: ["specification" as const] };
  expect(await coordinatorPolicyApi.save("p/one", input)).toEqual(response);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p%2Fone/orchestrator/policy", input);
});
it.each([
  { projectId: "other" }, { revision: -1 }, { revision: 1.5 }, { mode: "automatic-all" }, { allowedTargets: ["deploy"] },
  { allowedTargets: ["plan", "plan"] }, { mode: "auto-drafts", allowedTargets: [] }, { actor: undefined },
  { updatedAt: "not-a-date" }, { executionMode: "unlimited" }, { deploymentMode: "automatic" },
])("rejects a malformed or foreign policy %j", async patch => {
  vi.spyOn(productApi, "request").mockResolvedValue({ ...policy(), ...patch });
  await expect(coordinatorPolicyApi.get("p-1")).rejects.toThrow("could not be verified");
});
it("preserves abort signal for display-only reads", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(policy());
  const signal = new AbortController().signal;
  await coordinatorPolicyApi.get("p-1", signal);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator/policy", undefined, "GET", 120000, signal);
});
it.each([{ revision: 3 }, { mode: "auto-drafts" }, { allowedTargets: ["plan"] }])("rejects a saved reply that does not prove the requested change %j", async patch => {
  vi.spyOn(productApi, "request").mockResolvedValue({ ...policy(), revision: 2, ...patch });
  await expect(coordinatorPolicyApi.save("p-1", { expectedRevision: 1, mode: "review", allowedTargets: ["specification"] })).rejects.toThrow("did not match");
});
it("binds automatic draft advance to exact turn, context and policy without accepting another project's view", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view());
  const input = { expectedRevision: 3, expectedContextDigest: "context", expectedPolicyRevision: 7 };
  await orchestratorApi.advance("p-1", "turn/one", input);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator/turns/turn%2Fone/advance", input);
  request.mockResolvedValue(view({ projectId: "other" }));
  await expect(orchestratorApi.advance("p-1", "turn/one", input)).rejects.toThrow("could not be verified");
});

it('preserves omitted preview policy and binds explicit checked-source changes without upgrading deployment', async () => {
  const saved = { ...policy(), revision: 2, previewMode: 'checked-source' as const };
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(saved);
  const input = { expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const], previewMode: 'checked-source' as const };
  expect(await coordinatorPolicyApi.save('p-1', input)).toEqual(saved); expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/policy', input);
  request.mockResolvedValue({ ...saved, previewMode: 'manual' }); await expect(coordinatorPolicyApi.save('p-1', input)).rejects.toThrow(/did not match/);
  request.mockResolvedValue({ ...saved, previewMode: 'public-auto' }); await expect(coordinatorPolicyApi.get('p-1')).rejects.toThrow(/could not be verified/);
});

it('preserves existing-target permission on draft-only writes and requires exact explicit deployment policy readback', async () => {
  const response = { ...policy(), revision: 2, deploymentMode: 'existing-target' as const, previewMode: 'checked-source' as const };
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(response);
  const draftOnly = { expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const] };
  expect(await coordinatorPolicyApi.save('p-1', draftOnly)).toEqual(response);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/policy', draftOnly);
  await expect(coordinatorPolicyApi.save('p-1', { ...draftOnly, deploymentMode: 'manual' })).rejects.toThrow(/did not match/);
  expect(await coordinatorPolicyApi.save('p-1', { ...draftOnly, deploymentMode: 'existing-target' })).toEqual(response);
});
