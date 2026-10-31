import { expect,it } from 'vitest';
import { normalizeProjectSkillMetadata,validProjectSkillMetadata } from './skillMetadata';
import { validProjectSkills,type ProjectSkillRegistry } from './projectSkills';
import { validOrchestratorSkillsContext } from './orchestratorSkills';

it('keeps absent legacy metadata absent and normalizes only editable declarations',()=>{
 expect(normalizeProjectSkillMetadata(undefined)).toBeUndefined();expect(normalizeProjectSkillMetadata({license:' ',declaredTools:['']})).toBeUndefined();
 expect(normalizeProjectSkillMetadata({license:' MIT ',sourceRevision:' v2 ',declaredTools:['git',' node','']})).toEqual({license:'MIT',sourceRevision:'v2',declaredTools:['git','node']});
});
it('refuses unsafe source links, ambiguous declarations and purported capability grants',()=>{
 for(const m of [null,{sourceUrl:'javascript:alert(1)'},{sourceUrl:'https://user:secret@example.com/'},{license:'MIT\u202e'},{declaredTools:['git','git']},{declaredTools:['shell;rm']},{declaredTools:Array.from({length:17},(_,i)=>'tool'+i)},{permissions:['all']}])expect(validProjectSkillMetadata(m)).toBe(false);
 expect(validProjectSkillMetadata({license:'unknown',sourceUrl:'https://example.com/skill#usage',declaredTools:['git','mcp:glasses.search']})).toBe(true);
});
it('carries the same metadata through registry and bounded orchestrator context',()=>{
 const metadata={license:'MIT',sourceUrl:'https://example.com/skill',sourceRevision:'v2',declaredTools:['git']};
 const registry:ProjectSkillRegistry={projectId:'p-skills',revision:1,versions:[{id:'source',revision:1,title:'Source aware',provenance:'Team declaration',roles:['coding'],instructions:'Use the saved design.',digest:'a'.repeat(64),metadata}],assignments:[]};
 expect(validProjectSkills(registry,'p-skills')).toBe(true);
 const context={projectId:'p-skills',revision:1,registryDigest:'b'.repeat(64),versions:[{...registry.versions[0],instructionExcerpt:'Use the saved design.'}],omittedVersions:0,assignments:[]};
 expect(validOrchestratorSkillsContext(context,'p-skills')).toBe(true);
 registry.versions[0].metadata={...metadata,declaredTools:['git','git']};expect(validProjectSkills(registry,'p-skills')).toBe(false);
 context.versions[0].metadata={...metadata,sourceUrl:'http://example.com/skill'};expect(validOrchestratorSkillsContext(context,'p-skills')).toBe(false);
});
