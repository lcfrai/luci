import { afterEach, expect, it, vi } from "vitest";
import { coordinatorPolicyApi, effectiveAutomaticRepairLimit, type CoordinatorPolicy } from "./coordinatorPolicy";
import { productApi } from "./product";
import { orchestratorApi } from "./orchestrator";
import { view } from "../components/orchestrator/fixtures";
const policy = (): CoordinatorPolicy => ({ projectId: "p-1", revision: 1, mode: "review", reviewMode: "manual", allowedTargets: ["specification"], executionMode: "manual", deploymentMode: "manual", actor: { id: "alice", displayName: "Alice" }, updatedAt: "2026-10-12T00:00:00Z" });
afterEach(() => vi.restoreAllMocks());
it.each([0, 1, 2])('roundtrips an explicit repair budget %i without normalizing legacy policy', async automaticRepairLimit => {
  const legacy = policy(), before = JSON.stringify(legacy);
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(legacy);
  expect(effectiveAutomaticRepairLimit((await coordinatorPolicyApi.get('p-1')).automaticRepairLimit)).toBe(2);
  expect(JSON.stringify(legacy)).toBe(before);
  const saved = {...policy(), revision: 2, automaticRepairLimit}; request.mockResolvedValue(saved);
  const input = {expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const], automaticRepairLimit};
  expect((await coordinatorPolicyApi.save('p-1', input)).automaticRepairLimit).toBe(automaticRepairLimit);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/policy', input);
  expect((await coordinatorPolicyApi.get('p-1')).automaticRepairLimit).toBe(automaticRepairLimit);
});
it.each([undefined, null])('preserves an existing zero when an unrelated save uses %s', async automaticRepairLimit => {
  const saved = {...policy(), revision: 2, automaticRepairLimit: 0};
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(saved);
  const input = {expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const], ...(automaticRepairLimit === null ? {automaticRepairLimit: null} : {})};
  expect((await coordinatorPolicyApi.save('p-1', input)).automaticRepairLimit).toBe(0);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/policy', input);
});
it.each([-1, 3, 0.5, '0', null])('rejects malformed repair budget response %s', async automaticRepairLimit => {
  vi.spyOn(productApi, 'request').mockResolvedValue({...policy(), automaticRepairLimit});
  await expect(coordinatorPolicyApi.get('p-1')).rejects.toThrow(/could not be verified/);
});
it('retains uncertainty when explicit zero save returns another limit or an omitted field', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({...policy(), revision: 2, automaticRepairLimit: 2});
  const input = {expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const], automaticRepairLimit: 0};
  await expect(coordinatorPolicyApi.save('p-1', input)).rejects.toThrow(/did not match/);
  request.mockResolvedValue({...policy(), revision: 2});
  await expect(coordinatorPolicyApi.save('p-1', input)).rejects.toThrow(/did not match/);
});
it.each([-1, 3, 0.5])('refuses invalid requested repair budget%i before sending a mutation', async automaticRepairLimit => {
  const request = vi.spyOn(productApi, 'request');
  await expect(coordinatorPolicyApi.save('p-1', {expectedRevision: 1, mode: 'review', allowedTargets: ['specification'], automaticRepairLimit})).rejects.toThrow(/zero to two/);
  expect(request).not.toHaveBeenCalled();
});
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
it("binds explicit routine review policy readback without granting build or deployment", async () => {
  const response = { ...policy(), revision: 2, mode: "auto-drafts" as const, reviewMode: "approved-amendments" as const };
  vi.spyOn(productApi, "request").mockResolvedValue(response);
  const input = { expectedRevision: 1, mode: "auto-drafts" as const, reviewMode: "approved-amendments" as const, allowedTargets: ["specification" as const] };
  expect(await coordinatorPolicyApi.save("p-1", input)).toEqual(response);
  vi.spyOn(productApi, "request").mockResolvedValue({ ...response, reviewMode: "manual" });
  await expect(coordinatorPolicyApi.save("p-1", input)).rejects.toThrow("did not match");
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

it('requires exact readback for an explicitly enabled reviewed-scope integration policy', async () => {
  const saved = { ...policy(), revision: 2, integrationMode: 'reviewed-scopes' as const };
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(saved);
  const input = { expectedRevision: 1, mode: 'review' as const, allowedTargets: ['specification' as const], integrationMode: 'reviewed-scopes' as const };
  expect(await coordinatorPolicyApi.save('p-1', input)).toEqual(saved);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/policy', input);
  request.mockResolvedValue({ ...saved, integrationMode: 'manual' });
  await expect(coordinatorPolicyApi.save('p-1', input)).rejects.toThrow(/did not match/);
  request.mockResolvedValue({ ...saved, integrationMode: 'all-candidates' });
  await expect(coordinatorPolicyApi.get('p-1')).rejects.toThrow(/could not be verified/);
});
