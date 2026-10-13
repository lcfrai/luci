import { productApi, type DeploymentTarget } from './product';

export interface CoordinatorDeploymentInput {
  targetId: string; repository: string; branch: string; domain: string; port: number;
  healthPath: string; dockerfile: string; source: 'public' | 'private'; privateKeyUuid?: string;
  dataClientFramework: '' | 'none' | 'vite' | 'nextjs';
}
export interface CoordinatorDeploymentPlan extends CoordinatorDeploymentInput {
  targetFingerprint: string; serverUuid: string; expectedConfigurationRevision: number;
  expectedDataDomainRevision: number; expectedDataBindingFingerprint: string;
  acknowledgeRepositoryPush: boolean; acknowledgePublic: boolean; acknowledgeDeploymentCosts: boolean;
}
const inputKeys = ['targetId', 'repository', 'branch', 'domain', 'port', 'healthPath', 'dockerfile', 'source', 'privateKeyUuid', 'dataClientFramework'] as const;
const bindingKeys = [...inputKeys, 'targetFingerprint', 'serverUuid', 'expectedConfigurationRevision', 'expectedDataDomainRevision', 'expectedDataBindingFingerprint'] as const;
const keys = [...bindingKeys, 'acknowledgeRepositoryPush', 'acknowledgePublic', 'acknowledgeDeploymentCosts'];
const id = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const digest = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const text = (value: unknown, max: number): value is string => typeof value === 'string' && value.length > 0 && new TextEncoder().encode(value).length <= max && !/[\r\n\0]/.test(value);
const revision = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0 && (value as number) < 1_000_000_000;
function httpsURL(value: unknown): URL | null {
  if (!text(value, 2048) || value !== value.trim() || /[\\?#]/.test(value)) return null;
  try { const url = new URL(value); return url.protocol === 'https:' && !url.username && !url.password && url.hostname.includes('.') && !/\.(localhost|local)$/.test(url.hostname) ? url : null; } catch { return null; }
}
/** Client shape checks supplement the server's current authority, source and network guards. */
export function coordinatorDeploymentInputValid(value: unknown): value is CoordinatorDeploymentInput {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !keys.includes(key as typeof keys[number]))) return false;
  const plan = value as CoordinatorDeploymentInput, repository = httpsURL(plan.repository), domain = httpsURL(plan.domain);
  if (plan.privateKeyUuid !== undefined && typeof plan.privateKeyUuid !== 'string') return false;
  return id(plan.targetId) && !!repository && repository.pathname !== '/' && !!domain && domain.pathname === '/' && !plan.domain.endsWith('/') &&
    text(plan.branch, 200) && Number.isSafeInteger(plan.port) && plan.port >= 1 && plan.port <= 65535 &&
    text(plan.healthPath, 1024) && plan.healthPath.startsWith('/') && !plan.healthPath.startsWith('//') && !/[\\?#%]/.test(plan.healthPath) &&
    text(plan.dockerfile, 1024) && !/[\\:\0]/.test(plan.dockerfile) && !plan.dockerfile.startsWith('/') &&
    !plan.dockerfile.split('/').some(part => !part || part === '.' || part === '..' || /^(\.git|\.luci|\.ssh|\.aws|\.azure|\.kube|\.gnupg|\.env(?:\..*)?|\.netrc|\.npmrc|\.pypirc|\.dockercfg|credentials(?:\.json)?|secrets\.json|id_(?:rsa|ed25519|dsa|ecdsa))$|\.(pem|key|p12|pfx)$/i.test(part)) &&
    ['', 'none', 'vite', 'nextjs'].includes(plan.dataClientFramework) &&
    (plan.source === 'public' ? !plan.privateKeyUuid : plan.source === 'private' && id(plan.privateKeyUuid) && repository.host === 'github.com' && /^\/[A-Za-z0-9-]+\/[A-Za-z0-9_.-]+$/.test(repository.pathname));
}
function planValid(value: unknown, acknowledged: boolean): value is CoordinatorDeploymentPlan {
  if (!coordinatorDeploymentInputValid(value)) return false;
  const plan = value as CoordinatorDeploymentPlan;
  return bindingKeys.every(key => key === 'privateKeyUuid' || Object.prototype.hasOwnProperty.call(plan, key)) &&
    digest(plan.targetFingerprint) && id(plan.serverUuid) && revision(plan.expectedConfigurationRevision) && revision(plan.expectedDataDomainRevision) &&
    (plan.expectedDataDomainRevision === 0 ? plan.expectedDataBindingFingerprint === '' : digest(plan.expectedDataBindingFingerprint)) &&
    plan.acknowledgeRepositoryPush === acknowledged && plan.acknowledgePublic === acknowledged && plan.acknowledgeDeploymentCosts === acknowledged;
}
export function coordinatorDeploymentPlanValid(value: unknown): value is CoordinatorDeploymentPlan { return planValid(value, true); }
/** Compare every setting and observation binding, excluding only the three explicit consents. */
export function sameCoordinatorDeploymentBinding(prepared: CoordinatorDeploymentPlan, saved: CoordinatorDeploymentPlan): boolean {
  return planValid(prepared, false) && planValid(saved, true) && bindingKeys.every(key => key === 'privateKeyUuid' ? (prepared[key] ?? '') === (saved[key] ?? '') : prepared[key] === saved[key]);
}
export const coordinatorDeploymentApi = {
  async prepare(projectId: string, input: CoordinatorDeploymentInput, signal?: AbortSignal): Promise<CoordinatorDeploymentPlan> {
    if (!coordinatorDeploymentInputValid(input)) throw new Error('Choose a valid deployment server, repository, branch, HTTPS application domain and build options.');
    if (signal?.aborted) throw new DOMException('Deployment review was cancelled.', 'AbortError');
    const request = { ...input, acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false };
    const value = await productApi.request<CoordinatorDeploymentPlan>(`/v1/product/projects/${encodeURIComponent(projectId)}/orchestrator/deployment-plan`, request, 'POST', 30_000, signal);
    if (signal?.aborted) throw new DOMException('Deployment review was cancelled.', 'AbortError');
    if (!planValid(value, false) || inputKeys.some(key => key === 'privateKeyUuid' ? (value[key] ?? '') !== (input[key] ?? '') : value[key] !== input[key]))
      throw new Error('The deployment preparation response did not match the exact requested settings. Read and review the current plan again; no delegation was saved.');
    return value;
  },
  async targets(signal?: AbortSignal): Promise<DeploymentTarget[]> {
    if (signal?.aborted) throw new DOMException('Deployment server read was cancelled.', 'AbortError');
    const value = await productApi.request<{ targets: DeploymentTarget[] }>('/v1/setup/targets', undefined, 'GET', 15_000, signal);
    if (signal?.aborted) throw new DOMException('Deployment server read was cancelled.', 'AbortError');
    if (!value || !Array.isArray(value.targets) || value.targets.length > 100 || new Set(value.targets.map(target => target?.id)).size !== value.targets.length || value.targets.some(target =>
      !target || Object.keys(target).some(key => !['id', 'name', 'baseUrl', 'serverUuid'].includes(key)) || !id(target.id) || !id(target.serverUuid) || !text(target.name, 512) || !text(target.baseUrl, 2048)))
      throw new Error('Connected deployment servers could not be verified. Read them again before preparing a plan.');
    return value.targets;
  },
};
