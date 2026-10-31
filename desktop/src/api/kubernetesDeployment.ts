import { productApi } from './product';
import { validStoredBlueprint, type DeploymentBlueprint } from './deploymentBlueprint';

export interface KubernetesTarget { id: string; projectId: string; name: string; server: string; namespace: string; namespaceUid: string; revision: number; fingerprint: string }
export interface KubernetesTargetInput { id: string; expectedRevision: number; name: string; server: string; namespace: string; credentials: { ca: string; token?: string; certificate?: string; key?: string } }
export interface KubernetesIdentity { projectId: string; actorId: string; teamId?: string; targetId: string; targetFingerprint: string; operationId: string; planDigest: string }
export interface KubernetesResource { kind: 'Deployment' | 'Service'; name: string; uid: string; resourceVersion: string }
export interface KubernetesReview { id: string; request: { identity: KubernetesIdentity; blueprint: DeploymentBlueprint; approvalReference: string; expected: KubernetesResource[] }; target: KubernetesTarget;
  blueprintRevision: number; blueprintDigest: string; runId: string; checkId: string; runInputDigest: string; checkDigest: string; contextDigest: string; monthlyBudgetUsd: number; createdAt: string; expiresAt: string; applied: boolean }
export interface KubernetesReviewInput { targetId: string; blueprintRevision: number; blueprintDigest: string; runId: string; checkId: string; monthlyBudgetUsd: number }
export interface KubernetesOperation { revision: number; request: KubernetesReview['request']; requestDigest: string; namespaceUid: string; state: 'reserved' | 'unknown' | 'pending' | 'ready' | 'blocked'; observedAt: string;
  resources: (KubernetesResource & { desiredSha256: string; attempted: boolean; observedUid?: string; observedVersion?: string; observedGeneration?: number; ready: boolean })[]; applicationHealthVerified: false }
export interface KubernetesStatus { review: KubernetesReview; operation: KubernetesOperation | null; state: 'reviewed' | KubernetesOperation['state']; applicationHealthVerified: false; imageProvenance: string }
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const text = (v: unknown, max: number): v is string => typeof v === 'string' && v.length <= max;
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const positive = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) >= 1 && (v as number) <= 1_000_000_000;
const date = (v: unknown): v is string => text(v, 64) && Number.isFinite(Date.parse(v));
export function validKubernetesTarget(v: unknown, id: string): v is KubernetesTarget {
  if (!object(v) || v.projectId !== id || !text(v.id,40) || !/^cluster-[a-f0-9]{32}$/.test(v.id) || !text(v.name,120) || !v.name || !text(v.server,2048) || !text(v.namespace,63) || !v.namespace || !text(v.namespaceUid,128) || !v.namespaceUid || !positive(v.revision) || !hash(v.fingerprint) || 'credentials' in v || 'token' in v || 'key' in v) return false;
  try { const url = new URL(v.server); return url.protocol === 'https:' && !url.username && !url.password && !url.search && !url.hash && (url.pathname === '' || url.pathname === '/'); } catch { return false; }
}
function validResource(v: unknown): v is KubernetesResource {
  return object(v) && (v.kind === 'Deployment' || v.kind === 'Service') && text(v.name,63) && !!v.name && text(v.uid,128) && text(v.resourceVersion,128) && (!!v.uid === !!v.resourceVersion);
}
export function validKubernetesReview(v: unknown, id: string): v is KubernetesReview {
  if (!object(v) || !text(v.id,41) || !/^workload-[a-f0-9]{32}$/.test(v.id) || !object(v.request) || !object(v.request.identity) || !validStoredBlueprint(v.request.blueprint,id) || !validKubernetesTarget(v.target,id) || !positive(v.blueprintRevision) || !hash(v.blueprintDigest) || !text(v.runId,28) || !/^run-[A-Za-z0-9_-]{24}$/.test(v.runId) || !text(v.checkId,128) || !v.checkId || !hash(v.runInputDigest) || !hash(v.checkDigest) || !hash(v.contextDigest) || typeof v.monthlyBudgetUsd !== 'number' || !Number.isFinite(v.monthlyBudgetUsd) || v.monthlyBudgetUsd <= 0 || !date(v.createdAt) || !date(v.expiresAt) || Date.parse(v.expiresAt)-Date.parse(v.createdAt)!==600_000 || typeof v.applied !== 'boolean') return false;
  const identity = v.request.identity;
  if (identity.projectId!==id || !text(identity.actorId,128) || !identity.actorId || identity.teamId!==undefined && !text(identity.teamId,128) || identity.targetId!==v.target.id || identity.targetFingerprint!==v.target.fingerprint || identity.operationId!==v.id || !hash(identity.planDigest) || v.request.approvalReference!==v.id || v.request.blueprint.infrastructure.reference!==v.target.id || v.request.blueprint.infrastructure.namespace!==v.target.namespace) return false;
  if (!Array.isArray(v.request.expected) || v.request.expected.length<2 || v.request.expected.length>64 || !v.request.expected.every(validResource)) return false;
  return new Set(v.request.expected.map(row=>`${row.kind}/${row.name}`)).size===v.request.expected.length;
}
export function validKubernetesStatus(v: unknown,id: string): v is KubernetesStatus {
  if (!object(v) || !validKubernetesReview(v.review,id) || v.applicationHealthVerified!==false || v.imageProvenance!=='user-supplied immutable digest; checked-source-to-image provenance not verified') return false;
  const review=v.review;
  if (v.operation===null) return v.state===(v.review.applied?'unknown':'reviewed');
  const op=v.operation;
  if (!object(op) || !positive(op.revision) || !object(op.request) || JSON.stringify(op.request)!==JSON.stringify(v.review.request) || !hash(op.requestDigest) || op.namespaceUid!==v.review.target.namespaceUid || op.applicationHealthVerified!==false || !date(op.observedAt) || typeof op.state!=='string' || !['reserved','unknown','pending','ready','blocked'].includes(op.state) || v.state!==op.state || !Array.isArray(op.resources) || op.resources.length!==v.review.request.expected.length) return false;
  return op.resources.every((row,index)=>validResource(row) && object(row) && JSON.stringify({kind:row.kind,name:row.name,uid:row.uid,resourceVersion:row.resourceVersion})===JSON.stringify(review.request.expected[index]) && hash(row.desiredSha256) && typeof row.attempted==='boolean' && typeof row.ready==='boolean' && (row.observedUid===undefined || text(row.observedUid,128)) && (row.observedVersion===undefined || text(row.observedVersion,128)) && (row.observedGeneration===undefined || Number.isSafeInteger(row.observedGeneration) && (row.observedGeneration as number)>=0));
}
const path = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/kubernetes`;
const bad = () => new Error('The cluster response could not be confirmed for this project. Read retained deployments before retrying an action.');
export const kubernetesDeploymentApi = {
  targets: async (id:string,signal?:AbortSignal):Promise<KubernetesTarget[]> => { const v:unknown=await productApi.request(`${path(id)}/targets`,undefined,'GET',30_000,signal); if (!object(v) || !Array.isArray(v.targets) || v.targets.length>32 || !v.targets.every(row=>validKubernetesTarget(row,id)) || new Set(v.targets.map(row=>row.id)).size!==v.targets.length) throw bad(); return v.targets; },
  saveTarget: async (id:string,input:KubernetesTargetInput,signal?:AbortSignal):Promise<KubernetesTarget> => { const v:unknown=await productApi.request(`${path(id)}/targets`,input,'PUT',30_000,signal); if (!validKubernetesTarget(v,id) || v.namespace!==input.namespace || v.name!==input.name || v.revision!==input.expectedRevision+1 || input.id!=='' && v.id!==input.id) throw bad(); return v; },
  list: async (id:string,signal?:AbortSignal):Promise<KubernetesStatus[]> => { const v:unknown=await productApi.request(`${path(id)}/deployments`,undefined,'GET',30_000,signal); if (!object(v) || !Array.isArray(v.deployments) || v.deployments.length>128 || !v.deployments.every(row=>validKubernetesStatus(row,id)) || new Set(v.deployments.map(row=>row.review.id)).size!==v.deployments.length) throw bad(); return v.deployments; },
  review: async (id:string,input:KubernetesReviewInput,signal?:AbortSignal):Promise<KubernetesReview> => { const v:unknown=await productApi.request(`${path(id)}/review`,input,'POST',40_000,signal); if (!validKubernetesReview(v,id) || v.applied || v.target.id!==input.targetId || v.blueprintRevision!==input.blueprintRevision || v.blueprintDigest!==input.blueprintDigest || v.runId!==input.runId || v.checkId!==input.checkId || v.monthlyBudgetUsd!==input.monthlyBudgetUsd) throw bad(); return v; },
  action: async (id:string,review:KubernetesReview,action:'apply'|'observe',acknowledged:boolean,signal?:AbortSignal):Promise<KubernetesStatus> => { const v:unknown=await productApi.request(`${path(id)}/deployments/${encodeURIComponent(review.id)}/${action}`,{acknowledgeCost:acknowledged,acknowledgeImageProvenance:acknowledged},'POST',150_000,signal); if (!validKubernetesStatus(v,id) || v.review.id!==review.id || JSON.stringify(v.review.request)!==JSON.stringify(review.request)) throw bad(); return v; },
};
