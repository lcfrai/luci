import { context, turn, view } from '../components/orchestrator/fixtures';
import type { DeploymentBlueprint, BlueprintProjectContext } from '../api/deploymentBlueprint';
import type { OrchestratorView } from '../api/orchestrator';

export function assistedBlueprintFixture() {
 const ctx = context('p-1');
 const source = { commit: '', configurationRevision: 1, context: { briefRevision: 1, specificationRevision: 2, designRevision: 1 } };
 const blueprint: DeploymentBlueprint = { version: 1, projectId: 'p-1', source, infrastructure: { id: 'cluster', kind: 'kubernetes', reference: '', namespace: '' }, services: [{ id: 'app-1', kind: 'app', image: '', port: 8080, replicas: 1, resources: { requestMilliCpu: 100, limitMilliCpu: 200, requestMemoryMiB: 64, limitMemoryMiB: 128 } }] };
 ctx.deploymentBlueprint = { projectId: 'p-1', savedRevision: 0, savedDigest: '', source, contextDigest: 'b'.repeat(64), available: true, unavailableReason: '', sourceReviewDigest: 'c'.repeat(64), omittedBytes: 0 };
 const proposal = { expectedRevision: 0, expectedDigest: '', contextDigest: ctx.deploymentBlueprint.contextDigest, sourceReviewDigest: ctx.deploymentBlueprint.sourceReviewDigest, blueprint };
 const proposed = turn({ context: ctx, contextDigest: ctx.digest, answer: { summary: 'A small app service', question: '', options: [], rationale: 'Start with the saved project requirements.', blocking: false, impact: ['Service planning only'], nextAction: 'Review in Deploy', proposal: { target: 'deployment-blueprint', summary: 'Plan one application service', deploymentBlueprint: proposal, specification: null, designPlan: null, plan: null, gtm: null } } });
 const result: OrchestratorView = view({ context: ctx, turns: [proposed] });
 const observed: BlueprintProjectContext = { projectId: 'p-1', source, head: '', clean: false, reviewDigest: proposal.sourceReviewDigest, definitionStatus: 'approved', designStatus: 'draft' };
 return { context: ctx, blueprint, proposal, turn: proposed, view: result, observed };
}
