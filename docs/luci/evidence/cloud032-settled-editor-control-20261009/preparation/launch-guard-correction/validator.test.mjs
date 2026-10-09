import {test} from 'node:test';import assert from 'node:assert/strict';
import {validateSourceBinding} from '../actual/source-binding.mjs';
const helper='4'.repeat(40), observed='d'.repeat(40),product='e'.repeat(40);
const base={provenance:{sourceDirty:false,sourceCommit:product},expectedProduct:product,expectedHelper:helper,helperCommit:observed,helperDirty:false};
const selected={bindingMode:'exact-selected-helper-inputs-with-docs-only-descendant-checkout',helperSource:helper,observedCheckoutCommit:observed,observedCheckoutDirty:false,docsOnlyChangedPaths:['docs/luci/a.md'],selectedSources:Object.fromEntries(Array.from({length:17},(_,i)=>['input'+i,{}]))};
test('ordinary strict validator still refuses different checkout',()=>assert.throws(()=>validateSourceBinding(base),/checkout must match/));
test('explicit proven input mode preserves actual checkout and helper separately',()=>{const r=validateSourceBinding({...base,selectedHelperEvidence:selected});assert.equal(r.commit,observed);assert.equal(r.selectedHelperSource,helper);assert.equal(r.expectedSource,helper)});
test('wrong observed identity and incomplete inputs refuse',()=>{assert.throws(()=>validateSourceBinding({...base,selectedHelperEvidence:{...selected,observedCheckoutCommit:helper}}));assert.throws(()=>validateSourceBinding({...base,selectedHelperEvidence:{...selected,selectedSources:{}}}))});
test('product mismatch and non-doc delta still refuse',()=>{assert.throws(()=>validateSourceBinding({...base,expectedProduct:helper,selectedHelperEvidence:selected}));assert.throws(()=>validateSourceBinding({...base,selectedHelperEvidence:{...selected,docsOnlyChangedPaths:['desktop/src/App.tsx']}}))});
