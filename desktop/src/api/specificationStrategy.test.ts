import { describe, expect, it } from 'vitest';
import { emptySpecificationStrategy, specificationStrategyDraftValid, strategyProblems } from './specificationStrategy';

describe('strategy draft recovery and linked findings', () => {
  it('keeps unfinished drafts but rejects damaged and authority-bearing shapes', () => {
    const draft = emptySpecificationStrategy();
    draft.assumptions.push({id: 'assumption-1', claim: '', status: 'untested', experiment: '', evidenceIds: [], requirementIds: []});
    expect(specificationStrategyDraftValid(draft)).toBe(true);
    expect(specificationStrategyDraftValid({...draft, evidence: null})).toBe(false);
    expect(specificationStrategyDraftValid({...draft, assumptions: [{...draft.assumptions[0], status: ['supported']}]})).toBe(false);
    expect(specificationStrategyDraftValid({...draft, execute: true})).toBe(false);
    expect(specificationStrategyDraftValid({...draft, assumptions: [{...draft.assumptions[0], status: 'proved'}]})).toBe(false);
  });
  it('surfaces unresolved findings and removed links without mutating the draft', () => {
    const draft = emptySpecificationStrategy();
    draft.assumptions.push({id: 'assumption-1', claim: 'A queue helps', status: 'supported', experiment: 'Pilot', evidenceIds: [], requirementIds: ['req-1']});
    expect(strategyProblems(draft, ['req-1'])).toContain('Link an observation before marking an assumption supported or contradicted.');
    draft.evidence.push({id: 'source-1', source: 'Pilot notes', url: '', observation: 'Fewer missed requests', observedAt: ''});
    draft.assumptions[0].evidenceIds = ['source-1'];
    expect(strategyProblems(draft, ['req-1'])).toEqual([]);
    expect(strategyProblems(draft, [])).toContain('Review strategy links to removed requirements before saving.');
    draft.evidence = [];
    expect(strategyProblems(draft, ['req-1'])).toContain('Remove unavailable source links or restore the source before saving.');
    expect(draft.assumptions[0].evidenceIds).toEqual(['source-1']);
  });
});
