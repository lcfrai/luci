import { createHash, webcrypto } from 'node:crypto';
import { afterEach, expect, it, vi } from 'vitest';
import { deploymentBlueprintApi, validDeploymentBlueprintPlan, type DeploymentBlueprint, type DeploymentBlueprintPlan } from './deploymentBlueprint';
import { productApi } from './product';

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
      return { name: `app-deadbeef.${kind}.json`, mediaType: 'application/json' as const, content, sha256: createHash('sha256').update(content).digest('hex') };
    }),
  };
}
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

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
  const input = request(), signal = new AbortController().signal;
  expect(await deploymentBlueprintApi.compile(input, signal)).toEqual(plan);
  expect(send).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p-one/deployment-blueprint', input, 'POST', 30_000, signal);
  plan.artifacts[0].content += ' ';
  await expect(deploymentBlueprintApi.compile(input)).rejects.toThrow('digest check');
});
