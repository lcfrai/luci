import { validStoredBlueprint, type BlueprintSource, type DeploymentBlueprint } from './deploymentBlueprint';
import type { OrchestratorContext, OrchestratorProposal, OrchestratorTurn } from './orchestrator';

export interface OrchestratorBlueprintContext {
 projectId: string; savedRevision: number; savedDigest: string; source: BlueprintSource; contextDigest: string;
 available: boolean; unavailableReason: string; sourceReviewDigest: string; omittedBytes: number; saved?: DeploymentBlueprint;
}
export interface OrchestratorBlueprintProposal {
 expectedRevision: number; expectedDigest: string; contextDigest: string; sourceReviewDigest: string; blueprint: DeploymentBlueprint;
}
/** One scoped, in-memory transfer; not persisted preferences or save authority. */
export interface BlueprintDraftIntent {
 id: string; scope: string; projectId: string; turnId: string; turnContextDigest: string;
 proposal: OrchestratorBlueprintProposal;
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const integer = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) >= 0 && (v as number) <= 1_000_000_000;
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
export const blueprintValueKey = (value: unknown): string => JSON.stringify(value, (_key, item: unknown) => object(item) ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)) : item);
export function validBlueprintSource(value: unknown): value is BlueprintSource {
 if (!object(value) || typeof value.commit !== 'string' || value.commit !== '' && !/^[a-f0-9]{40}$/.test(value.commit) || !integer(value.configurationRevision) || !object(value.context)) return false;
 const c = value.context;
 return integer(c.briefRevision) && integer(c.specificationRevision) && integer(c.designRevision) && (c.specificationDigest === undefined || c.specificationDigest === '' || hash(c.specificationDigest)) && (c.designDigest === undefined || c.designDigest === '' || hash(c.designDigest));
}
export function validOrchestratorBlueprintContext(value: OrchestratorBlueprintContext | undefined, projectId: string): boolean {
 if (value === undefined) return true;
 if (!value || value.projectId !== projectId || !integer(value.savedRevision) || (value.savedRevision === 0 ? value.savedDigest !== '' : !hash(value.savedDigest)) || !validBlueprintSource(value.source) || !hash(value.contextDigest) || typeof value.available !== 'boolean' || !integer(value.omittedBytes)) return false;
 if (!value.available) {
  if (value.saved !== undefined) return false;
  if (value.unavailableReason === 'saved-topology-unconfirmed') return value.savedRevision === 0 && value.savedDigest === '' && value.omittedBytes === 0 && (value.sourceReviewDigest === '' || hash(value.sourceReviewDigest));
  if (value.unavailableReason === 'saved-topology-unsupported') return value.savedRevision > 0 && value.omittedBytes > 0 && (value.sourceReviewDigest === '' || hash(value.sourceReviewDigest));
  return value.unavailableReason === 'source-unconfirmed' ? value.sourceReviewDigest === '' : value.unavailableReason === 'topology-size-limit' && hash(value.sourceReviewDigest) && value.savedRevision > 0 && value.omittedBytes > 48 * 1024;
 }
 return value.unavailableReason === '' && hash(value.sourceReviewDigest) && value.omittedBytes === 0 && (value.savedRevision === 0 ? value.saved === undefined : !!value.saved && validStoredBlueprint(value.saved, projectId) && new TextEncoder().encode(JSON.stringify(value.saved)).length <= 48 * 1024);
}
export function validBlueprintProposal(context: OrchestratorContext, proposal: OrchestratorProposal | null | undefined): boolean {
 if (!proposal) return true;
 if (!['specification', 'design-plan', 'plan', 'gtm', 'deployment-blueprint'].includes(proposal.target)) return false;
 if (proposal.target !== 'deployment-blueprint') return proposal.deploymentBlueprint == null;
 const captured = context.deploymentBlueprint, candidate = proposal.deploymentBlueprint;
 return !!captured && validOrchestratorBlueprintContext(captured, context.projectId) && captured.available && !!candidate &&
   candidate.expectedRevision === captured.savedRevision && candidate.expectedDigest === captured.savedDigest && candidate.contextDigest === captured.contextDigest && candidate.sourceReviewDigest === captured.sourceReviewDigest &&
   validStoredBlueprint(candidate.blueprint, context.projectId) && blueprintValueKey(candidate.blueprint.source) === blueprintValueKey(captured.source) &&
   new TextEncoder().encode(JSON.stringify(candidate.blueprint)).length <= 48 * 1024 && proposal.specification === null && proposal.designPlan === null && proposal.plan === null && proposal.gtm == null;
}
export function blueprintDraftIntent(turn: OrchestratorTurn, scope: string): BlueprintDraftIntent {
 if (!scope || turn.projectId !== turn.context.projectId || turn.contextDigest !== turn.context.digest || !hash(turn.contextDigest) || !/^turn-[A-Za-z0-9_-]{1,123}$/.test(turn.id) || turn.status !== 'succeeded' || !!turn.appliedRevision || !validBlueprintProposal(turn.context, turn.answer?.proposal)) throw new Error('Review a current, complete service-blueprint proposal before opening Deploy.');
 return { id: crypto.randomUUID(), scope, projectId: turn.projectId, turnId: turn.id, turnContextDigest: turn.contextDigest,
   proposal: JSON.parse(JSON.stringify(turn.answer!.proposal!.deploymentBlueprint!)) as OrchestratorBlueprintProposal };
}
export function blueprintIntentMatchesContext(intent: BlueprintDraftIntent, context: OrchestratorContext): boolean {
 const b = context.deploymentBlueprint;
 return context.projectId === intent.projectId && context.digest === intent.turnContextDigest && !!b && b.available &&
   b.savedRevision === intent.proposal.expectedRevision && b.savedDigest === intent.proposal.expectedDigest && b.contextDigest === intent.proposal.contextDigest && b.sourceReviewDigest === intent.proposal.sourceReviewDigest &&
   blueprintValueKey(b.source) === blueprintValueKey(intent.proposal.blueprint.source);
}
