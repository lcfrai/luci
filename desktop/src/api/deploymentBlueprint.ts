import { productApi } from './product';
import { definitionApi } from './definition';
import { designPlanApi } from './designPlan';

export interface BlueprintContextReference {
  briefRevision: number; specificationRevision: number; specificationDigest?: string;
  designRevision: number; designDigest?: string;
}
export interface BlueprintSource { commit: string; configurationRevision: number; context?: BlueprintContextReference }
export interface BlueprintResources { requestMilliCpu: number; limitMilliCpu: number; requestMemoryMiB: number; limitMemoryMiB: number; gpuCount?: number }
export interface BlueprintService {
  id: string; kind: 'app' | 'inference'; image: string; port: number; replicas: number; resources: BlueprintResources;
  command?: string[]; args?: string[]; dependsOn?: string[];
  model?: { name: string; claimName: string; mountPath: string; relativePath?: string };
  telemetry?: { resourceId: string; endpoint: string };
}
export interface DeploymentBlueprint {
  version: 1; projectId: string; source: BlueprintSource;
  infrastructure: { id: string; kind: 'kubernetes' | 'existing-vm' | 'local-docker'; reference: string; namespace?: string };
  services: BlueprintService[];
  externalResources?: { id: string; kind: 'analytics' | 'database' | 'storage' | 'secret' | 'ingress'; reference: string }[];
}
export interface DeploymentBlueprintPlan {
  version: 1; digest: string; state: 'decisions-required' | 'compiled-prototype'; blueprint: DeploymentBlueprint;
  steps: { componentId: string; action: string; dependsOn: string[] }[];
  decisions: { path: string; code: string; message: string }[];
  artifacts: { name: string; mediaType: 'application/json'; sha256: string; content: string }[];
  capabilities: { id: string; state: string; description: string }[]; warnings: string[];
  resourceSummary: BlueprintResources & { replicas: number; gpuCount: number };
  sourceVerified: false; effectsApplied: false;
}
export interface BlueprintProjectContext {
  projectId: string; source: BlueprintSource; head: string; clean: boolean;
  reviewDigest: string; definitionStatus: string; designStatus: string;
}

const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, max: number): value is string => typeof value === 'string' && value.length <= max;
const count = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const hash = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const jsonKey = (value: unknown): string => JSON.stringify(value, (_key, item: unknown) => object(item) ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a.localeCompare(b))) : item);

// Match the compiler's documented sorting/omitempty rules, not a second compiler.
function normalizedBlueprint(input: DeploymentBlueprint): DeploymentBlueprint {
  const b = JSON.parse(JSON.stringify(input)) as DeploymentBlueprint;
  if (!b.infrastructure.namespace) delete b.infrastructure.namespace;
  if (!b.externalResources?.length) delete b.externalResources;
  else b.externalResources.sort((a, c) => a.id < c.id ? -1 : a.id > c.id ? 1 : 0);
  if (b.source.context) {
    if (!b.source.context.specificationDigest) delete b.source.context.specificationDigest;
    if (!b.source.context.designDigest) delete b.source.context.designDigest;
  }
  b.services.sort((a, c) => a.id < c.id ? -1 : a.id > c.id ? 1 : 0);
  for (const service of b.services) {
    service.dependsOn = [...new Set([b.infrastructure.id, ...(service.dependsOn ?? []), ...(service.telemetry ? [service.telemetry.resourceId] : [])])].sort();
    if (!service.command?.length) delete service.command;
    if (!service.args?.length) delete service.args;
    if (!service.resources.gpuCount) delete service.resources.gpuCount;
    if (service.model && !service.model.relativePath) delete service.model.relativePath;
  }
  return b;
}

export function validDeploymentBlueprintPlan(value: unknown, request: DeploymentBlueprint): value is DeploymentBlueprintPlan {
  try {
    if (!object(value) || value.version !== 1 || !hash(value.digest) || value.sourceVerified !== false || value.effectsApplied !== false ||
        !['decisions-required', 'compiled-prototype'].includes(String(value.state)) || !object(value.blueprint) ||
        jsonKey(value.blueprint) !== jsonKey(normalizedBlueprint(request)) ||
        !Array.isArray(value.steps) || value.steps.length > 97 || !Array.isArray(value.decisions) || value.decisions.length > 512 ||
        !Array.isArray(value.artifacts) || value.artifacts.length > 64 || !Array.isArray(value.capabilities) || value.capabilities.length > 32 ||
        !Array.isArray(value.warnings) || value.warnings.length > 32 || value.warnings.some(item => !text(item, 4096)) || !object(value.resourceSummary)) return false;
    if (value.decisions.some(row => !object(row) || !text(row.path, 256) || !text(row.code, 128) || !text(row.message, 4096)) ||
        value.capabilities.some(row => !object(row) || !text(row.id, 128) || !text(row.state, 128) || !text(row.description, 4096)) ||
        value.steps.some(row => !object(row) || !text(row.componentId, 128) || !text(row.action, 128) || !Array.isArray(row.dependsOn) || row.dependsOn.length > 97 || row.dependsOn.some(id => !text(id, 128)))) return false;
    for (const key of ['replicas', 'requestMilliCpu', 'limitMilliCpu', 'requestMemoryMiB', 'limitMemoryMiB', 'gpuCount']) if (!count(value.resourceSummary[key])) return false;
    if (value.state === 'compiled-prototype' ? value.decisions.length !== 0 || value.artifacts.length !== request.services.length * 2 : value.decisions.length === 0 || value.artifacts.length !== 0) return false;
    const names = new Set<string>();
    return value.artifacts.every(artifact => {
      if (!object(artifact) || !text(artifact.name, 100) || !/^[a-z0-9-]+\.(deployment|service)\.json$/.test(artifact.name) || names.has(artifact.name) ||
          artifact.mediaType !== 'application/json' || !hash(artifact.sha256) || !text(artifact.content, 256 << 10)) return false;
      names.add(artifact.name);
      const resource = JSON.parse(artifact.content) as Record<string, unknown>;
      return object(resource.metadata) && resource.metadata.namespace === request.infrastructure.namespace && object(resource.metadata.annotations) &&
        resource.metadata.annotations['luci.ai/plan-digest'] === value.digest &&
        (artifact.name.endsWith('.deployment.json') ? resource.kind === 'Deployment' && resource.apiVersion === 'apps/v1' : resource.kind === 'Service' && resource.apiVersion === 'v1');
    });
  } catch { return false; }
}

export const deploymentBlueprintApi = {
  async context(projectId: string): Promise<BlueprintProjectContext> {
    const [git, delivery, config, definition, design] = await Promise.all([
      productApi.git(projectId), productApi.gitDelivery(projectId), productApi.configuration(projectId), definitionApi.get(projectId), designPlanApi.get(projectId),
    ]);
    if (definition.projectId !== projectId || design.projectId !== projectId || typeof git.clean !== 'boolean' ||
        !text(delivery.head, 40) || delivery.head && !/^[a-f0-9]{40}$/.test(delivery.head) || !count(config.revision) ||
        !count(definition.briefRevision) || !count(definition.revision) || !count(design.revision) ||
        !text(git.reviewDigest ?? '', 64) || !text(definition.digest, 64) || !text(design.digest, 64)) throw new Error('Current project context could not be confirmed. Refresh before compiling.');
    return { projectId, head: delivery.head, clean: git.clean, reviewDigest: git.reviewDigest ?? '', definitionStatus: definition.status, designStatus: design.status,
      source: { commit: git.clean ? delivery.head : '', configurationRevision: config.revision,
        context: { briefRevision: definition.briefRevision, specificationRevision: definition.revision, specificationDigest: definition.digest, designRevision: design.revision, designDigest: design.digest } } };
  },
  async compile(request: DeploymentBlueprint, signal?: AbortSignal): Promise<DeploymentBlueprintPlan> {
    const value: unknown = await productApi.request(`/v1/product/projects/${encodeURIComponent(request.projectId)}/deployment-blueprint`, request, 'POST', 30_000, signal);
    if (!validDeploymentBlueprintPlan(value, request)) throw new Error('The plan response did not match this project and its exact inputs. Compile again after refreshing context.');
    for (const artifact of value.artifacts) {
      if (!globalThis.crypto?.subtle) throw new Error('This browser cannot verify manifest digests. The plan has not been accepted.');
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(artifact.content));
      const actual = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
      if (actual !== artifact.sha256) throw new Error('A generated manifest failed its digest check. Do not use the output.');
    }
    return value;
  },
};
