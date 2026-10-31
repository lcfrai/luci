import { expect, it } from 'vitest';
import { assistedBlueprintFixture } from '../test/blueprintAssistanceFixtures';
import { blueprintDraftIntent, blueprintIntentMatchesContext, validBlueprintProposal, validOrchestratorBlueprintContext } from './orchestratorBlueprint';

it('preserves legacy absence and requires exact saved/context/source provenance for topology proposals', () => {
 const f = assistedBlueprintFixture(); expect(validOrchestratorBlueprintContext(undefined, 'p-1')).toBe(true); expect(validBlueprintProposal(f.context, f.turn.answer!.proposal)).toBe(true);
 const changed = structuredClone(f.turn.answer!.proposal!); changed.deploymentBlueprint!.expectedRevision = 1;
 expect(validBlueprintProposal(f.context, changed)).toBe(false); changed.deploymentBlueprint!.expectedRevision = 0; changed.deploymentBlueprint!.sourceReviewDigest = 'd'.repeat(64);
 expect(validBlueprintProposal(f.context, changed)).toBe(false); expect(validOrchestratorBlueprintContext({ ...f.context.deploymentBlueprint!, projectId: 'other' }, 'p-1')).toBe(false);
});
it('retains exact detached proposal bytes and refuses reopened proposals after dirty-source changes', () => {
 const f = assistedBlueprintFixture(), intent = blueprintDraftIntent(f.turn, 'actor/workspace/p-1');
 expect(blueprintIntentMatchesContext(intent, f.context)).toBe(true);
 intent.proposal.blueprint.services[0].replicas = 3; expect(f.proposal.blueprint.services[0].replicas).toBe(1);
 expect(blueprintIntentMatchesContext(intent, { ...f.context, deploymentBlueprint: { ...f.context.deploymentBlueprint!, sourceReviewDigest: 'd'.repeat(64) } })).toBe(false);
 expect(() => blueprintDraftIntent({ ...f.turn, appliedRevision: 1 }, 'scope')).toThrow();
});
it('withdraws only topology assistance for an unconfirmed source capture and rejects cross-target attachments', () => {
 const f = assistedBlueprintFixture(), unavailable = { ...f.context.deploymentBlueprint!, available: false, unavailableReason: 'source-unconfirmed', sourceReviewDigest: '' };
 expect(validOrchestratorBlueprintContext(unavailable, 'p-1')).toBe(true);
 expect(validBlueprintProposal({ ...f.context, deploymentBlueprint: unavailable }, f.turn.answer!.proposal)).toBe(false);
 expect(validBlueprintProposal(f.context, { ...f.turn.answer!.proposal!, target: 'gtm' })).toBe(false);
});
