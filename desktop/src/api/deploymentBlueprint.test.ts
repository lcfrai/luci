import { afterEach, expect, it, vi } from 'vitest';
import { deploymentBlueprintApi, validDeploymentBlueprintPlan, validSavedBlueprintView, type DeploymentBlueprint, type DeploymentBlueprintPlan } from './deploymentBlueprint';
import { productApi } from './product';

const { webcrypto } = await vi.importActual<{ webcrypto: Crypto }>('node:crypto');

const request = (): DeploymentBlueprint => ({ version: 1, projectId: 'p-one', source: { commit: 'a'.repeat(40), configurationRevision: 2, context: { briefRevision: 1, specificationRevision: 3, designRevision: 4 } },
  infrastructure: { id: 'cluster', kind: 'kubernetes', reference: 'existing-east', namespace: 'app-one' },
  services: [{ id: 'app', kind: 'app', image: `registry.example/app@sha256:${'b'.repeat(64)}`, port: 8080, replicas: 1, resources: { requestMilliCpu: 100, limitMilliCpu: 500, requestMemoryMiB: 128, limitMemoryMiB: 512 }, dependsOn: ['cluster'] }],
});
function result(input = request()): DeploymentBlueprintPlan {
  const digest = 'd'.repeat(64);
  return { version: 1, digest, state: 'compiled-prototype', blueprint: input, sourceVerified: false, effectsApplied: false,
    steps: [{ componentId: 'cluster', action: 'review-existing-infrastructure', dependsOn: [] }, { componentId: 'app', action: 'compile-deployment-and-internal-service', dependsOn: ['cluster'] }], decisions: [], capabilities: [], warnings: [],
    resourceSummary: { replicas: 1, requestMilliCpu: 100, limitMilliCpu: 500, requestMemoryMiB: 128, limitMemoryMiB: 512, gpuCount: 0 },
    artifacts: ['deployment', 'service'].map(kind => {
      const content = JSON.stringify({ apiVersion: kind === 'deployment' ? 'apps/v1' : 'v1', kind: kind === 'deployment' ? 'Deployment' : 'Service', metadata: { namespace: 'app-one', annotations: { 'luci.ai/plan-digest': digest } } })+'\n';
      return { name: `app-deadbeef.${kind}.json`, mediaType: 'application/json' as const, content, sha256: 'c'.repeat(64) };
    }),
  };
}
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it('validates saved project topology and refuses an unconfirmed save response', async () => {
  const input = request(), body = { projectId: 'p-one', saved: { projectId: 'p-one', revision: 1, digest: 'e'.repeat(64), planDigest: 'd'.repeat(64), blueprint: input, actor: { id: 'one', displayName: 'Owner' }, at: '2026-10-31T00:00:00Z' }, sourceVerified: false, effectsApplied: false };
  expect(validSavedBlueprintView(body, 'p-one')).toBe(true);
  expect(validSavedBlueprintView(body, 'p-other')).toBe(false);
  const malformed = JSON.parse(JSON.stringify(body)); malformed.saved.blueprint.services[0].kind = ['app'];
  expect(validSavedBlueprintView(malformed, 'p-one')).toBe(false);
  const send = vi.spyOn(productApi, 'request').mockResolvedValue(body);
  const save = { expectedRevision: 0, expectedDigest: '', idempotencyKey: 'one-save-request', blueprint: input };
  expect(await deploymentBlueprintApi.save('p-one', save)).toEqual(body);
  expect(send).toHaveBeenCalledWith('/v1/product/projects/p-one/deployment-blueprint/saved', save, 'PUT', 30_000, undefined);
  body.saved.blueprint.infrastructure.reference = 'different-cluster';
  await expect(deploymentBlueprintApi.save('p-one', { ...save, blueprint: request() })).rejects.toThrow('exact blueprint');
});

it('binds the plan response to the exact project, context and explicit service inputs', () => {
  expect(validDeploymentBlueprintPlan(result(), request())).toBe(true);
  const foreign = result(); foreign.blueprint.projectId = 'p-other';
  expect(validDeploymentBlueprintPlan(foreign, request())).toBe(false);
  const stale = result(); stale.blueprint.source.context!.designRevision++;
  expect(validDeploymentBlueprintPlan(stale, request())).toBe(false);
  const unsupported = result(); unsupported.effectsApplied = true as false;
  expect(validDeploymentBlueprintPlan(unsupported, request())).toBe(false);
});

it('requires unresolved plans to contain decisions and no partial manifests', () => {
  const value = result(); value.state = 'decisions-required'; value.decisions = [{ path: 'services[0].image', code: 'image-required', message: 'Choose an image.' }]; value.artifacts = [];
  expect(validDeploymentBlueprintPlan(value, request())).toBe(true);
  value.artifacts = result().artifacts;
  expect(validDeploymentBlueprintPlan(value, request())).toBe(false);
  const duplicate = result(); duplicate.artifacts[1] = duplicate.artifacts[0];
  expect(validDeploymentBlueprintPlan(duplicate, request())).toBe(false);
});

it('checks exact manifest bytes before accepting output and forwards only the planning request', async () => {
  vi.stubGlobal('crypto', webcrypto);
  const plan = result(), send = vi.spyOn(productApi, 'request').mockResolvedValue(plan);
  for (const artifact of plan.artifacts) {
    const hash = await webcrypto.subtle.digest('SHA-256', new TextEncoder().encode(artifact.content));
    artifact.sha256 = Array.from(new Uint8Array(hash), byte => byte.toString(16).padStart(2, '0')).join('');
  }
  const input = request(), signal = new AbortController().signal;
  expect(await deploymentBlueprintApi.compile(input, signal)).toEqual(plan);
  expect(send).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p-one/deployment-blueprint', input, 'POST', 30_000, signal);
  plan.artifacts[0].content += ' ';
  await expect(deploymentBlueprintApi.compile(input)).rejects.toThrow('digest check');
});
