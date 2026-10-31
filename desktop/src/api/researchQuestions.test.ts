import { expect, it } from 'vitest';
import { requestResearchQuestion, takeResearchQuestion } from './researchQuestions';
it('keeps proposed research questions identity-scoped and cancellable without a network effect',()=>{
 const cancel=requestResearchQuestion('alice/workspace','project-one','What do repair volunteers use today?');expect(takeResearchQuestion('bob/workspace','project-one')).toBeNull();cancel();expect(takeResearchQuestion('alice/workspace','project-one')).toBeNull();
 requestResearchQuestion('alice/workspace','project-one','Compare repair request workarounds');expect(takeResearchQuestion('alice/workspace','project-two')).toBeNull();expect(takeResearchQuestion('alice/workspace','project-one')).toBe('Compare repair request workarounds');expect(takeResearchQuestion('alice/workspace','project-one')).toBeNull();
});
