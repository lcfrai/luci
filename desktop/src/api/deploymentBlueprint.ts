import { productApi } from './product';
import { definitionApi } from './definition';
import { designPlanApi } from './designPlan';

export interface BlueprintContextReference {
  briefRevision: number; specificationRevision: number; specificationDigest?: string;
  designRevision: number; designDigest?: string;
}
export interface BlueprintSource { commit: string; configurationRevision: number; context?: BlueprintContextReference }
export interface BlueprintResources { requestMilliCpu: number; limitMilliCpu: number; requestMemoryMiB: number; limitMemoryMiB: number; gpuCount?: number }
export interface BlueprintEnvironmentReference { name: string; kind: 'secret'|'config-map'; resourceName: string; key: string }
export interface BlueprintProbe { kind: 'http'|'tcp'; path?: string; initialDelaySeconds: number; periodSeconds: number; timeoutSeconds: number; failureThreshold: number }
export interface BlueprintHealth { startup?: BlueprintProbe; readiness?: BlueprintProbe }
export interface TelemetryTLSReference { resourceName: string; caKey: string; certKey: string; keyKey: string }
export interface NodeInstrumentation { runtime: 'nodejs'; dependencies: Record<string,string> }
export interface CollectorConfiguration { profile:'otelcol-contrib-0.163.0'; destination:{resourceId:string;endpoint:string;authorization?:{resourceName:string;key:string}}; receiverTLS:TelemetryTLSReference; events:{id:string;description:string}[] }
export interface BlueprintTelemetry { resourceId:string;endpoint:string;clientTLS?:TelemetryTLSReference;instrumentation?:NodeInstrumentation }
export const nodeTelemetryPackages = ['@opentelemetry/api','@opentelemetry/sdk-node','@opentelemetry/sdk-metrics','@opentelemetry/resources','@opentelemetry/exporter-trace-otlp-http','@opentelemetry/exporter-metrics-otlp-http'] as const;
export interface BlueprintService {
  id: string; kind: 'app' | 'inference' | 'collector'; image: string; port: number; replicas: number; resources: BlueprintResources;
  command?: string[]; args?: string[]; dependsOn?: string[];
  model?: { name: string; claimName: string; mountPath: string; relativePath?: string };
  telemetry?: BlueprintTelemetry;
  collector?: CollectorConfiguration;
  environment?: BlueprintEnvironmentReference[];
  health?: BlueprintHealth;
  deploymentStrategy?: 'recreate';
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
  supportingArtifacts?: {name:string;mediaType:'application/json'|'text/javascript'|'text/markdown';sha256:string;content:string}[];
  capabilities: { id: string; state: string; description: string }[]; warnings: string[];
  resourceSummary: BlueprintResources & { replicas: number; gpuCount: number };
  sourceVerified: false; effectsApplied: false;
}
export interface BlueprintProjectContext {
  projectId: string; source: BlueprintSource; head: string; clean: boolean;
  reviewDigest: string; definitionStatus: string; designStatus: string;
}
export interface SavedDeploymentBlueprint {
  projectId: string; revision: number; digest: string; blueprint: DeploymentBlueprint; planDigest: string;
  actor: { id: string; displayName: string }; at: string;
}
export interface SavedBlueprintView { projectId: string; saved: SavedDeploymentBlueprint | null; sourceVerified: false; effectsApplied: false }
export interface SaveBlueprintRequest { expectedRevision: number; expectedDigest: string; idempotencyKey: string; blueprint: DeploymentBlueprint }

const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, max: number): value is string => typeof value === 'string' && value.length <= max;
const count = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const hash = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const jsonKey = (value: unknown): string => JSON.stringify(value, (_key, item: unknown) => object(item) ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a.localeCompare(b))) : item);

const only = (v: Record<string,unknown>, keys: string[]) => Object.keys(v).every(key=>keys.includes(key));
export function validTelemetryTLS(value:unknown):value is TelemetryTLSReference { return object(value)&&only(value,['resourceName','caKey','certKey','keyKey'])&&text(value.resourceName,253)&&[value.caKey,value.certKey,value.keyKey].every(key=>text(key,253)); }
export function validBlueprintTelemetry(v:unknown):v is BlueprintTelemetry {
  if(!object(v)||!only(v,['resourceId','endpoint','clientTLS','instrumentation'])||!text(v.resourceId,63)||!text(v.endpoint,2048)||v.clientTLS!==undefined&&!validTelemetryTLS(v.clientTLS))return false;
  if(v.instrumentation===undefined)return true;const i=v.instrumentation;
  return object(i)&&only(i,['runtime','dependencies'])&&i.runtime==='nodejs'&&object(i.dependencies)&&Object.keys(i.dependencies).length===nodeTelemetryPackages.length&&nodeTelemetryPackages.every(key=>typeof (i.dependencies as Record<string,unknown>)[key]==='string'&&/^[0-9]+\.[0-9]+\.[0-9]+$/.test(String((i.dependencies as Record<string,unknown>)[key])));
}
export function validCollector(v:unknown):v is CollectorConfiguration {
  return object(v)&&only(v,['profile','destination','receiverTLS','events'])&&v.profile==='otelcol-contrib-0.163.0'&&object(v.destination)&&only(v.destination,['resourceId','endpoint','authorization'])&&text(v.destination.resourceId,63)&&text(v.destination.endpoint,2048)&&
    (v.destination.authorization===undefined||object(v.destination.authorization)&&only(v.destination.authorization,['resourceName','key'])&&text(v.destination.authorization.resourceName,253)&&text(v.destination.authorization.key,253))&&validTelemetryTLS(v.receiverTLS)&&Array.isArray(v.events)&&v.events.length>=1&&v.events.length<=16&&new Set(v.events.map(row=>object(row)?row.id:undefined)).size===v.events.length&&v.events.every(row=>object(row)&&only(row,['id','description'])&&text(row.id,63)&&/^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(row.id)&&text(row.description,256));
}

export function validStoredBlueprint(value: unknown, projectId: string): value is DeploymentBlueprint {
  if (!object(value) || value.version !== 1 || value.projectId !== projectId || !object(value.source) ||
      !text(value.source.commit, 40) || value.source.commit !== '' && !/^[a-f0-9]{40}$/.test(value.source.commit) || !count(value.source.configurationRevision) ||
      !object(value.infrastructure) || !text(value.infrastructure.id, 63) || typeof value.infrastructure.kind !== 'string' || !['kubernetes', 'existing-vm', 'local-docker'].includes(value.infrastructure.kind) ||
      !text(value.infrastructure.reference, 512) || value.infrastructure.namespace !== undefined && !text(value.infrastructure.namespace, 63) ||
      !Array.isArray(value.services) || value.services.length < 1 || value.services.length > 32) return false;
  const context = value.source.context;
  if (context !== undefined && (!object(context) || !count(context.briefRevision) || !count(context.specificationRevision) || !count(context.designRevision) ||
      context.specificationDigest !== undefined && !hash(context.specificationDigest) || context.designDigest !== undefined && !hash(context.designDigest))) return false;
  const ids = new Set<string>();
  if (!value.services.every(row => {
    if (!object(row) || !text(row.id, 63) || ids.has(row.id) || typeof row.kind !== 'string' || !['app', 'inference','collector'].includes(row.kind) || !text(row.image, 512) || !count(row.port) || !count(row.replicas) || !object(row.resources)) return false;
    ids.add(row.id);
    for (const key of ['requestMilliCpu', 'limitMilliCpu', 'requestMemoryMiB', 'limitMemoryMiB']) if (!count(row.resources[key])) return false;
    if (row.resources.gpuCount !== undefined && !count(row.resources.gpuCount)) return false;
    for (const key of ['command', 'args', 'dependsOn']) if (row[key] !== undefined && (!Array.isArray(row[key]) || row[key].length > 97 || row[key].some(v => !text(v, 4096)))) return false;
    if (row.model !== undefined && (!object(row.model) || !text(row.model.name, 128) || !text(row.model.claimName, 63) || !text(row.model.mountPath, 256) || row.model.relativePath !== undefined && !text(row.model.relativePath, 256))) return false;
    if (row.telemetry !== undefined && !validBlueprintTelemetry(row.telemetry)) return false;
    if (row.kind==='collector' ? !validCollector(row.collector) : row.collector!==undefined) return false;
    if (row.deploymentStrategy !== undefined && row.deploymentStrategy !== 'recreate') return false;
    if (row.environment !== undefined && (!Array.isArray(row.environment) || row.environment.length > 16 || row.environment.some(ref => !object(ref) || Object.keys(ref).some(key => !['name','kind','resourceName','key'].includes(key)) || !text(ref.name,128) || (ref.kind !== 'secret' && ref.kind !== 'config-map') || !text(ref.resourceName,253) || !text(ref.key,253)))) return false;
    if (row.health !== undefined && (!object(row.health) || Object.keys(row.health).some(key => key !== 'startup' && key !== 'readiness') || Object.values(row.health).some(probe => !object(probe) || Object.keys(probe).some(key => !['kind','path','initialDelaySeconds','periodSeconds','timeoutSeconds','failureThreshold'].includes(key)) || (probe.kind !== 'http' && probe.kind !== 'tcp') || probe.path !== undefined && !text(probe.path,512) || !count(probe.initialDelaySeconds) || probe.initialDelaySeconds>600 || !count(probe.periodSeconds) || probe.periodSeconds>300 || !count(probe.timeoutSeconds) || probe.timeoutSeconds>30 || !count(probe.failureThreshold) || probe.failureThreshold>120))) return false;
    return true;
  })) return false;
  return value.externalResources === undefined || Array.isArray(value.externalResources) && value.externalResources.length <= 64 && value.externalResources.every(row =>
    object(row) && text(row.id, 63) && typeof row.kind === 'string' && ['analytics', 'database', 'storage', 'secret', 'ingress'].includes(row.kind) && text(row.reference, 512));
}

export function validSavedBlueprintView(value: unknown, projectId: string): value is SavedBlueprintView {
  if (!object(value) || value.projectId !== projectId || value.sourceVerified !== false || value.effectsApplied !== false) return false;
  if (value.saved === null) return true;
  const row = value.saved;
  return object(row) && row.projectId === projectId && count(row.revision) && row.revision >= 1 && row.revision <= 1_000_000_000 && hash(row.digest) && hash(row.planDigest) &&
    object(row.actor) && text(row.actor.id, 128) && row.actor.id !== '' && text(row.actor.displayName, 512) && text(row.at, 64) && Number.isFinite(Date.parse(row.at)) && validStoredBlueprint(row.blueprint, projectId);
}

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
    if(service.collector) service.collector.events.sort((a,c)=>a.id<c.id?-1:a.id>c.id?1:0);
    if (!service.environment?.length) delete service.environment;
    else service.environment.sort((a,c) => a.name<c.name ? -1 : a.name>c.name ? 1 : 0);
    if (service.health) for (const probe of [service.health.startup,service.health.readiness]) if (probe && !probe.path) delete probe.path;
    service.dependsOn = [...new Set([b.infrastructure.id, ...(service.dependsOn ?? []), ...(service.telemetry ? [service.telemetry.resourceId] : []),...(service.collector?[service.collector.destination.resourceId]:[])])].sort();
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
    if(!validSupportingArtifacts(value,request))return false;
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

function validSupportingArtifacts(value:Record<string,unknown>,request:DeploymentBlueprint):boolean {
  const expected=new Map<string,string>();
  for(const service of request.services){if(service.collector)expected.set(`${service.id}.collector-config.json`,'application/json');if(service.telemetry?.instrumentation){expected.set(`${service.id}.bootstrap.mjs`,'text/javascript');expected.set(`${service.id}.events.mjs`,'text/javascript');expected.set(`${service.id}.dependencies.json`,'application/json');expected.set(`${service.id}.instrumentation.md`,'text/markdown');}}
  const rows=value.supportingArtifacts;
  if(value.state==='decisions-required')return rows===undefined||Array.isArray(rows)&&rows.length===0;
  if(expected.size===0)return rows===undefined||Array.isArray(rows)&&rows.length===0;
  if(!Array.isArray(rows)||rows.length!==expected.size||rows.length>33)return false;
  const seen=new Set<string>();let bytes=0;
  for(const row of rows){if(!object(row)||!text(row.name,100)||seen.has(row.name)||expected.get(row.name)!==row.mediaType||!hash(row.sha256)||!text(row.content,128<<10))return false;seen.add(row.name);bytes+=row.content.length;if(bytes>1<<20)return false;}
  return true;
}

export const deploymentBlueprintApi = {
  async saved(projectId: string, signal?: AbortSignal): Promise<SavedBlueprintView> {
    const value: unknown = await productApi.request(`/v1/product/projects/${encodeURIComponent(projectId)}/deployment-blueprint/saved`, undefined, 'GET', 30_000, signal);
    if (!validSavedBlueprintView(value, projectId)) throw new Error('The saved blueprint response did not match this project. Your local draft has been kept.');
    return value;
  },
  async save(projectId: string, request: SaveBlueprintRequest, signal?: AbortSignal): Promise<SavedBlueprintView> {
    const value: unknown = await productApi.request(`/v1/product/projects/${encodeURIComponent(projectId)}/deployment-blueprint/saved`, request, 'PUT', 30_000, signal);
    if (!validSavedBlueprintView(value, projectId) || !value.saved || jsonKey(value.saved.blueprint) !== jsonKey(normalizedBlueprint(request.blueprint)) || value.saved.revision !== request.expectedRevision + 1) throw new Error('The saved response did not confirm this exact blueprint. Keep your draft and review the saved revision.');
    return value;
  },
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
    for (const artifact of [...value.artifacts,...(value.supportingArtifacts??[])]) {
      if (!globalThis.crypto?.subtle) throw new Error('This browser cannot verify generated artifact digests. The plan has not been accepted.');
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(artifact.content));
      const actual = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
      if (actual !== artifact.sha256) throw new Error('A generated artifact failed its digest check. Do not use the output.');
    }
    return value;
  },
};
