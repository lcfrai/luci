import { beforeEach, expect, it } from 'vitest';
import { clearDiscoveryRecovery, readDiscoveryRecovery, retainDiscoveryRecovery, type PendingDiscoveryRequest } from './discoveryDocumentRecovery';
const pending:PendingDiscoveryRequest={action:'extract',idempotencyKey:'document-request-123456789',documentId:`doc-${'a'.repeat(64)}`,expectedRevision:1};
beforeEach(()=>localStorage.clear());
it('retains the exact request by actor/workspace/project scope and never mixes another scope',()=>{
  retainDiscoveryRecovery('actor-one/local/project-one',pending);expect(readDiscoveryRecovery('actor-one/local/project-one')).toEqual(pending);expect(readDiscoveryRecovery('actor-two/local/project-one')).toBeNull();expect(readDiscoveryRecovery('actor-one/team-two/project-one')).toBeNull();
});
it('preserves uncertain requests and refuses replacement or clearing another window request',()=>{
  retainDiscoveryRecovery('scope',pending);expect(()=>retainDiscoveryRecovery('scope',{...pending,idempotencyKey:'document-other-123456789'})).toThrow();
  expect(()=>clearDiscoveryRecovery('scope',{...pending,expectedRevision:2})).toThrow();expect(readDiscoveryRecovery('scope')).toEqual(pending);clearDiscoveryRecovery('scope',pending);expect(readDiscoveryRecovery('scope')).toBeNull();
});
it('counts multibyte correction reason with the retained attribution prefix and refuses oversized intent',()=>{
  const correct:PendingDiscoveryRequest={action:'correct',idempotencyKey:'document-correction-12345',documentId:pending.documentId,expectedRevision:3,extractionRevision:1,changes:[{page:1,segmentId:'p1-line1',expectedTextDigest:'a'.repeat(64),text:'Corrected',reason:'é'.repeat(991)}]};
  retainDiscoveryRecovery('scope',correct);expect(readDiscoveryRecovery('scope')).toEqual(correct);clearDiscoveryRecovery('scope',correct);
  correct.changes[0].reason+='é';expect(()=>retainDiscoveryRecovery('scope',correct)).toThrow();
});
